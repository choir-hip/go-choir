# Two test computers hold 4 to 6 times a normal computer's disk, and no code ever removes what self-development leaves there

Recorded 2026-11-24 (host clock, 00:50Z to 01:10Z). Read-only investigation.
Nothing in the repo or on a server was changed. One side effect to name: the
first `GET /api/runtime/observability` on the drill computer woke it from
hibernation (its snapshot, 1.5 GB, was consumed and is gone; the computer is
active now). Labels: "drill" is the d@e account, "b@c" is the b@c account,
"owner" and "paper" are the other two.

## Problem

On Node B, under `/var/lib/go-choir/vm-state`, space actually held by each
computer's sparse `data.img` (apparent 32 GiB; the paper 128 GiB):

| Computer | vm dir | Held now | Note |
| --- | --- | --- | --- |
| owner | `candidate-fleet-e15cb...` | 2,056 MB | created 2026-07-17 |
| b@c | `candidate-fleet-bf04b...` | 8,865 MB | created 2026-07-17 |
| drill | `vm-70070...` | 13,297 MB (13.6 GB re-read after wake) | plus a 1.5 GB memory snapshot while hibernated |
| paper | `vm-1669...` | 58,104 MB | out of scope here |

A fresh computer holds 0.49 to 0.66 GiB (the template image is 510 MiB held;
section 4). The owner's hypothesis is that engineering capsules leave
artifacts behind. The lead's second hypothesis is that nothing returns freed
blocks to the host.

## Evidence

### 1. Inside the drill's data disk (read-only, `GET /api/runtime/observability?data_volume=detail`, 2026-11-24 00:52Z, twice)

Route: the guest's own bounded metadata walk (`internal/datavolume`), reached
through the public proxy with the drill's key passed from its file and
`X-Choir-Computer: computer-59c902c5...`. It opens no file and writes
nothing. No other read-only route to a directory listing exists (`choir
files` is put/get/mkdir only).

Guest filesystem (statfs): total 33,501,757,440; free 19,988,066,304;
**used 13.51 GB (12.58 GiB)**; reserved-for-root 1.73 GB.
Host: `du` of `data.img` 13,615,584 KiB (**12.98 GiB**) right after the wake.
So the host holds within 3% of what the guest uses; there is no large
reservoir of freed-but-unreturned blocks.

Walk by top-level directory (allocated, st_blocks):

| Path | GiB | Files | Note |
| --- | --- | --- | --- |
| `rematerialize-quarantine-20261115T041047Z` | 1.079 | 31 | a whole prior realization (state.db and workspace) |
| `rematerialize-quarantine-20261115T054339Z` | 1.077 | 30 | same, 93 minutes later |
| `choir-updater` | 0.895 | 1,236 | incoming 0.305, releases 0.305 (3 x 0.102), store 0.280 |
| `state.db` | 0.811 | 1 | the store |
| `files` | 0.531 | 13,379 | `Source/platform` 0.384 (seeded source) |
| `capsule-artifacts` | **0.509, a floor** | 30,056+ | **walk truncated**; all under `subjects/` |
| `state.image-blobs` | 0.104 | 10,015 | |
| everything else | 0.06 | | state-actor.db, texture, WALs, shot-jobs |
| walked total | 5.51 | | |

Not in the walk: ext4's own metadata. The empty template image (below) has
used = 269,847 blocks = 1.03 GiB with the 0.38 GiB source seeded, so fixed
metadata (256 MiB journal, 2,097,152 inodes x 256 B, group tables) is about
0.65 GiB. Walked 5.51 + 0.65 = 6.16 GiB against 12.58 GiB used:
**6.4 GiB of guest space has no name.** The only named directory the walk
could not finish is `capsule-artifacts` (see "why truncated" below).

Why truncated, read from `internal/datavolume/datavolume.go` lines 328 to
340: each top-level entry gets an equal share of the remaining wall time and
entry budget (25 s over about 30 entries, so about 0.8 s each). The walk
reported 58,694 entries in the first call and 87,622 in the second, 1.0 to
1.3 s elapsed, so the stop is the per-entry time share, not the 3,000,000
entry cap. `capsule-artifacts/subjects` holds one directory per content
digest and the report folds all of them into one row; it cannot say how many
there are.

Drill's data image over time (hourly growth meter,
`/var/lib/go-choir/growth-meter/2026-11.jsonl`, 553 readings):

- 29.15 GiB on 11-14 00:00Z, 11.4 GiB at 22:00Z the same day: the updater
  prune landed and ran at the next boot (the 11-13 record had measured
  choir-updater at 20.10 GiB). The host returned the space, so discard works.
