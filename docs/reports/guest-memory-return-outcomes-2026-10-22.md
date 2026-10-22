# Guest memory return: what each probe outcome means and which lever it points to

Date: 2026-10-22. FP8 P4 (RES-06) with FP10 RES-10. Problem record:
[guest-damon-reclaim-state-unreported-2026-10-22.md](../problems/guest-damon-reclaim-state-unreported-2026-10-22.md).
Probe: `scripts/fp8_memory_return_probe.py`. No lever is built by this
lane; this page says which one the evidence points to.

## The chain

A guest page reaches the host again only through three steps:

1. **Freed in the guest.** Anonymous memory is freed when a process exits;
   page cache only under memory pressure, by DAMON_RECLAIM (cold regions,
   `min_age`), or by a write to a cgroup's `memory.reclaim` (capsule
   teardown does this for capsule cgroups).
2. **Gathered into a reportable block.** Free page reporting hands the host
   only free blocks of at least `page_reporting_order` (9: 2 MiB on
   x86_64). The kernel holds -1 until a reporting device registers.
3. **Released by the VMM.** Firecracker's balloon, with
   `free_page_reporting: true` negotiated (`VIRTIO_BALLOON_F_REPORTING`),
   discards the reported range and the host RSS falls.

## Reading the probe

The CSV has one row per 30 s; the summary's `reading` is a first pass. FP0
reads the rows.

| Outcome | What the rows show | Meaning | Lever it points to |
|---|---|---|---|
| **A. DAMON off** | `damon_enabled` false or `kdamond_pid` -1 on a reclaim-selected computer; `damon_bytes_reclaimed` flat | The boot arguments did not take (FP8 proof step 8.4's case) | Enable damon_reclaim another way: a guest unit that writes the parameters and `commit_inputs` (an image change; it would be the first writer, so it needs its own review). Check the boot args reached the guest first (`fc-config.json`) |
| **A'. DAMON on, idle** | kdamond running, `kdamond_cpu_ticks` rising, `damon_bytes_reclaimed` flat, `guest_cached_mib` flat | Running but nothing qualifies: watermarks off (free above `wmarks_high`), regions never `min_age` cold (the desk keeps touching them), or the quota (128 MiB/s, 10 ms) never spent | Tune `min_age` (`VM_GUEST_RECLAIM_MIN_AGE_S`) and read again; if the cache is warm, it is not cold cache and DAMON is the wrong lever: `memory.reclaim` on the daemon's cgroup when idle |
| **B. Freed but not returned** | `guest_memfree_mib` rises (DAMON bytes or the burst's end), `fc_rss_mib` does not fall | Step 2 or 3 stopped. If `page_reporting_registered` false or `balloon_reporting` false: no reporting device on that guest. If both true and `guest_reportable_free_mib` stays small next to MemFree: the freed memory is scattered 4 KiB pages, never a 2 MiB block | Device absent: the computer booted without the balloon (restored snapshot from before reporting, or the flag off): cold boot it. Fragmentation: a lower `page_reporting.page_reporting_order` boot argument (more, smaller reports), or the RES-10 step 3 balloon (inflation takes pages regardless of block size) |
| **C. Returned** | `fc_rss_mib` falls by at least half of the MemFree rise within a few samples | The chain works; the 13:06 flat reading was "nothing freed" (A or A'), or too short | Keep DAMON on test computers, take the paper next with the same probe as its proof; if A' showed the cache warm, `memory.reclaim` when idle |

Two windows in the summary:

- **burst_end** (with `--burst capsule`): from the burst's RSS peak to the
  first idle sample. The job's exit frees its anonymous memory and capsule
  teardown reclaims its cache, so this tests steps 2 and 3 with DAMON out
  of the picture. If it reads B, DAMON cannot help until reporting does.
- **idle**: the last 20 minutes. Tests DAMON (step 1) and, if it frees
  anything, steps 2 and 3 again.

A window whose first or last sample has no `fc_rss_mib` or no
`guest_memfree_mib` reads **undeterminable**, naming the missing side; it
is never a 0 MiB change. When `fc_rss_mib` is empty, the row's `errors`
column says why (no firecracker process matched the VM, with the pattern).
`page_reporting_registered` is true only when the order is set and the
balloon is bound with reporting negotiated (a boot-argument order alone
does not count).

Guard against Goodharting: `workingset_refault_file` and `pgmajfault`
rising in the idle window after DAMON frees cache means it is evicting
pages the desk still needs (store pages re-read from disk). A host RSS win
paid in refaults is not a win.

## Levers (none built here)

- **Enable damon_reclaim** by a guest unit (outcome A). Today only boot
  arguments enable it, for `VM_GUEST_RECLAIM_ONLY` computers.
- **`memory.reclaim`** on the daemon's cgroup (or the guest root) when the
  computer is idle (outcome A' with warm cache, or as the simple
  alternative to DAMON). Capsule teardown already uses it per capsule.
- **RES-10 step 3 balloon** (outcome B from fragmentation, or as the
  emergency path): inflate under host pressure
  (docs/definitions/choir-launch-fp10-res-10-2026-10-21.md). It waits for
  RES-09 (one controller owning guest memory targets).

## FP0 steps

1. Deploy the guest image carrying `memory_return` on procmem (this lane's
   orange commit). Rollout is by guest refresh; a guest without it gives
   empty DAMON columns and the summary says so.
2. Pick a reclaim-selected test computer that stays awake (71afbf14 is in
   `VM_GUEST_RECLAIM_ONLY`), and, for a control, a test computer that is
   not selected. Optionally name both in `VM_BALLOON_STATS_ONLY` so the
   balloon columns fill (a cold boot each).
3. Run, one computer at a time:

   ```
   scripts/fp8_memory_return_probe.py --computer-id <test computer id> \
     --burst capsule --cred-file <its 0600 credential file> \
     --out docs/evidence/fp8-memory-return-<computer>-<date>.csv
   ```

   About 25 minutes (1 baseline, 4 burst, 20 idle). `--burst none` samples
   only. The test computer must not hibernate for idle during the run (the
   probe stops after 3 silent samples, exit 3).
4. Record the summary and the outcome in the ledger and the problem record.
