import Foundation

// THE CANDIDATE TABLE: every model the configurator may choose for each role of the voice pipeline, as DATA (owner, 2026-11-11: "I want all
// the models downloadable. None ship with the app. The point is to build a configurator that selects the best models for the user's
// specific device"). Nothing here is weights: a candidate names a source, a pinned digest and sizes; the bytes come through the provider.
//
// WAYS THIS CAN GO WRONG (each has a test in ConfiguratorTests):
//  1. A candidate names a model the catalog (KnownModels) does not know, or a pinned digest that is not 64 hex characters.
//  2. Two candidates of one role share an id, or a role has no floor (something that needs no download and always exists).
//  3. Quality ranks are code, not data: they are only here, each with the basis it came from, and a re-ranking after a real phone run is
//     an edit of this table alone.
//  4. A candidate is marked measured when no run of ours exists: `measured` is true only for candidates in the Mac sample runs of
//     2026-11-11 (docs/reference/ON-DEVICE-MODELS.md); the rest are listed for newer phones and say so.

/// What a model is for in the pipeline. Each role maps to one stage of voice setup.
public enum ConfigRole: String, Codable, CaseIterable, Sendable {
    case vad, transcriber, voice, decision, responder
    public var stage: SetupStage {
        switch self { case .vad: return .bounds; case .transcriber: return .transcriber; case .voice: return .voice; case .decision: return .decision; case .responder: return .responder }
    }
    /// How much a step in quality for this role is worth in a configuration's score. The voice's own words matter most (owner: LFM3 is why this
    /// exists), so the responder's weight is large enough that a better responder is never traded for a nicer listener or voice.
    public var weight: Double {
        switch self { case .responder: return 10; case .voice: return 2; case .transcriber: return 2; case .decision: return 1; case .vad: return 1 }
    }
}

/// Memory classes. The floor is about a 6 GB A15/A16 phone (iPhone 13 Pro up to the 14 Pro Max, the owner's benchmark); older phones are not
/// supported and get a plain "not supported" result, no code path of their own.
public enum DeviceClass: Int, Codable, Comparable, Sendable {
    case unsupported = 0, tier6 = 6, tier8 = 8, tier12 = 12
    public static func < (a: DeviceClass, b: DeviceClass) -> Bool { a.rawValue < b.rawValue }
    public var label: String { switch self { case .unsupported: return "not supported"; case .tier6: return "6 GB class"; case .tier8: return "8 GB class"; case .tier12: return "12 GB class" } }
}

public struct ModelCandidate: Codable, Equatable, Sendable {
    public var id: String
    public var role: ConfigRole
    public var family: String
    public var quant: String
    /// Plain words for the person ("LFM3 1.7B").
    public var label: String
    /// File size on disk (0: needs no download, a floor).
    public var bytes: Int64
    /// Quality, higher is better, comparable within a role only. DATA: from our own sample runs; `qualityBasis` says how.
    public var qualityRank: Int
    public var qualityBasis: String
    public var license: String
    /// A limit the person or the owner should know about, if any (never a reason to rank lower: that is the owner's call).
    public var licenseLimit: String?
    /// The smallest memory class that should try it.
    public var minTier: DeviceClass
    /// About how much memory it holds once loaded (MB): weights plus its context, from file size and our measured overheads.
    public var residentMB: Double
    /// The part of `residentMB` that is a context of its own when the weights are shared with another role.
    public var contextMB: Double
    /// True only when one of our own runs measured it (the Mac sample runs of 2026-11-11).
    public var measured: Bool
    public var source: String
    /// The stage id this becomes in the engine's registry ("" for the soft cue, which is not a stage).
    public var kitStageID: String
    public var isFloor: Bool { bytes == 0 }
}

public enum CandidateTable {
    /// Weights plus the cost of a context: our Mac runs put a GGUF's resident size at about its file size plus 0.1 to 0.25 GB of KV cache.
    static func gguf(_ id: String, _ role: ConfigRole, family: String, quant: String, label: String, rank: Int, basis: String, tier: DeviceClass, measured: Bool,
                     license: String = "Apache-2.0", limit: String? = nil, context: Double = 200) -> ModelCandidate {
        let bytes = KnownModels.pinned.first { $0.id == id }?.bytes ?? 0
        return ModelCandidate(id: id, role: role, family: family, quant: quant, label: label, bytes: bytes, qualityRank: rank, qualityBasis: basis, license: license,
                              licenseLimit: limit, minTier: tier, residentMB: Double(bytes) / 1_048_576 * 1.05 + context, contextMB: context, measured: measured,
                              source: KnownModels.extraGGUF.first { $0.spec.id == id }.map { "hf:" + $0.repo } ?? "hf (see KnownModels)", kitStageID: role == .responder ? "local-ack" : "llama-qwen3-0.6b")
    }

