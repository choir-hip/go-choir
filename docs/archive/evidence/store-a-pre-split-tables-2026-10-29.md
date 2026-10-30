# Store A pre-split tables: writers, readers and last writes (2026-10-29)

Read-only census. Class: green (docs). Host: Node B, Store A = `platform` on
127.0.0.1:13306, Store B = `corpus` on :13307. Queries ran as the app user
`choir` (password from `/var/lib/go-choir/secrets/dolt-app-password`, never
printed), `SELECT` only, client timeout 30 s, `LIMIT`ed. Table list from
`/var/lib/go-choir/dolt-reset-20261029T220557Z/platform.old.counts`.
[V] verified, [I] inferred.

## Method and wiring

- [V] Prod DSNs: `nix/node-b.nix:1270,1380` give corpusd and checkpointd
  `CORPUSD_DOLT_DSN` = Store A (:13306); `unit-env/corpus-dsn.env` on the host
  gives `CORPUSD_CORPUS_DOLT_DSN` = Store B (:13307/corpus) (credential
  redacted). The only `platform.OpenStore*` callers are `cmd/corpusd/main.go:43`
  and `cmd/checkpointd/main.go:59`; both pass both DSNs.
- [V] `internal/platform/store.go:375` `Store.corpus()` returns the Store B
  pool whenever it exists; `corpusSchemaDDL` (incl. `og_objects`, `og_edges`)
  runs on `s.corpus()` (`store.go:418`), `ObjectGraphStore` uses `corpus()`
  (`objectgraph_store.go:196-270`), `publication_opinion.go:131` and
  `publication_related.go:522,545` too. So on Node B no statement in
  `internal/platform` touches Store A's og tables.
- [V] `internal/objectgraph/dolt_store.go` (real INSERT/SELECT on og_*) is the
  VM-local embedded workspace (`internal/store/store.go:967`), not a server.
- [V] No view and no FK touches any table below. The only FKs on Store A
  (`information_schema.key_column_usage`) are the 8 `route_*_fk` constraints
  among `computer_version_*` tables.
- [V] Timestamps: the 8 big tables store varchar ISO times with no index
  except `og_objects.updated_at`. I used: `ORDER BY updated_at DESC LIMIT 1`
  (og_objects), `MAX()` on the 25K-170K row tables, and filtered
  `LIMIT 1` probes (`created_at >= '2026-09-19T04'`) on items and
  ingestion_events (6-7 s, empty), plus the newest cycle's id via the
  `(cycle_id, created_at)` index. Host load stayed 2-5.

## The eight large tables (all on Store A)

| table | rows | newest write [V] | Go code on main |
|---|---|---|---|
| og_objects | 6.09M | 2026-09-19 03:52:52 (updated_at) | DDL on Store B pool; SQL in platform/objectgraph all via `corpus()` = Store B; embedded store is VM-local |
| og_edges | 3.05M | 2026-09-19 03:53:02 | same |
| ingestion_events | 3.00M | 2026-09-19 03:52:52 (none >= 09-19T04) | none (only a `processor_requests` JSON field in `internal/sourceapi/types.go:25`) |
| fetches | 2.38M | 2026-09-19 03:53:39 | none |
| items | 1.72M | none >= 09-19T04 (sampled 09-19 01:41) | none |
| cycle_events | 170K | 2026-09-19 03:53:39 | none |
| processor_requests | 152K | 2026-09-19 03:53:15 | none |
| cycles | 25K | 2026-09-19 03:53:39 | none |

[V] `grep` over `internal/ cmd/ scripts/ nix/ .github/` finds no SQL for
ingestion_events, fetches, items, cycle_events, processor_requests, cycles.
They were written by the retired sourcecycled process (a repo `cmd/` has no
sourcecycled; its DSN var `SOURCECYCLED_DOLT_DSN` still sits in
`corpus-dsn.env` and points at Store B). Store B (`corpus`) has none of these
tables and holds og_objects 662K (newest 2026-10-29 19:54:59), og_edges 499K,
og_lookup_keys, proj_capture_*.

