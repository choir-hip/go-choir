# Node B: the Nix store and host disk policy (2026-10-22)

Status: problem recorded (this commit); policy change follows on
`lane/fp0-nix-store-policy`. Companion to
[node-b-disk-fills-from-the-paper-data-image-2026-10-22.md](node-b-disk-fills-from-the-paper-data-image-2026-10-22.md)
(lane `lane/fp0-disk-pressure`, the paper's image and the compaction
pressure trigger), whose "Deploy floor" section this record takes up.

## Evidence (FP0, read-only on Node B, 2026-10-22)

- btrfs root 475 GiB. Free 139 GiB at 16:50, after an owner-approved early
  compaction of the paper (its store 33.0 to 14.5 GB) and deleting a
  week-old Dolt backup, which freed only 7.2 GiB (shared extents).
- Deploy R was refused at 14:28 by the deploy disk preflight
  (`DEPLOY_MIN_FREE_KIB` = 90 GiB in `scripts/node-b-deploy-disk-preflight`).
- Holders:
  - VM state about 150 GiB: the paper 51 GiB, regrowing about 85 to
    140 MiB/min until its 125.7 GiB cap or the 04:00 compaction; the
    candidate fleet dir 40 GiB, of which 32.4 GiB are two quarantined data
    images from 2026-10-07; the owner computer 15 GiB; test computers 9.6,
    12, 8.6, 8.7, 1.4 GiB; another user 3.1 GiB.
  - `/nix/store` 65 GiB. nix.conf: `min-free` 80 GiB, `max-free` 100 GiB,
    `auto-optimise-store` false, `keep-derivations` true. 51 auto gcroots,
    including 7 prebuild-results and builder releases under
    `/var/lib/go-choir/builder/releases/*/out`; 4 system generations. A
    guest release is about 3.9 GiB (storedisk.erofs 3.5 GiB, already LZ4
    compressed by mkfs.erofs, `nix/autoputer-vm.nix` storeDiskErofsFlags);
    the store holds 11 of them with little sharing (RES-13 reading,
    `docs/definitions/choir-launch-fp10-res-13-22-2026-10-21.md` R7).
  - platform-dolt 36 GiB, almost all `noms/oldgen` `.darc` archives; auto
    GC every ~30 min appends to oldgen (a 25.7 GB file at 11:12, 6.2 GB at
    14:54) and nothing collects oldgen. corpus-dolt 6.3 GB. (Out of this
    lane's scope; named so the totals add up.)
  - swap file 16 GiB (removal waits on tonight's 02:37 backup proof);
    builder 5.8 GB; offhost-backup 4 GB.
  - btrfs metadata DUP, 84% of 18.6 GiB used; 38 GiB unallocated at 14:55.

## What the repo does today (read from main at f0b1ee313)

- `nix/node-b.nix` `nix.settings`: `auto-optimise-store = false`,
  `min-free = 80 GiB`, `max-free = 100 GiB`. `nix.optimise.automatic`
  is already on, weekly (`Sun 03:30`); if it has been running, the store
  older than last Sunday is already hard-linked and only paths added since
  are not (FP0 checks `journalctl -u nix-optimise`).
- System generations are pruned to `+4` in three places that must agree:
  the 4-hourly `go-choir-nix-gc` unit, the daily `go-choir-disk-gc` sweep
  and the deploy preflight.
- Nothing prunes GC roots outside the system profile. The roots that hold
  store paths are:
  - `/var/lib/go-choir/prebuild-results/{frontend,nixos,service-*}-result`
    (ci.yml prebuild): one link per attribute, overwritten each prebuild,
    so bounded. The service links are the only roots of a service-pointer
    deploy's closure: `/var/lib/go-choir/services/<name>` is a `cp -a` copy
    whose binaries still name store paths (the ELF interpreter, ICU), and a
    service-pointer deploy does not switch the system, so when the service
    was built at a newer nixpkgs those paths are rooted by nothing else.
  - `/tmp/go-choir-*-result` (ci.yml deploy build): same shape and the same
    role as the prebuild links.
  - `/var/lib/go-choir/builder/releases/*/out`: written by
    `scripts/s2_station_acceptance.sh` (`REL_ROOT`), which was deleted on
    2026-10-12 (b9f195b1c). Nothing in the repo reads or writes that tree
    now. The live self-development builder is corpusd's host-build service
    (`internal/hostbuild`), which builds under
    `/var/lib/go-choir/host-build/work/<job>/result`, exports the app-layer
    closure into the blob store and removes the work dir (and its result
    root) when the job ends; a guest replays the exported closure into its
    own private store. So no live computer's release depends on a host
    store path rooted under `builder/releases`.
- The deploy floor (90 GiB) sits above `min-free` (80 GiB) plus a deploy's
  footprint. That is deliberate: on 2026-09-17
  ([archive mission, item 2](../archive/mission-mail-attachments-and-app-cutover-2026-09-17.md))
  `min-free` was 120 GiB and the floor 90 GiB, so every deploy build ran
  with free space under `min-free`; nix's auto-GC fired inside `nix build`,
  deleted the build's own not-yet-rooted inputs ("not a valid path") and
  the deploy looped. Lowering only the floor brings that back.
- No alarm fires on host free space or btrfs unallocated space.
  `go-choir-btrfs-headroom` balances once a day when unallocated is under
  30 GiB and says nothing otherwise; the deploy preflight speaks only when
  a deploy runs.

## The problem

1. **The floor blocks landings while the store keeps headroom it does not
   need.** With 90 GiB as the deploy floor and 80/100 GiB as nix's GC
   band, the host needs ~90 GiB idle to land anything, and the VM state
   (the paper's regrowth) eats into that by the hour. The floor cannot fall
   alone (2026-09-17); `min-free`/`max-free` must fall with it, in order.
2. **Store retention is unbounded outside the system profile.** Stale
   S2-probe roots under `builder/releases` keep releases alive that
   nothing uses. The generation count is written three times.
3. **New store paths are not deduplicated until Sunday.**
   `auto-optimise-store` is off; each deploy adds duplicate files (service
   packages, frontend bundles) that wait up to a week for `nix.optimise`.
4. **No warning before the floor.** The first sign of a low disk is a
   refused deploy (14:28) or ENOSPC in a guest; the btrfs metadata ENOSPC
   of 2026-10-16
   ([node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md](node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md))
   showed 135 GB free in df.

## Not this lane

The paper's image growth and compaction (lane `fp0-disk-pressure`), Dolt
oldgen collection, the swap file, and the quarantined candidate images
(32.4 GiB: an owner decision; they are evidence from 2026-10-07).

## Belief state and remaining error

- Believed: the store's reclaimable share outside the four system
  generations is the `builder/releases` roots plus whatever the 7 prebuild
  and `/tmp` result links pin that the current system does not. Not
  measured: which of the 51 auto roots pin which of the 11 releases. FP0's
  read (`nix-store --gc --print-roots`, `nix path-info -S`) settles it.
- Believed: a one-time `nix-store --optimise` saves little (single-digit
  GiB), because the weekly optimise already ran and the dominant files
  (storedisk.erofs, one per release, differing by build commit) never
  dedupe at file granularity. Not measured.
- Unknown: a deploy's real disk footprint (no `df` before and after in any
  log; bounded 4 to 8 GiB by the guest image).
