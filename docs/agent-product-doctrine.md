# Choir Agent Product Doctrine

This file carries product architecture rules for agents working on Choir. It is
loaded on demand when a mission touches authority boundaries, harness behavior,
Texture, runtime configuration, product-path verification, or run acceptance.
Long-running missions execute as `/goal <doc>.md` goal files; new missions are
authored with [skills/throughline/SKILL.md](../skills/throughline/SKILL.md), and
existing goal files in the older format remain valid. The
[durable-computer convergence Definition](archive/choir-coherent-computer-convergence-2026-07-21.md)
completed on 2026-07-24 and is historical product-evidence authority. The
current executable mission is named only by [ACTIVE.md](ACTIVE.md); this file
does not create a second schedule.

It inherits [Choir Doctrine](choir-doctrine.md) and must not become a competing
doctrine source. `AGENTS.md` is the operating contract loaded every session;
this file is the deeper product-architecture reference. When they conflict,
follow Choir Doctrine unless `AGENTS.md` carries a newer explicitly promoted
operating update.

## Authority Boundaries

- The persistent root RLM desks are the system desks `management`,
  `engineering` and `research`, and one app desk per app agent, Texture first
  (doctrine `I26`, owner 2026-10-21). Each runs yaegi Go cells in a killable
  subprocess. A desk is durable for its subject; a sub-RLM is a per-assignment
  run that a desk casts.
- **Texture** is the sole agent writer. Canonical document versions have two
  writer classes: `AuthorUser` owner edits (immediate canonical-head CAS) and
  `AuthorAppAgent` Texture revisions. No other desk writes document text. Its
  scope is exactly two jobs: revise the human-readable document and communicate
  with other desks by semantic act. It has no capsule, host, provider-routing,
  event-chain, or promotion authority; humans interface only with Texture, and
  Texture renders the commitment ledger into the owner-facing supervision
  surface. Appagents own their own durable app artifacts.
- **Research** has read-only world and message authority. It reports evidence
  through `choir.Report`; it has no Bash, raw database access, writable files, capsule
  commit, acceptance, route, or host authority.
- **Management** is the exactly-one-per-ComputerID coherence, error-correction,
  and resource-arbitration root. It never mutates documents or computer events,
  is not a concurrency limiter, admits engineering work through delegated
  `choir.Cast` (admission ledger and memory containment: `I26`), and escalates
  unresolved owner decisions.
- **Engineering** performs capability-bound guest-local capsule mutation
  through per-assignment sub-RLM cells. Its effect verbs are `Complete`,
  `Freeze`, and `Verify`; no desk name grants ambient execution authority.
- Retired profile aliases fail closed for self-development.
- Verification is a read-only contract over evidence. It cannot append, accept,
  materialize, checkpoint, or route an event.
- The **commitment ledger** is the management supervision surface: commitment
  objects live on the tape, never in a third store; reports resolve commitments;
  scores stay outside the acting desk's context. Desks talk through
  precommitment records ([precommitment-records.md](precommitment-records.md)).

One stable `ComputerID` plus its canonical event chain is the evolving
computer. A frozen capsule effect bundle is speculative and inert. Canonical
desired state changes only by an authorized acceptance event; effective state
changes only after verified guest materialization.

## Current Invariants

- **Stable computer identity:** `ComputerID` plus its canonical event chain is
  durable identity. `RealizationID` is replaceable machine state.
- **One event authority:** exactly one trusted guest `ComputerEventAppender`
  validates and sequences semantic events. corpusd mechanically performs typed
  head CAS; Trace, trajectories, guest-store state, vmctl, route tables, status,
  checkpoints, and reducers are projections or actuators.
- **Host PostgreSQL, guest SQLite:** the host runs PostgreSQL with two
  databases, control (accounts, computers, sessions, key escrow, mail, the host
  side of the tape receipts) and graph (the Autopaper object graph and shared
  publications). Each user computer keeps its working state in SQLite
  (`state.db`) on its own disk, a projection rebuilt from the tape by replay.
  None creates a third semantic store or alternate head. Dolt, Store A and
  Store B were deleted 2026-10-30. See [computer-ontology.md](computer-ontology.md).
