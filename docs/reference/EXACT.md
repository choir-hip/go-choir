# EXACT: the reference for building Autoradio's app on Exact

Written against `expo/exact` at commit `76cd50f9e863129b3fc75db1673d6ad4bbf79258`
(2026-11-10), Xcode 27.1, iOS 27 simulator, on an M1 Mac. **Corrected on
2026-11-18 against the checkout at `de212e671` (2026-11-16).** That is the
source these pages were checked against; `mobile/scripts/setup-exact.sh` still
pins `76cd50f9e`, and the pin moves in its own commit with a rebuild (rule 2).
A `file:line` in these pages is at `76cd50f9e` unless it says `de212e671`; many
lines have moved, so find the sentence, not the number. `94b8671` is stale. Exact is
pre-1.0 with "no API stability, no backwards compatibility, no migration guide,
by design" (Exact `README.md`). Pin a commit and move the pin on purpose.

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

## How Exact means an app to be built

Read on 2026-11-18 from the checkout at `de212e671`; paths are relative to it.
The owner's ruling (CLAUDE.md, Product): the phone app is Exact, and only voice
is native.

**Where logic belongs.** An app is three files: `app.contract` (the view),
`app.ts` (its data module) and `app.test.contract` (its tests)
(`docs/start-here.md:245`). Contract "doesn't fetch, read files, or run
arbitrary code. That's what data sources are for" (`README.md:433-434`), and
Exact's own sample prompt says "Put the view in Contract and keep the list in
`app.ts`" (`README.md:24`). The data module is TypeScript on Hermes on native
hosts and the browser's engine on the web (`README.md:220-223`). It does
everything the app does that is not drawing [doc]:

| Job | In `app.ts` | Source |
|---|---|---|
| Server calls | `fetch`, checked by `net.fetch <origin>` grants; `exactTimeout`; `exactBodyFrom` and `exactBodyTo` move files to and from disk without entering JavaScript (to 1 GiB); no `FormData` | `docs/reference.md:2084, 2545` |
| Server streams | `fetch(url, { exactStream: (event) => value })`: server-sent events, WebSocket (`net.websocket`), byte streams, long poll (`exactLongPoll`, `exactHeld`), upload progress. The mapper sees every message; messages that arrive together commit once; the stream closes when its resource leaves the view | `reference.md:2554-2700, 2771` |
| Files and SQLite | `storage.fs`, `storage.sqlite` under `fs.read`, `fs.write`, `sqlite.open` grants; `app:/data` is kept, `app:/cache` refetchable, `app:/tmp` scratch; bundled SQLite; a quit waits for storage to land | `reference.md:2324-2345, 2482-2510`; `recipes/files-and-grants.md:5-14, 62-64` |
| Secrets | `store.get` and `store.set` under `secret.keep <name>`: Keychain on Apple, AES-GCM files wrapped by the Android Keystore, unencrypted `0600` files on Linux | `reference.md:5159-5167` |
| Sign-in | `@exact/oauth` (PKCE, refresh, sign-out); `@exact/sign-in` (Apple, Google, passkeys) | `recipes/oauth.md`, `recipes/sign-in.md` |
| Timers | Inside one answer only: `setTimeout`, `AbortSignal.timeout`, cleared when the answer ends. No `setInterval`, `Date.now()`, `new Date()` or `Math.random()`: time and seeds arrive as arguments. A poll loop is a Contract `task`. A turn gets 100 ms of its own CPU time; `await scheduler.yield()` starts another | `reference.md:2208-2260, 2295-2311`; `recipes/timers.md:3` |
| Work while unseen | `@exact/background`: a registered tag wakes the root's `sync` or `periodicsync` handler with a 30 s budget; no timer fires in a wake. `@exact/background-transfer`: the system finishes an upload or download | `recipes/background-work.md:3-7, 56-60`; `recipes/background-transfers.md:3-8` |
| Shipped data, settings | a JSON file `app.ts` imports; `state x = v persist` (16 KB) | `recipes/shipped-data.md`, `recipes/persisted-setting.md` |

