# Choir Current Architecture

**Last updated:** 2026-05-16

This is the current architecture memo for Choir. It is meant to be the first
document read before changing `vtext`, conductor routing, workers, Trace, Dolt,
`vmctl`, publication, or appagent behavior. For current vocabulary, read
[glossary.md](glossary.md). For project direction, read
[project-goals.md](project-goals.md).

## Current Reality

Choir is a durable learning control system over versioned artifacts. The web
desktop is the current general-purpose projection of that substrate, not the
whole ontology. Read [docs/mission-geometry.md](mission-geometry.md) for the
higher-level frame: automatic computer -> automatic newspaper -> automatic radio
-> automatic capital.

The Automatic Computer already exists in deployed form: a web desktop, backend
services, appagents, and NixOS-on-NixOS VM infrastructure. The product object is
a persistent user **computer**, not a disposable sandbox. The current work is not
to invent the product from scratch. The current work is to stabilize the
deployed system around the right causal model.

Choir is not chat and not a generic coding-agent runner. The visible product is
a web desktop with apps. Some apps grow into appagents; most apps can remain
plain display/control surfaces. The hidden product machinery is a dark factory
of researchers, supers, cosupers, background computers, evidence, artifacts, document
versions, candidate worlds, promotion records, and eventually publications,
radio traversals, and citation/economic state.

The operating stance is now staging-first. Meaningful claims about vmctl,
gateway credentials, live model/search calls, background/candidate computers,
platform promotion, rollback, auth/session renewal, and Choir-in-Choir must be proven on
`https://draft.choir-ip.com` after commit, push, CI, deploy, and staging health
identity checks. Local development remains useful for fast frontend iteration
and focused unit shaping, but local proof does not establish product readiness.

This staging-first rule applies to platform behavior and shared runtime claims.
It does not mean every user-local computer change must wait for global CI/deploy.
Users should eventually be able to fork a candidate from their own active
computer, change apps inside that candidate, build a new Go/Svelte runtime,
install packages, and promote the verified candidate back into their own active
computer with local verifier and rollback evidence.

The current promotion architecture is stable platform, divergent computers.
Read
[stable-platform-divergent-computers-architecture-2026-05-17.md](stable-platform-divergent-computers-architecture-2026-05-17.md)
before changing source-lineage, app-package, runtime/UI promotion, platform
computer, or deployment behavior. The short version is:

```text
platform substrate changes -> GitHub main -> CI -> NixOS deploy
user computer changes      -> active source ref -> candidate -> verify -> promote
source/app sharing         -> app change package -> recipient candidate rebuild
new-user default image     -> official platform computer fork
public surface changes     -> platform computer candidate unless substrate changes
private public surface     -> selected route projection, not whole-computer exposure
```

Run acceptance is a first-class artifact. A `RunAcceptanceRecord` should be
synthesized from existing product/control evidence: runs, Trace moments, worker
exports, promotion candidates, verifier contracts, rollback refs, and deployed
build identity. It replaces ad hoc claims like "the Trace looked good."

The embedded Dolt runtime migration is complete on staging. As of commit
`c3b1a4b2547d672eadd9b3d74b76ba9371518648`, per-user runtime/control product
tables and VText tables open in the same embedded Dolt workspace inside the
user computer. The old `/state` path remains a marker and legacy-import source;
fresh accepted staging computers showed no runtime SQLite WAL/SHM pair. Host
auth/session state remains host-owned.

Platform publication now has a first service boundary. A host-side `platformd`
service writes to a separate localhost-only `dolt sql-server` primary and owns
platform-visible publication, route, artifact manifest/blob, retrieval source/
span, citation edge, provenance, consent/review, verifier, and rollback rows.
The browser never talks to Dolt. A signed-in user calls the proxy product API to
publish a selected private VText revision; the proxy reads that revision from
the user's resolved computer and submits only the public projection to
`platformd`. Public published snapshots now resolve through the Svelte Choir
shell and VText app at `/pub/vtext/...`: signed-out visitors get a guest
read-only VText surface, signed-in users can create private derivatives and
proposals, and proxy read APIs fetch sanitized publication bundles from
internal-only `platformd` endpoints. Platform services still never gain write
access to the live private document.

## Priority Order

1. Make the public desktop and auth-on-mutation access model work: signed-out
   users can see the real desktop, while mutable moves ask for identity at the
   boundary and then continue through an owned active/candidate computer.
2. Make `vtext`, researcher, super, and user edits work well and become
   machine-verifiable.
3. Add ingestion skills: URL to extracted text/content, YouTube transcripts, and
   text/Markdown/PDF/EPUB upload. Later add audio/video/image display apps so
   uploaded, linked, or agent-retrieved media can open in the desktop and become
   available for `vtext` transclusion.
4. Harden publication UX and review: retraction, supersession, route
   management, richer review evidence, and proposal inbox/acceptance flows.
5. Deepen Pretext-based text rendering/transclusion for published `vtext` and
   web content.
