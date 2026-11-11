import Foundation
import AutoradioKit
#if canImport(llama)
import llama
#endif

/// The phone's language models, through llama.cpp (the official `llama.xcframework`, release b12259).
///
/// Two roles, each with its own context, so one never waits for the other's cache:
///  - the DECISION role (System 1): a small text model answers the same fixed-form questions the cloud System 1 does; the option's
///    probability comes from the logits of one decoded step, never from sampled text.
///  - the RESPONDER role: the voice that talks while the desk works. Its context keeps the stable prefix of the prompt (instructions and
///    the page in front) in the KV cache, so at end of speech only the listener's words are read.
/// When both roles use the same GGUF file the weights are shared and each role adds only its own context (about 0.1 to 0.25 GB of KV
/// cache for a 0.6B model at 1,024 to 2,048 tokens).
enum LlamaCatalog {
    /// The model and its checksum are in the kit's one catalog (KnownModels), shared with voice setup.
    static var qwen3: ModelSpec { KnownModels.qwen3 }
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
                     .number("contextTokens", "Context tokens", 1024, 512, 8192)],
            needsModels: [LlamaCatalog.qwen3.id])
    }
    static let slot = ModelSlot<LlamaEngine>(LlamaCatalog.qwen3.id)
    let threads: Int32, gpu: Bool, ctx: UInt32
    public init(_ p: StageParams) { threads = Int32(p.int("threads", 4)); gpu = p.bool("gpu", true); ctx = UInt32(p.int("contextTokens", 1024)) }

    func engine() async throws -> LlamaEngine {
        try await Self.loadEngine(threads: threads, gpu: gpu, ctx: ctx, download: true)
    }

    /// The decision engine of the process. `download: false` loads only a file that is already on the phone.
    static func loadEngine(threads: Int32 = 4, gpu: Bool = true, ctx: UInt32 = 1024, download: Bool) async throws -> LlamaEngine {
        try await slot.get {
            let url = try await LlamaFiles.url(LlamaCatalog.qwen3.id, download: download)
            let e = try LlamaEngine(weights: try LlamaWeights.shared(path: url.path, gpu: gpu), threads: threads, contextTokens: ctx)
            return (e, {})
        }
    }

    public func answer(_ input: System1Input) async throws -> [System1Answer] {
        // The phone has one processor: a pending acknowledgement goes first (no timer: this resumes when that one is done).
        await LlamaLocalModel.shared.untilNoAcknowledgementPending()
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

/// Where a model's GGUF is: downloaded (SHA-256 checked) or, at start-up, only if already on the phone.
enum LlamaFiles {
    static func url(_ id: String, download: Bool) async throws -> URL {
        if download { return try await ModelStore.shared.ensure(id) }
        if let u = await ModelStore.shared.localFileIfPresent(id) { return u }
        throw LabError.unavailable("the model \(id) is not on this phone yet")
    }
}

/// The weights of one GGUF, shared by every context made from it.
final class LlamaWeights: @unchecked Sendable {
    let model: OpaquePointer, vocab: OpaquePointer
    private static let lock = NSLock()
    nonisolated(unsafe) private static var cache: [String: LlamaWeights] = [:]
    static func shared(path: String, gpu: Bool) throws -> LlamaWeights {
        lock.lock(); defer { lock.unlock() }
        if let w = cache[path] { return w }
        let w = try LlamaWeights(path: path, gpu: gpu)
        cache[path] = w
        return w
    }
    private init(path: String, gpu: Bool) throws {
        llama_backend_init()
        var mp = llama_model_default_params()
        #if targetEnvironment(simulator)
        mp.n_gpu_layers = 0
        #else
        mp.n_gpu_layers = gpu ? 99 : 0
        #endif
        guard let m = llama_model_load_from_file(path, mp) else { throw LabError.unavailable("llama.cpp could not load \(path)") }
        model = m; vocab = llama_model_get_vocab(m)!
    }
    deinit { llama_model_free(model) }
}

/// How a model wants its chat laid out. Qwen's models read ChatML with an empty think block for the non-thinking mode; LFM reads ChatML
/// without it. Which one a file uses is by its id.
struct ChatLayout {
    var begin = ""
    var thinkOff = "<think>\n\n</think>\n\n"
    static func forModel(_ id: String) -> ChatLayout {
        id.hasPrefix("lfm") ? ChatLayout(begin: "<|startoftext|>", thinkOff: "") : ChatLayout()
    }
    func prefix(system: String, userPrefix: String) -> String { begin + "<|im_start|>system\n\(system)<|im_end|>\n<|im_start|>user\n" + userPrefix }
    func suffix(userSuffix: String) -> String { userSuffix + "<|im_end|>\n<|im_start|>assistant\n" + thinkOff }
}

/// What one generation cost, measured.
struct LlamaStats {
    var promptTokens = 0, prefixTokens = 0, reusedTokens = 0
    var prefillMs = 0.0, firstTokenMs = 0.0, decodeMs = 0.0
    var generatedTokens = 0
    var decodeTokensPerSecond: Double { decodeMs > 0 && generatedTokens > 1 ? Double(generatedTokens - 1) / (decodeMs / 1000) : 0 }
    var promptTokensPerSecond: Double { prefillMs > 0 ? Double(max(0, promptTokens - reusedTokens)) / (prefillMs / 1000) : 0 }
}

/// One llama.cpp context. It remembers which tokens its KV cache holds (`cached`), so a prompt that starts with the same tokens reads
/// only what is new: the page in front is read once, ahead of the question, and at end of speech only the listener's words are.
final class LlamaEngine: @unchecked Sendable {
    let weights: LlamaWeights
    let ctx: OpaquePointer
    var model: OpaquePointer { weights.model }
    var vocab: OpaquePointer { weights.vocab }
    private let lock = NSLock()
    private var cached: [Int32] = []
    init(weights: LlamaWeights, threads: Int32, contextTokens: UInt32) throws {
        self.weights = weights
        var cp = llama_context_default_params()
        cp.n_ctx = contextTokens; cp.n_batch = min(contextTokens, 512); cp.n_ubatch = min(contextTokens, 512)
        cp.n_threads = threads; cp.n_threads_batch = threads
        guard let c = llama_init_from_model(weights.model, cp) else { throw LabError.unavailable("llama.cpp could not create a context") }
        ctx = c
    }
    deinit { llama_free(ctx) }

    func tokenize(_ s: String, special: Bool) -> [Int32] {
        guard !s.isEmpty else { return [] }
        var toks = [Int32](repeating: 0, count: s.utf8.count + 16)
        var n = llama_tokenize(vocab, s, Int32(s.utf8.count), &toks, Int32(toks.count), false, special)
        if n < 0 { toks = [Int32](repeating: 0, count: Int(-n)); n = llama_tokenize(vocab, s, Int32(s.utf8.count), &toks, Int32(toks.count), false, special) }
        return Array(toks.prefix(Int(max(0, n))))
    }

    /// Forget the cache (a cold start: setup measures cold and warm).
    func resetCache() { lock.lock(); llama_memory_clear(llama_get_memory(ctx), true); cached = []; lock.unlock() }

    /// Make the KV cache hold exactly `tokens[..<tokens.count - 1]`-or-more of this prompt: keep what already matches, drop the rest, read
    /// the new tokens in chunks. Caller holds the lock. `shouldStop` is asked between chunks (cooperative preemption); the cache then
    /// holds what was read. Returns the number of tokens that were already cached.
    private func ensure(_ tokens: [Int32], keepLast: Int = 1, shouldStop: (() -> Bool)? = nil) throws -> Int {
        var common = 0
        let limit = max(0, tokens.count - keepLast)           // the last token is always decoded: it yields the logits
        while common < min(cached.count, limit), cached[common] == tokens[common] { common += 1 }
        // Always drop everything after the matching prefix: the KV memory also holds the tokens the last reply generated, which
        // `cached` does not list.
        if !llama_memory_seq_rm(llama_get_memory(ctx), 0, Int32(common), -1) {            // a memory that cannot drop a tail (recurrent): start over
            llama_memory_clear(llama_get_memory(ctx), true); common = 0
        }
        cached = Array(cached.prefix(common))
        var i = common
        while i < tokens.count {
            if i < limit, let stop = shouldStop, stop() { break }
            let j = min(tokens.count, i + 256)
            var chunk = Array(tokens[i..<j])
            let rc = chunk.withUnsafeMutableBufferPointer { llama_decode(ctx, llama_batch_get_one($0.baseAddress, Int32($0.count))) }
            guard rc == 0 else { throw LabError.unavailable("llama_decode failed (\(rc))") }
            cached.append(contentsOf: chunk)
            i = j
        }
        return common
    }

    /// Read the stable prefix into the cache ahead of a question. Returns tokens in the prefix, tokens now cached, and the time it took.
    /// Stops between chunks when `shouldStop` says so (a turn started, or the page changed); what was read stays.
    func prefill(system: String, userPrefix: String, layout: ChatLayout, shouldStop: @escaping () -> Bool) throws -> (prefix: Int, cachedNow: Int, ms: Double) {
        let toks = tokenize(layout.prefix(system: system, userPrefix: userPrefix), special: true)
        let t0 = nowMs()
        var common = 0
        lock.lock()
        let limit = toks.count
        while common < min(cached.count, limit), cached[common] == toks[common] { common += 1 }
        if !llama_memory_seq_rm(llama_get_memory(ctx), 0, Int32(common), -1) { llama_memory_clear(llama_get_memory(ctx), true); common = 0 }
        cached = Array(cached.prefix(common))
        lock.unlock()
        var i = common
        while i < toks.count, !shouldStop() {
            lock.lock()
            // A turn that ran between two chunks changed the cache: this prefill is stale and stops.
            if cached.count != i { lock.unlock(); break }
            let j = min(toks.count, i + 64)
            var chunk = Array(toks[i..<j])
            let rc = chunk.withUnsafeMutableBufferPointer { llama_decode(ctx, llama_batch_get_one($0.baseAddress, Int32($0.count))) }
            if rc == 0 { cached.append(contentsOf: chunk) }
            lock.unlock()
            guard rc == 0 else { throw LabError.unavailable("llama_decode failed (\(rc))") }
            i = j
        }
        return (toks.count, i, nowMs() - t0)
    }

    /// A completion with thinking off. The prompt is the stable prefix (reused when cached) plus the suffix. `onPiece` receives each
    /// piece of text as it is produced (whole UTF-8 characters only). Stops at the model's end token (no token budget: the only limit
    /// is the context window), when the task is cancelled, or, with `stopAtSentence`, at the end of the first sentence.
    @discardableResult
    func generate(system: String, userPrefix: String, userSuffix: String, layout: ChatLayout, temperature: Double, stopAtSentence: Bool = true, measureTokens: Int? = nil,
                  onPiece: ((String) -> Void)? = nil, onStats: ((LlamaStats) -> Void)? = nil) throws -> String {
        let prefixToks = tokenize(layout.prefix(system: system, userPrefix: userPrefix), special: true)
        let suffixToks = tokenize(layout.suffix(userSuffix: userSuffix), special: true)
        let toks = prefixToks + suffixToks
        guard !toks.isEmpty else { throw LabError.badInput("empty prompt") }
        lock.lock(); defer { lock.unlock() }
        var stats = LlamaStats(promptTokens: toks.count, prefixTokens: prefixToks.count)
        let t0 = nowMs()
        stats.reusedTokens = try ensure(toks)
        stats.prefillMs = nowMs() - t0
        let smpl = llama_sampler_chain_init(llama_sampler_chain_default_params())!
        defer { llama_sampler_free(smpl) }       // frees the samplers added to the chain with it
        if temperature <= 0 { llama_sampler_chain_add(smpl, llama_sampler_init_greedy()) }
        else {
            llama_sampler_chain_add(smpl, llama_sampler_init_top_k(40))
            llama_sampler_chain_add(smpl, llama_sampler_init_temp(Float(temperature)))
            llama_sampler_chain_add(smpl, llama_sampler_init_dist(UInt32.random(in: 0...UInt32.max)))
        }
        onStats?(stats)
        var all = ""
        var pending = Data()
        let window = Int(llama_n_ctx(ctx))
        var used = toks.count
        let g0 = nowMs()
        while used < window {
            used += 1
            if Task.isCancelled { break }
            if let m = measureTokens, stats.generatedTokens >= m { break }      // a measurement, not speech: setup and the proof stop here
            var tok = llama_sampler_sample(smpl, ctx, -1)
            if stats.generatedTokens == 0 { stats.firstTokenMs = nowMs() - t0 }
            stats.generatedTokens += 1
            if llama_vocab_is_eog(vocab, tok) { break }
            var buf = [CChar](repeating: 0, count: 64)
            let n = llama_token_to_piece(vocab, tok, &buf, Int32(buf.count), 0, false)
            if n > 0 { pending.append(contentsOf: buf.prefix(Int(n)).map { UInt8(bitPattern: $0) }) }
            if let text = String(data: pending, encoding: .utf8), !text.isEmpty {      // a piece ends on a whole character
                pending = Data()
                all += text
                onPiece?(text)
                if stopAtSentence, all.contains(where: { ".!?\n".contains($0) }) { break }
            }
            let rc2 = withUnsafeMutablePointer(to: &tok) { llama_decode(ctx, llama_batch_get_one($0, 1)) }
            if rc2 != 0 { throw LabError.unavailable("llama_decode failed (\(rc2))") }
        }
        stats.decodeMs = nowMs() - g0
        onStats?(stats)
        // The generated tokens stay in the KV memory beyond `cached`; the next prompt drops them (it matches only `cached`).
        return all
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
        llama_memory_clear(llama_get_memory(ctx), true); cached = []
        let rc = toks.withUnsafeMutableBufferPointer { llama_decode(ctx, llama_batch_get_one($0.baseAddress, Int32($0.count))) }
        guard rc == 0, let logits = llama_get_logits_ith(ctx, -1) else { throw LabError.unavailable("llama_decode failed (\(rc))") }
        let raw = ids.map { logits[Int($0)] }
        let mx = raw.max() ?? 0
        let ex = raw.map { expf($0 - mx) }, sum = ex.reduce(0, +)
        return ex.map { $0 / sum }
    }
}

/// The phone's language model as the kit sees it: the voice that talks while the desk works. It uses the responder setup chose
/// (`ResponderChoice`), in a context of its own, loads a file already on the phone and never starts a download from here (setup
/// downloads; start-up warms).
public final class LlamaLocalModel: LocalLanguageModel, @unchecked Sendable {
    public static let shared = LlamaLocalModel()

    // Slots by model id: one responder engine per candidate that has been loaded (setup loads several to measure them).
    private static let slotLock = NSLock()
    nonisolated(unsafe) private static var slots: [String: ModelSlot<LlamaEngine>] = [:]
    static func slot(for id: String) -> ModelSlot<LlamaEngine> {
        slotLock.lock(); defer { slotLock.unlock() }
        if let s = slots[id] { return s }
        let s = ModelSlot<LlamaEngine>(id); slots[id] = s; return s
    }
    static func engine(_ id: String, download: Bool, contextTokens: UInt32 = 2048) async throws -> LlamaEngine {
        try await slot(for: id).get {
            let url = try await LlamaFiles.url(id, download: download)
            let e = try LlamaEngine(weights: try LlamaWeights.shared(path: url.path, gpu: true), threads: 4, contextTokens: contextTokens)
            return (e, {})
        }
    }

    private let fixedID: String?
    /// `modelID`: a fixed responder (the proof measures candidates); nil: whichever setup chose.
    public init(modelID: String? = nil) { fixedID = modelID }
    var modelID: String { fixedID ?? ResponderChoice.current }
    public var isLoaded: Bool { Self.slot(for: modelID).isLoaded }

    // The pending-acknowledgement gate is process-wide: a decision (System 1) waits for any responder's pending acknowledgement.
    private static let gate = NSLock()
    nonisolated(unsafe) private static var pending = 0
    nonisolated(unsafe) private static var waiters: [CheckedContinuation<Void, Never>] = []
    nonisolated(unsafe) private static var preempt = false
    private var lastGen: GenerationReport?, lastPre: GenerationReport?

    public func willGenerate() { Self.gate.lock(); Self.pending += 1; Self.preempt = true; Self.gate.unlock() }     // a turn preempts any page reading
    public func generationEnded() {
        Self.gate.lock(); Self.pending = max(0, Self.pending - 1)
        if Self.pending == 0 { Self.preempt = false }
        let go = Self.pending == 0 ? Self.waiters : []
        if Self.pending == 0 { Self.waiters = [] }
        Self.gate.unlock()
        for w in go { w.resume() }
    }
    /// Returns at once when no acknowledgement is pending, else when the last pending one has ended.
    func untilNoAcknowledgementPending() async {
        await withCheckedContinuation { (c: CheckedContinuation<Void, Never>) in
            Self.gate.lock()
            if Self.pending == 0 { Self.gate.unlock(); c.resume() } else { Self.waiters.append(c); Self.gate.unlock() }
        }
    }
    private var preempted: Bool { Self.gate.lock(); defer { Self.gate.unlock() }; return Self.preempt }

    public func warm() async { _ = try? await Self.engine(modelID, download: false) }
    public func lastReport() -> GenerationReport? { Self.gate.lock(); defer { Self.gate.unlock() }; return lastGen }
    public func lastPrefill() -> GenerationReport? { Self.gate.lock(); defer { Self.gate.unlock() }; return lastPre }

    public func prefill(system: String, userPrefix: String) async {
        guard let e = try? await Self.engine(modelID, download: false) else { return }
        let layout = ChatLayout.forModel(modelID)
        let r = try? e.prefill(system: system, userPrefix: userPrefix, layout: layout, shouldStop: { Task.isCancelled || self.preempted })
        if let r {
            Self.gate.lock(); self.lastPre = GenerationReport(promptTokens: r.cachedNow, prefixTokens: r.prefix, reusedTokens: 0, prefillMs: r.ms); Self.gate.unlock()
        }
    }

    public func generate(system: String, user: String, temperature: Double) async throws -> String {
        let e = try await Self.engine(modelID, download: false)
        return try e.generate(system: system, userPrefix: "", userSuffix: user, layout: ChatLayout.forModel(modelID), temperature: temperature)
    }

    /// The streamed completion: pieces as they are produced, until the model ends it. Cancelling the consumer stops the generation
    /// at the next token.
    public func stream(system: String, userPrefix: String, userSuffix: String, temperature: Double) -> AsyncThrowingStream<String, Error> {
        let id = modelID
        return AsyncThrowingStream { c in
            let t = Task.detached { [self] in
                do {
                    let e = try await Self.engine(id, download: false)
                    try e.generate(system: system, userPrefix: userPrefix, userSuffix: userSuffix, layout: ChatLayout.forModel(id), temperature: temperature,
                                   stopAtSentence: false, onPiece: { c.yield($0) },
                                   onStats: { s in
                                       Self.gate.lock()
                                       self.lastGen = GenerationReport(promptTokens: s.promptTokens, prefixTokens: s.prefixTokens, reusedTokens: s.reusedTokens, prefillMs: s.prefillMs)
                                       Self.gate.unlock()
                                   })
                    c.finish()
                } catch { c.finish(throwing: error) }
            }
            c.onTermination = { _ in t.cancel() }
        }
    }
}
#endif
