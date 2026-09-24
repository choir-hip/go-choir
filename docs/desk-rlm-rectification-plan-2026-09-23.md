# Desk-RLM Rectification Plan

**Date:** 2026-09-23 (v3 — adjudicated against a 9-agent consensus review,
`.agentic-consensus/agentic-consensus-20260923-201519`; D1 and D4 ratified
by owner same-day)
**Status:** proposed — under owner review, **not ratified**. Nothing here is
executable authority until the owner signs off; it then rewrites the mission
stack (`docs/world-wire-mission-stack-2026-09-22.md`) and amends
`AGENTS.md` + the doctrine docs where they conflict.
**Authority:** inherits `docs/choir-doctrine.md`, `docs/why-texture-2026-06-15.md`,
`docs/texture-live-supervision-architecture.md`, `AGENTS.md`, and the
precommitment-records direction (`docs/Precommitment Records — Engineering Memo.md`).
**Mutation class:** green (this document; the work it plans is orange/red).

## 0. Owner direction this plan encodes

Stated 2026-09-23, in conversation:

1. **Go all-in on the desk ontology.** Desks: `management`, `engineering`,
   `research`, `texture`.
2. **Each desk is a root RLM** — a persistent actor, not a per-task spawn.
3. **Each root RLM exposes its own modules from yaegi** — desk-specific
   `choir` package surface per desk.
4. **Agent-to-agent communication happens via yaegi Go functions** — the
   in-cell `choir.*` surface, not a separate tool-call channel.
5. **Restore management↔engineering communication** — the M2 charter deleted
   `assign_co_super` and left management with no reach into engineering; that
   was wrong.
6. **The product point:** the owner opens choir.news and sees a live-updating
   Texture document showing ongoing work state. Idea-level supervision, not
   action-level.
7. **Engineering and research read and write (never update) the object
   graph.** Their message module carries a raw function, but the default
   message is semantic — typed, evidence-asserting.
8. **Precommitments are the semantic frame, pulled forward.** Every desk
   precommits, at whatever granularity is meaningful; precommitments and
   scores surface to Texture as supervision state.
9. `processor`/`reconciler` are world-wire roles — refactor as RLMs later (or
   possibly delete; open question). `email` unchanged for now. `conductor`
   may become a "system one" model — explicitly deferred, out of scope here.

## 1. The mess, honestly stated (post-consensus corrections)

**Naming — three strata, different states:**

- *Live vocabulary:* **already cut over.** `agentprofile.Super = "management"`,
  `CoSuper = "engineering"`, `Researcher = "research"`
  (`internal/agentprofile/agentprofile.go:12-14`); desk agent IDs are already
  `engineering:{docID}` (`engineering_desk.go:20-21`,
  `texture_lifecycle_create.go:92`). The earlier draft's claim that the live
  ID is `co_super:{docID}` was wrong.
- *Durable vocabulary:* still old. `co_super_assignment_*` event kinds
  (`internal/types/lifecycle.go:60-64`), `choir.co_super_assignment` OG kinds
  (`internal/store/cosuper_assignments.go:25-39`), the `co_super_slots` SQL
  table (`internal/store/store.go:402`), the assignment identity seed
  `choir:co-super-assignment:v3`
  (`internal/agentcore/cosuper_assignment_runtime.go:104-108`), CLI evidence
  schema `choir.co_super_capsule_evidence/v1` (`cmd/choir/main.go:830`),
  lifecycle JSON field `co_super_assignments`. Renaming the identity seed
  breaks replay — the same revision would mint a new assignment ID.
- *Go identifiers:* `CoSuperAssignment`, `cosuper_assignment_fate.go`, etc.
  No durable risk; rename whenever.

Also latent: the frozen `computerevent` decoder maps historic profile tokens
to V1 values (`"super"`, `"co-super"`) while live constants are V2 — any
live validation comparing decoded historic `ActorProfile` against
`agentprofile.Super` needs an explicit normalization point.

**Execution — two substrates, but they already interop at the reducer:**

