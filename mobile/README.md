# Autoradio lab (`mobile/`)

Design: `docs/design/autoradio-lab-2026-11-10.md`. This directory holds `AutoradioKit`
(a host-agnostic Swift package, iOS 18+ / macOS 15+), the `autoradio-replay` command, and
(later) the Exact host app `AutoradioLab`. Mutation class: green (new tree, nothing in the platform runs it).

## Layout

- `AutoradioKit/` Swift package: events, stage protocols, registry, configuration, turn engine, replay, Apple-framework stages.
- `AutoradioKit/Sources/autoradio-replay/` the Mac command.
- `fixtures/` kept turns (tracked) used by the proof.
- `scripts/` `xe.sh` (run a command with the Xcode toolchain outside the Nix shell), `proof-replay.sh`.

## Ways the pieces can fail (written before the code, per the repo testing rule)

### Turn engine
1. Two turns overlap (second speech starts while the first is acknowledging): the engine must not resolve stages twice or interleave one record's events with another's. Rule: a new speech during acknowledging/holding is queued as `spokeAgain` and starts the next turn only after the phase returns to `playing`; the button always wins and ends holding.
2. A configuration change lands mid-turn: stages are resolved at the start of the turn and frozen; the change applies to the next turn. The record names the configuration that ran.
3. A stage throws or never returns (no transcript, desk silent): every await on a stage has a deadline; failure emits `error`, the turn still ends, the radio resumes. Never silence, never stuck.
4. Candidate speech turns out not to be a turn (noise, a cough, too few words): playback must return to normal and no record of an addressed turn is made.
5. Addressed but no speech (button tapped by mistake): `noSpeechTimeout` closes the turn.
6. Crash/restart: open turns are not resumed (repo rule); nothing is persisted mid-turn except the audio already written.
7. Clock: stamps come from one clock; in replay time is virtual and moves with the audio, stage durations are wall time added to the start stamp.

### Yield policy
1. Dip never returns (speech ended, state stuck at dipped): the policy is a pure function of the situation and the engine applies it every frame, so a silent frame with `notATurn` or no addressing returns to `play`.
2. Pause flaps on and off as the VAD toggles: once paused, stay paused until the engine says `notATurn` or the turn is over; speech seconds reset only when the VAD ends a speech.
3. A long dictation never pauses because the VAD hangover splits it: `speechSeconds` counts from the first speech start of the candidate segment, not the last VAD burst.
4. A cough pauses a programme: dip only after `dipAfter`, pause only when addressed or after `pauseAfter`; a segment that is not a turn resumes.
5. Dip lowers the reference signal too: the reference is what plays, so the engine records the level changes as `yielded` events.

### Replay
1. A different configuration changes the number of turns found (button mode with no button recorded): replay injects a tap at the first energy onset when the kept turn has no recorded inputs and the configuration is button-addressed, and says so in a `note` event.
2. Time runs ahead of the engine (fast mode): the bus delivers each frame to the engine in lockstep, so stamps follow the audio.
3. Stage timings are meaningless in virtual time: measured as wall time and added to the stamp.
4. The kept audio is not 16 kHz mono: it is converted on read.
5. The reference file is missing or a different length: it is optional and padded with silence.
6. A replay mutates the kept turn: it never writes into the turn folder; `--out` is separate.
7. Apple recognition needs authorization on the Mac: the command embeds usage strings; a missing authorization is an `error` event and a non-zero exit in the proof.

## The kit in one page

Everything a stage concludes is a `LabEvent` on one `EventLog` (monotonic host-clock stamps; audio frames are not events). A turn is kept as a `TurnRecord`: the folded events, the `LabConfiguration` that ran, device model, OS, app build, and the outcome (`heardThrough`, `spokeAgain`, `skipped`, `corrected`, `unknown`).

**Stage protocols** (`Sources/AutoradioKit/Stages.swift`), one per `StageKind`:

