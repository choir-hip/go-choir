# A computer freezes on its first console write after the vmctl that launched it exits (2026-10-10)

Owner-approved read of the owner's computer (2026-10-10 16:1xZ: "Read the
logs on my computer! You are approved. Let's learn from this failure").

## What the owner saw

The owner's computer (`candidate-fleet-e15cb89f…`, epoch 1188, launched
12:19:49Z) stopped answering at 15:51:07Z. The process stayed alive, the
guest kernel still accepted TCP on 8085, and every HTTP path (even a 404
path) hung. The wedge watchdog does not act on a guest that accepts TCP,
and vmctl's health waited on it, which failed the 17b97b8f deploy
([vmctl health problem](vmctl-health-fate-shares-a-hung-guest-2026-10-10.md)).

## Evidence (Node B, read-only)

- `/proc/4018312/task/*`: vCPU 0 sits in `futex_do_wait` with its system
  time frozen (5908 ticks across samples); vCPU 1 spins; vCPUs 2 and 3 idle.
  The syscall is `FUTEX_WAIT_BITSET_PRIVATE` with value 2, a contended Rust
  `Mutex`.
- `eu-stack -p 4018312`, vCPU 0, innermost last:
  `Vcpu::running` → `BusDeviceSync::write` (holds the serial device
  `Mutex`) → `vm_superio::serial::Serial::write` → `SerialOut::flush` →
  `StdoutRaw::write` → `write(2)` → **SIGPIPE** →
  `vmm::signal_handler::sigpipe_handler` → `Logger::log` →
  `panic_already_borrowed` → panic hook → `Logger::log` →
  `Mutex::lock_contended`. The thread deadlocked on its own logger lock
  inside a signal handler, while holding the serial device lock.
- Firecracker's stdout (`fd 1`) is `pipe:[288060835]`, `O_WRONLY|O_NONBLOCK`,
  and no other process holds the pipe. `console.log` stopped at 12:29.
  Firecracker's stderr is a live journald stream; journald did not restart;
  no SIGPIPE was ever logged, so this was the first SIGPIPE.
- vmctl launches firecracker with `cmd.Stdout = rotatingConsoleWriter`
  (`internal/vmmanager/manager.go`), so os/exec makes a pipe whose read end
  is a goroutine inside the launching vmctl process. That process (started
  12:19) was replaced at 13:18. From then on the pipe had no reader. The
  guest writes to its serial console rarely; its first console write after
  13:18 came when the host NixOS switch restarted the platform services at
  15:50–15:51 (the guest logged the disconnect), raising SIGPIPE.

## Cause

Every guest whose launching vmctl process has exited has a console pipe
with no reader. Its next serial write raises SIGPIPE in firecracker 1.15.1,
whose SIGPIPE handler can deadlock the vCPU thread (not async-signal-safe
logging). That vCPU never runs again; the guest kernel and the Go runtime
stall behind it. This is the "console pipe fate-sharing" residual from the
vmctl 360 review, now with a mechanism. It plausibly explains earlier
"stranded after vmctl restart" and "degraded after forced reboot" events.

At 16:2xZ a second live guest, `vm-48bc09812f3fce06d715f6c7f8e2db9a`
(launched 10:07:57), also had no reader on its console pipe.

## Fix

1. Firecracker writes its console straight to the console log file (an
   `O_APPEND` file descriptor, not a pipe), so no reader is needed and no
   SIGPIPE can occur. vmctl bounds the file by copy-and-truncate.
2. On reattach, vmctl drains a live guest's console pipe by opening
   `/proc/<pid>/fd/1`, so guests launched before the fix get a reader again
   and keep their console log.
3. The owner's computer cannot recover: vCPU 0 is deadlocked on itself.
   Only stopping and booting it recovers it; that is the owner's call.

Detection residual: the wedge watchdog misses a guest that accepts TCP but
whose vCPU is deadlocked. Mutation class red (vmctl); rollback is git revert.

## Landed (16:25Z)

86770ae7 deployed (CI 38067001852). On reattach vmctl logged "console pipe
has a reader again" for the owner's computer and for vm-48bc; each
firecracker stdout pipe now has one reader. choir.news health `ok`, vmctl
health 1 ms, owner computer epoch 1188, no destruction. New launches write
their console to a file. The owner's computer stays frozen until the owner
decides to stop and boot it (vCPU 0 is deadlocked on itself). Legacy pipe
guests still have a seconds-long readerless gap during each vmctl restart
until they next boot.
