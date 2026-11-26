#include <jni.h>
#include "llama.h"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cmath>
#include <cstdint>
#include <cstdio>
#include <cstring>
#include <fstream>
#include <limits>
#include <memory>
#include <mutex>
#include <stdexcept>
#include <string>
#include <unordered_map>
#include <vector>

namespace {

constexpr int32_t kEmbeddingDimension = 768;

struct JavaExceptionThrown {};

// Raises a Java exception and returns. Safe in a catch handler: it never throws a C++ exception.
void raise_java(JNIEnv * env, const char * kind, const std::string & message) {
    jclass type = env->FindClass(kind);
    if (!type) return;
    env->ThrowNew(type, message.c_str());
    env->DeleteLocalRef(type);
}

// For failures inside a function's try body, where the pending Java exception unwinds to its own catch.
[[noreturn]] void throw_java(JNIEnv * env, const char * kind, const std::string & message) {
    raise_java(env, kind, message);
    if (!env->ExceptionCheck()) throw std::runtime_error("JNI could not raise " + std::string(kind));
    throw JavaExceptionThrown{};
}

std::string to_utf8(JNIEnv * env, jstring value) {
    if (!value) return {};
    jclass type = env->GetObjectClass(value);
    if (!type) throw JavaExceptionThrown{};
    jmethodID method = env->GetMethodID(type, "getBytes", "(Ljava/lang/String;)[B");
    if (!method) {
        env->DeleteLocalRef(type);
        throw JavaExceptionThrown{};
    }
    jstring charset = env->NewStringUTF("UTF-8");
    if (!charset) {
        env->DeleteLocalRef(type);
        throw JavaExceptionThrown{};
    }
    jbyteArray bytes = static_cast<jbyteArray>(env->CallObjectMethod(value, method, charset));
    env->DeleteLocalRef(charset);
    env->DeleteLocalRef(type);
    if (!bytes) throw JavaExceptionThrown{};
    const jsize size = env->GetArrayLength(bytes);
    std::string out(static_cast<size_t>(size), '\0');
    if (size) env->GetByteArrayRegion(bytes, 0, size, reinterpret_cast<jbyte *>(out.data()));
    env->DeleteLocalRef(bytes);
    if (env->ExceptionCheck()) throw JavaExceptionThrown{};
    return out;
}

jstring to_java(JNIEnv * env, const std::string & value) {
    std::vector<jchar> units;
    units.reserve(value.size());
    for (size_t i = 0; i < value.size();) {
        const auto lead = static_cast<unsigned char>(value[i]);
        uint32_t cp = lead;
        size_t width = 1;
        if (lead >= 0xc2 && lead <= 0xdf) { cp = lead & 0x1f; width = 2; }
        else if (lead >= 0xe0 && lead <= 0xef) { cp = lead & 0x0f; width = 3; }
        else if (lead >= 0xf0 && lead <= 0xf4) { cp = lead & 0x07; width = 4; }
        else if (lead >= 0x80) throw std::runtime_error("native text is not UTF-8");
        if (i + width > value.size()) throw std::runtime_error("native text ends inside UTF-8");
        for (size_t j = 1; j < width; ++j) {
            const auto next = static_cast<unsigned char>(value[i + j]);
            if ((next & 0xc0) != 0x80) throw std::runtime_error("native text is not UTF-8");
            cp = (cp << 6) | (next & 0x3f);
        }
        if ((width == 2 && cp < 0x80) || (width == 3 && cp < 0x800) ||
            (width == 4 && cp < 0x10000) || cp > 0x10ffff || (cp >= 0xd800 && cp <= 0xdfff)) {
            throw std::runtime_error("native text is not UTF-8");
        }
        if (cp <= 0xffff) units.push_back(static_cast<jchar>(cp));
        else {
            cp -= 0x10000;
            units.push_back(static_cast<jchar>(0xd800 + (cp >> 10)));
            units.push_back(static_cast<jchar>(0xdc00 + (cp & 0x3ff)));
        }
        i += width;
    }
    static const jchar empty = 0;
    jstring result = env->NewString(units.empty() ? &empty : units.data(), static_cast<jsize>(units.size()));
    if (!result) throw JavaExceptionThrown{};
    return result;
}

std::string json_string(const std::string & value) {
    static const char hex[] = "0123456789abcdef";
    std::string out = "\"";
    for (unsigned char c : value) {
        switch (c) {
            case '"': out += "\\\""; break;
            case '\\': out += "\\\\"; break;
            case '\b': out += "\\b"; break;
            case '\f': out += "\\f"; break;
            case '\n': out += "\\n"; break;
            case '\r': out += "\\r"; break;
            case '\t': out += "\\t"; break;
            default:
                if (c < 0x20) {
                    out += "\\u00";
                    out.push_back(hex[c >> 4]);
                    out.push_back(hex[c & 0x0f]);
                } else out.push_back(static_cast<char>(c));
        }
    }
    out.push_back('"');
    return out;
}

std::string number(double value) {
    char buffer[64];
    std::snprintf(buffer, sizeof(buffer), "%.3f", std::isfinite(value) ? value : 0.0);
    return buffer;
}

using Clock = std::chrono::steady_clock;
double milliseconds(const Clock::time_point & start) {
    return std::chrono::duration<double, std::milli>(Clock::now() - start).count();
}

enum class ModelKind { Text, Embedding };

struct ModelState {
    ModelKind kind = ModelKind::Text;
    llama_model * model = nullptr;
    llama_context * context = nullptr;
    const llama_vocab * vocab = nullptr;
    int32_t dimension = 0;
    int32_t batch_size = 256;
    std::atomic<bool> cancelled{false};

