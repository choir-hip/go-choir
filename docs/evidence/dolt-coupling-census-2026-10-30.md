# Dolt / MySQL coupling census (2026-10-30)

Purpose: precise port map for the one-shot Dolt replacement (guest store to
SQLite, Store A + Store B to one PostgreSQL instance with two databases).
Feeds docs/definitions/choir-data-system-build-2026-10-30.md C1 and C2.
Read-only investigation, repo at 269460a5c (main). Mutation class: green.

Tags: [V] verified by grep/read of the cited line; [I] inferred.
Method: `rg`/python line scans over `cmd/` and `internal/` (2398 .go files;
`scripts/ograph-bench` is the only Go outside those). Comment-only hits were
excluded (they are listed where they matter). SQL "statements" are string
literals that begin with a SQL verb and contain a clause keyword, so dynamic
SQL built from fragments is undercounted: treat statement counts as a lower
bound [I]; "call sites" (`ExecContext|QueryContext|QueryRowContext|BeginTx|
db./tx./conn. Exec|Query|QueryRow`) are the better size measure.

## 0. Headline findings

1. **There is no `AS OF`, `dolt_log`, `dolt_diff`, `DOLT_BRANCH/CHECKOUT/MERGE`
   or `dolt_history_*` in any non-test Go code** [V]. `AS OF` appears only in
   `scripts/ograph-bench/main.go:679` (a benchmark tool) and in comments
   (`store.go:216,337`, `checkpoint_cadence.go:18`, `texture.go:1238`).
   Branch/merge/tag/reset appear only in
   `internal/computerversion/dolt_branch_isolation_pinned_test.go` [V].
   The Dolt commit therefore has no reader in product code: it is written
   (`DOLT_COMMIT`) and its only consumers are `HASHOF('HEAD')` (witness
   audit field) and the squash/GC tooling [V].
2. The product-path Dolt dependencies are exactly four: (a) `CALL DOLT_COMMIT`
   checkpoints (guest `store.go:401`; host `doltbatch/committer.go:244`),
   (b) `DOLT_HASHOF_TABLE` / `HASHOF('HEAD')` for the v2 witness
   (`computerversion/dolt_table_hashes.go:38,79`, `dolt_state_extractor.go:145`),
   (c) the Dolt workspace *directory* as the unit of projection-base tar/parts,
   install, rebase and GC (`projectionbase/*`, `autoputer/projection_base.go`),
   (d) `dolt_ignore` for two Store B projection tables
   (`platform/capture_index.go:100-104`). Everything else is MySQL dialect.
3. The embedded driver (`github.com/dolthub/driver/v2`) is imported by 5
   non-test files: `store/texture.go:38`, `store/dolt_maintenance.go:15`,
   `computerversion/dolt_state_extractor.go:15`, `projectionbase/squash.go:11`
   (+ `dolt/go/store/util/tempfiles` at `squash.go:13`,
   `cmd/projection-compact/main.go:18`; `dolt/go/libraries/doltcore/memlimit`
   at `autoputer/run.go:18`) [V]. `go-sql-driver/mysql` is imported by 3:
   `platform/store.go:15`, `cmd/vmctl/main.go:18`, `dbcred/dbcred.go:25` [V],
   plus `scripts/ograph-bench/{main,platform,postings}.go`.
4. `internal/textureowner` has zero Dolt coupling: two comment mentions only
   (`texture.go:709`, `prompt_bar.go:23`) [V]. `internal/agentcore` touches
   the DB in one file (`checkpoint_restore_bindings.go:143-170`, plus the
   workspace-path/extractor users in `rematerialize.go`, `replay_completeness.go`,
   `replay_eligibility.go:21,195,280`) [V].
5. Platform tests use the embedded Dolt driver as a stand-in MySQL server:
   `openTestPlatformStore` (`platform/service_test.go:22`) is called from 45
   test files in `internal/platform`; four more packages build a platform
   store the same way (`proxy/owner_act_receipts_test.go`,
   `researchtools/corpus_index_e2e_test.go`, `agentcore/standing_consensus_e2e_test.go`,
   `platform/{artifact_gc,file_cas_retire}_test.go`) [V]. A PostgreSQL test
   harness is a prerequisite for the host port [I].
6. The C4 migration needs a *Dolt reader* (guest boot converts its own Dolt
   state; host copies Store A/B out of Dolt servers) [V from the goal file,
   lines 119-121]. So the embedded driver, the mysql driver and `vendor/`
   entries cannot leave `go.mod` until a one-shot migrator package is
   retired after C5 [I]. Keep migrator code in a new package so the ported
   packages carry no Dolt import.

## 1. Per-package census (non-test code)

Counts are lines (file:line in the detail tables below). "Dolt" = Dolt procs /
system tables / hash functions / dolt_ignore / embedded driver import.
Dialect columns: ODKU = `ON DUPLICATE KEY UPDATE`; IGN = `INSERT IGNORE`;
REPL = `REPLACE INTO`; FU = `FOR UPDATE`; IS = `information_schema`;
JSON = `JSON_EXTRACT/JSON_UNQUOTE/JSON_VALID/CAST AS JSON`;
GEN = generated column; SHOW = `SHOW TABLES/COLUMNS`; AI = `AUTO_INCREMENT`;
TYPES = lines using MySQL-only column types (`LONGTEXT LONGBLOB DATETIME UNSIGNED`;
`VARCHAR/BIGINT/BOOLEAN` are portable and not counted); INLINE = inline
`INDEX/KEY/UNIQUE KEY` inside `CREATE TABLE`; ALTER = MySQL-only ALTER forms
(`AFTER col`, `DROP PRIMARY KEY, ADD PRIMARY KEY`, `DROP INDEX x ON t`).

