# VText Durable Draft Version Graph Eval Report

**Date:** 2026-05-25  
**Deployed commit:** `b2252fe4ecc9f05f827ca3c86e2703ada68d4820`  
**Target:** `https://draft.choir-ip.com`

## Summary

The first durable-draft behavior change is deployed and staging-proven:
stale user draft saves can now rebase over a moved VText head when the client
explicitly sends `allow_rebase`, while ordinary stale writes still return
conflict. The deployed proof shows a user draft based on revision `A` rebasing
onto newer head `B`, preserving both the incoming head update and the dirty
draft text, with lineage metadata on the resulting user revision.

The model suite was run on staging for:

- `fireworks-deepseek-v4-flash-medium`
- `fireworks-kimi-k2p6-low`
- `chatgpt-gpt-5-5-low`

The suite covered deep research, coding/super, and a long multi-section VText
prompt. All three models completed the deep research and coding/super rows. The
long row required reruns because the first long observation hit auth expiry for
v4-flash and Kimi; reruns produced usable long-row evidence.

This is a strong checkpoint, not mission completion. Remaining work includes
cross-device UI draft sync proof, more realistic user-edit-plus-worker
concurrency, and a denser long-document quality rubric.

## Deployed Dirty-Draft Proof

Evidence:

- CI run: `26423509844`, success.
- FlakeHub publish run: `26423509853`, success.
- Staging health: proxy and sandbox both reported
  `b2252fe4ecc9f05f827ca3c86e2703ada68d4820`, deployed
  `2026-05-25T23:19:07Z`.
- Acceptance command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_DURABLE_DRAFT_EVIDENCE_DIR=../test-results/vtext-durable-draft-staging-b2252fe-20260525T232239Z pnpm exec playwright test tests/vtext-durable-draft-version-graph.tmp.spec.js --project=chromium --reporter=line`
- Result: `1 passed`.
- Evidence artifact:
  `test-results/vtext-durable-draft-staging-b2252fe-20260525T232239Z/dirty-rebase-product-path.json`

Assertions proven by that artifact:

- current head is the rebased dirty-draft revision;
- dirty draft parent is the moved head;
- rebased content preserves incoming head text;
- rebased content preserves dirty draft text;
- metadata records `rebased_from_revision_id`;
- metadata records `rebase_onto_revision_id`.

## Model Suite Results

Primary evidence directory:

- `test-results/vtext-model-suite-durable-draft-b2252fe-20260525T232318Z/`

Supplemental reruns:

- `test-results/vtext-model-suite-durable-draft-b2252fe-long-rerun-20260525T233946Z/`
- `test-results/vtext-model-suite-durable-draft-b2252fe-v4-long-180s-20260525T234456Z/`

### Deep Research

| Model | Status | V1 | V2 | Revision Count | Tool Errors | Notes |
|---|---:|---:|---:|---:|---:|---|
| v4-flash medium | ok | 3.0s | 26.0s | 2 | 0 | Cleanest row; one researcher spawn, no edit errors. |
| Kimi low | ok | 4.3s | 41.3s | 3 | 0 | Produced an extra follow-up revision; no coordination noise. |
| GPT-5.5 low | ok | 5.3s | 50.3s | 2 | 4 | Useful output, but duplicate evidence/finding submissions remain. |

### Coding/Super

| Model | Status | V1 | V2 | Super Requests | Tool Errors | Notes |
|---|---:|---:|---:|---:|---:|---|
| v4-flash medium | ok | 3.0s | 15.0s | 1 | 0 | Fastest and cleanest coding/super row. |
| Kimi low | ok | 3.4s | 17.4s | 1 | 0 | Clean, slightly slower than v4-flash. |
| GPT-5.5 low | ok | 3.5s | 40.5s | 2 | 1 | Completed, but duplicate bash planning was skipped by guard. |

### Long Multi-Section

| Model | Status | V1 | V2 | Revision Count | Tool Errors | Notes |
|---|---:|---:|---:|---:|---:|---|
| v4-flash medium | ok on 180s rerun | 24.4s | 146.4s | 2 | 3 | Needed longer observation window; malformed researcher-delegation noise appeared. |
| Kimi low | ok on fresh rerun | 48.3s | 109.3s | 2 | 0 | Slow V1, but clean coordination and no tool noise. |
| GPT-5.5 low | ok | 18.6s | 93.6s | 2 | 2 | Best long-row content volume; duplicate cast messages were skipped. |

## Current Comparison

For this checkpoint, `fireworks-deepseek-v4-flash-medium` remains the best
default conductor/VText/research cadence candidate for shorter research and
coding/super tasks: it is fast and usually clean. Its long multi-section row
needs more work: it eventually produced a second revision with a longer wait,
but showed malformed delegation noise and less rich long-form synthesis.

`fireworks-kimi-k2p6-low` is the cleanest coordination model in this suite. It
had no tool errors across the successful rows and handled long multi-section
work cleanly, but its first long revision was slow.

`chatgpt-gpt-5-5-low` produced the richest long multi-section output and worked
again after account auth was restored, but it still creates duplicate side-effect
attempts: duplicate evidence/finding submissions, duplicate bash planning, and
duplicate cast messages. Runtime guards kept those from corrupting VText state,
but this is still coordination noise.

## Residual Risks

- This report does not yet prove a two-device UI draft sync workflow before
  `Revise`; it proves VM-backed product API persistence and local browser dirty
  rebase behavior.
- The long multi-section prompt is still a coarse content-quality check. It does
  not yet assert section-by-section obligations beyond revision timing and trace
  noise.
- Worker-update concurrency was exercised through model behavior, not through a
  deterministic product-path worker storm with user dirty edits at the same time.
- The model-suite harness can still hit auth expiry on long serial runs; fresh
  reruns mitigated this, but the observer should renew sessions before the final
  full mission pass.

## Recommendation

Keep `fireworks-deepseek-v4-flash-medium`, `fireworks-kimi-k2p6-low`, and
`chatgpt-gpt-5-5-low` in the next eval round.

Use v4-flash medium as the default latency baseline, Kimi low as the clean
coordination baseline, and GPT-5.5 low as the long-form quality comparator. Do
not optimize conductor cost/latency yet; the next mission pressure should be on
dirty user edit plus worker-update concurrency and long-document section-level
correctness.
