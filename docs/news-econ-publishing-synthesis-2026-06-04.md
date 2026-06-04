# News, Economics, Publishing, And First-Client Synthesis

**Date:** 2026-06-04  
**Mode:** read-only repo/subagent review plus synthesis  
**Primary question:** how to land the Choir news system while accounting for
the economics forecasting PoC, current VText/publishing gaps, recently found
bugs, and the first-client private legal cloud brief.

## Executive Summary

Choir is converging on the right product object: a document-first private
knowledge cloud where sources, drafts, citations, publications, and agent work
become durable artifacts. The news system should not land as a standalone
newspaper app bolted beside VText. It should land as a source ledger and issue
manifest substrate that can feed VText, public posts, client briefs, source
entities, and later automatic radio.

The current codebase has three important threads in flight:

1. **Sourcecycled/news:** a staged Go skeleton for RSS, Telegram, GDELT,
   vertical clustering, SQLite storage, and LLM synthesis. It is directionally
   useful but not product-ready and currently does not build because
   `github.com/mmcdole/gofeed` is imported without a `go.mod` entry.
2. **VText/source entities:** a much stronger landed substrate for inline
   expandable source references in VText, with deployed proof over a real
   YouTube target and live appagent preservation of `[label](source:ENTITY_ID)`.
3. **Publishing:** a meaningful platform publication skeleton exists, but it is
   not yet client-ready. It lacks copy full text, download/export, source
   metadata preservation through publication, polished public posts, and
   exact-span transclusion authoring.

The economics forecasting PoC in `yusefmosiah/marco` should be consulted now,
but not integrated wholesale. Its strongest contribution is not the forecast
models themselves. Its strongest contribution is provenance discipline:
official source registries, fetch records, stable item IDs, source manifests,
artifact handoffs, and explicit caveat labels such as `vintage_policy` and
`lookahead_status`.

The first-client legal cloud brief sharpens the priority order. The product
must become a private, cited, versioned, publishable document system with a
base artifact layer and role-gated publishing. News and economics matter
because they exercise the same source/provenance machinery that legal work
needs: exact citations, source confidence, document workflows, private corpus
boundaries, public/private publication, and client portal delivery.

## Current Repo State

### Head And Deploy

Local `main` is at:

```text
13a3404 Gate Node B deploys behind manual dispatch
```

Current staging health reports:

```text
proxy/upstream deployed_commit = 2d98c7df559dcffc2cfbc0fb3f87d1b1910ad963
deployed_at = 2026-05-31T21:29:42Z
```

This is expected: Node B deploys are now gated behind manual dispatch. The
latest local commit is a workflow/deploy-gating change, not a deployed runtime
behavior change.

### Dirty Worktree

Staged sourcecycled/news files:

- `cmd/sourcecycled/main.go`
- `configs/sources.json`
- `internal/cycle/cycle.go`
- `internal/cycle/storage.go`
- `internal/cycle/synthesize.go`
- `internal/sources/gdelt.go`
- `internal/sources/rss.go`
- `internal/sources/telegram.go`
- `internal/sources/types.go`

Modified VText draft/versioning files:

- `frontend/src/lib/VTextEditor.svelte`
- `frontend/tests/vtext-document-stream.spec.js`

Untracked related docs:

- `docs/mission-standalone-sourcecycled-data-platform-v0.md`
- `docs/sourcecycled-osint-omp-megareport-2026-05-31.md`
- `docs/vtext-user-draft-versioning-problem-2026-06-03.md`

There are no lingering Playwright/build/test processes from the interrupted
turn.

## Subagent Review Inputs

Three read-only subagents reviewed independent slices:

- **News/sourcecycled reviewer:** current sourcecycled/news implementation,
  docs, staged files, and source/entity integration implications.
- **Economics reviewer:** `https://github.com/yusefmosiah/marco` at
  `9c60babf7910580b142cffea02e100a28c47d1ef`; tests passed there:
  `49 passed in 11.52s`.
- **Publishing/VText reviewer:** VText, platform publication, source entities,
  transclusion, copy/download, and client-ready publishing gaps.

## Sourcecycled / News Review

