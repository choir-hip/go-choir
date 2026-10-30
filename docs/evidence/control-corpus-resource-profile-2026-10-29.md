# Control and corpus: responsibilities and observed resources (2026-10-29)

Status: read-only investigation, not an engine selection, capacity benchmark or
repair. Mutation class: green evidence. Owner requested understanding what
control does and comparing actual historical resource use with corpus.

Observed on Node B via `ssh node-b`, approximately 15:34-15:52 UTC, including
retrospective analysis of October 29 09:00-15:00 UTC. No runtime
code, configuration, service restarts, maintenance, database writes, root SQL
access, provider calls or model runs. Small SELECTs used the existing app
credential on loopback, never emitted as a value or command argument. The
251-row head-table read succeeded. A newest-100-receipt SELECT hit an 8 s
client deadline and was not retried. PROCESSLIST was denied to the app user;
privileges were not escalated. EXPLAIN returned column headers only, not a
usable plan. Thus no receipt row count, receipt-size census, or query-level
CPU/I/O attribution was obtained. A client timeout is not proof of server-side
cancellation.

Earlier comparison and sources:
[engine alternatives](storage-engine-alternatives-2026-10-29.md),
[selection goal](../definitions/choir-data-architecture-selection-2026-10-28.md),
[initial direction](../archive/design/data-rearchitecture-2026-10-28.md).

## Conclusions and corrections

1. Control's semantic job is simpler than corpus ingestion/retrieval, but its
   current implementation is not a negligible workload. It has both hot
   current state and append-only event receipts, plus Dolt database history.
   Avoid equating those three different forms of growth.
2. Same server engine, independently operated control and corpus services
   remains the owner's preference. Corpus requirements drive the shared-engine
   choice unless control's transactional/durability/isolation needs falsify it.
   Security first; stability second; scalability and simplicity jointly next.
   More engines are not justified merely to optimize control in isolation.
3. Current control has substantial CPU and storage reads. Corpus has repeated
   memory-growth/restart behavior, including a confirmed memcg OOM today.
   These observations do not prove one engine fault or predict either tier's
   unavoidable cost after rearchitecture.
4. Host CPU/I/O, cgroup pressure, current RAM, maintenance peaks, retained
   history and backup size must be measured separately. The same memory limit
   is not a fixed resource requirement, and all oldgen bytes are not
   necessarily obsolete data.
5. No continuous historical utilization series was found locally. Retained
   journals, daily audit points, cumulative counters and earlier receipts
   support discrete observations, not a smooth historical load curve.

## What the control database does

Store A is `platform` on port 13306. Store B is `corpus` on 13307; corpusd
fronts both SQL pools. They are separate database-server processes today.

| Control responsibility | Durable records | Work/growth driver |
| --- | --- | --- |
| Accept exactly one next computer event | Current canonical/desired/effective heads and state commitments; append/idempotency receipts | Every accepted event updates a head and inserts a receipt |
| Make acknowledgement and retry auditable | Sequence, previous head, digests, signed receipt, pin refs, request/idempotency commitments | Receipts accumulate with events, not merely computer count |
| Rebuild and verify computer projections | Job generation/lease/status, replay watermarks, bases, epoch roots, base pins, checkpoint verdicts | Jobs/heartbeats, retained bases and checkpoint frequency |
| Control lifecycle and safe self-development | Lifecycle operations/receipts, modes, generations, owner acts and admission joins | Start/stop/update/restore operations, retries and approvals |
| Project an accepted version into a serving route | Route slots, version/input refs, generations, transition receipts and certificates | Route transitions and immutable version inputs |
| Security control and key recovery | Public-key history, revocation epochs, escrow wraps, unwrap approvals and transparency records | Keys/computers and security operations |

The event flow is: pin immutable body/payload -> prepare guest projection ->
host serializable CAS -> receipt/head transaction commit -> verify receipt ->
finalize guest projection. The host is not merely a directory of users.

Evidence: `internal/platform/computer_events.go:17-156,194-305`,
`internal/computerevent/appender.go:756-790`,
`internal/platform/projection_jobs.go:18-88`,
`internal/platform/file_cas.go`, `internal/platform/owner_acts.go`,
`internal/platform/key_escrow.go`.

