# Search/VText Context Review

Date: 2026-05-26

Reviewed documents:
- [docs/design-search-provider-plane-v1.md](/Users/wiz/go-choir/docs/design-search-provider-plane-v1.md)
- [docs/design-vtext-platform-v3.md](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md)
- [docs/design-index.md](/Users/wiz/go-choir/docs/design-index.md)

Reviewed implementation paths:
- Search gateway: [internal/gateway/search.go](/Users/wiz/go-choir/internal/gateway/search.go), [internal/gateway/handlers.go](/Users/wiz/go-choir/internal/gateway/handlers.go), [internal/runtime/search_gateway.go](/Users/wiz/go-choir/internal/runtime/search_gateway.go), [internal/gateway/search_test.go](/Users/wiz/go-choir/internal/gateway/search_test.go), [internal/search/search.go](/Users/wiz/go-choir/internal/search/search.go)
- VText/runtime: [internal/runtime/runtime.go](/Users/wiz/go-choir/internal/runtime/runtime.go), [internal/runtime/vtext.go](/Users/wiz/go-choir/internal/runtime/vtext.go), [internal/runtime/tools_vtext.go](/Users/wiz/go-choir/internal/runtime/tools_vtext.go), [internal/runtime/tools_research.go](/Users/wiz/go-choir/internal/runtime/tools_research.go), [internal/runtime/tools.go](/Users/wiz/go-choir/internal/runtime/tools.go), [internal/runtime/tool_profiles.go](/Users/wiz/go-choir/internal/runtime/tool_profiles.go), [internal/runtime/vtext_workflow_verifier.go](/Users/wiz/go-choir/internal/runtime/vtext_workflow_verifier.go), [internal/runtime/prompt_defaults/conductor.md](/Users/wiz/go-choir/internal/runtime/prompt_defaults/conductor.md), [internal/runtime/prompt_defaults/vtext.md](/Users/wiz/go-choir/internal/runtime/prompt_defaults/vtext.md), [internal/runtime/skill_context.go](/Users/wiz/go-choir/internal/runtime/skill_context.go)
- Frontend surfaces: [frontend/src/lib/BottomBar.svelte](/Users/wiz/go-choir/frontend/src/lib/BottomBar.svelte), [frontend/src/lib/VTextEditor.svelte](/Users/wiz/go-choir/frontend/src/lib/VTextEditor.svelte)
- Sequencing evidence: [cmd/maild/main.go](/Users/wiz/go-choir/cmd/maild/main.go), [internal/maild/api.go](/Users/wiz/go-choir/internal/maild/api.go), [frontend/src/lib/EmailApp.svelte](/Users/wiz/go-choir/frontend/src/lib/EmailApp.svelte)

## Assessment

The two design docs are directionally better than the current runtime. The problem is not lack of ideas. The problem is that the live repo still encodes the older contract in runtime orchestration, prompt construction, verifier assumptions, tests, and frontend event projection. Right now the docs describe a cleaner platform than the code actually runs, and some tests still defend the older behavior. That is exactly the context-loss/accretion pattern you flagged.

## Findings

### [P0] Conductor still creates a canonical VText v1, so the single-writer contract is not true in the runtime.

The v3 design says:
- single canonical writer via `edit_vtext` only ([docs/design-vtext-platform-v3.md:28](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:28))
- conductor routes only ([docs/design-vtext-platform-v3.md:29](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:29))
- remove conductor canonical v1 ([docs/design-vtext-platform-v3.md:47](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:47))

The runtime still does the opposite when `create_initial_version` is set:
- it creates a user v0, then a conductor-authored appagent framing revision as v1 in [internal/runtime/runtime.go:1571](/Users/wiz/go-choir/internal/runtime/runtime.go:1571) through [internal/runtime/runtime.go:1598](/Users/wiz/go-choir/internal/runtime/runtime.go:1598)
- it marks that revision as `source=initial_vtext_seed`

Tests explicitly lock this behavior in:
- [internal/runtime/runtime_test.go:128](/Users/wiz/go-choir/internal/runtime/runtime_test.go:128) expects `create_initial_version=true`
- [internal/runtime/runtime_test.go:137](/Users/wiz/go-choir/internal/runtime/runtime_test.go:137) through [internal/runtime/runtime_test.go:199](/Users/wiz/go-choir/internal/runtime/runtime_test.go:199) expect a conductor-authored framing revision

