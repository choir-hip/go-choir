# Choir Doctrine

## Status

Canonical doctrine and architecture control document. The durable-computer
convergence Definition completed 2026-07-24 is the current substrate authority
(see ACTIVE.md); the product path follows the vision (choir-vision.md).

It states what Choir is and what all agents optimize; the conjecture set; the
derived architectural invariants; the evidence semantics that bound claims; the
live heresy index; and the product path. It is normative; supporting docs
explain or justify it and do not override it. It is doctrine, not a granular
requirements spec. When it conflicts with a support doc, this document wins
unless the support doc is a newer explicitly promoted doctrine update.
Historical specs, reviews, MissionGradient reports and mission ledgers are
evidence; they do not silently override it.

Supporting docs:

- [choir-vision.md](choir-vision.md) — product direction.
- [current-architecture.md](current-architecture.md),
  [computer-ontology.md](computer-ontology.md),
  [runtime-invariants.md](runtime-invariants.md)
- [precommitment-records.md](precommitment-records.md) — the core
  learning-and-oversight mechanism (precommitment records, PSA). The Markdown
  [conjecture-assertion-ledger-2026-06.md](conjecture-assertion-ledger-2026-06.md)
  is its historical ancestor, not the live ledger.
- [memo-per-computer-frontend-2026-08-13.md](memo-per-computer-frontend-2026-08-13.md)
  — computer-surface frontend is per-computer; host SPA is non-conformance.
- [why-texture-2026-06-15.md](why-texture-2026-06-15.md),
  [texture-agentic-invariants-2026-06-13.md](texture-agentic-invariants-2026-06-13.md),
  [source-external-data-publication.md](source-external-data-publication.md)
- [heresy-detectors.md](heresy-detectors.md) — executable detector manifest.

Reading order: this document; AGENTS.md for operating procedure; the relevant
domain invariant doc; the current goal file (throughline format); historical
reviews and proof artifacts as evidence only. Doctrine prose is being replaced
by executable enforcement (invariants toward model-checked specs, heresies toward
CI detectors); a heresy entry without a detector is not yet done being written.

## System Thesis

Choir is a human-improving, machine-compounding mainframe: a persistent-computer
system for owned learning over versioned artifacts, evidence, provenance, and
promotion history. "Self-improving mainframe" is historical shorthand; the
precise claim is that the improver is the person — the human supplies the
off-distribution judgment — and the system is the compounding memory that
accumulates that judgment as durable owned state surviving model churn.

The foundational argument is
[signal-is-sparse-not-the-learner-2026-08-01.md](signal-is-sparse-not-the-learner-2026-08-01.md):
sample inefficiency is undirected learning, not architecture; signal density
comes from a learner with standing questions and from correction by genuinely
independent others; both must be built into the environment the intelligence
runs in. Choir is that environment — the tape is standing state, the harness is
owned, plurality is architecturally available, correction is an ordinary write.
Supervision is the binding constraint on the frontier, and this system is the
architecture supervision lives in.

The core learning-and-alignment mechanism is **precommitment records**
(prospective self-alignment, PSA; [precommitment-records.md](precommitment-records.md)):
before a step with a verifiable outcome an agent commits a typed prediction; the
outcome resolves and scores it; the scored log is the agent's evolving world
model. Records live on the tape and in each desk's REPL, so feedback teaches in
context without modifying weights; they make less-verifiable work scoreable; and
the same records, viewed by role, are the organizational supervision surface.
One mechanism serves learning (context packs), alignment (procedural fidelity)
and audit.

The primary optimization target is not chat quality, local test passage, or
short-term product smoothness. It is, in order: truth from facts; correct
ontology; recognition of heresies; durable causality; evidence-bounded claims;
deletion of heretical legacy control paths; safe self-improvement by typed
conjecture. Product surfaces are downstream projections and falsifiers of the
substrate, not substitutes for it.

## Doctrine Of Doctrine

The first best thing is to recognize a heresy: to see and name a real flaw in
code, docs, tests, prompts, product path, or process. A newly recognized heresy
can make the system look worse by raising the open count, but epistemically it
is progress; invisible debt cannot be optimized. The second best is to
eliminate a named heresy: delete the bad path, fix the code, invert the test,
update the docs. The worse move is to preserve a clean story by hiding evidence,
refusing to name a flaw, shipping around it, or treating product motion as a
substitute for contact with facts.

