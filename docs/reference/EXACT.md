# EXACT: the app shell runtime for Autoradio

Observed: `expo/exact` at commit `76cd50f9e863129b3fc75db1673d6ad4bbf79258`
(2026-11-10), Xcode 27.1, iOS 27 simulator, on an M1 Mac. The earlier handoff
read `94b8671`; that pin is stale. Exact is pre-1.0 with "no API stability, no
backwards compatibility, no migration guide, by design" (Exact `README.md`).
Pin a commit and move the pin on purpose.

Owner ruling (2026-11-10): the app shell is built on Exact, with no SwiftUI
host ("no swiftui please. lets do all exact"). Our voice and playback pieces
are Swift modules inside it. This page and the subpages are the reference an
agent reads before touching Exact.

Evidence marks used everywhere in this set:

- **[run]** verified by running here (simulator, 2026-11-10), with the command
  or output named.
- **[src]** read from Exact's source at the pinned commit. Paths are relative
  to the Exact repo root; `file:line` is at the pinned commit.
- **[doc]** stated in Exact's docs, not checked against source or by running.
- **[unrun]** reasoned or planned, not run.

Our real example code is tracked in
[`mobile/exact-spike/`](../../mobile/exact-spike/README.md): one Swift package
and one app. Examples in these pages are lifted from it unless stated.

## What Exact is, in one page

A cross-platform runtime. You write the interface in **Contract**
(`app.contract`), a small declarative language that uses CSS names and CSS
layout, and the data logic in TypeScript (`app.ts`, run on Hermes on native
hosts) or Rust. The compiler lowers Contract to a **plan** (tables plus
bytecode). On iOS a Rust **runner** executes the plan, a Rust **kernel**
(Taffy, CSS rules) computes every box, the host measures text with CoreText,
and **UIKit views** present the result. No webview, no JS bridge in the UI,
no app JavaScript before first pixel [doc: `README.md` "How it works"].

The loop: `view` reads `state`, `derive` and `resource` values. A control's
event runs an `action` (a short, loop-free block that writes state together at
commit). A `resource` is a reactive request to a **source**, answered in
`app.ts` (or Rust, or a native module). A `mutation` is an explicit
request (`send`). Native code reaches the app through a **package** (declared
names, views, events) or the app's one **native module**.

Where Swift sits: nowhere in the UI. Swift is a plug-in: native views (any
`UIView`), calls (`later`, `call`), and device-level work (audio, sensors).
There is exactly one app module per app and any number of packages.

## Surprises (the ten that matter)

1. **No SwiftUI, no UIKit authoring.** Screens are Contract nodes (`column`,
   `row`, `text`, `list`, `button`, `svg`, `canvas`). UIKit draws them, but you
   never write a view controller unless you add a native view.
2. **The app module cannot reach the audio session; only packages can.**
   `AudioSession` is `package`-scoped in `ExactKit`
   (`host/apple/Sources/ExactKit/AudioSession.swift:42`), and the claim API is
   handed to package implementations only (`host/apple/modules/ExactExtensions.swift:374`,
   `host/apple/modules/ExactPackage.swift:777-792`) [src]. Our voice code must
   be a package. [run: the spike package claims through it.]
3. **The first claim's mode and options win while any claim is held.** A mode
   switch is release every claim, then claim again
   (`AudioSession.swift:238-262`) [src].
4. **Every capture claim is `.playAndRecord`.** Exact has no "record only"
   claim, and no "playback only, full quality" claim for the mic case. The
   listening state (hi-fi playback plus mic) is `.playAndRecord` with mode
   `default` and no HFP; whether that holds A2DP quality is unrun on a phone.
5. **There is no log of category changes.** The owner only tells claim holders
   interruption, resume, reset and route events
   (`AudioSession.swift:437-446`); the host logs one line per capture claim
   (`host/apple/Sources/ExactKit/NativeHatches.swift:221`) [src]. We log the
   category ourselves [run].
6. **Now Playing and remote commands exist only for a Contract `audio` or
   `video` element with `metadata=`** (`host/apple/videoarm/NowPlaying.swift:1-11`)
   [src]. If we play through our own `AVAudioEngine` we add
   `MPNowPlayingInfoCenter` ourselves and must avoid fighting Exact's remote
   command registrations.
7. **Bun must be 1.4.2 exactly.** 1.3.14 is refused (`scripts/bun.mjs:19`)
   [run]. And `exact.mjs setup` downloads Hermes for every platform (Android,
   Linux, Windows, tvOS, about 190 MB each) and stalled for us. Install only
   the three Apple targets by hand (see `exact/build-and-ship.md`) [run].
8. **Exact never makes provisioning profiles.** It signs with a profile that
   already exists on the Mac. A free account therefore needs the owner signed
   into Xcode (Settings > Accounts) so Xcode can create one for our bundle id.
   Without that, `xcodebuild -allowProvisioningUpdates` fails "No Account for
   Team" [run].
9. **The test driver owns the clock.** Between two driver operations nothing
   moves. `clock settle` and `clock +N real` advance things. A tone or an
   audio engine runs on real time, so drive it with `clock +N real` [doc,
   partly run].
10. **Over-the-air logic delivery does not exist.** `deploy.store.ios: "0"`
    links no update store. With `"A"` only signed plans and assets update;
    "signed TypeScript and Rust module delivery isn't implemented"
    (`scripts/app.schema.json:1003`) [src]. Everything ships as a binary.