The deployed route binding is explicit: `nix/node-b.nix:1622` sets
`VMCTL_ROUTE_DSN` to port 13306/database `platform`. The nearby comment
calling it a world-wire SQL server is stale; routeledger is in Store A in
this deployment, with vmctl the route CAS writer. Event authority and route
projection remain logically distinct despite sharing this database.

Outside Store A: event bodies/tape segments and content blobs live in artifact
storage; published objects and corpus metadata/edges are Store B; guest app
state is embedded; VM images/snapshots and ownership JSON are host files;
users/sessions/credentials are auth SQLite. Control registries can reference
large files without those bytes living in SQL. Filesystem pressure therefore
cannot be attributed to SQL database sizes alone.

### Current bounded head-table data

At 15:38:31 UTC, `SELECT computer_id,sequence,updated_at FROM
computer_event_heads ORDER BY computer_id LIMIT 1000` returned **251 rows**,
below the cap. Four rows had an updated_at date of October 29. These are
historical computer records, not 251 live VMs or four simultaneous workers.

The sum of head sequence positions was **5,984,911**; this is NOT a COUNT of
receipt rows and does not establish their retention/deletion state. The
paper's head was **4,940,220**, about **82.54%** of that sum. Paper identity is
corroborated by `docs/problems/paper-never-checkpointed-2026-10-27.md:385`.
The next-highest head was 595,949. IDs other than the already documented
paper are omitted here.

This establishes that a small active fleet can already have millions of
canonical event positions. It does not establish an event-per-user forecast.
The logical receipt volume remains to be measured safely on a captured base.

## Several hours of active Autopaper, not an idle sample

The owner rejected the initial 30-second sample as insufficient to characterize
corpus under active Autopaper. That correction is adopted: **exclude that sample
from workload characterization and engine-selection evidence**. The following
retrospective window is the relevant workload evidence.

### Activity and publication success, October 29 09:00-15:00 UTC

Read the retained corpusd journal for the six-hour window and streamed a bounded
metadata analysis over the paper VM's five rotated console files. The VM is
`vm-1669020adcece51c2f5ff9b6df399ef5`, mapped to paper computer
`computer-1355b83bced1613eb048ee28d66688a8`. Console retention began at
09:14:12; therefore the first publication count is a **partial-hour lower bound**.
All later hours have console coverage. No article bodies or credentials were
copied into this receipt.

| UTC hour | Successful paper publication calls logged | capture_search operations logged | front_publications operations logged |
| --- | ---: | ---: | ---: |
| 09:00-10:00 | 19, from 09:14:12 onward | 55 | 52 |
| 10:00-11:00 | 20 | 29 | 60 |
| 11:00-12:00 | 15 | 56 | 47 |
| 12:00-13:00 | 21 | 44 | 70 |
| 13:00-14:00 | 14 | 42 | 917 |
| 14:00-15:00 | 21 | 30 | 1,804 |

There are **110 logged successful paper publication calls** in the retained
portion of this window. The success marker is emitted only after
PublishVersion returns success
(`internal/agentcore/story_entities_source.go:175-183`). It proves successful
publication requests, not 110 distinct stories: revisions and idempotent
republication may repeat a story. Held seeds, refusals and failed publications
are not counted as successes. Paper-attributed entity errors alone would not
have proved successful publishing.

The corpusd retrieval counts are service-wide, **not paper-only**, and do not
distinguish owners' browsing, agents, probes or production readers. The large
later rise in front-list calls must not be described as ingestion growth.
The 11:00 hour contains one capture_search error with 59,999.69 ms logged
duration; the other hours' capture_search logs have no errors. This is a
mixed publishing/search/graph workload, not a synthetic bulk-ingestion test.

