# Pause and resume a live engineering sandbox across a planned restart (2026-11-09, r1)

Owner, 2026-11-09: "We should be able to pause and resume a live sandbox. I
guess we need to investigate the mechanics to make that safe." Context: a live
engineering capsule refuses every restart (internal/agentcore/planned_restart.go
unsafe(), liveCapsuleCount), so a computer that is always engineering never
takes an update, and "Restart to update" (update-now-2026-11-09.md) would
refuse most of the time.

Class: red (capsule lifecycle, the planned-restart path, assignment reconcile at
boot). Rollback: revert. The design adds no assignment state (see Decision 1),
so an older binary reading a computer mid-pause sees an ordinary `active`
assignment and an unknown file it ignores; at its next boot that assignment
meets today's "restart revoked absent assignment capsule" path.

**Doctrine delta (owner-authorized by the request above).** "A capsule never
spans a restart" (docs/design/capsule-overlay-computer-state-2026-11-03.md
failure mode 6; internal/capsule/state_dir_linux.go:21-29) becomes: a capsule
never spans a *crash*; it may span one *planned* restart as a paused capsule,
resumed at most once, under the planned-restart marker.

## Physics (investigation 2026-11-09)

A capsule's disk state (upper, work, tmp, source-lower) already sits on the
persistent volume (/mnt/persistent/capsule-state, nix/autoputer-vm.nix:1150);
the overlay has no index/redirect/metacopy (namespace.go:230), so the upper is
a self-contained diff. Pausing copies nothing. Lost today: the boot-scoped
sweep deletes earlier boot-* dirs (state_dir_linux.go:30-57) and the executor's
tables are memory-only. The agent side already resumes on a planned boot (run
memory durable, run_memory.go:90; passivate and reactivate, runtime.go ~3040,
actorruntime/handler.go:668). Process checkpointing (CRIU) is absent and
implausible (userns/netns, broker socket peer, 150 MiB-2 GiB RSS).

## Decisions

1. **No new capsule disposition.** The disposition stays `active`; the pause
   is a durable pause record file in the capsule dir (`pause.json`). Adding
   `paused` would touch Validate (types/engineering_assignment.go:428-442), the
   transition table (deskprotocol/transitions.go:67-75), fate replay
   (store/engineering_assignments.go:556-600, :2354), lifecycle reads and every
   `== Active` reader, and would break an automatic code rollback in exactly
   the window a pause opens. One helper, `capsulePauseState(assignment)`, reads
   the record; the restart gate (plannedRestartLiveCapsule), boot reconcile and
   the deadline/fate wakes consult it. Nothing else changes meaning.
