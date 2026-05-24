# MissionGradient: Fireworks Reasoning Protocol Hardening v0

**Status:** checkpoint_incomplete
**Date:** 2026-05-24
**Depends on:** current deployed staging baseline, Fireworks provider credentials on Node B, current ChatGPT gateway credentials if used as a comparison path
**Related docs:** [platform-os-app-state.md](platform-os-app-state.md), [mission-runtime-model-context-substrate-v0.md](mission-runtime-model-context-substrate-v0.md)

## One-Line Goal String

```text
/goal Run docs/mission-fireworks-reasoning-protocol-hardening-v0.md as a Codex-operated MissionGradient mission: harden Choir's provider protocol, tool-loop behavior, and dynamic model policy so any configured model can safely serve any compatible agent role. Treat current ChatGPT/Fireworks assignments as editable defaults, not architecture: conductor, VText, researcher, super, vsuper, co-super, verifier, and future roles must all be selectable across ChatGPT, Fireworks DeepSeek V4 Flash/Pro, Fireworks Kimi K2.6, and later catalog models when the current turn's modality and tool needs match. Research current Fireworks OpenAI-compatible chat-completions docs, Fireworks reasoning docs, Fireworks DeepSeek V4 Flash/Pro behavior, Fireworks Kimi K2.6 behavior, Kimi K2.6 docs, DeepSeek V4 reasoning-content semantics as secondary context, and Choir's ChatGPT Responses adapter as a comparison path. Run a deep request-shape and role matrix for max_tokens omitted vs bounded vs model maximum, reasoning_effort omitted/none/low/medium/high/max, thinking/reasoning budget parameters where supported, streaming vs non-streaming, tool-calling vs plain chat, required-tool behavior for VText/appagents, multimodal image input where supported, text-only verification where image input is not needed, and reasoning_content/thinking carry-forward across multi-turn tool loops. Prove behavior first with direct provider probes, then local Choir provider/runtime/tool-loop harness loops, then Node B/staging product-path prompt-bar runs. Fix the provider/runtime/model-policy/context plumbing so simple prompts do not hang, VText cannot spend a whole call writing uncaptured prose instead of producing a revision/tool action, long outputs remain possible, provider calls have correct per-call deadlines, interim progress is visible in Trace/VText, unsupported provider parameters are omitted, text-only models can be selected for non-image roles and verifier tasks, multimodal requirements are enforced only when the task actually needs media input, and per-computer model policy can be changed dynamically and agentically through durable product state rather than Node B env edits. Do not restore tiny 8k/16k ceilings as a false fix, hard-code one model per role, treat DeepSeek text-only status as unusability for verification, pass OpenAI Responses parameters to OpenAI Chat Completions providers, drop required reasoning_content when a provider requires carry-forward, hide hangs behind longer loop deadlines, or claim success without staging evidence for conductor, VText, researcher, super/vsuper/co-super, verifier text-only, Kimi multimodal, and runtime policy-edit paths. Land through git/CI/deploy, verify staging identity, update docs/model policy notes, and finish with a provider/model-policy protocol certificate, rollback refs, residual risks, and the next executable probe back toward the Chyron/Motion/Liquid/Python experiment rerun.
```

## Mission Frame

Choir now has multiple usable model/provider families:

- ChatGPT subscription models, currently used or intended for some foreground
  roles such as conductor and super when credentials are healthy;
- Fireworks-hosted DeepSeek V4 Flash/Pro, which are text-only but should be
  usable for any role whose current turn does not require image/media input;
- Fireworks-hosted Kimi K2.6, which is multimodal and useful for verification
  that needs screenshots or other image inputs.

The current mapping is a default model policy, not a permanent architecture.
Choir should allow any configured model to serve any compatible role. Conductor,
VText, researcher, super, vsuper, co-super, verifier, and future roles are
runtime consumers of model capabilities, not hard-coded provider homes. The
selection should be editable at runtime through durable product state, including
agentic edits by `super` in response to an owner prompt, without patching Node B
environment variables or deploying a new platform build just to change a
computer's model policy.

After restoring full model output budgets, simple staging runs began hanging at
`vtext loop started`, and some runs failed with Fireworks context deadline or
`max_tokens` stop reasons. A direct one-off Fireworks call is not enough
evidence either way: the real risk is the complete protocol shape used by Choir
agent loops, including tool schemas, system prompts, multimodal blocks, retries,
streaming/non-streaming mode, reasoning fields, reasoning carry-forward, and
timeout layering.