Logging completeness matters: `internal/platform/retrieval_metrics.go:40-52,104-158`
logs front_publications and capture_search on every operation, subject to
journal retention/rate limiting. In contrast, get_object, get_edge,
list_edges_from, list_edges_by_kind, list_edges_to and capture_index_write
are logged only at 100 ms or slower, or on error. Their journal counts and
latency distributions are **censored tails**, not throughput or whole-operation
percentiles. Four successful capture_index_write calls appear in that tail;
this does not mean only four writes occurred. The health counters below count
fast calls too, but cover a different, longer interval.

### Database resources across the active window

Fifteen completed corpus Dolt service lifetimes lie wholly inside 09:00-15:00.
Their first inferred start is 09:03:41.290 and last exit is 14:55:29.
Summing their logged wall durations covers **5.863 service-hours**; summing
their CPU durations gives **14.990 CPU-minutes**, or **0.0426 mean CPU cores**
over those covered lifetimes. Boundary-crossing lifetimes and restart gaps
are excluded, not prorated. This is database CPU, not total Autopaper cost.

Per-lifetime memory peaks range from **5G to 7G as formatted by systemd**.
These are maxima, not continuous observations, a time-weighted mean, or proof
of monotonic growth in each lifetime. The final covered lifetime ends with
the confirmed memcg OOM at 14:55:28-29 described below. Fifteen service exits
are not fifteen crashes: requested restarts and watcher actions also occur.

Thus hours of successful paper publishing coexist with low mean database CPU,
large memory peaks, interruptions and an actual memory-limit failure.
**[INFERENCE]** Average CPU alone would miss the main observed stability risk.
This does not establish whether the memory is useful cache, leakage, query
state or engine-history overhead.

### Application versus database, and what cannot be reconstructed

`go-choir-corpusd.service` is the Go application, not corpus Dolt. It also serves
control/tape operations; its entire resource bill cannot be assigned to corpus
ingestion. At 15:48:48 its cgroup reported 786,501,632 bytes current memory,
2,169,098,240 bytes peak and 17,544.855 CPU-seconds since October 28 21:56:14.
Those are application-lifetime counters, **not six-hour deltas**. Guest model,
extraction and app work are another resource domain.

The retained material does **not** provide a matching six-hour control CPU/I/O
series, continuous corpus RAM/pressure measurements, or per-device historical
I/O deltas. Control did not exit in this window, so its lifetime total cannot
be allocated to these hours. Formatted systemd disk totals mix RAID layers as
explained below and are not a substitute. No six-hour control/corpus resource
ratio, bytes-per-document efficiency or replacement-engine ranking is claimed.

For a complete matched comparison, retain lightweight counters across several
hours of verified active publishing: control DB, corpus DB, corpusd app, guest
and maintenance separately; track restarts, publication/capture outcomes,
array-layer I/O, memory composition/pressure and configuration changes.
The retrospective evidence establishes active use and instability, but does
not invent the missing time series.

## Current resource observations

### Instantaneous memory composition, not workload characterization

At 15:35:26, control memory.stat had anon 3,946,381,312 bytes and file
253,665,280 bytes; corpus had anon 4,994,883,584 bytes and file 1,432,629,248
bytes, plus their kernel charges. Control's file-cache refault counter was
728,712,186 over the current cgroup lifetime. Current pressure averages were
low in that snapshot. **[INFERENCE]** The small file-cache share and large
refault/read counters are consistent with repeated cache churn; they do not
identify which queries, commits, GC or backups caused the reads.

### I/O accounting trap: do not count the RAID layers twice

`lsblk` and `/proc/mdstat` show md127 as a RAID1 over two NVMe devices.
At 15:35:26 control io.stat included:

```text
259:1 rbytes=2509582573568 wbytes=110152270848
259:0 rbytes=535866200064  wbytes=110152270848
9:127 rbytes=3046021492736 wbytes=110324637696
```

The member read counts sum to the md127 read count; each member also carries
roughly a mirrored write copy. Summing all devices mixes levels: roughly
2x the array read count and 3x the array write count in this sample.
`systemctl show` I/O totals and systemd's formatted lifetime journal lines
can therefore be misleading if called physical device work without specifying
the layer. Report array I/O separately from aggregate member-device writes;
SSD internal amplification is not measured here.

