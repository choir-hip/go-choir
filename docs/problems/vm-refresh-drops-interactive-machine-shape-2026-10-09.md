# VM refresh drops the interactive machine shape

Date: 2026-10-09. Status: open. Mutation class of a fix: red (vmctl / VM
lifecycle). Found during the owner compaction window
([`guest-store-history-bloat-and-memory-shape-2026-10-09`](guest-store-history-bloat-and-memory-shape-2026-10-09.md)).

## Evidence

The owner computer `computer-03335285…` is `interactive` /
`premium_always_on`. Before the window it ran at 16 GiB (14.7 GB RSS at
02:46; boot at 01:02 via the resolve path).

After hold → stop → `maintenance-serve` → unhold → `POST
/internal/vmctl/refresh`, its `fc-config.json` reads:

    {"mem_size_mib":4096,"vcpu_count":2}

That is the manager default (`VM_MEM_MIB=4096`, 2 vCPU), not the
interactive shape. vmctl's environment sets `VM_INTERACTIVE_MEM_MIB=8192`;
the untracked `vmctl-priority.env` overrides it to `16384`. Computers
booted the same night through the resolve path read `[4,16384]`.

## Cause (verified in source)

1. vmctl's boot configs carry `MachineCPUCount`/`MachineMemSizeMib`
   from `machineShapeForOwnership` → `interactiveMachineShape()`
   (`internal/vmctl/ownership.go:1182`).
2. `Manager.RefreshVMWithConfig` (`internal/vmmanager/manager.go`) calls
   `refreshConfigForCurrentDeploy(mergeVMConfigOverrides(inst.Config, overrides), m.cfg)`.
3. `refreshConfigForCurrentDeploy` zeroes the machine shape **after** the
   caller's override is merged. `bootVM` then falls back to the manager
   default.

The zeroing came from d3a391c0 (2026-06-30). It was meant to drop a
*stale stored* shape when `VM_MEM_MIB` changes between deploys, but it
also drops the shape the caller just asked for.
`TestMergeVMConfigOverridesFillsDeployRefreshIdentity` runs the same
sequence and never checks the shape afterwards.

## Consequence

Every `refresh` boots an interactive computer at 2 vCPU / 4 GiB. That
includes the deploy's active-VM refresh. Every other boot path gives it
the interactive shape. So a computer's shape depends on which path last
booted it. For an always-on computer, that is usually the deploy refresh.

Memory sizing claims made from `VM_INTERACTIVE_MEM_MIB` were therefore
not reliable. The owner's 16 GiB held only until the next refresh.

## Fix direction

Clear the stale **stored** shape first, then apply the caller's
overrides: `mergeVMConfigOverrides(refreshConfigForCurrentDeploy(stored), overrides)`.
A refresh without a shape override still picks up current defaults.
