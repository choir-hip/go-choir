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
    - claim: "The workflow concurrency group ci-${github.ref} cancels an in-flight run on the same ref, so concurrent mission execution is scope-disjoint but main landing is a shared serialized resource."
      evidence_ref: ".github/workflows/ci.yml concurrency at 0e5f5c7e"
  unknowns:
    - "No authorized billing or metrics authority is available; actual billed minutes are unknown."
    - "Whether cache reuse across prior runs, serialized priming, or same-run artifact transfer is the best compile-cost candidate; they are separate experiments."
    - "Whether any optimization beyond re-enabling the already-proved lanes has matched-run critical-path benefit."

finish:
  deliver: "Restore the paused race and differential-SBOM safety signals without changing application/platform behavior or weakening CI, deploy, or publishing protection, then admit only optimizations proved to reduce GitHub Actions duration on matched runs."
  artifact: "A frozen, reviewed CI-only candidate and landed GitHub Actions workflow in which classifier selection invokes the complete reusable race workflow, the already-wired host-side SBOM job runs after successful check on selected main pushes, deploy-impact still skips CI-only changes, and every optimization has a separate evidence-backed admission receipt."
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
    - action: "Run the candidate in GitHub Actions on its PR and record all job conclusions and durations, including the classifier-selected race disposition and expected staging-deploy skip."
      proves: "The candidate parses and executes on the hosted platform without app/platform deployment."
      evidence_class: github_actions_candidate_run
    - action: "After an explicitly owner-authorized, serialized main landing, observe a selected main run where all five race jobs and the host-side SBOM job succeed; fetch the SBOM artifact manifest and verify complete checksummed output and differential counts."
      proves: "Both restored signals work on the canonical path; SBOM remains post-check non-blocking audit evidence unless separately changed under red ceremony."
      evidence_class: canonical_github_actions_acceptance
  rollback: "Revert the bounded CI commit to restore the two literal pause conditions. Never change main, dispatch a deploy, or alter gate/deploy/publish relationships without separate owner authority."
  landing:
    required: true
    environment: github_actions
    required_receipts: [pushed_candidate_commit, frozen_candidate_review, github_actions_candidate_run, owner_authorized_serialized_main_landing, canonical_github_actions_acceptance]
  not_done_when:
    - "Only local checks, a draft, or a reviewer narrative exists."
    - "A sampled condition is described as one shard even though it invokes the full reusable workflow."
    - "A duration or rounded runner-minute estimate is called actual billing."
    - "SBOM is described as gating check or deploy without a separately authorized red relationship change."
    - "A main push or workflow-dispatch deploy occurs without explicit owner authority and collision coordination."

