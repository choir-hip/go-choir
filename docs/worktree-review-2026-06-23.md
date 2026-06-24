# Worktree Review Report — 2026-06-23

## Overview

On 2026-06-23, seven Parallax mission worktrees were reviewed. Each was
spawned from main (`27af9008`) to advance a specific piece of the
object-graph refactor. This report covers the specifics of each worktree's
changes, quality assessment, and disposition recommendation.

## Worktree Inventory

| # | Paradoc | Worktree Path | Branch | HEAD | Committed? | Mutation Class |
|---|---------|---------------|--------|------|------------|----------------|
| 1 | Email Freeze Diagnosis | `~/.codex/worktrees/a98a/go-choir` | `diagnose/email-freeze` | `87f8adec` | 4 commits | Orange |
| 2 | Docs Checker Clear | `~/.windsurf/.../go-choir-6b7967c1` | `cascade/new-cascade-6b7967` | `f4966f68` | Uncommitted | Green/Yellow |
| 3 | Object Service Prototype | `~/.windsurf/.../go-choir-29131320` | `cascade/new-cascade-291313` | `468e739e` | Uncommitted | Orange |
| 4 | Qdrant Indexing Pipeline | `~/.windsurf/.../go-choir-87c664e7` | `cascade/new-cascade-87c664` | `b1b7b671` | Uncommitted | Yellow/Orange |
| 5 | Source Entity Migration | `~/.codex/worktrees/2bae/go-choir` | detached HEAD | `27af9008` | Uncommitted | Red/Orange (design only) |
| 6 | PPTX Renderer Prototype | `~/.windsurf/.../go-choir-f4fdeb09` | `cascade/new-cascade-f4fdeb` | `42689a92` | Uncommitted | Green/Yellow |
| 7 | Universal Wire Diagnosis | `~/.codex/worktrees/1569/go-choir` | detached HEAD | `27af9008` | Uncommitted | Orange (diagnosis only) |

**Risk note:** Five of seven worktrees have uncommitted changes on detached
HEADs or snapshot branches. If these worktrees are removed without committing
or stashing, the work is lost.

---

## 1. Email Freeze Diagnosis

**Worktree:** `~/.codex/worktrees/a98a/go-choir`
**Branch:** `diagnose/email-freeze`
**Commits:** 4 (`b6792772` → `f1111fe9` → `6706ae02` → `87f8adec`)
**Status:** Settled at branch level. Independent verifier accepted.

### Files Changed

| File | Change |
|------|--------|
| `docs/email-freeze-diagnosis-2026-06-23.md` | +147 lines (new diagnosis doc) |
| `docs/paradoc-email-freeze-diagnosis.md` | +28 lines (paradoc updated with Parallax State) |
| `docs/paradoc-email-freeze-diagnosis.ledger.md` | +31 lines (new ledger) |
| `frontend/src/lib/EmailApp.svelte` | +166/-40 (bootstrap hardening) |
| `frontend/tests/email-app-state.spec.js` | +133 lines (new Playwright spec) |

### Diagnosis Findings

The mission investigated the reported Email app freeze through staging browser
probes against `https://choir.news`.

**Reproduction attempts:**

- Probe 1: Registered a temporary passkey user, opened Email, cycled folders.
  No console errors, no 401 renewal loop, no hard freeze observed. Network
  showed duplicate `GET /api/email/aliases` and duplicate `GET /api/email/messages?folder=inbox`.
- Probe 2: Delayed the first `GET /api/email/messages?folder=inbox` response
  by 12 seconds. The app did not wedge — the second request returned 200 and
  the UI recovered.

**Root cause identified:**

The Email app had a dual bootstrap hazard:

1. `onMount` calls `loadAliases()` and `loadMessages(...)`.
2. A reactive block (`authenticated && !loadedOnce && !loading`) also calls
   `loadAliases()` and `loadMessages(...)`.

Both fire on initial open, producing duplicate requests. There was no request
generation guard, no timeout, and no abort. A stale or hung request could
overwrite newer state or keep `loading` true indefinitely.

**Weakened hypotheses:**

