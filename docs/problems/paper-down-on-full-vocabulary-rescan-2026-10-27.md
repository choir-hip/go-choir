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

## 18:44 to 19:19: 12 GiB, a two-seat rejection, the disk edit, the paper back

- b9045445d (12 GiB, owner's one-time go) deployed at 18:43; its vmctl
  restart killed the boot again (`start-interrupted-repeatedly-by-vmctl-
  restart`, two in a row) and vmctl left the paper stopped until a read
  woke it at 18:44. That boot (`mem_total_mib=11959`) ran the pass 32 min
  at 11.9 GB, 3.8 cores, no disk reads, without finishing.
- The 12 GiB shape starved the host: b@c's start was refused at 19:15
  (`host_memory_pressure … short 949 MiB`, host 31 GB).
- lane/vocab-skip 0c7be0fc3 (trust an earlier process's marker when the
  alias ledger did not grow) was rejected by review 360, DeepSeek and
  Devin: a pass killed after its pre-apply persist launders the count; a
  replayed row can name an ID renamed by the original cutover without
  growing the ledger; role fields go unverified. Not landed.
- Owner, 19:1x: "if it fails this, you have my consent to use other means.
  Editing the disk or a code change." FP0 held and stopped the paper
  (vmctl hold/stop, caller fp0-paper-vocab-marker-owner-2026-10-27),
  mounted `data.img` read-only, read the sidecars: the report last written
  17:48:41 (no killed pass reached its persist, so no partial apply),
  `fenced_at` 2026-10-26T21:54:24Z, `deposit_upcast_aliases` 4; the ledger
  `vocab-deposit-upcast.jsonl` holds 12 unique object aliases, last written
  10-27 11:06. Backups (sha256 9d5e4fdad642…, c4bf477efbcb…) in
  `/var/lib/go-choir/fp0-paper-vocab-backup-20261027.GLfl/` on Node B.
  Edit: `replay_opened_at` "" and `deposit_upcast_aliases` 12; fence date
  unchanged. Unheld; the 19:19:29 boot: `vocabulary-migration … complete
  duration=0s`; a guest route answered 200 in 0.2 s.
- **Residual:** the 8 aliases recorded since the fence were never swept,
  so rows may still reference those old event IDs (dangling references to
  at most 8 choir.event objects; no data rewritten wrongly).
- Owner, 19:1x: "when you bring the paper back, skip the rescan. Fix it
  and get rid of that dead code. We need snapshotting and hibernation
  everywhere automatically." Next: delete the boot-blocking full pass
  (a station goal file), return guests to 8 GiB, and a station for
  automatic checkpointing, snapshots and hibernation on every computer.
