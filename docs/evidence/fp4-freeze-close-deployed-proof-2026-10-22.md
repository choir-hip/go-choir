# FP4 freeze close: deployed proof on a test computer (2026-10-22)

Run by the self-development monitor (lane/fp4-selfdev-monitor) for FP0.
Read-only except for the acts listed under "Acts", all on the test
computer `computer-71afbf1461dffabcce8eeebe558d03bd` (vm-c5c801f8…,
warmness `test`). No act touched the owner's, the delegate's
(ad1b46d4) or the paper's (1355b83b) computer. No apply was approved.

Fix under proof: the close of an ended assignment's self-development
operation (`closeEndedAssignmentSelfdevOperation`,
internal/agentcore/engineering_assignment_fate.go), deploy G (4fca1e01b,
which contains 7c9e92bb5). Problem record:
docs/problems/selfdev-freeze-failure-leaves-operation-executing-2026-10-22.md.

## Ledger-ready summary

- Deploy G's freeze close, deployed proof on test computer 71afbf14:
  **the terminal-failure branch was not exercised; the
  completed-assignment branch was, and it held.**
- The cast went through the prompt bar to management, then to one
  engineering assignment. The first freeze was refused, as FP0
  expected: "capsule freeze: computer-surface frontend artifacts are
  underivable", at 07:32:41.
- The worker then built the frontend and froze again 30 s later, and
  that freeze succeeded. The assignment reported `completed`.
- The operation went executing, then frozen, then verified, then
  `awaiting_approval`. The close left it alone, as the invariant says:
  a completed assignment keeps its frozen candidate for Verify.
- So the operation never ended `failed`. Steps 4 and 5's failure
  expectation is still open: the candidate never reached a terminal
  failure.
- After the drill:
  - obligations selfdev reads `{open 1, busy 0, stalled 0}`;
  - /health is ready, with running_runs 0.
- vmctl deferred the computer three times for deploy H (43e318712) while
  the operation was busy. It refreshed at 07:34:17
  (`outcome=refreshed planned_restart=true`), 16 s after the turn
  settled.
- The `awaiting_approval` candidate survived that planned refresh.
- The candidate is left undecided on purpose.
- Side observations, not defects:
  - `go vet ./internal/selfdev` took 9 min 43 s, downloading the
    module graph;
  - the worker polled 84 times in that window.

## Build identity

| when (UTC) | x-choir-build-commit / guest build | source |
|---|---|---|
| 07:18:38 | guest `4fca1e01bce374dedd80aa45bdc60b35f641a29b` (deploy G) | /health, s1-health.txt |
| 07:34:14 | guest `43e318712553e2a69d5da87d37d41963223d0424` (deploy H) | /health, s5-health-073419.txt |

Deploy G (4fca1e01b) contains the close and its follow-ups. Deploy H
(43e318712) is a later descendant, and the planned refresh moved the
computer onto it in the middle of the drill (see step 6).

## Acts (all on the test computer, test key, sent by fd and never printed)

| when (UTC) | act | result |
|---|---|---|
| 07:10:15 | `POST …/lifecycle/start` key `fp4-freeze-close-wake-20261022T0710` | 502, which raced deploy G's vmctl restart; recorded in docs/problems/wake-during-deploy-vmctl-restart-orphans-the-vm-2026-10-22.md (on main as 7e87dd3d3) |
| 07:18:31 | `POST …/lifecycle/start` key `fp4-freeze-close-wake-20261022T0721` | 201, receipt `01a16279-f537-78f9-8c09-9d366d0e2726`, stopped to active, epoch 13229 to 13231 |
| 07:20:16 | `choir self-dev mode set --mode=propose_only --expected-generation=0` | generation 1 |
| 07:20:38 | `choir run start` (prompt bar), command `fp4fc-cast-20261022T0721` | trajectory `e4a33e14-fd85-5307-8faa-a22ece7bfed4`, doc `97f522a5-1b11-5ab2-9f55-6722a7f36114`, submission `417bd8ec-dc15-53f0-8ff6-c9dde1deb5c4` |

The prompt was:

> Engineering change request (FP4 freeze-close proof FP4FC-20261022T0721):
> through the engineering desk, make one small, real, reversible source
> change on this computer. In internal/selfdev/operations.go, add the
> comment line "// Operation is one self-development change, from freeze
> to its decision." directly above "type Operation struct". Check it with
> `go vet ./internal/selfdev`. Freeze the candidate for my review and
> await my decision; do not apply it.

## Steps

### 1. The computer serves the deploy: PASS

/health at 07:18:38 reported `4fca1e01b`, a descendant of 7c9e92bb5, with
status ready.

