# Effects-Red: Retained Computer Lifecycle Start Does Not Complete

<problem_id: retained-computer-lifecycle-start-timeout-2026-08-26>
<first_observed: 2026-08-26>
<mutation_class: red>
<computer_id: computer-03335285269bdba4f94377e56879f9e6>
<deployed_commit: ae12f82d>

## Problem

Attempting to start the retained computer (computer-03335285269bdba4f94377e56879f9e6)
via the product path so a sealed CoSuper activation could run fails to complete.
The computer is `state: stopped` (realization_epoch 794) both before and after two
start attempts; no partial lifecycle state was reached. This blocks the kernel
mission's deployed-sealed-CoSuper-activation proof, cross-model forced-death,
supervision/Texture transclusion, and Researcher conformance probes, all of which
require a running computer.

## Evidence (live product-path probes, 2026-08-26)

- `choir computer status --computer ...` → realization_epoch 794, state stopped.
- `choir computer start --computer ... --idempotency-key ...` →
  1st attempt: `http 502 {"error":"lifecycle resulting state was not observed"}`
  2nd attempt: `POST .../lifecycle/start: context deadline exceeded (Client.Timeout
   exceeded while awaiting headers)` after 120s.
- `choir computer restart --computer ... --idempotency-key ...` →
  `POST .../lifecycle/restart: context deadline exceeded (Client.Timeout exceeded
   while awaiting headers)` after 120s; computer still stopped. BOTH start and
   restart lifecycle verbs fail for this computer.
- `choir.news/health` → vmctl_status ok, upstream vmctl, vmctl_routing enabled
  (so the lifecycle controller itself is healthy; the transition for this
  computer specifically does not complete).
- Post-attempt status → still stopped; no partial state, no epoch bump.

## Consequence

The retained computer cannot be started via the product path from this
workstation, so no live CoSuper activation (capsule_go_eval/capsule_exec) can be
exercised. This is distinct from the predecessor-goal.complete gate (also
unsatisfiable while the predecessor is superseded). The kernel wiring itself is
deployed and verified; only the live-runtime proof is blocked.

## Admissible evidence class

Product-path observation (owner-scoped lifecycle start + status). No SSH, no
direct VM mutation. This is a problem-documentation receipt; do not attempt a
source fix as the first move (the start path is a platform lifecycle surface).

## Next action

Diagnose why the lifecycle start does not reach a running state for this
computer (vmctl logs / autoputer VM boot for computer-03335285269bdba4f94377e56879f9e6).
Do not keep retrying start (two attempts already timed out; repeated start can
leave a partial lifecycle state). The owner may start the computer through the
product surface or open the live activation path.
