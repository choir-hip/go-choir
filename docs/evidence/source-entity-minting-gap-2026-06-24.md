# Problem: Source Entity Minting Gap on Non-WorkerWake Revisions

**Date:** 2026-06-24
**Discovery context:** Staging QA for commit `078f7018` (Texture source fix)
**Severity:** Blocks mission settlement — source_ref citations cannot work
**Fix commit:** `21d02301` (deployed to staging `2026-06-24T04:02:32Z`)
**Status:** Fix deployed, staging verification pending

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

### Secondary root cause: URL parsing failure in researcher fallback

The researcher fallback (`synthesizeResearcherUpdateIfMissing` in
`internal/runtime/researcher_checkpoint_fallback.go`) extracts URLs from
`web_search` results and passes them through
`coagentSourcesFromTypedEvidenceRefs`. But
`coagentSourceFromTypedEvidenceRef` in
`internal/runtime/tools_worker_update.go:874` could not parse plain HTTP URLs:

1. `splitTypedWorkerUpdateRef` splits `https://example.com/article` on `:`
   giving `key="https"` which `normalizeWorkerUpdateRefKey` doesn't recognize
   → returns `""`, URL silently dropped.
2. `looksLikeArtifactPath` matches URLs (they contain `/`) and would
   misclassify them as `file_artifact` refs before the HTTP URL check.

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

## Fix

**Commit:** `21d02301` — two changes:

### 1. URL parsing fix (`internal/runtime/tools_worker_update.go`)

Check `isHTTPURL` before `looksLikeArtifactPath` in
`coagentSourceFromTypedEvidenceRef`. Plain HTTP URLs now return a `web_url`
packet source directly instead of being silently dropped or misclassified as
file artifacts.

### 2. Remove `workerWake` gate (`internal/runtime/texture_agent_revision.go`)

`evidenceSourceEntitiesAndRejectionsFromPendingUpdates` is now called
unconditionally (not gated behind `workerWake`). Pending `update_coagent`
packets in the Texture agent's mailbox are checked on every revision, not just
worker-triggered ones.

### Test (`internal/runtime/tools_test.go`)

`TestResearcherFailureSynthesizesCheckpointAfterSearch` extended to verify:
- Web_search result URLs appear in the fallback packet's `sources[]`.
- The packet sources produce source entities via
  `evidenceSourceEntitiesFromWorkerUpdates`.

## Verification Instructions

**Target:** `https://choir.news` (staging, commit `21d02301`)

### Step 1: Verify staging commit identity

```
curl -s https://choir.news/health | jq '.build.commit, .upstream_build.commit'
```

Both must report `21d02301e226efa4684abc88d17ce200555cf397`.

### Step 2: Create a Texture document with a source-citing prompt

1. Open `https://choir.news` in a browser.
2. Create a fresh passkey account (do not reuse existing sessions).
3. Submit a prompt-bar request: "Write a brief article about a current event
   with cited sources."
4. Wait for the conductor to create a Texture document and the initial
   revision to complete (may take 30-90 seconds including research time).

### Step 3: Inspect the head revision for source entities and citations

After the Texture document settles (no more active revisions for ~30 seconds):

1. Open the Texture document in the UI.
2. Fetch the head revision via the Texture API (e.g.,
   `GET /api/texture/documents/<doc_id>/revisions`). Inspect the API
   response — not the `metadata` field, but the top-level revision fields:
   - `.source_entities` — count the array length; must be > 0.
   - `.body_doc` — parse the body document and count `source_ref` nodes
     (inline citations). Must be > 0.
   - `.body_doc` — count `expanded_ref` nodes. These render as
     expanded/collapsed source displays in the UI.
3. Verify the article body contains inline numbered citations (not just
   source names in prose).
4. If the model produced a model-prior/interim first revision (v2) without
   citations, wait for the researcher fallback to trigger a second revision
   (v3) that should include source entities and citations. The park-on-idle
   mechanism (default 2 minutes) should keep the Texture agent alive long
   enough to receive the researcher's `update_coagent` packet.

### Step 4: Grep staging codebase for residual issues

These greps check for leftover references to the old `source_embed` tool
name and the old `WireTexture` component name. Known allowed matches are
excluded. Uses `git grep` for line-level matching (avoids symlink-directory
errors and file-level false negatives from `git ls-files | xargs grep`).

```bash
# source_embed: should only appear in comments and test guards.
# Filter out comment lines at line level.
git grep -n 'source_embed' -- ':(exclude)*_test.go' ':(exclude)*.md' \
  | grep -v '^[^:]*:[0-9][0-9]*:[[:space:]]*//' \
  | grep -v '^[^:]*:[0-9][0-9]*:[[:space:]]*/\*'

# WireTexture: use word matching to avoid UniversalWireTexture substring.
# Exclude Universal Wire production code and tests.
git grep -n -w 'WireTexture' -- ':(exclude)*_test.go' ':(exclude)*.md' \
  | grep -v 'universal_wire' | grep -v 'UniversalWireApp'
```

Both should return empty. `source_embed` is allowed in code comments
(e.g., `tools_texture.go` line ~1366). `WireTexture` is allowed in
Universal Wire production code (`universal_wire.go`,
`UniversalWireApp.svelte`) and tests.

### Step 5: Report

Report PASS/FAIL for each step with evidence:
- Staging commit hash and deploy timestamp.
- Texture doc ID and head revision ID.
- `.source_entities` array length, `source_ref` node count in `.body_doc`,
  `expanded_ref` node count in `.body_doc`.
- Whether the article body contains inline numbered citations.
- Screenshots of the document with citations rendered.
- Grep results.

### Notes for the reviewer

- The fix has two parts: (a) URL parsing so web_search URLs become packet
  sources in the researcher fallback, and (b) removing the `workerWake` gate
  so pending packets are always checked.
- The researcher fallback fires when a researcher run completes without
  sending `update_coagent`. It synthesizes a packet from the last successful
  research tool result (e.g., `web_search`).
- The Texture agent parks for 2 minutes after idle, waiting for coagent
  updates. If the researcher sends `update_coagent` within that window, the
  Texture agent wakes and produces a new revision with source entities.
- If the researcher sends `update_coagent` explicitly (not via fallback), the
  same minting path applies — the packet sources are collated into source
  entities by `evidenceSourceEntitiesAndRejectionsFromWorkerUpdates`.
- The first revision (v2) may be a model-prior/interim without citations.
  Look for v3 or later for the sourced revision. If only v2 exists and it
  has no sources, that is a FAIL.
- Keep the worktree clean: `git status --short` should show no changes after
  verification.
