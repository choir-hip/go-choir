---
definition_version: 2
definition_id: choir-rlm-restore-zero-2026-09-08
execution_mode: mission_orchestrator

start:
  captured_at: "2026-09-08T23:46:42Z"
  source:
    canonical_ref: "main@a5dea315c25f10f093800194a9e701ad1561e1da"
    deploy_identity: "Panel observation: staging https://choir.news health reported build 3ef4405c91c63bf048e34fcdd7df2d3fb4755bbf; diagnostic only, not product acceptance. Guest CodeRef, epoch, effects mode, and pre-A fence liveness are unknown at this capture."
  worktree_inventory:
    status: reconciled
    evidence_ref: "2026-09-08 panel reconciliation: git status --short empty; single worktree /Users/wiz/go-choir. Re-observe before any red mutation."
    preservation_rule: "Preserve every subsequently discovered dirty path, non-primary worktree, recovery journal, quarantine, base artifact, and unrelated WIP. Do not touch unclassified work. This mission owns only named restore/recovery surfaces."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: read_only
      paths_or_digest: "clean at opening panel receipt"
      recovery: leave_in_place
  candidates:
    - id: none
      ref: none
      base: none
      scope: []
      disposition: none
  observed_artifact:
    - claim: "RematerializeFromTape opens a fresh staged store, then ReconstructThroughTarget begins from a nil/zero local head; it never consumes a ProjectionBase."
      evidence_ref: "internal/agentcore/rematerialize.go:92-107; internal/computerevent/appender.go:693-707,959-988"
    - claim: "Only boot reconstruction consumes a ProjectionBase. Missing capability/watermark, HTTP or decode failure can return materialized=false without an error; runReplayPhase logs deferral and reconstructs anyway. Blob-digest mismatch is already an error."
      evidence_ref: "internal/autoputer/projection_base.go:29-72,112-114; internal/autoputer/run.go:551-559"
    - claim: "ProjectionBase Descriptor has computer_id, sequence, canonical_head, blob_sha256, reducer_version, schema_version, and vm_local_content_witness, but validation/handshake do not make the full tuple mandatory and no vocabulary_version exists."
      evidence_ref: "internal/projectionbase/types.go:51-80; internal/platform/file_cas_http.go:27-28,183-186"
    - claim: "recover_current is non-rewinding: ColdRecoverRequest admits computer_id, expected_canonical_head, expected_route_generation, and idempotency_key; strict decoding and proxy behavior reject checkpoint/historical input before host mutation."
      evidence_ref: "internal/vmctl/cold_recover.go:22-28,258-260,408-424; internal/proxy/computer_lifecycle.go:360-365"
    - claim: "ColdRecoveryVerifier currently establishes replay-completeness/serving identity but does not itself establish verified-base ancestry or tail-only work."
      evidence_ref: "internal/vmctl/cold_recover.go:112-115; internal/vmctl/recovery_authorities.go:62-129"
    - claim: "The active target-architecture Definition has execution proof but the later mission record withholds run acceptance for terminal digest conflict; restore-zero is a neighboring recovery gate, not that repair."
      evidence_ref: "docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md:149-194; docs/reports/choir-rlm-mission-state-2026-09-08.md:53-81"
  unknowns:
    - "Current source/deploy/worktree identities at implementation start; a fresh read-only reconciliation is required."
    - "Whether a verified, retained ProjectionBase compatible with the staging computer and a nontrivial tail exists."
    - "The retained-base selection/ancestry index for historical H older than the latest watermark."
    - "Measured full-path recovery cost and complete V1 role-bearing field inventory."

