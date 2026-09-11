# RLM Mission Three — Agentic Consensus Record

Date: 2026-09-11. Mode: convergent. Panel: 13 agents, 13 succeeded, 0 failed.
Prompt: `.agentic-consensus/mission3/prompt.md` (gitignored process diagnostics).
Raw outputs: `.agentic-consensus/mission3/run1/` (manifest, per-agent output, commands).
Base revision at run time: `main@6f1a8014`.

This record is the durable adjudication. The draft it produced is
[`../definitions/choir-rlm-engineering-carrier-2026-09-11.md`](../definitions/choir-rlm-engineering-carrier-2026-09-11.md).

## Panel

| Agent | Status | Duration |
| --- | --- | --- |
| omp-muse-spark (`opencode-zen/muse-spark-1.3-contributor-free`) | ok | 67s |
| opencode | ok | 75s |
| omp-ling (`opencode-zen/ling-3.0-flash-fin-free`) | ok | 92s |
| omp-glm53-flash (`opencode-go/glm-5.3-flash`) | ok | 110s |
| omp-gemini38 | ok | 187s |
| cursor | ok | 318s |
| omp-cursor-grok46 | ok | 348s |
| omp-gpt56-sol | ok | 354s |
| claude (opus) | ok | 431s |
| codex | ok | 477s |
| devin | ok | 479s |
| omp-gpt56-luna | ok | 788s |
| omp-nemotron-3-ultra | ok | 791s |

**Panel-health caveat (evidence exposure).** `omp-gpt56-sol` (942 bytes) and `omp-gpt56-luna`
(11 bytes) did not answer the task; they returned repository reconciliation commentary that
references this session's adjacent work (mission 2's open items 10-15, HEAD `6f1a8014`, the
untracked worktree paths). They read exogenous context rather than reasoning from the prompt
alone, so they are not blind replicates for this question and their content is excluded from
consensus below. Everything else answered the requested format.

## Locally verified claims

The orchestrator re-checked every load-bearing panel claim in the tree. Those marked **verified**
are confirmed; the rest are reported as panel assertion.

| Claim | Status | Evidence |
| --- | --- | --- |
| `capsule_go_eval` exposes two interchangeable params (`source`, `code`), empty `required`, silent source-wins fallback, then `CleanGoSource` | **verified** | `internal/agentcore/tools_capsule.go:746-773` |
| `CleanGoSource` is called at **five** sites, not four | **verified** (brief was wrong; claude and glm53 caught it) | `tools_capsule.go:773`, `yaegikernel/eval.go:122,148`, `session.go:86`, `cmd/capsule-broker/session_worker.go:392` |
| Run acceptance builds `capsule_effect_frozen` and `capsule_verification_recorded` by scanning for tool results **named** `commit_transaction` / `record_self_development_verification`, each guarded by `len(results) > 0`, and weights them | **verified** | `internal/agentcore/run_acceptance.go:627-645`, weights `:790-791,843` |
| `ReduceCellIntents` commits exactly three intent kinds (Message, Spawn, Complete) and commits all of them as mailbox envelopes; it performs no settlement write | **verified** | `internal/agentcore/rlm_reduce.go:167-198` |
| `record_assignment_result` is the assignment-fate author | **verified** | `internal/agentcore/cosuper_assignment_fate.go:20` |
| The nine R7 names plus `capsule_go_eval` are the exact ten-tool assigned set; the RLM overlay already excludes the four capsule ops | **verified** | `internal/agentcore/tool_profiles_authority_test.go:110` (`TestAssignedCoSuperBuilderIsExactClosedSet`), `:275` (`TestRLMAssignedCoSuperOverlayIsSealedGo`) |
| No model-id conditional exists in prompt assembly or REPL initialization | **verified** by two independent scouts and three panelists | `tool_profiles.go:195-302`, `runtimeprompts/prompts.go:57-62`, `session_worker.go:274-285`, `session.go:53-72`, `choir.go:113-132` |
| `update_coagent` citers outside Engineering | panel assertion (claude, devin) | `texture.go:2303`, textureowner `request_source` branches, researchtools; not re-verified here |
| Verifier-gated tools require `runMetadataCoSuperSlot == "verifier"` while `ChoirScope` carries role but not slot | panel assertion (devin) | `tools_capsule.go:410,491`; not re-verified here |
| The four capsule ops are shared with the non-RLM assigned path, so deleting them affects `actuator=tools` | panel assertion (devin) | `RegisterCapsuleLocalTools`, `tools_capsule.go:79-88`; not re-verified here |

