import Foundation

// ONE catalog of the on-device models, in the kit, so that voice setup, the engine's stages and the Mac proof all read
// the same list. Before this, the catalog lived in the models module (ModelHub.catalog) and voice setup built a
// ModelStore of its own that nothing had registered into: every id setup asked for failed with "model ... not registered"
// (the first phone walk, 2026-11-11). The shared store (`ModelStore.shared`) is created with this catalog already in it,
// so there is no registration step to forget.
//
// WAYS THIS CAN FAIL (each has a test in VoiceModelsTests):
//  1. Setup's ladder names an id the catalog lacks: a test walks every non-floor candidate of every ladder.
//  2. The catalog names an id twice: a test.
//  3. A pinned model has no usable checksum (not 64 hex characters): a test.

public enum KnownModels {
    /// Where the pinned (HTTP-fetched) models live.
    public static var storeRoot: URL {
        FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("AutoradioLab/models", isDirectory: true)
    }
    /// Where FluidAudio caches its Core ML bundles (its own default).
    public static var fluidRoot: URL {
        FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("FluidAudio/Models", isDirectory: true)
    }

    /// The small language model: quick decisions and the spoken acknowledgement. Qwen3-0.6B Q8_0 from the official GGUF repo.
    public static let qwen3 = ModelSpec(
        id: "qwen3-0.6b-q8_0",
        url: URL(string: "https://huggingface.co/Qwen/Qwen3-0.6B-GGUF/resolve/main/Qwen3-0.6B-Q8_0.gguf")!,
        sha256: "9465e63a22add5354d9bb4b99e90117043c7124007664907259bd16d043bb031",
        bytes: 639_446_688, fileName: "Qwen3-0.6B-Q8_0.gguf")

    // Responder candidates (research note 2026-11-11, docs/design/autoradio-on-device-voice-configurations-2026-11-11.md). Sizes and
    // SHA-256 digests are the Hugging Face API's own file listings (LFS object ids), fetched 2026-11-11.
    /// Qwen3-0.6B Q4_K_M, the first quantisation to try (non-thinking template). Apache-2.0. unsloth's GGUF of the official weights.
    public static let qwen3Q4 = ModelSpec(
        id: "qwen3-0.6b-q4_k_m",
        url: URL(string: "https://huggingface.co/unsloth/Qwen3-0.6B-GGUF/resolve/main/Qwen3-0.6B-Q4_K_M.gguf")!,
        sha256: "ac2d97712095a558e31573f62f466a3f9d93990898b0ec79d7c974c1780d524a",
        bytes: 396_705_472, fileName: "Qwen3-0.6B-Q4_K_M.gguf")
    /// Qwen3-1.7B Q4_K_M: the next size up from 0.6B, same family and template (non-thinking). Apache-2.0. unsloth's GGUF of the official
    /// weights; the digest is the Hugging Face API listing's LFS object id, fetched 2026-11-11.
    public static let qwen317 = ModelSpec(
        id: "qwen3-1.7b-q4_k_m",
        url: URL(string: "https://huggingface.co/unsloth/Qwen3-1.7B-GGUF/resolve/main/Qwen3-1.7B-Q4_K_M.gguf")!,
        sha256: "b139949c5bd74937ad8ed8c8cf3d9ffb1e99c866c823204dc42c0d91fa181897",
        bytes: 1_107_409_472, fileName: "Qwen3-1.7B-Q4_K_M.gguf")
    /// Qwen3.5-0.8B Q4_K_M, a trial responder (non-thinking by default). Apache-2.0. Whether llama.cpp b12259 runs its architecture is
    /// what the check on the device says.
    public static let qwen35 = ModelSpec(
        id: "qwen3.5-0.8b-q4_k_m",
        url: URL(string: "https://huggingface.co/unsloth/Qwen3.5-0.8B-GGUF/resolve/main/Qwen3.5-0.8B-Q4_K_M.gguf")!,
        sha256: "bd258782e35f7f458f8aced1adc053e6e92e89bc735ba3be89d38a06121dc517",
        bytes: 532_517_120, fileName: "Qwen3.5-0.8B-Q4_K_M.gguf")
    /// LFM3-1.7B QAD-Q4_0, a trial responder. LFM Open License v1.0: commercial use at or above US$10M annual revenue (the legal
    /// entity and its affiliates) needs a separate licence from Liquid AI; read section 5 before any shipping decision. Its thinking
    /// switch "can be inconsistent" (vendor card). Only chosen if its measured decode is fast enough on the device.
    public static let lfm3Text = ModelSpec(
        id: "lfm3-1.7b-qad-q4_0",
        url: URL(string: "https://huggingface.co/LiquidAI/LFM3-1.7B-GGUF/resolve/main/LFM3-1.7B-QAD-Q4_0.gguf")!,
        sha256: "43a7d09a0e9cba91b63eb7052488b9564569bddfd65754d2e426465e572a7452",
        bytes: 1_049_615_680, fileName: "LFM3-1.7B-QAD-Q4_0.gguf")