| package | role (section 2) | Dolt | ODKU | IGN | REPL | FU | IS | JSON | GEN | SHOW | AI | TYPES | INLINE | ALTER |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| internal/store | guest store | 4 | 14 | 1 | 0 | 7 | 12 | 0 | 0 | 1 | 1 | ~110 | 3 | 5 |
| internal/objectgraph | guest graph (DoltStore); twin of host schema | 0 | 4 | 0 | 1 | 1 | 2 | 15 | 3 | 0 | 0 | 15 | 2 | 0 |
| internal/computerversion | witness (guest+host tooling) + host route inputs | 4 | 2 | 0 | 0 | 2 | 0 | 0 | 0 | 5 | 0 | 8 | 0 | 0 |
| internal/projectionbase | host/guest tooling | 10 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| internal/doltbatch | host control + corpus | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| internal/platform | host control (A) + corpus (B) | 2 | 15 | 14 | 1 | 13 | 1 | 1 | 0 | 3 | 1 | ~125 | 22 | 8 |
| internal/routeledger | host control (A) via vmctl | 0 | 2 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 0 | 15 | 10 (+8 FK) | 0 |
| internal/selfdev | guest (uses store DB) | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| internal/dbcred | host DSN credential | 0 (imports mysql.Config) | | | | | | | | | | | | |
| cmd/vmctl | host (Store A client) | `sql.Open("mysql")` main.go:211 | | | | | | | | | | | | |
| cmd/checkpointd | host tooling | `ensureDoltHome` dolthome.go:18, capture.go:327 | | | | | | | | | | | | |
| cmd/corpusd | host service | `platform.OpenStore` main.go:43 | | | | | | | | | | | | |
| cmd/projection-compact, cmd/platform-dolt-squash, cmd/choir-rebuild-base | tooling | Dolt GC/squash via projectionbase | | | | | | | | | | | | |
| scripts/ograph-bench (Go) | tooling | AS OF + dolt_history_* + mysql driver | | | | | | | | | | | | |

Values marked `~` are per-line regex counts of column-type tokens; exact
counts are not needed to size the port [I]. All other numbers are line
counts from the scans [V].

Packages with SQL that are **already portable** (SQLite via `modernc.org/sqlite`,
no Dolt): `internal/auth` (11 tables), `internal/maild` (22 tables; uses
`INSERT OR REPLACE`), `internal/actor` (3), `internal/actorruntime` (log
db), `internal/base/journal` (2), `internal/gateway/searchplane` (2),
`internal/vmctl/{pulse,retention_prune}.go` (read-only opens) [V]. They need no
dialect work; they may adopt the common open helper later.

### 1a. Dolt-specific SQL, driver and procedure sites (executable lines)

| site | what | product path |
|---|---|---|
| store/store.go:401 | `CALL DOLT_COMMIT('-Am', ?)` in `commitDoltCheckpointLocking` (callers: `markDoltHistoryDirty` x7 in store/{computer_events,texture,vocab_migrate,vocab_migrate_og}.go, store.go:516; `getHistory` texture.go:~1250; Close) | guest checkpoint cadence |
| store/dolt_maintenance.go:289, 346, 357 | `CALL DOLT_GC()` (offline root, online on write conn, textureHandle) | guest periodic/boot GC (`autoputer/run.go:195,716,754,773`) |
| store/texture.go:246-293 | `embedded.ParseDSN/NewConnector`, `SHOW DATABASES`, `CREATE DATABASE texture` | guest open |
| store/dolt_maintenance.go:279-287 | second embedded connector for GC | guest GC |
| doltbatch/committer.go:244 | `CALL DOLT_COMMIT('-Am', ?)` (debounced) | Store A + Store B snapshots (`platform/store.go:281,300,333`) |
| computerversion/dolt_table_hashes.go:38,79; dolt_state_extractor.go:145 | `HASHOF('HEAD')`, `DOLT_HASHOF_TABLE(?)` | v2 witness; also `dolt:<db>:head` observation |
| computerversion/dolt_state_extractor.go:298-306 | embedded connector to a closed workspace | witness extraction (checkpoint, replay completeness, projection base) |
| projectionbase/squash.go:92 | `dolt_hashof_db(?)` | squash safety check |
| projectionbase/squash.go:163-164,175,184,211,236 | `dolt_tags`, `dolt_remotes`, `dolt_branches`, `dolt_commit_ancestors`, `dolt_log` | squash precondition counts |
| projectionbase/squash.go:224,231,251 | `DOLT_RESET('--soft')`, `DOLT_COMMIT('-A','--allow-empty')`, `DOLT_GC('--full','--archive-level',...)` | Store A squash; scratch compaction |
| projectionbase/rebuilder.go:95,459 | `runScratchDoltGC = choirstore.RunDoltGCWorkspace` | scratch base build |
| platform/capture_index.go:100,104 | `SELECT COUNT(*) FROM dolt_ignore`; `INSERT IGNORE INTO dolt_ignore` | Store B: keeps `proj_capture_index/terms` out of history |
| autoputer/run.go:18,167 | `dolt/go/libraries/doltcore/memlimit` Init/NodeCacheSize | guest memory setup (`gomemlimit.ApplyDaemon(doltInit, ...)`, `gomemlimit.go:62-205`, env `CHOIR_DOLT_CACHE_NODE_MIB`) |
| cmd/projection-compact/main.go:18 | `tempfiles` | tooling |
| cmd/checkpointd/dolthome.go:18-30, capture.go:327, main.go:43 | `DOLT_ROOT_PATH/.dolt` bootstrap | checkpointd embedded scratch |
| DSN params: store/texture.go:239,281; dolt_maintenance.go:275; squash.go:75; dolt_state_extractor.go:295 | `commitname=...&multistatements=true&clientfoundrows=true` | all embedded opens |
| DSN params: platform/config.go:11 | `parseTime=true&multiStatements=true&clientFoundRows=true` | Store A client; same string in nix/node-b.nix:1270,1380,1622 and start-services.sh:83 |
| agentcore/api_health_stall_holder.go:31 | `engineModulePrefix = "github.com/dolthub/"` (stack-walk to attribute stalls) | /health diagnostics |
| agentcore/api.go:180,732; store ReadDoltGCDisposition | `dolt_gc` health field | /health |
| agentcore/isolation_selftest.go:304 | probe that host Dolt :13306 is blocked from the guest | guest isolation self-test (must retarget to PG port) |
| objectgraph/engine_mutex.go (227 LOC), dolt_store.go (77 `engineMu` refs), derived_keys.go (5), newest_headers.go (10), agentcore/api.go (11) | process-wide mutex because "two DoltStore instances over one connector share one engine, not safe for concurrent queries" (dolt_store.go:62-66) | guest concurrency model |

