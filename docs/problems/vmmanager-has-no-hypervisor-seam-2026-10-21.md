# vmmanager has no hypervisor seam, and the recovered seam predates three VMM changes

Date: 2026-10-21. Item: RES-20 in
[the security and density checklist](../security-density-architecture-checklist-2026-10-21.md).
Station: [FP10 RES-20](../definitions/choir-launch-fp10-res-20-2026-10-21.md).
Class of the problem: substrate (VM lifecycle).

## The problem

Every computer launches through Firecracker-specific code in
`internal/vmmanager`, and nothing on `main` lets a second VMM launch,
be found, be measured or be confined. Cloud Hypervisor (CH) is the second
production candidate (owner, 2026-10-14) and the only path to VFIO/GPU
(video app desks), copy-on-write restore for forks, live migration and
vCPU core scheduling. None of that can be tried, even on one test
computer, without a seam.

## Evidence (read on `main` at d1c60ad87)

- Launch: `buildFirecrackerConfig` (`manager.go:1714`),
  `launchFirecracker` (`manager.go:2051`), `startFirecrackerProcess`
  (`manager.go:2150`), `firecrackerLaunchArgs` and
  `firecrackerRestoreArgs` (`snapshot.go:309-333`), `launchConfigHash`
  (`snapshot.go:341`), all write Firecracker's JSON and argv.
- Identity: four `/proc` matchers know only the name `firecracker`
  (`firecrackerCmdlineMatchesVM`, `firecrackerPIDsForVM`,
  `firecrackerCmdlineBytesMatchVM`, `manager.go:2397-2458`;
  `LiveFirecrackerVMs` and `firecrackerVMIDFromCmdline`,
  `process_control.go:25-80`). `VMResidentBytes` (`process_control.go:127`)
  feeds admission from the same scan. A process of another VMM would be
  invisible to boot guards, `DestroyVMState`, the reaper and admission.
- Control API: `fcapi.go` is Firecracker's REST surface (pause, full
  snapshot, load, `PATCH /hotplug/memory`), used by snapshots
  (`snapshot_lifecycle.go`) and the virtio-mem lease path
  (`memory_hotplug.go`).
- Confinement: the jailer slice (`confinement.go`) is built on the
  Firecracker jailer binary (`jailerArgs`, `rewriteFirecrackerConfigForJail`,
  `startJailedFirecracker`, `waitForJailedExec`). CH has no jailer, and the
  Firecracker jailer cannot carry CH (it closes fds above 2 and prepends
  `--id`; security review §6.1).

## The recovered seam does not fit main as it is

`origin/recovered/vmbackend-2026-10-14` (b6f27ded4) holds a `vmbackend`
package with no caller, written against `980ffaf4e` (977 commits before
main). Since then three changes landed or are in flight that the package
does not model:

1. the jailer on main: the Firecracker golden in that package is the
   pre-jailer config, so it no longer matches vmmanager's bytes (its own
   failure mode F4);
2. virtio-mem hotplug on main (`memory_hotplug.go`, Firecracker 1.15.1):
   the package's capability matrix says "Choir FC virtio-mem: no", and its
   `MachineSpec` has no hotplug region, so a migration would drop the
   device;
3. FP8 (`lane/fp8-image-jailer`, not landed): a second Firecracker build
   chosen per computer (`vmm_next.go`, `vmm-build.json`), a per-computer
   selector (`computer_select.go`), data-drive discard by build, and
   shared read-only inodes for jailed VMs (`jail_shared.go`). These are a
   second launch record and a second selection rule next to the package's
   `vm-backend.json` and `VM_BACKENDS_ENABLED`.

Landing the package as it is would put two launch records and two
selection rules on main, and a golden test that fails.

## Not a regression

Nothing is broken on staging. This is a missing capability and a stale
plan. The design that answers it is
[reports/vm-hypervisor-seam-design-2026-10-21.md](../reports/vm-hypervisor-seam-design-2026-10-21.md).
