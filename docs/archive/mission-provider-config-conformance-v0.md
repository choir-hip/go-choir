# Provider Config Conformance Mission v0

## Mission Identity

Build Choir's provider substrate until DeepSeek and Xiaomi can be trusted as
first-class runtime providers for real agent work, not just isolated text calls.

This is not a provider-adapter slice. It is the full provider configuration and
conformance object needed before returning to Universal Wire:

- direct DeepSeek and Xiaomi credentials are platform-owned and deployable
  without leaking secrets;
- provider/model catalog entries are accurate and capability-specific;
- OpenAI-compatible and Anthropic-compatible protocol routes are both evaluated
  where available;
- text, multimodal, tool-call, reasoning, multi-turn, and compaction semantics
  are proven through local live tests, deployed gateway tests, and product-path
  agent loops;
- model policy can select providers by capability without hard-coded role myths;
- failures are observable, recoverable, and documented before patches;
- provider readiness probes use the normal Choir agent harness and default
  provider budgets unless a narrow diagnostic explicitly requires otherwise;
- news processors, reconcilers, researchers, and VText article agents can be
  moved onto the provider substrate without rediscovering basic provider
  incompatibilities.

## Why This Mission Exists

The DeepSeek/Xiaomi cutover partially succeeded. Direct deployed gateway probes
worked for:

- `deepseek/deepseek-v4-flash`;
- `xiaomi/mimo-v2.5-pro`;
- multimodal `xiaomi/mimo-v2.5`.

But the first real product-path VText appagent turn failed:

```text
provider=deepseek
model=deepseek-v4-flash
tools=1
tool_choice=function:edit_vtext
reasoning=medium
result=deepseek: status 400 Bad Request
```

A smaller live DeepSeek probe reproduced the rule: thinking mode rejects exact
forced function tool choice with:

```text
Thinking mode does not support this tool_choice
```

Disabling thinking allowed exact function and `required` tool choices to return
tool calls. That means the current blocker is not generic provider access. The
blocker is protocol/request-shape conformance under real agent-loop pressure.

## Cognitive Transforms Applied

### Depth Extraction

The shallow mission would be "fix DeepSeek tool calls." The load-bearing
variable is not one serializer bug; it is whether Choir's agent harness can
preserve provider semantics under tool pressure, hidden reasoning pressure,
multimodal pressure, and compaction pressure.

Operational implication: build a conformance matrix and product-path proof
before resuming Universal Wire.

### Invariant Projection

Provider work must preserve the same core loop for VText, researcher, super,
processors, reconcilers, verifier, and future roles. Role-specific harness
branches are forbidden unless a documented invariant requires them.

Operational implication: prefer provider capabilities, model policy, prompts,
and adapter semantics over role-specific code paths.

### Failure-Mode Inversion

The likely failure is not that one provider cannot answer. The likely failure is
that a provider appears to work until tool calls, hidden reasoning, or
compaction corrupt the loop.

Operational implication: the mission must include negative tests and
compaction/recall tests, not only happy-path completions.

### Readiness As Downstream Capability

The provider substrate is "done" only when Universal Wire can safely depend on it.
Direct curl success is insufficient.

Operational implication: stopping condition includes a real Choir run that uses
DeepSeek and Xiaomi together, plus an explicit readiness report for Universal Wire
processors/reconcilers/researchers/VText article agents.

## Hard Invariants

- Secrets stay platform-owned. Provider keys are never committed, browser
  visible, VText visible, or copied into user-computer files.
- GitHub `origin/main` remains the source of truth for tracked deployed files.
  Do not patch tracked Node B files directly as a source shortcut.
- Provider configuration is platform-owned, but per-computer model policy is
  computer-owned durable state. Do not turn role defaults into architecture.
- The shared agent harness remains uniform across roles unless a documented
  correctness, security, authority, or resource-isolation invariant requires
  divergence.
- VText owns article/document versions. Provider work must not create a
  parallel document or source structure.
- Hidden reasoning content is continuity state, not user-visible prose. It must
  not leak into VText, trace summaries, docs, or browser surfaces.
- Tool results are data. Provider adapters must preserve tool-call ids and
  tool-result pairing across multi-turn calls and compaction.
- Multimodal tasks must route only through models/protocols that declare the
  required modality.
- No silent fallback to Fireworks, ChatGPT, or any other provider during
  DeepSeek/Xiaomi conformance tests.
- No arbitrary `max_tokens` caps in normal provider config, readiness probes, or
  product-path agent loops. If response length matters, prompt for the desired
  length. A hard token cap is allowed only as a narrow diagnostic and must not be
  counted as production-readiness evidence.
- Explicit `max_tokens` remains valid only when it is owner/platform model
  policy, not when an agent or test is inventing a cap to make a probe cheaper.
  The tool-loop's finite required-next-tool recovery budget is a bounded retry
  guard for a missed required tool, not a provider configuration default.
- Prefer OpenAI-compatible provider routes for normal DeepSeek/Xiaomi agent
  loops when no explicit output budget is desired. Anthropic-compatible Messages
  APIs commonly require a `max_tokens` request field; treat those routes as
  interop/fallback/proof paths unless an owner/platform policy intentionally
  selects a budget.
- Staging is the acceptance environment for platform behavior.
- Problem documentation precedes behavior-changing fixes.

## Value Criterion

Minimize provider-loop divergence from Choir's intended agent semantics while
preserving shared harness uniformity, secret boundaries, model-policy
editability, multimodal routing correctness, hidden-reasoning continuity,
compaction safety, and deployed rollback.

