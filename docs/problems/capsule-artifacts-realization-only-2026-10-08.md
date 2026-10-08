# Capsule subject artifacts and receipts live only on the realization disk

Date: 2026-10-08. Found by the [state homes inventory](../state-homes-inventory-2026-10-08.md)
(operational invariant O21). Mutation class of any fix: red (self-development
evidence, capsule artifacts).

## Evidence (code reading)

- `internal/capsule/executor.go:76-80`: "durable artifact authority
  (subjects/ and receipts/, named by durable capsule-subject:/receipt refs
  committed to the store) … artifactDir must live on persistent guest
  storage."
- `nix/autoputer-vm.nix`: `CHOIR_CAPSULE_ARTIFACT_DIR=/mnt/persistent/capsule-artifacts`.
- Subjects are written to `<artifactDir>/subjects/<digest>` (`executor.go:1278-1299`)
  and read back from the same path (`executor.go:130`). No upload of subject
  or receipt bytes to the platform content store was found.
- Engineering assignments and reports carry `capsule-subject:<digest>` refs
  (`internal/types/engineering_assignment.go:151, 524`).

## Problem

The store commits durable refs to bytes whose only copy is the realization's
persistent volume. When the realization is lost, replaced fresh, or moved
(hosted ↔ desktop), the tape and projections still name those subjects and
receipts, but they resolve to `ErrSubjectArtifactUnavailable`. Committed
self-development evidence becomes unverifiable after a disk loss, violating
O21 and the artifact-verified-success standing question.

Not yet observed on staging; the expected failure is a dangling
`capsule-subject:` ref after a lose-the-disk disposable test.

## Fix direction (not decided)

Treat subject and receipt bytes like file chunks: content-address them into
the platform content store (encrypted under the computer DEK where private)
before the ref commits, and hydrate on demand. The local directory then
becomes a cache.
