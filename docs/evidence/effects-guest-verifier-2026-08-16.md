# Effects guest verifier wiring — certificate authority, mode still off

**Boundary:** execute (route map 9 red prep / route map 10). Not live proof. Not a live send. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Parent HEAD:** `cb4ff48f` (docs stamp of live kernel GET 200 after 7eee9f10 refresh)
**Mutation class:** red (guest autoputer wiring)

## Why

Live kernel-capabilities on epoch 270 returned 200 signed `KernelCapabilityReceipt`. `reconcileSelfDevelopmentMaterialization` still no-ops unless updater **and** verifier **and** control **and** route are all present. Autoputer already wired updater, route, and mode-authority control. It did not call `WithSelfDevelopmentVerifier`.

The guest already runs `choir-receipt-signer --mode verifier-control` on `/run/choir-verifier/authority.sock` and already exports `CHOIR_VERIFIER_AUTHORITY_SOCKET`. Autoputer's sandbox omitted that socket from `ReadWritePaths`.

## What landed

- `selfDevelopmentVerifierOption()` mounts `receiptsigner.NewClient(socket, ModeVerifier)` from `CHOIR_VERIFIER_AUTHORITY_SOCKET`
- skip if empty; absolute socket required
- log: `self-development verifier authority wired; mode remains off`
- autoputer `ReadWritePaths` includes `/run/choir-verifier`
- `/mnt/persistent/choir-signers` remains `InaccessiblePaths` (verifier key material stays out of the runtime namespace)

Mode is not set. Outbox stays unarmed. Genesis stays proxy-disabled. No operation is created. The materializer still does nothing until an authorized operation exists.

## Tests

- `TestSelfDevelopmentVerifierOptionWiresAbsoluteSocket`
- `TestSelfDevelopmentVerifierOptionSkipsMissingSocket`
- `TestSelfDevelopmentVerifierOptionRejectsRelativeSocket`
- existing route/updater/guest-control tests still PASS

Command: `direnv exec . go test ./internal/autoputer -count=1 -timeout 60s -run 'TestSelfDevelopmentVerifierOption|TestSelfDevelopmentRouteOption|TestSelfDevelopmentUpdaterOption|TestGuestControlOptions'`. PASS.

## What this is not

This is not live verifier proof on staging. Staging remains `7eee9f10` / epoch 270 until this commit deploys and the constructed guest is owner-scoped refreshed. Kernel GET 200 is not permission to set mode. Orange in-process rehearsal remains the only promote/outbox composition proof. Live proof (route map 10) remains unpaid.

## Ceremony

- **Conjecture delta:** Materializer inertness on the retained guest was an unmounted verifier client, not a missing signer unit.
- **Protected surfaces:** mode stays `off`; genesis stays proxy-disabled; outbox `Armed=false`; owner gates remain; verifier key material stays inaccessible to autoputer; OwnerRecovery checkpoints still cannot authorize promotion.
- **Admissible evidence:** the tests above; `nix/autoputer-vm.nix` already required `go-choir-verifier-signer.service` and exported the socket.
- **Rollback:** revert this commit. Guest materializer returns to no-op for missing `selfdevVerifier`.
- **Heresy delta:** `repaired` the missing `WithSelfDevelopmentVerifier` mount and sandbox socket path. `preserved` unmounted effects, fail-closed mode off, and constructed freeze.

## Next

Wait for staging deploy of this commit. Then `choir computer refresh` on `computer-03335285269bdba4f94377e56879f9e6` (not rematerialize, not restart). Re-probe: genesis 409, start 409, kernel 200, mode off. Do not set mode yet. Then red promote+restore without a live send. Do not send live mail.
