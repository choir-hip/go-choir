# Autoradio: the first app is a lab (2026-11-10)

Status: design, not built. Mutation class of this document: green.
Rollback: revert. It frames the first build of the
[goal](../definitions/choir-autoradio-pass1-2026-11-10.md) and sits above
[the voice modes note](autoradio-voice-modes-2026-11-10.md) and
[the turns note](autoradio-turns-2026-11-10.md).

## The owner's statement (2026-11-10)

- "The first autoradio app should be experiments. Model experiments.
  Pipeline experiments. UX experiments. Data experiments. Scripting
  experiments. Latency experiments. We will narrow it down from there."
- "We want to make it as full-featured and realistic/complete as possible
  while also being maximally modular so we can switch things at runtime.
  This will pay dividends for debug and beyond."
- "We should try the range of new models from Liquid and more. My iPhone,
  14 Pro Max, is old enough to be the oldest supported for local stuff."

## What this means

The first app is the real radio with every stage replaceable while it
runs. It is not a set of demos. It signs in, plays a real edition from the
owner's computer, hears a real turn and gets a real revision from the
desk. What varies is which part does each job.

Three properties make it a lab.

1. **Stages with one interface each.** Every job in the path has one
   Swift protocol and several implementations behind it.
2. **A configuration is data.** A named set of choices, one per stage,
   plus their parameters. It can be changed from a lab panel in the app
   between turns, without a rebuild or a restart.
3. **Every turn is a record.** The record names the configuration, the
   time each stage took, what each stage produced, and what the listener
   did next. Experiments are compared from records, not from impressions.

## The stages

Named by job. The implementations listed are candidates from the research
so far, not a commitment.

| Stage | Job | Candidates |
| --- | --- | --- |
| Microphone and session | which input, which audio state | phone mic with full-quality playback; headset mic; voice-processing on or off |
| Addressing | is the listener talking to the radio | button; a word (text-defined spotter, trained spotter); open; hybrid window |
| Speech bounds | start, pause and end of a turn | Silero VAD; Smart Turn v3.2; Parakeet end-of-utterance |
| Speaker check | is it the listener's voice | WeSpeaker; CAM++; none |
| Transcription | words, live and final | Apple's on-device recogniser; Parakeet; LFM3-Audio live ASR; a hosted service |
| Yielding | what playback does while the listener speaks | dip after about 1 s; pause when the utterance counts or after about 10 s; pause at once |
| System 1 | the fast questions | Jev over the network; Qwen3-0.6B as a one-token scorer; LFM2.5-1.2B, LFM3-1.7B, Gemma under a grammar; Liquid d1 if it can be built; later an encoder trained on Jev's record |
| Holding | acknowledgement and holding conversation | a fixed line; a small text model into a voice; LFM3-Audio speech out |
| Voice | text to speech | Phonon; Kokoro; LFM3-Audio; the system voice; a hosted voice |
| System 2 | the revision | the desk on the owner's computer (not replaceable on the phone; its prompt and questions are the desk package's) |
| Programme | what is played | the edition document; a fixed test document for repeatable runs |

A model that covers several stages (LFM3-Audio does transcription, holding
and voice) registers one implementation per stage over one loaded model,
so it can be tried whole or mixed with others.

## Runtime switching

- A registry holds the implementations of each stage. The pipeline asks
  the registry at the start of each turn, so a change applies to the next
  turn.
- Models load and unload on demand, with their load time and memory
  recorded. Only what the current configuration needs stays loaded.
- Configurations are named and saved. The lab panel shows the current
  one, lets the owner change one stage, and offers the saved ones.
- A configuration can also be pushed from the owner's computer, so an
  agent can run a planned series while the owner only listens and talks.

## Records and replay

- **The turn record.** Configuration, stage timings (microphone to first
  word of acknowledgement, to first word of the revision), each stage's
  output, memory and thermal state, and the outcome. It is written on the
  phone and sent to the owner's computer, where the learning records
  already live.
- **Replay.** The app keeps the turn's audio and context (with the
  owner's consent; this is the owner's own phone). A kept turn can be run
  again through a different configuration, on the phone, with no one
  speaking. One hour of real use becomes a test set for every later model.
  This is the main dividend of the stage interfaces.
- **A trace view.** The last turn as a timeline on the phone: which stage
  ran, how long, what it said. This is the debugging surface.

## The kinds of experiment

| Kind | Varies | Read from |
| --- | --- | --- |
| Model | the implementation of one stage | replay over kept turns: agreement with Jev, word error, time, memory |
| Pipeline | which stages exist and in what order (speech-to-speech against transcribe, decide, speak) | end-to-end time and the owner's judgment |
| UX | addressing mode, yielding rule, acknowledgement style | live sessions; false and missed interruptions per hour |
| Data | what context goes to System 1 and the desk; what is prepared at edition build | the scored records |
| Scripting | how the edition is written for the ear; the desk's questions | listening through; skips and talk-backs |
| Latency | everything, stage by stage | the turn record |

## The oldest phone

The owner's iPhone 14 Pro Max is the floor: an A16 chip and 6 GB of
memory. Consequences to confirm on the device, not from memory:

- Apple's built-in language model needs Apple Intelligence hardware
  (iPhone 15 Pro or later), so it is very likely unavailable on this
  phone. It stays a stage implementation for newer phones.
- LFM3-Audio peaked at 1.9 GB on a Mac and built a 5.3 GB cache on first
  load. On 6 GB this is the first thing to test, and it may not fit
  beside a second model.
- Core AI exports need iOS 27. Whether this phone runs it is to check.
- Models of 0.5 to 1.2 GB (Qwen3-0.6B, LFM2.5-1.2B) and the small audio
  models are the safe set here.

What works on this phone works on every phone we will support.

## How this squares with "no switches"

The operating contract forbids runtime switches on the platform, because
they left code the owner asked for not running. The lab is the opposite
case: the owner asked for switching, the choice is visible in the app, and
every turn records which configuration ran. Two rules keep it honest.

- The lab's choices live in the app and in the configuration record. They
  are not platform flags and do not gate anything on the computer.
- Narrowing deletes. When an implementation loses, it is removed, not
  left off. The app that goes to other people has one configuration per
  supported phone and no lab panel.

## First build

Smallest thing that is already a lab:

1. Sign-in, the edition document, playback.
2. The stage protocols, the registry, the configuration and the turn
   record, with the trace view.
3. Two implementations of each stage that has a cheap second one (button
   and open addressing; Apple's recogniser and Parakeet; Jev and one
   on-device scorer; a fixed line and a small text model; two voices).
4. Kept turns and replay.

Then models are added one at a time, each as an implementation of a stage,
each measured by replay before it is tried live.

## Open

- Which runtime carries most on-device models (llama.cpp, MLX Swift, Core
  ML through FluidAudio, LiteRT-LM, Cactus). Several will coexist in the
  lab; the app that ships should carry one or two.
- App size and download: models are fetched on demand, not bundled.
- Whether replay audio may ever leave the phone. Default: no; only the
  record does.