- **Acceptance before effect:** an effect bundle is inert until acceptance.
  Guest materialization, checkpoint publication, and route CAS cannot
  acknowledge or substitute for the event.
- **ComputerVersion code identity:** `(CodeRef, ArtifactProgramRef)` names the
  code/artifact published at an event head. It is not computer identity, not
  promotion authority, and — alone — not a complete restore address. The fuller
  event-head addressing plus VM-local content witness for restore completeness
  is carried by the carrier/ontology work; promote that binding to settled
  doctrine only after deployed restore proof.
- **Per-computer frontend:** the UI that renders a computer is that computer's
  surface (`C15`/`I25`). User-authored frontend changes are scoped by
  `ComputerID` and take the same capsule-accept-materialize path as other
  self-development. A host-global SPA (`frontend-current`) is current
  non-conformance. Thin platform shell (TLS, auth, picker chrome) may remain
  host software. Do not ship UI in the current effects candidate; the browser
  would not read it. See
  [memo-per-computer-frontend-2026-08-13.md](memo-per-computer-frontend-2026-08-13.md).
- **vmctl projection:** vmctl remains the sole route-slot CAS actuator and
  verifies exact accepted-event, checkpoint, materialization, verifier, and
  route-certificate joins. Post-genesis legacy route authority refuses.
- **No embedded-store promotion:** the obsolete tag/commit/reset
  promotion adapter is deleted; accepted-event materialization is the only
  self-development path.
- **Timeout hardening landed:** `vmctl.Client` defaults to 60 seconds and the
  server has bounded read/write timeouts (120-second defaults). Staging proved
  the induced resolve-failure path returns a bounded 504; re-prove after a
  routing or timeout change rather than reopening the old 180-second diagnosis.

Texture delegation is agentic. Texture may revise, ask the research desk, report
or ask the management desk, do several of those, do none, wait for more
evidence, or report a blocker within its authority envelope. `patch_texture` and
`rewrite_texture` store canonical revisions; neither may become a semantic
workflow gate requiring a subsequent research, management, verification, or other
appagent call. Exact required-tool continuation is reserved for narrow
mechanical protocols, not appagent policy.

Supervision is asynchronous and continuous. Texture may write many semantic
versions between owner reads while it receives intermediate reports and sends
revised direction to research or management. Capsule work, verification, and
durable operations return `choir.Report` evidence handles as they progress, not
only a terminal report. Worker-VM and candidate-VM delegation are obsolete and
deleted; delegated agents use durable runs/trajectories and capsules. Management
admits engineering through a delegated `choir.Cast` and its durable
assignment/trajectory and capsule-bound operation handle; a subordinate must not
reconcile competing supervisors or receive a capability from model-visible text.

Verifier agents are read-only with respect to canonical product state. They may
execute only in an independently provisioned read-only capsule whose
capabilities cannot commit, accept, materialize, checkpoint, or route effects.

## Private Go Activation And Capability Profiles

The desks converge on one durable-actor activation kernel. Each persistent root
desk runs a private, disposable Yaegi interpreter in a killable subprocess with
an assignment-scoped module manifest, current activation capabilities, bounded
observations, and a typed outcome contract. The model incrementally authors and
executes Go. Differences among desks belong in organizational bindings, module
profiles, policy, and outcomes, not persona-specific model loops.

Restricted desk activations expose only the Go-cell operation to the model.
Search, source fetch, document and file-format transforms, artifact operations,
delegation, messaging, evidence, and work state are narrow imported modules;
there is no duplicate ambient JSON-tool path. Backends may use compiled services
or isolated converters internally without exposing shell, raw HTTP, or general
filesystem authority to the actor. All desks use in-cell yaegi `choir.*`
semantic-act functions rather than a tool-call channel; the retired
`update_coagent` interface is absent for desks, and its packet machinery
survives only as `Report`'s body.

