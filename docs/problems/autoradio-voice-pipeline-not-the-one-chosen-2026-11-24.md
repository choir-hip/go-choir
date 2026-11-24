# Autoradio's voice pipeline is not the one the owner chose (2026-11-24)

Read-only investigation from git history, goal files, design notes and code at
`origin/main` `2cc1b348d`. Nothing was built or run on a phone or a Mac. Mike's
files were read at `/Users/wiz/Dev/mike` (checkout head `66b5b7f0`, 2026-10-31);
its figures are marked **(Mike)** and were not re-measured here. "K" is
`mobile/AutoradioKit/Sources/AutoradioKit`, "M" is
`mobile/AutoradioKit/Sources/AutoradioModels`.

## Problem

The owner, 2026-11-24: "didnt we run experiments on this? i thought we had
selected this model weeks/months ago... we need to investigate what happened to
the audio pipeline, why it is so underdeveloped." Earlier: "The speech
recognizer isn't supposed to be Apple's! ... Fluid speech mini was my request
when we built this" (`docs/definitions/choir-voice-pipeline-2026-11-23.md`,
owner_rulings); and on 2026-11-24 "we are only using that currently because of
a coding bug. (we shuold be using fluid speech mini etc)"
(`docs/definitions/choir-shipping-the-app-2026-11-23.md:150`).

The model is Fluid Speech Mini
(`https://huggingface.co/FluidVoice-Labs/fluid-speech-mini-coreml`). On the
phone today the recogniser is Apple's, speech start and end is a loudness
detector, the end of a turn is one second of silence, and nothing checks whose
voice it is.

## Evidence

### 1. Timeline

