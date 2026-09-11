---
definition_version: 2
definition_id: choir-rlm-engineering-carrier-2026-09-11
execution_mode: mission_orchestrator

# DRAFT - not started, not the active mission. Mission two
# (`choir-rlm-versioned-rename-2026-09-09`) stays the active one until it reaches terminal deployed
# acceptance and the owner ratifies the settlement-authority transfer named below. This file is
# written so the mission can be started with
# `/goal docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md` without re-deriving scope.
# Drafted from the 2026-09-11 thirteen-agent consensus; adjudication record:
# `docs/reports/choir-rlm-mission-three-consensus-2026-09-11.md`.
# Owner decisions 2026-09-11: provider setup is phase 1 of this mission; the actuator=tools path is
# deleted, not kept as a rollback, once RLM is proven.

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
  slice: "Draft only. One thing is left before this mission can start: mission two must reach terminal deployed acceptance and give up the working-entrypoint position. The owner ratified the settlement-authority transfer on 2026-09-11 (the reducer owns assignment fate; the mission-one clause that a mailbox Complete never creates a terminal is replaced), and the OpenCode provider setup is phase 1 inside this mission, not a precondition outside it."
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
  blocker_or_risk: "Blocked on one thing, not in this mission's authority: mission two settlement. Risks: retiring the four settlement tools before the reducer owns settlement leaves the desk with no way to finish an assignment; retiring `commit_transaction`/`record_self_development_verification` weakens run acceptance until those checkpoints stop reading tool names; `update_coagent` retirement must stay desk-scoped or Texture and the research tools break; a text-only roster member (hy3) cannot serve an image task; phase 1 touches gateway/provider code and Node B credentials, which is red-class work inside an orange mission and needs its own rollback (the stub adapters) and its own Landing Loop; the owner's failure rule (fix the shared prompt for everyone) risks growing the prompt toward the weakest model, so every revision records its digest and size."
  next_action: "Phase 1 first: wire OpenCode Go and Zen (session identity on the wire, product User-Agent, fail-closed empty identity, three request shapes, keys on Node B), prove one live call per shape, and record the receipt. The problem record for phase 1 already exists in `docs/reports/choir-opencode-provider-research-2026-09-10.md`. Then the code-free Define: record the three defects, freeze the nine-operation mapping table and the prompt/REPL manifest, and state the falsifiers. No repair commit precedes that record."

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
    - action: "P1-provider: wire up the OpenCode Go and Zen providers inside this mission. Add a per-conversation session identity to provider requests (additive only: no routing change, no policy change), send a product User-Agent, refuse to run when the identity is empty, route the three request shapes by model id, install the keys the existing Node B way, and prove one live call per shape. No evaluation runs, no promotion weight, no substrate change. Problem record: `docs/reports/choir-opencode-provider-research-2026-09-10.md`."
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
    - action: "T3-settlement: make the staged Complete intent a settlement author by wiring it into the existing assignment-fate saga so the reducer-owned terminal produces the same durable fate, idempotency and conflict behaviour that `record_assignment_result` produces today. Owner ratified this transfer on 2026-09-11, replacing the mission-one clause that a mailbox Complete never creates a terminal. Two completion verbs must not survive."
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
    - action: "T4-registry: cut the assigned-CoSuper registry to the eval envelope alone, update the overlay catalogue sentence to name that one tool plus the in-cell surface, and update both closed-set tests. The `actuator=tools` branch and the four capsule file/exec operations it carries are slated for deletion, not kept as a rollback (owner decision, 2026-09-11); nothing in this mission may depend on them as a fallback."
      proves: "Engineering has one envelope; the JSON remainder is deleted rather than hidden, and the rollback story is stated rather than discovered."
      evidence_class: local_test
    - action: "T4-delete: delete the nine retired JSON paths and their reducer aliases, including the admission-grammar special cases that exist only for them. Unknown JSON tool names fail closed."
      proves: "No dual path survives the cutover."
      evidence_class: static_analysis
    - action: "T5-roster: run one frozen desk task (inspect, edit, run a test, complete with the exact receipt reference) under one prompt. Expected to pass, per the owner on 2026-09-11: `deepseek-v4.1-flash`, `muse-spark-1.3-contributor-free`, `glm-5.3-flash`, and gpt-5.6-luna (that last one already runs on the existing ChatGPT-authenticated path, not the OpenCode Zen or Go catalogue, so phase 1 does not provision it and the roster run uses that existing surface). Every other roster member is an experiment: run it, record pass or fail, tokens, latency and failure mode, and do not let it block the mission. This is experimental work whose success condition is learning what the yaegi tool API and system prompt must say for these models to use the desk."
    - action: "T5-prompt-fix: when a model fails, fix the shared prompt for every model - never a per-model branch, hint, retry ladder or schema fork. Record the prompt digest and size for each revision so growth is visible, and re-run the failing models against the new prompt. The same affordance failing across models is a prompt defect and is fixed in the shared body."
      proves: "One prompt genuinely serves a diverse roster, which is the owner's completion goal, without reintroducing the overfit that produced the fence workaround."
      evidence_class: deployed_proof
    - action: "T5-cache-exit: confirm the later cache mission is unblocked - the session identity travels on the wire outside any cache-keyed payload, the static affordance body carries no timestamps, prompt assembly order is unchanged, and the eval schema version is recorded. No optimization, no hit-rate target and no per-model cache key is claimed here."
    - action: "T5b-tools-actuator: keep the `actuator=tools` branch alive but unused through this mission, and prove the RLM path never reaches it. Deletion is deferred until the management and research desks also cross to RLM (owner decision, 2026-09-11); the deferred deletion is residue R8, closed when the last desk crosses. No mission proof may depend on the branch and no rollback path may target it."
      proves: "The next mission inherits a stable prefix and a stable identity rather than a second re-plumbing job."
      evidence_class: static_analysis
    - action: "T6-texture-design (non-gating): publish the Texture packet and reducer design that translates documents, patches, diffs, source graphs, controls and dispositions into the same in-cell discipline, stated as constraints against the canonical writer's invariants (single-writer, stale-base comparison, atomic revision-graph identity, retry-preserved pending mutations, versioned compare-and-swap, fresh-but-not-replay wakes, per-document locking, atomic researcher opening) and citing them from their own authority. No Texture runtime code lands in this mission."
      proves: "Mission four has a specified object, and the engineering carrier does not foreclose Texture."
      evidence_class: static_analysis
    - action: "T6-landing: run the Landing Loop on behaviors changed here — commit, push, monitor CI, monitor the staging deploy, verify the deployed commit identity, and run the deployed acceptance proof with effects OFF, then record the receipts. Settle this Definition and move the three registries atomically, closing residue R7."
      proves: "The cutover is proven on the deployed product path, and the mission record is closed with artifacts rather than narrative."
      evidence_class: deployed_proof
  rollback: "Revert the mission commits and redeploy; the retired JSON paths return with the revert. Phase 1 provider work rolls back by restoring the stub adapters and removing the installed keys. The `actuator=tools` branch is deliberately not a rollback target and is not deleted in this mission: it is held until the other desks cross (residue R8). Product restore stays a separate forward transaction on the computer's event chain, never a fix for a failed deploy."
  landing:
    required: true
    environment: "staging https://choir.news with effects OFF"
    required_receipts:
      - "pushed commit SHA and CI run"
      - "staging deploy and health/commit identity"
      - "deployed acceptance command and result, with accepted run/acceptance ids"
      - "the frozen roster conformance evidence artifact"
  not_done_when:
    - "Mission two has not reached terminal deployed acceptance."
    - "The provider-preparation sequence has not landed with a terminal receipt."
    - "Any retired name still exists on the live RLM path, or any retirement lacks its replay proof."
    - "Any run acceptance checkpoint still keys on a tool name, or a run can report `passed` with fewer checkpoints than before."
    - "Any model identifier or per-model branch appears anywhere in prompt assembly or REPL initialization."
    - "Any code path strips, repairs or tolerates malformed model output to make a cell work."
    - "None of the owner's expected-pass models (deepseek-v4.1-flash, muse-spark-1.3-contributor-free, glm-5.3-flash, gpt-5.6-luna) has completed the desk task on the frozen prompt."

