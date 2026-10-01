# S0 finding: deploy's active-VM refresh skips autoputer-internal changes

Date: 2026-10-01
Discovered by: S0a deploy pipeline (commit 2ee67928)
Station receiving it: S3 (boot surface) or the deploy-impact classifier owner

## Evidence

Commit `2ee67928` changed `internal/autoputer/boot_timeline.go` and
`internal/vmmanager/*` — guest binary internals, not the guest image
derivation's nix inputs. The `deploy-impact-classify` script classified the
push with `deploy_active_vm_refresh=false`, so `Deploy to Staging` ran the
host-side switch but did **not** refresh active computers. The owner
computer (epoch 991) kept running the pre-fix autoputer
(`guest_commit=2ee67928` only appeared after a manual
`/internal/vmctl/refresh`, which became epoch 992).

## Why it matters

The deploy is the canonical convergence path: `wait_for_autoputer_commit`
asserts the running guest binary's `deployed_commit`, but that gate is
only reached when `deploy_active_vm_refresh` fires. On autoputer-internal
pushes the pipeline can go green with active computers still running the
old binary — silently, with no deploy receipt distinguishing it. Any
behavior change that lives inside `internal/autoputer/` but outside the
guest-image derivation inputs is invisible to the refresh trigger.

## Not a fix here

S0a is observation. The classification rule lives in
`.github/scripts/deploy-impact-classify`; the refresh loop lives in
`ci.yml` (`deploy_active_vm_refresh`). Whichever of S3 / the deploy job
owns this needs the autoputer-package hash (or the runtime package
derivation output) in the refresh trigger, not just the image inputs.