### 2. propose_only, and a management cast through the prompt bar: PASS

- mode `propose_only`, generation 1;
- trajectory events (`GET /api/trajectories/e4a33e14…/events`):

| seq | time | kind | command |
|---|---|---|---|
| 1 | 07:20:39.887 | trajectory_started | fp4fc-cast-20261022T0721 |
| 2 | 07:21:09.012 | texture_turn_committed | texture-turn:29060839… ("routed execution and verification through management") |
| 3 | 07:21:28.657 | co_super_assignment_opened | engineering-open:assignment-eaa73935d0f9670f499fb0f0 |
| 4 | 07:21:49.765 | co_super_assignment_bound | engineering-bind:assignment-eaa73935d0f9670f499fb0f0 |
| 5 | 07:33:17.001 | co_super_capsule_disposition_set | …:1:freeze_requested |
| 6 | 07:33:17.756 | co_super_capsule_disposition_set | …:1:frozen |
| 7 | 07:33:18.727 | co_super_capsule_disposition_set | …:1:revoke_requested |
| 8 | 07:33:21.386 | co_super_capsule_disposition_set | …:1:revoked |
| 9 | 07:33:21.529 | co_super_assignment_reported | engineering-report:…:report:sha256:6161683e… |
| 10 | 07:34:01.099 | texture_turn_committed | texture-turn:d0f05251… |
| 11 | 07:34:01.099 | work_settled | texture-turn:d0f05251… |

### 3. The freeze: refused once, then succeeded (not the expected terminal failure)

From the engineering run's tool calls (trace export
`trace-e4a33e14-final.json`, layer `run_events`):

| time | call | result |
|---|---|---|
| 07:22:02 | `choir.Start(go vet ./internal/selfdev)` | job-ba43a9fe9c28 |
| 07:31:45 | (loop.retry `capsule_job_ended`) | vet done, exit 0, receipt capsule-exec:sha256:2646ba1a… |
| 07:31:57 | `choir.Start(go build -o …/release/bin/autoputer ./cmd/autoputer)` | job-2f07711a8707, done 07:32:30, receipt capsule-exec:sha256:dfcacdff… |
| 07:32:33.422 | `choir.Freeze(dfcacdff…, [2646ba1a…], [0161a8c3…])` | **tool_error: "reduce: persist tray-1: capsule freeze: computer-surface frontend artifacts are underivable"** (07:32:41.918) |
| 07:32:44 | `choir.Start(sh -c "cd frontend && … vite build")` | job-1609e2d7edb4, done 07:33:01, receipt capsule-exec:sha256:3436e3e8… |
| 07:33:03.684 | `choir.Freeze(…)`, the same arguments | "freeze staged", bundle (draft) `762e1a2bb8aa51271120374b5b4998cbbbfc5897e88099596bc068176f4b7001`, change_count 69 (07:33:12.802) |
| 07:33:16.732 | `choir.Complete("completed", …)` | report committed 07:33:21.529 |

- The refusal comes from internal/capsule/executor.go:1866. The capsule
  release held no `frontend/` file yet.
- The operation was created by the first freeze at 07:32:33.723, in
  state `executing`.
- The refusal is retryable while the assignment lives
  (rlm_reduce.go:1465): it stamps `last_intent_error` and fails only the
  cell.
- The stamp itself was not observed:
  - the stamp write is direct SQL, off the tape;
  - the 07:32:44 poll recorded only the state.
- The successful freeze clears the stamp (rlm_reduce.go:1473).
  `last_intent_error` is null now.

### 4. The operation after the report: `awaiting_approval`, not `failed`

`GET …/self-development/operations` (host-internal), 07:34:03 and
07:37:20:

```
operation_id   selfdev-bb0cebfa9edc224e29045f4921763db9
state          awaiting_approval
capsule_id     capsule-7660af55-344f-54ee-8790-5e45b7b53e64
bundle_digest  9a10224c0b2f80248b8f69791295c3d8c389926b863bfac28292cfb29d7efb68
verifier_refs  [01bb42de805e2a1a32f6d3e3131cb6a34ac329a0d8e3ffee51278bf1323f48b5]
created_at     2026-10-22T07:32:33.723043Z
updated_at     2026-10-22T07:33:38.409266Z
last_intent_error  null
```

- The poller saw `executing` at 07:32:44, `frozen` at 07:33:25 and
  `awaiting_approval` at 07:33:46.
- The report ended the assignment `completed` while the operation was
  `frozen`.
- For a completed assignment the close only takes `requested` and
  `executing` (engineering_assignment_fate.go:1411-1414), so it
  correctly left the frozen candidate for management's Verify.
