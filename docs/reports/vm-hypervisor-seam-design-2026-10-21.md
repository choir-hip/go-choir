# The Hypervisor Seam: Design Against Main and FP8 (RES-20)

Date: 2026-10-21. Base: `main` at d1c60ad87, plus FP8's branch
`lane/fp8-image-jailer` at ee2dcf0a3 (read, not landed). Station:
[FP10 RES-20](../definitions/choir-launch-fp10-res-20-2026-10-21.md).
Problem: [vmmanager-has-no-hypervisor-seam-2026-10-21.md](../problems/vmmanager-has-no-hypervisor-seam-2026-10-21.md).

This report updates
[vm-backend-interface-2026-10-14](vm-hypervisor-seam-design-2026-10-21.md)
(the interface, failure modes F1 to F8, S1 to S5 and CH1 to CH4, and the
staging test plan still hold) for what changed since: the jailer and
virtio-mem landed on main, and FP8 adds a per-computer VMM build. Read
the 10-14 report for the interface code; this one says where it now
attaches, what changes, and in what order.

Class: green (this document). Every build step below is red (VM
lifecycle, vmctl, snapshot validity, VMM confinement).

## 1. The answer in short

- **The seam sits inside vmmanager, between the decision and the VMM.**
  vmmanager keeps deciding everything about a computer (cmdline, drives,
  tap, epoch, credential disk, memory and hotplug region, discard,
  confinement mode). A `Backend` only turns that decision into a VMM's
  files, argv and API calls, and recognises its own processes in `/proc`.
  A `Cage` turns a launch plan into a confined launch. Lifecycle stays in
  vmmanager.
- **FP8's per-computer build is the selection mechanism.** `vmm-build.json`
  becomes the one launch record (it gains `backend`), and
  `computer_select.go` is the one selector. The recovered package's
  `vm-backend.json` and `VM_BACKENDS_ENABLED` are dropped.
- **Cloud Hypervisor needs a Choir cage**, built from the jailer code
  already on main (uid allocator, private jail dir, cgroup leaf, no-IP
  rule, shared read-only inodes, receipt, post-launch check), minus the
  Firecracker jailer binary.
- **What CH gives that Firecracker does not:** VFIO/GPU, copy-on-write
  restore, live migration, vCPU core scheduling, tap passed as an fd, and
  Landlock. virtio-mem is no longer a reason: Firecracker has it on main.
  CH density is unmeasured, and no density claim is made.
- **First build step:** M0, after FP8 lands: rebase the recovered package
  with no caller and re-capture its golden from post-FP8 vmmanager.

## 2. Where the seam goes

```text
vmctl  (ownership, admission, leases, deploy refresh)        unchanged API
  │  BootVM / Resume / Hibernate / SetMemoryHotplugRequestedMiB / LiveVMs
  ▼
vmmanager  — decides                                          neutral
  VMConfig ─► MachineSpec          cmdline, drives (+discard), tap, MAC,
                                   vCPUs, MemMiB, Hotplug{total,block,slot},
                                   Balloon, API socket wanted
           ─► vmmFor(cfg)          FP8: {backend, binary, jailer, next}
  ─────────────────────────────── the seam ───────────────────────────────
vmbackend.Backend   — formats      PlanLaunch(spec) -> LaunchPlan (data)
                                   PlanRestore, VMIDFromCmdline, Version
  optional by Caps():
    Snapshotter                    Pause, CreateSnapshot, CompleteRestore
    MemoryHotplug                  Status, SetRequestedMiB, UnplugAll
    BalloonController              SetTargetMiB
vmbackend.Cage      — confines     Confine(plan, Confinement) -> plan
    direct (today's thread policy), firecracker-jailer, choir-cage (CH)
vmbackend.Start(plan) -> Process   exec, fds, pid
  ▲
vmmanager  — lifecycle             epochs, receipts, tap/iptables, health,
                                   guest fence, snapshot budget and
                                   validation, OOM score, pid file, console
```

Three rules, kept from 10-14 and checked again against main:

1. **The backend formats; vmmanager decides.** A backend never reads
   `ManagerConfig` or the environment. That is what lets the Firecracker
   backend's bytes be checked against `buildFirecrackerConfig` before the
   old code is deleted.
2. **A plan is data.** `LaunchPlan` (argv, env, files to write, fds,
   `HostPaths` with `r`/`rw`, API socket, config digest) is checked
   without KVM, and a cage rewrites it before exec.
3. **Capabilities, not type switches.** A computer whose backend lacks
   `Snapshot` hibernates by kill; one whose backend lacks `MemoryHotplug`
   is never selected for a hotplug region (vmctl's lease controller sees
   `ErrMemoryHotplugUnavailable`, as today for an unselected computer).

