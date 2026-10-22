# A wake that races a deploy's vmctl restart boots an orphan VM; the caller gets 502 and the computer ends stopped by wedged (2026-10-22)

Found 07:17 UTC by the self-development track monitor, during the freeze
close's deployed proof. Test computer only: computer-71afbf14… (vm-c5c801f8…,
class test, owner batching-e2e-1022). Mutation class of this record:
green. A fix would be red (vmctl lifecycle).

## Evidence (vmctl journal on Node B, read-only; the monitor's own act log)

- 07:09:42. Staging's `x-choir-build-commit` is 4fca1e01b (deploy G). The
  monitor treated that as "deployed" and woke the hibernated test computer.
- 07:10:15. `POST /api/computers/<id>/lifecycle/start` with
  `{"idempotency_key":"fp4-freeze-close-wake-20261022T0710"}`, test key.
  The answer was **502 `{"error":"lifecycle actuation failed"}`**. In the
  old vmctl (pid 4045150):
  - 07:10:15 `admission decision=admit … need_mib=1024 available_mib=10231`
  - 07:10:16 `snapshot resume … not used (disk_identity_mismatch)`; cold boot follows
  - 07:10:17 `gateway token request failed for vm-c5c801f8…: … 127.0.0.1:8084: connect: connection refused` (the gateway was restarting for the deploy)
  - 07:10:17 `cold boot of VM vm-c5c801f8… after snapshot fallback`
- 07:10:50 to 07:11:00. systemd stops and starts go-choir-vmctl (deploy G
  activation). It finds left-over firecracker 4099850 in the cgroup and
  ignores it.
- 07:11:01. The new vmctl (pid 4100215): `route-authorized reattach adopted
  4 managed VM(s)`. Those are the delegate's, the paper's, 7cc96dc6's and
  the owner's. **Not vm-c5c801f8.** The test computer's ownership row
  still read `state: hibernated, stopped_by: idle`.
- 07:11:04, 07:13:15, 07:15:15. `VM vm-c5c801f8… runs outside the registry
  for 0s / 2m11s / 4m11s; reattach pending`.
- 07:17:15. `killing orphaned Firecracker process … (pid=4099850)`,
  `destructive kill … cause=reap-unmanaged:orphan epoch=0`, `wedged VM …
  refused connections for 5m0s; stopped, it boots again on resume`. The
  row is now `state: stopped, stopped_by: wedged`.

## What it shows (hypotheses until read in code)

- H1. The boot claim was in the old vmctl's memory only. The row never
  left `hibernated`, and the process ran with `epoch=0`. So the restart
  had nothing durable to reattach. Standing question 8: what a restart
  erases.
- H2. The wake booted with no gateway token, because the gateway was down
  for the same deploy. That is the shape Argon raised for FP3's rescue
  ("RecoverVM with the old boot config and no gateway token"). Not checked
  here: the guest never answered.
- H3. The orphan held host memory for about 7 minutes (07:10:17 to
  07:17:15) while admission counted it nowhere.
- The caller's 502 is honest, but the computer then ends `stopped by
  wedged`, a fate the owner did not cause. Per FP3 group 1 that needs an
  owner act or a new deploy target to boot again.

## The monitor's own error

`x-choir-build-commit` turns over when the web tier restarts. That is
before vmctl and the gateway restart, and before CI's refresh round
(07:11:02 to 07:12:43 here). A wake between those points races the
deploy. From now on the monitor waits for CI's `deploy refresh
caller=deploy` round before any lifecycle act after a deploy.

## Next safe probe

Wake again with an owner act now that vmctl is stable. That is the
designed exit from `stopped_by: wedged`. Then read the boot and build
identity.
