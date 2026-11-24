# On-device model benchmark on old phones: research and plan

Date of research: 2026-11-22. Read-only on the repo. Nothing was installed on the Mac or pushed to a phone.
Only read-only `adb` queries were made. One file was downloaded to the session scratchpad (the sherpa-onnx
Android tarball) and inspected with the NDK's `llvm-readelf` and `strings`; it was not run.
Every claim below carries a status: **verified** (I saw it on the phone or in a file or API reply today),
**doc** (a fetched page, URL cited), or **unverified**.

## 0. What the hardware is (verified, read-only adb)

Moto g stylus 5G (2023), serial ZY22H5K8QR:

- Android 14 (API 34), security patch 2026-05-01, `ro.board.platform=parrot`, `ro.soc.model=SM6450`.
- ABI list `arm64-v8a, armeabi-v7a, armeabi`. Page size 4096 (`getconf PAGESIZE`).
- CPU: 8 cores, 4 x part 0xd05 (Cortex-A55) and 4 x part 0xd41 (Cortex-A78). Features: `asimdhp`, `fphp`
  (fp16 arithmetic) and `asimddp` (int8 dot product). No `i8mm`, no `bf16`, no SVE. So int8 and fp16 CPU
  kernels with dot product are the fast path; i8mm/SME kernels (KleidiAI paths) will not help.
- `MemTotal` 3.59 GB, `MemAvailable` about 1.6 GB at the time of reading (phone idle, screen presumably off).
  This is the binding constraint: a 0.6B int8 transducer (about 630 MB of files) fits, a 2B GGUF does not
  fit comfortably.
- Libraries present in `/system/lib64`: `libc++.so`, `libvulkan.so`; in vendor: `libOpenCL.so`,
  `libGLESv2_adreno.so`, `libcdsprpc.so`, `libadsprpc.so` (the DSP RPC client libraries exist). GPU is
  Adreno (`ro.hardware.vulkan=adreno`; model number not read, expected Adreno 710 for SM6450, unverified).
- `/data/local/tmp` is writable by the shell user (uid 2000) and already holds a directory named `exact`
  (not ours; do not touch). Free space 70 GB.
- Unrooted reads that work (verified): `/sys/class/thermal/thermal_zone*/{type,temp}` (cpu-1-0..7, cpuss-0/1,
  gpuss-0/1, aoss-0, pa, pm6450_tz, nsp: all returned values), `dumpsys thermalservice` (Thermal Status
  integer and per-sensor temperatures including battery, CPU0-7, GPU0-1, ibat, nsp0/1),
  `dumpsys battery`, `/sys/class/power_supply/battery/{current_now,voltage_now,temp}`,
  `/sys/devices/system/cpu/cpu{0,6}/cpufreq/{scaling_cur_freq,scaling_governor}` (governor `walt`).
  Denied: `/sys/class/kgsl/kgsl-3d0/gpuclk` (GPU clock).
- The phone is currently on a charger (`AC powered: true`, level 96): see battery caveats in section 3.

iPhone 14 Pro Max: per `docs/reference/ON-DEVICE-MODELS.md` and `mobile/README.md`, A16, 6 GB, iOS 27.x,
UDID `00008120-00114DE80A90C01E`. `xcrun xctrace list devices` today lists the phone under **Devices
Offline** ("iPhone (27.3)"), so it was not reachable at research time (locked, untrusted or asleep). Xcode
27.1 is installed. NDK 27.3.13750724 is at `~/Library/Android/sdk/ndk/` (verified); that is enough to build
llama.cpp and whisper.cpp for Android. The page for llama.cpp's OpenCL backend asks for NDK 26.3, but its
CPU-only build does not.

## 1. Android: runtimes from an `adb shell` binary, no app

Short version: **sherpa-onnx is the only candidate that publishes ready-to-run Android arm64 command-line
binaries**, and it also carries the models we care about first. Everything else needs the NDK (which we have),
a model export step, or both.

