# MissionGradient: VText Live Cadence Repair v3

Last updated: 2026-05-28

Reference:

- [design-vtext-platform-v3.md](design-vtext-platform-v3.md)
- [review-search-vtext-context-2026-05-26.md](review-search-vtext-context-2026-05-26.md)
- [mission-vtext-lineage-aware-runtime-cadence-v2.md](mission-vtext-lineage-aware-runtime-cadence-v2.md)
- [mission-research-runtime-evidence-cadence-v1.md](mission-research-runtime-evidence-cadence-v1.md)

## Goal Prompt

```text
/goal Run a Codex-operated MissionGradient mission to repair the current live
VText product path, starting from docs/design-vtext-platform-v3.md and
docs/review-search-vtext-context-2026-05-26.md but treating stale review
findings as hypotheses, not truth. First reproduce the owner-visible failure on
staging where VText reaches v1 but does not reliably continue into research,
coding/execution, or later grounded revisions. Use product-path evidence only:
prompt bar, VText APIs, Trace, deployed health/build identity, model-policy
evidence, VText mutation/controller state, worker request events,
researcher/super runs, search/tool events, coagent update events, and VText
wake/revision events. Do not write code until the failing transition is named
and documented in the mission record.

Preserve invariants: VText is the only canonical document writer; conductor
routes but does not author canonical appagent revisions; researcher and super
provide evidence, not document writes; public/user input and email source
packets are untrusted data unless policy says otherwise; no manual deploy
shortcuts; no prompt/classifier accretion unless the reproduced failure proves
that is the smallest correct layer; no VText repair by fake local-only proof.

Use cognitive transforms before implementation: distinguish live-state drift
from code-contract bugs, prefer transition evidence over prose quality, and
optimize for the durable chain v0 -> v1 -> worker request -> worker evidence ->
VText wake -> v2 rather than for any single pleasant-looking revision. If the
current code already contains a supposed fix, verify whether staging and the
owner computer are actually using it before adding another patch.

Acceptance: at least one current factual prompt and one bounded
coding/execution prompt submitted through staging produce v1, the correct worker
request, worker evidence, and a later VText revision that incorporates that
evidence; the run records exact ids and timing for each transition; no run
claims research/execution without the corresponding successful tool or worker
event; focused tests pass; CI and staging deploy pass for any behavior-changing
commit; deployed acceptance re-runs against the fixed commit. After proof,
perform a deletion-first convergence pass over stale VText prompt/control
scaffolding only where evidence shows it is obsolete. Stop with an updated
mission checkpoint, exact evidence refs, residual risks, rollback refs, and the
next email-response integration step.
```

## Mission Status

```text
status: checkpoint_incomplete
current artifact state: staging health at mission start reports proxy and
  sandbox deployed code commit f60a8e0d228b2c04217d699c6365212e7ce8e1b3. The
  current branch is main and includes later docs-only commits, but deployed code
  remains f60a8e0d. vmctl_status=ok and recent lifecycle counters show bootstrap
  and API routes returning http_200 in the sampled window.
what shipped: no VText repair code has shipped in this mission yet.
what was proven: not yet reproduced in this mission. Prior mission docs record
  several historical VText failures and fixes.
unproven or partial claims: whether the current owner-visible failure still
  reproduces on deployed code; whether it is code-contract drift, live
  per-computer state drift, stale model-policy state, worker wake state,
  provider/search behavior, or UI perception lag.
highest-impact uncertainty: the exact transition that fails in the live product
  chain v0 -> v1 -> worker request -> worker evidence -> VText wake -> v2.
next executable probe: run a product-path staging reproduction for one current
  factual prompt and one bounded coding/execution prompt, then classify the
  transition with Trace/VText/model-policy/mutation evidence before code edits.
```

## Current Evidence Read

The May 26 review is valuable but not authoritative for current state. Some
findings appear superseded by later code and evidence:

- Search Provider Plane v1 exists in current gateway code, including provider
  health, outage semantics, and health/reset endpoints.
