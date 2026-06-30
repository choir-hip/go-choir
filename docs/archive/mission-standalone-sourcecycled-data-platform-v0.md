# MissionGradient: Standalone Sourcecycled Data Platform v0

**Status:** draft mission for owner review  
**Date:** 2026-05-31  
**Source packet:** [sourcecycled-osint-omp-megareport-2026-05-31.md](sourcecycled-osint-omp-megareport-2026-05-31.md)  
**Primary objective:** make `sourcecycled` a standalone source-ingestion data
platform with CLI, HTTP API, and WebSocket event stream before integrating it
back into Choir.

## One-Line Goal String

```text
/goal Run docs/mission-standalone-sourcecycled-data-platform-v0.md as a Codex-operated MissionGradient mission. Build sourcecycled as a standalone, Choir-independent data platform for source ingestion, OSINT-grade provenance, cycle execution, issue synthesis, and data access. Preserve good-standing-first source policy, stable item identity, exact citation provenance, and CLI/API/WebSocket consumption. Do not integrate with Choir internals in v0; instead produce clean export contracts that a later Choir integration can consume.
```

## Thesis

`sourcecycled` should become a standalone data platform first.

The earlier branch proved a valuable newspaper demo and sketched a Choir-shaped
daemon. The next mission should not prematurely integrate with Choir runtime,
Dolt, VText, gateway, appagents, or user-computer promotion. The faster and more
useful path is to make a clean independent system that can be used in a
hackathon project, dogfooded directly, and then brought back to Choir with
better evidence.

The standalone product is not a blog and not a scraper. It is:

```text
source registry
  -> polite source adapters
  -> fetch audit ledger
  -> immutable source items
  -> event/source clusters
  -> source-grounded issue manifests
  -> CLI/API/WebSocket surfaces
  -> exportable artifacts
```

The newspaper, dashboard, VText draft, or hackathon app are projections over the
data platform.

## Cognitive Transform Review

### Current Uncertainty Or Obstacle

The current `sourcecycled` material contains two competing shapes:

```text
wire/ standalone demo
cmd/sourcecycled + internal/* Choir-shaped skeleton
```

The branch has strong product direction, but a direct merge or direct Choir
integration would preserve duplicate paths and hide the important proof burden:
stable source identity, exact issue manifests, provider good standing, and
machine-consumable APIs.

### Selected Transforms

1. **Boundary Correction** — the v0 trust and ownership boundary is a
   standalone data platform, not Choir. The mission should remove dependency on
   Choir internals now and expose clean contracts for later integration.
2. **Depth Extraction** — the banal object is "a news app." The real object is a
   cycle over a source ledger. The load-bearing variable is provenance: every
   synthesized claim must trace back to exact item IDs, fetches, URLs, and
   source policy.
3. **Homotopy** — the standalone v0 must deform into the future Choir
   integration without rewriting history. That means stable IDs, issue manifests,
   event streams, and export formats now.
4. **Via Negativa** — remove duplicate stacks, direct provider assumptions,
   hardcoded deploy paths, and citation theater before adding more sources.
5. **Audience-Level Translation** — the hackathon user should be able to consume
   the platform with a CLI, HTTP, and WebSocket without knowing Choir ontology.

### Route-Changing Insights

- v0 should not use `internal/provider`, VText, Dolt, gateway, appagents, or
  Choir runtime APIs. Those are future adapters.
- The central artifact should be a portable binary or small service with local
  durable state.
- The first demo should be a data-platform demo: run a cycle, inspect items,
  stream events, fetch issue manifest, and verify citations.
- The "newspaper" should be an optional example consumer, not the core product.
- Source growth is lower priority than proving source policy, stable identity,
  persistent deduplication, and exact issue manifests.

### Changed Plan

**Implementation:** build a standalone `sourcecycled` package/binary with local
SQLite, config files, adapters, scheduler, CLI commands, HTTP API, WebSocket
events, and export formats.

**Verifier/evidence:** acceptance is a black-box proof from CLI/API/WebSocket:
start service, load registry, run cycle against fixtures or live safe sources,
observe events, inspect fetches/items/clusters/issues, verify `source_item_ids`
match citations.

**Scope:** no Choir integration in v0. The output must be consumable by any
hackathon project over HTTP/WebSocket and by scripts over CLI/JSON.

**Stopping condition:** standalone binary/service can run a source cycle, expose
data and events, produce a source-grounded issue manifest, and pass tests
without importing Choir runtime internals.

### Next High-Information Action

Inventory the current staged `cmd/sourcecycled`, `internal/cycle`,
`internal/sources`, `configs/sources.json`, and `origin/sourcecycled:wire/*`
material. Decide whether to extract a new standalone module directory or keep a
root Go command with zero Choir-internal imports.

