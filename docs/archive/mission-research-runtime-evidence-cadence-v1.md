# MissionGradient: Research Runtime Evidence Cadence v1

**Status:** draft
**Date:** 2026-05-25
**Supersedes:** the pruned VText runtime progress cadence v0 mission frame
**Purpose:** make Choir's research substrate fast, evidence-preserving, model-portable, and suitable as the bedrock for autopaper.

## One-Line Goal String

```text
/goal Run docs/mission-research-runtime-evidence-cadence-v1.md as a Codex-operated MissionGradient mission: make Choir's research -> reporting -> VText flow autopaper-grade. Treat VText cadence as one consumer of a broader research evidence substrate: full evidence remains durable in Trace/content/artifacts, while researcher, VText, Chyron, and owner UI receive consumer-specific compact projections from the same event. Instrument and optimize per-prompt and per-model latency from prompt submit -> v1 -> first search/fetch result -> first findings -> v2/v3, including tool-output bytes/estimated tokens, gateway fanout latency, gateway-owned provider cooldown behavior, model/provider config, channel updates, and Chyron events. Split model-visible tool results from durable full payloads, add safe retrieval refs, tune gateway provider fanout/cooldowns, add Parallel as an additional search provider after API probes, and evaluate prompt/tool-description variants that encourage pipelined research: report the first useful findings while continuing search/fetch in parallel. Test across the scoped available model range at low/no effort: Fireworks DeepSeek V4 Flash/Pro/Kimi K2.6, ChatGPT gpt-5.5 low/no reasoning, ChatGPT gpt-5.4-mini no/low reasoning, and Bedrock Haiku only; skip Z.ai, Bedrock Sonnet, and Bedrock Opus unless explicitly authorized. Preserve the uniform shared harness unless a divergence is explicitly approved. Do not overtruncate without durable refs, spam VText with raw logs, leak provider-health policy into researcher cognition, hide rate limits/provider failures from gateway/Trace evidence, replace existing search providers with Parallel, pay for generic HTML-to-markdown extraction, or claim local-only success. Land through git/CI/deploy and prove on staging with current-events, weather/sports, linked-source, autopaper-shaped, and mixed prompts showing faster useful VText iterations, bounded model-visible tool payloads, gateway-owned provider adaptation, rollback refs, residual risks, and the next realism axis.
```

## Mission Frame

The v0 cadence mission found that `v1 -> v2` latency is not only search time.
It is the whole product path:

```text
prompt submitted
  -> conductor routes
  -> VText writes a working v1
  -> VText spawns researcher or super
  -> researcher model decides tools
  -> search/fetch providers respond
  -> tool results are fed back into the researcher
  -> researcher submits findings
  -> VText wakes
  -> VText writes v2/v3
```

The right artifact is therefore not "faster VText" alone. It is a research
evidence flow with multiple consumers and one durable truth.

Autopaper depends on this. DeepSeek V4 Flash is currently the affordable
research model, but the system must not overfit to it. Search providers will
change. New indices will be added. Some sources will be expensive, long-tail,
non-English, domain-specific, or slow. The runtime needs to preserve evidence
quality while making the common path fast and observable.

## Core Ontology

One research event can have several projections:

```text
full evidence
  Durable full payload: raw provider response, fetched content, hashes,
  source refs, timings, costs, provider metadata, and enough provenance to
  replay or audit. Stored in Trace/content/artifact substrate.

researcher projection
  Moderate model-visible packet: compact ranked results, bounded snippets,
  enough source metadata to reason about reliability, retrieval refs for deeper
  reads, and only a concise gateway status note when provider conditions affect
  evidence quality or next steps.

VText projection
  Low-volume owner-readable findings: strongest claims, source names/URLs,
  uncertainty, open questions, and what changed since the prior revision.

Chyron projection
  Minimal live activity: "searched NBA news: 2 providers, 10 results",
  "researcher sent first findings", "fetching NBA.com", not raw logs.

owner UI / Trace projection
  Inspectable causal evidence: drill-in cards, copyable logs, provider status,
  model config, costs, retries, and full refs for debugging.
```

These should be projections of the same event, not separate inconsistent
records. The consumer should determine verbosity, not the tool implementation
silently dropping information.