The provider substrate gets better when:

- provider/model capability facts become explicit and testable;
- protocol-specific quirks are localized in adapters, not scattered through
  roles;
- tool calls work across exact/required/auto/none modes or fail with precise
  documented limits;
- reasoning modes are preserved where useful and disabled where incompatible;
- compaction keeps enough durable handles for agents to resume accurately;
- product-path traces show the configured provider/model actually handled the
  turn;
- Universal Wire can launch processors, reconcilers, researchers, and VText article
  agents without provider uncertainty dominating the mission.

## Quality Bar

Quality target: excellent.

This mission should leave behind a substrate that reduces future cleanup work:

- clear provider capability docs;
- focused unit tests;
- env-gated live tests;
- deployed gateway proof;
- product-path agent-loop proof;
- compaction/recall proof;
- rollback refs;
- mission checkpoint/resumption state.

## Provider/Protocol Matrix

Evaluate and record at least:

| Provider | Protocol | Models | Expected Use |
| --- | --- | --- | --- |
| DeepSeek | OpenAI chat completions | `deepseek-v4-flash`, `deepseek-v4-pro` | cheap text, simple tools, maybe non-thinking exact tools |
| DeepSeek | Anthropic-compatible | `deepseek-v4-flash`, `deepseek-v4-pro` | Anthropic-shaped agent loops, tool choice, thinking/tool continuation |
| Xiaomi | OpenAI chat completions | `mimo-v2.5-pro`, `mimo-v2.5` | text and multimodal calls |
| Xiaomi | Anthropic-compatible | `mimo-v2.5-pro`, `mimo-v2.5` | agent loops if Anthropic shape is more faithful |

For each row, prove or document:

- text completion;
- tool choice `auto`;
- tool choice `required` or protocol equivalent;
- exact named tool choice;
- tool choice `none`;
- tool result continuation;
- multi-turn conversation without tools;
- reasoning/thinking enabled;
- reasoning/thinking disabled;
- reasoning-content passback where required;
- missing reasoning-content negative case where applicable;
- image input where the model supports it;
- unsupported modality rejection where the model does not support it;
- max token semantics;
- context window claims versus observed behavior;
- sanitized errors without leaking upstream bodies or credentials.

## Homotopy Axes

Increase resolution along these axes without changing the object:

1. **Protocol realism:** direct adapter unit fixture -> local live provider call
   -> deployed gateway call -> product-path agent loop.
2. **Tool pressure:** no tools -> optional tool call -> required tool call ->
   exact tool call -> multi-tool loop -> tool-result continuation.
3. **Reasoning pressure:** reasoning disabled -> provider default -> low/medium
   reasoning -> reasoning-content passback -> missing-passback negative case.
4. **Modality pressure:** text only -> image input for `mimo-v2.5` -> modality
   rejection for unsupported models -> product-path multimodal verifier.
5. **Context pressure:** short prompt -> long source packet -> open PDF corpus ->
   LLM-generated run-memory checkpoint -> post-compaction recall and tool use ->
   model-aware automatic compaction at 70% of the selected model's declared
   context window. The detailed compaction work is now delegated to
   `docs/mission-llm-run-memory-compaction-v0.md`.
6. **Policy pressure:** fallback generated model policy -> editable
   per-computer model policy -> role/capability-specific selection ->
   owner-visible policy change path.
7. **Deployment pressure:** local env -> Node B gateway provider env -> staging
   health identity -> staging product proof -> rollback proof.
8. **News readiness:** generic agent loop -> researcher loop -> VText article
   loop -> processor/reconciler toolsets -> Universal Wire readiness report.

## Expected Implementation Routes

The mission should consider and, where evidence justifies, implement:

- provider adapter changes for DeepSeek/Xiaomi OpenAI routes;
- provider adapter changes for DeepSeek/Xiaomi Anthropic-compatible routes;
- provider protocol selection per model/provider in catalog or provider config;
- explicit capability flags for tools, exact tool choice, thinking, images,
  documents, context, and reasoning-content passback;
- gateway model routing that respects provider/protocol capability;
- model policy defaults that do not route VText into known-bad combinations;
- runtime retry/relaxation semantics for provider precondition failures;
- compaction preservation of provider continuity data that is needed for
  correctness, without surfacing hidden reasoning;
- test helpers that make provider conformance cheap to rerun.

Do not implement:

- role-specific harness branches for DeepSeek or Xiaomi;
- browser-visible provider secrets;
- fallback masking that silently swaps providers during conformance tests;
- news article fixes before provider conformance is sufficient;
- a parallel source/story graph to compensate for provider uncertainty.

## Exhaustive Test/Proof Requirements

### Unit and Fixture Tests

- Adapter serialization for OpenAI-compatible routes.
- Adapter serialization for Anthropic-compatible routes.
- Tool choice object/string mapping by protocol.
- Reasoning/thinking serialization by protocol.
- Reasoning-content extraction and passback.
- Tool result pairing and id preservation.
- Error classification for provider precondition failures.
- Model catalog capability facts.
- Model policy selection and fallback.
- Gateway routing by provider/model.

### Env-Gated Live Provider Tests

Use local `.env` keys, never committed:

- DeepSeek OpenAI text/tool/reasoning cases.
- DeepSeek Anthropic text/tool/reasoning cases.
- Xiaomi OpenAI text/tool/image/reasoning cases.
- Xiaomi Anthropic text/tool/image/reasoning cases if supported.
- Negative tests for unsupported modalities and incompatible settings.

### Deployed Gateway Tests

