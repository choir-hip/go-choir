# Data system build sheet (2026-10-30)

Station D1 of [the build goal](../definitions/choir-data-system-build-2026-10-30.md).
These are the decisions each builder needs, and nothing more. The frame is
[`choir-data-system-frame-2026-10-30.md`](choir-data-system-frame-2026-10-30.md),
revision 2. The package split and every call site are in
the Dolt coupling census,
§5 (WP0 and WP-A to WP-F).

## Decisions

1. **No dual paths.** New guest code is SQLite only. New host code is
   PostgreSQL only. Nothing runs Dolt and the new engines side by side.
   Before deletion, the rollback is the previous release plus the old data,
   which nothing touched: the migrators only read Dolt.
2. **Integration branch.** All builders merge into `cutover/data-system`.
   CI runs on its pull request. `main` receives it as the cutover (C5),
   because host code that is PostgreSQL only cannot deploy to staging
   before the data is migrated. Each builder works in its own worktree off
   the integration branch, rebases often, and owns only its package set
   (census §5).
3. **A fresh-database cutover.** Schemas are written clean in the new
   engines, with no retired columns and no migration ladders; the
   migrators copy rows. Each store has one `schema.go` with the current
   DDL and a schema version row. The census lists about 1,500 lines of
   migration code in `internal/store` that can be deleted.
4. **SQL forms.**
   - The code keeps `?` placeholders. The PostgreSQL connector in `dbx`
     rebinds them to `$n`.
   - Upserts are written
     `INSERT ... ON CONFLICT (pk) DO UPDATE SET c = excluded.c`, a form
     both SQLite and PostgreSQL accept.
   - `INSERT IGNORE` becomes `ON CONFLICT DO NOTHING`, and `REPLACE INTO`
     becomes the upsert form.
   - `FOR UPDATE` appears only in PostgreSQL code. SQLite writes go
     through the single write connection inside `BEGIN IMMEDIATE`.
   - `information_schema` and `SHOW` give way to `dbx` introspection.
     Under a fresh-database cutover most of those call sites disappear.
5. **Types.**

   | Engine | Strings and JSON | Integers | Binary | Timestamps |
   |---|---|---|---|---|
   | SQLite (guest) | `TEXT` | `INTEGER` | `BLOB` | `DATETIME`, with values bound as `time.Time` (the dbx opener sets the modernc time format) |
   | PostgreSQL (host) | `TEXT`; `JSONB` only where a query reads a JSON path | `BIGINT` (`UNSIGNED` dropped) | `BYTEA` | `TIMESTAMPTZ` |

   PostgreSQL also uses `SMALLINT` for `TINYINT`, which keeps the Go scan
   types unchanged. Any unsigned value that could pass 2^63 is stored as
   `TEXT`; the census lists candidates under §6.
6. **The witness is version 3.**
   - For each table, it is a SHA-256 over the rows in primary-key order,
     in a canonical encoding.
   - The schema hash covers `sqlite_master`.
   - It is computed by streaming, so memory stays bounded. Computation is
     O(rows), and incremental maintenance comes in phase 2.
   - The version bump makes witnesses from versions 1 and 2 unjudgeable.
     Their bases are deleted at C5 anyway.
7. **Snapshot version 1 replaces projection bases.**
   - A snapshot is the SQLite file written with `VACUUM INTO`, sealed
     under the computer key, stored in the CAS in the computer's private
     namespace, and named with its file root.
   - A synchronous semantic event, `snapshot_recorded`, names the
     snapshot.
   - In phase 1 a snapshot is taken at the migration. Phase 2 adds the
     cadence of 10,000 events or one hour.
   - `internal/projectionbase` and its tools are deleted.
   - Boot reads the computer's own disk. A missing disk is rebuilt from
     the newest snapshot plus tape replay.
8. **The guest keeps no event bodies.** Its event index keeps only the
   head row, the prepared row and, per event, the sequence, digest, kind
   and receipt digest. `event_json` and `event_head_receipt_json` are
   removed from the guest.
9. **Sealing.** In the new release, `projection_batch_recorded` payloads
   take the private path (`PreparePrivatePayload`, `appender.go:592-602`).
   That closes the plaintext problem for new writes. WP-A makes the change
   in `internal/store/projection_tape.go`.