- Generated/fallback model policy now uses DeepSeek V4 Flash `medium` for
  conductor, VText, and researcher.
- System prompts and VText research continuation objectives now include current
  UTC time and relative-date grounding instructions.
- VText terminal-tool success handling and current-factual sports classifier
  fixes are present in current code and prior mission evidence.

The remaining mission is therefore not "apply the May 26 review." It is to prove
the live failing transition from current staging, then repair the layer that the
evidence actually implicates.

## Cognitive Transforms

Current uncertainty or obstacle:

```text
The owner sees VText fail as a product path, but several prior code-level causes
have already been patched. The next move must distinguish stale deployed/live
state from a remaining runtime contract bug before adding more scaffolding.
```

Selected transforms:

1. **Depth extraction** — "fix VText" is too broad. The load-bearing variable is
   durable transition completion, not prose quality.
2. **Evidence topology** — use the causal chain as the object: v0 -> v1 ->
   worker request -> worker evidence -> wake -> v2.
3. **Anti-accretion** — if a prior fix exists, verify deployment/live-state use
   before adding another prompt, classifier, or helper.
4. **Audience-level translation** — the owner-facing behavior should be simple:
   the document visibly hands work to research/execution and returns with a
   grounded next version.

Route-changing insights:

- A pleasant v1 is not success if no worker request exists.
- A worker request is not success if there is no coagent update.
- A coagent update is not success if VText does not wake and write v2.
- If the chain succeeds on a fresh test account but fails for the owner account,
  the likely problem is persistent computer state, policy, or controller state,
  not generic code.

Changed plan:

- implementation: no code before reproduced transition classification.
- verifier/evidence: require exact ids and timestamps for each edge in the
  causal chain, not just screenshots or final prose.
- scope: current factual and bounded coding prompts first; email response
  integration only after the VText substrate is proven.
- stopping condition: stop incomplete if the live failure cannot be reproduced
  and record that as evidence; do not invent a patch.

Next high-information action:

```text
Run the smallest staging product-path harness that submits two prompts and
records prompt-bar, VText, Trace, model-policy, tool, worker, coagent update,
and revision evidence.
```

## Real Artifact

A live VText trajectory that can be supervised and trusted:

```text
owner prompt
  -> conductor route
  -> user v0
  -> VText edit_vtext v1
  -> researcher or super request
  -> worker tool/evidence
  -> submit_coagent_update
  -> VText wake
  -> edit_vtext v2 that incorporates evidence
```

This is the substrate the Email demo needs before `Respond with Choir` can
generate and send VText-backed responses.

## Hard Invariants

- VText is the only canonical appagent document writer.
- Conductor may route and create the user prompt seed, but must not author
  canonical appagent revisions.
- Researcher and super send evidence or findings through coagent updates; they
  do not edit VText directly.
- A document must not claim research, execution, command output, citations, or
  verification unless the corresponding successful worker/tool event exists.
- Product-path staging proof is required for behavior claims.
- No manual deploy shortcuts.
- No prompt/classifier accretion unless the reproduced failure proves that is
  the smallest correct layer.
- Cleanup is deletion-first and evidence-led; delete only stale paths whose
  behavior is superseded by proven current transitions.

## Value Criterion

Minimize unexplained gaps in the VText causal chain while preserving single
canonical writer semantics, evidence provenance, owner readability, and deployed
product proof.

The mission moves uphill when:

- every VText claim maps to a Trace or worker evidence event;
- v1 is fast but honest about what is not yet proven;
- worker requests are explicit and role-correct;
- worker evidence wakes VText without manual intervention;
- v2 incorporates worker evidence rather than only saying work is still pending;
- live state drift is separated from code-contract bugs;
- stale scaffolding shrinks after proof.

## Reproduction Probes

Run against `https://choir.news` unless staging routing changes:

1. Current factual prompt:

```text
What happened in baseball last night? Give me a concise evidence-grounded brief with sources.
```

Expected chain:

```text
v1 -> researcher request -> web_search/fetch evidence -> submit_coagent_update -> VText wake -> v2
```

