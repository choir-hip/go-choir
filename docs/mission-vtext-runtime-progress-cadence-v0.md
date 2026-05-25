# MissionGradient: VText Runtime Progress Cadence v0

**Status:** draft
**Date:** 2026-05-25
**Purpose:** make prompt-bar runs produce fast, iterative, human-readable VText updates while Chyron carries granular activity and Trace remains the dense causal ledger.

## One-Line Goal String

```text
/goal Run docs/mission-vtext-runtime-progress-cadence-v0.md as a Codex-operated MissionGradient mission: fix the runtime cadence behind VText progressive observability. Starting from deployed commit f482da6, instrument prompt-bar -> conductor -> VText -> researcher/super runs so evidence records every appagent VText revision, inter-revision gap, worker/researcher/super channel update, Chyron event, model/provider config, tool error, timeout, and retry. Then root-cause and repair the long silence between useful VText versions: researchers and super should send concise substantive early updates, VText should wake and write smaller coherent revisions when useful new evidence arrives, and long-running search/coding/mixed prompts should not wait for all work to finish before v2/v3. Keep VText as the single canonical writer and Chyron as the translucent prompt-bar ticker for granular tool calls and agent messages. Do not require v3+ for simple prompts, spam VText with raw tool logs, let conductor author canonical VText, hide failures in Trace only, accept habitual spawn/tool/provider/max-token errors, or claim success from local-only proof. Land through git/CI/deploy, verify staging identity, and iterate with Playwright over prompts requiring no search, search, coding, and mixed work until deployed evidence shows fast useful v1, smaller iterative updates for long-running work, readable Trace/Chyron evidence, no recurring tool-loop/provider errors, rollback refs, residual risks, and the next realism axis.
```

## Mission Frame

The prompt-bar Chyron is now placed correctly: it streams granular runtime events
inside the prompt bar and fades behind focused input. That makes the system feel
more alive during work, but it does not solve the deeper runtime issue.

The latest deployed proof after `f482da6` still showed long VText gaps:

```text
simple: v1 3.5s, no later revision needed
search: v1 5.5s, v2 61.3s, gap 56s
coding: v1 6.7s, v2 35.2s, gap 29s
mixed:  v1 11.9s, v2 67.6s, gap 55s
```

That is better than a blank app, but not yet the desired interaction model. A
user should see a useful early VText version, then smaller owner-readable
updates when real evidence arrives. They should not wait a minute for the
second meaningful version while researchers or super are already working.

The real artifact is a runtime cadence:

```text
prompt submitted
  -> conductor routes quickly and emits granular Chyron/Trace events
  -> VText writes a useful v1
  -> researcher/super sends first substantive update as soon as useful evidence exists
  -> VText wakes and writes v2 from the first useful update
  -> continuing work emits granular Chyron events and concise VText-worthy updates
  -> VText writes v3/v4 only when useful, not by rote
  -> final revision reconciles what changed, evidence, uncertainty, and next step
```

## Core Invariants

```text
VText is the only canonical document writer.
Chyron is granular live activity, not canonical prose.
Trace is the dense causal ledger.
Researchers/super provide concise substantive updates; they do not author VText.
Simple prompts may finish after one useful VText revision.
Long-running prompts should not have long silent gaps when useful evidence exists.
Errors are product evidence, not background noise.
```

## Value Criterion

Minimize user-perceived silence and false confidence while preserving evidence
quality, canonical writer boundaries, and provider/tool-loop correctness.

The mission is uphill when:

- time to useful v1 stays low;
- long-running prompts produce smaller VText updates before final completeness;
- Chyron shows granular tool calls and agent messages during gaps;
- Trace can explain who worked, which model they used, what failed, and what
  evidence caused each revision;
- repeated spawn/tool/provider/max-token errors disappear or become precise
  surfaced blockers.

## Behavioral Targets

These are initial targets for deployed evidence, not eternal SLOs:

- `simple` prompt such as `hey`: one useful VText revision is acceptable.
- `search` prompt such as `nba update` or current weather/news: useful v1
  should appear quickly, and first evidence-backed update should appear as soon
  as initial facts are available rather than after all research completes.