10. **The tape protocol is unchanged in phase 1.** It is still a
    compare-and-swap per event. The PostgreSQL `computer_event_append_receipts`
    table keeps the receipt JSON once, and its duplicate columns are
    dropped (WP-D). Batching is the first phase 2 item, I0, landed right
    after C5.
11. **PostgreSQL on Node B** (WP-F):
    - `services.postgresql` with `postgresql_17`, one cluster;
    - two databases, `control` and `graph`, with role `choir` and its
      password in `/var/lib/go-choir/secrets/pg-app-password`;
    - listening on 127.0.0.1:5432 and the unix socket;
    - systemd `MemoryMax=4G`, `shared_buffers=1GB`, `wal_compression=on`,
      `synchronous_commit=on`;
    - environment variables `CHOIR_PG_CONTROL_DSN` and
      `CHOIR_PG_GRAPH_DSN`.
12. **Migrators** (WP-F):
    - **Host:** `cmd/choir-migrate-host` reads Store A and Store B through
      the MySQL protocol and writes `control` and `graph`. It truncates
      each target table first, so a rerun starts clean. For every table
      it compares row counts and a per-table digest between source and
      target, then writes a report.
    - **Guest:** `internal/migrate/guestdolt` runs inside autoputer at
      boot when the Dolt directory exists and the SQLite file does not.
      It reads with the embedded Dolt driver, writes SQLite, verifies
      counts and digests, takes the migration snapshot, writes a marker,
      and keeps the Dolt directory until C5 deletes it.
    - These two are the only packages that import Dolt or the MySQL
      driver after the build.
13. **Tests.**
    - `dbxtest.SQLite(t)` opens a temporary SQLite database.
    - `dbxtest.Postgres(t)` uses `initdb` and `pg_ctl` from `PATH`: one
      cluster per package through `TestMain`, and one database per test.
    - The devShell adds `postgresql_17`, and the CI Go test jobs install
      PostgreSQL.
    - Tests that used embedded Dolt as a MySQL stand-in (45 platform test
      files and 5 cross-package files) move to `dbxtest.Postgres`.
14. **Deleted in the build:**
    - `internal/doltbatch`, `internal/projectionbase`;
    - `internal/store/dolt_maintenance.go`, `checkpoint_cadence.go`;
    - `cmd/choir-rebuild-base`, `cmd/projection-compact`,
      `cmd/platform-dolt-squash`;
    - the squash scripts;
    - the Dolt services and timers in `nix/node-b.nix`.

## Work packages and order

1. **WP0, the seam:** `internal/dbx`, `dbxtest`, `dbcred`, the split of
   `platform/store.go`, and the pgx module with vendoring. FP0 builds it
   first, and every other package depends on it.
2. **In parallel once WP0 lands**, one Sonnet builder each in its own
   worktree:
   - WP-A, the guest store;
   - WP-B, the guest object graph;
   - WP-C, the witness, replay and snapshots;
   - WP-D, host control;
   - WP-E, the host graph database;
   - WP-F, the migrators, nix and scripts.
3. **C0, the drills,** go into `scripts/cutover/` as the packages land:
   - the differential report from each migrator;
   - kill -9 against SQLite and against PostgreSQL;
   - the keep-set counts;
   - a boot, wake and serve check;
   - the paper publishing;
   - an owner computer opening Texture;
   - mail arriving;
   - a stale-epoch append being refused.
4. **C4, the rehearsal on Node B:**
   - The host migrator runs against reflink copies of Store A and Store B
     into a scratch PostgreSQL database.
   - The guest migrator runs against a reflink copy of d@e's data image.
   - Both produce differential reports.
   - Then the integration branch builds green in CI.
5. **C5:**
   1. Stop the computers and the Dolt-backed services.
   2. Run the host migrator for real.
   3. Merge the integration branch to `main`, which runs CI and the deploy.
   4. Computers boot the new release and migrate themselves, d@e first.
   5. Run the drills.
   6. When the differential and keep-set checks are green, delete the old
      data.

## What every builder must hold

- Bounded memory: no full-table loads, so scans stream.
- No secrets or email addresses in code, tests or logs.
- The tests that cover your package pass on the new engine. Tests of
  deleted code are deleted with it.
- `go build ./...` and `go vet` stay clean for your packages on the
  integration branch.
- Write a one-line ledger note for anything you decide that this sheet
  does not cover.
