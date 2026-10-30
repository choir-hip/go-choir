# Trace bundle format (Cut 0)

The contract between `choir trace export` (writer), `scripts/reeng_tracer.mjs`
(driver) and `scripts/reeng_invariants.mjs` (checker). Station C0 of
`docs/archive/definitions/choir-reeng-metamission-2026-10-12.md`; artifact (1) of
`docs/archive/definitions/choir-reeng-cut0-eyes-2026-10-12.md`.

A bundle is one JSON file describing one trajectory on one computer, read only
through product API routes (`https://choir.news/api/...`) under the owner's
session or a scoped API key. No SSH, no `/internal/*`. The guest store stays
the authority; a bundle is a projection (standing question 5). Bundles are
the owner's data and are written unredacted, outside the repo.

## Top level

```json
{
  "schema": "choir.trace_bundle.v1",
  "exported_at": "RFC3339",
  "base_url": "https://choir.news",
  "computer_id": "computer-...",
  "trajectory_id": "...",
  "build": { "commit": "...", "deployed_commit": "...", "built_at": "..." },
  "drill": null,
  "layers": { "<layer>": { ... } }
}
```

`drill` is filled by the tracer, not the exporter (see below).

## Layers

Every layer has the same envelope:

```json
{ "status": "present | empty | unserved", "sources": ["GET /api/..."], "note": "", "items": [] }
```

- `present`: the route answered and returned at least one item.
- `empty`: the route answered with nothing.
- `unserved`: no product route serves this layer today, or the route
  failed. `note` says which. A checker rule that needs an unserved layer
  reports `unknown`, never `pass`.

| layer | items | source |
|---|---|---|
| `trajectory` | one `LifecycleSnapshot` (trajectory, work_items, agents, co_super_assignments, head, updates) | `GET /api/trajectories/{id}` |
| `lifecycle_events` | `LifecycleEvent` rows, all pages, in `reducer_seq` order | `GET /api/trajectories/{id}/events?after=&limit=1000` |
| `runs` | `RunRecord` for every run of the trajectory, including Management lifecycle-control runs whose `metadata.assignment_trajectory_id` names it and every run named by an event, packet or assignment | `GET /api/runs?trajectory_id=`, `GET /api/runs/{id}` |
| `run_events` | `EventRecord` rows of every run in `runs`, each with `loop_id`, ordered by (`loop_id`, `seq`) | `GET /api/runs/{id}/events?after=&limit=1000` (new, owner-scoped) |
| `cells` | one item per `desk.cell` run event (below) | derived from `run_events` |
| `egress` | one item per egress call: the rows in the `egress` array of each `desk.cell` run event, and each standalone `desk.egress` run event (below). A folded row also carries `cell_event_id`, and an `event_id` of the cell event plus `#egress.<n>` | derived from `run_events` |
| `deliveries` | the snapshot's `updates` (delivery packets) and `work_items` | derived from `trajectory` |
| `records` | every record of the Texture and research inboxes of the trajectory's documents (Cut 1 L3a: their conversation is on records), each with `inbox`, `inbox_seq`, `sender_inbox`, `conversation_id`, `round`, `final` | `GET /api/inboxes/{inbox}/records?after=&limit=1000` |
| `inboxes` | one item per inbox read: `{inbox, head, cursor, open_rounds}` (cursor is the inbox's own desk's) | the same route |
| `turn_commits` | lifecycle events of kind `texture_turn_committed` | derived from `lifecycle_events` |
| `revisions` | every revision of every Texture document the trajectory touched (head doc, docs named by runs or events) | `GET /api/texture/documents/{id}/revisions` |
| `publications` | `PublicationBundle` for each published route of those documents | `GET /api/platform/publications/resolve?route=` (`unserved` with a note if no route can be found from the product API) |
| `search_arrivals` | async search arrivals | `unserved` until Cut 2 |
| `research_children` | child activations of `choir.Research` | `unserved` (children run without a run record and emit no events) |
| `captures` | the computer's captures created from the earliest run or event time (less 2 minutes, for clock skew) to the export, plus every capture a record, event or the snapshot names by `capture_id` (including inside a report's JSON-string packet). Each item: `capture_id`, `url`, `title`, `in_window`, `media_type`, `extractor`, `doc_hash`, `block_count`, `raw_bytes`, `raw_present`, and `blocks[]` of `{id (cid), kind, page, text, normtext}`. Bounded: 200,000 characters of block text per capture (`blocks_truncated` and a `note` say where it stopped), 20 million per layer, 500 listed captures (`window_complete` false and a note when the list was full) — Cut 2 amendment A5 | `GET /api/captures?since=&until=&id=` |
| `obligations` | one `GET /api/runtime/obligations` body at export time | `GET /api/runtime/obligations` |
| `boot` | one item: `{computer_id, build, recent_log, restart: {kind, reason}}` from observability and obligations | `GET /api/runtime/observability` |

A derived layer takes `unserved` if its parent layer is `unserved`.

### As built (`choir trace export`, cmd/choir/trace.go)

- `GET /api/runs/{id}/events` answers `{events, next_after}`; `next_after`
  is 0 on the last page. `limit` defaults to 200 and is clamped to 1000. A
  run of another owner answers 404 exactly like a missing run.
- `runs`: ids come from `GET /api/runs?trajectory_id=&limit=500` plus every
  string under a `run_id` / `loop_id` / `*_run_id` / `*_loop_id` key (or the
  `*_ids` array forms; `ci_run_id` and `deploy_run_id` excluded) anywhere in
  the snapshot and the lifecycle events. Each item is the full `RunRecord`
  from `GET /api/runs/{id}` (the list answers a projection). An id that
  answers 404 is named in `note`, not a failure.
