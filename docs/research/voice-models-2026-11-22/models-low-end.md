# The very low end of on-device voice: what can work on a Motorola moto g stylus 5G (2023) and on older iPhones

Date of research: 2026-11-22. Read-only. Every figure below was read from a page fetched today; the URL is beside it.
"Not stated" means the fetched page did not say. "ESTIMATE" means my arithmetic, with the anchor named. "THIN" means one
source, a vendor claim, or an unmeasured scaling guess.

Conventions: GitHub release sizes are the compressed `.tar.bz2` unless noted (resident memory is larger). RTF = processing
time / audio time (lower is faster; "Nx real time" = 1/RTF).

---------------------------------------------------------------------------------------------------

## 0. The phone, as read over adb today (read-only getprop, /proc, dumpsys meminfo)

- Model XT2315-1 ("moto g stylus 5G - 2023"), board `parrot`, SoC `SM6450` (Snapdragon 6 Gen 1), Android 14 reported by `ro.build.version.release`,
  ABIs `arm64-v8a,armeabi-v7a,armeabi` (so it runs both 64-bit and 32-bit apps).
- 8 cores: 4 x CPU part 0xd41 (Cortex-A78, max 2,208,000 kHz) and 4 x CPU part 0xd05 (Cortex-A55, max 1,804,800 kHz). Features include `asimddp` (int8 dot product) and `fphp/asimdhp` (fp16 arithmetic), which is what XNNPACK/ONNX Runtime int8 kernels use.
- `MemTotal` 3,589,868 kB. At the moment I read it: `MemFree` 100,000 kB, `MemAvailable` 315,348 kB. A process `sherpa-onnx-offline (pid 7303)` was running with 715,400 kB RSS (someone's benchmark; I did not touch it). So the "about 1.6 GiB available" is not a constant: with one 715 MB ASR process resident, available memory was about 0.3 GiB. Plan on a budget that shrinks with whatever the user has open.
- Whether the Google on-device recogniser or the Google TTS app is installed on this phone: NOT checked (outside the allowed adb queries).

CPU class used for estimates: the closest published measurements are on 4 x A76 (Raspberry Pi 5, RK3588) and on the Exynos 1380 (4 x A78 + 4 x A55, the same big.LITTLE layout as SM6450). The A78 runs at 2.21 GHz here versus 2.4 GHz on both of those. I treat the big cluster as equal to those chips within about +/-30% and discount the A55 cluster to near zero (stragglers hurt more than they help; pin to the four big cores).

---------------------------------------------------------------------------------------------------

## 1. Floor per job

### 1.1 Streaming recognition (English)

| Candidate | Disk | Memory | RTF on class A | Accuracy as stated | Licence | Format / runtime | Prebuilt Android arm64 CLI |
|---|---|---|---|---|---|---|---|
| sherpa-onnx-streaming-zipformer-en-20M-2023-02-17 | encoder 41M, decoder 527K, joiner 253K int8 ([docs](https://k2-fsa.github.io/sherpa/onnx/pretrained_models/online-transducer/zipformer-transducer-models.html)); release archive 127.9 MB with fp32 ([release](https://github.com/k2-fsa/sherpa-onnx/releases/tag/asr-models)) | not stated | docs print RTF 0.038 int8 / 0.049 fp32 on an unnamed machine ([docs](https://k2-fsa.github.io/sherpa/onnx/pretrained_models/online-transducer/zipformer-transducer-models.html)). ESTIMATE on class A: 0.10-0.25 (THIN; no ARM measurement of this model found) | WER not given on the model page ([HF](https://huggingface.co/csukuangfj/sherpa-onnx-streaming-zipformer-en-20M-2023-02-17)); source model is an icefall LibriSpeech pruned-transducer-stateless7-streaming-small | Apache-2.0 ([HF](https://huggingface.co/csukuangfj/sherpa-onnx-streaming-zipformer-en-20M-2023-02-17)) | ONNX int8, sherpa-onnx | Yes: `bin/sherpa-onnx` and `bin/sherpa-onnx-vad-with-online-asr` in [sherpa-onnx-v2.0.3-android-aarch64-termux-shared.tar.bz2](https://github.com/k2-fsa/sherpa-onnx/releases/tag/v2.0.3) (17.9 MB; I listed the archive: bin/, lib/libonnxruntime.so, lib/libsherpa-onnx-c-api.so) |
| Vosk small en-us 0.15 | 40M | "about 300Mb of memory in runtime" for a small model | not stated | WER 9.85 LibriSpeech test-clean, 10.38 TED-LIUM ([Vosk](https://alphacephei.com/vosk/models)) | Apache-2.0 | Kaldi, Vosk lib | not checked |
| Moonshine v2 streaming tiny (34M) | `model.safetensors` 176.2 MB fp32 ([HF tree](https://huggingface.co/moonshine-ai/moonshine-streaming-tiny)); community INT8 ONNX 79.8 MB ([HF search snippet, THIN](https://huggingface.co/felixem/moonshine-streaming-tiny-optimized)) | not stated | not measured on ARM. Paper: latency 50 ms, compute load 8.03 % on an Apple M3 only ([paper](https://arxiv.org/html/2602.12241)) | open-ASR average WER 12.01 %, LibriSpeech clean 4.49, other 12.09, AMI 19.03, Earnings-22 20.27 ([HF](https://huggingface.co/moonshine-ai/moonshine-streaming-tiny)) | MIT | Transformers/safetensors; Moonshine Voice library ([README](https://github.com/moonshine-ai/moonshine)) | no |
| Kroko en streaming zipformer | 57.3 MB archive ([release](https://github.com/k2-fsa/sherpa-onnx/releases/tag/asr-models)) | not stated | not stated | not stated ([HF](https://huggingface.co/Banafo/Kroko-ASR)) | ambiguous: card says community models CC-BY-SA, "commercial/OEM models licensed separately"; another Banafo page says Coqui Public License ([Space README](https://huggingface.co/spaces/Banafo/Kroko-Streaming-ASR-Python/blob/4f1572d4a555f4520892bc22ed2542016c336e55/README.md?code=true)). Do not ship without a written licence | sherpa-onnx | yes (sherpa) |
| Cactus Whistle (baseline) | 16.9 MB ([HF](https://huggingface.co/Cactus-Compute/whistle)) | not stated | not stated | not stated on the card | Apache-2.0 | `.cact`, Cactus engine | no |

Floor verdict, streaming English: **sherpa-onnx streaming zipformer 20M int8 (about 42 MB of weights)** is the smallest English streaming model with a real runtime and an Apache licence that I could confirm. Its accuracy is not published on its page, so it is a floor for latency and memory, not for quality. Vosk small is the best-documented accuracy at the same size (WER 9.85 on LibriSpeech clean, which is read speech; expect much worse on phone audio). Moonshine's streaming tiny is better documented on hard sets but has no small ARM package I could verify.

### 1.2 Offline (utterance-level) recognition

| Candidate | Disk | Memory | RTF on class A | Accuracy as stated | Licence | Runner |
|---|---|---|---|---|---|---|
| Moonshine tiny en, int8 `.litertlm` | 29.7 MB; `.tflite` i8 51.9 MB ([HF](https://huggingface.co/litert-community/moonshine-tiny)) | not reported | **Raspberry Pi 5, 4 threads, 5 s window: RTF 0.064 (i8), 0.099 (f32)** ([same page](https://huggingface.co/litert-community/moonshine-tiny)). ESTIMATE class A on 4 big threads: 0.06-0.12 (anchor: Pi 5 = 4 x A76 2.4 GHz; SD 6 Gen 1 = 4 x A78 2.2 GHz) | v1 tiny 12.66 avg WER vs Whisper tiny.en 12.81 ([paper v1](https://arxiv.org/html/2410.15608v2)); v2 tiny 12.01 ([paper v2](https://arxiv.org/html/2602.12241)) | MIT weights ([HF](https://huggingface.co/litert-community/moonshine-tiny)) | LiteRT-LM `omni_runner`, built with bazel `--config=android_arm64`; no prebuilt Android binary in the latest LiteRT-LM release (v0.19.1 assets listed are macOS only) |
| Moonshine tiny en quantized (v2 family, 2026-02-27) in sherpa | 29.9 MB ([release](https://github.com/k2-fsa/sherpa-onnx/releases/tag/asr-models)) | not stated | not stated (use Pi 5 figure above as anchor) | see above | MIT (README: MIT by default, non-English legacy under community licence) ([README](https://github.com/moonshine-ai/moonshine)) | `bin/sherpa-onnx-offline` from the termux-shared archive above; page confirms Moonshine v2 sections incl. Android ([sherpa docs](https://k2-fsa.github.io/sherpa/onnx/moonshine/index.html)) |
| whisper.cpp tiny.en q5_1 | 32.2 MB; tiny q8_0 43.6 MB; base.en q5_1 59.7 MB ([HF tree](https://huggingface.co/ggerganov/whisper.cpp)) | tiny 299 MB peak RSS, base 410 MB, small 889 MB (measured at -t 8 on RK3588, the unquantised files) ([Turing Pi](https://turingpi.com/whisper-cpp-piper-tts-arm64-turing-pi-rk3588/)) | RK3588 (4 x A76 + 4 x A55): tiny 0.109 at 4 threads (derived by me from their 32.56 s / 300 s), 0.10 at 8; base 0.212 / 0.16; small 0.541 / 0.47; medium 1.64 / 1.51 (same page). ESTIMATE class A: tiny 0.10-0.15 | Whisper tiny.en 12.81 avg ([paper v1](https://arxiv.org/html/2410.15608v2)) | MIT (OpenAI Whisper) | no prebuilt Android binary; build with the NDK |
| sherpa-onnx nemo-ctc-en-conformer-small | 76.5 MB archive ([release](https://github.com/k2-fsa/sherpa-onnx/releases/tag/asr-models)) | not stated | not stated | not stated here | not checked | sherpa-onnx-offline |

Floor verdict, offline: **Moonshine tiny (30 MB int8)**. It matches Whisper tiny.en on average WER at 0.7x the compute (Table 1 of the v1 paper: normalised FLOPs 0.7x vs 1.0x, and I saw no measurement on ARM in that paper) and the only ARM-class measurement I found for it is the Pi 5 figure above.

### 1.3 Text-to-speech for reading the news aloud

Reading news needs sustained synthesis well above 1x with margin, because a long article cannot be pre-rendered without latency. Treat 3x as the minimum, 10x as comfortable.

| Candidate | Disk | RTF / speed | Hardware of that measurement | Quality as stated | Licence | Runner |
|---|---|---|---|---|---|---|
| kale-tts-1-nano (7.5M) | 14.8 MB int8 | **44x real time (4 threads), 22x on 1 thread** | Galaxy Tab S10 Lite, Exynos 1380 = 4 x A78 + 4 x A55 ([HF](https://huggingface.co/KaleComputer/kale-tts-1-nano)). ESTIMATE class A: 35-44x (clock 2.21 vs 2.4 GHz) | "unclear words" +1.6 points vs reference recordings (5.82 %); melody gap +7.46 | Apache-2.0 | onnxruntime; **sherpa-onnx not supported** ([tiny card](https://huggingface.co/KaleComputer/kale-tts-1-tiny)) |
| kale-tts-1-tiny (45.7M) | 79.9 MB int8 | **24x, first sentence 0.16 s; 10x on 1 thread** | same Exynos 1380 ([HF](https://huggingface.co/KaleComputer/kale-tts-1-tiny)). ESTIMATE class A 20-24x | unclear words 7.23 % vs 5.82 % reference; no MOS | Apache-2.0 | onnxruntime (Android, Python, Node) |
| tiny-voca (4.5M) | 21.4 MB ONNX fp32; GGUF Q4_0 8.7 MB | "about 70x faster than real time on four CPU threads", CPU not named | ([HF](https://huggingface.co/n4ze3m/tiny-voca)) | Whisper-small.en WER 0.6 % val / 0.9 % test on 8 clips each; "can sound slightly synthetic"; ranks 5th and 7th of 45 on the Open TTS Leaderboard snapshot of 20 Nov 2026 (card claim) | Apache-2.0 | onnxruntime-web; PyTorch; GGUF |
| Kitten TTS 2 pico (7.2M) | **4.0 MB int4, 8.0 MB int8 per language**; plus G2P ~3 MB and voices ~2 MB ([HF](https://huggingface.co/KittenML/kitten-tts-2-pico)) | not stated on the card; **THIN** | the only measurement I found is a competitor's: on the Exynos 1380 tablet, "Kitten TTS nano with a searched style ran at 2.5x" and kitten-tts-2-mini was "20x slower" than kale tiny ([tiny card](https://huggingface.co/KaleComputer/kale-tts-1-tiny)). Pico itself unmeasured | no quality figure on the card | Apache-2.0 | onnxruntime >= 1.20 for int4; sherpa-onnx ships the older kitten-nano-en-v0_8-int8 at 31.2 MB ([release](https://github.com/k2-fsa/sherpa-onnx/releases/tag/tts-models)) |
| Piper en_US-amy-medium | int8 21.0 MB, fp16 35.9 MB, fp32 67.2 MB ([release](https://github.com/k2-fsa/sherpa-onnx/releases/tag/tts-models)) | RTF 0.103-0.119 (about 9x) | RK3588, A76 + A55 ([Turing Pi](https://turingpi.com/whisper-cpp-piper-tts-arm64-turing-pi-rk3588/)); a 116 MB "ryan-high" voice ran 0.63-0.67 (1.5x), too slow | not stated | MIT (Piper; per-voice licences vary, not checked) | `bin/sherpa-onnx-offline-tts` (CLI exists in the termux-shared archive) |
| Piper/VITS low/x_low voices | 13-21 MB int8 ([release](https://github.com/k2-fsa/sherpa-onnx/releases/tag/tts-models), e.g. fr_FR-siwis-low-int8 13.3 MB, vits-icefall-en_US-ljspeech-low 31.7 MB fp32) | not stated | | not stated | varies | sherpa-onnx |

Floor verdict, TTS: **Kale nano (14.8 MB, 44x on a near-identical SoC)** for speed and **Kitten TTS 2 pico int4 (4 MB)** for size, but pico's speed and quality are unmeasured, so by evidence the floor you can defend today is Kale nano or tiny-voca. Piper amy-medium int8 (21 MB, about 9x on RK3588) is the one with a prebuilt Android CLI. Kale and tiny-voca are weeks old (cards dated Nov 2026) and self-reported: label them THIN for quality.

### 1.4 Voice activity and end of turn

| Model | Size | Speed | Notes |
|---|---|---|---|
| Silero VAD v5 `silero_vad_v5.onnx` 2.3 MB; `silero_vad.onnx` 0.6 MB, int8 0.2 MB ([release](https://github.com/k2-fsa/sherpa-onnx/releases/tag/asr-models)) | 0.2-2.3 MB | not stated | runs inside `sherpa-onnx-vad` |
| TEN VAD `ten-vad.onnx` 0.3 MB, int8 0.1 MB (same release) | 0.1-0.3 MB | not stated | |
| Smart Turn v3 | 8 MB int8 ONNX, 32 MB unquantised, 8M params, 23 languages, BSD-2-clause ([HF](https://huggingface.co/pipecat-ai/smart-turn-v3), [blog](https://www.daily.co/blog/announcing-smart-turn-v3-with-cpu-inference-in-just-12ms/)) | per inference incl. preprocessing: 12.6 ms c7a.2xlarge, 15.2 ms c8g.2xlarge, 59.8 ms **c8g.medium (1 vCPU)**, 94.8 ms t3.medium. ESTIMATE one A78 core: 100-200 ms (THIN; Graviton4 core is far faster than an A78) | English accuracy 94.31 %; blog says a VAD such as Silero should run in front |
| livekit/turn-detector | 134.5M params ([HF API](https://huggingface.co/api/models/livekit/turn-detector)) | not measured | too heavy for the floor |

Floor verdict: **Silero v5 (or TEN VAD) plus Smart Turn v3 (about 10 MB, tens to low hundreds of ms per decision)**. This is cheap enough to run on every rung of the ladder.

### 1.5 Wake word and "is this speech meant for the device"

- Wake word / keyword spotting: sherpa-onnx KWS zipformer 3.3M, `sherpa-onnx-kws-zipformer-gigaspeech-3.3M-2024-01-01-mobile` 15.7 MB archive, `...wenetspeech-3.3M...-mobile` 15.3 MB, and `kws-zipformer-zh-en-3M-2025-12-20` 32.9 MB ([release](https://github.com/k2-fsa/sherpa-onnx/releases/tag/kws-models)). Open-vocabulary (you give the phrase); the CLI `bin/sherpa-onnx-keyword-spotter` is in the prebuilt archive. No RTF or false-accept figure found: THIN.
- microWakeWord (ESPHome/Home Assistant) targets microcontrollers; the one number I found is a `tensor_arena_size` of 22,860 bytes in a config example, which is working memory, not file size ([ESPHome](https://esphome.io/components/micro_wake_word/)). Android use only via the Home Assistant companion app (search snippet). THIN.
- "Is this speech addressed to the device": **I found no published small model for this**. Honest answer: it is a composition (VAD + KWS or a push/hold gesture + the server deciding addressing from the transcript). Nothing to benchmark that I can name.
- Moonshine Micro: command recognition plus VAD plus neural speech on an RP2350; "about 3.6 MiB of flash and about 468 KiB of provisioned SRAM", STT about 36 MMAC/s ([README](https://github.com/moonshine-ai/moonshine/tree/main/micro)). It is a fixed-command recogniser, not dictation; it shows how low the floor goes, not a news-quality route.

### 1.6 Speaker verification

- Sherpa speaker-embedding ONNX files: wespeaker_en_voxceleb_resnet34 26.5 MB, wespeaker_en_voxceleb_CAM++ 29.3 MB, 3dspeaker_speech_eres2net_sv_en_voxceleb_16k 26.5 MB, 3dspeaker campplus_sv_en_voxceleb_16k 29.6 MB, nemo_en_speakerverification_speakernet 23.4 MB ([release](https://github.com/k2-fsa/sherpa-onnx/releases/tag/speaker-recongition-models)). Speed, EER and licences: not stated on the release page and I did not find an ARM measurement. THIN. Smaller than about 23 MB: none found.
- Floor verdict: the 23-30 MB class above; run it once per utterance on a few seconds of audio, not per frame; benchmark before promising it on rungs below 3 GiB.

### 1.7 Tiny language model, or no language model

- Floor: **no on-device LM; the server thinks.** On the low rungs, on-device text generation costs more memory than everything else together.
- If one is wanted: google/gemma-3-270m-it, 268,098,176 params ([HF API](https://huggingface.co/api/models/google/gemma-3-270m-it)); litert-community build exists, with the note "Gemma3 270M on Android with GPU acceleration is WIP" and no device benchmark on the card ([HF](https://huggingface.co/litert-community/gemma-3-270m-it)); SmolLM2-135M-Instruct, 134,515,008 params ([HF API](https://huggingface.co/api/models/HuggingFaceTB/SmolLM2-135M-Instruct)). Neither page gave tokens/s or memory on an ARM phone: not stated. Qwen3-0.6B and LFM2.5-350M are already known to you.

---------------------------------------------------------------------------------------------------

## 2. Best quality that still fits class A

I only name combinations whose parts have a measurement or a documented size. Budgets are resident memory for everything loaded together; my sizes for ONNX Runtime overhead are ESTIMATES (weights x 1.5-2 plus 50-100 MB of runtime), checked only against the Whisper RSS figures above.

### Under about 1 GiB resident (all loaded together)

- Offline ASR: Moonshine base en quantized, 111.3 MB ([release](https://github.com/k2-fsa/sherpa-onnx/releases/tag/asr-models)); v1 paper: base 10.07 avg WER, 61.5M params ([paper](https://arxiv.org/html/2410.15608v2)). Or Moonshine v2 small (123.36M params, avg WER 7.84, [paper](https://arxiv.org/html/2602.12241)); I found no small ARM package or file size for it, so it cannot be recommended yet. Parakeet TDT 110M int8: 104.3 MB (and the 108 MB `parakeet_tdt_transducer_110m` streaming-capable transducer) ([release](https://github.com/k2-fsa/sherpa-onnx/releases/tag/asr-models)); accuracy/speed not on any page I fetched: THIN.
- Streaming ASR: sherpa zipformer 20M int8 or Moonshine streaming tiny.
- TTS: **Kale tiny** (79.9 MB, 24x on Exynos 1380) or Piper medium int8 (21 MB, 9x on RK3588). Supertonic 3 (about 99M params, OpenRAIL-M licence) ran 2.4x on that tablet ([kale tiny card](https://huggingface.co/KaleComputer/kale-tts-1-tiny), [Supertone](https://huggingface.co/Supertone/supertonic-3)): marginal. phonon-31m-en: 64.5 MB q8 GGUF, 55.1 MB q4k, self-reported Elo 2098 vs Kokoro 2043 and hard-set LLM WER 1.5 % vs 3.5 %, Apache-2.0, **speed not stated** ([HF](https://huggingface.co/gradium/phonon-31m-en)): the most interesting quality candidate to measure first.
- VAD + turn: Silero + Smart Turn (about 10 MB).
- Speaker verification: CAM++ or ResNet34 (26-30 MB).
- Total disk about 270-330 MB; ESTIMATE resident 450-800 MB.

### Under about 400 MB resident

- ASR: Moonshine tiny int8 30 MB (offline), plus streaming via sherpa zipformer 20M (42 MB) only if you need partial results.
- TTS: Kale nano (14.8 MB, about 40x) or Kale tiny.
- VAD/turn: Silero + Smart Turn. KWS 3.3M (about 16 MB). Skip speaker verification, or load it on demand.
- ESTIMATE resident 200-350 MB.

What does NOT fit: Parakeet 0.6B v2/v3 int8 (482-487 MB archives), Nemotron streaming 0.6B int8 (463-475 MB), Qwen3-ASR 0.6B int8 (878.7 MB), Kokoro int8 (132-147 MB on disk, and about 1x on the Exynos 1380 tablet at full size, 6x when pruned to 37-44M parameters, per the [kale tiny card](https://huggingface.co/KaleComputer/kale-tts-1-tiny)). All sizes from the [asr](https://github.com/k2-fsa/sherpa-onnx/releases/tag/asr-models) and [tts](https://github.com/k2-fsa/sherpa-onnx/releases/tag/tts-models) releases.

---------------------------------------------------------------------------------------------------

## 3. Platform-provided options (no download)

### Android

- `SpeechRecognizer.createOnDeviceSpeechRecognizer` exists from API 31; check `isOnDeviceRecognitionAvailable`, which returns false if no on-device service is present; `triggerModelDownload` from API 33 can fetch a language pack and "might trigger user interaction" ([API ref](https://developer.android.com/reference/android/speech/SpeechRecognizer)). The class doc says of the default path: "likely to stream audio to remote servers ... not intended to be used for continuous recognition". So: audio may leave the device unless you use the on-device factory and it is available. Not tested on this phone.
- ML Kit GenAI Speech Recognition: Basic mode "runs the traditional on-device recognizer", API level 31+; Advanced mode uses the on-device Gemini model; 15 Basic locales (en-US is not marked beta; most others are); alpha with no SLA; no RAM requirement stated ([ML Kit](https://developers.google.com/ml-kit/genai/speech-recognition/android)). Advanced chipsets listed: SM8750, SM8845, SM8850, SM8950, SM8975, MT6991/6993/6995, Exynos 2600, Pixel 10/11. **SM6450 is not listed, so this phone does not qualify for Advanced or Gemini Nano on the evidence of that page**; Motorola is not mentioned.
- ML Kit GenAI Text to Speech: Pixel 8-11 and Galaxy S25/S26/Z Fold8/Z Flip8 only, alpha ([ML Kit TTS](https://developers.google.com/ml-kit/genai/text-to-speech/android)). Not this phone.
- Classic `android.speech.tts.TextToSpeech`: exists on every device since API 4 ([API ref](https://developer.android.com/reference/android/speech/tts/TextToSpeech)); cost zero; I did not find a measured quality figure for the Google engine's offline voices.

### Apple

- `SFSpeechRecognizer`: iOS 10+; on-device recognition iOS 13+, check `supportsOnDeviceRecognition`; set `requiresOnDeviceRecognition` to keep audio off the network, though "on-device requests won't be as accurate"; the server path has per-device daily limits and a one-minute audio limit ([class](https://developer.apple.com/tutorials/data/documentation/speech/sfspeechrecognizer.json), [property](https://developer.apple.com/tutorials/data/documentation/speech/sfspeechrecognitionrequest/requiresondevicerecognition.json)).
- `SpeechAnalyzer` (iOS 26): `SpeechTranscriber` is the general model; `SpeechTranscriber.isAvailable` is "available given the device's hardware and capabilities" and `supportedLocales` "is empty if the device does not support the transcriber"; `DictationTranscriber` is "compatible with older devices" and uses the system dictation models; assets are ML models "downloaded from Apple's servers and managed by the system", shared across apps, with a limited number of per-app locale reservations ([SpeechAnalyzer](https://developer.apple.com/tutorials/data/documentation/speech/speechanalyzer.json), [DictationTranscriber](https://developer.apple.com/tutorials/data/documentation/speech/dictationtranscriber.json), [SpeechTranscriber](https://developer.apple.com/tutorials/data/documentation/speech/speechtranscriber.json), [AssetInventory](https://developer.apple.com/tutorials/data/documentation/speech/assetinventory.json)). Apple's pages do not list which iPhones support `SpeechTranscriber`; that is a runtime check. Memory the models use: not stated.
- `AVSpeechSynthesizer` / `AVSpeechSynthesisVoice`: voices differ by "language, locale, and quality", iOS 7/9+ ([doc](https://developer.apple.com/tutorials/data/documentation/avfaudio/avspeechsynthesisvoice.json)); a quality enum exists. Per-voice size and measured quality: not on Apple's page.

### Is a hybrid the honest answer at the floor?

Yes below about 3 GiB, by elimination: the platform recogniser and TTS cost no memory in your process, and the tiny always-on pieces (Silero or TEN VAD at 0.1-2.3 MB, Smart Turn 8 MB, optional KWS 16 MB) fit in well under 100 MB. The costs: audio may go to the vendor's servers unless you force the on-device path (Android default path; Apple server path), accuracy of forced on-device recognition is lower (Apple says so), and platform recognisers are session-oriented (one-minute limits, no custom barge-in control). Whether platform recognition and playback coexist with hands-free barge-in on cheap hardware (echo cancellation) is NOT established by any page I read.

---------------------------------------------------------------------------------------------------

## 4. Which iPhones, and the degradation ladder

### iPhones (Apple's pages where possible)

- iOS 26 supports iPhone 11, 11 Pro, 11 Pro Max, SE (2nd generation and later), 12 and later; iPhone XS, XS Max and XR end at iOS 18. Source is MacRumors quoting Apple ([MacRumors](https://www.macrumors.com/2025/06/09/ios-26-supports-iphone-11-and-newer/)). Apple's own current iOS page today lists the iPhone 11 family and SE (2nd generation and later) as compatible with the next release, iOS 27 ([apple.com/ios](https://www.apple.com/ios/ios-18/), which redirected to the iOS 27 page). I could not fetch Apple's iOS 18 or iOS 26 compatibility page directly (404), so the XS/XR end-of-line comes from MacRumors only.
- Chips and Neural Engine (Wikipedia, a secondary source; Apple does not publish RAM): A12 (XS, XS Max, XR): 8-core Neural Engine, 5 TOPS; A13 (11 family, SE 2): 8-core, 5.5 TOPS; A14 (12 family): 16-core, 11 TOPS; A15 (13 family, SE 3, 14, 14 Plus): 16-core, 15.8 TOPS; A16 (14 Pro, 15, 15 Plus): 16-core, 17 TOPS ([Apple silicon](https://en.wikipedia.org/wiki/Apple_silicon)).
- RAM as stated by Wikipedia: iPhone XS/XS Max 4 GB ([XS](https://en.wikipedia.org/wiki/IPhone_XS)); iPhone 11 4 GB ([11](https://en.wikipedia.org/wiki/IPhone_11)); iPhone SE 2nd generation 3 GB, SE 3rd generation 4 GB ([SE3](https://en.wikipedia.org/wiki/IPhone_SE_(3rd_generation))); iPhone 12/12 mini 4 GB, 12 Pro cited as 6 GB through a MacRumors headline ([12](https://en.wikipedia.org/wiki/IPhone_12)); 13/13 mini 4 GB ([13](https://en.wikipedia.org/wiki/IPhone_13)); 14/14 Plus 6 GB, 14 Pro cited as 6 GB through a headline ([14](https://en.wikipedia.org/wiki/IPhone_14)). Not stated in what I read: iPhone XR, 11 Pro/Pro Max, 13 Pro/Pro Max.
- Apple's per-app memory ceiling (jetsam limit) per model: not found. Budgets below are fractions of device RAM, my estimate.
- Apple Neural Engine on these chips supports Core ML models; Apple's own pages I fetched do not state operator limits. One third party shows a streaming zipformer small converted to Core ML with 1.6-2.5 ms per 320 ms chunk on Apple-silicon Macs ([HF](https://huggingface.co/maxiboch/zipformer-streaming-ctc-small-en)), not on an A12-A15.

### The ladder (ESTIMATED resident totals; my engineering budgets, not measurements)

| Rung | Resident budget | ASR | TTS | Turn / VAD | Wake / address | Speaker verification | On-device replies | Lost relative to the rung above |
|---|---|---|---|---|---|---|---|---|
| 6 GB iPhone (12 Pro, 14, 14 Plus; A14-A15) | about 1.2-1.5 GB | Moonshine base or Parakeet 110M (about 105-111 MB) or `SpeechTranscriber` | Kale tiny / phonon-31m / Kokoro-class | Silero + Smart Turn | KWS 3.3M | yes (26-30 MB) | optional 135-270M model (+ about 0.3-0.5 GB), THIN | nothing; this is the reference rung |
| 4 GB iPhone (XS, 11, 12, 13, SE 3; A12-A15) | about 0.7-0.9 GB | `DictationTranscriber` or Moonshine tiny (30 MB) | Kale nano/tiny or Piper medium int8 (21 MB) | Silero + Smart Turn | KWS | on demand only | none; server thinks | on-device replies; always-resident speaker verification |
| 3.4 GiB Android (this moto) | about 0.4-1.0 GiB, shrinking with other apps (observed 0.3 GiB available at one moment) | Moonshine tiny 30 MB (offline, RTF 0.06-0.12 est.) or zipformer 20M (streaming, RTF 0.1-0.25 est.) | Kale nano 14.8 MB (35-44x est.) or tiny 79.9 MB (20-24x est.) | Silero + Smart Turn | KWS 16 MB | one-shot per utterance only if benchmarked fast enough | none | speaker verification as a continuous gate; Kokoro; any LM |
| 2-3 GiB Android | under about 250 MB | platform recogniser (on-device factory if available) or Vosk small (about 300 MB runtime memory, so only on 3 GiB) | platform `TextToSpeech`; Kale nano only if memory allows | Silero or TEN VAD (under 3 MB); Smart Turn optional | KWS or push-to-talk | none | none | custom on-device ASR/TTS; barge-in becomes half-duplex |
| 32-bit-only Android (A53/A55 class, Go edition) | under about 150 MB | platform only | platform TTS | TEN/Silero VAD | push-to-talk | none | none | everything custom; see 5.3 |

---------------------------------------------------------------------------------------------------

## 5. What cannot work, and what to benchmark first

### 5.1 Cannot work on class A, and why

- **Parakeet 0.6B / Nemotron 0.6B / Qwen3-ASR 0.6B as the always-loaded recogniser.** Files are 463-879 MB int8 and 600M+ parameters ([asr release](https://github.com/k2-fsa/sherpa-onnx/releases/tag/asr-models)); there is no ARM-phone measurement on any page I fetched. The nearest evidence: a Nemotron-based streaming paper reports RTFx about 7.3 for an int4 variant on a "standard CPU" (unnamed) and Qwen3-ASR-1.7B at RTFx 0.49 on CPU ([arXiv 2604.14493](https://arxiv.org/pdf/2604.14493), reached through a search summary, THIN). On four A78 cores with 0.3-1.6 GiB free and a TTS model also loaded, I would not promise it; it is the ladder's top rung.
- **Kokoro at full size for news.** About 1x on the Exynos 1380 CPU ([kale tiny card](https://huggingface.co/KaleComputer/kale-tts-1-tiny)): no margin.
- **Whisper small and larger.** RK3588: RTF 0.47-0.54 with 889 MB peak RSS for small; medium RTF 1.5+ with 2.14 GB ([Turing Pi](https://turingpi.com/whisper-cpp-piper-tts-arm64-turing-pi-rk3588/)). The RTF alone might pass; the memory with everything else loaded does not.
- **Gemini Nano / ML Kit Advanced speech and ML Kit GenAI TTS** on this phone: the documented chipset and device lists exclude SM6450 and Motorola.
- **GPU or NPU routes**: the Moonshine LiteRT page reports a Galaxy S26 Hexagon NPU run slower than the Raspberry Pi 5 CPU, and 13 s first-launch JIT ([HF](https://huggingface.co/litert-community/moonshine-tiny)); the whisper-tiny LiteRT NPU builds list SM8450-SM8850 and MT6877-MT6993 only, not SM6450 ([HF](https://huggingface.co/litert-community/whisper-tiny)). Use CPU.
- **Reliable hands-free barge-in with continuous speaker verification on A55-only chips**: no measurement exists in anything I read; do not promise it.
- **A model that decides "this speech is for the device"**: none found.

### 5.2 Offered, not proven: Moonshine Micro

Fits any phone, but it recognises commands, not news dictation ([README](https://github.com/moonshine-ai/moonshine/tree/main/micro)).

### 5.3 32-bit-only Android

- Google's policy pages I could reach (Wear OS April 2026 and TV August 2025 posts) say Play "will continue to deliver apps to existing 32-bit devices", and that the 64-bit rule affects only apps with native code; phone-side 64-bit has been required since 2019 per that post ([Android Developers Blog](https://android-developers.googleblog.com/2026/04/get-your-wear-os-apps-ready-for-64-bit-requirement.html)). I found nothing that cuts off 32-bit-only phones.
- sherpa-onnx ships `sherpa-onnx-v2.0.3-android-armeabi-v7a-termux-shared.tar.bz2` (17.4 MB) and an `armeabi-v7a` app build ([release v2.0.3](https://github.com/k2-fsa/sherpa-onnx/releases/tag/v2.0.3)); whisper.cpp builds for ARMv7 with the NDK. **No RTF measurement on a 32-bit/A53 device was found.** Everything for that class is THIN; the safe design is platform recogniser + platform TTS + a VAD.

### 5.4 The ten to benchmark first on the Motorola (in order)

Runner for items 1-6 (all in the same archive): download the prebuilt arm64 CLI
[sherpa-onnx-v2.0.3-android-aarch64-termux-shared.tar.bz2](https://github.com/k2-fsa/sherpa-onnx/releases/download/v2.0.3/sherpa-onnx-v2.0.3-android-aarch64-termux-shared.tar.bz2) (bin/ has `sherpa-onnx`, `sherpa-onnx-offline`, `sherpa-onnx-offline-tts`, `sherpa-onnx-vad`, `sherpa-onnx-vad-with-online-asr`, `sherpa-onnx-keyword-spotter`; lib/ has `libonnxruntime.so`), `adb push` to `/data/local/tmp`, `LD_LIBRARY_PATH=lib`, pin with `taskset f0` (cpu4-7 are the A78 cores? check: cpu0-3 read as 1.8 GHz = A55, cpu4-7 = A78, from the max-freq reads above; confirm index order before pinning), and use `--num-threads=4`. Model URLs below are on the [asr](https://github.com/k2-fsa/sherpa-onnx/releases/tag/asr-models), [tts](https://github.com/k2-fsa/sherpa-onnx/releases/tag/tts-models), [kws](https://github.com/k2-fsa/sherpa-onnx/releases/tag/kws-models) and [speaker](https://github.com/k2-fsa/sherpa-onnx/releases/tag/speaker-recongition-models) release pages (download name = `https://github.com/k2-fsa/sherpa-onnx/releases/download/<tag>/<file>`).

1. **Moonshine tiny en quantized** (offline ASR): `asr-models/sherpa-onnx-moonshine-tiny-en-quantized-2026-02-27.tar.bz2` (29.9 MB). Runner `sherpa-onnx-offline` with the moonshine flags from the [sherpa Moonshine page](https://k2-fsa.github.io/sherpa/onnx/moonshine/index.html). Anchor to beat or match: Pi 5 RTF 0.064.
2. **Streaming zipformer en 20M**: `asr-models/sherpa-onnx-streaming-zipformer-en-20M-2023-02-17.tar.bz2` (127.9 MB, contains int8). Runner `sherpa-onnx` (streaming), or `sherpa-onnx-vad-with-online-asr`.
3. **Kitten TTS 2 pico int4 / int8** (English): [KittenML/kitten-tts-2-pico](https://huggingface.co/KittenML/kitten-tts-2-pico), files `kitten_tts_2_pico_en_int4.onnx` (4.0 MB) and `_en.onnx` (8.0 MB), `voices_en.npz`, `g2p/`. Runner: onnxruntime >= 1.20 via the card's Python or a small C++ harness; not in sherpa yet (sherpa has the older `tts-models/kitten-nano-en-v0_8-int8.tar.bz2`, 31.2 MB, runnable with `sherpa-onnx-offline-tts`: benchmark that as the sherpa-runnable proxy).
4. **Kale TTS 1 nano** (14.8 MB) then **tiny** (79.9 MB): [KaleComputer/kale-tts-1-nano](https://huggingface.co/KaleComputer/kale-tts-1-nano), [KaleComputer/kale-tts-1-tiny](https://huggingface.co/KaleComputer/kale-tts-1-tiny). Runner: onnxruntime (Android) as documented on the cards; not sherpa. Target: reproduce 44x / 24x on this SoC.
5. **Piper en_US-amy-medium int8**: `tts-models/vits-piper-en_US-amy-medium-int8.tar.bz2` (21.0 MB). Runner `sherpa-onnx-offline-tts`. Control for the "ONNX VITS on this phone" figure (RK3588 says RTF 0.10-0.12).
6. **Silero v5 + TEN VAD**: `asr-models/silero_vad_v5.onnx` (2.3 MB), `ten-vad.onnx` (0.3 MB). Runner `sherpa-onnx-vad`. Measure ms per 32 ms frame and idle CPU %.
7. **Smart Turn v3** int8 ONNX: [pipecat-ai/smart-turn-v3](https://huggingface.co/pipecat-ai/smart-turn-v3) (8 MB), runner onnxruntime from the [Smart Turn repo](https://github.com/pipecat-ai/smart-turn) (via the card). Compare with 59.8 ms on a 1-vCPU Graviton4.
8. **whisper.cpp tiny.en q5_1 (32.2 MB) and base.en q5_1 (59.7 MB)**: [ggerganov/whisper.cpp](https://huggingface.co/ggerganov/whisper.cpp), runner `whisper-bench`/`whisper-cli` built with the Android NDK (no prebuilt binary). Gives the cross-check to the RK3588 numbers (tiny 0.109 / base 0.212).
9. **phonon-31m-en q4k/q8 GGUF**: [gradium/phonon-31m-en](https://huggingface.co/gradium/phonon-31m-en) (55.1 / 64.5 MB), runner: the card's Android runtime (its Rust/C or Android guide; speed not stated on the card). Highest claimed TTS quality that might still be fast; measure RTF and first-chunk latency.
10. **Parakeet TDT CTC 110M int8**: `asr-models/sherpa-onnx-nemo-parakeet_tdt_ctc_110m-en-36000-int8.tar.bz2` (104.3 MB). Runner `sherpa-onnx-offline`. This is the quality ceiling for ASR under 400 MB resident; compare against Moonshine tiny and base (`sherpa-onnx-moonshine-base-en-quantized-2026-02-27.tar.bz2`, 111.3 MB).

Runners-up: sherpa KWS 3.3M mobile (`kws-models/sherpa-onnx-kws-zipformer-gigaspeech-3.3M-2024-01-01-mobile.tar.bz2`, 15.7 MB, `sherpa-onnx-keyword-spotter`); CAM++ speaker embedding (`speaker-recongition-models/wespeaker_en_voxceleb_CAM++.onnx`, 29.3 MB); Vosk small en (40 MB, Vosk runtime).

---------------------------------------------------------------------------------------------------

## 6. What I could not verify (consolidated)

- Class-A RTF for any model: every number is an ESTIMATE from Pi 5 (Moonshine), RK3588 (whisper.cpp, Piper) or Exynos 1380 (Kale, via the Kale cards). Nothing was run on this phone.
- RTF/memory for zipformer 20M, Kroko, Vosk, Parakeet 110M, Kitten TTS 2 pico, phonon-31m, KWS, CAM++: not stated or on non-ARM machines (sherpa docs do not name the CPU).
- WER for zipformer 20M, Kroko, Whistle: not stated on the pages read. Moonshine and Whisper WERs come from the authors' papers; Kale and tiny-voca quality numbers are self-reported by their authors weeks ago.
- Licences: Kroko ambiguous; Supertonic 3 is OpenRAIL-M; pocket-tts is gated CC-BY-4.0; per-voice Piper licences not checked; check each before shipping.
- Which iPhones run `SpeechTranscriber` vs `DictationTranscriber` only: no Apple list found. RAM for iPhone XR, 11 Pro, 13 Pro; Apple's per-app memory ceilings: not found. The iOS 26/iOS 18 compatibility lists come from MacRumors (citing Apple) because Apple's own iOS 26/18 pages 404'd or redirected.
- Whether the Google on-device recogniser, offline TTS voices, and ML Kit Basic speech are present and usable on this phone: not checked on the device.
- Moonshine v2 small/medium ARM packages and sizes: not found. Moonshine "micro" 1 MB models: sizes beyond the MCU demo not found.
- No third-party measurement of any of this on a Cortex-A55-only chip or a 32-bit-only system.
- The `taskset` mask for A78 cores: cpu7 reads max 2,208,000 kHz and cpu0 1,804,800 kHz from sysfs, so cpu4-7 are the A78s; confirm with `/sys/devices/system/cpu/cpu*/cpufreq/cpuinfo_max_freq` before pinning.
- Downloaded artefact: I downloaded and listed (did not run or install) the sherpa-onnx termux-shared archive into `dl/`. Nothing was pushed to the phone and nothing in the repo changed.
