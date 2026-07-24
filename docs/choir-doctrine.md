# Choir Doctrine

## Status

Canonical doctrine and architecture control document as of 2026-07-07.

This document states:

- what Choir is;
- what all agents are optimizing;
- the current coalesced conjecture set;
- the derived architectural invariants;
- the evidence semantics that bound claims;
- the live heresy set: concrete patterns that violate the intended system;
- the cutover rule that dual-path state is itself a bug.

This document is normative. Supporting docs may explain or justify it, but they
do not override it. This is doctrine, not a granular requirements spec: it
defines the conjectures, invariants, heresies, evidence semantics, and authority
boundaries that agents optimize.

Primary support docs:

- [current-architecture.md](current-architecture.md)
- [computer-ontology.md](computer-ontology.md)
- [conjecture-assertion-ledger-2026-06.md](conjecture-assertion-ledger-2026-06.md)
- [why-texture-2026-06-15.md](why-texture-2026-06-15.md)
- [texture-agentic-invariants-2026-06-13.md](texture-agentic-invariants-2026-06-13.md)
- [runtime-invariants.md](runtime-invariants.md)
- [source-external-data-publication.md](source-external-data-publication.md)

Reading order for architecture or behavior work:

1. this document;
2. [AGENTS.md](../AGENTS.md) for operating procedure;
3. the relevant domain invariant doc;
4. the current mission paradoc;
5. historical reviews and proof artifacts as evidence only.

Supersession rule: when this document conflicts with a support doc, this
document wins unless the support doc is a newer explicitly promoted doctrine
update. Historical specs, master-spec reviews, MissionGradient reports, and
mission ledgers are evidence. They do not silently override Choir Doctrine.

Enforcement direction (owner decision 2026-07-07): doctrine prose is being
replaced by executable enforcement. Invariants trend toward TLA+ specs
model-checked in CI, and heresies trend toward detectors that fail CI on
regression. This document trends toward thesis + invariants + pointers. The
mission form for long-running work is now `skills/definition/SKILL.md`
(`/goal <doc>.md`). The audited-construction Definition completed on 2026-07-17,
the self-development Definition was owner-superseded incomplete on 2026-07-21,
and the
[durable-computer convergence Definition](definitions/choir-coherent-computer-convergence-2026-07-21.md)
completed on 2026-07-24. They remain historical evidence. No product Definition
is executable until separately owner-ratified and promoted through every registry.
OG/Dolt supplies subordinate requirements and evidence only; it owns no
sequencing, mutation, resumption, completion, or escalation.
A heresy entry below without a CI detector is a heresy entry that is not yet done being
written.

## System Thesis

Choir is a human-improving, machine-compounding mainframe: a
persistent-computer system for owned learning over versioned artifacts,
evidence, provenance, and promotion history. "Self-improving mainframe" is
acknowledged historical shorthand and may still appear in older docs and code;
the precise claim is that the improver is the person — the human supplies the
off-distribution judgment — and the system is the compounding memory that
accumulates that judgment as durable owned state surviving model churn.

The primary optimization target is not chat quality, local test passage, or
short-term product smoothness. The target is:

1. truth from facts;
2. correct ontology;
3. recognition of heresies;
4. durable causality;
5. evidence-bounded claims;
6. deletion of heretical legacy control paths;
7. safe self-improvement by typed conjecture.

Product surfaces matter, but they are downstream projections and falsifiers of
the substrate, not substitutes for it.

## Doctrine Of Doctrine

The first best thing is to recognize a heresy: to see and name a real flaw in
the code, docs, tests, prompts, product path, or operating process. A newly
recognized heresy can make the system look worse by increasing the open heresy
count, but epistemically it is progress. Invisible debt cannot be optimized.

The second best thing is to eliminate a named heresy: delete the bad path, fix
the code, invert the test, update the docs, or otherwise remove the false
affordance.

The worse move is to preserve a clean story by hiding evidence, refusing to name
a flaw, shipping around it, or treating product motion as a substitute for
contact with facts. Choir optimizes the conjecture set of the system. Product
shipping is valuable only when it is downstream of truth, ontology, evidence,
and deletion pressure.

Heresy accounting therefore has three separate deltas:

- `discovered`: flaws newly recognized from facts;
- `introduced`: new bad paths created by the current change;
- `repaired`: named heresies reduced or eliminated.

A mission may make epistemic progress by increasing `discovered`. It must not
claim repair progress from discovery alone. A mission regresses if it increases
`introduced` without an explicit conjecture delta and human-readable acceptance
of that new debt.

## Framing Doctrine

Current framing: Choir is a human-improving, machine-compounding mainframe made
of persistent computers ("self-improving mainframe" remains acceptable
historical shorthand: the improver is the person, the system is the compounding
memory). Older framings such as personal writing system, publishing system, AI
workspace, sandbox, workflow app, StoryGraph app, or chat interface are
historical, surface-specific, or deprecated unless this document explicitly
promotes them.
Where those terms reappear below, they are quoted as detector vocabulary or
historical evidence, not endorsed naming.

Naming note (2026-07-07): the Universal Wire is renamed the **World Wire** in
product narrative — humility over totality; it indexes the world *as reported*,
contested and plural, not a god's-eye index. Code identifiers and routes still
say `universal-wire` until a code rename mission lands; until then the code
name is transitional, not endorsed framing.

Framing drift is doctrine drift. If a document, prompt, test, or UI label teaches
agents to optimize an older product story, it can pull code back toward the old
ontology. Reconciliation must therefore include sentiment and narrative
alignment, not only technical symbol deletion.

Preferred vocabulary:

- human-improving, machine-compounding mainframe (historically: self-improving
  mainframe);
- persistent computer;
- ComputerVersion (CodeRef, ArtifactProgramRef);
- durable artifact;
- artifact program;
- trajectory and work item;
- evidence, provenance, verifier contract, and acceptance class;
- stable ComputerID and canonical computer event chain;
- ComputerVersion checkpoint `(CodeRef, ArtifactProgramRef)`;
- capsule effect bundle — frozen speculative effects, never a VM or route;
- capsule (ephemeral effect chamber);
- materializer, acceptance, projection, and event-derived rollback.

Avoid making these the root frame unless the sentence is explicitly about a
surface: personal workspace, AI workspace, publishing system, sandbox, workflow,
chat, StoryGraph, or demo app.

## Conjecture Set

Each conjecture is tagged as one of:

- `asserted`: supported enough to serve as current doctrine;
- `active`: a live system conjecture under continued construction;
- `hyperthesis`: a named blind edge or incompleteness boundary.

### Object-Level Conjectures

`C1 asserted` Choir's primary product object is a persistent computer composed
of multiple ledgers, not a disposable sandbox and not a chat session.

