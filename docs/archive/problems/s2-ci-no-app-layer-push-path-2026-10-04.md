# S2-g: CI had no app-layer push path — every runtime change rebooted guests

**Date:** 2026-10-04
**Status:** fixed by this commit series (see Resolution).
**Mutation class of this record:** green (problem documentation only).
**Station receiving it:** S2 layering-runtime-from-release, slice S2-g
(CI wiring for tracking computers, time-to-healthy, no reboot).

## Evidence

- `deploy-impact-classify` forced `mark_guest_boot_contract` for *every*
  autoputer selection ("Until a proved in-guest runtime replacement
  endpoint exists, every autoputer selection must rebuild the canonical
  guest boot closure and refresh mutable active computers"), including
  pure app-layer changes (skills, search, autoputer package code).
- Deploy's `/internal/vmctl/refresh` path reboots every mutable active
  interactive computer — minutes of downtime for a binary-only change.
- The layered-apply machinery (S2-e/d/c) existed in repo but had no CI
  wiring: no choir-builder on Node B, no guest-image-manifest beside the
  deployed guest artifacts, no mint/push step in the deploy script.

## Resolution (this commit)

- `deploy-impact-classify` emits `deploy_app_layer=true` when autoputer
  is selected without any guest-boot-contract trigger (no
  host_os/active_vm_refresh/vmctl_restart). Guest image changes still
  take the reboot path.
- Deploy script gains an "app-layer release push" phase:
  1. Ensures `choir-builder` (new `.#choir-builder` package) is deployed
     under `/var/lib/go-choir/services/choir-builder`.
  2. Runs it against `--base-manifest /var/lib/go-choir/guest/guest-image-manifest`
     (now exported by `mkGuestImage` + `environment.etc`) and the deployed
     `storedisk.erofs`; produces `app-layer-closure.nar` + receipt.
  3. PUTs the nar to corpusd's platform-updates blob endpoint.
  4. Mints a signed platform-update offer per active tracking/canary
     computer (closure_digest, layering_entrypoint, store_schema_version,
     base_commit, code_commit from the receipt) and pushes it through the
     vmctl autoputer proxy.
  5. Waits for time-to-healthy via proxied guest /health (the apply
     restarts the guest, so the push response is not the oracle) and
     records the duration in the deploy log + `app_layer_release`
     receipt artifact.
- `nix/autoputer-vm.nix` places `choir-guest-image-manifest` in
  `/etc/` so both the guest updater and the host builder resolve the
  same base identity.

Mutation class: red (deploy routing + release transport).
Rollback: revert the commit; the guest-boot-contract path returns.
Heresy delta: repaired (deploy-mode single-path); discovered: none new.
Refs: metamission v4 Orientation 2026-10-05 slice S2-g.
