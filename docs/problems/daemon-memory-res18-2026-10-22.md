# Daemon memory on the paper: what holds it, and why it is not what fills the guest (RES-18, 2026-10-22)

Date: 2026-10-22. Checklist item: RES-18 in
[security-density-architecture-checklist-2026-10-21.md](../security-density-architecture-checklist-2026-10-21.md).
Status: **measured; no fix in this record**. Mutation class of this record:
green. Evidence class: read-only reads of the paper (computer-1355b83b…,
vm-1669020a) and host `/proc` on Node B. Every guest read was sent without the
host credential ([host-internal-token-sent-to-guests-by-fp0-probes-2026-10-22.md](host-internal-token-sent-to-guests-by-fp0-probes-2026-10-22.md)).
No SQL, no writes to a guest, no lifecycle acts, no forced GC.

## What RES-18 named, and where each stands

1. **The appender leak is fixed on main.** `b36519198` ("drop each projection
   payload from the appender cache once finalized") deletes the entry after a
   successful append (`internal/computerevent/appender.go:457-466`). In heap
   reads 1 and 2 below, code in `internal/computerevent` itself retains
   nothing measurable (0.0 MiB of flat `inuse_space`). Memory allocated
   beneath an append call is Dolt's write path (new nodes into the node
   cache, table files opened after a journal flush), counted in the Dolt
   buckets. The [problem record](guest-appender-retains-every-projection-payload-2026-10-19.md)
   stands as written.
2. **The Dolt cache knob is off everywhere, and cannot be turned on per
   computer.** `CHOIR_DOLT_CACHE_NODE_MIB` (`internal/gomemlimit/gomemlimit.go:61-63`)
   is read from the daemon's environment. That environment comes from the
   guest image (`nix/autoputer-vm.nix`, static) and from kernel arguments
   that the guest's env service copies into `/run/go-choir-autoputer.env`
   (`nix/autoputer-vm.nix:453-504`). No kernel argument carries the knob,
   so it can only be set for every computer at once, by an image change.
3. **`GOMEMLIMIT` is fixed at boot (J14).** It is set once from `MemTotal`
   (`internal/gomemlimit/gomemlimit.go:170-212`, called first in
   `internal/autoputer/run.go:165`). See "J14" below for when that matters.

## The knobs today

| knob | where | default | what it does |
|---|---|---|---|
| daemon `GOMEMLIMIT` | `gomemlimit.LimitFor` | MemTotal / 2, at least 512 MiB | soft limit; the GC works harder near it. Environment `GOMEMLIMIT` wins (`off` is the rollback) |
| desk worker `GOMEMLIMIT` | same | MemTotal / 16, at least 256 MiB | each worker's soft limit; `CHOIR_DESK_WORKER_GOMEMLIMIT` overrides |
| `GOGC` | not set anywhere | 100 | heap goal = 2 x live heap, until the soft limit binds |
| Dolt node cache | `memlimit.NodeCacheSize` (Dolt `libraries/doltcore/memlimit`) | 256 MiB | shared LRU of decoded prolly nodes, sized by node bytes. Dolt sizes it from the Go limit at first use; `ApplyDaemon` calls Dolt's `Init` before setting the soft limit, so Dolt sees no limit and takes its defaults |
| Dolt memtable | `memlimit.MemtableSize` | 256 MiB cap per NBS store | write buffer; with the knob set it gets the same MiB as the node cache |
| Dolt decoded chunks | `memlimit.DecodedChunksSize` | 32 MiB | old-format value cache |
| `CHOIR_DOLT_CACHE_NODE_MIB` | daemon env | unset | 16-256: a transient limit makes Dolt's own split give the node cache that many MiB (memtable the same, decoded chunks about 0.13x) |
| Dolt `mmap_archive_indexes` | Dolt repo config | false | archive-file indexes mapped from the file instead of read into the heap. Noms table files never support it (Dolt `store/nbs/file_table_reader.go:121`) |