The main control process had also accumulated approximately 3.048 TB in
`/proc/<pid>/io` read_bytes at 15:36:57, corroborating substantial reads
without summing the RAID layers. This is a current-process cumulative counter,
not an instantaneous rate or proof of a specific SQL query.

### Disk and backup accounting

Metadata-only du reads at 15:36-15:38 gave the following. GiB conversions use
KiB / 1,048,576; measurements are nearby snapshots, not atomic.

| Path/category | du-accounted KiB | GiB |
| --- | ---: | ---: |
| Active control repository, platform-dolt/platform | 56,267,968 | 53.661 |
| Control oldgen subset | 55,962,444 | 53.370 |
| Local control backup, platform-dolt/.choir-backup | 54,285,020 | 51.770 |
| Active corpus repository, corpus-dolt/corpus | 15,446,660 | 14.731 |
| Corpus oldgen subset | 14,525,352 | 13.852 |

A top-level control-directory read was 110,494,220 KiB because it includes
the local backup. Do not report it as the active database. Do not sum the
backup and active repository as unique btrfs physical consumption either:
shared-extent accounting was not measured. Oldgen includes retained reachable
committed data; its size alone does not distinguish current rows from
superseded versions. No live-data census was obtained here.

One broader artifact-directory du was stopped by its 20 s deadline; it was
not repeated. Subsequent calls measured only the known database components.
Overlapping parent/child operands in a single GNU du invocation suppress
already-counted child bytes; the active totals above use separate invocations,
not the resulting misleading small parent remainder.

At 15:35:26, `df -B1` reported root size 510,362,910,720 bytes, used
487,013,462,016 bytes, available **14,370,099,200 bytes**, **98% used**.
This is operational risk, not authorization to delete data or run maintenance.

## Recent history reconstructed from retained journals

`journalctl --list-boots` exposed a retained window starting October 27
09:28:09 UTC, despite host uptime of about 6d19h. Archived/active journals
occupied about 290.6M. Earlier history below comes from dated repo receipts,
not newly recovered raw journals. No active local Prometheus, node-exporter,
Grafana, VictoriaMetrics, Netdata or sysstat service was found by the inspected
unit names, listener list and Nix search. This does not rule out external or
custom-named monitoring.

### Daily Store A audits

```text
2026-10-28T00:15:19Z platform: commits=116872 oldgen=51964004KiB noms=52413480KiB
2026-10-28T00:15:20Z corpus:   commits=13382  oldgen=12632528KiB noms=12850132KiB
2026-10-29T00:06:18Z platform: commits=123730 oldgen=53839992KiB noms=54056468KiB
2026-10-29T00:06:18Z platform history audit FAILED: oldgen 53839992KiB > 52428800KiB
```

Over 23.85 hours, control added **6,858 Dolt commits** and **1.789 GiB of
oldgen** (about **1.80 GiB/day** over this interval). These are Dolt snapshot
commits, not semantic events or SQL transaction counts. The intervening
failed squash restored its backup; it did not successfully reset the series.

The audit script exits on Store A's threshold failure before visiting Store B
(`scripts/dolt-history-audit.sh:58-60`, loop through line 70). Therefore **the
missing October 29 corpus audit is not evidence of zero corpus growth**.
The current cached corpus commit count, 13,382, is stale from October 28.
No new expensive dolt_log count was run to fill that gap.

### CPU, maintenance and interruptions

Two prior long process lifetimes ended October 28. Units below preserve the
journal's spelling; they are not all converted to an assumed GB/GiB standard.
The I/O totals mix the device layers described above.

| Lifetime ending | Store | Wall time | CPU time | Mean cores, derived | Memory peak, as logged |
| --- | --- | --- | --- | ---: | --- |
| Oct28 05:50:19 | Control | 3d21h49m32.114s | 1d23h29m46.165s | 0.506 | 4.1G |
| Oct28 05:55:33 | Corpus | 5d3h45m43.640s | 6h22m31.854s | 0.052 | 5G |