    /// Models fetched by us over HTTP, pinned by SHA-256.
    public static var pinned: [ModelSpec] { [qwen3, qwen3Q4, qwen317, qwen35, lfm3Text] + extraGGUF.map(\.spec) }

    /// More GGUF quantisations of the responder families, for the configurator (the candidate table, ModelCandidates.swift). Sizes and
    /// SHA-256 digests are the Hugging Face API file listings' (LFS object ids) fetched 2026-11-11; names are the repos' own, not guessed.
    struct ExtraGGUF { var spec: ModelSpec; var repo: String; var license: String }
    static func gguf(_ id: String, repo: String, file: String, sha: String, bytes: Int64, license: String) -> ExtraGGUF {
        ExtraGGUF(spec: ModelSpec(id: id, url: URL(string: "https://huggingface.co/\(repo)/resolve/main/\(file)")!, sha256: sha, bytes: bytes, fileName: file), repo: repo, license: license)
    }
    static let lfmLicense = "LFM Open License v1.0 (commercial use at or above US$10M annual revenue needs a separate licence from Liquid AI)"
    static let extraGGUF: [ExtraGGUF] = [
        gguf("lfm3-1.7b-q4_k_m", repo: "LiquidAI/LFM3-1.7B-GGUF", file: "LFM3-1.7B-Q4_K_M.gguf", sha: "3793e1e6e2266919d6e015287221b41100175a98eddaba92e111aed3b05c93c4", bytes: 1_099_627_808, license: lfmLicense),
        gguf("lfm3-1.7b-q5_k_m", repo: "LiquidAI/LFM3-1.7B-GGUF", file: "LFM3-1.7B-Q5_K_M.gguf", sha: "41e51fb0be5f472d323055f2734d986d5a2429570973508fa6322f3f07f6d7f3", bytes: 1_258_663_200, license: lfmLicense),
        gguf("lfm3-1.7b-q6_k", repo: "LiquidAI/LFM3-1.7B-GGUF", file: "LFM3-1.7B-Q6_K.gguf", sha: "54e1bd947c77167823d45bf495f1c3e2d226ca7fb242f83cfa141ee3a6ab55c6", bytes: 1_427_638_304, license: lfmLicense),
        gguf("lfm3-1.7b-q8_0", repo: "LiquidAI/LFM3-1.7B-GGUF", file: "LFM3-1.7B-Q8_0.gguf", sha: "8f6b24182258205e9e01e21f1f24a1f208c1e8115578825d202ce39976af20b6", bytes: 1_846_238_496, license: lfmLicense),
        gguf("lfm3-1.7b-nvfp4", repo: "LiquidAI/LFM3-1.7B-GGUF", file: "LFM3-1.7B-NVFP4.gguf", sha: "462c2379586a45b30fe3dbfa30e6ff08cbb440734d6934689e24e4fe59a67bec", bytes: 1_280_019_776, license: lfmLicense),
        gguf("qwen3-1.7b-q6_k", repo: "unsloth/Qwen3-1.7B-GGUF", file: "Qwen3-1.7B-Q6_K.gguf", sha: "6a9cadec4883df6f3efcf337103479dcc6a3efa9f5f8cc64a904dba57808207a", bytes: 1_417_755_200, license: "Apache-2.0"),
        gguf("qwen3-1.7b-q8_0", repo: "unsloth/Qwen3-1.7B-GGUF", file: "Qwen3-1.7B-Q8_0.gguf", sha: "0becaa825564295d82e9af4d008bca5f8b7f5f73bf1c6a0b58f7c53ef26b47fd", bytes: 1_834_426_944, license: "Apache-2.0"),
        gguf("qwen3.5-0.8b-q8_0", repo: "unsloth/Qwen3.5-0.8B-GGUF", file: "Qwen3.5-0.8B-Q8_0.gguf", sha: "0ad885ffd4bb022fc4f0d33a3308fa108ef8613159d3b3a67e23abca056b7a6c", bytes: 811_843_840, license: "Apache-2.0"),
        gguf("qwen3.5-2b-q4_k_m", repo: "unsloth/Qwen3.5-2B-GGUF", file: "Qwen3.5-2B-Q4_K_M.gguf", sha: "aaf42c8b7c3cab2bf3d69c355048d4a0ee9973d48f16c731c0520ee914699223", bytes: 1_280_835_840, license: "Apache-2.0"),
        gguf("qwen3.5-2b-q8_0", repo: "unsloth/Qwen3.5-2B-GGUF", file: "Qwen3.5-2B-Q8_0.gguf", sha: "1b04acba824817554f4ce23639bc8495ff70453b8fcb047900c731521021f2c1", bytes: 2_012_012_800, license: "Apache-2.0"),
    ]