Heresy accounting has three separate deltas: `discovered` (flaws newly
recognized from facts), `introduced` (new bad paths created by the current
change) and `repaired` (named heresies reduced or eliminated). Discovery is
epistemic progress but never repair; a mission regresses if it increases
`introduced` without an explicit conjecture delta and human-readable acceptance
of that debt.

## Framing Doctrine

Current framing: Choir is a human-improving, machine-compounding mainframe made
of persistent computers. Older framings — personal writing system, publishing
system, AI workspace, autoputer, workflow app, StoryGraph app, chat interface —
are historical, surface-specific, or deprecated unless promoted here; where they
reappear below they are detector vocabulary or historical evidence, not
endorsed naming. Framing drift is doctrine drift: a document, prompt, test, or
UI label that teaches an older product story pulls code back toward the old
ontology, so reconciliation must include narrative alignment, not only symbol
deletion.

Naming: product names are Autopaper, Autoputer and Autoradio, never camel case.
"World Wire", "Universal Wire" and "sourcecycled" are retired names for
Autopaper; code identifiers and routes may still say `universal-wire` until the
code rename lands (`docs/autopaper-sw-design-2026-10-09.md` §9), which is
transitional, not endorsed framing. The shared graph of published objects that
computers adopt onto their own tapes is **the Autopaper object graph**.

Preferred vocabulary: persistent computer; stable `ComputerID` and canonical
computer event chain; durable artifact, ComputerVersion (CodeRef,
ArtifactProgramRef) as **code/artifact identity** at an event head — not alone a
complete restore address; trajectory and work item; evidence, provenance,
verifier contract, acceptance class; capsule (ephemeral effect chamber) and
capsule effect bundle (frozen speculative effects, never a VM or route);
materializer, acceptance, projection, and **event-derived restore**
(acceptance-fenced return; prefer "restore" over "rollback" for product undo —
git revert is a separate repo operation); per-computer **computer surface**
(Desktop, Texture, apps, Settings panes, asset graph) versus thin **platform
shell**, with a **serving join** required before route CAS when UI is in the
restore set. Do not make personal workspace, AI workspace, publishing system,
autoputer, workflow, chat, StoryGraph, or demo app the root frame unless the
sentence is explicitly about a surface.

## Conjecture Set

Each conjecture is tagged `asserted` (current doctrine), `active` (under
construction), or `hyperthesis` (a named blind edge).

### Object-Level Conjectures

`C1 asserted` Choir's primary product object is a persistent computer composed
of multiple ledgers, not a disposable autoputer and not a chat session.

`C2 asserted` Canonical user-facing truth is versioned artifact state. Texture is
the canonical document and artifact control-plane core; other appagents own
their own typed artifact domains.

`C3 asserted` A persistent computer is identified by stable `ComputerID` plus
its canonical event chain. Risky or long-running mutation *effects* execute in
capsules and remain inert as frozen effect bundles until an authorized
acceptance event. Accepted state is materialized into guest releases and
ComputerVersion code/artifact identities at event heads; vmctl may project a
checkpoint into a serving route, but neither materialization, checkpoint
publication, nor route CAS is promotion authority. A speculative
self-development candidate is never a VM, desktop, mutable branch, package,
lineage record, or candidate route.

`C4 active` Autopaper, publication, review, and later economic surfaces are
projections of the artifact-and-provenance substrate, not independent product
ontologies.

### Meta-Level Conjectures

`C5 asserted` Roles are authority envelopes, not identities. Actors get
obligations, evidence, scope, settlement criteria, an assignment-scoped module
manifest, and revocable activation capabilities — not persona-heavy workflow
scripts or ambient privilege derived from a role name.

`C6 active` The runtime converges on durable actors whose accountable identity
and relationships survive passivation while each wake runs in a disposable
private activation. The database and trajectory remember, Go delivers, and the
activation model incrementally authors and executes Go against
assignment-scoped modules. Interpreter heaps, goroutines, channels, and
subprocesses are activation-local and never restored across rewarm.

`C7 active` Trajectories and work items are the causality and settlement model.
Settlement is rule-as-data over durable obligations and subject refs, not root
run completion. Consequential activation operations form a causally joined
evidence graph; any assignment, blocker, question, disagreement, verification
result, continuation, or disposition needed for settlement or rewarm becomes
durable workstream state, not only code, transcript, or Trace.

