# Firecracker 1.16.1 restores virtio-mem snapshots wrong (2026-10-22)

Status: problem record, before code. Mutation class of the fix: **red**
(VM lifecycle, the host's VMM binary and jailer). Related:
`guest-data-image-never-trims-2026-10-20.md` (the 1.17 plan),
`firecracker-sigpipe-deadlock-after-vmctl-restart-2026-10-10.md`,
`host-kernel-216-days-unpatched-2026-10-15.md`.

## Problem

Node B runs `pkgs.firecracker` from the locked nixpkgs, **Firecracker
1.16.1**, for every computer and the jailer (`nix/node-b.nix`
`VM_FIRECRACKER_BIN`, `VM_JAILER_BIN`). Computers hibernate to Firecracker
snapshots, and test computers boot with a virtio-mem hotplug device
(`VM_MEMORY_HOTPLUG_ONLY`; it cannot plug on this CPU, see the NET-08
section). Firecracker 1.16.2
(https://github.com/firecracker-microvm/firecracker/releases/tag/v1.16.2)
fixes two virtio-mem bugs on exactly that path:

1. **Unplugged memory left writable after restore (#6174).** On a microVM
   restored from a snapshot memory file, memory the guest had unplugged
   stayed writable (and possibly mapped). The guest could write memory it
   had given back. 1.16.2 re-protects it and aborts if the re-map fails.
2. **Block accounting out of sync with the KVM memory slots (#6176).** A
   plug or unplug request that failed partway left the device's block
   bitmap disagreeing with the KVM memory slots; a snapshot taken after
   that carries the disagreement into the restore.

The same release also fixes the logger deadlock on a signal handler that
logs (#6086, #6143; the `SIGPIPE` handler now only counts a metric, the
class of the 2026-10-10 SIGPIPE record) and sets guest CPUID before the
CPUID-dependent MSRs so Linux 6.18 hosts keep eIBRS exposed to guests
(#6120; Node B runs the 6.18 series).

## The snapshot format changes

1.16.2 bumps the snapshot format version (#6100): snapshots from 1.16.0 or
1.16.1 do not load on 1.16.2, and the reverse. Every hibernated computer's
snapshot becomes unloadable at the deploy. vmctl already refuses a snapshot
whose recorded `firecracker --version` banner differs from the binary it
would launch (`internal/vmmanager/snapshot.go` `validateSnapshot`, exact
string compare, reason `firecracker_version_mismatch`), discards it and
cold boots, so Firecracker is never asked to load a 1.16.1 snapshot. The
host kernel bump to 6.18.57 refuses every snapshot the same way
(`host_kernel_mismatch`), so the two ship in one deploy and share one cold
boot per computer.

## Why 1.16.2 and not 1.17 (NET-08)

1.16.2 is the nearer upgrade: a patch release on the line Node B already
runs. 1.17 is NET-08's staged per-computer move for data-drive discard
(`nix/firecracker-next.nix`), one test computer first. The flake asserts
the host stays below 1.17 (`choirFirecrackerBelow`).

**Corrected 2026-10-22** ([memory-hotplug-cannot-work-on-node-b-39-bit-cpu-2026-10-22.md](memory-hotplug-cannot-work-on-node-b-39-bit-cpu-2026-10-22.md)).
This section first said 1.17 moves the virtio-mem hotplug region to
512 GiB, past the E-2236's 39-bit limit (#6255). That was wrong. The
region is at 512 GiB in v1.15.1, v1.16.1, v1.16.2 and v1.17.0 alike
(`layout.rs` `FIRST_ADDR_PAST_64BITS_MMIO`, the `past_mmio64_memory`
allocator). So 1.17 changes nothing for hotplug here, and hotplug cannot
plug on Node B under 1.16.x either. #6255's reporter just happened to run
1.17.0. The NET-08 precondition "not a hotplug computer" no longer rests
on 1.17. It stays as a way to keep one variable per test.

## Fix

Pin Firecracker 1.16.2 for both the binary and the jailer (one package),
land it with the 6.18.57 kernel bump, and verify after the deploy (the
deploy note, below, added with the fix).

As built: `flake.nix` `choirFirecrackerRelease` / `choirFirecrackerModule`
(an overlay on the Node B host only, the Dolt pattern), so
`pkgs.firecracker` is 1.16.2 for `VM_FIRECRACKER_BIN`, `VM_JAILER_BIN` and
the host's `firecracker` package. Both hashes are `lib.fakeHash`: FP0
fills them on the Mac before landing. `nix/firecracker-next.nix` (NET-08)
overrides the same package and is unaffected. Test:
`internal/vmmanager/snapshot_test.go`
`TestResumeRefusesSnapshotFromOtherFirecrackerPatch` (a 1.16.1 snapshot on
a 1.16.2 host is refused before Firecracker starts, discarded, and the
computer cold boots with `firecracker_version_mismatch` on its timeline).

## Deploy note

- Lands in the same deploy as the host kernel 6.18.57 bump, after 19:00
  UTC. Both void every snapshot; one deploy means one cold boot per
  computer instead of two.
- Every computer cold boots at its next refresh or wake: its snapshot is
  refused (`firecracker_version_mismatch`, and `host_kernel_mismatch` after
  the reboot) and discarded, never loaded. The binary is chosen at launch,
  so a computer still running is on 1.16.1 until its refresh (or the
  kernel reboot) relaunches it.
- The paper (`critical_protected`) takes a planned, marker-backed refresh
  while busy and resumes its work after the cold boot; any snapshot it
  holds is refused like every other.
- Rollback: revert the flake commit and redeploy. Snapshots taken on
  1.16.2 are refused on 1.16.1 the same way (cold boots, no load).

## FP0 verifies after the deploy

1. On Node B: `$(systemctl show go-choir-vmctl -p Environment | tr ' ' '\n'
   | sed -n 's/^VM_FIRECRACKER_BIN=//p') --version` prints
   `Firecracker v1.16.2`; the same for `VM_JAILER_BIN` (`jailer --version`).
2. A test computer: hibernate by snapshot, then wake. Expect boot kind
   `snapshot_resume`, and the snapshot meta's `firecracker_version` (and the
   timeline's) `Firecracker v1.16.2`.
3. **Removed (2026-10-22): no hotplug step.** A plug cannot succeed on
   Node B under any Firecracker version
   ([memory-hotplug-cannot-work-on-node-b-39-bit-cpu-2026-10-22.md](memory-hotplug-cannot-work-on-node-b-39-bit-cpu-2026-10-22.md)),
   so a plug, hibernate and wake check would fail for that reason, not
   because of 1.16.2. Do not roll back the KF deploy on a hotplug failure.
   The #6174 and #6176 paths stay unproven on Node B; they need a host
   that can plug.
4. No 1.16.1 snapshot loaded: every wake after the deploy whose snapshot
   predates it shows `fallback_cold` with reason
   `firecracker_version_mismatch` or `host_kernel_mismatch` on its boot
   timeline, and no `snapshot_resume` timeline anywhere records
   `Firecracker v1.16.1` after the deploy time.
