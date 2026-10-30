# Isolation Runtimes for Choir: Firecracker, Cloud Hypervisor, gVisor, libkrun and Others

Status 2026-10-30: line 243 lists Dolt in the guest image; guests run SQLite now. Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

Date: 2026-10-14, about 16:00 UTC. Owner (about 15:45 UTC): "Also been
meaning to consider Cloud Hypervisor for Node B... pros/cons? Really I want
to do both (and gVisor? And more? libkrun?) for our agentic red-teaming
work soon."

Mutation class of this document: green (docs only). It changes nothing.
Each proposal names its own class.

Sources: the code at `f54f05c42` (`internal/vmmanager`, `internal/vmctl`,
`internal/capsule`, `nix/node-b.nix`, `nix/autoputer-vm.nix`); the
reports [vm-density-limits-2026-10-14](../reports/vm-density-limits-2026-10-14.md)
and [roadmap-sequencing-appdev-security-2026-10-14](../archive/reports/roadmap-sequencing-appdev-security-2026-10-14.md);
[computer-ontology](../computer-ontology.md); the S9 and SP definitions;
upstream release notes, docs and security advisories read today through
the GitHub API and the web (URLs in the Sources section at the end). No
runtime was installed or benchmarked for this report. Every performance
number is either an upstream claim (cited) or an estimate (labelled).
`docs/reports/desktop-sync-and-local-vms-2026-10-14.md` is a parallel survey
that is not on `main` yet; section 6 states what it needs from this one.

## The short answer

1. **Node B stays on Firecracker.** Every density lever already planned
   (Diff snapshots, free page reporting, virtio-mem, virtio-pmem, UFFD) is
   a Firecracker 1.14 to 1.17 feature, and upgrading from 1.15.1 to 1.17.0
   adds the one thing the density report said Firecracker lacked:
   virtio-blk discard. Switching Node B to Cloud Hypervisor now would cost
   a red-class rewrite of the snapshot path in the middle of S3 and LR for
   gains Node B cannot use yet (no GPU; live migration needs a second host
   with the same CPU family; crash failover is not helped by live
   migration at all).
2. **Build the backend seam now, put Cloud Hypervisor behind it second.**
   vmctl already talks to the manager through an interface
   (`VMManager`, `internal/vmctl/ownership.go:249`). The Firecracker
   coupling is inside `vmmanager.Manager`, in about eight places. A small
   `Backend` interface there (section 5) lets one host run Firecracker and
   Cloud Hypervisor side by side, chosen per computer. Cloud Hypervisor
   earns its place with copy-on-write restore (cheap S9 fork fan-out), live
   migration (planned host evacuation for SP once a same-CPU second host
   exists), VFIO (GPU computers later) and core scheduling (an SMT
   side-channel control without turning SMT off).
3. **gVisor belongs inside and beside the computer, not instead of it.**
   It cannot run the NixOS computer as is (systemd, ext4 data image,
   memory snapshot of a whole OS). It fits two places: capsule children
   inside the guest (runsc with the systrap platform needs no nested
   virtualization), and host-side tools that touch untrusted content
   (fetchers, Chromium). Both are red-team targets.
4. **libkrun is a desktop runtime and a red-team specimen, not a server
   runtime.** Its own security model says the VMM and the guest are one
   security context. It has no snapshots. It runs on macOS (HVF) and Linux
   (KVM), which makes it a candidate for the Wails desktop.
5. **First finding for the red team, before any new runtime:** Choir runs
   Firecracker without the jailer. From the code, every Firecracker process
   inherits vmctl's root user, its `CAP_NET_ADMIN` and `CAP_SYS_PTRACE`,
   and a mount view of every computer's `data.img`. A guest-to-VMM escape
   would therefore be a cross-tenant compromise. That is backend-neutral
   and should be fixed (per-VM confinement wrapper) before LT runs escape
   experiments against the production shape. This should be filed as a
   problem doc first (section 7).

## 1. How Choir uses Firecracker today (checked in code)

- **Launch.** `vmmanager.Manager.buildFirecrackerConfig`
  (`internal/vmmanager/manager.go:1563`) writes a JSON config with a
  read-only erofs nix-store drive, the per-computer `data.img`, an optional
  credential disk, one tap interface, the guest kernel and initrd from
  microvm.nix, and a balloon with free page reporting
  (`balloon.go`). `firecrackerLaunchArgs` (`snapshot.go:290`) runs
  `firecracker --enable-pci --id <vm> --config-file ...`, with
  `--api-sock` in snapshot-hibernate mode and `--no-api` otherwise.
- **Process.** `startFirecrackerProcess` (`manager.go:1942`) runs the binary
  directly from vmctl with `exec.Command`. There is no jailer, no
  per-VM uid, no per-VM cgroup (density lever L17), no per-VM mount
  namespace. Firecracker's own default seccomp filters are on (no
  `--no-seccomp` anywhere). vmctl's unit (`nix/node-b.nix:700-760`) sets
  no `User=`, so it runs as root with `CapabilityBoundingSet =
  CAP_NET_ADMIN CAP_SYS_PTRACE`, `PrivateDevices=false` (for `/dev/kvm`),
  and `ReadWritePaths` over all of `/var/lib/go-choir`. Children inherit
  all of it. (From code; confirm on Node B with `/proc/<pid>/status`.)