Test-only Dolt coupling (separate list, 15 test files import the embedded
driver) [V]: `agentcore/{api_health_stall_holder,standing_consensus_e2e}_test.go`,
`computerversion/{dolt_branch_isolation_pinned,dolt_state_extractor,driver_v2_smoke}_test.go`,
`doltbatch/committer_test.go` (17 stmts), `gomemlimit/gomemlimit_test.go`,
`objectgraph/{dolt_store,engine_concurrency_probe}_test.go`,
`platform/{service,artifact_gc,file_cas_retire}_test.go`,
`proxy/owner_act_receipts_test.go`, `researchtools/corpus_index_e2e_test.go`,
`store/journal_amplification_census_test.go`. Test statements using Dolt
procs: `CALL DOLT_COMMIT` x13, `DOLT_GC` x11, `DOLT_HASHOF_TABLE` x3,
`DOLT_BRANCH` x3, `DOLT_TAG` x2, `DOLT_RESET` x2, `DOLT_CHECKOUT` x2,
`DOLT_STASH/REMOTE/MERGE/ADD` x1 each; `dolt_*` system-table reads in 16 test
files (squash_test 10, committer_test 7, snapshot_mode_test 4, ...). Other
test dialect: `EXPLAIN PLAN` (Dolt-specific plan syntax) in
`store/computer_event_prefix_test.go:164`, `store/event_table_test.go:1127`,
`objectgraph/generated_columns_test.go:30`, `platform/retrieval_store_b_test.go:271`;
`information_schema` in 13 test files (26 lines); `SHOW` in 4.
Test volume: store 142 test files / 46.6k LOC (347 SQL literals), platform 77 /
21.7k (120), objectgraph 17 / 5.1k (28), projectionbase 16 / 4.5k (31),
computerversion 13 / 1.9k (37).

### 1b. Dialect detail (non-test, file:line) [V]

- **ON DUPLICATE KEY UPDATE** (37 lines; 51 `VALUES(col)` references inside them need
  `EXCLUDED.col` / `excluded.col`): store/project.go:143,254,293,447,543,635;
  store/media_projection.go:42,61,80; store/{event_table.go:154,texture.go:802,
  desktop_live.go:98,store.go,graph_store.go}; objectgraph/dolt_store.go:687,
  1063,1227 (+1); platform/projection_jobs.go:224,418,586,657;
  platform/objectgraph_store.go:119,367,801,829; platform/store.go:531,548;
  platform/file_cas.go:116,120; platform/{tape_tombstone.go:178,retrieval_schema.go};
  routeledger/sql_ledger.go:231,348; computerversion/input_resolver.go:442,476.
- **INSERT IGNORE** (14): store/texture.go:1868; platform/checkpoints.go:207;
  objectgraph_store.go:166; projection_jobs.go:561,627,650,654; file_cas.go:44,109,113;
  retrieval_schema.go:157,409; capture_index.go:104,286.
- **REPLACE INTO** (2 MySQL): platform/capture_index.go:273; objectgraph/derived_keys.go:299.
  (`INSERT OR REPLACE` in maild is already SQLite.)
- **FOR UPDATE** (27 real; 3 more are comments): store/project.go:321,361,398,433,496;
  store/computer_events.go:153,180; objectgraph/dolt_store.go:1187; selfdev/operations.go:476
  (guest, SQLite: delete clause, take `BEGIN IMMEDIATE`); computerversion/input_resolver.go:531,559;
  routeledger/sql_ledger.go:268,298,391; platform/{key_escrow.go:104,207,310;
  projection_jobs.go:301,360,504,611,619; file_cas.go:94,99; checkpoints.go:58;
  self_development_modes.go:484; computer_events.go:334} (PG keeps it).
- **information_schema** (14): store/store.go:1231,1236,3629 (PK introspection),
  store/texture.go:430,437,454,2419,2457, run_memory_index_diet.go:87,
  run_memory_kind_index.go:54; objectgraph/dolt_store.go:400,404 (`TABLE_SCHEMA = DATABASE()`);
  platform/retrieval_schema.go:301; testdata/fc-deploy1-rollback:31. SQLite has none
  (use `pragma_table_info`, `pragma_index_list`, `sqlite_master`); PG has it but
  `DATABASE()` becomes `current_schema()`.
- **SHOW** (9): computerversion/dolt_state_extractor.go:315,342 (`SHOW TABLES`, `SHOW COLUMNS FROM`),
  dolt_table_hashes.go:41, store/texture.go:314 (`SHOW DATABASES`),
  platform/store.go:447,466,491 (`SHOW COLUMNS ... LIKE`, used by `ensureColumn`).
- **JSON / generated columns** (guest only plus one host line): objectgraph/derived_keys.go:88-91
  (4 derived families, `JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.x'))`),
  objectgraph/dolt_store.go:257,265,280,1252,1268,1289,1475,1526,1530,1757
  (`BIGINT GENERATED ALWAYS AS (...) STORED`, `og_rec_claim`, `idx_og_objects_rec_claim`),
  platform/objectgraph_store.go:439 (`metadataJSONEq`, the one host JSON read).
  SQLite: `json_extract`, generated columns allowed in CREATE TABLE but a STORED
  generated column cannot be added by ALTER TABLE [I] -> define at create time
  (fresh DB after cutover makes the whole `MigrateAccessPaths` machinery
  deletable). PG: `metadata::jsonb ->> 'x'`, `GENERATED ALWAYS AS (...) STORED`,
  `jsonb` casts need valid JSON in `body` (note `JSON_VALID` guard at dolt_store.go:268).
