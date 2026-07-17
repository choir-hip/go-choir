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
    - claim: "Successful selected-Race run 29550365185 executed the ordinary 3+4 Go matrices and then repeated the same package population through the reusable 4+1 Race workflow; the unsharded Race non-runtime job took about 12m10s."
      evidence_ref: "docs/problems/ci-duplicate-race-and-serialized-sbom-critical-path-2026-07-17.md"
    - claim: "In run 29550365185 the parent check completed about 12m28s after workflow start, then the 17m21s differential-SBOM job extended total workflow time to about 30 minutes even though deploy and FlakeHub did not depend on it."
      evidence_ref: "docs/problems/ci-duplicate-race-and-serialized-sbom-critical-path-2026-07-17.md"
    - claim: "The existing 3-way non-runtime and 4-way runtime scripts forward -race, so selected Race can substitute on the ordinary matrices instead of adding a second test horn."
      evidence_ref: "scripts/go-test-non-runtime-shards and scripts/go-test-runtime-shards at origin/main@ba1fd5a4"
  unknowns:
    - "Actual hosted duration of the consolidated Race matrices before a natural selected main run."
    - "Actual wall-clock and runner-duration reduction; all projected savings remain forecasts until matched hosted evidence."
    - "Whether SBOM candidate construction and the Go matrices contend enough on hosted infrastructure to change either lane's duration."

finish:
  deliver: "Preserve the restored Race and differential-SBOM assurances while removing the duplicate selected-Race Go-test horn, sharding standalone Race consistently, and overlapping unaccepted SBOM construction with the check path."
  artifact: "A reviewed and landed CI-only topology with one authoritative race_selected output; standard-or-Race execution on the same complete 3+4 Go matrices; one focused non-Race exception for the regression intentionally skipped under instrumentation; a complete sharded scheduled/manual race.yml; and an identity-bound SBOM candidate that only a successful post-check finalizer can promote to accepted cache and durable artifact."
  acceptance:
    - action: "Measure job timelines on successful selected-Race run 29550365185 and bind every structural claim to the current workflow and shard scripts."
      proves: "The duplicate horn and serialized SBOM critical path are observed rather than inferred; public durations remain telemetry, not billing."
      evidence_class: github_actions_timeline
    - action: "Run classifier, shard, SBOM construction/promotion, deploy-impact, workflow-contract, shell-syntax, YAML, and doc-truth contracts locally."
      proves: "One selector drives both matrices, all matrix scripts receive the intended mode, the non-Race-only regression and integration smoke remain selected, and malformed or mismatched SBOM candidates cannot be promoted."
      evidence_class: deterministic_local_contracts
    - action: "Freeze the bounded candidate by base, commit, scoped paths, and workflow digest; obtain independent review of the immutable candidate."
      proves: "Review can reject coverage loss, duplicated selector logic, unsafe SBOM publication, changed deploy dependencies, or scope expansion."
      evidence_class: frozen_candidate_review
    - action: "Run the candidate on a pull request and record hosted parsing plus every job conclusion; require the workflow/script contract suite to pass."
      proves: "The replacement topology parses and its deterministic contracts execute on GitHub-hosted runners; PR stimulus alone does not prove a main-only SBOM route or selected Race."
      evidence_class: github_actions_candidate_parse
    - action: "Immediately before merge, fresh-fetch main and confirm no active main CI or Race run can be cancelled; merge only through the pull request."
      proves: "The main landing is serialized and does not silently supersede another mission's evidence."
      evidence_class: github_actions_serialization
    - action: "On the landing or first suitable natural main run, require a successful SBOM candidate and finalizer, download the durable artifact, verify exact run/attempt/SHA identity, 12 unique package records, required-package success, all declared checksums, and differential consistency; confirm deploy_needed=false and Node B skipped for the CI-only landing."
      proves: "Parallel construction does not weaken accepted-baseline or durable-artifact integrity and the landing does not deploy."
      evidence_class: canonical_ci_only_sbom_acceptance
    - action: "Observe the first natural main run with race_selected=true; require all three non-runtime Race shards, all four runtime Race shards, the focused non-Race-only regression, integration smoke, and the parent check gate to pass, with no parent reusable-Race horn."
      proves: "Race substitutes for rather than duplicates ordinary coverage on the complete owned matrix population."
      evidence_class: post_land_race_selector_acceptance
    - action: "Compare plan-to-check, workflow completion, summed public job duration, and SBOM finalizer overhead with the observed baseline, reporting actual values without calling them billing."
      proves: "The new topology's real hosted Pareto effect is measured and can be rejected if assurance is preserved but time is not improved."
      evidence_class: matched_hosted_telemetry
  rollback: "Revert the bounded optimization commits through a pull request to restore origin/main@ba1fd5a4 behavior: complete reusable Race after ordinary matrices and differential SBOM after check. Coordinate main CI/Race cancellation groups before rollback. CI-only deploy classification must remain false, so no Node B rollback is expected."
  landing:
    required: true
    environment: github_actions
    required_receipts: [problem_documented_before_repair, deterministic_local_contracts, frozen_candidate_review, github_actions_candidate_parse, github_actions_serialization, canonical_ci_only_sbom_acceptance, post_land_race_selector_acceptance, matched_hosted_telemetry]
  not_done_when:
    - "Only workflow edits, local checks, a draft PR, or reviewer forecasts exist."
    - "Race selection is recomputed in multiple jobs or any selected package population runs both ordinary and Race modes."
    - "The regression intentionally skipped under Race or runtime-shard-1 integration smoke is lost."
    - "An SBOM candidate cache key can match an accepted-baseline restore prefix, or candidate construction can publish the durable artifact."
    - "The finalizer does not fail closed on run/attempt/SHA identity, exact package set, required-package status, declared files, checksums, or differential consistency."
    - "Deploy or FlakeHub waits on SBOM finalization, or check/deploy/publish protection is weakened."
    - "Projected latency or runner-duration savings are reported as observed before matched hosted proof."
    - "A direct main push, generic workflow dispatch, direct Race dispatch, or direct Node B mutation occurs."

