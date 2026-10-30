# M9a Deployed Proof — Platform Update Push + Pinned Restore

Date: 2026-09-26
Mission: choir-platform-update-push-restore-2026-09-26 (spine M9a)
Deployed build: `44e4169e4eca0cd7058d0154627b09a063209344` on staging
(`pr-20260926-44e4169e`), deploy job in CI run 36277989883.
Probe: `scripts/m9a_platform_update_probe.mjs` → `predicate_result:
satisfied` (exit 0), run at 2026-09-26 ~23:16Z.

## The proof

Computer: `computer-81695eb2108117eb31df6757c2f14d69` (fresh owner, fresh
machine — no ops-published base, no genesis-baseline ceremony).

Guest tape (corpusd `events/replay`, sequence order):

```text
1 genesis_imported           lifecycle-bootstrap-chain:...
2 effect_accepted            platform-update-accepted-upd-...-a
3 materialization_started    platform-update-started-upd-...-a
4 materialization_applied    platform-update-applied-upd-...-a
5 checkpoint_published       platform-update-checkpoint-published-...
6 route_projection_updated   platform-update-route-projection-updated-...
7 restore_requested          restore-...
```

`route_after_a`: route slot `committed_generation: 1` via
`TransitionBootstrap` under the platform-follow scope; code closure +
artifact program resolved against `platform-artifacts` —
`artifact+sha256://eb5c…/sha256/platform-update/eb5c…`.

Checkpoint: `c9e39f037b5a63013391520b5bc126c48635c1b149a8b3b4d2a59d699923263e`,
signed receipt `platform-control:868f96cca8726f99`, evidence class
`platform_follow` (no verifier fields, materialization-receipt bound).

Restore (product path `lifecycle/restore`, `vm_local` +
`computer_surface_frontend` scopes): `method: tape_reconstruct`,
`base_sequence: 0`, `tail_target_sequence: 6`, `tail_events_applied: 6`,
`witness_matched: true`, `original_denied: true`,
`frontend_restaged: true` — the platform-follow checkpoint is a live
restore operand on a baseless computer.

## What the deploy loop peeled off

Probe-driven fixes landed across four commits (each preceded by the
problem-doc update, per problem-documentation-first):

- `4153b5e4` — apply detached from the request that dies on guest
  restart (daemon + guest apply).
- `1d967302` — resumable tail (applied→checkpoint→route re-drive via
  tape, sweep extension, fetch-by-key checkpoint GET, bounded baseless
  replay fallback for `ReplayCompleteness`/`RematerializeFromTape`).
  Documented in
  `docs/problems/platform-update-stranded-tail-and-baseless-checkpoint-2026-09-26.md`
  strands 1–2 (`23e410ce`).
- `b920c85a` — `platform_follow` checkpoint evidence class
  (`CheckpointRequest.platform_follow`; corpusd verifies head/receipt/
  applied-event-kind joins server-side; verifier fields must be absent;
  route projection permitted). Strand 3 (`3bda9cb2`).
- `c61be3f1` — offer mint stages payload bytes into the shared
  `platform-artifacts` root as `sha256/platform-update/<digest>` and
  binds closure/program URIs there, so vmctl's `PinCode`/`PinProgram`
  verify real content. Strand 4 (`2cd34bc1`).
- `44e4169e` — `constructedOwnershipIdentity` recognizes the
  platform-follow evidence payloads (AcceptedEventAuthorizationEvidence
  + PromotionJoinEvidence) instead of only the selfdev constructed
  shape; unblocks `vmctl list`/`lookup` and the product checkpoint/
  restore ownership gate. Strand 5 (`a75d42b0`).

## Residual risk

- The resume window for a tail crash is one guest restart: if the tail
  dies *between* `applied` and a completed tail, resume-on-restart or
  re-push re-drives it, but an abandoned computer stays stranded until
  touched. The sweep covers restart-path only — acceptable for M9a;
  M11's restore edge and later cold-start sweeps inherit the gap.
- `platform_follow` checkpoints carry no verifier certificate: restore
  truth enforcement is the tape-reconstruction witness compare, not a
  verifier run. That matches the platform-update authority chain (the
  offer signature), but the class must not leak onto owner-recovery
  restore paths that expect its absence (exclusive-flags guard in
  `CheckpointFromRequest`).
