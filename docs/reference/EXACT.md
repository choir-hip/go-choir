# EXACT: the reference for building Autoradio's app on Exact

Observed: `expo/exact` at commit `76cd50f9e863129b3fc75db1673d6ad4bbf79258`
(2026-11-10), Xcode 27.1, iOS 27 simulator, on an M1 Mac. The earlier handoff
read `94b8671`; that pin is stale. Exact is pre-1.0 with "no API stability, no
backwards compatibility, no migration guide, by design" (Exact `README.md`).
Pin a commit and move the pin on purpose.

Owner ruling (2026-11-10): the app shell is built on Exact, with no SwiftUI
host ("no swiftui please. lets do all exact"). This page and the subpages are
the single reference an agent reads before touching Exact: the Contract
language, TypeScript members, Swift packages and the app module, rendering and
motion, build, test and ship. A lesson learned by any lane goes here or in a
subpage, not in a lane README (see `docs/design/autoradio-app-ui-2026-11-10.md`,
section 6).

Evidence marks used everywhere in this set:

- **[run]** verified by running here (simulator, 2026-11-10), with the command
  or output named.
- **[src]** read from Exact's source at the pinned commit. Paths are relative
  to the Exact repo root (`mobile/.exact-src` in a checkout of this repo);
  `file:line` is at the pinned commit.
- **[doc]** stated in Exact's docs, not checked against source or by running.
- **[test]** read in a fixture or test in the tree, not run by us.
- **[inferred]** reasoned from reads.
- **[seen]** observed by the builders of our app on the simulator
  (2026-11-10 and 2026-11-11), not read in Exact's source and not checked by
  a second person. What happened to us, not an Exact guarantee.
- **[unrun]** planned or sketched, not run.

Our real example code is tracked in
[`mobile/exact-spike/`](../../mobile/exact-spike/README.md): one Swift package
and one app. Examples in these pages are lifted from it unless stated.

## What Exact is, in three paragraphs

**A runtime, not a UI toolkit wrapper.** You write the interface in
**Contract** (`app.contract`), a small declarative language that uses CSS names
and CSS layout, and the data logic in TypeScript (`app.ts`, run on Hermes on
native hosts) or Rust. The compiler lowers Contract to a **plan** (tables plus
bytecode). On iOS a Rust **runner** executes the plan, a Rust **kernel**
(Taffy, CSS rules) computes every box, the host measures text with CoreText,
and **UIKit views** present the result; `svg` becomes Core Animation layers.
No webview, no JavaScript bridge in the UI, no app JavaScript before first
pixel [doc: `README.md` "How it works"]. It does not compile to Metal; Metal
appears only in optional canvas, GPU-module and SVG-filter paths
(`exact/rendering-and-motion.md`).

**The loop.** `view` reads `state`, `derive` and `resource` values. A control's
event runs an `action` (a short, loop-free block that writes state together at
commit). A `resource` is a reactive request to a **source**, answered in
`app.ts` (or Rust, or a native module). A `mutation` is an explicit request
(`send`). Per-frame work belongs to `every(frame, ...)` tasks and to
transitions, keyframes and Core Animation, not to JavaScript.

**Where Swift sits.** Nowhere in the UI. Swift is a plug-in: native views (any
`UIView`), calls (`later`, `call`), and device-level work (audio, sensors). A
**package** declares names, views and events and can take audio claims. The
app has exactly one **app module** (any number of packages), which is where
third-party Swift and Core ML code enters as static xcframeworks. We may
write Rust where Contract and TypeScript cannot do the job, such as a wgpu GPU
module (rule 5 below, `exact/capabilities.md`); we do not extend Exact's engine
itself (`exact/native-modules.md`).

## Decision table: I need X, use Y