`C8 active` Promotion is an authorized semantic event with explicit proposal,
verification, freshness, privacy, acceptance, materialization, checkpoint,
projection, and rollback receipts. Exactly one per-computer event appender owns
semantic ordering; infrastructure projections and verifier claims cannot
acknowledge or settle the event.

`C9 asserted` Shared-platform claims are evidence-scoped. Staging, verifier
contracts, and owner review define the admissible strength of a claim.

### Meta-Meta Conjectures

`C10 asserted` Choir evolves by conjecture learning, not checklist completion.
Claims must name scope, test, and blind edge.

`C11 asserted` Self-improvement is stratified: object claims, architectural
claims, and method/doctrine claims are different levels and need different gates.

`C12 hyperthesis` Conjecture machinery can become decorative unless it changes
route choice, deletion pressure, evidence semantics, and stopping conditions.

`C13 active` Model-authored Go should become Choir's adaptive actor activation
language: a private executable medium for composition, multiagent
orchestration, and reusable organizational learning. Reused source carries no
authority; each activation resolves imports and receives fresh scoped
capabilities. Active until product-path evidence shows the common kernel can
replace persona-specific loops without reducing correctness, containment,
restart continuity, or supervision legibility.

`C14 asserted` Reversibility is a recovery property, not the boundary of
autonomy. Reversible and irreversible effects may both be authorized by a
predeclared, effect-specific multiagent consensus policy. The policy binds the
exact subject, eligible seats and independence domains, quorum, dissent,
evidence, capabilities, consequence receipts, and recovery or compensation plan
before participant outputs are visible. A human may be a required, optional, or
absent seat; human approval is not a universal effect gate. Trusted reducers and
actuators enforce the decision — model consensus does not become canonical state
authority.

`C15 asserted` A user's computer includes the UI that renders it. User-authored
frontend is computer surface, scoped by `ComputerID`, and takes the same
capsule-accept-materialize path as any other self-development change. A
host-global SPA is not a computer. Thin platform shell (TLS, Caddy bootstrap,
`/auth/*`, picker chrome, proxy/vmctl/corpusd, NixOS) may remain host software;
Desktop, Texture, apps, Settings computer panes, and the served asset graph may
not. Recording a frontend digest without a serving join does not satisfy this.
Synthesis: [memo-per-computer-frontend-2026-08-13.md](memo-per-computer-frontend-2026-08-13.md).

### Open Hyperthesis Edges

`HYP1` Settlement rules may be wrong in ways current vocabulary does not state.
`HYP2` Durable-actor cutover may hide control loss between agent identity,
trajectory identity, and rewarm semantics. `HYP3` Promotion semantics may be
below the computer ontology even when approval and freshness checks pass.
`HYP4` Verified harnesses do not imply verified cognition; residual semantic
risk remains when protocol gates pass.

## Derived Architectural Invariants

Hard consequences of the conjecture set.

`I1` Canonical document versions have two writer classes. `AuthorUser` is the
owner-edit path and performs an immediate canonical-head CAS. `AuthorAppAgent`
is the Texture desk and the sole *agent* writer; the management, engineering,
and research desks, workers, and runtime updates never write document text
directly. Findings, worker updates, search results, and verifier output are
non-canonical until Texture incorporates them through an AuthorAppAgent
revision. Every revision is a new monotonic, self-contained snapshot of current
semantic state; prior versions are never required context. A Texture-agent turn
that changes semantic state commits exactly one new version; wait/block/no-change
turns commit none. `texture_turn_committed` counts turn outcomes, not revisions.

`I2` Texture must not be forced into semantic delegation. Runtime may expose
affordances and durable obligations; it must not convert role mentions or
metadata into a required semantic next step.

`I2a` Exogenous user and source input enters Choir through Texture-owned
artifact state by default. Prompt-bar requests, source/news ingestion, article
creation, mission work, and most user prompts must not route directly to the
management desk. Management is downstream coherence and admission authority:
when an artifact needs coding, privileged execution, candidate work, generation,
or verification, Texture communicates by semantic act and management may issue a
delegated `choir.Cast` to engineering.

`I2b` Texture must make owner-triggered work visible as artifact state. For
prompt-bar input, `V0` is the owner prompt and `V1` is Texture's first response.
For an existing user-authored Texture, the current user revision is already
canonical; the next Texture revision may be a substantive edit, draft,
acknowledgement, work-state note, blocker, or plan. Forbidden is a mechanically
forced trivial patch that hides ongoing delegation or background work.

