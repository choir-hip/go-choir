# When to snapshot an always-busy event-sourced SQLite store

Date: 2026-11-15. Every claim about an outside system carries a URL from this session. Anything labelled "derivation" or "inference" is mine, not a source's. Section 7 lists what I could not source.

## Summary

1. The one defensible optimum is the Young/Daly form. Replay debt grows quadratically with time since the last snapshot, so the interval that minimises (snapshot cost + expected replay cost) is T* = sqrt(2 C mu / (r lambda)). The online "break-even" rule (snapshot when accrued expected replay cost equals snapshot cost) gives the same answer (derivation, section 4). It depends on the restore demand rate r, which is a policy input and not a measurement.
2. With a 26-minute full capture, T* lands at roughly 20 to 54 hours for r between 1/day and 1/week. That is where the system is today. The trigger is not badly tuned. The cost of a snapshot, C, is what is wrong.
3. An RTO of 4 minutes is infeasible with full captures. The tail grows during the capture itself, so you would capture 30 to 60% of the time (section 4).
4. Literature and practice agree on one structure. Make the snapshot cost proportional to what changed (page-level deltas), keep the log tail short by continuous shipping, and re-base on a growth ratio. Then the trigger is a cheap RTO rule, not a delicate optimisation.
5. The 19% dedup is explained by chunk size, not by content-defined versus fixed chunking (inference, section 3). Chunks of 1 MiB or more are far larger than a 4 KiB B-tree page, and a write-heavy B-tree dirties pages scattered across the whole file.

## 1. Classical theory: Young and Daly

**Formulas.** Young (CACM 1974) gives tau_opt = sqrt(2 delta M). delta is the time to write a checkpoint and M is the mean time to interrupt. Daly (FGCS 22, 2006) gives a first-order form sqrt(2 delta (M+R)), where R is the restart time. His higher-order model finds that R drops out of the optimum. The practical rule is tau = sqrt(2 delta M) - delta when delta < M/2, and tau = M otherwise. The worst-case error of the Young form is under 5%, at delta = M/2.
Source: https://blog.acolyer.org/2015/06/10/a-higher-order-estimate-of-the-optimun-checkpoint-interval-for-restart-dumps/ (the Daly PDF at graal.ens-lyon.fr returned 404, so the formulas come from this secondary summary).

**Assumptions.**
- Total time is solve time + dump time + rework time + restart time.
- Interrupts have a mean time to interrupt M. Daly's paper models them as exponential, per the summaries; I did not read the paper.
- The first-order model assumes tau + delta << M and ignores failures during restart.
- Checkpoint cost is constant, and the checkpoint blocks the computation (delta is added to wall time).
- The objective is the minimum expected completion time of a finite job.

**What changes for our case.**
- *Failure is a demand.* Replace the failure rate 1/M by a restore/fork/migration demand rate r. The optimisation is unchanged if loss is measured as replay time, but the loss grows at lambda/mu replay-seconds per second, not 1. This gives T* = sqrt(2 C mu / (r lambda)), where C is in replay-second equivalents. This is my derivation; Young and Daly do not cover it. One of their assumptions fails here: the snapshot is not blocking (it is a short read transaction plus background hashing). C is therefore a weighted background cost, not wall time.
- *RTO objective.* Young/Daly minimise expected loss. An RTO is a hard worst-case bound, and the rule is deterministic: snapshot start-to-start interval T <= RTO mu / lambda - d, where d is the capture duration (derivation). This is the Postgres model: max_wal_size bounds recovery WAL, not expected loss.
- *Non-exponential demand.* A deploy or planned restart is a known moment, not a Poisson event. Take a snapshot just before it (section 4).

## 2. How production systems decide

