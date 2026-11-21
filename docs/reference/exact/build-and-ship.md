# Exact: project layout, build, signing, install, pinning

Part of [EXACT.md](../EXACT.md). Written against `76cd50f9e`, corrected against `de212e671` on
2026-11-18, pin moved to `f53347503` on 2026-11-20 (read, not built) (`file:line` marks are at `76cd50f9e` unless marked). Marks: **[run]**
(simulator, 2026-11-10, M1 Mac with another agent building at the same time),
**[src]**, **[doc]**, **[unrun]**. The runnable example is
[`mobile/exact-spike/`](../../../mobile/exact-spike/README.md).

## What each kind of edit costs to see

Marks per cell: **D** documented by Exact (its words, not our run), **M**
measured by us, **?** unknown (neither stated by Exact nor measured here). Exact
paths are at `de212e671`. "Loop" means a native client launched against the web
loop, `host/apple/build.mjs --ios --run --url <url>` or `bun exact.mjs android
--run --url <url>` (`docs/reference.md:1855-1870`; `host/android/README.md:153-165`).

| You edit | Web (`bun exact.mjs web`) | iOS | Android |
|---|---|---|---|
| `app.contract` | In the page in 99 ms on a fresh app (D, `README.md:155-156`); on ours 57 to 90 s (M, below) | Loop: plan replaced, state carried; "seconds" (D, `reference.md:1866,1877`; `recipes/app-json.md:48`); our time ?. `agent ios` / `test ios`: rebuilt as a baked plan (D, `recipes/ios-driving.md:17-19`); a Contract-only rebuild took 12 to 40 s through our build path (M). `bun exact.mjs ios`: bake and link, "compiles no Rust" (D, `reference.md:4693-4698`); "a minute or two" (D, `start-here.md:51`); a warm rebuild plus drive about 60 s (M) | Loop: plan replaced, state carried (D, `README.md:157-159`); time ?. Without the loop `test android` and `agent android` do not build; run `bun exact.mjs android` after the edit (D, `start-here.md:75`); a build after the first ? |
| `app.ts` or a file it imports | As above (D) | Loop: module pair replaced in the running app (D, `reference.md:1866-1868, 4718-4723`). `agent ios`: baked plan (D, `recipes/ios-driving.md:17-19`). A TypeScript-only change took about 57 s through our build path (M). A first use of a newer prelude feature raises `typescriptRuntime` and needs one new binary (D, `reference.md:2084, 2406, 2761`) | Loop: module pair replaced (D, `README.md:158-159`); time ? |
| `app.test.contract`, `app.test.ts` | No build. Caltrain's three tests take about two seconds on the web host (D, `README.md:370-371`); ours ? | No build for `test web`; `test ios` ? | `test android` drives an APK built earlier (D, `start-here.md:75`) |
| Swift package or app module | n/a | `bun exact.mjs ios` (D, `recipes/native-module.md:76`); a Swift change about 60 s warm (M); a cold build that adds Swift about 3 min (M); a first build in a new worktree about 5 min (M) | n/a |
| Kotlin package | n/a | n/a | New APK (D, `host/android/README.md:796-806`); time ? |
| `app.json`, grants, `appId`, assets, fonts, strings, icons, canvas surfaces | Rebuild (D) | Native build; the client keeps the last good generation until relaunched (D, `recipes/app-json.md:47-49`; `reference.md:1877-1879`) | New APK (D, `scripts/agent-android.mjs:1372`) |
| Rust data crate | We have none | `ios-driving.md:18-19` says rebuild; `reference.md:1867-1868` says declared Rust modules reload in the loop; Exact disagrees with itself (D) | New APK; the loop refuses a Rust pair (D, `host/android/README.md:160-164`) |
| First build for the target | `setup` a few minutes (D, `start-here.md:51`) | "Ten to fifteen minutes" (D, `start-here.md:51`); about 15 min (M, below) | Shelf, 6 min 33 s (M, 2026-11-18, below) |

What "a first build ten to fifteen minutes, later builds a minute or two"
(`start-here.md:51`, `README.md:351-353`) refers to: the first native build of a
platform, which compiles Exact's Rust host cold, and the later *native builds*
that reuse it. It does not describe what a Contract or `app.ts` edit costs in the
loop, where no native build runs. Our "57 s for a TypeScript-only change" is a
native build through `mobile/scripts/build-lab.sh` (it syncs the kit,
regenerates the interface, runs `bun install` in the app and then the whole
`ios` verb); whether any run used the loop is not recorded.

