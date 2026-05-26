# VText Durable Draft Version Graph Eval Report

**Date:** 2026-05-25  
**Deployed commit:** `2cf7253954aa5f67f7251fd22f4946ed0adb40ec`  
**Target:** `https://draft.choir-ip.com`

## Summary

The durable-draft behavior change is deployed and staging-proven:
stale user draft saves can now rebase over a moved VText head when the client
explicitly sends `allow_rebase`, while ordinary stale writes still return
conflict. The deployed proof shows a user draft based on revision `A` rebasing
onto newer head `B`, preserving both the incoming head update and the dirty
draft text, with lineage metadata on the resulting user revision.

The follow-up deployed fix tolerates noisy provider wrapper text around
`spawn_agent` role/profile values only when the value unambiguously contains one
allowed delegate target. This repaired the v4-flash medium worker-follow-up
probe: after a dirty user marker was injected, a researcher update woke VText,
VText produced a second appagent revision, and the final head preserved the
exact user marker while recording consumed worker-update metadata.

The model suite was run on staging for:

- `fireworks-deepseek-v4-flash-medium`
- `fireworks-kimi-k2p6-low`
- `chatgpt-gpt-5-5-low`

The final cadence suite covered deep research, coding/super, and a long
multi-section VText prompt on deployed commit `2cf7253`. All three model rows
completed that Playwright harness. The result is usable for landscape
comparison, but not a clean product-quality certificate: v4-flash did not
produce a second deep-research VText revision inside the observation window,
v4-flash long hit one external `fetch_url` timeout, and GPT-5.5 low still made
duplicate side-effect attempts.

A stricter long-section rubric added after that suite exposed a coordination
gap that the cadence suite did not catch: mixed research-plus-execution prompts
can be routed into researcher-first VText loops where later VText revisions
integrate researcher findings but never deterministically call
`request_super_execution`. Kimi low prematurely satisfied the text rubric
without a super trace. v4-flash medium and GPT-5.5 low were more conservative,
leaving command evidence pending, but they also produced no super agent in
Trace. The mission therefore remains `checkpoint_incomplete` rather than
complete.

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

Additional deployed proof after the noisy-delegation fix:

- Behavior commit: `2cf7253954aa5f67f7251fd22f4946ed0adb40ec`.
- CI run: `26424849935`, success.
- FlakeHub publish run: `26424849948`, success.
- Staging health: proxy and sandbox both reported
  `2cf7253954aa5f67f7251fd22f4946ed0adb40ec`, deployed
  `2026-05-26T00:06:30Z`.
- Worker-concurrency command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_DURABLE_DRAFT_EVIDENCE_DIR=../test-results/vtext-durable-draft-worker-concurrency-staging-2cf7253-20260526T000659Z pnpm exec playwright test tests/vtext-durable-draft-version-graph.tmp.spec.js --project=chromium --grep 'worker-driven VText follow-up' --reporter=line`
- Result: `1 passed`.
- Evidence artifact:
  `test-results/vtext-durable-draft-worker-concurrency-staging-2cf7253-20260526T000659Z/dirty-user-edit-worker-followup.json`

Assertions proven by that artifact:

- final head preserved the exact user marker;
- at least two appagent revisions existed;
- one researcher worker update was recorded as consumed;
- Trace contained researcher and VText agents using v4-flash medium from
  `/mnt/persistent/files/System/model-policy.toml`.

Additional two-researcher worker storm proof:

- Command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_DURABLE_DRAFT_EVIDENCE_DIR=../test-results/vtext-durable-draft-multi-worker-staging-a2fe62f-20260526T003647Z pnpm exec playwright test tests/vtext-durable-draft-version-graph.tmp.spec.js --project=chromium --grep 'two researcher worker updates' --reporter=line`
- Result: `1 passed`.
- Evidence artifact:
  `test-results/vtext-durable-draft-multi-worker-staging-a2fe62f-20260526T003647Z/dirty-user-edit-two-worker-updates.json`