| System | Trigger | Primary source |
|---|---|---|
| PostgreSQL | Whichever comes first of checkpoint_timeout (default 5 min) and max_wal_size (default 1 GB). Dirty-buffer writes are spread over checkpoint_completion_target (default 0.9) of the interval. Standbys take restartpoints, which can only occur at a primary checkpoint record. A shorter interval means faster recovery, but also more full-page WAL after each checkpoint. | https://www.postgresql.org/docs/current/wal-configuration.html |
| Postgres 17 incremental backup | WAL summaries track changed blocks. Relation files become incremental files holding only changed blocks. Restore needs pg_combinebackup and the whole dependency chain. The docs say it is worth it only for large, mostly static databases, because if most data changes the incremental is nearly as large as a full. | https://www.postgresql.org/docs/17/continuous-archiving.html |
| SQLite WAL | Automatic PASSIVE checkpoint when a commit leaves the WAL at 1000 pages or more. A checkpoint cannot finish while a reader is open, so a constant reader means the WAL grows without bound. Read cost grows with WAL size. The docs call this a read-optimised versus write-optimised trade-off. A wal_hook allows custom policy. | https://www.sqlite.org/wal.html |
| SQLite backup API | Copies in steps, releasing locks between steps. A write by another connection restarts the backup, and the docs warn it may never complete if restarts are frequent. VACUUM INTO and sqlite3_rsync are named as alternatives. | https://www.sqlite.org/backup.html |
| InnoDB | Fuzzy checkpoints: small batches of dirty pages flushed continuously, with a checkpoint LSN in the redo log. The doc page says large redo logs reduce checkpoint I/O. The adaptive flushing details came from the fetch tool's general knowledge, not this page, so I treat them as unverified. | https://dev.mysql.com/doc/refman/8.0/en/innodb-checkpoints.html |
| Redis RDB | "save N M" rules: every N seconds if at least M keys changed. fork() gives a copy-on-write child that writes the file, and the parent does no disk I/O. The docs warn fork can stall clients for ms to ~1 s on huge datasets. | https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/ |
| Redis AOF rewrite | auto-aof-rewrite-percentage (default 100) compares the AOF to its size after the last rewrite, with an auto-aof-rewrite-min-size floor (default 64 MB). 7.0 uses a base file plus incremental files and a manifest. 8.10 adds BACKUP START/SEAL, with immutable BASE and hard-linked INCR files. | https://raw.githubusercontent.com/redis/redis/unstable/redis.conf and the persistence page above |
| etcd (Raft) | snapshot-count default 100000 committed transactions. Compaction is separate and off by default. | https://etcd.io/docs/v3.5/op-guide/configuration/ |
| RocksDB | A checkpoint hard-links immutable SST files on the same filesystem and copies MANIFEST/CURRENT, so it is cheap. If the target is on another filesystem the SSTs are copied. | https://github.com/facebook/rocksdb/wiki/Checkpoints |
| LMDB | env.copy() makes a consistent copy inside a read transaction. Optional compact=True is slower. The page does not call LMDB copy-on-write, so I cannot cite that. | https://lmdb.readthedocs.io/en/release/ |
| Flink | Generalized log-based incremental checkpoints (FLIP-158): every state change goes to a durable short-term changelog. Materialisation of state happens in the background, default every 10 minutes, and the log is then truncated. In the post's benchmark (parallelism 50, 50K events/s), 99.9th-percentile checkpoint time fell from 10 s to 1 s. The costs were more storage and memory and slower recovery: 35-65 s versus 20-21 s. | https://flink.apache.org/2022/05/30/improving-speed-and-stability-of-checkpointing-with-generic-log-based-incremental-checkpoints/ and https://cwiki.apache.org/confluence/display/FLINK/FLIP-158:+Generalized+incremental+checkpoints |
| Litestream | Continuously ships WAL pages as LTX files. Compaction levels merge them (L1 every 30 s, L2 every 5 min, L3 every hour). A full snapshot is taken every 24 h by default, and retention is 24 h by default. Restore takes the latest snapshot and applies later files in order. | https://litestream.io/how-it-works/ |
| Pekko/Akka Persistence | snapshotEvery(N events, keepNSnapshots). A predicate can also snapshot on a domain event. Event deletion on snapshot is optional and off by default. Commands are stashed while a snapshot saves. | https://pekko.apache.org/docs/pekko/current/typed/persistence-snapshot.html |

Not covered, because I did not fetch a source: EventStoreDB, Axon, FoundationDB, CockroachDB, Kafka Streams.

**Pattern across all of them.** A large state is not re-snapshotted at a fixed cadence. Every system that holds large state either (a) makes the snapshot O(changed data) (Postgres 17, Litestream, Flink changelog, RocksDB links) or (b) forks a copy-on-write child (Redis). The triggers are a log-size bound (max_wal_size, snapshot-count), a time bound, and a growth ratio for rebasing (Redis).

## 3. Design space that makes the trigger mostly disappear