## Prerequisites and the exact commands [run]

Exact needs Rust (rustup; the pinned toolchain installs itself), **Bun 1.4.2
exactly**, Xcode 27.1 or later, and Hermes bundles.

```sh
git clone --depth 1 https://github.com/expo/exact exact-checkout
cd exact-checkout && git fetch --depth 1 origin f5334750386192245ee76add13a67ab51fb4e2fd && git checkout FETCH_HEAD   # our pin: mobile/exact.pin

# Bun 1.4.2 beside any other Bun. 1.3.14 is refused:
#   "error: Bun 1.3.14 is older than 1.4.2 or unavailable; set BUN=~/.bun-1.4.2/bin/bun"
curl -fsSL https://bun.sh/install | BUN_INSTALL=$HOME/.bun-1.4.2 bash -s bun-v1.4.2
export BUN=$HOME/.bun-1.4.2/bin/bun

# At 76cd50f9e `bun scripts/exact.mjs setup` also fetched Hermes for Android,
# Linux, Windows, tvOS and the game SDK's crates, each ~190 MB, and stalled for
# us. (de212e671's docs say it installs the host and Apple bundles only,
# reference.md:35-41; not re-run.) Installing only the Apple targets worked:
for t in aarch64-apple-darwin aarch64-apple-ios aarch64-apple-ios-sim; do
  cargo run --manifest-path vendor/ibex/crates/hermes-lean-sys-installer/Cargo.toml -- --target $t
done
```

Result: `~/.cargo/hermes-lean-sys/hermes-vanilla-d412d3bd8512-v5/<hash>/` for
each target (525 MB for five bundles after the aborted `setup`) [run]. At
`de212e671` the set is v6, with an Android arm64 bundle among them
(`docs/reference.md:30-34`); the path will differ.
`setup --check` says what is missing [doc: README.md Quick start].

Run Apple tools outside the Nix shell (the repo shell shadows `xcodebuild`,
`xcrun`):
`env -i HOME=$HOME USER=$USER PATH=$HOME/.cargo/bin:/usr/bin:/bin:/usr/sbin:/sbin DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer ...`.
Exact's `exact.mjs` wrapper reads `$BUN` first, then `~/.bun-1.4.2/bin/bun`.

## Project layout [src: scripts/app-shells.mjs, docs/recipes/app-json.md]

```text
myapp/
  app.contract          UI (first component is the root)
  app.ts                data module: sources, grants
  app.json              identity, host.ios, audio_session, deploy.store, hatches
  strings/en.json       strings, including purpose.<key> permission texts
  assets/               shipped whole (images, sounds, fonts)
  modules/apple/*.swift the app's one native module (optional)
  package.json          dependencies, including packages by file: path
  app.test.contract     tests, run by the driver
```

`exact new <dir>` scaffolds one and writes a unique id `com.example.<name>-<hex>`
and an `exact.mjs` wrapper with an absolute Bun path in it (do not commit that
file). `app.json` fields we use [run]:

```json
{ "id": "news.choir.exactspike", "app": {"id": "news.choir.exactspike", "name": "Exact Spike"},
  "audio_session": "playback",
  "host": { "ios": { "minimumOS": "17.0", "deviceFamily": ["iphone"], "backgroundModes": ["audio"] }, "web": {} },
  "deploy": { "store": { "ios": "0", "web": "0" } } }
```

Minimum iOS defaults to 17.0 and iPad is included unless narrowed
(`scripts/app-shells.mjs:37`) [src]. `fetch` and `processing` background modes
are refused in `backgroundModes`; they come from grants
(`host/apple/plist.mjs:46-48`).

## Build and run on the simulator [run]

```sh
cd myapp && $BUN install
nice -n 10 $BUN exact.mjs ios                 # build, install on a simulator
$BUN exact.mjs agent ios tree "tap listen" "clock settle" state
```

