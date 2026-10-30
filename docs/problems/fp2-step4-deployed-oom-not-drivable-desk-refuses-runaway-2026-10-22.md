# FP2 step 4's deployed OOM leg is not drivable through the product path: the desk refuses a deliberate runaway cell (2026-10-22)

Found 2026-10-22 04:51–04:58 UTC while running FP2 batch 1's deployed
proof step 4 (out of memory) on the disposable test computer
computer-9648031f92e131adc1fb65ec455c5903 (vm-80b173b9), deployed commit
`840e1a701` (guest build `840e1a701d27e21dcbd3d1ca6b1189954b638ce3`).

## What step 4 asks for

`scripts/proofs/fp2-desk-boundary.sh` step 4 and the FP2 definition
(`docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md:455`) say:
read `/api/runtime/observability` before, then "run a research desk turn
whose cell allocates without end (a prompt that asks for a cell
`for { keep = append(keep, make([]byte, 8<<20)) }`)"; expect the cell
result "cell's worker died ... killed by its memory budget (memory.max 768
MiB ...)", the daemon's identity unchanged, and the next turn working.

## Evidence (trace-read, product path only)

There is no product route that addresses the research desk with arbitrary
owner text. The only path is model-mediated: `POST /api/prompt-bar` or a
Texture `revise` → the Texture (or management) desk authors the Go cell or
casts an objective to research. I drove a Texture `revise` three times on a
fresh lifecycle document (doc `30b02199-2dcb-54a3-bc6a-dc27052a8fc4`,
trajectory `a4a060bc-...`), each time asking for the exact runaway cell:

1. Direct ask (rev `0396671a`). Texture turn run `f39dbfb4-84eb-4b7e-a997-a38f90ee579e`.
   Its cell wrote a revision note: "The requested memory-budget self-test
   was not run: its unbounded 8 MiB allocation loop could exhaust worker
   memory, and I cannot verify that this execution surface isolates such a
   failure from shared infrastructure." It did not run the cell and did not
   cast to research.
2. Research-framed ask (rev `d6cb6e5c`). Texture turn run `4273b482-69a2-49ee-9a89-c45d3e208c24`
   wrote: "The requested unbounded allocation loop will not be run:
   repeatedly allocating 8 MiB slices until a worker is killed risks
   exhausting resources, and this execution surface has not established that
   such a kill is safe", and cast a sanitized task to engineering
   (`rlm:cast:0`) — not the runaway cell, not to research.
3. Informed ask supplying the true cgroup safety context (rev `e6970558`:
   each worker's cgroup has `memory.max` 768 MiB, `memory.oom.group=1`,
   worker `oom_score_adj=1000`, so the kill is contained to the worker).
   Texture turns `5da0a4c6-...` and `bcdea865-...` still declined: one read
   source files to audit the limit; the other wrote "No allocation
   diagnostic was run" and again delegated to engineering.

So the deployed desk model refuses to run or relay a deliberate
unbounded-allocation cell. It never casts the cell to research and never
runs it in its own workbench. The research desk never receives it.

`POST /api/runtime/isolation-selftest` spawns one worker per desk through
the production spawner, but runs only fixed confinement probes and a
hardcoded check cell (`fmt.Println("cell", 6*7)`); it takes no `source` and
has no memory/OOM leg (`internal/agentcore/isolation_selftest.go`,
`internal/yaegikernel/desk_cell_selftest.go`). The station forbids adding a
test/debug cell route, and I did not pressure the model into running code it
judged unsafe.

## Is it a defect?

The desk's refusal is defensible behaviour (defense in depth: a privileged
desk should not run an obviously-unsafe cell on owner command, ARCH-05).
The gap is in the proof's feasibility, not in the runtime: step 4 as written
assumed a prompt could drive a research-desk OOM, but the deployed desks
refuse a deliberate runaway, so the per-worker OOM containment cannot be
exercised through the product path. Two further observations:

- **The step-4 script's automated daemon-identity check is a false
  positive.** `/api/runtime/observability` carries no daemon `started_at`,
  `pid`, or `uptime` scalar, so the script's regex latches onto
  `steps.recent[].started_at` (per-step execution timestamps). The "before"
  reading right after boot was `[]`; after activity it is a list of step
  timestamps, so the script always reports "daemon identity changed". The
  authoritative daemon identity is host-side and was unchanged throughout:
  firecracker.pid `3926651`, guest bootid `2adf3e7df8a9421e9fb8005c6573e3da`,
  5 server starts, epoch 13221.