- **Snapshots.** `fcapi.go` pauses, takes a Full snapshot, and loads it
  with a File memory backend. Snapshot identity pins the launch config
  hash and the Firecracker build version (`launchConfigHash`,
  `firecrackerBuildVersion`; S3 failure mode 9).
- **Process discovery.** Reattach and reaping match `/proc/*/cmdline` on a
  basename containing `firecracker` (`firecrackerCmdlineBytesMatchVM`,
  `manager.go:2220`).
- **Guest image.** `nix/autoputer-vm.nix:247` builds with microvm.nix
  `hypervisor = "firecracker"`, but vmmanager launches the binary itself.
  microvm.nix supports eight hypervisors (qemu, cloud-hypervisor,
  firecracker, crosvm, kvmtool, stratovirt, alioth, vfkit), so producing a
  second guest variant is a Nix change, not a new image pipeline.
- **Networking.** Per-VM tap with a /30, host-side iptables DNAT and
  FORWARD rules, guest reaches host services on the tap IP
  (`choir.gateway_url`, `choir.vmctl_url`, ...). This is hypervisor-neutral:
  Cloud Hypervisor and QEMU take a named tap the same way.
- **Capsules.** Inside the guest: namespaces, cgroup v2, seccomp, Landlock,
  a capability broker and a transaction tape (`internal/capsule`). Cells
  run in a Yaegi worker with an import allowlist
  (`internal/yaegikernel/allowlist.go`; S1 security floor).
- **Prior decisions.** The app-development metamission excluded "cloud-
  hypervisor, CPU/RAM hotplug (owner 2026-10-01; keep a seam - snapshot
  format behind a vmmanager interface)". The ontology and the 2026-08-21
  architecture review name Cloud Hypervisor dynamic resize as "the
  long-term shape". This report is the seam the owner asked to keep.

Version note: Choir pins Firecracker 1.15.1 from nixpkgs. Upstream is at
**1.17.0** (2026-09-10). Firecracker 1.15.1 is the patched release for
CVE-2026-5747 (virtio-PCI transport out-of-bounds write, affects
1.13.0 to 1.14.3 and 1.15.0). Choir uses `--enable-pci`, so this pin
matters: never roll back below 1.15.1 while PCI is on.

## 2. The runtimes, one by one

### Firecracker (current)

- **Model.** KVM hardware virtualization, Rust VMM, minimal device model:
  virtio-net, virtio-blk, virtio-vsock, balloon, virtio-pmem, virtio-mem,
  virtio-rng, serial, a partial i8042, RTC; virtio-PCI opt-in since 1.13.
  No virtio-fs, no VFIO, no GPU, no live migration, no nested
  virtualization.
- **Defences.** Per-thread seccomp filters by default; the **jailer**
  (chroot, namespaces, cgroups, uid/gid drop) as the second line. Choir
  does not use the jailer today.
- **Track record.** Two GitHub advisories in 2026: CVE-2026-5747 (high,
  virtio-PCI queue_size change after activation, OOB write; reported by
  Xinyang Ge and Claude) and CVE-2026-1386 (medium, jailer symlink
  follow). Recent releases also fix guest-driven host resource bugs
  (virtio-rng over-allocation, balloon stats descriptor stall, vsock
  busy-spin). The surface is small and well fuzzed; the PCI transport is
  the newest and least mature part.
- **Features Choir uses or plans.** Full and Diff snapshots (Diff is still
  a developer preview in the current docs), File and UFFD memory
  backends, balloon free page reporting (1.14), virtio-pmem and
  virtio-mem (1.14, with virtio-mem snapshot fixes in 1.16.2 and 1.17.0),
  2 MiB hugetlbfs, and new in 1.17.0: **virtio-blk discard** (opt-in,
  `Sync` engine only), Transparent huge pages, `sync_snapshot_files=false`
  (skip the fsync on snapshot create), bzImage boot, 6.18 guest kernels.
  1.16 added developer-preview PCI device hotplug.
- **Performance (upstream spec).** VMM up within 8 CPU ms; at most 125 ms
  to guest init; VMM memory overhead at most 5 MiB for a 1 vCPU / 128 MiB
  guest. Choir measured 6 to 7 ms snapshot load and 0.34 to 0.52 s to
  healthy after a wake (density report).
- **Snapshot format.** Versioned per binary; 1.16.2 bumped the format in a
  *patch* release (snapshots from 1.16.0/1.16.1 cannot load). Every
  Firecracker upgrade is a snapshot invalidation unless proven otherwise
  (density lever L20).
- **Platforms.** x86_64 and aarch64 Linux hosts only.
- **Production.** AWS Lambda and Fargate; widely used by agent sandbox
  vendors; Fly.io's CPU machines.
- **Fit for Choir.** Excellent for many small persistent computers on one
  host with memory snapshots. The gaps that matter: no live migration, no
  CoW multi-restore, no GPU, no virtio-fs, no nested virt.

### Cloud Hypervisor

