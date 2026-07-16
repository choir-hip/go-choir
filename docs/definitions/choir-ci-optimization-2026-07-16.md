---
title: "CI Re-Enablement and Evidence-Bounded Optimization"
definition_version: 2
draft: false
executable: true

start:
  captured_at: 2026-07-16T04:43:23Z
  source:
    canonical_ref: refs/remotes/origin/claude/ci-optimization-mission-ttur4k@0e5f5c7eb531dd880378996ebd280fe69bb4cf09
    origin_main_ref: refs/remotes/origin/main@d87bdc446ecc28585c3bc08d4d469b9f94d3c246
    relation: "PR #53 is one docs-only commit ahead of origin/main"
    deploy_identity: not_applicable_ci_only
  worktree_inventory:
    status: reconciled
    evidence_ref: "git status --short --branch and git worktree list --porcelain at 2026-07-16T04:43:23Z"
    preservation_rule: "Touch only this child worktree and CI/mission paths admitted below. Never touch or copy the parent main worktree's dirty Autoputer Definition."
  worktrees:
    - path: /Users/wiz/.codex/worktrees/8c9a/go-choir
      status: clean
      class: goal_candidate
      owner: ci-mission
      touch: goal_owned
      paths_or_digest: "branch codex/ci-optimization-mission-2026-07-16 at 0e5f5c7e"
      recovery: "Delete the child branch/worktree after preserving any accepted commits."
    - path: /Users/wiz/go-choir
      status: dirty
      class: other_agent_wip
      owner: autoputer-mission
      touch: forbidden
      paths_or_digest: "M docs/definitions/choir-audited-autoputer-construction-2026-07-15.md"
      recovery: "Leave in place on main; this CI mission must not read-modify-write it."
    - path: /Users/wiz/go-choir-autoputer-v2
      status: clean
      class: other_agent_wip
      owner: autoputer-mission
      touch: forbidden
      paths_or_digest: "branch autoputer-definition-v2 at fc6e65a7"
      recovery: "Leave in place."
  candidates:
    - id: ci-reenable-candidate-1
      ref: branch:codex/ci-optimization-mission-2026-07-16
      base: 0e5f5c7eb531dd880378996ebd280fe69bb4cf09
      scope: [.github/workflows/ci.yml, .github/workflows/race.yml, .github/scripts/build-sboms-differential, .github/scripts/ci-impact-classify, scripts/go-test-runtime-shards]
      disposition: active
      evidence_ref: none_yet
  observed_artifact:
    - claim: "Race and differential SBOM are paused by literal false conditions in ci.yml."
      evidence_ref: ".github/workflows/ci.yml at 0e5f5c7e"
    - claim: "Commit c96c7b49 already replaced nested-Nix SBOM generation with pinned host-side sbomnix, and main run 29295978398 job 86971008658 succeeded on that topology in 13m15s."
      evidence_ref: "docs/evidence/ci-optimization-baseline-2026-07-16.md"
    - claim: "Before pause commit 1b28520, either high_risk_race or sampled_race invoked the entire reusable race workflow: four runtime shards and one non-runtime job. There is no sampled-shard input."
      evidence_ref: "1b28520^:.github/workflows/ci.yml plus run 29295978398"
    - claim: "SBOM runs after check and is not a check or deploy prerequisite; it is currently non-blocking supply-chain audit evidence."
      evidence_ref: ".github/workflows/ci.yml jobs check, sbom, and deploy-staging at 0e5f5c7e"
    - claim: "CI-only .github/** changes are ignored by deploy-impact and should skip Node B deployment."
      evidence_ref: ".github/scripts/deploy-impact-classify and its contract test at 0e5f5c7e"
    - claim: "The parent workflow concurrency group ci-${github.ref} and reusable/scheduled Race group race-${github.ref} each cancel an in-flight run on the same ref, so concurrent mission execution is scope-disjoint but main CI and main Race observations are shared serialized resources."
      evidence_ref: ".github/workflows/ci.yml and .github/workflows/race.yml concurrency at 0e5f5c7e"
  unknowns:
    - "No authorized billing or metrics authority is available; actual billed minutes are unknown."
    - "Whether cache reuse across prior runs, serialized priming, or same-run artifact transfer is the best compile-cost candidate; they are separate experiments."
    - "Whether any optimization beyond re-enabling the already-proved lanes has matched-run critical-path benefit."

