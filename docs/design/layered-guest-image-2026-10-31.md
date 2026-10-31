# Layered guest image: a deploy ships only what changed (2026-10-31)

Status 2026-10-31: design and measured prototype. Mutation class of this
document: green. The prototype commit adds an unconsumed flake output and a
guest module only the layered variant imports (yellow: nothing launches it;
`guest-image`'s structure is unchanged, though any `flake.nix` edit moves the
floor digest once, see §8). The slices it orders are red (vmctl, VM
lifecycle, boot path).

Owner, 2026-10-31: minimize storage growth while minimizing memory spikes.
The owner put this structural fix inside the data system work
([one tape §5](one-tape-durability-2026-10-31.md#5-the-mac-and-the-hypervisor)
points here for the Mac/aarch64 side).

Out of scope: block dedupe of release files at install and retention by
reference. Another builder is doing those now. They do not compete with
this change: dedupe recovers bytes from the releases that already exist and
from the files that stay per release, and this design stops the bytes being
written in the first place (§5 compares the two).

## 1. What can go wrong (read first)

1. **An old vmctl launches a layered release with one drive.** The guest
   initrd waits for `nix-base`, fails to find it, and the boot fails.
   Guard: the vmctl that understands both layouts deploys first, while the
   fleet image is still single-layer (S1, §7). A release directory names the
   app image `storedisk-app.erofs`, never `storedisk.erofs`, so an older
   `releaseByID` refuses it (`release_missing`). The VM cold boots onto the
   legacy paths and never half-launches.
2. **A base is deleted while a retained release or snapshot still names it.**
   The release fails to load (`release_missing`) and its snapshots cold boot.
   That costs efficiency, not correctness, because `releaseByID` checks every
   file. Retention must count base references. This depends on the other
   builder's retention-by-reference work: a base is a referenced object and
   is never swept on its own age.
3. **Overlay stacking depth.** The kernel allows at most two stacked overlays
   (`FILESYSTEM_MAX_STACK_DEPTH = 2`). Today the stack is erofs, then the
   updater's S2 layering overlay over `/nix/store` (depth 1). Layered, it is
   two erofs, then the store overlay (1), then the updater overlay (2). That
   is still allowed, but a third overlay with the layered `/nix/store` as a
   lower fails with EINVAL. Capsules overlay the guest root (`/`, tmpfs), not
   the store (`internal/capsule/executor.go:651`), so nothing in the tree
   reaches depth 3 today. The cure that removes the risk: the updater's
   layering mounts one overlay with `lowerdir=/nix/.ro-app:/nix/.ro-base`
   and its private store as the upper, instead of stacking over
   `/nix/store`. That keeps the depth at 1. The d@e proof runs a capsule and
   a layered (S2) release on the layered base.
4. **Guest slab growth.** Every file the guest opens costs an overlay inode
   and dentry on top of the erofs ones, roughly 1 KB per file. A guest that
   touches 5,000 store files pays about 5 MB, and one that touches 20,000
   pays about 20 MB. That is guest RAM, which is precious. Gate on d@e:
   `Slab` in `/proc/meminfo` after boot and after an hour idle, measured
   against a single-image computer of the same shape, at most +8 MB.
5. **The app layer silently grows.** A path the base probe misses lands in
   the app layer, every deploy. This costs bytes, never correctness, because
   app is closure(real) minus closure(probe). Today the app layer is exactly
   the 25 per-deploy paths (§3). Guard: a CI check that refuses an app layer
   over 200 MB, naming the largest app paths.
6. **The base moves more often than expected.** The base derivation moves
   with nixpkgs (`flake.lock`), the guest Nix config (`nix/autoputer-vm.nix`,
   including unit text), the kernel, the guest scripts read at evaluation
   and the frontend lockfile. In October, 55 commits touched those inputs
   against 2,029 code commits. Each base move is one 3.34 GB image. If it
   moves on most deploys, the saving collapses. Measure the deploy-level
   rate for a week after the flip, as R9 does for the floor. §9 has the
   follow-up if the rate is high.
7. **Hibernated drives are missing at wake.** A Firecracker snapshot records
   its drive set, so a layered snapshot needs its base at wake. The base is
   addressed through the snapshot's own release (`expectedLaunchConfig`
   already rebuilds against the release), and `snapshotBoundPaths` must
   include the base path so a replaced file refuses the resume. A snapshot of
   a single-image release keeps restoring with one drive. The drive set comes
   from the release, never from global config.