finish:
  deliver: "A Choir computer restores from a verified ProjectionBase at watermark W plus immutable tail (W,H], with canonical event/Dolt head H as the restore address and the materialized witness as independent proof. Every supported recovery path refuses a missing, foreign, corrupt, non-ancestor, or incompatible required base; it never silently replays genesis. The final head and witness are verified before publication, and interruption resumes from durable verified progress. recover_current remains current-head-only."
  artifact: "A deployed staging restore contract shared by boot reconstruction, rematerialize/restore, replay-completeness/cold verification, and owner-scoped cold recovery: versioned immutable descriptors and retained-base selection; one verified installation/ancestry predicate; tail-only replay; durable operation/journal receipts; pre-publication head/witness verification; and no-SSH product-path proof. Offline `choir-rebuild-base` publishes compatible immutable bases but is not itself recovery proof."
  entrypoints:
    cli:
      - "choir computer rematerialize-from-tape -> POST /api/computers/{id}/lifecycle/rematerialize-from-tape"
      - "choir computer restore -> POST /api/computers/{id}/lifecycle/restore"
    guest_and_owner_routes:
      - "POST /api/computers/{id}/lifecycle/cold-recover (current-head-only)"
      - "POST /internal/vmctl/computers/{id}/cold-recover"
      - "GET /api/computers/{id}/self-development/replay-completeness"
    implementation:
      - "agentcore.Runtime.RematerializeFromTape and restore handlers"
      - "computerevent.ComputerEventAppender Reconstruct, ReconstructThroughTarget, RebindProjection"
      - "projectionbase Descriptor, publisher/rebuilder, installer, and retained-base resolution"
      - "autoputer.materializeProjectionBaseIfNeeded and runReplayPhase"
      - "vmctl.HTTPRecoveryVerifier.VerifyRecovery"
  acceptance:
    - action: "Focused base-contract tests build and consume a descriptor binding computer_id, W, canonical head at W, blob SHA-256, reducer_version, schema_version, vocabulary_version, and VM-local content witness. Independently mutate or omit each binding; also test W after H and a non-ancestor W."
      proves: "A base is a verified accelerator for this computer and chain position, never a parallel head or authority."
      evidence_class: local_test
    - action: "Exercise boot reconstruction, rematerialize, historical restore, replay-completeness, and cold recovery with missing, foreign, corrupt, partial-install, incompatible, and witness-mismatched required bases."
      proves: "Every named recovery surface returns a typed visible refusal, leaves the active route/accepted realization unchanged or quarantined, and cannot silently enter a non-bootstrap genesis replay."
      evidence_class: local_test
    - action: "Freeze target head H and sequence before reconstruction. Seed an isolated projection with a verified W and reconstruct to H for W=H, one-event, multipage, and concurrent-append fixtures. Instrument event enumeration, payload reads, reducer application, and durable applied-sequence spans across base resolution, replay, and mandatory verification. Assert the durable journal applies every event in (W,H] exactly once, no event at or before W is enumerated/fetched/reduced for recovery, and post-H appends are outside this operation. Report bounded ancestry/index lookup separately; verify H and an independent witness before rebind/swap/publication."
      proves: "Recovery work excludes the prefix and publication is fenced by canonical head plus witness, not merely blob identity or ComputerVersion."
      evidence_class: local_test
    - action: "Fault-inject base download, unpack, verified installation, durable tail batch, final verification, workspace flip, and route publication; retry the same recovery identity after each interruption."
      proves: "No partial store is served, no semantic event is duplicated, and recovery resumes from W or later durable tail progress rather than genesis."
      evidence_class: local_test
    - action: "Retain strict recover_current request decoding and fencing tests while exercising the shared verification predicate; checkpoint/historical/base-override fields must refuse before mutation and a valid request must re-read the current canonical head."
      proves: "Shared restore machinery does not turn current recovery into rewind authority."
      evidence_class: local_test
    - action: "Bind vocabulary_version into the ProjectionBase descriptor: a missing or unknown descriptor vocabulary version refuses at base publication/consumption before installation. Replay frozen V1 tape fixtures with original bytes under frozen V1 rules; unknown live versions fail closed. This mission introduces no live alias/fallback, live-writer change, role-bearing-field inventory, or V2/desk migration."
      proves: "Historic evidence remains decodable without preserving an alternate live protocol."
      evidence_class: local_test
    - action: "On staging through scoped CLI/product APIs and without SSH, select or publish a verified compatible base W with a nontrivial tail, run rematerialize/restore and owner-scoped recover_current, and fetch base-ancestry, applied-tail, final head, witness, verifier, serving-join, and route-publication receipts."
      proves: "The deployed product path uses the asserted base-plus-tail contract and remains non-rewinding."
      evidence_class: deployed_proof
    - action: "Before the drill, name the scoped owner CLI/API failure-injection and receipt-retrieval controls. Use an isolated/disposable staged realization and preserve the accepted route and immutable base. Define recovery identity as computer_id + operation_id/idempotency_key + frozen target H + selected descriptor digest + fencing state. Refuse a selected foreign/incompatible/corrupt required base before workspace or route mutation. Interrupt only through the named product-path control after a durable tail checkpoint, then resume in a fresh process with the identical identity; a changed identity conflicts. Retrieve refusal, checkpoint/restart, applied-tail, verification, and publication receipts. If these controls do not exist, record a blocked prerequisite; do not substitute SSH, a base override on recover_current, or a same-process retry."
      proves: "The production boundary fails visibly, retains durable evidence/quarantine, and resumes without a hidden genesis fallback."
      evidence_class: deployed_proof
    - action: "Keep cost-by-tail as non-completion telemetry: record recovery receipt counts for prefix events skipped, event enumeration/payload reads/reductions, and durable batch commits, requiring zero recovery-critical prefix replay and counts matching the known tail, while reporting ancestry lookup, base transfer, unpack, witness, and wall time separately. Fixed-fixture prefix-age comparisons belong in local instrumentation evidence; wall-time trends cannot satisfy acceptance or advance completion."
      proves: "The observed recovery-critical replay work is tail-bounded rather than lifetime replay."
      evidence_class: deployed_proof
  rollback: "Before publication, refuse and retain the journal, staged/quarantined realization, descriptor, and tail receipt on any ancestry, replay, witness, fencing, or verifier failure. After landing, revert the source commits and redeploy the prior accepted source; immutable events, descriptors, bases, and recovery evidence remain. A product restore is a forward event-chain transaction, not a platform-deploy rollback and never deletion/rewriting of canonical tape. Do not add a live legacy fallback; retain only the versioned V1 replay decoder. Pre-A checkpoint 99949fe2 remains the self-development fence, not this mission's rollback target."
  landing:
    required: true
    environment: "staging https://choir.news"
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance, recovery_operation_and_verifier_ids]
  not_done_when:
    - "Any non-bootstrap recovery path can proceed from a required-base failure by fresh-store/genesis replay."
    - "Any named entrypoint or its mandatory verifier still lifetime-replays, or uses a different base/verification contract."
    - "Descriptor/selection fails to bind computer, W/head ancestry, reducer/schema/vocabulary compatibility, digest, and witness."
    - "A partial install or restart can be mistaken for a verified base/tail checkpoint."
    - "recover_current accepts a historical selector or publishes before current-head/witness/fencing verification."
    - "V1 evidence requires byte rewrite, live aliases, or a legacy live fallback."
    - "Only panel agreement, timing, local tests, a deployment SHA, or an unresolved candidate exists in place of deployed proof."
    - "The cutover digest-conflict remainder is reported as repaired by this mission, or a second working spine is created."

