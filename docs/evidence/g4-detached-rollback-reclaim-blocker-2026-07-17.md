# G4 Detached Rollback Reclaim Blocker

**Observed:** 2026-07-17 on staging Node B after serialized fleet sequences 1–149 completed and vmctl restart succeeded.

**Mutation class:** **red**. Protected surfaces: detached legacy rollback state, ownership persistence, pressure reclaim, storage verification, immutable fleet routes, and rollback TTL.

## Problem

All 149 mutable fleet plans completed in frozen order. Each exact legacy ownership was detached under owner/G3 authority, a fresh sparse `ComputerVersion` realization was constructed and independently verified, signed bootstrap CAS published the exact ownership, and the resulting computer was hibernated. After a full vmctl restart, health reported 150 ownerships and all 149 route/ownership/version joins recomputed without mismatch.

The post-cutover storage proof then exited nonzero. It treats legacy VM directories retained by the 149 durable detach receipts as unowned orphan state because detached receipts are not represented in the ordinary ownership list consumed by storage/reclaim classification. The report shows:

```text
retention_mode: active
retention_projected_delete_count: 100
retention_projected_delete_bytes: 108.62 GiB
retention_reason: matching disposable VM state found
```

Many listed `orphan_state_dir` rows are exact legacy VM IDs intentionally retained for rollback-only TTL. For example, the former owner row `vm-d067e51c904a6fc6b7810ec7dee75ad1` and recovered owner row `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19` are no longer in `ownerships`, but their signed detach receipts remain in the same durable ownership JSON and are required by `restoreLegacyOwnershipExact`.

This is a substrate authority split. The registry knows the directories are rollback-authorized detached state; pressure reclaim and the storage verifier see only live ownerships and classify the same directories as orphan/disposable. Under present low pressure no deletion occurred, but the active reclaim plan can select them when pressure triggers. That violates G4 success retention and makes rollback durability depend on ambient disk pressure.

## Current containment

No deletion command was run. Node B has approximately 238 GiB free and pressure is false. vmctl remains healthy after restart; 149 constructed routes are live and the legacy directories remain present. Fleet execution must not enter G5 closure or dispose legacy data until reclaim and verification consume the same detached-receipt authority.

## Required repair

The ownership registry must expose a bounded, non-secret protected-state view derived from validated detached legacy receipts. vmctl pressure reclaim and `node-b-storage-proof`/its verifier must classify those exact VM IDs as detached rollback state, never orphan/disposable, for as long as the receipt remains registered. Exact restore deletes the receipt and returns the VM to ordinary ownership classification; explicit post-TTL disposal must delete both state and receipt through one audited lifecycle operation. A malformed, duplicate, or unregistered file must not gain protection merely by resembling a receipt.

Tests must prove:

1. a registered detached receipt excludes its VM directory from pressure-reclaim candidates;
2. an unrelated orphan remains eligible;
3. restore removes detached classification and returns the exact ownership; and
4. storage verification accepts a fully cut-over inventory only when every retained legacy directory is joined to a validated detach receipt.

**Rollback refs:** 149 per-route rollback envelopes under `/tmp/choir-g4-fleet-execution/*/rollback.json`; prior legacy receipts under `/tmp/choir-g4-fleet-execution/*/detach-receipt.json`; accepted deploy `627e74201aba346c92c71e8f1620c8bef7b82342`.

**Heresy delta:** discovered `1` (reclaim authority ignored durable detached rollback ownership); introduced `0`; repaired `0`.

**Conjecture delta:** serialized fleet construction and route cutover moved from accepted packet to deployed fact. Rollback-data fate-sharing under pressure moved from assumed to falsified.