finish:
  deliver: "Restore the paused race and differential-SBOM safety signals without changing application/platform behavior or weakening CI, deploy, or publishing protection, then admit only optimizations proved to reduce GitHub Actions duration on matched runs."
  artifact: "A frozen, reviewed CI-only candidate and landed GitHub Actions workflow in which classifier selection invokes the complete reusable race workflow, the already-wired host-side SBOM job runs after successful check on selected main pushes, and deploy-impact still skips CI-only changes. Hosted proof is split by actual stimulus: PR parsing, CI-only main SBOM/deploy-skip behavior, and a separately selected post-land race run."
  acceptance:
    - action: "Fetch GitHub run 29295978398 and current paused run 29468123745 job timelines."
      proves: "The baseline topology, durations, full five-job race fan-out, successful host-side SBOM, and present paused state are observed rather than inferred."
      evidence_class: github_actions_timeline
    - action: "Run classifier, SBOM differential, deploy-impact, workflow-contract, shell-syntax, YAML, and doc truth checks against the candidate."
      proves: "The candidate preserves local deterministic contracts, including CI-only deploy skip; it does not prove hosted execution."
      evidence_class: deterministic_local_contracts
    - action: "Freeze the candidate by base, commit, scoped paths, and digest; obtain independent review before treating it as ready."
      proves: "Review addresses an immutable candidate and can reject scope, protection, evidence, or rollback defects."
      evidence_class: frozen_candidate_review
    - action: "Run the candidate on a pull request and record hosted parsing plus every job conclusion. For this .github-only PR stimulus, record go=false, high_risk_race=false, race unselected, SBOM skipped because the event is not a main push, and deploy-impact skipped because it is not a main event."
      proves: "The candidate parses and executes on the hosted platform; it does not prove live SBOM, deploy-impact output, or race selection."
      evidence_class: github_actions_candidate_parse
    - action: "Immediately before the PR merge, query GitHub Actions for active main CI and Race runs. Record either no active run or an owner-coordinated window, record the expected merge SHA/event, and wait for that main CI receipt before another CI or Race observation on main."
      proves: "Same-ref cancellation cannot silently supersede unrelated main CI or Race evidence during this landing."
      evidence_class: github_actions_serialization
    - action: "After an explicitly owner-authorized, serialized CI-only main landing, observe check success, SBOM success, fetch and verify the complete checksummed SBOM artifact and differential counts, observe deploy-impact report deploy_needed=false, and observe deploy-staging skipped. Record race as selected or unselected from the actual landing SHA without requiring it for this receipt."
      proves: "The canonical CI-only stimulus restores host-side SBOM and preserves the Node B skip and post-check non-blocking relationship; it does not by itself prove race selection."
      evidence_class: canonical_ci_only_sbom_acceptance
    - action: "Separately observe the first post-land main push that the unchanged classifier actually selects via high_risk_race=true or push+go=true+sampled_race=true; require all five reusable race jobs and the parent check gate to succeed. Do not manufacture an app/platform change. A scheduled or separately owner-authorized direct race.yml run may prove the five-job workflow but cannot substitute for parent ci.yml selector/check proof."
      proves: "The restored parent classifier route selects the complete reusable race workflow and binds its result into check."
      evidence_class: post_land_race_selector_acceptance
  rollback: "Revert the bounded CI commit to restore the two literal pause conditions. The owner authorizes PR creation and PR-mediated main merge; a Node B deployment is admissible only through a later recorded, concrete CI deploy slice if the accepted path requires it. Retain protected relationships and coordinate both cancellation groups; generic workflow dispatch and direct Node B mutation remain excluded."
  landing:
    required: true
    environment: github_actions
    required_receipts: [pushed_candidate_commit, frozen_candidate_review, github_actions_candidate_parse, main_concurrency_window, owner_authorized_serialized_main_landing, canonical_ci_only_sbom_acceptance, post_land_race_selector_acceptance]
  not_done_when:
    - "Only local checks, a draft, or a reviewer narrative exists."
    - "A sampled condition is described as one shard even though it invokes the full reusable workflow."
    - "A duration or rounded runner-minute estimate is called actual billing."
    - "SBOM is described as gating check or deploy without a separately authorized red relationship change."
    - "One .github-only landing is claimed to prove both SBOM and race even though that stimulus sets go=false and high_risk_race=false and race depends on the landing SHA's sample bucket."
    - "A direct main push, generic workflow dispatch, direct Race dispatch, or direct Node B mutation occurs outside this Definition's recorded route and both relevant cancellation groups are not coordinated."

