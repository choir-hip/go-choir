# Run telemetry in its own table (F-c): design (2026-10-22)

Date: 2026-10-22. Station: FP1 follow-up, lever F-c of
[`paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22`](../problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md).
Lane `lane/fp1-event-table-design`, from `origin/main` `6b3bdcc1b`.
Mutation class of this document: green (docs only; no code, nothing run on
any computer, no SQL against any store). Mutation class of the change it
designs: **red** (store schema, projection, replay, content witness, event
readers, sequence allocation). Section 6 has the ceremony.

Evidence labels, as in the problem record: **[V]** verified by reading the
code at the cited `file:line` on this checkout; **[M]** measured, in a cited
record, not re-measured here; **[H]** hypothesis or derivation, with the
observation that settles it.

## Answer in brief

1. **What moves.** Only the ten batched telemetry kinds
   (`types.EventKind.Batchable()`, `internal/types/event_durability.go:31-45`),
   about 94% of `choir.event` appends on staging **[M]**. Run-state,
   acknowledged and evidence events stay in `og_objects` exactly as today.
2. **How a row gets there.** The writer stamps the existing, unused
   `ProjectionOp.Table` field (`internal/computerevent/projection_batch.go:68`)
   with `choir_events`. The op kind is still `object_recorded` and the body
   is still the same `objectgraph.Object`. A new binary's projector routes
   on the stamp. An old binary ignores the field and upserts into
   `og_objects` as it does today. The tape stays the single authority, and
   replay is a function of the tape alone.
3. **The table.** `choir_events`. Its primary key is the tape position
   `(tape_seq, op_index)`, which is sequential and unique by construction.
   It has three secondary indexes, one per reader shape:
   `(run_id, seq)`, `(owner_id, stream_seq)` and `(trajectory_id, stream_seq)`.
   That is **4 prolly trees per insert instead of 13**.
4. **Estimate.** About 30 to 38 KiB of journal per batched op instead of
   86 to 115 KiB, about 65% less **[H]**. For one telemetry record, including
   the per-event overhead, that is about 98 KB to about 43 KB with batching
   off, and about 88 KB to about 33 KB with batching on. On the paper's
   10-15 kind mix the whole journal falls **about 30 to 40%**: 1.6 GiB per
   10 minutes becomes about 1.0 to 1.1 GiB. F-c alone does not end the F2
   stall. With F-a and a multi-row insert into the new table (F-e on
   `choir_events`, deploy 3) the fall is about 50%.
5. **Rollout.** Three deploys:
   - Deploy 1 is inert. It adds the table, the projector route, the union
     readers, the witness and the manifest.
   - Deploy 2 adds the writer behind a per-computer setting,
     `store.event_table`, which is off by default and is an owner act on the
     tape. It runs on one test computer first.
   - Deploy 3 is the multi-row insert.

   Existing `choir.event` rows in `og_objects` are never touched. Readers
   read the union of the two tables.

## 0. The problem in one paragraph

On the paper, every tape event pays its journal bytes per touched prolly
tree. An `og_objects` row touches 13 trees: the primary key and 12 secondary
indexes. This is in the record's "What one tape event writes on the guest"
section **[V]**, and here in `internal/objectgraph/dolt_store.go:20-36`
(the table and two indexes), `:219-226` (metadata columns), `:250-257`
(record columns) and `:301-328` (the other indexes) **[V]**. Each projected
op is its own statement, and each statement flushes its own root-to-leaf
path in every tree
(`prolly_table_writer.go:320-326`, cited in the record) **[V]**.

On staging, a fit over the batching gate gives about 86 KB of store growth
per op and about 12.5 KB per tape event, with r² = 0.985
(`docs/evidence/telemetry-batching-phase1-staging-2026-10-22.json`,
`journal_effect.fit`) **[M]**. So batching, which removes tape events but
not ops, trims a telemetry record's journal by only about 10%. The lever is
the number of trees an op touches. `choir.event` is the dominant kind by
volume: 58% of the paper's tape events on 10-15, against 31% for
run-memory entries **[M]**.

## 1. Readers and writers (standing question 4)

All production callers go through the concrete `*store.Store`. No interface
declares the event methods. The store fields are concrete:
`internal/agentcore/runtime.go:95`, `internal/apihandler/handler.go:14` and
`internal/textureowner/handler.go:48` **[V]**.

### 1.1 Writers of `choir.event` objects

| # | Writer | Path to the tape | Kinds | After F-c |
|---|---|---|---|---|
| W1 | `Store.AppendEvent`, acknowledged branch (`internal/store/store.go:1964`, `:2100`) | `AppendEventOG` (`graph_store.go:1168-1199`) → `ogPut` → `objectgraph.Service.CreateObject` → `DoltStore.PutObject` → write validator → `interceptOG` (`projection_tape.go:61-89`) → one `object_recorded` op | every kind | Batched kinds with the setting on: the op is built directly, as the sealer builds it, then run through `PreflightWrite` and stamped `Table: choir_events`. All other kinds: unchanged |
| W2 | Telemetry sealer (`telemetry_sealer.go:109-134` `newTelemetryRecord`, which calls `buildEventObject`, `graph_store.go:1205-1239`) | the sealed batch under `authority:vm-local-projection-telemetry` (`projection_tape.go:106-109`) | batched kinds only (`recheckTelemetryOp`, `telemetry_sealer.go:486-506`) | stamped when the setting is on |
| W3 | Lifecycle fold: `projectLifecycleRun` (`lifecycle.go:3116`, through `projectLifecycleRunWithRetry`, `:3002`) puts a `buildEventObject` into its `PutBatchConditional` with an absent-condition on the event's canonical id (`:3330-3337`). The other absent-conditions in `lifecycle.go` (`:4394`, `:5037`, `:5113`, `:6209`) are `choir.lifecycle_event` objects, a different kind | `putBatch` → `evaluateConditions` against `og_objects` (`dolt_store.go:1133-1140`, `:1798-1838`) → intercept | run-state kinds: `loop.started`, `activation.passivated`, `loop.completed` and the rest. These are never batched (`event_durability.go`) | **unchanged**: stays in `og_objects`, and the condition keeps its meaning |
| W4 | Projector (`project.go:215-256` `projectObject`, one upsert into `og_objects`; `derived.Maintain` is a no-op for this kind because no `og_key_index` family covers it, `derived_keys.go:76-80`, `:297-305`) | live finalize (`computer_events.go:227`), replay finalize (`:222`), dry run (`:114`) | all | routes stamped ops to `choir_events` (section 2.2) |
| W5 | Vocabulary migration (`vocab_migrate_og.go:347`, rewrites rows in place) | not tape; a one-time V1→V2 cutover | all | does not touch `choir_events` (section 3.6) |
| W6 | Unbound store, the test seam (`projection_tape.go:22-24`) | direct SQL into `og_objects` | all | unchanged: no stamp without a tape |

`HandleInternalRunEvents` POST (maild, `internal/maild/trace_events.go:123-125`)
and every runtime emitter go through W1. No other writer exists **[V]**: the
only code that names `ogKindEvent` is in `internal/store` (`graph_store.go:22`).