- **Column types** (MySQL-only): `LONGTEXT` (store 46, platform 23, objectgraph 2, routeledger 1, cv 2),
  `LONGBLOB` (store 2, platform 1, objectgraph 4), `DATETIME` (store 38, platform 58,
  objectgraph 4, routeledger 4, cv 2), `UNSIGNED` (store 9: event_table.go:53-54,
  store.go:706-780; platform 19; routeledger 3: sql_ledger.go:31,45,46), `JSON NOT NULL`
  (platform: computer_events.go:49-150, owner_acts.go:68,84, standing_consensus.go:86; 12 columns),
  `AUTO_INCREMENT` (store/store.go:564 `wire_publish_debounce_entries.entry_id`;
  platform/store.go:211 `computer_key_escrow_transparency.seq`). No TINYINT, MEDIUMTEXT,
  ENUM, DECIMAL, ENGINE=, CHARSET, or COLLATE in SQL [V]. PG has no unsigned:
  map to BIGINT (values > 2^63 would break `uint64` scanning through pgx) [I].
  SQLite: types are affinity hints, but `DATETIME` text parsing by modernc depends on
  declared type [I]; `parseTime=true` in the DSN has no SQLite equivalent.
- **Inline `INDEX/KEY` in CREATE TABLE** (MySQL-only, 33 lines): store/event_table.go:73-75;
  objectgraph/derived_keys.go:71-72; platform/{capture_index 6, computer_events 10,
  owner_acts 4, store.go:143, standing_consensus.go:90 (UNIQUE KEY), retrieval_schema 1};
  routeledger/sql_ledger.go (10 + 8 `CONSTRAINT ... FOREIGN KEY`). Move to `CREATE INDEX`.
- **Other MySQL-only DDL**: `ALTER TABLE ... ADD COLUMN ... AFTER col` (platform/store.go:421-433);
  `ALTER TABLE t DROP PRIMARY KEY, ADD PRIMARY KEY (...)` (store/store.go:3648,
  store/texture.go:480-481, 2446-2447; SQLite cannot alter a primary key: table rebuild);
  `DROP INDEX x ON t` (store/run_memory_duplicate_index.go:38, run_memory_index_diet.go:75,
  texture.go:2482; SQLite/PG: `DROP INDEX x`); `CREATE INDEX IF NOT EXISTS` (65 lines; fine in SQLite and PG).
- **Not present** [V]: `GET_LOCK`, `LIMIT x,y`, `ENGINE=`, `CHARSET`, `TINYINT(1)`,
  backtick identifiers inside SQL except `computerversion/dolt_state_extractor.go:277,342,390`
  and `dolt_table_hashes.go:70` (quoting table names; use `"%s"`), `NOW()/CURRENT_TIMESTAMP`
  (times are passed as parameters), `UTC_TIMESTAMP`.
- **Driver semantics** [I]: `clientFoundRows=true` makes `RowsAffected` count matched rows;
  SQLite and PG already do. `multiStatements=true` is relied on by the multi-statement
  DDL consts (`schemaDDL`, `corpusSchemaDDL`, `textureSchemaDDL`, `SchemaDDL`, ...):
  modernc sqlite `Exec` runs multiple statements; pgx runs multiple statements only with
  no arguments (simple protocol). Placeholders: `?` everywhere (about 450 call sites);
  PG needs `$n` rebinding (centralise in the seam, do not edit call sites). Guest uses
  `SetMaxOpenConns(1)` (`configureEmbeddedDoltDB`, texture.go:~343) with three pools
  (write, read, texture; store.go:950-984); SQLite WAL permits parallel readers.

## 2. Classification and binary map

Binary map traced from `cmd/*` imports (`go list -deps`) and `main.go` reads [V].
Almost every binary links the embedded Dolt driver transitively (autoputer,
checkpointd, choir-rebuild-base, corpusd, maild, maildctl, platform-dolt-squash,
projection-compact, proxy, vmctl: 10 of 30 cmd directories import `dolthub/driver` and
`go-sql-driver/mysql` in their dependency closure), so link-size and cgo benefits
arrive only when the last importer goes.

| package | class | runs in |
|---|---|---|
| internal/store (+ storeschema, selfdev, textureowner via API) | guest store -> SQLite | autoputer (guest VM, `cmd/autoputer` -> `internal/autoputer.Run` opens `store.Store`; run.go:195,716,754); also embedded in host tools that replay tape into a scratch store: checkpointd, choir-rebuild-base (via projectionbase.Rebuilder) |
| internal/objectgraph (`DoltStore`) | guest store -> SQLite (schema comment: "identical to corpusd's platform store", dolt_store.go:19-20; platform copy at platform/store.go:~146-175) | autoputer; `objectgraph.NewDoltStore` at store/store.go:967,984, texture.go:177 |
| internal/computerversion: dolt_state_extractor.go, dolt_table_hashes.go | witness (guest + host tooling) | autoputer (agentcore checkpoint/rematerialize/replay), checkpointd/rebuilder (projectionbase/witness.go) |
| internal/computerversion/input_resolver.go (2 tables) + internal/routeledger (4 tables) | host control (Store A) | vmctl (`cmd/vmctl/main.go:205-249`, env `VMCTL_ROUTE_DSN`) |
| internal/platform (control files) | host control: Store A -> PG `control` | corpusd (`cmd/corpusd/main.go:43`), checkpointd (`cmd/checkpointd/main.go:59`, `OpenStoreWithoutBootstrap`) |
| internal/platform (corpus files: objectgraph_store, retrieval_schema, capture_index, publication_front/opinion/related/related_backfill, service_publication_read, artifact_gc line 447, store.go corpus part) | host corpus: Store B -> PG `graph` | corpusd, checkpointd (open only) |
| internal/doltbatch | host (both) | corpusd, checkpointd (delete) |
| internal/projectionbase (witness.go, squash.go, compact.go, rebuilder.go, install/parts/capture/rebase) | tooling + projection-base pipeline (workspace directory as artifact) | checkpointd, choir-rebuild-base, projection-compact, platform-dolt-squash; install side in autoputer (`projection_base.go:247-253` `isStoreEmpty` keys on `.dolt` / `*.db`) |
| internal/dbcred, internal/platform/config.go | host DSN/credential plumbing | corpusd, checkpointd, vmctl |
| cmd/projection-compact, cmd/platform-dolt-squash, cmd/choir-rebuild-base | tooling/CLI (delete or rewrite) | operator / nix timer (`nix/node-b.nix:2322 platformDoltSquashScript`, `2369 platformDoltHistoryAudit`) |
| scripts/ograph-bench (Go) | tooling (delete) | benchmark only |
| internal/auth, maild, actor, actorruntime, base/journal, gateway/searchplane | already SQLite; no port | auth, maild/maildctl, autoputer, gateway |
| internal/agentcore, textureowner, apihandler, content, mediastate, desktopstate, researchtools, actorruntime | other: consume `store.Store` API (72 + 21 + 6 + ... non-test importers) | autoputer |

