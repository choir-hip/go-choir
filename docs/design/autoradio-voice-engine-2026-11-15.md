# Autoradio's voice engine: hands-free voice, dictation and reading aloud (2026-11-15)

Status: design, docs only (green). Nothing was built, installed or run on a
phone or simulator. Claims are read from code or from files on this machine;
what was not read or run is marked **not verified**; everything made up is
marked **(invented)**.

Inputs: [the owner's review](../problems/autoradio-owner-phone-review-2026-11-15.md),
[the investigation](../evidence/autoradio-phone-review-investigation-2026-11-15.md),
and, read before stating anything about these dependencies,
[EXACT.md](../reference/EXACT.md), [exact/audio.md](../reference/exact/audio.md),
[exact/native-modules.md](../reference/exact/native-modules.md),
[ON-DEVICE-MODELS.md](../reference/ON-DEVICE-MODELS.md). The learning engine
definition needs only that a turn record keeps the listener's words and whether
they cut in; `TurnRecord` already does (`K/TurnEngine.swift`, `onRecord`).

Anchors are on `origin/lane/autoradio-speech-fixes` (the small fixes about to
land; this design sits on top). `K` = `mobile/AutoradioKit/Sources/AutoradioKit`,
`X` = `mobile/exact-package/apple`, `L` = `mobile/AutoradioLab`. The engine's
code comes from the Mike app's Swift speech engine,
`/Users/wiz/Dev/mike/mobile/plugins/speech/ios/Sources/MikeSpeechPlugin/`
(the owner's own code; the investigation read commit 66b5b7f05; read only
here). That is the only mention of Mike in this design.

## 0. Stance

The main thing is the happy path, working on the owner's phone: hands-free
listening in a voice session, end of turn, cutting in, no self-hearing, a
natural voice that starts fast. Owner rulings (2026-11-15) that bind it:

- Hands-free is the default; "Tap to talk" is removed; there is no button
  addressing mode. Choosing a voice or a model in the configurator is a product
  feature, not a switch; there is no off-by-default gate in front of hands-free.
- The configurator, model store and onboarding download stay live and are used.
  Fallback voices and other models stay available through them. Nothing is deleted.
- A model under 100 MB may be bundled; a larger one is downloaded in onboarding.
- Hard cutover: pre-TestFlight, one phone, no migration, no compatibility shim,
  no dual path. A slice may replace a path outright; each leaves the app working.
- Nothing is installed or launched on a physical phone without the owner's go;
  device proof is written as steps for the owner; simulator proof uses a
  simulator the builder creates with `mobile/scripts/sim-device.sh`, never the
  owner's `autoradio-lab`.

## 1. The choice: A or B, from code

**(A)** the engine's turn-taking brain adopted as the turn engine's front half,
with the app's stages as its pluggable parts. **(B)** the app's `TurnEngine`
kept whole, with the engine's policy, end-of-turn, echo, speaker check and
clause speech brought in as stages and fixes.

**Choice: A, bounded. The brain is new and fused; every part around it is a
registry stage, and the back half of `TurnEngine` is untouched.** Reasons:

1. The brain is one pure fusion. `TurnPolicy` (520 lines, 788 lines of tests)
   decides from VAD probability, live EOU words and `<EOU>`, Smart Turn, the
   speaker verdict, the text of the clause being played and the clock. The
   kit's stage contracts cannot carry that: `SpeechBounds` returns only
   `speechStarted/speechEnded` (`K/Stages.swift:182-195`), `Addressing.decide`
   takes `AddressInput` with no partial words and no answer text
   (`:206-228`), `YieldPolicy` sees only `speechSeconds` (`:241-248`). Doing B
   means widening four protocols and rewriting `TurnEngine.route`/`beginTurn`
   (`K/TurnEngine.swift:306-420`) to hold the same state the policy already
   holds, then re-proving it. A reuses tested code as it is.
2. The kit's cut is a sentence: "that sentence is heard to its end (never cut
   mid-word)" (`K/LocalSpeaker.swift`, failure 1). Cutting in at once needs a
   played-sample clock and a queue (`ClauseQueue`, 272 lines, tested), which
   the kit lacks.
3. The audio device does not need porting. `LiveAudio` is already one
   `AVAudioEngine` with voice processing on the microphone, a 16 kHz mic tap
   into `bus.mic()`, programme and voice nodes in the same mixer (so the echo
   canceller hears them), rebuild on route change, and no session calls of its
   own (`K/LiveAudio.swift:35-46, 190-240`; the session comes from
   `ExactSessionOwner`, `X/AutoradioModule.swift:8-50`, whose `.talking` claim
   is `voiceChat` + `defaultToSpeaker` + `allowBluetoothHFP`, `:21-24`, exactly
   what the engine asks for). The engine's own 460-line audio engine is
   therefore **not ported**; only its player half (80 lines) is lifted into
   `LiveAudio` (section 2.3). That removes the whole audio-session edit.

