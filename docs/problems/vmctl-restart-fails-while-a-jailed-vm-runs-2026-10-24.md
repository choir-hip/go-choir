# vmctl Cannot Restart While a Jailed VM Runs

Date: 2026-10-24, 00:1x UTC. Found by FP0 on staging. The D27 deploy
(f84c8fe34, SEC-04 step 3) restarted vmctl while one jailed VM was alive,
and vmctl stayed down from 23:57:58 to 00:16:20 UTC, about 18 minutes.
Mutation class of any fix: red (vmctl, VM lifecycle, the host's cgroup
tree). Parent record:
[jailed-firecracker-cannot-exec-dynamic-binary-2026-10-23.md](jailed-firecracker-cannot-exec-dynamic-binary-2026-10-23.md).

## Evidence (Node B)

- 23:47:05: D26's static jailer launched the first test computer,
  fp8-jailer-1023 (vm-fa28a867…), jailed: pid 782793 (after the wake),
  cgroup `system.slice/go-choir-vmctl.service/vms/vm-fa28a867…`.
- 23:57:57: the D27 switch stopped vmctl cleanly ("managed VMs left running
  for reattach"; KillMode=process). Four Firecracker processes stayed: the
  jailed one in `vms/vm-fa28a867…`, and three direct ones (the paper
  vm-1669020a…, candidate-fleet-e15cb89f…, vm-36a51fad…) in `supervisor`.
- Each of the 10 starts that followed failed:
  `Failed to spawn executor: Device or resource busy`,
  `Failed to spawn 'start-pre' task: Device or resource busy`,
  `Failed with result 'resources'`. Then `start-limit-hit`. The deploy job
  failed ("Job for go-choir-vmctl.service failed because of unavailable
  resources", deploy evidence /var/lib/go-choir/deploy-failures/40041867841-1.json).
- The cgroup state while vmctl was down (read at 00:1x): the unit root had
  0 processes and `subtree_control` = `cpu pids`; `vms` had `cpu pids`; the
  jailed leaf held 1 process; `supervisor` held 3. The unit has
  `Delegate=yes`, `DelegateSubgroup=supervisor`, `KillMode=process`, an
  `ExecStartPre` list, and systemd 261.3.
- The 23:44:53 restart (the D26 deploy) worked with the jailer on. At that
  time no jailed VM was running (the first test computer was down after
  its failed 23:07 launch).
- While vmctl was down, running guests kept running, but nothing could be
  launched, woken, refreshed or resolved. The paper had failed two vmctl
  health checks at 23:54:48 and 23:57:18, before the outage; that is a
  separate observation. The paper answered /health 200 in 22 to 44 ms at
  00:17.

## Recovery (FP0, 00:16)

1. Stopped the jailed test VM (TERM, pid 782793). It is a disposable test
   computer.
2. Cleared the delegated controllers bottom up: `-cpu -pids` in
   `vms/cgroup.subtree_control`, then in the unit root's. The two `rmdir`
   calls on the now-empty leaf and on `vms` answered EBUSY, and both
   directories remain, empty.
3. `systemctl reset-failed go-choir-vmctl`, then `systemctl start`. vmctl
   came up at 00:16:20 with `jailer=true` and reattached all three direct
   VMs, the paper included (pid 780706, epoch 13269). After the start, the
   unit root's `subtree_control` is empty, and systemd created a `.control`
   subgroup.

## Hypothesis (not confirmed)

systemd spawns ExecStartPre, a control process, into a cgroup inside the
delegated unit. Spawning fails with EBUSY while the unit's cgroup has
domain controllers (`cpu pids`) enabled in `subtree_control`. This is the
cgroup v2 rule that a cgroup giving controllers to children holds no
processes itself. Either systemd tries the unit root, or it tries to change
that `subtree_control` while `vms` has `cpu pids` enabled below. With no
jailed VM alive, `vms` held no process, and the 23:44 restart did not hit
this. That suggests the populated `vms` subtree, or its enabled
controllers, is what matters. Confirm from systemd 261's
delegation/control-process code, and with a reproduction on a disposable
host or in the agents VM, before choosing a fix.

## State now and the conservative step

- No jailed VM is running. The second test computer (fp8-jailer-1023b) was
  never launched. The jailer is rolled back to off (D28) so that no resolve
  can start a jailed VM before the next vmctl restart. That deploy's own
  restart happens with no jailed VM alive, the same condition as the clean
  23:44 restart.
- SEC-04 stays proved for one jailed launch, a hibernate and wake, and the
  breach probe (ledger 23:47 and 23:48). It is not proved for a vmctl
  restart while a jailed VM runs: that check found this problem.

## Fix directions (to evaluate)

1. Put the VMs' cgroups outside the unit: a separate slice, such as
   `choir-vms.slice`, delegated to vmctl. vmctl's own unit then never
   carries `subtree_control` with live children it doesn't own.
2. Keep the VMs under the unit, but have systemd place control processes in
   a subgroup that can never be refused (check what systemd 261 does with
   `DelegateSubgroup` for ExecStartPre), and keep the unit root's
   `subtree_control` stable across restarts.
3. A pre-start step that clears the controllers. Rejected: it cannot run,
   because the spawn of the pre-start task is what fails.

A deploy's vmctl restart must also be proven with a jailed VM alive before
`class:test` widening (SEC-04 step 5).
