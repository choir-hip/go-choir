# Design: platform-dolt storage normalization — 2026-09-18

Class: green (problem documentation + design; no runtime change). Owner asked
for a durable fix for the platform-dolt storage leak. Revised after an agentic
consensus panel (6 models) unanimously returned **RETHINK** on the first draft —
the draft re-fought a battle the repo already lost. This version carries the
corrected root cause.

## Invariant (owner-stated)

A **secure, auditable, immutable event log**, plus **snapshots from which an
autoputer's current state can be recovered after corruption**. Everything else
is negotiable.

## Root cause (corrected — was mis-diagnosed in v1)

**The leak is per-mutation `CALL DOLT_COMMIT`, not missing GC.**

`internal/platform/store.go`, `internal/cycle/storage.go`,
`internal/platform/objectgraph_store.go`, and ~49 other call sites (52 total
`DOLT_COMMIT`/`commitDolt` references across `internal/platform`, `internal/cycle`,
`internal/store`) commit every mutation to the Dolt commit graph. The 2026-08-26
receipt (`docs/evidence/platform-dolt-oldgen-218g-dead-history-2026-08-26.md`)
measured **6,903,253 commits**; Dolt GC — shallow *or* `--full` — collects only
*unreachable* chunks, and every commit on `main` is reachable from HEAD. So the
entire commit graph (months of superseded `og_objects` longblob bodies + event
receipts) is uncollectible by any GC. Auto-GC was already running correctly
throughout (78 shallow GCs, every ~45 min).

This is the **second** recurrence: squashed to ~2 commits on 2026-08-26, the
store re-grew to ~224G and was squashed again today (2026-09-18, `dump-20260918`,
20G SQL → 9.2G live). `dolt_log` is already back to **39,540 commits** ~1 day
post-squash. The per-mutation commit behavior is unchanged, so it will re-grow
again.

**Corollary the panel proved:** "schedule `DOLT_GC()`" (v1 Move 1) is the
known-wrong hypothesis — it was the 11/12 majority opinion on 2026-08-26,
falsified by the dissent's `dolt_log` measurement before execution. And
"watermark-bounded GC" is **not expressible**: Dolt GC reachability is over the
commit graph; `computer_replay_watermarks` is a mutable application row, not a
GC root.

## What the log actually is (category correction)

The audit/recovery path is **not** Dolt versioning. It is:

- the append-only SQL table `computer_event_append_receipts` (the event rows,
  `UNIQUE (computer_id, sequence)`), plus
- the filesystem CAS at `/var/lib/go-choir/platform-artifacts` (event payloads
  via `PayloadCommitment` → `computer-event-payload`, checkpoint artifacts via
  `checkpoint_artifact_ref`).

Dolt commits on the event tables are a storage-engine side effect, not the log.
Guest Texture history already weaned off `dolt_history_*`/`AS OF` onto the
indexed parent chain (`internal/store/texture.go`). So: **keep-all the SQL event
table + `platform-artifacts` pins; bound only the Dolt commit graph.**

## What Store B actually is (authority correction)

`og_*`/`items`/`fetches`/`platform_texture_revisions` are **not** derived or
rebuildable from the event log. `og_objects` holds platform Texture documents
(World Wire / publication truth); `items`/`fetches` are GDELT/RSS observations
that are re-fetchable in name only (sources rotate). The ontology already puts
world-wire state **outside** user-computer restore. Treat Store B as its own
source of truth with its own backup identity — not disposable corpus.

## Durable fix — three moves (revised)

### Move 1 — stop per-mutation `DOLT_COMMIT` (the actual leak fix)

