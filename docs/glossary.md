# Choir Glossary

**Status:** canonical vocabulary
**Last updated:** 2026-05-14

This glossary names the current Choir product and runtime vocabulary. It folds
the old root `PROJECT-GLOSSARY.md` into `docs/` and updates it for the current
computer, run-control, promotion, and publication ontology.

## Product Vector

### Automatic Computer

The private agentic workspace: a persistent user computer where apps, agents,
files, prompts, local builds, Dolt state, package installs, and candidate
branches can diverge from the platform baseline.

### Automatic Newspaper

The public memory layer where selected private artifacts become citeable,
disputable, forkable, reusable, and durable.

### Automatic Radio

The screenless traversal layer over promoted meaning. Radio is not separate from
`vtext`; `vtext` is the semantic score and radio is one performance of it.

### Automatic Capital

The later capital-formation layer. It is not a current implementation target,
but current systems should preserve provenance, citations, compute attribution,
artifact ownership, and publication boundaries so this layer remains possible.

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
