# Choir Glossary

**Status:** canonical vocabulary
**Last updated:** 2026-06-11 (actor runtime + conjecture vocabulary added;
continuation/channel/parent-child retirements recorded)

This glossary names the current Choir product and runtime vocabulary. It folds
the old root `PROJECT-GLOSSARY.md` into `docs/` and updates it for the current
cloud, computer, Wire, run-control, promotion, and publication ontology.

Read [computer-ontology.md](computer-ontology.md) for the durable computer
model and
[wire-news-system-learning-saga-2026-06-09.md](wire-news-system-learning-saga-2026-06-09.md)
for the news/Wire terminology correction.

## Product Vector

### private AI cloud

The practical product frame for Choir: an open-source, self-hostable AI work
system where an organization owns its computers, source artifacts, VTexts,
agent state, model policy, publication boundaries, and learning derivatives.

### automatic computer

The private agentic workspace: a persistent user computer where apps, agents,
files, prompts, local builds, Dolt state, package installs, and candidate
branches can diverge from the platform baseline.

### Wire

The reusable source-to-VText substrate. Wire ingests public or private sources,
lets processors, reconcilers, researchers, and VText agents synthesize them, and
produces source-backed VTexts, edition VTexts, indexes, and publication or
subscription artifacts.

Wire is not only public news. Public news is the Community Cloud instance of
Wire. Private Choir Clouds run the same substrate over private sources,
subscribed public sources, and domain-specific corpora.

### Universal Wire

The public Choir Community Cloud instance of Wire. It owns public source
artifacts, platform-level processors/reconcilers/researchers, public article or
report VTexts, public edition VTexts, and public indexes.

Use "Universal Wire" in architecture docs when disambiguating from private Wire
instances. User-facing copy may simply say "Wire" when the context is clear.

### Private Wire

A Wire instance inside a Private Choir Cloud. It may run over private documents,
client files, internal communications, private feeds, subscribed public Wire
artifacts, and domain-specific sources. Examples include Firm Wire, Matter Wire,
Research Wire, Market Wire, Science Wire, or executive briefings.

### automatic newspaper

The public memory layer where selected private artifacts become citeable,
disputable, forkable, reusable, and durable.

This remains a useful long-range projection, but current architecture should use
Wire for the concrete source-to-VText substrate.

### automatic radio

The screenless traversal layer over promoted meaning. Radio is not separate from
`vtext`; `vtext` is the semantic score and radio is one performance of it.

### automatic capital

The later capital-formation layer. It is not a current implementation target,
but current systems should preserve provenance, citations, compute attribution,
artifact ownership, and publication boundaries so this layer remains possible.

## Clouds, Hosts, And Computers

### cloud

A deployment boundary containing host infrastructure, platform computers, user
computers, candidate computers, source systems, policy, and publication or
subscription boundaries.

Do not use "tenant" as the main product term. A customer deployment may have a
thousand employees and its own NixOS hosts. It is a cloud, not a row in a shared
tenant table.

### Choir Community Cloud

The public/shared Choir deployment. It includes `choir.news`, public
publication surfaces, Universal Wire, public package/artifact surfaces, public
user accounts, Community Cloud platform computers, and user computers hosted
inside the public deployment.

### Private Choir Cloud

A customer-controlled Choir deployment. A Private Choir Cloud has its own NixOS
host or host cluster, platform computer(s), many user computers, candidate
computers, private source systems, model/search policy, compliance/egress
policy, and optional publication/subscription links to the Choir Community
Cloud.

### host

The NixOS machine or host cluster running cloud infrastructure: edge/proxy,
auth, gateway, computer lifecycle, platform services, source services, storage,
and other daemons. A host is infrastructure. A computer is the persistent
product/runtime object owned by a platform or user inside a cloud.

### platform computer

A persistent computer owned by a cloud itself rather than an individual user.
It runs platform-level agents and owns platform-level semantic state such as
Universal Wire source artifacts, public article/edition VTexts, platform agent
notebooks, publication queues, and cloud-level indexes.

A Private Choir Cloud can also have platform computers: firm-wide source
systems, policy agents, matter indexes, firm templates, and shared private
VTexts live there.

### user computer

A persistent computer owned by a user/person/service account inside a cloud. It
owns private VTexts, user preferences, files, user agents, user-level
processors/reconcilers, personal editions, forks, alerts, and candidate work.

