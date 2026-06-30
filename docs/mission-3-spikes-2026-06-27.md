# Mission 3 Spikes: Qdrant + Qwen3-Embedding + Dolt Object Graph

**Status:** ready for worker agent dispatch  
**Date:** 2026-06-27  
**Purpose:** gather implementation evidence before writing the Mission 3 paradoc  
**Meta:** these are bounded spikes, not platform behavior changes. No staging deploy required.  
**Desktop/Wails:** out of scope for now. All work targets web/node-b/local Go tests.

## Execution Order

```
Group A (parallel):
  Spike 1: Qdrant on node-b (NixOS config)
  Spike 2: Ollama Qwen3-Embedding Embedder (Go code)
  Spike 3: Dolt-backed objectgraph.Store (Go code)
  Spike 4: Dolt branch-per-VM workflow (script + Go test)

Group B (after Group A):
  Spike 5: Qdrant routing prototype (depends on Spike 1 + Spike 2)
```

## Shared Context for All Workers

- **Repo:** `/Users/wiz/go-choir`
- **Dev shell:** `nix develop -c <command>` for Go/Dolt work
- **Staging:** `https://choir.news` — do not deploy any spike
- **Mutation class:** yellow (tests, prototypes, config) unless noted
- **Protected surfaces:** no changes to `internal/runtime/`, `internal/store/`, `cmd/sourcecycled/`, or any deployed service
- **Existing Qdrant code:** `internal/qdrant/` — REST client, schema, pipeline, projection. All prototype-quality, already tested locally.
- **Existing objectgraph code:** `internal/objectgraph/` — `Store` interface, `SQLiteStore`, `Object`, `Edge`, `Service`. SQLite is the current store.
- **Existing Dolt integration:** `internal/store/store.go` — embedded Dolt workspace via `github.com/dolthub/driver`. Platform Dolt runs as `dolt sql-server` on node-b at `127.0.0.1:13306`.
- **ADR:** `docs/archive/adr-dolt-as-canonical-state.md` — Dolt is the default canonical store for durable product state.

---

## Spike 1: Qdrant on node-b (NixOS config)

**Mutation class:** orange (infrastructure config change on node-b)  
**Protected surfaces:** all existing node-b services must continue running  
**Rollback path:** revert the nix config change, rebuild, switch

### Objective

Enable Qdrant as a host service on node-b via the NixOS module
(`services.qdrant`). Verify it starts, responds to health checks, and is
reachable from the tap network that VMs use.

### Work Item

1. Read `nix/node-b.nix` to understand the existing service configuration pattern.
2. Add `services.qdrant` to the node-b NixOS config:
   ```nix
   services.qdrant = {
     enable = true;
     settings = {
       service = {
         host = "127.0.0.1";
         http_port = 6333;
         grpc_port = 6334;
       };
       storage = {
         storage_path = "/var/lib/qdrant/storage";
         snapshots_path = "/var/lib/qdrant/snapshots";
       };
       hsnw_index = {
         on_disk = true;
       };
       telemetry_disabled = true;
     };
   };
   ```
3. Build and switch the node-b configuration (or dispatch to operator if
   rebuild requires physical access).
4. Verify: `curl http://127.0.0.1:6333/healthz` returns healthy.
5. Verify: `curl http://127.0.0.1:6333/collections` returns `{"result":{"collections":[]}}`.
6. Create a test collection, upsert a point, search for it. Use the existing
   `internal/qdrant.Client` or raw curl.
7. Verify VM reachability: from a running VM, `curl http://172.X.0.1:6333/healthz`
   (the host tap IP). If Qdrant is bound to `127.0.0.1` only, determine whether
   VMs need it bound to the tap interface or whether a reverse proxy is needed.
   Report findings.

### Acceptance

- Qdrant running on node-b as a systemd service
- Health endpoint responding
- A test collection can be created, points upserted, and searched
- VM reachability documented (works directly or needs config adjustment)
- No existing node-b services broken

### Deliverables

- Nix config diff
- `curl` transcript or test output showing health, create, upsert, search
- VM reachability finding

---

## Spike 2: Ollama Qwen3-Embedding Embedder (Go code)

**Mutation class:** yellow (new code + tests, no runtime behavior change)  
**Protected surfaces:** no changes to existing `internal/qdrant/` files except additive new files  
**Rollback path:** delete the new file

### Objective

Implement the `qdrant.Embedder` interface using Ollama's HTTP API with the
`batiai/qwen3-embedding:0.6b` model. This replaces the deterministic hash
embedder used in tests with a real semantic embedding model.

### Work Item

1. Create `internal/qdrant/ollama_embedder.go` implementing `qdrant.Embedder`:
   ```go
   type OllamaEmbedder struct {
       baseURL string
       model   string
       client  *http.Client
   }

   func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder
   func (e *OllamaEmbedder) Model() EmbeddingModel  // {Name: "qwen3-embedding-0.6b", Version: "q8_0", Dimensions: 1024}
   func (e *OllamaEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error)
   ```