boundaries:
  mutation_class: red
  class_rationale: "The Definition/registry boundary is green. Re-enabling race and SBOM changes protected CI assurance and supply-chain publishing conditions, so implementation uses red ceremony even though it changes no app/platform runtime. The bounded doccheck cardinality repair below is yellow documentation-truth tooling, sequenced only after its observed problem record."
  conjecture_delta: "The pauses are reversible conditions over already-wired, previously successful lanes; restoration should reuse that topology. Optimization candidates must be separated by mechanism and admitted only by matched critical-path evidence."
  heresy_delta:
    discovered: ["The draft falsely treated sampled_race as partial-matrix execution, described the repaired SBOM topology as unwired, conflated cache reuse with same-run compilation sharing, implied SBOM gated deployment, and used public duration as billing proof.", "The mission graph permits one scope-disjoint CI maintenance entrypoint alongside the product spine, but doccheck still hard-codes one total graph entrypoint and rejects the live registry."]
    introduced: []
    repaired: ["The live Definition corrects those claims before workflow mutation and the SBOM problem record now names c96c7b49 plus its successful run."]
  authority_sources:
    - "Owner delegation and supervision in Codex task 019f6933-ea56-7403-b535-da4eafa4d1f7 on 2026-07-16: promote and run this independent CI-only mission concurrently; no app/platform changes; serialize shared main CI and Race observations."
    - "Owner authority in Codex task 019f6933-ea56-7403-b535-da4eafa4d1f7 on 2026-07-16: create PRs, merge this CI-only change to main, and deploy Node B when the accepted landing path requires it."
    - docs/choir-doctrine.md
    - AGENTS.md
    - docs/standing-questions.md
    - skills/definition/SKILL.md
  must_preserve:
    - "Autoputer remains the sole executable product Definition; this is a scope-disjoint CI-maintenance entrypoint, not a competing product mission."
    - "The parent dirty Autoputer Definition and all app/platform source are untouched."
    - "high_risk_race and sampled_race each select the entire reusable race workflow unless a new reviewed workflow input is designed and proved."
    - "SBOM remains post-check, main-push-only, and non-blocking relative to deploy."
    - "check, deploy-impact, deploy-staging, FlakeHub, docs-only routing, and supply-chain artifact integrity are not weakened."
    - "Deploy classifier/contract tests remain in deploy-impact unless equivalent coverage on every non-doc main push is designed and proved."
    - "Ripgrep setup is measured as conditional; no claim says it is fetched every run."
    - "The landing route is draft PR, hosted evidence, a recorded main concurrency window, and PR-mediated merge. Direct push to main remains outside this Definition even though the owner authorized main landing."
    - "Normal CI-only landing must prove deploy_needed=false and Node B skipped. The owner has authorized a Node B deployment only if a later Define slice names the exact accepted CI deploy path, event, verification, and rollback; generic workflow dispatch and direct Node B mutation remain excluded now."
    - "Main landing and separate main Race observation are coordinated against ci-${github.ref} and race-${github.ref}; force-push remains forbidden."
  excluded: [app_source, platform_source, direct_main_push, direct_node_b_mutation, workflow_dispatch, direct_race_dispatch, force_push, new_runner_provider, billing_claim_without_authority]
  protected_surfaces: [ci_check_gate, race_lane, sbom_audit_and_publishing, deploy_impact_classifier, deploy_staging, flakehub_publish, docs_only_fast_path, main_ci_concurrency, main_race_concurrency]
  completion_evidence_floor: [github_actions_timeline, deterministic_local_contracts, frozen_candidate_review, github_actions_candidate_parse, github_actions_serialization, canonical_ci_only_sbom_acceptance, post_land_race_selector_acceptance]
  scoped_yellow_repair:
    reason: "The live mission graph and manifest deliberately admit the scope-disjoint CI /goal, but strict doccheck still rejects that documented topology."
    mutation_class: yellow
    admitted_paths: [cmd/doccheck/main.go, cmd/doccheck/main_test.go]
    problem_ref: docs/problems/ci-maintenance-entrypoint-doccheck-cardinality-2026-07-16.md
    sequencing: "Commit this problem/documentation boundary before changing the checker; then require focused Go tests and a live doccheck pass."
    rollback: "Revert only the checker/test change, restoring the prior single-entrypoint contract, if it admits an invalid product or maintenance shape."
    excluded_effects: [product_authority_change, app_or_platform_behavior, generic_workflow_dispatch, node_b_deployment]