6. Add richer citation mechanics.
7. Add CHIPS and citation/compute economics.

Later layers should shape today's data model, but they should not be built
before the vtext loop is reliable.

## Product Loop

The core loop is:

```text
prompt -> conductor -> vtext -> researcher/persistent super -> cosuper -> vtext versions
```

Then:

```text
selected private versions/artifacts -> publication -> citation graph
```

Then:

```text
citations + compute accounting -> CHIPS economics
```

## VText Contract

`vtext` is the first appagent and the version-native control plane. It replaces
chat as the main surface for multiagent work.

Not every app is an appagent. Apps can be simple desktop surfaces. An app becomes
an appagent when it needs durable domain ownership, prompts, or dynamic agentic
UI. Likely sequence: `vtext` first, browser next, then mail, then calendar.
Trace can become an appagent later if there are enough trajectories to require
agentic search and dynamic visualization, but for now it is a development/debug
surface.

A `vtext` version is a canonical document state:

- `v0` is the initial user input.
- `v1` is the initial document seed included in the route that creates the
  `vtext`.
- `v2+` are user edits and `vtext`-authored revisions.

The `vtext` agent should not spend one extra LLM call producing an initial
answer from priors before the document opens. The route already has enough
context to create the initial seed. Opening the window should show `v0` as the
user prompt and `v1` as the current seeded state.

Workers do not send patches to `vtext`. That mixes concerns. Workers emit
updates: findings, evidence, source references, artifact refs, branch/commit
refs, preview refs, test results, questions, constraints, or proposal summaries.
The `vtext` appagent/writer decides whether and how those updates become a new
document version.

The first implementation can create a new `vtext` revision after each meaningful
worker update. That policy should be isolated so it can later debounce, batch, or
delay revisions when the user is not attending the latest version. Correctness
must not depend on the debounce policy.

The UI should show current document state for each version, not a temporal feed
of agent status updates. A single user prompt may produce many versions: tens
now, hundreds soon, eventually thousands.

## Machine-Verifiable VText

The hardest near-term problem is verification. The core behavior should be
testable without real providers, browsers, or timing luck.

Required deterministic tests:

- Prompt creation produces one document with `v0` user input and `v1` initial
  document seed.
- No vtext answer-from-priors call is needed to create `v1`.
- User edits always create user-authored versions.
- Worker updates are durably attached to the document trajectory.
- A `vtext` revision records which worker updates it consumed, skipped, or left
  pending.
- A stale worker result cannot overwrite or erase a later user-authored version.
- User edits redirect future synthesis.
- Trace can explain prompt -> conductor -> vtext -> worker update -> version
  during development/debugging.

Browser/e2e tests should verify integration, but the product contract should be
proven by deterministic backend/API tests with fake providers, fake workers, and
a fake clock.

## Agent Roles

`conductor` receives top-level user and connector input. It decides whether to
open an app, show a toast, or route to another flow. It does not mutate workspace
state. In the current VText path, its only agent delegation target is `vtext`;
it does not spawn `researcher`, `super`, or `cosuper`. Those document-work
requests begin after `vtext` owns the document.

`app` means a user-facing desktop surface. An app does not have to be an
appagent.

`appagent` means a user-facing app with durable domain ownership, usually because
it needs prompts, dynamic behavior, or canonical state. Appagents mutate their
own typed app state through product APIs. They do not get broad shell or
arbitrary filesystem mutation by default.

`vtext` is the single writer for canonical document versions. It synthesizes user
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
computer or under a vsuper. Only `super`/`vsuper` authority can lease cosuper work.
Cosupers should not be treated as one-shot subagents that disappear without live
coordination.

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
idle timeout. Current and future priority semantics are documented in
[vm-priority-policy.md](vm-priority-policy.md): ordinary primary computers stay
warm while capacity allows, candidates and workers hibernate first, and
configured always-on primary computers have an explicit protected/resume lane.

`active_computer`:

- The user-facing desktop computer.
- Hosts the visible desktop, apps, appagents, per-user embedded Dolt, private
  app state, local files, prompts, and user-specific runtime state.
- May diverge from the platform baseline.
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
- Produces exports, findings, manifests, patchsets, traces, diagnostics, and
  promotion candidates.
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
- Needed during the publication pass so published `vtext` artifacts can be
  served without hydrating a user's private active computer.
- Can host publication readers, public previews, cached renderers, and other
  platform-visible app surfaces.
- Should not be confused with a user's active/background computer model.

## Public Identity And Routing

Public viewing and mutation authority are separate.

```text
choir-ip.com              -> public platform computer surface
choir-ip.com/:handle      -> public user-selected handle surface
custom-domain.example     -> verified alias to a selected public surface
```

Handles are chosen product identities, not privileged account names. A user may
have multiple accounts during testing, and no account receives a special route
because of who owns it. Custom domains are a roadmap value proposition: after a
domain owner proves control, the domain can serve the same published personal
desktop/newspaper surface that would otherwise live under `choir-ip.com/:handle`.

