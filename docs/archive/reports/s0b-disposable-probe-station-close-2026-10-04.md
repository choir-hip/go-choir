# S0b Disposable-Computer Probe — Station Close Report

**Station:** S0 (reality + boot timeline), slice S0b
**Boundary:** `s0b-boundary-close-2026-10-04` on the S0 station file
**Date:** 2026-10-04
**Panel:** 8 `accept_with_edge` / 3 `send_back`
(`.agentic-consensus/s0b-boundary-panel-20261004`)

## What was measured

A disposable computer (`computer-ac1808b4f04fc749c0781083ee747403`,
`vm-e1b9f92d`) was created through the owner bootstrap path and probed
across ten surfaces. A second disposable
(`computer-a99366facf24b872703de326d3b33832`) carried the Go-effect
re-run after the first run's keydriver scope miss; a third
(`computer-efef241fa2a8652758e4105e89412a76`) carried two further
attempts. All three ops wedged `executing` — which is itself the
Go-effect finding (see below).

## Receipts (10)

| Probe | Verdict | Evidence |
|---|---|---|
| Disposable lifecycle | pass | `s0b-disposable-lifecycle-2026-10-04.json` |
| M9a signed push + restore | partial | `s0b-m9a-bundle-probe-2026-10-04.json` — applied + witnessed restore; route slot never promoted |
| Capsule health map | pass | `s0b-capsule-health-map-2026-10-04.json` |
| Go effect (selfdev) | wedge | `s0b-selfdev-executing-wedge-2026-10-04.json` |
| Egress lifecycle capsule | pass | `s0b-egress-lifecycle-capsule-2026-10-04.json` |
| Absent runtime dep | gap | `s0b-absent-runtime-dep-2026-10-04.json` |
| Absent snapshot surface | gap | `s0b-absent-snapshot-surface-2026-10-04.json` |
| Absent Nix DB | gap | `s0b-absent-nix-db-2026-10-04.json` |
| Absent build surface | gap | `s0b-absent-build-surface-2026-10-04.json` |
| Builder-substrate decision | decision | `s0b-builder-substrate-decision-2026-10-04.json` |

## What the probes found

**1. M9a route-projection owner-binding defect.** A signed push
applied and a witnessed restore ran, but the route slot was never
promoted: the guest's `desired_event_head` advanced while
`canonical_event_head` stayed at the pre-push digest. Root cause
source-traced to the route projection binding the push to the owner's
event chain, not the computer's — the push reaches the computer but the
route certificate binds the wrong chain.
`docs/problems/s0-m9a-route-projection-owner-binding-2026-10-04.md`.

**2. Selfdev ops-launch freeze-authority gap.** A `POST
/api/computers/{id}/self-development/operations` creates a document
lifecycle and a desk run with `trajectory_id` set, but no engineering
assignment (`selfdev_texture_join.go` stamps no `assignment_id`).
`OperationStore` is populated on exactly one path —
`assignedEngineeringCapsuleToolCtx` — which requires a bound
Engineering assignment. The ops launch therefore can never bind the
freeze authority: `choir.Freeze` inside `capsule_go_eval` returns
`"freeze intent without assignment authority"`, the refusal surfaces
only as a cell result, and the op sits `executing` with
`verifier_refs=[]` forever. Three ops on three disposables, three
identical wedges.
`docs/problems/s0-selfdev-executing-wedge-2026-10-04.md`.

**3. Builder-substrate decision.** Three candidate branches evaluated:

- *Scoped guest service* (build via an in-guest host endpoint): weakened —
  the only reachable in-guest endpoint is gateway maild, which does not
  run Nix.
- *Host-level builder service*: open — the host already runs corpusd,
  gateway, vmctl; a builder service on the host tap is the boring option.
- *Privileged builder capsule*: open pending the capsule-namespace probe —
  whether a capsule can be granted a writable store mount + nix daemon
  socket without crossing the guest boundary.

`docs/evidence/s0b-builder-substrate-decision-2026-10-04.json` records
the decision with named edges.

**4. Unreached surfaces (honest gaps).** The sealed guest has no
guest-observable capsule/desk projection surface, no Nix DB for
dependency inspection, no guest-side build surface, and no
snapshot/UFFD endpoint — all four are host-level surfaces and belong
to S1 (security floor authority) and S3 (fast resume), not guest
probes.

## Send-back adjudication

Three of eleven panelists sent back; all named the same two gaps:

1. *No builder-substrate select-or-falsify receipt* — closed post-panel
   by `s0b-builder-substrate-decision-2026-10-04.json` (selects two,
   falsifies one, names edges).
2. *Void Go-effect cell (keydriver scope miss)* — closed post-panel by
   the re-run on two further disposables; the wedge receipt documents
   the confirmed freeze-authority gap as the finding.

## Named edges into S2

1. Repair the selfdev ops-launch freeze-authority gap — launch Go
   effects through an engineering assignment, not a raw ops POST;
   surface in-cell refusals on `operation.terminal_error`.
2. Run the capsule-namespace probe before committing the
   privileged-builder-capsule substrate branch.
3. Snapshot/UFFD surface is host-level (S3), not a guest probe.

## Landing loop

- Pushed commits: `e87f3294` (S1a POST legs + flake fix),
  `d0b6a8d8`..`6c7ca90c` (S0 close + problem docs + mechanism
  corrections), `e6436134` (ACTIVE).
- CI: `37184188348` success on `e87f3294`.
- Staging: `https://choir.news` serving `e87f3294` post-deploy; the
  disposable probes ran on staging throughout.
- Acceptance level: `deployed` (disposable-computer probes on staging;
  Go-effect observation blocked on the named ops-launch defect).
- Mutation class: `green` (evidence + problem docs + station files; no
  runtime change).
- Protected surfaces touched: none (read-only probes + documented
  defects).
- Heresy delta: `discovered` — M9a owner-binding defect + ops-launch
  freeze-authority gap recorded; neither repaired.
- Rollback refs: disposables `computer-ac1808b4`, `-a99366fac`,
  `-efef241f` reaped or reaped on session end; platform restart at
  07:57 was unrelated infra (corpusd re-start, no code change).