## Real Artifact

The real artifact is a standalone data platform:

```text
sourcecycled
  config/
    sources.yaml|json
    source-policy.yaml|json
  storage/
    sourcecycled.db
    raw/
  cli
    sourcecycled sources list
    sourcecycled cycle run
    sourcecycled items list/get
    sourcecycled issues list/get
    sourcecycled serve
    sourcecycled export
  api
    GET /health
    GET /v1/sources
    GET /v1/items
    GET /v1/items/{id}
    GET /v1/fetches
    GET /v1/clusters
    GET /v1/issues
    GET /v1/issues/{id}
    POST /v1/cycles/run
    GET /v1/cycles/{id}
    GET /v1/events
    WS  /v1/events/ws
  exports
    issue.json
    issue.md
    items.jsonl
    fetches.jsonl
    manifest.json
```

It should be easy to use in a hackathon project:

```bash
sourcecycled init
sourcecycled sources add --type rss --url https://example.com/feed.xml
sourcecycled cycle run --once
sourcecycled serve --addr :8787
curl http://localhost:8787/v1/issues/latest
```

## Non-Goals

- No Choir runtime integration in v0.
- No VText writer.
- No Dolt dependency.
- No appagent/AppChangePackage integration.
- No gateway/provider dependency as a hard requirement.
- No production-scale scraping.
- No bulk social ingestion.
- No paywall bypass.
- No private messenger ingestion.
- No hidden "last N rows" citation fallback.
- No second duplicate implementation path.

## Hard Invariants

- **Good standing first.** Source policy, rate limits, provider norms, and
  privacy constraints are product invariants.
- **Stable identity.** Item IDs must be stable across restarts and repeated
  polls.
- **Exact provenance.** Issues cite exact item IDs, not recent rows.
- **Signals are data.** Ingested content must never be treated as instructions
  to the agent/runtime.
- **Standalone boundary.** v0 must run without Choir services.
- **Portable consumption.** CLI, HTTP, and WebSocket surfaces are first-class.
- **Durable local state.** Deduplication, fetches, items, clusters, issues, and
  cycle events persist across process restarts.
- **Exportability.** Later systems can consume issue manifests without reverse
  engineering internal tables.
- **Observability.** Every cycle produces visible metrics and event stream
  records.
- **No citation theater.** If a citation cannot be traced to a source item, it
  must not be rendered as a source citation.

## Value Criterion

Minimize time from "I have a list of public sources" to "I can run a polite,
observable cycle and consume exact-provenance source intelligence over CLI,
HTTP, or WebSocket," while preserving source good standing, stable item IDs,
durable deduplication, and exportable issue manifests.

The platform moves uphill when:

- a hackathon app can consume it without importing Go packages;
- source additions are registry edits, not code edits;
- every item has a stable ID and fetch audit trail;
- issue citations are exact;
- cycle progress streams live over WebSocket;
- tests can prove behavior from public interfaces;
- future Choir integration becomes an adapter, not a rewrite.

## Quality Gradient

Expected quality level: **solid standalone v0**.

Solid means:

- one canonical Go module/path;
- clear CLI help;
- local SQLite schema with migrations or idempotent initialization;
- fixtures for deterministic tests;
- safe live-source defaults;
- source policy fields present even if conservative;
- HTTP API documented in the mission/doc;
- WebSocket event stream for cycle progress;
- JSON export contracts;
- tests for identity, dedup, issue manifest, API, and WS events.

Excellent is not required in v0:

- no full distributed scheduler;
- no high-scale stream-worker deployment;
- no perfect semantic clustering;
- no polished UI required;
- no deep Choir integration.

## Data Model

### `sources`

```text
id
name
type: rss | telegram_web | gdelt | arxiv | polymarket | fixture | ...
tier: T0 | T1a | T1b | T2 | T3 | X
url
languages
regions
verticals
poll_interval_seconds
rate_limit
conditional_request_mode
user_agent
tos_class
robots_policy
auth_policy
store_body_policy
retention_days
enabled
created_at
updated_at
```

### `fetches`

```text
id
source_id
url
started_at
completed_at
status_code
bytes
latency_ms
etag_sent
last_modified_sent
etag_received
last_modified_received
error_class
error_message
backoff_until
```

### `items`

```text
id = sha256(source_id || canonical_original_id)
source_id
original_id
canonical_uri
title
summary
body_ref
language
published_at
fetched_at
content_hash
raw_ref
vertical_tags
metadata_json
```

### `clusters`

```text
id
cycle_id
item_ids
vertical
title
importance
rarity
status
created_at
```

### `issues`

