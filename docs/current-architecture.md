# Choir Current Architecture

**Last updated:** 2026-07-10 (archive-removal pass; every claim is marked
**Live**, **Target**, or **Retired**; actor-runtime, capsule,
candidate-computer, and storage claims are governed by
[docs/definitions/og-dolt-heresy-completion-2026-07-08.md](definitions/og-dolt-heresy-completion-2026-07-08.md)
as current executable authority. The earlier assessment and hard-cutover
sources were absorbed into that definition and removed from the worktree.
Previous revision: 2026-06-11 ontology revision — durable actors, trajectories,
conjecture vocabulary; see the Ontology section.)

This is the current architecture memo for Choir. It is meant to be the first
document read before changing `texture`, conductor routing, workers, Trace, Dolt,
`vmctl`, publication, or appagent behavior. For current vocabulary and project
direction, read [semantic-registry.md](semantic-registry.md) and
[choir-doctrine.md](choir-doctrine.md). For the current common platform/default
computer OS, desktop shell, and app catalog state, read
[platform-os-app-state.md](platform-os-app-state.md).

## Document Contract

This document tracks the architecture that the current repo and staging system
actually implement, plus explicitly labeled active hardening where the code
already has a partial substrate. It should not be used as a speculative product
roadmap. Sections labeled target-only are not current-state claims.

Every claim in this document belongs to exactly one of three states, and the
tags below make that explicit (this repairs heresy H020, mixed current/target
onboarding):

- **Live (2026-07)** — implemented and running in this repo/staging now.
- **Target** — decided direction, not yet (fully) implemented. Where an owner
  decision exists, it is cited.
- **Retired** — a former design or vocabulary that still has code/doc residue
  but must not receive new work.

Untagged prose inside a tagged section inherits the section's tag.

Intended-but-unbuilt architecture belongs here as an explicitly labeled
**Target** claim, not in a parallel roadmap document.

When this document says "current," it should be backed by one of:

- code in this repository;
- current canonical docs that reflect landed code;
- staging evidence on `https://choir.news`;
- a precise note that the behavior is code-present but not yet staging-proven.

If code, staging, and this document disagree, code/staging evidence wins and
this document should be fixed.
When older browser/Trace/continuation language appears here, read it as
implementation or evidence residue. Current doctrine is Source Viewer before
explicit Web Lens inspection, Trace as evidence rather than a user app, and
`continuation-level` as transitional residue only.

## Ontology (2026-06-11 Revision)

The architecture program of 2026-06-11 revised the core ontology. Its settled
claims are now maintained in this document, [choir-doctrine.md](choir-doctrine.md),
and [runtime-invariants.md](runtime-invariants.md); the source program remains
available in Git history. The runtime protocols are model-checked in `specs/`
(TLC runs in CI).

**Transitional honesty (revised 2026-07-07):** the actor cutover is further
along than earlier revisions of this paragraph claimed. **Live (2026-07):** the
actor runtime is fully wired and is the *only* execution substrate — the
`dispatchActor` hook panics if nil, with no legacy fallback path
(internal/runtime/runtime.go); cold-start, coagent wake, cancel, park-resume,
and the tool loop all run through the actor; warm delivery is a Go channel, not
DB polling (H030 repaired). `internal/runtime` is the live business-logic layer
(~106K LOC of tool loops, texture state machine, wire synthesis, run memory)
awaiting *extraction and deletion*, not a zombie awaiting wiring. **Retired
(residue still in tree):** parent/child run control and RunContinuations are
named heresies (H001–H008) with deletion scheduled in the current umbrella
mission [docs/definitions/og-dolt-heresy-completion-2026-07-08.md](definitions/og-dolt-heresy-completion-2026-07-08.md)
(Phase B/C; the older hard-cutover mission doc is superseded source material).
They must receive no new callers. This section states the settled vocabulary so
new work stops accreting on the retired ontology.

| Term | Meaning |
|---|---|
| **actor** | an agent as a durable actor: goroutine + mailbox while resident; idempotent update log + compacted memory snapshot while passivated. Actors never "complete" — they passivate and re-warm. |
| **activation** | one residency: wake → work (possibly hours, many compactions) → passivate. Bounded by budgets and eviction, not retired leases (no lease concept in v1). |
| **update** | the one agent-to-agent message primitive (`update_coagent`): typed, idempotent by update_id, durably logged before delivery. The only wake source. |
| **mailbox** | the in-memory delivery vehicle for a resident actor. Never the truth — always rebuildable from the log. |
| **sweep** | the recovery rule: any non-resident agent with unprocessed backlog is activation-eligible. Covers boot, crash windows, post-eviction re-wake. |
| **trajectory** | the causality object: durable record with kind, subject refs, and an explicit settlement rule (data, not code). Replaces parent/child trees as the control model. |
| **work item** | a durable assignment on a trajectory: objective, authority envelope, fingerprint-deduped. Replaces retired RunContinuation. |
| **settlement** | a trajectory's goal closure, earned by its rule (e.g. publication: published AND listed AND no open work). Replaces root-run completion as liveness truth. |
| **obligation** | an open work item, blocker, or question on a live trajectory. "Open obligations with no resident assignee" is the stall query — observability, never a planner. |
| **authority envelope** | what a bounded profile (super/vsuper/co-super/researcher/texture/...) may do — code-enforced capability boundary. Profiles are envelopes, not personas. |
| **capsule** | (designed, not built) an ephemeral effect-fenced execution chamber inside a computer; never a seat of agency, never promotion authority. |
| **MutationTransaction / promotion** | state change via a single commit point: per-ledger prepare → verify → owner approval → atomic flip → reconcile; freshness CAS against the foreground; rollback window explicit. |
| **conjecture / hyperthesis / assertion** | the epistemic vocabulary: a claim under test with a named blind edge and scope; an assertion is a supported conjecture with receipts; heresy is a circulating claim whose proof died. |

