# Guest tools that load and unload live (2026-11-09)

Status: design, revision 2 after the tier 2 review (Devin ACCEPT_WITH_CHANGES,
Sonnet SEND_BACK on revision 1; both named the same cheaper route). Revision
1's mechanism, a Firecracker drive swap per load, is kept only as phase 3. Station L3.5 of
[the ordinary prompt path](../definitions/choir-ordinary-prompt-path-2026-11-09.md).

Owner, 2026-11-09: "a computer could be running without a particular tool.
Then you load that tool, and only then it goes into memory, and then you use
it, and then you could even offload that tool and clear it from memory
without restarting the computer. If that's possible, we should make it
happen. If it's possible, to keep it secure as well." And: "say we have a
video editing desk, or something for Blender or CAD. How could this work
with packages from the library as well?"

Builds on [guest tool layers](guest-tool-layers-2026-10-31.md) (section 9
named on-demand attach as a later option and its obstacles) and
[the layered guest image](layered-guest-image-2026-10-31.md).

## 1. What a tool is, really

A tool is a set of store paths (a closure minus the floor) that some
process on the computer executes for a while. The record of truth is the
release's `layers.json`: the layer file, its digest and its store paths. A
computer needs the bytes only while a process runs them. So the honest unit
is a **mount with a lifetime**, not a property of the image.

Today a tool layer is a lower of the `/nix/store` overlay, fixed at boot
(nix/autoputer-vm.nix, the `/nix/.ro-tool-*` loop). Overlayfs cannot add or
drop a lower under a live mount. That, and nothing in Firecracker, is why a
tool cannot come or go without a cold boot.

## 2. The physics

- **Memory.** An erofs mount nobody reads holds a superblock and a few
  inodes: kilobytes. A tool in use holds the pages its processes touched in
  the guest page cache (Chromium about 150 to 300 MB touched; Claude Code
  about 100 MB; LibreOffice 300 MB or more). Unmounting the filesystem frees
  its whole page cache in the guest. So "load" costs nothing until use, and
  "unload" is what returns memory. L3 already gives the first half; the
  second half is the new thing.
