---
definition_version: 2
definition_id: choir-rlm-engineering-carrier-2026-09-11
execution_mode: mission_orchestrator

# DRAFT — not chartered, not the working entrypoint. Mission two
# (`choir-rlm-versioned-rename-2026-09-09`) remains the sole working entrypoint until it reaches
# terminal deployed acceptance and the provider-preparation sequence (see now.blocker_or_risk)
# lands with its own Landing Loop. This file exists so the mission can be ratified and run with
# `/goal docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md` without re-deriving scope.
# Drafted from the 2026-09-11 thirteen-agent consensus; adjudication record:
# `docs/reports/choir-rlm-mission-three-consensus-2026-09-11.md`.

start:
  captured_at: "2026-09-11T04:20:00Z"
  source:
    canonical_ref: "main@6f1a8014 (draft base; re-pin at charter)"
    deploy_identity: "staging https://choir.news serves cb571960 via proxy 0475ed84; retained computer computer-03335285269bdba4f94377e56879f9e6 active at epoch 896; effects OFF; OpenCode Go and Zen providers not yet wired"
  worktree_inventory:
    status: reconciled
    evidence_ref: "2026-09-11 read-only git status; single worktree /Users/wiz/go-choir with pre-existing untracked user artifacts only"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP. This Definition owns the Engineering carrier surface: the eval envelope, the assigned-CoSuper registry, reducer intent reduction, run acceptance checkpoints, prompt assembly for the engineering desk, and the OpenCode provider preparation."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      recovery: leave_in_place
  candidates:
    - id: none
      ref: none
      base: none
  observed_artifact:
    - claim: "The evaluation envelope is not minimal: `capsule_go_eval` declares two interchangeable string parameters (`source`, `code` described as 'Alias for source.'), an empty required list, a silent source-wins fallback, and then calls `yaegikernel.CleanGoSource`."
      claim_scope: current
      evidence_ref: "internal/agentcore/tools_capsule.go:746-773"
    - claim: "A call with neither parameter executes an empty cell; a call with both silently resolves to `source`. Neither case fails at the schema boundary."
      claim_scope: current
      evidence_ref: "internal/agentcore/tools_capsule.go:754,769-772"
    - claim: "`CleanGoSource` strips markdown fences from model-authored Go at five call sites."
      claim_scope: current
      evidence_ref: "internal/yaegikernel/eval.go:301-317; callers tools_capsule.go:773, eval.go:122,148, session.go:86, cmd/capsule-broker/session_worker.go:392"
    - claim: "The assigned CoSuper registry is an exact closed set of ten JSON tools (the eval envelope plus the nine R7 retirement names) under the tools actuator, while the RLM overlay already excludes the four capsule file/exec operations. The remainder is hidden, not deleted."
      claim_scope: current
      evidence_ref: "internal/agentcore/tool_profiles_authority_test.go:110 (TestAssignedCoSuperBuilderIsExactClosedSet), :275 (TestRLMAssignedCoSuperOverlayIsSealedGo)"
    - claim: "Run acceptance builds the `capsule_effect_frozen` and `capsule_verification_recorded` checkpoints by scanning for tool results named `commit_transaction` and `record_self_development_verification`, each behind a `len(results) > 0` guard, and weights those checkpoints. Retiring the tools removes evidence without failing acceptance."
      claim_scope: current
      evidence_ref: "internal/agentcore/run_acceptance.go:627-645,790-791,843"
    - claim: "The reducer is a message router, not a settlement authority: `ReduceCellIntents` commits Message, Spawn and Complete only as mailbox envelopes, while `record_assignment_result` is the sole assignment-fate author."
      claim_scope: current
      evidence_ref: "internal/agentcore/rlm_reduce.go:167-198; internal/agentcore/cosuper_assignment_fate.go:20"
    - claim: "No model-id conditional exists anywhere in prompt assembly or REPL initialization; the RLM affordance text is one static overlay and initialization is one session setup."
      claim_scope: current
      evidence_ref: "internal/agentcore/tool_profiles.go:195-302; internal/runtimeprompts/prompts.go:57-62; cmd/capsule-broker/session_worker.go:274-285; internal/yaegikernel/session.go:53-72; internal/yaegikernel/choir.go:113-132"
    - claim: "The gateway and provider request structs carry no session identity, and the gateway decoder ignores unknown JSON fields, so an additive optional field is version-skew safe."
      claim_scope: current
      evidence_ref: "internal/provider/provider.go:69-105; internal/gateway/handlers.go:32-83,364-368"
  unknowns:
    - "Untested here: whether every `update_coagent` citer outside Engineering (Texture, research tools) has a working in-cell path, and what validation the staged message path loses."
    - "Untested here: the exact durable form of the verifier slot and whether `ChoirScope` can carry it."
    - "Untested here: whether the four capsule operations are load-bearing for the preserved `actuator=tools` rollback route."
    - "Assumed from the 2026-09-10 provider research note and to be re-pinned at the preparation Definition: live session-header enforcement, per-model wire shape, roster call results, image-input matrix, one-key coverage."

