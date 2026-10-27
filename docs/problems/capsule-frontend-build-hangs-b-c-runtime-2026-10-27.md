# A capsule frontend build hung b@c's runtime and vmctl recovered it (2026-10-27)

Status: open; the cause is not confirmed. Mutation class of any fix:
unknown. Documented before any fix.

## Evidence (UTC, host journal and the round's trace export)

- **00:30:06.** b@c (computer-6c50d6e5…, VM candidate-fleet-bf04bbc7…)
  takes a planned refresh onto train 29 (cba66873b) while idle, between
  the two attempts of the citation round (trajectory 150b7de5…).
- **00:36.** The second attempt starts in a fresh capsule. Engineering
  (DeepSeek) works through about 215 loop iterations, reaching about 223k
  input tokens.
- **01:08:57.** Engineering starts a capsule job: `npm run build` of the
  frontend, using the shared node_modules when the package files match,
  15-minute limit.
- **01:09:08.** b@c's runtime health check fails for the first time.
  Failures repeat every 15 s.
- **01:10:28.** vmctl kills the VM (`cause=recover`) and cold boots it
  (epoch 48, healthy at 01:10:42). Under the restart rule the open work
  closes and does not resume: the assignment is cancelled
  `interrupted_by_restart`, and the capsule is revoked.
- **Host signals.** vmctl's pressure reclaim saw intermittent host IO
  pressure (01:00:57, 01:04:54, 01:08:57) and no memory pressure.

## Ruled out (read in code)

- **Train 29's job-memory deletion.** The removed lease hook acted only
  for jobs that declared memory_mib, and with hotplug off it admitted them
  unchanged. This job declared none.
- **Balloon statistics newly on for b@c.** The balloon is reporting only:
  amount 0, deflate_on_oom true.

## Hypotheses (not confirmed)

- **H1, guest memory.** The build exhausted guest memory. The capsule
  budget is 3/4 of MemTotal, so the runtime and its seven loops share the
  last quarter, and the guest thrashed without an OOM kill long enough to
  miss health checks.
- **H2, guest CPU.** The build pegged the guest's vCPUs, and the runtime's
  health handler starved past the check timeout.
- **H3, host IO.** Host IO pressure (other computers, the paper backfill)
  stretched the build's disk IO, which made H1 or H2 worse.

The first attempt's build on the same computer passed, before the refresh,
so this is not reproducible on every build.

## Next observation

Repeat the build with instruments: the capsule records `/proc/pressure/*`
and `free -m` every 5 s during the build to a file. Prefer
`nice -n 10` and a bounded Node heap. FP0 sends this as the third attempt
of the citation round.
