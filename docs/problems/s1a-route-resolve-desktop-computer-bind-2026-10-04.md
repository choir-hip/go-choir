# S1a: guest route resolution 403s — the tap bind conflated the route slot's desktop with a computer ID

**Date:** 2026-10-04
**Status:** fixed (`69983b0e`), deployed acceptance pending.
**Mutation class of this record:** green (problem documentation).
**Subsystem:** `internal/vmctl/route_authority.go`, `internal/vmctl/handlers.go`.

## Finding

`HandleResolveComputerVersionRoute` (S1a guest-authority boundary) refused
every tap-sourced route-resolution call with

```
403 caller not bound to route slot computer: no ownership for computer primary
```

The route slot ID is `computer:<owner>:<desktop>` — its third segment is the
**desktop**, not a stable computer ID (`routeledger.RouteSlotID(ownerID,
desktopID)`; every caller passes a desktop). The S1a bind passed that segment
straight into `bindRequestToGuestComputer`, which resolves ownership via
`GetOwnershipByComputerID` — a lookup that can never match the literal
`"primary"`. Result: all guest `ResolveComputerVersionRoute` calls 403'd
since the S1a tap boundary landed.

This only surfaced during the S2 layered-update probe, the first mission to
exercise the guest→vmctl route-resolution path under the new boundary. The
broader platform-follow and owner self-development route *projection* pushes
(`self_development_route.go:125,167`) correctly bind `Projection.ComputerID`
(a real computer) — only the **resolve** GET conflated the slot field.

## Root cause

Two ownership-keying conventions live side by side and one bind path used the
wrong one:

- Route slots are keyed `owner:desktop` (vmctl `ownershipKey`, proxy
  `RouteSlotID(userID, desktopID)`).
- `bindRequestToGuestComputer` resolves by *computer* (`GetOwnershipByComputerID`).

Passing a slot's desktop segment into a computer-keyed bind is a
never-matches 403 — not a leak (fail-closed, correct posture), but a broken
product path.

## Fix (`69983b0e`)

Added `bindRequestToRouteSlot(r, ownerID, desktopID)`: resolves the
ownership by `GetOwnershipForDesktop(owner, desktop)` (the actual route-slot
keying) and still matches the caller's tap source IP against that
ownership's `computer_url` host. `route_authority.go` now binds
`slotOwnerID, slotDesktopID` from `ParseRouteSlotID`. The projection pushes
still bind `Projection.ComputerID` — unchanged and correct.

## Verification

`scripts/s2_layered_update_probe.mjs` — a tap-sourced guest resolves its own
route slot after platform-follow apply (previously 403). Also covered by any
guest kernel-capability route read.

## Lesson (ledger-first)

A behavior claim made without the ledger is a hypothesis. The probe's push
error named the exact bind (`caller not bound to route slot computer: no
ownership for computer primary`) — the desktop literal "primary" in the
message was the tell that the slot segment was being mis-keyed. Reading the
slot's construction (`RouteSlotID(owner,desktop)`) before patching the bind
is what made the fix a one-line keying correction instead of a guessed
ownership fallback.
