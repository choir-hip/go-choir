# Effects retry-path clustering: three substrate failures — 2026-08-17

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

## Cluster

The self-development operation retry path (Super → Texture join → CoSuper) failed three independent substrate ways this session, two now fixed:

1. **CoSuper restart reconcile aborted on terminal-unbound capsules** — `cec68e23` skips `CapsuleDisposition == Unbound` in the terminal cleanup branch. Fixed and verified live; `run:assignment-fa38b037` cancelled.

2. **Guest credential stale** — the guest's 4-minute capability expired between the refresh and the retry (idle > TTL), so corpusd refused renewal (`credential_revocation_epoch` 41 vs issued generation 40). Remedied by a fresh refresh immediately followed by the retry. Not a code change; the retry must run within the capability TTL.

3. **Texture Super opener provenance mismatch** (open) — the Super work item `38b96770-5fb8-585a-8234-db9e4dfbd331` carries `requested_by_run_id = 3b18a6d7-5fd4-5de4-86d2-c27954698548` (the original Texture caller run, now passivated) and empty `CreatedByRunID`. The retry's Texture caller is a recreated run `aa4fc186-ee42-4c25-823a-61bb506a0568` (pending). `validateLifecycleTextureControlTarget` → `lifecycleTargetWorkRequestedByTexture` requires the work item's creator run to equal the current caller run, so the opener returns `lifecycle invalid transition`:

```
apply self-development Texture Super opener: validate lifecycle Texture control target:
target work does not bind the open target obligation and caller provenance
```

## Remaining fix

`ensureSelfDevelopmentTextureJoin`/`ensureSelfDevelopmentTextureCaller` must not re-run the Super opener against a recreated Texture caller run whose identity differs from the run that created the already-open Super work item. Either reuse the original caller run (reactivate `3b18a6d7`) or accept the existing open Super work item bound to the same Texture agent without re-proving creator-run provenance.

## State

Operation `selfdev-b090bcd72d300fed17cb3f5a142f8595` is `executing`, no bundle. CoSuper terminal, credential fixed. Constructed freeze `7122f279` unchanged. Mode `propose_only` generation 1. No mail.
