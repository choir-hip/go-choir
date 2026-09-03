---
definition_version: 2

start:
  captured_at: "2026-09-03T02:20:00Z"
  source:
    canonical_ref: "main@cb7804f9"
    deploy_identity: "staging https://choir.news; behavior-bearing deployed commit 42d476044ec80efe8a31d043af577ad77ba7572b (2026-08-28); guest label 7bd488cd is docs-only; retained computer computer-03335285269bdba4f94377e56879f9e6 active epoch 844; pre-A checkpoint 99949fe2 published and verified as immutable restore fence; effects OFF"
  worktree_inventory:
    status: reconciled
    evidence_ref: "2026-09-03 git status; false completion flip reverted; problem receipt + partial evidence + this revision uncommitted pending consensus adjudication commit"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP; this Definition owns itself, its receipts, and the Super wake, scheduling, and vmctl boot surfaces."
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      recovery: leave_in_place
  candidates:
    - id: none
      ref: none
      base: none
      scope: []
      disposition: none
  observed_artifact:
    - claim: "Pre-A checkpoint 99949fe2 remains the published, verified, immutable restore fence on computer-03335285269bdba4f94377e56879f9e6."
      evidence_ref: "docs/evidence/effects-red-pre-a-checkpoint-published-2026-08-19.md; docs/ACTIVE.md"
    - claim: "5/5 product-path normal boots passed 2026-09-03 (epochs 838-843) without RUNTIME_MAINTENANCE_HOLD; residual scans did not hang boot. Boot-probe criterion partially satisfied; full landing receipts still owed."
      evidence_ref: "docs/evidence/effects-red-substrate-boot-probe-partial-evidence-2026-09-03.md"
    - claim: "ArrivalOrdinal allocation and durability proven on staging (ordinals 2 and 3 survived restart); selection and FIFO execution never observed because no Super woke. Panel caveat (grok46, medium confidence): the ordinals are Texture execution_request lifecycle controls — Super-admissibility must be confirmed before the FIFO test runs."
      evidence_ref: "docs/evidence/effects-red-substrate-boot-probe-partial-evidence-2026-09-03.md"
    - claim: "Boot-as-scheduler is the substrate defect: reconcile selects pending backlog as a wake source at boot and after terminal events; storm-stop held across 6 boots 2026-09-03 (because claimedPersistentSuperProducerReportIDs suppression held, not because boot stopped scheduling) but the ontology remains wrong."
      evidence_ref: "docs/evidence/effects-red-boot-as-scheduler-ontological-error-2026-09-03.md"
    - claim: "The narrow in-flight resume carve-out is INTENDED but NOT structurally isolated: rewarmInterruptedPersistentSuperActors (runtime.go:2220) calls full reconcilePersistentSuperActor (runtime.go:2267), which falls through the entire selection ladder (listPendingPersistentSuperLifecycleControls -> maybeRewakeSelfDevelopmentTextureAfterTerminalSuper -> resumeSelfDevelopmentSuperForPendingInstruction -> listAndSettlePersistentSuperBacklog -> listPendingPersistentSuperAdmissibleReports) when reactivateRestartedPersistentSuperControlRun misses. Its doc comment ('does not create a new Super from arbitrary backlog') is aspirational, not enforced."
      evidence_ref: "internal/agentcore/super_controller.go:362-427; internal/agentcore/runtime.go:2215-2273"
    - claim: "Proven live wake paths that must survive the repair: ApplyTextureTurn -> wakeTextureControl -> WakeUpdatedCoagent (internal/textureowner/texture_turn_runtime.go:236-245); ensureSelfDevelopmentTextureJoin -> reconcilePersistentSuperActor (internal/agentcore/selfdev_texture_join.go:55-59, proven 2026-08-20); CoSuper-cancel Texture rewake (internal/agentcore/cosuper_assignment_fate.go:218-221); update_coagent delivery (tools_worker_update.go); exported owner API (internal/agentcore/texture_tool_api.go:48)."
      evidence_ref: "docs/evidence/effects-red-super-texture-rewake-2026-08-20.md"
  unknowns:
    - "Whether the live Texture rewake path survives terminal-continuation narrowing without degradation — carried as acceptance criterion 5 rather than assumed."
    - "Scope boundary: boot also reconciles Researcher and generic actors (runtime.go:603-613, 2581-2601); this Definition constrains Super and CoSuper minting only, and records the generic-actor boot boundary as unpaid scope."

