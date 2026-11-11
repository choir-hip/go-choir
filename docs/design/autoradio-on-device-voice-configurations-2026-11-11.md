# Autoradio on-device voice: pipeline versus voice model, and phone configurations

Research note, 2026-11-11. Owner question (2026-11-11): which is faster on the
phone, a pipeline (local text model into local speech synthesis) or a
voice model; the answer may depend on the phone; setup must cope with many
configurations; are there quantisations that are easier to run; is there a
voice-to-voice model that fits the 14 Pro Max. Reference data lives in
[ON-DEVICE-MODELS.md](../reference/ON-DEVICE-MODELS.md); this note is the
analysis and the proposal. Rulings 23, 24, 27, 28 of
[the app UI note](autoradio-app-ui-2026-11-10.md) are the requirement: at the
end of the listener's speech the phone starts saying something relevant
within a few hundred ms, keeps talking sensibly for seconds, is replaced
when the desk's speech is ready, and no timer sits around a model call.

Mutation class: green (docs only). Rollback: revert.

## Evidence classes

Every claim below carries one of these. All web facts were fetched on
2026-11-11 from the primary page named (model cards and file listings from
the Hugging Face API, vendor pages, repos).

- **[ours]** measured by us; the repo file is cited.
- **[vendor]** the maker's own claim.
- **[3P]** measured by a third party.
- **[est]** my arithmetic or scaling from marked inputs; a hypothesis for
  setup to test, not a fact.
- **[unk]** no source found.

## 1. The short answer

1. On this phone the pipeline is the only route with a credible path to the
   latency target. A pipeline of Parakeet EOU, a 0.6B to 1.7B text model and
   a streaming synthesiser has parts whose phone cost is measured or
   bounded; the one-model route has no iPhone measurement anywhere and our
   own Mac run of it was slower than real time. [ours][est]
2. LFM3-Audio-2B as speech-in speech-out is not proven on any iPhone: the
   vendor, FluidAudio and llama.cpp publish no iPhone figure, and the only
   speech-out routes are Core ML (int8, about 2.9 GB on disk, text-to-speech
   only, no documented speech-to-speech) and llama.cpp (server/CLI
   documented, iOS not documented). [vendor][unk]
3. Its quantised builds exist and are smaller (GGUF Q4_K_M 1.1 GB plus a
   0.17 to 0.30 GB projector; MLX 4-bit 1.59 GB; LiteRT int4 1.53 GB) but
   the only iOS-capable one with documented speech out is the Core ML int8
   pack; its authors tried int4 there and it failed their speech-frame
   check. [vendor]
4. No open voice-to-voice model under about 2B parameters has a documented
   iPhone run apart from the previous Liquid model (LFM2.5-Audio-1.5B, MLX
   Swift, 0.9 to 1.7 GB by quantisation, no phone figure). Moshi is 4.8 GB
   at 4-bit; the new open full-duplex NVIDIA model is 11B. Nothing else fits
   6 GB with room for the app. [vendor][3P]
5. The decisive cost in the pipeline is not the model, it is prefill of the
   page text: on an A16 about 500 prompt tokens per second for a 1.1B model
   means 2 s for a 1,000-token page, unless the page is already in the KV
   cache. Warm the cache while the page plays. [3P][est]
6. Whether speech-to-speech can win on A19 Pro class phones is open; on
   A16/6 GB it should not be attempted before the experiments in section 8.

## 2. Pipeline versus one voice model

### 2.1 Budget of each stage

Target: first audible, relevant sentence within a few hundred ms of the end
of speech (ruling 23), then a front-loaded stream (ruling 27).