## 3. What moves behind the seam (main d1c60ad87 + FP8)

Line numbers are main's. "FP8" marks files that exist only on
`lane/fp8-image-jailer`; re-read them after it lands.

| today | goes to | note |
|---|---|---|
| `buildFirecrackerConfig` (`manager.go:1714`) | neutral half builds `MachineSpec` and stays; format half is `FirecrackerBackend.PlanLaunch` | `guestIdentityKernelParams` (`:1892`) and the cmdline stay neutral |
| `firecrackerBalloonConfig` (`balloon.go:17`) | `MachineSpec.Balloon`; format per backend | |
| `firecrackerMemoryHotplugConfig` (`memory_hotplug.go:108`), `validateMemoryHotplugSizes` (`:76`) | `MachineSpec.Hotplug`; validation per backend (Firecracker's rules are 1.15.1's) | new since 10-14 |
| `applyDataDriveDiscard` (`data_discard.go:74`), `firecrackerSupportsBlockDiscard` (`:33`), FP8's per-build discard | `Drive.Discard` decided by vmmanager from the build; format per backend | new since 10-14 |
| `launchFirecracker` (`manager.go:2051`), `firecrackerLaunchArgs*` (`snapshot.go:309-327`) | `PlanLaunch` + `Cage.Confine` + `Start` | |
| `startFirecrackerProcess` (`manager.go:2150`), `startWithThreadPolicy` (`launch_linux.go:26`) | the `direct` cage + `Start`; console sink, OOM score, pid file and exit watcher stay neutral | |
| `jailerArgs`, `rewriteFirecrackerConfigForJail`, `jailFirecracker*Args`, `startJailedFirecracker` (`confinement.go:300-409, 946`) | the `firecracker-jailer` cage | |
| `allocateVMMUID`, `prepareJail`, `ensurePrivateDir`, cgroup leaf, `ensureVMMNoIPRules`, `ConfinementReceipt`, FP8 `jail_shared.go` | shared cage substrate, used by both jailer and choir-cage | backend-neutral already in substance |
| `waitForJailedExec` (`confinement.go:1061`) | `Registry.Identify` | today it asks the Firecracker matcher |
| `firecrackerRestoreArgs` (`snapshot.go:328`), `fcapi.go` Pause/CreateFullSnapshot/LoadSnapshot | `Snapshotter` | |
| `fcapi.go` `PatchMemoryHotplug`, `memoryHotplugStatusVia`, `unplugBeforeSnapshot` (`memory_hotplug.go:164, 227`) | `MemoryHotplug` | new since 10-14 |
| `launchConfigHash` (`snapshot.go:341`), `launchConfigHasMemoryHotplug` (`memory_hotplug.go:260`) | `LaunchPlan.ConfigDigest`; hotplug read from the spec, not by parsing Firecracker JSON | |
| `firecrackerBuildVersion` (`boot_timeline.go:199`) | `Backend.Version` | |
| `firecrackerCmdlineMatchesVM`, `firecrackerPIDsForVM*`, `firecrackerCmdlineBytesMatchVM` (`manager.go:2397-2458`), `LiveFirecrackerVMs`, `firecrackerVMIDFromCmdline` (`process_control.go:25-80`) | `Registry.Identify / LiveVMs / PIDsForVM` (one permissive rule; 10-14 heresy 1) | |
| `VMResidentBytes` (`process_control.go:127`), vmctl `host_capacity.go:41,94,235`, `host_admission.go:102,508` | read through `Registry.LiveVMs`; vmctl's interface method renamed `LiveVMs` | admission must see every VMM |
| `killFirecrackerProcess`, `liveFirecrackerForVMLocked`, `cleanupOrphanedFirecrackerLocked` (`manager.go:2241-2316`), `ReapUnmanagedVM`, `SetVMOOMScoreAdj` | stay, renamed, ask the `Registry` | |
| FP8 `vmm_next.go` `vmmBuild`, `vmmFor`, `vmm-build.json` | `vmmBuild` gains `Backend`; `vmmFor` picks the backend first, then the build | §4 |
| `pidPath` = `firecracker.pid`; `SnapshotMeta.FirecrackerVersion`; boot timeline `FirecrackerBin/Ver` | names kept; add `backend` fields; missing means firecracker | reattach and old snapshots survive the deploy |
| `IsFirecrackerAvailable` (`config.go:242`) | "the default backend is available"; the fail-closed gate stays | |

