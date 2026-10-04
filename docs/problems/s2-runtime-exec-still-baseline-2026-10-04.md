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

Remaining: the guest autoputer must exec the release's binary, resolving its
`/nix/store` closure paths. The base store is EROFS read-only, so the
materialized private store must appear at the canonical `/nix/store` paths
for the release's own deps. Mechanism: the autoputer systemd `ExecStart`
wrapper resolves the release binary path — `$CHOIR_UPDATER_ROOT/current/
bin/autoputer` when a current release exists — and exec's it inside a mount
namespace (`unshare -m`) that bind-mounts each materialized private-store
path onto its canonical `/nix/store/<base>` target (base-present paths
resolve through the read-only EROFS unchanged). CAP_SYS_ADMIN in the
autoputer unit or a `unshare`-based wrapper supplies the mount namespace;
rollback is `restorePrior` re-pointing `current` and restarting onto the
prior/base binary. Refuses to layer when a closure ref is absent from both
the private store and the read-only base.
the base binary reads from the release dir). The deployed autoputer's own
code never changes without a VM image rebuild. This is exactly the gap the
S2 layering station exists to close — most updates should stop rebooting
VMs, and an app-layer change should land a *new runtime*, not just new
served assets.

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