- `cells` / `egress` items are the event payload with `event_id`, `loop_id`,
  `seq` and `ts` added where the payload lacks them. `deliveries` items are
  `{kind: "update" | "work_item", item}`. Derived layers list
  `derived from <parent> ...` in `sources`.
- `revisions`: documents are every string under a `doc_id` / `document_id` /
  `*_doc_id` key (or `*_doc_ids`) in the snapshot, lifecycle events and run
  records; run-event payloads are not searched. A 404 document is noted.
- `publications`: routes come from revision metadata (`corpusd_route_path`,
  `corpusd_publication_ref.route_path`), from lifecycle events with reason
  `platform publication accepted` (their `artifact_refs`), and from
  `/api/universal-wire/stories` entries whose `story_texture_doc_id` is one
  of the trajectory's documents. The first two are written when a
  publication is accepted, so the layer is `unserved` only when `revisions`
  is; a failing stories read is named in `note` without changing status.
  No route found is `empty`.
- Any failing route (non-2xx other than the 404s above, or a cursor that
  does not advance) marks its layer `unserved`, with the HTTP status in
  `note`. Items read before the failure stay in the layer.

### Records in I3 (Cut 1 L3a)

A record committed after the boot is a dispatch node caused by its sender's
run active at that moment (a system closure record is the restart's own
fate). A run whose `metadata.request_source` is `inbox_record` is caused by
the newest record of its inbox committed before it; a pre-boot record with
no owner input since the boot makes it a violation.

## Run event kinds added in Cut 0

Both are persisted on the acting desk run (not published on the live bus).

Since journal cut R4 (2026-10-22) a cell's per-call egress rows are written
inside its `desk.cell` event, in an `egress` array (same fields as the
`desk.egress` event below), so a cell is one record. A standalone `desk.egress`
event is written only for a refused frame (`refused: true`), for the end of an
async search (`verb: search_done`, which outlives the cell), and for a row that
arrived after its cell event was built. Tapes written before the cut hold one
`desk.egress` event per call. Readers take both: the bundle's `egress` layer
is built from both shapes.

`desk.cell`, one per `desk_go_eval` cell:

```json
{
  "desk_session": "agent:<agent_id> | <activation id>",
  "worker_pid": 1234,
  "run_id": "...",
  "started_at": "RFC3339Nano", "finished_at": "RFC3339Nano",
  "duration_ms": 0,
  "waited_for_run": "", "waited_ms": 0,
  "error": "", "reuse": "", "diag": "",
  "intents": 0
}
```

`desk.egress` (an element of `desk.cell`'s `egress` array, or a standalone
event in the cases above), one per host-mediated `choir.*` network or product
call made from a cell:

```json
{
  "request_id": "id the worker minted",
  "response_request_id": "id on the result frame the host wrote",
  "verb": "web_search | fetch_url | product_api | research | ...",
  "desk_session": "...", "worker_pid": 1234, "run_id": "...",
  "started_at": "RFC3339Nano", "finished_at": "RFC3339Nano",
  "is_error": false, "error": ""
}
```

Cell source and output stay in the existing `tool.result` row (`tool =
desk_go_eval`). Since journal cut R3 (2026-10-22) `tool.invoked` is a live-bus
line and is not on the tape: the `tool.result` payload carries the call's
`arguments` (the cell `source` is `arguments.source`), `started_at` and
`duration_ms`. Tapes written before the cut hold a `tool.invoked` row with the
arguments and a result without them; readers accept both.

## Drill block (written by the tracer)

```json
{
  "label": "...",
  "restart_requested_at": "RFC3339",
  "in_flight_run_id": "run observed running when the restart was sent",
  "ready_at": "RFC3339",
  "boot_at": "RFC3339",
  "boot_at_source": "observability.recent_log[0] | first answer after restart (upper bound)",
  "obligations_before": {}, "obligations_after": {},
  "quiescent": true,
  "quiescent_at": "RFC3339",
  "load": { "parallel_cell_requested": true, "second_run_submitted_at": "RFC3339", "I5_load": "present | absent | unknown" },
  "attempts": 1
}
```

- `boot_at`: when the restarted guest booted. Taken from the first line of
  `GET /api/runtime/observability` `recent_log` (standard-logger timestamp,
  UTC) when the 512-line ring has not wrapped and the stamp falls between
  `restart_requested_at` and `ready_at`; otherwise the first time the
  computer answered after the restart (an upper bound). The checker's I3
  uses it as the boot instant.
- `quiescent`: the tracer exports only after no run of the computer is
  `pending` or `running`, waiting at most `--quiesce-min` (default 2).
  `false` means the bounded wait ran out; `open_at_export` lists the runs
  still open, so an I2 failure on them is read against that.
- `load.I5_load`: `present` when the bundle shows two overlapping
  `desk.egress` calls inside one `desk.cell` (a `desk.cell` with
  `waited_for_run` set is reported, not required: runs of one desk agent
  are serialized, so the product path cannot provoke a desk wait; the
  session-pool tests cover it); `absent` when the export served those layers and
  the load never appeared (the checker reports I5 unknown); `unknown` when
  the export failed or was skipped.

The tracer also records extra non-contract fields (`in_flight_run_after`,
`restart_response`, `continued_after_restart`, `publication`, `note`); the
checker must not depend on them.

## Checker verdicts

`scripts/reeng_invariants.mjs <bundle>` prints one line per rule and writes
JSON `{rule, verdict: pass | fail | unknown, evidence: [...], reason}`.
Rules I1–I5 are defined in the Cut 0 file; I6 arrives with Cut 1's cursors.
