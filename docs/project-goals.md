# Choir Project Goals

**Status:** canonical current goals
**Last updated:** 2026-06-09

This file replaces the old root `PROJECT-GOALS.md`. It keeps the live direction
from that file while removing completed checklists and stale continuation
instructions.

## Core Goal

Make the deployed Automatic Computer coherent enough that Choir can improve
Choir from inside Choir.

The system should receive user intent, route it through `conductor`, accumulate
meaning in `vtext`, delegate execution through bounded workers, mutate
candidate computers, verify candidate deltas, and promote only when the target
state is safe, useful, and rollbackable.

The current product vector remains:

```text
automatic computer -> automatic newspaper -> automatic radio -> automatic capital
```

The current concrete product language is:

```text
private AI cloud -> Wire -> radio -> later capital/economic surfaces
```

Wire is the reusable source-to-VText substrate. Universal Wire is the public
Choir Community Cloud instance. Private Choir Clouds run their own Wire
instances on client-controlled NixOS hosts with private platform computers, many
user computers, private sources, and publication/subscription boundaries.

Do not implement token mechanics or capital surfaces now. Preserve provenance,
evidence, artifact, citation, trajectory, computer lineage, model, publication,
and compute-usage facts so later projections remain possible.

## Current To Ideal Continuum

Current state:

- Deployed web desktop, auth, proxy, gateway, vmctl, runtime service, Svelte UI,
  app surfaces, and VText exist.
- Prompt-bar routing, worker delegation, run memory, worker export, promotion
  queue, and run-acceptance records exist in partial/product-slice form.
- Background/candidate computers and promotion are not yet first-class enough to
  safely run large Choir-in-Choir missions without Codex-style observation and
  repair.
- Documentation now distinguishes product computers from implementation
  sandboxes and separates personal promotion from platform/public promotion.

Near target:

- Prompt bar always routes through `conductor`.
- `conductor` routes to `vtext` or another appagent without frontend shortcut
  policy becoming the source of truth.
- `vtext` is the primary cumulative semantic artifact.
- Researchers, super, vsuper, and cosupers leave enough unified evidence that
  VText reports, acceptance records, and Super Console/zot can explain what
  happened without a human browsing Trace.
- Candidate computers can run bounded work, export typed deltas, and produce
  promotion certificates.
- Personal promotion can update one user's computer without a global deploy.
- Platform behavior-changing work still lands through commit, push, CI, deploy,
  staging identity check, and deployed product-path evidence.

Ideal direction:

- Users evolve their own computers quickly from inside Choir.
- Useful local changes can become typed packages or public proposals.
- Platform updates merge into divergent user computers ledger by ledger.
- VText becomes the semantic substrate for publication and radio traversal.
- VText becomes a multimedia computational-essay surface where durable snippets
  can embed and expand into the desktop's source, media, evidence, and app
  windows without losing the reader's place.
- The artifact graph records evidence, claims, deltas, verifiers, promotions,
  failures, and reuse so future work starts with more structure than the last.

## Product Goals

### VText

`vtext` should feel like the document itself:

- one primary version-native writing surface;
- computational-essay support: prose, sources, interactive graphics,
  animations, images, audio, podcasts, video, web captures, PDF/EPUB excerpts,
  code diffs, evidence, and nested VTexts as typed snippets;
- magazine-quality responsive reading built on Choir's document model and
  Pretext-powered layout/measurement where appropriate;
- embedded snippets that can expand into their owning desktop app/window while
  preserving the VText reading position;
- minimal chrome;
- user edits become user-authored versions;
- appagent synthesis becomes agent-authored versions;
- worker updates become evidence for synthesis, not patches blindly applied;
- version history, citations, artifacts, and publication refs remain durable.

### Conductor

`conductor` should be the owner of top-level routing:

- prompt-bar input enters through `conductor`;
- simple outcomes can be lightweight UI responses;
- document work usually routes to `vtext`;
- future connector, email, schedule, or watch events should also enter through
  conductor before moving to the relevant appagent or researcher.

### Multiagent Runtime

The MAS should become visible and trustworthy:

- `vtext` can spawn researchers for current/external information;
- `super` handles foreground orchestration and bounded capability minting;
- `vsuper` owns candidate computers;
- cosupers are leased within explicit boundaries;
- unified logs/evidence record delegation chains, tool calls, messages, and
  synthesis points;
- workers return structured updates, deltas, evidence, diagnostics, or
  questions.

### Computers And Promotion

The core self-development path is:

```text
active computer
-> candidate computer
-> typed deltas and evidence
-> verifier contracts
-> promotion certificate
-> route switch with rollback
```

Personal promotion should support local app changes, runtime service binaries,
Svelte builds, prompt packages, themes, file/blob artifacts, Dolt branches,
source/build bundles, and generated media without requiring global CI/deploy.

Platform/public promotion should remain higher ceremony because it changes the
official baseline, public packages, publication graph, or deployed behavior.

### Desktop And Apps

The desktop should behave like an actual computer surface, not a marketing page
or chat wrapper:

- a launcher/start-button flow for apps;
- stable desktop icons and windows;
- Files with upload support and typed artifact handoff;
- Browser rationalized around the real web-surface constraints;
- Settings as the place for user preferences, prompts, themes, and provider
  policy where appropriate;
