# Exact: what of audio and voice Exact can carry

Part of [EXACT.md](../EXACT.md). Written 2026-11-21 against Exact at `f53347503` (the checkout at
`mobile/.exact-src`; every `file:line` below is in that tree unless it says "ours"). Nothing here was run: no
build, simulator, test or device. Marks: **[src]** read in Exact's source, **[doc]** read in its docs, **[ours]**
read in our Swift, **[unrun]** a claim only a device can settle. Companion pages: [audio.md](audio.md) (the
session owner), [native-modules.md](native-modules.md), [deck.md](deck.md), [gaps.md](gaps.md).

The owner's question (2026-11-21): should the audio session, microphone, speech, on-device models and audio
player stay Swift; anything Exact can carry should move, because shared code is the cross-platform cost.

## Summary

| # | Capability | Verdict | What Exact gives, and what stays |
|---|---|---|---|
| 1 | Play a remote file or stream with seek, rate, position, end | **MOVES** | The Contract `audio` element: AVPlayer on Apple, Media3 ExoPlayer on Android, `<audio>` on web. `currentTime`, `playbackRate` 0.25 to 4, `timeupdate` every 0.25 s, `ended`. Cost: no clip end bound, pitch algorithm and rate range differ from ours |
| 2 | Play generated speech that arrives in chunks, gapless, with barge-in | **STAYS** | Nothing takes PCM from shared code. The `sound` arm plays WAVs declared at build, 10 s at most. System-voice speech has its own package (item 5) but no PCM |
| 3 | Background, lock screen, now playing, interruptions, route changes | **PARTLY** | Moves for what the `audio` element plays, on iOS fully; on Android the notification and session exist but carry no title, artwork or seek commands. Stays: the same for audio played by our own engine, and the interruption recommendation, which the `audio` element's owner never shows the page |
| 4 | Microphone as samples at a chosen rate, with permission, while playing, with echo cancellation | **PARTLY** | Exact owns the capture, the permission and a voice-processing switch (`@exact/media-capture`). Samples reach only native code (a package's track tap), never TypeScript. Our models stay native consumers of that tap; echo reference with playback outside our engine is unproven |
| 5 | Recognition, VAD, synthesis, running a model from shared code | **PARTLY** | Recognition moves (`@exact/speech-recognition`: SpeechAnalyzer on iOS 26, SpeechRecognizer on Android, foreground only). System-voice synthesis moves (`@exact/speech-synthesis`). VAD, speaker identity and every Core ML, llama.cpp or FluidAudio model stay native: Exact has no inference runtime. `@exact/language-model` is the platform's own model only |
| 6 | Adding native code | n/a (cost table below) | A package: one interface, a Swift class on Apple, a Kotlin `ExactExtension` on Android. Each native piece we keep is written twice |

Sizing over the 49 rows of the boundary list (section 8, estimates by file, not a line audit): **MOVES 646
lines, PARTLY 4,400 (about 3,550 of them portable or replaceable), STAYS 8,845**, of 13,891. Four more
audio-adjacent files not on the list (`RunSheetPlayer`, `PlayerControls`, `PlaybackSupport`,
`SegmentTracker`, 1,155 lines) call no audio API and are TypeScript-portable. First to move: remote audio
on the `audio` element, with the microphone check beside it.

## 1. Playing a remote file or stream: MOVES

- `audio "https://…" paused=… currentTime=… playbackRate=… timeupdate=… ended=…` lowers to the same media node
  as `video`, marked as an audio tag (`contract/lower/src/media.rs:93-100`). Declared ranges:
  `playbackRate` 0.25 to 4, `volume` 0 to 1, `currentTime` at least 0 (`media.rs:48-52`).
- **Apple** [src]: `host/apple/videoarm/VideoArm.swift` is an `AVPlayer`. It streams a URL (assets built at
  `:607-617`), seeks with zero tolerance (`:781-788`, so range requests land on the exact time), sets rate on
  change and emits `ratechange` (`:512-514`), reads `playbackRate` from the player (`:321`), and reports
  `timeupdate` on a 0.25 s periodic observer, only while a `timeupdate` listener is bound (`:255-266`). The
  end is `AVPlayerItemDidPlayToEndTime`, which emits `pause` then `ended` (`:729-739`). Events are
  `loadedmetadata, play, playing, pause, ended, waiting, seeking, seeked, ratechange, volumechange, canplay`
  (`docs/contract-grammar.md:2104`), plus `error`, `durationchange` and `timeupdate` (`apps/video-player/app.contract:128`).
- **Android** [src]: `host/android/video/src/main/java/dev/exact/video/MediaPlayer.kt` is Media3 ExoPlayer;
  speed and pitch are `PlaybackParameters(speed, pitch)` (`:227`, `:264`), `preservesPitch` read at `:259`.
- **Web** [doc]: `docs/reference.md:4540` (the media session table): the page's own media element and
  `navigator.mediaSession`.
- The source is a URL or `app:/` path only. The Apple player takes no request headers (`VideoArm.swift:110-121`,
  plain `AVURLAsset(url:)`); ours does not either (`RemoteAudioPlayer.swift:74,111`), so nothing is lost.

**Risks of moving, from what our Swift does and the element does not:**

1. **Clip end.** Ours places a boundary observer at the clip's end (`RemoteAudioPlayer.swift:133-142`).
   The element has no end bound. Shared code would pause when a 0.25 s `timeupdate` passes `end`: up to a
   quarter second of overshoot, and none at all while the page is frozen.
2. **Pitch algorithm.** Ours sets `.timeDomain` for speech (`RemoteAudioPlayer.swift:74`, "keeps the pitch, no
   warble"). The element uses `.spectral` when `preservesPitch` is true, else `.varispeed`
   (`VideoArm.swift:560`). Whether `.spectral` is as good for speech at 2x to 3x is a listening test.
3. **Rate range.** Ours clamps 1/32 to 32 (`PlayerControls.swift`, header); the element refuses a literal
   outside 0.25 to 4 at build (`media.rs:49`) and clamps computed values. Our own default range needs checking.
4. **Heard without sound.** The recorded bug
   ([remote-audio-is-recorded-heard-without-sound](../../problems/remote-audio-is-recorded-heard-without-sound-2026-11-19.md))
   is "ended" counted as "heard". The element's `playing` event and `state.mediaSession.readback` help, but
   whether a muted or silent route can be observed is unknown to us and to Exact's docs.
5. **A view in the tree.** The element is a view. Exact allows one with empty bounds to play, in a closed
   dialog too, and without `controls` it shows nothing (`docs/reference.md:4409-4411`;
   `media.rs:67-72`).

## 2. Generated speech in chunks, gapless, with barge-in: STAYS

Searched `docs/`, `contract/`, `host/apple`, `host/android`, `host/web-js` and `packages/` for `pcm`,
`AudioWorklet`, `AudioContext`, `AudioBuffer`, `scheduleBuffer`, `enqueue` and `stream` near `audio`.

- The `sound` arm plays WAVs declared at the top of the contract: "16-bit or float PCM, one or two channels, at
  most 10 s; the compiler reads it" (`docs/contract-for-agents.md:2980-2985`). A path built at run time plays
  only files a `sound` line declared. Nothing writes a buffer from TypeScript. Apple runs it on its own
  `AVAudioEngine` with a C mixer (`docs/reference.md:4328`), Android on AAudio (`:4329`).
- The `audio` element could play sentence-sized WAV files written to `app:/cache`, one after another. That is
  a new `src` each time: AVPlayer loads an item, and the gaplessness Exact builds is for `loop` only
  (`VideoArm.swift:485-492`, an item queued behind itself). A gap per sentence is likely and unmeasured.
  It also takes the audio out of our engine's reference tap (item 4).
- Barge-in "stop at once": the element stops when `paused` commits, one frame plus a command to the media
  server. Ours stops a player node inside the engine lock (`LiveAudio.swift:690-692`).
- `@exact/speech-synthesis` is system-voice text to speech with no PCM out (section 5).

Our engine keeps this: two player nodes, a time-pitch unit, a mixer tap that feeds the echo reference, and a
render-position playhead (`LiveAudio.swift:36-50, 367-370`; `Playhead.swift:1-40`). Moving it needs a
chunk-playback interface Exact does not have: a package of ours with `push(pcm)`, `stop()`, `position()`,
written once in Swift and once in Kotlin.

## 3. Background, lock screen, now playing, interruptions, routes: PARTLY

What Exact does for the `audio` element on iOS [src, doc]:

- `audio_session: "playback"` plus `host.ios.backgroundModes: ["audio"]` keeps it playing locked; the build
  refuses a media session without both (`docs/reference.md:4543-4555`). `AudioSession.playsInBackground`
  (`AudioSession.swift:96` in our notes) is true under those keys.
- `metadata=MediaMetadata(title, artist, album, artwork)` publishes `MPNowPlayingInfoCenter`; the bound
  `seekbackward`, `seekforward`, `seekto`, `previoustrack`, `nexttrack`, `stop` become `MPRemoteCommandCenter`
  commands; play and pause are always offered (`docs/reference.md:4530-4541`;
  `host/apple/videoarm/NowPlaying.swift`, 288 lines). Offsets via `seekbackwardOffset` and `seekforwardOffset`
  (`docs/contract-for-agents.md:3330-3345`).
- Interruption and route events reach **claims**, not elements: `interrupted`, `resumed`, `reset`, `route`
  (`audio.md`, "Events a claim hears"). The element hears the platform's own pause, and the page sees `pause`.
  The page never sees the resumption recommendation, and `VideoArm.swift` has no interruption handler (the
  only `interrupt` hit is a comment at `:557`). So B2 and B3 of our `PlaybackContinuity.swift:9-17` (resume
  only when the system recommends it) cannot be written in shared code against the element alone. A package
  claim (`context.audioSession.claim`) hears them; shared code would read them through a package source.
- While the element plays, shared code keeps running in the background: `page.frozen` is false on iOS with
  "active audio eligible for background execution" (`docs/contract-for-agents.md:5498-5506`). This is what the
  deck's journal wants (`deck.md`).

What Exact does on Android: `audio_session: "playback"` lets an audible `audio` element play through Home and
lock, with a Media3 `MediaSessionService` notification and a system pause control (`docs/reference.md:4407-4414`;
`host/android/app/src/playback/java/dev/exact/video/PlaybackService.kt:21`). But the player is given
`MediaItem.fromUri(uri)` and nothing else (`MediaPlayer.kt:226`), the notification title falls back to the app
name (`PlaybackService.kt:84`), and the table says "Contract metadata and custom actions stay in
`state.mediaSession`" (`docs/reference.md:4541`). I found no Android code that publishes title, artwork or
seek commands from the Contract. Headphones unplugged stop output (`:4388-4390`). So on Android the lock screen
is play and pause with the app's name, not our episode title or skip buttons [unrun].

For audio played by our own engine, none of this applies: Now Playing and remote commands are ours to add
(`audio.md`, "Now Playing and remote commands"), as `RunSheetPlayer` and `RemoteAudioPlayer.swift:240-271` do.

## 4. Microphone as samples, with echo cancellation: PARTLY

- `@exact/media-capture` (`packages/media-capture/exact.json`): `getUserMedia(audio, audioConstraints)`,
  `setTrackEnabled`, `stopTrack`, `microphoneStatus`, `enumerateDevices`. Constraints include `sampleRate`,
  `channelCount`, `echoCancellation`, `autoGainControl`, `noiseSuppression`.
- **Apple** [src]: `packages/media-capture/apple/MediaCapture.swift:1-27`: one `AVAudioEngine` for the
  process, input tap `installTap(onBus:0, bufferSize:4096)` (`:635`), claims the session as `.playAndRecord`.
  `echoCancellation`, `noiseSuppression` or `autoGainControl` true turns on voice processing, "one switch on
  Apple" (`:618`, `setVoiceProcessingEnabled`). Permission is `AVAudioApplication.recordPermission` read first.
- **Android** [src]: `MediaCapture.kt:1-27`: one `AudioRecord`; `echoCancellation` asks for
  `AcousticEchoCanceler` else the `VOICE_COMMUNICATION` source; with `device.microphone-background` it holds
  a foreground service of type `microphone` so it records while hidden (`:14-17`).
- **Web**: `packages/media-capture/web/index.js`, `getUserMedia`.
- **Where samples go.** Not to TypeScript. A track's consumers are native: Apple `ExactTrackConsumer.live(_ source:
  ExactAudioSource)` with `tap { AVAudioPCMBuffer, AVAudioTime }` on the audio thread
  (`host/apple/modules/ExactPackage.swift:908-982`); Android `TrackConsumer.live(AudioSource)` with
  `tap { ShortArray, Int }` of 16-bit interleaved PCM (`host/android/canvas/.../Tracks.kt:105-123`). Exact's
  own consumers are `@exact/recording` (file, level) and `@exact/speech-recognition`. TypeScript gets a
  file, a level (`audioLevel`, `packages/recording/exact.json:120`) or a transcript. The rate is the device's;
  the consumer converts, as ours does (`LiveAudio.swift:351-357`).
- **While audio plays.** Exact's capture is a separate engine from the `audio` element and from
  `AVSpeechSynthesizer`. Voice processing on Apple's I/O unit cancels what that unit renders. Ours turns it on in
  the one engine that also plays the programme (`LiveAudio.swift:344-347`), with a mixer tap as the reference
  signal (`:367-370`). Whether echo from an `AVPlayer` outside the capture engine is cancelled is **[unrun]**
  and could fail silently. The media-capture `hatch` (`MediaCapture.swift:42-45`) hands the app the capture's
  engine and a mixer so the app may "attach nodes after it", a possible route, but the engine exists only
  while a track holds a lease, so play-only needs another engine.
- Session states: our three (`playing`, `listening`, `talking`) are `speech-playback`, `default` and `voiceChat`
  claims (`AutoradioModule.swift:21-26`). Exact's capture claim is always `.playAndRecord`, first claim wins
  (`audio.md`, "Rules of composition").

## 5. Recognition, VAD, synthesis, models: PARTLY

**Recognition: moves.** `@exact/speech-recognition` (`packages/speech-recognition/exact.json`): `startSpeechRecognition(track=,
lang=, continuous=, interimResults=, processLocally=, phrases=)`, `stopSpeechRecognition`, and a polled
`speechRecognitionState` (`poll.default` 100 ms, `exact.json:149-151`) with interim and final results, alternatives,
confidence and time ranges.

- Apple (`SpeechRecognition.swift:4-17`): iOS 26 `SpeechAnalyzer` with `SpeechTranscriber`, else
  `DictationTranscriber`, on device; before 26 `SFSpeechRecognizer`. It takes `track=` (our mic), `path=` (a
  file) but `microphone=true` is refused. Language assets via `AssetInventory` (`:143-169`). This is what our
  `SpeechAnalyzerTranscriber` and `SFSpeechTranscriber` do (`AppleStages.swift:23-170`).
- Android (`SpeechRecognition.kt:1-22`): `SpeechRecognizer`; on device with `processLocally` (API 31); input is
  `microphone=true` only (a track or file is `NotSupportedError`); `continuous` restarts sessions at each end
  "since the class is not intended to be used for continuous recognition".
- **Foreground only on both:** "leaving the foreground aborts every one, its finals kept" (`SpeechRecognition.swift:27-28`,
  `.kt:19`; the `startSpeechRecognition` name is `foreground: true`). Our hands-free keeps listening with the
  app backgrounded (`PlaybackContinuity.swift:156-158`, mic given up on background but radio plays): check that
  we do not rely on background recognition.
- Poll, not push: partials arrive at best every 100 ms. Latency to the page is unmeasured **[unrun]**.
- Not available: Parakeet EOU streaming with end-of-utterance, partials at 160 ms chunks. Exact's result has
  `speech` and `sound` booleans (`SpeechRecognition.swift:262,484`) but they follow results, not audio.

**VAD, speaker identity, turn policy inputs: stay.** Searched for `vad`, `voice activity`, `speaker`, `diariz`,
`embedding`, `wake` across the tree: no VAD or speaker API. Silero VAD (`SileroVAD.swift`), WeSpeaker
(`Voice/WeSpeakerEmbedder.swift`) and the energy VAD in `BasicStages.swift` need PCM, which only a native track
consumer sees (section 4).

**Synthesis: system voice moves.** `@exact/speech-synthesis` (`packages/speech-synthesis/exact.json`): `speak(text, lang, voice,
volume, rate, pitch, audioSessionType)`, `cancelSpeech`, `pauseSpeech`, `resumeSpeech`, polled state with
word boundaries. Apple is `AVSpeechSynthesizer` in the application session (`SpeechSynthesis.swift:1-27`);
a `playback` queue speaks on with background mode; an interruption pauses the queue and only `resumeSpeech`
resumes. Android is `TextToSpeech` with audio focus; `pauseSpeech` stops and repeats a word on resume
(`SpeechSynthesis.kt:1-24`). It writes no PCM, so the speech does not pass through our reference tap, and our
`AVSpeechSynthesizer.write` path (`AppleStages.swift:172-296`, with its "cut off after about 13 seconds"
workaround at `:240-250`) has no Exact counterpart. Kokoro, PocketTTS and LFM voices (FluidAudio) produce PCM
and stay with the engine.

**Models from shared code: nothing documented.** Searched `onnx`, `coreml`, `llama`, `whisper`, `tflite`,
`litert`, `mlx`, `sherpa`, `parakeet`, `WebAssembly`, `worker`: only `packages/text-detection` and
`barcode-detection` (Vision), `@exact/language-model`, and Rust and wasm for data modules.

- `@exact/language-model` (`LanguageModel.swift:1-17`; `LanguageModel.kt:1-12`): Apple Foundation Models
  (iOS 26), Android ML Kit GenAI over Gemini Nano, web Prompt API. The platform's model only; not our GGUF or
  Core ML files. Android generates only in the foreground (`.kt:10-11`). Useful for the System 1 role only if the
  platform model is acceptable.
- A Rust data crate runs on iOS, Android and web (as wasm) and takes crates from the workspace lock
  (`docs/reference.md:6131-6170`). A crate that links an inference runtime would run on all three. **Nobody has
  tried this** and a data crate's contract is request and answer, no audio thread. Latency is unknown.
- Android native libraries: Maven coordinates in `host.android.maven` (`host/android/README.md:916-935`), pinned
  and locked, are documented; our own `.so` is not, apart from Rust's `jniLibs` (`:125`).
- Apple: SwiftPM products via `swiftPackages` (`docs/reference.md`; gaps.md, gap 19) would replace our copied
  xcframeworks of FluidAudio and llama.cpp. Not adopted or proven for these two.

## 6. How a native piece is added, and the cost on a second platform

A **package** is an `exact.json` interface (names, shapes, views, events) plus one implementation per platform
(`docs/recipes/native-module.md`; `native-modules.md`). Apple: a class conforming to `ExactPackage`, with
`later(name, args, preferred, reply)`, source reads polled and `context.changed(topic)` to ask again
(`ExactPackage.swift:453`), and `context.audioSession.claim(mode:options:background:events:)`. Android: a
Kotlin `ExactExtension` (`host/android/README.md:866-880`) with the same names, and in its `ExtensionContext`
`audioFocus.claim(type, attributes, pauseWhenDucked, events)` (`AudioFocus.kt:36`), `foregroundService.claim(…)`
(`ForegroundService.kt:159`) and `tracks` (`Extensions.kt:214, 218, 276`). Web: a `web/index.js`. The
app module (our `ModelsModule`) cannot take audio claims on Apple (`AudioSession.swift:42`, package scope);
our audio is therefore a package (`mobile/exact-package/apple`).

Push from native to the page is "ask again" only (gaps.md, gap 8); streams from a server use `exactStream`.
So a native piece should expose a snapshot struct and `changed` at a bounded rate.

What a second platform (Kotlin) must write for each piece we keep native:

| Native piece kept | Kotlin must write | Exact gives it for free |
|---|---|---|
| Chunk player with reference tap, playhead, barge-in | An `AudioTrack` or Oboe player with a timeline, a time-stretch unit (Android has `PlaybackParams`, not a pitch-kept 3x), render position, and the reference mix for AEC | nothing |
| Models: Parakeet, Silero, WeSpeaker, Llama, LFM, embedder | Equivalents of each runtime (FluidAudio is Apple-only; so ONNX Runtime or llama.cpp AAR), each with a Kotlin download and load | nothing |
| Mic consumer (VAD, ASR, gate fed from the track) | A `TrackConsumer` reading 16-bit PCM | capture, permission, AEC option, background service |
| Session claims | `audioFocus.claim` with the three states | the focus owner, foreground service |
| Lock screen for engine audio | A `MediaSession` of our own | only for `audio` elements, and then without metadata |
| Deck (journal, held rules) | Written in Kotlin ("Kotlin implements the same", `deck.md`) unless it moves to TypeScript | nothing |
| MailSeal, CredentialReader (not voice) | Keystore equivalents | `store` secrets (Android Keystore wrapped) |

Today the Kotlin side is empty (`deck.md`, "Android": the Kotlin deck does not exist). Every line of Swift
that stays is therefore a line owed in Kotlin, at least for the interface and the session/mic wiring; the
pure policy files (`Voice/TurnPolicy.swift` and its neighbours) are what TypeScript would carry once for both.

## 7. Where our Swift does something an Exact API would not (risks of moving), and what looks wrong

Subtle behaviour a move would lose or change:

- **One engine, one session, three states.** Programme, voice and mic share one `AVAudioEngine` so voice
  processing and the echo reference see all output (`LiveAudio.swift:36-50, 344-347, 367-370`). Moving playback
  to the element splits that.
- **Release then claim.** A state change is release, then claim, because the first claim's mode wins
  (`AutoradioModule.swift:31-36`; `audio.md`). The session briefly falls to ambient between them.
- **Own-change window.** A route loss within 4 s of the app's own session change is ignored, because voice
  moves a Bluetooth headset from A2DP to HFP (`PlaybackContinuity.swift:170-173`). An element-only design has
  no way to tell its own route change from the headset leaving.
- **Interruption memory.** `InterruptionMemory` (`PlaybackContinuity.swift:36-57`) resumes only on the
  system's recommendation and forgets on the reader's tap. See section 3 for why the element hides it.
- **Playhead from rendered audio** (`Playhead.swift:1-40`): position is source samples upstream of the time-pitch
  unit, frozen across an engine restart. The element's `timeupdate` is the item's time, 0.25 s.
- **Continuity across relaunch:** the run sheet and the deck journal are files read after first unlock
  (`deck.md`); the element has no relaunch story of its own.
- **Engine recovery:** a configuration-change observer with a 1 s ignore window (`LiveAudio.swift:405-410`), a
  mic retry ladder for a route still settling (`:388-399`), and a media-services rebuild (`:218-232`).

What looks like a bug or a workaround (not fixed here):

1. `AutoradioModule.swift:52` and `:58` call `AVAudioSession.setActive(true)` directly. The package should not
   touch the session; Exact's owner may overwrite it (`audio.md`, "The rule"). `reassert()` at `:58` is used
   after the background and interruptions, the very moments the owner changes the session.
2. `AutoradioModule.swift:23` asks for a `speech-playback` claim to get a playback session when nothing is
   speaking. It works because that claim's category is `.playback`, but it relies on an owner detail and
   `audio.md` lists "does the speech-playback claim hold for play-only" as an open question.
3. `RemoteAudioPlayer.swift:29` ignores a reported pause for 1.5 s after its own pause to tell the system's
   pauses from its own, and `scheduleRecheck` (`:34-43`) polls: a heuristic where the element's `pause` event
   has the same ambiguity.
4. `LiveAudio.swift:373` reads `UserDefaults "AutoradioProofSilent"` in the production start path to mute output
   for a Mac proof; a test switch in shipping code.
5. `LiveAudio.swift:495` waits `secs + 10` and `:543` polls 60 times at 50 ms: deadline guards around the
   engine, undefended against a long block at 3x or an engine that stays down.
6. `AppleStages.swift:240-250`: the end of a synthesized utterance depends on a 1 s quiet constant "inherited and
   not yet checked on a device".
7. The recorded defect: remote audio counted heard at its end regardless of sound
   (`remote-audio-is-recorded-heard-without-sound-2026-11-19.md`), and `ChoirPlayback.rendering()` testing a
   position, not output.
8. `PlaybackContinuity.swift:156-160`: on `didEnterBackground` voice gives up the microphone while Exact's
   recognition would abort anyway; the two paths must agree, and nothing says they do **[unrun]**.

## 8. Sizing: the boundary list, one line per file

`scripts/autoradio-native-voice-list.txt`, 49 `path` rows. Counts are `wc -l` of the file at `origin/main`
(`db31a7cd1`); the verdict is from the file's header and its calls, not a line audit. "m/s" is a rough
movable/stays split. Two non-voice rows (`MailSeal.swift` 72, `CredentialReader.swift` 35) are left out.

| File | Lines | Verdict | Why |
|---|---|---|---|
| LiveAudio.swift | 1120 | STAYS | the one engine: PCM chunk player, mic tap, AEC reference (section 2, 4) |
| Audio.swift | 174 | STAYS | PCM frames and buses |
| SessionCoordinator.swift | 361 | PARTLY 100/261 | states stay with the engine; mutex and logging go if claims are simpler |
| PlaybackContinuity.swift | 207 | PARTLY 90/117 | `InterruptionMemory` and notices are pure; the watcher reads engine and session |
| RemoteAudioPlayer.swift | 274 | MOVES | the `audio` element (section 1) |
| Playhead.swift | 85 | PARTLY 25/60 | `SpeechCut` text is pure; render position is the engine's |
| TurnEngine.swift | 787 | STAYS | native timing over frames and barge-in; portable only after an event boundary exists |
| Events.swift | 236 | STAYS | host clock and event log at audio rate |
| AppleStages.swift | 296 | PARTLY 150/146 | the two recognisers move; write-to-buffer synthesis stays |
| BasicStages.swift | 379 | STAYS | mic source, energy VAD, speech bounds on PCM |
| Stages.swift | 316 | STAYS | stage protocols over PCM and models |
| Core.swift | 246 | STAYS | descriptors for those stages |
| ScriptedStages.swift | 26 | STAYS | proof transcriber |
| HandsFree.swift | 328 | STAYS | turn policy driven by native stages |
| HandsFreeSwitch.swift | 73 | PARTLY 40/33 | one on/off state is TypeScript; opening the mic is native |
| LocalAck.swift | 376 | STAYS | on-phone model stream (llama) |
| LocalSpeaker.swift | 70 | STAYS | sentence cut over the engine's voice |
| MicCheck.swift | 62 | MOVES | `microphoneStatus`, `audioLevel`, speech-recognition on a track |
| VoiceChoice.swift | 265 | PARTLY 150/115 | catalog is data; fallback speaker is native |
| VoiceSampler.swift | 159 | PARTLY 80/79 | system voice via `speak`; neural voices via the engine |
| VoiceSetup.swift | 629 | STAYS | model download, load and measure |
| TurnTimeline.swift | 148 | STAYS | derived from the native event log; portable with it |
| MailReading.swift | 24 | MOVES | hands text to the voice; shared code can call `speak` or the deck |
| KeptTurns.swift | 160 | STAYS | mic recording of turns |
| FixtureTurn, FixtureTurnAudio, FixtureClip | 83 | STAYS | generated fixtures |
| Voice/ (8 files) | 1579 | PARTLY 1250/329 | `TurnPolicy`, gate, fusion, profile, contract are pure; WeSpeaker is Core ML |
| AutoradioModels/ (11 files) | 1798 | STAYS | FluidAudio, llama.cpp, Core ML (section 5) |
| autoradio-models/, autoradio-replay/ | 262 | STAYS | Mac proof commands, not phone code |
| Configurator.swift | 401 | STAYS | measures loaded models |
| ModelCandidates.swift | 143 | PARTLY 143/0 | a data table; the consumer is native |
| ModelStore.swift | 203 | STAYS | weights on disk, load and unload |
| KnownModels.swift | 205 | PARTLY 205/0 | catalog data (URLs, hashes) |
| ExternalModels.swift, LiveConfigBackend.swift | 52 | STAYS | model registrar and backend |
| LabRuntime.swift | 467 | STAYS | runs and measures the engine |
| Embedder.swift | 197 | STAYS | interface to the model; prompts are text |
| Choir/ChoirPlayback.swift | 80 | STAYS | engine binding for the deck |
| Deck.swift | 511 | PARTLY 300/211 | rules and journal are logic; engine binding and the locked-screen file stay |
| exact-package/apple/AutoradioModule+Deck.swift | 46 | MOVES | disappears with the deck's native names |
| TakeSession.swift | 240 | MOVES | `recording` (file, `audioLevel`), `speech-recognition` on a track, `audio` for review; the radio's gate coupling is the catch |
| Choir/ClipFingerprint.swift | 141 | PARTLY 110/31 | DSP is portable; decoding audio to PCM is native |
| Choir/ClipLocator.swift | 256 | PARTLY 200/56 | same |
| AutoradioKit/Package.swift | 53 | STAYS | native manifest |
| exact-package/apple/AutoradioModule.swift | 254 | STAYS | the package class and session owner (a Kotlin twin is owed) |
| exact-package/apple/AutoradioModule+Voice.swift | 119 | PARTLY 60/59 | voice cards |

Totals: **MOVES 646** (RemoteAudioPlayer 274, TakeSession 240, MicCheck 62, Deck binding 46, MailReading 24);
**PARTLY 4,400** (about 3,550 portable or replaceable, about 850 native); **STAYS 8,845** (13,891 in all; the
list's rows for the files counted once). Note the STAYS bulk is Swift that runs models and the engine. The
PARTLY bulk is pure policy whose inputs are native.

Not on the list, no audio API in them (0 to 2 `AV` or `MP` references each): `RunSheetPlayer.swift` 794,
`PlayerControls.swift` 58, `PlaybackSupport.swift` 135, `SegmentTracker.swift` 168: 1,155 lines that decide
what plays and are TypeScript-portable now (`deck.md`: "the player is deleted" when shared code commands the
deck).

## 9. Order, and the smallest experiment for each PARTLY

Order:

1. **Now, self-contained, Exact clearly carries it:** `MicCheck` onto `microphoneStatus` and `audioLevel`;
   `MailReading` to a shared call; the four pure player files to TypeScript behind the deck.
2. **Remote audio** (`RemoteAudioPlayer`, 274) onto the `audio` element, after experiment A.
3. **Recognisers** (`AppleStages` 150) onto `@exact/speech-recognition` for iOS 26 and Android; keep Parakeet.
4. **`TakeSession`** onto `recording` plus recognition plus the `audio` element.
5. **Wait:** the engine, chunk playback, VAD, speaker gate, every model, the turn engine. These wait for the
   newer models the owner will set up, and for a decision on a Kotlin chunk player. Keep their interface
   narrow (push PCM, stop, position; frames in, events out) so the pure policy can go to TypeScript later.

Experiments, smallest first:

- **A. The audio element on a real episode** (settles 1 and the Android half of 3). Builder, Exact loop: a page
  with `audio`, `timeupdate`, `currentTime`, `playbackRate=2.5`, a clip with `end`, on the web host; check
  clip-end overshoot logic against 0.25 s ticks. Lead, one simulator or phone run: lock screen title and
  skip buttons, an interruption (does it resume), a route loss, `.spectral` against `.timeDomain` at 2 to 3x
  by ear, and one Android device for the notification title.
- **B. Recognition from a track** (settles 5, recognition). Builder, Exact loop: `bun packages/speech-recognition/test/web.mjs chrome`
  (`packages/speech-recognition/test/device.md` step 6) hears a spoken file on Chrome's fake device. Lead,
  one simulator: `getUserMedia` then `startSpeechRecognition(track=)` on an iOS 26 runtime; record time to first
  partial against Parakeet, and the foreground abort.
- **C. Echo with playback outside the capture engine** (settles 4). Lead, one phone: mic track with
  `echoCancellation: true`, the `audio` element playing at speaker volume, capture the track through a hatch tap
  and measure residual against our reference-tap build. A simulator cannot answer this.
- **D. Chunked PCM into Exact's engine** (settles 2). Lead, one simulator: through `MediaCaptureModule.hatch`
  (`MediaCapture.swift:42-45`) attach a player node to the capture engine, schedule 200 ms buffers, count
  underruns; separately, two `audio` elements alternating WAV chunks to measure the gap.
- **E. A model in a Rust data crate** (settles 5, models). Builder, Exact loop: add an inference crate to a scratch
  data crate and run `bun exact.mjs lock`; if the lock and `cargo test` accept it, the lead runs one iOS build
  and one Android build and times a single inference through `native`-less `answer`. Only the compile question
  needs no device.
- **F. Poll latency** (settles 5, latency). Lead, simulator: `speechRecognitionState` at the default 100 ms
  and at 20 ms; time from a final result to a page commit.

## What I could not determine

- Whether Exact's `audio` element resumes after an interruption on iOS, and whether the page can learn the
  system's recommendation. Source shows no handler; only a run answers.
- Whether voice processing in Exact's capture engine cancels audio from a different player. Device only.
- Android lock-screen metadata for the `audio` element: no publishing code found; not run.
- Any measured latency for poll-based native-to-page streams, or for a model in a Rust crate.
- The line splits in section 8 are estimates from headers; `Voice/` was not read file by file.

## Sources

Exact at `f53347503`: `docs/reference.md` (4327-4560 sound, session, media session; 6131 Rust data),
`docs/contract-for-agents.md` (2979-2985, 3310-3345, 5483-5508), `contract/lower/src/media.rs`,
`host/apple/videoarm/VideoArm.swift`, `host/apple/Sources/ExactKit/AudioSession.swift`,
`host/apple/modules/ExactPackage.swift`, `host/android/README.md`, `host/android/video/`,
`host/android/app/src/playback/`, `host/android/canvas/.../Tracks.kt`, `AudioFocus.kt`, `Extensions.kt`,
`packages/{media-capture,recording,speech-recognition,speech-synthesis,language-model,calls}`. Ours: the files
named in section 8 at `origin/main` `db31a7cd1`. Read 2026-11-21.
