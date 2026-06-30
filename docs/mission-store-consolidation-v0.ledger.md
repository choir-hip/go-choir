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
