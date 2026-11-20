import Foundation
import AVFoundation
import Speech

// The take RECORDER: the voice half of a take. It holds the microphone and the radio's gate, writes the file, reads it with the on-device recogniser, plays it back for review and stops itself at
// the ceiling. It decides nothing about publishing: the session (whether to publish, to whom, with what words, the listener's own ids and muted voices) is shared code (AutoradioLab/take-session.ts,
// takes.ts). The shared code hands over the numbers (`TakeLimits`) and reads one snapshot back (`snapshot`): a phase, the length, the transcript, and `ended`, a CODE for why a take ended without a
// recording (mic, gate, open, short, empty, large; `ceiling` when the recorder stopped itself and the take is in review). The words for each code are shared code's.
//
// Ways this can fail (each is a test in TakeSessionTests):
//   R1  recording starts only when the microphone is allowed and the radio has yielded; the radio is paused the whole time a take is in hand (recording and review) and given back exactly once,
//       on publish (done), discard or a failure that ends the take
//   R2  a take under the minimum is not kept and says so; the ceiling stops it by itself and says so
//   R3  re-record deletes the previous file before the new one starts; discard deletes the file; nothing leaves the phone from here
//   R6  the entry on air when recording began is `responds_to`; no entry leaves it empty
//   R7  logs carry counts, never the words

public enum TakeState: String, Sendable { case idle, recording, transcribing, review }

/// What the shared code asks of the recorder (take-session.ts LIMITS): the spec's numbers, not ours.
public struct TakeLimits: Sendable, Equatable {
    public var minSeconds: Double, maxSeconds: Double, maxBytes: Int
    public init(minSeconds: Double, maxSeconds: Double, maxBytes: Int) { self.minSeconds = minSeconds; self.maxSeconds = maxSeconds; self.maxBytes = maxBytes }
}
/// AAC-LC in .m4a, mono, 24 kHz, 32 kbit/s: how the file is written.
enum TakeRecorderFormat { static let sampleRate = 24_000.0, bitRate = 32_000 }

@MainActor public protocol TakeRecorderEngine: AnyObject {
    func start(to url: URL) throws
    /// Stop and keep the file; the seconds recorded.
    func stop() -> Double
    /// Stop and drop what was recorded.
    func cancel()
}
public protocol TakeTranscribing: Sendable { func transcribe(file: URL) async -> String }
@MainActor public protocol TakeRadioGate: AnyObject {
    /// The radio goes quiet and gives the microphone to the take (the one voice). Nil: done; else why it could not.
    func yieldForTake() async -> String?
    /// The take is over: the radio goes on as it was.
    func giveBack()
}
@MainActor public protocol TakePreviewing: AnyObject {
    func play(_ url: URL)
    func stop()
}

@MainActor public final class TakeSession {
    public private(set) var state: TakeState = .idle
    public private(set) var transcript = ""
    public private(set) var duration = 0.0
    public private(set) var bytes = 0
    public private(set) var respondsTo: TakeResponds?
    public private(set) var previewing = false
    /// Shared code is sending the take. Only so the snapshot moves and the page is asked again; shared code clears it, and nothing here reads it as a fact.
    public private(set) var publishing = false
    /// mic | gate | open | short | empty | large | ceiling, or "" (see the header). Cleared by the next start, a discard and done.
    public private(set) var ended = ""
    public private(set) var detail = ""
    public var onChange: (() -> Void)?
    /// The one session of the app (TakesHost sets it): sign-out and an account change discard its take before anything else (ChoirApp.signOut, AccountWipe), so a take is never shown to, or published by, the next account.
    nonisolated(unsafe) public static weak var live: TakeSession?

    let recorder: TakeRecorderEngine, transcriber: TakeTranscribing, gate: TakeRadioGate, preview: TakePreviewing
    let micAllowed: () async -> Bool
    let responds: () -> TakeResponds?
    let dir: URL
    let now: () -> Date
    private var limits = TakeLimits(minSeconds: 3, maxSeconds: 59.5, maxBytes: 512 * 1024)
    private var file: URL?
    private var startedAt: Date?
    private var holding = false

    public init(recorder: TakeRecorderEngine, transcriber: TakeTranscribing, gate: TakeRadioGate, preview: TakePreviewing,
                micAllowed: @escaping () async -> Bool, responds: @escaping () -> TakeResponds?, dir: URL, now: @escaping () -> Date = Date.init) {
        self.recorder = recorder; self.transcriber = transcriber; self.gate = gate; self.preview = preview
        self.micAllowed = micAllowed; self.responds = responds; self.dir = dir; self.now = now
    }