boundaries:
  mutation_class: red
  drafting_mutation_class: green
  authority_sources:
    - "Owner-settled restore-zero direction in this drafting charge"
    - "docs/reports/choir-rlm-mission-state-2026-09-08.md"
    - "docs/choir-doctrine.md; docs/computer-ontology.md"
    - "AGENTS.md; docs/standing-questions.md"
    - "Adjacent active authority: docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md"
  must_preserve:
    - "Canonical immutable computer events and H are the single semantic history and restore address; ProjectionBase is only a verified accelerator."
    - "The materialized witness proves reconstructed content; a witness's embedded Dolt head, blob digest, or ComputerVersion cannot substitute for H."
    - "Only (W,H] is replayed after accepting W; an explicit new-computer bootstrap W=0 is separate from recovery and cannot be reached by failure fallback."
    - "recover_current is non-rewinding and independently fenced to the current canonical head."
    - "V1 historic bytes remain decodable under frozen versioned replay rules; live writers/callers have no legacy fallback."
    - "One ComputerEventAppender remains semantic writer; base installation/replay progress cannot mint semantic events."
    - "Final witness/head/verifier/route checks precede exposure; shared platform/world-wire and irreversible external effects are outside a user-computer restore."
    - "Pre-A checkpoint 99949fe2 remains untouched as the self-development fence."
    - "Problem-documentation-first: the first executable mutation boundary is a code-free Define commit that records genesis-by-default rematerialization, silent required-base deferral, and missing ancestry/compatibility verification; their evidence, authorized scope, rollback, and atomic topology disposition; and no repair code or repair-code candidate. Record its immutable commit in a boundary-define receipt with problem_ref, authorization_ref, and registry_conformance_ref. Repair commits cite that prior Define."
    - "Simplification over addition: each Define/Implement receipt names the existing surface extended, the silent-fallback/genesis-by-default path deleted or made unreachable (with affected citers), and every new package/module with concrete evidence that named existing surfaces cannot carry its contract. No-deletion-applicable requires a reachability finding. A retained flawed path is a completion blocker. The end state reads as how the substrate should have been all along."
    - "Mission 0 and mission 1 may rehearse concurrently, but commits on shared files, deployment, route, and registry authority serialize."
  excluded:
    - "Mission-1 terminal semantic identity, compile/runtime classification, one-shot unification, fate saga, and singleton-detector replacement."
    - "Mission-2 desk rename implementation and exhaustive V1 field inventory; this mission only establishes the base vocabulary-version seam."
    - "Engineering, Texture, Research, Management, prompt-bar, continuation deletion, shadow evaluations, and provider/hill-climbing work."
    - "Conductor-agentic behavior."
    - "Styleguide control and report-formatting control."
    - "Native goals v4 or Definition-harness machinery."
    - "Candidate-A/self-development authoring or promotion, World Wire, tape deletion, and historical-definition reopening."
  protected_surfaces:
    - "internal/agentcore/rematerialize.go and restore/replay-completeness handlers"
    - "internal/computerevent/appender.go"
    - "internal/autoputer/projection_base.go and internal/autoputer/run.go"
    - "internal/projectionbase/*"
    - "internal/platform/file_cas.go and file_cas_http.go"
    - "internal/vmctl/cold_recover.go and recovery_authorities.go"
    - "internal/proxy/computer_lifecycle.go"
    - "internal/agentcore/replay_completeness.go, checkpoint_restore_bindings.go, and api_self_development.go lifecycle/replay routes (mandatory verifier carrier; classified 2026-09-09)"
    - "internal/proxy/self_development.go and handlers.go replay-completeness path (deployed verifier transport; classified 2026-09-09)"
    - "internal/vmctl TrustedGuestKeyCopier seam (sole guest-data path in cold recovery; classified 2026-09-09)"
    - "file-CAS hydration seam invoked in runReplayPhase (durable replay input; classified 2026-09-09)"
    - "cmd/choir/main.go and cmd/choir-rebuild-base/main.go"
    - "canonical event-head, checkpoint/route projection, recovery journal/quarantine, and staging deployment routing"
  completion_evidence_floor: [local_test, deployed_proof]
  conjecture_delta:
    discovered:
      - "One verified base-selection/install/replay contract can remove lifetime recovery replay without changing reconstructed state."
      - "A durable base/tail checkpoint plus fenced publication can resume recovery without trusting process-local state."
    falsifiers:
      - "Any required state cannot be authenticated from immutable descriptor, canonical tail, and witness."
      - "A restart cannot identify verified W or durable tail progress without a mutable process-only fact."
      - "Fixed-tail recovery still enumerates/reduces the prefix after accounting for base/witness fixed costs."
  heresy_delta:
    discovered:
      - "Rematerialization bypasses the existing base substrate and reconstructs from nil."
      - "Boot base discovery can silently fall through to genesis reconstruction."
      - "The current descriptor/handshake and cold verifier do not establish full base ancestry/tail-only proof."
    introduced: []
    repaired: "none; mark repaired only after terminal deployed receipts"

