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

    /// EmbeddingGemma 2, Q8_0, the official GGUF (ggml-org, Apache-2.0; the Gemma Prohibited Use Policy applies): the on-device embedder behind search.
    /// Size and SHA-256 are the Hugging Face listing's and were re-checked on the downloaded file, 2026-11-17. Runs on llama.cpp b12259.
    /// docs/reference/EMBEDDINGGEMMA.md. Fetched at first use into the store, never in the bundle.
    /// The name a vector store records: the model file and the runtime. A store made by another engine is not read.
    public static let embeddingEngine = "embeddinggemma-2-q8_0@llama.cpp-b12259"
    /// A passage whose cosine to the question is below this is not a match on vectors alone. 0.67: the trial's 50 answerable questions had a best cosine of at least 0.716 and its 6 unanswerable ones at most 0.627 (reference page, section 0.6).
    public static let embeddingCosineFloor: Float = 0.67
    public static let embeddingGemma2 = ModelSpec(
        id: "embeddinggemma-2-q8_0",
        url: URL(string: "https://huggingface.co/ggml-org/embeddinggemma-2-GGUF/resolve/main/embeddinggemma-2-Q8_0.gguf")!,
        sha256: "2188ac1deca4b77dffefd603c2776a9d76d9d74ec01841392982ebb840b09135",
        bytes: 309_855_456, fileName: "embeddinggemma-2-Q8_0.gguf")

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


    /// Fluid Speech Mini by FluidVoice: English speech recognition, a Core ML build of a Parakeet TDT 0.6B v3 derivative, in one folder of files.
    /// Repository, commit, every file's size and every SHA-256 were read on 2026-11-24 from Hugging Face's API (`/api/models/<repo>?blobs=true`,
    /// the LFS object ids) and, for the files the API gives no digest for, computed on the downloaded bytes at that commit. Only the files the
    /// loader reads are listed (the model card, its examples and its images are not fetched). FluidAudio loads it as `.ultra` and wants the joint
    /// network under the name `JointDecisionv3.mlmodelc`, so the folder is copied under that name after the download (Mike's fetch script does the same).
    /// Licence CC BY 4.0: the credit below is the one NOTICE in the repository asks for. Settings > Downloads and the first-run welcome show it beside the option (the list entry carries it: `ModelListEntry.credit`; a test fails for a CC model without one).
    /// docs/reference/ON-DEVICE-MODELS.md, "Fluid Speech Mini".
    public static let fluidSpeechMini = ModelBundleSpec(
        id: "fluid-speech-mini", repo: "FluidVoice-Labs/fluid-speech-mini-coreml", commit: "d81c3160dfe5a5e1f8f7267164a07d30617a3a1a",
        files: [
            .init(path: "Decoder.mlmodelc/analytics/coremldata.bin", bytes: 243, sha256: "d4beee43a6eab29b2adcc298f2e8582e8fb79d5fe3aa9f1ab692ade681ca369d"),
            .init(path: "Decoder.mlmodelc/coremldata.bin", bytes: 491, sha256: "6094d06d05c1ca4599c7b13c9a4ca7c00826ee2396796cf3084ce602a9884468"),
            .init(path: "Decoder.mlmodelc/metadata.json", bytes: 3_373, sha256: "13fcb358af8b028703dc7f424fd149ce9524642de510f3d682b75d471b1a001d"),
            .init(path: "Decoder.mlmodelc/model.mil", bytes: 13_411, sha256: "4ccd8ecbc7eb59bed7ad38d471bf6b78ef52a24500bf64619c549ed45fbee4e9"),
            .init(path: "Decoder.mlmodelc/weights/weight.bin", bytes: 11_845_120, sha256: "d173e381488d07fd7648e2ccecbfb1bceff0457d75461550b26ba7fa2d23309a"),
            .init(path: "Encoder.mlmodelc/analytics/coremldata.bin", bytes: 243, sha256: "3a3caa2c6aec983d94a5659eff09a00f77077147ea6726b8ddfdf022f0238ce9"),
            .init(path: "Encoder.mlmodelc/coremldata.bin", bytes: 427, sha256: "494d8e2ce30b37bcdf494b3ba7ddf6e1bc921b9a59e369a9efd44d013ca3bf7d"),
            .init(path: "Encoder.mlmodelc/metadata.json", bytes: 2_906, sha256: "1a2229e2cc4c271e4876715c9e36aef18bc7287e35690022c7f3939d85260ea9"),
            .init(path: "Encoder.mlmodelc/model.mil", bytes: 712_643, sha256: "f96b7707936ea7ce5517c5b0f5c15cc634006bc58af1fe2587ac5979dd180343"),
            .init(path: "Encoder.mlmodelc/weights/weight.bin", bytes: 233_096_704, sha256: "cf0b61022de8c698984a8c7185c2d093fa744694621881ff062c46fee2546131"),
            .init(path: "JointDecision.mlmodelc/analytics/coremldata.bin", bytes: 243, sha256: "9e767d69e1092b27b28d9f236b9cbcded58ff3891e320cba7855e2c24e803d69"),
            .init(path: "JointDecision.mlmodelc/coremldata.bin", bytes: 462, sha256: "284b235c08cd4769202af017f854deace69c0209c29f2253a2073bb68d58d38d"),
            .init(path: "JointDecision.mlmodelc/metadata.json", bytes: 2_790, sha256: "fe1ce083f9ff57524926a92fc264b0cf7d88f3ba9852f7acf9f59dcdf427e01f"),
            .init(path: "JointDecision.mlmodelc/model.mil", bytes: 7_547, sha256: "1109ce72f6d014a6b99e49b41e83b3082d7555d74a6275838e7cbea42a6bd0ee"),
            .init(path: "JointDecision.mlmodelc/weights/weight.bin", bytes: 6_350_284, sha256: "f991f65979c1b3fee3db39a645aa87e6bef1a304ed1816d074b17479bea797fc"),
            .init(path: "LICENSE", bytes: 18_657, sha256: "9ba9550ad48438d0836ddab3da480b3b69ffa0aac7b7878b5a0039e7ab429411"),
            .init(path: "NOTICE", bytes: 690, sha256: "05119420d65cf41d7434a83027c23f1360740d7e8c25d6b47263698ffe1f4c86"),
            .init(path: "Preprocessor.mlmodelc/analytics/coremldata.bin", bytes: 243, sha256: "c9beeb989c8d66f8be11df59bc6df277ec76cee404f6865b46243835ef562f6d"),
            .init(path: "Preprocessor.mlmodelc/coremldata.bin", bytes: 486, sha256: "dbde3f2300842c1fd51ef3ff948a0bcffe65ffd2dca10707f2509f32c1d65b1d"),
            .init(path: "Preprocessor.mlmodelc/metadata.json", bytes: 2_841, sha256: "2a98699e22d279dd37fa1d238aeb1c6db1df0d6fad687775324157689d8f3acf"),
            .init(path: "Preprocessor.mlmodelc/model.mil", bytes: 28_181, sha256: "4b8518a956450fec57f06c2a21bdffc26973f7f1fa6842fb38fe917f896b6b93"),
            .init(path: "Preprocessor.mlmodelc/weights/weight.bin", bytes: 491_072, sha256: "129b76e3aeafa8afa3ea76d995b964b145fe83700d579f6ff42c4c38fa0968ea"),
            .init(path: "config.json", bytes: 314, sha256: "5e6e1189fd8e9da59633b929a977c716d696b84aff6ab9b040423bc8b9c8b3be"),
            .init(path: "parakeet_vocab.json", bytes: 151_122, sha256: "7ec60e05f1b24480736ec0eed40900f4626bce1fa9a60fd700ec7e2a59198735")
        ],
        folder: "fluid-speech-mini", license: "CC BY 4.0",
        credit: "Fluid Speech Mini by FluidVoice. Built on NVIDIA Parakeet TDT 0.6B v3 and Moondream Parakeet Ultra. Licensed under CC BY 4.0 (https://creativecommons.org/licenses/by/4.0/).",
        copies: [["JointDecision.mlmodelc", "JointDecisionv3.mlmodelc"]])

    /// Fluid Speech Pro by FluidVoice: the full-size sibling of Mini (multilingual Parakeet TDT 0.6B v3 fine-tune with one extra encoder block, 8-bit
    /// encoder), Core ML build `FluidVoice-Labs/fluid-speech-pro-coreml`, 652,324,639 bytes in the files below. Read 2026-11-24 from Hugging Face's API
    /// at the commit pinned here (the files the loader reads; README, examples, images and the duplicate `parakeet_v3_vocab.json` are not fetched).
    /// It has the v3 contract exactly, so the pinned FluidAudio 0.18.3 loads it as `.ultra` from a local folder with the joint under the v3 name
    /// (proved on the Mac, ON-DEVICE-MODELS.md); the vendor's example names `.fluidParakeetPro`, a version only in the altic-dev fork, which we do not need.
    /// Licence CC BY 4.0; the credit is the one NOTICE asks for.
    public static let fluidSpeechPro = ModelBundleSpec(
        id: "fluid-speech-pro", repo: "FluidVoice-Labs/fluid-speech-pro-coreml", commit: "3b9d38a93d606f84011e5a0a669b7ea146375a22",
        files: [
            .init(path: "Decoder.mlmodelc/analytics/coremldata.bin", bytes: 243, sha256: "d172cb80cbf3a2cd768952557164db1b50b9c6aef16af3653d2fd7c03d9fe8e5"),
            .init(path: "Decoder.mlmodelc/coremldata.bin", bytes: 491, sha256: "5059c27aa8cc44eb735247d335e18da599d15e30d67b3d2d5633679fc5806f8a"),
            .init(path: "Decoder.mlmodelc/metadata.json", bytes: 3_311, sha256: "47ee2ad3514945b6ee3ccd4171ed3b2318e38f3b929e4398ce33b50aca606083"),
            .init(path: "Decoder.mlmodelc/model.mil", bytes: 11_823, sha256: "9d605291dfdaef07ef88b8d3dbf0702766f1ceea5649d5d6dee4d2322d8fa678"),
            .init(path: "Decoder.mlmodelc/weights/weight.bin", bytes: 23_604_992, sha256: "7b3e3a4e3fbb580ae990613915f8ea2309226d85999ae37120912b8352d1df9e"),
            .init(path: "Encoder.mlmodelc/analytics/coremldata.bin", bytes: 243, sha256: "ac53d09c1b1f7f00fd1a07a630d57c4193089b19fe0d3b1571a3ecf91be62aa4"),
            .init(path: "Encoder.mlmodelc/coremldata.bin", bytes: 427, sha256: "abfbc09cc0b8aba7911e38390390f1c5850e3665862238035052e015659ebb74"),
            .init(path: "Encoder.mlmodelc/metadata.json", bytes: 2_758, sha256: "cc74be4442be9fe7d6428aa238364d40e91d3c0ca4ce3cfa2f4fef4ad7a42cee"),
            .init(path: "Encoder.mlmodelc/model.mil", bytes: 621_903, sha256: "f71d68afe10e227c39d43af07c1aa1a88dd0eb2f63109a3e03711066e38a2e9d"),
            .init(path: "Encoder.mlmodelc/weights/weight.bin", bytes: 613_088_448, sha256: "4abdc3b4af2ff9b2843105f29677f0e674d2b08638b1ce6feac60876c235c659"),
            .init(path: "JointDecision.mlmodelc/analytics/coremldata.bin", bytes: 243, sha256: "3dd0d66c51f69d683c587d5c88f3256d5e1690105ecf9d8ca4b0c2b8e1c883c7"),
            .init(path: "JointDecision.mlmodelc/coremldata.bin", bytes: 462, sha256: "febdb37a5ad1a119539c33cf68fac1c51f80f40f82c90b6ef312033191ff4063"),
            .init(path: "JointDecision.mlmodelc/metadata.json", bytes: 2_728, sha256: "36fa268dd3c132c7305bbe1f17b5891a00f91caf0a883782375c35a97e2f4ada"),
            .init(path: "JointDecision.mlmodelc/model.mil", bytes: 7_178, sha256: "618bc532db01d5d80da27d0137e4b50cf2f56dde54e0686a4a1e732d74c1a55c"),
            .init(path: "JointDecision.mlmodelc/weights/weight.bin", bytes: 12_642_764, sha256: "f0939e8807dded497926a7c3634af698557ec8b155d804f8f40e367e97f90849"),
            .init(path: "LICENSE", bytes: 18_657, sha256: "9ba9550ad48438d0836ddab3da480b3b69ffa0aac7b7878b5a0039e7ab429411"),
            .init(path: "NOTICE", bytes: 451, sha256: "fb46bca8b337792a755ec6c77e0e33ebd27d181f46540e4b88382d179942625c"),
            .init(path: "Preprocessor.mlmodelc/analytics/coremldata.bin", bytes: 243, sha256: "886396da81b6b29d60d79d903bd5d1f00077a7a4323f40cd2e9214d4205d8e6b"),
            .init(path: "Preprocessor.mlmodelc/coremldata.bin", bytes: 429, sha256: "38f130eac9ffde8c684043d76e3eecdd3b6f9564d9efa59f7a87a34a272a4901"),
            .init(path: "Preprocessor.mlmodelc/metadata.json", bytes: 2_606, sha256: "c25d98c19a181b7fa65ee50c34f5066f67172c22f0bf116a42b59443ee4bc37d"),
            .init(path: "Preprocessor.mlmodelc/model.mil", bytes: 11_891, sha256: "71d4ad016334b3ad73ea7b69b6813feefe3b81f59e0b5e924f90f088cb02a7c9"),
            .init(path: "Preprocessor.mlmodelc/weights/weight.bin", bytes: 2_150_580, sha256: "44180071e400951080786feb46452bffc4fb26363a703655ebadb760c8ad4030"),
            .init(path: "config.json", bytes: 646, sha256: "b688c4d174c4fb043b368dad402ae91d4ae68cf2a53db42ff663340913c907db"),
            .init(path: "parakeet_vocab.json", bytes: 151_122, sha256: "7ec60e05f1b24480736ec0eed40900f4626bce1fa9a60fd700ec7e2a59198735")
        ],
        folder: "fluid-speech-pro", license: "CC BY 4.0",
        credit: "Fluid Speech Pro by FluidVoice. Built on NVIDIA Parakeet TDT 0.6B v3 and Moondream Parakeet Ultra. Licensed under CC BY 4.0 (https://creativecommons.org/licenses/by/4.0/).",
        copies: [["JointDecision.mlmodelc", "JointDecisionv3.mlmodelc"]])

    /// Models that are a folder of files fetched by the store.
    public static var bundles: [ModelBundleSpec] { [fluidSpeechMini, fluidSpeechPro] }

    /// Models fetched by us over HTTP, pinned by SHA-256.
    public static var pinned: [ModelSpec] { [qwen3, qwen3Q4, qwen317, qwen35, lfm3Text, embeddingGemma2] + extraGGUF.map(\.spec) }

    /// More GGUF quantisations of the responder families, for the configurator (the candidate table, ModelCandidates.swift). Sizes and
    /// SHA-256 digests are the Hugging Face API file listings' (LFS object ids) fetched 2026-11-11; names are the repos' own, not guessed.
    /// `rev`: the repository commit the file was read at; when not "main" the download url names it and the model list carries it. The older rows were pinned by digest alone.
    struct ExtraGGUF { var spec: ModelSpec; var repo: String; var license: String; var revision: String? = nil; var credit: String? = nil }
    static func gguf(_ id: String, repo: String, file: String, sha: String, bytes: Int64, license: String, rev: String = "main", credit: String? = nil) -> ExtraGGUF {
        ExtraGGUF(spec: ModelSpec(id: id, url: URL(string: "https://huggingface.co/\(repo)/resolve/\(rev)/\(file)")!, sha256: sha, bytes: bytes, fileName: file),
                  repo: repo, license: license, revision: rev == "main" ? nil : rev, credit: credit)
    }
    static let lfmLicense = "LFM Open License v1.0 (commercial use at or above US$10M annual revenue needs a separate licence from Liquid AI)"
    static let extraGGUF: [ExtraGGUF] = [
        gguf("lfm3-1.7b-q4_k_m", repo: "LiquidAI/LFM3-1.7B-GGUF", file: "LFM3-1.7B-Q4_K_M.gguf", sha: "3793e1e6e2266919d6e015287221b41100175a98eddaba92e111aed3b05c93c4", bytes: 1_099_627_808, license: lfmLicense),
        gguf("lfm3-1.7b-q5_k_m", repo: "LiquidAI/LFM3-1.7B-GGUF", file: "LFM3-1.7B-Q5_K_M.gguf", sha: "41e51fb0be5f472d323055f2734d986d5a2429570973508fa6322f3f07f6d7f3", bytes: 1_258_663_200, license: lfmLicense),
        gguf("lfm3-1.7b-q6_k", repo: "LiquidAI/LFM3-1.7B-GGUF", file: "LFM3-1.7B-Q6_K.gguf", sha: "54e1bd947c77167823d45bf495f1c3e2d226ca7fb242f83cfa141ee3a6ab55c6", bytes: 1_427_638_304, license: lfmLicense),
        gguf("lfm3-1.7b-q8_0", repo: "LiquidAI/LFM3-1.7B-GGUF", file: "LFM3-1.7B-Q8_0.gguf", sha: "8f6b24182258205e9e01e21f1f24a1f208c1e8115578825d202ce39976af20b6", bytes: 1_846_238_496, license: lfmLicense),
        gguf("lfm3-1.7b-nvfp4", repo: "LiquidAI/LFM3-1.7B-GGUF", file: "LFM3-1.7B-NVFP4.gguf", sha: "462c2379586a45b30fe3dbfa30e6ff08cbb440734d6934689e24e4fe59a67bec", bytes: 1_280_019_776, license: lfmLicense),
        // LFM2.5-350M Q4_K_M, a quick-decision option (owner, 2026-11-24). Size and SHA-256 are the host's own listing (`/api/models/LiquidAI/LFM2.5-350M-GGUF?blobs=true`, the LFS
        // object id), read 2026-11-24 without downloading the file; the address names the repository commit that listing was read at. LFM Open License v1.0 (the file `LICENSE` of that commit).
        gguf("lfm2.5-350m-q4_k_m", repo: "LiquidAI/LFM2.5-350M-GGUF", file: "LFM2.5-350M-Q4_K_M.gguf", sha: "7e6f72643caafc9a68256686638c4d7916f2cec76d1df478d4c3ddcd95a6aed4", bytes: 229_312_224, license: lfmLicense, rev: "657e078c94084481950a2d555a941481f715536b"),
        // LFM3-Omni-2B (owner, 2026-11-24: a bigger model for newer phones), vendor repo at commit d508b88, Q8_0 of the text path only (the mmproj, which holds
        // the vision and audio encoders and the speech-out path, is NOT fetched: text in, text out). LICENSE read from the hub 2026-11-24 (SHA-256 4d28ca14...).
        gguf("lfm3-omni-2b-q8_0", repo: "LiquidAI/LFM3-Omni-2B-GGUF", file: "LFM3-Omni-2B-Q8_0.gguf", sha: "59118f661eaacb93d204723d76355b53347f0940b508c11412f6290fd49158d4",
             bytes: 1_846_240_768, license: lfmLicense, rev: "d508b88f08a5b4286f2d2805a6a8d69a0ff44c72",
             credit: "LFM3-Omni-2B by Liquid AI, used under the LFM Open License v1.0 (https://huggingface.co/LiquidAI/LFM3-Omni-2B/blob/main/LICENSE)."),
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
                             approxBytes: 250_000_000, license: "NVIDIA Open Model License (model); FluidAudio Apache-2.0", cachePath: c, cacheFolders: ["parakeet-eou-streaming"], loadsWithoutFetching: true),
            ManagedModelSpec(id: "parakeet-tdt-0.6b-v2", label: "Parakeet TDT 0.6B v2 (English, Core ML)", source: "hf:FluidInference/parakeet-tdt-0.6b-v2-coreml",
                             approxBytes: 480_000_000, license: "CC-BY-4.0 (NVIDIA model); FluidAudio Apache-2.0", cachePath: c, cacheFolders: ["parakeet-tdt-0.6b-v2", "parakeet-tdt-0.6b-v2-coreml"],
                             credit: "Parakeet TDT 0.6B v2 by NVIDIA, converted to Core ML by FluidInference. Licensed under CC BY 4.0 (https://creativecommons.org/licenses/by/4.0/); the files are modified Core ML conversions.", loadsWithoutFetching: true),
            ManagedModelSpec(id: fluidSpeechPro.id, label: "Fluid Speech Pro (Core ML)", source: "hf:" + fluidSpeechPro.repo,
                             approxBytes: fluidSpeechPro.totalBytes, license: fluidSpeechPro.license + " (FluidVoice model); FluidAudio Apache-2.0",
                             cachePath: storeRoot, cacheFolders: [fluidSpeechPro.folder], loadsWithoutFetching: true),
            ManagedModelSpec(id: fluidSpeechMini.id, label: "Fluid Speech Mini (English, Core ML)", source: "hf:" + fluidSpeechMini.repo,
                             approxBytes: fluidSpeechMini.totalBytes, license: fluidSpeechMini.license + " (FluidVoice model); FluidAudio Apache-2.0",
                             cachePath: storeRoot, cacheFolders: [fluidSpeechMini.folder], loadsWithoutFetching: true),
            ManagedModelSpec(id: "kokoro-82m-ane", label: "Kokoro 82M ANE v3 (Core ML)", source: "hf:FluidInference/kokoro-82m-coreml",
                             approxBytes: kokoroBytes, license: "Apache-2.0 (model); FluidAudio Apache-2.0", cachePath: c, cacheFolders: ["kokoro-82m-coreml", "kokoro"],
                             credit: VoiceRows.credit(model: "kokoro-82m-ane")),
            ManagedModelSpec(id: "pocket-tts", label: "PocketTTS 100M (Kyutai, Core ML)", source: "hf:FluidInference/pocket-tts-coreml",
                             approxBytes: 300_000_000, license: "CC-BY-4.0 (Kyutai model); FluidAudio Apache-2.0", cachePath: c, cacheFolders: ["pocket-tts"],
                             credit: "PocketTTS by Kyutai, converted to Core ML by FluidInference. Licensed under CC BY 4.0 (https://creativecommons.org/licenses/by/4.0/); the files are modified Core ML conversions."),
            // Phonon 31M: the pack `pocket-tts-coreml/phonon/phonon-31m-en` (Apache-2.0, its own LICENSE; the repo's other folders are CC-BY-4.0).
            // FluidAudio's `PocketTtsSmallDownloader` keeps 126 of its files: the MLState model, the fp32 Mimi decoder, the constants and all 103
            // voices, 173,136,428 bytes by the Hugging Face API listing of 2026-11-15. It is asked to put the pack in `fluidRoot` (see PhononVoice).
            ManagedModelSpec(id: "phonon-31m", label: "Phonon 31M (Gradium, Core ML)", source: "hf:FluidInference/pocket-tts-coreml/phonon/phonon-31m-en",
                             approxBytes: 173_000_000, license: "Apache-2.0 (Gradium model); FluidAudio Apache-2.0", cachePath: c, cacheFolders: ["pocket-tts-coreml/phonon"],
                             credit: VoiceRows.credit(model: "phonon-31m")),
            ManagedModelSpec(id: "lfm3-audio-2b", label: "LFM3-Audio-2B (Core ML, ASR + TTS)", source: "hf:FluidInference/lfm3-audio-2b-coreml",
                             approxBytes: 2_900_000_000, license: "LFM Open License (read it: commercial use limited by revenue)", cachePath: c),
            ManagedModelSpec(id: qwen3.id, label: "Qwen3-0.6B Q8_0 (llama.cpp GGUF)", source: "hf:Qwen/Qwen3-0.6B-GGUF",
                             approxBytes: qwen3.bytes, license: "Apache-2.0 (model); llama.cpp MIT", cachePath: storeRoot),
            ManagedModelSpec(id: embeddingGemma2.id, label: "EmbeddingGemma 2 Q8_0 (llama.cpp GGUF, 768-d embeddings)", source: "hf:ggml-org/embeddinggemma-2-GGUF",
                             approxBytes: embeddingGemma2.bytes, license: "Apache-2.0 (model; Gemma Prohibited Use Policy applies); llama.cpp MIT", cachePath: storeRoot),
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

    /// Kokoro's offered size: the model's own files (about 350 MB, the model card's figure) plus the voice files its first load fetches (every Kokoro voice of the catalog, `VoiceStages.KokoroVoice.load`),
    /// summed from the pack's own listing (`VoiceRow.bytes`), so the size shown is what lands on the phone.
    public static var kokoroBytes: Int64 { 350_000_000 + VoiceRows.bytes(model: "kokoro-82m-ane") }

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
    /// The Test button on a model's row (ModelTrial): what only the model itself can do. A build that links no model libraries says so.
    func transcribe(_ id: String, _ audio: [Float]) async throws -> String { throw LabError.unavailable(ModelProviders.missingNote) }
    func speechSpan(_ id: String, _ audio: [Float]) async throws -> (start: Double?, end: Double?) { throw LabError.unavailable(ModelProviders.missingNote) }
    func answer(_ id: String, job: String, prompt: String) async throws -> String { throw LabError.unavailable(ModelProviders.missingNote) }
    func unload(_ id: String, role: String) async {}
    func measureConfiguration(_ picks: [String: String], rounds: Int) async throws -> ConfigMeasurement { throw LabError.unavailable(ModelProviders.missingNote) }
}

