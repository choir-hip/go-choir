# Choir Doctrine

## Status

Canonical doctrine and architecture control document. Supersedes the 2026-07-07
revision; the durable-computer convergence Definition completed 2026-07-24 is the
current substrate authority (see ACTIVE.md), and the product path follows the
re-centered vision (choir-vision.md).

This document states:

- what Choir is and what all agents are optimizing;
- the coalesced conjecture set;
- the derived architectural invariants;
- the evidence semantics that bound claims;
- the live heresy index and its reduction rules;
- the product path from the current substrate to the vision.

This document is normative. Supporting docs explain or justify it; they do not
override it. This is doctrine, not a granular requirements spec.

Primary support docs:

- [choir-vision.md](choir-vision.md) — product direction; the automatic computer
  for supervised self-development first, the World Wire downstream.
- [current-architecture.md](current-architecture.md)
- [computer-ontology.md](computer-ontology.md)
- [conjecture-assertion-ledger-2026-06.md](conjecture-assertion-ledger-2026-06.md)
- [why-texture-2026-06-15.md](why-texture-2026-06-15.md)
- [texture-agentic-invariants-2026-06-13.md](texture-agentic-invariants-2026-06-13.md)
- [runtime-invariants.md](runtime-invariants.md)
- [source-external-data-publication.md](source-external-data-publication.md)
- [heresy-detectors.md](heresy-detectors.md) — executable detector manifest.

Reading order for architecture or behavior work: this document; AGENTS.md for
operating procedure; the relevant domain invariant doc; the current mission
paradoc; historical reviews and proof artifacts as evidence only.

Supersession rule: when this document conflicts with a support doc, this document
wins unless the support doc is a newer explicitly promoted doctrine update.
Historical specs, master-spec reviews, MissionGradient reports, and mission
ledgers are evidence. They do not silently override Choir Doctrine.

Enforcement direction: doctrine prose is being replaced by executable
enforcement. Invariants trend toward model-checked specs; heresies trend toward
detectors that fail CI on regression. This document trends toward thesis +
invariants + pointers. A heresy index entry without a CI detector is an entry
that is not yet done being written.

## System Thesis

Choir is a human-improving, machine-compounding mainframe: a persistent-computer
system for owned learning over versioned artifacts, evidence, provenance, and
promotion history. "Self-improving mainframe" is acknowledged historical
shorthand; the precise claim is that the improver is the person — the human
supplies the off-distribution judgment — and the system is the compounding
memory that accumulates that judgment as durable owned state surviving model
churn.

The foundational argument is
[signal-is-sparse-not-the-learner-2026-08-01.md](signal-is-sparse-not-the-learner-2026-08-01.md):
sample inefficiency is undirected learning, not architecture; signal density
comes from a learner with standing questions and from correction by genuinely
independent others; both must be built into the environment the intelligence
runs in. Choir is that environment — the tape is standing state, the harness is
owned, plurality is architecturally available, and correction is an ordinary
write. Supervision is the binding constraint on the frontier, and this system is
the architecture supervision lives in.

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

The first best thing is to recognize a heresy: to see and name a real flaw in the
code, docs, tests, prompts, product path, or operating process. A newly
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

Heresy accounting has three separate deltas:

- `discovered`: flaws newly recognized from facts;
- `introduced`: new bad paths created by the current change;
- `repaired`: named heresies reduced or eliminated.

A mission may make epistemic progress by increasing `discovered`. It must not
claim repair progress from discovery alone. A mission regresses if it increases
`introduced` without an explicit conjecture delta and human-readable acceptance
of that new debt.

## Framing Doctrine

Current framing: Choir is a human-improving, machine-compounding mainframe made
of persistent computers. Older framings such as personal writing system,
publishing system, AI workspace, autoputer, workflow app, StoryGraph app, or chat
interface are historical, surface-specific, or deprecated unless this document
explicitly promotes them. Where those terms reappear below, they are quoted as
detector vocabulary or historical evidence, not endorsed naming.