`I2c` Agent-to-agent semantic-act identity is runtime-owned: the runtime mints
or derives an idempotency identity from the delivery envelope and normalized act
payload; an LLM must not have to invent it. Model-visible acts (`choir.Report`,
`Cast`, `Ask`, `Precommit`, `Resolve`, `Cancel`, `Escalate`, `Note`) are
precommitment-record acts describing findings, evidence, refs, blockers, and
questions; their runtime identity is not semantic content.

`I3` Parent/child is not a control ontology. Provenance-only spawned-by edges may
remain temporarily, but control, liveness, settlement, cancellation, budgeting,
and recovery must not depend on parent/child semantics.

`I4` Work items are trajectory obligations, not child-run artifacts. They may
record who requested them, but their meaning is assignment-on-trajectory, not
descent-from-parent.

`I5` Dual paths are bugs. A replacement path does not settle a mission while the
old path remains available for new accretion unless that residual path is
explicitly frozen, gated, and on a named deletion clock.

`I6` No new dependencies on a live heresy. Existing dependencies are debt; new
ones are regressions.

`I7` If a blocker, assignment, question, or verification result matters for
settlement or rewarm, it must become durable obligation state rather than
remaining narrative or trace text.

`I8` Acceptance names must not outrun evidence class. Smoke, architectural,
export, promotion, and settlement proof must be distinguished.

`I9` Shared-platform behavior claims require staging truth. Local proof is
insufficient for vmctl, auth/session renewal, provider behavior, promotion,
rollback, or Choir-in-Choir claims.

`I10` Architectural mode changes require an explicit conjecture delta. An agent
must not silently pivot the system from agentic to workflow, from trajectory to
run-tree, or from promotion protocol to shortcut behavior to satisfy a probe.

`I11` Problem documentation comes before behavior-changing fix commits for new
reliable failures.

`I12` Supporting docs and tests must not normalize retired ontology.

`I13` Trace, Terminal, and Browser are not normal user-facing product apps.
Trace is an evidence substrate. Raw Terminal is retired, and so is the Super
Console/zot repair surface that replaced it (removed 2026-10-22,
`docs/problems/remove-zot-2026-10-22.md`). Manual Browser is replaced in the
source path by Source Viewer/reader artifacts plus explicit Web Lens
live/original inspection.

`I14` Source evidence remains object identity, not link-shaped prose. Texture and
successor surfaces represent sources as durable source entities and
transclusions. Clickable URLs, markdown web links, footnote prose, source-handle
inventories, or "Source:" lines are not acceptable substitutes for source-backed
claims.

`I15` Source citation is tri-state; citation shape is a display mode, not a node
type. Every source entity is cited (`source_ref` in the body), toolbar-only (a
Style.texture source that shapes the document but is not cited), or
marked-unused (`mark_source_unused` with a rationale in revision metadata). No
source is silently ignored. `display_mode` (`numbered_ref` or `expanded_ref`) is
a reader-toggleable presentation choice on the same node. Style textures are
source entities in the toolbar, not body citations.

`I16` Prompts provide data and invariants, not boolean control flow. A prompt
names the style texture, available sources, run context, and invariants; it must
not branch on runtime metadata to switch behavior. Such decisions belong in the
style texture, the run context, or tool availability.

`I17` The actor execution unit is durable trajectory/workstream plus accountable
actor plus disposable private activation. No interpreter process, heap,
goroutine, channel, or subprocess is durable actor identity or
settlement-critical memory.

`I18` Yaegi imports present an attenuated capability vocabulary; they grant no
authority. Every consequential operation is independently authorized by current
activation capabilities and a trusted broker. Model-authored code runs only in a
disposable guest-local capsule with resource and effect containment; Yaegi is
not the security boundary.

`I19` General process execution is absent by default; it may be mounted for an
effects-capable assignment, not inherited from a role name. Direct Bash and Go
execution calls use one capsule execution broker and one receipt and
transaction-tape path. Spawned processes never inherit Choir capabilities,
credentials, canonical-state access, or control sockets.

`I20` Persisted model-authored source is inert. Loading or reusing it never
restores an activation capability, live handle, interpreter heap, or prior
authority; imports, handles, heads, policy, and operations are resolved and
authorized anew.

`I21` Goroutines and channels express ephemeral computation. Independent
responsibility, communication, waiting, recovery, and supervision use durable
assignments, authenticated typed messages, work items, and obligations. An
activation parks rather than staying alive to wait for another actor.