### 1.2 Readers through the store's event methods

| Store method (file:line) | Production callers | Surface | Filter | Order / cursor / limit |
|---|---|---|---|---|
| `ListEventsAfter` (`store.go:2126`) | `api_run_events.go:73` (relaxed read) | `GET /api/runs/{id}/events` (`apihandler/routes.go:22`), used by `choir trace` (`cmd/choir/trace.go:327-358`), the Playwright helper (`frontend/tests/helpers/run-trace.js:11-30`), `scripts/reeng_tracer.mjs:436`, `:457`, `scripts/reeng_ttl_session.mjs:101`, `scripts/claude_delegate_probe.py:79`, `scripts/management_latency_probe.py:127`. Acceptance evidence links have the form `/api/runs/{id}/events?after=<seq-1>&limit=1` (`run_acceptance.go:343-353`) | run_id | seq ascending; `after` is a seq cursor; limit up to 1000 (+1) |
| `ListEvents` (`store.go:2120`, delegates to the method above) | `api.go:436` (consistent); `run_memory.go:437`, which is unreachable in production: it is gated on reason `continuation_selection` (`run_memory.go:432-434`) and no caller passes it | `GET /internal/runtime/runs/{id}/events` (`routes.go:56`), desk tooling | run_id | seq ascending; up to 1000 |
| `ListEventsByOwnerAfter` (`store.go:2189`) | `live_ws.go:79` (consistent), then a desktop filter (`live_ws.go:85`, `:174`) | WebSocket `/api/ws?after_seq=` (`routes.go:38`); the frontend cursor is the highest `stream_seq` seen (`frontend/src/lib/Desktop.svelte:981-1043`) | owner | stream_seq ascending, after a cursor; 500 |
| `ListEventsByOwner` (`store.go:2157`) | `apihandler/costs.go:74` (relaxed); `textureowner/texture.go:1969` (relaxed) | `GET /api/costs` (`routes.go:27`), used by `scripts/cpu_efficiency_probe.py:74`; `GET /api/texture/documents/{id}/diagnosis` (`routes.go:82`), which `TextureEditor.svelte:1396` calls with limit 80 | owner | newest first by timestamp; up to 10000 (costs) or 500 (diagnosis) |
| `ListEventsByTrajectory` (`store.go:2253`) | `run_acceptance.go:91` (consistent; it sorts again at `:94`); `texture_workflow_verifier.go:126` has **no caller** | `POST /api/run-acceptances/synthesize` (`routes.go:53`) | owner + trajectory_id | stream_seq ascending; 3000 |
| `ListEventsByChannel` (`store.go:2224`) | `system1_verb.go:176` only | the `choir.System1` desk egress; the channel is `system1:<computer>` (`:81`), written by `:323` with kind `EventSystem1Judgment`, which is **acknowledged** (`event_durability.go:66`) | owner + channel_id | timestamp ascending; 10000 |
| `ListEventsByTrajectoryAfter` (`store.go:2283`) | **none** (only `telemetry_sealer_test.go:688`) | — | — | — |
| `ListEventsOG` (`graph_store.go:1242`) | **none** (only `lifecycle_test.go:315`) | — | — | — |
| `ListEventsByOwnerOG` (`graph_store.go:1263`) | **none** | — | — | — |

Every list method first calls `beforeEventRead` (`telemetry_reads.go:41-50`).
This is the sealer's read barrier. F-c leaves it as it is.

### 1.3 Readers inside the write path (sequence seeds)

- **Per-run `seq` seed** (`store.go:2012-2045`). On a run's first append in
  each process it reads `ogListByMetadata(choir.event, run_id, 10000)` and
  takes the maximum `seq` for that owner. With the sealer it adds
  `telemetrySeqGap`.
- **`stream_seq` seed** (`store.go:2054-2074`). On the first append in each
  process it reads `ListObjects{Kind: choir.event, Limit: 500}`, the newest
  by `updated_at` (`dolt_store.go:719-760`), and takes the maximum.

Both read `og_objects` only. **After the cutover they must read both
tables, or numbers are handed out twice** (failure F1, section 5).

### 1.4 Readers that scan `og_objects` for every kind

| Reader | What it does with `choir.event` rows | After F-c |
|---|---|---|
| Vocabulary plan and serving fence (`vocab_migrate_og.go:347`, `:871`; event fields `kind` and `phase` are skipped, `:835-838`), reached from `MigrateAndFenceServingVocabulary` (`autoputer/run.go:693`, `:765`; `projectionbase/rebuilder.go:390`; `agentcore/rematerialize.go:204`). Boot takes a fast path when nothing was replayed (`vocab_migrate.go:219-225`) | scans role leaves | the full fence pass also reads `choir_events` (section 3.6) |
| `ListObjectGraphFingerprints` (`object_graph_fingerprints.go:27-60`), used **only as a diagnostic** when live and replay differ (`replay_completeness.go:352-361`, `checkpoint_restore_bindings.go:181-182`) | fingerprints every row | gains a `choir_events` fingerprint list |
| `ReadObjectSnapshot` (`dolt_store.go:767`), `snapshotResidueObjects` (`residue_import.go:240-247`) | filter on `owner_id AND computer_id`. Events carry `computer_id = ''`, because `ogPut` sets none (`graph_store.go:91-111`, `service.go:125-136`) | not affected |
| Content extractor and replay eligibility | hash and classify every table (section 3.3) | explicit wiring |
| `og_key_index` (derived keys) | no family for `choir.event` (`derived_keys.go:76-80`) | not affected |

### 1.5 Surfaces outside Go

The frontend reads events only through `/api/ws` catch-up and the texture
diagnosis. CLI, scripts and Playwright read `/api/runs/{id}/events` and
`/api/costs`. **No route, cursor or response shape changes**, so none of
them change. No script or proof queries `og_objects` for `choir.event`
(`scripts/proofs/fp1-store.sh:64` checks only the generated columns) **[V]**.
Live WebSocket delivery comes from the runtime bus, not the store
(`runtime.go:1970`), so it is not affected.

### 1.6 What the readers need: the index set

| Need | Readers | Served by |
|---|---|---|
| A run's events in `seq` order after a cursor; `MAX(seq)` for a run | `ListEventsAfter`, `ListEvents`, per-run seed | `idx_choir_events_run_seq (run_id, seq)` |
| An owner's events in `stream_seq` order after a cursor; an owner's newest N | WS catch-up, costs, diagnosis | `idx_choir_events_owner_stream (owner_id, stream_seq)` |
| A trajectory's events in `stream_seq` order | acceptance synthesis | `idx_choir_events_trajectory_stream (trajectory_id, stream_seq)` |
| The store-wide newest `stream_seq` | stream seed | the primary key read backwards (newest 500 by tape position) |
| A channel's events | System1 egress only, whose kind is acknowledged | **none**: post-cutover channel reads stay on `og_objects` (section 2.1) |

### 1.7 Deletion citers (deleted in deploy 1, not extended)

