# World Wire Store B inventory at teardown (2026-10-09)

What the corpus store (Store B, `corpus-dolt`, :13307) held when it was
reset. Counts, owners and dates only; no content was read. Input to the
World Wire redesign (station SW). The migration plan waits on that design.
Owner, 2026-10-09: "now that youve done this inventory, document it and lets
do our world wire redesign before we commit to the plan."

## Why it was taken down

Owner: the Universal Wire app "wasnt connecting to it correctly. the
articles didnt load." Work moved to stabilizing the computer and enabling
self-development. Ingestion stopped on 10-05 (`sourcecycled` out of the
boot set). The 10-05 analysis is
[`world-wire-rearchitecture-2026-10-05`](../world-wire-rearchitecture-2026-10-05.md).

## State now

- 12:33Z: the live repo was swapped for an empty one (runbook in
  [`node-b-disk-free-space`](../problems/node-b-disk-free-space-2026-10-09.md)).
  corpusd recreated the schema; all World Wire reads now return empty.
- Old repo: `/var/lib/go-choir/corpus-dolt/corpus.reset-20261009`,
  **104 G**, almost all Dolt history (`.dolt/noms/oldgen`). Untouched.
- 12:57Z: a history-free SQL dump of the old repo's current state is being
  written to `/var/lib/go-choir/corpus-dolt/rebuild-20261009/corpus.sql`.
  It only reads the old repo and aborts below 40 G free. ~4 G in its first
  4 minutes.
- Artifact files outside Dolt: `platform-artifacts/sha256/og` is 2.4 G of
  externalized object bodies (`og_objects.body_ref`).

## Tables (current-state row counts)

| Group | Table | Rows | Notes |
|---|---|---|---|
| Ingestion: sources | `sources` | 211 | 137 RSS, 73 Telegram, 1 GDELT; all `active`; last polled 2026-10-05 |
| Ingestion: content | `items` | 2,364,413 | feed items: title, body, URL, canonical URL, published, language, region, verticals, reader snapshot, raw JSON |
| Object graph | `og_objects` | 8,761,146 | objects derived from items; bodies inline or in `og/*.bin` |
| Object graph | `og_edges` | 4,383,779 | |
| Ingestion: logs | `fetches` | 3,325,886 | one row per HTTP fetch (status, etag, raw snapshot ref) |
| Ingestion: logs | `ingestion_events` | 4,346,101 | |
| Processing logs | `cycles` / `cycle_events` | 35,252 / 235,822 | sourcecycled cycles |
| Processing logs | `processor_requests` | 215,254 | the deleted processor's queue (mostly `dispatch_failed` per 10-05) |
| Publications | `publications` | 633 | see owners below |
| Publication chain | `publication_versions`, `review_records`, `consent_records`, `publication_proposals`, `retrieval_manifests` / `_sources` / `_spans`, `artifact_blobs`, `artifact_manifests`, `provenance_*`, `verifier_attestations` | 633–1,280 each | one set per publication |
| Publication chain | `publication_source_entities`, `publication_transclusions` | 3,647 each | |
| Routes | `public_routes`, `rollback_refs` | 926 each | public URL slots |
| Wire articles | `platform_texture_documents` / `_revisions` | 185 / 545 | all owned by `universal-wire-platform`; created 06-27 to 07-12 |
| Other | `citation_edges` 641, `publication_policies` 619, `platform_subjects` 76, `proposal_delivery_records` 7, `publication_version_proposals` 7 | | |
| Empty | `issues`, `platform_vtext_*`, `reconciler_requests` | 0 | |

## Publications by owner

| Owner | Published | Dates |
|---|---|---|
| `universal-wire-platform` (the wire itself) | 485 | 06-10 to 06-30 |
| the owner (`5bd6de97…`) | **38** | 05-28 to 06-24 |
| 12 other accounts | ~110 (2–8 each) | 06-04 to 06-22 |

All 633 are `published`. Since the 12:33Z reset none of them resolve.

## How code reaches it

- Everything goes through `platform.Store.corpus()`, which **falls back to
  Store A when `CORPUSD_CORPUS_DOLT_DSN` is unset**. Unwiring the DSN is not
  a teardown: it would point World Wire at Store A's stale pre-split copies
  (the 2026-10-05 og wrong-store class).
- Product read: `GET /api/universal-wire/stories` → proxy →
  corpusd `/internal/platform/universal-wire/stories`
  (`internal/proxy/universal_wire.go`), shown by
  `frontend/src/lib/UniversalWireApp.svelte`.
- Platform computers already registered in vmctl:
  - `universal-wire-platform` (`internal/vmctl/platform_computer.go`);
  - two `candidate-fleet` VMs owned by it, one stopped and one hibernated.
- Artifact GC's `og` live set reads `og_objects` here; with an empty Store B
  every `og/*.bin` is unreachable (GC stays dry-run).

## Classes for the redesign (observations, not decisions)

- **Durable work product:** the publications and their chain, routes, and
  the 185 wire articles. Small: hundreds to low thousands of rows.
- **Source observations:** `sources`, `items`, `og_*`. Large, and still
  worth something as a starting corpus. The 10-05 director note: raw fetches
  and items are bounded working data; claims and provenance are the durable
  class.
- **Operational logs:** `fetches`, `ingestion_events`, `cycles`,
  `cycle_events`, `processor_requests`. About 12 M rows with no content
  value.
- **History:** the 104 G is Dolt history, not data. A current-state dump is
  a few GB.