- `coding` prompt: VText should report route and first super progress promptly,
  then revise when code/evidence/blocker arrives.
- `mixed` prompt: first useful research or super update should produce a VText
  revision before the whole mixed run finishes.
- No long-running prompt should show more than roughly 25-30 seconds of
  evidence-bearing work without either a VText update or a clearly visible
  Chyron/Trace reason why VText is waiting.
- VText updates should be owner-readable summaries, not raw tool logs.

## Implementation Gradient

### 1. Instrument Before Mutating

Extend the deployed Playwright/runtime proof so every prompt records:

- submission id, trajectory id, doc id;
- every appagent VText revision, timestamp, content length, and preview;
- every inter-revision gap;
- worker/researcher/super channel messages addressed to VText;
- VText wake/debounce scheduling facts where available;
- Chyron text snapshots and prompt-focus opacity;
- Trace model/provider/reasoning config for every agent;
- tool-loop errors, provider errors, timeouts, max-token stops, retries, and
  spawn failures.

The proof should distinguish:

```text
no update needed
update available but VText did not wake
VText woke but did not write
worker never sent useful update
provider/tool failed
waiting for final completeness by prompt contract
```

### 2. Root-Cause The Current Gaps

Inspect and instrument the implicated runtime surfaces:

- `internal/runtime/runtime.go` VText mutation and worker-update reconciliation;
- VText wake/debounce scheduling;
- `submit_research_findings`, channel messages, and VText-addressed updates;
- researcher and super prompts/contracts for early updates;
- `initialVTextToolChoice`, forced continuation, and edit-vtext loops;
- provider adapter stop reasons, max tokens, deadlines, and retry behavior;
- Trace summary fields used by the proof.

Classify each observed delay:

- no evidence yet;
- evidence exists but not addressed to VText;
- VText wake not scheduled;
- wake scheduled but debounced too long;
- VText chooses to wait for more;
- tool choice/prompt prevents edit;
- provider/tool-loop failure;
- UI stream/catch-up failure.

### 3. Fix Update Cadence

Repair the smallest layer that actually causes the silence. Likely candidates:

- prompt changes requiring researchers/super to send first useful findings early;
- runtime tools that mark first findings as VText-worthy without dumping raw logs;
- wake scheduling that triggers after meaningful updates, not only terminal
  worker completion;
- VText prompts that explicitly write partial-but-useful versions when evidence
  is incomplete;
- continuation logic that avoids extra loops after a final answer but continues
  when addressed worker updates remain unconsumed;
- provider/tool-loop defaults that avoid recurring max-token or deadline errors.

Do not force artificial revisions. A new version should carry a useful change
in narrative state, evidence, uncertainty, or next action.

### 4. Iterate With Trace-Led Feedback

After each deployed or local-fast patch, rerun the prompt matrix and compare:

- v1 latency;
- v2/v3/v4 timings when present;
- largest inter-revision gap during active work;
- count and quality of worker/researcher/super updates;
- Chyron density and readability;
- recurring error count by kind;
- final document quality.

If the same error class appears twice, stop treating it as incidental and root
cause it before continuing.

### 5. Quality Pass

Once behavior improves:

- simplify any added scheduling or prompt machinery;
- remove dead compatibility branches;
- add focused unit tests for wake/update classification where possible;
- keep heavyweight Playwright proof as a deployed acceptance script, not a
  default unit-test tax;
- update this mission with a checkpoint or completion evidence.

## Evidence Ledger

For each prompt class, record:

- prompt text;
- submission id;
- trajectory id;
- VText document and revision ids;
- revision timeline and gaps;
- Chyron proof screenshot/video or DOM metrics;
- Trace copied logs or JSON summary;
- recurring errors found and fixed;
- staging commit identity;
- Playwright command and result.

## 2026-05-25 Checkpoint: Prompt Guidance Improved But Did Not Finish Cadence

Status: `checkpoint_incomplete`.

Patch `d7a4b0b6468ddbfa9b329393be16100e61acb43e` shipped and staging
`/health` reported both proxy and sandbox at that deployed commit. CI and
FlakeHub publish workflows for the commit completed successfully.

Focused staging probe:

```text
PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com \
  npx playwright test tests/vtext-cadence-diagnostics.tmp.spec.js --reporter=line
```

Evidence artifact:

```text
frontend/test-results/vtext-cadence-diagnostics.-16b88-t-cadence-for-search-prompt-chromium/attachments/nba-update-cadence-664770f7c2af344f9f69cfd981d8025141e2e3f6.json
```

Observed `nba update` timeline:

```text
v0 user prompt:        2026-05-25T15:24:44Z
v1 working brief:      2026-05-25T15:24:49Z  (+5s)
researcher first tool: 2026-05-25T15:24:55Z
first findings packet: 2026-05-25T15:25:26Z
second findings packet:2026-05-25T15:25:36Z
v2 grounded brief:     2026-05-25T15:25:37Z  (+53s from v1)
v3 update:             2026-05-25T15:26:07Z
```

Broad staging matrix:

```text
PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com \
  npx playwright test tests/vtext-chyron-progressive-observability.tmp.spec.js --reporter=line
```

Evidence artifact:

```text
frontend/test-results/vtext-chyron-progressive-o-92d7c-bservable-and-model-labeled-chromium/attachments/vtext-chyron-summary-234fe6edb8e04182f8ec00ada2520e39d8ddc42b.json
```

Observed prompt classes:

```text
simple: first VText 9.8s, one revision
search: first VText 5.6s, second VText 58.6s, gap 53s
coding: first VText 12.0s, second VText 40.6s, gap 29s
mixed:  first VText 12.2s, second VText 45.2s, gap 33s
```

Belief-state update:

- VText wake/debounce is not the primary delay for search. Once researcher
  findings were submitted, VText wrote promptly.
- The moderate prompt guidance eliminated the malformed early
  `submit_research_findings` error seen before the patch.
- The researcher still issued too many searches before the first findings
  packet, despite prompt guidance that said to checkpoint after one focused
  batch. This remains the largest search-cadence gap.
- VText still attempted a redundant `edit_vtext` after one successful grounded
  revision in the focused probe; this is noisy and should be discouraged in
  VText role guidance rather than enforced through a generic tool-loop rule.

Constraint update:

Do not bluntly enforce the researcher checkpoint by changing the shared tool
loop. The shared loop should stay uniform across agents. The next probe should
try stronger role/tool-description guidance first:

- make "first checkpoint" a terminal obligation of the first evidence pass;
- state that after a non-empty first `web_search`, the usual next tool is
  `submit_research_findings`, not another broad search;
- make `submit_research_findings` the researcher communication primitive for
  early findings, not only a final report;
- tell VText to stop after one successful `edit_vtext` unless a tool result
  explicitly requires a next tool or new addressed evidence arrives.

## Forbidden Shortcuts

- Conductor-authored canonical VText.
- Prompt echo counted as useful v1.
- Mandatory v3+ for simple prompts.
- Raw tool log dumps in VText.
- Fake Chyron text or local-only Chyron proof.
- Ignoring repeated `spawn_agent`, provider timeout, auth, or max-token errors.
- Treating Trace-only visibility as sufficient user feedback.
- Calling a checkpoint complete because the UI looks lively while VText remains
  stale.

## Stopping Condition

Complete only when deployed staging evidence shows:

- useful v1 remains fast across simple/search/coding/mixed prompts;
- search and mixed prompts produce at least one evidence-backed update materially
  sooner than the previous 55-56 second gap, or the trace proves no useful
  evidence existed earlier;
- coding/super prompts produce first-route or first-progress VText updates
  before terminal completion when work is long-running;
- Chyron streams real granular activity during gaps and stays non-blocking;
- Trace exposes enough model/tool/channel evidence to explain each revision;
- repeated habitual errors are fixed or precisely blocked with root cause and
  next probe;
- rollback refs, residual risks, and next realism axis are recorded.

If incomplete, update this file with:

```text
status: checkpoint_incomplete | blocked_incomplete
last checkpoint:
current artifact state:
what shipped:
what was proven:
unproven or partial claims:
belief-state changes:
remaining error field:
highest-impact remaining uncertainty:
next executable probe:
suggested resume goal string:
evidence artifact refs:
rollback refs:
```