8. **Data drive order.** The guest mounts the data volume by position
   (`/dev/vdb /mnt/persistent` in the deployed fstab, measured), not by
   label. The base drive is therefore appended after data and credential,
   and both erofs layers are found by label (`nix-base`, `nix-app`).
9. **Consumers that hash or open `storedisk.erofs`.** These are
   `internal/builder` (the base-identity leg is the sha256 of the whole
   3.4 GB file, read in full per host build), `internal/hostbuild/client.go`,
   `nix/node-b.nix` (`VM_STORE_DISK_IMAGE`,
   `CORPUSD_HOST_BUILD_BASE_STOREDISK`, the guest link check),
   `internal/server/debug_pagecache.go` (it marks an overlay with several
   lowers as `stacked`), `scripts/density/host_collect.py` and
   `scripts/res08_pagecache_probe.py`. Each moves to the layer pair. The
   builder's identity becomes the two layer store hashes, which also stops
   the 3.4 GB read on every build.

## 2. Measured facts (Node B, 2026-10-31, read-only)

Today's seven releases are 3,431,481,344 to 3,432,390,656 bytes of
`storedisk.erofs` each, about 24 GB. `dump.erofs --ls` of the seven store
roots: every release has 952 top-level store paths, and **the same 25 change
on every deploy, and only those**. Across all seven, 175 distinct paths are
not common, which is 25 × 7. The other 927 paths are identical in all seven.

| Set | Paths | NAR bytes | Share |
|---|---|---|---|
| Per deploy (fast) | 25 | 162,562,176 (163 MB) | 2.6% |
| Stable (slow) | 927 | 6,098,131,432 (6.10 GB) | 97.4% |

The fast set: `autoputer` 81.6 MB, `choir-updater` 30.7, `capsule-broker`
27.0, `choir-receipt-signer` 7.4, `choir-capture` 7.0, `system-path` 4.5,
`choir-delegate` 3.5, and 18 pieces of NixOS glue under 0.5 MB each: the
toplevel, `etc`, units, `closure-info` and the image manifest.

The largest slow paths (NAR): `libreoffice` 1.56 GB, `chromium-unwrapped`
0.74, `gcc` 0.29, `claude-code-2.1.300` 0.27, `go-1.26.10` 0.23,
`pandoc-cli` 0.22, the nixpkgs source behind `etc-nix-registry.json` 0.21,
`go-choir-frontend-deps` 0.18, `linux-6.18.57-modules` 0.14, `python3` 0.14,
`openjdk-minimal-jre` 0.09, `nodejs-slim` 0.09. The other ~900 paths make up
about 2.0 GB.

The non-store artifacts already share bytes across releases. `vmlinux`
(411 MB), `initrd` (26 MB) and `rootfs.ext4` (42 MB) carry Nix link counts
of 12 to 14, so they are identical across deploys and the release reflinks
share their extents. Only `storedisk.erofs` is new each time.

## 3. Prototype (built and measured)

`flake.nix` gains `packages.x86_64-linux.guest-store-layers` and
`guest-store-base-layer`. `nix/guest-layered-store.nix` is the guest mount
module: `/nix/.ro-base` (label `nix-base`) and `/nix/.ro-app` (label
`nix-app`), each erofs and needed for boot, and `/nix/store` as a read-only
overlay with `lowerdir = [ /nix/.ro-app /nix/.ro-base ]` (no upperdir).

- **Base** = the whole closure of a *probe* guest. The probe is the same
  NixOS config with the layered module, a fixed commit stamp
  (`base-probe`), and every `goChoirPackages` entry replaced by an empty
  stub, except the slow `frontendDeps`. Its derivation never sees Go
  source.
- **App** = closure(real layered guest) minus closure(probe), computed at
  build time with `comm`. There is no import-from-derivation.
- Both are built with the same multithreaded erofs-utils and
  `-zlz4 -T 0 --all-root` that microvm.nix uses today.

Evidence:

- **Base is invariant to Go code.** With a comment appended to
  `cmd/autoputer/main.go`, `guest-store-base.erofs.drv` kept its path
  (`xnhk38rw…`), while `guest-store-layers` and `guest-image` both moved.