| Stage | A16 / 6 GB | A17 Pro, A18 (8 GB) | A19 Pro (12 GB) | Evidence |
|---|---|---|---|---|
| Speech bounds (Silero hop 256 ms) | 0.6 ms per hop, 106 MB, load 93 ms | not run | not run | [ours] mobile/README.md model table (phone row) |
| Streaming ASR, Parakeet EOU 120M, chunk 320 ms | RTF 0.035 (about 11 ms of compute per 320 ms chunk), 149 MB, first load 34 s | not run | not run | [ours] mobile/README.md |
| Streaming ASR final text on Mac | first token 168 ms | | | [ours] same |
| Text model prefill | about 505 tok/s, TinyLlama 1.1B Q4_0, iPhone 14 Pro, Dec 2023 llama.cpp | about 646 tok/s, same model, iPhone 15 Pro | Qwen3-0.6B Core AI prefill 5,807 tok/s on iPhone 17 Pro (different runtime) | [3P] llama.cpp discussion 4508; john-rocky/apple-silicon-llm-bench |
| Text model decode | 54 tok/s, TinyLlama 1.1B Q4_0, A16 | 57 tok/s, same, A17 Pro | Qwen3-0.6B Q4 MLX 159 to 167 tok/s; LFM3-1.7B Q4_0 43.6 tok/s at 1.90 GB; Gemma 4 E2B 39 (llama.cpp) to 61 (LiteRT-LM) | [3P] bench repo; [vendor] LFM3-1.7B card (the figure is in a chart image; I read it from a search summary, not the image) |
| First sentence of about 15 tokens | 15 / 54 = 0.28 s | similar | about 0.1 s | [est] |
| Streaming synthesis, first audio | Kokoro: 253 ms Mac, "physical iOS inference remains unvalidated" by FluidAudio, does not stream. PocketTTS: first frame 120 to 239 ms Mac. Phonon 31M: 28 ms p95 first audio on CPU (Arm's measurement, not an iPhone); about 20x real time on the Neural Engine. No A16 figure for any | no figure | no figure | [ours] mobile/README.md (Kokoro, PocketTTS Mac); [vendor] FluidAudio README, Gradium blog |
| System voice (AVSpeechSynthesizer) | first audio was "the largest variable" at up to 4.4 s in the first staging mixed-turn run | | | [ours] mobile/README.md line 132; not a clean figure (the line says the split is partly inferred) |

Sum for the pipeline on an A16 with the page already cached: end-of-speech
wait of one ASR chunk (320 ms, plus whatever the end-of-utterance rule adds)
+ prefill of the listener's words (about 30 tokens, 0.1 s) + 0.28 s decode +
synthesis first audio (0.03 to 0.3 s) = roughly 0.7 to 1.0 s. [est] Without a
cached page it is 2 to 3 s more. The ASR chunk and the sentence decode are
the two pieces that can overlap with speech: start prefill of the partial
transcript while the listener is still speaking and, if the end-of-turn
detector fires, only the last words remain to prefill. [est]

### 2.2 One voice model

LFM3-Audio-2B (released 2026-10-27, files 2026-11-03/05) takes text or
16 kHz audio and returns text, 24 kHz speech or both; voice chat is English
only; a 32K context; "time to first token as low as 50 ms with streaming
audio input" (hardware not stated). [vendor] HF model card, LiquidAI/LFM3-Audio-2B.

What the numbers say for a phone:

- Vendor, M5 Pro Mac, Core ML int8 pack: text-to-speech first audio 37 ms
  median, 4.3 to 4.5x real time; live ASR 10.4x; per 80 ms speech frame
  9.4 ms backbone plus 4.9 ms depth decoder; peak footprint 1.9 GB for ASR.
  [vendor] FluidInference/lfm3-audio-2b-coreml.
- Ours, a loaded M1 Mac: live ASR RTF 4.97 (17.6 s to transcribe 3.6 s,
  slower than real time), text-to-speech first audio 2.4 s, RTF 1.65 (slower
  than real time). [ours] mobile/README.md model table. That is about 5x
  slower than the vendor's M5 Pro on ASR and about 7x on speech out.
- An A16 GPU is weaker than an M1. [est] On our own numbers LFM3-Audio speech
  output on the 14 Pro Max via Core ML would be slower than real time and
  could not stream continuously. This is the hypothesis experiment 2
  (section 8) tests directly.
- The faster Core AI build (iOS 27; a 14 Pro Max runs iOS 27, because Apple
  says iOS 27 supports the same iPhones as iOS 26 and that list includes the
  14 Pro Max [3P, 9to5mac]) is 15 to 20% faster than Core ML on the M5 Pro
  (RTFx 5.07 versus 4.3 to 4.5 TTS; ASR 24.6 versus 18.4). [vendor]
  FluidInference/lfm3-audio-2b-coreai. That does not close a 7x gap.
- No speech-to-speech chat is documented by FluidAudio; ours would be a
  composition of its live ASR, a text step and its synthesis, which is the
  pipeline again with a heavier ASR and TTS than Parakeet and Kokoro. A
  single-model reply with speech out needs the llama.cpp realtime path, which
  is documented for macOS/Linux servers (`llama serve`, `/v1/realtime`) and
  not for iOS. [vendor] HF GGUF card, Liquid docs.

### 2.3 Where each wins

| | Pipeline | One voice model |
|---|---|---|
| Time to first audio, A16 | estimated 0.7 to 1.0 s with a warm cache | unknown; our Mac run implies seconds |
| Quality of what is said | set by the text model chosen, can be changed; page-grounded prompts easy; the vendor warns small models need retrieval for facts | one 2B model, "not recommended for complex multi-step reasoning"; tool calling supported |
| Memory | sum of parts, each measurable (about 0.15 + 0.4 to 1.1 + 0.1 to 0.7 GB) | one model, 1.4 to 2.9 GB, plus its codec |
| Interruption | ours (stop the synthesiser, drop the stream); model-independent | not documented for LFM3-Audio; ours again |
| Prosody and overlap | each sentence separate | the single model can in principle carry tone from the listener's voice; not demonstrated |
| Licence | per part: Apache-2.0 / MIT / CC-BY 4.0 for the likely parts | LFM Open License (section 4) |

The pipeline wins on the owner's phone and for the design (a front-loaded
stream of stand-alone sentences is exactly what a sentence splitter over
streamed tokens produces). A voice model would win only if its first audio
beat about 0.5 s on the phone and its sentences were better than a 1B text
model's, neither of which is shown. [est]