Retired vocabulary: **"continuation"** (named two unrelated mechanisms; both
are replaced — work items and warm steering). **Parent/child as control**
(provenance-only edge remains). **"Channel"** unqualified — say *mailbox* for
delivery, *document/trajectory channel* for the product surface, *Go channel*
for the primitive. **"Sandbox" as product ontology** — the product object is a
persistent computer (code/service rename deferred until capsules land).

The self-improvement frame (one promotion discipline at every grain):

| Level | Scope | Candidate | Verifier | Promotion |
|---|---|---|---|---|
| 1. Improvement in the small | Texture media content | draft revision | citation checks, review, rubrics | revision becomes current |
| 2. Self-development | Choir's own code and architecture | candidate computer | verifier fleet, RunAcceptance | MutationTransaction route switch |
| 3. Meta-learning | the conjecture discipline itself | docs branch / one-mission trial | did action/evidence/scope/stopping change | skill/doc/invariant updates |

Same five-tuple at every level: `(CLAIM, TEST, HYPERTHESIS_EDGE, ΔO, SCOPE)`.
Level N changes are admitted by gates at level N+1 — self-reference without
gates is the named anti-pattern.

## Current Reality

Choir is a durable learning control system over versioned artifacts. The web
desktop is the current general-purpose projection of that substrate, not the
whole ontology. Read [choir-doctrine.md](choir-doctrine.md) for the higher-level
frame and [semantic-registry.md](semantic-registry.md) for current vocabulary.

The Automatic Computer already exists in deployed form: a web desktop, backend
services, appagents, and NixOS-on-NixOS VM infrastructure. A native macOS app
(Wails v3) wraps the web desktop with `ASWebAuthenticationSession` for passkey
auth, transparent title bar, and cloud-mode-by-default. See
[cmd/desktop/README.md](../cmd/desktop/README.md).
The product object is a persistent user **computer**, not a disposable sandbox.
The current work is not to invent the product from scratch. The current work is
to stabilize the deployed system around the right causal model.

Choir is not retired chat and not a generic coding-agent runner. The visible product is
a web desktop with apps. Some apps grow into appagents; most apps can remain
plain display/control surfaces. The hidden product machinery is a dark factory
of researchers, supers, cosupers, background computers, evidence, artifacts, document
versions, candidate worlds, promotion records, and eventually publications,
radio traversals, and citation/economic state.

The operating stance is now staging-first. Meaningful claims about vmctl,
gateway credentials, live model/search calls, background/candidate computers,
platform promotion, rollback, auth/session renewal, and Choir-in-Choir must be proven on
`https://choir.news` after commit, push, CI, deploy, and staging health
identity checks. Local development remains useful for fast frontend iteration
and focused unit shaping, but local proof does not establish product readiness.

This staging-first rule applies to platform behavior and shared runtime claims.
It does not mean every user-local computer change must wait for global CI/deploy.
The intended personal-computer path is that users can fork a candidate from
their own active computer, change apps inside that candidate, build a new
Go/Svelte runtime, install packages, and promote the verified candidate back
into their own active computer with local verifier and rollback evidence. Treat
that as target architecture until the product path is fully code-backed and
proven.

The current promotion architecture is stable platform, divergent computers.
Read [computer-ontology.md](computer-ontology.md) before changing
source-lineage, app-package, runtime/UI promotion, platform-computer, or
deployment behavior. The short version is:

```text
platform substrate changes -> GitHub main -> CI -> NixOS deploy
user computer changes      -> active source ref -> candidate -> verify -> promote
source/app sharing         -> app change package -> recipient candidate rebuild
new-user default image     -> official platform computer fork
public surface changes     -> platform computer candidate unless substrate changes
private public surface     -> selected route projection, not whole-computer exposure
```

The current cloud vocabulary matters for source/news work:

- the **Choir Community Cloud** is the public/shared deployment;
- a **Private Choir Cloud** is a customer-controlled NixOS host or host cluster
  with platform computer(s), many user computers, candidate computers, private
  source systems, and optional publication/subscription links;
- **Wire** is the reusable source-to-Texture substrate;
- **World Wire** is platform-level work in the Community Cloud, not a
  user-computer feature;
- personalization is user-computer work over accessible public/private corpora.

## Service Topology

The current codebase/deployable topology is one product split across a small
number of host services and per-computer runtimes:

```text
browser
  -> Caddy edge
    -> auth service for WebAuthn/session lifecycle
    -> proxy service for authenticated product API routing
      -> user's resolved computer runtime
    -> platform publication read routes where safe

host services
  -> vmctl owns computer lifecycle, warmness, hibernation, and reclaim policy
  -> gateway owns provider credentials and provider request mediation
  -> corpusd owns platform/public publication ledger writes
  -> maild owns email ingress/drafts/notifications where configured
  -> sourcecycled owns the current experimental source-service daemon

platform computer runtime
  -> cloud-level processors/reconcilers/researchers/Texture agents where present
  -> cloud-owned Wire artifacts, editions, indexes, and agent notebooks
  -> cloud-owned source/publication state that is semantic product state

per-user computer runtime
  -> conductor routes owner intent
  -> app surfaces project durable state
  -> appagents own canonical semantic artifacts when needed
  -> researcher/super/vsuper/co-super workers create evidence and candidates
  -> user processors/reconcilers personalize accessible corpora where present
  -> embedded Dolt owns private computer product state
  -> zot can run as a Super Console subprocess when configured
```

Important boundary rules:

- Caddy is edge/static routing infrastructure, not semantic authority.
- The browser talks to product APIs, not raw agent, prompt, event, vmctl, Dolt,
  provider, or platform-internal mutation endpoints.
- `auth`, `proxy`, `gateway`, `vmctl`, `corpusd`, `maild`, and `sourcecycled`
  are host/platform or sidecar services in the current codebase. They should
  stay narrow and should not become private document, appagent, or user-computer
  semantic owners.
- The per-user computer runtime is where private conductor, Texture, appagent,
  Trace, run memory, app state, source metadata, and candidate-control product
  state live.
- Platform-level semantic work, such as World Wire article/edition Textures
  and public source synthesis, should be scoped to platform computer authority
  even when host daemons perform serving, lifecycle, or adapter work.
- Provider secrets stay in the gateway/platform boundary. Per-computer model
  policy chooses among platform-declared capabilities without copying secrets
  into user state.
- VM/computer lifecycle state is product-facing only through redacted,
  authenticated product APIs. Browser-public raw `vmctl` is not a product
  surface.
- Platform publication receives selected public/private projections. It never
  gains write authority over the live private source document.

This topology supersedes older architecture sketches that described a generic
local-first multiagent runtime, shared scheduler, or broad sandbox-owned "OS
layer" as the product ontology. The product object is the persistent computer;
`sandbox` remains an implementation/service name.

Run acceptance is a first-class artifact. A `RunAcceptanceRecord` should be
synthesized from existing product/control evidence: runs, Trace moments, worker
AppChangePackages, recipient adoptions, verifier contracts, rollback refs, and
deployed build identity. It replaces ad hoc claims like "the Trace looked good."

The embedded Dolt runtime migration is complete on staging. As of commit
`c3b1a4b2547d672eadd9b3d74b76ba9371518648`, per-user runtime/control product
tables and Texture tables open in the same embedded Dolt workspace inside the
user computer. The old `/state` path remains a marker and legacy-import source;
fresh accepted staging computers showed no runtime SQLite WAL/SHM pair. Host
auth/session state remains host-owned.

Runtime model selection is policy-driven. Provider secrets and the platform
model catalog are platform-owned, but the effective model policy for a user
computer is computer-owned durable state. The platform may ship default role
mappings, such as ChatGPT for a foreground role or Fireworks-hosted DeepSeek/Kimi
for another, but those mappings are not architectural boundaries. Any configured
model may serve conductor, Texture, researcher, super, vsuper, co-super, verifier,
or future roles when its declared capabilities match the current turn. Text-only
models can run orchestration, research, coding, writing, and text/code/evidence
verification. Multimodal models are required only for turns that actually need
screenshots, images, video frames, uploaded files, or other media input. The
target architecture is a hierarchy of platform catalog -> platform defaults ->
per-computer policy -> per-run/task override -> modality requirement, with
owner and `super` edits flowing through product state rather than Node B config
patches.

Platform publication now has a first service boundary. A host-side `corpusd`
service writes to a separate localhost-only `dolt sql-server` primary and owns
platform-visible publication, route, artifact manifest/blob, retrieval source/
span, citation edge, provenance, consent/review, verifier, and rollback rows.
The browser never talks to Dolt. A signed-in user calls the proxy product API to
publish a Texture; the proxy reads the head revision and the full revision chain
from the user's resolved computer and submits the public projection plus a
canonical `version_history` manifest — per-revision content/citations/typed
provenance and a content-addressed hash chain — to `corpusd`. A published
Texture IS its full versioned history, not only the head projection; the
manifest is the signable spine a reader/verifier can independently replay and
check. Public published snapshots now resolve through the Svelte Choir
shell and Texture app at the current `/pub/texture/...` compatibility route (`texture-cutover-allow:` public route shim; deletion receipt: `texture-hard-cutover-v0`): signed-out visitors get a guest read-only Texture
surface, signed-in users can create private derivatives and proposals, and proxy
read APIs fetch sanitized publication bundles from internal-only `corpusd`
endpoints. Platform services still never gain write access to the live private
document.

For the current contract covering external source ingestion, source cleaning,
Texture source metadata, transclusion, publication policy, and export, read
[source-external-data-publication.md](source-external-data-publication.md).

## Implemented Foundations And Active Hardening

This section separates code-present foundations from remaining hardening. It is
not a backlog of things that do not exist.

Code-present/current foundations:

1. Public and signed-out desktop surfaces exist, and mutable paths are expected
   to cross an auth boundary before continuing through an owned computer. Treat
   individual public/auth-on-mutation journeys as staging-proof-sensitive.
