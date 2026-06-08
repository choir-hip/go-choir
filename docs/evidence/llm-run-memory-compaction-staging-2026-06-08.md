# LLM Run-Memory Compaction Staging Evidence - 2026-06-08

## Scope

This evidence records deployed product-path proof for
`docs/mission-llm-run-memory-compaction-v0.md` after commit
`aa8bee5fe5500d48841cef054b9ab8b449929e4e` reached staging.

Acceptance environment: `https://choir.news`.

## Deploy Identity

Staging `/health` during both Playwright proofs reported:

- proxy commit: `aa8bee5fe5500d48841cef054b9ab8b449929e4e`
- upstream sandbox commit: `aa8bee5fe5500d48841cef054b9ab8b449929e4e`

GitHub Actions for the same SHA:

- CI run `27163306315`: success
- FlakeHub push run `27163306209`: success

## Product-Path LLM Checkpoint Proof

Command:

```text
PLAYWRIGHT_BASE_URL=https://choir.news \
LLM_COMPACTION_EVIDENCE_PATH=/tmp/choir-llm-compaction-staging-proof.json \
npx playwright test tests/llm-compaction-staging-proof.tmp.spec.js --project=chromium --reporter=line
```

Result: passed in 41.8s.

Observed product-path ids:

- prompt-bar submission: `103926d5-4892-49b6-9e30-5c8a5991381c`
- VText document: `795c15d7-b6f3-4170-bfa0-0e2ea90c867c`
- compacted source loop: `5bf5e7d6-953e-44cd-952c-14ad734f2cd1`
- continuation record: `345fe622-2918-4ead-a9a9-c42549b40168`
- run-memory compaction entry: `1a9e1b1b-03ea-4b4d-ac38-da885d33eaf7`

Trace artifact summary:

- compaction reason: `continuation_selection`
- compaction entry kind: `compaction`
- runtime model on compaction entry: `deepseek-v4-flash`
- checkpoint status: `llm_checkpoint`
- checkpoint provider: `deepseek`
- checkpoint model: `deepseek-v4-flash`
- threshold tokens: `700000`
- raw compacted entry ids: `["6f3a5cf1-29ec-4507-af4d-783c192ded47"]`
- checkpoint text included `get_run_memory_entry` retrieval guidance.

## Product-Path Exact Retrieval Proof

Command:

```text
PLAYWRIGHT_BASE_URL=https://choir.news \
LLM_COMPACTION_RETRIEVAL_EVIDENCE_PATH=/tmp/choir-llm-compaction-retrieval-staging-proof.json \
npx playwright test tests/llm-compaction-retrieval-staging-proof.tmp.spec.js --project=chromium --reporter=line
```

Result: passed in 1.1m.

Observed product-path ids:

- source prompt-bar submission: `65224b93-9ff0-4c57-a9a2-9c0339ca416e`
- source VText document: `1deb9054-35f9-44ec-902a-ec451d1e761a`
- compacted source loop: `407a1799-8221-4a2c-83ff-88ec6f9540ba`
- continuation record: `e0763f80-7383-47cf-8563-c29ecd5256e9`
- run-memory compaction entry: `382289c6-d071-4ab8-a7bf-489f6f29af8b`
- raw compacted entry id: `95ae652b-93fb-4653-a101-b50458abf03f`
- retrieval prompt-bar submission: `fb7cdb96-c447-4e83-bf91-b644f6d98560`
- retrieval VText document: `db5f68f2-dda2-4410-95f8-d8d80ae45b1c`
- retrieval VText loop: `500ea20d-ddf9-4ef4-b697-68271f983d69`

Compaction trace artifact summary:

- compaction reason: `continuation_selection`
- compaction entry kind: `compaction`
- runtime model on compaction entry: `deepseek-v4-flash`
- checkpoint status: `llm_checkpoint`
- checkpoint provider: `deepseek`
- checkpoint model: `deepseek-v4-flash`
- threshold tokens: `700000`
- raw compacted entry ids: `["95ae652b-93fb-4653-a101-b50458abf03f"]`
- checkpoint text included `get_run_memory_entry` retrieval guidance.

Retrieval trace evidence:

- moment `544e73a1-cd33-429e-9bc8-f32d218a97f9`: `tool.invoked`, summary `invoked get_run_memory_entry`
- moment `5c4bda7c-0cb1-471f-a39e-035ea1e2837c`: `tool.result`, summary `get_run_memory_entry returned`
- the public Trace moment detail for the tool result contained the exact
  sentinel that had only appeared in the compacted raw source run-memory entry.

## Interpretation

This proves the deployed product path can:

- create a real VText agent run through the normal prompt-bar path;
- compact that run through the runtime's LLM checkpoint path using DeepSeek;
- record the selected model's 1M context-window derived threshold as `700000`;
- preserve exact raw run-memory entry handles in the checkpoint;
- expose the checkpoint through public Trace artifacts;
- drive a later VText agent to call `get_run_memory_entry`; and
- recover exact pre-compaction content by handle.

This did not spend tokens to naturally exceed a 700k estimated prompt-pressure
threshold on staging. Automatic threshold selection is covered by local runtime
tests and the deployed checkpoint recorded the correct threshold value; staging
proof used the public continuation-selection path to force compaction without a
700k-token live run.
