# G4 First Fleet Canary Unpublished Route Blocker

**Observed:** 2026-07-17 on staging Node B during accepted G4 sequence 1.

**Mutation class:** **red**. Protected surfaces: constructed ownership state, signed bootstrap/rollback CAS, public proxy routing, authenticated owner product path, and exact legacy rollback.

## Problem

After the TAP allocator repair, the exact `a@b.com` canary constructed and booted successfully, independently verified, and committed its signed generation-1 bootstrap route. The owner authenticated through the normal `https://choir.news` passkey flow. Chrome then rendered `CHOIR BIOS — BOOTSTRAP FAILED (502)` while the public proxy logged the exact cause:

```text
desktop primary is not published
```

The routed candidate itself remained healthy (`GET http://10.200.1.2:8085/health` returned HTTP 200, `status: ready`) and vmctl recorded active ownership at that URL. The ownership was `published: false` because construction intentionally creates an unpublished candidate and no route-commit lifecycle step promoted that ownership to public product routing.

This is a fate-sharing defect between the SQL route authority and the ownership registry. A valid immutable route alone is insufficient: public proxy resolution also requires the exact constructed ownership to become published. The existing `PublishDesktop` product operation is not a replacement: it rejects `primary` because legacy primary desktops are assumed already published, and it is not bound atomically to a signed route transition.

## Receipts and containment

- fresh sparse disk receipt: `disk-instantiation:sha256:a00fe3ddc7d7e9418a33cdd32082f55d086dd3115cdebfd2d97b734f414c4531`;
- independent verification: `verification:sha256:60d40689ef99eb70e660641de4f6af8391e983a3ad3a25e1b6d9ffaf3af6cecf`;
- frozen bootstrap candidate: `route-bootstrap:sha256:4eb5c83f68c54c8b4b4293988bed076cb7786ce0aceda92cbb96da961956b875`;
- generation-1 bootstrap receipt: `c27002f6-4a08-5a7c-81ca-9c770a209b07` at `2026-07-17T16:27:32.252029033Z`;
- owner-authenticated Chrome observation: repeated public HTTP 502 with `VM route returned 502; retrying`;
- generation-2 rollback-to-absence receipt: `bce7a01a-cec0-4533-9e60-e3e04ce22b88` at `2026-07-17T16:31:26.871410599Z`.

Containment followed the pre-frozen contract: stop the routed candidate, signed rollback to route absence, exact stopped/unrouted disposal, exact detach-receipt legacy restore, invariant legacy `data.img` tuple, vmctl restart, and HTTP 404 route readback. No later fleet row moved.

## Required repair

The signed route transition and constructed-ownership publication must fate-share with exact identity/version/disk bindings and restart-safe idempotency. Bootstrap success must leave the routed primary ownership published. Bootstrap rollback must return it to a safely disposable unpublished state before exact disposal. Any registry persistence failure must not leave a route selecting an unpublished candidate. Tests must exercise the public proxy contract, rollback, replay, stale identity, persistence failure, and restart readback rather than merely asserting an internal boolean.

**Rollback ref:** `bce7a01a-cec0-4533-9e60-e3e04ce22b88` plus restored legacy detach receipt `legacy-detach:sha256:e162860f52283e1ca8ab9a1c9b7bf5ca3f62739feb98f320706b1d4b7c58c402`.

**Heresy delta:** discovered `1` (route authority and product-routability state were split without a fate-sharing transition); introduced `0`; repaired `0`.

**Conjecture delta:** owner authentication and browser automation moved from blocked to proved. Fleet cutover remains falsified until route CAS and ownership publication fate-share.