2. Prompt bar, conductor routing, Texture documents/revisions/history/export,
   worker updates, Trace projections, run acceptances, AppChangePackages,
   adoptions, continuations, and computer source-lineage APIs exist in the
   runtime product surface.
3. Texture already has deterministic backend coverage for document creation,
   revisions, user edits, worker update integration, stale-result protection,
   source entities, source repairs, attachments, diagnosis, import, export,
   blame, diff, and history.
4. URL/content ingestion, content items, podcast routes, Web Lens/browser-session
   implementation state, PDF, EPUB, image, audio, video, and
   ContentViewer-style source surfaces exist at varying quality levels. Current
   app state is tracked in
   [platform-os-app-state.md](platform-os-app-state.md).
5. Platform publication has `corpusd`, proxy publish/read APIs, public
   `/pub/texture/...` compatibility routes (`texture-cutover-allow:` public route shim; deletion receipt: `texture-hard-cutover-v0`), sanitized publication
   bundles, export, retrieval search, proposal delivery state, and
   private-derivative/proposal flows.
6. The source/Wire substrate has current code in `cmd/sourcecycled`,
   `internal/cycle`, `internal/sourcefetch`, `internal/sourcecontract`,
   `internal/sources`, runtime content/source entity handling, and frontend
   source panels/viewers. `source_search` can query Source Service for
   researcher turns when configured, and Texture can preserve
   `source_service_item:<id>` refs, but there is not yet a user-facing
   Wire app over an edition Texture, subscription/event stream, newsletter
   pipeline, or durable per-source scheduling proof.

Active hardening:

1. Keep public desktop and auth-on-mutation verified on staging as the source
   system changes land.
2. Make Texture/researcher/super/user edit flows smoother, more observable, and
   less dependent on timing luck, while preserving the existing single-writer
   and machine-verifiable revision contract.
3. Turn source/Wire from substrate into a prominent Wire product surface with
   real edition Textures, userland personalization, and later newsletter and
   radio-queue projections.
4. Harden publication UX and review: retraction, supersession, route
   management, richer review evidence, export polish, and proposal
   inbox/acceptance flows.
5. Deepen Pretext-based responsive rendering and transclusion for published
   `texture`, computational essays, evidence reports, and web content.
6. Add richer citation mechanics.

Target-only direction:

1. CHIPS and citation/compute economics.
2. Choir Base/File Provider sync and native desktop surfaces.
3. Radio, voice input/output, watch-first screenless control, and native mobile
   radio/control apps.

Later layers should shape today's data model, but they should not be built in a
way that weakens the existing Texture/source/publication contract.

## Product Loop

The current intended core loop is:

```text
prompt -> conductor -> texture -> researcher/persistent super -> cosuper -> texture versions
```

Then:

```text
selected private versions/artifacts -> publication -> citation graph
```

Then:

```text
citations + compute accounting -> CHIPS economics
```

## Texture Contract

`texture` is the first appagent and the version-native control plane. It replaces
retired chat as the main surface for multiagent work.

The target shape of `texture` is hypermedia, not flat text. A `texture` should be
able to become a computational essay or owner-readable campaign/report packet:
prose plus typed snippets for images, audio, podcasts, video, web captures,
PDF/EPUB excerpts, code diffs, interactive graphics, animations, Trace excerpts,
run-acceptance records, app-change packages, candidate demo videos, sources,
and nested Textures. Those snippets are durable artifact references with layout
intent, provenance, and expansion targets, not pasted browser-local state.

Pretext is the preferred layout primitive for this direction: use it for
accurate multiline text measurement, responsive text flow, rich inline
measurement, magazine-style columns, and text wrapping around embedded objects.
Pretext should not become the Texture data model or a replacement for app
ownership. Choir owns the semantic block/snippet model; Pretext helps render
that model with stable responsive geometry.

Every embedded snippet should have two forms:

- an inline or embedded form that reads naturally in the Texture flow;
- an expanded form that opens the owning desktop app/window without losing the
  reader's place in the Texture.

For example, an embedded video expands into the Video app, a podcast excerpt
expands into Podcast, a PDF source excerpt expands into PDF, an image expands
into Image, trace evidence expands into an evidence artifact or Super Console
diagnosis path, and an embedded Texture expands into another Texture window. This
preserves app boundaries while making Texture the composition and reentry surface.

The multi-window desktop is part of the reading model. Sources, demos, and
media should be worth opening because opening them does not destroy the current
reading context. A user reading a computational essay can click through a
source, inspect an animation or candidate demo, play a clip, or compare another
Texture in a new window, then return to the same place in the essay.

The canonical example is an article that is also a computational essay. The
user should be able to ask Choir to make an article that combines argument,
sources, interactive graphics, animations, multimedia clips, generated or
uploaded media, and reviewable evidence. The generated Texture should not flatten
those materials into links at the bottom. It should arrange them as readable
snippets in the essay, with sources and media tempting enough to open because
the desktop preserves context. Clicking a source opens Source Viewer by default;
explicit live/original inspection opens Web Lens; clicking a graphic opens the
owning interactive app or viewer; clicking a nested argument opens another
Texture; clicking a candidate demo opens the review/approval context. This is one
reason Texture must stay a composition surface over typed artifacts instead of
becoming a monolithic media app.