The paper's boot log, every boot today (05:12 to 10:40 UTC):
`autoputer: gomemlimit role=daemon mem_total_mib=7942 limit_mib=3971
desk_worker_limit_mib=496 dolt_budget=default dolt_node_cache_mib=256`.
Desks cgroup: memory.max 3971 MiB, memory.high 2978 MiB.

## Measurements

Deploys refreshed the paper four times in the window: 10:06:10 UTC
(5289387c9, Firecracker pid 9918), 10:40:38 (7448bf899, pid 39029),
10:56:55 (8cb7e1acb, pid 54632) and 11:26:48 (4bc3aa3ec, pid 74211). Each
refresh gave the guest a new address (10.201.124.2, .130.2, .131.2,
.137.2), so later reads resolved the paper's address from vmctl's list
just before each read. Guest: 8 GiB
machine, `MemTotal` 7942 MiB, 4 vCPUs, balloon at 0 MiB with free page
reporting, no DAMON reclaim (not in `VM_GUEST_RECLAIM_ONLY`).

### Host RSS and the guest split (procmem; host `/proc`)

| UTC | paper Firecracker VmRSS | guest anon | guest page cache | guest free | daemon RSS | desk workers |
|---|---|---|---|---|---|---|
| 10:07:15 / :23 | 4.68 GiB | 1.77 GiB | 2.89 GiB | 2.94 GiB | 1.82 GiB | 0 |
| 10:12:11 | 7.97 GiB | 2.37 GiB | 5.00 GiB | 0.16 GiB | 2.36 GiB | |
| 10:18:12 | 7.87 GiB | 2.26 GiB | 4.24 GiB | 0.69 GiB | 2.22 GiB | |
| 10:26:14 | 7.98 GiB | 2.60 GiB | 4.33 GiB | 0.21 GiB | 2.57 GiB | 12, 0.29 GiB RSS |
| *refresh 10:40:38* | | | | | | |
| 10:42:18 (host only) | 7.24 GiB | | | | | |
| 10:48:57 | 7.95 GiB | 2.56 GiB | 4.81 GiB | 0.12 GiB | 2.53 GiB | 13, 0.32 GiB |
| 10:51:17 | 7.86 GiB | 1.83 GiB | 4.08 GiB | 1.46 GiB | 1.79 GiB | 12, 0.30 GiB |
| 10:56:49 | 7.94 GiB | 2.67 GiB | 4.65 GiB | 0.12 GiB | 2.63 GiB | 14, 0.34 GiB |
| *refreshes 10:56:55, 11:26:48* | | | | | | |
| 11:30:19 | 7.92 GiB | 2.26 GiB | 5.07 GiB | 0.23 GiB | 2.28 GiB | 7, 0.16 GiB |

The whole guest is touched within two to six minutes of boot, and stays
touched. The 10:40 boot ran no boot GC (its journal was under 1 GiB) and
still reached 7.24 GiB 100 s after the process started, so the fill is the
boot's store reads and writes, not the GC alone.

Free memory in the guest does not come back to the host promptly. At
10:51:17, just after an online GC, the guest had 1.46 GiB free and the
host still held 7.86 GiB; at 10:18:12, 0.69 GiB free against 7.87 GiB.
Hypothesis: the daemon's freed heap returns to the guest kernel in pieces
smaller than the balloon's reporting order (2 MiB and up), and the page
cache takes it back before it is reported (density lever L5 names lowering
`page_reporting.page_reporting_order`). Not traced.

The owner's computer (vm-4a9afa23, host `/proc` only) held 3.84 GiB
(4,025,196 kB) through every sample, flat.

### What the page cache holds (pagecache read, 10:13:51)

