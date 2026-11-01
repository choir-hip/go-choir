# The first sharded off-host backup thrashed at its 4G line for three hours

Date: 2026-11-01. Status: mitigated for tonight's run; the durable fix is not
made yet.

## Problem

The daily off-host backup started at 02:30:38Z. At 05:39 it was still on its
first shard (`shard-pin-receipts-0`, about 1.36M of the 5.4M files in
`platform-artifacts/sha256/pin-receipts`). It was moving at about 84 files a
second. The earlier unsharded runs took 35 minutes (2026-10-30) and 73
minutes (2026-10-31).

## Evidence (read-only, Node B, 05:3x)

- The unit is capped at MemoryHigh 4G, MemoryMax 6G, GOMEMLIMIT 3GiB, set by
  086e1669a on 2026-10-31. Tonight's run is the first under that cap.
- restic's anonymous memory was 4.04 GiB, which is the MemoryHigh line, with
  2.8 MB of file pages. The unit had 16.9M memory.high events,
  30.0M major faults and 2.03 billion file refaults.
- The restic process: read_bytes 6.2 TB from the block device and rchar
  0.87 GB. So almost none of the reads were restic reading files. They were
  its binary and its cache being evicted and faulted back in. The unit as a
  whole: IOReadBytes 12.3 TB and CPU 60,090 s in 3h09m, about 5.3 cores.
  GOMEMLIMIT 3GiB sat under a live heap of about 4 GiB, so the collector ran
  without stopping.
- Cause, a hypothesis consistent with the numbers: the first run of each
  shard has no shard parent yet. It follows the last full snapshot
  (scripts/off-host-backup.sh, "the shards"), whose tree node for
  pin-receipts holds all 5.4M entries. The sharding's premise was that a
  shard's tree memory is a fraction of the whole. That premise does not hold
  for the parent's tree on the first run.

## Mitigation (05:39Z)

`systemctl set-property --runtime go-choir-offhost-backup.service
MemoryHigh=7G MemoryMax=8G`. This is a runtime-only drop-in under
/run/systemd/system.control. It lasts until the next reboot, or until it is
removed with `systemctl revert`. The major faults stopped (30,007,218 to
30,007,219 in a minute). The shard went from 84 to about 1,700 files a
second. The host had 16 GiB available after the change.

## Direction (not started)

- Keep the first run's parent tree out of the cap. Either run a shard's
  first snapshot without a parent (its files are small, so a re-read is
  cheap), or size the cap for the first night only.
- GOMEMLIMIT must sit above the live heap, or the collector spins.
- The real lever is the 11M small files themselves. They are old-system flat
  tape directories (pin-receipts 5.4M, computer-event-payload 2.9M,
  computer-event 2.5M). Packing or retiring them after the cutover ends the
  sharding.
