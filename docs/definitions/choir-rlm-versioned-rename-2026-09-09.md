---
definition_version: 2
definition_id: choir-rlm-versioned-rename-2026-09-09
execution_mode: draft_non_executable

start:
  captured_at: "2026-09-09T23:29:36Z"
  source:
    canonical_ref: "main@34629502"
    deploy_identity: "staging https://choir.news ok via proxy 0475ed84; mission-0 and mission-1 both completed with deployed proof (precondition for mission 2 satisfied)"
  worktree_inventory:
    status: reconciled
    evidence_ref: "2026-09-09 read-only git status; 4 untracked leftover paths preserved as unrelated WIP"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP; this Definition will own only the versioned-rename surfaces named at charter."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: read_only
      paths_or_digest: "clean except 4 untracked leftovers at draft receipt"
      recovery: leave_in_place
  candidates:
    - id: none
      ref: none
      base: none
      scope: []
      disposition: none
  observed_artifact:
    - claim: "No executable V2 desk vocabulary exists; the desired vocabulary lives only as architecture text. The sole executable new-desk residue is a raw spawn-authorization alias, never canonicalized or persisted."
      evidence_ref: "docs/designs/rlm-target-architecture-2026-09-04.md:79-85; internal/agentcore/rlm_reduce.go:82-85; RenameScout convergence note 2026-09-09"
    - claim: "Live alias normalization runs in two general canonicalizers before any version-selected decode: agentprofile.Canonical and modelpolicy.NormalizeRole both accept legacy aliases on the live path."
      evidence_ref: "internal/agentprofile/agentprofile.go:115-138; internal/modelpolicy/model_policy.go:172-185"
    - claim: "Mission-0's vocabulary_version seam gates ProjectionBase descriptors only; it does not select a V1/V2 tape or content role decoder."
      evidence_ref: "internal/projectionbase/types.go:22-33,63-113; internal/projectionbase/verify.go:17-48"
    - claim: "ProjectionBatch V1/V2 is an independent format discriminator, not a vocabulary version; it must not be confused with the absent vocabulary V2."
      evidence_ref: "internal/computerevent/projection_batch.go:11-21,157-215"
    - claim: "V1 role-bearing fields span twelve classes with scout-mapped file anchors: identities, digests, grants, mailboxes, assignments, runs, events, graph edges, prompts, policies, replay dispatch."
      evidence_ref: "RenameScout surface map 2026-09-09; docs/reports/choir-rlm-mission-state-2026-09-08.md rename section"
    - claim: "Mission 0 and mission 1 are both completed with deployed proof; the carried mission order names versioned rename next with Engineering as first executable carrier proof."
      evidence_ref: "docs/definitions/choir-rlm-restore-zero-2026-09-08.md now.status completed; docs/definitions/choir-rlm-settlement-gate-2026-09-09.md now.status completed; docs/reports/choir-rlm-mission-state-2026-09-08.md carried mission order"
  unknowns:
    - "V1 field sites beyond the scout-mapped anchors; the charter freezes the exhaustive inventory."
    - "Exact V2 writer/validator cutover list beyond the scout-mapped vocabulary surfaces."
    - "The mission-state counts of five overlay JSON tools and four legacy capsule operations; verify before freezing the carrier proof."
    - "Whether any live V1 tape content outside role fields constrains the frozen decoder."

