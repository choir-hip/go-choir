# VM Backend Interface: Firecracker and Cloud Hypervisor Behind One Seam

Date: 2026-10-14, about 21:00 UTC. Base: `origin/main` at `980ffaf4e`.

Owner: "Firecracker and Cloud Hypervisor would be the two real production
candidates."

Inputs: [isolation-runtimes-2026-10-14](../research/isolation-runtimes-2026-10-14.md)
(section 5, the seam), [computer-isolation-security-review-2026-10-14](computer-isolation-security-review-2026-10-14.md)
(section 5.3, one confinement contract; section 6, jailer prior art),
[self-hosting-and-desktop-2026-10-14](self-hosting-and-desktop-2026-10-14.md)
(section 3.3, what leaks past the seam) and
[vm-density-limits-2026-10-14](vm-density-limits-2026-10-14.md).

## 0. Status, scope and class

**What landed.** A new package, `internal/vmmanager/vmbackend`, that nothing
imports yet:

- the `Backend` interface, its capability flags and optional interfaces;
- a Firecracker backend whose config is byte-identical to today's
  `buildFirecrackerConfig` (golden test);
- a Cloud Hypervisor backend for cold boot, stop and metrics;
- the per-computer launch record and the switch and selection rules;
- one process registry across backends;
- conformance tests that run against a fake backend on any host and
  against real binaries on a KVM host (Linux, opt-in).

**What did not land, and why.** The lead changed the plan mid-task. Two
other agents are changing the same `vmmanager` and `vmctl` code right now:
the Firecracker jailer slice, and admission by measured use with
sustained-pressure reclaim. So this pass does not move or refactor any
Firecracker code, and does not edit any existing `vmmanager`, `vmctl` or
Nix file. Moving vmmanager onto the seam is the migration in section 9. It
lands after both of those branches.

**Mutation class of this change:** yellow. It is new code with no caller,
plus tests. Product behaviour does not change, and no protected surface is
touched. The `vmctl` binary does not import the package. The deploy
classifier is path-based, so it still classifies the change as a vmctl
deploy (section 12). Every migration step in section 9 is red (VM
lifecycle).

## 1. Files

| file | what it holds |
|---|---|
| `backend.go` | `Name`, `Caps`, `MachineSpec`, `Drive`, `NetIface`, `Balloon`, `LaunchPlan`, `HostPath`, `Handle`, `Metrics`, `Backend`, `Snapshotter`, `MemoryController` |
| `registry.go` | `Registry`: `Identify`, `LiveVMs`, `PIDsForVM` across backends; `ScanProc`, `SplitCmdline` |
| `selection.go` | `LaunchRecord` (`vm-backend.json`), `CheckSwitch`, `Policy` (`VM_BACKENDS_ENABLED`) |
| `process.go` | `Start(plan)`, `Process.Done/ExitErr/Stop`, `ProcMetrics` (from `/proc`) |
| `apiclient.go` | HTTP over a unix socket, shared by both VMM APIs |
| `firecracker.go` | `FirecrackerBackend`, including `Snapshotter` (pause, Full snapshot, File-backend load) |
| `cloudhypervisor.go` | `CloudHypervisorBackend`; event-monitor decoding (`EndsRealization`) |
| `tap_linux.go`, `tap_other.go` | `OpenTap`: open an existing tap as `IFF_TAP\|IFF_NO_PI\|IFF_VNET_HDR`, to pass it as a file descriptor (fd) |
| `*_test.go` | the conformance suite, golden, argument, selection and registry tests, and `real_linux_test.go` |

## 2. The interface

