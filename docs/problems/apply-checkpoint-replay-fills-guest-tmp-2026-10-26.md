# An apply's checkpoint replays the whole tape into the guest's /tmp, fills it, and kills the runtime (2026-10-26)

Status 2026-10-30: still open as a disk-fill concern, but the Dolt replay-into-tmp wording is stale: guests now run SQLite and recover from their own sealed snapshots plus the tape tail. Re-check against that path. Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

Status: documented; no fix chosen. Mutation class of a fix: red (self-development
apply, checkpoint/witness, boot path). Found by FP0 reading vmctl's log and the
guest console after b@c stopped answering.

## What happened (b@c, computer-6c50d6e5…, VM candidate-fleet-bf04bbc7…)

- 11:18:03 UTC: FP0 approved Reader round 7 (selfdev-83fbdfa9). The operation went
  to `materializing` at 11:18:10.
- 11:19:05: the runtime (pid 2866, up since the 10:51:46 boot) opened a second store
  under `/tmp/choir-replay-completeness-*` and began replaying the computer event
  tape into it: page fetches after=36377 … after=44569 (11:20:07 to 11:22:46, 2-3.4 s
  per 1024-event page).
- About 11:26:34 (console uptime 2088 s): Dolt `errors.Fatalf`, "fatal error: write
  /tmp/choir-replay-completeness-786065405/runtime.texture/texture/.dolt/noms/…: no
  space left on device". A Dolt Fatalf is not recoverable: the runtime process died.
  Stack: `selfdevReconcileDrain` → `reconcileSelfDevelopmentMaterialization` →
  `materializeSelfDevelopmentOperation` (self_development_materializer.go:481) →
  `recordMaterializationApplied` (:683) → `bindCheckpointAtHead`
  (checkpoint_restore_bindings.go:54) → `openReplayProbe` (replay_completeness.go:318)
  → `ReconstructThroughTarget` → `FinalizeReplayBatch` (store/computer_events.go:88,242).
- The temp root is removed only by the probe's own close (replay_completeness.go:245),
  which never ran, so /tmp stayed full: at 11:30 and later the guest's updater could
  not write `/tmp/choir-overlay-probe-*` either ("no space left on device").
- vmctl: health failed from 11:26:37; route kept active during grace; the 11:30:25
  deploy refresh to train 10 was deferred ("guest did not answer the planned-restart"),
  then skipped (state degraded); 11:35:28 "wedged VM … refused connections for 5m0s;
  stopped, it boots again on resume". b@c reads `stopped` (epoch 39).

## Why now

The same path ran for the window-manager applies at 09:01 and 09:28 and passed. The
probe replays the entire tape on every apply, so its scratch grows with the tape;
b@c's tape crossed what its /tmp holds (about 45k+ events). Every later apply on b@c,
and on any computer whose tape is that large, fails the same way.

## Scope beyond b@c (to check before any fix)

`bindCheckpointAtHead` has three callers: the self-development apply
(self_development_materializer.go:683), rematerialize (rematerialize.go:484) and the
platform update path (platform_update.go:388). If the platform update path runs on a
computer with a long tape (the paper's tape is far larger than b@c's), it would hit
the same limit. Not yet traced whether platform_update runs on the paper today.

## Facts the fix must respect

- A full replay per apply is O(tape) work and scratch; the cost grows without bound.
- The probe's scratch sits in the guest's /tmp and is not cleaned when the process
  dies; a dead probe leaves the computer unable to write /tmp at all.
- Dolt's Fatalf cannot be recovered in process; the replay must not be able to
  exhaust the volume the live runtime needs.
- Restart rules (CLAUDE.md): the crash boot closes the open operation; nothing
  resumes it.

## Candidate directions (not chosen)

1. Bound the probe: scratch on the persistent volume with a size check before start,
   refuse (named error, apply still recorded, checkpoint deferred) rather than crash.