- Auth renewal loop: `fetchWithRenewal` performs one request, one
  `renewSession()` on 401, one retry. No loop. Not the cause.
- Desktop suspension: Email app registry has `heavy: false`.
  `suspendBackgroundHeavyWindows` only targets heavy apps. Not the cause.

**Not reproduced:** The exact reported hard freeze on an affected account. The
fix is preventive for the confirmed request-state hazard.

### Code Fix

`EmailApp.svelte` was hardened with:

1. **Single guarded bootstrap:** Replaced dual `onMount` + reactive `!loadedOnce`
   paths with one `bootstrapMailbox()` function controlled by an
   `initialLoadStarted` guard.
2. **Request generation guards:** `loadMessages` and `loadDetail` now use
   monotonically increasing generation counters (`messageLoadGeneration`,
   `detailLoadGeneration`). Only the latest generation can mutate `messages`,
   `selectedId`, `detail`, `error`, and `loading`.
3. **Cross-load invalidation:** Starting a new mailbox load invalidates any
   previous detail load via `ownerMessageLoad` parameter.
4. **Fetch timeout:** All Email API calls now use `fetchEmailWithTimeout`
   with a 15-second `AbortController` timeout. `fetchWithRenewal` remains the
   one-renewal auth helper.
5. **Stale-response guards:** Every `await` boundary checks
   `isLatestMessageLoad(requestId)` or `isLatestDetailLoad(requestId,
   ownerMessageLoad)` before mutating state.

### Tests

`frontend/tests/email-app-state.spec.js` adds two Playwright tests:

1. **"email bootstrap performs one aliases request and one mailbox request"**:
   Routes all `/api/email/**` through Playwright mock, opens Email, asserts
   exactly one `GET /api/email/aliases` and one `GET /api/email/messages?folder=inbox`.
2. **"stale slower mailbox response cannot overwrite newer folder state"**:
   Holds the inbox response, switches to Sent folder, verifies Sent state is
   shown, then releases the inbox response and verifies it does not overwrite
   the Sent folder view.

### Verification

- `npm run build` passed.
- Focused Playwright execution was blocked by local auth-origin harness issues
  (stale auth service on port 8081, WebAuthn origin mismatch with `127.0.0.1`).
  This is a harness problem, not an Email behavior problem.
- Independent verifier thread `019ef323-c1a8-7640-bb77-a8e64c774160` reviewed
  branch head `6706ae02` and returned `accept` with no blocking findings.

### Disposition: **KEEP — land through staging loop**

This is the most production-ready work of all seven worktrees. The fix is
clean, minimal, and addresses a real hazard. The Playwright tests are
well-structured. Needs staging landing loop verification per AGENTS.md.

---

## 2. Docs Checker Clear

**Worktree:** `~/.windsurf/worktrees/go-choir/go-choir-6b7967c1`
**Branch:** `cascade/new-cascade-6b7967` (snapshot at `f4966f68`)
**Commits:** None (uncommitted working changes)

### Files Changed

| File | Lines |
|------|-------|
| `AGENTS.md` | +8/-4 |
| `README.md` | +14/-7 |
| `docs/README.md` | +10/-3 |
| `docs/choir-doctrine.md` | +92/-46 |
| `docs/computer-ontology.md` | +4/-2 |
| `docs/current-architecture.md` | +16/-8 |
| `docs/doc-authority-manifest.yaml` | +1444 lines |
| `docs/mission-graph.yaml` | +10 |
| `docs/mission-portfolio-2026-06-11.md` | +30/-15 |
| `docs/platform-os-app-state.md` | +16/-8 |
| `docs/runtime-invariants.md` | +6/-3 |
| `docs/source-external-data-publication.md` | +8/-4 |
| `docs/texture-agentic-invariants-2026-06-13.md` | +16/-8 |
| `internal/proxy/platform_publish_test.go` | +2/-2 |
| `internal/runtime/model_policy_test.go` | +4/-4 |
| **Total** | **+1565/-115** |

### Vocabulary Updates

The docs changes are mechanical vocabulary replacements across current
documentation:

- "vtext" → "predecessor" (the pre-Texture document system name)
- "lease" → "retired lease" (the rejected worker-lease control concept)
- "Terminal app" → "retired terminal" (the removed raw terminal app)
- "Trace app" / "Open Trace" → "retired Trace app" (Trace is not a user app)
- `buildAgentRevisionRequest` → "retired `buildAgentRevisionRequest`"
- Detector patterns in `choir-doctrine.md` updated: `initialTextureToolChoice`
  → `initial`+`TextureToolChoice`, `WithInitialToolChoice` →
  `With`+`InitialToolChoice`

These changes bring current docs into alignment with the object-graph
vocabulary refactor.

### Manifest Expansion

`docs/doc-authority-manifest.yaml` was expanded by +1444 lines. Approximately
100+ historical mission docs, evidence docs, and ADRs were added with:

```yaml
claim_scope: historical
is_root: false
is_evidence: true
annotations:
  doc_role: historical_evidence
  authority: support
  lifecycle: archived
```

This resolves R3 ("not reachable from current roots") warnings by registering
the docs in the manifest, and resolves H1/H5 warnings by marking them as
historical evidence rather than current claims.

One existing entry was updated:
`docs/mission-texture-hard-cutover-v0.md`: `claim_scope: current` →
`historical`, `is_evidence: false` → `true`.

### Test File Changes

Two Go test files were modified to avoid triggering doccheck detectors:

**`internal/proxy/platform_publish_test.go`:**
```diff
-// duplicate router case (introduced by the vtext->Texture blind rename) shadowed
+// duplicate router case (introduced by the predecessor->Texture blind rename) shadowed
```

**`internal/runtime/model_policy_test.go`:**
```diff
-    if strings.Contains(raw, "[roles.vtext]") {
-        t.Fatalf("generated model policy still contains legacy [roles.vtext]:\n%s", raw)
+    if strings.Contains(raw, "[roles.v"+"text]") {
+        t.Fatalf("generated model policy still contains legacy [roles.v"+"text]:\n%s", raw)
```

The `model_policy_test.go` change is a workaround: it splits the string
`"vtext"` into `"v"+"text"` so the doccheck scanner won't flag the test file
for using retired vocabulary. This is fragile and makes the test harder to
read.

### Disposition: **KEEP with cleanup**

The docs vocabulary updates and manifest expansion are valuable and should be
landed. The test file workaround in `model_policy_test.go` should be
replaced with a cleaner approach — either exempt `_test.go` files from the
doccheck scanner, or use a constant:

```go
const legacyRoleName = "vtext" // retired; kept for regression check
if strings.Contains(raw, "[roles."+legacyRoleName+"]") {
```

---

## 3. Object Service Prototype

**Worktree:** `~/.windsurf/worktrees/go-choir/go-choir-29131320`
**Branch:** `cascade/new-cascade-291313` (snapshot at `468e739e`)
**Commits:** None (uncommitted working changes)

### Files Created

| File | Purpose |
|------|---------|
| `internal/objectgraph/object.go` | Core types: `Object`, `Edge`, `ObjectKind`, `EdgeKind`, `CanonicalID`, `ContentHash` |
| `internal/objectgraph/store.go` | `Store` interface, `ListFilter`, `ErrNotFound` |
| `internal/objectgraph/service.go` | `Service` with `CreateObject`, `GetObject`, `ListObjects`, `PutEdge`, `QueryObjects` |
| `internal/objectgraph/registry.go` | Kind/edge registration, YAML loading, 14 default kinds, 11 edge kinds |
| `internal/objectgraph/dolt_store.go` | Dolt-backed store adapter |
| `internal/objectgraph/sqlite_store.go` | SQLite-backed store adapter |
| `internal/objectgraph/memory_store.go` | In-memory store (testing/reference) |
| `internal/objectgraph/blob_store.go` | Content-addressed file blob store |
| `internal/objectgraph/service_test.go` | Unit tests |

### API Surface

The service API matches the paradoc specification:

```go
CreateObject(ctx, kind, body, metadata) (canonicalID, error)
GetObject(ctx, id, version) (object, error)
ListObjects(ctx, filter) ([]object, error)
PutEdge(ctx, from, to, kind, metadata) (edgeID, error)
QueryObjects(ctx, vector, structured, provenance) ([]object, error) // stub
```

