# Pressure Reclaim Hibernated a User's Computer, Which Then Stayed Down 5 Minutes

Date: 2026-10-14. Found by the load and density measurements (raw data and
summary outside the repo, in the session scratchpad `perf/load_density_*`).
Status: documented; no fix yet. Mutation class of any fix: red (vmctl).

## What happened

| time (UTC) | event |
|---|---|
| 11:40:47 | vmctl's memory-pressure reclaim hibernated a real user's primary computer (computer-7cc96dc6…, vm-48bc). No test computer existed yet. |
| 11:43 | The user's computer woke; its snapshot was rejected with `launch_config_mismatch` (the 11:45 goproxy deploy had changed the launch config) and it fell back to a cold boot. |
| 11:43-11:52 | The computer refused connections for about 5 minutes until vmctl stopped it and booted it again (11:52:16). Healthy since. |

Pressure at 11:40 (PSI `some avg10` above vmctl's reclaim threshold of 1.0)
came from ordinary host work at the same moment: two `du` scans of
/var/lib/go-choir and vm-state (one by the measurement agent, one by the
artifact-store investigation), and two `nix build`s of the deploy toplevel.
No guest needed the memory.

Two more reclaims followed during the measurements: an 8 GiB snapshot write
(hibernating one test computer) pushed PSI to 2.5 and vmctl hibernated a
second test computer, so one hibernate caused the next.

## Why (from the measurements; hypotheses marked)

1. **The trigger is pressure, not need.** MemFree on the host is about
   0.25 GiB; the rest of the free memory is page cache. Any burst of I/O or a
   guest page fault goes through direct reclaim, so PSI crosses 1.0 while
   12+ GiB is still available. The reclaim then frees nothing that was
   short.
2. **Reclaim can cascade:** a hibernate writes 8 GiB (snapshots are the full
   guest size on disk), which raises PSI, which triggers the next reclaim.
3. **A wake after a deploy that changed the launch config is a cold boot,
   and that cold boot wedged** (refused connections for 5 minutes).
   Hypothesis: the failed snapshot restore left state the cold boot tripped
   on; vmctl's health check took 5 minutes to give up. Not yet confirmed
   from vmctl's logs.
4. **Admission reserves the full 8 GiB guest** (needs 12.67 GiB available
   per boot or wake) while an idle guest uses 1.2 GiB, so a returning
   user's wake can be refused with two test computers up.

## Rules for now (no code)

- No host-wide `du` or similar scans on Node B while users are active; use
  btrfs quota numbers or a scoped, `ionice -c3 nice` scan.
- Measurements that boot test computers stop at PSI 1.0, not 5 (vmctl's
  threshold), so they never cause a reclaim of someone else's computer.
- Before demos and other owner-visible windows: no host deploys that change
  the guest launch config, and no load tests.

## Fix directions (to design, red class)

- Reclaim on a sustained, meaningful signal (PSI over a longer window plus
  low MemAvailable), never on a short I/O burst; never reclaim a computer
  that had user activity in the last N minutes.
- Rate-limit reclaim to one hibernate in flight, and exclude the snapshot
  write's own pressure.
- A snapshot rejected for `launch_config_mismatch` should cold-boot within
  the normal 10 s; find why this one wedged (vmctl journal 11:43-11:52).
- Admission by measured guest use, not the full guest size.
