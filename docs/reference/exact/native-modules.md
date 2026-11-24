# Exact: native Swift modules and packages

Part of [EXACT.md](../EXACT.md). Written against `76cd50f9e`, corrected against
`de212e671` on 2026-11-18 (`file:line` marks are at `76cd50f9e` unless marked).
Marks: **[run]**, **[src]**, **[doc]**, **[unrun]** as in the main page.

## When to write a package at all

Only for what the platform alone knows: the audio session and its claims, the
microphone, on-device models, a system object, a platform widget. The recipe's
own example is the device's thermal state, "which TypeScript cannot" read
(`docs/recipes/native-module.md:4-5`) [doc]. Network calls, streams from a
server, storage, secrets, sign-in, sync, parsing and every decision about what
to do next are `app.ts` (`EXACT.md`, "How Exact means an app to be built"). A
package is a narrow interface between shared code and the platform; the wider
it is, the more there is to implement again on Android. A package that
carries the app's logic is the defect recorded in
`docs/problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md`: the
Apple implementation is the only one, so Android has nothing behind the pages.

**Android's shape** [src, `host/android/README.md:796-806`, de212e671]. A package
has one interface (`exact.json`, `"kind": "interface"`) and one implementation
per platform, each under its own folder and exported from `package.json`
(`"./apple"`, `"./android"`, `"./web"`). An Android implementation is
`android/exact.json` (`"platform": "android"`, `"implements": "@pkg@^1.0"`,
`"sources": {"kotlin": [...]}`, `"class": "pkg.Module"`) and a Kotlin class that
subclasses `ExactExtension`: `later(name, args, reply)` for each source and
send, `release`, `dispose`, `foreground(shown)`, `resumed()`, `preferred`, and
an `ExtensionContext` with `data`, `cache`, `temporary` (the directories behind
`app:/data`, `app:/cache`, `app:/tmp`), `changed(topic)` (ask the topic's
readers again, the Android twin of Swift's `context.changed`), `wake`,
`requestPermissions`, `startActivityForResult` and a track table. The build
compiles it into the APK when the app binds the package; Maven libraries are
declared in the package and locked (`README.md:845-886`). A whole one is
`packages/geolocation/android/Geolocation.kt` (`GeolocationModule(context:
ExtensionContext) : ExactExtension(context)`). Android runs one session a
process. Native views are Kotlin `ExactExtension.view`
(`docs/contract-for-agents.md:3315-3325`).

What a platform does with an interface name it has no implementation for:
Exact's standard packages answer `NotSupportedError` and `available: false`
(`docs/reference.md:3143`), and under the agent an interface that is its own
substitute answers (`host/android/README.md:832-834`). For *our* interface on
a platform with no implementation outside the agent, the source states
nothing [unknown].

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
No `app.json` entry is needed for a package [doc: `contract-for-agents.md:3315-3317` at `de212e671`].

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

**Native asks shared code (the one pattern, 2026-11-21, `retrievalAsk`).** Exact has no call from native into `app.ts`, and a voice turn needs shared
code's answer for the words just heard. The pattern: native keeps the ask (an id and its inputs) and `touch`es a package source; a root `resource`
in `members-retrieval.contract` reads that source and passes the ask to a data-module source; that source does the work and hands the answer back
through one app-module op (`ModelsEntry.swift`, `retrievalAnswer`); native waits on the id with a deadline and goes on without an answer when it
does not come (`RetrievalFacts.swift`). The facts only native holds (here: the Swift run sheet's entries, the open pages) ride in the ask as one JSON string.
[unrun on a device: the source is read only while the app runs its root resources; the proof's ask drive logs `AUTORADIO retrieval ask answered|no-answer`.]

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
  a locked screen without Exact's foreground logic dropping it? Exact's rule
  for shared code: `page.frozen` becomes true on iOS in the background unless
  active audio with the background audio mode keeps execution eligible ("a
  configured playback category alone grants nothing"), and on Android when the
  Activity stops with no promoted foreground-service claim; macOS, Linux and
  Windows never freeze (`docs/contract-for-agents.md:4923-4932`,
  `recipes/suspending-feed.md:35-41`). Unrun on a phone. The rule's source and what our app meets: [audio.md](audio.md) "What keeps shared code running in the background".

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

## Moving a page to shared logic: the pattern

Written 2026-11-18 from the Paper page, the first page moved (lane `exact-paper`;
`mobile/AutoradioLab/paper.ts`). The rule is CLAUDE.md, Product: the phone app is
Exact, only voice is native. A page moves when its data source is answered by
`app.ts` instead of by the Swift package. Every step below was done and run on
the web host; the phone and Android builds are the lead's.

**1. One name, one answerer.** A source is answered by the Swift package or by
`app.ts`, decided by the name, never both and with no switch. A name the
contract imports with `use name from "@choir/autoradio"` is the package's. A
name the contract does not import, used as `resource x = name(a, b) as shape
Shape else empty()`, is `app.ts`'s (`docs/recipes/fetch-loading-error.md`,
`reference.md` "Generate TypeScript data-source types": the generated `Sources`
type has exactly the undeclared names). Both kinds live in one app: the web
build ran with the package's `openPages`, `theme`, `demoInfo` and our own
`paperFront` side by side. To move a page: delete the name from
`exact-package/interface/<page>.json` (and its Swift `handle(...)`), delete the
`use name` line, keep the resource line but pass **positional** arguments (a
named argument is a compile error for a source the package does not own:
`type-named-argument`). Order the arguments as `app.ts` destructures them.

**2. Where types come from.** The package's shapes went with the name. Declare the
shapes in a `.contract` file (`paper-types.contract`) and `use PaperFront from
"./paper-types.contract"` in every contract that read them. Never write the same
shape twice: `bun exact.mjs contract build` writes `app.contract.d.ts` (gitignored)
from the contract, and `app.ts` imports `Sources`, `Answer`, `Storage` from it, so a
field added in the contract is a type error in TypeScript until answered. The
answer is held to its shape at run time, exactly (no extra field, no missing one).

**3. Shape of the TypeScript.** `app.ts` stays small: the grants, a `Sources`
map with one line per moved name, and the `answer` dispatcher. Logic lives in
modules it imports (`paper.ts` the source, `paper-model.ts` pure data,
`paper-layout.ts` pure layout, `api.ts` the one place that builds requests).
Do not name a module `server.ts`: Exact reads it as the test backend. A source
never throws for a failure the page has words for; it answers the page's own
state (`status: "error"`, `message`), so the page shows its existing error and
retry, never a spinner word. Time is never read: `Date.now()` and `new Date()` are
refused. The server's `Date` header is "now" for ranking and "2 h ago"; a
fixed `Date.UTC(...)` of a parsed string is fine. State between answers is a
module-level `Map` (kept lists); what was asked is forgotten when a `tick` argument changes (a pull to
refresh) and the next fetch replaces rows by id (clips.ts); a fetch that lands after the account
changed is dropped (`accountGeneration()` taken before the request).

**4. Grants.** `export const grants` is read by the build without running the
module: a string or a template of constants **in `app.ts` itself**. An imported
constant is refused (`grants must be a constant the build reads without running
the module`). So the origin is `export const ORIGIN = 'https://choir.news'` in
`app.ts` and `api.ts` imports it; the grant is `net.fetch ${ORIGIN}`. A file the
module writes (the offline copy) needs `fs.read` and `fs.write` for its exact
path, spelled in the grant and in the module, and a test (`paper.test.ts`, "the
grants") asserts the two agree. Server routes the page reads must be public;
a route that needs a token is a later page's step (`store`, `secret.keep`).

**5. Shared files from outside the app.** `app.json` `typescript.sources` maps a
folder outside the app to an import name: `{"fixtures": "../fixtures", "layouts":
"../layouts"}` lets `paper.ts` import `./fixtures/demo/front.json`. A committed
symlink of the same name (`AutoradioLab/fixtures -> ../fixtures`) lets `bun test`
and the editor resolve it; the build does not follow symlinks, it follows the
mount. This is how Go, Swift and Bun tests share one fixture.

**6. Tests, failure list first.** (a) `paper.test.ts`, `bun test`: call
`paperFront(args, storage)` with `globalThis.fetch` stubbed and a fake `storage`
(a map of paths to bytes); the canned body is the real captured response
(`mobile/fixtures/paper/front-live-*.json`, public content, nothing stripped).
Cover: the real body, an empty list, a 5xx, a thrown transport error, malformed
bodies, a saved offline copy, one request per list, ordering and trimming rules.
Bun has no grants and no shape check; the build and `test web` supply them.
(b) `paper.test.contract`, `bun exact.mjs test web`: `answer fetch "<origin>"
with 200 <one-line json, at most 64 KiB>` leads the file or a test; `fail fetch
"<origin>"` fails the first load. Assert `expect state <resource>.<field>`. The
deck mounts a page only when the package's `openPages` says which is on top, and
the web host has only that source's placeholder, so on the web the page's own
tree is not drawn: assert the resource, and look at the page on the simulator
and on Android. `agent` has no `answer fetch` step; only authored tests do.
(c) `contract build app.contract --json --errors-only` must print `[]` (it also
type-checks `app.ts`).

**7. What to delete in the same change.** The Swift source and everything only it
used (here: PaperSource, PaperComposer, LayoutEngine, TextMeasure, MyEdition,
PaperViews, PaperCurrentList, FrontOrder, the interface name, its shapes, its Swift
tests); the places other Swift read what it produced must be fed another way or
dropped, and the report says which. What stays: Swift the radio or another page
still uses (`PaperStory`, `StretchCandidates`, `RadioProgramme` read the front for
the radio). Run `python3 -I mobile/scripts/gen-exact-interface.py` to rewrite
`exact-package/exact.json`, and `mobile/scripts/sync-exact-package.sh` plus
`bun install` in `AutoradioLab` so the build sees the smaller package. Whatever
Swift data the moved page still needs (here the reader's saved topics) becomes a
small package source the contract passes in as an argument, until its owner page
moves too.

## Mail on the phone: plain files written by shared code (2026-11-19 lane `exact-mail`; the seal removed 2026-11-24, lane `exact-rest`)

The Mail pages, the signed-in requests to the listener's computer and the phone's copy of the mail are `mail.ts` and its siblings in `mobile/AutoradioLab`
(`mail-model.ts` pure data, `mail-sanitize.ts` the allowlist HTML cleaner, `mail-store.ts` the copy). One send stays native (`mailOpenReview`: the in-app web view);
"Read this to me" is shared code's radio (`radioListen`), and `mailAsk` is shared code's too.

- **At rest.** The owner, 2026-11-23: "yes, get rid of mail seal. i do want mail stored on phone's disk". There is no cipher and no key: shared code writes plain JSON
  under `app:/data/mail` (`rows-<folder>.json`, `msg-<hash of the id>.json`, `state.json`), and the platform's own file protection is the only seal. The native pair
  `mailSeal`/`mailOpen`/`mailForget`/`mailCan` (`Choir/MailSeal.swift`) and `mail-seal.ts` are deleted; the earlier sealed files (`*.bin`) are never read (the app has not launched).
  A file that does not parse is removed and fetched again. Every file carries `owner`, a digest of the sign-in key it was written under; a file of another key is removed, never served, and a write needs a signed-in key.
  `app:/data` is `Library/Application Support/exact/<app id>/data` and IS in a device backup (`host/apple/src/picker.rs:46-51`): the mail is in iCloud and iTunes backups. Shared code sets no file-protection attribute,
  so the OS default class applies (readable after the first unlock; the sandbox keeps other apps out). Android: no Android host is built in this tree; whatever the Exact host's private data folder and the device's Auto Backup give is what applies, and it is unverified. Mail is not yet in local search (the plain mail sync in `content.ts` and `mailDocs` in `search.ts` were deleted with the seal work;
  a search over the phone's copy is now possible and is owed).
- **Show original.** `iframe` has no `srcdoc` (gaps.md 30), so the cleaned document is written PLAIN to `app:/cache/mail-<hash>.html` only while it is asked for, and every
  other `mail-*.html` is removed on each message answer, at sign-out and at every launch. `app:/cache` is `$HOME/Library/Caches/exact/<app id>` (`host/apple/src/picker.rs:52`):
  under Apple's rules Library/Caches is not in a device backup; the file is plain on the phone while the page is open (residual). A link in it is inert (no `href`; its
  address is written after it). A body over 120 000 characters shows a statement that it is too large to show in full; the message page shows the text, cut at 200 000 (the HTML is read up to 300 000).
- **Sign-out.** Shared code leads (`account.ts`): `beginSignOut` moves the account generation and turns mail writes off BEFORE the native sign-out; a store write checks the generation
  before and after it writes; `wipeMail` removes the files.
