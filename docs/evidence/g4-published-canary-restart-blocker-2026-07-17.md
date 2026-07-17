# G4 Published Canary Restart Blocker

**Observed:** 2026-07-17 on staging Node B after the route-publication repair deployed.

**Mutation class:** **red**. Protected surfaces: ownership persistence, detached legacy rollback receipts, constructed primary ownership, signed route state, vmctl restart durability, and public product routing.

## Problem

The repaired sequence-1 canary succeeded through the real product path: owner-authenticated Chrome loaded the online desktop, `/api/shell/bootstrap` returned HTTP 200 from `candidate-fleet-49ee3bd0ec6f366a164c02d2`, `/api/files` returned HTTP 200, and `audit.txt` contained `Choir audited control realization`. The generation-1 route and exact constructed ownership were both live and `published: true`.

The mandatory vmctl restart check then failed. vmctl repeatedly exited while loading the ownership registry:

```text
load legacy detach receipt legacy-detach:sha256:7c51827a9aaebd720d3940eade9be46aaf6fa84e7c2ebc1404b5ba9926679b8f: owner desktop is also registered
```

This is a persistence invariant mismatch. Successful audited cutover intentionally retains two records for one route slot during the bounded rollback TTL:

1. the exact current constructed ownership selected by the immutable route; and
2. the exact detached legacy receipt needed to restore the former realization after rollback-to-absence.

The live registry permits that state because construction occurs after legacy detach. `loadLocked` rejects it generically on restart, conflating an invalid duplicate live legacy owner with the required constructed-current plus detached-legacy rollback pair.

## Current containment

The candidate guest and generation-1 route were healthy before restart. The service restart left the Firecracker guest processes alive but vmctl unavailable. No later fleet row moved. The generation-1 route remains the last committed SQL route state, and the pre-frozen generation-2 rollback remains the recovery authority once vmctl can load the valid registry pair.

## Required repair

`loadLocked` must continue rejecting duplicate detach receipts, a detached VM ID registered live, and any legacy/non-constructed duplicate owner desktop. It may accept the owner/desktop collision only when the current row is a fully validated, committed `constructed-computer-version` ownership for the same route slot and a different VM ID. Restart must then reattach or safely stop that exact constructed realization without losing `Published`, construction version/disk receipts, or the detached legacy rollback receipt.

Tests must persist a constructed current ownership plus detached legacy receipt, reload it, prove both exact records survive, and still reject an ordinary duplicate legacy owner.

**Rollback refs:** frozen rollback candidate `route-bootstrap:sha256:40f5a9f1da1d08c4d982e998d5fbfb97d7a83efc0e7c1cd0a9132c8f3fc266d0`; the generation-1 bootstrap receipt remains in Node B route SQL; detached receipt named above remains in ownership JSON.

**Heresy delta:** discovered `1` (live ownership state accepted a rollback pair that restart validation rejected); introduced `0`; repaired `0`.

**Conjecture delta:** route/ownership publication moved to deployed public product proof. Restart durability of the retained legacy rollback pair moved from assumed to falsified.