`C2 asserted` Canonical user-facing truth is versioned artifact state. Texture is
the canonical document and artifact control-plane core; other appagents own
their own typed artifact domains.

`C3 asserted` A persistent computer is identified by stable `ComputerID` plus
its canonical event chain. Risky or long-running mutation *effects* execute in
capsules and remain inert as frozen effect bundles until an authorized
acceptance event. Accepted state is materialized into guest releases and
ComputerVersion checkpoints; vmctl may project a checkpoint into a serving
route, but neither materialization, checkpoint publication, nor route CAS is
promotion authority. A speculative self-development candidate is never a VM,
desktop, mutable branch, package, lineage record, or candidate route.

`C4 active` Wire, publication, review, and later economic surfaces are
projections of the artifact-and-provenance substrate, not independent product
ontologies.

### Meta-Level Conjectures

`C5 asserted` Roles are authority envelopes, not identities. Actors should be
given obligations, evidence, scope, and settlement criteria, not persona-heavy
workflow scripts.

`C6 active` The runtime should converge on durable actors: the database
remembers, Go delivers, actors passivate and rewarm, and polling, parent/child
control, and continuation synthesis disappear.

`C7 active` Trajectories and work items are the intended causality model.
Settlement is rule-as-data over durable obligations and subject refs, not root
run completion.

`C8 active` Promotion is an authorized semantic event with explicit proposal,
verification, freshness, privacy, acceptance, materialization, checkpoint,
projection, and rollback receipts. Exactly one per-computer event appender owns
semantic ordering; infrastructure projections and verifier claims cannot
acknowledge or settle the event.

`C9 asserted` Shared-platform claims are evidence-scoped. Staging, verifier
contracts, and owner review define the admissible strength of a claim.

### Meta-Meta Conjectures

`C10 asserted` Choir should evolve by conjecture learning rather than by
checklist completion. Claims must name scope, test, and blind edge.

`C11 asserted` Self-improvement must be stratified: object claims, architectural
claims, and method/doctrine claims are different levels and require different
gates.

`C12 hyperthesis` Conjecture machinery can become decorative unless it changes
route choice, deletion pressure, evidence semantics, and stopping conditions in
practice.

### Open Hyperthesis Edges

`HYP1` Settlement rules may still be wrong in ways current vocabulary does not
state cleanly.

`HYP2` Durable-actor cutover may still hide control loss at the boundary
between agent identity, trajectory identity, and rewarm semantics.

`HYP3` Promotion semantics may still be below the computer ontology even when
approval and freshness checks pass.

`HYP4` Verified harnesses do not imply verified cognition. Residual semantic
risk remains real even when protocol gates pass.

## Derived Architectural Invariants

These are hard consequences of the conjecture set.

`I1` Texture owns canonical document versions. Findings, worker updates, search
results, and verifier output are non-canonical until Texture incorporates them.

`I2` Texture must not be forced into semantic delegation. Runtime may expose
affordances and durable obligations; it must not convert role mentions or
metadata into a required semantic next step.

`I2a` Exogenous user and source input enters Choir through Texture-owned artifact
state by default. Conductor may classify, open, or create the target
Texture/context, but ordinary prompt-bar requests, sourcecycled/news ingestion,
article creation, mission work, and most user prompts must not route directly
to super. Super is downstream execution authority invoked from Texture when the
artifact needs coding, privileged execution, candidate work, generation,
verification, or other supervision.

`I2b` Texture must make owner-triggered work visible as artifact state. For
prompt-bar input, `V0` is the owner prompt and `V1` is Texture's first response to
that prompt. For an existing user-authored Texture, the current user revision is
already canonical; the next Texture-authored revision may be a substantive edit,
draft, acknowledgement, work-state note, blocker, or research/execution plan.
What is forbidden is a mechanically forced trivial patch that hides ongoing
delegation or background work from the owner-readable artifact.

`I2c` Agent-to-agent update identity is runtime-owned. `update_coagent` records
need stable internal identity for idempotency, wake delivery, delivery marking,
Trace joins, and recovery, but an LLM must not have to invent that identity.
Model-visible update payloads may describe kind, target, findings, evidence,
refs, blockers, and questions; the durable `update_id` is minted or
deterministically derived by the runtime from the delivery envelope and
normalized payload.

`I3` Parent/child is not a control ontology. Provenance-only spawned-by edges
may remain temporarily, but control, liveness, settlement, cancellation,
budgeting, and recovery must not depend on parent/child semantics.

`I4` Work items are trajectory obligations, not child-run artifacts. A work
item may record provenance about who requested it, but its meaning is
assignment-on-trajectory, not descent-from-parent.

`I5` Dual paths are bugs. A replacement path does not settle a mission while
the old path remains available for new accretion unless that residual path is
explicitly frozen, gated, and on a named deletion clock.

`I6` No new dependencies may be introduced on a live heresy. Existing
dependencies are debt; new dependencies are regressions.

`I7` If a blocker, assignment, question, or verification result matters for
settlement or rewarm, it must become durable obligation state rather than
remaining only narrative or trace text.

`I8` Acceptance names must not outrun evidence class. Smoke, architectural
proof, export proof, promotion proof, and continuation/settlement proof must be
distinguished.

`I9` Shared-platform behavior claims require staging truth. Local proof is
insufficient for vmctl, auth/session renewal, provider behavior, promotion,
rollback, or Choir-in-Choir claims.

`I10` Architectural mode changes require an explicit conjecture delta. An agent
must not silently pivot the system from agentic to workflow, from trajectory to
run-tree, or from promotion protocol to shortcut behavior in order to satisfy a
probe.

`I11` Problem documentation comes before behavior-changing fix commits for new
reliable failures.

`I12` Supporting docs and tests must not normalize retired ontology.

`I13` Trace, Terminal, and Browser are not normal user-facing product apps.
Trace is an evidence substrate for agentic tracing. Raw Terminal is replaced by
singleton Super Console/zot as an exceptional repair surface. Manual Browser is
replaced in the source path by Source Viewer/reader artifacts plus explicit Web
Lens live/original inspection.

`I14` Source evidence remains object identity, not link-shaped prose. Texture
and successor artifact surfaces represent sources as durable source entities and
transclusions. Ordinary clickable URLs, markdown web links, footnote prose,
source-handle inventories, or "Source:" lines are not acceptable substitutes for
source-backed claims and must not be accepted as proof that a source/citation
path works.