### What Exists

The staged sourcecycled code is a Go skeleton with:

- JSON source registry in `configs/sources.json`;
- source types and item structs in `internal/sources/types.go`;
- RSS adapter using `gofeed`;
- Telegram web-preview scraper;
- GDELT GKG fetcher;
- concurrent polling and in-memory deduplication;
- vertical bucket clustering;
- Fireworks/DeepSeek synthesis through Choir `internal/provider`;
- SQLite item and issue storage;
- daemon entrypoint in `cmd/sourcecycled/main.go`.

The docs are more mature than the implementation. The standalone mission doc
correctly defines the target as:

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

### Current Blockers

The sourcecycled packages currently do not build. `internal/sources/rss.go`
imports:

```go
github.com/mmcdole/gofeed
```

but that dependency is absent from `go.mod` and `go.sum`.

Other concrete gaps:

- `configs/sources.json` includes `polymarket`, but no Polymarket adapter is
  implemented.
- Deduplication is in-memory only, so duplicates return after process restart.
- Storage does not yet contain fetches, cycles, clusters, cycle events, source
  policy, content hashes, prompt hashes, or exact citation maps.
- RSS identity uses GUID directly and needs a robust fallback for empty GUIDs.
- GDELT and Telegram fetch paths lack fetch ledger, rate policy, status
  validation, persistent backoff, and provider-good-standing evidence.
- Synthesis asks the model to cite `[Sx-y]`, but saved issues only track all
  item IDs globally; there is no parsed issue manifest proving each rendered
  citation maps to exact source items.
- The current command is a blocking daemon only. There is no CLI subcommand
  interface, HTTP API, WebSocket stream, export format, or black-box acceptance
  proof.
- The staged code imports Choir internals, while the standalone mission argues
  for a Choir-independent v0.

### Recommendation

Do not merge the staged sourcecycled code as the product path as-is. Either:

1. land it as an explicitly experimental checkpoint after fixing the build and
   documenting its limitations; or
2. reshape it first into the standalone data platform described in
   `docs/mission-standalone-sourcecycled-data-platform-v0.md`.

The second path is cleaner. The product object should be a data/source ledger,
not a newspaper daemon.

## Marco Economics PoC Review

### What Marco Is

Marco is an artifact-first Python PoC for macroeconomic research:

```text
source/provenance
  -> normalized macro panels
  -> backtests/experiment artifacts
  -> agent/API/frontend consumption
```

Important modules:

- `src/emf_macro/news.py`: official RSS/RDF/Atom ingestion, fetch audit rows,
  stable item IDs, source manifests, JSONL bundles.
- `src/emf_macro/macro_forecast.py`: FRED-MD target construction, walk-forward
  baselines, VAR, linear top-factor models, optional XGBoost, optional
  Chronos-2 path.
- `src/emf_macro/economic_model_agent.py`: forecast summary and metrics handoff
  artifacts.
- `src/emf_macro/news_agent.py`: deterministic news ledger maintenance,
  marginal updates, token-budgeted `model.md`, exact item citation discipline.
- `configs/news_sources.json`: useful official macro-policy source registry.

### Useful For Choir

Marco should inform Choir’s news/data platform in five ways:

1. **Official macro source lane.** Fed, ECB, BIS, SEC, BEA, RBI, BoE, BoJ, and
   similar feeds are a high-quality first macro-policy source set.
2. **Fetch/item provenance contracts.** Source manifests, fetch records,
   stable item IDs, raw snapshot hashes, and publication snapshot labels are
   directly relevant.
3. **Citation discipline.** Generated economic/news summaries should cite exact
   item IDs, not loosely reference source names.
4. **Artifact handoffs.** Forecasts and news ledgers should become structured
   research artifacts that VText can cite and inspect.
5. **Forecast caveats.** Economic claims need fields like `vintage_policy`,
   `lookahead_status`, and `evidence_level`.

### What Not To Import Yet

Do not integrate Marco’s forecast models as authoritative Choir behavior yet.
Marco mostly uses latest-revised macro snapshots, not real-time vintage-safe
data. Its backtests are useful for experimentation, but they can overstate
real-world forecast validity.

