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

## Run Checkpoint & Resumption State

```text
status: checkpoint_incomplete
last checkpoint: mission created before reproduction.
current artifact state: deployed code identity is f60a8e0d; no VText repair
  code has been authored in this mission.
what shipped: none.
what was proven: current health only.
unproven or partial claims: live VText failure transition.
belief-state changes: May 26 review is now a hypothesis source, not direct
  implementation plan, because several findings are stale against current code.
remaining error field: reproduce and classify current failure.
highest-impact remaining uncertainty: whether the failure is generic code,
  deployed/live-state drift, owner-computer policy/state, or provider/search
  behavior.
next executable probe: run staging product-path factual and coding prompts and
  record full transition evidence.
suggested resume goal string: use the Goal Prompt above.
evidence artifact refs: pending.
rollback refs: no behavior change yet.
```