2. Incremental witness: replay only from the newest pinned replay base
   (c374be0f9 pins replay bases) instead of from genesis.
3. Clean the scratch at boot (stale choir-replay-completeness-* roots).

## Addendum 12:3x: the crash boot repeats it (a crash loop)

b@c was restarted at 11:50 (epoch 40). At 11:58:21 its runtime died the same
way: `fatal error: write /tmp/choir-replay-completeness-1700721818/...: no
space left on device`. vmctl stopped it as wedged at 12:07:24
(console.log lines 3437 to 3571; vmctl journal 12:05 to 12:07).

The boot path that ran the replay is the crash settlement, not a new apply:

```text
triggerSelfDevelopmentReconcile -> selfdevReconcileDrain
 -> reconcileSelfDevelopmentMaterialization (self_development_materializer.go:233)
 -> settleSelfDevelopmentOperationAfterCrash (:182)
 -> materializeSelfDevelopmentOperation (:481)
 -> recordMaterializationApplied (:683)
 -> bindCheckpointAtHead (checkpoint_restore_bindings.go:54)
 -> openReplayProbe (replay_completeness.go:318) -> ReconstructThroughTarget
```

The boot log at 11:50 correctly said the apply is "interrupted; settles from the
updater journal, never re-applied". But settling from the journal records the
materialization as applied, and that recording binds a checkpoint at head,
which replays the whole tape (about 57k events by 11:58) into /tmp. So every
boot of b@c re-runs the step that killed it, about eight minutes in. It is a
crash loop under the restart rule's letter: no work is resumed, but a boot-time
settlement repeats an unbounded step. The updater's own overlay probe then
fails too (`/tmp/choir-overlay-probe-*: no space left on device`), so /tmp is
shared between the runtime's scratch and the updater's probes.

Consequences:

- b@c must stay stopped until the fix covers the crash-settlement path; a
  restart reproduces the crash.
- The fix must bound the replay's disk (or move the checkpoint bind off the
  apply-recording path) AND make the crash settlement idempotent with a
  bounded cost. A settlement that crashes the computer twice should close the
  operation with that cause, as the restart rule closes a schedule whose tick
  crashes twice.
- selfdev-83fbdfa9 stays `materializing` until a boot settles it.

## Investigation (2026-10-26, lane apply-replay-bound; read-only, from code)

### (a) Where the guest's /tmp lives, how big it is, and what a probe writes there

- **/tmp is RAM.** The guest declares no root filesystem
  (`nix/autoputer-vm.nix` has no `fileSystems."/"`), so microvm.nix's default
  applies: `/` is `tmpfs` with `size=50%` of guest memory at mount
  (microvm.nix `nixos-modules/microvm/mounts.nix:95-101`). The runtime unit has
  no `PrivateTmp` (`nix/autoputer-vm.nix:1083-1101`), so the probe writes the
  shared root tmpfs that the updater and every other guest service also use.
- **Size.** Interactive guests run 4 CPU / 8192 MiB on Node B
  (`nix/node-b.nix:1640-1641`, read by `interactiveMachineShape`,
  `internal/vmctl/ownership.go:1333`): /tmp is about **4 GiB**, taken from the
  same 8 GiB the runtime needs. Platform computers (the paper) run 4096 MiB
  (`platformVMMemSizeMib`, `ownership.go:1965`, chosen by
  `machineShapeForOwnership`, `:1323`): their /tmp is about **2 GiB**. The
  persistent data volume is `/mnt/persistent`, a virtio-blk ext4 image of at
  least 32 GiB (`dataImageSizeMB`, `internal/vmmanager/manager.go:77`; the nix
  `size = 2048` at `autoputer-vm.nix:322` is only the build-time artifact). The
  live store is there (`RUNTIME_STORE_PATH=/mnt/persistent/state`,
  `autoputer-vm.nix:1018`), and rematerialize already stages its replay beside
  it (`rematerialize.go:89`).
