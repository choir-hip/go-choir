# MissionGradient: VText Lineage-Aware Runtime Cadence v2

**Status:** draft
**Date:** 2026-05-25
**Supersedes:** `docs/mission-research-runtime-evidence-cadence-v1.md` as the active runtime-cadence mission frame.
**Purpose:** make VText the lineage-aware single-writer narrative surface for Choir, while research/supervision, Trace, and Chyron provide the right evidence at the right granularity.

## One-Line Goal String

```text
/goal Run docs/mission-vtext-lineage-aware-runtime-cadence-v2.md as a Codex-operated MissionGradient mission: make VText the lineage-aware single-writer narrative surface for Choir. Preserve exact user prompts and subsequent user edits in every VText agent context window, while summarizing older appagent versions, trajectories, worker messages, and tool evidence into compact revision context. Repair the VText/researcher and VText/super cadence so VText writes a fast useful v1, opens researcher/super work asynchronously, terminates quickly, and then wakes on substantive worker updates to produce smaller v2/v3/v4 revisions without waiting for all work to finish. Keep researcher/super/vsuper/cosuper updates as concise substantive messages, not raw tool logs; keep Chyron as the translucent prompt-bar ticker for granular tool calls and agent-to-agent messages; keep Trace as the dense causal ledger. Remove hidden coupling where worker-opening tools force canonical VText writes, unless a better architecture is documented and approved. Preserve the uniform shared harness and do not fork the core tool loop per role. Instrument and prove on staging across no-search, search, coding/super, and mixed prompts with VText revision timelines, exact user-input preservation, revision-context packets, Trace/Chyron evidence, model/provider config, pending mutation/wakeup evidence, screenshots, rollback refs, residual risks, and the next realism axis.
```

## Mission Frame

The v1 research-cadence mission improved the fast path for staged probes, but
manual product use still exposed a deeper VText problem. A broad sports prompt
could write an initial "gathering evidence" revision, stall for several
minutes, and end with a `model stopped at max_tokens` toast. The terminal error
is probably a symptom, not the root cause. The actual failure is that VText did
not keep the user in the loop by integrating early researcher evidence into
smaller revisions.

The real artifact is not a faster search call. It is a lineage-aware document
agent:

```text
user prompt or edit
  -> exact user input is pinned into VText context
  -> VText writes a useful first narrative version
  -> VText opens researcher/super work and returns quickly
  -> worker updates arrive as concise messages
  -> VText wakes, reads compact version/evidence context, and writes the next useful version
  -> older versions and trajectories are summarized without losing exact user input
```

## Core Ontology

VText owns canonical document versions. It is not a trace dashboard and it is
not a raw event reducer. It should explain the whole run state in plain
language: objective, past work, current work, what changed, evidence, learnings,
risks, and next step.

Trace owns dense causal evidence: model calls, tool calls, tool outputs,
provider errors, channel messages, run state, timings, and debug logs.

Chyron owns granular live presence in the prompt bar: tool calls and
agent-to-agent messages streaming as a translucent ticker. Chyron can make the
system feel alive while VText waits for enough substance to revise, but Chyron
is not canonical memory.

Researchers and supers send concise substantive updates. They do not author
VText. Their update cadence should resemble a coding agent's user-facing status
messages: specific enough to supervise, sparse enough not to drown the run.

## Core Invariants

```text
VText is the single canonical document writer.
Exact user prompts and subsequent user edits are preserved in VText context.
Older VText revisions, trajectories, tool outputs, and worker messages may be summarized.
Full evidence remains durable in Trace/content/artifact refs before compression.
VText opens researcher/super work asynchronously and should not remain active while merely waiting.
Substantive worker updates wake VText; raw tiny events go to Trace/Chyron.
Conductor does not author canonical VText content.
Chyron is granular activity, not a substitute for VText narrative revisions.
The shared harness and core tool loop stay uniform across roles unless human-approved evidence requires divergence.
```

## Value Criterion

Minimize user-perceived silence during evidence-bearing work while preserving
canonical writer boundaries, exact user intent, evidence fidelity, and harness
simplicity.

The mission moves uphill when:

- a useful v1 appears quickly for ordinary prompts;
- long-running prompts produce smaller v2/v3/v4 revisions when useful new
  evidence arrives;
- simple prompts do not receive artificial extra revisions;
- VText runs terminate quickly after opening researcher/super work;
- worker updates reliably wake VText when they carry new narrative substance;
- exact user prompts and edits remain visible to VText even after version
  history is summarized;
- Trace and Chyron explain what happened without forcing the user to read raw
  causal logs;
- recurring provider/tool/spawn/max-token errors become rare or precise blockers.