| Date | What | Source |
| --- | --- | --- |
| 2026-10-21 to 27 | In Mike, Fluid Speech Mini is chosen and measured; upstream FluidAudio 0.17.7 is chosen over a fork whose long-audio path dropped spans. | **(Mike)** `docs/fluid-speech-mini-iphone.md`, "Spike result" |
| 2026-10-31 | The resource-fabric design names it as "Speech in", with Silero, Parakeet EOU, Smart Turn, a final Fluid Speech Mini pass, Phonon clause speech and speaker checks as what Mike already runs; notes CC BY 4.0 needs credit. Owner: "I am using all of the voice modes on device". | `45c2cb60f`; `docs/design/resource-fabric-2026-10-31.md:826-880` |
| 2026-11-09 | The mission handoff makes lane V "Port Mike's recognition, turn-taking, echo cancellation and AI voice as a Swift module"; models download at onboarding, "start with whatever Mike runs today". | `docs/design/autoradio-mission-handoff-2026-11-09.md:142, 233` |
| 2026-11-10 | Pass 1 definition: "Mike speech code is mine. Lane V ports it." "Voice comes first." | `docs/definitions/choir-autoradio-pass1-2026-11-10.md:256-266`; `0802632dd` |
| 2026-11-10 | The same day the plan becomes a lab: "stages behind interfaces, runtime configuration". Its transcription row lists Apple's recogniser, Parakeet, LFM3-Audio live ASR and a hosted service; Fluid Speech Mini is not in it. Wave B lane M1 lists "Parakeet, Silero VAD, Smart Turn, speaker check, LFM3-Audio". The lane that was to port Mike's loop is replaced by lanes that build stages from FluidAudio. | `08c067089`; `docs/design/autoradio-lab-2026-11-10.md:63-65`; pass1 line 384 |
| 2026-11-10 | `5cf2db628`: Silero, Parakeet, Kokoro, PocketTTS, LFM3-Audio, Qwen3 as stages. | git log |
| 2026-11-11 05:47 | First app commit `29e01354f` writes the listener configuration: `button` addressing, `energy-vad`, speaker check `none`, `apple-speech`, `avspeech`; comment "the best Apple stages for the rest". Its body names a Gemini review of the diff and says "nothing has run on a phone". | `git show 29e01354f:.../Choir/ChoirApp.swift` lines 8-20 |
| 2026-11-15 morning | Owner's first phone review: "The voice interview does not work at all. The hands-free voice work from Mike is not in the app. Too many versions were attempted. Go all in on hands-free voice as it works in Mike." | `docs/problems/autoradio-owner-phone-review-2026-11-15.md:33-35` (`6263d0b25`) |
| 2026-11-15 14:23 | Investigation traces Mike's loop (nine models, 21 files) and orders slices; slice 2 is "Mike's loop adopted whole; the other voice versions deleted". It states that lane V "as written was not built". | `62383b472`; `docs/evidence/autoradio-phone-review-investigation-2026-11-15.md` |
| 2026-11-15 | Owner rulings: other versions deprecated once Mike's loop is in; models under 100 MB may be bundled, larger ones download; then "four existing pieces return after hands-free works" and "be practical; the happy path is the main thing". | `docs/problems/autoradio-owner-phone-review-2026-11-15.md:73-100` |
| 2026-11-15 14:45 | Design of the voice engine (choice A: Mike's brain with the app's stages around it). Slice 1 keeps "the existing default" recogniser "so a turn works first"; Fluid Speech Mini becomes the default final pass in **slice 4**, the last, filed under dictation. This file is only on `origin/lane/autoradio-voice-engine` (one commit, `09a047f5b`); it is not an ancestor of main. | `git show 09a047f5b:docs/design/autoradio-voice-engine-2026-11-15.md` lines 85, 352-450 |
| 2026-11-15 15:06 | 21 minutes later `f83548c47` lands "hands-free voice turns": Mike's `TurnPolicy.swift` unchanged, SpeakerFusion, SpeakerProfile, VoiceContract, VoicePreparation, driven by the app's own energy VAD and Apple recogniser, "no Smart Turn model yet" (`HandsFree.swift:7, 56-57, 144, 260`). Not the design just written (no brain stage, no EOU stream, no Smart Turn). Commit body: only a `Ring-Expedite` line. Addressing becomes `open`; recogniser, VAD, speaker check unchanged. | `f83548c47` |
| 2026-11-15 | `ada99810a`, `4bab09a5f`: neural voice default, Phonon 31M added as a voice (the speaking side got attention the same day). | git log |
| 2026-11-16/17 | Voice work is about sessions and echo: one voice at a time, full-band capture (`cb6def5c9`, lane `voice-fullband`, never merged, never tried by the owner). | `docs/changes.md:35`; M6 definition |
| 2026-11-18 | `33cf20826` joins 162 commits of the phone work to main as one (SpeakerGate, WeSpeakerEmbedder, VoiceChoice enter main). The Exact cutover goal opens: "Native code is the voice and nothing else." | `docs/definitions/choir-autoradio-exact-cutover-2026-11-18.md:48` |
| 2026-11-19 to 23 | The cutover train: `a8b603d22` (11-19), `4024ae23d` (11-21), `0a397a684` (11-23); then Android, the paper, App Store work. No commit in this period touches the recogniser, VAD or end-of-turn defaults. | git log |
| 2026-11-22 | Android model bench: Parakeet TDT 0.6B v2, Whisper tiny, Moonshine, a zipformer, on a moto g stylus. Fluid Speech Mini is not in it; "Parakeet EOU ... Silero VAD cost ... not started". | `docs/evidence/model-bench-2026-11-22/README.md:160-165` |
| 2026-11-23 | Owner reads what runs: "How did the voice pipeline not get built?" The letter of the day and mission M6 record that nothing is built. | `7866f4027`, `eff8f254a`, `docs/reports/five-days-and-where-we-stand-2026-11-23.md:41-55` |

### 2. Each piece of Mike's loop in Autoradio

"Default" means reached by `ChoirApp.listenerConfiguration`
(`K/Choir/ChoirApp.swift:15-24`) with no setup choice. "Phone" means a run on
the owner's phone with evidence.