Naming note: the Universal Wire is renamed the **World Wire** in product
narrative — humility over totality; it indexes the world *as reported*, contested
and plural, not a god's-eye index. Code identifiers and routes still say
`universal-wire` until a code rename mission lands; until then the code name is
transitional, not endorsed framing.

Framing drift is doctrine drift. If a document, prompt, test, or UI label teaches
agents to optimize an older product story, it can pull code back toward the old
ontology. Reconciliation must include sentiment and narrative alignment, not only
technical symbol deletion.

Preferred vocabulary:

- human-improving, machine-compounding mainframe (historically: self-improving
  mainframe);
- persistent computer;
- durable artifact, artifact program, ComputerVersion (CodeRef,
  ArtifactProgramRef) as **code/artifact identity** at an event head — not alone
  a complete restore address;
- trajectory and work item;
- evidence, provenance, verifier contract, and acceptance class;
- stable ComputerID and canonical computer event chain;
- capsule effect bundle — frozen speculative effects, never a VM or route;
- capsule (ephemeral effect chamber);
- materializer, acceptance, projection, and **event-derived restore** (forward
  rematerialization / acceptance-fenced return; prefer "restore" over "rollback"
  when meaning product undo — git revert remains a separate repo operation).

Avoid making these the root frame unless the sentence is explicitly about a
surface: personal workspace, AI workspace, publishing system, autoputer, workflow,
chat, StoryGraph, or demo app.

## Conjecture Set

Each conjecture is tagged as one of:

- `asserted`: supported enough to serve as current doctrine;
- `active`: a live system conjecture under continued construction;
- `hyperthesis`: a named blind edge or incompleteness boundary.

### Object-Level Conjectures

`C1 asserted` Choir's primary product object is a persistent computer composed
of multiple ledgers, not a disposable autoputer and not a chat session.

`C2 asserted` Canonical user-facing truth is versioned artifact state. Texture is
the canonical document and artifact control-plane core; other appagents own
their own typed artifact domains.

`C3 asserted` A persistent computer is identified by stable `ComputerID` plus
its canonical event chain. Risky or long-running mutation *effects* execute in
capsules and remain inert as frozen effect bundles until an authorized
acceptance event. Accepted state is materialized into guest releases and ComputerVersion code/
artifact identities at event heads; vmctl may project a checkpoint into a serving
route, but neither materialization, checkpoint publication, nor route CAS is
promotion authority. A speculative self-development candidate is never a VM,
desktop, mutable branch, package, lineage record, or candidate route.

`C4 active` Wire, publication, review, and later economic surfaces are
projections of the artifact-and-provenance substrate, not independent product
ontologies.

### Meta-Level Conjectures

`C5 asserted` Roles are authority envelopes, not identities. Actors should be
given obligations, evidence, scope, settlement criteria, an assignment-scoped
module manifest, and revocable activation capabilities—not persona-heavy
workflow scripts or ambient privilege derived from a role name.

`C6 active` The runtime should converge on durable actors whose accountable
identity and organizational relationships survive passivation while each wake
runs in a disposable private activation. The database and trajectory remember,
Go delivers, and the activation model incrementally authors and executes Go
against assignment-scoped modules. Interpreter heaps, goroutines, channels, and
subprocesses are activation-local and are never restored across rewarm.

`C7 active` Trajectories and work items are the intended causality and
settlement model. Settlement is rule-as-data over durable obligations and
subject refs, not root run completion. Consequential activation operations form
a causally joined evidence graph; any assignment, blocker, question,
disagreement, verification result, continuation, or disposition needed for
settlement or rewarm becomes durable workstream state rather than remaining
only in code, a transcript, or Trace.

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

`C13 active` Model-authored Go should become Choir's adaptive actor activation
language: a private executable medium for composition, multiagent
orchestration, and reusable organizational learning. Reused source carries no
authority; each activation resolves imports and receives fresh scoped
capabilities. This conjecture remains active until product-path evidence shows
that the common kernel can replace persona-specific loops without reducing
correctness, containment, restart continuity, or supervision legibility.

