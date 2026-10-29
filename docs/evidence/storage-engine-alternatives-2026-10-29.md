# Storage-engine alternatives: lifetime resources before convenience (2026-10-29)

Status: comparative research and review perspectives, not a selection or an
implementation design. Mutation class: green. No runtime code/configuration,
production queries, engine prototypes or performance benchmarks were run for
this comparison. This does not authorize the migration or change the active
goal's acceptance gates. The owner requested that alternatives stay open:
"a bit more setup at first is fine if it improves resource utilization or
stability longterm."

This extends the [selection mission](../definitions/choir-data-architecture-selection-2026-10-28.md),
[initial direction](../design/data-rearchitecture-2026-10-28.md),
[previous consensus](data-architecture-consensus-2026-10-28.md), and
[portability inventory](portability-inventory-and-verification-2026-10-28.md).
Three independent read-only research slices covered server engines, COW engines
and LSM engines; the author inspected the cited repository paths and primary
sources for the central claims below. This is synthesis, not a new consensus
vote. Upstream documentation is mechanism evidence, not a Choir benchmark.

Subsequent owner clarification: prefer the **same server technology** for
control and corpus, but **separate independently operated services**, initially
co-located and later separable as corpus grows. Corpus requirements drive that
shared-engine default. Security first, stability second, scalability and
simplicity jointly next; do not add an engine just to optimize control alone.
The [read-only resource investigation](control-corpus-resource-profile-2026-10-29.md)
maps control's actual work and reconstructs hours of active paper publishing.
It is separate from this mechanism comparison; its initial 30-second idle
sample is excluded from workload characterization and selection evidence.

## 1. What the first review changes

SQLite in guests and PostgreSQL on the host remain plausible defaults, not
winners. MySQL/InnoDB and MariaDB/InnoDB are not merely cheaper migrations;
LMDB, MDBX, bbolt, RocksDB and Pebble are not rejected for lacking SQL. Additional
setup and application code can be worthwhile if sustained resource use and
failure behavior improve. Count their continuing correctness and maintenance
cost, rather than treating initial development effort as a veto.

The decision has separate axes: semantic ownership; transaction/recovery
boundaries; physical layout; engine; and deployment isolation. An engine name
answers only part of it. A computer's single semantic writer is not a single
writer for all computers on the host.

### Repository facts carried into the comparison

- Texture history follows content-addressed revisions and parent pointers,
  not Dolt history (`internal/store/texture.go:1235-1288`). The earlier
  investigation found no production branch/merge/AS OF product path. However,
  `internal/computerversion/dolt_table_hashes.go:32-82` uses Dolt table roots
  for the witness. Removing Dolt requires replacing that contract.
- The guest graph is not just Get/Put: `internal/objectgraph/dolt_store.go`
  has kind/owner/computer filters, generated metadata keys, ordering,
  tombstones and edge queries. `newest_headers.go:15-22,83-99` has bounded
  ordered/keyset reads. `derived_keys.go:16-37,62-91` already maintains
  secondary key families transactionally. This makes a purpose-built KV
  layer credible, but does not prove every guest SQL consumer fits it.
- Graph IDs are string primary keys; secondary indexes are numerous
  (`internal/objectgraph/dolt_store.go:21-48,278-340`). Host retrieval adds
  kind/owner/computer/time and reverse-edge indexes
  (`internal/platform/retrieval_schema.go:208-211`). Key width, ordering and
  index coverage are material resource inputs.
- Local prepare, host canonical-head CAS and local finalize are separate
  steps (`internal/computerevent/appender.go:756-790`). Finalize commits
  projected rows and their watermark together
  (`internal/store/computer_events.go:178-243`). No replacement may sever
  that atomicity or assume the cross-system steps became one transaction.
- Guest actor recovery uses another SQLite file
  (`internal/actorruntime/adapter.go:105-122`). The portability inventory
  also identifies files, keys and direct SQL state outside complete replay.
  "Everything else is derived" is not an established description today.