## Core Invariants

```text
Full evidence is durable before it is compressed.
Compressed projections must name what was omitted and how to retrieve it.
VText remains the single canonical document writer.
Researcher and super send substantive updates; they do not author VText.
Chyron streams granular activity; it is not canonical memory.
Trace remains the dense causal ledger and debugging surface.
The shared tool loop/harness stays uniform across roles unless human-approved evidence requires divergence.
Provider failures and rate limits are gateway-owned operational evidence, not hidden noise.
Parallel is additive, not a replacement for existing search providers.
```

## Value Criterion

Minimize time to useful, grounded VText updates while preserving evidence
fidelity, auditability, model portability, and provider diversity.

The mission moves uphill when:

- time to useful v1 stays low;
- first useful findings arrive sooner after the first search/fetch result;
- VText writes smaller useful v2/v3 revisions instead of waiting for final coverage;
- model-visible tool payloads are bounded and readable;
- durable full evidence remains retrievable;
- gateway fanout improves quality without unnecessary latency;
- rate-limited providers cool down automatically inside the gateway;
- Chyron shows live progress without stealing canonical responsibility;
- behavior holds across Fireworks, ChatGPT, and other configured models at low/no effort.

## Design Hypothesis

The likely improvement path is a four-part gradient:

1. **Measure before changing.** Record output bytes, estimated tokens, provider
   timings, provider attempts, first result time, first findings time, and
   first VText update time.
2. **Project tool outputs.** Store full responses durably, but pass compact
   consumer-specific projections forward.
3. **Pipeline research.** Encourage the researcher to submit first findings in
   the same subsequent turn that starts the next useful search/fetch branch.
4. **Tune gateway provider policy.** Use fanout, cooldowns, and Parallel
   selectively based on measured quality/latency/cost, not provider ideology.

## Evidence Tiers

### Tier 0: Full Durable Evidence

Stored in Trace/content/artifacts, not necessarily passed to the model:

- raw normalized provider response;
- provider-specific raw fields when valuable;
- request params, timing, status, cost when available;
- source URL/title/date/provider/score;
- response byte length and hash;
- truncation/projection metadata;
- parent run/trajectory/doc ids;
- retrieval id for later chunk or full-source access.

### Tier 1: Researcher Projection

Passed back into the researcher model after a tool call:

- query/objective;
- source diversity summary;
- result count and omitted count;
- top bounded result cards;
- bounded snippets/excerpts;
- source reliability signals when available;
- retrieval refs for deeper fetch/read steps;
- explicit note if full evidence is larger than visible output.
- concise gateway status only when degraded provider availability affects
  coverage, freshness, language, or confidence.

This projection should usually be in the low thousands of tokens, not tens of
thousands.

### Tier 2: VText Projection

Sent to VText as `submit_research_findings`:

- 2-6 strongest findings;
- concise source attributions;
- confidence/uncertainty;
- open questions and next branch;
- changes since last packet.

This is narrative input, not raw search output.

### Tier 3: Chyron Projection

Streamed to the prompt bar:

- terse human-readable activity;
- no raw JSON;
- no private tokens;
- no long source excerpts;
- fades behind prompt focus.

Examples:

```text
researcher searched "nba update": 2 providers, 10 results
researcher sent first findings to VText
VText drafting v2 from initial findings
search coverage degraded; gateway skipped a cooling-down provider
```

## Gateway Provider Policy

Provider choice belongs inside the gateway, not in the researcher's cognition.
The researcher should ask for evidence; the gateway should decide which indices
to use, avoid known-bad providers, record provider health, and return a clean
research packet plus durable evidence refs.

The default gateway provider policy should become adaptive:

- fanout can remain `2` as a quality-oriented default only if it is parallel or
  proven not to dominate latency;
- provider calls should expose latency and rate-limit evidence to Trace and
  gateway health surfaces;
- repeated 429/payment/quota errors should put a provider into cooldown for
  product-path searches;
- search should preserve provider differences rather than over-normalizing
  away useful fields;
- normalized common fields should coexist with provider-specific extras in full
  durable evidence;
- the model-visible projection should be clean regardless of provider-specific
  raw shape.