    public var elapsed: Double { startedAt.map { max(0, now().timeIntervalSince($0)) } ?? (state == .idle ? 0 : duration) }
    private func changed() { onChange?() }

    /// The snapshot the shared code reads (take-session.ts `Native`).
    public var snapshot: [String: Any] {
        ["phase": state.rawValue, "elapsed": elapsed, "seconds": duration, "transcript": transcript, "previewing": previewing, "publishing": publishing, "ended": ended, "detail": detail,
         "respondsTitle": respondsTo?.title ?? "", "respondsRoute": respondsTo?.route ?? ""]
    }
    /// The recording's bytes, for the shared code to upload (the file stays here: the preview plays it).
    public var audio: Data? { file.flatMap { try? Data(contentsOf: $0) } }

    // MARK: Recording

    /// Start (from idle) or start again (from review: the previous file is deleted first).
    public func start(_ l: TakeLimits) async {
        guard state == .idle || state == .review else { return }
        limits = l
        if state == .review { dropFile(); transcript = ""; duration = 0; bytes = 0 }
        ended = ""; detail = ""
        guard await micAllowed() else { ended = "mic"; if holding { endHold() }; state = .idle; changed(); return }
        if !holding {
            if let why = await gate.yieldForTake() { ended = "gate"; detail = why; state = .idle; changed(); return }
            holding = true
        }
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let url = dir.appendingPathComponent("take-\(UUID().uuidString.prefix(8).lowercased()).m4a")
        respondsTo = responds()
        do { try recorder.start(to: url) } catch { ended = "open"; endHold(); state = .idle; changed(); return }
        file = url; startedAt = now(); state = .recording
        print("AUTORADIO take recording started responds=\(respondsTo != nil)")
        changed()
    }

    /// The module calls this a few times a second while recording: the ceiling stops it.
    public func tick() async {
        guard state == .recording, elapsed >= limits.maxSeconds else { return }
        await stop(auto: true)
    }

    public func stop(auto: Bool = false) async {
        guard state == .recording, let url = file else { return }
        let secs = recorder.stop()
        startedAt = nil
        // The file's own length when it can be read (it includes the encoder's priming, about 0.11 s).
        let real = Self.fileSeconds(url) ?? secs
        duration = max(real.isFinite ? real : 0, 0)
        if duration < limits.minSeconds {
            try? FileManager.default.removeItem(at: url); file = nil; duration = 0
            ended = "short"; endHold(); state = .idle; changed(); return
        }
        bytes = (try? Data(contentsOf: url).count) ?? 0
        if bytes > limits.maxBytes || bytes == 0 {
            let none = bytes == 0
            try? FileManager.default.removeItem(at: url); file = nil; bytes = 0
            ended = none ? "empty" : "large"; endHold(); state = .idle; changed(); return
        }
        state = .transcribing
        ended = auto ? "ceiling" : ""
        changed()
        let words = await transcriber.transcribe(file: url)
        guard state == .transcribing else { return }          // discarded while it was being read
        transcript = words.trimmingCharacters(in: .whitespacesAndNewlines)
        state = .review
        print("AUTORADIO take recorded seconds=\(Int(duration)) bytes=\(bytes) chars=\(transcript.count)")
        changed()
    }

    static func fileSeconds(_ url: URL) -> Double? {
        guard let f = try? AVAudioFile(forReading: url), f.processingFormat.sampleRate > 0 else { return nil }
        return Double(f.length) / f.processingFormat.sampleRate
    }

    // MARK: Review

    public func togglePreview() { setPreview(!previewing) }
    /// `on` plays the recording; off stops it. Only a take in review is played.
    public func setPreview(_ on: Bool) {
        guard on != previewing else { return }
        if on { guard state == .review, let f = file else { return }; preview.play(f); previewing = true } else { preview.stop(); previewing = false }
        changed()
    }
    public func setPublishing(_ on: Bool) { guard on != publishing else { return }; publishing = on; changed() }
    /// The preview reached its end.
    public func previewEnded() { previewing = false; changed() }