Coalesce the ~52 per-mutation commit sites behind **one debounced snapshot
committer**: batch mutations in ordinary SQL transactions, and `DOLT_COMMIT` on
a cadence (e.g. every N seconds / M mutations / on checkpoint), not per write.
This is the owner-directed fix already recorded in the 2026-08-26 receipt
("post-overhauls the platform moves to regular snapshotting instead of unbounded
per-event commit history").

- Gate first: re-measure `dolt_log` on Node B (currently 39,540 and climbing).
- The event log's immutability does not depend on per-write Dolt commits — the
  SQL rows are the log. Batched commits still give periodic addressable
  snapshots; they just stop the 6.9M-commit explosion.
- Keep server auto-GC on; it can now actually collect unreachable chunks because
  the reachable set stops exploding.
- Extend the existing `go-choir-platform-dolt-history-audit` timer
  (`nix/node-b.nix`) to alert on commit-*rate* and `dolt_log` growth SLO, not
  only the 50 GiB oldgen floor — it fired on size but the recurrence is
  commit-count-driven.

### Move 2 — split by authority into two stores (blast-radius + independent retention)

Only after Move 1. Split by **authority**, not "versioned vs not":

- **Store A — canonical event/control store** (Dolt): `computer_event_*`,
  `computer_lifecycle_*`, `computer_checkpoints`, `computer_replay_watermarks`,
  `computer_version_*`, `computer_key_*`, `computer_route_*`, `consent_records`,
  `control_key_history`, receipt/rollback-ref tables. Keep-all the SQL rows +
  artifact pins; bound only the commit graph via Move 1 batching + periodic
  squash.
- **Store B — world-wire/corpus store** (Dolt or a conventional engine):
  `og_*`, `items`, `fetches`, `ingestion_events`, `cycle_*`, `processor_requests`,
  `provenance_*`, `publication_*`, `platform_texture_revisions`, `artifact_*`.
  Its own source of truth, its own backup identity, its own commit cadence.
  `og_objects` already does application-level versioning (`version_id`,
  `content_hash`, `superseded_by`, `tombstone`) — Dolt versioning on top is
  double-versioning for zero read benefit; keep Dolt only if `AS OF`/diff/merge
  is a demonstrated product query, else a conventional engine is cheaper.
- **Separate sql-server process** for Store B (own systemd unit): two DBs on one
  server share process/OOM/crash domain and do not give independent GC memory
  isolation. A GC-OOM on Store B must not take the canonical log down.

### Move 3 — content-address fat payloads into `platform-artifacts` (not a Dolt `blobs` table)

Move `items.body`/`raw_json`/`reader_snapshot` and `og_objects.body` bytes into
the existing filesystem CAS (`/var/lib/go-choir/platform-artifacts`), keeping
only `content_hash` digest + byte size in SQL. Do **not** put blobs in a Dolt
`blobs` table — that keeps fat bytes on the versioned chunk store and re-creates
the OOM-on-scan risk (27.3G OOM on 2026-08-26). Dedup is a bonus; the real win
is small versioned rows and keeping bulk bytes off the chunk engine.

## Migration path (offline dump-and-split — never dual-write)

The writers are two local daemons (`go-choir-sourcecycled`, `go-choir-corpusd`);
no zero-downtime cluster constraint. Dual-write across two DBs without
distributed transactions risks divergence — use a fenced cutover instead:

1. Quiesce `go-choir-sourcecycled` + `go-choir-corpusd` (systemd stop).
2. **Offline** `dolt dump` (never full-scan via the SQL server — it OOM-killed
   at 27.3G on 2026-08-26). Split the dump: canonical tables → Store A,
   corpus/OG → Store B.
3. Init Store A fresh from the canonical dump (single root commit); init Store B
   from the corpus dump.
4. Update DSNs in service configs; restart.
5. Keep `dump-20260918` as the rollback ref until the split is verified.

## Recovery invariant preserved

- Store A keeps the full SQL event log + `computer_checkpoints` +
  `computer_replay_watermarks` + `platform-artifacts` pins → autoputer rebuild
  by replay-from-checkpoint (existing restore path).
- Store B is world-wire/corpus truth, restored from its own backup — not part of
  user-computer recovery (per the ontology).
- Event-log truncation, if ever wanted, is a separate explicit protocol
  (verified checkpoint + artifact bundle + external anchor + rollback window +
  recovery rehearsal) — never a GC setting.

## Open questions for triage

- **Blocking pre-migration:** inventory every `og_*`/`platform_texture_revisions`
  consumer; classify each table derived-rebuildable / mirror-able /
  source-of-truth; set Store B retention per class. Do not backfill until done.
- Does any consumer read `og_*`/`items` via Dolt `AS OF`/time-travel? (Believed
  no — Texture uses the parent chain — but confirm before choosing Store B's
  engine.)
- Commit-batching cadence: how much event latency is acceptable between a write
  and its addressable Dolt snapshot? (Checkpoint cadence is the natural bound.)
- Backup manifest: Store A head + Store B head + `platform-artifacts` roots +
  key version need one coordinated identity; add restore drills.

## References

- **Root-cause receipt (authoritative):** `docs/evidence/platform-dolt-oldgen-218g-dead-history-2026-08-26.md`
- Commit sites: `internal/platform/store.go`, `internal/cycle/storage.go`,
  `internal/platform/objectgraph_store.go` (+ ~49 more `DOLT_COMMIT`/`commitDolt`)
- History-audit timer: `nix/node-b.nix` (`go-choir-platform-dolt-history-audit`)
- Guest GC precedent (embedded store only): `internal/store/dolt_maintenance.go`
- Event payload CAS: `internal/computerevent/appender.go` (`PayloadCommitment`)
- Texture parent-chain history: `internal/store/texture.go`
- Consensus panel outputs: `.agentic-consensus/agentic-consensus-20260918-191939/`