## 3. Local responder text models

Sizes are Hugging Face file listings fetched 2026-11-11 (GGUF unless said).
Speed is decode tokens per second; "17P" is iPhone 17 Pro (A19 Pro). Nothing
below was measured on an A16 by us. Rows without a phone figure say so.

| Model | Quant (file size) | iPhone figure | Thinking | Licence tag | Note |
|---|---|---|---|---|---|
| Qwen3-0.6B | Q4_K_M 397 MB; Q5_K_M 444; Q8_0 639 (ours); MLX 4-bit 335 MB | 17P: MLX Q4 159 to 167, LiteRT-LM 121, Core AI 193 [3P]; ours Mac Q8 1,050 MB resident, 65 to 71 ms per decision [ours] | hybrid; `enable_thinking=False` is a hard off switch [vendor card] | apache-2.0 | 0.44B non-embedding; IFEval 68.2 for the 1.7B sibling in Qwen's own table [vendor] |
| Qwen3-1.7B | Q4_K_M 1,107 MB; Q8_0 1,834; MLX 4-bit 968 MB | none found | same | apache-2.0 | |
| Qwen3.5-0.8B | Q4_K_M 533 MB; Q5_K_M 590; Q8_0 812 | none found | non-thinking is the default [vendor card] | apache-2.0 | IFEval 52.1 non-thinking [vendor] |
| Qwen3.5-2B | Q4_K_M 1,281 MB; Q8_0 2,012 | older iPhone, Debug build: llama.cpp 39.1, MLX 61.2 tok/s, 1.3 to 1.5 GB [3P] | non-thinking default per family | apache-2.0 | IFEval 61.2 non-thinking [vendor] |
| LFM3-1.7B | Q4_K_M 1,100 MB; QAD-Q4_0 1,050; Q5_K_M 1,259; Q8_0 1,846; MLX 4-bit | 17P Q4_0: 43.6 tok/s, 1.90 GB peak [vendor, via search summary] | reasoning on by default; `enable_thinking=False` "can be inconsistent" [vendor card] | LFM Open License | 256K context, KV cache 1.6 GB at 128K; IFEval 96.7 (reasoning on) [vendor] |
| LFM2.5-1.2B-Instruct | Q4_K_M 731 MB; Q5_K_M 843; Q8_0 1,246 | 17P 45.5 tok/s [independent, relayed in the existing page] | instruct (no thinking trace) | LFM Open License | |
| LFM2.5-350M | Q4_K_M 229 MB; Q8_0 379 | none found (Mac Core ML 58.9 tok/s, TTFT 383 ms [3P bench repo]) | | LFM Open License | smallest credible responder; quality on grounded speech unknown |
| Gemma 3 1B it | Q4_K_M 806 MB; Q8_0 1,069; MLX 4-bit 733 | none found | none | gemma terms | |
| Gemma 3 270M it | Q4_K_M 253 MB | none found | none | gemma terms | |
| Gemma 3n E2B | Q4_K_M 3,027 MB | none found | | gemma terms | does not fit beside anything on 6 GB |
| Gemma 4 E2B | Q4_K_M 3,107 MB; Q8_0 5,048 | 17P: LiteRT-LM 61.1, Core AI 47, MLX 49, llama.cpp Q4_K_M 38.8 tok/s; 0.5 to 3 GB by runtime [3P] | `enable_thinking`, off by removing a system token [vendor card] | apache-2.0 | 2.3B effective, 5.1B with embeddings; 128K context; audio input |
| SmolLM3-3B | Q4_K_M 1,915 MB | none found | on by default; `/no_think` in the system prompt [vendor card] | apache-2.0 | too large for the 6 GB tier next to anything |
| Phi-4-mini-instruct | Q4_K_M 2,492 MB | none found | none | mit | 3.8B; same objection |
| Llama 3.2 1B Instruct | Q4_K_M 808 MB; Q8_0 1,321; MLX 4-bit 695 | none found; 52 tok/s on a 16 Pro is a blog claim, not used | none | llama3.2 community licence | |
| Llama 3.2 3B Instruct | Q4_K_M 2,019 MB | none found | none | llama3.2 | too large |
| MiniCPM5-2B | Q4_K_M 1,561 MB | none found | deep-thinking trained (RL on reasoning) | apache-2.0 | 2.5B; released 2026-11; not evaluated |
| Apple Foundation Models (FoundationModels framework) | no download | 17 Pro 70 to 86 tok/s, answered 60% of one benchmark [independent, relayed] | | system | Apple's list of Apple Intelligence iPhones is "iPhone 16 models or later, iPhone 15 Pro, iPhone 15 Pro Max, iPhone Air" (and a model named iPhone Duo); the 14 Pro Max is not on it. Unavailable there. [vendor, support.apple.com/en-us/121115] The framework's own page did not render for the fetcher, so its context size stays unconfirmed. |