### Canonical ID Scheme

```
obj:<kind>:<owner>:<hash-or-uuid>
```

Where `<kind>` is the registered object kind (e.g., `choir.source_entity`),
`<owner>` is the owning user/computer ID, and `<hash>` is a SHA-256 content
hash or UUID.

### Default Object Kinds (14)

| Kind | Store | Versioned | Body in Blob |
|------|-------|-----------|-------------|
| `choir.texture_doc` | Dolt | yes | no |
| `choir.texture_paragraph` | Dolt | no | no |
| `choir.texture_revision` | Dolt | yes | no |
| `choir.source_entity` | Dolt | no | no |
| `choir.source_ref` | Dolt | no | no |
| `choir.mail_message` | SQLite | no | no |
| `choir.mail_thread` | SQLite | no | no |
| `choir.contact` | SQLite | no | no |
| `choir.web_capture` | Dolt | no | yes |
| `choir.calendar_event` | SQLite | no | no |
| `choir.file_blob` | Dolt | no | yes |
| `choir.supervision_observation` | Dolt | no | no |
| `choir.supervision_finding` | Dolt | no | no |
| `choir.supervision_message` | Dolt | no | no |

### Default Edge Kinds (11)

`contains`, `cites`, `revises`, `authored_by`, `references`, `depends_on`,
`responds_to`, `has_avatar`, `has_attachment`, `captured_from`,
`belongs_to_thread`.

### Store Adapters

- **`MemoryStore`**: Fully implemented. In-memory map with mutex protection.
  Useful for testing and as a reference implementation.
- **`FileBlobStore`**: Fully implemented. Content-addressed file storage with
  SHA-256 hashing, directory sharding (`hash[:2]/hash[2:]`), and dedup.
- **`DoltStore`**: Implements the `Store` interface. Uses Dolt SQL for
  persistence. The implementation includes table creation, object
  insert/query/list, and edge storage.
- **`SQLiteStore`**: Implements the `Store` interface. Uses SQLite for
  host-level lightweight objects.

### Tests

`service_test.go` covers:

- `TestServiceCreateObject_GetObject`: Round-trip create and retrieve.
- `TestServiceCreateObject_UnregisteredKind`: Rejects unknown kinds.
- `TestServiceListObjects_ByKind`: Filter by object kind.
- `TestServicePutEdge`: Create objects and add an edge between them.
- `TestServicePutEdge_UnregisteredKind`: Rejects unknown edge kinds.
- `TestServicePutEdge_MissingEndpoint`: Rejects edges to nonexistent objects.
- `TestServiceQueryObjects`: Query via structured filter.
- `TestServiceCrossStoreRouting`: Create objects in different stores (Dolt vs
  SQLite), retrieve both through the service, list across stores.

### Disposition: **KEEP — solid foundation**

The API surface, registry, store adapter pattern, and canonical ID scheme are
sound. The package is well-structured Go with clean interfaces and good test
coverage. Next steps: implement real Dolt SQL backing (current DoltStore may
need schema verification), wire into runtime, and connect to the Qdrant
indexing pipeline.

---

## 4. Qdrant Indexing Pipeline

**Worktree:** `~/.windsurf/worktrees/go-choir/go-choir-87c664e7`
**Branch:** `cascade/new-cascade-87c664` (snapshot at `b1b7b671`)
**Commits:** None (uncommitted working changes)

### Files Created

| File | Purpose |
|------|---------|
| `internal/qdrant/client.go` | REST API client for Qdrant |
| `internal/qdrant/schema.go` | Point payload schema and collection config |
| `internal/qdrant/naming.go` | Collection and alias naming conventions |
| `internal/qdrant/embed.go` | Embedder interface and HashEmbedder |
| `internal/qdrant/pipeline.go` | Shadow-collection build and alias switch |
| `internal/qdrant/samples.go` | 5 sample Choir objects |
| `internal/qdrant/qdrant_test.go` | Unit + integration tests |
| `cmd/qdrantctl/main.go` | CLI tool for running the pipeline |
| `docker-compose.qdrant.yml` | Local Qdrant instance |
| `docs/design-qdrant-indexing-pipeline-2026-06-23.md` | Design doc (142 lines) |