**Page-level deltas.** SQLite already has the dirty-page stream in its WAL. Litestream ships it continuously (https://litestream.io/how-it-works/). Postgres 17 does the same at block level (https://www.postgresql.org/docs/17/continuous-archiving.html). Restore applies pages at disk speed, so recovery is O(bytes), not O(events). Your own numbers show the gap: 44 GB restores in ~3 min, but the same tail replays at only 77-120 events/s. Replaying logical events is the slow path. Shipping pages keeps the tail small without replay.

**Base plus delta, and when to re-base.** Redis re-bases when the file has grown by a percentage (default 100%, floor 64 MB). Postgres 17 keeps a chain and warns that the chain must be tracked, and that a heavily changing database gains little. A growth-ratio rule for us: re-base when cumulative delta bytes >= 0.5 to 1 x base, or when chain restore time exceeds the RTO. The 0.5 to 1 value is a suggestion; I found no source specifying it for chains. Redis's 100% is the closest cited number.

**Why 19% dedup is low (inference, not sourced).** restic uses content-defined chunks of 512 KiB min, 1 MiB average, 8 MiB max (https://restic.readthedocs.io/en/stable/100_references.html). borg defaults to 512 KiB min, ~2 MiB target, 8 MiB max, and also offers a fixed chunker aligned to the page size (https://borgbackup.readthedocs.io/en/stable/internals/data-structures.html). Content-defined chunking is designed to survive insertions that shift bytes. A SQLite file is page-aligned and mutated in place, so nothing shifts and CDC buys nothing. What matters is chunk size against dirty-page scatter. A 1 MiB chunk holds 256 pages of 4 KiB. A 44 GB file has about 11M pages, so about 43,000 such chunks. If 1M pages are dirtied between snapshots, that is about 23 dirty pages per chunk, and almost every chunk changes. Even 100K dirty pages is ~2 per chunk, so about 90% of chunks still change. Both figures are my estimate of dirty pages, which I have not measured; measure pages dirtied per day from the WAL. The fix is page-sized (4-64 KiB) fixed chunks, or page-delta shipping, not a smarter chunker.

**Copy-on-write filesystem snapshots (ZFS, btrfs, LVM, reflink).** These give an instant consistent point-in-time view and O(changed blocks) incremental sends. I could not source this: the OpenZFS zfs-send man page I fetched states the -i and -I semantics but says nothing about cost scaling (https://openzfs.github.io/openzfs-docs/man/master/8/zfs-send.8.html). btrfs, LVM and reflink were not fetched. This would replace today's "short read-transaction for a page copy" with a near-free freeze, if the host filesystem permits.

**Continuous replication versus periodic snapshot.** Litestream's design is continuous page-log shipping plus a periodic base. With it, the periodic snapshot only bounds restore chain length, not data loss.

## 4. Trigger rules and their rationale

- **Time-based** (Postgres checkpoint_timeout, Litestream 24 h, your weekly): simple, bounds staleness. It ignores load.
- **Log-size or event-count** (max_wal_size, etcd snapshot-count 100000, Pekko snapshotEvery, your 540,000): bounds the tail directly, independent of arrival rate.
- **Replay-time or RTO-based** (yours): bounds recovery directly. Convert to events with mu: 4 min at 100 events/s is 24,000 events. Your current 540,000-event ceiling is 90 min of replay at 100 events/s.
- **Growth-ratio** (Redis AOF): rebase when the delta chain is large relative to the base.
- **Event-aligned** (Pekko predicate snapshots; Redis BACKUP START/SEAL): snapshot at a domain moment. Pekko's predicate, and Redis letting a control plane stagger or time the backup, are the cited precedents. For us the moments are before a deploy or planned restart (a marker is already written then), after a fork request, and after compaction.
- **Load-aware and spread:** Postgres spreads writes over 90% of the interval, and Redis refuses to start an AOF rewrite while an RDB save runs, to avoid two heavy background I/O jobs at once. Neither defers a checkpoint because of foreground load. They throttle it.
- **Rate limiters like yours** (10 x last capture, sealed bytes / 8 GiB per day) have no analogue in the sources I read. They are a cost cap and amount to a spending policy. They are the reason intervals reach 54 hours. Note that 44 GB / 8 GiB per day is about 5 days, so the cap is the binding constraint, not the formula.

**Ski rental.** The deterministic break-even rule is the best deterministic online policy: rent until the accrued rent equals the buy price, then buy. Its ratio is below 2. Karlin et al. show a randomised rule that achieves about e/(e-1), about 1.58, and no randomised rule does better (https://en.wikipedia.org/wiki/Ski_rental_problem, citing Karlin et al., Algorithmica 1988 and 1994). I found no paper that applies ski rental to checkpoint scheduling (my search returned none), so the following mapping is my own. If restore demand is unknown, the "rent" is the expected replay time of the tail. Because replay debt at time t after a snapshot is lambda t / mu, and expected cost accrues as r lambda t^2 / (2 mu), setting that equal to C gives t = sqrt(2 C mu / (r lambda)). That is exactly the Young-form T* above. The two framings agree. The ski-rental view adds a guarantee: if r is unknown or non-stationary, break-even stays within a factor of about 2 of the best fixed hindsight policy (assuming the ski-rental analogy holds).

**Our numbers.** lambda = 5 events/s (range 3 to 9), mu = 100 events/s, C = capture cost in replay-second equivalents. If C were the full 26 min (1,560 s):

| r (restore demand) | T* |
|---|---|
| 1 per hour | sqrt(2 x 1560 x 100 / (5 x (1/3600))) = about 3.9 h |
| 1 per day | about 20 h |
| 1 per week | about 54 h |

If the capture runs in the background and costs the foreground only about 10% of its duration, C = 156 s, and the intervals shrink by sqrt(10) = 3.2: about 6.5 h at 1/day and about 1.2 h at 1/hour. C should include upload bandwidth and dedup-less storage, which I could not price (bytes per capture and the host's bandwidth are not in the brief).

**RTO bound.** The tail at the moment the next snapshot becomes usable is (T + d) lambda / mu, where T is the start-to-start interval and d the capture time. For RTO = 4 min = 240 s:

| lambda | T_max = RTO mu / lambda - d | Duty cycle of captures |
|---|---|---|
| 9 events/s | 2,667 - 1,560 = 1,107 s | 58% |
| 5 events/s | 4,800 - 1,560 = 3,240 s | 32% |
| 3 events/s | 8,000 - 1,560 = 6,440 s | 20% |

At this cost a 4-minute RTO means a capture is running 20 to 58% of the time, with ~44 GB uploaded each time (given the 19% dedup, nearly all of it). That is about 12 MB/s sustained at one capture per hour. This is the quantitative case for deltas. If d fell to 2 minutes (a delta of changed pages), T_max at lambda = 9 is 2,667 - 120 = 2,547 s and the duty cycle falls to about 5%.

## 5. Keeping the snapshot from hurting foreground writes

- Fork copy-on-write (Redis): the child holds a frozen view, and the parent does no disk I/O. Cost: fork stalls of ms to ~1 s and memory growth with write rate (https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/).
- Fuzzy and spread checkpoints: Postgres spreads over 90% of the interval, InnoDB flushes small batches (sources above).
- Static copy: SQLite backup API steps and releases locks, but a write-heavy source restarts it and it may never finish (https://www.sqlite.org/backup.html). Your design (a short read transaction for the page copy, then hashing a static copy) avoids that by taking the copy once. The cost is a long-held read transaction, which blocks WAL checkpoints for its duration (https://www.sqlite.org/wal.html), so keep it short.
- A replica or immutable files: RocksDB hard links, Redis's hard-linked BASE and INCR files.
- Bounding recovery regardless of load: use a size or event bound (max_wal_size, snapshot-count) rather than a time bound, since a size bound holds at any arrival rate. Postgres will start a checkpoint early when WAL approaches max_wal_size, which is a load-adaptive trigger.

## 6. Decision matrix

| Approach | Optimises | Cost model | Recovery bound | Weight | Used by |
|---|---|---|---|---|---|
| Fixed time interval | Simplicity | C / T | None under load | Trivial | Litestream (24 h), Postgres timeout |
| Event count or log size | Tail length | C per N events | N / mu, any load | Trivial | etcd, Pekko, Postgres max_wal_size |
| RTO (replay time) | Recovery SLO | C per (RTO mu / lambda - d) | Hard, if d is small | Low | Your current rule |
| Young/Daly or break-even | Expected loss | sqrt(2 C mu / (r lambda)) | Expected only | Low | HPC checkpointing |
| Growth ratio | Chain or log size | Re-base at 100% growth | Chain-length bound | Low | Redis AOF |
| Fork copy-on-write | Foreground impact | Page copies and fork stall | Per-snapshot interval | Medium (not for 44 GB on disk) | Redis |
| Page-delta, continuous | Cost per snapshot | Proportional to changed pages | O(bytes), small tail | Medium to high | Litestream, Postgres 17 |
| Changelog plus background materialisation | Snapshot latency | Log writes + materialisation | Log replay | High | Flink |
| Immutable files or hard links | Snapshot cost | About zero | Instant | Needs LSM storage | RocksDB, Redis 8.10 |
| Copy-on-write filesystem | Freeze cost | O(changed blocks) (unsourced) | Instant freeze | Needs host support | ZFS, btrfs (not sourced) |

## 7. Recommendation

**Structure.** Ship page deltas continuously (Litestream-like LTX from the SQLite WAL, or sqlite3_rsync). Keep a periodic full base, and re-base when cumulative delta bytes reach a fraction of the base (start at 0.5 to 1x, tune from measured dirty pages). Keep the event-log tail short so that logical replay stays well under the RTO. The tape stays canonical and untruncated; the snapshot is only an accelerator. Switch to page-sized fixed chunks in the meantime, which should lift dedup well above 19% if my chunk-size explanation holds.

**Single defensible trigger rule.** Pick one objective and state it:
- *Hard RTO (recommended).* Start the next capture when (events since last capture start + lambda_hat x d) / mu >= RTO, that is T = RTO mu / lambda - d. Plus snapshot before any planned restart, deploy or fork. This is deterministic and bounds recovery at any arrival rate. The only tunable is the RTO. With deltas (d about 2 min) and RTO = 10 min, T at lambda = 9 is 6,667 - 120 about 6,500 s, so about hourly.
- *Expected loss.* T* = sqrt(2 C mu / (r lambda)). Only valid if you can estimate r, the rate of restore, fork and migration demand, from the record. If r is zero, the rule says never snapshot, which is why the RTO rule is safer.

**Where there is no clear optimum (policy choices).**
- The RTO value itself (4 min versus 30 min). It trades upload cost against restore latency. No source determines it.
- The weight of background work against foreground work inside C.
- The re-base ratio (0.5 versus 1.0). Redis picked 100% by convention.
- The cap on daily upload bytes. It is a spending decision, not an optimum.
- Whether to delete old snapshots or keep several. Pekko defaults to keeping a few, Litestream to 24 h.

## 8. What I could not source

- The Daly PDF itself (404). Formulas come from a secondary summary.
- Any checkpoint-specific online (ski-rental) result. The mapping is my own.
- ZFS/btrfs/LVM incremental-cost claims, EventStoreDB, Axon, FoundationDB, CockroachDB, Kafka Streams, LMDB's copy-on-write design, InnoDB adaptive flushing parameters (the fetch tool produced these from memory, not the page).
- The 0.5 to 1x delta re-base ratio, and the dirty-page rate behind the dedup estimate. Both need measurement on your system.

## Sources

- https://blog.acolyer.org/2015/06/10/a-higher-order-estimate-of-the-optimun-checkpoint-interval-for-restart-dumps/
- https://laro.lanl.gov/esploro/outputs/journalArticle/A-higher-order-estimate-of-the/9916364420003761 (Daly 2006 record; not read)
- https://www.postgresql.org/docs/current/wal-configuration.html
- https://www.postgresql.org/docs/17/continuous-archiving.html
- https://www.sqlite.org/wal.html
- https://www.sqlite.org/backup.html
- https://dev.mysql.com/doc/refman/8.0/en/innodb-checkpoints.html
- https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/
- https://raw.githubusercontent.com/redis/redis/unstable/redis.conf
- https://etcd.io/docs/v3.5/op-guide/configuration/
- https://github.com/facebook/rocksdb/wiki/Checkpoints
- https://lmdb.readthedocs.io/en/release/
- https://flink.apache.org/2022/05/30/improving-speed-and-stability-of-checkpointing-with-generic-log-based-incremental-checkpoints/
- https://cwiki.apache.org/confluence/display/FLINK/FLIP-158:+Generalized+incremental+checkpoints
- https://litestream.io/how-it-works/
- https://pekko.apache.org/docs/pekko/current/typed/persistence-snapshot.html
- https://restic.readthedocs.io/en/stable/100_references.html
- https://borgbackup.readthedocs.io/en/stable/internals/data-structures.html
- https://en.wikipedia.org/wiki/Ski_rental_problem
- https://openzfs.github.io/openzfs-docs/man/master/8/zfs-send.8.html