## Revision Context Packet

Each VText model turn should receive a compact context packet assembled from
durable state:

```text
exact_user_input_ledger
  Initial prompt and subsequent user edits, verbatim, ordered by time.

current_document_head
  Latest revision id, title, concise content summary, and active open questions.

recent_revision_window
  Recent revision ids with short summaries or diffs.

older_revision_summary
  A checkpoint summary of older versions when the revision count grows.

pending_worker_updates
  Unconsumed researcher/super/vsuper/cosuper updates addressed to VText.

evidence_refs
  Durable Trace/content/artifact refs for full source data, tool outputs, screenshots, logs, and benchmarks.

run_state
  Active or completed worker handles, model/provider configs, failures, timeouts, and pending mutation/wakeup state.
```

The packet is model context, not the durable truth. Durable truth remains in
Dolt/Trace/content/artifacts. The packet can be pruned and reassembled as long
as exact user input and evidence refs are not lost.

## Worker Update Contract

Researchers and super-family agents should send updates when they have one of:

- first useful evidence;
- changed belief state;
- a blocker or provider/tool failure that affects the answer;
- a completed subtask;
- a meaningful correction to an earlier claim;
- a handoff or next-step recommendation.

They should not send every raw tool call to VText. Raw tool calls are Trace and
Chyron material.

For research, the desired cadence is:

```text
search/fetch starts
  -> Chyron shows the activity
first useful result appears
  -> researcher sends concise first findings to VText
  -> researcher may continue search/fetch in parallel
VText writes v2 from first findings
more evidence changes the answer materially
  -> researcher sends another update
  -> VText writes v3 when useful
```

For coding/supervision, the desired cadence is analogous:

```text
super/vsuper/cosuper work starts
  -> Chyron shows delegation/tool activity
first plan, change, test result, blocker, or candidate evidence appears
  -> responsible agent sends concise update to VText and supervisor
  -> VText writes the next narrative version when useful
```

## Architecture Questions To Resolve

1. **VText run duration:** should VText always return immediately after opening
   worker work, relying on wakeups for later revisions?
2. **Pending mutation locks:** where can an active or pending VText mutation
   block worker-update integration, and how should stale locks be recovered?
3. **Worker-opening tool outputs:** should `spawn_agent` or
   `request_super_execution` ever instruct VText to immediately call
   `edit_vtext`, or should cadence be driven only by VText prompt policy and
   worker-update wakeups?
4. **Version context:** which exact user-input and revision-summary fields are
   already available, and which need a new product/runtime query?
5. **Cancel/resume:** how should user cancellation interrupt VText and downstream
   work while preserving a resumable revision context?

## Implementation Gradient

### 1. Document And Reproduce The Stall

Start with the manual failure class, not an abstract benchmark:

- broad sports prompt such as `Last Night in Baseball`;
- current-events/search prompt;
- no-search prompt;
- coding/super prompt;
- mixed prompt.

Record submission id, trajectory id, document id, revision ids, model/provider
config, tool calls, worker updates, Chyron events, pending mutation state, and
where the run stalls.

### 2. Audit VText State Machine

Inspect the deployed runtime transitions:

```text
initialVTextToolChoice
edit_vtext
spawn_agent
request_super_execution
submit_research_findings
submit_worker_update
maybeWakeVTextOnWorkerMessage
scheduleVTextWorkerWake
reconcileVTextWorkerState
pending mutation reads/writes
active run detection
wait_agent usage
```

Classify every delay:

- no worker was started;
- worker started but produced no update;
- update arrived but did not wake VText;
- wake was scheduled but suppressed;
- VText was active and blocked its own successor;
- VText woke but chose not to write;
- provider/model/tool failed;
- UI failed to display new revision.

### 3. Build Revision Context Packet

Add the smallest product/runtime path that lets VText see exact user inputs,
recent revision history, summarized older history, pending worker updates, and
evidence refs without dumping Trace into the prompt.

Keep it generic. Do not create a research-only context format if the same
VText/super flow needs it.

### 4. Repair Asynchronous Cadence

Prefer architectural simplification over role-specific harness branches:

- VText writes v1, opens work, and exits quickly.
- Worker updates create durable messages and wake VText.
- VText revisions consume acknowledged update cursors.
- Chyron streams granular activity continuously.
- Trace remains the dense debugger.

If code currently forces a canonical VText write from a worker-opening tool
result, remove or replace that coupling only after tests prove the state machine
still wakes on real worker updates.

### 5. Prove Across Prompt Classes

Run staging Playwright/product-path probes across:

- no-search: e.g. `hey`;
- weather/current factual;
- sports/news broad prompt;
- linked-source prompt;
- coding/super prompt;
- mixed research plus action prompt.

