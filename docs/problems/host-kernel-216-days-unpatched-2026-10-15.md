# Node B's host kernel has not been rebooted in 216 days and lags known exploited fixes

Date: 2026-10-15 (~23:25 UTC on 10-14). Class: red (host kernel, every
computer's isolation). Found by
`docs/reports/boundary-strength-ladder-2026-10-15.md`; confirmed read-only on
Node B.

## Evidence (read-only, 2026-10-14 23:25 UTC)

- `uname -r` → `6.18.16`; `uptime` → up 216 days.
- `/run/booted-system/kernel` → linux-6.18.16. `/run/current-system/kernel`
  → linux-6.18.37. NixOS has installed 6.18.37, but the host has not
  rebooted into it; every deploy since then switched userspace only.
- `/sys/module/kvm_intel/parameters/nested` → `Y` (nested virtualization
  on).
- `/sys/devices/system/cpu/smt/active` → `1` (SMT on). The CPU is a Xeon
  E-2236 (Coffee Lake); its final microcode is 0xfa, and Intel support
  ended 2025-06-30.

## More evidence: the reboot itself is the risk (read-only, 2026-10-14 ~23:50 UTC)

Found while preparing the fix. All of it was read on Node B or from the
code; nothing was changed.

- **The go-choir host configuration has never cold-booted.**
  `/run/booted-system` is `nixos-system-choiros-b-26.05.20260306.aca4d95`:
  the choiros-rs configuration on NixOS 26.05, booted 2026-03-12. Every
  go-choir-b generation (NixOS 26.11.20260701) was only switched into. The
  runtime hostname is still `choiros-b`; it becomes `go-choir-b` at the
  next boot.
- **The boot path changes in more than the kernel.** The booted system
  used the scripted stage 1 (kernel params `loglevel=4 lsm=landlock,yama,bpf`).
  The current one uses systemd stage 1 (`boot.initrd.systemd.enable = true`,
  kernel param `root=fstab`). Root is on md RAID1 (`md127`), and `/boot` is
  on `md126` (metadata 1.0). Neither path has run on this host. fstab and
  mdadm.conf are the same in both.
- **No boot menu entry is known good.** `/boot/kernels` holds only
  linux-6.18.37. All 11 GRUB entries (generations 1314 to 1324, all from
  2026-10-14) boot 6.18.37 with go-choir userspace. The proven 6.18.16
  kernel and initrd survive only in `/nix/store`, and only while
  `/run/booted-system` roots them, which ends at the reboot. Booting "the
  previous generation" is therefore not a rollback to a proven boot.
- **Console access is not verified.** `/dev/ipmi0` exists. The board is an
  ASRock Rack E3C242D4U2-2T, whose BMC has KVM-over-IP. `nix/hardware.nix`
  names an OVH SYS-1. Nothing in the repo records anyone opening the OVH
  IPMI/KVM console or rescue netboot for this server.
- **One-shot boots can't be trusted here.** `grub.cfg` honors `next_entry`
  (`grub-reboot`). But GRUB can't write `grubenv` on md RAID, so the
  one-shot may never clear itself (inference, untested).
- **Hibernating before the reboot buys nothing.** A snapshot records the
  host kernel release. On a mismatch (`host_kernel_mismatch`,
  `internal/vmmanager/snapshot.go:533`), vmctl cold-boots instead, and that
  cold boot is a crash boot. All 147 hibernated computers (count at 23:50)
  cold-boot on their next wake after the reboot. Any guest-image change has
  the same effect, so the guest-kernel deploy alone already causes it.
- **Planned-restart markers only partly cover a host reboot.** The guest
  writes its own marker (`POST /internal/runtime/planned-restart`). It
  refuses while busy (409). The marker lasts 15 minutes
  (`plannedRestartMaxAge`) and is read once. So a reboot preserves a
  computer's parked work only when two things hold: the computer was idle
  when marked, and it cold-boots within 15 minutes. vmctl's sweeper wakes
  always-on and critical computers about a minute after vmctl starts.
  Other computers wake on their next request. By then their markers are
  stale, so they boot as crash boots. They were idle when marked, so no
  run was in flight.
- **What was running at 23:50:** 2 active (the owner's premium_always_on
  `candidate-fleet-e15cb…` and one primary), 1 failed, 147 hibernated,
  19 stopped. The paper computer `vm-1669020a…` (critical_protected) was
  stopped by vmctl's wedge detector at 23:36 UTC. That is a separate,
  pre-existing problem, not caused by this work.
- **The two kernels take effect at different times.** The deploy rebuilds
  the guest image, and the deploy refresh restarts idle active computers
  onto it (busy ones defer). So the guest kernel lands at deploy. The host
  kernel and `nested=0` land only at the reboot.
- **Firecracker 1.17.0 is not in any nixpkgs.** Main nixpkgs (`ec84054`)
  and the nixos-26.05 head both ship 1.15.1. The nixos-unstable head
  (`7fd4004`) ships 1.16.1. Getting 1.17.0 means an overlay override (new
  `src` and `cargoHash`, or the upstream static release binary) in its own
  change. The Firecracker version is part of snapshot validity, so the
  bump also turns every hibernated computer's next wake into a cold boot.
- **A release-branch kernel doesn't evaluate here.** `linuxPackages_6_18`
  from nixos-26.05 (6.18.56) fails under the main input's NixOS modules
  (`attribute 'buildDTBs' missing`). The kernel input has to be the same
  branch as main, at a newer revision.

## Why it matters (from the ladder report; CVE details there)

- **Below 6.18.38:** a container-escape LPE on CISA's known-exploited list
  (used in July in an agent container escape) and the KVM bug
  "Januscape". Capsules share the guest kernel, which was last seen at
  6.18.37, so they are exposed too.
- **Below 6.18.45:** Vghost (vhost), a host local-root bug.
- **An unexplained escape:** a Firecracker-on-KVM guest-to-host escape was
  announced 2026-10-03 (Vercel Sandbox, "KVM 0day"). There is no write-up
  or patch yet. If the bug is in KVM, only a host kernel update helps.
- **Nested KVM** widens the KVM attack surface, and no Choir guest needs it.
- **SMT** leaves cross-tenant side channels open (VMScape names Coffee
  Lake).

## What the fix needs (not yet done)

1. **A kernel at or above 6.18.45 in the host configuration.** Pin
   `boot.kernelPackages` or bump nixpkgs. The guest kernel (flake.nix
   guestKernel) needs the same.
2. **`kvm_intel nested=0`** (`boot.extraModprobeConfig`) and a decision on
   SMT (`nosmt`). Turning SMT off costs host threads (6 cores / 12 threads
   → 6), which matters for density.
3. **A host reboot.** The kernel and the KVM module parameters only take
   effect at boot. A reboot stops every computer, the paper included. It
   needs a maintenance window: planned-restart markers for running
   computers (CLAUDE.md, "Restarts end work or resume it") and an owner
   decision on timing. Deploys never restart a busy computer; a host
   reboot does.

## Belief and remaining error

- Belief: the host kernel is the weakest dated link under every computer
  boundary, and it ranks above the jailer and confinement work.
- Remaining error:
  - whether 6.18.45 or later is in the pinned nixpkgs (check before the
    bump);
  - whether every guest and service comes back cleanly after a reboot
    after 216 days of uptime (boot path last exercised in March);
  - the cost of nosmt for density.
