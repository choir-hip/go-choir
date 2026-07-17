# G4 Fleet Cutover Blocker — First-Bootstrap Rollback

Observed 2026-07-17 while serializing the complete G4 per-route cutover packet from deployed staging inventory.

Mutation class: **red**. Protected surfaces: route-ledger authority, signed G3 execution envelopes, vmctl route CAS, and exact legacy ownership restore. This checkpoint records the problem before repair; no route or ownership was mutated while discovering it.

## Problem

Staging has 149 legacy ownerships with no ComputerVersion route. The deployed signed bootstrap path can advance an absent route to generation 1, but the route ledger exposes only `bootstrap`, `promote`, and `rollback` transitions. `rollback` requires a valid old and new ComputerVersion and a prior receipt whose `New` equals the requested rollback version. There is no signed CAS that can reverse a generation-1 bootstrap to route absence.

The deployed exact legacy detach/restore boundary correctly requires route absence before restore. Therefore, after the first bootstrap CAS, a failed product-path acceptance cannot restore the preserved legacy ownership. Deleting the route row manually would bypass signed authority, immutable receipts, stale-generation checks, and the single vmctl route-writer invariant. Leaving the new route active would not be rollback.

This is a **substrate** blocker, not a per-account symptom. It affects every one of the 149 legacy routes and prevents a truthful rollback-safe G4 packet.

## Code evidence

- `internal/routeledger/ledger.go`: transition kinds are only `bootstrap`, `promote`, and `rollback`; `TransitionCommand.Validate` requires every command to have a valid `New` ComputerVersion.
- `internal/routeledger/sql_ledger.go`: an absent slot accepts only bootstrap; every existing-slot transition updates the slot to `command.New`; no authorized transition deletes a slot.
- `internal/vmctl/bootstrap_candidate.go`: a frozen bootstrap candidate contains only the bootstrap plan.
- `internal/vmctl/promotion_authority.go`: bootstrap G3 acceptance explicitly requires an empty rollback-plan digest.
- `internal/vmctl/legacy_detach.go`: exact restore correctly refuses while the route exists.

No replacement unbootstrap/revoke implementation is present or merely unwired.

## Required repair contract

Add one typed **bootstrap rollback to absence** transition, not a generic delete:

1. It is valid only for a current generation-1 slot whose exact current ComputerVersion equals the frozen bootstrap result.
2. It binds the original bootstrap receipt and refuses any receipt from another slot/version or any receipt not committed at generation 1.
3. It is part of the frozen bootstrap candidate before G3 and its exact plan digest is signed by owner/G3 acceptance.
4. It runs through the same vmctl-only signed route CAS and transactionally appends an immutable transition receipt while deleting exactly one matching route slot.
5. Identical replay is idempotent even though the slot is absent; stale generation, substituted version, changed receipt, changed plan, unsigned acceptance, or a later promotion refuses without mutation.
6. Route resolution returns not found after the rollback. The preserved legacy detach receipt may then restore the exact old ownership.
7. Rollback order is bounded: stop the constructed candidate; execute signed bootstrap rollback; dispose the now-unrouted exact candidate; restore the exact legacy receipt. A crash leaves either the accepted new route or an absent route with durable receipts—never two route authorities.

The transition must not become a generic route deletion API and must not introduce another route writer or store.

## Evidence required before G4 resumes

- focused memory and SQL ledger contracts for exact generation-1 rollback-to-absence, idempotent replay, and stale/cross-slot refusal;
- frozen-candidate and signed-acceptance binding tests;
- vmctl HTTP/application lifecycle proof joining bootstrap receipt → rollback-to-absence receipt → route 404;
- deployed disposable legacy sequence: detach → construct/verify → signed bootstrap → signed rollback-to-absence → dispose candidate → exact legacy restore, including restart durability and unchanged preserved legacy state;
- refreshed complete fleet inventory and per-route rollback plans using only the accepted boundary.

Rollback for the repair itself: revert its source commit before executing any fleet bootstrap. If a disposable bootstrap was already rolled back to absence, restore the exact legacy receipt before reverting.

Heresy delta: `discovered`: missing rollback authority for first bootstrap; `introduced`: none; `repaired`: none at this checkpoint.
