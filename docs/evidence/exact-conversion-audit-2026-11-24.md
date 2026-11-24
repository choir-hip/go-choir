# What is left in Swift and what moves to Exact (audit, 2026-11-24)

This is a read-only audit of origin/lane/exact-rest and origin/lane/onboard-consent. The line counts for split files are estimates from reading the code, not measurements. Nothing was built or run.

I counted 111 Swift source files (19,280 lines) and 72 test files (669 tests). About 45% of the lines stay native and about 55% move to Exact.

**Refs and caveats.** `exact-rest` (c46bc56ab, the non-voice side) and `onboard-consent` (b6ebe6d78, which contains voice-default) branch from different bases, so their union is not a real tree.
- **Voice-side files:** these come from `onboard-consent`. They are marked (O) below.
- **Everything else:** this comes from `exact-rest`, marked (R).
- **Files not counted:** `onboard-consent` still carries about 3,000 lines that `exact-rest` deleted (Notebook, RunSheet, TextureBody, MailSeal, the Joins and Podcasts handlers).
- **Native-passkey:** that ref changes ChoirClient and ChoirSignIn and deletes CredentialReader. I used the `exact-rest` versions of these files and noted the change in the rows.
- **Missing lanes:** `voice-pro` and `responder-omni` do not exist on origin.
- **Split estimates:** the "N stay" figures for split files are my estimates from reading the code.
- **Line counts:** `LiveAudio.swift` is 1,212 lines but about 75 KB, so line counts understate native weight.

## 1. Totals

| | Source lines | Tests |
|---|---|---|
| Stay native | 5,818 (+ 2,900 inside split files) = 8,718 | 170 |
| Move to Exact | 5,776 (+ 4,786 inside split files) = 10,562 | 369 |
| Split | 7,686 across 24 files | 130 (11 files) |

A test file for a mover goes to `<its target>.test.ts`, written from the failure list in the Swift file's header, not translated.

**What Exact has and lacks for the movers**
- **Has:** fetch with POST, JSON, `storage.fs` and sqlite, secrets, and `crypto.subtle` digests. These are in `EXACT.md` lines 357-359 and 67, and `ui-model.md` line 86.
- **Has:** timers only inside one answer. There is no `Date.now`, so native must stamp every fact with time.
- **Has:** a 100 ms CPU turn and a 64 MiB heap (`internals.md`).
- **Lacks:** a POST-bodied `exactStream` (`gaps.md` item 36). The desk turn is a POSTed stream spoken sentence by sentence. The fix is either a server route over WebSocket (the `net.websocket` grant exists) or a GET with a ticket.
- **Lacks:** a native-to-shared push. Native can only call `changed()` and shared code reads again (`native-modules.md`, "Out"). The cost of `changed` at 20 Hz is unmeasured.
- **Lacks:** a documented playback rate or ended event on the `audio` element. `audio.md` lines 147-156 list src, paused, currentTime, timeupdate and metadata only.
- **Lacks:** any file-hash call. Hashing the large model files stays on native.
- **Unproven:** whether shared code keeps running on a locked phone. That holds only while a `speech-playback` claim is held, and it is unrun (`audio.md`, "What keeps shared code running").

## 2. The boundary left after everything movable has moved

Shared code to native, all through the existing `later` and `radioSet` pattern:
1. `micOpen(on, echoCancel)` and the push-to-talk hold and release.
2. `finalise(turnId, fromMs, toMs)`, which returns the final text.
3. `speak({id, text, voice, kind})`, `speakStop({id, atSentenceBoundary})` and `setLevel(play|dip|pause)`.
4. `localModel.generate({id, prompt})`, `localModel.cancel` and `embed(texts)`.
5. `speakerEmbed(utteranceId)`, which returns vectors.
6. `modelsFetch`, `modelsLoad` and `modelsUnload`, `deviceFacts`, `micPermission`, `hearLine`, the deck's five names and the take recorder ops.

