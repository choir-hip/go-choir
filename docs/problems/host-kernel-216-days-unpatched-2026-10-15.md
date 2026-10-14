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
