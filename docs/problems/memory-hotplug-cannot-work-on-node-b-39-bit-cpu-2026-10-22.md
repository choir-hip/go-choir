# Memory hotplug cannot work on Node B under any Firecracker we pin (39-bit CPU, region at 512 GiB) (2026-10-22)

Status: problem record, before any fix. **Reasoned from source and the
upstream issue, not drilled on Node B.** Found by: Sonnet's review of the KF
deploy (F1), checked by the kf-r2 lane against the Firecracker source.
Mutation class of any fix: **red** (VM lifecycle, the host VMM's guest
memory layout). Corrects an earlier premise in
[firecracker-1-16-1-virtio-mem-restore-bugs-2026-10-22.md](firecracker-1-17-for-every-computer-2026-10-22.md),
[kernel-pin-behind-guest-reachable-checksum-fix-2026-10-22.md](kernel-pin-behind-guest-reachable-checksum-fix-2026-10-22.md)
and the FP8 definition's NET-08 note, which said Firecracker **1.17** moves
the hotplug region to 512 GiB.

## Problem

Firecracker places the virtio-mem hotplug region at **512 GiB guest
physical in every version Choir has run or planned**, not only in 1.17.
Node B's Xeon E-2236 has 39-bit physical addresses, so the guest's
addressable range ends at 512 GiB - 1 and the region starts one byte past
it. Memory hotplug is turned on for test computers on Node B today, and,
by this reading, no plug can ever succeed there.

## Evidence

**Firecracker source** (shallow clones of the release tags, read-only,
2026-10-22). Identical in v1.15.1, v1.16.1, v1.16.2 and v1.17.0:

- `src/vmm/src/arch/x86_64/layout.rs:124-134`: `MMIO64_MEM_START = 256 << 30`,
  `MMIO64_MEM_SIZE = 256 << 30`,
  `FIRST_ADDR_PAST_64BITS_MMIO = MMIO64_MEM_START + MMIO64_MEM_SIZE`
  (512 GiB = 2^39).
- `src/vmm/src/vstate/resources.rs:84-85` (`:83-84` in 1.17.0): the
  `past_mmio64_memory` allocator starts at `FIRST_ADDR_PAST_64BITS_MMIO`.
- `src/vmm/src/builder.rs` `allocate_virtio_mem_address` (1.16.2 `:629`):
  the virtio-mem region is allocated `FirstMatch` from
  `past_mmio64_memory`, so it starts at exactly 512 GiB.

So 1.17 changes nothing here. The earlier records' "1.17 places the region
at 512 GiB" came from issue #6255's reporter, who happened to run 1.17.0.

**The guest cannot address it.** Choir sets no CPU template
(`internal/vmmanager` has no `cpu-config` or template field), so on Intel
Firecracker passes the host's CPUID leaf 0x80000008 through: the guest sees
39 physical address bits. Linux on x86 caps `iomem_resource` at
2^phys_bits - 1, so virtio-mem's `request_mem_region` at 512 GiB fails
and the driver probe fails with -EBUSY. That is the guest log in #6255:
`could not reserve device region`, `probe with driver virtio_mem failed
with error -16`. Independently, KVM's EPT cannot map a guest physical
address past the host's MAXPHYADDR.

**Upstream, firecracker-microvm/firecracker#6255** ("Failure when setting
up memory hotplugging"; read 2026-10-22: open, "Awaiting author", assigned
to ilstam). On a 39-bit CPU, configuring the device and reading its status
succeed. The first `PATCH /hotplug/memory` fails with "Failed to create
memory hotplug device: Device is not active". The maintainer's proposed
diffs move `MMIO64_MEM_START` to `128 << 30` or shrink `MMIO64_MEM_SIZE`.
They note that server CPUs have more than 39 bits, so mostly older or
consumer hardware is affected. Nothing is merged.

**Choir uses hotplug on Node B.** `nix/node-b.nix:1465-1466` sets
`VM_MEMORY_HOTPLUG=on` and `VM_MEMORY_HOTPLUG_ONLY=class:test`. Test
computers boot with an unplugged 4 GiB region (vm-4a9afa23 and
vm-c5c801f8 in
[daemon-memory-res18-2026-10-22.md](daemon-memory-res18-2026-10-22.md),
"J14"). **No plug has ever been recorded succeeding on Node B.** The
2026-10-15 probe was refused before plugging
([overnight-landing-2026-10-15.md:136](../archive/reports/overnight-landing-2026-10-15.md)),
and FP8's RES-09 probe did not run on 2026-10-22 (host memory admission;
metamission ledger 03:16). FP8 P5 diagnoses the refusal as a selection
ordering bug. Once that is fixed, the probe would reach the plug and fail
for this reason instead.

## What it does today

- Boot: Firecracker accepts the device and the computer boots. The guest
  logs one virtio_mem probe failure. Nothing else changes.
- A lease grow (a capsule job that declares `memory_mib` on a test
  computer): `SetMemoryHotplugRequestedMiB` fails. vmctl rolls the
  reservation back and returns the error
  (`internal/vmctl/memory_hotplug.go:469-482`). It fails closed. The job
  runs at the base size or gets the refusal; which of the two is unread.
- Hibernate: `unplugBeforeSnapshot` (`internal/vmmanager/memory_hotplug.go:245`)
  does nothing when nothing is plugged. Snapshots are unaffected.
- The 1.16.2 fixes for virtio-mem restore (#6174, #6176) cannot apply on
  Node B in practice: nothing is ever plugged. 1.16.2 still carries the
  logger-deadlock and eIBRS fixes, and the KF deploy stands on those.

## How FP0 confirms it (read-only)

1. `grep -m1 'address sizes' /proc/cpuinfo` on Node B: `39 bits physical`
   (read 2026-10-22).
2. A running hotplug test computer's console log (`vm-state/<vm>/` console
   file): grep `virtio_mem` for `could not reserve device region`.
3. Optional, it mints its own test computer:
   `scripts/memory_hotplug_probe.py`. Expected result: the grow fails with
   `Device is not active`, or the guest's plugged size stays 0. That is
   the confirmation, not a regression of the KF deploy.

## Fix options (not chosen; this record stops at the problem)

1. Turn hotplug off on Node B (delete the two lines; the rollback the
   node-b.nix comment already names). This is the conservative default
   until one of the options below lands. It loses nothing that works today.
2. Patch the layout in our overlay, as in the maintainer's diff
   (`MMIO64_MEM_START = 128 << 30`). This changes the guest memory map
   under the same `Firecracker v1.16.2` banner, and vmctl's snapshot check
   compares only the banner (`validateSnapshot`). A patched build would
   then load unpatched snapshots. It needs a distinct version string or a
   layout field in the snapshot meta first.
3. Wait for #6255's upstream fix, then take it on the reviewed path
   (the flake's `choirFirecrackerBelow` bound, NET-08).
4. A host CPU with 40 or more physical address bits.

## Belief and remaining error

- Belief: hotplug has never worked on Node B and cannot under 1.15.1 to
  1.17.0. The KF deploy (1.16.2) neither causes nor fixes this.
- Remaining error: the guest-side failure is reasoned from the kernel
  path and #6255's log, not read from a Choir console. What a capsule job
  does after a refused grow is unread.

Deleted 2026-10-26 (`6013d77d3` on lane/delete-dead): the controller, routes, device config and guest lease hook were removed; see docs/evidence/switched-off-inventory-2026-10-26.md.