now:
  status: blocked_incomplete
  slice: "Draft only. Mission three is chartered when its three preconditions clear: (1) mission two reaches terminal deployed acceptance and releases the working-entrypoint position; (2) the OpenCode provider preparation lands with its own Landing Loop and a terminal receipt; (3) the owner ratifies the settlement-authority transfer that retiring `record_assignment_result` requires, superseding the mission-one clause that mailbox Complete never creates a terminal."
  question: none
  reconciliation:
    observed_at: "2026-09-11T04:20:00Z"
    source_ref: "main@6f1a8014 (draft base)"
    deploy_identity: "staging https://choir.news serves cb571960; effects OFF; provider preparation not yet landed"
  evidence_refs:
    - "docs/reports/choir-rlm-mission-three-consensus-2026-09-11.md (13-agent consensus, adjudications, dissent)"
    - "docs/reports/choir-opencode-provider-research-2026-09-10.md (provider prerequisite plan, live conformance matrix, verified roster, one-prompt invariant)"
    - "docs/reports/choir-rlm-missions-overview-2026-09-09.md (mission stack scope and ordering)"
    - "docs/mission-residues.md R7 (retirement inventory and the replay-returns-original-receipt obligation)"
    - "internal/agentcore/tool_profiles_authority_test.go:110,275 (registry closed sets)"
    - "internal/agentcore/run_acceptance.go:627-645,790-791,843 (tool-name-keyed acceptance evidence)"
    - "internal/agentcore/rlm_reduce.go:167-198; internal/agentcore/cosuper_assignment_fate.go:20 (reducer versus fate author)"
    - "internal/agentcore/tools_capsule.go:746-773; internal/yaegikernel/eval.go:301-317 (eval envelope and fence stripping)"
  blocker_or_risk: "Blocked on three preconditions, none of which is in this mission's authority: mission two settlement, provider preparation landing, and owner ratification of the settlement-authority transfer. Substantive risks: retiring the four settlement tools without first moving settlement into the reducer leaves the desk with no terminal authority; retiring `commit_transaction`/`record_self_development_verification` silently weakens run acceptance until those checkpoints stop keying on tool names; deleting the four capsule operations also removes them from the `actuator=tools` route that mission one preserved as a rollback; `update_coagent` retirement must stay desk-scoped or Texture and research paths break; a text-only roster member (`hy3`) cannot serve an image-reading task."
  next_action: "On charter: run the first Define as a code-free boundary — record the three discovered defects (alias with empty required, tool-name-keyed acceptance, reducer-is-not-a-settler), freeze the nine-operation mapping table and the prompt/REPL manifest, and state the falsifiers. No repair commit precedes that record."