| Kind | Protocol | Shape |
| --- | --- | --- |
| `micSource` | `MicSource` | `inputPreference` (built-in or current route) |
| `addressing` | `Addressing` | `decide(_ AddressInput, now) -> AddressDecision`; `holdsTurnOpen`; `noSpeechTimeout` |
| `speechBounds` | `SpeechBounds` | `feed(frame) -> BoundsUpdate`, `reset()`, `speechSeconds(now)` |
| `speakerCheck` | `SpeakerCheck` | `isListener(frames) async -> Bool` |
| `transcriber` | `Transcriber` | `begin() -> TranscriptionSession` (`append`, `partials`, `finish()`, `cancel()`) |
| `yieldPolicy` | `YieldPolicy` | pure `target(for: YieldSituation) -> PlayLevel` (play, dip, pause) |
| `system1` | `System1` | `answer(System1Input{context, listenerWords, questions}) -> [System1Answer{option, score, ms}]` |
| `holder` | `Holder` | `lines(for: HolderInput) -> AsyncStream<String>` |
| `voice` | `Voice` | `speak(AsyncStream<String>) -> AsyncThrowingStream<AudioFrame>` |
| `desk` | `Desk` | `submit(DeskTurn) -> AsyncThrowingStream<Revision>` |
| `programme` | `Programme` | `load() -> ProgrammeDocument` (blocks: speech, clip, listenerPassage) |

Every implementation is a class conforming to `Stage` (`static stageID`, `static descriptor: StageDescriptor` with parameters, defaults, ranges, needs). To add a model:

```swift
registry.register(.transcriber, MyParakeet.self) { params in MyParakeet(params) }   // uses stageID and descriptor
```

`StageRegistry.builtIn()` registers lane K's stages; wave B builders add theirs after calling it (or inside `registerAppleStages`-style functions of their own). `LabConfiguration` is `{name, stages: {kind: {id, params}}}` JSON; the `TurnEngine` resolves a turn's stages at the start of the turn, so a change applies to the next turn.

`ModelStore` (fetch on demand to Application Support, pinned by SHA-256, resumable, unload what the configuration does not need) ships with no models registered.

Audio: stages never touch AVAudioSession. The host supplies an `AudioSessionOwner` (`claim(.listening|.talking, ...)`); every claim is logged as an `audioSession` event. The Exact app adapts Exact's claims; the replay command uses `NullSessionOwner`.

### Desk contract (not implemented; the server lane supplies it)

`ChoirDesk.submit` should POST the turn (`turnID`, `programmeID`, `blockIndex`, `listenerWords`) as a prompt at a place in the document and stream `Revision` values `{id, fromBlock, blocks, keepRest, isFinal}` back. No endpoint is invented here. `RemoteSystem1` posts `{context, listener_words, questions:[{id,text,options}]}` with `Authorization: Bearer <Keychain service news.choir.autoradio, account system1>` and expects `{answers:[{question_id, option, score}]}`.

## Running

```sh
mobile/scripts/proof-replay.sh                       # Mac end-to-end proof; exits non-zero on failure
mobile/AutoradioKit/.build/release/autoradio-replay --list-stages
mobile/AutoradioKit/.build/release/autoradio-replay --turn mobile/fixtures/turns/plain --config mobile/fixtures/configs/open.json
python3 -I mobile/scripts/make-fixtures.py           # regenerate fixtures (macOS: say, afconvert)
```

All Xcode/Swift commands run outside the Nix shell through `mobile/scripts/xe.sh`.

## The host app: Exact

`AutoradioLab/` is an Exact app (Contract screens: Player, Lab, Trace; no SwiftUI, no UIKit views of ours). AutoradioKit is attached as the Exact package `@choir/autoradio` (`exact-package/`): `scripts/sync-exact-package.sh` copies the kit sources in and writes `exact-package/apple/exact.json`; `scripts/gen-exact-interface.py` writes the interface.

Pinned Exact: `expo/exact` at `76cd50f9e863129b3fc75db1673d6ad4bbf79258`, checked out at `mobile/.exact-src` (gitignored), Bun 1.4.2 at `~/.bun-1.4.2`. Commands: `mobile/scripts/exact.sh ios` (simulator), `mobile/scripts/exact.sh ios --device --phone 00008120-00114DE80A90C01E`.

