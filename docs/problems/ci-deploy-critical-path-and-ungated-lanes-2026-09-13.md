# Deploy Critical Path Serialization and Ungated CI Lanes

Date: 2026-09-13 UTC
Status: repaired and verified on hosted runs (dispatch run 34803283760)
Mutation class: red (deployment routing and CI check topology are protected surfaces)
Classification: CI assurance substrate

## Problem

Three independent inefficiencies in the current CI/deploy topology, measured on
hosted runs from 2026-09-13:

1. **The deploy Nix build is serialized after the test gate.** `deploy-staging`
   waits on `check` (correctly — untested code must not activate), then performs
   the host NixOS closure build on Node B inside the critical path. In run
   `34779534767` the `nix build` phase took **751s of a ~870s deploy**; every
   other phase (checkout, preflight, switch, service install, health probes,
   receipts) completed in seconds. Because `buildCommit`/`buildDate` are
   embedded via `ldflags` into every Go service, every commit forces a full
   rebuild of all service binaries — Nix can never cache them across commits.
   The build itself is pure and side-effect-free: it does not need to wait for
   tests, only activation does.

2. **`deploy-impact-classify` sends unknown `scripts/*` paths to a full
   host+guest deploy.** The `*)` catch-all marks `deploy_host_os` +
   `mark_guest_boot_contract` for any unrecognized path. Only
   `scripts/guest-signer-state-migrate` and `scripts/guest-signer-state-project`
   are baked into closures (via `nix/autoputer-vm.nix` `builtins.readFile`);
   every other script under `scripts/` is CI tooling, local developer tooling,
   or operator tooling run from a checkout. Verified instance: commit
   `e27a217a` (docs report + `scripts/generate_carrier_narrative_pdf_2026_09_13.py`)
   classified `deploy_needed=true` with `deploy_host_os=true`,
   `deploy_vmctl_restart=true`, `deploy_active_vm_refresh=true` and ran a full
   ~15-minute deploy including a 751s closure build — for a report generator
   that never reaches the host.

3. **`go-test-runtime` is not gated on the `go` plan output.** Its siblings
   `go-test` and `go-test-scale` carry `if: needs.plan.outputs.go == 'true'`;
   `go-test-runtime` does not. Git history shows the job predates the `plan`
   classifier (added in `dc91c2cc` before lane classification existed) and never
   received the gate — drift, not intent. On docs-only pushes it runs 8 shards
   (~3.5 min each, ~28 runner-minutes) whose result `check` explicitly ignores
   via `require_success "${{ needs.plan.outputs.go }}"`.

## Evidence

Run `34779534767` (commit `e27a217a`, docs+script only, success):

- `Deploy to Staging (Node B)`: 20:07:11 -> 20:21:49 (~14m38s)
- Phase `nix build`: 751s (Host NixOS closure); frontend bundle 55s in parallel
- Phase `nixos switch`: 109s (including one failed attempt + retry)
- All other phases: <= 5s each

Run `34781965874` (commit `379435c9`, docs-only, success):

- `Go Test (standard, agentcore/textureowner shard N)` x8: ~2m20s-3m38s each,
  all ignored by `check` because `go=false`
- No deploy (correctly classified docs-only)

Run `34770950209` (commit `def29358`, agentcore change, race-selected):

- `check` completed ~8m30s after start; deploy ran 17:22:20 -> 17:39:05 (~16m45s)

## Existing replacement opportunity

- The deploy job already computes exactly which attributes to build
  (`deploy_frontend`, `deploy_host`/`deploy_host_os`, `host_services`). The same
  selection can run on Node B *before* `check` completes: `nix build` is pure,
  writes only to the Nix store, and produces no activation. When `deploy-staging`
  later runs the identical `nix build` against the identical commit, the store
  realizes the cached outputs in seconds. Nix store locking makes a concurrent
  identical build safe (the loser waits or duplicates work, never corrupts).
- `deploy-impact-classify` already maintains an explicit allowlist of ignored
  and precisely-mapped script paths; extending it is the established pattern.
- `needs.plan.outputs.go` already exists and gates the sibling jobs.

## Required repair invariants

- Activation remains gated: `deploy-staging` must still require
  `check.result == 'success'` before any `switch-to-configuration`, service
  pointer install, service restart, or VM refresh. The prebuild job must never
  activate anything.
- The prebuild must build the identical attribute set the deploy would select,
  from the identical commit, in a checkout disjoint from `/opt/go-choir` so it
  cannot race the deploy job's `git reset --hard` + `git clean -ffdx`.
- The prebuild must be failure-safe: a failed or cancelled prebuild leaves the
  deploy path unchanged (deploy builds as today). It must not join the
  `staging-deploy-node-b` concurrency group (it must never cancel an in-flight
  activation) and must not appear in any `needs:` edge that gates `check` or
  deploy.
- Classifier changes must keep the `*)` catch-all conservative for genuinely
  new paths; only paths verified absent from host and guest closures may be
  ignored or precisely mapped.
- `scripts/guest-signer-*` must map to the guest boot contract (they are
  `builtins.readFile` inputs of `nix/autoputer-vm.nix`), not ignored.
- `go-test-runtime` gating must preserve the integration-tagged smoke step on
  shard 1 and the `check` contract (`require_success` on `go`).

## Rollback

Revert the bounded commits through a pull request. The prebuild job is purely
additive; removing it restores the serialized build. Classifier changes revert
to the conservative catch-all. No Node B rollback is expected: this commit
touches only `.github/**`, which the deploy classifier ignores.