For AutoradioLab run these through `mobile/scripts/exact.sh` (`exact.sh ios`, `exact.sh agent ios ...`), not a raw
`bun exact.mjs`. The wrapper checks the Exact pin and runs `scripts/gen-build-info.sh`, which writes
`build-info.gen.ts` (the commit and time the About page's Build row shows). A raw `bun exact.mjs ios` skips both:
the build fails when `build-info.gen.ts` is missing (it is a static import), and with a stale file About's Build row
shows the previous commit. The row says `unknown` only when the generator found no git commit.

Measured (first cold build; Exact checkout at the pinned commit; M1, another
agent compiling concurrently; compiled under `nice -n 10`):

- Rust: `Finished host-dev profile [optimized] target(s) in 5m 37s` (plus an
  8 s relink after).
- Swift host (SwiftPM): `Build complete! (47.63 sec)`, 142 steps.
- Total wall for the first build: about 15 minutes including the Contract
  compiler build that `exact new` starts in the background.
- Output: `target/clients/<hash>/news.choir.exactspike/ios-simulator/aarch64-apple-ios-sim/embedded/development/standalone/Exact Spike.app`
  - 46 MB `.app`, of which the main binary is 42 MB; dylibs are
  `libexact_modules` 648 KB (our package lives in it), `libexact_web` 784 KB,
  `libexact_svg` 460 KB, `libexact_video` 268 KB, `libexact_midi` 124 KB,
  `libexact_sound` 96 KB.
- Disk: `target/` 3.8 GB for the Exact checkout plus 3.8 GB for the app.

Notices in the log, harmless: "no Metal toolchain, so no ExactSvgFilter.metallib
... draw with Core Image" and "no Canvas 2D GPU module" (install
`xcodebuild -downloadComponent MetalToolchain` to remove them).

The generated `Info.plist` carried [run]: `ExactAudioSession = playback`,
`ExactCapturesInBackground = true`, `UIBackgroundModes = [audio]`,
`NSMicrophoneUsageDescription` (from `strings/en.json`), `UIDeviceFamily`.

## Our own loop is slow, and the mark is most of it [run, 2026-11-18]

Measured in the build tree on `lane/autoradio-integ20`, an M1 Mac:

| What | Time |
|---|---|
| `bun exact.mjs contract build app.contract --json --errors-only` | 64 to 66 s |
| The same with `MarkAt`'s `view` replaced by one empty box | 16 s |
| `bun exact.mjs web`, until the first plan is served | 38 s |
| A one-line edit to `radio.contract` in that loop | 90 s the first time, 57 s the second |
| `bun exact.mjs agent web … tree screenshot` (builds its own copy) | 99 s first, 39 s unchanged |

Exact's figure for a fresh app is 99 ms per edit (`README.md:155-156`). Ours is
about six hundred times that, and the Swift has no part in it: this is the
Contract compiler on our pages. The plan is 5.9 MB with the mark and 1.3 MB
without it; the web build reports `MarkAt: 4616382 B (29 instances)` of a
16 MB `app.js` ("shared bodies counted per use": a component's body is
compiled once per use). The mark is drawn in pure Contract `fn`s
(`mobile/AutoradioLab/mark.contract`), each polygon's points a deep expression,
and it is used 29 times, 12 of them in the motion gallery (`motion.contract`,
a lab page).

What follows: cut the mark's uses to the few the product shows, move lab pages
out of the product app, and find a cheaper form for the mark (points computed
once and shared, or a canvas). The remaining 16 s is still far from Exact's
figure and is not yet explained; `Btn` has 231 uses and `Card` 93.

## Android [run, 2026-11-18, this Mac]

Proven today with Exact's own Shelf sample, not with Autoradio. From the Exact
checkout, `bun scripts/agent-android.mjs build shelf --apk`
(`host/android/README.md:73-75`) built the APK in **6 min 33 s** on the first
build for the target, after three things the first attempt lacked:

1. **The Rust target `aarch64-linux-android` on the pinned 1.99.1 toolchain**
   (`rust-toolchain.toml:3`). The script checks and says so:
   `rustup target add aarch64-linux-android --toolchain 1.99.1`
   (`scripts/agent-android.mjs:1436-1441`).
2. **The pinned Android Hermes bundle**, installed with the hermes-lean-sys
   installer and `--target aarch64-linux-android` (`README.md:278-280`). Exact's
   pages say `exact setup` installs it with the target (`README.md:790-791`;
   `host/android/README.md:174-175`); our first attempt did not have it.
