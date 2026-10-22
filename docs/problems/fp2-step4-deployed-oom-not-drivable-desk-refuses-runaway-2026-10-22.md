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
(`docs/problems/desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md`)
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
