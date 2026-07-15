---
title: "Choir Autoputer Completion — Reconciled v2"
definition_version: 2

start:
  captured_at: 2026-07-14T08:25:48Z
  source:
    canonical_ref: refs/heads/main@7b143240c93c65650745e73eabea66bd94ef21d6
    origin_ref: refs/remotes/origin/main@7b143240c93c65650745e73eabea66bd94ef21d6
    relation: canonical_ref_equals_origin_ref
    deploy_identity: unknown
  worktree_inventory:
    status: reconciled
    evidence_ref: sha256:7a331cd12905062861b504a41001990e46a55d762315b3942f32edf263b7bb9e
    preservation_rule: "Treat /Users/wiz/go-choir as protected user WIP: read only; do not edit, stage, stash, reset, switch, or clean it."
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty
      class: user_wip
      owner: user
      touch: read_only
      paths_or_digest: "25 unstaged tracked paths; 0 staged paths; 0 untracked paths; diff +630/-1197; sha256:7a331cd12905062861b504a41001990e46a55d762315b3942f32edf263b7bb9e"
      recovery: leave_in_place
  candidates:
    - id: R1-toolregistry-facade-extinction-07
      ref: /Users/wiz/go-choir
      base: refs/heads/main@7b143240c93c65650745e73eabea66bd94ef21d6
      scope: [protected_25_path_user_wip]
      disposition: paused
      touch: read_only
      evidence_ref: sha256:7a331cd12905062861b504a41001990e46a55d762315b3942f32edf263b7bb9e
  observed_artifact:
    - claim: "A prior inventory observation saw 24 unstaged tracked paths, diff +630/-1083, with digest sha256:152b467bd2f2f5f7b1f3d658d5d02208f9f0be828b35be34877b8264763192d6; it is historical only and does not describe the current protected worktree."
      evidence_ref: historical_observation_before_2026-07-14T08:25:48Z
    - claim: "docs/runtime-dissolution-inventory.yaml remains canonical evidence, but its canonical_parent f72a141ef0f97fbec6521831dc3f5836b9526631 is stale against the captured source."
      evidence_ref: docs/runtime-dissolution-inventory.yaml
  unknowns:
    - deployed source identity at capture time

finish:
  deliver: "An external agent can use Choir as one persistent user computer: inspect and operate it without SSH, change it through a candidate, verify and promote or roll back that candidate, and continue with the same durable computer after restart."
  artifact: "A runtime-free clean cutover supporting the persistent-computer product path, with fetched staging artifacts for scoped operation, restart durability, candidate promotion and rollback, and containment."
  acceptance:
    - action: "On staging, use the authenticated product API or Choir CLI under a scoped key to construct or select the computer, inspect it, operate it, and fetch the resulting durable artifact without SSH."
      proves: "An external agent can operate the persistent user computer through the supported product path."
      evidence_class: deployed_product_path
    - action: "Recompute the runtime ownership map, run focused owner/caller tests, and apply the scoped runtime ratchet after each clean-cutover slice."
      proves: "The tested slice moved callers to its canonical owner without a compatibility facade and stayed inside its structural boundary."
      evidence_class: source_and_focused_test_support
    - action: "Restart the accepted staging computer or its serving lifecycle, then fetch the same durable state and continue a product-path operation."
      proves: "Accepted computer state and operability survive restart."
      evidence_class: deployed_restart_proof
    - action: "Create an isolated candidate change, verify it, promote it through the product path, observe the promoted artifact, then exercise rollback or an explicit safe refusal and re-fetch the prior accepted state."
      proves: "Candidate promotion and rollback are real, scoped, and recoverable product capabilities."
      evidence_class: deployed_promotion_rollback_proof
    - action: "Attempt a co-super or external-agent operation outside its granted scope and fetch the refusal/containment receipt."
      proves: "External agency remains contained by explicit authority."
      evidence_class: deployed_containment_proof
    - action: "Close this Definition with Autopaper still unauthorized unless a separate owner-authorized successor Definition exists."
      proves: "Autoputer completion does not automatically authorize Autopaper."
      evidence_class: authority_and_registry_receipt
  rollback: "Pause or discard an unaccepted candidate; for a landed failure, restore the last accepted source and computer-state refs, redeploy, and repeat the scoped product-path proof before resuming."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - "internal/runtime, a compatibility facade, or dual ownership remains"
    - "the protected user WIP or any candidate lacks a safe disposition"
    - "only local tests, the runtime ratchet, wrapper counts, documentation rhythm, review, or deploy identity are green"
    - "restart, promotion/rollback, containment, or fetched staging product evidence is missing"
    - "Autopaper is treated as automatically authorized"

