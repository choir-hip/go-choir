# Replay difference classification — 2026-08-12

Durable classification of the 26 DoltStateExtractor differences from
`docs/evidence/choir-supervised-self-development-replay-completeness-2026-08-12.json`.
This is a Define receipt, not a repair. It does not license checkpoint, restore,
or effects.

- Computer: `computer-03335285269bdba4f94377e56879f9e6`
- Probe digest: `a6e61857598fb1761ed58e5b27c527c12ebaf19e850db4ceae9743d76a0a12f0`
- Heads: `live_head` null, `replay_head` null
- Owner direction: pre-launch; no backwards compatibility
- Panel: convergent 2026-08-12, six usable `APPROVE WITH CONDITIONS` (codex excluded: runner still passes removed `--autoputer`; deepseek timed out). Session transcripts are gitignored under `.agentic-consensus/replay-resolve-20260812/`.

## Root classes

The 26 observations are not 26 independent ledgers.

1. **Aggregate** — `dolt:texture:content_root` hashes sorted table *content* hashes only (`internal/computerversion/dolt_state_extractor.go`). Extra empty retired tables still move it. Schema-only drift on `computer_event_index` does not. It converges only as a consequence.
2. **Retired / schema residue** — five tables absent from current DDL plus one extra live column on an otherwise live event-projection table. Current `CREATE TABLE IF NOT EXISTS` cannot remove them.
3. **Unsupported direct-SQL state** — fourteen current tables with nonempty live hashes and empty replay hashes. `ReconstructInto` does not write them. Manifest class: `empty_until_supported`.

No row is pinned-receipt state today. No row is event-derived except the event-projection tables, which were not among the 26 content mismatches (`computer_event_index` table hashes were both empty).

## Per-difference table

| # | Key | Reason | Class | Writer / fact | Resolution |
| --- | --- | --- | --- | --- | --- |
| 1 | `dolt:texture:content_root` | values differ | aggregate | extractor content-root | consequence of 2–26 |
| 2 | `schema:app_adoptions` | missing from replay | `retired_absent` | no current CREATE/writer | absent after workspace replace |
| 3 | `schema:app_change_packages` | missing from replay | `retired_absent` | no current CREATE/writer | absent after workspace replace |
| 4 | `schema:candidate_package_intakes` | missing from replay | `retired_absent` | no current CREATE/writer | absent after workspace replace |
| 5 | `schema:computer_event_index` | values differ | `event_projection` + schema drift | live-only column `supervision_transaction_json`; current DDL `internal/store/store.go` omits it; table itself is not retired | exact current schema on a fresh workspace |
| 6 | `schema:computer_source_lineages` | missing from replay | `retired_absent` | no current CREATE/writer | absent after workspace replace |
| 7 | `schema:computer_supervision_commands` | missing from replay | `retired_absent` | no current CREATE/writer | absent after workspace replace |
| 8 | `table:agent_evidence` | nonempty vs empty | `empty_until_supported` | `internal/store/texture.go` INSERT | discard; fail-closed while nonempty |
| 9 | `table:app_adoptions` | missing from replay; live empty hash | `retired_absent` | paired with #2 | absent after workspace replace |
| 10 | `table:app_change_packages` | missing from replay; live empty hash | `retired_absent` | paired with #3 | absent after workspace replace |
| 11 | `table:candidate_package_intakes` | missing from replay; live empty hash | `retired_absent` | paired with #4 | absent after workspace replace |
| 12 | `table:computer_source_lineages` | missing from replay; live empty hash | `retired_absent` | paired with #6 | absent after workspace replace |
| 13 | `table:computer_supervision_commands` | missing from replay; live empty hash | `retired_absent` | paired with #7 | absent after workspace replace |
| 14 | `table:desktop_app_instances` | nonempty vs empty | `empty_until_supported` | `internal/store/desktop_live.go` | discard; fail-closed while nonempty |
| 15 | `table:desktop_sessions` | nonempty vs empty | `empty_until_supported` | `internal/store/desktop_live.go` | discard; fail-closed while nonempty |
| 16 | `table:desktop_window_placements` | nonempty vs empty | `empty_until_supported` | `internal/store/desktop_live.go` | discard; fail-closed while nonempty |
| 17 | `table:desktop_workspaces` | nonempty vs empty | `empty_until_supported` | `desktop_live.go`, `store.go` | discard; fail-closed while nonempty |
| 18 | `table:media_progress` | nonempty vs empty | `empty_until_supported` | `internal/store/media.go` | discard; fail-closed while nonempty |
| 19 | `table:media_recents` | nonempty vs empty | `empty_until_supported` | `internal/store/media.go` | discard; fail-closed while nonempty |
| 20 | `table:og_edges` | nonempty vs empty | `empty_until_supported` | VM-local `internal/objectgraph/dolt_store.go`; platform OG is out of scope | discard; fail-closed while nonempty |
| 21 | `table:og_objects` | nonempty vs empty | `empty_until_supported` | VM-local `internal/objectgraph/dolt_store.go` | discard; fail-closed while nonempty |
| 22 | `table:run_memory_entries` | nonempty vs empty | `empty_until_supported` | `internal/store/run_memory.go` | discard; fail-closed while nonempty |
| 23 | `table:texture_agent_mutations` | nonempty vs empty | `empty_until_supported` | `internal/store/texture.go` INSERT IGNORE | discard; fail-closed while nonempty |
| 24 | `table:texture_controller_checkpoints` | nonempty vs empty | `empty_until_supported` | `internal/store/texture.go` | discard; fail-closed while nonempty |
| 25 | `table:texture_document_aliases` | nonempty vs empty | `empty_until_supported` | `internal/store/texture.go` | discard; fail-closed while nonempty |
| 26 | `table:user_preferences` | nonempty vs empty | `empty_until_supported` | `SaveUserPreference` uses `time.Now()`; later first vertical reducer, not this slice | discard; fail-closed while nonempty |

