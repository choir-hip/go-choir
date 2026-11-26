// Silero v6.2.1 op15 through the ONNX Runtime shipped in the Autoradio APK.
// The packaged libonnxruntime.so is loaded from the installed app's lib/arm64-v8a;
// the same 512-sample windows, 64-sample context and [2,1,128] state as SileroVad.kt.
#include "onnxruntime_c_api.h"

#include <chrono>
#include <cmath>
#include <cstdint>
#include <cstdio>
#include <cstring>
#include <fstream>
#include <string>
#include <vector>

namespace {

constexpr int kSampleRate = 16000;
constexpr int kWindow = 512;
constexpr int kContext = 64;
constexpr int kStateRows = 2;
constexpr int kStateWidth = 128;

struct Wav {
    std::vector<float> samples;
};

// The proof script converts the repository's 16 kHz mono WAV fixture to little-endian float32,
// so this probe reads one format and cannot silently resample or reinterpret samples.
bool read_f32(const char * path, Wav & out) {
    std::ifstream file(path, std::ios::binary | std::ios::ate);
    if (!file) return false;
    const std::streamsize bytes = file.tellg();
    if (bytes <= 0 || bytes % 4 != 0) return false;
    file.seekg(0);
    std::vector<unsigned char> raw(static_cast<size_t>(bytes));
    if (!file.read(reinterpret_cast<char *>(raw.data()), bytes)) return false;
    out.samples.resize(raw.size() / 4);
    for (size_t i = 0; i < out.samples.size(); ++i) {
        const uint32_t bits = static_cast<uint32_t>(raw[i * 4]) | (static_cast<uint32_t>(raw[i * 4 + 1]) << 8) |
                              (static_cast<uint32_t>(raw[i * 4 + 2]) << 16) | (static_cast<uint32_t>(raw[i * 4 + 3]) << 24);
        float value = 0;
        std::memcpy(&value, &bits, sizeof(value));
        if (!std::isfinite(value) || value < -1.0f || value > 1.0f) return false;
        out.samples[i] = value;
    }
    return true;
}

double milliseconds(std::chrono::steady_clock::time_point start) {
    return std::chrono::duration<double, std::milli>(std::chrono::steady_clock::now() - start).count();
}

std::string failure(const OrtApi * api, OrtStatus * status) {
    if (!status) return "no status";
    std::string message = api->GetErrorMessage(status);
    api->ReleaseStatus(status);
    return message;
}

}  // namespace