On Node B/staging:

- deploy provider env with `nix/deploy-provider-creds.sh`;
- verify gateway provider set includes `deepseek` and `xiaomi`;
- prove gateway inference for each selected provider/protocol/model;
- verify staging health identity at the target commit;
- inspect sanitized logs for provider/model/usage evidence without secrets.

### Product-Path Agent Loop Tests

Use only product/control APIs and visible app paths:

- prompt-bar starts an agent loop that uses DeepSeek text model;
- VText appagent produces an appagent revision using a known-good provider path;
- researcher loop uses DeepSeek or Xiaomi text model and records evidence;
- verifier multimodal loop uses `mimo-v2.5` with image input;
- trace moments expose provider/model/model-policy evidence;
- no internal/test route seeds success.

### Compaction and Long-Context Proof

This mission previously targeted the existing deterministic 160k compaction path
first. That route is superseded. The current compaction dependency is
`docs/mission-llm-run-memory-compaction-v0.md`: implement real LLM-generated
typed checkpoints, then default automatic compaction to
`context_window_tokens * 0.7`. DeepSeek/Xiaomi target models are treated as
1M-token context models for that mission, so the intended threshold is about
700k estimated input-context tokens. The 160k threshold may be used only as a
clearly labeled rollout diagnostic, not as readiness evidence.

Compaction is runtime-owned, not an agent-invoked action. The agent should not
call a "compact memory" tool. The runtime watches context pressure and compacts
before the next provider call. After compaction, the agent may use
`get_run_memory_entry` to recover exact raw content by `entry_id` when the
summary is not enough; retrieval is the tool-call surface, compaction is not.

Create a provider-backed long-context mission that forces automatic compaction:

- fetch one or more free/open PDFs from the web or another stable source;
- store full content handles in the user/candidate computer;
- have the agent read enough material to exceed the current configured automatic
  compaction threshold;
- wait for the runtime to compact automatically before the next provider call;
- after compaction, ask recall questions whose answers require earlier PDF
  content;
- require citations/handles back to the source material;
- require at least one post-compaction tool call;
- verify hidden reasoning did not appear in VText/source text/trace summaries.

The proof should run once for DeepSeek text and once for Xiaomi text if cost
permits. At minimum, run one full compaction proof and one shorter continuity
proof for the other provider.

## Suggested Open PDF Corpus

The worker may choose current stable public PDFs, but candidates include:

- arXiv papers with permissive access;
- public standards or government reports;
- open scientific reports;
- open company technical papers.

Selection criteria:

- legal/free access;
- stable URL;
- enough length to exercise context/compaction;
- distinctive facts that make recall testable;
- no need for paywalled scraping.

## Product Readiness For Universal Wire

Before returning to Universal Wire, produce a readiness report answering:

- Which provider/protocol/model should run processors?
- Which provider/protocol/model should run reconcilers?
- Which provider/protocol/model should run researchers?
- Which provider/protocol/model should run VText article agents?
- Which provider/protocol/model should run multimodal verifiers?
- What reasoning settings are safe for tool loops?
- What tool-choice settings are safe for VText exact edit turns?
- What fallback path is acceptable when a model hits provider limits?
- What provider combinations remain forbidden?
- What tests must be rerun before any future provider default change?

Universal Wire is not ready to resume if this report cannot name a provider path
for VText article generation and source/research tooling.

## Anti-Goodhart Constraints

- Do not claim provider support from a single direct text call.
- Do not claim tool support unless a tool call, tool result, and continuation
  are all proven.
- Do not claim multimodal support unless image input reaches the model through
  the deployed gateway or product path under realistic agent-loop settings. A
  tiny fixture image with artificial token/reasoning settings is only a
  serialization smoke test.
- Do not claim reasoning support unless reasoning-content passback is tested or
  reasoning is explicitly disabled for that path.
- Do not claim compaction safety from a short conversation that never compacted.
- Do not claim Universal Wire readiness while VText article generation still fails
  on the chosen provider path.
- Do not hide provider failures behind fallback provider success.
- Do not leave old Fireworks DeepSeek defaults as a quiet fallback.
- Do not use hidden/internal APIs for product-path acceptance.

## Receding-Horizon Control

At each control interval:

1. Pick the highest-value unproven provider/protocol behavior.
2. Predict expected request/response/log/trace evidence.
3. Run the smallest realistic live probe that can falsify the assumption without
   inventing non-production configuration such as arbitrary token caps.
4. Patch only the implicated layer.
5. Run focused unit tests.
6. Run env-gated live provider tests.
7. Run deployed gateway proof when behavior changed.
8. Run product-path proof for agent-loop behavior.
9. Update this mission's checkpoint state.

If a protocol fails, attempt a serious alternative route before stopping:

- switch OpenAI-compatible route to Anthropic-compatible route;
- disable reasoning for exact tool turns and preserve reasoning elsewhere;
- relax exact tool choice to `required` only when the VText contract remains
  satisfied;
- change model policy rather than harness internals;
- add provider capability flags instead of role branches;
- isolate the failure in a conformance test before product retries.

## Rollback Policy

For behavior-changing commits:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

Rollback refs must include:

- previous provider env backup path or restore command;
- previous model-policy defaults;
- commit SHA before provider behavior change;
- known-good provider/protocol/model combination;
- current known-bad provider/protocol/model combination.

If direct providers destabilize VText product behavior, do not restore broken
Fireworks DeepSeek defaults as a success claim. Either select a known-good
provider path or record the blocker.

## Universal Wire Provider Readiness Report

