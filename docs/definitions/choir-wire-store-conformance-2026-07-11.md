# Choir Universal Wire: D-WIRE Conformance and Legacy Migration Deletion

## Subordinate Invocation Semantics

This document is the settled Wire receipt imported by:

```text
/goal docs/definitions/choir-autoputer-completion-2026-07-14.md
```

Do not invoke it as an independent mission. Its world-wire cutover and deletion
evidence remain a predecessor receipt; the active mission does not rerun it
during resumption.

## Why this mission exists

Twelve activation attempts, including the 12-hour run of 2026-07-10/11,
failed for the reasons documented in
`docs/definitions/choir-autopaper-activation-attempt-report-2026-07-11.md`.
The root finding: the activation definition hard-coded the wire path onto the
platform VM's single-connection embedded Dolt — two days **after** D-WIRE
(og-dolt-heresy-completion-2026-07-08.md) settled that the world-wire store
runs in sql-server mode, multi-writer, with existing wire data discarded as
junk. Agents faithfully rebuilt the rejected topology because the mission
document told them to. This mission is the conformance repair: make the code
match the decisions that already exist.

## Source Authority Order

1. `docs/definitions/choir-autoputer-completion-2026-07-14.md`.
2. This settled subordinate Definition within D-WIRE scope.
3. `docs/computer-ontology.md` "Dolt Store Taxonomy" (owner two-store
   directive: world-wire store + VM-local embedded store, nothing else) and
   **D-WIRE** in `docs/definitions/og-dolt-heresy-completion-2026-07-08.md`
   (world-wire store in sql-server mode, no data migration, code-only
   cutover). The three-domain D-STORES yaml block in that same file is
   orchestrator-settled, unratified, and demoted by this Definition (see
   Settled Inputs).
4. `AGENTS.md` and `docs/choir-doctrine.md`.
5. `docs/current-architecture.md` (corpusd publication boundary; D-STORES
   world-wire vs VM-local split).