finish:
  deliver: "The persistent staging computer (computer-03335285269bdba4f94377e56879f9e6) executes competing Texture execution requests in strict FIFO arrival-ordinal order under live-trigger-only Super wakes; boot is a structurally isolated recovery event that can never schedule work (exact-run resume entry point only); terminal events wake Texture, never select backlog; the nine 08-19 cancel producer reports are settled at the store layer as late evidence; and product-path normal boots reach guest health 200 without RUNTIME_MAINTENANCE_HOLD."
  artifact: "Deployed staging proof: live Texture triggers select exactly one work item per cycle in FIFO order while later requests wait untouched; a dedicated exact-run resume entry point structurally cannot reach backlog selection; boot with admissible unclaimed backlog mints zero Super runs (positively asserted, selector-depth precondition recorded); terminal Super with pending backlog mints zero successors while the Texture rewake path still works; nine producer reports carry store-level CAS settlement receipts and all pending selectors exclude them; boot-probe receipts with full landing chain."
  acceptance:
    - action: "Confirm >= 3 pending ordinalized Super-admissible requests on staging (verify ordinals 2/3 admissibility; queue additional requests via the product API as needed). For each live cycle, deliver the named trigger — an owner Texture turn whose typed execution_request addresses the persistent Super (ApplyTextureTurn -> wakeTextureControl path) — and observe via /api/runs exactly one new Super run whose selected work item is the lowest pending ordinal, recording per-cycle: request update ID, ArrivalOrdinal, trigger ID, selected assignment, parent run ID, terminal disposition. Repeat for >= 3 cycles; later requests remain pending with delivered_to_run_id null; zero supersession (I26-permitted expiry by deadline or owner correction is not supersession)."
      proves: "Live-trigger-only wakes select work in computer-scoped arrival-ordinal order; the wake is deleted, not the selection; competing requests never cancel an executing assignment."
      evidence_class: deployed_proof
    - action: "PRECONDITION: before boot, record that at least one admissible, unclaimed backlog item exists (pre-repair selector depth > 0, evidenced in a boot log line). Execute one product-path boot (choir computer refresh); snapshot /api/runs filtered to the persistent Super agent before and after; assert across the window [refresh start, guest /health 200] that zero Super or CoSuper run rows are created, except an exact runtime_restarted resume of a pre-existing run ID; assert from boot logs that reconcile never entered selection (positive did-not-enter assertion, not mere row absence); assert pending ordinals remain pending with unchanged delivery cursor and lifecycle version."
      proves: "Boot is a structurally isolated recovery event; backlog never auto-executes at boot even when admissible and unclaimed (criterion would have failed on unrepaired code)."
      evidence_class: deployed_proof
    - action: "While an assignment is executing, restart via product path; assert the exact interrupted run resumes through the dedicated isolated resume entry point with the SAME run ID and passivated_reason=runtime_restarted, no duplicate assignment is created, and the resume did not fall through to any selection call (boot log)."
      proves: "Rare-reboot resume works for in-flight work only, per I26 durability without re-supersession."
      evidence_class: deployed_proof
    - action: "Enumerate the nine undelivered 08-19 CoSuper cancel producer reports (pending producer reports with requested_by_agent_id=co-super:assignment-97191e37-657c-5acf-af18-f1c80d09def2; record full IDs in the settlement receipt). Settle them via a dedicated runtime/store lifecycle-reducer command with CAS precondition (each report still pending and undelivered), terminal disposition, and idempotent settlement receipt. Assert: all pending store selectors exclude settled IDs; the claimedPersistentSuperProducerReportIDs metadata claim-scan is retired (replaced, not coexisting); a boot after settlement mints zero Super runs referencing them."
      proves: "Stale residue is settled terminally at the store layer; no reconcile path can resurrect it; deletion over addition (claim-scan retired)."
      evidence_class: deployed_proof
    - action: "Terminal-event probe, positive and negative: (a) terminate a Super while >= 1 admissible unclaimed backlog item exists and assert zero successor Super is minted from undelivered backlog (maybeContinuePersistentSuperInbox path); (b) then prove the live Texture rewake path intact: terminal Super -> Texture instruction (maybeRewakeSelfDevelopmentTextureAfterTerminalSuper) -> owner-visible Texture turn -> NEW typed execution_request -> exactly one new Super, without HTTP operations POST."
      proves: "Terminal events wake Texture, never select backlog; the proven live rewake path survives the narrowing."
      evidence_class: deployed_proof
    - action: "Execute product-path boot probe (>= 5 consecutive normal boots without hold parameters; guest /health 200 within a 60s hard timeout per boot) under the full landing receipt chain [source_commit, ci, deploy, environment_identity, deployed_acceptance]."
      proves: "Guest reaches health 200 within deadline without RUNTIME_MAINTENANCE_HOLD; residual scans do not hang normal boot; landing chain is coherent."
      evidence_class: deployed_proof
  rollback: "Git revert of wake-path and scheduling commits; checkpoint 99949fe2 remains the immutable pre-A fence."
  landing:
    required: true
    environment: staging
    required_receipts: [source_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - "A Super or CoSuper run is minted from pending backlog — lifecycle controls, mailbox backlog, or producer reports — at boot, on refresh, after a terminal event, or through any reconcile entry."
    - "The isolated resume entry point falls through to any selection call, or a replacement Super is minted from the settled producer reports."
    - "A terminal event selects a pending control or report directly instead of waking the Texture agent."
    - "Settlement coexists with the claimedPersistentSuperProducerReportIDs claim-scan instead of replacing it, or any pending selector can still return settled report IDs."
    - "A live wake path named in observed_artifact (wakeTextureControl, ensureSelfDevelopmentTextureJoin, update_coagent delivery, CoSuper-cancel Texture rewake) regresses."
    - "Competing Texture execution requests trigger cancellation of an in-flight assignment (I26-permitted deadline expiry or owner correction excepted)."
    - "Normal boot hangs or requires RUNTIME_MAINTENANCE_HOLD to reach health 200."
    - "More than one CoSuper assignment is live simultaneously."
    - "Candidate A authoring or RLM session interpreter coding is attempted in this Definition."

boundaries:
  mutation_class: red
  authority_sources:
    - "Choir Doctrine I26 (Super singleton coherence, scheduling contract: one Super per ComputerID, at most one active Super run, one live CoSuper assignment, FIFO among non-expired, fail-closed deadlines)"
    - "docs/computer-ontology.md:130-133 (reconstruction never reruns a model, tool, or network observation; exact-run resume is live-work continuation, not reconstruction)"
    - "Owner direction 2026-09-03 (boot is recovery, not scheduler; always-on steady state; live-trigger-only wakes)"
    - "docs/evidence/effects-red-boot-as-scheduler-ontological-error-2026-09-03.md (repair direction revised after 10-model consensus)"
  must_preserve:
    - "Two writer classes: AuthorUser performs owner canonical-head CAS; AuthorAppAgent (Texture) is the sole agent document-text writer. Settlement tombstones are lifecycle/CAS store writes under owner authority — not Texture revisions, not a third writer, never Super-consumption."
    - "Pre-A checkpoint 99949fe2 untouched"
    - "Effects remain OFF; no live mail/trading/mutations"
    - "Live wake paths: ApplyTextureTurn -> wakeTextureControl -> WakeUpdatedCoagent; ensureSelfDevelopmentTextureJoin / startSelfDevelopmentPersistentSuper; update_coagent delivery -> ReconcileCoagentWake for non-boot entry; CoSuper-cancel Texture rewake (cosuper_assignment_fate.go); maybeRewakeSelfDevelopmentTextureAfterTerminalSuper as a Texture-agent wake (never a direct Super mint); reactivateRetryableLifecycleInjectionRuns (Researcher path)"
    - "Wake vs selection distinction: deleting backlog-selection WAKES must not delete FIFO DRAINING of pending backlog under live triggers"
    - "I26 FIFO applies when a live trigger selects; it does not authorize boot or terminal scans as scheduler ticks. Later arrival alone never cancels an active request; only deadline expiry or owner correction may"
    - "Super singleton: exactly one persistent Super identity per ComputerID, at most one active Super run"
  excluded:
    - "RLM session interpreter code (owned by Def 2)"
    - "Candidate A authoring and solitaire implementation (owned by Def 3)"
    - "World Wire redesign, prompt caching, or naming cutover"
    - "Cold-recover B14 image rebuilding (residual only if required for live computer recovery)"
    - "Researcher and generic-actor boot reconciliation (recorded as unpaid scope boundary; only Super/CoSuper minting is constrained)"
  protected_surfaces:
    - "internal/agentcore/super_controller.go"
    - "internal/agentcore/runtime.go"
    - "internal/agentcore/selfdev_texture_join.go"
    - "internal/agentcore/cosuper_assignment_fate.go"
    - "internal/store/lifecycle.go"
    - "internal/vmctl/handlers.go"
  completion_evidence_floor:
    - deployed_proof

conjectures:
  - id: live-trigger-only-wake-suffices
    conjecture: "Restricting Super wakes to live triggers (newly committed Texture execution_request, delivered control packet) plus structurally isolated exact in-flight resume eliminates assignment supersession and storm cycling without priority or preemption machinery."
    falsifier: "After the repair, a Super run is minted from undelivered backlog — controls, mailbox, or reports — at boot, refresh, or a terminal event."
  - id: tombstone-settlement-is-terminal
    conjecture: "Store-layer CAS settlement with terminal disposition fully retires the nine 08-19 cancel producer reports: every pending selector structurally excludes them."
    falsifier: "Any pending store selector (ListPendingLifecycleUpdates, ListAllPendingLifecycleUpdates, or successor) returns a settled report ID, or any run metadata references one after settlement."
  - id: fifo-scheduler-suffices
    conjecture: "A durable computer-scoped arrival ordinal plus FIFO among non-expired requests orders competing live-triggered work without ping-pong."
    falsifier: "A live-triggered selection executes out of ordinal order or cancels an executing assignment (deadline expiry and owner correction excepted)."
  - id: texture-rewake-survives-narrowing
    conjecture: "Narrowing terminal continuation to delivered-controls-only does not degrade the proven Texture rewake path, because rewake wakes the Texture agent (which commits a NEW execution_request) rather than minting Super from backlog."
    falsifier: "After a terminal Super with an in-flight self-development operation, neither a Texture instruction nor (after the owner Texture turn) a new Super appears."

heresies:
  discovered:
    - "Boot-as-scheduler: reconcile selects pending backlog as a Super wake source at boot and after terminal events (named 2026-09-03, owner); the resume carve-out is aspirational, not structurally isolated (consensus R1)."
    - "Competing pending Texture execution requests ping-pong fresh assignments, cancelling every CoSuper before author/build/freeze (discovered 2026-08-21)."
    - "vmctl normal boots hung 3/3 while hold-param boots succeeded 4/4 due to unindexed body scans (discovered 2026-08-25)."
  introduced: []
  repaired:
    - "Store-level ArrivalOrdinal and super_controller fail-closed deadlines implemented."
    - "Storm-stop claim machinery 3654d925 and replacement prompt 5e01ac3a shipped; superseded as end-state by store-layer settlement under the corrected ontology."

measures:
  - name: supersession_count
    kind: gate
    baseline: "repeated ping-pong cancellations observed 2026-08-21"
    desired: "0"
    decision_use: "certifies live-trigger FIFO scheduling contract"
    cannot_prove: "cannot prove RLM interpreter readiness"
  - name: boot_minted_run_count
    kind: gate
    baseline: "boot/refresh historically minted replacement Supers from backlog (2026-08-28: six); 2026-09-03 probes showed zero only because claim-suppression held, not because scheduling was removed"
    desired: "0 across all probe boots, measured under the admissible-unclaimed-backlog precondition of acceptance criterion 2"
    decision_use: "certifies boot-is-recovery ontology"
    cannot_prove: "cannot prove live-path correctness"
  - name: normal_boot_success_rate
    kind: gate
    baseline: "5/5 achieved 2026-09-03 (partial evidence)"
    desired: "5/5 consecutive normal boots reaching health 200 within 60s with full landing receipts"
    decision_use: "certifies substrate unblocked for Def 2 sealed proof"
    cannot_prove: "cannot prove guest code correctness"

now:
  status: working
  slice: "Wake-path ontological repair: structural isolation, live-trigger FIFO proof, store-layer settlement"
  question: none
  reconciliation:
    observed_at: "2026-09-03T02:20:00Z"
    source_ref: "main@cb7804f9"
    deploy_identity: "staging https://choir.news behavior-bearing commit 42d47604; computer-03335285269bdba4f94377e56879f9e6 active epoch 844"
    authority_identities:
      - "docs/evidence/effects-red-boot-as-scheduler-ontological-error-2026-09-03.md"
      - "docs/evidence/effects-red-substrate-boot-probe-partial-evidence-2026-09-03.md"
      - "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "git status --short; receipts and revision uncommitted pending adjudication commit"
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
    selected: "Correct the Super wake ontology: live-trigger-only wakes from an exhaustive allowlist (newly committed Texture execution_request, delivered control packet); boot schedules nothing and resumes only the exact interrupted run through a structurally isolated entry point; terminal events wake Texture only; the nine 08-19 cancel producer reports are settled by store-layer CAS lifecycle reducer with the claim-scan retired; prove FIFO selection across live cycles on staging."
    kind: architecture
    status: settled
    source: owner
    evidence_ref: "docs/evidence/effects-red-boot-as-scheduler-ontological-error-2026-09-03.md"
    owner_ratification_ref: "owner direction 2026-09-03 (boot is recovery, not scheduler); consensus adjudication 2026-09-03 (10/10 returned, unanimous REVISE-then-execute with direction approved)"
    recorded_at: "2026-09-03T02:20:00Z"
    consequence: "5e01ac3a replacement-continuation direction is superseded as an end-state; the full backlog-selection ladder (lifecycle controls first, then Texture-rewake fallthrough, pending-instruction resume, mailbox settle, producer reports) is deleted as a wake source; exact-run resume is structurally isolated; Definition 1 acceptance now comprises six criteria including a terminal-event probe."
  evidence_refs:
    - "docs/evidence/effects-red-boot-as-scheduler-ontological-error-2026-09-03.md"
    - "docs/evidence/effects-red-substrate-boot-probe-partial-evidence-2026-09-03.md"
    - "docs/evidence/effects-red-super-texture-rewake-2026-08-20.md"
    - "docs/evidence/effects-red-assignment-supersession-loop-2026-08-21.md"
    - "docs/reports/choir-held-computer-boot-outage-postmortem-2026-08-28.md"
  blocker_or_risk: "Test suite currently enforces the wrong ontology (backlog -> new Super as success case); contract flip across lifecycle_control_injection_test.go and survivor/continuation tests is the bulk of the repair. Panel medium-confidence flag: staging ordinals 2/3 admissibility as Super controls is unconfirmed and gates criterion 1."
  next_action: "Implement the wake-path repair per the revised problem-receipt repair direction (structural isolation first), then run the six acceptance criteria in order against staging."

receipts:
  - id: substrate-scheduling-baseline-2026-09-02
    boundary: define
    commit_or_artifact: "main@a52ef06d"
    proof_refs:
      - "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
      - "docs/definitions/choir-scheduling-and-candidate-proof-2026-08-21.md"
    rollback_ref: "checkpoint 99949fe2 remains the immutable pre-A fence"
    disposition: "accepted as Definition 1 establishing strict substrate-first serialization; supersedes substrate scope of choir-scheduling-and-candidate-proof-2026-08-21.md"
    problem_ref: assignment-supersession-loop-2026-08-21
    authorization_ref: "owner direction 2026-09-02; agentic consensus unanimous Option B"
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "docs/ACTIVE.md; docs/mission-graph.yaml; docs/doc-authority-manifest.yaml"
  - id: substrate-scheduling-ontology-revision-2026-09-03
    boundary: define
    commit_or_artifact: "this revision"
    proof_refs:
      - "docs/evidence/effects-red-boot-as-scheduler-ontological-error-2026-09-03.md"
      - "docs/evidence/effects-red-substrate-boot-probe-partial-evidence-2026-09-03.md"
      - ".agentic-consensus/agentic-consensus-20260902-212526/manifest.tsv"
    rollback_ref: "checkpoint 99949fe2 remains the immutable pre-A fence"
    disposition: "accepted; revises Definition 1 to live-trigger-only wakes, structurally isolated exact-run resume, terminal-wakes-Texture-only, and store-layer settlement; incorporates all convergent consensus repairs (R1-R8); supersedes the 00:55Z 2026-09-03 completion claim and the 5e01ac3a replacement-from-backlog end-state"
    problem_ref: boot-as-scheduler-ontological-error-2026-09-03
    authorization_ref: "owner direction 2026-09-03; agentic consensus 2026-09-03 (10 models returned, 1 provider-failed, unanimous REVISE with ontology direction approved; all required repairs applied)"
    candidate_or_evidence_refs:
      - "docs/evidence/effects-red-boot-as-scheduler-ontological-error-2026-09-03.md"
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "docs/ACTIVE.md; docs/mission-graph.yaml; docs/doc-authority-manifest.yaml"
---

# Definition 1: Substrate & Scheduling Readiness

This Definition executes Phase 1 of the 3-Definition autonomous engineering sequence,
revised 2026-09-03 to the boot-is-recovery ontology after a 10-model consensus review.

## Ontology (normative for this Definition)

- **Steady state is always-on.** While the guest process is up, work arrives via
  live triggers only. The exhaustive allowlist of Super wake triggers:
  1. a newly committed Texture `execution_request` (owner Texture turn ->
     `ApplyTextureTurn` -> `wakeTextureControl`);
  2. a delivered control packet bound to the current run (`update_coagent`
     delivery -> `WakeUpdatedCoagent` -> `ReconcileCoagentWake`);
  3. exact in-flight recovery of an interrupted run (structural carve-out below).
  Everything else is forbidden as a wake: boot, refresh, startup reconciliation,
  terminal completion alone, health polling, and backlog discovery are NOT
  triggers. (`choir computer refresh` is a product-path call but is never a wake.)
- **Boot is a recovery event, never a scheduler tick.** On boot the computer
  restores state deterministically. The ONLY boot Super entry is a dedicated
  exact-run resume entry point for runs passivated with `runtime_restarted`
  (or injection-append failure only when it represents the same already-admitted
  run consuming only controls previously delivered to that run); it returns
  unconditionally without reaching any selection call. Pending backlog is
  durable and waits; the next live trigger drains it in FIFO order.
  Lower tiers that boot-warm execute nothing.
- **Terminal events wake Texture, never select backlog.** A Super terminal may
  wake the bound Texture operation (`maybeRewakeSelfDevelopmentTextureAfterTerminalSuper`);
  the Texture turn then commits a NEW `execution_request`, and that new live
  trigger wakes Super. A terminal event itself must never select an
  already-pending control or report.
- **Stale residue is settled, never consumed.** Superseded or orphaned producer
  reports are tombstoned by a store-layer lifecycle reducer with CAS
  precondition, terminal disposition, and idempotent receipt; every pending
  selector structurally excludes them.
- **I26 scoping**: FIFO among non-expired requests governs live-trigger
  selection. It does not authorize boot or terminal scans as scheduler ticks.
  Later arrival alone never cancels an active request; only deadline expiry or
  owner correction may.

## Core Deliverables
1. **Live-Trigger FIFO Scheduling Contract**: Competing execution requests are
   selected strictly by computer-scoped `ArrivalOrdinal` under the named live
   triggers. Executing assignments are protected from supersession cancellations.
2. **Boot-Does-Not-Schedule (structurally enforced)**: The full backlog-selection
   ladder — `listPendingPersistentSuperLifecycleControls`,
   `maybeRewakeSelfDevelopmentTextureAfterTerminalSuper` (as Super mint),
   `resumeSelfDevelopmentSuperForPendingInstruction`,
   `listAndSettlePersistentSuperBacklog`,
   `listPendingPersistentSuperAdmissibleReports`, the boot work-item sweep, and
   terminal `maybeContinuePersistentSuperInbox` on undelivered backlog — is
   deleted as a wake source. A dedicated exact-run resume entry point is added
   and the boot rewarm calls it, not full reconcile.
3. **Producer Report Settlement**: The nine undelivered cancel producer reports
   from the 08-19 storm are settled at the store layer (CAS, terminal
   disposition, idempotent receipts); `claimedPersistentSuperProducerReportIDs`
   metadata claim-scan is retired. No replacement Super is minted from them.
4. **vmctl Normal-Boot Stability**: Product-path normal boots reach `/health`
   200 within deadline without relying on `RUNTIME_MAINTENANCE_HOLD`.

Candidate A and RLM session interpreter implementations are strictly excluded
from this Definition.