    public func discard() {
        if state == .recording { recorder.cancel() }
        dropFile(); transcript = ""; duration = 0; bytes = 0; startedAt = nil; ended = ""; detail = ""; publishing = false; respondsTo = nil
        if state != .idle || holding { endHold(); state = .idle }
        changed()
    }

    /// The take was published (shared code did it): the file goes and the radio is given back.
    public func done() { discard() }

    // MARK: Internals

    private func dropFile() { if let f = file { try? FileManager.default.removeItem(at: f) }; file = nil; preview.stop(); previewing = false }
    private func endHold() { if holding { holding = false; gate.giveBack() } }
}

// MARK: The system parts

/// AVAudioRecorder to AAC-LC in an .m4a, mono, 24 kHz, 32 kbit/s (11.2). The session is the owner's (the radio gate claims it first).
@MainActor public final class AVTakeRecorder: TakeRecorderEngine {
    private var rec: AVAudioRecorder?
    public init() {}
    public func start(to url: URL) throws {
        let settings: [String: Any] = [AVFormatIDKey: kAudioFormatMPEG4AAC, AVSampleRateKey: TakeRecorderFormat.sampleRate, AVNumberOfChannelsKey: 1,
                                       AVEncoderBitRateKey: TakeRecorderFormat.bitRate]
        let r = try AVAudioRecorder(url: url, settings: settings)
        guard r.prepareToRecord(), r.record() else { throw LabError.unavailable("the recorder did not start") }
        rec = r
    }
    public func stop() -> Double { let t = rec?.currentTime ?? 0; rec?.stop(); rec = nil; return t }
    public func cancel() { rec?.stop(); rec?.deleteRecording(); rec = nil }
}

/// Plays the recording back for review.
@MainActor public final class AVTakePreview: NSObject, TakePreviewing, AVAudioPlayerDelegate {
    private var player: AVAudioPlayer?
    public var onEnd: (() -> Void)?
    public override init() { super.init() }
    public func play(_ url: URL) { player = try? AVAudioPlayer(contentsOf: url); player?.delegate = self; player?.play() }
    public func stop() { player?.stop(); player = nil }
    nonisolated public func audioPlayerDidFinishPlaying(_ p: AVAudioPlayer, successfully ok: Bool) { Task { @MainActor in self.player = nil; self.onEnd?() } }
}

/// The transcript, made on the phone from the recorded file with the recogniser the app already has: the file is read as 16 kHz mono frames
/// and fed to the same on-device `Transcriber` stage the turns use. No audio and no words leave the phone.
public struct FileTranscriber: TakeTranscribing {
    let make: @Sendable () -> Transcriber
    public init(make: @escaping @Sendable () -> Transcriber) { self.make = make }
    public func transcribe(file: URL) async -> String {
        guard let frames = Self.frames(file) else { return "" }
        do {
            let s = try await make().begin()
            for f in frames { s.append(f) }
            return try await s.finish()
        } catch { print("AUTORADIO take transcript failed"); return "" }
    }
    static func frames(_ url: URL) -> [AudioFrame]? {
        guard let f = try? AVAudioFile(forReading: url), let out = AVAudioFormat(commonFormat: .pcmFormatFloat32, sampleRate: 16_000, channels: 1, interleaved: false),
              let conv = AVAudioConverter(from: f.processingFormat, to: out) else { return nil }
        var frames: [AudioFrame] = []
        var t = 0.0
        let inCap: AVAudioFrameCount = 16_384
        while f.framePosition < f.length {
            guard let inb = AVAudioPCMBuffer(pcmFormat: f.processingFormat, frameCapacity: inCap), (try? f.read(into: inb)) != nil, inb.frameLength > 0 else { break }
            let cap = AVAudioFrameCount(Double(inb.frameLength) * 16_000 / f.processingFormat.sampleRate) + 64
            guard let ob = AVAudioPCMBuffer(pcmFormat: out, frameCapacity: cap) else { return nil }
            var fed = false
            var err: NSError?
            conv.convert(to: ob, error: &err) { _, status in if fed { status.pointee = .noDataNow; return nil }; fed = true; status.pointee = .haveData; return inb }
            if err != nil { return nil }
            guard let ch = ob.floatChannelData, ob.frameLength > 0 else { continue }
            let s = Array(UnsafeBufferPointer(start: ch[0], count: Int(ob.frameLength)))
            frames.append(AudioFrame(samples: s, sampleRate: 16_000, t: t)); t += Double(s.count) / 16_000
        }
        return frames
    }
}