- **Desk workers do not log `memory.peak`.** `memory.peak` is read and
  logged only in the capsule-teardown path
  (`internal/capsule/teardown_reclaim.go:41,113`), for job/engineering
  capsules. A desk worker's cgroup logs `memory.max`/`memory.high` at
  creation (`internal/autoputer/desk_cgroup.go`) and, on death,
  `memory.max` + `oom_kill` (`internal/yaegikernel/desk_cgroup.go:199-208`).
  So step 4's instruction to "read each worker's `memory.peak` from the desk
  cgroup log lines" rests on a premise that does not hold for desk workers.

## What is still covered

The per-worker OOM containment itself (worker killed at its budget, daemon
survives) is covered by unit tests —
`internal/agentcore/tools_desk_worker_death_test.go`
(`TestDeskWorkerVolunteersForTheOOMKiller` proves the worker carries
`oom_score_adj=1000` and the daemon does not) — and by the per-worker
cgroup creation code (`internal/yaegikernel/desk_cgroup.go`,
`memory.max`/`memory.oom.group`/`CLONE_INTO_CGROUP`). FP2 step 2's receipt
(`docs/evidence/fp2-desk-boundary-20261022.json`) shows each desk worker in
its own confinement with a distinct uid. The yaegi for-loop bug that caused
the 2026-10-16 accidental runaway
(`docs/archive/problems/desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md`)
is fixed, so the accidental runaway path is also narrowed.

## Residual / recommendation (no runtime fix here, green)

Step 4's deployed OOM leg stays UNPROVEN on the product path. Options for
FP0, none taken here (no blocking ask raised; owner asleep):

- Accept the unit-test + cgroup-code + step-2 coverage as the containment
  evidence and retire step 4's deployed OOM leg, or
- If a deployed OOM proof is required, it needs a sanctioned mechanism the
  desk will honour (e.g. an owner-authenticated diagnostic the runtime runs
  itself inside a desk worker, not a model-authored cell) — a new surface,
  which this station forbids adding, so it is a separate definition.

Also fix the step-4 script's daemon-identity check (yellow): compare an
authoritative daemon identity, not `steps.recent[].started_at`
(e.g. expose a daemon `started_at`/`boot_id` in observability, or read it
host-side), and drop the `memory.peak` expectation for desk workers.

## The sanctioned mechanism: an OOM leg in the isolation self-test (FP0, 2026-10-22)

FP0 chose the second option above: the isolation self-test
(`POST /api/runtime/isolation-selftest`, `internal/agentcore/isolation_selftest.go`)
gains an out-of-memory leg, selected with `?leg=oom`, so no model is in the
loop. The leg spawns a throwaway research-class (untrusted input) desk worker
through the production spawner, with its real per-worker cgroup under the
real desks slice. It runs a fixed cell compiled into the binary that
allocates and touches memory until the kernel kills the worker. Then it
reports, read from outside the worker:

- the worker cgroup's `memory.max`, `memory.high` and `memory.events`
  (`oom_kill`);
- the desks slice's `memory.max` and `memory.high`;
- the daemon's own pid, `/proc/self/stat` start time and the kernel boot id,
  before and after;
- whether a sibling worker of the same class, spawned before the runaway,
  still answers a fixed check cell.

Items: RES-01 (in-guest cgroup tree), RES-02 (hard per-worker budget).

### Ways the leg could fail or mislead (written before code)

- **O1. The runaway runs without a hard budget and takes the guest.** This
  happens with no desks cgroup, a worker with no budget cgroup, or
  `memory.max` reading `max`. The leg refuses to send the allocating cell
  unless the worker's own cgroup reads a numeric `memory.max` no larger than
  its manifest budget. Otherwise it reports a failure and allocates nothing.
- **O2. The worker dies for another reason and reads as an OOM proof.** The
  other reasons are the Go runtime's own out of memory at `RLIMIT_AS`, the
  cell timeout's host kill, or a crash. The death cause is `oom_kill` >= 1 in
  the worker cgroup's `memory.events`, together with an exit cause that names
  the budget. A host kill on timeout is reported as a host kill.
