# A hung checkpointd bootstrap blocks the deploy's NixOS switch (2026-10-15)

Status: documented; not fixed. Mutation class of the fix: orange (a unit
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

## Belief

Hypothesis, not confirmed: the DDL `Exec` waited on a Dolt lock or
server-side stall while the deploy was restarting the services that share
Dolt. Nothing bounds it on the client side, and the unit's 8h start
timeout makes a timer-triggered oneshot able to hold any `nixos switch` for
up to 8 hours. That is a fate-sharing defect: a maintenance tick can block
the whole deploy.

## Remaining error

- Which server-side lock or stall the DDL waited on was not captured.
- Whether `Bootstrap` should run on every 90 ms maintenance tick at all
  (it re-applies the schema DDL each time).
- The 10:05 panic in the old build.

## Candidate repairs (not chosen)

1. Bound the unit: `TimeoutStartSec` of a few minutes for the oneshot.
2. Give `OpenStore`/`Bootstrap` a deadline (and use the signal context).
3. Don't bootstrap the schema from checkpointd; it is a reader/maintainer,
   and the owning service bootstraps.
