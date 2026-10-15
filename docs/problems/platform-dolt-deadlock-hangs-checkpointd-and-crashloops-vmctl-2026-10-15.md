# Platform Dolt deadlock hangs checkpointd, blocks the deploy switch and crash-loops vmctl (2026-10-15)

Status: documented. Recovered by restarting platform Dolt and vmctl by hand; not fixed. Mutation class of the fix: orange (a unit
timeout and a bootstrap deadline). The service touches checkpoints, a
protected surface.

## Evidence

- P4's deploy (CI run 38717028854, push 97e023100) stayed in "Deploy to
  Staging (Node B)" for over 30 minutes. On Node B,
  `switch-to-configuration switch` (pid 1644414) had been running 29 min,
  and the only systemd job was the start of `go-choir-checkpointd.service`
  (job 12094915), activating since 10:10:15 UTC.
- The unit is `Type=oneshot` with `TimeoutStartSec=8h` and is started by a
  timer. A normal run takes about 90 ms; journal over 3 days: 3,813 starts,
  most consuming 86–96 ms.
- At 10:09:15 the switch stopped the previous invocation with SIGTERM.
  At 10:10:15 the timer started a new one, and the switch then waited on
  that start job.
- The hung process (pid 1644584) used 244 ms CPU in 28 min and held open
  connections to 127.0.0.1:13306 and :13307. Its SIGQUIT stack at
  10:38 UTC:
  `platform.OpenStore → (*Store).Bootstrap → (*DB).ExecContext →
  mysql.(*mysqlConn).exec → readResultSetHeaderPacket → readWithTimeout`.
  The statement is 0x165f (5,727) bytes, which is a schema DDL. The context
  is `context.Background()`, so there is no deadline.
- After SIGQUIT ended the process, the switch finished within seconds.
- Separately, at 10:05:06 a checkpointd invocation from the previous
  build (exec 1c0c9f…) panicked with a nil-pointer dereference in
  `dolt env.(*DoltCliConfig).createLocalConfigAt`, after
  `mkdir /root/.dolt: read-only file system`. Not investigated.

## Root cause found (10:44–10:48 UTC): the platform Dolt server was deadlocked

- `go-choir-platform-dolt` (pid 1404751, up since 2026-10-14 21:35) was
  accepting connections but not answering queries. It sat at 0% CPU, in
  state S, with no read/write I/O over 5 s. Its last non-connection log
  line was at 09:49:02 (`error running query … nothing to commit`, for
  connection 843).
- vmctl's paper health checks have failed since 09:54. After the switch
  restarted vmctl, vmctl exited every 30 s with
  `ping ComputerVersion route database: context deadline exceeded`. It
  restarted 10 times, hit `start-limit-hit` and stayed failed. Staging
  `/health` still answered 200 the whole time.
- P4's deploy (run 38717028854) failed. The switch exited with status 4
  because checkpointd failed after the operator SIGQUIT, and the deploy's
  vmctl health checks could not pass.
- Dolt ignored SIGQUIT, so there is no goroutine dump. `systemctl restart`
  timed out in stop-sigterm after 90 s and SIGKILLed it (10:45:56 →
  10:47:27). The new Dolt answered immediately: the paper's `/health`
  returned 200 at 10:47:35. `systemctl reset-failed` and `start` brought
  vmctl back at 10:47:44 with 0 restarts, reattaching 3 VMs (the paper,
  vm-48bc…, candidate-fleet-e15c…).

## Belief

The checkpointd hang is a symptom. Its bootstrap DDL waited on the
deadlocked server. Nothing bounds it on the client side, and the unit's 8h start
timeout makes a timer-triggered oneshot able to hold any `nixos switch` for
up to 8 hours. That is a fate-sharing defect: a maintenance tick can block
the whole deploy.

## Remaining error

- What deadlocked platform Dolt at about 09:49. Candidates: a transaction
  left open by a client killed during the 09:4x deploy (P5, 2eae77de6), a
  Dolt-internal lock, or a commit/GC interaction. No stack was captured;
  next time, use `kill -ABRT` (Go dumps goroutines on SIGABRT) before
  restarting.
- Nothing monitors platform Dolt liveness. A query-level probe would have
  caught this at 09:50 instead of 10:44.
- vmctl exits when the database ping times out. A crash loop with a
  start limit leaves the host with no VM lifecycle and needs a manual
  `reset-failed`.

- Which server-side lock or stall the DDL waited on was not captured.
- Whether `Bootstrap` should run on every 90 ms maintenance tick at all
  (it re-applies the schema DDL each time).
- The 10:05 panic in the old build.

## Candidate repairs (not chosen)

1. Bound the unit: `TimeoutStartSec` of a few minutes for the oneshot.
2. Give `OpenStore`/`Bootstrap` a deadline (and use the signal context).
3. Don't bootstrap the schema from checkpointd; it is a reader/maintainer,
   and the owning service bootstraps.

## Root cause investigation

Full write-up, with the Node B journal evidence, the 3-week Dolt clustering
assessment, monitoring proposals and the recommendation:
[dolt-clustering-assessment-2026-10-15](dolt-clustering-assessment-2026-10-15.md).
Summary: root cause **not confirmed** (no stack; best hypothesis is a stall in
Dolt's online maintenance path, GC and conjoin, against 24 GB of retained
history). Two corrections to this doc: the server degraded from 09:23 (a
conjoin plus auto GC at 09:23:05, then `checkpointd` runs of 4.5 minutes) and
stopped answering about 10:05 (last `ConnectionClosed` 10:05:06, first
`DOLT_COMMIT` timeout started 10:04:57), not at 09:49; and SIGQUIT was
received (three header lines were printed) but the dump stalled. The Dolt
problems cluster on unbounded history plus unobserved online maintenance.
Recommended first steps: a query-level probe that captures a stack with
SIGABRT, bounded client deadlines, and ending history retention on Store A.