    static let lfmLimit = "LFM Open License v1.0: commercial use at or above US$10M annual revenue (the legal entity and affiliates) needs a separate licence from Liquid AI; read section 5 before shipping."
    static let sampleBasis = "Mac sample runs 2026-11-11 (four asks on invented pages, cold and warm cache)"

    /// All candidates, every role. Order within a role is by rank, not by listing.
    public static let all: [ModelCandidate] = {
        var t: [ModelCandidate] = []

        // --- responder: the voice that talks while the desk works. Ranks from our own sample runs: LFM3 with thinking off was the most
        //     faithful to the page and the only one that said plainly when the page had nothing; then Qwen3-0.6B Q8 (grounded, sometimes
        //     echoes the prompt), Qwen3.5-0.8B (relevant, wanders), Qwen3-0.6B Q4 (generic filler), Qwen3-1.7B (echoes, fabricates).
        //     Within a family a higher quant ranks higher (more of the weights' quality; the vendor's QAD Q4_0 is distilled for its 4-bit form).
        func lfm(_ id: String, _ quant: String, _ rank: Int, _ tier: DeviceClass, measured: Bool, note: String = "") {
            t.append(gguf(id, .responder, family: "LFM3 1.7B", quant: quant, label: "LFM3 1.7B", rank: rank,
                          basis: sampleBasis + (note.isEmpty ? "; the quant's rank within the family is the vendor's ordering (higher bits, higher quality), not our run" : "; " + note),
                          tier: tier, measured: measured, license: KnownModels.lfmLicense, limit: lfmLimit, context: 260))
        }
        lfm("lfm3-1.7b-qad-q4_0", "QAD Q4_0", 93, .tier6, measured: true, note: "this quant was the one run")
        lfm("lfm3-1.7b-q4_k_m", "Q4_K_M", 91, .tier6, measured: false)
        lfm("lfm3-1.7b-q5_k_m", "Q5_K_M", 94, .tier6, measured: false)
        lfm("lfm3-1.7b-q6_k", "Q6_K", 96, .tier8, measured: false)
        lfm("lfm3-1.7b-q8_0", "Q8_0", 98, .tier8, measured: false)
        lfm("lfm3-1.7b-nvfp4", "NVFP4", 90, .tier8, measured: false, note: "whether llama.cpp b12259 runs NVFP4 on iOS is unknown: the on-device check decides")
        t.append(gguf("qwen3-0.6b-q8_0", .responder, family: "Qwen3 0.6B", quant: "Q8_0", label: "Qwen3 0.6B", rank: 62, basis: sampleBasis, tier: .tier6, measured: true, context: 235))
        t.append(gguf("qwen3-0.6b-q4_k_m", .responder, family: "Qwen3 0.6B", quant: "Q4_K_M", label: "Qwen3 0.6B", rank: 52, basis: sampleBasis, tier: .tier6, measured: true, context: 235))
        t.append(gguf("qwen3.5-0.8b-q8_0", .responder, family: "Qwen3.5 0.8B", quant: "Q8_0", label: "Qwen3.5 0.8B", rank: 58, basis: sampleBasis + "; the Q4 was the one run", tier: .tier6, measured: false))
        t.append(gguf("qwen3.5-0.8b-q4_k_m", .responder, family: "Qwen3.5 0.8B", quant: "Q4_K_M", label: "Qwen3.5 0.8B", rank: 56, basis: sampleBasis, tier: .tier6, measured: true))
        t.append(gguf("qwen3-1.7b-q8_0", .responder, family: "Qwen3 1.7B", quant: "Q8_0", label: "Qwen3 1.7B", rank: 50, basis: sampleBasis + "; the Q4 was the one run", tier: .tier8, measured: false, context: 260))
        t.append(gguf("qwen3-1.7b-q6_k", .responder, family: "Qwen3 1.7B", quant: "Q6_K", label: "Qwen3 1.7B", rank: 49, basis: sampleBasis + "; the Q4 was the one run", tier: .tier8, measured: false, context: 260))
        t.append(gguf("qwen3-1.7b-q4_k_m", .responder, family: "Qwen3 1.7B", quant: "Q4_K_M", label: "Qwen3 1.7B", rank: 47, basis: sampleBasis, tier: .tier6, measured: true, context: 260))
        // Newer phones only, never run by us: a bigger sibling is expected to be at least as good as the 0.8B; the configurator measures it.
        t.append(gguf("qwen3.5-2b-q4_k_m", .responder, family: "Qwen3.5 2B", quant: "Q4_K_M", label: "Qwen3.5 2B", rank: 64, basis: "UNMEASURED: expected above its 0.8B sibling; never run by us", tier: .tier8, measured: false, context: 300))
        t.append(gguf("qwen3.5-2b-q8_0", .responder, family: "Qwen3.5 2B", quant: "Q8_0", label: "Qwen3.5 2B", rank: 66, basis: "UNMEASURED: expected above its 0.8B sibling; never run by us", tier: .tier12, measured: false, context: 300))
        t.append(ModelCandidate(id: "cue-only", role: .responder, family: "none", quant: "", label: "A soft sound and no words", bytes: 0, qualityRank: 0, qualityBasis: "the floor",
                                license: "", licenseLimit: nil, minTier: .tier6, residentMB: 0, contextMB: 0, measured: true, source: "built in", kitStageID: "local-ack"))

        // --- decision: quick decisions (System 1). The Qwen3-0.6B Q8_0 file is also a responder candidate; when both pick it the weights are shared.
        t.append(gguf("qwen3-0.6b-q8_0", .decision, family: "Qwen3 0.6B", quant: "Q8_0", label: "A small language model on this phone", rank: 60,
                      basis: "58 ms per decision on the Mac, correct on the clear request (2026-11-11)", tier: .tier6, measured: true, context: 120))
        t.append(ModelCandidate(id: "system1-computer", role: .decision, family: "computer", quant: "", label: "Your computer", bytes: 0, qualityRank: 40,
                                qualityBasis: "needs the network; not measured by the configurator", license: "", licenseLimit: nil, minTier: .tier6, residentMB: 0, contextMB: 0,
                                measured: true, source: "your computer", kitStageID: "choir-system1"))

        // --- the audio models (fetched and compiled by FluidAudio from Hugging Face; the library checks them). Resident sizes are our Mac
        //     process growth on load; "first sound" on a phone is unmeasured (FluidAudio says Kokoro is unvalidated on iOS).
        func audio(_ id: String, _ role: ConfigRole, _ label: String, rank: Int, basis: String, bytes: Int64, mb: Double, tier: DeviceClass, stage: String, license: String, measured: Bool = true) {
            t.append(ModelCandidate(id: id, role: role, family: label, quant: "", label: label, bytes: bytes, qualityRank: rank, qualityBasis: basis, license: license, licenseLimit: nil,
                                    minTier: tier, residentMB: mb, contextMB: 0, measured: measured, source: KnownModels.managed.first { $0.id == id }?.source ?? "hf (FluidAudio)", kitStageID: stage))
        }
        audio("silero-vad-v6.2.1", .vad, "Silero speech detector", rank: 50, basis: "0.001 of real time on the Mac", bytes: 2_000_000, mb: 110, tier: .tier6, stage: "silero-vad", license: "MIT")
        t.append(ModelCandidate(id: "energy", role: .vad, family: "level detector", quant: "", label: "Level detector built in", bytes: 0, qualityRank: 10, qualityBasis: "the floor",
                                license: "", licenseLimit: nil, minTier: .tier6, residentMB: 0, contextMB: 0, measured: true, source: "built in", kitStageID: "energy-vad"))
        audio("parakeet-tdt-0.6b-v2", .transcriber, "Parakeet, the larger listener", rank: 80, basis: "0.044 of real time on the Mac; more accurate than the streaming model, no partials", bytes: 480_000_000, mb: 700, tier: .tier6, stage: "parakeet-tdt", license: "CC-BY-4.0")
        audio("parakeet-eou-120m", .transcriber, "Parakeet, the smaller listener", rank: 60, basis: "0.175 of real time on the Mac; streaming with end-of-utterance, partials while you speak", bytes: 250_000_000, mb: 250, tier: .tier6, stage: "parakeet-eou", license: "NVIDIA Open Model License")
        t.append(ModelCandidate(id: "apple-speech", role: .transcriber, family: "Apple", quant: "", label: "Apple's recogniser", bytes: 0, qualityRank: 20, qualityBasis: "the floor",
                                license: "", licenseLimit: nil, minTier: .tier6, residentMB: 0, contextMB: 0, measured: true, source: "built in", kitStageID: "apple-speech"))
        audio("kokoro-82m-ane", .voice, "Kokoro voice", rank: 80, basis: "first sound 243 ms on the Mac (does not stream); unvalidated on a phone", bytes: 350_000_000, mb: 750, tier: .tier6, stage: "kokoro", license: "Apache-2.0")
        audio("pocket-tts", .voice, "Pocket voice, the lighter one", rank: 65, basis: "first frame 98 ms on the Mac, streams frames", bytes: 300_000_000, mb: 500, tier: .tier6, stage: "pocket-tts", license: "CC-BY-4.0")
        t.append(ModelCandidate(id: "system-voice", role: .voice, family: "Apple", quant: "", label: "The system voice", bytes: 0, qualityRank: 20, qualityBasis: "the floor",
                                license: "", licenseLimit: nil, minTier: .tier6, residentMB: 0, contextMB: 0, measured: true, source: "built in", kitStageID: "avspeech"))
        return t
    }()

    public static func candidates(_ role: ConfigRole, in table: [ModelCandidate] = all) -> [ModelCandidate] {
        table.filter { $0.role == role }.sorted { $0.qualityRank > $1.qualityRank }
    }
    public static func candidate(_ id: String, _ role: ConfigRole, in table: [ModelCandidate] = all) -> ModelCandidate? {
        table.first { $0.id == id && $0.role == role }
    }
}