An effects-capable implementation assignment may additionally expose direct
Bash. Direct Bash and the Go `execution.Run`/`execution.Shell` surface are two
front ends to one capsule execution broker, transaction tape, cancellation path,
and receipt schema. Commands run inside the guest-local effect capsule; child
processes receive only capsule-local inputs and an explicit ordinary
environment, and must not inherit activation capabilities, broker or provider
credentials, canonical database access, sensitive descriptors, guest-control
sockets, or host authority.

Yaegi is an orchestration runtime, not a sandbox or authority source. An import
only makes an operation nameable. Every consequential module call validates the
current activation, actor, computer, trajectory, work item, operation, resource
budget, capability grant, and relevant state head in trusted compiled code. The
capsule supplies process, filesystem, network, resource, and effect containment.
Trusted reducers alone own durable shared state and external or canonical
effects.

Model-authored functions may be frozen as immutable source artifacts and reused
or revised. Source is inert: reuse never preserves a live handle, interpreter
heap, credential, activation capability, or prior authority; a later activation
resolves its module requirements and receives fresh capabilities. Repeatedly
successful source may become a reviewed orchestration recipe, but neither origin
nor reuse is promotion. Goroutines and channels are ephemeral activation
dataflow; durable responsibility and communication use assignments, work items,
authenticated typed messages, and obligations, and actors park with typed
continuations rather than retaining an activation while another actor works.

## Citable Orchestration Evidence

The runtime records exact Go-cell source and result identity plus every
consequential capability boundary crossing: shell/process runs, source and
artifact access, assignments, agent messages, blockers, disagreements,
capability requests, verification, effect-bundle freezing, continuations, and
outcomes produce host-authored causal receipts. This is a normalized
orchestration graph, not an instruction or syscall recording.

The host derives the complete salient excerpt population for supervision so an
actor cannot conceal an inconvenient command, refusal, failure, or dissent by
leaving it out of prose. Supervisory actors select, disposition, and explain
those receipts. Texture may cite and transclude exact immutable excerpts in
canonical versions. Reports and transclusions remain evidence projections: they
cannot append computer events, accept effects, or replace Texture's canonical
ownership.

## Harness Minimalism

Keep the agent loop programmatically uniform where authority permits: provider
call semantics, cancellation, retry, compaction, and durable trajectory
projection do not fork by persona. Capability resolution, privacy-safe canonical
event append, capsule isolation, and typed role policy are deliberate security
boundaries. Production actor roles are the system desks and the app desks
(explicitly bounded appagents); retired aliases refuse. Prefer prompts, tool descriptions,
capability policy, and product-visible state over role-specific harness
branches. If a proposed fix requires programmatic divergence in the core loop
for one role, document the evidence, the invariant protected, and the simpler
alternatives rejected, and obtain explicit human approval before landing it.
Divergence is acceptable only when it protects correctness, security, authority
boundaries, or resource isolation in a way policy or prompt contract cannot
express. This is a repository-maintainer exception for changing the shared
harness; it is not a product rule that a human must approve irreversible
effects (those follow the policy-governed consensus contract, doctrine `C14`).

The optimal-harness thesis (2026-08-31, distilled; the review is archived):

- **The baseline is the measuring stick.** A single model in a ReAct loop with
  no guardrails plus good restore can in principle self-develop. Every harness
  mechanism must beat that baseline at production scale or be deleted. The
  architecture is the baseline plus what scale demands: orchestration as
  model-written code (the RLM), a concurrency hierarchy of capsules, and
  heterogeneous-model supervision, which catches a model drifting into an
  attractor basin. Supervision is inspectable code in the pipeline, not a
  trusted hierarchy.
- **The harness supplies four things, exactly what models cannot supply for
  themselves.** (1) Durable state: event log, receipts, artifacts, identity
  (attributable receipts) and time (wakes, timers, deadlines), because models
  are stateless and cannot wake themselves. (2) Effect boundaries with
  quantitative risk controls: record every effect and refusal, bound each effect
  class (rate, spend, position, recipient limits), and revoke. Bounds limit
  magnitude, never establish correctness, so each irreversible class also
  pre-declares automated policy checks, idempotency keys, consequence receipts,
  and a compensation path; restore repairs the machine, never external reality,
  and wrong external effects are corrected forward. (3) Physical backstops: OS
  capsule death and resource ceilings, acting on exhaustion, never on semantic
  judgment. (4) Multiplexed grounding: all input (owner, external agents, feeds,
  later audio) arrives as one ordered stream, and interruption is a first-class
  state transition, not a cancellation. Nothing else.