Verdicts: og_objects, og_edges: dead-but-referenced (the DDL still declares
them in `corpusSchemaDDL`, and the fallback path `corpus()` returns Store A's
pool if `CORPUSD_CORPUS_DOLT_DSN` is unset, as in the documented rollback;
no production reader/writer on Node B). The six wire tables: dead. All last
written 2026-09-19 (40 days).

## Other tables in the count file

Dead on Store A (no Go reference to the name anywhere on main, newest write
<= 2026-06-30, not 14 days): artifact_blobs 640, artifact_manifests 640,
citation_edges 641, consent_records 633, platform_subjects 76,
provenance_activities/agents/edges/entities (640/640/640/1280),
publications 633, publication_policies 619, publication_proposals 633,
publication_versions 633, publication_source_entities 3647,
public_routes 926, retrieval_manifests/sources/spans 633 each,
review_records 633, rollback_refs 926 (the string appears only as a type in
`internal/types/acceptance.go`). Newest 2026-06-30 15:20:09; the
publication_source_entities and publication_transclusions at 06-27.
Same names exist EMPTY on Store B [V] (0 rows), so they are old leftovers
from before the Cut 2 DDL removal (`store.go:62-69`).

Dead, DDL-only on main, rows old on Store A:
- publication_transclusions 3647 (06-27), publication_version_proposals 7
  (06-07), proposal_delivery_records 7 (06-07): `CREATE TABLE IF NOT EXISTS`
  in `corpusSchemaDDL`, no INSERT/SELECT. DDL runs on Store B.
- platform_texture_documents 185 / platform_texture_revisions 545 (newest
  2026-07-12): DDL plus column-migration code on `corpus()` only
  (`ensurePlatformTextureRevisionColumn`); no row access.
- issues 0, reconciler_requests 0 (type in `sourceapi/types.go`),
  platform_vtext_documents 0, platform_vtext_revisions 0, consensus_decisions
  0, consensus_guest_core_keys 0, platform_update_offers 0: empty; the
  consensus_* and platform_update_offers have DDL, the others no code.
- sources 211: newest updated_at 2026-09-19T03:54:12Z, no Go code: dead
  (sourcecycled family).

LIVE on Store A (control, not pre-split): computer_event_*, lifecycle,
checkpoints, projection_*, route/version tables, control_key_history,
owner_act_receipts, self_development_*, projection_worker_status, and the
key-escrow tables. computer_key_escrow_transparency newest 2026-10-29
22:49:39 (written during this census), computer_key_escrows 2026-10-26.
computer_key_unwrap_requests/approvals 2026-08-26 (write path live in
`internal/platform/key_escrow.go`, via `s.db` = Store A).

## Finding to carry into any Store A rebuild [V]

`computer_key_escrows`, `computer_key_escrow_transparency`,
`computer_key_unwrap_requests`, `computer_key_unwrap_approvals` are created
by `corpusSchemaDDL` (`store.go:181-210`), which bootstraps on the Store B
pool, but `internal/platform/key_escrow.go` reads and writes them through
`s.db` (Store A). On Node B Store A has them only because they pre-existed;
Store B holds empty copies. A fresh Store A (history-free rebuild from a dump
of live tables) must carry these four tables or key escrow breaks; the
bootstrap would not recreate them on Store A. They are control data, not
pre-split leftovers; the placement mismatch is a separate bug [I].
The rebuild keep-set must include computer_key_*.

## Conclusion

Droppable from Store A without a code reader on Node B: the six wire tables
(~7.4M rows), og_objects and og_edges (9.1M rows, after confirming the
rollback runbook no longer needs the shared-pool fallback), `sources`, and
the ~20 old publication/provenance/retrieval/artifact/consent tables.
Dropping frees nothing until history is rewritten (see
`docs/problems/store-a-dead-wire-data-and-unbounded-history-2026-10-09.md`).