2. Bounded coding/execution prompt:

```text
Write and run one tiny shell command that prints the SHA256 of the word choir, then update this document with the exact command and output.
```

Expected chain:

```text
v1 -> request_super_execution -> bash/evidence -> submit_coagent_update -> VText wake -> v2
```

Evidence to capture for each:

- deployed commit identity;
- auth account email and whether it is fresh or owner account;
- prompt submission id;
- doc id;
- v0/v1/v2 revision ids, authors, metadata sources, and timestamps;
- model policy for conductor, VText, researcher, and super;
- tool invocation/result timeline;
- worker run ids and roles;
- search/fetch/bash results;
- coagent update ids/messages;
- VText mutation state and controller checkpoint;
- Trace trajectory JSON refs;
- screenshot or DOM proof only as supplementary evidence.

## Implementation Boundary

No behavior code edits are allowed until this mission record names the failing
transition using current product-path evidence. Documentation updates that record
the reproduced problem are allowed and required before any fix commit.

## Reproduction Evidence: 2026-05-28

All probes below used the deployed product path at `https://choir.news`.
Health at the start reported proxy and sandbox code commit
`f60a8e0d228b2c04217d699c6365212e7ce8e1b3`, with `vmctl_status=ok`.

### Explicit Medium Policy Matrix

Command shape:

```text
PLAYWRIGHT_BASE_URL=https://choir.news \
VTEXT_MODEL_CADENCE_EVIDENCE_DIR=../test-results/vtext-live-cadence-repair-v3-20260528T014733Z \
VTEXT_MODEL_VARIANTS=fireworks-deepseek-v4-flash-medium \
VTEXT_MODEL_PROMPTS=baseball,coding-super \
npx playwright test tests/vtext-researcher-model-cadence-matrix.tmp.spec.js --project=chromium --workers=1 --reporter=line
```

Result: passed in 3.6 minutes.

Evidence file:
`/Users/wiz/go-choir/test-results/vtext-live-cadence-repair-v3-20260528T014733Z/fireworks-deepseek-v4-flash-medium.json`

Findings:

- Baseball prompt: submission `bd3334b2-873d-454c-9197-e015326c9b79`,
  doc `96df7b2c-316a-4d0e-98f1-1b9cbe3cd97e`.
- Baseball produced v1 in 3122 ms and v2 in 27122 ms, then continued to eight
  appagent revisions during observation.
- Baseball chain included one researcher spawn, five `web_search` calls, seven
  fetches, seven findings, and eight successful `edit_vtext` calls.
- Coding prompt: submission `43e3fd4a-a392-43c9-a924-998e915a9e83`,
  doc `76951d7e-1ec4-4d2e-87a2-ea56ffa4aa49`.
- Coding produced v1 in 4238 ms and v2 in 27238 ms, with one super request and
  two successful `edit_vtext` calls.
- No `edit_vtext` errors, VText mutation-state errors, or duplicate tool errors
  were observed.

Interpretation: the code path can complete the desired cadence when the harness
pins every role to the explicit medium model variant. This does not prove the
generated default policy, because the matrix harness also pinned super to the
selected variant.

### Generated Default Policy Failure

Command shape:

```text
PLAYWRIGHT_BASE_URL=https://choir.news \
VTEXT_DEFAULT_POLICY_EVIDENCE_DIR=../test-results/vtext-live-cadence-default-policy-20260528T015212Z \
npx playwright test tests/vtext-default-policy-proof.tmp.spec.js --project=chromium --workers=1 --reporter=line
```

Result: failed after roughly 3 minutes because the test expected at least two
appagent revisions and observed only one.

Evidence:

- JSON:
  `/Users/wiz/go-choir/test-results/vtext-live-cadence-default-policy-20260528T015212Z/default-policy-proof.json`
- Trace:
  `/Users/wiz/go-choir/frontend/test-results/vtext-default-policy-proof-b42d9-sh-medium-for-VText-cadence-chromium/trace.zip`