`ListEventsByTrajectoryAfter`, `ListEventsOG` and `ListEventsByOwnerOG` have
no production callers. Each is deleted and its test repaired. They are not
made union-aware. `ogEdgeEventRun` (`graph_store.go:56`) is never written.
It is noted here and left alone. Also noted and out of scope:
`VerifyTextureWorkflow` has no caller, and `compactRunEventLedger` is
unreachable.

## 2. The table

### 2.1 Which events move: the ten batched kinds only

The routing rule is `rec.Kind.Batchable()`. The ten kinds are `loop.delta`,
`loop.progress`, `loop.retry`, `loop.compaction.started`,
`loop.compaction.completed`, `tool.invoked`, `tool.result`, `desk.cell`,
`desk.egress` and `texture.agent_revision.progress`
(`event_durability.go:36-45`). The rule does not depend on whether batching
is on. Reasons:

- **Volume.** On staging, batchable appends were 94% of runtime events
  (`events_appended_batchable` 618 against `acknowledged` 39, evidence
  `8_counters.process_2_before_kill`) **[M]**.
- **Blast radius.** Run-state transitions, owner acts, email and browser
  acts, Texture revision events and System1 judgments stay where every
  reader and the lifecycle compare-and-write already expect them.
- **The lifecycle fold stays as it is.** It conditions on the event's
  canonical id being absent from `og_objects` (`lifecycle.go:3336`) and
  carries only acknowledged kinds.
- **The channel reader needs no index.** Its one caller reads an
  acknowledged kind (`event_durability.go:66`).
- **The class discipline already exists.** The sealer has five layers,
  including a forcing test over every `EventKind`
  (telemetry-batching design §3.4). F-c reuses the same predicate.

### 2.2 Routing: the op carries its table

`ProjectionOp` already has `Table string \`json:"table,omitempty"\``
(`projection_batch.go:68`). Nothing in production sets it. `Validate` reads
it only to refuse the two presence tables (`:194-197`). An op without it
serializes byte for byte as today **[V]**.

- **Writer (deploy 2).** With `store.event_table` on, the writer builds the
  op for a batched-kind event with `Table: "choir_events"` (W1, W2).
- **Projector (deploy 1).** In `projectOp` (`project.go:69-92`), case
  `ProjectionOpObject`: if `strings.TrimSpace(op.Table) == "choir_events"`,
  it calls `projectChoirEvent(ctx, tx, op.Body, pos, i)`. Otherwise it calls
  `projectObject` as today. Any other `Table` value keeps today's behaviour.
  The projector does **not** start refusing unknown `Table` values: a
  stricter `Validate` could refuse historical tapes on replay.
- **`projectChoirEvent` refuses** (it returns `ErrProjectionBatchInvalid`,
  which the dry run turns into `batchRefused` before the event is
  canonical, `appender.go:427-437`) any op that:
  - is not of object kind `choir.event`;
  - has a body that is not a `types.EventRecord` of a batched kind;
  - has a non-empty `computer_id`, `version_id` or `superseded_by`, or
    `tombstone` set;
  - has a zero `created_at` or `updated_at`;
  - has a `run_id`, `trajectory_id` or `owner_id` longer than 255
    characters. Today's `og_meta_*` VARCHAR(255) generated columns refuse
    the same rows (`dolt_store.go:186-190`, `:274`).

Why a stamp and not the alternatives (section 7 has the full comparison):

- A new op kind would make an old binary refuse the batch at `Validate`
  (`projection_batch.go:188-190`). After a rollback it could then neither
  finalize nor replay the tape.
- Routing on kind alone in the projector would make a new binary's replay
  of an old tape put historical events into `choir_events` while the live
  store still holds them in `og_objects`. Live and replay would then differ
  for ever.
- A projector that reads the setting row inside the transaction is
  deterministic too. But it couples projection to a row's state and costs a
  read per op. The stamp makes the decision visible on the tape itself.

### 2.3 DDL

Created empty by `bootstrap` (`store.go:765`) in `schemaDDL` (`:253`) as
`CREATE TABLE IF NOT EXISTS`, with the indexes inline. Nothing scales with
`og_objects` at boot. This is the og_key_index precedent
(`docs/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md`,
"Design").

```sql
CREATE TABLE IF NOT EXISTS choir_events (
  tape_seq      BIGINT UNSIGNED NOT NULL,  -- computer_event_index.sequence of the projection_batch_recorded event
  op_index      INT UNSIGNED    NOT NULL,  -- the op's index in that batch
  canonical_id  VARCHAR(255)    NOT NULL,  -- unchanged content-hash id (not unique, not indexed)
  object_kind   VARCHAR(128)    NOT NULL,  -- always 'choir.event'; kept so a row is a whole Object
  owner_id      VARCHAR(255)    NOT NULL,
  computer_id   VARCHAR(255)    NOT NULL DEFAULT '',
  version_id    VARCHAR(255)    NOT NULL DEFAULT '',
  content_hash  VARCHAR(128)    NOT NULL,
  body          LONGBLOB        NOT NULL,  -- byte-identical to what og_objects.body would hold
  metadata      LONGTEXT        NOT NULL,  -- byte-identical to og_objects.metadata
  created_at    DATETIME        NOT NULL,
  updated_at    DATETIME        NOT NULL,
  tombstone     BOOLEAN         NOT NULL DEFAULT FALSE,
  superseded_by VARCHAR(255)    NOT NULL DEFAULT '',
  run_id        VARCHAR(255)    NOT NULL DEFAULT '',  -- from metadata.run_id
  seq           BIGINT          NOT NULL DEFAULT 0,   -- from metadata.seq
  stream_seq    BIGINT          NOT NULL DEFAULT 0,   -- from metadata.stream_seq
  trajectory_id VARCHAR(255)    NOT NULL DEFAULT '',  -- from metadata.trajectory_id
  event_kind    VARCHAR(128)    NOT NULL,             -- from metadata.kind
  PRIMARY KEY (tape_seq, op_index),
  INDEX idx_choir_events_run_seq (run_id, seq),
  INDEX idx_choir_events_owner_stream (owner_id, stream_seq),
  INDEX idx_choir_events_trajectory_stream (trajectory_id, stream_seq)
);
```

Every column is a pure function of the op and its tape position. The
projector stamps no `time.Now()`, so no column needs to go into
`IgnoredContentColumns` (`replay_completeness.go:495-503`,
`projectionbase/witness.go:26`). The twelve `og_objects` columns are kept,
so a row reproduces the `objectgraph.Object` exactly: readers decode `body`
(`ogDecode`, `graph_store.go:328`), the vocabulary fence reads
`metadata`, and the diagnostic fingerprints can compare like with like. The
empty columns cost a few bytes per row.

The write is `INSERT ... ON DUPLICATE KEY UPDATE`, keyed on the tape
position. A re-projection of the same event (finalize after
`RecoverPrepared`, `appender.go:772`) is idempotent and never wedges the
computer.

### 2.4 The key: tape position, not `stream_seq`

`stream_seq` is not unique by construction:

