# The paper is never checkpointed (2026-10-27)

Status: open; documented before any fix. Station D8 of
docs/definitions/choir-consolidate-and-scale-metamission-2026-10-26.md.
Owner, 2026-10-27: "We need to have auto checkpointing on. We need to
figure out how to make it resource efficient and do it the right way, but
it's an essential feature"; "We need snapshotting and hibernation
everywhere automatically." Investigation: Sonnet, read-only, 19:3x.

## Evidence

- The paper's boot: `projection recovery resume (local=4394600 W=34660
  H=4394600)`; its host projection base sits 4.36M events behind its tape.
- `go-choir-checkpointd` (timer, every minute) logged succeeded jobs today
  for d@e (`checkpoint-verify:<digest>`, from d@e's guest cadence) and b@c
  (`cadence`), none for the paper. The only paper line in 7 days:
  `checkpoint reconcile: enqueue computer-1355b83b…: context canceled`
  (10-27 00:26:50); nothing else in 40 days of journal.
- The service hit `start operation timed out` on 10-26 20:02 and 10-27
  00:13, 00:26, 08:38; the owner's computer 03335285 (595k events) has a
  failing job (seed-witness extraction).

## Why (code; verified unless marked)

- Host cadence has no class or size filter: `ReconcileProjectionJobs`
  (`internal/platform/projection_jobs.go:~285-320`) enqueues every computer
  where `checkpointDue` holds; the paper is due.
- Hypothesis A: the paper's job row is `blocked` or `failed`; a blocked row
  with unchanged inputs returns silently and a failed row waits out a
  backoff up to 16 h, neither logging. Hypothesis B: the job is killed by
  `TimeoutStartSec=10min`, `MemoryMax=6G` or the 5-min lease before it logs;
  a first paper job replays about 4.36M events. The job row
  (`GET /projection_jobs/<id>`) decides.
- The guest-side cadence (`VM_CHECKPOINT_CADENCE_ONLY`, `nix/node-b.nix:1777`,
  `internal/vmmanager/checkpoint_cadence.go`) is on for d@e only and refuses
  `critical_protected` and `public_platform` by design until the live
  extract's append hold is measured on the paper (design
  `apply-checkpoint-without-replay-2026-10-26.md`, R2). On d@e one
  checkpoint took 5.5 s and 118 MB.

## Hypothesis A confirmed (FP0, 2026-10-27, corpusd job read)

The paper's projection-base job row reads `status: blocked`,
`reason: cadence`, `failures: 9`, `updated_at: 2026-10-15T02:42:29Z`,
`target_sequence: 402176`, `watermark_sequence: 34660`,
`cold_tail: 4365298`, `alert: urgent_tail,repeated_failure`, with the error
`projection base refused: installed base does not open: runtime store:
bootstrap: apply schema: Error 1049: database not found: texture`, on the
seed base `abacf6b0…`. The job has been blocked for twelve days; a blocked
row with unchanged inputs never retries and logs nothing, so the alert
reached no one. Next: why the seed base lacks the `texture` database, and
the smallest reseed (design pass running).

## Design pass (Opus, read-only, 2026-10-27)

Verified:
- The seed `abacf6b0…` belongs to the paper alone: sequence 34660,
  vocabulary v2, 462 MB, made 2026-10-13 18:43. Its texture workspace is
  only `.darc` archive files, which the repo's own GC no longer writes
  (archive level 0 since 10-19, `squash.go:251`).
- The engine finds the `texture` database but cannot load it: the error comes
  from `verifyInstalledHead` (`install.go:274-281`) via bootstrap. The
  likely cause is damage, from Dolt bug #11747 or archive-version skew; this
  is a hypothesis until S0 opens it.
- Turning on the guest cadence today would not help: `checkpointBaseSeed`
  (`checkpoint_verdicts.go:174-177`) hands the host the same broken base to
  verify against.
- The guest bind runs a full `SELECT *` under the append hold
  (`checkpoint_restore_bindings.go:87-135`, `dolt_state_extractor.go:389`).
  That is 2.2 s on d@e and minutes on the paper, so it cannot be the paper's
  checkpoint as built.
- A host genesis rebuild runs about 42 to 50 events/s, which is 24 to 29
  hours for 4.39M events. The 10-minute service run, the up-to-16 h failure
  backoff and the scratch prune on re-enqueue keep it from finishing.
- Every publish writes a full tar (`publisher.go:63-127`). At the paper's
  size that would be terabytes a day.
- Periodic memory snapshots of a busy computer buy nothing safe: restoring
  one after a crash resumes work, which the restart rule forbids. A snapshot
  belongs only to a planned stop.

Slices, in order:
- S0, operator read: open the seed with the current binary in checkpointd
  scratch, to confirm the cause.
- S1, orange, tier 1:
  - quarantine a base refused twice and fall back to an older one;
  - re-admit a blocked row when the build changes;
  - log blocked rows hourly;
  - give checkpointd a per-job deadline below its service timeout.
- S2, red, tier 2, owner decision: a host capture as the paper's first base.
  - Hold the guest's appends, pause it, reflink its data image, and resume.
  - On the host, at low priority, compact and check that the head equals H.
  - Publish the result typed `live_captured`.
  - From there, incremental replay at about 50 events/s keeps up with the
    paper's 7 to 12.
- S3, red, tier 2: witness v2, a hash-only bind (per-table Dolt root hashes)
  in milliseconds. Lift the class refusal; cadence for every computer.
- S4, red, tier 2: incremental base publication (content-addressed table
  files plus journal segments).
- S5, red, tier 2: a busy hibernate on planned host stops (hold, pause,
  snapshot, stop; on wake, resume).
- S6, orange, tier 1: diff snapshots for every non-protected computer.

## Correction: the seed base opens (FP0, 2026-10-27 21:0x UTC)

S1 (6d7a37d07, deployed 21:02:30) re-admitted the paper's blocked row on
the new build. Generation 22 is `running`. It installed the seed
`abacf6b0…` and is replaying from event 34660, at about 1024 events per
30 s; the replay page fetch alone takes 8 to 9.5 s. So the seed is not
damaged. The 10-15 refusal came from code that has since been fixed, and
the row sat blocked for twelve days only because a blocked row never
retried. The design pass's damage hypothesis is refuted, and S0 is moot.

What follows:
- The replay must cover about 4.38M events, roughly 36 hours at this rate.
  S1's 8-minute job deadline ends each attempt. A failed row re-enqueues
  under a new generation and prunes its scratch, so a host replay of the
  paper never converges. It only burns up to 8 minutes of a core per
  attempt, under the backoff.
- S2, the host capture, stays the route to the paper's first base. The
  fetch time is a residual: about 9 s per 1024 events from the artifacts
  store, investigated separately.

## Owner decision, S2 (2026-10-27 20:43 UTC)

Owner: "Yes, Paper's first checkpoint is a copy of its live desk [disk].
Notice that the auto paper actually doesn't have unique data. It doesn't
have its own file system. So it's really something which can be run in a
very lightweight way. I mean, I could even imagine it running on a
ephemeral sandbox ... not that I want to go that way right now, but yes,
first copy."

S2 is authorized: the paper's first base is a host capture of its live
data image, typed `live_captured`. A second point carries weight: the paper
holds no data that exists nowhere else; everything it has comes from its
sources and its tape. So a captured base being wrong costs a rebuild, not
data loss. That lowers S2's risk class for the paper. The ephemeral-sandbox
idea is recorded, not pursued.

## S2 d@e proof (FP0, 2026-10-28 00:28 UTC)

S2 landed as 198f89515..0947393dd, plus 825adcbbf (a lease-guard
exemption CI caught; the CI mirror now runs the store source guards for
store, projectionbase and agentcore lanes). Reviews: 364, 365, 366
(Devin and DeepSeek), 367 (Devin). CI 40607091576 passed, and staging is
on 825adcbbf.

`checkpointd capture-base -computer computer-59c902c5…` from the runbook
command, 00:28:37 to 00:28:43 UTC:
- vmctl: `paused 18ms (clone 18ms) clone_err=<nil> resume_err=<nil>`;
  Firecracker `resume vm` took 173 us.
- Kernel: `EXT4-fs (loop0): recovery complete`, then mounted, then
  unmounted at 00:28:43.
- The base is at sequence 12492, `d12049fcfbf1…`. The watermark readback
  names it (`watermark_sequence 12492`).
- The unit ran 6.7 s, used 4.6 s of CPU, peaked at 1017 MB of memory, read
  949 MB and wrote 465 MB.
- Cleanup: no `data.img.capture` or `capture-pause.json` in the VM
  directory, and no mount under the capture root.
- The guest was not restarted: its Firecracker process was 87810 s old
  after the capture.

Still open: d@e's next cadence base seeding from the root, with its
`epoch_root`; and the paper run after the 04:00 compaction.

## Owner decision, epoch root (2026-10-27 21:5x UTC)

Owner asked "Why do we need to replay?" and then said: "it's cool to have
the same event log for the autoputer. If we can make it work, we should. But
maybe we lose some event log data and restart it from here?"

The decision: the paper's captured base is an epoch root. Derivability is
measured from the root. Every later base replays only the tape above it, and
checkpoints are judged against it the usual way, so the paper joins the
fleet's ordinary checkpoint loop. The tape below the root is no longer needed
for any proof. Nothing deletes it: archiving or deleting the pre-root prefix
is a separate, irreversible owner act. The cost is that the state at the root
is no longer proven derivable from the paper's own history; the owner
accepts this because the paper holds no unique data.

## Epoch root: what the code does (lane/ckpt-capture)

The owner decision above changes what `live_captured` means. A captured base is an **epoch
root**: it defines state at its sequence. Derivability is measured from
it. Proofs no longer need the tape below it. Nothing deletes that prefix;
it stays where it is, and deleting it would be a separate owner act.

- **The root itself.** It is marked `origin: live_captured` and keeps that
  mark.
- **Bases replayed from the root.** They are ordinary derived bases with no
  origin mark. Each records `epoch_root {base_ref, sequence}` so the
  provenance stays visible. Their own descendants record the same root.
- **Verdicts.** A checkpoint is judged against the nearest base, as before,
  and that includes the root. A match is Verified and a mismatch is
  NotDerivable (review 364, finding 4).
- **The floor.** No replay starts below the root. A genesis repair of the
  computer starts from the root. A quarantined root with no base above it
  stays the seed, so its job fails closed and blocks; it never falls back
  to a pre-root base or to genesis. A checkpoint below the root settles
  `unverified/no_base`.
- **Where the floor is enforced.** In two places:
  - corpusd: the epoch-root row, read at job admission;
  - the host rebuilder: the highest floor among the computer's published
    descriptors, checked whatever the seed policy.
- **Rollback safety.** The root's descriptor carries
  `vocabulary_version: "v2+live_captured"`. A pre-S2 reader refuses it
  (fails closed); descendants read normally.
- **Review 365.**
  - A checkpoint below the root settles `unverified/no_base` in the
    checkpointd verdict pass too, before the held-base judge: a held
    pre-root base that matches never makes it Verified.
  - The seed fast path (the advertised watermark) applies the floor.
  - At the floor's sequence a seed must be the root or one of its
    descendants. A resumed genesis scratch with no root provenance is
    discarded, and the next attempt seeds from the root (one transient
    retry, never a blocked job).
  - The floor scan fails closed on a descriptor that names the computer
    and cannot be read or parsed.
  - A capture whose tape head, or whose captured store head, is not above
    the advertised watermark is refused: before the pause for the tape
    head, at settle for the store head.

**Residuals.**
- **Guest restore below the root.** The guest's own restore and
  rematerialize (`internal/agentcore/restore_base.go`) can still replay
  the pre-root tape. That code is a guest security surface; the fix is a
  follow-up, not part of this lane.
- A resumed scratch left by a failed publish costs one transient retry
  before the job seeds from the root.
- The boot path keeps a capture pause marker when a Firecracker process
  outlives the boot cleanup, and the reattach path retries a marker on
  every later reattach call. Both read `/proc`, so they have no unit test
  on a non-Linux host.

## S2 runbook

`checkpointd capture-base` (cmd/checkpointd/capture.go) does one named
computer per run. Nothing schedules it.

**What it needs.** The `go-choir-checkpointd` unit cannot run it: the
shared hardening blocks `@mount` and has no capabilities. So it runs in a
transient unit with:
- root, with `CAP_SYS_ADMIN` in the bounding set (a loop mount);
- access to `/dev/loop*`, so no `PrivateDevices`;
- the host-internal token as the `choir-internal-token` credential, with
  `CHOIR_INTERNAL_AUTH_MODE=enforce` (Node B's mode, nix/node-b.nix);
- a writable `HOME` for Dolt.

Because it holds the platform's internal token and its default paths are
Node B's, run it only on Node B, by the operator.

**Deploy order.** vmctl before checkpointd: the precheck needs vmctl's dry
run. Both ship in the same release, so one deploy brings both; a
checkpointd that meets an older vmctl refuses at the precheck.

```sh
sudo systemd-run --unit=choir-capture-base --collect --wait --pipe \
  -p Nice=19 -p IOSchedulingClass=idle -p CPUQuota=100% \
  -p MemoryHigh=6G -p MemoryMax=8G -p MemorySwapMax=0 -p OOMScoreAdjust=1000 \
  -p RuntimeMaxSec=3h \
  -p CapabilityBoundingSet="CAP_SYS_ADMIN CAP_DAC_OVERRIDE CAP_DAC_READ_SEARCH CAP_FOWNER" \
  -p LoadCredential=choir-internal-token:/var/lib/go-choir/secrets/internal-service-token \
  -p StateDirectory=go-choir/capture-base-home \
  -E CHOIR_INTERNAL_AUTH_MODE=enforce \
  -E HOME=/var/lib/go-choir/capture-base-home -E GOMEMLIMIT=6GiB \
  /var/lib/go-choir/services/checkpointd/bin/checkpointd capture-base \
    -computer <computer-id> -min-free-gib 24