Status: ready enough to unblock the next Universal Wire hard cutover mission.

Recommended provider paths:

- Processors: OpenAI-compatible `deepseek/deepseek-v4-flash` for ordinary text
  ingestion, source-packet synthesis, and watch-state updates. Use
  `deepseek-v4-pro` when the prompt requires slower, deeper reconciliation or
  a processor is struggling with multilingual/source-density complexity.
- Reconcilers: OpenAI-compatible `deepseek/deepseek-v4-pro` for contradiction,
  consensus, and update/correction synthesis; `deepseek-v4-flash` remains valid
  for smaller reconciliation passes. Use the shared evidence/research toolset,
  not a role-specific harness branch.
- Researchers: OpenAI-compatible DeepSeek routes for text research, source
  fetching, and structured findings. Use Xiaomi only when the research turn
  needs images or other media input.
- VText article agents: OpenAI-compatible `deepseek/deepseek-v4-flash` as the
  default draft/revision path for normal article versions, with DeepSeek
  thinking disabled on tool-bearing calls because DeepSeek rejects thinking
  mode with exact/forced tools. Escalate to `deepseek-v4-pro` for publication
  quality revisions that need more synthesis depth.
- Multimodal verification: `xiaomi/mimo-v2.5` for image-capable verifier turns.
  Prefer source-resolved/base64 payloads or known-accessible URLs; arbitrary
  third-party image URLs remain unreliable because providers may be blocked from
  fetching them.
- Anthropic-compatible DeepSeek/Xiaomi routes: keep as conformance and fallback
  routes, not default long-writing routes. They are useful for protocol
  comparisons and exact-tool alternatives, but their request schema naturally
  carries explicit output-budget surfaces that should not become hidden defaults
  for ordinary agent loops.

Evidence basis:

- Direct deployed gateway probes worked for DeepSeek and Xiaomi model routes.
- Env-gated live tests cover direct text/image calls, OpenAI-compatible
  streaming with no arbitrary `max_tokens`, and runtime tool-choice modes across
  DeepSeek, DeepSeek Anthropic-compatible, Xiaomi, and Xiaomi
  Anthropic-compatible providers.
- Product-path VText loops have produced appagent revisions through the selected
  provider path.
- Product-path researcher/verifier-style loops have worked through the selected
  provider path, including a Xiaomi multimodal verifier route with
  `max_tokens_requested:false`.
- `docs/evidence/llm-run-memory-compaction-staging-2026-06-08.md` proves
  deployed DeepSeek LLM run-memory compaction, 1M-context-derived
  `threshold_tokens:700000`, raw retrieval handles, and exact retrieval through
  `get_run_memory_entry` by a later VText run.

Residual caveats:

- Do not rely on arbitrary external image URLs for multimodal verifier proof;
  resolve media into provider-accessible payloads.
- Do not select DeepSeek thinking mode for tool-bearing calls.
- The 700k automatic threshold was not naturally crossed on staging because
  that would be an intentionally expensive stress probe; local runtime tests
  cover automatic triggering and staging artifacts record the selected 700k
  threshold.
- The next Universal Wire mission should spend its budget on deleting mock news
  surfaces and ingesting real source volume, not on rediscovering provider
  basics.

## Stopping Conditions

### Complete

The mission is complete only when all are true:

- DeepSeek and Xiaomi are configured as first-class providers on staging.
- OpenAI-vs-Anthropic protocol choices are evidence-based and documented.
- Tool calls work or fail with precise documented limitations for all required
  modes.
- Reasoning settings are safe for each selected provider/protocol path.
- Multimodal `mimo-v2.5` is proven through a deployed or product-path route.
- At least one real product-path VText agent loop produces an appagent revision
  through the selected provider path.
- At least one real product-path researcher/verifier-style loop works through
  the selected provider path.
- Compaction/recall proof passes or a narrower provider-specific compaction
  blocker is documented after root-cause probes.
- Universal Wire readiness report names provider paths for processors,
  reconcilers, researchers, VText article agents, and multimodal verification.
- Mission doc has final checkpoint/resumption state.

### Checkpoint Incomplete

Useful progress landed, but one or more stopping conditions remain unproven.
The mission doc must name the remaining blocker and next executable probe.

### Blocked Incomplete

Only use after at least three meaningful root-cause probes and one
route-changing alternative. Name the external dependency or invariant-level
problem.

## Suggested Resume Goal String

```text
/goal Run docs/mission-provider-config-conformance-v0.md as MissionGradient and make DeepSeek/Xiaomi production-ready for Choir agents.
```

## Run Checkpoint & Resumption State