- **Built on Node B** (offloaded to node-a-builder; 1 min 48 s wall clock
  for the whole set, including a Go compile and the base image):

  | Image | Bytes | Paths |
  |---|---|---|
  | `storedisk-base.erofs` | 3,343,863,808 (3.34 GB) | 950 (927 real, 23 probe glue) |
  | `storedisk-app.erofs` | 94,781,440 (94.8 MB) | 25 |
  | today's single `storedisk.erofs` | 3,432,390,656 | 952 |

  The app path list matches the measured fast set exactly, name for name.
  Its NAR total is 162,562,208 bytes. The base carries about 6 MB of probe
  glue (stub packages, probe units) that nothing references.
- **The overlay composes.** In a private mount namespace on Node B, both
  images were loop-mounted read-only and overlaid (app over base). All 951
  paths of the layered toplevel's closure resolved, zero were missing, and
  zero names were in both layers. `init` (ELF) and `autoputer` ran from the
  merged view. The mounts, loop devices and outputs were removed afterwards.
- **erofs multi-device (option b), tried.** `mkfs.erofs` 1.9.4 rebuild mode
  (`mkfs.erofs merged.erofs app.erofs base.erofs`) refuses compressed
  sources ("unsupported datalayout 2") and inline-tail sources. With
  `-Enoinline_data` and no compression, it produced a 0.8 MB metadata image
  with a `device_table`. So a shared blob means an *uncompressed* base.

Not proven yet: a guest boot from the two drives. That needs vmctl to
attach the second drive (S1) and is the first d@e step.

## 4. Options

| | (a) Two erofs drives, overlay | (b) erofs multi-device / chunk blob | (c) virtiofs from host store | (d1) block dedupe of single images | (d2) ship binaries through the updater's S2 layers |
|---|---|---|---|---|---|
| New bytes per Go-only deploy | **95 MB** (measured) | metadata ~1 MB + app blob ~163 MB uncompressed | 0 (the host store) | ~181 MB after the dedupe pass, but 3.4 GB is written first | per computer × N (data disks) |
| Base bytes | 3.34 GB lz4, once per base | ~6.1 GB uncompressed, once | n/a | n/a | n/a |
| Build per deploy | 0.6 s mkfs of 163 MB (measured on the fast set) | as (a) | none | the full 3.4 GB mkfs, then a dedupe read | none on the host |
| Host page cache | **one inode per base**, shared by every VM on every release that names it | one per blob, but ~1.8× the bytes for the same hot set (uncompressed) | host store pages shared, plus a cache in each guest | **per inode**: deduped extents are not shared page cache, so VMs on two releases cache twice | as today |
| Guest memory | +1 drive, overlay slab (§1.4) | no overlay; one mount | a virtiofsd per VM (tens of MB of host RSS) plus DAX gaps | as today | as today |
| Boot path | one more mount in the initrd | `device=` mount options tied to drive order | a daemon per VM before boot | as today | layered exec, which has its own failure history |
| Security epoch | release names its base; floor and epoch rules unchanged (§6) | as (a) | the host store is visible to the guest | as today | as today |
| Hibernate | drives from the release (§1.7) | as (a) | Firecracker has **no virtio-fs**; on Cloud Hypervisor, vhost-user devices are a snapshot gap | as today | as today |
| aarch64 / vfkit | two raw disk images; VZ attaches read-only raw images | as (a) | VZ has virtiofs (Mac only) | n/a | n/a |
| Verdict | **chosen** | later, only if the app layer itself needs sharing | rejected on Firecracker, density and the guest's view of the host | complementary, not a substitute | wrong owner: a platform deploy is not self-development |

(d3) Three layers (OS, tools, app) buy little. One nixpkgs pin moves the OS
and the tools together, so they move as one base.

## 5. Recommendation

**(a): two erofs drives, a content-keyed base reused across deploys and a
small per-deploy app layer, joined by a read-only overlay in the guest.**

- It cuts the bytes at the source: 95 MB per Go-only deploy against
  3.43 GB, about 36× less.
- It cuts the build: no 3.4 GB mkfs and no 3.4 GB Nix store write per
  deploy, so less garbage collection churn on Node B.
