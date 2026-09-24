# Choir Agent Product Doctrine

This file carries product architecture rules for agents working on Choir. It is
loaded on demand when a mission touches authority boundaries, harness behavior,
Texture, runtime configuration, product-path verification, or run acceptance.
Long-running missions execute as `/goal <doc>.md` goal files. New missions are
authored with [skills/throughline/SKILL.md](../skills/throughline/SKILL.md);
the older `skills/definition/SKILL.md` format remains valid for the existing
goal-file corpus but is deprecated for new authoring. The
[durable-computer convergence Definition](archive/choir-coherent-computer-convergence-2026-07-21.md)
completed on 2026-07-24 and is historical product-evidence authority. The
current executable product mission is named only by `docs/ACTIVE.md`; this file
does not create a second schedule.
It inherits [Choir Doctrine](choir-doctrine.md) and must not become a competing
doctrine source.

`AGENTS.md` is the operating contract loaded every session; this file is the
deeper product-architecture reference. When they conflict, follow Choir Doctrine
unless `AGENTS.md` is carrying a newer explicitly promoted operating update.

## Authority Boundaries

- `conductor`, `processor`, and `reconciler` are deferred to world-wire/system-one.
  They are not part of the live four-desk topology; their legacy packet
  machinery remains until their respective phases.
- The persistent root RLM desks are `management`, `engineering`, `research`, and
  Texture. Each runs yaegi Go cells in a killable subprocess. A desk is durable
  for its subject; a sub-RLM is a per-assignment run that a desk casts.
- Appagents own durable app artifacts. Canonical document versions have two
  writer classes: `AuthorUser` owner edits (immediate canonical-head CAS) and
  `AuthorAppAgent` Texture revisions (sole agent writer). No other desk writes
  document text.
- The research desk has read-only world and message authority. It reports
  evidence through `choir.Report`; it has no Bash, raw Dolt, writable files,
  capsule commit, acceptance, route, or host authority.
- The management desk is the one-per-ComputerID coherence, error-correction,
  and resource-arbitration root. It does not mutate documents or computer
  events. It admits engineering work through delegated `choir.Cast` and
  escalates unresolved owner decisions.
- The engineering desk performs capability-bound guest-local capsule mutation
  through per-assignment sub-RLM cells. Its effect verbs are `Complete`,
  `Freeze`, and `Verify`; no desk name grants ambient execution authority.
- Retired profile aliases fail closed for self-development.
- Verification is a read-only contract over evidence. It cannot append, accept, materialize, checkpoint, or route an event.

One stable `ComputerID` plus its canonical event chain is the evolving
computer. A frozen capsule effect bundle is speculative and inert. Canonical
desired state changes only by an authorized acceptance event; effective state
changes only after verified guest materialization.

## Supervision Contract (2026-08-21)

- **Texture desk scope is exactly two jobs:** revise the human-readable document
  and communicate with other desks by semantic act. No capsule, host,
  provider-routing, event-chain, or promotion authority. Humans interface only
  with Texture.
- **Management desk singleton:** exactly one per ComputerID — coherence,
  error-correction, and resource arbitration over the whole computer; not a
  concurrency limiter and never a document/computer mutator.
- **Engineering admission and containment:** management admits one live
  engineering assignment per computer in the initial sequential phase. An
  assignment may hold N capability-bound capsules; the transitional
  implementation is 1:1. Requests have computer-scoped arrival ordinals, FIFO
  selection among non-expired requests, terminal/supersession/deadline expiry,
  failure rather than hanging deadlines, and retryable refusal with work pending.
- **Memory containment:** `memory.high` = requested, `memory.max` =
  2×requested, `memory.events` OOM feedback; no PSI pause/resume or zram.
  Later parallel Textures/trajectories and N assignments require an
  admission-ledger release gate after sequential proof.
- **Commitment ledger:** commitment objects live on the tape, never in a third
  store. Reports resolve commitments; scores remain outside the acting desk's
  context and form the management supervision surface.

## Current Invariants (2026-07-08)

- **Stable computer identity:** `ComputerID` plus its canonical event chain is
  durable identity. `RealizationID` is replaceable machine state.
- **One event authority:** exactly one trusted guest `ComputerEventAppender`
  validates and sequences semantic events. corpusd mechanically performs typed
  head CAS; Trace, trajectories, embedded state, vmctl, route tables, status,
  checkpoints, and reducers are projections or actuators.
- **Two Dolt stores:** narrow event-head/idempotency and platform-control rows
  live on the existing corpusd world-wire sql-server; the VM-local embedded
  Dolt indexes the chain and materializes effective state. Neither creates a
  third semantic store or alternate head.
- **Acceptance before effect:** an effect bundle is inert until acceptance.
  Guest materialization, checkpoint publication, and route CAS cannot
  acknowledge or substitute for the event.