Limits: a 64 MiB heap; pure-JavaScript npm packages only; an answer must have
exactly its declared shape, checked at run time (`reference.md:1998, 2198`;
`start-here.md:345-359`). The sample modules show the size: Shelf
(`apps/shelf`) has a 301-line `app.ts`, a 157-line `app.test.ts`
and a 1,228-line `app.contract`; the REST starter (`docs/patterns/rest`) has a
51-line `app.ts` and a 52-line Bun server [measured with `wc`, 2026-11-18].

**What a native package is for.** What only the platform knows: the device's
thermal state, "which TypeScript cannot" read (`recipes/native-module.md:4-5`);
a map or camera view (`recipes/native-views.md:3-4`); the audio session and
on-device models (`exact/audio.md`). A package has one interface
(`exact.json`) and one implementation per platform: Swift `ExactPackage`
on Apple, Kotlin `ExactExtension` on Android (`host/android/README.md:796-806`;
a whole one is `packages/geolocation/android/Geolocation.kt`), a web
implementation, Rust on Linux. A value a launch gives the app is a setting, not
a module (`recipes/native-module.md:7-12`). `app.ts` asks a module with
`native.call` or `native.later`; the build has no module, so a resource that
asks one shows its `else` row until launch (`recipes/native-views.md:62-63`).

**The development loop Exact intends.** The web is the loop: "You work on the
seconds-long web loop, and the native hosts are swept behind you"
(`README.md:180-181`). `bun exact.mjs web` serves the app and prints its URL; it
"rebuilds on save" (`start-here.md:72`), and on a fresh app a saved Contract or
TypeScript edit was in the page in 99 ms (`README.md:155-156`). A native app
follows the same URL: `host/apple/build.mjs --ios --run --url <url>` (a phone:
`--device --run --url`) and `bun exact.mjs android --run --url <url>`
(`reference.md:1855-1870`; `exact.mjs` passes `--url` through, `scripts/app-cli.mjs:343`
[inferred]). Once such a client runs:

| You edit | What happens | Source |
|---|---|---|
| `app.contract` | the running app's plan is replaced, state carried | `reference.md:1866, 1877`; `host/android/README.md:157-159` |
| `app.ts` or a `.ts`/`.json` it imports | the module pair is replaced in the running app, state carried | `reference.md:1866-1868, 4718-4723`; `host/android/README.md:158-159`; `README.md:767-768` |
| Rust data crate | Android: refused, the running app kept, a new APK is needed. iOS: `recipes/ios-driving.md:18-19` says rebuild, `reference.md:1867-1868` says declared Rust modules reload; Exact's pages disagree. We have no Rust data | `host/android/README.md:160-164` |
| Swift or Kotlin package or app module, `app.json`, a grant, `appId`, assets, fonts, strings, icons, canvas surfaces | a native build; the client keeps the last good generation on screen until the host is rebuilt and relaunched | `recipes/app-json.md:47-49`; `reference.md:1877-1879`; `scripts/agent-android.mjs:1372` |

"A first native build takes ten to fifteen minutes on a laptop; later builds
reuse it and take a minute or two" (`README.md:351-353`; `start-here.md:51`).
That is the cost of a *native build*: the Rust host cold, then a bake and link
that "compiles no Rust" when only the Contract or `app.ts` changed
(`reference.md:4693-4698`). It is not what a Contract or `app.ts` edit costs
once the app follows the web loop. A TypeScript edit that first uses a newer
prelude feature raises the module's `typescriptRuntime` and needs one new
binary (`reference.md:2084, 2406, 2761`).

**How logic is tested without a device.**

