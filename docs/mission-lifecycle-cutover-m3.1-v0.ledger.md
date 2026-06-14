# Mission M3.1 Ledger

## 2026-06-13 - Open Recovery Paradoc

Claim: the M3 regression is not an implementation bug in
`requiredContinuationAfterVTextEdit`, but a proof/proxy capture: the acceptance
precondition requiring researcher participation before vmctl refresh leaked into
runtime semantics and paradoc next moves.

Move: shift / document. Created M3.1 paradoc as the new source program for
regression recovery. It records the aggregate review, answers how the pivot
happened, and names immediate and long-term repair paths.

Expected Delta V: define V and reduce ambiguity around the next move. Actual
Delta V: M3.1 V initialized at 9; no code fixes yet. The main gain is observer
shift from "debug deterministic continuation" to "remove forced workflow and
repair acceptance witness."

Receipt: `docs/mission-lifecycle-cutover-m3.1-v0.md`.

Open edge: no tests or code changes in this pass. The next pass must make the
documentation checkpoint durable in git before behavior-changing fixes, then
run focused tests and runtime shards.

## 2026-06-13 - Promote VText Agentic Invariant

Claim: VText semantics are fragile enough and central enough that they must be
contractual doctrine, not inferred from scattered prompts or past behavior.

Move: construct / doctrine. Added `docs/vtext-agentic-invariants-2026-06-13.md`
and updated `AGENTS.md` so future workers must read the invariant before
touching VText tools, prompts, routing, revision creation, coagent wake
behavior, Trace/VText projection, VText run acceptance, or VText-backed
missions.

Expected Delta V: -1 by resolving the missing shared VText invariant. Actual
Delta V: -1. M3.1 V moves from 9 to 8. Code still violates the invariant until
the forced researcher continuation and related tests are removed.

Receipt: `docs/vtext-agentic-invariants-2026-06-13.md`, `AGENTS.md`,
`docs/mission-lifecycle-cutover-m3.1-v0.md`.

Open edge: documentation is necessary but not sufficient. Next move remains a
behavior rollback: remove VText researcher hard continuation, narrow
required-next-tool, rewrite tests, and verify with focused runtime tests plus
runtime shards.

## 2026-06-14 - Make M3.1 Ready As Active Graph Node

Claim: after docs truth v1, a ready paradoc must be discoverable in the mission
graph and must carry a copy-pasteable Suggested Goal String, not just a terse
path stub.

Move: construct / handoff. Added `m3.1-lifecycle-recovery` to
`docs/mission-graph.yaml`, made M3 proper depend on it, marked M3 proper
blocked in the graph, added a full Suggested Goal String here, and added a
recovery gate note to `docs/mission-lifecycle-cutover-v0.md`.

Expected Delta V: -1 against handoff ambiguity. Actual Delta V: -1. M3.1 is
ready to execute as the active preflight mission; the code/test recovery V
remains 8.

Receipt: `docs/mission-graph.yaml`,
`docs/mission-lifecycle-cutover-m3.1-v0.md`,
`docs/mission-lifecycle-cutover-v0.md`.

## 2026-06-14 - Remove Forced Semantic VText Continuation Locally

Claim: the smallest rollback batch can repair the regression without adding a
new role-specific harness branch. VText should retain delegation affordances,
but runtime must not force researcher or super continuation from semantic prompt
text.

Move: repair / contain. Removed the VText researcher hard continuation from
`edit_vtext`, kept the email draft handoff as a bounded app protocol, narrowed
`next_required_tool` handling to the typed worker VM lease/start protocol,
deleted prompt-bar researcher routing intent, exposed trajectory/work evidence
on browser-public run status, prevented prompt/VText-only smoke from accepting a
run acceptance record, and updated M3 handoff away from deterministic researcher
continuation.

Expected Delta V: -8 for the local rollback batch. Actual Delta V: -8 locally.
M3.1 local rollback V is 0, with settlement still pending commit/push,
CI/deploy, staging identity, and deployed lifecycle evidence.