These are different windows and include different useful/maintenance work.
They establish substantial historical control CPU, not a fair relative
capacity or unit-cost ranking. Logs recorded raw I/O of 11.7T read/596.8G
written for control and 742.1G read/194G written for corpus; they must not be
mistaken for unambiguous member-device totals.

Control's liveness probe aborted it at 05:50, 06:02 and 06:13 on October 28.
It was stopped for the squash attempt at 11:52; the squash exceeded its 10G
cgroup limit, restored the backup and restarted Store A at 11:57:10. The
maintenance unit's own log reports 10G peak, 2m19.811s CPU and 5m5.874s wall.
It is a separate cost from the long-lived database service.

In the retained extract, 71 completed control auto-GCs had median **9.97 s**,
maximum **30.73 s**. Thirty-five completed corpus auto-GCs had median **5.86 s**,
maximum **730.61 s**. These are completed-operation wall durations; they
exclude aborted/incomplete work and do not measure total maintenance CPU.
The two slow corpus completions were October 29 03:44:13 (12m10.609s) and
04:47:42 (10m1.707s), during the previously recorded pressure incident.

**Twenty-six corpus service starts** were recorded on October 29 through the
15:35 extract, first at 02:15:29, last at 14:55:33. These are NOT 26 crashes:
many are explicit requested stops/restarts. The 02:15 restart follows three
failed liveness probes; the 04:54 restart is in the existing mitigation
receipt. The remaining stop callers were not all attributed.

At **14:55:28**, the kernel explicitly reported CONSTRAINT_MEMCG and killed
corpus PID 3930472: anon-rss=7,319,380kB, file-rss=72,436kB. The service then
reported `Result=oom-kill`, 7G peak and restarted at 14:55:33. This is a
confirmed cgroup OOM, unlike a merely observed SIGKILL with unknown sender.
Current policy is MemoryHigh=6,979,321,856 and MemoryMax=7,516,192,768 bytes
for corpus; control MemoryHigh=4,294,967,296, MemoryMax=infinity.

Control's 8.00 GiB current-lifetime cgroup peak is confounded by backup:
the October 29 backup log explicitly raised its MemoryHigh from 4G to 8G
at 02:34:45 for sync, then restored 4G at 02:37:41. It is not proof normal
foreground control operations need 8 GiB. The backup unit separately ran
02:34:43-03:28:11 with 8.9G logged peak and 59m10.509s CPU. Its impact must
not be charged solely to the database process or ignored as "not ingestion."

### Earlier receipts and their discontinuities

- [August 26 history incident](../archive/evidence/platform-dolt-oldgen-218g-dead-history-2026-08-26.md):
  225G repository, approximately 4.5G current chunk state, 6.9M Dolt commits;
  a SQL dump attempt reached 27.3G peak and OOMed. **This predates the A/B
  split and contained corpus rows**; it is evidence about the old shared
  store/history mechanism, not a control-only workload baseline.
- [October 5 corpus](../archive/problems/world-wire-corpus-resource-burn-2026-10-05.md):
  99 GB, 10.2 GB RSS, millions of objects/fetches/ingestion rows and sustained
  CPU. The report's claim that it shared control authority used stale
  taxonomy; the later code/deployment evidence places control separately.
- The October 9 corpus reset breaks the size history: the old 104G repository
  and history-free 23 GB dump must not be joined smoothly to later small-store
  readings. See the [reset receipt](../archive/problems/node-b-disk-free-space-2026-10-09.md)
  and [October 9 memory/config receipt](../archive/problems/node-b-memory-overcommitted-swap-2026-10-09.md).
- [October 15 control clustering](../archive/problems/dolt-clustering-assessment-2026-10-15.md):
  about 25G directory, 92,290 commits, oldgen growth about 1.6G/day, stalls and
  substantial service I/O. The precise stall mechanism was not proven then.
- [October 22 control growth](../archive/problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md):
  about 36 GiB, almost all oldgen. Large new archive files may be conjoin
  repacking; their file sizes cannot be read as newly ingested bytes.