## Computers And Candidate Worlds

### computer

The durable user-facing execution object. A computer is not one database, one VM
snapshot, one repo checkout, or one browser session. It is a product object made
from VM/runtime state, Dolt/app state, source/build state, content blobs,
artifact provenance, and route identity.

Read [computer-ontology.md](computer-ontology.md) for the ledger and promotion
model.

### active computer

The computer currently routed to the user. It hosts the visible desktop, apps,
appagents, private state, local files, prompts, and user-specific runtime state.
It should stay stable and responsive.

### background computer

A fork of the user's active computer used for risky, long-running, or mutable
work. It can run tests, install packages, build local runtime changes, generate
artifacts, and report candidate deltas without destabilizing the active
computer.

### candidate computer

A background computer that is explicitly allowed to mutate and fail. It may later
be discarded, archived, merged, promoted to active, or packaged for publication.

### candidate world

The substrate-neutral term for speculative state. A candidate world may be a
computer, worktree, Dolt branch, package branch, source branch, or future state
branch.

### sandbox

The current implementation name for the runtime service/process. It is not the
product noun. Use `sandbox` only when referring to existing code, service names,
paths, or compatibility surfaces.

When writing product or architecture docs, prefer **computer**, **computer
runtime**, or **platform/user/candidate computer**. "Sandbox" implies
ephemerality and should not describe durable Choir computers.

### VM / microVM

The implementation substrate for some computers. Use VM terms when discussing
Firecracker, NixOS images, host-process fallback, routing, snapshots, or
capacity. Use computer terms when discussing the product object.

### `vmctl`

The host-side VM lifecycle, capacity, ownership, and routing service. It should
support active/background computer forks, candidate execution, promotion
support, and rollback machinery.

### `platform_vm_pool`

Platform-level VM capacity for public, unauthenticated, or shared serving work.
It can serve published `vtext` artifacts without hydrating a user's private
active computer.

## State And Promotion

### ledger split

The rule that Choir state lives in multiple ledgers with different merge laws:
VM/runtime state, Dolt/app state, source/build state, blob/content state,
artifact provenance, and route identity.

### typed artifact

A durable, inspectable unit of change: Dolt commit, source patch, build bundle,
blob hash, artifact graph record, verifier result, app package, agent package,
or route-switch certificate. Typed artifacts are promotable; opaque machine
accidents are not.

### personal promotion

A promotion that changes one user's computer. It does not target `origin/main`
or the global staging deployment. It still needs lineage, typed deltas, verifier
evidence, foreground-tail reconciliation, route switch evidence, and rollback.

### platform/public promotion

A promotion that changes shared state: the official Choir baseline, public
packages, publication artifacts, shared app/agent packages, or public artifact
graph state. It requires higher ceremony and usually staging/deploy proof when
deployed behavior changes.

### promotion certificate

A durable record proving what base, candidate, active tail, merge result,
verifier evidence, route transition, and rollback target were used for a
promotion.

### Dolt

The desired canonical store for product state. Choir has two important Dolt
levels:

- per-user embedded Dolt for private desktop/appagent state;
- platform Dolt for platform-visible facts, publication metadata, routing
  records, public artifacts, citation graph, and compute accounting.

Read [adr-dolt-as-canonical-state.md](adr-dolt-as-canonical-state.md).

### SQLite

A narrow implementation store for hot runtime, cache, local compatibility,
transitional, or auth/session roles when justified. SQLite should not become the
default home for durable product truth.

### platform Dolt

The platform-visible Dolt ledger. It is not a global message bus. Cross-VM work
should use direct transport or relays for live delivery, then write compact
durable facts for recovery, provenance, publication, citation, routing, and
compute accounting.

### embedded Dolt

The per-user Dolt database inside a computer/runtime. It owns private `vtext`,
appagent, prompt, trace, run-memory, theme, file metadata, and promotion records
where those records belong to the user's computer.

## Agents And Authority

### dark factory

The mostly background production system behind the desktop: researchers,
supers, vsupers, cosupers, candidate computers, evidence, artifacts, tests,
previews, Trace, promotion records, and appagent synthesis.

### `conductor`

The intake/router for top-level user input and later connector events. It routes
events to appagents or flows. It does not own semantic artifacts and should not
become the supervisor of every semantic loop.

