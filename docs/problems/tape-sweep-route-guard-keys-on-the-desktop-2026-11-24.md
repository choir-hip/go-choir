# The tape sweep's route-slot guard keys on the desktop id, so it never refuses (2026-11-24)

**Problem.** `tapeRoutedComputers` (`internal/platform/tape_sweep.go`) reads each
`computer_version_route_slots.route_slot_id`, assumes the slot is
`computer:<owner>:<computer>` and records its third segment as a computer id;
`tapeDestroyedProofs` then refuses a computer whose id is in that set ("has route
slot"). The slot id is made by `routeledger.RouteSlotID(owner, desktop)` at every
caller (`internal/proxy/computer_lifecycle.go:535`, `agentcore/*`), and
`internal/vmctl/guest_reach.go` says its third segment "is the desktop, not a
stable computer ID". So the set holds desktop ids (`primary`), never
`computer-<hex>`, and the refusal is dead.

**Evidence.** Two review seats of lane `account-delete` read it independently
(2026-11-24); the callers above.

**Belief.** The guard has never refused anything. Pre-existing and not caused by
the account deletion: the scheduled sweep's "destroyed" proof leans on the
registry list and the ownership lookup, which do work. For an account purge the
route-slot rows hold version ids and no content, and the purge proves a computer
destroyed by its registry and ownership absence.

**Remaining error.** Whether a route slot should hold a computer back from the
destroyed sweep is a design question (a slot with no owner computer is leftover);
not decided here. Fixing the key needs the owner and desktop to computer mapping,
which is gone once the ownership is. Left for the lead; the change entry and the
data inventory no longer say a route slot is refused.
