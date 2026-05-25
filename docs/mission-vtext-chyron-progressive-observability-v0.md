# MissionGradient: VText, Chyron, And Progressive Observability v0

**Status:** draft
**Date:** 2026-05-25
**Purpose:** make ordinary prompt-bar runs feel alive, fast, inspectable, and truthful while preserving VText as the single canonical writer.

## One-Line Goal String

```text
/goal Run docs/mission-vtext-chyron-progressive-observability-v0.md as a Codex-operated MissionGradient mission: make Choir's prompt-bar runs progressively observable and faster to trust. Preserve VText as the single canonical document writer, but make it produce useful early and frequent revisions: v1 should be a short owner-readable working response, v2 should land as soon as initial findings or first agent progress exists, and later versions should refine in small coherent increments instead of waiting for all researchers/super work to finish. Put conductor routing notes, tool-call activity, agent-to-agent messages, and interim worker/research progress into a translucent prompt-bar Chyron ticker rather than fake VText drafts, with Chyron history visible from Trace. In Trace, show each agent role's resolved provider/model/reasoning/tool-profile configuration. Fix Markdown table rendering in VText. Use Playwright against staging across prompts requiring search, coding, both, and neither, including “hey” and “nba update”, to prove prompt-to-v1 latency, prompt-to-v2 latency, full VText revision timing where present, continuing revisions during long-running work, Chyron streaming, Trace model config visibility, and readable tables. Land through git/CI/deploy, verify staging identity, and finish with screenshots/video/DOM timing evidence, Trace/VText/run-acceptance refs, rollback refs, residual risks, and the next realism axis. Do not let conductor write canonical VText, hide progress only in Trace, spam VText with tiny tool events, fake Chyron text, use local-only proof, or claim completion without deployed product-path evidence.
```

## Mission Frame

The current failure is not that Choir cannot eventually research and write.
The failure is the human experience during the run:

- early VText revisions are often just the prompt copied back;
- useful content appears only after a long wait;
- researchers and super can do real work without sending useful early updates;
- Trace contains the causal truth but is too dense to be the primary live
  supervision surface;
- Chyron exists conceptually as the right low-level progress surface but is not
  yet the default path for routing notes, tool calls, and agent messages;
- VText tables render as broken inline Markdown when the model emits compact
  table syntax.

The real artifact is a progressive observability loop:

```text
prompt bar submit
  -> conductor routes and emits prompt-bar Chyron status, not canonical prose
  -> VText writes a concise v1 answer quickly
  -> researchers/super send substantive early updates to VText and granular events to Chyron
  -> VText writes v2 as soon as first findings exist
  -> VText continues small coherent revisions while work proceeds
  -> Trace remains the deep evidence ledger with model config and Chyron history
```

## Core Invariants

```text
VText is the only canonical document writer.
Conductor may route and announce progress, but does not author canonical VText.
Chyron is granular live observability, not canonical text.
Trace is the causal ledger and must expose provider/model/reasoning configuration.
Researchers/super send meaningful progress updates; they do not wait for final completeness.
VText revisions are owner-readable narrative/document state, not raw tool logs.
```

## Value Criterion

Minimize the time between prompt submission and trustworthy human feedback while
preserving canonical-authority boundaries, source/evidence quality, and readable
final documents.

The winning behavior is not just "more versions." It is:

- a fast useful first VText revision;
- a visible Chyron stream during the gap between revisions;
- initial findings incorporated quickly;
- later revisions improve the document without bloating it;
- Trace can explain which model each role used and why.

## Homotopy Axes

Low-to-high realism should preserve the same event topology:

1. One simple no-search prompt such as `hey`.
2. One fast factual/research prompt such as `nba update`.
3. One prompt requiring search plus synthesis.
4. One coding/system prompt that routes to super.
5. One mixed prompt requiring both research and code/system action.

At every level:

- VText owns canonical revisions.
- Chyron owns granular progress.
- Trace owns dense evidence and model configuration.

## Required Behavioral Targets

These are not hard millisecond SLOs yet; they are evidence targets to measure
and improve.

- `v1`: should not be a prompt echo. It should be a short useful response,
  orientation, or "working answer" from VText.
- `v2`: should appear after initial findings/progress, not only after all
  researchers or super tasks complete.
- Later versions: are not required for every prompt, but long-running work should
  prefer smaller coherent updates over one giant late dump.