### app

A user-facing desktop surface. Not every app is an appagent. Apps can remain
plain display/control surfaces until they need durable domain ownership,
prompts, dynamic UI, or agentic behavior.

### appagent

An app with durable domain ownership. It owns semantic obligations for its
artifact or product area. `vtext` is the first canonical appagent.

### `vtext`

The version-native document app and appagent. It owns canonical document
versions, receives evidence and worker updates, synthesizes new versions, and is
the semantic substrate for publication and radio.

Only VText agents write canonical VText versions. Other agents may read VTexts
and message VText agents with evidence, requests, and proposed changes.

### processor

An agent role that works from incoming or query-selected source material toward
candidate understanding and requests. A processor asks what is new, what
changed, which source handles matter, what should be watched, what needs
research, and what VText work should be requested.

Processors may exist at Community Cloud platform level, Private Cloud platform
level, and user-computer level. A user-level processor can actively query and
research over the user's accessible public/private corpus; it is not merely a
deterministic subscription filter.

Processors do not write canonical VText versions.

### reconciler

An agent role that works from existing VTexts, source neighborhoods, notes, and
history toward coherence over time. A reconciler asks which VTexts disagree,
which claims are stale, which source artifacts contradict each other, which
related VTexts should be transcluded, and which emergent questions deserve new
research or VText work.

Reconcilers may exist at Community Cloud platform level, Private Cloud platform
level, and user-computer level. User-level reconcilers personalize and preserve
coherence across the user's editions, forks, interests, alerts, and private
context.

Reconcilers do not write canonical VText versions.

### `researcher`

The proactive sensing layer. Researchers gather current/external information,
read local context, write structured evidence and findings, and notify
implicated appagents. They do not patch code, mutate vtexts, or own terminal
authority.

### `super`

The foreground computer's privileged orchestration root. It can request bounded
execution authority, background/candidate computers, local cosupers, and worker
resources. It should not speculatively mutate canonical foreground state.

### `vsuper`

The sovereign worker inside a background/candidate computer. It can mutate that
candidate state within scope and spawn subordinate cosupers inside the same
boundary. It cannot promote canonical state.

### `cosuper`

A durable execution co-agent leased by `super` or `vsuper`. A local cosuper may
inspect, diagnose, use scratch space, run bounded checks, and report. It should
not structurally mutate canonical state.

### worker

The general category for delegated agents such as researchers, supers, vsupers,
cosupers, and future specialized roles. Workers produce updates, evidence,
deltas, diagnostics, candidates, or reports.

### platform agent

An agent running under a platform computer's authority. Platform agents may own
platform-level notes, evidence, and requests, but they still obey role
authority: for example, a platform reconciler may request a public Wire VText
revision but does not write that VText itself.

### user agent

An agent running under a user computer's authority. User agents may personalize,
fork, brief, research, and request VText work over the user's accessible corpus,
but they do not mutate platform-owned VTexts.

### agent notebook / agent evidence

Dolt-backed durable non-canonical state for processors, reconcilers,
researchers, supers, and other agents: checkpoints, findings, uncertainty,
source handles, requests, watch items, blockers, and evidence refs.

Agent notebooks/evidence are not VText versions. They are inputs to VText
agents, researchers, verifiers, and future runs.

### verification contract

A structured check request naming a target, purpose, invariants, required
checks, required capabilities, independence requirement, and result schema.
Verification is a phase and contract, not a separate privileged agent species.

## Runs, Memory, And Trace

### task

O(10 minutes). A single local objective or prompt-level piece of work.

### run

O(1 hour). A coherent loop with checks and maybe small branches.

### leap

O(8 hours). An overnight candidate-world run requiring compaction, candidate
mutation, bounded autonomy, and durable evidence.

### fly

O(60 hours). A weekend system run made from chained leaps, durable obligations,
candidate integration, verification, and promotion queue management.

### run memory

The durable operational memory of long-range work: current artifact state,
goal/value criterion, invariants, failed approaches, verifier status, candidate
world state, next probes, rollback points, and user taste updates.

### compaction

Inference-time learning. A compaction is not a chat summary; it is an
operational sufficient statistic that preserves what the next controller needs
to keep working.

### trajectory

The causal path started by one user request and continued through conductor
routing, appagent ownership, worker delegation, evidence, versions, and later
revisions.

