# On-device speech recognition survey (2026-11-22)

Method: model cards and file listings read through the Hugging Face API and raw README endpoints on 2026-11-22; the Open ASR leaderboard table pulled from the Space's Gradio config (the page itself renders no text). Nothing was run on a phone by me. Every figure below carries its source. "not stated" means the page does not say.

Marking: **[V]** = vendor/author figure; **[3P]** = measured by someone other than the model's maker (the leaderboard rows are run by the leaderboard's maintainers or the submitter, see caveat); **[ours]** = measured by this project.

## 1. What the app uses today

| Fact | Value | Source |
|---|---|---|
| Default recogniser | Apple's on-device recogniser, stage id `apple-speech`, class `SFSpeechTranscriber` (SFSpeechRecognizer, `requiresOnDeviceRecognition = true`, partial and final results, locale en-US) | `mobile/AutoradioKit/Sources/AutoradioKit/AppleStages.swift:24-30`; default set in `.../Choir/ChoirApp.swift:18` and `.../BasicStages.swift:374` |
| Also written, not default | `apple-speechanalyzer` (SpeechAnalyzer, iOS 26) | `AppleStages.swift:101-118` |
| Registered, not default | Parakeet EOU 120M (streaming), Parakeet TDT 0.6B v2 (batch), via FluidAudio v0.18.3 Core ML; both in the setup ladder | `docs/current-architecture.md:125-131`; `mobile/README.md:302-303`; `mobile/AutoradioKit/Sources/AutoradioModels/ParakeetStages.swift` |
| Speech start/end | energy detector by default; Silero VAD v6.2.1 registered (about 1 MB) | `docs/current-architecture.md:129` |
| Parakeet EOU 120M | chunk 320 ms, 219 MB, NVIDIA Open Model License; Mac: load 28 s, RTF 0.05; **iPhone 14 Pro Max (A16) [ours]: first load 34 s, RTF 0.035, 149 MB resident** (taken before the 2026-11-15 ruling) | `mobile/README.md:302,313`; `docs/current-architecture.md:130` |
| Parakeet TDT 0.6B v2 | about 480 MB, CC-BY-4.0, Mac RTF 0.02-0.03; **never measured on a phone** | `mobile/README.md:303`; `docs/reference/ON-DEVICE-MODELS.md:54` |
| WER on the app's own audio | **none recorded**. The only accuracy evidence in the repo is one demo sentence per model on the Mac. No WER for Apple's recogniser, EOU or TDT on a phone microphone. | grep of `docs/` and `mobile/README.md` |
| Memory budget rule | 6 GB phone: iOS ends a process near 2.2-2.3 GB on a 4 GB phone; 6,144 MB figure for 6 GB is a forum figure | `docs/reference/ON-DEVICE-MODELS.md:264-266` |

Plain reading: the shipped recogniser is Apple's, which the repo has never scored; the Parakeet options exist but the owner's phone has only a load/speed/memory number for EOU, no accuracy number. Any "better" claim below therefore compares against an unmeasured baseline. First job: build a 50-100 clip set from the owner's real voice, radio bleed and room, and score Apple, EOU and TDT on it.

## 2. The leaderboard, with its caveats

Source: Open ASR leaderboard Space config, https://hf-audio-open-asr-leaderboard.hf.space/config (the English table; also described at https://huggingface.co/spaces/hf-audio/open_asr_leaderboard). The Space page itself renders no text; the table was read from the Gradio config. Columns: AMI-Cleaned, Earnings22-Cleaned-AA-chunked, Gigaspeech-Cleaned, SPGISpeech, URGENT24 (noisy), Voice Arena Monsoon (Indian English), Voxpopuli-AA-Cleaned, plus two private sets (scripted, conversational). This is a harder, newer set mix than the old 8-set average used on older cards (Nemotron/Parakeet EOU cards use the older set: LibriSpeech, AMI, Earnings22, Gigaspeech, SPGI, TEDLIUM, VoxPopuli). **The two averages are not comparable.**

Caveats: RTFx is a GPU number (H200/B200-class, batched) and says nothing about a phone. Rows marked 🤗 are open weights; others are hosted APIs. Many rows are self-submitted by the vendor.

Rows relevant here (rank, model, average WER, RTFx), from the same source:

| Rank | Model | Avg WER | RTFx (GPU) |
|---|---|---|---|
| 13 | FluidVoice-Labs/fluid-speech-pro | 6.06 | 3690 |
| 17 | Qwen/Qwen3-ASR-1.7B-hf | 6.15 | 836 |
| 21 | Reson8/Rumble-2-Flash | 6.60 | 6338 |
| 23 | CohereLabs/cohere-transcribe-03-2026 | 6.72 | 924 |
| 30 | nvidia/parakeet-tdt-0.6b-v2 (fast-gpu-asr) | 6.91 | 17254 |
| 34 | nvidia/parakeet-tdt-0.6b-v2 | 6.93 | 6148 |
| 40 | nvidia/parakeet-tdt-0.6b-v3 | 7.04 | 6322 |
| 42 | FermionResearch/Phonon-2.5-English | 7.05 | 2111 |
| 43 | Qwen/Qwen3-ASR-0.6B-hf | 7.07 | 750 |
| 48 | nvidia/nemotron-speech-streaming-en-0.6b | 7.47 | 1116 |
| 51 | openai/whisper-large-v3 | 8.24 | 471 |
| 56 | openai/whisper-large-v3-turbo | 8.75 | 809 |
| 61 | mistralai/Voxtral-Mini-4B-Realtime-2602 | 9.72 | 103 |
| 66 | nvidia/nemotron-3.5-asr-streaming-0.6b | 11.09 | 1353 |

Not on the English board (fetched list of 68 rows): moondream/parakeet-ultra, parakeet-redux, Cactus whistle, SenseVoiceSmall, Moonshine, nara-sar, hima-asr, parakeet-unified, nemotron-labs-unified. Their numbers are the makers' own, on a different set mix.

## 3. Candidate table