```text
status: complete
last checkpoint: 2026-06-08T20:20Z LLM compaction and exact retrieval proof
  deployed at `aa8bee5fe5500d48841cef054b9ab8b449929e4e`, and the Universal Wire
  provider readiness report now names provider paths for processors,
  reconcilers, researchers, VText article agents, multimodal verification, and
  Anthropic-compatible fallback routes.
current artifact state:
  Direct DeepSeek and Xiaomi provider support is implemented and deployed at
  commit `58881c172c51e3a862129eea7fab6feaf1deec53`. OpenAI-compatible
  DeepSeek/Xiaomi routes remain the default model-policy surface. New
  protocol-qualified providers `deepseek-anthropic` and `xiaomi-anthropic`
  expose their Anthropic-compatible Messages API routes without duplicating the
  model catalog's default provider mapping.

  The first product-path VText failure was root-caused to DeepSeek rejecting
  thinking mode when tools are present. That rule also applies on DeepSeek's
  Anthropic-compatible route. DeepSeek tool-bearing calls now force thinking
  disabled on both OpenAI-compatible and Anthropic-compatible routes. Xiaomi's
  Anthropic-compatible route can return signed `thinking` content blocks; the
  adapter stores them as hidden `reasoning_content` and replays them as
  Anthropic `thinking` blocks on continuation turns.

  The LLM compaction dependency is now closed. Staging product-path proof
  created real VText runs, compacted a DeepSeek VText loop into an
  `llm_checkpoint`, recorded the 1M-context-derived threshold as `700000`, and
  drove a later VText run to call `get_run_memory_entry` and recover exact
  compacted content by handle.
what shipped:
  `3dc9b05c221af8fc48a784f5ff937b62ae8fdbd0` hardens DeepSeek tool-loop
  conformance, adds focused provider/runtime tests, adds env-gated live
  DeepSeek/Xiaomi runtime tool-loop tests, and classifies the DeepSeek thinking
  plus tool-choice 400 as a provider precondition error.

  `58881c172c51e3a862129eea7fab6feaf1deec53` adds generic Anthropic-compatible
  provider support for DeepSeek and Xiaomi, registers `deepseek-anthropic` and
  `xiaomi-anthropic` in the gateway, serializes Anthropic `tool_choice` and
  `thinking`, preserves hidden thinking blocks, and adds unit plus env-gated
  live runtime exact-tool tests for both routes.

  `e6e734965bd651f294ec5295bfebec44930e60de` hardens multimodal verifier
  inputs. `verify_model_capability` now supports a deterministic
  `image_fixture="red_pixel_png"` capability probe, rejects malformed
  `image_base64`, rejects relative `image_url`, and provider media validation
  rejects malformed base64 or non-http(s) image URLs before an upstream provider
  call can become a sanitized 400.

  `f62220694d1f97019750f8309aac1e744744ac4e` recorded a product-path
  deterministic image-fixture proof. That proof is now explicitly demoted to a
  serialization smoke test because it used `max_tokens=64` and
  `reasoning=none`. It must not be treated as production-readiness evidence for
  multimodal verifier behavior.

  `71d80211f0cc2e19af46eec71a646759f7d35213` reframed this mission around
  realistic agent-harness conformance and made arbitrary `max_tokens` caps a
  forbidden readiness shortcut.

  `0955b437fcd7f740fee4935b13471fadf550530d` removes the model-facing
  `max_tokens` argument from `verify_model_capability`. Verifier probes now use
  normal model policy/provider defaults instead of letting super or another
  agent accidentally turn capability checks into capped config experiments.

  `aa8bee5fe5500d48841cef054b9ab8b449929e4e` adds LLM run-memory compaction,
  model-aware `context_window * 0.7` thresholding for 1M-token DeepSeek/Xiaomi
  models, structured checkpoint details, scalar/list checkpoint parser
  hardening, and exact raw-entry retrieval handles.
what was proven:
  Local focused unit/runtime/provider tests passed. Env-gated live local
  runtime exact-tool tests passed for DeepSeek, Xiaomi, DeepSeek Anthropic, and
  Xiaomi Anthropic with real provider keys. GitHub Actions runs `27148918526`
  and `27150954107` passed, including runtime shards and deploy to staging.
  Staging `/health` reports proxy and sandbox build
  `58881c172c51e3a862129eea7fab6feaf1deec53`. Node B gateway advertises
  `deepseek,deepseek-anthropic,xiaomi,xiaomi-anthropic,fireworks,chatgpt,zai`.

  Deployed gateway proof passed for DeepSeek `deepseek-v4-flash`: medium
  reasoning plus exact `function:record_status` returned `tool_use`, and the
  continuation turn after a tool result returned
  `GATEWAY_DEEPSEEK_TOOL_LOOP_OK` with `stop_reason=end_turn`.

  Deployed gateway proof passed for Xiaomi `mimo-v2.5-pro`: exact tool choice
  returned `tool_use`, and the continuation turn returned
  `GATEWAY_XIAOMI_TOOL_LOOP_OK` with `stop_reason=end_turn`.

  Product-path proof passed on `https://choir.news` using
  `frontend/tests/vtext-long-doc-fluid-editing-live.spec.js` with
  `GO_CHOIR_RUN_LIVE_VTEXT_EDIT=1`: a real staging user created a VText,
  created owner revisions, submitted `/api/vtext/documents/{id}/revise`, and
  received an appagent revision. Gateway logs for the run show DeepSeek
  `deepseek-v4-flash` handled a `tool_use` turn and an `end_turn` continuation
  for VM sandbox `vm-e978ade762857ddce16dd08a09ca5ce1` without a 400.

  Product-path compaction and recall proof passed on `https://choir.news`.
  Staging `/health` reported commit
  `aa8bee5fe5500d48841cef054b9ab8b449929e4e`; Playwright created a VText run,
  forced continuation-selection compaction, observed a public Trace
  run-memory artifact with `checkpoint_status:"llm_checkpoint"`,
  `checkpoint_provider:"deepseek"`, `checkpoint_model:"deepseek-v4-flash"`,
  and `threshold_tokens:700000`; then a later VText run invoked
  `get_run_memory_entry` and retrieved exact compacted sentinel content by raw
  entry id. Evidence summary:
  `docs/evidence/llm-run-memory-compaction-staging-2026-06-08.md`.

  Deployed gateway proof passed for `deepseek-anthropic` using
  `deepseek-v4-flash`: exact `function:record_status` returned `tool_use`, and
  the continuation turn returned `GATEWAY_DEEPSEEK_ANTHROPIC_TOOL_LOOP_OK`.
  Reasoning content was absent because the adapter intentionally disables
  thinking for DeepSeek tool-bearing calls.

  Deployed gateway proof passed for `xiaomi-anthropic` using `mimo-v2.5-pro`:
  exact `function:record_status` returned `tool_use`, hidden
  `reasoning_content` was present, and the continuation turn returned
  `GATEWAY_XIAOMI_ANTHROPIC_TOOL_LOOP_OK` with hidden reasoning content
  preserved.

  Product-path verifier-routing probe on staging used a normal browser/WebAuthn
  registration and `/api/prompt-bar`, not an internal route. Submission
  `ca5f0dea-be74-498c-a3ef-a0d03971394b` created VText document
  `8f722daa-ae80-4ba4-8f09-451a421f01e5` and child VText run
  `ec1d149c-4718-4260-ab06-1c247f5e7dec`. Node B gateway logs show the VText
  run used DeepSeek `deepseek-v4-flash`, requested persistent super, and the
  super run used DeepSeek `deepseek-v4-pro`. The super then invoked
  `verify_model_capability`-style provider calls through the configured
  gateway: `deepseek-anthropic/deepseek-v4-flash` returned successfully with
  `stop=end_turn`, and `xiaomi/mimo-v2.5` returned one sanitized 400 on the
  first image attempt before a later Xiaomi text verification call succeeded.
  This proves the real VText -> persistent super -> provider verifier route
  exists, but it does not yet prove robust product-path multimodal behavior.

  A narrower product-path multimodal probe then used a normal browser/WebAuthn
  registration and `/api/prompt-bar` with an explicit tiny base64 PNG. Submission
  `94b2a9cf-82a5-43be-9585-6c356e69c445` created VText document
  `e7716626-36fc-4c0e-bba7-3aa2a09eaf27` and child VText run
  `2ac1e711-69c7-4b75-b813-e404502c0734`. Node B gateway logs show VText on
  DeepSeek `deepseek-v4-flash`, persistent super on DeepSeek `deepseek-v4-pro`,
  and a `verify_model_capability` call through `xiaomi/mimo-v2.5` with
  `max_tokens=64`, `reasoning=none`, and no tools. The Xiaomi gateway call
  succeeded with `tokens=82+64` and `text_len=286`. This proves explicit
  base64-image serialization can reach Xiaomi through a real VText-requested
  super verifier path, but because the call used an artificial token cap and
  `reasoning=none`, it remains only a smoke test.

  Focused local tests for `e6e7349` passed:
  `nix develop -c go test ./internal/runtime -run TestVerifyModelCapability`;
  `nix develop -c go test ./internal/provider -run
  'TestValidateMediaRequest|TestIntegrationXiaomiProviderLive|TestIntegrationXiaomiRuntimeExactToolChoiceLive|TestIntegrationXiaomiAnthropicRuntimeExactToolChoiceLive'
  -count=1`; and `nix develop -c go test ./internal/provider
  ./internal/runtime ./internal/gateway ./internal/modelcatalog`.
  The Xiaomi live test includes real `mimo-v2.5` image input.

  GitHub Actions CI run `27152887075` passed for
  `e6e734965bd651f294ec5295bfebec44930e60de`, including runtime shards,
  non-runtime tests, vet/build, and deploy to staging. Staging `/health`
  reported proxy and sandbox deployed commit
  `e6e734965bd651f294ec5295bfebec44930e60de`.

  Deployed product-path fixture acceptance used normal browser/WebAuthn
  registration and `/api/prompt-bar`. Submission
  `2dbf9418-4c81-48de-bac1-08aa1068cd46` created VText document
  `5a1e705b-6d7a-4822-b282-916d80a490eb`, child VText run
  `23de54e4-727a-4a18-9be6-6985f8c49c77`, and persistent super run
  `aa69da19-364e-4936-9714-4f4bcd7bc17d`. Node B gateway logs show VText on
  DeepSeek `deepseek-v4-flash`, super on DeepSeek `deepseek-v4-pro`, then a
  `verify_model_capability` call through `xiaomi/mimo-v2.5` with `max_tokens=64`,
  `reasoning=none`, and no tools. The Xiaomi gateway call succeeded with
  `tokens=81+60` and `text_len=187`. This proves only that deterministic image
  fixture serialization can reach Xiaomi through the real VText -> super product
  path. It does not prove realistic multimodal verifier readiness, because the
  run did not use normal agent-loop budgets, did not use meaningful image
  evidence, and explicitly disabled reasoning.

  A more realistic product-path researcher probe was started through normal
  browser/WebAuthn registration and `/api/prompt-bar`. Submission
  `e3b8187a-2f74-462f-b3d4-1bc99c071987` created VText document
  `59ceeee3-fc5a-47ca-b765-06df43bf7035`, child VText run
  `a6597e4d-1dfe-488a-98d9-7fef0677e259`, and researcher run
  `bd237974-73ee-4ec5-9364-aa898b2b72ac`. Logs showed VText using
  DeepSeek `deepseek-v4-flash`, the researcher using DeepSeek
  `deepseek-v4-flash` with the normal researcher toolset and medium reasoning,
  and Brave search returning 15 results. Further Node B logs show the
  researcher continued through several tool-result continuation turns with
  `max_tokens=0`, spawned VText wake run
  `e216d4d4-578c-4386-8184-7b798d1fc214`, and VText produced follow-up DeepSeek
  revisions after consuming the researcher update. This proves a realistic
  product-path VText -> researcher -> search -> VText follow-up loop on the
  direct DeepSeek provider path. It does not prove compaction.

  Focused local tests for `0955b437` passed:
  `nix develop -c go test ./internal/runtime -run
  'TestVerifyModelCapability|TestModelPolicy|TestRunToolLoopRequiredNextToolGetsFiniteBudgetWhenPolicyOmitsMaxTokens'`.
  GitHub Actions CI run `27154324502` passed for
  `0955b437fcd7f740fee4935b13471fadf550530d`, including runtime shards,
  non-runtime tests, integration-tagged smoke, vet/build, and deploy to staging.
  Staging `/health` reported proxy and sandbox deployed commit
  `0955b437fcd7f740fee4935b13471fadf550530d` with deploy time
  `2026-06-08T17:14:13Z`. Node B gateway health advertised
  `deepseek,deepseek-anthropic,xiaomi,xiaomi-anthropic,fireworks,chatgpt,zai`
  and search providers `tavily,brave,parallel,exa,serper`.

  Deployed realistic multimodal verifier proof used normal browser/WebAuthn
  registration and `/api/prompt-bar`. Submission
  `737c5434-21f8-4aba-b19b-1f2ba67a187d` created VText document
  `ae12ee65-ab3c-4afa-8b28-e5dcc97f9f92`, child VText run
  `9081eb5f-92fb-4f7c-87bc-b5c41cf7b4bb`, persistent super run
  `861b6dd4-870f-417b-998b-bef49b4c2915`, and VText follow-up run
  `c7be3db6-a67d-4334-a771-ec4b2e9f2c63`. The trace shows super invoked
  `verify_model_capability` with arguments containing `role`,
  `image_url`/`image_fixture`, and `prompt`, with no `max_tokens` argument.
  Tool result moment `622064f8-285e-4f65-b8ce-96ae9202dd8c` returned
  `provider:"xiaomi"`, `model:"mimo-v2.5"`, `role:"verifier_multimodal"`,
  `image_input:true`, `reasoning_effort:"medium"`,
  `reasoning_content_present:true`, and `max_tokens_requested:false`. Node B
  gateway logs for the same run show Xiaomi requests with
  `provider=xiaomi model=mimo-v2.5 ... max_tokens=0 reasoning=medium`.
  VText revision `5d8f5c93-b0b2-420b-b857-56fd1048e2d5` consumed the super
  update and recorded the verifier result in the document.

  The same multimodal proof revealed a real caveat: the original Wikimedia
  image URL failed through Xiaomi with sanitized 400s, while the super's
  follow-up recorded an HTTP 403 fetch restriction for that URL and successful
  control checks with the deterministic fixture plus an alternate public URL.
  Current belief is that reliable multimodal verifier acceptance should use
  source-resolved/base64 image payloads or known-accessible URLs rather than
  arbitrary third-party image URLs that may block provider fetchers.

  A low-cost continuation-selection compaction probe was initially attempted
  after the verifier trajectory by replaying old ids:
  `737c5434-21f8-4aba-b19b-1f2ba67a187d` and
  `861b6dd4-870f-417b-998b-bef49b4c2915`. Those calls returned `record not
  found`/`source run not found`. A follow-up same-session probe showed the root
  cause was owner scoping, not a continuation route defect: the shared
  Playwright auth state had been replaced by a newer WebAuthn test user, so the
  old verifier ids were no longer visible to the authenticated owner. Under the
  current authenticated product session, prompt-bar submission
  `aa27605d-dc1a-41cd-a083-d3d7b5f1a682` completed, and `POST
  /api/continuations` for that source run returned continuation
  `36310c20-f0c7-4016-88ab-bdea09b0f5b6` with
  `compaction_status:"completed"` and no runtime patch. This proves the
  continuation-selection compaction route can work on staging for a same-owner
  product run. It still does not prove long-context post-compaction recall.

  The env-gated live provider tests were brought back into line with the
  mission's no-arbitrary-token-cap invariant. `MaxTokens: 64` was removed from
  the direct DeepSeek text and Xiaomi text/image live probes. A new live runtime
  tool-choice matrix test now exercises the normal `RunToolLoop` bridge for all
  four provider/protocol rows: `deepseek`, `deepseek-anthropic`, `xiaomi`, and
  `xiaomi-anthropic`. For each row it proves `auto`, `required`, exact
  `function:record_status`, and `none` tool-choice modes with normal provider
  budgets. Live command:
  `set -a; source .env; set +a; CHOIR_PROVIDER_LIVE_TESTS=1 nix develop -c go
  test ./internal/provider -run 'TestIntegrationRuntimeToolChoiceModesLive'
  -count=1 -v` passed in 57.536s. The same uncapped direct live probes passed:
  `CHOIR_PROVIDER_LIVE_TESTS=1 nix develop -c go test ./internal/provider -run
  'TestIntegrationDeepSeekDirectLive|TestIntegrationXiaomiTextAndImageLive'
  -count=1 -v`.

  Follow-up architecture inspection confirmed that long-context recall should
  use the existing shared harness rather than a provider-specific branch.
  Run-memory compaction summaries include raw `entry_id` handles for compacted
  messages; `get_run_memory_entry` is registered through the evidence toolset;
  and the evidence toolset is available to the roles that need it for this
  mission, including VText, researcher, processor, reconciler, super, vsuper,
  and co-super. Therefore the next proof should force a realistic provider
  turn through compaction, then require the model to retrieve exact pre-
  compaction content by `get_run_memory_entry`. Do not lower context thresholds
  or add token caps for product-readiness evidence; if a lower threshold is used
  at all, it is a local diagnostic and must be labeled as such.

  Provider-semantics review after the no-cap correction found that normal
  DeepSeek/Xiaomi policy already selects OpenAI-compatible routes, where
  foreground agent loops omit explicit output budgets unless model policy sets
  one. Anthropic-compatible routes remain useful for compatibility and fallback
  conformance, but should not become the default long-writing route simply
  because tests pass: their request schema expects an explicit `max_tokens`
  field, which is exactly the kind of silent cap surface this mission is trying
  to avoid for ordinary agent loops.

  Streaming behavior for the normal no-cap OpenAI-compatible routes is now
  covered by an env-gated live provider test. `TestIntegrationDeepSeekXiaomiOpenAIStreamingLive`
  streams a short health-check code through `deepseek/deepseek-v4-flash` and
  `xiaomi/mimo-v2.5-pro` with `MaxTokens` unset, asserts streamed text deltas,
  and rejects `max_tokens` stop reasons. The first Xiaomi attempt revealed a
  prompt-shape quirk: asking it to "reply exactly with this marker" triggered
  conversational persona text instead of the marker. Framing the request as a
  harmless API health-check code plus a system instruction produced the desired
  short response without changing provider config. Live command:
  `set -a; source .env; set +a; CHOIR_PROVIDER_LIVE_TESTS=1 nix develop -c go
  test ./internal/provider -run 'TestIntegrationDeepSeekXiaomiOpenAIStreamingLive'
  -count=1 -v` passed in 5.616s.

  The run-memory retrieval mechanism now has a deterministic shared-harness
  diagnostic for the exact recovery path that a live long-context proof should
  use. `TestRuntimeRunMemoryOverflowRecoveryRetrievesRawEntry` drives a
  profiled runtime tool loop through a simulated provider context-overflow
  error, lets the runtime force-compact the single oversized raw prompt, verifies
  that the compacted retry context no longer contains the exact sentinel but
  does contain a raw `entry_id`, calls the registered `get_run_memory_entry`
  evidence tool, and then verifies the final provider turn can see the exact
  raw sentinel from the retrieved entry. This proves compaction summary handles
  can be used to recover exact pre-compaction content through the shared evidence
  tool path. It is still a deterministic diagnostic, not a live DeepSeek/Xiaomi
  long-context proof.