boundaries:
  mutation_class: red
  reclassification_rule: "The active model/research ownership slice is red because it moves computer-owned model policy, provider/model selection, verification calls, source/search tools, content/evidence reads, and researcher checkpoint pressure. Implementation rollback is 474b9b70214ba6c063e6e3231430ce75474e014c."
  authority_sources:
    - "owner direction recorded for this 2026-07-14 reconciliation"
    - AGENTS.md
    - docs/standing-questions.md
    - docs/choir-doctrine.md
    - docs/computer-ontology.md
    - docs/agent-product-doctrine.md
    - docs/definitions/choir-autoputer-completion-2026-07-14.md
    - docs/runtime-dissolution-inventory.yaml
  must_preserve:
    - "Exactly one active goal: /goal docs/definitions/choir-autoputer-completion-2026-07-14.md."
    - "Exactly two non-conflated Dolt stores: the corpusd world-wire ObjectGraphStore in sql-server mode, and one VM-local embedded Dolt workspace per user computer for app state and branch-based promotion/rollback."
    - "Machine-world state remains in the computer filesystem and VM snapshot; route-slot control rows are not a third product-state store, and vmctl remains their sole CAS writer."
    - "Each runtime extraction moves one cohesive ownership boundary and every caller directly to the canonical owner, then deletes the old path with no alias, wrapper, facade, re-export, callback seam, or dual authority."
    - "The 25-path /Users/wiz/go-choir worktree and paused R1-toolregistry-facade-extinction-07 candidate remain protected and read only until an explicit adopt/rebuild/discard decision."
    - "Staging product artifacts, not local narration or structural counts, prove persistent-computer behavior."
    - "One lead integrates and verifies, parallelizes disjoint read-only mapping and independent review, serializes overlapping mutations and shared authority, and continues until complete, blocked_incomplete, or superseded."
  excluded:
    - runtime behavior changes in this yellow migration
    - automatic Autopaper authorization
    - a third Dolt store or shadow current-state projection
    - hard documentation-to-code ratios
    - commit-budget scripts or commit-count quotas
    - routine consensus or panel loops
    - process-report companion documents
    - a new HTML renderer or generated HTML view
    - a persistent-deliberation or configuration mission pivot
  protected_surfaces:
    - /Users/wiz/go-choir
    - persistent computer state and restart lifecycle
    - candidate promotion and rollback
    - external-agent and co-super authority containment
    - Texture, Trace, vmctl, auth/session, provider/gateway, run acceptance, and deployment routing when later slices touch them
  completion_evidence_floor:
    - canonical pushed source plus CI and staging deploy identity
    - authenticated scoped staging product artifacts
    - restart durability proof
    - candidate promotion and rollback proof
    - containment proof
    - independent recomputation for protected claims
    - terminal candidate and protected-WIP dispositions

measures:
  - name: docs_to_implementation_rhythm
    kind: weak_signal
    baseline: "The predecessor accumulated phase, lock, and panel transcripts around implementation slices."
    desired: "Keep one compact Define update per coherent Implement boundary when useful, without a ratio or quota."
    decision_use: "Prompt simplification when process prose grows faster than delivered and verified product behavior."
    cannot_prove: "Completion, implementation quality, behavior preservation, or product operability."
  - name: runtime_wrappers
    kind: telemetry
    baseline: "4 before rehearsal -> 4 in the paused candidate observation."
    desired: "Decrease only when a cohesive clean cutover actually deletes wrapper ownership; no target is authorized by this observation."
    decision_use: "Select wrapper debt for caller/owner inspection and detect accidental wrapper growth."
    cannot_prove: "A clean cutover, behavior preservation, runtime extinction, or product acceptance."
  - name: runtime_ratchet
    kind: gate
    baseline: "docs/runtime-dissolution-inventory.yaml is the retained baseline, with stale canonical_parent f72a141ef0f97fbec6521831dc3f5836b9526631 requiring scoped reconciliation."
    desired: "For each authorized slice, preserve unrelated categories and require only the exact, mechanically justified decreases or non-increases."
    decision_use: "Steer slice selection and gate structural regressions within the ratchet's declared source scope."
    cannot_prove: "Runtime behavior, staging operation, restart durability, promotion/rollback, containment, or the persistent-computer product outcome."

