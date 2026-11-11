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
| `op 2/4 \`tap listen\`: the app hung up (exit code 0)` | the app exited during the tap | a second driver on the same simulator closed the socket; see the limits section |
| `no --storage named, so this drive keeps the store \`default\`...` | note | `--reset-storage` to empty it |

## Driver limits and the "app hung up" message (resolved by the lane builders)

Source: `origin/lane/ar-integrate` `mobile/README.md`, read against the pinned
Exact source. This replaces our earlier hypotheses (a) to (c) about an audio
abort, which are not needed.

1. **"the app hung up (exit code 0)" is not a crash and not an audio bug.** It is
   `Agent.exitAfterLanding` (`host/apple/Sources/ExactKit/Agent.swift:140-146`)
   running because the driver's socket closed [src]. A second driver launching
   the same bundle id on the same simulator does exactly that: every launch uses
   `simctl launch --terminate-running-process` (`scripts/agent.mjs:529, 569`)
   [src]. **Two drivers on one simulator conflict.** Give every driver a
   simulator of its own (`xcrun simctl create`, then `EXACT_SIM`); our spike's
   earlier exit most likely had this cause [inferred: it was run while another
   agent was building; the other driver was not confirmed].
2. **The attach bound is about 20 s.** The app opens its agent socket only after
   the first frame is applied (`ExactIOS/main.swift:117-141`) and, on the
   simulator, after the scene is foreground-active and the window key
   (`ExactKit/IOS/AgentFrontIOS.swift:29-45`, up to 15 s). The driver gives up
   after a fixed 20 s, not configurable (`scripts/agent.mjs:567-579`) [src]. A
   cold first launch on a loaded Mac misses it ("the app never opened its agent
   socket"); the second launch is warm. Retry a refused launch.
3. **A system alert blocks the driver.** A speech-recognition permission alert
   over the app keeps it from being foreground-active; it survives the app being
   killed and blocks every later launch until the simulator is restarted.
   `simctl privacy ... grant all` does not cover speech recognition (`simctl
   privacy` has no speech service) [run, lane report]. Fix in our kit: while the
   runtime is virtual (driven by the agent) the transcriber does not ask.
4. **`context.agent` does not silence an audio claim.** Under the agent our claim
   was real (`cat=PlayAndRecord mode=Default in=[MicrophoneBuiltIn]
   out=[Speaker]`, `AudioSession.swift:238`); only Exact's own sounds are
   replaced (`state.sounds.output = "agent"`). An `AVAudioEngine` we start opens
   the simulator's audio hardware, and `AVSpeechSynthesizer.write` on
   `@MainActor` stalled the main thread until the drive's `clock` op timed out at
   120 s (`agent.mjs:85`, `REPLY_MS`). A package must branch on `context.agent`
   for anything that can block the main thread [run, lane report]. Our lab runs
   a virtual engine under the agent (same events and yield levels, playback as a
   timed wait, microphone frames from a kept turn).
5. **`tree` right after navigation** carries "N requests in flight ... start with
   `clock data`": issue `clock data` before reading data-bound text. The tree
   lists the panels of every tab (hidden ones marked `[hidden]`), so an assertion
   on text must not depend on which tab shows.
6. **Pointer drive:** `tap <id> down`, then bare `tap up` (not `tap <id> up`),
   delivers `pointerdown` and `pointerup` to a `Pressable`; `tap up` with a
   target is refused [run, lane report].
7. **On a phone** a drive can end with "the app left the foreground ... so it is
   suspended" when the phone is locked or showing a permission alert nobody can
   answer. On a phone the module runs the real engine even under the agent (only
   the simulator is virtual) [run, lane report]. Nothing is installed on the
   owner's phone by builders without the owner's agreement.
8. **Timings** [run, lane reports, loaded M1]: a warm rebuild plus drive takes
   about 60 s; a TypeScript-only change about 57 s; a cold Exact build about 15
   minutes; a first `ios` build in a new worktree about 5 minutes (cargo 220 s,
   swift 128 s) when the caches were warm. A single exit 128 seen once was
   transient. **Rule: one shared build tree, one build at a time, one simulator
   at a time.** Each worktree compiling its own copy overloaded the Mac
   (`build-and-ship.md`).
9. **Mac-side wiring** (lane scripts, may have moved): `mobile/scripts/sim-device.sh`
   makes and boots a private simulator; `exact.sh` passes `EXACT_SIM` through its
   `env -i`; `proof-sim.sh` ends with `PROOF OK` or `PROOF FAILED at <step>` and
   retries a refused launch.

## Common failure classes to expect [doc]

`bake-unknown-module` (a native view tag not in the app's `modules` list),
`lower-screen-by-state`, `analyze-read-after-assign`,
`type-window-opacity`, `tamber-contrast`. `contract build --json` gives
diagnostics with stable ids and ranges; `bun scripts/evidence.mjs --app <name>`
lists recorded drives and what no current run covers (`docs/testing.md:50-66`).

## Open questions

- Can the driver run a package against real audio hardware on the simulator at
  all, or must a package branch on `context.agent`?
- Can the driver feed scripted microphone audio, and if not, what is the
  cheapest double for the lab's replay?