finish:
  deliver: "Versioned rename is one durable truth per vocabulary version: V1 tape decodes under frozen V1 rules with original bytes preserved, V2 writers emit only the new desks vocabulary, unknown live values reject at activation, and the exhaustive V1 field inventory exists as a frozen artifact. Rename-first: no bundling, no alias period, no rename-last."
  artifact: "A frozen V1 field inventory artifact plus the version-selected decode/encode/activation contract across tape replay, writers, validators, policies, prompts, grants, and role-bearing persistence, with Engineering as the first executable carrier proof and a deployed staging proof."
  entrypoints:
    implementation:
      - "internal/agentprofile/agentprofile.go (canonical profiles, Canonical normalizer, PolicyFor)"
      - "internal/capsule/roles.go (broker roles and verb sets)"
      - "internal/promptstore/store.go and defaults (role registry, strict validation, prompt files)"
      - "internal/modelpolicy/model_policy.go (role selection, NormalizeRole)"
      - "internal/agentcore/rlm_reduce.go (FromRole, desk routing, spawn synonyms)"
      - "internal/computerevent/event.go, http_client.go, appender.go, projection_batch.go (V1 envelope, tape decode, replay dispatcher)"
      - "internal/projectionbase/types.go, publisher.go, verify.go, rebuilder.go (vocabulary_version seam extension)"
      - "internal/types/task.go, cosuper_assignment.go, evidence.go (V1 role-bearing schema)"
      - "internal/store/store.go, graph_store.go, lifecycle.go, cosuper_assignments.go, project.go (role-bearing persistence and digests)"
      - "internal/objectgraph/object.go (edge kinds and metadata)"
      - "internal/autoputer/run.go, internal/agentcore/rematerialize.go, restore_base.go (boot replay dispatch)"
  acceptance:
    - action: "Produce the exhaustive frozen V1 field inventory artifact covering all twelve classes (identities, digests, grants, mailboxes, assignments, runs, events, graph edges, prompts, policies, replay dispatch) with file:line anchors for every site; unknown or unanchored sites fail the inventory."
      proves: "The rename scope is bounded by evidence, not by assumption."
      evidence_class: local_test
    - action: "Add version-selected frozen V1 decode before the general canonicalizers: V1 tape decodes under frozen V1 rules with original bytes preserved, and replay of V1 tape is byte-identical through the appender path."
      proves: "History remains readable exactly as written; decode never silently modernizes."
      evidence_class: local_test
    - action: "Cut every live writer to the new desks vocabulary only (Management, Engineering, Research desks per the target architecture); no V1 role string originates from a V2 writer, and grant/policy/prompt surfaces validate the new vocabulary."
      proves: "New truth is written once, in one vocabulary."
      evidence_class: local_test
    - action: "Reject unknown live values at activation: any role or desk value outside the version-selected vocabulary fails closed before spawn, admission, or persistence."
      proves: "The unnamed can never execute or persist."
      evidence_class: local_test
    - action: "Retire the live alias path per rename-first: the general canonicalizers no longer accept legacy aliases on the live path, with no alias period and no rename-last fallback. V1 aliases survive only inside the frozen V1 decoder."
      proves: "One vocabulary is live; the old one is read-only history."
      evidence_class: local_test
    - action: "Deliver the Engineering carrier proof: the mission-state-counted overlay JSON tools and legacy capsule operations replaced or deleted (counts verified at charter), with replay returning original receipts."
      proves: "The first desk runs end-to-end on the new vocabulary."
      evidence_class: local_test
    - action: "Extend the vocabulary_version seam beyond descriptors to tape/content role decoding, and prove the full matrix on staging with effects OFF: V1 replay byte-identical, V2 write/activate clean, unknown-value refusal, and the Engineering carrier path, all bound to one attempt/computer/capsule/deployment identity with CI green."
      proves: "The rename holds on the physical staging computer, not just in unit tests."
      evidence_class: deployed_proof
    - action: "Run the focused contracts with no regression: replay byte-identity, writer vocabulary, activation refusal, grant/policy attestation, prompt selection, and projection-base validation."
      proves: "Rename preserves adjacent behavior on the contracts it touches."
      evidence_class: local_test
  rollback: "Revert the source commits and redeploy the prior accepted source; immutable events, reports, and recovery evidence remain. A retained V1-live writer path is a completion blocker. Product restore remains a forward event-chain transaction."
  landing:
    required: true
    environment: "staging https://choir.news"
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - "Any V1 role string originates from a live writer, validator, policy, or prompt path."
    - "Any unknown live role or desk value spawns, admits, or persists."
    - "Any V1 tape replay mutates bytes or applies modernized semantics."
    - "Any legacy alias is accepted outside the frozen V1 decoder."
    - "Any V1 field class lacks anchored inventory coverage."
    - "Only panel agreement, timing, local tests, or a deployment SHA exists in place of deployed proof."

boundaries:
  mutation_class: red
  drafting_mutation_class: green
  authority_sources:
    - "Owner-settled mission order; docs/reports/choir-rlm-mission-state-2026-09-08.md"
    - "docs/choir-doctrine.md; docs/computer-ontology.md"
    - "AGENTS.md; docs/standing-questions.md"
    - "Predecessors: docs/definitions/choir-rlm-restore-zero-2026-09-08.md; docs/definitions/choir-rlm-settlement-gate-2026-09-09.md"
  must_preserve:
    - "Rename-first: no bundling, no alias period, no rename-last."
    - "V1 tape decodes under frozen V1 rules with original bytes preserved."
    - "Problem-documentation-first: a code-free Define receipt precedes every repair-code commit."
    - "Simplification over addition: each receipt names the surface extended and every new module with evidence no surface carries its contract."
    - "Mission-0 drill debt stays mission-0-owned (residue R1); cutover stays remainder holder (residue R6)."
    - "Pre-A checkpoint 99949fe2 remains untouched as the self-development fence."
  excluded:
    - "Mission-0 restore and mission-1 settlement implementation (completed predecessors, consumed read-only)."
    - "Texture packet landing beyond the Engineering carrier proof; Research; Management."
    - "Prompt-bar diet, latency split, continuation census, shadow evaluations, native goals."
    - "Conductor-agentic behavior; styleguide control; provider/hill-climbing work."
    - "Candidate-A authoring, promotion, World Wire, tape deletion."
  protected_surfaces:
    - "internal/agentprofile/agentprofile.go"
    - "internal/capsule/roles.go"
    - "internal/promptstore/store.go and defaults"
    - "internal/modelpolicy/model_policy.go"
    - "internal/agentcore/rlm_reduce.go"
    - "internal/computerevent/event.go, appender.go, projection_batch.go, http_client.go"
    - "internal/projectionbase/types.go, publisher.go, verify.go, rebuilder.go"
    - "internal/types/task.go, cosuper_assignment.go, evidence.go"
    - "internal/store/store.go, graph_store.go, lifecycle.go, cosuper_assignments.go, project.go"
    - "internal/objectgraph/object.go"
    - "run acceptance and staging deployment routing"
  completion_evidence_floor: [local_test, deployed_proof]
  conjecture_delta:
    discovered:
      - "No V2 implementation waits unwired; the vocabulary must be built, not connected."
      - "Two general canonicalizers plus spawn synonyms form the live alias path that version-selected decode must precede."
    falsifiers:
      - "V1 tape contains role-bearing content outside every inventoried class; then the inventory is incomplete and the freeze waits."
      - "A live consumer requires V1 role strings at runtime; then rename-first needs re-scoping before completion."
  heresy_delta:
    discovered:
      - "Live alias normalizers accepting legacy values on the write path."
      - "Vocabulary seam gating descriptors while tape decode stays unversioned."
      - "Raw spawn synonyms canonicalizing nothing and persisting nothing."
    introduced: []
    repaired: "none; mark repaired only after terminal deployed receipts"