Native to shared code, as batched facts at 10 Hz or less, each stamped with host time:
- speechStarted, speechEnded, partial and eou, each with a time in ms.
- A clock tick and the mic level.
- spokenSentence, speakEnded (with whether sound was rendered), and generatedSentence.
- Route, interruption and session events, plus model status.

Native claims and releases the audio session itself, in the order release, claim, engine restart (`SessionCoordinator`). Shared code never sees claims.

**What really has to be native, with evidence**
- **Audio-thread taps:** the mic tap uses a 1024-frame buffer, about 21 ms at 48 kHz, and the mixer reference tap uses 2048. Both push stamped frames to the bus (`LiveAudio.swift` lines 410-421, O).
- **Per-frame detection:** the voice-activity detection runs once per frame, and Silero works in 256 ms hops. The ducking reflex `tickYield` lives in the same loop (`TurnEngine.swift`).
- **Rendered position:** `Playhead` counts samples actually rendered.
- **Lock-screen commands:** these must act while shared code is frozen.
- **Non-verbal cue:** the "heard you" cue is played natively at the end of speech.
- **Text-to-speech end rule:** the system voice's end-of-utterance rule (`SpeechFixesTests` S2).

**What is not latency-sensitive.** Every policy threshold is 300 ms or longer. Barge-in needs 300 ms of speech (`bargeInSpeechMs`). The production yield policy is `dip-then-pause`, which dips after 1.0 s of speech and pauses after 10 s. Silence ends a turn after 1,000 ms, and the false-start window is 1,500 ms. `TurnPolicy` is pure and driven by ms stamps, so it replays identically however fast events arrive.

The `EngineLimits` header also says no model call gets a deadline. A hop of 50 to 100 ms costs nothing in the policy. The one unmeasured risk is the `changed` round trip. If it exceeds about 100 ms on a device, add one native deadline-timer op, with the decision still made in shared code.

## 3. Slices, each landing alone

1. **Device probe (yellow):** measure the native-fact to shared-code to native-command round trip, and the locked-phone claim. This deletes nothing. Only a device can prove it.
2. **Model configuration and choice:** `voice-config.ts`, `voice-setup.ts` and `voice-choice.ts` take over `ModelList`, `ModelCandidates`, `KnownModels`, `Configurator`, `LiveConfigBackend`, `ListenerPipeline`, `ModelOffer`, the `VoiceChoice` catalog and the `VoiceSetup` steps. Native keeps fetch, load and `deviceFacts`. The failure lists are the headers of those files and `ModelListTests` L1 to L3. It deletes about 2,900 lines and 99 tests, and it must wait for voice-default and onboard-consent. Only a device can prove a real download and a cellular consent.
3. **Speaker, enrolment and kept turns:** `speaker.ts` and `kept-turns.ts`, using `speakerEmbed`. It conflicts with onboard-consent (SpeakerGate, KeptTurns, VoiceEnrolment). Only a device can prove embedding accuracy.
4. **Turn policy and hands-free:** `turn-policy.ts` and `hands-free.ts` take over `TurnPolicy`, the policy-driving half of `HandsFree`, and `HandsFreeSwitch`. Native emits events. The tests are written from `voice-chat.md` and the H1 to H8 list, not from the Swift trace tests. It conflicts with `HandsFree.swift` edits on voice-default and onboard-consent. Only a device can prove barge-in feel and echo with Bluetooth.
5. **Turn pipeline:** `voice-turn.ts` takes over `TurnEngine`, System 1, the holder, the desk client, `DeskContext`, `LocalAck` prompting, `Conversation`, `RetrievalFacts`, `HeldPages` and `TurnTimeline`. The desk stream needs the WebSocket or GET-with-ticket route first (server work, orange). It conflicts with `exact-rest` (ChoirApp, RadioBridge) and native-passkey (ChoirClient, key read). This is the largest slice. Only a device can prove first-sound latency.
6. **Deck rules and remote audio:** the rules and held list move to `deck.ts`, and `RemoteAudioPlayer` is replaced by the `audio` element. Do this only after the locked-phone proof and a rate answer from Exact. Only a device can prove it.
7. **Lab harness:** retire `autoradio-replay`, the `LabRuntime` config store and the fixtures, in favour of bun replay of traces.

