# Guest tools that load and unload live (2026-11-09)

Status: design, for review (tier 2: it designs a red change to vmctl, the
guest boot path and Firecracker drive handling). Station L3.5 of
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

## 3. The design

### 3.1 Store paths that point outside the overlay

The base layer carries, for every store path of every tool layer, a
**symlink** `/nix/store/<hash>-<name>` → `/nix/.tool/<layer>/store/<hash>-<name>`
instead of the path itself. The layer builder writes this farm at image
build time from the same closure lists it already computes
(nix/guest-tools.nix). The tool layers stop being store overlay lowers.

Each slot's erofs mounts at `/nix/.tool/<layer>` (read-only, nodev,
nosuid). Unloaded, the symlinks dangle: exec fails with ENOENT, which the
exec wrapper (nix/guest-tool-exec-wrapper.sh) already turns into a clear
message, and which it now turns into a load request (3.3). Loaded, every
path resolves; ELF interpreters and RPATHs follow symlinks, so the binaries
run unchanged.

Rejected: bind-mounting each store path onto a stub directory (hundreds of
mounts per tool); a per-tool overlay (still a lower of nothing the store
sees); fetching into the data volume (per-computer bytes, no shared page
cache).

### 3.2 Who decides: vmctl, host side

The guest can only **ask**. vmctl owns the slot table per VM:
`slot → {layer, file, digest, state: empty | loaded}`.

- **Load** (guest asks for layer L): vmctl checks L is in this release's
  `layers.json` and in the computer's tool profile, resolves the layer file
  in the jail's `.shared` (pre-linked at boot for every layer of the
  release, so a load creates no file), verifies its digest once per
  release, and PATCHes the slot from the placeholder to the layer. It
  answers `loaded` with the slot's device; the guest mounts it.
- **Unload** (guest asks, or a lease ends, or idle): the guest stops the
  tool's processes (only those whose executable or mapped files are under
  `/nix/.tool/L`; `fuser -m`), unmounts, flushes the block device's
  buffers, and reports `unmounted`; vmctl then PATCHes the placeholder back.
  vmctl never swaps a slot the guest still has mounted: a swap under a live
  mount would hand the kernel different bytes for cached blocks. If the
  guest does not report, the slot stays loaded (safe: memory, not
  correctness).
- **Refusals**, each typed and visible: a layer outside the profile; a
  layer not in the release; a digest mismatch; no free slot.

The channel is the guest's existing authenticated host route (the same
bearer and pin the gateway checks), a new endpoint on vmctl's guest-facing
API. A load request names a layer, never a path.

### 3.3 Triggers

- The exec wrapper, on a missing tool: request the load, wait (bounded,
  2 s), then exec. A tool call never hangs: on timeout it exits 127 with
  the reason.
- Callers that run a tool by absolute path (the capsule broker for Claude
  Code, `CHOIR_BROWSER` for Playwright) call a guest helper
  `choir-tool load <layer>` first.
- Unload: after the tool's last process exits plus an idle grace (10
  minutes, measured, not a switch), and at once when memory pressure
  reaches the balloon's reclaim threshold.

### 3.4 Hibernate, wake, restarts

- A snapshot records the slot table. vmctl restores the drive set as it
  stood (loaded layers behind their slots), so a woken computer finds its
  mounts intact. `snapshotBoundPaths` already includes tool disks.
- A cold boot starts with every slot empty: the loaded set is memory, not
  durable state (restart rules: a crash closes work, it does not resume
  it).
- A deploy that moves a layer's digest moves its symlink farm too; the
  computer gets the new layer at its next cold boot, as today.

### 3.5 Security

- The guest names a layer; vmctl decides from the release and the
  computer's profile. No guest input becomes a host path.
- Layers are read-only erofs from the signed release, digest-checked;
  mounted nodev, nosuid; the guest cannot write or substitute one.
- Spend authority is unchanged: the Claude delegate's grant is the gateway
  relay's list. Loading Claude Code grants nothing.
- Unload kills only processes executing from the tool's mount.
- Rate: one load or unload in flight per slot; repeated refusals are logged
  once per minute.

## 4. Desks and Library packages

The same mechanism generalizes, in the next station (not this build):

- **A tool is a package.** Host-built from a Nix derivation, published as a
  signed, content-addressed erofs layer with its store path list. Stored
  once per host however many computers adopt it.
- **A desk names its tools** in its manifest (`tools: [blender]`).
  Installing the desk passes them through install review; a tool from
  outside first-party gets the stricter tier, and runs only inside the
  non-first-party desk sandbox.
- **Leases.** A desk takes a tool by lease (the custodian lease pattern
  Mail uses); the lease's end unloads it. Grants flow through leases, as
  doctrine says.
- **Generic slots.** A computer has a fixed number of slots, assigned at
  load. The symlink farm for a Library tool cannot live in the base (the
  base is built per platform release); it lives in the tool's own small
  "farm" layer or is created at load in a private store directory the
  overlay already has as its upper. Open question for that station.
- **The profile comes from installed packages**, replacing the node-b.nix
  list.
- **No GPU** in Firecracker: Blender renders and video encodes on CPU.
  Modelling, CAD, editing and modest renders fit; heavy rendering would
  need a host render service later.

## 5. Proof on d@e

1. Cold boot: no tool mount, `free` and `/proc/meminfo` recorded.
2. `choir-tool load browser`: loaded under 2 s; Playwright runs a page.
3. Unload: no process left under the mount; guest page cache drops back to
   within 10 MB of step 1.
4. Claude Code: a delegate round loads, runs and unloads it.
5. A load of a layer outside the profile is refused with its reason.
6. Hibernate with the browser loaded, wake, run a page.
7. A tool exec with the layer unloaded loads it on demand and runs.

## 6. Build order (red; Ring-Expedite under the current tempo)

1. Nix: symlink farm in the base; tool layers mounted at `/nix/.tool/*`,
   out of the store overlay. Every slot still boots loaded (no behaviour
   change yet). Proved: d@e runs every tool through the farm.
2. vmctl: slot table, load and unload endpoint, PATCH, snapshot record.
3. Guest: `choir-tool`, wrapper trigger, idle unload.
4. Boot every slot empty; the proof above.

Rollback: revert; a cold boot returns to slots loaded at boot.

## 7. Risks to probe first

- The guest block layer after a backing-file swap: confirm the virtio-blk
  config change invalidates the device's buffer cache (step 2 flushes
  explicitly; the probe confirms no stale block survives).
- Nix tooling inside the guest that refuses a symlinked store path (the
  capsule's own `nix` use, if any).
- Firecracker's PATCH on a jailed drive path (the path must exist inside
  the jail; pre-linking covers it).
