# Naming Rectification Plan

**Date:** 2026-06-27
**Status:** planning — comprehensive audit of all object/concept/process/system names
**Harness:** OMP (oh-my-pi) with ast-grep for AST-aware renaming
**Tool:** `ast-grep` (installed at `/opt/homebrew/bin/ast-grep`, alias `sg`)
**Vision:** `docs/vision-choir-category-texture-transclusion-v0.md` — the vision
defines the target vocabulary: textures, transclusions, autopapers, editions,
algorithms, styleguides, renderers, schedules. Naming should converge toward
the vision's terms, not away from them.

## Why

Choir's vocabulary has accumulated through multiple architectural iterations.
Names from previous iterations persist in code, docs, and contracts long after
the concepts they describe were renamed, deleted, or superseded. This causes:
- **Agent confusion:** new agents read old docs and copy retired vocabulary
  (e.g., VText re-emerged in mission-3c_2 docs after being renamed to Texture)
- **Name collisions:** `sourcegraph` (the package) vs `texture_source_graph` (the
  store concept) vs `objectgraph` (the substrate) — three "graph" concepts, two
  dealing with sources
- **Ontology drift:** `sandbox` is an implementation service name but docs use it
  as if it were the product object (which is `computer`)
- **Heresy propagation:** the doctrine bans certain vocabulary but code and docs
  still use it

## Method

Each renaming uses the OMP harness with ast-grep for AST-aware replacement:
1. **Audit:** grep + ast-grep to find all occurrences (code, docs, tests, configs)
2. **Classify:** code symbols vs docs vs test fixtures vs historical evidence
3. **AST rename:** use ast-grep for Go code (handles imports, type refs, method
   calls, struct fields), `sed`/manual for docs and configs
4. **Verify:** `go build ./...`, `go test -race ./...`, `nix develop -c go vet`
5. **Docs pass:** update all docs except those with `texture-cutover-allow` or
   equivalent historical-preservation markers
6. **Commit:** one commit per rename, with before/after line counts

## Renaming Targets

### 1. `sandbox` → `computer` (product ontology)

**Current:** `cmd/sandbox/`, `internal/sandbox/`, 212 non-test files reference
"sandbox" in code and docs.

**Problem:** The product object is a **computer** — a persistent user-owned
execution environment. `sandbox` is an implementation service name that leaked
into the product ontology. The computer ontology doc says: "Use **sandbox** only
for existing service/process names or legacy references." But the code uses it
everywhere as the primary name.

**Target:**
- `cmd/sandbox/` → `cmd/computer/` (or `cmd/autoputer/` — see below)
- `internal/sandbox/` → `internal/computer/` (or fold into runtime/actorruntime)
- Product docs: `sandbox` → `computer` (except where explicitly referring to the
  implementation service)
- Code symbols: `Sandbox*` types, `sandbox*` functions → `Computer*`

**Note:** The user has mentioned `sandbox → autoputer` as a possible name. This
needs a decision: `computer` (doctrine term) vs `autoputer` (new coinage). The
doctrine already says `computer` is the product term. `autoputer` might be the
service/binary name (like `sourcecycled`, `platformd`).

### 2. `platformd` → `corpusd` (service rename)

**Current:** `cmd/platformd/`, `internal/platform/`, referenced across proxy,
provider, runtime.

**Problem:** The rearchitecture doc planned `platformd → corpusd` as a side PR.
The service publishes Texture articles to the platform. `corpusd` better
describes what it does — managing the published corpus.

**Target:**
- `cmd/platformd/` → `cmd/corpusd/`
- `internal/platform/` → `internal/corpus/`
- Code symbols: `Platform*` → `Corpus*`, `platformd` → `corpusd`
- Config/env vars: `RUNTIME_PLATFORMD_URL` → `RUNTIME_CORPUSD_URL`,
  `PROXY_PLATFORMD_URL` → `PROXY_CORPUSD_URL`
- Docs: `platformd` → `corpusd`

### 3. `internal/sourcegraph/` → fold into `internal/cycle/` (delete package)

