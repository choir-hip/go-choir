# The paper's 48 GB disk filled with Dolt history and live data, and the runtime crash-loops (2026-10-19)

Status: documented; recovery not started. Found at the 16:22 UTC hourly
check. Class: red (the always-on paper's persistent state and VM
lifecycle). Owner rule: "we need paper publishing 24/7, even after
crashes."

This is the residual named in
[`guest-dolt-storage-growth-2026-10-15`](guest-dolt-storage-growth-2026-10-15.md),
under "Belief and remaining error": "the history and live class grows about
6 GB/day on the paper. That fills the 48 GB disk in about a week after the
fix, unless history is bounded". It filled in four and a half days. Bounding
history (squash plus `dolt gc --full`) is the open owner decision "Store A
squash".

## Evidence (guest console on Node B, read-only; counts, sizes, states and times only)

- **The warning was on for about 41 hours.** The first "compaction needed"
  line is at 2026-10-17 23:17:19: "avail 5508 MiB is under 25% of the disk
  after gc; history or live data fills it, not the journal". The online GC
  logged it after every run from then on. Nothing acted on it, and my
  hourly checks watched Node B's btrfs, not the guest's disk.
- **Live data, not journal.** The GC's own lines, sampled:
  `used=42 GiB live=38 GiB` (avail 4556 MiB), then `used=46 GiB live=43
  GiB` (avail 703 MiB), then `emergency: avail 0 MiB … used=46 GiB live=44
  GiB`, then `live=45 GiB`, then `live=46 GiB`. Each online GC collected
  the journal (180, 430, 154 MiB to 0) and freed nothing: "avail 0 MiB ->
  0 MiB".
- **Writes started failing before the crash.** "conjoin of database failed
  … write /mnt/persistent/… " warnings from 15:08:26, every five minutes.
- **The crash.** At uptime 41155 s (about 15:40 UTC) Dolt raised a fatal
  error: "write …/.dolt/noms/vvvv…: no space left on device: error writing
  to database journal file". The runtime restarted and ran on until a
  periodic GC failed at 16:18:40 ("gc_copier, CopyTableFile error: write
  …/noms/oldgen/nbs_table_…: no space left on device"), then died again.
- **The crash loop.** From 16:19:11 to 16:19:19 the runtime restarted five
  times, about every two seconds. Each boot ran the emergency boot GC
  ("avail 0 MiB <= 512 MiB; used=46 GiB live=46 GiB") and died writing a
  manifest ("nbs_manifest_…: no space left on device: fatal error closing
  table persister", from `store.MaybeRunDoltGC`). No runtime line after
  16:19:19.
- **The platform.** choir.news health is ok on 8be39bb36. The front page
  serves its stored stories. The paper's runs API returns no runs, and
  nothing new publishes.
- `data.img` is 49,152 MiB (48 GiB), last modified 16:20.

## Mechanism

- Default Dolt GC collects only the journal. Everything reachable from a
  commit stays, and the store commits a checkpoint every 100 finalized
  events. History plus live data grew from 38 to 46 GiB in about two days
  here, in line with the 6 GB/day measured on 10-15.
- The emergency GC at 0 MiB cannot finish: default GC writes its output
  before it frees anything (measured 1.9 GB on 10-15). So every boot dies
  in the boot GC. That is failure mode FM3 from the 10-15 doc, reached
  through history rather than the journal.
- `compaction_needed` was made visible on 10-15 (FM8) so that this would
  not be silent. It was visible. No one was watching it: it is a signal
  with no consumer.

## Recovery options (not chosen yet)

1. **Grow the disk while the computer is stopped** (the 10-15 recovery,
   32 to 48 GiB). Hold the computer, stop the VM, reflink-copy
   `data.img` as the rollback, extend the file, grow the filesystem, boot.
   Fast and proven. It buys about two to three days at 6 GB/day, plus
   whatever the boot GC then reclaims.
2. **Offline compaction** (`docs/runbooks/offline-guest-gc.md` with
   `projection-compact`: squash plus `dolt gc --full`). This reclaims most
   of the 46 GiB (1.7 GiB after compaction on 10-09). The runbook keeps
   `--full` "not automated pending review", and the squash is the owner's
   open decision.
3. Both: grow now to restore publishing, then compact on the owner's
   decision.

## Lessons

- A health flag without a consumer is the same as no flag. The hourly
  checks should read the guest's `dolt_gc` record and disk, not only the
  host's.
- The 10-15 doc predicted this fill date to within days. A predicted
  deadline belongs in the handoff's open-decision list with its date.
