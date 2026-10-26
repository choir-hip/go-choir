# A Diff snapshot fails under the jailer: mincore returns EPERM (2026-10-26)

Found by FP0 during D5's staging proof (train 18, 0db9ee271, D5 step 1: Diff
for d@e only). Class: red (VM lifecycle), not yet fixed.

## Evidence

d@e (computer-59c902c5…, vm-70070cf7…) was cold booted under the new unit
by the deploy refresh at 18:46:11 (3cb626bb6), idle from then, and the idle
sweep hibernated it at 18:58:

```
18:58:05 vmmanager: Diff snapshot of VM vm-70070cf7… failed (firecracker API PUT /snapshot/create:
  status 400: Create snapshot error: Cannot get dirty bitmap: Error calling mincore:
  Operation not permitted (os error 1)); taking Full in the same pause
18:58:15 vmmanager: hibernated VM vm-70070cf7… by snapshot (fence=4ms pause=0ms snapshot=8893ms
  bytes=8590024293 disk_bytes=646328320 disk_bytes_method=btrfs_extents snapshot_type=Full
  diff_refused=diff_create_failed)
```

The fallback worked as designed: a Full snapshot in the same pause, no lost
work. The wake at 18:58:48 restored it in `load=10ms wake_to_healthy=307ms`
(CLI wall 0.70 s): a snapshot that survives is already a sub-second wake.

Every Firecracker on Node B now runs under the jailer (SEC-04 waves 1 and 2a,
landed 10-25/10-26): `NoNewPrivs: 1`, `Seccomp: 2`, `Seccomp_filters: 6`,
`CapEff: 0`. The L3a proof that motivated D5 (10-13) predates the jailer.

## Hypotheses (not yet tested)

1. Firecracker's seccomp filter for the thread that serves `/snapshot/create`
   does not allow `mincore`, with an errno action rather than kill.
2. A filter the jailer or our unit adds (the six filters) denies it.
3. The kernel refuses mincore on this mapping for an unprivileged process
   (a capability-dependent mincore rule on newer kernels).

## Consequence

D5 delivers nothing while the jailer is on: every hibernate writes Full
(8 GiB logical, ~9 s). Do not widen VM_SNAPSHOT_DIFF_ONLY. D2 is unaffected.

## Cause (found 2026-10-26, about 19:50)

Hypothesis 2, in a form none of the three named: the denial came from the
vmctl unit's own systemd `SystemCallFilter` (`@system-service @privileged
@mount`, `SystemCallErrorNumber=EPERM`), which has no `mincore`. Firecracker
inherits it from vmctl. The jailer and Firecracker's own filter were not the
cause. Fix 13011657a adds `mincore` to the unit's filter; on the live unit
`systemctl show` lists it after train 20 deployed.

## Proof (2026-10-26)

d@e was cold-started at 20:12:23 under the fixed unit, then went idle.

```
20:24:02 vmmanager: hibernated VM vm-70070cf7… by snapshot (fence=4ms pause=0ms
  snapshot=1598ms bytes=8590024293 disk_bytes=503521280 disk_bytes_method=btrfs_extents
  snapshot_type=Diff diff_refused=)
20:26:13 vmmanager: resumed VM vm-70070cf7… from snapshot (load=4ms wake_to_healthy=287ms
  credential=delivered)
```

The snapshot took 1.6 s instead of about 9 s and holds 503 MB on disk. The
wake took 287 ms to healthy, with a 0.56 s CLI round trip. D5 step 1 is
proved for d@e; step 2 (every test computer, `class:test`) lands with this
record.

## Status

Fixed and proved on d@e. Step 2 widens Diff to test computers; the paper
and the owner's computers stay on Full until a separate decision.