`I15` Source citation is tri-state and citation shape is a display mode, not a
separate node type. Every source entity is cited (`source_ref` in the body),
toolbar-only (a Style.texture style source that shapes the document but is not
cited in the body), or marked-unused (`mark_source_unused` with a rationale in
revision metadata). No source is silently ignored. The former `source_embed`
block node type is removed: all citations are `source_ref` nodes, and
`display_mode` (`numbered_ref` collapsed inline point, or `expanded_ref`
expanded block) is a reader-toggleable presentation choice on the same node.
Style textures are source entities in the toolbar, not body citations. There is
no `WireTexture` prompt control-flow branch: article-format and citation
guidance is unconditional, driven by the default Style.texture.

`I16` Prompts provide data and invariants, not boolean control flow. A prompt
should name the style texture, the available sources, the run context, and the
invariants (cite sources, no model priors as grounded, canonical revisions via
tools). It must not branch on runtime metadata to switch behavior
(`{{if .WireTexture}}`, first-owner-prompt special cases, worker-finding gates).
Unconditional invariant text is not control flow. Decisions that used to live in
prompt branches belong in the style texture, the run context, or tool
availability.

## Proof-Carrying Autonomy

Autonomy increases at the mutation layer only when accountability increases at
the conjecture/evidence layer. A stronger agent must carry a stronger proof
object, not merely move faster.

Mutation classes:

- `green`: docs, comments, labels, and prompt/default text that do not change
  runtime behavior.
- `yellow`: tests, detector manifests, or prompt framing that can change what
  future agents optimize but does not change product behavior directly.
- `orange`: runtime behavior, product APIs, app state, database queries, or
  provider/model routing.
- `red`: protected surfaces: Texture canonical writes, Trace/evidence semantics,
  canonical event acceptance, capsule effects, materialization/checkpoint/route
  projection, rollback, auth/session renewal, vmctl, gateway/provider calls,
  run acceptance, and deployment routing.
- `black`: irreversible or production-destructive operations. These require
  explicit human authority and rollback/restore evidence before execution.

Protected-surface conjecture detour: before an orange or red change lands, the
mission must name the conjecture delta, affected protected surfaces, admissible
evidence class, rollback path, and whether the change discovers, introduces, or
repairs heresy. If the intended fix requires weakening a protected invariant,
the invariant change is the mission, not an implementation detail.

Evidence packet contract:

- mutation class and protected surfaces touched;
- claims made and evidence class for each claim;
- tests, probes, staging/deploy identity when applicable;
- rollback refs or precise rollback blocker;
- heresy delta: `discovered`, `introduced`, `repaired`;
- conjecture delta and remaining blind edge;
- residual risks and a short human-learning digest.

## Evidence Semantics

Claim classes:

- `docs-level`: doctrine and design only.
- `smoke-level`: narrow product-path proof that a surface still opens or a
  minimal path still executes.
- `architectural-level`: proof of the intended causal invariant.
- `export-level`: transferable candidate/source evidence exists.
- `promotion-level`: owner-gated promotion and rollback evidence exists.
- `settlement-level`: trajectory/work-item settlement evidence exists for the
  relevant mission.

**Current code conformance:** `internal/types/acceptance.go` currently exposes
`docs-level`, `staging-smoke-level`, `export-level`, `promotion-level`, and
transitional `continuation-level`. `architectural-level` and `settlement-level`
are doctrine evidence classes, not yet Go `RunAcceptanceLevel` constants;
`smoke-level` is represented by the narrower `staging-smoke-level`. Do not infer
implementation of a stronger class from this doctrine taxonomy.

Rules:

1. `accepted` at smoke level must not be summarized as architectural success.
2. `continuation-level` is transitional and should be deleted or re-pointed to
   trajectory/work-item settlement, not preserved as doctrine.
3. Every acceptance claim must name its evidence class in reports.
4. A source-evidence acceptance claim must prove source entity / transclusion
   behavior. A visible web link or source list is negative evidence for that
   claim unless it is explicitly outside the artifact's source/citation path.
5. A weaker evidence class can falsify a stronger claim, but cannot satisfy it.

## Live Heresies

Each heresy entry includes:

- `heresy_id`
- `bad pattern`
- `detectors`
- `why it violates the spec`
- `successor pattern`
- `deletion gate`

Reduction rule: a heresy is `reduced` only when its detector count decreases or
when explicit non-countable evidence shows the bad pattern can no longer be
used. A replacement path working is not reduction while the old path remains
available. Discovery of a new detector or uncited site is epistemic progress,
not repair progress.

Heresy ledger rule: missions that touch doctrine, runtime control, Texture,
Trace/evidence, promotion, source/Web Lens, or app-state ontology must report
`discovered`, `introduced`, and `repaired` separately. `Delta V` may not count
as progress when introduced heresy count rises unless a human-readable
conjecture delta accepts that debt. Discovery alone never counts as repair.

### Parent/Child And Spawn Residue

#### H001 - Parent/Child API Residue

`bad pattern:` live API/request/response shapes still make `parent_id` the
normal way to create work and still serialize `parent_loop_id` / `ParentRunID`
as ordinary control-facing fields.

`detectors:` `parent_id`, `parent_loop_id`, `ParentRunID`, `StartChildRun`,
`active_child_runs`.

`evidence:` [internal/runtime/api.go](../internal/runtime/api.go),
[internal/runtime/runtime.go](../internal/runtime/runtime.go),
[internal/types/task.go](../internal/types/task.go),
[internal/runtime/api_spawn_test.go](../internal/runtime/api_spawn_test.go).

`why it violates the spec:` it teaches humans and agents to think in parent and
child lifecycles rather than trajectories, assignments, and provenance-only
spawn.

`successor pattern:` trajectory-aware delegation surfaces, provenance-only
`spawned_by_run_id`, and explicit `requested_by_*` metadata.

`deletion gate:` M3 / M3.1.

#### H002 - Parent/Child Store Residue

`bad pattern:` durable schema and helper APIs still normalize parent/child
control queries and slot state around `parent_loop_id`.

`detectors:` `parent_loop_id`, `CountActiveChildRuns`, `ListChildRuns`, direct
store helpers that expose child-run control semantics.

`evidence:` [internal/store/store.go](../internal/store/store.go).

`why it violates the spec:` even after trajectories landed, the store still
offers old causal affordances that new code can easily copy.

`successor pattern:` trajectory- and slot-scoped queries plus provenance-only
spawn references.

`deletion gate:` M3.

#### H003 - Researcher Parent-Target Routing

`bad pattern:` researcher output routing still dereferences `ParentRunID` to
decide where findings should go.

`detectors:` `resolveFindingsTarget`, parent lookup from researcher runs.

`evidence:` [internal/runtime/tools_researcher.go](../internal/runtime/tools_researcher.go).

