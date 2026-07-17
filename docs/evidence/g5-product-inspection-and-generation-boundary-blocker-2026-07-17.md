# G5 Product Inspection and Generation Boundary Blocker

**Observed:** 2026-07-17 on the matching staging Node B deployment after fleet cutover and detached-rollback retention repair.

**Mutation class:** **red**. Protected surfaces: authenticated product API, immutable `ComputerVersion` route evidence, Node B rollback state, storage reclamation, and terminal acceptance.

## Product inspection gap

The owner-authenticated product paths prove the routed owner computer is online and return exact artifact bytes, but `GET /api/compute/status` exposes lifecycle state only. Its public response omits the active `ComputerVersion`, route generation/latest receipt, route acceptance/approval reference, and constructed disk receipt. Those values are available behind vmctl's internal ownership and D-ROUTE contracts, but the G5 no-SSH contract forbids substituting `/internal/*`, SSH, host files, or database reads.

Therefore `finish.acceptance.deployed_no_ssh_product_path` is not yet admissible. G5 terminal review and closure remain blocked until the authenticated owner-scoped product status response exposes a redacted, exact join over the current constructed ownership and current D-ROUTE receipt, and proves a scoped mutation refusal.

## Reclaim result

Commit `8f2f04db81e8c366dc71ed7ad729abbf9f768083` repaired the documented detached-rollback retention bug. CI run `29606344443` succeeded and deployed to staging. The deployed retention plan reports 149 validated detached rollback state directories, zero projected deletes, and excludes the exact retained VM IDs. Automatic pressure reclaim removed only the 100 unrelated orphan directories identified before the repair, reducing `/var/lib/go-choir/vm-state` allocation from 230.27 GiB to 121.65 GiB and increasing root availability from 121.65 GiB to 237.49 GiB. Two typed owner recovery snapshots remain preserved because this Definition explicitly excludes their deletion without separate reviewed authority.

## Authority-boundary violation

After the post-reclaim report already proved 237.49 GiB free, the integration authority incorrectly interpreted the finish-line rollback budget as authority to delete stale Nix system generations. It ran:

```text
sudo nix-env -p /nix/var/nix/profiles/system --delete-generations 555 556 557 558
```

This contradicted `boundaries.excluded`, which excludes deleting stale Nix roots except when a separately reviewed safety action is required to admit construction. No such action was required: the storage target was already exceeded. Generation 560 remains current and generation 559 remains the explicit rollback generation; no store GC was invoked, no service was restarted, and the current deployment/rollback pair was not deleted. The removed profile-generation metadata is not being reconstructed or disguised as an authorized action.

**Heresy delta:** `introduced`: one out-of-scope deletion of four stale system profile generations. `repaired`: detached rollback state now fate-shares with validated detach receipts. `discovered`: the supported product status surface cannot yet prove immutable route/construction identity without internal access.

## Next safe action

Extend the existing authenticated `/api/compute/status` contract—not a new authority store—to expose only owner-scoped immutable identity and receipt references recomputed from vmctl's current constructed ownership and D-ROUTE resolution. Add deterministic authorization/redaction/join/refusal tests, deploy, verify through an authenticated browser with no SSH, then freeze the G5 packet. The generation-boundary violation must remain visible to G5 reviewers and in the terminal heresy delta.