- **Fail over, do not escalate.** Redundancy and supervision exist so the system
  keeps running and keeps being right. For every such mechanism ask "when this
  fires, what keeps running?"; a mechanism whose only product is a stop pending a
  human decision is mis-designed. A mechanical halt of one effect class with the
  machine continuing is a legitimate control. Human input is stream input and a
  policy-selected seat, never an in-loop blocking gate; human ratification is per
  effect class, not per effect.
- **One actuator surface.** Every agent acts through the same structured, gated,
  receipted interface; no raw-shell side door (`I19`, `I24`).
- **Agents are desks, not people.** Model calls are stateless and staffing
  scales, so a persona is a fiction the machine would have to maintain. A name
  must stay true at N=1 and N=∞ and must not require the machine to maintain a
  self.
- **Concept admission.** Prefer the standard term. A new concept enters only if
  no standard term covers it, it does load-bearing work in authority, evidence,
  privacy, or causality, and it is exercisable, testable, replaceable, and
  retirable.
- **Operative hypothesis, not doctrine:** a bespoke, out-of-distribution toolset
  (model-written Go over a receipted event log) breaks the trained
  assistant/eval frame and makes models act as agents whose effects are real.
  Tool-call failure telemetry is a strain gauge, analysis-only for now; the
  hypothesis is dropped if controls show no difference.

## Prompt Control-Flow Antipattern

Prompts provide data and invariants, not boolean branches that switch behavior
(doctrine `I16`). A prompt names the style texture, the available sources, the
run context, and the invariants (cite sources, no model priors as grounded,
canonical revisions via tools). It must not branch on runtime metadata
(`{{if .WireTexture}}`, first-owner-prompt special cases, worker-finding gates).
Unconditional invariant text is not control flow. Decisions that used to live in
prompt branches belong in the style texture, the run context, or tool
availability.

Source citation is tri-state and citation shape is a display mode, not a node
type (doctrine `I15`): every source entity is cited (`source_ref` in the body),
toolbar-only (a Style.texture style source), or marked-unused
(`mark_source_unused` with a rationale). All citations are `source_ref` with
`display_mode` (`numbered_ref` | `expanded_ref`); the former `source_embed`
block node is removed.

## Texture as Artifact Control Plane

Texture is also Choir's artifact control plane and the delegated controller for
long-running artifact trajectories. Exogenous user/app/source input enters
Texture-owned artifact state: prompt-bar requests, source/news ingestion,
article creation, mission work, and most user prompts open or create
Texture/context first (doctrine `I2a`). Management is not the direct ingress
target for ordinary user or source prompts, and management before Texture is a
route invariant failure. Texture may later send management a semantic act when
the artifact needs execution, coding-agent trees, generated artifacts,
verification, candidate work, or another privileged action; management after
Texture is valid only through a capability the runtime grants. Management admits
engineering with delegated `choir.Cast`; research and engineering return
`choir.Report` material as work advances. Texture incorporates what they teach
into idea-level versions and redirects the trajectory. The owner samples and
corrects the current head asynchronously rather than approving each version.

Read [texture-agentic-invariants-2026-06-13.md](texture-agentic-invariants-2026-06-13.md)
before changing Texture tools, prompts, routing, revision creation, desk wake
behavior, Trace/Texture projection, run acceptance involving Texture, or
missions that use Texture as their owner-readable narrative. Texture is the
canonical document/versioning core and must remain an agentic participant in a
multi-agent system, not a workflow runner. Runtime may expose affordances and
durable obligations, but must not force Texture to call research, management,
verification, or any semantic appagent merely because prompt text, revision
metadata, or an acceptance probe mentions that role.

## Runtime Configuration

