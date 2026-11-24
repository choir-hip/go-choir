# On-device models (Autoradio phone app)

Reference page (convention: [README.md](README.md)). Compiled 2026-11-10
from three design notes: [turns](../design/autoradio-turns-2026-11-10.md),
[lab](../design/autoradio-lab-2026-11-10.md),
[voice modes](../design/autoradio-voice-modes-2026-11-10.md). Sections 1 to
7 were written without a web fetch, so their figures are **relayed-unverified**
unless marked otherwise. **Section 8 (2026-11-11) was fetched from primary
sources on 2026-11-11 and supersedes sections 1 to 7 where they differ**;
the analysis is in
[autoradio-on-device-voice-configurations-2026-11-11](../design/autoradio-on-device-voice-configurations-2026-11-11.md).
Phone measurements by us as of 2026-11-11: Silero VAD and Parakeet EOU on
the A16 only (`mobile/README.md`); everything else is untested on any phone.
A cell that says "not recorded" means the notes do not give the fact; do not
fill it from memory, fetch and cite it.

Qualifiers used: **vendor** (the maker's own figure), **independent** or
**3P** (third-party benchmark), **unmeasured** or **unk** (no figure exists),
**ours** (we measured it, file cited), **est** (our arithmetic from marked
inputs; a hypothesis, not a fact).

## 1. The floor device

The owner's iPhone 14 Pro Max (A16, 6 GB) is the oldest phone supported for
local models. "What works on this phone works on every phone we will
support." Consequences, all **to confirm on the device**:

- Apple's built-in language model needs Apple Intelligence hardware (iPhone
  15 Pro or later), so it is very likely unavailable here. It stays a stage
  implementation for newer phones.
