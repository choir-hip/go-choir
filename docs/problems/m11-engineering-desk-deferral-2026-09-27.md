# M11 blocker: engineering desk occurrence hot-loops ErrDeferUnprocessed; selfdev operation never advances past `executing`

**Status:** open, blocking M11 episode leg `primary_started -> awaiting_approval`.
**First observed:** 2026-09-27, staging `bc58fa8f` (M1+M2+R5a deployed), two
independent fresh-owner episodes on fresh computers.

## Evidence

Probe: `scripts/m11_selfdev_episode_probe.mjs` (POST `/auth/register` ->
`POST /api/computers/bootstrap-chain` -> `PATCH .../mode` propose-only ->
`POST .../operations`). Both attempts:

- `computer-2b804495f349fd0b85c292ac39e5953b` (01:41Z, op `selfdev-f19ec10e...`):
  tape stops at seq 8 — `genesis_imported`, 1x `trajectory_started`, 6x
  `projection_batch_recorded`; no further events in 25 min.
- `computer-41c2401582dc4ae06a9325887a63117c` (02:23Z, op
  `selfdev-96a4881b40e798c9404cf2948f0eddfa`): identical tape stop at seq 8.

Guest autoputer journal for `41c24015` (go-choir-vmctl, `[686]` runtime):

```
dispatcher: deferred 9b6e9e98-8bfe-4b33-9ec1-684442cef511
engineering:e9dc741d-991c-5e6e-9449-ed8e08c82e90/{cf2215f5,...} until +0.5s
dispatcher: deferred 9b6e9e98-8bfe-4b33-9ec1-684442cef511  (repeats ~500ms)
engineering:e9dc741d-991c-5e6e-9449-ed8e08c82e90/{ad66946a,...} until +0.5s
  (two pending updates to the same desk agent, alternating)
```

So the `coagent_result`/`DocumentRevisionOccurrence` wake reaches the desk
agent `engineering:{docID}` inside the guest; the handler in
`actorruntime/handler.go` (engineering branch) calls
`ReconcileEngineeringRevisionCast`, which returns a non-`ErrNotFound` error
that is wrapped in `ErrDeferUnprocessed` and logged without the cause
(`internal/actor/dispatcher.go:353` before af788403), so the concrete
failure was invisible from host observability.

## Diagnosed cause (2026-09-27, staging `6529641d` with cause= logging)

`PreflightSourceSnapshot("")` on the guest runs `git -C
/mnt/persistent/files/Source/platform diff --quiet` — and **the directory
does not exist on a virgin computer**. Nothing in the guest image, genesis
import, or bootstrap chain creates `files/Source/platform`. Prior
self-development evidence (M9a et al.) all ran on Node B's long-lived
computer whose Source tree was populated manually on 2026-09-05 by an
operator rsync onto a mounted data.img
(`docs/evidence/rlm-option-b-actuator-refresh-2026-09-05.md:51-66`) — the
same "does not exist" failure, fixed by hand and never productized.
`docs/archive/mission-super-console-source-mount-promotion-v0.md` already
recorded "no sandbox startup path creates stable Source/platform".

So `startAssignedEngineeringForDocument` fails at
`preflight immutable assignment subject: source snapshot git diff --quiet --
failed (exit 128) ... No such file or directory`, the occurrence wraps it in
`ErrDeferUnprocessed`, the dispatcher's defer-attempt rollback pins the
backoff at 500ms, and the desk hot-loops on the cast wake forever.

## Fix (this mission)

`internal/vmmanager`: `createDataImage` seeds `files/Source/platform` into
the staging tree `mkfs.ext4 -d` copies into a fresh data.img. The seed is a
`git clone` of the host's deployed checkout (`VM_SOURCE_SEED_REPO`,
defaulting to `/opt/go-choir` when its `.git` exists) checked out to the
vmctl binary's `buildinfo.Commit`, with `core.filemode=false` so ext4 seeding
noise can't read as dirt. Virgin computers then satisfy the assignment
subject preflight; existing computers with populated Source are untouched
(the seed only runs on `createDataImage`, i.e. absent data.img).

## Failure signature

Symptom: operation stays `executing` forever; the assignment never opens; the
tape sees no `engineering_assignment_opened`/`trajectory_*` advancement.
Diagnosis (added 2026-09-27): `source snapshot git diff --quiet -- failed
(exit 128): fatal: cannot change to '/mnt/persistent/files/Source/platform':
No such file or directory`.

## Second failure cluster (2026-09-27, seeded build 382c6844 on computer-ff406316)

After the Source seed fix deployed, the probe's fresh computer progressed past
preflight-with-missing-dir into a NEW defer cluster on desk agent
`engineering:be7a90af-…` — three distinct causes across successive deliveries
of the same cast:

1. `open cast: bind assigned Engineering activation: co-super assignment
   invalid transition (persist pre-bind capsule revoke intent: lifecycle
   command digest conflict)` — a prior delivery's `cleanupCapsule` wrote a
   `capsule-revoke-intent:` fate command carrying `ExpectedLifecycleVersion`
   inside its digest; the next delivery's bind sees `RevokeRequested`
   disposition (`invalid transition`), its own cleanup retry conflicts on the
   stable CommandID because the version field moved the digest. One stranded
   saga permanently poisons the assignment.
2. `open cast: preflight immutable assignment subject: capsule subject
   artifact ref is invalid` — `PreflightSourceSnapshot` only parses
   `capsule-subject:sha256:` refs (`internal/capsule/executor.go:117`), but
   implementation opens durably store `capsule-source-git:<commit>:sha256:`
   (blessed in `types/engineering_assignment.go:154`). Any open that survives
   to a second wake (resume path
   `resumeAssignedEngineeringForDocument`) wedges forever.
3. `reconcile lifecycle work assignment: lifecycle command digest conflict`
   — a concurrent `ReconcileLifecycleWorkAssignment` →
   `ReconcileEngineeringAssignmentsForTrajectory` open raced the revision-cast
   open for the same deterministic assignmentID; identical CommandID,
   mismatching binding payload → conflict. There is no per-assignment
   serialization across the two reconcile entry points (the
   `lifecycleWorkReconcileMu` only covers `reconcileAssignedWorkItemActor`,
   not the engineering path).

Adjacent latent defect observed on fleet computers: boot engineering-desk
reconcile fails with `lifecycle: unknown frozen event kind
"owner_instruction_queued"` — the M1-deleted kind still exists in pre-M1
computers' frozen tapes; the decoder has no tombstone mapping. Pre-M11,
affects old computers only.

## Interleaving hypothesis (evidence: three distinct causes, all within ~1s)

The join's explicit `DispatchActor` and the commit-path
`dispatchTextureRevisionWake` deliver the same revision occurrence; the first
delivery's saga stranded after durable open (cause unproven — possibly the
concurrent open race itself, where the loser ran `cleanupCapsule` on the
winner's in-flight assignment). Every subsequent delivery then fails at a
different defense: resume-preflight ref rejection, or bind refusing the
poisoned `RevokeRequested` disposition.

## Cluster fix (2026-09-27, second commit)

- `capsule.PreflightSourceSnapshot` accepts `capsule-source-git:` refs and
  re-derives the subject digest from the pinned commit — resume no longer
  wedges on implementation-open bindings.
- `ReconcileEngineeringAssignmentsForTrajectory` converges on the recorded
  durable intent instead of requiring its own computed intent; fate/cancel
  writes tolerate `ErrLifecycleCommandConflict`/`ErrEngineeringAssignmentInvalid`
  by reloading and adopting an already-recorded state.
- `startAssignedEngineeringForDocument` holds `engineeringAssignmentOpenMu`
  for the open/spawn/bind saga so duplicate wakes cannot interleave
  divergent fate intents on one deterministic assignment identity.