`why it violates the spec:` recipient identity is inferred from ancestry rather
than from explicit requester metadata, update addressing, or work-item
ownership.

`successor pattern:` stamp requester agent/run/work-item identity when the
obligation is created and route results by addressed update or owning work
item.

`deletion gate:` M3 / Texture hardening.

#### H004 - Trace And Verifier Parent Topology

`bad pattern:` trace and verifier logic still treat `ParentRunID` as live
causal structure rather than frozen provenance.

`detectors:` verifier checks over parent runs, trace edge inference from root
and child runs.

`evidence:` [internal/runtime/texture_workflow_verifier.go](../internal/runtime/texture_workflow_verifier.go),
`internal/runtime/api_trace.go` (deleted; references retained as provenance).

`why it violates the spec:` operator-facing and test-facing truth surfaces keep
rendering the wrong graph, so legacy ontology remains cognitively primary.

`successor pattern:` derive causality from `trajectory_id`, work items,
`requested_by_*`, co-super slots, and update/message edges.

`deletion gate:` M3 / M4.

#### H005 - Work Items Modeled As Spawned Child Artifacts

`bad pattern:` work items are created and labeled as spawned-child artifacts,
including `kind="spawned_child_run"`, `parent_run_id`, `spawned_child:`
fingerprints, and “spawn_agent child objective” reasoning.

`detectors:` `spawned_child_run`, `spawned_child:`, `spawned_work_item_id`,
`passivated_spawned_work_item_id`, `spawned child work`.

`evidence:` [internal/runtime/runtime.go](../internal/runtime/runtime.go).

`why it violates the spec:` it writes the wrong ontology into the durable
obligation substrate. A work item is supposed to mean assignment on a
trajectory, not descent from a parent.

`successor pattern:` assigned trajectory obligations with provenance fields
like `requested_by_agent_id` and `requested_by_run_id`.

`deletion gate:` M3.1 before further lifecycle work.

### Continuation Residue

#### H006 - Live Continuation Runtime

`bad pattern:` the runtime still has a first-class continuation control plane:
selection, compaction-before-handoff, bounded authority, lease clamping, dedupe
by source run, and child-run launch.

`detectors:` `run_continuations`, `RunContinuation`, `ContinuationProposal`,
`SelectRunContinuation`, `StartRunContinuation`,
`maybeStartConfiguredContinuation`, `"request_source": "run_continuation"`,
`request_source.*run_continuation`.

`evidence:` `internal/runtime/continuation.go` (deleted; references retained as provenance),
[internal/store/continuations.go](../internal/store/continuations.go),
[internal/store/store.go](../internal/store/store.go).

`why it violates the spec:` it preserves the old continuation orchestration
model in parallel with work items and trajectories.

`successor pattern:` work items + passivation evidence + update-driven warm/cold
actor wake.

`deletion gate:` M4.

#### H007 - Continuation Product Path

`bad pattern:` `/api/continuations/*` remains blessed in repo-level contracts,
allowlists, and handlers.

`detectors:` `/api/continuations`, `HandleRunContinuationsRoot`,
`HandleRunContinuationDetail`, continuation allowlist entries.

`evidence:` [AGENTS.md](../AGENTS.md),
[internal/runtime/tools_product_api.go](../internal/runtime/tools_product_api.go),
[internal/runtime/api.go](../internal/runtime/api.go).

`why it violates the spec:` it keeps “continuation” alive as a legitimate
product noun and gives new agents an easy old path to copy.

`successor pattern:` work-item- and trajectory-based product/control routes, or
temporary `410 Gone` shims during cutover.

`deletion gate:` M4.

#### H008 - Continuation Acceptance Semantics

`bad pattern:` acceptance and trace still treat continuation events as proof of
progress, and `continuation-level` remains a live acceptance concept.

`detectors:` `continuation-level`, `continued`, continuation events in
acceptance synthesis and trace.

`evidence:` [internal/runtime/run_acceptance.go](../internal/runtime/run_acceptance.go),
[internal/types/acceptance.go](../internal/types/acceptance.go),
`internal/runtime/api_trace.go` (deleted; references retained as provenance),
[AGENTS.md](../AGENTS.md).

`why it violates the spec:` the verifier surface still encodes old run and
continuation machinery instead of trajectory/work-item settlement.

`successor pattern:` acceptance should pivot to passivation checkpoints, open
work items, rewarm evidence, and trajectory settlement; `continuation-level`
should be retired or explicitly re-pointed. A rename alone does not count as
repair.

`deletion gate:` M4.

### Tool Forcing And Texture Agency Residue

#### H009 - Generic Required-Next-Tool Trust Channel

`bad pattern:` any successful tool result that emits `next_required_tool` or
`next_tool` can force exact next-step behavior in the tool loop.

`detectors:` `next_required_tool`, `next_tool`, `required_next_tool`.

`evidence:` [internal/runtime/toolloop.go](../internal/runtime/toolloop.go),
[internal/runtime/toolloop_test.go](../internal/runtime/toolloop_test.go).

`why it violates the spec:` arbitrary tool JSON becomes workflow control policy
instead of staying a narrow mechanical protocol.

`successor pattern:` typed, allowlisted continuation envelopes used only for
bounded mechanical transitions.

`deletion gate:` M3.1.

#### H010 - Texture Semantic Delegation Forcing

`bad pattern:` `edit_texture` can require `spawn_agent` for researcher follow-up
after a canonical write.

`detectors:` `requiredContinuationAfterTextureEdit`, `explicitResearcher`,
`runMetadataExplicitResearcher`, `explicit_researcher_request`,
`durableMetadataKeys`, `textureEditResearcherIntentText`,
`textureTrajectoryHasResearcherParticipation`,
`next_required_tool=spawn_agent`.

`evidence:` [internal/runtime/tools_texture.go](../internal/runtime/tools_texture.go),
[internal/runtime/texture_test.go](../internal/runtime/texture_test.go),
[docs/texture-agentic-invariants-2026-06-13.md](texture-agentic-invariants-2026-06-13.md).

`why it violates the spec:` it turns Texture from an appagent into a workflow
stepper.

`successor pattern:` `edit_texture` stores the revision and stops; Texture decides
what semantic delegation, if any, to perform.

`deletion gate:` M3.1.

#### H011 - Super As Direct Ingress For Texture-Centered Work

`bad pattern:` conductor routes ordinary user, prompt-bar, sourcecycled/news,
article, mission, or document/artifact work directly to super based on prompt
heuristics. This bypasses Texture-owned artifact state and treats Choir as prompt
routing to agents instead of living Texture/artifact state that coordinates
agents.