Why this matters: the core architecture claim in the v3 doc is already false at document creation time. That means every later "single-writer" cleanup is starting from a broken root invariant.

### [P0] VText still depends on heuristic prompt taxonomy and hard-coded next-tool sequencing, despite the v3 design explicitly rejecting that model.

The v3 design says:
- no prompt taxonomy ([docs/design-vtext-platform-v3.md:13](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:13), [docs/design-vtext-platform-v3.md:50](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:50), [docs/design-vtext-platform-v3.md:94](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:94))
- no fixed multi-step workflow ([docs/design-vtext-platform-v3.md:13](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:13))
- remove `requires_worker_grounding` spawn-first and `initialVTextToolChoice` spawn-first ([docs/design-vtext-platform-v3.md:43](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:43) through [docs/design-vtext-platform-v3.md:46](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:46))

The runtime still hardcodes those patterns:
- `requires_worker_grounding` is still computed and stored in [internal/runtime/vtext.go:1655](/Users/wiz/go-choir/internal/runtime/vtext.go:1655) through [internal/runtime/vtext.go:1674](/Users/wiz/go-choir/internal/runtime/vtext.go:1674)
- `initialVTextToolChoice` still forces `spawn_agent` or `request_super_execution` based on that metadata in [internal/runtime/runtime.go:1687](/Users/wiz/go-choir/internal/runtime/runtime.go:1687) through [internal/runtime/runtime.go:1700](/Users/wiz/go-choir/internal/runtime/runtime.go:1700)
- `requiredContinuationAfterInitialVTextEdit` still runs prompt-marker heuristics and emits a required next tool in [internal/runtime/tools_vtext.go:115](/Users/wiz/go-choir/internal/runtime/tools_vtext.go:115) through [internal/runtime/tools_vtext.go:180](/Users/wiz/go-choir/internal/runtime/tools_vtext.go:180)
- those heuristics are literal string classifiers in [internal/runtime/tools_vtext.go:183](/Users/wiz/go-choir/internal/runtime/tools_vtext.go:183) through [internal/runtime/tools_vtext.go:257](/Users/wiz/go-choir/internal/runtime/tools_vtext.go:257)
- research tools still inject a required `submit_coagent_update` continuation in [internal/runtime/tools_research.go:126](/Users/wiz/go-choir/internal/runtime/tools_research.go:126) through [internal/runtime/tools_research.go:198](/Users/wiz/go-choir/internal/runtime/tools_research.go:198) and [internal/runtime/tools_research.go:313](/Users/wiz/go-choir/internal/runtime/tools_research.go:313) through [internal/runtime/tools_research.go:347](/Users/wiz/go-choir/internal/runtime/tools_research.go:347)

The verifier and tests also entrench the old topology:
- [internal/runtime/vtext_workflow_verifier.go:306](/Users/wiz/go-choir/internal/runtime/vtext_workflow_verifier.go:306) through [internal/runtime/vtext_workflow_verifier.go:316](/Users/wiz/go-choir/internal/runtime/vtext_workflow_verifier.go:316) require researcher/co-super child runs to be proven via `spawn_agent`
- [internal/runtime/vtext_test.go:4115](/Users/wiz/go-choir/internal/runtime/vtext_test.go:4115) through [internal/runtime/vtext_test.go:4145](/Users/wiz/go-choir/internal/runtime/vtext_test.go:4145) require `edit_vtext` to return `next_required_tool=spawn_agent`
- [internal/runtime/vtext_prompt_unit_test.go:64](/Users/wiz/go-choir/internal/runtime/vtext_prompt_unit_test.go:64) through [internal/runtime/vtext_prompt_unit_test.go:90](/Users/wiz/go-choir/internal/runtime/vtext_prompt_unit_test.go:90) assert prompt text that tells VText to spawn researcher next

Why this matters: the repo still uses prompt-shape and keyword-shape as control flow. That is the context-starved patching pattern in executable form.

### [P0] Search provider plane v1 is not implemented. The current gateway is still the legacy sequential/ephemeral system the design says to replace.

