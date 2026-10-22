# The kernel pin is behind a guest-reachable host fix (6.18.55; the virtio-net checksum fix is 6.18.57)

Date: 2026-10-22. Found by: FP0's kernel research (a Sonnet research agent asked by the owner whether to move to kernel 7; the full report, with citations, is in FP0's run folder `_runs/kernel7/report.md`). Mutation class of the fix: red (the host and guest kernel; the host change takes effect only at a host reboot).

## Problem

Host and guests run Linux 6.18.55, the `nixpkgs-kernel` lock from 2026-10-03 (`flake.lock` rev 7fd4004d…). 6.18.56 (2026-10-11, 403 commits), 6.18.57 (2026-10-15, 622) and 6.18.58 (2026-10-21, 49) shipped since. 6.18.57 carries "ipv4: validate checksum_start before completing checksum" (commit 6785011f8b16): guest-supplied virtio-net checksum metadata (`csum_start`, `csum_offset`) was not checked against the IPv4 header, so a deferred checksum write could rewrite the header-length byte and leak or corrupt host kernel memory. The fix is in the official stable changelogs (ChangeLog-6.18.57, ChangeLog-7.2.11). Its public write-up ("PwnEscape") reports a repeatable host memory disclosure; the same researcher's "KVM 0day" was confirmed by a Firecracker-based sandbox vendor in early October. That they are the same bug is not confirmed.

## Why Choir is exposed (reasoned, not drilled)

Firecracker offers the guest `VIRTIO_NET_F_CSUM` and opens each tap with `IFF_VNET_HDR`, so a guest kernel controls that metadata on the host's tap device. Exploiting it needs control of the guest kernel's virtio driver (guest root), not a desk cell or a capsule: desk cells and capsules have no raw sockets. Choir's guests run model-driven code with root daemons, so guest root is the boundary that matters.

## Fix

1. Update `nixpkgs-kernel` alone (`nix flake update nixpkgs-kernel`, the kernel half of the patch cadence named in `flake.nix`) to a nixos-unstable revision whose `linuxPackages_6_18` is at least 6.18.57.
2. Raise the flake's security floor assertion from 6.18.45 to 6.18.57, naming this fix beside the older ones.
3. Deploy: the guest kernel reaches each computer at its next refresh (a cold boot; snapshots taken under 6.18.55 are refused, as for any kernel change). The host kernel takes effect only at a host reboot. That is the owner's timing; it also clears the pending reboot (booted system 26.11.20260701, current 26.11.20261015).

## What stays

Stay on the 6.18 LTS line (supported to about December 2028; Firecracker tests 6.18 as host and guest, and no 7.x). Revisit at the next 7.x LTS once Firecracker lists it. Cadence from the research: weekly `nixpkgs-kernel` bump for the guest, a monthly planned host reboot, an emergency host reboot within 72 hours for a guest-reachable host fix (this one), and an alarm when the booted kernel differs from the activated one.

## Related

- The Node B CPU has 39-bit physical addresses (`grep 'address sizes' /proc/cpuinfo`, read 2026-10-22). Firecracker places the virtio-mem hotplug region at 512 GiB, which a 39-bit CPU cannot address (firecracker issue #6255, open). **Corrected 2026-10-22:** this line first blamed 1.17. The placement is the same in 1.15.1, 1.16.1, 1.16.2 and 1.17.0, so hotplug cannot plug on Node B today either ([memory-hotplug-cannot-work-on-node-b-39-bit-cpu-2026-10-22.md](memory-hotplug-cannot-work-on-node-b-39-bit-cpu-2026-10-22.md)). Firecracker 1.16.2 fixes two virtio-mem snapshot-restore bugs and changes the snapshot format; it is the nearer upgrade.
- docs/problems/host-kernel-216-days-unpatched-2026-10-15.md (the floor's origin).