## 3. Schema sources and table counts

Schemas are all Go string constants applied at open with `CREATE TABLE IF NOT EXISTS`
plus imperative migrations. There is no migrations directory and no
`.sql` file on the product path (`scripts/ograph-bench/schema.sql` is bench only) [V].

**Guest store (embedded Dolt, database `texture`): 26 tables** [V; matches the
"24 of 26 tables" in selfdevprotocol/witness_v2.go:20].
- `store/store.go` 16: events, run_memory_entries, wire_publish_debounce_entries,
  wire_publish_debounce_state, co_super_slots, media_progress, media_recents,
  user_preferences, desktop_workspaces, desktop_sessions, desktop_app_instances,
  desktop_window_placements, computer_event_projection_heads, computer_event_index,
  self_development_start_intents, self_development_operations (22 CREATE INDEX).
- `store/texture.go` 6 (`textureSchemaDDL`, 13 CREATE INDEX): texture_revisions,
  texture_source_entities, texture_source_refs, texture_document_aliases,
  texture_agent_mutations, agent_evidence.
- `store/event_table.go` 1: choir_events (inline INDEX x3).
- `objectgraph/dolt_store.go:21-52` `SchemaDDL`: og_objects, og_edges (+5 indexes);
  `objectgraph/derived_keys.go` og_key_index (derived, ignored by the witness).
- Imperative migrations to delete after a fresh-DB cutover [I]: `retiredTextureTables`
  drops, `ensureTextureColumn` (texture.go:~395-470), PK re-scope
  (store.go:3625-3650, texture.go:2417-2490), `store/derived_columns.go` (331 LOC),
  `autoputer/derived_columns.go` (100), `store/run_memory_{index_diet,kind_index,duplicate_index}.go`
  (405+127+119), `store/vocab_migrate*.go` (504+1069), `store/residue_import.go` (671).
- Epoch: `storeschema.Version = 3` with a receipt file `store-schema.json` inside the
  workspace directory (storeschema/schema.go:43-80); `LeaseUntrusted` marker likewise
  (projectionbase/install.go:210). These are file-in-directory conventions; they survive
  if the workspace stays a directory holding the SQLite file.

**Store A (platform db, port 13306): 33 tables** [V]: `platform/store.go:180` `schemaDDL` 4
(computer_key_escrows, computer_key_unwrap_requests, computer_key_unwrap_approvals,
computer_key_escrow_transparency); `computer_events.go` 8 (computer_event_heads,
computer_event_append_receipts, control_key_history, computer_self_development_modes,
computer_lifecycle_operations, computer_lifecycle_receipts, computer_checkpoints,
computer_route_projection_certificates); `projection_jobs.go` 7; `owner_acts.go` 3;
`standing_consensus.go` 2; `file_cas.go` 2; `tape_tombstone.go` 1; and in the same server
for vmctl: `routeledger/sql_ledger.go` 4 + `computerversion/input_resolver.go` 2.
Migrations: `ensureColumn` via `SHOW COLUMNS` (store.go:~436-455), `ALTER ... AFTER` x4
(store.go:421-433, `build_commit`, `expected_pending_transition_ref`).

**Store B (corpus db, port 13307): 10 tables** [V]: `platform/store.go:70` `corpusSchemaDDL` 7
(publication_transclusions, publication_version_proposals, proposal_delivery_records,
platform_texture_documents, platform_texture_revisions, og_objects, og_edges) +
`retrieval_schema.go` og_lookup_keys + `capture_index.go` proj_capture_index,
proj_capture_terms (the two in `dolt_ignore`). Shared-pool mode (no
`CORPUSD_CORPUS_DOLT_DSN`) puts A and B in one database; PG split must keep `corpus()`
as the only handle to B (it is: 18 of 20 uses go through `s.corpus()`; the exceptions
are `platform/store.go` bootstrap and `corpusCommitter`).

Already-SQLite schemas (no port): auth 11 tables (+ `users_new` rebuild), maild 22,
actor 3, journal 2, searchplane 2.

## 4. Dolt semantics relied on, and minimal replacements

