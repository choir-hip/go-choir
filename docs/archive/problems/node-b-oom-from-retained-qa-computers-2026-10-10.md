# Node B OOM from retained QA computers (2026-10-10)

Status: open. Mutation class of any response: red (vmctl lifecycle on
staging).

## What happened (06:00–06:06Z)

- About five minutes after the c013d99b deploy, `choir.news` and Node B
  SSH stopped answering. TCP connected, but SSH never sent its banner.
  Two M11 rerun 10 attempts failed at their first page load (05:55Z
  reload timeout, 06:00Z `ERR_TIMED_OUT`).
- Kernel global OOM at 06:05:54Z. Killed: `nix-daemon`, a user
  `systemd`, `go-choir-checkpointd`, and an **unmanaged** Firecracker
  process (pid 1117847). That process had been left over in the vmctl
  cgroup since 2026-10-04 13:32, when a vmctl stop did not reap it.
  `go-choir-vmctl` then died with SIGBUS (core dumped). It restarted at
  06:06:00 (NRestarts=1).
- After the restart, vmctl reattached the owner computer's VM at
  06:06:50, with guest health 200. It skipped reattach for six QA VMs
  whose guest health failed.
- At 06:07Z: 31 GiB total, about 0 available. 12 Firecracker processes,
  about 28 GiB RSS. Load average 50–60, falling.

## Cause (finding)

Disposable QA computers are never stopped. Every M11 rerun and Texture
suite run leaves its computer running (2.5–3.8 GiB each). An M11
computer that stalls in `materializing` stays busy indefinitely. Five
VMs are confirmed this session's by user id in committed receipts:

- rerun 4: `vm-480d590c`
- suite run of 22:03Z: `vm-e2504f4a`
- rerun 7: `vm-1a9c51e0`
- rerun 8: `vm-419eeec3`
- rerun 9: `vm-fc88c237`

Together they hold about 14.8 GiB. Four more VMs from 2026-10-09 are not
attributable from receipts.

**Fate-sharing (standing question):** QA probes share one host with the
owner computer, with no admission bound. Tonight's probe volume was
enough to OOM the host and crash vmctl. Only the reattach path kept the
owner computer alive.

## Response (owner-authorized, 06:23–06:27Z)

- vmctl `/internal/vmctl/stop` returned `{"status":"stopped"}` for all
  five QA users, but every Firecracker process kept running. Those VMs
  were not in vmctl's registry, so the stop only cleared the ownership
  record (a false "stopped").
- Meanwhile the owner computer failed health checks from 06:24:55. At
  06:26:50 vmctl rebooted it (`epoch=1186`, new address 10.200.28.2), and
  its health is 200 since then. Under the restart rule that was a crash
  restart, so its open work was closed as interrupted.
- The five QA Firecracker processes were sent SIGTERM by exact VM id,
  with their disks kept. Available memory went from about 0.25 GiB to
  17.8 GiB, and was 14.5 GiB at 06:28Z.

## Why pressure reclaim did not prevent it (trace)

Reclaim is enabled on Node B (`VMCTL_PRESSURE_RECLAIM_MODE=active`,
floor 4096 MiB / 15%, min idle 30m), along with the 30-minute idle
sweeper. From 04:30Z to 06:06Z it logged
`active=2 eligible=0 protected=2 pressure=true` 35 times. It saw the
pressure, but its registry held only two ownerships, both protected. The
other ten running VMs were invisible to it. They had dropped out of the
registry when vmctl restarted on deploys and "reattach skipped … guest
health check failed" left them running unmanaged. Reclaim, the idle
sweeper and stop all act on the registry only, so the substrate fault is
that **vmctl's registry does not account for every Firecracker process
it started.**

## Proposed vmctl upgrade (red; not started)

1. Reconcile process and registry. A VM whose reattach fails is stopped,
   not left running. A periodic sweep finds Firecracker processes in the
   vmctl cgroup that are not in the registry and terminates them, keeping
   their disk state. Unmanaged VMs then cannot accumulate.
2. Admission control. Refuse to boot a new computer when the host is
   under pressure (return 503, with retry after), so probes cannot push a
   pressured host over.
3. Stop tells the truth. It succeeds only once the process has exited.
4. OOM priority by class. Set `oom_score_adj` per VM: owner and premium
   negative, ephemeral QA positive. Make vmctl itself negative, so the
   kernel kills a QA VM before the owner computer or vmctl.
5. An ephemeral class for QA computers. Probe accounts get a TTL, and
   their `guest_busy` protection is capped, so a QA computer stuck in
   `materializing` cannot hold memory forever. Probes also stop their
   computer at exit.

## Residuals

- `qa-computer-reaper`: probes and suites should stop their disposable
  computer at exit, or vmctl should bound QA computers.
- `vmctl-orphan-reap`: a stopped vmctl left a Firecracker process
  running for six days.
