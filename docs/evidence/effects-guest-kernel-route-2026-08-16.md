# Effects guest kernel/route wiring — updater + route, verifier unmounted

**Boundary:** execute (route map 9 red prep / route map 10). Not live proof. Not a live send. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Parent HEAD:** `d61fcd91` (docs stamp of live 409 mode-off refuse after owner-scoped refresh)
**Mutation class:** red (guest autoputer wiring)

## What landed

Live kernel-capabilities on epoch 269 returned 503 `kernel capability authority unavailable`. `readKernelCapabilityReceipt` requires updater, route, owner, desktop, realization, and updater root together. Autoputer already wired `WithSelfDevelopmentUpdater` from `CHOIR_UPDATER_ROOT`; it did not call `WithSelfDevelopmentRoute`.

This slice mounts the computer-version route from guest env:

- `RUNTIME_VMCTL_URL`, else `PROXY_VMCTL_URL`
- `CHOIR_OWNER_ID` (skip if empty)
- `CHOIR_DESKTOP_ID` (default `primary`)
- absolute `http(s)` only

Guest cmdline already exports those identities (`nix/autoputer-vm.nix` `choir.vmctl_url` / `choir.owner_id` / `choir.desktop_id` / `choir.realization_id`; updater root `/mnt/persistent/choir-updater`). G1 `TestSelfDevelopmentEffectsOffGuestHarness` already expected the *second* refuse (`computer route identity unavailable`) after authority is mounted.

It does **not** mount `WithSelfDevelopmentVerifier`. Materializer no-ops unless updater **and** verifier **and** control **and** route are all present. Kernel GET is a read. Mode is not set. Outbox stays unarmed.

## Tests

- `TestSelfDevelopmentRouteOptionWiresOwnerAndVmctl` — `RUNTIME_VMCTL_URL` + owner → option
- `TestSelfDevelopmentRouteOptionSkipsMissingOwner` — no owner → skip
- `TestSelfDevelopmentRouteOptionSkipsMissingURL` — no vmctl URL → skip
- `TestSelfDevelopmentRouteOptionFallsBackToProxyURL` — empty runtime URL, `PROXY_VMCTL_URL` → option
- `TestSelfDevelopmentRouteOptionRejectsNonHTTPURL` — socket path → error
- `TestGuestKernelCapabilitiesRefuseUnmountedAuthority` — operations present, updater/route absent → 503 `kernel capability authority unavailable`
- `TestGuestKernelCapabilitiesRefuseMissingComputerVersionRoute` — updater + 404 vmctl → 503 `computer route identity unavailable`
- `TestGuestStartRefusesModeOffBeforeAnyEffect` — mode-off 409 still holds

Command: `direnv exec . go test ./internal/autoputer -count=1 -timeout 60s -run 'TestSelfDevelopmentRouteOption|TestSelfDevelopmentUpdaterOption|TestGuestControlOptions'` and `direnv exec . go test ./internal/agentcore -count=1 -timeout 120s -run 'TestGuestKernelCapabilitiesRefuseUnmountedAuthority|TestGuestKernelCapabilitiesRefuseMissingComputerVersionRoute|TestGuestStartRefusesModeOffBeforeAnyEffect'`. PASS.

## What this is not

This is not live kernel fail-closed on staging. Staging remains `21b79872` until this commit deploys. The retained computer is a constructed freeze (`7122f279`); global active-VM refresh preserves those rows. Owner-scoped `choir computer refresh` after deploy is required. A later live 503 (`computer route identity unavailable` / `kernel capability receipt unavailable`) is still fail-closed, not live proof. Orange in-process rehearsal remains the only promote/outbox composition proof.

## Ceremony

- **Conjecture delta:** Kernel capability authority is a guest route+updater mount, not an unmounted first-check 503.
- **Protected surfaces:** verifier stays unmounted; genesis stays proxy-disabled; mode stays `off`; outbox `Armed=false`; owner gates remain; OwnerRecovery checkpoints still cannot authorize promotion.
- **Admissible evidence:** the tests above; G1 harness already expected `computer route identity unavailable`.
- **Rollback:** revert this commit. Guest kernel returns to 503 `kernel capability authority unavailable`.
- **Heresy delta:** `repaired` the missing `WithSelfDevelopmentRoute` mount that made G1's second refuse unreachable on the retained guest. `preserved` unmounted verifier, effects-OFF default, and fail-closed promote.

## Next

Wait for staging deploy of this commit. Then `choir computer refresh` on `computer-03335285269bdba4f94377e56879f9e6` (not rematerialize, not restart). Probe kernel-capabilities: want past `kernel capability authority unavailable`. Do not set mode yet. Then red promote+restore without a live send. Do not send live mail.
