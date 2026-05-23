# MissionGradient: Runtime Model And Context Substrate v0

**Status:** checkpoint_incomplete — deployed substrate patch at `415b87e`; product-path proof still pending
**Date:** 2026-05-23
**Purpose:** remove execution blockers and make model/context configuration good enough to resume the Chyron, Motion, Liquid, and Python mode experiment rerun.
**Depends on:** [mission-human-proof-experiment-rerun-v1.md](mission-human-proof-experiment-rerun-v1.md)

## One-Line Goal String

```text
/goal Run docs/mission-runtime-model-context-substrate-v0.md as a Codex-operated MissionGradient mission: fix the current blocking runtime issues, add the Fireworks DeepSeek/Kimi models, make per-computer model policy editable as durable text, add multimodal provider message support for screenshots/images, and upgrade run context management using Pi-style batch pruning with recoverable raw tool outputs. Preserve platform provider secrets as server-owned, user model policy as computer-owned editable state, Dolt/product APIs as canonical state, and Trace/VText/run-acceptance as evidence. Land through git/CI/deploy, verify staging identity, and resume the human-proof experiment rerun only after deployed product-path proof shows auth/login, gateway model routing, model policy resolution, multimodal evidence plumbing, and context compaction/retrieval are working or precisely blocked.
```

## Mission Frame

The experiment rerun is currently blocked by substrate quality rather than by
the Chyron idea itself. The system needs to be able to:

- authenticate reliably under staging pressure;
- route model calls through the right provider/model without redeploying for
  every model addition;
- let a user computer modify its own model policy by normal Choir agency;
- send screenshots and image evidence to multimodal models;
- keep long agent runs bounded without losing exact tool-output evidence.

This mission is a prerequisite to returning to:

1. Chyron Shelf observability.
2. Process/window/agent animation language.
3. Choir Liquid Material Engine.
4. Python code mode.

Do not hand-code those experiment features in this mission. This mission fixes
the execution substrate that lets Choir-in-Choir attempt them honestly.

## Core Invariants

```text
Provider secrets are platform/server-owned.
Model policy is user-computer-owned editable state.
Model calls record the exact resolved provider/model/reasoning used.
Multimodal evidence is passed as durable artifacts, not loose browser state.
Run memory pruning removes bulk from hot context, never from durable memory.
```

## Real Artifact

The artifact is a deployed runtime/model/context substrate:

```text
blocking auth/runtime issue fixed
  -> Fireworks model catalog includes new DeepSeek/Kimi models
  -> per-computer model policy exists as editable text
  -> runtime validates and resolves that policy per run
  -> provider schema can carry screenshot/image artifacts to multimodal models
  -> run memory prunes batches and preserves exact raw outputs for retrieval
  -> Trace/VText/run acceptance explain what model and context path was used
```

## Revised Implementation Order

### 1. Fix Blocking Runtime Issues

Start with the observed staging blockers, not speculation. At minimum:

- root-cause the `login/begin failed: failed to save challenge` failure;
- fix the auth persistence/write-contention path if confirmed;
- prove login/auth through deployed staging product path;
- preserve existing ChatGPT auth on Node B; do not switch accounts unless the
  user explicitly asks.

Do not move to model work while a basic login/gateway blocker is still active
unless the model work is independent and safe to do in parallel.

### 2. Add Fireworks Models To The Platform Catalog

Add these model ids to Choir's Fireworks catalog/config:

- `accounts/fireworks/models/deepseek-v4-pro`
- `accounts/fireworks/models/deepseek-v4-flash`
- `accounts/fireworks/models/kimi-k2p6`

Represent capability truthfully:

- DeepSeek models are text-only unless later evidence proves otherwise.
- Kimi K2.6 is cataloged as multimodal-capable.
- A model's upstream capability is distinct from Choir's adapter capability.
  If Choir can only send text at a checkpoint, say so.

### 3. Add Multimodal Provider Message Support

Extend the provider request schema so multimodal models can receive screenshots
and other image evidence.

Preferred product primitive:

```text
artifact_ref -> runtime/gateway resolves -> provider-specific image payload
```

This keeps screenshot provenance, redaction, size limits, and replayability
inside Choir's evidence model.

Required behavior:

- text-only models reject or ignore image artifacts explicitly;
- multimodal-capable provider adapters can receive text plus images;
- screenshot/image artifacts used for verification are visible in Trace or run
  acceptance as evidence refs;
- no private DOM or secret-bearing screenshot is sent without an explicit
  verifier/evidence path.

### 4. Add Editable Per-Computer Model Policy

Model policy must be easy for the user or super to change. The canonical user
policy should be a durable editable text file inside the computer, not only a
hidden settings row.

Candidate location:

```text
/System/Model Policy.md
```

or a structured text file such as:

```text
/System/model-policy.toml
```

Example policy shape:

```toml
[defaults]
fallback_provider = "chatgpt"
fallback_model = "gpt-5.5"

[roles.conductor]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "low"

[roles.super]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "medium"

[roles.vsuper]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-pro"

[roles.cosuper_coding]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-pro"

[roles.vtext]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"

[roles.researcher]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"

[roles.verifier_multimodal]
provider = "fireworks"
model = "accounts/fireworks/models/kimi-k2p6"
requires = ["image", "tool_use"]
```

Runtime requirements:

- parse and validate the policy into an effective role table;
- keep the previous valid policy if an edit is invalid;
- use platform fallback defaults so a bad policy does not brick the computer;
- apply policy changes to new runs, not mid-call mutation of already running
  agents;
- record the exact resolved provider/model/reasoning in run metadata, Trace,
  and run acceptance evidence;
- let prompt-bar -> VText/super update the text policy through normal
  computer-owned mutation.

### 5. Upgrade Run Context Management

Use the `pi-context-prune` pattern as the design reference:

- keep the complete durable session/run log;
- remove bulk old tool results from hot model context, not from storage;
- summarize completed batches rather than pruning every tiny tool turn;
- preserve tool-call ids, tool names, arguments, status, turn index, timestamp,
  and full raw result text in a recoverable index;
- let agents retrieve exact old outputs by id when summaries are insufficient;
- avoid needless cache busting by batching pruning around meaningful work units.

Choir's current run memory already has compaction entries and recent-tail
reconstruction. This mission should improve it toward recoverable raw-output
pruning rather than replacing it with an unrelated memory system.

Minimum acceptable upgrade:

- summaries name the raw tool-output ids they summarize;
- exact raw tool outputs remain durable and retrievable by tool/output id;
- context reconstruction uses latest summary plus recent tail;
- tool-call/result boundaries are not split in a way that leaves the model
  without the right anchors;
- Trace exposes compaction/pruning events and retrieval refs clearly enough for
  debugging long runs.

## Authority Boundaries

**Platform/server-owned**

- provider API keys and OAuth credentials;
- gateway provider implementations;
- platform model catalog and model health;
- staging deploy and CI.

**User-computer-owned**

- effective model role policy;
- policy text file and its revision history;
- run metadata and model resolution records;
- VText explanations of policy changes.

**Run-owned**

- resolved model choice;
- media artifact refs passed to the provider;
- context pruning checkpoints;
- raw-output retrieval ids.

## Acceptance Evidence

The mission is complete only with deployed evidence for:

- auth/login blocker fixed or precisely isolated;
- the three Fireworks models visible in the model catalog/config path;
- at least one run resolved from editable per-computer model policy;
- model resolution recorded in Trace/run metadata;
- a multimodal request path accepts a screenshot/image artifact or precisely
  reports the remaining adapter blocker;
- context pruning produces a compact hot context while preserving and
  retrieving exact raw tool output;
- staging health reports the deployed commit used for proof.

## Forbidden Shortcuts

- Do not switch ChatGPT auth accounts without user instruction.
- Do not hard-code only one new model path and call runtime config done.
- Do not put user model policy only in environment variables.
- Do not store provider secrets in user-computer state.
- Do not claim Kimi screenshot verification works if the adapter only sends
  text.
