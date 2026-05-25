# MissionGradient: Fireworks Reasoning Protocol Hardening v0

**Status:** checkpoint_incomplete
**Date:** 2026-05-24
**Depends on:** current deployed staging baseline, Fireworks provider credentials on Node B, current ChatGPT gateway credentials if used as a comparison path
**Related docs:** [platform-os-app-state.md](platform-os-app-state.md), [mission-runtime-model-context-substrate-v0.md](mission-runtime-model-context-substrate-v0.md)

## One-Line Goal String

```text
/goal Run docs/mission-fireworks-reasoning-protocol-hardening-v0.md as a Codex-operated MissionGradient mission: harden Choir's provider protocol, tool-loop behavior, and dynamic per-computer model policy so any configured compatible model can serve any agent role on the turns it is capable of serving. Treat current ChatGPT/Fireworks assignments as editable defaults, not architecture: conductor, VText, researcher, super, vsuper, co-super, verifier, and future roles must all be selectable across ChatGPT, Fireworks DeepSeek V4 Flash/Pro, Fireworks Kimi K2.6, and later catalog models when the current turn's modality, tool, context, and latency needs match. DeepSeek V4 Flash/Pro are text-only but still valid for orchestration, research, writing, coding, and verification that does not need image/media input; Kimi K2.6 and ChatGPT multimodal paths are required only for turns that actually carry screenshots, images, video frames, or other media inputs. Research current Fireworks OpenAI-compatible chat-completions docs, Fireworks reasoning docs, Fireworks-hosted DeepSeek V4 Flash/Pro behavior, Fireworks-hosted Kimi K2.6 behavior, Kimi K2.6 docs, DeepSeek V4 reasoning-content semantics as secondary context only, and Choir's ChatGPT Responses adapter as a comparison path. Run a deep request-shape and role matrix for max_tokens omitted vs bounded vs model maximum, reasoning_effort omitted/none/low/medium/high/max, thinking/reasoning budget parameters where supported, streaming vs non-streaming, tool-calling vs plain chat, required-tool behavior for VText/appagents, multimodal image input where supported, text-only verification where image input is not needed, and reasoning_content/thinking carry-forward across multi-turn tool loops. Prove behavior first with direct provider probes, then local Choir provider/runtime/tool-loop harness loops, then Node B/staging product-path prompt-bar runs. Fix the provider/runtime/model-policy/context plumbing so simple prompts do not hang, VText cannot spend a whole call writing uncaptured prose instead of producing a revision/tool action, long outputs remain possible, provider calls have correct per-call deadlines, interim progress is visible in Trace/VText, unsupported provider parameters are omitted, text-only models can be selected for non-image roles and verifier tasks, multimodal requirements are enforced only when the task actually needs media input, and per-computer model policy can be changed dynamically and agentically through durable product state rather than Node B env edits. Prove at least one foreground role and one background/tool-heavy role move to a different compatible model family through product state and then run successfully. Do not restore tiny 8k/16k ceilings as a false fix, hard-code one model per role, treat DeepSeek text-only status as unusability for verification, pass OpenAI Responses parameters to OpenAI Chat Completions providers, drop required reasoning_content when a provider requires carry-forward, hide hangs behind longer loop deadlines, or claim success without staging evidence for conductor, VText, researcher, super/vsuper/co-super, verifier text-only, Kimi multimodal, and runtime policy-edit paths. Land through git/CI/deploy, verify staging identity, update docs/model policy notes, and finish with a provider/model-policy protocol certificate, rollback refs, residual risks, and the next executable probe back toward the Chyron/Motion/Liquid/Python experiment rerun.
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
In practice this means both directions must be possible: if the active default
puts conductor/super on ChatGPT, those roles should still be movable to
Fireworks or another compatible model; if the active default puts VText or
researcher on Fireworks, those roles should still be movable to ChatGPT or
another compatible model. Conductor, VText, researcher, super, vsuper,
co-super, verifier, and future roles are runtime consumers of model
capabilities, not hard-coded provider homes.

Compatibility is decided per turn, not per role. A text-only model can
supervise, research, write, code, or verify text/source/Trace evidence. A
multimodal model is needed only when the turn includes screenshots, images,
video frames, or other media inputs. A strong coding model may be useful for
`super`, `vsuper`, or a worker `co-super`, but that is a policy preference, not
an architectural law. The selection should be editable at runtime through
durable product state, including agentic edits by `super` in response to an
owner prompt, without patching Node B environment variables or deploying a new
platform build just to change a computer's model policy.

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

## Target Model Policy Shape

The desired end state is a capability lattice, not a role-to-provider table.
The platform model catalog answers "what can this model do and how must it be
called?" The active computer's model policy answers "which compatible model
should this computer prefer for this role or task right now?"

That means:

- every configured compatible model is eligible for every role, including
  conductor, VText, researcher, super, vsuper, co-super, verifier, and future
  roles;
- current defaults such as "ChatGPT for conductor/super" or "Fireworks for
  VText/researcher" are only the active computer's starting policy. They are
  not proof that those roles belong to those providers;
- compatibility is checked at the turn boundary from actual requirements:
  modality, tool calling, context length, output budget, reasoning controls,
  latency, cost, and provider deadline class;
- DeepSeek V4 Flash/Pro being text-only excludes only turns that actually need
  image/media input. It does not exclude them from verification, supervision,
  coding, writing, or research in general;
- Kimi K2.6 or another multimodal model is required only for turns that carry
  screenshots, image artifacts, video frames, or other media inputs;
- the owner should be able to ask Choir to change policy in natural language,
  causing `super` to edit durable per-computer model policy through product
  state, after which later runs use the new effective policy without a platform
  deploy;
- provider secrets remain platform-owned, and ordinary users edit policy by
  selecting from declared model capabilities, not by editing secrets.

This mission should move along that gradient. It does not need to finish the
entire future UI, but it must not reinforce fixed role/model assumptions while
repairing the current Fireworks and ChatGPT regressions.

## Core Invariant

```text
Provider protocol correctness and task-compatible model selection beat static role defaults.
```

The model catalog can record context windows, modalities, theoretical output
limits, tool-call support, reasoning controls, and provider features. It should
not become a role lock-in table. Runtime selection must then match the current
task: text-only models are valid for text-only verification, coding, research,
writing, and orchestration; multimodal models are required only when the turn
needs image or media input. The request shape must reflect what each
provider/model actually supports in multi-turn agent loops. A provider adapter
must omit unsupported or harmful parameters, preserve required state such as
reasoning content when a provider requires it, force tool use when an appagent
cannot safely answer in plain text, and expose progress before long calls.

The target is not "switch from ChatGPT to Fireworks." The target is that Choir
can run any role on any configured compatible model, then change that policy
dynamically and agentically when a user asks for a faster, cheaper, stronger, or
multimodal path.

## Real Artifact

The artifact is a deployed provider/runtime substrate where:

- simple prompt-bar tasks complete quickly and visibly;
- VText produces the human-readable narrative revision through its revision
  tool path instead of hanging or emitting uncaptured plain prose;
- researcher routes can call web/search/fetch tools and report findings;
- super/vsuper/co-super can run tool loops without protocol errors;
- conductor, VText, researcher, super, vsuper, co-super, verifier, and future
  roles can be switched among configured compatible models without code edits
  or platform deploys;
- role defaults are visible as the effective model policy for a computer, while
  the model catalog remains the capability registry and not an enforcement
  table of "which role must use which provider";
- at least one foreground role and one background/tool-heavy role are moved to a
  different compatible model family through product state and then produce
  product-path Trace/VText evidence using that policy;
- DeepSeek V4 Flash/Pro can be used for verifier tasks that do not require
  image input;
- Kimi K2.6 can process screenshots/images through the declared multimodal
  path;
- multimodal requirements are declared and enforced at the task/turn boundary,
  not by permanently locking verifier roles to one model;
- per-computer model policy is durable product state and can be edited by the
  owner or by `super` through the product path, including ordinary natural
  language requests such as asking Choir to use a faster writing model or a
  stronger coding model for subsequent runs;
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

## Current Provider-Doc Findings

These findings are current working assumptions to verify against direct probes
and staging behavior:

- Fireworks' Chat Completions API is OpenAI-compatible but adds reasoning
  controls. It documents DeepSeek V4 support for `reasoning_effort` values
  `none`, `low`, `medium`, `high`, `xhigh`, and `max`; `low`/`medium` are
  promoted to `high`, `xhigh` to `max`, and `none` disables thinking.
- Fireworks documents `reasoning_history` as the control for preserving,
  stripping, or interleaving historical `reasoning_content` in multi-turn
  conversations. Choir must either preserve required reasoning fields or
  deliberately disable/strip them where the provider/model supports that.
- Fireworks tool calling supports `auto`, `none`, `required`, and exact
  function-object `tool_choice` for compatible chat-completions models.
- Kimi's own K2.6 docs say K2.6 is OpenAI-format compatible, supports text,
  image, and video input, and emits `reasoning_content` when thinking is
  enabled. They also warn that multi-step tool calls must keep assistant
  `reasoning_content` in context, and recommend streaming for thinking models.
- Kimi's own docs also constrain native Kimi `tool_choice` to `auto`/`none` in
  thinking mode. Fireworks-hosted Kimi behavior must therefore be tested
  directly before using exact required tool choice with Kimi; Kimi multimodal
  verification can still run without tools.
- DeepSeek's own API docs expose `reasoning_content` on DeepSeek V4 responses
  and streaming deltas. These docs are secondary for this mission because Choir
  calls DeepSeek V4 through Fireworks, but they support the carry-forward risk.

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

The pass condition is not that every provider/model can do every possible turn.
The pass condition is that incompatible turns fail or reroute at the capability
boundary with clear evidence, while compatible turns are allowed for every role
without code changes.

The pass condition also includes negative proof against role lock-in: the
system must not reject a model merely because a role has a different historical
default. For example, a DeepSeek text-only model may serve `super`,
`researcher`, `vtext`, or text-only `verifier` turns; it should be rejected only
when the turn actually requires unsupported media input or another unsupported
provider feature.

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
8. Role-agnostic policy proof: move at least one foreground role and one
   background/tool-heavy role to a different compatible model family through
   product state, then prove the next run records and uses that policy.
9. Agentic policy edit: an owner prompt asks `super` to adjust the computer's
   model policy; later runs use the changed policy, and Trace/VText explain the
   resolved model without exposing secrets.

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
- Do not treat `RecommendedFor` catalog hints as enforcement. They are UI and
  policy suggestions; capability checks are what decide compatibility.
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
- Product-path policy tests prove an owner or `super` can change effective
  model selection for a computer without a platform deploy or Node B env edit.
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
- role/model defaults are treated as editable policy: at least one proof should
  move a role away from its current default provider family and back, or record
  the exact unsupported capability that prevents that turn from moving;
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
last checkpoint: deployed reasoning-content carry-forward patch at e3bd495, followed by staging product-path smoke proofs on 2026-05-24
current artifact state: commits 9a30124, 67cb492, and 0c4c0ff hardened Fireworks max-token handling, tool_choice propagation, and required-tool control turns. Commit 1001d05 fixed the deployed short-story/VText-only path by routing creative draft prompts to VText instead of forcing researcher/super. Commit 871ea7c documented the Fireworks/Kimi reasoning-content protocol gap. Commit e3bd495 preserved `reasoning_content` through provider, gateway, gatewayruntime, and runtime tool-loop assistant history without rendering it as visible user text. Commit f82b3fe preserved selected model token budgets while bounding required-tool misses. Commit 196ae15 converts runtime-required next tools from broad `tool_choice: required` to exact `function:<tool_name>` wire choices, serialized as OpenAI-compatible function-choice objects by ChatGPT/Fireworks adapters.
current unlanded patch: plain `verifier` no longer aliases to `verifier_multimodal`. Generated and fallback model policy now declare `verifier` as Fireworks DeepSeek V4 Pro text-only and `verifier_multimodal` as Fireworks Kimi K2.6 image-capable. Model catalog hints now mark DeepSeek V4 Pro as eligible for text-only verification while preserving Kimi as the multimodal verifier hint.
what shipped: 196ae15612985390bfdbef5fd35d2373a91bec43 reached staging; `/health` reported proxy and upstream deployed_commit at 196ae15 with deployed_at `2026-05-25T02:47:52Z`. GitHub Actions CI run 26380538907 succeeded, including Go test shards, Go vet/build, and 15s Deploy to Staging (Node B). FlakeHub publish run 26380538884 was triggered separately by the same push.
what was proven: direct Fireworks probes on Node B show DeepSeek V4 Flash accepts omitted, 4096, and 131072 `max_tokens`; DeepSeek V4 Pro accepts reasoning_effort omitted/none/low/medium/high/max, with `none` suppressing reasoning_content and the other modes returning it; DeepSeek V4 Pro required-tool first turn returns tool_calls plus reasoning_content; a second turn succeeds when the assistant reasoning_content and tool_calls are passed back with the tool result; Fireworks-hosted Kimi K2.6 returns reasoning_content and processes a valid image URL. Focused local tests prove reasoning_content is preserved in Fireworks non-streaming responses, Fireworks streaming reasoning deltas, gatewayruntime response parsing, and runtime tool-loop assistant history. Local tests after 196ae15 prove exact required next tool choice is selected by the tool loop and serialized as OpenAI-compatible exact function choice for ChatGPT Responses and Fireworks Chat Completions. Local tests after the current verifier-policy patch prove `verifier` and `verifier_multimodal` are distinct policy roles, `verifier` resolves to DeepSeek V4 Pro, text-only verifier calls omit Fireworks `max_tokens`, and image input to a text-only model is blocked before provider call. Deployed staging proof `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com npx playwright test tests/worker-required-next-tool-proof.tmp.spec.js --project=chromium --reporter=line` passed in 4.4m with marker `WORKER_REQUIRED_NEXT_TOOL_PROOF_1779677337507`, trajectory `fce533d3-1b8c-48f1-b08b-69eda055646c`, worker `worker-9a03a64c63e2a337`, worker run `54b77f9c-851d-4f70-8500-405b0f3e714f`, `startStatus=worker_run_started`, `finishStatus=worker_run_completed`, `finishState=completed`, and completed worker child runs `7b1bf544-2f8a-45ba-95e3-d7547dd8148e` and `ca535f3e-b8f5-476c-8baf-ef14381518e7`. Node B logs for the proof show Fireworks DeepSeek V4 Pro receiving `tool_choice=function:start_worker_delegation` and returning `stop=tool_use` rather than exhausting a long completion. Earlier staging proofs also include `tests/gateway-e2e-deployed.spec.js` passing in 10.4s, simple VText smoke `PROVIDER_SMOKE_1779668302525`, research smoke `RESEARCH_SMOKE_1779668370511`, product-path model-policy smoke `a21bdc6f-50fc-4384-9209-84f484b567d2`, and super smoke `98613774-e39a-4467-883c-1684d520266c`.
unproven or partial claims: worker/vsuper model-path proof is now partial rather than absent: request/start/observe worked and the vsuper run executed a command, but `finish_worker_delegation`, terminal worker certificate, and co-super spawning remain unproven after e3bd495. Text-only verifier role selection is locally/catalog-proven but not yet product-path proven on deployed staging. Kimi multimodal is direct-provider proven, but there is not yet a product verifier path that passes an image/screenshot artifact through the gateway to Kimi; artifact_ref image inputs still block before provider call by design. ChatGPT comparison/fallback remains unproven in this checkpoint because current product-default proofs used Fireworks. The deployed live-search Playwright spec `tests/vtext-deployed-live-search.spec.js` timed out before prompt submission due a registration UI fill hang, so it is not provider evidence but does reveal that the spec should use the same robust product auth helper as the gateway proof. The first worker/vsuper smoke lost auth with a 401 while polling Trace, so long-running proof observers need session renewal even when the underlying run continues.
belief-state changes: Fireworks protocol is no longer globally broken. The sharp worker-required-next-tool failure was caused by broad required-tool selection, not by output budget alone. Exact provider tool choice turns `request_worker_vm -> start_worker_delegation` into a deterministic control transition while preserving long-output defaults for ordinary model calls. Gateway call deadlines are now 10 minutes end-to-end through both gateway handler and sandbox gateway client, so the prior five-minute concern is not the current configured limit. The remaining instability field shifts from "cannot start/finish worker-vsuper" to "can the same path reliably produce useful human evidence and verifier/co-super work for real experiment payloads?"
remaining error field: product-path verifier model-routing proofs are still missing. The worker proof now reaches terminal finish evidence and completed child runs, but it does not yet prove rich experiment behavior, human-readable VText dashboards, screenshots/video, or text-only/multimodal verifier specialization. VText can still emit confusing transient tool errors before eventual success. The product model-policy path works through file APIs, but the higher-level UX/agentic edit path is not yet polished. Long product-path proofs can outlive a browser session unless the proof harness refreshes passkey auth.
highest-impact remaining uncertainty: whether the now-working worker/vsuper/co-super path can be driven to useful feature evidence without Codex hand-coding the experiments, and whether verifier specialization can use DeepSeek text-only and Kimi multimodal turns through product policy rather than platform defaults.
next executable probe: add a narrow verifier product path for (a) text-only DeepSeek verification and (b) Kimi image verification from a URL/base64 image, then return to the Chyron/Motion/Liquid/Python experiment rerun with VText as the narrative dashboard and screenshots/video as owner evidence.
suggested resume goal string: use the One-Line Goal String above
evidence artifact refs: official Fireworks Chat Completions and Reasoning docs; Kimi docs; DeepSeek reasoning-content docs as secondary context; Node B direct probe outputs in Codex transcript; local focused test commands `nix develop -c go test ./internal/provider ./internal/runtime ./internal/gatewayruntime ./internal/gateway -run 'TestFireworksProvider(PreservesReasoningContent|StreamPreservesReasoningContent|CallUsesOpenAIChatCompletions|CallOmitsMaxTokensWhenUnset|StreamUsesOpenAIChatCompletionsChunks)|TestRunToolLoop(CarriesAssistantReasoningContent|InitialToolChoiceAppliesOnlyFirstCall|RequiredToolTurnIsBoundedAndRetriesMissingTool)|TestCallWithToolsRoutesThroughGatewayWireContract|TestExecuteStreamsGateway(ReasoningWithoutRendering|Deltas)|TestProviderInference' -count=1` and broader focused provider/runtime tests; GitHub Actions run 26376503916; staging health at e3bd495; deployed proof submissions `cdada072-92ad-4377-a728-ae2de3bb18a7`, `7f4da787-e3ae-4c90-8eda-351beaf0c3b4`, `a21bdc6f-50fc-4384-9209-84f484b567d2`, `98613774-e39a-4467-883c-1684d520266c`, and worker/vsuper proof submission `67add974-aae5-4757-b4c2-d27638795fa4`
rollback refs: previous deployed provider/model-policy behavior before 9a30124; revert 1001d05 for VText routing regression; revert e3bd495 if reasoning_content serialization regresses; revert 196ae15 if exact tool choice breaks a provider adapter, preferably preserving provider_call observability and omitted-Fireworks-max-token behavior
```
