# D2 proved: a snapshot survives a deploy that moves the guest release (2026-10-26/27)

Station D2 of
[choir-consolidate-and-scale-metamission-2026-10-26.md](../definitions/choir-consolidate-and-scale-metamission-2026-10-26.md).
Before D2, every deploy voided every snapshot, and every wake after a
deploy was a 10 s cold boot. D2 has been switched on since train 26b
(4831fe67d, deployed 22:46 UTC on 10-26).

## The run (d@e, computer-59c902c5…, VM vm-70070cf7…)

All times are UTC, read from the vmctl journal on Node B and the CLI.

1. **23:44:11, hibernate.** d@e was idle on guest release
   `pbfga1kzhr5pim4rl9rizd5byd142nkj`. It hibernated by snapshot: Diff,
   2462 ms, 8.59 GB logical, 1.60 GB on disk (btrfs extents).
2. **23:46, deploy.** Train 29 (cba66873b) was pushed; it changes the
   guest boot closure. CI passed and staging served cba66873b at
   00:03:05. vmctl health then reported current guest release
   `jw4flh6l47qayf8s8hb34rbjzjralz1i`, with d@e on
   `pbfga1kz…, behind=true`.
3. **00:03:07, wake.** A plain bearer read
   (`GET /api/runtime/observability`, d@e's key) to the hibernated
   computer answered 200 in 1307 ms. That is the bearer-wake fix from
   train 28 (f6bd05924); before it, this read got a 502. vmmanager logged
   the resume from snapshot: load 9 ms, `wake_to_healthy=1042ms`, the
   credential delivered.
4. **00:05:53 to 00:06:05, planned refresh.** About 2 m 46 s after the
   wake, the deferred-refresh sweep refreshed d@e onto the new release:
   `deploy refresh caller=retry-sweep ... target=cba66873b...
   outcome=refreshed planned_restart=true busy_allowed=false`
   (new epoch 13332, 12 s).

## What this proves, and what it does not

**Proved.** A snapshot taken on release N resumes after release N+1
deploys. It resumes in about one second, against 10 s for a cold boot. The
computer then moves to N+1 by a planned, marker-backed refresh while idle.

**Not proved:**
- **Under one second.** 1042 ms misses the one-second target by 42 ms. The
  earlier D5 wakes, with no deploy in between, took 265 to 287 ms. The
  difference is not explained yet (hypothesis: a cold page cache for the
  retained release's files after the deploy).
- **Load.** One computer was measured, not five (that is D7).

**Also seen.** b@c woke at 23:01:58 with `launch_config_mismatch` and cold
booted in 12 s. Its snapshot predates the D2 switch, so this is expected
and is not a D2 failure.

## Next

- Move `ciGateEpochCheck` from `observe` to `enforce`. The observe deploys
  since train 26b were clean (trains 28 and 29 both passed the epoch
  check).
- Run D7: five computers in a row.
- Investigate the 1042 ms against 280 ms.