Empty table hash: `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.

## Adjudication

- **This slice's "resolved"** means diagnostic equivalence after an owner-scoped product-path VM-local workspace replacement onto current DDL: zero extractor differences at the post-cutover snapshot. The retained computer has a null event head; a cleaned null-head computer stays **ineligible**. Do not append `GenesisImported` or publish a checkpoint to flip that bit.
- **Replay eligibility** requires a later non-null canonical chain, matching heads, retired objects absent, and every `empty_until_supported` table empty. Not this cutover's success criterion on the retained computer.
- **Event-derived restore readiness** remains fail-closed. No vertical reducers in this slice. Direct writes after cutover correctly make eligibility false until a later reducer replaces that writer.
- **Rejected:** hash filters; adding `supervision_transaction_json` back to current DDL; in-place `ALTER`/`DROP` as a migration; treating lifecycle credential restart as cutover; reusing `POST /api/computers/{id}/self-development/genesis` (it appends `GenesisImported` and calls `PublishCheckpoint`; proxy currently disables that route).
- **Loss boundary:** the entire previous VM-local embedded Dolt workspace. Five retired tables are already empty (schema-only loss). The fourteen unsupported tables' live rows are discarded and not migrated. Platform store, cycle state, CAS tape, route, and guest credentials stay out.
- **Additive-only exception:** `CREATE TABLE IF NOT EXISTS` remains the standing rule. One pre-launch exception: replace the workspace rather than migrate it. That is a cutover, not a migration framework.

## Next implementation slice

Add an owner-scoped product-path workspace-replace verb (lifecycle, not the existing genesis route): quiesce writers, quarantine the old workspace as inert evidence, `OpenFresh` current DDL, reopen, append no event, publish no checkpoint. Then re-run the read-only replay probe. Expect `equivalent`, still-null heads, `eligible=false` because non-nil heads are required. Checkpoint, restore, and effects remain refused.
