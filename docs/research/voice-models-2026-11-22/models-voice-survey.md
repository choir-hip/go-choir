# On-device voice models survey (fetched 2026-11-22)

Every figure carries its source URL. "not stated" = the page does not say. Hugging Face (HF) list pages gave unlabeled counts; fetcher inferred downloads/likes, treat list-page counts as approximate. WebFetch summarises pages with a small model: numbers were not re-read against raw HTML.

## 0. What the app uses today

Sources: docs/reference/ON-DEVICE-MODELS.md (sections 2, 8, 9, 10), mobile/AutoradioKit/Sources/AutoradioKit/ModelCandidates.swift, KnownModels (approxBytes lines 109-129), grep of Sources.

| Job | Model in code | Runtime | Size | Notes |
|---|---|---|---|---|
| Voice, default | Kokoro 82M, voice af_heart (+4 voices, 2.7 MB each) | Core ML via FluidAudio (`kokoro-82m-ane`) | about 350 MB (relayed); repo ANE-v3 folder 728.8 MB incl. ja/zh | Does not stream; "unvalidated on physical iPhone" per vendor (ON-DEVICE-MODELS 8.1, 9). Apache-2.0 |
| Voice, alternative | Phonon 31M (Gradium) | Core ML, FluidAudio PocketTtsSmallManager | 173 MB pack (64.5 MB q8 GGUF original) | streams 80 ms frames; English; Apache-2.0; no iPhone measurement (section 10) |
| Voice, alternative | Pocket TTS (alba) | Core ML FluidAudio | not recorded | section 9 |
| Voice, floor | system voice (AVSpeechSynthesizer) | system | 0 | |
| Recogniser | Parakeet TDT 0.6B v2 (English) | Core ML | about 480 MB (KnownModels approxBytes) | CC-BY-4.0 |
| Recogniser, streaming/end-of-utterance | Parakeet EOU 120M | Core ML | about 250 MB | NVIDIA Open Model License; measured on A16 per mobile/README |
| Recogniser, floor | Apple SFSpeechRecognizer (AppleStages.swift) | system | 0 | |
| VAD | Silero VAD v6.2.1, 256 ms variant | Core ML (FluidInference/silero-vad-coreml) | about 2 MB | MIT; measured on A16 |
| End of turn | Smart Turn v3.2 | Core ML port | 8M params | Mac 3.5 ms/window; not run on phone |
| Speaker profile | WeSpeaker ResNet34-LM | Core ML directly (WeSpeakerEmbedder.swift) | 7.95 MB, 256-d | CC-BY-4.0; thresholds unverified on phone |
| Small LM (decisions) | Qwen3-0.6B Q8_0 (639 MB) / Q4_K_M (397 MB); also Qwen3-1.7B, Qwen3.5, LFM3-1.7B pinned | llama.cpp b12259 GGUF | see table at ON-DEVICE-MODELS section 8.3 | Qwen3-0.6B is the 6 GB floor choice |
| Embeddings | EmbeddingGemma 2 Q8_0 | llama.cpp GGUF | 309,855,456 B | EMBEDDINGGEMMA.md |
| Speech-to-speech (lab) | LFM3-Audio-2B | Core ML FluidAudio | ASR about 2.5 GB | not on phone |

The doc's own resident-memory planner: Mac proof chose, for a 6 GB phone, LFM3 QAD Q4_0 + Parakeet TDT + Pocket voice + Silero (ON-DEVICE-MODELS line ~378).

## 1. The four linked items