boundaries:
  mutation_class: red
  class_rationale: "Race selection/check acceptance and SBOM accepted-baseline publication are protected assurance surfaces. The change alters no app/platform runtime and the problem/Definition checkpoint precedes repair code."
  conjecture_delta: "A selected Race execution on the same complete 3+4 shard population is an assurance-preserving substitute for the duplicate ordinary horn when the one known instrumentation skip is run explicitly without Race. Unaccepted SBOM construction can overlap tests because only a post-check identity/checksum finalizer creates accepted state."
  heresy_delta:
    discovered: ["The terminal restoration topology remained Pareto-suboptimal: selected runs duplicated the complete Go-test population and serialized a 17-minute SBOM job after check.", "The completed Definition's artifact froze the restoration topology and declared no next action even though its title and owner intent included evidence-bounded optimization.", "TestCancelRunTrajectoryDrainsMoreThanOneActivePage intentionally skips under Race, so naive substitution would silently narrow selected-run coverage."]
    introduced: []
    repaired: ["The prior registry/doccheck mismatch and paused Race/SBOM routes remain repaired.", "The new problem record and reopened Definition separate observed facts from forecast savings and make the non-Race-only test an explicit acceptance invariant."]
  authority_sources:
    - "Owner delegation and supervision in Codex task 019f6933-ea56-7403-b535-da4eafa4d1f7 on 2026-07-16: promote and run this independent CI-only mission concurrently; no app/platform changes; serialize shared main CI and Race observations."
    - "Owner authority in Codex task 019f6933-ea56-7403-b535-da4eafa4d1f7 on 2026-07-16: create PRs, merge this CI-only change to main, and deploy Node B when the accepted landing path requires it."
    - docs/choir-doctrine.md
    - AGENTS.md
    - docs/standing-questions.md
    - skills/definition/SKILL.md
    - "Owner request on 2026-07-16 to resume the /goal, investigate why restored CI remains about 30 minutes with an apparently duplicate Race horn, use the agentic-consensus panel, and design a Pareto-efficient workflow."
  must_preserve:
    - "Autoputer remains the sole executable product Definition; this is a scope-disjoint CI-maintenance entrypoint, not a competing product mission."
    - "The parent dirty Autoputer Definition and all app/platform source are untouched."
    - "high_risk_race and sampled_race derive one authoritative race_selected output consumed by both complete Go matrices and the parent check."
    - "Selected Race substitutes -race on all three non-runtime and all four runtime shards; it is not a reduced sample and does not retain the duplicate parent reusable-workflow horn."
    - "The complete scheduled/manual race.yml route remains available and keeps all runtime and non-runtime package coverage."
    - "Runtime shard 1 preserves integration smoke; TestCancelRunTrajectoryDrainsMoreThanOneActivePage receives one focused non-Race invocation when Race is selected."
    - "SBOM construction may overlap checks only as an unaccepted, run-bound artifact; only a successful post-check finalizer may create the accepted cache key and durable artifact."
    - "check, deploy-impact, deploy-staging, FlakeHub, docs-only routing, and supply-chain artifact integrity are not weakened."
    - "Deploy and FlakeHub remain dependent on check rather than SBOM finalization."
    - "The landing route is draft PR, hosted evidence, a recorded main concurrency window, and PR-mediated merge. Direct push to main, force-push, generic workflow dispatch, and direct Race dispatch remain excluded."
    - "Normal CI-only landing must prove deploy_needed=false and Node B skipped."
    - "Main landing and natural selected-Race observation are coordinated against ci-${github.ref} and race-${github.ref}."
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
  - name: plan-to-check latency
    kind: telemetry
    baseline: "about 12m28s on selected-Race run 29550365185"
    desired: "bounded by the slowest consolidated Race shard plus the parent gate, without a second Go-test horn"
    decision_use: "Reject consolidation if complete assurance costs more check-path time than the restored topology."
    cannot_prove: "Actual billing or causal improvement without a comparable hosted stimulus."
  - name: workflow completion latency
    kind: telemetry
    baseline: "about 30 minutes on run 29550365185 because 17m21s SBOM started after check"
    desired: "approximately max(check path, SBOM candidate path) plus a small finalizer"
    decision_use: "Verify SBOM construction actually leaves the serialized critical path."
    cannot_prove: "Correctness or actual billing."
  - name: public runner duration
    kind: telemetry
    baseline: "run 29550365185 includes seven ordinary Go jobs plus five reusable Race jobs"
    desired: "remove the seven-job duplicate horn when Race is selected, net of candidate/finalizer overhead"
    decision_use: "Quantify resource-direction improvement without presenting public elapsed time as billed minutes."
    cannot_prove: "Invoice cost."
  - name: assurance preservation
    kind: gate
    baseline: "complete ordinary and Race populations, focused non-Race behavior implicitly covered by the ordinary horn, integration smoke, post-check accepted SBOM"
    desired: "same observable contracts with one selected matrix mode and fail-closed SBOM promotion"
    decision_use: "Reject any faster candidate that narrows coverage or accepted-artifact integrity."
    cannot_prove: "Absence of all CI defects."