boundaries:
  mutation_class: orange
  red_subclass: "phase 1 (gateway/provider routing and Node B credentials) and phase 4 (run acceptance)"
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
    - "Provider credential-management substrate work beyond phase 1 of this mission."
  protected_surfaces:
    - "run acceptance"
    - "gateway and provider calls (phase 1)"
    - "Texture canonical writes (design constraint only)"
    - "capsule execution and the capability broker"
  completion_evidence_floor:
    - "deployed_proof for the phase 1 provider calls, the roster conformance, the replay-for-deletion proofs and the final landing"
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
  - "Prompt digest equality across roster members for a fixed desk, with prompt size recorded at each revision."

receipts:
  - id: engineering-carrier-owner-decisions-2026-09-11
    boundary: define
    commit_or_artifact: "owner decisions recorded 2026-09-11 in conversation; folded into this Definition and the consensus record"
    proof_refs:
      - "owner: sweep the OpenCode configuration into this mission as its first phase"
      - "owner: get rid of the actuator=tools path; no rollback logic is planned to be used; the bar is the other desks crossing, so deletion is deferred to residue R8"
      - "owner: the reducer, from inside the cell, marks an assignment finished"
      - "owner: the roster is experimental with deepseek-v4.1-flash, muse-spark-1.3-contributor-free, glm-5.3-flash and gpt-5.6-luna expected to pass; other models are worth testing; success condition is the experiment itself"
      - "owner: when a model fails, fix the shared prompt for every model"
    rollback_ref: "none; a decision record"
    disposition: "recorded: phase 1 is provider setup; actuator=tools is deletion-slated, not preserved"
    problem_ref: "docs/reports/choir-opencode-provider-research-2026-09-10.md (provider problem record); docs/mission-residues.md R7 (retirement inventory)"
    authorization_ref: "owner statement 2026-09-11"
    candidate_or_evidence_refs: []

  - id: engineering-carrier-mission-three-consensus-2026-09-11
    boundary: define
    commit_or_artifact: "docs/reports/choir-rlm-mission-three-consensus-2026-09-11.md; draft base main@6f1a8014"
    proof_refs:
      - "thirteen-agent convergent panel at .agentic-consensus/mission3/run1 (non-durable process diagnostics): codex, claude/opus, cursor, opencode, devin, gpt-5.6 sol and luna, gemini-3.8, cursor-grok-4.6, muse-spark-1.3-contributor-free, nemotron-3-ultra-free, glm-5.3-flash, ling-3.0-flash-fin-free; 13 ok, 0 failed"
      - "orchestrator re-verification in source of: the eval alias with empty required, the five CleanGoSource call sites, the tool-name-keyed acceptance checkpoints, the reducer-versus-fate split, both closed-set tests, and the absence of model conditionals in the prompt path"
    rollback_ref: "docs-only draft; revert this commit to remove the draft Definition and the consensus record"
    disposition: "drafted, not chartered: awaits mission-two settlement and owner ratification of the settlement-authority transfer; the OpenCode provider setup is folded in as phase 1 (owner decision, 2026-09-11)"
    problem_ref: "docs/reports/choir-rlm-mission-three-consensus-2026-09-11.md findings on acceptance evidence, empty-envelope execution, the hidden remainder and the settlement-authority split"
    authorization_ref: "owner instruction 2026-09-11 to draft mission three with agentic consensus"
    candidate_or_evidence_refs: []