### 1.1 tiny-voca (text-to-speech) - https://huggingface.co/n4ze3m/tiny-voca
- Job: English TTS, two fixed American voices (F2 female, M2 male). Publisher Muhammed Nazeem (n4ze3m).
- Params 4,537,291 (4.5M) including waveform generator. Source-filter synthesiser, no neural vocoder. Trained from scratch on 17.6 h of synthetic speech (Qwen3-TTS generated) on one RTX 4090, 7.7 h, $9.56 total.
- Files: PyTorch .pt 18.2 MB fp32; ONNX 21.4 MB fp32 opset 17; GGUF Q4_0 8.7 MB, Q8_0 10.9 MB, F16 14.9 MB (two more variants unlisted); phonemizer data 5.4 MB (dizione). Not published: CoreML, MLX, LiteRT, ExecuTorch.
- Licence: Apache-2.0 weights and code; phonemizer data derives from CMU dict etc. Commercial use: Apache-2.0 allows it; the synthetic training data came from Qwen3-TTS (output terms of that model not checked by me).
- English only. Output 24 kHz mono.
- Quality: WER (Whisper small.en) val 0.6%, test 0.9%; "no formal listening test". Open TTS Leaderboard v1.7.0, 20 Nov 2026: F2 rank 5 (WER 0.74%, UTMOS22 4.43), M2 rank 7 (0.76%, 4.35); 45 ranked models.
- Speed: "about 70x faster than real time on four CPU threads" (CPU model not stated). Memory: not stated. Streaming: no (one utterance up to 512 tokens or 32 s per call; sentence splitting in the Python tool). Voice cloning: no. Prosody controls: speed, pitch shift, pitch range. Deterministic.
- Maturity: 10,768 downloads last month, 419 likes, 4 discussions, 10 Spaces (page, 2026-11-22). Single author, weeks old. Known weaknesses on the page: slightly synthetic, F2 sounds young, foreign names mispronounced, numbers/abbreviations depend on phonemizer.
- Our job: a candidate voice replacing Kokoro (82M, about 350 MB) at about 9-22 MB, especially for the low-end Android. Both voices are plainly synthetic-sounding "radio" voices rather than expressive. ONNX runs on onnxruntime mobile on both platforms; no Core ML build, so on iPhone it is ONNX/CPU unless converted. Needs dizione phonemizer ported (check it exists for Swift/Kotlin: not stated; github.com/n4ze3m/dizione).

### 1.2 Who and Eye - https://desertant.com/blog/who-and-eye/ (Daniel Duke, Desert Ant Labs, 2026-10-20)
Desert Ant SDK 4.0 adds two models. Page: no source code linked; points to model pages.
- Who = speaker diarisation (who speaks when, anonymous numbering by first speech, overlaps kept, `keepingSpeakers(minimumShare:)`). Eye = image classification/photo ranking (558 scene categories, closed eyes, crops) - irrelevant to our app.
- Detail from https://desertant.com/models/who/ (SDK 4.3.0; HF repo huggingface.co/desert-ant-labs/who; SDK github.com/Desert-Ant-Labs/desert-ant-core):
  - Sizes: Core ML 81 MB fp16 or 42 MB 8-bit (same 12.6% AMI DER, "no memory savings"); LiteRT 32 MB; ONNX 99 MB (not yet run by SDK). Parameters: not stated.
  - Memory: 241 MB loaded; peaks 279 MB (8-min), 377 MB (34-min), 867 MB (2.65 h).
  - Speed: 953x realtime on M5 Pro Neural Engine; Android is LiteRT on CPU, "3 to 7 times slower than the Neural Engine" (blog). No phone or Android figures given.
  - DER (Who / pyannote community-1 / FluidAudio): AMI 12.6 / 13.1 / 19.4; NOTSOFAR-1 16.2 / 18.7 / 32.7; VoxConverse 8.4 / 7.3 / 9.6; DiPCo 41.1 / 33.4 / 42.7; Earnings-21 27.7 / 7.3 / 12.0. (Page text contradicts itself on Earnings-21: says 16% wrong-speaker.)
  - Streaming: not mentioned (processes 5-minute sections). Languages: not stated. It does not identify named speakers or enroll a voice.
  - Licence: pricing says free up to 100k monthly active devices per model; weights/SDK open-source licence not stated. Contact for custom licence. Commercial use: free below 100k MAD per model per the page.
  - Platforms: Core ML iOS 18+; LiteRT Android/Linux/Windows/web.