- `vmctl-sigbus-under-oom`: vmctl crashed with SIGBUS under global OOM
  instead of shedding load.

## Fix: vmctl host-capacity guards (owner-approved 2026-10-10)

Red ceremony (vmctl VM lifecycle on staging).

- Conjecture delta: host capacity is safe only if vmctl's registry
  accounts for every Firecracker process it started, refuses new computers
  below the memory floor, and ranks VMs for the kernel OOM killer. The
  existing reclaim was correct, but it was blind.
- Changes (`internal/vmctl/host_capacity.go`,
  `internal/vmmanager/process_control.go`):
  1. `ReconcileVMProcesses` runs first in every sweep (every 2 minutes).
     - It scans every Firecracker process on the host and gives each
       tracked VM its OOM priority: -400 for protected computers, +500
       for ordinary ones.
     - It retries reattach for processes behind a stopped ownership.
       After 10 minutes (`VMCTL_UNMANAGED_REAP_GRACE`) it powers them off,
       keeping their disk state, unless the computer is premium,
       critical, platform or held. Those are never reaped.
     - A process with no ownership at all is powered off on its second
       sighting.
  2. Memory admission. A new computer, or a start that adds a VM, is
     refused with 503 / Retry-After (`host_memory_pressure`) when
     available memory minus the VM size would fall below the reclaim
     floor. Protected computers, and recovery of a tracked VM, are always
     admitted.
  3. Stop and logout power off an unmanaged process, and stop fails if
     the process survives. No more false "stopped".
  4. OOM order. vmctl goes from -500 to -900. Every VM launches at +500
     instead of inheriting vmctl's protection, and reconcile lowers
     protected computers to -400.
  5. Busy cap. `VMCTL_PRESSURE_MAX_BUSY_PROTECT=2h` applies to proof
     accounts only (retention's `example.com` / `example.test` and the
     proof user prefixes). Real users' computers keep busy protection
     however long their work runs.
- Tests first (`host_capacity_test.go`, `process_control_test.go`): the
  failure modes are listed at the top of the test file. They cover reap
  only after grace, never reap protected, reattach on recovery, orphan
  reaped on second sight, managed VM untouched, a truthful stop,
  admission refusing below the floor but admitting premium, OOM priority
  by class, and the busy cap for proof computers only.
- Protected surfaces: vmctl VM lifecycle (reattach, stop, admission),
  host OOM policy.
- Admissible evidence: after deploy, the vmctl log shows
  `process reconcile`, live VMs have the expected `oom_score_adj`, and
  `free` stays above the floor through an M11 rerun.
- Rollback: git revert, plus restoring the nix OOMScoreAdjust.
  Powered-off VMs keep their state and resume on the next request.
- Heresy delta:
  - Discovered "registry blind to unmanaged VM processes" and "stop
    reports success for a running VM".
  - Repaired both on staging proof only.
  - Introduced none.
- Residual `probe-self-stop`: auth logout does not stop the computer, so
  probes cannot stop their own. Reconcile, idle hibernation and the busy
  cap now bound them instead.

## Staging proof (cf0969cf, deployed 06:58Z)

- `go-choir-vmctl` OOMScoreAdjust=-900 (MainPID `oom_score_adj` -900).
- 06:58:33: the owner computer was reattached. Reconcile set its VM to
  `oom_score_adj` -400. The six untracked VMs failed reattach again and
  entered the 10-minute grace.
- 07:10:28: `process reconcile live=8 managed=2 reattached=0 reaped=6
  pending=0 protected_unmanaged=0`. Each VM was logged as "powered off
  unmanaged VM … state kept". Two Firecracker processes remain: the owner
  computer and M11 rerun 10. Available memory went from 0.25 GiB (06:23Z)
  to 23.9 GiB.
- Heresy "registry blind to unmanaged VM processes": **repaired**
  (staging proof above). The truthful stop, memory admission and the
  proof-account busy cap are deployed but have not yet been exercised on
  staging.

## Second owner-computer reboot (07:06Z)

After the cf0969cf deploy, vmctl reattached the owner computer at
06:58:33 with health OK. From 06:59:43 its health checks failed. At
07:05:37 vmctl marked it "unhealthy on resolve; recovering before
routing", and at 07:06:17 rebooted it (epoch 1187, new address). That is
a crash restart under the owner rule: open work was closed, the second
time this morning (after 06:26:50).

- Hypothesis: host memory pressure. Six unmanaged QA VMs were still
  running until reconcile reaped them at 07:10:28, and rerun 10's
  computer booted at 07:00:57.
- Not established: whether the guest itself was busy, for example in
  boot replay.
- It has been healthy since, on cf0969cf.

Residual `owner-recovery-reboot`: recovery of a protected computer
reboots it without trying anything gentler.