finish:
  deliver: "The engineering desk lives entirely on the in-cell carrier and nothing else: `capsule_go_eval` is the desk's only JSON envelope, every other affordance is a typed in-cell function staging intents for the one reducer, the nine R7 names are deleted rather than hidden and each earned its deletion by replay proof, run acceptance no longer keys on tool names, one model-independent prompt with one REPL initialization serves a diverse roster with zero output repair, and Texture's packet/reducer design is specified against the canonical writer's invariants without landing code."
  artifact: "One deployed staging cutover on https://choir.news with effects OFF: the simplified envelope, the reducer-owned settlement path, the in-cell freeze/verify/inspect surface, the closed assigned registry, the frozen roster conformance evidence, and the Texture design artifact."
  entrypoints:
    implementation:
      - "internal/agentcore/tools_capsule.go"
      - "internal/agentcore/tool_profiles.go"
      - "internal/agentcore/rlm_reduce.go"
      - "internal/agentcore/cosuper_assignment_fate.go"
      - "internal/agentcore/run_acceptance.go"
      - "internal/yaegikernel/eval.go"
      - "internal/yaegikernel/session.go"
      - "internal/yaegikernel/choir.go"
      - "cmd/capsule-broker/session_worker.go"
      - "internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml"
  acceptance:
    - action: "T0-preparation (gate, owned outside this mission): land the OpenCode Go and Zen providers with a per-conversation session identity carried additively (no routing or policy change), a product User-Agent, fail-closed refusal when the identity is empty, the three wire shapes routed by model id, credentials delivered by the existing Node B path, and one live call per wire shape. Record the terminal receipt. This is the overview's 'only preparatory act': no evaluation runs, no promotion weight, no substrate change."
      proves: "Cross-model comparison is never gated on credential plumbing, and a provider outage can never be mistaken for a prompt or carrier failure."
      evidence_class: deployed_proof
    - action: "T1-simplify: reduce `capsule_go_eval` to a single required `source` string, delete the `code` alias, reduce the tool description to one sentence that states the affordance, and move the explanation arbitrary LLMs need into the desk's static affordance overlay. A call with no source and a fenced source both fail as parse or compile errors; neither is repaired, and neither executes an empty cell."
      proves: "The affordance the prompt describes is the only affordance the schema offers, and the envelope is small enough to explain once in the prompt."
      evidence_class: local_test
    - action: "T1-repair-deletion: delete `CleanGoSource` and its test, and every call site, including the broker worker path at `cmd/capsule-broker/session_worker.go:392`. Prove zero references remain and that a fenced cell now fails with an actionable error instead of being silently stripped."
      proves: "No code-level output repair exists on the carrier; a model that wraps code in fences is visible as a prompt or model problem, not absorbed."
      evidence_class: static_analysis
    - action: "T1-invariant: freeze the desk's resolved affordance component and its REPL initialization manifest, and assert a byte-identical prompt digest for a fixed desk across every roster model after excluding run entropy (timestamp, assignment and run-context tails). Add a guard that fails if any model identifier or per-model branch appears anywhere in the prompt-assembly or session-initialization path."
      proves: "The prompt varies by desk and never by model, and the property cannot regress silently."
      evidence_class: local_test
    - action: "T2-freeze: publish the operation-equivalence table for all nine retirement names — old JSON name, in-cell successor or intent kind, receipt class, canonical identity fields, the pre-declared exclusion list for nondeterministic fields, and the fixture that will prove it. Inventory every citer of each name and mark desk scope, naming the Texture and research paths explicitly."
      proves: "Scope is evidence-bounded and no retired name is left ambiguous or silently desk-leaking."
      evidence_class: static_analysis
    - action: "T3-settlement: make the staged Complete intent a settlement author by wiring it into the existing assignment-fate saga so the reducer-owned terminal produces the same durable fate, idempotency and conflict behaviour that `record_assignment_result` produces today. Record the owner ratification that supersedes the mission-one clause forbidding mailbox Complete from creating a terminal. Two completion verbs must not survive."
      proves: "Settlement has exactly one author under the new carrier, so the fate tool can be deleted rather than shadowed."
      evidence_class: local_test
    - action: "T3-acceptance: rebuild the `capsule_effect_frozen` and `capsule_verification_recorded` checkpoints so they derive from canonical events and in-cell receipts rather than from tool results named `commit_transaction` and `record_self_development_verification`, and make a run that lacks the evidence fail loudly instead of reporting `passed` with fewer checkpoints. Prove the negative: a run without the in-cell equivalent must not pass."
      proves: "Retirement cannot silently weaken run acceptance, and the acceptance score stops depending on tool names."
      evidence_class: local_test
    - action: "T3-parity: close the two parity gaps before any deletion — carry the verifier slot durably into the session scope so verifier-gated capabilities exist in-cell, and either carry `update_coagent`'s validation into the staged message path or record an explicit, ratified reduction of it."
      proves: "Retiring a tool does not retire the authority or the checks that tool performed."
      evidence_class: local_test
    - action: "T3-in-cell-surface: provide the missing in-cell affordances — a staged freeze intent, a staged verify intent, and a synchronous read-only bundle inspection (the overview's read-only exemption, not a two-turn protocol)."
      proves: "Every affordance the desk needs exists on the carrier before the JSON remainder disappears."
      evidence_class: local_test
    - action: "T4-replay: for each retired operation, capture the golden receipt before cutover (canonical input, semantic identity, exact receipt bytes, reference and digest, and an effect census); after cutover and a forced actor rewarm, invoke the successor under the same semantic identity and require exact canonical equality with zero additional effects; reuse the identity with changed canonical input and require a pre-effect conflict; use a fresh identity and prove a fresh operation. For the read-only operations, mutate the underlying state between calls: the same identity must return the original observation while a new identity observes the change."
      proves: "Each deletion is earned: replay through the new path returns the original receipt, and the new path is not merely similar."
      evidence_class: deployed_proof
    - action: "T4-registry: cut the assigned-CoSuper registry to the eval envelope alone, update the overlay's catalogue sentence to name that one tool plus the in-cell surface, and update both closed-set tests. Record explicitly what happens to the `actuator=tools` route and whether its four capsule operations remain as a named, unreachable-from-RLM rollback or are deleted with that consequence stated."
      proves: "Engineering has one envelope; the JSON remainder is deleted rather than hidden, and the rollback story is stated rather than discovered."
      evidence_class: local_test
    - action: "T4-delete: delete the nine retired JSON paths and their reducer aliases, including the admission-grammar special cases that exist only for them. Unknown JSON tool names fail closed."
      proves: "No dual path survives the cutover."
      evidence_class: static_analysis
    - action: "T5-roster: run one frozen desk task (inspect, edit, execute a test, complete with the exact receipt reference) on the verified diverse roster — at least five models, at least three vendors, all three wire shapes, including free and cheap members — under the frozen prompt digest, with no per-model fork, no repair, and no extra hints or retries. Record per-model pass or fail, tokens, latency and failure mode. A failing model is marked and either excluded with a recorded reason or fixed for every model; a per-model accommodation is a mission failure, not a fix."
      proves: "One prompt genuinely serves a diverse roster, which is the owner's completion goal, without reintroducing the overfit that produced the fence workaround."
      evidence_class: deployed_proof
    - action: "T5-cache-exit: confirm the later cache mission is unblocked — the session identity travels on the wire outside any cache-keyed payload, the static affordance body carries no timestamps, prompt assembly order is unchanged, and the eval schema version is recorded. No optimization, no hit-rate target and no per-model cache key is claimed here."
      proves: "The next mission inherits a stable prefix and a stable identity rather than a second re-plumbing job."
      evidence_class: static_analysis
    - action: "T6-texture-design (non-gating): publish the Texture packet and reducer design that translates documents, patches, diffs, source graphs, controls and dispositions into the same in-cell discipline, stated as constraints against the canonical writer's invariants (single-writer, stale-base comparison, atomic revision-graph identity, retry-preserved pending mutations, versioned compare-and-swap, fresh-but-not-replay wakes, per-document locking, atomic researcher opening) and citing them from their own authority. No Texture runtime code lands in this mission."
      proves: "Mission four has a specified object, and the engineering carrier does not foreclose Texture."
      evidence_class: static_analysis
    - action: "T6-landing: run the Landing Loop on behaviors changed here — commit, push, monitor CI, monitor the staging deploy, verify the deployed commit identity, and run the deployed acceptance proof with effects OFF, then record the receipts. Settle this Definition and move the three registries atomically, closing residue R7."
      proves: "The cutover is proven on the deployed product path, and the mission record is closed with artifacts rather than narrative."
      evidence_class: deployed_proof
  rollback: "Revert the mission commits and redeploy; the retired JSON paths return with the revert. The `actuator=tools` route is either preserved deliberately or its removal is named as a deliberate rollback reduction. Product restore remains a separate forward transaction on the computer's event chain, never a fix for a failed deploy."
  landing:
    required: true
    environment: "staging https://choir.news with effects OFF"
    required_receipts:
      - "pushed commit SHA and CI run"
      - "staging deploy and health/commit identity"
      - "deployed acceptance command and result, with accepted run/acceptance ids"
      - "the frozen roster conformance evidence artifact"
  not_done_when:
    - "Mission two has not reached terminal deployed acceptance, or this Definition has not been chartered and owner-ratified for the settlement-authority transfer."
    - "The provider-preparation sequence has not landed with a terminal receipt."
    - "Any retired name still exists on the live RLM path, or any retirement lacks its replay proof."
    - "Any run acceptance checkpoint still keys on a tool name, or a run can report `passed` with fewer checkpoints than before."
    - "Any model identifier or per-model branch appears anywhere in prompt assembly or REPL initialization."
    - "Any code path strips, repairs or tolerates malformed model output to make a cell work."
    - "Only one model, or one vendor, has been exercised on the frozen prompt."

