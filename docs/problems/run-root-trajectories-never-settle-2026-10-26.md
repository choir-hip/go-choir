# Run-root trajectories never settle, and read as "invalid settlement rule" (found while chasing themes round 7)

Status: investigation draft, read-only. Written 2026-10-26 from b@c reads (choir CLI) and a read-only grep of /Users/wiz/Dev/go-choir.
Mutation class of the finding: green (docs). Nothing was changed on any computer.

## Summary

The symptom was misread. Trajectory 23733daa is not the stuck work. It is the **conductor run's own registry trajectory**, minted by `stampAndMintTrajectory` with a default settlement rule that has no `required_subject_refs`. Every run-root trajectory on b@c looks like this and shows exactly this `waiting_on`. The apply did not cause it; the round 6 prompt at 14:29 produced the identical record.

The real round 7 work is trajectory **1c612d15**, a Texture lifecycle trajectory created 0.35 s later. It did not hang. It was **legitimately blocked by a source-lineage mismatch** reported by management, and it is waiting on the owner.

Two separate problems come out of this:

1. **P-A (misleading status, no behavior harm):** run-root trajectories are created `live` with a rule that can never validate, are never settled, and `choir trajectory <id>` reports them as "invalid settlement rule". 33 of the 50 trajectories listed on b@c are in this state, some from 11 hours ago. Cosmetic/hygiene, but it sent this investigation down the wrong path.
2. **P-B (the real round 7 blocker, mechanism NOT proven):** after round 6 was approved and applied, management refused to delegate engineering because "the approved round 6 stack is based on 9771c768... while this computer booted 3217a2f7... and does not contain the stack patch". This may relate to the plain-release apply not carrying the source tree into the booted base. This needs its own investigation.

## Evidence

### 1. Where the string comes from

`internal/agentcore/trajectory.go`, `EvaluateTrajectorySettlement` (line 142-144):

```go
if len(rec.SettlementRule.RequiredSubjectRefs) == 0 {
    return false, []string{"invalid settlement rule: required subject refs are required"}
}
```

It runs only on the **non-lifecycle** path of `GET /api/trajectories/<id>` (`internal/agentcore/api_trajectory.go` ~line 150-170): the handler first tries `GetLifecycleSnapshot`; only on `ErrNotFound` does it fall back to `TrajectoryObligations`, which calls `EvaluateTrajectorySettlement`. Only run-root trajectories take that path.

### 2. What the rule's required refs are, by creator

| Creator | Rule | Result |
|---|---|---|
| `stampAndMintTrajectory` -> `defaultSettlementRuleForKind` (agentcore/trajectory.go:29-38) | publication kind: `publish_ref`, `edition_ref`; **every other kind: no required refs** | the "invalid" run-root records |
| Texture lifecycle create (`textureowner/texture_lifecycle_create.go:98`, `texture_handoff.go:237`, `coagent_route.go:220`, `store/story_open.go:127`) | `required_subject_refs: ["artifact"]` | the real work trajectories |
| `store/engineering_assignment_seed.go:60` | `["subject"]` | assignment seed |

The lifecycle validator `validateLifecycleSettlementRule` (store/lifecycle.go:1321-1338) rejects an empty `RequiredSubjectRefs` too. So the default run-root rule is rejected by both evaluators and could never settle even if something tried. Nothing does: the only writers of `TrajectorySettled` are the lifecycle settle path (store/lifecycle.go:5263) and graph_store; neither touches run-root trajectories. Run completion (agentcore/runtime.go:4066) only calls `SleepAgentMutationAfterTextureTurn`, not a settle.

### 3. Trajectory 23733daa (the "stuck" one)

`bc trajectory 23733daa-...`: kind `document`, status live, subject_refs `{channel_id, root_loop_id}` both equal to its own id, rule `{version durable-work/v1, require_no_open_work_items true}` with no required refs, created 15:23:30.085Z, never updated, `open_work_items: []`, `pending_updates: 0`, `waiting_on: ["invalid settlement rule: required subject refs are required"]`.

`bc run status 23733daa-...`: `state: completed`, `agent_profile: conductor`. Its result is `{"schema":"choir.durable_work.v1","action":"open_app","app":"texture",...}`: the conductor did its job (opened a Texture), then ended. The trajectory row is an artifact of that run, not a work item.

### 4. The real round 7 trajectory: 1c612d15-e941-5974-930c-a75c61101f1a