### Client

`client.go` implements a REST API client covering:

- `Health(ctx)` — health check
- `CreateCollection(ctx, name, config)` — create a collection with vector config
- `DeleteCollection(ctx, name)` — delete a collection
- `GetCollectionInfo(ctx, name)` — get point count and config
- `UpsertPoints(ctx, collection, points)` — insert/update points
- `Search(ctx, collection, vector, limit)` — vector search
- `CreateAlias(ctx, alias, collection)` — point an alias at a collection
- `UpdateAlias(ctx, alias, collection)` — atomically switch alias
- `ListAliases(ctx)` — list all aliases

### Naming Convention

```
Collection: choir_{owner_id}_{object_kind}_v{index_version}
Alias:      choir_{owner_id}_{object_kind}_v0_active
```

The alias drops the version suffix and appends `_active`, providing a stable
query target that always points to the current collection.

### Embedding

`embed.go` defines an `Embedder` interface:

```go
type Embedder interface {
    Embed(text string) ([]float32, error)
    Model() EmbeddingModel
}
```

`HashEmbedder` is a deterministic 384-dimensional embedder for prototype
testing. It is explicitly not semantically meaningful — it only verifies
pipeline mechanics (same text → same vector, different text → different
vector).

The design doc recommends `text-embedding-3-small` (1536 dim, $0.02/1M tokens)
for production, with a comparison table against `text-embedding-3-large` and
`bge-large-en-v1.5`.

### Pipeline

`pipeline.go` implements `BuildAndSwitch`:

1. Create shadow collection (`choir_{owner}_{kind}_v{version}`)
2. Embed and upsert all objects into the shadow collection
3. Verify: point count matches expected count
4. Atomically switch alias (`choir_{owner}_{kind}_v0_active`) → new collection
5. Return old collection name for rollback/GC

Also includes `GarbageCollectOldCollection` for cleanup after confidence window.

### Payload Schema

Every Qdrant point carries:

| Field | Type | Description |
|-------|------|-------------|
| `canonical_id` | string | Globally unique object ID from the graph |
| `object_kind` | string | Registered object type |
| `content_hash` | string | Hash of canonical serialized bytes |
| `owner_id` | string | Owning user or computer |
| `text` | string | Embedded text |
| `embedding_model` | string | Model name |
| `embedding_version` | string | Model version |
| `metadata` | JSON | Kind-specific metadata |

### Tests

`qdrant_test.go` includes:

**Unit tests (no Qdrant required):**
- `TestHashEmbedderDeterminism`: Same text → same vector.
- `TestHashEmbedderDifferentInputs`: Different text → different vector.
- `TestCollectionNaming`: Collection name format.
- `TestAliasName`: Alias name format.
- `TestSampleObjects`: All sample objects have required fields.

**Integration test (skips if Qdrant unavailable):**
- `TestPipelineBuildAndSwitch`: Full pipeline — create collection, upsert
  points, verify count, switch alias, verify alias points to new collection,
  run a second build to test alias switching, verify search works via alias.

### CLI

`cmd/qdrantctl/main.go` runs the full pipeline with flags:
- `--url` (Qdrant URL, default `http://localhost:6333`)
- `--owner` (default `user:alice`)
- `--kind` (default `choir.source_entity`)
- `--version` (default `1`)
- `--gc-old` (garbage-collect old collection after switch)

### Disposition: **KEEP — well-structured prototype**

The shadow-collection + alias-switch pattern is the right approach for
transactional index updates. The code is clean, well-tested, and properly
documented. The design doc is thorough with practical model recommendations.
Next steps: wire to the object service to read real objects from Dolt, add
payload filtering, and add batch embedding with rate limiting.

---

## 5. Source Entity Migration Design

**Worktree:** `~/.codex/worktrees/2bae/go-choir`
**Branch:** Detached HEAD at `27af9008`
**Commits:** None (uncommitted working changes)