int main(int argc, char ** argv) {
    if (argc < 3) {
        std::fprintf(stderr, "usage: silero-ort model.onnx input.wav\n");
        return 2;
    }
    Wav wav;
    if (!read_f32(argv[2], wav)) {
        std::fprintf(stderr, "input must be little-endian float32 samples in [-1,1]\n");
        return 2;
    }
    const OrtApi * api = OrtGetApiBase()->GetApi(ORT_API_VERSION);
    if (!api) { std::fprintf(stderr, "no ORT API %d\n", ORT_API_VERSION); return 1; }
    OrtEnv * env = nullptr;
    OrtSessionOptions * options = nullptr;
    OrtSession * session = nullptr;
    OrtMemoryInfo * memory = nullptr;
    OrtStatus * status = nullptr;
    status = api->CreateEnv(ORT_LOGGING_LEVEL_WARNING, "autoradio-a6", &env);
    if (!status) status = api->CreateSessionOptions(&options);
    if (!status) status = api->SetIntraOpNumThreads(options, 1);
    if (!status) status = api->SetSessionGraphOptimizationLevel(options, ORT_ENABLE_ALL);
    if (!status) status = api->CreateSession(env, argv[1], options, &session);
    if (!status) status = api->CreateCpuMemoryInfo(OrtArenaAllocator, OrtMemTypeDefault, &memory);
    if (status) { std::fprintf(stderr, "setup failed: %s\n", failure(api, status).c_str()); return 1; }

    const char * input_names[] = {"input", "state", "sr"};
    const char * output_names[] = {"output", "stateN"};
    std::vector<float> context(kContext, 0.0f);
    std::vector<float> state(kStateRows * kStateWidth, 0.0f);
    std::vector<float> input(kContext + kWindow);
    int64_t input_dims[] = {1, kContext + kWindow};
    int64_t state_dims[] = {kStateRows, 1, kStateWidth};
    const int64_t rate = kSampleRate;

    const auto started = std::chrono::steady_clock::now();
    double minimum = 1.0, maximum = 0.0, total = 0.0;
    int windows = 0, speech = 0;
    for (size_t offset = 0; offset + kWindow <= wav.samples.size(); offset += kWindow) {
        std::memcpy(input.data(), context.data(), kContext * sizeof(float));
        std::memcpy(input.data() + kContext, wav.samples.data() + offset, kWindow * sizeof(float));
        std::memcpy(context.data(), wav.samples.data() + offset + kWindow - kContext, kContext * sizeof(float));

        OrtValue * input_tensor = nullptr;
        OrtValue * state_tensor = nullptr;
        OrtValue * rate_tensor = nullptr;
        OrtValue * outputs[2] = {nullptr, nullptr};
        status = api->CreateTensorWithDataAsOrtValue(memory, input.data(), input.size() * sizeof(float), input_dims, 2, ONNX_TENSOR_ELEMENT_DATA_TYPE_FLOAT, &input_tensor);
        if (!status) status = api->CreateTensorWithDataAsOrtValue(memory, state.data(), state.size() * sizeof(float), state_dims, 3, ONNX_TENSOR_ELEMENT_DATA_TYPE_FLOAT, &state_tensor);
        if (!status) status = api->CreateTensorWithDataAsOrtValue(memory, const_cast<int64_t *>(&rate), sizeof(rate), nullptr, 0, ONNX_TENSOR_ELEMENT_DATA_TYPE_INT64, &rate_tensor);
        const OrtValue * inputs[] = {input_tensor, state_tensor, rate_tensor};
        if (!status) status = api->Run(session, nullptr, input_names, inputs, 3, output_names, 2, outputs);
        if (status) { std::fprintf(stderr, "run failed: %s\n", failure(api, status).c_str()); return 1; }
        float * probability = nullptr;
        float * next = nullptr;
        api->GetTensorMutableData(outputs[0], reinterpret_cast<void **>(&probability));
        api->GetTensorMutableData(outputs[1], reinterpret_cast<void **>(&next));
        const float p = probability[0];
        if (!std::isfinite(p) || p < 0.0f || p > 1.0f) { std::fprintf(stderr, "invalid probability\n"); return 1; }
        std::memcpy(state.data(), next, state.size() * sizeof(float));
        minimum = std::fmin(minimum, p);
        maximum = std::fmax(maximum, p);
        total += p;
        if (p >= 0.5f) ++speech;
        ++windows;
        for (int i = 0; i < 3; ++i) if (inputs[i]) api->ReleaseValue(const_cast<OrtValue *>(inputs[i]));
        for (int i = 0; i < 2; ++i) if (outputs[i]) api->ReleaseValue(outputs[i]);
    }
    long peak_kb = 0;
    if (std::FILE * status_file = std::fopen("/proc/self/status", "r")) {
        char line[256];
        while (std::fgets(line, sizeof(line), status_file)) {
            if (std::strncmp(line, "VmHWM:", 6) == 0) peak_kb = std::strtol(line + 6, nullptr, 10);
        }
        std::fclose(status_file);
    }
    std::printf("windows=%d speech_windows=%d probability_min=%.6f mean=%.6f max=%.6f total_ms=%.3f vmhwm_kb=%ld\n",
                windows, speech, minimum, windows ? total / windows : 0.0, maximum, milliseconds(started), peak_kb);
    return 0;
}