Signing: the free provisioning team is `QSGU6VRJL6` ("Yusef Nathanson (Personal Team)"); `N9HKM55LWY` is only the certificate's person id. Exact needs an installed profile for `news.choir.autoradio.lab`; `scripts/make-profile.sh` obtains one with a stub Xcode project and `-allowProvisioningUpdates`.

## Audio states: ways it can fail (written before the code)

The adapter holds one standing capture claim; the kit asks for `listening` or `talking` through `AudioSessionOwner`.
Policy (`LabConfiguration.audioPolicy`): `switchOnAddress` (default: listening, talking while a turn is open), `alwaysTalking`
(voice processing the whole run, the Mike-plugin shape), `alwaysListening` (phone mic, no voice processing, never switches).

1. The first claim's mode wins in Exact (`AudioSession.swift:238-262`): claiming `voiceChat` while the `default` claim is held changes nothing and the
   log still says "claimed". Rule: a switch is release, then claim, in that order, and the adapter reads `AVAudioSession.mode` afterwards and emits
   it; a mismatch is an `error` event.
2. An empty options list silently means `defaultToSpeaker + allowBluetoothHFP` (`NativeHatches.swift:239-243`): every claim passes an explicit list.
3. `playAndRecord` without `defaultToSpeaker` plays the programme from the earpiece receiver: listening carries `defaultToSpeaker` too.
4. The category change stops the running `AVAudioEngine`: after every switch the engine is restarted, taps reinstalled (voice processing changes the
   input and mixer formats) and the programme node resumed. A programme block that was mid-buffer continues from the frames already rendered, not from
   the start and not skipped.
5. `setVoiceProcessingEnabled` throws or is unsupported (simulator, some routes): `error` event, engine still starts without it.
6. A release leaves no claim for a moment: the session falls to `.ambient` and is deactivated (`restoreAfterCaptures`). The engine must be stopped before
   the release and started after the new claim, never run across the gap.
7. Interruption (a phone call) stops the engine and Exact reactivates the session only when the system recommends resuming: on `interrupted` pause the
   programme and mark the engine down; on `resumed` rebuild and restart; with no `resumed` the app says so in `message` and a tap on Start audio recovers.
8. Route change (AirPods in or out): the engine posts `AVAudioEngineConfigurationChange` and has stopped; restart taps and engine once, debounced, and
   emit the new route. A switch caused by our own claim must not trigger a second restart.
9. Media services reset: every AVAudioEngine and node object is invalid. The engine is rebuilt from scratch; the programme position is kept by block.
10. A switch is slow (20-41 s reported with AirPods on iOS 27.0.1): the claim runs off the main actor's critical path, the turn engine proceeds, a
    watchdog emits `error` at 5 s and the latency is recorded in the turn record (`audioSwitchMs`).
11. Player controls before Start audio: a player node on a stopped engine raises an Objective-C exception. All node calls go through `LiveAudio` and
    do nothing while the engine is down.
12. Under the Exact agent no audio hardware is opened (`context.agent`): the runtime runs a virtual engine, which plays by waiting and takes mic frames
    from a kept turn, so Start audio, Talk and Trace are driven without a device.

## Yielding live: ways it can fail (written before the code)

The `YieldPolicy` is pure; the engine applies it and `ProgrammePlayer.setLevel` drives the nodes (dip = programme node volume 0.25, pause = node pause).

1. Dip changes `programmeNode.volume`, but a restart of the engine (session switch) rebuilds the node's output: the volume is kept in `LiveAudio` and re-applied in `bring`.
2. Pause then `play` replays from the node's queue: after an engine restart the queue is gone, so playback continues from the rendered-frame offset (audio-state item 4).
3. Pause is applied while the user has paused (`userPaused`): resume must not start playing.
4. The yield event and the audible effect disagree (event says dipped, node still at 1.0): the Lab screen shows the node's real volume and audibility (`audio-level`), and the proof reads them while a turn is held.
5. Under the agent there is no node: the virtual engine keeps the same state variables, so the events and the numbers are the same, but nothing proves the speaker. Only the phone does that.

## Background and lock screen: ways it can fail (written before the code)