The search design says the current system must be replaced before further search prompt work ([docs/design-search-provider-plane-v1.md:3](/Users/wiz/go-choir/docs/design-search-provider-plane-v1.md:3), [docs/design-search-provider-plane-v1.md:12](/Users/wiz/go-choir/docs/design-search-provider-plane-v1.md:12), [docs/design-search-provider-plane-v1.md:156](/Users/wiz/go-choir/docs/design-search-provider-plane-v1.md:156)).

The live gateway still has the old behavior:
- sequential provider calls in [internal/gateway/search.go:170](/Users/wiz/go-choir/internal/gateway/search.go:170) through [internal/gateway/search.go:218](/Users/wiz/go-choir/internal/gateway/search.go:218), not parallel fan-out
- fixed in-memory cooldown map in [internal/gateway/search.go:109](/Users/wiz/go-choir/internal/gateway/search.go:109) through [internal/gateway/search.go:113](/Users/wiz/go-choir/internal/gateway/search.go:113) and [internal/gateway/search.go:324](/Users/wiz/go-choir/internal/gateway/search.go:324) through [internal/gateway/search.go:349](/Users/wiz/go-choir/internal/gateway/search.go:349), not durable health
- coarse error classification only for cooldown decisions in [internal/gateway/search.go:394](/Users/wiz/go-choir/internal/gateway/search.go:394) through [internal/gateway/search.go:412](/Users/wiz/go-choir/internal/gateway/search.go:412)
- handler returns a generic 502 on failure in [internal/gateway/handlers.go:790](/Users/wiz/go-choir/internal/gateway/handlers.go:790) through [internal/gateway/handlers.go:795](/Users/wiz/go-choir/internal/gateway/handlers.go:795), not structured `search_outage`
- route registration has no search health/reset ops endpoints in [internal/gateway/handlers.go:813](/Users/wiz/go-choir/internal/gateway/handlers.go:813) through [internal/gateway/handlers.go:821](/Users/wiz/go-choir/internal/gateway/handlers.go:821)
- runtime search client still only decodes `query/provider/providers/attempts/results` in [internal/runtime/search_gateway.go:37](/Users/wiz/go-choir/internal/runtime/search_gateway.go:37) through [internal/runtime/search_gateway.go:80](/Users/wiz/go-choir/internal/runtime/search_gateway.go:80)

The most important contract break is still present: the gateway can return HTTP 200 with zero merged results. A provider that returns `[]` without error is counted as success in [internal/gateway/search.go:195](/Users/wiz/go-choir/internal/gateway/search.go:195) through [internal/gateway/search.go:207](/Users/wiz/go-choir/internal/gateway/search.go:207), and `mergeSearchBatches` can return no results in [internal/gateway/search.go:243](/Users/wiz/go-choir/internal/gateway/search.go:243) through [internal/gateway/search.go:276](/Users/wiz/go-choir/internal/gateway/search.go:276).

Why this matters: v3 makes retrieval SLA a platform invariant ([docs/design-vtext-platform-v3.md:35](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:35)). That invariant is currently unavailable, so VText prompt/orchestration work is still being asked to compensate for broken infrastructure.

### [P1] The frontend chyron is still a client-side summary of raw low-level events, not the server-owned `chyron_line` contract in the design.

The v3 design wants:
- server-emitted `chyron_line`
- one sentence, bounded, no UUIDs
- frontend renders `chyron_line` only

See [docs/design-vtext-platform-v3.md:110](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:110) through [docs/design-vtext-platform-v3.md:117](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:117).

The current frontend does not do that. [frontend/src/lib/BottomBar.svelte:119](/Users/wiz/go-choir/frontend/src/lib/BottomBar.svelte:119) through [frontend/src/lib/BottomBar.svelte:172](/Users/wiz/go-choir/frontend/src/lib/BottomBar.svelte:172) derive ticker text locally from raw `tool.invoked`, `tool.result`, `channel.message`, and loop lifecycle events.

Why this matters: this keeps the presentation layer coupled to internal event taxonomy and guarantees more projection drift every time runtime events change.

### [P1] The instruction/context surface is still fragmented across multiple live layers, which recreates the contradiction problem the v3 doc is trying to solve.

The v3 summary says current failures come from contradictory instruction layers ([docs/design-vtext-platform-v3.md:11](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:11)).

