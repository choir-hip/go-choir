# Provider Config Conformance Mission v0

## Mission Identity

Build Choir's provider substrate until DeepSeek and Xiaomi can be trusted as
first-class runtime providers for real agent work, not just isolated text calls.

This is not a provider-adapter slice. It is the full provider configuration and
conformance object needed before returning to Global Wire:

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
before resuming Global Wire.

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

The provider substrate is "done" only when Global Wire can safely depend on it.
Direct curl success is insufficient.

Operational implication: stopping condition includes a real Choir run that uses
DeepSeek and Xiaomi together, plus an explicit readiness report for Global Wire
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
- Global Wire can launch processors, reconcilers, researchers, and VText article
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
   forced compaction -> post-compaction recall and tool use.
6. **Policy pressure:** fallback generated model policy -> editable
   per-computer model policy -> role/capability-specific selection ->
   owner-visible policy change path.
7. **Deployment pressure:** local env -> Node B gateway provider env -> staging
   health identity -> staging product proof -> rollback proof.
8. **News readiness:** generic agent loop -> researcher loop -> VText article
   loop -> processor/reconciler toolsets -> Global Wire readiness report.

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

Create a provider-backed long-context mission that forces compaction:

- fetch one or more free/open PDFs from the web or another stable source;
- store full content handles in the user/candidate computer;
- have the agent read enough material to exceed the configured compaction
  threshold;
- force or wait for compaction;
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

## Product Readiness For Global Wire

Before returning to Global Wire, produce a readiness report answering:

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

Global Wire is not ready to resume if this report cannot name a provider path
for VText article generation and source/research tooling.

## Anti-Goodhart Constraints

- Do not claim provider support from a single direct text call.
- Do not claim tool support unless a tool call, tool result, and continuation
  are all proven.
- Do not claim multimodal support unless image input reaches the model through
  the deployed gateway or product path.
- Do not claim reasoning support unless reasoning-content passback is tested or
  reasoning is explicitly disabled for that path.
- Do not claim compaction safety from a short conversation that never compacted.
- Do not claim Global Wire readiness while VText article generation still fails
  on the chosen provider path.
- Do not hide provider failures behind fallback provider success.
- Do not leave old Fireworks DeepSeek defaults as a quiet fallback.
- Do not use hidden/internal APIs for product-path acceptance.

## Receding-Horizon Control

At each control interval:

1. Pick the highest-value unproven provider/protocol behavior.
2. Predict expected request/response/log/trace evidence.
3. Run the smallest live probe that can falsify the assumption.
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
- Global Wire readiness report names provider paths for processors,
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
status: checkpoint_incomplete
last checkpoint: 2026-06-08T16:20Z Anthropic-compatible provider routes
  implemented, deployed, and proven for exact tool choice plus continuation
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
unproven or partial claims:
  Full auto/required/none tool-mode matrix, non-tool reasoning-content passback
  for DeepSeek Anthropic, streaming behavior for the new routes,
  product-path researcher/verifier runs, product-path multimodal verification,
  compaction/recall safety, and the final Global Wire provider readiness report
  remain unproven.
belief-state changes:
  The selected DeepSeek OpenAI-compatible path is now viable for VText exact
  edit/tool loops when tool-bearing calls disable thinking. Anthropic-compatible
  routes are viable conformance/alternative routes for exact tool loops, but
  they are not yet selected as default policy. Provider readiness is still not
  complete enough for Global Wire hard cutover until researcher, multimodal
  verifier, and compaction behavior are proven or precisely bounded.
remaining error field:
  Complete the provider/protocol conformance matrix beyond the repaired
  OpenAI-compatible tool loop, then select safe model-policy defaults for
  processors, reconcilers, researchers, VText article agents, and multimodal
  verifiers.
highest-impact remaining uncertainty:
  Whether the repaired OpenAI-compatible default path or the new
  Anthropic-compatible route should run long-horizon agent loops, and whether
  hidden reasoning/compaction continuity remains correct under long-context
  pressure.
next executable probe:
  Run one product-path researcher/verifier-style loop, then one long-context
  compaction proof with post-compaction recall and tool use. Extend the
  tool-mode matrix to auto/required/none after the product-path probes.
suggested resume goal string:
  /goal Run docs/mission-provider-config-conformance-v0.md as MissionGradient and make DeepSeek/Xiaomi production-ready for Choir agents.
evidence artifact refs:
  Prior mission evidence lives in
  `docs/mission-global-wire-hard-cutover-real-newsroom-v0.md`.
rollback refs:
  Restore previous provider env/model policy only if direct providers break
  staging behavior. Do not treat Fireworks DeepSeek as a valid fallback while
  Fireworks access is unavailable.
```