---

# Mission 3 - Engineering moves onto the in-cell carrier

This is a draft. It is not started. Mission two is still the active mission. This one starts when
mission two is finished and you say go.

## What this mission does

Engineering is the first desk to run fully on the in-cell carrier. Right now the desk is split in
two: one tool is the new way, nine JSON tools are the old way. After this mission:

- `capsule_go_eval` is the desk's only JSON tool. The model hands it Go source.
- Everything else is a normal Go function inside the cell: files, running commands, messages,
  spawning, finishing, verification, inspection.
- The nine old JSON tool names are gone, not hidden. Each one has to prove it works the new way
  before it is deleted.
- The prompt is the same for every model. No per-model text, no per-model branching, and no code
  that fixes up model output.

That last point is the real goal: one prompt per desk that works for every model.

## Why the old way existed

Two reasons, and we now handle both differently.

1. Models sometimes wrap code in fences. We fixed that in code, in a function called
   `CleanGoSource`. That is why some weaker models worked at all. This mission deletes that
   function. If a model wraps its code, we see it as a model problem instead of hiding it.
2. Some models did not understand the old two-field envelope. So the envelope carried an
   explanation. The explanation moves into the system prompt, where it belongs, and the envelope
   gets smaller: one field, required, nothing to guess.

## Three defects, written down before any fix

