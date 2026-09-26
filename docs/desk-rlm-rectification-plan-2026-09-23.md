# Desk-RLM Rectification Plan

**Date:** 2026-09-23 (v3 — adjudicated against a 9-agent consensus review,
`.agentic-consensus/agentic-consensus-20260923-201519`; D1 and D4 ratified
by owner same-day). **Revised 2026-09-25** — R2 closed on substrate; a
second consensus pass restructured the spine. See **§11** for the current
mission graph, verified substrate corrections, and owner rulings. §§1–10
are retained as the adjudicated base; where §11 names a contradiction it
governs.
**Status:** **ratified 2026-09-25 by the owner** ("yes, i ratify and give
permission for the whole thing"). §11 (the post-R2 restructure) is now
executable authority for the mission spine; it governs over §§1–10 where
they conflict. §§1–10 remain the adjudicated base record.
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

  Hygiene interleave (orthogonal, no gate — run inside the K tail / before R2):
1.4 Platform-computer park           [red, small — universal-wire-platform
                                      computer is wedged pre-genesis and
                                      spams; hold+hibernate parks it; needs
                                      the IsHeld guard on the warm path]
1.5 Test-signal purge                [yellow — delete low-signal unit tests,
                                      prefer E2E; three authoring rules into
                                      AGENTS.md; parallel subagents]
…then M7 (skip the harness), M9a/b, M10, M11 (self-dev proof — the wire
gate), then world-wire missions (processor/reconciler RLM-ify or delete).
```

Dependencies: R0 independent (land first). R1 ∥ R0. K before R3 (R3's
"revision wakes the desk" IS the kernel's dispatcher — building live desks
on process-local wakes recreates the wrong-path cluster under clean names).
R2 ∥ K partially (ledger schema doesn't need the kernel; desk wakes do).
R3 after K + R2. R4 after R3. R5 anytime after R1, before or after R3 —
it's independent but red. 1.4/1.5 are orthogonal hygiene — they gate nothing
and run inside the K tail or before R2; 1.5 precedes R2 only so R2's CI
absorbs a quieter suite.

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

## 10. Deferred: world-wire redesign

M12–M16 (supervision workbench, beta hardening, wire observation plane,
editorial + publication transaction, World Wire live) need redesign on the
desk-RLM substrate: the wire roles (`processor`, `reconciler`, possibly
`email`, `conductor`) RLM-ify or delete, and the wire becomes
precommitment-records-native. Explicitly deferred — do not design now;
recorded so the stack doesn't read as settled.

## 11. Restructure 2026-09-25 — post-R2 consensus revision

**Status:** **ratified 2026-09-25 by the owner** — supersedes §6's mission list and
amends §7's joins where named. Provenance: a divergent panel
(`.agentic-consensus/agentic-consensus-20260925-192348`, 5 usable routes:
claude, cursor, gemini38, grok46, glm53-flash) generated the option space;
a convergent panel (`.agentic-consensus/agentic-consensus-20260925-193232`,
same 6 routes + devin) adjudicated it. All substrate claims below were
re-verified against `main` after the panel returned.

### 11.1 Verified substrate corrections (vs. what §1–§6 claim)

These are facts the restructure is built on; the §§ they contradict are
marked stale, not silently rewritten:

- **`super_controller.go` is gone** — renamed to
  `internal/agentcore/management_controller.go` (commit `66981cef`). The
  fate sweeps §5/§11's "kernel timer" wants relocated live at
  `management_controller.go:301-302`
  (`resumeStrandedFrozenAssignmentCommits`,
  `enforceEngineeringAssignmentDeadlines`). Every draft citing
  `super_controller.go:333` is stale.
- **D11 is residual, not a build.** The derivable wake already exists:
  `armAssignedEngineeringFateWatchdog` arms `assigned_engineering_fate_deadline`
  at assignment-open and on bound+pending-fate states
  (`engineering_assignment_fate.go:720,851,892-908`), dispatched via
  `actorruntime/handler.go:93`. The residue is the **computer-wide**
  deadline-cancel scan and stranded-freeze scan — those still run only
  inside management reconcile. There is no periodic-timer symbol; the
  correct mechanism is an armed `NotBefore` wake on the dispatcher
  due-index, not a ticker.
- **The in-cell carrier is engineering-only.** `InCellCarrier` is
  `profile == Engineering && capsule.HostSelectsRLM()`
  (`tool_profiles.go:308`); `executeActivation` still takes the tool loop
  when the registry is non-empty (`runtime.go:3065`). `spawnSessionWorker`
  lives in `cmd/capsule-broker/session_worker.go` and is reached only from
  the broker (`handleInitSession`/`sessionFor`), not the host runtime — so
  a host desk-cell is a **build**, not a lift-and-flag.
- **`IntentCast` bypasses the desk policy table.** `commitActIntent` on
  `IntentCast` mints a record and calls `openDelegatedCastAssignment`
  (`rlm_reduce.go:621-636`) with no `AllowedSpawnTargets` check; it
  hard-codes `Kind: EngineeringAssignmentImplementation` and passes
  `in.ToDesk` as the target doc id, so every Cast opens an engineering
  assignment regardless of target. `agentprofile.go:115-116`'s
  `{Research}`/`{Texture,Research}` policy binds only the tool-loop desk —
  it is dead policy for the cell path, not the real blocker.
- **`commit()` is sequential, not atomic.** Record → envelope →
  `CommitInboxCursor` (`rlm_reduce.go` ~560-612). A crash mid-commit leaves
  a partial state — the strand-2 shape R2 was chartered to close.
- **Scores are a stamp, not an accrual.** `choir.Resolve` writes one
  `CommitmentScore` onto the linked resolution record
  (`rlm_reduce.go:735-739`). Derived accrual, materiality, and context
  packs do not exist.
- **Management still feeds the worker-update queue.** `report_to_texture`
  → `QueueLifecycleUpdate` (`tools_engineering_assignment.go`); the R2
  receipts' "only wire roles write" framing is wrong while management is
  tool-loop.
- **`deskModuleSets` still exports the demoted verbs.** `Message`,
  `Outcome`, `Spawn`, `Assign` sit beside the semantic-act verbs
  (`yaegikernel/choir.go:139-155`); R2's own artifact item said Outcome
  folds into Report, Assign into Cast, Message demotes to Note — undone.
- **Staging runs `537fce04`; `choir.Resolve` (`5404d2c7`) is post-deploy.**
  Nothing on staging can prove resolution records until that SHA is
  deployed.
- **All three live drafts anchor stale** (`main@b0adf6f7`, staging
  `4de7fdf9`) and cite the dead `super_controller.go`.
- **The R3 draft imports R4 scope** — "desk scores"/"materiality" appear in
  its `finish.deliver` (`:53,:62`) and acceptance (`:89,:170-171`) while
  `boundaries.excluded` says packs are R4. This is the exact
  "not-reachable-alone" defect that broke R2.
- **There is no `desk-rlm-rectification-plan-2026-09-25.md`** — some panel
  briefs referenced a `-09-25` file that does not exist; the live plan is
  this `-09-23` file.

### 11.2 Restructured mission graph (proposed)

Spine (rectification → self-dev gate). Each `finish.acceptance` names only
its own observable; no mission's acceptance may name a later mission's
deliverable (the R2 boundary rule, bidirectional — also: no
`start.observed_artifact` may cite a nonexistent file).

```text
R2x   substrate-integrity repair          [red, small]
        atomic act commit (record+envelope+cursor transactional or named
        partial-commit recovery) + fate-sweep ungating: delete
        management_controller.go:301-302 after a coverage audit of the
        derivable wakes. Proof: crash mid-commit leaves all-or-nothing;
        a past-deadline assignment cancels while management is unselected.
R3a   texture ledger consumer             [red]
        ResolveTextureActorOccurrence + producerOccurrence* +
        evidenceSourceEntitiesFromWorkerUpdates + the
        ListAllPendingLifecycleUpdates desk-evidence branch resolve desk
        acts from commitment records; dual-read against worker_updates_*.
        Proof: a reducer-minted Report/Resolve record appears in Texture's
        evidence input; processor/reconciler (D4) unaffected.
R3b   host desk-cell carrier              [red]
        host-side spawn of the sessionWorker pattern for non-capsule
        desks; restricted stdlib; InCellCarrier fanned per profile; Cast
        validates target desk + kind. Proof: a management-profiled
        activation runs a yaegi eval in a killable host subprocess; kill
        mid-cell → derivable wake re-fires.
R3c   management live + cast              [red]
        management desk on cells; choir.Cast(engineering) opens+binds+
        executes an assignment (the R2-deferred deployed proof);
        report_to_texture retires to the staged path. Proof: a management
        cell Cast → engineering assignment → Report resolves the cast
        record; restart mid-episode resumes from the tape.
R3d   texture live + authoring            [red]  — split point flagged
        texture desk on cells; genuine AuthorAppAgent revisions
        metabolizing ledger traffic under editorial discretion (D14);
        deletes the desk-originated worker_updates consumer path. Proof:
        doc head advances mid-task via a texture-cell authoring turn
        citing a record id — not a per-milestone projection.
R3r   research cell                       [red, off spine]
        research on cells with the network/memory cap policy resolved
        (D2 residual, contained here). Proof: research cell Report mints
        a record under its cap boundary.
R4    scores + surfacing + packs          [orange]
        derived accrual views, materiality projection, context packs,
        learning-claims gate. Proof: a falsified commitment stays visible
        in the doc; the acting desk's pack contains zero own-score fields.
R5a   vocab decoders + seed freeze        [yellow/orange, off spine]
        per-family frozen decoders + V1→V2 profile normalization +
        explicit keep-choir:co-super-assignment:v3 decision. Proof: a
        pre-migration tape folds identically. Must precede M11 (M11's
        proof tape must be readable under frozen decoders).

M7    skip the harness                    [orange]
        selfdev operations advance to materialization on derivable
        continuations, driven by the management cell — no external
        OMP/CLI stepping process. Proof: a self-dev op reaches a
        materialized change with no external driver.
M9a   platform→computer push + restore    [red]
        platform-signed update push + restore to a pinned head. Proof:
        staging computer applies a pushed update and restores to a pinned
        commit. On M11's restore edge.
M11   self-development gate               [red/black]
        the reversible-selfdev-v1 episode: expected on material actions,
        qualified consensus, promotion, live-play, candidate-B
        falsification, restore. Receipts are scored commitment_records
        readable in the live Texture doc. Depends: M7 + M9a + R3d + R4 +
        R5a.
M9b   computer→computer publish           [orange, after M11]
M10   choir→microVMs                      [red, after M11]
R5b   kind/SQL rename-migrate             [deferred indefinitely]
```

Parallel structure: `R2x ∥ R3a` after closed R2; `R3b` after R2x; `R3c`
after R3b(+R3a); `R3d` after R3a+R3b(+R3c for real traffic); `R3r ∥ R3c`
after R3b; `M7` after R3c; `R4` after R3d; `R5a` anywhere before M11;
`M9a` in parallel once M7 produces something signable.

### 11.3 Owner rulings captured 2026-09-25

- **The live Texture doc gates M11** — owner: "texture doc as live
  supervision is extremely important … it is the delivery and UX for both
  the agent control plane and world wire articles — living documents."
  Therefore R3d + R4 are upstream of the gate; records-only supervision is
  rejected.
- **M7 sits after R3c** — the management cell is the driver; the
  engineering-cell-alone variant (phantom driver) is rejected.

### 11.4 Contested joins — rulings (with dissent)

- **Fate-sweep relocation:** folded into R2x, on the spine — not a free
  mission (two call sites + a coverage audit), not dropped (the residue is
  real: the computer-wide scans have no derivable wake). *Dissent:* claude,
  glm53, cursor would give it its own small mission to keep R2x's
  acceptance singular.
- **R5 split:** R5a (decoders + freeze + keep-v3) gates M11; R5b (rename)
  deferred indefinitely. No agent kept R5 whole on the spine. *Dissent:*
  none material.
- **Texture consumer ordering:** read (R3a) precedes authoring (R3d);
  dual-read is the admit state, not a permanent second authority —
  deletion of the desk-evidence branch is R3d's acceptance item, and R3a
  names R3d as the deletion owner (not a calendar date). *Dissent:* none.
- **Compound-vs-seam:** compound product-atom (merge R3+R4) rejected — it
  repeats R2's defect; R3d is still the compound-risk mission and is
  flagged with a split point (R3d-a = cell + consumer, no write; R3d-b =
  the write) to decide at charter, not mid-flight.

### 11.5 Residuals carried (unassigned, not dropped)

- `deskModuleSets` demoted verbs (`Message`/`Outcome`/`Spawn`/`Assign`)
  must be cut or deferred — assigned to R3b/R3c.
- The `commit`+`CommitInboxCursor` atomicity gap is R2x scope (cursor).
- `engineering:{docID}` desk agent "never runs"
  (`engineering_desk.go:18-19`) — still true; do not write acceptance as
  if that parent executes.
- `docs/desk-rlm-rectification-plan-2026-09-25.md` does not exist — a few
  panel briefs cited it; the live plan is this file.