- Podcast/Radio as an early proof that `vtext` can become screenless traversal;
- multi-window reading as a first-class affordance: sources, snippets, media,
  Trace, and related VTexts can open beside the current essay without collapsing
  the user's place or task context;
- theme creation/editing as a user-facing demonstration of local computer
  divergence and personal promotion.

Apps do not need to become appagents immediately. They become appagents when
they need durable domain ownership, prompts, dynamic UI, or semantic memory.

### Public Identity And Domains

Public viewing should not require login. Mutable work should require identity at
the boundary where the user tries to write, run an LLM-backed action, create a
candidate, publish, or promote.

Default public routes should be:

```text
choir.news              -> platform/public surface
choir.news/:handle      -> user-selected public handle surface
```

No user receives a special path by personal identity. Handles are selected and
owned product records. The stretch goal is custom domain support, where a
verified domain such as `mosiah.org` aliases to the user's selected public
computer/newspaper surface. See
[public-identity-and-custom-domains.md](public-identity-and-custom-domains.md).

### Ingestion, Publication, And Radio

After the `vtext` loop is reliable, add ingestion and publication pressure:

- URL/content extraction;
- YouTube transcript pulling;
- text, Markdown, PDF, EPUB, image, audio, and video uploads;
- media display apps whose content can be embedded as typed VText snippets and
  expanded into their full app windows;
- computational essays that combine prose, citations, interactive or animated
  graphics, multimedia evidence, source excerpts, and nested VTexts;
- immutable publication events over selected private versions/artifact refs;
- citation graph mechanics over published refs;
- radio traversal over promoted meaning.

Wire should be treated as reusable infrastructure, not a bespoke news app:

- Universal Wire ingests public sources and publishes public source-backed
  VTexts and editions.
- Private Wire instances ingest private and subscribed public sources for
  firm, matter, research, market, and executive briefings.
- User computers run user-level processors and reconcilers to personalize
  accessible corpora into user-owned VTexts.

### State Model

Follow the ledger split:

- Dolt owns canonical product state by default.
- SQLite remains narrow hot runtime, cache, compatibility, or transitional
  storage when justified.
- Source/build changes live in source ledgers or typed packages.
- Uploaded/generated content lives in content-addressed blob storage with
  Dolt/artifact metadata.
- Runtime caches and temp files are machine state unless converted into typed
  artifacts.

Read [adr-dolt-as-canonical-state.md](adr-dolt-as-canonical-state.md).

## Near-Term Mission Pressure

1. Make the public desktop/auth-on-mutation slice real enough that users can see
   Choir before logging in, but cannot mutate without identity.
2. Make returning-account active computers warm, recoverable, observable, and
   pressure-aware so users do not see black screens or pay avoidable cold-start
   latency, without weakening isolation or credential boundaries.
3. Keep primary user computers online while capacity is available, reclaim
   lower-priority idle resources first under pressure, and model a future
   premium 24/7 uptime class before billing or product UI makes it harder to
   add correctly.
4. Instrument the lifecycle and load dynamics of public, new-account, and
   returning-account paths so optimization is driven by deployed p50/p95/p99
   evidence, stochastic/progressive load, and UX-visible waiting states rather
   than anecdote.
5. Keep documentation canonical enough that long-running agents do not follow
   stale mission files.
6. Make run acceptance cover promotion-level and continuation-level self
   development, not just export-level evidence. `continuation-level` is
   transitional: it is being re-pointed at trajectory/work-item settlement
   evidence under the durable-actors rearchitecture
   (`docs/choir-rearchitecture-durable-actors-2026-06-11.md`, portfolio M4);
   until that lands, the current continuation-level evidence requirement
   stays in force.
7. Make candidate computer lineage, typed delta export, verifier contracts, and
   rollback certificates first-class.
8. Use Playwright/Codex to prompt Choir to develop Choir once the product path
   is safe enough, with Codex observing, learning, and repairing when needed.
9. Build the small missing product surfaces that make this demonstrable:
   launcher, Files upload, theme editing, podcast/radio improvements, and
   browser surface rationalization.

These are not a brittle ladder. The order can deform as evidence changes, but
the invariants should not: stable foreground, mutable candidates, typed deltas,
verification, promotion, rollback, and durable learning.

## Absorbed Historical Signal

This file absorbs live signal from the deleted root `PROJECT-GOALS.md` and old
Mission 1/2/3/5/6/7 docs:

- Deploy and provider hardening are now covered by `README.md`, `AGENTS.md`,
  [runtime-invariants.md](runtime-invariants.md), and staging-first evidence
  rules.
- Service topology and product APIs are covered by `README.md` and
  [current-architecture.md](current-architecture.md).
- Desktop UX goals are represented here as app/launcher/windowing goals, while
  old component-specific rewrite instructions stay in git history.
- Cogent remains a reference for tool loops and work-control ideas, not a
  permanent external control plane.
- VM-only language is superseded by the computer ontology.

## Non-Goals For The Current Phase

Do not build now:

- token mechanics;
- wallets;
- staking;
- public citation scoring;
- token-denominated billing;
- decentralized inference markets;
- automated carry/revenue accounting;
- backend-browser replacement as a general solution without a specific product
  verification path.

These concepts explain what information to preserve. They are not the next code
target.