- `owner_instruction_queued` tombstoned into the frozen event-kind table
  (decode-only; pre-M1 computers' tapes still carry it).

Residual (documented, not fixed): an assignment cancelled pre-bind is
terminal; attempt>1 requires a supersede tuple naming a recorded prior
report — which a never-bound attempt cannot have. Cancellation of a live
saga is now unreachable within one runtime (mutex), but a cast cancelled by
an external path still cannot auto-reopen. A future reopen path needs a
new supersede kind (`retry_after_block` with a cancelled-attempt receipt).

## Third wedge (2026-09-27, build 1bd014bd): reaper races the live saga

Even with resume-ref and converge fixes, bind still failed bare
`co-super assignment invalid transition`. Root cause: the durable open's
`work_opened` event wakes `ReconcileLifecycleWorkAssignment`, which calls
`ReconcileEngineeringAssignmentsForTrajectory` outside the saga mutex. The
reaper saw the live saga's `Open`+`Unbound` window as stranded, persisted
its own revoke intent, and cancelled the assignment while the saga was
binding. Cancellation is terminal; attempt>1 requires a supersede tuple
naming a recorded prior report — which a never-bound attempt cannot carry
— so the cast became permanently unopenable.

Fix (commit `ed8f471e`): `ReconcileEngineeringAssignmentsForTrajectory`
runs under `engineeringAssignmentOpenMu`, the same mutex the saga holds.

Adjacent defect seen during the same window (veteran computer
03335285…, pre-M1 tape): `activation occurrence must remain unprocessed:
persistent Management slot occupied by run b05f42a6-…` — a non-terminal
management run blocks all fresh wakes on that computer; out of M11 scope
but recorded so it is not lost.

## Fourth wedge (2026-09-27, build ed8f471e): desk ends its turn with prose; operation never leaves `executing`

Fresh-computer probe (marker `M11_SELFDEV_EPISODE_1790485385506`,
computer `40ca2130…`, operation `selfdev-d478fd1f…`) reached the desk: the
guest ran 27 `capsule_go_eval` cells in ~3.5 minutes
(`jcvk9drr…-go-choir-run-autoputer-runtime[677]` tool-loop lines
05:03:31–05:06:38, gateway `vm-cedfe962…` inference log). At iteration 28 the
model returned `stop=end_turn`, `tool_calls=0`, `text_len=3789`
(gateway provider log 05:06:48) — a narrative close instead of a
`capsule_go_eval` commit. The tool loop accepted it because the assigned
Engineering overlay wires `WithTerminalToolResult("capsule_go_eval",
fate_terminal)` but no completion obligation: no `initialToolChoice`, no
`requiredWriteTools`, no `completionGuard`. `end_turn` is plain completion,
the run ends clean, and the operation sits in `executing` until the probe's
`waitForOperation` deadline — the canonical tape (178 events) carries only
`trajectory_started` + `projection_batch_recorded`; zero tool/effect events.

The host-side evidence is decisive because guest tool-loop output is not
log-forwarded: every provider call round-trips `choir.gateway_url`, so the
gateway's `stop=`/`tool_calls=` fields are the iteration ledger.

Fix (this commit): wire a `completionGuard` for `assignedEngineeringOverlay`
runs that scans persisted `tool_result` blocks for a `capsule_go_eval`
`fate_terminal` output. `end_turn` without one retries with a bounded
reminder (`maxCompletionGuardRetries`), then errors the run loudly —
`handleExecutionError` → `terminalizeRun` joins assignment fate rather than
stranding the operation.

Ops note from the same window: the `5cff1633` deploy's NixOS switch +
service restarts (05:33–05:35) briefly stopped all `go-choir-*` units —
guest file-sync logged `connect: connection refused` at 05:33:15 — and
hibernated the probe VM (`vm-cedfe962…`, pressure reclaim at 05:34:47).
The deploy reported success and health-checked every service; the outage
was the switch/restart window itself, not a stuck unit. Future probes
should expect a per-deploy interruption of in-flight ops and plan retries.

Verified on staging build `88d5a447` (probe computer `4b20e20e…`, op
`selfdev-814208951f89a61d0a4c4e4c98c683b0`): the desk `end_turn`ed at
06:51:16, the guard retried, and after the third prose ending the run
terminalized; `cancelBoundEngineeringRun` propagated the failure into the
operation — observed `state=failed`, `terminal_error="tool loop: completion
guard \"\" was not satisfied after 2 retries"` at 06:59:13, *before* the
probe deadline. The silent-`executing` wedge is closed.

Residual: the desk model (deepseek-v4.1-flash) ended its turn without
`choir.Freeze`/`choir.Complete` across all three chances — the substrate is
honest now, but reaching `awaiting_approval` additionally requires the desk
to actually commit a terminal fate, which is a model-completion problem,
not a wedge. Cosmetic gap: the persisted error's quoted guard reason is
empty (`""`) even though the guard sets `Reason` — investigate when the
guard's reason string is next needed for triage.


Known candidates inside the deferring call:

1. `reconcileEngineeringCast` rejects the revision
   (`AuthorKind != AuthorUser` after `prepareTextureRevisionV2` normalization,
   or `RevisionID` mismatch vs occurrence's `HeadRevisionID`).
2. `startAssignedEngineeringForDocument` finds zero or >1 open engineering
   work items (`parentWorkID` guard).
3. `OpenEngineeringAssignment` write fails an authority invariant
   (`requireEngineeringDocumentParentAuthority` — parent agent role/profile/
   lifecycle, parent revision author-kind, parent work assignment).
4. `PreflightSourceSnapshot("")` fails in the guest capsule executor
   (no sourceDir / git binary in guest).

## What proves closure

`m11_selfdev_episode_probe.mjs` reaches `awaiting_approval` (the
`waitForOperation` leg) on a fresh computer: either the desk commits a
`fate_terminal` freeze inside the retry window, or the run fails loudly with
a guard-exhaustion error rather than stranding `executing`.

## Recovery posture

- `af788403` logs the wrapped cause (`dispatcher: deferred ... cause=%v`);
  once deployed, re-running the probe will name the concrete error and this
  receipt can be updated with the diagnosed line.
- The failed computers are disposable; no rollback needed — `POST
  /api/computers/bootstrap-chain` mints a fresh one per probe run.
- Until the cause is fixed, `executing` operations on staging accumulate as
  deferred wakes; the dispatcher hot-loop is bounded (500ms->30s backoff) but
  wastes guest CPU.

## What proves closure

`m11_selfdev_episode_probe.mjs` reaches `awaiting_approval` (the
`waitForOperation` leg) on a fresh computer, and the journal shows the desk
occurrence incorporated rather than deferred.
