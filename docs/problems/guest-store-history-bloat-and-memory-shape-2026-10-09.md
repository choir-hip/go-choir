# Guest projection store is 5x its live data; every guest gets 16 GiB to cope

Date: 2026-10-09. Owner direction: "investigate and fix this storage leak,
the gc policy, and shrink my guest vm ram to 4gb once we've done so."
Operational invariants O10 (declared bound per growth surface), O11
(memory budgets), O21 (a realization holds no unique state). Prior record:
[`guest-dolt-journal-and-host-image-leak-2026-10-03`](guest-dolt-journal-and-host-image-leak-2026-10-03.md)
(offline GC, store 27 → 6.7 GiB; durable fixes never landed) and
[`guest-vm-embedded-dolt-memory-starved-2026-10-01`](guest-vm-embedded-dolt-memory-starved-2026-10-01.md)
(the owner's slow Texture traced to the embedded store starving for
memory; "fixed" with more RAM). Mutation class of a fix: red (guest
persistent state, VM shape, projection bases).

## Evidence (read-only; every measurement on a reflink copy of the owner `data.img`, never the live device)

**Guest disk (12 GiB used of 32):**

| Path | Size |
|---|---|
| `state.texture/texture/.dolt/noms` | 8.9 GiB (`oldgen` 7.9, newgen 1.1, journal small) |
| `choir-updater` | 2.1 GiB (releases 873 MB, **`incoming` 720 MB**, store 552 MB) |
| `files` | 144 MB |

- The GC disposition reads `skipped_size, used 12 GiB, threshold 5`.
- The in-guest guard (`internal/store/dolt_maintenance.go`) measures
  whole-disk used minus journal, so updater files count as "live store".
- `dolt gc --full` never runs anywhere.

**What the store holds:**
- 1,197 commits, one branch; **1,151 of them on 2026-10-07** (the restore).
- Biggest tables:
  - `computer_event_index`: 577,544 rows;
  - `og_objects`: 205,966 rows, of which `choir.event` 165,146,
    lifecycle events and commands ~21.5k, runs 2.8k, wake outbox 2.6k;
  - `run_memory_entries`: 68,161 rows.
- Growth since the 2026-10-03 offline GC (6.7 GiB): about 2.2 GiB in 6
  days.

**Compaction measurement:**

| Step (on the copy) | `noms` size |
|---|---|
| As is | 8.9 GiB |
| History squashed to 2 commits, `dolt gc` | 8.1 GiB (plain GC keeps `oldgen`) |
| + `dolt gc --full` | **1.7 GiB** |

**Live data is 1.7 GiB; 81% of the store is history and old chunk
generations.**

**Nothing reads that history.** Texture history follows the revision parent
chain (`internal/store/texture.go:1346`); Dolt commits exist "for AS OF
addressability" (`internal/store/store.go:122`). One dependency:
checkpoints record `HASHOF('HEAD')` as a state witness
(`internal/computerversion/dolt_state_extractor.go`), so a compaction must
re-derive that witness.

**ProjectionBases copy the bloat.**
`platform-artifacts/sha256/projection-base` is 40 GB, 74 objects. The
owner's bases are 8.7 GB each (2026-10-07, 10-08, 10-09), plus a 15.9 GB
base from 2026-09-09. Artifact GC still runs dry-run only.

**Memory shape.**
- `/var/lib/go-choir/vmctl-priority.env`: hand-made on Node B
  2026-10-01, untracked, outside git. It sets
  `VM_INTERACTIVE_MEM_MIB=16384` for **every** interactive computer; the
  tracked unit declares 8192.
- 12 VMs run with 16 GiB, no balloon, on a 31 GB host. That includes
  tonight's QA signup.
- Host at 02:46: the owner VM at 14.7 GB RSS, swap 12 MB free.

**Host leaks still open from 2026-10-03:**
- ~11 loop devices attached to **deleted** `data.img` files, whose space
  is never freed;
- two 32 GB-apparent `data.img.quarantine-1-*` (2026-10-07) in the owner's
  VM directory;
- `console-b14-serve3.log` (104 MB, August).

During this investigation, the `dolt` CLI kept a copy's mount busy after
it exited, and the first `umount` failed. That is a likely source of the
stale loops: a mount helper that does not retry or detach.