    /// Every model the app can load, with the facts from the model cards (sizes are the cards' figures, not measurements).
    /// Library-managed ones are fetched, checksummed (Hugging Face LFS object ids) and compiled by FluidAudio itself.
    public static var managed: [ManagedModelSpec] {
        let c = fluidRoot
        return base + extraGGUF.map { ManagedModelSpec(id: $0.spec.id, label: $0.spec.fileName, source: "hf:" + $0.repo, approxBytes: $0.spec.bytes, license: $0.license, cachePath: storeRoot) }
    }
    private static var base: [ManagedModelSpec] {
        let c = fluidRoot
        return [
            ManagedModelSpec(id: "silero-vad-v6.2.1", label: "Silero VAD v6.2.1 (256 ms, Core ML)", source: "hf:FluidInference/silero-vad-coreml",
                             approxBytes: 2_000_000, license: "MIT (model); FluidAudio Apache-2.0", cachePath: c, cacheFolders: ["silero-vad"]),
            ManagedModelSpec(id: "parakeet-eou-120m", label: "Parakeet EOU 120M (streaming, Core ML)", source: "hf:FluidInference/parakeet-realtime-eou-120m-coreml",
                             approxBytes: 250_000_000, license: "NVIDIA Open Model License (model); FluidAudio Apache-2.0", cachePath: c, cacheFolders: ["parakeet-eou-streaming"]),
            ManagedModelSpec(id: "parakeet-tdt-0.6b-v2", label: "Parakeet TDT 0.6B v2 (English, Core ML)", source: "hf:FluidInference/parakeet-tdt-0.6b-v2-coreml",
                             approxBytes: 480_000_000, license: "CC-BY-4.0 (NVIDIA model); FluidAudio Apache-2.0", cachePath: c, cacheFolders: ["parakeet-tdt-0.6b-v2", "parakeet-tdt-0.6b-v2-coreml"]),
            ManagedModelSpec(id: "kokoro-82m-ane", label: "Kokoro 82M ANE v3 (Core ML)", source: "hf:FluidInference/kokoro-82m-coreml",
                             approxBytes: 350_000_000, license: "Apache-2.0 (model); FluidAudio Apache-2.0", cachePath: c, cacheFolders: ["kokoro-82m-coreml", "kokoro"]),
            ManagedModelSpec(id: "pocket-tts", label: "PocketTTS 100M (Kyutai, Core ML)", source: "hf:FluidInference/pocket-tts-coreml",
                             approxBytes: 300_000_000, license: "CC-BY-4.0 (Kyutai model); FluidAudio Apache-2.0", cachePath: c, cacheFolders: ["pocket-tts"]),
            ManagedModelSpec(id: "lfm3-audio-2b", label: "LFM3-Audio-2B (Core ML, ASR + TTS)", source: "hf:FluidInference/lfm3-audio-2b-coreml",
                             approxBytes: 2_900_000_000, license: "LFM Open License (read it: commercial use limited by revenue)", cachePath: c),
            ManagedModelSpec(id: qwen3.id, label: "Qwen3-0.6B Q8_0 (llama.cpp GGUF)", source: "hf:Qwen/Qwen3-0.6B-GGUF",
                             approxBytes: qwen3.bytes, license: "Apache-2.0 (model); llama.cpp MIT", cachePath: storeRoot),
            ManagedModelSpec(id: qwen3Q4.id, label: "Qwen3-0.6B Q4_K_M (llama.cpp GGUF)", source: "hf:unsloth/Qwen3-0.6B-GGUF",
                             approxBytes: qwen3Q4.bytes, license: "Apache-2.0 (model); llama.cpp MIT", cachePath: storeRoot),
            ManagedModelSpec(id: qwen317.id, label: "Qwen3-1.7B Q4_K_M (llama.cpp GGUF)", source: "hf:unsloth/Qwen3-1.7B-GGUF",
                             approxBytes: qwen317.bytes, license: "Apache-2.0 (model); llama.cpp MIT", cachePath: storeRoot),
            ManagedModelSpec(id: qwen35.id, label: "Qwen3.5-0.8B Q4_K_M (llama.cpp GGUF)", source: "hf:unsloth/Qwen3.5-0.8B-GGUF",
                             approxBytes: qwen35.bytes, license: "Apache-2.0 (model); llama.cpp MIT", cachePath: storeRoot),
            ManagedModelSpec(id: lfm3Text.id, label: "LFM3-1.7B QAD-Q4_0 (llama.cpp GGUF)", source: "hf:LiquidAI/LFM3-1.7B-GGUF",
                             approxBytes: lfm3Text.bytes, license: "LFM Open License v1.0 (commercial use limited by revenue: read it)", cachePath: storeRoot),
        ]
    }