## 4. Android

These run unchanged on Android: everything that moves, meaning the policy, hands-free state, the pipeline, model choice, speaker scoring, the stores, and the deck rules if slice 6 lands. Android also needs the slice 1 probe, because its page freezes when the activity stops unless a foreground service holds a claim.

Kotlin remains to be written as an `ExactExtension` that implements the narrow interface above:
- **Audio:** `AudioRecord` with the platform echo canceller, `AudioTrack` with sample-counted position, audio focus, and MediaSession for the lock screen.
- **Models:** Core ML models are Apple-only. Silero, recognition, text-to-speech and the embedders each need ONNX or LiteRT equivalents, and llama.cpp needs a JNI build.
- **Platform facts and views:** device facts, file hashing, the take recorder, and the web view.

## Table: source files (111)

| File (ref) | Lines | Does | Verdict and Exact file or reason |
|---|---|---|---|
| AccountWipe (R) | 104 | Sign-out wipes registered stores | moves: `account.ts` wipe list; `storage.fs` delete |
| AppleStages (R) | 296 | SFSpeech transcribe, AVSpeech synthesise | native: SFSpeechRecognizer, AVSpeechSynthesizer |
| Audio (R) | 174 | Audio frames, bus, session protocols | native: mic and reference frames off the audio thread |
| BarState (R) | 260 | Native facts the bar reads | split (110 stay): mic level, speed, player facts; rest dead with SegmentTracker |
| BasicStages (O) | 273 | Energy VAD, mic, yield policies, fakes | split (110 stay): EnergyVAD and RouteMic stay; addressing, yield thresholds and fakes go to `voice-turn.ts` |
| Choir/AutoradioDesk (R) | 221 | Desk turn POST, stream, speak | split (90 stay): reading the POSTed stream until gaps.md 36 closes; rest to `desk-client.ts` |
| Choir/ChoirApp (R) | 257 | Engine controller, sign-out order | split (70 stay): building native stages stays; voice default, desk config and sign-out order go to `account.ts` |
| Choir/ChoirClient (R) | 418 | Keychain key, HTTP client | moves: `api.ts` authedJSON; key is Exact secret `choir.key` after native-passkey (a native Keychain read of about 40 lines, if any) |
| Choir/ChoirDesk (R) | 173 | Texture desk stage, proof only | moves: delete or `desk-client.ts` |
| Choir/ChoirPlayback (R) | 74 | Deck engine adapter | native: glue to AVPlayer and the programme player |
| Choir/ChoirProgramme (R) | 55 | Markdown to spoken paragraphs | moves: `words.ts` |
| Choir/ChoirSignIn (R) | 125 | Local sign-out, revoke | moves: `account.ts`; native-passkey deletes the device flow |
| Choir/ChoirSystem1 (R) | 83 | System 1 judgments request | moves: `api.ts`; plain POST |
| Choir/ClipFingerprint (R) | 141 | Clip fingerprint, FFT | native: Accelerate vDSP; Hermes 100 ms CPU turn |
| Choir/ClipLocator (R) | 349 | Find clip again in file | split (100 stay): AVFoundation byte-range decode stays; search to `clips.ts` |
| Choir/CredentialReader (R) | 35 | Reads Keychain key | moves: deleted by native-passkey |
| Choir/RadioProgramme (R) | 288 | Programme segment types | moves: `programme-model.ts` has the decoder |
| Choir/Takes (R) | 27 | Take responds-to record | moves: `take-session.ts` |
| Concurrency (R) | 110 | Once, deadline, task bag, lock | native: serves code that stays; shrinks |
| Configurator (R) | 401 | Pick one model per role | moves: `voice-config.ts`; needs only native `deviceFacts` |
| Conversation (R) | 124 | Conversation entries | moves: `conversation.ts` exists |
| Core (R) | 246 | JSON value, stage registry, config types | split (80 stay): registry building native stages; config types to `voice-config.ts` |
| Deck (R) | 601 | Deck: play, hold, rules, journal | split (350 stay): engine drive, journal file, lock-screen reflex, session retry stay; rules 1 and 4 and the held list move to `deck.ts` after the locked-phone proof |
| DeskContext (R) | 63 | What phone tells model about now | moves: `voice-turn.ts` |
| EditionSetup (R) | 186 | Edition writer native half | moves: `edition-writer.ts` exists |
| EmbedDevice (O) | 92 | Wi-Fi, power, thermal; ensure | native: NWPathMonitor, Low Power Mode, thermal state |
| Embedder (R) | 27 | Embedding model protocol | native: llama.cpp seam |
| Events (R) | 236 | Clock, event log, types | split (70 stay): host clock stamped on audio stays; log and types to `voice-turn.ts` |
| ExternalModels (R) | 10 | ObjC seam to app-module models | native: xcframeworks link only in the app module |
| FixtureClip (R) | 12 | Generated clip fixture | moves: app asset read by path |
| FixtureTurn (R) | 58 | Generated kept-turn fixture | moves: app asset read by path |
| FixtureTurnAudio (R) | 13 | Fixture speech accessor | moves: goes with the asset |
| HandsFree (O) | 336 | Hands-free turn-taking driver | split (110 stay): per-frame ingest, VAD feed and session partials stay; policy driving, commit, barge-in, visibility and speaker call go to `hands-free.ts` |
| HandsFreeSwitch (R) | 73 | On/off, idle expiry | moves: `hands-free.ts`; native gets `micOpen(bool)` |
| HeldPages (R) | 82 | Page in front for voice | moves: gone when JS builds context |
| KeptTurns (O) | 178 | Kept turn store | split (60 stay): writing mic.caf stays; prune, list and context.json go to `kept-turns.ts` |
| KnownModels (O) | 251 | Model catalog | moves: server-fetched model-list data |
| LabRuntime (O) | 487 | Lab runtime, config store, memory | split (90 stay): resident memory and engine construction stay; config store to `voice-config.ts` |
| LaunchTimings (R) | 61 | Launch timing, prepared audio | split (25 stay): PreparedAudio samples and process clock |
| LiveAudio (O) | 1212 | AVAudioEngine, taps, nodes, ProgrammePlayer | native: installTap, MPNowPlayingInfoCenter, rendered-sample position |
| LiveConfigBackend (R) | 42 | Configurator backend | moves: with Configurator |
| LocalAck (R) | 397 | Local model acknowledgement | split (80 stay): llama.cpp call and non-verbal cue stay; prompt, splitter, history and holder go to `local-ack.ts` |
| LocalSpeaker (R) | 70 | Speak stream, cut at sentence | native: cut lands on a rendered boundary |
| MicCheck (R) | 62 | First-run mic check rules | moves: `onboarding.ts` |
| ModelCandidates (O) | 150 | Candidate table per role | moves: `voice-config.ts` data |
| ModelStore (O) | 277 | Fetch, hash, load, unload | split (170 stay): file SHA-256 of large GGUF, load and unload stay; specs and ledger go to `voice-config.ts` |
| PlaybackContinuity (R) | 209 | Interruption, route, lock-screen recovery | native: AVAudioSession, MPRemoteCommandCenter |
| PlaybackSupport (R) | 33 | URL typed in bar | moves: `bar.ts` |
| PlayerControls (R) | 58 | Speed, cursor, autoplay | moves: `deck.ts` and settings |
| Playhead (R) | 85 | Position from rendered audio | native: counts rendered samples |
| ProofKey (R) | 35 | Simulator key import | native: Keychain write, simulator only |
| QueueVoice (R) | 19 | Queue hooks for voice | moves: `voice-queue.ts` exists; delete |
| RadioBridge (R) | 80 | Mailbox native to shared | native: is the boundary |
| ReaderFollow (R) | 66 | Block and sentence being read | moves: `reader.ts` |
| RemoteAudioPlayer (R) | 274 | AVPlayer for podcasts and clips | moves: Exact `audio` element (audio.md 147-156). Lacks: playback rate to 3x, ended and interruption events |
| RetrievalFacts (R) | 130 | Native asks shared for passages | moves: JS calls `retrieval-run.ts` directly |
| RunSheetTypes (R) | 19 | Run sheet enums | moves: `runsheet-model.ts` |
| SavedAnswerHolder (R) | 148 | First answer from held passages | split (30 stay): small-model call; holder to `local-ack.ts` |
| ScriptedStages (R) | 26 | Fixed-words transcriber | native: stage stand-in |
| SegmentTracker (R) | 168 | Place events, no production reader | moves: delete |
| SessionCoordinator (R) | 367 | Audio session states | native: claim, release and engine-restart order |
| SourceLink (R) | 23 | Where a tapped link goes | moves: `bar.ts` and `reader.ts` |
| Stages (R) | 316 | One protocol per job | split (150 stay): Transcriber, Synth, VAD and Embedder protocols; System1, Desk, Holder, Addressing and Yield move |
| TakeSession (R) | 240 | Take recorder | native: AVAudioFile, SFSpeech, radio gate |
| TurnEngine (R) | 789 | Turn lifecycle and pipeline | split (200 stay): preroll, frame append, VAD feed, speak and claim calls stay; phases, System1 to holder to desk to voice, and outcomes go to `voice-turn.ts` |
| TurnTimeline (R) | 148 | Timing trace | moves: `voice-turn.ts` |
| Voice/SpeakerFusion (O) | 108 | Join two embeddings | moves: `speaker.ts` |
| Voice/SpeakerGate (O) | 114 | Is it the reader's voice | moves: `speaker.ts`, cosine over vectors |
| Voice/SpeakerProfile (R) | 292 | Speaker models, scoring | split (100 stay): embedding calls stay; scoring and catalog to `speaker.ts` |
| Voice/TurnPolicy (R) | 520 | Endpointing, barge-in | moves: `turn-policy.ts`; pure, no audio |
| Voice/VoiceContract (R) | 221 | Voice wire types | moves: contract types |
| Voice/VoicePreparation (R) | 193 | Model preparation status | moves: `voice-setup.ts` |
| Voice/VoiceShim (R) | 11 | Two shim names | moves: delete |
| Voice/WeSpeakerEmbedder (R) | 120 | Speaker embedding | native: Core ML |
| Voice/ListenerPipeline (O) | 189 | Default pipeline from list | moves: `voice-config.ts` |
| Voice/ModelList (O) | 104 | Model list as data | moves: `voice-config.ts`, server fetch |
| Voice/VoiceEnrolment (O) | 122 | Learn reader's voice | moves: `speaker.ts` |
| ModelOffer (O) | 113 | Download jobs, consent | moves: `voice-setup.ts` |
| VoiceChoice (R) | 265 | Catalog, choice, fallback voice | split (90 stay): SpeakingVoice first-sound fallback stays; catalog, choice and readiness go to `voice-choice.ts` |
| VoiceSampler (R) | 159 | Tap a voice, hear a line | moves: `voice-choice.ts` |
| VoiceSetup (O) | 649 | Model setup steps and checks | split (200 stay): FluidAudio fetch, Core ML compile and proof run stay; steps, picks and words go to `voice-setup.ts` |
| Models/EmbedDeviceTrial (R) | 131 | Embedding measurement | native: Darwin tool |
| Models/EmbedTrialFixture (R) | 31 | Trial corpus | native: with the trial |
| Models/FluidSpeechMini (O) | 82 | Speech recogniser | native: FluidAudio Core ML |
| Models/HubModelProvider (O) | 370 | Fetch, load, prove models | split (250 stay): load stays; setup order to `voice-setup.ts` |
| Models/Lfm3Audio (R) | 98 | LFM3 audio stages | native: FluidAudio |
| Models/LlamaEmbedder (R) | 76 | Embedder | native: llama.cpp |
| Models/LlamaSystem1 (R) | 401 | Local language models | native: llama.cpp |
| Models/ModelHub (O) | 120 | Model registry | native: loading |
| Models/ModelProof (O) | 147 | Mac model measuring | native: Mac tool |
| Models/ParakeetStages (R) | 121 | Streaming recogniser | native: Core ML |
| Models/SileroVAD (R) | 86 | VAD | native: Core ML per 256 ms hop |
| Models/VoiceStages (R) | 196 | Neural TTS frames | native: FluidAudio |
| Tool/autoradio-models (R) | 176 | Mac measuring CLI | native: Mac only |
| Tool/autoradio-replay (R) | 86 | Mac engine replay | moves: replaced by bun trace tests |
| Pkg/Module (O) | 243 | Package: claims, registry, ops | native: `context.audioSession.claim` |
| Pkg/+Bar (R) | 154 | barFacts, jump, speed | native: facts only audio code knows |
| Pkg/+Deck (R) | 53 | Deck names | native: deck interface |
| Pkg/+Imported (R) | 28 | Hand imported text to voice | moves: gone when JS builds context |
| Pkg/+Mail (R) | 24 | Open approval page | native: in-app web view |
| Pkg/+Notebook (R) | 15 | contentBusy fact | native: fact |
| Pkg/+Onboarding (O) | 335 | Voice setup, mic, hearing ops | split (230 stay): mic permission, setup trigger and hear-line stay; rest to `voice-setup.ts` |
| Pkg/+Platform (R) | 166 | UIKit/AppKit seam | native: UIKit calls; Exact `Platform.open` unchecked |
| Pkg/+Queue (R) | 24 | radioNote touch | native: `changed()` topic |
| Pkg/+Reader (R) | 116 | Web view, aim, page text | split (60 stay): WKWebView node stays; aim and pageText ops go |
| Pkg/+Retrieval (R) | 27 | Retrieval ask, embed ops | split (15 stay): embed ops stay; ask goes |
| Pkg/+Settings (R) | 19 | settingsFacts | native: bundle version, kept turns |
| Pkg/+Spine (R) | 81 | Open pages, pagesMirror | split (20 stay): pagesMirror and holdPages go |
| Pkg/+Takes (R) | 87 | Recorder native side | native: recorder |
| Pkg/+Talk (O) | 45 | Conversation entries | moves: `conversation.ts` |
| Pkg/+Voice (R) | 119 | Voice cards ops | split (40 stay): hear-a-line stays; list and pick go to `voice-choice.ts` |

