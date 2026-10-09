# A fresh realization never restores the owner's files and can overwrite them

Date: 2026-10-09. Found by the SH slice 3 lose-the-disk proof (operational
invariant O21). Mutation class of any fix: red (owner data durability).

## Evidence

Disposable `computer-9e4829e4fb32151553fd4b502a1aa04a` on staging (deployed
`eedd7b22`, slice 3 key delivery):

- 01:07:12Z owner file `sh-o21-proof.txt` written, `POST /api/files/sync`
  → root `06e22c9c…` (2431 files, 2441 chunks).
- 01:07:13Z realization removed; a fresh realization (new data image) was
  refused `projection_base_missing`, the repair ran, and
  `vm-2d8b21b5…` booted 01:08:05Z.
- Guest console: `privacy key delivered from custodian escrow … (head 4)`,
  base materialized, `computer event authority reconstructed (replay
  complete)` — but **no** `file tree hydrated` line, and
  `GET /api/files/sh-o21-proof.txt` never returned the file.

## Problem

`fileSync.HydrateIfNeeded` (`internal/autoputer/file_sync.go`) hydrates
only when the files root contains **no** regular files. A fresh
realization's files root is pre-seeded by the image (the platform source
tree under `files/Source/platform`), so hydration is always skipped on a
fresh realization of an existing computer.

Worse (code-derived; not yet observed — by 01:22Z the disposable had
published no new root): the periodic file sync (15 min) would upload the
seed-only tree as the computer's latest file root and cite it on the tape. The owner's files
are still in older roots in the content store, but the computer's current
state no longer contains them — silent data loss on every lost or moved
realization. This is the files row of the
[state homes inventory](../state-homes-inventory-2026-10-08.md), which
assumed hydration covered a fresh realization.

## Fix direction

A realization whose privacy key had to be delivered (missing over an
existing chain) has a fresh persistent volume — the key and the files live
on the same volume. Such a realization must hydrate the latest durable root
over the seeded tree, and file sync must refuse to publish until that
hydration succeeds, so a seed-only tree can never become the latest root.

Custody path verified in the same run: key-escrow transparency seq 36
`realization_delivery` for this computer, key digest `69437ebc…` equal to
the checkpoint job's audited key use (seq 35).

## Re-run after the fix (2026-10-09 02:21–02:36Z): still fails, two new causes

Lose-the-disk proof on disposable `computer-bb7eee83…`
(receipt [`evidence/sh-lose-the-disk-2026-10-09.json`](../evidence/sh-lose-the-disk-2026-10-09.json)):

- **Key: pass.** Fresh realization `vm-4db30e98…` booted 02:21:42 and logged
  `privacy key delivered from custodian escrow … (head 4)`. Transparency
  recorded `realization_delivery`, digest `067a8ff5…`.
- **Files: fail.** `GET /api/files/sh-o21-proof.txt` returned 404 on all
  90 tries (15 min). There was no `file tree hydrated` line.

Causes (console of `vm-4db30e98…`):

1. **Version skew.** The fresh realization boots the **base image**
   (`rbaf63…`, built at `eedd7b22`), which predates the hydration fix
   `c72c38c4`. App-layer pushes reach only computers that are active at
   deploy time. A new or newly realized computer runs the last image-build
   code until the next push.
2. **The fresh-volume signal lives only in memory.** At 02:33:49 the
   `39c0d991` app-layer push restarted the guest onto code that has the
   fix. That boot found the key already on disk, so it never marked the
   volume fresh. The lost flag was the only guard keeping file sync from
   publishing the seed-only tree as the latest root. Freshness must be
   durable on the volume and must not depend on which boot delivered the
   key.

The owner's files are still in older roots in the content store; the gap
is in which root the realization serves and publishes.