- **ComputerVersion code identity:** `(CodeRef, ArtifactProgramRef)` names the
  code/artifact published at an event head. It is not computer identity, not
  promotion authority, and — alone — not a complete restore address. The
  fuller event-head addressing plus VM-local content witness for restore
  completeness is carried by the carrier/ontology work; promote that fuller
  checkpoint binding to settled doctrine only after deployed restore proof.
- **Per-computer frontend:** the UI that renders a computer is that computer's
  surface (`C15`/`I25`). User-authored frontend changes are scoped by
  `ComputerID` and take the same capsule-accept-materialize path as other
  self-development. A host-global SPA (`frontend-current`) is current
  non-conformance, not the product. Thin platform shell (TLS, auth, picker
  chrome) may remain host software. Do not ship UI in the current effects
  candidate; the browser would not read it. See
  `docs/memo-per-computer-frontend-2026-08-13.md`.
- **vmctl projection:** vmctl remains the sole route-slot CAS actuator and
  verifies exact accepted-event, checkpoint, materialization, verifier, and
  route-certificate joins. Post-genesis legacy route authority refuses.
- **No embedded-store promotion:** the obsolete tag/commit/reset
  `DoltPromotionAdapter` is deleted; accepted-event materialization is the only
  self-development path.
- **Timeout hardening landed:** `vmctl.Client` defaults to 60 seconds and the
  server has bounded read/write timeouts (120-second defaults). Staging proved
  the induced resolve-failure path returns a bounded 504; re-prove after a
  routing or timeout change rather than reopening the old 180-second diagnosis.

Texture delegation is agentic. Texture may revise, ask the research desk, report
or ask the management desk, do several of those, do none, wait for more evidence,
or report a blocker within its authority envelope. `patch_texture` and
`rewrite_texture` store canonical revisions; neither may become a semantic
workflow gate that requires a subsequent research, management, verification, or
other appagent call. Exact required-tool continuation is reserved for narrow
mechanical protocols, not appagent policy.

Prompt bar, source ingestion, and article/news creation should show deferred
world-wire ingress followed by Texture artifact materialization. Management
before Texture is a route invariant failure. Management after Texture is valid
only when Texture sends a semantic act through a capability the runtime grants.

Supervision is asynchronous and continuous. Texture may write many semantic
versions between owner reads while it receives intermediate reports and sends
revised direction to research or management. Capsule work, verification, and
durable operations return `choir.Report` evidence handles as they progress, not
only a terminal report. Worker-VM and candidate-VM delegation are obsolete and
deleted. Generic delegated agents use durable runs/trajectories and capsules.

Management admits engineering through a delegated `choir.Cast` and its durable
assignment/trajectory and capsule-bound operation handle. A subordinate must
not reconcile competing supervisors or receive a capability from model-visible
text.

Verifier agents are read-only with respect to canonical product state. They may
execute only in an independently provisioned read-only capsule whose
capabilities cannot commit, accept, materialize, checkpoint, or route effects.

## Private Go Activation And Capability Profiles

The desk architecture converges management, engineering, research, Texture, and
bounded appagents on one durable-actor activation kernel. Each persistent root
desk runs a private, disposable Yaegi interpreter in a killable subprocess with
an assignment-scoped module manifest, current activation capabilities, bounded
observations, and a typed outcome contract. The model incrementally authors and
executes Go. Differences among desks belong in organizational bindings, module
profiles, policy, and outcomes rather than persona-specific model loops.

Restricted desk activations expose only the Go-cell operation to the model.
Search, source fetch, document and file-format transforms, artifact operations,
delegation, messaging, evidence, and work state are narrow imported modules;
there is no duplicate ambient JSON-tool path. Backends may use compiled services
or isolated converters internally without exposing shell, raw HTTP, or general
filesystem authority to the actor.

The four desks use in-cell yaegi `choir.*` semantic-act functions rather than a
tool-call channel. The retired `update_coagent` interface is absent for desks;
its packet machinery survives as `Report`'s body and for deferred world-wire
roles until their phase. No successful desk migration makes a deferred role
complete.

An effects-capable implementation assignment may additionally expose direct
Bash for ergonomic computer interaction. Direct Bash and the Go
`execution.Run`/`execution.Shell` surface are two front ends to one capsule
execution broker, transaction tape, cancellation path, and receipt schema.
Commands run inside the existing guest-local effect capsule. Child processes
receive only capsule-local inputs and an explicit ordinary environment; they
must not inherit activation capabilities, broker or provider credentials,
canonical database access, sensitive descriptors, guest-control sockets, or
host authority.

