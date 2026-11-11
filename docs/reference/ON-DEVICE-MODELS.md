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
| Voice | Phonon (Gradium) | 2026-10-21 | 31M parameters, 64 MB | Apache-2.0 | Swift and Core ML | 28 ms to first audio on Arm CPUs (vendor); English only; no iPhone measurement |
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
  Phonon.
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
