# The paper's store snapshot aborts at its time budget, and its scratch copy takes Node B under the deploy floor (2026-11-22)

Problem. At 07:54Z Node B had 50 GiB free (deploy floor 60 GiB, alarm 65).
The paper computer's data image went from 56.0 GiB at 06:01Z to 107.2 GiB at
07:01Z, one step. While free space is under 60 GiB,
`scripts/node-b-deploy-disk-preflight` refuses every staging deploy.

Evidence (read-only, metadata and console lines only).
- Growth-meter ledger `/var/lib/go-choir/growth-meter/2026-11.jsonl`: the
  same step on 2026-11-20 (54 to 102.6 GiB by 18:00Z, back to 53.9 at 19:00Z)
  and 2026-11-21 (55 to 105.8 GiB at 12:00Z, back to 55.4 at 13:00Z).
- Guest console: a snapshot capture began about 06:21Z; at 07:51:08Z
  "snapshot failed: aborted at the 1h30m0s time budget
  (CHOIR_SNAPSHOT_MAX_DURATION) with nothing recorded"; backoff until
  08:11:08Z. `store_stalled` lines during the capture. The last good snapshot
  root is from 2026-11-21T12:00:38Z (19.9 h old at the alarm).
- `internal/agentcore/snapshot_producer.go:38-57,95-97`: a capture writes a
  store-sized scratch copy on the data volume (the store is about 50 GiB).
- Discard is on for the data drive (`fc-config.json`, `nix/node-b.nix:1731`);
  the host gets the blocks back only after the guest's hourly
  `choir-data-fstrim` (`nix/autoputer-vm.nix:1002-1036`).
- The image is flat at 108 GiB at 07:57Z; it is bounded at 128 GiB, so the
  host cannot fall below about 30 GiB from this alone.

Belief. The step is the snapshot's scratch copy. On the two earlier days the
snapshot finished and the space came back within two to three hours. Today the
capture did not finish in 90 minutes, so there is no new snapshot, and each
retry writes the copy again. The store has grown to where a full copy plus
chunking no longer fits the budget; raising the budget would hide that.

Remaining error. Not confirmed: that the trim ran (no fstrim line in the
console), or that the aborted capture removed its scratch file. The separating
observation is whether the image drops by 10:00Z. Not known: why the capture
is slower than on 2026-11-21 (store size, the read stalls, or load). The
structural fault is a snapshot that copies the whole store each time: its cost
grows with the store, and it needs free space equal to the store on the guest
and on the host. Related: `paper-chunk-writes-drain-node-b-disk-2026-11-12.md`.

Nothing was changed. No remedy that trims or deletes was run.

## 09:04Z: the retry holds the copy again; the deploy of d7d276b55 was refused

- CI run 45000646351 (the proxy resolve fix): every test job passed; "Deploy
  to Staging (Node B)" failed at the disk preflight, "root filesystem still
  below required headroom", after its own bounded reclaim.
- The retry began at 08:11Z; its scratch copy finished at 08:28:27Z ("wal
  bound: checkpoint after snapshot capture") and it is streaming. In the first
  attempt the copy took about 6 minutes and the stream stage the other 83 of
  the 90. `store_stalled` holders in `ReadObjectSnapshotFiltered` through the
  morning (25, 86, 43, 31, 11 per console file): the stream contends with the
  running paper for the guest's own store. The producer logs no rate.
- `snapshot_producer.go` (about line 590): the host keeps the chunks an
  attempt uploaded and a retry skips them, but the scratch copy is rewritten
  and every chunk is read and hashed again each attempt. So retries make
  progress and each holds about 50 GiB while it runs; the backoff is 20 min.
- The host GC (`go-choir-disk-gc.service`) last ran 00:58Z and freed nothing;
  no dead nix store paths; the journal is 249 MB. Not verified as reclaimable:
  `vm-state/candidate-fleet-bf04bbc7…` (19.7 GiB, last written 2026-11-10).
- No existing host-side way to make the guest trim or to hold the retry was
  found. Nothing was run. Decision: wait for the second attempt's outcome
  (budget ends about 09:58Z) and the trim after it; no deletion without the
  owner.

What the fix should be (not built): a snapshot that does not copy the store
(read from a consistent view), a trim straight after a capture ends or
aborts, a stream rate in the log, and a deploy preflight that says what holds
the space.

## 12:10Z: the retry succeeded; three statements above are corrected

Seen in the paper's console and in the code (a read-only investigation; nothing
on Node B was changed or restarted).

- The retry finished at 09:27:34Z: "snapshot taken ... 55045910528 bytes
  (13438943 pages of 4096) in 1h6m26.149s ... 13124 chunks, 10416 uploaded
  (6653754264 bytes sealed), 2708 already held ... copy 7m17.275s (30733
  pages/s), pin 7m17.275s, witness 54m51.741s, upload 4m15.148s". Node B had
  100 GiB free at 10:02Z after the guest's hourly trim.
- **Correction: the slow stage is the witness, not the stream.** The order is
  copy, head cross-check, witness over the scratch file, upload
  (`internal/snapshot/snapshot.go`, about lines 683 to 727). The witness is a
  digest of the whole store and took 55 of the 66 minutes. It reads the
  capture's own scratch file, so it does not contend with the running paper;
  the `store_stalled` lines are the paper's own object reads.
- **Correction: an aborted capture does remove its scratch** (every path,
  `snapshot.go` about lines 603 to 617). The image stays large until the
  guest's trim runs (`choir-data-fstrim`: 10 minutes after boot, then hourly;
  the volume is ext4 without the `discard` mount option, on purpose,
  `nix/autoputer-vm.nix` about line 994).
- **Correction: 55,045,910,528 bytes is 51.3 GiB.**
- How often: a capture is due when the tail's replay at the assumed 113
  events a second passes 4 minutes (27,120 events; the paper adds about 8,000
  an hour), but not before the capture floor: the larger of 10 times the last
  capture's duration and 24 hours for every 8 GiB sealed
  (`snapshot_producer.go` `snapshotCaptureFloor`). With 66 minutes and 6.65 GB
  sealed that is about 18.6 hours, so the paper captures about once a day and
  the next is due near 04:00Z on 2026-11-23. The 113 is d@e's worst reading
  of 2026-11-14; a measured rate replaces it only when lower and is kept in
  memory only.
- Each capture costs three passes over the whole store (write the copy, read
  it for the witness, read it again to hash and seal the chunks), and 79% of
  the chunks were new a day after the last capture, because the chunks are
  fixed page ranges and SQLite's writes are scattered.

What the fix should be, restated. Removing the scratch copy alone (the witness
and the upload inside the read transaction) trades 51 GiB of scratch for a WAL
pinned for an hour, about 2 GiB at the write rate seen. That removes the disk
swing; it does not remove the hour. The structural cost is the whole-store
witness and re-seal each day; an earlier incremental design did not converge
(`incremental-snapshots-design-three-review-rounds-structural-assessment-2026-11-15.md`).
A trim straight after a capture shortens the swing's tail and nothing else.
None of this is built. The second store-sized copy, the compaction (due by layout age when this was written, since removed), is
its own record:
`compact-copy-staged-on-a-51-gib-store-holds-node-b-under-the-deploy-floor-2026-11-22.md`.