Quantisation guidance. Decode speed on a phone is bound by reading the
weights, so 4-bit files roughly double decode speed over Q8 at a small
quality cost for 1B-class models. [est, supported by TinyLlama on A16: Q4_0
54.2, Q8_0 34.3, F16 20.1 tok/s [3P]] Ordering by size for our two measured
anchors: Qwen3-0.6B Q4_K_M 397 MB is 38% smaller than the Q8_0 639 MB we
run today. The first change worth testing is Q4_K_M, not a different model.

Quality for short grounded speech: no published benchmark measures it. The
vendor instruction-following scores above are not that. Judge by replay
(experiment 1).

## 4. LFM3-Audio-2B in detail

Files (Hugging Face API, 2026-11-11):

| Build | Size | Runtime | Speech out | iOS status |
|---|---|---|---|---|
| BF16 safetensors | 4,005 MB + codec 147 MB | Transformers, vLLM, SGLang | yes | no |
| GGUF Q4_0 / Q4_K_M / Q8_0 / F16 | 1,050 / 1,100 / 1,846 / 3,467 MB | llama.cpp v0.8.0 (b11960) | yes: `llama-tts`, `POST /tts`, `/v1/realtime` | the xcframework carries libmtmd (PR merged 2026-06-25) [vendor]; no iOS statement for LFM3-Audio; unconfirmed |
| mmproj (audio encoder and speech head) | Q4_0 161, Q4_K_M 172, Q8_0 304, BF16 571 MB | llama.cpp | | |
| MLX 4-bit / 8-bit / bf16 | 1,585 / 2,335 MB / not listed, + codec 147 MB | MLX-VLM (Python, Apple Silicon) | yes | no MLX Swift port found for LFM3; a Swift SDK supports LFM2.5-Audio only |
| Core ML pack (FluidAudio) | ASR about 2.5 GB, speech out adds about 0.4 GB; LM int8 block-32; context 4,096 | FluidAudio `LFM3AudioManager`, iOS 18+ | text-to-speech and live ASR; no speech-to-speech chat | the only documented Swift route; no iPhone figure; int4 build failed the authors' speech-frame check |
| Core AI pack | LM 2,159 MB + depth decoder 218 MB | iOS 27 | same | same |
| LiteRT / litertlm | int4 1,531 MB + tflite 0.7 GB; Kavic int4 1,146 / int8 2,141 MB | LiteRT-LM (Android/cross-platform) | | not iOS-documented |
| ONNX | q4 decoder 1,687 MB + encoder 255 to 510 + codec 90 to 179 + depth 336 | ONNX Runtime | | not iOS-documented |
| Community | nativ-community MLX 4/5/6/8-bit, mxfp4/nvfp4; prithivMLmods and others GGUF | | | untested by us |

