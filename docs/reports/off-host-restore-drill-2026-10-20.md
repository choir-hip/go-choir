# Off-host backup restore drill, first run (2026-10-20)

Status 2026-10-30: the Dolt restore rows are obsolete after the cutover; the tape and image drill results remain the record. A lost disk was rebuilt from the host alone on 2026-10-30. Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

Scope: the restore drill of docs/problems/no-off-host-backup-2026-10-20.md
section 9, run on Node B against snapshot `17cdae99` (run 2, 21:42 UTC) in the
`daily` repository on node-a. Script: `scripts/off-host-restore-drill.sh`.
Each piece ran as a transient unit capped at 8G high, 10G max, 4G swap, Nice
19 and idle I/O, restoring into `/var/lib/go-choir-restore-drill` (never a
live path) and deleting its restore before it exited.

## Results

| Piece | What was restored | Check | Result | Time | Memory peak |
|---|---|---|---|---|---|
| tape | 200 random files: 41 `computer-event`, 43 `computer-event-payload`, 16 `file-cas-chunks`, 100 `pin-receipts` | `cmp` with live | 200 identical, 0 different, 0 missing | 6 min restore (walks the 9.6M-file tree), 5 min to pick the sample | 7.6 GB |
| tape | `auth.db`, `mail.db` (sqlite `.backup` copies) | `PRAGMA integrity_check`, table count | ok, 10 tables; ok, 20 tables | in the above | |
| dolt | corpus Dolt backup directory (3.5 GB) | `dolt backup restore`, then query | head `jtl76geb…` equals run 2's recorded head; 31 tables; 1 branch | 34 s restore | 8.0 GB |
| dolt | platform Dolt backup directory (31 GB, 17 files) | `diff -rq` with live | identical | 253 s (about 120 MB/s) | |
| image | one stopped test computer's `data.img` (`vm-a708…`, the S6-0 drill computer) | `cmp` with live; `e2fsck -fn` | identical; ext4, clean | 21 s with `--sparse` (65 s and 32 GB on disk without) | 7.3 GB |

## What this shows and what it does not

Shown: the repository on node-a decrypts with the key on Node B, and the
owner's offline copy of that key opens it (owner, 2026-10-20: "MATCH");
sampled tape files, both sqlite stores, both Dolt stores and a guest image
come back byte-identical or structurally sound; a Dolt store restores
from its backup directory into a working database at the recorded head.

Not shown:

- **A restore onto another host.** Every piece ran on Node B. A recovery
  host needs about 10 GB of RAM for restic alone (it walks the whole
  snapshot tree), the repository password from the offline copy, and the
  tunnel key or another route to node-a.
- **The platform Dolt store restored into a running database.** Its
  backup directory is byte-identical to live; the `dolt backup restore`
  path was shown on the corpus store only (disk: a second 31 GB copy did
  not fit beside the 40 GB guard).
- **A computer rebuilt from the restored tape onto an empty disk**
  (problem record section 13 item 10 (b)), which decides whether guest
  images can leave the nightly run.
- **A full restore and its time.** Rough recovery time from these rates:
  the Dolt and image data at about 120 MB/s is about 15 minutes for 100
  GB; the tape's 9.5 million small files are the unmeasured part.
- **Booting a restored image.** Checked with `e2fsck -n` only.

## Lessons

- Restore needs the same memory as backup. The first attempt ran without
  a cgroup and reached 6.3 GB with 8 GB available on the host; it was
  stopped and rerun capped. The script now says to run it capped.
- Image restores need `--sparse`, or a 5.7 GB image takes 32 GB.
- The platform Dolt backup directory is 31 GB in 17 files. When a sync
  rewrites them, restic re-reads all 31 GB, which is part of each run's
  read volume.
