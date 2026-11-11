# Exact: native Swift modules and packages

Part of [EXACT.md](../EXACT.md). Pinned commit `76cd50f9e`. Marks: **[run]**,
**[src]**, **[doc]**, **[unrun]** as in the main page.

## Two ways in; only one reaches the audio session

| | App module | Package |
|---|---|---|
| Where | `modules/apple/*.swift` (or an `*.xcframework`) in the app | an npm-style package: `exact.json` interface plus `apple/exact.json` implementation, added as a `file:` dependency |
| Shape | one `ExactModule` subclass, named by `let exactModule: ExactModule.Type = X.self` | a class conforming to `ExactPackage` (`host/apple/modules/ExactPackage.swift:289`) |
| Entry points | `later(_:reply:)`, `call(_:)`, `views`, hatch methods | `later(_:_:preferred:reply:)`, `call`, `view(_:id:props:events:)`, `displayed`, `bound`, `foreground`, `permissions` |
| Declared names | none; TS calls `native.call/later/watch` | typed `names`, `shapes`, `views`, events in `exact.json` |
| Audio session claims | **no** | **yes**, `context.audioSession.claim(...)` |
| Verified | source only | **[run]** (the spike) |

[src: `host/apple/modules/ExactNativeModule.swift:227-243`, recipe
`docs/recipes/native-module.md`.] The app module is `dlopen`ed from
`libexact_modules.dylib` (648 KB in our build [run]) after the first painted
frame. The ABI is a versioned C function table (`NativeModule.swift:1-70`),
with every entry on the main thread.

Grep for `audioSession` in `host/apple/modules/*.swift`: the only place the
context is built with an `ExactAudioSession` is `ExactExtensions.swift:374`,
for packages [src].

## Adding our code: the package recipe [run]

Files (real, in `mobile/exact-spike/package/`):

`exact.json` (interface):

```json
{
  "kind": "interface", "name": "@choir/spike", "version": "1.0.0",
  "substitute": "implementation",
  "shapes": { "SpikeStatus": { "state": "string", "rms": "number", "frames": "number", "log": "string" } },
  "names": [
    { "source": "spikeStatus", "live": true, "answers": "SpikeStatus",
      "substitute": { "default": { "state": "none", "rms": 0, "frames": 0, "log": "" } } },
    { "send": "spikeMode", "options": { "mode": "string" }, "answers": "SpikeStatus" }
  ]
}
```

`apple/exact.json` (implementation): `"kind":"implementation"`,
`"implements":"@choir/spike@^1.0"`, `"platform":"apple"`, `"targets":["ios"]`,
`"sources":{"swift":["Spike.swift"]}`, `"class":"SpikeModule"`.

`package.json` exports `"."` and `"./apple"`. The app's `package.json` lists
`"@choir/spike": "file:../spikepkg"`, then `bun install`. The app's Contract
imports names with `use spikeStatus, spikeMode, SpikeStatus from "@choir/spike"`.
No `app.json` entry is needed for a package [doc: contract-for-agents.md:2349].

Swift skeleton:

```swift
import ExactPackage
public final class SpikeModule: ExactPackage {
    private let context: ExactPackageContext
    public init(context: ExactPackageContext) { self.context = context }
    public func later(_ name: String, _ args: [String: Any], preferred: Any?,
                      reply: @escaping ([String: Any]) -> Void) { /* answer once */ }
}
```

The class is made once per session, on the main thread.

## Calling convention

**In (Contract/TypeScript to Swift)** [src]:

- A declared `source` (`resource st = spikeStatus() ...`) or `send`
  (`send moded = spikeMode(mode="listening")`) arrives as
  `later("spikeMode", ["mode": "listening"], preferred: nil, reply:)`.
  Keyword arguments become the `args` dictionary. Reply exactly once, from
  any thread, with a JSON-shaped dictionary.
- A non-object answer (list, bool, `NSNull` for an absent option) replies as
  `["exact:answer": value]` (`ExactPackage.swift:294-296`).
- A synchronous `call(_:_:) throws` answers within a 100 ms budget on the main
  thread; the web has no equivalent, so portable code falls back to `later`.
- For an app module, TypeScript uses `native.call({op: ...})` and
  `native.later({op: ...})` and `native.watch(topic)` inside a source
  [doc: recipes/native-module.md].

**Out (Swift to the UI)** [src]:

- `context.changed("<pkgShortName>.<source>")` asks again every live source of
  that name; the Contract resource updates. Our topic is
  `spike.spikeStatus` [run: lines in the log show `changed` firing the
  re-reads]. The bridge is "ask again", not "push a value": keep a snapshot
  struct and answer `later("spikeStatus")` with it.
- `context.emit(event, args)` raises a declared `event` name on a Contract node
  (see `packages/fixtures/tally/exact.json`, `"event": "tallied", "raised": "emit"`).
- A package view's `events` object sends `press`, `change`, `focus`, `blur`,
  `key`, `submit`, `load`, `message` (`ExactPackage.swift:35-43`).
- `context.log(line)` shows in `agent ios logs`; `print` does too, `NSLog` does
  not reach `logs` but reaches the process stderr that the driver echoes
  prefixed `app:` [run: our `NSLog("SPIKE ...")` lines came back through the
  driver this way].
- Rate: the spike publishes at most once per second. Coalesce; there is no
  documented rate limit [unrun].

## Threading and lifecycle

