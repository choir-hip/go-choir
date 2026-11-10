import Foundation
import AVFoundation

/// A chunk of mono Float32 PCM. `t` is the time of its first sample on the host (or virtual) clock.
public struct AudioFrame: Sendable {
    public var samples: [Float]
    public var sampleRate: Double
    public var t: Double
    public init(samples: [Float], sampleRate: Double, t: Double) { self.samples = samples; self.sampleRate = sampleRate; self.t = t }
    public var duration: Double { Double(samples.count) / sampleRate }
    public var rmsDb: Double {
        guard !samples.isEmpty else { return -120 }
        var s: Float = 0
        for x in samples { s += x * x }
        return 20 * log10(Double(max((s / Float(samples.count)).squareRoot(), 1e-6)))
    }
}

public let busSampleRate: Double = 16_000

/// Delivers mic frames and, separately, the playback reference signal. Frames are not events.
/// Sources push; stages and recorders subscribe. Every subscriber gets every frame.
public final class AudioBus: @unchecked Sendable {
    private let lock = NSLock()
    private var micSubs: [UUID: AsyncStream<AudioFrame>.Continuation] = [:]
    private var refSubs: [UUID: AsyncStream<AudioFrame>.Continuation] = [:]
    public init() {}

    public func mic() -> AsyncStream<AudioFrame> { subscribe(\.micSubs) }
    public func reference() -> AsyncStream<AudioFrame> { subscribe(\.refSubs) }

    private func subscribe(_ kp: ReferenceWritableKeyPath<AudioBus, [UUID: AsyncStream<AudioFrame>.Continuation]>) -> AsyncStream<AudioFrame> {
        AsyncStream(bufferingPolicy: .unbounded) { cont in
            let id = UUID()
            lock.lock(); self[keyPath: kp][id] = cont; lock.unlock()
            cont.onTermination = { [weak self] _ in
                guard let self else { return }
                self.lock.lock(); self[keyPath: kp][id] = nil; self.lock.unlock()
            }
        }
    }
    /// Lockstep consumer (replay): when set, `deliverMic` awaits it so time never runs ahead of the engine.
    public var syncMic: (@Sendable (AudioFrame) async -> Void)?
    public func deliverMic(_ f: AudioFrame) async {
        if let h = syncMic { await h(f) }
        pushMic(f)
    }
    public func pushMic(_ f: AudioFrame) { lock.lock(); let c = Array(micSubs.values); lock.unlock(); for x in c { x.yield(f) } }
    public func pushReference(_ f: AudioFrame) { lock.lock(); let c = Array(refSubs.values); lock.unlock(); for x in c { x.yield(f) } }
    /// Ends all streams (end of a file, or the app stopping).
    public func finish() {
        lock.lock(); let c = Array(micSubs.values) + Array(refSubs.values); micSubs = [:]; refSubs = [:]; lock.unlock()
        for x in c { x.finish() }
    }
}

// MARK: Reading audio files

public enum AudioFiles {
    /// Read any audio file as mono Float32 at `rate`.
    public static func readMono(_ url: URL, rate: Double = busSampleRate) throws -> [Float] {
        let file = try AVAudioFile(forReading: url)
        let inFmt = file.processingFormat
        let outFmt = AVAudioFormat(commonFormat: .pcmFormatFloat32, sampleRate: rate, channels: 1, interleaved: false)!
        guard let conv = AVAudioConverter(from: inFmt, to: outFmt) else { throw LabError.badInput("cannot convert \(url.lastPathComponent)") }
        var out: [Float] = []
        let inBuf = AVAudioPCMBuffer(pcmFormat: inFmt, frameCapacity: 8192)!
        let ratio = rate / inFmt.sampleRate
        var done = false
        while !done {
            let outBuf = AVAudioPCMBuffer(pcmFormat: outFmt, frameCapacity: AVAudioFrameCount(8192 * ratio) + 64)!
            var err: NSError?
            let status = conv.convert(to: outBuf, error: &err) { _, st in
                do {
                    try file.read(into: inBuf, frameCount: 8192)
                } catch { st.pointee = .endOfStream; return nil }
                if inBuf.frameLength == 0 { st.pointee = .endOfStream; return nil }
                st.pointee = .haveData
                return inBuf
            }
            if let err { throw err }
            if outBuf.frameLength > 0, let p = outBuf.floatChannelData?[0] {
                out.append(contentsOf: UnsafeBufferPointer(start: p, count: Int(outBuf.frameLength)))
            }
            if status == .endOfStream || status == .error { done = true }
        }
        return out
    }