- `app.test.contract` drives the app by `testId` through the same ten driver
  operations on every host; steps are `tap`, `type`, `expect text|state|tree`,
  `clock data|settle|+N`, `reload`, `back`, `fail fetch`, `answer fetch ... with
  200 {...}` and, for an app with a server, `request POST ...`
  (`recipes/tests.md:21-32`; `recipes/rest-backend.md:34-60`). `bun exact.mjs
  test web` runs it; `agent web --storage s1 "clock data" "tap add" tree` is the
  same driver by hand; `verify` builds, runs the tests, `check`s the first
  screen and each test's end light and dark at two widths, reads the
  accessibility tree, relaunches once and reads at the largest text size
  (`start-here.md:70-71, 75`). `test` and `agent` compile their own copy (no
  `contract build` first, no dev server) on web, iOS, macOS and Windows; on
  Linux and Android they drive a build you made first (`start-here.md:75`).
- `bun test` runs the data module's `answer(source, args, store, storage, native)`
  directly with a fake `storage`. Shelf does this (`apps/shelf/app.test.ts:1-4,
  35-45`) and Exact runs its own tooling with `bun test ./scripts/`
  (`reference.md:16`). Exact's docs describe no other app-level `bun test`
  recipe; `docs/testing.md` is about testing Exact itself.
- What they see: the tree, state and text of the real plan on the real runner,
  fetch faults and fixtures, and a Bun-checked module. What they cannot: Bun has
  no clock or randomness guard, so `Date.now()` passes there and is refused by
  every build (`reference.md:2305-2309`; `agent-pitfalls.md:1068`); Bun has no
  `scheduler` (`reference.md:2224`); a stub patched into `window.fetch` after
  boot changes nothing (`agent-pitfalls.md:1540`); the browser keeps no 100 ms
  turn budget (`reference.md:2218-2224`); a native package is a placeholder or
  its interface's substitute, not the Swift or Kotlin (`recipes/native-views.md:62-63`;
  `host/android/README.md:832-834`); no real audio, touch or lock screen
  (`agent ios --touch platform` makes taps real touches,
  `recipes/ios-driving.md:3-9`); `prefer frozen true` tests an authored
  reaction, not suspension (`contract-for-agents.md:4937-4943`). `check` is the
  web's and iOS's; Android refuses it (`host/android/README.md:326`).

**The web host when the app ships only native.** It is still the fast loop, the
parity oracle for layout (`README.md:177-179`) and the cheapest place to run
`app.test.contract`. "A native-only app does not need a web build"; check each
target platform that ships and report any that could not be run
(`start-here.md:145-147`). Run `verify ios` or `verify android` once at the end
for a phone app (`start-here.md:75`).

**What an Android or iOS dev loop does with a running app.** `bun exact.mjs
android --run --url http://127.0.0.1:<port>/` launches the app following the
loop (`adb reverse` is set up for it, `-S` stops a running reader first); a
Contract or `app.ts` save reloads it with its state, a native change needs a
new APK (`host/android/README.md:153-165`; `scripts/agent-android.mjs:1372`).
Refused: `--url` without `--run`, with `--headless`, or a non-http(s) URL
(`scripts/agent-android.mjs:1431-1432`); a replaceable Rust module (above); a
production bake opens no build channel (`host/android/README.md:167`). On Apple the dev
client registers its opening link for that server's origin alone with a
per-build token (`host/apple/build.mjs:19-22`), and a generation whose native
inputs changed is not applied (`reference.md:1877-1879`).

**Production updates.** With `deploy.store.<platform>` `"A"`, signed plans,
assets and one TypeScript module pair update over the air; never native code
(`scripts/app.schema.json:1208`; `README.md:797-801`; `reference.md:4770-4777`).
It needs the binary's TypeScript runtime and exactly its grants, a signing key
and a receipt (`recipes/mobile-build.md:7-10`). Exact says Android's store reads
`"0"` when absent. We ship `"0"` and have not run this [doc, unrun].

## How we departed from it