Not every app is an appagent. Apps can be simple desktop surfaces. An app becomes
an appagent when it needs durable domain ownership, prompts, or dynamic agentic
UI. Likely sequence: `texture` first, then source/Web Lens ownership if it needs
durable domain agency, then mail, then calendar.
Trace is no longer a product app direction. Trace remains evidence: structured
events, unified logs, run bundles, acceptance records, and diagnosis artifacts.
Humans should not be expected to browse a retired Trace app to debug Choir.

A `texture` version is a canonical document state:

- `v0` is the initial user input. For prompt-bar flow this is the raw owner
  prompt; for direct Texture editing this is the user-authored document body.
- `v1` is Texture's first response to `v0`, created through the Texture edit
  path. It may be a draft/seed, an acknowledgement, a work-state revision, a
  blocker, or a substantive edit depending on the request.
- `v2+` are user edits and later Texture-authored revisions.

The conductor must not write the first appagent document version. It routes the
prompt, creates or opens the Texture document shell, preserves the user's seed,
and starts Texture. Texture writes the first response version. The prior
"conductor creates an initial seed" policy is superseded because it blurred the
single-writer boundary. Conductor may materialize the `v0` owner prompt as
canonical input, but Texture owns `v1`.

For existing user-authored Texture documents, the current user revision is
already canonical document state. A follow-up owner request should not force a
trivial cleanup patch before Texture can delegate, wait, or reason. If Texture
needs background work, the next revision should honestly represent that work
state instead of hiding it in Trace.

The target Texture loop is deliberately small:

```text
prompt -> conductor route -> v0 owner input -> Texture writes v1 response
  -> Texture sends durable co-agent messages when needed
  -> workers reply with durable updates/evidence
  -> Texture wakes and writes the next version
```

The complexity should live in durable agent-to-agent communication and evidence,
not in prompt taxonomies, conductor-authored drafts, tool-choice classifiers, or
hidden workflow state machines.

Workers do not send patches to `texture`. That mixes concerns. Workers emit
updates: findings, evidence, source references, artifact refs, branch/commit
refs, preview refs, test results, questions, constraints, or proposal summaries.
The `texture` appagent/writer decides whether and how those updates become a new
document version.

For candidate coding work and human approval, the default owner-review artifact
should be video-first when the behavior is visual or temporal. A candidate
approval Texture should embed a short demo video when available, then provide the
summary, package/diff refs, verifier status, rollback path, risks, and links to
evidence bundles or run acceptance. Diffs and logs are still important, but
they should not be the only human proof for interactive product behavior.

The first implementation can create a new `texture` revision after each meaningful
worker update. That policy should be isolated so it can later debounce, batch, or
delay revisions when the user is not attending the latest version. Correctness
must not depend on the debounce policy.

The UI should show current document state for each version, not a temporal feed
of agent status updates. A single user prompt may produce many versions: tens
now, hundreds soon, eventually thousands.

## Machine-Verifiable Texture

The hardest near-term problem is verification. The core behavior should be
testable without real providers, browsers, or timing luck.

Required deterministic tests:

- Prompt creation produces one document with `v0` owner prompt/user input and a
  started Texture writer run.
- Texture creates `v1` through the same Texture edit path used for later appagent
  revisions; conductor cannot create appagent-authored document text.
- Owner-triggered work that cannot complete immediately produces an honest
  acknowledgement/work-state revision instead of a trivial instruction cleanup.
- User edits always create user-authored versions.
- Worker updates are durably attached to the document trajectory.
- A `texture` revision records which worker updates it consumed, skipped, or left
  pending.
- A stale worker result cannot overwrite or erase a later user-authored version.
- User edits redirect future synthesis.
- Unified logs/evidence can explain prompt -> conductor -> texture -> worker
  update -> version during development/debugging.

Browser/e2e tests should verify integration, but the product contract should be
proven by deterministic backend/API tests with fake providers, fake workers, and
a fake clock.

## Agent Roles

`conductor` receives top-level user and connector input. It decides whether to
open an app, show a toast, or route to another flow. It does not mutate workspace
state. In the current Texture path, its only agent delegation target is `texture`;
it does not spawn `researcher`, `super`, or `cosuper`. Those document-work
requests begin after `texture` owns the document.

`app` means a user-facing desktop surface. An app does not have to be an
appagent.

`appagent` means a user-facing app with durable domain ownership, usually because
it needs prompts, dynamic behavior, or canonical state. Appagents mutate their
own typed app state through product APIs. They do not get broad shell or
arbitrary filesystem mutation by default.

Every desktop app that has user-visible view state must participate in the
shared app-state protocol. The shell owns a universal `app_context` persistence
path for each window: apps hydrate from the context they receive, emit typed
context/state changes back to the shell, and the shell saves that state through
the server-backed desktop API. Folder selection, selected document/message,
reader position, media source identity, candidate review selection, and similar
view state must not live only in browser component variables. Per-app code can
define the shape of its typed context, but persistence and reload semantics must
use the universal shell/API path.

`texture` is the single writer for canonical document versions. It synthesizes user
edits and worker updates into durable document state.

`researcher` reads local files and the web, then writes findings/evidence to
Dolt. Researcher does not own canonical document text.

`super` is the per-user foreground orchestration root for resource-heavy or
mutable execution. The useful distinction is authority: `super` can request
`vmctl` resources such as background/candidate worker worlds and promotions.

