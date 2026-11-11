import Foundation
import AVFoundation
import AutoradioKit

/// The on-device models of the lab. Nothing is bundled in the app: every model is fetched on first use (FluidAudio
/// fetches its own Core ML bundles from Hugging Face; the llama.cpp GGUF goes through the ModelStore, pinned by SHA-256),
/// and every load is measured (bytes fetched, load time, resident memory before and after, memory after unload).
public enum ModelHub {
    /// Where FluidAudio caches its Core ML bundles (its own default).
    public static var fluidRoot: URL { KnownModels.fluidRoot }
    /// The one store of the app (the kit's): voice setup, these stages and the proof all use it, with the whole catalog in it.
    public static var store: ModelStore { .shared }

    /// Every model the lab can load: the kit's one catalog.
    public static var catalog: [ManagedModelSpec] { KnownModels.managed }

    /// Register every model stage into `registry` and the catalog into the store.
    public static func register(into registry: StageRegistry) {
        registry.register(.speechBounds, SileroVAD.self) { SileroVAD($0) }
        registry.register(.transcriber, ParakeetEOUTranscriber.self) { ParakeetEOUTranscriber($0) }
        registry.register(.transcriber, ParakeetTDTTranscriber.self) { ParakeetTDTTranscriber($0) }
        registry.register(.voice, KokoroVoice.self) { KokoroVoice($0) }
        registry.register(.voice, PocketTTSVoice.self) { PocketTTSVoice($0) }
        registry.register(.transcriber, LFM3Transcriber.self) { LFM3Transcriber($0) }
        registry.register(.voice, LFM3Voice.self) { LFM3Voice($0) }
        registerLlamaStages(registry)
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
        ModelProviders.current = HubModelProvider.shared          // voice setup fetches and checks through this
        #if canImport(llama)
        LocalLanguage.model = LlamaLocalModel.shared              // the acknowledgement is written by this
        #endif
        ModelProof.runFromLaunchArguments()
        fputs("autoradio models: registered \(ModelHub.catalog.count) models, \(r.all.count) stages\n", stderr)
    }
}