1. The Info.plist lacks `UIBackgroundModes: audio` or `ExactCapturesInBackground`: the process suspends on lock. Checked in the built plist (below).
2. `playsInBackground` is true only with the `audio` mode and a capture claim with `background: true` (`AudioSession.swift:96`): both claims carry `background: true`, including after a switch.
3. A switch while locked: the release leaves no claim for an instant and the session is deactivated (`restoreAfterCaptures`); iOS can then refuse to reactivate from the background. Unmeasured; on the phone's list.
4. The microphone cannot be opened from the background, so a locked-screen session must already be live: Start audio before locking, never a first claim after.
5. Interruption while locked (a call) with no `resumed`: the programme stays silent. The Lab screen shows engine "down" on unlock.
6. Now Playing and the lock-screen controls belong to Exact's own audio element (`UIBackgroundModes` and the remote command centre are Exact's). Our engine's `MPNowPlayingInfoCenter` and `MPRemoteCommandCenter` calls in `ProgrammePlayer` are not what Exact shows; building that is out of scope here. What it would take: an Exact `audio` element (a hold the owner knows) carrying the programme, or a package-side `MPNowPlayingSession` plus an active remote command target, with the app category `playback` and an activated session; Exact would need to not fight over `MPRemoteCommandCenter`.

## Driving the app and proving it (`scripts/proof-sim.sh`)

```sh
mobile/scripts/setup-exact.sh                  # pinned Exact checkout at mobile/.exact-src, Bun 1.4.2 at ~/.bun-1.4.2 (EXACT_REPO=<local clone> to avoid the network)
export EXACT_SIM=$(mobile/scripts/sim-device.sh)   # a simulator of your own (see below)
mobile/scripts/build-lab.sh ios                # sync the kit into the package, build, install on $EXACT_SIM
mobile/scripts/exact.sh agent ios tree "tap tab-lab" "clock data" tree
mobile/scripts/proof-sim.sh                    # last line PROOF OK or PROOF FAILED at <step>; NOBUILD=1 skips the build
```

**Why "the app never opened its agent socket" (cause, from Exact's source).** Three things, all of them outside our package:

1. *Two drivers on one simulator.* Exact allows one app per bundle id per simulator and every launch uses `simctl launch --terminate-running-process` (`scripts/agent.mjs:529`, `:569`). Another builder's drive of `news.choir.autoradio.lab` on the same simulator kills our running copy: the driver sees "the app hung up (exit code 0)" (the exit is `Agent.exitAfterLanding`, `Agent.swift:140-146`, which runs when the socket closes: it is the other driver's launch, not a crash) or never gets its socket. Fix: `sim-device.sh` creates and boots a simulator of our own; `exact.sh` passes `EXACT_SIM` through its `env -i`.
2. *The socket opens late.* The app binds it only after the first frame is applied and, on the simulator, after the scene is foreground-active (`ExactIOS/main.swift:134-141`, `AgentFrontIOS.swift:29-45`, up to 15 s); the driver gives up after 20 s (`agent.mjs:567-579`). A cold first launch on a machine at load 400 (several builders compiling) misses that; the second launch is warm. `proof-sim.sh` retries a refused launch.
3. *A system alert holds the foreground.* The speech recognizer asked for authorization at its first use, which put the iOS permission alert over the app; it stays after the app is killed, and every later launch is then not foreground-active and never opens its socket. `simctl privacy ... grant all` does not cover speech recognition (`simctl privacy` has no speech service). Fix in the kit: while the runtime is virtual (driven by the agent) the transcriber does not ask (`LabEnvironment.promptsAllowed`); restarting the simulator clears a stale alert.

The agent's tree shows `[hidden]` on elements of tabs that are not showing, but they are all in the tree: assertions read text by test id, and the proof orders its checks along the one transcript.