`C14 asserted` Reversibility is a recovery property, not the boundary of
autonomy. Reversible and irreversible effects may both be authorized by a
predeclared, effect-specific multiagent consensus policy. The policy binds the
exact subject, eligible seats and independence domains, quorum, dissent,
evidence, capabilities, consequence receipts, and recovery or compensation
plan before participant outputs are visible. A human may be a required,
optional, or absent seat; human approval is not a universal effect gate.
Trusted reducers and actuators enforce the resulting decision—model consensus
does not become canonical state authority.

### Open Hyperthesis Edges

`HYP1` Settlement rules may still be wrong in ways current vocabulary does not
state cleanly.

`HYP2` Durable-actor cutover may still hide control loss at the boundary between
agent identity, trajectory identity, and rewarm semantics.

`HYP3` Promotion semantics may still be below the computer ontology even when
approval and freshness checks pass.

`HYP4` Verified harnesses do not imply verified cognition. Residual semantic risk
remains real even when protocol gates pass.

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
article creation, mission work, and most user prompts must not route directly to
super. Super is downstream execution authority invoked from Texture when the
artifact needs coding, privileged execution, candidate work, generation,
verification, or other supervision.

`I2b` Texture must make owner-triggered work visible as artifact state. For
prompt-bar input, `V0` is the owner prompt and `V1` is Texture's first response to
that prompt. For an existing user-authored Texture, the current user revision is
already canonical; the next Texture-authored revision may be a substantive edit,
draft, acknowledgement, work-state note, blocker, or research/execution plan.
What is forbidden is a mechanically forced trivial patch that hides ongoing
delegation or background work from the owner-readable artifact.

`I2c` Agent-to-agent update identity is runtime-owned. The runtime mints or
deterministically derives `update_id` from the delivery envelope and normalized
payload; an LLM must not have to invent it. Model-visible update payloads may
describe kind, target, findings, evidence, refs, blockers, and questions; the
durable `update_id` is not semantic content.

`I3` Parent/child is not a control ontology. Provenance-only spawned-by edges may
remain temporarily, but control, liveness, settlement, cancellation, budgeting,
and recovery must not depend on parent/child semantics.

`I4` Work items are trajectory obligations, not child-run artifacts. A work item
may record provenance about who requested it, but its meaning is
assignment-on-trajectory, not descent-from-parent.

`I5` Dual paths are bugs. A replacement path does not settle a mission while the
old path remains available for new accretion unless that residual path is
explicitly frozen, gated, and on a named deletion clock.

`I6` No new dependencies may be introduced on a live heresy. Existing
dependencies are debt; new dependencies are regressions.

`I7` If a blocker, assignment, question, or verification result matters for
settlement or rewarm, it must become durable obligation state rather than
remaining only narrative or trace text.

`I8` Acceptance names must not outrun evidence class. Smoke, architectural,
export, promotion, and settlement proof must be distinguished.

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

`I17` The complete actor execution unit is durable trajectory/workstream plus
accountable actor plus disposable private activation. No interpreter process,
heap, goroutine, channel, or subprocess is durable actor identity or
settlement-critical memory.

`I18` Yaegi imports present an attenuated capability vocabulary; they do not
grant authority. Every consequential operation is independently authorized by
current activation capabilities and a trusted broker. Arbitrary model-authored
code runs only in a disposable guest-local capsule with resource and effect
containment; Yaegi is not the security boundary.

`I19` General process execution is absent by default. It may be mounted for an
effects-capable assignment, not inherited from a role name. Direct Bash and Go
execution calls must use one capsule execution broker and one receipt and
transaction-tape path. Spawned processes never inherit Choir capabilities,
credentials, canonical-state access, or control sockets.

`I20` Persisted model-authored source is inert. Loading or reusing it never
restores an activation capability, live handle, interpreter heap, or prior
authority; imports, handles, heads, policy, and operations are resolved and
authorized anew.

