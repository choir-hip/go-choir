# Recover-current stale manager and warmness race

Date: 2026-08-23
Mutation class: red

## Problem

A stopped ownership can still have a running Firecracker instance in the VM manager after warmness policy or a manager restart. `StopVMForDesktop` only called `vmManager.StopVM` for ownership states `active` or `degraded`, so a stale `stopped` record did not fence the actual running VM before `data.img` quarantine/swap. The premium always-on warmness loop can also select the stopped ownership again during the long staging/replay window.

## Evidence

During staging recovery of `computer-03335285269bdba4f94377e56879f9e6`, ownership reported `stopped` while Firecracker for the same VMID remained running. The recovery journal reached `swapped`; guest startup then logged `store: open ... fresh=false` and repeatedly failed `computer event projection repair required` while the old/current realization was still being managed. The product request held the vmctl path until timeout, and the expected `state=active`/`phase=done` transition never occurred.

The VM manager process and ownership state therefore disagreed at the exact protected boundary where a file-level tape recovery must fence the old realization.

## Repair boundary

`StopVMForDesktop` must stop any manager-tracked instance for the ownership VMID regardless of stale ownership state. `recover_current` must mark the ownership as recovery-in-progress before staging, and premium warmness must skip that marker until recovery success or an explicit retry clears it. The recovery marker does not alter canonical events or permit historical rewind.
