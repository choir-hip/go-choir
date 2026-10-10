# A vmctl restart reboots a busy computer (2026-10-10)

Status: open. Found by M11 rerun 11. Mutation class of the fix: red
(vmctl VM lifecycle). It breaks the owner rule "deploys never restart a
busy computer".

## What happened (findings)

- 08:49:35Z: commit cc72a94c changed only `docs/**` and
  `scripts/m11_selfdev_episode_probe.mjs`. The deploy classifier has no
  rule for that script, so it took the conservative branch:
  `scripts/m11_selfdev_episode_probe.mjs -> unknown deployed path:
  conservative host + canonical guest image` (`deploy_vmctl_restart=true`,
  `deploy_active_vm_refresh=true`). The probe script runs only on a
  workstation.
- 08:57:23: vmctl restarted on that deploy. `reattach skipped for VM
  vm-795dcd90…` because the guest health check failed: the guest was
  busy mid-verification in rerun 11.
- 08:57:24: the next resolve for that user logged `killing orphaned
  Firecracker process for VM vm-795dcd90… (pid=3949451) before restart`.
  At 08:57:35 vmctl booted it again (epoch 12984).
- Effect:
  - the verifier assignment was restart-cancelled and re-cast;
  - the guest has failed health checks since 08:58:34 (marked degraded);
  - rerun 11 is probably lost.

## Cause

1. The deploy classifier treats unknown paths as "deploy everything".
2. The resolve path (`startExistingVM`) kills any live process behind a
   stopped ownership. It does not wait for the unmanaged-reap grace that
   reconcile applies (cf0969cf). A guest that was merely slow to answer
   one health check during vmctl's restart is treated as an orphan and
   rebooted. That makes a busy guest's restart a crash restart, and its
   open work is closed.

Introduced: cf0969cf kept this resolve behavior when it added the
grace to reconcile only, so the heresy pre-dates it but was not
repaired there.

## Fix direction

1. Classify probe and acceptance scripts (`scripts/*_probe.mjs`,
   `scripts/m11_*`) as operator tooling, which deploys nothing.
2. Resolve on a stopped `vmctl-restart` ownership whose process is still
   live:
   - retry reattach with a patient health wait;
   - if the guest still does not answer, refuse with 503 and
     Retry-After;
   - never kill it.

   Reconcile's grace and the protected-class rules remain the only path
   that powers off an unmanaged process.
