# G4 Fleet Cutover Blocker — Legacy Ownership Detach Boundary

**Observed:** 2026-07-17 on deployed staging Node B at source `main@92ed135389d363c774864c6d74f7fde711bc0a5b`.

## Mutation and authority

- Mutation class: **red**.
- Substrate: vmctl ownership lifecycle and D-ROUTE bootstrap ordering.
- Protected surfaces: existing user/platform computers, ownership persistence, Firecracker state, ComputerVersion verification, route bootstrap CAS, rollback.
- Governing gate: `G4-frozen-deployed-cutover-packet`; no fleet route CAS is authorized.
- Heresy delta: `discovered: 1` — legacy ownerships cannot enter the audited constructor without either unsupported host mutation or destructive deletion; `introduced: 0`; `repaired: 0`.

## Complete observed inventory

Durable normalized inventory: `docs/evidence/g4-fleet-inventory-2026-07-17.json`.

Source receipts and hashes:

- vmctl list: `/tmp/choir-g4-inventory/choir-g4-vmctl-list.json`, SHA-256 `5c9af115b4fbe81785e8aff69ccb98f856debd4189cc7a5495357b6965733a09`;
- persisted ownership registry: `/tmp/choir-g4-inventory/choir-g4-ownerships-persisted.json`, SHA-256 `8d61a8647490b70d212c289ec151ab3e6749248bed68bd77d8c890b37b90f77f`;
- auth users: `/tmp/choir-g4-inventory/choir-g4-auth-users.json`, SHA-256 `40c23ed36e94eb08597f0eb40620f49c64040b3358e2ba79efa13c6b5c213658`;
- route slots: `/tmp/choir-g4-inventory/choir-g4-route-slots.tsv`, SHA-256 `effc5350d93c75029e4441cae7f91acf0cbfe8ef93910f66bb114f9c329307c8`;
- route receipts: `/tmp/choir-g4-inventory/choir-g4-route-receipts.tsv`, SHA-256 `181879c7eb26d55f6608ddc3dec5dae63fe9382b44133ca6705e9e2fdcf01dd9`;
- storage proof: `/tmp/choir-g4-inventory/node-b-storage-report.json`, SHA-256 `8d1a30763d1b03379186e9e6a91e0e3115fc8a6de8eae12867215cea01276a0a`;
- VM cleanup gates: `/tmp/choir-g4-inventory/node-b-vm-cleanup-gates.json`, SHA-256 `37beba068c2144dfc9d71571ef12044e4dbcf5cd23e94ba01704940e559cf9c7`.

Observed totals:

- 759 auth records;
- 150 persisted computers and 150 matching state directories;
- lifecycle states: 147 hibernated, 1 stopped, 1 active, 1 failed;
- classifications: 134 missing-auth-record computers, 3 protected owner/control computers, 3 registered non-protected computers, 2 registered ephemeral-test computers, 5 synthetic non-UUID computers, and 3 platform computers;
- exactly one route slot, the accepted synthetic control route at generation 3;
- exactly one ownership already carries committed ComputerVersion and disk-receipt bindings (`candidate-control-20260717-j`);
- 149 legacy ownerships lack `construction_version`, `construction_disk`, and any D-ROUTE slot.

No inventory row authorizes deletion. Existing policy explicitly refuses protected, platform, registered, and missing-auth records; test/orphan deletion is outside this mission except a separately reviewed action required to admit construction.

## Blocking dependency cycle

For each of the 149 legacy owner/desktop keys:

1. The production constructor correctly refuses a candidate while the same owner/desktop ownership exists.
2. Signed bootstrap preparation correctly requires an independently verified construction whose identity exactly matches the target route owner/desktop.
3. The existing `/internal/vmctl/remove` lifecycle endpoint correctly requires an existing ComputerVersion route, and terminal removal destroys VM state.
4. The route cannot be bootstrapped before the matching construction exists.

Therefore:

```text
legacy ownership exists
  -> matching construction refused
  -> signed route bootstrap cannot be frozen
  -> route-required removal unavailable
  -> legacy ownership still exists
```

Manual edits to `ownerships.json`, direct state-directory moves, deletion, or an unsigned route bootstrap would bypass the settled authorities and are forbidden. Patching the old constructor or weakening identity checks would turn the safety refusal into split brain.

## Required source repair before G4 can freeze

Add one vmctl-owned, typed **legacy detach/restore** lifecycle boundary; do not add a second route writer or state store.

Detach must:

- accept exact owner ID, desktop ID, VM ID, lifecycle state/epoch, and a content digest of the frozen inventory row;
- refuse any ownership already carrying ComputerVersion or disk-receipt bindings;
- require no route slot yet exists;
- stop a running/failed legacy VM through the VM manager when necessary;
- atomically remove only the ownership key from the persisted registry while preserving the old state directory byte-for-byte under its existing VM ID;
- emit a signed/hash-addressed receipt containing the complete restorable ownership metadata, state-directory identity, and precondition digest;
- be idempotent only for the identical receipt and refuse stale VM/state/epoch or a changed route.

Restore must:

- require the exact detach receipt, absence of a replacement ownership, and absence of a committed route;
- reinsert the preserved legacy ownership and persist it without copying or mutating its `data.img`;
- refuse after successful route bootstrap except through a separately authorized ComputerVersion rollback.

The serialized cutover then becomes:

```text
freeze exact inventory row
-> detach legacy ownership, preserve old realization
-> construct matching candidate through production materializer
-> independently verify and prepare signed bootstrap
-> product-read candidate
-> bootstrap D-ROUTE
-> product-read routed candidate
-> retain detached legacy realization for bounded rollback TTL
```

On any pre-bootstrap failure: dispose the failed candidate if present, restore the detached legacy ownership from its exact receipt, verify persistence/readback, and stop fleet execution. On successful bootstrap, no legacy reattach is permitted; rollback is only to an accepted ComputerVersion receipt.

## Rollback and admissible proof

Rollback for the repair is removal of the new endpoint before any detach receipt exists. After a detach, rollback is exact restore from that receipt. G4 remains blocked until focused persistence/restart/stale-binding/route-conflict tests pass on canonical main, the deployed Node B endpoint proves detach → failed-candidate restore on a disposable legacy fixture, and the complete inventory plus serialized per-route plans are frozen for independent review.
