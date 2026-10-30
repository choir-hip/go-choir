# Recovery replay guest I/O ceiling — structural assessment 2026-08-24

## Symptom cluster (five observations, one substrate)

1. Boot #1 (epoch 710): replay 83k→99,765 at ~82 ev/s, then SILENT (no advance
   2m; no errors/fatal; health still 503) → stall-gate fail at 02:48:33.
2. Boot #2 (epoch 711): deterministic kernel-OOM crash loop ~20s ("running
   dolt gc" from the startup milestone GC at 7 GiB disk used) — fixed with
   B11 env + size guard.
3. Boot #3 (epoch 712): with the GC disabled, replay resumed 99.6k→103,148
   then stalled (same signature as #1).
4. Boot #5 (epoch 714, stall timeout 300s): resumed at 107,372; instrumentation
   shows **every event apply = 3.2-6.5s** (0.2-0.3 ev/s) at seq 107,877+
   (logs: "replay apply slow seq=... elapsed=4.1s" per event). Boot #4
   (between #3 and #5) stalled at seq 105,196.
5. (Unrelated but adjacent) the other active fleet computer
   (`candidate-fleet-d03dacaa...`, running since ~21:24) has looped for ~7h
   emitting "runtime: mint trajectory ... invalid genesis" every ~3s — the
   store append projection batch refuses "invalid computer event transition:
   invalid genesis" — the same projection-substrate family, separate computer.

## Payload/GC exclusions

- Platform serves the stalled pages instantly (99,701..100,000: 300 events,
  1.25 MiB in 3s; payloads for 103,148 and 107,877+ are empty/47B).
- GC is OFF in the guest since boot #3 (RUNTIME_DOLT_GC_DISABLED=1 + size
  guard) — the stalls persist without any GC.
- Checkpoint policy 500 events/60s (B10-conformant).

## Root cause (assessment)

The replay apply's per-event storage write cost scales with the workspace
size on the GUEST: the 4 GiB machine with a 7+ GiB dirty working set (the Dolt
chunk store vvvv… 5.7 GiB + journal 109 MiB) thrashes the guest page cache /
writeback — each per-event transaction flush takes 3-6s. At <100k (smaller
workspace) the same apply ran at 82 ev/s. The host-side equivalent is proven:
node-b's direct-file run of the SAME binary against a COPY of the same store
replayed 60k→132k constant at 27-82 ev/s and FINISHED (health 200, runtime
wired) in the A2/A3 experiment.

So: the replay code + the store + the tape are fine; the GUEST's memory/IO
budget (4096 MiB, virtio-blk over the btrfs-backed 32 GiB disk file) is the
ceiling. Without a substrate change the guest needs 24,600 events × 3-6s =
20-40 hours (per-quantum 30m, resumable, but effectively stalled).

## Options

A. Host-side replay driver (A2-proven path): run the same runtime against the
   RETAINED data.img on the host until head 132,539 (+ witness), then BootVM
   the guest (fast resume — local==platform head → replay no-op → head+witness
   verify → runtime up → route CAS). Conflicting with the definition's letter
   ("guest resumable replay converges") — could be codified as the B14
   host-drive revision. ~30-40 min.
B. Raise the guest machine size 4096 MiB → 8192/12288 MiB (vmmanager
   interactiveVMMemSizeMib): eases writeback thrash; expected per-event cost
   0.5-1.5s → recovery ~3-8h (1-2 sessions of 30m quantum… still slow, still
   <phase-0's 3.8 ev/s estimate only marginally better).
C. Dolt per-event nonfsync: remove the per-event fsync; durability stays at
   the checkpoint commits (B7/B10 contract already). Per-event cost would
   drop to write-speed (~10-30ms) → recovery ~10-30 min. Requires identifying
   the embedded-dolt toggle (DSN/repo-config/session var) and a runtime
   change — the knob was not found in a 20-min search; risk of dolt-
   internals work.
D. Appender batching: per-page transaction (batch the per-event finalize
   writes, commit at the checkpoint) — a store-write change in the appender
   (medium risk, no dolt internals) — the writeback pressure drops by
   batching — expected similar to C.

## Recommendation

C first (or D if C's knob is a dead end) — the durability contract already
says checkpoints; the fsync per event is a pure cost. Fall back to A if the
owner prefers the proven host path. B keeps the guest path but at hours of
wall time. The d03dacaa "invalid genesis" loop is a separate finding (same
projection substrate on a live computer — resolve after the recovery; it is
fleet-health-relevant, not recovery-blocking).