- LFM3-Audio peaked at 1.9 GB and built a 5.3 GB first-load cache on a Mac
  (vendor/authors' figures on an M5 Pro). On 6 GB it may not fit beside a
  second model; test first.
- Core AI exports need iOS 27; whether this phone runs it is to check.
- Safe set: models of 0.5 to 1.2 GB (Qwen3-0.6B, LFM2.5-1.2B) and the small
  audio models.

## 2. Candidates by job

Release dates and sizes are as the notes give them. Status for every row:
untested by us.

| Job | Candidate | Released | Size | Licence | iOS route | Figures (device, source) |
|---|---|---|---|---|---|---|
| Addressing / word spotting | Soniqo `SpeechWakeWord` (open vocabulary) | not recorded | about 4 MB | model Apache-2.0; package licence to confirm | not recorded | Typed as text, no training. Its own figure: 0.27 false triggers per utterance on its negative set (vendor, no device) |
| Addressing | LiveKit `livekit-wakeword` | not recorded | 451 KB to 7 MB | Apache-2.0 | not recorded | Trained in about an hour on a Mac from synthetic speech (vendor, no device) |
| Addressing (not usable) | Porcupine | n/a | n/a | free tier ended June 2026 | n/a | n/a |
| Addressing (not usable) | openWakeWord | n/a | n/a | ready models non-commercial | n/a | n/a |
| Speech bounds, end of turn | Smart Turn v3.2 | not recorded | 8M parameters | not recorded | Core ML port | 3.5 ms per window on a Mac (no iPhone figure) |
| Speech bounds | Silero VAD; Parakeet end-of-utterance | not recorded | not recorded | not recorded | not recorded | unmeasured |
| Speaker check | WeSpeaker; CAM++; the owner's Mike plugin's learned-voice check | not recorded | not recorded | not recorded | Mike: in the owner's plugin | Mike's accuracy measured only on simulated audio |
| Speaker check (the reader's voice, lane autoradio-talk, 2026-11-15) | WeSpeaker ResNet34-LM as Core ML, `FluidInference/speaker-diarization-coreml` folder `wespeaker_v2.mlmodelc`, pinned commit `7eab74f3` | the repository's last change 2026-10-12 | 256-number embedding; 7.95 MB on disk (weights 7,243,904 B, 8-bit palettised); input `waveform` [3,160000] and `mask` [3,589], output `embedding` [3,256] | CC-BY-4.0 (WeSpeaker, via pyannote `wespeaker-voxceleb-resnet34-LM`); attribution required; the segmentation model in the same repository is MIT and is not used | Core ML directly (`WeSpeakerEmbedder.swift`), NOT FluidAudio: the linked FluidAudio 0.18.3 was built with the Diarizer trait off (`build-xcframeworks.sh`: `--traits TTS`; `nm libFluidAudio.a` finds no `DiarizerManager`, `EmbeddingExtractor` or `DiarizerModels`) | none on a phone. The vendor's notice records cosine >= 0.997 against the PyTorch checkpoint on AMI ES2004a; the thresholds in `SpeakerProfile` (0.35, fitted on LibriSpeech by the Mike engine) are inherited and unverified for this conversion and this phone's microphone |
| Transcription | Apple's on-device recogniser | n/a | none | n/a | system | unmeasured by us |
| Transcription | Parakeet | not recorded | not recorded | not recorded | not recorded | unmeasured |
| Transcription | LFM3-Audio live ASR (FluidAudio) | see below | see below | see below | Core ML, iOS 18+ | about 10x real time on an M5 Pro Mac (authors); no iPhone figure |
| System 1 decisions | Apple Foundation Models (iOS 26) | iOS 26 | no download | system | Foundation Models framework | about 70 to 86 tokens/s on an iPhone 17 Pro (independent, estimated); answered only 60% of that benchmark's questions; context reported 4K tokens, unconfirmed |
| System 1 | Liquid LFM2.5-1.2B | not recorded | about 0.5 to 1.2 GB class | LFM Open License | llama.cpp (see section 3) | 45.5 tokens/s and 88% on an instruction-following test, iPhone 17 Pro (independent; the only independent iPhone figures for a Liquid model) |
| System 1 | Liquid LFM3-1.7B | 2026-10-27 | about 1 GB at 4-bit | LFM Open License | llama.cpp | vendor: 1.8 to 2.8 times faster than its comparators on an iPhone 17 Pro; absolute speed unmeasured; reasoning on by default, turning it off "can be inconsistent" |
| System 1 | Liquid d1 decision models | opened 2026-10-07 | not recorded | not recorded | no iPhone build known | yes/no and multiple choice in one forward pass, no tokens; 30 ms on an M5 Pro Mac (vendor); no independent evaluation |
| System 1 | Qwen3-0.6B as a one-token scorer | not recorded | about 456 MB | not recorded | community Core AI export | prefill about 5,800 tokens/s on an iPhone 17 Pro (community export, not vendor) |
| System 1 | Gemma 4 E2B | not recorded | not recorded | not recorded | not recorded | 0.3 s to first token on an iPhone 17 Pro (Google's figure) |
| System 1 (long run) | small encoder with answer heads, trained on Jev's record | not built | tens to a few hundred MB | ours | not decided | An existing router on mmBERT-small answers seven questions in about 14 ms on four server cores (its own figure) |
| Holding conversation | any System 1 text model above, streamed into a voice | | | | | conversational quality unmeasured |
| Voice | Phonon 31M (Gradium) | 2026-10-20 (card) | 31M parameters (the card; 41.1M with the audio tokenizer); the Core ML pack the app downloads is 173 MB, the 8-bit weights 64.5 MB | Apache-2.0 | FluidAudio `PocketTtsSmallManager` (0.18.0 and later), Core ML; see section 10 | 28 ms to first audio on Arm CPUs (vendor); English only; no iPhone measurement |
| Voice | Kokoro; system voice; LFM3-Audio speech out; hosted voice | not recorded | not recorded | not recorded | not recorded | unmeasured |
| Speech-to-speech | Liquid LFM3-Audio-2B | not recorded | 1.9 GB peak, 5.3 GB first-load cache (Mac) | LFM Open License | see section 4 | vendor: first token "as low as 50 ms"; FluidAudio on M5 Pro Mac: synthesis about 4.4x real time, 37 ms to first audio; no iPhone figure |

## 3. Runtimes

What the notes record; nothing more is known here.

- **llama.cpp (`llama.xcframework`)**: Liquid's documented route now that
  its own SDK is deprecated; use a grammar to constrain output. Merged
  LFM3-Audio support 2026-10-27; its iOS framework carries the multimodal
  library, so audio in should work. Whether speech out is reachable from the
  framework's public interface is **not confirmed**. A Swift wrapper is
  estimated at one to two weeks.
- **MLX Swift**: has a port of the previous model only (LFM2.5-Audio, with
  speech-to-speech). A port to LFM3 is estimated at several weeks.
- **Core ML via FluidAudio**: the documented Swift route for LFM3-Audio
  (`LFM3AudioManager`, iOS 18 and later): offline transcription, live
  English transcription in chunks of about a second, streaming synthesis in
  two voices. It does not document speech-to-speech chat; its authors have
  not measured it on an iPhone. Core ML build limits context to 4,096
  positions.
- **LiteRT-LM, Cactus**: named as candidates only; nothing recorded about
  what they can or cannot do.
- **Core AI (iOS 27)**: community exports exist (the Qwen3-0.6B figure
  above). Whether the 14 Pro Max runs iOS 27 is unconfirmed.
- **Apple Foundation Models**: typed output, no download; Apple Intelligence
  hardware required (iPhone 15 Pro or later).
- Which runtime carries most models is open. Several will coexist in the
  lab; the shipped app should carry one or two. Models are fetched on
  demand, not bundled.

## 4. The Liquid family

- **LFM3 (LFM3-1.7B, 2026-10-27)** and **LFM2.5-1.2B**: text models, rows
  above. Reasoning on by default in LFM3.
- **d1 (opened 2026-10-07)**: decision models, no tokens generated. No
  iPhone build or figure known; the best fit for System 1 if one appears.
- **LFM3-Audio-2B**: turn-based English voice chat; takes a system prompt
  (so the passage being played can be supplied). Three iOS routes: (1)
  FluidAudio Core ML build, documented, transcription and synthesis, no
  speech-to-speech documented; (2) llama.cpp, merged 2026-10-27, audio in
  likely, speech out unconfirmed; (3) MLX Swift, previous model only. Owner,
  2026-11-10: "definitely get lfm3-audio-2b on ios." Interruption handling
  would be ours.
- **LEAP SDK** (Liquid's own phone SDK): deprecated; do not build on it.
- **LFM Open License**: commercial use allowed only under $10M of annual
  revenue (as relayed in the turns note; read the licence text before any
  shipping decision).

## 5. Rules for a phone model

From the turns note:

- It holds; it does not answer. It works only from the listener's words,
  the passage being played and the prepared material. Where it does not
  know, it says it is finding out. It must not state something the desk may
  contradict.
- What it said goes into the document as blocks, so the desk reads the
  holding conversation before it revises and the listener hears one
  continuous radio.
- Its decisions are recorded like Jev's and scored the same way.
- Jev stays the reference. The phone model earns a question only when its
  record on that question matches Jev's.
- Only text and context go to Jev; no audio leaves the phone. Replay audio
  stays on the phone by default.
- Provisional order: decisions, Apple's model first (newer phones only),
  then LFM3-1.7B or LFM2.5-1.2B under a grammar, then d1 if a build appears;
  holding conversation, a small text model streaming into an on-device
  voice; speech-to-speech is a later experiment, not a plan.
- Lab rule: narrowing deletes. A losing implementation is removed, not
  left off.

## 6. Measurement plan

On the owner's iPhone, in the voice spike (all unmeasured today):

1. Time to first token at about 2K tokens of context for Apple's model
   (where available), LFM2.5-1.2B, LFM3-1.7B, and Qwen3-0.6B as a one-token
   scorer.
2. The FluidAudio LFM3-Audio demo: memory, speed, heat. If it holds,
   compose the holding conversation from its live transcription, a small
   text model and its voice; try speech-to-speech only if that is not good
   enough.
3. Agreement with Jev on a fixed set of our own questions (replay over kept
   turns; Jev's record has no state, so the set must be generated).
4. Time to first audio for the text model plus Phonon.
5. Heat and battery over ten minutes of holding conversation.
6. The audio-state switch time, wake-word false triggers over programme
   audio, speaker-check accuracy on a real microphone, and battery per hour
   per mode (voice-modes note).
7. The whole path to first acknowledgement, timed on a phone; Jev's part
   (110 to 260 ms mean upstream for 1 to 12 questions, our measurement on
   2026-11-10) is the only part measured.

Every turn writes a record (configuration, stage timings, memory and
thermal state, outcome) so models are compared by replay, not impression.

## 7. Could not verify

- Any iPhone figure for d1, LFM3-Audio, LFM3-1.7B in absolute terms, and
  Phonon (the app can now play it beside Kokoro, section 10; nobody has measured it on the phone).
- Whether Apple's model reaches context beyond a reported 4K tokens.
- Whether llama.cpp's public interface exposes LFM3-Audio speech out.
- Whether a 14 Pro Max runs iOS 27 and Core AI exports.
- Memory fit of LFM3-Audio on 6 GB.
- Licence, size and release date for most non-Liquid candidates, and which
  licence governs the Soniqo package.
- Whether two microphones can be open at once (believed not; general
  knowledge, a spike question).
- No new phone-sized decision model from October or November 2026 was found
  beyond those listed; that is a limit of the search, not proof.
- No fetch was made for this page: re-fetch and cite before relying on any
  figure.

## 8. Findings of 2026-11-11 (fetched today; each marked)

Source for each line is a Hugging Face API listing or card, a vendor page or
a repo, all fetched 2026-11-11; links in the design note's source list.

### 8.1 Corrections to sections 1 to 7

- **Apple Foundation Models and the 14 Pro Max (vendor, support.apple.com
  121115):** the Apple Intelligence iPhone list is "iPhone 16 models or
  later, iPhone 15 Pro, iPhone 15 Pro Max, iPhone Air" plus a model named
  iPhone Duo. The 14 Pro Max is not on it: unavailable there. Apple's
  framework page did not render for the fetcher; its context size stays
  unconfirmed.
- **iOS 27 on the 14 Pro Max (3P, 9to5mac):** Apple says iOS 27 supports the
  same iPhones as iOS 26, a list that includes the 14 Pro and 14 Pro Max, so
  Core AI builds are possible.
- **LFM Open License (vendor, full text):** Threshold is annual revenue of
  US$10,000,000 or more, counted across the legal entity and its controlled,
  controlling and commonly controlled entities; non-profit research
  exception; works that only link to the interfaces are not derivative works.
  FluidInference's card says commercial use at or above the threshold needs
  a separate licence.
- **LFM3-Audio-2B (vendor card):** 2.0B parameters, 32K context, text or
  16 kHz audio in, text or 24 kHz speech out; voice chat English only; 18
  speech languages; "time to first token as low as 50 ms" (hardware not
  stated); interruption not mentioned; licence LFM Open License v1.0.
- **Phonon (vendor, Gradium):** 31M English, Apache-2.0, 65 MB; 28 ms p95 time
  to first audio on a CPU streaming benchmark (Pocket TTS 165 ms on it); the
  40M multilingual variant is non-commercial. No iPhone measurement. Arm
  separately reports about 28 ms and 15x real time on Arm CPUs (relayed).
- **FluidAudio (vendor README):** Kokoro does not stream; "physical iOS
  inference remains unvalidated" for Kokoro; Phonon 31m about 20x real time on
  the Neural Engine; Parakeet and Kokoro v3 need iOS 18.

### 8.2 LFM3-Audio-2B builds (Hugging Face API sizes)

| Build | Size | Notes |
|---|---|---|
| GGUF Q4_0 / Q4_K_M / Q8_0 / F16 | 1,050 / 1,100 / 1,846 / 3,467 MB | llama.cpp v0.8.0 (b11960) or newer; `llama-tts`, `POST /tts`, `/v1/realtime` |
| mmproj Q4_0 / Q4_K_M / Q8_0 / BF16 | 161 / 172 / 304 / 571 MB | audio projector, needed for audio in |
| MLX 4-bit / 8-bit | 1,585 / 2,335 MB + 147 MB codec | MLX-VLM, Apple Silicon; no MLX Swift port for LFM3 found |
| Core ML (FluidAudio) | LM int8 block-32, ASR about 2.5 GB, speech out +0.4 GB, context 4,096 | int4 failed the authors' speech-frame check; vendor M5 Pro: TTS first audio 37 ms, 4.3 to 4.5x real time, live ASR 10.4x, ASR peak 1.9 GB |
| Core AI (iOS 27) | LM 2,159 MB + depth decoder 218 MB | vendor M5 Pro: TTS 5.07x, ASR 24.6x |
| LiteRT / litertlm | 1,146 to 1,531 MB int4; 2,141 MB int8 | Android oriented |
| ONNX | q4 decoder 1,687 MB + encoder and codec | not iOS-documented |

No smaller LFM3 audio model exists. No iPhone measurement of LFM3-Audio by
anyone (unk). Our Mac run (ours, `mobile/README.md`, loaded M1) was slower
than vendor M5 Pro numbers by about 5x for ASR and 7x for speech out, and
slower than real time for both.

### 8.3 Text model files and figures

Qwen3-0.6B Q4_K_M 397 MB (Q8_0 639 MB is what we run); Qwen3-1.7B Q4_K_M
1,107 MB; Qwen3.5-0.8B Q4_K_M 533 MB, non-thinking by default (card);
Qwen3.5-2B Q4_K_M 1,281 MB; LFM3-1.7B Q4_K_M 1,100 MB, QAD-Q4_0 1,050 MB,
thinking on by default, switch off "can be inconsistent" (card);
LFM2.5-1.2B Q4_K_M 731 MB; LFM2.5-350M Q4_K_M 229 MB; Gemma 3 1B Q4_K_M
806 MB; Gemma 3 270M Q4_K_M 253 MB; Gemma 4 E2B Q4_K_M 3,107 MB (Apache-2.0,
2.3B effective); SmolLM3-3B Q4_K_M 1,915 MB (`/no_think`); Phi-4-mini Q4_K_M
2,492 MB; Llama 3.2 1B Q4_K_M 808 MB, 3B 2,019 MB; MiniCPM5-2B Q4_K_M
1,561 MB.

iPhone decode speeds (3P unless marked): iPhone 17 Pro (A19 Pro): Qwen3-0.6B
MLX Q4 159 to 167 tok/s, LiteRT-LM 121, Core AI 193; Gemma 4 E2B LiteRT-LM
61, MLX 49, Core AI 47, llama.cpp Q4_K_M 39, sustained 600 s run falls to 18
to 27; LFM3-1.7B Q4_0 43.6 tok/s at 1.90 GB (vendor chart, read through a
search summary). iPhone 14 Pro (A16), December 2023 llama.cpp: TinyLlama 1.1B
Q4_0 pp512 505, tg128 54 tok/s; Q8_0 34; Phi-2 2.7B Q4_0 tg 23. iPhone 15 Pro
(A17 Pro) same model Q4_0 pp 646, tg 57. Nothing newer on an A16 or A17 Pro.
Sources: john-rocky/apple-silicon-llm-bench, llama.cpp discussion 4508.

### 8.4 Speech-to-speech models

Moshi MLX q4 4,806 MB + 385 MB codec (CC-BY 4.0; 160 ms theoretical; the
repo says MLX is for iPhone and Mac, no iPhone figure); NVIDIA PersonaPlex 7B
(8.4B, 16.7 GB BF16); NVIDIA NemotronLabs VoiceChat 11B (about 450 ms
turn-taking, MLX 4-bit about 9 GB, OpenMDW 1.1); LFM2.5-Audio-1.5B MLX
4/5/6/8-bit about 0.9/1.1/1.3/1.7 GB with an MLX Swift library; Sesame CSM-1B
6.2 GB (speech generator only); Ultravox v0.5 1B (audio in, text out). No
distilled Moshi found. None has an iPhone measurement. Nothing except the
two Liquid models is small enough for 6 GB beside the app.

### 8.5 iOS memory limits

Apple documents no per-device figure. Forum measurements (3P): about 6,144 MB
with the increased-memory-limit entitlement on an 8 GB iPhone 16 Pro Max and a
12 GB iPhone 17 Pro Max (6,656 MB with the debugging entitlement); 2.2 to 2.3
GB available on a 4 GB iPhone 13, iOS 15.3.1. A 6 GB iPhone: unk. The
llama.cpp xcframework includes libmtmd (PR merged 2026-06-25); open issue
#30556 reports freed model memory leaving the footprint elevated on iOS.

### 8.6 Still to measure on the phone

Everything in section 6 plus: Qwen3-0.6B Q4_K_M versus Q8_0, Qwen3.5-0.8B,
LFM3-1.7B QAD-Q4_0 decode and prefill with a cached page; Phonon 31M first
audio; `os_proc_available_memory()` at idle and after each load.


## How the app uses them (round 3, 2026-11-11)

- One catalog: `KnownModels` in the kit (`mobile/AutoradioKit/Sources/AutoradioKit/KnownModels.swift`), one store
  (`ModelStore.shared`, created with the catalog registered), one provider (`HubModelProvider` in the models module,
  installed by the app module's registrar) that fetches, loads and checks every model. Voice setup, the engine's stages
  and `autoradio-models --setup` all go through them. The first phone walk's "model ... not registered" came from a store
  of setup's own that nothing had registered into.
- A build with no model libraries is refused: `scripts/build-lab.sh` runs `scripts/ensure-deps.sh` (xcframeworks from
  `~/.cache/autoradio-deps`, else built), and `sync-exact-package.sh` exits unless `ALLOW_NO_MODELS=1`.
- Verification: FluidAudio checks Hugging Face LFS SHA-256 object ids of the files it fetches (read in v0.18.3
  `Shared/Download/ModelCache.swift`); the Qwen3 GGUF is checked against the pinned SHA-256 by the kit.
- The responder (the voice that talks while the desk works) is its own setup stage ("Talking while it works"), separate from quick
  decisions, each with its own llama.cpp context (a 0.6B model costs about 0.1 to 0.25 GB of KV cache per context at 1,024 to 2,048
  tokens; two roles on one GGUF share the weights). Ladder: Qwen3-0.6B Q4_K_M, Q8_0, Qwen3.5-0.8B Q4_K_M, LFM3-1.7B QAD-Q4_0 (LFM Open
  License: read it), then the soft cue alone. Sources and SHA-256 digests are the Hugging Face API listings of 2026-11-11.
- The response is a stream, sentence by sentence, front-loaded, cut when the desk's speech is ready, with a hand-over sound. No
  deterministic filters, word, token or time limits: it is steered by the prompt (`AckPrompt.system()`), and the recent lines ride in
  the prompt (`AckPrompt.userSuffix`). The prompt is split so the cache can be reused: the fixed instructions and the page in front
  (title, kind, text) are a byte-stable prefix read into the KV cache when the page comes to the front (`warmResponder`), and at end
  of speech only the listener's words are read. The turn timeline records prompt tokens, tokens already cached and the prefill time.
- Setup measures on the device: `os_proc_available_memory()` (iOS) instead of physical memory for the plan, and per candidate the
  tokens per second reading and writing, first sound for a voice, memory in use and left, and the phone's temperature; a candidate
  too slow or leaving too little memory fails its check and setup moves on. The numbers are shown in plain words on the result row.
- Proof on the Mac: `SETUP=1 mobile/scripts/proof-models.sh` (setup's own path, every model); the phone-only facts are
  listed in the round 3 report.

### Responder sample runs on the Mac (2026-11-11, `SETUP=1 mobile/scripts/proof-models.sh`; Mac numbers, not phone)

Four asks on invented demo pages, cold and warm cache, the same prompt for every model (`AckPrompt.system()`, the page as the stable prefix,
recent lines of the same page in the suffix). Reading speed is prompt tokens per second, writing is decode tokens per second, memory is the
whole Mac process resident after the load (all earlier models still loaded), so compare the models with each other, not with a phone.

| Model | Read | Write | First sentence, warm | What it said |
|---|---|---|---|---|
| Qwen3-0.6B Q8_0 | 2,690 | 52 | 0.3 to 0.5 s | grounded on pages that have text; echoes the instructions at times ("Show that I understood what the listener asked."); invents on an empty page ("The park closed because it was not in the mail.") |
| Qwen3.5-0.8B Q4_K_M | 1,671 | 51 | 0.3 to 0.9 s | relevant but wanders ("Today's paper is a newspaper rather than a magazine"), adds off-topic lines and questions to the listener |
| Qwen3-0.6B Q4_K_M | 2,429 | 59 | 0.15 to 0.4 s | generic filler ("I understand your question, and the front page includes the top stories you mentioned."); little page content |
| Qwen3-1.7B Q4_K_M | 892 | 30 | 0.4 to 0.5 s | echoes the prompt ("The most useful thing I can truthfully say right now is that ..."), fabricates on an empty page ("the park closed because of maintenance") |
| LFM3-1.7B QAD-Q4_0 (non-thinking: the prompt ends `<think></think>` per its card) | 897 | 34 | 0.5 to 1.0 s | the most faithful to the page and the only one that says plainly when the page has nothing ("I do not have any information about a park"); but it repeats itself and does not stop (11 to 15 sentences) |

Ladder order follows: Qwen3-0.6B Q8_0 (its file is the one the decision role loads, so no extra weights), Qwen3.5-0.8B, Qwen3-0.6B Q4_K_M,
Qwen3-1.7B, then LFM3 last before the soft cue only because of its licence; the owner can move it up after reading section 5 of the licence.

## The configurator (round 3, 2026-11-11)

Owner: "I want all the models downloadable. None ship with the app. The point is to build a configurator that selects the best models for the
user's specific device in the onboarding." Code: `mobile/AutoradioKit/Sources/AutoradioKit/ModelCandidates.swift` (the table),
`Configurator.swift` (device probe, thresholds, planner, runner, record), `LiveConfigBackend.swift`, `VoiceSetup.swift` (rows),
`HubModelProvider.swift` (measurement). Tests: `ConfiguratorTests.swift`.

**Procedure.** Probe the device (model id, chip, physical memory, `os_proc_available_memory()`, free disk, thermal state, OS). Below about 6 GB
the result is a plain "not supported" with every role on its built-in floor. Otherwise plan configurations (one candidate per role: voice
activity, listener, voice, decision, responder), the WHOLE pipeline resident together: memory is estimated from file sizes and measured
overheads, two roles on one file count its weights once, and configurations that could fit are ordered best quality first (score = sum of role
weight times the candidate's quality rank; the responder weighs 10, the voice and listener 2, the decision and the voice-activity model 1).
Then try them in order: download only what that configuration needs (resuming, SHA-256 or the library's own check), load everything at once,
measure, and accept the first that leaves headroom and meets the speeds; otherwise unload it, delete what it fetched that the next does not need,
and step down. At most six attempts (a count), then the built-in voice. Weights of every candidate not in the accepted configuration are
deleted. The whole record (probe, plan, every attempt's downloads and measurements, reasons, the thresholds in force, the chosen configuration
and one plain line per role) is saved and goes in the trace export (`configuration`).

**Thresholds** (`ConfigThresholds.standard`, one place, each with its source in the code comment):

| Threshold | Value | Where it came from |
|---|---|---|
| planning headroom | 600 MB | the Mac runs: a process grows 0.1 to 0.25 GB while generating; a guess to tune |
| app base | 300 MB | unmeasured on a phone |
| headroom after load | 500 MB | forum: iOS ends a process at its limit (2.2 to 2.3 GB on a 4 GB phone); a guess to tune |
| responder writes | 15 tok/s | speech is about 4 tokens a second; 3 times ahead |
| responder reads | 150 tok/s | a 400-token page cold in about 3 s (warm: only the question) |
| first sound of the voice | 1,500 ms | hand-over from the phone's voice; Mac Kokoro 243 ms, Pocket 98 ms |
| listener speed | 0.5 of real time | must keep up using half the compute; Mac 0.04 to 0.18 |
| a decision | 1,500 ms | Mac 58 ms |
| thermal after the run | nominal or fair | serious and critical mean throttling |
| sustained run | 3 rounds of the same work | a fixed amount of work, not a duration |
| available memory (assumed where the device cannot say) | 60% of physical | forum: 6.1 GB of an 8 GB phone with the increased-memory entitlement |

**Candidate table.** Responder (quality rank is data, from our Mac sample runs of 2026-11-11; `*` = a quant we have not run ourselves, ranked within its family
by the vendor's bit-depth ordering; sizes and digests are the Hugging Face API listings):

| Id | Size | Rank | Min tier | Measured |
|---|---|---|---|---|
| lfm3-1.7b-q8_0 | 1,846 MB | 98 | 8 GB | no * |
| lfm3-1.7b-q6_k | 1,428 MB | 96 | 8 GB | no * |
| lfm3-1.7b-q5_k_m | 1,259 MB | 94 | 6 GB | no * |
| lfm3-1.7b-qad-q4_0 | 1,050 MB | 93 | 6 GB | yes |
| lfm3-1.7b-q4_k_m | 1,100 MB | 91 | 6 GB | no * |
| lfm3-1.7b-nvfp4 | 1,280 MB | 90 | 8 GB | no (whether llama.cpp b12259 runs NVFP4 on iOS is unknown) |
| qwen3.5-2b-q8_0 / q4_k_m | 2,012 / 1,281 MB | 66 / 64 | 12 GB / 8 GB | no (never run) |
| qwen3-0.6b-q8_0 / q4_k_m | 639 / 397 MB | 62 / 52 | 6 GB | yes |
| qwen3.5-0.8b-q8_0 / q4_k_m | 812 / 533 MB | 58 / 56 | 6 GB | q4 yes |
| qwen3-1.7b-q8_0 / q6_k / q4_k_m | 1,834 / 1,418 / 1,107 MB | 50 / 49 / 47 | 8 / 8 / 6 GB | q4 yes |
| cue-only | 0 | 0 | the floor | |

LFM3-1.7B's repository (`LiquidAI/LFM3-1.7B-GGUF`) lists BF16, F16, NVFP4, Q4_K_M, Q5_K_M, Q6_K, Q8_0 and QAD-Q4_0; there is no plain Q4_0 and nothing under
about 1.05 GB. BF16 and F16 (3.5 GB) are not in the table. Other roles: voice activity (Silero, level detector floor), listener (Parakeet TDT, Parakeet EOU, Apple
floor), voice (Kokoro, Pocket, system voice floor), decision (Qwen3-0.6B Q8_0, the computer floor). LFM3's licence limit (commercial use at or above US$10M annual
revenue needs a separate licence from Liquid AI) is recorded in the table; it does not lower its rank: that is the owner's call.

**Mac proof** (`CONFIGURE=1 mobile/scripts/proof-models.sh`, simulated 6, 8 and 12 GB profiles; the planner enforces the memory budget, speeds are the Mac's):
6 GB chose LFM3 QAD Q4_0 with Parakeet TDT, Pocket voice and Silero (decision through the computer); 8 GB chose LFM3 Q8_0 with Parakeet TDT, Kokoro and Silero; 12 GB
chose LFM3 Q8_0 with the on-phone Qwen3-0.6B decision as well.

## 9. The default voice (2026-11-15, owner QA round 1)

Owner on the phone: the app spoke with the built-in system voice, and a better voice loaded when he picked one in Settings. Each line is
marked **verified** (read or run on 2026-11-15 by us, source named) or **relayed** (from a card or an earlier page, not re-checked).

**Why the system voice spoke (verified, code).** (1) `StageRegistry.listenerConfiguration()` named the stage `avspeech` for the voice.
(2) A voice picked on the Components page or by voice setup went through `LabRuntime.setStage`, which changed the in-memory configuration
only: every launch built the listener configuration again, so a pick lasted until the app closed. (3) Nothing recorded that a neural voice's
files were on the phone, so nothing could prefer one safely: a Kokoro or Pocket stage whose files are missing makes FluidAudio fetch about
350 MB inside the first sentence. Fix: `VoiceChoice.swift` (saved choice, default Kokoro Heart, readiness record, `makeSpeaker` with a line
by line fall back to the system voice, with the reason on the log).

**What the linked FluidAudio (v0.18.3, `~/.cache/autoradio-deps/xcframeworks`, built 2026-11-11) offers for speech (verified, symbol
listing of `libFluidAudio.a` and the `.swiftmodule`; there is no `.swiftinterface` in the xcframework, only the binary module).**
`KokoroAneManager` (`initialize(preloadVoices: Set<String>?)`, `synthesizeDetailed(text:voice:speed:)`, `setDefaultVoice`),
`PocketTtsManager` (`synthesize`, `synthesizeStreaming`), `LuxTtsManager`, `NeuTtsManager`, `Lfm3Audio*`, **and `PocketTtsSmallManager`, which
speaks Phonon** (see section 10). **Correction (2026-11-15, verified, FluidAudio v0.18.3 source and the `.a`'s symbols):** an earlier version
of this paragraph said the linked library had no Phonon synthesizer. That was wrong. The Phonon voice is not a class of its own: it is
`PocketTtsSmallModel.phonon31m` run by `PocketTtsSmallManager` (252 `PocketTtsSmall` symbols are in `libFluidAudio.a` for the device slice),
beside `PhononTextNormalizer` and `PhononTextChunker`, which only prepare its text. The search that found none looked for a type named
Phonon. The three repositories in the library's strings are three different things: `phonon-31m-en` is the voice (a folder inside
`FluidInference/pocket-tts-coreml`), `FluidInference/phonon-2-coreml` and `Muesli-HQ/phonon-2.5-coreml` are speech recognisers (Fermion
Research's Phonon-2 and 2.5, retrained Parakeet), not voices. Two unrelated models share the word.

**What the app speaks with now.** Default: Kokoro 82M, voice `af_heart` (Apache-2.0, `FluidInference/kokoro-82m-coreml`; model about 350 MB,
relayed from `KnownModels`; the repo's `ANE-v3` folder is 728.8 MB because it also holds the Japanese and Chinese models, verified by the
API listing). That is over the 100 MB line, so it is downloaded during setup (voice setup's first rung), not bundled; the system voice speaks
until `Voices.readiness` records it. Cards also offer Kokoro `af_bella`, `am_adam`, `bf_emma`, `bm_george` (all five voice files are in
the repo's `voices/` folder, 2.7 MB each, verified by the API listing; the stage preloads all five with the model) and Pocket `alba`
(Pocket's other voice names were not found in the library's strings), and the phone's own voice. Kokoro does not stream (vendor, section
8.1): the first sound of a block is its synthesis time; blocks of speech are now at most 280 characters and the next block is made while
the current one plays. Kokoro on a physical iPhone is **unvalidated by the vendor**; the owner's phone picked and played one on 2026-11-15
(which voice and how long it took to a first sound: not recorded, measure it).

**Still to measure on the phone:** time to first sound of the first block for Kokoro `af_heart` after a cold launch (the first load compiles
the models: the speaker gives the system voice a line when no sound has come in 25 s), memory with Kokoro plus the other models resident on
the 6 GB 14 Pro Max (`os_proc_available_memory()` before and after), and whether the four other Kokoro voices load without a second fetch.

## 10. Phonon 31M as an alternative to Kokoro (2026-11-15, owner QA round 1)

The owner asked for Phonon beside Kokoro to listen to and compare, and where it is downloaded from. Everything below was fetched on
2026-11-15 (about 22:59 UTC) and is marked: **verified** = read from the primary source on that date, **relayed** = a vendor claim not
tested here, **not run** = needs the device.

**Which Phonon.** There are two unrelated models called Phonon. (a) Gradium's Phonon, a text-to-speech model family released 2026-10-20 (the
card; the page earlier said 2026-10-21). (b) Fermion Research's Phonon-2 and 2.5, speech recognisers that retrain NVIDIA's Parakeet; the
FluidAudio enum cases `.phonon2` and `.phonon25` and the repositories `FluidInference/phonon-2-coreml` (CC-BY-4.0) and
`Muesli-HQ/phonon-2.5-coreml` are these. The earlier finding that the repository `FluidInference/phonon-2-coreml` "holds speech-recognition
files" was right and is the reason it must never be used for a voice: it is (b) (**verified**, its model card: "Core ML build of
FermionResearch/Phonon-2 ... English automatic speech recognition", encoder 341 MB).

**Where the voice weights are (verified, Hugging Face API and model cards).**

| What | Repository and path | Files and sizes | Licence |
|---|---|---|---|
| The original (Gradium) | `gradium/phonon-31m-en` (updated 2026-10-22) | `model.q8.gguf` 64,512,352 B; `model.q4k.gguf` 55,075,168 B; `model.safetensors` 102,823,920 B; `default-voice.safetensors` 256,136 B; tokenizer and config files | Apache-2.0 for weights, tokenizer and voices (card header and text; the runtime `ptts` is MIT OR Apache-2.0) |
| The ~100 voices | `gradium/phonon-voices` | named on the card; not downloaded | stated Apache-2.0 on the model card for "voices" |
| **The Core ML pack the app uses** | `FluidInference/pocket-tts-coreml`, folder `phonon/phonon-31m-en` (repository last modified 2026-11-01) | 126 files, 173,136,428 B after FluidAudio's own skip rules (below); the big ones: `pocket_state.mlmodelc` weights 79,068,864 B, `mimi_decoder.mlmodelc` weights 44,897,216 B, `constants_bin/text_embed_table.bin` 8,194,048 B, four 3,073,840 B voice states, 103 voice files of 256,208 B | The folder carries its own `LICENSE` (Apache-2.0 text, 11,357 B) and a README that says so. The repository's front-page tag says `cc-by-4.0` (that is Kyutai's Pocket TTS, the other folders): the Phonon folder is the exception |

The Gradium card also lists Phonon 40M models (English, French, German, Spanish, Portuguese, multilingual) under **CC BY-NC 4.0**:
non-commercial, not for the app. Only the 31M English is Apache-2.0 (**verified**, card table). The card lists a runtime for iPhone
(`github.com/gradium-ai/xn-ptts`, `ios/PhononTTS`): its README was not read here and the app does not use it.

**The runtime the app already links (verified: v0.18.3 source, tag `c8b41f98`, 2026-11-07, and the symbols of the cached `libFluidAudio.a`).**
FluidAudio added Phonon 31M TTS in v0.18.0 (release notes: "Gradium Phonon 31m TTS"). The API, read in
`Sources/FluidAudio/TTS/PocketTTS/Small/PocketTtsSmallManager.swift` and `PocketTtsSmallModel.swift`:
`PocketTtsSmallManager(model: .phonon31m, defaultVoice: String?, computeUnits:, mimiPrecision:)` (an actor, iOS 18 / macOS 15);
`initialize(directory: URL?, progressHandler:)` downloads and loads; `synthesizeStreaming(text:voice:temperature:seed:)` returns an
`AsyncThrowingStream<PocketTtsSynthesizer.AudioFrame, Error>` of 80 ms frames (streaming, 24 kHz, `arch.json` `sample_rate` 24000, mono);
`synthesize` returns 24 kHz 16-bit WAV data. The first sound is the first frame, as with Pocket. Voices are named `<locale>_<Name>`
(`en_us_Easton`) or `default`; four have ready-made KV snapshots (`default`, `en_us_Easton`, `en_us_Zoey`, `en_gb_Maeve`) and start at
once; the other 99 are prefilled once on first use. The download keeps (`PocketTtsSmallDownloader.shouldSkipAsset`) `pocket_state.mlmodelc`,
`mimi_decoder.mlmodelc`, the JSON descriptors, `constants_bin/` and `voices/` and skips the fp32 reference models, every `.mlpackage`,
`constants/`, `manifest.json` and `.wav` files. The 173 MB is the sum of what that rule keeps, computed from the API listing (**verified**
arithmetic, not a download). So the pack is 173 MB, not the 64 MB of the GGUF: it holds the Core ML model in fp16, the fp32 Mimi decoder
the pack's README says to use ("the fp16 Mimi adds near-white noise at about -80 dBFS in pauses") and every voice. That is over the 100 MB
line (bundling stays a later decision; it downloads).

**Upgrade cost: none (verified).** The pinned FluidAudio v0.18.3 already has it; `mobile/scripts/ensure-deps.sh` and
`build-xcframeworks.sh` (`TTS` trait on) need no change and no new xcframework is built. A newer FluidAudio exists on `main` (HEAD
`16780f04`, 2026-11-13, not tagged; the newest tag is v0.18.3): the diff of `Sources/FluidAudio/TTS` between v0.18.3 and that head is 14
files, +881 -60 lines, in Kokoro ANE, Paradee, `PocketTtsSynthesizer.swift` and `PhonemeChunker.swift`. The one change that touches the
Phonon path (read in the diff) is a chunker fix in `PocketTtsSynthesizer.swift`: a chunk no longer ends on a decimal number ("a fee of
$822.95." was cut at the point, "stopping before the decimals"). It is unreleased and not needed to hear Phonon, so it was not taken;
a pin on the next tag is the way to get it. Nothing of the existing Kokoro, Pocket, Parakeet, Silero or LFM3 call sites changes.

**What was built.** `PhononVoice` (stage `phonon`, `AutoradioModels/VoiceStages.swift`) beside `KokoroVoice` and `PocketTTSVoice`, the model
`phonon-31m` in `KnownModels` (source `hf:FluidInference/pocket-tts-coreml/phonon/phonon-31m-en`, 173 MB), three voice cards right after
Kokoro's (Easton, Zoey, Maeve, each marked "(Phonon)" so the cards read back to back), its fetch and check in `HubModelProvider`, a case in
`ModelProof`. It reuses the app's existing path: `VoiceSampler` (progress bar, one download for the three cards), `Voices.readiness`
(marked only after a fetch that ended in a spoken line), `makeSpeaker` (the phone's voice speaks a line the model cannot). The default
stays Kokoro Heart. The pack is placed under `KnownModels.fluidRoot` (`initialize(directory:)` is given the folder above it, so the
progress bar and the "is it on disk" check watch the folder FluidAudio writes).
Known looseness, not fixed here: the existing `pocket-tts` entry names the cache folder `pocket-tts` but FluidAudio writes
`Models/pocket-tts-coreml` (also, on a Mac, under `~/.cache/fluidaudio`, not the app's `fluidRoot`); its progress bar still works because
it watches the whole `fluidRoot`, its "on disk" check likely misses. Phonon's entry avoids this.

**Not run (needs the device, the lead's one build).** That the Exact build links and runs `PocketTtsSmallManager`; the real download from
Hugging Face; the first sound time and the sound itself on the 14 Pro Max (the vendor says 28 ms to first audio on Arm CPUs and about 20x
real time on the Neural Engine; **relayed**, unmeasured on a phone); memory beside Kokoro. The model needs iOS 18 (MLState); the
app's minimum is 18.0.

Sources: https://huggingface.co/gradium/phonon-31m-en ; https://huggingface.co/FluidInference/pocket-tts-coreml/tree/main/phonon/phonon-31m-en
(and `/api/models/FluidInference/pocket-tts-coreml/tree/main/phonon/phonon-31m-en?recursive=true`) ;
https://huggingface.co/FluidInference/phonon-2-coreml ; https://github.com/FluidInference/FluidAudio/releases (v0.18.0 to v0.18.3) ;
FluidAudio tag v0.18.3 `PocketTtsSmallManager.swift`, `PocketTtsSmallModel.swift`, `PocketTtsSmallSession.swift`.

## 11. Fluid Speech Mini, the recogniser the owner chose (2026-11-24)

Owner, 2026-11-24: Apple's recogniser and voice "isnt good enough"; "we are only using that currently because of a coding bug. (we shuold be
using fluid speech mini etc)"; "we will keep updating the models as better ones come". The model is the one the owner chose and built into
their other app, Mike. Everything below was read on 2026-11-24 and is marked: **verified** (primary source that day), **relayed** (Mike's own
notes, not re-run here), **not run** (needs the phone).

**What it is (verified, the repository's `config.json` and `NOTICE`).** `FluidVoice-Labs/fluid-speech-mini-coreml`, version 1.1.0, English
only, 16 kHz mono, 15-second windows, vocabulary 8192 plus a blank. Built on NVIDIA Parakeet TDT 0.6B v3 and Moondream Parakeet Ultra (the
decoder and joint network are Parakeet Ultra's; the audio preprocessor is FluidInference's Parakeet v3 Core ML one). Minimum OS iOS 18 and
macOS 15 (`config.json`), the same floor as the package. Batch, not streaming: no partial words. Mike's notes (**relayed**): runs on the
Neural Engine, about 277 MB of RAM per 15 s window, 1.51% word error on isolated clips against Parakeet 0.6B v3's 1.92%, and its output
carries punctuation and capitals.

**The pin (verified, `GET https://huggingface.co/api/models/FluidVoice-Labs/fluid-speech-mini-coreml/tree/<commit>?recursive=1`, no credentials, 2026-11-24).**
Commit `d81c3160dfe5a5e1f8f7267164a07d30617a3a1a`, last modified 2026-10-13, tag `license:cc-by-4.0`. The repository lists 31 files; the app
fetches the 24 the loader and the licence need (`.gitattributes`, the model card, two example files and two images are not fetched; `parakeet_v3_vocab.json`
is byte for byte `parakeet_vocab.json`, the same 151,122 bytes, and is not fetched). 252,730,493 bytes in all (253 MB). The tree API gives the
LFS object id (the SHA-256) only for the large files; the digests of the small ones were computed on the bytes downloaded at that commit, and the LFS
ones were re-checked against the API's. Every file's size and SHA-256 is in `KnownModels.fluidSpeechMini`; the big ones are the encoder
weights 233,096,704 B, the decoder weights 11,845,120 B, the joint weights 6,350,284 B.

**Loading (verified, FluidAudio v0.18.3 source, `AsrModels.loadLocal`, and Mike's `FluidSpeechMini.swift`).** `AsrModels.loadLocal(from: dir,
version: .ultra)` reads exactly that folder and never downloads; the `.ultra` build wants `Preprocessor`, `Encoder`, `Decoder` and
`JointDecisionv3` `.mlmodelc` and `parakeet_vocab.json`. The repository names the joint network `JointDecision.mlmodelc`, so the store copies
it as `JointDecisionv3.mlmodelc` after the download (`ModelBundleSpec.copies`). Use upstream `FluidInference/FluidAudio`, which the app pins
at 0.18.3, and not the `altic-dev` fork: Mike's spike (2026-10-21, **relayed**) found the fork's long-audio stitching dropped whole spans (22.4%
word error on a 31-minute stream) where upstream gave 2.05%.

**Licence and credit (verified, `LICENSE` and `NOTICE`).** CC BY 4.0. It asks for the credit "Fluid Speech Mini by FluidVoice. Built on
NVIDIA Parakeet TDT 0.6B v3 and Moondream Parakeet Ultra.", a link to the licence, and that LICENSE and NOTICE stay with the files (the
store fetches both into the model's folder). The credit text is `KnownModels.fluidSpeechMini.credit`. **The app has no About or licences
page that lists the models it uses yet** (the About page shows the build and server commits only, `mobile/AutoradioLab/about.ts`); until
one exists the credit is carried by the model list, and showing it is an open item for the lane that owns Settings.

**How the app fetches it.** Through the model store (`ModelStore.ensureBundle`), file by file, resumable, each file checked against its
SHA-256 before it is moved into place, only from voice setup's fetch after the person has agreed (the lane `onboard-consent` owns the
prompt). The stage itself never downloads: asked to begin without the whole folder it throws `unavailable`.

**What was built.** `FluidSpeechMiniTranscriber` (stage `fluid-speech-mini`, `AutoradioModels/FluidSpeechMini.swift`): ported from Mike's
file, keeping its behaviour: the model is warm (it runs once on a second of silence when it loads, because Core ML specialises for the Neural
Engine on the first prediction) and one pass over the speech-bounded audio gives the final transcript. Registered with the other model stages
in `ModelHub.register`, fetched and checked in `HubModelProvider`, a case in `ModelProof`.

**Measured on the Mac, 2026-11-24 (Mac, not phone; FluidAudio 0.18.3; the files fetched through the store and every size and SHA-256 re-checked).**
Fluid Speech Mini loads from the pinned folder through `AsrModels.loadLocal(version: .ultra)` (load plus warm-up 0.42 s on the Mac) and
transcribes. On the three synthesised turns in `mobile/fixtures` it and Parakeet TDT v2 both score 0% word error (a voice made by `say` is not
a test of recognition). On real readers, the 73 clips of the LibriSpeech "clean" validation subset (hf-internal-testing/librispeech_asr_dummy,
8.0 minutes, 1,169 words; the reference lower-cased, punctuation dropped, "harbour" and "harbor" treated as one), corpus word error:

| Recogniser | Word error | Time per second of audio (Mac) |
| --- | --- | --- |
| Parakeet TDT 0.6B v2 | 2.57% | 0.014 |
| Fluid Speech Mini | 3.93% | 0.017 to 0.019 |
| Parakeet EOU 120M (the live-words stage) | 8.13% | 0.068 |
| Apple's recogniser (on device, this Mac) | 12.66% | not timed |

Read: Fluid Speech Mini is three times better than Apple's here and about 5 points better than the streaming model, **but not better than
Parakeet TDT v2 on this set (3.93% against 2.57%)**; Mike's 1.51% against Parakeet v3's 1.92% was a different set and a different Parakeet
(v3). Its table rank (90, above TDT's 80) follows the owner's choice and Mike's numbers, not this measurement; the owner decides whether to
re-rank. One set of read book speech on a Mac, no noise, no phone microphone: weak evidence either way. The run is
`AUTORADIO_REAL_SPEECH=<folder> AUTORADIO_COMPARE_PARAKEET=1 swift test --filter testRealSpeechWordError` (the folder is not in the
repository; it is made from the parquet file named above).

**The model list as data (`Voice/ModelList.swift`).** `ModelList` is one structure (job, repository, pinned revision, files with size and
SHA-256, licence and credit, per-device recommendation rules, rank) that decodes from JSON, skips an entry it cannot verify or a job it does not
know, and has a built-in copy derived from `CandidateTable` and `KnownModels.bundles`. Not built: the server route, the fetch, and the choice
reading it instead of `CandidateTable`.

**What is left of Mike's pipeline (sizes in lines of Swift, plus model files).** Silero is already the speech detector the pipeline picks when
its model is ready (nothing to port; making it ready by default is voice setup's download). Smart Turn v3.2: Mike's `SmartTurn.swift` is 56 lines
plus a four-file Core ML bundle of 16.8 MB (aufklarer/Smart-Turn-v3.2-CoreML, pinned in Mike's `VoiceModels.swift`), and the turn policy that asks it
after a pause (Mike's `TurnPolicy.swift`, 520 lines, of which the Smart Turn rules are a part) must be fitted to `HandsFree`'s end-of-turn rule:
about 150 lines and a day. EouStream: Mike's `EouStream.swift` is 224 lines over FluidAudio's public `RnntDecoder`, `Tokenizer` and
`AudioMelSpectrogram` (public in 0.18.3); it replaces the manager-based Parakeet EOU stage and fixes the lost words after a mid-sentence end;
about 250 lines with tests and a day. The speaker check as a stage: `SpeakerEmbedder` 196, `SpeakerFusion` 108 and `SpeakerProfile` 292 lines, two
embedding models (WeSpeaker, CAM++) and the fusion file, behind the existing `SpeakerCheck` protocol plus an enrolment step in onboarding; about
600 lines, two days, and it needs the models' pins read.