Sizes are file sizes from the HF API (https://huggingface.co/api/models/<id>?blobs=true, 2026-11-22) unless marked. "Dl" is the repo's download counter at the same fetch.

### 3a. Streaming recognisers (needed for hands-free barge-in)

| Model | Params; architecture | On-disk (per format) | Languages | Licence; commercial | Reported WER | Speed / phone / memory | Formats | Punct., timestamps, endpointing | Maturity |
|---|---|---|---|---|---|---|---|---|---|
| **nvidia/parakeet-unified-en-0.6b** https://huggingface.co/nvidia/parakeet-unified-en-0.6b | 600M; unified FastConformer-RNNT, one checkpoint for offline and streaming, min latency 160 ms; the card says current pipeline is *buffered* streaming (left context recomputed), "longer than cache-aware" | .nemo 2,474 MB. **Core ML port (FluidInference) https://huggingface.co/FluidInference/parakeet-unified-en-0.6b-coreml**: int8 encoder about 590 MB per tier, fp16 about 1,177 MB; GGUF community ports exist (handy-computer, 2.3M dl) | English | NVIDIA Open Model License on the NVIDIA card ("ready for commercial/non-commercial use"). The Core ML repo says CC-BY-4.0. **The two disagree; resolve before shipping** | Core ML card, LibriSpeech test-clean (2,620 files, int8): batch 2.16% avg, streaming 2080 ms 2.21%; 150-file sweep: 320 ms 2.37%, 640 ms 2.40%, 1120 ms 2.25% [V, FluidInference]. NVIDIA card gives only a chart for the HF-leaderboard average (not extractable) | RTFx on an Apple chip not named: streaming 2080 ms 66x, 320 ms tier 10x, 640 ms 27x (Core ML card). No iPhone figure. Memory: not stated | Core ML (FluidAudio `StreamingUnifiedAsrManager`, `UnifiedAsrManager`), NeMo, GGUF | punctuation and capitalisation yes; timestamps not stated; no EOU token stated | NVIDIA release 2026-04-07; Core ML repo 3,458 dl, updated 2026-09-25; NVIDIA repo 7,585 dl |
| **nvidia/nemotron-speech-streaming-en-0.6b** https://huggingface.co/nvidia/nemotron-speech-streaming-en-0.6b | 600M (618M safetensors); cache-aware FastConformer-RNNT, chunks 80/160/560/1120 ms, 24 encoder layers | .nemo 2,473 MB; GGUF q8_0 700 MB (run by NVIDIA's NeMo-Speech.cpp); Core ML int8 encoder about 564 MB, folder about 625 MB per chunk size (https://huggingface.co/FluidInference/nemotron-speech-streaming-en-0.6b-coreml); LiteRT int8 895 MB, fp16 1,209 MB (community, spybyscript) | English | NVIDIA Open Model License; card says commercial use ready | NVIDIA card, older HF 8-set average: **6.93 at 1.12 s, 7.07 at 0.56 s, 7.67 at 0.16 s, 8.43 at 0.08 s** (LibriSpeech clean 2.32/2.46/2.56/2.80). Leaderboard English 2026 mix: 7.47 [3P-ish, rank 48]. **Core ML port measured by its publisher on M2: LibriSpeech clean 1.99% (1120 ms), 2.12% (560 ms), but about 10% at 160 ms and about 60% at 80 ms (20 files)** : the 160 ms Core ML conversion is much worse than NVIDIA's own 2.56 | Core ML on M2: RTFx 9.6x (1120), 8.5x (560), 3.5x (160), 1.9x (80). No phone figure. Memory not stated | NeMo, safetensors, GGUF, Core ML, LiteRT/TFLite (community) | punctuation and capitalisation yes; timestamps not stated; no EOU | NVIDIA, created 2025-12-17, updated 2026-08-05; 106k dl; 655 likes |
| **nvidia/nemotron-3.5-asr-streaming-0.6b** https://huggingface.co/nvidia/nemotron-3.5-asr-streaming-0.6b | 638M; same cache-aware RNNT, 40 language-locales, language-ID prompt; chunks 80/160/320/560/1120 ms | .nemo 2,368 MB; GGUF q8_0 742 MB; Core ML int8 about 592 MB encoder (aufklarer, community); ONNX int4 788 MB MIT-tagged (onnx-community); LiteRT (community) | 40 locales; NVIDIA itself recommends the English-only model for English-only use | OpenMDW-1.1; ready for commercial use | English FLEURS: 9.43 (80 ms), 8.88 (160), 8.27 (320), 7.99 (560), 7.91 (1.12 s) [V]. **Leaderboard English avg 11.09 (rank 66), worse than the English-only sibling** | no phone figure | as listed | punctuation yes; language tag appended | 417k dl (most-downloaded here), 1,288 likes, updated 2026-09-10 |
| **moonshine-ai/moonshine-streaming-small / medium / tiny** https://huggingface.co/moonshine-ai/moonshine-streaming-small | 123M / 245M / 34M (card table); sliding-window encoder, autoregressive decoder; 80 ms lookahead; card says target is "0.1-1 TOPS and sub-1 GB memory" | safetensors total: tiny 176 MB, small 561 MB, medium 1,064 MB (fp32 repo); quantised/runtime sizes not stated on the card | English (card); runtime also covers other languages per the GitHub README | MIT for code and models by default (README, https://github.com/moonshine-ai/moonshine); legacy non-English non-streaming models are non-commercial | Card, older 8-set average [V]: **tiny 12.01, small 7.84, medium 6.65**; LibriSpeech clean 4.49 / 2.49 / 2.08 | no latency or phone numbers on the card or README; "not stated" | HF Transformers; runtime library for Python, iOS, Android, WASM, Pi (README); ONNX and GGUF community ports exist | not stated on card; library advertises live streaming; hallucination and repeat loops on short/noisy segments are a named limitation | Useful Sensors / Moonshine AI; about 40k dl each; updated 2026-02 |
| **nvidia/nemotron-labs-unified-asr-0.6b** https://huggingface.co/nvidia/nemotron-labs-unified-asr-0.6b | 600M; unified RNNT, offline plus streaming from 160 ms to 2,080 ms, language-ID prompt | .nemo 2,508 MB; Core ML (FluidInference, Glimpse) | 25 European languages | OpenMDW-1.1; commercial ok | card claims it beats parakeet-tdt-v3 offline and Nemotron 3.5 streaming in most scenarios (chart; numbers not extracted) | none | NeMo, Core ML (community) | punctuation yes | 4,297 dl, released 2026-10-27; new |
| **parakeet-realtime-eou-120m (the app's current EOU)** https://huggingface.co/nvidia/parakeet_realtime_eou_120m-v1 | 120M; streaming RNNT with `<EOU>` token | Core ML 213 MB encoder per chunk size (FluidInference) | English | NVIDIA Open Model License | NVIDIA card, 160 ms setting, older 8-set average: **9.30** (LibriSpeech clean 3.61). Core ML card, LibriSpeech clean: 8.29% at 160 ms, 4.87% at 320 ms | **iPhone 14 Pro Max: RTF 0.035, 149 MB [ours]**; M2 RTFx 4.78x (160 ms), 12.48x (320 ms) | Core ML, NeMo | has end-of-utterance token | 56k dl on the Core ML repo |
| **moondream/parakeet-redux** https://huggingface.co/moondream/parakeet-redux | 149M params (the brief's "0.1B" is 0.149B); ternary 1.58-bit encoder of parakeet-tdt-v3. **Offline (TDT), not streaming** | safetensors 178 MB | 25 European languages | CC-BY-4.0 weights; the runtime ("Photon", proprietary or not: not verified) is required to run packed weights | Own harness: English 7-set average **6.55** vs 6.26 for the v3 original; noise 9.04 vs 6.72 (clearly worse in noise); business speech 6.96 vs 6.15 [V] | x86 8 cores 113x; M2 CPU 38x, GPU 43x [V]. No phone or Arm-phone figure; "NEON on ARM" stated only in prose | Photon only (not ONNX/Core ML/GGUF) | timestamps (segment and word) yes; small VAD head included | 9k dl, 328 likes; created 2026-09-18; vendor unknown to me |

### 3b. Offline recognisers (fine for a held-to-talk take)

| Model | Params; architecture | On-disk | Languages | Licence | Reported WER | Speed / memory | Formats | Features | Maturity |
|---|---|---|---|---|---|---|---|---|---|
| **FluidVoice-Labs/fluid-speech-pro** https://huggingface.co/FluidVoice-Labs/fluid-speech-pro | 627M; FastConformer-TDT, a retrained parakeet-tdt-0.6b-v3 plus "one small extra block" | safetensors 2,508 MB; **Core ML 649 MB** (encoder 613 + decoder 24 + joint 13; https://huggingface.co/FluidVoice-Labs/fluid-speech-pro-coreml); **ONNX int8 about 850 MB** (encoder 746); GGUF q8_0 941 MB | 25 European (auto-detect) | CC-BY-4.0; card: "ready for commercial and non-commercial use" | **Leaderboard 6.06 average, rank 13 (English board)**; URGENT24 noisy 9.83 (rank 2 among open models per the card). Greedy decoding is the default; leaderboard used 4-beam search (card says about 0.6% fewer errors) | "about 3,700x real time on one GPU" (this is the leaderboard figure, GPU only). No phone or CPU figure. Memory not stated | NeMo/Transformers, ONNX (onnx-asr, sherpa-onnx), MLX, GGUF (parakeet.cpp), Core ML (FluidAudio) | punctuation, word and segment timestamps (NeMo only), optional "clean mode" (not yet on Core ML) | FluidVoice team; created 2026-10-30; 9.8k dl, 178 likes; Core ML repo 359 dl. Self-submitted leaderboard row |
| **moondream/parakeet-ultra** https://huggingface.co/moondream/parakeet-ultra | 627M; same architecture, post-trained, fp, GPU-targeted | safetensors 1,255 MB (bf16) | 25 European | CC-BY-4.0 | own harness, English 7 sets 5.80 vs 6.26 (v3); noise 5.82 | GPU only (9,743x on B200 in Photon); no phone | Photon only | timestamps | 14k dl; 2026-09-22. No Core ML/ONNX: **not mobile-runnable today** |
| **nvidia/parakeet-tdt-0.6b-v2** (current "TDT 0.6B" family) https://huggingface.co/nvidia/parakeet-tdt-0.6b-v2 | 600M TDT, English | .nemo 2,472 MB; Core ML (FluidInference) | English | CC-BY-4.0 | leaderboard English 6.93 (rank 34) | Mac RTF 0.02-0.03 [ours]; no phone figure [ours] | Core ML in use | punctuation and timestamps | 117k dl |
| **AtomicChat/Rumble-2-Flash-GGUF** https://huggingface.co/AtomicChat/Rumble-2-Flash-GGUF (base https://huggingface.co/Reson8/Rumble-2-Flash) | 0.5B (card); Qwen3-ASR-0.6B derivative per AtomicChat's weight comparison (vendor card names no base); 18-layer audio encoder plus 14-layer decoder, autoregressive | GGUF AD-Q5_K_M 283 MB + mmproj Q8_0 214 MB = **about 0.50 GB** ("recommended"); Q8_0 346 MB; BF16 648 MB; mmproj F16 379 MB | 45 languages | Apache-2.0 | Reson8 on leaderboard: **6.60 avg, rank 21 (self-run on H200 vLLM)**. AtomicChat re-measured GGUF on an A10: LibriSpeech clean 2.09-2.12, other 4.70-4.80 (a [3P] check of quantisation loss only, not an English-average claim) | **needs a patched llama.cpp**: stock b12521 adds trailing hallucinated words on short clips (clean WER 3.13 vs 2.09); patch not yet upstream. No phone figures | GGUF (llama.cpp `qwen3vl` plus `qwen3a`), Transformers, vLLM | filler/digits/verbatim options; punctuation not stated | **brand new**: repo created 2026-11-18, 1,068 dl, 5 likes |
| **Cactus-Compute/whistle** https://huggingface.co/Cactus-Compute/whistle (blog https://cactuscompute.com/blog/whistle, 2026-10-02) | parameters not stated; 8-block encoder, 8-layer decoder, 8,192 pieces, beam 5 | **16.9 MB** (`whistle.cact`); safetensors 220.6 MB | EN, DE, FR, ES, IT, NL, PL | Apache-2.0 (HF tag; the Cactus runtime repo is also Apache-2.0 per its page) | blog says it beats Whisper base on LibriSpeech clean/other, SPGISpeech, Earnings-22, FLEURS and loses on TED-LIUM, AMI, MLS; **the WER values are in charts I could not extract**; 86,174 utterances; no test audio in training per the vendor [V] | Apple M4 Pro CPU, 10 s clip: time to first token 11.1 ms, decode 1,319 tok/s (vs Whisper base 73.2 ms, Moonshine tiny v2 22.8 ms) [V]. **No phone numbers, no memory number.** Non-streaming (30 s window) | Cactus engine: iOS ARM64, Android ARM64/ARMv7 static libs, C API and Python; no Swift or Kotlin SDK documented | word timestamps; punctuation not stated | 85.6k dl, 584 likes; Cactus (a startup); Oct 2026 |
| **Xviers/whisper-large-v3-turbo-GGUF** https://huggingface.co/Xviers/whisper-large-v3-turbo-GGUF | Whisper large-v3-turbo, about 809M (leaderboard); encoder-decoder | q4_0 463 MB (candle) / 474 MB (whisper.cpp), q5_0 574 MB, q8_0 865-874 MB; q2_k 286 MB | multilingual | MIT (HF tag) | leaderboard English **8.75** (openai/whisper-large-v3-turbo, rank 56); this repo has no WER of its own | none stated; known to be slow on phones, not measured here | GGUF for whisper.cpp and Candle | whisper.cpp timestamps | a 507-dl repack by an unknown user; use whisper.cpp's own official ggml or the OpenAI repo instead |
| **FunAudioLLM/SenseVoiceSmall** https://huggingface.co/FunAudioLLM/SenseVoiceSmall | not stated on card; non-autoregressive; 400k hours, 50+ languages; also emotion and audio-event tags | model.pt 936 MB | zh, yue, en, ja, ko stressed | HF tag "other: model-license" pointing to FunASR `MODEL_LICENSE`; **I did not read it, so commercial use is unverified** (the repo code is MIT) | card says it beats Whisper on Chinese; **no English WER table in the card text** | "70 ms for 10 s of audio, 15x faster than Whisper-Large" [V], hardware not stated | PyTorch, ONNX and libtorch export via funasr | emotion, events | 27k dl; 2024-07; not on the English board |
| **nvidia/nemotron-labs / Phonon-2.5-English (Fermion)** | retrained Parakeet | already in docs as `FluidInference/phonon-2-coreml` | English | not checked | leaderboard 7.05 (rank 42) | none | Core ML | | listed for completeness; no gain over Parakeet TDT v2 on the board |
| **YeeNyunn/nara-sar** https://huggingface.co/YeeNyunn/nara-sar | 326M; CTC fine-tune of Meta omniASR-CTC-300M for **Burmese plus English**, with a 6-gram LM | ONNX int8 366 MB + LM 84 MB; fp32 1.3 GB | Burmese, English | Apache-2.0, attribution required | no English WER on the card text read | default 2 threads, CPU; no speed figure | ONNX, safetensors | not stated | 97 dl, created 2026-11-17. **Wrong tool for an English-first app** (Burmese specialisation), disqualify unless Burmese matters |
| **Prahaladha/hima-asr** https://huggingface.co/Prahaladha/hima-asr | 0.46B (920 MB); 11 Indian languages plus English, plus a "decision head" | ONNX int8 487 MB, q4 360 MB; MLX 4-bit 379 MB | 11 Indian + English | HF tag: "apache-2.0-and-gemma" (Gemma terms apply) | no English WER in the text read | CPU ONNX int8 35x real time, q4 31x, on a MacBook Pro M4 Pro [V]. No phone | ONNX, MLX, PyTorch | decisions (tone, urgency) | 2.1k dl, 2026-10-12. Indic-focused; not for English-first |
| **CohereLabs/cohere-transcribe-03-2026** https://huggingface.co/CohereLabs/cohere-transcribe-03-2026 | 2.07B (API); safetensors **4,132 MB** | 4.1 GB bf16; no small quantisation seen in the listing | not read (card is gated for my fetch) | apache-2.0 (HF tag) | leaderboard English 6.72 (rank 23) | 924x on GPU | safetensors | | **Too big**: 4.1 GB of weights exceeds the whole 3.4 GiB RAM of the Android target and the practical iPhone budget (about 2.2-2.3 GB per process on 4 GB phones, 6 GB phones higher but unmeasured). Not a candidate. |
| **Metric-AI/OhanASR-v0.1 and OhanASR-Streaming-v0.1** https://huggingface.co/Metric-AI/OhanASR-Streaming-v0.1 | not stated; `model.pt` **9,119 MB** each | 9.1 GB | not stated (card gated) | **cc-by-nc-4.0: non-commercial** | not readable | none | one PyTorch pickle | | 0 downloads, restricted access. **Disqualified**: licence, size, no mobile format |

## 4. Rankings

Two rules for "better": (1) it must beat Apple SFSpeech on the owner's own clips, which has never been measured; (2) the app already links FluidAudio 0.18.3. Whether that tag has `UnifiedAsrManager`, `StreamingUnifiedAsrManager`, `NemotronStreamingAsrManager` or the Fluid Speech Pro extra block is **not verified** (the Core ML cards document them against "FluidAudio" without a version). Check the pinned tag before costing the work.

### (a) iPhone 14 Pro Max class (6 GB, Apple Neural Engine, FluidAudio and Core ML already in the app)

**Streaming (hands-free barge-in)**

1. **parakeet-unified-en-0.6b, Core ML int8, 640 ms or 320 ms tier.** One checkpoint gives live partials and a final offline-quality pass. About 590 MB per tier. Streaming WER on LibriSpeech clean is 2.37-2.40% at 0.32-0.64 s with punctuation. The Core ML card gives speed on an unnamed Apple chip. Not measured on A16. Risks: memory (about 0.6 GB model plus compile), the licence discrepancy on the card, buffered not cache-aware streaming.
2. **nemotron-speech-streaming-en-0.6b, Core ML, 560 ms variant.** Same speed class (about 8.5x on M2), 2.12% clean. NVIDIA's own table shows 160 ms as usable (7.67) but the community Core ML 160 ms build is about 10%, so run 560 ms. Cache-aware, mature (106k dl). About 625 MB per variant.
3. **Parakeet EOU 120M (already present)** as the floor and the endpointing partner: 149 MB, RTF 0.035 measured on the owner's phone, but WER is the worst here (9.30 vs 7.67 for Nemotron at the same 160 ms on NVIDIA's own tables). Moonshine streaming small (123M, 7.84 on the older average) is a plausible fourth but has no Core ML build or phone number in what I read.

Benchmark first: **Parakeet Unified Core ML, 640 ms tier**, int8, scored against Apple and EOU on the same recorded set. It adds the least new machinery (a FluidAudio manager) and gives both a streaming and a final pass from one load.

Disqualifiers: licence ambiguity (NVIDIA Open Model License vs CC-BY on the port); process memory over about 2 GB with TTS (Kokoro), Qwen and the models loaded together; a FluidAudio tag that lacks the manager; first-load Core ML compile time (EOU took 34 s; a 0.6B encoder will be longer); Nemotron 3.5 multilingual (worse English).

**Offline (held-to-talk)**

1. **FluidVoice fluid-speech-pro Core ML (649 MB).** Best third-party-run score among sub-1B open models on the current English board (6.06). CC-BY-4.0 and explicit commercial use. Self-submitted row, and the Core ML build lacks clean mode. Needs the extra block supported in the Swift runtime (not verified).
2. **Parakeet TDT 0.6B v2 (already registered)**: 6.93, the cheap upgrade of what is already wired; never measured on a phone.
3. **Rumble-2-Flash AD-Q5_K_M + mmproj Q8 (0.50 GB), llama.cpp.** 6.60 on the self-run board; but it needs a patched llama.cpp, the repo is four days old, and the Swift side would be new code. Whistle (16.9 MB) is the long shot for a tiny always-on pre-pass.

Benchmark first: **fluid-speech-pro Core ML**; if the runtime change is a day of work, otherwise TDT v2 on the A16 to close the open measurement.

Disqualifiers: no Core ML or GGUF build (parakeet-ultra), non-commercial licence (Ohan), size (Cohere).

### (b) Low-end Android (Snapdragon 6 Gen 1, about 3.4 GiB RAM, arm64, CPU)

Budget: with the OS and the app, assume well under 1 GB for the recogniser. No model here has an Android number from its maker or from anyone else; every ranking below is by file size and format only.

**Streaming**

1. **Moonshine streaming small (123M, 7.84)**, run through the Moonshine library (MIT; Android listed in its README). Designed for "sub-1 GB memory" per its card. I could not find a quantised size or any phone latency. Tiny (34M, 12.01) is the fallback.
2. **Nemotron-speech-streaming-en 0.6b, LiteRT int8 (895 MB, community port by spybyscript) or GGUF q8_0 (700 MB, NVIDIA's NeMo-Speech.cpp; Android support not verified).** Best streaming accuracy per megabyte here, but 0.6-0.9 GB of weights on a 3.4 GiB device that also holds the app and a radio is the memory danger; the port is a community conversion with 1.9k downloads.
3. **Parakeet EOU 120M in ONNX/sherpa-onnx** (smallest cache-aware streaming; the repo has Core ML for iOS; an Android build was not found in what I read).

Benchmark first: **Moonshine streaming small.** Disqualifiers: no Android runtime build; resident memory over about 0.8 GB; real-time factor above 1 on the Snapdragon 6 Gen 1 CPU (none has been published); MIT/Apache licences are fine, NVIDIA models are fine commercially but need their notice.

**Offline**

1. **Cactus Whistle (16.9 MB, Apache-2.0, Android ARM64 supported).** The only model here small enough to be certain about memory. Languages: seven. WER values are in charts I could not read, so its English accuracy against Apple's or Google's recogniser is unknown. Vendor-only evidence.
2. **Rumble-2-Flash AD-Q5_K_M + mmproj Q8 (0.50 GB), llama.cpp on Android CPU.** Accurate on the self-run board (6.60); speed on a Snapdragon 6 Gen 1 CPU unknown, and the autoregressive decoder plus an audio tower may be slow there; needs the unpatched-llama.cpp fix.
3. **fluid-speech-pro ONNX int8 (about 850 MB) via sherpa-onnx or onnx-asr.** Best accuracy, but 0.85 GB of weights and a 0.6B encoder on that CPU will probably run slower than real time (not measured). **moondream/parakeet-redux (178 MB, ternary)** would be the answer to this size problem (38x real time on an M2 CPU per the card) but only runs on the proprietary Photon runtime with no Android evidence: watch, don't plan.

Benchmark first: **Whistle** (a 17 MB test is nearly free; it settles whether tiny wins) then Rumble-2-Flash if Whistle's accuracy is poor.

Disqualifiers: non-commercial licence (Ohan; Moonshine's legacy non-English models), no CPU format (parakeet-ultra), weights above about 1 GB (Cohere 4.1 GB, whisper-large GGUF q8 865 MB marginal and slow), Burmese/Indic specialisations.

## 5. What is marketing and what is measured

- **Third-party-ish:** only the leaderboard rows (6.06 etc.). Even these are submitted by the model's maker for most open rows, run on server GPUs, and use a mix of public sets with private sets. They do not predict a phone microphone with radio bleed.
- **Vendor-only:** every on-device speed claim (Whistle 11.1 ms, Redux 113x, Hima 35x, Moonshine none), SenseVoice "70 ms for 10 s", the Core ML RTFx figures (the publisher's own, Mac M2 or an unnamed chip), and all "best under 1B" statements.
- **Community conversions:** Core ML, LiteRT, ONNX and GGUF ports are not the same model as the original: the Nemotron 160 ms Core ML build scored about 10% against NVIDIA's own 2.56, so a port can lose accuracy silently.
- **Ours:** only the EOU and Silero phone numbers in `mobile/README.md`.

## 6. What I could not verify

- Any model's accuracy or speed on an iPhone 14 Pro Max or a Snapdragon 6 Gen 1; any Android number at all.
- The Whistle WER values (charts only); the nara-sar and SenseVoice English WER; Cohere card text (gated for my fetch); Ohan card (gated).
- Whether the pinned FluidAudio 0.18.3 contains the unified, Nemotron or Fluid Speech Pro managers.
- The Photon runtime's licence and Arm status; SenseVoice's model licence terms; Cactus runtime licence for commercial use (page says Apache-2.0).
- Apple's own recogniser accuracy on the owner's clips (no repo measurement).
- The Moonshine docs page (404) so no latency table; NeMo-Speech.cpp Android support.
- The NVIDIA charts for Parakeet Unified's leaderboard average.

---

# Addendum (owner's added candidates, fetched 2026-11-22)

## 7. What MLX means for our targets

- **iPhone.** MLX is Apple's array framework; the aufklarer builds are driven by the Swift package `soniqo/speech-swift` (https://github.com/soniqo/speech-swift, README fetched). The README lists "macOS 15+ or iOS 18+, Apple Silicon (M1/M2/M3/M4)" and says MLX runs on the **GPU** while Core ML runs on the **Neural Engine**; Qwen3-ASR is offered on both backends. An iOS echo demo exists. The only iPhone it benchmarks is the **iPhone 16 Pro (A18 Pro, iOS 26.5)** (https://github.com/soniqo/speech-swift/blob/main/docs/benchmarks/ios-coreml.md), and **every row is Core ML, none MLX**, and **no Qwen3-ASR row** (rows: Parakeet-EOU 120M 0.04 RTF, 297 MB peak; Omnilingual 300M 0.28 RTF, 495 MB). Whether MLX runs the 4-bit 0.6B on an A16 iPhone 14 Pro Max at usable speed: **not stated anywhere I read, not verified**. The README says nothing about iPhone 14. The package also carries a competing copy of our stack (Silero, WeSpeaker, Kokoro), but it would be a second runtime beside FluidAudio. soniqo.audio's own catalogue line (https://soniqo.audio): "Qwen3-ASR: 52 langs, RTF 0.06, 4-/8-bit", runtimes MLX and Core ML; no device named.
- **Android: MLX does not run on Android.** MLX targets Apple silicon (Metal) and CUDA/CPU Linux; speech-swift states no Android support ("AI speech toolkit for Apple Silicon"). For the Motorola the MLX builds are irrelevant. Qwen3-ASR does have other formats, below.
- soniqo's own Android evidence is on the Galaxy S23 (Snapdragon 8 Gen 2, much faster than the Snapdragon 6 Gen 1): **Parakeet-EOU 120M ONNX INT8: 0.21 RTF, 232 MB; Omnilingual 300M LiteRT: 0.15 RTF, 831 MB**, CPU only (https://soniqo.audio, vendor figures). No Qwen3-ASR figure.

## 8. Qwen3-ASR family

| Item | Facts | Source |
|---|---|---|
| **Qwen/Qwen3-ASR-0.6B(-hf)** | Apache-2.0. `-hf` repo: 782M params (the audio encoder plus the 0.6B Qwen3 decoder; the original repo lists 938M), safetensors 1,565 MB bf16. 52 languages and dialects (30 languages + 22 Chinese dialects). Autoregressive Whisper-style audio encoder plus Qwen3 text decoder. Card claims streaming/offline in one model, "2000x throughput at concurrency 128" (GPU, vendor). **Streaming is only in the vLLM backend** (not usable on a phone). 535k downloads (the `-hf` repo), created 2026-06-26 | https://huggingface.co/Qwen/Qwen3-ASR-0.6B-hf ; https://huggingface.co/Qwen/Qwen3-ASR-0.6B |
| English accuracy | Leaderboard English board: **Qwen3-ASR-0.6B-hf 7.07 (rank 43)**, Qwen3-ASR-1.7B-hf 6.15 (rank 17), RTFx 750 / 836 (GPU) | the Space config in section 2 |
| **aufklarer/Qwen3-ASR-0.6B-MLX-4bit** | Apache-2.0; 708 MB file ("about 680 MB" on the card); MLX 4-bit conversion; used by `speech-swift` `Qwen3ASR`; card gives no WER, speed or memory; 8.2k dl, created 2026-02-28. Also in the collection: 0.6B 5-bit and 8-bit, 1.7B 4/5/8-bit | https://huggingface.co/aufklarer/Qwen3-ASR-0.6B-MLX-4bit |
| **aufklarer/Qwen3-ASR-1.7B-MLX-8bit** | Apache-2.0; 2.04B params; **2,463 MB** file. Too large for the 3.4 GiB Android phone; on a 6 GB iPhone it exceeds the safe per-process budget once the app, TTS and LLM are loaded (budget figures in `docs/reference/ON-DEVICE-MODELS.md:264-266`). Not a candidate. 8.9k dl | https://huggingface.co/aufklarer/Qwen3-ASR-1.7B-MLX-8bit |
| Non-MLX mobile formats for Qwen3-ASR | **LiteRT (Android-oriented)**: `litert-community/Qwen3-ASR-0.6B` (Apache-2.0, updated 2026-11-04, 2.4k dl): `qwen3_asr_0.6b_5s_i8.litertlm` **960 MB** (recommended; CPU or GPU; runs via LiteRT-LM Omni "across Android, iOS, macOS, Linux"), `..._i8.tflite` 794 MB, plus about 1.9 GB f32 variants precompiled for named MediaTek and Qualcomm chips (SM8450 to SM8850; **no Snapdragon 6 Gen 1 build**). Streaming is **5-second sliding windows with overlap** (not true streaming). The card gives no latency or memory numbers I could find. **GGUF**: `ggml-org/Qwen3-ASR-0.6B-GGUF` (Apache-2.0, updated 2026-11-21, 36k dl): Q8_0 805 MB + mmproj Q8_0 214 MB (about 1.0 GB), BF16 1,509 MB + 379 MB; needs a recent llama.cpp; the card says the model emits a language tag first. `handy-computer/Qwen3-ASR-0.6B-gguf` 505k dl. **ONNX**: `rhasspy/qwen3-asr-0.6b-onnx-int4-merged` 785 MB on disk (decoder int4 340 MB, embeddings 311 MB, encoder int4 121 MB); its card measures a Raspberry Pi 5, 4 threads, 3.2 s command: 2.20 s latency (vs 3.42 s for the unmerged export), i.e. **about 0.7 RTF on a Pi 5** for a short command (author figure; not a phone). Core ML: `FluidInference/qwen3-asr-0.6b-coreml` (144 dl). | https://huggingface.co/litert-community/Qwen3-ASR-0.6B ; https://huggingface.co/ggml-org/Qwen3-ASR-0.6B-GGUF ; https://huggingface.co/rhasspy/qwen3-asr-0.6b-onnx-int4-merged ; https://huggingface.co/FluidInference/qwen3-asr-0.6b-coreml |
| Verdict | **Android:** plausible only as the LiteRT int8 (960 MB) or the GGUF Q8 (about 1.0 GB). On 3.4 GiB total RAM that is already about 30% of the device, with speed unmeasured and an autoregressive LLM decoder on a mid CPU. A 0.6B autoregressive decoder will be slower than Moonshine small or a CTC/transducer model. **iPhone:** the Core ML or MLX 4-bit builds are possible (about 0.7 GB) but have no phone number, and 7.07 on the board is worse than Parakeet TDT v2 (6.93), Fluid Speech Pro (6.06) and Rumble-2-Flash (6.60, itself a fine-tune of Qwen3-ASR-0.6B). Not a first benchmark on either target. |

## 9. Forced aligner (word timestamps for takes)

`aufklarer/Qwen3-ForcedAligner-0.6B-4bit` (https://huggingface.co/aufklarer/Qwen3-ForcedAligner-0.6B-4bit), Apache-2.0, 415M params, **979 MB** (bf16 1.84 GB; 8-bit and bf16 variants exist in the collection), MLX; 9.7k dl, created 2026-02-24.
- **What it does:** given audio **and its transcript**, it returns a start and end time per word in a single non-autoregressive forward pass. Resolution 80 ms; 5,000 classes = 400 s maximum (the upstream card says "up to 5 minutes" and 11 languages: zh, en, yue, fr, de, it, ja, ko, pt, ru, es; https://huggingface.co/Qwen/Qwen3-ASR-0.6B-hf).
- **What it gives us:** word times for a held-to-talk take whose text we already have, so a transcript word can seek the audio, a reader's correction can highlight, and podcast quote clips can be cut on a word boundary. It does not recognise speech; it aligns text that exists. Parakeet TDT and Fluid Speech Pro already return word timestamps (NeMo); on Core ML the FluidAudio TDT path has them (not verified for every model above), so the aligner is only needed where the recogniser emits none (Rumble, Whisper-style, Apple's recogniser gives segment-level times).
- **Cost:** an encoder of 24 layers plus a 28-layer decoder, 979 MB, 415M params: heavy for a feature that sits behind a recogniser, MLX only (so iPhone-only, Apple GPU, speed not stated on the card), no Android format found. Recommendation: do not adopt; use the recogniser's own word timestamps. Revisit only if a take needs timing from a text the recogniser did not make.

## 10. WeSpeaker ResNet34-LM MLX

`aufklarer/WeSpeaker-ResNet34-LM-MLX` (https://huggingface.co/aufklarer/WeSpeaker-ResNet34-LM-MLX): MIT, 6.63M params, 27 MB (fp), 80 fbank mel in, 256-d L2-normalised embedding out, BatchNorm fused into Conv2d, Conv weights transposed to channels-last for MLX; Swift via `speech-swift` `SpeechVAD`. 9.4k dl.
- **What the MLX build changes against ours.** We already ship the same network as Core ML (`FluidInference/speaker-diarization-coreml`, `wespeaker_v2.mlmodelc`, 7.95 MB 8-bit palettised, CC-BY-4.0 attribution per our reference page; `docs/reference/ON-DEVICE-MODELS.md:52`). The MLX build is the same pyannote checkpoint (card cites the MIT licence of WeSpeaker; our page lists CC-BY-4.0 for the pyannote conversion, so **the two licence statements differ; the upstream pyannote checkpoint terms should be read once**). It is 27 MB vs 7.95 MB, GPU not ANE, requires the MLX runtime in the app, and does not run on Android. It changes nothing for accuracy (same weights, fusion is arithmetically neutral per the card; no cosine check against PyTorch is given). **Recommendation: no reason to switch; it would add MLX to the app for no gain.** The Core ML build stays the iPhone build; for Android, a WeSpeaker ONNX export would be needed (not examined).

## 11. Every model in the collection `aufklarer/mlx-speech-models` (106 items; https://huggingface.co/collections/aufklarer/mlx-speech-models)

Grouped; all MLX, all by aufklarer. ASR and speech-in:
- Qwen3-ASR 0.6B (4/5/8-bit), 1.7B (4/5/8-bit); Qwen3-ForcedAligner-0.6B (4bit/8bit/bf16)
- Omnilingual-ASR-CTC 300M (4/8-bit), 1B (4/8), 3B (4/8), 7B (4/8)
- Nemotron-3.5-ASR-Streaming-0.6B (bf16/8bit/5bit); Kyutai-STT-1B (FP16/8/4); Cohere-Transcribe-2B (FP16/8/5); Voxtral-Mini-3B-2507 (FP16/8/5); MOSS-Transcribe-Diarize-0.9B-INT8
- Speech front-end and speaker: Silero-VAD v5 and v6.2.1, Pyannote-Segmentation, WeSpeaker-ResNet34-LM, SpeechBrain-ECAPA (VoxLingua107 21M, VoxCeleb 20M), Nemotron-3-Diarization-100M-INT8, WavLM-Base-Plus, DeepFilterNet3, OpenUnmix (HQ, L), HTDemucs-FT, FlashSR, AudioSR
- Voice-to-voice and chat: PersonaPlex-7B (4/8-bit), VoiceChat-11B-Perception (int8/int5), Hibiki-Zero-3B (4/8)
- Text models: Qwen3.5-0.8B-Chat, Qwen3-4B-Instruct-2507 (4/5-bit), gemma-4-E2B-it and E4B-it (2/4-bit), MADLAD400-3B-MT, Clef-flash-9B-4bit
- Speech out and audio generation (not ASR): Qwen3-TTS-12Hz (0.6B/1.7B, Base/CustomVoice, several quantisations), CosyVoice3-0.5B, Magpie-TTS-Multilingual-357M, VibeVoice (Realtime-0.5B, 1.5B), Higgs-TTS-3-4B, Chatterbox-Multilingual (and hi), Fish-Audio-S2-Pro, F5TTS-v1-Base, OmniVoice, VoxCPM2, Voxtral-4B-TTS-2603, FireRedTTS3, CSM-1B, Indic-Mio, MOSS-VoiceGenerator, MusicGen/MAGNeT, Stable-Audio-3, Audio2Face-3D
Relevance: one more ASR entry worth a look is the **Nemotron-3.5-ASR-Streaming-0.6B-MLX** set (same 40-locale model whose English average on the board is poor, 11.09). The Kyutai-STT-1B is a streaming STT of 1B (too big for the 6 GB target alongside the rest; not read further). No ASR in the collection is below 0.6B except the 300M Omnilingual CTC (offline, multilingual, Core ML 495 MB peak on iPhone 16 Pro, 0.28 RTF, vendor figure).

## 12. Small recognisers the first pass skipped

| Model | Params; size | Streaming? | Licence | English accuracy | Phone / CPU numbers | Formats | Source |
|---|---|---|---|---|---|---|---|
| **Moonshine tiny (v1)** | 27.1M; safetensors 108 MB | no (offline) | MIT | not on the card text read | not stated | HF, ONNX (onnx-community 40k dl), LiteRT (litert-community), GGUF (community) | https://huggingface.co/moonshine-ai/moonshine-tiny |
| **Moonshine base (v1)** | 61.5M; 246 MB | no | MIT | not on the card text read | not stated | HF, ONNX (onnx-community 50k dl), GGUF | https://huggingface.co/moonshine-ai/moonshine-base |
| **Moonshine streaming tiny / small / medium** (newest, 2026-02) | 34M / 123M / 245M (card), 176 / 561 / 1,064 MB (fp32 safetensors) | **yes**, 80 ms lookahead | MIT | older 8-set average **12.01 / 7.84 / 6.65** (card) | none on the card; the Moonshine runtime lists iOS and Android (README) | HF, GGUF (handy-computer), runtime library | https://huggingface.co/moonshine-ai/moonshine-streaming-small |
| Moonshine docs / newer release | The README names 10 STT languages and "tiny 1MB models" (a `micro/` directory); I could not open the model docs page (404) so there is **no verified list of newer releases beyond the streaming family** | | MIT by default | | | | https://github.com/moonshine-ai/moonshine |
| **Whisper tiny.en / base.en / small.en** | 37.8M / 72.6M / 241.7M; HF fp 151 / 290 / 967 MB | no (30 s windows) | Apache-2.0 | OpenAI card on LibriSpeech: tiny.en clean 8.44, other 14.86 (small.en values not populated on the card) | none | **whisper.cpp ggml** (https://huggingface.co/ggerganov/whisper.cpp, MIT): tiny.en 78 MB (q8_0 44, q5_1 32), base.en 148 MB (q8_0 82, q5_1 60), small.en 488 MB (q8_0 264, q5_1 190). whisper.cpp itself needs no GPU and has iOS and Android builds in its tree (not re-verified here) | https://huggingface.co/openai/whisper-tiny.en ; https://huggingface.co/ggerganov/whisper.cpp |
| **distil-whisper/distil-small.en** | 166M; ggml 336 MB (fp16), ONNX int8 encoder 92 MB plus merged decoder 80 MB | no | MIT | card: "within 1% WER" of its teacher for the distil family; "within 3% of large-v2, 5.6x faster" for small.en | relative latency only | whisper.cpp ggml, ONNX, safetensors | https://huggingface.co/distil-whisper/distil-small.en |
| Whisper accuracy context | The English leaderboard has whisper-large-v3 at 8.24 and large-v3-turbo at 8.75; the small/base/tiny models are not on the 68-row board and would score worse. | | | | | | section 2 |
| **sherpa-onnx streaming zipformer (English)** | 2023-02-17 20M: encoder int8 **43 MB** (fp32 89 MB); 2023-06-26: encoder int8 **71 MB** (fp32 262 MB), trained on LibriSpeech only; 2023-06-21 (LibriSpeech + GigaSpeech): encoder int8 179 MB; 2023-02-21: 180 MB int8 | **yes** (transducer) | Apache-2.0 (HF tag) | **no WER on the repo or docs page I read** (and the training sets are small; LibriSpeech-only models are known to be weak on conversational audio, which I did not verify) | docs: RTF 0.062 (int8) and 0.077 (fp32) for the 2023-06-26 model; **hardware not stated**, and no Android RTF | ONNX (sherpa-onnx runtime has Android and iOS builds per its docs; not re-fetched) | https://k2-fsa.github.io/sherpa/onnx/pretrained_models/online-transducer/zipformer-transducer-models.html ; https://huggingface.co/csukuangfj/sherpa-onnx-streaming-zipformer-en-20M-2023-02-17 |
| sherpa-onnx + Parakeet-EOU 120M | the same Android runtime path soniqo uses: **Galaxy S23, ONNX INT8, 0.21 RTF, 232 MB** | yes | NVIDIA Open Model License | 9.30 average (160 ms, NVIDIA card) | that single Galaxy S23 row | ONNX | https://soniqo.audio |

## 13. Revised rankings with the additions

**iPhone 14 Pro Max, streaming:** unchanged (Parakeet Unified Core ML; Nemotron-speech-streaming-en Core ML 560 ms; EOU as floor). Added and not promoted: Moonshine streaming small (no Core ML build found, only GGUF/HF/runtime), Qwen3-ASR (not streaming outside vLLM; 5 s windows on LiteRT).
**iPhone, offline:** unchanged (Fluid Speech Pro Core ML, TDT v2, Rumble-2-Flash). Qwen3-ASR-0.6B MLX or Core ML is below all three on the board (7.07 vs 6.06 / 6.93 / 6.60) and has no phone numbers.
**Low-end Android, streaming:** (1) Moonshine streaming small, (2) sherpa-onnx Parakeet-EOU 120M ONNX int8 (the only recogniser with a measured Android phone number, 232 MB, 0.21 RTF on a faster Galaxy S23; WER the weakest of the set), (3) sherpa-onnx zipformer 2023-06-26 int8 (about 71 MB, small and fast, but accuracy unmeasured and English-only LibriSpeech training). Nemotron-speech-streaming LiteRT/GGUF drops to a stretch goal for memory reasons.
**Low-end Android, offline:** (1) Whistle (17 MB), (2) a whisper.cpp small.en q5_1 (190 MB) or distil-small.en as the well-understood baseline: accuracy on the order of Whisper small but unmeasured on this phone, (3) Rumble-2-Flash GGUF (0.50 GB) or Qwen3-ASR LiteRT int8 (960 MB), both heavy. Benchmark first stays Whistle; add whisper.cpp base.en q5_1 (60 MB) as the control, since it needs only a ggml file.
MLX builds: iPhone-only at best, no Android; none enters a top three.

## 14. Added to "what I could not verify"

- Whether MLX runs the Qwen3-ASR 4-bit model at usable speed on an A16 iPhone 14 Pro Max (speech-swift documents only iPhone 16 Pro, Core ML).
- Any Qwen3-ASR latency, RTF or memory figure on any phone (LiteRT, GGUF or MLX).
- The sherpa-onnx zipformer WERs and phone RTFs (docs give RTF with no hardware named); whisper.cpp's platform support (not re-fetched); Moonshine v1 WER and any newer non-streaming release (docs page 404).
- The pyannote WeSpeaker licence (MIT per the MLX card, CC-BY-4.0 per our own reference page).
- Whether the Snapdragon 6 Gen 1 is supported by the LiteRT precompiled builds (it is not in the listed chips).