now:
  status: blocked_review
  slice: "repair final clean-cutover ratchet findings before product acceptance"
  question: "Will deleting the deprecated delegate_worker_vm alias and closing the ratchet's dynamic-tool and ownership-package import blind spots make the frozen runtime-free source mechanically complete?"
  reconciliation:
    observed_at: 2026-07-15T01:00:53Z
    source_ref: refs/remotes/origin/main@232dd36d521acfb103e8a87148f1e51b92360520
    deploy_identity: "CI 29378501285 PASS; deploy job 87237465845; activation receipt 232dd36d521acfb103e8a87148f1e51b92360520@2026-07-15T00:23:23Z."
    authority_identities:
      - "owner-autoputer-reconciliation@2026-07-14"
      - docs/computer-ontology.md
      - docs/agent-product-doctrine.md
      - docs/runtime-dissolution-inventory.yaml@canonical_parent:232dd36d521acfb103e8a87148f1e51b92360520
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: sha256:7a331cd12905062861b504a41001990e46a55d762315b3942f32edf263b7bb9e
    status: final_review_blocked
    protected_surfaces: [tool_registry, runtime_extinction_ratchet, external_agent_tool_contract, deployment_identity, candidate_promotion, persistent_computer_state]
    admissible_evidence: "Exact registered-tool inventory; no retired runtime imports in any production package; focused registry and ratchet tests; regenerated inventory; independent frozen review; canonical CI/deploy identity; deployed no-SSH, restart, promotion/rollback, and containment receipts."
    rollback_ref: 232dd36d521acfb103e8a87148f1e51b92360520
    conjecture_delta: "The runtime package and production imports are gone, but clean cutover is not yet complete: final independent review found one registered deprecated tool alias and two ratchet blind spots. Deployed acceptance has proved scoped refusal and restart recovery, while candidate verification remains in progress."
    heresy_delta:
      discovered: "Production still registers delegate_worker_vm as a deprecated alias of start_worker_delegation; the ratchet omits dynamically named registered tools and excludes owner-package imports from its zero-import claim."
      introduced: none
      repaired: none
  candidate:
    id: R1-runtime-extinction-final-repair-17
    state: review_blocked
    ref: refs/remotes/origin/main@232dd36d521acfb103e8a87148f1e51b92360520
    owner: orchestrator
    base: refs/remotes/origin/main@232dd36d521acfb103e8a87148f1e51b92360520
    digest: "Delete the deprecated tool alias, make the inventory account for or reject dynamic registered names, count retired imports in every production package, regenerate the ratchet, and freeze the minimal repair."
    scope: [vmctl_tool_registration, runtime_inventory_tool_names, retired_runtime_imports, runtime_ratchet]
  evidence_refs:
    - "final-independent-review:FinalCompletionReview:REJECT:P1 delegate_worker_vm compatibility alias"
    - "final-independent-review:FinalCompletionReview:REJECT:P1 dynamically named tools absent from inventory"
    - "final-independent-review:FinalCompletionReview:P2 ownership-package retired-import exclusion"
    - "deployed-restart: primary stopped at epoch 1882 and recovered ready at epoch 1884; Texture document 8d91f7c5-6467-4285-a357-0d8ea9fb447c retained revision 21924b19-d183-46e6-a172-617ddc636928"
    - "deployed-containment: scoped key ak_c2757ec1-a13c-4ccc-8bcd-8648497125df received HTTP-403 missing required scope write:runtime for computer stop and was revoked"
  blocker_or_risk: "P1 source blockers remain. The deployed candidate adoption autoputer-acceptance-232dd36d is still verifying; no promotion or rollback claim is admissible yet."
  next_action: "Commit this problem receipt before repair code, then delete the alias, close both ratchet blind spots, regenerate the inventory, run focused tests and the ratchet, and obtain independent exact-diff acceptance."