What stays pluggable (all through the registry, `K/Stages.swift`, ids in
`K/BasicStages.swift:351-373` and `AutoradioModels`):

| Seam | Happy path (default) | Still selectable through the configurator |
| --- | --- | --- |
| turn taking (new stage kind `turnTaking`, invented; `VoiceConfig` thresholds are its params) | the engine's brain | none (one brain) |
| final pass `Transcriber` (the engine feeds the utterance through `begin/append/finish`, `K/Stages.swift:230-240`) | Fluid Speech Mini (new stage id `fluid-speech-mini`, invented); in slice 1 the existing default so a turn works first | Apple recogniser, Parakeet TDT, Parakeet EOU, LFM3 |
| clause synthesis `Voice` (the engine reads the `AudioFrame` stream and resamples to 24 kHz) | Phonon (new stage id `phonon`, invented) | Kokoro, Pocket TTS, Apple synthesizer (`avspeech`) |
| `Holder`, `System1`, `Desk`, `Programme` | unchanged | unchanged |
| session | `AudioSessionOwner` | unchanged |

## 2. What the engine is

### 2.1 The twenty-one files (`wc -l` on the source tree)

| File | Lines | Role | Port |
| --- | ---: | --- | --- |
| `VoiceEngine.swift` | 1,223 | actor: progressive model load, listening loop, final pass, clause speech, 10 ms played-sample clock, `VoiceMode` status, prewarm, install | edit: audio start/stop, emit type, final pass through a `Transcriber`, clause synth through a `Voice` |
| `TurnPolicy.swift` | 520 | pure: turn end, barge-in, false start, hold and resume | unchanged, plus one input (3.2) |
| `VoiceAudioEngine.swift` | 460 | its own `AVAudioEngine`, voice processing, session calls, rebuild | **not ported**; player half lifted into `LiveAudio` (2.3) |
| `VoiceContract.swift` | 221 | `VoiceConfig`, `VoiceState`, `VoiceEvent` | unchanged |
| `ClauseQueue.swift` | 272 | pure: clause order, timeline, interruption fraction, "steps" that give way | unchanged |
| `EouStream.swift` | 224 | Parakeet EOU 120M, 320 ms: live words and `<EOU>`, re-armed per `<EOU>` | unchanged if FluidAudio 0.18.3 matches (R2) |
| `SmartTurn.swift` | 56 | Smart Turn v3.2: finished-probability of the last 8 s | unchanged |
| `SpeakerEmbedder.swift` / `SpeakerProfile.swift` / `SpeakerFusion.swift` | 196 / 292 / 108 | WeSpeaker and CAM++, learned voice (three agreeing turns, no enrolment), joined space; `speaker-fusion.bin` 270,612 B | unchanged; the bundle lookup (`SpeakerFusion.swift:36-47`) becomes embedded bytes |
| `VoiceModels.swift` | 307 | pinned multi-file downloads, size and SHA-256 per file, staged and resumable | edit: install root string |
| `VoiceModelCache.swift` / `VoicePreparation.swift` / `VoiceSupport.swift` | 228 / 193 / 201 | load-once cache, "getting ready" state, rings, fades, prewarm gate | unchanged |
| `PhononVoice.swift` | 401 | Phonon 31m install, load, warm, idle unload; `synthesize` throws `busy` in voice mode | edit: drop `busy` |
| `FluidSpeechMini.swift` | 242 | Fluid Speech Mini warm final pass; `modelDirectory` reads the app bundle (`:27-37`) | edit: read the downloaded directory |
| `LiveDictation.swift` | 394 | sliding-window dictation; `LiveAudioSource` (`:14`); its own microphone and session (`:204-310`) | edit: microphone source fed by `bus.mic()` |
| `OnDeviceSpeech.swift` / `OnDeviceVoice.swift` | 294 / 188 | Apple recogniser and system voices (the app has both as stages) | port only `Failure` (about 40 lines) |
| `ScreenAwake.swift` | 19 | keeps the screen on | unchanged |
| `MikeSpeechPlugin.swift` | 523 | Capacitor bridge; a serial queue so clause 1 never overtakes clause 0 | **replaced** (2.2) |

13 test files, 2,615 lines (`TurnPolicyTests` 788, `ClauseQueueTests` 308, ...).
Pure ones need nothing; `VoiceModelIntegrationTests` needs
`TEST_RUNNER_MIKE_VOICE_MODELS`.

### 2.2 The port boundary, in five lines

1. Unchanged: `TurnPolicy`, `VoiceContract`, `ClauseQueue`, `EouStream`,
   `SmartTurn`, the three speaker files, `VoiceModelCache`, `VoicePreparation`,
   `VoiceSupport`, `ScreenAwake`, with their tests.
2. Edited: `VoiceEngine` (no audio engine of its own; transcribes and speaks
   through stages), `LiveDictation` (microphone source), `FluidSpeechMini` and
   `SpeakerFusion` (where the files are), `VoiceModels` (root), `PhononVoice` (`busy`).
