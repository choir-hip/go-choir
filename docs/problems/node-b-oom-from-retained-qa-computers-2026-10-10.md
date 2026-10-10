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

## Response

- M11 reruns are paused until memory is back above about 8 GiB available.
- Proposed: stop the five confirmed QA computers through vmctl's internal
  stop (`/internal/vmctl/stop`, `user_id` plus desktop `primary`, disk
  state kept). This is an operational action on staging, so it waits for
  the owner's go-ahead.

## Residuals

- `qa-computer-reaper`: probes and suites should stop their disposable
  computer at exit, or vmctl should bound QA computers.
- `vmctl-orphan-reap`: a stopped vmctl left a Firecracker process
  running for six days.
- `vmctl-sigbus-under-oom`: vmctl crashed with SIGBUS under global OOM
  instead of shedding load.