```go
type Name string // "firecracker" (Default), "cloud-hypervisor"; durable strings

type Backend interface {
    Name() Name
    Caps() Caps
    Version(ctx context.Context, binary string) (string, error) // pinned into timelines and snapshot identity
    PlanLaunch(spec MachineSpec) (LaunchPlan, error)             // pure: no I/O
    VMIDFromCmdline(args []string) (vmID string, ok bool)        // /proc/<pid>/cmdline, split on NUL
    Metrics(ctx context.Context, h Handle) (Metrics, error)
}

type Snapshotter interface { // iff Caps().Snapshot
    Pause(ctx context.Context, h Handle) error
    CreateSnapshot(ctx context.Context, h Handle, dir string) (SnapshotFiles, error)
    PlanRestore(spec MachineSpec, dir string) (LaunchPlan, error)
    CompleteRestore(ctx context.Context, h Handle, dir string) error
}

type MemoryController interface { // iff Caps().BalloonResize
    SetBalloonTargetMiB(ctx context.Context, h Handle, mib int) error
}

type MachineSpec struct {
    VMID, ComputerID, StateDir, Binary string
    Kernel, Initrd, Cmdline            string
    VCPUs, MemMiB                      int
    Drives                             []Drive    // guest order is contract: vda store, vdb data, then credential
    Net                                *NetIface  // tap name, MAC, optional open TapFD
    Balloon                            *Balloon
    APISocketPath                      string     // "" = no API (Firecracker --no-api)
}

type LaunchPlan struct {
    Backend      Name
    VMID, Binary string
    Args, Env    []string
    WriteFiles   []PlannedFile // Firecracker's fc-config.json
    ExtraFiles   []*os.File    // child fds 3, 4, ... (Cloud Hypervisor's tap)
    HostPaths    []HostPath    // every host path the VMM opens, with r or rw
    APISocket    string
    ConfigDigest string        // Firecracker: equals vmmanager's launchConfigHash
}

// Start writes WriteFiles, execs, and returns a Process: PID, Done(),
// ExitErr(), Stop(grace) (SIGKILL, then wait).
func Start(plan LaunchPlan, stdout, stderr io.Writer) (*Process, error)
```

Design choices:

- **The backend formats; vmmanager decides.** vmmanager keeps building the
  kernel command line, the drive list, the tap, the /30, the epoch and the
  credential disk exactly as today. `MachineSpec` carries the result, and
  a backend only turns it into argv and files. That is what makes the
  Firecracker golden test possible: the same inputs give the same bytes.
- **`LaunchPlan` is data, not a closure.** Tests can check it without KVM.
  A confinement wrapper can rewrite it before `Start`: add the jailer,
  rewrite paths into a jail, or turn `HostPaths` into Landlock rules
  (section 6).
- **Capabilities, not type switches.** The conformance suite fails a
  backend that reports a capability without its interface, or the other
  way round (F1).
- **Stop is the same for both backends:** SIGKILL. vmmanager writes the
  destruction receipt first, as it does today. Graceful guest shutdown is
  not part of this slice: Firecracker computers are killed today, and the
  guest's journalling owns `data.img` consistency.

## 3. Where today's Firecracker code goes

| today (`internal/vmmanager`) | after migration |
|---|---|
| `buildFirecrackerConfig` (`manager.go:1563`) | splits in two. The neutral half builds `MachineSpec` (cmdline, drives, MAC, tap) and stays. The format half is `FirecrackerConfig` |
| `firecrackerBalloonConfig` (`balloon.go:17`) | `MachineSpec.Balloon` (neutral); format in each backend |
| `launchFirecracker` (`manager.go:1888`), `firecrackerLaunchArgs` (`snapshot.go:290`) | `Backend.PlanLaunch` |
| `startFirecrackerProcess` (`manager.go:1942`) | `vmbackend.Start`, plus vmmanager's console sink, OOM score, PID file and exit watcher (neutral) |
| `firecrackerRestoreArgs` (`snapshot.go:302`), `fcapi.go` | `Snapshotter` |
| `launchConfigHash` (`snapshot.go:315`) | `LaunchPlan.ConfigDigest` |
| `firecrackerBuildVersion` (`boot_timeline.go:182`) | `Backend.Version` |
| `firecrackerCmdlineBytesMatchVM`, `firecrackerPIDsForVM` (`manager.go:2159-2250`), `LiveFirecrackerVMs`, `firecrackerVMIDFromCmdline` (`process_control.go:25-80`) | `Registry.Identify`, `LiveVMs`, `PIDsForVM` |
| `killFirecrackerProcess`, `liveFirecrackerForVMLocked`, `cleanupOrphanedFirecrackerLocked`, `ReapUnmanagedVM`, `SetVMOOMScoreAdj` | stay in vmmanager, renamed without "Firecracker", and ask the `Registry` |
| `pidPath` = `firecracker.pid` | keep the file name (a rename breaks reattach across the deploy); later `vmm.pid` with a fallback read |
| `BootTimeline.FirecrackerBin/FirecrackerVer`, `SnapshotMeta.FirecrackerVersion` | keep the JSON fields; add `vm_backend` / `backend_version`. A missing value means firecracker |
| `IsFirecrackerAvailable` (`config.go:159`, `cmd/vmctl/main.go:133`) | "the default backend's binary is available"; the fail-closed gate is kept |