For each, record:

- time to v1;
- v2/v3/v4 timings when useful;
- largest evidence-bearing silence;
- worker update count and previews;
- Chyron event count and examples;
- whether exact user input remained in VText context;
- whether old versions were summarized instead of lost;
- tool/provider/spawn/max-token errors;
- final document quality.

### 6. Quality Pass

After behavior improves:

- remove dead compatibility paths;
- simplify wakeup and mutation logic names;
- add focused unit tests for context packet assembly and worker-update wakeups;
- keep heavyweight Playwright probes outside default unit-test paths;
- update this mission with a checkpoint, completion proof, or blocker packet.

## Forbidden Shortcuts

- Making conductor the canonical VText author.
- Treating Chyron as a substitute for VText revisions.
- Dumping raw Trace/tool logs into VText.
- Losing exact user prompts or subsequent user edits during summarization.
- Adding role-specific core tool-loop behavior without explicit approval.
- Forcing artificial v3+ revisions for simple prompts.
- Calling local-only proof success for runtime/gateway/auth/worker behavior.
- Hiding max-token, provider, spawn, wakeup, or pending-mutation failures in Trace only.
- Adding a second document writer to work around VText cadence.

## Evidence Ledger

For each serious proof, record:

```text
staging commit:
Playwright/API command:
prompt class and prompt text:
submission id:
trajectory id:
VText document/revision ids:
exact user input ledger proof:
revision-context packet preview:
worker update timeline:
VText wakeup timeline:
Chyron examples:
Trace/log refs:
model/provider config:
failures and retries:
rollback refs:
residual risks:
```

### 2026-05-25 Staging Reproduction Checkpoint

staging commit: current `https://draft.choir-ip.com` deployment at probe time; deploy identity not yet verified in this checkpoint.
Playwright/API command: `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_MODEL_VARIANTS=fireworks-deepseek-v4-flash-none VTEXT_MODEL_PROMPTS=baseball npx playwright test tests/vtext-researcher-model-cadence-matrix.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
prompt class and prompt text: broad sports/current factual prompt, `Last Night in Baseball`.
submission id: `5da9e6bc-2bfd-4aed-b275-2f1663bd0a50`.
trajectory id: same as submission id for prompt-bar trace lookup unless later evidence identifies a distinct trajectory id.
VText document/revision ids: document `42e11734-7464-40af-96bb-64962d96d157`; appagent revision `186afb6d-d40d-4244-92a2-a6e1b6cdb8aa`.
model/provider config: all roles were set through product-path model policy to `fireworks` / `accounts/fireworks/models/deepseek-v4-flash` / reasoning `none`.
observed transition: VText wrote a status-style first revision at about 8.0s saying evidence was being gathered, but did not call `spawn_agent`, did not start a researcher, and produced no search calls. The probe completed with only one appagent revision, zero search attempts, and no second VText revision.
classification: this run did not reproduce the manual max-token toast. It revealed a narrower classifier/policy failure: `Last Night in Baseball` is a current factual sports request that needs research grounding, but the deployed VText first-turn path did not require or perform research continuation.
baseline comparison: the same matrix probe for `nba update` on the same model policy passed with v1 at about 3.3s, researcher spawn at about 6.3s, first search result at about 10.3s, first findings at about 15.3s, and v2 at about 22.3s.
evidence artifact refs: `/Users/wiz/go-choir/test-results/vtext-model-cadence-matrix-2026-05-25T19-29-44-863Z/fireworks-deepseek-v4-flash-none.json`; Playwright trace `/Users/wiz/go-choir/frontend/test-results/vtext-researcher-model-cad-118ca-orks-deepseek-v4-flash-none-chromium/trace.zip`.
remaining error field: VText can still write "evidence being gathered" without opening the worker that would gather evidence when the prompt wording falls outside the current research classifier.
next safe probe: add focused classifier coverage for "last night in baseball" and related sports recap/current factual phrasing, then rerun the staging baseball prompt after landing to confirm researcher spawn, first findings, and v2 timing.

### 2026-05-25 Reasoning/Model Matrix Reproduction

Playwright/API command: `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_MODEL_VARIANTS=fireworks-deepseek-v4-flash-low,fireworks-deepseek-v4-flash-medium,fireworks-kimi-k2p6-low,chatgpt-gpt-5-5-low VTEXT_MODEL_PROMPTS=baseball npx playwright test tests/vtext-researcher-model-cadence-matrix.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
prompt class and prompt text: broad sports/current factual prompt, `Last Night in Baseball`.
evidence artifact directory: `/Users/wiz/go-choir/test-results/vtext-model-cadence-matrix-2026-05-25T19-33-43-242Z/`.
runner result: all four Playwright cases completed from the test runner's perspective in 10.5m, but the product behavior differed by model/reasoning.