- `/mnt/persistent` (the computer's Dolt store and files): 2,797 files,
  21.76 GiB on disk, **4.04 GiB resident**.
- `/nix/store` (base OS): 6.14 GiB on disk, 0.29 GiB resident (the
  autoputer binary 132 MiB, choir-updater 55 MiB, desk-worker 20 MiB).
- Inactive(file) 4.75 GB against Active(file) 0.17 GB: nearly all of the
  cache is cold, reclaimable, and never handed back to the host, because
  the guest frees it only under pressure.

Why the Dolt files are in cache: the store writes its chunk journal at
about 2.3 MiB/s, and Dolt GC rewrites the journal about every 10 minutes.
The console log from 05:00:10 to 10:51:13 UTC shows 33 completed GCs (31
online, 2 at boot), each over a 1,056 to 2,117 MiB journal, 12 to 41 s
each, 46.9 GiB of journal in all. Each GC reads the journal and writes a
table file of about the same size, all through the page cache. Dolt's
"live" size grew from 12 GiB (04:59) to 21 GiB (10:36). Each GC also
purges Dolt's node cache at its safepoint (Dolt `NodeStore.PurgeCaches`),
so on the paper the node cache is emptied and refilled about every 10
minutes.

### What holds the daemon's heap (heap profiles, no GC forced)

Bytes are `inuse_space` scaled from the 512 KiB sampling rate, as of the
last GC; buckets by the first matching stack frame.

| bucket | heap 1, 10:08:33 (boot + 2.4 min) | heap 2, 10:25:09 (boot + 19 min) | heap 3, 11:30:20 (boot + 3.5 min) |
|---|---|---|---|
| Dolt live size at the time (GC log, rounded) | 20 GiB | 20 GiB | 22 GiB |
| Dolt table-file indexes (`nbs.newFileTableReader`) | 500 MiB (57%) | 505 MiB (49%) | 555 MiB (56%) |
| of which noms table files (`nomsFileTableReader`) | 293 MiB | 293 MiB | 286 MiB |
| of which archive files (`newInMemoryArchiveIndexReader`) | 208 MiB | 213 MiB | 269 MiB |
| Dolt node reads (`nodeStore.Read`, chunk decompress) | 204 MiB | 165 MiB | 283 MiB |
| Dolt node writes (serializer buffers of new nodes) | about 80 MiB | 125 MiB | 22 MiB |
| Dolt journal index (`rangeIndex`, journal reads) | 6 MiB | 72 MiB | 44 MiB |
| other Dolt (has-cache LRU, schema copies, ...) | 120 MiB | 109 MiB | 36 MiB |
| Choir code (search term counts 24 to 34 MiB, ...) | 37 MiB | 52 MiB | 40 MiB |
| total profiled live | 880 MiB | 1,041 MiB | 993 MiB |

Heaps 1 and 2 are text profiles (`heap?debug=1`), heap 3 the proto
profile (`heap`), so heap 3 has no `MemStats` below. Bucket edges differ
slightly between the two parsers ("other Dolt" most).

Runtime view (`runtime.MemStats`):

| | heap 1 | heap 2 |
|---|---|---|
| HeapAlloc | 987 MiB | 1,169 MiB |
| HeapInuse | 1,162 MiB | 1,511 MiB |
| HeapIdle not yet returned (HeapIdle - HeapReleased) | 680 MiB | 821 MiB |
| HeapSys | 1,920 MiB | 2,608 MiB |
| Sys - HeapSys (stacks, spans, GC metadata, other) | 54 MiB | 142 MiB |
| NextGC (heap goal) | 1,821 MiB | 2,143 MiB |
| NumGC | 38 | 295 |
| GC CPU fraction since start | 0.12% | 0.39% |
| TotalAlloc since start | 23.5 GiB | 256.5 GiB |

Readings:

- **Dolt's indexes, not its cache, are the largest thing in the heap.**
  Every table file's chunk index is read into the heap at open: 500 to
  555 MiB for a 20 to 22 GiB store, the same 25 MiB per GiB of live store
  in heap 1 and heap 3. The node cache (reads plus newly written nodes,
  about 290 MiB) reaches its 256 MiB budget within 2.5 to 3.5 minutes of
  boot. Choir's own retained memory is 40 to 50 MiB.
- **The soft limit never binds.** The heap goal (1.8 to 2.1 GiB) comes from
  `GOGC=100`, well under the 3,971 MiB limit. The daemon's RSS is about
  twice its live heap plus 0.7 to 0.8 GiB of idle heap the runtime has not
  returned yet. GC is cheap here (under 0.4% of CPU).
- **Allocation churn is high**: 256 GiB in 19 minutes, about 225 MiB/s. The
  largest single source is Dolt copying a table schema on every schema read
  (`schema.(*indexImpl).copy` from `UnmarshalSchemaAtAddr`, about 38 GiB, 15%),
  reached from objectgraph list and get calls. That is a CPU cost, not
  retained memory; it belongs to the CPU track
  ([computer-cpu-efficiency-2026-10-14.md](../archive/reports/computer-cpu-efficiency-2026-10-14.md)).
- Desk workers are small: 7 to 14 at 0.16 to 0.35 GiB RSS together, most
  of it the shared binary.

## What this means for host memory

On the paper, the daemon is not what fills the guest; the page cache is.
Host RSS equals guest used memory (the balloon reports only pages the guest
frees), and the guest is full: 1.8 to 2.7 GiB anonymous (daemon 1.8 to
2.6 GiB, workers 0.2 to 0.35 GiB) and 4.1 to 5.1 GiB of mostly cold Dolt
file cache.
Any heap the daemon gives back is refilled by the store's write stream
within minutes. So, today:

- the node cache knob, a lower `GOGC`, or a lower `GOMEMLIMIT` would move
  memory from the daemon to the page cache inside the guest. **They would
  not lower the paper's host RSS** (hypothesis from the split above; no
  experiment run).
