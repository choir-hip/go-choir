# `choir.Assign` is a dead verb (2026-10-15)

Mutation class: green (this note); the deletion that follows is yellow.

## Problem

`choir.Assign` (`ChoirScope.Assign` in `internal/yaegikernel/choir.go`) and
its broker action `ActionAssign` do nothing. The handler (`handleAssign` in
`internal/yaegikernel/broker.go`) writes the payload into an in-process map
(`Broker.assignments`) inside the worker and returns `Status: "dispatched"`.
No assignment is created, no engineering capsule is opened, nothing is
persisted, and no tray intent exists for it.

## Evidence

- `docs/reports/master-report-protocol-research-autopaper-2026-10-11.md`
  (about line 81): Assign only writes into a map inside the worker and returns
  "dispatched"; nothing else happens. Recommended: delete.
- `docs/archive/protocol-review-assign-and-engineering-routing-2026-10-11.md`:
  the trace audit counts `choir.Assign` at 0 uses.
- `docs/reports/dead-code-census-2026-10-12/desk-config.md` section 1.4: the
  deletion checklist (parity and broker test lines included).
- No caller: grep across `internal`, `cmd`, `nix` and the runtime prompts finds
  no use besides the definition, the broker, and tests that exercise it.
- Not taught: `internal/yaegikernel/surface.go` no longer lists it, and the
  engineering overlay no longer names it.
- Engineering work opens only through a management Cast
  (`openDelegatedCastAssignment`).

## Decision

The owner approved full deletion on 2026-10-15. Scope: `ChoirScope.Assign`,
`ActionAssign`, `AssignPayload`, `AssignResult`, `handleAssign`, the
`Broker.assignments` map and its `assign/` receipts in `DrainReceipts`, and
the tests that exercise them.

Out of scope (they only share the word): `CancelAssignment`, the
`EngineeringAssignment` store and types, engineering assignments generally,
and `modelpolicy.Assign`.

## Rollback

Git revert of the deletion commit.
