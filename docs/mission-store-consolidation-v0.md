# Mission: Store Consolidation, Host Sandbox Deletion, and corpusd Rename

**Date:** 2026-06-30
**Status:** planning — Parallax mission, pre-execution
**Authority:** user directive — aggressive deletion, no fallbacks, smaller codebase, pre-launch
**Sources:**
- `docs/computer-ontology.md`
- `docs/naming-rectification-2026-06-27.md`
- `docs/vision-choir-category-texture-transclusion-v0.md`
- `docs/choir-architecture-review-next-moves-2026-06-11.md`
- `docs/production-readiness-checklist.md`
- `docs/plan-store-consolidation-and-host-sandbox-deletion-v0.md`

## Mission Conjecture

If witness A (consolidated two-store architecture: platform Dolt SQL server
+ per-VM embedded Dolt, with host sandbox deleted and platformd renamed to
corpusd) satisfies spec S (all platform-level data in corpusd, all private VM
state in embedded Dolt, no SQLite sidecars, no embedded objectgraph, no host
sandbox process, no fallbacks) under invariants I (no silent failures, visible
corpusd downtime, pre-launch data is not precious, user VM QA account
preserved) and quality Q (sourcecycled writes to corpusd, runtime queries
corpusd for object graph, articles appear on wire, maild routes through vmctl,
proxy serves landing page via permanent VM) over domain D (staging deployment
on choir.news, platform VM + user VM + host services), then deeper goal G
(the codebase is small enough to sustain high-pace iteration, the architecture
is simple enough to reason about, and the source processing pipeline can scale
to tranche-based synthesis with cross-referencing) is achieved or materially
advanced.

### Deeper Goal (G)

A small, simple, two-store architecture that enables:
1. High-pace iteration (less code = faster changes)
2. Tranche-based synthesis (all sources in one queryable store = cross-vertical queries)
3. No silent failures (corpusd downtime is visible, not hidden)
4. Clean ownership boundaries (platform data in corpusd, private state in VMs)

### Witness/Spec (A/S)

**Witness:** The consolidated architecture:
- **corpusd** (renamed from platformd): host-level Dolt SQL server at
  `127.0.0.1:13306`, database `corpus`. The single object store for all
  platform-level data: published articles, source captures, web captures,
  edges, source items, cycle events, processor requests, provenance,
  publication lifecycle. Accessed via HTTP API by VMs and sourcecycled.
- **Embedded Dolt per VM**: each VM has its own embedded Dolt workspace for
  private runtime state: run records, agent records, channels, events,
  Texture drafts (pre-publication). No object graph.
- **Qdrant on host**: platform-level vector store for published articles,
  accessible to all VMs. Per-VM Qdrant only for private documents.
- **No host sandbox process**: all runtime work happens in VMs. Maild routes
  through vmctl. Proxy WS routes through vmctl. Landing page served by a
  permanent VM.
- **No SQLite sidecars**: sourcecycled writes to corpusd, not SQLite.
- **No embedded objectgraph**: deleted entirely.

**Spec:** 6 PRs, sequentially deployable:
1. Add object graph tables + API endpoints to corpusd
2. Rewrite sourcecycled to use corpusd (delete SQLite)
3. Rewrite runtime to query corpusd (delete embedded objectgraph)
4. Delete `internal/objectgraph/` package + clean up tests
5. Delete host sandbox + fix maild/proxy WS/landing page entanglements
6. Rename platformd → corpusd

### Invariants / Qualities / Domain Ramp (I/Q/D)

**Invariants (never violate):**
- I1: No silent failures. corpusd downtime is visible and loud.
- I2: User VM (`yusefnathanson@me.com`) data is preserved. No data loss for
  the QA account.
- I3: No fallbacks. If corpusd is down, the pipeline stops, not silently degrades.
- I4: Pre-launch data is not precious. Aggressive deletion is acceptable.
- I5: Each PR deploys independently and is verifiable on staging.

**Qualities:**
- Q1: Sourcecycled writes all source items, cycle events, and processor
  requests to corpusd (not SQLite).
- Q2: Runtime queries corpusd HTTP API for object graph data (not embedded Dolt).
- Q3: Articles appear on the Universal Wire after the consolidation.
- Q4: Maild routes through vmctl to user VMs (not host sandbox).
- Q5: Proxy serves landing page via a permanent VM (not host sandbox).
- Q6: Qdrant runs on the host, accessible to all VMs.
- Q7: `internal/objectgraph/` package is deleted.
- Q8: Host sandbox systemd service is deleted.
- Q9: `platformd` is renamed to `corpusd` throughout code, config, and docs.