`I21` Activation-local goroutines and channels express ephemeral computation.
Independent responsibility, communication, waiting, recovery, and supervision
use durable assignments, authenticated typed messages, work items, and
obligations. An activation parks rather than remaining alive to wait for
another actor.

`I22` Every model-authored Go cell and every consequential capability boundary
crossing produces immutable causal evidence. Shell/process execution,
assignment, agent-to-agent messaging, source and artifact access, capability
requests, verification, effect-bundle freezing, continuation, and outcome
receipts are citable. This requires a normalized orchestration graph, not
instruction- or syscall-level surveillance.

`I23` The host derives the complete salient receipt set for supervision; an
actor cannot hide a consequential action or dissent by omitting it from a
report. Host-owned privacy policy removes credentials and unauthorized private
content without allowing actor-controlled redaction to erase the receipt
identity, event class, disposition, or existence of inconvenient evidence.
Supervisors disposition and explain the admissible evidence. Texture may
transclude exact immutable excerpts into canonical versions, but Trace,
receipts, reports, and transclusions remain evidence inputs and never acquire
Texture or canonical computer-event authority.

`I24` A capability available through an actor's Go modules must not remain
available through a parallel ambient model-tool path. Restricted profiles use
the private Go activation only; an effects-capable implementation assignment
may additionally receive direct Bash as an ergonomic view of the same capsule
execution broker.

`I14` Source evidence remains object identity, not link-shaped prose. Texture
and successor artifact surfaces represent sources as durable source entities and
transclusions. Ordinary clickable URLs, markdown web links, footnote prose,
source-handle inventories, or "Source:" lines are not acceptable substitutes for
source-backed claims.

`I15` Source citation is tri-state and citation shape is a display mode, not a
separate node type. Every source entity is cited (`source_ref` in the body),
toolbar-only (a Style.texture source that shapes the document but is not cited in
the body), or marked-unused (`mark_source_unused` with a rationale in revision
metadata). No source is silently ignored. `display_mode` (`numbered_ref`
collapsed inline point, or `expanded_ref` expanded block) is a reader-toggleable
presentation choice on the same node. Style textures are source entities in the
toolbar, not body citations.

`I16` Prompts provide data and invariants, not boolean control flow. A prompt
should name the style texture, the available sources, the run context, and the
invariants. It must not branch on runtime metadata to switch behavior. Decisions
that used to live in prompt branches belong in the style texture, the run
context, or tool availability.

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
  projection, rollback, auth/session renewal, vmctl, gateway/provider calls, run
  acceptance, and deployment routing.
- `black`: irreversible or production-destructive operations. These require the
  strongest predeclared authority policy, qualified independent consensus,
  exact subject binding, durable consequence evidence, and a recovery or
  compensation plan before execution. A human seat is required only when that
  policy says so. Repository-agent safety rules may separately require human
  authorization for destructive maintenance; do not generalize that operator
  boundary into product ontology.

Protected-surface conjecture detour: before an orange or red change lands, the
mission must name the conjecture delta, affected protected surfaces, admissible
evidence class, rollback path, and whether the change discovers, introduces, or
repairs heresy. If the intended fix requires weakening a protected invariant, the
invariant change is the mission, not an implementation detail.

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
- `staging-smoke-level`: narrow product-path proof that a surface still opens or
  a minimal path still executes.
- `export-level`: transferable candidate/source evidence exists.
- `promotion-level`: policy-authorized promotion, materialization, and recovery
  evidence exists, including the exact consensus decision receipt.
- `settlement-level`: trajectory/work-item settlement evidence exists for the
  relevant mission. (Doctrine class; `settlement` is synthesized from trajectory
  and work-item state, not a separate Go constant.)

**Current code conformance:** `internal/types/acceptance.go` exposes
`docs-level`, `staging-smoke-level`, `export-level`, and `promotion-level`.
`continuation-level` has been retired. `settlement-level` is a doctrine evidence
class, not a Go `RunAcceptanceLevel` constant. Do not infer implementation of a
stronger class from this taxonomy.

Rules:

1. `accepted` at staging-smoke level must not be summarized as architectural
   success.
2. Every acceptance claim must name its evidence class in reports.
3. A source-evidence acceptance claim must prove source entity / transclusion
   behavior. A visible web link or source list is negative evidence for that
   claim unless it is explicitly outside the artifact's source/citation path.
4. A weaker evidence class can falsify a stronger claim, but cannot satisfy it.

## Live Heresies

The full detector inventory — grep patterns, targets, and reduction counts — is
the executable manifest in [heresy-detectors.md](heresy-detectors.md). That
manifest is authoritative for detector vocabulary. This section is the doctrine
index: what each heresy is, what replaces it, and the gate that retires it.

Reduction rule: a heresy is `reduced` only when its detector count decreases or
when explicit non-countable evidence shows the bad pattern can no longer be
used. A replacement path working is not reduction while the old path remains
available. Discovery of a new detector or uncited site is epistemic progress, not
repair progress.

Heresy ledger rule: missions that touch doctrine, runtime control, Texture,
Trace/evidence, promotion, source/Web Lens, or app-state ontology must report
`discovered`, `introduced`, and `repaired` separately. Discovery alone never
counts as repair.

### Parent/Child And Spawn Residue

`H001` **Parent/child API residue** — `parent_id` as the normal way to create
work; `parent_loop_id` / `ParentRunID` as ordinary control-facing fields.
Successor: trajectory-aware delegation surfaces, provenance-only
`spawned_by_run_id`, explicit `requested_by_*` metadata.

`H002` **Parent/child store residue** — durable schema and helpers normalize
control queries around `parent_loop_id`. Successor: trajectory- and
slot-scoped queries plus provenance-only spawn references.

`H003` **Researcher parent-target routing** — findings routed by dereferencing
`ParentRunID`. Successor: stamp requester identity when the obligation is
created; route by addressed update or owning work item.

`H004` **Trace/verifier parent topology** — trace and verifier treat
`ParentRunID` as live causal structure. Successor: derive causality from
`trajectory_id`, work items, `requested_by_*`, co-super slots, and update edges.

`H005` **Work items modeled as spawned children** — `spawned_child_run`,
`spawned_child:` fingerprints, child-objective reasoning. Successor: assigned
trajectory obligations with `requested_by_agent_id` / `requested_by_run_id`.

### Continuation Residue

`H006` **Live continuation runtime** — first-class continuation control plane
(selection, compaction-before-handoff, lease clamping, dedupe by source run).
Successor: work items + passivation evidence + update-driven warm/cold actor wake.

`H007` **Continuation product path** — `/api/continuations/*` blessed in
contracts, allowlists, and handlers. Successor: work-item- and trajectory-based
routes, or temporary `410 Gone` shims during cutover.

`H008` **Continuation acceptance semantics** — acceptance and trace treat
continuation events as proof of progress. Successor: acceptance pivots to
passivation checkpoints, open work items, rewarm evidence, and trajectory
settlement; `continuation-level` is retired (done in acceptance.go).

### Tool Forcing And Texture Agency Residue

`H009` **Generic required-next-tool trust channel** — any tool result emitting
`next_required_tool` / `next_tool` forces exact next-step behavior. Successor:
typed, allowlisted continuation envelopes for bounded mechanical transitions.

`H010` **Texture semantic delegation forcing** — `edit_texture` can require
`spawn_agent` after a canonical write. Successor: `edit_texture` stores the
revision and stops; Texture decides its own semantic delegation.

`H011` **Super as direct ingress for Texture-centered work** — conductor routes
ordinary user/source/article/mission work to super by prompt heuristics,
bypassing Texture-owned artifact state. Successor: conductor creates or
resolves the Texture artifact; Texture owns the artifact and then decides
whether to revise, transclude, research, send a typed execution request to the
persistent Super, or wait.

`H012` **Researcher intent by substring oracle** — narrative text containing
"researcher" acts as control-plane signal. Successor: structured intent metadata
or explicit Texture-authored delegation state.