Honourable mentions: the first cold build is about 15 minutes (Rust 5m37s,
Swift host 48s) and the `target/` of one Exact checkout plus one app is 7.6 GB
[run]; actions have no loops or `await` and read state as it was at their
start (`docs/contract-for-humans.md:260-324`) [doc]; an `else empty()` clause
on a `resource` is how you give a source a placeholder [doc].

## Decision guide: Exact UI model or a Swift module

| Need | Write it in | Why |
|---|---|---|
| Screens, lists, forms, navigation, settings | Contract | It is the model; no Swift needed |
| A timeline or waveform that is lines, rects, text | Contract `svg` | Part of the model; unproven for hundreds of nodes |
| Per-frame custom drawing | `canvas surface=...` or a package `UIView` | Canvas 2D on Core Graphics; or any `UIView` from a package |
| Network calls, JSON, small state machines | `app.ts` (Hermes) | Grant-checked `fetch`, SQLite, fs |
| Audio session, `AVAudioEngine`, mic, wake word | Swift package | Only packages reach the owner's claims |
| Streams into the UI (RMS, partial transcripts) | Swift package + `context.changed` | The only push path is "ask the source again" |
| Playback with lock-screen controls | Contract `audio` + `metadata=` first | Exact's Now Playing exists only there |
| Anything needing a system object (CallKit, CoreML) | Swift package | Same |

## Patterns we will reuse

Screen, list, form, sheet and navigation patterns live in
`exact/ui-model.md`. The native-stream-to-UI pattern (a Swift module producing
a level meter or a transcript) lives in `exact/native-modules.md`, with the
spike's `Spike.swift`. The background-audio app pattern (permissions, plist
keys, claims) lives in `exact/audio.md`.

The shape in one paragraph: a package declares one live source (`spikeStatus`)
and one send (`spikeMode`). The Contract view reads the source as a
`resource` and calls the send from an action. The Swift side keeps a snapshot
struct, mutates it on its own queues, and calls `context.changed(...)` at a
bounded rate (once per second in the spike). The UI re-asks and redraws.

## Sharp edges (file:line at the pinned commit)

- `AudioSession.swift:238-262`: first claim configures; later claims join
  with no reconfiguration [src].
- `AudioSession.swift:201-231`: speech claims apply only while no other I/O
  runs; otherwise "next boundary" [src].
- `AudioSession.swift:96-101`: `playsInBackground` is true only with the
  `audio` plist mode and (a playback category, a background-allowed capture,
  or a playback speech claim) [src].
- `packages/media-capture/apple/MediaCapture.swift:307-322`: without
  `device.microphone-background`, capture tracks mute with reason
  `background` and the lease is released when the app leaves the foreground
  [src].
- `host/apple/Sources/ExactKit/NativeModule.swift:1-70`: the native module ABI
  is a versioned C table; every entry is on the main thread [src].
- `host/apple/modules/ExactNativeModule.swift:233-243`: `later` must reply
  once; a dropped reply fails the call [src].
- `host/apple/build.mjs:26-30`: device signing needs a profile on the Mac
  [src]; Exact does not create one.
- `host/apple/build.mjs:995`: a media session requires `audio_session:
  "playback"` and the `audio` background mode [src].

Full list with fixes: `exact/gaps.md`.

## Index

- [exact/ui-model.md](exact/ui-model.md): Contract, state, sources, layout, lists, inputs, navigation, custom drawing, theming, accessibility.
- [exact/native-modules.md](exact/native-modules.md): app module vs package, calling convention, streams out, threading, errors.
- [exact/audio.md](exact/audio.md): the session owner, claims, modes, background, Now Playing, interruptions.
- [exact/build-and-ship.md](exact/build-and-ship.md): layout, CLI, build, signing, install, pinning.
- [exact/testing-and-debugging.md](exact/testing-and-debugging.md): the driver, logs, failure messages.
- [exact/internals.md](exact/internals.md): architecture, execution, rendering, measured numbers.
- [exact/gaps.md](exact/gaps.md): what we need that is missing, with the cheapest fix.

## Rules we follow

1. Read this page before touching Exact. Never answer an Exact question from
   memory.
2. Pin the commit in `mobile/`; move the pin in its own commit with a rebuild.
3. Voice and playback code takes the session only through Exact's claims, and
   is written so the same code could run behind a different session owner
   (`AutoradioKit` takes a session-owner protocol; it never calls
   `AVAudioSession`).
4. Mark every new claim here **[run]**, **[src]**, **[doc]** or **[unrun]**.

## Open questions

- Does the spike's tone plus mic tap run on the phone, and survive a locked
  screen? (Blocked on Xcode account sign-in.)
- What ended the simulator process after the claim? See
  `exact/testing-and-debugging.md`.
- Does `.playAndRecord` with `allowBluetoothA2DP` hold A2DP playback quality
  with the phone's own microphone as input?

## Sources

`https://github.com/expo/exact` at the commit above, cloned 2026-11-10. Docs
read: `README.md`, `docs/start-here.md`, `docs/contract-for-humans.md`,
`docs/contract-for-agents.md`, `docs/reference.md`, `docs/testing.md`,
`docs/recipes/*.md`.