measures:
  - name: hosted critical path
    kind: telemetry
    baseline: "24m15s pre-pause full-signal run 29295978398; 5m53s current paused run 29468123745; change sets differ"
    desired: "lower on matched candidate runs without losing selected evidence"
    decision_use: "Rank separately scoped optimization candidates."
    cannot_prove: "Correctness, actual billed cost, or causality across unmatched change sets."
  - name: runner duration
    kind: telemetry
    baseline: "85m15s summed raw job duration for run 29295978398; 31m51s for run 29468123745"
    desired: "lower on matched runs"
    decision_use: "Identify expensive lanes and setup steps."
    cannot_prove: "Actual billing; rounded runner minutes are estimates only."
  - name: assurance preservation
    kind: gate
    baseline: "Race paused; SBOM paused; check/deploy/publish relationships otherwise intact"
    desired: "Selected full race and host-side SBOM restored with protected relationships unchanged"
    decision_use: "Reject any candidate that narrows or misstates assurance."
    cannot_prove: "Absence of all CI defects."

now:
  status: working
  slice: "standalone doccheck topology repair (PR 1), then frozen workflow restoration (PR 2)"
  question: "Can PR 1 validate the scope-disjoint entrypoint without blurring the frozen workflow stimulus, and can its FlakeHub publication side effect receive explicit authority before any main merge?"
  reconciliation:
    observed_at: 2026-07-16T06:00:00Z
    source_ref: "candidate 8e4aa074f970b69ce59cffa07b280f164ca1c161 reviewed against origin/main a1d2f88c6a7135c8a1db916b6fb4f00acf43fb36; fresh fetch is required immediately before rebase"
    deploy_identity: not_applicable_ci_only
    authority_identities: [owner-delegation-019f6933-2026-07-16, choir-doctrine@a1d2f88c, AGENTS@a1d2f88c, definition-skill@a1d2f88c]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "isolated /private/tmp/go-choir-ci-goal at 6b935c58; protected parent Autoputer inventory unchanged; origin/main a1d2f88c has no CI-scope delta but overlapping mission registries requiring explicit reconciliation"
    status: fresh_fetch_required
  candidate:
    id: ci-reenable-candidate-1
    state: reviewed
    ref: commit:8e4aa074f970b69ce59cffa07b280f164ca1c161
    owner: ci-mission
    base: 27f03875702f503d8ef551035733eb5f40e27a1c
    digest: "candidate ci.yml sha256 f60b63fe5e1613a9f7432f1f7c79bc577a1aee52dac7195ec4de1ad39ee4ee25; pre-candidate ci.yml sha256 69b5de792e84446eb5802ae4f60839bbf8bfb2026e64749c3e9dac6e0f93b14c"
    scope: [.github/workflows/ci.yml]
  scope_amendment:
    id: doccheck-scoped-maintenance-entrypoint-contract
    mutation_class: yellow
    admitted_paths: [cmd/doccheck/main.go, cmd/doccheck/main_test.go]
    problem_ref: docs/problems/ci-maintenance-entrypoint-doccheck-cardinality-2026-07-16.md
    acceptance: "Exactly one working product spine remains the authority-root /goal; at most one working scope-disjoint ci_maintenance entrypoint is admitted only when its manifest identity is current, entry-only, and non-authority-root."
    excluded: [workflow_semantics, app_source, platform_source, node_b_deployment]
  decision:
    selected: "Use two PRs: land the doccheck/registry repair first so it cannot change the frozen workflow candidate's classifier stimulus, then recreate the frozen workflow candidate from fresh main with its digest unchanged."
    kind: authority
    status: settled
    source: owner
    evidence_ref: "owner-message-019f6933-2026-07-16-main-merge-node-b-authority"
    owner_ratification_ref: not_applicable
    recorded_at: 2026-07-16T06:00:00Z
    consequence: "PR 1 may be validated as a draft, but cmd/doccheck/main.go classifies flakehub=true, so its main merge awaits narrow owner authority for the automatic publication despite expected Node B skip. After PR 1, recreate PR 2 from fresh main and preserve the ci.yml digest; it remains .github-plus-docs, flakehub=false, and expected Node B skip."
  evidence_refs: [docs/evidence/ci-optimization-baseline-2026-07-16.md, docs/evidence/ci-reenable-candidate-review-2026-07-16.md, docs/problems/ci-maintenance-entrypoint-doccheck-cardinality-2026-07-16.md, run-29295978398, run-29468123745, commit-c96c7b49, candidate-8e4aa074, origin-main-a1d2f88c]
  blocker_or_risk: "Strict doccheck currently rejects the deliberate scope-disjoint entrypoint; its repair must not admit a second product authority. Its cmd/doccheck path also classifies flakehub=true, so main merge needs narrow publication authority even though deploy-impact skips Node B. CI-only workflow landing cannot deterministically select race, so terminal race proof must await a naturally selected post-land main push or separately owner-authorized stimulus. Parent ci-${github.ref} and Race race-${github.ref} cancellation groups both require coordination."
  next_action: "Commit this publication-side-effect checkpoint, implement and validate the bounded yellow doccheck contract, create PR 1 from fresh main, and obtain hosted evidence. Do not merge PR 1 until the owner grants narrow FlakeHub publication authority. Then recreate the frozen workflow candidate as PR 2 from fresh main with its ci.yml digest unchanged."