`I22` Every model-authored Go cell and consequential capability boundary
crossing produces immutable causal evidence: shell/process execution,
assignment, agent messaging, source and artifact access, capability requests,
verification, effect-bundle freezing, continuation, and outcome receipts. This
is a normalized orchestration graph, not instruction- or syscall-level
surveillance.

`I23` The host derives the complete salient receipt set for supervision; an actor
cannot hide a consequential action or dissent by omitting it from a report.
Host-owned privacy policy removes credentials and unauthorized private content
without letting actor-controlled redaction erase receipt identity, event class,
disposition, or the existence of inconvenient evidence. Supervisors disposition
and explain the admissible evidence. Texture may transclude exact immutable
excerpts into canonical versions, but Trace, receipts, reports, and
transclusions remain evidence inputs and never acquire Texture or canonical
computer-event authority.

`I24` A capability available through an actor's Go modules must not remain
available through a parallel ambient model-tool path. Restricted profiles use
the private Go activation only; an effects-capable implementation assignment may
additionally receive direct Bash as a view of the same capsule execution broker.

`I25` The UI that renders a user computer is that computer's surface. Restore
restages that surface. The platform control plane may version independently.
Checkpoint creation fails closed unless served UI identity is derivable from the
bound release or an explicit frontend join; vmctl route CAS greens only after
that serving join. CI pointer rotation and a host `frontend-current` tree are not
computer changes. The current host-global SPA is non-conformance, not ontology.

`I26` Each computer has persistent root RLM desks of two kinds, each running
yaegi Go cells in killable subprocesses. System desks: management, engineering
and research. App desks: one per app agent; Texture is the first, then Mail,
Calendar, Slides and others as apps become app agents. An app desk ships with
its app package and is the sole agent writer of its app's object kinds
(Texture: document text). Research has read-only world and message authority;
engineering mutates only through capsule-bound per-assignment sub-RLM cells;
management supplies computer-wide coherence/error correction, resource
arbitration, delegated-`choir.Cast` admission into engineering, and owner
escalation, holds no document/event mutation authority, and is not a
concurrency limiter. Desks reach each other only by semantic act.
(Owner-ratified 2026-10-21, owner notes §36. Until S7-0 packages the existing
apps, Texture is the only app desk and is not yet packaged.)

The management admission ledger permits one live engineering assignment per
computer in the initial sequential phase: each request carries a durable
computer-scoped arrival ordinal, selection is FIFO among non-expired requests,
requests expire on operation terminality / superseding owner correction /
deadline, assignment deadlines fail rather than hang, and admission refusal is
retryable with work left pending. An assignment may hold N capability-bound
capsules (the transitional implementation binds one). Containment is
`memory.high` = requested, `memory.max` = 2×requested, with `memory.events` OOM
counters as feedback; PSI pause/resume and zram are not required initially.
Parallel Textures/trajectories and N concurrent assignments require an
admission-ledger release gate after sequential correctness. The commitment
ledger is the management supervision surface: its objects live on the tape,
reports resolve commitments, and scores stay off the acting desk's context.