2. The Ollama API endpoint is `POST /api/embed` with body:
   ```json
   {"model": "batiai/qwen3-embedding:0.6b", "input": ["text1", "text2"]}
   ```
   Response:
   ```json
   {"model": "...", "embeddings": [[...], [...]]}
   ```
3. Create `internal/qdrant/ollama_embedder_test.go`:
   - Test that `Model()` returns correct metadata (dimensions=1024)
   - Test `EmbedTexts` with a mock HTTP server (no Ollama dependency required)
   - Test error handling: connection refused, malformed response, empty input
   - If Ollama is available locally (`localhost:11434`), add an integration test
     guarded by `if os.Getenv("OLLAMA_TEST") == ""` skip
4. Do not modify any existing files. Only add new files.

### Acceptance

- `OllamaEmbedder` implements `qdrant.Embedder` correctly
- Unit tests pass with mock HTTP server
- Model metadata is correct (1024 dimensions, qwen3-embedding-0.6b)
- No existing tests broken

### Deliverables

- `internal/qdrant/ollama_embedder.go`
- `internal/qdrant/ollama_embedder_test.go`
- Test output: `nix develop -c go test ./internal/qdrant/ -run OllamaEmbedder -v`

---

## Spike 3: Dolt-backed objectgraph.Store (Go code)

**Mutation class:** yellow (new code + tests, no runtime behavior change)  
**Protected surfaces:** no changes to `internal/objectgraph/sqlite_store.go`, `internal/objectgraph/store.go`, or `internal/objectgraph/service.go`  
**Rollback path:** delete the new file(s)

### Objective

Implement the `objectgraph.Store` interface backed by Dolt SQL instead of
SQLite. This validates that the object graph can migrate to Dolt as its
canonical store.

### Work Item

1. Read `internal/objectgraph/store.go` for the `Store` interface (6 methods).
2. Read `internal/objectgraph/sqlite_store.go` for the existing SQLite
   implementation and schema. The Dolt store should use the same table schema
   (`og_objects`, `og_edges`) adapted for Dolt SQL dialect.
3. Read `internal/store/store.go` lines 533-559 to see how the embedded Dolt
   workspace is opened via `github.com/dolthub/driver`.
4. Create `internal/objectgraph/dolt_store.go`:
   ```go
   type DoltStore struct {
       db *sql.DB
   }

   func NewDoltStore(db *sql.DB) (*DoltStore, error)
   ```
   - Use `database/sql` with the Dolt driver (`github.com/dolthub/driver`).
   - Create tables with `CREATE TABLE IF NOT EXISTS` using Dolt-compatible SQL.
   - Dolt uses MySQL dialect. Adapt the SQLite schema:
     - `TEXT` → `VARCHAR(...)` or `TEXT` (Dolt supports both)
     - `BLOB` stays `BLOB`
     - `INTEGER` for booleans stays `INT` or `BOOLEAN`
     - `ON CONFLICT` syntax may differ — use `INSERT ... ON DUPLICATE KEY UPDATE`
       (MySQL/Dolt syntax) instead of SQLite's `ON CONFLICT ... DO UPDATE`
   - Implement all 6 `Store` methods: `PutObject`, `GetObject`, `ListObjects`,
     `PutEdge`, `ListEdges`, `Close`.
5. Create `internal/objectgraph/dolt_store_test.go`:
   - Use an embedded Dolt workspace (temp directory) for testing, similar to
     how `internal/store` opens its workspace.
   - Test all 6 methods: put/get round-trip, list with filters, edges, tombstones.
   - Mirror the existing SQLite store test coverage.
   - If embedded Dolt is hard to bootstrap in a test, use a local Dolt sql-server
     on a random port. Document the approach.

### Acceptance

- `DoltStore` implements `objectgraph.Store` correctly
- All tests pass: `nix develop -c go test ./internal/objectgraph/ -run DoltStore -v`
- The existing SQLite tests still pass (no changes to them)
- Insert/list/get latency is comparable to SQLite for small datasets (report numbers)

### Deliverables

- `internal/objectgraph/dolt_store.go`
- `internal/objectgraph/dolt_store_test.go`
- Test output with latency measurements
- Notes on any Dolt SQL dialect differences encountered

---

## Spike 4: Dolt branch-per-VM workflow (script + Go test)

**Mutation class:** yellow (test-only, no runtime behavior change)  
**Protected surfaces:** none — this is a standalone experiment  
**Rollback path:** delete the test file

### Objective

Validate the Dolt branch/merge workflow for distributed object graph writes.
Worker VMs write to their own branches; a merge VM merges branches into main.
Measure merge latency and conflict behavior with append-mostly workloads.

### Work Item

1. Create `internal/objectgraph/dolt_branch_test.go` (or a standalone script in
   `scripts/` if a Go test is awkward for this).