Lifecycle snapshot, kind `task`, subject_refs `{artifact: texture://documents/1ae2b734-..., doc_id: 1ae2b734-...}`, rule requires `artifact`, `lifecycle_version 4`, `reducer_seq 4`.
- Events: `trajectory_started` 15:23:30.43 (command_id `fp0-themes-r7-1793028209`, i.e. the prompt's idempotency key), then three `texture_turn_committed` at 15:23:53, 15:24:34, 15:24:52.
- Work item 07c10897 is still `open`, assigned to `texture:1ae2b734-...`.
- Document 1ae2b734 head revision (v3, 15:24:52, model gpt-6-luna): "Round 7 is blocked by a source-lineage mismatch ... approved round 6 stack is based on 9771c7680e..., but this computer booted 3217a2f7e4..., which lacks that patch. No source edits, build, render, freeze, or completion have occurred." Metadata carries `conductor_loop_id: 23733daa-...` (the link between the two trajectories).
- Management runs spawned by Texture: e360b912 (created 15:23:54, completed 15:24:20: "engineering delegation was refused because the approved round-6 stack is based on 9771c768... while this computer booted 3217a2f7..."); 8e3157a4 (created 15:24:35, completed 15:24:48: "do not retry implementation on the mismatched base; the owner must restore or provide the approved round-6 source lineage"). Their trajectories are the same run-root shape as 23733daa (channel_id `management:<owner>`), so they show the same `waiting_on`.
- No engineering assignment or capsule event exists for round 7 (compare round 6 below).

So the sequence is: prompt 15:23:30 -> conductor run -> Texture lifecycle trajectory -> Texture asks management at 15:23:54 -> management refuses at 15:24:20 -> Texture publishes the blocker doc 15:24:52. Everything that "updated around 15:23-15:24" is this chain.

### 5. Comparison with round 6 (normal prompt, 14:29)

- Conductor run-root trajectory `8092e6f6` (document, created 14:29:17.85, run completed 14:29:19): subject_refs `{channel_id, root_loop_id}`, no required refs, live, `waiting_on` = the **same string**, `settlement_ready false`. Identical to 23733daa.
- Real work trajectory `4fa3887f` (created 14:29:18.2, 0.34 s later): same shape as 1c612d15. Events: `trajectory_started`, turns, `co_super_assignment_opened`/`bound` 14:29:57-58, capsule `freeze_requested`/`frozen`/`revoked` 14:57:06-08, `co_super_assignment_reported` 14:57:08, more turns, and at **15:23:35.24** a `texture_turn_committed` plus `work_settled`; work item 9a01fab4 `completed`. The Texture revision at that point says the candidate selfdev-a681cea5 "has now applied it".
- So 4fa3887f's final update (15:23:35) is the round 6 close-out, 20 s after the apply landed (15:23:15) and 5 s after the round 7 prompt; it is unrelated to the round 7 trajectory beyond timing. Note its trajectory `status` is still `live` even after `work_settled`: the work item settled, the trajectory row did not (it has `lifecycle_state: null`). This may be worth a follow-up but is not the symptom here.
- So the "stuck" shape is **not new and not caused by the apply**; it is present in every round.

### 6. Prevalence

b@c `choir trajectories` returns 50 records. 33 have no `required_subject_refs`, all `status: live`, all `kind` task or document, subject_refs exactly `{channel_id, root_loop_id}`, spanning 05:19 to 16:33 today. Spot-checks: 7254d75f (management, 05:19) and 8092e6f6 (conductor, 14:29) have a **completed** run; 9142ab64 (management, 16:33) completed in 28 s. None ever left `live`. The listing does not show a single settled run-root trajectory. The 17 trajectories with `required_subject_refs: [artifact]` are the Texture work ones; several of those are also `live` and some `settled`.

d@e: **not checked.** `/tmp/choir trajectories --api-key-file d-e.key` requires a named computer id (owner-wide key), which I do not have, and the task authorized only the b@c function. Expectation (hypothesis): same, because the code path is not computer-specific.

### 7. Did the apply or restart cause it?

No evidence for it:
- Round 6 produced the identical record with no apply in flight.
- 23733daa was minted at 15:23:30.085 by the normal spawn path; the apply finished 15:23:15 (self-dev operation `updated_at 2026-10-26T15:23:15.619Z`, state `applied`, code_ref `code:sha256:4038c2cf...`). The lifecycle trajectory 1c612d15 started 0.35 s later and processed three turns without interruption (no crash-close visible).
- I could not read a restart marker or the boot log. Whether the apply restarted the runtime is **unknown**; `bc computer status` shows `state: active, realization_epoch 43`, and I have no earlier value to diff.

### 8. Does anything ever resolve such a trajectory?

No, from the code read: nothing settles, closes or garbage-collects run-root trajectories (see 2). They stay `live` indefinitely, and `choir trajectory <id>` will always report the same `waiting_on`. They are not wedged work (no work items, no pending updates), just never-closed registry rows. The only mutator for them I found is the owner `trajectory cancel` command (not run; it would be a mutation).

## Mechanism: proven vs hypothesis

Proven (code + records):
- The string's origin and the exact condition (empty `RequiredSubjectRefs`).
- Run-root trajectories are minted with an empty list for every kind except publication, and the evaluator hard-fails on that.
- 23733daa belongs to a completed conductor run; the actual round 7 work is 1c612d15, which is blocked on owner action by management's report, not by a runtime defect.
- Round 6's identical record shows the apply is not the trigger.

Hypotheses (not proven):
- H1: P-B arises because the plain frontend-only release applies served assets/code_ref but does not change the VM's booted source base, so the next engineering capsule's base (`3217a2f7...`) lacks the previous round's approved stack (`9771c768...`). The documents quoting these SHAs are model-authored; I did not independently verify either SHA, nor find the refusal site in the Go code (grep for the guard text turned up nothing quickly). Needs the management run's tool trace (`choir trace export e360b912-...`) and the engineering admission code.
- H2: the engineering refusal (not the trajectory string) is why "nothing progressed"; the owner would see the blocker doc in Texture, but the CLI view of the conductor trajectory suggests a hang.
- H3: `trajectory.status` stays `live` after `work_settled` (4fa3887f) because only an explicit settle transition changes it; unverified whether that is intended.

## Smallest fix options

For P-A (status noise; all are small):
1. **Mint run-root trajectories with a valid, honest rule or a distinct kind**: e.g. `kind: run` / settled-on-run-completion, or give them `required_subject_refs: ["root_loop_id"]` (which they always carry) so the evaluator passes once no work is open. Smallest: change `defaultSettlementRuleForKind` default to `RequiredSubjectRefs: []string{"root_loop_id"}` and settle the row when its run completes. Orange (runtime behavior, shared data model).
2. **Do not evaluate settlement for run-root rows**: in `TrajectoryObligations`, if the record has `root_loop_id` and no lifecycle snapshot, return a "run trajectory: <run state>" verdict instead of "invalid settlement rule". Green/orange, lowest risk, display only.
3. **Settle (or garbage-collect) run-root trajectories at run completion** (the substrate-level answer; it also removes 33+ permanently-live rows per day from `trajectories` listings). Note CLAUDE.md "Convergence Before Patching": the same trajectory registry is already being replaced by lifecycle trajectories for Texture work; check whether run-root minting (`stampAndMintTrajectory`) is superseded and can be deleted rather than patched ("prefer deletion").
4. Docs/CLI only: make `choir trajectory` print the run state next to the verdict so the next investigator is not misled.

For P-B: no fix proposed; investigate first. Smallest next step is read-only: export the e360b912 management trace and the engineering admission refusal, and compare the booted base ref before and after the plain-release apply on b@c (and check whether the release should rebase the engineering base, or whether Texture should refuse a "start from the running tree" request earlier with the correct reason).

## Heresy delta (this investigation)
- discovered: run-root trajectory rows are permanently `live` and report an invalid settlement rule (P-A); possibly `live` trajectory after `work_settled` (H3); round 7 blocker P-B (mechanism unproven).
- introduced: none. repaired: none.

## Not done / residual
- d@e not read (needs its computer id).
- No restart marker or boot log read; apply-restart relation unknown.
- No mutation attempted (no `trajectory cancel`, no self-dev action).
- Raw reads saved under /private/tmp/claude-501/-Users-wiz-Dev-go-choir/99fb2dd5-a4f7-4cc5-a0f5-6ee541f24543/scratchpad/ (traj.json, t_*.json, r_*.json, op.json).

## FP0 addendum, 2026-10-26 16:4x

- P-B is the SD-01 race FP0 owns until U4 lands: FP0 lands every applied change on main by hand before the next deploy. FP0 did not land themes round 6 (applied 15:23) before b@c's base moved; it was promoted verbatim at 16:38 as 670f9ea50 (patch sha256 056f181abb9470a5c50e…, matching the operation's source_patch_sha256). The booted base management named, 3217a2f7, is a docs-only main commit (15:06) while staging still ran 9771c7680, so the guest's "booted base" reads a main commit rather than the deployed build; that mechanism is not yet traced and stays a hypothesis here. U4 (lane u4-rebase, in review 321) and base-join (landed in train 17) are the structural answer.
- P-A is a separate, real defect: 33 of 50 listed b@c trajectories are run-root rows that stay live forever. Fix after this record, smallest option first (settle the run-root row when its run completes, or delete the mint if nothing reads it).