- Our job: offline post-hoc diarisation of recordings (podcast clips, takes). It is not a speaker verifier ("is this the owner?"), so it does not replace WeSpeaker profile matching. Not streaming, so not for live hands-free addressing.

### 1.3 Voz (speech recognition) - https://huggingface.co/desert-ant-labs/voz
- Job: ASR with word timestamps; Parakeet TDT 0.6B v3 converted/compressed. Weights v0.5.0, 2026, no exact date.
- Size 337 MB (vs 1.6 GB Whisper large-v3-turbo); browser 409 MB. Formats: Core ML (.mlmodelc), LiteRT (.tflite) for Android/Linux, ONNX fp16 (Windows), ONNX web. No GGUF/MLX/ExecuTorch.
- Languages 25 European; no language ID (use "Ear"). Not English-only.
- Quality: WER 6.43% mean over six Open ASR Leaderboard sets vs Whisper large-v3-turbo 7.00%; clean audiobooks about 2%; meetings/podcasts 8-11%; word start error 80 ms.
- Speed (10 min audio, Neural Engine): iPhone 18 Pro 0.98 s (615x), M5 1.15 s, M6 0.84 s, M3 Ultra 0.72 s. Page says Android LiteRT figures "don't carry over to phone CPUs"; none given.
- Memory: constant for any file length; browser about 1.2 GB; phone figure not stated. Streaming: live sessions in Swift via LiveConfiguration (Kotlin not stated). First load 10-30 s Apple.
- Licence: Desert Ant Labs Source-Available License: "free for most apps, commercial licence required at scale"; terms at https://license.desertant.com/ (not read). Parakeet base CC-BY-4.0. Not OSI open.
- Maturity: 1,470 downloads last month, 61 likes. SDKs: Swift 5.0.0 (desert-ant-core), Kotlin `ai.desertant:voz:5.0.0`, JS.
- Our job: replaces Parakeet TDT v2 (480 MB) as recogniser and adds Android via one shared SDK. At 337 MB it is nearly as large as our whole Android budget; see section 4.

### 1.4 Fluid Speech Pro (speech recognition) - https://huggingface.co/FluidVoice-Labs/fluid-speech-pro
- Job: ASR (not TTS, not speaker). Publisher FluidVoice (fluidvoice.com), listed Nov 2026. Built on NVIDIA Parakeet TDT 0.6B v3 and "Moondream Parakeet Ultra" (both CC BY 4.0). 0.63B params, FastConformer + TDT + one extra block.
- Languages: 25 European, auto-detected.
- Quality: Open ASR Leaderboard 6.06 average WER, #2 among open models; public-set 5.43; AMI 7.55, Earnings-22 5.42, GigaSpeech 7.51, SPGISpeech 2.31, URGENT 9.83, VoxPopuli 1.70. Multilingual 4.88 vs Parakeet v3 5.52. RTFx 3,724 on one GPU.
- Formats (separate repos): NeMo/Transformers (F32 safetensors, size not listed), ONNX, MLX, GGUF (parakeet.cpp), Core ML. No LiteRT or ExecuTorch.
- Core ML repo https://huggingface.co/FluidVoice-Labs/fluid-speech-pro-coreml : about 652 MB, encoder 8-bit; "tested on Mac only so far"; iOS minimum not stated; iPhone speed not stated; M5 Max 25 ms per 15 s window; about 250 MB peak memory; 3.43 WER on 400 English clips; greedy only; no word timestamps or clean mode in that build; loads through a fork `github.com/altic-dev/FluidAudio` (not the upstream FluidInference one we pin). 359 downloads.
- Streaming: not described (15 s windows).
- Licence CC BY 4.0, commercial use stated allowed, attribution "Fluid Speech Pro by FluidVoice".
- Maturity: main repo 9,819 downloads/month, 178 likes; ONNX 751, GGUF 1.18k, MLX 469; org also has "fluid-speech-mini" (gguf 3.5k, coreml 1.04k, onnx 372, Oct 13; not opened by me) and a Qwen3.5-9B-4bit repo. https://huggingface.co/FluidVoice-Labs. No phone SDK named by the org.
- Our job: would replace Parakeet TDT (our fluid-based Core ML path) with lower WER but 652 MB; no streaming. Does not add the hands-free recogniser or speaker job. Questionable gain over Parakeet v2/v3 for English only; mostly bigger.