Observed matrix:

| variant | submission id | doc id | first vtext | later revisions | research/search behavior | failure signal |
| --- | --- | --- | --- | --- | --- | --- |
| `fireworks-deepseek-v4-flash-low` | `2fcefccf-f8d2-40f0-9ea2-4e34c201f6c1` | `6205f024-fa83-478f-b665-3cbb799854d5` | 5.7s | none | no `spawn_agent`; zero search queries/attempts | status-style v1 says research in progress without opening researcher |
| `fireworks-deepseek-v4-flash-medium` | `8ae23cae-0bda-4adc-ac03-5c083676a557` | `6660142f-4fdd-4dce-b856-73ff31a10f71` | 4.2s | v2 at 36.2s, v3 at 74.2s | spawned researcher at 8.2s; first search result 15.2s; first findings 24.2s; 5 queries/21 attempts | one duplicate/retry `spawn_agent` error and one `read_evidence` error, but useful revisions arrived |
| `fireworks-kimi-k2p6-low` | `f43588e9-1b90-448b-b0b2-8b1ac58a819b` | `1c5a115e-e783-4903-99e6-04633526dfce` | 5.1s | v2 at 35.1s | spawned researcher at 9.1s; first search result 12.1s; first findings 22.1s; 5 queries/21 attempts | one `cast_agent` error after v2, but useful evidence-backed v2 arrived |
| `chatgpt-gpt-5-5-low` | `20aaa20e-eca4-47e4-b3cc-ed1b3bba9b0b` | `4d13da9d-3b5e-4b45-9bf2-f977d60af83b` | none | none | no tools, no search | first VText provider call failed: `chatgpt: status 400 Bad Request (sanitized)` with `toolChoice=function:edit_vtext` |

Belief-state update: the baseball stall/failure is not one uniform runtime bug. DeepSeek V4 Flash at `none` and `low` can end after a status-style v1 without opening research. DeepSeek V4 Flash `medium` and Kimi `low` recognize the need for research and produce later revisions, though they still show noisy duplicate/coordination errors. GPT-5.5 `low` currently fails before the first required `edit_vtext` tool call, so that path is a provider/request-shape or model-policy compatibility problem, not a VText cadence success/failure signal.

Remaining error field: the product path still needs deterministic runtime/tool-choice enforcement for current factual sports prompts so model reasoning level does not decide whether research is opened; the ChatGPT GPT-5.5 low provider failure needs a separate request-shape diagnosis before it can be included in cadence comparisons.

### 2026-05-25 ChatGPT Request-Shape Root Cause

staging/provider symptom: after switching Node B to a valid ChatGPT account, the `chatgpt-gpt-5-5-low` baseball probe still produced zero VText revisions. Submission `d154a4fd-5f3d-46ee-88a6-8a9e90fc4354`, document `70c27bc6-964c-440c-85d7-36d794089ae5`, and evidence file `/Users/wiz/go-choir/test-results/vtext-model-cadence-matrix-2026-05-25T19-51-20-105Z/chatgpt-gpt-5-5-low.json` show the first VText provider call failed with `chatgpt: status 400 Bad Request (sanitized)` before any tool result.
gateway identity check: Node B `go-choir-gateway` was active, `/var/lib/go-choir/gateway-provider.env` contained `CHATGPT_AUTH_PATH=/var/lib/go-choir/codex-auth.json`, the auth file existed, and gateway logs showed `provider: resolved chatgpt (seed_model=gpt-5.5 reasoning=low)`.
direct endpoint probes: Node B probes against `https://chatgpt.com/backend-api/codex/responses` with the deployed Codex OAuth token showed that `gpt-5.5` accepts `instructions`, `input`, `reasoning.low`, function tools, exact `tool_choice={"type":"function","name":"edit_vtext"}`, and `tool_choice="required"` when `max_output_tokens` is omitted. The same endpoint returns `400 {"detail":"Unsupported parameter: max_output_tokens"}` for `gpt-5.5` and `gpt-5.4-mini` whenever `max_output_tokens` is present, including small values such as `4096` and large values such as `65536`.
root cause: Choir's runtime requests a catalog-derived positive output budget for ChatGPT foreground agent loops. `MaxInteractiveOutputTokensForSelection` currently omits max tokens for Fireworks but not ChatGPT, so VText sends `max_output_tokens=65536` to the ChatGPT Codex Responses endpoint. That parameter is unsupported by the deployed ChatGPT endpoint and causes the first required `edit_vtext` turn to fail before tools can run.
candidate fix: treat ChatGPT like Fireworks for ordinary interactive loops: omit explicit max-output-token parameters unless a per-computer/model policy explicitly sets `max_tokens`. Keep explicit max token overrides available for future provider paths only if the endpoint supports them.

