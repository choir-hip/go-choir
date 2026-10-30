# The Jailer Gives Every VM Its Own Copy of the Shared Store Image

Date: 2026-10-21. Found while auditing memory density for the
[security, density and architecture checklist](../security-density-architecture-checklist-2026-10-21.md)
(item RES-05). Read from code at `origin/main` 8f71f9c64; nothing was run,
and the page-cache effect is inference from Linux semantics, not a
measurement. Status: fix built on lane/fp8-image-jailer (FP8, see "As built" below), not deployed; it acts only on jailed launches, and the jailer is off. Mutation class of any fix:
**red** (vmctl, VM lifecycle, tenant isolation).

## What is shared today

- Every guest's nix store is one read-only virtio-blk drive, an
  lz4-compressed EROFS image (`internal/vmmanager/manager.go:1731-1738`;
  `nix/autoputer-vm.nix:290-310`).
- The image is one host file per deploy:
  `VM_STORE_DISK_IMAGE=/var/lib/go-choir/guest/storedisk.erofs`
  (`nix/node-b.nix:1295`), a symlink into the `guest-image` store output
  (`nix/node-b.nix:2068-2085`; `flake.nix:523-532`).
- Firecracker reads it with buffered I/O, so all direct-launched VMs share
  one host page-cache copy of the image. Each guest still keeps its own
  decompressed page cache in guest RAM (density finding 3 and lever L10 in
  [vm-density-limits-2026-10-14.md](../reports/vm-density-limits-2026-10-14.md)).

## The problem

With `VMCTL_JAILER=on`, every read-only drive, the store image included, is
placed in each VM's jail by `placeReadOnlyCopy`: a reflink where the
filesystem allows, else a full byte copy
(`internal/vmmanager/confinement.go:345-353, 600-604, 630-664`).

- A reflink shares disk extents but is a new inode, and the Linux page cache
  is per inode. N jailed VMs can therefore hold up to N host copies of the
  hot store pages instead of one.
- If reflink fails (for example across the `/nix/store` and VM-state
  filesystems), each launch makes a full copy of the about 2.9 GB image, and
  the code only logs it (`confinement.go:656-658`).
- No document records this cost. The security review assumes the image is
  shared "as a read-only hard link" (`docs/archive/reports/computer-isolation-security-review-2026-10-14.md:923-926`);
  the code copies.

## Why it matters now

The jailer rollout is authorized (owner notes §27 item 3) and is item 4 of
the ACTIVE landing order. Turning it on as built trades one security layer
for a density regression that grows with the number of running computers.

## Failure modes any fix must close

1. Jailed VMs read the store through distinct inodes (host page cache not
   shared).
2. A reflink fallback silently copies gigabytes per launch (disk and launch
   latency).
3. The shared image becomes writable by any VMM uid (a hard link must stay
   root-owned and read-only).
4. A deploy swaps the image under running jailed VMs (today a refresh resets
   to the current inode, `manager.go:1692-1705`).
5. Snapshot wake under the jailer maps a different store inode than the one
   the snapshot was taken against.

## Fix directions (not designed)

A root-owned read-only hard link into the jail when the jail and the image
share a filesystem, or a read-only bind mount of the one image file into the
jail's mount namespace; then measure host RSS with several jailed VMs
against the same count unjailed.

## As built (FP8, 2026-10-21, lane/fp8-image-jailer)

`internal/vmmanager/jail_shared.go`: vmctl keeps one copy of each read-only
artifact (kernel, initrd, store image) per source identity (path, device,
inode, size, mtime) under `<jail base>/.shared/`, root-owned 0444, built once
by reflink (else byte copy) and fsynced, and hard-links that inode into each
jail. Every jailed VM reads the store through one inode. A copy no jail links
is swept at the next launch; a file there that is not vmctl's 0444 regular
file is rebuilt, never linked. If the shared copy cannot be built or linked,
the launch falls back to today's private copy and logs why. The receipt's
`jail_files` says `store.img=shared` (or `shared(reflink)` when this launch
built it).

Failure modes: (1) closed: one inode. (2) one copy per deploy, not per
launch. (3) closed: root-owned 0444, the VMM uid cannot write or chmod it; it
can only unlink its own name. (4) closed: a new source identity is a new
copy; running jails keep the old inode until they relaunch. (5) closed: the
copy is keyed by the same identity `snapshotBoundPaths` binds, and a restore
whose source changed is refused before the jail is built.

Test: `TestPrepareJailSharesReadOnlyArtifacts`. Proof (FP0): the station
file's "FP0 proof steps", RES-05.
