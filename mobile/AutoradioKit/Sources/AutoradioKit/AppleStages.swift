import Foundation
import AVFoundation
import Speech

func registerAppleStages(_ r: StageRegistry) {
    r.register(.transcriber, SFSpeechTranscriber.self) { SFSpeechTranscriber($0) }
    r.register(.voice, AVSpeechVoice.self) { AVSpeechVoice($0) }
    r.register(.transcriber, ScriptedTranscriber.self) { ScriptedTranscriber($0) }
    if #available(iOS 26.0, macOS 26.0, *) {
        r.register(.transcriber, SpeechAnalyzerTranscriber.self) { SpeechAnalyzerTranscriber($0) }
    }
}

func pcmBuffer(_ f: AudioFrame) -> AVAudioPCMBuffer? {
    guard f.sampleRate.isFinite, f.sampleRate > 0,
          let fmt = AVAudioFormat(commonFormat: .pcmFormatFloat32, sampleRate: f.sampleRate, channels: 1, interleaved: false),
          let b = AVAudioPCMBuffer(pcmFormat: fmt, frameCapacity: AVAudioFrameCount(f.samples.count)) else { return nil }
    b.frameLength = AVAudioFrameCount(f.samples.count)
    f.samples.withUnsafeBufferPointer { if let base = $0.baseAddress, let ch = b.floatChannelData { ch[0].update(from: base, count: f.samples.count) } }
    return b
}

// MARK: Transcription: SFSpeechRecognizer, on device only

public final class SFSpeechTranscriber: Transcriber, @unchecked Sendable {
    public static let stageID = "apple-speech"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Apple on-device (SFSpeechRecognizer)", summary: "requiresOnDeviceRecognition = true; partial and final results.",
                        params: [.text("locale", "Locale", "en-US"), .number("finalTimeout", "Final timeout (s)", 8, 1, 30)])
    }
    let locale: String, finalTimeout: Double
    public init(_ p: StageParams) { locale = p.string("locale", "en-US"); finalTimeout = p.double("finalTimeout", 8) }

    public func begin() async throws -> TranscriptionSession {
        // Under the Exact agent (virtual audio) no system alert may open: it takes the foreground, and the driver
        // then refuses taps and the next launch never opens its agent socket.
        if SFSpeechRecognizer.authorizationStatus() == .notDetermined && !LabEnvironment.promptsAllowed {
            throw LabError.unavailable("speech recognition not yet authorized; not asking while driven by an agent")
        }
        let status = await LiveAudio.requestSpeechPermission()
        guard status == .authorized else { throw LabError.unavailable("speech recognition not authorized (\(status.rawValue))") }
        guard let rec = SFSpeechRecognizer(locale: Locale(identifier: locale)), rec.isAvailable else { throw LabError.unavailable("recognizer unavailable for \(locale)") }
        guard rec.supportsOnDeviceRecognition else { throw LabError.unavailable("on-device recognition not supported for \(locale)") }
        return SFSession(rec, finalTimeout)
    }

    final class SFSession: TranscriptionSession, @unchecked Sendable {
        let req = SFSpeechAudioBufferRecognitionRequest()
        var task: SFSpeechRecognitionTask?
        let partials: AsyncStream<String>
        let pcont: AsyncStream<String>.Continuation
        let lock = NSLock()
        var latest = ""
        var final: CheckedContinuation<String, Error>?
        var done = false
        let timeout: Double
        init(_ rec: SFSpeechRecognizer, _ timeout: Double) {
            self.timeout = timeout
            (partials, pcont) = AsyncStream.makeStream(of: String.self)
            req.requiresOnDeviceRecognition = true
            req.shouldReportPartialResults = true
            task = rec.recognitionTask(with: req) { [weak self] result, error in
                guard let self else { return }
                if let r = result {
                    let t = r.bestTranscription.formattedString
                    self.lock.lock(); self.latest = t; self.lock.unlock()
                    if r.isFinal { self.complete(.success(t)) } else { self.pcont.yield(t) }
                } else if error != nil {
                    self.lock.lock(); let t = self.latest; self.lock.unlock()
                    self.complete(.success(t))   // an error after speech: keep what we heard
                }
            }
        }
        func snapshot() -> String { lock.lock(); defer { lock.unlock() }; return latest }
        func complete(_ r: Result<String, Error>) {
            lock.lock()
            if done { lock.unlock(); return }
            done = true; let c = final; final = nil; lock.unlock()
            pcont.finish()
            c?.resume(with: r)
        }
        func append(_ f: AudioFrame) { if let b = pcmBuffer(f) { req.append(b) } }
        func finish() async throws -> String {
            req.endAudio()
            let tmo = timeout
            Task { [weak self] in
                try? await Task.sleep(nanoseconds: UInt64(tmo * 1e9))
                guard let self else { return }
                self.complete(.success(self.snapshot()))
            }
            return try await withCheckedThrowingContinuation { c in
                lock.lock()
                if done { lock.unlock(); c.resume(returning: latest); return }
                final = c; lock.unlock()
            }
        }
        func cancel() { task?.cancel(); complete(.success("")) }
    }
}

