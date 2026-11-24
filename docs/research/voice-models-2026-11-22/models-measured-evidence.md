# Measured evidence for on-device speech models (phones and low-end ARM) - research of 2026-11-22

Method note. Every number below was read from a page fetched in this session, through a summarising fetch tool, except where marked "read from PDF text" (I ran pdftotext on the downloaded PDF and read the table myself). Quote-level exactness is therefore high but not perfect for the summarised pages. "Maker" means the party that sells or publishes the model or library being measured. "Independent" means a party with no stake in the model; I flag when independence is doubtful. Items not stated on the page are written "not stated".

## 0. The headline

The independent, measured, on-phone evidence is very thin. Almost every phone number is from the library maker (FluidAudio, soniqo/speech-swift, Cactus, React Native ExecuTorch) and almost no phone number comes with WER. Real independent phone measurements found:

1. I-Parakeet (arXiv 2609.30846, Sony Interactive Entertainment author, not NVIDIA or a library maker): Parakeet-CTC 0.6B on a Nothing Phone (3a), Snapdragon 7s Gen 3 (SM7635). WER, RTF and memory. Strongest single mid-range-Android data point.
2. VoicePing offline TTS evaluation (2026-02-15): 18 TTS entries on a Galaxy S10 and an iPad Pro (A12X). RTF, memory on iOS. No quality test.
3. Sustained-load thermal paper (arXiv 2603.23640): iPhone 16 Pro and Galaxy S24 Ultra. It measures an LLM, not speech, but is the only measured phone throttling data I found.
4. Apple Developer Forums (June 2026): Apple engineer's statement of echo-canceller convergence (~200 ms, >20 dB).
5. Desktop-only independent: Lyonesse/Inscribe SpeechAnalyzer vs Whisper (M2 Pro), 9to5Mac (M2 Pro), Microsoft streaming-ASR paper (AMD EPYC), NVIDIA's own Parakeet v3 noise table (maker).

## 1. Measurement records

Format: model / build | device, chip | runtime, compute unit | metric = value | test set | who | date | URL

### 1a. Phones