- **Model.** KVM (and Microsoft MSHV) hardware virtualization, Rust VMM
  sharing rust-vmm crates with Firecracker, but a general-purpose cloud
  VMM: PCI, ACPI, CPU/memory/device hotplug, VFIO (now also via iommufd
  and vfio-cdev), vhost-user devices (virtio-fs through virtiofsd),
  vDPA, TPM, UEFI, Windows guests, SEV-SNP and TDX.
- **Defences.** Seccomp filters on by default (`--seccomp=errno` added in
  v53 for debugging), optional Landlock self-sandboxing of the process's
  file access, a published threat model. Its threat model treats the
  management API, migration peers and **snapshot files as trusted input**,
  and says it is not hardened against guest-driven host CPU/memory
  exhaustion: use cgroups.
- **Track record.** GitHub advisories: CVE-2026-45782 (high, virtio-blk
  async I/O use-after-free, affects v21 to v51.1, fixed v52.0/v51.2),
  CVE-2026-27211 (high, host file exfiltration by a guest writing a QCOW2
  header into a raw image that format autodetection then follows; fixed
  v51.0/v50.1), CVE-2023-30612 (API socket fd close). A larger device
  model than Firecracker, so a larger surface, but the same language and
  fuzzing culture.
- **Features relevant to Choir.**
  - Snapshot/restore since 0.8; restored VMs come back paused. v52 writes
    and reads the memory file sparsely (SEEK_DATA/SEEK_HOLE), v52 added
    UFFD on-demand restore, v53 background prefault and an **offload
    daemon** protocol (snapshot streamed to an external process that can
    compress or encrypt it; postcopy).
  - **Copy-on-write restore** (`memory_restore_mode=copyonwrite`): many
    VMs restored from one snapshot share it through the host page cache,
    writes stay private. Not compatible with virtio-mem, hugepages, KSM or
    VFIO. This is the cheapest fork fan-out mechanism on the list.
  - **Live migration** over TCP (mTLS since v53, multi-connection), with
    cancellation and an **n-2 version compatibility guarantee from v54**
    (released today, 2026-10-14). Postcopy since v53. Migration needs
    same-vendor CPUs; an Intel Node B cannot live-migrate to an AMD EPYC.
  - virtio-mem and ACPI memory hotplug, CPU hotplug, balloon with free
    page reporting and deflate-on-OOM.
  - virtio-blk with discard and write-zeroes; raw, QCOW2 and now VMDK
    images (set `image_type=raw` and keep `backing_files` off).
  - virtio-fs (DAX is documented as not stable from the daemon side);
    virtio-pmem.
  - **Core scheduling** for vCPU threads (v52): SMT siblings only run
    threads of the same VM. This addresses the density report's open
    "SMT off for tenant isolation" decision without halving the EPYC.
  - Nested virtualization on x86 KVM (guests can run KVM if the host
    allows it).
  - VMGenID on every restore (reseeds the guest RNG; matters for forks).
- **Performance.** Direct-kernel boot in the same order as Firecracker
  (upstream does not publish a spec table like Firecracker's; unmeasured
  here). Per-VM VMM overhead is likely somewhat higher (more threads and
  devices); **unmeasured, benchmark required** before any density claim.
- **Platforms.** x86_64, aarch64, riscv64 (experimental); Linux KVM or
  MSHV hosts. Not macOS.
- **Production.** Kata Containers backend; Fly.io GPU machines (chosen
  because Firecracker has no PCI passthrough); Microsoft (MSHV, Azure);
  Apple's open-source Containerization package uses it as its Linux
  backend.
- **Fit for Choir.** Strong. Everything Choir does with Firecracker has a
  Cloud Hypervisor equivalent, plus CoW restore, live migration, VFIO and
  core scheduling. The cost is migration work and a larger surface.

### gVisor (runsc)

- **Model.** User-space kernel. The Sentry (Go) implements the Linux
  syscall ABI; the Gofer mediates filesystem access; the Sentry's own host
  syscalls are a short enumerated list under seccomp. Platforms: systrap
  (default since 2023; works in VMs without nested virt), KVM (best on
  bare metal), ptrace (unsupported).
- **Track record.** Memory-safe language, continuous fuzzing, but two high
  CVEs this autumn: CVE-2026-96812 (published 2026-09-25, CVSS 4.0 8.8:
  a `/dev/cuse` node in an image reaches the host through the gofer, host
  root on CUSE-enabled hosts) and CVE-2026-95702 (2026-10-09, 8.5: VFS
  double free gives code execution in the Sentry, still confined by
  host seccomp and namespaces; fixed in 20260831.0). Latest release
  20261005.0. gVisor does not defend against hardware side channels; DoS
  relies on cgroups.
- **Features.** OCI runtime; checkpoint/restore (`runsc checkpoint`,
  compression, background restore, CPU feature masking for cross-host
  restore; connected sockets reset on restore); filesystem snapshots
  (`runsc fscheckpoint`); netstack user-space networking
  (`--network=sandbox`) or host networking; NVIDIA GPU via nvproxy
  (checkpoint of GPU processes reported failing in 2026-01).
- **Performance.** No device emulation and fast start; syscall-heavy and
  file-heavy work is slower than native (Go builds and tests are exactly
  that). Unmeasured here.
