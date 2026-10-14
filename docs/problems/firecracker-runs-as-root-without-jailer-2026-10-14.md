# Firecracker Runs as Root Without the Jailer

Date: 2026-10-14. Found by the isolation-runtimes research
(docs/research/isolation-runtimes-2026-10-14.md, §5) from the vmctl unit
and code; confirmed read-only on Node B at about 16:00 UTC. Status:
documented; no fix yet. Mutation class of any fix: red (vmctl, VM
lifecycle, tenant isolation).

## Evidence (Node B, live)

For each running Firecracker process (pids 1288901, 1299614, 1299762):
- uid 0 (root); no `jailer` process anywhere;
- effective capabilities 0x81000 = CAP_NET_ADMIN + CAP_SYS_PTRACE
  (inherited from the vmctl unit's bounding set);
- seccomp mode 2 (Firecracker's built-in seccomp filters are active);
- root directory `/` (no chroot); one mount namespace shared by the
  VMMs started by the same vmctl (the unit's own namespace).
- `go-choir-vmctl`: `User=` empty (root), CapabilityBoundingSet
  `cap_net_admin cap_sys_ptrace`, NoNewPrivileges yes.

## Why it matters

The jailer is Firecracker's documented production setup: each VMM runs as
an unprivileged per-VM uid/gid, chrooted to its own directory with only
its own kernel, rootfs, data image and sockets, in its own namespaces and
cgroup. Today a guest that escaped into its Firecracker process (a VMM
bug plus a seccomp bypass) would run as root with CAP_NET_ADMIN and
CAP_SYS_PTRACE and a view of every computer's data image and snapshot,
so one escape compromises every tenant and the host's network.
Firecracker's seccomp filters reduce but do not remove this.

## Fix directions (to design, red class)

- Run each VMM under the jailer (or an equivalent per-VM wrapper):
  per-VM uid/gid, chroot with hard-linked or bind-mounted per-VM files,
  new mount/pid/net namespaces, per-VM cgroup (pairs with density lever
  L17, per-VM cgroups); vmctl keeps the privileged steps (tap creation,
  iptables) and hands the VMM only its own fds/paths.
- Drop CAP_SYS_PTRACE from what the VMM inherits; check why vmctl needs it.
- Verify snapshot/restore, balloon and the planned virtio-mem/pmem paths
  work under the jailer's chroot.
- Pair with the tap isolation fix
  (docs/problems/tap-deny-rule-covers-one-of-sixteen-guest-subnets-2026-10-14.md)
  and the red-team station LT's first experiment (assumed VMM breach).