Measured on `lane/autoradio-integ20`, 2026-11-18
([the record](../problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md)):
the pages are 50 Contract files of 7,742 lines, and the app's `app.ts` is
11 lines of grants and an empty source map. The logic is 38,149 lines of Swift
in `AutoradioKit` behind 3,136 lines of binding and 181 interface names, and
the package has no Android implementation. Every change to a page's logic
therefore needed a native build (our warm builds: about 60 s for Swift,
about 57 s for a TypeScript-only change, 12 to 40 s for a Contract-only change,
from the lane reports; `scripts/build-lab.sh` always syncs the kit and runs the
whole `ios` verb, and no run of an app following the web loop is recorded). The
pages follow Exact's grain; the logic does not.

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

**Where Swift sits.** Nowhere in the UI and nowhere in the app's logic. Native
code is a plug-in for what only the platform knows: native views (any `UIView`),
calls (`later`, `call`), and device-level work (audio session, sensors,
on-device models). A **package** declares names, views and events once in
`exact.json` and implements them per platform (Swift on Apple, Kotlin on
Android); on Apple it can take audio claims. The app has exactly one **app
module** (any number of packages), which is where third-party Swift and Core ML
code enters as static xcframeworks. Network, storage, secrets, sign-in, streams
and state machines are `app.ts`, which both phones share. We may write Rust
where Contract and TypeScript cannot do the job, such as a wgpu GPU module
(rule 5 below, `exact/capabilities.md`); we do not extend Exact's engine itself
(`exact/native-modules.md`).

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
| Network calls, JSON, state machines, sign-in, sync, parsing | `app.ts` (Hermes on native, the browser's engine on web): the same file on iOS and Android | Grant-checked `fetch`, `storage.sqlite`, `storage.fs`, `store` secrets (`recipes/files-and-grants.md:5-14`); a plain GET needs no `app.ts` (`recipes/declared-source.md`) |
| A server stream (server-sent events, WebSocket, long poll, upload progress) | `fetch(url, { exactStream })` in `app.ts` | The mapper sees every message and the view re-reads on commit; no native code (`docs/reference.md:2605-2700`) |
| Data that lasts across launches | `storage.sqlite` or `storage.fs` under `app:/data`; a token in `store` under `secret.keep`; a small setting `state x = v persist` | `recipes/files-and-grants.md`, `recipes/persisted-setting.md` |
| A timer, a poll, a countdown | a Contract `task` (`every`, `after`); inside one answer `setTimeout`; no `setInterval` | `recipes/timers.md`; `docs/reference.md:2227-2311` |
| Work after the app is hidden | `@exact/background` (30 s wake, no timers), `@exact/background-transfer`; `page.frozen` per host (`contract-for-agents.md:4923-4932`: Android freezes when the Activity stops unless a foreground-service claim is held; iOS when in the background without active audio; macOS, Linux and Windows never) | `recipes/background-work.md`, `recipes/suspending-feed.md` |
| Audio session, `AVAudioEngine`, mic, wake word | Native package: Swift on Apple, Kotlin `ExactExtension` on Android | Only packages reach the owner's claims on Apple (`exact/audio.md`); Android's package shape is `host/android/README.md:796-806` |
| A stream from a device into the UI (RMS, partial transcripts) | Package plus `context.changed` | The only native push path is "ask the source again" (`exact/native-modules.md`). A stream from a *server* is `exactStream`, above |
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
  they need it; then **`app.ts` for all app logic** (network, storage,
  secrets, sign-in, streams, sync, parsing, decisions); a native package or
  UIKit view last, only where Exact has no path (microphone and audio
  session, on-device models, system services), and written so the same
  interface could be met on another host. A feature added in Swift because
  the code was already there is the defect named in
  `docs/problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md`.
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
   default because Exact swaps them into a running app without a native build
   (above; "a minute or two" is a native rebuild, not this); Rust is for what
   they cannot do, such as a wgpu GPU module, and each Rust crate states its
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
11. Test logic where it is cheap: `app.test.contract` on `test web`, `bun test`
    on the module's `answer`, a device only for what they cannot see (above).
    A change to `app.ts` is proven on the web loop before any native build.

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
| "Over-the-air logic updates do not exist; everything ships as a binary." | Wrong at `de212e671`. With `deploy.store.<platform>` `"A"`, signed plans, assets and one TypeScript module pair update over the air, never native code (`scripts/app.schema.json:1208`; `README.md:797-801`). What is not implemented is delivering a TypeScript and a Rust module together. We ship `"0"` and have not run an update [doc, unrun] |
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
   [run]. And `exact.mjs setup` downloaded Hermes for every platform (Android,
   Linux, Windows, tvOS, about 190 MB each) and stalled for us at `76cd50f9e`;
   `docs/reference.md:35-41` at `de212e671` says it installs the host bundle
   and, on macOS, the iOS and tvOS ones, while `README.md:790` says it adds the
   Android bundle "with the target". For Android we installed the bundle by hand
   (`exact/build-and-ship.md`, "Android") [run].
8. **Exact never makes provisioning profiles.** It signs with a profile that
   already exists on the Mac. A free account therefore needs the owner signed
   into Xcode (Settings > Accounts) so Xcode can create one for our bundle id.
   Without that, `xcodebuild -allowProvisioningUpdates` fails "No Account for
   Team" [run].
9. **The test driver owns the clock.** Between two driver operations nothing
   moves. `clock settle` and `clock +N real` advance things. A tone or an
   audio engine runs on real time, so drive it with `clock +N real` [doc,
   partly run].
10. **Over-the-air delivery covers a TypeScript module, not native code.**
    `deploy.store.ios: "0"` links no update store and every change is a new
    binary; that is what we ship. With `"A"`, signed plans, assets and one
    TypeScript module pair update (`scripts/app.schema.json:1208`,
    `README.md:797-801` at `de212e671`) [src]. Only "signed TypeScript and Rust
    module delivery" together "isn't implemented". Our earlier reading of that
    sentence, "no logic updates", was wrong.

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
bounded rate (once per second in the spike). The UI re-asks and redraws. This
is the pattern for a *device* stream. A stream from our server is `fetch` with
`exactStream` in `app.ts`, which both phones share.

## Sharp edges (file:line at `76cd50f9e`; unchecked at `de212e671`)

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
- [exact/build-and-ship.md](exact/build-and-ship.md): what each edit costs per host, layout, CLI, build scripts, kit sync, xcframeworks, signing, install, pinning, build discipline, the Android build.
- [exact/testing-and-debugging.md](exact/testing-and-debugging.md): what each check can see, the driver, its limits, logs, failure messages, the Android drive.
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

`https://github.com/expo/exact` at `76cd50f9e`, cloned 2026-11-10, and
rechecked at `de212e671` on 2026-11-18, reading in full `README.md`,
`docs/start-here.md`, `docs/recipes/{mobile-build,tests,native-module,native-views,rest-backend,sign-in,oauth,fetch-loading-error,after-a-write,declared-source,persisted-setting,files-and-grants,shipped-data,background-work,background-transfers,suspending-feed,functions,tools,timers,report-errors,store-update,porting-and-release,ios-driving,app-json}.md`,
the data-module, dev-loop and storage sections of `docs/reference.md`,
the build, dev-loop, data-module, driving and module sections of
`host/android/README.md`, and `apps/shelf` and `docs/patterns/rest`. Earlier
reading, 2026-11-10: `README.md`, `docs/start-here.md`, `docs/contract-for-humans.md`,
`docs/contract-for-agents.md`, `docs/reference.md`, `docs/testing.md`,
`docs/recipes/*.md`. Lane knowledge folded in from the `mobile/README.md` of
branches `lane/ar-integrate`, `lane/ar-models` and `lane/ar-ui` (2026-11-10;
`lane/ar-spine` was not on origin), from the `mobile/README.md` of
`lane/ar-app` and the builders' notes of 2026-11-11 (marked [seen]), and from the 2026-11-10 contract and
animation research.
