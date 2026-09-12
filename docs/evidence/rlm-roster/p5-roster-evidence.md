# P5 roster evidence — arm attempts on the retained computer

Artifact for the P5-roster acceptance item and for
`finish.landing.required_receipts` ("the frozen roster conformance evidence
artifact naming the id that served each run"). It records every arm attempt
with its durable identity, the provider and model that actually served it, and
the failure mode. Classification of model behaviour stays human-adjudicated at
this stage; this artifact records observations, it does not score models.

Retained computer: `computer-03335285269bdba4f94377e56879f9e6`.
Trajectory (the document work channel every tell targets):
`24693e87-39b8-591a-b08e-4dfd9d49bed6`, subject document
`040930e8-6c6d-5534-8547-6ba844c8070a`.
Frozen desk task: this directory's `p5-frozen-desk-task.txt`, sha256
`a0386c97d302b748e0b708a21176e9daabee406176d4294b8d514464215b560b`, 1051 bytes,
passed through `--expected-task-sha256`.

## Arm attempts

| # | arm (overlay id) | request id | tell version | outcome |
|---|---|---|---|---|
| A1 | `p5-chatgpt-g56luna` | not recorded (pre-harness pilot) | v1 (prose) | assignment opened, stalled on 402 |
| A2 | `p5-deepseek-v41-flash` | `p5-roster-v2-deepseek-v41-flash-arm1` | roster-v2 | refused by the desk as a duplicate |
| A3 | `p5-deepseek-v41-flash` | `p5-roster-v3-deepseek-v41-flash-arm1` | roster-v3 | assignment opened, base-policy served, cancelled |
| A4 | `p5-deepseek-v41-flash` | `p5-roster-v4-deepseek-v41-flash-arm1` | roster-v4 | tell accepted, never driven |

### A1 — `p5-chatgpt-g56luna` (2026-09-12)

- Assignment `run:assignment-0f95f728-4381-52d5-93da-62515ce2951a`, attempt 1,
  kind implementation, capsule `capsule-e4df7e5a-9ac4-51db-84c7-9d93036c7c19`,
  opened 19:06:04Z, last update 19:06:06Z.
- Objective carried the overlay id as **prose**:
  `ROSTER-V1 open exactly one implementation assignment with
  model_policy_overlay_id=p5-chatgpt-g56luna.` The structured field was empty
  (`metadata.model_policy_overlay_id: null`).
- Served: `deepseek / deepseek-v4-flash`, `llm_policy_source
  /mnt/persistent/files/System/model-policy.toml` — the base policy, not an
  overlay. The implementation failed at iteration 0 with a sanitized DeepSeek
  HTTP 402 and produced no artifact.
- The served task body was a **paraphrase** of the frozen §7 bytes, not the
  frozen text.
- Cancelled 20:12:21Z by `choir run cancel`; the reducer then wrote
  `revoke_requested` (seq 1117), `revoked` (1118) and
  `co_super_assignment_cancelled` (1119).
- Parent management run `0d512e91-bcc3-465f-9a91-61ef64d7879e` opened the
  assignment and was cancelled at 20:24Z.

### A2 — refused as a duplicate

- Tell queued at 20:15:56Z (seq 1120). The desk committed a turn at 20:16:43Z:
  *"Incorporate the latest management evidence and preserve the
  single-active-assignment invariant. Do not open a duplicate or claim pending
  execution results."* No assignment opened.
- Cause was the harness, not the desk: six ROSTER-V1 execution work items were
  open at once (`4c202a6b` 15:41, `72c68a04` 16:30, `41a5f3d1` 16:54,
  `3d1eb469` 17:16, `85c8ab7d` 19:04, plus the long-lived `d7d7cf61`), one
  minted per tell and never dispositioned. Document head was revision 16,
  recording the operation as blocked and a retry as needing a new explicit
  execution request on a viable provider.
- Wrapper v3 added the superseded-arm disposition and the explicit
  authorization of the arm.

### A3 — `p5-deepseek-v41-flash`, base-policy served

- Tell queued 20:24:29Z (seq 1123); desk turn committed 20:25:21Z; privileged
  work item `23a34bf6-9a72-49fa-8694-4c6fe03cc849`; control delivered
  20:25:39Z; drainer run `1ad1dc39-8fcc-48ff-8d30-61862ac9347f` created
  20:25:25Z ("Process pending coagent update packets for privileged
  execution.").
- Assignment `co_super_assignment_opened` at 20:26:31Z (seq 1128):
  `run:assignment-239f7309-78ca-5db7-9e4a-dc757559d013`, attempt 1,
  implementation, capsule `capsule-abc9e390-a7f7-5c18-a629-f6326c04790b`,
  created 20:26:38Z, last update 20:26:40Z.
- **Structured field empty** (`metadata.model_policy_overlay_id: null`) even
  though the wrapper named the argument. Served `deepseek /
  deepseek-v4-flash` from
  `/mnt/persistent/files/System/model-policy.toml` — the base policy. The
  engineering desk's call at 20:26:53Z is
  `provider=deepseek model=deepseek-v4-flash messages=1 tools=6`.
- Preflight for this arm resolved `p5-deepseek-v41-flash` to
  `opencode-go/deepseek-v4.1-flash` with `live_run_refused: false`, so the
  overlay itself resolves correctly; it simply never reached the assignment.
- Cancelled 20:37Z; reducer wrote `revoke_requested` (1133) and `revoked`
  (1134).

### A4 — accepted, never driven

- Tell accepted with `cursor: 1136`, `status: pending`, `roster_tell_version:
  roster-v4`.
- No lifecycle event after 1136, no run created after the cancelled A3 arm, and
  no gateway inference call in the following nine minutes. A2 and A3 each
  produced a committed desk turn within 25–52 seconds.
- The drainer run `1ad1dc39` had completed at 20:27:38Z, before the A4 tell, so
  no resident drainer existed to consume the instruction.

## What the arms establish

1. **The frozen task and the overlay both resolve.** Preflight is green for
   `p5-deepseek-v41-flash` → `opencode-go/deepseek-v4.1-flash`, with the task
   digest pinned.
2. **No arm has been served by its arm overlay.** A1 and A3 resolved the base
   policy's `[roles.engineering]`, which is `deepseek / deepseek-v4-flash` and
   unfunded. `roster collect` now records `policy_source` and the served
   provider/model and fails the receipt with
   `policy_source_not_arm_overlay` the moment an arm is base-served, instead of
   polling toward a pass.
3. **Pass/fail for the roster remains unmeasured.** No arm produced a receipt
   with model behaviour, so the expected-pass four (deepseek-v4.1-flash,
   muse-spark-1.3, glm-5.3-flash, gpt-5.6-luna) are untouched.

## Open decisions (recorded in the Definition's `now.next_action`)

- **Mechanism.** The owner decision named the overlay-id mechanism. It depends
  on the desk model translating the instruction into a tool argument, which
  failed twice: prose in the objective was refused loudly by the opener guard,
  and a named argument produced an empty field and a silent base-policy route.
  The Definition's acceptance also allows a per-run engineering base-policy
  swap that is restored afterwards, which needs no model compliance. The base
  policy is re-read on every resolve (`modelpolicy.Manager.Load`), so a swap
  takes effect without a restart.
- **Scope.** Whether the tell-activation gap in A4 is this mission's to fix or a
  product owner-instruction defect to record and escalate.

## Live-catalogue evidence still outstanding

P4-registry requires observing the live catalogue on staging, because a unit
test cannot prove the deployed catalogue. The sealed set asserted in code is
exactly `[capsule_go_eval]`; the served set additionally carries the
reconciliation and report channels named in the P4-registry acceptance item,
and the A3 desk call recorded `tools=6`. A count is not names, and the names
are needed to close the item. They are already durable in the run's
`provider_call_started` event (`tool_names`), but no owner-facing route exposes
run events (`GET /api/runs/{id}` and `POST /api/runs/{id}/cancel` are the only
run sub-routes) and `/api/trajectories/{id}/capsule-evidence/{assignment}`
returns `413 capsule evidence exceeds response bounds`. Closing the item
therefore needs a deploy: either the gateway's inference log line carrying tool
names, or a bounded owner-facing projection of the run-progress events.