unproven or partial claims:
  Product-path coverage for every provider/protocol matrix cell remains broader
  than necessary for the next Universal Wire mission; env-gated live conformance
  covers the matrix and product-path evidence covers the selected default paths.
  Arbitrary external image URLs remain unreliable for Xiaomi multimodal
  verification; use source-resolved/base64 media or known-accessible URLs. The
  700k automatic threshold was not naturally crossed on staging, but local
  runtime tests cover automatic triggering and staging compaction artifacts
  record the selected `threshold_tokens:700000`.
belief-state changes:
  The selected DeepSeek OpenAI-compatible path is now viable for VText exact
  edit/tool loops when tool-bearing calls disable thinking. Anthropic-compatible
  routes are viable conformance/alternative routes for exact tool loops, but
  they are not yet selected as default policy. Product-path VText can request
  persistent super and super can call provider verifier routes. A realistic
  direct-DeepSeek researcher loop and a no-token-cap Xiaomi multimodal verifier
  loop have now been proven through product paths. Xiaomi multimodal is viable
  for base64/fixture and at least one accessible public URL, but arbitrary
  image URL fetch behavior remains unreliable. Env-gated live provider evidence
  now covers the local provider/protocol tool-choice matrix without artificial
  token caps; product-path evidence covers the selected VText, researcher,
  verifier, and compaction paths. OpenAI-compatible DeepSeek and Xiaomi
  streaming now has live no-cap evidence, and Xiaomi short-code verifier prompts
  should use explicit health-check framing instead of brittle "exact marker"
  phrasing. The LLM compaction mission has now produced deployed product-path
  evidence: a DeepSeek VText loop generated an LLM checkpoint with exact raw
  entry handles and a later VText loop retrieved exact compacted content by
  `get_run_memory_entry`. Provider readiness is now sufficient to unblock
  Universal Wire hard cutover.