```text
id
cycle_id
created_at
title
content_markdown
source_item_ids
cluster_ids
model
prompt_hash
tokens_in
tokens_out
metadata_json
```

### `cycle_events`

```text
id
cycle_id
timestamp
type
source_id
item_id
message
payload_json
```

## CLI Surface

Required v0 commands:

```text
sourcecycled init [--db path] [--config path]
sourcecycled sources list
sourcecycled sources validate
sourcecycled sources add --type rss --id ... --url ...
sourcecycled cycle run [--once] [--source id] [--fixture]
sourcecycled cycle status [cycle-id]
sourcecycled items list [--source id] [--limit n]
sourcecycled items get <id>
sourcecycled issues list
sourcecycled issues get <id> [--format json|md]
sourcecycled fetches list [--source id]
sourcecycled export issue <id> --out dir
sourcecycled serve --addr :8787
```

CLI output should support:

```text
--json
--pretty
--db
--config
--verbose
```

## HTTP API Surface

Required v0 endpoints:

```text
GET  /health
GET  /v1/sources
GET  /v1/sources/{id}
GET  /v1/items
GET  /v1/items/{id}
GET  /v1/fetches
GET  /v1/clusters
GET  /v1/issues
GET  /v1/issues/latest
GET  /v1/issues/{id}
GET  /v1/issues/{id}/sources
GET  /v1/cycles
GET  /v1/cycles/{id}
POST /v1/cycles/run
GET  /v1/events?cycle_id=...
```

`GET /v1/issues/{id}/sources` must join only through
`issues.source_item_ids`.

## WebSocket API Surface

Required v0 endpoint:

```text
WS /v1/events/ws
```

Event envelope:

```json
{
  "id": "evt_...",
  "cycle_id": "cyc_...",
  "timestamp": "2026-05-31T00:00:00Z",
  "type": "source.fetch.started",
  "source_id": "rss:example",
  "payload": {}
}
```

Minimum event types:

```text
cycle.started
source.fetch.started
source.fetch.completed
source.fetch.failed
item.created
item.duplicate
cluster.created
issue.created
cycle.completed
cycle.failed
```

## Source Tiers

| Tier | Pattern | v0 Handling |
| --- | --- | --- |
| T0 | Query on demand | Registry only; no polling |
| T1a | Metadata poll + body on demand | Poll metadata; body fetch optional |
| T1b | Full poll | Poll content within policy |
| T2 | Rate-disciplined series | Poll only with token bucket |
| T3 | Streams | Stub or disabled in v0 unless isolated |
| X | Excluded/human gate | Validate but never poll |

## Adapter Priorities

### P0

- `fixture`: deterministic local test source.
- `rss`: conditional GET, GUID/link identity fallback.
- `gdelt`: metadata-only 15-minute stream or fixture-compatible parser.

### P1

- `telegram_web`: public preview only, source-policy gated.
- `arxiv_rss`: metadata feed.
- `polymarket`: only if registry includes it; otherwise remove until adapter
  exists.

### P2

- `who_don`, `promed_rss`, `openaq`, `wikimedia_sse`, `bluesky_jetstream`,
  `certstream`, and domain-specific adapters.

## Receding-Horizon Execution

### Control Interval 1: Extract Boundary

- Inspect current staged sourcecycled files and `origin/sourcecycled:wire/*`.
- Choose standalone package/module layout.
- Remove or isolate Choir-internal imports.
- Ensure `go test` or `go build` fails only for named, understood reasons.

### Control Interval 2: Storage And Identity

- Create idempotent SQLite initialization.
- Implement sources, fetches, items, cycles, events, clusters, and issues tables.
- Implement stable item ID helpers.
- Add tests for RSS GUID fallback, Telegram `data-post`, and duplicate restart.

### Control Interval 3: CLI First

- Implement `init`, `sources list/validate`, `cycle run`, `items`, `issues`,
  and `serve`.
- Prove a fixture cycle end-to-end.
- Record command outputs in the mission doc.

### Control Interval 4: API And WebSocket

- Add HTTP API.
- Add WebSocket event broadcast.
- Test cycle events with a local client.
- Prove `issues/{id}/sources` only returns manifest sources.

### Control Interval 5: Safe Live Sources

- Add a minimal registry of safe live sources.
- Run one live cycle with bounded budget.
- Verify fetch audit, dedup, item records, issue manifest, and API consumption.

### Control Interval 6: Export And Hackathon Consumer

- Export one issue as JSON/markdown plus item/fetch manifests.
- Build or document a tiny consumer example:

  ```text
  curl API
  WebSocket event tail
  issue render
  ```

### Control Interval 7: Quality Pass