- Since then a regular sawtooth: +2.5 to +3.3 GiB for one to three hours,
  then back to the floor (for example 11.6 to 14.3 and back on 11-15 12:00 to
  13:00Z, and 11.9 to 14.4 and back on 11-18 19:00 to 20:00Z; 18.1 GiB once on
  11-18 21:01Z). These are the store-copy and compact-copy transients, and the
  returns show the trims reach the host.
- The floor itself: 9.2 GiB (11-15 02:00Z) then 11.6 after the two
  quarantines (+2.4), now 12.7 to 13.0. The floor creeps about 0.17 GiB a day
  since 11-15.

Self-development volume on the drill (`choir self-dev list`, read-only):
302 operations since 2026-10-26, 291 with a frozen bundle (289 distinct
digests): 122 applied, 163 failed, 14 rejected, 3 rolled back. Recent days
run 5 to 21 operations a day. Each operation whose capsule changed the tree
runs `PersistGrantedCandidate` (below).

### 2. What writes to the data disk, and what removes it (from code)

The guest data volume is `/mnt/persistent` (`nix/autoputer-vm.nix`).

| Writer | Where | Remover | Verdict |
| --- | --- | --- | --- |
| Capsule overlay upper, work and scratch `/tmp` (`internal/capsule/executor.go:643`, `2714-2735`) | `capsule-state/boot-<id>/<capsule>/` | `removePrivateTree` at Destroy (`executor.go:1148`); boot sweep of earlier boots (`state_dir_linux.go:83`); paused ones removed by `pause_linux.go:427-537` | has remover |
| Source cache built once per digest (`source_cache_linux.go`) | `capsule-state/boot-<id>/capsule-source-cache` | keeps 4 most recent not in use (`sourceCacheKeep`, `:165`) plus the per-boot sweep | has remover |
| **Candidate subject: full copy of the frozen source tree** (`PersistGrantedCandidate`, `executor.go:1990-2050`, called from `agentcore/engineering_assignment_fate.go:1266` whenever the frozen digest differs from the base) | `capsule-artifacts/subjects/<digest>/workspace/platform`, files made read-only | **none.** The only `removePrivateTree` on that path is the `.candidate-*` temporary. No code under `internal` or `cmd` deletes `subjects/`. No doc records a retention rule. The subject-artifact record (2026-09-28) made it durable on purpose so verification can re-read it | **no remover** |
| Execution, fate and granted receipts (`executor.go:1458-1579`) | `capsule-artifacts/receipts/{execution,fate,granted}` | none | **no remover** (small files) |
| Egress ledger, one `.jsonl` per capsule id (`executor.go:903`) | `capsule-artifacts/egress/` | none | **no remover** (small) |
| Job records and output logs, content-addressed, each at most `MaxLogBytes` (`jobs.go:773-802`) | `capsule-artifacts/jobs`, `job-logs` | none | **no remover** (bounded per file, unbounded in count) |
| Rematerialize quarantine: the prior state.db, WAL and workspace, renamed aside (`agentcore/rematerialize.go:232-279`) | `/mnt/persistent/rematerialize-quarantine-<time>` | none: the name appears only at the creation site | **no remover**; 1.08 GiB each on the drill |
| Workspace-replace quarantine (`agentcore/workspace_replace.go:56`) | `workspace-replaced-<time>` | none | **no remover** (none present on the drill now) |
| Updater incoming, store, releases | `choir-updater/` | `choir-updater prune` once per boot (`internal/updater/prune.go`, `nix` unit `go-choir-updater-prune`), keeps current plus at most two predecessors | has remover; 0.9 GiB now vs 20 GiB on 11-13 |
| Store compaction copy `state.db.compact-staging` (`internal/store/compact.go:104`) | beside `state.db` | swapped in at a planned restart or discarded; not present now | has remover; transient up to the store size |
| Snapshot capture scratch copy of the store (`internal/store/store_copy.go`, deleted by the S3 lane) | beside `state.db` | deleted after capture; S3 removes the copy | transient, returned by trim |
| Go module and build caches | env set to `/mnt/persistent/go` and `/mnt/persistent/go-build-cache` (`autoputer-vm.nix:1231-1232`) | none, but **not written**: the drill's walk shows neither directory. Capsule builds run with `HOME=/root`, `TMPDIR=/tmp` inside the capsule (`broker_env.go:24`), so their caches live in the per-capsule upper and die with it | not a source of growth |
| Nix store paths | the guest's `/nix/store` is the read-only erofs store drive and the four tool-layer images; host-built releases enter through `choir-updater` | the host's own gc | not on the data disk |
| Checkpoints, candidate VMs | host side (`artifacts/computer-checkpoints`) or retired (Choir-in-Choir, candidate VMs are not built) | n/a | not on the data disk |
| Files the engineer files for the owner, `files/Self-development/<op>/screens` | `files/` | none (user data) | 0.53 GiB on the drill |

