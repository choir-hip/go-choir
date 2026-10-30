# S2: host-service builder substrate landed — closure + base-identity + evidence receipt

**Date:** 2026-10-04
**Status:** builder substrate landed and proven on the host (Node B); the
updater-side materialization contract (closure + base-identity + GC-rooted
data-disk store-path replay) is the next slice and is not yet built.
**Mutation class of this record:** green. The builder code is `orange`
(new runtime surface, not yet wired to a protected path).
**Station:** S2 (choir-appdev-s2-layering-runtime-from-release-2026-10-01.md).

## What landed

- `internal/builder` — host-side builder. `Build` resolves the deployed
  guest base identity (sha256 of `choir-guest-image-v1` manifest + sha256
  of `storedisk.erofs`), builds/evaluates a flake app-layer installable,
  diffs its runtime closure (`nix path-info -r`) against the base
  manifest's bound store paths, exports exactly the base-absent delta via
  `nix-store --export`, and writes a canonical evidence receipt.
- `cmd/choir-builder` — CLI entrypoint. Runs on the host where nix +
  a writable store exist.

## Host evidence (deployed base `f61de45b`)

Installable `.#frontend` against the deployed `f61de45b` base:

```
runtime_path=/nix/store/mc8gx2jd3l6ghvs2pyhnybvx7wr22619-go-choir-frontend-0.1.0
closure_paths=1
exported=/tmp/builder-smoke/app-layer-closure.nar sha256=19b0bea4...
base_manifest_sha256=fb6da33f... base_storedisk_sha256=38f60c20...
```

`nix-store --import` on the exported nar resolved
`/nix/store/mc8gx2jd…-go-choir-frontend-0.1.0` — the blob is a valid,
importable narchive. Base store-path bindings (autoputer/updater/
capsule-broker/kernel/kernel-config) parsed from the manifest verbatim.

## Contract the builder proves

A release carries: the delta closure blob, the base-image identity it
resolves against (`guest_image_manifest_digest` + `store_disk_sha256`),
and derivation/input evidence (`derivation_path`, `output_path`,
`code_commit`). Every closure path not already in `base.store_paths` is
exported — the guest materializes exactly those, resolving base-present
paths in place in the read-only base store.

## Open (next slice — not this commit)

- `ReleaseManifest` cannot yet carry the closure/base-identity join; the
  updater decoders use `DisallowUnknownFields` and `Files` can't represent
  a Nix store-path graph (no dir/symlink entries). Requires a manifest
  extension + a closure materialization path that replays the nar at
  GC-rooted data-disk store paths and refuses on unresolved base.
- `builder` is not yet wired to a systemd unit or the self-dev release
  path; `cmd/choir-builder` is the operative entrypoint.
