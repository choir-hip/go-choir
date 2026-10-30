# S0 Problem: Texture boot reconcile crash-loops autoputer on live trajectory with no open texture work

**Date:** 2026-10-02
**Discovered by:** S0a reality probes on `choir.news` (deployed `aa2bd814`, selfdev-refresh active computer)
**Symptom:** guest `go-choir-autoputer` service restart-loop every ~8-9 min; `running_runs` stuck at 0; health endpoint times out during restarts; my wake minted but no run ever reached `running`.

## Evidence

Guest journal (`journalctl -u go-choir-vmctl`), three consecutive identical boundaries:

```
2026/10/02 14:57:28 autoputer: runtime startup refused: actorruntime: reconcile Texture owner:
  reconcile subject 5bd6de97-…/computer-0333…/texture:f1d3764d-4658-573d-b42f-020d713573d8:
  start reconciled Texture revision: load lifecycle work for Texture revision:
  Texture lifecycle open work unavailable
2026/10/02 15:05:46  (same, verbatim)
2026/10/02 15:14:28  (same, verbatim)
```

Each cycle: passivate_interrupted_activations (~2m20s), `reconcile_terminal_run_outcomes`, dead-wake outbox drain, then reconcile hits `texture:f1d3764d` and `autoputer: runtime startup refused` kills the service. Health endpoint returns 502/503 during the window — this is why `running_runs` stayed 0 and every probe timed out.

## Root cause

`internal/textureowner/texture_controller.go` `reconcileAgentWakeLocked`:
- `textureLifecycleActivationEligible` (:701) gates on trajectory `Status == TrajectoryLive` and no cancellation intent.
- `submitTextureAgentRevisionRun` (`texture_agent_revision.go:418-424`) then *separately* requires an `Open` work item `AssignedAgentID == textureAgentID`, returning `errTextureLifecycleOpenWorkUnavailable` when none exists.
- The recovery path at the submit site (:913, before this fix) re-ran `textureLifecycleActivationEligible` — which still returns `true` for a live trajectory — and so propagated the error instead of swallowing it.
- `Start` (:313-316) calls `ReconcileActorWake` for every eligible texture subject and returns its error → `actorruntime: reconcile Texture owner` fails → `autoputer: runtime startup refused` → process exit → systemd restart → loop.

The two predicates disagree on a doc whose trajectory is `Live` but whose texture-agent work item is already closed/drained. `f1d3764d` is exactly that: its trajectory remains `Live` (eligible) while the open texture work item was consumed by an earlier run or never re-armed after the wake-storm.

## Why this is not the dispatch stall

Earlier S0a work already fixed the wake-minting gap (`initial_dispatch` occurrence on refresh) in `aa2bd814` / `b907e04b`. That repair is orthogonal: it corrects which wake a refresh mints. The crash loop is downstream — a reconcile of a doc whose lifecycle authority is stale. The guest was restart-looping *before* this doc could ever dispatch.

## Fix (next commit)

`reconcileAgentWakeLocked` error path now rechecks the *actual* condition `submit` uses — an `Open` work item assigned to `textureAgentID` — on a fresh `GetLifecycleSnapshot`, instead of re-checking trajectory eligibility. If no open texture-agent work exists, return `nil` (quiescent wake, boot proceeds). Only propagate when open work genuinely exists, which is a real contract violation worth failing on.

## Residual / follow-up

- A live trajectory with `ownerHeadPending` but no open texture work will now stall silently instead of crashing. If owner directives need to mint work items on demand, that's a separate contract change in `submitTextureAgentRevisionRun` / work-minting — flagged for the Texture-model swap mission.
- `f1d3764d`'s trajectory state should be inspected post-deploy: if the trajectory itself should have settled when its texture work closed, the settle path is the real gap.
