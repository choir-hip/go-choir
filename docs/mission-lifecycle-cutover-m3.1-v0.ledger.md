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

Receipt: `docs/archive/mission-lifecycle-cutover-m3.1-v0.md`.

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
`docs/archive/mission-lifecycle-cutover-m3.1-v0.md`.

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
recovery gate note to `docs/archive/mission-lifecycle-cutover-v0.md`.

Expected Delta V: -1 against handoff ambiguity. Actual Delta V: -1. M3.1 is
ready to execute as the active preflight mission; the code/test recovery V
remains 8.

Receipt: `docs/mission-graph.yaml`,
`docs/archive/mission-lifecycle-cutover-m3.1-v0.md`,
`docs/archive/mission-lifecycle-cutover-v0.md`.

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

## 2026-06-14 - Repair Runtime Package Deploy Impact Locally

Claim: the deployed overclaim was caused by deploy topology, not by the
run-acceptance logic in the pushed source. Authenticated product-path runs can
exercise a user computer whose sandbox runtime package is served through
vmctl's `/internal/vmctl/runtime-package/sandbox` endpoint. The deploy for
`27af4f2f6cf9caddc8fc3ae0ea96d5dbbdc1428a` refreshed sandbox/gateway and
reported matching build identity, but did not restart vmctl, so newly booted
user computers could still receive the old sandbox runtime package.

Move: repair / contain. Changed deploy-impact classification so sandbox runtime
package changes, including `internal/runtime/*`, mark `deploy_vmctl_restart`.
Added classifier coverage for `internal/runtime/run_acceptance.go`, and widened
the comprehensive run-acceptance test to protect both
`mission-lifecycle-cutover-v0` and `mission-lifecycle-cutover-m3.1-v0` against
prompt/VText-only acceptance.

Expected Delta V: -1 locally. Actual Delta V: -1 locally. M3.1 local V is 0,
but settlement remains pending until the repair is committed, pushed, deployed
with vmctl restart, and re-proved through the deployed acceptance synthesis
probe.

Receipts:
- `.github/scripts/deploy-impact-classify-test` passed.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'Test(RunAcceptanceSynthesizeDoesNotAcceptPromptVTextOnlySmoke|HandleRunStatusPublicIncludesTrajectoryEvidence)' -count=1` passed.
- `nix develop -c scripts/go-test-runtime-shards` passed.

Open edge: deploy and product-path proof remain mandatory. The next deployed
acceptance synthesis probe must return `staging-smoke-level/blocked` for M3.1
prompt/VText-only smoke before this mission can settle.

## 2026-06-14 - Settle M3.1 Lifecycle Recovery

Claim: M3.1 has completed its regression recovery purpose. VText remains an
agentic canonical-document owner rather than a deterministic workflow stepper,
generic semantic `next_required_tool` is no longer trusted, prompt-bar
researcher intent no longer routes runtime control, M3 handoff no longer points
at deterministic researcher continuation, and lifecycle acceptance no longer
settles from prompt/VText smoke.

Move: settle / handoff. Pushed the deploy-impact repair at
`aa7279f74adccd81ddd96356e29994a584442991`, ran CI, then manually dispatched
CI run `27514505833` with `force_staging_deploy=true` because the repair commit
itself changed only workflow/test/docs files while the stale runtime-package
state needed a real vmctl refresh. The deploy reported
`deploy_vmctl_restart=true`, restarted vmctl, refreshed three active
interactive computers, and completed staging health checks. `https://choir.news/health`
then reported proxy and sandbox at `aa7279f74adccd81ddd96356e29994a584442991`.

Expected Delta V: settle at V=0. Actual Delta V: V=0. No forced semantic VText
delegation remains in the rollback scope, no generic semantic
`next_required_tool` control remains in the rollback scope, and deployed
acceptance synthesis now blocks M3.1 prompt/VText-only smoke.

Receipts:
- Push CI run `27514456013` succeeded for
  `aa7279f74adccd81ddd96356e29994a584442991`.