### Files Changed

| File | Lines |
|------|-------|
| `docs/README.md` | +5/-1 |
| `docs/doc-authority-manifest.yaml` | +19 |
| `docs/mission-graph.yaml` | +15 |
| `docs/paradoc-source-entity-migration.md` | +2431/-50 |
| **Total** | **+2420/-50** |

### What It Contains

The paradoc was expanded from 75 lines to ~2,500 lines with a detailed
migration design covering:

**Schema design:**
- `choir.source_entity`: canonical object with `kind`, `display_title`,
  `display_url`, `canonical_url`, `owner_id`, content hash, metadata.
- `choir.source_ref`: edge object pinned to source entity version and Texture
  revision, keyed by deterministic body-node occurrence/path hashes.

**Migration mapping:**
- Maps `CoagentPacketSource` and `EvidenceRecord` types to the new object
  schema.
- Historical backfill materializes graph objects/refs from
  `source_entities_json` without rewriting historical revision bodies.

**Producer/consumer change sites:**
- Researcher agent: write source entity objects.
- Runtime: carry source entity objects across revisions.
- Texture agent: read and cite source entity objects.
- Frontend: render source refs.

**5-phase rollout:**
1. `legacy_only` — schema and store support, no behavior change.
2. `shadow_write` — write graph objects alongside legacy metadata.
3. `graph_read` — read from graph, keep legacy as fallback.
4. `enforce` — graph is primary, legacy is compatibility shim.
5. Deprecation — instrument and remove legacy paths.

**Rollback plan:** Version-pinned refs, per-revision backfill status, default-
off rollout.

**Supplemental probes:**
- Frontend `sourceOpenPlan` normalizes open-surface aliases but not
  source/target kind aliases.
- `TextureEditor` still opens by legacy `src_...` lookup.
- No frontend test/spec covers the source-opening path.
- Qdrant is a precise blocker for phase 5 (the local Qdrant pipeline paradoc
  is still open).

**Independent review infrastructure:**
- Review authorization/request/verdict capture templates.
- The mission is `blocked` pending owner-authorized independent review.

### Disposition: **LEARN — extract design, discard paradoc bloat**

The schema design, migration mapping, and phased rollout plan are valuable
reference material. However, the +2,431 lines in the paradoc file is bloated
with execution narrative, review-packet boilerplate, and supplemental probe
results. This should be extracted into a proper
`docs/design-source-entity-migration-2026-06-23.md` and the paradoc restored
to a concise mission control document.

No code was written. The design is the output.

---

## 6. PPTX Renderer Prototype

**Worktree:** `~/.windsurf/worktrees/go-choir/go-choir-f4fdeb09`
**Branch:** `cascade/new-cascade-f4fdeb` (snapshot at `42689a92`)
**Commits:** None (uncommitted working changes)

### Files Created

| File | Purpose |
|------|---------|
| `pptx-prototype/index.html` | Vite entry point |
| `pptx-prototype/vite.config.js` | Vite config |
| `pptx-prototype/package.json` | Dependencies: `pptxgenjs`, `@aiden0z/pptx-renderer` |
| `pptx-prototype/.gitignore` | Node artifacts |
| `pptx-prototype/README.md` | Setup instructions |
| `pptx-prototype/DESIGN.md` | Design notes (library choice, limitations, schema) |
| `pptx-prototype/src/main.js` | Svelte app bootstrap |
| `pptx-prototype/src/App.svelte` | Root component |
| `pptx-prototype/src/SlidesApp.svelte` | Slide rendering component |
| `pptx-prototype/src/mockDeck.js` | Mock `choir.slide_deck` object |
| `pptx-prototype/src/pptxGenerator.js` | PPTX binary generation via `pptxgenjs` |

### Library Decision

**Renderer:** `@aiden0z/pptx-renderer@^1.2.0`
- Browser-native PPTX renderer.
- Parses OOXML, builds a model, renders slides as HTML/SVG DOM.
- API: `PptxViewer.open(buffer, container, options)`.
- Supports `renderMode: 'slide'` and `renderMode: 'list'`.
- Size: ~1400 KB gzipped (ES module).
- Dependencies: `jszip`, `echarts`; optional `pdfjs-dist@^5` for EMF-PDF.
- Visual regression pipeline: 452+ cases, high fidelity.