This mission is not a generic model swap, and it is not a migration to
DeepSeek's own API. It is a provider protocol, tool-loop, and runtime
model-policy hardening mission. Fireworks-hosted DeepSeek V4 Flash, DeepSeek V4
Pro, and Kimi K2.6 are the immediate failing cases; ChatGPT remains a comparison
path and a valid selectable provider family when its credentials are healthy.
DeepSeek's own API docs are useful only as secondary context for
reasoning-content semantics.

## Core Invariant

```text
Provider protocol correctness and task-compatible model selection beat static role defaults.
```

The model catalog can record context windows, modalities, theoretical output
limits, tool-call support, reasoning controls, and provider features. Runtime
selection must then match the current task: text-only models are valid for
text-only verification, coding, research, writing, and orchestration;
multimodal models are required only when the turn needs image or media input.
The request shape must reflect what each provider/model actually supports in
multi-turn agent loops. A provider adapter must omit unsupported or harmful
parameters, preserve required state such as reasoning content when a provider
requires it, force tool use when an appagent cannot safely answer in plain text,
and expose progress before long calls.

## Real Artifact

The artifact is a deployed provider/runtime substrate where:

- simple prompt-bar tasks complete quickly and visibly;
- VText produces the human-readable narrative revision through its revision
  tool path instead of hanging or emitting uncaptured plain prose;
- researcher routes can call web/search/fetch tools and report findings;
- super/vsuper/co-super can run tool loops without protocol errors;
- conductor, VText, researcher, super, vsuper, co-super, verifier, and future
  roles can be switched among configured compatible models without code edits;
- DeepSeek V4 Flash/Pro can be used for verifier tasks that do not require
  image input;
- Kimi K2.6 can process screenshots/images through the declared multimodal
  path;
- multimodal requirements are declared and enforced at the task/turn boundary,
  not by permanently locking verifier roles to one model;
- per-computer model policy is durable product state and can be edited by the
  owner or by `super` through the product path;
- long output remains possible without making every trivial call reserve the
  model's maximum token budget or letting appagents burn the whole call outside
  their required tool/action path;
- Trace shows where a run is during provider calls, retries, tool calls, and
  handoffs;
- docs explain provider-specific semantics and the runtime model-policy
  hierarchy.

## Source Seeds

Use current official docs and record exact citations in the final evidence:

- Fireworks Chat Completions API:
  [docs.fireworks.ai/api-reference/post-chatcompletions](https://docs.fireworks.ai/api-reference/post-chatcompletions)
- Fireworks reasoning guide:
  [docs.fireworks.ai/guides/reasoning](https://docs.fireworks.ai/guides/reasoning)
- Fireworks documentation index:
  [docs.fireworks.ai/llms.txt](https://docs.fireworks.ai/llms.txt)
- Kimi API platform introduction:
  [platform.kimi.ai/docs/introduction](https://platform.kimi.ai/docs/introduction)
- Kimi documentation index:
  [platform.kimi.ai/docs/llms.txt](https://platform.kimi.ai/docs/llms.txt)
- DeepSeek API docs for secondary reasoning-content context:
  [api-docs.deepseek.com/api/create-chat-completion](https://api-docs.deepseek.com/api/create-chat-completion)

Also inspect provider-specific behavior through actual requests. Documentation
is the map; observed behavior is the terrain.

## Belief State To Test

Current hypotheses, to be falsified or confirmed:

1. Explicit huge `max_tokens` may be benign for trivial direct calls but harmful
   in rich agent/tool-loop request shapes.
2. Fireworks-hosted DeepSeek V4 Flash/Pro may support `reasoning_effort`, but
   the accepted values and default behavior must be proven against Fireworks'
   API, not inferred from older DeepSeek `-chat` or `-reasoner` naming.
3. Kimi K2.6 and Fireworks-hosted DeepSeek V4 modes may produce
   `reasoning_content` or analogous thinking fields that must be either safely
   ignored, explicitly excluded from history, or passed back according to the
   actual Fireworks model behavior.
4. Choir currently treats model maximum output as the per-call request budget;
   this may be the wrong abstraction. Model capability, requested answer budget,
   reasoning budget, and loop deadline are separate concepts.
5. Non-streaming calls hide progress. Even when non-streaming remains supported,
   Trace should receive a pre-provider-call event and timeout/retry evidence.
6. A five- or ten-minute timeout may be reasonable for one slow generation, but
   it must not become a silent whole-loop freeze with no progress evidence.
7. Static role defaults are too rigid. The safer target is a durable,
   dynamically editable policy that can select any configured compatible model
   for a role or task.
8. The verifier role is not inherently multimodal. Some verification requires
   images; much verification is text/code/evidence inspection and should be able
   to use text-only models.
9. VText failures that show `stop=max_tokens` may be caused by missing
   required-tool semantics rather than insufficient output budget: a model can
   spend the whole provider call drafting prose that never becomes a VText
   revision.

## Experiment Matrix

Run the matrix at three layers:

1. **Direct API probes** from local or Node B shell, without printing secrets.
2. **Local Choir harness probes** through provider and runtime/tool-loop tests.
3. **Deployed staging probes** through the visible product path.

### Models

- ChatGPT configured model(s), when credentials are healthy.
- `accounts/fireworks/models/deepseek-v4-flash`
- `accounts/fireworks/models/deepseek-v4-pro`
- `accounts/fireworks/models/kimi-k2p6`

Remove or keep removed stale `kimi-k2p5-turbo` paths. It must not appear in
runtime defaults, tests, or product model selectors unless explicitly marked as
historical.

### Request Parameters

For each relevant model, test:

- `max_tokens` omitted;
- modest explicit output budget, for example `4096`;
- long explicit output budget, for example `32768`;
- catalog maximum where supported, for example `131072`;
- provider-declared max if docs conflict with catalog;
- `reasoning_effort` omitted;
- `reasoning_effort: none`;
- `reasoning_effort: low`;
- `reasoning_effort: medium`;
- `reasoning_effort: high`;
- `reasoning_effort: max`;
- provider-specific thinking/reasoning budget fields where documented;
- top-p/top-k/temperature omitted versus explicitly set only where needed.

### Conversation Shapes

For each model/parameter family, test:

- plain single-turn chat;
- two-turn chat where prior assistant reasoning/thinking fields may appear;
- tool-calling request with one simple tool;
- required-tool request with one simple tool;
- multi-turn tool loop with tool result returned to the model;
- VText-like system prompt with edit tool;
- VText-like system prompt that proves the model calls a revision/tool action
  rather than returning plain assistant prose;
- researcher-like tool use;
- super/vsuper/co-super-like tool use;
- Kimi multimodal screenshot/image request;
- text-only verifier task using DeepSeek V4 Flash/Pro;
- attempted image-input verifier task using DeepSeek V4 Flash/Pro, which should
  fail before provider call with a clear modality blocker or reroute to a
  multimodal model;
- cancellation and timeout behavior.

### Role Selection Shapes

For each provider/model family that supports the required modality, test:

- conductor;
- VText;
- researcher;
- super;
- vsuper;
- co-super;
- verifier text-only;
- verifier multimodal where supported.

## Runtime Design Questions

Answer with evidence before patching broadly:

- Should Choir omit `max_tokens` by default for Fireworks chat completions and
  use explicit budgets only for bounded tasks?
- If explicit budgets are needed, should the runtime distinguish:
  `model_max_output_tokens`, `requested_output_budget`,
  `reasoning_budget_tokens`, and `loop_budget`?
- Should Fireworks calls default to streaming so progress and tool calls are
  observable earlier?
- Which appagent turns need `tool_choice: required` or a provider-neutral
  equivalent so plain assistant text cannot masquerade as progress?
- Which reasoning fields are Fireworks provider inputs versus provider outputs
  for the exact DeepSeek V4 Flash/Pro and Kimi K2.6 model IDs?
- If Fireworks returns `reasoning_content`, should Choir store it, pass it
  back, redact it, or discard it? Does the answer differ for Fireworks-hosted
  DeepSeek V4 Flash/Pro and Kimi K2.6?
- What is the right timeout hierarchy:
  provider connect timeout, first-byte timeout, single model-call deadline,
  tool-call deadline, and whole-loop deadline?
- Which errors should cause retry, fallback to another model, or immediate
  blocker evidence?
- What is the durable model-policy hierarchy: platform catalog, platform
  defaults, per-computer policy file/state, per-run override, task/turn
  modality requirement, and agentic policy edits?
- How should `super` safely edit a user's computer model policy through VText or
  product APIs without direct Node B config mutation?
- What UI or API evidence should show the effective model selected for a run
  without exposing secrets or overwhelming ordinary users?

## Homotopy Axes

Increase realism while preserving the same object:

1. Direct single-call probe.
2. Direct tool-call probe.
3. Local provider unit/integration probe.
4. Local runtime tool-loop probe.
5. Local or staging prompt-bar probe with Trace/VText evidence.
6. Full role matrix: conductor, VText, researcher, super, vsuper, co-super,
   verifier text-only, and verifier multimodal.
7. Runtime policy edit: owner or `super` changes a computer's model selection,
   then a subsequent run uses the new effective policy without platform deploy.

Do not skip from direct API success to product success. Direct API success only
proves credentials and a request shape, not Choir's harness.

## Dense Feedback

Add or preserve observability so a hung run is diagnosable:

- emit a `loop.progress` or dedicated provider-call event before each model
  call with provider, model, iteration, message count, tool count, output
  budget, reasoning mode, stream mode, and timeout class;
- emit completion/error events with elapsed time and sanitized error class;
- include provider request-shape summaries in Trace without exposing prompts,
  secrets, or private content unnecessarily;
- keep VText as human narrative, not raw provider telemetry.

## Forbidden Shortcuts

- Do not solve hangs by merely raising the whole-loop timeout.
- Do not restore small universal `8k` or `16k` output ceilings as the main fix.
- Do not pass OpenAI Responses-only fields to OpenAI Chat Completions providers.
- Do not hard-code one model family per role as the long-term architecture.
- Do not treat the current conductor/super defaults as proof that only ChatGPT
  can serve foreground roles, or the current VText/researcher defaults as proof
  that only Fireworks can serve document/research roles.
- Do not let VText or another canonical-artifact appagent answer in plain text
  when the product state requires a tool-mediated revision/action.
- Do not treat text-only models as invalid for verifier work that does not need
  media input.
- Do not mutate Node B environment variables as a substitute for per-computer
  runtime model policy.
- Do not assume Fireworks-hosted Kimi K2.6 and Fireworks-hosted DeepSeek V4
  Flash/Pro have identical reasoning semantics.
- Do not pass private chain-of-thought to product users.
- Do not leak API keys or raw provider credentials in logs, Trace, tests, or
  mission evidence.
- Do not claim local direct API success proves staging prompt-bar behavior.
- Do not edit Node B tracked files directly as a config shortcut.

## Acceptance Contracts

### Documentation Contract

- Current Fireworks docs are reviewed and cited, with Kimi and DeepSeek docs
  used as secondary context where they explain model-level semantics.
- The final report records observed behavior where docs are ambiguous.
- Runtime model policy docs explain the difference between model capability,
  requested output budget, reasoning budget, and loop deadline.
- Runtime model policy docs explain the hierarchy of platform model catalog,
  platform defaults, per-computer policy, per-run/task override, and modality
  requirement.

### Direct API Contract

- All three current Fireworks models have direct probe results.
- Matrix rows record status, latency, stop reason, output token count, and any
  reasoning/thinking fields.
- Failures are classified by request shape, not summarized as "Fireworks bad".

### Harness Contract

- Provider adapter tests cover omitted versus explicit `max_tokens`.
- Reasoning parameter behavior is tested for supported and unsupported values.
- Tool-loop tests prove pre-provider progress evidence exists before long calls.
- Multi-turn tests cover reasoning/thinking carry-forward or deliberate discard.
- Model policy tests prove compatible text-only models can serve text-only
  verifier work, while image-input tasks require a multimodal model or clear
  blocker before provider call.
- Runtime policy tests prove per-computer policy can change effective model
  selection without code edits.
- Tool-choice/tool-requirement tests prove appagents that must mutate canonical
  artifacts cannot exhaust a model call with uncaptured plain text.

### Staging Product Contract

On deployed staging, prove all of:

- simple prompt such as `whats the weather in boston now` does not freeze at
  `vtext loop started`;
- research prompt routes to researcher when appropriate and produces a VText
  revision with interim progress;
- VText does not fail from provider `max_tokens` on ordinary revisions;
- super/vsuper/co-super smoke can call the configured coding model without a
  provider protocol error;
- text-only verifier smoke can run on DeepSeek V4 Flash/Pro or another
  configured text-only model;
- Kimi multimodal verifier path can process a screenshot/image or records an
  exact provider limitation;
- per-computer model policy changes take effect in later runs without platform
  redeploy or Node B env mutation, including at least one foreground role and
  one background/tool-heavy role;
- Trace shows provider-call progress and completion/error evidence live.

## Rollback

Rollback refs must include:

- previous provider adapter commit;
- previous model policy defaults;
- previous model-policy storage/API behavior;
- previous gateway/sandbox deploy SHA;
- any changed timeout/config defaults.

If the hardened Fireworks path remains unstable, the rollback plan may include
temporarily routing affected roles back to a known-good provider while keeping
the instrumentation patch, but that must be labeled a tactical rollback, not a
protocol fix.

## Run Checkpoint & Resumption State

```text
status: checkpoint_incomplete
last checkpoint: provider-neutral first-turn required-tool patch and focused tests on 2026-05-24
current artifact state: commit 9a30124 separates model catalog max from requested max_tokens, omits Fireworks max_tokens unless policy explicitly sets it, records optional max_tokens in model policy metadata, emits provider_call progress before blocking provider calls, and logs gateway request shape with tools/system/max/reasoning fields. The current working patch adds `tool_choice` through runtime, gateway, gatewayruntime, provider bridges, Fireworks Chat Completions, and ChatGPT Responses; VText agent revision runs now send `tool_choice:"required"` only on the first provider call, so later turns can finish naturally after tool results.
what shipped: 9a30124cadc46f9fb613900cfb71b3ea272d5645 reached staging and `/health` reported the same deployed_commit. The required-tool patch is local and tested, not yet deployed.
what was proven: focused local runtime/provider/gateway/gatewayruntime tests passed; direct Fireworks probes on Node B show DeepSeek V4 Flash accepts reasoning_effort omitted/none/low/medium/high/max; DeepSeek V4 Flash/Pro return tool_calls quickly; Kimi K2.6 handles a valid image_url; a Fireworks tool loop can complete a second turn without carrying reasoning_content back; gateway direct probes with large VText-like prompts/tools complete, with omitted max_tokens faster than forcing 131072; deployed gateway E2E prompt-bar smoke passed and opened a VText with initial revisions. New direct probes show Fireworks accepts `tool_choice:"required"` for DeepSeek V4 Flash, DeepSeek V4 Pro, and Kimi K2.6, returning `finish_reason=tool_calls` for each. New focused tests prove `tool_choice` reaches Fireworks and ChatGPT request bodies, crosses gateway/gatewayruntime bridges, is visible in provider-call progress, applies only to the first tool-loop call, and is used for VText agent revision runs.
unproven or partial claims: deployed staging VText completion remains broken until the required-tool patch is deployed and product-proven. Full role matrix, super/vsuper/co-super smoke, dynamic per-computer policy through product UI/API, ChatGPT comparison path, and Kimi multimodal product verifier path remain unproven.
belief-state changes: the current blocker is no longer invisible gateway auth or an unknown silent hang for the scratch case. Provider-call progress and gateway logs prove Fireworks accepted the request with max_tokens omitted, then the VText model generated uncaptured prose until the provider output cap. Direct Fireworks `tool_choice:"required"` probes confirm the likely fix is a protocol/tool-loop constraint, not reintroducing tiny output ceilings.
remaining error field: the required-tool patch still needs deploy and staging proof; VText/appagent loops may still need recovery behavior for malformed or partial tool calls; ChatGPT conductor auth remains a separate residual issue unless it blocks comparison/fallback tests; per-computer dynamic model policy edit path is not yet product-proven.
highest-impact remaining uncertainty: whether first-turn required-tool semantics make ordinary staging prompts complete without sacrificing long output or causing VText to over-call tools after a successful action.
next executable probe: commit/push/deploy the required-tool patch, verify staging identity, prove VText weather/story/current-events prompts produce revisions on staging, then prove role-agnostic per-computer policy edits across foreground and background roles.
suggested resume goal string: use the One-Line Goal String above
evidence artifact refs: Node B direct/gateway probe output in Codex transcript; focused test commands in Codex transcript; staging health for 9a30124; Playwright gateway E2E result `frontend/test-results/gateway-e2e-deployed/test-results.json`; scratch trajectory `6174ec99-c4ab-472d-a2bc-7d9aa8355f15`; local focused test command `nix develop -c go test ./internal/runtime ./internal/provider ./internal/gatewayruntime ./internal/gateway -run 'TestRunToolLoop(EmitsProviderCallProgressBeforeCall|InitialToolChoiceAppliesOnlyFirstCall|EndTurn)|TestVTextAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory|TestChatGPTProviderCallSuccess|TestFireworksProviderCallUsesOpenAIChatCompletions|TestFireworksProviderCallOmitsMaxTokensWhenUnset|TestGatewayBridgeProviderCallWithToolsUsesPerRunModelSelection|TestCallWithToolsRoutesThroughGatewayWireContract|TestMultiProvider_FireworksToolCalls'`; broader package command `nix develop -c go test ./internal/provider ./internal/gateway ./internal/gatewayruntime && nix develop -c go test ./internal/runtime -run 'TestRunToolLoop|TestVTextAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory|TestInitialConductorSeedRejectsVTextPriorsRevision|TestVTextPrompt'`
rollback refs: previous deployed provider/model-policy behavior before 9a30124; revert the required-tool patch commit if staging regressions appear; git revert 9a30124 if needed, preferably preserving provider_call observability
```
