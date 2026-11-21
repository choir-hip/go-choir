# EXACT: the reference for building Autoradio's app on Exact

Pinned: `expo/exact` at commit `f5334750386192245ee76add13a67ab51fb4e2fd`
(upstream `main`, 2026-11-20 10:40 -0800; `mobile/exact.pin`), moved on 2026-11-20 from
`de212e671a69f4fea18bed67d5b73b259ddf3061` (2026-11-16), 762 commits (before that on 2026-11-17 from
`76cd50f9e863129b3fc75db1673d6ad4bbf79258`, 2026-11-10, 3018 commits). **The 2026-11-20 move is read, not built**: the section "Pin move,
2026-11-20" says what was read and what is unrun; until the lead's build passes, treat the pin line as a proposal.
Observed with Xcode 27.1, iOS 27 simulator, on an M1 Mac. The subpages and the
examples below were written and their `file:line` read at the old pin, then
**corrected on 2026-11-18 against the checkout at `de212e671`**: a `file:line`
in these pages is at `76cd50f9e` unless it says `de212e671` (or `f53347503`, the pin of 2026-11-20); many lines have
moved, so find the sentence, not the number. The sections "Pin move, 2026-11-17" and "Pin move, 2026-11-20"
say what was re-read at each new pin. `94b8671` is stale. Exact is pre-1.0 with
"no API stability, no backwards compatibility, no migration guide, by design"
(Exact `README.md`). Pin a commit and move the pin on purpose. We carry no fork:
local changes to the pinned source are patch files in `mobile/exact-patches/`
(applied by `mobile/scripts/setup-exact.sh`; each one explained in that
directory's README and, for the Markdown editor, in `exact/capabilities.md`).

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

## Pin move, 2026-11-17 (76cd50f9e to de212e671)

What was done (read, not built: **[src]** unless marked; the first builds at the new pin are the lead's):

- **The Markdown patch stays.** `mobile/exact-patches/0001-markdown-editor-hides-all-syntax.patch` passes `git apply --check`
  at the new pin and `setup-exact.sh` applies it from the pin's checkout. Upstream's `host/apple/Sources/ExactMarkdown` changed only in
  `MarkupAtoms.swift` and `MarkupRuns.swift` (atom icons), and `markdown/src/style.rs` gained `Paragraph.content` and
  `Styled.delimiters` (matched emphasis, code and strike delimiters) but still no option that hides all syntax, so the patch is not
  replaced. `Styled.delimiters` is the new upstream hook a later version of the patch could use instead of the hidden-range walk.
- **Manifest and interface.** Every key our `app.json` uses (`host.ios.launchImage|minimumOS|deviceFamily|backgroundModes`,
  `deploy.store`, `audio_session`) is still in `scripts/app.schema.json`; `audio_session` gained `playback-mix`. Package
  `exact.json` (implementation) and the interface format are unchanged in shape; `targets` takes `["ios","macos"]`.
- **Package seam** (`host/apple/modules/ExactPackage.swift`): additive only. New context members (`label`, `data`, `cache`,
  `temporary`, `directory`, `credentials`, `push`, `hostEvents`), `ExactPackageImplementation`; `ExactPackage` protocol unchanged.
  Our module needed no change for it.
- **A package's Swift now loads as a framework of its own, at first use** (LLP 1067.000.001, `4e1b0e245`). "A package whose type
  the app's Swift imports still loads at launch": our `Models_*.swift` import `ExactPkg_autoradio`, so ours does. **[unrun]**
  whether the app module's `import ExactPkg_autoradio` still resolves; the first iOS build says.
- **The audio session owner was rewritten** (`ExactKit/AudioSession.swift`): background execution now needs an activated, running
  owner, a `playback` or `playAndRecord` category and `audio` in `UIBackgroundModes`; a call's session stays the call's until the system
  deactivates it; holders reuse an active session without querying `AVAudioSession`. Our claims (`speech-playback`, `default`,
  `voiceChat`, `background: true`) are unchanged in name. **[unrun]** whether the programme keeps playing with the phone locked: re-prove
  it on the phone after the move.
- **Contract language**: the retired spellings the migrator rewrites are `syntax-list-type`, `syntax-method-form`,
  `syntax-view-keyword` (`when`/`each` for `if`/`for`), `syntax-else-keyword`, and press paints moved into an active part. Our contracts
  use `if`/`else if`/`for`; `bun exact.mjs contract migrate app.contract` rewrites any other. `tamber` is renamed `forma`
  (`app.tamber` is now `app.forma`); we use neither. A package view now holds an enum to the cases it declares (`393202fae`): an
  `enum<...>` prop needs a state typed with every case. **[unrun]** all of these for our contracts: `contract build` is the check.
- **Toolchain**: Bun stays exactly 1.4.2 (`packageManager`, `engines`). Rust moves from 1.97.0 to 1.99.1 (`rust-toolchain.toml`; rustup
  installs it on demand; this Mac has it). Xcode 27.1 or later (unchanged). The Hermes receipt schema changed
  (`vendor/ibex/crates/hermes-lean-sys/receipt_schema.rs`): if the build asks for Hermes bundles, `bun exact.mjs setup`. Nothing is
  installed system-wide by this change.
- **Readiness `setup-exact.sh` now provides** (found by the first builds at this pin, 2026-11-17): (1) Exact's own `node_modules`
  (`bun install --frozen-lockfile` in `.exact-src`, stamped with the pin; every build's type check imports rolldown from it);
  (2) the pinned Rust toolchain's targets, read from `.exact-src/rust-toolchain.toml` (not a literal): `aarch64-apple-ios-sim`,
  `aarch64-apple-ios` and `aarch64-apple-darwin`, each added with `rustup target add` and printed when added (the build says "--ios needs
  Rust's aarch64-apple-ios-sim std" without them); (3) the Hermes JS engine bundles for those three triples, run through Exact's
  `hermes-lean-sys-installer` (`--check` first, offline; the install only when a pinned bundle is not cached). Exact's CLI sets
  `HERMES_LEAN_SYS_OFFLINE=1` for every build it spawns (`scripts/app-cli.mjs`), so a build never downloads; a missing bundle stops it with
  "is not cached ... and offline mode is enabled". Bundles are sha256-pinned by Exact and live in `~/.cargo/hermes-lean-sys/`.
  (4) `exact-package/apple/exact.json` is generated by `sync-exact-package.sh` (its file list changes with every Kit file) and is
  gitignored, so a build leaves the tree clean. `bun exact.mjs update` and `lock` are for scaffolding drift and Rust locks: not run, our launcher
  `AutoradioLab/exact.mjs` still matches `run({ dir, name, exact2, args })`.

### The Mac host at the new pin (re-checked against the investigator's plan)

- **Still true [src]:** `mac` (spelling) / `macos` (manifest key) builds the AppKit host with the same package for both platforms
  (`app-cli.mjs` `mac: ['host/apple/build.mjs', ...]`); `host.macos` takes `minimumOS` (never below "14.0"), `window` with
  `width`, `height`, `minWidth`, `minHeight`, `keepRunning`, `presenter`, `activationPolicy`, `team`, `infoPlist`, `entitlements`,
  `link` and more; `ExactPackagePlatformView` is `NSView`; **the Mac has no audio session** (`AudioSession.swift` is `#if os(iOS) ||
  os(tvOS)`; the engine starts with no category and a capture's claim composes nothing; `docs/reference.md` "The Mac has no session").
- **New [src]:** `mac --sandbox` signs the App Sandbox and the agent drives it; `exact appstore` builds the Mac App Store package; a
  production Mac build captures its own windows; `test macos` and `agent macos` exist; `app-cli.mjs` refreshes the contract's
  declaration on `ios`/`mac`. None is used by us yet.
- **How we build for macOS** (`mobile/scripts/build-lab.sh mac --run`) and what the app does not support there: see "Building for
  the Mac" below.

## Pin move, 2026-11-20 (de212e671 to f53347503)

Owner, 2026-11-20: "We're going to be writing the latest versions of Exact for some time. We definitely should be on the latest before we
write all this exact code." **Read, not built** (no Exact checkout other than the lead's, which stays at the old pin until the lead moves it;
every mark is **[src]** read in upstream source with `git show`/`git diff` against it, or **[unrun]**). Upstream `main` was at
`fb72ddc3a`, then `55492cd8a`, then `f53347503` while this was read (it moves several times a day; the last step is 11 files of Mac
drag-out, none ours). 762 commits, 3,699 files. Exact still has no changelog or migration guide; its only upgrade route is
`docs/start-here.md#upgrading-an-inherited-app` (`exact update`, `contract migrate`, `lock`, `contract build --json --errors-only`).
The procedure to move the pin, with each command's expected result, is in [build-and-ship.md](exact/build-and-ship.md#moving-the-pin).

**Nothing we use was removed or renamed, as far as the sources say; no adaptation was found necessary. That is a reading, and Exact's
compiler is the check.** What was compared, with what we use:

- **Contract grammar [src].** Additive or relaxing in every place we write: a child component may own `resource`, `mutation` and `task`
  at bounded uses (`contract-grammar.md`, the component section); `expect state` takes `!=` and `state <path>` operands (our `expect state
  pfront.sections.0 .id == "world"` still parses: `state_path` in `contract/syntax/src/parser/state_step.rs:7` is the old walker); new test steps
  `expect layout|sample|screenshot|drag`, `presence`, `drop stream`, `stored`. New refusals checked against our files by grep, none hit: the
  geometry fence `bake-geometry-feedback` (`contract/cli/src/fence.rs:1`) reads only `exactGeometry`/`exactNativeChain`/`exactSafeAreaUnsettled`
  (`runner/src/native_geometry.rs:52`), we call none (we call `exactViewport` and `exactPage`); `lower-host-name-declared` (an app declaring a
  `--exact-*` property: none); `syntax-resource-clause-order` (ours are `... as shape X else empty()`, in order); `lower-css-shorthand` now looks
  through derives (`contract/lower/src/shorthands.rs`): our computed `padding`/`border-radius` are literal arms or numbers; `lower-attr-pair`
  and `lower-draggable-*` (`draggable`, `dataTransfer`: none). Retired spellings `contract migrate` rewrites (`-exact-press-*`,
  `syntax-list-type`, `-method-form`, `-view-keyword`, `-else-keyword`) are the 2026-11-17 list, unchanged (`contract/syntax/src/migrate.rs`
  differs by one line), and we write none. Warnings only: `lower-tablist-conditional`, `lower-alert-font`; `lower-replace-root` is gone.
- **Safe area and keyboard (LLP 1123, about 40 commits) [src; the one place a visual change is plausible, [unrun]].** The kernel now lays out
  keyboard regions, bars in slots, reservations and "a box that ignores the safe area". `env(safe-area-inset-*)` is still the document's insets
  (`contract-for-agents.md` "Dimension rows": 0 under `viewport-fit="auto"`, the device's under `cover`), and the new
  `var(--exact-safe-area-inset-*)` is the box's own. We use `viewport-fit="cover"` and `interactive-widget="resizes-content"` on the root
  (`AutoradioLab/app.contract:453`) and `env(safe-area-inset-top|bottom)` in about fifteen places (bars, the reader, paper, queue); `keyboardHeight`
  and `resizes-content` are documented as before (`contract-for-agents.md` "keyboardHeight"). **Look at the prompt bar over the keyboard and the
  top and bottom bands in the first screenshots.**
- **TypeScript [src].** The generated `app.contract.d.ts` keeps `Answer`/`Sources`/`Store`/`Storage` (`contract/cli/src/typescript.rs:183`); it adds
  the global `exactHost()` (an answer that reads it is not baked) and `RequestInit` options. `grants` is still a constant read from syntax (a
  template of a top-level const is fine; imported app-local constants are now allowed too), our `fs.read app:/...`, `net.fetch` and
  `device.*` lines are the same grammar. Our launcher `AutoradioLab/exact.mjs` calls `ensureBun("1.4.2")` and `run({ dir, name, exact2, args })`:
  both signatures are unchanged (`scripts/bun.mjs:4`, `scripts/app-cli.mjs:265`). `bun exact.mjs update` would overwrite our launcher: do not run it.
- **Package seam [src].** `host/apple/modules/ExactPackage.swift`: the `ExactPackage` protocol (`init(context:)`, `later`, `call` required) and
  `ExactPackageContext`, `audioSession.claim(mode:options:background:events:)`, `ExactSubscription`, `ExactPackageView`, `ExactPackageEvents` are
  unchanged; the diff adds screen geometry, track picture observers, `ExactVideoSource.configure(stopped:)` (old form kept), `ExactTransfers`
  `form` parts. `ExactModule` (our `ModelsModule`: `call`, `later(_:reply:)`, `let exactModule`) is unchanged; upstream moved `ExactTabs` to
  `ExactNativeTabs.swift` and `NativeHatches.swift` to `ExactNativeViews/` (gaps 1 and 3 cite the old place). The adapter that makes a package
  (`packageAdapter` in `host/apple/compose.mjs`) still calls `ClassName(context:)` and the package module is still `ExactPkg_<name>`. Our
  `apple/exact.json` keys (`kind`, `implements`, `platform`, `targets`, `views`, `sources.swift`, `class`) and the interface keys
  (`shapes`, `views`, `names`, `substitute`) are read by `contract/cli/src/declaration.rs`, whose only changes are shape field order for
  `@exact/model-viewer` and a stricter grant-selector check on `substitute` (none of ours). Apple modules now build `-O` through
  `swiftOptimization()` (`host/apple/toolchain.mjs`), as before.
- **Build [src].** `rust-toolchain.toml` is unchanged (`1.99.1`, `wasm32-unknown-unknown`); Bun is still exactly 1.4.2; Xcode 27.1 or later;
  the only root `package.json` change is `playwright-core` 1.64.1 to 1.65.1 (`bun.lock` names all 65 workspaces: the failure
  expo/exact#892 fixed). The Hermes bundle receipt changed (`vendor/ibex/crates/hermes-lean-sys/build_support.rs`, `receipt_schema.rs`), so
  `setup-exact.sh`'s `--check` will likely fail once and install the new bundles online. **New:** the Apple host is composed "by use"
  (`567d7a64a`, `734742f72`, `host/apple/Package.swift`, `app.json` `link`): a production build links only what the plan and grants use;
  **development links every capability except `media_picker`, `auth_session`, `numerals` and `tips`** (`scripts/app.schema.json`, property `link`), so
  our dev builds need no `link` entry; a distribution build might (we use `text markup="markdown"`, `input type="file"`, `audio` claims and a package view, which the plan selects by use; which capability names a distribution build needs is **[unrun]**). **[unrun]**
  whether the roster a package reports in the build's probe still matches our interface (`checkModuleRoster`).
- **Patch 0001 [run, 2026-11-20].** Applies unchanged (`git apply --cached --check` against a scratch index read from `f53347503` passed; the three
  files it depends on are byte-identical to the old pin). Upstream still dims syntax on iOS and offers no prop to hide it. See
  `mobile/exact-patches/README.md`.
- **Workarounds this makes unnecessary (see [gaps](exact/gaps.md)):** gap 19 (no SwiftPM libraries in a package): `swiftPackages` in a package's
  `apple/exact.json` or `app.json` `host.ios` now declares products, locked by a `Package.resolved` the app commits (`bun exact.mjs lock`), built
  into one dynamic library the app and every package share (`7209c04ad`). We do **not** adopt it in this move: FluidAudio and llama.cpp stay xcframeworks
  until a build passes at the new pin. Nothing else we carry is removed.

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
   Linux, Windows, tvOS, about 190 MB each) and stalled for us at `76cd50f9e`
   (`mobile/scripts/setup-exact.sh` now installs only the host and Apple bundles through Exact's installer, see "Pin move");
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

## Storage error codes: how we branch on them (2026-11-21)

A storage refusal is an `Error` with a `code` that is the same on every host (`'ENOENT'`, `'EACCES'`, `'EBUSY'`, `'denied'`, `'full'`, `'failed'`: `reference.md`, "storage refusal"; Branch on the code, never the message). `account.ts` takes a file as ABSENT only when a read or stat rejects `ENOENT`; every other rejection, a read past a deadline, or no storage is UNREADABLE and refuses the page, never "nothing there". A removal counts only when the stat afterwards says `ENOENT`. No call from app code reloads the data module or opens a fresh context (module reload is the development client and over-the-air update: `reference.md:2289-2296`), so the account boundary ends work by closing a `World` object (`api.ts`), not by a reload. Timers (`setTimeout`) end with the answer that made them; a background job started by an answer (the Clear in `account.ts`) relies on its deadline anyway: unverified on a device.

`fs.rm` of a folder (read 2026-11-21 at the pin, `mobile/.exact-src`): on the Apple and Unix host it removes the folder with its contents (`vendor/ibex/crates/ibex2/src/stdlib/app_fs_unix.rs:236-260`, called for `FsOp::Remove` at `:674` and `:970`: children first, then the folder). The reference states no rule for the other hosts (web, Android), so every folder wipe in `account.ts`, `podcast.ts` and `files.ts` goes through `rmTree` (`api.ts`: children first, then the folder), tested over a fake fs that refuses a non-empty folder.

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

## Building for the Mac

**[unrun]** until the first Mac build is seen: this is the plan as built in the `mac-app` lane.

- Command: `mobile/scripts/build-lab.sh mac --run` (add `ALLOW_NO_MODELS=1` until the macOS slices of the model libraries exist).
  It takes `~/.cache/autoradio-mac-build.lock` itself for the build. The window is `host.macos.window` in `AutoradioLab/app.json`
  (430 by 900, minimum 360 by 560), `minimumOS` 15.0 (the Kit needs macOS 15).
- The module is the same Swift as on the phone. Every UIKit or `AVAudioSession` call goes through `AutoradioModule+Platform.swift`
  (`Platform.open`, `systemScheme`, `reduceMotion`, `outputName`, the notices); on the Mac the session owner is `MacSessionOwner`, which
  holds no claim and reports "ready", because the Mac has no session.
- Not supported on the Mac yet: lock-screen and background playback concepts (`host.ios.backgroundModes`, `audio_session` are iOS-only
  words), echo cancellation until the engine's voice processing is wired on the Mac, the phone's route names, and anything the
  Mac-specific work would add (menus, keyboard shortcuts, a centred column).

### The model libraries on the Mac

- FluidAudio and llama.cpp reach the Exact app only as static xcframeworks, and each needs a `macos` arm64 slice for `mac`
  (`scripts/ensure-deps.sh macos` checks `Info.plist` for it; an iOS-only copy is "missing" for the Mac, as Exact says "has no macos
  arm64 slice in its Info.plist"). `build-xcframeworks.sh fluidaudio` builds three slices (`sim`, `dev`, `mac`:
  `--triple arm64-apple-macos15.0`, about 10 minutes each on a loaded Mac) and `build-llama-xcframework.sh` builds the macOS one itself
  (cmake, arm64 only, macOS 15.0, the release script's own options; the release script is not used for macOS because it deletes the other
  `build-*` directories and builds universal at 13.3). **[unrun]** until the lead's first run: where SwiftPM puts the macOS product (the
  script finds `libFluidAudio.a` instead of assuming `Release-<sdk>`).
- Where things live on the Mac: models under `~/Library/Application Support/Autoradio/models` (the app is not sandboxed in a development
  build), exports under `~/Documents/AutoradioExports`.
- Permissions **[src]** (`runner/src/device.rs`): `device.microphone` writes `NSMicrophoneUsageDescription` and `device.speech-recognition`
  `NSSpeechRecognitionUsageDescription` for macOS from `strings/en.json`; `device.microphone-background` needs nothing on a Mac. A
  development build is ad hoc signed with no hardened runtime, so the first microphone use shows the system prompt and nothing else is
  required; an `exact release` build derives the hardened-runtime audio-input entitlement from the grant.

### Audio, sign-in and files on the Mac

- **Audio [src]:** the engine runs with no session: `AutoradioKit`'s `LiveAudio` asks `AVAudioInputNode.setVoiceProcessingEnabled(_:)` on macOS
  as on iOS (`AVAudioIONode.h` in the macOS SDK declares it for macOS 10.15; both IO nodes switch together, and only while the engine is
  stopped, which `bring(voiceProcessing:)` already honours). Play, pause and the single-voice hold live in the engine and the run sheet, not
  in a session. Device changes (headphones in or out) arrive as `AVAudioEngineConfigurationChange`, which `LiveAudio.observe()` already
  handles. **[unrun]** by ear: echo cancellation with speakers, and whether the voice-processing input node's format (it can be more
  than one channel on a Mac) converts to the mono bus (`Resampler`).
- **Sign-in:** the approval page opens through `Platform.open` (`NSWorkspace`). The key is stored in the Keychain without the iOS-only
  `kSecAttrAccessible` class on the Mac. **[unrun]** whether an ad hoc signed app can write the login keychain without a prompt.
- **Files and the Reader:** the picker is Exact's `input type="file"` (the host opens `NSOpenPanel` on the Mac); PDFKit and ImageIO are
  present on macOS. The Reader's web view is our `WebPageView` (`WKWebView`; `drawsBackground` is false on the Mac).
- **Proof:** The Mac proof script (`proof-mac.sh`) was deleted with demo mode on 2026-11-20; a Mac app proof is to be rewritten on real data when the Mac app starts.

### Apple privacy manifests (required-reason APIs) at this pin

**[src]** `host/apple/privacy.mjs`, `docs/reference.md` "Apple privacy manifests". An iOS build scans every binary of a bundle (the app,
Exact's `libexact_*.dylib` and our `libExactPkg_autoradio.dylib` together) for required-reason APIs and stops with, for example,
"uses disk-space APIs without an audited reason" when one lacks a declaration. It audits five categories (file timestamp, system boot time,
disk space, active keyboards, user defaults) and refuses `getattrlist` without an audit; Exact seeds its own reasons for the first, second,
fourth and fifth, never for disk space. macOS skips the check (Apple's reporting excludes it). An app declares its own in
`PrivacyInfo.xcprivacy` beside `app.json`; reasons are merged, each must be one of Apple's approved codes for the category, and only
categories the binaries really use are written.

Ours is `mobile/AutoradioLab/PrivacyInfo.xcprivacy`, with codes read from Apple's "NSPrivacyAccessedAPIType" page (2026-11-17):

| Category | Our use | Code |
| --- | --- | --- |
| Disk space | `volumeAvailableCapacityForImportantUsage` in `VoiceSetup` and `Configurator`: the setup plan and model downloads depend on free space | `E174.1` (check there is sufficient space to write files) |
| File timestamp | `attributesOfItem` size and modification date in `ModelStore`, `PaperSource`, `ChoirFiles`, `ReaderShelf`: files in the app container | `C617.1` |
| System boot time | `ProcessInfo.systemUptime` for intervals and frame times inside the app | `35F9.1` |
| User defaults | `UserDefaults.standard` for the app's own settings | `CA92.1` |

Not declared: active keyboards (we never read them; Exact declares its own). The disk-space and boot-time information must not leave the
device: it does not, except that turn timings in an export the person shares are intervals, not boot time. Re-run this table when a new use
of one of these APIs is added.

### The pin is checked, not trusted

The pin is one file, `mobile/exact.pin`. `scripts/setup-exact.sh` reads it, fetches that exact sha from the clone's origin when an older
clone lacks it, takes the local patches off, checks out the pin, reapplies the patches, and exits non-zero with a `setup-exact:` line on any
failure. `scripts/check-exact-pin.sh` is called first by `build-lab.sh` (every target) and `exact.sh` (the common entry to Exact's CLI): it
refuses to go on unless `.exact-src` HEAD equals the pin and every patch is applied, printing both shas. Reason: a checkout left at the old
pin once printed `fatal: unable to read tree` from setup and then built an iOS app against the OLD Exact, exit 0. Moving the pin is a change to
`exact.pin` (and this page), never to a script.