Honest summary: of the four, only tiny-voca is a voice. Voz and Fluid Speech Pro are both Parakeet-TDT-0.6B-v3 derivatives (recognisers). Who is diarisation.

## 2. Candidates per job

### 2.1 Text-to-speech
List: https://huggingface.co/models?pipeline_tag=text-to-speech&sort=trending (trending, 2026-11-22; top includes tiny-voca, Zonos3-Mini, Reverb-TTS, OmniVoice, Kitten TTS 2 mini, Kokoro-82M 13.3M downloads, gradium/phonon-31m-en 363k downloads, Supertonic 3).

| Model | Params / size | Format | Licence | Languages | Voices | Speed (hardware) | Streaming | Source |
|---|---|---|---|---|---|---|---|---|
| tiny-voca | 4.5M; ONNX 21.4 MB, GGUF Q4_0 8.7 MB | ONNX, GGUF, PT | Apache-2.0 | en | 2 fixed | 70x RT, 4 CPU threads | no | https://huggingface.co/n4ze3m/tiny-voca |
| kale-tts-1-nano | 7.5M; ONNX int8 14.8 MB | ONNX | Apache-2.0 | en (US/UK) | 10 fixed, synthetic | 44x RT on Galaxy Tab S10 Lite (Exynos 1380, 8 cores) Android ORT CPU; 0.09 s to first sentence; 22x on 1 thread; iOS not tested | yes, sentence by sentence | https://huggingface.co/KaleComputer/kale-tts-1-nano |
| Kitten TTS 2 mini | 82M; int8 ONNX 82.7 MB, int4 47.5 MB | ONNX | Apache-2.0 | 16 | 118 | not stated | not stated | https://huggingface.co/KittenML/kitten-tts-2-mini |
| gradium Phonon 31M (in use) | 41.1M; 55-64 MB GGUF; 173 MB CoreML pack | GGUF, Core ML | Apache-2.0 (31M en only; 40M variants CC BY-NC) | en | about 100 | vendor 28 ms first audio on Arm CPU | yes | ON-DEVICE-MODELS section 10; list page above |
| Kokoro 82M (in use) | 82M; about 350 MB CoreML | Core ML, ONNX, others | Apache-2.0 | multi | 5 used | none by us | no (vendor) | ON-DEVICE-MODELS section 9 |
| Reverb-TTS | 297M; int4 safetensors 191 MB, int8 307 MB | ExecuTorch (MLX, Vulkan, QNN int4) | weights CC BY-NC 4.0 | 7 | 16 + cloning | iPhone 17 ExecuTorch MLX fp16 20.6x RT; no Snapdragon figure | not stated | https://huggingface.co/software-mansion/Reverb-TTS |
| Supertonic 3 | about 99M; size not stated | ONNX | model OpenRAIL-M, code MIT | 31 | fixed presets | none in text | not stated | https://huggingface.co/Supertone/supertonic-3 |

Disqualifiers: Reverb-TTS weights are non-commercial (out). Supertonic 3 has OpenRAIL-M use restrictions to read and no sizes or numbers on the page. Kitten 2 mini has no speed numbers and 118 voices at 83 MB int8 is uncertain for memory. kale and tiny-voca are weeks old, single author, synthetic data trained.
Only Phonon and kale are shown streaming.