`detectors:` prompt-bar routing heuristics that select super for Texture-class
objectives, source/article ingestion paths that create super work before a
Texture/context artifact exists, `texturePromptNeedsSuperExecution`,
`prompt_bar_no_worker_decision_route`, no-worker route predicates that patch
individual prompts instead of removing the direct-super ingress path.

`evidence:` [internal/runtime/runtime.go](../internal/runtime/runtime.go),
[docs/texture-agentic-invariants-2026-06-13.md](texture-agentic-invariants-2026-06-13.md).

`why it violates the spec:` conductor becomes a policy engine for Texture/super
authority rather than a router that materializes exogenous input as
Texture-owned artifact state. Super receives authority before the artifact
control plane has interpreted the request, recorded audit-worthy decisions, or
attached downstream evidence back to canonical context.

`successor pattern:` conductor creates or resolves the Texture/context artifact;
Texture owns the canonical artifact and then decides whether to write/revise,
attach or transclude sources, ask researcher, call `request_super_execution`,
coordinate coding-agent trees through super, wait, or record an off-document
decision/blocker.

`deletion gate:` M3.2 / Texture control-plane routing cleanup.

#### H012 - Researcher Intent By Substring Oracle

`bad pattern:` narrative text containing “researcher” can act as control-plane
signal.

`detectors:` substring-based intent inference for researcher or super routing,
`texturePromptExplicitlyRequestsResearcher`, `promptBarExplicitResearcherIntent`,
`texturePromptNeedsSuperExecution`, keyword lists that force super execution.

`evidence:` [internal/runtime/runtime.go](../internal/runtime/runtime.go),
[internal/runtime/tools_texture.go](../internal/runtime/tools_texture.go).

`why it violates the spec:` prose is treated as authority metadata and silently
changes routing semantics.

`successor pattern:` structured intent metadata or explicit Texture-authored
delegation state; no substring oracles.

`deletion gate:` M3.1.

### Acceptance And Authority Residue

#### H013 - Acceptance Overclaim

`bad pattern:` smoke-level accepted states can read as stronger proof than they
are, and some levels are still grounded in old run/continuation semantics.

`detectors:` `staging-smoke-level`, `accepted` on minimal prompt/Texture
evidence, `continuation-level`.

`evidence:` [internal/runtime/run_acceptance.go](../internal/runtime/run_acceptance.go),
[AGENTS.md](../AGENTS.md),
historical source in Git history.

`why it violates the spec:` architectural missions can appear settled on
surface health rather than causal proof.

`successor pattern:` explicit smoke vs architectural vs settlement evidence
classes, with hard reporting discipline.

`deletion gate:` M3 / M4.

#### H014 - Continuation-Level Without Compaction

`bad pattern:` the code can upgrade to `continuation-level` without the full
compaction evidence the doctrine requires.

`detectors:` `continuation-level` granted without a compaction gate, including
paths where `continued` plus another weaker level substitutes for compaction
evidence.

`evidence:` [internal/runtime/run_acceptance.go](../internal/runtime/run_acceptance.go),
[docs/runtime-invariants.md](runtime-invariants.md).

`why it violates the spec:` the name outruns the evidence class and keeps the
old continuation proof shape alive.

`successor pattern:` until repoint lands, continuation-grade proof must require
both compaction and continued evidence; afterward it should be renamed.

`deletion gate:` M4.

#### H015 - Agent-Scoped Residency Short-Circuit

`bad pattern:` resident-run reuse can short-circuit trajectory-scoped
obligation delivery.

`detectors:` resident return before work-item merge or update injection in
trajectory-specific reconciliation.

`evidence:` [internal/runtime/runtime.go](../internal/runtime/runtime.go),
historical source in Git history.

`why it violates the spec:` authority lives on trajectory and work item, but
delivery can be skipped because “some activation of this agent exists.”

`successor pattern:` if the actor is resident, inject the new work item or
update into its durable mailbox path rather than returning early.

`deletion gate:` M3.

#### H016 - Agent-Wide Active-Run Fallback

`bad pattern:` cancellation and super-controller provenance still fall back to
latest-active-run selection.

`detectors:` `GetLatestActiveRunByAgent`, active-run control fallback,
requester provenance recovered from latest active run.

`evidence:` [internal/runtime/runtime.go](../internal/runtime/runtime.go),
[internal/runtime/super_controller.go](../internal/runtime/super_controller.go),
[internal/store/store.go](../internal/store/store.go).

`why it violates the spec:` it preserves cross-trajectory authority bleed and
ambient lineage inference as compatibility truth.

`successor pattern:` resolve through resident activation when present, or via
trajectory/work-item/slot authority; requester provenance should be stamped at
dispatch time.

`deletion gate:` M3.

### Durable Obligation Residue

#### H017 - Blockers And Questions Not Durable As Obligations

`bad pattern:` blockers and questions are meaningful coordination objects but
usually remain only typed updates or narrative text, not durable obligation
state.

`detectors:` `kind=\"blocker\"`, blocker synthesis, absence from
`TrajectoryObligations`.

`evidence:` [internal/runtime/tools_worker_update.go](../internal/runtime/tools_worker_update.go),
[internal/runtime/researcher_checkpoint_fallback.go](../internal/runtime/researcher_checkpoint_fallback.go),
[internal/runtime/delegate_worker_update_fallback.go](../internal/runtime/delegate_worker_update_fallback.go),
[internal/runtime/trajectory.go](../internal/runtime/trajectory.go),
historical source in Git history.

`why it violates the spec:` the docs say blockers and questions are
obligations, but the control substrate does not fully express them that way.

`successor pattern:` blockers/questions that matter for settlement, re-entry,
or supervision become durable obligation state.

`deletion gate:` post-M3 substrate hardening.

#### H018 - Assignment Semantics Not Universally Materialized

`bad pattern:` the architecture intends assignment updates to create durable
work items, but the generic update append path does not universally do that.

`detectors:` `kind=\"assignment\"` without corresponding `CreateWorkItem`.

`evidence:` [internal/store/store.go](../internal/store/store.go),
[internal/runtime/tools_worker_update.go](../internal/runtime/tools_worker_update.go),
historical source in Git history.

`why it violates the spec:` the one-message/one-obligation model remains only
partially realized.

`successor pattern:` transactional update append plus work-item materialization
for assignment-class messages.

`deletion gate:` post-M3 messaging/lifecycle hardening.

### Naming And Doctrine Residue

#### H019 - Lease Vocabulary Drift

`bad pattern:` docs and contracts still use lease language even though v1
explicitly rejects lease as an architectural control concept.

`detectors:` `lease`, `leased`, `worker lease`, `lease_seconds`.

