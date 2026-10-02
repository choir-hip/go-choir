# S0m finding: desk runs submit but never dispatch after guest reboot

**Date observed**: 2026-10-02
**Computer**: `computer-03335285269bdba4f94377e56879f9e6` (owner guest, `candidate-fleet-e15cb89f25d963c220319b7b`)
**Deployed**: autoputer `65275e46`

## Symptom

`/api/prompt-bar` submissions mint trajectories and runs that stay
`state: pending` indefinitely — `running_runs: 0` on `/health` while
`desk_pending_mutations` climbs (observed 14 within ~10 min of boot).

Observed across three consecutive `s0m_escalate_acceptance_probe` runs
(v3, v4, v5): each submitted a trajectory successfully (prompt_bar_submit
leg OK, ~60–105 s) and then timed out `queued=false delivered=false` —
the desk cell's run record exists with correct metadata
(`deepseek-v4.1-flash high` under the new policy) but never entered
`running`.

## What is NOT the cause

- Model: v3 ran under `chatgpt/gpt-5.6-luna low`, v4/v5 under
  `opencode-go/deepseek-v4.1-flash high` — identical stall.
- Policy file: verified live via `GET /api/model-policy/resolve?role=texture`
  returning the post-edit selection.
- The prompt-bar submit leg itself completes (run + doc minted).

## Mechanism (confirmed by code read)

`Runtime.activate` (agentcore/runtime.go:294-305) dispatches `initial_dispatch`
synchronously and only logs on error — no durable retry. The sole rescue is
`armFreshMintManagementResumeWatchdog`, which `freshMintManagementResumeArming`
restricts to `isPersistentManagementAgentRun` (management_controller.go:612-616).
A texture-desk fresh mint whose `initial_dispatch` drops therefore strands in
`pending` with zero retry authority — matching observed v3/v4/v5 runs
(`1ef7bb18`, `ce6322a3`) pending indefinitely on `deepseek-v4.1-flash@high`.

## Handoff

Blocks the finish-acceptance chain (ask→report→resolve requires live desk
cells). Per S0m boundaries this is substrate-level (delivery path), not a
model-policy issue. Repaired 2026-10-02 via the second route — see below.

## Repair (landed)

The mint's `initial_dispatch` obligation is now derivable: `actorWakeOutboxFromObject`
gained an `ogKindRun` case — a run object projected `pending` at mint
(`UpdatedAt==CreatedAt`, no `actor_reactivated_from_passivated`) mints an
`initial_dispatch` outbox row to `run.AgentID` atomically in the same batch.
`projectLifecycleRun` and `CreateRunOG` gained the derivation call (they
bypass `commitLifecycleTransition`, where the loop used to live inline — now
extracted to `appendActorWakeOutboxes`), and `MigrateActorWakeOutbox` now
scans `ogKindRun`, so the 38 stranded pending runs re-drive on next guest
boot. `wake.AgentID=""` keeps `fromAgentID` identical to `rt.activate`'s
send, so a delivered dispatch dedups onto the same deterministic update_id;
a duplicate that does land no-ops past pending.

## Evidence

- `docs/evidence/s0m-rn3c-escalate-acceptance-2026-10-02.json` (v3: timeout)
- `docs/evidence/s0m-rn3c-escalate-acceptance-v4-2026-10-02.json` (v4: timeout, deepseek pending)
- `docs/evidence/s0m-ask-acceptance-2026-10-02.json` (ask probe: timeout)
- Guest health: `running_runs: 0`, `desk_pending_mutations: 14` at ~T+8min post-boot.