measures:
  - name: prefix_event_reads
    kind: weak_signal
    baseline: "Fresh rematerialization begins at genesis."
    desired: "zero recovery-critical reads/reductions at or before W; resumed runs exclude committed tail progress."
    decision_use: "Detects a bypassed base or a verifier/entrypoint still scanning the prefix."
    cannot_prove: "Cannot prove ancestry, reconstructed-state equivalence, publication safety, or absence of uninstrumented work."
  - name: recovery_cost_by_tail
    kind: telemetry
    baseline: "Unknown on staging; full-path genesis and tail distributions were not measured at opening."
    desired: "At fixed build/environment, variable replay work follows |(W,H]|; base transfer, unpack, and witness costs are reported separately."
    decision_use: "Chooses base cadence/fixture and exposes hidden lifetime scans."
    cannot_prove: "Cannot prove authority, correctness, crash safety, or completion."
  - name: required_base_refusal_matrix
    kind: gate
    baseline: "Boot has succeeds-with-nothing discovery paths; rematerialize does not consult a base."
    desired: "Every required-base failure class yields typed refusal and no recovery genesis fallback."
    decision_use: "Blocks completion and identifies missing failure coverage."
    cannot_prove: "Cannot prove a valid base recreates correct state."
  - name: nonrewind_contract
    kind: gate
    baseline: "Strict current-head ColdRecoverRequest rejects historical fields."
    desired: "Unchanged after shared restore implementation."
    decision_use: "Blocks any accidental historical authority in recover_current."
    cannot_prove: "Cannot prove historical restore/rematerialize is tail-only."
  - name: panel_agreement
    kind: weak_signal
    baseline: "11/13 convergent panel plus recovered Fable solo; panel Fable and hy3 failed."
    desired: "No additional agreement threshold."
    decision_use: "Explains the draft's order and review scope only."
    cannot_prove: "Cannot authorize promotion, prove code, or advance completion."