Memory at each precision, weights only: GGUF Q4_K_M 1.1 GB + mmproj 0.17 to
0.30 GB = 1.3 to 1.4 GB; Q8_0 1.85 + 0.30 = 2.15 GB; Core ML int8 about
2.9 GB of files, 1.9 GB peak footprint for ASR on a Mac (vendor), 238 MB
resident right after load and 78 MB after unload on our Mac before any
prediction (ours; the first Core ML compile built a multi-GB cache and took
739 s including the download). The weights fit a 6 GB phone in principle.
Whether they fit beside Parakeet, the app and a text model is the experiment
2 question; no phone measurement exists. The open llama.cpp issue
"Freed model memory leaves process footprint elevated on iOS" (#30556,
2026-10-14, title only, not read in full) is a reason to measure unload on
the device. [vendor]

Smaller or distilled LFM3-Audio: none found. The family has no audio model
below 2B.

True speech-to-speech on iOS today: not documented by any route. The
nearest are (a) llama.cpp realtime on a Mac, (b) Core ML text-to-speech and
live ASR composed by us, (c) LFM2.5-Audio-1.5B through MLX Swift (the
previous model; MLX bf16 about 3 GB, 8-bit 1.7, 6-bit 1.3, 5-bit 1.1, 4-bit
0.9 GB; no phone figure) [vendor, mlx-audio-swift README].

Licence (LFM Open License v1.0, full text fetched): the Threshold is annual
revenue of 10 million US dollars or more, measured over the "Legal Entity"
including controlled, controlling and commonly controlled entities; the
grants are subject to a Commercial Use limitation in section 5, the HF card
of the Core ML pack states that commercial use by organizations at or above
the threshold needs a separate licence from Liquid AI, qualified non-profits
have an exception for non-commercial and research use, and redistribution
needs the licence text, change notices and attribution. The licence says
works that merely link to the interfaces of the Work are not Derivative
Works. We do not ship the model inside the app (models are fetched), which
does not change the Commercial Use limitation. Read section 5 of the
licence before any shipping decision. [vendor]

## 5. Other voice-to-voice models

| Model | Size | Phone evidence | Licence | Full duplex |
|---|---|---|---|---|
| Kyutai Moshi (moshiko, moshika) | 7B temporal transformer; MLX q4 4,806 MB + codec 385 MB; q8 8,169 MB | the repo says the MLX build is for iPhone and Mac; tested on an M3 MacBook Pro; no iPhone figure; 4.8 GB leaves no room on 6 GB [vendor] | CC-BY 4.0 weights, MIT/Apache code | yes; 160 ms theoretical, 200 ms practical on an L4 GPU |
| NVIDIA PersonaPlex 7B | 8.4B parameters, 16.7 GB BF16; ten quantised repos not inspected | none | NVIDIA Open Model License | yes |
| NVIDIA NemotronLabs VoiceChat 11B | 11.1B; MLX 4-bit about 9.2 GB | none | OpenMDW 1.1 | yes; about 450 ms turn-taking [vendor] |
| LFM3-Audio-2B, LFM3-Omni-2B | 2.0 / 2.2B | none | LFM Open License | not stated |
| LFM2.5-Audio-1.5B | 1.5B | MLX Swift library; no figure | LFM Open License | no |
| Sesame CSM-1B | 1.55B, 6.2 GB BF16 checkpoint | none; speech generator only, needs a text model in front | apache-2.0 | no |
| Ultravox v0.5 Llama 3.2 1B | 1.37 GB, audio in, text out (needs a TTS) | none | mit | no |
| Qwen2.5-Omni-3B | 5.5B parameters, 12 GB BF16 | none | other | no |
| Qwen3-Omni-30B-A3B | 30B | none | | |
| Mini-Omni 2 | GGUF exists in a community repo | not inspected | | |