Already backend-neutral, and unchanged: taps and iptables
(`manager.go:3089-3428`), epochs, health probes, the guest fence and resume
calls, boot timelines, the snapshot budget, metadata and validation, data
and credential disks, receipts and the console file.

## 4. Capability matrix

"Choir" columns show what this code wires. "Upstream" columns show what the
VMM can do.

| capability | Choir FC | Choir CH (this slice) | upstream FC 1.15.1 (pinned) / 1.17.0 | upstream CH 52.0 (pinned) / v54 |
|---|---|---|---|---|
| cold boot | yes | yes | yes | yes |
| full memory snapshot and restore (S3) | yes | **no** (CH computers kill-hibernate) | yes | yes |
| diff snapshot | no | no | developer preview | no (sparse files instead) |
| copy-on-write restore (S9 forks) | no | no | no | yes |
| live migration | no | no | no | yes (mTLS v53, n-2 v54) |
| balloon with free page reporting | yes (`VM_BALLOON_FREE_PAGE_REPORTING`) | args only (`--balloon ...free_page_reporting=on`); unproven | yes | yes |
| balloon resize at run time | no | no | yes | yes (`vm.resize`) |
| virtio-mem | no | no | yes (1.14+) | yes, plus ACPI hotplug |
| virtio-blk discard | no | no | 1.17.0, opt-in | yes |
| tap by fd (no `CAP_NET_ADMIN` in the VMM) | no | **yes** (`--net fd=3`) | no | yes |
| seccomp | yes (default) | **yes** (`--seccomp true`, v52: true\|false\|log) | yes | yes; errno mode v53; main thread filtered from v54 |
| Landlock | no | opt-in (`Landlock: true`) | no | yes |
| core scheduling | no | yes (CH default `core_scheduling=vm` since v52) | no (launcher's job) | yes |
| VMM device counters | no | yes (`GET /api/v1/vm.counters`) | metrics file (unused) | yes |
| raw image pinning | n/a | yes (`image_type=raw` on every disk) | n/a | yes |
| VFIO / GPU | no | no | no | yes |

## 5. Choosing a backend per computer, and changing it only when stopped

**Recording.** The backend is a property of a computer's realization:

- vmctl's ownership record carries the *requested* backend (migration
  step M4). It is a per-computer setting, empty by default.
- vmmanager writes the backend that *actually launched* into
  `<VM_STATE_DIR>/<vmID>/vm-backend.json` at every cold boot
  (`WriteLaunchRecord`: temp file, fsync, rename, dir fsync).
- Snapshot metadata records the backend and its version.
- A computer with no file reads as `firecracker` (`Implicit: true`).
  Firecracker is the only backend that has ever launched a computer.
- A record that cannot be parsed, or that names an unknown backend, is an
  error, never a guess.

**Selection.** `Policy.Select(requested, testComputer)` applies these
rules:

- The host gate is `VM_BACKENDS_ENABLED`. Empty means firecracker only.
- A typo in the list is an error, not a silently disabled backend.
- A non-default backend needs both the host gate and a test computer, that
  is, `warmnessClassForOwnership(...) == WarmnessClassTest` (the test tier
  of 2026-10-13).
- No production computer can be selected onto Cloud Hypervisor until the
  owner widens the rule.

**Switching.** `CheckSwitch(from, to, phase)`:

- Staying on the same backend is always allowed.
- A change is allowed only from `stopped`. It is refused while running,
  pending, hibernated (either mode) or failed.
- A change therefore always means the next launch is a cold boot on the
  new backend: no process to adopt, no snapshot to load.