| Mike's piece | In the code | In the default | Tested | Run on the phone |
| --- | --- | --- | --- | --- |
| Silero VAD (256 ms hops) | Yes: `M/SileroVAD.swift` stage `silero-vad`, 2026-11-11 | No: `energy-vad` (`ChoirApp.swift:19`). It is in setup's ladder (`K/VoiceSetup.swift:109`). Fed to the policy as 0.9 or 0, not a probability (`HandsFree.swift:5`) | Setup and model-list tests; a Mac replay command (`mobile/README.md:294`) | Once: load 93 ms, 0.6 ms per hop, 106 MB, A16, "before the 2026-11-15 ruling" (`mobile/README.md:301`) |
| Parakeet EOU live words and `<EOU>` | Partly: `M/ParakeetStages.swift` stage `parakeet-eou` (a transcriber). Mike's `EouStream.swift` (re-armed on each `<EOU>`) is not ported | No (Apple's recogniser). In the ladder | Mac replay | Once: first load 34 s, RTF 0.035, 149 MB (`mobile/README.md:302`) |
| Smart Turn v3.2 | No model code, no download row. Only the policy's input case exists (`Voice/TurnPolicy.swift:19, 60`); `HandsFree.swift:144` sets `smartTurnAvailable = false` | No | Policy tests with a fake answer | Never |
| `TurnPolicy` | Yes, unchanged, 520 lines: `K/Voice/TurnPolicy.swift` | Yes, driven by `HandsFree.swift` with `fallbackSilenceMs = 1000` (`:56-57`) | Yes: `TurnPolicyTests` 788 lines, `HandsFreeTests` | `docs/current-architecture.md:166` says hands-free was "never spoken to" on a device; the 2026-11-17 goal says the owner "confirmed voice works" after a session fix (`choir-autoradio-overnight-2026-11-17.md:37`). Not reconciled here |
| Fluid Speech Mini final transcript | **No.** No file, stage, model row or test. The words appear in `docs` only (`resource-fabric`, M6, shipping, the investigation) | No | No | Never, here. Only in Mike |
| Speaker embeddings, learned voice | Ported: `Voice/SpeakerProfile`, `SpeakerFusion`, `SpeakerGate`, `WeSpeakerEmbedder` (2026-11-15/18) | Not as a stage: `speakerCheck` is `none`. `HandsFree` gathers samples and judges only when the gate has already learned (`:119, 131, 292`) | `SpeakerGateTests` | Never shown learned; the 2026-11-23 letter says "I do not believe it has" |
| Phonon voice, clause queue, played-sample clock | Phonon: stage registered (`M/VoiceStages.swift`). `ClauseQueue` and the played-sample clock: no file | Kokoro is the default voice (`VoiceChoiceTests.swift:74`); Phonon is selectable | Voice choice tests | Owner heard a neural voice speak after switching in Settings (2026-11-15 problem record); Phonon on the phone not recorded |
| Pinned downloads, checksums | Different mechanism: `ModelStore` (SHA-256, resumable) for GGUF files; FluidAudio fetches its own Core ML models unpinned (`M/ModelHub.swift:6`). Mike's `VoiceModels.swift` pins repository, revision and per-file size and SHA-256; not ported | Not for the Core ML models | `VoiceModelsTests` | Download stalled under FluidAudio's downloader once (`mobile/README.md:312`) |
| One voice-processing engine | Yes: `LiveAudio` is one `AVAudioEngine`; default `audioPolicy = .alwaysTalking` (`ChoirApp.swift:22`) | Yes, and that is the call-style sound the owner objected to | `AudioSessionHoldTests` and others | Yes; the owner's complaints are the evidence |

What persists: only the voice choice (`LabRuntime.swift:360`). The other picks of
voice setup are held in memory; nothing reads them at launch (M6, "what is true
now"; the same defect as `autoradio-system-voice-default-2026-11-15.md`, fixed
for the voice only).

### 3. Why the default never changed, and what was measured

- The three lines (`speechBounds`, `transcriber`, `speakerCheck`) are
  unchanged since `29e01354f` (2026-11-11). `git log -L15,24:ChoirApp.swift`
  shows later commits touching the file for other reasons (`f83548c47` changed
  `button` to `open` and added `alwaysTalking`). I found no commit message,
  review note, goal file or ruling that says "keep Apple's recogniser", and none
  that says "replace it". No review recorded in any commit body or ledger
  compared the default with the owner's words.
- Experiments on recognisers in this repo: `docs/reference/ON-DEVICE-MODELS.md:53`
  records Apple's recogniser as "unmeasured by us" and Parakeet "unmeasured";
  `mobile/README.md:302-303` has Mac and one phone figure for Parakeet EOU and
  Mac only for TDT; `docs/evidence/model-bench-2026-11-22` (Android, other
  models). `scripts/proofs` and `mobile/fixtures` hold no recogniser comparison
  (fixtures are turn scripts). The calibration ledger has no claim about which
  recogniser to use; C35 and C36 were about LFM3-Audio (different model). **Fluid
  Speech Mini was never measured in this repository, on a phone or a Mac.**
  The only measurements are Mike's.