- The two host Dolt processes deliberately isolate control from corpus
  maintenance (`nix/node-b.nix:1148-1197`). The
  [October 29 incident](../problems/corpus-dolt-throttled-at-memory-high-front-times-out-2026-10-29.md)
  records severe corpus pressure while platform Dolt remained below its own
  pressure threshold. Shared host pressure and application dependencies
  still caused an outage. These are existing receipts, not new measurements.

Immutable data is not intrinsically a bad fit for SQL. Unnecessary retained
version history, duplicated large payloads and unbounded operations are the
problems. Small immutable metadata can remain SQL rows. Likewise, "stored
once" means one canonical home: replicas, bounded caches and independent
backups are useful copies, not violations by themselves.

## 2. Mechanisms: where each engine pays

| Family | Foreground work | Deferred or retained work | Main challenge under sustained churn |
| --- | --- | --- | --- |
| SQLite WAL B-trees | Changed pages appended to WAL; SQL/index work | WAL checkpoint; old readers retain required WAL; FTS5 merges if used | Bound writer transactions, readers, WAL and query workspaces |
| PostgreSQL heap plus indexes | New tuple versions, indexes and WAL | Pruning/vacuum, checkpoints, full-page images, retrieval-index maintenance | Keep cleanup ahead of churn; cap concurrent query/maintenance work |
| InnoDB clustered B-trees | Row/index changes, redo and undo; binlog when required | Purge, dirty-page flushing, doublewrite, FTS maintenance | Bound undo history and flushing debt; account for wide clustered keys |
| LMDB/MDBX/bbolt COW B+trees | Copy changed pages and publish new roots | Old readers delay page reuse; freelist/reuse work; compact-copy if needed | Bound reader lifetime, dirty pages, fragmentation and growth/open costs |
| RocksDB/Pebble LSM | WAL and memtable writes | Flush and SST compaction; retained versions/files; compressed reads | Keep compaction debt stable under the admitted memory/CPU/I/O budget |

"No GC" is not a sufficient criterion. Reclamation may move from a tracing
pass into page reuse, purge, vacuum, checkpoints or compaction. The questions
are how much work is required, what blocks it, whether it can stream/spill,
and what happens to serving latency and free space when it falls behind.
SQLite FTS5 itself merges index segments: using SQLite does not remove all
merge-style maintenance [S3]. No universal write-amplification ratio follows
from this table.

## 3. Host: PostgreSQL versus the two InnoDB products

Evaluate control and corpus separately. The control tier is small,
latency-sensitive transactional CAS/leases/receipts. Corpus combines ingest,
publication metadata, graph adjacency, metadata filtering and retrieval.
One engine may win both, but that is an outcome, not a premise.

### Oracle MySQL/InnoDB

**Positive case.** InnoDB clusters row data by primary key. Point-addressed
control records can be reached without a separate heap lookup. Composite
keys can make adjacency/range access local. The buffer pool is configurable;
direct data-file I/O can avoid a second OS data-page cache where the platform
and filesystem support it. These are potential ongoing resource benefits,
not just SQL-porting savings [S4-S6].

**Costs and risks.** Every secondary-index entry carries the primary key.
Choir's long canonical IDs can multiply index bytes and reduce fan-out.
Short internal keys are a legitimate experiment, but include the extra unique
canonical-ID mapping, lookups and invariant costs. Do not assume a literal
Dolt schema port is optimal [S4].

InnoDB still has undo and purge. Long read snapshots can prevent old versions
from being reclaimed; purge lag can lead to delays on writes [S7]. Durable
I/O includes redo, data/index pages, doublewrite protection and binary logging
when the selected PITR/replication design needs it. Direct I/O does not replace
doublewrite or make writes durable without synchronization [S6]. The buffer
pool does not cap all server memory: connection/query buffers, temporary
work, instrumentation and maintenance count too [S5].

MySQL supplies native binary JSON and targeted JSON indexes, plus InnoDB
FULLTEXT; graph tables do not require a dedicated graph engine [S8-S9]. It
must pass actual language/tokenization, ranking, freshness and deletion
requirements, not merely insert text quickly.

**Could win if:** clustered access and configured cache/I/O produce lower
whole-system memory, CPU or write bytes with equal transactional correctness
and retrieval semantics. **Loses that case if:** wide-key/index duplication,
purge debt, FTS behavior or additional required services erase the savings.