**Virtual audio.** The Exact agent plays nothing and opens no device (`state.sounds.output = "agent"`). Under the agent `labStart` starts `LiveAudio` in virtual mode: the same events, claims (those are real, on the simulator's session), yield levels and playback state, with playback as a timed wait and no `AVAudioEngine`. The mic is whatever is pushed into the bus: the Lab screen's "Inject fixture turn" button presses talk, feeds the bundled kept turn (`FixtureTurn.swift`, from `fixtures/turns/plain`) as 32 ms frames in real time, adds silence, and releases: the whole live engine path runs, and the turn lands in Trace like a spoken one. The same button on a phone injects through the real engine, which is a useful smoke test of the live path with a known utterance.

## Trace export

`Export JSON` in Trace writes the turn record to `Documents/AutoradioExports/<id>.json` in the app container, and also copies it to `app:/data/exports/` and offers Exact's `share` hatch (`share(title=, files=[path])`, `Share.swift:1-30`): the system share sheet on a phone; under the agent a hold, never a sheet. Without the sheet, pull it:

```sh
xcrun devicectl device copy from --device 00008120-00114DE80A90C01E --domain-type appDataContainer \
  --domain-identifier news.choir.autoradio.lab --source Documents/AutoradioExports/<id>.json --destination .
# simulator:
cp "$(xcrun simctl get_app_container $EXACT_SIM news.choir.autoradio.lab data)/Documents/AutoradioExports/<id>.json" .
```

## Background and lock screen (verified in the built Info.plist)

`AutoradioLab/.../Autoradio Lab.app/Info.plist` carries `UIBackgroundModes: [audio]` (from `app.json` `host.ios.backgroundModes`), `ExactCapturesInBackground: true` (from the `device.microphone-background` grant in `app.ts`), `ExactAudioSession: playback`, `NSMicrophoneUsageDescription` and `NSSpeechRecognitionUsageDescription` (from `strings/en.json`). Both claims pass `background: true`, so `playsInBackground` (`AudioSession.swift:96`) is true while a claim is held. What is not proved: that the programme keeps playing with the screen locked, and that a listening-to-talking switch while locked is allowed (the release leaves an instant with no claim). Both are on the phone list below.

Now Playing and the lock-screen transport belong to Exact's own audio element. Ours plays through our own `AVAudioEngine`, so the lock screen shows nothing for it, and `ProgrammePlayer`'s `MPNowPlayingInfoCenter`/`MPRemoteCommandCenter` calls are not what Exact displays. To get them: either (a) route the programme through an Exact `audio` element (a hold the owner knows, with Exact's media session), or (b) publish from the package: an `MPNowPlayingInfoCenter` entry plus `MPRemoteCommandCenter` targets, with the session active and the app category `playback`; Exact's owner must be told not to fight it. Not built.

## To measure on the phone

The phone (iPhone 14 Pro Max, `00008120-00114DE80A90C01E`) refused the launch until the developer certificate is trusted on it (Settings, General, VPN & Device Management). Nothing below has been measured on a phone. Build and install: `mobile/scripts/build-lab.sh ios --device --phone 00008120-00114DE80A90C01E`. Collect a turn afterwards with the `devicectl` command above (Trace, Export JSON). Every number is read from the Lab screen (audio state, last switch ms) or the turn record (`audioSwitchMs` in `record.summary`).

1. **Switch latency listening to talking and back.** Lab, pick `Switch on address`, Start audio, hold Talk, release. The record's `audioSwitchMs` is the sum of both switches; the events `audio switch listening -> talking in N ms` give each. Run on the speaker, on wired headphones, on AirPods. Repeat 10 times per route; report median and worst.
2. **The 20 to 41 s route-switch stall reported with AirPods (iOS 27.0.1).** AirPods connected, `Switch on address`, Start audio, hold Talk: the engine restarts onto HFP. Watch for `audio switch ... took N s` (an `error` event fires above 5 s). Compare with `Always talking` (one switch at start only) and `Always listening` (none).
3. **Voice processing's effect on playback quality.** Play the same programme clip in `Always listening` and `Always talking` on the speaker and on AirPods; record the speaker output with a second device, or listen. Note the route in `audio-route` (in/out ports) and `voice processing on/off`.
4. **Background capture.** Start audio, lock the screen, wait 5 minutes: does the programme keep playing, does the mic indicator stay on, does an injected turn (not possible locked) or a spoken turn after unlock still land in Trace? Then lock during a held turn and release.
5. **Yielding on the speaker.** Inject fixture turn (or speak) while the programme plays: confirm by ear that it dips at about 1 s and pauses when the utterance counts; `audio-level` shows the node's real volume.
6. **Interruption and route change.** Phone call during playback: events `audio interrupted`, then `resumed` if the system recommends; engine `up` again. Plug and unplug AirPods during playback: one `audio route` event and one restart.

## For EXACT.md (what the reference pages lack or get wrong; Exact source at 76cd50f9e)

1. **The "open problem: the simulator exit after the claim" in `testing-and-debugging.md` is not an audio bug.** "the app hung up (exit code 0)" is `Agent.exitAfterLanding` (`host/apple/Sources/ExactKit/Agent.swift:140-146`) running because the driver's socket closed; a second driver launching the same bundle id on the same simulator (`--terminate-running-process`, `scripts/agent.mjs:569`) does exactly that. Hypotheses (a) to (c) there are not needed. Give every driver its own simulator (`simctl create`).
2. **When the agent socket opens**: after the first frame is applied (`ExactIOS/main.swift:117-141`, `exactSession(didChange:)`) and after the scene is foreground-active and the window key (`ExactKit/IOS/AgentFrontIOS.swift:29-45`, up to 15 s); the driver's bound is a fixed 20 s (`scripts/agent.mjs:567-579`), not configurable. A cold launch on a loaded machine, or any system alert over the app, misses it ("the app never opened its agent socket"). `simctl privacy grant all` does not grant speech recognition (no such service); an unanswered speech alert survives the app being killed and blocks every later launch until the simulator is restarted.
3. **A package's `context.agent` does not silence the audio session**: under the agent our claim was real (`cat=PlayAndRecord mode=Default in=[MicrophoneBuiltIn] out=[Speaker]` on the simulator, `AudioSession.swift:238`); only Exact's own sounds are replaced (`state.sounds.output = "agent"`). An `AVAudioEngine` we start ourselves under the agent opens the simulator's audio hardware; `AVSpeechSynthesizer.write` on `@MainActor` stalled the main thread on a fresh simulator until the drive's `clock` op timed out at 120 s (`agent.mjs:85`, `REPLY_MS`). The package must branch on `context.agent` for anything that can block the main thread.
4. **Claim options**: the bit order and raw values are in `NativeHatches.swift:236-243` (`mixWithOthers 0x1`, `duckOthers 0x2`, `allowBluetoothHFP 0x4`, `allowBluetoothA2DP 0x20`, `defaultToSpeaker 0x8`, `bluetoothHighQualityRecording 1<<19`, `farFieldInput 1<<18`, `overrideMutedMicrophoneInterruption 0x80`); mode names accepted: `default`, `measurement`, `voiceChat`, `videoChat`, `videoRecording` (an unknown name silently becomes `default`, `NativeHatches.swift:233`). `audio.md`'s listening claim lists `mixWithOthers`; ours omits it on purpose (a radio should take the session) and adds nothing else. `defaultToSpeaker` matters: `playAndRecord` without it plays from the earpiece.
5. **Release-then-claim works and is fast on the simulator**: listening to talking 228 to 349 ms, talking to listening 251 to 349 ms, including the claim call, the engine stop and the engine restart (the kit's `audioSwitch` events); the second claim's `AVAudioSession.mode` reads `VoiceChat` right after. Not a phone number.
6. **Mutation results are options**: `mutation exp as shape X` reads as `option<X>`; a field read is `type-not-a-record`. Use `derive v = match exp { case some(e) => e.field, case none => "" }`. `share(title=, files=[path])` takes a path held in a derive and compiles.
7. **`tree` right after a navigation** carries the note "N requests in flight ... start with `clock data`": always issue `clock data` before reading data-bound text. The tree lists the panels of every tab (hidden ones marked `[hidden]`), so an assertion on text must not depend on which tab shows.
8. **Pointer drive**: `tap <id> down` then `tap up` (the bare `tap up`, not `tap <id> up`) delivers `pointerdown` and `pointerup` to a `Pressable`; `tap up` with a target is refused.
9. **The Exact build log**: a first `ios` build in a new worktree took about 5 minutes (cargo 220 s, swift 128 s); the exit 128 seen once was a transient (a second run succeeded). The driver rebuilds nothing itself when the build is current.
