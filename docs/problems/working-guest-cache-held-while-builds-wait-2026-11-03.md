# Working guests keep their page cache while builds wait for host memory (2026-11-03)

Status: open. Mutation class of this doc: green. Evidence: read-only metadata
on Node B (vmctl journal, firecracker `/proc/<pid>/status`, the guests'
host-sourced `procmem` route), and code.

## Evidence

- 21:00-21:30Z: capsule jobs on two self-development arms waited about 10
  minutes each in "waiting for host memory: ~2 GiB over the host floor,
  under this job's 3072 MiB".
- 21:30Z, firecracker RSS against the guest's own meminfo (MiB):

  | computer (class test) | host RSS | guest Cached | guest MemFree | last ceiling act |
  |---|---|---|---|---|
  | 2bf3cab1 (luna arm) | 4080 | 2658 | 4769 | 18:06Z |
  | 4d38420c (pool arm) | 3614 | 2406 | 5086 | 20:22Z |
  | a875023a (claude arm) | 2280 | 624 | 6947 | 21:27Z |
  | e80d229b (deepseek arm) | 2279 | 574 | 6921 | 21:25Z |

  Host MemAvailable 9.3 GiB of 31.1; floor 4779 MiB.
- The elastic ceiling (`VMCTL_GUEST_MEMORY_CAP_COMPUTERS`, `class:test=1024`)
  acts on all four arms whenever they are between rounds: 40 acts in the
  10 h journal, each bringing used memory back to about 1 GiB. On 2bf3cab1
  and 4d38420c it has written nothing since their current rounds began,
  while each holds about 2.5 GiB of page cache.
- The paper (ceiling 1024, the same mechanism) sits at 2.2 GiB host RSS,
  down from 8.1 GiB uncapped (nix comment, balloon ladder 2026-11-01).

## Cause

The ceiling borrows busy_cold's busy rule
(`cacheReturnBusyColdRefusal`, `internal/agentcore/memory_cache_return.go`):
a live capsule or an open self-development operation is 409 busy, and the
ceiling stands aside. A "live capsule" is any non-terminal assignment bound
to a capsule that is active, freezing, frozen-undecided or being revoked
(`plannedRestartLiveCapsule`), so an arm stands aside for its whole round,
including the time its own build job waits for host memory. That rule is
the restart rule (CLAUDE.md: never restart mid-apply or with a live
capsule). Dropping clean page cache is not a restart: it cuts no work and
never takes anonymous memory, so a build still grows to the guest's full
size. Its only cost is rereads from NVMe, which the owner chose
("More disk reads, less memory usage", 2026-11-01).

## Mechanisms weighed

- **Ceiling (D6 S6): returns memory.** Readings above, plus the paper.
- **busy_cold (D6 S5):** at most 1 GiB per 10 minutes, refault guard;
  on the paper it backs off (`refault_stop` 21:21Z). It has the same busy
  rule, so it would not help a working arm either.
- **DAMON (`VM_GUEST_RECLAIM_ONLY`, RES-06):** idle host RSS flat with it
  on (docs/problems/start-refused-for-memory-no-reclaim-2026-10-26.md);
  costs a cold boot per computer. Not widened.

## Second defect: the two gates disagree

The spawn gate (`ReportCapsuleReservations`) admits a build when
`MemAvailable >= floor + burst pool (1 GiB) + reserved + increment`, the
start rule, as the design says (capsule-heavy-work-scheduling §1). The
facts a job waits on publish `MemAvailable - floor - others`, without the
pool, so a job can read 1 GiB more headroom than any admission will grant.

## Remaining error

Expected gain if the ceiling acts through work on the arms: a working arm
falls to about the idle arms' 2.3 GiB host RSS, so about 1.8 GiB on
2bf3cab1 and 1.3 GiB on 4d38420c, about 3 GiB on the 21:30Z reading. The
idle arms also show about 1 GiB of guest-free memory the host still holds
(free pages under reporting's 2 MiB order); that is not addressed here. Unknown: how much a running build slows when its cache
is dropped each minute (to be read on the arms' build durations).