### MariaDB/InnoDB is a distinct candidate

It shares the clustered-index/undo model, not all Oracle MySQL behavior.
MariaDB's community thread pool is a real alternative for resource-conscious
self-operation; it can reduce active-thread overhead, but can also queue a
simple query behind load. Bursty and long non-yielding workloads require
special attention. It is not a hard concurrency/memory bound [S10].

Its I/O defaults and controls differ by version: Unix default moved to
O_DIRECT in 10.6, and 11.0 deprecated the old flush-method variable in favor
of separate buffering/write-through controls. Direct I/O can reduce double
caching, but buffered reads may be beneficial on some workloads; confirm
actual settings and filesystem behavior [S11].

MariaDB JSON is a LONGTEXT compatibility alias, unlike MySQL's binary JSON.
Indexed generated fields may make parsing cost immaterial on hot paths; an
unindexed JSON-heavy workload may behave differently. Same protocol does not
imply identical JSON, collation, DDL or replication semantics [S12].

**Could win if:** its thread scheduling, I/O choices, physical backup tooling
and complete indexed workload give a smaller or steadier self-hosted system.
**Loses that case if:** query semantics, repeated parsing, purge or maintenance
behavior make the full deployment less efficient or predictable.

### PostgreSQL's strong case, and its liabilities

SQL joins, jsonb indexing, full text and a broad operational ecosystem can
avoid custom query machinery or another retrieval service. This is a strong
corpus argument, not proof of the smallest resource footprint [S13].

Updates create new heap tuples. HOT can avoid new ordinary index entries only
under its eligibility conditions, including sufficient same-page space and no
disqualifying indexed-column change. Choir indexes updated_at: updates that
change that indexed field cannot simply be assumed to obtain HOT savings.
Pruning/vacuum, index design and update frequency must be measured [S14-S15].

Count shared_buffers and OS cache, concurrent per-operation work_mem,
maintenance, autovacuum and application result materialization. A configured
cache or work_mem value is not a total server-memory ceiling [S16]. WAL includes
full-page images for the first modification of a page after a checkpoint when
full_page_writes is enabled, not every row update. Frequent checkpoints can
increase those writes; WAL archive failures or lagging replication slots can
retain WAL beyond ordinary size targets [S17].

**Could win if:** native query/retrieval features meet the workload with fewer
components and lower total resources, while vacuum/index cleanup remain
stable. **Loses that case if:** equivalent maintained queries show worse
resource peaks, cleanup debt or serving tails than the InnoDB candidates.

### Recovery and replication are configurations, not engine labels

All three require correctly configured durable local commits and storage that
honors flushes. Compare the intended backup/PITR tool and a real restore.
Online DDL is operation/version-dependent; it can still require metadata locks,
a table rebuild and substantial temporary memory/disk.

Remote durability is separate from automated failover. MySQL semisynchronous
replication acknowledges relay-log persistence, not application of the
transaction, and can fall back to asynchronous mode on timeout [S18].
PostgreSQL's synchronous modes distinguish remote write, durable flush and
remote apply [S19]. Fencing and promotion of a sufficiently current replica
remain requirements. Do not silently weaken the acknowledgement guarantee
when replication becomes unavailable.

### Number of host instances

The review preference is **two independently budgeted instances: control and
corpus**, whichever server engine wins. Multiple logical databases/schemas
in one instance are useful permission/ownership boundaries, not equivalent
restart, WAL, physical-recovery or resource isolation. PostgreSQL explicitly
shares WAL across its logical databases [S20].

Compare this against one instance to quantify idle/duplicate-cache overhead
and interference, not to assume isolation is free. Separate processes on one
host still share its kernel, devices and failure domain. Reserve memory and
disk capacity and admit maintenance explicitly. Avoid an instance per daemon
without a recovery/security/resource reason. Identity can eventually use a
separate logical database in the control tier; migrating healthy auth SQLite
is not a prerequisite for retiring Dolt. Owner mail data must not be folded
into the public corpus to simplify an engine migration.