**Domain Ramp:**
- D0: Local tests pass (unit + integration)
- D1: Staging: corpusd has object graph tables, sourcecycled writes to corpusd
- D2: Staging: runtime queries corpusd, articles appear on wire
- D3: Staging: host sandbox deleted, maild/proxy/landing page work
- D4: Staging: full rename, all services running with corpusd names

### Variant (Conjecture Descent) V

V = driving conjectures still undecided
  + conjectures whose evidence class is below the settlement tier
  + conjectures with no strong definitive statement yet recorded

**Initial conjectures (7):**
- C1: corpusd can store object graph data (web captures, source entities, edges)
  with the same query semantics as the embedded objectgraph
- C2: sourcecycled can use corpusd (MySQL) instead of SQLite with acceptable
  performance and atomic transactions
- C3: The runtime can query corpusd HTTP API for object graph data with
  acceptable latency for wire stories and synthesis
- C4: Maild can route through vmctl to user VMs instead of the host sandbox
- C5: A permanent VM can serve the landing page with acceptable cold-start
  and availability
- C6: Qdrant on the host is accessible to all VMs with acceptable latency
- C7: The 6-PR sequence is independently deployable without breaking staging

**V = 7** (all conjectures undecided)

### Budget

- Passes: ~20-30 (6 PRs, ~3-5 passes each)
- Wall-clock: multiple sessions
- Authority: user directive for aggressive deletion, no fallbacks

### Authority / Bounds

- Aggressive deletion is authorized (pre-launch, data not precious)
- User VM QA account (`yusefnathanson@me.com`) must be preserved
- No fallbacks — failures must be visible
- Multiple PRs — scope and document end-to-end now, implement in sequence
- corpusd rename is part of this mission

### Mutation Class / Protected Surfaces

- **Class:** red — platform architecture, data stores, service deletion
- **Protected surfaces:**
  - User VM data (yusefnathanson@me.com)
  - Universal Wire publication flow
  - Maild email delivery
  - Proxy routing
  - Gateway credential issuance
  - Sourcecycled source ingestion

### Evidence Packet

- Commits + PRs for each step
- Staging deploy identity (commit SHA)
- Health checks: corpusd, sourcecycled, platform VM, user VM, maild, proxy
- Wire stories endpoint returns articles
- Sourcecycled logs show corpusd writes
- Runtime logs show corpusd queries
- No host sandbox process running
- Rollback refs for each PR

### Heresy Delta

- **Discovered:** embedded objectgraph is a separate Dolt workspace from the
  main store (architectural heresy — should be one store). Sourcecycled SQLite
  is a separate database for the same data domain. Host sandbox is a bootstrap
  artifact that's still entangled with maild and proxy WS.
- **Introduced:** none yet (planning phase)
- **Repaired:** (target) all three heresies above

### Position / Live Conjectures / Open Edges

**Position:** Planning phase. Architecture mapped. Cognitive transforms run
(inversion, failure mode, boundary, network topology, scale, transaction,
host sandbox deletion, maild entanglement, Qdrant placement). Plan written.
Ready to create mission set and begin execution.

**Live conjectures:** C1-C7 (all undecided, see Variant)

**Open edges:**
- E1 (resource): corpusd (Dolt SQL server) is a single process. If it crashes,
  the entire news pipeline stops. Monitoring needed (see production checklist).
- E2 (missing_oracle): no monitoring/alerting for corpusd downtime yet.
- E3 (frame_lock): the processor concurrency limit (MAX_PROCESSORS=1) is
  artificial but should not be fixed on the old architecture. Fix after
  consolidation, pegged to 80% of real capacity on the new architecture.

### Next Move

Create the mission set (this paradoc + ledger + mission graph update + beads
sync), then begin PR 1: add object graph tables + API endpoints to corpusd.

### Ledger File

`docs/mission-store-consolidation-v0.ledger.md`

### Version / Lineage

v0, 2026-06-30. Supersedes the planning section of
`docs/plan-store-consolidation-and-host-sandbox-deletion-v0.md` (which becomes
a source doc).

### Learning State

Retained here. Will promote outward to:
- `docs/computer-ontology.md` (two-store architecture confirmed)
- `docs/naming-rectification-2026-06-27.md` (corpusd rename executed)
- `docs/production-readiness-checklist.md` (corpusd monitoring items)
- `docs/choir-architecture-review-next-moves-2026-06-11.md` (C/M11 updated)