Receipts:
- `nix develop -c go test ./internal/runtime -run 'Test(EditVTextInitialContinuationDoesNotSmuggleRequiredTool|EditVTextExplicitResearcherDoesNotForceSpawnContinuation|EditVTextExplicitResearcherDoesNotForceSpawnAfterSuperBase|EditVTextExplicitResearcherFromBaseRevisionContentSurvivesWorkerPrompt|EditVTextExplicitResearcherFromSeedPromptSurvivesRequestIntent|EditVTextExplicitResearcherDoesNotDuplicateExistingResearcher|HandlePromptBarResearcherMentionDoesNotSetRoutingFlag|RunToolLoopRequiredNextTool|RunToolLoopIgnoresSemanticRequiredNextToolFromUntrustedProducer|HandleRunStatusPublicIncludesTrajectoryEvidence|RunAcceptanceSynthesizeDoesNotAcceptPromptVTextOnlySmoke|RunAcceptanceSynthesizeAcceptsRuntimeSupervisionWithoutAppPackage|InitialVTextToolChoice)'` passed.
- `nix develop -c scripts/go-test-runtime-shards` passed after the runtime
  changes.
- Independent review reported no blockers and two P3 cleanup findings; both
  were repaired, then
  `nix develop -c go test ./internal/runtime -run 'Test(EditVTextExplicitResearcherDoesNotForceSpawnContinuation|RunToolLoopRequiredNextToolUsesRequiredChoice|RunToolLoopIgnoresSemanticRequiredNextToolFromUntrustedProducer|InitialVTextToolChoiceUsesExactTools)'`
  passed.

Open edge: final settlement is external to this local proof. The next move is
commit, push, CI/deploy monitoring, staging identity verification, and deployed
lifecycle evidence. Actor memory cross-trajectory scoping remains a named
successor edge, not a blocker for this rollback.

## 2026-06-14 - Deployed Acceptance Overclaimed M3.1 Smoke

Claim: the local run-acceptance repair did not cover every deployed synthesis
path. The deployed M3.1 acceptance endpoint still accepted prompt/VText-only
smoke for the new mission id.

Move: observe / document. After commit
`27af4f2f6cf9caddc8fc3ae0ea96d5dbbdc1428a` reached staging, public health
reported proxy and sandbox deployed at that commit and the deployed adaptive
lifecycle Playwright proof passed. A separate authenticated product-path
submission then called `/api/run-acceptances/synthesize` for
`mission-lifecycle-cutover-m3.1-v0` with trajectory
`4e28d8aa-34fc-42ca-a5e8-64620f6e888f`. The endpoint returned
`runacc-94d318d49e2ba66a99ce` at `staging-smoke-level/accepted` with only
`submitted` and `vtext_opened` checkpoints.

Expected Delta V: 0 if deployed synthesis matched the local invariant. Actual
Delta V: +1. The forced VText continuation rollback still reached staging, but
settlement is blocked until M3.1 acceptance synthesis no longer accepts
prompt/VText smoke.

Receipts:
- CI run `27514147770`, Docs Truth run `27514147777`, and FlakeHub publish run
  `27514147780` succeeded.
- `https://choir.news/health` reported proxy and sandbox
  `deployed_commit=27af4f2f6cf9caddc8fc3ae0ea96d5dbbdc1428a`.
- `GO_CHOIR_RUN_DEPLOYED_LIFECYCLE=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news pnpm --dir frontend exec playwright test tests/adaptive-lifecycle-control-deployed.spec.js --project=chromium --reporter=list`
  passed.
- Deployed acceptance synthesis returned
  `runacc-94d318d49e2ba66a99ce` as `staging-smoke-level/accepted` for trajectory
  `4e28d8aa-34fc-42ca-a5e8-64620f6e888f`, proving the overclaim remains on at
  least one deployed mission-id path.

Open edge: first next move is a code fix, but only after this documentation
checkpoint commit. Add local coverage for the M3.1 target mission id and rerun
the deployed acceptance synthesis to verify `staging-smoke-level/blocked`.
