# Voice model catalog: what to add to Settings, Voice and models

Reference page (convention: [README.md](README.md)). Written 2026-11-24, read-only research and a plan; no code,
no model was downloaded, nothing was run on a phone. Owner, 2026-11-24: "i want to get more voice models (all types)
in the voice and models page in settings".

Marks: **read today** (a primary page or the Hugging Face API, fetched 2026-11-24, by me), **prior** (a finding in
[the 2026-11-22 research](../research/voice-models-2026-11-22/README.md) or in
[ON-DEVICE-MODELS.md](ON-DEVICE-MODELS.md) that I did not re-read), **maker** (the maker's own figure), **3P** (someone
other than the maker measured it), **ours** (we measured it), **none** (no figure exists). The prior research went
through a summarising fetch tool and says so; a figure marked prior may be mis-copied. **No model below has an iPhone 14
Pro Max (A16) measurement of ours except Silero VAD and Parakeet EOU** (mobile/README.md). Every recommendation is a
hypothesis until a phone run.

## 1. What exists now, and what is about to land

**Main (read today):** `settings:voice` is `VoiceScreen` in `mobile/AutoradioLab/settings.contract`: a sentence, the
voice cards (`VoiceCardsList`, `voice.contract`) and a note that "Voice setup, with the model downloads and the checks
for this device, opens from here once first run has it". The page lists voices only. Models are listed in three places in
Swift: `KnownModels.managed` (id, label, source, approximate bytes, licence, cache path), `VoiceCatalog.options` (ten voice
rows: five Kokoro, three Phonon, one Pocket TTS, the system voice) and `CandidateTable` (roles `vad`, `transcriber`,
`voice`, `decision`, `responder`, with rank, tier and licence). `Voice/ModelList.swift` already holds the model list as data
(job, repository, pinned revision, files with size and SHA-256, licence, credit, per-device rules, rank) with a built-in copy
derived from the tables; "not built: the server route, the fetch, and the choice reading it".
`ModelStore` has `delete(_:)` and `deleteBundle(_:)`; no screen calls them.