| I need | Use | Why / where |
|---|---|---|
| Screens, lists, forms, navigation, settings | Contract | It is the model (`exact/ui-model.md`) |
| A timeline or waveform of lines, rects, text | Contract `svg` | Part of the model; hundreds of nodes unproven |
| Entrance, exit, stagger, spring, layout reorder | `transition`, `keyframes`, `-exact-exit-animation`, `-exact-layout-transition` | Native, no TypeScript (`exact/rendering-and-motion.md` section 3) |
| Card drag with throw | `pan`/`panrelease` (Spark pattern) or a drag timeline | `rendering-and-motion.md` 8c |
| A flat card in 3D (flip, lean, tilt) | `rotate="y 40deg"` with `perspective` on the parent | Native; angle animates through `transition` |
| A spinning solid (the TetraMark) | Pure Contract `fn` projection plus `svg polygon` plus `every(frame)` (portable across hosts); a Swift package view with `CATransformLayer` only if measured too slow on the phone | No `preserve-3d`; `rendering-and-motion.md` 8a |
| Glass, blur, shadow, depth | `backgroundMaterial="glass"`, `backdrop-filter`, `box-shadow` | Native; `rendering-and-motion.md` 5 |
| Themes | bound data attributes for user themes; `app.tamber` for shipped modes | Binding proven upstream, unrun on device |
| Per-frame custom drawing | pure Contract first; then an Exact GPU surface; a package `UIView` last; `canvas surface=...` Canvas 2D (Core Graphics, calls Hermes each frame) as last resort | `rendering-and-motion.md` 2 and 8a |
| Network calls, JSON, small state machines | `app.ts` (Hermes) | Grant-checked `fetch`, SQLite, fs |
| Audio session, `AVAudioEngine`, mic, wake word | Swift package | Only packages reach the owner's claims (`exact/audio.md`) |
| Streams into the UI (RMS, partial transcripts) | Swift package plus `context.changed` | The only push path is "ask the source again" |
| Playback with lock-screen controls | Contract `audio` with `metadata=` first | Exact's Now Playing exists only there |
| A third-party Swift or Core ML library | static `.xcframework` in the app module | A package cannot import it (`exact/native-modules.md`) |
| A system object (CallKit, Core ML, PDFKit) | Swift package view or the app module | Same |
| Text wrapping around a figure, pull quote or side note | `wrap-flow="both"` and `shape-outside` (and `shape-margin`) on the obstacle box, beside a `text` body (apps/textflow) | `exact/capabilities.md` section 4; unrun by us |
| A shader surface (fragment-shader field, glass, a mark with lighting) | Exact wgpu GPU module: a Rust crate with WGSL, used as `canvas surface=name(args)`; portable to Metal, Vulkan and WebGPU | `exact/capabilities.md` section 2; Android draws nothing under canvas children; build cost unmeasured |
| A 3D or particle scene, physics | The game add-on, inside a GPU module, only if a scene graph, particles or physics are needed | `exact/capabilities.md` section 6; heavy, unmeasured |
| A read-only rich-text document, tappable citations | `text markup="markdown"` with atom pills and `atompress`; `textarea markup="markdown"` is the editor | `exact/capabilities.md` section 5; no documented footnote tap |
| Particles, Lottie, blend modes | none native; see the cannot-do list (particles: GPU module or game add-on) | `rendering-and-motion.md` 9 |
| A build or driver run | one shared build tree, one build, one simulator at a time | `exact/build-and-ship.md` |

## Rules we work by

- **As much in Exact as possible** (owner, 2026-11-11: "i would rather have
  as much in exact as possible"). Order of preference for any piece of UI or
  behaviour: Contract; then Exact's own capabilities (the wgpu GPU module,
  Canvas 2D, textflow, the Markdown editor, the game add-on), in Rust where
  they need it; then TypeScript members for data logic; a Swift package or
  UIKit view last, only where Exact has no path (microphone and audio
  session, on-device models, system services), and written so the same
  interface could be met on another host.
