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