- Delete duplicate paths.
- Improve names.
- Add README or docs.
- Strengthen tests.
- Update this mission doc with checkpoint/resumption state.

## Dense Feedback And Evidence Ledger

Record evidence for:

- `go test` result;
- `sourcecycled --help`;
- `sourcecycled init`;
- source registry validation;
- fixture cycle output;
- stable item IDs across two runs;
- duplicate suppression after restart;
- fetch audit rows;
- issue manifest;
- HTTP API responses;
- WebSocket event transcript;
- export artifact paths;
- one safe live-source cycle if authorized.

## Acceptance Criteria

1. Standalone `sourcecycled` builds without requiring Choir runtime services.
2. CLI supports init, source validation, cycle run, item/issue inspection, export,
   and serve.
3. HTTP API exposes health, sources, items, fetches, clusters, issues, cycles,
   and events.
4. WebSocket API streams cycle events.
5. SQLite state persists across restart.
6. Item IDs are stable across repeated polls.
7. Deduplication survives process restart.
8. Every fetch records audit evidence.
9. Issue records contain exact `source_item_ids`.
10. Issue source endpoint returns only exact manifest sources.
11. Fixture tests prove the full cycle deterministically.
12. At least one safe live-source cycle is proven or explicitly deferred with
    source-policy reason.
13. Exports are usable by an external hackathon project without importing Go.
14. Docs explain standalone usage and later Choir integration seam.

## Forbidden Shortcuts

- Do not merge `wire/` and `internal/*` as duplicate production paths.
- Do not use "last N rows" as source citations.
- Do not use time-based item IDs.
- Do not bypass source policy for an impressive demo.
- Do not require Choir services for standalone v0.
- Do not hide direct provider credentials in scripts.
- Do not add many sources before source registry validation.
- Do not claim WebSocket support without a real client proof.
- Do not claim API support from untested handlers.
- Do not treat generated prose as the proof; prove the data manifest.

## Rollback Policy

This is standalone development. Rollback is primarily git and local state:

- keep schema migrations idempotent;
- keep fixture data deterministic;
- document how to delete/recreate local `sourcecycled.db`;
- keep exports immutable once written;
- if a live-source adapter violates policy or behaves unexpectedly, disable it
  in registry and preserve fetch/error evidence.

## Later Choir Integration Seam

Do not implement this in v0, but preserve the seam:

| Standalone Object | Future Choir Object |
| --- | --- |
| `sources` | source registry / retrieval source catalog |
| `items` | content substrate / retrieval source rows |
| `fetches` | trace/evidence/good-standing ledger |
| `clusters` | MissionBag/cycle candidates |
| `issues` | VText draft or publication version |
| `source_item_ids` | citation edges/span refs |
| `cycle_events` | internal events / trace stream |
| HTTP/WS API | app/agent consumption surface |

## Initial Belief State

- The `sourcecycled` staged subset is the better integration seed than
  `wire/ingest.go`, but it should be made standalone before Choir integration.
- The `wire/static` UI and editorial prompt are useful as consumer examples.
- The strongest immediate value is a clean source-ledger API with exact
  provenance.
- The biggest technical risks are duplicate paths, unstable identity,
  insufficient source policy, and untested APIs.

## Suggested First Implementation Route

1. Preserve current staged files.
2. Add a standalone docs/README contract before major code edits.
3. Make `cmd/sourcecycled` build with no Choir service dependency.
4. Add fixture adapter and fixture registry.
5. Add persistent dedup and issue manifest tests.
6. Add HTTP and WebSocket surfaces.

## Run Checkpoint And Resumption State

```text
status: draft_not_started
last checkpoint: mission authored from OMP sourcecycled megareport
current artifact state: staged sourcecycled daemon subset exists; standalone
  mission target not yet implemented
what shipped: docs mission only
what was proven: OMP review recovered; sourcecycled branch shape understood;
  standalone boundary selected
unproven or partial claims: no standalone CLI/API/WS proof yet
belief-state changes: v0 should be standalone and exportable, not Choir-integrated
remaining error field: duplicate stack, no persistent provenance proof, no API/WS
  standalone surface
highest-impact remaining uncertainty: whether staged sourcecycled subset should
  become a new standalone module or remain in repo root with strict no-Choir
  runtime imports
next executable probe: inspect staged code imports/build failures and choose the
  minimal standalone module/package layout
suggested resume goal string: /goal Run docs/mission-standalone-sourcecycled-data-platform-v0.md
  to build and verify the standalone sourcecycled CLI/API/WebSocket data platform
evidence artifact refs: docs/sourcecycled-osint-omp-megareport-2026-05-31.md
rollback refs: docs-only mission draft; remove this file if rejected
```