extension VoiceRows {
    /// The sum of the voice files of one model, from the pack's listing (0 for a pack that ships its voices inside one fetch).
    public static func bytes(model: String) -> Int64 { all.filter { $0.modelID == model }.reduce(0) { $0 + $1.bytes } }
    /// The credit its licence asks for, the same for every voice of the model ("" when it has no voice rows).
    public static func credit(model: String) -> String? { all.first { $0.modelID == model }?.credit }
}

/// Which responder model setup chose (read at start, written when setup finishes).
public enum ResponderChoice {
    public static let defaultID = "qwen3-0.6b-q8_0"
    static let key = "AutoradioResponderModel"
    /// A download finishing never changes what is in use (only Use does): it may fill a choice that was never made, and only with a model that can be one. True when it wrote.
    @discardableResult public static func adopt(_ id: String, defaults: UserDefaults = .standard) -> Bool {
        guard KnownModels.isPinned(id), defaults.string(forKey: key) == nil else { return false }
        defaults.set(id, forKey: key); return true
    }
    public static var current: String {
        get { UserDefaults.standard.string(forKey: key).flatMap { KnownModels.isPinned($0) ? $0 : nil } ?? defaultID }
        set { UserDefaults.standard.set(newValue, forKey: key) }
    }
}

/// Which quick-decision model the phone runs (read at start, written when setup finishes with a decision pick). Only a pinned model counts: a name this build has no file for is the default.
public enum DecisionChoice {
    public static let defaultID = "qwen3-0.6b-q8_0"
    static let key = "AutoradioDecisionModel"
    /// As `ResponderChoice.adopt`: fills a choice never made, with a model that can be the decision model (a built-in id that setup fell back to is not one).
    @discardableResult public static func adopt(_ id: String, defaults: UserDefaults = .standard) -> Bool {
        guard resolve(saved: id) == id, defaults.string(forKey: key) == nil else { return false }
        defaults.set(id, forKey: key); return true
    }
    /// The decision model a saved name means. The decision role loads weights through llama.cpp, so only a pinned GGUF is one.
    public static func resolve(saved: String?) -> String { saved.flatMap { KnownModels.isPinned($0) && $0 != KnownModels.lfm3Text.id ? $0 : nil } ?? defaultID }
    public static var current: String {
        get { resolve(saved: UserDefaults.standard.string(forKey: key)) }
        set { UserDefaults.standard.set(newValue, forKey: key) }
    }
}

/// The text the decision model reads, in the model's own chat layout. The scorer reads one step's logits after the assistant turn begins, so the prompt ends where the letter goes.
/// Qwen3 ends with its empty think block (thinking off); the LFM2 family starts with its start token, has no think block (the model's own template, `chat_template.jinja`
/// of LFM2.5-350M-GGUF at the pinned commit: bos, `<|im_start|>role\n...<|im_end|>\n`, then `<|im_start|>assistant\n`).
public enum DecisionPrompt {
    public static func text(modelID: String, context: String, listenerWords: String, question: String, options: [String], letters: [String]) -> String {
        let lfm = modelID.hasPrefix("lfm")
        var s = (lfm ? "<|startoftext|>" : "") + "<|im_start|>system\nYou classify what a radio listener said. Reply with the letter of the best option only.<|im_end|>\n<|im_start|>user\n"
        if !context.isEmpty { s += "Context: \(context)\n" }
        s += "Listener said: \"\(listenerWords)\"\nQuestion: \(question)\n"
        for (l, o) in zip(letters, options) { s += "\(l). \(o)\n" }
        s += "Answer with the letter only.<|im_end|>\n<|im_start|>assistant\n" + (lfm ? "" : "<think>\n\n</think>\n\n")
        return s
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