### 2026-05-25 ChatGPT Max-Token Fix Proof

commits: documentation checkpoint `c55c587`; code fix `8c97fab9163b5b914b915e8349729e0e0e8d475f`.
CI/deploy: GitHub Actions CI run `26417709079` passed, including non-runtime tests, runtime shards 0-3, vet/build, and staging deploy. FlakeHub publish run `26417709068` passed. Staging `/health` reported proxy and sandbox deployed at commit `8c97fab9163b5b914b915e8349729e0e0e8d475f`, deployed at `2026-05-25T20:04:53Z`.
local verification before push: `nix develop -c go test ./internal/provider ./internal/gatewayruntime -count=1` passed; focused runtime/provider checks for ChatGPT provider serialization, model-policy max-token selection, exact tool choice, and max-token loop behavior passed.
deployed proof command: `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_MODEL_VARIANTS=chatgpt-gpt-5-5-low VTEXT_MODEL_PROMPTS=baseball npx playwright test tests/vtext-researcher-model-cadence-matrix.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
deployed proof result: passed in 2.0m. Evidence file `/Users/wiz/go-choir/test-results/vtext-model-cadence-matrix-2026-05-25T20-05-32-837Z/chatgpt-gpt-5-5-low.json`.
submission id: `45bf7bda-1e4d-4b6a-aff5-d028f77c7282`.
document id: `b8b76621-74bd-4881-b497-232969355f60`.
revision timeline: v1 `64ce1bd4-0588-4aa1-b2d5-daf3126c1951` at about 4.7s; v2 `d30016da-4b2a-48e3-bf6b-6577eeb68517` at about 41.7s; v3 `bf90fc53-ec7d-4f81-9fb6-2dc43e8e9a8e` at about 90.7s.
worker/search timeline: researcher spawn at about 10.7s; first search result at about 13.7s; first findings at about 28.7s; first findings to next VText revision about 13.0s; 12 search queries and 49 search attempts.
gateway evidence: after deploy, Node B gateway logs showed ChatGPT calls with `max_tokens=0`, including `tool_choice=function:edit_vtext` and `tool_choice=function:submit_research_findings`; the prior `400 Bad Request` failure did not recur.
residual risks: the proof still recorded duplicate/stale `edit_vtext` tool-result errors with output length 52 while successful revisions were created. This is not the ChatGPT max-token root cause, but it remains part of the broader VText cadence cleanup field.

### 2026-05-25 Coordination-Noise Baseline

Playwright/API command: `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_MODEL_VARIANTS=fireworks-deepseek-v4-flash-medium,fireworks-kimi-k2p6-low,chatgpt-gpt-5-5-low VTEXT_MODEL_PROMPTS=baseball,deep-research,linked-source,coding-super,mixed npx playwright test tests/vtext-researcher-model-cadence-matrix.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
evidence artifact directory: `/Users/wiz/go-choir/test-results/vtext-model-cadence-matrix-2026-05-25T20-16-55-259Z/`.
prompt classes: broad sports research (`Last Night in Baseball`), deeper current research (`research the current Artemis II launch schedule...`), linked-source synthesis, bounded super/coding execution (`write and run a tiny shell command...`), and mixed research plus shell-command drafting.
model scope: product-path model policy set conductor, VText, researcher, super, vsuper, co-super, and verifier roles to the same tested provider/model/reasoning variant for each run.

Observed matrix highlights:

| variant | useful research cadence | super/coding cadence | coordination-noise signal |
| --- | --- | --- | --- |
| `fireworks-deepseek-v4-flash-medium` | baseball v1 5.3s, findings 27.3s, v2 38.3s; deep research v1 9.1s, findings 31.1s, v2 54.1s | mixed routed to super once but produced no v2 within the observation window | no completed-mutation VText errors in captured successful prompts; researcher import attempts hit ordinary 403 fetch errors |
| `fireworks-kimi-k2p6-low` | baseball v1 2.6s, findings 15.6s, v2 23.6s; deep research v1 5.7s, findings 32.7s, v2 46.7s | mixed routed to super once but produced no v2 within the observation window | deep research produced two post-success `edit_vtext` errors: `vtext mutation is completed, not pending`; baseball had a VText `cast_agent target lookup: record not found` after useful revision work |
| `chatgpt-gpt-5-5-low` | all five prompts completed with v1/v2; baseball v1 4.9s, findings 26.9s, v2 37.9s; deep research v1 5.5s, findings 24.5s, v2 44.5s | coding-super v1 4.2s, super request(s), v2 25.2s with command/output evidence | every prompt showed post-success `edit_vtext` calls rejected as `vtext mutation is completed, not pending`; deep research also repeated duplicate `submit_research_findings` primary-key inserts; coding-super had a duplicate `bash` command skip |