## Consensus (agreed by 2+ agents)

1. **Provider preparation is a separate preparatory sequence that lands before mission three is
   chartered.** Not mission work. Different protected surface (`red` gateway/provider/credentials),
   different rollback (Node B env file + gateway restart), and mixing them makes a provider outage
   indistinguishable from prompt or carrier failure. (11 of 13 substantive outputs; dissent below.)
2. **Eval simplification and the nine retirements are one mission with serialized slices.** The
   exact singleton registry cannot be true until both are done; splitting produces an accepted
   hybrid where the sole doorway still depends on legacy envelopes or output repair.
3. **Texture contributes a design artifact inside this mission; its landing is mission four.**
   Read-only observations may remain synchronous typed functions.
4. **The mission is not "delete nine tools whose replacements exist."** A reducer-side settlement
   path, an in-cell freeze/verify/inspect surface, and the verifier slot must be built first.
5. **Every retirement must be earned by replay proof**, not by catalogue intent.
6. **The one-prompt invariant is structural before it is empirical**: one static affordance body,
   one REPL initialization, zero model-id conditionals, zero output repair.
7. **Three defects must be documented before any repair commit** (problem-documentation-first):
   the `source`/`code` alias with empty `required` (a call with neither executes an empty cell), the
   tool-name-keyed acceptance checkpoints, and `record_assignment_result` being the only fate author
   while `choir.Complete` is only a mailbox envelope.
8. **`inspect_self_development_bundle` is read-only** and takes the exemption the overview grants.

## Dissent and disagreements

- **Gate choice (claude, strongest dissent).** Making third-party model behaviour the completion
  gate while forbidding per-model accommodation "creates a trap with exactly one escape: the
  accommodation you just banned". Adopted: the **structural** invariant is the gate (repair deleted,
  single required parameter, prompt digest identical, CI guard against model conditionals), and the
  roster run is required evidence whose failure responses are pre-declared and never per-model.
- **Provider prep placement (ling).** Ling alone argues it should be a gate-zero inside mission
  three. Rejected: it is the one place where an ops failure would be misread as a prompt failure,
  and the overview already names it the only preparatory act.
- **Texture placement (cursor-grok, gemini38).** Both argue Texture's design should be a sibling
  Definition or residue, not inside mission three's finish. Partially adopted: the design is in
  scope as a deliverable but explicitly **non-gating**; mission three may complete without it.
- **Retirement ordering (claude).** Two tiers by substitute-existence — T1 the four capsule ops and
  bundle inspect (substitutes already prebound), T2 the four settlement tools (no substitute). This
  is adopted as the internal phase structure rather than a scope split.
- **Replay proof grain (codex vs muse-spark).** Codex adds a state-mutation check for the read-only
  class and requires zero additional effects; muse-spark wants byte-equality after a closed
  canonicalization list. Both adopted: equality on canonical fields with a pre-declared exclusion
  list, plus the mutation check for read/list/inspect.

## Unique high-value findings

- **Silent acceptance-evidence loss** (claude, verified): retiring `commit_transaction` and
  `record_self_development_verification` leaves run acceptance reporting `passed` with strictly less
  evidence, because the checkpoints are keyed on tool *names* behind `len(results) > 0` guards.
- **The settlement-authority transfer is the real work** (claude, cursor, devin, codex, verified):
  `choir.Complete` becomes a mailbox envelope today; mission one owner-settled that mailbox Complete
  never creates a terminal. Retiring `record_assignment_result` therefore **supersedes a mission-one
  owner clause** and needs ratification at charter, not a silent overlay change.
- **Validation parity gap** (devin): `update_coagent` carries far more caller/target/policy
  validation than the staged message path, so retiring it retires the checks unless the reducer
  gains them.
