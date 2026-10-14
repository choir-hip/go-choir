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

## Design: slice 1 (J1), 2026-10-14

Mutation class **red** (vmctl, VM lifecycle, snapshot validity, host
firewall). Source plan: the security review
(docs/reports/computer-isolation-security-review-2026-10-14.md) §4.2 (the
jailer contract), §6.2 (use the Firecracker jailer now) and §7.2 J1.

### Facts checked before design (Node B, read-only, about 18:00 UTC)

- `pkgs.firecracker` 1.15.1 ships `bin/jailer` (Jailer v1.15.1) beside
  `bin/firecracker`; since 1.13 the jailer does not check the exec-file name.
- Jailer v1.15.1 source (`src/jailer/src/env.rs`, `chroot.rs`, `cgroup.rs`):
  it copies the binary into `<base>/firecracker/<id>/root/` (open without
  truncate, chowned to the VM uid), chowns `root/` to the VM uid 0700,
  creates `/dev`, `/dev/net`, `/run` (0700, VM uid) and the device nodes
  with `mknod` (fails if they exist), writes `firecracker.pid` with
  `create_new` (fails if it exists), and with `--new-pid-ns` the jailer
  parent exits after writing the child's host pid. It prepends
  `--id <id> --start-time-us ...` to Firecracker's arguments; arguments
  after `--` go to Firecracker. Cgroup v2: it writes `+<controller>` into
  every ancestor's `cgroup.subtree_control` and moves itself into
  `<parent>/<id>`. It clears the environment and closes fds above 2.
- vmctl's mount namespace has `/var/lib/go-choir` and
  `/var/lib/go-choir/vm-state` as **separate mounts** (systemd
  `ReadWritePaths`), so a hard link from `vm-state/<vm>/data.img` to
  anywhere outside the vm-state mount fails with `EXDEV`. vm-state is
  `nosuid` but not `nodev`. Guest images live in `/nix/store` (another
  mount, same btrfs filesystem).
- `/sys/fs/cgroup` is read-only for vmctl (`ProtectControlGroups=yes`);
  vmctl and its three VMMs share `system.slice/go-choir-vmctl.service`,
  whose `MemoryLow=14G` and `MemorySwapMax=0` protect the guests.