## 4. Guest: SQL versus a purpose-built transactional ordered map

### LMDB

LMDB is a serious small-state candidate: an embedded COW B+tree, one writer,
concurrent snapshots, atomic transactions across named subdatabases, mapped
reads and no WAL checkpoint pipeline [S21-S22]. Its potential advantages are
low execution-layer overhead and no separate engine block cache.

A large mapping reserves virtual address space, not equivalent physical RAM.
Touched pages still become resident. On Choir's current Firecracker setup,
guest file cache being internally reclaimable does not prove it is returned
to the host. Measure host VM RSS and guest anon/file residency after scans,
backups and quiescence. SQLite buffered I/O also populates OS cache; mmap is
neither an automatic memory leak nor an automatic fleet-memory saving.

The oldest reader prevents reuse of later-retired pages. A slow snapshot or
backup can grow the file while logical state stays constant. Map capacity is
explicit: exercise MAP_FULL, coordinated resizing, MAP_RESIZED and stale
reader cleanup. Key-size and duplicate-value limits also matter for long
composite graph keys; choose a format that fits the selected build [S22].

"Zero-copy" ends at ownership boundaries: mapped bytes cannot outlive their
transaction. Go binding choice determines copying and OS-thread requirements.
Keeping a transaction open merely to avoid copying can be more expensive
through retained pages. Compare the actual service API, not a naked Get.

**Could win if:** bounded named query paths and small mutable state permit
cheap atomic indexes and roots, with lower VM-wide resource use and no
unacceptable growth/backup stalls. **Loses that case if:** page-copy traffic,
reader retention, binding/copy costs or custom query work erase the advantage.

### MDBX / libmdbx

MDBX is a distinct LMDB-descended COW candidate, with growth/shrink geometry,
dirty-page spill controls and richer reader/page-reuse diagnostics. These
features could improve long-run variable-size and churn behavior. Its reuse
subsystem is explicitly GC; do not sell it as maintenance-free [S23].

Automatic growth is not a total memory bound. Eligible free-tail shrink does
not compact all interior fragmentation or override readers. Reader cancellation
policies must produce an explicit failure/retry, never a silently partial
snapshot. Evaluate a pinned C library and Go binding across target platforms,
fully durable sync mode, backup and reopen behavior [S23-S24].

**Could win over LMDB/SQLite if:** those controls demonstrably reduce peaks,
operational intervention and sustained bytes at equivalent behavior.
**Loses that case if:** reuse/geometry work or integration adds complexity
without a measured resource or stability benefit.

### bbolt

bbolt gives transactional ordered buckets in Go without a C binding. Its
single-writer/concurrent-reader model fits the computer. It deserves a trial
for small, query-stable current state [S25].

Its limitations are concrete: long readers retain COW pages; growth remapping
can wait on readers; random writes and large transactions can be costly; the
file does not shrink merely because rows were deleted. Compact-copy needs
source plus destination space and a consistent source view [S25].

Most importantly, writable open loads its freelist. NoFreelistSync trades
foreground savings for full-database freelist reconstruction during recovery;
the source explicitly documents that tradeoff [S26]. Persisted freelists still
have a size-dependent heap/open cost. The existing no-boot-whole-store-pass
and bounded-memory gates apply: do not quietly waive them because the engine
is Go-native. Investigate admitted-size and fragmentation behavior before
claiming it passes.

**Could win if:** its fully indexed transaction path, open and reclamation
cost remain within the hard gates and beat the alternatives in the actual
small-state envelope. **Loses that case if:** freelist/open memory or remap/
copy costs break those gates, even if point reads are fast.

### SQLite WAL, fairly configured

SQLite supplies query planning, indexes, constraints, SQL transactions,
ordering and FTS5. If query shapes remain broad or evolve with app kinds,
that can be both a resource and a correctness advantage. A KV implementation
must reproduce the required behavior, not all of SQL [S1-S3].

WAL is one writer with concurrent snapshot readers, not "no contention."
Old readers can prevent checkpoint completion/WAL reuse. The page-cache
setting is a per-connection/session cache target, not a whole-VM memory cap;
query workspaces, values, Go/native allocations and OS cache are additional
costs [S1-S2]. FTS5 has automatic segment merges, including potentially long
crisis merges [S3].