now:
  status: working
  slice: "reconciliation landed 2026-09-09 (read-only receipt); next is implement preparation for slice 1 (descriptor + vocabulary_version + verified ancestry predicate)"
  question: none
  reconciliation:
    observed_at: "2026-09-09T01:30:00Z"
    source_ref: "main@24be54a29253229080cd1b1f5a3cc94c647bfdea (clean; single primary worktree /Users/wiz/go-choir; unrelated worktrees preserved in place)"
    deploy_identity: "staging proxy build 3ef4405c91c63bf048e34fcdd7df2d3fb4755bbf (built 20260908151005), vmctl_status ok; source ahead by docs-only delta. Staging computer/guest/epoch/effects/fence and base-retention state still require scoped-auth re-observation before red mutation."
    authority_identities:
      - "docs/reports/choir-rlm-mission-state-2026-09-08.md"
      - "docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md"
      - "docs/standing-questions.md"
      - "docs/ACTIVE.md; docs/mission-graph.yaml (discovery only); docs/doc-authority-manifest.yaml"
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "docs/evidence/choir-rlm-restore-zero-reconciliation-2026-09-09.md section 1"
    status: reconciled
  candidate:
    id: none
    state: none
    ref: none
    owner: none
    base: none
    digest: none
    scope: []
  decision:
    selected: "Mission 0 delivers verified-base-plus-tail restore; mission 1 retains settlement/digest work; mission 2 requires deployed acceptance of both. Restore-zero is a predecessor/gate, not a cutover duplicate."
    kind: architecture
    status: settled
    source: owner
    evidence_ref: "Owner-settled drafting charge; docs/reports/choir-rlm-mission-state-2026-09-08.md"
    owner_ratification_ref: "not_applicable for direction; executable promotion still requires atomic registry/topology receipt"
    recorded_at: "2026-09-08T23:46:42Z"
    consequence: "Draft and reconcile only. A code-free Define may follow the read-only receipt; repair code waits for that Define and for red-surface candidate classification."
  evidence_refs:
    - "internal/agentcore/rematerialize.go"
    - "internal/autoputer/projection_base.go"
    - "internal/computerevent/appender.go"
    - "internal/projectionbase/types.go"
    - "internal/vmctl/cold_recover.go"
    - "internal/vmctl/recovery_authorities.go"
    - "docs/evidence/choir-rlm-restore-zero-reconciliation-2026-09-09.md"
  blocker_or_risk: "Staging computer/guest/epoch/effects/fence and base-retention identities still require scoped-auth re-observation before red mutation or drill. Caller map and surface classification landed in docs/evidence/choir-rlm-restore-zero-reconciliation-2026-09-09.md sections 3-4."
  next_action: "Implement preparation for slice 1 (descriptor + vocabulary_version + verified installation/ancestry predicate with simplification adjudication). No repair code until preparation names the exact surfaces and citers."
receipts:
  - id: restore-zero-define-and-topology-2026-09-09
    boundary: define
    commit_or_artifact: "24be54a29253229080cd1b1f5a3cc94c647bfdea"
    proof_refs:
      - "docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md remainder disposition receipt"
      - "docs/definitions/choir-rlm-settlement-gate-2026-09-09.md blocked stub"
      - "docs/reports/choir-rlm-mission-state-2026-09-08.md promotion-gate section"
    rollback_ref: "registry-only change; revert restores prior topology"
    disposition: "restore-zero promoted to sole working entrypoint; cutover to blocked remainder holder; mission 1 to blocked stub"
    problem_ref: "genesis-by-default rematerialization; silent required-base deferral; missing ancestry/compatibility verification"
    authorization_ref: "Owner topology answers 2026-09-09: cutover Blocked remainder holder; mission 1 Blocked stub"
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "docs/ACTIVE.md; docs/mission-graph.yaml; docs/doc-authority-manifest.yaml"
  - id: restore-zero-reconciliation-2026-09-09
    boundary: define
    commit_or_artifact: "this commit (reconciliation receipt; green docs-only)"
    proof_refs:
      - "docs/evidence/choir-rlm-restore-zero-reconciliation-2026-09-09.md (caller map sections 3-4; surface classification; live source/deploy identities)"
    rollback_ref: "docs-only; revert restores prior Definition text"
    disposition: "read-only reconciliation complete; red-surface candidates classified and protected_surfaces extended; scoped-auth staging identities carried as explicit unknowns"
    problem_ref: "genesis-by-default rematerialization; silent required-base deferral; missing ancestry/compatibility verification"
    authorization_ref: "Owner topology answers 2026-09-09 (sole working entrypoint)"
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: "staging proxy 3ef4405c91c63bf048e34fcdd7df2d3fb4755bbf observed anonymously; scoped computer/base identities pending"
      deployed_acceptance: not_applicable
    registry_conformance_ref: "no topology change; docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml unchanged and still conformant"
