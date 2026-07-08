# Mission: Object-Graph Hard Cutover, Dolt All-In, Heresy Elimination — v0

**Status:** paradoc / owner-approved direction, sequencing proposed
**Date:** 2026-07-07
**Decisions (owner, 2026-07-07):** (1) the object graph becomes the canonical
data model by hard cutover, not accretion; (2) Dolt's version-control features
become load-bearing; (3) all named heresies are eliminated and doctrine prose
is replaced by executable enforcement.
**Evidence base:** three deep research reports (object-graph cutover ground
truth; Dolt feature surface incl. embedded-driver constraints; heresy
elimination inventory) produced 2026-07-07, plus
[assessment-overall-state-2026-07-07.md](./assessment-overall-state-2026-07-07.md).
**Mutation class:** Red/Black — storage substrate, runtime authority,
promotion/rollback, protected surfaces.

---

## The Shape of the Whole

The three decisions are one program. Their dependency structure, not their
topics, dictates the order:

1. **Heresy deletion shrinks the migration.** Continuations
   (internal/store/continuations.go, ~146 LOC + tables), parent/child
   metadata slots, and agent-wide authority fallbacks all live in the store
   surface. Deleting them first means the cutover migrates ~22 clean
   entities instead of 26 with dead patterns fossilized into `choir.*`
   object kinds. **Never port a heresy into the object graph.**
2. **Batch commits serve both tracks.** The OG cutover's hot-table wall
   (events at 50–200 writes/sec vs 10–50ms per `DOLT_COMMIT`) and the
   Dolt-native audit trail need the same infrastructure: application-level
   write batching with one commit per batch, commit message carrying agent
   identity.
3. **Dolt server mode is the gate for branch-based candidates.** The
   embedded driver is single-writer; concurrent branch/merge operations
   fail. Audit read-path (`AS OF`, `dolt_history_*`, `dolt_log`) works in
   embedded mode today; candidate-computer branches do not. So: audit
   features immediately, branch-based promotion only after the platform
   store moves to sql-server mode — and only after
   `specs/promotion_protocol.tla` is rewritten to model branch semantics
   (spec-first, per the autoputer mission).

Key discovery that reshapes the plan: **the cutover is already half-built.**
`internal/store/graph_store.go` (77KB) carries ~75 OG-backed method variants
in a live dual-write Phase 3, with `backfillOGFromSQL` migration and 34 edge
kinds defined. What remains is capability gaps, batching, the flip, and the
deletion — not greenfield construction.

## Program Sequence

### Phase 0 — Foundations (week 1, parallel, no blockers)

- **Heresy detector CI, discovery mode.** `scripts/check-heresies.sh` +
  detector manifest wired into CI, logging counts per family without
  failing. Baselines: H001-05 ~10 sites, H006-08 ~200, H009-24 ~50,
  H013-18 ~20, H027-29 ~100 (frontend/docs). Flip each family to
  fail-on-regression as its phase completes. This is the mechanism that
  replaces doctrine prose with enforcement.
- **H030 doc closure** (already repaired 2026-06-27; registry update only).
- **Batch-commit infrastructure.** A write-batcher in the store layer:
  collect N mutations (or T ms), one `DOLT_COMMIT` per batch, commit
  message = agent identity + batch summary. Unblocks Phase 3 hot tables and
  makes `dolt_log` a legible audit trail.
- **Proxy/vmctl timeout hardening** (from the overall assessment; small and
  independent): bounded resolve timeout 30–60s, `http.Server`
  Read/WriteTimeouts, fast 504. Every later staging proof depends on
  legible failures instead of 180s hangs.

### Phase 1 — Heresy kill wave 1 + Dolt audit reads (weeks 2–3)

- **M3.1 Texture forcing removal** (H009–H012, H024a/b, H026; ~2–3 pts).
  Delete semantic `next_required_tool` forcing (keep only the mechanical
  patch_texture first-write gate), researcher substring oracles, direct
  super ingress, model-minted update IDs. Proof gate: Texture produces
  honest first revisions and makes delegation decisions unforced.
- **M3.2 Parent/child deletion** (H001–H005, H015–H016; ~3–4 pts).
  H005 first: delete `ensureSpawnedCoagentWorkItem` (runtime.go:759, 902,
  1342, 1435) in favor of trajectory-scoped `CreateWorkItem`. Then
  `GetLatestActiveRunByAgent` (5 production call sites) replaced with
  trajectory-scoped lookups; trace/verifier topology from trajectory+work-item
  edges. Proof gate: all authority is trajectory-scoped.
