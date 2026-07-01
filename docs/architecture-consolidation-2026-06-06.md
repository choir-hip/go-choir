# Architecture Consolidation - 2026-06-06

**Status:** cleanup ledger  
**Purpose:** preserve valid architecture signal from older broad sketches while
removing distracting superseded docs.

## Deleted

- `docs/architecture.md`
- `docs/multiagent-architecture.md`

## Why Delete

`docs/architecture.md` was a 2026-04-10 design sketch for a unified multiagent
system. It already warned readers to prefer `docs/current-architecture.md`,
`docs/north-star.md`, `docs/runtime-invariants.md`, and
`docs/implementation-scope.md`.

`docs/multiagent-architecture.md` was a smaller 2026-04-20 local runtime shape.
It also warned readers that the newer docs take precedence. Its useful role
graph and API boundary notes had already been absorbed into current architecture
and runtime invariants.

Keeping either file as a large live file was harmful because they mixed
still-useful service topology with stale local-first framing, old sandbox
ontology, old scheduler language, pre-current VText lifecycle assumptions, and
migration planning that has already happened or changed.

## Mined Into Current Architecture

The valid signal was folded into
[current-architecture.md](current-architecture.md):

- Caddy is the edge/static router.
- `auth`, `proxy`, `gateway`, `vmctl`, and `corpusd` are narrow host/platform
  services.
- Per-user computer runtime owns private conductor, appagent, VText, Trace, run
  memory, app state, source metadata, and candidate-control product state.
- Browser callers use product APIs, not raw agent/prompt/event/vmctl/Dolt
  internals.
- Provider secrets remain in the gateway/platform boundary while per-computer
  model policy selects among declared capabilities.
- VM lifecycle is surfaced only through redacted product APIs.
- Platform publication receives selected projections and never writes live
  private documents.
- Runs and agents are distinct: a run is one execution, while a durable agent
  identity such as `vtext:<doc_id>` can span multiple runs.
- For VText work, `channel_id = doc_id` remains the document-family
  coordination handle.
- Tool access and delegation targets are code policy, not prompt-only advice.
- Browser-visible product paths are `POST /api/prompt-bar`,
  `/api/prompt-bar/submissions/{id}`, read-only `/api/trace/*`, and product
  VText APIs such as `/api/vtext/documents/{id}/revise`; `/api/agent/*` remains
  non-product/internal.

## Superseded Signal

The following ideas from the old sketch should not be carried forward as
current authority:

- treating `sandbox` as the product ontology rather than an implementation name;
- treating a shared scheduler abstraction as the main product object;
- pre-current conductor/VText bootstrap language;
- old migration phasing from choiros-rs/cogent into Go;
- stale local-first framing where staging/product-path proof now controls;
- old API surfaces that expose implementation control rather than product
  intent.