## Table: test files (72)

| Test file | n | Source tested | Verdict |
|---|---|---|---|
| AccountWipeTests | 4 | AccountWipe | moves |
| AckTimingTests | 4 | LocalAck timing | moves |
| AimAndLensTests | 1 | bar aim | moves |
| AnswerKeepsPlaceTests | 3 | LiveAudio gate | native |
| AudioHeldTests | 20 | LiveAudio, SessionCoordinator | native |
| AudioSessionHoldTests | 4 | SessionCoordinator | native |
| AutoradioDeskTests | 24 | AutoradioDesk | moves |
| BarStateTests | 4 | BarState facts | native |
| ChoirClientTests | 26 | ChoirClient | moves |
| ClipLocatorTests | 11 | ClipLocator | split |
| ConfiguratorTests | 24 | Configurator | moves |
| ContentFullScreenTests | 9 | contract layout text | moves |
| CredentialReaderTests | 6 | CredentialReader | moves (deleted by native-passkey) |
| DJClipsTests | 2 | clip candidates | moves |
| DeckTests | 24 | Deck | split |
| EditionSetupTests | 10 | EditionSetup | moves |
| EmbedDeviceTests | 2 | EmbedDevice | native |
| FakeRetrieval | 0 | helper | moves |
| HandsFreeTests | 5 | HandsFree | moves |
| HandsFreeToggleTests | 20 | HandsFreeSwitch | moves |
| HeldPagesTests | 8 | HeldPages | moves |
| JoinsTests | 1 | clip parse | moves |
| LearningLoopTests | 0 | none left | moves (delete) |
| ListenTextTests | 1 | listen rule | moves |
| LocalAckTests | 34 | LocalAck, LocalSpeaker | moves |
| MarkupSyntaxHiding | 0 | empty | native |
| MarkupSyntaxHidingTests | 5 | Exact host patch 0001 | native |
| MicAskTests | 2 | mic ask rule | moves |
| MicCheckTests | 8 | MicCheck | moves |
| MicFirstRunTests | 11 | mic permission mid-run | split |
| MicLevelTests | 4 | feedMic | native |
| NavigationTests | 6 | card stack, layout | moves |
| NoWaitingWordsTests | 1 | no "Loading" strings | moves |
| OnAirSoundTests | 9 | SpeakingVoice | native |
| PhoneRepairFixAccountTests | 1 | notebook rule | moves |
| PlaybackContinuityTests | 6 | PlaybackContinuity | native |
| PlayerControlsTests | 7 | PlayerControls | moves |
| PlayerDJTests | 1 | programme wiring | moves |
| PlayerRepairFixTests | 5 | RemoteAudioPlayer | moves with the audio element |
| PlayheadTests | 19 | Playhead, ProgrammePlayer | native |
| ProgrammePlayerRepair2Tests | 9 | ProgrammePlayer | native |
| ProgrammePlayerRepairTests | 10 | ProgrammePlayer | native |
| ProgrammePlayerTests | 34 | ProgrammePlayer | native |
| ProofKeyTests | 4 | ProofKey | native |
| ProvenanceTests | 0 | none left | moves (delete) |
| RadioFactsTests | 2 | RadioBridge | native |
| RepairTests | 12 | Concurrency, key host | split |
| RetrievalFactsTests | 9 | RetrievalFacts | moves |
| RoundThreeRepairTests | 9 | engine cancel, download | split |
| SharedGenerationTests | 8 | SharedGeneration | native |
| SourceLinkTests | 3 | SourceLink | moves |
| SpeakerGateTests | 13 | SpeakerGate | moves |
| SpeechFixesTests | 9 | UtteranceEnd | split |
| SpineTests | 1 | open set | moves |
| StretchDecoder | 0 | helper | moves |
| TakeSessionTests | 13 | TakeSession | native |
| TalkTests | 22 | engine, conversation, first answer | moves |
| TestConfiguration | 0 | helper | moves |
| TurnContextTests | 13 | DeskContext | moves |
| TurnPolicyTests | 54 | TurnPolicy | moves |
| TypedTurnTests | 5 | typedTurn | moves |
| VoiceChoiceTests | 29 | VoiceChoice | split |
| VoiceCoexistTests | 5 | route change with voice on | native |
| VoiceFrontTests | 4 | VoiceFront bound | moves |
| VoiceModelsTests | 17 | KnownModels, ModelStore | split |
| VoiceSetupTests | 8 | VoiceSetup | split |
| ListenerPipelineTests (O) | 14 | ListenerPipeline | moves |
| ModelListTests (O) | 5 | ModelList | moves |
| ModelOfferTests (O) | 5 | ModelOffer | moves |
| VoiceEnrolmentTests (O) | 7 | VoiceEnrolment | moves |
| KeptTurnsPruneTests (O) | 4 | KeptTurnStore.prune | moves |
| FluidSpeechMiniTests (O) | 9 | FluidSpeechMini | native |