`evidence:` [AGENTS.md](../AGENTS.md),
[docs/current-architecture.md](current-architecture.md),
historical source in Git history,
`internal/runtime/continuation.go` (deleted; references retained as provenance),
[internal/runtime/tools_vmctl.go](../internal/runtime/tools_vmctl.go).

`why it violates the spec:` it invites agents to smuggle lease-shaped control
back into the actor model.

`successor pattern:` activation caps, eviction safety, budget, worker handle,
trajectory obligation, and explicit evidence classes.

`deletion gate:` doctrine cleanup concurrent with M4/M6.

#### H020 - Mixed Current/Target Onboarding

`bad pattern:` foundational docs deliberately mix target doctrine and live
legacy surfaces in a way that still gives both apparent authority.

`detectors:` current/target sections without hard deprecation banners for
retired ontology in onboarding docs.

`evidence:` [docs/current-architecture.md](current-architecture.md),
[docs/README.md](README.md) if present, and other first-read architecture
docs.

`why it violates the spec:` agents can cite either model as sanctioned and keep
building on the old one.

`successor pattern:` sharp separation between live surfaces, target doctrine,
and explicitly retired ontology.

`deletion gate:` ongoing doctrine maintenance.

#### H021 - Stale Or Self-Contradictory Doctrine

`bad pattern:` assertions and architecture notes remain live after code or
newer doctrine falsifies them.

`detectors:` assertion/doc claims contradicted by current code.

`evidence:` [docs/conjecture-assertion-ledger-2026-06.md](conjecture-assertion-ledger-2026-06.md),
[docs/current-architecture.md](current-architecture.md),
[docs/texture-agentic-invariants-2026-06-13.md](texture-agentic-invariants-2026-06-13.md).

`why it violates the spec:` stale doctrine is a heresy vector; agents optimize
what they read.

`successor pattern:` assertions die when their axioms die; doctrine updates are
part of architecture missions, not post-hoc polish.

`deletion gate:` continuous maintenance; mandatory on architecture missions.

### Multi-Step Forcing And Polling Residue

#### H022 - Forced Multi-Step Worker Delegation Script

`bad pattern:` worker delegation can return scripted next-tool chains such as
observe/finish/cancel sequences.

`detectors:` `delegation_required`, `chained_required_tool`, `next_tools`,
worker-delegation results that encode exact semantic tool choreography.

`evidence:` pre-purge review in Git history,
`internal/runtime/tools_vmctl.go`, `internal/runtime/tools.go`.

`why it violates the spec:` it preserves H009's vice under worker-specific
names and makes exact role choreography look like tool protocol.

`successor pattern:` narrow mechanical envelopes only; semantic worker progress
is durable evidence and obligations, not exact next-tool scripts.

`deletion gate:` M3.1/M4 depending on whether the site is generic forcing or
continuation/progress plumbing.

#### H023 - Synchronous Control-Plane Polling

`bad pattern:` foreground runtime code polls internal worker run state until a
terminal condition.

`detectors:` `pollInternalWorkerRun`, polling loops over worker run state,
`time.After(500 * time.Millisecond)` control waits.

`evidence:` pre-purge review in Git history,
`internal/runtime/tools_vmctl.go`.

`why it violates the spec:` it keeps run-tree blocking semantics under the
actor surface and works against asynchronous supervision.

`successor pattern:` durable work items, updates, evidence handles, and wakeable
actors; foreground supervision receives a handle instead of waiting on a poll
loop.

`deletion gate:` M3 lifecycle cutover plus M4 continuation deletion.

#### H024 - Texture First-Tool Forcing By Super-Keyword Oracle

`bad pattern:` prompt keywords can force Texture's initial tool choice to
`request_super_execution`.

`detectors:` `initialTextureToolChoice`, `WithInitialToolChoice`,
`exactRequiredToolChoice`, super-keyword routing lists.

`evidence:` pre-purge review in Git history,
`internal/runtime/runtime.go`.

`why it violates the spec:` even if conductor routes to Texture, the tool loop can
still replace Texture agency with a hidden workflow edge.

`successor pattern:` Texture receives owner intent and available affordances, then
chooses the next semantic move.

`deletion gate:` M3.1.

#### H024a - Trivial First Patch As Hidden Work-State

`bad pattern:` Texture is forced to call `patch_texture` before it can delegate or
wait, and the first write removes or normalizes the owner's instruction without
recording that research, execution, verification, or other background work is
underway.

`detectors:` `initialTextureToolChoice`, `WithInitialToolChoice`, first-revision
metadata with tiny `delta_chars` after an owner work request, revision rationale
that "consumes" an instruction-bearing annotation, Trace showing later
`spawn_agent`/`update_coagent` activity while the Texture revision has no
owner-visible work-state.

`evidence:` [internal/runtime/runtime.go](../internal/runtime/runtime.go),
[internal/runtime/texture_agent_revision.go](../internal/runtime/texture_agent_revision.go),
[docs/texture-agentic-invariants-2026-06-13.md](texture-agentic-invariants-2026-06-13.md).

`why it violates the spec:` it satisfies a mechanical cadence rule while making
the owner-visible artifact less truthful. The bug is not that Texture writes
before research completes; the bug is that the write does not honestly represent
the work Texture has begun or the evidence it is waiting on.

`successor pattern:` Texture's first response to an owner-triggered request is an
honest canonical artifact revision: substantive output when available, or an
acknowledgement/work-state revision that preserves the obligation and names the
active background work. Delegation may follow or happen in the same activation,
but the artifact must not imply the request was merely cleaned up.

`deletion gate:` M3.2 / Texture prompt register and decision notes.

#### H024b - Model-Invented Coagent Update IDs

`bad pattern:` model-authored `update_coagent` calls must provide globally
idempotent `update_id` strings, so natural local labels such as `checkpoint-1`
can collide owner-wide and drop otherwise valid findings.

`detectors:` `update_id` required in the model-facing tool schema,
`update_id ... already exists with different payload`, prompt text asking
researchers or workers to choose checkpoint ids, tests that depend on
model-authored human-readable update ids.

`evidence:` [internal/runtime/tools_worker_update.go](../internal/runtime/tools_worker_update.go),
[internal/store/store.go](../internal/store/store.go),
[docs/choir-prompting-invariants.md](choir-prompting-invariants.md).

`why it violates the spec:` the runtime treats `update_id` as an idempotency and
delivery primitive, but exposes it as if it were semantic content the model can
name correctly. That makes delivery correctness depend on prompt compliance
instead of the durable actor substrate.

`successor pattern:` runtime mints or derives `update_id` from the delivery
envelope plus normalized payload. Explicit IDs are reserved for trusted internal
deterministic paths and tests that assert idempotency directly.

`deletion gate:` M2/M3 messaging cutover repair.