- Manual forced deploy CI run `27514505833` succeeded.
- Deploy log: `deploy_vmctl_restart=true`; `Phase vmctl restart: 1s`; active
  interactive computers `vm-6a5ec0aa6ed8d9ae77de95ab660c532a`,
  `vm-e4bfbd6e51aa7e11b9a481c7245cfe51`, and
  `vm-d01a2bdc8b486a710b795e1ffb8d06ff` refreshed.
- `https://choir.news/health` reported proxy and sandbox
  `deployed_commit=aa7279f74adccd81ddd96356e29994a584442991`.
- `GO_CHOIR_RUN_DEPLOYED_LIFECYCLE=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news pnpm --dir frontend exec playwright test tests/adaptive-lifecycle-control-deployed.spec.js --project=chromium --reporter=list`
  passed.
- Deployed acceptance synthesis for trajectory
  `2487ebb5-4087-47fa-a44e-d36d204fe84b` returned
  `runacc-8b635aa7aa2fe7098d7a` as `staging-smoke-level/blocked`, with only
  `submitted` and `vtext_opened` checkpoints.

Settlement: M3.1 is settled. Resume M3 proper from
`docs/archive/mission-lifecycle-cutover-v0.md`. Actor memory cross-trajectory scoping
remains a named successor edge, not a blocker for this recovery.

## 2026-06-14 - Reopen Prompt-Pipeline Forcing Blocker

Claim: the prior V=0 settlement claim was premature. Runtime
`next_required_tool` forcing was removed, but VText prompt-pipeline wording
still mandates semantic delegation by telling VText to call `spawn_agent` or
`request_super_execution` in the same run. This violates the VText agentic
invariant even without a hard tool-loop continuation.

Move: observe / document. Recorded the blocker before code changes, per Problem
Documentation First. The repair scope is narrow: soften prompt language from
mandatory role/tool sequences into affordance/obligation language while keeping
grounded factual safety, then update tests to assert non-forcing language and
absence of "call spawn_agent now / in this run" semantics.

Expected Delta V: +1 for reopening a real blocker. Actual Delta V: +1. M3.1
returns to V=1 until prompt-pipeline forcing is repaired and verified.

Receipts:
- `internal/runtime/vtext_agent_revision.go` still contains "call spawn_agent
  with role=\"researcher\" in this run", "followed by a researcher spawn in the
  same run", and semantic `request_super_execution` forcing phrases.
- `internal/runtime/vtext_prompt_unit_test.go` and
  `internal/runtime/vtext_test.go` assert mandatory wording.

Open edge: next move is a scoped code/test repair, then focused VText
prompt/tool-loop/API tests, runtime shards, independent review, and a corrected
settlement update.

## 2026-06-14 - Repair Prompt-Pipeline Forcing Locally

Claim: the follow-up blocker is repaired locally. VText prompt-pipeline wording
now presents researcher and super delegation as affordances VText may choose
within its authority envelope, not as mandatory semantic tool sequences.
Grounded safety remains: VText may not write factual/current/source claims from
model priors, but may write uncertainty-bearing working revisions and record
the missing evidence or source representation as a blocker.

Move: repair / verify. Softened mandatory `spawn_agent` and
`request_super_execution` language in `internal/runtime/vtext_agent_revision.go`.
Updated prompt tests to assert non-forcing language, broadened the denylist for
imperative researcher/super sequencing, and added media-source prompt coverage
for represented-evidence-or-blocker behavior.

Expected Delta V: -1 locally. Actual Delta V: -1 locally. M3.1 local V is 0,
but settlement remains pending until this runtime repair is committed, pushed,
deployed, and re-proved through staging identity and deployed lifecycle /
acceptance proof.

Receipts:
- Static phrase scan found forced semantic-delegation phrases only in the test
  denylist, not in VText prompt source.
