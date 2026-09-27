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
that is wrapped in `ErrDeferUnprocessed`. The dispatcher logs `deferred`
without the cause (`internal/actor/dispatcher.go:353` before af788403), so the
concrete failure was invisible from host observability.

`StartLifecycle` succeeded: the desk agent row exists (`LifecycleVersion=1`,
`ChannelID=docID`), the work item `work:engineering` exists
(`AssignedAgentID=engineering:{docID}`, `AuthorityProfile=engineering`), and
the initial revision committed with `AuthorKind=user`. So the deferral is
downstream of durable creation — inside `reconcileEngineeringCast` or
`startAssignedEngineeringForDocument` (store write `OpenEngineeringAssignment`,
preflight `PreflightSourceSnapshot`, or an identity/authority invariant).

## Failure signature

Symptom: operation stays `executing` forever; the assignment never opens; the
tape sees no `engineering_assignment_opened`/`trajectory_*` advancement.

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
5. Store/Dolt write error in guest (unknown; previously unobservable).

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
