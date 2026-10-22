# Every Guest Keeps Its Own Copy of the Shared Base OS in RAM

Date: 2026-10-22. Found while opening SDC item RES-08 (lever L10 in
[vm-density-limits-2026-10-14.md](../reports/vm-density-limits-2026-10-14.md)).
Read from code at `d1c60ad87`; nothing was run. Status: documented; the
size of the problem is **unmeasured**, and the fix is behind a security
panel (DEC-05). Mutation class of any fix: **red** (VM devices, guest
boot, tenant isolation).

## What is true today (code)

- The base OS (the guest's nix store closure) is one lz4-compressed EROFS
  image, attached to every guest as a read-only virtio-blk drive
  (`internal/vmmanager/manager.go:1733-1739`; `nix/autoputer-vm.nix:290-302`;
  host path `VM_STORE_DISK_IMAGE` in `nix/node-b.nix:1295`).
- Firecracker reads the image with buffered I/O, so the host page cache
  holds at most one copy of the hot blocks for all direct-launched VMs
  (the jailer breaks even that: [jailer-copies-store-image-per-vm-2026-10-21.md](jailer-copies-store-image-per-vm-2026-10-21.md)).
- Inside each guest, every block read is copied into guest RAM and
  decompressed into the guest's own page cache. Executables and libraries
  run from that private copy. N guests running the same release therefore
  hold N private copies of the same hot base-OS pages.

## What is not known

- **How big the duplicated part is.** Idle test computers showed about
  640 MiB of guest page cache (density finding 3, procmem n=3). The density
  report's L10 estimate (0.3 to 0.45 GiB per idle guest) rests on the
  *hypothesis* that most of that cache is store reads; the rest would be
  Dolt store and other data files on `/mnt/persistent`, which no sharing
  can remove. Nothing in the guest reports page-cache residency by
  filesystem today (`/internal/debug/pprof/procmem` reads `meminfo` and
  per-process RSS only), so the hypothesis cannot be checked without a
  shell in the guest.
- **How much of the host page cache the image holds** with N guests
  running, and how that changes once the jailer is on.

## Why it matters

At density's "medium" tier the idle guest is about 0.5 GiB; if the
hypothesis holds, the base-OS copy is most of it, and L10 is the largest
remaining idle lever. If it does not hold, L10 is not worth a new
cross-tenant side channel, and guest page-cache reclaim (L5) is the lever.
Either way the measurement decides, and it has to come before the panel.

## Failure modes any fix must close

1. A shared mapping of base-OS pages across tenants opens a cache-line
   timing channel (Flush+Reload) that today's private copies do not have.
   Firecracker's pmem doc: "It is not recommended to use the same backing
   file for virtio-pmem across different VMs ... which could be exploited
   as a side channel by an attacker inside the microVM."
2. A guest write to a read-only pmem device shuts the VM down (pmem doc);
   any guest path that writes the store must be gone first.
3. The image file becomes writable by a VMM uid, or a deploy swaps it under
   running guests or snapshots (pmem restore reopens the same path).
4. DAX bypasses the guest block layer, so any block-level integrity check
   added later (dm-verity) cannot cover the DAX path.
5. The guest's `struct page` array for the pmem region (about 1.6% of the
   image size per guest) eats into the saving.
6. Shared page-cache pages are charged to whichever VM cgroup faulted them
   first once per-VM cgroups land (L17), which skews per-VM accounting.

## Next

Design, threat analysis and FP0's read-only measurements:
[res-08-shared-base-os-pmem-dax-design-2026-10-22.md](../reports/res-08-shared-base-os-pmem-dax-design-2026-10-22.md).
Station: [choir-launch-fp10-res-08-2026-10-21.md](../definitions/choir-launch-fp10-res-08-2026-10-21.md).