| Runtime | Prebuilt Android arm64 CLI? | Accelerator on SM6450 | Models in, how | Streaming transducer (Parakeet/Nemotron) | Whisper family |
|---|---|---|---|---|---|
| **sherpa-onnx v2.0.3** (2026-11-22) | **Yes (verified).** Release asset `sherpa-onnx-v2.0.3-android-aarch64-termux-shared.tar.bz2` (17.9 MB) holds `bin/` with `sherpa-onnx` (online), `sherpa-onnx-offline`, `sherpa-onnx-vad`, `sherpa-onnx-vad-with-offline-asr`, `sherpa-onnx-vad-with-online-asr`, `sherpa-onnx-offline-tts`, `sherpa-onnx-keyword-spotter`, diarization, denoiser and more; `lib/` holds `libonnxruntime.so` and `libsherpa-onnx-c-api.so`/`cxx-api.so`; `include/` holds the C and C++ API headers. Also a plain `sherpa-onnx-v2.0.3-android.tar.bz2` (48 MB, JNI libs for apps) and static-link variants. | CPU through ONNX Runtime 1.28.x (the `libonnxruntime.so` strings show the version, the NNAPI EP and the XNNPACK EP are compiled in; the binary has a `--provider` option and prints `Available providers` on failure). NNAPI is deprecated since Android 15 ([doc](https://developer.android.com/ndk/guides/neuralnetworks)); on Android 14 it exists but SM6450's DSP is not guaranteed to take our ops. QNN/HTP needs "SM8350 or newer" per [sherpa QNN page](https://k2-fsa.github.io/sherpa/onnx/qnn/index.html) and model binaries per SoC: SM6450 is not on that list, and the prebuilt tarball is not a QNN build. **No Adreno GPU path.** | ONNX only, exported by the sherpa team and hosted on GitHub releases / Hugging Face (`sherpa-onnx-*` model folders with `encoder/decoder/joiner.int8.onnx` + `tokens.txt`). Safetensors/NeMo checkpoints need their export script (`scripts/` in the repo). | Offline Parakeet: yes, `--model-type=nemo_transducer`, models `sherpa-onnx-nemo-parakeet-tdt-0.6b-v2-int8` (English, encoder 622 MB int8), `-v3-int8`, and `...unified-en-0.6b-int8-non-streaming` ([doc](https://k2-fsa.github.io/sherpa/onnx/pretrained_models/offline-transducer/nemo-transducer-models.html)). **Streaming Parakeet/Nemotron: not released.** Open PR #4141 "multilingual Nemotron Labs Unified ASR streaming" (2026-10-29) and issue #4131 are open ([search via GitHub API](https://github.com/k2-fsa/sherpa-onnx/pull/4141)); the pretrained-models index lists streaming only for Zipformer/Conformer/LSTM transducers, Paraformer, and Zipformer-CTC/T-one. | Yes, offline: `whisper/index.html` in the same index; also Moonshine v1 tiny/base and v2 tiny/base, SenseVoice, Cohere Transcribe ([index](https://k2-fsa.github.io/sherpa/onnx/pretrained_models/index.html), [moonshine](https://k2-fsa.github.io/sherpa/onnx/moonshine/index.html)). |
| **ONNX Runtime directly** (1.28.x) | No CLI for ASR. `onnxruntime_perf_test` exists in source but is not a published Android asset I could verify. | XNNPACK (CPU), NNAPI (deprecated), QNN: "built and tested with QNN 2.22.x and SC8280, SM8350, Snapdragon X"; pre-built packages are Windows-only, Android needs a source build ([doc](https://onnxruntime.ai/docs/execution-providers/QNN-ExecutionProvider.html)); SM6450 not mentioned. | ONNX | Only through our own driver code | Same |
| **whisper.cpp v1.9.5** (2026-10-06) | **No** prebuilt Android binary in the release API (assets filter found none). The README points to `examples/whisper.android` (an app) and has no `adb shell` recipe. Build with NDK: `-DANDROID_ABI=arm64-v8a -DANDROID_PLATFORM=android-28` and push (the llama.cpp doc has the pattern; whisper.cpp shares ggml). | CPU (NEON/dotprod). Vulkan backend exists (`-DGGML_VULKAN=1`, [README](https://github.com/ggml-org/whisper.cpp)); whether the Adreno on this phone gives a correct and faster result: **unverified, risk**. Core ML encoder is Apple only. | GGML `.bin` made by their scripts or downloaded; `quantize` tool for q5_0 etc. `whisper-bench` runs the encoder alone; `scripts/bench.py` writes CSV ([README](https://github.com/ggml-org/whisper.cpp)). | No (Whisper only) | Yes, all sizes, quantised |
| **llama.cpp v1.0.0** (2026-11-13) | **No** prebuilt Android asset found by the API filter. Build with NDK `GGML_NATIVE=OFF GGML_OPENMP=OFF`, `android-28`; the doc's adb recipe is `adb push` to `/data/local/tmp/llama.cpp` then `LD_LIBRARY_PATH=lib ./bin/...` ([doc](https://github.com/ggml-org/llama.cpp/blob/master/docs/android.md)). | CPU. OpenCL backend lists verified Adreno 750/810/830/840/X1/X2 only; **Adreno 710 is not listed, and phone-class A6x GPUs are called likely unsupported** ([doc](https://github.com/ggml-org/llama.cpp/blob/master/docs/backend/OPENCL.md)). Treat as CPU only. | GGUF (+ `mmproj` for audio). For LLMs: Qwen3 0.6B/1.7B, LFM2.5 350M/1.2B, Gemma 3 270M, SmolLM etc. as GGUF from Hugging Face. | No | Audio-in models in `llama-mtmd-cli`: Ultravox 0.5 (1B), Voxtral Mini 3B, Qwen3-ASR 0.6B/1.7B, Qwen2.5-Omni, Gemma 4 E2B/E4B ([doc](https://github.com/ggml-org/llama.cpp/blob/master/docs/multimodal.md)). Whisper itself lives in whisper.cpp, not here. Too big for 3.4 GB except Qwen3-ASR-0.6B (Apache-2.0, 0.9B params BF16, LibriSpeech 2.11 / 4.55 offline per [card](https://huggingface.co/Qwen/Qwen3-ASR-0.6B)); the GGUF existence for Qwen3-ASR is implied by the multimodal page, its size and speed unverified. |
| **LiteRT-LM v0.19.1** (2026-11-05) | Only macOS and Apple xcframework assets in the release. Android is a Kotlin API ([repo](https://github.com/google-ai-edge/LiteRT-LM)); a CLI needs a Bazel source build: heavy. v0.19.0 notes add Qwen3-ASR, Kokoro and Qwen3-TTS support (repo page, unverified in detail). | CPU/GPU/Qualcomm NPU claimed in general; no SM6450 statement | `.litertlm` | There is a community **Nemotron Speech Streaming LiteRT** (soniqo): encoder INT8, decoder/joint FP32, 80 ms chunks, first partial 0.42 s, chunk compute 79.9 ms on a server CPU (CCX23), CC-BY-4.0 ([card](https://huggingface.co/soniqo/Nemotron-Speech-Streaming-LiteRT)); that is a ready streaming-Nemotron artefact, but it needs the LiteRT runtime and a driver. | Not checked |
| **ExecuTorch v1.6.0** (2026-11-12) | No release binaries. `parakeet_runner` and `whisper_runner` are built by `make parakeet-cpu` etc. for the host; Android for those is an example app in a separate repo ([parakeet](https://github.com/pytorch/executorch/blob/main/examples/models/parakeet/README.md), [whisper](https://github.com/pytorch/executorch/blob/main/examples/models/whisper/README.md)). | Backends listed for Parakeet: portable, xnnpack, vulkan, metal, mlx, cuda; **no QNN or CoreML mentioned**. | Export with a Python script (torch, torchao) to `.pte` | Offline Parakeet TDT only; no streaming described | Whisper runner lists CUDA, Metal, CPU |
| **MNN 3.6.2** (2026-10-29) | Release has Android CPU/OpenCL/Vulkan libs (zip) and an iOS CPU+Metal zip; tools need a build | Adreno via OpenCL/Vulkan | MNN format converted from ONNX | Not checked | Not checked |
| **ncnn 20261020** | Prebuilt Android libs incl. Vulkan | Vulkan | ncnn params from ONNX/PNNX | Not checked | Not checked |

Reading of the table:

1. **No runtime reaches an NPU on this phone.** The QNN path (sherpa-onnx, ONNX Runtime, ExecuTorch, LiteRT)
   is documented for SM8xxx-class parts and per-SoC compiled model binaries; SM6450 (Snapdragon 6 Gen 1) is on
   none of the lists I found. The DSP libraries are on the phone (`libcdsprpc.so`), so it cannot be ruled out
   by hardware alone, but there is no documented route and building one needs the QNN SDK (a licence click-through).
   **Plan for CPU only**, and treat any accelerator number as a bonus experiment (XNNPACK vs plain CPU
   provider, NNAPI once, whisper.cpp Vulkan once).
2. **The Adreno GPU is documented for none of the speech paths** (llama.cpp OpenCL excludes it; sherpa-onnx has
   no GPU provider on Android). Vulkan in whisper.cpp or ncnn/MNN is the only chance, and each needs its own
   conversion and a correctness check.
3. **Streaming Parakeet/Nemotron on Android command line today**: not in sherpa-onnx (open PR). Options are
   (a) the streaming models sherpa-onnx does ship (English streaming Zipformer 2023-06-26: int8 encoder 68 MB,
   decoder 1.3 MB, joiner 254 KB, [doc](https://k2-fsa.github.io/sherpa/onnx/pretrained_models/online-transducer/zipformer-transducer-models.html);
   WER not given on that page), as the streaming baseline; (b) wait for or build the Nemotron PR; (c) our
   own LiteRT driver with the soniqo export. Order: (a) first.
4. **Nemotron facts** (doc): NVIDIA `nemotron-3.5-asr-streaming-0.6b`: 0.6B, cache-aware FastConformer + RNN-T,
   chunk 80/160/320/560/1120 ms, OpenMDW-1.1 licence "ready for commercial use", formats Safetensors/PyTorch/NeMo
   plus a q8_0 GGUF for a "NeMo-Speech.cpp" runtime (not evaluated), no ONNX on the card; its quoted WER is FLEURS,
   not LibriSpeech ([card](https://huggingface.co/nvidia/nemotron-3.5-asr-streaming-0.6b)). The older
   English-only `nvidia/nemotron-speech-streaming-en-0.6b` exists (search result); card not read.
   Parakeet TDT 0.6B v2: CC-BY-4.0, LibriSpeech greedy WER 1.69 clean / 3.19 other ([the card URL I fetched
   returned 404 under `nvidia/parakeet-tdt-0.6b-v2`; the figures came from the answer of the second fetch of
   a Hugging Face card, so re-confirm on the card before quoting them]).

## 2. iPhone, and what the repo already has

### What the repo has (verified)

- `docs/reference/ON-DEVICE-MODELS.md` (sections 1 to 10) lists the candidate models and says only Silero VAD
  and Parakeet EOU were ever measured on a phone ("RTF 0.035, first load 34 s, 149 MB" for Parakeet EOU 120M
  on the A16, taken before an owner ruling).
- `mobile/README.md` section "On-device model stages" (lines about 295 to 320) holds a stage table with cold
  first load, warm load, resident size and RTF for: silero-vad, parakeet-eou, parakeet-tdt 0.6B v2, kokoro,
  pocket-tts and the llama GGUF stage, measured on an M1 iMac; it explains that Core ML "maps lazily, so loaded
  under-reports until the first prediction".
- `mobile/scripts/proof-models.sh` replays a 3.6 s fixture turn through each stage and prints a line per stage;
  `autoradio-replay --list-stages` lists 28 stages; `autoradio-models` is the SwiftPM executable target
  (`mobile/AutoradioKit/Sources/AutoradioModels`) with FluidAudio 0.18.3 (trait `TTS` only: the Diarizer trait
  is off) and a llama.cpp release binary (macOS only). `mobile/scripts/build-llama-xcframework.sh` and
  `build-xcframeworks.sh` already build iOS slices.
- The lab app can be installed and driven on the phone with `devicectl` (`build-lab.sh ios --device --phone
  <udid>`, then `scripts/exact.sh agent ios --device`); it needs the phone unlocked and kept awake.
- **No benchmark harness exists for phones.** The grep for `benchmark|realtime factor|rtf` over `mobile`
  hit tests and README prose only; the only RTF figures are the README stage table.
- Rules that apply: builds are the lead's, one Mac build at a time through the lock; `scripts/check-autoradio-native-boundary`
  counts Swift/Kotlin under `mobile/AutoradioKit/` and `mobile/exact-package/apple/` and every `.kt` under
  `mobile/`, so the iOS bench host must live **outside** `mobile/` (see section 5), and the harness is a
  developer-side script (docs-like, no CI).

### What can run on the iPhone

An iPhone cannot run a loose command-line binary. The options are a signed host app or an XCTest bundle hosted
by a throwaway app, launched with `xcodebuild test -destination id=<udid>` or `devicectl device process launch`.
The device result comes back as a JSON file (container pulled with `devicectl device copy from`) or an
`xcresult`.

| Route | Reaches the A16 Neural Engine/GPU? | Streaming transducer | Whisper | Status |
|---|---|---|---|---|
| **FluidAudio** (Core ML, Apache-2.0; 0.18.3 pinned in repo) | Yes, ANE | Parakeet EOU 120M streaming (measured by us); README names Nemotron streaming and Nemotron 3.5 Transcribe Lite; TDT v2/v3 batch | Not its job | Existing code; `fluidaudiocli` is macOS only and its documented benchmarks are diarization/VAD, not ASR WER ([README](https://github.com/FluidInference/FluidAudio)). So we write the WER loop ourselves in an XCTest. |
| **sherpa-onnx iOS** | CPU (ONNX Runtime); CoreML provider string exists in the lib | Zipformer streaming | Yes | Repo has `build-ios.sh`, `Package.swift`, `ios-swift` and `ios-swiftui` examples (contents API listing). The v2.0.3 release API shows no iOS xcframework asset, so it must be built (about the cost of the 10-minute FluidAudio slice). Not verified further. |
| **whisper.cpp + CoreML encoder** | ANE for the encoder; "more than 3x faster than CPU" per README | No | Yes | XCFramework + Swift package exist; need `generate-coreml-model.sh` on the Mac (Python, coremltools: installs on the Mac, so owner decision) |
| **llama.cpp xcframework** | Metal GPU | n/a | n/a | Already built in repo (b12259); fits the small LM row |
| **MLX / MLX Swift** | GPU | not checked | not checked | **Not researched**; the repo page says MLX Swift has only LFM2.5-Audio. Leave out of v1. |
| **LiteRT-LM xcframework** | Metal? | Nemotron LiteRT export maybe | not checked | Release has `CLiteRTLM.xcframework.zip` (95 MB); not evaluated. |

Recommended iPhone first row: **FluidAudio Parakeet EOU 120M (streaming) and Parakeet TDT 0.6B v2 (batch)**,
because they are already linked, already measured once on this phone (EOU only), and the harness is a small
XCTest over an existing package.

## 3. Measurement protocol (same on both platforms)

### Audio and references

- **Primary set (licence clean, cheap):** LibriSpeech test-clean and test-other (CC BY 4.0, 346 MB and 328 MB
  downloads, [OpenSLR 12](https://www.openslr.org/12)). Fixed manifest of **100 utterances per condition**
  chosen once by a seeded sample, committed as a list of utterance ids with sha256 of the flac; first run uses
  **10** of them (the first 10 of the same list). Convert to 16 kHz mono 16-bit wav on the Mac (`ffmpeg` or
  `sox`; check they are present, nothing is installed by this plan).
- **Noisy and far-field synthetic:** the same test-clean utterances mixed with MUSAN noise at fixed SNR
  (e.g. 10 and 5 dB) and optionally convolved with a room impulse response. MUSAN is CC BY 4.0 as a corpus
  (11 GB, [OpenSLR 17](https://www.openslr.org/17/)); the page does not say whether the music files carry other
  licences, so take only the `noise` subset and read its per-file notes first. The RIR corpus is OpenSLR 28:
  **licence not fetched**, read before use. Mixing script is Python on the Mac and the mixed wavs are the
  fixture (so the phone does no mixing).
- **The real condition (speech over a playing radio) needs our own recordings.** Plan: the owner (or a helper)
  reads 50 fixed sentences (taken from LibriSpeech transcripts so references exist) next to a speaker playing
  the Autoradio programme at three volumes, recorded on **the phone's own microphone** and on the iPhone's.
  Record once, keep the wavs in the repo's evidence folder only if the speaker consents; the mic path
  (AGC/noise suppression of the phone's audio stack) is part of the real condition and cannot be reproduced by
  the synthetic mix. Until then the synthetic mix is a proxy and must be labelled so.
- **Unseen-data warning:** models trained on LibriSpeech (Parakeet, Zipformer) will look better here than on
  speech over radio; report LibriSpeech as a regression gate, not as the product number.

### Metrics

| Metric | How | Where computed |
|---|---|---|
| WER | Mac script: normalise reference and hypothesis the same way (lowercase, strip punctuation, expand numbers consistently; Whisper's English normaliser or a simple one, but one rule for all models), `jiwer` or a 30-line edit distance. Report corpus WER (total errors / total ref words) with a bootstrap interval over utterances, plus substitutions, deletions, insertions. | Mac, from pulled transcripts |
| Real-time factor | decode wall time / audio duration, per utterance, then median and p90. Use the monotonic clock around the decode only (sherpa-onnx CLIs print `Elapsed seconds` and `Real time factor (RTF)` themselves; a custom driver should print the same). | Phone, parsed on Mac |
| Model load time | wall time from process start to ready, **cold** (first run after the other model evicted page cache) and **warm** (immediately again). Page cache cannot be dropped without root (`echo 3 > /proc/sys/vm/drop_caches` needs root), so "cold" = first run after a different, larger model file was read. Always label it. | Phone |
| Time to first partial (streaming only) | feed audio at real-time pace in chunks (e.g. 100 ms); log time from the first chunk fed to the first non-empty hypothesis, and per-chunk compute time. **The stock `sherpa-onnx` CLI decodes a wav as fast as it can, so it gives RTF but not a real first-partial latency;** a small C++ driver on the C API (headers are in the tarball) is needed (about 100 lines, build with NDK). | Phone driver |
| Final latency | time from last audio chunk fed to the final transcript | Phone driver |
| Peak resident memory | a device-side wrapper starts the process, polls `/proc/<pid>/status` every 20 ms and keeps the last `VmHWM`, `VmRSS`, `RssAnon`, `RssFile` before exit; `/proc/<pid>/smaps_rollup` gives PSS and is readable because the process runs as the shell uid (same uid as the poller). Weights are often mmap'd: report anon and file separately, or the number lies. Cross-check once with `dumpsys meminfo <pid>` while it runs. On iPhone use the XCTest `XCTMemoryMetric` or `task_info` `phys_footprint`. | Phone |
| Thermal state | every 5 s during a 10-minute loop: `thermal_zone*/temp` for `cpu-1-0..7`, `cpuss-0`, `gpuss-0`, `pm6450_tz`; battery temp; `dumpsys thermalservice` first line "Thermal Status: n" (0 none, 1 light, 2 moderate...); per-cluster `scaling_cur_freq` (cpu0 little, cpu6 big) against `cpuinfo_max_freq`. Throttling = frequency below its start value at constant load, or RTF drift over the loop. On iPhone `ProcessInfo.thermalState`. | Phone, parsed on Mac |
| Sustained RTF | loop the 10 utterances for 10 minutes from a cool start (Thermal Status 0, battery temp < 30 C); report RTF in the first and last minute | Phone |
| Battery draw | `current_now` x `voltage_now` from `/sys/class/power_supply/battery`, sampled with the thermal sample, **valid only while unplugged**; also the `Charge counter` in `dumpsys battery` before and after (coarse, 10-minute run is roughly 1-3% of a battery). While USB is attached the phone is charging and `current_now` reads about -171 mA at the time of my read (verified), which is a charger, not the model. Unplugged runs need `adb tcpip`/wireless debugging (changes adbd mode on the phone: ask the owner) or a device-side script that logs to a file and is read after the cable is re-plugged. iPhone: `UIDevice.batteryLevel` is 1% steps; use the Xcode energy gauge by hand. | Phone |

### What cannot be measured without root (state this in every result file)

- Cannot pin CPU/GPU frequency or governor, so runs share whatever the `walt` governor and the thermal
  engine do; control by cool-down and by repeating, not by fixing clocks.
- Cannot drop the page cache (cold-load numbers are approximate), cannot read GPU clock/utilisation
  (`kgsl` denied), cannot read DSP/NPU use, per-rail power or per-process energy, cannot use `perf`/hardware
  counters except through `simpleperf` on debuggable apps (we have no app).
- Cannot read other processes' memory (Android 14 shell reads its own children only); the LMKD may kill a
  big model silently on a 1.6 GB budget: the wrapper must record the exit status (137 = killed).
- iPhone: no `top`, no sysfs; memory and thermal come only from in-process APIs; ANE use is invisible except
  through speed.

### Fairness rules

- Same wavs, same normaliser, same thread count policy: report 2, 4 and 6 threads once per model, then fix the
  best for the sustained run (the 4 A78 + 4 A55 split makes "8 threads" slower than "4").
- Screen off and radios idle; phone below 30 C battery temperature, Thermal Status 0 at the start; charger state
  recorded; one model at a time; 60 s cool-down between models; 3 repeats of the 10-utterance set, take the median run.
- Quantisation is part of the model identity (int8 vs fp16 vs q4); the row says which.
- WER difference below the bootstrap interval is "no difference".
- Record the runtime version, the model's source URL and sha256, and the phone's build fingerprint
  (`getprop ro.build.fingerprint`) in each row.

## 4. Harness design

### Row format (one row per model per phone per condition), TSV, one line per run

`date, git_sha_of_harness, phone_id, phone_model, os_build, runtime, runtime_version, accelerator, threads,
model_id, model_source_sha256, quant, task (asr_batch|asr_stream|tts|vad|spk|llm), condition
(clean|other|noise10|noise5|radio), n_utts, wer, wer_ci_lo, wer_ci_hi, rtf_median, rtf_p90, first_partial_ms_median,
final_latency_ms_median, load_cold_ms, load_warm_ms, peak_rss_mb, peak_pss_mb, anon_mb, file_mb, start_temp_c,
max_temp_c, thermal_status_max, freq_ratio_end, sustained_rtf_first_min, sustained_rtf_last_min, batt_mw, status
(ok|killed|error|unverified_wer), notes`

Same columns on the iPhone with `accelerator` = `ane|gpu|cpu`, and device-side fields filled from in-process
APIs. Missing values are empty, not zero.

### Where it lives

- `scripts/model-bench/` (developer-side, docs-like per CLAUDE.md, so landing it triggers no CI or deploy):
  `bench.py` (Mac driver, stdlib plus optional `jiwer`), `prepare_fixture.py` (make the wav set and manifest),
  `android/run_one.sh` (the device-side wrapper: poll VmHWM, thermal sample loop, timing), `android/driver.cpp`
  (the real-time streaming feeder, built with the NDK, optional second step), `wer.py`, `runtimes/*.json`
  (one small file per runtime: download URL, sha256, binary names, command template), `models/*.json`
  (model id, URL, sha256, files, licence, runtime). iOS host project in `scripts/model-bench/ios/`
  (outside `mobile/` so the native-boundary gate does not count it).
- `docs/evidence/model-bench-<date>/` for the TSV rows, the manifest, the pulled transcripts and the run log.
  `docs/reference/ON-DEVICE-MODELS.md` gets the verified rows in the same change (the README rule: "update the page").
- Downloads and fixtures cache in `~/.cache/model-bench/` on the Mac (outside the repo; large and untracked).
  The Mac needs only `python3`, `curl`, `adb`, `ffmpeg`/`sox` (check; if missing, ask).

### What the Android driver does, in order

1. Check the phone: `adb -s ZY22H5K8QR get-state`, battery temp, Thermal Status 0, free space; refuse otherwise.
2. `adb shell mkdir -p /data/local/tmp/mbench-<run-id>`; `adb push` the runtime (tarball unpacked on the Mac),
   `libc++_shared.so` from the NDK sysroot when the runtime needs it (the sherpa tarball's binaries list
   `libc++_shared.so` as NEEDED and the tarball does not contain it: verified), the model files, the wavs and
   `run_one.sh`. **Only that directory is written.**
3. Run with `LD_LIBRARY_PATH=/data/local/tmp/mbench-<run-id>/lib`; capture stdout/stderr and the wrapper's
   samples to files in the same directory; `adb pull` them.
4. `adb shell rm -rf /data/local/tmp/mbench-<run-id>` in a `finally` and again on Ctrl-C; then list
   `/data/local/tmp` and fail loudly if our directory is still there.
5. Compute WER on the Mac, append the TSV row, print it.

### What it must never do

- Touch any app's data, `/sdcard`, settings, or install anything (no `adb install`, no `pm`, no `settings put`,
  no `svc`, no `setprop`, no `tcpip` without the owner's say-so); no `su`; no reboot; no `kill` of anything
  but its own pid; never write outside its own `/data/local/tmp/mbench-*` directory; never delete a directory it did not create
  (the existing `/data/local/tmp/exact` stays alone).
- Never put a person's name, an email or this project's name into a download request; it fetches
  public release URLs only, with a generic client string, and records sha256.
- Refuse to run when battery is under 30% or battery temperature above 35 C, or when the phone reports
  Thermal Status above 0 before the run.
- Never drive the iPhone harness onto the owner's installed app container; the host app is a separate bundle
  id and is removed (`devicectl device uninstall app`) at the end; the lead runs the build (CLAUDE.md: one Mac
  build at a time; builds are the lead's), the harness script only prints the command.
- Keep iPhone audio and transcripts of the owner's own speech out of the repo unless the owner agrees.

### Order to build it (first number early)

1. **Step 1 (an hour, no build):** on the Mac, download the sherpa-onnx v2.0.3 `android-aarch64-termux-shared`
   tarball (done once into the scratchpad already; it is untrusted until hashed) and the model
   `sherpa-onnx-nemo-parakeet-tdt-0.6b-v2-int8` (about 630 MB; includes `test_wavs`). Take **10 LibriSpeech
   test-clean utterances**, convert to wav. Push runtime + `libc++_shared.so` + model + wavs to
   `/data/local/tmp/mbench-first`, run `sherpa-onnx-offline --model-type=nemo_transducer --num-threads=4 ...`
   on them, pull stdout. Read the printed RTF; compute WER by a 30-line Python. First row.
   (Expected checks in the first run: does the binary load at all with Android 14's linker and the NDK 27
   `libc++_shared.so`; does it print its own provider line; is the model killed by LMKD.)
2. Add the device wrapper (VmHWM poll, thermal sample) and a second thread count.
3. Add the streaming baseline: `sherpa-onnx` (online) with the English streaming Zipformer int8, then the
   C++ driver for real-time first-partial latency.
4. Add Whisper through sherpa-onnx (offline, same binary) and Moonshine tiny/base; then whisper.cpp built with
   NDK 27.3 for the quantised Whisper comparison (this is the first NDK build).
5. Add VAD (`sherpa-onnx-vad`, Silero), TTS (`sherpa-onnx-offline-tts`), speaker model (sherpa's speaker
   embedding examples or our WeSpeaker conversion; **not checked**) and a small LM through llama.cpp (Qwen3-0.6B
   Q4_K_M 397 MB, LFM2.5-350M Q4_K_M 229 MB, from our own page's numbers).
6. Add noisy set, 10-minute sustained loop, wireless-debug battery runs.
7. iPhone: XCTest host that loads FluidAudio Parakeet EOU and TDT, runs the same manifest, prints a JSON row
   in the same format; later sherpa-onnx iOS and whisper.cpp CoreML.

## 5. Risks

- **libc++ and ABI.** The sherpa binaries need `libc++_shared.so` (NEEDED) which the tarball omits; we must push
  the one from NDK 27.3 (present). A mismatch with the NDK the release was built with is possible: sherpa's
  CI NDK version was not read (unverified); fall back to the plain `sherpa-onnx-v2.0.3-android-static-link-onnxruntime`
  asset if symbols are missing. The binaries carry the Termux RUNPATH, which does not exist on a stock phone;
  `LD_LIBRARY_PATH` or `$ORIGIN/../lib` covers it (RUNPATH has `$ORIGIN:$ORIGIN/../lib`, verified), so keep the
  `bin/` and `lib/` layout.
- **Page size.** The sherpa ELF segments are aligned 0x4000 (16 KB): loads fine on a 4 KB device (verified
  alignment, not run). The reverse trap (a 4 KB-aligned binary on a 16 KB-page system) does not apply to this
  phone (4096 page size) and does not apply to Android 14 on this device; it would matter on newer devices only.
- **Android 14 shell execution.** Executables from `/data/local/tmp` run for the shell uid; app-style
  restrictions (execution from app data, phantom-process killer for app children) do not apply to an adb shell
  child. Dalvik is not involved. SELinux has no extra rule for `shell_data_file` exec on this build: assumed,
  verified only by the first run.
- **Memory.** About 1.6 GB available of 3.59 GB. TDT 0.6B int8 (about 630 MB of weights, more activations) is
  probably OK; Whisper large, Voxtral 3B, Gemma 4 E2B Q4 (3.1 GB file) will not fit: record a kill as a
  result, do not retry.
- **Models whose only format has no mobile runtime.** Nemotron streaming: Safetensors/NeMo/PyTorch and a q8_0
  GGUF for NeMo-Speech.cpp; ONNX only in community repos; the only mobile-ready artefact I found is the
  soniqo LiteRT export (needs a LiteRT driver). Qwen3-ASR streaming "only with the vLLM backend" per its card.
  Parakeet on ExecuTorch has no Android runner outside an example app. Treat "no mobile runtime" as a result
  row (`status=unsupported`), not as work.
- **Accelerators.** No NPU route for SM6450; Adreno OpenCL not listed in llama.cpp; NNAPI deprecated.
  Expect a CPU-only table on this phone; claims of NPU speed from vendor pages are for SM8xxx chips.
- **Licences.** LibriSpeech CC BY 4.0, MUSAN CC BY 4.0 (music files' own licences unverified), Parakeet TDT
  0.6B v2 CC-BY-4.0 and Nemotron OpenMDW-1.1 (commercial use stated on the card), Qwen3-ASR Apache-2.0,
  FluidAudio Apache-2.0, soniqo Nemotron LiteRT CC-BY-4.0 (inherited), LFM models LFM Open License (revenue
  threshold, see our reference page). Traps seen in our own docs: the older GigaAM Russian transducer is
  non-commercial (sherpa page warning); Phonon 40M multilingual is non-commercial; Porcupine/openWakeWord models
  are unusable. A benchmark run is not a distribution, but a model chosen for the app inherits these; the row
  has a `licence` column in `models/*.json`.
- **Accuracy of the proxy.** Benchmarks on read speech overstate radio-condition accuracy; keep the label.
- **Rules.** Classify the landing: the scripts and docs are yellow/green (developer-side), but a landing that
  edits `docs/reference/ON-DEVICE-MODELS.md` is green; the iOS host build is a lead build; the claim "TDT 0.6B
  int8 runs at RTF < X on SM6450" belongs in `docs/calibration/claude-ledger.md` before the first run.

## 6. Could not verify

- Whether the sherpa-onnx Android binary actually starts on this phone (never run), its CPU feature use,
  its real RTF for any model, and whether the bundled ONNX Runtime was built with XNNPACK usable through
  `--provider=xnnpack` (strings show the provider names; use not tested). The version string `1.28.2`
  in the library is read from strings and may be ORT or another component.
- The exact Adreno model, Vulkan version, NNAPI device list on this phone.
- Whether SM6450 could run QNN at all (no doc either way).
- The sherpa-onnx CLI option list (`--help` was not run); names were found by `strings` only.
- The iPhone: it was "Offline" to Xcode, so nothing about its current state was read (iOS version "27.3" is
  from the device list). No iOS xcframework of sherpa-onnx v2.0.3 was found in the release assets.
- Parakeet TDT v2 LibriSpeech WER (1.69/3.19) was read from a model card fetch whose URL path I used first
  returned 404 and then succeeded on a repeated query; re-read the card before using the figure.
- Nemotron streaming accuracy on LibriSpeech: not published on the 3.5 card (FLEURS only); the English-only
  model's card was not read.
- Qwen3-ASR GGUF existence, MLX Swift audio models, LiteRT-LM Android CLI feasibility, MNN and ncnn speech
  models, speaker-verification and TTS runtimes beyond what sherpa's binary list shows.
- RIR corpus and MUSAN music licences; whether `ffmpeg`/`sox` are installed on the Mac.
- No measurement of any kind was made on either phone.