| # | Dolt feature | relied on by (product path) | verified usage | replacement under SQLite / PG |
|---|---|---|---|---|
| 1 | `DOLT_COMMIT` guest checkpoints + dirty-gen/debounce/count cadence (store.go:190-215,330-420; checkpoint_cadence.go 584 LOC; owner setting `store.debounced_checkpoint`, env `CHOIR_GUEST_CHECKPOINT`) | none in reads: "reads never depend on it" (texture.go:1239); durability is SQL COMMIT + tape (store.go:216, doltbatch/committer.go:1-9) | [V] no `AS OF`/`dolt_log` reader exists | Delete. SQLite WAL + `synchronous` setting gives the durability; the close-time commit becomes `PRAGMA wal_checkpoint(TRUNCATE)`. Remove `markDoltHistoryDirty` x7, `commitDoltCheckpoint*`, `finalizeSinceCheckpoint/doltCheckpointEvery`, `/health` checkpoint counters (keep field names as zeros for one release if the host probes read them [I]). |
| 2 | Commit for `AS OF` reads | claimed in comments (store.go:216,337) | [V] zero executable `AS OF`; only `scripts/ograph-bench` | None needed. Delete comments; delete `scripts/ograph-bench`. Time travel, if wanted, is the tape (not SQL). |
| 3 | Table hashes in the v2 witness (`DOLT_HASHOF_TABLE`, relies on prolly-tree history independence: tape-replayed store == live store) | `agentcore/checkpoint_restore_bindings.go:143-170` (guest, under append hold), `rematerialize.go:190-194`, `projectionbase/witness.go:51` (host base build), judged by platform checkpoint verdicts and `cmd/checkpointd/verdicts.go`; `selfdevprotocol/witness_v2.go`, `restore.go:40` (`DoltTables`, `DoltHead`), `replay_eligibility.go:21,195,280` (`emptyDoltTableHash`, `dolt:texture:table:` keys) | [V] | No native equivalent. Replace with an order-independent per-table digest of canonical row encodings: keep a `_table_digest(table, digest, rows)` side table updated in the same transaction (SQLite: register a Go scalar function via modernc `RegisterDeterministicScalarFunction` and use triggers, or compute in the write path), combining row hashes with a multiset hash (sum of 256-bit row digests, or LtHash) so insert/update/delete are O(1); fallback is the existing v1 row witness (`DoltStateExtractor.Extract`: `SELECT * ... ORDER BY` in canonical JSON, already history-independent, but it took "seconds on d@e and minutes on the paper" per witness_v2.go:7-10). Schema hash: hash `sqlite_master` SQL, not `SHOW COLUMNS` output (Dolt type strings change, so every old witness and base becomes unjudgeable: a new witness version v3 and a fresh base after cutover; the goal already deletes old data) [I]. `DoltHead` / `dolt:<db>:head` (= `HASHOF('HEAD')`, audit-only per dolt_state_extractor.go:42-47) becomes the content-root digest or is dropped; `replay_completeness.go:545` filters the `dolt:*:head` key, so rename in one place. Keep operational-column exclusions (`computer_event_index.prepared_at/finalized_at`, `computer_event_projection_heads.updated_at`): witness.go:25-37. |
| 4 | Texture history checkpoint (`commitDoltCheckpoint` before the chain walk, texture.go:~1250) | `GetHistory`/`GetHistoryByScope` | [V] the walk uses the explicit parent chain via `GetRevision` with a cycle check (texture.go:1255-1275) | Delete the single call. The chain is already the authority (texture.go:1235-1241). Removes a write from a read path. |
| 5 | Store A / projection squash and GC (`DOLT_RESET --soft`+`DOLT_COMMIT`+`DOLT_GC --full`; `platform-dolt-squash`, `projection-compact`, nix timer, `dolt-history-audit.sh`) | host maintenance only; ledger-neutral (refuses unless root hash unchanged: squash.go:92,249) | [V] | PG: delete (autovacuum; no commit history exists). Guest: `VACUUM`/`PRAGMA incremental_vacuum` on a schedule replaces `dolt_maintenance.go` (495 LOC) and `startPeriodicDoltGC` (autoputer/run.go:716,754,773); remove `ReadDoltGCDisposition` and `RUNTIME_DOLT_GC_*` envs. Scratch-store GC in `rebuilder.go:459` is deleted (scratch is a throwaway file). |
| 6 | Branch isolation (`DOLT_BRANCH/CHECKOUT/MERGE/TAG/RESET`) | none | [V] test-only (`dolt_branch_isolation_pinned_test.go`) | Delete the test. Candidate/capsule isolation is already file-level (capsules are guest-local copies per CLAUDE.md); SQLite candidate = file copy via `VACUUM INTO` or the backup API [I]. |
| 7 | Debounced Store A/B snapshots (`doltbatch.Committer`: Mark/CommitNow, modes, `PLATFORM_DOLT_SNAPSHOTS`, `DOLT_SNAPSHOT_DEBOUNCE`) | 40 call sites: `markDirty/markCorpusDirty/commitBoundary/CommitNow` in 20 platform files (projection_jobs 8, key_escrow 6, objectgraph_store 4, tape_tombstone 3, control_keys 3, ...) | [V] | Delete package and call sites; PG transaction commit is the durable point. Recovery-boundary writes ("checkpoints, replay watermarks", committer.go:6-9) need nothing extra. `commitTapeTombstone` / `file_cas_retire.go:185` error text "rows deleted, dolt commit failed" disappears. |
| 8 | `dolt_ignore` for `proj_capture_index/terms` | Store B capture projection (`capture_index.go:94-105`) | [V] | Delete `ensureCaptureIndexTables`'s ignore step; in PG these are plain tables (optionally `UNLOGGED`; they are rebuildable projections [I]). |
| 9 | Embedded engine non-thread-safety -> `engineMutex`, `MaxOpenConns(1)`, three pools, stall-holder diagnostics | every guest store call | [V] dolt_store.go:62-66, engine_mutex.go | SQLite WAL: remove the global engine mutex; one writer connection + N readers; `busy_timeout`; `BEGIN IMMEDIATE` where code relied on `FOR UPDATE` row locks (7 guest sites). `api_health_stall_holder.go` (dolthub stack prefix) retargets or is deleted. |
| 10 | Workspace = directory of Dolt noms files, tarred / parted / installed / rebased as the projection base | `projectionbase/{install,parts,capture,rebase,seed,verify}.go` (`TextureWorkspacePath` x14), `autoputer/projection_base.go:247-253`, checkpointd | [V] | The base becomes one SQLite file (+ marker + `store-schema.json`). Capture must produce a consistent image (checkpoint WAL then copy, or `VACUUM INTO`/backup API) [I]. Keep `<path>.texture` directory convention so install/normalise code (install.go:182-196) is unchanged. |
| 11 | Dolt server auth/ports/DSN, liveness probes, users units, off-host backup of Dolt dirs | nix/node-b.nix (186 `dolt` mentions: services at 1113,1163; users units 1196-1197; liveness 2449-2481), `scripts/` (40 files), `docs/` (414 files) | [V] counts | PG roles + `pg_isready` probe; backup per goal file line 158 (WAL archiving). Not Go; belongs to its own work package. |
| 12 | `clientFoundRows`, `parseTime`, `multiStatements` driver behaviours | all SQL | [I] | See section 1b "Driver semantics". |
| 13 | Row-lock / `INSERT IGNORE` / `ON DUPLICATE` idempotency idioms | claim-once paths (projection jobs, file CAS, checkpoints, key escrow, route ledger) | [V] sites above | PG: `ON CONFLICT DO NOTHING`, `ON CONFLICT (pk) DO UPDATE SET c = EXCLUDED.c`, `SELECT ... FOR UPDATE` unchanged. SQLite: same `ON CONFLICT` syntax (3.24+). One helper builds both. Caveat: MySQL `INSERT IGNORE` also swallows non-unique errors (truncation, FK); `ON CONFLICT DO NOTHING` does not: a behaviour tightening that tests may expose [I]. |
| 14 | DDL under live engine for derived columns (`MigrateAccessPaths`, per-step engine holds, boot refusal when table has rows: autoputer/derived_columns.go:12-35) | guest boot/operator endpoint `/internal/store/derived-columns` | [V] | Fresh SQLite schema includes the generated columns and indexes from creation; delete the migration state machine and the operator endpoint (and `restore_run_memory_indexes`). Needs a decision on STORED vs VIRTUAL generated columns in SQLite [I]. |