- **What the scratch holds.** The probe does not replay from genesis on a
  computer the size of b@c: with an advertised base it installs that base and
  replays only the tail (`openProbeReplayStore`, `replay_completeness.go:448-471`);
  without one it refuses any chain over 10,000 events
  (`MaxRecoveryTailEvents`, `recoveryplan/recovery_plan.go:15`, checked at
  `replay_completeness.go:454`), so a 45k chain cannot have taken the genesis
  path. The scratch is therefore (1) the base blob, downloaded whole into the
  scratch (`projectionbase/install.go:124`), (2) its unpacked copy, made while
  the blob still exists (`install.go:146-157`; the blob is an uncompressed tar,
  `publisher.go:246-263`), so the install peaks at **twice the blob**, and (3)
  the tail replay's Dolt journal growth, one prepare and one finalize per event
  (`store/computer_events.go:88`, the same per-event path as live writes).
- **Bytes.** No test or evidence file measures a probe scratch directly. The
  nearest measurements: compact projection bases are about 1.7 GB
  (`nix/node-b.nix:1316`); the per-event journal cost of the same write path is
  141 KiB per event on the local tape harness and 220 to 310 KiB per event on
  the paper (`docs/archive/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md`,
  sections "Correction" and the 10-15 cross-check). So a probe needs about
  **2 x blob + 0.14 to 0.31 GiB per 1k tail events**. The b@c log lines
  (`after=36377 … after=44569`) are the slow-page log, printed only when a page
  takes over 2 s (`computerevent/appender.go:970-972`), so they bound the tail
  from below (at least 8 pages, about 8k events, about 1.1 to 2.5 GiB) but do
  not give the watermark. With a base of order 1 GB the install peak alone is
  about 2 GB: 4 GiB of tmpfs does not hold it plus the tail. On the paper's
  2 GiB /tmp the base install alone would not fit.

### (b) Does platform_update.go:388 run on long-tape computers such as the paper today?

No automatic path reaches it. The paper is not at risk from it today.

- Its only entry points are `POST /internal/runtime/platform-update`
  (`internal/apihandler/routes.go:57`, `platform_update.go:778`) and the boot
  sweep (`runtime.go:949-955` → `settleOrResumePlatformUpdate`,
  `platform_update.go:947`). The POST has had no caller since b9f195b1c deleted
  the station probes (`docs/archive/reports/surface-audit-cli-mcp-api-2026-10-14.md:307`;
  a repo grep finds no script, nix unit or Go caller). Deploys reach computers
  through vmctl's planned refresh, a different mechanism (marker plus base
  refresh), including the busy-refresh of `critical_protected` and
  `public_platform` computers; it never calls `ApplyPlatformUpdate`.
- The boot sweep acts only on a pending update-accepted transition
  (`pendingPlatformUpdate`, `:1036`) or a stranded `platform-update-applied-*`
  event without its route sibling (`resumeStrandedPlatformUpdateTail`,
  `:881-927`), and even then it first fetches an already-minted checkpoint by
  key and binds only if none exists (`:364-375`). The paper reboots on every
  deploy; a strand on its tape would have run this probe on every one of those
  boots, so the absence of any paper /tmp exhaustion is weak evidence there is
  none. Not verified against the paper's tape (this lane has no computer access).
- The paper **is** exposed to the same probe through manual acts: an owner
  checkpoint bind (`rematerialize.go:438,484`), the operator stamp heal
  (`selfdev_stamp_heal.go:78,111`), the replay-completeness API
  (`api_self_development.go:148`) or a self-development apply on it. Each would
  install a ~1.7 GB base into a 2 GiB tmpfs. None runs on a schedule; the fix
  below covers all of them because it changes `openReplayProbe`, their one
  shared entry.

### (c) What "settles from the updater journal, never re-applied" means for selfdev-83fbdfa9

