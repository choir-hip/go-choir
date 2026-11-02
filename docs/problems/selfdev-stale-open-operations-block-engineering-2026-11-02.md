# Stale open self-development operations block all engineering on a computer (2026-11-02)

Mutation class of this doc: green. Evidence read-only (CLI self-dev list/head, node-b console.log, code).

## Problem
Any self-development operation in an OpenStates state (internal/selfdev/operations.go:49) holds every
engineering admission on its computer: `openOperationHold` (internal/agentcore/carried_patch.go:418) feeds
`operation_open` (internal/agentcore/engineering_admission.go:197-205), retried every 60 s with no cap. The cause is
visible only in the console. No owner act can close an operation that will not settle.

## Evidence
- d@e (computer-59c902c5...): `rollback-4dca8039...` degraded since 2026-10-27 (updated 2026-10-30), target = applied
  `selfdev-4585da37...` (release 63a13ae9, gen 1). Head: desired fcddd72b (the rollback event) != effective ce1737e7,
  pending empty. Console (node-b vm-70070cf7.../console.log): 106 lines of
  "rollback-4dca... in degraded needs retry: rollback release unavailable: updater: pinned release unavailable", one
  per 60 s retry wake (not several per second).
- b@c: a 6-day-old awaiting_approval operation blocked engineering until the orchestrator rejected it.
- Pinned release availability inside the guest (`<updater root>/releases/63a13ae9...`) is not readable from the host
  (guest data.img). Host guest-releases/ holds VM images, not the updater store. The updater itself never deletes
  releases (no RemoveAll of releases in internal/updater/updater.go), so "retention removed it" is a hypothesis.

## Why no existing act settles d@e
- A degraded rollback leaves degraded only via `rollbackSelfDevelopmentOperation`
  (internal/agentcore/self_development_materializer.go:789), whose first step is `ReadPinnedManifest`
  (internal/updater/updater.go:822-830). It fails, so the operation can never reach rolled_back/failed.
- `choir self-dev rollback --to-operation X` (cmd/choir/selfdev.go) creates a NEW rollback operation
  (selfdev.StartRollback, operations.go:393); it does not touch 4dca. 4dca stays open, so engineering stays blocked.
  A rollback to any applied op may even succeed (if that release is retained) and still leave the hold.
- The "manual re-bind" named in the cause string (self_development_materializer.go:271) does not exist as a command
  or route; grep finds only that string and a comment at :966.

## Cluster (3 symptoms, one substrate)
awaiting_approval for days (b@c), degraded with unavailable release (d@e), rebase blocked behind a lock
(docs/problems/selfdev-blocked-behind-unperformed-rebase-2026-10-27.md): an open operation with no terminal path
that the owner or management can invoke, plus an unbounded hold whose cause is not on the ledger.
The state machine already allows the closing edges: awaiting_approval->failed, accepted->failed, degraded->failed
(operations.go:675-698), but only the materializer writes them, and only from journaled outcomes.

## Recommendation
1. Smallest fix for d@e: in the materializer, a degraded/rollback_pending rollback whose
   `ReadPinnedManifest` fails closes `degraded -> failed` (existing edge) with TerminalError "pinned release unavailable;
   computer stays on its current release", at self_development_materializer.go:789. Risk: head desired(fcddd) !=
   effective(ce17) stays; later ops bind to the live head so this is tolerated, but verify on d@e.
2. Common substrate fix: an owner act `self-dev abandon --operation --reason` (route beside rollbacks at
   api_self_development.go:204; CLI beside cmd/choir/commands.go:646, scope owner:acts) that moves any open,
   non-materializing operation to `failed` with the reason recorded on the tape (as recordMaterializationFailed does).
3. Surface the hold: after N retries in deferEngineeringAdmission (engineering_admission.go ~:213) write the
   blocker to the management/Texture commitment ledger, so it is not console-only.
Do 1 now; 2 and 3 are the class fix. Retention never deleting a pinned release is not shown to be the cause.
