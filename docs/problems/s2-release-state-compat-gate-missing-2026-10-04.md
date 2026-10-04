# S2-d: no pre-mutation state-compatibility gate — a stale binary can exec against a newer store

**Date:** 2026-10-04
**Status:** fixed by this commit series (see Resolution).
**Mutation class of this record:** green (problem documentation only).
**Station receiving it:** S2 layering-runtime-from-release, slice S2-d
(state-compat gate).

## Evidence

The vm-3dc68688 crash-loop (metamission v4 Orientation 2026-10-05) was a
hand-staged August autoputer (`43310064`) exec'ing against an October
persistent store. `internal/updater/updater.go` `Apply` checked only
`BaseImageManifestDigest` (image identity); event-schema/reducer
compatibility was probed *after* restart by the health check — a post-mortem
fence. Nothing could refuse a release whose binary predated the guest's
persistent store, or whose declared base commit did not match the booted
image's `build_commit`.

The guest store carried no declared schema epoch at all: `internal/store`
applied `schemaDDL` + `ensureColumns` at open with no version marker the
updater could read.

## Resolution (this commit)

- `internal/storeschema` (new leaf package): `Version` is the persistent
  store schema epoch; `Write` persists `store-schema.json` inside the
  unified Dolt workspace on `store.Open`, `Read` decodes it. Kept leaf so
  the updater daemon and host builder need not link embedded Dolt.
- `ReleaseManifest` gains `store_schema_version`, `min_store_schema_version`,
  and `base_commit`. `Apply` refuses **before mutation** (no staging, no
  nar replay, no pointer swap) when:
  - persisted epoch > `store_schema_version` (stale binary vs newer store);
  - persisted epoch < `min_store_schema_version` (unmigratable store);
  - declared `base_commit` ≠ booted `build_commit`;
  - a declared window has no receipt wired/absent (fail closed).
- `cmd/choir-updater` `--store-schema-path` (env `CHOIR_STORE_SCHEMA_PATH`)
  wires `/mnt/persistent/state.texture/store-schema.json`; the autoputer
  unit exports the path.
- The corpusd mint (`internal/platform/platform_update.go`) carries the
  three fields through to the release manifest; the host builder stamps
  `store_schema_version` into `builder-receipt.json` from `storeschema.Version`.
