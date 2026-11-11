import Foundation
import AutoradioKit
#if canImport(llama)
import llama
#endif

/// System 1 on the phone: a small text model answers the same fixed-form questions the cloud System 1 does, and the
/// option's probability comes from the logits of one decoded step, never from sampled text.
///
/// Route: llama.cpp (the official `llama.xcframework`, release b12259) running Qwen3-0.6B Q8_0 from the official
/// GGUF repo. The prompt ends where the answer letter would go; the stage reads the next-token logits, softmaxes
/// over the option letters only, and reports the winner with its probability.
enum LlamaCatalog {
    static let qwen3 = ModelSpec(
        id: "qwen3-0.6b-q8_0",
        url: URL(string: "https://huggingface.co/Qwen/Qwen3-0.6B-GGUF/resolve/main/Qwen3-0.6B-Q8_0.gguf")!,
        sha256: "9465e63a22add5354d9bb4b99e90117043c7124007664907259bd16d043bb031",
        bytes: 639_446_688, fileName: "Qwen3-0.6B-Q8_0.gguf")
    static var specs: [ManagedModelSpec] {
        [ManagedModelSpec(id: qwen3.id, label: "Qwen3-0.6B Q8_0 (llama.cpp GGUF)", source: "hf:Qwen/Qwen3-0.6B-GGUF",
                          approxBytes: qwen3.bytes, license: "Apache-2.0 (model); llama.cpp MIT", cachePath: ModelHub.store.root)]
    }
}

func registerLlamaStages(_ r: StageRegistry) {
    #if canImport(llama)
    r.register(.system1, LlamaSystem1.self) { LlamaSystem1($0) }
    #else
    r.register(.system1, id: "llama-qwen3-0.6b",
               descriptor: StageDescriptor(displayName: "Qwen3-0.6B via llama.cpp (unavailable)",
                                           summary: "unavailable: this build has no llama.cpp framework (build mobile/scripts/build-llama-xcframework.sh, then rebuild the app)",
                                           needsModels: [LlamaCatalog.qwen3.id])) { _ in throw LabError.unavailable("no llama.cpp framework in this build") }
    #endif
}

#if canImport(llama)
public final class LlamaSystem1: System1, @unchecked Sendable {
    public static let stageID = "llama-qwen3-0.6b"
    public static var descriptor: StageDescriptor {
        StageDescriptor(
            displayName: "Qwen3-0.6B via llama.cpp (one-token scorer)",
            summary: "On-device System 1: the option letter's probability from the logits of one step. Downloads 639 MB on first use.",
            params: [.number("threads", "CPU threads", 4, 1, 8), .flag("gpu", "Metal offload", true),
                     .number("contextTokens", "Context tokens", 2048, 512, 8192)],
            needsModels: [LlamaCatalog.qwen3.id])
    }
    static let slot = ModelSlot<LlamaEngine>(LlamaCatalog.qwen3.id)
    let threads: Int32, gpu: Bool, ctx: UInt32
    public init(_ p: StageParams) { threads = Int32(p.int("threads", 4)); gpu = p.bool("gpu", true); ctx = UInt32(p.int("contextTokens", 2048)) }

    func engine() async throws -> LlamaEngine {
        let (threads, gpu, ctx) = (threads, gpu, ctx)
        return try await Self.slot.get {
            await ModelHub.store.register(LlamaCatalog.qwen3)
            let url = try await ModelHub.store.ensure(LlamaCatalog.qwen3.id)
            let e = try LlamaEngine(path: url.path, threads: threads, gpu: gpu, contextTokens: ctx)
            return (e, {})
        }
    }

    public func answer(_ input: System1Input) async throws -> [System1Answer] {
        let e = try await engine()
        return try input.questions.map { q in
            let t0 = nowMs()
            let letters = (0..<min(q.options.count, 26)).map { String(UnicodeScalar(UInt8(65 + $0))) }
            let prompt = Self.prompt(input, q, letters)
            let probs = try e.scoreLetters(prompt: prompt, letters: letters)
            let best = probs.indices.max { probs[$0] < probs[$1] } ?? 0
            return System1Answer(questionID: q.id, option: q.options[best], score: Double(probs[best]), ms: nowMs() - t0)
        }
    }