| Model / build | Device, chip | Runtime / unit | Metric | Test set | Who | Date | URL |
|---|---|---|---|---|---|---|---|
| Parakeet-CTC 0.6B, I-Parakeet integer-only INT8 weights + INT8/INT16 act. | Nothing Phone (3a), Snapdragon 7s Gen 3 (SM7635) | QNN SDK (QAIRT 2.47), NPU, no CPU fallback | WER 4.97, RTF 0.048, peak mem 612 MiB (RTF includes 23% silence-padding overhead; static graphs 3-35 s) | LibriSpeech test-other, Whisper English normaliser | Independent academic (Sony Interactive Entertainment author); paper "under review" | 2026 (arXiv id 2609.30846; exact date not stated) | https://arxiv.org/abs/2609.30846 (table read from PDF text) |
| same checkpoint, parakeet.cpp FP16 weights / FP32 compute | same phone | CPU, 2 threads | WER 3.76, RTF 0.36, 1517 MiB | same | same | same | same |
| same, parakeet.cpp q8_0 weights / FP32 compute | same phone | CPU, 2 threads | WER 3.76, RTF 0.42, 1045 MiB | same | same | same | same |
| same checkpoint via stock Qualcomm toolchain, FP16 and INT8 PTQ | same phone | NPU | WER 100.0 for both (broken output) | same | same | same | same |
| Parakeet-TDT 0.6B v3 CoreML | iPhone 16 Pro Max / iPhone 13 | Core ML, encoder rawValue 1 | first-load (cold) encoder compile 3361 ms / 4396 ms; warm 162 ms on 16 Pro Max; preprocessor cold 9 ms vs 633 ms | n/a (compile time only) | Maker (FluidAudio) | not stated | https://github.com/FluidInference/FluidAudio/blob/main/Documentation/Benchmarks.md |
| Parakeet-EOU 120M CoreML INT8 | iPhone 16 Pro (A18 Pro, iOS 26.5) | Core ML | RTF 0.04, peak mem 297 MB | not stated; median of 5 after 1 warm-up | Maker (soniqo/speech-swift) | not stated | https://github.com/soniqo/speech-swift/blob/main/docs/benchmarks/ios-coreml.md |
| Kokoro-82M CoreML | iPhone 16 Pro | Core ML | RTF 0.08, 676 MB | same | Maker | not stated | same |
| Supertonic-3 99M CoreML | iPhone 16 Pro | Core ML | RTF 0.15, 956 MB | same | Maker | not stated | same |
| Omnilingual 300M CoreML INT8 | iPhone 16 Pro | Core ML | RTF 0.28, 495 MB | same | Maker | not stated | same |
| Parakeet-EOU 120M ONNX INT8 (streaming, 20 s clip) | Galaxy S23 (SM-S918B), arm64 | ONNX Runtime, CPU only | RTF 0.21, peak RSS 232 MB; no WER | 20 s clip | Maker (soniqo) | not stated | https://soniqo.audio/benchmarks |
| Parakeet-TDT 0.6B ONNX INT8 | Galaxy S23 | ONNX CPU | RTF 0.082, 1.15 GB (1.34 GB before disabling the ONNX CPU memory arena) | 20 s clip | Maker | not stated | same |
| Nemotron Streaming 0.6B LiteRT | Galaxy S23 | LiteRT, CPU | RTF 0.67, 1.30 GB | 20 s clip | Maker | not stated | same |
| Omnilingual CTC 300M LiteRT | Galaxy S23 | LiteRT CPU | RTF 0.15, 831 MB | 20 s clip | Maker | not stated | same |
| Supertonic-3 99M TTS LiteRT | Galaxy S23 | LiteRT CPU | RTF 0.34, 832 MB, time-to-first-audio ~1.1 s | preset voice | Maker | not stated | same |
| Kokoro-82M TTS ONNX FP32 | Galaxy S23 | ONNX CPU | RTF 0.53, 640 MB | preset voice | Maker | not stated | same |
| Parakeet-TDT-0.6B "CQ4" | iPhone 17 Pro / iPhone 15 Pro (also Macs, iPad M5) | Cactus runtime (CPU, INT4 per another snippet; unit not stated on page) | 20 s end-to-end 0.51 s / 1.40 s; no RTF or WER on page | 20 s audio | Maker (Cactus; "measured by" not stated) | not stated | https://docs.cactuscompute.com/v2.0/ |
| Moonshine-tiny (v1) | iPhone 17 Pro / 16 Pro / SE 3; Galaxy S24; OnePlus 12 | ExecuTorch XNNPACK (quantisation not stated) | Encode 5 s audio: 99 / 95 / 115 / 284 / 277 ms. Encode 10 s: 178/177/204/555/528. Decode per token (5 s audio): 49 / 48 / 47 / 37 / 29 ms | not stated; avg of 10 runs | Maker of the RN-ExecuTorch wrapper (Software Mansion), not the model maker; docs say "0.4.x no longer actively maintained" | not stated | https://docs.swmansion.com/react-native-executorch/docs/0.4.x/benchmarks/inference-time |
| Moonshine-tiny streaming mode "fast" latency / tok-s | same devices | same | 0.8 s / 19.0 t/s (17 Pro); 1.5 s / 11.3 (16 Pro); 1.5 s / 10.4 (SE 3); 2.0 s / 8.8 (S24); 1.6 s / 12.5 (OnePlus 12). Balanced: 2.0/3.2/3.7/4.6/3.4 s. Quality: 4.3/6.6/8.0/7.7/6.8 s | not stated | same | not stated | same |
| Whisper-tiny | same devices | same | Encode 30 s (fixed 30 s window): 1034 / 1344 / 1269 / 2916 / 2143 ms; decode 128 / 114 / 142 / 89 / 84 ms/token; streaming "fast" latency 2.8 / 3.7 / 4.4 / 5.5 / 5.3 s | not stated | same | not stated | same |
| WhisperKit models (various) | iPhone 11, 13 Pro, 15 Pro Max | Core ML | speed factors in commit diffs: iPhone 15 Pro Max iOS 18.2 large-v3-turbo 2.41x, base.en 45.25x; 13 Pro large-v2 1.14x; iPhone 11 tiny.en 6.24x | Argmax datasets | Maker (Argmax); read only from a search summary of commit diffs, **not verified on the page** | Oct 2024 (older than 12 months) | https://huggingface.co/spaces/argmaxinc/whisperkit-benchmarks |
| Whisper-Tiny, Zipformer (QNN) | Snapdragon 8 Gen 3 / 8 Gen 1 / 888 | Qualcomm AI Hub, QNN | Whisper-Tiny encoder 12.989 ms on 8 Gen 3; Zipformer encoder 12.0 ms (8 Gen 1) (search-result text, not opened on the model page) | n/a | Maker (Qualcomm) | not stated | https://huggingface.co/qualcomm/Whisper-Tiny , https://huggingface.co/qualcomm/Zipformer |
| 18 TTS entries (7 Android, 11 iOS) | Galaxy S10 (Exynos 9820, 8 GB, 4 threads); iPad Pro 3rd gen (A12X, 4 GB) | sherpa-onnx (CPU) and platform TTS | see Section 4 table | 12 English prompts, warm, 1 warm-up | Independent (VoicePing; author Nakajima; they sell a voice product, but they are not the model maker) | 2026-02-15 | https://voiceping-landingpage-prod.pages.dev/en/blog/research-offline-tts-eval/ |

Qualcomm AI Hub coverage of Snapdragon 6 Gen 1 / SM6450: I found none. The Whisper-Tiny AI Hub page lists only Snapdragon 8 Gen 1, 8 Gen 3, 8 Elite (Galaxy variants); the page says the model is "currently not supported on any Mobile chipset" while still listing the devices (contradiction on the page itself), and gives no latency figures in its text. The nearest measured sibling in the whole search is the Snapdragon 7s Gen 3 in the I-Parakeet paper (a 7-series, newer than SM6450). I did not find any 6-series or 7-series row on AI Hub.

FluidAudio's own page has no per-iPhone RTF or WER table: its iPhone data is Core ML compile/first-load time only (above). All its RTF and WER rows are Mac (M2, M4 Pro, M5 Pro). Treat FluidAudio's "RTFx 100-400" as Mac-only.

sherpa-onnx: no published per-phone RTF table found. Only a Raspberry Pi 4 (Cortex-A72) RTF 0.685 for whisper tiny.en, 1 thread, greedy (per a search summary of the sherpa docs, not opened). The VoicePing paper uses sherpa-onnx on an S10 for TTS. A 2026-05 "verification report" mentioned in a search summary gave Whisper-tiny.en RTF 0.07 on Galaxy S10 via sherpa-onnx; I could not open it (the index page I fetched had no numbers).

ExecuTorch / LiteRT: the React Native ExecuTorch table above is the only Whisper/Moonshine phone table I opened. Arm AI Portal has Whisper int8 LiteRT/ExecuTorch Android pages (appeared in search); I did not open them.

