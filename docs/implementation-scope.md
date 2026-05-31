# Implementation Scope

**Last updated:** 2026-05-31

This is the near-term build order. For the complete current architecture, read
[docs/current-architecture.md](current-architecture.md). For the broader goal
continuum, read [docs/project-goals.md](project-goals.md).

## Immediate Priority: Public Desktop And Auth-On-Mutation

The immediate product bottleneck is now the access model:

```text
public desktop view
-> mutation intent
-> login/register overlay
-> authenticated active/candidate computer
-> product-path mutation
```

Required behavior:

- Signed-out visitors see the real desktop shell, not a marketing placeholder.
- Anonymous read-only inspection is allowed for public platform/user surfaces.
- Prompt-bar submission or any durable mutation asks for auth at the boundary.
- The typed prompt or mutation intent survives register/login.
- After auth, mutation resumes through the normal product APIs and the user's
  active or candidate computer.
- Loading and LLM-call states are visible, recoverable, and tied to real request
  or run state where possible.
- Existing signed-in accounts still open the desktop and can always reach
  logout.

This mission is defined in
[mission-public-desktop-auth-on-mutation-v0.md](mission-public-desktop-auth-on-mutation-v0.md).

## Next Priority: Verifiable VText

The current product bottleneck is:

```text
prompt -> conductor -> vtext -> researcher/persistent super -> cosuper -> user edits -> versions
```

Required behavior:

- The prompt bar routes through `conductor`.
- `conductor` opens or creates the VText document shell and preserves the user
  seed, but does not write appagent document text.
- `vtext` writes `v1` through the VText edit path. This is the first canonical
  artifact version.
- User edits create user-authored versions.
- Workers emit updates, not patches.
- `vtext` decides whether worker updates become new document versions.
- Initial revision policy can be one version per meaningful VText synthesis or
  worker update, with later debouncing/batching allowed.
- Unified logs/evidence and Super Console repair reports explain the causal path
  during development/debugging; humans should not need to browse Trace.

Via negativa for the next VText repair: remove prompt/classifier/state-machine
scaffolding before adding more. The simple target is durable co-agent messages
plus the VText single-writer revision loop.

Machine-verifiable tests should use fake providers, fake workers, and fake time
before relying on browser/e2e coverage.

## Second Priority: Ingestion And Real Readers

After the vtext loop is reliable, deepen ingestion skills and real reader/media
apps:

- URL to extracted text/content
- YouTube transcript pulling
- text and Markdown upload
- PDF upload
- EPUB upload
- real PDF reading rather than object/embed-only opening
- real EPUB archive parsing rather than extracted-text-only reading
- app-grade audio, video, and image controls/state

These should feed `vtext` through typed app/worker updates and durable artifacts,
not ad hoc prompt stuffing. Media display apps do not need to be appagents at
first; they become appagents only if they need durable prompts, dynamic UI, or
domain ownership. The current common platform app state is tracked in
[platform-os-app-state.md](platform-os-app-state.md).

## Third Priority: Publication

Publication starts as an immutable event over selected private `vtext`
version/artifact refs. Local private history can continue beyond what is
published.

Do not decide the whole publication economy now. Preserve forward compatibility
for publishing selected snapshots, ranges, all versions up to N, redacted
projections, later editions, collaboration submissions, paywalls, and CHIPS
incentives.

## Later Priorities

1. Pretext-based rendering/transclusion for text, published `vtext`, web content,
   and multimedia references.
2. Citation graph mechanics over published immutable refs.
3. CHIPS and citation/compute economics.

## Current Non-Goals

Do not implement now:

- CHIPS token mechanics
- wallets
- staking
- public citation scoring
- token-denominated billing
- decentralized inference markets
- automated carry/revenue accounting
- backend-browser replacement for Playwright

These concepts explain why the architecture must preserve provenance, citations,
artifacts, versions, and compute usage. They are not the next code target.

## Simplification Rules

Future coding agents must not simplify Choir into:

- chat plus task runner
- one global agent with tools
- workers patching `vtext` directly
- mutable work on the live desktop
- SQLite-only runtime truth
- single-VM-only assumptions
- platform Dolt as a global message bus
- provider-specific product behavior