- The apply wrote its planned-restart marker before calling the updater and
  cleared it when the updater returned (`markPlannedRestart`,
  `planned_restart.go:310-336`). The Fatalf came later, inside
  `recordMaterializationApplied`, so every boot since is markerless: a crash
  boot.
- At a crash boot, `markSelfDevelopmentOperationsInterruptedAfterCrash`
  (`self_development_materializer.go:128-160`) marks the `materializing` op
  interrupted, and the boot-phase reconcile (`runtime.go:961`) calls
  `settleSelfDevelopmentOperationAfterCrash` (`:164-187`). The updater journal
  for `selfdev-apply-<decision>` is terminal `applied` (the crashed pass had
  already verified its receipts, `:478-481`), so the settlement takes the
  default branch: `materializeSelfDevelopmentOperation(..., journalOnly=true)`
  → `recordMaterializationApplied`. The `materialization-applied` event is
  already on the tape (appended before the checkpoint, `:596-618`, deduplicated
  by idempotency), and the settlement then calls `bindCheckpointAtHead` again:
  **the same probe, the same scratch, the same /tmp.** "Never re-applied" holds
  (the updater is not called), but the checkpoint is re-attempted.
- So it will not settle by itself on b@c as built: each crash boot re-runs the
  probe that killed the runtime. The journal-only path spends no apply attempt
  (`applyOrReadJournal`, `:983-996`, skips `markPlannedRestart`), so nothing
  bounds the repetition except the unit's start limit (5 starts in 10 minutes,
  `autoputer-vm.nix:928-930`) and vmctl's wedge stop. **Hypothesis, not
  confirmed by a trace after 11:50:** the 11:50 boot re-entered this loop; a
  console read of b@c for a second `choir-replay-completeness` Fatalf would
  confirm it.
- The owner's rollback does not escape it: a rollback needs a prior checkpoint
  and the current applied head (`startSelfDevelopmentRollback`,
  `api_self_development.go:824-870`), and its own apply records through
  `recordMaterializationApplied` (`:563`), which binds a checkpoint through the
  same probe. The op needs the fix, not an owner act. Once the probe fits (or
  refuses by name), the next crash boot or reconcile pass settles it.

### (d) Does a pinned replay base let the probe replay incrementally?

No, not beyond what it already does.

- The probe already starts from the newest base it can verify: the advertised
  watermark (`resolveRecoveryTarget` + `installStagedBase`,
  `replay_completeness.go:463-471`; `restore_base.go:54-67`). It replays from
  genesis only when no base was ever advertised, and then only for chains of at
  most 10,000 events (`:448-461`). So "the apply path replays from genesis" is
  not what happened on b@c: it installed a whole base and replayed the tail.
- c374be0f9 pins the base a checkpoint's replay started from
  (`pinCheckpointBase`, `restore_base.go:157-171`, called at
  `checkpoint_restore_bindings.go:101`) so that a *restore* to an older
  checkpoint can install it (`newestPinnedBaseAtOrBelow`, `restore_base.go:129-154`).
  A pinned base is a base that was once advertised; checkpointd only advances
  the watermark. The newest pin is therefore never newer than the advertised
  base, and switching the probe to it can only lengthen the tail.
- The probe's cost is O(store) + O(tail) per apply, not O(tape): it copies the
  whole projection (twice, at the install peak) every time. A truly incremental
  probe would keep a retained replay store between checkpoints and advance it
  by the tail only. That is new persistent state (a second copy of the store on
  the data volume) with its own trust and corruption story; it is a design, not
  a patch, and is not chosen here (see the design section).

## Fix design (chosen 2026-10-26, lane apply-replay-bound)

Mutation class: **red** (checkpoint/witness computation site, self-development
apply error handling, boot path). Review tier 2 by the operating contract (boot
path, a new goroutine); the lane lands the commits on its branch and leaves the
review to the orchestrator.

### The options, judged against the investigation

