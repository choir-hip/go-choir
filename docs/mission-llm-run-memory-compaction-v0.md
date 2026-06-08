# LLM Run-Memory Compaction Mission v0

## Mission Identity

Replace Choir's current cave-man deterministic run-memory checkpointing with a
simple, reliable LLM compaction layer that is good enough to unblock the
DeepSeek/Xiaomi provider/config upgrade and return attention to Global Wire.
This mission owns the remaining provider-conformance blocker: when it completes,
it must also update `docs/mission-provider-config-conformance-v0.md` with the
compaction evidence and final provider readiness conclusion.

This is not a maximal memory-system mission. The artifact is a runtime-owned
LLM compaction state transition:

```text
durable raw run memory + rendered prompt pressure
  -> LLM-generated typed checkpoint
  -> recent raw tail + exact retrieval handles
  -> successful post-compaction agent continuation on Node B
```

The stopping condition is not "best possible long-horizon memory." The stopping
condition is that automatic LLM compaction works on staging/Node B for real
DeepSeek/Xiaomi agent loops, with a default threshold of:

```text
context_window_tokens * 0.7
```

For the current target DeepSeek/Xiaomi models, treat `context_window_tokens` as
`1_000_000`, so the default threshold is `700_000` estimated input-context
tokens. The 160k threshold may be used only as a transitional local/staging
diagnostic while bringing the LLM compactor up; it is not the desired final
default for this mission.

## Why This Mission Exists

The provider/config mission exposed that Choir's current run-memory compaction
is architecturally pointed in the right direction but implementation-level
primitive:

- compaction is runtime-owned and automatic;
- raw provider-facing messages are durable;
- compaction entries preserve `entry_id` handles;
- `get_run_memory_entry` can recover exact raw entries after compaction;
- recent raw tail retention exists;
- tool-result cut points are partially protected.

But the actual checkpoint is still a deterministic text block assembled from
truncated message descriptions. That is not sufficient for long-running
providers, Global Wire processors/reconcilers, or self-development work.

We need real LLM compaction: a structured continuation artifact that preserves
objective, constraints, decisions, obligations, failures, evidence handles,
active resources, and next actions without pretending a string truncation is
agent memory.

This is therefore both a compaction mission and the closing dependency for the
provider conformance mission. If LLM compaction is proven, finish the provider
mission's remaining readiness report in the same run. If LLM compaction is
blocked, leave provider conformance explicitly checkpoint-incomplete with this
blocker named.

## Cognitive Transforms Applied

Current uncertainty or obstacle:

Choir has durable run memory, but the current compaction mechanism is a
deterministic truncation summary. The risk is that the next implementation
either overbuilds a memory system or underbuilds by polishing the deterministic
path. The mission must route directly to a real, simple LLM compaction state
transition and use Node B proof to decide provider readiness.

### Mechanism Upgrade

The old object was "summarize old chat enough to fit." The real object is a
state transition in the harness. The compactor should transform raw transcript
and event evidence into an explicit checkpoint that future prompts can use.

Operational consequence: implement compaction as runtime code plus an LLM
checkpoint call, not as an agent-invoked tool or another role-specific branch.

### Anti-Optimization

The goal is not maximal context-window exploitation. Bigger context proofs are
expensive and can hide brittle behavior. The first durable win is a reliable
compactor with conservative thresholding and exact retrieval handles.

Operational consequence: use `context_window * 0.7`, prove it on Node B, and
leave fancier context engineering for later.

### State-First, Transcript-Second

The transcript remains the audit ledger. The checkpoint is active state. The
prompt builder renders the current provider view from stable instructions,
active run metadata, checkpoint state, recent raw turns, and retrieval handles.

Operational consequence: summaries must be typed and auditable; raw transcript
must remain retrievable.

### Failure-Mode Inversion

The likely failure is not that compaction never runs. The likely failure is that
it runs and silently drops the thing that matters: user constraints, current
objective, tool obligations, source handles, provider identity, or failed
attempts.

Operational consequence: verifier prompts must ask questions that require old
content, old constraints, and exact retrieval after compaction.

### State Machine

Compaction is not a paragraph generator. It has states and impossible states:

```text
not_needed -> requested -> compacting -> compacted -> continued
                         -> failed -> emergency_fallback_or_blocked
```

Impossible or bad states include:

- compacted without a durable raw transcript;
- compacted without exact retrieval handles;
- compacted while orphaning a tool result;
- provider call retried with neither LLM checkpoint nor labeled fallback;
- provider conformance marked complete while compaction remains only a local or
  deterministic proof.