3. **`cargo fetch --locked` for the whole workspace.** The build's own
   `cargo fetch --target` left `jpeg-encoder` unfetched, and its offline
   `cargo tree` then failed.

Environment: `JAVA_HOME` the Homebrew `openjdk@21` (21.0.12; Exact wants 21.0.8
or later in 21, `host/android/README.md:83-84`), `ANDROID_HOME`
`~/Library/Android/sdk` (a default the build also tries, `:77`), Bun 1.4.2 first
on `PATH`. Gradle fetched its 9.8.0 distribution (the committed wrapper, `:20`)
and platform 37.2 (`:23`, `:77`).

The APK installed on an emulator (an android-36 arm64 image, an AVD of its own)
and was driven with

```sh
ANDROID_SERIAL=emulator-5554 bun scripts/agent.mjs android --reader --app shelf \
  --storage s1 "clock data" tree "screenshot <png>"
```

The tree and the screenshot were correct. That is Exact's reader on Android
drawing a TypeScript-data app (`host/android/README.md:256-262`).

**Autoradio itself on Android [run, 2026-11-18].** `bun exact.mjs android` in
`mobile/AutoradioLab` (it added `"android": {}` to `app.json`'s `host`, kept on
the local branch `android-probe` in the build tree, not landed) built and
installed in 260 s on the first build. `bun exact.mjs agent android --storage a1
"clock data" tree logs "screenshot <png>"` drew the deck, the bar, the mark and
the tray with its labels. Under the driver all 114 source reads were answered
`(substituted)` and a send failed: `autoradio.pageOpen: NotSupportedError; it
ends unsent`. Our package has no Android implementation, so the pages have
nothing behind them. One refusal to fix in the contracts:
`-exact-exit-animation: refused on Android (LLP 1063)`; and the compiler warns
that `sf/…` symbols draw only on Apple (`lower-symbol-platform`).

**Not yet done:** a physical Android phone (the owner's old one never appeared
on the USB bus); `android --run --url` against the web loop; the app outside
the driver; the time of a second Android build.
From Exact's own pages, not our runs: minSdk 31 unless `host.android.minimumOS`
says `"10"` or `"11"` (`:22`, `:1545-1548`); `arm64-v8a` only (`:23`); its
controls are the desktop painter's shapes and a sheet or chooser is refused
(`:1642`); `check` is refused for every Android carrier (`:326`).

## Signing and device install (free provisioning)

**Exact never creates provisioning profiles.** It signs with "a development
identity and a provisioning profile on this Mac that covers the phone and the
bundle id", chosen automatically, or `EXACT_IDENTITY` (a SHA-1) and
`EXACT_PROFILE` (a path) override it (`host/apple/build.mjs:26-30`;
`host/apple/devices.mjs:713-749`) [src]. It looks in
`~/Library/Developer/Xcode/UserData/Provisioning Profiles` and
`~/Library/MobileDevice/Provisioning Profiles` (`devices.mjs:713`).

What we found [run]:

- The Mac has two valid "Apple Development" identities for the owner's team id.
- The only profile on disk was for a different bundle id and team, and expired.
- A stub Xcode project with `PRODUCT_BUNDLE_IDENTIFIER=news.choir.exactspike`,
  automatic signing, built with `xcodebuild -allowProvisioningUpdates
  -allowProvisioningDeviceRegistration -destination id=<udid>`, failed with:
  `error: No Account for Team "<team>". Add a new account in Accounts settings
  or verify that your accounts have valid credentials.` and `No profiles for
  'news.choir.exactspike' were found`. Xcode's `DVTDeveloperAccountManagerAppleIDLists`
  `IDE.Prod` list was empty.

**Therefore the owner must sign in to the Apple account under Xcode >
Settings > Accounts.** After that, the stub project builds once (generating the
profile for the bundle id and the phone), and then:

```sh
$BUN exact.mjs ios --device --phone <udid>     # build, sign, devicectl install
$BUN exact.mjs ios --device --run --phone <udid>
```

