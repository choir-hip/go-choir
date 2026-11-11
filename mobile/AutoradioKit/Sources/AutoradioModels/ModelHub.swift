import Foundation
import AVFoundation
import AutoradioKit

/// The on-device models of the lab. Nothing is bundled in the app: every model is fetched on first use (FluidAudio
/// fetches its own Core ML bundles from Hugging Face; the llama.cpp GGUF goes through the ModelStore, pinned by SHA-256),
/// and every load is measured (bytes fetched, load time, resident memory before and after, memory after unload).
public enum ModelHub {
    /// Where FluidAudio caches its Core ML bundles (its own default).
    public static var fluidRoot: URL {
        FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("FluidAudio/Models", isDirectory: true)
    }
    public static let store: ModelStore = {
        let root = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("AutoradioLab/models", isDirectory: true)
        return ModelStore(root: root)
    }()

    /// Every model the lab can load, with the facts from the model cards (sizes are the cards' figures, not measurements).
    public static var catalog: [ManagedModelSpec] {
        let c = fluidRoot
        return [
            ManagedModelSpec(id: "silero-vad-v6.2.1", label: "Silero VAD v6.2.1 (256 ms, Core ML)", source: "hf:FluidInference/silero-vad-coreml",
                             approxBytes: 2_000_000, license: "MIT (model); FluidAudio Apache-2.0", cachePath: c),
            ManagedModelSpec(id: "parakeet-eou-120m", label: "Parakeet EOU 120M (streaming, Core ML)", source: "hf:FluidInference/parakeet-realtime-eou-120m-coreml",
                             approxBytes: 250_000_000, license: "NVIDIA Open Model License (model); FluidAudio Apache-2.0", cachePath: c),
            ManagedModelSpec(id: "parakeet-tdt-0.6b-v2", label: "Parakeet TDT 0.6B v2 (English, Core ML)", source: "hf:FluidInference/parakeet-tdt-0.6b-v2-coreml",
                             approxBytes: 480_000_000, license: "CC-BY-4.0 (NVIDIA model); FluidAudio Apache-2.0", cachePath: c),
            ManagedModelSpec(id: "kokoro-82m-ane", label: "Kokoro 82M ANE v3 (Core ML)", source: "hf:FluidInference/kokoro-82m-coreml",
                             approxBytes: 350_000_000, license: "Apache-2.0 (model); FluidAudio Apache-2.0", cachePath: c),
            ManagedModelSpec(id: "pocket-tts", label: "PocketTTS 100M (Kyutai, Core ML)", source: "hf:FluidInference/pocket-tts-coreml",
                             approxBytes: 300_000_000, license: "CC-BY-4.0 (Kyutai model); FluidAudio Apache-2.0", cachePath: c),
            ManagedModelSpec(id: "lfm3-audio-2b", label: "LFM3-Audio-2B (Core ML, ASR + TTS)", source: "hf:FluidInference/lfm3-audio-2b-coreml",
                             approxBytes: 2_900_000_000, license: "LFM Open License (read it: commercial use limited by revenue)", cachePath: c),
        ] + LlamaCatalog.specs
    }

    /// Register every model stage into `registry` and the catalog into the store.
    public static func register(into registry: StageRegistry) {
        registry.register(.speechBounds, SileroVAD.self) { SileroVAD($0) }
        registry.register(.transcriber, ParakeetEOUTranscriber.self) { ParakeetEOUTranscriber($0) }
        registry.register(.transcriber, ParakeetTDTTranscriber.self) { ParakeetTDTTranscriber($0) }
        registry.register(.voice, KokoroVoice.self) { KokoroVoice($0) }
        registry.register(.voice, PocketTTSVoice.self) { PocketTTSVoice($0) }
        registry.register(.transcriber, LFM3Transcriber.self) { LFM3Transcriber($0) }
        registry.register(.holder, LFM3Holder.self) { LFM3Holder($0) }
        registry.register(.voice, LFM3Voice.self) { LFM3Voice($0) }
        registerLlamaStages(registry)
        let specs = catalog
        Task { for s in specs { await store.register(managed: s) } }
    }
}

