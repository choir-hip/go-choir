# Assessment: Overall System State — 2026-07-07

**Status:** assessment / evidence
**Method:** three parallel read-only codebase reviews (actor runtime + object
graph; universal wire pipeline; storage substrate inventory) plus live API
probes against choir.news via the choir CLI and curl.
**Prompted by:** owner's report that "the mental models in the code are at a
mismatch now" after ~a dozen refactors in as many days.
**Companions:** [design-choir-headless-surface-v0.md](./design-choir-headless-surface-v0.md),
[memo-choir-cli-trajectories-decode-wire-502-2026-07-06.md](./memo-choir-cli-trajectories-decode-wire-502-2026-07-06.md),
the in-flight Dolt-vs-SQLite evaluation (separate agent).

---

## Verdict In One Paragraph

The felt mismatch is real, and it is not primarily in the code — it is
between the code and the story the docs tell about the code. Three central
claims in the current mission framing are inverted or stale: the actor
runtime is *fully* wired (not "partially"), `internal/runtime` is the *live
application-logic layer* (not a "zombie"), and the object graph is *additive*
(not a "migration" of the data model). Meanwhile the wire's brokenness is not
wire-pipeline code at all — it is VM lifecycle plus a timeout mismatch in the
proxy/vmctl path. And the "audited computer" audit trail is application-level
tables and hash chains, not the Dolt version-control features the canonical
ADR was justified by. The system is in a stable intermediate state; the
dominant risks are narrative drift (doctrine: framing drift is doctrine
drift) and unscheduled deletion work, not missing functionality.

---

## 1. Actor Runtime / Object Graph — docs understate, code overdelivers

Full report retained in session evidence; key facts:

- **Actor runtime is the only execution substrate.** `internal/runtime`'s
  `dispatchActor` hook panics if nil — there is no legacy fallback path
  (internal/runtime/runtime.go:76-80). Cold-start, coagent wake, cancel,
  park-resume, and the tool loop all run through the actor
  (internal/actorruntime/handler.go, ExecuteActivationSync). H030 (mailbox
  polling) is repaired: warm delivery is a Go channel
  (internal/actor/actor.go:141, 247-275).
- **`internal/runtime` is not a zombie.** 148 files / ~106K LOC of live
  business logic (tool loops, texture state machine, wire synthesis, run
  memory). What remains is *extraction and deletion*, estimated 8–16 points
  (mission-3c paradoc), not wiring.
- **Object graph is additive, not replacive.** `internal/objectgraph/`
  (~2,233 LOC, ~28% test-to-source ratio) holds source entities, web
  captures, wire clusters. Canonical runs/tasks/trajectories/work items still
  live in `internal/store/`. Two data models coexist without conflict — and
  without a single canonical story.
- **Doctrine I5 (dual paths are bugs) is actively violated** by ~8–10 live
  dual paths with no deletion clocks: continuations (H006–H008,
  internal/store/continuations.go), parent/child residue (H001–H005,
  `ensureSpawnedCoagentWorkItem` at internal/runtime/runtime.go:759, 902,
  1342, 1435), texture forcing (H009–H024b, `next_required_tool` ×49).
- **Specs describe the target, not the system.** `actor_protocol.tla` and the
  rewritten `promotion_protocol.tla` model-check green in CI, but the live
  heresies (tool forcing, parent/child, continuations) are not modeled.

**Migration completeness estimates:** actor substrate 95%; wire wiring 70%;
object-graph integration 60%; business-logic extraction 0%; continuation
deletion 0%; parent/child deletion 5%; texture-forcing removal 0%.

## 2. Universal Wire — broken substrate, not broken pipeline

- **Confirmed root cause of the stories hang:** the proxy's
  `resolveSandboxURL` calls `vmctl.Client.ResolveDesktopContext` for the
  hard-coded platform computer (internal/proxy/handlers.go:970, :1045). The
  vmctl HTTP client timeout is **180s** (internal/vmctl/client.go:22); the
  proxy's transient-retry window is **10s** (handlers.go:46) — a hung VM is
  not "transient," so the request burns the full 180s with no response
  headers. The proxy's `http.Server` has **no Read/WriteTimeout**
  (internal/server/server.go:88-91). Staging metrics match exactly:
  api.resolve max 180,029ms, 23 errors, while corpusd-routed platform calls
  resolve in ~23ms. The wire *handler itself* returns immediately when the
  VM is up (internal/runtime/universal_wire.go).
- **Five wire generations traced; two fully deleted.** Gen 1 (deterministic
  scaffold) and Gen 2 (read-triggered repair) are gone per the heresy doc;
  Gen 3 (agent pipeline: processor → texture → publication), Gen 4
  (wirepublish eligibility), Gen 5 (web-capture graph synthesis) are active
  and CI-green (2026-07-03), with staging verification pending.
- **Implication: the next "wire redesign" should not be a wire redesign.**
  The pipeline code is one coherent generation now. What is missing is
  (a) request-path hardening — bounded vmctl resolve timeout (30–60s),
  server timeouts, 504-before-180s; (b) platform-VM lifecycle reliability;
  (c) one staging proof that articles publish end-to-end. If those land and
  stories still fail, *then* redesign.

## 3. Storage / Audited Computer — the audit trail is application-level

- **Dolt's version-control features are not load-bearing.** Zero uses of
  `AS OF`, branches, merge, diff, or `DOLT_LOG` in the tree. Dolt is used as
  a durable MySQL-compatible store plus per-mutation `DOLT_COMMIT()` markers
  (internal/platform/store.go:452-470, objectgraph_store.go:52,
  internal/cycle/storage.go:49-63).