[doc: reference.md:859-870; unrun here.] The phone must be connected,
unlocked, paired, Developer Mode on. Free profiles last 7 days and cap the
number of bundle ids per week; use one bundle id for the spike and one for the
app [unrun; Apple policy, not Exact's].

Agent driving on a real phone: `bun exact.mjs agent ios --device [--phone
<name|udid>]`; the phone connects outward to a temporary Mac-side port with a
per-launch token, over trusted LAN, unencrypted (`EXACT_AGENT_HOST` overrides
the Mac IPv4) [doc: reference.md:2469-2473]. Keep the app visible.

## Lane build scripts and build discipline (lanes `ar-integrate`, `ar-models`)

Source: `mobile/README.md` on `origin/lane/ar-integrate` and `origin/lane/ar-models`
(`lane/ar-spine` was not on origin). Script names are as the lanes wrote them;
check `mobile/scripts/` on main before relying on one.

| Script | Does |
|---|---|
| `scripts/xe.sh` | runs a command with the Xcode toolchain outside the Nix shell |
| `scripts/setup-exact.sh` | pinned Exact checkout at `mobile/.exact-src` (gitignored) and Bun 1.4.2 at `~/.bun-1.4.2`; `EXACT_REPO=<local clone>` avoids the network |
| `scripts/exact.sh` | wraps `exact.mjs` in `env -i` with the pinned Bun: `exact.sh ios` (simulator), `exact.sh ios --device --phone <udid>`, `exact.sh agent ios ...` |
| `scripts/sim-device.sh` | creates and boots a simulator of your own; export it as `EXACT_SIM` |
| `scripts/build-lab.sh` | syncs the kit into the package, builds, installs on `$EXACT_SIM` (or the phone with `--device --phone`) |
| `scripts/sync-exact-package.sh`, `gen-exact-interface.py` | copy kit sources into the Exact package, write `apple/exact.json` and the interface (per-lane fragments are the plan, design doc section 6) |
| `scripts/build-xcframeworks.sh` | static xcframeworks for third-party Swift/C++ (FluidAudio, llama.cpp) from pinned tags in `mobile/.deps` |
| `scripts/make-profile.sh` | obtains a provisioning profile with a stub Xcode project and `-allowProvisioningUpdates` |
| `scripts/proof-real.sh` | drives the simulator on real data (staging, the drill account) and prints `PROOF OK` or `PROOF FAILED at <step>`; `NOBUILD=1` skips the build |

**Third-party libraries:** static xcframeworks in the app module only
(`native-modules.md`).

**Build discipline (owner rule from the agent reports, 2026-11-10):** a cold
Exact build is about 15 minutes and each worktree compiling its own copy
overloaded the Mac (load average above 100 in model-lane runs). Use **one
shared build tree, one build at a time, one simulator at a time**. Exact's own
rule agrees: each worktree builds into its own `target/` and its scripts refuse
a shared or symlinked one (`AGENTS.md` of the Exact repo [doc]); a private
`CARGO_TARGET_DIR` outside every checkout is fine. Timings: a warm rebuild plus
drive about 60 s, a TypeScript-only change about 57 s, a first build in a new
worktree about 5 minutes with warm caches, a cold build about 15 minutes
(`testing-and-debugging.md`, item 8). Exact keeps shared caches under
`~/.cache/exact` (modules by input SHA-256, registry crates); deleting them only
costs the next first build its time (Exact `AGENTS.md` [doc]). These are
timings of native builds. Exact's loop (table above) swaps a Contract or
`app.ts` edit into a running app with no native build; we have not timed it.

**Merging lanes (seen in our build, 2026-11-11):** a lane's last unproven
commit can break the merged build. Re-fetch and merge every lane head just
before the final proof. Booting a simulator puts the Mac at load about 200 for
minutes and the driver cannot attach above about 60, so keep one simulator
booted across runs (`testing-and-debugging.md`, "Driving and proving the
app").

**Signing facts from the lanes:** the free provisioning team id is the one
shown under Xcode > Settings > Accounts (not the certificate's person id); Exact
needs an installed profile for the bundle id. A free account's profile lasts 7
days. On 2026-11-10 `devicectl device process launch` worked after the
certificate was trusted and a build was installed on the phone; builders do not
install on the owner's phone without agreement.

**Rust is allowed, not the default** (owner, 2026-11-11: "we can write rust"). The sentence that follows was the earlier rule and holds as the default path only.

**Earlier rule: we write no Rust.** Exact's Rust is the engine; the build compiles it, we do
not author it.

## Archive, TestFlight, CI

- `bun exact.mjs ios --device --archive out.ipa --unsigned` writes an ad-hoc
  signed `.ipa` for a re-signing service; with `EXACT_IDENTITY` and
  `EXACT_PROFILE` it signs on the runner. Needs Xcode 27.1 selected, rustup,
  Bun 1.4.2, and an icon of at least 512 px [doc: reference.md:830-857].
- Archives and `--release` bake production trust: with `deploy.store.ios` not
  `"0"` they need `EXACT_UPDATE_RECEIPT` or `EXACT_UPDATE_GENESIS=1`. We set
  `"0"` (no update store) so the release path needs neither.
- A cold build is "tens of minutes"; keep the app's `target/` and
  `~/.cache/exact` between CI runs to get minutes [doc].
- TestFlight upload is Apple's tooling after the `.ipa`; not exercised.

## Over-the-air updates: off for us, not absent in Exact

`deploy.store.<platform>` is `"0"` (no update store, every change is a new
binary) or `"A"` (signed plans, assets and a TypeScript module pair update over
the air, never native code: `scripts/app.schema.json:1208`, `README.md:797-801`
at `de212e671`) [src]. What "isn't implemented" is delivering a TypeScript and a
Rust module together. A production bake with updates needs
`deploy.signing.keys` and `EXACT_UPDATE_RECEIPT` or `EXACT_UPDATE_GENESIS=1`
(`recipes/mobile-build.md:7-10`); the binary must carry the module's
`typescriptRuntime` and exactly its grants, so a new grant is a new binary
(`reference.md:4770-4777`). Android's store reads `"0"` when absent. We set
`"0"` in version 1 and have not run an update [doc, unrun]. Moving the app's
logic into `app.ts` is what would make `"A"` worth deciding; it is an owner act,
not a default. Rust data-module replacement exists in development (`wasmi` on
iOS) but not in production for Android (`host/android/README.md:1731-1736`).

## Telemetry-like behaviour to switch off

After agent runs Exact prints: "a feedback note stays in .exact/ and goes to
Expo (a workers.dev URL) only as this project's answer allows (`bun exact.mjs
feedback status` says it); EXACT_FEEDBACK=0" [run]. Set `EXACT_FEEDBACK=0` in
our scripts and CI, and check `feedback status` before the first agent run on
any machine.

## Versioning and upgrading

- Exact has no releases or CLI package. The pin is the one line in `mobile/exact.pin` (a full 40-character sha; `setup-exact.sh`,
  `check-exact-pin.sh`, `build-lab.sh` and `exact.sh` all read it). Pinned now: `f5334750386192245ee76add13a67ab51fb4e2fd`
  (2026-11-20, moved from `de212e671` by reading only; the first build at it is the lead's, see [EXACT.md](../EXACT.md) "Pin move, 2026-11-20").
- `bun exact.mjs update` rewrites the app's `exact.mjs` and would replace our launcher: do not run it. Exact's own upgrade route
  (`docs/start-here.md#upgrading-an-inherited-app`) is `update`, `contract migrate app.contract` (rewrites retired spellings in place),
  `lock` (the Swift package lock, only when `swiftPackages` is declared) and `contract build app.contract --json --errors-only`. There is no
  changelog: what changed is in `git log`, `QUEUE.md` and the docs' own diffs.
- The owner wants the pin moved routinely ("we're going to be writing the latest versions of Exact for some time", 2026-11-20). Move it often
  and in small steps: every day the pin lags adds commits to read and breakage to meet at once (762 commits at the 2026-11-20 move).

### Moving the pin

First run on a new pin (2026-11-21, de212e671 to f53347503): the checkout moved, patch 0001 applied, 803 shared tests passed, the contract built with no errors, and the native build and `proof-real.sh` passed on the first try. The one failure was `test web`: `cargo metadata --offline` could not find a crate the new lock names. `setup-exact.sh` now fetches the locked crates when that check fails. Exact's own error message for this names `exact.mjs setup` as the remedy; we do not run the full setup (it installs toolchains we pin ourselves), so `setup-exact.sh` is the command to rerun, not that one.

Reading first (no Exact checkout of your own; the lead's `mobile/.exact-src` in the build tree is the one object store, and the
Mac build queue uses it, so **do not check out, reset or build in it while reading**):

1. `git -C <build tree>/mobile/.exact-src fetch -q origin` (changes refs, not files). `NEW=$(git -C ... rev-parse origin/HEAD)`.
   Note `OLD=$(cat mobile/exact.pin)`. Use `git -C ... show $NEW:<path>`, `diff $OLD $NEW -- <path>`, `log $OLD..$NEW -- <path>`; in `zsh`
   write `${NEW}:path`, because `$NEW:h` is a modifier.
2. Read, in this order: `package.json`, `rust-toolchain.toml`, `scripts/app.schema.json` (our `app.json` keys),
   `host/apple/modules/ExactPackage.swift` and `ExactNativeModule.swift` (our package and app module), `host/apple/compose.mjs` and
   `build.mjs` (how a package is built and linked), `contract/syntax/src/migrate.rs` and `git diff $OLD $NEW -- docs/contract-grammar.md` (grammar),
   `git log --format='%h %s' $OLD..$NEW -- contract/types contract/lower contract/syntax` (new refusals: new diagnostic ids are
   `git grep -h -o -E '"(syntax|type|lower|bake)-[a-z0-9-]+"'` at each end, `comm -13`), and the files our patches edit. Grep our contracts for
   every construct a new refusal names (`grep` over `mobile/AutoradioLab/*.contract`).
3. A patch is checked with no checkout: `GIT_INDEX_FILE=<scratch>/idx git -C <.exact-src> read-tree $NEW` then
   `GIT_INDEX_FILE=<scratch>/idx git -C <.exact-src> apply --cached --check <patch>` (reads the tree into a scratch index file; touches no
   working file). Remove the scratch index.
4. In a lane (`lane new exact-pin-<date> --base origin/main`): write `$NEW` into `mobile/exact.pin`, make each adaptation, refresh or drop
   patches, update this page, [EXACT.md](../EXACT.md) and [gaps.md](gaps.md) in the same change, `bun test` in `mobile/AutoradioLab`, push
   `lane/exact-<name>` (the queue refuses any other ref name).

Running it (the lead; one native build at a time, under `~/.cache/autoradio-mac-build.lock`). **The queue does not move the pin**:
`~/.local/state/choir-buildq/run-one.sh` checks out the lane's ref in the build tree and runs `build-lab.sh ios`, whose first step is
`check-exact-pin.sh`; it stops with "the Exact checkout is not at the pin" until `setup-exact.sh` has moved `mobile/.exact-src`. And after the
move every queued ref still on the old `exact.pin` fails that check, so rebase them first.

```sh
Q=$HOME/.local/state/choir-buildq; B=/Users/wiz/Dev/go-choir/.lane/trees/qa-mac
ls $Q/*.req $Q/*.running 2>/dev/null          # nothing queued or running; else wait, or touch $Q/STOP and let it drain
cd /Users/wiz/Dev/go-choir && git fetch -q origin && cd $B && git status --short   # clean
git checkout -q --detach origin/lane/exact-pin-upstream
/usr/bin/lockf -k -t 7200 $HOME/.cache/autoradio-mac-build.lock sh mobile/scripts/setup-exact.sh   # moves .exact-src, reapplies patches, bun install, Hermes
mobile/scripts/check-exact-pin.sh && echo pin-ok
```

Expected from `setup-exact.sh` (**[unrun]**): `patched: 0001-markdown-editor-hides-all-syntax.patch`; `bun install` lines;
possibly `hermes: installing the pinned bundles (online, once per pin)`; last line `exact f533475, bun 1.4.2`. Then
`mobile/scripts/exact.sh contract build app.contract --json --errors-only` (about a minute), `exact.sh test web`, then a queue request
(`printf 'origin/lane/exact-pin-upstream\nbuild\n' > $Q/exact-pin.req`, then `real`) for the iOS build and `proof-real.sh`. Rollback:
put the old sha back in `mobile/exact.pin` (revert the commit), rerun `setup-exact.sh` (it takes the patches off, checks the old pin out and
reapplies them), and expect a rebuild of Exact's Rust host for the old sources.

## Open questions

- Does `exact new` plus `ios --device` work end to end with a free account's
  7-day profile, and can it be automated without the Xcode GUI?
- Can the app's `target/` be shared between the spike and the real app?
- Incremental Swift-only rebuild cost: lanes measured about 60 s warm and 57 s
  for a TypeScript-only change (see above).
- TestFlight: does Exact's `.ipa` pass processing with `deploy.store: "0"`?
