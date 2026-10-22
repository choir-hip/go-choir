# FP4 freeze close, failure branch: deployed proof on a test computer (2026-10-22)

Run by the self-development monitor (lane/fp4-selfdev-monitor) for FP0, after
deploy I (4c501bae6, which carries f68100cf6). It follows the earlier drill,
docs/evidence/fp4-freeze-close-deployed-proof-2026-10-22.md, which exercised
only the completed-assignment branch. Target: the test computer
`computer-71afbf1461dffabcce8eeebe558d03bd` (vm-c5c801f8…, warmness `test`),
and no other computer. No apply is approved.

Under proof: `closeEndedAssignmentSelfdevOperation`
(internal/agentcore/engineering_assignment_fate.go). An assignment whose
own report ends it `failed` while its operation is still `executing`
(after a refused freeze) closes the operation `failed`, with the terminal
error "assignment failed before its candidate froze; last refusal: freeze:
…".

## Plan (written before any act)

1. Wait for deploy I's `deploy refresh caller=deploy` round, and confirm
   the computer serves 4c501bae6 or a descendant of f68100cf6.
2. Clear the open candidate `selfdev-bb0cebfa9edc224e29045f4921763db9`
   (`awaiting_approval`). Reject it, as the test computer's owner, through
   the product path (`choir self-dev reject --reason …`, which is
   `POST …/self-development/operations/<id>/decision` with
   `decision: reject`). Never approve it. With it open, a new freeze is
   refused `ErrOperationOpen` (FP4's R3 case), and that would confound
   the drill.
3. Cast the request below through the prompt bar (`choir run start`).
4. Expected:
   - the assignment reports `failed`;
   - its operation closes `failed` with "assignment failed before its
     candidate froze; last refusal: freeze: capsule freeze: computer-surface
     frontend artifacts are underivable";
   - obligations selfdev reads busy 0, stalled 0;
   - /health reads no `selfdev_active_operations`, the field vmctl's busy
     probe reads (internal/vmctl/ownership.go:3459-3476);
   - the next vmctl refresh of this computer is not deferred by the
     operation.
5. Budget: about 40 minutes of wall time from the cast. If the worker does
   not give up, or management re-casts, the drill stops: I record it here
   and cancel nothing on my own.

## Mechanism and reasoning

The operation must open first. Only a refusal after that point leaves the
operation `executing` with a stamp for the close to carry. Read in code
(origin/main 97de4db0b; deploy I changes only the cancellation close in engineering_assignment_fate.go):

- `choir.Freeze` stages without checking its references
  (internal/yaegikernel/intent.go:437).
- The reducer's freeze body, `freezeCapsuleEffectBundle`
  (internal/agentcore/tools_capsule.go:156), refuses only two things
  before it opens the operation:
  - an empty build, test or toolchain slot;
  - a job still running.
- It then opens the assignment's operation (`OpenFreezeCandidate`, tools_capsule.go:189).
- Every later refusal leaves the operation `executing`, and the reducer
  stamps `last_intent_error` as "freeze: <error>"
  (internal/agentcore/rlm_reduce.go:1465). Those refusals are: receipt
  resolution, the distinct slots, the receipt outcomes, the classifier,
  staging the release and the source-change check.

The refusal chosen is the one the first drill met naturally:

- "capsule freeze: computer-surface frontend artifacts are underivable"
  (internal/capsule/executor.go:1866).
- A release with no `frontend/` file can never freeze.
- The capsule is fresh, so it has no frontend build unless the worker runs
  one (in the drill, `vite build` under `choir.Start`).

The request therefore forbids every build and names three cheap
receipts:

- toolchain: `go version`, `node --version` and `npm --version`;
- check: `gofmt -l` on the one file;
- build stand-in: a `test -s` on the file.

It then asks for one Freeze and, on refusal, a failed report with no
retry. Why this mechanism:

- **Harmless.** The change is one comment, inside a capsule on a test
  computer. Nothing can freeze, so nothing reaches a decision or an
  apply.
- **Cheap.** No go vet and no go build, which avoids the cold-cache
  module download that took 9 min 43 s last time. There is no npm, and
  only a handful of cells.
- **Test-only.** The prompt names itself a drill. It asks management not
  to re-cast, so no second assignment and no second operation.
- **Robust to how the worker reads it.** It does not depend on the worker
  citing receipts correctly. If it cites a slot wrongly, the
  distinct-slot or outcome refusal still comes after the operation opens,
  so the stamp is still there. Only the refusal text would differ.

Risks:

- Management may paraphrase the constraint away when it casts (it
  paraphrased the first drill's request). If the worker then builds the
  frontend, the freeze succeeds. The drill records that and stops
  without approving.
- The worker may give up with `blocked` or `partial` instead of `failed`.
  Those are recorded as they come.

## The request (sent verbatim through the prompt bar)

> Engineering freeze-failure drill FP4FF-20261022 (test computer only; it
> exercises the failed-before-freeze path, so nothing should freeze, be
> approved or be applied). Through the engineering desk, in one assignment:
> 1. In internal/selfdev/operations.go add the comment line "// Operation
> is one self-development change, from freeze to its decision." directly
> above "type Operation struct".
> 2. Record three cheap receipts, each in its own cell: the toolchain
> (`go version`, `node --version`, `npm --version`); the check
> `gofmt -l internal/selfdev/operations.go` (no output expected); and, as
> the build stand-in, `sh -c 'test -s internal/selfdev/operations.go'`.
> 3. Do not run go vet, go build, npm or vite. This drill keeps the capsule
> free of frontend and binary build artifacts on purpose.
> 4. Call Freeze exactly once, citing those three receipts. It is expected
> to be refused, because the release has no frontend artifacts.
> 5. When Freeze is refused, do not retry and do not work around it:
> Complete the assignment as failed, quoting the refusal text.
> Management: cast this once and do not re-cast or retry; report the
> failure back to me as the result.

## Acts and results

### Step 2 (run first, at 08:03, before deploy I). Clear the open candidate: done (the decision landed, but the API answered 409)

- 08:01:38: `choir self-dev show` read selfdev-bb0cebfa… as `awaiting_approval`, with bundle `9a10224c…` and verifier `01bb42de…`.
- 08:03:09: `choir self-dev reject --idempotency-key fp4ff-reject-bb0cebfa-20261022 --reason "FP4 drill: clear …"`.
  - The CLI exited 1 with `http 409: {"error":"self-development operation conflict: state is rejected, expected awaiting_approval"}`.
  - The build was still 43e318712; deploy I was in CI.
- 08:03:2x: the operation is `rejected` (updated 08:03:10.635).
  - decision_actor: the test computer's owner;
  - decision_event `cdc10b03…`;
  - decision_receipt `01a162a2-ac78-7894-8a0b-913e8dc04f63`.
- So the candidate is cleared, and no approve was sent.
- The wrong status is recorded as docs/problems/selfdev-reject-lands-but-answers-409-2026-10-22.md (lane commit bd2d4e0f3).

### Step 1. Deploy I on staging, and the wake: done

- Deploy I changed identity. 4c501bae6's CI failed on a flaky tapestore
  test, and so did 54040b730's run (39745680806). Staging served
  `58e7058023269e9e7e0540830bb7d74cd8eca559` from 08:31:18
  (`x-choir-build-commit`); it descends from f68100cf6.
- vmctl's `deploy refresh caller=deploy` round for 58e705802 ran
  08:32:54-08:34:36:

  | vm | outcome |
  |---|---|
  | vm-4a9afa23 (the delegate's) | deferred |
  | vm-1669 | refreshed |
  | candidate-fleet-e15cb89f | refreshed |
  | vm-48bc0981 | refreshed |

  The plan (`GET /internal/vmctl/deploy-refresh`, read-only) listed 4
  candidates, 0 skipped and 1 pending. It did not list the test computer,
  which vmctl had hibernated for idle at 08:06:14 (snapshot 6029 ms).
  So the round logs no line for it.
- 08:35:53: owner lifecycle start on the test computer, key
  `fp4ff-wake-20261022T0836`. Result:
  - 201, receipt `01a162c0-c6e0-7677-b620-909444019c09`;
  - hibernated to active, epoch 13232 to 13233.
- vmctl at 08:35:54:
  - "snapshot resume of VM vm-c5c801f8… not used (launch_config_mismatch)";
  - "cold boot … after snapshot fallback";
  - booted at 08:36:03, host 10.201.118.2.
  - **The snapshot from the old build was refused by the launch-config
    hash, and the computer cold-booted.**
- Guest /health at 08:36: ready, running_runs 0, no
  `selfdev_active_operations`, build and deployed commit 58e705802
  (deployed_at 08:36:00Z).
- Obligations: selfdev `{open 0, busy 0, stalled 0}`; restart
  `crash_or_stop` (a cold boot with no planned-restart marker).
  selfdev-bb0cebfa stays `rejected`. Mode `propose_only`, generation 1.

### Step 3. The cast

- 08:36:50: `choir run start --idempotency-key=fp4ff-cast-20261022T0837`
  with the request above, verbatim. Result:
  - trajectory `978fbce4-665c-50ff-b74a-7a3ad94fc384`;
  - doc `fc911a4e-c75c-5892-a139-9e817396243a`;
  - submission `b50b5651-0da4-5e8b-95f7-c85992a048a7`.

### Step 4. The freeze is refused, the assignment fails, the operation closes `failed`: PASS

Trajectory events (`GET /api/trajectories/978fbce4…/events`):

| seq | time (UTC) | kind | note |
|---|---|---|---|
| 1 | 08:36:51.126 | trajectory_started | fp4ff-cast-20261022T0837 |
| 2-3 | 08:37:09.315 | texture_turn_committed, work_settled | Texture routed it to management |
| 4 | 08:37:21.778 | co_super_assignment_opened | assignment-80a8fbe25dcb9d6ce3c46816 |
| 5 | 08:37:39.814 | co_super_assignment_bound | |
| 6-9 | 08:38:37.388-08:38:39.007 | co_super_capsule_disposition_set | freeze_requested, frozen, revoke_requested, revoked (the fate path's capsule teardown) |
| 10 | 08:38:39.092 | co_super_assignment_reported | report:sha256:5a96130a… |
| 11 | 08:39:02.719 | texture_turn_committed | "confirms the expected freeze refusal, no retry or workaround, and no candidate for verification" |
| 12 | 08:39:03.322 | trajectory_settled | |

The engineering run's tool calls (trace export `trace-978fbce4-final.json`):

| time | call | result |
|---|---|---|
| 08:37:58 | edit operations.go | wrote, receipt capsule-go-eval:sha256:04db1a06… (management's cast paraphrased the comment text; irrelevant here) |
| 08:38:03 | `choir.Exec` go version, node --version, npm --version | go1.26.9, v24.21.0, 11.19.0; receipt b566ffe6… |
| 08:38:09 | `choir.Exec gofmt -l …/operations.go` | no output; receipt daf2549d… |
| 08:38:15 | `choir.Exec sh -c 'test -s …'` | exit 0; receipt 14b26008… |
| 08:38:21.446 | `choir.Freeze(b566ffe6…, [daf2549d…], [14b26008…])` | **tool_error "reduce: persist tray-1: capsule release contains no frozen runtime artifacts"** (08:38:21.946) |
| 08:38:37.162 | `choir.Complete("failed", "none", "…Freeze refused as expected…No retry or workaround performed.", …)` | report committed 08:38:39.092 |

- The refusal is the empty-release one (internal/capsule/executor.go:1846,  `len(files) == 0`), which fires before the frontend check. The plan
  allowed for either.
- The worker cited the toolchain cell as the build recipe and the
  stand-in as the toolchain. The slots are distinct and successful, so
  the refusal came from staging the release, after the operation opened.
- Management cast once, with no re-cast. The engineering run ended
  `failed`.

The operation (`GET …/self-development/operations`, host-internal, 08:38:4x
and 08:39:57):

```
operation_id       selfdev-5e28993f14799a5c73e912d8d368ec5d
trajectory_id      978fbce4-665c-50ff-b74a-7a3ad94fc384
state              failed
error              assignment failed before its candidate froze; last refusal: freeze: capsule release contains no frozen runtime artifacts
last_intent_error  freeze: capsule release contains no frozen runtime artifacts
verifier_refs      []   (no bundle digest: nothing froze)
created_at         2026-10-22T08:38:21.693013Z   (the freeze opened it)
updated_at         2026-10-22T08:38:39.241779Z   (149 ms after the report)
```

- The poller (every 15 s) saw `executing` with the stamp
  "freeze: capsule release contains no frozen runtime artifacts" at
  08:38:24, then `failed` at 08:38:40.
- This is the terminal error the close builds
  (engineering_assignment_fate.go:1440 on deploy I) from the `failed`
  disposition and the stamp.

### Step 5. Obligations and health: PASS

- `GET /api/runtime/obligations` at 08:38:4x and 08:39:57: selfdev
  `{open 0, busy 0, stalled 0}`.
- `GET /health` (guest, through the host) at 08:39:57: status ready,
  running_runs 0, and none of `desk_pending_mutations`,
  `selfdev_active_operations` or live capsules. These fields are
  omitempty, so absent means 0.
- vmctl's guest busy probe returns busy only when
  `running_runs > 0 || desk_pending_mutations > 0 ||
  selfdev_active_operations > 0` (internal/vmctl/ownership.go:3489). All
  three read 0.
- At 08:38:4x, while Texture was writing its report turn, /health read
  running_runs 1 and desk_pending_mutations 1, with
  `selfdev_active_operations` already absent.

### Step 6. The next vmctl refresh: not yet observable

- No deploy has been pending since the drill. vmctl has logged no line
  for vm-c5c801f8 since its 08:36:03 boot.
- What is shown is that the probe vmctl uses to defer a refresh reads not
  busy (step 5).
- I will append the outcome of the next refresh round, or of an idle
  hibernate, which uses the same probe, when one happens.

## Result

The failure branch of the freeze close passes on staging, on deploy I
(58e705802), on the test computer:
- the assignment reported `failed` after a refused freeze;
- the operation closed `failed`, 149 ms after the report, with "assignment
  failed before its candidate froze; last refusal: freeze: capsule release
  contains no frozen runtime artifacts";
- obligations read busy 0 and stalled 0;
- /health shows no `selfdev_active_operations`.

From cast to close took 1 min 49 s. Nothing froze, and nothing was
approved or applied.

## Raw evidence (local scratchpad; not in the repo)

The files are under
`/private/tmp/claude-501/-Users-wiz-Dev-go-choir/99fb2dd5-a4f7-4cc5-a0f5-6ee541f24543/scratchpad/mon/ffproof/`:

- acts.log;
- s1-wake-*, s1-wake-vmctl.txt, s1-health-*, s1-obl-*, s1-ops-*;
- s2-reject-*, s2-ops-after-reject-*, s2-head-*, s2-observability-*;
- s3-prompt.txt, s3-run-start.json;
- s4-ops-*, s4-events-*;
- s5-obl-*, s5-health-*;
- s6-health-*, s6-obl-*, s6-ops-*, s6-traj-*;
- poll.log;
- trace-978fbce4-final.json.