receipts:
  - id: predecessor-B0-authority
    boundary: define
    commit_or_artifact: 008a7b88cf200119c0f762cc51cfba6be3007445
    proof_refs: [docs/evidence/choir-autoputer-completion-suite-consensus-2026-07-11.md]
    rollback_ref: 27db14c36c482e321b56a056f6ce5e0accb338a4
    disposition: complete
  - id: predecessor-S0-ratchet
    boundary: implement
    commit_or_artifact: 2327fcef4716aef070eb4b819296f01b44267364
    proof_refs: [docs/evidence/s0-runtime-ratchet-dispatch-2026-07-11.md]
    rollback_ref: 008a7b88cf200119c0f762cc51cfba6be3007445
    disposition: complete
  - id: predecessor-S1-deploy
    boundary: implement
    commit_or_artifact: 9dff369044c2147140782958de3e91971caed6bc
    proof_refs: [docs/evidence/s1-deploy-unblock-dispatch-2026-07-12.md]
    rollback_ref: 2327fcef4716aef070eb4b819296f01b44267364
    disposition: complete
  - id: predecessor-S2-wire
    boundary: implement
    commit_or_artifact: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
    proof_refs: [docs/evidence/s2-wire-authority-cutover-dispatch-2026-07-12.md]
    rollback_ref: "9dff369044c2147140782958de3e91971caed6bc; 481fb8c89a33743021e4fa96568a0936a4f6ba45"
    disposition: complete
  - id: predecessor-S3-partial
    boundary: implement
    commit_or_artifact: docs/runtime-dissolution-inventory.yaml
    proof_refs: [docs/evidence/s3-step2-phase-gate-2026-07-13.md, docs/evidence/s3-api-route-authority-dispatch-2026-07-13.md, docs/evidence/s3-api-handler-ownership-blocker-2026-07-13.md]
    rollback_ref: b7b1262e455a779ca00c8d968ef28b3fa6af9b50
    disposition: "checkpoint_incomplete; landed extraction and ratchet reductions retained, failed whole-handler candidate excluded"
  - id: R1-promptspec-package-cutover-01
    boundary: implement
    commit_or_artifact: 642b391a1589196cccf8c35169b1d32b5e791131
    proof_refs: ["https://github.com/choir-hip/go-choir/actions/runs/29293772373", "staging-submission:c8e6d073-2382-4d01-81a8-3616bcd08de0", "texture-revision:7b837837-29b8-4a6c-a6ad-491f42a024ae"]
    rollback_ref: f4d47c1b5cd412333384de7ef516a7d723c443b3
    disposition: complete
  - id: R1-prompt-packages-cutover-02
    boundary: implement
    commit_or_artifact: 488664d98b7466f47b7639607ef318b241be44e7
    proof_refs: ["https://github.com/choir-hip/go-choir/actions/runs/29295978398", "staging-submission:abd95009-6186-40f4-8525-8532959d04fa", "texture-revision:8fb7afe3-2d2b-4286-9cea-52a4dcc34f25"]
    rollback_ref: 6627cc3294c8e950f5b7c5339b8e0bb056ace3d8
    disposition: complete
  - id: R1-prompt-store-package-cutover-03
    boundary: implement
    commit_or_artifact: fb97e4b36ec32df9b6edb6b3eaf69e812e722b4e
    proof_refs: ["https://github.com/choir-hip/go-choir/actions/runs/29298475787", "deploy-job:86977543375", "staging-submission:98ae4573-13cd-4a42-b14a-ccdecca65d9a", "texture-revision:8fec021e-4f7f-4464-ba65-6602200740aa"]
    rollback_ref: caa714e1f1070a1b12d076210588d547c0bc9315
    disposition: complete
  - id: R1-agent-profile-policy-cutover-04
    boundary: implement
    commit_or_artifact: 0490b4de1f784d5753baa215979ec7a1a076becd
    proof_refs: ["https://github.com/choir-hip/go-choir/actions/runs/29300688070", "deploy-job:86984154290", "staging-trajectory:91dd8fa2-45d0-4d6f-977d-aa9af5223373", "texture-revision:6545e764-afe1-46c1-8f57-2f72f043a991"]
    rollback_ref: fb97e4b36ec32df9b6edb6b3eaf69e812e722b4e
    disposition: complete
  - id: R1-work-item-fingerprint-owner-cutover-05
    boundary: implement
    commit_or_artifact: 4a1bbdd1a43b0d0cbda6b5ef03950aa48785a97
    proof_refs: ["https://github.com/choir-hip/go-choir/actions/runs/29303459550", "deploy-job:86992455034", "staging-trajectory:5053c721-50ad-4238-9608-7ba694f881c5", "work-item:09907fb1-715d-4b40-8c4a-57b4fddf1789", "texture-revision:d6beaad9-bdbd-4a01-8001-63858a7aaaf5"]
    rollback_ref: 0490b4de1f784d5753baa215979ec7a1a076becd
    disposition: complete
  - id: R1-search-gateway-owner-cutover-06
    boundary: implement
    commit_or_artifact: 59f514efae75bd00a07743c4944a7018d23a49d8
    proof_refs: ["https://github.com/choir-hip/go-choir/actions/runs/29306556937", "deploy-job:87001461766", "staging-trajectory:4ba004d6-ac56-4a2a-9c49-284c15376b82", "researcher-run:6eeedde6-7e44-40c0-91d5-55c7c2f491c4"]
    rollback_ref: 4a1bbdd1a43b0d0cbda6b5ef03950aa48785a97
    disposition: complete
  - id: R1-wire-publication-terminalization-09
    boundary: implement
    commit_or_artifact: 93af4b20bdd9a9d62c6d82a2b39db41480e6e685
    proof_refs: ["https://github.com/choir-hip/go-choir/actions/runs/29332283029", "deploy-job:87083822349", "vm-activation-job:57518b1d-97b1-5d6b-bb63-276202e25485", "staging-trajectory:a57593ae-3ab1-4dd6-b4d3-88f1d851ef31", "stuck-work-item:c9812e4a-79a7-462e-a04d-faba6dd77908", "authenticated-cancel:HTTP-200-idempotent", "boot-recovery-terminalized-at:2026-07-14T12:30:23.942315805Z"]
    rollback_ref: bfefa64f1f1d9df9a58a38f782e21f6a8fc5aedf
    disposition: complete
  - id: R1-api-owner-cutover-10
    boundary: implement
    commit_or_artifact: 21e150bfc2bd591fb5de356b7b2b250309a4ab43
    proof_refs: ["https://github.com/choir-hip/go-choir/actions/runs/29334142720", "deploy-job:87090051004", "activation-receipt:21e150bfc2bd591fb5de356b7b2b250309a4ab43@2026-07-14T12:59:00Z", "staging-costs-unauthenticated:HTTP-401", "staging-costs-wrong-method:HTTP-405", "staging-costs-authenticated:HTTP-200-estimate-recent-summary-known-models"]
    rollback_ref: 5fd2fd24
    disposition: complete

  - id: R1-promotion-owner-cutover-11
    boundary: implement
    commit_or_artifact: fc0e3a0f90e4db30c512c03ee7a07fdf0523b3df
    proof_refs: ["https://github.com/choir-hip/go-choir/actions/runs/29342824267", "deploy-job:87119847487", "activation-receipt:fc0e3a0f90e4db30c512c03ee7a07fdf0523b3df@2026-07-14T14:58:52Z", "staging-package:c81c9e67-157d-45b1-bff3-e1090be31849", "staging-adoption:74e7ccba-e929-445c-822b-37f33cd9e72b", "staging-trajectory:promotion-acceptance-20260714140154206", "run-acceptance:runacc-9704ea95ef27a1b1c0f4:promotion-level:accepted"]
    rollback_ref: 4f8032d52b9d3bef90b9e81d1bb832e272550b75
    disposition: complete

  - id: R1-candidate-package-owner-cutover-12
    boundary: implement
    commit_or_artifact: cb0e36ba9cb9568f838e470935a90345446e69eb
    proof_refs: ["https://github.com/choir-hip/go-choir/actions/runs/29345667840", "deploy-job:87129761444", "activation-receipt:cb0e36ba9cb9568f838e470935a90345446e69eb@2026-07-14T15:38:14Z", "focused-candidate-transitions:PASS", "runtime-shard-0/4:PASS", "runtime-ratchet:PASS", "candidate-transition-review:ACCEPT", "candidate-authority-review:ACCEPT", "staging-review-unauthenticated:HTTP-401", "staging-review-owner-scoped-missing:HTTP-404", "staging-review-write-blocked:HTTP-405", "staging-full-intake-read-absent:HTTP-404", "staging-full-intake-write-blocked:HTTP-405"]
    rollback_ref: 2f6215ab4256eb1ad4acead67a788d1825a6c0af
    disposition: complete

  - id: R1-browser-desktop-owner-cutover-13
    boundary: implement
    commit_or_artifact: a0b7c9d904be618c8ace59d0b19b6d10bb7d4226
    proof_refs: ["https://github.com/choir-hip/go-choir/actions/runs/29349986819", "deploy-job:87144519480", "activation-receipt:a0b7c9d904be618c8ace59d0b19b6d10bb7d4226@2026-07-14T16:38:25Z", "browser-owner-tests:PASS", "desktop-owner-tests:PASS", "runtime-shard-0/4:PASS", "runtime-ratchet:PASS", "browser-transition-review:ACCEPT", "desktop-authority-review:ACCEPT", "staging-browser-capabilities:HTTP-200-ready", "staging-browser-sessions:HTTP-200", "staging-browser-lifecycle:7e28e2bb-a20e-40b6-b30c-53951b0dcbb8-idle-to-closed", "staging-desktop-state:HTTP-200-owner-scoped"]
    rollback_ref: b3d8205fe3047ccbde392abbaed681a2e60dbee1
    disposition: complete

  - id: R1-content-media-owner-cutover-14
    boundary: implement
    commit_or_artifact: 5fbe4d50
    proof_refs: ["https://github.com/choir-hip/go-choir/actions/runs/29353600501", "deploy-job:87156686176", "activation-receipt:474b9b70214ba6c063e6e3231430ce75474e014c@2026-07-14T17:31:15Z", "content-owner-tests:PASS", "media-owner-tests:PASS", "runtime-shard-1/4:PASS", "runtime-ratchet:PASS", "content-media-transition-review:ACCEPT", "content-media-authority-review:ACCEPT", "staging-content-import:HTTP-201:0b807e44-8b7a-4ce7-846a-0b7c046e1e65", "staging-content-list-get:HTTP-200", "staging-media-progress-put-get:HTTP-200", "staging-media-recents-put-get:HTTP-200"]
    rollback_ref: b3d8205fe3047ccbde392abbaed681a2e60dbee1
    disposition: complete

  - id: R1-model-research-owner-cutover-15
    boundary: implement
    commit_or_artifact: 36caa912d9a2855cfa473d88e3393bc39c64c74e
    proof_refs: ["https://github.com/choir-hip/go-choir/actions/runs/29358737663", "deploy-job:87174088250", "activation-receipt:36caa912d9a2855cfa473d88e3393bc39c64c74e@2026-07-14T18:46:29Z", "modelpolicy-tests:PASS", "researchtools-tests:PASS", "runtime-shard-1/4:PASS", "runtime-shard-2/4:PASS", "runtime-shard-3/4:PASS", "runtime-shard-4/4:PASS", "runtime-ratchet:PASS", "model-research-authority-review:ACCEPT", "model-research-behavior-review:ACCEPT", "owner-contract-repair-review:ACCEPT", "final-owner-authority-review:ACCEPT", "staging-model-policy-resolve:HTTP-200:researcher=chatgpt/gpt-5.4-mini/medium", "staging-trajectory:e6283f92-7167-4426-a21a-a13d64b9b04b", "staging-researcher-run:1af42058-a64d-45b3-a7a3-863265d401a0:completed"]
    rollback_ref: 474b9b70214ba6c063e6e3231430ce75474e014c
    disposition: complete

view:
  path: none
  generator: none
---

# Choir Autoputer Completion — Reconciled v2
