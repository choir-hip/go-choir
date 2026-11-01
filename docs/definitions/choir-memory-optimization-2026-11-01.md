# Choir memory optimization: the backup, the paper, and Node B (2026-11-01)

The owner set this goal on 2026-11-01: "optimize memory usage for backup
and the paper computer and in general without major cost in compute or
storage." Earlier the same night: "Increasing memory is not the solution",
and "if we had trouble backing up the old files, we shouldn't need to keep
backing up those old files ... We should probably be able to get rid of all
those old files off of Node B anyway."

```yaml
definition_version: throughline/1
readiness: executable
readiness_authority: owner /goal, 2026-11-01 (quoted above)
start:
  observed_at: 2026-11-01T05:40Z
  deployed: 9d3cbb7a4 (paper compaction swapped at 05:30, 21.59 -> 20.66 GB)
  node_b: >-
    31 GiB total. The paper's firecracker holds 7.8 GiB RSS. The off-host
    backup's restic holds 4.0 GiB, thrashing at its 4G MemoryHigh: 6.2 TB of
    refaults, 84 files/s. The deploy's nix build holds about 2.7 GiB. The
    owner's computer holds 1.4 GiB, postgres about 1.1 GiB, corpusd 0.75 GiB.
    b@c's wake was refused because it was charged its 7.2 GiB peak.
  paper_guest: >-
    AnonPages 1.13 GiB, Cached 6.14 GiB, Inactive(file) 6.08 GiB,
    Active(file) 27 MB. state.db is resident at 4.83 GiB 18 minutes after
    boot. The guest reports MemAvailable 6.67 GiB, which the host cannot get
    back.
  backup: >-
    The repo is at 456 GB against a 450 GiB quota. Nightly adds were 109 GB
    (10-31) and 39 GB (11-01). sha256/{pin-receipts 5.4M, computer-event 2.5M,
    computer-event-payload 2.9M} are loose tape files. Today's census shows
    0 covered by segment copies (an old binary: segment v2 unsupported).
finish:
  deliver: >-
    Node B's steady memory goes to the computers' real working sets, and the
    backup runs in bounded memory without thrashing. Neither costs material
    compute or storage.
  artifacts:
    - every pin goes to tape segments, so no new loose tape files are written
    - taperepack copy and delete, so the 11M loose files are gone from Node B
      (the off-host backup keeps the historical copies)
    - the backup's sharding is removed once nothing is left to shard; restic
      runs under a GOMEMLIMIT above its live heap
    - busy_cold (D6 S5) returns the paper's cold cache, periodic, with the
      refault guard
    - admission charges the settled RSS (VMCTL_ADMISSION_NEED=wake)
    - the cause of the nightly backup growth is named, with a fix or a
      recorded decision
  acceptance: >-
    Node B readings, metadata only: the paper's host RSS falls to near its
    working set with refaults flat; the next nightly backup finishes in under
    an hour, with no memory.high thrash and no shard thrash; the sha256 loose
    file count is near zero; b@c wakes with no extra memory.
value:
  better_means: >-
    Minimize resident memory held for no work (cold cache, stale peaks,
    thrash) on Node B, and keep refaults, CPU and storage near flat.
  goodharting_would_be: >-
    Raising caps or quotas, dropping data from the backup without a proven
    copy, or reclaiming hot pages so that refaults move the cost into disk IO.
boundaries:
  mutation_class: red (tape write path, guest memory on the paper, deletes of verified-covered tape files)
  invariants:
    - a loose tape file is deleted only when taperepack proves a verified segment copy
    - no reclaim on the paper during a self-development apply or a live capsule
    - production reads are metadata only
    - no content is read
now:
  status: working
  slice: >-
    Landed and deployed: busy_cold, admission wake and the seed deletion
    (dd84b1f6c, 06:41Z); every pin but a public non-batch payload segmented,
    with taperepack on the same rule (16f7b5d01, 07:09Z; two seats accept);
    the paper's busy_cold entry =active (88654a665). A new-rule census is
    running on Node B. Tonight's backup is still on its shards under the
    runtime 7G/8G cap.
  belief: >-
    believed_state: no new loose tape files after 07:09Z except public
    non-batch payloads; the paper returns up to 1 GiB of cold cache per
    10-minute pass. main_uncertainty: whether the paper's host RSS falls with
    the guest's reclaim (free page reporting) and whether state.db refaults
    stay under the guard. next_observation: the paper's first active act line
    (rss before and after, refaults).
  accepted_edge: >-
    d@e cannot prove a busy_cold act (266 MiB inactive file, under the 512 MiB
    keep), so the paper's first watched acts are the proof; rollback is its
    entry back to =observe.
  next_action: >-
    Read the paper's first acts; after the census, taperepack copy --apply for
    every computer it names; then census, keep-digests export and delete
    --apply; then drop the backup's BIG_DIRS sharding and the runtime cap.
receipts: []
```