- Lifecycle-folded events carry `seq` 0 and no `stream_seq` (evidence
  `duplicates_whole_chain.excluded_note`) **[M]**.
- Both counters are in-memory high-water marks seeded heuristically
  (section 1.3), and are documented as "a gap, never a duplicate", which is
  a property of the allocator and not of stored data.

The tape position is assigned by the one per-computer appender under its
mutex. `AppendNewPayload` holds `a.mu` from `appender.go:402` through
`appendLocked` and finalize (`:458`), and the head CAS rejects a gap
(`computer_events.go:183-185`). This is doctrine C8: one appender owns
semantic ordering. Both the live finalize and replay know the position,
because `finalizeBatch` reads `sequence` from `computer_event_index`
(`computer_events.go:144`). `projectBatch` gains a `pos uint64` parameter
and passes it through with the op index (`project.go:61-65`).

**Dry run.** `DryRunProjectionBatch` (`computer_events.go:97-118`) has no
sequence. It reads `computer_event_projection_heads.sequence + 1` inside its
own transaction. Because the appender holds `a.mu` across the dry run, pin,
prepare, CAS and finalize, that is the event's real position **[V]**. The
dry run's rows are then the same rows in the same tree, and the journal
deduplicates them, as it does today (record, "Correction") **[M]**. Failure
F6 tests that the provisional and final positions are equal.

### 2.5 Trees per insert

| Transaction | Trees today (record) | Trees after F-c, one batched op |
|---|---|---|
| Dry run (rolled back, mostly deduplicated) | 13 | 4 |
| Prepare (`computer_event_index`) | 4 | 4 (unchanged) |
| Finalize: heads, index row and status index, projection | 3 + 13 | 3 + 4 |
| **Per op in the projection** | **13** | **4** |

### 2.6 Journal estimate and its assumptions

The model is J_op ≈ T × c_tree + c_row + c_root.

- **T** is the number of trees.
- **c_tree** is the per-tree path. It is 6.7 to 7.6 KiB at about 10^5 rows,
  measured locally (record, "Local measurement") **[M]**, and about 8 to
  9 KiB at about 10^6 rows **[H]**.
- **c_row** is the row's own new bytes, about 2 to 3 KiB. On staging the
  projection payload averaged 2.97 KB per op in the first segment
  (1,954,086 B / 657 ops), base64 included **[M]**.
- **c_root** covers the table and root chunks, about 1 KiB **[H]**.

| | T | J_op, about 10^5 rows | J_op, about 10^6 rows |
|---|---|---|---|
| `og_objects` row (today) | 13 | about 90 KiB (staging fit: 86 KB **[M]**) | about 114 KiB |
| `choir_events` row | 4 | about 30 KiB | about 38 KiB |
| Saving per batched op | | about 60 KiB (about 66%) | about 76 KiB (about 67%) |

| Per telemetry record, staging-fit constants (a = 12.5 KB per tape event, b = 86 KB per op) | Today | After F-c (b ≈ 30 KB) |
|---|---|---|
| Batching off (k = 1) | 98.5 KB | about 43 KB (-57%) |
| Batching on (k = 5.2, as measured) | 88 KB | about 33 KB (-63%) |
| Batching on plus deploy 3 (one multi-row INSERT per batch into `choir_events`) | — | about 15 to 20 KB **[H]** |

Deploy 3's estimate takes a single-statement amortization of the sequential
primary key (about 7/k KiB per row) and about 0.6 of a leaf per row for each
secondary index, because rows of one run share leaves. The raw-table probe
measured -37% for six rows as one statement on 13 random-key trees (record)
**[M]**.

**Whole journal on the paper (derived)** **[H]**:

- Batched kinds are about 0.58 × 0.94 ≈ 0.55 of tape events (10-15 mix).
- They save about 65% of a row that is about 87% of their per-record cost
  (86 of 98.5 KB).
- So the journal falls by about 0.55 × 0.65 × 0.87 ≈ 31%.
- If run-memory ops (7 trees, record) cost less than `og_objects` rows, the
  share is larger, up to about 36 to 40%.
- So 1.6 GiB per 10 minutes becomes **about 1.0 to 1.1 GiB per 10
  minutes**, and the 1 GiB GC pass comes about every 15 minutes instead of
  about every 10.

Each pass still takes about 19 s (16.6 s per GiB plus 2 s, record), so F2
needs the record's stopgaps as well: smaller passes, a non-blocking GC, and
F1's fast 503. With F-a and deploy 3 the fall is about 50%. The next lever
by volume is `run_memory_entries`, 31% of events, with a random UUID key
and 6 more trees; the same pattern applies, in a design of its own.

**Assumptions, each settled by the staging proof (section 6.3).**

1. A secondary index of `choir_events` costs no more per insert than one of
   `og_objects`. Its keys are narrower, so it is probably cheaper.
2. A sequential primary key costs no more than a random one per statement.
   The rightmost leaf is partly filled, so it is probably cheaper.
3. Dolt stores `body` and `metadata` in or out of the row leaf at these
   sizes in a way that adds about c_row. Not read here; this is the ±2 KiB
   in c_row.
4. The batched share of the paper's ops today is close to 10-15's 55%. It
   is not known, and the record's reading 4 gives it.
5. The per-tape-event overhead is unchanged: 12.5 KB on staging, about
   45 KiB locally.

**History.** The 28 to 29 GiB live store is history: each 100-event
checkpoint pins about 100 distinct paths per random-key table (record,
cause 3). A sequential primary key and run- or owner-clustered secondaries
put the 100 events between checkpoints into a few leaves. The live store's
growth should fall more than the journal's **[H]**. This is not estimated;
the paper's `after_journal_mib - (after_avail_mib - avail_mib)` per GC pass
(record, reading 6) measures it.

## 3. Single state authority, replay, witness, restore

### 3.1 Authority

The tape is the only authority for an event, as it is for every other
projection. `choir_events` is a projection of `projection_batch_recorded`
ops, as `og_objects` and `run_memory_entries` are. The two tables never
disagree about one event, because one op lands in exactly one of them: the
stamp decides, on the tape. A reader that finds an event in neither table
is reading a lost-on-crash batched record, which batching already defines
(telemetry-batching design §5.2), or a bug.

### 3.2 Projection and replay

Replay is `ReconstructInto` / `ReconstructThroughTarget`
(`appender.go:1129-1168`) → `FinalizeReplayBatch` (`computer_events.go:87-89`)
→ `finalizeBatch` with the event's own `sequence` (`:144`) →
`projectBatchForReplay` (`:222`) → `projectOp`. That is the same route with
the same stamp and position as the live finalize. A fresh replay therefore
writes the same `choir_events` rows as the live store.

The deposit upcast (`vocab_upcast_deposit.go:556-680`) rewrites `op.Body`
in place and keeps `op.Table`, so upcast replays route the same way **[V]**.
Nothing is truncated: replay writes a fresh workspace (`store.go:536`) or
an installed base plus the tail.

### 3.3 Content witness and replay eligibility

**Does the witness hash change?** Yes, from the first stamped op on.