- researcher-visible tool output should not ask the researcher to manage
  provider health, rotate providers, or reason about which backend to retry
  unless the gateway reports an evidence-quality blocker.

## Parallel Search Integration Gradient

Parallel should be added as another provider family, not a replacement.

The first probes should establish:

- auth and request shape using `PARALLEL_API_KEY`;
- Search API latency and output shape for current-events, entity, and source
  discovery prompts;
- cost/quotas and failure semantics;
- whether Parallel output is already compact enough to serve as researcher
  projection;
- what raw fields need to be preserved in full evidence.

Expected roles:

- Search API: another search provider, especially for objective-shaped queries.
- Extract API: out of scope except for a future explicit paywalled-content
  pathway. Choir should own ordinary HTML -> text/markdown extraction through
  its own content/Obscura stack rather than paying generic extraction APIs for
  routine pages.
- Task/deep-research API: later, for minutes-long synthesis tasks, not the
  default fast VText path.

Parallel is promising because its docs describe Search as returning ranked,
LLM-optimized excerpts and reducing traditional search -> fetch -> conversion
loops. That claim must be measured in Choir rather than accepted; generic
paid extraction is not part of the ordinary fast path.

## Prompt And Tool-Description Optimization

Use empirical prompt optimization without importing a heavy framework first.
DSPy-style thinking is useful: define the program, define metrics, run examples
across models, and compare prompt/tool-description candidates.

Candidate variants:

- **Baseline:** current prompts and tool descriptions.
- **A: Pipelined researcher prompt:** after the first useful result, submit
  findings and continue the next search/fetch in the same parallel tool batch.
- **B: Tool-description nudge:** make `submit_research_findings` describe
  itself as the normal interim communication primitive, not a final report.
- **C: Projection-aware tool output:** tool result explicitly says "visible
  summary, full evidence ref available."

Avoid for now:

- role-specific tool-loop forks;
- separate scanner/reporter agent split;
- VText reading raw tool results directly;
- hard-coded forced tool calls after search.

## Model Matrix

Run all evals at low or no effort where possible. Do not overfit to
DeepSeek V4 Flash.

At minimum:

- Fireworks DeepSeek V4 Flash;
- Fireworks DeepSeek V4 Pro;
- Fireworks Kimi K2.6;
- ChatGPT `gpt-5.5` with low and no reasoning where supported;
- ChatGPT `gpt-5.4-mini` with no and low reasoning;
- Bedrock Haiku only;
- skip Z.ai entirely until access exists;
- skip Bedrock Sonnet and Opus unless explicitly authorized because they are
  too expensive for routine search evals;
- record unavailable providers as explicit blockers, not silent omissions.

Each model should run the same prompt set where capabilities allow:

- no-search simple prompt;
- weather/current factual prompt;
- sports/news prompt;
- linked-source prompt;
- autopaper-shaped news brief;
- mixed research plus small action/coding prompt.

## Metrics

For every run, record:

- model/provider/reasoning policy;
- prompt class;
- time to route decision;
- time to v1;
- first search/fetch tool invocation;
- first search/fetch result;
- tool output bytes;
- estimated visible tokens;
- durable full evidence bytes/hash/ref;
- first `submit_research_findings`;
- first VText update after findings;
- v2/v3/v4 timings;
- largest active-work silence;
- provider attempts, successes, rate limits, errors, latency;
- Chyron event count and examples;
- Trace copy/log evidence;
- final document quality notes.

Key derived metrics:

```text
first_tool_result -> first_findings
first_findings -> next_vtext_revision
visible_tool_tokens / durable_evidence_bytes
gateway_fanout_latency_delta
rate_limited_provider_reuse_count
model portability delta
```

## Implementation Gradient

### 1. Instrument Current Flow

Patch product-path Playwright/API probes to capture the metrics above on
staging. Do not mutate behavior until the baseline is clear.

### 2. Add Durable Evidence References

Ensure search/fetch tool executions can persist full outputs or source records
with refs and hashes before projection.

### 3. Add Projection Layer

Return compact model-visible tool results while preserving full evidence refs.
Keep the projection schema stable enough for researcher, VText, Chyron, and
Trace to derive their views from one event.

### 4. Tune Researcher Cadence