boundaries:
  mutation_class: red
  class_rationale: "The Definition/registry boundary is green. Re-enabling race and SBOM changes protected CI assurance and supply-chain publishing conditions, so implementation uses red ceremony even though it changes no app/platform runtime."
  conjecture_delta: "The pauses are reversible conditions over already-wired, previously successful lanes; restoration should reuse that topology. Optimization candidates must be separated by mechanism and admitted only by matched critical-path evidence."
  heresy_delta:
    discovered: ["The draft falsely treated sampled_race as partial-matrix execution, described the repaired SBOM topology as unwired, conflated cache reuse with same-run compilation sharing, implied SBOM gated deployment, and used public duration as billing proof."]
    introduced: []
    repaired: ["The live Definition corrects those claims before workflow mutation and the SBOM problem record now names c96c7b49 plus its successful run."]
  authority_sources:
    - "Owner delegation in Codex task 019f6933-ea56-7403-b535-da4eafa4d1f7 on 2026-07-16: promote and run this independent CI-only mission concurrently; no app/platform changes; serialize shared main landing; no main merge or workflow-dispatch deploy without explicit authority."
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
    - "Main landing is serialized against ci-${github.ref}; no force-push, main merge, or workflow-dispatch deploy is authorized."
  excluded: [app_source, platform_source, Node_B_mutation, main_merge, workflow_dispatch_deploy, force_push, new_runner_provider, billing_claim_without_authority]
  protected_surfaces: [ci_check_gate, race_lane, sbom_audit_and_publishing, deploy_impact_classifier, deploy_staging, flakehub_publish, docs_only_fast_path, main_ci_concurrency]
  completion_evidence_floor: [github_actions_timeline, deterministic_local_contracts, frozen_candidate_review, github_actions_candidate_run, canonical_github_actions_acceptance]

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
  slice: "green Define boundary and baseline reconciliation before CI repair"
  question: "What smallest re-enable candidate preserves every protected relationship and earns hosted evidence?"
  reconciliation:
    observed_at: 2026-07-16T04:43:23Z
    source_ref: 0e5f5c7eb531dd880378996ebd280fe69bb4cf09
    deploy_identity: not_applicable_ci_only
    authority_identities: [owner-delegation-019f6933-2026-07-16, choir-doctrine@0e5f5c7e, AGENTS@0e5f5c7e, definition-skill@0e5f5c7e]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "start.worktrees plus git status/worktree inventory 2026-07-16T04:43:23Z"
    status: reconciled
  candidate:
    id: ci-reenable-candidate-1
    state: rehearsing
    ref: branch:codex/ci-optimization-mission-2026-07-16
    owner: ci-mission
    base: 0e5f5c7eb531dd880378996ebd280fe69bb4cf09
    digest: none
    scope: [.github/workflows/ci.yml]
  decision:
    selected: "Run one independent CI-maintenance goal concurrently with Autoputer, but serialize any main landing; restore existing full-race and host-side-SBOM topology without changing gate/deploy relationships."
    kind: authority
    status: settled
    source: owner
    evidence_ref: owner-delegation-019f6933-2026-07-16
    owner_ratification_ref: not_applicable
    recorded_at: 2026-07-16T04:43:23Z
    consequence: "Green Definition/registry mutation and bounded CI-only candidate work are authorized; main merge and workflow-dispatch deploy are not."
  evidence_refs: [docs/evidence/ci-optimization-baseline-2026-07-16.md, run-29295978398, run-29468123745, commit-c96c7b49, pre-pause-1b28520-parent]
  blocker_or_risk: "A same-ref main push cancels an in-flight main run; canonical acceptance requires later explicit owner authority and serialization."
  next_action: "Commit this code-free green Define boundary, then prepare and locally rehearse the smallest CI-only re-enable candidate."

receipts:
  - id: corrected-problem-and-baseline-define
    boundary: define
    commit_or_artifact: pending_green_define_commit
    proof_refs: [docs/evidence/ci-optimization-baseline-2026-07-16.md, docs/problems/ci-sbom-nested-nix-sandbox-2026-07-09.md]
    rollback_ref: 0e5f5c7eb531dd880378996ebd280fe69bb4cf09
    disposition: "Corrected review findings and admitted only a bounded re-enable candidate; no workflow mutation in this boundary."
    problem_ref: docs/problems/ci-sbom-nested-nix-sandbox-2026-07-09.md
    authorization_ref: owner-delegation-019f6933-2026-07-16
    candidate_or_evidence_refs: [run-29295978398, run-29468123745, commit-c96c7b49, pre-pause-1b28520-parent]
    landing:
      source_commit: not_applicable_docs_define_boundary
      ci_ref: not_applicable_until_push
      deploy_ref: not_applicable_ci_only
      environment_identity: github_actions
      deployed_acceptance: not_applicable_until_candidate
    registry_conformance_ref: pending_same_commit_registry_checks

view:
  path: none
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-ci-optimization-2026-07-16.md --serve 127.0.0.1:8789 --watch"
---

# CI Re-Enablement and Evidence-Bounded Optimization

This is an owner-authorized CI-maintenance `/goal`, concurrent only because its
source and effect scope is disjoint from the Autoputer product mission. It does
not compete for product authority. Main-branch CI remains a shared resource and
must be serialized before any owner-authorized landing.