Nothing here fits a 6 GB phone beside the app except LFM3-Audio-2B and
LFM2.5-Audio-1.5B, and neither has a phone measurement. The 2026 additions
found (NVIDIA's two) are 8B to 11B. I found no Moshi-Lite or other distilled
Moshi on Hugging Face or in Kyutai's model list (a blog's roadmap claim of a
1B variant is unconfirmed). Kyutai's latest releases are text-to-speech
(Pocket TTS, 2026-10-21).

## 6. Streaming TTS options on iOS

| Option | Size | First audio / speed | Streams | iOS route | Licence |
|---|---|---|---|---|---|
| Phonon 31M (Gradium, 2026-10-20) | 65 MB files | 28 ms p95 first audio on CPU in Gradium's benchmark; Arm measured about 28 ms and 15x on Arm CPUs; about 20x on the Neural Engine per FluidAudio; no iPhone figure; Pocket TTS 165 ms on the same benchmark [vendor] | yes | FluidAudio, iOS 18+ | Apache-2.0 (English 31M); the 40M multilingual is non-commercial |
| PocketTTS 100M (Kyutai) | | about 200 ms first chunk; 6x real time on an M4 Air CPU [vendor]; ours Mac first frame 120 to 239 ms, RTF 0.5, 1,027 MB resident [ours] | yes, 80 ms frames | FluidAudio (ours); community ports | weights CC-BY 4.0, code MIT |
| Kokoro 82M | 327 MB | ours Mac first audio 253 ms for the first sentence, RTF 0.07, 722 MB resident [ours]; vendor Mac 36 to 54 ms inference per 3 s [vendor]; no iPhone figure; FluidAudio says it does not stream | by sentence | FluidAudio (ours) | apache-2.0 |
| Supertonic 3 | about 99M | RTF 0.3 on an e-reader [vendor]; first audio not stated | not stated | iOS example in repo; the repo is archived | code MIT, model OpenRAIL-M |
| NeuTTS Air | 0.7B; Q4 GGUF 527 MB | "real-time on mid-range devices", no figure | not mentioned | llama.cpp | apache-2.0; needs a reference clip |
| Piper | | | | repository archived 2025-10-06, development moved to a GPL-named repo | MIT here |
| AVSpeechSynthesizer | none | our staging run: up to 4.4 s first audio in a mixed turn, partly inferred | system | native | n/a |
| LFM3-Audio speech out | see section 4 | M1 first audio 2.4 s [ours] | yes | Core ML | LFM Open |

Given the pipeline budget, TTS is a 30 to 300 ms term if the synthesiser
streams in small frames (Phonon, PocketTTS) and the A16 can match the Mac
within about 2x; Kokoro's non-streaming design makes its first audio the time
to synthesise a whole sentence. [est]

## 7. Device-tier configuration table

Principle: **setup measures and picks; this table is the starting guess.**
Memory limits: Apple does not document them. Third-party forum measurements:
about 6,144 MB on an 8 GB iPhone 16 Pro Max and a 12 GB iPhone 17 Pro Max with
the increased-memory-limit entitlement (about 6,656 MB with the debugging
entitlement); 2.2 to 2.3 GB available on a 4 GB iPhone 13 (iOS 15.3.1) [3P,
Apple developer forums threads 834805, 702400, 770868]. For a 6 GB iPhone,
and for none of these devices without the entitlement: **[unk]**. Apple's
documentation says `os_proc_available_memory()` reports the limit minus the
current footprint and that the limit can change. The budgets below are
therefore assumptions that setup replaces with a read of that call. [est]

Starting budgets (resident, all stages, assumption): 4 GB tier 1.2 GB; 6 GB
tier 2.0 GB; 8 GB tier 3.5 GB; 12 GB tier 4.5 GB.

