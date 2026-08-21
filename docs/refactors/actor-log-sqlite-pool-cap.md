# Refactor: Single Actor-Log SQLite Opener With Capped Connection Pool

**Date:** 2026-08-21
**Mutation class:** yellow — test/CI reliability change; no product behavior change.
**Status:** documented, not implemented. Do soon.

## Problem

The actor log (`internal/actor/log_sqlite.go`, file `<store-path>-actor.db`)
holds the durable wake log (`actor_updates`) and compacted actor memory
(`actor_snapshots`). It is a delivery projection, not a semantic authority —
the canonical event chain in embedded Dolt remains the single state authority,
and tape restore deliberately never reconstructs this file.

The flake class is a connection-pool configuration gap:

- `internal/actorruntime/adapter.go` (production) opens the log with
  `?_busy_timeout=60000` but no pool cap. `database/sql` defaults to an
  unbounded pool, so multiple connections hit a single-writer SQLite file.
  Under default rollback-journal mode, reader/writer collisions can return
  `SQLITE_BUSY` immediately in lock-upgrade paths where the busy handler is
  bypassed.
- `internal/actorruntime/adapter_test.go` hand-rolls the same open without a
  cap, so tests reproduce the production drift instead of pinning safe config.
- `internal/actor/actor_test.go` already does it correctly:
  `db.SetMaxOpenConns(1)`.

Live evidence: CI run `32452830454`, job `96684359663`,
`TestAdapterSQLiteResearcherAdmissionRecoveryExecutesWithoutSnapshot` failed
with `database is locked (5) (SQLITE_BUSY)` on `LoadSnapshot` while the runtime
actor goroutine wrote concurrently. The same SHA passed on re-dispatch —
classic timing-dependent flake, not a regression.

## Why not other fixes

- **WAL mode:** would allow readers during writes, but adds `-wal`/`-shm`
  files and checkpointing concerns for no measurable benefit at mailbox
  volumes. Skip unless profiling shows contention after the pool cap.
- **Move the log into Dolt/the event tape:** wrong by doctrine. Every agent
  wake would become a semantic event, inflating the tape with non-state
  transitions and violating "no second state authority" hygiene. Delivery
  metadata stays off the semantic tape (`docs/current-architecture.md`: actor
  logs are projections/actuators; replay manifest excludes them).
- **CI retries:** masks the race instead of removing it.

## Refactor

1. **One opener.** Add a helper in `internal/actor` (e.g.,
   `OpenLogDB(path) (*sql.DB, *SQLiteLog, error)`) that applies
   `?_busy_timeout=60000` **and** `db.SetMaxOpenConns(1)`, mirroring the
   existing `configureEmbeddedDoltDB` convention for Dolt pools.
2. **Migrate every call site.** Replace raw
   `sql.Open("sqlite", …+"?_busy_timeout=60000")` in
   `internal/actorruntime/adapter.go` and all test files with the helper.
   Production and tests then cannot drift apart.
3. **Guard against reintroduction.** No raw `sql.Open("sqlite", …)` for the
   actor log outside the helper — enforce via heresy-detector family or a
   doccheck/source lint rule.
4. **Keep per-test `t.TempDir()` isolation** as-is; the bug was intra-process
   concurrency, not cross-test interference.

## Acceptance

- Focused `internal/actorruntime` and `internal/actor` suites pass repeatedly
  (`-count=10`) locally under load.
- One full CI run green including the previously flaky shard.
- Grep confirms zero uncapped actor-log opens outside the helper.

## Rollback

Revert the single refactor commit; behavior-neutral otherwise.