Evaluate prompt/tool-description variants A/B/C across the model matrix. Prefer
prompt/tool contract improvements over shared harness divergence.

### 5. Tune Gateway Fanout And Cooldowns

Measure whether the current two-provider fanout is sequential latency or
acceptable quality. Add cooldown policy for rate-limited providers. Consider
parallel provider calls if it improves latency without resource or quota harm.
Keep this policy encapsulated in the gateway; expose only clean evidence
summaries to researchers and detailed health evidence to Trace/operator UI.

### 6. Add Parallel Search

Probe Parallel Search locally, then integrate it as an additive search provider
behind the same evidence/projection substrate. Land only after product-path
evidence shows auth, latency, output shape, and failure semantics are
understood. Do not integrate Parallel Extract as part of this mission.

### 7. Prove On Staging

Run the prompt/model matrix on staging and compare against the v0 baseline.
The proof must show both better cadence and preserved evidence fidelity.

## Forbidden Shortcuts

- Dropping full evidence to make context smaller.
- Truncating without explicit omitted counts and retrieval refs.
- Returning raw 100KB tool results to routine model turns.
- Treating Chyron activity as a substitute for VText revision cadence.
- Replacing existing search providers with Parallel.
- Adding generic paid extraction APIs for ordinary HTML -> markdown conversion.
- Making researchers manually manage provider health, cooldowns, or backend
  retry policy.
- Hiding rate-limit or provider failures from gateway/Trace evidence.
- Hard-coding model-specific prompts that only work on DeepSeek V4 Flash.
- Forking the shared tool loop per role without explicit approval.
- Calling local-only probes product success.
- Treating a pretty final VText as proof that early cadence improved.

## Evidence Ledger

Each serious proof should record:

```text
staging commit:
Playwright/API command:
prompt set:
model matrix:
baseline artifact:
post-patch artifact:
gateway provider health artifact:
full evidence refs:
projection examples:
VText revision timeline:
Chyron examples:
Trace/log refs:
rollback refs:
residual risks:
```

## Run Checkpoint And Resumption State

```text
status: checkpoint_incomplete
last checkpoint: deployed d39788e added a model-visible `web_search` projection hint telling researchers to submit findings after useful search results, but the deployed sports/current-events probe regressed to six searches before first findings.
current artifact state: focused runtime/gateway tests pass; staging identity shows d39788e; shared tool-loop instrumentation now records provider request previews, tool names, message roles, model responses, stop reasons, and tool-call names in Trace; the local Playwright cadence probe can summarize those fields from staging evidence.
what shipped: durable tool-output projection envelope, compact web_search/fetch_url model projections, full_output/full_output_sha256 Trace payload fields, gateway quota/rate-limit cooldowns, Parallel Search provider, provider credential deploy preservation, first-pass researcher prompt guidance, shared tool-loop request/response instrumentation, and a `web_search` projection cadence hint.
what was proven: Parallel Search auth/output works locally and on staging; cooldown avoids repeatedly calling exhausted Tavily/Exa/Brave providers; model-visible search outputs are bounded to roughly 1-2k estimated tokens in tested prompts; once findings arrive, VText can write the next version within ~7-10s; the new Trace instrumentation can show exact researcher prompt, response, and tool-call sequence.
unproven or partial claims: cross-model cadence matrix beyond DeepSeek V4 Flash; whether the projection schema is sufficient for longer linked-source/autopaper prompts; whether Chyron projection is wired to the same event stream; whether a generic tool-contract gate can produce first findings without overconstraining all agents.
belief-state changes: provider health was a real gateway problem and is now partially addressed; remaining silence is primarily researcher cadence/oversearching before first findings, not provider latency alone. Prompt-only cadence nudges in the researcher system prompt, VText spawned objective, `web_search` description, and `web_search` projection hint have not reliably changed broad sports/news behavior. More advisory text can add noise and worsen the hard case.
remaining error field: researcher still violates intended first-checkpoint cadence on broad prompts; only two VText revisions appear for long search prompts; model matrix is not yet broad enough; the first useful search result can sit for tens of seconds while the researcher performs additional search-only turns.
highest-impact remaining uncertainty: whether to use the existing generic `next_required_tool` mechanism as a narrow tool-contract after a research search, or whether that would be too blunt and should be replaced by a different event-driven VText wakeup/reporting architecture.
next executable probe: prototype the smallest generic tool-contract gate that does not fork the shared harness: after the first successful `web_search` in a researcher run, require the next tool call to be `submit_research_findings` when that tool is available, then prove with tests and the same staging sports prompt that the sequence becomes `web_search -> submit_research_findings` before any additional search-only turn. If that fails or overconstrains behavior, revert and switch to an event-driven VText wakeup design.
suggested resume goal string: use the One-Line Goal String above.
evidence artifact refs: frontend/test-results/vtext-model-cadence-smoke-20260525T173217Z/fireworks-deepseek-v4-flash-none.json; frontend/test-results/vtext-model-cadence-smoke-cooldown-20260525T173521Z/fireworks-deepseek-v4-flash-none.json; frontend/test-results/vtext-model-cadence-sports-20260525T173734Z/fireworks-deepseek-v4-flash-none.json; frontend/test-results/vtext-model-cadence-sports-promptfix-20260525T174501Z/fireworks-deepseek-v4-flash-none.json; frontend/test-results/vtext-model-cadence-sports-firstpass-20260525T175345Z/fireworks-deepseek-v4-flash-none.json; frontend/test-results/vtext-model-cadence-sports-instrumented-20260525T180626Z/fireworks-deepseek-v4-flash-none.json; frontend/test-results/vtext-model-cadence-sports-projectionhint-20260525T181317Z/fireworks-deepseek-v4-flash-none.json.
rollback refs: platform rollback target before this run is f482da6.
```