`vsuper` is the sovereign worker inside a background/candidate computer or candidate world. It
may mutate candidate state within scope and may spawn local cosupers inside that
VM boundary. It cannot promote canonical state.

`cosuper` is a durable execution co-agent, usually running inside a background
computer or under a vsuper. Only `super`/`vsuper` authority can request or assign
cosuper work. Legacy lease wording is H019 residue unless it is explicitly about
capacity/QoS rather than actor control. Cosupers should not be treated as
one-shot subagents that disappear without live coordination.

`worker` is the general category for delegated agents such as researcher, super,
cosuper, and future specialized workers with their own tools.

For now, high reliance on `super` and `cosuper` is acceptable. Getting the
factory working end to end matters more than perfect least privilege. Repeated
privileged actions should later become narrower tools, workers, or appagents.

## Computer Model

The product noun is **computer**. A computer is a persistent user-owned
machine-world composed from VM/runtime state, Dolt/app state, source/build
state, content blobs, artifact provenance, and route identity. See
[computer-ontology.md](computer-ontology.md).

The implementation may back a computer with a Firecracker VM, NixOS image,
host-process fallback, worktree, or later substrate. The user-facing object is
still the computer.

VM-backed computers are retained by a typed warmness policy, not by a single
idle timeout. Ordinary primary computers stay
warm while capacity allows, candidates and workers hibernate first, and
configured always-on primary computers have an explicit protected/resume lane.

`active_computer`:

- The user-facing desktop computer.
- Hosts the visible desktop, apps, appagents, per-user embedded Dolt, private
  app state, local files, prompts, and user-specific runtime state.
- May diverge from the platform baseline.
- May expose one singleton Super Console repair app backed by out-of-process
  `zot`; this is repair mode for the active computer, not a normal scripting
  surface.
- Should stay stable and responsive.
- Should not be edited directly by `super`/`cosuper` for risky mutable work.

`background_computer`:

- A fork of the user's active computer for risky, long-running, or mutable work.
- Used for code edits, package installs, tests, builds, deploy prep, generated
  workspace changes, and anything that may destabilize the active desktop.
- Reports results back as artifacts, findings, branch/commit refs, previews,
  tests, and proposed merges.
- Can merge back into the active computer, publish a typed package, or be
  promoted to active while the previous active snapshot remains available for
  rollback.

`candidate_computer`:

- A background computer allowed to mutate and fail.
- Produces findings, traces, diagnostics, AppChangePackages, and recipient
  adoption evidence.
- Does not mutate canonical foreground state directly.
- Becomes canonical only through promotion after verifier contracts and owner
  decision, or remains discardable/archivable with rollback evidence.

`candidate_world` remains the broader substrate-neutral term for a speculative
state branch. A candidate world may be a computer, a worktree, a Dolt branch, a
package branch, or a future substrate.

Shared worker computers are not a current architecture primitive. They may
become a later cost optimization, but they should not complicate the immediate
model. While existing OVH capacity is available, free users can receive
temporary background computer forks gated by capacity. Privacy is a product tier
decision, not a reason to preserve shared workers now.

`platform_vm_pool`:

- A platform-level pool for public/unauthenticated and shared serving work.
- Needed during the publication pass so published `texture` artifacts can be
  served without hydrating a user's private active computer.
- Can host publication readers, public previews, cached renderers, and other
  platform-visible app surfaces.
- Should not be confused with a user's active/background computer model.

## Public Identity And Routing

Public viewing and mutation authority are separate.

```text
choir.news              -> public platform computer surface
choir.news/:handle      -> public user-selected handle surface
custom-domain.example     -> verified alias to a selected public surface
```

Handles are chosen product identities, not privileged account names. A user may
have multiple accounts during testing, and no account receives a special route
because of who owns it. Custom domains are a roadmap value proposition: after a
domain owner proves control, the domain can serve the same published personal
desktop/newspaper surface that would otherwise live under `choir.news/:handle`.

Anonymous users may inspect public surfaces. When they attempt to mutate, Choir
should ask them to register or log in, then create or resume a user-owned active
or candidate computer. Platform/public mutation is a fork/proposal/promotion
path, not direct anonymous mutation of the platform computer.

This section is the current authority for the public-identity roadmap target.

### Routing Invariants

- **Route-over-ComputerVersion (H031):** no product route resolves to a VM or
  desktop instance identity. Routes point at `ComputerVersion = (CodeRef,
  ArtifactProgramRef)` records. The current implementation has a seam in
  `internal/proxy/lineage_route_resolver.go` that still falls back to hard-coded
  platform VM/desktop constants when the `route_profile` parser fails or
  `PROXY_RUNTIME_DB_PATH` is unset; this is a known violation tracked by H031.
- **Timeout hardening (I3):** `vmctl.Client` and `DefaultVmctlTimeout` default to
  60s, `internal/server/server.go` sets `ReadTimeout`/`WriteTimeout` defaults to
  120s, and staging `/api/universal-wire/stories` returns a fast 504 within
  60s for an induced vmctl resolve failure. Bounded path is proven.

## Promotion Paths

Personal promotion changes one user's computer. It may promote a new local Go
binary, Svelte build, app bundle, theme, prompt, package install, Dolt branch,
file/blob, or generated artifact into that user's active computer without a
global platform deploy. It still needs source lineage, typed deltas, verifier
evidence, foreground-tail reconciliation, route/adoption records, and rollback.
Runtime Go and Svelte UI changes should be treated as a matched pair when app
behavior crosses that boundary.