- The host-side lever for this computer is guest page-cache reclaim
  (density lever L5, RES-06: DAMON_RECLAIM, already built and selected per
  computer by `VM_GUEST_RECLAIM_ONLY`), and fewer bytes written by the
  store (the tape-volume work:
  [projection-event-volume-design-2026-10-19.md](../archive/reports/projection-event-volume-design-2026-10-19.md)).
- The daemon's heap matters as the **floor** of a guest's shape. It sets
  the smallest base memory the paper could run in (RES-09 aims at a
  1.5 to 2 GiB base), and it is what is left once reclaim takes the cold
  cache.

## Problems found

1. **The daemon's floor grows with the store's history** (supported by
   two points, not proven). Dolt holds an index entry per chunk in the
   heap, so the index share scales with the store's chunk count: 500 MiB
   at 20 GiB live (10:08) and 555 MiB at 22 GiB live (11:30), about
   25 MiB per GiB both times. Live grew 9 GiB in about 6.5 hours today
   (12 GiB at 04:59, 21 GiB at 10:36, 22 GiB at 11:22), which would add
   about 35 MiB of heap an hour, about 0.8 GiB a day, until the history is
   compacted ([guest-store-compaction-scheduled-2026-10-20.md](../archive/problems/guest-store-compaction-scheduled-2026-10-20.md)).
   The live figures are rounded to whole GiB, so the per-GiB ratio is
   good to about 5%.
2. **The paper's page cache fills its guest within two to six minutes of
   every boot** and is never handed back, because DAMON reclaim is not on for
   it. Selecting it is an operator act: a `nix/node-b.nix` change (red)
   that takes effect at the paper's next refresh, since boot arguments are
   in the launch hash (deploys refreshed it four times in 80 minutes
   today). Risk to weigh: reclaimed store pages are read again from disk
   on a computer that is already store-bound after boot.
3. **Reclaim reached its first test computer only late in this window.**
   computer-71afbf14… (vm-c5c801f8) is named in `VM_GUEST_RECLAIM_ONLY`.
   Its 09:49:44 UTC boot had no `damon_reclaim.*` boot arguments (its
   `fc-config.json` at 10:17; why is not traced here). Deploy refreshes
   cold booted it at 10:58:39 and 11:28:27; the 11:28 boot carries all six
   `damon_reclaim.*` arguments. Its host RSS then stayed at 1.32 to 1.35 GiB
   from 11:31:55 to 11:41:33, against 4.28 GiB at 10:07 in the boot without
   reclaim. Not a controlled comparison (activity differs and is not read
   here), but it is the first reading of a computer with reclaim on.