### Acceptance And Authority Residue

`H013` **Acceptance overclaim** — staging-smoke accepted states read as stronger
proof than they are. Successor: explicit smoke vs architectural vs settlement
evidence classes with hard reporting discipline.

`H014` **Continuation-level without compaction** — retired with
`continuation-level`; successor is trajectory/work-item settlement evidence.

`H015` **Agent-scoped residency short-circuit** — resident-run reuse skips
trajectory-scoped obligation delivery. Successor: if the actor is resident,
inject the new work item or update into its durable mailbox path.

`H016` **Agent-wide active-run fallback** — cancellation and super-controller
provenance fall back to latest-active-run selection. Successor: resolve through
resident activation or trajectory/work-item/slot authority; stamp requester
provenance at dispatch time.

### Durable Obligation Residue

`H017` **Blockers/questions not durable as obligations** — meaningful coordination
objects stay narrative-only. Successor: blockers/questions that matter for
settlement, re-entry, or supervision become durable obligation state.

`H018` **Assignment semantics not universally materialized** — assignment
updates do not always create durable work items. Successor: transactional update
append plus work-item materialization for assignment-class messages.

### Naming And Doctrine Residue

`H019` **Lease vocabulary drift** — docs/contracts keep lease language although
v1 rejects lease as control. Successor: activation caps, budget, worker handle,
trajectory obligation, explicit evidence classes.

`H020` **Mixed current/target onboarding** — foundational docs give legacy
surfaces apparent authority. Successor: sharp separation between live surfaces,
target doctrine, and explicitly retired ontology.

`H021` **Stale or self-contradictory doctrine** — assertions remain live after
code or newer doctrine falsifies them. Successor: assertions die when their
axioms die; doctrine updates are part of architecture missions. (This revision
of this document is itself the repair action for this heresy class.)

### Multi-Step Forcing And Polling Residue

`H022` **Forced multi-step worker delegation script** — delegation returns
scripted observe/finish/cancel next-tool chains. Successor: narrow mechanical
envelopes only; semantic worker progress is durable evidence and obligations.

`H023` **Synchronous control-plane polling** — foreground code polls internal
worker run state to a terminal condition. Successor: durable work items, updates,
evidence handles, and wakeable actors.

`H024` **Texture first-tool forcing by keyword oracle** — prompt keywords force
Texture's initial tool to `request_super_execution`. Successor: Texture receives
owner intent and affordances, then chooses its next semantic move.

`H024a` **Trivial first patch as hidden work-state** — Texture's first write
removes the owner's instruction without recording that research or execution is
underway. Successor: the first response is an honest canonical revision —
substantive output, or an acknowledgement/work-state revision naming active
background work.

`H024b` **Model-invented coagent update IDs** — model-authored `update_id`
strings can collide owner-wide. Successor: runtime mints or derives `update_id`
from the delivery envelope plus normalized payload (implemented; see I2c).

`H025` **Dead parent/child result-channel API** — `PostChildResult`,
`WaitForChildResult`, etc. with no production callers. Successor: delete; keep
only trajectory/work-item/update semantics.

`H026` **Prompt-pipeline forcing** — prompt defaults mandate specific semantic
workers as a required sequence. Successor: prompts describe obligations,
authority, evidence, and affordances; they do not mandate role choreography.

### Retired App Surface Residue

`H027` **Trace app residue** — Trace presented as a user-facing desktop app or
manual navigation destination. Successor: trace evidence APIs, run bundles,
acceptance records, and machine-readable causal ledgers; no Trace desktop app.

`H028` **Raw Terminal app residue** — Terminal as a user-facing app or ordinary
manual shell workflow. Successor: singleton Super Console per user computer,
backed by zot; PTY terminology only as hidden implementation detail.

`H029` **Browser as source-gathering app residue** — Browser as the default
source reader. Successor: Texture source marker -> transcluded expansion -> Source
Viewer/reader -> explicit Web Lens live/original inspection.

### Repaired, Detector-Retained