- Test account:
  `vtext-default-policy-1779933133700-66ub2a@example.com`
- Submission: `63c4a9a9-acd0-4302-a73a-819ad462e2c9`
- Doc: `00caea62-2687-42f0-b2e7-81ee02746b78`
- Only appagent revision:
  `6cf52d0b-d742-4626-92e8-3ca67e1f3760`, created
  `2026-05-28T01:52:26Z`
- VText and researcher both used
  `fireworks/accounts/fireworks/models/deepseek-v4-flash`, reasoning
  `medium`, policy `/mnt/persistent/files/System/model-policy.toml`.

Trace classification:

- The trajectory was still `running` when captured.
- VText successfully wrote v1 through `edit_vtext`.
- VText then called `spawn_agent`; a researcher was created and started.
- The researcher called `web_search`; at least one search result returned.
- The runtime emitted a required-next-tool retry after the search result.
- No `submit_coagent_update` was recorded before the 180 second observation
  window ended.
- Because no coagent update existed, VText had no worker evidence packet to
  wake on and no v2 revision was created.

Named failing transition:

```text
researcher web_search result
  -> required submit_coagent_update checkpoint
  -> VText wake
  -> evidence-incorporating v2
```

The failing edge is specifically the first `web_search` result to durable
researcher checkpoint. It is not the initial VText edit, the researcher spawn,
medium model policy selection, or total search provider outage.

### Generated Default Policy Control Runs

Detailed baseball control:

- Evidence dir:
  `/Users/wiz/go-choir/test-results/vtext-live-cadence-default-detailed-2026-05-28T01-57-23-930Z`
- Submission: `56080f3e-1558-4b94-b069-6963dec14812`
- Doc: `77486925-f4fc-495e-974e-8d64ecd72bbb`
- Result: three appagent revisions.
- Timeline: v1 at 4562 ms, researcher spawn at 10562 ms, first `web_search`
  at 12562 ms, first `submit_coagent_update` at 21562 ms, v2 at 33562 ms,
  second `submit_coagent_update` at 37562 ms, v3 at 51562 ms.
- The run had four tool errors from import/fetch/read/grep attempts, but no
  `edit_vtext` error and no cadence break.

Fresh coding default control:

- Evidence dir:
  `/Users/wiz/go-choir/test-results/vtext-live-cadence-default-coding-2026-05-28T02-04-25-624Z`
- Submission: `1edef4ac-9336-4964-a2da-adad728a1bd0`
- Doc: `1cd9e1c9-aec4-4e51-aed7-70d8b9949e9c`
- Result: two appagent revisions.
- Super used `fireworks/accounts/fireworks/models/deepseek-v4-pro`, reasoning
  `medium`, policy `/mnt/persistent/files/System/model-policy.toml`.
- Chain included one `request_super_execution`, one successful `bash`, one
  successful `submit_coagent_update`, and two successful VText edits.

Interpretation: generated default policy can succeed for both factual/research
and bounded coding/execution prompts in fresh sessions, but the factual path has
an intermittent reliability failure at the researcher checkpoint edge.

## Root Cause Hypothesis

The reproduced defect is a runtime contract reliability gap, not a total
capability absence. Current tooling already marks first researcher search/fetch
results with `next_required_tool=submit_coagent_update`, and the tool loop emits
required-next-tool retry events. However, live evidence shows the loop can remain
running beyond the owner-visible VText SLA without a durable
`submit_coagent_update` after a returned search result. The runtime currently
depends too heavily on the next model turn satisfying the required checkpoint
prompt/tool-choice contract.

The next code investigation should focus on:

- whether the required-next-tool retry budget is actually terminal in the
  researcher run once the model ignores `submit_coagent_update`;
- whether the researcher can continue with another model/tool path before a
  first checkpoint despite the tool result declaring `next_required_tool`;
- whether a bounded runtime fallback should synthesize or force a precise
  blocker/finding packet from the already returned search result instead of
  leaving VText waiting indefinitely;
