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
