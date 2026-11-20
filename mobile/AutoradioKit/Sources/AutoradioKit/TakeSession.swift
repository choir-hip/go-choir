import Foundation
import AVFoundation
import Speech

// Recording a take: start, stop, review, publish (design 11.2). This file is the state machine and the system parts behind four small
// protocols, so the machine is tested with fakes and only the real recorder, recogniser and player need a device.
//
// Ways this can fail (written before the code; each is a test in TakeSessionTests):
//   R1  recording starts only when the microphone is allowed and the radio has yielded; the radio is paused the whole time a take is in
//       hand (recording and review) and given back exactly once, on publish, discard or a failure that ends the take
//   R2  a take of under 3 s is not kept and says so; the 60 s ceiling stops it by itself and the notice says it did
//   R3  re-record deletes the previous file before the new one starts; discard deletes the file; nothing leaves the phone before Publish
//   R4  a transcript that comes back empty cannot be published; the transcript is never editable (there is no setter)
//   R5  publish uploads, then publishes, and remembers the id and the publisher's handle so the public list never offers the voice back;
//       a failure after the upload keeps the stored id, so a retry does not upload twice
//   R6  the entry on air when recording began is `responds_to`; no entry (or an opening, an answer, a take) leaves it absent
//   R7  logs carry counts and ids, never the words