    /// Write mono Float32 to a CAF file.
    public static func writeMono(_ samples: [Float], rate: Double = busSampleRate, to url: URL) throws {
        let fmt = AVAudioFormat(commonFormat: .pcmFormatFloat32, sampleRate: rate, channels: 1, interleaved: false)!
        let file = try AVAudioFile(forWriting: url, settings: fmt.settings, commonFormat: .pcmFormatFloat32, interleaved: false)
        guard !samples.isEmpty else { return }
        let buf = AVAudioPCMBuffer(pcmFormat: fmt, frameCapacity: AVAudioFrameCount(samples.count))!
        buf.frameLength = AVAudioFrameCount(samples.count)
        samples.withUnsafeBufferPointer { buf.floatChannelData![0].update(from: $0.baseAddress!, count: samples.count) }
        try file.write(from: buf)
    }
}

// MARK: Sources

/// Anything that pushes mic (and optionally reference) frames onto a bus.
public protocol AudioSource: AnyObject, Sendable {
    func start(into bus: AudioBus, clock: LabClock) async throws
    func stop() async
}

/// A scheduled input from the listener's hands (button), kept with the turn so replay can press it again.
public struct ScheduledInput: Codable, Sendable, Equatable {
    public enum Kind: String, Codable, Sendable { case buttonDown, buttonUp }
    public var t: Double      // seconds from the start of the audio
    public var kind: Kind
    public init(t: Double, kind: Kind) { self.t = t; self.kind = kind }
}

/// Plays a kept turn into the bus with a virtual clock. `realTime == false` goes as fast as the consumers allow.
public final class FileAudioSource: AudioSource, @unchecked Sendable {
    public let mic: [Float]
    public let reference: [Float]?
    public let inputs: [ScheduledInput]
    public let realTime: Bool
    public let frameSeconds: Double
    public let tail: Double
    /// Called at each input's time (buttons) before the frames after that time are pushed.
    public var onInput: (@Sendable (ScheduledInput) async -> Void)?
    /// Called after each frame is pushed; replay uses it to let the engine drain before time moves on.
    public var afterFrame: (@Sendable () async -> Void)?
    private var task: Task<Void, Never>?

    public init(mic: [Float], reference: [Float]? = nil, inputs: [ScheduledInput] = [], realTime: Bool = false,
                frameSeconds: Double = 0.032, tailSeconds: Double = 2.0) {
        self.mic = mic; self.reference = reference; self.inputs = inputs.sorted { $0.t < $1.t }
        self.realTime = realTime; self.frameSeconds = frameSeconds; self.tail = tailSeconds
    }
    public var duration: Double { Double(mic.count) / busSampleRate }

    public func start(into bus: AudioBus, clock: LabClock) async throws {
        let n = Int(frameSeconds * busSampleRate)
        let base = clock.now
        task = Task { [self] in
            var i = 0, nextInput = 0
            let total = mic.count + Int(tail * busSampleRate)  // trailing silence lets the end-of-turn rule fire
            let wall0 = ProcessInfo.processInfo.systemUptime
            while i < total && !Task.isCancelled {
                let t = Double(i) / busSampleRate
                while nextInput < inputs.count && inputs[nextInput].t <= t {
                    clock.setVirtual(base + inputs[nextInput].t)
                    await onInput?(inputs[nextInput]); nextInput += 1
                }
                var chunk = [Float](repeating: 0, count: n)
                if i < mic.count { let e = min(i + n, mic.count); for k in i..<e { chunk[k - i] = mic[k] } }
                if realTime {
                    let wait = t - (ProcessInfo.processInfo.systemUptime - wall0)
                    if wait > 0 { try? await Task.sleep(nanoseconds: UInt64(wait * 1e9)) }
                }
                clock.setVirtual(base + t)
                if let r = reference, i < r.count {
                    var rc = [Float](repeating: 0, count: n)
                    let e = min(i + n, r.count); for k in i..<e { rc[k - i] = r[k] }
                    bus.pushReference(AudioFrame(samples: rc, sampleRate: busSampleRate, t: base + t))
                }
                await bus.deliverMic(AudioFrame(samples: chunk, sampleRate: busSampleRate, t: base + t))
                await afterFrame?()
                i += n
            }
            bus.finish()
        }
    }
    public func stop() async { task?.cancel() }
    /// Wait for the whole file to be delivered.
    public func finished() async { await task?.value }
}
