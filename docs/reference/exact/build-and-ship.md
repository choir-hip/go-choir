# Exact: project layout, build, signing, install, pinning

Part of [EXACT.md](../EXACT.md). Pinned commit `76cd50f9e`. Marks: **[run]**
(simulator, 2026-11-10, M1 Mac with another agent building at the same time),
**[src]**, **[doc]**, **[unrun]**. The runnable example is
[`mobile/exact-spike/`](../../../mobile/exact-spike/README.md).

## Prerequisites and the exact commands [run]

Exact needs Rust (rustup; the pinned toolchain installs itself), **Bun 1.4.2
exactly**, Xcode 27.1 or later, and Hermes bundles.

```sh
git clone --depth 1 https://github.com/expo/exact exact-checkout
cd exact-checkout && git fetch --depth 1 origin 76cd50f9e863129b3fc75db1673d6ad4bbf79258 && git checkout FETCH_HEAD

# Bun 1.4.2 beside any other Bun. 1.3.14 is refused:
#   "error: Bun 1.3.14 is older than 1.4.2 or unavailable; set BUN=~/.bun-1.4.2/bin/bun"
curl -fsSL https://bun.sh/install | BUN_INSTALL=$HOME/.bun-1.4.2 bash -s bun-v1.4.2
export BUN=$HOME/.bun-1.4.2/bin/bun

# DO NOT run `bun scripts/exact.mjs setup` for iOS-only work: it also fetches
# Hermes for Android, Linux, Windows, tvOS and the game SDK's crates, each
# ~190 MB, and stalled for us. Install only the Apple targets:
for t in aarch64-apple-darwin aarch64-apple-ios aarch64-apple-ios-sim; do
  cargo run --manifest-path vendor/ibex/crates/hermes-lean-sys-installer/Cargo.toml -- --target $t
done
```

Result: `~/.cargo/hermes-lean-sys/hermes-vanilla-d412d3bd8512-v5/<hash>/` for
each target (525 MB for five bundles after the aborted `setup`) [run].
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
| `scripts/proof-sim.sh` | drives the simulator and prints `PROOF OK` or `PROOF FAILED at <step>`; `NOBUILD=1` skips the build |

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
costs the next first build its time (Exact `AGENTS.md` [doc]).

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

## Over-the-air updates: off

`deploy.store.<platform>` is `"0"` (no update store, every change is a new
binary) or `"A"` (signed plans and assets update over the air). "Signed
TypeScript and Rust module delivery isn't implemented"
(`scripts/app.schema.json:1003`) [src]. Rust data-module replacement exists in
development (`wasmi` on iOS) but production capability "does not grant store
permission" (`docs/reference.md:2917-2921`). Set `"0"` for iOS in version 1.

## Telemetry-like behaviour to switch off

After agent runs Exact prints: "a feedback note stays in .exact/ and goes to
Expo (a workers.dev URL) only as this project's answer allows (`bun exact.mjs
feedback status` says it); EXACT_FEEDBACK=0" [run]. Set `EXACT_FEEDBACK=0` in
our scripts and CI, and check `feedback status` before the first agent run on
any machine.

## Versioning and upgrading

- Exact has no releases or CLI package. Pin the commit in
  `mobile/EXACT_PIN` (to be created) and in this page.
- `bun exact.mjs update` rewrites the app's `exact.mjs` and keeps its custom
  verbs [doc: schema "commands"]. The README logs breaking changes as dated
  "Upgrade note" lines (for example the 2026-10-16 Snapback mutation note).
- Every upgrade: re-read `README.md` upgrade notes, rebuild the spike app, run
  the driver script, compare the plist keys, then move the pin.
- Expect daily churn: the commit we read (2026-11-10) was newer than the one in
  the handoff (2026-11-09 `94b8671`).

## Open questions

- Does `exact new` plus `ios --device` work end to end with a free account's
  7-day profile, and can it be automated without the Xcode GUI?
- Can the app's `target/` be shared between the spike and the real app?
- Incremental Swift-only rebuild cost: lanes measured about 60 s warm and 57 s
  for a TypeScript-only change (see above).
- TestFlight: does Exact's `.ipa` pass processing with `deploy.store: "0"`?