- Do not prune by deleting raw tool outputs.
- Do not use local-only proof for auth, gateway routing, model policy, or
  Choir-in-Choir runtime behavior.
- Do not return to Chyron/Motion/Liquid/Python until this substrate is proven
  or a precise lower-level blocker is documented.

## Dense Feedback

During implementation, keep a small evidence ledger:

- root cause and fix for each blocker;
- model catalog before/after;
- effective model policy file revision;
- run id showing policy resolution;
- multimodal artifact id and provider request path;
- run-memory pruning checkpoint id;
- raw-output retrieval id;
- CI/deploy/staging identity.

## Run Checkpoint & Resumption State

Use this section during execution.

```text
status: checkpoint_incomplete
last checkpoint: substrate patch landed and deployed at 415b87e; local test-loop cleanup adbc04f is committed but not pushed
current artifact state: auth SQLite busy handling, Fireworks catalog entries, per-computer model policy file, provider image blocks, per-run model routing, run-memory raw-entry retrieval, and the AGENTS dev-shell/runtime-test guidance are implemented. Staging currently reports proxy and sandbox deployed_commit 415b87ee5167382250087b60c26aa18b4423b789.
what shipped: 415b87ee5167382250087b60c26aa18b4423b789 "Harden runtime model and context substrate" shipped through GitHub Actions run 26344950167 and deployed to staging at 2026-05-23T22:17:41Z. adbc04f871f772dbda6feba33ab2c3abaa639ddf "Speed up local runtime test loop" is local-only at this checkpoint and must be pushed before a clean long run.
what was proven: focused dev-shell tests passed for auth busy timeout, provider multimodal conversion/validation, gateway per-run model routing, model-policy resolution/fallback, run-memory compaction raw ids, store run-memory retrieval, stub-provider zero-delay behavior, streaming completion polling, and runtime shard script execution for representative shards. Staging health proves 415b87e is live.
unproven or partial claims: deployed product-path auth/login under long-run pressure, gateway routing through editable per-computer model policy, artifact_ref image resolver into a live multimodal provider call, Trace/run-acceptance display of model/context evidence, product-path compaction retrieval, and the full Chyron human-proof rerun remain unproven.
belief-state changes: the model/context substrate is no longer purely local; the next risk is product-path integration rather than code existence. Runtime package tests are broad embedded-Dolt integration tests; CI shards them on separate runners, while local execution should use scripts/go-test-runtime-shards or scripts/go-test-local instead of unbounded serial package runs.
remaining error field: staging auth/gateway/model/context reliability under real Choir-in-Choir load; whether adbc04f changes CI/deploy behavior; whether the Chyron proof can use model/context improvements without exposing new evidence gaps.
highest-impact remaining uncertainty: whether staging product-path runs resolve and record the new per-computer model policy, preserve auth/gateway reliability, and let Choir-in-Choir reach human proof without Codex hand-coding the experiment.
next executable probe: push adbc04f, monitor CI/deploy, verify staging identity, then run a narrow product-path auth/model/context smoke before resuming the Chyron proof.
suggested resume goal string: use the one-line goal string above
evidence artifact refs: local `nix develop -c go test ./cmd/gateway ./cmd/sandbox ./internal/auth ./internal/provider ./internal/store -count=1`; local `nix develop -c go test ./internal/runtime -run 'TestParseModelPolicyResolvesRoles|TestRuntimeResolvesModelPolicyIntoRunMetadata|TestRuntimeFallsBackToPreviousValidModelPolicy|TestRunMemoryCompactionNamesRawRetrievalEntries|TestRunMemoryCompactionDoesNotSplitToolResultPair|TestDelegateWorkerVMMarksPackageRequiredVSuperWithoutPackageIncomplete' -count=1 -timeout=90s`; local focused aggregate changed-path test with the same new test set
rollback refs: revert 415b87ee5167382250087b60c26aa18b4423b789 for model/context substrate regressions; revert adbc04f871f772dbda6feba33ab2c3abaa639ddf if the local/CI shard script path regresses CI or developer test behavior after push.
```