- **Production.** GKE Sandbox, Cloud Run (first generation), App Engine;
  many code-execution sandboxes.
- **Fit for Choir.** Not a computer runtime: a Choir computer is a NixOS
  system with systemd, an ext4 data image, Dolt and a memory snapshot of
  the whole OS. gVisor fits (a) capsule children inside the guest, as a
  stronger alternative or additional layer to the namespace capsule, and
  (b) host-side processes that handle untrusted content (research egress
  fetchers, Web Lens, Chromium/Playwright).

### libkrun (krunvm, muvm, krunkit, crun's krun handler)

- **Model.** A *library* VMM: a process links libkrun and becomes a
  KVM (Linux) or HVF (macOS arm64) VM with a minimal device set
  (virtio-console, blk, fs, gpu with venus or native context, net, vsock,
  balloon with free page reporting only, rng). Variants for SEV-SNP and
  TDX. Releases: libkrun 1.19.6 (2026-09-29), `main` is the incompatible
  2.0 API; krunvm 0.2.7; muvm 0.7.0 (2026-10-13); krunkit 1.3.2.
- **Security model (upstream, verbatim in spirit).** The VMM and the guest
  are "a single entity": the host must isolate the VMM with namespaces
  and uid. virtio-fs gives no protection against the guest reaching other
  directories on the same filesystem (issue #329); TSI networking makes
  the VMM proxy the guest's sockets, so egress policy must be applied to
  the VMM process. No GitHub security advisories published; the crun krun
  handler had CVE-2025-24965 (path traversal from a malicious image).
- **Features.** No snapshot/restore, no live migration, no memory hotplug.
  GPU acceleration on macOS (krunkit, used as a Podman machine provider)
  and on Asahi Linux (muvm).
- **Fit for Choir.** Not for Node B computers (no snapshots kills S3
  hibernation). Good candidate for the desktop (one VMM API on macOS and
  Linux, GPU), and a useful red-team specimen precisely because its
  boundary is weaker by design: it tests whether Choir's *host-side*
  containment holds when the VMM is assumed compromised.

### Others worth knowing

| runtime | model | why it matters to Choir | why not the default |
|---|---|---|---|
| QEMU (`-M microvm` or q35) | KVM/HVF/WHPX, C | the only VMM on Linux, macOS and Windows; every feature (live migration, virtio-fs, VFIO, snapshots) | large C device model and a steady CVE stream; microvm machine type trims devices but it is the same binary |
| crosvm | KVM (and WHPX on Windows), Rust | strongest device isolation: each virtio device in its own minijailed process with its own seccomp policy; ChromeOS and Android (AVF, Cuttlefish) | built for client devices; snapshot support is newer; not a multi-tenant server VMM in common use |
| Kata Containers 4.x | orchestration over QEMU, Cloud Hypervisor, Firecracker or Dragonball; runtime-rs (Rust) is the default since 4.0.0 | proof that one manager can drive several VMMs; reference for an abstraction | adds a shim, an agent and virtiofsd; ten advisories since 2026-01, five critical (virtiofs guest-root to host-root CVE-2026-47243, Dragonball virtio-blk escape CVE-2026-93417, annotation path abuse); Choir has no containerd/Kubernetes to justify it |
| Apple Virtualization.framework (vfkit, Lima, `apple/container`) | Apple hypervisor, macOS 11+ | the native macOS desktop path; save/restore of machine state since macOS 14 (arm64 only); virtio-fs, Rosetta for Linux | macOS only; restore fails on any config mismatch (for example a changed MAC) |
| Apple Containerization (Swift) | one lightweight VM per container | its `VirtualMachineManager` protocol has two backends, Virtualization.framework and Cloud Hypervisor: a direct precedent for Choir's backend seam | Swift, container-oriented |
| Hyper-V / WSL2 (HCS) | Windows hypervisor | the Windows desktop path (a Choir guest as a WSL2 distro or an HCS VM) | Windows only; different snapshot model |
| Hyperlight (Microsoft) | micro-VM with no guest kernel, embedded in the host app (KVM, MSHV, Windows WHP) | millisecond, hardware-isolated function calls: a candidate for cells | no OS inside; code must target its guest API |
| WASM (wazero 1.12, pure Go; Wasmtime 36 LTS / 50 rc) | language-level sandbox, capability imports (WASI) | a memory-safe, deny-by-default sandbox for cells, in-process, no cgo with wazero | cells are Go source run by Yaegi today; WASM needs a compile step per cell, and "publication is source plus pinned inputs; no cross-tenant binary execution" means a module must be built locally from source |

## 3. Comparison table

| | Firecracker 1.15.1 (1.17.0) | Cloud Hypervisor v54 | gVisor 20261005 | libkrun 1.19 |
|---|---|---|---|---|
| isolation | KVM, minimal VMM | KVM/MSHV, general VMM | user-space kernel + seccomp | KVM/HVF, VMM is in the caller's process |
| language | Rust | Rust | Go | Rust (C API) |
| second line | jailer (unused by Choir), seccomp | seccomp, Landlock; bring your own uid/cgroup/netns | seccomp, namespaces, gofer split | none: host must namespace it |
| 2026 advisories | 2 (1 high) | 2 high (+1 older) | 2 high (CVE records) | none published |
| memory snapshot | Full; Diff (preview); File/UFFD backends | yes; sparse; UFFD on-demand; CoW restore; offload daemon | checkpoint/restore of the sandbox | no |
| live migration | no | yes, n-2 compatible from v54 | restore elsewhere (cold) | no |
| memory elasticity | balloon + FPR, virtio-mem | balloon + FPR, virtio-mem, ACPI hotplug | n/a (host process) | balloon FPR only |
| block discard | 1.17.0, opt-in | yes | n/a | not documented |
| virtio-fs / DAX | no | virtio-fs; DAX unstable | n/a (gofer) | virtio-fs, unconfined |
| virtio-pmem | yes | yes | n/a | no |
| VFIO / GPU | no | yes (iommufd, migration v2) | nvproxy | virtio-gpu (venus, native context) |
| nested virt | no | yes (x86) | not needed (systrap) | no |
| SMT mitigation knob | no (host-level only) | core scheduling per VM | no | no |
| hosts | Linux x86_64/aarch64 | Linux x86_64/aarch64/riscv64 | Linux x86_64/arm64 | Linux, macOS arm64 |
| boot / overhead | <=125 ms to init; <=5 MiB VMM (spec) | similar order, unmeasured | fast start; syscall cost | fast; unmeasured |
| runs the Choir computer as is | yes | yes (guest variant from microvm.nix) | no | no snapshots; would lose S3 |

## 4. Node B: stay, move, or both

### Recommendation

**Stay on Firecracker for production computers; build a backend
interface; add Cloud Hypervisor as a second backend, used first on the
red-team host, then for the specific jobs only it can do.** Do not move the
fleet.

### What Cloud Hypervisor would give Node B (concrete)

| gain | value on Node B today | value later |
|---|---|---|
| copy-on-write restore | none for hibernation (each computer's snapshot is its own) | high for S9: N ephemeral forks of one parent snapshot share its pages; fork boot is a restore, not a boot. Also a shared "warm seed" for new computers if identity can be re-keyed after VMGenID |
| live migration | none: one host, and SP's failover is host *loss*, where the source is gone | medium: planned evacuation between Node B and a same-CPU host (SP doc calls Node C "near-identical") for kernel upgrades without hibernating busy computers; consistent with the rule that deploys never restart a busy computer |
| UFFD on-demand restore + offload daemon | low (Firecracker has a UFFD backend; Choir would write the page server, lever L18) | medium: an off-the-shelf snapshot stream that can compress and encrypt (SP3 cold tier) |
| VFIO / GPU | none (no GPU in Node B) | real if Choir ever offers GPU computers; Fly.io chose Cloud Hypervisor for exactly this |
| virtio-fs | low: the erofs store drive plus `data.img` works; virtio-fs adds a virtiofsd per VM and its own CVE class | possible host-shared read-only store; but DAX is unstable and the density report already prefers virtio-pmem + erofs DAX (L10) |
| core scheduling | medium: Node B is 6c/12t; SMT siblings shared across tenants today | high on the EPYC: avoids the "SMT off halves busy capacity" trade |
| discard | Firecracker 1.17.0 now has it | — |
| nested virt | none | lets a computer host its own VMs (capsule-as-microVM); also needed if the red team wants VMs inside a target |

### What it would cost

- **Engineering.** Backend interface (S to M), Cloud Hypervisor backend
  (M: config translation, API client, snapshot and restore, process
  discovery, console), guest variant from microvm.nix plus a kernel config
  check for ACPI (S), snapshot identity and budget rework (M), staging
  proof (M). All red class (VM lifecycle, snapshot validity). Estimate:
  three to five weeks for one engineer-agent with staging access.
- **Snapshot format.** Not portable between the two VMMs, and not
  portable across most versions of either. A computer changes backend only
  from *stopped* (cold boot). A hibernated computer that is moved would
  lose its snapshot, which under the restart rule is a crash boot: open
  obligations close as "interrupted by a restart". So the switch must
  happen only for stopped or idle computers, or carry a planned-restart
  marker.
- **Density.** Unmeasured. The density report's levers were measured on
  Firecracker. Any Cloud Hypervisor density claim needs the same benchmark
  (idle ladder, hibernate/wake, busy ladder) on the red-team host first.
- **Surface.** A larger device model, an API socket that is always on, and
  a threat model that trusts snapshot files and migration peers. Both mean
  the snapshot directory and any migration listener are protected
  surfaces.
- **Two backends to keep green.** Every S3 and LR change would need proof
  on both, or a declared "Firecracker only" capability flag.

### Do these now regardless of backend

1. **Upgrade Firecracker 1.15.1 to 1.17.0** (red: VM lifecycle). Gains:
   discard for `data.img` (density L8 without offline zerofree), THP
   option, vsock and virtio-mem fixes. It invalidates every snapshot
   (format bump in 1.16.2), so pair it with L20 or an announced window.
2. **Per-VM confinement** (red: vmctl, VM lifecycle): run each VMM under
   its own uid, mount namespace with only its own state dir, its own
   cgroup (density L17), and no inherited capabilities. For Firecracker
   that is the jailer; for any other VMM a small wrapper does the same
   (systemd transient scope with `User=`, `PrivateMounts=`,
   `CapabilityBoundingSet=`, `DeviceAllow=/dev/kvm`, or a Go helper using
   the same namespace code the capsule uses). This is backend-neutral and
   is the single highest-value isolation change on the list.

## 5. The minimal backend seam in vmmanager

vmctl does not need to change. Its `VMManager` interface
(`internal/vmctl/ownership.go:249`) and the optional `snapshotResumer`,
`snapshotDiscarder` and `vmProcessController` interfaces stay as they
are. The seam goes inside `vmmanager.Manager`, which keeps everything
hypervisor-neutral (ownership of state dirs, data and credential disks,
tap and iptables, epochs, health probes, boot timelines, snapshot budget)
and delegates only the VMM-specific parts.

Firecracker-specific code today: `buildFirecrackerConfig`,
`firecrackerBalloonConfig`, `firecrackerLaunchArgs` and
`firecrackerRestoreArgs`, `startFirecrackerProcess`, `fcapi.go` (pause,
snapshot, load), `firecrackerBuildVersion`, `launchConfigHash`, and the
cmdline matchers (`firecrackerPIDsForVM`, `firecrackerCmdlineBytesMatchVM`,
`LiveFirecrackerVMs`).

```go
// MachineSpec is the backend-neutral description of one realization.
type MachineSpec struct {
    VMID, ComputerID          string
    Kernel, Initrd, Cmdline   string
    VCPUs, MemMiB             int
    Drives                    []Drive   // id, host path, read-only, discard
    Net                       NetIface  // tap name, MAC
    Balloon                   *Balloon  // free page reporting, initial size
    ConsolePath               string
    Confinement               Confinement // uid, cgroup path, state dir
}

type Backend interface {
    Name() string    // "firecracker", "cloud-hypervisor", "qemu", ...
    Version() string // pinned into snapshot identity
    Caps() Caps      // DiffSnapshot, CoWRestore, LiveMigrate, Discard,
                     // VirtioMem, Pmem, VFIO, CoreSched
    Launch(ctx context.Context, spec MachineSpec, stateDir string) (Proc, error)
    Pause(ctx context.Context, p Proc) error
    Snapshot(ctx context.Context, p Proc, dir string) (SnapshotMeta, error)
    Restore(ctx context.Context, spec MachineSpec, dir string) (Proc, error)
    OwnsProcess(cmdline []string, vmID string) bool
}
```

Rules that keep it small and safe:
- The backend is a property of the computer's launch config, recorded in
  `fc-config.json`'s successor and in snapshot metadata. Restore refuses a
  snapshot whose backend name or version differs (the existing S3 identity
  check, generalized). A backend change is a cold boot from stopped.
- Capabilities, not type switches: S3, LR and S9 code asks `Caps()`; a
  backend without `CoWRestore` makes S9 forks cold boot, a desktop backend
  without snapshots makes hibernate a stop.
- One registry of live VMMs across backends (process reconcile must see a
  Cloud Hypervisor process as well as a Firecracker one).
- The first commit is a refactor with Firecracker as the only backend and
  zero behavior change (proof: the existing vmmanager tests plus one
  staging boot, hibernate and wake). Cloud Hypervisor lands as a second
  commit, enabled only on hosts that set it.
- Rollback: the refactor reverts as one commit; the second backend is off
  unless a host's config names it.

The desktop (section 6) is a third consumer of the same interface. Keep
`Launch`/`Restore` free of tap/iptables assumptions by putting networking
behind `NetIface`, so a macOS backend can supply vmnet or a user-mode
network instead.

## 6. Desktop and self-hosting (for the parallel survey)

- Linux self-host: the same two server backends (Firecracker, Cloud
  Hypervisor); KVM required.
- macOS desktop: Virtualization.framework (through vfkit or a small Swift
  helper; save/restore on arm64, macOS 14+) or libkrun/krunkit (HVF; no
  snapshots). microvm.nix already builds vfkit guests.
- Windows desktop: WSL2 or Hyper-V (HCS); QEMU with WHPX as a fallback;
  crosvm also runs on Windows.
- Implication: snapshots, discard, live migration and CoW restore are
  optional capabilities. Cold boot and the persistent `data.img` are the
  only things every backend must do. The Wails app should consume the same
  `Backend` interface, not a separate one.

## 7. Red-team design for station LT

### Where it runs

- **Never on Node B, never with production data.** Owner open decision 3
  in the roadmap ("whether LT's first target set includes the live
  paper's host services or only forks") should be answered "forks only"
  for the first campaign.
- **A dedicated, wipeable bare-metal host** of the same CPU family as the
  production target (Intel for Node B parity; an EPYC once that is
  production). Not node-a (another project's root; no plaintext Choir
  data there), and not the SP standby (it will hold plaintext replicas).
  Reimage between campaigns from the NixOS config; keep only exported
  findings and JSONL evidence.
- **Targets are synthetic computers** minted from the production image
  and seeded with fixture tapes, and S9 ephemeral forks of a synthetic
  "victim" computer. Forks of real user computers wait for S9's re-keying
  and data-class classification.
- **No real credentials.** A stub gateway and stub provider keys; egress
  from the red-team host goes through an allowlisted proxy so a successful
  exfiltration lands in a sink, not the internet.

### Target matrix

| target | runtime | what is tested |
|---|---|---|
| T1 Choir computer, production shape | Firecracker 1.15.1, no jailer, root vmctl (today) | assumed-breach baseline: what does code in the VMM's context reach? |
| T2 Choir computer, confined | Firecracker 1.17.0 + jailer/wrapper, per-VM cgroup | the same experiments; the delta is the value of section 4's item 2 |
| T3 Choir computer | Cloud Hypervisor v54 + seccomp + Landlock + wrapper, core scheduling on | device-model escape, snapshot/migration trust, CoW fork isolation |
| T4 capsule children | namespace capsule (today) vs runsc (systrap) inside the guest | capsule-to-guest-root escape, egress from capsule, Go build compatibility and cost |
| T5 host tools | fetchers and Chromium under runsc vs plain processes | malicious page or document to host compromise |
| T6 libkrun VM | krunvm/libkrun with TSI and virtio-fs, inside a host netns and uid | host containment when the VMM is the guest's peer; TSI egress policy |
| T7 (optional) QEMU microvm | QEMU | large-surface baseline for the fuzzers |

### Experiments

| id | experiment | targets | pass condition |
|---|---|---|---|
| E1 | **Assumed breach of the VMM**: run attacker code with the VMM process's uid, capabilities, mounts and cgroup (no exploit needed) and list what it can read, write, signal or ptrace | T1, T2, T3, T6 | T2/T3/T6: only its own state dir, tap and `/dev/kvm`; no other `data.img`, no vmctl state, no `CAP_SYS_PTRACE` over siblings. T1 is expected to fail; that is the finding |
| E2 | **Guest-root device fuzzing**: virtio queue registers after activation (the CVE-2026-5747 class), descriptor-chain reuse (the Cloud Hypervisor UAF class), balloon stats, vsock flood, PCI config space | T1 to T3, T6, T7 | VMM crash at worst, contained to that VM; host and siblings healthy |
| E3 | **Capsule escape**: from a capsule child to guest root or to the runtime's credential; Yaegi kernel escapes | T4 | no escape; tokens absent; compare namespace vs runsc |
| E4 | **Egress policy**: from guest, capsule and tool reach host-internal ports, sibling taps, link-local, IPv6, DNS exfiltration, raw sockets; under libkrun TSI the VMM's netns | T1 to T6 | only allowlisted destinations; host services reachable only with the guest's own scoped authority (S0 problem doc) |
| E5 | **Resource exhaustion**: CPU spin, memory balloon refusal, virtio-mem unplug refusal, disk fill of a sparse `data.img`, snapshot-write storms, fork bombs in capsules | T1 to T4 | per-VM cgroups bound each attacker; sibling wake latency and PSI stay under the density benchmark's safety stops |
| E6 | **Cross-tenant side channels**: SMT (MDS/L1TF class on Coffee Lake), shared host page cache (store image, CoW restore, virtio-pmem DAX L10), KSM if ever on | T2 vs T3 (core scheduling on/off) | measured leakage rate per configuration; feeds the SMT and L10 decisions |
| E7 | **Snapshot and fork integrity**: tamper a snapshot or state file, restore twice (forbidden by S9), fork without re-keying, RNG duplication without VMGenID, QCOW header written into a raw image | T1 to T3 | tampering refused or contained; second restore refused; forks hold fresh identity; raw images never parsed as QCOW |
| E8 | **Management plane**: reach the API socket from a guest or capsule, a Cloud Hypervisor migration listener, vmctl endpoints | T1 to T3 | sockets reachable only by vmctl; listener absent or mTLS |

Findings become problem docs and go through the normal gates, as the
roadmap says. E1 needs no exploit and no new runtime, so it can run
first, on the existing Firecracker shape, as soon as the host exists.

### Prerequisites

1. The red-team host (SP-adjacent purchase; a rented bare-metal box is
   enough).
2. The backend seam (section 5) and the per-VM confinement wrapper.
3. S9 fork re-keying for fork targets; synthetic computers until then.
4. A runsc build in the guest image behind a capsule flag (T4), and on
   the host for T5.

## 8. Proposed order

1. File the problem doc for "VMMs run as root with inherited capabilities
   and every computer's state visible" (green), then fix it with the
   jailer or a wrapper plus per-VM cgroups (red; pairs with density L17).
2. Firecracker 1.17.0 upgrade with L20 or an announced snapshot-invalidating
   window (red).
3. Backend seam refactor, Firecracker only, no behavior change (red,
   small).
4. Cloud Hypervisor backend on the red-team host; run the density
   benchmark there for both backends.
5. LT campaign 1: E1, E4, E5 on T1/T2 (no new runtime needed), then E2,
   E6, E7 with T3.
6. Decide per job, with numbers: CoW forks for S9 on Cloud Hypervisor;
   live migration only when a same-CPU second host exists; GPU only when
   a product needs it.
7. Capsules under runsc (T4) if E3 shows the namespace capsule is the
   weak link, or Go build cost under runsc is acceptable.

## Residuals and confidence

- No runtime was run for this report. Cloud Hypervisor's per-VM overhead,
  gVisor's cost on Go builds and libkrun's boot time are unmeasured.
- The vmctl-as-root reading is from the unit file (no `User=`); confirm on
  Node B with `/proc/<firecracker pid>/status` before filing.
- Whether Cloud Hypervisor's sparse snapshot (v52) skips untouched pages
  for anonymous private guest memory, or only for file/memfd backing, is
  not established here. If only the latter, it does not replace the Diff
  snapshot lever (L3) without `shared=on`.
- gVisor CVE details come from CVE records (OpenCVE), not a gVisor
  advisory page.
- Cross-vendor (Intel to AMD) restore or migration is not supported by
  either VMM; SP host choices decide whether live migration is usable.

## Sources

- Firecracker releases (v1.17.0, v1.16.2, v1.16.0): https://github.com/firecracker-microvm/firecracker/releases
- Firecracker CVE-2026-5747 (virtio-PCI OOB write): https://github.com/firecracker-microvm/firecracker/security/advisories/GHSA-776c-mpj7-jm3r
- Firecracker advisories (incl. CVE-2026-1386, jailer): https://github.com/firecracker-microvm/firecracker/security/advisories
- Firecracker specification (boot, overhead): https://github.com/firecracker-microvm/firecracker/blob/main/SPECIFICATION.md
- Firecracker design (device model): https://github.com/firecracker-microvm/firecracker/blob/main/docs/design.md
- Firecracker snapshot support (Diff preview, versioning, VMGenID): https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/snapshot-support.md
- Firecracker jailer: https://github.com/firecracker-microvm/firecracker/blob/main/docs/jailer.md
- Cloud Hypervisor releases (v54.0, v53.0, v52.0): https://github.com/cloud-hypervisor/cloud-hypervisor/releases
- Cloud Hypervisor advisories: https://github.com/cloud-hypervisor/cloud-hypervisor/security/advisories (GHSA-f47p-p25q-83rh, GHSA-jmr4-g2hv-mjj6, GHSA-g6mw-f26h-4jgp)
- Cloud Hypervisor snapshot/restore (UFFD, CoW restore, offload, VMGenID): https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/snapshot_restore.md
- Cloud Hypervisor threat model: https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/threat-model.md
- Cloud Hypervisor live migration (n-2 compatibility): https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/live_migration.md
- Cloud Hypervisor memory, hotplug, balloon, fs, landlock, seccomp: https://github.com/cloud-hypervisor/cloud-hypervisor/tree/main/docs
- gVisor platforms: https://gvisor.dev/docs/architecture_guide/platforms/
- gVisor security model: https://gvisor.dev/docs/architecture_guide/security/
- gVisor checkpoint/restore: https://gvisor.dev/docs/user_guide/checkpoint_restore/
- gVisor filesystem snapshots: https://gvisor.dev/docs/user_guide/fs_snapshot/
- gVisor releases: https://github.com/google/gvisor/releases
- CVE-2026-96812 (gVisor gofer, CUSE): https://app.opencve.io/cve/CVE-2026-96812
- CVE-2026-95702 (gVisor VFS double free): https://app.opencve.io/cve/CVE-2026-95702
- libkrun README (devices, security model, TSI): https://github.com/containers/libkrun
- libkrun virtio-fs escape issue: https://github.com/containers/libkrun/issues/329
- crun krun handler CVE-2025-24965: https://www.sentinelone.com/vulnerability-database/cve-2025-24965/
- muvm: https://github.com/AsahiLinux/muvm ; krunvm: https://github.com/containers/krunvm ; krunkit: https://github.com/containers/krunkit
- Kata Containers 4.0.0 (runtime-rs default) and advisories: https://github.com/kata-containers/kata-containers/releases ; https://github.com/kata-containers/kata-containers/security/advisories
- CVE-2026-47243 (Kata runtime-rs virtiofs escape): https://nvd.nist.gov/vuln/detail/CVE-2026-47243
- crosvm architecture (per-device jailed processes): https://crosvm.dev/book/architecture/overview.html ; https://chromium.googlesource.com/crosvm/crosvm/+/refs/heads/main/README.md
- QEMU microvm machine type: https://www.qemu.org/docs/master/system/i386/microvm.html
- microvm.nix (eight hypervisors): https://github.com/microvm-nix/microvm.nix
- Apple Virtualization save/restore (macOS 14): https://developer.apple.com/documentation/virtualization/vzvirtualmachine/restoremachinestatefrom(url:completionhandler:)
- Apple Containerization (VZ and Cloud Hypervisor backends): https://github.com/apple/containerization ; https://github.com/apple/container
- Fly.io on GPUs and Cloud Hypervisor: https://fly.io/blog/wrong-about-gpu/ ; https://fly.io/docs/gpus/getting-started-gpus/
- Hyperlight: https://github.com/hyperlight-dev/hyperlight
- wazero: https://github.com/tetratelabs/wazero ; Wasmtime: https://github.com/bytecodealliance/wasmtime