2. Using an embedded Dolt workspace or a local Dolt sql-server:
   - Create a database with the `og_objects` table.
   - Create branch `worker-1` from `main`.
   - Create branch `worker-2` from `main`.
   - On `worker-1`: insert 50 objects with unique canonical IDs.
   - On `worker-2`: insert 50 different objects with unique canonical IDs.
   - Commit on both branches.
   - Merge `worker-1` into `main`. Record: success? conflicts? latency?
   - Merge `worker-2` into `main`. Record: success? conflicts? latency?
   - Verify `main` has all 100 objects.
3. Conflict test:
   - Create branch `worker-3` from `main`.
   - On `main`: update object X's metadata.
   - On `worker-3`: update the same object X's metadata to a different value.
   - Commit both.
   - Merge `worker-3` into `main`. Record: conflict? what kind? how to resolve?
4. Use Dolt SQL procedures:
   - `CALL DOLT_CHECKOUT('-b', 'worker-1')` to create branches
   - `CALL DOLT_COMMIT('-a', '-m', 'message')` to commit
   - `CALL DOLT_CHECKOUT('main')` to switch
   - `SELECT DOLT_MERGE('worker-1')` to merge
5. Measure and report:
   - Time per branch creation
   - Time per commit
   - Time per merge (with 50 objects, with 500 objects)
   - Conflict behavior for append-mostly (different PKs) vs same-row updates

### Acceptance

- Branch creation, commit, and merge all work via SQL procedures
- Append-mostly merges (different PKs) succeed with zero conflicts
- Same-row conflict behavior is documented
- Latency numbers are reported
- The "one branch per transaction" limitation is confirmed and the sequential
  merge workflow (write → commit → switch → merge → commit) is validated

### Deliverables

- Test file or script
- Output transcript showing branch/commit/merge operations
- Latency measurements
- Conflict behavior documentation

---

## Spike 5: Qdrant routing prototype (depends on Spike 1 + Spike 2)

**Mutation class:** yellow (test-only, no runtime behavior change)  
**Protected surfaces:** none — uses existing `internal/qdrant/` client code  
**Rollback path:** delete the test file  
**Prerequisites:** Spike 1 (Qdrant running on node-b) and Spike 2 (OllamaEmbedder)

### Objective

Validate that Qdrant similarity search can serve as the routing/dedup mechanism
for source captures. Embed news captures, upsert with VM ownership metadata,
and search with new captures to determine routing.

### Work Item

1. Create `internal/qdrant/routing_test.go` (integration test, skip if
   `QDRANT_TEST` env var is not set).
2. Using the `OllamaEmbedder` (Spike 2) and a Qdrant instance (Spike 1):
   - Create a collection `wire_captures` with 1024-dim cosine distance.
   - Embed 20-30 sample news headlines from different topics (politics, sports,
     tech, health). Use real news-like text, not lorem ipsum.
   - Upsert points with payload: `{canonical_id, object_kind: "choir.web_capture",
     text: "...", vm_owner: "vm-1"}` (first 10 on vm-1, next 10 on vm-2, etc.)
   - Search test 1: embed a new headline about the same topic as an existing
     cluster. Verify the nearest result is from the correct topic and the
     similarity score is high (>0.7? >0.8? document the actual threshold).
   - Search test 2: embed a headline about a completely different topic. Verify
     the nearest result has a low similarity score (<0.5? document actual).
   - Search test 3: embed a headline that is a paraphrase of an existing
     capture (same story, different source). Verify high similarity — this is
     the semantic dedup case.
   - Search test 4: embed a headline about a related but different story in the
     same topic area. Document the similarity score — this is the boundary case
     that determines the routing threshold.
3. Report the similarity score distribution:
   - Same story, different source: score range
   - Same topic, different story: score range
   - Different topic: score range
4. Propose a routing threshold: above = semantic dup → route to same VM; below
   = new story → route to least-loaded VM.

### Acceptance

- 20-30 captures embedded and upserted to Qdrant
- Similarity search returns correct nearest neighbors
- Score distribution documented for same-story / same-topic / different-topic
- Routing threshold proposed with evidence
- Semantic dedup is validated: paraphrased headlines score high

### Deliverables

- `internal/qdrant/routing_test.go`
- Test output with similarity scores
- Score distribution summary
- Proposed routing threshold with justification

---

## Post-Spike: Mission 3 Paradoc

After all spikes complete, the orchestrator (me) will:

1. Review all spike deliverables and evidence
2. Fine-tune the Mission 3 design based on real measurements
3. Draft the Mission 3 paradoc incorporating:
   - Confirmed Dolt branch-per-VM workflow
   - Confirmed Dolt-backed objectgraph.Store
   - Confirmed Qdrant routing threshold
   - Confirmed Ollama embedder integration
   - Confirmed Qdrant on node-b
4. Present the paradoc for owner review before any implementation begins

## Notes

- Desktop/Wails integration is explicitly out of scope for these spikes.
  Desktop auth is currently broken and will be addressed after Mission 3.
- No staging deployments. All work is local code, local tests, or node-b
  infrastructure config.
- Workers should use `nix develop -c` for all Go/Dolt work.
- Workers should report dirty paths, commits, test output, and any blockers.
