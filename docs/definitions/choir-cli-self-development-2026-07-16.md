---
title: "SUPERSEDED INCOMPLETE — Make Choir Self-Developing"
definition_version: 2

start:
  captured_at: 2026-07-18T06:04:32Z
  source:
    canonical_ref: refs/heads/main@a36ebb08b024d74c06c3124c49c46e5acc4d2b63
    deploy_identity: "choir.news reported 9d9945e65f5b54069e1a86a530cb0960d96b3474 at original authoring"
  worktree_inventory:
    status: reconciled
    evidence_ref: "Full pre-supersession Definition retained at refs/heads/selfdev/architecture-recovery@2526f108a36c498f2f90ac89fcc6e4140685d9d9"
    preservation_rule: "Retain all construction, review, failure, security, architecture, candidate, and rollback evidence. Never execute historical next actions."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-current-session
      touch: read_only
      paths_or_digest: refs/heads/selfdev/g1-round72-signed-activation@5517c2eb5c94678eb4ec323fef2cec34b96f7c6a
      recovery: "Preserve as rejected evidence; never merge or deploy."
  candidates:
    - id: round72-signed-activation
      ref: refs/heads/selfdev/g1-round72-signed-activation@5517c2eb5c94678eb4ec323fef2cec34b96f7c6a
      base: refs/remotes/origin/main@7913a3da0343ee03cf32b7622aaf9f2de35ee887
      scope: [historical_self_development_runtime_candidate]
      disposition: discarded
      evidence_ref: "Frozen G1 rejection: mutable-root/updater authorization, exact inventory, and capsule freeze-ingress blockers"
  observed_artifact:
    - claim: "The mission produced canonical event, capsule, updater, checkpoint, route, public API/CLI, mode, kernel-capability, and rollback source/evidence across many reviewed slices."
      evidence_ref: refs/heads/selfdev/architecture-recovery@2526f108a36c498f2f90ac89fcc6e4140685d9d9
    - claim: "No complete deployed self-development loop, Genesis import, accepted apply, restart/reconstruction, rejection, event-derived rollback, or G3 closure occurred. Effects remained OFF."
      evidence_ref: "Supersession reconciliation 2026-07-21"
    - claim: "Round 72 passed focused source and immutable-guest checks but G1 rejected it because mutable agentcore could ask the updater to authorize caller-computed input without independent canonical accepted-event authority."
      evidence_ref: refs/heads/selfdev/g1-round72-signed-activation@5517c2eb5c94678eb4ec323fef2cec34b96f7c6a
    - claim: "A staged Texture delayed-research run independently exposed the same conflation of artifact revision, provider-run completion, durable actor wake, work settlement, and effect authority."
      evidence_ref: refs/heads/selfdev/architecture-recovery@2526f108a36c498f2f90ac89fcc6e4140685d9d9
  unknowns: []

finish:
  deliver: "Historical mission tombstone preserving exact evidence and refusing further execution."
  artifact: "A superseded non-executable Definition pointing to the completed historical durable-computer convergence Definition."
  acceptance:
    - action: "Confirm all three registries mark this mission superseded/non-entrypoint, mark convergence completed/non-entrypoint, and expose no executable `/goal`."
      proves: "This file and its completed successor cannot compete with current mission authority."
      evidence_class: documentation_registry_conformance
    - action: "Resolve the rejected candidate and full historical ledger by exact immutable Git refs."
      proves: "Supersession did not erase or launder incomplete work."
      evidence_class: git_object_identity
  rollback: "Restore the pre-supersession Definition from 2526f108 only as historical evidence; never restore its executable status without a new owner decision and coherent registry cutover."
  landing:
    required: false
    environment: not_applicable
    required_receipts: [docs_truth_check, registry_conformance]
  not_done_when:
    - "Any registry names this file as active or executable."
    - "Any current card or instruction authorizes repairing, merging, or deploying Round 72."
    - "Intermediate accepted gates are represented as mission completion."