// MARK: Transcription: SpeechAnalyzer (iOS 26 / macOS 26)

@available(iOS 26.0, macOS 26.0, *)
public final class SpeechAnalyzerTranscriber: Transcriber, @unchecked Sendable {
    public static let stageID = "apple-speechanalyzer"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Apple SpeechAnalyzer", summary: "The newest on-device API (iOS/macOS 26). May need a one-time language asset download.",
                        params: [.text("locale", "Locale", "en-US")])
    }
    let locale: String
    public init(_ p: StageParams) { locale = p.string("locale", "en-US") }

    public func begin() async throws -> TranscriptionSession {
        let loc = Locale(identifier: locale)
        let tr = SpeechTranscriber(locale: loc, transcriptionOptions: [], reportingOptions: [.volatileResults], attributeOptions: [])
        if let req = try await AssetInventory.assetInstallationRequest(supporting: [tr]) { try await req.downloadAndInstall() }
        let analyzer = SpeechAnalyzer(modules: [tr])
        guard let fmt = await SpeechAnalyzer.bestAvailableAudioFormat(compatibleWith: [tr]) else { throw LabError.unavailable("no analyzer audio format") }
        let (seq, builder) = AsyncStream.makeStream(of: AnalyzerInput.self)
        try await analyzer.start(inputSequence: seq)
        return AnalyzerSession(tr, analyzer, builder, fmt)
    }

    final class AnalyzerSession: TranscriptionSession, @unchecked Sendable {
        let partials: AsyncStream<String>
        let pcont: AsyncStream<String>.Continuation
        let analyzer: SpeechAnalyzer
        let builder: AsyncStream<AnalyzerInput>.Continuation
        let fmt: AVAudioFormat
        var conv: AVAudioConverter?
        var reader: Task<String, Never>?
        init(_ tr: SpeechTranscriber, _ a: SpeechAnalyzer, _ b: AsyncStream<AnalyzerInput>.Continuation, _ f: AVAudioFormat) {
            analyzer = a; builder = b; fmt = f
            (partials, pcont) = AsyncStream.makeStream(of: String.self)
            let pc = pcont
            reader = Task {
                var finals = ""
                var vol = ""
                do {
                    for try await r in tr.results {
                        let t = String(r.text.characters)
                        if r.isFinal { finals += (finals.isEmpty ? "" : " ") + t.trimmingCharacters(in: .whitespaces); vol = ""; pc.yield(finals) }
                        else { vol = t; pc.yield((finals + " " + vol).trimmingCharacters(in: .whitespaces)) }
                    }
                } catch {}
                pc.finish()
                return (finals + " " + vol).trimmingCharacters(in: .whitespaces)
            }
        }
        func append(_ f: AudioFrame) {
            guard let b = pcmBuffer(f) else { return }
            if b.format == fmt { builder.yield(AnalyzerInput(buffer: b)); return }
            if conv == nil { conv = AVAudioConverter(from: b.format, to: fmt) }
            guard let c = conv, let out = AVAudioPCMBuffer(pcmFormat: fmt, frameCapacity: AVAudioFrameCount(Double(b.frameLength) * fmt.sampleRate / b.format.sampleRate) + 16) else { return }
            var given = false
            var err: NSError?
            c.convert(to: out, error: &err) { _, st in
                if given { st.pointee = .noDataNow; return nil }
                given = true; st.pointee = .haveData; return b
            }
            if err == nil, out.frameLength > 0 { builder.yield(AnalyzerInput(buffer: out)) }
        }
        func finish() async throws -> String {
            builder.finish()
            try await analyzer.finalizeAndFinishThroughEndOfInput()
            return await reader?.value ?? ""
        }
        func cancel() { builder.finish(); reader?.cancel() }
    }
}

