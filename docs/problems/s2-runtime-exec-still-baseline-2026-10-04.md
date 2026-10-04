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
