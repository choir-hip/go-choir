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


## Source repair candidate — deployment pending

Candidate Git binary-diff SHA-256: `f777e852c566791ff577daa3ae5534c0432f51208dea0af8f2b215fe80d14d4d`.

The candidate implements `bootstrap_rollback` as a typed, generation-1-only route transition. It requires identical old/new ComputerVersion bindings (the version is receipt/evidence identity, not a resulting route), expected generation 1, and the exact deterministic bootstrap receipt. Memory and SQL ledgers verify that the target receipt is the current slot's generation-1 bootstrap receipt, append a generation-2 rollback receipt, and delete exactly the matching slot. Replay returns the original receipt while the route remains absent; a new or substituted command against absence refuses.

A frozen bootstrap candidate now contains both bootstrap and rollback plans. The future bootstrap receipt ID is deterministically derived from the route slot and unique bootstrap idempotency key, so the rollback target is frozen before G3. Owner/G3 acceptance signs both plan digests. Each SQL authorization envelope fate-shares the executed plan with its companion; the SQL boundary independently verifies both signed digests, their kinds, route, evidence, and command validity rather than trusting vmctl to claim that a rollback exists.

The existing apply-bootstrap endpoint now requires explicit `action: bootstrap|rollback`. Bootstrap rollback verifies the exact current generation-1 bootstrap result, then uses the same signed evidence/CAS boundary. After commit, vmctl requires route resolution to return not found and returns the immutable rollback receipt with `route_absent: true`.

Focused proof:

- memory ledger: deterministic bootstrap receipt, cross-slot refusal without mutation, exact rollback-to-absence, receipt validation, absent-route refusal, and idempotent replay;
- SQL ledger: the same contracts plus persisted absence and replay after store restart;
- signed vmctl/HTTP path: frozen paired plans, owner/G3 signatures, bootstrap receipt join, stale bootstrap replay refusal, signed rollback-to-absence, signed evidence join, route-not-found, and idempotent HTTP replay;
- `go test ./internal/routeledger ./internal/vmctl -count=1`: pass;
- `go vet ./internal/routeledger ./internal/vmctl`: pass.

The repair is deployed and the full disposable lifecycle passed at `e6fa53f10db3ba9499175d7a1d7912a0cbe2f876`: signed detach → construct/independently verify → paired-plan G3 acceptance → generation-1 bootstrap → candidate health/readback → stop → signed generation-2 rollback-to-absence → idempotent replay → exact unrouted disposal → exact legacy restore → vmctl restart. Route absence and the legacy image metadata invariant held, and a forged rollback-plan digest returned HTTP 409 without registry mutation. See `docs/evidence/g4-bootstrap-rollback-deployed-proof-2026-07-17.md` and its packet SHA-256 `76c6521bba9df23683164d696f8577fb37ef0f0c6071e81c0e96fad37cec2f87`. The substrate blocker is repaired; G4 remains closed only for refreshed fleet freezing and independent adjudication.
