# Pause and resume a live engineering sandbox across a planned restart (2026-11-09)

Owner, 2026-11-09: "We should be able to pause and resume a live sandbox. I
guess we need to investigate the mechanics to make that safe." Context: a live
engineering capsule refuses every restart (internal/agentcore/planned_restart.go
unsafe(), liveCapsuleCount), so a computer that is always engineering never
takes an update, and "Restart to update" (update-now-2026-11-09.md) would
refuse most of the time.

Class: red (capsule lifecycle, the planned-restart marker path, the assignment
capsule disposition). Rollback: revert; a paused capsule that is not resumed
falls back to today's revoke-and-cancel at planned boot.

**Doctrine delta (owner-authorized by the request above).** "A capsule never
spans a restart" (docs/design/capsule-overlay-computer-state-2026-11-03.md
failure mode 6; internal/capsule/state_dir_linux.go:21-29) becomes: a capsule
never spans a *crash*; it may span a *planned* restart as a paused capsule,
resumed once, under the marker. The crash rule is unchanged.

## Physics (investigation 2026-11-09)

A capsule's disk state (upper, work, tmp) already sits on the persistent volume
(/mnt/persistent/capsule-state, nix/autoputer-vm.nix:1150); the overlay has no
index/redirect/metacopy, so the upper is a self-contained diff. Pausing needs
no copy: quiesce, syncfs, a small record. What is lost today is only (1) the
boot-scoped sweep that deletes earlier boot-* directories
(state_dir_linux.go:30-57) and (2) the executor's in-memory tables. The agent
side already resumes: run memory is durable (run_memory.go:90) and a planned
boot passivates and reactivates running runs (runtime.go ~3040;
actorruntime/handler.go:668). Process checkpointing (CRIU) is absent and
implausible (userns/netns, broker socket peer, 150 MiB-2 GiB RSS).

## Design (option a1: keep the disk state in place, respawn, continue)

**Pause** (inside HandleInternalPlannedRestart, before the marker is acked,
only for a planned restart):
1. For each live engineering assignment: wait for its in-flight cell to end
   (bounded; if it cannot drain in time, no pause, the restart is refused as
   today), Quiesce the capsule (cgroup freeze), interrupt running jobs (their
   records say "interrupted by a planned restart; rerun").
2. Write a pause record beside the capsule dir: capsule id, assignment id,
   subject digest, lower layout, LastManifest digest, upper file count and
   bytes, claude session id, old boot id. syncfs; the record is durable before
   the marker is acked.
3. Set the assignment's capsule_disposition to `paused` (new, explicit); the
   restart gate does not count paused capsules.
4. Move paused capsule dirs out of the boot-scoped sweep (a sibling `paused/`
   directory under the capsule state dir, renamed atomically).

**Resume** (planned boot only, after the marker is consumed):
1. Reconcile treats a `paused` assignment like a kept queued request
   (engineering_admission.go plannedBootKeepsQueuedRequest): keep the row, arm
   one resume wake. A crash boot (no marker) closes paused capsules and their
   assignments with "interrupted by a restart" and deletes their dirs.
2. Admission runs again (host build reservation, refresh hold).
   resumeDelegatedCastAssignment (engineering_assignment_runtime.go:529)
   re-preflights the subject; Spawn adopts the paused dir (validated against
   the record: LastManifest digest and file count) instead of refusing an
   existing dir, remounts the overlay with fresh lowers (the shared source
   cache is rebuilt from the durable subject), and re-mints the capability
   from the same deterministic opaque value.
3. The reactivated run gets one run-context turn: "your sandbox was paused for
   a platform update and resumed; running jobs and cell state were lost; tool
   paths in /tmp may be stale; rerun builds".
4. The capsule disposition returns to `active`.

**Not paused (refusals stay):** a self-development apply in flight, a frozen or
awaiting-decision candidate, a snapshot in flight, a capsule that cannot drain.

## Failure modes (tests first)

1. Pause record or upper missing or mismatched at resume: close the
   assignment "interrupted by a restart" (no resume), keep the unfinished
   patch fallback if enabled.
2. Resume crashes the computer: the marker was consumed, so the next boot is a
   crash boot and closes the paused capsule and run (one resume per planned
   restart, no loop).
3. A crash (no marker) with a paused capsule on disk: closed, dir deleted,
   never resumed.
4. Stale toolchain paths after the update: the resume turn says so; caches in
   tmp that name old store paths are dropped.
5. Source base moved under the paused work: Freeze later meets the existing
   stack_base_moved / clean-rebase path (not removed by pause).
6. Claude-mode capsule: a fresh relay secret is minted; the delegate resumes
   with --resume or the delegate run is reported lost.
7. Disk: a paused upper stays charged against the state floor; pause records
   report bytes.
8. A pause that cannot finish before the host's restart deadline: the guest
   refuses the restart (409) rather than acking a half-paused capsule.

## Measures to record at first use

Upper and tmp bytes and file counts at pause; pause duration (drain, freeze,
syncfs); resume duration (validate, remount, first cell).