// MARK: Voice: AVSpeechSynthesizer written to buffers, so audio goes through the same player and reference signal

public final class AVSpeechVoice: Voice, @unchecked Sendable {
    public static let stageID = "avspeech"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "System voice (AVSpeechSynthesizer)", summary: "Synthesised to buffers; the player plays them.",
                        params: [.text("voice", "Voice identifier (blank = default)", ""), .number("rate", "Rate", 0.5, 0.1, 0.9)])
    }
    let voiceID: String, rate: Float
    public init(_ p: StageParams) { voiceID = p.string("voice"); rate = Float(p.double("rate", 0.5)) }

    public func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        let (voiceID, rate) = (voiceID, rate)
        return AsyncThrowingStream { cont in
            let task = Task { @MainActor in
                if LabEnvironment.virtualAudio {
                    // No synthesis under the agent: the first use of the synthesizer on a simulator blocks the main
                    // thread (the drive's socket stops answering). Silence of about the spoken length instead.
                    for await line in text {
                        if Task.isCancelled { break }
                        let secs = max(0.4, Double(line.count) / 14.0)
                        cont.yield(AudioFrame(samples: [Float](repeating: 0, count: Int(secs * 22050)), sampleRate: 22050, t: ProcessInfo.processInfo.systemUptime))
                    }
                    cont.finish(); return
                }
                let synth = AVSpeechSynthesizer()
                let ender = SpeechEndDelegate()
                synth.delegate = ender
                for await line in text {
                    if Task.isCancelled { break }
                    let u = AVSpeechUtterance(string: line)
                    u.rate = rate
                    if !voiceID.isEmpty { u.voice = AVSpeechSynthesisVoice(identifier: voiceID) }
                    else { u.voice = AVSpeechSynthesisVoice(language: "en-US") }
                    // One resume, whichever comes first: the end of the utterance (the delegate's didFinish, or silence after an
                    // empty buffer), the 30 s timer (main), or the task being cancelled. `Once` is the single place that decides.
                    // An empty buffer alone is not the end: the synthesizer sends them between phrases.
                    let once = Once<Void>()
                    let end = UtteranceEnd { once.resume(()) }
                    ender.set(end, for: u)
                    end.arm()               // a line that yields no buffers still ends, after `startQuiet`, not at the 30 s backstop
                    await withTaskCancellationHandler {
                        await withCheckedContinuation { (done: CheckedContinuation<Void, Never>) in
                        once.arm(done)
                        // A synthesizer that never reports its end must not hang the turn.
                        DispatchQueue.main.asyncAfter(deadline: .now() + 30) { once.resume(()) }
                        synth.write(u) { buf in
                            guard let pcm = buf as? AVAudioPCMBuffer else { return }
                            if pcm.frameLength == 0 { end.buffer(empty: true); return }
                            end.buffer(empty: false)
                            let n = Int(pcm.frameLength)
                            var s = [Float](repeating: 0, count: n)
                            if let p = pcm.floatChannelData { for i in 0..<n { s[i] = p[0][i] } }
                            else if let p = pcm.int16ChannelData { for i in 0..<n { s[i] = Float(p[0][i]) / 32768 } }
                            cont.yield(AudioFrame(samples: s, sampleRate: pcm.format.sampleRate, t: ProcessInfo.processInfo.systemUptime))
                        }
                        }
                    } onCancel: { once.resume(()) }
                }
                withExtendedLifetime(ender) {}      // the synthesizer's delegate is weak
                cont.finish()
            }
            cont.onTermination = { _ in task.cancel() }
        }
    }
}