    ~ModelState() {
        if (context) llama_free(context);
        if (model) llama_model_free(model);
    }
};

std::mutex heavy_mutex;
std::mutex registry_mutex;
std::unordered_map<jlong, std::shared_ptr<ModelState>> registry;
std::once_flag backend_once;

void quiet_log(enum ggml_log_level, const char *, void *) {}

bool abort_requested(void * data) {
    auto * state = static_cast<ModelState *>(data);
    return state && state->cancelled.load(std::memory_order_acquire);
}

std::shared_ptr<ModelState> state_for(JNIEnv * env, jlong handle) {
    std::lock_guard<std::mutex> lock(registry_mutex);
    auto found = registry.find(handle);
    if (found == registry.end()) throw_java(env, "java/lang/IllegalStateException", "native model handle is not loaded");
    return found->second;
}

void require_regular_file(JNIEnv * env, const std::string & path) {
    if (path.empty() || path.find('\0') != std::string::npos) {
        throw_java(env, "java/lang/IllegalArgumentException", "model path is empty or invalid");
    }
    std::ifstream file(path, std::ios::binary);
    if (!file) throw_java(env, "java/lang/IllegalArgumentException", "model file is not readable");
}

std::vector<llama_token> tokenize(ModelState & state, const std::string & text, bool add_special, bool parse_special) {
    if (text.size() > static_cast<size_t>(std::numeric_limits<int32_t>::max())) {
        throw std::invalid_argument("prompt is too large");
    }
    const int32_t length = static_cast<int32_t>(text.size());
    int32_t count = llama_tokenize(state.vocab, text.data(), length, nullptr, 0, add_special, parse_special);
    if (count == std::numeric_limits<int32_t>::min()) throw std::runtime_error("token count overflow");
    if (count < 0) count = -count;
    std::vector<llama_token> tokens(static_cast<size_t>(count));
    if (count) {
        const int32_t written = llama_tokenize(state.vocab, text.data(), length, tokens.data(), count, add_special, parse_special);
        if (written < 0) throw std::runtime_error("llama_tokenize failed");
        tokens.resize(static_cast<size_t>(written));
    }
    return tokens;
}

int32_t decode(ModelState & state, std::vector<llama_token> & tokens, size_t offset, size_t count) {
    if (state.cancelled.load(std::memory_order_acquire)) return 2;
    const auto batch = llama_batch_get_one(tokens.data() + offset, static_cast<int32_t>(count));
    return llama_decode(state.context, batch);
}

void decode_checked(JNIEnv * env, ModelState & state, std::vector<llama_token> & tokens, size_t offset, size_t count) {
    const int32_t result = decode(state, tokens, offset, count);
    if (state.cancelled.load(std::memory_order_acquire) || result == 2) {
        throw_java(env, "java/lang/InterruptedException", "native model operation cancelled");
    }
    if (result != 0) throw std::runtime_error("llama_decode failed");
}

void clear_memory(ModelState & state) {
    if (llama_memory_t memory = llama_get_memory(state.context)) llama_memory_clear(memory, true);
}

struct Utf8Prefix { size_t valid; bool incomplete; };

// The same acceptance rules as to_java: a byte string it calls valid never throws there. An
// invalid lead byte or a decoded code point that is overlong, a surrogate or above U+10FFFF ends
// the valid prefix at its start; an incomplete sequence at the end is reported as such.
Utf8Prefix valid_utf8_prefix(const std::string & text) {
    for (size_t i = 0; i < text.size();) {
        const auto lead = static_cast<unsigned char>(text[i]);
        if (lead < 0x80) { ++i; continue; }
        size_t width = 0;
        uint32_t cp = 0;
        if (lead >= 0xc2 && lead <= 0xdf) { width = 2; cp = lead & 0x1f; }
        else if (lead >= 0xe0 && lead <= 0xef) { width = 3; cp = lead & 0x0f; }
        else if (lead >= 0xf0 && lead <= 0xf4) { width = 4; cp = lead & 0x07; }
        else return {i, false};
        if (i + width > text.size()) return {i, true};
        for (size_t j = 1; j < width; ++j) {
            const auto next = static_cast<unsigned char>(text[i + j]);
            if ((next & 0xc0) != 0x80) return {i, false};
            cp = (cp << 6) | (next & 0x3f);
        }
        if ((width == 2 && cp < 0x80) || (width == 3 && cp < 0x800) ||
            (width == 4 && cp < 0x10000) || cp > 0x10ffff || (cp >= 0xd800 && cp <= 0xdfff)) {
            return {i, false};
        }
        i += width;
    }
    return {text.size(), false};
}

std::string token_piece(ModelState & state, llama_token token) {
    char buffer[128];
    int32_t count = llama_token_to_piece(state.vocab, token, buffer, sizeof(buffer), 0, false);
    if (count < 0) {
        std::vector<char> large(static_cast<size_t>(-count));
        count = llama_token_to_piece(state.vocab, token, large.data(), static_cast<int32_t>(large.size()), 0, false);
        if (count < 0) throw std::runtime_error("llama_token_to_piece failed");
        return std::string(large.data(), static_cast<size_t>(count));
    }
    return std::string(buffer, static_cast<size_t>(count));
}

struct TokenCallback {
    JNIEnv * env;
    jobject object;
    jmethodID method;
};

void emit_token(TokenCallback & callback, const std::string & piece) {
    if (!callback.object || piece.empty()) return;
    jstring value = to_java(callback.env, piece);
    callback.env->CallVoidMethod(callback.object, callback.method, value);
    callback.env->DeleteLocalRef(value);
    if (callback.env->ExceptionCheck()) throw JavaExceptionThrown{};
}

jstring native_info(JNIEnv * env, jclass) {
    const std::string version = llama_version() ? llama_version() : "";
    return to_java(env, "{\"name\":\"llama.cpp\",\"version\":" + json_string(version) +
                   ",\"commit\":\"3574df84b776611928d9b227bf614be71fbb9849\",\"abi\":1,\"available\":true}");
}

jlong native_load(JNIEnv * env, jclass, jstring java_path, jstring java_kind, jint context_tokens, jint threads, jint batch_size) {
    try {
        const std::string path = to_utf8(env, java_path);
        const std::string kind = to_utf8(env, java_kind);
        require_regular_file(env, path);
        if (kind != "text" && kind != "embedding") throw_java(env, "java/lang/IllegalArgumentException", "unknown native model kind");
        if (context_tokens <= 0 || context_tokens > (1 << 20) || threads <= 0 || threads > 64 || batch_size <= 0 || batch_size > 4096) {
            throw_java(env, "java/lang/IllegalArgumentException", "invalid native model parameters");
        }
        std::lock_guard<std::mutex> heavy(heavy_mutex);
        {
            std::lock_guard<std::mutex> registry_lock(registry_mutex);
            if (!registry.empty()) throw_java(env, "java/lang/IllegalStateException", "another heavyweight model is loaded");
        }
        std::call_once(backend_once, [] { llama_backend_init(); });
        auto state = std::make_shared<ModelState>();
        state->kind = kind == "text" ? ModelKind::Text : ModelKind::Embedding;
        state->batch_size = batch_size;
        auto model_params = llama_model_default_params();
        model_params.n_gpu_layers = 0;
        state->model = llama_model_load_from_file(path.c_str(), model_params);
        if (!state->model) throw std::runtime_error("llama_model_load_from_file failed");
        state->vocab = llama_model_get_vocab(state->model);
        if (!state->vocab) throw std::runtime_error("llama_model_get_vocab failed");
        auto context_params = llama_context_default_params();
        context_params.n_ctx = static_cast<uint32_t>(context_tokens);
        // An embedding model is non-causal: one ubatch must hold the whole passage, since a later
        // decode would pool only its own tokens. A text model keeps the caller's batch.
        const int32_t prompt_batch = state->kind == ModelKind::Embedding
            ? context_tokens : std::min(batch_size, context_tokens);
        context_params.n_batch = static_cast<uint32_t>(prompt_batch);
        context_params.n_ubatch = context_params.n_batch;
        context_params.n_threads = threads;
        context_params.n_threads_batch = threads;
        context_params.embeddings = state->kind == ModelKind::Embedding;
        context_params.pooling_type = LLAMA_POOLING_TYPE_UNSPECIFIED;
        context_params.no_perf = true;
        context_params.abort_callback = abort_requested;
        context_params.abort_callback_data = state.get();
        state->context = llama_init_from_model(state->model, context_params);
        if (!state->context) throw std::runtime_error("llama_init_from_model failed");
        if (state->kind == ModelKind::Embedding) {
            state->dimension = llama_model_n_embd_out(state->model);
            if (state->dimension != kEmbeddingDimension) throw std::runtime_error("embedding model is not 768-dimensional");
        }
        const jlong handle = reinterpret_cast<jlong>(state.get());
        {
            std::lock_guard<std::mutex> registry_lock(registry_mutex);
            registry.emplace(handle, state);
        }
        return handle;
    } catch (const JavaExceptionThrown &) { return 0; }
      catch (const std::invalid_argument & error) { raise_java(env, "java/lang/IllegalArgumentException", error.what()); return 0; }
      catch (const std::exception & error) { raise_java(env, "java/lang/IllegalStateException", error.what()); return 0; }
}

jstring native_generate(JNIEnv * env, jclass, jlong handle, jstring java_prompt, jint max_tokens,
                        jlong seed, jfloat temperature, jint top_k, jfloat top_p, jobject callback_object) {
    try {
        const std::string prompt = to_utf8(env, java_prompt);
        if (prompt.empty()) throw_java(env, "java/lang/IllegalArgumentException", "prompt is empty");
        if (max_tokens <= 0 || max_tokens > (1 << 20) || !std::isfinite(temperature) || temperature < 0 ||
            !std::isfinite(top_p) || top_p <= 0 || top_p > 1 || top_k <= 0) {
            throw_java(env, "java/lang/IllegalArgumentException", "invalid generation parameters");
        }
        auto state = state_for(env, handle);
        if (state->kind != ModelKind::Text) throw_java(env, "java/lang/IllegalStateException", "loaded model does not generate text");
        std::lock_guard<std::mutex> heavy(heavy_mutex);
        state->cancelled.store(false, std::memory_order_release);
        clear_memory(*state);
        auto tokens = tokenize(*state, prompt, false, true);
        const uint32_t window = llama_n_ctx(state->context);
        if (tokens.empty() || tokens.size() >= window) throw_java(env, "java/lang/IllegalArgumentException", "prompt does not fit the context");
        TokenCallback callback{env, callback_object, nullptr};
        jclass callback_class = nullptr;
        if (callback_object) {
            callback_class = env->GetObjectClass(callback_object);
            if (!callback_class) throw JavaExceptionThrown{};
            callback.method = env->GetMethodID(callback_class, "onToken", "(Ljava/lang/String;)V");
            if (!callback.method) {
                env->DeleteLocalRef(callback_class);
                throw JavaExceptionThrown{};
            }
        }
        const auto started = Clock::now();
        size_t offset = 0;
        while (offset < tokens.size()) {
            const size_t count = std::min<size_t>(static_cast<size_t>(state->batch_size), tokens.size() - offset);
            decode_checked(env, *state, tokens, offset, count);
            offset += count;
        }
        const double prefill_ms = milliseconds(started);
        if (state->cancelled.load(std::memory_order_acquire)) {
            if (callback_class) env->DeleteLocalRef(callback_class);
            return to_java(env, "{\"text\":\"\",\"promptTokens\":" + std::to_string(tokens.size()) +
                           ",\"generatedTokens\":0,\"prefillMs\":" + number(prefill_ms) +
                           ",\"firstTokenMs\":0,\"decodeMs\":0,\"tokensPerSecond\":0,\"cancelled\":true}");
        }
        auto chain_params = llama_sampler_chain_default_params();
        chain_params.no_perf = true;
        std::unique_ptr<llama_sampler, decltype(&llama_sampler_free)> sampler(llama_sampler_chain_init(chain_params), &llama_sampler_free);
        if (!sampler) throw std::runtime_error("llama_sampler_chain_init failed");
        if (temperature <= 0) {
            llama_sampler_chain_add(sampler.get(), llama_sampler_init_greedy());
        } else {
            llama_sampler_chain_add(sampler.get(), llama_sampler_init_top_k(top_k));
            llama_sampler_chain_add(sampler.get(), llama_sampler_init_top_p(top_p, 1));
            llama_sampler_chain_add(sampler.get(), llama_sampler_init_temp(temperature));
            llama_sampler_chain_add(sampler.get(), llama_sampler_init_dist(static_cast<uint32_t>(seed)));
        }
        const auto decode_started = Clock::now();
        std::string text;
        std::string pending;
        int generated = 0;
        double first_token_ms = 0;
        size_t used = tokens.size();
        while (generated < max_tokens && used < window && !state->cancelled.load(std::memory_order_acquire)) {
            llama_token token = llama_sampler_sample(sampler.get(), state->context, -1);
            ++generated;
            ++used;
            if (generated == 1) first_token_ms = milliseconds(started);
            if (llama_vocab_is_eog(state->vocab, token)) break;
            pending += token_piece(*state, token);
            while (!pending.empty()) {
                const auto prefix = valid_utf8_prefix(pending);
                if (prefix.incomplete) break;                      // a later token may complete it
                if (prefix.valid == 0) { pending.erase(0, 1); continue; }   // a byte that is not text
                const std::string piece = pending.substr(0, prefix.valid);
                emit_token(callback, piece);
                text += piece;
                pending.erase(0, prefix.valid);
            }
            if (generated >= max_tokens || used >= window || state->cancelled.load(std::memory_order_acquire)) break;
            std::vector<llama_token> next{token};
            decode_checked(env, *state, next, 0, 1);
        }
        const double decode_ms = milliseconds(decode_started);
        const bool cancelled = state->cancelled.load(std::memory_order_acquire);
        const double tokens_per_second = generated > 1 ? static_cast<double>(generated - 1) / (decode_ms / 1000.0) : 0;
        if (callback_class) env->DeleteLocalRef(callback_class);
        return to_java(env, "{\"text\":" + json_string(text) + ",\"promptTokens\":" + std::to_string(tokens.size()) +
                       ",\"generatedTokens\":" + std::to_string(generated) + ",\"prefillMs\":" + number(prefill_ms) +
                       ",\"firstTokenMs\":" + number(first_token_ms) + ",\"decodeMs\":" + number(decode_ms) +
                       ",\"tokensPerSecond\":" + number(tokens_per_second) + ",\"cancelled\":" + (cancelled ? "true" : "false") + "}");
    } catch (const JavaExceptionThrown &) { return nullptr; }
      catch (const std::invalid_argument & error) { raise_java(env, "java/lang/IllegalArgumentException", error.what()); return nullptr; }
      catch (const std::exception & error) { raise_java(env, "java/lang/IllegalStateException", error.what()); return nullptr; }
}

jfloatArray native_embed(JNIEnv * env, jclass, jlong handle, jstring java_prompt) {
    try {
        const std::string prompt = to_utf8(env, java_prompt);
        auto state = state_for(env, handle);
        if (state->kind != ModelKind::Embedding) throw_java(env, "java/lang/IllegalStateException", "loaded model does not embed text");
        std::lock_guard<std::mutex> heavy(heavy_mutex);
        state->cancelled.store(false, std::memory_order_release);
        clear_memory(*state);
        auto tokens = tokenize(*state, prompt, true, false);
        const uint32_t window = llama_n_ctx(state->context);
        if (tokens.empty()) throw_java(env, "java/lang/IllegalArgumentException", "embedding prompt is empty");
        if (tokens.size() > window) throw_java(env, "java/lang/IllegalArgumentException", "embedding prompt does not fit the context");
        // One ubatch, so the pooled vector covers the whole prompt (native_load sizes it for this).
        decode_checked(env, *state, tokens, 0, tokens.size());
        const float * raw = llama_get_embeddings_seq(state->context, 0);
        if (!raw) throw std::runtime_error("llama_get_embeddings_seq failed");
        double squared = 0;
        for (int32_t i = 0; i < state->dimension; ++i) {
            if (!std::isfinite(raw[i])) throw std::runtime_error("embedding contains a non-finite value");
            squared += static_cast<double>(raw[i]) * raw[i];
        }
        const double norm = std::sqrt(squared);
        if (!(norm > 0) || !std::isfinite(norm)) throw std::runtime_error("embedding has zero norm");
        jfloatArray result = env->NewFloatArray(state->dimension);
        if (!result) throw JavaExceptionThrown{};
        std::vector<jfloat> values(static_cast<size_t>(state->dimension));
        for (int32_t i = 0; i < state->dimension; ++i) values[i] = static_cast<jfloat>(raw[i] / norm);
        env->SetFloatArrayRegion(result, 0, state->dimension, values.data());
        if (env->ExceptionCheck()) throw JavaExceptionThrown{};
        return result;
    } catch (const JavaExceptionThrown &) { return nullptr; }
      catch (const std::invalid_argument & error) { raise_java(env, "java/lang/IllegalArgumentException", error.what()); return nullptr; }
      catch (const std::exception & error) { raise_java(env, "java/lang/IllegalStateException", error.what()); return nullptr; }
}

// Cancel and unload may race a finishing call: a handle already gone is a no-op, never a Java
// exception a caller cannot answer.
void native_cancel(JNIEnv *, jclass, jlong handle) {
    std::shared_ptr<ModelState> state;
    {
        std::lock_guard<std::mutex> registry_lock(registry_mutex);
        auto found = registry.find(handle);
        if (found == registry.end()) return;
        state = found->second;
    }
    state->cancelled.store(true, std::memory_order_release);
}

void native_unload(JNIEnv *, jclass, jlong handle) {
    std::lock_guard<std::mutex> heavy(heavy_mutex);
    std::shared_ptr<ModelState> state;
    {
        std::lock_guard<std::mutex> registry_lock(registry_mutex);
        auto found = registry.find(handle);
        if (found == registry.end()) return;
        state = found->second;
        registry.erase(found);
    }
}

jstring native_tts_capabilities(JNIEnv * env, jclass) {
    return to_java(env, "{\"available\":false,\"modelId\":\"tiny-voca\",\"voices\":[\"F2\",\"M2\"],"
                       "\"reason\":\"Tiny Voca synthesis is not implemented on Android.\"}");
}

} // namespace