measures:
  - name: v1_inventory_coverage
    kind: gate
    baseline: "Scout-mapped anchors across twelve classes; exhaustiveness unproven."
    desired: "Every V1 role-bearing site anchored; unknown sites fail the inventory."
    decision_use: "Blocks the V1 freeze while sites remain unanchored."
    cannot_prove: "Cannot prove absence of uninventoried content."
  - name: writer_vocabulary_purity
    kind: gate
    baseline: "Live writers emit V1 role strings."
    desired: "Zero V1 role strings originate from live writers; unknown values refuse at activation."
    decision_use: "Blocks completion while V1 remains writable."
    cannot_prove: "Cannot prove runtime activation reachability beyond the refusal tests."
  - name: replay_byte_identity
    kind: gate
    baseline: "V1 replay unmeasured against original bytes."
    desired: "V1 tape replay is byte-identical through the appender path."
    decision_use: "Blocks the frozen-decoder cutover on any mutation."
    cannot_prove: "Cannot prove decoder totality beyond the inventoried classes."
  - name: panel_agreement
    kind: weak_signal
    baseline: "Draft consensus pending."
    desired: "No additional agreement threshold."
    decision_use: "Explains draft review scope only."
    cannot_prove: "Cannot authorize promotion, prove code, or advance completion."

now:
  status: blocked_incomplete
  slice: "draft under owner review; executable only after charter ratification with fresh reconciliation"
  question: "Is this draft, with scout-mapped surfaces and rename-first direction, ready for consensus review and charter ratification?"
  reconciliation:
    observed_at: "2026-09-09T23:29:36Z"
    source_ref: "main@34629502"
    deploy_identity: "staging https://choir.news ok via proxy 0475ed84; missions 0 and 1 completed with deployed proof"
    authority_identities:
      - "docs/reports/choir-rlm-mission-state-2026-09-08.md (carried mission order, rename-first)"
      - "docs/designs/rlm-target-architecture-2026-09-04.md:79-85 (desks vocabulary)"
      - "docs/definitions/choir-rlm-restore-zero-2026-09-08.md (completed predecessor)"
      - "docs/definitions/choir-rlm-settlement-gate-2026-09-09.md (completed predecessor)"
      - "docs/mission-residues.md (R1, R6 open)"
      - "AGENTS.md; docs/standing-questions.md"
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "4 untracked leftover paths preserved as unrelated WIP; re-observe before charter"
    status: reconciling
  candidate:
    id: none
    state: none
    ref: none
    owner: none
    base: none
    digest: none
    scope: []
  decision:
    selected: "Mission 2 charters versioned rename as drafted after review; closes no predecessor acceptance; retains cutover remainder-holder status and mission-0 drill debt ownership."
    kind: architecture
    status: proposal
    source: orchestrator
    evidence_ref: "Owner-settled mission order and rename-first direction; scout surface map 2026-09-09"
    owner_ratification_ref: "charter ratification still required before execution"
    recorded_at: "2026-09-09T23:29:36Z"
    consequence: "Draft and review only. No rename repair executes under this file before charter ratification with fresh reconciliation."
  evidence_refs:
    - "internal/agentprofile/agentprofile.go"
    - "internal/capsule/roles.go"
    - "internal/computerevent/event.go and appender.go"
    - "internal/projectionbase/types.go"
  blocker_or_risk: "Charter ratification pending. V1 inventory exhaustiveness unproven beyond scout anchors; tool/operation counts unverified; live V1-runtime-consumer risk open via falsifier."
  next_action: "Owner review of this draft; consensus review if directed; then charter ratification with fresh reconciliation."

receipts: []