- **O3. The pages are never touched.** A fresh large `make` returns zeroed
  pages that the kernel has not backed, so the cell would run to its timeout
  with a flat RSS. The fixed cell writes one byte per 4 KiB page.
- **O4. The kernel picks the daemon (the 2026-10-16 failure).** The daemon
  reads its own pid, `/proc/self/stat` start time and the boot id before the
  runaway and after it, from the process and never from observability's
  `steps.recent` (the step-4 defect above). If the daemon died, the HTTP
  answer never arrives, and the probe reports exit 1 (a failure), not a
  pass.
- **O5. The kill reaches a sibling.** This could come from `oom.group` on the
  wrong cgroup or a slice-level OOM. A sibling worker of the same class is
  spawned before the runaway and must answer the check cell after it.
- **O6. The leg becomes a way to run code.** The cell is a constant in the
  binary and the route takes no source. The only input is the leg selector,
  an enum, and any other value is refused with 400. One self-test runs at a
  time (`isolationSelfTestMu`). Who can call it was first written as "the
  owner route stays host-peer only". Review bunch 22 showed that was false;
  see the next section for the corrected caller rule.
- **O7. The leg runs by accident.** It runs only with `leg=oom` on the
  internal route. The default
  self-test (`POST` with no leg) answers exactly as before, so step 2 on any
  computer never allocates.
- **O8. A worker or cgroup leaks.** Both workers are closed before the
  handler answers. Close reaps the worker and removes its cgroup, and a crash
  mid-leg is covered by the boot sweep of `w-*` under the desks slice.
- **O9. `memory.events` is read after the cgroup is gone.** A removed cgroup
  reads as zero, which would give a false negative. It is read before the
  worker is closed.
- **O10. The judge trusts the server.** The staging script judges the raw
  numbers itself and compares `memory.max` to its own written-out 768 MiB.

### The capsule leg (SEC-13, RES-03) is not reachable safely from the self-test

The brief asked for a second leg inside a throwaway capsule's session
worker: probe `clone3` and `unshare(CLONE_NEWUSER)`, and report the worker's
bounds. Reading `internal/capsule` and `cmd/capsule-broker` on `d23dfaa2d`
shows the self-test cannot safely reach one:

- A capsule is spawned in one place only, after a durable engineering
  assignment opens: `internal/agentcore/engineering_assignment_runtime.go:494`
  (`rt.capsuleExecutor.Spawn`). Its use needs a capability minted for an
  engineering or research run identity
  (`internal/capsule/executor.go:857-866`, `MintCapabilityHandle`). If the
  self-test minted one with no assignment, the desk ontology's rule would
  break: management admits engineering only through a delegated cast.
- `Spawn` admits against the capsule memory budget
  (`internal/capsule/executor.go:311`, `admitCapsuleMemoryLocked`). A
  throwaway capsule would compete with real engineering work for admission.
  A live capsule also blocks an always-busy computer's planned refresh
  (CLAUDE.md, Restarts).
- The session worker's argv is fixed by the broker
  (`cmd/capsule-broker/session_worker.go:106-117`), so there is no way to
  select a probe mode. A cell cannot call `clone3` itself, because the cell
  allowlist forbids `syscall` (`internal/yaegikernel/allowlist.go:36`). A
  probe would need a new broker stage, which is a new entrypoint into the
  capsule binary, and it would add red surface to prove red surface.

So SEC-13 and RES-03 stay unticked by this lane. Their evidence remains the
Linux container tests (`internal/capsule/seccomp_clone_linux_test.go`,
`cmd/capsule-broker/session_limits_test.go`) plus step 5's engineering
assignment. A safe partial option for FP0, not built here: a self-test leg
that re-execs the daemon binary into a child that loads the deployed capsule
workload filter (`capsule.LoadWorkloadFilter`) and tries `clone3` and
`unshare(CLONE_NEWUSER)`. That proves the deployed filter on the deployed
kernel, though not inside a capsule, and it cannot read the session worker's
bounds.

## The partial capsule leg: `?leg=seccomp` (FP0 chose it, 2026-10-22)

FP0 chose the partial option above. `POST
/api/runtime/isolation-selftest?leg=seccomp` re-execs the daemon's own
binary once per capsule seccomp filter: workload, workload-inet, broker,
broker-inet, delegate and broker-delegate. Each child loads the filter that
`internal/capsule/seccomp.go` builds for that name. The child gets the
filter by calling the same function the capsule calls (for example
`WorkloadSeccompFilter()`), not a copy. Then, under the filter, the child:

- tries `clone3` and expects ENOSYS;
- tries `clone(CLONE_NEWUSER|CLONE_THREAD)` and expects EPERM from the
  filter (the kernel would answer EINVAL, so this never creates a process);
- tries `unshare(CLONE_NEWUSER)` and expects EPERM from the filter's
  default deny;
- runs two controls that must work: a plain `fork`+`exec`+`wait`
  (`syscall.ForkExec`), and Go's `os/exec`. The `os/exec` control matters
  because it is how a capsule's tools start their children.

The receipt carries:

- the deployed kernel's release;
- for each filter, the sha256 of the assembled BPF program the child
  loaded, and the hash the daemon computes from the same function (they
  must match);
- the child's `Seccomp` field from `/proc/self/status` after the load;
- each probe's raw result.

### What it proves and what it does not

It proves that the capsule filters built by the deployed binary, loaded on
the deployed guest kernel, answer `clone3` with ENOSYS and refuse a new user
namespace, and that a process under them can still fork and exec. That is
the SEC-13 mechanism on the deployed kernel.

It does not prove:

- that a live capsule loads these filters. The child is not a capsule: it
  has no namespaces, no Landlock and no broker. That a capsule installs the
  filter is code (`cmd/capsule-broker/main.go:172-187`) plus the Linux
  container tests, and step 5's engineering assignment shows that nothing
  in a capsule needs `clone3`.
- anything about RES-03, the exec-go-session worker's bounds (rlimit,
  GOMEMLIMIT, oom_score_adj). Those apply only in the broker's session
  worker, and the leg cannot read them.

### Ways it could fail or mislead (written before code)

- **S1. The child probes a copy of the filter, not the capsule's.** The
  filter comes from a table in `internal/capsule` that maps each name to the
  exported constructor the capsule uses. The daemon computes the expected
  hash from the same constructor and the child reports the hash of what it
  loaded, so a mismatch fails. A test fails if the table omits a filter that
  the existing clone test covers.
- **S2. The filter never loaded and the kernel answered instead.** Then
  `clone3(NULL)` is EINVAL or EFAULT, not ENOSYS, and `clone(NEWUSER|THREAD)`
  is EINVAL, not EPERM. The child also reports `Seccomp: 2` from
  `/proc/self/status`, and the judge requires it.
- **S3. A probe succeeds and does harm.** The clone probe uses a flag
  combination the kernel rejects. `unshare` in a multithreaded Go process
  can only answer EPERM (the filter) or EINVAL (the kernel). The child is a
  throwaway process in its own process group, killed with the daemon
  (Pdeathsig) and bounded by a timeout.
- **S4. The refusal breaks legitimate children.** Both controls must start a
  child that exits 0. The child execs the same binary in a no-op mode, so
  the control needs nothing else on the guest.
- **S5. The leg takes caller input.** It doesn't. The filter names are
  compiled in, the route takes only `leg=seccomp`, and the caller rule is
  the OOM leg's (see the review bunch 22 section).
- **S6. The judge trusts the server.** The staging script holds its own list
  of the six filters and the expected errno for each probe. It requires
  each child's hash to equal the daemon's expected hash and the six
  programs to be distinct.

Built 2026-10-22 on `lane/fp2-selftest-oom`: `internal/capsule/seccomp_selftest*.go`, the
`capsule-seccomp-selftest` mode in `cmd/autoputer/main.go`, `?leg=seccomp`, and
`desk_cell_confinement_probe.py --leg seccomp` (FP2 proof step 7). Local
evidence, unprivileged on Linux 6.18: `TestSeccompSelfTestThroughAChild` and
`TestIsolationSelfTestSeccompLegThroughTheRealBinary` pass. The deployed run
is FP0's.

## Review bunch 22 (2026-10-22): what was wrong and how it was repaired

The panel was Gemini flash, DeepSeek and Devin, and it sent back
`bb50792d0`. What was wrong, and the repair for each:

1. **Reachability.** "Host peer only" was false at the authorization layer.
   The proxy forwards every authenticated `/api/*` call to the guest and
   injects the owner (`internal/proxy/handlers.go`, `HandleProtectedAPI`).
   So any owner browser session, or any API key with `write:runtime`, could
   POST `?leg=oom`, including on the always-on paper.

   The OOM and seccomp legs now live on `POST
   /internal/runtime/isolation-selftest?leg=oom|seccomp`:
   - the public edge refuses `/internal/*`, and the proxy never forwards it;
   - the guest admits only its host peer (`server.GuestHostCaller`: the
     default gateway, never loopback or any other in-guest address), then
     calls `hostauth.Authorize`, the pairing `/internal/debug/pprof/` uses.
     Round 2 found that the deployed guest runs hostauth in off mode, where
     `Authorize` returns true, so the credential is not checked there: the
     transport check alone is the gate (see "Review bunch 27" below);
   - the `/api` route answers 400 to a `leg`, and the proxy refuses any
     `leg` on it with 404 before the guest, for cookie sessions and API keys
     alike.

   The probe and the FP2 proof steps reach the internal route from Node B
   over ssh, with the token header file passed to curl by path (the guest
   ignores it while hostauth is off).
2. **No free-memory guard.** The leg now refuses when MemAvailable is below
   the budget plus 1 GiB headroom: 512 MiB is the daemon's `MemoryMin`, and
   512 MiB covers the other desk workers' growth and the store's page cache.
   It checks before any worker starts and again before the cell. Both
   numbers are in the receipt.
3. **The runaway could run without a live sibling.** A sibling that fails to
   spawn, or does not answer its first ping, now ends the leg before the
   runaway spawns.
4. **A nil-pointer panic on the timeout path.** A cell timeout reaped the
   worker and released its cgroup (`w.budget = nil`) before the leg read
   `w.budget.path`. The path is now saved before `Eval`.
5. **A kill before the cell ran could pass.** The fixed cell's first act is
   `choir.ReadCapture("choir-oom-selftest-start")`. It reaches the host as
   a broker frame mid-cell, and the self-test's own egress hook records it.
   Both judges require this start marker.
6. **The judge accepted any confinement mode.** Both judges now require mode
   `required`, the guest's production mode. A test spawned in auto mode
   therefore fails only on the mode.

The seccomp children's timeout now kills the child's whole process group
(`cmd.Cancel`).

Not yet proved: the root kill test, and both legs on the deployed guest
(FP0).

## Review bunch 27 (2026-10-22): the deployed gate is the transport check alone

DeepSeek v4.1 flash accepted round 2 with edits. Its E1 was a doc-accuracy
finding, and it is right. Checked against the code:

- `nix/autoputer-vm.nix` sets neither `CHOIR_INTERNAL_AUTH_MODE` nor an
  internal credential. Only `nix/node-b.nix` does (`internalAuthMode =
  "enforce"`, for the host services). No Go code sets the variable either.
- `hostauth` treats an empty mode as `off` (`internal/hostauth/hostauth.go`),
  and `Authorize` returns true in off mode. Its doc comment says to call it
  only after the transport check.
- So on a deployed guest the handler's gate,
  `server.GuestHostCaller(r) && hostauth.Authorize(r)`, is
  `GuestHostCaller` alone: the request must come from the guest's default
  gateway, the host's address on the tap. That is the same rule as the
  planned-restart routes (`requireInternalRuntimeCaller`). `/internal/debug/pprof/`
  pairs `Authorize` with `HostSourcedCaller`, which is looser (it also admits
  loopback).

What this changes in the text above: "holding the internal service
credential" overclaims for the deployed default. It is corrected in the
handler comment (`internal/agentcore/isolation_selftest.go`), the probe
docstring, and the FP2 definition. What it does not change: the gate's
behavior, or the guest's auth mode. Setting enforce mode and loading a
credential on guests would be a red change of its own and is not taken here.

Residual, named: **guest internal routes rely on the transport gate; enforce
mode on guests is a follow-up.** A guest-root process with CAP_NET_ADMIN could
assign the gateway address locally and pass `GuestHostCaller`. A confined desk
cell cannot: it is non-root, in a private network namespace, and its seccomp
filter has no socket or connect. The same residual already applies to pprof
and the planned-restart routes. Reviewer's lower-severity notes, recorded and
not acted on: any Node B process can run the legs on any guest (the paper
guard lives in `scripts/proofs/lib.sh`, not the route), and the legs' Linux-only
runtime paths are covered by unit tests and inspection until FP0's Linux run.