- *Tool-loop desks* (management, research, texture, processor, reconciler,
  conductor): model ↔ tool calls, `update_coagent` packets.
- *Yaegi-cell engineering*: `capsule_go_eval` sole tool; `choir.*` staged
  intents; reducer commits them **sequentially, not atomically**
  (`rlm_reduce.go:405-475` — the earlier draft's "commit together or not at
  all" was false).
- The channels already bridge: staged `choir.Message` on assigned desks
  reduces through the `update_coagent` authority path
  (`rlm_reduce.go:446-455, 524-559`). What's missing is the semantic layer,
  not a bridge.
- `update_coagent`'s blast radius is ~60 files: texture evidence ingestion
  (`texture_evidence_sources.go`, `texture_media_sources.go`,
  `coagent_injection.go`), revision metadata (`worker_updates_*` fields in
  `texture_revision_metadata.go:158-163`), actor park/resume
  (`actorruntime/handler.go`), Super delivery, terminal child-outcome
  fallback (`researcher_checkpoint_fallback.go`), replay goldens. The tool
  registration is retired for engineering; the packet machinery is
  load-bearing elsewhere.

**Supervision.** The engineering-bound document has **no live writer**. The
`engineering:{docID}` desk agent never runs (`engineering_desk.go:18`).
Owner revisions trigger host reconcile → assignment opens directly; the
completion report lands as trajectory events; nothing writes an
`AuthorAppAgent` revision. The owner sees their own last edit. Strand-2's
5-hour wedge is the same amputation: the deadline sweep and stranded-freeze
resume run only inside persistent-Super selection (`super_controller.go:333`)
with no periodic ticker.

**Stale prompt residue:** `super_controller.go:30` still tells the model to
call the deleted `assign_co_super`; `rlm_engineering_runtime.yaml:61` tells
every document cast to `choir.Freeze` (the strand-2 trigger). Prompt overlays
are load-bearing for correctness, not cosmetics.

## 2. The semantic-act substrate (adjudicated)

The consensus correction: **one transport/provenance envelope, two record
semantics — not "every verb is a scored commitment."**

- **Operational acts** (fate-bearing, tracked, not scored): `Cast`
  (delegation — opens an assignment + the caster's implicit commitment),
  `Complete`/`Freeze`/`Verify` (engineering effect verbs — kept, not folded
  into Report), `Ask` (query — resolves on answer, liveness not truth),
  `Note` (raw transport — unscored escape hatch, deliberately weak).
- **Epistemic acts** (claims that resolve and score): `Precommit` (frozen
  prediction *before* resolving evidence — the memo's hard requirement) and
  `Report` (evidence assertion — resolves on recipient acceptance or
  independent verification; the act names its resolver, and the resolver is
  never the claimant for engineering claims).
- **Missing verbs the panel named:** `Resolve` (the resolver's act — D7's
  other half), `Cancel`/`Withdraw` (retracting a commitment), `Escalate`
  (to management or owner), `Reply`/`Answer` (Ask's counterpart).
- `choir.Outcome` folds into `Report`; `choir.Assign` (synchronous broker
  path, `choir.go:199-209`) is a third delegation path to delete or fold
  into `Cast`; `choir.Spawn` is already a staged delegated-cast shape —
  `Cast` must implement *admission*, not rename it.
- The `execution_request` packet kind (privileged control Super executes)
  has no verb — it needs one or an explicit death.

**Delegated cast needs new admission authority.** The current document-cast
API requires an `AuthorUser` revision; assignment identity and parent
control derive from it (`cosuper_assignment_runtime.go:134-183`). A
desk-authored `Cast` is a new contract: admission authority, identity
scheme (must not reuse `choir:co-super-assignment:v3` without a version
bump + replay story), scheduling, and fate binding. Management's cast
surface must also implement the one-live-assignment admission ledger
(doctrine: one live engineering assignment per computer).

**Foliation — corrected.** "Chunk by resolution boundary" stands as the
*recording* criterion (a claim someone could be wrong about). But
"supervision depth = delegation depth" fails as a *display* rule: a deep
delegation can hold one owner-relevant decision; a shallow one can hold
many; a per-cell precommit floods the doc. **Texture keeps editorial
discretion** — the ledger is the foliation for provenance; the doc renders
a materiality projection (top-level claims, falsified claims, overdue
commitments), not the org chart. Commitments need a per-assignment bound,
not just the 16-intent cell tray cap.

**Object graph:** engineering/research assert and read, never update.
`Report` generalizes update_coagent's evidence write. The commitment record
is an **OG object with provenance edges — never a third store** (Phase 2a
rule). Scores stay off the acting agent's context (epistemic boundary —
feeding scores back recreates reward hacking).

**The ledger is not the strand detector by itself.** A record surfaces a
wedge only if something wakes to check it — that trigger is the ontology
kernel's job (R-kernel below). Strand-2's actual fix is the adjudicated
executor-layer patch (freeze validates before effect, watchdog coverage,
store/executor reconcile, Quiesce failure handling) — it lands **now**, as
its own orange commit, not inside the rebuild.

## 3. Target architecture

```text
owner ──revision──► texture doc (lifecycle document)
                        │
                        ▼  revision occurrence wakes the bound desk
              ┌─────────────────────┐
              │   texture desk       │  root RLM, yaegi cells (subprocess)
              │   (sole doc writer)  │  modules: doc revision, inbox,
              └─────────┬───────────┘  report, precommit, note
                        │ choir.Cast / Report
                        ▼
              ┌─────────────────────┐
              │  management desk     │  root RLM, yaegi cells (subprocess)
              │  one per computer    │  modules: inbox, cast, report,
              └─────────┬───────────┘  precommit, escalate, spawn research
                        │ delegated cast (new admission authority)
                        ▼
              ┌─────────────────────┐
              │  engineering desk    │  root RLM; mutation via
              │  capsule-bound       │  capsule-bound sub-RLM cells
              └─────────┬───────────┘  modules: file/exec, freeze/
                        │              complete/verify, inbox, report,
                        ▼              precommit, note
              ┌─────────────────────┐
              │   research desk      │  root RLM, yaegi cells
              │   read-only world    │  modules: read/search, inbox,
              │   + message authority│  report, ask, precommit, note
              └─────────────────────┘  (current read-only scope can't
                                       message — policy redesign needed)
```

All acts write through one commitment ledger on the tape (OG objects, not a
third store). Reports resolve commitments; texture metabolizes the ledger
into `AuthorAppAgent` revisions under editorial discretion. The doc head
moves as work proceeds.

**Isolation (D2 corrected):** desk cells are still model-authored Go —
yaegi already fail-closes without a worker binary (`sidecar.go:85-92`).
Desk cells run in a **killable subprocess with restricted stdlib exports**
(minimum), not in the host process; engineering mutation stays
capsule-bound. Research needs a network/memory-capped boundary — untrusted
web-derived code in-process can OOM the daemon.

**Definitions:**

- **Desk** — a persistent root-RLM actor bound to a profile and a channel;
  runs yaegi Go cells in a subprocess; never terminates while its subject
  is live. (Note: `yaegikernel.Session` persistence is per-activation only —
  restart-persistent desk state is a kernel contract, not free.)
- **Cast** — async admission between actors. *Owner-cast* (owner revision
  admits work — kept) and *delegated cast* (desk's staged `choir.Cast`
  admits work — new admission authority).
- **Commitment** — a typed claim with resolver and deadline on the ledger;
  resolves to an outcome; accrues to the desk's score.
- **Assignment** — the durable work record a cast opens. The fate saga
  (freeze/revoke/destroy/record, ~1100 lines in
  `cosuper_assignment_fate.go`) survives as host machinery — desk actors
  cannot replace cgroup operations; what changes is who opens assignments
  and who notices wedges.
- **Root RLM** — the desk's cell loop is the actor; sub-RLMs are
  per-assignment runs the desk casts.

## 4. Naming cutover — three strata, three treatments

- **Stratum A (live vocabulary):** mostly done. Remainder: Go symbol names,
  prompt overlays, stale comments (`agentprofile.go:100-104` still claims
  the assignment registry includes `update_coagent`), frontend fixtures
  (`role: 'cosuper'` in `responsive-layout.spec.js`), the `super-console`
  app name. Green/yellow, one sweep.
- **Stratum B (durable vocabulary):** event kinds, OG kinds, schema strings,
  SQL tables, CLI schemas, lifecycle JSON fields, replay goldens
  (content-addressed over operation identity — regeneration required).
  **Red-class mission of its own**, with per-family frozen decoders (the
  existing decoder covers only profile tokens). Identity seeds
  (`choir:co-super-assignment:v3`) may never rename — a v4 seed with a
  replay story is the alternative.
- **Stratum C (historic↔live normalization):** the V1→V2 profile decode
  mismatch needs one explicit normalization point.

The "zero old names outside frozen decoders" acceptance applies per-stratum,
not globally — `\bSuper\b` also matches `SuperConsoleApp.svelte` and
`super(message)` in auth.js; the grep needs a real allowlist.

## 5. What gets deleted or replaced

- `update_coagent` tool + packet machinery — replaced by `choir.Report`/
  `Cast` for the four desks, **but** the packet schema likely survives as
  Report's body, and texture's evidence pipeline (`worker_updates_*`
  metadata, source ingestion, park/resume) must migrate first.
  Processor/reconciler keep it until their phase (D4).
- `actuator=tools` desk path — one carrier: yaegi cells.
- The phantom desk-agent concept — desk agents become real actors.
- `assign_co_super` stays deleted; its replacement is the delegated cast.
- `choir.Assign` (sync broker path) and `choir.Outcome` — fold into
  `Cast`/`Report`; `choir.Message` demoted to `choir.Note`.
- `super_controller.go` — R3 replaces it, doesn't sit beside it; the fate
  sweeps it hosts (deadline, stranded-freeze resume) move to the kernel's
  periodic timer, not to a desk.
- Strand-2 bug class — fixed in the standalone patch (R0) and carried
  forward: freeze validates before effect; watchdog on every terminal-saga
  failure; store/executor reconcile; Quiesce restores Active on cgroup
  failure; Freeze reduce gets WithoutCancel; the Freeze prompt mandate
  gates behind selfdev-operation presence.

## 6. Proposed mission sequence (post-consensus)

Replaces the M2-tail region of the mission stack. The consensus ordering:

```text
R0  Strand-2 patch                   [orange, small, NOW — independent of
                                      the rebuild; adjudicated in the
                                      problem doc]
R1  Docs + live-name leftovers       [green/yellow: stratum A only —
                                      Go symbols, prompts, CLI, frontend,
                                      AGENTS.md, doctrine amendments,
                                      mission-stack rewrite]
K   Ontology kernel                  [red — the ratified M3: durable wakes,
                                      fenced per-actor commits, derivable
                                      continuations, periodic timer,
                                      wrong-path deletion (~44 instances)]
R2  Commitment ledger + carrier      [red: OG commitment record type,
      generalization + verbs              yaegi carrier to all desks
                                      (subprocess, restricted stdlib),
                                      semantic-act verbs, delegated-cast
                                      admission authority, update_coagent
                                      migration for the four desks]
R3  Live desks + supervision loop    [red: desk actors on derivable wakes,
                                      management→engineering cast,
                                      texture writes doc revisions from
                                      the ledger under editorial
                                      discretion; super_controller
                                      replaced]
R4  Scores + surfacing + packs       [orange: score accrual, materiality
                                      projection, context packs,
                                      learning-claims gate]
R5  Durable vocabulary migration     [red: stratum B — event kinds, OG
                                      kinds, SQL, schemas, goldens;
                                      may be deferred or partial]
…then M7 (skip the harness), M9a/b, M10, M11 (self-dev proof — the wire
gate), then world-wire missions (processor/reconciler RLM-ify or delete).
```

Dependencies: R0 independent (land first). R1 ∥ R0. K before R3 (R3's
"revision wakes the desk" IS the kernel's dispatcher — building live desks
on process-local wakes recreates the wrong-path cluster under clean names).
R2 ∥ K partially (ledger schema doesn't need the kernel; desk wakes do).
R3 after K + R2. R4 after R3. R5 anytime after R1, before or after R3 —
it's independent but red.

## 7. Open decisions (owner) — post-consensus

- **D1 — Management liveness: RATIFIED 2026-09-23.** One persistent
  `management` desk per computer (doctrine: one Super per computer).
  Residual: current persistent ID is `management:{ownerID}` — the
  owner-vs-computer scoping migration lands in R1/R5.
- **D2 — Capsule binding:** **corrected** — desk cells run in a killable
  subprocess with restricted stdlib (not in-process); engineering mutation
  stays capsule-bound; research needs network/memory caps.
- **D3 — Texture desk scope:** per-doc. Consensus agrees; sole-writer
  arbitration is per canonical document (CAS on doc head).
- **D4 — processor/reconciler: RATIFIED 2026-09-23.** Deferred — they stay
  on the tool-loop until world wire (likely deleted there; core system
  first). `update_coagent` survives until they migrate — the deletion
  boundary must be explicit in R2.
- **D5/D6 — conductor, email:** deferred.
- **D7 — Report resolution:** the act names its resolver, but admission
  policy authorizes it; recipient-acceptance and independent-verification
  are separate outcomes; resolver is never the claimant for engineering
  claims.
- **D8 — Score shape:** not bare scalar — keep raw evidence, typed outcome,
  scorer identity, discrepancy class, specificity from day one (the memo's
  schema); scalar views are derived.
- **D9 — Commitment quotas:** per-assignment open-commitment bound +
  deadline, separate from the 16-intent cell tray cap.
- **D10 — Verification opener:** host-side (current) vs. management cast.
  Verification assignments are opened host-side today
  (`engineering_desk.go:176-230`); handing to the model weakens independent
  verification. Recommend host-side stays.
- **D11 — Fate sweeps placement:** the deadline sweep + stranded-freeze
  resume move to the kernel's periodic timer (K), not to a desk and not
  gated on desk selection.
- **D12 — Commitment record location:** OG object with provenance edges,
  never a third store (settled by Phase 2a rule — recorded here so R2
  doesn't re-open it).
- **D13 — `execution_request` packet kind:** privileged control path needs
  a verb or an explicit death.
- **D14 — Texture authoring:** desk-cell-authored `AuthorAppAgent`
  revisions vs. the existing `ApplyTextureTurn` path — one must be named
  canonical; doctrine requires a genuine authoring turn, not a synthetic
  projection.

## 8. Acceptance shape for the rectification

- Owner opens a live engineering-bound doc on choir.news mid-task and sees
  `AuthorAppAgent` revisions advancing — via a genuine texture authoring
  turn, not a per-milestone projection (doctrine forbids revision-per-
  milestone).
- A management→engineering delegated cast is observable on canonical
  evidence under the new admission authority.
- Restart durability proven: kill the process mid-task; desk state,
  pending casts, and open commitments recover from the tape (K's
  acceptance, exercised by R3).
- Reports resolve commitments; per-desk scores visible in the doc;
  scores never enter the acting agent's context.
- Stratum-A grep clean; stratum-B migration verified or explicitly
  deferred with decoders in place.
- The strand-2 wedge class closed: R0 patch + kernel timer + ledger
  surfacing, verified by staging reproduction.

## 9. What this plan does NOT change

- The document channel as owner-input path (kept).
- Owner-cast admission (kept; one of two cast subspecies).
- The yaegi cell carrier (generalized, not replaced).
- Capsule isolation for engineering mutation.
- Tape-primary / reducer authority model.
- The precommitment-records direction — it becomes the substrate, with the
  memo's full record schema (frozen prediction, observation, scorer
  identity, discrepancy class, specificity), not a scalar.
- The assignment/fate saga as host machinery.