now:
  status: working
  slice: "Pareto CI topology repair"
  question: "Can one classifier-selected 3+4 matrix mode and fail-closed parallel SBOM construction remove duplicate work without weakening any accepted assurance?"
  reconciliation:
    observed_at: 2026-07-17T04:15:27Z
    source_ref: "origin/main ba1fd5a4973618326c8eebe9b14456941724c114; selected-Race baseline run 29550365185 at e3de55581a1cae3ecce1431f5f4440ab01f62fc8"
    deploy_identity: "not_applicable_ci_only; landing must classify deploy_needed=false and Node B must skip"
    authority_identities: [owner-request-2026-07-16-ci-pareto, choir-doctrine@ba1fd5a4, AGENTS@ba1fd5a4, definition-skill@ba1fd5a4]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "clean isolated worktree /private/tmp/go-choir-ci-integration-owner on codex/ci-pareto-topology-2026-07-17; protected parent /Users/wiz/go-choir has five Autoputer-owned modified paths and remains untouched"
    status: reviewed_candidate_ready_for_hosted_pr
  candidate:
    id: ci-pareto-candidate-1
    state: reviewed_ready
    ref: commit:a3bf59a1f938ce3ed92ce2d968c7c386db369801
    owner: ci-mission
    base: ba1fd5a4973618326c8eebe9b14456941724c114
    digest: "ci.yml 2beb6d3f970694c9cbb7851d0c1d125950ef049086c809a6379bf11a02911f7d; race.yml 957f8f3dab3fca192a78156e7ff032effb5fbcc4e7bddd98e14794612e6ffc4b; verifier 83ad36cbede20bba0d75655dad5600c88262aa1acdeb20990b6cbe8d8c148773; workflow contract 34a029dd2b110286736abe21b0552a0dcb86f01491e210d79e23b51c57f535d3"
    scope: [.github/workflows/ci.yml, .github/workflows/race.yml, .github/scripts/ci-impact-classify, .github/scripts/ci-impact-classify-test, .github/scripts/ci-workflow-contract-test, .github/scripts/verify-sbom-candidate, .github/scripts/verify-sbom-candidate-test, docs/definitions/choir-ci-optimization-2026-07-16.md, docs/problems/ci-duplicate-race-and-serialized-sbom-critical-path-2026-07-17.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
  decision:
    selected: "Derive race_selected once; substitute -race on the existing complete matrices; explicitly preserve the one non-Race-only regression and integration smoke; shard standalone Race non-runtime; split SBOM construction from fail-closed post-check promotion."
    kind: implementation_authority
    status: settled
    source: owner
    evidence_ref: "owner request 2026-07-16 plus five-agent consensus recorded in docs/problems/ci-duplicate-race-and-serialized-sbom-critical-path-2026-07-17.md"
    owner_ratification_ref: not_applicable
    recorded_at: 2026-07-17T03:45:00Z
    consequence: "The former complete restoration receipt remains historical evidence, but the Definition is reopened until the optimized topology lands and passes matched hosted acceptance."
  evidence_refs: [docs/problems/ci-duplicate-race-and-serialized-sbom-critical-path-2026-07-17.md, run-29550365185, agentic-consensus-2026-07-17, commit-01a4a053, commit-1ba8d909, commit-a3bf59a1, local-ci-contract-suite, local-focused-non-race-regression, local-race-integration-smoke, independent-review-race-resolved, independent-review-sbom-resolved]
  blocker_or_risk: "No local or frozen-review blocker remains. Review found and the candidate repaired four defects: manifest-supplied requiredness, unverified differential contents, an unexecuted standalone Race PR route, and a mismatched finalizer cache restore path. Hosted PR/main receipts and measured improvement remain unproved."
  next_action: "Commit this reviewed candidate/registry receipt, push a draft PR, and require both CI and path-filtered standalone Race hosted checks before serialized merge."

receipts:
  - id: ci-pareto-candidate-implement
    boundary: implement
    commit_or_artifact: a3bf59a1f938ce3ed92ce2d968c7c386db369801
    proof_refs: [docs/problems/ci-duplicate-race-and-serialized-sbom-critical-path-2026-07-17.md, local-ci-contract-suite, local-focused-non-race-regression, local-race-integration-smoke, independent-review-race-resolved, independent-review-sbom-resolved]
    rollback_ref: ba1fd5a4973618326c8eebe9b14456941724c114
    disposition: "Frozen candidate consolidates selected Race onto the complete 3+4 matrices, keeps the one focused non-Race exclusion and integration smoke, path-tests standalone race.yml on its own PR changes, and permits accepted SBOM publication only after exact baseline-relative verification downstream of check."
    problem_ref: docs/problems/ci-duplicate-race-and-serialized-sbom-critical-path-2026-07-17.md
    authorization_ref: owner-request-2026-07-16-ci-pareto
    candidate_or_evidence_refs: [candidate-a3bf59a1, ci-yml-sha256-2beb6d3f970694c9, race-yml-sha256-957f8f3dab3fca19, verifier-sha256-83ad36cbede20bba, workflow-contract-sha256-34a029dd2b110286]
    landing:
      source_commit: pending_pr_merge
      ci_ref: pending_hosted_pr
      deploy_ref: ci_only_deploy_skip_required
      environment_identity: github_actions
      deployed_acceptance: pending
    registry_conformance_ref: pending_receipt_commit
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
      source_commit: 71f07f7b3c1418d9e1b8e6426f8fbbd6c37d51bb
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/29530740469
      deploy_ref: deploy_needed=false; Node_B_job_87730340741_skipped
      environment_identity: github_actions
      deployed_acceptance: canonical_ci_only_sbom_accepted; post_land_race_selector_pending
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
  - id: pr1-hosted-parse
    boundary: external_evidence
    commit_or_artifact: pr-55@2467f450880ee5feb7d80acb62aec9f5e0d9004b
    proof_refs: [run-29478611966, job-87557024073]
    rollback_ref: "Close PR 55 without merge; no canonical source or protected external effect has changed."
    disposition: "Hosted CI passed. Plan CI Lanes reported go=true, sbom=true, flakehub=true, high_risk_race=false, sampled_race=false, sample_bucket=12. Race, SBOM, rolling FlakeHub, deploy-impact, and Node B skipped for the pull_request event."
    problem_ref: docs/problems/ci-maintenance-entrypoint-doccheck-cardinality-2026-07-16.md
    authorization_ref: owner-main-landing-authority
    candidate_or_evidence_refs: [pr-55, run-29478611966, job-87557024073, origin-main-a1d2f88c]
    landing:
      source_commit: 2467f450880ee5feb7d80acb62aec9f5e0d9004b
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/29478611966
      deploy_ref: pull_request_event_skipped
      environment_identity: github_actions
      deployed_acceptance: hosted_parse_accepted; canonical_main_effects_pending
    registry_conformance_ref: "Docs Truth Check passed in run 29478611966"
  - id: owner-flakehub-and-ci-merge-authority
    boundary: define
    commit_or_artifact: owner-message-2026-07-16T19:50:40Z-flakehub-and-ci-merge-authority
    proof_refs: [owner-message-2026-07-16T19:50:40Z-flakehub-and-ci-merge-authority, pr-55, run-29478958889-attempt-2]
    rollback_ref: "Close PR 55 before merge, or revert its merge commit through a pull request if canonical acceptance fails."
    disposition: "Owner explicitly authorized PR 55's rolling FlakeHub publication and merge, and authorized the remaining CI mission PR merges within the existing Definition boundaries."
    problem_ref: docs/problems/ci-maintenance-entrypoint-doccheck-cardinality-2026-07-16.md
    authorization_ref: owner-message-2026-07-16T19:50:40Z-flakehub-and-ci-merge-authority
    candidate_or_evidence_refs: [pr-55, run-29478958889-attempt-2, origin-main-a1d2f88c]
    landing:
      source_commit: e02bd0ad9e04a149466f9c3fec436395c2ec9da9
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/29530010061
      deploy_ref: deploy_needed=false; Node_B_job_87728809862_skipped
      environment_identity: github_actions
      deployed_acceptance: "PR 1 canonical acceptance passed; rolling FlakeHub job 87728809190 succeeded"
    registry_conformance_ref: "Docs Truth Check passed in run 29530010061"
  - id: pr2-frozen-identity-rebind
    boundary: define
    commit_or_artifact: 20d487470d037e9985ee4305d948b5b1f692bd42
    proof_refs: [candidate-8e4aa074, candidate-20d48747, merge-e02bd0ad, run-29530010061]
    rollback_ref: "Close PR 2 without merge, or revert only its workflow merge commit through a pull request."
    disposition: "Cherry-picked only frozen candidate 8e4aa074 onto fresh main e02bd0ad. The workflow candidate is rebound to 20d48747 with its reviewed ci.yml digest unchanged; PR 1 canonical acceptance is closed."
    problem_ref: docs/problems/ci-sbom-nested-nix-sandbox-2026-07-09.md
    authorization_ref: owner-message-2026-07-16T19:50:40Z-flakehub-and-ci-merge-authority
    candidate_or_evidence_refs: [frozen-ci-yml-sha256-f60b63fe5e1613a9f7432f1f7c79bc577a1aee52dac7195ec4de1ad39ee4ee25, candidate-8e4aa074, candidate-20d48747, origin-main-e02bd0ad]
    landing:
      source_commit: 71f07f7b3c1418d9e1b8e6426f8fbbd6c37d51bb
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/29530740469
      deploy_ref: deploy_needed=false; Node_B_job_87730340741_skipped
      environment_identity: github_actions
      deployed_acceptance: canonical_ci_only_sbom_accepted; post_land_race_selector_pending
    registry_conformance_ref: "Docs Truth Check passed in PR run 29530622886 and main run 29530740469"
  - id: pr2-canonical-sbom-acceptance
    boundary: external_evidence
    commit_or_artifact: 71f07f7b3c1418d9e1b8e6426f8fbbd6c37d51bb
    proof_refs: [pr-56, run-29530622886, run-29530740469, job-87730340035, artifact-8389010915, job-87730211197, job-87730340741]
    rollback_ref: "Revert merge 71f07f7b through a pull request to restore the two literal pause conditions."
    disposition: "PR parsing passed with go=false, high_risk_race=false, flakehub=false and protected lanes skipped. Main host-side SBOM succeeded; all 11 delivered SBOM checksums matched the 12-record manifest, differential counts were 10 built, 1 reused, 1 optional skip, 0 failed, 3 added components, and 0 removed components. deploy_needed=false and Node B skipped. Race remains correctly pending a natural selected stimulus."
    problem_ref: docs/problems/ci-sbom-nested-nix-sandbox-2026-07-09.md
    authorization_ref: owner-message-2026-07-16T19:50:40Z-flakehub-and-ci-merge-authority
    candidate_or_evidence_refs: [frozen-ci-yml-sha256-f60b63fe5e1613a9f7432f1f7c79bc577a1aee52dac7195ec4de1ad39ee4ee25, artifact-upload-sha256-ff1b123f140a2c5ca12430557680ca1e92d5f199e27518b328585acbe46887ff, artifact-id-8389010915]
    landing:
      source_commit: 71f07f7b3c1418d9e1b8e6426f8fbbd6c37d51bb
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/29530740469
      deploy_ref: deploy_needed=false; Node_B_job_87730340741_skipped
      environment_identity: github_actions
      deployed_acceptance: canonical_ci_only_sbom_accepted
    registry_conformance_ref: "Docs Truth Check passed in run 29530740469"

  - id: natural-race-terminal-attempt-1
    boundary: external_evidence
    commit_or_artifact: 5520fc0341ef1f38470860fac19d545b4a992e8e
    proof_refs: [run-29535188855, job-87744744084, job-87744744128, job-87747182458, job-87747205065]
    rollback_ref: "No CI rollback indicated: the restored selector chose all five reusable Race jobs. Product-owning repairs must land through their own Definition and pull request."
    disposition: "Natural sampled selection succeeded structurally but terminal acceptance failed. Runtime Race shards 0-2 passed; runtime shard 3 and non-runtime Race failed on product tests, so the parent gate failed. deploy_needed=false and Node B skipped."
    problem_ref: "now.blocker_or_risk; product-owned durable problem records pending"
    authorization_ref: owner-message-2026-07-16T19:50:40Z-flakehub-and-ci-merge-authority
    candidate_or_evidence_refs: [frozen-ci-yml-sha256-f60b63fe5e1613a9f7432f1f7c79bc577a1aee52dac7195ec4de1ad39ee4ee25, natural-stimulus-5520fc03]
    landing:
      source_commit: 5520fc0341ef1f38470860fac19d545b4a992e8e
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/29535188855
      deploy_ref: deploy_needed=false; Node_B_job_87747205065_skipped
      environment_identity: github_actions
      deployed_acceptance: terminal_race_failed_product_tests
    registry_conformance_ref: "Docs Truth Check passed in run 29535188855"

  - id: natural-race-terminal-acceptance
    boundary: external_evidence
    commit_or_artifact: 317c1c537afc30f2e71d0a20a62e2a0af17eb67a
    proof_refs: [run-29548529828, job-87785940855, job-87785953951, job-87785953966, job-87785953995, job-87785953998, job-87785954000, job-87787418412]
    rollback_ref: "Revert merge 71f07f7b through a pull request to restore the two literal pause conditions if the accepted Race or SBOM topology must be withdrawn."
    disposition: "Terminal acceptance passed on a natural high-risk main stimulus. Classifier outputs were go=true, high_risk_race=true, sampled_race=false. The non-runtime Race job, all four runtime Race shards, and the parent check gate succeeded."
    problem_ref: docs/problems/ci-sbom-nested-nix-sandbox-2026-07-09.md
    authorization_ref: owner-message-2026-07-16T19:50:40Z-flakehub-and-ci-merge-authority
    candidate_or_evidence_refs: [frozen-ci-yml-sha256-f60b63fe5e1613a9f7432f1f7c79bc577a1aee52dac7195ec4de1ad39ee4ee25, natural-high-risk-stimulus-317c1c53]
    landing:
      source_commit: 317c1c537afc30f2e71d0a20a62e2a0af17eb67a
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/29548529828
      deploy_ref: "not part of this terminal acceptance; product-owned deploy job 87787428646 succeeded in the same run"
      environment_identity: github_actions
      deployed_acceptance: terminal_race_accepted
    registry_conformance_ref: "Docs Truth Check job 87785953690 passed in run 29548529828"

view:
  path: none
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-ci-optimization-2026-07-16.md --serve 127.0.0.1:8789 --watch"
---

# CI Re-Enablement and Evidence-Bounded Optimization

This is an owner-authorized CI-maintenance `/goal`, concurrent only because its
source and effect scope is disjoint from the Autoputer product mission. It does
not compete for product authority. Main-branch CI remains a shared resource and
must be serialized before any owner-authorized landing.
