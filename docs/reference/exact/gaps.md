# Exact: gaps the Autoradio lab will hit, and the cheapest fix

Part of [EXACT.md](../EXACT.md). Pinned commit `76cd50f9e`. Marks: **[run]**,
**[src]**, **[doc]**, **[unrun]**. "Upstream" links: none searched yet; no
Exact issue or PR numbers are known to us. `issues/` in the Exact repo holds
their tracker as files [src].

| # | Gap | Evidence | Cheapest fix |
|---|---|---|---|
| 1 | The app module cannot take audio claims; only packages can | `ExactExtensions.swift:374`, `AudioSession.swift:42` (`package` scope) [src] | Write all audio code as a package (works, [run]). Avoid an upstream patch. |
| 2 | No claim for "mic open, hi-fi output, no processing" by name; every capture claim is `.playAndRecord` | `AudioSession.swift:238-262` [src] | Use mode `default`/`measurement` with `allowBluetoothA2DP` and no HFP; measure on a phone. If it fails, upstream: a claim kind with `.playAndRecord` + `bluetoothHighQualityRecording`. |
| 3 | An empty options list silently means `defaultToSpeaker + allowBluetoothHFP` | `NativeHatches.swift:239-243` [src] | Always pass explicit options. Document in AutoradioKit's session-owner adapter. |
| 4 | Mode switch = release all, re-claim; session briefly falls to `ambient` and may deactivate | `AudioSession.swift:263-285` [src] | Hold an Exact-side hold across the switch (a muted `audio` element or the speech-playback claim) if the gap is audible; measure first. |
| 5 | No category-change log | [src] | Log in our adapter on every claim, release and event; keep a ring buffer for the lab's trace. |
| 6 | Now Playing and remote commands only for `audio`/`video` elements with `metadata=` | `NowPlaying.swift:1-11` [src] | Play the programme in a Contract `audio` element, or add `MPNowPlayingInfoCenter` ourselves; test for command-center collisions. |
| 7 | App module `ExactModule` and package views cost "a view of its own" per hatched node | `ExactNativeModule.swift:~255` [src] | Keep native views few (one timeline, not one per segment). |
| 8 | Push into the UI is "ask the source again", not a value stream | `NativeModule.swift`, `ExactNativeModule.swift:161` [src] | Snapshot struct plus `changed` at bounded rate; for 10-20 Hz meters draw natively in a package view. |
| 9 | Remote notification registration refused | `docs/contract-grammar.md`, `host/apple/activities.mjs:28` [src] | Swift package of ours or upstream change. Not needed for v1 if push is later. |
| 10 | No OTA logic delivery | `scripts/app.schema.json:1003` [src] | Accept; ship binaries. Put prompts and models in server-fetched data, not code. |
| 11 | Bun 1.4.2 exactly; `setup` fetches every platform's Hermes | [run] | Lean install (see `build-and-ship.md`); script it in `mobile/`. |
| 12 | Free provisioning needs an Xcode account; Exact does not create profiles | [run] | Owner signs in under Xcode > Settings > Accounts; one stub project builds the profile; then `ios --device`. |
| 13 | 15-minute cold build, 3.8 GB `target/` per checkout and per app | [run] | One shared checkout; keep `target/` and `~/.cache/exact` between runs; CI mirror on the agents VM. |
| 14 | The decision records `llp/` are missing from the tree | `ls llp` fails [run] | Treat docs and source comments as the spec; watch `QUEUE.md` and `rules/DEFERRED.md` for planned changes. |
| 15 | The agent driver may not exercise real audio; unknown hang with a package that opens hardware | `testing-and-debugging.md` [run] | Branch on `context.agent`; supply a scripted double for tests; run hardware only on the phone. |
| 16 | Feedback notes may leave the machine (workers.dev) | build output [run] | `EXACT_FEEDBACK=0` everywhere. |
| 17 | Custom timeline performance unknown | `ui-model.md` [unrun] | Prototype: SVG first, then a package `UIView` drawn with Core Graphics. |
| 18 | Texture rendering | Markdown element exists, fit unknown [doc] | Evaluate `markdown` and `Text` runs with links before writing a renderer. |
| 19 | A package depending on a local SwiftPM library (AutoradioKit) is unestablished | `native-modules.md` [unrun] | Try listing the kit's files in `apple/exact.json` `sources.swift`; else vendor-copy through a build step; else an upstream patch to `host/apple/compose.mjs`. |
| 20 | Wake word, speaker verification, on-device models need continuous background audio | [unrun] | Package-owned engine with a `background:true` claim; verify the screen-locked behaviour on the phone first (the spike's step 4). |

## What to ask upstream first

1. A documented way for the app module to take audio claims.
2. A `claim` log (category, mode, options, route) surfaced in `agent logs`.
3. Local SwiftPM dependencies for packages.
4. A device-signing path that accepts an existing Xcode-managed profile.

## Open questions

- Is any of gaps 2, 4, 6 or 19 already tracked in Exact's `issues/`?
- Does the phone spike show A2DP output held with the mic open?
- How audible is the release/re-claim gap?