## 5. Proposed work packages

Principle: disjoint by file (directory where possible); one seam lands first.
Estimates are from the scans above; hours are not estimated.

### WP0 (seam, lands first, 1 builder): `internal/dbx` (new) + DSN plumbing

Deliverables: (a) `dbx.OpenSQLite(path, opts)` (modernc, WAL, `busy_timeout`,
`foreign_keys`, one write conn + read pool, `BEGIN IMMEDIATE` helper) and
`dbx.OpenPG(dsn, opts)` (pgx stdlib); (b) a `Dialect` value with:
placeholder rebinding (`?` to `$n`), `Upsert(table, keyCols, updateCols)`,
`InsertIgnore`, `ForUpdate()` (`""` on SQLite), `JSONText(col, path)`,
`ColumnExists/TableExists/IndexExists` introspection, `AddColumnIfMissing`,
`DropIndex(name, table)`, identifier quoting, type map (`LONGTEXT`->`TEXT`,
`LONGBLOB`->`BLOB`/`BYTEA`, `DATETIME`->`TIMESTAMP`/text, `UNSIGNED`->`BIGINT`,
`AUTO_INCREMENT`->`AUTOINCREMENT`/`GENERATED ... AS IDENTITY`), error
classification (unique violation, busy/locked, serialization failure);
(c) test harnesses: `dbxtest.SQLite(t)` and `dbxtest.Postgres(t)` (PG per-test
database from a template; the only way the 45 platform test files and 5
cross-package tests can stop using embedded Dolt); (d) rewrite
`internal/dbcred` for PG DSNs (71 LOC, `mysql.Config` use at dbcred.go:25) and
`platform/config.go` DSN names; (e) mechanical pre-split of
`platform/store.go` into `store.go` (pools, Bootstrap order, A) and
`corpus_schema.go` (`corpusSchemaDDL`, corpus column ensures), so WP-D and WP-E do not
edit the same file.
Size: 1 new package (~700-1000 LOC [I]), 3 existing files touched. Blocks all others.

### WP-A guest store: `internal/store` (+ `internal/storeschema`, `internal/selfdev`, `internal/autoputer/run.go`, `internal/agentcore/api.go` health fields, `internal/gomemlimit`)
- 74 non-test files (44.9k LOC), 188 call sites, ~230 SQL literals, 26 tables
  (22 in store/texture/event_table + shared graph tables via WP-B), 14 ODKU, 1 IGN,
  7 FOR UPDATE, 12 information_schema, 5 MySQL-only ALTER, 3 inline INDEX,
  ~110 MySQL-type lines.
- Deletes: `dolt_maintenance.go` (495), `checkpoint_cadence.go` (584), commit paths
  in `store.go`, `markDoltHistoryDirty` x7, texture history checkpoint (texture.go:~1250),
  retired-table/PK/column migrations, derived-column and run-memory-index migrations
  (~1500 LOC) if the cutover is fresh-DB.
- Tests: 142 files / 46.6k LOC; Dolt-specific ones listed in 1a
  (`checkpoint_cadence_test`, `journal_amplification_census_test`, `online_gc_*_test`,
  `derived_columns_test`, `run_memory_*_index_*_test`, `stall_holders_census_test`).
- Interface to B: the constructor calls `objectgraph.NewDoltStore(db)` at
  store.go:967,984 and texture.go:177; agree the renamed constructor
  (`objectgraph.NewSQLStore`) in WP0 so A and B land independently.

### WP-B guest object graph: `internal/objectgraph` (+ `internal/autoputer/derived_columns.go`)
- 14 non-test files (5.5k LOC): `dolt_store.go` 1989, `derived_keys.go` 934,
  `engine_mutex.go` 227, `newest_headers.go`. ~47 SQL literals, 65 call sites,
  4 ODKU, 1 REPL, 1 FOR UPDATE, 15 JSON lines, 3 generated-column lines,
  2 information_schema, 2 inline INDEX.
- Work: rename `DoltStore` -> SQL store, SQLite generated columns at create time,
  remove `engineMu` (77 refs) and `MigrateAccessPaths` machinery, `og_key_index`
  `REPLACE INTO` -> `ON CONFLICT`, `json_extract`.