Writers with no remover that a capsule or self-development causes:
`capsule-artifacts/subjects`, `receipts`, `egress`, `jobs`, `job-logs`,
`rematerialize-quarantine-*`, `workspace-replaced-*`.

### 3. Discard: it is on, and it works

- Every computer's launch config on Node B has the data drive with
  `"discard": true, "io_engine": "Sync"` and Firecracker 1.17.0 (all four
  `fc-config.json` read; `nix/node-b.nix:1718-1719` sets `VM_DATA_DISCARD=1`
  and `VM_DATA_DISCARD_ONLY=*`; `internal/vmmanager/data_discard.go`; the
  design record is `docs/problems/guest-data-image-never-trims-2026-10-20.md`,
  which also holds the trial proof). The trim is in the guest unit
  `choir-data-fstrim` (`nix/autoputer-vm.nix:1002-1036`, ten minutes after
  boot, then hourly, idle IO class). A grep of the tree finds both.
- The unit's journal line is not on the console (searched all four consoles
  on the host: no `fstrim` text, which the 2026-11-13 clustering assessment
  also found), so a direct guest-side `fstrim -v` result is not seen. The
  evidence is behavioral: the 11-14 return of 17.7 GiB, the sawtooth returns
  in section 1, and host 12.98 GiB against guest 12.58 GiB.
- Host: `/` is btrfs, `compress=zstd:3`, `discard=async`, 476 G, 154 G free,
  68% used. A guest trim becomes a hole punch in a regular file
  (`data_discard.go`). The guest ext4 mounts with no `discard` option; the
  periodic trim was chosen on purpose (record 2026-10-20).
- **Refuted: "an image only grows to its high-water mark".** The drill's image
  fell from 29.15 to 11.4 GiB and swings back each night. What the host
  holds is what the guest holds, plus the 1 to 3 hour transients.

What would return more, and the risks:

| Mechanism | Frees | Risk |
| --- | --- | --- |
| More or faster fstrim, or `discard` mount option | nothing: freed blocks are already returned; the hold is live data | mount `discard` adds latency to every delete (the 2026-10-20 record's reason) |
| Host hole punch or `fallocate --dig-holes` while the computer is stopped | nothing: ext4 does not zero freed blocks, so a zero scan finds no hole | none, but useless |
| Host `e2fsck -E discard` or a loop-mount plus `fstrim` of a stopped image (the deleted host compactor, removed 2026-10-30) | only what the guest's own hourly trim missed (about nothing now) | must be a cold-stopped image with no memory snapshot: `snapshotBoundPaths` and `diskIdentity` (`snapshot.go:482-519`) bind a snapshot to `data.img`'s size, mtime, inode, so a changed image refuses the resume and cold boots; a reflinked `data.img.capture` or hard link (jail, `confinement.go:129`) shares extents, so punching one file frees nothing until the other goes; on compressed btrfs extents a partial punch frees nothing until the whole extent is punched (fp0 record 2026-10-22) |
| Lane `snapshot-capture-s3` | removes the scratch copy of the store: shrinks the 2.5 to 3.3 GiB transients on the drill and 40 GiB on the paper | none for this problem; it neither adds nor needs a trim path. A GC that deletes files must not run during a capture (the same restart-and-pin rule) |

### 4. Budget for a 2 GiB-memory tester after a week, no self-development

Sources, all measured on Node B:

- Empty template image, `.data-image-templates/data-50eab21d...img`: 510,164
  KiB held (**0.49 GiB**), guest used 1.03 GiB (includes the seeded source).
- Fresh test computers in the growth meter sit at 0.49 to 0.66 GiB and stay
  flat while idle: `vm-90c9b5bb6` 0.49 GiB for 21 days, `vm-9b5b54a2b` 0.49 for
  19 days, six computers 0.53 to 0.66 for one to two days.
- Each platform deploy steps a computer by +0.12 to +0.16 GiB
  (`vm-33c2cb67f`, `vm-f1b7d0a35`, `vm-bf8b7d2b4`: 0.58, 0.71, 0.87 across two
  refreshes), which the boot prune now bounds at current plus two
  predecessors: the drill's whole updater directory is 0.9 GiB.
- The owner's computer, 129 days old and in daily use, holds **2.06 GB** with
  a 32 GiB image and 8 GiB of memory.
- Memory snapshot while hibernated: the drill's, from an 8 GiB machine, was
  1.5 GB held (touched pages; `new-computer-creation-14s` record: 1.15 to 1.37
  GiB resumed working set). A 2 GiB machine cannot exceed 2 GiB.

Budget, by source: baseline 0.5 GiB + one week of deploys 3 x 0.15 = 0.45 GiB
(capped by the prune near 0.9 GiB of updater) + store and files for a week of
ordinary use, taken from the drill's non-self-dev directories (state.db 0.8,
files 0.5, blobs 0.1 for a computer with 170k events) but small for a tester
=> **about 1.5 GiB typical, 2.1 GB the observed upper end for a long-lived
ordinary computer**; plus up to 2 GiB of snapshot while hibernated; plus a
transient of one store-size during a snapshot capture or compact copy
(measured 2.5 to 3.3 GiB on a 0.8 GiB store, because the copy's temporary
files add to the full store). Proposed per-computer ceiling for an ordinary
tester: **3 GiB held data.img, 5 GiB including a snapshot**, with an alarm
above it. This is an estimate: no 2 GiB-memory computer exists to measure, the
one-week growth of a real tester was not observed (the fresh ones were idle),
and the owner's number was taken from one computer.