- vmctl's bounding set is `CAP_NET_ADMIN CAP_SYS_PTRACE`; its syscall filter
  denies `@mount`. `CAP_SYS_PTRACE` came with 7a15266d7 with no stated
  reason; no vmctl code needs it (reading `/proc/<pid>/fd/1` of a root,
  dumpable VMM needs no capability; a jailed VMM's console is a file).
- No host user exists with uid >= 900000.

### Shape

One flag, `VMCTL_JAILER=off|on` (default off), plus `VMCTL_JAILER_ONLY`, a
comma list of VM ids or computer ids that narrows `on` to named computers
(staging rollout: one test computer, then all). A VM's mode is decided at
each launch (cold boot, refresh, recover, resume, snapshot restore); a
running VM keeps the mode it was launched with until its next launch.

Jailed launch:

```text
jailer --id <vm> --exec-file <firecracker> --uid U --gid U
       --chroot-base-dir <state>/.jail --new-pid-ns
       --cgroup-version 2 --parent-cgroup <vmctl service cgroup>/vms
       --cgroup pids.max=128 --cgroup cpu.weight=100
       --resource-limit no-file=2048
       -- --api-sock /run/fc.sock --enable-pci --config-file /fc-config.json
```

- **uid/gid**: one per VM, `gid = uid`, from 900000..904095. Allocated once,
  written durably to `vm-state/<vm>/vmm-uid` (temp file, fsync, rename,
  fsync dir) under a manager lock after scanning every VM dir's `vmm-uid`;
  first choice `base + fnv32(vm) mod count`, linear probe on collision. The
  uid lives and dies with the VM's state dir, so it is never reused while
  that VM's files exist; restarts read the same file.
- **jail**: `<state>/.jail/firecracker/<vm>/root/` (inside the vm-state
  mount, so `data.img` can be a hard link). Before every launch vmctl
  removes `<state>/.jail/firecracker/<vm>/` and rebuilds it: parents root
  0700, verified with `Lstat` (directory, not a symlink, root-owned, not
  group/other writable: the CVE-2026-1386 lesson). Files: `data.img` and
  `credential.img` (and a legacy `rootfs.img`) are **hard links** chowned
  to the VM uid, 0600 (the same inode as the host file, so the host file is
  owned by that uid too); `kernel`, `initrd` and read-only drives
  (`store.img`) are **reflinks** (`FICLONE`) of the Nix store files,
  falling back to a full copy, root-owned 0444; never a hard link to the
  store and never a chown of a source. `fc-config.json` is written with
  jail-relative paths (`/kernel`, `/initrd`, `/store.img`, `/data.img`,
  `/credential.img`); the host copy in `vm-state/<vm>/` holds the same
  bytes, so the snapshot launch-config hash covers the jail paths.
- **network**: unchanged firewall model. vmctl creates the tap in the host
  namespace with `ip tuntap add ... user U group U` (an existing tap is
  deleted and recreated), so the capability-less VMM can attach to it;
  every tap rule is as today. VMM uids get no IP: `OUTPUT -m owner
  --uid-owner 900000-904095 -j DROP` (iptables and ip6tables), installed at
  vmctl start when the jailer is enabled. Per-VM network namespaces (the
  jailer's `--netns`, with the tap moved into the namespace and a veth to
  the host) are slice 2.
- **cgroup**: the vmctl unit gets `Delegate=yes` and
  `DelegateSubgroup=supervisor`, so vmctl runs in
  `go-choir-vmctl.service/supervisor` and each jailed VMM in
  `go-choir-vmctl.service/vms/<vm>`, still under the unit's `MemoryLow` and
  `MemorySwapMax=0`. At start (jailer enabled) vmctl moves any process left
  in the unit's root cgroup (VMMs launched by an earlier vmctl) into
  `supervisor`, because cgroup v2 refuses to enable controllers on a cgroup
  that holds processes. No `memory.max` in this slice (measure first; a cap
  that ignores snapshot page cache would kill guests).
- **process tracking**: the jailer parent exits after the fork; vmctl reads
  `root/firecracker.pid`, checks `/proc/<pid>/cmdline` names `--id <vm>`,
  saves the pid as today, and watches exit with a pidfd. The cmdline
  matcher now requires `argv[0]` to be a firecracker binary, so the
  short-lived jailer process (whose `--exec-file` names firecracker) never
  counts as the VM. Reattach after a vmctl restart is unchanged (pid file +
  cmdline).
- **snapshots**: the API socket is `root/run/fc.sock`. Capture asks
  Firecracker for `/snapshot.partial/vmstate` and `/snapshot.partial/mem`
  (a jail dir owned by the VM uid), and after Firecracker stops vmctl
  renames them (same mount) into `vm-state/<vm>/snapshot.partial/`; commit
  is unchanged. Files stay owned by the VM uid, 0600. Metadata records
  `confinement` and `vmm_uid`. Restore rebuilds the jail from the expected
  launch config, hard-links `snapshot/{vmstate,mem}` into `root/snapshot/`,
  and loads `/snapshot/vmstate` and `/snapshot/mem`. A snapshot taken in
  another mode, or under another uid, is refused with the new reason
  `confinement_mismatch` (cold boot). The balloon is config only and needs
  nothing in the jail.
- **privileges**: the launch runs on a dedicated, locked OS thread that is
  discarded after the fork. On it vmctl (a) sets `PR_SET_SPECULATION_CTRL`
  force-disable for indirect branches (IBPB/STIBP) and speculative store
  bypass (SSBD), inherited by the VMM across exec (always for jailed
  launches; `VMCTL_VMM_SPECULATION_BARRIERS=on` opts direct launches in),
  and (b) drops the bounding set: for the jailer to `SYS_ADMIN MKNOD CHOWN
  SETUID SETGID DAC_OVERRIDE` (the VMM itself runs as a non-root uid and
  holds no capability), for a direct VMM to `NET_ADMIN` only (it attaches
  to an ownerless tap). The VMM environment is empty in both modes (today
  it inherits `VMCTL_ROUTE_DSN`).
- **vmctl unit**: bounding set `NET_ADMIN SYS_ADMIN MKNOD CHOWN SETUID
  SETGID DAC_OVERRIDE FOWNER KILL SETPCAP` (no `SYS_PTRACE`);
  `DAC_OVERRIDE`/`FOWNER`/`KILL` because VM files, sockets, `/proc/<pid>`
  entries and signals now cross uids; `SETPCAP` for the bounding drop;
  syscall filter adds `@mount`; `ProtectControlGroups=no`.
- **receipt**: the boot timeline carries a `confinement` block (mode, why,
  uid, jail root, cgroup, speculation and capability-drop results),
  reachable through vmctl's boot-timeline route without SSH.
- **verification**: `scripts/vmm_confinement_verify.py` reads, for each
  Firecracker pid, `Uid`, `CapEff`, `CapBnd`, `NoNewPrivs`, `Seccomp`,
  speculation lines, `readlink root`, `ns/mnt`, `ns/pid`, `ns/net` and
  `cgroup`, and judges each against the contract above.

### Failure modes (written before code)

1. Jail path symlink/race (CVE-2026-1386 class): parents root 0700 and
   `Lstat`-checked; the per-VM jail is removed and rebuilt each launch.
2. `data.img` hard link crosses a mount (`EXDEV`): the jail is inside the
   vm-state mount; `EXDEV` fails the launch, never a copy of a writable
   disk.
3. Store images on another mount: reflink, else copy; never link or chown
   the source.
4. uid collision or reuse while files remain: durable per-VM `vmm-uid`,
   allocated under a lock after a scan; freed only with the state dir.
5. uid range overlaps a host user (`/etc/passwd`): jailer host marked
   unprepared, launches go direct with a receipt.
6. Capability-less VMM cannot attach an ownerless tap: tap owned by the VM
   uid; an existing tap is recreated.
7. Leftovers from the last launch (`firecracker.pid` `create_new`, `mknod`
   `EEXIST`, binary copy without truncate): whole jail dir rebuilt.
8. VMM exit not seen after `--new-pid-ns` (jailer parent exits): pid file +
   cmdline check + pidfd watcher; unexpected exit marks Running/Pending
   instances failed.
9. The jailer process counted as the VM's Firecracker: matcher requires
   `argv[0]` basename `firecracker`.
10. Duplicate `--id` (jailer prepends one): stripped from Firecracker args.
11. Controllers cannot be enabled on a cgroup holding processes:
    `DelegateSubgroup=supervisor`, start-time migration of root-resident
    processes, `vms` parent created.
12. VMs losing `MemoryLow`/`MemorySwapMax=0`: leaves stay under the unit.
13. A `memory.max` that OOM-kills during snapshot writes: none in slice 1.
14. Snapshot files lost with the jail: renamed out to `snapshot.partial/`
    after Firecracker stops and before commit.
15. Mixed-mode or other-uid resume: `confinement_mismatch` refusal plus a
    differing launch-config hash; cold boot.
16. Restored VM cannot open its drives (vmstate holds jail paths): the
    restore jail is rebuilt with the same layout.
17. API socket path over 107 bytes: dial through `/proc/self/fd/<dirfd>/`.
18. vmctl blocked by uid-owned files, sockets, `/proc` entries, signals:
    `DAC_OVERRIDE`, `FOWNER`, `KILL` in vmctl's bounding set.
19. Direct VMMs inheriting the widened set: bounding drop to `NET_ADMIN` on
    the launch thread; a non-legacy capability that cannot be dropped fails
    the launch.
20. Speculation or bounding changes leaking into vmctl: discarded locked
    thread (Go creates new threads from its template thread while the
    caller is locked); prctl errors are recorded, not fatal.
21. vmctl secrets in the VMM environment: empty environment.
22. Console adoption of a jailed VMM needs ptrace across uids: jailed
    consoles are files; adoption skips the pipe probe for them.
23. Rollback to direct with uid-owned disks: direct launch chowns
    `data.img`/`credential.img` back to root and removes the stale jail.
24. VMM uid IP connectivity: owner-match DROP (v4, v6); if it cannot be
    installed the host is unprepared and launches go direct with a receipt.
25. Jailer host unprepared (binary missing, cgroup migration, firewall, uid
    range): every launch direct, reason in the boot timeline and log.
26. VM id the jailer refuses (not `[A-Za-z0-9-]{1,64}`): direct, receipt.
27. Destroyed VM leaves a jail and cgroup leaf: both removed by
    `DestroyVMState`; an empty leaf is removed after its VMM exits.
28. vmctl restart with jailed VMs: reattach unchanged; `confinement.json`
    in the VM dir tells adoption the console is a file.

### Ceremony

- Conjecture delta: confining each VMM to its own uid, chroot, mount/pid
  namespace and cgroup leaf (and no IP) turns a VMM escape from "every
  tenant and the host network" into "one computer's files", at the cost of
  one cold boot per hibernated computer at the cutover.
- Protected surfaces: vmctl (launch, kill, reattach), VM lifecycle and
  snapshot validity, the host firewall (one OUTPUT rule), the vmctl unit's
  privileges and cgroup.
- Admissible evidence: `/proc/<pid>/status`, `root`, `ns/*`, `cgroup` of
  each VMM on Node B (the verify script's JSON), the boot timeline's
  confinement block, and the lifecycle proofs (boot, hibernate by
  snapshot, wake, stop/start, vmctl restart) on a disposable staging
  computer. Local tests prove argument building, layout and path rewriting
  only.
- Rollback: `VMCTL_JAILER=off` (or an empty `VMCTL_JAILER_ONLY` with the
  flag off) for new launches; jailed computers keep running until their
  next launch, which is direct (ownership reclaimed, snapshot refused, cold
  boot). Code: `git revert`. The unit changes revert with the Nix file.
- Heresy delta: discovered: VMMs inherit vmctl's whole environment
  (`VMCTL_ROUTE_DSN`); `CAP_SYS_PTRACE` had no consumer. Introduced: none
  known (vmctl's own bounding set grows because it runs the jailer; the
  privilege split in §4.3 of the review is the follow-up). Repaired (when
  on): finding F1 of the review for jailed computers.

### Residuals (not in slice 1)

Per-VM network namespace (slice 2); `memory.max` after measurement;
`/dev/kvm` back to 0660; direct-mode VMMs keep `NET_ADMIN` until the
direct path is removed (step e); vmctl privilege split; core scheduling.