1. The envelope is not simple. `source` and `code` mean the same thing, nothing is required, and a
   call that sends neither runs an empty cell. A call that sends both silently picks one.
2. Run acceptance reads tool names. It marks a run as having a frozen effect and a recorded
   verification by looking for results named `commit_transaction` and
   `record_self_development_verification`. Delete those tools and a run still says "passed" while
   carrying less evidence.
3. The reducer cannot finish an assignment. Today a staged Complete is only a mailbox message, and
   `record_assignment_result` is the one thing that writes the assignment's fate. Deleting it moves
   that authority into the reducer. This replaces a rule you set in mission one.

## Phases

| Phase | Work | Done when |
| --- | --- | --- |
| 1 | OpenCode Go and Zen setup: session id on the wire, product User-Agent, three request shapes, keys on Node B | One live call works for each request shape, and staging health lists both providers |
| 2 | Shrink the eval tool to one required field; delete `CleanGoSource` and all its call sites | No repair code is left; bad input fails loudly; the prompt digest is identical for every model |
| 3 | Freeze the mapping: old name to new function, receipt class, and what counts as the same answer | The table is published and every old name is accounted for |
| 4 | Build what is missing: reducer owns settlement, acceptance stops reading tool names, two parity gaps closed, new in-cell functions for freeze, verify, inspect | Each is covered by a test, including the case where evidence is missing |
| 5 | For every old operation: same input gives the same receipt and no extra side effects, then delete it | All nine have a green replay record and the registry holds one tool |
| 6 | One frozen prompt, one desk task, a diverse set of models (experiment) | deepseek-v4.1-flash, muse-spark-1.3-contributor-free, glm-5.3-flash and gpt-5.6-luna complete the task under one prompt; the others are recorded |
| 7 | Texture design document (no code); then the normal deploy and proof | Design published; staging runs the new path with effects off |

Phase 1 can start as soon as this mission is chartered. It is separate from the prompt work, and
its problem record already exists in `docs/reports/choir-opencode-provider-research-2026-09-10.md`.

## Models in the roster

The roster is an experiment, not a fixed list. The owner expects these four to pass on one prompt:
`deepseek-v4.1-flash`, `muse-spark-1.3-contributor-free`, `glm-5.3-flash`, and gpt-5.6-luna.
gpt-5.6-luna is already set up: it runs on the existing ChatGPT-authenticated path, not the
OpenCode Zen or Go catalogue, so phase 1 does not need to provision it.

Everything else is worth testing and gets recorded: `ling-3.0-flash-fin-free`, the two Nemotron ids,
`qwen3.8-flash`, `mimo-v2.5`, `hy3`, `muse-spark-1.3-contributor`. Every one can read images except
`hy3`. Checked 2026-09-10 and listed in `docs/reports/choir-opencode-provider-research-2026-09-10.md`
section 11.

## Owner decisions

Recorded 2026-09-11:

- OpenCode setup is phase 1 of this mission, not a separate mission.
- `actuator=tools` gets deleted. It is not kept as a rollback. The wait ends when the management
  and research desks also cross, so the branch sits unused through this mission and the deletion is
  residue R8.
- The reducer, from inside the cell, marks an assignment finished. This replaces the mission-one
  rule that a mailbox Complete cannot finish one.
- The roster is experimental. Success is learning what the tool API and system prompt must say,
  with those four models expected to work.
- When a model fails, the shared prompt gets fixed for everyone. Never a per-model branch.