Probe limitations: the `fireworks-deepseek-v4-flash-medium` and `fireworks-kimi-k2p6-low` columns both hit transient authenticated API `401` responses for `linked-source` and `coding-super` within the Playwright session, while later prompts in the same variant continued. Treat those rows as probe/session noise until reproduced separately.

belief-state update: the highest-confidence coordination-noise bug is not model-specific factual quality. It is a loop termination/idempotency issue after successful VText side effects. `edit_vtext` creates or completes the canonical mutation, but the tool loop then feeds the result back into another provider iteration. Some models then call `edit_vtext` again or try adjacent coordination tools, producing noisy mutation-state errors even though the useful revision already exists. Worker-opening tools (`spawn_agent` and `request_super_execution`) are similar side-effect boundaries: after the required continuation has succeeded, the VText run should normally return and let worker updates wake the next VText run.

remaining error field: VText tool-loop termination should become explicit for successful VText side-effect tools unless their result declares a `next_required_tool`. Same-turn duplicate `edit_vtext` calls should not race or produce mutation-state errors after the first successful edit. Researcher duplicate `submit_research_findings` evidence-id errors and super duplicate `bash` planning remain secondary coordination-noise classes after VText termination is fixed.

candidate fix: add a generic tool-loop terminal-success option, enabled for VText runs with `edit_vtext`, `spawn_agent`, and `request_super_execution`. After a successful terminal tool batch with no `next_required_tool`, return from the loop instead of asking the model for another turn. Separately, skip additional `edit_vtext` calls in the same VText tool batch as a non-error duplicate notice so a model that emits two edits at once cannot turn the second one into a completed-mutation error.

### 2026-05-25 Terminal-Tool Fix Acceptance Blocker