- The expected "assignment … before its candidate froze; last refusal:
  freeze: …" was not produced, because the assignment did not end
  before its candidate froze.
- The bundle digest changes by design:
  - the report cites the draft digest `762e1a2b…`;
  - the verified finalizer replaces it with `9a10224c…`
    (tools_capsule.go:328 at frozen, :579 at verified).

### 5. Obligations and health: PASS (busy 0, stalled 0)

`GET /api/runtime/obligations`, 07:34:20 and 07:37:20:

```
selfdev  {open 1, busy 0, stalled 0}
         selfdev-bb0cebfa… awaiting_approval, busy false, stalled false
restart  {kind planned, reason "platform update", target 43e318712…}
open_work_items  {count 1, oldest_at 2026-10-22T05:24:19Z}
```

- The one open work item predates this drill (05:24). The drill's own
  work item is `completed`.
- /health at 07:34:19: status ready, runtime_health ready,
  running_runs 0, commit 43e318712.

### 6. vmctl's deploy refresh for this computer: PASS (refreshed after three busy deferrals)

From the vmctl journal on Node B, for vm-c5c801f8…:

| time | caller | outcome |
|---|---|---|
| 07:31:36 | deploy, target 43e318712 | deferred, "busy: … self-development operation or a live capsule would not survive a restart" |
| 07:32:07 | retry-sweep | deferred, same reason |
| 07:33:07 | retry-sweep | deferred, same reason |
| 07:34:17 | retry-sweep | **refreshed**, planned_restart=true, new epoch 13232, host 10.201.114.2:8085 |

- The guest console at 07:34:16-17 shows the boot phases
  `selfdev_close_after_crash` (0 s) and
  `selfdev_materialization_reconcile` (0 s).
- There is no "closed from" line: the boot was planned (obligations
  `restart.kind planned`).
- The `awaiting_approval` operation survived the refresh unchanged.

## Side observations (not defects; trace evidence, with hypotheses labelled)

- **The vet check was slow.** `go vet ./internal/selfdev` ran 9 min 43 s
  (07:22:02 to 07:31:45).
  - The job tails show `go: downloading …` lines for the module graph.
  - Hypothesis: the capsule's Go module cache starts cold.
  - The go build that followed took 33 s, and the vite build 17 s.
- **The worker polled the whole time.** The engineering worker made 84
  `capsule_go_eval` calls during the vet wait, 114 in total. Many were
  `println("waiting")` cells. Each call is a model turn.
- **The first freeze cost one round trip.** The worker did not build the
  frontend before its first freeze. The executor's refusal told it what
  was missing, and it recovered on its own within 30 s.
- **The trajectory still reads `live` after `work_settled`.** That is by
  design: `trajectory_settled` is a separate event
  (types/lifecycle.go:58).

## What remains for the failure branch

- The close's terminal-failure branch is unit-tested on the FP4 lanes.
  It still has no deployed proof:
  - an assignment that ends `failed` (or `completed`) while its
    operation is `executing` with a refusal stamp;
  - expected: operation `failed`, terminal error "assignment … before
    its candidate froze; last refusal: freeze: …".
- A natural trigger needs a freeze that keeps failing until the worker
  gives up. This computer's executor recovers from the frontend refusal,
  so the same prompt is not a reliable trigger.
- Candidates:
  - a change whose capsule build cannot produce `bin/autoputer` (the
    delegate's 10-21 refusal, "capsule release must contain executable
    bin/autoputer");
  - an explicit instruction to report failure after one refused freeze.
- The undecided candidate selfdev-bb0cebfa… stays `awaiting_approval`.
  It is not busy and does not block refreshes (step 6). Leaving it is
  harmless. A reject decision would close it, if FP0 wants it cleared.

## Raw evidence (local, scratchpad; not in the repo)

The files are under
`/private/tmp/claude-501/-Users-wiz-Dev-go-choir/99fb2dd5-a4f7-4cc5-a0f5-6ee541f24543/scratchpad/mon/fcproof/`.
The digest that records them is
`/Users/wiz/Dev/lanes/_runs/digests/selfdev-monitor.md`.

- acts.log;
- s1-health.txt;
- s2-mode-set.json, s2-prompt.txt, s2-run-start.json;
- s3-ops-073403.txt, s3-events-073403.txt;
- s4-snap-073419.txt;
- s5-obl-073419.txt, s5-health-073419.txt;
- s6-ops-073720.txt, s6-obl-*.txt, s6-traj-073720.txt, s6-events-*.txt;
- poll.log;
- trace-e4a33e14-0732.json, trace-e4a33e14-final.json.