Anonymous users may inspect public surfaces. When they attempt to mutate, Choir
should ask them to register or log in, then create or resume a user-owned active
or candidate computer. Platform/public mutation is a fork/proposal/promotion
path, not direct anonymous mutation of the platform computer.

Read [public-identity-and-custom-domains.md](public-identity-and-custom-domains.md).

## Promotion Paths

Personal promotion changes one user's computer. It may promote a new local Go
binary, Svelte build, app bundle, theme, prompt, package install, Dolt branch,
file/blob, or generated artifact into that user's active computer without a
global platform deploy. It still needs source lineage, typed deltas, verifier
evidence, foreground-tail reconciliation, route/adoption records, and rollback.
Runtime Go and Svelte UI changes should be treated as a matched pair when app
behavior crosses that boundary.

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
and promoted to the platform computer or published through platformd where
possible. Platform host deploys remain necessary for shared substrate changes
such as gateway APIs, vmctl/runtime protocol, auth/routing/security, and
platformd service behavior.

The algebraic question for either path is whether active and candidate deltas
from the same base have a valid join. The VM/runtime ledger is usually not
semantically merged as an opaque machine; typed ledgers such as source/build,
Dolt/app state, blobs, and artifact provenance are merged or conflicted, then
the route pointer changes atomically with rollback.

## State Placement

Choir needs multiple state ledgers with different merge laws.

Read [adr-dolt-as-canonical-state.md](adr-dolt-as-canonical-state.md) for the
Dolt/SQLite decision record.

Per-user embedded Dolt holds private product state:

- desktop/app graph
- appagent state
- `vtext` document/version content
- prompts and policies
- local trajectories
- researcher findings and evidence metadata
- publication staging metadata

Per-user snapshot filesystem holds workspace and file state:

- working trees
- uploaded files
- large media
- build artifacts
- generated outputs
- filesystem aliases or materialized shortcuts for Dolt-backed `vtext` documents

`vtext` spans both: canonical content lives in embedded Dolt, while the
filesystem should expose natural aliases/shortcuts so documents are discoverable
from the desktop and file browser.

Platform Dolt holds platform-visible state:

- users/accounts/tenants
- VM lifecycle, capacity, and routing records
- platform VM pool records
- publication records
- public artifact metadata
- citation graph
- compute/accounting records
- later CHIPS economy state

Platform Dolt is not the hot-path message bus and not the store for every private
event. Cross-VM work should use direct transport or a relay, then write compact
durable facts for routing, recovery, provenance, publication, citation, and
compute accounting.

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

## Provider Neutrality

No LLM provider, search provider, or auth-specific model gateway should be
required by the product architecture. Z.AI, ChatGPT auth, Fireworks, Bedrock, and
future providers are adapters. Stored records should capture provider/model
attribution when available, but product behavior should not require one provider.

## Publication Sequence

Publication should be forward-compatible without deciding the whole economic
model now.

Start with private `vtext` version history. A publication is an immutable event
over selected private version/artifact refs. The local VM can continue to
accumulate unpublished versions after publication.

Reasonable sequence:

1. Manual publish of one selected version/snapshot.
2. Publish selected ranges or all versions up to version N.
3. Add redaction/projection before publication.
4. Add editions: later local versions can become later public versions.
5. Add collaboration submissions where an author approves or denies proposed
   changes.
6. Add paywalls, delayed release windows, subscriptions, or author-gated access.
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
loading toast. Better version: animate or expand the prompt into the new `vtext`
window when the conductor opens it.

All apps should eventually support true fullscreen, not only maximized windows.

The browser app currently depends on frontend iframe behavior. Backend-browser
work can be deferred, but the eventual browser app should run browsing on the
backend so it can bypass iframe blockers and so users can inspect previews from
background/candidate computers, including Choir running inside a browser app.

Trace should stay out of the default `vtext` writing UI. There may be a hidden
deep link from a `vtext` version or menu into the relevant Trace trajectory, but
the document surface should remain conservative and clean for writing, research,
publishing, and reading. Worker-vtext internals belong in Trace by default, not
as a prominent `vtext` panel.

## Browser Tooling

Do not replace Playwright just to replace Playwright. Keep it for current e2e and
auth-profile testing until a replacement is clearly better.

Backend-browser candidates such as Kuri, Lightpanda, or Obscura may be useful
later for content ingestion, backend browsing, and agent loops. That exploration
is deferred until the vtext verification loop is stable.

## Pretext

Pretext is relevant later as a text rendering/transclusion layer for live
documents, published `vtext`, and embedded web/media content. It should not become
a blocker for the current verification work.

## What Not To Collapse

Future coding agents should not simplify Choir into:

- chat plus a task runner
- one global agent with tools
- one active computer that mutable workers freely edit
- workers patching `vtext` text directly
- platform Dolt as a global polling bus
- provider-specific product behavior
- publication as a flat export with no version/provenance model

The invariant is simpler:

```text
versioned living documents + appagents + candidate computer execution + durable
provenance + publication/citation readiness
```