The current runtime still has at least four active instruction layers:
- default role prompt files seeded from disk in [internal/runtime/prompt_store.go:171](/Users/wiz/go-choir/internal/runtime/prompt_store.go:171) through [internal/runtime/prompt_store.go:183](/Users/wiz/go-choir/internal/runtime/prompt_store.go:183)
- dynamic system prompt augmentation in [internal/runtime/tool_profiles.go:287](/Users/wiz/go-choir/internal/runtime/tool_profiles.go:287) through [internal/runtime/tool_profiles.go:412](/Users/wiz/go-choir/internal/runtime/tool_profiles.go:412)
- runtime skill-extract injection from `mission-gradient` and `cognitive-transform-portfolio` in [internal/runtime/skill_context.go:11](/Users/wiz/go-choir/internal/runtime/skill_context.go:11) through [internal/runtime/skill_context.go:44](/Users/wiz/go-choir/internal/runtime/skill_context.go:44)
- the large per-run VText request builder in [internal/runtime/vtext.go:1893](/Users/wiz/go-choir/internal/runtime/vtext.go:1893) through [internal/runtime/vtext.go:2027](/Users/wiz/go-choir/internal/runtime/vtext.go:2027)

This is before counting tool descriptions, which also carry workflow policy, for example [internal/runtime/tools_research.go:92](/Users/wiz/go-choir/internal/runtime/tools_research.go:92) through [internal/runtime/tools_research.go:97](/Users/wiz/go-choir/internal/runtime/tools_research.go:97).

Why this matters: the repo has not actually converged on a single contract surface. It has accumulated more prompt law around the old behavior.

### [P1] `docs/design-index.md` currently overstates sequencing discipline relative to the live repo.

The index says approved designs should be implemented in phase order ([docs/design-index.md:5](/Users/wiz/go-choir/docs/design-index.md:5) through [docs/design-index.md:17](/Users/wiz/go-choir/docs/design-index.md:17)). The v3 design puts Email/maild at P6 after search plane and VText platform work ([docs/design-vtext-platform-v3.md:152](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:152) through [docs/design-vtext-platform-v3.md:160](/Users/wiz/go-choir/docs/design-vtext-platform-v3.md:160)).

But the repo already contains substantial mail work:
- [cmd/maild/main.go:1](/Users/wiz/go-choir/cmd/maild/main.go:1)
- [internal/maild/api.go:1](/Users/wiz/go-choir/internal/maild/api.go:1)
- [frontend/src/lib/EmailApp.svelte:1](/Users/wiz/go-choir/frontend/src/lib/EmailApp.svelte:1)

while P0 and core P1-P4 VText/search cleanup are still not implemented.

Why this matters: the index currently reads like a control document, but it does not reflect the actual execution order the repo followed. In practice that makes it a misleading supervision surface.

## Assumptions

- I treated [internal/search/search.go](/Users/wiz/go-choir/internal/search/search.go) as legacy/dead code because I found no active references outside itself.
- I did not review `maild` correctness in this pass except as evidence that execution already moved ahead of the stated phase order.
- I treated the design docs as target-state design documents. If they are intended to describe already-shipped behavior, the severity of the drift goes up.

## Recommendations

1. Freeze new VText prompt work and all further Email/mail workflow surface expansion until search plane P0 and VText single-writer cleanup are real in code.
2. Delete the conductor-authored v1 path: `create_initial_version`, framing revisions, and tests that require them. Make the first appagent revision come only from `edit_vtext`.
3. Replace string-marker workflow classifiers with a typed context packet. The first deletion targets are `requires_worker_grounding`, `initialVTextToolChoice`, `vtextPromptNeedsResearchContinuation`, and automatic `next_required_tool` choreography where it is compensating for missing state rather than enforcing a true invariant.
4. Move search from prompt compensation to infrastructure: durable health store, explicit outage semantics, health/reset ops endpoints, parallel execution, and a hard ban on 200 + empty when eligible providers exist.
5. Make the verifier and tests assert the target architecture, not the old topology. Right now some of the hardest drift is locked in by tests.
6. Turn [docs/design-index.md](/Users/wiz/go-choir/docs/design-index.md) into a status-bearing index with columns like `target`, `current drift`, `blocking files/tests`, and `next deletion`. Without that, it reads as approval without accountability.

## Bottom line

The design direction is mostly right. The key failure is that the repo still carries the old contract in runtime code, prompts, verifier logic, tests, and frontend event projection. That is why the system keeps adding structure instead of converging. The next pass should be deletion-led and invariant-led, not feature-led.
