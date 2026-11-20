# Exact: gaps the Autoradio lab will hit, and the cheapest fix

Part of [EXACT.md](../EXACT.md). Written against `76cd50f9e`, corrected against `de212e671` on
2026-11-18, re-read against `f53347503` on 2026-11-20 (the row's last sentence says what changed; `file:line` marks are at `76cd50f9e` unless marked). Marks: **[run]**,
**[src]**, **[doc]**, **[seen]** (observed by our app builders on the simulator, 2026-11-11, not read
in source), **[unrun]**. "Upstream" links: none searched yet; no
Exact issue or PR numbers are known to us. `issues/` in the Exact repo holds
their tracker as files [src].

| # | Gap | Evidence | Cheapest fix |
|---|---|---|---|
| 1 | The app module cannot take audio claims; only packages can | `ExactExtensions.swift:374`, `AudioSession.swift:42` (`package` scope) [src]; at `f53347503` the scope is unchanged (two members of `AudioSession` gained `package`, still not `public`) | Write the audio *session* code as a package (works, [run]); the logic around it is `app.ts`. Avoid an upstream patch. |
| 2 | No claim for "mic open, hi-fi output, no processing" by name; every capture claim is `.playAndRecord` | `AudioSession.swift:238-262` [src] | Use mode `default`/`measurement` with `allowBluetoothA2DP` and no HFP; measure on a phone. If it fails, upstream: a claim kind with `.playAndRecord` + `bluetoothHighQualityRecording`. |
| 3 | An empty options list silently means `defaultToSpeaker + allowBluetoothHFP` | `NativeHatches.swift:239-243` [src]; at `f53347503` that file is `host/apple/Sources/ExactNativeViews/NativeHatches.swift` (moved, not re-read) | Always pass explicit options. Document in AutoradioKit's session-owner adapter. |
| 4 | Mode switch = release all, re-claim; session briefly falls to `ambient` and may deactivate | `AudioSession.swift:263-285` [src] | Hold an Exact-side hold across the switch (a muted `audio` element or the speech-playback claim) if the gap is audible; measure first. |
| 5 | No category-change log | [src] | Log in our adapter on every claim, release and event; keep a ring buffer for the lab's trace. |
| 6 | Now Playing and remote commands only for `audio`/`video` elements with `metadata=` | `NowPlaying.swift:1-11` [src] | Play the programme in a Contract `audio` element, or add `MPNowPlayingInfoCenter` ourselves; test for command-center collisions. |
| 7 | App module `ExactModule` and package views cost "a view of its own" per hatched node | `ExactNativeModule.swift:~255` [src] | Keep native views few (one timeline, not one per segment). |
| 8 | Push from a *native package* into the UI is "ask the source again", not a value stream (a stream from a server is `fetch` with `exactStream`, which does push) | `NativeModule.swift`, `ExactNativeModule.swift:161` [src]; `docs/reference.md:2605` | Snapshot struct plus `changed` at bounded rate; for 10-20 Hz meters draw natively in a package view. |
| 9 | Remote notification registration refused | `docs/contract-grammar.md`, `host/apple/activities.mjs:28` [src] | Swift package of ours or upstream change. Not needed for v1 if push is later. |
| 10 | ~~No OTA logic delivery~~ Corrected: signed plans, assets and one TypeScript module pair update over the air with `deploy.store.<platform>` `"A"`; native code never does. Only a TypeScript and a Rust module together are not implemented | `scripts/app.schema.json:1208`, `README.md:797-801` at `de212e671` [src] | We ship `"0"` for now. Once the logic is in `app.ts`, deciding `"A"` is an owner act (signing keys, a receipt, exact grants: `recipes/mobile-build.md:7-10`). Prompts and models stay server-fetched data. |
| 11 | Bun 1.4.2 exactly; at `76cd50f9e` `setup` fetched every platform's Hermes. At `de212e671` its docs list the host and Apple bundles (`reference.md:35-41`) and say the Android bundle comes "with the target" (`README.md:790`); our Android build still needed the bundle installed by hand | [run] | Lean install (see `build-and-ship.md`, "Android"); script it in `mobile/`. |
| 12 | Free provisioning needs an Xcode account; Exact does not create profiles | [run] | Owner signs in under Xcode > Settings > Accounts; one stub project builds the profile; then `ios --device`. |
| 13 | 15-minute cold build, 3.8 GB `target/` per checkout and per app; parallel worktree builds overloaded the Mac | [run] | One shared build tree, one build, one simulator at a time; keep `target/` and `~/.cache/exact` between runs; CI mirror on the agents VM. |
| 14 | The decision records `llp/` are missing from the tree | `ls llp` fails [run] | Treat docs and source comments as the spec; watch `QUEUE.md` and `rules/DEFERRED.md` for planned changes. |
| 15 | `context.agent` does not silence a package's audio; the driver attaches within a fixed 20 s; two drivers on one simulator conflict; a speech alert blocks it | `testing-and-debugging.md` [run, src] | Branch on `context.agent`; virtual engine under the agent; one simulator per driver; the phone proves hardware. |
| 16 | Feedback notes may leave the machine (workers.dev) | build output [run] | `EXACT_FEEDBACK=0` everywhere. |
| 17 | Custom timeline performance unknown | `ui-model.md` [unrun] | Prototype: SVG first, then a package `UIView` drawn with Core Graphics. |
| 18 | Texture rendering | `text markup="markdown"` reader exists; links and taps through atom pills and `atompress`; no documented footnote tap; fixed heading scale [src] | Evaluate `markdown` and `Text` runs with links before writing a renderer; citations as atoms is the candidate (`capabilities.md` section 5). |
| 19 | A package cannot depend on a SwiftPM library or import a third-party module | `host/apple/build.mjs` `swiftModuleArgs` [src]; proven by the models lane | Settled for Apple: copy the kit into the package by script; third-party code as static xcframeworks in the app module (`native-modules.md`). This is the cost of putting logic in Swift: it is copied, not shared, and Android gets none of it. **At `f53347503` [src]:** upstream now takes SwiftPM products: `swiftPackages: [{url, exactVersion, products}]` in a package's `apple/exact.json` or `app.json` `host.ios`/`host.macos`, locked by a committed `Package.resolved` (`bun exact.mjs lock`), built into one shared dynamic library (`host/apple/compose.mjs` `swiftPackageGraph`; commit `7209c04ad`; `scripts/app.schema.json` `$defs.swiftPackages`). That would replace our xcframework copies of FluidAudio and llama.cpp; not adopted until a build passes at the new pin, and unproven for these two libraries. |
| 20 | Wake word, speaker verification, on-device models need continuous background audio | [unrun] | Package-owned engine with a `background:true` claim; verify the screen-locked behaviour on the phone first (the spike's step 4). |
| 21 | No `preserve-3d`, no `matrix`/`skew`, 3D axis/z/perspective do not animate | `SpaceTransform.swift:1-5`, `property.rs` [src] | Package `CATransformLayer` view or pure-Contract projection (`rendering-and-motion.md`). |
| 22 | `mix-blend-mode`, animated backdrop blur, particles, Lottie/Rive unavailable | `SvgIsland.swift:283`, `contract-for-humans.md:1036` [src, doc] | See the cannot-do table in `rendering-and-motion.md`. |
| 23 | Keyframed 3D rotation likely lowers to z only on iOS; iOS drag-timeline run not found | `box_motion_tests.rs:117` [src]; [inferred] | Use `transition`/frame task; Spark `pan` pattern. Verify on device. |
| 24 | The agent driver's 20 s attach bound is fixed | `scripts/agent.mjs:567-579` [src] | Retry; warm launch; clear alerts. Upstream: make it configurable. |
| 25 | (Correction) Text flow around an obstacle was listed as missing. It exists: `wrap-flow`, `shape-outside`, `shape-margin` | `apps/textflow/app.contract`, `README.md:597` [src] | Use it for side notes and figures; unrun by us; supported shapes beyond `circle()` unverified (`capabilities.md` section 4). |
| 26 | Metal Toolchain not installed: no vello Canvas 2D, SVG filters on Core Image | `xcodebuild -showComponent metalToolchain`: uninstalled [run]; `host/apple/build.mjs:155, 872` [src] | Install with `xcodebuild -downloadComponent MetalToolchain` under real Xcode if an animating Canvas 2D or SVG filters matter. Not needed for boxes, text, lists. |
| 27 | A GPU canvas with children drew nothing on Android (QUEUE.md, 2026-10-16). At `de212e671` the reader gives a `Composite` surface its children's picture: "built and unit tested on a Mac, not seen on a device" | `host/android/README.md:1728-1730`, `QUEUE.md:566` [src] | Give a GPU surface no children, or keep the Contract form (pure Contract mark), until seen on a device. |
| 28 | GPU module ABI is unversioned; the crate must sit in the Exact workspace; build time unmeasured | `gpu/README.md:3-4` [src] | Rebuild host and module together; pin the sha; measure the first build. |
| 29 | A native `buttonStyle` button (`bordered`, `filled`, `glass`) cannot be themed: `text` takes no `font-family` (`lower-button-style`), the label colour ignored the theme, `filled` tints system blue | [seen] | `button appearance="none"` wrapping one themed face component (`ui-model.md`). |
| 30 | `iframe` has no `srcdoc` | [seen] | Write the HTML to `app:/cache/x.html` and use `iframe src="app:/cache/x.html" sandbox=""`. |
| 31 | `padding-bottom` on a scroll's content column was not a reliable end inset; padding was not re-tested | [seen] | A trailing spacer component. |
| 32 | The driver cannot tap what the floating bar or keyboard covers, and cannot drive a `scroll id=` without a `testId` | [seen] | Layout: primary buttons above long content, scroll frames that end above the bar, `testId` on scrolls (`testing-and-debugging.md`). |
| 33 | `simctl spawn defaults write` does not override a value the app wrote itself | [seen] | Delete the app's preferences plist, set state with the app's controls (`testing-and-debugging.md`). |
| 34 | `text` collapses `\n` | [seen] | `white-space="pre-wrap"`. |
| 35 | Field or prop names `when` and `from` were risky in a contract | [seen]; whether the compiler refuses them is unchecked | Use other names (`stamp`, `sender`). |

## What to ask upstream first

1. A documented way for the app module to take audio claims.
2. A `claim` log (category, mode, options, route) surfaced in `agent logs`.
3. Local SwiftPM dependencies for packages.
4. A device-signing path that accepts an existing Xcode-managed profile.

## Open questions

- Is any of gaps 2, 4, 6 or 21 to 24 already tracked in Exact's `issues/`?
- Does the phone spike show A2DP output held with the mic open?
- How audible is the release/re-claim gap?