- It is the only option that **lowers host memory**. The base is one inode
  for every VM across releases, so after a deploy the old-release and
  new-release VMs share the base's hot pages instead of caching two
  3.4 GB images. That includes the jail `.shared` copy, which is keyed by
  source path and inode (`internal/vmmanager/jail_shared.go`).
- The guest-side cost is bounded and measurable (§1.4), and the mechanism is
  arch-neutral.

Expected bytes on Node B, with `KEEP_MAX` 8 releases:

- Go-only deploys: 8 × 95 MB + 3.34 GB ≈ **4.1 GB**, against ~27.5 GB today.
- Two live bases (a base moved inside the window): ≈ **7.5 GB**.
- The block dedupe the other builder is landing still applies to the
  shared non-store files and to the monolithic releases during migration.

## 6. Security epoch, rollback, hibernate, Mac

- **Binding.** `release.json` gains `layout: "layered"`, `base_id` (the
  base's 32-character store hash) and `base_store_path`. A release names
  exactly one base, and the base is never rewritten in place. Everything
  that forces a release forces its base.
  - A base CVE is fixed through `flake.lock` or `nix/autoputer-vm.nix`.
    Both are floor inputs (`nix/guest-floor-paths.txt`), so the floor digest
    moves, no snapshot resumes onto the old release, and every computer cold
    boots onto the new base. A base change outside the floor (the frontend
    lockfile) raises `security_epoch` under the existing rule.
  - To force a new base with no content change (a suspected host-side
    tamper), add a `base_generation` integer to
    `nix/guest-release-policy.json` and mix it into the base derivation.
- **Rollback.**
  - The image flip is a revert of the flake change: the next deploy builds
    one image again, and the vmctl that reads both layouts keeps launching
    it.
  - vmctl rolls back only after the image has.
  - A layered snapshot under a rolled-back image still resumes onto its own
    layered release while that release is retained, and cold boots after
    that.
- **Hibernate.** Covered by §1.7: the drive set comes from the release, the
  base is a bound path, and retention counts base references.
- **Mac/aarch64.** Erofs and overlayfs are arch-neutral, so the same two
  derivations build for `aarch64-linux`. The Mac already has an aarch64-linux
  remote builder (`/etc/nix/machines`). vfkit attaches two read-only raw
  disk images the same way. The bundle stays hypervisor-neutral
  ([one tape §5](one-tape-durability-2026-10-31.md#5-the-mac-and-the-hypervisor)):
  a Mac imports a computer and runs it on its own arch's base and app
  layers, built from the same commit.

## 7. Migration, by ring

Each slice is red. Commit lines name the class and the rollback.

- **S0 (this commit).** The prototype nix and the measurements. Nothing
  consumes them.
- **S1, vmctl reads both layouts.** Lands while the fleet is still on a
  single image.
  - `node-b-guest-release-materialize` copies the base once into
    `/var/lib/go-choir/guest-bases/<base_id>/storedisk-base.erofs`
    (reflink, read-only, root-owned, the same atomic `.tmp` and `mv`), and
    the app image into the release directory as `storedisk-app.erofs`.
  - `releaseByID` loads `base_id` and checks the base file.
  - `buildFirecrackerConfig` appends a read-only `base` drive last.
  - `snapshotBoundPaths` includes the base.
  - Retention keeps a base while any retained release names it (with the
    other builder's retention-by-reference).
  - The updater's layering mounts one overlay over both lowers (§1.3).
  - `debug_pagecache` reports `base` and `app` instead of `stacked`.
  - The builder's base identity is the layer pair (§1.9).
- **S2, the deploy also builds `guest-image-layered`.** That is the
  single-image artifacts plus `storedisk-app.erofs`, `layers.json` and the
  layered toplevel's `initrd` and `kernel-params`. It is installed as a
  second release, and vmctl launches it only for computers in the layered
  ring (a computer setting, R0 = d@e). The single image stays the fleet
  default ("on for at least one", widen by ring).
- **S3, widen R0 → R1 → … → all.** Then flip: `node-b.nix` binds the
  layered image as the guest image and the single-image build is deleted
  (no dual path, Operating Tempo rule 4). The old monolithic releases age
  out through retention as their snapshots pass the support window.

### Proof on d@e (`computer-59c902c5...`, R0)

Build steps:

1. `nix build .#guest-store-layers` on Node B. Read `layers.json`, then
   record the base and app sizes and the app path count, which must be 25.
2. Deploy S1 and S2, and set d@e's layout to layered.
3. Cold boot d@e. Health is green, and
   `findmnt /nix/store` shows `overlay lowerdir=…/nix/.ro-app:…/nix/.ro-base`.
4. Run one engineering capsule to verdict, and one layered (S2) apply with
   `layering-diag.log` showing `mount_ok`.
5. Run one idle hibernate and wake cycle (the R0 soak in
   [progressive delivery](progressive-delivery-2026-10-26.md)). The wake
   must resume onto its own layered release.
6. Push a Go-only commit. The base id is unchanged, the new release
   directory's exclusive bytes are ≤ 120 MB (`btrfs filesystem du`), and
   no new `guest-bases/` entry appears.
7. Memory gates, against a single-image computer of the same shape:
   - guest `Slab` and `MemAvailable` after boot and after 1 h idle;
   - the firecracker RSS;
   - `fincore` of the base file, showing one cached copy while d@e and
     another layered computer run.

Pass: steps 3 to 7 hold. A failed gate is documented first, in the mission
ledger.

## 7a. S1 landed (build step 2, 2026-10-31, branch lane/layered-image)

vmctl reads both layouts; the fleet stays on the single image. Deviations
and decisions from the text above:

- The layered release carries its own `layers.json`
  (`{"schema":1,"layout":"layered","base_id","base_store_path"}`, written by
  `node-b-guest-release-materialize` from the image's `layers.json`), not new
  `release.json` fields; `release.json` is built in Nix and unchanged.
- Bases live in `/var/lib/go-choir/guest-bases/<base_id>/storedisk-base.erofs`
  (`VM_GUEST_BASES_DIR` overrides; default is `guest-bases` beside the releases
  directory). `guestRelease.StoreDisk` is the app layer for a layered release.
- `VMConfig.BaseDiskPath` (omitted when empty) adds the `base` drive last;
  `snapshotBoundPaths` appends it after the single-image set, so single-image
  bound paths and fc configs are byte-identical.
- `GET /internal/vmctl/guest-releases` entries carry `base_id`; retention keeps
  a base while a remaining release, an in_use entry or a live `fc-config.json`
  names it, or it is under two hours old, and deletes none when a remaining
  release's `layers.json` is unreadable.
- Builder identity: `BaseStoreDiskBasePath` (flag `-base-storedisk-base`, env
  `CORPUSD_HOST_BUILD_BASE_STOREDISK_BASE`); the base leg is its `<base_id>`
  directory name, never a 3.4 GB read.
- The updater wrapper mounts one overlay over `/nix/.ro-app:/nix/.ro-base`
  when both are mounted; single-image guests keep `lowerdir=/nix/store`.
- `debug_pagecache` reports `layers: ["app","base"]` for the layered store
  overlay instead of `stacked`.

## 8. This commit's own effect

`flake.nix` is a floor input, so pushing this commit moves `floor_digest`
once and hibernated snapshots cold boot at the next deploy, as any
`flake.nix` edit does. The `guest-image` derivation is otherwise
structurally unchanged: `mkGuestVm` now calls `mkGuestVmWith` with the same
packages and no extra modules. Nothing in `node-b.nix` references the new
outputs. Do not push it alone. It lands with S1.

## 9. Follow-ups (investigated, not acted on)

- **The base is 6.1 GB NAR**, and four slow paths are about 2.3 GB of it:
  - `libreoffice` 1.56 GB;
  - `gcc` with binutils, 0.33 GB;
  - the nixpkgs source behind `etc-nix-registry.json`, 0.21 GB;
  - `go`, 0.23 GB.

  Each is worth an "is it needed in every guest" check. Trimming shrinks
  the one-time base cost and the guest's cold-read set.
- **`vmlinux` is 411 MB.** That looks like a kernel with debug info. It is
  shared across releases today, but every kernel change costs 411 MB per
  release kept.
- **If the base moves too often (§1.6).** Exclude the probe's unit and glue
  paths from the base and key the materialized base by the digest of its
  sorted path list rather than its derivation, so a unit-text edit reuses
  the installed base even though Nix rebuilds it once.