3. Not ported: `VoiceAudioEngine` and its session calls (`:96-107, 398-408,
   375-396`: `LiveAudio` plus `ExactSessionOwner` already do them), the Apple
   recogniser and voices, Android.
4. Replaced: the Capacitor file by a registry stage (`turnTaking`, invented)
   and a kit protocol `VoiceBrain` (invented, below); the TypeScript
   conversation layer (`useVoiceChat.ts`, `voiceEngine.ts`, `speakable.ts`,
   `clauses.ts`, `spokenPrefix.ts`) by the app's Swift desk half (3.1).
5. Added: PhononCore (static) and the three `PhononTTS` Swift files (450 lines).

The engine's files go in the app module (`L/modules/apple/Voice/`, invented
path) because they import FluidAudio and PhononTTS; the kit "may not depend on
a SwiftPM library" (native-modules.md). Pure files with only Foundation imports
(`TurnPolicy`, `ClauseQueue`, `VoiceContract`, `VoiceSupport`, `SpeakerProfile`,
`VoicePreparation`, `SpeakerFusion` logic; their import lines were read) move
into the kit with their tests. The kit finds the app-module brain by name the
way it finds `AutoradioModelsRegistrar` (native-modules.md); `VoiceBrain`
(invented) is the kit protocol: `open()`, `close()`, `events`
(`VoiceEvent`: `state`, `partial`, `turn`, `bargeIn`, `clauseStarted`,
`clausePlayed`, `interrupted(fraction)`, `answerFinished`, `ended`, `ignored`,
`metrics`, `install`, `preparation`), `speak(turnId:clauseId:text:step:)`,
`endAnswer`, `cancelAnswer`, `forgetSpeaker`. Ordering (the plugin's serial
queue) is one consumer of one stream inside the brain (invented).

### 2.3 Audio session ownership, and the player half