**Current:** `internal/sourcegraph/web_capture_graph.go` (170 lines) + 
`internal/cycle/web_capture_graph.go` (19-line wrapper).

**Problem:** Name collision with `internal/store/texture_source_graph.go` (the
Texture source graph). `sourcegraph` is a single-file package that projects
sourcecycled items into objectgraph web captures — it's a cycle-stage
projection, not a separate package.

**Target:**
- Inline `sourcegraph/web_capture_graph.go` into `cycle/web_capture_graph.go`
- Delete `internal/sourcegraph/` package
- Update callers: `runtime/sourcecycled_web_captures.go` imports `cycle` instead
  of `sourcegraph`

### 4. `internal/store/texture_source_graph.go` → `texture_source_store.go`

**Current:** `internal/store/texture_source_graph.go` (676 lines).

**Problem:** The name `texture_source_graph` collides with `sourcegraph` and
`objectgraph`. It's a store-layer persistence concern (pinned source records,
source ref edges in Dolt), not a graph package.

**Target:**
- Rename file: `texture_source_graph.go` → `texture_source_store.go`
- Rename types: `TextureSourceGraphWriteSet` → `TextureSourceWriteSet`
- Rename functions: `CreateRevisionWithSourceGraph` → `CreateRevisionWithSources`
- Rename tests: `TestTextureSourceGraph*` → `TestTextureSourceStore*`

### 5. Retired vocabulary cleanup (ongoing, PR #7)

**Current:** 975 doc warnings from retired vocabulary. PR #7 (docs checker)
addresses the bulk. Additional terms to verify:

| Retired term | Replacement | Status |
|--------------|-------------|--------|
| `vtext` / `VText` | `texture` / `Texture` | Code clean, docs mostly clean (PR #7) |
| `continuation` / `RunContinuation` | `trajectory` / `work item` | Code has residue (H006-H008) |
| `continuation-level` | `retired` (mark as retired) | PR #7 |
| `parent_id` / `ParentRunID` | `trajectory_id` / `work_item_id` | Code has residue (H001-H005) |
| `spawned_child` | `work_item` | Code has residue (H005) |
| `lease` | `assignment` / `activation` | Code has residue (H019) |
| `channel` (delivery) | `mailbox` | Actor runtime uses mailbox; old code uses channel |
| `Trace app` / `Trace UI` | deleted (evidence substrate only) | H027, deletion planned |
| `Terminal app` | `Super Console` / `zot` | H028 |
| `Browser app` / `Web Lens` (as source gatherer) | `Source Viewer` + `Web Lens` (live inspection only) | H029 |

### 6. `channel` → `mailbox` (actor protocol)

**Current:** `internal/actor/actor.go` uses `mailbox` correctly. But
`internal/runtime/` still has `channel` references (e.g., `channel_store.go`,
`Channel*` types) from the old concurrency model.

**Problem:** `channel` is overloaded — it means both Go channels (concurrency
primitive) and the old delivery concept. The actor runtime uses `mailbox` for
delivery. The old `channel` concept is deleted (Phase 2a deleted `channels.go`),
but `channel_store.go` and `Channel*` types remain.

**Target:**
- `channel_store.go` → `mailbox_store.go` (or fold into actor runtime)
- `Channel*` types → `Mailbox*` types
- Docs: `channel` (delivery sense) → `mailbox`

### 7. Other candidates to audit

These need investigation — may or may not need renaming:

- **`vmctl`** — VM control service. Name is implementation-level. Should it
  become `computerctl` or stay as the implementation tool name?
- **`vmmanager`** — same question
- **`gateway` / `gatewayruntime`** — is this still the right name?
- **`wirepublish`** — should this fold into `internal/wire/` during extraction?
- **`sourceapi` / `sourcecontract` / `sourcefetch` / `sources`** — four source
  packages. Can any be consolidated?
- **`texturedoc`** — separate from `runtime/texture*.go`. Should it fold into
  `internal/texture/` during extraction?
- **`agentprofile` / `toolregistry` / `provider` / `provideriface`** — four
  packages for agent configuration. Can any be consolidated?
- **`server`** — what is this vs `runtime` vs `actorruntime`?
- **`persistentdisk`** — is this still used?
- **`markdownstructure`** — is this still used?
- **`buildinfo`** — is this still used?

### 8. Vision-aligned vocabulary (new — from the texture/transclusion vision)

The vision doc (`docs/vision-choir-category-texture-transclusion-v0.md`)
defines the target product vocabulary. These terms don't all need code renames
now (most don't exist in code yet), but the naming plan should track them so
future code converges on the right terms:

| Vision term | Current code term | Status |
|-------------|-------------------|--------|
| `texture` | `texture` / `Texture` | Aligned ✓ |
| `transclusion` | `transclusion` / `source_ref` | Partial — `source_ref` is the storage-level term for a transclusion edge |
| `autopaper` | (does not exist yet) | New concept — a texture that transcludes algorithm + styleguide + schedule + renderer + context |
| `edition` | `Wire.texture` (singleton) | The vision replaces the singleton with per-cycle edition textures |
| `algorithm texture` | (does not exist yet) | New — what to cover |
| `styleguide texture` | (does not exist yet) | New — how to write (style implies substance) |
| `renderer texture` | `UniversalWireApp.svelte` (hardcoded) | The vision makes the renderer a transcluded texture containing JS/CSS |
| `schedule texture` | (does not exist yet) | New — when to publish |
| `context texture` | (does not exist yet) | New — background, previous coverage, entity profiles |
| `cycle` | `cycle` | Aligned ✓ — the vision confirms cycle as the fundamental tick |
| `corpus` | `platform` / `platformd` | Aligns with rename #2 (`platformd → corpusd`) — the vision's Publish functor maps to Corpus |
| `computer` | `sandbox` | Aligns with rename #1 (`sandbox → computer`) — the vision's user VM is a computer |

**Action:** No immediate renames. Track these terms. When the vision's
implementation missions land, use these terms in new code and docs. The
`source_ref` → `transclusion` rename is a candidate for early alignment but
should wait until the wire API rewrite (vision step 3) to avoid churn.

## Sequencing

Renamings should happen **after** the runtime refactor and app extraction, not
during. Reasons:
1. The extraction will move code between packages — renaming first means
   renaming code that's about to move
2. ast-grep renames are easier on stable code (no concurrent structural changes)
3. Some renames depend on extraction outcomes (e.g., `sourcegraph` folds into
   `cycle`, but `cycle` may change during extraction)

**Exception:** The `sourcegraph` fold (#3) and `texture_source_graph` rename
(#4) can happen now — they're small, self-contained, and reduce confusion
immediately. These are green/yellow mutation class.

**Exception:** PR #7 (retired vocabulary cleanup) is already in flight.

Everything else waits for post-extraction.

## Process For Each Rename

```
1. Audit:    ast-grep + grep to find all occurrences
2. Plan:     list every file, every symbol, every doc reference
3. Rename:   ast-grep for Go code, manual for docs/configs
4. Build:    nix develop -c go build ./...
5. Test:     nix develop -c go test -race ./...
6. Vet:      nix develop -c go vet ./...
7. Docs:     update all docs (except historical-preservation markers)
8. Commit:   one commit, with before/after stats
9. Verify:   no remaining occurrences (grep confirms clean)
```

## ast-grep Usage

ast-grep (alias `sg`) is installed at `/opt/homebrew/bin/ast-grep`.

For Go renaming:
```bash
# Find all references to a type
sg run --lang go --pattern 'TextureSourceGraphWriteSet' --json

# Rename a type
sg run --lang go --pattern 'TextureSourceGraphWriteSet' --rewrite 'TextureSourceWriteSet' --in-place

# Rename a function call
sg run --lang go --pattern 'CreateRevisionWithSourceGraph($$$ARGS)' --rewrite 'CreateRevisionWithSources($$$ARGS)' --in-place
```

For package renames, combine ast-grep (for code symbols) with `go mod edit` (for
import paths) and `mv` (for directory moves).