- **Host.** One layer file per release on the host, shared by every
  computer (the jail's `.shared` copy is keyed by inode). The host page
  cache for a layer is shared and evictable; a layer no VM reads costs none.
- **Time.** A load is a Firecracker `PATCH /drives/{id}`, a block rescan and
  an erofs mount: tens of milliseconds, plus the first page faults. Budget:
  load under 2 s at p95 before the tool's first exec.
- **Drives.** Firecracker drives are fixed at boot (no hotplug), but a
  drive's backing file can be replaced while the VM runs (`PATCH
  /drives/{id}` with `path_on_host`; the guest sees a virtio-blk config
  change). So the slots stay fixed and only what is behind them moves. Slots
  are bounded (`maxToolLayers` 8).

## 3. Review of revision 1, and what changed

Revision 1 swapped each slot's backing file through Firecracker's drive
`PATCH` on every load, with a symlink farm in the base pointing tool store
paths at `/nix/.tool/<layer>`. Both seats found the same gate:

- **Mount namespaces.** The autoputer runs in a private mount namespace
  (`unshare -m --propagation private`, nix/autoputer-vm.nix:266-268), and
  tools run inside capsule chroots that bind only `/nix/store`
  (internal/capsule/executor.go:2666, 2713-2719). A mount made later in the
  root namespace reaches neither, so a loaded tool would still be absent for
  every real consumer, and an unmount in one namespace frees nothing while
  another holds the mount.
- **Shared paths.** office and browser share 208 store paths (633 MB); a
  symlink has one target, so loading one layer alone could dangle the
  other's libraries.
- **Snapshots.** `expectedLaunchConfig` recomputes tool disks from the
  profile and refuses a wake whose launch hash differs
  (manager.go:2030, snapshot.go:639-641); a live drive swap breaks every
  wake unless the slot table is persisted, replayed and retained.
- **Cheaper route.** Both seats: the owner's memory outcome needs no drive
  swap. An attached layer nobody reads costs no page cache, and the page
  cache a tool used can be dropped without a restart.

So the design splits by what each phase buys.

## 4. Phase 1: memory only while used (this build)

Keep the proven layout: tool layers are store overlay lowers, attached at
boot (L3). Add **residency control**:

- **Load** is the first page fault, as today. Nothing to build.
- **Unload** is eviction. A guest helper, `choir-tool evict <layer>`, walks
  the layer's own erofs mount (`/nix/.ro-tool-<layer>`, root namespace) and
  calls `posix_fadvise(POSIX_FADV_DONTNEED)` on each file. Page cache
  belongs to the erofs superblock, which every mount namespace and capsule
  bind shares, so one walk evicts the layer's pages for all of them. Pages a
  live process has mapped stay (the kernel keeps them); the rest go.
- **When.** The helper runs when no process has an executable or mapping
  under the layer's store paths (a `/proc/*/maps` scan; psmisc is not in
  the guest), after an idle grace measured from the tool exec log, and at
  once at the balloon's reclaim threshold.
- **Security.** Nothing changes: eviction drops only clean read-only pages;
  the delegate's spend authority stays the gateway grant.
- **Proof on d@e.** Cold boot, record `Cached` and the layer's resident
  pages (`fincore` or a `mincore` walk, the helper prints it). Run a
  Playwright page (browser layer resident rises by about 150 to 300 MB),
  let it exit, evict: resident pages for the layer fall to near zero and
  `Cached` falls by the same amount. Same for a Claude delegate round. A
  tool exec after eviction runs (pages fault back in).

Cost: one small guest helper and a timer. No vmctl, no snapshot change, no
new channel. Rollback: revert.

## 5. Phase 2: the tool is absent until loaded

For the owner's "running without a particular tool": the tool's store paths
do not resolve until it is loaded.

- Drive set stays static at boot (each profile-allowed layer's real file
  behind its slot), so snapshots, launch hashes and retention are unchanged.
- Tool layers leave the store overlay; the base carries a symlink farm for
  their store paths pointing at `/nix/.tool/<layer>/<hash>-<name>` (the
  image root is the store directory, flake.nix:622-627), excluding paths the
  base or app already carry. Each shared path has one owner layer, and
  `layers.json` records each layer's `needs`, so `load(L)` mounts the
  owners' closure (or the desktop-libs layer is built).
- Load and unload are guest-side mounts, performed in the namespaces that
  consume them: the autoputer mounts in its own namespace;
  `prepareCapsuleRoot` binds `/nix/.tool` recursively (read-only, nosuid,
  nodev) with slave propagation so a load reaches live capsules; the root
  namespace mounts for the operator shell. Unload unmounts everywhere and
  requires no consumer (a `/proc/*/maps` scan; killing a consumer capsule
  is explicit and visible). `blockdev --flushbufs` before each mount.
- System PATH entries for tools (autoputer-vm.nix:1074-1085, 1192, 1204)
  go through the exec wrapper, which loads on a missing tool and waits a
  bounded 2 s. The base's `neededForBoot` tool mounts become non-blocking.
- Proof adds the capsule case: Playwright inside a capsule job sees the
  loaded browser; after unload the browser's store paths do not resolve.

## 6. Phase 3: tools from the Library (live attach)

A Library tool is not in the release, so its file is not behind any slot at
boot. Only here is the drive swap needed:

- Generic slots boot on placeholders; vmctl swaps one to a tool package's
  layer through `PATCH /drives/{id}` (Firecracker 1.17 supports it on a
  running VM) after checking the package is installed on this computer and
  its digest matches.
- The guest asks through the existing tap-source-bound guest endpoint
  pattern (handlers.go:1433-1444, route_authority.go:377), naming a package,
  never a path; vmctl resolves names against the launch record; accepted
  swaps are rate-limited.
- Each layer file is hard-linked into the jail root at boot or at install
  (confinement.go:473-491), and `blockdev --flushbufs` runs after each swap
  and before mount.
- Snapshot plumbing: a durable per-VM slot table, captured into the
  snapshot meta, replayed by `expectedLaunchConfig` instead of recomputed
  from the profile, bound by `snapshotBoundPaths`, kept by retention, and
  rebuilt on reattach. PATCH is serialized against snapshot capture.
- Desks name tools in their manifest, install review judges them, a desk
  takes a tool by lease and the lease's end unloads it, and the profile
  comes from installed packages. A Library tool's symlink farm ships in its
  own small farm layer or is written at load into the overlay's upper.
- No GPU in Firecracker: Blender renders and video encodes on CPU.

## 7. Order

Phase 1 now (station L3.5). Phase 2 when the owner wants absence, not just
eviction. Phase 3 with the first Library tool package (a video or CAD
desk). Each phase is red, lands with its own proof on d@e, and rolls back
by revert.