## Belief

1. The owner's hypothesis is **partly true, and the part that is true is
   large only on the drill and b@c**. Capsules clean their own state. What
   stays is what self-development deliberately makes durable (candidate
   subjects, receipts) and what the rematerialize path moves aside
   (quarantines), none of which any code ever removes. p = 0.8 that this is
   what distinguishes the drill (13.3 GB) and b@c (8.9 GB) from the owner
   (2.1 GB), who has no recorded self-development volume of that kind.
2. Of the drill's 13 GiB: about 2.2 GiB is the two quarantines (measured),
   about 0.65 GiB ext4 metadata (derived), about 3 GiB is ordinary state and
   releases (measured), and about 6.4 GiB is unnamed. The best single
   candidate for the unnamed part is `capsule-artifacts/subjects`: the walk
   saw 0.51 GiB in 30,056 files before it stopped, 291 frozen bundles exist
   on the drill, the 11-15 floor of 9.2 GiB predates the quarantines, and
   nothing deletes a subject. p = 0.6 that subjects plus receipts, egress
   and job logs hold at least 4 GiB of the 6.4. The other 0.4 is open
   (deleted-but-open files, directories the walk did not name, my ext4
   estimate being low).
3. b@c at 8.9 GB: the same writers, fewer operations (p = 0.6). It was not
   queried; the instruction allowed only the drill.
4. The lead's hypothesis (no space returns to the host) is false for the
   fleet as built. p = 0.95: the config, the 17.7 GiB return and the daily
   returns all agree. It was true until 2026-10-22 and was the cause of the
   paper's earlier growth (the 2026-10-20 record).
5. What fills an ordinary computer is deploys, bounded by the boot prune, and
   the store. Nothing about capsules.

## Remaining error

- The 6.4 GiB residual on the drill is inferred, not seen. The measurement
  that settles it: one yellow change to `internal/datavolume` that labels
  `capsule-artifacts/subjects` children by digest and divides the walk's time
  by entry weight, not equally; then read the drill again. Prediction to
  record before the read: more than 40 subject directories and 4 GiB or more.
- Whether every subject is still needed. A subject is read back by
  `PreflightSourceSnapshot` for any open or resumed assignment and verification
  (`executor.go:430-440`) and by reconcile at boot
  (`capsule-subject-artifact-ephemeral-2026-09-28.md`). The refs live in
  assignment and operation records; which of them are terminal and which an
  applied operation's rollback still reads has not been read from the store
  code. A collector must not guess this.
- Whether a rematerialize quarantine is read by anything. Not found in code;
  its purpose is the safety net of a failed flip. The owner decides the
  retention.
- b@c and the owner were not read from inside. Their breakdown is unknown.
- I did not see the guest's trim service run (no journal access). The
  behavioral evidence is strong, not direct.
- The 30,056-file floor may be an undercount of the other truncated entries
  too: only `capsule-artifacts` reported `truncated`.
- Whether the floor's creep (0.17 GiB a day on the drill) is subjects (about
  10 operations a day) or state growth.

## Ranked garbage-collection options

Class and proof use the repo's table. Frees are for the drill unless said.

1. **Measure first (yellow, rollback: revert).** `datavolume`: rows by digest
   under `capsule-artifacts/subjects`, rows for `receipts`, `egress`, `jobs`,
   `job-logs`, and a time share by entry weight so no top entry is a floor.
   Frees nothing. Proof: the drill read after deploy names the 6.4 GiB or
   shows it is elsewhere. Do this before 2 and 3; if the prediction fails,
   stop and re-read.
