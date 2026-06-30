# Mission: Store Consolidation, Host Sandbox Deletion, and corpusd Rename — Ledger

Append-only Parallax mission ledger. State lives in
`docs/mission-store-consolidation-v0.md`; this file is written every pass,
never re-read in full.

---

## PASS 0 — 2026-06-30 — Mission creation

**Move:** construct (mission document + ledger + graph node + beads sync)
**Conjecture:** The mission conjecture is sound: consolidating to two stores
(corpusd + per-VM embedded Dolt), deleting the host sandbox, and renaming
platformd → corpusd will advance the deeper goal of a small, simple
architecture that enables tranche-based synthesis.
**Verdict:** proposed (no evidence yet — planning phase)
**Expected ΔV:** 0 (no conjectures decided, mission just created)
**Actual ΔV:** 0
**Receipt:** `docs/mission-store-consolidation-v0.md` created,
`docs/plan-store-consolidation-and-host-sandbox-deletion-v0.md` created,
cognitive transforms run (Sequential Thinking, 13 thoughts),
mission graph + beads sync pending.

**Position:** Planning complete. Architecture fully traced:
- Platform Dolt SQL server at `127.0.0.1:13306` (corpusd) — stays, becomes the
  single object store
- Embedded Dolt per VM — stays, private runtime state only
- Embedded objectgraph (`internal/objectgraph/`) — DELETE, move to corpusd
- Sourcecycled SQLite (`internal/cycle/storage.go`) — DELETE, move to corpusd
- Host sandbox process (`go-choir-sandbox.service`) — DELETE, route through vmctl
- Qdrant — move to host (platform-level vector store)
- platformd → corpusd rename

**Cognitive transform Results:**
- Inversion: confirmed corpusd is the right home (VM fragility vs host stability)
- Failure mode: corpusd downtime is visible failure > current silent failure
- Boundary: publish boundary is clean (corpusd = platform, embedded Dolt = private)
- Network topology: HTTP API better than direct MySQL from VMs
- Scale: consolidation ENABLES tranche-based synthesis (cross-vertical queries)
- Maild: routes through vmctl, not host sandbox
- Qdrant: host-level for platform corpus, per-VM for private documents
- Landing page: permanent VM, horizontally scalable

**Open edges:**
- E1 (resource): corpusd is a single process — monitoring needed
- E2 (missing_oracle): no monitoring/alerting for corpusd yet
- E3 (frame_lock): processor concurrency fix deferred to post-consolidation

**Next move:** PR 1 — add object graph tables + API endpoints to corpusd

---

## PASS 1 — 2026-06-30 — PR 1: object graph tables + API on corpusd

**Move:** construct (add og_objects + og_edges tables to platform schema DDL,
ObjectGraphStore implementing objectgraph.Store over platform DB, HTTP API
endpoints for objects/edges CRUD, 8 tests)
**Conjecture:** C1 — corpusd can store object graph data with same query
semantics as the embedded objectgraph.
**Verdict:** supported (local evidence: 8 tests pass, all platform tests pass,
all objectgraph tests pass, go vet clean, CI green on PR #37)
**Expected ΔV:** -1 (C1 decided)
**Actual ΔV:** -1
**Receipt:** PR #37 (commit 881b70ad), CI run 28472014525 all green.
Files: `internal/platform/store.go` (schema DDL),
`internal/platform/objectgraph_store.go` (Store impl),
`internal/platform/objectgraph_handlers.go` (HTTP API),
`internal/platform/objectgraph_test.go` (8 tests),
`cmd/platformd/main.go` (wiring),
`internal/objectgraph/memory_store.go` (NormalizedLimit export),
`internal/objectgraph/{dolt_store,sqlite_store,service}.go` (rename).

**Position:** PR 1 implemented and CI green. The platform Dolt SQL server
now has og_objects and og_edges tables, an ObjectGraphStore that implements
the objectgraph.Store interface, and HTTP API endpoints at
/internal/platform/objects and /internal/platform/edges. The objectgraph
Service is wired in cmd/platformd/main.go with the platform store as the
durable backend.

**Branch hygiene note:** Initial PR #35 was opened off the wrong base
(mission/bootstrap-admin-api-key-v0, 24 commits ahead of main). Closed and
re-created as PR #37 off main (1 commit, 9 files, 833 insertions). Also
opened PR #36 (docs+beads infrastructure) as a separate PR off main.

**Open edges:** same as Pass 0 (E1-E3), plus:
- E4: PR #36 (docs+beads) needs to merge before PR #37 can merge cleanly
  (PR #37 doesn't depend on PR #36 code-wise, but the mission docs should
  be on main first)

**Next move:** Merge PR #36 then PR #37 to main. Then begin PR 2: rewrite
sourcecycled to use corpusd (delete SQLite).