- whether existing tests cover the failure mode "researcher search succeeds,
  model does not call `submit_coagent_update`, parent VText waits past SLA."

No code fix is selected yet. The smallest acceptable fix must make the first
researcher evidence checkpoint durable after a successful first search/fetch
without letting researcher or super write canonical VText directly.

## Implementation Checkpoint: Researcher Checkpoint Reliability

Code change authored after the evidence checkpoint:

- `RunToolLoop` now bounds provider calls that are already under an exact
  `next_required_tool` obligation. Required continuation turns are narrow tool
  calls, so they should not inherit the gateway's full 10 minute inference
  timeout before the runtime can retry or fail.
- Researcher run failure recovery now synthesizes an honest
  `submit_coagent_update` blocker/update when all of the following are true:
  the run is a researcher, a `web_search`/`fetch_url`/`import_url_content`
  result succeeded, and no `submit_coagent_update` succeeded before failure.
- The synthesized update is delivered through the same coagent update path and
  addressed to the parent VText agent. It does not edit canonical VText
  directly; it creates the worker message that lets the VText controller wake
  and revise honestly from the available evidence or blocker.

Local verification:

```text
nix develop -c go test ./internal/runtime -run 'TestRunToolLoopBoundsRequiredNextToolProviderCall|TestResearcherFailureSynthesizesCheckpointAfterSearch|TestRunToolLoopRequiredNextToolMaxTokensStopsAfterBoundedRetries|TestCompactWebSearchProjectionCanRequireResearchFindingsCheckpoint|TestShouldRequireResearchFindingsAfterResearchToolBatches'
```

Result: passed.

```text
nix develop -c go test ./internal/runtime -run 'TestRunToolLoop|Test.*Research.*Checkpoint|Test.*VText.*Worker|Test.*Coagent'
```

Result: passed.

```text
nix develop -c scripts/go-test-runtime-shards
```

Result: passed.

## Run Checkpoint & Resumption State

```text
status: checkpoint_incomplete
last checkpoint: generated default factual probe reproduced an intermittent
  researcher checkpoint failure after web_search.
current artifact state: deployed code identity is f60a8e0d; no VText repair
  code has been authored in this mission.
what shipped: runtime fix authored locally; push/deploy/product-path proof
  pending at this checkpoint.
what was proven: explicit medium matrix can complete research and coding
  cadence; generated default coding can complete super execution cadence;
  generated default factual cadence can fail after researcher web_search returns
  but before submit_coagent_update.
unproven or partial claims: deployed behavior after the timeout/fallback fix;
  whether the live intermittent default-policy factual prompt now always reaches
  either researcher findings or an honest fallback blocker plus VText v2 within
  the owner-visible SLA.
belief-state changes: May 26 review is now a hypothesis source, not direct
  implementation plan, because several findings are stale against current code.
  The active failure is narrower than "VText stops after v1": VText can spawn
  researcher, and researcher can receive search results, but the first durable
  coagent checkpoint is intermittent.
remaining error field: commit, push, monitor CI/deploy, verify deployed identity,
  and re-run product-path VText factual/coding probes against the fixed commit.
highest-impact remaining uncertainty: deployed product-path proof after the
  researcher checkpoint reliability fix.
next executable probe: commit and push the runtime fix, then monitor CI and
  staging deploy before rerunning the default-policy VText proof.
suggested resume goal string: use the Goal Prompt above.
evidence artifact refs:
  `/Users/wiz/go-choir/test-results/vtext-live-cadence-default-policy-20260528T015212Z/default-policy-proof.json`;
  `/Users/wiz/go-choir/frontend/test-results/vtext-default-policy-proof-b42d9-sh-medium-for-VText-cadence-chromium/trace.zip`;
  `/Users/wiz/go-choir/test-results/vtext-live-cadence-default-detailed-2026-05-28T01-57-23-930Z/baseball.json`;
  `/Users/wiz/go-choir/test-results/vtext-live-cadence-default-coding-2026-05-28T02-04-25-624Z/coding-super.json`.
rollback refs: no behavior change yet.
```