- [October 24 control pressure](../archive/problems/platform-dolt-gc-throttled-at-memory-high-2026-10-24.md):
  a 3G high limit, little file cache, heavy refault/reads and GC convoys;
  a later GC at the same cap succeeded, so pressure alone was not sufficient
  to explain every stall. Current 4G/high and 3GiB Go soft-limit settings
  differ from that receipt.
- [October 28 control squash](../archive/problems/store-a-weekly-squash-2026-10-20.md):
  50.89 GiB, 120,335 commits before maintenance; OOM at 10G and rollback.
- October 29 corpus front outage:
  fast listing stage but failed object/edge reads, memory pressure and runtime
  mitigation. The new kernel receipt above extends that record with an
  actual later OOM; it does not settle leak versus cache versus workload.

## Corpus workload: cached counters, not new scans

GET `http://127.0.0.1:8086/health` returned status/store ok. Its retrieval
counters started at October 28 21:56:14 (corpusd restart), independently of
Dolt restarts. Selected counters at approximately 15:40-15:41:

| Operation | Calls | Errors | p95 bucket upper bound | Maximum |
| --- | ---: | ---: | ---: | ---: |
| get_object (PK) | 291,717 | 117 | 5 ms | 60,001.65 ms |
| list_edges_by_kind | 92,211 | 21 | 5 ms | 26,704.85 ms |
| front_publications | 8,322 | 13 | 50 ms | 42,673.44 ms |
| capture_index_write | 4,178 | 0 | 50 ms | 104,452.53 ms |
| capture_search | 688 | 11 | 1,000 ms | 60,000.69 ms |
| list_objects | 236 | 14 | 60,000.93 ms | 60,000.93 ms |

These metrics cover named operations, not all SQL or all work per request.
Bucket bounds are not exact percentiles. A fast front_publications stage does
not mean a complete front-page response is fast; hydration/graph reads follow.
Calls are not unique documents ingested, and their errors need not be distinct
user-visible failures. This is a mixed retrieval/ingest workload, not an
append-only bulk-loader workload.

Cached tape counters in the same response reported segments_mode=write,
578,208 segment pins, 5,252 legacy pins, 0 segment fallbacks and 80 created
segment files. Pins are not a semantic event count. This illustrates another
separate resource tier: tape/artifact bytes outside either SQL store.

## Interpretation and next safe measurements

The working model should be **small current control state + growing audit/
receipt history + physical engine history**, versus **growing corpus metadata,
content and derived retrieval indexes**. Control may be easier to shard by
computer, but every active computer's accepted event currently visits it.
Corpus scaling depends on input bytes, documents, processing work, query fan-out,
index freshness and retention. Neither is explained by registered-user count.

**[INFERENCE]** Eliminating unused Dolt history and controlling payload/index
copies should reduce unnecessary work, but these observations cannot tell us
how much of today's control size is required live receipts versus obsolete
versions. Likewise they cannot predict the memory floor or ingestion capacity
of SQLite, InnoDB, PostgreSQL or an LSM replacement.

Next evidence, without turning this into a production stress test:

1. A captured-base table/index census separating live receipt rows, small
   current rows, obsolete/pre-split data and Dolt history; account for local
   backup extents separately. Use a reviewed, memory-capped offline procedure.
2. Stable query/transaction fingerprints and matched resource windows during
   ordinary work and maintenance. The app user's PROCESS privilege refusal
   is not a reason to bypass access controls or run unrestricted scans.
3. Retained lightweight time-series counters and restart/config annotations,
   with a consumer for audit failures; the current audit's first-store failure
   suppresses the second-store sample.
4. In the eventual offline comparison, preserve event-head/receipt/idempotency
   correctness, independently operate control/corpus services with the same
   default technology, and score corpus against sustained growth with complete
   query/index/backup costs. Do not extrapolate from the excluded idle snapshot,
   treat the retrospective window as a capacity test, or classify every
   repeated restart as an engine crash.

No repair was attempted. The ongoing disk-headroom risk, control audit
failure, failed squash and corpus OOM are explicit residuals, not hidden by
an engine recommendation.

## Fleet-scale sensitivity: 100,000 continuously active computers

