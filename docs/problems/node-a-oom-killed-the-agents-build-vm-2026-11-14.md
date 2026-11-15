# Node A ran out of memory and the kernel killed the agents build VM

2026-11-14T06:42Z. Recorded 06:44Z.

## Problem

Node A's kernel killed the hypervisor process of the agents build VM in a
global out-of-memory kill. The VM restarted by itself within thirty seconds.
A landing train (late take plus the run-memory retirement census) was two
minutes into its suites on that VM and died with it. Nothing was pushed, so
main is unaffected; the train has to run again.

## Evidence

- Node A kernel log, 06:42:32Z: `oom-kill: constraint=CONSTRAINT_NONE ...
  global_oom, task_memcg=.../microvm@agents.service`, killed
  `cloud-hyperviso` with 10.47 GB resident in shared memory.
- `scripts/land` on the workstation: the VM closed the connection, "the run
  on choir-agents died without an EXIT line".
- Node A has 31.9 GB and no swap. Configured guest memory: agents 12.5 GB,
  runner 12.5 GB, ws-owner 7.5 GB, ws-03 4 GB, ws-04 4 GB: about 40 GB.
- Resident at 06:43Z, after the kill: runner 10.5 GB, ws-owner 6.3 GB,
  agents 5.1 GB one minute after boot, ws-03 and ws-04 0.5 GB each; 6.6 GB
  available on the host.

## Belief

The memory overcommit (owner ruling N5 of the store-once goal) is safe only
while the build VMs are not both full. A guest never gives page cache back
to the host, so each build VM grows to its configured size during a suite
and stays there. For the first time tonight a landing ran on each build VM
at once while ws-owner held 6 GB, and the sum passed the host. The kernel
picks the largest process; it chose a build VM this time, and with other
numbers it can choose the runner mid-landing. The two assigned user VMs
(ws-03, ws-04) are small and so unlikely victims, but nothing guarantees it.

## What I am doing now

No heavy suite on the agents VM until this is repaired. Landings and
builder suites go through the runner only, one at a time. This halves the
landing rate.

## Remaining error and options

Not repaired. Options, for a deliberate Node A change (test, fresh login,
switch): a memory balloon or free-page reporting so idle guests return
cache; zram or disk swap on the host so the kill becomes slowness;
`OOMScoreAdjust` so the user VMs are never chosen and a build VM always is;
smaller build VMs. The protection for ws-03 and ws-04 is the one that
should not wait.
