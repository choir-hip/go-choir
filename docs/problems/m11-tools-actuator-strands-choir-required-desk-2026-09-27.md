# M11 blocker: fresh computer boots actuator=tools; engineering desk is handed a choir-less interpreter and cannot ever complete

**Status:** open, blocking M11 episode leg `primary_started -> awaiting_approval`.
**First observed:** 2026-09-27, staging `b0a21b18`, fresh-owner probe run
(op `selfdev-814208951f89a61d0a4c4e4c98c683b0`, computer
`computer-4b20e20e19a721a2c2f698c045e4821d`, VM
`vm-b568ff8cd85b27dd7c82079b1adbbad1`).

## Evidence

Run `run:assignment-ecfa18d0-4aff-52c9-88e9-0551fad6886f` (queried from the
guest store via `GET http://<guest>:8085/internal/runtime/runs/{id}/events`
with `X-Internal-Caller: true`): 187 events, 48 loop iterations, terminal
`loop.cancelled`:

```
tool loop: completion guard "" was not satisfied after 2 retries
```

Every `capsule_go_eval` cell that touched the choir namespace failed at
compile time — the model tried `import "choir"`, `import "go-choir/choir"`,
bare `freeze()`, `choir.Complete(...)`, `choir.Freeze(...)`:

```
{"error":"6:14: undefined: choir"}     (package main + import "choir")
{"error":"1:28: undefined: choir"}     (bare choir.Complete(execution_refs))
{"error":"14:2: undefined: choir"}     (final terminal-call attempt)
```

The model correctly diagnosed the substrate in its own response text:
"no self-development bundle can be bound in this actuator ... the `choir`
namespace is not reachable" — and spent 40+ iterations producing a
dedicated, citable refusal of the terminal call. Model discipline was not
the failure; the interpreter literally had no choir scope.

## Diagnosed cause

`cmd/capsule-broker/main.go` `handleGoEval` dispatches on the broker's
boot-time actuator:

```go
if b.actuator == actuatorRLM {
    if !b.sessionWorkerReady { return session_unavailable }
    return b.handleGoEvalSession(...)   // persistent worker, ChoirExports bound
}
return b.handleGoEvalOneShot(...)       // NewEvaluator(allowlist, nil): no choir
```

`b.actuator` resolves from `choir.actuator=` on the guest kernel cmdline,
which `vmmanager` renders from `VMConfig.Actuator`, sourced from the
ownership record (`ownerships.json` `actuator` field). A fresh owner/
computer carries **no actuator write** — `ParseActuator("")` fails closed
to `tools`, so the capsule broker ran `tools` and every `capsule_go_eval`
cell executed on the choir-less one-shot interpreter.

Meanwhile the host runtime composes the engineering overlay and the
completion guard unconditionally: the guard demands `choir.Complete` inside
a cell — a verb that cannot exist on the tools route. The failure is
structural, not stochastic: any engineering assignment on a
`actuator=tools` computer strands by construction.

Only the long-lived operator computer (`computer-0333528...`) carries
`"actuator": "rlm"` in `ownerships.json` — set by hand through
`POST /internal/vmctl/refresh {actuator}` (the same operator-write pattern
as the Source seeding defect in
`docs/problems/m11-engineering-desk-deferral-2026-09-27.md`: productized
surfaces get exercised only on the machine the operator hand-tuned).

## Host-side incoherence to fix (options, not yet decided)

1. `internal/capsule/roles.go EffectiveActuator()` — the overlay/schema
   builder derives the model-facing surface from the *host* env/cmdline
   read, while dispatch lives on the *guest's* ownership-driven cmdline.
   On a `tools` guest the host still emits `capsule_go_eval` +
   the choir.Complete guard. Either the host should refuse engineering
   assignments when the guest route is tools (fail at `OpenEngineeringAssignment`,
   not after 46 cells), or the engineering profile should not compose on
   tools guests at all.
2. Default `actuator` for new interactive ownerships — if the in-cell
   carrier is the product path (R-sequence ratified), `tools` on a fresh
   computer is a legacy-only route; consider making RLM the default for
   computers that will run engineering desks, or gating selfdev operation
   admission on `actuator == rlm` at the API boundary with a typed refusal.

## Probe-side mitigation (landed)

`scripts/m11_selfdev_episode_probe.mjs` now arms `actuator=rlm` through the
owner-facing `POST /api/computers/{id}/lifecycle/refresh {actuator:"rlm"}`
before opening the operation (commits `84247adf`, `4f861a3a`). Verified:
`fc-config.json` boot args carry `choir.actuator=rlm` after the refresh.

## What proves closure

`m11_selfdev_episode_probe.mjs` reaches `awaiting_approval` on a fresh
computer with the arm leg, and the desk run's `tool.result` events show
cells executing with `choir` bound (no `undefined: choir` errors).
Substrate closure additionally requires the host-side incoherence above
to be resolved (typed refusal or route-aware guard).

## Recovery posture

- Failed computers are disposable; no rollback needed.
- Until the substrate fix lands, engineering desks on `tools` computers
  strand at `executing` — the dispatcher is bounded but burns cells.
