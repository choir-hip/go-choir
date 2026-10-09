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

## Owner computer: files present before the 2026-10-07 restore are absent now (2026-10-09)

Read-only comparison: each image was mounted `ro,noload` from a loop device; only path and size were listed, no content. Compared:
- the owner's two quarantine images in its VM directory;
- the post-compaction reflink backup (= the current disk's `files/`).

| Image | `files/` entries | Paths absent from current |
|---|---|---|
| current (`pre-compact-20261009T053125Z`) | 2,386 (144 MB) | — |
| `data.img.quarantine-1-39ddc7f2…` (10-07 11:46, pre-restore) | 2,152 (93 MB) | **484**: `files/Source` 432, `files/.choir` 37, `files/System` 2, and at least 5 `*.texture` documents |
| `data.img.quarantine-1-e3dacdac…` (10-07 22:38) | 2,382 (144 MB) | 3 (`files/Source`) |

**Hypothesis, unconfirmed:** the 10-07 restore did not re-hydrate the owner's file root. That is the same gap this doc records for lose-the-disk.

**Alternative:** the owner or an agent deleted these files after the restore.

Distinguish the two by checking the platform file-root chain for this computer: whether a recorded root still lists these paths, and when each was removed.

Until then, both quarantine images are retained as recovery sources. The owner directed "delete images not required for recovery"; these are required.

## Cause 1, version skew: owner decision and what it implies (2026-10-09 ~14:30Z)

Cause 2 is fixed in 799097e3 (durable fresh-volume marker). For cause 1 the
owner chose **"Layer + move base"** over "always reboot path" and "layer new
realizations": keep pushing app layers to running computers, and also move
the base image to the deploy's commit without rebooting them.

What the deploy job does today, read from source:
- The base image is a symlink, `/var/lib/go-choir/guest` → a store path.
  Only the NixOS activation script `go-choir-guest-image`
  (`nix/node-b.nix:920`) moves it, during a host switch.
- With the shared Go build, any Go change alters every host service
  package. A host switch made only to move the base would restart every
  host service (proxy, vmctl, corpusd, auth and the rest).
- The layer builder joins against the current pointer's manifest
  (`ci.yml`, app-layer push). Apply refuses a layer whose base digest
  differs from the computer's booted base (`internal/updater/updater.go:276`),
  and boot skips one (`nix/autoputer-vm.nix:166`).

So "move base" needs, in order:
1. **A standalone cutover.** The activation logic moves into a script the
   deploy job can run without a host switch. The new image is rooted
   against garbage collection, because the running system closure no
   longer references it.
2. **Order inside an app-layer deploy:** build and push the layer against
   the bases the targets actually booted, *then* move the pointer.
3. **A base registry.** After a move, running computers stay on older
   bases until they reboot. Every later layer must be built per distinct
   booted base among its targets, so each such base stays registered and
   rooted.
4. **Retirement.** A base is unrooted once no ownership was booted from it.

The other options, for comparison:
- **Always reboot path.** Deletes the platform app-layer path from CI, and
  each runtime deploy reboots active computers. No per-base builds.
- **Layer new realizations.** vmctl installs the latest layer before a new
  realization boots, so bases change only on reboot-path deploys.

### Decision (2026-10-09 ~14:40Z): always reboot path

Owner, on seeing those implications: "i want things to be updated, and also
fast and use minimal resources and the code to be secure and maintainable."
Against those criteria the agent chose **always reboot path**:
- *Updated:* every computer and every new realization runs the deploy's
  commit; skew cannot occur.
- *Fast:* a VM refresh is about 30 s of replay plus boot. At today's handful
  of active computers this is no slower than the app-layer push (up to
  465 s when its offers were refused).
- *Minimal resources:* no per-base layer builds and no retained old bases.
- *Secure, maintainable:* it deletes a deploy path instead of adding a base
  registry, a cutover script and retirement.

Layering stays for Gate 2 self-development releases. Revisit when many
active computers make serial refresh slow; the next step then is parallel
refresh, not per-base layers.

Change: `deploy-impact-classify` sends every guest-runtime change through
the canonical guest boot path (base rebuilt, active computers refreshed),
and the platform app-layer push block is removed from the deploy job.

## Resolved: lose-the-disk passes on staging (2026-10-09 14:45Z)

Run of `scripts/sh_lose_the_disk_proof.mjs` against base image 799097e3
(receipt [`evidence/sh-lose-the-disk-2026-10-09T14-44-09-081Z.json`](../evidence/sh-lose-the-disk-2026-10-09T14-44-09-081Z.json)):
a fresh disposable computer wrote and synced a private file (2,460 files,
root `8bfd4753…`); its realization and volume were removed; the next
realization (a different VM) logged `privacy key delivered from custodian
escrow … (head 4)` and `file tree hydrated 2460 files from CAS root
8bfd4753…`, and `GET /api/files/sh-o21-proof.txt` returned the written
content 67 s after removal. Both causes are closed: hydration on a seeded
tree (c72c38c4), and the fresh-volume flag now durable (799097e3) with
every runtime deploy on the reboot path (a47122de), so no skew.

An earlier attempt at 14:30Z failed in the proof harness, not the product:
the script registered an account but never made a computer-routed request,
so no computer was realized (fixed in 32a1c2e8).

**Residual, bounded:** for the first 40 s after removal, recovery was
refused with `projection_base_missing` (`internal/recoveryplan/recovery_plan.go:110`).
Trace: the computer was removed ~10 s after creation, before its first
verified projection checkpoint, so the watermark was 0. The
`go-choir-checkpointd` timer (every 1 min) published the base (target 4) at
14:45:20 and the next resolve succeeded. A computer that loses its disk
before its first checkpoint is unavailable for at most about a minute; no
state is lost, since the chain is intact. Not worth a change now.