Exact allows one session owner; our code never calls `setCategory`/`setActive`
(audio.md "The rule"). Nothing ported touches the session. The only session
change is in `LabRuntime.start` (`K/LabRuntime.swift:153`): the first claim
becomes `.talking` instead of `.listening` (the policy `alwaysTalking`,
already used by the interview, `X/AutoradioModule+Onboarding.swift:244`). A
claim change is release-then-claim and restarts the engine (audio.md "Claim
options and switch timing"), which `LiveAudio` already handles
(`recover`, `K/LiveAudio.swift:132-145`).

What `LiveAudio` lacks and the engine needs is a clause player with a clock.
Lift `VoiceAudioEngine.swift:153-231` (80 lines: `schedule(samples, at:, epoch:)`,
`playedSample`, `renderedSample`, `pausePlayer`, `resumePlayer`, `stopPlayer`,
the latency term at `:285`) into `LiveAudio` as a third node, `clauseNode`, at
24 kHz in the same mixer (an edit to `wire()`, `K/LiveAudio.swift:90-96`), with
an `epoch` bumped by `rebuild()` (`:141-147` replaces the nodes) so clauses
scheduled for an old graph are dropped, as the source does. The played-position
rule in audio.md ("Playback position with a time-pitch unit") applies:
freeze the position before a stop.

## 3. Binding to the app

### 3.1 The desk half

| Engine | Binds to | Where |
| --- | --- | --- |
| `turn(turnId, text, audioMs, continues)` | `TurnEngine.runTurn(_:transcript:)` (called today after `finalTranscript`; line not re-read, near `:410`), which starts `runPipeline`; `continues` cancels the replaced turn's pipeline (`interruptPipeline`, `:447`) | replaces `ingest` (`:255`), `route` (`:306`), `beginTurn` (`:357`) as the way a turn begins |
| `DeskTurn(turnID, programmeID, blockIndex, listenerWords, interrupted, spokenText, cutBlockIndex, page)` | built as `runPipelineBody` builds it (`baseReq`, about `:503`), with the cut from `interrupted(fraction)` | `K/Stages.swift:122` |
| streamed answer `DeskEvent.sentence/.failed/.done` | `AutoradioDeskStage.speech(_:heard:)` (`K/Choir/AutoradioDesk.swift`), `POST /api/autoradio/turns` ([AUTORADIO-TURNS.md](../reference/AUTORADIO-TURNS.md), not re-read) | `K/Stages.swift:156,167` |
| `speak(turnId, clauseId, text, step)` | the sentence loop of `LocalSpeaker.run` (`K/LocalSpeaker.swift:36`): `host.speak` per sentence instead of `voice.speak` then `playback.speak`; the clause synth is the configured `Voice` stage | |
| `clauseStarted` | `heard.add(sentence)` (`HeardLines`, `K/LocalAck.swift:131`; LocalSpeaker failure 5: recorded when a sentence starts to be heard) | |
| `interrupted(turnId, clauseId, fraction)` | `SpokenCut(blockIndex, full, spoken, fraction)` (`K/Playhead.swift:47`) with `SpeechCut.text(of:upTo:)` (whole words, `Playhead.swift`, near `:70`) | feeds `DeskTurn.interrupted/spokenText/cutBlockIndex` and the interview's `heard` |
| `state`, `partial`, `bargeIn` | `BarState` (`K/BarState.swift:251`), `.partialTranscript` event, `MarkState`; `state(.userSpeaking)` calls `playback.setLevel(.pause)` for a programme | replaces per-frame `tickYield` |
| `metrics` | `TurnTimeline` (`K/TurnTimeline.swift`) | |
| interview | `InterviewDesk.submit` already reads `turn.interrupted/spokenText` and calls `InterviewSession.hear(words, interrupted:, heard:)` (`K/Interview.swift:720, 756-766`); no change to `Interview.swift` | |

### 3.2 One edit to `TurnPolicy`

A programme is a long answer. `programmePlaying(text:)` (invented) puts the
policy in its `speaking` phase with answer id `"programme"` while a speech block
plays, so the existing rules (two words not in the block, 300 ms of speech, hold
at 700 ms, speaker check, VAD 0.7) protect the programme from its own
leakage; `answerEnded` ends it. Music and silence send nothing.

### 3.3 Reading aloud, dictation, and the five bindings

**Reading (any long text): one existing call, `LabRuntime.speakText(address:text:title:)`**
(`K/LabRuntime.swift:177`) → `ProgrammePlayer.speakText` (`K/LiveAudio.swift:551`),
which already queues text as a programme item; the cap `prefix(200)` is lifted
(chunk by paragraph). The programme's `voice` stage is the configured `Voice`
(Phonon by default), so no second API exists. Call sites: mail "Read it to me"
(`L/mail.contract:164`, `X/AutoradioModule+Mail.swift:68-75`,
`MailReading.shared.handler`, `K/LabRuntime.swift:124`) and a story's Play
(`L/paper.contract:37,64,83` → `choirPlay`) move with no change of call; the
programme player's speech blocks (`playBlock`, `K/LiveAudio.swift:581-600`) too.
New: a **Listen** control (invented) on the reader page (`L/reader.contract`,
`L/members-reader.contract`), Textures (`L/textures.contract`), PDF and EPUB
viewers (`L/viewers.contract`, `K/Epub.swift`) and captured text, through a new
send `readerListen` (invented, `X/AutoradioModule+Reader.swift`) that calls
`speakText` with the text the source already holds
(`K/TextureReaderSource.swift`). Where captured text lives was not located.

**Dictation: `VoiceBrain.dictate(field:) -> AsyncStream<DictationUpdate{confirmed, volatile, level, ended}>`,
`endDictation() -> String`, `cancelDictation()`** (invented; the cases are the
source's `LiveDictationSession.Event`). One microphone only, so the source's
`MicrophoneSource` (own engine and session) is replaced by an `EngineMicSource:
LiveAudioSource` (invented, the protocol is `LiveDictation.swift:14`) fed from
`bus.mic()`. While a field is dictated the brain drops commits instead of
sending turns (`VoiceEngine.carryOut` `.commit`, `:954-959`). Fluid Speech Mini
with `SlidingWindowAsrManager` gives live text. Fields (contracts): dictate in
the bar (`bar.contract:197`), mail subject and body (`mail.contract:277,281`),
the launcher, Textures, Files and PDF searches (`launcher.contract:85`,
`textures.contract:49`, `files.contract:143`, `viewers.contract:169`); not mail
To, file rename and new folder, theme id, lab config fields (identifiers, not
language; Q3).

**Interview and programme speech:** the interview's questions go through
`LocalSpeaker`/`host.speak`, answers come through `InterviewDesk` (table above).
The interview's private synthesizer and engine start
(`X/AutoradioModule+Onboarding.swift:30,186-195,203`) are unused after slice 1.

## 4. Barge-in and echo

From the code:

1. One engine with voice processing on the input, with every speaker node in
   the same mixer, so the canceller hears the output (`LiveAudio` `bring`,
   `K/LiveAudio.swift:195-215`; the source did the same).
2. Mode `voiceChat` via the `.talking` claim.
3. Words rule: a cut-in needs two words not in the clause playing, 300 ms of
   speech, VAD 0.7 while an answer is awaited or playing; playback is held after
   700 ms and resumed after 1.5 s without such words (`TurnPolicy.swift:414-465`,
   `VoiceContract.swift`).
4. If echo cancellation cannot start, the answer's words never count and three
   words are needed (`TurnPolicy.swift:96-102, 419, 435`).
5. Learned voice: after three agreeing turns a cut-in is checked against the
   listener's voice before the answer is stopped; other voices are not sent as
   turns (`checkSpeaker`, `ignored(.otherVoice)`, `VoiceEngine.swift:889-995`).
   Thresholds were fitted on simulated audio (the source says so); the first
   device run measures them.

When the programme is the app's player it is already in the same mixer; the
only change is 3.2 (the policy knows what the programme says). Podcast audio
must go through the programme node; Exact's `audio` element is not in the
engine's reference and would be heard as the listener (audio.md notes it as a
candidate for long-form; rejected here for this reason). While a session is
open the programme plays in `voiceChat` mode; Exact documents lower quality
there (audio.md "The two states we want"; Q2).

## 5. Build risks and slice 0

**R1. Phonon.** Read from the release asset in the SwiftPM cache on this machine
(`PhononCore.xcframework.zip`, 39,007,317 B): static `libptts_ffi.a` for
`ios-arm64` (50,769,320 B) and `ios-arm64-simulator` (50,435,088 B), `Headers`
with `ptts.h` and `module.modulemap` (`module PhononCore`). That is the shape
Exact links ("a static `lib*.a` slice gives `-I Headers -L dir -l<name>`").
The wrapper `PhononTTS` is source: `Phonon.swift` 234, `PhononModels.swift` 155,
`PhononPlayer.swift` 61 lines (`xn-ptts` v0.4.0 checkout under Mike's
`mobile/.cache`). Not verified: that Exact's module compile finds `PhononCore`,
that `-lptts_ffi` resolves, the 50 MB the link adds, that the wrapper compiles as
app-module sources.

**R2. `EouStream` on FluidAudio 0.18.3.** The source uses `AudioMelSpectrogram`,
`RnntDecoder`, `Tokenizer` (and `FluidSpeechMini`/`LiveDictation` use `AsrManager`,
`TdtDecoderState.make`, `SlidingWindowAsrManager`) as of 0.17.7. The app's built
`~/.cache/autoradio-deps/xcframeworks/FluidAudio.xcframework` (`libFluidAudio.a`)
contains all those names and `VadManager`, `StreamingEouAsrManager`; that proves
names, not signatures (no text interface ships; `abi.json` is 149 bytes).

**R3.** How an Exact app module puts files (a `.mlmodelc` directory) in the
`.app` is not verified; `scripts/check-no-weights.sh` fails any `*.mlmodelc` or
`*.bin` over 100 KB in the bundle and must move to a 100 MB ceiling.

**Slice 0 (yellow; rollback: revert), the build probe.** On a simulator the
builder creates: (1) copy `PhononCore.xcframework` (iOS slices; checksum
`a10382948448087bc9f63231de3c2970d606979fd69bef7acb95d2dfa3a41cef`, read in the
package's `Package.swift`) to `L/modules/apple/` and the three wrapper files;
`scripts/build-lab.sh`; (2) add the FluidAudio-bound files against the existing
xcframework (the compile is the signature check); (3) record `.app` size;
(4) read back `speaker-fusion.bin` through `mobile/scripts/embed-assets.py`
(name read, behaviour not) and one small `.mlmodelc` as a resource; (5) run the
moved pure tests. No behaviour changes; nothing is called.

Fallbacks, each an existing thing:

- R1 fails: speak through a voice stage that exists, Kokoro or Pocket
  (`AutoradioModels/VoiceStages.swift`, FluidAudio's TTS trait is already in the
  build) or `avspeech` (with the end-of-utterance fix on the fixes branch).
  This costs nothing extra because the clause synth is already a `Voice` stage
  (section 1); the configurator picks it. The source's own `OnDeviceVoice` is the second.
- R2 fails for `EouStream`: the app already runs FluidAudio's
  `StreamingAsrManager` EOU on 0.18.3 (`ParakeetStages.swift:22-27`) and the
  configurator already downloads that model (`parakeet-eou-120m`,
  `K/VoiceSetup.swift` ladder). Feed `TurnPolicy` its partials and `<EOU>`. The
  source records the manager loses words after a mid-sentence `<EOU>` (FluidAudio
  #1007), so it is degraded. Without live words there is no cut-in
  (`TurnPolicy.swift:435`): do not ship a loop that cannot be cut into.
- R2 fails for Fluid Speech Mini or dictation: the final pass uses the Parakeet
  TDT stage that already compiles (`ParakeetStages.swift:88-109`); only the
  model changes.
- R3 fails: download the small models too (the rule says "may" bundle).

**Cheapest first try for EOU (slice 0, step 2):** the configurator's
`parakeet-eou-120m` already downloads the same Hugging Face repository through
FluidAudio; if its cache directory holds `streaming_encoder.mlmodelc`,
`decoder.mlmodelc`, `joint_decision.mlmodelc` and `vocab.json`, `EouStream` loads
from it and slice 1 needs no new EOU download code. Not verified.

## 6. Models

Sizes in bytes. "Pinned list" = sum of `size` fields in `VoiceModels.swift` (added
with a script; they agree). "Measured" = `wc -c` on this machine.

| Model | Job | Size | Figure | Where |
| --- | --- | ---: | --- | --- |
| Silero VAD v6.2.1 | speech or silence | 1,063,425 | pinned list | **bundled** |
| Smart Turn v3.2 | confirm end of turn | 16,787,529 | pinned list | **bundled** |
| WeSpeaker (FBank + Embedding) | is it the listener | 15,291,553 | pinned list | **bundled** |
| CAM++ | same, combined | 16,615,589 | pinned list | **bundled** |
| speaker fusion matrix | joined space | 270,612 | measured | **bundled** (embedded) |
| Parakeet EOU 120M, 320 ms | live words, candidate end | 224,238,270 | pinned list | **downloaded** |
| Fluid Speech Mini | final pass, dictation | 252,730,493 | measured, whole directory | **downloaded** |
| Phonon 31m English | speech | 162,877,586 | the source's constant `phononBytes`; model files **not found** here | **downloaded** |

Bundled 50,028,708 B; downloaded 639,846,349 B. Phonon conflicts with this
repo's reference page (about 64-65 MB, vendor); show the sum from the pinned
`bundle.json`, not a constant. Fluid Speech Mini is CC BY 4.0 and needs its
attribution line shown (`NOTICE`). The engine needs about 700 MB resident and
`physicalMemory >= 5 GB` (`VoiceEngine.swift:52`); the owner's 6 GB phone passes.

**Download step, reused.** `ModelStore` (pinned SHA-256, resumable, Application
Support; `K/ModelStore.swift`), `VoiceSetup` rows and `SetupBackend.download`
(`K/VoiceSetup.swift:214`), the onboarding `setup` step
(`L/onboarding.contract:184`, handlers `X/AutoradioModule+Onboarding.swift:401-414`)
and `DeviceProfile` (disk, Wi-Fi) all stay live. `ModelStore` holds one file per
spec (`ModelSpec.fileName`), so the engine's multi-file models download with the
source's own installer (`VoiceModels.install`, `PhononModels.download`), called
from `SetupBackend.download` for the new candidates (invented glue). Ladders
(`SetupCatalog.ladder`): `transcriber` gains Fluid Speech Mini first; `voice`
gains Phonon first, Kokoro, Pocket and the system voice stay below; a
configurator check that fails (slow first sound, no memory) steps down as it does
today. The `bounds` row has no live use (the engine's VAD is fixed and bundled).

## 7. Slices, ordered by the happy path

Mutation class orange unless said; rollback revert. Every landing carries
`Ring-Expedite: the owner's phone has no working voice`. None touches a red
surface; the learned voice profile stays on the phone (if it ever leaves, red).
Failure items are `prevent / detect`.

### Slice 0: the build probe (section 5). Yellow.

### Slice 1: a hands-free turn on the device (the smallest change that makes one work)

Mechanism: the brain hears from `bus.mic()` and starts the turn.
Files: `L/modules/apple/Voice/*` (brain, `EouStream`, `SmartTurn`, `TurnPolicy`),
`K/VoiceBrain.swift`, `K/LabRuntime.swift` (`start`: claim `.talking`; the
`micTask` that sent each frame to `engine.ingest` becomes the brain's input and a
task that calls `runTurn`), `K/TurnEngine.swift`, `K/Choir/ChoirApp.swift:16` and
`K/BasicStages.swift:373` (default `turnTaking`, no `addressing`/`speechBounds`),
the talk setting removal (section 8), the interview's mic-check and hearing steps
(`Onboarding.swift:186-195`) now served by the brain, `L/bar.contract:205`
(mic is voice on/off). The final pass uses the configured `Transcriber` as it is
today; speech out is unchanged (existing `Voice`, sentence granularity). Models:
Silero and Smart Turn bundled, EOU through the existing download step.
Failures:
1. EOU missing at open / `VoiceMode.status` names it; open fails with a plain line, no loop that cannot be cut into / trace error.
2. Turn sent twice on `continues` / cancel the replaced pipeline / test with two `turn` events.
3. The radio's own voice becomes a turn / `.talking` claim, words rule / replay of `TurnPolicyTests`, device step.
4. Screen says the microphone is open when it is not / line shown only between `open()` and `ended` / contract test per state.
5. Phone call or Siri leaves a half-open session / claim `interrupted` and `reset` close it, `resumed` reopens / host test with injected claim events.
6. Below the memory floor / `open()` throws `unavailable`, plain line, typing works / test with injected memory.
7. Cough, door / the final pass returns empty and the policy discards (`:1005`) / fixture of noise.
8. Session when the app goes to the background / the brain closes on `didEnterBackground` until a device shows locked-screen capture (audio.md, unrun) / owner step.
Tests first: `TurnPolicyTests` moved in; a brain test with a fake `bus.mic()`
feeding the fixture clip (`ios/Tests/.../Fixtures/dictation.m4a`) and fakes for
`Transcriber`; `SettingsDataTests` without `talkMode`; onboarding line states.
Simulator: inject the fixture clip through the virtual audio path
(`LabRuntime.start(virtual:)`): a `turn` arrives, `runPipeline` calls the desk
(test server), the interview records one radio line and one owner turn with no
tap. Owner steps: finish onboarding, answer the interview aloud with no tap and
say whether your words appear; stay silent ten seconds; cough; lock the screen
and say whether it ends.

### Slice 2: a natural voice that starts fast, and cutting in

Mechanism: clause speech with the played-sample clock.
Files: `LiveAudio` (`clauseNode`, 2.3), the brain's speaking side (`ClauseQueue`,
`pumpSpeech` reading the configured `Voice`), `LocalSpeaker` (3.1), `phonon`
`Voice` stage, Phonon download row, `PhononVoice` (`busy` dropped),
`VoiceModels`/`FluidSpeechMini` roots as needed, the configurator ladder
(section 6), `X/AutoradioModule+Onboarding.swift` (private synthesizer
`:30,203` unused). Failures:
1. Clause 1 overtakes clause 0 / one consumer / ordering test.
2. After a cut-in the old answer keeps coming / `cancelAnswer` plus cancelling the desk task / no `speak` after `bargeIn`.
3. Wrong `heard` to the desk / `SpeechCut.text(of:upTo:)` whole words / fractions 0, 0.5, 1, NaN.
4. Slow first sound on a cold model / `prewarmAtLaunch`, "getting ready" in the step / `metrics.commitToFirstAudio`.
5. A clause fails / log and move on (`VoiceEngine.swift:1133`) / trace.
6. Rebuild after a route change replays or drops audio / epoch drop and `rewind` / test.
7. Server text speaks badly (citations, numbers) / read `AUTORADIO-TURNS.md` first / ten real outputs listened to.
8. Interview question spoken twice / fixes branch plus a count test / test.
Tests first: `ClauseQueueTests` moved in; `LocalAckTests` rewritten to a fake host.
Simulator: typed turn speaks clause by clause (trace `clauseStarted/Played`);
scripted `bargeIn` stops it and the next request carries `heard`. Owner steps: ask
by voice and say how fast it starts; interrupt mid-sentence and say whether it
stops about half a second later and listens; say "hm" and say whether it carries
on; leave a television on.

### Slice 3: no self-hearing, and reading aloud

Mechanism: the programme and long texts are answers to the policy; the speaker check is on.
Files: `TurnPolicy` (`programmePlaying`), `ProgrammePlayer`, `readerListen` and
the Listen controls (3.3), the speaker models' bundled files and rows, "forget
my voice" in Settings (`forgetSpeaker`), the `speakText` cap. Failures:
1. A paragraph cut at its first phrase pause (the old defect) / Phonon streams to the end, the clock ends it / 600-character test.
2. The reading's own words count as cut-in words / pass the block text / policy test.
3. Reading and answer collide / the answer goes first, the programme node pauses, resumes after `answerFinished` / interleave test.
4. The speaker check rejects the owner / threshold 0.35 errs toward the person; measured on a device / owner step.
5. Listen has no text yet / disabled until the source has text / contract test.
6. A 3,000-paragraph text queues huge / the queue holds paragraphs, not audio / test.
Tests first: new `TurnPolicy` cases; reader-source test that Listen passes the
held text. Simulator: story, Texture and mail read to the end
(`programmeAudioStart` per paragraph); a scripted cut-in pauses the programme.
Owner steps: Play one story to the end; Listen on a reader page, a Texture and an
EPUB chapter; speak over the programme; play on the speaker loud and say whether
it hears itself.

### Slice 4: dictation, and Fluid Speech Mini as the default final pass

Mechanism: dictation into text fields through the same microphone.
Files: `LiveDictation` edits, `EngineMicSource`, `fluid-speech-mini` stage and
download row, a dictation button in `L/components.contract` and the call sites of
3.3. Failures: speech goes to the field and the radio / dictation mode drops
commits / host test; two microphones / the source reads `bus.mic()` only / grep
for a second `AVAudioEngine()`; text lands in the wrong field / session tied to
the field, blur ends it / contract test; text lost on background / `ended: true`
keeps it / `LiveDictationTests` (165); no session open when dictating / open loads
only Fluid Speech Mini, "getting ready" / timing. Tests first:
`LiveDictationTests` with a fake source. Simulator: the fixture clip into the bar
and the mail body, no turn sent. Owner steps: dictate into the bar, a mail body
and the launcher search.

## 8. The talk setting and the microphone controls

The setting is `talkMode`, group `talk` (`K/SettingsData.swift:39-44`). All three
values go; nothing meaningful survives, so the setting and its group go:
`button` "Tap to talk" (`ButtonAddressing`), `word` (`HybridAddressing`, five
minutes after a turn), `open` (`OpenAddressing`; now the only behaviour).

Removed from the live paths: the catalog entry, `addressingStage(forTalkMode:)`,
`listeningLine(forTalkMode:)`, `apply(to:)` (`SettingsData.swift:85-96`); the
handler branch (`X/AutoradioModule+Settings.swift:165`); the Talk screen and row
(`L/settings.contract:30,239,396-401`, `members-settings.contract:14`,
`app.contract:266`); `d["listenLine"]` (`X/AutoradioModule+Onboarding.swift:367`)
and `onboarding.contract:285`, which returns to "The microphone is open. Talk any
time, even over me." shown only while the session is open (slice 1 failure 4);
the `.addressing` defaults (`BasicStages.swift:373`, `ChoirApp.swift:16`) and the
tests of those ids (`KeptTurns.swift:111-112`, `LabRuntime.swift:263`). Gestures
(`swipeLeft`, `swipeRight`) stay. A stored `AutoradioSetting.talkMode` is
ignored: `AppSettings.value` looks up only catalog ids (`:65-68`). No migration.

Controls that remain (not a mode): the bar's mic button (`bar.contract:205`,
`micTap`, `spine.contract:56-64`) is voice on/off: closing releases the claim,
the microphone and the orange indicator stop; `choirTalkCancel` (`:64`,
`LabRuntime.cancelTalk`, `:251`) becomes "stop speaking" (`cancelAnswer`); player
pause; `LabRuntime.stop()` (`:184`).

## 9. Status of every existing piece (nothing is deleted)

| Piece | Status |
| --- | --- |
| The engine's brain, Silero, EOU, Smart Turn, speaker models, `ClauseQueue` | **on the happy path** |
| `LiveAudio` (device, programme node, now the clause node), `ExactSessionOwner`, `ProgrammePlayer`, `LocalSpeaker` (sentence loop), `TurnEngine.runTurn`/`runPipeline`, `AutoradioDeskStage`, `Interview`, `ModelStore`, onboarding setup step, `SetupBackend` | **on the happy path** |
| Fluid Speech Mini, Phonon | **default** `Transcriber` and `Voice` |
| Kokoro, Pocket TTS, Apple synthesizer (`avspeech`), Apple recogniser, Parakeet TDT and EOU stages, LFM3 audio, Silero stage (`AutoradioModels`, `AppleStages`) | **selectable** through the configurator; Parakeet EOU is also the R2 fallback; the Silero stage is not consumed by the brain (it has its own copy) |
| Configurator and ladders (`Configurator`, `ModelCandidates`, `LiveConfigBackend`, `VoiceSetup`) | **live**, used now to download and to choose voice and transcriber |
| `ButtonAddressing`, `HybridAddressing`, `OpenAddressing`, `EnergyVAD`, `TurnEngine.ingest/route/beginTurn/finalTranscript/tickYield`, `talkDown/talkUp`, the `listening` claim and `AudioPolicy.switchOnAddress/alwaysListening`, the interview's private synthesizer and engine start, the `bounds` setup row | **unused for now** (no caller; code kept) |
| `LocalAck` holding line and the responder ladder branches (`lane/ar3-voice`, its backup, `docs/on-device-voice-research`) | **later additions**; attach at `Holder` and the brain's "step" clauses (`speak(..., step: true)`, `ClauseQueue.yieldSteps`); slice 2 passes `step` through and keeps `runPipelineBody`'s holder resolution so they can return; the shipped holder is the soft cue floor |
| Mac replay command, lab spikes | untouched |

## 10. Open questions, each with a default

- Q1. When does the session open? Default: when onboarding ends and the app is in
  the foreground; closes in the background or on the mic button.
- Q2. Programme quality in `voiceChat` mode while a session is open? Default:
  accept it.
- Q3. Dictation fields? Default: section 3.3's list.
- Q4. A phone under the memory floor or iOS below 18? Default: say so plainly in
  onboarding; typing and taps work.
- Q5. Downloads over 100 MB on cellular? Default: ask first, showing the total
  (about 640 MB).
- Q6. Speaker learning? Default: on (WeSpeaker decides, CAM++ installed too,
  kept on the phone, "forget my voice" in Settings).
- Q7. Locked-screen listening? Default: not in this change.

## 11. Not verified, and invented

Not verified: that the port builds (R1-R3); that any source test passes here;
latency, memory, heat or battery on the phone; locked-screen capture; that the
configurator's EOU cache has the layout `EouStream` needs; that a test target
can see app-module files; that `embed-assets.py` suits a binary file; where
captured text lives; `AUTORADIO-TURNS.md` and the rendering reference (not
re-read); Phonon's iOS bundle size; the source commit (relayed).

Invented: the `turnTaking` stage kind and its id, `VoiceBrain`, `L/modules/apple/Voice/`,
`clauseNode`, the stage ids `fluid-speech-mini` and `phonon`, `programmePlaying`,
`EngineMicSource`, dictation mode and API, `readerListen` and Listen, the
`SetupBackend.download` glue, the mic button's on/off meaning, the slice
numbering and failure items.