extern "C" JNIEXPORT jint JNICALL JNI_OnLoad(JavaVM *, void *) {
    llama_log_set(quiet_log, nullptr);
    return JNI_VERSION_1_6;
}

extern "C" JNIEXPORT jstring JNICALL Java_choir_autoradio_NativeBridge_info(JNIEnv * env, jclass type) {
    try { return native_info(env, type); }
    catch (const JavaExceptionThrown &) { return nullptr; }
    catch (const std::exception & error) { raise_java(env, "java/lang/IllegalStateException", error.what()); return nullptr; }
}
extern "C" JNIEXPORT jlong JNICALL Java_choir_autoradio_NativeBridge_load(JNIEnv * env, jclass type, jstring path, jstring kind, jint context_tokens, jint threads, jint batch_size) {
    return native_load(env, type, path, kind, context_tokens, threads, batch_size);
}
extern "C" JNIEXPORT jstring JNICALL Java_choir_autoradio_NativeBridge_generate(JNIEnv * env, jclass type, jlong handle, jstring prompt, jint max_tokens, jlong seed, jfloat temperature, jint top_k, jfloat top_p, jobject callback) {
    return native_generate(env, type, handle, prompt, max_tokens, seed, temperature, top_k, top_p, callback);
}
extern "C" JNIEXPORT jfloatArray JNICALL Java_choir_autoradio_NativeBridge_embed(JNIEnv * env, jclass type, jlong handle, jstring prompt) {
    return native_embed(env, type, handle, prompt);
}
extern "C" JNIEXPORT void JNICALL Java_choir_autoradio_NativeBridge_cancel(JNIEnv * env, jclass type, jlong handle) { native_cancel(env, type, handle); }
extern "C" JNIEXPORT void JNICALL Java_choir_autoradio_NativeBridge_unload(JNIEnv * env, jclass type, jlong handle) { native_unload(env, type, handle); }
extern "C" JNIEXPORT jstring JNICALL Java_choir_autoradio_NativeBridge_ttsCapabilities(JNIEnv * env, jclass type) {
    try { return native_tts_capabilities(env, type); }
    catch (const JavaExceptionThrown &) { return nullptr; }
    catch (const std::exception & error) { raise_java(env, "java/lang/IllegalStateException", error.what()); return nullptr; }
}
