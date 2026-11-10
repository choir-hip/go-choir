# Exact: testing, driving and debugging

Part of [EXACT.md](../EXACT.md). Pinned commit `76cd50f9e`. Marks: **[run]**,
**[src]**, **[doc]**, **[unrun]**.

## The driver: ten operations, the same on every host [doc: README.md "Agent native"]

`tree`, `screenshot`, `tap`, `type`, `state`, `layout`, `logs`, `clock`,
`prefer`, `perf`, run by `scripts/agent.mjs` through the app wrapper:

```sh
bun exact.mjs agent ios tree "tap listen" "clock settle" state      # [run]
bun exact.mjs agent ios "tap listen" "clock +1500 real" tree logs   # [run: hung, see below]
bun exact.mjs agent ios "screenshot .exact/a.png" check             # [doc]
```

`agent ios` targets the booted simulator (`--sim <udid|name>` or `EXACT_SIM`
choose another); `--device [--phone <name|udid>]` targets a phone over a
temporary, unencrypted Mac-side port with a per-launch token
(`docs/reference.md:2469-2473`) [doc]. A drive builds first if the build is
older than its sources; an edit to only `app.contract` or what `app.ts`
imports reaches a running simulator build as a baked plan in about a second
[doc: recipes/ios-driving.md].

What `tree` returned for the spike [run]:

```text
epoch 3 · incarnation 1 · clock 0 ms · 16 nodes
View#1 [root]
  View#2 / View#3 / Text#4 "Exact spike"
    ScrollView#5 / View#6
      Text#7 [state] "state: stopped"   ... Control#10 [listen] (press) ...
```

Names in brackets are the `testId`s. Drive by `testId`, never coordinates
[doc].

### The clock belongs to the driver [doc, partly run]

Between two operations nothing moves. `clock settle` ends transitions and runs
`after(1, ...)`; `clock +60000` jumps virtual time; `clock +N real` lets wall
time run for N ms (needed for anything on the real clock: an
`AVAudioEngine`, `UIView.animate`); `clock data` waits for in-flight answers;
`clock until state <path> == <literal> within <ms>` polls `state` on real
time (`docs/testing.md:98-107`). Under the driver nothing plays on any host
(`state.sounds.output = "agent"`); a package's real hardware is skipped when
`context.agent` is true unless the app is bound as in production (`--bind`)
(`host/apple/modules/ExactPackage.swift:79-82`) [src]. **Our spike package
does not branch on `context.agent`, so under the driver it opens the real
audio hardware**, which may explain the next section.

### Authored tests [doc]

`app.test.contract` runs unchanged on web, macOS and the iOS simulator:

```text
test "search, swipe to delete, delete from the menu"
  type "search" "gro"
  expect tree missing "note-2"
  tap "note-2-delete"
```

`bun exact.mjs test web|ios|macos`; `verify` builds, runs tests, checks where
each ends and relaunches [doc: README.md Quick start].

### Voice flows for agent tests [doc, handoff]

