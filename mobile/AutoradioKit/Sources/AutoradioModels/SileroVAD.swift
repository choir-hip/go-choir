import Foundation
import FluidAudio
import AutoradioKit

/// Silero VAD v6.2.1 as Core ML through FluidAudio (256 ms hops, 4096 samples at 16 kHz). Start and end are decided by
/// Silero-style hysteresis over the model's probability; the hop is the time resolution of both events.
public final class SileroVAD: SpeechBounds, @unchecked Sendable {
    public static let stageID = "silero-vad"
    public static var descriptor: StageDescriptor {
        StageDescriptor(
            displayName: "Silero VAD (Core ML)",
            summary: "Neural speech bounds from FluidAudio's Silero v6.2.1. Decides in 256 ms hops; downloads about 2 MB on first use.",
            params: [.number("threshold", "Speech probability to start", 0.6, 0.2, 0.95),
                     .number("minSilenceSeconds", "End after silence (s)", 0.6, 0.2, 5),
                     .number("speechPaddingSeconds", "Padding (s)", 0.1, 0, 0.5)],
            needsModels: ["silero-vad-v6.2.1"])
    }
    static let slot = ModelSlot<VadManager>("silero-vad-v6.2.1")

    let threshold: Float, minSilence: Double, padding: Double
    private let lock = NSLock()
    private var buffer: [Float] = []
    private var state: VadStreamState?
    private var t0: Double?
    private var speaking = false
    private var startT = 0.0

    public init(_ p: StageParams) {
        threshold = Float(p.double("threshold", 0.6)); minSilence = p.double("minSilenceSeconds", 0.6); padding = p.double("speechPaddingSeconds", 0.1)
    }

    public func reset() async {
        lock.lock(); buffer = []; state = nil; t0 = nil; speaking = false; lock.unlock()
    }
    public func speechSeconds(now: Double) async -> Double {
        lock.lock(); defer { lock.unlock() }
        return speaking ? max(0, now - startT) : 0
    }

    public func feed(_ f: AudioFrame) async -> BoundsUpdate {
        let manager: VadManager
        do {
            manager = try await Self.slot.get {
                let m = try await VadManager(config: VadConfig(defaultThreshold: 0.6, computeUnits: .cpuAndNeuralEngine))
                return (m, {})
            }
        } catch { return .none }
        let s = samples16k(f)
        lock.lock()
        if t0 == nil { t0 = f.t }
        buffer.append(contentsOf: s)
        var chunks: [[Float]] = []
        while buffer.count >= VadManager.chunkSize { chunks.append(Array(buffer.prefix(VadManager.chunkSize))); buffer.removeFirst(VadManager.chunkSize) }
        var st = state
        let origin = t0 ?? f.t
        lock.unlock()
        if chunks.isEmpty { return .none }
        if st == nil { st = await manager.makeStreamState() }
        var cfg = VadSegmentationConfig.default
        cfg.minSilenceDuration = minSilence; cfg.speechPadding = padding
        cfg.negativeThreshold = max(0, threshold - 0.15)
        var update = BoundsUpdate.none
        for c in chunks {
            guard let r = try? await manager.processStreamingChunk(c, state: st!, config: cfg, returnSeconds: false) else { continue }
            st = r.state
            if let e = r.event {
                let at = origin + Double(e.sampleIndex) / 16000
                lock.lock()
                switch e.kind {
                case .speechStart: if !speaking { speaking = true; startT = at; update = .speechStarted(at: at) }
                case .speechEnd: if speaking { speaking = false; update = .speechEnded(at: at, seconds: max(0, at - startT - padding)) }
                }
                lock.unlock()
            }
        }
        lock.lock(); state = st; lock.unlock()
        return update
    }

    /// Per-hop probabilities for a whole 16 kHz signal (used by the proof; not part of the stage protocol).
    public static func probabilities(_ samples: [Float]) async throws -> (probs: [Float], inferenceMs: Double) {
        let m = try await slot.get { (try await VadManager(config: .default), {}) }
        let r = try await m.process(samples)
        return (r.map(\.probability), r.reduce(0) { $0 + $1.processingTime } * 1000)
    }
}