Owner scenario, not a demand forecast or measured engine capacity: 100,000
computers, four agents per computer, each averaging 50 generated tokens/second,
plus 1,000-10,000 times the present Autopaper ingestion workload. This means
400,000 agents, 20 million generated tokens/second and 1.728 trillion/day.
If 50 tokens/second instead means decoding speed only while a model is
generating, multiply by generation duty cycle; do not apply that discount
again if it is already the full-day average.

### Control scales with accepted acts, not accounts or tokens directly

Let A be active agents and e accepted computer events per agent-second.
Then event acceptance demand is A * e, plus lifecycle/security/projection
work. Each accepted event currently advances a per-computer head and adds a
receipt in one transaction. This is not merely an infrequently updated fleet
registry. Event bodies and tape bytes remain a separate storage domain.

Sensitivity points below are **assumptions, not measurements**. The 1,000-byte
retained receipt assumption is illustrative, not a census of current rows.

| Assumed accepted events/agent-second | Fleet events/second | Events/day | Receipt bytes/day at assumed 1,000 bytes/event, decimal TB |
| --- | ---: | ---: | ---: |
| 0.01 | 4,000 | 345,600,000 | 0.3456 |
| 0.1 | 40,000 | 3,456,000,000 | 3.456 |
| 1 | 400,000 | 34,560,000,000 | 34.56 |

These bytes exclude indexes, transaction logs, replication, backups, event
bodies and amplification; compression and deduplication are also not assumed.
Removing Dolt history removes one growth mechanism, not legitimate receipts.
At the middle point, a year of full receipt retention is approximately
1.26 PB before those other costs. Retention cannot be shortened without
preserving the actual replay, idempotency, audit and verification contracts.

Tokens do not determine e. For example, assuming all generation occurs in
1,000-token turns gives 20,000 fleet turns/second; 10,000-token turns gives
2,000. Neither establishes events/turn. Tool calls, streamed-record granularity,
semantic commits, retries and checkpoint/projection activity must be measured.
These turn examples are not statements about current event emission.

**[INFERENCE]** ComputerID is a natural partition-key candidate for the ordered
head/receipt path, because ordering is per computer rather than one global
fleet sequence. This is not proof all control transactions partition cleanly.
Four agents on one computer share its chain; an exceptionally intense computer
can hit a single-chain serialization limit even while fleet shards have spare
capacity. Engine throughput alone does not resolve that limit.

### Corpus has independent ingest and agent-demand axes

Let I0 be a verified baseline of unique accepted provider input, M its growth
factor, q agent corpus reads/second and w agent corpus writes/second. Corpus
must serve M * I0 provider inputs, A * q agent reads and A * w agent writes,
plus their graph/index/materialization work. Track input bytes as well as
documents; retries, source duplicates, reindexing and repeated publication
calls are not unique new input.

| Assumed corpus reads/agent-second | Fleet corpus requests/second | Response traffic at assumed 10,000 bytes/request, decimal GB/second |
| --- | ---: | ---: |
| 0.01 | 4,000 | 0.04 |
| 0.1 | 40,000 | 0.4 |
| 1 | 400,000 | 4 |

These are logical service requests, not SQL-operation counts or external
network egress. A response may require multiple object/edge/index reads.
Search complexity, selectivity, result size and hydration fan-out matter more
than a naked requests/second figure. Authorized immutable-object cache hits
can avoid origin work; cache keys and invalidation cannot leak private or
provider-restricted content or bypass revocation.

Agent writes are not free because reads dominate: one corpus write per agent
per 1,000 seconds already means 400 writes/second, before provider ingestion
and downstream indexes. Provider ingestion may need deduplication, extraction,
relationship/index updates and later reprocessing; source-document rate is
not its total write rate. Current publication and capture_index_write counters
do not establish a unique provider-input baseline, so no numerical 10,000x
capacity claim is inferred from them.

**[INFERENCE]** Corpus is likely harder to distribute than a cleanly
computer-partitioned receipt stream: shared objects, cross-computer discovery,
global indexes and popular keys do not follow one computer's ownership.
Bulk ingest and interactive agent reads also have different latency and
maintenance requirements. This is a reason to evaluate isolation and query
shape, not to assume PostgreSQL, MySQL or MariaDB automatically supplies a
global search system or transparent write sharding.