```

It prints one JSON line with these fields: `vm_id`, `paused_ms`,
`cloned_ms`, `sequence`, `canonical_head`, `base_ref`, `blob_bytes`,
`origin`, `advertised`, `duration` and `error`.

**What a run does, in order.**
1. Takes the run lock (`/var/lib/go-choir/capture-base/.capture-base.lock`;
   a second run refuses), then sweeps what a killed run left: mounts under
   the mount root, and any `vm-state/<vm>/data.img.capture` that has no
   `capture-pause.json` beside it. An empty `-state-dir` is refused.
2. Before anything pauses, asks vmctl for a dry run (the VM and its
   owner), reads the tape head and the advertised watermark, and refuses
   when the head is not above the watermark. A dry-run answer that carries
   a `capture_path` is refused ("vmctl did not honor dry_run"): that vmctl
   predates the dry run and captured.
3. Asks vmctl for the capture: a durable pause marker, a pause, a reflink
   with a 30 s deadline and a resume watchdog, a resume confirmed by
   `GET /`, and an fsync of the clone. A marker that cannot be removed
   fails the handoff. An abandoned clone keeps the marker until FICLONE
   returns, and its cleanup removes only the files it made (inode check).
   While the capture holds the VM's operation lock, a StopVM waits for
   it: 80 s at worst (10 s pause call, 30 s clone, 3 x 10 s resume, 10 s
   probe), a few seconds when nothing fails. Logout and rescue, which
   hold the router lock while they stop a VM, wait as long.
4. Mounts the clone `loop,nosuid,nodev,noexec`.
5. Refuses a symlink, device, fifo or socket anywhere in the store tree.
6. Checks the head against the tape and drops prepared rows.
7. Checks free space inside the image, on the capture's filesystem and on
   the artifacts filesystem.
8. Compacts, witnesses, packs, reads back, and publishes the root.
9. Advertises it. corpusd records the epoch root because the sidecar says
   `live_captured`.
10. Unmounts, then removes the clone.

On an advertisement error the watermark is read back. The base is
retracted on a definite answer that it is not advertised: an absent, lower
or higher watermark, or another base at its sequence. corpusd records only
a higher sequence, so a higher watermark means this base never will be;
a `live_captured` sidecar kept then would raise the host rebuilder's floor
and block the computer's jobs (review 366).

**On "advertise outcome unknown"** (the readback failed), the base is kept,
because the POST may have landed. Once corpusd answers, read
`GET /internal/computers/files/watermark?computer_id=...` once. If it does
not name the base's `base_ref`, remove the base's blob and its
`.descriptor.json` sidecar; a sidecar left behind keeps the floor.

**A vmctl that dies inside the pause.** The marker stays behind. At
reattach, vmctl resumes the VM, confirms it with `GET /`, and removes the
marker and the clone. Resuming continues the same guest process: it is not
a boot, so the restart rule does not apply. If a boot finds a marker, it
clears it.

**d@e proof first** (`computer-59c902c5d46caecc0a180a8557164e72`,
`vm-70070cf7337d45f2d46eafc83b9783b9`). Run the command and record:
- `paused_ms` and `cloned_ms` from the report, and the vmctl journal line
  `data image capture ... paused`. Expect under 1 s for a small image.
- The ext4 journal replay at mount: `journalctl -k`, looking for
  `EXT4-fs ... recovery complete` or `mounted filesystem`.
- Compaction before and after (the `capture: compacted` line), and the
  total duration.
- Readback: `GET /internal/computers/files/watermark?computer_id=...` names
  `base_ref`, and corpusd's `computer_projection_epoch_roots` row names it.
- The guest stays healthy after the pause: its health endpoint answers, and
  no restart or interrupted work appears in its journal.
- Cleanup: no `data.img.capture` and no `capture-pause.json` in the VM dir,
  no mount under `/var/lib/go-choir/capture-base` (`findmnt`), and no
  `.capture-base` or `.projection-base-readback-*` left behind.
- The next cadence job seeds from the root. Its base records `epoch_root`,
  and its verdicts judge normally.

**The paper** (`computer-1355b83bced1613eb048ee28d66688a8`,
`vm-1669020adcece51c2f5ff9b6df399ef5`). Run the same command after the
04:00 daily store compaction, while the store is near 22 GiB and the
capture's own compaction is close to a no-op.

Peak extra space on `/` is roughly:
- 22 GiB for the blob;
- 22 GiB for the readback;
- the reflink divergence: the guest's writes over the run, about 5 GiB/h,
  plus any compaction writes.

At 84 GiB free with a 24 GiB floor, that fits. Run before the 04:00
compaction, with the store at 35-37 GiB, and it refuses at a floor check.
That is the intended outcome, not a fault. The running cadence job on seed
`abacf6b0…` is superseded when the root is advertised, because it has
another seed and a lower watermark.

**Rollback.**
1. Revert the S2 commits (`git revert`).
2. Before the old build runs checkpointd, point the computer's watermark
   back at its last pre-root base, because a pre-S2 reader refuses the root
   descriptor:
   ```sql
   UPDATE computer_replay_watermarks
      SET watermark_sequence=<prior>, base_ref='<prior ref>'
    WHERE computer_id='<id>';
   DELETE FROM computer_projection_epoch_roots WHERE computer_id='<id>';
   ```
3. Base the job reset on that row.
4. The epoch-roots table can stay; an old build ignores it.
5. Retract the root's blob and sidecar only after nothing pins it.

To undo the root while keeping the S2 build, do steps 2 and 3 and also
remove the root's `.descriptor.json` sidecar (and every descendant's
whose `epoch_root` names it). The host rebuilder reads its floor from the
published sidecars, not from corpusd, so a sidecar left behind keeps the
floor.

Once the watermark points at a pre-root base again, the computer can still
replay the prefix, because nothing deleted it.

## S3 finding: Dolt table hashes match a tape replay (2026-10-28 01:0x UTC)

Measured on the agentcore checkpoint fixture: the live store and a fresh
tape replay (`ReconstructInto` into `OpenFresh`) at the same head, compared
by `DOLT_HASHOF_TABLE` per table. 24 of 26 tables match exactly; schema,
column tags, indexes and collation agree. The two that differ are
`computer_event_index` and `computer_event_projection_heads`. These are the
two tables whose wall-clock columns the v1 witness already ignores
(`prepared_at`/`finalized_at`, `updated_at`). A replay writes those columns
afresh, so the two tables can never match by construction. The live working
set is not committed (26 `dolt_status` rows), so an `AS OF HEAD` read after
the hold is not the state at H.

Decision: witness v2 hashes every table except those two. It names them as
operational tables: they are the tape's own bookkeeping, and the head H the
checkpoint is published at commits to it. On the fixture the v2 read under
the hold took 7.6 ms. The v1 row extract of the same store took 128 ms
(2.2 s on d@e).

Owner, 2026-10-28: "we shouldnt keep shipping code in off state"; "not on
for all computers but on for at least one". Rings decide reach, the code
doesn't. So v2 is the bind for every computer. The cadence's code-level
refusal of `critical_protected` and `public_platform`, and of `*` and
`class:` tokens, is removed. The node-b ring becomes d@e (R0) plus
`class:test` (R1).

## Consequences

Any rebuild, rematerialize or fresh realization of the paper replays from
event 34660, much of it V1-era tape (the vocabulary migration then runs in
full). Hibernation and snapshots for the paper (D3, D8) have no recent base
to stand on.

## Fix direction (not chosen)

1. Read the paper's job row (A or B).
2. Seed the paper's base near its head once: a one-time checkpointd run with
   raised limits in a window with disk headroom (the data image is at 93%),
   or the guest cadence for the paper after measuring the append hold on
   d@e.
3. Then the ordinary cadence keeps it current; the same policy for every
   computer, with hibernation and snapshots on top (D2, D3).

## 2026-10-28 06:14 UTC: the paper capture was OOM-killed in the witness

The S2 capture of the paper (unit `choir-capture-base`, started 04:16) paused
the paper for 989 ms, reflinked, mounted, and compacted the store (26.3 GB,
done 04:32:39). It then spent about 1h40m in the content witness and was
killed by the kernel OOM killer at 06:14:11 (`anon-rss:14512556kB`, cgroup
`choir-capture-base.service`, limits MemoryHigh 14G / MemoryMax 16G). The
paper itself was unaffected (Firecracker up throughout).

Cause, from code: `computerversion.queryTableContentHash` collects every
row of a table as a JSON string in one slice and sorts it before hashing. A
table's memory therefore scales with its full JSON size; the paper's largest
tables exceed host memory. Raising the cap does not fix this class (host has
31 GB; the store grows).

Fix (follows this record): an external merge sort. Sorted runs spill to zstd
files past 512 MiB and are k-way merged into the same sha256, so the digest
is byte-identical to the in-memory sort (existing witnesses still
re-verify). Spill directory: `CHOIR_WITNESS_SPILL_DIR`, default the system
temp dir. Killed-run leftovers: the loop mount was unmounted by FP0;
`data.img.capture` (a reflink) stays for the next run's sweep.

## 2026-10-28 07:45 UTC: what the killed capture left, and its disk cost

Two findings from cleaning up after the 06:14 OOM kill.

1. **The clone's compaction is expensive on disk.** The leftover
   `data.img.capture` measured 98.9 GiB total, **71.9 GiB exclusive** (btrfs
   `filesystem du`): the store compaction inside the clone rewrote most of
   its blocks, so they no longer share with the paper's `data.img`. Root
   free fell from 58G to 51G while it sat there, and the deploy of
   203be57db refused at its 60 GiB headroom preflight (run 40647471944).
   A capture of the paper needs about 75 GB free while it runs; the
   capture's `-min-free-gib` floor should reflect the clone's growth, not
   only the staged base.
2. **The capture's loop mount leaked into service mount namespaces.**
   go-choir-corpus-dolt, go-choir-platform-dolt and go-choir-corpusd
   restarted while the capture was mounted; their private mount namespaces
   copied the mount, so after the kill the loop device stayed attached and
   the clone's blocks stayed allocated. The capture should mount in its own
   private namespace (or with private propagation) so no other unit can
   pin it.

Action taken by FP0 (07:44): unmounted it inside those three namespaces
(`nsenter -m umount`), confirmed no `capture-pause.json`, and removed the
marker-less clone, as the capture's own start sweep would. Root free: 122G.

## 2026-10-28 10:30 UTC: the paper's first base is captured and published

Run on 203be57db (witness spill + capture inode pin), started 08:34:12,
`CHOIR_WITNESS_SPILL_DIR=/var/lib/go-choir/capture-base-home`, MemoryMax
10G, CPUQuota 300%:

- paused 1684 ms (clone 1684 ms); compaction 08:34 -> 08:37 (274 -> 2
  commits, 28.2 -> 26.5 GB); witness and staged readback 08:37 -> 10:30.
- result: `origin=live_captured`, `sequence=4526210`, `canonical_head
  ae3ace99d10c...44f4b6`, `base_ref 5a241cb0f411...690629`, `blob_bytes
  26459016704`, `advertised=true`, duration 1h56m30s.
- unit: 8G memory peak (the 06:14 run was killed at 14G), 46 min CPU, 1.5T
  read, 225.6G written, no kernel OOM kill.
- cleanup verified: no `data.img.capture`, no pause marker, no spill runs,
  no capture mount or loop device. Root free 99G.

The witness pass reads the 26 GB store roughly twice (1.5T read in all), so
the paper's capture costs about two hours and ~70 GiB of transient clone
growth. That cost is what an incremental base (S4) and a hash-only bind
(S3) are for; it also argues for separating the paper's content-addressed
objects from its authority tables (see the store-engine contention note in
the 10-28 handoff).
