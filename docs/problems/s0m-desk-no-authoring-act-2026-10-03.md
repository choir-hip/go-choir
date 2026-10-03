# S0m finding: texture desk runs but authors no act — the agency boundary

**Date observed**: 2026-10-03
**Computer**: `computer-03335285269bdba4f94377e56879f9e6` (owner guest,
`candidate-fleet-e15cb89f25d963c220319b7b`)
**Deployed**: autoputer `4dbb4a5a` (post stranded-bound-rebind repair)
**Mutation class**: green (observation/boundary documentation only)

## Symptom

The prompt-bar submit mints a live trajectory and the texture desk cell
**dispatches and runs** — `running_runs > 0`, a `texture_turn_committed`
lifecycle event commits — but the turn completes **with no authoring
act**: no `choir.Ask`, no `choir.Resolve`, no `choir.Note`, no
`choir.Cast`. The trajectory's `updates` stays empty; nothing binds a
research carrier; nothing resolves an ask.

Observed on trajectories `d3061b0b` (stranded-bound probe) and
`17b4883f`/`cdf50edc` (canary): desk runs, commits a turn, authors
nothing.

## What is repaired (this is NOT a delivery defect)

The full dispatch + delivery substrate is now landed and verified:

- **Boot-time initial_dispatch** — `ae47c8a4` actor-wake outbox `ogKindRun`
  case: a pending minted run owes its agent a durable activation wake.
- **Post-boot desk-mint starvation** — `ebfd2e98` retry on deadline-exceeded.
- **Restart-recast supersede** — `39e924aa` durable-invalid supersede
  incorporates.
- **Stranded-bound rebind** — `c31bf43a` (this deploy): pure unbind clears
  `DeliveredAt` so a freed packet re-enters the pending scan; union
  liveness oracle (`getRunForComputer`) stops freeing live passivated
  carriers; tri-state claim fate settles completed carriers;
  `replay.Completed` suppresses discharged-turn re-mints.

Result: desks that used to stay `pending` forever now execute. The wedge
moved from *delivery* to *agency*.

## The boundary

Two kinds of "no act":

1. **Model agency** (established, `s0m` now-card): the desk *has*
   `choir.Ask`/`choir.Resolve` but does not emit them after consuming —
   the escalate probes already hit this (`texture cell completed with no
   authoring act`). A prompting/steering limitation, not a missing
   mechanism.
2. **Act surface** (open question): is `choir.Ask` actually reachable as
   a callable verb in the texture desk's tool surface on `4dbb4a5a`, or
   is the cell completing because the verb it wants isn't presented?
   `choir.Resolve` is confirmed a live texture verb (`ChoirScope.Resolve`
   → `IntentResolve` → `CommitmentKindResolve`). `choir.Ask` needs the
   same reachability check.

## Why this is substrate-shaped, not just prompting

This is the third boundary in the same delivery/wake substrate in <24h
(dispatch-stall → stranded-rebind → no-authoring-act). Per *Convergence
Before Patching* / *Root Cause Clustering*, the next action is a
structural assessment, not another point patch: the desk-authoring act
surface may share a single authority gap with the delivery fixes already
landed — the cell completes "cleanly" (commits a turn) precisely because
no act was emitted, so there is no failure to retry against.

## Admissible evidence / next probe

- Whether `choir.Ask` is presented to the texture desk's tool surface on
  `4dbb4a5a` (reachability check).
- Whether an explicit `choir.Ask` in a desk prompt produces a bound
  control (separates "verb reachable" from "model didn't choose it").
- If reachable but unchosen: prompt/pack steering so the desk authors
  the act the trajectory needs — that is a `yellow` (prompt) change, not
  substrate.

## Boundary for S0m

Blocks the finish-acceptance chain (stranded-bound rebind re-proof needs
an authored Ask control to kill-and-rebind; mechanical-resolve needs Ask
then Resolve). All delivery invariants are now green; the residual is the
desk-authoring surface.
