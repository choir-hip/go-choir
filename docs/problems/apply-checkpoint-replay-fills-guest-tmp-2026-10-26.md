# An apply's checkpoint replays the whole tape into the guest's /tmp, fills it, and kills the runtime (2026-10-26)

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
  the paper (`docs/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md`,
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
  the station probes (`docs/reports/surface-audit-cli-mcp-api-2026-10-14.md:307`;
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