`I27` One object graph per computer, and one home per kind of data. Every
meaningful kind of data a computer holds is an object graph kind with exactly
one writer, and lives in one place: on the tape, in replay and restore, and
reachable by a desk through a verb. Each kind belongs to the package that
declares it (the platform, an app package, or the computer itself), and its
schema travels with that package. The substrate stores, replays and restores
every kind without knowing it; an update changes only the kinds it owns, and
leaves every other kind byte-identical under replay. A user customizes a kind
by extension only: new fields, kinds and edges in their own namespace that
point at the kind, never a restructuring of another package's kind. A
different app is a new app in its own namespace, not a fork of an existing one;
it may declare kinds for genuinely new data, but data that already has a kind
stays on that kind, and the new app reads it, extends it, and writes it through
the owning package's verbs. A copy of a kind's data into a second kind, table,
file convention or browser store is a second home; a one-time import that
empties the old home is a migration. A file is an object: a blob in the
computer's content-addressed store, referenced by digest, plus a path view; the
files tree is a view, never a home for app data or configuration, and a file
format (`.ics`, `.pptx`) is an import or export of a kind, not its storage.
Named exceptions, kept few: secrets and keys (their own vault); the tape and its
index (the graph is the tape's projection); platform control and routing shared
across owners; raw store files and recovery logs; rebuildable caches, derived
indexes, unsent drafts and per-device UI choices; source, build and capsule
state (`docs/computer-ontology.md`); blob bytes themselves (one CAS per computer
for owner bytes). The Autopaper object graph and a computer's graph are one
model with two scopes and different restore rules, and an organization sharing
one graph under RBAC is a third scope, not a second model. Mail's transport
stays a host service with its own security boundary; its messages are still a
kind. (Owner-ratified 2026-10-24, ledger 19:0x to 19:2x; inventory and migration
order in `docs/problems/data-has-more-than-one-home-2026-10-24.md`. Kinds are
compiled into the platform registry today, so package-declared kinds are the
first substrate this needs; current staging is non-conformance, `H038`.)

## Proof-Carrying Autonomy

Autonomy increases at the mutation layer only when accountability increases at
the conjecture/evidence layer. A stronger agent carries a stronger proof object,
not merely more speed.

Mutation classes:

- `green`: docs, comments, labels, prompt/default text with no runtime change.
- `yellow`: tests, detector manifests, or prompt framing that change what future
  agents optimize but not product behavior directly.
- `orange`: runtime behavior, product APIs, app state, database queries, or
  provider/model routing.
- `red`: protected surfaces — Texture canonical writes, Trace/evidence
  semantics, canonical event acceptance, capsule effects,
  materialization/checkpoint/route projection, rollback, auth/session renewal,
  vmctl, gateway/provider calls, run acceptance, and deployment routing.
- `black`: irreversible or production-destructive operations. These require the
  strongest predeclared authority policy, qualified independent consensus, exact
  subject binding, durable consequence evidence, and a recovery or compensation
  plan before execution. A human seat is required only when that policy says so.
  Repository-agent safety rules may separately require human authorization for
  destructive maintenance; do not generalize that operator boundary into product
  ontology.

Before an orange or red change lands, the mission names the conjecture delta,
affected protected surfaces, admissible evidence class, rollback path, and
whether it discovers, introduces, or repairs heresy. If the fix requires
weakening a protected invariant, the invariant change is the mission.

Evidence packet: mutation class and protected surfaces; claims and the evidence
class of each; tests, probes, staging/deploy identity; rollback refs or precise
blocker; heresy delta; conjecture delta and remaining blind edge; residual risks
and a short human-learning digest.

## Evidence Semantics

Claim classes: `docs-level` (doctrine and design only); `staging-smoke-level`
(narrow product-path proof that a surface opens or a minimal path executes);
`export-level` (transferable candidate/source evidence exists);
`promotion-level` (policy-authorized promotion, materialization, and recovery
evidence, including the exact consensus decision receipt); `settlement-level`
(trajectory/work-item settlement evidence for the mission; a doctrine class
synthesized from trajectory and work-item state, not a Go constant).

Code conformance: `internal/types/acceptance.go` exposes `docs-level`,
`staging-smoke-level`, `export-level`, and `promotion-level`;
`continuation-level` is retired. Do not infer implementation of a stronger class
from this taxonomy.

Rules:

1. `accepted` at staging-smoke level must not be summarized as architectural
   success.
2. Every acceptance claim names its evidence class in reports.
3. A source-evidence acceptance claim must prove source entity / transclusion
   behavior. A visible web link or source list is negative evidence for it unless
   explicitly outside the artifact's source/citation path.
4. A weaker evidence class can falsify a stronger claim, but cannot satisfy it.

## Live Heresies

The executable manifest [heresy-detectors.md](heresy-detectors.md) holds the
detector inventory and is authoritative for detector vocabulary. This section is
the doctrine index: each heresy and its successor.

Reduction rule: a heresy is `reduced` only when its detector count decreases or
explicit non-countable evidence shows the bad pattern can no longer be used. A
replacement path working is not reduction while the old path remains available.
Discovery of a new detector or uncited site is epistemic progress, not repair.
Missions touching doctrine, runtime control, Texture, Trace/evidence, promotion,
source/Web Lens, or app-state ontology report the three deltas separately.

**Parent/child and spawn residue** (successor: trajectory, work-item and slot
scoped control; provenance-only `spawned_by_run_id`; explicit `requested_by_*`;
see `I3`, `I4`). `H001` `parent_id` / `parent_loop_id` / `ParentRunID` as
control-facing fields. `H002` store schema normalizing control queries around
`parent_loop_id`. `H003` findings routed by dereferencing `ParentRunID` (stamp
requester identity at obligation creation instead). `H004` trace/verifier
treating `ParentRunID` as causal structure (derive from `trajectory_id`, work
items, update edges). `H005` work items as `spawned_child_run`. `H025` dead
`PostChildResult` / `WaitForChildResult` API (delete).

**Continuation residue** (successor: work items + passivation evidence +
update-driven wake). `H006` first-class continuation control plane. `H007`
`/api/continuations/*` in contracts and handlers (work-item/trajectory routes or
temporary `410 Gone` shims). `H008` continuation events as acceptance proof
(passivation checkpoints, open work items, rewarm evidence, trajectory
settlement). `H014` continuation-level without compaction (retired with
`continuation-level`).

**Tool forcing and Texture agency residue** (successor: Texture receives intent
and affordances, then chooses; see `I2`, `I16`). `H009` `next_required_tool` /
`next_tool` forcing (typed, allowlisted envelopes for bounded mechanical
transitions only). `H010` `edit_texture` requiring `spawn_agent`. `H011` a
router sending ordinary user/source/article/mission work to super by prompt
heuristics, bypassing Texture-owned artifact state. `H012` substring
"researcher" as control signal (structured intent or Texture-authored delegation
state instead). `H022` scripted observe/finish/cancel next-tool chains. `H024`
prompt keywords forcing Texture's first tool. `H024a` Texture's first write
erasing the owner's instruction without recording work underway (an honest first
revision instead — substantive, or a work-state acknowledgement). `H024b`
model-invented `update_id`s (repaired: runtime-derived, `I2c`). `H026` prompt
defaults mandating workers as a required sequence.