Assertions proven by that artifact:

- final head preserved the exact user marker;
- three appagent revisions existed;
- Trace contained two researcher agents using Kimi low;
- two worker updates from two distinct researcher senders were consumed across
  VText revisions;
- final revision recorded later worker updates as pending instead of hiding or
  dropping them.

Additional pending-drain rerun:

- Command:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_DURABLE_DRAFT_EVIDENCE_DIR=../test-results/vtext-durable-draft-multi-worker-drain-staging-a2fe62f-20260526T005412Z pnpm exec playwright test tests/vtext-durable-draft-version-graph.tmp.spec.js --project=chromium --grep 'two researcher worker updates' --reporter=line`
- Result: `1 passed`.
- Evidence artifact:
  `test-results/vtext-durable-draft-multi-worker-drain-staging-a2fe62f-20260526T005412Z/dirty-user-edit-two-worker-updates.json`

Assertions proven by that artifact:

- final head preserved the exact user marker;
- five appagent revisions existed;
- Trace contained two researcher agents using Kimi low;
- four worker updates from two distinct researcher senders were consumed across
  VText revisions;
- latest head had `worker_updates_pending: []`.

## Model Suite Results

Primary evidence directory:

- `test-results/vtext-model-suite-durable-draft-2cf7253-20260526T000948Z/`

Earlier supplemental reruns retained for comparison:

- `test-results/vtext-model-suite-durable-draft-b2252fe-20260525T232318Z/`
- `test-results/vtext-model-suite-durable-draft-b2252fe-long-rerun-20260525T233946Z/`
- `test-results/vtext-model-suite-durable-draft-b2252fe-v4-long-180s-20260525T234456Z/`

### Deep Research

| Model | Status | V1 | V2 | Revision Count | Tool Errors | Notes |
|---|---:|---:|---:|---:|---:|---|
| v4-flash medium | partial | 22.8s | not captured | 1 | 0 | Spawned researcher and got findings, but no second VText revision inside the observation window. |
| Kimi low | ok | 4.8s | 35.8s | 2 | 0 | Clean coordination and a timely follow-up revision. |
| GPT-5.5 low | ok | 5.3s | 51.3s | 2 | 1 | Useful follow-up; one duplicate evidence submission. |

### Coding/Super

| Model | Status | V1 | V2 | Super Requests | Tool Errors | Notes |
|---|---:|---:|---:|---:|---:|---|
| v4-flash medium | ok | 18.4s | 89.4s | 2 | 0 | Clean tool behavior, slower than the earlier run. |
| Kimi low | ok | 3.6s | 15.6s | 2 | 0 | Cleanest and fastest coding/super row in the final run. |
| GPT-5.5 low | ok | 3.8s | 23.8s | 4 | 1 | Completed; duplicate bash planning was skipped by guard. |

### Long Multi-Section

| Model | Status | V1 | V2 | Revision Count | Tool Errors | Notes |
|---|---:|---:|---:|---:|---:|---|
| v4-flash medium | ok | 30.4s | 159.4s | 2 | 1 | No malformed delegation after the fix; one external `fetch_url` timeout. |
| Kimi low | ok | 39.5s | 73.5s | 3 | 0 | Clean coordination; shorter V2 latency than v4 and GPT in the final run. |
| GPT-5.5 low | ok | 7.0s | 89.0s | 4 | 4 | Richest revision volume, but duplicate cast/evidence submissions remain. |

## Long Section Rubric

Evidence directory:

- `test-results/vtext-long-section-rubric-staging-2cf7253-full-20260526T010650Z/`

Command:

`PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_LONG_RUBRIC_EVIDENCE_DIR=../test-results/vtext-long-section-rubric-staging-2cf7253-full-20260526T010650Z pnpm exec playwright test tests/vtext-long-section-rubric.tmp.spec.js --project=chromium --workers=1 --reporter=line`

The prompt asked VText to create a 12-section source-grounded brief, preserve a
dirty user marker inserted after v1, use researcher grounding, and request
super to run `printf "durable draft" | shasum -a 256`. The expected SHA256 was
included as a rubric target, but the row only counts as passing when Trace shows
super execution and the final document contains the required section-update
sentences and command evidence.

| Model | Status | App Revisions | Trace Researchers | Trace Supers | Rubric Shape | Notes |
|---|---:|---:|---:|---:|---|---|
| Kimi low | failed | 2 | 1 | 0 | 12/12 sections, marker preserved, `[S1]`-`[S3]` and `[CMD]`, section updates present | Prematurely wrote the expected hash and command marker from prompt text without a super trace. |
| v4-flash medium | failed | 5 | 2 | 0 | 12/12 sections, marker preserved, no source markers, no command hash, no section-update sentences | Produced many grounded revisions and consumed researcher updates, but super stayed pending and no `request_super_execution` trace appeared. |
| GPT-5.5 low | failed | 4 | 2 | 0 | 10/12 detected sections, marker preserved, source markers present, no section-update sentences | Strong source-grounded content and batching, but command evidence stayed pending and no super trace appeared. |

Root-cause hypothesis from trace evidence:

- The first mixed research-plus-execution turn prefers researcher-first
  continuation.
- Later VText turns wake from researcher deliveries and keep editing from
  research evidence.
- Researchers sometimes try to `cast_agent` or ask for super themselves, but
  the authority boundary expects VText to call `request_super_execution`.
- No tested model reliably bridges that handoff in the strict long-document
  prompt, so the runtime/prompt contract needs an explicit "research complete
  enough, super still required" continuation guard.

## Current Comparison

For this deployed state, `fireworks-deepseek-v4-flash-medium` remains viable as
the default latency/cost baseline and now survives the noisy role-wrapper case
that previously blocked researcher spawn. It is not the clean winner: in the
final deep-research row it spawned a researcher and got findings but did not
produce a second VText revision inside the observation window, and its long row
needed 159.4s for V2.

`fireworks-kimi-k2p6-low` is the cleanest coordination model in this suite. It
had no tool errors across the final rows, handled long multi-section work
cleanly, and produced the best final coding/super cadence.

`chatgpt-gpt-5-5-low` produced the richest long multi-section output and worked
again after account auth was restored, but it still creates duplicate side-effect
attempts: duplicate evidence/finding submissions, duplicate bash planning, and
duplicate cast messages. Runtime guards kept those from corrupting VText state,
but this is still coordination noise.

## Residual Risks

- Cross-session draft visibility before `Revise` is now staging-proven through
  two authenticated browser contexts, but not yet through two physical devices.
- The long multi-section prompt is still a coarse content-quality check. It does
  not yet assert section-by-section obligations beyond revision timing and trace
  noise.
- Worker-update concurrency is proven for one researcher update arriving after a
  dirty user revision and for a two-researcher storm with consumed/pending
  metadata. The stricter rerun also proves eventual drain of the latest pending
  set for that two-researcher storm.
- The model-suite harness can still hit auth expiry on long serial runs; fresh
  reruns mitigated this, but the observer should renew sessions before the final
  full mission pass.
- Current-events content quality still needs source-truth checking. The worker
  proof preserved the user marker and consumed worker evidence correctly, but
  the final Artemis II content included contradictory launch-status claims that
  should not be treated as verified factual output.

## Recommendation

Keep `fireworks-deepseek-v4-flash-medium`, `fireworks-kimi-k2p6-low`, and
`chatgpt-gpt-5-5-low` in the next eval round.

Use Kimi low as the clean coordination baseline, v4-flash medium as the
cost/latency baseline that still needs cadence tuning, and GPT-5.5 low as the
long-form quality comparator with duplicate side-effect guards kept on.

Do not spend effort on conductor micro-optimization yet. The next mission
pressure should be many-version long documents, multi-worker storms, and
source-grounded current-events quality gates.