#### H025 - Dead Parent/Child Result-Channel API

`bad pattern:` dead or test-only parent/child result-channel APIs remain in the
codebase and model the forbidden ontology.

`detectors:` `PostChildResult`, `PostChildError`, `WaitForChildResult`,
parent/child channel APIs with no production callers.

`evidence:` pre-purge review in Git history,
`internal/runtime/channels.go`.

`why it violates the spec:` unused compatibility surfaces still teach future
agents the old causal model and become easy copy targets.

`successor pattern:` delete dead parent/child channel APIs; keep only
trajectory/work-item/update semantics.

`deletion gate:` M3.

#### H026 - Prompt-Pipeline Forcing

`bad pattern:` prompt defaults or revision-request builders tell Texture to call
specific semantic workers as a required sequence.

`detectors:` prompt text mandating `spawn_agent` or `request_super_execution`,
`buildAgentRevisionRequest`, "call spawn_agent now", numbered role-sequence
scripts in Texture prompt defaults.

`evidence:` pre-purge review in Git history,
`internal/runtime/prompt_defaults/texture.md`,
`internal/runtime/texture_agent_revision.go`.

`why it violates the spec:` prompt text is architecture when it controls agent
behavior. Moving forcing from runtime code into prompts still violates Texture
agency.

`successor pattern:` prompts describe obligations, authority, evidence, and
available affordances; they do not mandate semantic role choreography.

`deletion gate:` M3.1.

### Retired App Surface Residue

#### H027 - Trace App Residue

`bad pattern:` Trace is presented as a user-facing desktop app, dashboard, or
manual navigation destination.

`detectors:` `Trace app`, `Trace UI`, `Open Trace`, desktop registry entries or
launchers for `trace`, tests expecting a Trace icon, copy that tells users to
manually browse Trace as the debugging surface.

`evidence:` historical source in Git history, <!-- texture-cutover-allow: historical mission evidence path; deletion receipt: texture-hard-cutover-v0 -->
[docs/platform-os-app-state.md](platform-os-app-state.md),
[frontend/src/lib/FeaturesApp.svelte](../frontend/src/lib/FeaturesApp.svelte),
[frontend/tests/desktop-shell-core.spec.js](../frontend/tests/desktop-shell-core.spec.js).

`why it violates the spec:` Trace became a misleading user-facing surface. The
right object is agentic tracing: causal/evidence records that agents, Texture,
run-acceptance synthesis, and Super Console can summarize or open when needed.

`successor pattern:` keep trace evidence APIs, run bundles, acceptance records,
diagnosis artifacts, and machine-readable causal ledgers; do not expose Trace
as a normal desktop app.

`deletion gate:` doctrine upgrade plus a Trace-surface cleanup mission: no
desktop launcher, no "Open Trace" UI copy, and no current docs directing humans
to use a Trace app.

#### H028 - Raw Terminal App Residue

`bad pattern:` Terminal is presented as a user-facing app or ordinary manual
shell workflow.

`detectors:` `Terminal app`, `terminal` app IDs in product-facing registry or
desktop-state tests, comments that say users open Terminal, routes that keep
`/api/terminal/ws` as a live product affordance rather than a compatibility
shim.

`evidence:` historical source in Git history, <!-- texture-cutover-allow: historical mission evidence path; deletion receipt: texture-hard-cutover-v0 -->
[internal/sandbox/terminal.go](../internal/sandbox/terminal.go),
[frontend/tests/terminal-app.spec.js](../frontend/tests/terminal-app.spec.js),
[internal/store/desktop_test.go](../internal/store/desktop_test.go).

`why it violates the spec:` nobody should be using a manual terminal as the
normal operating model for a persistent computer. Semi-manual diagnosis and
repair belongs in Super Console, backed by zot as a coding agent inside the
computer.

`successor pattern:` singleton Super Console per user computer, backed by zot,
with terminal/PTY terminology allowed only as hidden implementation detail.

`deletion gate:` Super Console cleanup mission: product-facing tests, comments,
copy, routes, and app state use Super Console language; any surviving terminal
route is explicitly compatibility-only or removed.

#### H029 - Browser As Source-Gathering App Residue

`bad pattern:` Browser is presented as the source-gathering app or default
source reader for web material.

`detectors:` `Browser for source gathering`, `Browser app`, `BrowserApp`,
`browser_sessions`, `AppHint: "browser"`, source-open plans that choose Browser
or Web Lens merely because a URL exists, docs that say users manually browse
for sources.

`evidence:` [README.md](../README.md),
[docs/current-architecture.md](current-architecture.md),
[docs/platform-os-app-state.md](platform-os-app-state.md),
historical source in Git history,
historical source in Git history,
[internal/runtime/content_extract.go](../internal/runtime/content_extract.go),
[internal/store/browser.go](../internal/store/browser.go),
[internal/types/browser.go](../internal/types/browser.go),
[frontend/src/lib/BrowserApp.svelte](../frontend/src/lib/BrowserApp.svelte),
[frontend/src/lib/apps/registry.ts](../frontend/src/lib/apps/registry.ts),
[frontend/tests/browser-app.spec.js](../frontend/tests/browser-app.spec.js).

`why it violates the spec:` web-origin sources should become durable source
objects with reader artifacts and provenance. A manual browser app makes live
page viewing look like the primary source workflow and keeps source evidence
too close to transient iframe/session state.

`successor pattern:` Texture source marker -> inline/transcluded expansion ->
Source Viewer/reader window -> explicit Web Lens live/original inspection when
needed. Browser/backend-session names may remain only as transitional
implementation names until the source/Web Lens contract is renamed.

`deletion gate:` source/Web Lens cleanup mission: default web-source opens use
Source Viewer/reader artifacts, explicit live/original opens use Web Lens, and
user-facing docs/tests/copy no longer call this a Browser app or source
gathering workflow.

#### H030 - Actor Runtime Database Polling

`status:` **repaired 2026-06-27.** `internal/actor/actor.go:141` declares
`mailbox chan Update` and the warm loop selects on the channel; the log is
queried only for cold-start replay, post-drain overflow catch, and Sweep boot
recovery.
The entry remains as detector vocabulary because this heresy recurred three
times; the deletion gate below is now the regression test.

`bad pattern:` the actor runtime polls the durable log as the delivery
mechanism instead of using Go channels. The loop queries `log.Unprocessed`
every iteration. There are zero `chan` declarations in the actor package. A
vestigial `pending []Update` slice exists but is cleared and ignored. The
database is both the memory AND the delivery mechanism, contradicting the
design principle "The database remembers. Go delivers."