    /// Every id the app knows, library-managed or pinned.
    public static var ids: [String] { managed.map(\.id) }
    public static func contains(_ id: String) -> Bool { ids.contains(id) }
    public static func isPinned(_ id: String) -> Bool { pinned.contains { $0.id == id } }
}

/// How the app module, which links the third-party model libraries, plugs into the kit: it fetches, loads and checks the
/// library-managed models. The kit cannot import those libraries (an Exact package is compiled with its own files only),
/// so the app module's registrar installs this at start. With none installed, setup says so plainly instead of pretending.
public protocol ModelProvider: Sendable {
    /// Fetch (checksummed by the provider's library), compile and load the model. `onBytes` receives bytes on disk so far. `role` is
    /// what the model is for ("decision", "responder", or "" for an audio model): the same file may serve two roles, each in its own
    /// context.
    func fetch(_ id: String, role: String, onBytes: @escaping @Sendable (Int64) -> Void) async throws
    /// Run the model on this phone's bundled fixture and measure it.
    func check(_ id: String, role: String) async throws -> SetupCheck
    /// Drop a loaded model from memory (the configurator unloads a rejected configuration).
    func unload(_ id: String, role: String) async
    /// With every model of a configuration loaded (role -> candidate id), run them and measure: memory left, the responder's speeds with the
    /// page cache warm, first sound, recogniser speed, and the temperature after `rounds` rounds of the same work.
    func measureConfiguration(_ picks: [String: String], rounds: Int) async throws -> ConfigMeasurement
}
public extension ModelProvider {
    func unload(_ id: String, role: String) async {}
    func measureConfiguration(_ picks: [String: String], rounds: Int) async throws -> ConfigMeasurement { throw LabError.unavailable(ModelProviders.missingNote) }
}

/// Which responder model setup chose (read at start, written when setup finishes).
public enum ResponderChoice {
    public static let defaultID = "qwen3-0.6b-q8_0"
    static let key = "AutoradioResponderModel"
    public static var current: String {
        get { UserDefaults.standard.string(forKey: key).flatMap { KnownModels.isPinned($0) ? $0 : nil } ?? defaultID }
        set { UserDefaults.standard.set(newValue, forKey: key) }
    }
}

/// The device's own readings (what setup uses instead of guessing from physical memory).
public enum DeviceReading {
    #if os(iOS)
    @_silgen_name("os_proc_available_memory") private static func osProcAvailableMemory() -> Int
    #endif
    /// Memory this process may still use before the system ends it, in MB (iOS: `os_proc_available_memory()`); nil where there is no such call.
    public static func availableMemoryMB() -> Double? {
        #if os(iOS)
        let b = osProcAvailableMemory()
        return b > 0 ? Double(b) / 1_048_576 : nil
        #else
        return nil
        #endif
    }
    public static func thermal() -> String {
        switch ProcessInfo.processInfo.thermalState { case .nominal: return "nominal"; case .fair: return "fair"; case .serious: return "serious"; case .critical: return "critical"; @unknown default: return "unknown" }
    }
}

public enum ModelProviders {
    private static let box = Locked<(any ModelProvider)?>(nil)
    public static var current: (any ModelProvider)? {
        get { box.value }
        set { box.value = newValue }
    }
    /// What the user reads when this build links no model libraries.
    public static let missingNote = "This build has no on-device model libraries, so this model cannot be fetched or checked here."
}