`H030` **Actor runtime database polling** — **repaired 2026-06-27**.
`internal/actor/actor.go` uses `mailbox chan Update`; the log is queried only for
cold-start replay, post-drain overflow, and Sweep boot recovery. Detector remains
active against regression. The test: if there are no `chan` declarations in
`internal/actor/actor.go`, the heresy is present.

`H031` **Candidate computer modeled as VM identity** — **production route
identity repaired** by the audited-construction Definition phases B/D/F. The
detector remains active. The only self-development candidate is a frozen capsule
effect bundle; no route resolves to a VM/desktop identity.

## Banned Patterns

Agents must not introduce:

1. new `ParentRunID` or `parent_id` control reads;
2. new `spawned_child_*` work-item semantics;
3. new uses of `run_continuations` or continuation-shaped APIs for active control;
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
15. new actor runtime loops that poll the durable log as a delivery mechanism —
    the test is whether `internal/actor/actor.go` contains `chan` declarations
    and the warm loop `select`s on the channel rather than calling
    `log.Unprocessed` in a polling pattern;
16. new product routes, promotion records, or speculative-candidate bindings that
    treat a VM, desktop, mutable branch, package, lineage record, or
    `ComputerVersion` route as self-development candidate authority. The only
    self-development candidate is a frozen capsule effect bundle; vmctl routes
    only accepted checkpoints after an authorizing event.

## Product Path

The durable-computer convergence Definition completed 2026-07-24 delivered the
generic durable-work kernel: restart reconstruction, reducer-owned
settlement/cancellation, signed identity, and desktop/headless conformance, with
effects OFF as the **pre-gate resting state**. That is the substrate the product
path builds on. The M-gates from earlier doctrine revisions are superseded by
that convergence and by this product path.

The active effects Definition
(`choir-supervised-self-development-effects-2026-08-11`) does **not** flip a
global effects-ON boolean. After its rehearsal, decision-policy, and restore
gates pass, authority is effect-relative: every effect class runs only through
its predeclared multiagent consensus policy and audited actuator. Reversible
computer-local effects may use a lighter qualified quorum because restore can
bound the excursion. Irreversible external or shared effects remain inside the
autonomy window but require stronger ex ante evidence, narrower subject
binding, durable consequence receipts, and recovery by compensation or a new
forward action rather than fictional rewind. A human is one possible
policy-selected seat, never a universal approval gate. Restore remains
acceptance-fenced and scoped (VM-local + release; platform/cycle/host frontend
OUT unless a successor changes that). Until that Definition closes with
deployed proof, do not teach "effects remain OFF forever" as the destination —
teach it as the gate before policy-governed effects.

Product order follows the vision (choir-vision.md):

1. **The automatic computer.** Own the harness; make self-development and
   self-supervision one architecture. Standing trajectory state selects
   observations; plurality (harness-owned provider choice, gateway holds
   credentials host-side) preserves dissent. Capsule effects are the typed
   transaction on the audit log; correction is an ordinary write. This is the
   supervised self-development center, and it is the binding constraint to
   solve first.
2. **The World Wire.** Downstream of the artifact-and-provenance substrate, the
   Wire indexes the world as reported — contested, plural, evidence-bounded.
   `universal-wire` identifiers remain transitional until a rename mission.

Rule: an architectural mission is not settled merely because the replacement path
works. It settles when the replacement works and the named heresy set for that
mission is reduced. Discovery of new heresies is epistemic progress, not repair
progress; keep discovered, introduced, and repaired counts separate.

## Change Protocol

When changing architecture, doctrine, or mission structure:

1. name the conjecture delta;
2. name which invariant changes, if any;
3. name which heresy is discovered, reduced, introduced, or retired;
4. name the evidence class required;
5. refuse silent mode changes.

If a proposed change would alter the system from agentic to workflow, from
trajectory to run-tree, from durable obligation to narrative only, or from
promotion protocol to shortcut path, that change requires an explicit conjecture
and a human-reviewable doctrine update before code lands.

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
