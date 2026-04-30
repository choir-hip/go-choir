# Implementation Scope

**Last updated:** 2026-04-30

This is the near-term build order. For the complete current architecture, read
[docs/current-architecture.md](current-architecture.md).

## First Priority: Verifiable VText

The current product bottleneck is:

```text
prompt -> conductor -> vtext -> researcher/super/cosuper -> user edits -> versions
```

Required behavior:

- The prompt bar routes through `conductor`.
- `conductor` opens `vtext` by creating `v0` from user input and `v1` from a
  short conductor framing note.
- `vtext` does not need an extra initial answer-from-priors call before the
  window opens.
- User edits create user-authored versions.
- Workers emit updates, not patches.
- `vtext` decides whether worker updates become new document versions.
- Initial revision policy can be one version per meaningful worker update, with
  later debouncing/batching allowed.
- Trace explains the causal path during development/debugging.

Machine-verifiable tests should use fake providers, fake workers, and fake time
before relying on browser/e2e coverage.

## Second Priority: Ingestion

After the vtext loop is reliable, add ingestion skills and apps:

- URL to extracted text/content
- YouTube transcript pulling
- text and Markdown upload
- PDF upload
- EPUB upload
- later audio, video, and image display apps

These should feed `vtext` through typed app/worker updates and durable artifacts,
not ad hoc prompt stuffing. Media display apps do not need to be appagents at
first; they become appagents only if they need durable prompts, dynamic UI, or
domain ownership.

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