An ordinary rowid table with a non-integer primary key stores a separate
unique index. WITHOUT ROWID can avoid that lookup/duplication, but then
primary-key width affects secondary indexes. Test both layouts where
appropriate; do not compare an optimized KV encoding only to an accidental
SQL schema [S27].

**Could win if:** its complete query/index/recovery implementation uses less
total resource than a purpose-built layer. **Loses that case if:** a correctly
indexed, equally durable KV design gives materially better VM-wide peaks,
sustained bytes/CPU or serving stability at acceptable continuing complexity.

## 5. LSM engines: guest candidates and a strong corpus-index hypothesis

RocksDB and Pebble accumulate WAL-backed memtables, flush sorted immutable
SSTs and merge SSTs by compaction. Sorted batching, compression and SST ingestion
can reduce useful-work cost even with one semantic writer. One writer does
not remove the reason to evaluate LSM [S28-S32].

The strongest hypothesis is bulk **rebuildable corpus indexes** whose ingest
and storage costs dominate. That is distinct from making an LSM the authority
for control records, and does not imply the current corpus is wholly
rebuildable. Inventory authoritative publication/ownership state separately.

### RocksDB

RocksDB offers rich compaction, compression, transaction and backup options.
Leveled compaction generally trades more rewriting for lower read/space
amplification; universal compaction changes that tradeoff. These options can
matter enough to justify C++ bindings and tuning, not just packaging cost.
No fixed multiplier is transferable to Choir [S28-S31].

A block-cache budget is not total memory: memtables, indexes/filters, pinned
iterator blocks and temporary flush/compaction work count. Buffered reads can
also cache compressed SST pages in the OS while the block cache holds
uncompressed data. Column families can multiply budgets unless deliberately
shared/managed [S28].

Compaction debt eventually causes slowdown or write stops. Increasing stall
thresholds does not increase cleanup capacity; full disk can prevent the
compaction needed to recover space. A column-family stall can affect the whole
database [S29]. Test admission and disk reserve, not only peak ingest.

### Pebble

Pebble is a production-used Go-integrated LSM with snapshots, indexed atomic
batches, checkpoints and SST ingestion. It is not a feature-equivalent RocksDB:
its documented scope includes leveled compaction but not RocksDB transactions,
universal compaction or column families. Current v2+ formats are not directly
interchangeable with arbitrary RocksDB files; format upgrades can be permanent
[S32]. Go integration does not remove caches, compaction debt or allocator
tradeoffs.

An atomic batch is not isolated read-modify-write. With a single trusted writer,
serialize the entire read/decide/commit sequence and atomically include rows,
secondary keys, watermark and witness changes. Verify that every mutation path
obeys that rule. If independent writers are necessary, evaluate a proper
transaction layer or a transactional engine rather than claiming snapshots
make a prior read conditional [S31-S32].

### Checkpoints are not free backups

Immutable SSTs permit same-filesystem hardlinked checkpoints. They retain old
files as compaction produces replacements, so cheap checkpoint creation can
become retained disk and write traffic. Off-device backup must copy and verify
the data. A hard link to a live mutable SQLite database is not a snapshot.
Use each engine's documented backup/checkpoint mechanism and bind the result
to the exact tape head and logical root [S33-S34].

**An LSM wins if:** sustained compression/batching/ingestion savings exceed
compaction, reads, memory baseline, retained snapshots and application-index
cost while cleanup debt remains stable. **It loses if:** the budget needed
to avoid debt/stalls is worse than B-trees, or the complete query/recovery
implementation removes the resource advantage. No arbitrary per-VM minimum
memory or performance ranking has been established here.

## 6. More options and hybrids remain open