boundaries:
  mutation_class: orange
  drafting_mutation_class: green
  authority_sources:
    - "owner mission-three goals stated 2026-09-10 (one prompt per desk for all models; no code-level parsing workarounds; simplest possible eval tool with the explanation in the system prompt; diverse roster; cache-conscious but not cache-optimizing)"
    - "docs/reports/choir-rlm-missions-overview-2026-09-09.md mission 3 scope"
    - "docs/mission-residues.md R6 and R7"
    - "docs/standing-questions.md"
  must_preserve:
    - "Historic tape decodability under frozen versioned rules; no rewrite of prior bytes."
    - "Mission-two vocabulary: this mission writes engineering-desk values only through the version-selected live vocabulary."
    - "The read-only synchronous exemption for observations."
    - "Provider credentials stay host-side; the conversation identity is request metadata, never a trust input."
  excluded:
    - "Texture runtime code, canonical-writer mutation, and any deletion of Texture JSON writers."
    - "Cache optimization, promotion weights, or any evaluation that promotes."
    - "Management and Research desk cutovers."
    - "Any per-model prompt fork, schema hint, retry ladder or output repair."
    - "Provider credential-management substrate work beyond the preparation Definition."
  protected_surfaces:
    - "run acceptance"
    - "gateway and provider calls (preparation only)"
    - "Texture canonical writes (design constraint only)"
    - "capsule execution and the capability broker"
  completion_evidence_floor:
    - "deployed_proof for the roster conformance, the replay-for-deletion proofs and the final landing"
    - "local_test for the envelope simplification, the prompt invariant, the settlement path, the acceptance rebuild and the parity gaps"
    - "static_analysis for the repair deletion, the registry cut, the path deletion and the cache exit"
  conjecture_delta:
    discovered:
      - "Falsified: that retiring the nine names is nine interchangeable deletions whose replacements already exist. `record_assignment_result` is the only assignment-fate author, `commit_transaction` and `record_self_development_verification` are the only sources of two acceptance checkpoints, and `inspect_self_development_bundle` is verifier-slot gated. Substitutes exist for the four capsule operations and the bundle inspection; they do not exist for the settlement and freeze paths."
      - "Falsified: that a chat-completions-only provider adapter is sufficient for the mission's roster. Muse Spark serves only the Responses API and Qwen serves the Anthropic Messages shape."
    falsifiers:
      - "If the frozen roster cannot pass the desk task on a single prompt without per-model accommodation, the one-prompt invariant is false as stated and the mission reports that rather than tuning around it."
      - "If replay through the new path cannot reproduce an original receipt for any retired operation, that operation is not deletable in this mission."
  heresy_delta:
    discovered:
      - "Tool-name-keyed acceptance evidence: run acceptance proves a frozen effect and a recorded verification by scanning for two tool *names* behind a presence guard, so deleting the tools removes evidence while acceptance still reports `passed`."
      - "Empty-envelope execution: because neither eval parameter is required, a call with neither executes an empty cell, and a call with both silently selects one."
      - "Hidden remainder: the RLM overlay already drops the four capsule operations while the shared registry still carries all ten, so the desk is dual-pathed by configuration rather than by capability."
      - "Settlement authority outside the reducer: the assignment fate is written by a JSON tool while the in-cell Complete intent only mails an envelope."
    introduced: []
    repaired: "none; mark repaired only after the deployed receipts exist"