- Every module entry (`init`, `later`, `call`, view create/props/destroy)
  runs on the main thread. Long work goes on our own queues; replies and
  `changed` may be called from any thread [src: ExactNativeModule.swift
  header comment, NativeModule.swift:1-20].
- Our `AVAudioEngine` tap callback runs on the audio thread; the spike hops to
  main with `DispatchQueue.main.async` before touching state [run].
- `foreground(_ shown:)` is called on app foreground changes (package protocol,
  `ExactPackage.swift:316`); `destroy(_:)` at session end [src].
- The module instance lives per session; hot reload of Contract/TS does not
  restart it, a Swift edit rebuilds the app (`bun exact.mjs ios`) [doc:
  recipes/native-module.md].
- Crash and restart policy for our own state is ours; Exact does not resume it.

## Errors

- Throw `ExactPackageRefusal("message")` from `view(...)` / `setProps` to
  refuse; `ExactNativeRefusal` from an app module's `call` [src].
- Reply `["error": "NotSupportedError"]` for an unknown name (the spike does).
- Permission and hardware states are "facts" in packages, for example
  `microphoneStatus` in `packages/media-capture/exact.json`; mirror that
  pattern for our own permission-sensitive names [src].

## Packaging our Swift package (AutoradioKit)

Settled by reading the pinned source and proving it (lane `ar-models`,
2026-11-10, branch `origin/lane/ar-models` `mobile/README.md`):

- **A package may not depend on a SwiftPM library or import any third-party
  module.** It is compiled by `swiftc` alone (`host/apple/build.mjs`
  `swiftModuleArgs`): only the files listed in `sources.swift`, with
  `-I <packages dir>`; no `-F`, no library `-I`, no SwiftPM [src]. (A package may
  declare `native.library` for a Rust-linked dynamic library; that is not a
  Swift route.)
- **Third-party Swift and Core ML code goes in the app's own module** as a
  static `*.xcframework` in `modules/apple/`. The app module is compiled with
  every such xcframework linked (`frameworkSlice`): a static `lib*.a` slice
  gives `-I Headers -L dir -l<name>`; a static framework slice gives `-F
  -framework`. `import FluidAudio` works there [src; proven by a simulator
  build with FluidAudio linked, lane report].
- **Dynamic frameworks are not embedded by the build** ("a dynamic library is
  not linked into the module"); that they would then fail to load at launch is
  inferred, unrun. The official llama.cpp release is a dynamic framework with
  no simulator slice, so it cannot be used as is.
- No SwiftPM dependency, CocoaPod or remote URL is read for an app or package.
- The app module can import package modules (`-I` of the packages directory). A
  package reaches app-module code only by runtime lookup: the app module defines
  an `@objc(...)` registrar class and the package finds it with
  `NSClassFromString`. No registrar class means no models; the app still builds.
- A `.swiftmodule` placed in a slice's `Headers` directory is found by `-I`.
  `xcodebuild -create-xcframework -headers` refuses a `.swiftmodule` without a
  `.swiftinterface`, so copy the module directories into each slice's `Headers`
  afterwards.

How our app wires it (`mobile/` on the lane branches; scripts may have moved):
`AutoradioKit` stays dependency-free and is copied into the package;
`scripts/sync-exact-package.sh` copies kit sources in (rewriting `import
AutoradioKit` to `import ExactPkg_autoradio`), writes `exact-package/apple/exact.json`
and copies model sources and xcframeworks into `AutoradioLab/modules/apple/`;
`scripts/gen-exact-interface.py` writes the interface;
`scripts/build-xcframeworks.sh` builds the static slices from pinned tags
(FluidAudio by `swift build --triple arm64-apple-ios18.0[-simulator] --sdk
<iphone sdk> --disable-default-traits --traits TTS`, about 10 minutes per slice
on a loaded Mac; llama.cpp by its own `build-xcframework.sh`, static libraries
merged with `libtool`). Third-party sources live in `mobile/.deps` (gitignored).
Whether a package's `sources.swift` may point outside its own directory was
not tested; our answer is to copy the kit in rather than reference it.

## The spike, end to end [run, simulator]

```
$ bun exact.mjs agent ios tree "tap listen" "clock settle" state
...
app: SPIKE stopped cat=SoloAmbient mode=Default in=[] out=["Speaker"]
app: SPIKE claimed listening in 363ms cat=PlayAndRecord mode=Default in=["MicrophoneBuiltIn"] out=["Speaker"]
op 2/4 `tap listen`: the app hung up (exit code 0)
```

The Contract button reached Swift, the claim succeeded, and the session
changed from `SoloAmbient` to `PlayAndRecord`. The tone and RMS lines never
printed; see the open problem in `testing-and-debugging.md`.

## Open questions

- What is the real coalescing behaviour of `changed` at 20 Hz?
- Can a package hold a long-lived background task (wake-word detector) across
  a locked screen without Exact's foreground logic dropping it?

## Native views and mutation results

- Native view cost, props, `context.frames` and the `CATransformLayer` recipe:
  `rendering-and-motion.md` sections 6 and 8a.
- `mutation exp as shape X` reads as `option<X>`; a field read is
  `type-not-a-record`. Use `derive v = match exp { case some(e) => e.field,
  case none => "" }` (lane `ar-integrate` report [run]). `share(title=,
  files=[path])` takes a path held in a derive and compiles.
- Under the driver, `context.agent` does not silence an audio claim: ours was
  real on the simulator, and an `AVAudioEngine` we start opens the simulator's
  hardware (`audio.md`, `testing-and-debugging.md`). Branch on `context.agent`
  for anything that can block the main thread.