4. **The node cache knob has no per-computer path** (above), so the LR
   report's optional Dolt probe ([lr-guest-memory-return-2026-10-14.md](../archive/reports/lr-guest-memory-return-2026-10-14.md)
   step 8) cannot run without changing every guest image.
5. **A text heap profile of the paper is 1.1 GB.** `heap?debug=1` at 10:25
   took 37 s and returned 1,146 MB (139,753 stack records): a real load on a
   store-bound computer. Later reads should use the proto form
   (`heap`, no `debug`), which is a few megabytes.

## J14: when a boot-time limit matters

- **The paper:** no memory hotplug and a balloon with `deflate_on_oom`, so
  inflation does not lower `MemTotal`. `MemTotal` cannot change after boot;
  J14 does not apply.
- **Hotplug test computers** (class:test, vm-4a9afa23 and vm-c5c801f8): an
  8 GiB base plus a 4 GiB hotplug region, unplugged at boot. `MemTotal` at
  boot is the base, and a lease can only raise it. A limit fixed at boot is
  then lower than a recomputed one: the GC works harder during a lease,
  nothing grows unbounded. Leases are for capsule jobs, not the daemon
  ([memory-hotplug-controller-design-2026-10-14.md](../archive/reports/memory-hotplug-controller-design-2026-10-14.md), J14).
- **It starts to matter with RES-09's small base.** On a 1.5 to 2 GiB base
  the daemon's limit is 0.75 to 1 GiB, which is under this daemon's live
  heap (about 1 GiB): the GC would run near its 50% CPU cap. A computer
  with a small store may fit; the paper would not. That is a reason to set
  the daemon's limit from the shape the computer is meant to run at, not a
  reason to recompute it on every plug.

## What is unknown

- How full the node cache is on a test computer with a small store. If it
  stays well under 64 MiB, `CHOIR_DOLT_CACHE_NODE_MIB=64` changes nothing
  there.
- Whether the table-index share keeps tracking live store size past
  22 GiB (problem 1).
- How fast host RSS falls on a computer with DAMON reclaim on, and what the
  daemon floor then is.
- What the 4.04 GiB of Dolt cache is by file (the pagecache read names no
  persistent files, by design).

## Lever decision

**No lever is implemented in this lane.** The data shows no clear, small
lever:

- `CHOIR_DOLT_CACHE_NODE_MIB` binds on the paper (its cache is full), but
  saves at most about 190 MiB live, about 0.4 GiB at the GC's 2x goal, and
  the page cache takes that memory back while reclaim is off. A smaller
  cache also costs decompression CPU on a computer that is already
  store-bound after boot ([paper-unreachable-minutes-after-every-boot-2026-10-22.md](paper-unreachable-minutes-after-every-boot-2026-10-22.md)).
  On test computers its effect is unmeasured.
- Recomputing `GOMEMLIMIT` when `MemTotal` changes would raise the
  daemon's limit during a lease: no density gain (J14 above).

**Measurements that would decide:**

1. On computer-71afbf14…, now booted with reclaim (problem 3), during and
   after a busy stretch: its host RSS every 2 minutes for 30 minutes, its
   procmem split, and one proto heap read. If host RSS falls to near guest anon, reclaim works and the daemon
   becomes the largest term. If its node-read plus node-write share is
   128 MiB or more, a per-computer path for the node cache knob is worth
   building (vmmanager boot argument from a `*_ONLY` selector, plus one
   case in the guest's env service).
2. On the paper, one more proto heap read at a clearly larger live size
   (26 GiB or more, about a day on), set against the GC log's `live=`
   figure. If index MiB per live GiB stays near 25, the lever for the
   daemon floor is history compaction, or archive files with
   `mmap_archive_indexes` (archives already hold 269 of the 555 MiB), not
   the node cache.
3. Before RES-09 shrinks a base below 4 GiB: the daemon's live heap on that
   shape, to set its limit.
