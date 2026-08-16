# Effects guest mode authority — WithSelfDevelopmentControl wired

**Boundary:** execute (route map 9 red prep / route map 10). Not live proof. Not a live send. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Parent HEAD:** `1a2c8ee6` (docs pin of staging 4543624b red product-path smoke)
**Mutation class:** red (guest autoputer wiring)

## What landed

Autoputer guest boot previously mounted only `WithOwnerRecoveryControl` so owner-recovery checkpoints could publish while effects mode CAS stayed unwired. Public start then returned 503 `self-development mode authority unavailable` on staging `4543624b`.

This slice mounts **both** options on the same exchanged guest credential:

- `WithOwnerRecoveryControl` — checkpoint publication only (unchanged)
- `WithSelfDevelopmentControl` — signed mode **reads** so start can fail closed at mode `off`

It does **not** set mode, arm the outbox, start the materializer, or delete owner gates. `verifyStartModeReceipt` still requires signed `propose_only`. Materialization still requires updater + verifier + route + realization together; those remain independently unwired unless already present.

## Tests

- `TestGuestStartRefusesAbsentModeBeforeAnyEffect` — unmounted control → 503, no operation, no event
- `TestOwnerRecoveryControlDoesNotAuthorizeProposal` — owner-recovery only → 503
- `TestGuestStartRefusesModeOffBeforeAnyEffect` — mounted control + platform mode `off` → 409 `current signed mode does not authorize proposal`, no operation, no event
- `TestGuestControlOptionsWiresOwnerRecoveryAndModeAuthority` — nil credentials mount nothing; non-nil mounts both options

Command: `direnv exec . go test ./internal/agentcore -count=1 -timeout 120s -run 'TestGuestStartRefusesAbsentModeBeforeAnyEffect|TestOwnerRecoveryControlDoesNotAuthorizeProposal|TestGuestStartRefusesModeOffBeforeAnyEffect'` and `direnv exec . go test ./internal/autoputer -count=1 -timeout 60s -run TestGuestControlOptionsWiresOwnerRecoveryAndModeAuthority`. PASS.

## What this is not

This is not red rehearsal of propose → consensus → promote → restore. Staging is still `4543624b` until this commit deploys. Autoputer selection rebuilds the canonical guest boot closure and refreshes **mutable** active interactive computers; `constructed-computer-version` realizations are preserved. The retained tape-recovery computer may therefore keep the old guest binary until a guest-boot refresh that is **not** rematerialize. Do not treat unit tests as that refresh.

## Ceremony

- **Conjecture delta:** Mode-off refuse is a guest signed-mode read, not an unmounted-control 503.
- **Protected surfaces:** genesis stays proxy-disabled; mode stays `off`; outbox `Armed=false`; owner-recovery remains a distinct control; OwnerRecovery checkpoints still cannot authorize promotion.
- **Admissible evidence:** the four tests above; G1 harness `TestSelfDevelopmentEffectsOffGuestHarness` already expected 409 and never enables effects.
- **Rollback:** revert this commit. Guest start returns to 503 on unmounted control.
- **Heresy delta:** `repaired` the owner-recovery-only mount that made mode-off refuse unreachable. `preserved` fail-closed propose_only, owner gates, and effects-OFF default.

## Next

Wait for staging deploy of this commit. Then confirm start on `computer-03335285269bdba4f94377e56879f9e6` returns 409 `does not authorize proposal` without setting mode and without a live send. If the constructed retained computer kept the old binary, perform a guest-boot refresh that is not rematerialize. Then red promote+restore. Do not send live mail.
