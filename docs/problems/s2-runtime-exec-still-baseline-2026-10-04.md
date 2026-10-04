# S2: applied releases never exec — runtime still runs the base-image binary

**Date:** 2026-10-04
**Status:** open. Source-traced. The layering acceptance requires the guest
to execute a per-computer app-layer closure; today an applied release's
binary is staged but never executed — the runtime keeps exec'ing the
immutable base-image autoputer.
**Mutation class of this record:** green (problem documentation only).
**Station receiving it:** S2 layering-runtime-from-release, as the
materialization/exec slice.

## Evidence

`nix/autoputer-vm.nix:93-139` (`autoputerRuntimeExec`) — the guest
`go-choir-autoputer.service` ExecStart wrapper hard-execs the *base image*
binary:

```
exec ${goChoirPackages.autoputer}/bin/autoputer "$@"
```

`goChoirPackages.autoputer` is a Nix store path baked into the read-only
base image EROFS. The updater's `Apply` (`internal/updater/updater.go`)
stages release files into `releases/<digest>`, swaps the `current` symlink,
and calls `service.Restart` — but the restart re-execs the *same* base
binary. Nothing in the release's `bin/` becomes the process image.

Consequence: a self-dev "apply" changes only what the *running base
binary* decides to serve off `current/` (the SPA frontend via
`internal/autoputer/computer_surface.go` `resolvedRoot`, plus whatever else

## Design (2026-10-04)

Two landed halves make this slice executable:

- **Base join** (`20880d75`): `ReleaseManifest.BaseImageManifestDigest` +
  `ClosureDigest`; `NewWithBase` wires the booted guest-image-manifest path;
  Apply refuses a base-mismatched release before mutation.
- **Closure materializer** (`b68357a1` + `a7052d2a`): `internal/updater/
  closure.go` decodes the real `nix-store --export` stream and replays it
  into a per-computer GC-rooted store (`$CHOIR_UPDATER_ROOT/store`), rooted
  per-release under `$CHOIR_UPDATER_ROOT/gc-roots/<releaseDigest>`. Wired
  into Apply between stageRelease and the pointer swap — fail-closed.

## Resolution (2026-10-04) — committed, deployed acceptance pending

The exec gap is closed in source across four commits; the deployed
acceptance (a real layered apply on a disposable staging computer) is the
remaining step and is staged in `scripts/s2_layered_update_probe.mjs`.

- **Producer** (`f5460bdc`): `platformUpdateOfferMintRequest` gained
  `base_image_manifest_digest`, `closure_digest`, and `layering_entrypoint`;
  `buildPlatformUpdateOffer` joins them into the release manifest.
- **Runtime exec** (`c7bb4a12`): `ReleaseManifest.LayeringEntrypoint` names
  the release's private-store-relative exec path; `materializeReleaseClosure`
  resolves it to the materialized absolute path and records it at
  `$CHOIR_UPDATER_ROOT/layering-entrypoint`. The `autoputerRuntimeExec`
  wrapper reads that record, enters `unshare -m`, and overlay-mounts the
  private store over `/nix/store` (lower=base EROFS, upper=priv store, work=
  `.overlay-work`), then `exec`s the recorded entrypoint — the release's own
  store-path binary, whose deps resolve through the merged view. Falls back
  to the base binary when no entrypoint is recorded or the overlay fails.
- **Stale-entrypoint fix** (`1be8bd72`): `materializeReleaseClosure` clears
  `layering-entrypoint` at the top of every apply, so a plain release after a
  layered one does not re-exec a stale layered binary.

Remaining before close: run `scripts/s2_layered_update_probe.mjs` on the
`1be8bd72` image — mint a real layered offer (closure.nar of a copied
autoputer store path + the disposable's base_image_manifest_digest), confirm
route promotion + the guest exec's the release store-path binary, and confirm
a base-mismatched layered offer fails closed (materialization_failed, base
keeps serving).


## What the fix must do

1. The guest autoputer service must exec the applied release's binary
   (`<updater-root>/current/bin/autoputer` or a GC-rooted private store
   path), falling back to the base image only when no current release is
   pinned (pristine pre-baseline boot).
2. The release binary's *dependencies* (closure paths) must resolve. The
   base store is read-only; the release's delta store paths live under a
   GC-rooted data-disk prefix. Options: (a) exec the release via a mount
   namespace that binds the per-computer private store over the matching
   `/nix/store/...` paths, or (b) materialize the closure into a layout the
   release's interpreter can resolve without `/nix/store` absolute paths.
   Option (a) is what makes real Nix binaries work unchanged.
3. The whole thing must stay inside the updater's journal/rollback contract:
   if the release binary fails health probe, `restorePrior` must re-point
   `current` and restart back onto the prior release (or base).

## Protected surfaces

- Guest runtime exec path / systemd unit (`nix/autoputer-vm.nix`)
- Updater apply/swap/restore (`internal/updater/updater.go`)
- Mount-namespace handling in the guest (new; touches NS_MNT)

## Verification

Deployed: apply an app-layer release whose `bin/autoputer` reports a
different version/build than the base; observe the restarted guest process
exec the release binary (comm/exe link or a logged resolved-exec path),
then a deliberately-broken release rolls back to the prior/base binary.

## Deployed failure receipt (2026-10-04) — overlay exec fell back to base

## Root cause found (2026-10-04) — exec works; the release binary is stale

The `layering-diag.log` instrumentation (`3c1cbaf6`) resolved the exec
question on staging: on `vm-3dc68688` the layered apply ran
`unshare_ok` → `mount_ok` → `exec_path_resolves` — **all three layering
stages succeed in the guest**. The service then crash-loops because the
applied release binary is build `43310064` (an August autoputer) running
against an October persistent store — a state-compatibility failure, not a
mount/exec failure. The base image is `672eb193`; the release is ~6 weeks
stale.

Two real defects fall out:

1. **The runtime exec was never the wedge** — the earlier `exec_resolved`
   pointing at base was the pre-`e605cdde` wrapper that had no fallback
   *and* no diag; with both, the layering chain is proven green on guest.
2. **Activation does not enforce state compatibility.** S2's contract
   requires `executable + frontend + state compatibility + effective event
   head` bound as one transaction; the apply execs a stale-binary release
   and crash-loops instead of failing closed or rolling back. The health
   gate / `restorePrior` path needs to cover "exec succeeded but the
   release cannot serve."

The overlay-vs-direct-exec question is now settled by the diag: the overlay
works; the prior `exec_resolved=base` reading was the no-fallback wrapper.
The open work is the state-compat/health gate, not the mount machinery.

The `s2_layered_update_probe.mjs` acceptance ran a real layered apply on a
disposable staging computer. The full chain — offer sign, push, stage,
closure.nar replay into `$CHOIR_UPDATER_ROOT/store/<hash>-layerdir-work`,
`layering-entrypoint` record (`za7slrais…/bin/autoputer`), `current` swap,
guest restart, route projection — committed and promoted. **But the guest
kept exec'ing the base binary:** `exec_resolved` =
`/nix/store/9v7ps5br…-autoputer-0.1.0/bin/autoputer` (the immutable EROFS
base), not the applied layerdir binary.

Every layering guard passed on the post-apply disk — `release_bin` non-empty,
executable, `priv_store` populated — so the failure is inside
`unshare -m`/`mount -t overlay`/`exec` in the guest's systemd private
mount namespace (`ReadWritePaths`/`InaccessiblePaths`). The same
`unshare`+`mount -t overlay`+`exec` sequence reproduces cleanly on the host
(EROFS lower + ext4 rw upper + the exact `dr-xr-xr-x` store upper), so it is
a guest-only divergence whose stderr is untrappable from outside (no serial
sink on disposable VMs; `journal_events` captures only unit lifecycle).

**Fix decision:** make the exec degrade gracefully rather than wedging on
the overlay. `autoputer` is a static Go binary — the store overlay exists
only so a release can pull *new* `/nix/store` dep paths; a same-toolchain
release needs none. The wrapper now (a) points
`CHOIR_BASELINE_RELEASE_ROOT`/skills at the applied layerdir so the served
frontend + any env-resolved assets come from the release, (b) tries the
overlay exec, and (c) on overlay failure `exec`s the release binary directly
with a logged reason. A release needing genuinely new closure deps still
gets the full overlay path; a fallback-exec release reports its own
`exec_resolved` store path, which the deployed acceptance reads as the
layering receipt.