- `nix develop -c go test ./internal/runtime -run 'Test(VTextPromptInitialRevisionUsesSingleWriterLoop|VTextPromptForFactualFirstRevisionForbidsUngroundedContent|VTextPromptPrioritizesSuperAfterResearchForMixedObligation|VTextPromptSteersCurrentEventsToResearcherNotSuper|VTextPromptExplicitResearcherExposesAffordanceWithoutForcing|BuildAgentRevisionRequestRequiresSuperContinuationForActiveWorker|VTextAgentRevisionRegistersMediaSourceRefs|RunToolLoopRequiredNextTool|RunToolLoopIgnoresSemanticRequiredNextToolFromUntrustedProducer|EditVTextInitialContinuationDoesNotSmuggleRequiredTool|EditVTextExplicitResearcherDoesNotForceSpawnContinuation|EditVTextExplicitResearcherDoesNotForceSpawnAfterSuperBase|EditVTextExplicitResearcherFromBaseRevisionContentSurvivesWorkerPrompt|EditVTextExplicitResearcherFromSeedPromptSurvivesRequestIntent|EditVTextExplicitResearcherDoesNotDuplicateExistingResearcher|HandlePromptBarResearcherMentionDoesNotSetRoutingFlag|InitialVTextToolChoice)' -count=1`
  passed.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'Test(RunAcceptanceSynthesizeDoesNotAcceptPromptVTextOnlySmoke|HandleRunStatusPublicIncludesTrajectoryEvidence)' -count=1`
  passed.
- `nix develop -c scripts/go-test-runtime-shards` passed after the final prompt
  wording changes.
- Independent review found no blocking remaining prompt-forcing issue after the
  media-source and grounded-history follow-up fixes.

Open edge: commit/push/CI/deploy/staging proof remains mandatory before
settlement is re-claimed.

## 2026-06-14 - Settle M3.1 Prompt-Pipeline Follow-up

Claim: M3.1 is settled again after the review follow-up. The runtime
prompt-pipeline no longer mandates semantic VText delegation through researcher
or super tool sequences, tests protect the non-forcing invariant, and the
repaired runtime commit was deployed and proved on staging.

Move: settle. Committed and pushed
`0ab8bd6a20f09ad38ef0c4c7293d42bbf8845efe`, monitored CI/deploy, verified
staging build identity, reran deployed lifecycle proof, and synthesized a
deployed M3.1 prompt/VText-only acceptance record.

Expected Delta V: settle at V=0. Actual Delta V: V=0. No forced semantic VText
delegation remains in runtime prompt-pipeline wording for the rollback scope,
no generic semantic `next_required_tool` control remains in the rollback scope,
and deployed acceptance synthesis still blocks M3.1 prompt/VText-only smoke.

Receipts:
- Push CI run `27515490562` succeeded for
  `0ab8bd6a20f09ad38ef0c4c7293d42bbf8845efe`.
- Deploy log reported `deploy_vmctl_restart=true`, host services
  `gateway,sandbox`, `Phase vmctl restart: 11s`, and active sandbox runtime
  hot-refresh for five interactive computers.
- `https://choir.news/health` reported proxy and sandbox
  `deployed_commit=0ab8bd6a20f09ad38ef0c4c7293d42bbf8845efe`.
- `GO_CHOIR_RUN_DEPLOYED_LIFECYCLE=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news pnpm --dir frontend exec playwright test tests/adaptive-lifecycle-control-deployed.spec.js --project=chromium --reporter=list`
  passed.
- Deployed acceptance synthesis for trajectory
  `97d6cf3b-0fc9-4ba3-8db1-5c6b28042c33` returned
  `runacc-98a84912f02bcb4e0f82` as `staging-smoke-level/blocked`, with only
  `submitted` and `vtext_opened` checkpoints.

Settlement: M3.1 is settled. Resume M3 proper from
`docs/archive/mission-lifecycle-cutover-v0.md`. Actor memory cross-trajectory scoping
remains a named successor edge, not a blocker for this recovery.
