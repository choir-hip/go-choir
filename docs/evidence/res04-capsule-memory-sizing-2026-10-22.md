# RES-04 sizing evidence: what an engineering capsule's release build needs (2026-10-22)

For RES-04 (capsule memory per doctrine,
`internal/capsule/capsule_memory.go`): the numbers the request and the
ceiling should be set from. The problem record is
`docs/problems/capsule-release-build-runs-at-the-2gib-ceiling-2026-10-22.md`;
the fix is fd8337941 (lane/fp4-capsule-memory).

## Deployed receipts (observe policy, deploy Q, test computer 71afbf14)

| capsule | work | request = admitted = max | peak | events high / max / oom / oom_kill | doctrine_would_throttle |
|---|---|---|---|---|---|
| capsule-3b63529a… (A) | release build (Go and frontend), about 20 min | 2 GiB | 2,147,594,240 (at max) | 0 / 385,311 / 73 / 2 | true |
| capsule-9ae08362… (B) | three cheap receipts | 2 GiB | 158,109,696 | 0 / 0 / 0 / 0 | false |
| step 3 capsules (earlier run) | `go vet`, small work | 2 GiB | about 650 MB | 0 | false |

`peak` is `memory.peak`, which counts page cache. Receipts do not yet
separate anonymous memory from page cache.

## Local measurement (capsule-shaped scope, cold cache, ./cmd/autoputer at 71ec8e704)

`systemd-run --user --scope -p MemoryMax=2G -p MemorySwapMax=0
-p CPUQuota=100%`, fresh `GOCACHE`, warm module cache, 1,420 packages.
Script: a 0.5 s sampler of `memory.stat` `anon` beside `time -v go build`.

| shape | wall | anon peak | largest process RSS | memory.max events | oom_kill |
|---|---|---|---|---|---|
| GOMAXPROCS=4 (today's capsule: Go sees 4 guest CPUs) | 9 m 32 s | 1.27 GiB (compile phase) | 1.44 GiB | 1,957 | 0 |
| GOMAXPROCS=1, GOMEMLIMIT=1280MiB (fd8337941) | 7 m 53 s | 846 MiB (link phase) | 1.50 GiB | 3,570 | 0 |

## What this says for RES-04's values

- **Request.** With the bounds, a cold Go release build needs about
  850 MiB anonymous plus page cache it can give back. The broker and session
  worker add about 150 MiB (B's whole capsule peaked at 158 MB). The 2 GiB
  request (`engineering_assignment_runtime.go:34-38`, the desk manifest's
  `CapsuleRequestedBytes`) leaves about 1 GiB of anonymous headroom for a
  frontend build or a model cell beside it. No change to the request.
- **memory.high under doctrine.** A `memory.high` at the request (2 GiB)
  throttles a build whose page cache fills the cgroup: `memory.high`
  reclaims page cache first, so the throttle is reclaim work rather than
  an anonymous stall, but `doctrine_would_throttle` (peak above request)
  will read true for every cold build, because `memory.peak` counts page
  cache. A receipt field for the anonymous peak would make the flag mean
  what it says. That is a follow-up, not in this change.
- **Raising the cap is not needed by this evidence.** A 4 GiB build
  capsule would take two thirds of the 6 GiB capsule budget (3/4 of the
  guest's 8 GiB) and half the guest, and guest page cache does not return
  to the host.
- **Build cache.** Every capsule compiles 1,420 packages cold (1.4 GB of
  cache). A seeded cache would cut the time and the page cache. A shared
  writable cache lets one capsule poison another's builds, so it needs its
  own design (red). That is a residual.

## Deployed proof (FP0's; on test computer 71afbf14 only)

After fd8337941 is deployed and `/health` on 71afbf14 reports it:

1. Precondition: no self-development operation is open (`choir self-dev
   head`).
2. Cast request A of the FP4 drill
   (`docs/evidence/fp4-admission-queue-deployed-proof-2026-10-22.md`, "Plan
   and prompts"), with one line added before its step 1: "First record
   `sh -c 'echo GOMAXPROCS=$GOMAXPROCS GOMEMLIMIT=$GOMEMLIMIT'` once as a
   cell exec and once as a job." Expected output for both:
   `GOMAXPROCS=1 GOMEMLIMIT=1342177280`.
3. Let A build and freeze. It does not need to be approved; reject or leave
   it as the drill says.
4. Read A's capsule receipt: the guest log line "capsule … memory:
   policy=observe …" at teardown, or `capsule_memory` in GET
   /api/runtime/observability. **Pass: `oom_kill=0`** (and `oom` far below
   73). Record `peak`, `events max` and `doctrine_would_throttle`.
5. Read the build job's wall time from its status in A's trace (`choir
   trace export`: the job's started and ended times, its argv and exit
   code). Record it next to the 2026-10-22 run's "about 20 minutes".
   Expected well below that if the build is one job from a cold cache.
6. Record whether any `capsule_go_eval` cell hit the 30 s broker read
   timeout. Three did in the 13:40 run. Fewer or none supports the
   reclaim-stall hypothesis, but does not prove it.

A fail is `oom_kill > 0`. Then read A's job argv and start times. Did two
heavy processes run at once (the frontend build beside `go build`, or a
second build)? Did a cell override the bounds? Those cases need a
different fix (serialize the release build's jobs) rather than a larger
cap.