### loop

One LLM/tool execution record inside a larger trajectory.

### work

A unit of agentic effort or causal activity. Prefer modeling generic work plus
actors, messages, timestamps, versions, and causes over premature workflow
schemas.

### Trace

The app/surface for inspecting trajectories, loops, delegations, tool calls,
message flow, worker families, and synthesis points. Trace is currently a debug
surface and may later become an appagent if volume requires agentic search.

## VText And Desktop Terms

### version

A canonical document state in `vtext`.

- `v0`: initial user-prompt-created document.
- `v1`: conductor framing/seed version.
- `v2+`: later user edits and appagent-authored revisions.

### user-authored version

A version created from a batch of user edits when the user commits/revises the
document.

### agent-authored version

A version authored by the `vtext` agent after synthesis.

### worker update

Structured output from a worker: findings, evidence, source references, artifact
refs, branch/commit refs, preview refs, test results, questions, constraints, or
proposal summaries. Worker updates are synthesis inputs, not canonical document
patches.

### `Revise`

The control inside `vtext` that finalizes the user's current edit batch into a
new user-authored version and re-engages the appagent.

### prompt bar

The bottom-bar input for top-level user requests. It should route through
`conductor`.

### prompt management

The per-user system for inspecting and editing role prompts inside Choir.
Prompt configuration belongs inside the user's computer, not as a host-global
setting.

## Terms To Avoid As Primary Names

Use these only for code compatibility, history, or explicit references:

- `etext`
- writer
- supervisor
- terminal agent
- Factory Droid / factory workflows as architecture
- sandbox as product noun
- VM as product noun
- run when the precise concept is trajectory, loop, task, leap, or fly

Preferred replacements:

- `vtext`
- `vtext agent`
- `super`
- `computer`
- active/background/candidate computer
- trajectory / loop / run / leap / fly, depending on duration and scope

## Actor Runtime Vocabulary (2026-06-11)

Doctrine sources: `choir-rearchitecture-durable-actors-2026-06-11.md`,
`system-v1-one-cut-2026-06-11.md`, `choir-role-free-actor-protocol-2026-06-11.md`.
Code cutover in progress per `mission-portfolio-2026-06-11.md`; these are the
target terms new work should use.

### actor

An agent as a durable actor: a goroutine with a Go-channel mailbox while
resident; an idempotent durable update log plus a compacted memory snapshot
while passivated. Actors never "complete" — they passivate on quiescence and
re-warm on the next update or sweep. Implemented in `internal/actor`.

### activation

One residency of an actor: wake → work (possibly hours, many loops and
compactions, mailbox live throughout) → passivate. Replaces "run" as the unit
of residency; runs remain as activation records (a read model). Bounded by
step/token budgets, activation caps, and eviction — there is no lease concept
in v1.

### update

The single agent-to-agent message primitive (`update_coagent`): typed
(findings, evidence, verification, blocker, question, proposal, status,
directive, assignment, capability_request), idempotent by `update_id`,
durably appended before delivery. Some kinds carry ledger effects in the same
transaction (assignment → work item; verification → acceptance evidence).
The only wake source.

### mailbox

The in-memory delivery vehicle of a resident actor. Never authoritative —
always rebuildable from `log minus processed`. Say *mailbox* for delivery,
*document/trajectory channel* for the product surface, *Go channel* for the
language primitive; the unqualified word "channel" is retired.

### passivation / eviction / sweep

Passivation: graceful sleep, only with zero unprocessed backlog (the check is
atomic with delivery). Eviction: forced passivation at any moment —
deliberately crash-equivalent (no snapshot; backlog stays durable). Sweep:
any non-resident agent with unprocessed backlog is activation-eligible —
one rule covering boot recovery, crash windows, and post-eviction re-wake.

### trajectory

The causality object: a durable record (kind, subject refs, explicit
settlement rule stored as data, status live/settled/cancelled). Replaces
parent/child run trees as the control model. What survives is provenance.

### provenance (spawned_by)

The frozen, past-tense fact that one run spawned another: `spawned_by` is
an event that happened, not a relationship that holds. No control reads —
no waiting on, budgeting by, or cancelling through the spawn edge. Say
"spawned by", "spawn site", "spawned run"; never "parent" or "child", even
for provenance — the present-tense relationship reading is where the
retired bug classes lived.