**Acceptance, authority and obligation residue.** `H013` smoke-accepted states
read as stronger proof (explicit evidence classes, `I8`). `H015` resident-run
reuse skipping trajectory-scoped obligation delivery (inject into the actor's
durable mailbox path). `H016` latest-active-run fallback for
cancellation/provenance (resolve through resident activation or
trajectory/work-item/slot authority). `H017` blockers/questions narrative-only
and `H018` assignment updates not always creating work items (durable obligation
state with transactional work-item materialization, `I7`). `H023` foreground
polling of worker run state (durable work items, updates, evidence handles,
wakeable actors).

**Naming and doctrine residue.** `H019` lease vocabulary (activation caps,
budget, worker handle, trajectory obligation). `H020` mixed current/target
onboarding (sharp separation of live, target, retired). `H021` stale or
self-contradictory doctrine (assertions die when their axioms die; doctrine
updates are part of architecture missions).

**Retired app surfaces** (`I13`). `H027` Trace as a user-facing app (trace
evidence APIs, run bundles, acceptance records instead). `H028` raw Terminal as
an app or manual shell workflow (the Super Console/zot that replaced it was
removed 2026-10-22). `H029`
Browser as default source reader (Texture source marker -> transcluded expansion
-> Source Viewer/reader -> explicit Web Lens inspection).

**Repaired, detector retained.** `H030` actor runtime database polling —
repaired 2026-06-27: `internal/actor/actor.go` uses `mailbox chan Update`, and the
log is queried only for cold-start replay, post-drain overflow, and Sweep boot
recovery; no `chan` declarations in that file means the heresy is present.
`H031` candidate computer modeled as VM identity — production route identity
repaired by the audited-construction Definition phases B/D/F; the only candidate
is a frozen capsule effect bundle and no route resolves to a VM/desktop identity.

**Open.** `H032` host-global computer surface — Caddy `frontend-current` (or any
one host SPA) presented as a user computer's UI, so a capsule UI change is a
platform deploy or a no-op and restore cannot return the surface the user
operates. Successor: per-computer computer surface under `C15`/`I25`; thin
platform shell only; serving join before route CAS. Current staging is the
heresy; add no zero-count CI detector that fails the live hop before the
successor lands.

**Second homes for data** (`I27`). `H038` a kind of data with more than one
home or none of the graph's: app data or configuration in the files tree by
convention (the model policy, Calendar `.ics`), owner data outside the computer
(Mail in maild's SQLite), direct-SQL tables off the tape (`agent_evidence`,
`co_super_slots`), a file named by path strings that delete and rename do not
update, registered kinds with no writer beside live tables holding the same
data, per-package key-value silos, and browser storage as app state.
Successor: package-declared kinds with one writer, the file object (blob by
digest plus path view) first, then the migration order in the data-homes
problem record. Current staging is the heresy; add no zero-count CI detector
that fails the live hop before the successor lands.

