# Node B Nix Store Retention v0 Ledger

## 2026-06-14 — paradoc opened

- Claim: Node B needs a separate Nix-store retention mission rather than
  folding Nix GC policy into the completed VM-state cleanup mission.
- Move: created `docs/mission-node-b-nix-store-retention-v0.md` as the next
  Parallax source program.
- Evidence: read-only probes showed 476G root filesystem with 357G used and
  117G free, `/nix/store` at 243G, `/var/lib/go-choir/vm-state` at 127G,
  system generations 489-496 retained, 9,409 absolute dead `/nix/store` paths,
  and large dead guest-image/playwright/runtime-deps/closure-info paths.
- Safety boundary: no live `nix store gc`, root deletion, guest image deletion,
  or retention config change was performed by this move.
- Expected ΔV: 0 against implementation obligations; this creates the source
  program for the next mission.
- Actual ΔV: 0.

## 2026-06-14 — read-only Nix root classification

- Claim tested: a typed root inventory can separate rollback-critical roots
  from stale/ad hoc roots before any Nix GC, generation pruning, or root
  deletion is authorized.
- Move: added `scripts/node-b-nix-root-report`, ran it against Node B, and
  saved the Markdown evidence packet at
  `docs/evidence/node-b-nix-root-classification-2026-06-14.md`.
- Evidence: captured at `2026-06-14T20:15:21Z`; root filesystem had
  `350.96 GiB` used and `122.16 GiB` available; `/nix/store` apparent size
  remained `243G`; `nix show-config` reported `min-free=0`,
  `max-free=9223372036854775807`, and `auto-optimise-store=true`; dead store
  path count was `9,423`.
- Root classification: keep current system, booted system, running-process
  roots, active service/guest runtime paths, root-owned flake registry cache,
  and the four newest system generations 493-496. Stale rollback candidates
  are generations 489-492. Temporary/ad hoc roots needing formal TTL/owner
  rules include `/tmp/go-choir-*result`, `/tmp/guest-image-*`, and
  `/tmp/go-choir-guest-image-new`. Invalid root:
  `/opt/go-choir/result -> /nix/store/srzb724qbl2s871jjkjja61zdfqwzv2j-prefetch-npm-deps-0.1.0`.
- Proposed policy: keep four system generations; set routine sweep target to
  180 GiB free with a 120 GiB emergency floor; set Nix daemon
  `min-free=120 GiB` and `max-free=180 GiB`; run weekly off-peak store
  optimisation rather than per-build `auto-optimise-store`; formalize result
  roots with type/owner/TTL and delete only through the reviewed service path
  or separately owner-approved manual action.
- Safety boundary: no manual `nix store gc`, root deletion, generation pruning,
  guest image deletion, service restart, or Node B config mutation was
  performed by this move. `nix-store --gc --print-roots` did emit its normal
  stale-temporary-root cleanup messages; no live GC was invoked.
- Verification: `scripts/node-b-nix-root-report --local --format json` passed;
  `.github/scripts/deploy-impact-classify-test` passed after adding the new
  report command to the operator-tooling ignore set.
- Expected ΔV: 4, covering root classification, generation target, GC/free-space
  thresholds, and optimisation cadence.
- Actual ΔV: 4. Remaining V=3: host config, CI/deploy, post-cleanup proof.

## 2026-06-14 — host retention policy patch

- Claim tested: the selected policy can be represented declaratively as host
  config/deploy-helper changes without touching guest images or manually
  deleting roots.
- Move: patched `nix/node-b.nix` to keep four system generations in the disk
  sweep, raise the sweep free-space floor/target to 120/180 GiB, set Nix daemon
  `min-free=128849018880` and `max-free=193273528320`, disable per-build
  `auto-optimise-store`, and enable weekly `nix.optimise` at `Sun 03:30` with a
  45-minute randomized delay. Patched `scripts/node-b-deploy-disk-preflight` to
  use the same four-generation and 120 GiB preflight floor. Marked that deploy
  helper as non-deployed workflow/operator tooling in deploy-impact.