commit under test: `3e064c319cfcfdaf13a60781c3cf013d96d843e6`.
staging identity: `/health` reported proxy and sandbox deployed at `3e064c319cfcfdaf13a60781c3cf013d96d843e6`.
acceptance command: `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_MODEL_VARIANTS=fireworks-deepseek-v4-flash-medium,fireworks-kimi-k2p6-low,chatgpt-gpt-5-5-low VTEXT_MODEL_PROMPTS=baseball,deep-research,coding-super npx playwright test tests/vtext-researcher-model-cadence-matrix.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
partial evidence artifact: `/Users/wiz/go-choir/test-results/vtext-model-cadence-matrix-2026-05-25T20-49-02-782Z/fireworks-deepseek-v4-flash-medium.json`.
observed transition: after the terminal-tool fix, `fireworks-deepseek-v4-flash-medium` produced a clean first VText revision for `Last Night in Baseball` with no post-success mutation errors, but it did not open researcher work or produce v2 in the observation window.
root cause: this is the previously observed research-classifier gap resurfacing once VText no longer asks the model for an extra free-form turn after `edit_vtext`. The prompt has current factual sports intent but does not contain the existing `mlb`/`score`/`update` markers.
remaining error field: terminal VText runs need deterministic research continuation for baseball/current recap phrasing, not reliance on the model voluntarily calling `spawn_agent` after a successful edit.
candidate fix: extend the VText research-continuation classifier to cover `baseball`, `last night`, and recap phrasing, with unit coverage for `Last Night in Baseball`.

### 2026-05-25 Terminal-Tool Fix Final Proof

behavior commit: `3c16961d37ab41959dd7cbe3ca7fe73a98e8b8ff`.
CI/deploy: GitHub Actions CI run `26419476815` passed, including runtime shards, non-runtime tests, vet/build, and staging deploy. FlakeHub publish run `26419476784` passed.
staging identity: `/health` reported proxy and sandbox deployed at `3c16961d37ab41959dd7cbe3ca7fe73a98e8b8ff`, deployed at `2026-05-25T20:57:48Z`.
local verification: `nix develop -c go test ./internal/runtime -run 'TestRunToolLoopTerminalToolSuccessStopsWithoutExtraProviderTurn|TestRunToolLoopRequiredNextToolSatisfiedInSameBatchDoesNotRetry|TestExecuteToolsSkipsDuplicateVTextEditsInSameTurn|TestVTextInitialEditRequiresContinuationButSpawnDoesNotForceSecondEdit|TestRunToolLoopRequiredNextTool|TestRunToolLoopMaxTokens' -count=1` passed; broader focused runtime/VText/tool-loop checks also passed.

research acceptance command: `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_MODEL_VARIANTS=fireworks-deepseek-v4-flash-medium,fireworks-kimi-k2p6-low,chatgpt-gpt-5-5-low VTEXT_MODEL_PROMPTS=baseball,deep-research npx playwright test tests/vtext-researcher-model-cadence-matrix.tmp.spec.js --project=chromium --workers=1 --reporter=line`, executed as part of a three-prompt matrix where the coding rows hit a late-session 401 and were rerun separately.
research evidence directory: `/Users/wiz/go-choir/test-results/vtext-model-cadence-matrix-2026-05-25T20-58-18-911Z/`.
research result: all successful baseball and deep-research rows across `fireworks-deepseek-v4-flash-medium`, `fireworks-kimi-k2p6-low`, and `chatgpt-gpt-5-5-low` produced v1, researcher findings, and v2 with `edit_vtext` error count `0`, VText mutation-state error count `0`, and duplicate-tool error count `0`.
baseball classifier proof: after adding `baseball`/`last night`/`recap` research markers, `fireworks-deepseek-v4-flash-medium` for `Last Night in Baseball` produced v1 at about 6.6s, first findings at about 24.6s, and v2 at about 45.6s.

super/coding acceptance command: `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_MODEL_VARIANTS=fireworks-deepseek-v4-flash-medium,fireworks-kimi-k2p6-low,chatgpt-gpt-5-5-low VTEXT_MODEL_PROMPTS=coding-super npx playwright test tests/vtext-researcher-model-cadence-matrix.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
super/coding evidence directory: `/Users/wiz/go-choir/test-results/vtext-model-cadence-matrix-2026-05-25T21-14-37-624Z/`.
super/coding result: all three variants produced v1, requested super execution, and produced v2 with `edit_vtext` error count `0` and VText mutation-state error count `0`. `fireworks-deepseek-v4-flash-medium` had v1 about 3.3s and v2 about 17.3s; `fireworks-kimi-k2p6-low` had v1 about 4.2s and v2 about 16.2s; `chatgpt-gpt-5-5-low` had v1 about 3.8s and v2 about 29.8s.
remaining coordination noise: GPT-5.5 low still produced duplicate `bash` planning inside the super run for the coding-super prompt; those were skipped by existing duplicate-bash guards and did not create VText mutation errors. Researcher-side tool noise remains for content-file reads/imports and duplicate finding ids in some model/prompt combinations. These are now distinct secondary noise classes, not the VText completed-mutation loop.
rollback refs: before terminal-tool behavior change, `cd580c5`; before classifier fix, `3e064c3`.

### 2026-05-26 Manual QA Regression Checkpoint

staging commit: `/health` reported proxy and sandbox deployed at `f3d48e7cbc49b1cbf5e70c96d536b2e5e1517778`, deployed at `2026-05-26T02:35:54Z`.
manual QA evidence: mobile Safari screenshots at about 10:35-10:37 showed `CHOIR BIOS` bootstrap failures with `BOOTSTRAP FAILED (502)`, `Bootstrap probe 1 is still waiting; retrying`, and repeated `VM route returned 502; retrying`. The same manual session showed simple baseball VText documents staying in weak early states: `Baseball Scores` v1 said research was in progress with no grounded data yet, and `Baseball tonight` v2 said prior findings were too broad, referenced the target date `May 26, 2026`, and was still waiting for targeted research.
health evidence: staging `/health` at investigation time reported `vmctl_status=ok`, but lifecycle counters showed bootstrap instability: `bootstrap.total` count `89`, errors `23`, statuses `http_502=8` and `resolve_error=15`; `bootstrap.resolve` max duration `15098ms`; `bootstrap.upstream` max duration `5156ms`; general API counters also showed `api.total` `http_502=17` plus `resolve_error=18`.
model-policy evidence: deployed runtime defaults still generate DeepSeek V4 Flash with `reasoning = "none"` for `conductor`, `researcher`, and `vtext`, and platform fallback uses the same `none` reasoning for those roles. This conflicts with the current accepted suite direction, where `fireworks-deepseek-v4-flash-medium`, `fireworks-kimi-k2p6-low`, and `chatgpt-gpt-5-5-low` are the comparison targets. It also matches the earlier matrix result where V4 Flash `none`/`low` could write status-style VText without useful research continuation while V4 Flash `medium` did produce later evidence-backed revisions.
classification: the manual regression is probably a compound failure, not one model-quality issue. VM/bootstrap pressure is real at the proxy lifecycle layer, and VText/researcher quality is plausibly worsened by live/default policy still allowing V4 Flash `none` for the foreground research cadence roles.
remaining error field: determine whether the current user's persistent `System/model-policy.toml` is a generated stale policy, a custom policy, or a fallback path; change generated/fallback policy so new and generated-default computers use V4 Flash `medium` for conductor/VText/researcher; decide whether stale generated policies should migrate to the new default without rewriting genuinely custom owner policy; then rerun a small staging proof before broader evals. Separately investigate VM route 502/resolve pressure and avoid long Playwright matrices while bootstrap lifecycle counters remain noisy.

