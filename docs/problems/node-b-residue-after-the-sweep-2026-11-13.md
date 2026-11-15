# What still grows on Node B after the sweep

Date: 2026-11-13. Status: open. Found by a read-only survey of Node B
(growth-meter ledger, `du`, `pg_class`), 16:00Z to 17:00Z.

## Problem

With the destroyed computers gone and the snapshot loop fixed, Node B still
grows about 10 GiB a day against 135 GiB free. Four items have no bound or a
bound that does not hold.

## Evidence

| item | size | per day | bound today |
|---|---|---|---|
| `platform-artifacts/capture-bodies` | 24 GiB, 387,000 files, 11 GiB older than 7 days | +1.7 GiB | none: not in the collector's namespaces, and the writer never prunes |
| the paper's data image | 50.8 GiB allocated; the store inside is 21 to 22 GiB | +2.1 GiB | discard and an hourly guest fstrim are configured, yet about 30 GiB is not returned |
| d@e's data image | 28.6 GiB; its tape is 3.5 GiB | +2.0 GiB | the same |
| `computer_event_append_receipts` | 27.7 GiB (17.6 heap, 10.0 indexes), 12.7 million rows | +0.8 GiB | none; compaction saved 5% on the paper |
| the paper's tape segments | 71.8 GiB | +2.6 GiB | none for a live computer (station S5 cuts the rate) |
| `platform-update` | 7.7 GiB | +1.0 GiB | the collector, 168 hour grace |

Bounded and healthy: guest releases (8.6 GiB, retention every 30 minutes),
`/nix/store` (25.9 GiB, collected every 4 hours), journald, chunks.
One-time: b@c's hibernated image 19.2 GiB (never deleted, owner rule), two
stopped images 7.7 GiB, old work directories 2.5 GiB.

corpusd has no memory ceiling; its unit peaked at 9.7 GiB, almost all file
cache (268 MB of its own memory). The host never fell below 11.6 GiB
available in 24 hours. Not urgent.

## Belief

Capture bodies are the only large store with no bound at all. The two data
images are the largest possible release (up to about 50 GiB) and the least
understood: nobody has looked at what fills the volume beside the store.
Receipts need fewer rows, not smaller rows (station S6, after S5).

## Remaining error

The images were measured from the host only. The share of capture bodies
that nothing references is unknown. One index on the receipts table
(`receipt_id_idx`, 2.2 GiB, 2,184 scans) may be droppable; its readers are
not yet identified.

## Update, 2026-11-13T17:15Z

- Capture bodies are nearly all referenced by a capture row (36 unreferenced
  files, 5.7 MB, at the 2026-10-29 accounting; every reader goes through the
  row). A collector would release megabytes, so `lane/capture-bodies-gc` is
  parked, not landed. A bound needs an owner retention ruling on capture rows.
  The filesystem already compresses (zstd:3; a 200-file sample compresses to
  31%), so `du`'s 24 GiB overstates the disk used.
- The data images are not a trim failure. Host allocation sits 0.3 to 2.5 GiB
  above what each guest's filesystem reports as used. The volume holds about
  27 GiB beside the store on both the paper and d@e, and no surface says
  which directories. Unpruned by any code found: the updater's release
  closures and store, the Go module cache, the Go build cache (its own 5 day
  trim only), the seeded source checkout.
- New problem: d@e's data volume is 95.5% used, 1.5 GB available (compute
  status, about 17:05Z), under the 4 GiB capsule admission floor. Capsules there may
  already be refused. Not verified.
- A paper snapshot costs about 4 GB, not 25 (5,858 new chunks of 10,524 on
  2026-11-12); the 25 GB in two code comments is from before chunk
  compression. Design: docs/design/snapshots-cost-the-change-2026-11-13.md.

Next: a `data_volume` section in guest observability (statfs and allocated
bytes per top-level directory), then prune what it names.

## Host survey, 2026-11-13T23:50Z (read-only)

A second pass over every directory above 0.2 GiB found no unaccounted
consumer on the host. Corrections and additions to the survey above:

- `vm-state/candidate-fleet-bf04…` (19.2 GiB logical) is not a candidate: it
  is b@c's VM directory under an old id scheme, registered and hibernated.
  `candidate-fleet-e15cb…` is the owner's computer, active. Nothing creates
  or reaps `candidate-fleet-*` as a class. A third such id in
  `ownerships.json` has no directory (a dangling record).
- `btrfs filesystem du` reports logical bytes. Per-path sums come to about
  367 GiB against 305.7 GiB of data used; the gap is zstd compression and
  links shared between `/nix`, the guest release store and the jails.
- The nix store is bounded: two system generations, a collector every four
  hours (9.3 GiB freed at 20:40Z). The "9.9 GiB in a day" was the gap
  between collections.
- The file-chunk collector is working: 10,193, 10,686 and 12,868 objects
  deleted in three hourly runs (7.2 to 8.5 GB each), `in_grace` at 0.
- Receipts: 28 GB, 12.6 M live rows, 2.8% dead, so the size is live rows,
  not bloat. Five indexes total 10 GB; `receipt_id_idx` (2.3 GB) has 2,198
  scans in its lifetime and its reader is not identified.
- Unbounded and small: `artifact-gc-log` (1.5 GiB: applied sweep manifests
  and compressed day logs nothing prunes), `tape_sweep_ref_chunks`
  (935 MB of sweep resume state), `taperepack-work` (1.8 GiB from a one-time
  run on 11-01, referenced by no unit).
- Needing an owner decision: four tape directories (5.5 GiB) of computers
  with no ownership record that the sweep skips as registered or routed;
  three hibernated test-class VM directories (5.6 GiB) the retention prune
  has not removed, cause not established.

The two largest controllable items are live and inside the paper: its guest
store (docs/problems/the-papers-store-doubles-in-eleven-days-2026-11-13.md)
and its tape.