receipts:
  - id: corrected-problem-and-baseline-define
    boundary: define
    commit_or_artifact: 27f03875702f503d8ef551035733eb5f40e27a1c
    proof_refs: [docs/evidence/ci-optimization-baseline-2026-07-16.md, docs/problems/ci-sbom-nested-nix-sandbox-2026-07-09.md]
    rollback_ref: 0e5f5c7eb531dd880378996ebd280fe69bb4cf09
    disposition: "Corrected review findings and admitted only a bounded re-enable candidate; no workflow mutation in this boundary."
    problem_ref: docs/problems/ci-sbom-nested-nix-sandbox-2026-07-09.md
    authorization_ref: owner-delegation-019f6933-2026-07-16
    candidate_or_evidence_refs: [run-29295978398, run-29468123745, commit-c96c7b49, pre-pause-1b28520-parent]
    landing:
      source_commit: 27f03875702f503d8ef551035733eb5f40e27a1c
      ci_ref: not_applicable_until_push
      deploy_ref: not_applicable_ci_only
      environment_identity: github_actions
      deployed_acceptance: not_applicable_until_candidate
    registry_conformance_ref: "27f03875: YAML/frontmatter parse, doccheck, dangling-reference check, and stale draft-path search passed"
  - id: ci-reenable-candidate-implement
    boundary: implement
    commit_or_artifact: 8e4aa074f970b69ce59cffa07b280f164ca1c161
    proof_refs: [docs/evidence/ci-reenable-candidate-review-2026-07-16.md, local-ci-contract-suite, pre-pause-1b28520-parent]
    rollback_ref: 27f03875702f503d8ef551035733eb5f40e27a1c
    disposition: "Workflow patch remains accepted, but P1/P2 review repaired its conflated acceptance and concurrency contracts; hosted parsing and two distinct canonical proof stimuli remain."
    problem_ref: docs/problems/ci-sbom-nested-nix-sandbox-2026-07-09.md
    authorization_ref: owner-delegation-019f6933-2026-07-16
    candidate_or_evidence_refs: [candidate-8e4aa074, pre-candidate-ci-yml-sha256-69b5de792e84446e, frozen-candidate-ci-yml-sha256-f60b63fe5e1613a9, classifier-go-false-high-risk-false-sampled-false-bucket-12, docs/evidence/ci-reenable-candidate-review-2026-07-16.md]
    landing:
      source_commit: 8e4aa074f970b69ce59cffa07b280f164ca1c161
      ci_ref: pending_current_main_registry_reconciliation_and_push_authority
      deploy_ref: not_applicable_ci_only_expected_skip
      environment_identity: github_actions_pending
      deployed_acceptance: pending_hosted_parse_plus_separate_sbom_and_race_receipts
    registry_conformance_ref: not_applicable
  - id: owner-main-landing-authority
    boundary: define
    commit_or_artifact: owner-message-019f6933-2026-07-16-main-merge-node-b-authority
    proof_refs: [owner-message-019f6933-2026-07-16-main-merge-node-b-authority]
    rollback_ref: "Revert the CI candidate to 27f03875702f503d8ef551035733eb5f40e27a1c if hosted evidence fails."
    disposition: "Owner expanded authority from child-branch parsing to PR creation and PR-mediated main merge. Node B deployment is authorized only when a later recorded CI deploy slice names the exact accepted path; app/platform source and direct node mutation remain excluded."
    problem_ref: docs/evidence/ci-reenable-candidate-review-2026-07-16.md
    authorization_ref: owner-message-019f6933-2026-07-16-main-merge-node-b-authority
    candidate_or_evidence_refs: [candidate-8e4aa074, origin-main-a1d2f88c]
    landing:
      source_commit: pending_rebase
      ci_ref: pending_pr
      deploy_ref: normal_ci_only_deploy_skip_expected; later_concrete_deploy_slice_required_if_selected
      environment_identity: github_actions; Node_B_only_if_later_slice_admitted
      deployed_acceptance: pending
    registry_conformance_ref: pending_same_boundary_registry_validation

view:
  path: none
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-ci-optimization-2026-07-16.md --serve 127.0.0.1:8789 --watch"
---

# CI Re-Enablement and Evidence-Bounded Optimization

This is an owner-authorized CI-maintenance `/goal`, concurrent only because its
source and effect scope is disjoint from the Autoputer product mission. It does
not compete for product authority. Main-branch CI remains a shared resource and
must be serialized before any owner-authorized landing.