1. **Bound and relocate the scratch.** Chosen. The replacement already exists:
   rematerialize stages its replay beside the live store on `/mnt/persistent`
   (`rematerialize.go:89`). The probe is the one replay that still uses
   `os.MkdirTemp("", …)`.
2. **Incremental replay from the newest pinned base.** Rejected: (d) shows a
   pinned base is never newer than the advertised base the probe already starts
   from, so it would only lengthen the tail.
3. **Boot-time removal of stale `choir-replay-completeness-*` roots.** Chosen,
   as the complement of 1: a probe killed in flight leaves no scratch behind
   into the next start.

The substrate cause is that every probe copies the whole projection (O(store),
twice at the install peak) and writes it to RAM. Option 1 removes the RAM
coupling and makes the cost admitted and bounded; it does not remove the
O(store) copy. The O(store) copy goes away only with a retained replay store
advanced by the tail between checkpoints; that is new persistent state and a
design of its own (residual R1 below), not a patch to land today.

### The mechanism

- **Where.** The probe's root moves to `<dir of the live store>/replay-probe/`
  (`/mnt/persistent/replay-probe/choir-replay-completeness-*` in a guest), on
  the 32 GiB data volume instead of the RAM tmpfs.
- **Admission, before any work.** Before the live capture (which holds appends
  and hashes the live store), the probe estimates its scratch: for a base,
  `2 x blob_size_bytes` (download plus unpack) plus `tail x 320 KiB`; with no
  advertised base, `sequence x 320 KiB`. 320 KiB per event is the top of the
  measured 220 to 310 KiB. It refuses with the named error
  `ErrReplayScratchInsufficient` unless free space covers the estimate plus a
  reserve of `max(2 GiB, 10% of the volume)`. An estimate it cannot make (base
  source unreachable) still requires the reserve; the probe's own base refusal
  then reports the source error as before.
- **Bound, in flight.** The live store shares the volume and keeps writing
  while the probe replays, so the estimate is not enough. A watchdog reads free
  space once a second while the probe holds its scratch and cancels the
  probe's replay context with `ErrReplayScratchInsufficient` when free space
  falls under the reserve. Replay checks its context after every event
  (`computerevent/appender.go:989-997`), so it stops within one event and one
  small commit. The scratch is removed by the probe's close as today. Dolt's
  Fatalf is reached only if more than the reserve is written in one second.
- **Stale roots at boot.** The first thing `Start` does (before the
  maintenance-hold check, which must not keep a full volume full) is remove
  every `choir-replay-completeness-*` root under the new parent and under
  `os.TempDir()` (the old location: a runtime restart inside one VM boot keeps
  tmpfs). No probe can be running then: the probe is in-process and the
  runtime is single-instance per guest.
- **The apply when the checkpoint cannot be bound.** The
  `materialization-applied` event is already on the tape before the checkpoint
  is bound (`self_development_materializer.go:596-618`); that is the apply's
  record. On `ErrReplayScratchInsufficient` the materializer stamps the
  operation (`RecordIntentError`, the owner-visible stamp; a stable text, so
  an unchanged refusal appends nothing) with "checkpoint deferred: …" and holds
  the operation in `materializing` without arming the 60 s retry wake. The
  route is not promoted without a checkpoint (unchanged contract). The next
  reconcile pass (any commit, or a boot) tries again, cheaply refused by the
  admission check until space exists. The same hold applies to the crash-boot
  settlement path.

### Owner ruling on resources

Owner, 2026-10-22: durable resource-use improvements beat short-term uptime.
Moving 2 to 5 GiB of scratch per apply from RAM-backed tmpfs to the data volume
is a durable RAM saving on every computer that applies; the cost is disk
writes on the data image, freed on the host by the discard path (RES-16) after
the probe removes its root. Refusing an apply's checkpoint (the op held,
visible) instead of risking the runtime is the uptime trade the ruling allows.

### Conjecture delta

