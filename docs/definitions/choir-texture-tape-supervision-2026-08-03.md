---
definition_version: 2
definition_id: choir-texture-tape-supervision-2026-08-03
execution_mode: mission_orchestrator

start:
  captured_at: 2026-08-04T02:29:51Z
  source:
    canonical_ref: main@794b99c9bf1526ee74a72fec8ba31e0c21df6d16
    deploy_identity: unknown; the last accepted durable-kernel receipt is 4ffcae3ab24fba8bc24ce1767e4e638667a50367, not a current staging observation
  worktree_inventory:
    status: reconciled
    evidence_ref: "2026-08-03 read-only `git status --short --branch` and `git worktree list --porcelain` receipt"
    preservation_rule: "Preserve every unrelated worktree and historical candidate in place. This draft owns only its documentation files and registry entries."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-current-session
      touch: goal_owned
      paths_or_digest: [docs/definitions/choir-texture-tape-supervision-2026-08-03.md, docs/problems/texture-lifecycle-dual-tape-authority-2026-08-03.md, docs/choir-doctrine.md, docs/heresy-detectors.md, docs/current-architecture.md, docs/agent-product-doctrine.md, docs/runtime-invariants.md, docs/texture-agentic-invariants-2026-06-13.md, docs/supervision-protocol.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
      recovery: "Revert only this draft's documentation changes before any implementation exists."
    - path: /Users/wiz/.codex/worktrees/eb3c6a2a-cb9f-4067-8cd8-e8ec6224cb6f/go-choir
      status: dirty
      class: other_agent_wip
      owner: unknown
      touch: forbidden
      paths_or_digest: [untracked .context/]
      recovery: "Leave in place on codex/definition-v1-1; do not inspect or incorporate."
    - path: /Users/wiz/go-choir-terminal-outcome-closure
      status: dirty
      class: user_wip
      owner: owner-or-prior-mission
      touch: forbidden
      paths_or_digest: [internal/objectgraph/dolt_store.go, internal/objectgraph/registry.go, internal/objectgraph/store.go, internal/store/graph_store.go, internal/store/store.go]
      recovery: "Leave in place on autoputer-terminal-outcome-closure; do not inspect or incorporate."
  candidates:
    - id: rejected-selfdev-round-72
      ref: 5517c2eb5c94678eb4ec323fef2cec34b96f7c6a
      base: historical rejected branch evidence
      scope: [self-development effects, capsule, updater, public self-development API]
      disposition: discarded
      evidence_ref: docs/definitions/choir-cli-self-development-2026-07-16.md
  observed_artifact:
    - claim: "The completed convergence mission deployed a restart-safe generic artifact/trajectory/work/update kernel with reducer-owned settlement and effects OFF."
      evidence_ref: docs/definitions/choir-coherent-computer-convergence-2026-07-21.md
    - claim: "ComputerEventAppender already provides the sole canonical per-computer sequence, immutable event pinning, corpusd head CAS, crash recovery, and reconstruction."
      evidence_ref: internal/computerevent/appender.go
    - claim: "Texture/lifecycle commands currently commit embedded-Dolt state and a separate LifecycleEvent stream without going through ComputerEventAppender."
      evidence_ref: docs/problems/texture-lifecycle-dual-tape-authority-2026-08-03.md
    - claim: "The Choir CLI can read lifecycle snapshots and durable cursors, but it does not run /goal and self-development effects remain unauthorized."
      evidence_ref: docs/current-architecture.md
    - claim: "The public staging health path currently reports deployed commit 794b99c9bf1526ee74a72fec8ba31e0c21df6d16, matching this Definition's canonical source base."
      evidence_ref: "GET https://choir.news/health?texture-tape-definition=20260803 at 2026-08-04T03:03Z"
  unknowns:
    - "The exact current choir.news host/guest/deployment identity must be fetched through the public no-SSH product path before activation."
    - "The smallest closed supervision-transaction schema and migration import that preserve existing Texture/source semantics must be frozen from a caller inventory."
    - "The compatibility-floor release and reducer-version rule that make rollback forward-readable must be rehearsed before the first event-schema cutover."

finish:
  deliver: "An owner supervises one long-running Choir trajectory from a human-bandwidth Texture appagent while Super operationally supervises concurrent scoped CoSuper workers. Texture owns fulfillment of the user's document request: it writes, messages Researcher and Super when grounded evidence or execution is needed, integrates their outputs, and keeps canonical document state legible and operationally honest. A material mid-run instruction becomes a new intent revision that Super semantically rebases across every affected branch; every displayed material fact and authored summary is event-derived and survives restart/reconstruction without rerunning a model."
  artifact: "A deployed layered-supervision contract on one canonical tape: Texture is the owner-facing appagent responsible for request-fulfilling, operationally grounded document state and agentic messaging to Researcher/Super; Researchers emit sourced evidence; Super owns operational decomposition, N-way assignment, reconciliation, findings, work obligations, and settlement proposals; CoSupers emit dense scoped non-effect results; Texture represents the owner's point of view as a versioned human-scale narrative plus mandatory deterministic control blocks; ComputerEventAppender owns serial causal acknowledgement while worker execution may proceed concurrently; and embedded Texture/lifecycle/CLI views are rebuildable projections. The competing independently authored LifecycleEvent tape is deleted or mechanically derived, no production Texture mutation bypass remains, and the future capsule-promotion seam has one exact composed candidate identity without activating effects."
  acceptance:
    - action: "Through the Texture/prompt-bar product path on one explicitly disposable staging computer, submit a request that genuinely requires sourced evidence and three independent non-effect operational investigations. Observe Texture create honest V0/V1 document state, choose typed messages to Researcher and Super, integrate a sourced Researcher packet, observe Super atomically open three scoped CoSuper assignments from one granted intent/base, witness at least one pair of attempt execution windows overlap, append at least one result out of assignment order, reconcile every obligation, incorporate the grounded result, and fetch the resulting Texture/lifecycle snapshot from desktop and CLI."
      proves: "Texture remains an agent responsible for user-request fulfillment and operationally grounded document state; Researcher evidence, Super supervision, concurrent CoSuper work, artifact state, obligations, and trajectory form one serially acknowledged canonical event-derived system without making this role sequence mandatory for unrelated requests or activating effects."
      evidence_class: deployed_product_path_projection
    - action: "Within the same deployed fan-out trajectory, retry one failed CoSuper attempt, cancel one still-open assignment, deliver one result after cancellation, and create material disagreement between two results. Require typed attempt lineage and idempotency, retain the late result as evidence, make Super disposition every attempt/result and preserved dissent, and refuse settlement until the assignment set is closed."
      proves: "Concurrent execution does not create parallel authority: retry, cancellation, late delivery, dissent, and reconciliation are explicit events on one tape, and tool success or branch completion cannot settle the trajectory."
      evidence_class: deployed_fanout_causality_and_disposition
    - action: "While the trajectory is live, submit a materially changed owner instruction through Texture as a new intent revision. Require Super to create a durable rebase obligation, reconcile every affected CoSuper assignment and artifact premise as preserved, invalidated, superseded, or compensation-required, and project the human-readable delta back into Texture."
      proves: "Interruptibility is semantic rather than binary: Super propagates changed intent, compatible work survives, invalid assumptions remain visible to the owner, and settlement cannot skip reconciliation."
      evidence_class: deployed_semantic_rebase
    - action: "Record independent-model disagreement or verifier dissent during the trajectory and observe both the selected decision and retained minority evidence in the Texture projection."
      proves: "The supervision surface preserves disagreement instead of collapsing it to an untraceable verdict."
      evidence_class: deployed_dissent_and_decision_projection
    - action: "Ask the owner to inspect the current Texture without reading raw Trace, transcripts, or CoSuper output. Require it to show the current intent and latest delta, what Super now believes, material open commitments/blockers, preserved dissent, irreversible or owner-only decisions, and exact drill-down provenance; require an honest overflow/attention count rather than a false clear state when the bounded view cannot show everything."
      proves: "Texture is the owner-perspective supervisor: it compresses N worker actions through Super's M technical claims into K human decisions without hiding material state or exceeding the main surface's human-bandwidth contract."
      evidence_class: owner_human_bandwidth_legibility
    - action: "Stop and restart the runtime, discard/reconstruct the embedded projection from externally pinned events, and then use the existing public no-SSH construction/lifecycle path to replace the disposable computer realization at the unchanged accepted ComputerVersion, desired/effective heads, and stable ComputerID. Recompute from sequence zero, compare canonical digest, artifact head, intent lineage, obligations, dispositions, evidence refs, decisions, and settlement state with the pre-restart snapshot, and obtain a signed identity-preserving realization/route-reattachment receipt. Do not invoke self-development promotion, emit checkpoint/route-projection events, or change desired state."
      proves: "Texture and lifecycle state are deterministic projections of the tape across both projection rebuild and replaceable realization, while ordinary identity-preserving lifecycle reattachment remains distinct from promotion."
      evidence_class: deployed_restart_reconstruction_equivalence
    - action: "Retry identical commands, submit same-key changed payloads, race stale-head edits, crash after pin/prepare/CAS/finalize boundaries, and attempt direct legacy Texture/lifecycle writes."
      proves: "Idempotency, stale-head refusal, crash recovery, and single authority hold without dual writes, lost acknowledgements, or projection-only mutation."
      evidence_class: deployed_authority_and_failure_atomicity
    - action: "Inventory every non-test caller of Texture/lifecycle mutation before cutover; after cutover prove each calls the canonical transaction service or deterministically refuses, and prove the former LifecycleEvent stream is derived or has no production writer."
      proves: "The replacement is a deletion/cutover, not an additional path beside the old authority."
      evidence_class: code_level_caller_deletion_proof
    - action: "Model-check or property-test the future capsule promotion seam without invoking capsule freeze or an actuator: N branch results must converge through an explicit integration/rebase step into one content-addressed candidate at the then-current base head; only that candidate may bind effect proposal, independent verification, owner decision, desired-state acceptance, materialization receipts, checkpoint, route generation, and rollback. Exercise overlapping branches, composition failure, stale base, duplicate acceptance, a second pending transition, materialization failure, checkpoint failure, route CAS loss, and rollback."
      proves: "Fan-out and promotion share one causal model before effects activate: branches are evidence producers, promotion is one serialized and fully joined candidate transition, and failure adds compensating history rather than rewriting the tape."
      evidence_class: formal_and_property_checked_capsule_promotion_contract
    - action: "For the landed source identity, complete push, CI, staging deployment, exact signed host/guest/computer identity, desktop/CLI acceptance, restart/reconstruction, compatibility-floor release rollback rehearsal, and three-registry terminal closure. The rollback rehearsal disables new supervision writes and proves forward-readable projection rebuild on the prior compatible release; it must not emit rollback_requested or rollback_applied computer events."
      proves: "The one-tape supervision loop is deployed and operable, not a local reducer demonstration."
      evidence_class: complete_deployed_supervision_projection
  rollback: "Before emitting the first supervision event, freeze a compatibility-floor release that understands every new event and reducer version, can reconstruct the projection, and can disable new writes. This mission's rollback rehearsal is release-level only: it disables supervision writes, deploys that floor, and rebuilds projections; rollback_requested and rollback_applied computer events are inadmissible. On failure, disable writes, retain all events, deploy only that floor or a forward-compatible repair, rebuild projections nondestructively, and keep the affected computer stopped if equality cannot be proved. Never delete events, reset Dolt, restore opaque data.img, re-enable independent lifecycle writes, or route around the canonical appender."
  landing:
    required: true
    environment: staging at choir.news on one explicitly disposable ComputerID, followed by the Definition's declared cutover scope
    required_receipts: [pushed_commit, ci, deploy, signed_host_guest_computer_identity, migration_import, canonical_event_chain, concurrent_super_cosuper_trajectory, witnessed_attempt_overlap, out_of_assignment_order_result, retry_cancel_late_dispositions, desktop_cli_projection, semantic_rebase, dissent_retention, owner_human_bandwidth_legibility, restart_reconstruction_equivalence, identity_preserving_realization_replacement, legacy_writer_deletion, capsule_promotion_contract_check, compatibility_floor, compatibility_floor_release_rollback_rehearsal, registry_conformance]
  not_done_when:
    - "Texture merely displays event links while its canonical revisions can still be written outside the tape."
    - "Texture is a raw action/event dump, or Super/CoSuper technical prose becomes the owner's required reading surface."
    - "Texture is reduced to a passive renderer that cannot agentically write, message Researcher or Super, wait, integrate grounded results, or report an honest blocker."
    - "Texture claims that work happened, evidence exists, or a request is fulfilled without exact event-derived operational/evidence refs."
    - "A material blocker, disagreement, commitment, belief change, or owner-only decision can be omitted without an honest attention/overflow indicator and drill-down ref."
    - "Super is treated as an effect worker, CoSupers are allowed to supervise or settle themselves, or a new trajectory-supervisor role duplicates Super."
    - "Fan-out creates one tape or desired-state head per branch, requires worker completion order to match assignment order, loses retry/cancel/late-result lineage, or permits settlement before every branch and attempt is dispositioned."
    - "A CoSuper result, tool success, individually verified branch artifact, or individually frozen branch bundle is treated as a promotable composed candidate."
    - "The lifecycle event stream and computer event chain are dual-written or reconciled by best effort."
    - "Only new computers work while existing in-scope state has no explicit import/refusal disposition."
    - "A new instruction is appended as transient message text without versioned intent, affected-state dispositions, and a settlement-blocking rebase obligation."
    - "Reconstruction depends on the existing embedded projection, a model/tool rerun, process memory, an opaque VM image, SSH, or raw vmctl."
    - "Only local tests, event counts, projection digest without semantic field comparison, review agreement, CI, or deploy identity is green."
    - "Any self-development operation is advanced, any release is staged beneath updater/incoming, or any effect-proposed, effect-accepted/rejected, materialization, checkpoint-publication, route-projection, or rollback event is emitted by this mission's acceptance path."

boundaries:
  mutation_class: red
  authority_sources: [owner_direction_2026-08-03_texture_is_audit_projection, docs/choir-doctrine.md, docs/choir-vision.md, AGENTS.md, docs/standing-questions.md, docs/computer-ontology.md, docs/agent-product-doctrine.md, docs/definitions/choir-coherent-computer-convergence-2026-07-21.md]
  must_preserve:
    - "One stable ComputerID and one canonical event chain; no third semantic store."
    - "The owner is root authority. Texture is the owner-facing appagent: it represents the owner's point of view, owns request-fulfilling document semantics, and agentically messages Researcher/Super. Super is the operational supervisor; Researchers ground claims; CoSupers are scoped workers; the event appender sequences and reducers materialize."
    - "Texture document claims about work, evidence, decisions, or completion carry exact event-derived refs or remain visibly pending/conjectural."
    - "Texture may write, ask Researcher, ask Super, ask both, ask neither, wait, or report a blocker; the mission must not encode one mandatory role sequence."
    - "Private Texture content is encrypted/pinned under the existing privacy protocol; corpusd sees commitments and mechanical CAS, not plaintext meaning."
    - "Artifact, intent, trajectory, work, update, finding, decision, and settlement identities are owner/computer scoped and stale-head checked."
    - "Appender sequencing head and worker observed base are distinct. Attempt, result, cancellation, and late-delivery events append against the current canonical sequencing head while retaining their original observed working base and staleness disposition; intent revision, disposition closure, settlement, and desired-state transitions refuse stale semantic heads."
    - "A supervision transaction is atomic: its typed mutations all reduce or none become visible."
    - "The closed supervision vocabulary is frozen before implementation. Settlement-critical meaning cannot live only in generic message/tool prose. At minimum it distinguishes Texture-authored revision/intent, actor message, Researcher packet, Super assignment/belief/finding/reconciliation/decision/settlement, CoSuper attempt/result, rebase/disposition, cancellation, and archive semantics."
    - "Fan-out is concurrent execution with serial canonical acknowledgement. Every CoSuper assignment binds a stable assignment and attempt identity, parent Super decision, intent revision, observed base head, scope digest, capability/policy digest, obligations, and idempotency commitment; later tape advancement does not erase that working base."
    - "Cancellation closes authority but not history. A late result remains retained evidence and cannot support settlement until Super explicitly dispositions it."
    - "Fan-out branches are not promotion units. A future effects-on successor must integrate selected results into one new immutable CapsuleEffectBundle at the then-current canonical base, verify its composed behavior independently, and accept at most that exact digest as the single pending desired-state transition."
    - "Promotion authority is a complete join: exact proposed bundle and base, independent verifier evidence, owner decision, accepted desired-state event, verified guest materialization receipt, checkpoint identity, expected route generation/CAS receipt, and rollback target. Checkpoint and route systems are actuators/projections, never semantic authors."
    - "Effects remain OFF; verifier, checkpoint, vmctl, and route projections cannot authorize semantic state."
    - "No SSH, raw vmctl, host edit, candidate VM, worker VM, AppAdoption, mutable branch, or model rerun is part of acceptance or rollback. Existing public no-SSH lifecycle construction may replace/reattach the same ComputerID at unchanged accepted desired/effective state for reconstruction proof; its signed identity receipt is not self-development promotion authority."
  excluded:
    - "General Texture visual polish, rich media layout, mobile UI, publication, World Wire, citation economics, or inbox redesign beyond the minimum projection needed for acceptance."
    - "Self-development effect activation; actual CapsuleEffectBundle freezing or staging; self-development operation advancement; guest updater activation; checkpoint/route promotion; Choir-in-Choir grants; cross-computer execution; or fleet-wide autonomous mutation. Capsule bindings may be modeled and property-tested only."
    - "A generic event-sourcing rewrite of ledgers that are not required by the supervised trajectory artifact."
    - "A separate trajectory-supervisor or meta-supervisor actor. Super owns operational supervision; Texture owns owner-perspective compression; CoSupers own only scoped worker results."
  protected_surfaces: [ComputerEventAppender, immutable_event_payloads, corpusd_event_head_CAS, embedded_Dolt, Texture_canonical_writes, lifecycle_reducers, trajectory_obligations, fanout_assignment_and_attempt_lineage, update_dispositions, settlement, capsule_candidate_identity_contract, promotion_evidence_join, privacy_and_keys, public_CLI_API, restart_reconstruction, deployment_identity]
  completion_evidence_floor: [code_level_caller_deletion_proof, contract_and_property_tests, formal_or_model_checked_transition_invariants, formal_and_property_checked_capsule_promotion_contract, independent_frozen_candidate_review, deployed_product_path_projection, deployed_fanout_causality_and_disposition, deployed_semantic_rebase, deployed_restart_reconstruction_equivalence, signed_staging_identity, compatibility_floor_release_rollback_rehearsal]
  conjecture_delta:
    claim: "Long-horizon trajectories become supervisable when an agentic Texture fulfills the user's document request from the owner's point of view, messages Researcher/Super for grounded evidence and execution, and faithfully compresses Super-supervised CoSuper work on one canonical tape; owner instruction changes are typed revisions that force semantic reconciliation."
    test: "One deployed mid-run instruction revision is propagated by Super across three concurrently executed CoSuper assignments including retry, cancellation, late delivery, and dissent; preserves compatible results; marks invalidated/superseded/compensation-required state; blocks settlement until every affected assignment and attempt is dispositioned; remains human-legible in Texture; and reconstructs identically after realization replacement."
    observer_blind_spot: "The bounded N-to-M-to-K proof does not establish the recursion limit or optimal compression policy for thousands of concurrent trajectories; honest overflow is required instead of an overclaim."
    fastest_falsifier: "Any production write bypasses the appender; identical event chains reduce differently; fan-out creates branch authority or loses attempt lineage; rebase/settlement can close with undispositioned affected state; a branch result can enter promotion without a newly composed current-base candidate; or owner/CLI/desktop views disagree at one canonical head."
  heresy_delta:
    discovered: [H032_dual_semantic_tape_for_Texture_and_lifecycle]
    introduced: []
    repaired: []

measures:
  - name: production_mutation_bypasses
    kind: gate
    baseline: "greater than zero: Texture/lifecycle production callers currently invoke embedded reducers without ComputerEventAppender"
    desired: zero after cutover, with every residual caller classified as migration-only, projection-only, or deterministic refusal
    decision_use: "Blocks cutover and completion while an alternate semantic writer remains."
    cannot_prove: "Does not prove reducer correctness, human usability, reconstruction equality, or semantic-rebase quality."
  - name: projection_reconstruction_digest
    kind: gate
    baseline: unavailable because the canonical tape does not yet reconstruct Texture/lifecycle state
    desired: exact canonical digest and field-level equality before/after clean rebuild and realization replacement
    decision_use: "Blocks completion on divergence and localizes reducer nondeterminism."
    cannot_prove: "Digest equality alone does not prove the projection is useful or that the event schema captured the right meaning."
  - name: owner_orientation_cost
    kind: gate
    baseline: unknown
    desired: "From one bounded Texture view, the owner identifies current intent and delta, Super's current belief state, material obligations/blockers, dissent, irreversible gates, and the next owner decision without reading raw trajectory transcripts; overflow is counted and linked rather than hidden."
    decision_use: "Blocks completion of the bounded supervision artifact and selects the minimum narrative/control-block projection needed for honest owner orientation."
    cannot_prove: "One bounded human inspection does not prove supervision scales to arbitrary trajectory count or duration; it proves the required compression contract for this trajectory."
  - name: fanout_disposition_completeness
    kind: gate
    baseline: unavailable because CoSuper assignment/attempt lineage is not on the canonical tape
    desired: "For the accepted three-way fan-out, every assignment and attempt has exactly one current disposition; retry, cancellation, late result, dissent, and rebase lineage remain queryable; settlement is impossible while any required disposition is absent."
    decision_use: "Blocks settlement and completion when concurrent work can escape Super reconciliation or Texture attention."
    cannot_prove: "A bounded three-way trajectory does not prove throughput or optimal scheduling at arbitrary fan-out; it proves the causal and authority contract."

now:
  status: working
  slice: "Repair the third frozen implementation review blockers before landing the one-tape supervision candidate."
  question: none
  reconciliation:
    observed_at: 2026-08-04T14:29:55Z
    source_ref: 6dd0072fb3daf85a077c97fea2114f9dcf515147
    deploy_identity: "choir.news remains pre-implementation at the previously observed deployed identity; no staging claim is made by this review checkpoint"
    authority_identities: [owner_direction_2026-08-03_texture_is_audit_projection, owner_ratification_2026-08-03_take_draft_into_defined_mission, docs/choir-doctrine.md@6dd0072f, docs/definitions/choir-texture-tape-supervision-2026-08-03.md@6dd0072f]
    policy_resolution_ref: not_applicable; this Definition changes event/projection authority but does not change persistent-agent model/tool policy
    worktree_inventory_ref: "Complete goal candidate: 110 tracked/untracked paths enumerated by /tmp/texture-tape-candidate-sha256.txt; generated TLC scratch removed; unrelated worktrees remain preserved."
    status: reconciled
  candidate:
    id: texture-tape-supervision-implementation-v3
    state: rejected
    ref: docs/problems/texture-lifecycle-dual-tape-authority-2026-08-03.md#third-frozen-implementation-review-blockers--2026-08-04
    owner: Main
    base: 6dd0072fb3daf85a077c97fea2114f9dcf515147
    digest: "sha256:289580c58dca44ef348adf1c20345d7dc9f8101e993b963365568a9d1c66ebb1"
    scope: "110 paths in /tmp/texture-tape-candidate-sha256.txt"
  decision:
    selected: "Execute the H032-first one-tape supervision mission with non-effect N-way CoSuper fan-out; model the complete capsule promotion join but keep capsule freeze, effects, materialization, checkpoint, and route activation for the successor."
    kind: architecture
    status: settled
    source: owner
    evidence_ref: docs/evidence/texture-tape-definition-consensus-2026-08-03.md
    owner_ratification_ref: "Owner instruction in the 2026-08-03 design dialogue: use agentic consensus and iterate the draft into a defined mission, with special attention to capsule fan-out and promotion."
    recorded_at: 2026-08-04T03:03:00Z
    consequence: "The mission may now execute schema/caller/migration candidate work and the H032 repair within its red ceremony. It must prove concurrent non-effect fan-out and semantic rebase, must not add a duplicate supervisor or activate effects, and must preserve the single-composed-candidate promotion seam for the successor."
  evidence_refs: [docs/evidence/texture-tape-supervision-candidate-2026-08-04.md, specs/texture_supervision.tla, specs/texture_supervision.cfg, specs/texture_supervision_witness.cfg, docs/evidence/texture-tape-definition-consensus-2026-08-03.md, docs/problems/texture-lifecycle-dual-tape-authority-2026-08-03.md, internal/computerevent/appender.go, internal/store/computer_event_rebuild.go, internal/store/supervision_projection.go, internal/agentcore/runtime.go, internal/apihandler/product_api_tool.go, internal/textureowner/supervision_api.go]
  blocker_or_risk: "Three independent focused reviewers rejected sha256:289580c58dca44ef348adf1c20345d7dc9f8101e993b963365568a9d1c66ebb1: empty-tape rebuild cannot boot; import reservation can lack a frozen plan; global tape expectation remains trajectory-local; artifact/belief rebase targets lack digest resolution; settlement evidence refs need not exist; and Super can synthesize owner authority through product_api_request. The four-model consensus runner timed out without verdicts."
  next_action: "Land this code-free problem checkpoint, repair the six source sequences with focused regressions, rerun the frozen candidate review, then proceed to landing only if accepted."

receipts:
  - id: texture-tape-definition-round-1
    boundary: define
    commit_or_artifact: "round-one candidate digest a7d7618ee0f5119c22949dc1bb8f2427d9141439e52b9d2934ec60d7babb44c4"
    proof_refs: [docs/evidence/texture-tape-definition-consensus-2026-08-03.md, "choir.news /health deployed_commit 794b99c9bf1526ee74a72fec8ba31e0c21df6d16 at 2026-08-04T03:03Z"]
    rollback_ref: "Revert only the Definition/doctrine/registry documentation candidate; no runtime mutation or external product effect occurred."
    disposition: "repair accepted; effects-OFF retained, fan-out and promotion contracts made explicit, owner ratification recorded"
    problem_ref: docs/problems/texture-lifecycle-dual-tape-authority-2026-08-03.md
    authorization_ref: owner_ratification_2026-08-03_take_draft_into_defined_mission
    candidate_or_evidence_refs: [docs/evidence/texture-tape-definition-consensus-2026-08-03.md]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: "read-only pre-mission staging identity 794b99c9bf1526ee74a72fec8ba31e0c21df6d16"
      deployed_acceptance: not_applicable
    registry_conformance_ref: "2026-08-04 same-boundary promotion: ACTIVE names the sole executable Definition; mission graph status working/entrypoint true/mission_orchestrator; authority manifest root entry working"
  - id: texture-tape-definition-final-review
    boundary: define
    commit_or_artifact: "accepted documentation candidate digest 0272c02b4511934e382062013debded67951762b09e8f76ce230896bb1256de5"
    proof_refs: [docs/evidence/texture-tape-definition-consensus-2026-08-03.md]
    rollback_ref: "Revert only the goal-owned documentation candidate; no runtime mutation or external product effect occurred."
    disposition: "accepted by focused blocker-clearance panel: Cursor, Claude, and OMP Gemini 3.6; no blocker or owner decision"
    problem_ref: docs/problems/texture-lifecycle-dual-tape-authority-2026-08-03.md
    authorization_ref: owner_ratification_2026-08-03_take_draft_into_defined_mission
    candidate_or_evidence_refs: ["digest 0272c02b4511934e382062013debded67951762b09e8f76ce230896bb1256de5", docs/evidence/texture-tape-definition-consensus-2026-08-03.md]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: "read-only pre-mission staging identity 794b99c9bf1526ee74a72fec8ba31e0c21df6d16"
      deployed_acceptance: not_applicable
    registry_conformance_ref: "2026-08-04 ACTIVE/mission-graph/doc-authority-manifest aligned on sole working mission_orchestrator entrypoint; dashboard render, doccheck tests/report, dangling-reference scan, diff check, and heresy regression gate passed"

  - id: texture-tape-implementation-review-round-3
    boundary: implement
    commit_or_artifact: "rejected candidate sha256:289580c58dca44ef348adf1c20345d7dc9f8101e993b963365568a9d1c66ebb1 at base 6dd0072fb3daf85a077c97fea2114f9dcf515147"
    proof_refs: [docs/problems/texture-lifecycle-dual-tape-authority-2026-08-03.md#third-frozen-implementation-review-blockers--2026-08-04]
    rollback_ref: "No canonical runtime commit or deployment; preserve the worktree candidate while repairing after this code-free checkpoint."
    disposition: "rejected by AppenderPrivacyReview, ReducerAuthorityReview, and CompatibilityProofReview; external four-model panel timed out without verdicts"
    problem_ref: docs/problems/texture-lifecycle-dual-tape-authority-2026-08-03.md
    authorization_ref: owner_ratification_2026-08-03_take_draft_into_defined_mission
    candidate_or_evidence_refs: ["sha256:289580c58dca44ef348adf1c20345d7dc9f8101e993b963365568a9d1c66ebb1", "/tmp/texture-tape-implementation-acceptance-review/manifest.tsv"]
    landing:
      source_commit: pending_problem_checkpoint
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: no_new_staging_observation
      deployed_acceptance: not_applicable
    registry_conformance_ref: "scripts/doccheck report-only complete before this checkpoint; terminal registry closure remains pending"

view:
  path: /tmp/choir-texture-tape-supervision.html
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-texture-tape-supervision-2026-08-03.md --serve 127.0.0.1:8787 --watch"
---

# Make Texture the Supervisory Projection of the Computer Tape

This is the sole executable product `/goal` entrypoint. The owner settled the
architectural direction—Texture is the audit-log projection on which
self-development rests—and ratified this effects-OFF finish line after an
independent frozen-candidate review. Execution starts with a closed
schema/caller/migration candidate; protected runtime mutation remains forbidden
until that candidate satisfies the Definition's first formal and review gate.

## Authority Model

```text
Researcher evidence + CoSuper worker actions (N, dense and technical)
  -> Super operational supervision state (M claims, obligations, decisions)
  -> canonical per-ComputerID tape
  -> Texture agent's grounded owner-perspective document (K decisions/deltas)
  <-> human owner judgment and revised intent
```

The owner is the root authority. **Texture is the owner-facing appagent:** it is
responsible for fulfilling the user's document request in canonical state. It
may write, message Researcher, message Super, ask both, ask neither, wait, or
report a blocker; it incorporates grounded evidence and operational results and
must not narrate work as complete when the tape says it is pending. **Super is
the operational supervisor:** it
decomposes granted intent, scopes CoSuper work, maintains beliefs and
obligations, reconciles evidence and dissent, and proposes settlement. **CoSupers
are workers:** they execute bounded assignments and return dense technical
results or non-effect artifact/evidence refs; they do not settle themselves or redefine
intent. **Researchers ground claims:** they return sourced evidence without
editing the document. Texture is also the human-side supervisor: it represents
the owner's point of view, turns dense state into a versioned human-bandwidth
narrative, and carries owner corrections back into the trajectory.

Texture is not merely a passive deterministic template. Its agent may author a
legible summary or revision; that authored result is itself an immutable typed
event with provenance, so reconstruction replays it without rerunning the model.
Mandatory control blocks—current intent/delta, material obligations and
blockers, dissent, irreversible gates, owner-attention requests, and overflow
counts—are deterministic projections and cannot be omitted by narrative choice.
Every narrative claim about evidence or operations binds exact refs or remains
visibly pending/conjectural. The full tape remains available for drill-down.
Texture does not own a second causal log.

The closed supervision transaction must support atomic mutations needed by the
existing durable-work kernel: initial intent/artifact/work creation; Super work
assignment, belief/finding, decision, and settlement proposal; CoSuper result and
evidence return; Texture intent/narrative revision; typed-update disposition;
rebase disposition; settlement; cancellation; and archive. Exact schema names
are frozen by the first caller-inventory candidate before implementation, but
the semantic set may not be weakened into generic message/tool prose or split
into independently acknowledged writes.

## Fan-Out And Promotion Contract

Fan-out is parallel work, not parallel authority. One Super transaction opens N
assignments. Each assignment and attempt binds its stable identity, parent Super
decision, current intent revision, observed working base, scoped obligations,
scope digest, capability/policy digest, and idempotency commitment. CoSupers may
execute concurrently. Their results append to the one computer tape in whatever
completion order occurs; the appender serializes acknowledgement without
pretending execution was serial.

Retry creates a new attempt on the same assignment lineage. Cancellation ends
the assignment's authority but does not erase later evidence. A result arriving
after cancellation remains on the tape with a `late` disposition and cannot
support settlement unless Super explicitly reopens or incorporates it under a
new authorized obligation. Super must reconcile every assignment and attempt,
including failure and dissent, before proposing settlement. Texture projects
the few material conclusions and owner decisions while preserving exact
drill-down refs.

This mission uses non-effect CoSuper results. It does not freeze
`CapsuleEffectBundle` objects or stage releases. The future effects-on successor
inherits this promotion law:

```text
N branch results/evidence
  -> Super-authored integration plan
  -> one bounded integrator CoSuper
  -> one new composed CapsuleEffectBundle at the then-current base head
  -> independent composition verification
  -> owner decision
  -> one effect_accepted desired-state transition
  -> guest materialization applied/failed receipt
  -> checkpoint publication receipt
  -> expected-generation route projection receipt
```

Branch outputs and branch-local bundles are never independently promotable
pieces of a hidden merge. Overlap, composition failure, or a head advance opens
an explicit rebase/recomposition obligation. Only the exact newly composed and
verified digest may be accepted, and the reducer admits only one pending
desired-state transition. Checkpoint and route systems actuate/project accepted
state; they do not authorize it. Every failure or rollback adds a compensating
event and preserves the prior effective state and evidence.

## Semantic Rebase Contract

A material owner instruction through Texture during live work creates a new
intent revision, not an appended transient message. It opens one durable rebase
obligation bound to the old and new intent heads. Super must reconcile every
affected CoSuper assignment, open work item, belief, or artifact premise with
one explicit disposition:

- `preserved` — still valid under the new intent;
- `invalidated` — must not support later settlement;
- `superseded` — replaced by a named new item or premise; or
- `compensation_required` — an already-realized consequence needs a named
  compensating obligation.

Super proposes these semantic dispositions, using CoSuper evidence where
needed; Texture projects the owner-legible intent delta and consequences. The
reducer enforces scope, completeness, head freshness, and the rule that the
trajectory cannot settle while a material rebase obligation or affected item
is unresolved. History is never rewritten.

## Cutover And Migration

Before behavior changes, inventory every production mutation caller. Rehearse
an additive, content-addressed import transaction for pre-cutover Texture and
lifecycle state. The import names source projection digests and establishes the
first reducer head; it does not pretend that historical state was originally
event-native.

After cutover, direct embedded mutations become reducer-private or refuse. The
retired `LifecycleEvent` implementation name may survive only as a mechanically derived compatibility view
whose identity binds the canonical computer event. Dual-write, shadow
reconciliation, and a permanent legacy-new mode switch do not satisfy the
mission. Any temporary gate must freeze old-path accretion and carry a deletion
clock inside the same Definition.

## Formalization Seam

Before implementation, project the transaction/reducer contract into the
smallest existing spec or property-test seam that can make these states
unreachable:

- a canonical event is acknowledged while its supervision transaction cannot
  be reduced or recovered;
- a Texture/lifecycle projection advances without a finalized canonical event;
- two identical event prefixes and reducer versions produce different
  projection digests or semantic fields;
- a stale intent/artifact head silently merges; or
- a trajectory settles while a material rebase obligation, assignment attempt,
  late result, or affected-state disposition remains open;
- concurrent CoSuper results create branch event authority or lose their
  assignment/base/capability lineage;
- an individually verified branch result becomes a promotion unit without a
  newly composed current-base candidate; or
- two accepted desired-state transitions are pending simultaneously.

The model/spec is not implementation proof. Completion also requires exact
event-to-code conformance tests, crash traces, and deployed reconstruction.

## Mission Path

1. Inventory all production mutation callers and freeze the closed event schema,
   reducer transaction, privacy map, import, fan-out attempt lineage,
   reconstruction law, promotion-contract model, and compatibility floor.
2. Independently challenge the frozen candidate; repair or reject it before
   protected implementation begins.
3. Implement event-first Texture/lifecycle reduction and the minimal desktop/CLI
   supervision projection; prove local contract/property/model conformance,
   including the non-effect fan-out and future single-candidate promotion law.
4. Cut over the disposable staging computer, delete/refuse alternate writers,
   and run three-way fan-out, retry, cancellation, late delivery, semantic
   rebase, dissent, crash, restart, reconstruction, and rollback acceptance.
5. Land through origin/main, CI, exact staging identity, product-path proof, and
   terminal registry closure. Self-development effects remain OFF throughout.

## Successor Boundary

Only after this mission is complete may a separately ratified successor connect
CoSuper coding work—whether driven through Claude Code, Codex, or another
external agent—to the one-composed-candidate capsule path defined above. That
successor must prove real bundle freeze, composition verification, owner
acceptance, guest materialization, checkpoint, route projection, and rollback
on the same tape. Super will supervise fan-out and integration operationally;
Texture will already be the human-bandwidth place where the owner understands
and corrects it.