Yaegi is an orchestration runtime, not a sandbox or authority source. An import
only makes an operation nameable. Every consequential module call must validate
the current activation, actor, computer, trajectory, work item, operation,
resource budget, capability grant, and relevant state head in trusted compiled
code. The capsule supplies process, filesystem, network, resource, and effect
containment. Trusted reducers alone own durable shared state and external or
canonical effects.

Model-authored functions may be frozen as immutable source artifacts and reused
or revised. Source is inert: reuse never preserves a live handle, interpreter
heap, credential, activation capability, or prior authority. A later activation
resolves its module requirements and receives fresh capabilities. Repeatedly
successful source may become a reviewed orchestration recipe, but neither
origin nor reuse is promotion.

Local goroutines and channels are ephemeral activation dataflow. Durable
responsibility and communication use assignments, work items, authenticated
typed messages, and obligations. Actors park with typed continuations rather
than retaining an activation while another actor works.

## Citable Orchestration Evidence

The runtime records exact Go-cell source and result identity plus every
consequential capability boundary crossing. Shell/process runs, source and
artifact access, assignments, agent messages, blockers, disagreements,
capability requests, verification, effect-bundle freezing, continuations, and
outcomes produce host-authored causal receipts. This is a normalized
orchestration graph, not an instruction or syscall recording.

The host derives the complete salient excerpt population for supervision so an
actor cannot conceal an inconvenient command, refusal, failure, or dissent by
leaving it out of prose. Supervisory actors select, disposition, and explain
those receipts. Texture may cite and transclude exact immutable excerpts in
canonical versions. Reports and transclusions remain evidence projections:
they cannot append computer events, accept effects, or replace Texture's
canonical ownership.

## Harness Minimalism

Keep the agent loop programmatically uniform where authority permits: provider
call semantics, cancellation, retry, compaction, and durable trajectory
projection should not fork by persona. Capability resolution, privacy-safe
canonical event append, capsule isolation, and typed role policy are deliberate
security boundaries. Production actor roles are the management, engineering,
research, and Texture desks, explicitly bounded appagents, and deferred
world-wire/system-one roles; retired aliases refuse.

Prefer prompts, tool descriptions, capability policy, and product-visible
state over role-specific harness branches. (Prompt content itself is moving
from persona framing toward obligation/authority-envelope framing, but the structural point
here, prompt/policy over code branches, holds either way.) If a proposed fix
requires programmatic divergence in the core loop for one role, document the
evidence, the invariant being protected, the simpler alternatives rejected,
and obtain explicit human approval before landing it. Divergence is acceptable
only when it protects correctness, security, authority boundaries, or
resource isolation in a way that cannot be represented cleanly as policy or
prompt contract.
This is a repository-maintainer exception for changing the shared harness
implementation. It is not a product rule that a human must approve
irreversible effects; product effect authority follows the policy-governed
multiagent consensus contract below.

## Prompt Control-Flow Antipattern

Prompts provide data and invariants, not boolean branches that switch behavior.
A prompt should name the style texture, the available sources, the run context,
and the invariants (cite sources, no model priors as grounded, canonical
revisions via tools). It must not branch on runtime metadata to switch behavior
(`{{if .WireTexture}}`, first-owner-prompt special cases, worker-finding gates).
Unconditional invariant text is not control flow. Decisions that used to live in
prompt branches belong in the style texture, the run context, or tool
plan and `choir-doctrine.md` invariant I16.

Source citation is tri-state and citation shape is a display mode, not a
separate node type (Choir Doctrine I15). Every source entity is cited
(`source_ref` in the body), toolbar-only (a Style.texture style source), or
marked-unused (`mark_source_unused` with a rationale). The former
`source_embed` block node is removed; all citations are `source_ref` with
`display_mode` (`numbered_ref` | `expanded_ref`).

## Texture as Artifact Control Plane

Texture is also Choir's artifact control plane and the delegated controller for
long-running artifact trajectories. Deferred world-wire ingress routes exogenous
user/app/source input into Texture-owned artifact state: prompt-bar requests,
sourcecycled/news ingestion, article creation, mission work, and most user
prompts should open or create Texture/context first. Management is not the direct
ingress target for ordinary user or source prompts. Texture may later send the
management desk a semantic act when the artifact needs execution, coding-agent
trees, generated artifacts, verification, candidate work, or another privileged
action. Management admits engineering with delegated `choir.Cast`; research and
engineering return `choir.Report` material as work advances. Texture incorporates
what they teach into idea-level versions and redirects the trajectory. The owner
samples and corrects the current head asynchronously rather than approving each
version.

Read `texture-agentic-invariants-2026-06-13.md` before changing Texture tools,
prompts, routing, revision creation, desk wake behavior, Trace/Texture
projection, run acceptance involving Texture, or missions that use Texture as
their owner-readable narrative. Texture is the canonical document/versioning
core and must remain an agentic participant in a multi-agent system, not a
workflow runner. Runtime may expose affordances and durable obligations, but it
must not force Texture to call research, management, verification, or any
semantic appagent merely because prompt text, revision metadata, or an acceptance
probe mentions that role.

