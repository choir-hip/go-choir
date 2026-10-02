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

## Suspect

Post-restart delivery path. `Runtime.Start` comment (agentcore/runtime.go:718-722)
notes boot-time rewarm/delivery sweeps are deleted — the actor-wake outbox
projector is the sole post-restart delivery path. If a run is minted while the
projector is not yet started, or the wake event isn't projected, the pending
run may never dispatch. This is consistent with a run landing during/just after
boot staying pending forever.

## Evidence

- `docs/evidence/s0m-rn3c-escalate-acceptance-2026-10-02.json` (v3: timeout)
- `docs/evidence/s0m-rn3c-escalate-acceptance-v4-2026-10-02.json` (v4: timeout, deepseek pending)
- `docs/evidence/s0m-ask-acceptance-2026-10-02.json` (ask probe: timeout)
- Guest health: `running_runs: 0`, `desk_pending_mutations: 14` at ~T+8min post-boot.

## Handoff

Blocks the finish-acceptance chain (ask→report→resolve requires live desk
cells). Per S0m boundaries this is substrate-level (delivery path), not a
model-policy issue. Next action: inspect the actor-wake outbox projector's
post-boot drain of pending runs — whether a pending run minted before
`startProjector` completes is ever picked up, and whether the desk cell's
wake event is projected at all when `state: pending`.