`DoltStateExtractor` lists tables with `SHOW TABLES`
(`dolt_state_extractor.go:263-286`). It hashes each table's columns and
rows, and builds `content_root` from the sorted per-table hashes (`:212-225`).
The new table enters automatically, and stamped events leave
`og_objects`' hash. Live and replay are always extracted by the same
binary, so they agree.

Three pieces need explicit wiring:

1. **Replay manifest** (`replay_eligibility.go:44-94`). It is default-deny:
   an unlisted table is "unknown" and the computer is replay-ineligible
   (`:177-181`, `:245-246`). Ineligibility blocks checkpoint and restore
   (`checkpoint_restore_bindings.go:60`, `:187`). Add `choir_events` as
   `event_projection`. Declared tables must also be present in both
   observation sets (`:209-219`), so item 2 needs a matching rule here.
2. **Old checkpoints.** `WitnessContentMatches`
   (`selfdevprotocol/rematerialize.go:66-92`) and `WitnessFromObservationSets`
   (`selfdevprotocol/restore.go:142-179`) require equal table sets and equal
   content roots. A checkpoint witnessed before deploy 1 has no
   `choir_events`. A deploy-1 binary that replays to that head would create
   the empty table, and every empty table adds one line to `content_root`.
   The restore would be refused.

   **Rule: an empty `choir_events` is not observed.** The extractor
   (`dolt_state_extractor.go:153-161`, beside `IgnoredTables`) gains
   `OmitWhenEmpty: {choir_events}`, checked with one
   `SELECT 1 FROM choir_events LIMIT 1`. The manifest gets one more class,
   `event_projection_optional`, which is not reported missing when it is
   absent from both sets (`replayEligibilityManifestVersion` 3 → 4).

   Then every head before the first stamped op has the same witness under
   old and new binaries. The same rule covers the base witness check
   (`projectionbase/verify.go:68-105`) and an old base installed under the
   new DDL (`restore_base.go:194`).
3. **Diagnostics.** Today `compareReplayObjectGraph` names the
   `og_objects` rows that differ. Deploy 1 adds the same per-row
   fingerprints for `choir_events`, so an inequivalence names its rows.

**Pre-existing, not introduced.** Checkpoint witnesses are already a
function of the binary's schema. `SHOW COLUMNS` enters the schema hash
(`dolt_state_extractor.go:290-333`), so any added column
(`store.go:814-830`) or table makes earlier checkpoints unverifiable under
the new binary. The closing heresy delta records this as discovered. The
rule above avoids it for this change only.

### 3.4 Restore to a prior event head

Restore stages a fresh store or an installed base. It reconstructs through
the target head (`agentcore/rematerialize.go:127-164`), checks the witness
against the checkpoint (`:183-200`), and then swaps the whole workspace
directory into place (`:227-265`, per the code audit). It uses no Dolt reset.
`choir_events` rows after the target head are therefore not in the staged
workspace. Rows are keyed by tape position, so "after the head" is exactly
`tape_seq > head.sequence`. `Reopen` (`store.go:946-980`) keeps the
in-memory `seq` and `stream_seq` marks, which are at least the restored
store's maxima, so the next numbers leave a gap and never repeat. This is
today's behaviour and F-c does not change it. A process restart seeds from
both tables (section 3.5).

### 3.5 Sequence seeds (the write path's reads)

- **Per-run.** `max(today's og_objects seed, SELECT MAX(seq) FROM
  choir_events WHERE run_id = ?)`. This is one index seek. The owner filter
  of today's seed (`store.go:2024-2026`) is dropped for the table half.
  Runless events share run_id '' across owners, so the maximum can only
  over-estimate, which is a gap and safe.
- **Store-wide `stream_seq`.** `max(today's newest-500 og_objects seed,
  the maximum stream_seq among the newest 500 choir_events rows by
  primary key)`. This has the same quality as today's newest-by-time
  heuristic: tape order is allocation order except for appends in flight.
- Both keep the sealer's `telemetrySeqGap` and `seqFloor` rules
  (`store.go:2027-2041`, `:2071-2073`).

### 3.6 Vocabulary, fingerprints, residue

- **Fence.** The full fence pass (`servingOGRoleFields`,
  `vocab_migrate_og.go:871`) also reads `choir_events` role leaves, with the
  same skip map. Stamped ops exist only on V2 binaries with the write
  validator (`PreflightWrite`, `dolt_store.go:613-626`), and replayed
  deposits are upcast before they are projected. The V1→V2 row migration
  (`planOGMigration`) therefore does not rewrite the table. The fence makes
  the "never V1" claim fail closed instead of trusting it (F11).
- **Fingerprints.** See section 3.3, item 3.
- **Residue import** snapshots legacy direct-written rows into the tape
  (`residue_import.go:51-122`). It must never read `choir_events`, whose
  rows are already tape-derived (F21).

## 4. Migration and rollback

The doctrine rule for this pass is: no boot step scans or rewrites all of
`og_objects`, and schema changes are online and rollback-safe
(`docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md:270`).

### 4.1 Three deploys

| Deploy | Contents | Effect on a computer |
|---|---|---|
| 1, "expand" (red, inert) | DDL (empty table at boot); `projectChoirEvent` and the route; `pos` through `projectBatch`; union readers (section 4.4) and both seeds; manifest class and entry; extractor `OmitWhenEmpty`; fence and fingerprints; the three caller-less methods deleted; plan-shape tests | **None observable**: no writer stamps, the table stays empty and is not observed, and the readers' second query returns nothing. Rollback to the previous build: an empty unknown table. See the rollback matrix |
| 2, "writer" (red) | the `store.event_table` setting in `KnownComputerSettings` (`computer_setting.go:55-61`), applied at once (the pattern of `computer_settings.go:105`) and at boot (the pattern of `telemetry_batching.go:39-56`); W1 and W2 stamp when it is on; a new counter `tape_ops_by_table` in `TelemetryStats` (`telemetry_stats.go:114-131`, `:182-205`) | off: byte-identical tape and store to deploy 1. On, per computer: batched kinds go to `choir_events` |
| 3, "F-e on the new table" (red) | in `projectBatch`, consecutive stamped ops of one batch become one multi-row upsert into `choir_events` | only useful with batching on; measured on its own |

Deploy 1 must be the stable release, with at least one staging day and one
paper boot, before deploy 2 is anywhere. A rollback of deploy 2 then lands
on a binary that projects and reads the table.

### 4.2 The selector

`store.event_table` is a per-computer owner setting. It is an object of
kind `choir.computer_setting`, written through the tape
(`computer_setting.go:76-96`) and set with
`POST /api/computer-settings {"key":"store.event_table","value":true}`, the
route the batching gate used. It decides only whether the writer stamps.
The projector never reads it. It goes on for one `warmness_class: test`
computer first (minted with `scripts/qa_test_computer.mjs`, as in the
gate), then the paper. Turning it off is the live rollback: new events go
back to `og_objects`, and the union reads keep every event visible. No
environment override is proposed; telemetry.batch has
`CHOIR_TELEMETRY_BATCH`. F-c does not depend on the sealer, so it can go on
the paper with batching still off
(docs/problems/host-planned-restart-skips-telemetry-drain-2026-10-22.md).