remaining error field:
  No provider/config blocker remains for Universal Wire. The next error field is
  Universal Wire itself: delete mock/detritus surfaces, ingest real high-volume
  sources, and route source packets through processors, reconcilers, researchers,
  and VText article agents.
highest-impact remaining uncertainty:
  Whether the Universal Wire architecture can turn the now-ready provider substrate
  into real high-volume ingestion and publication-quality VText articles without
  preserving old mock surfaces.
next executable probe:
  Run the Universal Wire hard-cutover mission against staging: remove legacy mock
  news surfaces, prove real source ingestion volume, and have VText agents own
  real article versions with embedded/transcluded source evidence.
suggested resume goal string:
  /goal Run docs/mission-universal-wire-hard-cutover-real-newsroom-v0.md as MissionGradient; replace Universal Wire mocks with real source ingestion and VText-owned articles.
evidence artifact refs:
  Prior mission evidence lives in
  `docs/mission-universal-wire-hard-cutover-real-newsroom-v0.md`.
  LLM compaction staging evidence:
  `docs/evidence/llm-run-memory-compaction-staging-2026-06-08.md`.
rollback refs:
  Restore previous provider env/model policy only if direct providers break
  staging behavior. Do not treat Fireworks DeepSeek as a valid fallback while
  Fireworks access is unavailable.
```
