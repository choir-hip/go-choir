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
- **The base is a 26.11 pre-release snapshot.** Main nixpkgs (`ec84054`)
  is nixos-unstable frozen on 2026-07-01, which reports NixOS 26.11. A
  kernel from nixos-26.05 crosses lineages. It fails under the base's
  NixOS modules (`attribute 'buildDTBs' missing`, then `target`) and only
  works with an attribute shim. A kernel from current nixos-unstable
  stays in the base's lineage and evaluates as-is.

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

## What the fix needs: status (2026-10-15, branch `ops/kernel-6.18-security`, not pushed)

1. **A kernel at or above 6.18.45 for host and guest: prepared.**
   - A new flake input, `nixpkgs-kernel` (nixos-unstable @ `7fd4004`,
     2026-10-11), is used only for `boot.kernelPackages =
     linuxPackages_6_18` on `go-choir-b` and `go-choir-autoputer-vm`.
     Both evaluate to **6.18.55**. Main nixpkgs is untouched.
   - Lineage: the base is a NixOS 26.11 pre-release snapshot
     (nixos-unstable, 2026-07-01). The kernel input is the same branch,
     newer, so it needs no shim. nixos-26.05's 6.18.56 is one point
     release newer, but it crosses lineages and needed an attribute shim,
     so it was dropped (owner, 2026-10-15). The full base bump (main
     nixpkgs) belongs to the patch pipeline's weekly lane, not this
     branch.
   - The kernel, `-dev` (the guest vmlinux) and config are in
     cache.nixos.org. `nix build --dry-run` builds only the module closure
     and the initrds.
   - An assertion holds the floor at 6.18.45. That covers Copy Fail
     (6.18.22), CVE-2026-53266 (6.18.36), CVE-2026-53362 and Januscape
     (6.18.38), and Vghost (6.18.45).
   - The guest kernel config differs from 6.18.37 in 21 lines: gcc 15.2
     to 16.2, rustc and pahole versions, TCP_AO=y, MODULE_DECOMPRESS=y
     (modules stay xz), CXL_RAS, ACPI_PLATFORM_PROFILE=y. Every option the
     guest kernel capability probe and Firecracker boot need is
     unchanged: overlay=m, Landlock in CONFIG_LSM, namespaces, the seccomp
     filter, virtio blk/net/mmio/balloon, PAGE_REPORTING, erofs (with its
     compressors) and ext4.
   - Not built locally: this Mac has only an aarch64-linux builder. The
     6.18.55 bzImage was substituted and checked (`file`: 6.18.55). Node B
     builds the rest at deploy, as usual.
   - The guest half lands at deploy, through the deploy refresh. Busy
     computers stay on 6.18.37 until the next deploy or the reboot.
2. **`kvm_intel nested=0`: prepared** (`boot.extraModprobeConfig` in
   `nix/node-b.nix`). A grep found no Choir code that needs nested
   virtualization: Firecracker runs only on the host, and no guest
   module, service or test uses `/dev/kvm` or VMX.
   **SMT: owner decision; not changed.** `nosmt=force` is in
   `nix/node-b.nix` as a comment. The cost:
   - 6 cores / 12 threads become 6 threads. Today 3 always-on 4-vCPU
     guests exactly fill 12 threads; with SMT off that is 2:1 vCPU
     overcommit, and host builds (deploys) get half the threads.
   - Real throughput loss is smaller than half. SMT usually adds about
     15 to 30% throughput on mixed loads (estimate), so expect roughly a
     20% loss for busy computers. By the CPU report's figure of about 1.2
     cores per busy computer, that is about 5 busy computers instead of
     6 to 7 (estimate).
   - RAM binds first on this 32 GB host either way.
   - The other route is core scheduling, one cookie per tenant. It keeps
     the threads and costs development work.
   - Recommendation: decide before strangers' computers run on Node B.
     Today the tenants are the owner and the platform.
3. **A host reboot: prepared, not done.**
   - Runbook: `docs/runbooks/node-b-host-reboot.md`. Script:
     `scripts/node_b_host_reboot.py`. It is dry-run by default; `--execute`
     also needs `--console-verified`.
   - The script's dry run on Node B correctly refuses today: the staged
     kernel is 6.18.37, nested=0 is not staged yet, and the guest image is
     6.18.37.
   - Marker policy: markers go only to idle computers, immediately before
     the reboot, and the post-checks report each one's age and actual
     boot kind. Busy computers are waited for, or the run aborts.
     `--allow-crash-close` and `--no-markers` are the explicit
     crash-semantics options.
   - Main risk: the console. If ssh doesn't return, recovery needs the
     OVH IPMI/KVM console, which nobody has verified. No GRUB entry is
     known good, and this is the go-choir configuration's first cold
     boot.
4. **Firecracker 1.17.0: not in this branch.** No nixpkgs has it yet:
   main and nixos-26.05 have 1.15.1, and nixos-unstable has 1.16.1.
   - From the kernel input: no. It would only give 1.16.1 (unstable).
   - The easy route is an overlay on main's `firecracker` with the
     1.17.0 `src` and `cargoHash`. The alternative is the upstream static
     release binary from `fetchurl`.
   - Cost: a Rust build on Node B, and one cold boot for every
     hibernated computer, because the Firecracker version is part of
     snapshot validity.
   - Ship it with or right after the reboot window. Its effect doesn't
     need a reboot; it needs VM restarts, which the deploy refresh
     provides.

## Belief and remaining error

- Belief: the host kernel is the weakest dated link under every computer
  boundary, and it ranks above the jailer and confinement work.
- Remaining error:
  - resolved: the main nixpkgs pin has only 6.18.37. A kernel-only input
    gives 6.18.55 (status item 1);
  - whether the host comes back at all. This is the go-choir
    configuration's first cold boot, with a new systemd stage 1 on md
    RAID, and no GRUB entry is known good. The boot path last ran in
    March, under the choiros-rs system;
  - whether the OVH IPMI/KVM console works for this server (the main
    reboot risk);
  - how long POST and boot take. That decides whether planned-restart
    markers are still fresh (15-minute TTL);
  - the cost of nosmt for density: estimated in status item 2, not
    measured.

## 2026-10-22: the lag recurs and nobody sees it unless they ask

Reported by FP0 on 2026-10-22: Node B again runs a booted system older than
the activated one. The detector exists three times over
(`internal/hostboot`, vmctl's internal `GET /internal/vmctl/host/boot-state`,
`scripts/verify-host-hardening.py --boot-state-json`), and the staging deploy
prints a `Host reboot pending` warning. All of them answer only when someone
asks: nothing on the host says it on its own, at boot or while the host sits
between deploys. The 216 days happened in exactly that gap.

Fix (lane `fp0-ci-guards`, orange: vmctl runtime, read-only, log only): vmctl
writes one journal line from `hostboot.Read` at start and every 24 hours,
naming booted_system, current_system, the booted and activated kernel
versions, the reasons, and since when the reboot has been pending (a
`WARNING` line when pending, `OVERDUE` past 7 days). It stays off public
`/health`, as the internal route does: a public "kernel X, reboot pending"
names the exploits that work. Rollback: revert the commit; nothing persists.