Operational consequence: implementation and tests should assert state
transitions and event evidence, not just that a summary string exists.

### Model Versus System

The intelligence is not only in the checkpoint model. The durable behavior comes
from model + harness + prompt builder + run-memory store + exact retrieval tool
+ staging proof.

Operational consequence: do not search for the perfect compaction prompt before
shipping the harness primitive. The 80/20 is a typed prompt, parser, durable
details, raw tail, retrieval handles, thresholding, and Node B proof.

### Value Of Information

The highest-information observation is whether a real staging agent can compact,
continue, and retrieve exact pre-compaction content under DeepSeek/Xiaomi model
policy.

Operational consequence: avoid speculative tuning. Implement the smallest LLM
compactor that can generate typed checkpoints, then spend proof budget on the
Node B product path.

## Hard Invariants

- Compaction is runtime-owned and automatic. Agents do not call a compaction
  tool.
- `get_run_memory_entry` remains the exact retrieval escape hatch after a
  checkpoint; retrieval is tool-visible, compaction is not.
- Raw run memory remains durable, owner-scoped, and append-only enough for
  audit/recovery.
- Provider message invariants must not be broken. Never orphan tool results,
  tool calls, reasoning/tool-use continuations, or media handles.
- Active identity, system/developer instructions, project rules, tool policy,
  model policy, and owner/computer state are not recovered from an LLM summary;
  they remain prompt-builder/runtime state.
- No role-specific harness branches for DeepSeek, Xiaomi, VText, researcher,
  processor, reconciler, or super. Compaction is a shared harness primitive.
- No hidden fallback to deterministic-only compaction as a success claim.
  Deterministic truncation may exist only as an emergency diagnostic/fallback and
  must be reported as such.
- No arbitrary normal-agent `max_tokens` caps. Keep provider budgets governed by
  model policy and prompt for compact output where needed.
- LLM compaction output must not leak hidden reasoning into VTexts, source text,
  trace summaries, or user-visible prose.
- Staging/Node B is the acceptance environment for provider/config readiness.
- Problem documentation precedes behavior-changing fixes.

## Value Criterion

Minimize post-compaction agent divergence while preserving shared harness
uniformity, source-of-truth raw memory, provider message validity, exact
retrieval handles, owner scoping, and Node B product-path proof.

The system gets better when:

- the checkpoint is structured enough for a different future model call to
  continue the same work;
- user constraints and current objective survive compaction;
- old source/evidence/tool handles remain recoverable;
- recent turns remain raw;
- tool-call adjacency remains valid;
- compaction emits auditable before/after evidence;
- provider/config readiness can be concluded without spending more time on
  memory uncertainty.

## Desired 80/20 Architecture

### Runtime Trigger

Use automatic thresholding:

```text
effective_threshold_tokens = context_window_tokens * 0.7
```

For DeepSeek/Xiaomi target models:

```text
1_000_000 * 0.7 = 700_000
```

The runtime may temporarily retain the existing 160k threshold as an explicit
diagnostic override during rollout. The final default for this mission should be
model-aware 70% thresholding.

### Prompt Pressure Estimate

Good enough now:

- include rebuilt run-memory messages;
- include system prompt length;
- include tool schema/catalog prompt length;
- include active skill/prompt additions if already rendered into the system
  prompt;
- include a simple fixed safety reserve.

Do not block this mission on exact provider tokenizer accounting.

### LLM Checkpoint Schema

The LLM compactor should return a compact, typed checkpoint with at least:

- current objective;
- current active task;
- user hard constraints;
- completed work;
- key decisions and rationale;
- open obligations;
- failed attempts and do-not-repeat notes;
- source/evidence/artifact handles;
- tool-result and raw-entry retrieval handles;
- files/docs/resources touched when known;
- active blockers and uncertainties;
- next one to three actions.

Store both a machine-readable details object and a readable checkpoint text.
The prompt-visible checkpoint can be concise, but the durable compaction entry
should preserve the structured fields.

### Recent Raw Tail

Keep recent turns raw. Start with the existing `RunMemoryKeepRecentTokens`
behavior unless proof shows it is failing. The mission is not to tune tail size;
it is to replace the lossy deterministic summary with a real checkpoint.

### Exact Retrieval Handles

Continue recording compacted raw `entry_id`s and
`raw_tool_result_entry_ids`. The compaction prompt should explicitly tell the
future agent when and why to call `get_run_memory_entry`.

### Compaction Reliability

Simple/reliable wins to implement before fancy memory:

- one active compaction per run/session;
- compaction start/completed/failed events with before/after estimates;
- precise error if LLM compaction fails;
- deterministic emergency fallback clearly marked as non-readiness evidence;
- focused tests that prove tool-call/result validity across compaction;
- product-path proof on Node B.

## Homotopy Axes

Increase realism without changing the object:

1. **Checkpoint realism:** deterministic summary baseline -> LLM typed
   checkpoint -> LLM checkpoint with retrieval instructions -> Node B proof.
2. **Trigger realism:** explicit 160k diagnostic override -> model-aware
   `context_window * 0.7` default.
3. **Prompt realism:** raw messages only -> messages plus system/tool prompt
   estimate -> product-path rendered prompt pressure.
4. **Tool pressure:** no tools after compaction -> one retrieval tool call ->
   normal role toolset after compaction.
5. **Role realism:** generic run -> VText/researcher run ->
   processor/reconciler-ready run.
6. **Provider realism:** local fake/provider fixture -> local live DeepSeek or
   Xiaomi -> Node B/staging product path.

## Expected Implementation Routes

Consider these routes, and choose the simplest that proves the artifact:

- add a runtime LLM compactor call that uses the existing provider substrate;
- use the selected run model/provider unless a platform compactor model policy
  exists or is needed;
- emit structured compaction entries into existing `RunMemoryEntry.Details`
  rather than creating a parallel memory database;
- keep `Summary` as a readable prompt block generated from the structured
  checkpoint;
- add model-catalog context-window facts for DeepSeek/Xiaomi if missing;
- compute `0.7 * context_window` as the default threshold;
- preserve the existing exact retrieval tool;
- add a compaction lock/idempotency guard in store/runtime;
- keep deterministic compaction only as labeled emergency fallback.

Avoid:

- building vector memory or graph memory now;
- adding a new agent role just to compact;
- letting agents decide when to compact;
- making compaction depend on Global Wire-specific ontology;
- claiming readiness from local-only proof;
- using old deterministic summaries as the normal path.

## Test And Proof Requirements

### Unit/Fixture

- LLM compaction prompt builder includes raw entry ids, current objective, and
  retrieval instructions.
- Structured checkpoint parses and renders into prompt-visible summary.
- Invalid LLM checkpoint output fails cleanly or uses a labeled emergency
  fallback.
- Tool-call/result adjacency remains valid after compaction.
- Recent raw tail is preserved.
- Existing `get_run_memory_entry` can recover compacted raw content.
- Context threshold derives from model context window at 70%.
- Explicit diagnostic threshold override still works for tests.

### Local Live Provider

- Run an env-gated DeepSeek or Xiaomi compaction call using real provider keys.
- Prove the compactor returns the typed checkpoint schema without arbitrary
  output caps.
- Prove a subsequent provider call can use the checkpoint and retrieve at least
  one exact raw entry when asked.

### Node B / Staging

Required before declaring provider/config upgrade ready:

- deployed commit identity verified on `https://choir.news`;
- provider/model logs show DeepSeek or Xiaomi handling the compaction path;
- one product-path run crosses automatic compaction threshold or uses an
  explicitly labeled staging diagnostic threshold first, then final proof uses
  the model-aware 70% default if budget permits;
- post-compaction continuation answers questions requiring old context;
- at least one post-compaction `get_run_memory_entry` call succeeds;
- trace/run events show compaction started/completed with before/after
  estimates and no hidden reasoning leak;
- final report distinguishes diagnostic 160k proof from 70%-threshold readiness
  proof.
- `docs/mission-provider-config-conformance-v0.md` is updated with the
  compaction evidence, remaining provider caveats, and explicit Global Wire
  readiness conclusion.

## Anti-Goodhart Constraints

- Do not claim LLM compaction if the checkpoint text was built only by
  deterministic truncation.
- Do not claim long-context safety from a short run that never compacted.
- Do not claim 70%-threshold readiness from a 160k diagnostic override.
- Do not claim Node B proof from local tests.
- Do not hide compaction failure behind a new provider fallback.
- Do not add arbitrary normal-agent output token caps to make tests cheap.
- Do not treat `get_run_memory_entry` as optional in the proof; exact retrieval
  is part of the safety story.
- Do not build Global Wire news fixes during this mission except to write the
  final provider/config readiness conclusion that unlocks that work.

## Rollback Policy

For behavior-changing commits:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

Rollback refs must include:

- commit before LLM compaction changes;
- previous run-memory threshold behavior;
- previous deterministic compaction behavior;
- known-good provider route for ordinary agent loops;
- any staging diagnostic threshold override used during proof.