boundaries:
  mutation_class: red
  authority_sources: [owner_supersession_2026-07-21, docs/definitions/choir-coherent-computer-convergence-2026-07-21.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
  must_preserve: [effects_OFF, rejected_candidate_identity, historical_evidence_refs, rollback_refs, no_deployed_completion_claim]
  excluded: [runtime_mutation, candidate_repair, merge, deploy, Genesis_import, updater_apply, checkpoint_or_route_effect]
  protected_surfaces: [active_Definition, mission_registries, rejected_candidate_disposition, self_development_effect_authority]
  completion_evidence_floor: [documentation_registry_conformance, git_object_identity]

measures:
  - name: competing_entrypoint_count
    kind: gate
    baseline: one
    desired: zero
    decision_use: "Reject supersession until this mission is non-executable everywhere."
    cannot_prove: "Successor product correctness."

now:
  status: superseded
  slice: none
  question: none
  reconciliation:
    observed_at: 2026-07-21T19:41:58Z
    source_ref: refs/heads/selfdev/g1-round72-signed-activation@5517c2eb5c94678eb4ec323fef2cec34b96f7c6a
    deploy_identity: "Node B staging host and active guests reported 832ae951e84400a54bd7f8ef52a312e872b5c3ef"
    authority_identities: [docs/definitions/choir-coherent-computer-convergence-2026-07-21.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: start.worktree_inventory
    status: reconciled
  candidate:
    id: round72-signed-activation
    state: discarded
    ref: refs/heads/selfdev/g1-round72-signed-activation@5517c2eb5c94678eb4ec323fef2cec34b96f7c6a
    owner: owner-and-current-session
    base: refs/remotes/origin/main@7913a3da0343ee03cf32b7622aaf9f2de35ee887
    digest: 5517c2eb5c94678eb4ec323fef2cec34b96f7c6a
    scope: [historical_self_development_candidate]
  decision:
    selected: "Supersede this mission incomplete; preserve evidence; do not repair, merge, or deploy Round 72; activate generic durable-computer convergence."
    kind: purpose
    status: settled
    source: owner
    evidence_ref: "Owner direction recorded 2026-07-21"
    owner_ratification_ref: "Owner statement: step back and supersede the current defined mission with a new one"
    recorded_at: 2026-07-21T19:41:58Z
    consequence: "Historical evidence only. Self-development effects remain OFF."
  evidence_refs: [refs/heads/selfdev/architecture-recovery@2526f108a36c498f2f90ac89fcc6e4140685d9d9, refs/heads/selfdev/g1-round72-signed-activation@5517c2eb5c94678eb4ec323fef2cec34b96f7c6a, docs/definitions/choir-coherent-computer-convergence-2026-07-21.md]
  blocker_or_risk: "This mission never reached deployed product acceptance; intermediate receipts cannot be promoted into completion."
  next_action: none

receipts:
  - id: superseded-incomplete-2026-07-21
    boundary: terminal
    commit_or_artifact: refs/heads/selfdev/architecture-recovery@2526f108a36c498f2f90ac89fcc6e4140685d9d9
    proof_refs: ["full historical Definition at 2526f108", "rejected runtime candidate at 5517c2eb", "effects OFF; no deployed G3"]
    rollback_ref: refs/remotes/origin/main@7913a3da0343ee03cf32b7622aaf9f2de35ee887
    disposition: superseded_incomplete
    problem_ref: "Common artifact/actor/work/effect authority conflation documented before successor"
    authorization_ref: owner_supersession_2026-07-21
    candidate_or_evidence_refs: [refs/heads/selfdev/g1-round72-signed-activation@5517c2eb5c94678eb4ec323fef2cec34b96f7c6a]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "ACTIVE.md, mission-graph.yaml, doc-authority-manifest.yaml candidate cutover"

successor:
  status: completed_non_executable
  historical_goal: docs/definitions/choir-coherent-computer-convergence-2026-07-21.md

view:
  path: none
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-cli-self-development-2026-07-16.md --output /tmp/choir-superseded-selfdev.html"
---

# SUPERSEDED INCOMPLETE — Make Choir Self-Developing

Owner direction on 2026-07-21 ended this mission before deployed product acceptance. Its complete pre-supersession ledger remains addressable at `2526f108a36c498f2f90ac89fcc6e4140685d9d9`; the rejected Round 72 code remains at `5517c2eb5c94678eb4ec323fef2cec34b96f7c6a`. Neither is executable authority. Effects remained OFF. The convergence Definition completed on 2026-07-24 and is now historical evidence; any future mission requires separate owner ratification and registry promotion.
