#include "llama.h"

#include <chrono>
#include <cmath>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <vector>

static double now_s() {
    return std::chrono::duration<double>(std::chrono::steady_clock::now().time_since_epoch()).count();
}

int main(int argc, char ** argv) {
    if (argc < 3) {
        std::fprintf(stderr, "usage: embed-bench model.gguf 'prompt' [threads]\n");
        return 2;
    }
    const int threads = argc > 3 ? std::atoi(argv[3]) : 4;
    const char * prompt = argv[2];
    llama_backend_init();
    const double t0 = now_s();
    llama_model_params mp = llama_model_default_params();
    mp.n_gpu_layers = 0;
    llama_model * model = llama_model_load_from_file(argv[1], mp);
    if (!model) { std::fprintf(stderr, "model load failed\n"); return 1; }
    const double t1 = now_s();
    llama_context_params cp = llama_context_default_params();
    cp.n_ctx = 2048;
    cp.n_batch = 2048;
    cp.n_ubatch = 2048;
    cp.n_threads = threads;
    cp.n_threads_batch = threads;
    cp.embeddings = true;
    cp.pooling_type = LLAMA_POOLING_TYPE_UNSPECIFIED;
    llama_context * ctx = llama_init_from_model(model, cp);
    if (!ctx) { std::fprintf(stderr, "context failed\n"); llama_model_free(model); return 1; }
    const llama_vocab * vocab = llama_model_get_vocab(model);
    const int need = -llama_tokenize(vocab, prompt, std::strlen(prompt), nullptr, 0, true, false);
    if (need <= 0) { std::fprintf(stderr, "tokenize failed: %d\n", need); llama_free(ctx); llama_model_free(model); return 1; }
    std::vector<llama_token> tokens(need);
    const int got = llama_tokenize(vocab, prompt, std::strlen(prompt), tokens.data(), tokens.size(), true, false);
    if (got <= 0) { std::fprintf(stderr, "tokenize failed: %d\n", got); llama_free(ctx); llama_model_free(model); return 1; }
    tokens.resize(got);
    if (static_cast<int>(tokens.size()) > cp.n_ctx) tokens.resize(cp.n_ctx);
    const double t2 = now_s();
    llama_batch_ext * batch = llama_batch_ext_init(ctx);
    for (size_t i = 0; i < tokens.size(); ++i) {
        const int idx = llama_batch_ext_add_token(batch, 0, tokens[i]);
        const llama_pos pos = static_cast<llama_pos>(i);
        if (idx < 0 || !llama_batch_ext_set_pos(batch, idx, &pos) || !llama_batch_ext_set_output_embd(batch, idx, true)) {
            std::fprintf(stderr, "batch setup failed at %zu\n", i);
            llama_batch_ext_free(batch); llama_free(ctx); llama_model_free(model); return 1;
        }
    }
    if (llama_process(ctx, LLAMA_PROCESS_TYPE_DECODE, batch) != 0) {
        std::fprintf(stderr, "decode failed\n");
        llama_batch_ext_free(batch); llama_free(ctx); llama_model_free(model); return 1;
    }
    const double t3 = now_s();
    const float * raw = llama_get_embeddings_seq(ctx, 0);
    const int dim = llama_model_n_embd_out(model);
    if (!raw || dim <= 0) { std::fprintf(stderr, "no embedding\n"); llama_batch_ext_free(batch); llama_free(ctx); llama_model_free(model); return 1; }
    double norm = 0.0;
    for (int i = 0; i < dim; ++i) norm += static_cast<double>(raw[i]) * raw[i];
    norm = std::sqrt(norm);
    std::printf("dim=%d tokens=%zu load_seconds=%.6f prefill_seconds=%.6f raw_norm=%.6f first=%.9g\n",
                dim, tokens.size(), t1 - t0, t3 - t2, norm, raw[0]);
    llama_batch_ext_free(batch);
    llama_free(ctx);
    llama_model_free(model);
    llama_backend_free();
    return 0;
}