## Problem

The guest projection store is a cache of the tape (O21), but it keeps an
unbounded Dolt history that nothing reads. The guards made things worse
in sequence:

1. The in-guest GC was made safe by skipping large stores.
2. Skipped GC let the store grow.
3. The growing store needed more RAM.
4. RAM was raised to 16 GiB, by an untracked host file, for every
   computer.
5. That exhausted host memory and slowed everything, including Texture.

Checkpoints copy the bloated store, so the host pays for it again.

## Fix direction (proposal; see the SL/SA ordering in the metamission)

1. **Bounded history by construction.** The projection store keeps
   current state only, plus the commits replay needs as resume points.
   Compaction (squash + `dolt gc --full`) runs **off the guest**, in the
   checkpoint worker, which has the memory. Projection bases become
   compact (~1.7 GB instead of 8.7).
2. **Guest reclaim = rebase onto a compact base.** When a guest's store
   exceeds its declared bound, the next boot re-materializes from the
   latest compact base plus tape tail; this is the O21 path. In-guest GC
   handles only the journal. The `HASHOF('HEAD')` witness is re-derived.
3. **Declared memory shape, tracked.** Delete the untracked env override.
   Interactive guests get 4 GiB in `nix/node-b.nix`, after the owner
   computer is measured at 4 GiB on a compact store (boot replay, Texture,
   research, one capsule build).
4. **Host bounds.**
   - keep N compact bases per computer, and turn artifact GC on;
   - detach loops reliably in every mount helper;
   - add an orphan-image sweep;
   - clean the updater's `incoming` directory after apply.
5. **Owner relief first, once the compaction tool exists:** one owner-
   approved maintenance window. Hold, stop, reflink backup, compact, verify
   heads and witness, boot, then switch to 4 GiB and measure.

## Owner maintenance window receipt (2026-10-09 05:31–05:38 UTC)

Owner approved ("Yes, do it. Downtime is fine"). Tool:
`projection-compact` (`internal/projectionbase/compact.go`, d339311e),
deployed to Node B in forced staging deploy run 37885830317 (50d5707a).

Sequence (as applied):
1. Hold the computer.
2. Stop it; firecracker gone.
3. Reflink backup `data.img.pre-compact-20261009T053125Z`. **Rollback:** stop, then swap this file back in.
4. Offline mount, then compact.
5. Verify the tape projection.
6. `fstrim`, then unmount.
7. `maintenance-serve` (health `ready`).
8. Unhold, then a normal refresh.

| Measure | Before | After |
|---|---|---|
| `state.texture/texture/.dolt/noms` | 9,566,672,041 B | **1,744,115,732 B** |
| Guest fs used | 12.0 GB | 4.2 GB (5.9 GB after boot) |
| Host `data.img` allocated | 17.7 GB | **4.5 GB** |
| `computer_event_projection_heads` (row digest) | `657160502cdadfb1` | `657160502cdadfb1` |
| `computer_event_index` count / max sequence | 577,757 / 577,757 | 577,757 / 577,757 |

- The tool exits non-zero unless `WitnessContentMatches(after, before)`, so the content witness held.
- After the normal boot: health `ready`, computer-surface SPA 200, `running_runs` 0.
- Owner VM RSS: 2.7 GB (was 14.7 GB at 02:46, before compaction).

**New finding, confirmed by this boot:** guest GC is still skipped. The boot log reads
`dolt gc skipped: live=5 GiB exceeds safe bounded-guest GC size (5 GiB)`,
but the store is 1.7 GiB. As noted above, the guard counts whole-disk used minus the journal. The other ~4 GiB is mostly:
- `choir-updater`: releases, store, and `incoming` (754 MB, still present);
- `files`.

Fix: the guard measures the Dolt store's non-journal noms bytes only.

**Host leaks, partly open:**
- One loop on a deleted image was detached.
- Ten more are **mounted**:
  - nine stacked on `/mnt/ed554data` (2026-10-04);
  - one on `/mnt/m11-probe` (2026-09-28).
  They are agent probe mounts over deleted guest images. They need an operator unmount; this session was not permitted to.
- The quarantine images `data.img.quarantine-1-*` (2026-10-07) are kept pending an owner decision. They may hold pre-restore state.