- Researchers and super: should send concise substantive progress messages to
  VText at meaningful milestones and granular tool/agent messages to Chyron.
- Chyron: should stream conductor routing, tool calls, tool results summaries,
  and agent messages as a translucent left-to-right ticker inside the prompt bar,
  becoming very faint while the user is typing so input remains dominant.
- Trace: should show role, provider, model, reasoning effort, tool profile, and
  model-policy source for each agent where available.
- VText Markdown: tables must render as tables or be normalized into readable
  non-table prose; broken inline table pipes are unacceptable.

## Implementation Gradient

### 1. Observe Current Failure On Staging

Use product-path Playwright against `https://draft.choir-ip.com` and record:

- prompt-to-v1 latency;
- prompt-to-v2 latency;
- complete appagent VText revision timeline, with inter-revision gaps and
  v3/v4/v5 timing when long-running work produces them;
- number and size of revisions;
- which agents spawned;
- when researchers/super send messages;
- whether Chyron shows meaningful progress;
- whether Trace lists model configuration;
- whether VText tables render correctly.

Prompts must include at least:

- `hey`
- `nba update`
- a current-events search prompt;
- a coding/system prompt;
- a mixed research-plus-system prompt.

### 2. Root-Cause Revision Timing

Inspect the runtime prompt contracts, VText agent tools, researcher/super
message tools, run loop scheduling, and VText mutation controller.

Classify each delay:

- model/provider latency;
- agent waiting pattern;
- prompt instruction problem;
- tool availability problem;
- VText mutation lock or revision scheduling problem;
- UI live-update problem.

### 3. Make Progress Messaging First-Class

Adjust prompts/tools/contracts so:

- VText writes an early useful revision without waiting for complete research;
- researchers submit initial findings as soon as they have useful verified
  facts, then continue;
- super sends substantive status updates when it chooses a route, receives a
  blocker, or changes strategy;
- low-level tool calls and agent-to-agent messages are emitted into Chyron and
  Trace, not stuffed into VText.

Do not solve this by making conductor write VText.

### 4. Build Chyron As Product Observability

The Chyron should be a prompt-bar ticker layer that:

- streams granular progress text left-to-right without blocking Shelf buttons
  or prompt input;
- remains visible but becomes very faint when prompt input is focused;
- has an inspectable history visible from Trace;
- receives real events from runtime/Trace, not fake seed text;
- can be proven with Playwright video.

### 5. Improve Trace Model Configuration Disclosure

Trace agent surfaces should show the resolved model config for every agent role:

```text
role
provider
model
reasoning effort
tool profile / tool count
policy source
```

Prefer concise inline badges plus detail on selection. Do not make users dig
through raw JSON to answer "which model did this agent use?"

### 6. Fix VText Markdown Tables

Fix rendering/normalization so Markdown tables in VText display readably. If a
model emits malformed compact tables, either normalize them before storing or
render them robustly enough that they do not appear as one broken paragraph.

## Evidence Ledger

Record each nontrivial claim with:

- prompt text;
- trajectory id;
- VText document/revision ids and timestamps;
- Trace screenshot or copied log;
- Chyron screenshot/video;
- DOM timing metrics;
- staging commit identity;
- tests run;
- residual caveats.

## Forbidden Shortcuts

- conductor-authored canonical VText;
- prompt echo counted as useful v1;
- hiding progress only in Trace;
- fake or decorative Chyron text;
- raw tool-event spam in VText;
- local-only proof for product behavior;
- internal/test route success seeding;
- screenshots without deployed commit identity;
- claiming completion if only one prompt class works.

## Stopping Condition

Complete only when staging evidence shows:

- across the prompt matrix, VText creates a useful early revision and long-running
  prompts show continued smaller revisions from ongoing work when more evidence
  is still arriving;
- `nba update` or equivalent search prompt writes initial useful content sooner
  than the previous all-at-end behavior;
- Chyron streams real conductor/tool/agent activity and remains non-blocking;
- Trace exposes agent model configuration;
- VText tables render readably;
- rollback refs, residual risks, and next realism axis are documented.

If incomplete, update this mission with:

```text
status: checkpoint_incomplete | blocked_incomplete
last checkpoint:
what shipped:
what was proven:
unproven or partial claims:
highest-impact remaining uncertainty:
next executable probe:
rollback refs:
```