**Writer:** `pptxgenjs@^3.12.0`
- Generates `.pptx` binary from a JSON object.
- Used in the prototype to create a sample PPTX from the mock deck.

**Alternatives rejected:**
- Custom parser in `SlidesApp.svelte` (current): too naive, loses layout.
- `pptxjs` / `pptx-parser`: older, lower fidelity.
- `pptxgenjs` alone: can write but not render.
- Server-side LibreOffice: adds infrastructure, less interactive.

### Mock Schema

The mock `choir.slide_deck` object includes:
- `kind`, `title`, `aspectRatio`, `theme`
- `slides[]`: each with `id`, `backgroundColor`, `elements[]`
- Elements: text boxes, bullet lists, shapes (rectangles), images
- Coordinate system: absolute pixels in the prototype; the design doc
  proposes normalized 0–1 coordinates for production

### Limitations Documented

Per the library README:
- Not supported: 3D effects, animations/transitions, equations (OMML), full
  EMF/WMF vector rendering, shadow/reflection/glow, embedded OLE, slide notes.
- PPTX input should be treated as untrusted; use `RECOMMENDED_ZIP_LIMITS`.

### Performance Notes

- Default mode is eager (parse whole deck, build model, render).
- For large decks: `lazySlides: true` (~52-66% model build time reduction),
  `lazyMedia: true` (~72-97% initial media byte reduction).

### Proposed Production Schema

```
Kind: choir.slide_deck
Schema: kind, title, aspectRatio, theme, masters[], slides[]
Each slide: id, master, layout, backgroundColor, elements[]
Each element: id, type, x, y, w, h (normalized 0-1), plus type-specific fields
Persistence: structured object in Dolt; optionally cache .pptx binary as attachment
Rendering: load object → PptxViewer.open(buffer, stageEl, { renderMode: 'slide', fitMode: 'contain' })
```

### Disposition: **THROWAWAY PROTOTYPE — extract design doc**

The code is a standalone sandbox app at the repo root, not integrated into
Choir's frontend. It proves the library works but is not production code.
Extract `DESIGN.md` to `docs/design-pptx-renderer-2026-06-23.md` and discard
the `pptx-prototype/` directory. The library decision and schema proposal are
the durable learnings.

---

## 7. Universal Wire Diagnosis

**Worktree:** `~/.codex/worktrees/1569/go-choir`
**Branch:** Detached HEAD at `27af9008`
**Commits:** None (uncommitted working changes)

### Files Changed

| File | Lines |
|------|-------|
| `docs/mission-graph.yaml` | +17 |
| `docs/paradoc-universal-wire-diagnosis.md` | +423/-13 |
| `docs/paradoc-universal-wire-diagnosis.ledger.md` | new (untracked) |
| **Total** | **+427/-13** |

### Diagnosis Findings

The mission investigated the reported `/api/universal-wire/stories` HTTP 502
through staging probes against `https://choir.news`.

**Probe results:**

1. **Unauthenticated request:** `GET /api/universal-wire/stories` returned
   `HTTP/2 401` with `{"error":"authentication required"}`. The route exists
   and is auth-gated. No 502.

2. **Platform sandbox direct:** Sandbox `/health` returned `200` and
   `status=ready`. Direct `/api/universal-wire/stories` without identity
   returned `401`. With diagnostic `X-Authenticated-User` injection, returned
   `200` with `{"stories":[],"style_sources":[],"source":"universal-wire-texture-index"}`.

3. **Authenticated browser session:** Registered a real passkey user
   (`bff2a19c-b584-4732-9fd0-16b82bdff617`). With valid session,
   `GET /api/universal-wire/stories` returned `200` in 128ms with empty
   stories array.

4. **Log inspection:** Proxy logs had no Universal Wire, 502, bad-gateway, or
   sandbox resolution matches. Sandbox logs had no Universal Wire
   errors. vmctl logs showed platform processor runs submitting and completing
   (liveness evidence).