The handoff says the driver can "simulate heard and spoken words on a clock"
through the speech packages (`packages/speech-recognition`,
`packages/speech-synthesis`, `packages/media-capture` agent track "Agent
microphone", `MediaCapture.swift:22-24`) [src]. We built none of this. A
double for AutoradioKit should be a package implementation that branches on
`context.agent` and feeds scripted events with `context.changed`.

## Logs

- `agent ios logs`: runner journal, `console.log` from `app.ts`, package
  `context.log(...)` and Swift `print`. `NSLog` goes to the process stderr; the
  driver echoed it as `app: <timestamp> Exact Spike[pid:tid] SPIKE ...`
  [run].
- Simulator process log: `xcrun simctl spawn booted log show --last 3m
  --predicate 'processImagePath CONTAINS "Exact Spike"'`. It is slow (our
  invocation exceeded 60 s) so prefer the driver's echo or `log stream`.
- Build logs: `.exact/ios-build.log` and `.exact/ios-plan.log` in the app dir
  [doc: ios-driving.md]. `--prebuild` starts a build apart from the shell.
- `state` shows the runner's state tree, `layout` the boxes, `perf` frame
  health (`host/apple/Sources/ExactKit/FrameHealth*.swift`) [src].
- A `host/apple` line "audio session: claimed by a capture (mode X)" is the
  only host-side audio log (`NativeHatches.swift:221`); our package logs the
  category and route itself (see `audio.md`).

## Observed failure messages and what they meant [run]

| Message | Cause | Fix |
|---|---|---|
| `error: Bun 1.3.14 is older than 1.4.2 or unavailable; set BUN=~/.bun-1.4.2/bin/bun` | wrong Bun | install 1.4.2, `export BUN=` |
| `setup` sits on "Installing pinned Hermes bundle for target aarch64-linux-android" | it fetches every platform | run the installer crate for the three Apple targets only |
| `error: No Account for Team "..."` / `No profiles for '<bundle id>' were found` | no Apple account in Xcode | owner signs in under Xcode > Settings > Accounts |
| `error: tool 'xcodebuild' not found` / `'simctl' not found` / `'devicectl' not found` | the Nix dev shell shadows Xcode tools | use `env -i ... DEVELOPER_DIR=...` |
| `host/apple: no Metal toolchain...` | optional | ignore or `xcodebuild -downloadComponent MetalToolchain` |
| `op 2/4 \`tap listen\`: the app hung up (exit code 0)` | the app exited during the tap | open problem, below |
| `no --storage named, so this drive keeps the store \`default\`...` | note | `--reset-storage` to empty it |

## Open problem: the simulator exit after the claim

Sequence [run]: build and install; `agent ios tree "tap listen" "clock settle"
state`. The driver printed the tree, then the claim log lines (`stopped`, then
`claimed listening`, category `PlayAndRecord`), then `the app hung up (exit
code 0)` at op 2 of 4. The tone and RMS lines (`SPIKE running`, `SPIKE
rms=...`) never appeared. Exit code 0 means the process exited cleanly or the
driver's socket closed; it is not a crash report (we found no new
`.ips` in `~/Library/Logs/DiagnosticReports` from the app).

Tried, none conclusive:

1. Second drive `agent ios "tap listen" "clock +1500 real" tree logs`: ran
   over 3 minutes without output and was stopped.
2. `simctl privacy ... grant microphone news.choir.exactspike`: the command
   hung and was stopped.
3. `simctl launch --console-pty`: printed nothing (the app uses `NSLog`).

Hypotheses, none verified: (a) `AVAudioEngine` start with the input node on
the simulator, or `installTap` with a zero-sample-rate input format on the
simulator without a microphone permission, aborted the app (an uncaught
Objective-C exception would normally write an `.ips`); (b) the driver's tap
returned only after the commit, and our `later` handler runs the whole
engine start on the main thread before replying, so the driver's wait timed
out and killed the app; (c) the driver treats a package that opens real
hardware as unsupported under `context.agent`. **Next probes, in order:**
make `start` reply before starting the engine and do the engine work on a
queue; branch on `context.agent`; add `NSLog` lines around each engine step;
check `inFmt.sampleRate == 0`; run on the phone, which has a real microphone.
(`print`/`NSLog` placement matters: only lines after the claim were checked.)

## Common failure classes to expect [doc]

`bake-unknown-module` (a native view tag not in the app's `modules` list),
`lower-screen-by-state`, `analyze-read-after-assign`,
`type-window-opacity`, `tamber-contrast`. `contract build --json` gives
diagnostics with stable ids and ranges; `bun scripts/evidence.mjs --app <name>`
lists recorded drives and what no current run covers (`docs/testing.md:50-66`).

## Open questions

- Why did the app exit at the tap? (Probes above.)
- Can the driver run a package against real audio hardware on the simulator at
  all, or must a package branch on `context.agent`?
- Can the driver feed scripted microphone audio, and if not, what is the
  cheapest double for the lab's replay?