measures:
  - "Per-model desk-task pass or fail on the frozen prompt, with tokens and latency."
  - "Count of code-level output repairs in the carrier path (target zero, measured by a guard rather than by prose)."
  - "Retired operations with a green replay proof (target nine of nine)."
  - "Acceptance checkpoints per run before and after the acceptance rebuild (must not decrease silently)."
  - "Prompt digest equality across roster members for a fixed desk."

receipts:
  - id: engineering-carrier-mission-three-consensus-2026-09-11
    boundary: define
    commit_or_artifact: "docs/reports/choir-rlm-mission-three-consensus-2026-09-11.md; draft base main@6f1a8014"
    proof_refs:
      - "thirteen-agent convergent panel at .agentic-consensus/mission3/run1 (non-durable process diagnostics): codex, claude/opus, cursor, opencode, devin, gpt-5.6 sol and luna, gemini-3.8, cursor-grok-4.6, muse-spark-1.3-contributor-free, nemotron-3-ultra-free, glm-5.3-flash, ling-3.0-flash-fin-free; 13 ok, 0 failed"
      - "orchestrator re-verification in source of: the eval alias with empty required, the five CleanGoSource call sites, the tool-name-keyed acceptance checkpoints, the reducer-versus-fate split, both closed-set tests, and the absence of model conditionals in the prompt path"
    rollback_ref: "docs-only draft; revert this commit to remove the draft Definition and the consensus record"
    disposition: "drafted, not chartered: awaits mission-two settlement, the provider-preparation landing, and owner ratification of the settlement-authority transfer"
    problem_ref: "docs/reports/choir-rlm-mission-three-consensus-2026-09-11.md findings on acceptance evidence, empty-envelope execution, the hidden remainder and the settlement-authority split"
    authorization_ref: "owner instruction 2026-09-11 to draft mission three with agentic consensus"
    candidate_or_evidence_refs: []
