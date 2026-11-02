# Plain self-development release declares no store schema epoch (2026-11-02)

Class: red (release manifest, apply path). Rollback: revert the commit.

## Evidence

Staging, 2026-11-02 20:18Z, b@c (computer-6c50d6e566143ae32cdb660faa60e24d,
non-layered releases): approved rebase selfdev-0452fc697f9a0e36c44998872b367177
failed with "updater: apply refused: the guest store floor is 4 (...) and the
release declares no store schema epoch, so nothing proves it is at or above the
floor" (`checkManifestAgainstFloor`, internal/updater/updater.go).

## Cause

The layered path stamps the epoch (`layeredSelfDevelopmentManifest`,
`base.StoreSchemaVersion = r.StoreSchemaVersion`). The plain path in
`self_development_materializer.go` built its `ReleaseManifest` with no
`StoreSchemaVersion`. Every computer has a store floor since the data system
cutover, so every plain self-development apply was refused.

Other mint sites: the platform update (`platform_update.go`) carries the
request's epoch; the genesis baseline (`updater.go`) is deliberately undeclared
and allowed by `allowUndeclared`; rollback re-finalizes a pinned manifest and
keeps its epoch.

## Fix

A plain release is the capsule's frozen `bin/autoputer` plus frontend. When the
bundle's `ReleaseBinary.Origin` is `reused_running`, the binary is the running
binary at Freeze time. Freeze now records the epoch compiled into it
(`ReleaseBinaryProvenance.StoreSchemaVersion`, bound into the bundle digest)
and the materializer declares that value, never its own `storeschema.Version`:
a planned restart between verify and mint could otherwise put a newer-epoch
binary in the materializer and over-declare. At mint the reused-binary-is-running
check is re-run; a mismatch settles the operation failed without minting. A
bundle frozen by an older binary has no field and stays undeclared (the floor
refuses it, fail closed), as does a capsule-built binary.
`checkManifestAgainstFloor` is unchanged. `MinStoreSchemaVersion` is left unset.

## Also

The "engineering: source patch without delegate" log line fired on rebase
assignments, which replay a carried patch and never call the delegate. It is
now skipped for assignments with a rebase lineage (`rebaseLineage`);
`delegate_use` is still recorded.