Rank (a) iPhone 14 Pro Max: 1 Kokoro (in use, memory fine; benchmark first-sound latency since it does not stream); 2 Phonon 31M (streams, Core ML, already built); 3 tiny-voca or kale (ONNX CPU only on iPhone; benchmark only if size matters). Benchmark first: Phonon vs Kokoro first-audio on the phone, then memory with recogniser resident.
Rank (b) low-end Android: 1 kale-tts-1-nano (only candidate with a measured mid-range Android CPU number: 44x RT, 14.8 MB, streaming); 2 tiny-voca (8.7-21 MB, 70x RT on desktop 4 threads only); 3 Phonon 31M (55 MB GGUF q4k; Android runtime `gradium-ai/xn-ptts` mentioned only on card as iPhone, not read; no Android number). Kokoro (350 MB+) is likely too heavy. Benchmark first: kale and tiny-voca on Snapdragon 6 Gen 1 for RTF and listening quality over 10 minutes of news reading. Disqualify if RTF < about 5x under load with recogniser running, or if numbers/names/abbreviations in news text are mispronounced (both rely on phonemizers; tiny-voca page admits this).

### 2.2 Voice activity detection / turn end
List: https://huggingface.co/models?pipeline_tag=voice-activity-detection&sort=trending (mostly diarisation/segmentation; Silero is not a top trending HF entry because it is distributed on GitHub; HF has many ports: https://huggingface.co/models?search=silero-vad&sort=downloads).

| Model | Size | Format | Licence | Speed / hardware | Source |
|---|---|---|---|---|---|
| Silero VAD v6.2.1 (in use) | about 2 MB (app approxBytes; TEN card lists Silero 2.16 MB JIT / 2.22 MB ONNX) | Core ML (FluidInference), ONNX, GGUF, MLX | MIT | no numbers on card | https://huggingface.co/FluidInference/silero-vad-coreml (71.1k dl, 31 likes) |
| TEN VAD | library 320 KB iOS, 373-532 KB Android; model size not stated | ONNX + prebuilt C libs (iOS framework, Android .so) | Apache-2.0 "with additional conditions" (read NOTICES) | RTF 0.0049 iPhone 8 (A11), 0.057 Galaxy J6+, 0.049 Oppo A3s; M1 0.016 | https://huggingface.co/TEN-framework/ten-vad (1,922 dl, 157 likes) |
| Smart Turn v3 (end of turn, not VAD) | 8M params; int8 ONNX 8 MB | ONNX | BSD-2-Clause | no figures on page | https://huggingface.co/pipecat-ai/smart-turn-v3 (225 likes) |