## Checkpoint: Required First Findings Helped, But Manual Stalls Remain

```text
status: checkpoint_incomplete
checkpoint date: 2026-05-25
deployed commit under review: 8c18016
new evidence: the required-first-findings tool-contract path improved staging probes, including `nba update` on Fireworks DeepSeek V4 Flash: first VText in ~2.6s, first search result in ~11.6s, first findings in ~18.6s, v2 in ~29.6s, and no provider/tool failures in the captured probe. Breadth probes for simple, weather, linked-source, and mixed prompts mostly passed, though one long serial Playwright run hit a product API 401 while polling a mixed prompt; the mixed prompt passed when rerun alone.
new user-reported failure: a manual staging prompt, `Last Night in Baseball`, stayed on the initial VText revision for several minutes and then surfaced `tool loop: model stopped at max_tokens (iteration 2)`. The screenshot shows VText had only written a status-style v1 saying evidence was being gathered and that a researcher had been requested.
belief-state change: the `max_tokens` toast is probably not the root cause by itself. It appears after a long stall, so the first question is why the prompt did not search, emit findings, and write an interim VText revision before the model hit the output cap or provider timeout. The current probe suite may under-cover this because it measures the hard `nba update` path successfully but does not yet replay the exact stalled baseball prompt or older/per-computer model-policy states.
remaining error field: manual broad sports prompts can still lose the user for minutes; VText may still write a "waiting for evidence" v1 and then fail instead of producing smaller interim revisions; a long serial Playwright probe can lose auth while polling; deployed traces must make it obvious whether the stall is in spawn, search provider call, researcher model turn, findings delivery, VText wakeup, edit_vtext, or provider output cap.
next executable probe: reproduce the manual baseball failure on staging with product-path Playwright and Trace polling, then classify the exact stalled transition. Inspect request/response instrumentation for the VText and researcher turns: model/provider, max_tokens requested or omitted, tool_choice, available tools, tool calls, channel messages, and whether any successful search result or findings message arrived before the terminal error.
```

## Sources And Prior Art

- Parallel Search API docs describe ranked, LLM-optimized excerpts and fewer
  traditional search/scrape/extract hops as a target worth testing in Choir.
- Parallel overview distinguishes Search, Extract, and Task API usage; Choir
  should start with Search only for the fast path, reserve Extract for a
  future explicit paywalled-content pathway, and leave Task API for later deep
  research.
- Pi-style context pruning reinforces the split between durable history/full
  evidence and compact model-visible context.
- DSPy-style prompt optimization suggests measuring prompt/tool-description
  variants against explicit metrics across examples and models rather than
  hand-tuning for one model.