Before: the replay probe's scratch is small and disposable, so `/tmp` is fine.
After: the scratch is O(base) + O(tail), must be admitted against the
persistent volume's free space and bounded while it runs, and a refusal is a
named, visible deferral of the checkpoint, never a runtime death.

### Protected surfaces

Checkpoint/route projection (the witness and the bind are unchanged; where the
replay store lives and when it may run change), self-development apply (the
materializer's handling of one named error), boot path (one new phase that
only removes the probe's own scratch).

### Admissible evidence

- Local (yellow tests, written first): scratch exhaustion refuses by name and
  leaves no scratch and no live capture; exhaustion in flight cancels the
  replay by name and removes the scratch; stale roots are removed at boot; the
  materializer holds the operation with the stamp and arms no retry when the
  checkpoint refuses; a base-plus-tail probe's witness equals a genesis probe's
  on the same fixture (the incremental path's correctness bar for R1).
- Staging (required, not claimable locally): after deploy, b@c's next boot
  settles selfdev-83fbdfa9 (operation `applied`, a
  `selfdev-checkpoint-published-*` event, route generation advanced), the
  console shows no Fatalf, the boot log shows the scratch phase, and the probe
  root is under `/mnt/persistent/replay-probe` while it runs.

### Rollback

`git revert` of the fix commit: the probe returns to `/tmp`. A probe killed
under the fix leaves its root on the data volume, and the reverted code has no
boot phase to remove it; remove `/mnt/persistent/replay-probe` by a guest act
after a revert (bytes only, no state).

### Heresy delta

- Discovered: (H1) the crash-boot journal-only settlement re-runs the
  checkpoint probe with no attempt bound, a crash-restart loop path the restart
  rule forbids (hypothesis for b@c after 11:50; code path confirmed); (H2) the
  replay scratch shares RAM with the runtime (tmpfs root); (H3) this document's
  first framing "replays the whole tape" was wrong: it installs a whole base and
  replays the tail.
- Introduced: none intended. The held `materializing` operation counts as
  self-development work and blocks a planned refresh while refused; that was
  already true of the crashed state, and is now visible on the operation.
- Repaired: the probe can no longer exhaust `/tmp`, and a predictable
  exhaustion of the data volume refuses by name instead of a Fatalf (H2, and
  H1 for this cause). H1 for any other Fatalf in the settlement stays open (R3).

### Residuals

- R1: a retained replay store advanced by the tail between checkpoints, to
  remove the O(store) copy per probe (design, tier 2).
- R2: stream the base download into the unpack (verify the digest before the
  rename, not before the unpack) to remove the 2 x blob install peak; it
  touches the restore installer's verification order.
- R3: bound the crash-boot settlement's checkpoint attempts the way live applies
  are bounded (`spendGuestApplyAttempt`).
- R4: the reconcile pass signs a verifier certificate before the bind on every
  held pass; hoisting the admission check before the signature would make a
  held pass cheaper.

## Review 309 (2026-10-26): findings on slice 1 and what slice 1b does

Slice 1 (`1d7d92a22`, landed as `9771c7680`) was reviewed by two seats (Devin,
DeepSeek v4.1 flash; bunch 309). Both accepted the code: the bind no longer
replays, the witness is identical to the replay path's, the publish inside the
hold is correct, and restore still refuses a non-derivable store. Findings,
recorded before the fix (slice 1b):

1. Medium (both seats): `settlePendingPlatformUpdateAfterCrash` and
   `resumeStrandedPlatformUpdateTail` re-drive the checkpoint and route tail on
   every crash boot with no attempt bound. Only the self-development settlement
   was bounded. A tail that crashes the computer deterministically (a full data
   volume, an out-of-memory kill) repeats on every boot: the loop the restart
   rule forbids. Fix: a durable `spendGuestAttempt` before the work, reason
   `platform_update_settlement`, limit 2, one count per update shared by the two
   paths; a planned, marker-backed restart is not counted (the marker bounds it:
   a resume that crashes is a markerless boot, which is bounded here).
2. Low: the scratch reserve is a flat 2 GiB; this document specifies
   `max(2 GiB, 10% of the volume)`. Fix: derive it from the same `Usage` in
   admission and in the watch.
3. Low: the close cause said "crashed the computer twice" but counts starts, so
   two boots on which the base source was down close an operation with a false
   cause. Fix: "the settlement did not finish on two boots".
4. Low: `allowedTransition` has no `accepted -> degraded` edge, so an operation
   whose settlement was spent before it materialized wedged. Ruling (FP0):
   `degraded` means applied with an unsettled tail and stays open; an operation
   that installed nothing settles `failed`. Fix: an `accepted` operation closes
   `failed`; one that reached `materializing` or later closes `degraded`.
5. Low: `spendGuestAttempt` returns nil when there is no
   `GuestApplyAttemptsPath`, so a runtime without a store path had no bound. For
   a settlement bound that fails open. Fix: both settlements refuse, held and
   visible, when the count cannot be kept.
6. Nits: the dead `replayCtx`; the witness digest struct duplicated in
   `WitnessFromObservationSets` (now one `witnessDerivabilityDigest`);
   `CheckpointEligible` is a hard-coded placeholder until slice 2's
   `verified` / `not_derivable` (documented in the code).
7. Observation, not fixed here: the settlement's live extract still reads every
   table under the append hold (design R2, unmeasured on the paper).

**The withdrawn bind-time derivability gate.** The second seat flagged that the
delta withdraws a recorded Definition action ("refuse checkpoint creation while
any behavior-bearing VM-local row is not event- or receipt-derivable",
`docs/archive/choir-tape-recovery-2026-08-13.md`) with no ruling in the delta.
The withdrawal rests on the owner's in-session statements of 2026-10-26: "why
do we need to be replaying stuff to apply anyway? ... I think we're probably
doing it the wrong way", and "the redesign should cover this". FP0 read these as
the ruling. It is a reading, open to the owner's reversal. Until slice 2 a
non-derivable row yields a checkpoint that publishes and is refused at restore;
restore still fails closed (`WitnessContentMatches`), and slice 2 moves the
proof to the host, where `not_derivable` becomes visible before a route
promotion.

## Review 315 (2026-10-26): findings on slice 1b and what the next commit does

Two seats reviewed slice 1b (`ac6dcce04`). DeepSeek: changes required (one High);
Devin: a Medium on the same mechanism.

1. High: the platform settlement spent its attempt before
   `commitPlatformUpdateApplied`, which appends the applied event and then drives
   the tail. Two failed appends after a journaled `applied` (a store refusing
   appends while it replays, the b@c environment) spent the bound with the applied
   event never committed; the transition stayed pending and
   `ApplyPlatformUpdate` then refused every later offer, security updates
   included. Fix: the commit is split. The applied event is recorded first,
   outside the bound (it is O(1) and the journal says what happened); the spend
   gates only the tail.
2. Medium: a spent bound left the held state only in a log line. Fix: at
   spend-out (or with no durable count) one idempotent `recovery_recorded`
   event names the update and the cause. It is not `materialization_failed`: the
   release is live and a failed event would falsify the tape. Not
   `lifecycle_observed`: it carries a receipt join the event cannot satisfy.
3. Low: the stranded path spent before checking the pinned offer's realization
   and expiry, which `ApplyPlatformUpdate` refuses before the tail; it now
   checks first. The degraded close routes every pre-materializing state
   (`awaiting_approval`, `accepted`) to `failed`.

Found while fixing, not fixed: `ApplyPlatformUpdate` validates the offer's
5-minute expiry before it reaches the applied-event branch, so a stranded tail
older than the offer's expiry is refused by the crash-boot re-drive whatever the
bound says; nothing finishes it except the next signed update superseding it. The
held event now makes that visible only once the bound is spent; an expired strand
logs and writes nothing. Recorded for the redesign of the tail (slice 2).

## Review 323 (2026-10-26): findings on the 315 fix and what the next commit does

Both seats confirmed the 315 High is fixed on the platform settlement. New:

1. High (DeepSeek): the self-development settlement had the same shape the
   platform one had. `settleSelfDevelopmentOperationAfterCrash` spent the bound
   before `recordMaterializationApplied`, whose first durable step is the applied
   append. Two boots on which the store refused that append spent the bound; the
   third closed the operation `degraded` with no applied event, the pending
   transition still bound (every later offer refused, security updates
   included), and a cause ("the apply is recorded on the tape") that was not
   true. Fix: the applied-event append is split out
   (`appendMaterializationAppliedEvent`) and runs first, outside the bound; the
   spend gates only the checkpoint and route tail. The cause is now true, and the
   rollback wording says the rollback is what is recorded.
2. Medium (Devin): a journaled terminal `refused` outcome is pre-mutation, but a
   crash boot's platform settlement fell to the "no verified recovery" branch and
   left the transition pending forever. Fix: it discharges as failed, like a
   missing journal. The self-development settlement had the sibling hole: it
   spent before reading the outcome, so a refusal arriving after the bound was
   spent closed `degraded` for an operation that installed nothing. Now only an
   `applied` journal has a tail and a bound; every other terminal outcome
   settles through the normal path with no spend.

Adjudication of the rest:

- Adopted: a stranded tail that cannot be re-driven (retired realization,
  expired offer) now leaves the same idempotent `recovery_recorded` event as a
  spent bound, once, instead of a log line on every boot (DeepSeek 3, Devin on
  the expiry residual). The degraded cause names the rollback when it is one
  (DeepSeek nit 5).
- Deferred, successor named: platform shapes whose effective state is unknown
  (a mid-flight journal, an unreadable journal or updater key, receipts that do
  not verify) still leave the transition pending with a log line. Fail-closed is
  right for unknown state, but there is no guest-side owner act that discharges a
  platform update. Successor: a guest-side owner discharge of a pending platform
  update, in the tail redesign (slice 2) with the host-side proof.
- Deferred with the same successor (re-review 332, Medium): the selfdev twin of
  those unknown-state shapes. A journaled terminal `failed` without a verified
  recovery receipt, and an `applied` journal whose receipts fail
  `recordSettlementApplied`, hold the operation `materializing` with the pending
  transition bound. Pending blocks every later decision, so no guest-side act
  discharges it either; an owner rollback is the only way out today. Fail-closed
  is right for unknown effective state. Successor: the guest-side owner discharge
  in slice 2, covering the platform and the selfdev shapes together.
- Adopted (re-review 332, nits): a journaled refusal or failure with no stated
  reason settles with a named cause ("the updater journaled a refused outcome
  with no reason"), never an empty one; and an `applied` journal on an
  operation that is still `awaiting_approval` or `accepted` (a contradiction: the
  updater runs only after `materializing`) is held with a named error, with no
  applied event recorded and no attempt spent.
- Rejected, reason: a per-boot dedup for the platform spend (DeepSeek 4, 315-3).
  The sweep runs once per boot (`runtime.go`), and the invariant is stated in
  `spendPlatformUpdateSettlementAttempt`. Rejected: boot-path test for the
  `accepted -> failed` close (DeepSeek nit 6): a terminal journal implies the
  operation left `accepted`, so the branch is defensive and cannot be forged
  without bypassing the transition table; the helper-level test stands. Rejected:
  the "settled as applied" log wording (cosmetic; the line says the tail is held).
- Accepted residual: the applied-event record on a crash boot is unbounded. It is
  O(1) and bounding it recreates the 315 High (an unrecorded apply leaves the
  transition pending and refuses security updates). The cost is that a store which
  crashes the guest on that very append boot-loops; that is a store defect no
  settlement bound can fix, and the restart rule's remedy (the next planned
  restart, the platform's deploy discharge) applies.