- **Contract first, for portability** (owner, 2026-11-11). UI, drawing and
  motion are written in Contract so the app can run on other Exact hosts
  (Android). A Swift package view is for what has no Contract path (audio,
  models, system services) or for an optimisation taken after a recorded
  measurement, with the Contract form kept.

1. Read this page before touching Exact. Never answer an Exact question from
   memory.
2. Pin the commit in `mobile/`; move the pin in its own commit with a rebuild.
3. Voice and playback code takes the session only through Exact's claims, and
   is written so the same code could run behind a different session owner
   (`AutoradioKit` takes a session-owner protocol; it never calls
   `AVAudioSession`).
4. Mark every new claim here **[run]**, **[src]**, **[doc]**, **[test]**,
   **[inferred]** or **[unrun]**. A sketch nobody built stays **[unrun]**.
5. We may write Rust (owner, 2026-11-11: "we can write rust"; this replaces
   the earlier "TypeScript, not Rust"). Contract and TypeScript stay the
   default because they rebuild in about a minute; Rust is for what they
   cannot do, such as a wgpu GPU module, and each Rust crate states its
   added build time.
6. Third-party Swift or Core ML code enters as a static xcframework in the app
   module, never as a package dependency.
7. Per-frame work stays out of Hermes: use frame tasks, transitions, keyframes
   or Core Animation.
8. One shared Exact build tree, one build at a time, one simulator per driver.
   Each worktree compiling its own copy overloaded the Mac.
9. An Exact lesson learned by any lane is written into this page or a subpage,
   with its source, not into a lane README.
10. Drive by `testId`, never coordinates; run `clock data` before reading
    data-bound text.

## Read these before building a screen

Seen in our build (2026-11-11); the pages named hold the detail and the
caveats.

- A proof's state must be set with the app's own controls after deleting its
  preferences plist; `defaults write` does not override the app
  (`exact/testing-and-debugging.md`).
- The driver refuses to tap anything the floating bar or keyboard covers, and
  taps need about 1.2 s between them (`exact/testing-and-debugging.md`,
  `exact/ui-model.md`).
- Native `buttonStyle` buttons cannot be themed; wrap one themed face in
  `button appearance="none"` (`exact/ui-model.md`).
- A scroll needs a `testId` to be driven, and a trailing spacer, not
  `padding-bottom`, was the reliable end inset (`exact/ui-model.md`).
- `text` collapses `\n` without `white-space="pre-wrap"`; `iframe` has no
  `srcdoc` (`exact/ui-model.md`, `exact/gaps.md`).
- A glass layer over a page needs an opaque backing or a scrim; a glass node
  under a text field covers it (`exact/rendering-and-motion.md`).
- One booted simulator, one build at a time; load above about 60 stops the
  driver (`exact/testing-and-debugging.md`).

## Known wrong ideas (corrected)

| Wrong idea | What is true |
|---|---|
| "Exact has no 3D transforms." | Box 3D exists: `rotate` with axis, `translate` z, parent `perspective`, `backface-visibility`. Missing: `preserve-3d`, `matrix`, `skew`, animating axis, z or perspective (`rendering-and-motion.md` 4) |
| "Contract compiles to Metal / GPU." | It compiles to a plan run by a Rust interpreter; UIKit (iOS), a `RenderNode` replay (Android), a vello/tiny-skia painter (Linux) or the DOM (web) presents it. Metal appears only in optional artifacts: the GPU module, vello Canvas 2D, SVG filters. The README line "a wgpu GPU module (Metal on Apple, WebGPU in the browser)" is about the optional module (`rendering-and-motion.md` 2b) |
| "Exact has no text flow around shapes." | It has: `wrap-flow`, `shape-outside`, `shape-margin` (apps/textflow). It is the basis for side notes and figures in a reader (`exact/capabilities.md` section 4) |
| "Exact cannot do shaders; we write no Rust." | We may write Rust (owner, 2026-11-11). A shader surface is a Rust crate with WGSL (`exact/capabilities.md` section 2.1) |
| "Data themes are unproven." | The binding mechanism is proven upstream; our bindings are unrun on a device |
| "Project the 3D mark in TypeScript on a canvas." | That is the worst ranked option: a Hermes call per frame. Use pure Contract; a `CATransformLayer` package view is an iOS-only optimisation |
| "The simulator exit after the audio claim is an audio bug." | "the app hung up (exit code 0)" is the driver's socket closing, usually a second driver on the same simulator (`exact/testing-and-debugging.md`) |
| "A package can depend on our SwiftPM library or a third-party one." | It is compiled with `swiftc` and its own files only; use the app module and static xcframeworks |
| "A `pan` event gives the total drag." | It gives the move since the last event |
| "Select any route as a tab." | The iOS host builds tabs only from `tablist` tabs; push other routes |
| "Over-the-air logic updates exist." | They do not (`scripts/app.schema.json:1003`); everything ships as a binary |
| "`agent ios` is safe with two drivers." | Two drivers on one simulator kill each other's app |

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

