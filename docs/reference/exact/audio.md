# Exact: the audio session owner and claims

Part of [EXACT.md](../EXACT.md). Pinned commit `76cd50f9e`. Marks: **[run]**
verified by running (iOS 27 simulator, 2026-11-10), **[src]**, **[doc]**,
**[unrun]**. All line numbers are in
`host/apple/Sources/ExactKit/AudioSession.swift` unless a path is given.

## The rule

Exact has **one `AVAudioSession` owner**, `AudioSession` (`:42`), "the only
`setCategory` caller" (file header, `:1-39`). Everything that makes sound or
records (the sound arm, a `video`/`audio` element, a canvas's audio, a
package's capture, speech, a call) takes a **hold** or **claim** and the owner
composes one session from them. Our code must not call `setCategory` or
`setActive` itself; if it does, the owner's next change overwrites it.

Our access path is the package API `context.audioSession.claim(mode:options:background:events:)`
(`host/apple/modules/ExactPackage.swift:792`), which reaches the owner through
a C table (`host/apple/Sources/ExactKit/NativeHatches.swift:195-250`).
`AudioSession` itself is `package`-scoped and the app module cannot see the
claim API (`host/apple/modules/ExactExtensions.swift:374`) [src]. **[run]:**
the spike package claims and the session changes accordingly.

## Claim kinds

| Kind | API (package) | Category | Mode and options | Activated by |
|---|---|---|---|---|
| Hold (app sound) | internal: sound arm, `audio`/`video` element, canvas audio | `ambient` by default; `playback` if `audio_session:"playback"` | none | owner (`reconcile`, `:~300`) |
| Capture | `claim(mode:options:background:events:)` | `.playAndRecord` always | mode in `default`, `measurement`, `voiceChat`, `videoChat`, `videoRecording`; options below | owner at first claim (`:238-262`) |
| Speech | `claim(mode: "speech-transient"` / `"speech-transient-solo"` / `"speech-playback", ...)` | `.playback` | transient: `.voicePrompt` + duckOthers + interruptSpokenAudioAndMixWithOthers; playback: `.spokenAudio`; solo: default (`:~178`) | owner, only while no other I/O runs (`:201-231`) |
| Call | `claimCall(video:events:)` | `.playAndRecord` | `.voiceChat`/`.videoChat`, `allowBluetoothHFP` | the system (CallKit `didActivate`); never by the app (`:288`, `:315`) |

Options by name (bits in `NativeHatches.swift:236`): `mixWithOthers`,
`duckOthers`, `allowBluetoothHFP`, `allowBluetoothA2DP`, `defaultToSpeaker`,
`bluetoothHighQualityRecording`, `farFieldInput`,
`overrideMutedMicrophoneInterruption`. **Trap:** passing an empty options
array means "defaults", which are `defaultToSpeaker + allowBluetoothHFP`
(`NativeHatches.swift:239-243`). HFP on AirPods collapses playback to phone-call
quality. Always pass an explicit non-empty list.

## Rules of composition [src]

- **First claim wins.** While any capture claim is held, the category is
  `.playAndRecord` with the mode and options of the claim that started the
  running capture; later claims join with no reconfiguration (`:238-262`,
  `configuring`). So a mode change is **release every capture claim, then claim
  again**; the category changes when the first claim comes and when the last goes
  and nowhere else (`:263-269`).
- **After the last release** the owner restores what the app's own sound wants:
  `ambient` and deactivate when nothing holds (`restoreAfterCaptures`, `:270-285`),
  or the app category if a hold remains. With `audio_session:"playback"` but no
  Exact-side hold, the session therefore falls to `.ambient` and is given up
  (`.notifyOthersOnDeactivation`). If our own `AVAudioEngine` plays through the
  session, it needs a claim for as long as it plays. A play-only (no mic)
  state therefore needs either an Exact hold (a Contract `audio` element) or a
  speech-playback claim, since a capture claim implies a mic.
- **Speech never interrupts a capture.** While captures exist, speech plays in
  the capture's session (`:208`).
- Before any claim, with `audio_session:"playback"`, the category was
  `SoloAmbient`, not `playback`: the owner touches the session only at the
  first hold [run: first spike log line].

## The two states we want

| State | Claim we make | Engine | Expected route (unrun on phone) |
|---|---|---|---|
| **Listening**: programme at full quality, mic open, no voice processing | `claim(mode:"default", options:["defaultToSpeaker","allowBluetoothA2DP","mixWithOthers"], background:true)` | `AVAudioEngine`, input tap, `setVoiceProcessingEnabled` not called | speaker or A2DP out, phone mic in |
| **Talking**: echo cancelled, voice processing | release, then `claim(mode:"voiceChat", options:["defaultToSpeaker","allowBluetoothHFP"], background:true)` | `engine.inputNode.setVoiceProcessingEnabled(true)` before `start()` | HFP or speaker; programme quality degrades |

Both are implemented in
[`mobile/exact-spike/package/apple/Spike.swift`](../../../mobile/exact-spike/package/apple/Spike.swift)
(`start(talking:)`). [run, simulator]: the listening claim produced

```
SPIKE claimed listening in 363ms cat=PlayAndRecord mode=Default in=["MicrophoneBuiltIn"] out=["Speaker"]
```

(363 ms covers the claim call on the simulator, not the engine or a phone.)
Whether `.playAndRecord` + A2DP option keeps hi-fi output with the phone mic as
input is the question the phone spike must answer. `measurement` mode switches
off system gain/processing, which may suit the wake-word path; also unrun.

The spike's talking path is [unrun]. Time for the listening-to-talking switch
(two `setCategory` calls plus an engine rebuild with voice processing) and
whether it is audible has not been measured.

## Events a claim hears

`events: (String) -> Void` receives `interrupted`, `resumed`, `reset`, `route`
(`NativeHatches.swift:~250`, `audioEvent`). Sources (`:412-446`):

- iOS 27+: `didBecomeInactiveNotification` (system source only) and
  `resumptionRecommendationNotification`; earlier: `interruptionNotification`.
  An interruption begin tells every claim `interrupted`; an end with
  `shouldResume` reactivates the session (`reconcile`) then tells `resumed`
  (`:~385`). The app's own deactivation is not an interruption.
- `mediaServicesWereResetNotification`: owner re-applies the claim's category
  and mode (without activating), then tells `reset`.
- `routeChangeNotification`: tells `route` only. **Our engine must handle the
  engine configuration change and restart itself**; `MediaCapture` does this
  on `AVAudioEngineConfigurationChange` (comment at
  `packages/media-capture/apple/MediaCapture.swift:14-26`).

## Phone calls and CallKit

A plain phone call produces an interruption: claims hear `interrupted`, and
`MediaCapture` marks its tracks muted. `resumed` follows only if the system
recommends resuming. For apps that are calls, `@exact/calls` takes a call
claim (`packages/calls`); the system activates the session, and a capture
joins "in the call's configuration" (`:30-38`). We are not a calling app
[src]; what to do with an incoming call during a live conversation is a
product decision, not an Exact feature.

## Background

- `host.ios.backgroundModes: ["audio"]` writes `UIBackgroundModes: [audio]`.
  The grant line `device.microphone-background purpose.<key>` also writes
  `ExactCapturesInBackground: true` [run: both present in the built
  `Info.plist`, with `NSMicrophoneUsageDescription` from `strings/en.json`].
- `playsInBackground` (`:96`) is true only with the `audio` plist mode and one
  of: `audio_session:"playback"`, a capture claim with `background:true`, or a
  playback-type speech claim.
- `MediaCapture` mutes its tracks and drops its lease when the app leaves
  foreground **unless** `ExactCapturesInBackground` is set
  (`packages/media-capture/apple/MediaCapture.swift:307-322`). A package with
  its own engine is not subject to that logic but must pass `background:true`.
- Whether capture and playback continue with the screen locked for the spike
  is **[unrun]** (no device).

## Now Playing and remote commands

Exact publishes `MPNowPlayingInfoCenter` and `MPRemoteCommandCenter` only for
a Contract `audio` or `video` with `metadata=MediaMetadata(title=, artist=,
album=, artwork=)`; play and pause always, plus whichever of `seekbackward`,
`seekforward`, `seekto`, `previoustrack`, `nexttrack`, `stop` the element
binds (`host/apple/videoarm/NowPlaying.swift:1-40`, `:173-181`;
`docs/reference.md:1964-1998`) [src, doc]. The build refuses a media session
unless `audio_session:"playback"` and `host.ios.backgroundModes` contains
`audio` (`host/apple/build.mjs:995`). The owner is the claimant that most
recently played. An example from the docs:

```contract
audio "assets/episode.mp3" paused=paused currentTime=seek timeupdate=ticked
  metadata=MediaMetadata(title=`Episode 12`, artist="The Show", album="", artwork="assets/art.png")
```

If the programme plays through our own engine (the echo-reference design),
Now Playing is ours to add. The remote command registrations in
`NowPlaying.swift` are made for claimants only, so a custom registration is
unlikely to collide until an `audio` element exists in the same app [unrun].
Podcast long-form playback is a candidate for the built-in element.

## Open questions

- Does `.playAndRecord` with `allowBluetoothA2DP` keep A2DP output with the
  phone mic as input, on a real route?
- Mode switch latency and audibility, measured on a device.
- Is mid-playback `release` then `claim` audible as a gap, given the owner's
  `restoreAfterCaptures` momentarily falls to `ambient` and deactivates when no
  hold exists? Holding an Exact-side hold across the switch might avoid it.
- Does the owner's `speech-playback` claim hold a `.playback` category usable
  for play-only without a mic (the "no mic" listening state)?
- Do background capture and playback survive screen lock with our package's
  engine?

## Claim options and switch timing (lane `ar-integrate`, 2026-11-10)

Source: `origin/lane/ar-integrate` `mobile/README.md`, with `host/apple/Sources/ExactKit/NativeHatches.swift:233-243` [src].

- Option bits: `mixWithOthers 0x1`, `duckOthers 0x2`, `allowBluetoothHFP 0x4`,
  `defaultToSpeaker 0x8`, `allowBluetoothA2DP 0x20`, `overrideMutedMicrophoneInterruption
  0x80`, `farFieldInput 1<<18`, `bluetoothHighQualityRecording 1<<19`. Mode names
  accepted: `default`, `measurement`, `voiceChat`, `videoChat`, `videoRecording`;
  an unknown name silently becomes `default` (`NativeHatches.swift:233`).
- `playAndRecord` without `defaultToSpeaker` plays from the earpiece, so a
  listening claim carries it. Our claims omit `mixWithOthers` on purpose (a radio
  should take the session).
- Release then claim works on the simulator: listening to talking 228 to 349 ms,
  talking to listening 251 to 349 ms, including the engine stop and restart; the
  second claim's `AVAudioSession.mode` reads `VoiceChat` right after [run,
  simulator, not a phone number]. A category change stops the running
  `AVAudioEngine`: restart it and reinstall taps after every switch.
- Reported but unmeasured by us: 20 to 41 s route-switch stalls with AirPods on
  iOS 27.0.1; our adapter emits an `error` at 5 s [unrun on a phone].
- `Info.plist` for our lab carries `UIBackgroundModes: [audio]`,
  `ExactCapturesInBackground: true` (from the `device.microphone-background`
  grant), `ExactAudioSession: playback`, and the microphone and speech usage
  strings (from `strings/en.json`) [run]. That the programme keeps playing with
  the screen locked is not proven.