- Mike's figures, copied, **(Mike)**, `docs/fluid-speech-mini-iphone.md` and
  `docs/voice-chat.md`:
  - 253 MB Core ML build (24 files, 252,730,493 B by Mike's manifest
    `mobile/scripts/speech-model.manifest.json`, revision
    `d81c3160dfe5a5e1f8f7267164a07d30617a3a1a`); a distilled student of Parakeet
    TDT 0.6B v3; English only; needs iOS 18; Neural Engine; "about 277 MB of RAM
    per 15 s window"; batch, not streaming, 16 kHz mono, 15 s windows.
  - Spike on a Mac, Core ML, 2026-10-21: 1.51% WER on isolated clips against
    Parakeet 0.6B v3's 1.92%; 2.05% WER and no dropped spans over a 31-minute
    LibriSpeech stream through upstream FluidAudio 0.17.7; the
    `altic-dev/FluidAudio` fork gave 22.4% there (twelve spans dropped). Output
    carries punctuation and capitals.
  - `voice-chat.md`: "4.54% average WER"; "10.9% WER against Parakeet v3's 20.4%
    with background voices, self-reported"; "about 25 ms per 15 s window on a
    Mac, to be measured on the iPhone". Mike has no iPhone latency figure for it
    in these two files; the owner uses the loop daily on the phone (2026-10-31).
  - Loads under upstream only if the joint file is presented as
    `JointDecisionv3.mlmodelc` (the repo ships `JointDecision.mlmodelc`).
  - Licence CC BY 4.0: credit "Fluid Speech Mini by FluidVoice", link the
    licence, keep LICENSE and NOTICE with the files.
  - Our pin is FluidAudio 0.18.3 (`mobile/AutoradioKit/Package.swift:18`), not
    the 0.17.7 Mike's spike used. Whether 0.18.3 loads it is **not verified**.

### 4. Cause (see Belief)

Evidence for each candidate:

- **A dropped requirement?** No. The words reached the plans three times:
  handoff lane V (`11-09`), pass 1 ("Lane V ports it", `11-10`), the
  investigation and the voice-engine design (`11-15`, Fluid Speech Mini named as
  the default final pass).
- **A slice order never scheduled?** Yes. The design put the named model in the
  last slice and called it dictation; slice 1 kept "the existing default". No
  goal file or ledger entry schedules slices 0 to 4 after 2026-11-15; no
  definition on main cites the investigation or the design
  (`grep` for the investigation's name finds no goal file; M6's receipts cite the
  design at a path that does not exist on main).
- **A silent substitution?** Partly. The first app commit wrote Apple's
  recogniser as "the best Apple stages" with no flag that the owner's choice was
  absent; `f83548c47` then built a smaller thing than the design written 21
  minutes before it, with no note.
- **A review that did not check against the owner's words?** Not shown either
  way; no seat's findings on these commits cite the owner's request.
- **Something else?** The 2026-11-10 lab redesign turned "port a tested loop" into
  "build stages and let setup pick"; Apple's recogniser became "the baseline"
  and the configurator was thought to be the way better models would arrive,
  while its picks were never persisted. On 2026-11-15 the owner's own ruling
  ("be practical; the happy path") licensed the interim. Then 2026-11-18 to 23
  went to the Exact cutover.

## Belief

