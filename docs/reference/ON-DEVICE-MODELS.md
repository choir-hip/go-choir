# On-device models (Autoradio phone app)

Reference page (convention: [README.md](README.md)). Compiled 2026-11-10
from three design notes: [turns](../design/autoradio-turns-2026-11-10.md),
[lab](../design/autoradio-lab-2026-11-10.md),
[voice modes](../design/autoradio-voice-modes-2026-11-10.md). No web fetch
was made for this page, so every figure below is relayed from those notes'
own research of 2026-11-10 and is **relayed-unverified** unless it says
otherwise. **Nothing has been run by us on any phone as of 2026-11-10; every
candidate is untested in our lab.** A cell that says "not recorded" means the
notes do not give the fact; do not fill it from memory, fetch and cite it.

Qualifiers used: **vendor** (the maker's own figure), **independent**
(third-party benchmark), **unmeasured** (no figure exists), **ours** (we
measured it; none yet).

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
