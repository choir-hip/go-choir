# Pause and resume a live engineering sandbox across a planned restart (2026-11-09, r3)

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
   `capsulePauseState` fails closed: an IO or parse error reads as live, not
   paused. The restart gate is the only count that changes: `/health`
   `live_capsules` and the host's idle, crash-loop and busy fallbacks keep
   counting a paused capsule as live.

   **The gate's two halves (r2 F1).** `unsafe()` is `SelfdevActive > 0 ||
   LiveCapsules > 0 || SnapshotInFlight`, and a bound implementation
   assignment is itself a self-development operation in `executing` for the
   whole round (`assignmentOperation`, by freeze key). For the deploy-refresh
   mark only, both halves exclude an assignment whose capsule carries a
   committed, unexpired pause record: its capsule leaves `LiveCapsules` and
   its executing operation leaves `SelfdevActive`. Started-but-uncommitted
   pauses still refuse, as do executing operations without a record.
   Hibernate, snapshot and restore keep today's count (a paused capsule
   refuses them): out of scope.
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
   the pause is abandoned and the capsule thawed). A pause starts only when
   the assignment's disposition is `active` and no fate saga is in flight
   (`assignedEngineeringFatePending`, engineering_assignment_fate.go:774);
   otherwise the mark refuses as today. Pause is single-flight per capsule: a
   second phase-1 mark joins the pause in progress. The hibernate fence is not
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
5. **Unpause on every path that is not a restart.** Before the phase-2 ack:
   a pause not acked within its TTL (default 10 min after commit), or a
   handler error, unpauses: thaw, remove pause.json, lift the admission
   block; the run continues. After the ack (marker written) the TTL no
   longer applies, since an unpause then would consume the record just
   before the host's kill and turn a planned restart into a crash close;
   only the host clearing its marker (the DELETE of a mark after a failed
   refresh, `marker.clear`) unpauses. A record past its TTL is not paused
   for phase 2: phase 2 refuses and the TTL path unpauses it. A repeat mark
   over an already-paused capsule is idempotent. The host's kill lands
   within the marker's 15-minute freshness, or the boot reads as a crash.
6. **The dir does not move while mounted.** pause.json stays in
   `boot-<old>/<id>/`. At boot, BootScopedStateDir (which runs before the
   marker is consumed, autoputer/run.go:304 vs :621) uses the non-consuming
   PlannedRestartMarkerPending: when a marker is pending, it spares dirs that
   hold a valid pause.json by renaming them (now unmounted, a fresh VM) into
   the new boot dir; otherwise it sweeps as today. The crash-vs-planned
   decision is made after consumption, at reconcile (Decision 7), and a spared
   dir whose boot turns out to be a crash is deleted there.
7. **Reconcile decides, before anything else engineering.** Every entry to
   `reconcileEngineeringAssignmentsForTrajectory` checks the pause record
   first, not only the boot sweep (r2 F2): it also runs on lifecycle
   work-item wakes (`ReconcileLifecycleWorkAssignment`,
   actorruntime/handler.go:458) and after an engineering run ends
   (`kickEngineeringReconcileAfterPersist`, runtime.go:2355), where a paused
   capsule would otherwise fail `assignedEngineeringCapsuleUsable` (state
   frozen, not active) and be revoked. Two cases: the capsule is in the
   executor's memory (a live pause on this process) - keep the assignment
   and arm nothing, the unpause machinery owns it; the capsule is absent
   (boot) - the branches below. At boot, after the
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
8. **Resume is a normal spawn seeded with the preserved upper (r3 substrate
   shift).** Rounds r0-r2 kept finding protections an "adopt the old dir"
   path drops (bundle binds, the source-lower ref, the overlay workdir, the
   computer layer, the source-cache key, spawn spec fields). Each is a step
   Spawn already performs. So the resume does not adopt the old dir; it
   spawns a fresh capsule through the executor's ordinary Spawn, with the
   same capsule id and the spawn spec recorded in pause.json, and one new
   input: `SeedUpper`, the preserved upper directory, which Spawn renames
   (same persistent filesystem, no copy) into the new dir as its upper
   before the mount. Everything else is Spawn's own: an empty workdir, a
   freshly staged source lower from the durable subject (inputs and the
   carried patch re-staged as at bind), the shared source cache with its
   ref and cache key, the read-only `selfdev/{bundle,carried}` binds, a
   fresh computer layer, cgroup and broker. The resume path is still not
   the bind saga (resumeDelegatedCastAssignment, engineering_assignment_
   runtime.go:529, binds and activates, which would cancel a bound
   assignment): it does admission (host build reservation), validates
   pause.json against the binding (assignment id, attempt, capsule id,
   subject digest, upper digest and counts), tears down live residue of the
   old dir (a process-only restart under `Restart=on-failure` keeps the boot
   id, so the old merged mount may be mounted and its cgroup frozen with
   tasks: kill via `cgroup.kill`, wait until empty, then delete, then unmount;
   the upper digest is validated after teardown), spawns with `SeedUpper`,
   deletes the rest of the old dir, mints the capability handle from the same
   deterministic opaque with its TTL capped at the remaining deadline, renames
   the record to pause.consumed.json, and only then releases the run. Any
   resume error (validation, teardown, admission after a bounded retry,
   source staging such as the subject commit missing from the new release's
   repo, spawn, mount) closes the assignment "interrupted by a restart" and
   deletes the dir; a zero remaining deadline closes it before any work.

   **The lower changes under the upper.** A platform update replaces the
   guest root, the lowest overlay layer. overlayfs permits offline changes
   to lower trees when index, redirect, metacopy and xino are off
   (namespace.go:230); copied-up files keep their old-release content and
   shadow the new root (for example a file the capsule edited under /etc),
   and copied-up `/computer/files` entries shadow the fresh computer layer.
   pause.json records the old release commit; the resume context names the
   change; a mount error closes as interrupted. Acceptance requires a drill
   across a real release change, not only a same-release restart.
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

## r3 additions (Devin r3 F1-F2, Sonnet r1 B1-B4 and N1-N9)

- **Ack is atomic and the post-ack pause is bounded by the marker.** One
  per-capsule mutex covers the gate check, setting `acked` in pause.json,
  the marker write and the sync; the pre-ack TTL path takes the same mutex
  and refuses once `acked` is set. After the ack the record's expiry is the
  marker's: a watchdog reads the non-consuming marker check each minute and
  unpauses when the marker is gone or older than `plannedRestartMaxAge`
  (15 min); a fresh marker keeps the capsule frozen. An unpause after a mark
  DELETE retries until it succeeds.
- **The host keeps asking each minute.** Every "pausing" and "paused,
  awaiting ack" refusal carries `holds_new_work: true`
  (`guestRefusalHoldsNewWork`, deploy_refresh.go:143), so vmctl's retry stays
  at one minute, under the pre-ack TTL. Test the cadence.
- **The run's reactivation is gated at one choke point.** Not only
  handleInitialDispatch: the gate sits in the run-to-assignment lookup that
  every activation path passes (channel_message, coagent_result,
  owner_revision, the management rewarm of runtime_restarted runs), so no
  wake runs a turn against an absent capsule. It lifts when the resume
  completes or closes.
- **The boot spare decision is reason-aware.** `BootScopedStateDir` takes an
  injected check (capsule does not import agentcore) that returns the
  pending marker's reason and is evaluated at `processStartedAt`, the same
  instant `ConsumePlannedRestartMarker` uses; only a platform-update reason
  spares. After reconcile, every spared dir that was not resumed is deleted.
- **A paused capsule refuses thaw, extract and inspect.** A `paused` flag in
  the executor's capsule record makes ThawGranted, ExtractGranted and
  InspectGrantedWorktree refuse; ForceDestroy (owner cancel, a terminal
  trajectory) clears the record and the admission block. The next pause
  removes a stale pause.consumed.json.
- **Scope.** The gate only matters to marks with `allow_busy` (a paused run
  stays in `rt.running`): critical_protected and public_platform computers
  and the owner update, which sends allow_busy
  (update-now-2026-11-09.md item 1). An ordinary computer's sweep still
  waits for idle.
- **Digest limits, stated.** `walkUpperdir` skips `upper/computer` and the
  release scaffold; the digest proves the work tree, not those.

**Restore markers.** Only a planned-restart marker whose reason is a
platform update spares pause dirs; a self-development restore marker
(`PlannedRestartSelfDevelopmentRestore`) does not, and the boot sweeps them
as today.

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
15. Every capsule paused: phase 2 acks (`LiveCapsules` and `SelfdevActive`
    both exclude the paused assignment); with one capsule still pausing, or
    an executing operation without a record, phase 2 refuses.
16. A lifecycle work-item wake or a run end during the pause runs reconcile;
    the paused capsule is kept, nothing is revoked or armed.
17. Pending marker plus a process-only restart (same boot id) with the
    merged mount live and the cgroup frozen: the resume tears down the
    residue, then resumes; a teardown failure closes as interrupted.
18. After resume: `source-lower-ref` names the rebuilt cache, Diff produces
    the same patch as before the pause, and a write to /selfdev/bundle from
    the capsule fails (read-only).
19. After the phase-2 ack the TTL expires before the kill: no unpause; the
    planned boot resumes.
20. A restore-reason marker does not spare pause dirs.
21. The marker write and a pre-ack TTL unpause race: exactly one wins; never
    a marker over a thawed capsule.
22. After the ack the refresh fails and the DELETE is lost: the watchdog
    unpauses within a minute of the marker's 15-minute expiry.
23. A channel_message or coagent_result wake for the paused run at planned
    boot defers until the resume completes or closes.
24. Resume across a real release change: the capsule runs, its patch is
    unchanged, `/selfdev/bundle` is read-only, the workdir is fresh.
25. Each resume error closes "interrupted by a restart" and deletes the dir;
    a spared dir with no open assignment is deleted after reconcile.
26. A thaw or extract call on a paused capsule is refused.

## Measures to record at first use

Upper/tmp bytes and file counts at pause; drain, freeze, digest and record
durations; resume duration (validate, mount, first cell).

## Adjudicated review findings, r3 (Devin r3 SEND_BACK; Sonnet r1 SEND_BACK)

Third round on this mechanism; per the repo rule the next move was the
substrate, not another patch: adopt-the-old-dir became a normal spawn seeded
with the preserved upper (Decision 8), which closes Devin r3 F2 (workdir,
computer layer), Sonnet N3 and N4 and the r2 F4 class at the root. Devin r3
F1 and Sonnet B1 (post-ack wedge, gate/unpause race): the r3 additions'
first item. Sonnet B2 (the lower changes): Decision 8's lower paragraph.
Sonnet B3 (one entry point; resume errors without a fate): the choke point
and Decision 8's close-on-any-error. Sonnet B4 (`holds_new_work`): adopted.
N1, N2, N5-N9: the r3 additions. The build goes ahead on r3 with its own
tier 2 review instead of a fourth design round.

## Adjudicated review findings, r2 (Devin SEND_BACK F1-F4)

| Finding | Outcome |
|---|---|
| F1 the executing self-development op still refuses the gate | Adopted: both halves exclude a committed, unexpired record, deploy-refresh mark only (Decision 1) |
| F2 runtime reconciles revoke a paused capsule | Adopted: the record guards every reconcile entry; in-memory vs absent cases (Decision 7) |
| F3 pending marker plus a same-boot-id process restart | Adopted: residue teardown before adoption (Decision 8) |
| F4 adoption drops the bundle bind mounts and leaves a stale source-lower-ref | Adopted: both named adoption steps (Decision 8) |
| Notes: health keeps counting paused as live; TTL pre-ack only and expired is not paused; refuse unless active and no fate saga; single-flight; fail closed; hibernate and snapshot out of scope; restore markers do not spare; kill within 15 min | Adopted (Decisions 1, 3, 5, restore paragraph) |
| Sonnet N3 deferral confirmed | Unchanged |

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