2. **Two-phase, asynchronous pause.** The host's mark request has a 20 s
   timeout (vmctl/deploy_refresh.go:85,161) and cells run for minutes, so no
   drain happens inside the request. Phase 1: a mark request (allow_busy, or
   the owner's update) that finds live capsules starts a background pause and
   answers 409 `{reason: "pausing"}` (the host retries each minute; the
   guest's refresh hold already holds new engineering, refresh_hold.go). Phase
   2: the next mark request finds every capsule paused and acks in
   milliseconds: marker, syncFilesystems, 200. The marker is never written
   while any pause is incomplete.
3. **Pause holds its own admission block.** For the pausing run: no new cell
   or capsule op is admitted ("pausing for a platform update; this cell will
   run after the restart"); the pause waits for the run's in-flight cell and
   the capsule's inflightOps to reach zero (bounded, default 10 min; on expiry
   the pause is abandoned and the capsule thawed). The hibernate fence is not
   reused (it is shared and stays open while a mark is in flight,
   hibernate.go:68-160).
4. **One commit point: the durable pause record.** Order: admission block ->
   drain -> Quiesce (cgroup freeze) -> claude-token scan of the upper (the same
   check as refuseClaudeTokenInChanges; a hit abandons the pause) -> compute
   the upper's own digest and counts (walkUpperdir; LastManifest is not
   populated in production, capsule.go:252) -> write pause.json (tmp + fsync +
   rename + fsync dir). The record's presence is the commit. Before it: a crash
   or abandon leaves an ordinary live capsule. Running jobs are not touched:
   closeInterruptedJobs already closes them at every boot (jobs.go:828).
5. **Unpause on every path that is not a restart.** A pause not followed by a
   restart within its TTL (default 10 min after commit), the host clearing its
   marker after a failed refresh, the DELETE of a mark, or a handler error:
   thaw, remove pause.json, lift the admission block; the run continues. A
   repeat mark request over an already-paused capsule is idempotent.
6. **The dir does not move while mounted.** pause.json stays in
   `boot-<old>/<id>/`. At boot, BootScopedStateDir (which runs before the
   marker is consumed, autoputer/run.go:304 vs :621) uses the non-consuming
   PlannedRestartMarkerPending: when a marker is pending, it spares dirs that
   hold a valid pause.json by renaming them (now unmounted, a fresh VM) into
   the new boot dir; otherwise it sweeps as today. The crash-vs-planned
   decision is made after consumption, at reconcile (Decision 7), and a spared
   dir whose boot turns out to be a crash is deleted there.
7. **Reconcile decides, before anything else engineering.** At boot, after the
   marker is consumed and before queued admissions
   (resumeQueuedEngineeringRequestsAtPlannedBoot) and before bound-deadline
   re-arm, each assignment with a spared pause dir is resolved:
   - crash boot (no marker): close the assignment "interrupted by a restart",
     delete the dir (unmount and kill a surviving cgroup first: a
     process-only restart keeps the same kernel boot id,
     state_dir_linux.go:23-30);
   - planned boot: a new branch before assignedEngineeringCapsuleUsable
     (engineering_assignment_fate.go:780), which would otherwise revoke and
     cancel, keeps the assignment and arms one resume wake. Deadline and fate
     wakes that see a pause record defer to the resume wake.
8. **A dedicated resume path, not the bind saga.** resumeDelegatedCastAssignment
   (engineering_assignment_runtime.go:529) binds and activates, which would
   cancel a bound assignment. The resume path does only: admission (host build
   reservation), validate pause.json against the binding (assignment id,
   attempt, capsule id, subject digest, upper digest and counts), adopt the dir
   (Spawn accepts an existing dir only with a valid record), mount the
   preserved source-lower verbatim (no re-staging of inputs or carried patch),
   a fresh shared source cache from the durable subject, a fresh computer
   files layer, and MintCapabilityHandle from the same deterministic opaque
   with its TTL capped at the assignment's remaining deadline. It then renames
   pause.json to pause.consumed.json (one resume within a boot) and only then
   releases the run.
9. **The run waits for its capsule.** On a planned boot the engineering run's
   reactivation (actorruntime/handler.go:668-690) is deferred
   (ErrDeferUnprocessed, as for Texture at :731) while its assignment has a
   pause record, until the resume completes or closes. The resume injects one
   run-context turn before the run's first turn: the sandbox was paused for a
   platform update and resumed; the in-flight cell and kernel state are gone;
   running jobs were interrupted (rerun them); /computer/files is re-read and
   may differ; tool paths in /tmp, $HOME and $GOCACHE that name old store
   paths may be stale.
10. **Claude-mode capsules.** The delegate process dies with the VM; at resume
    a fresh relay secret is minted (executor.go ~600) and the delegate restarts
    with --resume <session> (claudedelegate/spec.go:93,311). If its session
    files are not under the capsule's persistent dirs, or --resume fails, the
    resume turn reports the delegate run lost and the assignment continues
    without it.

**Not paused (refusals stay):** a self-development apply in flight, a frozen
or awaiting-decision candidate, a snapshot in flight, a verifier-slot capsule
(implementation slot only), a capsule that cannot drain within its bound.

## Failure modes (tests first)

1. Crash during phase 1 before pause.json: next boot sees an ordinary live
   capsule; crash path as today.
2. Crash after pause.json, before the marker: markerless boot; reconcile
   deletes the spared or residual dir and closes the assignment.
3. Marker written, host refresh fails and clears it: TTL unpause; the run
   continues; no restart happened.
4. Mark DELETE while paused: unpause.
5. Repeat mark request while pausing: still 409 pausing; while paused: ack.
6. Planned boot, valid record: one resume; the run's first turn is the resume
   context; tools work.
7. Planned boot, record mismatched (digest, counts, binding): close
   "interrupted by a restart", delete the dir.
8. Resume crashes the computer: marker consumed, next boot is a crash: close.
9. A second resume wake in the same boot: sees pause.consumed.json, no-op.
10. A queued engineering request at planned boot is admitted only after the
    paused assignments are resolved.
11. A bound-deadline wake during the pause defers to the resume.
12. A process-only restart (same kernel boot id) with a frozen cgroup and live
    mount: crash path unmounts, kills, deletes.
13. Claude token found in the upper at pause: pause abandoned, capsule thawed,
    restart refused as today.
14. Code rollback mid-pause: the older binary sees `active`; its boot path
    revokes and cancels as today; pause.json is ignored and swept.

## Measures to record at first use

Upper/tmp bytes and file counts at pause; drain, freeze, digest and record
durations; resume duration (validate, mount, first cell).

## Adjudicated review findings (r0: Devin SEND_BACK B1-B8; Sonnet SEND_BACK B1-B6)

| Finding | Outcome |
|---|---|
| Sonnet B1 / Devin B2: resume through the bind saga cancels the assignment | Adopted: dedicated resume path (Decision 8) |
| Sonnet B2: reconcile cancels a paused assignment at planned boot | Adopted: explicit branch before :780 (Decision 7) |
| Sonnet B3: run reactivates before its capsule | Adopted: deferred reactivation (Decision 9) |
| Sonnet B4 / Devin B1: new disposition breaks validation, replay, rollback | Adopted Sonnet's fix: no new disposition, a pause record (Decision 1) |
| Sonnet B5 / Devin B6: drain cannot fit the host's 20 s request | Adopted: two-phase async pause (Decision 2) |
| Sonnet B6 / Devin B4: no unwind; renaming a mounted dir | Adopted: one commit point, unpause on every non-restart path, no move while mounted (Decisions 4-6) |
| Devin B3 / Sonnet q4: LastManifest is unpopulated | Adopted: own upper digest at pause (Decision 4) |
| Devin B5: no drain mechanism; shared fence | Adopted: pause-scoped admission block (Decision 3) |
| Devin B7: GC vs mounts, cgroups, process-only restart | Adopted: decision at reconcile after consumption, crash path unmounts and kills (Decisions 6-7) |
| Devin B8: resume vs queued admissions and wakes | Adopted: resolve paused first; wakes defer (Decision 7) |
| Notes: TTL cap, preserved source-lower, token scan, computer layer warning | Adopted (Decisions 4, 8, 9) |
| Sonnet N1 old store paths in caches | Adopted in the resume turn (Decision 9); no cache scrubbing |
| Sonnet N3 AdmitBuild double count across a guest restart | Deferred: verify in the build; the host reservation is per capsule id |
| Sonnet N6 tool_use without tool_result | Adopted: the drain completes the cell, so none is left open |
