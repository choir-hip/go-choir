# Emergency guest release restage (break-glass)

Date: 2026-09-18. Mutation class: **red** — touches a constructed
`ComputerVersion`'s served release store and the guest boot path. Use only when
the sanctioned update paths (self-dev materialization, `RestagePinnedRelease`,
`ImportBaseline`) are unavailable or the computer is pinned to a stale release.

## When to use

A retained owner computer is `snapshot_kind: constructed-computer-version`. CI
`DEPLOY_ACTIVE_VM_REFRESH` preserves it (records `active_computers.status=empty`),
so a platform deploy does **not** move its served frontend/runtime. The guest
serves the SPA from `CHOIR_UPDATER_ROOT/current` (a symlink into
`releases/<content-digest>`), and `current` only advances via
`RestagePinnedRelease` / `ImportBaseline` / self-dev apply. `computer refresh`
reboots onto the canonical guest image but **preserves** `current`, so the old
bundle persists.

Symptom: after a deploy, the guest still serves the old `index-<hash>.js` even
though the canonical image carries the new bundle.

## Why this works

`EnsureComputerSurface` runs on every guest boot. It calls
`updater.VerifyCurrentRelease(updaterRoot)`; if `current` is missing or invalid,
`ensureServingBaseline` builds a manifest from the immutable baseline
(`CHOIR_BASELINE_RELEASE_ROOT` = the image's `autoputer` package, which embeds
the frontend) and `ImportBaseline` stages it into `releases/<digest>` and points
`current` at it. Invalidating `current` therefore re-stages the **booted
image's** baseline — the same path that "accidentally" delivered the attachments
bundle on 2026-09-18.

Precondition: the canonical guest image the computer will boot must carry the
target baseline. `computer refresh` boots the canonical image (not the pinned
`code_ref`), so this holds after a deploy that rebuilt the image.

## Procedure

Run from the workstation with `node-b` SSH access. Substitute the computer's
`vm_id` (the `candidate-fleet-*` dir under `/var/lib/go-choir/vm-state/`).

```bash
CID="computer-03335285269bdba4f94377e56879f9e6"
VMID="candidate-fleet-e15cb89f25d963c220319b7b"   # = vm_id for $CID

# 1. Stop the computer (clean shutdown; data.img must not be live-mounted).
./choir computer stop --computer "$CID" --idempotency-key "restage-stop-$(date +%s)"

# 2. On node-b: attach data.img read-write and invalidate `current`.
ssh node-b "
  set -e
  DEV=\$(losetup --find --show /var/lib/go-choir/vm-state/$VMID/data.img)
  mkdir -p /mnt/guestrestage
  mount -o rw \"\$DEV\" /mnt/guestrestage
  cd /mnt/guestrestage/choir-updater
  # Rename (do NOT delete) so rollback is a symlink restore.
  mv current current.bak
  sync
  cd /
  umount /mnt/guestrestage
  losetup -d \"\$DEV\"
"

# 3. Refresh: boots canonical image; EnsureComputerSurface re-stages baseline.
./choir computer refresh --computer "$CID" --idempotency-key "restage-refresh-$(date +%s)"
```

Notes:
- `data.img` is a raw ext4 (no partition table) — `losetup` + `mount` directly.
- If `mount` reports read-only, detach stale loop devices (`losetup -a`,
  `losetup -d /dev/loopN`) and re-attach with `--find --show` then `mount -o rw`.
- `current` is a symlink to
  `/mnt/persistent/choir-updater/releases/<digest>`; `releases/` also holds the
  prior release dir(s). Renaming to `current.bak` keeps the old release for
  rollback.

## Verify

```bash
sleep 30
H="Authorization: Bearer $CHOIR_API_KEY"; C="X-Choir-Computer: $CID"
curl -fsS -H "$H" -H "$C" https://choir.news/ | grep -oE 'index-[A-Za-z0-9_-]+\.js'
curl -fsS -H "$H" -H "$C" https://choir.news/health   # expect status ok, new commit
```

Confirm the served `index-<hash>.js` and `MailApp-<hash>.js` match the target
bundle, and `build.commit` is the canonical deploy commit.

## Rollback

If the guest fails to boot or serves nothing, restore the prior release:

```bash
./choir computer stop --computer "$CID" --idempotency-key "restage-rb-stop-$(date +%s)"
ssh node-b "
  set -e
  DEV=\$(losetup --find --show /var/lib/go-choir/vm-state/$VMID/data.img)
  mount -o rw \"\$DEV\" /mnt/guestrestage
  cd /mnt/guestrestage/choir-updater
  mv current.bak current     # restore the prior symlink
  sync; cd /; umount /mnt/guestrestage; losetup -d \"\$DEV\"
"
./choir computer refresh --computer "$CID" --idempotency-key "restage-rb-refresh-$(date +%s)"
```

## Risk

- Mounting `data.img` while the VM is live corrupts the filesystem — always stop
  first and confirm `vmctl` reports `stopped`.
- If `EnsureComputerSurface` cannot stage the baseline (missing
  `selfdevUpdater`/`selfdevRealizationID`, or the booted image's baseline lacks
  the target), the guest serves nothing until `current` is restored. Keep
  `current.bak` until verified.
- This bypasses the self-dev receipt/audit trail; it is a break-glass repair,
  not the product update path. Record the operation in the mission/evidence log.