Super Console repair is a user-computer-local inner loop, not platform
promotion. When a user's active computer is broken, its singleton Super Console
may use out-of-process `zot` to inspect unified logs and private source/build
state, patch runtime/UI/app code inside that computer, rebuild/restart locally,
verify, and write a markdown diagnosis report. That local repair may remain
private, later inspire a platform fix, or later be turned into a typed package,
but it is not automatically a platform merge or AppChangePackage.

Platform/public promotion changes shared state: the official Choir baseline,
public packages, publication artifacts, shared app/agent packages, or public
artifact graph state. It requires higher ceremony: verifier contracts,
provenance, compatibility with divergent user computers, rollback, and often
staging/deploy proof.

Source/package publication is not automatically platform deployment. A user may
publish app/runtime/UI deltas from an approved candidate as an
`AppChangePackage`; another user imports that package into their own candidate
computer, rebases it onto their own source ref, builds their own matched
runtime/UI artifacts, verifies, and promotes into their own active computer.
Promotion records must account for foreground-tail changes by naming the target
active source ref at candidate start and cutover, plus merge/rebase evidence
and conflicts.

This migration should be treated as one continuous app-change trajectory, not a
checklist ladder. "Package exists", "candidate built", "record exists", and
"route changed" are observations, not success states, unless the same
AppChangePackage identity is carried through source refs, rebuilt artifacts,
verifier results, authority decisions, rollback, Trace, and run acceptance.

The public/logged-out product surface and new-user default base image should be
modeled as the official platform computer. It can receive admin-controlled,
later governance-controlled, candidate promotions without host redeploy when the
substrate is unchanged. Host NixOS deploy remains for shared substrate behavior.
Later, organizations, schools, communities, and hobbyist groups can publish
their own promoted computer distributions as alternative base images for new
users.

Private user computers may later expose selected public routes, like personal
websites or personal newspapers, but that must be explicit projection with
visibility, provenance, and rollback. It must not expose the whole computer.
Near term, automatic newspaper work should be built in user/candidate computers
and promoted to the platform computer or published through corpusd where
possible. Platform host deploys remain necessary for shared substrate changes
such as gateway APIs, vmctl/runtime protocol, auth/routing/security, and
corpusd service behavior.

The algebraic question for either path is whether active and candidate deltas
from the same base have a valid join. The VM/runtime ledger is usually not
semantically merged as an opaque machine; typed ledgers such as source/build,
Dolt/app state, blobs, and artifact provenance are merged or conflicted, then
the route pointer changes atomically with rollback.

## State Placement

Choir needs multiple state ledgers with different merge laws. The Dolt substrate
is split into two stores that must not be conflated (see D-STORES in
[docs/definitions/og-dolt-heresy-completion-2026-07-08.md](definitions/og-dolt-heresy-completion-2026-07-08.md)):

- **World-wire store:** platform `ObjectGraphStore` at
  `internal/platform/objectgraph_store.go`, served by `corpusd`. The platform
  decision (D-WIRE) is to move this to sql-server mode now; no data migration
  is needed and existing wire data is junk.
- **VM-local embedded store:** one embedded Dolt workspace per user VM at
  `internal/objectgraph/dolt_store.go`. Capsules are designed, not built; the
  workspace currently backs the computer directly. When capsules land, the same
  workspace will be shared by all capsules in that VM. Promotion
  (fork/promote/rollback) is an operation on this embedded store, not a property
  of the world-wire store and not a separate promotion workspace.

The current Dolt/SQLite decision is the D-STORES/D-WIRE contract in this
section and [computer-ontology.md](computer-ontology.md). Earlier decision
records remain available only through Git history.

VM-local embedded Dolt (one workspace per user VM, shared by its capsules once
 capsules are built) currently holds private product state directly:

- desktop/app graph
- appagent state
- `texture` document/version content
- prompts and policies
- local trajectories
- researcher findings and evidence metadata
- publication staging metadata

Promotion (fork/promote/rollback) operates against this embedded store, not
against the world-wire store and not a separate promotion workspace. D-PROMO
is settled for pinned `*sql.Conn` single-writer discipline on the embedded
store: the `TestDoltEmbeddedBranchIsolationPinnedConnection -count=10` bar
passes. The current `DoltPromotionAdapter` is tag-only interim and must not be
enabled in any production promotion flow until the Phase D branch-adapter
conformance binding lands.

Per-user snapshot filesystem holds workspace and file state:

- working trees
- uploaded files
- large media
- build artifacts
- generated outputs
- filesystem aliases or materialized shortcuts for Dolt-backed `texture` documents

`texture` spans both: canonical content lives in embedded Dolt, while the
filesystem should expose natural aliases/shortcuts so documents are discoverable
from the desktop and file browser.

World-wire store (historically misnamed "Platform Dolt") holds the public/source
object graph served by `corpusd`:

- publication proposals, publications, publication versions, and public routes
- public artifact metadata and manifests
- source/retrieval/citation/provenance records and wire/source object graph
  (`og_objects` / `og_edges`)
- consent, review, verifier, and related control records scoped to public/source
  objects
- later CHIPS economy state for public transactions