### 4.3 Existing `choir.event` rows: never touched

The rows already in `og_objects` stay there. Readers read the union. There
are three reasons.

- **Cost.** A move deletes from 13 trees and inserts into 4, about 120 KiB
  of journal per row by the record's constants **[H]**. The paper holds an
  unknown but large number of rows (section 9), so a move costs hundreds of
  GiB of journal and GC.
- **Replay.** A move that is not on the tape makes live (moved) and replay
  (not moved) differ. A move that is on the tape doubles the tape.
- **Doctrine.** The boot rule forbids it at boot, and an online move is a
  long red maintenance job for bytes that are already written.

The `og_objects` half of each union read costs what it costs today, and
its share stops growing. Removing old telemetry from either table is
retention. That is a separate owner decision: how long trace telemetry is
kept.

### 4.4 Union reads

Each method runs its `og_objects` query as today, plus one indexed query on
`choir_events`, then merges and cuts to the limit:

| Method | `choir_events` query | Merge |
|---|---|---|
| `ListEventsAfter` | `WHERE run_id=? AND seq>? ORDER BY seq LIMIT ?` | by seq; the cursor and evidence-link semantics are unchanged |
| `ListEventsByOwnerAfter` | `WHERE owner_id=? AND stream_seq>? ORDER BY stream_seq LIMIT ?` | by stream_seq |
| `ListEventsByOwner` | `WHERE owner_id=? ORDER BY stream_seq DESC LIMIT ?` | by timestamp, descending, as today |
| `ListEventsByTrajectory` | `WHERE trajectory_id=? AND owner_id=? ORDER BY stream_seq LIMIT ?` (the index serves the first term and the order) | by stream_seq |
| `ListEventsByChannel` | none: the post-cutover channel read stays on `og_objects` | — |

Reads go to `readDB` under the engine mutex, with labelled op names. This
needs a small exported `DoltStore` helper, because the mutex is unexported
(`dolt_store.go:65-69`). The upstream race the mutex was kept for is
recorded in the engine-concurrency design. `run_memory_entries` is read on
the write connection (`run_memory.go:95-130`), but event reads are more
frequent (WebSocket catch-up), so the read pool is the better default.

### 4.5 Rollback matrix

| From → to | Tape | Store reads | Replay and witness | Repair |
|---|---|---|---|---|
| Setting on → off | new events unstamped | union: nothing hidden | equivalent | none |
| Deploy 2 → deploy 1 | stamped ops stay; deploy 1 projects them | union | equivalent | none |
| Deploy 3 → deploy 2 | same op shape; one statement becomes N | same | same rows, so equivalent | none |
| Deploy 1 → the previous build, no op ever stamped | none | today's | an empty table **unknown** to the old manifest: replay-ineligible, so checkpoint and restore are refused; the computer keeps running (og_key_index precedent, finding 3) | forward, or `DROP TABLE choir_events` by hand (it is empty; deploy 1 recreates it) |
| Deploy 2 → the previous build, after stamped ops | **the old projector ignores `Table` and upserts stamped ops into `og_objects`**: new events after the rollback project normally | events stamped while deploy 2 ran are **invisible** to trace, WS catch-up, acceptance and costs. They are not lost: they are on the tape and in the table | replay-ineligible (unknown table). An old-binary replay would put the stamped events into `og_objects`, so it would not be equivalent to the live store anyway | forward to deploy 1 or later: everything is visible and equivalent again |

The old binary's behaviour on a stamped op is a characterization test on
today's code, written before any change (F5). Today's `projectObject`
reads only `op.Body` (`project.go:73-74`, `:215-256`), and `Validate` does
not refuse `Table` values other than the presence tables
(`projection_batch.go:194-197`) **[V]**. The schema epoch
(`storeschema/schema.go:17-26`) does not change: an additive table that
older binaries tolerate is "freely backward compatible". The remaining cost
of a rollback, an ineligible replay, is named above.

## 5. Failure modes, written before any code, each with its test

Harness: the store's tape harness (`internal/store/telemetry_harness_test.go`:
a real `Store`, the real appender, recording pin and CAS, `reopen`), unless
noted otherwise.

