# The paper is down for a full vocabulary rescan after a deploy refresh (2026-10-27)

Status: open; documented while the paper is still booting. Class of a
fix: red (the boot vocabulary fence). Related:
`s0-vocab-rescan-fires-on-any-replay-2026-10-01.md` (the 2026-10-16
section measured the full pass at 3 min 44 s on the paper).

## Evidence (read-only, FP0, 17:58 to 18:0x)

- Deploy run 40552500965 (68eb6bbbc, a log-line change) failed at its
  active-computer health check: `curl: (7) Failed to connect to
  10.201.36.2:8085` (the paper, vm-1669020a…).
- vmctl: the paper's process from the 17:21 refresh was already failing
  health checks at 17:47:21 and 17:47:42, before the deploy; vmctl
  restarted at 17:47:56 and reattached it; the deploy re-armed the
  protected computer's retry at 17:48:25 and killed it for a planned
  refresh at 17:48:27. Why the 17:21 process went unhealthy is not known
  (its console is not among the four kept files; unread).
- The new boot: `restart kind=planned … target=68eb6bbbc…`, `projection
  recovery resume (local=4394599 W=34660 H=4394600 tail=1)`, then
  `startup phase=vocabulary-migration status=starting` at 17:48:41 and
  nothing after for 13+ minutes. Earlier boots today logged the phase
  `complete duration=0s` (no replayed rows).
- Firecracker for the paper is busy, not hung: 3.2 cores, 34 GB read in
  12 min (`/proc/<pid>/io` read_bytes 14.5 GB); `data.img` written at
  17:58. The data image is 93% of its cap (119.9 of 128 GiB; vmctl's
  compaction pressure is in observe).

## Why (code, verified unless marked)

- One applied replay row (`tail=1`) sets `replayed` and runs
  `MigrateAndFenceServingVocabulary` (`internal/autoputer/run.go:811`).
  The 10-16 fix narrowed the check to the rows replay wrote, but falls
  back to the full whole-table pass in some cases (a marker from another
  process, an alias count mismatch); which case fired here is unread
  (hypothesis: the reattached 17:21 process left a marker).
- The full pass took 3 min 44 s on 10-16; today it is past 13 min, so
  it grows with the store.

## Effect

The paper serves nothing while the pass runs (published stories are
served by the host; the paper's desks, publishing and the owner's
surfaces on it are down). Every deploy that lands while the paper has a
one-row tail can repeat it.

## 18:07: a second deploy killed the boot mid-rescan

- FP0's docs push ed6a0776c ran the full CI and deployed, because staging
  was still on 052c75484 after run 40552500965 failed: the deploy carried
  68eb6bbbc's code. It restarted vmctl at 18:07:29, which closed the
  paper's in-flight boot (`interrupted-by-vmctl-restart`, `destructive
  kill … cause=reap-unmanaged:orphan`) about 19 minutes into the rescan.
- The paper booted again at 18:07:37 as `restart kind=crash_or_stop`
  (`tail=0`), so its open work closes as interrupted, and the vocabulary
  pass started over at 18:07:44.
- Lesson: while a protected computer is in a long boot, any push that
  reaches a deploy restarts vmctl and kills that boot. FP0 holds pushes to
  main until the paper is healthy. A deploy that would restart vmctl
  while a critical_protected boot is in flight is a candidate gate.

## 18:31: out of memory, a boot loop

- Console: `Out of memory: Killed process 705 (autoputer) … anon-rss:7752140kB`
  at guest uptime 1425 s (18:31), in the vocabulary pass; the runtime
  restarted (pid 1080, `restart kind=crash_or_stop`, `tail=0`) and began
  the pass again at 18:31:23. The 17:48 attempt (Firecracker RSS 8.3 GB
  at 12 min) was most likely headed the same way.
- The pass runs on every boot until one completes and stamps the fence,
  so the paper loops: about 23 minutes per attempt, each ending in OOM.
- The guest has 8 GiB (`VM_INTERACTIVE_MEM_MIB=8192`, one global setting
  for every interactive computer; `GOMEMLIMIT` is half of it, 3969 MiB, so
  most of the 7.75 GB is outside Go's limit, hypothesis: Dolt).
- Options: (a) 12 GiB for interactive guests (lane/paper-mem 6878e651b,
  held for the owner: it changes every computer's shape at next boot, and
  whether 12 GiB suffices is unknown); (b) a red boot-path change so the
  pass streams instead of holding the whole table; (c) let the paper's
  pass run with the desks off, if a boot mode allows it (unread).
