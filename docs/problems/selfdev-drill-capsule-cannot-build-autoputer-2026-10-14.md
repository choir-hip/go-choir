# Self-Development Drill After the Go Module Proxy: Modules Arrive, the Capsule Still Cannot Build bin/autoputer in Time

Date: 2026-10-14, 18:50 UTC. Drill:
`node scripts/reeng_tracer.mjs --label selfdev-goproxy-1802 --scenario engineering --expect-sha 865dc152116b64f210de3a9cf9a823fa257f5aaa --timeout-min 30`
(exit 6, the scenario's claim failed). Receipt and evidence in
`~/.config/choir-qa/traces/selfdev-goproxy-1802/` (receipt.json, the
engineering run's 236 events in two pages, the tracer log, the goproxy
journal for the window). Test computer
`computer-e6cb974fe6e3a5261222f126b82dc026` (minted by the tracer, stopped
by it at 18:33:13; started once at 18:40 to read the ledger through
product routes and stopped again at 18:42).

Follows [c1e-b1-engineering-assignments-cancelled-on-arrival-2026-10-14.md](c1e-b1-engineering-assignments-cancelled-on-arrival-2026-10-14.md)
(cause 3, Go modules) and the owner's option B,
[capsule-go-module-proxy-design-2026-10-14.md](../reports/capsule-go-module-proxy-design-2026-10-14.md),
deployed at 3439462a.

## Outcome

Freeze not reached. Last phase: `management_cast_engineering`. The
operation never opened (`decision_boundary_not_reached`, state null). The
Go module proxy did its job; the build after it did not finish.

## Timeline (UTC; ledger and journal, not inference)

| time | event |
|---|---|
| 18:02:54 | identity fence ok: staging 865dc152, no CI busy |
| 18:03:11 | mint (16.3 s), guest build 865dc152; mode propose_only armed |
| 18:03:26 | Texture Casts management (round 1) |
| 18:03:38 | management Casts engineering: `assignment-7867b850ec2ba578e7aa2b46`, parent Texture's Cast |
| 18:04:45 | engineering writes `frontend/public/choir-drill/choir-drill-773a285a.txt`; the deterministic check exits 0 |
| 18:05:19 / 18:05:35 | `npm ci`, `npm run build` exit 0; the file is in `frontend/dist` |
| 18:05:51-18:06:40 | **goproxy: 513 requests, 513 hits, 0 misses, 0 failures** (171 modules: .info, .mod, .zip each), 331.8 MB served, slowest 1,178 ms, all attributed to `capsule-ce1e802b-...` |
| 18:06:22 | synchronous `go build` cell: the exec ends at its 30 s default (`yaegikernel` broker `DefaultTimeout`), exit -1, after downloading every module |
| 18:07:42 | backgrounded build: `cgo: C compiler "gcc" not found: exec: "gcc": executable file not found in $PATH` |
| 18:08:00 | `CGO_ENABLED=0` attempt: `dolthub/gozstd` does not compile without cgo (`undefined: DDict`, `Reader`, `NewReader`) |
| 18:08:48 | the desk finds `/nix/store/...-gcc-wrapper-15.2.0` by listing the store and sets `CC`; two of its builds run at once (a kill hit the wrong pid) |
| 18:12:06 | a polling cell hits the 60 s session eval timeout |
| 18:13:13 | `# github.com/dolthub/gozstd` / `gcc: fatal error: Killed signal terminated program cc1` |
| 18:15:08 | next attempt: `unicode/uregex.h` not found (go-icu-regex, ICU headers) |
| 18:16:03 | the desk finds `icu4c-78.3-dev` in the store, sets the ICU include and lib paths, kills stray builds, restarts with `go build -p 1` |
| 18:17:33 | **platform deploy efbd69e2 lands on the host mid-run** (vmmanager tap rules). The guest stayed on 865dc152 (tracer: contamination []); the goproxy unit restarted after the module downloads were done. |
| 18:19 | the Go linker starts (`link ... -extld .../g++`) |
| 18:33:05 | the linker is still running: 13 min 43 s elapsed, 9 min 21 s CPU (about 68% of one CPU), RSS 400-465 MB; no `g++` child yet |
| 18:33:12 | drill deadline (30 min): acceptance fails at `frozen`; the tracer stops the computer |
| 18:40:11 | (my ledger read) the start is a crash boot; the run closes `loop.cancelled`, fate `interrupted_by_restart`, as the restart rule requires |

59 cells in the run; about 40 of them start, poll or repair the build.

## Findings

1. **The module proxy works on the product path.** First capsule traffic
   ever (the journal had only warm lines before). Every module
   `go build ./cmd/autoputer` needs came from the warm cache in 49 s; no
   egress to proxy.golang.org or storage.googleapis.com was needed.
2. **The capsule has no C toolchain on its PATH.** `cmd/autoputer` needs
   cgo (`dolthub/gozstd`, `dolthub/go-icu-regex` with ICU). Workloads run
   with `PATH=/run/current-system/sw/bin:/bin:/usr/bin`
   (`cmd/capsule-broker/main.go:483`, `internal/capsule/executor.go:500`).
   The guest's `environment.systemPackages` (`nix/autoputer-vm.nix:867`)
   has `go`, `icu` and `pkg-config` but no `gcc`/`binutils`; the
   autoputer service's own PATH has `gcc` (line 775), so the compiler is
   in the store, just not reachable. No `CGO_CFLAGS`/`PKG_CONFIG_PATH`
   for ICU is set either (the repo's dev shell sets them on the host). The
   desk rebuilt that environment by hand from store paths, about 10
   minutes and 20 cells.
3. **The engineering capsule's budget is 1 CPU and 1 GiB, no swap**
   (`internal/agentcore/engineering_assignment_runtime.go:27-29`, written
   as cgroup v2 `cpu.max 100000 100000`, `memory.max`,
   `memory.swap.max 0` by `internal/capsule/namespace.go:26`).
   Hypotheses, not yet confirmed from the cgroup's own counters (the
   capsule is gone): the `cc1` SIGKILL at 18:13 is the cgroup OOM killer
   (gozstd compiles zstd's C sources; two builds were running at once);
   the 14+ minute link is one capped CPU plus page-cache reclaim under the
   1 GiB limit (a cold `GOCACHE` under `/tmp`, which is disk-backed,
   `executor.go:1978`). An uncapped host link time was not measured for
   comparison.
4. **Cells are short; builds are long.** `choir.Exec` defaults to 30 s and
   a session eval to 60 s, so the desk must background the build and poll
   it, cell by cell, with the model in the loop on every poll.
5. The desk did what the overlay asks (build the real binary, no stub), and
   it did not Freeze on a partial tree. The drill's 30 min bound ended it
   while it was still linking.

## Substrate vs symptom

Substrate: the capsule's build environment (toolchain, budget, execution
time) was sized for small scripted work, not for building the platform
binary. The module proxy fixed the network leg of the same problem. This
is the second leg; the drill will not reach Freeze by prompt changes.

## Options (not chosen here; red/orange, need review)

- **A. Do not rebuild Go for a change that does not touch Go.** For a
  frontend-only change the right `bin/autoputer` is the running one
  (recorded in the c1e-b1 doc's Q2). Stage the guest's own deployed
  binary (content-addressed, known digest) into the release tree, and
  build only when Go sources changed. Removes the build from this drill
  entirely; does not help Go changes.
- **B. Give engineering capsules the dev shell.** Put `gcc`/`binutils` on
  the capsule PATH and set the ICU cgo environment (`CGO_CFLAGS`,
  `CGO_LDFLAGS` or `PKG_CONFIG_PATH`) in the L2 workload env, mirroring
  the flake's dev shell. Small, and removes finding 2.
- **C. A build budget.** A larger engineering capsule (for example 2-4
  CPUs, 3-4 GiB) while a build runs, and a warm read-only `GOCACHE` for
  the deployed commit (the same pattern as the module proxy), so a
  one-file Go change compiles incrementally.
- **D. A long-running exec primitive** (start, then wait with a bound, or
  an exec timeout the cell can raise for a build) so the model is not
  polling every 60 s.

Recommendation: A first (it is what the drill's change needs, and it is
the smallest), with B so Go changes can build at all; C and D when the
first Go-changing drill runs.

## Also seen

- Memory pressure on Node B: PSI `some avg10` reached 5.81 and then 1.46
  around 18:16-18:17, coinciding with the efbd69e2 deploy (switch, a
  `candidate-fleet-*` VM started at 18:16:44); it was 0.00 again by
  18:17. The drill kept running; the spike is not attributed to it.
- The goproxy's size accounting reads 24 bytes; separate doc,
  [goproxy-cache-root-is-a-symlink-2026-10-14.md](goproxy-cache-root-is-a-symlink-2026-10-14.md).

## Decision (lead, 2026-10-14, No Blocking Asks): A, then B

C and D are follow-ups unless trivially contained. Mutation class: red
(capsule workload environment, self-development Freeze and verification).
Protected surfaces: capsule exec environment (three hops: executor ->
broker -> session worker / direct exec -> cell `choir.Exec`), the Freeze
release assembly (`StageGrantedRelease`), the capsule effect bundle schema
and the in-cell inspection mirror, the management Verify pass path.
Admissible evidence: unit tests on the pure classification, staging and
env builders (any OS); Linux-gated executor tests where they exist; the
deployed drill after a deploy (not run here). Rollback: revert the commits;
bundles frozen with the new `release_binary` field are omitempty-additive,
and a revert only loses the reuse path (Freeze goes back to refusing a tree
without `bin/autoputer`).

### What the release does with bin/autoputer (verified in code)

The self-development materializer mints a plain file release from the
bundle's `runtime_files` (`self_development_materializer.go`). A plain
release carries no `layering-entrypoint`, and the guest's exec wrapper
(`nix/autoputer-vm.nix`, `autoputerRuntimeExec`) runs the base image's
`${autoputer}/bin/autoputer` for it; the release's `frontend/` is what the
computer surface serves. So the `bin/autoputer` in a self-development
release is a frozen, hashed artifact, not the process that runs after the
apply. The frontend is not embedded in the autoputer binary (only
`cmd/desktop` embeds a frontend). The binary's inputs are exactly what the
flake's `goServicesSrc` filter admits: `go.mod`, `go.sum`, and non-test
files under `cmd/` and `internal/` (every `//go:embed` in the autoputer
closure is under `internal/`).

### A: reuse the running binary when the change touches no Go input

At Freeze, if the capsule's release tree has no `bin/autoputer`, the
executor may stage the computer's running binary (`/proc/self/exe`) as
`bin/autoputer`, only when all of these hold:

1. No changed `workspace/platform` path is a binary input: top-level
   `go.mod`, `go.sum`, `go.work`, `go.work.sum`, `flake.nix`,
   `flake.lock`; anything under `cmd/`, `internal/`, `vendor/`, `nix/`;
   any `.go` file anywhere else (a new Go package needs one, so cgo
   sources outside these trees cannot enter the binary without a `.go`
   change). `*_test.go` files are not inputs (the flake excludes them;
   `go build` ignores them).
2. The capsule's source is a git pin (`capsule-source-git:<commit>`) and
   that commit equals the running binary's linked commit
   (`buildinfo.Commit`, full 40 hex). A dev build (`local`) or a source
   pinned elsewhere refuses reuse, and Freeze says why.

The bundle records `release_binary` = {origin `reused_running` or
`capsule_built`, sha256, running commit, source base commit}. The
verification pass (management's Verify) re-hashes the running executable
and refuses a pass when a reused binary no longer matches it.

Ways A could fail, and what guards each:

- A1. The classification misses a Go input, so the reused binary does not
  reflect the source. Guard: mirror the flake filter and add conservative
  extras (rule 1); a test pins each input class.
- A2. The source pin and the running binary are different commits (a
  persistent computer's `Source/platform` was seeded at creation and the
  guest image moved since; or the source is a prior candidate artifact).
  Guard: rule 2, refuse with both commits named.
- A3. The running executable is unreadable or changes during the copy.
  Guard: the staged file's hash comes from the same read that writes it and
  must equal the cached identity hash; refuse otherwise.
- A4. The desk staged its own binary for a non-Go change. Guard: the
  desk's binary is kept and recorded `capsule_built`; no silent swap.
- A5. A platform deploy restarts the computer between Freeze and Verify.
  Guard: the Verify pass check fails with a named reason (the bundle's
  binary is not this computer's running build).
- A6. The new bundle field breaks the in-cell inspection's strict decode.
  Guard: the mirror gains the field; the drift test sets every field.
- A7. Old bundles lack the field. Guard: omitempty; absent means legacy and
  no check.
- A8. The release-size accounting skips the reused file. Guard: it counts
  toward the same total.
- A9. (Residual, pre-existing) a computer running a layered release that
  applies a plain self-development release execs the base binary after the
  swap, so "running binary" and "binary that runs after apply" differ
  there. Not introduced by A; recorded.

### B: the dev shell's toolchain in engineering capsules

The autoputer service already has the toolchain in its PATH (`gcc`,
`binutils`, `pkg-config`) and the ICU cgo flags (`CGO_CFLAGS`,
`CGO_CXXFLAGS`, `CGO_LDFLAGS`, `PKG_CONFIG_PATH`). The guest gains one
variable, `CHOIR_CAPSULE_BUILD_PATH` (the bin dirs of `gcc`, `binutils`,
`pkg-config` from the guest's own store, read-only through the capsule's
lower root). The executor hands the build path and the four cgo variables
to the broker; the broker adds them, with `CGO_ENABLED=1`, to the
environment of direct exec, of the session worker, and of every
`choir.Exec` a cell runs. The build path is appended after the existing
PATH, so `sh` and every tool that resolved before still resolves to the
same file.

Ways B could fail:

- B1. One hop drops the variables (each hop is an allowlist). Guard: one
  shared builder in `yaegikernel` used at every hop; unit tests per hop.
- B2. A new key carries a credential. Guard: fixed key names, values are
  store paths; the credential filters are unchanged.
- B3. PATH order changes `/bin/sh` (busybox ash vs bash, see
  capsule-workload-cannot-run-a-shell-or-install-packages-2026-10-10.md).
  Guard: append, never prepend; a test pins the order.
- B4. A desk session outside a capsule gains the toolchain. Guard: the env
  is added only when the process env carries the build path.
- B5. Memory: zstd's C compile plus the Go compiler, or two builds at once,
  exceed the capsule's 1 GiB; the link's RSS was 400-465 MB. See the
  measurement below for the sizing.

### Measurement (2026-10-14, local; not the guest)

`go build ./cmd/autoputer`, peak RSS of the largest child process:

| where | step | wall | peak RSS |
|---|---|---|---|
| macOS arm64, `/usr/bin/time -l`, cold `GOCACHE`, `-p 1` | compile everything | 1,021 s (336 s user) | 690 MiB (one compile) |
| linux/arm64 container (golang:1.26, libicu-dev, external link via `g++`), GNU time | relink only | 87 s | 1.80 GiB |
| same | relink, `GOGC=50` | 100 s | 1.75 GiB |
| same | relink, `-ldflags=-s -w` | 25 s | 1.50 GiB |
| same | relink, `-p 1` | 57 s | 1.81 GiB |

The binary is 175 MB (130 MB stripped); a cold `GOCACHE` is 1.4-1.6 GB.
`GOGC` and `-p 1` do not lower the link peak; dropping DWARF does. At the
capsule's 1 GiB the drill's link held 400-465 MB RSS for 14+ minutes, which
fits file pages being reclaimed and re-faulted under the cgroup limit.

Sizing: the engineering capsule gets 2 GiB (holds the stripped link with
headroom; the DWARF link is near the limit) and keeps 1 CPU. The admission
budget is 3/4 of the guest's MemTotal (`capsule_executor_linux.go`), about
2.9 GiB on the default 4 GiB guest, with one engineering capsule live at a
time, so 2 GiB is admitted. The overlay tells the desk to build with
`-ldflags='-s -w'` and to run one build at a time. Memory implication: the
cap is a ceiling, not a reservation; most of the capsule's extra use is
page cache the guest kernel can reclaim, but a build's anonymous memory can
now reach 2 GiB of the guest's 4 GiB.

## What landed (branch, not deployed)

- A: `internal/capsule/release_binary.go` (classification, reuse plan,
  staging, provenance checks), wired into `StageGrantedRelease`; bundle
  field `release_binary` (`transaction.CapsuleEffectBundle`, Validate binds
  it); the in-cell inspection mirror reads and reports it; management's
  Verify pass re-checks a reused binary against the running build
  (`releaseBinaryStillRunning`); Freeze's result names the origin; the
  engineering overlay stops demanding a build for a change with no Go
  input.
- B: guest variable `CHOIR_CAPSULE_BUILD_PATH` (gcc, binutils, pkg-config);
  `yaegikernel.BuildToolchainCarry` / `WorkloadToolchainEnv` used by the
  executor's broker launch (`capsule.BrokerProcessEnv`), the broker's
  direct exec (`directExecEnvFrom`), the session worker spawn
  (`sessionWorkerEnv`) and each cell exec (`cellExecEnv`).
- B5: engineering capsule memory 1 GiB to 2 GiB.

Not proven here: the product path. The deployed drill after a deploy is the
proof (`node scripts/reeng_tracer.mjs --scenario engineering --expect-sha
<sha> --timeout-min 30`); a Linux capsule smoke test that runs `gcc` inside
a real capsule needs the root/cgroup-v2 harness
(`CHOIR_CAPSULE_INTEGRATION=1`) and the guest's store, not available here.

## Follow-ups (recorded, not done)

- C. A build budget for Go changes: more than one CPU while a build runs,
  and a warm read-only `GOCACHE` for the deployed commit (the module-proxy
  pattern), so a one-file Go change compiles incrementally. A cold compile
  at one CPU is tens of minutes (1,021 s at `-p 1` on an M-series laptop).
- D. A long-running exec primitive (start, then wait with a bound) so a
  build is not polled cell by cell under the 30 s exec and 60 s cell
  defaults.
- Reuse when the source pin and the running binary differ in commit but
  not in binary inputs (`git diff --name-only <running> <pin>` classified
  by `BinaryInputPath`). Today reuse requires equal commits, so a
  persistent computer whose `Source/platform` was seeded at an older commit
  than its guest image must build.

## Residuals

- A9 (pre-existing): a computer running a layered release that applies a
  plain self-development release execs the base binary after the swap.
- The reused binary is not secret-scanned at Freeze (the scan guards
  capsule output; this file is the deployed build, hash-checked against
  the running process).
- Heresy delta: discovered A9 (above); introduced none known; repaired the
  capsule build environment's missing toolchain (finding 2) and, for
  changes with no Go input, the build requirement itself (finding 1's
  consequence).
