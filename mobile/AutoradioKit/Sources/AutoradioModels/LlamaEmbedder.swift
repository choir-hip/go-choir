import Foundation
import AutoradioKit
#if canImport(llama)
import llama

/// The phone's embedder: EmbeddingGemma 2 (Q8_0 GGUF) through the same llama.cpp b12259 the language models use (docs/reference/EMBEDDINGGEMMA.md).
/// The file is fetched at first use into the model store and checked by SHA-256; nothing is in the bundle. One context, one decode at a time.
/// The graph mean-pools but returns vectors of norm in the hundreds, so this normalises (llama.cpp's own server does the same).
public final class LlamaEmbedder: Embedder, QueryEmbedding, @unchecked Sendable {
    public static let modelID = KnownModels.embeddingGemma2.id
    public let engine = KnownModels.embeddingEngine
    public let dimension = 768
    /// Longest prompt the context takes, in tokens (the card says retrieval passages should be 512 or fewer; the model reads 2,048).
    public static let maxTokens: UInt32 = 2048
    static let slot = ModelSlot<LlamaEmbedder>(modelID)

    private let weights: LlamaWeights
    private let ctx: OpaquePointer
    private var batch: llama_batch
    private let lock = NSLock()
    public private(set) var rawNormRange: (min: Float, max: Float) = (.infinity, 0)

    init(weights: LlamaWeights, threads: Int32) throws {
        self.weights = weights
        var cp = llama_context_default_params()
        cp.n_ctx = Self.maxTokens; cp.n_batch = Self.maxTokens; cp.n_ubatch = Self.maxTokens
        cp.n_threads = threads; cp.n_threads_batch = threads
        cp.embeddings = true
        cp.pooling_type = LLAMA_POOLING_TYPE_UNSPECIFIED          // the file says mean
        guard let c = llama_init_from_model(weights.model, cp) else { throw LabError.unavailable("llama.cpp could not create an embedding context") }
        ctx = c
        batch = llama_batch_init(Int32(Self.maxTokens), 0, 1)
        guard Int(llama_model_n_embd_out(weights.model)) == dimension else { throw LabError.unavailable("the embedding model is not 768-dimensional") }
    }
    deinit { llama_batch_free(batch); llama_free(ctx) }

    /// The process's embedder: loads (downloading the file first when `download`) once, then reuses it.
    public static func shared(download: Bool, threads: Int32 = 4, gpu: Bool = true) async throws -> LlamaEmbedder {
        try await slot.get {
            let url = try await LlamaFiles.url(modelID, download: download)
            let e = try LlamaEmbedder(weights: try LlamaWeights.shared(path: url.path, gpu: gpu), threads: threads)
            return (e, {})
        }
    }

    func tokens(_ s: String) -> [Int32] {
        var toks = [Int32](repeating: 0, count: s.utf8.count + 8)
        var n = llama_tokenize(weights.vocab, s, Int32(s.utf8.count), &toks, Int32(toks.count), true, false)     // the file adds <bos> and <eos>
        if n < 0 { toks = [Int32](repeating: 0, count: Int(-n)); n = llama_tokenize(weights.vocab, s, Int32(s.utf8.count), &toks, Int32(toks.count), true, false) }
        return Array(toks.prefix(Int(max(0, n))))
    }

    /// One prompt, its unit vector and its token count.
    func embedOne(_ text: String) throws -> (vec: [Float], tokens: Int) {
        lock.lock(); defer { lock.unlock() }
        var toks = tokens(text)
        if toks.count > Int(Self.maxTokens) { toks = Array(toks.prefix(Int(Self.maxTokens))) }
        guard !toks.isEmpty else { throw LabError.badInput("empty text") }
        batch.n_tokens = Int32(toks.count)
        for (i, t) in toks.enumerated() { batch.token[i] = t; batch.pos[i] = Int32(i); batch.n_seq_id[i] = 1; batch.seq_id[i]![0] = 0; batch.logits[i] = 1 }
        if let mem = llama_get_memory(ctx) { llama_memory_clear(mem, true) }
        let rc = llama_decode(ctx, batch)
        guard rc == 0 else { throw LabError.unavailable("llama_decode failed (\(rc))") }
        guard let p = llama_get_embeddings_seq(ctx, 0) else { throw LabError.unavailable("no pooled embedding") }
        let raw = Array(UnsafeBufferPointer(start: p, count: dimension))
        let n = raw.reduce(Float(0)) { $0 + $1 * $1 }.squareRoot()
        guard n > 0, n.isFinite else { throw LabError.unavailable("the embedding is empty") }
        rawNormRange = (min(rawNormRange.min, n), max(rawNormRange.max, n))
        return (raw.map { $0 / n }, toks.count)
    }

    public func embed(_ prompts: [String]) async throws -> [[Float]] { try prompts.map { try embedOne($0).vec } }
    /// One question, at once (the retriever is synchronous).
    public func embedQuery(_ text: String) -> [Float]? { try? embedOne(EmbedPrompt.query(text)).vec }
}
#endif