### Settlement

Settled when: all 6 PRs merged, deployed to staging, articles appear on wire,
no host sandbox process, corpusd renamed, user VM preserved. Each conjecture
C1-C7 decided with typed verdict.

---

## Suggested Goal String

```text
Use Parallax on docs/mission-store-consolidation-v0.md. Mission: consolidate
to two stores (platform Dolt SQL server as corpusd + per-VM embedded Dolt),
delete embedded objectgraph, delete sourcecycled SQLite, delete host sandbox
process, move Qdrant to host, rename platformd to corpusd. 6 PRs, sequentially
deployable. Variant V=7 (C1-C7 undecided). Budget: ~20-30 passes. Authority:
aggressive deletion, no fallbacks, pre-launch data not precious, preserve
yusefnathanson@me.com QA VM. Invariants: no silent failures, visible corpusd
downtime. First move: PR 1 — add object graph tables + API endpoints to
corpusd. Ledger: docs/mission-store-consolidation-v0.ledger.md. Settlement:
all 6 PRs merged, deployed, articles on wire, no host sandbox, corpusd renamed.
```

---

## PR Sequence

### PR 1: Add object graph tables + API to corpusd

Add web capture, source entity, edge, source item, cycle event, and processor
request tables to the platform Dolt database. Add HTTP API endpoints to
corpusd for: web capture projection (POST), source item listing (GET), object/
edge queries (GET), processor request queue (GET/POST/PUT).

**Decides:** C1 (corpusd can store object graph data)

### PR 2: Rewrite sourcecycled to use corpusd

Replace `cycle.NewStorage()` (SQLite) with a corpusd-backed store. Source
items, cycle events, processor requests, and poll state all go to corpusd.
Delete the SQLite store.

**Decides:** C2 (sourcecycled can use corpusd instead of SQLite)

### PR 3: Rewrite runtime to query corpusd

Update `sourcecycled_web_captures.go`, `universal_wire.go`, and
`wire_synthesis.go` to query corpusd HTTP API instead of the embedded
objectgraph. The runtime's `ObjectGraph()` method becomes a corpusd client.

**Decides:** C3 (runtime can query corpusd with acceptable latency)

### PR 4: Delete objectgraph package

Delete `internal/objectgraph/` (DoltStore, SQLiteStore, MemoryStore, Service,
Store interface). Delete `internal/runtime/objectgraph_runtime.go`. Delete
`internal/sourcegraph/`. Clean up all tests that mock the objectgraph. Delete
the `objectgraph-dolt/` directory on the platform VM (frees 91% full disk).

**Decides:** C1 (confirmed — no regression after deletion)

### PR 5: Delete host sandbox + fix entanglements

Delete `go-choir-sandbox.service` from nix/node-b.nix. Update proxy to route
WS through vmctl (not `127.0.0.1:8085`). Update maild to route through vmctl.
Create a permanent VM for the landing page. Move Qdrant to the host.

**Decides:** C4 (maild routes through vmctl), C5 (permanent VM serves landing
page), C6 (Qdrant on host accessible to VMs)

### PR 6: Rename platformd to corpusd

Rename `cmd/platformd/` → `cmd/corpusd/`, `internal/platform/` →
`internal/corpus/`, all env vars (`PLATFORMD_*` → `CORPUSD_*`), Nix service
names, and docs. Rename the Dolt database from `platform` to `corpus`.

**Decides:** C7 (full sequence deployable without breaking staging)

---

## References

- `docs/plan-store-consolidation-and-host-sandbox-deletion-v0.md` — detailed
  deletion/migration plan (source doc)
- `docs/computer-ontology.md` — computer/VM/Dolt architecture
- `docs/naming-rectification-2026-06-27.md` — platformd → corpusd rename plan
- `docs/vision-choir-category-texture-transclusion-v0.md` — corpus as target term
- `docs/choir-architecture-review-next-moves-2026-06-11.md` — architecture review
- `docs/production-readiness-checklist.md` — monitoring/alerting items
- `internal/platform/store.go` — corpusd store (Dolt SQL server)
- `internal/objectgraph/` — embedded objectgraph (to be deleted)
- `internal/cycle/storage.go` — sourcecycled SQLite store (to be deleted)
- `nix/node-b.nix` — service configuration