| Candidate | Positive case worth retaining | Evidence needed before promoting it |
| --- | --- | --- |
| Badger | Go transactional KV with value separation; potentially fewer rewrites of sufficiently large values | Remaining value-size distribution after CAS split; value-log GC/read cost and headroom; synced durability (upstream defaults SyncWrites false) [S35] |
| redb | Maintained Rust COW transactional KV; potential safety/resource tradeoff | Actual Go/Rust integration, target builds, indexed query and backup/reclamation cost; no transfer of upstream benchmark rankings [S36] |
| MyRocks | SQL over RocksDB; potentially useful SSD write/space savings on corpus | Pinned distribution/support, SQL/search/transaction limitations, backup and sustained compaction, including any required second search engine [S37] |
| DuckDB | Columnar SQL for bulk analysis and rebuildable analytical projections; may avoid custom scan/aggregation code | Actual transactional workload and memory peaks; chosen in-process or server deployment contract, retrieval freshness; not assumed incapable of server use [S38] |
| TiKV/TiDB | Replicated/sharded transactional storage when corpus scale justifies it | Full SQL/KV/control-plane deployment and compaction/replication cost, not standalone KV throughput [S39] |
| FoundationDB | Ordered distributed transactions and failure handling when those requirements dominate | SQL/index layer, cluster baseline, transaction/scan limits and operational proof [S40] |
| SQLite/libSQL family; embedded MySQL/PostgreSQL deployment | Preserve relational query machinery while varying packaging/replication | A concrete capability/resource gain over the baseline; do not count a remote SQL service as equivalent guest-local independence |
| Dolt-small / DoltLite | Incumbent control and content-root/versioning alternatives already in the goal | Same gates and lifetime maintenance accounting; no credit for unused version features; DoltLite maturity and semantics need their own proof |

A hybrid is legitimate: a transactional B-tree for current authority plus
immutable compressed retrieval segments or an LSM for derived indexes. It must
save more than the extra caches, binary/runtime footprint, operational work
and recovery joins cost. "Use one engine everywhere" is not a hard constraint;
"one canonical owner and one explicit recovery contract per data kind" is.
Neither this list nor a convenient default is a paper-cut rejection.

## 7. What KV actually adds, and what engine choice cannot remove

Do not build a generic SQL engine to evaluate KV. First inventory all readers
and define named query shapes. Ordered composite keys can implement kind/
owner/computer listing, timestamp order, metadata equality and forward/reverse
adjacency. Streaming intersections, primary fetches and bounded traversals may
be sufficient. The existing side-index implementation is evidence that some
hot paths already have this form, not proof that all do.

The application then owns exact key encoding/order, index maintenance on
update/delete/tombstone, uniqueness/CAS, pagination ties, aggregates, missing/
null/invalid JSON semantics, query cancellation, index backfill and compatibility
with older binaries. Every authoritative change must atomically update its
secondary keys and witness. Preserve arbitrary-query behavior where consumers
need it or explicitly migrate the consumer contract; do not silently drop it.

Physical B-tree roots, page hashes, file hashes and SST manifests are not
substrate-neutral logical roots. All candidates need the same defined logical
witness: canonical row encoding, schema identity, exclusions, atomic root
maintenance and an independent recomputation/audit path. Fast O(tables) reads
of maintained roots move work to mutation/schema transitions; they do not make
that work disappear or prove the root matches the rows. No new cryptographic
construction is selected by this memo.

Prefer one principal guest transactional store for tightly coupled current
state. Keep a separate actor/recovery file only with an explicit loss,
checkpoint and reconstruction contract; combining files alone does not solve
the host-CAS/local-finalize recovery protocol. Keys, files, actor obligations
and shared state must be included or explicitly excluded from the portable
unit. A source-unavailable restore is the decisive completeness test.

Start comparisons at equal power-loss durability: SQLite WAL FULL, synced COW
commits and WAL-enabled synchronous LSM commits, subject to the storage stack
honoring flushes. Relax durability only for a proven projection whose canonical
inputs were already durable. Process kill, VM/OS crash, host/storage loss and
partition are different fault classes. Asynchronous WAL shipping alone does
not give zero acknowledged-write loss after losing a host.

## 8. Decision evidence: measure complete systems over time

Keep host and guest scorecards separate; preserve the incumbent control. The
current reviewed goal's gates and survivor limits remain in force. This memo
expands research candidates, not the number of full-paper replays. Screen
mechanisms/integration requirements first, then choose representative candidates
for offline trials under those limits. Do not run every name on the paper.