6. Observed source:
   - `internal/platform/objectgraph_store.go` and `internal/platform/store.go`
     (the world-wire store, served by corpusd)
   - `internal/runtime/wire_platform_publish.go` (articles already publish to
     corpusd via `WirePublishURL`)
   - `internal/runtime/universal_wire.go` (stories route currently reads the
     edition from the guest's embedded store — the nonconforming read path)
   - `internal/runtime/wire_publication.go` (edition/alias bootstrap in the
     embedded store — the nonconforming edition authority)
   - `internal/store/migration.go` (`backfillOGFromSQL` — the legacy
     migration to delete)
   - `cmd/sourcecycled/main.go` (ingestion; its durable dispatch ledger
     already uses the Node B dolt sql-server)
   - `internal/proxy/handlers.go` (`protectedAPIResolveTarget` routes stories
     into the live VM — the fate-sharing to remove)
7. `docs/definitions/choir-autopaper-activation-2026-07-10.md` — historical
   evidence only. Its topology sections are superseded.

## Settled Inputs (do not re-litigate)

- **D-WIRE (owner, 2026-07-08):** world-wire store is a localhost dolt
  sql-server, multi-writer (proxy, runtime, wire agents). Cutover is
  code-only: TCP DSN, config-governed connection limits. NO DATA MIGRATION —
  existing wire data is junk; stand up fresh.
- **Two-store taxonomy (owner; `docs/computer-ontology.md` "Dolt Store
  Taxonomy"):** the Dolt substrate is exactly two stores — the world-wire
  store (corpusd) and the VM-local embedded store (one per user VM). Wire/
  publication state must not live in a VM-local embedded store.
- **Route-ledger demotion (owner, 2026-07-11):** the three-domain D-STORES
  yaml and D-ROUTE's "vmctl-owned ComputerVersion route ledger" in
  og-dolt-heresy-completion-2026-07-08.md are ORCHESTRATOR-settled syntheses
  (their own source lines say so), never owner-ratified, and never
  implemented. They are not authority. The legitimate need inside D-ROUTE —
  one durable route-slot record with CAS transitions and receipts — is a
  table, not a third store; if built, it lives as a platform-control database
  on the corpusd sql-server (vmctl as sole writer of its tables), conforming
  to the two-store taxonomy. Any doc segment claiming a third Dolt domain is
  superseded by this entry pending explicit owner ratification.
- **Owner, 2026-07-11:** delete the legacy relational→objectgraph boot
  migration ("get rid of the legacy migration — it's all junk data anyway").
  The repo is pre-launch; there is no user data worth a migration ceremony.
- **Publication semantics (owner, 2026-07-11):** "publication" means saved to
  the platform Dolt (corpusd/world-wire store). A piece is published when it
  is durably in that store — not when a live VM can render it.
- **Sequencing (owner, 2026-07-11, restating the deleted
  mission-autoputer-before-autopaper doctrine):** autoputer with working
  self-development precedes autopaper editorial ambitions. This mission is
  substrate conformance, not editorial completion.

## Mission Purpose

1. **Wire state lives in the world-wire store.** Ingested source captures,
   canonical wire articles, and the edition/feed object are rows in the
   corpusd-served sql-server store. The VM-local embedded store holds no wire
   authority.
2. **The stories read path never touches VM lifecycle.** `/api/universal-wire/
   stories` is served from the world-wire store (via corpusd or the proxy's
   product API), with no vmctl resolve, no sandbox proxy, no guest health in
   the request path. Reading the news requires only corpusd and its store.
3. **Delete `backfillOGFromSQL` and its migration machinery.** No boot-time
   legacy migration anywhere. Fresh stores start empty; pre-launch data is
   discarded per the owner decision above.
4. **Sourcecycled writes captures to the world-wire store** (directly or via
   a corpusd/host API), not through the vmctl sandbox proxy into a guest
   objectgraph. Booting a VM must never be a side effect of ingesting a feed.
5. **Processor runs become workers against the wire store.** Wherever the
   processor executes (the platform computer is acceptable as an execution
   substrate), it reads captures from and writes articles/editions to the
   world-wire store. The VM is a worker, not the database.

## Mission Non-Purpose

- No reconciler/editorial-review work. The reconciler is post-publication by
  definition and is out of scope until a one-agent publish path is stable.
- No new services. corpusd and its sql-server primary already exist.
- No changes to the promotion spine, RouteProfile, or the VM-local embedded
  store's role for *private user* computer state.
- No renaming ceremonies (the "platform computer" rename is a flagged open
  decision, not this mission's work).
- No run-lifecycle-authority redesign (report cornerstone C4) or
  artifact-verified completion (C5) beyond what the minimal publish path
  needs; those are named follow-on missions.

## Open Decisions (defaults govern unless owner overrides)

- **Scope of migration deletion:** delete `backfillOGFromSQL` for all
  computers (recommended; pre-launch, all junk), or platform-path only?
  Default if unanswered: delete wholesale, keep the relational tables
  readable in git history.
- **Stories serving owner:** corpusd serves `/api/universal-wire/stories`
  directly, or the proxy serves it by querying the world-wire store?
  Default: proxy product API reads the world-wire store through corpusd's
  boundary, per current-architecture.md ("the browser never talks to Dolt").
- **Edition object shape in the world-wire store:** reuse the existing
  publication/route/manifest rows corpusd already owns, or add an edition
  table? Default: smallest schema that lets the stories route list published
  wire articles as a feed; do not import the embedded-store Texture edition
  format wholesale.

## Invariants

- Wire/publication state is never written to a VM-local embedded store.
- No request on the stories read path may boot, resume, recover, or health-
  check a VM.
- No serving process performs legacy data migration at startup.
- A published article is durable and visible across VM refreshes, deploys,
  and guest recovery, because none of those touch the world-wire store.
- Sourcecycled remains the only source-cycle trigger; the typed ingestion
  handoff remains the activation identity (these survived the post-mortem
  intact).

## Completion Semantics

The mission is `complete` when all of the following are observed on staging:

1. `backfillOGFromSQL` and its cursor/completion tables are deleted from the
   codebase; a fresh platform guest boots to a ready runtime in bounded time
   with no migration log lines.
2. A sourcecycled cycle writes captures into the world-wire store with no
   vmctl sandbox-proxy call on the ingestion path.
3. A processor run produces at least one article durably present in the
   world-wire store (corpusd), with ingestion lineage intact.
4. `/api/universal-wire/stories` returns that article with the platform VM
   **stopped** — the read path is proven independent of VM lifecycle.
   Stop and observe the VM through the product/vmctl API or `choir` CLI, never
   SSH, `systemctl`, raw process inspection, or journal access. If that bounded
   lifecycle surface does not exist, add only the smallest vmctl/`cmd`-side
   lifecycle/status control needed for this proof; it must not add an
   `internal/runtime` surface and it is not R3 observation completion.
5. A deploy (guest refresh included) occurs after publication, and the same
   stories response is returned unchanged afterward.
6. The Universal Wire app (`frontend/src/lib/UniversalWireApp.svelte`) renders
   the article for a signed-in human, not only a curl diagnostic.
7. The active mission's runtime inventory records the removed Wire authority,
   importers, routes/symbols, and applicable file/LOC delta; the settled receipt
   remains valid only while the production ratchet does not regress.

Item 4 is the keystone: it is the observable that was structurally impossible
under the superseded topology.

## Follow-on missions (named, not executed here)

1. **Autoputer stability + self-development** — VM lifecycle that
   distinguishes slow from dead, trustworthy deploy verification. (Report
   cornerstones C2/C3 remainder.)
2. **One run-lifecycle authority** — a single processor capacity/completion
   contract with retry semantics distinguishing "succeeded already" from
   "failed before starting". (C4.)
3. **Artifact-verified agent completion** — a run completes when its required
   artifact exists. (C5.)
4. **Autopaper editorial (reconciler)** — post-publication review, only after
   1–3.
5. **Decision-provenance hygiene** — the registry currently lets
   orchestrator-settled syntheses sit beside owner decisions with the same
   `status: settled`, which is how both the autopaper topology drift and the
   phantom third store happened. Every settled node needs an explicit
   `settled_by: owner | orchestrator` field, and orchestrator-settled nodes
   are proposals until ratified.

## Supersession Record

- Supersedes: topology/Real-Artifact/Determined-State authority of
  `choir-autopaper-activation-2026-07-10.md`. Preserved from it: the evidence
  ledger, the single-authoritative-activation invariant, sourcecycled's
  trigger monopoly, and the typed ingestion handoff identity.
- Conforms to: D-STORES, D-WIRE (og-dolt-heresy-completion-2026-07-08.md).
- Investigation basis:
  `choir-autopaper-activation-attempt-report-2026-07-11.md`.