### 2026-05-26 Default-Policy Proof Revealed Temporal Grounding Gap

behavior commit under test: `e537327d0b12ac65b24b84a41346c64ce9ab9ac6`.
deployed proof command: `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com VTEXT_DEFAULT_POLICY_EVIDENCE_DIR=../test-results/vtext-default-policy-staging-e537327-20260526T024846Z pnpm exec playwright test tests/vtext-default-policy-proof.tmp.spec.js --project=chromium --workers=1 --reporter=line`.
evidence artifact: `frontend/test-results/vtext-default-policy-proof-b42d9-sh-medium-for-VText-cadence-chromium/attachments/default-policy-proof-e305f1048c113da50c80951bbeeae28465af1da0.json`.
observed transition: fresh generated-default policy produced VText and researcher runs on `fireworks/accounts/fireworks/models/deepseek-v4-flash` with `reasoning=medium`, and the document reached two appagent revisions in about 46s.
quality failure: the v2 content still claimed `Last Night in Baseball` covered a `May 12-13, 2026` date range, even though the staging proof ran on `May 26, 2026`. That reproduces the user's manual complaint that the system can progress past v1/v2 while still fetching wrong or useless time-sensitive information.
root cause: neither the shared role system prompt nor `buildVTextResearchContinuationObjective` supplies an absolute current date/time or tells researchers how to anchor relative-date requests such as `last night`, `today`, `tonight`, `latest`, or `now`. The researcher objective only repeats the raw user prompt, so search/query selection can drift to stale snippets.
remaining error field: add temporal context to agent prompts and researcher delegation objectives without adding deterministic routing. Then rerun a narrow default-policy baseball proof and require the first/second revisions not to mention stale May 12-13-style date ranges.

## Run Checkpoint And Resumption State

```text
status: draft
last checkpoint: v1 research cadence shipped durable search projections, Parallel Search integration, provider cooldowns, tool-loop request/response instrumentation, and a narrow first-findings contract. Staging probes improved, but manual broad sports use still stalled for minutes before a max-token error.
current artifact state: `docs/mission-research-runtime-evidence-cadence-v1.md` records the latest v1 checkpoint. The working tree may contain candidate runtime edits removing reverse-coupling where `spawn_agent` and `request_super_execution` returned `next_required_tool=edit_vtext`; those edits are plausible but not yet proven on staging.
what shipped: see the v1 mission checkpoint and git history around `8c18016` and `4e18a1e`.
what was proven: deployed probes showed fast first findings for `nba update` after `8c18016`, but manual broad sports prompts can still fail the real product expectation.
unproven or partial claims: whether the stall comes from VText staying active, worker wake suppression, prompt/tool policy, provider timeout, max-token request shape, stale model policy, or UI catch-up; whether the candidate removal of worker-tool forced VText writes is sufficient or merely cleanup.
belief-state changes: VText version lineage and exact user-input preservation are now central. Research cadence is one instance of a broader VText/supervision cadence problem.
remaining error field: long evidence-bearing silence; status-style v1 revisions that do not evolve; unclear pending mutation/wakeup behavior; sparse version-awareness in VText context.
highest-impact remaining uncertainty: the exact stalled transition for the manual broad sports failure.
next executable probe: reproduce `Last Night in Baseball` or an equivalent broad sports prompt on staging with Trace/Playwright polling, then inspect VText active-run, pending-mutation, worker-update, and wakeup state before making further runtime changes.
suggested resume goal string: use the One-Line Goal String above.
evidence artifact refs: `docs/mission-research-runtime-evidence-cadence-v1.md`; `frontend/test-results/vtext-model-cadence-sports-requiredtool-20260525T184650Z/fireworks-deepseek-v4-flash-none.json`; `frontend/test-results/vtext-model-cadence-breadth-requiredtool-20260525T185024Z/fireworks-deepseek-v4-flash-none.json`; `frontend/test-results/vtext-model-cadence-mixed-requiredtool-rerun-20260525T185623Z/fireworks-deepseek-v4-flash-none.json`.
rollback refs: before the v1 cadence run, `f482da6`; before the first-findings contract, `4a625c6`/`d39788e` depending on the rollback target desired.
```