- **Provenance comes from the application layer:** `parent_revision_id`
  chains, `author_kind`/`author_label` stamps, `superseded_by` pointers,
  SHA256-chained base journal, and fixture snapshots (`DoltHeadSnapshot`,
  `ObjectGraphSnapshot` via cmd/evidenceroot) for promotion evidence.
- **The ADR's rationale has drifted from usage.** The accepted ADR
  (docs/archive/adr-dolt-as-canonical-state.md, 2026-05-14) makes Dolt
  canonical for durable product state; the versioned-history rationale is
  not yet exercised by any read path. Dolt is also the sole source of
  cgo/ICU build friction (flake.nix:54-62; worker VMs cannot run Dolt tests
  with plain `go test`), while SQLite here is pure Go.
- **The real decision for the Dolt-vs-SQLite agent** is therefore a fork:
  (a) *commit to Dolt* — make `AS OF`/`DOLT_LOG` the audit read path so the
  audited-computer claim rests on the store's native history; or
  (b) *acknowledge the audit trail is application-level* — then the store
  choice is about build friction, per-write commit latency (one
  `DOLT_COMMIT` per object write is a likely throughput ceiling), and
  replication needs, where SQLite wins locally but has no sync story.
  SQLite substitution is feasible only with batch commits + the existing
  app-level version tables + application-level snapshots (all present).
- Ancillary stores: SQLite for auth/session (auth.db, WAL) and maild;
  Qdrant for embeddings (decoupled from causality); content-addressed blob
  store with manifests in Dolt; no store-abstraction layer exists — a
  substrate switch is a breaking change, not a config flip.

## 4. Headless Surface — the loop works; retrieval is hollow

Live probes against choir.news (2026-07-07, choir CLI + curl):

- **Try→prove loop verified end to end:** `run start` → conductor decision →
  appagent revision in ~10s → `texture revisions` content → trajectory
  visible. Fixed en route: trajectories decode bug (settlement_rule object),
  test isolation from `$CHOIR_API_KEY`, new `texture revisions` verb.
- **Undocumented capabilities found:** `GET /api/texture/documents` (no id)
  lists all docs — free `texture list` verb;
  `/api/texture/documents/{id}/stream` is a working SSE endpoint (snapshot
  events) — headless live-watch already exists; `/api/compute/status` is
  rich (roles, epochs, warmness, protection); `/api/pulse/summary` is
  privacy-bounded and public-safe; `/api/app-change-packages/pull` has a
  clean contract (`package_id is required`).
- **Search is hollow:** `/api/platform/retrieval/search` returns zero
  results for terms that verifiably exist in Texture documents ("choir",
  "universal wire"). Either retrieval ingestion of Texture docs is not
  wired or the index is empty. The "prove" pillar's retrieval half is a
  working door into an empty room.
- **No server-side paging:** `/api/trajectories` ignores `?limit=`; the
  CLI's client-side truncation at 50 is the only control.
- Auth hygiene is good: 401 with no route disclosure for bad/missing keys.

## 5. Synthesis — where the mismatch actually lives

The system's felt unsettledness decomposes into five named gaps:

1. **Narrative inversion (docs vs code).** The autoputer mission doc's three
   premises are each wrong in a direction that misdirects agents: it sends
   them to "finish wiring" (done) instead of "extract and delete" (not
   started). Per the Framing Doctrine, this is doctrine drift with teeth.
2. **Two data models, no declared relationship.** `internal/store/` is
   canonical for runtime state; objectgraph is canonical for source/wire
   state. Neither the doctrine nor the specs say which one the "object
   graph" future belongs to, or on what clock.
3. **Substrate vs pipeline misattribution.** A dozen wire refactors targeted
   the pipeline; the outage is VM lifecycle + timeouts. The pipeline is now
   one generation and CI-green — the remaining work is boring hardening.
4. **Audit-story drift.** "Audited computer" is true, but by application
   tables and hash chains, not by the store's native history. The ADR's
   stated rationale and the code's actual reliance have diverged; the
   storage decision should resolve that fork explicitly.
5. **Deletion debt without clocks.** ~8–10 named heresies live with no
   deletion schedule. Doctrine says discovery is progress and this repo is
   good at discovery; the backlog is in `repaired`.

## 6. Recommended ordering (for owner decision, not self-executed)

1. **Harden the proxy/vmctl request path** (bounded resolve timeout, server
   timeouts, fast 504) — small, spec-independent, converts every future VM
   outage from a 180s mystery hang into a legible error. Also unblocks
   honest wire verification.
2. **Correct the mission-suite premises** in
   mission-suite-autoputer-autopaper-spec-first-v0.md (or a successor memo)
   so the next agents optimize extraction/deletion, not re-wiring.
3. **Resolve the storage fork** (Dolt-native audit vs application-level
   audit) — the other agent's Dolt/SQLite evaluation should answer the six
   open questions in the storage inventory, starting with per-write commit
   semantics and rollback mechanics.
4. **Put deletion clocks on the top heresy clusters** (continuations,
   parent/child, texture forcing) per doctrine I5.
5. **Wire retrieval ingestion** (search returning zero over existing docs)
   — the audited computer should be able to find its own evidence.
6. **Then** Phase 1.5 CLI verbs and the MCP, per the headless-surface design
   doc — the surface should narrate a system whose story is settled.