---

# RLM Mission 3 — Engineering crosses the carrier

Draft. Not chartered; not the working entrypoint. Mission two
(`choir-rlm-versioned-rename-2026-09-09`) holds that position until it reaches terminal deployed
acceptance, the OpenCode provider preparation lands with its own Landing Loop, and the owner
ratifies the settlement-authority transfer named below.

## What this mission is

Engineering becomes the first desk to live entirely on the in-cell carrier. `capsule_go_eval`
becomes the desk's only JSON envelope; every other affordance — files, execution, messaging,
spawning, completion, verification, inspection — becomes a typed in-cell function staging intents
for the one reducer; the nine residue-R7 names are deleted rather than hidden, each earning its
deletion through replay proof. In parallel, Texture's packet and reducer design is written against
the canonical writer's invariants and lands as a document, not as code. One model-independent
prompt with one REPL initialization serves a diverse roster with zero output repair.

Owner completion goal: **one prompt per desk that works for all models**, reached through the
structural invariant (no repair, one required parameter, identical prompt digest, a guard against
model conditionals) and evidenced by the frozen roster run.

## The three defects this mission must document before it repairs

1. **The envelope is not minimal.** `source` and `code` are interchangeable, nothing is required,
   a call with neither executes an empty cell, and a call with both silently picks one.
2. **Acceptance evidence keys on tool names.** `capsule_effect_frozen` and
   `capsule_verification_recorded` are built by scanning for `commit_transaction` and
   `record_self_development_verification` behind presence guards, so retiring those tools silently
   weakens run acceptance.
3. **The reducer is not a settler.** Staged Complete only mails an envelope; `record_assignment_result`
   is the sole assignment-fate author. Retiring it moves settlement authority into the reducer and
   supersedes a mission-one owner clause.

## Phase ladder

| Phase | Work | Gate |
| --- | --- | --- |
| T0 | Provider preparation (OpenCode Go and Zen, session identity, three wire shapes) — a separate Definition with its own Landing Loop | Terminal receipt before this mission charters |
| T1 | Envelope simplification, `CleanGoSource` deletion at all five sites, prompt and initialization invariant with a guard | Code-free Define first; no repair commit before the problem record |
| T2 | Operation-equivalence freeze: nine mappings, receipt classes, canonical versus excluded fields, citer inventory, desk scope | Frozen table before any deletion |
| T3 | Reducer-owned settlement, acceptance rebuild off tool names, parity gaps (verifier slot, message validation), missing in-cell affordances | Owner ratification for the settlement transfer |
| T4 | Replay-for-deletion per operation, registry cut to the eval envelope, deletion of the retired paths | No deletion without its replay proof |
| T5 | Frozen-roster conformance on one prompt, cache-mission exit notes | Roster evidence before completion |
| T6 | Texture design artifact (non-gating), Landing Loop, registry closure, R7 closed | Deployed proof with effects OFF |

## Roster

Verified 2026-09-10 and recorded in
`docs/reports/choir-opencode-provider-research-2026-09-10.md` §11: free Zen
(`muse-spark-1.3-contributor-free`, `ling-3.0-flash-fin-free`, and the two Nemotron ids as telemetry
because of ~100s pool latency) and cheap Go (`deepseek-v4.1-flash`, `glm-5.3-flash`,
`qwen3.8-flash`, `mimo-v2.5`, `hy3`, `muse-spark-1.3-contributor`). Image input is verified for
every roster member except `hy3`.

## Decisions this draft asks the owner to confirm at charter

- Ratify the settlement-authority transfer that retiring `record_assignment_result` requires, and
  the supersession of the mission-one clause that mailbox Complete never creates a terminal.
- Confirm that the `actuator=tools` route keeps its four capsule operations as a named rollback, or
  accept their deletion and the reduced rollback story.
- Confirm that the roster run is required evidence whose failure responses are pre-declared
  (exclude the model, or fix the shared prompt for every model) and never a per-model fork.
