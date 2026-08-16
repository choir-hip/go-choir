# Effects checkpoint imports trusted image baseline for SPA — 2026-08-16

**Boundary:** execute. Not live checkpoint. Super was not started. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Observation:** `docs/evidence/effects-replay-eligible-spa-underivable-2026-08-16.md`

## What landed

When updater `current` is missing, checkpoint bind uses the same trusted baseline as genesis/kernel: `CHOIR_BASELINE_RELEASE_ROOT` under `/nix/store/`, route CodeRef/ArtifactProgramRef, then `ImportBaseline`. That stores the running image (including `frontend/`) so restore can restage the same digest. Untrusted baseline roots still 409 `served SPA is underivable`. Genesis stays proxy-disabled.

Owner-recovery publish now keeps already-prefixed `code:` / `artifact-program:` refs instead of double-prefixing genesis baseline labels.

This does not rematerialize. It does not invent `choir computer create`. It does not start Super. It does not send mail.

## Tests

`go test ./internal/agentcore -run 'TestCheckpointBind|TestCheckpointComputerVersion'`

## Unchanged

Do not run live checkpoint until staging has this guest and another owner-scoped refresh. Genesis 409. Armed=false. Super not started. Constructed freeze remains 7122f279.