It does not own VM lifecycle, platform VM pool state, auth/user account state,
candidate-computer identity, promotion rollback, or general compute accounting.
Those belong to platform control ledgers or VM-local embedded stores as named by
their subsystem. Per D-WIRE, the world-wire store moves to sql-server mode for
multi-writer access by proxy/runtime/wire agents. Per D-STORES, this does not
change promotion mechanics: promotion operates on the VM-local embedded store.

Do not keep the whole filesystem in git. Source files under a repo belong to the
source/build ledger. Uploaded files and generated media belong to a
content-addressed blob ledger with Dolt/artifact metadata. Runtime caches and
temporary files are machine state unless converted into typed artifacts.

## Messaging And Routing

Use hot-path delivery for live work:

```text
sender runtime -> channel/queue/transport/relay -> receiver runtime
```

Use durable records for important handoffs, recovery, replay, audit, and
provenance:

```text
append durable handoff -> deliver hot-path payload -> commit effects/events -> ack
```

Do not implement cross-VM routing as:

```text
sender writes platform DB -> receiver polls platform DB -> receiver acts
```

The database is the ledger. It is not the network.

The durable-actor model makes this concrete (target; cutover in progress):
within one runtime, **the database remembers, Go delivers** — sends durably
append to an idempotent update log, then deliver into a Go-channel mailbox,
activating the recipient if cold; nothing polls. Across VMs, the
transactional outbox carries the same semantics over HTTP (at-least-once
visibility, exactly-once ledger effects). Both protocols are model-checked:
`specs/actor_protocol.tla`, `specs/actor_protocol_xvm.tla`. Today's
`channel_messages` + per-turn inbox polling is the legacy path this replaces.

## Provider Neutrality

No LLM provider, search provider, or auth-specific model gateway should be
required by the product architecture. Z.AI, ChatGPT auth, Fireworks, Bedrock, and
future providers are adapters. Stored records should capture provider/model
attribution when available, but product behavior should not require one provider.

Model policy is a runtime object, not a deployment decision. The platform model
catalog records capabilities such as context window, output ceiling, tool use,
and input modalities. A computer-owned policy chooses defaults for roles and may
request provider-specific options such as reasoning effort or an explicit
`max_tokens` budget. The ordinary case should rely on provider defaults where
that provider behaves better without an explicit maximum; Fireworks-hosted
DeepSeek V4 Flash/Pro, for example, are text-only models that remain valid for
coding, research, orchestration, and text-only verification, while Kimi K2.6 or
another multimodal model is required only when a turn actually includes image or
media input.

## Publication Sequence

Publication should be forward-compatible without deciding the whole economic
model now.

Start with private `texture` version history. A publication is an immutable event
over selected private version/artifact refs. The local VM can continue to
accumulate unpublished versions after publication.

Reasonable sequence:

1. Manual publish of one selected version/snapshot.
2. Publish selected ranges or all versions up to version N.
3. Add redaction/projection before publication.
4. Add editions: later local versions can become later public versions.
5. Add collaboration submissions where an author approves or denies proposed
   changes.
6. Add paywalls, delayed retired release windows, subscriptions, or author-gated access.
7. Add CHIPS-mediated incentives and citation economics.

This preserves the purity option of publishing every version without forcing it
as the first behavior.

## UI And Desktop Principles

The UI should be easy to change. The desktop is the demo of the Automatic
Computer, not a fixed visual doctrine.

The root product should show the real desktop to signed-out visitors. Login is a
mutation boundary, not a prerequisite for viewing. Prompt-bar input, file writes,
LLM-backed actions, candidate creation, publication, and promotion should trigger
auth when needed while preserving the user's current intent.

The prompt bar should react optimistically to user input. Simple version: show a
loading toast. Better version: animate or expand the prompt into the new `texture`
window when the conductor opens it.

All apps should eventually support true fullscreen, not only maximized windows.

Web Lens currently still carries browser-session implementation names and
frontend iframe behavior. The product ontology is narrower: durable web-derived
sources should default to Source Viewer/reader artifacts, and Web Lens is an
explicit live/original inspection surface reached from a source object. Backend
browsing work can support Web Lens, source acquisition, and candidate-computer
inspection, but it should not reintroduce a manual general browser as the
primary source-gathering workflow.

Trace should stay out of the default `texture` writing UI and should not remain a
human-facing product app. Relevant evidence should be available as unified logs,
run bundles, acceptance records, and diagnosis artifacts that Texture or Super
Console can open or summarize. The document surface should remain conservative
and clean for writing, research, publishing, and reading.

## Browser Tooling

Do not replace Playwright just to replace Playwright. Keep it for current e2e and
auth-profile testing until a replacement is clearly better.

Backend-browser candidates such as Kuri, Lightpanda, or Obscura may be useful
later for content ingestion, backend browsing, and agent loops. That exploration
is deferred until the texture verification loop is stable.

## Pretext

Pretext is relevant later as a text rendering/transclusion layer for live
documents, published `texture`, and embedded web/media content. It should not become
a blocker for the current verification work.

## What Not To Collapse

Future coding agents should not simplify Choir into:

- retired chat plus a task runner
- one global agent with tools
- one active computer that mutable workers freely edit
- workers patching `texture` text directly
- world-wire store as a global polling bus
- provider-specific product behavior
- publication as a flat export with no version/provenance model

The invariant is simpler:

```text
versioned living documents + appagents + candidate computer execution + durable
provenance + publication/citation readiness
```