- **Dolt audit read-path** (parallel; low risk, embedded-driver-safe).
  Serve `texture history` and revision evidence from
  `dolt_history_<table>` + `AS OF` instead of application
  `parent_revision_id` chains; keep the column deprecated for a window.
  Open questions to settle by experiment first: dolt_history latency on
  real revision counts; "latest revision" query cost without the app index.

### Phase 2 — Heresy kill wave 2 + cold-entity cutover (weeks 3–5)

- **M3.3 Acceptance + durable obligations** (H013–H014, H017–H018; ~2 pts).
  Blockers/questions/assignments materialize as durable work items;
  acceptance levels map to evidence classes.
- **M4 Continuation deletion** (H006–H008; ~3–4 pts). Gate: verified zero
  production callers; work-item passivation + trajectory settlement cover
  the semantics. Delete continuations.go, `/api/continuations/*` routes,
  `continuation-level` acceptance. **This must land before the store
  entities migrate** — the continuation tables simply never get OG kinds.
- **OG cutover, cold entities.** Runs → trajectories + work items →
  acceptances → texture documents/revisions/decisions → run memory. These
  map naturally to object+edges (per-entity dual-write flip, OG reads
  default, SQL reads as fallback during the window). Low write rates make
  per-batch commits trivial here.

### Phase 3 — Hot-path cutover (weeks 5–7, highest risk)

- **Events, channel messages, worker updates, coagent mailboxes** — the
  entities where SQL's compound keys and atomicity currently do real work.
  Requires from Phase 0's batcher: seq ordering enforced in application,
  `UNIQUE(loop_id, seq)` as app-level validation, mailbox cursor + ack in
  one batch. Rollback plan per entity: fall back to SQL path if OG batch
  latency exceeds budget (define budget up front; report says commit ≈
  10–50ms, events need ≥10× batching).
- **Trace events** as a separate track (own Dolt store, very high write
  rate; async queue + periodic flush).
- **History bloat control:** enable Dolt automatic GC (1.75+); measure
  storage growth on events under batching before declaring victory.

### Phase 4 — Dolt-native promotion over ComputerVersion (weeks 7–10, spec-first)

**Corrected framing (owner, 2026-07-07): candidate computers are no longer
VMs.** Per the executable definition
[substrate-independent-audited-computer-2026-07-04](./definitions/substrate-independent-audited-computer-2026-07-04.md),
the product object is `ComputerVersion = (CodeRef, ArtifactProgramRef)`;
substrates (Firecracker, Cloud Hypervisor, Nucleus, containers) are
materializers; a candidate is "forked by tape/program reference," and route
identity converges on ComputerVersion records, not VM IDs
(invariant `route-over-computer-version`). Speculative *effects* execute in
capsules — the container-based effect chambers already wired in
internal/capsule + internal/runtime/tools_capsule.go (spawn_capsule,
mint_capability, capsule_exec, commit_transaction) — not in forked VMs.

So Dolt branches enter this plan **as the ArtifactProgramRef fork
mechanism**, not as "a candidate VM's database branch":

- **Rewrite `specs/promotion_protocol.tla` over ComputerVersion**: candidate
  = (same-or-new CodeRef, forked ArtifactProgramRef); ledger fork = Dolt
  branch; capsule transactions append to the candidate tape; promotion =
  atomic route flip to the candidate ComputerVersion (merge-to-main + tag as
  the ledger operation); rollback = route flip back (reset-to-tag). Health
  window and per-ledger verify retained from the existing spec. TLC-checked
  in CI before any Go change.
- **Platform Dolt to sql-server mode** where multi-writer access is real.
  Settle by experiment whether single-writer-per-process topologies keep
  embedded mode.
- **Promotion records name ComputerVersion**, replacing `DoltHeadSnapshot`
  fixtures + `merge_conflicts_json` strings (app_promotion.go) with
  `DOLT_MERGE` conflict tables and `DOLT_TAG` certificates — the tag *is*
  part of the ArtifactProgramRef, which is what the definition doc requires
  promotion evidence to reference instead of opaque images.

### Phase 4b — Candidate-VM residue elimination (new heresy cluster)