| | 6 GB: A15 Pro, A16 (13 Pro, 14 Pro; owner's phone) | 8 GB: A17 Pro, A18 (15 Pro, 16) | 12 GB: A19 Pro class (17 Pro, Air) | Unknown / 4 GB |
|---|---|---|---|---|
| Speech bounds | Silero VAD (1 MB) | same | same | Silero if it loads, else energy detector |
| Transcription | Parakeet EOU 120M (219 MB), TDT only after the voice is loaded and measured | EOU, TDT 0.6B (480 MB) as end-of-turn refinement | TDT 0.6B + EOU | Apple's recogniser (no download), EOU if memory reads at least 3 GB |
| Local responder | Qwen3-0.6B Q4_K_M (397 MB) or Q8_0 (639 MB, ours); trial Qwen3.5-0.8B Q4_K_M (533 MB), LFM2.5-350M Q4_K_M (229 MB); LFM3-1.7B QAD-Q4_0 (1,050 MB) only if measured decode is at least 25 tok/s | LFM3-1.7B Q4_K_M or Qwen3-1.7B Q4_K_M; Apple Foundation Models where available (iPhone 15 Pro and newer) | LFM3-1.7B Q5/Q8, Gemma 4 E2B Q4 (3.1 GB) as a quality trial; Apple FM | none (the desk answers); stock holding line |
| Voice | Phonon 31M (65 MB) if its measured first audio is under 300 ms, else PocketTTS (streams), else Kokoro | PocketTTS or Kokoro (quality), Phonon fallback | Kokoro or PocketTTS | system voice |
| Quick decisions | the same local model as the responder (a one-token scorer, ours 65 to 71 ms on a Mac), else Jev through the listener's computer | same | same, or Apple FM | Jev through the computer |
| Voice model route (experiment only) | not loaded in the same process as the responder; test alone | LFM3-Audio Core ML int8 as an alternative voice and transcriber, alone | LFM3-Audio on Core AI | never |
| What loads together | EOU 0.15 + responder 0.4 to 1.0 + voice 0.07 to 0.7 = 0.6 to 1.9 GB | the same plus TDT and a 1.1 GB responder: 2.0 to 3.2 GB | up to 4 GB | 0.3 GB |
| Fallback order | smaller quant, then smaller model, then floor (system voice, Apple recogniser, Jev through the computer) | same | same | floors |

What setup measures on the device, per candidate, before it is chosen (it
extends the existing `SetupCheck` of `VoiceSetup.swift`, which holds load
time, speed, memory and output correctness; the rows marked + are new):

- load time (cold and warm) and download stall, as today;
- decode tokens per second and prefill tokens per second at the real page
  length, with the page cached and uncached;+
- time to first sentence (end of listener speech replaced by a recorded
  clip's last word, to first audio out), measured end to end;+
- first-audio ms and real-time factor for the voice, with 60 s of continuous
  speech to catch a stall;+
- resident footprint after load and at peak, and `os_proc_available_memory()`
  at idle before anything loads and after each load, so the budget is the
  real one;+
- `ProcessInfo.thermalState` before, after and a 10 minute soak; a candidate
  whose sustained speed falls below the stream it must feed is demoted
  (third-party data: Gemma 4 E2B on an iPhone 17 Pro fell from 56 to 27 tok/s
  in a 600 s run) [3P];+
- the model that is chosen has to keep at least 1 GB of headroom reported by
  `os_proc_available_memory()` while running;+
- unload actually returns memory (llama.cpp issue #30556).+

Current `SetupCatalog` (origin/lane/ar3-voice) already has the shape: four
stages, a ladder per stage, `minMemoryGB` per candidate, the floor last. The
changes this note proposes are a list, not code: (1) add a fifth stage,
`responder`, apart from `decision`, because the responder (talks for
seconds) and the decision scorer (one token) have different needs; (2) put
Q4_K_M ahead of Q8_0 for the responder; (3) replace `minMemoryGB` (a device's
physical RAM) with the measured budget; (4) add Phonon 31M as the first voice
candidate on the 6 GB tier once its first audio is measured on the phone.

## 8. Recommendation and experiments

For the owner's phone now (iPhone 14 Pro Max, A16, 6 GB):

1. Build the holding conversation as a pipeline: Silero, Parakeet EOU,
   Qwen3-0.6B in non-thinking form (Q4_K_M first), a sentence splitter, and a
   streaming synthesiser (Phonon 31M if it measures well, else PocketTTS).
   Keep the page's text in a prefilled KV cache so only the listener's words
   are prefilled at the end of speech.
2. Do not load LFM3-Audio into the same process as the pipeline. Run it
   alone as a measured experiment because the owner asked for it
   (2026-11-10); do not make it the live path until it measures.
3. Do not spend effort on a smaller LFM3-Audio quant: the int4 Core ML
   attempt failed its authors' check, and the GGUF and MLX quants have no
   iOS speech-out route yet.

Experiments, fastest first:

1. **Phone: responder and voice on the A16.** The repo's own proof
   (commands in `mobile/README.md`, section "To measure on the phone"):
   `-ModelProofStages kokoro,pocket-tts,llama-qwen3-0.6b` through
   `devicectl ... process launch`. Add to it: Qwen3-0.6B Q4_K_M, Qwen3.5-0.8B
   Q4_K_M, LFM3-1.7B QAD-Q4_0 with a llama-bench style pp512/tg128 run
   (the llama.swiftui Bench button, Release build run from the home screen,
   as in llama.cpp discussion 4508), and Phonon 31M through FluidAudio.
   Needs the owner's consent (the repo rule is that builders do not install
   on the owner's phone). Settles tok/s, prefill, first-audio ms, resident
   MB, and thermal state on the A16. About an hour.
2. **Phone: does LFM3-Audio fit and keep up?**
   `-ModelProofStages lfm3-audio-live,lfm3-audio-tts` (already written;
   2.9 GB download, 30 GB free, first compile builds a multi-GB cache).
   Record whether jetsam kills the app, the TTS real-time factor, and
   `os_proc_available_memory()` after load. If the RTF is above 1 the voice
   model is closed for this phone. About 30 minutes plus the download.
3. **Mac: ceiling for the whole pipeline versus the voice model**, no phone
   needed: run `fluidaudiocli lfm3-audio benchmark` and `llama serve -hf
   LiquidAI/LFM3-Audio-2B-GGUF:Q4_K_M --port 8080` with a recorded question
   against the realtime endpoint, and time the same question through Parakeet
   EOU, Qwen3-0.6B and PocketTTS in the replay tool (`autoradio-replay --turn
   mobile/fixtures/turns/plain ...`). Gives the two end-to-end numbers on one
   machine; the phone ratio from experiment 1 and 2 scales them.

## 9. What I could not find

- Any iPhone measurement of LFM3-Audio (any runtime), Phonon, Kokoro or
  PocketTTS first audio, or Qwen3 and LFM3 decode on an A16 or A17 Pro.
  The only A16 text-model data is a Dec 2023 TinyLlama and Phi-2 run.
- Apple's documented or measured foreground memory limit for 6 GB iPhones
  (and any limit without the entitlement).
- The Foundation Models framework's own documentation text (the page did not
  render), so its context size on current OS versions is unconfirmed.
- The numeric table behind Liquid's "1.8 to 2.8 times faster" claim: the card
  shows it as an image; the 43.6 tok/s figure came through a search summary.
- Any quality benchmark for short page-grounded spoken replies.
- Whether llama.cpp's iOS framework can drive LFM3-Audio speech output.
- A Moshi derivative below 7B.
- A16 memory bandwidth in GB/s (Wikipedia gives only "50% above A15"; A19
  Pro is 76.8 GB/s per Wikipedia).

## Sources (fetched 2026-11-11)

Hugging Face API file listings and cards: LiquidAI/LFM3-Audio-2B, -GGUF,
-MLX-4bit, -MLX-8bit, -ONNX, LFM3-Omni-2B, LFM3-1.7B(-GGUF),
LFM2.5-1.2B-Instruct-GGUF, LFM2.5-350M-GGUF, LFM2.5-Audio-1.5B-GGUF;
FluidInference/lfm3-audio-2b-coreml and -coreai; litert-community and Kavic
LFM3-Audio builds; Qwen/Qwen3-0.6B, Qwen3.5-0.8B and unsloth GGUF listings;
google/gemma-4-E2B-it; HuggingFaceTB/SmolLM3-3B; openbmb/MiniCPM5-2B;
kyutai/moshiko-mlx-q4, pocket-tts; nvidia/personaplex-7b-v1,
NVIDIA-NemotronLabs-VoiceChat-11B; neuphonic/neutts-air; sesame/csm-1b;
fixie-ai ultravox. Repos: FluidInference/FluidAudio, kyutai-labs/moshi,
kyutai-labs/pocket-tts, supertone-inc/supertonic, rhasspy/piper,
ggml-org/llama.cpp (release v0.8.0, `build-xcframework.sh`, discussion 4508),
Blaizzy/mlx-audio-swift, john-rocky/apple-silicon-llm-bench. Vendor pages:
docs.liquid.ai (llama.cpp, LFM3-Audio-2B), liquid.ai/blog/lfm3,
gradium.ai/blog/phonon-closer-look, support.apple.com/en-us/121115. Others:
9to5mac (iOS 27 compatibility), Wikipedia (A16, A17, A18, A19), Apple
developer forums threads 834805, 702400, 770868, 826325.