- Parse evidence:
  - `nix eval --json .#nixosConfigurations.go-choir-b.config.nix.settings`
    confirmed `min-free=128849018880`, `max-free=193273528320`, and
    `auto-optimise-store=false`.
  - `nix eval --json .#nixosConfigurations.go-choir-b.config.nix.optimise`
    confirmed weekly optimiser settings.
  - `nix eval --json .#nixosConfigurations.go-choir-b.config.systemd.services.go-choir-disk-gc.serviceConfig.Environment`
    confirmed `GO_CHOIR_DISK_GC_MIN_FREE_KIB=125829120` and
    `GO_CHOIR_DISK_GC_TARGET_FREE_KIB=188743680`.
- Deploy-impact evidence: current diff classified as
  `deploy_host=true`, `deploy_host_os=true`, `deploy_vmctl_restart=true`,
  `deploy_ordinary_guest=false`, `deploy_playwright_guest=false`, and
  `deploy_active_vm_refresh=false`.
- Safety boundary: no live root deletion, generation pruning, Nix GC, guest
  image deletion, service restart, or Node B mutation was performed locally.
  The patch will deploy through the normal host OS path.
- Expected ΔV: 1 for host config implemented and parsed.
- Actual ΔV: 1. Remaining V=2: CI/deploy and post-cleanup proof.

## 2026-06-14 — deployed service run and settlement proof

- Claim tested: the reviewed routine retention policy can run on Node B through
  the declared service path, reclaim the dead Nix-store pressure, and preserve
  rollback and vmctl guest-image surfaces.
- Move: pushed behavior commit `e4bfae61`, monitored CI/deploy, verified
  staging identity, started `go-choir-disk-gc.service` once, and collected
  post-service read-only evidence.
- CI/deploy evidence: CI run `27510888401` succeeded. Deploy job
  `81310709649` succeeded. Deploy selected host OS only, skipped frontend
  install, skipped ordinary guest image build/install, skipped Playwright guest
  image build/install, restarted vmctl, and skipped active computer refresh.
  Staging health reported deployed commit
  `e4bfae6106ad33c9c8f021819b335041348f4078`.
- Routine policy evidence: `nix show-config` on Node B reported
  `auto-optimise-store=false`, `min-free=128849018880`, and
  `max-free=193273528320`. Timers reported `go-choir-disk-gc.timer` scheduled
  for `2026-06-15T00:04:15Z` and `nix-optimise.timer` scheduled for
  `2026-06-21T03:39:15Z`.
- Service evidence: `go-choir-disk-gc.service` ran from `20:26:51Z` to
  `20:27:30Z` with `Result=success` and `ExecMainStatus=0`; journal reported
  `9624 store paths deleted, 220991.43 MiB freed`.
- Post-cleanup storage evidence: at `2026-06-14T20:31:40Z`, `df -h` showed
  `/` and `/nix/store` at `476G` total, `174G` used, `299G` available, `37%`
  used. `du -sh /nix/store` reported `31G`; `/var/lib/go-choir/vm-state`
  reported `122G`. `nix-store --gc --print-dead` counted `0` store paths.
- Rollback/protected-surface evidence: system generations retained are `494`,
  `495`, `496`, and `497 (current)`. `/run/current-system` resolves to
  `/nix/store/0z8w1db6apxvqbvkkwf8s219bhgzdm5h-nixos-system-go-choir-b-26.05.20260409.4c1018d`;
  `/run/booted-system` resolves to
  `/nix/store/jmlg9chm4l2s8wn4synl464bvq16l9yw-nixos-system-choiros-b-26.05.20260306.aca4d95`.
  Ordinary and Playwright vmctl guest files all still exist:
  `vmlinux`, `rootfs.ext4`, `initrd`, `storedisk.erofs`, and `kernel-params`.
- Docs-only guard evidence: local deploy-impact probe for the docs-only mission
  updates returned `deploy_needed=false` and every deploy class false.
- Safety boundary: no manual root deletion, manual guest-image deletion, or
  ad hoc `nix store gc` was performed outside `go-choir-disk-gc.service`.
- Expected ΔV: 2 for CI/deploy and post-cleanup proof.
- Actual ΔV: 2. Remaining V=0; v0 settled. Future axes are explicit TTL/owner
  cleanup of ad hoc result roots, reboot convergence if desired, and moving
  repeated guest-image builds away from the long-lived launch host.
