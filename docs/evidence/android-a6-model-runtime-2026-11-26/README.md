# The APK's compiled model runtime, run on the Motorola (2026-11-26)

Device: Motorola `ZY22H5K8QR`, moto g stylus 5G (2023), Android 14 (API 34),
SM6450. Package `news.choir.autoradio.lab.one`, versionName 1.0, versionCode
474790. The installed APK was pulled back and its SHA-256 read: `977dfbeb…333ce6`
(91,475,679 bytes).

**What was run.** `scripts/proofs/android-a6-model-runtime.sh` pulls that APK,
extracts `lib/arm64-v8a`, checks every library is AArch64 with `LOAD` segments
at `0x4000` (16 KiB), then runs the APK's own Kotlin/JNI:

- `choir.autoradio.NativeModels` (loaded from the APK's dex under `app_process`,
  with `libautoradio_models.so` and its llama.cpp dependencies from that same
  `lib/arm64-v8a`) generated text from Qwen3-0.6B Q4_K_M and embedded text with
  EmbeddingGemma 2 Q8_0;
- the APK's packaged `libonnxruntime.so` ran Silero v6.2.1 op15 through the ONNX
  Runtime C API on the repository's VAD fixture
  (`docs/evidence/model-bench-android-2026-11-26/silero-v6.2.1/output.wav`,
  3.458 s of 16 kHz mono speech, converted to float32 by the script). It used the
  same 512-sample windows, 64-sample context and `[2,1,128]` state as
  `SileroVad.kt`; that Kotlin class and its iterator are not themselves executed
  by this proof.

No test switch ships in the app and nothing is installed by the proof. The
runtime was compiled from llama.cpp source at the pinned `b12497` commit with
NDK r27d and `-Wl,-z,max-page-size=16384`
(`docs/problems/android-native-runtime-archive-2026-11-26.md` explains why the
official prebuilt archive is not used).

## Packaged libraries

From `packaged-libs.txt` (the pulled APK's `lib/arm64-v8a`):

| Library | SHA-256 (packaged, stripped) | LOAD |
| --- | --- | --- |
| `libautoradio_models.so` | `78b7ecd7164db715d44f9c18ade7ea19de5f40f1c4d3875e43b09b6b25fc0a6f` | `0x4000` |
| `libllama.so` | `3a3bf8e63269e79e465c3b7ff8955074191c986ddd71edd57be7d0a5edd6b857` | `0x4000` |
| `libggml.so` | `c426f8dde72011926c8115bd391e6507224566ac519b677d858cf637c540d493` | `0x4000` |
| `libggml-base.so` | `0f488cb6bc88e66d248357353e5eeb4bffbb249721c9e26442e9abade5d9269a` | `0x4000` |
| `libggml-cpu.so` | `98380b0edf38e7f388b37b3195c15733cfedbc7868a199f29449e264a1a1d8b8` | `0x4000` |
| `libonnxruntime.so` | `580c8a2e25cde963e753dac1d0d3f52467016b7f4a52ce11a5d2800a0a1a3150` | `0x4000` |
| `libonnxruntime4j_jni.so` | `8b3d9427a9013a265987668b36332fa69967a761e90295c7bdca9a41c97c56bd` | `0x4000` |

The build tree's unstripped libraries and their digests are in
`mobile/exact-package/android/jniLibs.receipt.json` (gitignored; not part of
this record).

## Results

Qwen3-0.6B Q4_K_M (`qwen3-q4.txt`): load 1,910 ms, prompt 36 tokens, prefill
219.9 ms (164 tok/s), first token 223.0 ms, 8 generated tokens, decode 309.1 ms
(22.6 tok/s), output "The capital of France is Paris.", peak RSS 1,118,160 KiB.

EmbeddingGemma 2 Q8_0 (`embeddinggemma-q8.txt`): load 1,369 ms, 768 dimensions,
`norm=1.000000`, `first=-0.0015134859`, 104 ms, peak RSS 633,388 KiB.

Silero v6.2.1 op15 (`silero-vad.txt`): 108 windows (512 samples each), 100 with
probability ≥ 0.5, minimum 0.015065, mean 0.932352, maximum 0.999999, 43.040 ms,
peak RSS 56,628 KiB.

The earlier benchmark (`docs/evidence/model-bench-android-2026-11-26/`) measured
the generic archive runtime at 204.8 prefill / 36.0 decode tok/s for Q4_K_M.
The APK's number is lower and is not directly comparable: it includes this
runtime's own batching, a single short prompt, a cold model load and the JNI
boundary, and the APK's libraries are stripped at packaging. It is the first
in-APK number and replaces no measured row.

## What this does not prove

The runtime is executed through the APK's dex and its own JNI, not through a
shared page or a voice turn. `application`/`app_process` is not the normal app
process: `AutoradioModule`, `ModelDownloadManager`, `NativeModelSelfTest`, the
`System.loadLibrary("autoradio_models")` init that the app uses at first call,
and `NativeModels.cancel()`/`close()` are compiled and reviewed but not driven
in-app here. No microphone capture, ASR, speaker check, TTS, search indexing or
real hands-free turn is exercised; Tiny Voca still refuses. The rate above is
one short prompt, not a sustained or thermal run. iOS and the web are unchanged.
