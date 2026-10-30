# Recovery Substrate Root Cause Clustering: Five Substrate Failures — 2026-08-23

- Date: 2026-08-23
- Mutation class: Red
- Parent: `docs/definitions/choir-durable-substrate-recovery-2026-08-23.md`
- Supporting Design: `docs/designs/choir-durable-substrate-2026-08-23.md`

## Problem Cluster

Over August 21–23, 2026, attempts to recover stopped staging computer `computer-03335285269bdba4f94377e56879f9e6` (epoch 361) failed due to a cluster of five interconnected substrate flaws:

1. **Unbounded Boot-Time Replay Loop ($O(\text{history})$ Replay Trap):**
   - `autoputer/run.go:225` executes `appender.Reconstruct` during guest initialization *before* opening HTTP `:8085`.
   - With 132,436 events accumulated in the canonical tape, executing sequential serializable transactions in Dolt takes multiple hours.
   - The Firecracker hypervisor health check (`vmmanager/manager.go:1883`) times out at 3 minutes (`connect: connection refused`) and terminates the VM on every boot cycle.

2. **Circular Guest-Dependent Restore Dependency:**
   - The restore endpoints (`POST /api/computers/{id}/lifecycle/rematerialize-from-tape` and `/restore`) were implemented as guest-authority endpoints.
   - The platform proxy (`internal/proxy/computer_lifecycle.go:311`) refused requests with `502 computer authority unavailable` when the VM was stopped.
   - A stopped computer could not be recovered because recovery required it to already be running.

3. **Forensic & Pre-Failure Image Loss via Aggressive Pruning:**
   - `internal/vmctl/cold_recover.go:543` called `storage.QuarantineDataImage(root, vmid, generation, operationID, 3)` with hardcoded `maxRetained=3`.
   - `internal/vmmanager/manager.go:2258` (`pruneCompletedQuarantines`) pruned oldest quarantine images first across repeated failed recovery attempts.
   - This permanently destroyed the pre-failure `data.img`, leaving only blank-seed partial images.

4. **Blank-Seed Cold Recovery from Zero:**
   - When host-orchestrated recovery (`choir-host-orchestrated-recovery-2026-08-22`) was attempted, `cold_recover.go` created a blank sparse image, copied only the privacy key, and attempted to boot fresh.
   - This re-triggered the 132k-event sequential replay from sequence 1, immediately hitting the same 3-minute boot timeout.

5. **Absence of a Materialized Differential Base:**
   - No platform-side projection snapshot publishing existed. Existing snapshot types (`DoltHeadSnapshot`, `ObjectGraphSnapshot`) were test fixtures only.
   - The platform held all 132,490 projection batch bodies (~756 MiB) on disk in `platform-artifacts/sha256/computer-event-payload/`, but had no offline or staged mechanism to pre-materialize state to head 132,436.

## Substrate Repair Direction

Per `AGENTS.md` Convergence Doctrine, these five symptoms trace to a single substrate requirement: **the platform must publish and consume typed, content-addressed `ProjectionBase` snapshot artifacts**.

The repair is divided into two sequential Definitions:
1. **Immediate Recovery (Rail A / `docs/definitions/choir-durable-substrate-recovery-2026-08-23.md`):**
   - Run an isolated, resumable offline rebuilder on the host to process all 132,436 events in batch.
   - Publish the first `ProjectionBase` blob artifact at head 132,436 into `platform-artifacts/sha256/projection-base/` with atomic `fsync`.
   - Hydrate the base into the guest via the host gateway, allowing the guest to boot with $\Delta = 0$ events replayed and open `:8085` in seconds.
   - Explicitly disable `maxRetained=3` pruning during recovery.
2. **Substrate Overhauls (`docs/definitions/choir-durable-substrate-overhauls-2026-08-23.md`):**
   - Track K: Key escrow with passkey PRF wrapping and host two-approval recovery.
   - Track F: Content-addressed file-CAS with `FileRootCommitted` Merkle roots and periodic `ProjectionBase` event watermarks.
   - Track M: Host MTA spool with guest Maildir format.
   - Assurance & Scale: Automated daily restore drills, continuous scrub, and recovery cells.

## State

Computer `computer-03335285269bdba4f94377e56879f9e6` remains stopped at epoch 361. The canonical chain is intact at head 132,436. All 132k projection batch bodies exist on disk. Effects remain OFF. Rollback: revert doc commit.