Provider secrets and platform model catalogs are platform-owned. Per-computer
model policy is computer-owned durable **runtime configuration**, editable
through the product path as a config write. It is **not** the self-development
effect content axis: model-policy bundles are retired as the first
self-development proof (no apply path to resolution; the surface is system-owned
and scheduled for broker-mediated replacement). Do not teach agents that "the
computer changing itself" means editing model policy, and do not patch Node B
environment variables or tracked server files as a substitute for a runtime
policy path unless the mission is explicitly a platform config deploy.

Role defaults are policy defaults, not architecture. Any configured model may
serve a production desk or bounded role when its declared capabilities match the
current turn. Text-only models are valid for orchestration, research, coding,
and verification that need no media input; multimodal models are required only
when the turn needs screenshots, images, video frames, files, or other media. A
current policy mapping a desk to a given provider is the active computer's
effective policy, not a hard-coded role boundary; capability is evaluated for
the next turn, not permanently for the role. Do not add role-specific provider
assumptions ("management must be ChatGPT", "verification must be multimodal")
unless the turn's capability requirements imply them. The long-term target is
dynamic, agentically editable per-computer model policy: an owner prompt may ask
management to admit a capability-bound engineering change to the computer's
model policy, and subsequent runs use it without a platform deploy or Node B
edit. The platform catalog records model capabilities and provider request
semantics; per-computer policy selects among them. Provider request schemas
must preserve modality; when an adapter cannot resolve a media artifact, record
the blocker precisely.

## Product-Path Verification

Browser or Playwright acceptance may use public authenticated product APIs:

- `/api/prompt-bar`, `/api/prompt-bar/submissions/{id}`
- `/api/texture/*`
- `/api/trajectories/*` (lifecycle snapshot and events)
- `/api/runs/*` (run records and `/api/runs/{id}/events`, the owner-scoped
  run-event read; `choir trace export` assembles them into a trace bundle)
- `/api/current-computer`
- `/api/computers/*/self-development/*`
- `/api/continuations/*` (transitional H007/H008 residue; prefer
  trajectory/work-item evidence and add no new continuation-shaped acceptance)
- `/api/run-acceptances/*`

Do not use browser-public internal or test-only routes to bypass the product
path: `/api/agent/*`, `/api/prompts`, `/api/test/*`, `/internal/*`, or raw event
mutation endpoints. The verifier must observe product/control evidence and must
not manually seed success records.

## Run Acceptance Records

For long-running mission proof (`/goal <doc>.md`), the mission's own evidence
ledger and completion semantics govern what counts as settled — throughline for
new files, the Definition format for the existing corpus. `RunAcceptanceRecord`
is a historical evidence projection for older runs, not self-development
authority.

The completed convergence Definition's terminal receipt records exact
source/deploy/host/guest identity; artifact, subject/activation, obligation,
update-disposition, and work-settlement refs; restart/reconstruction and
cancellation traces; UI/headless protocol conformance; authority refusals;
rollback; mutation and heresy deltas; and residual risk. That receipt left
self-development effects OFF as a **pre-gate resting state**.

Do not claim deployed self-development from the superseded CTS Definition,
rejected Round 72 candidate, AppChangePackage/AppAdoption, RunAcceptance,
worker/candidate VM, local tests, a verifier statement, checkpoint publication,
or route transition. The self-development gate is sequenced by
[ACTIVE.md](ACTIVE.md); the superseded effects Definition
(`archive/choir-supervised-self-development-effects-2026-08-11.md`) is
historical evidence for the effect-policy shape, not the live path. Effects turn
on only through effect-specific multiagent consensus policies and audited
actuators, not a global ON boolean. Reversible effects gain a restore path;
irreversible effects require stronger policy, evidence, and consequence receipts
but do not categorically require a human decision; human participation is a
policy-selected seat.

`continuation-level` is transitional H008/H014 residue: the durable-actor
contract re-points this level at trajectory/work-item settlement evidence. Until
that cutover lands, it keeps its current meaning and evidence requirement — do
not weaken it, and do not claim trajectory settlement evidence in its place
before the level is formally re-pointed. Do not introduce new `continuation-level`
claims or APIs as doctrine; M4 must delete or explicitly shim the old surface.