### work item

A durable assignment on a trajectory: objective, authority envelope,
fingerprint-deduped. Replaces RunContinuation. Blockers and questions open
obligation work items addressed to whoever can discharge them.

### settlement

A trajectory's earned closure, evaluated from its rule inside the
transactions that could change the verdict (never polled). Example rule
(publication): no open work items AND publish ref recorded AND edition
updated. Replaces root-run completion as liveness truth.

### authority envelope

What a bounded profile (super, vsuper, co-super, researcher, vtext,
processor, reconciler, conductor) may do — the code-enforced capability
boundary. Profiles are envelopes, not personas: actors are prompted with
obligations, trajectory state, and authority, never with identities
("you are X" is a banned prompt pattern — see the role-free actor protocol).

### capsule (designed, not built)

An ephemeral, effect-fenced execution chamber inside a computer (Nucleus-
class). Runs risky/bounded work; produces effect reports; never a seat of
agency, never canonical-state authority. Companion records: CapsuleSpec
(launch policy) and CapsuleResult (durable outcome + policy hashes).

### MutationTransaction / commit point / rollback window

The promotion protocol: per-ledger prepare (durable, idempotent, inert) →
verify → owner approval → one atomic commit-point flip (also the visibility
gate) → reconcile secondaries from the commit point alone. Freshness CAS
blocks committing against a moved foreground. The rollback window is explicit
state, closed by the first write the previous version cannot read; reverting
after that is the torn-rollback failure. Model-checked:
`specs/promotion_protocol.tla`.

### parallax / paradoc / conjecture circuit / shift / probe

Parallax (`skills/parallax/SKILL.md`) is the mission discipline succeeding
MissionGradient. Its literal shape is a **conjecture circuit**: the mission
document claims that if witness A satisfies spec/objective S under invariants
I and quality Q over domain D, then deeper goal G is achieved or materially
advanced. The circuit tests and constructs that claim through four moves:
**probe** (test under the current observer), **shift** (move the observer:
instrument, vantage, vocabulary, domain, prover, inversion), **construct**
(extend the witness), and **settle** (decide or accept the edge). A Parallax
mission document is a **paradoc**.

## Conjecture Vocabulary (2026-06-11)

Source: `conjecture-learning-proof-theory-2026-06-11.md`. The compact frame:
an observer is a proof system; its hyperthesis is its incompleteness; a
claim's authority is the reach of its evidence — including this one.

### conjecture

The active control object: `(CLAIM, TEST, HYPERTHESIS_EDGE, ΔO, SCOPE)` —
what might be true, how the current observer would know, how the claim could
survive falsely, the smallest observer upgrade that shrinks the edge, and
where the claim may be asserted if supported.

### hyperthesis edge

The named incompleteness of the current observer over a claim, classed as
independence, resource, missing_oracle, or frame_lock — each with a
different fix. Most systems run with null hyperthesis: confidence without a
named blind spot.

### assertion

A supported conjecture with receipts (evidence refs) and an explicit scope,
carrying invalidation triggers: when a premise dies, the assertion reverts
to a conjecture, visibly. Ledger: `conjecture-assertion-ledger-2026-06.md`.

### heresy

A claim still in circulation after its proof died — in docs, prompts, code
comments, or UI copy. Heresy sweeps are consistency maintenance, not
housekeeping: one tolerated contradiction licenses anything downstream.

## Retired Terms (2026-06-11)

- **continuation** — named two unrelated mechanisms (RunContinuation
  synthesis; channel handoff to persistent super). Replaced by work items
  and warm steering. Still in code during cutover; do not use in new
  doctrine.
- **parent/child run** — retired entirely, including for provenance. The
  control semantics are replaced by trajectories + settlement; the
  surviving edge is `spawned_by`, a past-tense event fact (see provenance).
  Still in code during cutover (`ParentRunID`, `StartChildRun`); the field
  rename lands with M3; do not use the words in new doctrine.
- **channel**, unqualified — disambiguate (mailbox / document channel /
  Go channel).
- **sandbox** as product ontology — the product object is a persistent
  computer; the code/service rename waits for capsules.
- **lease** — no lease concept in v1; say budgets, caps, envelopes.
  Reserved for future QoS/pricing tiers.