Also seen, not examined: funasr/fsmn-vad, pyannote/segmentation-3.0 (4.4M dl, list page above).
Both VADs are tiny, so memory is not an issue. Rank (a): Silero (already measured on A16), TEN VAD second, Smart Turn for end of turn only. Rank (b): TEN VAD first (it is the only one with Android low-end RTF published: 0.049-0.057 on old phones, vendor figures), Silero second (ONNX via onnxruntime or the C++ ports), Smart Turn third (add only if endpointing misfires). Benchmark first: TEN VAD vs Silero false positives on radio-playback audio (the radio's own TTS bleeding into the mic is our real test; neither card tests that). Disqualify TEN VAD if the "additional conditions" in its licence restrict us (not read); disqualify any VAD that flags TTS output as speech more than Silero.

### 2.3 Speaker verification / diarisation
List: https://huggingface.co/models?pipeline_tag=audio-classification&search=speaker&sort=trending showed mostly irrelevant small repos; the useful hits were found by name.

| Model | Params / size | Format | Licence | Numbers | Source |
|---|---|---|---|---|---|
| WeSpeaker ResNet34-LM (in use) | 7.95 MB Core ML, 256-d | Core ML | CC-BY-4.0 | cosine >= 0.997 vs PyTorch; phone accuracy none | ON-DEVICE-MODELS section 2 |
| CAM++ Core ML (aufklarer) | 6.9M; about 14 MB; 192-d | Core ML fp16 | Apache-2.0 | no EER; no timings | https://huggingface.co/aufklarer/CamPlusPlus-Speaker-CoreML (407 dl) |
| ECAPA-TDNN ExecuTorch (mlboydaisuke) | 20.8M; XNNPACK fp32 83.9 MB, Core ML fp16 42.5 MB; 192-d | ExecuTorch .pte | Apache-2.0 | 23.7 ms XNNPACK vs 2.4 ms Core ML on Mac arm64; no EER (synthetic voices same 0.727-0.844, different -0.045 to 0.074) | https://huggingface.co/mlboydaisuke/ECAPA-TDNN-Speaker-ExecuTorch (28 dl) |
| Desert Ant Who (diarisation, offline) | 32 MB LiteRT, 81 MB Core ML | LiteRT, Core ML | free up to 100k MAD per model | 241 MB loaded | https://desertant.com/models/who/ |
| Nemotron-3-Diarization (NVIDIA) | not opened | MLX 8-bit exists | not read | in HF VAD list: 241k dl, 851 likes | https://huggingface.co/models?pipeline_tag=voice-activity-detection&sort=trending |

Rank (a): WeSpeaker (in use) then CAM++ (similar size, Apache-2.0 vs CC-BY attribution), Who only for offline clips. Rank (b): the LiteRT-capable choices are few: ECAPA ExecuTorch XNNPACK (84 MB, runs Android CPU; heaviest) or CAM++ via ONNX (CAM++ ONNX exists per its conversion check, I did not locate the file); Who LiteRT 32 MB but is diarisation, not enrol-and-verify. Benchmark first: EER of WeSpeaker/CAM++ on our own radio-bleed vs owner voice recordings through the phone microphone; none of these cards publish EER. Disqualify any whose embedding cannot separate the owner from the radio's own TTS voice at one threshold.

### 2.4 Small on-device LMs (under about 2B)
List: https://huggingface.co/models?pipeline_tag=text-generation&num_parameters=min:0,max:2B&sort=trending (LFM3-1.7B tops; Qwen3-0.6B 15.2M downloads; Llama-3.2-1B; Gemma-3-1b-it; microsoft BitNet b1.58 2B; many unknown small ones).

| Model | Size | Format | Licence | Source |
|---|---|---|---|---|
| Qwen3-0.6B (in use) | 0.6B (0.44B non-embedding); Q4_K_M 397 MB, Q8_0 639 MB | GGUF and many | Apache-2.0; 32,768 context | https://huggingface.co/Qwen/Qwen3-0.6B ; sizes ON-DEVICE-MODELS 8.3 |
| LiquidAI LFM2.5-350M | 350M; Q4_K_M 229 MB (ON-DEVICE-MODELS 8.3) | GGUF 233k dl, ONNX, MLX, LiteRT (litert-community/LFM2.5-350M, 1.08k dl, updated 3 days ago) | LFM Open License | https://huggingface.co/models?search=LFM2.5-350M&sort=downloads |
| LiquidAI LFM3-1.7B | about 1.7B; Q4_0 1.05 GB, Q4_K_M 1.1 GB, Q8_0 1.85 GB, NVFP4 1.28 GB | GGUF (llama.cpp b11960+) | lfm1.0 (commercial use at or above US$10M annual revenue needs a licence, ON-DEVICE-MODELS 8.1) | https://huggingface.co/LiquidAI/LFM3-1.7B-GGUF ; 445,908 dl; no speed or memory on page |

Others on the same list: Gemma 3 270M (253 MB Q4_K_M) and Gemma 3 1B (806 MB) per ON-DEVICE-MODELS 8.3. Phone speeds, third-party, from the same doc: iPhone 17 Pro Qwen3-0.6B 121 to 193 tok/s depending on runtime; LFM3-1.7B Q4_0 43.6 tok/s at 1.90 GB (vendor chart relayed); nothing newer than 2023 for an A16.
Rank (a): Qwen3-0.6B Q4_K_M or Q8_0 (already fits and runs), LFM3-1.7B QAD Q4_0 as the quality step if memory allows, LFM2.5-350M as the cheap fallback. Rank (b): LFM2.5-350M Q4_K_M (229 MB) first, Qwen3-0.6B Q4_K_M (397 MB) second, LFM3-1.7B (1.05 GB resident near 1.9 GB measured) disqualified on this phone. Benchmark first: decision accuracy against Jev on our fixed question set for 350M vs 0.6B; if the 350M agrees, it wins on low-end.

## 3. Recogniser (recap, since two of the four items are recognisers)
Current: Parakeet TDT 0.6B v2 about 480 MB, Parakeet EOU 120M about 250 MB (streaming), Apple recogniser (free). Candidates: Voz 337 MB (Core ML and LiteRT, 25 languages, source-available licence); Fluid Speech Pro Core ML 652 MB (Mac-tested only). Neither streams on Android per the pages. The cheapest Android recogniser was not surveyed in depth (only a mention): this survey did not fetch Whisper-tiny, Moonshine or sherpa-onnx streaming zipformer pages. That is the largest gap.

## 4. Memory budget on the low-end Android (3.4 GiB total RAM)
Operating system, other apps and the Android runtime commonly leave an app only about 1 to 1.5 GiB before it is killed (general knowledge, not from a fetched source; verify on the device with `dumpsys meminfo`). The stack must therefore sit well under 2 GiB, and honestly under about 1 GiB resident. Arithmetic from file sizes above (resident usually exceeds file size; the repo's own planner multiplies by 1.05 plus context, ModelCandidates.swift line 71):
- Recogniser: Voz 337 MB (page gives no phone memory) or Parakeet EOU 250 MB; Parakeet TDT v2 480 MB.
- Voice: kale-tts 15 MB, tiny-voca 9-21 MB, Phonon 55-64 MB, Kokoro about 350 MB.
- VAD plus speaker: about 2 MB + 8-14 MB.
- LM: LFM2.5-350M 229 MB, Qwen3-0.6B 397 MB, plus KV cache.
Cheapest viable stack: about 337 (Voz) + 15 + 12 + 229 = about 600 MB of file size; with runtime overhead maybe 1 GB. That fits only if models are loaded and unloaded around use, not all resident. The 1.7B LM, Kokoro and a 0.6B recogniser together (about 2.5 GB) do not fit. Not verified on any Android device by anyone in the sources: every Android number above is either a different phone (J6+, A3s, Tab S10 Lite) or none.

## 5. What to benchmark first
1. On Snapdragon 6 Gen 1: kale-tts-1-nano and tiny-voca RTF and listening test on real news text with the recogniser running; Phonon 31M GGUF second.
2. On both phones: Silero vs TEN VAD false positives with the radio's own TTS audible.
3. Speaker verification EER with the owner, the radio voice and a third speaker through the phone mic (WeSpeaker vs CAM++), since no card gives one.
4. LM: LFM2.5-350M vs Qwen3-0.6B agreement with Jev on the decision set; peak resident memory with everything loaded (`os_proc_available_memory()` on iOS, meminfo on Android).
5. Recogniser on Android: Voz LiteRT CPU speed and memory (the page says its speed does not carry over to phone CPUs).

## 6. Could not verify
- Any Android or Snapdragon number for Voz, Who, Fluid Speech Pro, Phonon, Kokoro, LFM models.
- Who's weights licence and languages; Voz licence text (license.desertant.com not opened); TEN VAD "additional conditions"; Supertonic 3 OpenRAIL-M restrictions.
- tiny-voca: release date, memory, CPU model behind "70x"; whether the dizione phonemizer has a mobile port; Qwen3-TTS output terms for the synthetic training data.
- Fluid Speech Pro iOS minimum and iPhone speed; fluid-speech-mini was not opened.
- Download/likes counts on HF list pages are unlabeled and were inferred by the fetcher.
- Speaker-verification EER for every candidate (cards report none).
- Whisper-class small recognisers and Moonshine/sherpa-onnx streaming models were not surveyed.
- Whether models listed on the vendor pages are the versions the app's pinned FluidAudio 0.18.3 can load (Fluid Speech Pro's Core ML build loads through a fork).