- **Verifier slot gap** (devin): verifier-gated capabilities need the durable slot in session scope.
- **Rollback weakening** (devin): deleting the four capsule ops from the shared registry also
  removes them from the `actuator=tools` route that mission one preserved as a fallback.
- **Fifth `CleanGoSource` call site** (claude, glm53; verified): the broker worker path
  `session_worker.go:392` was missing from the brief.
- **Pre-declared failure responses** (claude): the Definition must say what happens when a roster
  model fails, before results are seen.

## Adjudications

**(a) Provider preparation: separate sequence, lands before charter.** Reason: distinct mutation
class, protected surface, credential path and rollback; keeps provider flakiness out of prompt
signal; matches the overview's "only preparatory act". May run beside mission two's closeout.

**(b) One mission, two tiers, strictly serialized slices.** Simplification first Define; T1
(substitutes exist: four capsule ops, bundle inspect) before T2 (no substitute: the four settlement
tools). Splitting into two missions leaves the dual path alive or lands deletion without proof.

**(c) Texture design inside, non-gating.** Design artifact only; no runtime code; mission three's
completion does not wait on it.

**(d) Replay proof per retired operation.** Freeze a mapping table first (old name → in-cell
function → intent kind → receipt class → canonical vs excluded fields → fixture). Then per
operation: capture the golden receipt before cutover (canonical input, semantic identity, exact
receipt bytes/ref/digest, effect census); deploy and force an actor/host rewarm; invoke the
successor under the same semantic identity and require exact canonical equality with zero extra
effects; reuse the identity with changed canonical input and require a pre-effect conflict; use a
new identity and prove a fresh operation. Read/list/inspect additionally mutate the underlying
state between calls: same identity returns the original observation, a new identity observes the
change. Evidence: `local_test` harness, then one `deployed_proof` per operation class.

**(e) One-prompt proof and falsification.** Freeze the resolved affordance component and the REPL
initialization manifest **before** seeing model results. Gate: structural invariant (no repair, one
required parameter, prompt digest identical per desk, CI guard against model-id conditionals).
Evidence: the verified roster run — ≥5 models, ≥3 vendors, all three wire shapes, one frozen desk
task, no per-model fork, no repair, no extra hints. Falsified by any prompt fork, any restored
repair, any model-id branch, or a model that passes only with extra schema hints or retries.
Pre-declared responses to a failing model: exclude the model and record it, or fix the shared prompt
for every model — never a fork.

**(f) Cache-mission exit.** The session identity travels on the wire outside the cache key; the
static affordance body carries no timestamps; prompt assembly order is unchanged; the eval schema
version is recorded; no per-model prompt forks. No optimization work or hit-rate claim belongs to
this mission.

## Unverified / carried assumptions

- The live provider facts (session-header enforcement, roster call results, image-input matrix) are
  from the 2026-09-10 research note; panelists were asked to treat them as given and several
  explicitly flagged them as unverified by them. They must be re-pinned at the prep Definition.
- `update_coagent` citer inventory, the verifier-slot gate and the `actuator=tools` coupling are
  panel assertions that the mission's operation-equivalence freeze must verify before deletion.
- Texture's eight canonical-writer invariants were not in evidence for the panel; the design
  artifact must cite them from their own authority.

## Recommendation

Charter mission three only after (i) mission two reaches terminal deployed acceptance, (ii) the
OpenCode provider preparation lands with its own Landing Loop, and (iii) the owner ratifies the
settlement-authority transfer that retiring `record_assignment_result` requires. Run the first
Define as a **code-free** boundary: it records the three defects above, freezes the nine-operation
mapping table and the prompt/REPL manifest, and states the falsifiers. Repair code follows the
problem record, never precedes it.

## Owner override, 2026-09-11

Two adjudications above are overridden by the owner and the Definition now follows the owner:

- **(a) provider preparation as a separate sequence** is withdrawn. OpenCode Go and Zen setup is
  phase 1 inside this mission. It still keeps its own rollback (the stub adapters), its own
  Landing Loop, and its own problem record, so an ops failure is still not confused with a prompt
  failure.
- **the `actuator=tools` rollback question is closed.** That branch is deleted, not preserved. No
  rollback logic will be used. The deletion still waits for a proven RLM, and the bar for "proven"
  is the owner's call.