- Tests: 17 files / 5.1k LOC (`dolt_store_test` 13 Dolt-open helpers,
  `engine_concurrency_probe_test`, `generated_columns_test` with `EXPLAIN PLAN`,
  `derived_keys_test`, `boot_discovery_test`).

### WP-C witness, replay, projection base (guest-host protocol): `internal/computerversion` (dolt_*.go only), `internal/selfdevprotocol` (witness_v2.go, restore.go), `internal/projectionbase`, `internal/agentcore` (checkpoint_restore_bindings, rematerialize, replay_completeness, replay_eligibility, api_health_stall_holder), `internal/autoputer/projection_base.go`, `cmd/checkpointd` (dolthome.go, capture.go), `cmd/choir-rebuild-base`, `cmd/projection-compact`, `cmd/platform-dolt-squash`
- Non-test: ~17 + 3 + 4 + 5 + 8 files; SQL: ~16 Dolt-only statements (squash.go 10,
  dolt_table_hashes 2, dolt_state_extractor 5 SHOW/HASHOF/backtick).
- Work: define witness v3 (per-table digest + sqlite_master schema hash), rewrite
  extractor on SQLite, delete squash/compact/GC and the three tool binaries (plus
  `scripts/platform-dolt-squash.sh`, `test-platform-dolt-squash.sh`), adapt install /
  parts / capture / rebase to a single SQLite file, update `isStoreEmpty`.
- Protocol surface touched on the host side (verdict judge, `computer_checkpoints`
  witness JSON): coordinate with WP-D through the `selfdevprotocol` types only.
- Tests: projectionbase 16 files / 4.5k LOC (squash_test 399, compact_test 173,
  rebuilder tests 1.2k), computerversion dolt tests (extractor 342, smoke, branch
  isolation 193 to delete), agentcore replay/rematerialize tests.

### WP-D host control, Store A -> PG `control`: `internal/platform` control files, `internal/doltbatch` (delete), `internal/routeledger`, `internal/computerversion/input_resolver.go`, `cmd/vmctl/main.go`, `cmd/corpusd`, `cmd/checkpointd` (main.go, worker.go open path)
- ~33 platform files + 2 routeledger + 1 computerversion: tables 33 (27 platform +
  4 routeledger + 2 inputs), ~150 call sites [I] (platform total 233 incl. URL
  `.Query()` false positives; B's ~78 subtracted), ~110 SQL literals, 15 ODKU
  (platform) + 4 (routeledger/cv), 10 IGN, 13 FOR UPDATE + 5 elsewhere,
  22+18 inline INDEX/FK lines, `ALTER ... AFTER` x4, `SHOW COLUMNS` ensureColumn,
  ~40 `markDirty/commitBoundary` call sites to delete, 12 JSON columns to `jsonb`/`text`,
  3 BIGINT UNSIGNED groups (route generations, event sequences).
- Tests: `openTestPlatformStore` in 45 files; platform tests 77 files / 21.7k LOC;
  `snapshot_mode_test.go`, `doltbatch/committer_test.go` deleted.

### WP-E host corpus, Store B -> PG `graph`: `internal/platform` corpus files (objectgraph_store.go, retrieval_schema.go, capture_index.go, publication_front/opinion/related/related_backfill.go, service_publication_read.go, artifact_gc.go:447, corpus_schema.go from WP0)
- ~9 files, ~3.5k LOC, 10 tables, ~78 call sites [I], ~60 SQL literals, 11 ODKU/IGN
  lines (objectgraph_store 4+2, retrieval_schema 3, capture_index 3), 1 REPL
  (capture_index.go:273), 1 JSON predicate (objectgraph_store.go:439), 1
  `information_schema` (retrieval_schema.go:301), `dolt_ignore` (capture_index.go:100-104)
  to delete, `EXPLAIN PLAN` test.
- Shares only the pool handle with WP-D (`s.corpus()`); interface is `*sql.DB` +
  dialect from WP0.
- Tests: `retrieval_store_b_test`, `capture_index_test`, publication tests.

### WP-F migrators and infrastructure (new code + non-Go)
- New one-shot packages: guest Dolt->SQLite converter (reads the old workspace with
  the embedded driver, writes the new file, seals first snapshot), host Store A/B ->
  PG copier (mysql driver). These are the only code allowed to keep the Dolt/mysql
  imports; they are deleted after C5.
- `nix/node-b.nix` (Dolt services 1113/1163, users units, probes, squash/audit timers,
  all `CORPUSD_DOLT_DSN`/`VMCTL_ROUTE_DSN` lines 1270,1380,1622), `start-services.sh:64-93`,
  `flake.nix` (dolt package), `scripts/` (40 files mention Dolt: backup, restore drill,
  liveness, density, hardening probes, `ograph-bench`), `go.mod`/`vendor/`
  (`go mod tidy && go mod vendor` after C5), CI shards that name Dolt-heavy packages.
- Size: 2 new packages, ~6 nix/script files substantive, ~35 files with text edits [I].

Collision check: files appear in exactly one WP except `platform/store.go`
(split in WP0), `autoputer/*` (run.go -> A, derived_columns.go -> B,
projection_base.go -> C) and `agentcore/api.go` (A, health fields only);
`computerversion` is split by file (C: dolt_*.go; D: input_resolver.go).

## 6. Residual unknowns [I]

- Statement counts exclude SQL assembled from fragments; the `store` package has
  more call sites per statement than `platform` because of helper wrappers.
  A compile-driven port (swap the driver, fix errors, run tests) is the safe
  completeness check; this census is the plan, not the proof.
- Whether `BIGINT UNSIGNED` columns ever hold values above 2^63 (hash-like
  sequences) was not checked; event sequence and generation columns look small.
- Timestamps: `DATETIME` columns scanned as `time.Time` rely on `parseTime=true`;
  how each package stores UTC vs local was not audited.
- PG strictness (GROUP BY, implicit casts, `LIKE` collation, `ORDER BY` with
  mixed NULLs) was not audited; 18 `GROUP BY/HAVING/COUNT(DISTINCT)` lines in platform
  are the likely first failures.