/// When one utterance written to buffers is over. The end is the synthesizer's `didFinish`; an empty buffer is not the end
/// (`AVSpeechSynthesizer.write` sends them between phrases, and ending there cut speech off after about 13 seconds, the defect
/// Mike's speech plugin records). An empty buffer followed by `quiet` seconds of nothing counts as the end only for a system
/// that never calls the delegate: rendering runs far faster than real time, so that quiet means the synthesizer is done.
final class UtteranceEnd: @unchecked Sendable {
    private let lock = NSLock()
    private let quiet: TimeInterval, startQuiet: TimeInterval
    private let onEnd: @Sendable () -> Void
    private var received = 0, done = false
    /// `quiet` (1 s) is Mike's constant, inherited and not yet checked on a device. `startQuiet` is how long a line may produce
    /// no buffer at all before it counts as ended.
    init(quiet: TimeInterval = 1, startQuiet: TimeInterval = 5, onEnd: @escaping @Sendable () -> Void) {
        self.quiet = quiet; self.startQuiet = startQuiet; self.onEnd = onEnd
    }

    func buffer(empty: Bool) {
        lock.lock(); received += 1; lock.unlock()
        guard empty else { return }
        endIfQuiet(after: quiet)
    }
    /// Called when the utterance is handed to the synthesizer: nothing arriving for `startQuiet` is the end.
    func arm() { endIfQuiet(after: startQuiet) }
    private func endIfQuiet(after delay: TimeInterval) {
        lock.lock(); let mark = received; lock.unlock()
        DispatchQueue.global().asyncAfter(deadline: .now() + delay) { [self] in
            lock.lock(); let still = received == mark; lock.unlock()
            if still { didFinish() }
        }
    }
    func didFinish() {
        lock.lock()
        if done { lock.unlock(); return }
        done = true; lock.unlock()
        onEnd()
    }
}

/// Carries the synthesizer's delegate calls to the utterance being written.
final class SpeechEndDelegate: NSObject, AVSpeechSynthesizerDelegate, @unchecked Sendable {
    private let lock = NSLock()
    private var utterance: AVSpeechUtterance?, end: UtteranceEnd?
    func set(_ e: UtteranceEnd, for u: AVSpeechUtterance) { lock.lock(); end = e; utterance = u; lock.unlock() }
    /// Only the utterance being written ends it: a late callback for an earlier one is ignored.
    private func finished(_ u: AVSpeechUtterance) {
        lock.lock(); let e = (utterance === u) ? end : nil; lock.unlock()
        e?.didFinish()
    }
    func speechSynthesizer(_ s: AVSpeechSynthesizer, didFinish u: AVSpeechUtterance) { finished(u) }
    func speechSynthesizer(_ s: AVSpeechSynthesizer, didCancel u: AVSpeechUtterance) { finished(u) }
}

/// Process-wide facts about how the app is being run.
public enum LabEnvironment {
    /// False while driven by the Exact agent: nothing may show a system permission alert.
    nonisolated(unsafe) public static var promptsAllowed = true
    /// True while the audio engine is virtual (the Exact agent): no hardware, no speech synthesis.
    nonisolated(unsafe) public static var virtualAudio = false
}