// MARK: Sharing a loaded model between stage instances

/// One loaded model per process. A stage asks for it; the first ask loads it (measured), later asks reuse it, and
/// `ModelStore.unload` drops it.
final class ModelSlot<T>: @unchecked Sendable {
    let id: String
    private let lock = NSLock()
    private var value: T?
    private var loading: Task<T, Error>?
    private var cleanup: (@Sendable () async -> Void)?
    init(_ id: String) { self.id = id }

    func get(_ make: @escaping @Sendable () async throws -> (T, @Sendable () async -> Void)) async throws -> T {
        lock.lock()
        if let v = value { lock.unlock(); return v }
        if let t = loading { lock.unlock(); return try await t.value }
        let id = self.id
        let t = Task<T, Error> { [self] in
            // The slot forgets this load when the load ends, not when one waiting caller leaves: a caller that times out or is
            // cancelled must not let a second caller start a second copy of the same model beside the first.
            defer { lock.lock(); loading = nil; lock.unlock() }
            var made: T?
            _ = try await ModelHub.store.measuredLoad(id) {
                let (v, c) = try await make()
                made = v
                lock.lock(); self.value = v; self.cleanup = c; lock.unlock()
                return { [self] in self.drop() }
            }
            return made!
        }
        loading = t
        lock.unlock()
        return try await t.value
    }
    func drop() {
        lock.lock(); let c = cleanup; value = nil; cleanup = nil; lock.unlock()
        if let c { Task { await c() } }
    }
    var isLoaded: Bool { lock.lock(); defer { lock.unlock() }; return value != nil }
}

// MARK: Audio helpers

/// Linear resample to 16 kHz mono; every model here reads 16 kHz.
func samples16k(_ f: AudioFrame) -> [Float] {
    if Int(f.sampleRate) == 16000 || f.samples.isEmpty { return f.samples }
    guard f.sampleRate.isFinite, f.sampleRate > 0 else { return [] }     // a corrupt rate would divide by zero below
    let ratio = f.sampleRate / 16000
    let n = Int(Double(f.samples.count) / ratio)
    var out = [Float](repeating: 0, count: n)
    for i in 0..<n {
        let p = Double(i) * ratio, j = Int(p), fr = Float(p - Double(j))
        let a = f.samples[min(j, f.samples.count - 1)], b = f.samples[min(j + 1, f.samples.count - 1)]
        out[i] = a + (b - a) * fr
    }
    return out
}

func pcm(_ samples: [Float], rate: Double) -> AVAudioPCMBuffer? {
    guard rate.isFinite, rate > 0, !samples.isEmpty,
          let fmt = AVAudioFormat(commonFormat: .pcmFormatFloat32, sampleRate: rate, channels: 1, interleaved: false),
          let b = AVAudioPCMBuffer(pcmFormat: fmt, frameCapacity: AVAudioFrameCount(samples.count)) else { return nil }
    b.frameLength = AVAudioFrameCount(samples.count)
    samples.withUnsafeBufferPointer { if let base = $0.baseAddress, let ch = b.floatChannelData { ch[0].update(from: base, count: samples.count) } }
    return b
}

func nowMs() -> Double { Double(DispatchTime.now().uptimeNanoseconds) / 1e6 }

/// Hands `Exact`'s package module the registrar by name: the kit looks up this class with `NSClassFromString`
/// (the app module and the package module are separate Swift modules in one dylib, and neither imports the other's registrar).
@objc(AutoradioModelsRegistrar)
public final class AutoradioModelsRegistrar: NSObject {
    @objc public static func register(_ registry: AnyObject) {
        guard let r = registry as? StageRegistry else { return }
        ModelHub.register(into: r)
        ModelProof.runFromLaunchArguments()
        fputs("autoradio models: registered \(ModelHub.catalog.count) models, \(r.all.count) stages\n", stderr)
    }
}