    /// Qwen3 chat format with thinking switched off (the empty think block), ending where the letter goes.
    static func prompt(_ i: System1Input, _ q: Question, _ letters: [String]) -> String {
        var s = "<|im_start|>system\nYou classify what a radio listener said. Reply with the letter of the best option only.<|im_end|>\n<|im_start|>user\n"
        if !i.context.isEmpty { s += "Context: \(i.context)\n" }
        s += "Listener said: \"\(i.listenerWords)\"\nQuestion: \(q.text)\n"
        for (l, o) in zip(letters, q.options) { s += "\(l). \(o)\n" }
        s += "Answer with the letter only.<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n"
        return s
    }

    /// Everything the proof wants to know about the engine, without a question.
    public static func loadForProof(threads: Int = 4, gpu: Bool = true) async throws {
        _ = try await LlamaSystem1(StageParams(["threads": .number(Double(threads)), "gpu": .bool(gpu)])).engine()
    }
}

final class LlamaEngine: @unchecked Sendable {
    let model: OpaquePointer, ctx: OpaquePointer, vocab: OpaquePointer
    private let lock = NSLock()
    init(path: String, threads: Int32, gpu: Bool, contextTokens: UInt32) throws {
        llama_backend_init()
        var mp = llama_model_default_params()
        #if targetEnvironment(simulator)
        mp.n_gpu_layers = 0
        #else
        mp.n_gpu_layers = gpu ? 99 : 0
        #endif
        guard let m = llama_model_load_from_file(path, mp) else { throw LabError.unavailable("llama.cpp could not load \(path)") }
        var cp = llama_context_default_params()
        cp.n_ctx = contextTokens; cp.n_batch = contextTokens; cp.n_ubatch = min(contextTokens, 512)
        cp.n_threads = threads; cp.n_threads_batch = threads
        guard let c = llama_init_from_model(m, cp) else { llama_model_free(m); throw LabError.unavailable("llama.cpp could not create a context") }
        model = m; ctx = c; vocab = llama_model_get_vocab(m)!
    }
    deinit { llama_free(ctx); llama_model_free(model) }

    func tokenize(_ s: String, special: Bool) -> [Int32] {
        var toks = [Int32](repeating: 0, count: s.utf8.count + 16)
        var n = llama_tokenize(vocab, s, Int32(s.utf8.count), &toks, Int32(toks.count), false, special)
        if n < 0 { toks = [Int32](repeating: 0, count: Int(-n)); n = llama_tokenize(vocab, s, Int32(s.utf8.count), &toks, Int32(toks.count), false, special) }
        return Array(toks.prefix(Int(max(0, n))))
    }

    /// Probability of each letter as the next token, softmaxed over the letters only.
    func scoreLetters(prompt: String, letters: [String]) throws -> [Float] {
        lock.lock(); defer { lock.unlock() }
        var ids: [Int32] = []
        for l in letters {
            let t = tokenize(l, special: false)
            guard t.count == 1 else { throw LabError.badInput("option letter \(l) is not one token") }
            ids.append(t[0])
        }
        var toks = tokenize(prompt, special: true)
        guard !toks.isEmpty else { throw LabError.badInput("empty prompt") }
        llama_memory_clear(llama_get_memory(ctx), true)
        let rc = toks.withUnsafeMutableBufferPointer { llama_decode(ctx, llama_batch_get_one($0.baseAddress, Int32($0.count))) }
        guard rc == 0, let logits = llama_get_logits_ith(ctx, -1) else { throw LabError.unavailable("llama_decode failed (\(rc))") }
        let raw = ids.map { logits[Int($0)] }
        let mx = raw.max() ?? 0
        let ex = raw.map { expf($0 - mx) }, sum = ex.reduce(0, +)
        return ex.map { $0 / sum }
    }
}
#endif