public enum TakeState: String, Sendable { case idle, recording, transcribing, review, publishing }

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
    public private(set) var notice = ""
    public private(set) var transcript = ""
    public private(set) var duration = 0.0
    public private(set) var respondsTo: TakeResponds?
    public private(set) var previewing = false
    public private(set) var published: Take?
    public private(set) var mine: [MyTake] = []
    public private(set) var myRecord: TakeRecordSummary?
    public var onChange: (() -> Void)?

    let recorder: TakeRecorderEngine, transcriber: TakeTranscribing, gate: TakeRadioGate, preview: TakePreviewing
    let client: () -> TakesClient?
    let micAllowed: () async -> Bool
    let responds: () -> TakeResponds?
    let own: TakeOwn
    let dir: URL
    let now: () -> Date
    private var file: URL?
    private var startedAt: Date?
    private var storedID: String?
    private var holding = false
    private var sha: String { file.flatMap { try? Data(contentsOf: $0) }.map(TakesClient.Upload.hex) ?? "" }

    public init(recorder: TakeRecorderEngine, transcriber: TakeTranscribing, gate: TakeRadioGate, preview: TakePreviewing, client: @escaping () -> TakesClient?,
                micAllowed: @escaping () async -> Bool, responds: @escaping () -> TakeResponds?, own: TakeOwn = .shared, dir: URL, now: @escaping () -> Date = Date.init) {
        self.recorder = recorder; self.transcriber = transcriber; self.gate = gate; self.preview = preview; self.client = client
        self.micAllowed = micAllowed; self.responds = responds; self.own = own; self.dir = dir; self.now = now
    }

    public var elapsed: Double { startedAt.map { max(0, now().timeIntervalSince($0)) } ?? (state == .idle ? 0 : duration) }
    public var canPublish: Bool { state == .review && !transcript.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty }
    private func changed() { onChange?() }

    // MARK: Recording

    /// Start (from idle) or start again (from review: the previous file is deleted first).
    public func start() async {
        guard state == .idle || state == .review else { return }
        if state == .review { dropFile(); storedID = nil; transcript = ""; duration = 0 }
        notice = ""; published = nil
        guard await micAllowed() else { notice = "The microphone is not allowed. Allow it in Settings to record a take."; if holding { endHold() }; state = .idle; changed(); return }
        if !holding {
            if let why = await gate.yieldForTake() { notice = "Could not start: \(why)"; state = .idle; changed(); return }
            holding = true
        }
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let url = dir.appendingPathComponent("take-\(UUID().uuidString.prefix(8).lowercased()).m4a")
        respondsTo = responds()
        do { try recorder.start(to: url) } catch { notice = "Could not record: the microphone could not be opened."; endHold(); state = .idle; changed(); return }
        file = url; startedAt = now(); state = .recording
        print("AUTORADIO take recording started responds=\(respondsTo != nil)")
        changed()
    }

    /// The module calls this a few times a second while recording: the ceiling stops it.
    public func tick() async {
        guard state == .recording, elapsed >= TakeBounds.maxSeconds - TakeBounds.stopEarly else { return }
        await stop(auto: true)
    }

    public func stop(auto: Bool = false) async {
        guard state == .recording, let url = file else { return }
        let secs = recorder.stop()
        startedAt = nil
        // The file's own length when it can be read (it includes the encoder's priming, about 0.11 s); the ceiling is the server's, so clamp to it.
        let real = (try? Data(contentsOf: url)).flatMap(Mp4Header.durationSeconds) ?? secs
        duration = min(max(real.isFinite ? real : 0, 0), TakeBounds.maxSeconds)
        if duration < TakeBounds.minSeconds {
            try? FileManager.default.removeItem(at: url); file = nil; duration = 0
            notice = "A take is at least \(Int(TakeBounds.minSeconds)) seconds. Nothing was kept."
            endHold(); state = .idle; changed(); return
        }
        let bytes = (try? Data(contentsOf: url).count) ?? 0
        if bytes > TakeBounds.maxBytes || bytes == 0 {
            try? FileManager.default.removeItem(at: url); file = nil
            notice = bytes == 0 ? "Nothing was recorded." : "That take is too large to publish. Record a shorter one."
            endHold(); state = .idle; changed(); return
        }
        state = .transcribing
        notice = auto ? "Stopped at \(Int(TakeBounds.maxSeconds)) seconds, the longest a take can be." : ""
        changed()
        let words = await transcriber.transcribe(file: url)
        guard state == .transcribing else { return }          // discarded while it was being read
        transcript = StretchCandidates.cut(words.trimmingCharacters(in: .whitespacesAndNewlines), bytes: TakeBounds.maxTranscriptBytes)
        if transcript.isEmpty { notice = (notice.isEmpty ? "" : notice + " ") + "Nothing was recognised. Record it again." }
        state = .review
        print("AUTORADIO take recorded seconds=\(Int(duration)) bytes=\(bytes) chars=\(transcript.count)")
        changed()
    }

    // MARK: Review

    public func togglePreview() {
        guard state == .review, let f = file else { return }
        if previewing { preview.stop(); previewing = false } else { preview.play(f); previewing = true }
        changed()
    }
    /// The preview reached its end.
    public func previewEnded() { previewing = false; changed() }

    public func discard() {
        guard state == .review || state == .recording || state == .transcribing else { return }
        if state == .recording { recorder.cancel() }
        dropFile(); storedID = nil; transcript = ""; duration = 0; startedAt = nil; notice = ""
        endHold(); state = .idle; changed()
    }

    // MARK: Publish

    public func publish() async {
        guard canPublish, let url = file, let c = client() else {
            if state == .review, client() == nil { notice = "Sign in to publish a take."; changed() }
            return
        }
        preview.stop(); previewing = false
        state = .publishing; notice = ""; changed()
        do {
            guard let data = try? Data(contentsOf: url) else { throw ChoirError.decode("the recording is gone") }
            // The server reads the length from the file's own header and holds the claim to within 2 s of it: claim what the file says.
            let claimed = min(max(Mp4Header.durationSeconds(data).flatMap { $0.isFinite && $0 > 0 ? $0 : nil } ?? duration, TakeBounds.minSeconds), TakeBounds.maxSeconds)
            let t = try await c.publish(.init(audio: data, transcript: transcript, duration: claimed, respondsTo: respondsTo))
            let id = t.takeID
            own.add(id: id, handle: t.publisher)
            published = t
            dropFile(); storedID = nil; transcript = ""; duration = 0
            endHold(); state = .idle
            print("AUTORADIO take published id=\(id)")
            changed()
            await refresh()
        } catch {
            state = .review
            // The recording is kept (state review, file on disk): nothing is lost when the server is not there yet or refuses.
            if error is TakesUnavailable { notice = "Takes are not available yet. Your recording is kept; publish it again later." }
            else if let r = error as? TakesRefused { notice = "\(r.description) Your recording is kept." }
            else { notice = "Could not publish: \(Self.why(error)). Your recording is kept; try again." }
            print("AUTORADIO take publish failed: \(Self.why(error))")
            changed()
        }
    }

    static func why(_ e: Error) -> String { (e as? ChoirError).map { $0.description } ?? (e as? TakesUnavailable).map { $0.description } ?? "the connection failed" }

    // MARK: The listener's takes and record

    public func refresh() async {
        guard let c = client() else { return }
        do { let m = try await c.mine(); mine = m; for t in m { own.add(id: t.takeID, handle: t.publisher) } }
        catch is TakesUnavailable { notice = "Takes are not available yet."; changed(); return }
        catch {}
        let h = own.handle
        if !h.isEmpty { myRecord = try? await c.record(publisher: h) }
        changed()
    }

    public func delete(_ id: String) async {
        guard let c = client() else { return }
        do { try await c.delete(takeID: id); own.remove(id: id); mine.removeAll { $0.takeID == id }; notice = ""; print("AUTORADIO take deleted id=\(id)") }
        catch { notice = "Could not delete: \(Self.why(error))." }
        changed()
    }

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
        let settings: [String: Any] = [AVFormatIDKey: kAudioFormatMPEG4AAC, AVSampleRateKey: TakeBounds.sampleRate, AVNumberOfChannelsKey: 1,
                                       AVEncoderBitRateKey: TakeBounds.bitRate]
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