Do not adopt Marco’s local Python HTTP server as a production boundary. Use the
schemas and source lists, then reimplement the contracts in Choir or a
standalone sourcecycled service.

## Publishing / VText Review

### What Exists

Choir already has a real publication skeleton:

- private VText documents/revisions with metadata, citations, history, diff,
  blame, and owner scoping;
- a limited VText Markdown renderer;
- inline Source Entity refs like `[label](source:id)`;
- platform publish proxy from private VText to platformd;
- platform publication/version/routes/artifacts/retrieval/citation/provenance
  rows;
- public publication bundle resolution;
- public reader derivative/proposal flow;
- source entity/media-source detection for YouTube/images, transcript
  availability fields, and source cards.

### Current Gaps

Publishing is not client-ready yet:

- no **copy full text** action;
- no **download/export** endpoint or UI for Markdown, plain text, HTML, PDF, or
  DOCX;
- published artifacts are effectively text blobs;
- public posts lack client-ready metadata and polish: author, published date,
  SEO/social metadata, canonical render, custom slug workflow, preview, and
  status controls;
- publishing currently forwards content and citations, but not full revision
  metadata such as `source_entities`, `media_source_refs`, and transclusions;
- embedded links are basic Markdown links, not validated rich embeds;
- embedded media is private VText/source-entity oriented, not projected into a
  public artifact manifest;
- transclusion is ledger/proposal-level, not an authoring-level span picker;
- retrieval spans are currently too coarse for “quote this paragraph” or
  “transclude this selected excerpt”;
- proposal delivery is not yet a polished author review inbox.

### Most Important Publishing Fix

The highest-leverage publishing improvement is not PDF export first. It is:

```text
preserve immutable source/provenance metadata through publication,
then expose copy/download from the immutable artifact.
```

If copy/download operates from rendered DOM or drops source metadata, it will
weaken the citation economy. Export should come from the canonical artifact and
render model.

## First-Client Brief Implications

The attached draft brief describes a private legal cloud. It reframes the news
mission because the first paid wedge is not “a news product.” The wedge is:

```text
private document cloud
  + base artifact layer
  + source/citation/provenance
  + style/rubric documents
  + client/assistant/partner portals
  + private inference evaluation
```

The draft’s strongest product claims align with Choir’s architecture:

- the core object is the document, not chat;
- sources and citations are first-class;
- work product must be versioned and durable;
- private infrastructure and role boundaries matter;
- style guides/rubrics should be versioned artifacts;
- publishing/client portals are a core interface;
- the “base layer” is the artifact layer between user devices, clients,
  assistants, and agents.

The news system should therefore become part of the same data/source substrate
needed for legal work:

- source registries;
- fetch ledgers;
- artifact ingestion;
- source entities;
- cited synthesis;
- publishable briefs;
- role-gated publication and download.

## Prioritized Plan To Land The Plane

### P0: Stabilize Current VText Draft Versioning

There is local WIP that changes VText autosave from canonical revision creation
to local draft persistence. That is directionally right because user typing
should not create many canonical versions before the user explicitly asks VText
to revise or save.

Before landing more source/news work, reconcile this with existing Source
Entity acceptance. `frontend/tests/vtext-source-entities.spec.js` still assumes
typed edits create canonical autosave revisions. The new draft model should
instead prove:

```text
typed draft persists without advancing canonical versions
explicit revise/save creates one user revision
source refs survive that explicit revision
appagent revision preserves source refs
```

### P0: Decide And Enforce Sourcecycled Boundary

The current staged sourcecycled code conflicts with its own mission doc. The
doc says standalone v0; the code imports Choir internals and is a daemon.

Recommendation:

- make sourcecycled standalone for v0;
- expose CLI/API/WebSocket and export contracts;
- treat Choir integration as a later adapter;
- do not let sourcecycled become a second publishing system.

### P0: Build Provenance Before More Feeds

The next sourcecycled implementation pass should prioritize:

- source registry fields for tier, region, language, vertical, rate policy,
  body retention, auth policy, and source-good-standing;
