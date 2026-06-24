# Problem: Source Entity Minting Gap on Non-WorkerWake Revisions

**Date:** 2026-06-24
**Discovery context:** Staging QA for commit `078f7018` (Texture source fix)
**Severity:** Blocks mission settlement — source_ref citations cannot work

## Evidence

Staging QA on `https://choir.news` (commit `078f7018`, deployed
2026-06-24T02:49:52Z):

- Fresh passkey account, prompt: "Write a brief article about a current event
  with cited sources."
- Texture doc: `c2ea5c4f-8f67-4729-9a66-3071ced22c61`
- Revision: `d5ffc87b-82e0-4944-b9aa-94bd043c89e3` (v2)
- Research ran: web_search via parallel, 16 results.
- Article format: PASS (coherent article, not Q&A).
- `source_entities_len: 0`
- `source_ref count: 0`
- `expanded_ref count: 0`
- Model named sources in prose (Reuters, BBC, CNN, NPR) but never called
  `insert_source_ref`.

Screenshots: `/tmp/choir-staging-source-proof-2026-06-24T03-06-04-220Z/`

## Root Cause

Source entities from research are only minted when `workerWake` is true.

`internal/runtime/texture_agent_revision.go:363`:
```go
workerWake := scheduledMessageSeq > 0 || strings.HasPrefix(strings.TrimSpace(req.Intent), "integrate_")
```

`internal/runtime/texture_agent_revision.go:386`:
```go
if workerWake {
    evidenceEntities, sourceRejections := rt.evidenceSourceEntitiesAndRejectionsFromPendingUpdates(...)
```

The staging QA flow was: user prompt → Texture agent → research (web_search) →
revision. This is a single-pass flow:

1. `scheduledMessageSeq` was 0 (no worker update delivery to the Texture
   agent).
2. Intent did not start with `integrate_`.
3. `workerWake` was false.
4. `evidenceSourceEntitiesAndRejectionsFromPendingUpdates` was never called.
5. `source_entities_len = 0`.
6. The model had research results in its context but no registered source
   entities to cite. `insert_source_ref` requires a `source_entity_id` that
   resolves to a registered entity. With zero entities, the model correctly
   avoided calling the tool and instead named sources in prose.

## Analysis

The source entity minting pipeline exists and works:
- `evidenceRecordToSourceEntity` converts evidence records to source entities.
- `evidenceSourceEntitiesFromWorkerUpdates` collates entities from coagent
  update packets.
- `evidenceSourceEntitiesFromPendingUpdates` reads pending worker updates from
  the mailbox.

But the pipeline is only activated on `workerWake` — when a worker update is
delivered to the Texture agent's mailbox and triggers a revision. In a
single-pass flow (user prompt → research → revision), the researcher's findings
may not be delivered as a structured `update_coagent` packet before the
Texture agent produces its revision.

The gap is not in the prompt or schema — it is in the runtime wiring. The
model cannot cite sources that are not registered as source entities.

## Belief State

- **What works:** article-format guidance (unconditional prompt text), schema
  validation, tool definition, grep cleanliness.
- **What doesn't work:** source entity minting on non-workerWake revisions.
- **Conjecture 1 status:** weakened. The bridge (unconditional article-format
  guidance) produces article-format output but does not produce source
  citations without source entities in the run context.

## Remaining Error Field

- Source entities must be minted from research results even when
  `workerWake` is false.
- The researcher's web_search results must be converted to source entities
  available to the Texture agent's `patch_texture` tool.
- The model must see source entities in its run context to call
  `insert_source_ref`.