Picovoice: its open STT benchmark (real-time transcription) lists Nemotron 3.5 ASR Streaming, Moonshine, Vosk, Whisper.cpp, Paraformer Streaming and Cheetah, with WER, word-emission latency, core-hours, peak memory, but the numbers are only in images, and hardware is an AMD Ryzen 9 5900X desktop (10 cores), not a phone. Maker-run (Picovoice sells Cheetah). It states Nemotron 3.5 at 2.2 GB model, 5.1 GB peak memory, ~2.5 cores continuous. Noise conditions not specified. https://picovoice.ai/docs/benchmark/real-time-transcription/ . I did not open Picovoice's TTS benchmark (it measures time-to-first-audio and memory for Kokoro and Piper) or the Moonshine streaming page.

### 1b. Desktop or server numbers worth knowing (not phone)

| Model / build | Device | Metric | Test set | Who | Date | URL |
|---|---|---|---|---|---|---|
| Parakeet TDT v3 CoreML | M5 Pro, ANE default | WER 2.27 clean / 4.12 other; RTFx ~150; 60-min Earnings-22 concat WER 16.5 | LibriSpeech, Earnings-22 | Maker | not stated | FluidAudio Benchmarks.md |
| Parakeet Unified int8 (batch / streaming) | M5 Pro, cpuAndNeuralEngine | avg WER 2.15 / 2.21; RTFx 123 / 29 | LS test-clean | Maker | not stated | same |
| Nemotron Speech Streaming 0.6B, int8, chunk 560 / 1120 / 2240 ms | M5 Pro | aggregate WER 2.71 / 2.58 / 2.64; RTFx 40.7 / 24.3 / 87.4 (page also gives a CPU-only-on-iOS ~66 RTFx remark without device) | LS test-clean, 2620 files | Maker | not stated | same |
| Parakeet EOU 120M, chunk 320 / 160 ms | M2 | WER 4.88 / 8.23; RTFx 19.3 / 5.8 | LS test-clean | Maker | not stated | same |
| Nemotron 0.6B streaming ONNX int4 k-quant (0.67 GB) | AMD EPYC 7V12, 32 cores | avg WER 8.20 over 8 ESB sets at 0.56 s delay, RTFx ~7.2; at 0.16 s delay 8.89, RTFx 2.87; FP32 ONNX 8.03; int4 + ConvInteger/MatMulInteger 10.14 (broke); int4 RTN 8.46 | AMI, Earnings22, GigaSpeech, LS clean/other, SPGISpeech, TED-LIUM, VoxPopuli | Microsoft authors (independent of NVIDIA) | April 2026 | https://arxiv.org/abs/2604.14493 (tables via fetch summary; no ARM or phone) |
| Moonshine v2 Tiny/Small/Medium | Apple M3, ONNX Runtime CPU | response latency after VAD end of speech 50 / 148 / 258 ms; compute load 8.0 / 18.0 / 29.0 %; Whisper Tiny 289 ms, Base 553, Small 1940 | Open ASR sets | Maker (Moonshine team) | Feb 2026 | https://arxiv.org/html/2602.12241 . No phone or Pi measurement; Cortex-A55 appears only as back-of-envelope. Their WER (tiny/small/medium avg 12.01 / 7.84 / 6.65; LS clean 4.49 / 2.49 / 2.08; AMI 19.0 / 12.5 / 10.7) are max-claims on server. |
| Apple SpeechAnalyzer, Whisper Tiny/Base/Small (WhisperKit CoreML quantised) | M2 Pro 32 GB, macOS 26.5.1 | WER clean/other: SA 2.12/4.56; Small 3.74/7.95; Base 5.42/12.51; Tiny 7.88/17.04; legacy SFSpeechRecognizer 9.02/16.25. Speed 12x-40x real time overall, SA ~3x faster than Small; no per-engine table; timings may be noisy (dev workload on machine) | LS test-clean 2620 + test-other 2939, no noise added; OpenAI-style normaliser; corpus-level WER | Lyonesse/Inscribe (company blog, product maker; independence unclear, raw transcripts published; Whisper numbers reproduced within 0.11-0.42 points of OpenAI's) | 2026-07-13, updated 07-15 | https://lyonesse.app/blog/apple-speech-api-benchmark.html (redirect from get-inscribe.com). A third-party summary claims "noise-augmented" versions; the primary page says no noise was added. I trust the primary. |
| SpeechAnalyzer vs WhisperKit | M4 Mac mini, macOS 26 beta 1 | WER 14.0 vs 15.2 (base.en) / 12.8 (small.en); speed factor 70 vs 111 / 35 | 10% of Earnings-22, ~12 h | Argmax (maker of WhisperKit; Apple side by open-source script) | 2025-06-20 (older than 12 months) | https://www.argmaxinc.com/blog/apple-and-argmax |
| SpeechAnalyzer vs Parakeet v2 vs Whisper L-v3-Turbo | M2 Pro MBP | WER 10.3 / 12.3 / 1.5; CER 1.9 / 5.8 / 0.2; time 9 s / 2 s / 40 s on a 7:31 podcast; the author's WER varied 5.3-10.3 for Apple depending on normalisation | single 7.5-minute podcast | 9to5Mac (journalist; anecdotal) | 2025-07-03 (older than 12 months) | https://9to5mac.com/2025/07/03/how-accurate-is-apples-new-transcription-ai-we-tested-it-against-whisper-and-parakeet/ |
| SpeechAnalyzer vs whisper.cpp small.en | Mac | WER 1.98 vs 4.28 (40-speaker) per a search summary; publisher sells a dictation app on Apple's path | not opened | vendor | 2026 | https://dev.to/iravoice/apple-speechanalyzer-vs-whispercpp-a-40-speaker-mac-benchmark-40i4 (not opened; HF dataset page opened: clean read speech, M5 Max, whisper.cpp 1.8.4, per-clip WER only) |
| WhisperKit (arXiv 2507.10860) | M3 Max, Neural Engine | hypothesis-text latency ~0.45 s (abstract 0.46 s), confirmed-text ~1.7 s; WER ~2.2; encoder 612 ms (orig) -> 218 ms (d750); LS WER 1.93 FP16 vs 1.96 compressed (0.6 GB) | LS, Earnings-22, CV17 | Maker (Argmax) | July 2025 | https://arxiv.org/html/2507.10860v1 |

## 2. Robustness evidence (not clean read speech)

### 2.1 What the two named enhancement papers actually measured (opened both abstracts via fetch; full text not opened)

- arXiv 2603.04710, "When Audio Separation Hurts Zero-Shot ASR: Evaluating SAM-Audio with Whisper on Bengali and English Speech" (Islam, Nahar, Hamid; submitted Mar 2026, revised Jul 2026, IEEE COMPAS 2026). Front end: SAM-Audio (a generative audio-separation model, not a conventional denoiser). ASR: five Whisper variants (tiny-to-large-v3 range; only base and large-v3 numbers on the page). Data: noisy Bengali and English, datasets not named on the abstract page. Result: PSNR rose (English 32.28 -> 35.99 dB, 71.84% of utterances better) but WER/CER got worse in every model-dataset configuration. Bengali large-v3 WER 65.83 -> 77.35; English base WER 10.53 -> 21.66. Relevance to us: shows enhancement hurts Whisper; not Parakeet, not phones, not echo, not far-field.
- arXiv 2512.17562, "When De-noising Hurts: A Systematic Study of Speech Enhancement Effects on Modern Medical ASR Systems" (Chondhekar et al., 19 Dec 2025). Front end: MetricGAN-plus-voicebank. ASR: Whisper, NVIDIA Parakeet, Gemini Flash 2.0, Parrotlet-a. Data: 500 medical recordings, nine or ten noise conditions (abstract and the "40 configurations" disagree). Metric: semantic WER with medical normalisation. Result: noisy original beat enhanced audio in all 40 configurations; semWER increase 1.1% to 46.6% absolute. Relevance: the one paper that includes Parakeet; supports "do not put a generic denoiser in front of a modern recogniser". Neither paper tests AEC, overlapping talkers, or TTS-playback residue. Neither tests anything on a phone. A caveat I could not resolve without the full text: which enhancer settings, whether the recordings were synthetic noise added to clean audio.
- Conclusion for design: both are "learned denoiser in front of ASR" results, and both say it hurts. That does not transfer automatically to a linear AEC (like Apple's voice processing) that removes a known reference signal. No paper I found compares ASR accuracy with and without AEC on a phone.

### 2.2 Noise, music and far field

- Parakeet-TDT-0.6B-v3 vs Canary-1B-v2, LibriSpeech clean with MUSAN music/noise mixed in (maker paper, NVIDIA, arXiv 2509.14128, Sept 2025; table read from PDF text). WER at SNR 100/50/25/10/5/0/-5 dB: Parakeet v3 1.92/1.92/1.96/2.15/2.62/4.82/12.21; Canary 2.18/2.16/2.01/2.29/2.80/5.08/19.38. Maker-run, additive noise on clean read speech, not far-field, not speech-on-speech. Whisper and Moonshine not in the table.
- FFASR benchmark (Treble + Hugging Face; paper arXiv 2609.38897; launched June 2026): 15,637 held-out utterances, 15 talkers, 14 simulated rooms, nine conditions (anechoic near field, static far field high/mid/low SNR, moving talker). Mean WER across the evaluated systems rises from 4.4% (near field) to 41.3% (static low SNR). Hidden test set, run on one NVIDIA L4, models from the HF Hub. The leaderboard table did not load for me, so I have no per-model numbers; the abstract does not name models. https://treble-technologies-ffasr.hf.space/ , https://arxiv.org/abs/2609.38897 . This is the best candidate "far-field" source to read next, but it is a server-GPU benchmark.
- Open ASR Leaderboard paper (arXiv 2510.06961): clean/short-form English sets, no noise sets (per search summary; not opened). Not evidence for our conditions.
- Babble and competing talker: a search summary said a 2025 study found Whisper weakest on babble and side-speaker noise below 0 dB; I did not open it. NVIDIA's "noisy clinical" FAQ page is marketing with no numbers. I found no measured result for "another voice in the room" or TV/radio background for Parakeet, Nemotron, Moonshine or SpeechAnalyzer. FluidAudio's diariser tables show overlap is hard for diarisation (AMI IHM DER 18-24%, missed speech 9-19% in overlapping-scoring tables), maker-run on Mac.
- Accented English: only Common Voice/Earnings-22 style sets in the sources above; nothing accent-specific found.
- Short commands: nothing found. All opened numbers are on utterances of several seconds or longer. Whisper needs a 30 s padded window on the RN-ExecuTorch runtime (latency table above), which penalises short commands.

### 2.3 Barge-in and Apple voice processing

- Apple Developer Forums thread 829784 (created June 2026; device and OS not stated): app uses .playAndRecord, mode .voiceChat, voice processing enabled on both nodes; the first few hundred ms of a TTS greeting leaked into the input before echo suppression engaged. Apple engineer: AEC needs a short convergence period (typically >20 dB suppression), normally ~200 ms, edge cases slightly longer; suggestion: start playback at lower volume and ramp over ~200 ms. Developer's workaround was to mute recording during playback. https://developer.apple.com/forums/thread/829784
- From other Apple forum threads (seen only in a search summary, not opened): voice processing ducks other audio, output level and bandwidth differ by design (band-limited), and it cannot be enabled while the engine runs. https://developer.apple.com/forums/thread/829771 , https://developer.apple.com/forums/thread/829753 , WWDC19 session 510.
- No measured echo-return-loss or residual-echo figure for Apple VPIO on a given iPhone found anywhere. No peer-reviewed false-interruption study found. EchoChain (arXiv 2604.16456) is about whether cloud full-duplex models revise answers after an interruption, not about acoustics; it did build test audio by pre-synthesising user and barge-in speech with cloned voices and firing the barge-in at a fixed offset from detected assistant onset. Vendor claims (e.g. "neural VAD + AEC cuts false barge-in by up to 92%") are unsourced.

## 3. Streaming vs offline

- First-partial latency measured on a phone, with a recognised methodology: none found for Parakeet EOU, Nemotron streaming, Parakeet Unified, Moonshine streaming, zipformer or SpeechAnalyzer. The closest:
  - RN-ExecuTorch Moonshine-tiny "streaming latency" 0.8 s (iPhone 17 Pro) to 2.0 s (Galaxy S24) in "fast" mode; their definition of latency is not stated on the page.
  - Chunk size is a floor: Nemotron streaming tiers 560/1120/2240 ms (FluidAudio), Parakeet EOU 160/320 ms (WER worsens sharply at 160 ms: 8.23 vs 4.88 on M2). The Microsoft paper shows Nemotron 0.6B WER at algorithmic delay 5.6 s / 1.12 s / 0.56 s / 0.16 s / 0.08 s = 7.10 / 7.46 / 7.28 / 8.04 / 8.91 (PyTorch, server).
  - Parakeet-EOU Core ML INT8 model card (a community port, not opened; from a search summary): ~17 ms mean encoder per chunk, partials about 340 ms after each chunk on M-series; no phone.
- Methodology reference: WhisperKit's paper (arXiv 2507.10860) defines per-word latency as the gap between the audio cursor and the transcript cursor, reported separately for hypothesis (0.45 s) and confirmed text (~1.7 s), using Deepgram's method with TIMIT ground-truth word timestamps. That is the only published definition of first-partial/confirmed latency I found with numbers.
- Moonshine v2: "response latency" = time from VAD end-of-speech to returned transcript (50 ms Tiny on M3); this is a finalisation latency, not first-partial.
- SpeechAnalyzer vs Whisper/Parakeet: see 1b. Desktop only, M2 Pro / M4 / M5 Max. Conditions: LibriSpeech (clean and "other"), Earnings-22, a single podcast. No iPhone measurement of SpeechAnalyzer found in any source. Results on LibriSpeech are favourable to SpeechAnalyzer; on Earnings-22 it is about the same as Whisper base/small; on the podcast Whisper large-v3-turbo wins by a wide margin but is 4x slower than Apple.

## 4. TTS on phones

From VoicePing (independent, 2026-02-15; Galaxy S10 CPU 4 threads; iPad Pro A12X; warm; 12 English prompts):

| Platform | Model (sherpa-onnx unless noted) | Median RTF | Memory MB | Note |
|---|---|---|---|---|
| Android | System TTS | 0.058 | n/r | |
| Android | Piper ryan-low / amy-low | 0.077 / 0.076 | n/r | |
| Android | Matcha-Icefall + HiFiGAN | 0.135 | n/r | |
| Android | Kitten Nano en v0.2 fp16 | 0.387 | n/r | |
| Android | Kokoro en v0.19 | 1.133 | n/r | slower than real time |
| Android | Kokoro Int8 multi-lang v1.1 | 2.423 | n/r | |
| iOS | AVSpeech | tok/s 151 | 21 | |
| iOS | Matcha + Vocos | 0.084 | 211 | |
| iOS | Kitten Nano EN v0.2 / v0.1 | 0.368 / 0.407 | 193 / 108 | |
| iOS | Kokoro EN v0.19 | 0.621 | 833 | |
| iOS | Kokoro Int8 multi-lang v1.0 / v1.1 | 1.822 / 1.569 | 515 / 588 | int8 slower than fp32 here |
| iOS | VITS LJS / VCTK / Melo int8 | 2.02 / 2.06 / 2.87 | 140 / 122 / 211 | |

No listening test (MOS or ABX) was run, time-to-first-audio not measured, one device per platform. Maker numbers on newer hardware: Kokoro-82M Core ML RTF 0.08 on iPhone 16 Pro and 0.53 FP32 ONNX on Galaxy S23 CPU, Supertonic-3 time-to-first-audio ~1.1 s on S23 (soniqo, above). Kokoro on FluidAudio Mac M4 Pro: Swift+Core ML RTFx 23.2, peak 1.5 GB process-wide; on an M5 Pro the ANE variant was 49 ms for 3.35 s of English audio (maker). Picovoice TTS benchmark (not opened) and a "Kokoro vs Supertonic" CPU repo (summary only) exist. Listening or arena evidence including small on-device TTS: none found with a source I could open.

Observation: on Galaxy S10-class CPUs Kokoro does not run faster than real time (RTF 1.1, 2.4 int8); on S23-class CPU it does (0.53). So TTS on a Cortex-A78/A55 mid-low phone is not established by anything measured.

## 5. Thermal and battery

- arXiv 2603.23640 (March 2026): Qwen 2.5 1.5B 4-bit, 20 back-to-back iterations. iPhone 16 Pro (MLX Swift, GPU): peak 40.49 tok/s, "Warm" from iteration 3 at 24.96 (-38%), "Hot" from iteration 17 at 23.67 (-41.5%); battery 100% -> 95%. Galaxy S24 Ultra (MLC-LLM): boost to 12.21 tok/s, plateau 10.38 (-15%) with min 9.55 (-22%); battery 100% -> 93%, ~1.59 W average via BatteryManager (authors caveat accuracy). Duration is minutes, not hours; it is an LLM, not ASR. It supports "expect throttling within a few minutes of continuous GPU load on flagship phones". CPU- and ANE-bound ASR may behave differently; nothing measured.
- WhisperKit paper: moving the decoder KV cache to a stateful model cut energy per forward pass 75% (1.5 W -> 0.3 W), a maker claim from a search summary.
- A Preprints.org paper (202510.0757, not peer reviewed, Oct 2025) measured ASR and keyword-spotting energy on smartphones with short runs and found temperatures within 2 C of baseline; summary only.
- No measurement found of hours-long sustained ASR or TTS on a phone, and no battery-percent-per-hour for ASR.

## 6. Known failure and degradation reports

- I-Parakeet: stock Qualcomm toolchain FP16 and INT8 conversions of Parakeet-CTC 0.6B yielded WER 100.0 on the phone NPU. Naive per-tensor INT8 min-max in simulation: test-other 6.38 vs FP32 3.76 (their method 5.32). The cause given is activation ranges that a single INT8 grid cannot cover (BatchNorm output, pre-encoder). (Read from PDF text.)
- Microsoft Nemotron ONNX: int4 k-quant 8.20 vs FP32 ONNX 8.03 (fine), but replacing the matmuls with ConvInteger/MatMulInteger kernels raised WER to 10.14; FP32 ONNX itself (8.03) is worse than the PyTorch reference (7.28), i.e. the port lost 0.75 absolute before any quantisation. https://arxiv.org/abs/2604.14493
- Core ML Parakeet Japanese repack: the upstream int8 per-channel encoder failed to load on the A15 Neural Engine (E5RT error); the maintainer re-exported from fp16 with 6-bit palettisation (model-card statement seen in a search summary only). https://huggingface.co/josscii/parakeet-0.6b-ja-coreml . A different maintainer reports Parakeet-CTC zh-CN int8 encoder CER 10.45 -> 10.54 (self-reported). This is relevant to us: A15-class ANE and int8 per-channel is a known load failure.
- FluidAudio: SenseVoice fp16 vs int8 WER 3.22 vs 3.25; Paraformer CER 2.12 both; (maker; Mac) i.e. int8 weights alone did not hurt there.
- Whisper family hallucination on non-speech: Jan 2025 study (arXiv 2501.11378; older than 12 months) found a recurring set of hallucination strings on non-speech audio, and music removed from the data because tagging was unreliable. A 2026 preprint (arXiv 2606.07473) reports Whisper large-v3 non-speech hallucination 86.88% -> 27.33% with representation steering; I could not read the PDF text (summary from a search result only). Calm-Whisper (arXiv 2505.12969, May 2025) claims >80% reduction from fine-tuning three decoder heads (title says "calming crazy heads"); PDF unreadable here. I found no study measuring Parakeet, Nemotron, Moonshine or SpeechAnalyzer hallucination on silence or music.
- whisper.cpp low-bit: an arXiv study (2503.09905, Mar 2025) found INT4/5/8 kept accuracy; one GitHub discussion (#3752, old Intel CPU, one 11 s clip) reported q2_k produces hallucination and gibberish on tiny and base. Quantising Whisper-small paper (2511.08093) found aggressive formats lose accuracy in noise. All only from search summaries; not opened.
- Corrections to claims seen on the way: a third-party summary said the Lyonesse benchmark used noise-augmented audio; the primary page says no noise was added. Another summary attributed "Whisper-tiny on Galaxy S10 RTF 0.07" to a source I could not open.

## 7. Candidate coverage: which models have ANY independent phone measurement

| Candidate | Independent phone measurement | Maker/library phone measurement | None |
|---|---|---|---|
| Parakeet-CTC 0.6B (integer-only NPU port) | Yes: Snapdragon 7s Gen 3, WER 4.97 test-other, RTF 0.048 (needs a custom port, not a shipped build) | | |
| Parakeet-CTC 0.6B via parakeet.cpp FP16/q8 | Yes: same phone, CPU 2 threads, RTF 0.36-0.42, WER 3.76 test-other | | |
| Parakeet TDT 0.6B v3 | | iPhone 16 Pro Max/13 Core ML compile times (FluidAudio); iPhone 17 Pro/15 Pro 20 s latency (Cactus, CQ4); Galaxy S23 CPU ONNX RTF 0.082 (soniqo) | WER on any phone |
| Parakeet EOU 120M | | iPhone 16 Pro RTF 0.04; Galaxy S23 CPU RTF 0.21 (soniqo) | WER on any phone; first-partial latency on any phone |
| Nemotron streaming 0.6B | | Galaxy S23 LiteRT CPU RTF 0.67, 1.30 GB (soniqo) | iPhone, WER on any phone; any Android below flagship |
| Parakeet Unified | | | any phone |
| Moonshine tiny (v1) | | iPhone 17 Pro/16 Pro/SE 3, Galaxy S24, OnePlus 12 encode/decode times (RN-ExecuTorch wrapper maker) | WER on phone; Moonshine v2 streaming on any phone |
| Whisper tiny/base | | RN-ExecuTorch same devices; WhisperKit iPhone 11/13/15 speed factors (Argmax, 2024); Qualcomm AI Hub (flagship chips) | independent phone WER |
| Apple SpeechAnalyzer/SpeechTranscriber | | | any iPhone measurement; only Mac measurements |
| Zipformer streaming (sherpa-onnx) | | Qualcomm AI Hub encoder latency on 8 Gen 1 | sherpa-onnx phone RTF, WER on phone |
| Silero VAD / FSMN-VAD | | | any phone; FluidAudio Mac only (M1/M5 Pro) |
| Kokoro-82M | Yes (S10 Android RTF 1.13, iPad A12X RTF 0.62, 833 MB) | iPhone 16 Pro RTF 0.08, S23 RTF 0.53 | time-to-first-audio on phone (only Supertonic S23 ~1.1 s from maker) |
| Piper / Matcha / Kitten / VITS (sherpa-onnx) | Yes (S10, iPad A12X) | | quality evidence |
| Supertonic-3 | | iPhone 16 Pro RTF 0.15, S23 RTF 0.34 | independent |

## 8. What the evidence says is best, by hardware class

A16-class iPhone (iPhone 14 Pro / 15 / 15 Plus): no source measured an A16. Nearest data: iPhone 15 Pro (A17 Pro) Cactus Parakeet-TDT CQ4 20 s in 1.40 s (RTF ~0.07, maker); iPhone 13 (A15) cold Core ML encoder compile 4.4 s vs 3.4 s on 16 Pro Max (maker); A15 ANE rejects int8 per-channel encoders in at least one Core ML Parakeet port. iPhone 16 Pro: Parakeet-EOU Core ML RTF 0.04, Kokoro Core ML RTF 0.08 (maker, WER not reported). On the evidence, "best measured on A16-class" cannot be answered with independent data. What can be said: Parakeet-family Core ML is demonstrably fast on iPhone (maker data); SpeechAnalyzer has no iPhone measurement at all; Whisper-tiny/Moonshine-tiny through ExecuTorch XNNPACK are slower and fixed-window.

Mid/low Snapdragon or Cortex-A78/A55 CPU: only I-Parakeet (SD 7s Gen 3, which has A720/A520 cores, not A78/A55) is independent. It shows CPU Parakeet-CTC 0.6B at RTF 0.36-0.42 with 1.0-1.5 GB, i.e. CPU is real time but heavy; the NPU port is RTF 0.048 but needs custom integer work (stock Qualcomm toolchain gave WER 100). Galaxy S23 (SD 8 Gen 2) CPU: Parakeet-EOU 120M RTF 0.21, 232 MB (maker) is the lightest streaming option measured. Moonshine-tiny on Galaxy S24/OnePlus 12 encodes 10 s in about 0.53-0.56 s (RTF ~0.055) with XNNPACK. Nothing measured on Snapdragon 6 Gen 1 / SM6450 or on any pure A55-class core, and Qualcomm AI Hub has no 6- or 7-series row that I could find.

## 9. Biggest gaps we must measure ourselves

1. WER, not just RTF, for every shipped build (exact quantisation and Core ML or ONNX conversion) on the real target phone, with the same text normaliser. Almost all phone data lacks WER.
2. First-partial latency and finalisation latency of each streaming candidate on phone, end-to-end from microphone timestamp (no one has published this).
3. SpeechAnalyzer on an actual iPhone (A16-class): speed, memory, behaviour on short commands, and noise/background-media WER. No source.
4. The target low-end Android (SM6450 / A78+A55): RTF, memory, WER, thermal. Zero published data on this class for any candidate except the 7s Gen 3 paper.
5. Robustness conditions: second talker in the room, TV/radio/music background, accented English, short commands, far-field. No published result for any candidate on these. FFASR leaderboard may have server numbers; go read its table.
6. Barge-in: residual TTS echo after Apple VPIO, false-interruption rate, time to stop TTS; plus effect of VPIO on recognition accuracy (nobody measured). Also ASR WER with and without VPIO and with TTS playing.
7. Silence/music/TV hallucination for the Whisper family and, as a negative control, for Parakeet/Nemotron/Moonshine/SpeechAnalyzer: unmeasured.
8. Sustained 30-60 minute listening: thermal state, battery % per hour, with a pocketed phone; none exist for ASR.
9. TTS time-to-first-audio and memory on phones, and listening-test quality of small models.

## 10. Protocol details we may be missing

- Fix one text normaliser and publish it. Evidence it matters: Apple WER ranged 5.3-10.3 on one file in 9to5Mac depending on normalisation; Lyonesse mirrored OpenAI's English normaliser and reproduced Whisper's published WER to within 0.11-0.42 points; I-Parakeet used the Whisper English normaliser; WhisperKit's medical-style semantic WER differs from plain WER. Report corpus-level WER (total errors / total words), not mean of per-utterance WER (FluidAudio reports both "avg" and "aggregate"; they differ by up to 0.6 points).
- Count empty output as 100% WER for that utterance (Lyonesse did) so silent failures show.
- Static-graph NPU runtimes pad to fixed lengths (I-Parakeet: +23% frames; Whisper 30 s fixed window): report RTF including padding, and test short commands specifically because these designs hurt them.
- Cold vs warm: first Core ML load of the Parakeet v3 encoder takes 3.4-4.4 s, warm 160 ms; RN-ExecuTorch says initial runs can be up to 2x slower. Report both and say whether the model was ANE-compiled at install.
- RTF definitions differ: soniqo RTF = wall time/audio (lower is better), FluidAudio RTFx = audio/wall (higher is better), Argmax "speed factor" = audio seconds per second; Microsoft notes RTFx is only comparable within one hardware and baseline. Always write the direction.
- First-partial latency: adopt the WhisperKit/Deepgram method (per-word audio-cursor vs transcript-cursor gap, with ground-truth word timestamps; hypothesis and confirmed text separately), and add an end-of-speech to final-transcript measure (Moonshine's definition, VAD-triggered). Also log chunk size, since it is the floor (160 vs 320 vs 560 ms changes both latency and WER).
- Barge-in recordings: follow the EchoChain construction: pre-synthesised user and interruption utterances with several cloned or distinct voices so every model gets identical input, and fire the interruption at a fixed offset from the detected assistant-speech onset. Add what Apple's engineer implies: also fire it inside the first ~200-400 ms of playback (AEC convergence window) and at several playback volumes, with the phone at its real speaker/mic geometry, and capture the raw mic signal and the VPIO output together. Measure false stops (noise or affirmation stopping the agent) and missed interruptions separately.
- Background media and overlap: mix at stated SNRs (the NVIDIA table spans 100 to -5 dB with MUSAN; FFASR uses three SNR bands plus moving talker) but also play real TV/radio through a second speaker in a real room, because additive mixing on clean audio is the only noise evidence that exists for Parakeet.
- Enhancement A/B: the two papers say a learned denoiser hurt every tested recogniser, so run raw vs enhanced vs AEC-only for each candidate; do not assume enhancement helps.
- Thermal protocol: run at least 30-60 minutes; log thermal state (iOS ProcessInfo.thermalState; Android thermal API), per-chunk latency and battery; fix screen state and ambient temperature (the 2603.23640 study held 22 C +/- 2 and discarded a warm-up run); note its warning that Android BatteryManager readings are unreliable under load.
- Record exact build identifiers: quantisation, compute unit (rawValue 1/2 or cpuAndNeuralEngine), OS version (macOS beta and iOS point releases changed results), runtime version.
- Dataset sizes and seeds: Whisper-tiny on LibriSpeech varies 7.88 (clean) to 17.04 (other); use both clean and a harder set, plus our own recorded in-room set.

## 11. What I could not open or verify

- FluidAudio page gives no per-iPhone RTF or WER (only compile times); WhisperKit BENCHMARKS.md has no numbers; the live WhisperKit benchmark Space was not opened (iPhone figures are from search-result commit diffs, Oct 2024).
- Picovoice result figures (images only); Picovoice TTS and Moonshine pages.
- FFASR leaderboard table (did not load); FFASR full paper; the full texts of 2603.04710 and 2512.17562 (abstract-level only, which is what was fetched).
- 2606.07473 and Calm-Whisper PDFs (compressed, unreadable by the fetcher); only summaries.
- sherpa-onnx RTF pages for Pi/phones; Arm AI Portal Whisper pages; Qualcomm AI Hub individual profile jobs; the "SOTA On-Device STT in 2026" verification report (page opened had no data); IraVoice dev.to post; whisper.cpp discussion #3752; HN thread on SpeechAnalyzer.
- Cactus docs omit "measured by", dates and WER; a search summary claimed an RTF and WER table for a MacBook Air M3, but the page I fetched showed only a Parakeet CQ4 20 s-latency table.
- Parakeet EOU and Nemotron first-partial latency on a phone: nothing found. Moonshine v2 on phone: nothing found.
- The fetch tool summarises pages with a small model; one page (arXiv 2604.16456) contained text instructing the summariser about formatting, which I treated as page content and ignored.

## Source list

- https://github.com/FluidInference/FluidAudio/blob/main/Documentation/Benchmarks.md
- https://arxiv.org/abs/2603.04710
- https://arxiv.org/abs/2512.17562
- https://arxiv.org/abs/2609.30846 (and the PDF text)
- https://arxiv.org/abs/2604.14493
- https://arxiv.org/html/2602.12241
- https://arxiv.org/html/2507.10860v1
- https://arxiv.org/pdf/2509.14128
- https://arxiv.org/html/2603.23640
- https://arxiv.org/abs/2609.38897 and https://treble-technologies-ffasr.hf.space/
- https://lyonesse.app/blog/apple-speech-api-benchmark.html
- https://www.argmaxinc.com/blog/apple-and-argmax
- https://9to5mac.com/2025/07/03/how-accurate-is-apples-new-transcription-ai-we-tested-it-against-whisper-and-parakeet/
- https://huggingface.co/datasets/researchaudio/apple-speechanalyzer-vs-whisper-cpp-mac
- https://soniqo.audio/benchmarks
- https://github.com/soniqo/speech-swift/blob/main/docs/benchmarks/ios-coreml.md
- https://docs.cactuscompute.com/v2.0/
- https://docs.swmansion.com/react-native-executorch/docs/0.4.x/benchmarks/inference-time
- https://voiceping-landingpage-prod.pages.dev/en/blog/research-offline-tts-eval/
- https://picovoice.ai/docs/benchmark/real-time-transcription/
- https://aihub.qualcomm.com/mobile/models/whisper_tiny
- https://developer.apple.com/forums/thread/829784
- https://arxiv.org/html/2604.16456v1
- https://github.com/argmaxinc/WhisperKit/blob/main/BENCHMARKS.md