## Banned Patterns

Agents must not introduce (each is a new dependency on a live heresy, `I6`):

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
10. new blocker-or-assignment semantics that stay narrative-only while used in
    settlement reasoning;
11. new docs that normalize retired ontology without labeling it transitional;
12. new Trace desktop/app/dashboard surfaces;
13. new raw Terminal app affordances outside Super Console internals;
14. new Browser-as-source-gathering or URL-means-Web-Lens defaults;
15. new actor runtime loops that poll the durable log as a delivery mechanism
    (the test: `internal/actor/actor.go` contains `chan` declarations and the
    warm loop `select`s on the channel rather than calling `log.Unprocessed`);
16. new product routes, promotion records, or candidate bindings that treat a VM,
    desktop, mutable branch, package, lineage record, or `ComputerVersion` route
    as self-development candidate authority — the only candidate is a frozen
    capsule effect bundle, and vmctl routes only accepted checkpoints after an
    authorizing event;
17. new host-global product UI taught as the computer's surface, or a frontend
    digest recorded as restore completeness without a serving join (thin
    platform shell may remain host-global; Desktop, Texture, apps, Settings
    computer panes, and the asset graph may not).

## Product Path

The durable-computer convergence Definition completed 2026-07-24 delivered the
generic durable-work kernel — restart reconstruction, reducer-owned
settlement/cancellation, signed identity, desktop/headless conformance — with
effects OFF as the **pre-gate resting state**. The product path builds on that
substrate; earlier M-gates are superseded.

Self-development effects do **not** flip a global effects-ON boolean. Every
effect class runs only through its predeclared multiagent consensus policy
(`C14`) and audited actuator; the effect-policy shape — rehearsed,
decision-policy-gated, restore-fenced — is carried from the superseded effects
Definition (`archive/choir-supervised-self-development-effects-2026-08-11.md`)
as design evidence. Reversible computer-local effects may use a lighter qualified
quorum because restore bounds the excursion; irreversible external or shared
effects require stronger ex ante evidence, narrower subject binding, durable
consequence receipts, and recovery by compensation or a new forward action
rather than fictional rewind. A human is one policy-selected seat, never a
universal approval gate. Restore remains acceptance-fenced and scoped (VM-local
+ release IN; platform/cycle OUT). Computer-surface frontend is IN by
`C15`/`I25`; the host-global SPA is current non-conformance.

The self-development gate is sequenced by `docs/ACTIVE.md`, not here. Until it
passes with deployed proof, do not teach "effects remain OFF forever" as the
destination; teach it as the gate before policy-governed effects.
Self-development is only real when built on precommitment scoring (owner,
2026-10-14).

Product order follows the vision (choir-vision.md):

1. **The automatic computer.** Own the harness; make self-development and
   self-supervision one architecture. Standing trajectory state selects
   observations; plurality (harness-owned provider choice, gateway holding
   credentials host-side) preserves dissent. Capsule effects are the typed
   transaction on the audit log; correction is an ordinary write; precommitment
   records are the learning and oversight mechanism. This is the supervised
   self-development center and the binding constraint.
2. **Autopaper** (live since 2026-10-13), downstream of the
   artifact-and-provenance substrate: it indexes the world as reported —
   contested, plural, evidence-bounded.
3. **Autoradio**, the listening and voice surface over the same objects, after
   the architecture work.

Rule: an architectural mission is not settled merely because the replacement
path works. It settles when the replacement works and the named heresy set for
that mission is reduced.

## Change Protocol

When changing architecture, doctrine, or mission structure: name the conjecture
delta; name which invariant changes, if any; name which heresy is discovered,
reduced, introduced, or retired; name the evidence class required; refuse silent
mode changes. A change that would alter the system from agentic to workflow,
from trajectory to run-tree, from durable obligation to narrative only, or from
promotion protocol to shortcut path requires an explicit conjecture and a
human-reviewable doctrine update before code lands.

## Short Rule For Agents

Optimize the conjecture set of Choir, not merely the local tests. When in doubt:
preserve ontology over convenience; seek truth from facts before preserving a
nice story; name real heresies even when the count looks worse; prefer deleting
a heresy to adding a bridge around it; treat dual paths as bugs; do not let a
probe or test invent the architecture; document the problem before fixing it.
