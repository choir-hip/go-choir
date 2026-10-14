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

## More found while reading the code (2026-10-14, before the fix)

Read at `980ffaf4e`. No new staging evidence; these are code facts.

1. **One pressure pass can hibernate five computers.** `ReclaimPressureVMs`
   hibernates up to `MaxCandidates` per pass (`VMCTL_PRESSURE_RECLAIM_MAX_CANDIDATES=5`
   on Node B), one after another. Each writes 8 GiB. That multiplies the
   cascade above by up to five per sweep.
2. **The under-capacity keepalive drops on the same burst.** The idle
   sweep keeps idle primary computers warm only while the sample shows no
   reclaim pressure (`warmness_policy.go:225`). One `avg10` burst during a
   sweep hibernates every primary idle past 30 minutes, with no bound.
3. **Two host timers call reclaim, not only the sweep.** The deploy disk
   preflight (`scripts/node-b-deploy-disk-preflight`) and the disk retention
   sweep (`nix/node-b.nix:84`) POST `/internal/vmctl/reclaim`, which runs
   the same single-sample decision. A hibernate frees no root disk (a
   snapshot adds 8 GiB to it), so these calls never needed pressure
   reclaim; they need retention prune. Whether the 11:40 reclaim came from
   the sweep or from a deploy timer is not known (hypothesis; the vmctl
   journal around 11:40 would say).
4. **Nothing measures a computer's memory.** vmctl has no per-VM RSS
   reading, so admission can only assume the full 8 GiB guest.

## Fix design: density quick tier, levers L2 and L1 (red class)

Report: [VM density](../reports/vm-density-limits-2026-10-14.md), order
items 2 and 3.

Protected surfaces: vmctl VM lifecycle (reclaim hibernation, start
admission). Not touched: restart semantics (a crash restart still closes
work; nothing here resumes anything), snapshot validity rules, the idle
sweeper's timing, wedge handling, protection classes.

Conjecture: false reclaims come from the trigger (a 10-second PSI average,
one sample, no memory floor), not from the ranking; refusals of returning
users come from the 8 GiB reservation, not from real memory. A sustained
signal plus measured admission ends both without raising OOM risk, because
the floor still holds and reclaim still fires when memory is really short.

Admissible evidence: unit tests on the decision functions (local); then,
after deploy, read-only journal decision lines on Node B and a bounded
minted-test-computer run with the owner's OK. Local proof says nothing
about real PSI behaviour.

### Failure modes written before the code

Reclaim:
- R1. A burst (one `du`, one `nix build`, one cold boot) still triggers a
  reclaim. Pinned: a qualifying sample needs `memory some avg60` over its
  threshold **and** MemAvailable under the floor, for N consecutive
  samples; `avg10` alone never counts.
- R2. Real shortage is no longer answered (the fix is too tight). Pinned:
  N sustained samples with both conditions do reclaim.
- R3. Our own snapshot write counts as pressure and causes the next
  reclaim. Pinned: samples taken while any hibernate is in flight, or
  inside the cool-down after one ends, do not count and break the streak.
- R4. Two reclaims run at once (sweep, monitor tick and the HTTP endpoint
  all call it). Pinned: one reclaim in flight; a second call returns
  without hibernating.
- R5. Reclaim hibernates several computers per pass. Pinned: one per pass
  under the sustained signal.
- R6. A computer a user touched in the last N minutes is reclaimed.
  Activity is the existing `LastActiveAt` (stamped on every resolve, wake
  and transition) plus the guest busy probe. Pinned for real users; a test
  computer keeps yielding to real ones, as today.
- R7. Owner-marked computers (premium, critical, platform, held) become
  reclaimable. Pinned: unchanged.
- R8. A stale window (monitor stopped) keeps a past streak alive. Pinned:
  the newest sample must be younger than twice the sample interval.
- R9. The keepalive still drops on a burst (finding 2). Pinned: under the
  sustained signal the keepalive reads the sustained state.
- R10. Decisions cannot be explained after the fact. Pinned: every
  qualifying or suppressed sample and every reclaim logs its inputs
  (MemAvailable, floor, avg10, avg60, streak, cool-down, in flight).

Admission:
- A1. Concurrent wakes all pass one MemAvailable reading. Pinned: a
  reservation ledger, checked and written under one lock; a second
  admission sees the first's reservation.
- A2. A reservation never ends (a boot that never reports back). Pinned:
  a failed start releases at once; a finished start keeps only what its
  VM has not yet taken (need minus measured RSS) for a settle window; an
  in-flight reservation expires after a bound.
- A3. A computer with no measurement is under-estimated. Pinned: a
  conservative default (2 GiB) and a minimum (1 GiB), capped at the guest
  size.
- A4. The host floor is crossed. Pinned: never admit when MemAvailable
  minus reservations would fall under floor plus headroom plus need.
- A5. Protected computers are refused. Pinned: they are admitted as today,
  but reserve, so ordinary admissions count them.
- A6. A recover of a running VM is refused. Pinned: unchanged (it replaces
  in place, no admission).
- A7. A missing sample refuses everyone. Pinned: unchanged (admit).
- A8. Decisions are invisible. Pinned: a log line per decision and the
  ledger plus recent decisions in vmctl `/health`.

Rollback: `VMCTL_PRESSURE_RECLAIM_SIGNAL=instant` and
`VMCTL_ADMISSION_MODE=guest-size` restore today's decisions exactly; a git
revert of the code commit restores today's code.

Heresy delta: discovered 4 (above); introduced 0 intended; repaired after
the code lands and staging confirms.
