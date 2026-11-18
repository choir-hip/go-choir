import Foundation
import llama

/// Route A: the official GGUF through llama.cpp (the release b12259 xcframework). Mean pooling is the file's own setting.
final class LlamaEmbedder: TrialEmbedder {
    let name = "gguf-llama.cpp-b12259"
    let model: OpaquePointer, vocab: OpaquePointer, ctx: OpaquePointer
    let dim: Int
    let loadMs: Double
    var batch: llama_batch
    var rawNormMin: Float = .infinity, rawNormMax: Float = 0

    init(path: String, gpu: Bool, threads: Int32, maxTokens: UInt32 = 2048) throws {
        let t0 = nowMs()
        llama_backend_init()
        var mp = llama_model_default_params()
        mp.n_gpu_layers = gpu ? 99 : 0
        guard let m = llama_model_load_from_file(path, mp) else { throw NSError(domain: "llama", code: 1, userInfo: [NSLocalizedDescriptionKey: "llama.cpp could not load \(path)"]) }
        model = m; vocab = llama_model_get_vocab(m)!
        var cp = llama_context_default_params()
        cp.n_ctx = maxTokens; cp.n_batch = maxTokens; cp.n_ubatch = maxTokens
        cp.n_threads = threads; cp.n_threads_batch = threads
        cp.embeddings = true
        cp.pooling_type = LLAMA_POOLING_TYPE_UNSPECIFIED     // the file says mean
        guard let c = llama_init_from_model(m, cp) else { throw NSError(domain: "llama", code: 2, userInfo: [NSLocalizedDescriptionKey: "llama.cpp could not create a context"]) }
        ctx = c
        dim = Int(llama_model_n_embd_out(m))
        batch = llama_batch_init(Int32(maxTokens), 0, 1)
        loadMs = nowMs() - t0
    }
    var poolingName: String { "\(llama_pooling_type(ctx).rawValue)" }
    deinit { llama_batch_free(batch); llama_free(ctx); llama_model_free(model) }

    func tokens(_ s: String) -> [Int32] {
        var toks = [Int32](repeating: 0, count: s.utf8.count + 8)
        var n = llama_tokenize(vocab, s, Int32(s.utf8.count), &toks, Int32(toks.count), true, false)   // add bos and eos (the file says so); no special parsing
        if n < 0 { toks = [Int32](repeating: 0, count: Int(-n)); n = llama_tokenize(vocab, s, Int32(s.utf8.count), &toks, Int32(toks.count), true, false) }
        return Array(toks.prefix(Int(max(0, n))))
    }

    func embed(_ text: String) async throws -> (vec: [Float], tokens: Int) {
        var toks = tokens(text)
        if toks.count > Int(llama_n_ctx(ctx)) { toks = Array(toks.prefix(Int(llama_n_ctx(ctx)))) }
        batch.n_tokens = Int32(toks.count)
        for (i, t) in toks.enumerated() {
            batch.token[i] = t; batch.pos[i] = Int32(i); batch.n_seq_id[i] = 1; batch.seq_id[i]![0] = 0; batch.logits[i] = 1
        }
        if let mem = llama_get_memory(ctx) { llama_memory_clear(mem, true) }
        let rc = llama_decode(ctx, batch)
        guard rc == 0 else { throw NSError(domain: "llama", code: Int(rc), userInfo: [NSLocalizedDescriptionKey: "llama_decode failed (\(rc))"]) }
        guard let p = llama_get_embeddings_seq(ctx, 0) else { throw NSError(domain: "llama", code: 3, userInfo: [NSLocalizedDescriptionKey: "no pooled embedding"]) }
        // The file's graph mean-pools but does not normalise (raw norms are in the hundreds); the caller of this route normalises, as llama.cpp's own server does.
        let raw = Array(UnsafeBufferPointer(start: p, count: dim))
        let n = norm(raw); rawNormMin = min(rawNormMin, n); rawNormMax = max(rawNormMax, n)
        return (raw.map { $0 / n }, toks.count)
    }
}