## Patterns we reuse

Screen, list, form, sheet and navigation patterns live in
`exact/ui-model.md`. Motion, 3D, drawing and theme recipes live in
`exact/rendering-and-motion.md`. The native-stream-to-UI pattern (a Swift
module producing a level meter or a transcript) lives in
`exact/native-modules.md`, with the spike's `Spike.swift`. The background-audio
app pattern (permissions, plist keys, claims) lives in `exact/audio.md`.

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

## Subpage index

- [exact/ui-model.md](exact/ui-model.md): Contract, state, sources, layout, lists, inputs, navigation and tabs, accessibility.
- [exact/rendering-and-motion.md](exact/rendering-and-motion.md): the pipeline, what is native vs drawn, transforms 2D and 3D, transitions, keyframes, frame tasks, gestures, shared elements, what draws Contract by host, canvas/SVG/GPU, data-bound styling, recipes, cannot-do list.
- [exact/capabilities.md](exact/capabilities.md): the on-demand capabilities: the wgpu GPU module (with the full how-to for writing one in Rust), Canvas 2D, textflow, Markdown, the game add-on, native modules; what our build needs (Metal Toolchain); iOS and Android status.
- [exact/native-modules.md](exact/native-modules.md): app module vs package, calling convention, streams out, threading, errors, third-party libraries.
- [exact/audio.md](exact/audio.md): the session owner, claims, modes, background, Now Playing, interruptions.
- [exact/build-and-ship.md](exact/build-and-ship.md): layout, CLI, build scripts, kit sync, xcframeworks, signing, install, pinning, build discipline.
- [exact/testing-and-debugging.md](exact/testing-and-debugging.md): the driver, its limits, logs, failure messages.
- [exact/internals.md](exact/internals.md): architecture, execution, rendering, measured numbers.
- [exact/gaps.md](exact/gaps.md): what we need that is missing, with the cheapest fix.

## Open questions

- Does the spike's tone plus mic tap run on the phone, and survive a locked
  screen? (Blocked on Xcode account sign-in.)
- Does `.playAndRecord` with `allowBluetoothA2DP` hold A2DP playback quality
  with the phone's own microphone as input?
- The device behaviour of our data-bound theme strings and of a 3D `rotate`
  transition (`exact/rendering-and-motion.md`, open items).

## Sources

`https://github.com/expo/exact` at the commit above, cloned 2026-11-10. Docs
read: `README.md`, `docs/start-here.md`, `docs/contract-for-humans.md`,
`docs/contract-for-agents.md`, `docs/reference.md`, `docs/testing.md`,
`docs/recipes/*.md`. Lane knowledge folded in from the `mobile/README.md` of
branches `lane/ar-integrate`, `lane/ar-models` and `lane/ar-ui` (2026-11-10;
`lane/ar-spine` was not on origin), from the `mobile/README.md` of
`lane/ar-app` and the builders' notes of 2026-11-11 (marked [seen]), and from the 2026-11-10 contract and
animation research.