## Runtime Configuration

Provider secrets and platform model catalogs are platform-owned. Per-computer
model policy is computer-owned durable **runtime configuration** and may remain
editable through the product path as a config write. It is **not** the
self-development effect content axis: the active Definition retires model-policy
bundles as the first self-development proof (no apply path to resolution; surface
is system-owned and scheduled for broker-mediated replacement). Do not teach
agents that "the computer changing itself" means editing model policy. Do not
patch Node B environment variables or tracked server files as a substitute for a
runtime policy path unless the mission is explicitly a platform config deploy.

Role defaults are policy defaults, not architecture. Any configured model may
serve a production desk or bounded role when its declared capabilities match the
current turn: management, engineering, research, Texture, verification, or a
future bounded role. Text-only models are valid for orchestration, research,
coding, and verification that does not need media input. Multimodal models are
required only when the turn needs screenshots, images, video frames, files, or
other media inputs. If a current policy maps a desk to ChatGPT or Fireworks,
treat that as the active computer's effective policy, not a hard-coded role
boundary. Capability is evaluated for the next turn, not permanently for the
role. Do not add new role-specific provider assumptions such as "management must
be ChatGPT", "Texture must be Fireworks", or "verification must be multimodal"
unless the current turn's capability requirements actually imply that. The
long-term target is dynamic, agentically editable per-computer model policy: an
owner prompt may ask management to admit a capability-bound engineering change
to the computer's model policy, and subsequent runs should use that policy
without a platform deploy or Node B environment edit. The platform catalog
records model capabilities and provider request semantics; per-computer policy
selects among those capabilities.

Provider request schemas must preserve modality. If a task needs screenshots,
videos, files, or other media evidence, route through a model/provider path that
declares that modality and record the blocker precisely when the adapter cannot
resolve the artifact.

## Product-Path Verification

Browser or Playwright acceptance may use public authenticated product APIs such as:

- `/api/prompt-bar`
- `/api/prompt-bar/submissions/{id}`
- `/api/texture/*`
- `/api/trace/*`
- `/api/current-computer`
- `/api/computers/*/self-development/*`
- `/api/continuations/*` (transitional H007/H008 residue; prefer
  trajectory/work-item product evidence when available and do not add new
  continuation-shaped acceptance)
- `/api/run-acceptances/*`

Do not use browser-public internal or test-only routes to bypass the product path:

- `/api/agent/*`
- `/api/prompts`
- `/api/test/*`
- `/internal/*`
- raw event mutation endpoints

The verifier must observe product/control evidence. It must not manually seed success records.

## Run Acceptance Records

For long-running mission proof (`/goal <doc>.md`), the mission's own evidence
ledger and completion semantics govern what counts as settled — throughline
for new files, the Definition format for the existing corpus.
`RunAcceptanceRecord` is a historical evidence projection for older runs; it
is not self-development authority.

The completed convergence Definition's terminal receipt records exact
source/deploy/host/guest identity; artifact, subject/activation, obligation,
update-disposition, and work-settlement refs; restart/reconstruction and
cancellation traces; UI/headless protocol conformance; authority refusals;
rollback; mutation and heresy deltas; and residual risk. That receipt left
self-development effects OFF as a **pre-gate resting state**.

Do not claim deployed self-development from the superseded CTS Definition,
rejected Round 72 candidate, AppChangePackage/AppAdoption, RunAcceptance,
worker/candidate VM, local tests, a verifier statement, checkpoint publication,
or route transition. The self-development gate is now carried by the revised
roadmap (`docs/world-wire-mission-stack-2026-09-22.md`, Phase 3) after the
carrier and precommitment records; the superseded effects Definition
(`archive/choir-supervised-self-development-effects-2026-08-11.md`)
is historical evidence for the effect-policy shape, not the live path.
Effects turn on only through effect-specific multiagent consensus policies and
audited actuators, not as a global ON boolean. Reversible effects gain a restore
path; irreversible effects require stronger policy, evidence, and consequence
receipts but do not categorically require a human decision. Human participation
is a policy-selected seat.

`continuation-level` is transitional H008/H014 residue: the durable-actor contract re-points this
acceptance level at trajectory/work-item settlement evidence. No deleted
portfolio mission remains executable authority.
Until that cutover lands, retired `continuation-level` keeps its current meaning and
evidence requirement above — do not weaken it and do not claim trajectory
settlement evidence in its place before the level is formally re-pointed.
Do not introduce new retired `continuation-level` claims or APIs as doctrine; M4 must
delete or explicitly shim the old surface.