### Implications for selection and measurements

- Both tiers can require clusters well before 100 million users. Control's
  simpler query shapes do not imply negligible write/retention volume.
- Keep the same-engine default and independent control/corpus services, but
  evaluate control's ordered acceptance separately from corpus ingestion and
  retrieval. One engine family need not mean one topology or identical indexes.
- Current corpusd fronts both stores. Separate database processes alone do
  not prove end-to-end fault isolation: shared application resources and
  dependencies remain possible coupling points.
- Multi-region control must specify authoritative ordering, partition behavior
  and acknowledgement durability; corpus must specify object availability,
  permission freshness, index visibility and acceptable search lag. Replica
  count and an engine label do not settle those semantics.
- Next measurements: accepted events and retained bytes per active agent-hour;
  events per turn/tool act and their burst distribution; per-computer skew;
  corpus reads/writes, cacheability and fan-out per agent-hour; unique provider
  bytes/documents and downstream amplification; recovery/maintenance behavior
  at the resulting retained dataset size. Use real workload mixes, not an
  assumed constant conversion from tokens to SQL.

## Reproduction receipts

Read-only commands used (no secrets in these examples):

```sh
ssh node-b 'systemctl show go-choir-platform-dolt.service go-choir-corpus-dolt.service -p ActiveState -p ExecMainStartTimestamp -p MemoryCurrent -p MemoryPeak -p MemoryHigh -p MemoryMax -p CPUUsageNSec -p IOReadBytes -p IOWriteBytes -p ControlGroup'
ssh node-b 'journalctl --list-boots --no-pager; journalctl --disk-usage'
ssh node-b 'lsblk -b -o NAME,MAJ:MIN,TYPE,SIZE,FSTYPE,MOUNTPOINTS; cat /proc/mdstat'
ssh node-b 'journalctl -u go-choir-platform-dolt -u go-choir-corpus-dolt --since "2026-10-27 00:00:00 UTC" --no-pager -o short-iso --grep "Consumed|Started |Failed with result|Successfully completed auto GC"'
ssh node-b 'journalctl -u go-choir-platform-dolt-history-audit --since "2026-10-27 00:00:00 UTC" --no-pager -o short-iso'
ssh node-b 'journalctl -k --since "2026-10-29 14:54:00 UTC" --until "2026-10-29 14:56:00 UTC" --no-pager -o short-iso --grep "oom|Out of memory|Killed process|Memory cgroup"'
ssh node-b 'journalctl -u go-choir-corpusd.service --since "2026-10-29 09:00:00 UTC" --until "2026-10-29 15:00:00 UTC" --no-pager -o json --grep "corpusd retrieval op=|computer-1355b83bced1613eb048ee28d66688a8|publish|ingest|source"'
```

For publication verification, a read-only streaming parser examined only
`console.log` and `.1` through `.4` under
`/var/lib/go-choir/vm-state/vm-1669020adcece51c2f5ff9b6df399ef5/`.
It extracted date/hour and counted `story publish: ... published vN (revision ...)`
success markers, excluding `not published` and gap messages. The path follows
VM_STATE_DIR in `nix/node-b.nix` and consoleLogPath in
`internal/vmmanager/manager.go`; these are bounded rotated files, not a scan
of the guest data image. Counts are limited by retention, not an exact
database census.

The superseded passive sampler read memory.current, cpu.stat and io.stat twice,
15:39:18-15:39:48, before the timed-out receipt query. It found control using
11.014 CPU-seconds with 1,046.395 MiB md127 reads, versus 0.050 CPU-seconds and
zero md127 reads for corpus. This is retained solely as provenance for the
correction, **not decision evidence or an active-workload comparison**.

The raw service lifecycle/GC extract is also in this session's artifact://73;
the audit/probe/squash extract is artifact://74. Artifacts are supplementary
session diagnostics; dated selected values, commands, units, source semantics
and limitations above are the durable receipt.