**`lane/onboard-consent` (not on main; read its diff against main, 104 files):** adds `settings:downloads`
(`downloads.ts`, `offers.ts`, `downloads.contract`) which already is a per-job picker in Exact: three groups (`voice`,
`voice-rest`, `search`), each job lists its options with name, size, "Recommended for this phone", "On this phone", a note
and a licence credit line, a pick is kept (`consent.ts picks`), one consented press per group, Wi-Fi and mobile-data
agreements, names and sizes read from the model list through the native side (`offers.ts` "writes no model's name or
size"). Readiness is derived from the pinned files and the pin the check passed for, and `scripts/autoradio-native-voice-list.txt`
fixes which native files may exist. `downloads.ts` carries the rule D9: "an option appears only when the list offers it".

**`lane/voice-pro` (not on main; held until onboard-consent lands):** Fluid Speech Pro as a recogniser row (652 MB, rank
95, recommended from 7 GB), the pin, the ladder row, a stage in `AutoradioModels/FluidSpeechPro.swift`.
**`lane/responder-omni`, `lane/omni-ears` (not on main):** LFM3-Omni-2B Q8_0 as a responder row (text path, 1.85 GB, 8 GB
class) and the Omni measurements (section 3.7).

**The consequence for the plan.** There is already a page that does per-job choice with sizes, recommendation and credit:
the Downloads page on onboard-consent. A second page for the same thing would be a second doctrine. "Voice and models" in
Settings should be that page's data-driven job list, plus the voice cards, and *adding a model is adding a model-list
entry*, not editing a screen. What main's readiness code does today (a flag per id, no per-file manifest) is being replaced
on that lane; nothing below should build on main's readiness.

## 2. The jobs on the page

| Job (list `job`) | Today (main) | Page shows |
| --- | --- | --- |
| Hearing you (`transcriber`) | Apple's recogniser until a model is ready; Parakeet EOU 120M (streaming), Parakeet TDT 0.6B v2, Fluid Speech Mini (CC BY 4.0); Pro on voice-pro | choice per job |
| Knowing when you speak (`vad`) | energy detector, Silero VAD v6.2.1 (Core ML, ~2 MB) | choice |
| Knowing when you have finished (end of utterance) | `HandsFree` end-of-turn rule; Parakeet EOU's end token | **no role exists**; needs one (section 6) |
| The voice (`voice`) | system voice, Kokoro 82M (5 of its voices), Phonon 31M (3 of 103), Pocket TTS (1 of 4) | voice cards grouped by model |
| Quick decisions (`decision`) / answering (`responder`) | Qwen3-0.6B Q8_0/Q4_K_M, Qwen3-1.7B, Qwen3.5-0.8B/2B, LFM3-1.7B in seven quants | choice |
| Knowing your voice (speaker) | WeSpeaker ResNet34-LM Core ML (CC-BY-4.0) and CAM++ in `Voice/` | choice (a "speaker" job is already in `offers.ts`) |
| Speech to speech | none | not offered (section 3.7) |

## 3. Survey

Sizes are file sizes from a repository listing unless marked. "Local files": whether the runtime loads from a folder with no
network (the app's rule: the store fetches after consent, the stage never downloads). "Both phones": has a build for iPhone
and for Android. The Android column is thin everywhere: our Android voice is Kotlin and its runtime for these models is not
in this repository's reading, so "Android" below means "a published build exists", not "the app can load it".

### 3.1 Speech recognition

| Model | Maker; licence (use and credit) | Size; memory | Quality | Speed | Runtime; local files; Android build | Source |
| --- | --- | --- | --- | --- | --- | --- |
| **Fluid Speech Pro** (in app on voice-pro) | FluidVoice; CC BY 4.0: commercial use allowed; credit "Fluid Speech Pro by FluidVoice. Built on NVIDIA Parakeet TDT 0.6B v3 and Moondream Parakeet Ultra." and keep LICENSE and NOTICE | 652 MB, 25 European languages, punctuation; Mac footprint counter cannot separate it from Mini (prior) | Mac, ours: 3.76% clean, 3.39% other, 35.8% AMI far-field (Mini 3.93, 3.56, 40.8; Parakeet v2 2.57, 3.06, 38.0). Leaderboard avg 6.06, rank 13 (prior, submitter's) | Mac 0.018 to 0.031 s per s of audio, ours. **No iPhone figure from anyone** (card: "Tested on Mac only") | FluidAudio `.ultra`, Core ML, `loadLocal`; local; Core ML only, but ONNX, GGUF and MLX builds are published by the same maker (prior) | ON-DEVICE-MODELS section 12 (voice-pro) |
| **Fluid Speech Mini** (on main) | FluidVoice; CC BY 4.0; credit as above (Mini wording) | 253 MB, English, batch | ours above; maker 1.51% on isolated clips | Mac 0.017 to 0.020 | same | KnownModels.fluidSpeechMini |
| **Parakeet TDT 0.6B v3**, Core ML (FluidInference) | NVIDIA base, **the card contradicts itself: "Apache 2.0" in the text, tag cc-by-4.0** (read today); the NVIDIA model is CC-BY-4.0 (prior); credit NVIDIA if CC BY | size not on the card (v2's Core ML is ~480 MB in KnownModels; v3 not read) | maker: ~110x real time on an M4 Pro; no WER on the page (read today). 2.29% clean / 4.51% other and 0.105 s median wait on an iPhone 16 Pro (3P, a voice2text issue; prior) | 3P as left | FluidAudio Core ML, iOS 17+ (read today); local; ONNX ports exist (soniqo: Galaxy S23 CPU RTF 0.082, 1.15 GB, maker, prior) | https://huggingface.co/FluidInference/parakeet-tdt-0.6b-v3-coreml |
| **Parakeet EOU 120M** streaming (on main) | NVIDIA Open Model License (KnownModels) | ~250 MB; ours on A16: 149 MB resident, first load 34 s, RTF 0.035 (before the 2026-11-15 ruling) | Mac: 8.13% on our clean set (ours), 4.88% at 320 ms (maker, M2) | iPhone 16 Pro RTF 0.04, 297 MB (maker soniqo, prior) | Core ML; ONNX INT8 on Galaxy S23: RTF 0.21, 232 MB (maker, prior) | prior |
| **Nemotron Speech Streaming 0.6B en**, Core ML | NVIDIA; **the Core ML card contradicts itself (metadata nvidia-open-model-license, text Apache-2.0)**, read today; resolve against NVIDIA's own card before use | encoder int8 ~564 MB, decoder ~28 MB, joint ~7 MB (read today) | Core ML publisher, M2: 1.99% at 1120 ms, 2.12% at 560 ms, **~10% at 160 ms, ~60% at 80 ms** (read today). Leaderboard English avg 7.47 (prior) | M2 9.6x RTFx at 1120 ms (read today); **iPhone: none** (read today) | FluidAudio Core ML; GGUF q8_0 700 MB, LiteRT int8 895 MB community (prior) | https://huggingface.co/FluidInference/nemotron-speech-streaming-en-0.6b-coreml |
| **Moonshine streaming tiny / small / medium** | Moonshine AI; MIT (prior) | 176 / 561 / 1,064 MB fp32 safetensors; int8 `.litertlm` of v1 tiny 29.7 MB (prior) | maker, open-ASR 8-set avg: 12.01 / 7.84 / 6.65 (prior) | iPhone 17 Pro to Galaxy S24 tiny-v1 numbers are a wrapper maker's (prior); none on A16 | LiteRT and ONNX, iOS and Android runtime library; Core ML none read | prior |
| **Apple SpeechAnalyzer / SFSpeechRecognizer** | system; no credit | none | Mac, 3P (a product maker): SA 2.12 clean / 4.56 other; ours on our clean set: Apple 12.66% (SFSpeech, a different test) | not timed | system; works offline only on supported devices | prior; owner ruled it "isnt good enough" |
| **LFM3-Omni-2B as ears** | Liquid AI; LFM Open License v1.0 | 1.3 GB Q4_0 + mmproj, 2.35 GB Q8_0 pair; Mac resident 1.5 to 2.4 GB | ours: weaker than Fluid on every set (clean 5.99, cafeteria 0 dB 27.6 vs Pro 14.0), invents text on silence (704 words in 16 empty clips) | ~9x slower than Fluid | llama.cpp with the mmproj; **Swift cannot call the audio path today** | ON-DEVICE-MODELS section 14 (omni-ears) |

### 3.2 Voice activity and end of utterance

| Model | Maker; licence | Size | Quality | Speed | Runtime; Android | Source |
| --- | --- | --- | --- | --- | --- | --- |
| **Silero VAD v6.2.1** (on main) | Silero; MIT | ~2 MB | ours on A16: measured fit (mobile/README.md) | tiny | Core ML (FluidInference); ONNX for Android | KnownModels |
| **Smart Turn v3.2**, Core ML (aufklarer) | Pipecat; **BSD-2-Clause, read today**; keep the licence text, no screen credit required | **16.8 MB** compiled model, 8.0M parameters (read today) | maker: **92.90% accuracy, 8.48% false positives, 5.61% false negatives on 1,000 upstream clips at 0.5** (read today; the v3 card says 94.31% on English, prior; the numbers do not agree and are not for our audio) | M5 Pro: 3.5 ms per 8 s window on CPU and Neural Engine; ONNX CPU 36.3 ms (maker, read today). One A78 core ~100 to 200 ms (estimate, prior) | Core ML, iOS 17+; ONNX 8 MB int8 for Android (prior) | https://huggingface.co/aufklarer/Smart-Turn-v3.2-CoreML |
| **TEN VAD** | TEN framework; Apache-2.0 "with additional conditions" (prior); **conditions not read** | ~0.3 MB | maker: RTF 0.0049 on an iPhone 8, 0.049 to 0.057 on weak Android (prior) | | C libraries for iOS and Android | prior |
| **turn-1-mini v2** | p99lab; Apache-2.0 | ~5.4M parameters | maker: 92.2% recall / 3.8% false alarm on its own multi-party bench (prior) | ~2.6 ms CPU per 160 ms step | Core ML build listed in speech-swift | prior |
| **Parakeet EOU's end token** (on main) | NVIDIA | in the 120M model | | | already in the live stage | prior |

### 3.3 Text to speech

| Model | Maker; licence (use, credit) | Size; memory | Voices | Quality | Speed | Runtime; Android | Source |
| --- | --- | --- | --- | --- | --- | --- | --- |
| **Kokoro 82M** (on main) | hexgrad / FluidInference; **Apache-2.0, read today** (repo tag, last modified 2026-11-08); give notice of the licence | ~350 MB Core ML; 676 MB peak on iPhone 16 Pro (maker soniqo, prior) | **54 `voices/*.json` in the Core ML repo (read today)**; we show 5; languages in the pack are not counted here | leaderboard English WER 1.46 (secondary article, prior) | RTF 0.08 iPhone 16 Pro (maker soniqo); Galaxy S23 CPU 0.53, S10 1.1 (maker, 3P VoicePing; prior) | FluidAudio Core ML; ONNX for Android | https://huggingface.co/FluidInference/kokoro-82m-coreml |
| **Phonon 31M** (Gradium; on main) | Gradium; **Apache-2.0, read today** (31M English only; the 40M variants are CC BY-NC, prior) | 55 MB GGUF; the Core ML pack we pin is 173 MB with all voices | **103 in the pack**: 25 English, 12 characters, and French, German, Spanish, Portuguese catalogue voices, but "speaks English only" (read today, FluidInference card) | none published | ~20x real time and ~20 ms first audio on an M5 Pro (maker, read today); iPhone: none | Core ML; GGUF (`model.q4k.gguf`, `model.q8.gguf`, read today) | https://huggingface.co/gradium/phonon-31m-en |
| **Pocket TTS** (Kyutai; on main) | Kyutai; **CC-BY-4.0, "Attribution to Kyutai is required"** (read today); screen must credit | base files ~600 MB (cond 200, flowlm 200, flow decoder 190, mimi 11); 100M English pack | base 4 voices; **27 in the English 100M pack**; small v4 packs 18M / 7M / 2M at 109 / 86 / 58 MB RSS (read today) | leaderboard WER 1.62 (secondary) | small models 24x to 51x real time on an M5 Pro (maker, read today). **Card: "chunk text at sentence boundaries to 50 tokens or fewer"; several voices never end on a whole paragraph** (read today) | Core ML; the same card warns of a hiss in pauses on the all-ANE path | https://huggingface.co/FluidInference/pocket-tts-coreml |
| **Supertonic 3** | Supertone; **OpenRAIL tag, read today** (a use-restriction licence; "code MIT" per prior); restrictions not read | repo 411.5 MB (read today); 956 MB peak on iPhone 16 Pro (maker soniqo, prior) | 10 voice styles F1 to F5, M1 to M5 (read today); 31 languages (prior) | leaderboard English WER 1.54 (secondary) | RTF 0.15 iPhone 16 Pro (maker) | ONNX, Core ML | https://huggingface.co/Supertone/supertonic-3 |
| **Kitten TTS 2 mini / pico** | KittenML; Apache-2.0 | mini int8 82.7 MB; pico 4 to 8 MB per language | mini 118 voices, 16 languages (prior) | none stated | none on a phone (a competitor says mini is 20x slower than its own) | ONNX | prior |
| **kale-tts-1-nano** | Kale Computer; Apache-2.0 | 14.8 MB int8 | 10 synthetic | 5.82% "unclear words" (self) | 44x real time on a Galaxy Tab S10 Lite (self) | ONNX Runtime | prior; weeks old, one author |
| **Piper** (sherpa-onnx) | MIT engine; per-voice licences not checked | 21 MB (int8, amy-medium) | many | not stated | ~9x on an RK3588 (3P) | sherpa-onnx CLI exists | prior |
| **Reverb-TTS, OmniVoice, Cilla-mini, F5-TTS** | **weights non-commercial** | | | | | | prior: out |

### 3.4 Speaker verification (voice print)

| Model | Maker; licence | Size | Quality | Runtime | Source |
| --- | --- | --- | --- | --- | --- |
| **WeSpeaker ResNet34-LM** (on main) | WeSpeaker; CC-BY-4.0 (credit on screen: the Settings voice-print row already shows `voicep.credit`) | 7.95 MB Core ML, 256-d | cosine >= 0.997 against PyTorch; no phone EER (prior) | Core ML | prior |
| **CAM++ Core ML** (aufklarer) | Apache-2.0 | ~14 MB, 192-d | no EER (prior) | Core ML | prior |
| **ReDimNet2-B6 Core ML** (aufklarer) | **MIT, read today** | repo 54.9 MB total (read today); the card says 28.6 MiB (prior; the two disagree) | paper 0.287% EER on Vox1-O (maker); the Core ML build has none (prior) | Core ML, 13.6 ms per 6 s on an M5 Pro | https://huggingface.co/aufklarer/ReDimNet2-B6-CoreML |
| **ECAPA-TDNN ExecuTorch** | Apache-2.0 | 42.5 to 83.9 MB | none | ExecuTorch (iPhone and Android) | prior |

None publishes an equal-error rate for a phone microphone with the radio playing; that is a test we must run
(enrol 10 s, a second voice at the same loudness, radio bleed).

### 3.5 Small language models (the quick decision and the local responder)

| Model | Maker; licence | Size on disk; resident | Speed | Runtime | Source |
| --- | --- | --- | --- | --- | --- |
| **Qwen3-0.6B** Q8_0 / Q4_K_M (on main) | Alibaba; Apache-2.0 | 639 / 397 MB | iPhone 17 Pro 121 to 193 tok/s by runtime (3P, prior); A16 ours: unmeasured | llama.cpp GGUF (on main) | KnownModels |
| **Qwen3.5-0.8B / 2B** (on main) | Alibaba; Apache-2.0 | 533 MB Q4 / 1.28 GB Q4 | none on a phone | llama.cpp; "whether b12259 runs it is what the check says" | KnownModels |
| **LFM3-1.7B** (on main, seven quants) | Liquid AI; LFM Open License v1.0, commercial use at or above US$10M revenue needs a licence | 1.05 GB Q4_0 to 1.85 GB Q8_0 | 43.6 tok/s at 1.9 GB (maker chart, prior); Mac 36 tok/s Q4_0 (ours) | llama.cpp | KnownModels |
| **LFM2.5-350M** | Liquid AI; LFM Open License | Q4_K_M 229 MB (prior) | none | GGUF, ONNX, MLX, LiteRT exist (prior) | prior |
| **FunctionGemma 270M** | Google; **licence not read** | 236 MB, 128 tok/s on an iPhone 16 Pro, Core ML ANE (maker soniqo, prior) | | Core ML (speech-swift) | prior |
| **Gemma 3 270M / 1B** | Google; Gemma terms, not read | 253 MB / 806 MB Q4_K_M (prior) | none | GGUF, LiteRT | prior |
| **Gemma 4 E2B 4-bit** | Google; Apache-2.0 (prior) | "5.1B with embeddings" (prior) | iPhone 17 Pro, ANE 33 then 22 tok/s over 10 min (3P blog, prior) | Core ML, MLX, LiteRT-LM | not a 6 GB model |
| **LFM3-Omni-2B Q8_0 text path** (responder-omni) | Liquid AI; LFM Open License | 1.85 GB | Mac 19.4 tok/s writes, 697 ms first token (ours) | llama.cpp | omni-ears |

### 3.6 On-phone voice quality: what nobody has measured

No listening test (MOS or ABX) of any small on-device voice exists that I could open; the leaderboard WER figures above
measure intelligibility on an H200, not pleasantness. The only independent phone TTS table is VoicePing's (2026-02-15, a
Galaxy S10 and an iPad Pro A12X, no quality test). Any "quality" ranking of voices is the owner's ear, which is why the
page plays a sample on tap.

### 3.7 Speech to speech

LFM3-Omni-2B does audio in and 24 kHz audio out in one model, with two voices (female, male). We measured it
(ON-DEVICE-MODELS section 14 on omni-ears): as ears it is worse than Fluid on all sets and invents text on silence; it
answers a spoken question directly (19 of 20 clean, hand read); first audio 0.25 to 0.33 s from text; 2.4 GB resident at
Q8_0. LFM3-Audio-2B (FluidAudio, 2.9 GB Core ML) is the same family. Moshi and PersonaPlex are Mac models (prior). **Nothing
to offer on a 6 GB phone.** For an 8 GB phone it is a possible later responder row, not a listener. Not recommended now.

## 4. Recommendations: what to add first

Best value first. "Pins" = the model list's `revision`, files with size and SHA-256, read at a commit. All credits go in
the list entry's `credit` and onto the screen (section 5).

| # | Add | Type | Why | Cost | Needs |
| --- | --- | --- | --- | --- | --- |
| 1 | **More Kokoro voices** (up to the 54 files the pack publishes; start with the English ones, 10 to 15) | voice | No new download: the model we already fetch holds them. Apache-2.0. Biggest visible gain for the least work. | rows only | `VoiceOption` rows with real names and blurbs (read from the pack's own `voices_catalog`/names, not invented); sample line; Apache notice on the credits screen; the voice-card list grouped by model |
| 2 | **More Phonon voices** (25 English of the 103; we show 3) | voice | Same: no new download (the 173 MB pack already holds all 103). Apache-2.0. | rows only | as 1; the pack's own names. Only the English-speaking ones, because the model speaks English only |
| 3 | **Smart Turn v3.2** | end of utterance | 16.8 MB, BSD-2-Clause, no screen credit; the one piece of Mike's pipeline the survey calls cheap. A better stop-talking decision is the hands-free gap the page can show | a new role and ~150 lines to fit `HandsFree`'s rule, a day (prior estimate); A16 timing unmeasured | **a new `ConfigRole` (say `turn`)** and a model-list job; native stage; its false positive rate (8.48% on its own set) is the test, not the card |
| 4 | **Fluid Speech Pro** (voice-pro) | hearing you | Already built; recommended from 8 GB phones | nothing new | wait for onboard-consent; per-file manifest; first phone run decides the 7 GB line |
| 5 | **Parakeet TDT 0.6B v3** (25 European languages) | hearing you, 6 GB phones | Mini is English only and Pro needs 8 GB; v3 is the multilingual recogniser a 6 GB phone can hold. NVIDIA base | read the size and files at a commit; **resolve the card's licence contradiction against NVIDIA's card first**; CC BY credit to NVIDIA if CC BY | model-list entry only, if the stage `ParakeetStages` loads v3; else a stage |
| 6 | **LFM2.5-350M Q4_K_M** | quick decision, 6 GB | 229 MB (prior), same llama.cpp path, so a list entry; the cheap alternative to Qwen3-0.6B for the decision job | pin read; no phone speed yet; LFM Open License text on the screen | list entry (GGUF) |
| 7 | **Pocket TTS English 100M pack, 27 voices**, with the 50-token chunking rule | voice | A distinct voice style; CC BY credit to Kyutai | the card's chunk warning is a real defect for long reading: a sentence splitter exists for Phonon, check it | credit on screen; chunker test |
| 8 | **ReDimNet2-B6** as an alternative voice-print model, after an equal-error test of ours | knowing your voice | MIT, small, strong paper figure | **do not list it until our own test beats WeSpeaker on a phone recording** | test first |

Not now: Nemotron streaming (the card's licence contradicts itself; ~10% at 160 ms and no iPhone figure; 564 MB), Supertonic 3
(OpenRAIL restrictions unread; 956 MB peak), Moonshine (no Core ML path read; the Android case later), LFM3-Omni as ears
(measured worse), Reverb/OmniVoice/F5 (non-commercial), TEN VAD (licence conditions unread), Kitten and kale (weeks old, one
author).

**Android.** Core ML packs do not load on Android. The model list entry needs a `platform`/`format` field (a Core ML entry
for iPhone, an ONNX or LiteRT or GGUF entry for Android, same job and name), and the recommendation rule keys on it. Not
settled by this page: the Android runtime's choice (sherpa-onnx, ONNX Runtime, LiteRT). Entries 1, 2, 6 have a GGUF or ONNX
form; 3 has an 8 MB ONNX; 4 has ONNX/GGUF/MLX builds by the maker; 5 has an ONNX port; Pocket and Kokoro have ONNX.

## 5. What the page shows, per type

One page, `settings:voice`, built from the model list through the native side, with these parts. Everything is Exact; nothing
below adds Swift logic.

1. **Per job a section** (Hearing you, Knowing when you speak, Knowing when you have finished, The voice, Quick decisions,
   Answering, Knowing your voice): the *current choice* with its name, size and state (On this phone, Not downloaded,
   Downloading n%, Failed with the reason); the *alternatives* as rows with name, size, the plain one-line reason, a "Recommended for
   this phone" tag from the list's device rule, and a pick control.
2. **A download button per group**, the consented press of onboard-consent (the agreement is per group; Wi-Fi and mobile-data
   rules stay). No download starts from picking a row.
3. **A Delete control per downloaded model** that frees the space (`ModelStore.deleteBundle` exists; no screen calls it) and says
   what it frees and what the radio uses instead until the model is back. The one native call. A model that another job also
   uses (Qwen3-0.6B as decision and responder) says so before it is removed.
4. **Credit and licence line per model**, from the list entry: the maker, the licence name, a link, and the credit text the
   licence asks for (CC BY 4.0 for Fluid Speech, Parakeet, Pocket TTS, WeSpeaker). CC BY 4.0 section 3(a) makes the credit a
   release blocker, as `KnownModels` says; on onboard-consent it already shows on the Downloads page, so the Settings page uses
   that component. A single "Models and licences" screen under About lists every model in the list, downloaded or not.
5. **Voices**: the voice cards, grouped by model (Kokoro, Phonon, Pocket TTS, This phone's voice), tap to hear a sample, the
   chosen one marked; a voice whose model is not on the phone shows the model's size and offers the group's download rather than
   fetching in the middle of a sentence (the existing `VoiceChoice` rule).
6. **Facts that are measured on this phone** (peak memory after load, first sound, the check's result) once setup's check has
   run; never a maker's number presented as ours.

## 6. Native versus Exact

| Part | Where | Why |
| --- | --- | --- |
| The page, the list, picks, consent, credits, sizes, the recommendation display, delete button and its confirmation, the "Models and licences" screen | **Exact** (`settings.contract`, `downloads.ts`, `offers.ts`) | everything but voice and the platform |
| The model list: entries, device rules, job names, platform field | **data** (served or built in), read by Exact through the existing native read | "recommendations per device can be variable for every model type" (owner, in `ModelList.swift`) |
| Fetching, hashing and moving files | native store (`ModelStore`), already; one per-file manifest on onboard-consent | disk and SHA-256, the platform's file and network API |
| Loading and running a recogniser, a voice, a detector, a speaker model | **native** (Swift on iPhone, Kotlin on Android) | the audio session, the Neural Engine, the runtime |
| A new end-of-utterance stage (Smart Turn) | native stage plus a list job | runs on the audio path |
| `ConfigRole` for the new job | native enum and list `job` string; Exact reads the string | one place |
| Deleting a model's files | native one call, Exact button | files |

A new model of an existing job in an existing runtime (items 1, 2, 5 if v3 loads, 6) is a list entry and, for a voice, a
`VoiceOption` row, and no page change. Only a new job (item 3) or a new runtime (none in the first eight) touches code.

## 7. What can be done before onboard-consent lands, and what must wait

**Can be done now, on main, green or yellow:**
- Read and pin the files for items 3, 5, 6 and 7 at a commit (size and SHA-256 from the Hugging Face API at that commit, as
  `KnownModels` does) and write them into the list as data; the *write* is orange (behavior) but changes nothing a phone
  runs until a stage reads it.
- Read the voice names and one-line blurbs of the Kokoro and Phonon voices from the packs and write the `VoiceOption` rows
  (orange; a voice not downloaded stays gated by the existing `VoiceChoice` readiness).
- Resolve the two licence questions that block entries (Parakeet v3, Nemotron; read NVIDIA's cards), and read the Supertonic
  OpenRAIL text and the TEN VAD conditions, and record the results in this page (green).
- Add the roles and job names to the list's vocabulary and test that a served unknown job is skipped (already a test).
- Write the failure list and tests for the page's per-job rows from the list (yellow, in Exact, on the Mac; the Exact loop
  that needs no native build).

**Must wait for onboard-consent:**
- The Settings page's real downloads, picks and consent (that lane owns consent, the per-file manifest written after a
  consented fetch and a first load, and `settings:downloads`). A model's "On this phone" must mean that manifest, not main's
  flag.
- Any model that `onboard-consent` withholds until proved on a phone: "library models without a local-only load withheld".
  A new FluidAudio-loaded model (Parakeet v3, a Core ML Kokoro voice pack) is in that class until a phone proves a local-only
  load.
- Fluid Speech Pro (held on voice-pro until onboard-consent).
- The delete control, because its "what it frees" and its readiness reset belong to that manifest.

**Must wait for a phone or the owner, not for a lane:** any speed, memory or heat claim (none is ours below Silero and
Parakeet EOU); the recommendation lines (7 GB for Pro is a rule from file sizes); equal-error tests for speaker models; a
listening test of the added voices by the owner.

## 8. Open questions for the owner

1. Which voices do you want: Kokoro's English ones only, or its other languages too? (The page could grow a language line
   for each.)
2. Is Parakeet TDT v3 (25 European languages) wanted for 6 GB phones, given Pro covers them on 8 GB? Or English only for
   now?
3. Do you want Smart Turn v3.2 as an offered end-of-utterance model with a new Settings section, or kept inside the hands-free
   rule with no choice shown?
4. Do you accept the OpenRAIL (Supertonic) and "Gemma terms" (FunctionGemma, Gemma) licences in principle, or are they out
   until you read them?
5. Should Delete be shown for every model, including the one the radio needs to work (it falls back to the iPhone voice and
   Apple's recogniser until restored)?
6. Android: which runtime, and does it need its own page rows now, or only a `platform` field in the list?

## 9. What I could not verify

- Any iPhone 14 Pro Max or Android figure for a recogniser, voice or detector other than Silero and Parakeet EOU (ours).
- That Parakeet TDT v3's Core ML size, files and licence; the card contradicts itself and gives no sizes.
- Smart Turn's accuracy on our audio (two cards give 94.31% and 92.90%).
- The number of English voices among Kokoro's 54.
- Supertonic's OpenRAIL clauses, TEN VAD's conditions, FunctionGemma's and Gemma's terms, the LFM Open License's "commercial"
  boundary in our case.
- Moonshine v2 phone numbers; every ONNX Android build above was read from a card, none was run.
- The prior research was read through a summarising fetch tool and is marked prior; re-read a figure before relying on it.