- persistent fetch audit records;
- stable item IDs and content hashes;
- persistent dedup;
- cycles and cycle events;
- clusters and issue manifests;
- exact citation maps from generated issue text to item IDs;
- black-box tests over CLI/API/export.

### P1: Add Publishing Export Primitives

Add the smallest client-visible publishing features that compound:

- copy full text;
- download Markdown;
- download plain text;
- download HTML;
- later PDF/DOCX after render model is stable.

These should be implemented against immutable private revisions and immutable
publication versions, not transient DOM.

### P1: Preserve Metadata Through Publication

Extend publish payloads and platform records so published VText versions keep:

- source entities;
- media source refs;
- transclusions;
- content item references;
- publication/source provenance;
- render block metadata.

This is necessary for public expandable citations and client-ready downloads.

### P1: Use Marco As A Macro-Policy Lane

Consult Marco immediately for:

- official macro source list;
- source registry schema;
- fetch record shape;
- source manifest shape;
- forecast caveat fields.

Do not ship Marco forecasts as authoritative product output yet. If forecast
artifacts are used, label them as research artifacts with explicit vintage and
lookahead caveats.

### P2: Finish The Client Brief In VText

The attached legal cloud draft should become a real VText/publication proof:

1. create/import the brief as a VText;
2. apply a style/rubric pass;
3. add source/citation placeholders or source entities where claims need
   backing;
4. publish a client-ready preview;
5. copy/download the full text from the product path;
6. verify the public link and export artifacts.

This becomes a concrete dogfood loop for the publishing system.

## Recommended Next Mission

The next coding mission should not start with more source adapters. It should
be:

```text
Fix VText draft versioning and source-ref proof semantics, then add publication
copy/download primitives that preserve source metadata.
```

That mission directly serves:

- VText stability;
- first-client brief delivery;
- public/client publishing;
- source entity persistence;
- future news issue publication.

The sourcecycled standalone mission should run next or in parallel only after
the current staged WIP is either isolated or made buildable.

## Concrete Acceptance Gates

### VText Draft Gate

- Typing in VText does not create a new canonical revision.
- Reload restores the draft.
- Explicit revise/save creates exactly one user revision.
- Source refs survive that user revision.
- Appagent revision preserves source refs.

### Publishing Gate

- A VText revision can be copied as full text.
- A VText revision can be downloaded as `.txt`, `.md`, and `.html`.
- A published version can be copied/downloaded from the public reader.
- Export uses immutable revision/publication content, not DOM scraping.
- Source metadata is preserved through publication.

### Sourcecycled Gate

- `go test ./cmd/sourcecycled ./internal/cycle ./internal/sources` passes.
- Source registry rejects unsupported source types or has adapters for them.
- Dedup persists across restart.
- Fetch records exist for every request.
- Issue manifests map citations to exact item IDs.
- CLI/API/export surfaces prove the data platform without Choir runtime.

### Marco-Informed Economics Gate

- Macro-policy sources enter the source registry with policy metadata.
- Any forecast-derived claim includes `vintage_policy`, `lookahead_status`,
  model, target, data date, and evidence level.
- Forecast artifacts are cited as research artifacts, not treated as ground
  truth.

## Open Questions

- Should sourcecycled live in this repo as a standalone command/module, or move
  to a separate repo now?
- Should the first client brief be published as a public demo, an authenticated
  portal document, or a private share link?
- What export formats are required immediately beyond `txt`, `md`, and `html`?
- Is PDF export acceptable through generated HTML first, with DOCX later?
- Which economic source lane matters first for news: official macro policy,
  market data, prediction markets, or legal/regulatory updates?

## Bottom Line

Land the news system by landing the source ledger, not by shipping a newspaper
surface first. Use Marco to improve provenance and economic caveats, not as a
runtime dependency. Use the first-client brief to force publishing quality:
copy, download, metadata preservation, and role-gated public/private delivery.

The immediate path is narrow and high leverage:

```text
VText draft/versioning repair
  -> publication copy/download/export
  -> metadata-preserving publication
  -> sourcecycled standalone provenance substrate
  -> news/econ VText/publication projection
```