Already neutral and untouched: taps and the host firewall
(`hostfirewall.go`), epochs, health probes, the guest hibernate fence,
boot timelines, snapshot budget and disk identity (`snapshot_disk*.go`),
receipts, the console file, guest reclaim (FP8 `guest_reclaim.go`, a
kernel parameter).

## 4. Selection and the launch record: one of each

FP8 already solves "a second VMM binary for one test computer first":

- `computer_select.go`: ids, `class:test`, `*`, read at every launch;
- `vmm-build.json`: the build a VM launched with, read after a vmctl
  restart and bound into its snapshot.

The seam extends both instead of adding the recovered package's
`vm-backend.json` and `VM_BACKENDS_ENABLED` policy:

- `VM_CLOUD_HYPERVISOR_BIN` and `VM_CLOUD_HYPERVISOR_ONLY` (selector
  syntax). Empty `_ONLY`: CH is off. A missing or non-executable binary is
  refused at load and logged; every launch uses Firecracker (FP8's rule).
- **Hard guard:** CH is selected only for a computer whose warmness class
  is `test`, whatever the selector says (`*` included). Widening this is
  an owner act and a code change, not an env edit (failure S4).
- `vmmBuild{Backend, Binary, Jailer, Next}`; a record without `backend`
  reads as firecracker (S1); an unknown backend is an error (S2).
- **Selection applies at cold boot only.** A restore uses the backend in
  the snapshot's record. If that backend is no longer enabled on the host
  (the flag was rolled back), restore falls back to a cold boot with
  reason `backend_unavailable`. This replaces the 10-14 rule "switch only
  from stopped", which a host-level selector cannot enforce: the selector
  can change while a computer is hibernated, and honouring the snapshot's
  backend means the change never turns a planned wake into a crash boot
  (the restart rule). In the first CH slice CH has no `Snapshot`, so a CH
  computer hibernates by kill and its wake is a cold boot by design.

## 5. Confining Cloud Hypervisor without a jailer

CH ships no jailer, and the Firecracker jailer cannot carry it (it closes
fds above 2, so the tap fd is lost, and prepends `--id`, which CH
rejects). Choir builds the cage. Two shapes were weighed:

| | Go cage in vmctl (recommended) | systemd transient unit |
|---|---|---|
| reuse | the jailer substrate on main: uid allocator, jail dir, cgroup leaf, no-IP rule, shared inodes, receipt | none; a second implementation of the same contract |
| tap fd | kept by construction (`ExtraFiles`) | systemd-run cannot hand an arbitrary fd to a transient service |
| new dependency | none | D-Bus and systemd on the launch path of every CH computer |
| verify | the same post-launch `/proc` check as the jailer | same |

**The choir-cage**, a re-exec of the vmctl binary (`vmctl vmm-cage`, so
no new package in the closure), does in order:

1. new mount and PID namespaces; mount propagation private;
2. a per-VM root under the jail base, populated only from
   `LaunchPlan.HostPaths`: read-only artifacts by FP8's shared-inode hard
   link, `data.img` and `credential.img` by hard link chowned to the VMM
   uid (as the jailer path does now), `/dev/kvm` by `mknod`; then
   `pivot_root`;
3. joins its cgroup v2 leaf (memory, pids, cpu weight, the jailer's
   defaults);
4. rlimits (`NOFILE`), `PR_SET_NO_NEW_PRIVS`, drops the whole bounding
   set, `setgroups([])`, `setgid`, `setuid` to the allocated VMM uid;
5. execs `cloud-hypervisor` with fd 3 (the tap, opened by vmctl before the
   cage with `IFF_TAP|IFF_NO_PI|IFF_VNET_HDR`) and `--seccomp true`;
   Landlock rules from the same `HostPaths` once proven on the test host.

The VMM uid is in the no-IP range, so CH cannot open sockets to the host
network, the same as a jailed Firecracker. After exec, vmmanager reads
`/proc/<pid>/{status,cgroup,ns/*,root}` and compares them with the
`ConfinementReceipt`; any mismatch kills the VMM and fails the boot
closed.

**Order rule:** CH never launches outside the cage, not even on a test
computer. This is stricter than 10-14 (which allowed CH as root on test
computers), because the jailer is now on main and a larger device model
should not run with more privilege than the smaller one.

## 6. What Cloud Hypervisor gives

Checked against the 10-14 research and main as it is now. "Unmeasured"
means no claim.

| gain | Firecracker on main | Cloud Hypervisor | worth it for |
|---|---|---|---|
| **VFIO / GPU passthrough** | none | yes | video app desks; the reason Fly.io runs CH for GPU machines. The main driver of RES-20 |
| **virtio-mem** | **yes** (1.15.1, `memory_hotplug.go`, lease controller in vmctl) | yes (`--memory hotplug_method=virtio-mem,hotplug_size=`, resized with `vm.resize desired_ram`), plus ACPI memory and CPU hotplug | not a reason to switch; the seam must carry the region so CH computers keep leases (`MemoryHotplug`) |
| **balloon resize** | config and `PATCH /balloon` upstream; Choir sets it at boot only | `vm.resize-balloon` (name to verify at the pin) | equal |
| **snapshot / restore** | full snapshot, used for hibernate (S3) | full snapshot, restored paused; sparse memory file (v52); UFFD on-demand restore and an offload daemon (v53) | hibernate is equal; CH's offload daemon is a ready path to a compressed and encrypted cold tier |
| **copy-on-write restore** | no | `memory_restore_mode=copyonwrite`: many VMs restored from one snapshot share its pages through the host page cache | S9 forks: N forks of one parent cost one parent's memory plus their writes. Not compatible with virtio-mem, hugepages, KSM or VFIO, so a fork template has no hotplug region |
| **live migration** | no | yes (mTLS v53; n-2 versions from v54); same CPU vendor only | a second same-vendor host: evacuate busy computers for host upgrades instead of a planned restart |
| **core scheduling** | no (would be the launcher's job) | `core_scheduling=vm` by default: SMT siblings run threads of one VM only | DEC-06: keeps SMT on with tenant isolation, instead of turning SMT off and losing busy capacity |
| **tap by fd** | no; the VMM opens `/dev/net/tun` | yes (`--net fd=3`) | a smaller cage: no `/dev/net/tun`, no `CAP_NET_ADMIN` ever |
| **Landlock** | no | yes, rules from `HostPaths` | defence in depth inside the cage |
| **discard** | 1.17 (FP8's next build) | yes | equal |
| **density per idle VM** | measured (density report ladder) | **unmeasured**; more threads and devices suggest more VMM overhead | needs the idle ladder on one test computer (step M7) before any claim |

Costs that stay: a larger device model (CVE-2026-45782 and -27211 were
CH's); CH's threat model treats snapshot files and migration peers as
trusted input, so the snapshot directory must not be writable by the VMM
uid and any migration listener is a protected surface; and every S3 and
lease change must then be proved on both backends or flagged
Firecracker-only by capability.

## 7. Failure modes added since 10-14 (write the tests first)

The 10-14 table (F1-F8, S1-S5, CH1-CH4) stands. Added:

| id | failure | consequence | test |
|---|---|---|---|
| F9 | the spec drops the hotplug region or discard flag in translation | a lease computer boots without virtio-mem; the lease controller fails; or discard turns off silently | equivalence test in M2 covers hotplug and discard modes, jailed and direct |
| F10 | two launch records (`vmm-build.json`, `vm-backend.json`) disagree | reattach or restore picks the wrong VMM | there is one record (§4); test that a record with `backend` round-trips and one without reads firecracker |
| F11 | `VMResidentBytes` or `LiveFirecrackerVMs` misses a CH process | admission undercounts host memory; the reconcile reaps or ignores a live computer | M1 before any CH launch; a fake second backend in the registry test is counted by admission |
| F12 | the cage loses fd 3 or execs with a capability left | no guest network; or a VMM with privilege | cage unit test on the plan; post-launch `/proc` check kills on mismatch |
| F13 | `waitForJailedExec` waits for a process named firecracker | every caged CH boot times out | it asks `Registry.Identify` (M1) |
| F14 | the selector names a production computer (`*`, a user id) | CH reaches a user | the hard test-class guard (§4) |
| F15 | a snapshot is restored under a backend other than the one that took it | load fails or, worse, misbehaves | snapshot meta carries `backend`; restore honours it or cold boots (`backend_unavailable`) |
| F16 | plugged virtio-mem memory at snapshot (CH) | snapshot larger than budget, or CoW restore refused | `MemoryHotplug.UnplugAll` before `CreateSnapshot` for both backends, as `unplugBeforeSnapshot` does now |
| F17 | the VMM uid can write its snapshot directory | a guest-controlled VMM forges the next restore's input (CH trusts snapshot files) | snapshot dir root-owned, linked read-only into the jail (as `linkSnapshotIntoJail` does) |

## 8. Migration, re-sequenced (after FP8 lands)

Each step one commit, revertible alone, through the Landing Loop. Path
classifier note from 10-14 stands: any `internal/vmmanager/` change costs a
vmctl deploy and an active-VM refresh, so M0 rides with M1.

| step | change | class | proof |
|---|---|---|---|
| M0 | Rebase `vmbackend` from `recovered/vmbackend-2026-10-14` onto main after FP8. Add `Hotplug`, `Drive.Discard`, `MemoryHotplug`; drop `selection.go`'s record and policy (§4); add the `Cage` type. Re-capture the Firecracker golden from post-FP8 `buildFirecrackerConfig`, jailed and direct, with and without hotplug and discard. No caller | yellow | package tests; golden equals vmmanager's bytes and `launchConfigHash` |
| M1 | Discovery through `Registry` (Firecracker only): all matchers, `waitForJailedExec`, `VMResidentBytes`, vmctl's `LiveVMs` | red | existing tests; staging: vmctl planned restart reattaches every computer, admission readings unchanged |
| M2 | Firecracker launch through `PlanLaunch` + cages (`direct`, `firecracker-jailer`) + `Start`; delete the format code | red | equivalence test (F4, F9); staging: wake of a computer hibernated before the deploy, no `launch_config_mismatch` |
| M3 | Snapshots and hotplug through `Snapshotter` and `MemoryHotplug`; delete `fcapi.go`; `backend` in `vmm-build.json`, snapshot meta and boot timeline | red | S3 and hotplug tests; staging: hibernate, wake, a lease plug and unplug |
| M4 | choir-cage (`vmctl vmm-cage`), exercised by Firecracker first behind a selector (`VMCTL_CAGE_ONLY`), so the cage is proved on the VMM we know | red | staging: one test computer boots caged, receipt matches `/proc` |
| M5 | Cloud Hypervisor backend behind `VM_CLOUD_HYPERVISOR_ONLY` (off), the test-class guard, CH1's reboot watcher, `backend_no_snapshot` hibernate, the Nix package and env behind a host flag (off) | red | `real_linux_test.go` on a KVM host; then the 10-14 §10 staging plan on one test computer |
| M6 | CH `MemoryHotplug` (virtio-mem leases on CH) | red | lease plug and unplug on the CH test computer |
| M7 | CH idle density ladder against Firecracker, same shape | measure | evidence under `docs/evidence/`; only then any density statement |

Each later CH capability is its own mission: CH snapshots, CoW restore for
S9 forks, VFIO for video app desks, live migration.

## 9. Open questions and decisions

- **Owner:** the supervised metamission
  (`definitions/choir-supervised-app-development-metamission-2026-10-01.md:536`)
  still excludes "cloud-hypervisor, CPU/RAM hotplug". virtio-mem hotplug
  has since landed on main, and the owner named CH a production candidate
  on 2026-10-14. That owner-dated exclusion should be amended by the
  owner or FP0, not by this lane.
- **Owner (DEC-06):** whether CH's core scheduling is the answer to SMT
  before strangers. It only helps computers that run on CH.
- **Unverified at the pin** (needs the KVM host, `real_linux_test.go`):
  PVH ELF note in the guest `vmlinux` (CH boots only PVH on x86_64); tap
  fd acceptance and link; guest device names under CH's PCI enumeration;
  CH's virtio-mem block size rules and the balloon resize endpoint name;
  whether nixpkgs now carries CH v54 (main-thread seccomp).
- **Design choice made here, conservatively:** the Go cage over a systemd
  unit (§5), and no CH outside the cage (§5). Both can be revisited by a
  panel at M4.

## 10. Heresy and conjecture delta

- *discovered:* the recovered package and FP8 each add a launch record
  and a selector (F10); `waitForJailedExec` ties the jailer path to the
  Firecracker matcher (F13); the 10-14 capability matrix says Choir's
  Firecracker has no virtio-mem, which main contradicts.
- *introduced:* none (docs).
- *repaired:* none yet; M0 repairs the stale matrix and golden, M1 the
  identity split and F13, M3 F10.

**Conjecture:** the seam is pure formatting, identification and caging;
lifecycle stays neutral. Since 10-14 it has absorbed two more
Firecracker-specific features (virtio-mem, discard) as spec fields, with
no lifecycle change. Refuted if M2's equivalence test or the
hibernate-across-deploy check fails, or if the CH cage needs lifecycle
changes beyond the receipt check.


## Merged from docs/reports/vm-backend-interface-2026-10-14.md (2026-10-14)

**VM Backend Interface: Firecracker and Cloud Hypervisor Behind One Seam.**
Date: 2026-10-14, about 21:00 UTC. Base: `origin/main` at `980ffaf4e`.
- Evidence goes to `docs/evidence/vm-backend-ch-staging-<date>/`: timelines,
- evidence so far: Firecracker's config is reproduced byte for byte from a

(Full text in git history at the path above.)