This fits the restart rule (CLAUDE.md, "Restarts End Work (Crash) Or
Resume It"):

- A snapshot is never portable between VMMs. Changing the backend of a
  hibernated computer would quietly turn its next wake into a crash boot.
  So the change needs an explicit stop first. Stop is an owner or vmctl
  act that already closes open work, and it discards the snapshot
  (`discardStoppedSnapshotLocked`).
- At restore, the existing S3 identity check generalises:
  - a snapshot whose backend name or version differs from the computer's
    current backend is refused (`backend_mismatch`, a new fallback
    reason);
  - a fallback is a cold boot.

## 6. One confinement contract for both VMMs

The jailer slice (another agent's branch) gives Firecracker a per-VM uid
and chroot. This seam is shaped so the same contract
(security review, section 5.3) covers Cloud Hypervisor without a second
design:

- **`LaunchPlan.HostPaths` is the jail's file list.** Each backend declares
  every host path its VMM opens, with `r` or `rw`:
  - `/dev/kvm`, the kernel and initrd;
  - each drive (read-only drives as `r`);
  - the config file and the API socket directory;
  - the event-monitor file;
  - `/dev/net/tun`, only when the tap is attached by name.

  A cage builds bind mounts or hard links from this list. Cloud
  Hypervisor's `--landlock-rules` is built from the same list
  (`landlockRules`). Conformance check F6 makes a missing path a test
  failure, not a boot failure on staging.
- **A wrapper rewrites plans; backends do not know about cages.** The
  intended shape is
  `type Confiner func(LaunchPlan, Confinement) (LaunchPlan, error)`:
  - For Firecracker, the jailer confiner:
    - turns the plan into `jailer --id <vm> --exec-file <firecracker> --uid --gid --chroot-base-dir ... -- <args>`;
    - rewrites `fc-config.json`'s paths to jail-relative paths;
    - moves the API socket into the jail.

    `VMIDFromCmdline` keeps working, because the exec'd process is still
    named `firecracker` and still carries `--id`. **The golden in
    `firecracker_test.go` must then be re-captured from vmmanager in the
    same commit, because every committed snapshot's launch hash changes
    once (F4).**
  - For Cloud Hypervisor, the cage:
    - runs it in a systemd transient unit (`User=`, `RootDirectory=` +
      `BindPaths=` from `HostPaths`, `DeviceAllow=/dev/kvm`, an empty
      capability set);
    - passes the tap as fd 3.

    The Firecracker jailer cannot carry Cloud Hypervisor for two reasons:
    it closes every fd above 2, and it prepends `--id`, which Cloud
    Hypervisor rejects (security review, section 6.1).
- **Inner sandbox per VMM.** Firecracker uses its default seccomp. Cloud
  Hypervisor uses:
  - `--seccomp true`;
  - Landlock, once proven on the test host;
  - `core_scheduling=vm` (its default);
  - `image_type=raw` on every disk.
- **Post-launch verifier, the same for both.** It reads
  `/proc/<pid>/{status,cgroup,ns/*,root}` and compares them with the
  `Confinement`. A mismatch kills the VMM, and the boot fails closed.
- **This slice:** Cloud Hypervisor runs as the same user as Firecracker
  does today (vmctl's root). This is no worse for networking, because CH
  gets the tap as an fd. It is worse for the device-model surface (a
  larger VMM with no outer cage), which is why it is test-computers-only
  and lands after the jailer.

## 7. Failure modes (written before the code; each has a test)

| id | failure | consequence | test |
|---|---|---|---|
| F1 | a capability is reported but not implemented, or the reverse | S3 or LR code calls an operation the backend cannot serve | `runConformance` "F1"; `TestConformanceCatchesCapsLie` |
| F2 | the process launched from a plan is not recognised by `/proc` identification | a running computer is invisible: boot over it, or `DestroyVMState` deletes live state | conformance "F2"; FC and CH cmdline tests |
| F3 | two backends both claim one process | a reaper kills the wrong VM | `TestRegistryRefusesAmbiguousIdentity` (ambiguous means nobody's) |
| F4 | the plan is not deterministic, or Firecracker's JSON drifts from vmmanager's | every snapshot is refused with `launch_config_mismatch`; every wake is a crash boot | conformance "F4"; `TestFirecrackerConfigMatchesVMManagerGolden` (bytes and hash) |
| F5 | an invalid spec reaches exec (an empty VM id or one containing `/`, a relative state dir, no vCPUs, a duplicate drive) | path traversal in state dirs; an undiagnosable VMM error | conformance "F5" |
| F6 | a path the VMM opens is missing from `HostPaths`, or a read-only drive is listed `rw` | the jail or Landlock breaks boot, or exposes the store read-write | conformance "F6"; `TestCloudHypervisorLandlockRules` |
| F7 | an fd named in args is not passed, or a nil fd is passed | no networking, or CH fails | conformance "F7"; `TestCloudHypervisorArgs` |
| F8 | live: the VMM exits on an argument it rejects; it is not found in `/proc`; it reports no metrics; it survives Stop | the flags are wrong for the pinned version, or a leak | conformance "F8", live against `/bin/sh` everywhere and against real binaries in `real_linux_test.go` |
| S1 | a computer with no record reads as anything but Firecracker | it moves backend silently | `TestLaunchRecordMissingIsImplicitFirecracker` |
| S2 | a corrupt, unknown or future record is guessed | a computer boots on a backend it never ran on | `TestLaunchRecordRefusesGuessing` |
| S3 | a backend change outside `stopped` | a snapshot loads on the wrong VMM, or an unrequested crash boot | `TestCheckSwitchOnlyFromStopped` |
| S4 | a non-default backend on a host that never opted in, or for a production computer | Cloud Hypervisor reaches users early | `TestPolicyDefaultIsFirecrackerOnly`, `TestPolicySelectTestComputersOnly` |
| S5 | a typo in `VM_BACKENDS_ENABLED` | a backend silently disabled or enabled | `TestPolicyRefusesTypos` |
| CH1 | **Cloud Hypervisor reboots the guest inside the same process on a guest reset** (checked in the v52.0 source: `EpollDispatch::Reset` calls `vm_reboot`). Firecracker exits instead (`panic=1`, `reboot=k`) | a guest panic becomes a silent reboot with the same epoch and an already-consumed credential envelope. vmmanager never sees a crash restart (VAL-CROSS-117 broken) | `--event-monitor path=<state>/ch-events.json`, plus `CloudHypervisorEvent.EndsRealization` (`TestCloudHypervisorEvents`). vmmanager must kill the VMM on `vm/rebooting` (M5). Defence in depth: the second boot finds the envelope consumed and fails closed |
| CH2 | the tap is opened without `IFF_VNET_HDR`, or with a different multi-queue flag than at creation | corrupt frames, or `EINVAL` | `OpenTap` sets the same flags as Firecracker; checked live only (`CHOIR_VMBACKEND_TAP`) |
| CH3 | a path containing `,` or `=` breaks CH's option parser, so a disk or socket parses wrong | wrong disk attached | `TestCloudHypervisorArgsRefusals` |
| CH4 | the event monitor or console is a pipe to vmctl | SIGPIPE in the VMM after a vmctl restart (the 2026-10-10 deadlock) | both are files by construction |

## 8. Cloud Hypervisor launch, packaging and what was checked

**Packaging (checked).** At the repo's nixpkgs pin (`ec84054698e3`,
`nixos-unstable`), `cloud-hypervisor` is **52.0**, `mainProgram`
`cloud-hypervisor`, next to `firecracker` 1.15.1. v52.0 already contains
the fixes for CVE-2026-45782 (virtio-blk use-after-free, fixed in v52.0)
and CVE-2026-27211 (QCOW2 autodetect, fixed in v51.0). v54 (upstream, the
same day) adds a main-thread seccomp filter and restore without
`CAP_NET_ADMIN`. Neither is needed for a cold-boot slice; take v54 when
nixpkgs has it, before CH runs anything but test computers.

**The proposed host change**, behind a flag with no default change. It
was not applied in this pass: no Nix edits while the jailer branch owns
`node-b.nix`.

```nix
# nix/node-b.nix (sketch)
options.choir.vmBackends.cloudHypervisor.enable = lib.mkEnableOption "Cloud Hypervisor as a test-computer VM backend";
config = lib.mkIf config.choir.vmBackends.cloudHypervisor.enable {
  environment.systemPackages = [ pkgs.cloud-hypervisor ];
  systemd.services.vmctl.serviceConfig.Environment = [
    "VM_CLOUD_HYPERVISOR_BIN=${pkgs.cloud-hypervisor}/bin/cloud-hypervisor"
    "VM_BACKENDS_ENABLED=firecracker,cloud-hypervisor"
  ];
};
```

With the flag off, the host closure is unchanged. Rollback is to unset the
flag: vmctl then refuses to select CH, and a stopped CH test computer cold
boots on Firecracker (`CheckSwitch` allows it from stopped).

**Arguments** (`TestCloudHypervisorArgs`, for a 4-vCPU, 8 GiB computer):

```
cloud-hypervisor
  --api-socket path=<state>/<vm>/api/ch.sock
  --kernel /var/lib/go-choir/guest/vmlinux
  --initramfs /var/lib/go-choir/guest/initrd
  --cmdline "<the exact cmdline vmmanager builds today, incl. vm_id= and epoch=>"
  --cpus boot=4
  --memory size=8192M
  --disk path=<storedisk.erofs>,readonly=on,image_type=raw,id=store
         path=<state>/<vm>/data.img,image_type=raw,id=data
         path=<state>/<vm>/credential.img,image_type=raw,id=credential
  --net fd=3,mac=AA:FC:00:00:00:XX,id=eth0
  --balloon size=0M,deflate_on_oom=on,free_page_reporting=on   (only with VM_BALLOON_FREE_PAGE_REPORTING)
  --serial tty --console off
  --seccomp true
  --event-monitor path=<state>/<vm>/ch-events.json
  [--landlock --landlock-rules path=...,access=r|rw ...]        (opt-in)
```

**Checked against the v52.0 source** (from the pinned nixpkgs `src`; not
run):

- `--api-socket path=`, `--initramfs`, `--console` (default `tty`),
  `--serial` (default `null`; `tty` writes stdout), `--seccomp`
  (`true|false|log`), `--landlock`, `--landlock-rules`
  (`path=...,access=[rw]`) and `--event-monitor path=|fd=` all exist.
- The disk syntax includes `readonly=on`, `id=` and `image_type=raw`.
  Without `image_type`, the image type is `Unknown` and the format is
  probed, which is why it is pinned.
- Net `fd=` parses as an integer list with optional brackets, so `fd=3`
  is valid.
- The balloon syntax is `size,deflate_on_oom,free_page_reporting`. There
  is no stats interval.
- `core_scheduling` defaults to `vm`.
- `/api/v1/vm.counters` and `/api/v1/vmm.ping` exist.
- Events are pretty-printed JSON objects written back to back
  (`DecodeCloudHypervisorEvents` handles that and a truncated tail).

**Not yet verified (needs the KVM host; `real_linux_test.go` covers the
first three):**

1. whether the guest `vmlinux` has a PVH ELF note. CH boots only a PVH
   entry point on x86_64; Firecracker does not need one;
2. whether CH accepts the tap fd opened by `OpenTap`, and whether the
   guest gets link;
3. RSS and metrics on a real CH process;
4. whether the guest's fixed device names (`/dev/vdb` for data, from
   microvm.nix volumes) hold under CH's PCI enumeration. The disk order is
   the same, and Firecracker already runs with `--enable-pci`;
5. whether `reboot=k` and the i8042 arguments are harmless under CH. CH
   has an i8042 reset device on x86_64. With CH1 handled, a reset ends the
   realization either way;
6. whether the guest image needs a `hypervisor = "cloud-hypervisor"`
   variant at all. microvm.nix generates the same kernel, initrd and erofs
   store for both; the runner is unused, because vmmanager launches the
   VMM itself.

## 9. Migration plan (lands after the jailer and admission/reclaim branches)

Each step is one commit, red class, with full ceremony, and goes through
the Landing Loop (push, CI, staging deploy, verify the staging commit,
deployed proof). Every step reverts as one commit.

| step | change | proof | rollback |
|---|---|---|---|
| M0 | Rebase this package onto the merged jailer and admission work. Re-capture the Firecracker golden from post-jailer `buildFirecrackerConfig`. Add the `Confiner` type and make the jailer a `Confiner` | package tests; the golden matches post-jailer vmmanager | drop the commit |
| M1 | **Discovery through `Registry`.** `firecrackerPIDsForVM`, `liveFirecrackerForVMLocked`, `DestroyVMState`'s guards, `LiveFirecrackerVMs` → `LiveVMs`, `ReapUnmanagedVM`, `SetVMOOMScoreAdj`. The Registry holds Firecracker only. **Must precede any CH launch** (F2) | existing vmmanager and vmctl tests unchanged; staging: reattach after a vmctl restart, and the host-capacity reconcile sees every computer | revert |
| M2 | **Launch through `Backend` for Firecracker.** vmmanager builds `MachineSpec`; `FirecrackerBackend.PlanLaunch` plus `Start`. A new equivalence test in vmmanager: `PlanLaunch(spec).WriteFiles[0].Data == json.Marshal(buildFirecrackerConfig(...))` and `ConfigDigest == launchConfigHash(...)`, for kill and snapshot modes. Then delete the old format code | existing tests, plus the equivalence test; staging: cold boot, **snapshot hibernate and wake of a computer hibernated before the deploy** (hash continuity: no `launch_config_mismatch`) | revert; snapshots stay valid in both directions because the bytes are identical |
| M3 | **Snapshot through `Snapshotter`;** delete `fcapi.go`. `WriteLaunchRecord` at every cold boot. `SnapshotMeta` gains `backend` and `backend_version` (missing means firecracker); restore refuses a mismatch (`backend_mismatch`). Boot timelines gain `vm_backend` | S3 tests; staging: hibernate and wake; `vm-backend.json` present after boot | revert; files written by M3 are ignored by older code |
| M4 | **vmctl selection.** The ownership record gets a `vm_backend` field; `Policy` from `VM_BACKENDS_ENABLED`; `CheckSwitch` on every request to change it; an operator API to set it on a stopped test computer; `ParsePolicy` errors fail vmctl start | vmctl tests; staging: setting CH on a running test computer is refused | revert; the field is ignored |
| M5 | **Cloud Hypervisor enabled behind the host flag.** `OpenTap` after `createTapDevice`; the event-monitor watcher kills on `EndsRealization` (CH1); hibernate on a backend without `Caps().Snapshot` takes the kill path with reason `backend_no_snapshot`; the Nix flag from section 8 (off by default); CH computers get the same console file, OOM score and destruction receipts | `real_linux_test.go` on the KVM host with both binaries; unit test for the watcher | unset the flag; stopped CH test computers cold boot on Firecracker |
| M6 | **The staging test plan** (section 10) | section 10 | as M5 |

Not in this plan, each needing its own mission:

- CH snapshot and restore (`vm.snapshot`, `--restore source_url=file://`,
  plus the S3 validation generalised to CH's snapshot directory format);
- CH copy-on-write restore for S9 forks;
- live migration;
- virtio-mem leases (density L11);
- discard (L8).

## 10. Staging test plan: one test computer under Cloud Hypervisor

Preconditions: M1 to M5 on `main`, and the flag set on Node B for the test
only. Staging is the acceptance environment, so this is a Node B change
made through the normal deploy, never by hand.

1. **Pick the computer.** A fresh test-tier computer (a QA owner in
   `TestUserIDs`), never a real user or platform computer. Record its
   `computer_id` and VM id.
2. **Stop it,** then set `vm_backend=cloud-hypervisor` through the M4
   operator API. Expect: accepted from stopped. Setting it while the
   computer runs is refused with `ErrSwitchNotStopped`; record that
   refusal as evidence.
3. **Cold boot.** Expect a healthy guest within `VM_BOOT_READY_TIMEOUT`.
   Collect:
   - the boot timeline with `vm_backend=cloud-hypervisor` and
     `backend_version="cloud-hypervisor v52.0"`;
   - `vm-backend.json`;
   - `/proc/<pid>/cmdline` naming `cloud-hypervisor` and `vm_id=`;
   - `/proc/<pid>/status` showing the same user as the Firecracker VMMs.
4. **Product path.** Log in as the test owner on `https://choir.news` and
   run the standard computer acceptance proof. Checks:
   - desktop loads, an agent run completes, a file write survives;
   - Texture renders;
   - the guest reaches gateway, vmctl, maild, platform and the proxy over
     its tap.
5. **Isolation smoke.** From inside the guest, the tap-to-tap deny and
   anti-spoof rules hold. They are backend-neutral iptables, but the MAC
   and tap now arrive through a different path.
6. **Discovery.** Restart vmctl (a planned restart, with the marker). The
   CH computer is reattached, not reaped. The host-capacity reconcile
   lists it, and `DestroyVMState` on it is refused while it is live (F2).
7. **Metrics.** `Backend.Metrics` returns RSS and `vm.counters`. Compare
   idle RSS with a Firecracker computer of the same shape (the density
   report's idle ladder, one rung only).
8. **Hibernate.** Under `VM_HIBERNATE_MODE=snapshot`, hibernate takes the
   kill path with `backend_no_snapshot`. Wake is a cold boot that closes
   open work as "interrupted by a restart" (the restart rule; expected).
9. **Guest reset (CH1).** Trigger a guest panic on the test computer. The
   watcher kills the VMM on `vm/rebooting`; vmmanager marks the computer
   failed; recovery boots a new epoch. There must be no silent in-process
   reboot (check the event file and the epoch).
10. **Switch back.** Stop it, set `firecracker`, cold boot. The computer
    is healthy on Firecracker, and its data survives (same `data.img`).

Evidence goes to `docs/evidence/vm-backend-ch-staging-<date>/`: timelines,
event file, `/proc` samples, the acceptance artifact and the vmctl log
lines.

## 11. Overlaps with the concurrent branches

**Firecracker jailer (per-VM uid and chroot):**

- **Paths it will likely change:** `startFirecrackerProcess`,
  `firecrackerLaunchArgs`, `firecrackerRestoreArgs`,
  `buildFirecrackerConfig` (jail-relative paths), `prepareAPISocket` and
  `apiSocketPath` (the socket moves into the jail; path length), the
  console sink, `pidPath`, and the cmdline matchers if argv changes.
- **What it means for this package:**
  - the Firecracker golden and `FirecrackerBackend.PlanLaunch` must follow
    in M0;
  - `HostPaths` is meant to be the jailer's file list;
  - `VMIDFromCmdline` must keep matching the jailed process.
- **Snapshot continuity:** the jailer changes `fc-config.json`'s bytes, so
  it invalidates every committed snapshot once (`launch_config_mismatch`,
  then a cold boot). The jailer branch should say so in its own landing
  notes.

**Admission by measured use, and sustained-pressure reclaim:**

- **Paths it will likely change:** `host_capacity.go` (`LiveFirecrackerVMs`
  consumers), `pressure_reclaim.go`, `warmness_policy.go`,
  `SetVMOOMScoreAdj`, and any per-VM RSS reader.
- **What it means for this package:**
  - `ProcMetrics` (VmRSS and CPU from `/proc`) and `Registry.LiveVMs` are
    the backend-neutral versions of what that branch measures;
  - after M1 it should read through them, so a CH computer is measured
    and reclaimable too;
  - until M1 a CH computer would be invisible to admission. That is a
    second reason M1 must precede any CH launch.

**Both branches:** the file list in this commit is disjoint from both
(new directory only), so the merge conflict risk is zero. The semantic
overlap is all in M0 to M2.

## 12. Verification and deploy impact

- `go build ./...` passes.
- `go vet` passes for the package on darwin and on `GOOS=linux`
  (`CGO_ENABLED=0`).
- `go test -race ./internal/vmmanager/vmbackend ./internal/vmmanager ./internal/vmctl`
  passes.
- The conformance suite was mutation-checked: breaking CH's `vm_id`
  parsing fails F2 for both CH configurations.
- `real_linux_test.go` was not run: this host is macOS, without KVM.
- No Nix file was touched, so there was nothing to `nix-instantiate
  --parse`.
- `git diff origin/main --name-only | .github/scripts/deploy-impact-classify`
  classifies `internal/vmmanager/*` as "vmctl host service + canonical
  guest boot closure + active VM refresh". The classifier is path-based;
  the vmctl binary does not import this package, so the actual runtime
  change is nil. Not pushed. Landing it alone would cost a vmctl restart
  and an active-VM refresh for no behaviour, so it should ride with M0 or
  M1.

## 13. Heresy delta, conjecture, residuals

**Heresy delta:**

- *discovered:*
  1. vmmanager has two Firecracker identity rules:
     - `firecrackerCmdlineBytesMatchVM`, used by the boot, destroy and
       kill guards, accepts any argument whose base name contains
       "firecracker";
     - `firecrackerVMIDFromCmdline`, used by `LiveFirecrackerVMs` and the
       host-capacity reconcile, accepts only argv[0].

     A process that the guards see but the reconcile does not is possible
     (for example under a wrapper). `Registry` unifies them on the
     permissive rule.
  2. CH1: Cloud Hypervisor's in-process reboot would hide crash restarts.
  3. Every launch path assumes the VMM is the only identity source;
     nothing would see a second VMM.
- *introduced:* none (no caller).
- *repaired:* none yet (M1 repairs 1 and 3; M5 repairs 2).

**Conjecture delta:** a backend seam can be pure formatting plus
identification, with the whole lifecycle kept neutral in vmmanager. The
evidence so far: Firecracker's config is reproduced byte for byte from a
neutral spec, and CH needs no lifecycle changes for cold boot. It does
need one new neutral duty, the reboot watcher. Refuted if M2's equivalence
test or the staging hibernate-across-deploy check fails.

**Residuals:**

- CH runs unconfined (same user) until the cage lands. Mitigation: test
  computers only, host flag off by default.
- PVH and guest-device questions are unverified (section 8).
- CH density is unmeasured; no density claim is made.
- CH snapshot, copy-on-write restore and migration are not designed here.
- `VM_BACKENDS_ENABLED` and `vm_backend` are names proposed by this
  report; M4 fixes them.

**Next realism axis:** run `real_linux_test.go` on a KVM host with both
binaries (PVH, tap fd, metrics), then M1.