If LLM compaction destabilizes runs, restore the prior runtime compaction path
as a blocker state, not as a success claim.

## Stopping Conditions

### Complete

The mission is complete only when all are true:

- Runtime uses LLM-generated structured checkpoints for normal automatic
  compaction.
- Deterministic compaction is not the normal readiness path.
- Context threshold defaults to `context_window * 0.7`, with DeepSeek/Xiaomi
  target models treated as 1M-token context models.
- Recent raw tail and exact raw entry retrieval remain available.
- Tool-call/result adjacency tests pass.
- Local live provider compaction proof passes.
- Node B/staging product-path proof demonstrates post-compaction continuation
  and exact retrieval.
- Provider/config mission checkpoint is updated with the compaction result and
  a clear conclusion about whether the model/provider/config upgrade is ready
  to unblock Global Wire.
- If the provider/config stopping conditions are satisfied after compaction,
  `docs/mission-provider-config-conformance-v0.md` is marked complete; if not,
  it remains `checkpoint_incomplete` with only the precise residual provider
  caveats named.

### Checkpoint Incomplete

Use only when useful progress landed but a stopping condition remains unproven.
Name whether the missing piece is LLM checkpoint generation, thresholding,
Node B proof, exact retrieval, or provider behavior.

### Blocked Incomplete

Use only after root-cause probes and at least one serious alternative route.
Name the smallest safe next probe or the external dependency.

## Slash Goalstring

```text
/goal Run docs/mission-llm-run-memory-compaction-v0.md as MissionGradient; ship LLM compaction and close provider conformance.
```

## Run Checkpoint & Resumption State

```text
status: checkpoint_incomplete
last checkpoint:
  2026-06-08 mission created from compaction notes and owner correction:
  deterministic checkpointing is too primitive for readiness; normal path should
  become real LLM compaction, then threshold at context_window * 0.7.

  2026-06-08 implementation preflight problem checkpoint: code inspection
  confirmed the runtime still uses deterministic `summarizeRunMemoryMessages`
  checkpoints assembled from truncated message descriptions, fixed 160k default
  threshold config, and approximate message-only token pressure. The existing
  positive substrate is durable raw run memory, latest-checkpoint rebuild,
  recent tail retention, partial tool-result cut protection, and
  `get_run_memory_entry`. The next behavior-changing commit should replace the
  normal checkpoint path with an LLM-generated typed checkpoint, add
  model-catalog context windows for the 1M-token DeepSeek/Xiaomi models, and
  make model-aware 70% thresholding the normal default while preserving explicit
  diagnostic overrides.
current artifact state:
  Current runtime run-memory compaction is automatic and durable but uses a
  deterministic text checkpoint assembled from truncated message descriptions.
  It preserves raw entries and exposes get_run_memory_entry, but it is not yet
  real LLM compaction. Provider conformance is checkpoint-incomplete primarily
  because compaction/recall readiness and the final Global Wire provider
  readiness conclusion remain unproven.
what shipped:
  Mission doc only.
what was proven:
  Prior deterministic tests prove raw entry retrieval mechanics. They do not
  prove LLM compaction or Node B long-context safety.
unproven or partial claims:
  LLM typed checkpoint generation, model-aware 70% thresholding, Node B
  product-path compaction, post-compaction exact retrieval by a real model, and
  provider/config readiness remain unproven.
belief-state changes:
  The right 80/20 is not to polish deterministic summary formatting. Replace
  normal compaction with LLM structured checkpointing, keep exact retrieval
  handles, add a few reliability guards, prove on Node B, then return to news.
remaining error field:
  Implement runtime LLM compactor, threshold at context_window * 0.7, verify
  tool/message invariants, and prove staging behavior.
highest-impact remaining uncertainty:
  Whether DeepSeek/Xiaomi reliably generate usable typed checkpoints and then
  follow retrieval handles after compaction under product-path pressure.
next executable probe:
  Inspect provider/model catalog context-window representation and runtime
  provider-call plumbing, then implement the smallest shared-harness LLM
  compactor that writes structured details plus prompt-visible summary into
  existing RunMemoryEntry compaction records.
suggested resume goal string:
  /goal Run docs/mission-llm-run-memory-compaction-v0.md as MissionGradient; ship LLM compaction and close provider conformance.
evidence artifact refs:
  Provider/config context: docs/mission-provider-config-conformance-v0.md
rollback refs:
  Revert behavior-changing compaction commits to the commit before this mission's
  implementation begins.
```