`detectors:` `log.Unprocessed` called inside the actor loop body (not just
cold-start replay), `pending []Update` instead of `mailbox chan Update`, no
`chan` declarations in `internal/actor/actor.go`, comments saying "re-query"
or "steers are already in the log" inside the warm loop.

`detector refs:` [docs/heresy-detectors.md](heresy-detectors.md) H030 row;
`scripts/check-heresies.sh` (discovery mode); `.github/workflows/ci.yml`
`Heresy Detector Discovery` job.

`evidence:` [internal/actor/actor.go](../internal/actor/actor.go),
[docs/heresy-detectors.md](heresy-detectors.md), and the pre-purge Git history.

`why it violates the spec:` the design specifies Go-channel mailboxes for
warm delivery with the durable log only for crash recovery and cold-start
replay. Polling the database every iteration reintroduces the old
database-as-message-bus model under a new name. This heresy recurred three
times: the original `channels.go` message bus, the actor runtime design that
replaced it, and the actor runtime implementation that regressed to polling.

`successor pattern:` `residentActor.mailbox chan Update` (buffered Go channel).
`Send` does a non-blocking channel send when warm. The `loop` selects on the
channel with an idle timer. The log is queried once on cold-start activation
to replay backlog, once after channel drain to catch overflow, and by Sweep
for boot recovery — never as a polling delivery mechanism.

`deletion gate:` the actor runtime must contain `chan` declarations. The warm
loop must `select` on the channel, not call `log.Unprocessed` in a polling
pattern. The test: if there are no `chan` declarations in
`internal/actor/actor.go`, the heresy is present regardless of comments.


#### H031 - Candidate Computer Modeled as VM Identity

`status:` **production route identity repaired** by completed phases B, D, and F
of [audited construction](definitions/choir-audited-autoputer-construction-2026-07-15.md).
The detector remains active against regression and residual legacy
candidate-desktop identity surfaces.

`bad pattern:` Implementing the candidate computer concept as physical VM or desktop instances. This includes forking by cloning a VM/image, running speculative mutations inside a candidate VM, and promotion/rollback as VM-route or image operations.

`detectors:` vmctl candidate-desktop publish/switch lifecycle (`internal/vmctl/handlers.go:312`, `client.go:191`), candidate_computer_package files capturing VM state as candidate identity, route resolutions targeting VM/desktop IDs (see Banned Patterns list item 16).

`detector refs:` [docs/heresy-detectors.md](heresy-detectors.md) H031 row;
`scripts/check-heresies.sh` (discovery mode); `.github/workflows/ci.yml`
`Heresy Detector Discovery` job.

`evidence:` [docs/computer-ontology.md](computer-ontology.md) and the subordinate
H031 contract in
[docs/definitions/og-dolt-heresy-completion-2026-07-08.md](definitions/og-dolt-heresy-completion-2026-07-08.md).

`why it violates the spec:` For self-development, a candidate is a frozen
`CapsuleEffectBundle` bound to stable `ComputerID` and a base event head. It is
never a VM, desktop, mutable branch, package, lineage record, ComputerVersion,
or route. Coupling acceptance to those projections creates a second authority.

`successor pattern:` speculative effects execute in guest-local capsules and
freeze as an inert bundle; an authorized acceptance event advances desired
state; verified guest materialization advances effective state; a
ComputerVersion checkpoint and vmctl route CAS project that applied event.

`deletion gate:` No self-development route resolves to a VM identity or accepts
legacy owner/G3, worker, package, lineage, mutable-branch, or candidate-route
authority. Post-genesis routes require the exact accepted-event checkpoint and
RouteProjectionCertificate joins.

## Banned Patterns

Agents must not introduce:

1. new `ParentRunID` or `parent_id` control reads;
2. new `spawned_child_*` work-item semantics;
3. new uses of `run_continuations` or continuation-shaped APIs for active
   control;
4. new semantic `next_required_tool` or `next_tool` forcing;
5. new semantic first-tool forcing or prompt-pipeline role choreography;
6. new durable metadata that re-derives a semantic delegation obligation across
   turns;
7. new synchronous control-plane polling when a durable handle/update can carry
   the state;
8. new acceptance language that calls smoke evidence architectural success;
9. new authority logic based on latest active run when trajectory- or
   slot-scoped authority exists;
10. new blocker-or-assignment semantics that remain narrative-only while being
   used in settlement reasoning;
11. new docs that normalize retired ontology without labeling it transitional;
12. new Trace desktop/app/dashboard surfaces;
13. new raw Terminal app affordances outside Super Console implementation
    internals;
14. new Browser-as-source-gathering or URL-means-Web-Lens defaults;
15. new actor runtime loops that poll the durable log as a delivery mechanism
    instead of using Go channels — the test is whether `internal/actor/actor.go`
    contains `chan` declarations and the warm loop `select`s on the channel
    rather than calling `log.Unprocessed` in a polling pattern.
16. new product routes, promotion records, or speculative-candidate bindings
    that treat a VM, desktop, mutable branch, package, lineage record, or
    `ComputerVersion` route as self-development candidate authority. The only
    self-development candidate is a frozen capsule effect bundle; vmctl routes
    only accepted checkpoints after an authorizing event.

## Active Cutover Order

Near-term architectural order:

1. M3.1 - remove Texture workflow forcing and document the invariant.
2. M3 - complete lifecycle cutover, especially parent/child residue and
   rewarm/authority cleanup.
3. M4 - delete continuation substrate and re-point acceptance semantics.
4. M5 - use product-path substrate falsifiers only after the above deletion
   work reduces dual-path ambiguity.
5. M6+ - promotion/route semantics and review surfaces on top of the cleaned
   substrate.

Rule: an architectural mission is not settled merely because the replacement
path works. It settles when the replacement works and the named heresy set for
that mission is reduced. Discovery of new heresies is epistemic progress, not
repair progress; keep discovered, introduced, and repaired counts separate.

## Change Protocol

When changing architecture, doctrine, or mission structure:

1. name the conjecture delta;
2. name which invariant changes, if any;
3. name which heresy is discovered, reduced, introduced, or retired;
4. name the evidence class required;
5. refuse silent mode changes.

If a proposed change would alter the system from agentic to workflow, from
trajectory to run-tree, from durable obligation to narrative only, or from
promotion protocol to shortcut path, that change requires an explicit
conjecture and a human-reviewable doctrine update before code lands.

## Short Rule For Agents

Optimize the conjecture set of Choir, not merely the local tests.

When in doubt:

- preserve ontology over convenience;
- seek truth from facts before preserving a nice story;
- name real heresies even when the count looks worse;
- prefer deleting a heresy to adding a bridge around it;
- treat dual paths as bugs;
- do not let a probe or test invent the architecture;
- document the problem before fixing it.