**Conclusion:** The 502 was not reproduced. The route works. The stories list
is empty (no content has been ingested/processed yet). The 502 was likely
transient or already resolved.

### Web Capture Object Design

The paradoc proposes `choir.web_capture` object kind:

| Field | Type | Description |
|-------|------|-------------|
| `canonical_id` | string | `obj:choir.web_capture:<owner>:<hash>` |
| `url` | string | Original fetch URL |
| `canonical_url` | string | Normalized URL |
| `title` | string | Page title |
| `fetched_at` | timestamp | When the capture was made |
| `content_blob_id` | string | Blob ID of raw HTML |
| `extracted_text_blob_id` | string | Blob ID of extracted text |
| `embedding_model` | string | Embedding model used |
| `embedding_version` | string | Embedding model version |

### Feed Query Design

The feed should be a graph query over web capture objects, not a bespoke
pipeline. The existing sourcecycled pipeline can be adapted to write web
capture objects. The processor reads them and turns them into texture
documents.

### Verification Plan

A test that writes a web capture object and retrieves it through the feed
query.

### Disposition: **LEARN — extract findings, close 502 investigation**

The 502 is not reproducible; the route works and returns empty stories. The
`choir.web_capture` schema proposal is useful for the object graph. Extract
the key findings and schema into a proper design doc. The paradoc is bloated
with execution narrative (+423 lines) that should be distilled.

---

## Summary

### Disposition Classification

| # | Mission | Disposition | Code? | Land? |
|---|---------|-------------|-------|-------|
| 1 | Email Freeze Diagnosis | **KEEP** | Yes — production fix | Yes, via staging loop |
| 2 | Docs Checker Clear | **KEEP with cleanup** | Test workaround needs fix | Yes, after cleanup |
| 3 | Object Service Prototype | **KEEP** | Yes — Go package | Foundation for real service |
| 4 | Qdrant Indexing Pipeline | **KEEP** | Yes — Go package + CLI | Foundation for real pipeline |
| 5 | Source Entity Migration | **LEARN** | No — design only | Extract design doc |
| 6 | PPTX Renderer Prototype | **THROWAWAY** | Yes — standalone app | Extract design doc, discard code |
| 7 | Universal Wire Diagnosis | **LEARN** | No — diagnosis only | Extract findings, close 502 |

### Priority Actions

1. **Land the email freeze fix** (worktree #1). It's the most production-ready
   work. Run the staging landing loop: commit → push → CI → staging → verify.

2. **Commit the object service and Qdrant prototypes** (worktrees #3, #4) to
   preserve them before worktree cleanup. They're solid foundations to build
   on.

3. **Clean up the docs checker work** (worktree #2): fix the
   `model_policy_test.go` string-splitting workaround, then land the docs
   changes.

4. **Extract design docs** from worktrees #5, #6, #7:
   - `docs/design-source-entity-migration-2026-06-23.md`
   - `docs/design-pptx-renderer-2026-06-23.md`
   - `docs/design-universal-wire-diagnosis-2026-06-23.md`

5. **Discard the PPTX prototype code** (`pptx-prototype/` directory) after
   extracting the design doc.

6. **Commit or stash the detached-HEAD worktrees** (#5, #7) before they're
   cleaned up.

### Cross-Cutting Observations

- **Paradoc bloat:** Multiple paradoc files accumulated hundreds of lines of
  execution narrative. The paradoc should be a concise mission control
  document; detailed findings should live in separate design/evidence docs.

- **No integration between prototypes:** The object service and Qdrant
  pipeline are complementary but not yet wired together. The natural next
  step is connecting the Qdrant pipeline's `SampleObjects()` to read from the
  object service's `ListObjects()`.

- **Test coverage:** The email freeze fix and Qdrant pipeline have good
  tests. The object service has tests but they use `MemoryStore` only — Dolt
  and SQLite store adapters need integration tests. The docs checker has no
  test for the actual doccheck warning count reduction.

- **Evidence honesty:** The email freeze and universal wire diagnoses both
  honestly bound their evidence — the hard freeze and the 502 were not
  reproduced. This is good practice and should be maintained.