The candidate-computer-as-VM implementation is now itself a heresy
(unregistered; needs a number and doctrine entry). Deletion inventory,
gated on Phase 4's spec + route-over-ComputerVersion landing:

- vmctl candidate-desktop lifecycle (candidate desktop publish/switch,
  internal/vmctl/handlers.go:312, client.go:191) — candidates are not
  desktops-in-waiting; they are ComputerVersion refs materialized on demand.
- internal/computerversion/candidate_computer_package*.go +
  candidate_package_app_change_bridge — re-point from VM-state capture to
  ComputerVersion references (the package format survives; its identity
  fields change).
- cmd/candidatepackage, cmd/vmrealize, cmd/vmstatecompare/vmstateobserve —
  audit each: keep what implements materializer/equivalence contracts
  (cross-substrate proof tooling), delete what implements VM-image identity.
- **The wire's platform-computer routing is this heresy's symptom**: the
  proxy hard-codes owner="universal-wire-platform" → desktop="platform" →
  vmctl VM resolve (handlers.go:970) — a route pointing at a VM identity,
  exactly what route-over-computer-version forbids. The 180s hang was this
  heresy expressing itself. Wire fix converges with route-over-version, not
  just timeouts.
- `data.img`-as-canonical residue: per the definition doc, legacy-canonical
  per state class until extract/replay proof; the OG cutover (Phases 2–3)
  is what makes state classes typed and replayable — these tracks meet.
- Service rename sandbox→autoputer and doctrine vocabulary ("candidate
  world/candidate computer" in choir-doctrine.md:118,143 and README's
  "vmctl worker or candidate computer" runtime model) — update to
  ComputerVersion/capsule/materializer vocabulary in the Phase 5 doctrine
  replacement.

### Phase 5 — Deletion and doctrine replacement (weeks 10–12)

- **Drop SQL tables** after a stability window; delete dual-write code and
  the SQL method variants; delete `backfillOGFromSQL`.
- **M5 surface cleanup** (H019–H029, parallel-anytime): retired app
  residue (Trace/Terminal/Browser launchers), lease→budget vocabulary,
  Live/Target/Retired doc restructure.
- **Doctrine replacement.** choir-doctrine.md shrinks to: the system
  thesis, the invariants that remain prose-worthy, and pointers to the
  enforcement that now owns the rest — TLA+ specs (TLC in CI), the heresy
  detector manifest (fail-on-regression), and the narrative layer (the
  grip checkpoint, imported into docs/). The heresy registry closes: each
  entry marked repaired with its detector as the permanent guard.
  Open owner decisions folded in here: self-improving→human-improving
  framing, Universal→World Wire rename (currently nowhere in the tree).

## Effort and Risk Summary

| Track | Estimate | Highest risk |
| --- | --- | --- |
| Heresy elimination (M3.1→M4) | 14–20 pts / 2.5–4 wks | Texture agency proof (M3.1 gate) — forcing removal before the agent reliably chooses |
| OG hard cutover | 4–6 wks, ~125 files, 2–3K LOC | Hot-table batching (events); mailbox cursor atomicity |
| Dolt all-in | audit reads ~1 wk; server mode ~2 wks; branches 4–6 wks | Embedded single-writer; merge conflicts under concurrent agents; unproven promotion spec |

Tracks overlap; the combined program is ~10–12 weeks at recent velocity, with
the first user-visible wins (audit reads via Dolt history, heresy detectors,
timeout hardening) landing in weeks 1–3.

## Settlement Criteria

1. Heresy detector CI green at fail-on-regression for every family; registry
   entries closed with detector references.
2. `internal/store` SQL tables dropped; OG is the only durable model;
   `go test ./...` green without dual-path code.
3. `choir texture history` served from `dolt_history`; at least one
   promotion executed as an atomic route flip between ComputerVersions
   (ledger fork via Dolt branch, capsule-transaction mutation, merge+tag,
   TLC-checked spec) with a demonstrated rollback route flip.
4. No route resolves to a VM identity: wire and all published routes point
   at ComputerVersion records; vmctl candidate-desktop lifecycle deleted.
5. choir-doctrine.md reduced to thesis + invariants + enforcement pointers;
   no live heresy entries; vocabulary updated to
   ComputerVersion/capsule/materializer.
6. The choir CLI's `trajectory`/`texture` verbs read identical shapes before
   and after (external contract preserved), verified against production.