The cause is a slice order that put the named model last, plus a design that
never reached main, plus an interim ("`open` hands-free on the existing stages,
until the rest returns") that no goal file turned into a dated slice. The
requirement was never dropped; it was never scheduled. The 2026-11-18 cutover
and the paper then took the attention, and the interim default stood for nine
days with its own doc line ("Not in the default path ... registered or sketched
only", `docs/current-architecture.md:155`) saying so.

Confidence: high that Fluid Speech Mini was never built or measured here; high
that the 11-15 design is not on main; medium on the ordering of attention (from
commit dates and goal files, not from the lead's reasons).

## Remaining error

- Why `f83548c47` did not follow the design: no trace of the builder's reason.
- Whether any lane other than those listed holds Fluid Speech Mini work: I
  searched `git log --all -i --grep fluid` and the docs; found none.
- Which of slices 0 and 1 of the 11-15 investigation landed (the system-voice
  fix, the Listen control): not checked.
- Whether FluidAudio 0.18.3 loads the Core ML build, and what the phone costs in
  load time and memory: no run.
- Whether hands-free has been used on the phone (documents disagree, row 4).
- Android: Mike's ONNX build and the 2026-11-22 bench do not include it.

## Next: closing the gap

Order is by what the owner named first, then what depends on it. Sizes are
rough. "Mac" means Swift package tests or `autoradio-replay` with fixture audio
through Core ML, under the one build lock; the phone steps are the owner's.

0. **Put the record and a reference page on main (green).** Merge the design
   (`09a047f5b`) or supersede it; add a Fluid Speech Mini row to
   `docs/reference/ON-DEVICE-MODELS.md` (CLAUDE.md: read and write the page
   before using a new dependency); put the 2026-11-15 slice order in M6 with
   dates. Small. Safe tonight.
1. **V0, keep the picks (orange).** Persist every stage choice and read them at
   launch; delete the fixed list's role as the default. Files: `LabRuntime`,
   `VoiceSetup`, `ChoirApp`. Small, about half a day. Mac: tests. Phone: relaunch
   and read the running stages. Safe tonight only with a device check.
2. **V2, Fluid Speech Mini as a `Transcriber` stage (orange).** Port from Mike
   `FluidSpeechMini.swift` (242 lines; `modelDirectory` reads the bundle, change
   to the downloaded directory) and `VoiceModels.swift` (307 lines; pinned
   repository, revision, size and SHA-256 per file). Model:
   `FluidVoice-Labs/fluid-speech-mini-coreml` @ `d81c3160`, 24 files,
   252,730,493 B (**Mike**); `JointDecisionv3` rename; credit line in Settings;
   check FluidAudio 0.18.3 against `AsrModels.loadLocal(version: .v3)`. Medium,
   about 1.5 days. Mac: a fixture wav through the stage, WER against Apple's
   recogniser and Parakeet on the same set (the first measurement of it here);
   load time and RAM on the Mac. Phone: load, memory, latency on the 14 Pro Max.
   Land as a stage and a ladder row before it is the default; make it the default
   after the phone run. Landing the stage tonight is safe; flipping the default
   is not without a phone run.
3. **V1, Silero as the speech detector (orange).** The stage exists. Feed
   `TurnPolicy` the probability, not 0.9 or 0, as Mike's `VoiceEngine` does
   (hop 256 ms, own hysteresis). Model: `FluidInference/silero-vad-coreml` @
   `b419383c`, 1,063,425 B (under 100 MB, may be bundled by the 11-15 ruling).
   Small, about a day. Mac: fixture replay through Core ML. Phone: false
   starts with a television on.
4. **V3a, Smart Turn (orange).** Port `SmartTurn.swift` (56 lines); model
   `aufklarer/Smart-Turn-v3.2-CoreML` @ `75f06971`, 16,787,529 B; set
   `smartTurnAvailable` true and answer `runSmartTurn` (`HandsFree.swift:260`).
   Small, about a day. Mac: fixture clips, pause mid-sentence against finished.
   Phone: thresholds.
5. **V3b, live words (orange).** Port `EouStream.swift` (224 lines, re-arm per
   `<EOU>`); model `FluidInference/parakeet-realtime-eou-120m-coreml` @
   `40a23f4c`, folder `320ms`, 224,238,270 B. Risk: it drives FluidAudio internals
   as of 0.17.7 (**not verified** on 0.18.3). Medium, 1 to 2 days. Mac:
   fixture partials. Phone: first-load time (34 s seen once).
6. **V5, the speaker check as a stage (orange).** Turn `speakerCheck` on with
   WeSpeaker (`FluidInference/speaker-diarization-coreml` @ `7eab74f3`,
   15,291,553 B) and CAM++ (`FluidInference/campplus-coreml` @ `321b18e2`,
   16,615,589 B), thresholds fitted on real audio. Medium. Phone only for the
   thresholds (recordings of the owner and of the radio's own voice).
7. **V4/V6, the same sound, echo, clause speech (orange).** The full-band session
   from `lane/voice-fullband`, `ClauseQueue.swift` (272 lines, with 308 lines of
   tests) and the played-sample clock (`VoiceEngine.swift:1036-1093`),
   Phonon clause speech (`PhononVoice.swift`, 401 lines; about 162.9 MB by Mike's
   constant, not confirmed). Large, several days; pure parts testable on the Mac,
   everything that matters needs the phone. Not tonight.

Safe the night before the owner's QA: item 0; the stage and ladder row of item 2
and its Mac fixture measurement (no change to what runs); item 3 and 4 only as
tested ports with the phone default untouched. Not safe: changing the default
recogniser, the audio session or the echo path without a phone run.