Use two stages: (1) equivalent logical model, queries and retained data, to
attribute engine effects; (2) idiomatic physical layouts per engine, with every
extra mapping/index included, to compare attainable systems. A literal schema
port and an optimized KV design answer different questions.

Before measurement, record candidate-specific expectations and falsifiers in
the program's precommitment ledger. This memo states qualitative hypotheses,
not scored predictions or newly invented numeric budgets.

The evidence packet should contain:

1. Frozen base/tape head, actual object/value/key-width and query distributions,
   update/delete rates, indexes and retrieval correctness/freshness contracts.
2. Pinned engine, Go binding, architecture, compiler/allocator/codecs,
   filesystem and virtual-block settings; effective sync and replication
   settings, instance topology and admitted concurrency. Include the deployed
   x86 host and eventual arm64 target, not just the developer's machine.
3. End-to-end commit/query tails and throughput while performing checkpoint,
   backup, vacuum/purge/compaction/FTS maintenance and online schema/index work.
   Run through repeated maintenance cycles and catch-up; stable debt matters
   more than burst speed or maintenance postponed beyond the observation.
4. Empty/idle, warm, maintenance-peak and post-quiescence memory: application
   heap, native allocations, mapped/file cache, guest-wide memory and host
   Firecracker RSS. Do not add overlapping RSS/cache counters. Record cold-open
   and restore peaks, especially freelists/table metadata, not just steady state.
5. Device bytes and CPU per semantic event and indexed object; WAL/redo/binlog,
   page writes, index/root maintenance, compression, compaction and filesystem
   amplification. Record allocated versus live bytes, temporary disk and
   retained snapshot/backup space. Core SQLite lacks transparent general page
   compression; filesystem compression has costs too, so state the complete
   stack rather than crediting only an LSM's codec.
6. Deliberately stalled readers/iterators/backups, high fragmentation, large
   values, slow storage, ENOSPC, failed sync and map/geometry exhaustion; reader
   release/death and cleanup recovery. Verify no partial row/index/root or
   poisoned future write, and bound failure/refusal latency.
7. Process-kill and VM/OS/storage failure separately, plus restore/PITR and
   replica timeout/promotion/fencing where relevant. Compare acknowledged data,
   query answers, obligation fates, tape watermark and independently verified
   logical roots. No rerunning models/network observations during replay.
8. One versus two host instances under simultaneous corpus/control load:
   duplicate idle/cache cost versus maintenance, restart and saturation
   isolation. Roles/pools alone are not an isolation experiment.

Select on a lifetime Pareto comparison of correctness, memory peaks/density,
CPU, physical I/O, SSD wear, disk/backup retention, serving tails and continuing
operational complexity. More setup is acceptable; recurrent unbounded work is
not. A small benchmark win is not sufficient to justify a new engine, but
initial convenience is not sufficient to reject a durable resource win either.

## Primary-source ledger

Documentation checked during this research; latest-source behavior is not a
pinned release guarantee. Pin versions and recheck defaults before experiments.
Upstream performance claims and example benchmark numbers are not adopted.

