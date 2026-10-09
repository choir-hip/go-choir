# A guest-runtime fix never reaches a computer that already has an app layer

Date: 2026-10-09. Status: open. Mutation class of a fix: red (deployment
routing). Related: [`fresh-realization-skips-file-hydration`](fresh-realization-skips-file-hydration-2026-10-09.md)
(version skew), [`layered-release-spa-underivable`](layered-release-spa-underivable-2026-10-09.md).

## Evidence

Deploy run 37890087817 (50c7074b) changed:
- `internal/store/dolt_maintenance.go` (guest runtime);
- `internal/vmmanager/manager.go` (vmctl and the guest boot closure).

The classifier chose:
- `deploy_active_vm_refresh=true`;
- `deploy_app_layer=false`;
- explanation: "autoputer runtime selected -> canonical guest boot closure + active VM refresh".

The deploy rebuilt the guest image ("guest image pointer updated") and refreshed the owner computer at 06:06.

After the refresh, the owner guest's `/health` reports
`build.commit = 39c0d991…, built_at 20261009022615`. That is the app-layer
release pushed at 02:26, not 50c7074b. The fixed GC guard did not run.
The boot log at 06:06:48 still reads
`dolt gc skipped: live=5 GiB`.

## Cause (verified in source)

- The guest boot wrapper (`nix/autoputer-vm.nix`, the
  `layering-entrypoint` block) execs `$CHOIR_UPDATER_ROOT/current`'s
  release whenever one is present. That is deliberate: a self-developed
  (divergent) computer keeps its layer across platform reboots.
- The classifier (`.github/scripts/deploy-impact-classify`, e527c169)
  treats the two delivery paths as exclusive. An app-layer push happens
  only when no reboot path is selected. Its comment assumes the reboot
  path "stays for base/kernel/systemd changes" and supersedes the layer;
  the boot wrapper never lets it.

## Consequence

Once a computer has accepted any app-layer release, it stays on that
runtime through every reboot-path deploy until the next app-only deploy:
- any change touching vmctl, the guest boot closure, or host OS
  alongside guest runtime code leaves layered computers on stale runtime;
- the deploy still reports success.

This is the skew already seen for sleeping and new computers, now
affecting active ones too.

## Fix direction

When the autoputer runtime is selected, always push the app layer to
tracking/canary computers. On the reboot path, push it after the
refresh. The push step already:
- builds against the freshly updated guest manifest;
- excludes held and divergent computers;
- records an applied count.

The reboot path still delivers the new base.

## First run with the fix (forced deploy 37892708774, 4e82febe): push ran, both targets refused

The classifier change worked: the reboot-path deploy built and staged the
app layer after the refresh (`app-layer-closure.nar` sha256 `11b40c0c…`,
10 closure paths). Both targets refused it; result `0/0 applied, 2 skipped`
after 465 s.

- **`vm-48bc0981…`:**
  - attempt 1: `updater refused apply: … layering entrypoint
    /mnt/persistent/choir-updater/store/cr0zi1f5…` (truncated);
  - attempt 2: `base event head is stale`.
- **Owner `candidate-fleet-e15cb89f…`:** `offer binds a different
  realization` on both attempts. The guest had been rebooted at 06:44:49
  by the same deploy and was thrashing at 4 GiB.

The run also failed its smoke test (proxy `degraded`, vmctl `unavailable`
during restart). Five ownerships are in `failed`.

Residual: delivery to layered computers now depends on the S2 push path's
realization/head binding
([`s2-app-layer-offer-bind-gaps`](s2-app-layer-offer-bind-gaps-2026-10-05.md)).
A refused push leaves the computer on the old layer, but the deploy is
not failed for it (`skipped` is counted, not gated).