2. **Subject collector at boot (red: verification-subject authority;
   rollback: revert, but a deleted subject is gone, so keep a retention
   window).** Run in the existing boot oneshot pattern that
   `go-choir-updater-prune` proved (nothing else running; no concurrency
   design): delete a `subjects/<digest>` that no open assignment, no
   non-terminal operation and no applied operation's rollback target
   references, and that is older than 7 days; delete receipts, egress and
   job-logs by the same age and a no-live-reference check. Frees: probably 4
   to 6 GiB on the drill, similar on b@c, nothing on the owner. Proof, on the
   drill only: one boot; `data_volume` before and after; `choir self-dev list`
   unchanged; one new self-development operation freezes, verifies and
   reconciles across a planned restart (`resume` path); host `du` returns to
   the new floor within 3 hours. Needs the two-seat review the repo requires
   for red.
3. **Quarantine retention (red: recovery material; rollback: none after
   deletion, so the owner names the window).** Delete
   `rematerialize-quarantine-*` and `workspace-replaced-*` after the
   successor realization has run a planned and a crash boot and is older than
   7 days, keeping the newest one. Frees 1.08 GiB now (the older of the two on
   the drill), 2.16 GiB if the owner keeps none. Proof: next rematerialize on
   the drill (it is a drill-account act) leaves one quarantine and the old
   one is gone after a boot; the store's witness still matches.
4. **Store the candidate as a patch plus the base commit, not a tree copy
   (orange to red; rollback: revert, readers fall back to the tree).** The
   resume path already re-derives a subject from a pinned commit
   (`capsule-source-git:<commit>:sha256:<digest>`, `executor.go:417-437`);
   the exported `source.patch` is sha256-bound. A subject becomes kilobytes,
   which removes the growth instead of collecting it. Larger change: every
   reader of `subjectArtifactPath` must reconstruct. Frees: the same space as
   option 2, and stops the 0.17 GiB a day. Proof: digest of the
   reconstructed tree equals the frozen digest in a test, then the drill as in
   option 2. Do after 2 so the collector exists for old subjects.
5. **A per-computer ceiling alarm (yellow, rollback: revert).** Growth meter
   already reads each `data.img` hourly; add a class-based line at 3 GiB held
   for an ordinary computer and 8 GiB for a self-development computer. Frees
   nothing; it would have shown b@c and the drill months ago. Proof: the line
   fires on the drill today.
6. **Fold the small ones into the option 2 pass (orange).** `capsule-artifacts/egress`
   and `jobs` ledgers per capsule id (one `.jsonl` each). Frees under 0.1 GiB;
   do it only because it is the same loop.
7. **Not recommended:** a mount `discard` option (latency on every delete,
   frees nothing more); a host hole punch or zero scan of a stopped image
   (ext4 does not zero freed blocks; frees nothing); loop-mount `fstrim` on a
   stopped image (the compactor was deleted for cause on 2026-10-30; an image
   with a snapshot would refuse to resume); shrinking the 32 GiB image (the
   capsule floor is 4 GiB free: `StateFloorBytes`; and a smaller image would
   not change what is held, since the image is sparse); any change to
   Firecracker settings (they are right).

## Claims to enter in the calibration ledger before the measuring change reads

- C-new-a: `capsule-artifacts` on the drill holds at least 4 GiB; p = 0.6;
  test: option 1's per-digest read; settle by the deploy of that change.
- C-new-b: after option 2 on the drill, host `du` of `data.img` returns to
  at most 9 GiB within 3 hours of the boot; p = 0.55; test: the growth meter.
- C-new-c: a new 2 GiB tester after a week of ordinary use holds at most
  3 GiB; p = 0.7; test: the growth meter for the first tester.


## Owner ruling, 2026-11-24

"i like measuring. i dont like running the collector at boot. should wait some time after boot." Option 2 is therefore not a boot-time collector: it runs after the computer has been up and idle for a set time, never in the boot path. The other options are agreed in the order given.

## Update 2026-11-24 (lane `disk-report`, not landed)

Option 1 is built: the data-volume report names `capsule-artifacts/subjects`
by digest, `receipts` by kind, gives every row an oldest and newest change
time, and no longer cuts the first large directory at an equal share. The
claims above are entered as C206, C207 and C208. The investigator's wake came
from the proxy's generic route, which wakes a hibernated computer for any
bearer request; the detail read now answers 409 instead. A cookie session
still wakes. The report runs inside the guest, so a hibernated computer cannot
be read without waking it, only left unread.