- S1: [SQLite WAL, readers/checkpoints and WAL-reset fix](https://www.sqlite.org/wal.html).
- S2: [SQLite cache_size and synchronous](https://www.sqlite.org/pragma.html).
- S3: [SQLite FTS5 automerge/crisismerge](https://www.sqlite.org/fts5.html#special_insert_commands).
- S4: [MySQL 8.4 clustered/secondary indexes](https://dev.mysql.com/doc/refman/8.4/en/innodb-index-types.html).
- S5: [MySQL 8.4 memory use](https://dev.mysql.com/doc/refman/8.4/en/memory-use.html).
- S6: [MySQL 8.4 disk I/O and doublewrite](https://dev.mysql.com/doc/refman/8.4/en/innodb-disk-io.html).
- S7: [MySQL 8.4 purge](https://dev.mysql.com/doc/refman/8.4/en/innodb-purge-configuration.html).
- S8: [MySQL 8.4 JSON](https://dev.mysql.com/doc/refman/8.4/en/json.html).
- S9: [MySQL 8.4 FULLTEXT](https://dev.mysql.com/doc/refman/8.4/en/innodb-fulltext-index.html).
- S10: [MariaDB thread pool](https://mariadb.com/docs/server/ha-and-performance/optimization-and-tuning/buffers-caches-and-threads/thread-pool/thread-pool-in-mariadb).
- S11: [MariaDB flush method and newer controls](https://mariadb.com/docs/server/server-usage/storage-engines/innodb/innodb-flush-method).
- S12: [MariaDB JSON alias/semantics](https://mariadb.com/docs/server/reference/data-types/string-data-types/json).
- S13: [PostgreSQL JSON](https://www.postgresql.org/docs/18/datatype-json.html).
- S14: [PostgreSQL HOT](https://www.postgresql.org/docs/18/storage-hot.html).
- S15: [PostgreSQL vacuum](https://www.postgresql.org/docs/18/routine-vacuuming.html).
- S16: [PostgreSQL resource settings](https://www.postgresql.org/docs/18/runtime-config-resource.html).
- S17: [PostgreSQL WAL/checkpoints and retention](https://www.postgresql.org/docs/18/wal-configuration.html).
- S18: [MySQL 8.4 semisync and asynchronous fallback](https://dev.mysql.com/doc/refman/8.4/en/replication-semisync.html).
- S19: [PostgreSQL synchronous_commit](https://www.postgresql.org/docs/18/runtime-config-wal.html#GUC-SYNCHRONOUS-COMMIT).
- S20: [PostgreSQL database hierarchy/shared WAL](https://www.postgresql.org/docs/18/manage-ag-overview.html).
- S21: [Symas LMDB description](https://www.symas.com/lmdb).
- S22: [LMDB API, map/key/value limits, sync flags and transaction lifetime](https://raw.githubusercontent.com/LMDB/lmdb/mdb.master/libraries/liblmdb/lmdb.h).
- S23: [MDBX internals](https://libmdbx.dqdkfa.ru/docs/en/deep-dive/internals.html).
- S24: [MDBX durability modes](https://libmdbx.dqdkfa.ru/docs/en/getting-started/durability-modes.html).
- S25: [bbolt README and limitations](https://github.com/etcd-io/bbolt/blob/main/README.md).
- S26: [bbolt DB options and freelist](https://github.com/etcd-io/bbolt/blob/main/db.go).
- S27: [SQLite WITHOUT ROWID layout](https://www.sqlite.org/withoutrowid.html).
- S28: [RocksDB memory](https://github.com/facebook/rocksdb/wiki/Memory-usage-in-RocksDB).
- S29: [RocksDB write stalls](https://github.com/facebook/rocksdb/wiki/Write-Stalls).
- S30: [RocksDB compaction styles](https://github.com/facebook/rocksdb/wiki/Compaction).
- S31: [RocksDB operations, batches and sync](https://github.com/facebook/rocksdb/wiki/Basic-Operations).
- S32: [Pebble scope, production use and formats](https://github.com/cockroachdb/pebble/blob/master/README.md).
- S33: [RocksDB checkpoints](https://github.com/facebook/rocksdb/wiki/Checkpoints).
- S34: [SQLite online backup](https://www.sqlite.org/backup.html).
- S35: [Badger options, including SyncWrites](https://github.com/dgraph-io/badger/blob/main/options.go) and [value-log GC](https://github.com/dgraph-io/badger/blob/main/docs/quickstart.md).
- S36: [redb status and COW design](https://github.com/cberner/redb/blob/master/README.md).
- S37: [MariaDB MyRocks](https://mariadb.com/docs/server/server-usage/storage-engines/myrocks/about-myrocks-for-mariadb).
- S38: [DuckDB 2.0 concurrency, including server mode](https://duckdb.org/docs/2.0/connect/concurrency.html).
- S39: [TiDB/TiKV architecture](https://docs.pingcap.com/tidb/stable/tidb-faq/).
- S40: [FoundationDB architecture](https://apple.github.io/foundationdb/architecture.html) and [limits](https://apple.github.io/foundationdb/known-limitations.html).