| # | Failure | Test that catches it |
|---|---|---|
| F1 | A seed reads `og_objects` only, so `seq` or `stream_seq` is handed out twice after the cutover or after a reopen | Harness: with the setting on, append batched and acknowledged events across several runs; `reopen` (a new `Store`, so the marks start empty, harness `:201-210`); append more; assert no duplicate `(run_id, owner, seq)` and no duplicate `stream_seq` across both tables. Repeat with batching on, with parallel appenders, and after `ReconstructThroughTarget` |
| F2 | A reader forgets the union, or merges or limits wrongly | Differential test over all four union methods: one event script into two stores, setting off against setting on, plus a store where the setting toggles in mid-run; identical results for every cursor and limit, including `limit+1` paging in `handleRunEvents`. Source guard: any function in `internal/store` that reads `ogKindEvent` must be on the union-aware list |
| F3 | Live and replay disagree: a wrong position, a wrong table, a `time.Now()` default | Harness: a mixed tape (toggled on, off, on; singles and sealed batches); `ReconstructInto` a fresh store; `replayDoltStateExtractor` of both is equivalent and `replayEligibility` is eligible |
| F4 | An old checkpoint becomes unverifiable because the empty table enters the witness | Build a witness on a workspace without `choir_events`; open it with the new DDL (empty table); the witness matches (`WitnessContentMatches`). After one stamped op, the table is observed and the old witness no longer matches, which is correct |
| F5 | An old binary refuses or wedges on a stamped op | **Characterization test on today's `main`, before the change**: `projectBatch` with `Table: "choir_events"` on an object op writes `og_objects` and `Validate` passes |
| F6 | The dry run's provisional position differs from the final one (extra chunks), or the dry run passes an op that finalize refuses (poison after the CAS) | Hook `DryRunProjectionBatch` (`hookedProjection`, harness `:154`) to record the position; assert it equals the finalized `tape_seq` for every stamped op. Table-driven refusals at the dry run, never on the tape: run_id of 300 characters, zero timestamps, a non-batched kind, a non-`choir.event` object, a non-empty `computer_id` |
| F7 | Re-finalizing (`RecoverPrepared` after a crash) fails on the primary key | Finalize the same prepared event twice, then crash between commit and return and run `RecoverPrepared`: no error, one row |
| F8 | The lifecycle fold starts writing to the table, so its absent-condition stops meaning anything | `projectLifecycleRun` (a `ReplaceLifecycleActivationRequest`) with a `loop.started` event, setting on: the event is in `og_objects`, a second identical fold returns `ErrConcurrentStateChange`, and a source guard checks that `lifecycle.go` never sets `Table` |
| F9 | The class leaks: a non-batched kind is stamped | A forcing test over `types.EventKinds()`: only `Batchable()` kinds are ever stamped; `recheckTelemetryOp` also asserts `Table ∈ {"", choir_events}`; the projector refusal from F6 |
| F10 | The channel read misses events | Pin `EventSystem1Judgment` as acknowledged, and pin `ListEventsByChannel`'s production callers to `system1_verb.go` (a source-scan test). A new caller fails the test and must add a channel index or a union |
| F11 | The fence is blind to the table | A raw-SQL V1 role leaf in a `choir_events` row makes the full fence pass fail closed |
| F12 | The manifest reports an empty or absent optional table as missing, or a non-empty one as fine when replay lacks it | `replay_eligibility_test.go`: absent from both sets is eligible; present in both with equal hashes is eligible; present in live only is ineligible |
| F13 | Restore keeps rows after the target head | Stamped events at heads h1 < h2; `ReconstructThroughTarget(h1)`: the maximum `tape_seq` is at most h1, and the witness equals one taken at h1 |
| F14 | Event-table reads race the projector on the shared engine | A `-race` test with finalize running concurrently with union reads (the pattern of `engine_mutex_live_test.go`) |
| F15 | Boot scans `og_objects`, or the DDL is slow on a large store | `Open` over many `og_objects` rows: `DOLT_HASHOF_TABLE('og_objects')` is unchanged and the open time is bounded (the pattern of `TestReopenWithManyRowsDoesNotTouchOgObjects`) |
| F16 | A reader query does not use its index | Plan-shape tests that `EXPLAIN` each `choir_events` query text, as the generated-column plan tests do (`dolt_store.go:1243`) |
| F17 | The saving is smaller than predicted | Local probe on the harness (journal per batched op, setting off against on, same seed, `doltJournalBytes`, `dolt_maintenance.go:192`). It is directional only; the staging proof decides |
| F18 | The setting is not applied at once, or is lost across a reboot | API test: `POST /api/computer-settings` takes effect without a restart; a crash boot reads it from the store (the gate's "read_at_boot") |
| F19 | A planned or crash restart adds a resumer | None is added. A source guard keeps the boot path's new statements to the DDL and the setting read (CLAUDE.md, "Restarts End Work") |
| F20 | The deleted methods have hidden callers | Compilation; the grep in section 1.2 |
| F21 | Residue import re-emits table rows as ops | The residue snapshot ignores `choir_events` (a test with rows present) |

## 6. Mutation class, ceremony, staging proof

### 6.1 Class: red

The change touches the guest store schema, the projector (tape to state),
replay and restore, the content witness and the replay manifest, the event
readers (trace, WS catch-up, acceptance evidence) and sequence allocation.

### 6.2 Ceremony

- **Conjecture delta.**
  - New: *a durable op's journal cost is paid per touched prolly tree, so
    moving batched telemetry from a 13-tree table to a 4-tree table with a
    sequential key cuts its per-op journal by about 60% and the paper's
    total by about a third*.
  - It is falsified if the staging fit gives b_ce/b_og > 0.5, or the
    paper's total falls by less than 20%.
  - It refines the record's ranking (F-c first) and makes F-e's value
    larger on the new table.
- **Protected surfaces.** Store schema (`schemaDDL`); `projectOp` and
  `projectBatch`; `DryRunProjectionBatch` and `finalizeBatch`; replay
  (`FinalizeReplayBatch`); the restore witness compare (`selfdevprotocol`);
  the replay airworthiness manifest; `DoltStateExtractor`; `AppendEvent`
  and its seeds; the four event read methods. Not touched: the tape event
  format and reducer, the payload format, the head CAS, auth, vmctl,
  gateway, deploy routing.
- **Admissible evidence class.**
  - Staging test-computer readings through product routes only:
    `/health` `effective_release.event_head.sequence`;
    `/api/runtime/observability?memory=detail` store bytes, the
    `telemetry_batching` counters and the new `tape_ops_by_table`;
    `GET .../self-development/replay-completeness`; the tape audit through
    corpusd's replay page and payload routes; WebSocket recording.
  - Local harness numbers are directional only.
  - Not admissible: local claims about the paper's journal; anything read
    over SSH; SQL against a deployed store.
- **Rollback path.** The setting off (live); then git revert of deploy 2
  (safe to deploy 1); then git revert of deploy 1, at the costs in 4.5.
- **Heresy delta.** See the closing section.

### 6.3 Staging proof plan

The proof runs on one fresh test computer, as in the batching gate. These
are the same readings the record and the gate used.

1. **Same build for both arms.** The gate's flag-off and flag-on intervals
   were on different builds (evidence `environment.between_builds`). Here
   both arms run on the deploy-2 build.
2. **Baseline arm.** Setting off, batching off. Run the gate's scripted
   workload (Texture create and revise, and a research turn) for at least
   10 minutes and at least 1,000 tape ops. Take a snapshot at each segment
   boundary of three readings: store bytes, the tape head, and the
   `telemetry_batching` counters (`tape_events`, `tape_ops`,
   `events_appended_by_kind`, `tape_ops_by_table`).
3. **F-c arm.** Set `store.event_table=true` (record the tape seq of the
   setting write). Run the same workload and the same snapshots.
4. **Fit.** delta_bytes = a × tape_events + b_og × og_ops + b_ce × ce_ops
   over all segments of both arms (the gate's segment method).
   - Prediction: b_og ≈ 86 KB, b_ce ≈ 30 KB, so b_ce/b_og ≈ 0.35.
   - Pass: b_ce/b_og ≤ 0.5. Report the bytes per telemetry record for both
     arms.
5. **Correctness on the same computer.**
   - Replay completeness is `equivalent` and `eligible` at a head after the
     cutover, with stamped ops in the replayed tail.
   - The whole-chain audit finds no duplicate `seq` or `stream_seq`, and the
     gate's step-3 ordering rule holds across both tables.
   - `GET /api/runs/{id}/events` for a run that spans the setting write
     returns every event the recorded WebSocket published.
   - `POST /api/run-acceptances/synthesize` on a trajectory with stamped
     `tool.result` events cites them.
   - `/api/costs` and the texture diagnosis return events from both tables.
6. **Crash drill.** An owner lifecycle refresh (SIGKILL), as in gate
   step 4. After the boot: no duplicate numbers, runs closed "interrupted
   by a restart", setting still on.
7. **Live rollback drill.** Setting off: new batched events land in
   `og_objects`, and readers still return the whole stream.
8. **Composition.** Batching on, setting on: the fit gives the k = 5 cost
   per telemetry record. After deploy 3, the same with multi-row inserts.
9. **Paper.** After steps 1 to 8 pass and the panel agrees: set the paper's
   setting, then take the record's readings 1 to 6 before and after, two
   samples 600 s apart inside one GC interval.

The final artifact is a JSON evidence file in the shape of
`docs/evidence/telemetry-batching-phase1-staging-2026-10-22.json`.

## 7. Alternatives considered

| Alternative | What it would buy | Why F-c wins |
|---|---|---|
| **Fewer indexes on `og_objects`, for all kinds** | Every op of every kind pays fewer trees | The 12 indexes exist because their absence was the CPU problem: JSON scans at about 1.4 s per call (`dolt_store.go:212-214`; guest-cpu-is-unindexed-store-scans-2026-10-14). Dolt has no partial indexes, so a record-scoped index costs every kind (`:271-274`, `:318-328`). Dropping one saves 1 of 13 trees per op (about 7%) and regresses its reader. It is also DDL on the largest table |
| **F-e alone: multi-row upserts in `og_objects`** | -37% per row at k = 6 (raw probe, record) | Useful only with batching on, which cannot go on the paper before the planned-restart drain fix. It still pays 13 trees per statement. It composes with F-c and is stronger there (deploy 3) |
| **A separate Dolt database for telemetry** | GC of telemetry would no longer hold the main store's write connection | It writes the same bytes. Finalize (heads, index and projection) is one transaction on one connection today (`computer_events.go:133-235`); a second database splits it, so a crash between the two leaves a head that says "finalized" over a missing row. The witness, restore and base code assume one database, `texture` (`replay_completeness.go:492`, `rematerialize` workspace swap, `projectionbase`): a second workspace means a second extractor, swap and base. It can be revisited after F-c if GC isolation is still wanted |
| **No telemetry in Dolt at all**: serve trace reads from the tape (the corpusd payload pages) or a local append-only log | Removes about 90 KiB per op altogether | The trace and acceptance would fate-share with corpusd (standing question 7). A local log is new state with no replay, witness or restore story, and acceptance cites events by id and `stream_seq` as evidence. It is the right long-term question (retention, telemetry-batching design §8), but it is not a first-pass change. F-c keeps every existing invariant (tape → Dolt projection → witness) and is reversible |
| **Move every `choir.event` (K1)** | About 6% more volume | It moves the lifecycle fold and its condition, the acknowledged evidence kinds and the System1 channel read. It needs a fifth tree, `(channel_id, …)`, or a condition on the new table. Open question Q1 |
| **A new op kind (`event_recorded`) instead of the stamp** | Explicit in the op kind | Old binaries refuse unknown op kinds at `Validate` (`projection_batch.go:188-190`): a rollback could neither finalize nor replay |
| **Route by kind in the projector, with no stamp** | No writer change | A new binary's replay of an old tape would put historical events into the new table while the live store holds them in `og_objects`. They are never equivalent, unless every row is moved |
| **A bounded background move of old rows** | Single-table reads | About 120 KiB of journal per moved row, and it is not on the tape (section 4.3) |
| **Primary key `stream_seq`, or `(owner_id, stream_seq)`** (3 trees) | One index fewer | `stream_seq` is not unique by construction (section 2.4). A key built from caller data can collide on replay, and a collision is either poison or a silent overwrite |

## 8. Open questions for the owner, each with a recommended default

| # | Question | Default |
|---|---|---|
| Q1 | Move only the ten batched kinds (K2), or every `choir.event` (K1)? | **K2.** About 94% of the volume, with no change to the lifecycle fold, the evidence kinds or the channel read |
| Q2 | Keep the `(trajectory_id, stream_seq)` index (4 trees), or serve acceptance synthesis by fanning out over the trajectory's runs (3 trees, about 7 to 9 KiB less per op)? | **Keep it** in deploy 2. Trajectory derivation has fallbacks (`runtime.go:4611-4619`, `traceTrajectoryIDForRun`), so a fan-out is not provably equal. Revisit with the staging numbers |
| Q3 | Name: a new `choir_events`, or reuse the legacy empty `events` table (created at boot with two indexes, `store.go:254-268`, `:796-803`, and declared `empty_until_supported`, `replay_eligibility.go:50`)? | **New name.** Reusing it changes an existing manifest class and inherits its columns and indexes. Retiring the legacy table is a separate small change |
| Q4 | Fold the multi-row insert into deploy 2, or keep it as deploy 3? | **Deploy 3**, so each saving is measured on its own |
| Q5 | Ship deploy 1 as its own release before deploy 2? | **Yes.** At least one staging day and one paper boot first |
| Q6 | Existing rows in `og_objects`: never touched, or moved or expired later? | **Never touched by F-c.** Retention, meaning how long trace telemetry is kept, is a separate owner decision with its own design |
| Q7 | Enable on the paper before the batching drain fix? | **Yes**, after the test computer passes. F-c works with batching off |
| Q8 | Event-table reads on `readDB` under the engine mutex, or on the write connection as `run_memory_entries` does? | **`readDB` under the engine mutex**, with labelled ops |
| Q9 | Accept the narrowing of `ListEventsByChannel` to acknowledged kinds for post-cutover rows (pinned by F10), or add a channel index (5 trees)? | **Accept, pinned.** Its only caller reads an acknowledged kind |
| Q10 | Delete the three caller-less methods in deploy 1? | **Yes** (standing question 4) |

## 9. Not determined from code

- The paper's current kind mix: the batched share of its ops, and whether
  `telemetry.batch` is on there. The record's reading 4 gives both.
- The number of `choir.event` rows in the paper's `og_objects`, and its
  tree depth. They decide c_tree and the union read's `og_objects` cost.
- Whether Dolt keeps `body` and `metadata` in the row leaf or stores them
  out of band at 2 to 4 KB. This is the ±2 KiB in c_row.
- The per-statement journal cost of a sequential-key tree against a
  random-key one. Assumed no higher; the local probe (F17) measures it.
- Whether Dolt answers `ORDER BY stream_seq DESC LIMIT n` on
  `(owner_id, stream_seq)` and `MAX(seq)` on `(run_id, seq)` as seeks.
  F16's plan tests decide.
- Whether any checkpoint taken before deploy 1 will ever be restored on
  staging or the paper. The section 3.3 rule makes it possible, but the
  need is not known.
- Whether anything outside this repo reads a guest store's `og_objects`
  for `choir.event`. Nothing was found in code.

## Heresy delta

- **Discovered**
  - Checkpoint witnesses are a function of the binary's schema. Any added
    table or column makes earlier checkpoints unverifiable under the new
    binary, because `WitnessContentMatches` requires equal table sets and
    `SHOW COLUMNS` enters the schema hash.
  - The `seq` and `stream_seq` seeds are heuristics over `og_objects`
    ordering.
  - Three event methods have no production caller:
    `ListEventsByTrajectoryAfter`, `ListEventsOG` and `ListEventsByOwnerOG`.
  - `compactRunEventLedger`'s event read is unreachable, and
    `VerifyTextureWorkflow` has no caller.
  - The legacy `events` table is still created at every boot, with two
    indexes and no readers.
  - `ListEventsByChannel` has a single caller, System1.
- **Introduced (by the design, once built)**
  - A stamped op's projection depends on whether the binary honours
    `ProjectionOp.Table`: an old binary puts it in `og_objects`.
  - Event reads are a union of two tables until retention.
  - `ListEventsByChannel` covers acknowledged kinds only for post-cutover
    rows (pinned).
- **Repaired:** none by this document. Once landed: the 13-tree write
  amplification of batched telemetry.
