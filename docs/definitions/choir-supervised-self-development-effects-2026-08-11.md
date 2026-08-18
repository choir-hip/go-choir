---
definition_version: 2
definition_id: choir-supervised-self-development-effects-2026-08-11
execution_mode: mission_orchestrator

start:
  captured_at: 2026-08-11T18:40:00Z
  source:
    canonical_ref: main@6ff6b7d0
    deploy_identity: "staging https://choir.news reconciled 2026-08-11: frontend 914f7a5d976a, proxy 914f7a5d976a, proxy status ok, deploy time 2026-08-11T18:11:01Z (Settings > Runtime status, verified live); supersedes the prior stale capture 26c53692494aed1a2ea337550990d70c7cd16735 via CI 31445846546"
  worktree_inventory:
    status: reconciled
    evidence_ref: 2026-08-11 read-only git status; clean single worktree /Users/wiz/go-choir
    preservation_rule: Preserve every non-primary worktree and all unrelated WIP; this Definition owns only itself, its review evidence, and the three registry entries at supersession.
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      paths_or_digest: [docs/definitions/choir-supervised-self-development-effects-2026-08-11.md, docs/definitions/choir-supervised-self-development-effects-2026-08-11-supplement.md, docs/problems/irreversible-effects-human-gate-drift-2026-08-13.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
      recovery: revert the docs-only commit
  candidates:
    - id: none
  observed_artifact:
    - claim: "Checkpoints bind no state. ComputerVersion is CodeRef plus ArtifactProgramRef only; the Dolt state extractor is read-only and ProjectionMaterializer is non-runtime; rollback re-applies a prior release while rows written under it remain. Three stores call DOLT_COMMIT; nothing in production calls checkout, branch, or reset, and those commits carry no event-head binding."
      evidence_ref: internal/computerversion/types.go:40-48; internal/computerversion/dolt_state_extractor.go:27-83; internal/computerversion/projection_materializer.go:9-22; internal/agentcore/self_development_materializer.go:224-273; internal/platform/store.go:481-499; internal/store/store.go:119; internal/cycle/storage.go:56
    - claim: "The pre-consensus substrate enforces per-candidate owner approval: the decision binding requires an external-owner: authority prefix, and accept_once requires exact operation/bundle/heads/pending-transition/commitments plus a future canonical UTC expiry. This is fail-closed implementation residue, not target ontology; replacement requires a policy-bound multiagent consensus receipt and reducer, not deletion of the gate."
      evidence_ref: internal/agentcore/self_development_decision_binding.go:30-33,45,53; internal/platform/self_development_modes.go:226-242
    - claim: "The frozen CapsuleEffectBundle refuses any bundle lacking SourceTreeRef, a capsule-exec BuildRecipeRef, RuntimeArtifactRef, TestReceipts, and DependencyToolchainRefs, so every required ref must bind a real capsule execution receipt."
      evidence_ref: internal/capsule/transaction/builder.go:95-111
    - claim: "Freeze/propose/verify tools have no production call site; an assigned CoSuper holds exactly five capsule-local tools and no upward channel. CoSuper lacks update_coagent, pinned by TestSurvivorContract_GenericCoSuperCannotAuthorPersistentSuperPackets, because Super reads executability from the model-written packet.kind."
      evidence_ref: internal/agentcore/tool_profiles.go:309-315,386; internal/agentcore/tools_capsule.go:61-102; internal/agentcore/super_controller.go:784-796; internal/agentcore/update_coagent_survivor_contract_test.go:193-199
    - claim: "The capsule can build: lower layer is the guest root, platform source is snapshotted by git commit, and go/gcc/git/make/nodejs/pkg-config/icu are on PATH with persistent Go caches and Dolt ICU CGO flags set."
      evidence_ref: nix/autoputer-vm.nix:675-700,717-745,777-798; internal/autoputer/capsule_executor_linux.go:14-56
    - claim: "Staging serves the web frontend from the host (Caddy on Node B, /var/www/go-choir/frontend-current), outside the updater-controlled release. This candidate is API-only because the browser would not read a capsule UI; computer-surface frontend remains per-computer by C15."
      evidence_ref: nix/node-b.nix:23-24,161,193-207
    - claim: "Supersession cannot be an operation state — the selfdev machine is linear and terminal — and the decision verifier admits exactly one input artifact ref and one verifier ref, so a supersession citation must ride B's proposal event."
      evidence_ref: internal/selfdev/operations.go:423-446; internal/agentcore/self_development_decision_binding.go:45,53
    - claim: "Supervision already flows upward (capsule -> CoSuper -> update_coagent -> Super -> Texture), and Texture revisions already carry a metadata blob plus typed source citations separate from prose. No observation subsystem is required."
      evidence_ref: internal/agentcore/tools_worker_update.go:176; internal/textureowner/texture.go:2303; internal/textureowner/texture_revision_metadata.go:20-60; internal/textureowner/texture_evidence_sources.go
    - claim: "There is no migration framework; tables are created with CREATE TABLE IF NOT EXISTS at startup, so a code-only rollback leaves rows behind."
      evidence_ref: internal/platform/computer_events.go:18,39,63
    - claim: "Owner key issuance is disposed: one-click bootstrap mints an owner-wide admin key, with headless create and revoke proven on staging."
      evidence_ref: 367265f8; 6ff6b7d0; .agentic-consensus/key-model-2026-08-11-convergent/
  problems_documented:
    - id: rollback-is-code-only-2026-08-11
      problem: "The computer cannot be returned to a point in its history: checkpoints bind no state, Dolt history is written but never read back, and rollback reinstalls a prior binary while its rows remain."
      evidence_ref: see supplement section 3
      consequence: "Reversibility is the proof target; building and verifying revert is in scope."
    - id: approval-was-the-wrong-safety-property-2026-08-11
      problem: "Per-candidate owner approval is encoded in the decision binding and accept_once. It was correctly identified as the wrong universal gate, but the first repair then incorrectly inferred that reversibility defines the autonomy boundary and irreversible effects require a separate human decision."
      evidence_ref: "see supplement section 2; docs/problems/irreversible-effects-human-gate-drift-2026-08-13.md"
      consequence: "Effect-specific multiagent consensus replaces universal per-candidate owner approval across reversible and irreversible effects. Reversibility changes recovery and policy strength; a human is a policy-selected seat, not a universal gate."
    - id: actor-isolation-stopgap-2026-08-11
      problem: "CoSuper has no upward actor channel because Super derives executability from the model-written packet.kind, so holding update_coagent would let a worker open privileged execution on its supervisor."
      evidence_ref: see supplement section 5
      consequence: "CoSuper regains update_coagent; executability moves to sender authorization; the pinned survivor contract is replaced by a stronger assertion, never deleted."
    - id: model-policy-retired-as-content-axis-2026-08-11
      problem: "The original content axis (computer-scoped model policy) had no path to its target and is a system-owned surface already scheduled for replacement."
      evidence_ref: see supplement section 1
      consequence: "Content is source code. Model policy is excluded."
    - id: replay-completeness-non-equivalence-2026-08-12
      problem: "The deployed pre-drop replay completeness probe reconstructed the retained event chain into a disposable projection but returned not_equivalent: 26 deterministic DoltStateExtractor differences, with both live_head and replay_head null. Five schema observations and five table observations were absent from replay; sixteen schema/content observations differed."
      evidence_ref: docs/evidence/choir-sandbox-autoputer-replay-completeness-2026-08-12.json
      consequence: "Classified 2026-08-12 (1 aggregate content_root, 5 retired tables as schema+table, 1 event_projection schema drift via live-only supervision_transaction_json, 14 empty_until_supported direct-SQL tables), then resolved 2026-08-12 by owner-scoped product-path workspace replacement: POST /api/computers/{id}/lifecycle/replace-workspace quarantined the retained workspace and opened current DDL (no event, no checkpoint), and a restart reopened the store. The post-cutover probe reports equivalent with 82 live and 82 replay observations and zero differences; heads remain null, so replay eligibility, checkpoint, restore, and effects stay fail-closed. See docs/evidence/choir-supervised-self-development-replay-difference-classes-2026-08-12.md and docs/evidence/choir-supervised-self-development-replay-completeness-post-cutover-2026-08-12.json."
    - id: replay-non-equivalent-direct-write-authority-2026-08-18
      problem: "The replay-completeness result is not equivalent because four nonempty behavior-bearing tables still have live direct-write authorities outside the event projection: run memory, self-development operations/start intents, and Texture agent mutations. An existing graph-backed run-memory adapter is not wired into serving, and the other three families have no reducer-backed replacement or replay read path."
      evidence_ref: docs/evidence/effects-red-computer-surface-boot-2026-08-18.md
      consequence: "Eligibility must remain false. A repair must route every production writer through the existing event-projection batch seam and import the retained residue as a co-moving state-as-of-now event before replay can authorize a checkpoint or retry."
    - id: replay-probe-credential-renewal-refused-2026-08-12
      problem: "After the replay gate repair deployed at d2ab2d2d, the owner-scoped product CLI reaches the retained active computer but fails during event-chain reconstruction: the guest capability renewal endpoint returns a non-201 response and the guest reports renewal refused. The required read-only replay diff is not recaptured; no state mutation or clean rematerialization is authorized."
      evidence_ref: "CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=10m go run ./cmd/choir computer replay-completeness --computer computer-03335285269bdba4f94377e56879f9e6 at 2026-08-12T18:46Z returned HTTP 500: replay completeness: reconstruct event chain: computer event appender: fetch durable chain: computer event client: capability: guest credential: renewal refused; /health reported deployed_commit d2ab2d2d2184a7918f1a6ff73b6bd29638f85b5c; computer status reported the requested stable computer active at realization_epoch 203."
      consequence: "An owner-scoped lifecycle restart at 2026-08-12T18:47:50Z refreshed the retained guest credential through the product path without source or state repair. The replay probe then completed; recurrence across capability expiry remains a residual risk and is not masked."
    - id: chain-bootstrap-null-head-2026-08-12
      problem: "After workspace replacement made the retained computer diagnostically equivalent (82 live == 82 replay, zero differences), it still cannot reach replay eligibility because the canonical event chain is empty: Store.Head is null and replayEligibility requires both event heads non-nil and equal. The only production genesis route (POST /self-development/genesis) is checkpoint-coupled and proxy-disabled, and the Definition forbids reusing it."
      evidence_ref: docs/evidence/choir-supervised-self-development-chain-bootstrap-design-2026-08-12.md
      consequence: "Resolved 2026-08-13 by POST /api/computers/{id}/lifecycle/bootstrap-chain on the retained computer: one EventGenesisImported bound to the guest release identity (git 5d132f66360899d634befd69ba537ab918005c24, guest-image sha256:f777ceea59d6628838076fdeb70719653fd61822c1fce5e0e319cb2af14911c7, empty EmbeddedDoltRefs) established a sequence-1 canonical head with no checkpoint and no selfdev Operation. The post-bootstrap probe reports equivalent (82==82, zero differences) with live_head==replay_head and eligible=true. See docs/evidence/choir-supervised-self-development-replay-completeness-post-bootstrap-2026-08-13.json."

  unknowns:
    - "Whether checkpoint creation binds the sequence-1 canonical head plus a VM-local content witness and refuses while any behavior-bearing local row is not event- or receipt-derivable; the retained computer is now eligible and is the checkpoint-design substrate."
    - "Whether rematerialization is product-ready. ProjectionMaterializer is non-runtime today; if the classification and subsequent build show it is not usable, restore falls back to a single-workspace pin checkout on an interim basis with the event head still the sole semantic authority."
    - "Which concrete decision-policy schema, independence domains, quorum, dissent, abstention, timeout, recusal, replacement, and consequence-receipt contracts satisfy the first reversible and irreversible acceptance cases without hard-coding one universal panel."
    - "Whether the upward coagent packet payload can carry operation id, bundle digest, receipt id, and head into Texture revision metadata and citations without a payload schema change."
    - "Whether the CTS-observed registry gap (Texture production registry omitting update_coagent) is still present on the deployed staging build."
    - "Whether guest capability renewal refusal recurs after the five-minute capability lifetime, and which server-side refusal class caused the 2026-08-12 incident; the current acceptance proves only that an owner-scoped lifecycle restart restored a fresh capability."
    - "Retained computer epoch 8253 disposition; ak_45ce1796 row and root-only auth rollback cleanup."
  unknowns_correction:
    recorded_at: 2026-08-15T23:23:00Z
    preserves: "start.unknowns as captured 2026-08-11/13 observation; do not treat that list as the live backlog"
    consumed_by: choir-tape-recovery-2026-08-13
    consumed:
      - "checkpoint binds event head + CodeRef + ArtifactProgramRef + VM-local Dolt content witness + frontend identity and fails closed on live-only rows"
      - "rematerialization is a deployed tape-only product path; pin checkout is not a completion route"
      - "guest capability renewal succeeds across restore without a subsequent start (epoch 268, store_closed=false)"
      - "serving_join: two computers plus unsigned platform shell serve three distinct index.html hashes"
    still_open:
      - "concrete decision-policy schema, independence domains, quorum, dissent, abstention, timeout, recusal, replacement, and consequence-receipt contracts"
      - "whether the upward coagent packet payload can carry operation id, bundle digest, receipt id, and head into Texture revision metadata without a payload schema change — answered 2026-08-16: yes, via existing packet.sources typed URIs (operation:/capsule_bundle:/receipt:/event_head:) with no payload schema change; identities persist in revision metadata and typed citations, never prose"
      - "whether Texture production registry still omits update_coagent on the deployed staging build — source confirmation 2026-08-16: current main omits generic update_coagent on Texture (AllowCoAgentTools=false). Staging health 2026-08-16T02:05Z is 4543624b (product-path forward). CTS-safe: do not register the generic resolver. Deployed Texture-registry re-check of 466c0504 remains unpaid"
      - "whether in-process rehearsal can walk reversible propose→consensus→promote→restore and irreversible propose→consensus→outbox without a live send — answered 2026-08-16 orange: yes, TestEffectsRehearsal. Live 2026-08-16: propose_only generation 1 at epoch 272; pre-A checkpoint 409. Residue import product path is proxied and not executed live. Super not started."
      - "retained computer epoch 8253 / ak_45ce1796 classified 2026-08-15 as historical CTS residual, not current identity (paid restore epoch 268; key returns 401). Residual hygiene only; do not reopen tape-recovery"

finish:
  deliver: "The computer works autonomously under effect-specific multiagent consensus on top of the tape-recovery restore substrate owned by choir-tape-recovery-2026-08-13. A CoSuper capsule authors, builds, tests, freezes, and proposes its own source change (solitaire: headless play API, durable persistence, play history); a policy-selected qualified panel approves the exact subject with no per-candidate human decision; it promotes, runs, and writes real state, then reverts through that restore substrate. The correction spine runs: defective A promotes, admissible evidence falsifies it, B supersedes A, and restart proves B. A separate stronger irreversible policy, selected before outputs and requiring no human seat for this acceptance, authorizes one exact external send to an owner-controlled acceptance inbox; delivery and a follow-up correction are durably receipted. Texture revises an owner-readable document throughout."
  artifact: "An authenticated staging computer trajectory carrying: versioned reversible and irreversible decision policies; frozen eligible-seat and independence manifests; proposals, objections, abstentions, dissents, adjudication, and consensus receipts bound to exact subjects and heads; a frozen bundle A with real SourceTreeRef, capsule-exec BuildRecipeRef, DependencyToolchainRefs, TestReceipts, RuntimeArtifactRef, and independent verifier receipt; a policy-authorized promotion, checkpoint, and route transition; solitaire rows, falsification evidence, superseding B, restart proof, and exact acceptance-fenced restore to a pre-A checkpoint; plus an exact external-send proposal to an owner-controlled acceptance inbox, a no-human-seat qualified consensus decision, trusted outbox/provider delivery receipt, and follow-up correction receipt. Texture revisions cite every consequential transition while keeping machine identity in metadata and typed citations."
  acceptance:
    - action: "Restore substrate preconditions are satisfied-by choir-tape-recovery-2026-08-13: replay completeness/rematerialization, checkpoint witness, revert build and verification, and scope refusal. This Definition consumes that substrate for its promotion-revert proof and does not independently green any restore leg."
      proves: "Whole-computer restore is single-writer; this Definition builds effect promotion on top of it rather than re-proving it."
      evidence_class: deployed proof via the tape-recovery receipts
    - action: "Policy-governed reversible promotion: a CoSuper capsule authors, builds, tests, freezes, and proposes; the decision policy freezes eligible seats and independence domains before outputs, the qualified panel decides on the exact bundle digest and heads, and the candidate promotes with no per-candidate human decision. Prove refusal when policy is absent, expired, revoked, stale, missing a required seat, below quorum, or carrying unresolved policy-blocking dissent."
      proves: "The computer can authorize reversible self-development through policy-evaluated multiagent consensus without turning a human into the universal gate or allowing the panel to shrink itself."
      evidence_class: deployed proof
    - action: "Capsule authorship proof: every required bundle ref binds a real execution receipt from the authoring capsule."
      proves: "The candidate is authored by the computer, not hand-authored and pushed through the pipeline."
      evidence_class: deployed proof
    - action: "Live E2 inside the envelope: A promotes -> solitaire is played through the headless API and writes rows -> evidence falsifies A -> B supersedes A -> restart proves B effective."
      proves: "The correction spine runs autonomously and survives restart."
      evidence_class: deployed proof
    - action: "Total revert of the reversible excursion using the tape-recovery restore substrate: return to a checkpoint taken before A promoted. Show the solitaire API absent, the solitaire rows absent, state hashes equal to the recorded checkpoint, and the event chain still carrying the full history of what was undone."
      proves: "Reversible computer-local state has a complete acceptance-fenced recovery path; this is an additional safety property, not the boundary of autonomy."
      evidence_class: deployed proof
    - action: "Irreversible consensus and execution: propose one exact external email to an owner-controlled acceptance inbox. Show the reversible-effect policy refuses it. Select a stronger irreversible-effect policy before participant outputs, bind recipient and payload exactly, obtain its qualified independent consensus with no unresolved safety dissent and no human seat required for this acceptance, execute through the trusted outbox, and retain provider delivery plus consequence receipts. Then show a different policy that explicitly requires a human seat refuses when that seat is absent."
      proves: "Irreversible effects remain inside the autonomy window; stronger policy and durable accountability replace a categorical human gate, while a policy may still choose to require a human."
      evidence_class: deployed proof
    - action: "Supervision legibility: at each consequential transition a Texture revision is readable through the public API and CLI, its prose human and its metadata and typed citations carrying operation, bundle digest, receipt, and head. At least one revision is committed while capsule work is still open."
      proves: "The owner can watch and intervene during autonomous work rather than reading a post-hoc report."
      evidence_class: deployed proof
    - action: "Replay the accepted trajectory from canonical events."
      proves: "Single semantic authority reconstruction; no second state authority was introduced."
      evidence_class: deployed proof

  rollback: "Revoke the active effect policies and capabilities; total restore to a prior checkpoint uses the choir-tape-recovery-2026-08-13 substrate (VM-local projection plus computer-surface frontend; platform and cycle stores out of scope); revert behavior commits through origin/main and CI to the last accepted runtime. Direct deployed-node edits are neither proof nor rollback. Irreversible consequences such as the acceptance email are not recoverable by restore: preserve their receipts and use compensation or a new forward action."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  completion_cutover:
    required: true
    purpose: "Staging trajectory proof alone is a false complete if haunted teaching remains live authority. These obligations are part of goal.complete, not post-hoc cleanup."
    upon_deployed_acceptance:
      - id: doctrine-promote
        action: "Promote earned invariants into choir-doctrine / computer-ontology / agent-product-doctrine: effect-specific consensus policy is the autonomy boundary; checkpoint = event head + CodeRef + ArtifactProgramRef + VM-local content witness + frontend identity; restore is acceptance-fenced and scoped; irreversible effects require stronger policy and consequence receipts, not a categorical human gate; effects OFF is pre-gate not destination. Platform/cycle remain OUT of restore. Computer-surface frontend is IN by C15/I25; the host-global SPA non-conformance is closed by choir-tape-recovery-2026-08-13 serving_join (2026-08-15), not by treating UI as platform software."
        class: promote-to-doctrine
      - id: lexicon-cutover
        action: "Cut product and API vocabulary from rollback/accept_once/approval-as-safety to restore/decision_policy/qualified_consensus/consequence_receipt where those names teach the old world (selfdev states, CLI, Settings copy, Definition skill templates)."
        class: rewrite-in-place
      - id: roadmap-registry-archive
        action: "Confirm choir-self-development-roadmap is historical-only in registries; stamp SUCCESS on this Definition; ensure CTS cannot be read as an entrypoint; remove live authority edges that still schedule D1/Mission-0/accept_once rehearsal."
        class: delete-live-authority
      - id: survivor-detector-replace
        action: "Replace (do not merely green) survivor/detector pins of CoSuper-without-update_coagent and packet.kind executability; add detectors requiring every reversible and irreversible effect to bind its declared policy, subject, eligible seats, independence domains, quorum, dissent disposition, expiry, and consequence receipts; reject both universal-human-gate and ungoverned-effect paths; reject treating ComputerVersion code tuple alone as full restore."
        class: test/detector update
      - id: owner-product-surface
        action: "Ship or open a successor Definition for owner-reachable policy arm/revoke, consensus and consequence receipts via CLI/API/Settings. Owner-reachable restore invoke, checkpoint list, and whole-computer restore CLI are owned by choir-tape-recovery-2026-08-13 and satisfied-by its owner_reachable_whole_computer_restore receipt."
        class: successor-mission
      - id: ops-identity
        action: "Land restore-aware health/identity reporting (event head + CodeRef + witness + route; distinguish product restore from failed deploy / git revert). Retain release artifacts needed for advertised checkpoints."
        class: successor-mission
      - id: frontend-ownership
        action: "Decided 2026-08-13: computer-surface frontend is per-computer, not platform control-plane (docs/memo-per-computer-frontend-2026-08-13.md; doctrine C15/I25). The serving envelope is owned by choir-tape-recovery-2026-08-13 and satisfied-by its serving-join receipt. This Definition does not ship UI in the solitaire candidate; tape-recovery serving_join is paid (guest-static hop after vmctl resolve). Thin platform shell (TLS, auth, picker chrome) remains OUT of restore."
        class: promote-to-doctrine
      - id: successor-preconditions
        action: "Rewrite RLM / World Wire / in-choir drafts to inherit the envelope (not effects-OFF Phase 1; WW requires irreversible-decision path)."
        class: rewrite-in-place
      - id: residual-realism
        action: "Name residual risks in the terminal receipt: schema-as-event vs additive-only; backup/copy boundaries; tape retention vs erasure; owner verbs (arm/revoke/restore/irreversible decide)."
        class: residual-risk
  not_done_when:
    - "Revert is code-only: the release pointer moved and VM-local state did not."
    - A revert touched the shared platform store or cycle state, rewinding service state or another computer.
    - A checkpoint was created while behavior-bearing local rows were not event- or receipt-derivable, promising a restore that cannot be performed.
    - A revert was reported as successful without re-extracting state and comparing it to the hashes recorded at the target checkpoint.
    - Checkpoints do not bind a target event head and content witness, so no point in history is addressable and "arbitrary rollback" is aspirational.
    - The VM-local Dolt HEAD was treated as restore authority rather than as an audit receipt joined to the event head.
    - An effect executed without a predeclared, active policy bound to its exact subject, eligible seats, independence domains, quorum, abstention/timeout/recusal/replacement rules, dissent disposition, and expiry.
    - A policy-required seat was quietly removed after seeing outputs, independence was fabricated, quorum shrank on failure, or policy-blocking dissent remained unresolved.
    - An irreversible effect executed under a reversible-effect policy, without a durable consequence receipt, or was categorically refused solely because no human approved it.
    - "Revert erased history: the event chain no longer shows the excursion that was undone."
    - The candidate diff was hand-authored while the report implies capsule authorship.
    - A's defect was caught by A's own verification (that proves the gate, not the correction spine).
    - CoSuper still lacks update_coagent, or regained it by weakening the privilege property rather than relocating authority to the sender.
    - Texture exposes only a final report after the work settles, or no revision was committed while capsule work was still open.
    - Texture prose recites machine identifiers at the owner; identity belongs in metadata, citations, and at the Super level.
    - Texture revision metadata or citations omit the operation, bundle digest, receipt, or head, leaving prose that cannot be joined to what it claims.
    - Only E1 evidence exists (single promotion without correction supersession) at registry close.
    - CTS remains active in all three registries without a successor pointer.
    - Deployed trajectory proof is claimed complete while completion_cutover obligations above remain undone (haunted doctrine/roadmap/tests/CLI still teach the pre-envelope world).
    - "The retained pre-drop replay result is treated as a clean match, ignored as legacy noise, or used to authorize restore without classifying all 26 differences."

    - Product restore exists only as automation against staging with no owner-reachable arm/revoke/restore/refusal surface scheduled or shipped.
    - Ops still treat staging SHA alone as computer identity after the first successful product restore.

boundaries:
  mutation_class: red
  authority_sources: [owner-ratified decisions, docs/choir-vision.md, docs/choir-doctrine.md, docs/computer-ontology.md, docs/agent-product-doctrine.md, docs/standing-questions.md, AGENTS.md]
  must_preserve:
    - Canonical computer event chain is the single semantic state authority.
    - Effects stay OFF until the staging rehearsal and decision-policy gates pass; afterward each effect class is authorized only through its active policy and audited actuator.
    - Restore never erases history. Returning to a prior point is a forward transaction that restores reversible state; the event chain retains the full record of what was done and undone. Irreversible consequences remain on the tape and are corrected by compensation or new forward action.
    - Reversibility changes recovery cost and required safeguards, not the boundary of autonomy. Both reversible and irreversible effects require predeclared, effect-specific multiagent consensus; a human is a policy-selected seat, never a universal gate.
    - Checkpoints bind the target event head, code, artifact program, and a VM-local content witness. A checkpoint that cannot restore state is not a checkpoint, and one that cannot be rebuilt from the tape must fail closed rather than be recorded.
    - The event chain is the address and the authority. VM-local Dolt is a projection and an audit witness, never an alternate head.
    - "Restore is scoped to the user computer: event-derived VM-local projection and computer-surface frontend (owned by choir-tape-recovery-2026-08-13). The shared platform store and cycle state are out of scope, because restoring them would rewind other computers."
    - "Restore is acceptance-fenced, not a distributed transaction: stage, verify against recorded content hashes, and flip visibility only on exact match. Partial never greens."
    - The candidate is authored inside a CoSuper capsule; every required bundle ref binds a real execution receipt from that capsule.
    - CoSuper holds update_coagent and reports upward as an actor, while a CoSuper packet can never open privileged execution on Super. Executability derives from the sender's authorization, not from the model-written packet.kind. The pinned survivor contract is replaced by a stronger test, never deleted.
    - CoSuper freezes and proposes its own candidate. Proposal is worker authority, not a supervisory gate.
    - "Decision authority is policy-relative: every effect binds an exact subject, policy version, eligible-seat and independence manifest, quorum, dissent and failure rules, expiry, evidence, and consequence-receipt contract. Participants cannot redefine the policy after seeing outputs."
    - "The owner establishes and can revoke the constitutional policy envelope. Qualified multiagent consensus authorizes instances within it. Human participation is required only when the selected policy says so; revocation never requires agent cooperation."
    - Irreversible effects require stronger policy, narrower subject binding, larger or more diverse independent consensus where declared, durable provider and consequence receipts, and compensation or new forward action when correction is needed.
    - The historical self-development roadmap is migration evidence only — not live schedule authority; execute this Definition, not Mission 0–5 from that table.
    - "Reconnection stays minimal on purpose: restore the channel and hold the privilege property. Decision-policy and consensus work is built only to satisfy the exact reversible and irreversible acceptance cases; the RLM/Yaegi successor may rewrite actor activation without weakening these authority contracts."
    - A's defect is pre-declared in a Define receipt before A is proposed, so the falsification cannot later be reported as discovered.
    - "Self-development schema changes remain additive (CREATE TABLE IF NOT EXISTS, never in-place ALTER/DROP as a migration framework) because there is no migration framework and rollback does not reverse schema. One owner-authorized pre-launch exception: a product-path VM-local workspace replacement onto current DDL may discard retired schema residue and unsupported direct-SQL rows. That replacement is a cutover, not a migration, appends no event, publishes no checkpoint, and does not license restore or effects."
    - "The solitaire candidate itself touches no protected surface: no auth/session, no event/decision path, no provider routing, and no Texture write from solitaire code. This is distinct from the mission's Texture writes, which are required — Texture supervises the self-development through its own owner and atomic reducer, and the candidate is its subject, never its author."
    - Texture's canonical write path, single-owner contract, and atomic revision transition are preserved exactly; observing self-development adds an evidence source to Texture, never a second writer or a second document authority.
    - A Texture revision is evidence and legibility, never acceptance authority; reading a revision does not accept an effect, and no Texture write may append, accept, materialize, checkpoint, or route.
    - No headless/CDP/virtual-authenticator substitute for exact owner presence; no SSH-shaped operations; product path only.
    - "Problem-documentation-first: a discovered platform problem is documented in a code-free Define receipt before any repair-code commit."
  excluded:
    - RLM/Yaegi actor authoring (successor capability mission; it inherits the deployed effect-policy envelope and does not gain ambient effect authority).
    - Model-policy content as a self-development effect (see problems_documented; the surface is system-owned and slated for replacement).
    - Production environment.
    - "CTS's full downward-control choreography — Texture-originated Researcher follow-ups, Texture-to-persistent-Super execution requests, parallel multi-CoSuper assignment, and the generic transcluded-source contract. What is NOT excluded, and is now required, is CTS's supervision core: Texture revising a canonical owner-readable document at each transition while work remains open. Finishing CTS still cannot deliver the vision proof, but supervision was never the severable part."
    - In-choir computer-control draft (separate successor authority).
  protected_surfaces: [self-development mode CAS, canonical computer event chain, materializer + decision binding, checkpoint/route projection, Texture canonical writes, updater root, vmctl lifecycle, auth/session renewal, gateway/provider calls, deployment routing]
  completion_evidence_floor: [deployed proof, independent review of frozen bundle + decision binding]

measures:
  - name: rehearsal pass
    kind: gate
    baseline: 0
    desired: "propose -> auto-approve -> promote -> write state -> total revert -> verified hash match, PASS on staging before the live run"
    decision_use: unlocks the live run; a rehearsal that skips revert does not count
    cannot_prove: the correction spine, or that restore holds for a large state excursion
  - name: capsule-bound bundle refs
    kind: gate
    baseline: 0
    desired: all five required bundle refs (source tree, build recipe, toolchain, tests, runtime artifact) bound to receipts from the authoring capsule
    decision_use: distinguishes capsule authorship from hand authorship
    cannot_prove: that the authored capability is correct
  - name: supervised transition coverage
    kind: gate
    baseline: 0
    desired: every consequential transition has an owner-readable Texture revision whose metadata and citations carry the exact operation, digest, receipt, and head while its prose stays human; at least one committed while work was still open
    decision_use: gates the supervision claim; an uncovered transition is an unsupervised transition
    cannot_prove: that the owner's decision was correct
  - name: tape segment count
    kind: weak_signal
    baseline: 0
    desired: ">= 8 (mode receipt, accept A, materialize A, falsify, supersede B, restart proof, revoke, each with its Texture revision)"
    decision_use: inspect the next transition; never advances complete alone
    cannot_prove: acceptance
  - name: consensus agreement
    kind: weak_signal
    baseline: roadmap convergent panel unanimous on sequence shape (3/3 verdicts)
    desired: independent review acceptance of frozen E2 candidate
    decision_use: schedule review; never settles authority
    cannot_prove: the product works

now:
  status: working
  slice: "Reconciled 2026-08-18 after the route-local replay deadline repair. Source commit 34c68283 extends only the owner-only replay response deadline and gives Node B a bounded 10-minute upstream budget; staging remains ae806003 and the retained computer remains active at epoch 308 until the repair lands. Replay eligibility remains false and no candidate or bundle exists. Effects remain OFF; no live mail."
  question: "Does the decision-policy envelope authorize reversible and irreversible effects on top of a proven whole-computer restore substrate?"

  reconciliation:
    observed_at: 2026-08-18T14:50:03Z
    source_ref: "main 34c68283 (route-local repair, not yet deployed); prior staging CI 32148760924/ae806003; owner refresh receipt 01a01551-9aa3-7ef6-aaee-bde3cfa10a46; retained epoch 308; guest ae806003; prior owner-authorized replay-completeness read returned HTTP 502 replay completeness authority unavailable after 120.54 seconds; internal/proxy/self_development.go; internal/proxy/config.go; nix/node-b.nix"
    deploy_identity: "Local source commit 34c68283 extends only the replay response writer deadline through ResponseController and sets the Node B replay client budget to 10m; ordinary proxy routes retain the 120s global server write deadline. Staging `/health` and the retained guest still report ae8060035195be3d862cac325244491e85befb7f until the repair landing loop completes. No fresh replay result, equivalent projection, or eligibility result is established. The computer remains propose_only generation 1; operation selfdev-b090bcd72d300fed17cb3f5a142f8595 remains executing with no candidate or bundle; constructed freeze 7122f279 is not promotion authority."
    authority_identities: [docs/choir-vision.md, docs/choir-doctrine.md, docs/standing-questions.md, docs/computer-ontology.md, docs/agent-product-doctrine.md, docs/memo-per-computer-frontend-2026-08-13.md, AGENTS.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: 2026-08-18 docs-first outer-deadline failure receipt plus route-local source repair; preserved untracked DSH rehearsal docs remain outside this mission
    status: reconciled

  candidate:
    id: none
    state: none
  decision:
    selected: "Effect-specific multiagent consensus is the autonomy boundary; reversibility is a recovery property. Restore of an effect excursion consumes the choir-tape-recovery-2026-08-13 substrate (event-derived VM-local projection plus computer-surface frontend; platform/cycle OUT) and is addressed by event head and acceptance-fenced. This Definition does not independently green checkpoint, rematerialization, scope refusal, or total restore. Checkpoints bind event head, CodeRef, ArtifactProgramRef, VM-local content witness, and frontend identity; VM-local Dolt HEAD is an audit receipt. CoSuper authors and proposes. A policy-selected qualified panel authorizes each exact effect subject under frozen eligibility, independence, quorum, dissent, expiry, and consequence rules; human participation is optional unless that policy requires it. Reversible and irreversible effects are both admissible, with stronger policy and durable consequence receipts for irreversible effects. The mission proves this with reversible self-development plus restore and one exact no-human-seat email send to an owner-controlled acceptance inbox."
    kind: architecture
    status: owner-ratified-correction
    source: owner
    evidence_ref: "docs/problems/irreversible-effects-human-gate-drift-2026-08-13.md; docs/choir-vision.md; docs/choir-doctrine.md"
    owner_ratification_ref: "owner correction 2026-08-13: irreversible effects are not outside the autonomy window; effect-specific multiagent consensus is the governing boundary, and human approval is optional as one possible consensus participant."
    recorded_at: 2026-08-13T14:18:16Z
    consequence: "The 2026-08-11 inference that reversibility substitutes for approval is superseded. Replace owner-armed standing-rule plus fixed Super/Texture pair and irreversible refusal with policy-bound qualified consensus across both reversible and irreversible effects. Preserve fail-closed current gates until their policy-based replacement passes deployed acceptance."
  evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, docs/evidence/effects-red-gateway-inference-eof-2026-08-18.md, docs/evidence/effects-red-capability-ttl-and-background-renewal-2026-08-18.md, docs/evidence/effects-red-capsule-toolchain-and-overlay-write-2026-08-18.md, docs/evidence/effects-red-super-coagent-cancellation-delivery-2026-08-18.md, docs/evidence/effects-red-cosuper-completed-run-cancel-2026-08-17.md, docs/evidence/effects-red-cosuper-capsule-tools-2026-08-17.md, docs/evidence/effects-red-texture-caller-reactivate-2026-08-17.md, docs/evidence/effects-red-retry-cluster-2026-08-17.md, docs/evidence/effects-red-guest-credential-renewal-2026-08-17.md, docs/evidence/effects-red-cosuper-terminal-2026-08-17.md, docs/evidence/effects-red-cosuper-nonconvergence-2026-08-17.md, docs/evidence/effects-red-cosuper-run-state-index-2026-08-17.md, docs/evidence/effects-red-cosuper-boot-repair-still-running-2026-08-17.md, docs/evidence/effects-red-cosuper-boot-miss-2026-08-17.md, docs/evidence/effects-red-cosuper-refresh-still-running-2026-08-17.md, docs/evidence/effects-red-cosuper-activation-budget-2026-08-17.md, docs/evidence/effects-red-super-cosuper-spawn-2026-08-17.md, docs/evidence/effects-red-super-broker-readiness-2026-08-17.md, docs/evidence/effects-red-super-capsule-spawn-timeout-2026-08-17.md, docs/evidence/effects-red-super-capsule-hosts-2026-08-16.md, docs/evidence/effects-red-super-texture-rewake-2026-08-16.md, docs/evidence/effects-red-super-inject-after-tools-2026-08-16.md, docs/evidence/effects-red-super-assign-channel-2026-08-16.md, docs/evidence/effects-red-super-texture-control-join-2026-08-16.md, docs/evidence/effects-red-super-texture-join-2026-08-16.md, docs/evidence/effects-residue-import-proxy-2026-08-16.md, docs/evidence/effects-residue-import-command-2026-08-16.md, docs/evidence/effects-residue-import-2026-08-16.md, docs/evidence/effects-live-desktop-og-tape-2026-08-16.md, docs/evidence/effects-sql-only-project-2026-08-16.md, docs/evidence/effects-unified-tape-contracts-2026-08-16.md, docs/evidence/effects-recovery-domain-complete-from-head-2026-08-16.md, docs/choir-unified-event-tape-design-2026-08-16.md, docs/evidence/effects-unified-event-tape-design-review-2026-08-16.md, docs/choir-effects-pre-a-checkpoint-eligibility-2026-08-16.md, docs/evidence/effects-pre-a-checkpoint-eligibility-review-2026-08-16.md, docs/evidence/effects-red-pre-a-checkpoint-ineligible-2026-08-16.md, docs/evidence/effects-red-named-mode-2026-08-16.md, docs/evidence/effects-red-verifier-refresh-2026-08-16.md, docs/evidence/effects-guest-verifier-2026-08-16.md, docs/evidence/effects-red-kernel-route-live-2026-08-16.md, docs/evidence/effects-guest-kernel-route-2026-08-16.md, docs/evidence/effects-red-mode-off-refuse-2026-08-16.md, docs/evidence/effects-owner-guest-boot-refresh-2026-08-16.md, docs/evidence/effects-guest-mode-authority-2026-08-16.md, docs/evidence/effects-red-product-path-smoke-2026-08-16.md, docs/evidence/effects-product-path-forward-2026-08-16.md, docs/evidence/effects-rehearsal-2026-08-16.md, docs/evidence/effects-supervision-wiring-2026-08-16.md, docs/evidence/effects-trusted-outbox-2026-08-16.md, docs/evidence/effects-irreversible-email-v1-policy-review-2026-08-16.md, docs/evidence/effects-irreversible-email-v1-policy-2026-08-16.md, docs/evidence/effects-decision-policy-reducer-2026-08-16.md, docs/evidence/effects-freeze-propose-wiring-2026-08-16.md, docs/evidence/effects-reconnection-2026-08-16.md, docs/evidence/effects-reversible-selfdev-v1-policy-review-2026-08-15.md, docs/evidence/effects-reversible-selfdev-v1-policy-2026-08-15.md, docs/evidence/effects-decision-policy-schema-repair-review-2026-08-15.md, docs/evidence/effects-decision-policy-schema-review-2026-08-15.md, docs/evidence/effects-decision-policy-schema-2026-08-15.md, docs/evidence/effects-invoke-readiness-2026-08-15.md, docs/definitions/choir-tape-recovery-2026-08-13.md, docs/choir-self-development-roadmap-2026-08-11.md, docs/choir-crashed-prime-session-review-2026-08-09.md, docs/memo-persistent-rlm-actors-2026-08-09.md, docs/memo-live-retrospective-evals-2026-08-09.md]
  blocker_or_risk: "The payload-specific 64 MiB repair and the Node B-only 119-second route repair are deployed; source commit 34c68283 adds a route-local deadline and 10-minute Node B client budget but is not deployed. The retained guest remains at ae806003 epoch 308 and replay eligibility is unproven after the prior 120.54-second HTTP 502. Replay equivalence, checkpointability, restore substrate, and self-development retry remain forbidden. Existing risks remain: forging Texture sender authorization; adding generic update_coagent to Texture; CoSuper opening Super; self-promote; live effect."
  next_action: "Land 34c68283 through the normal red landing loop, verify staging identity, owner-refresh the same computer onto that guest, and run a fresh owner-authorized replay-completeness read. Do not blind-retry, remove response bounds, SQL-empty tables, bind a checkpoint, rematerialize, restore, retry self-development, self-promote, CAS qualified_consensus, or send mail while eligibility is false. Keep OwnerRecovery checkpoint 663540be out of promotion, treat 7122f279 as a freeze only, keep genesis 409, keep Armed=false, preserve the epoch-308 serving proof, and do not declare new_epoch."

receipts:
  - id: effects-red-gateway-inference-eof-2026-08-18
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-gateway-inference-eof-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-gateway-inference-eof-2026-08-18.md, .agentic-consensus/effects-retry-investigation-20260817]
    rollback_ref: this docs-only problem receipt; no runtime rollback
    disposition: "accepted as problem documentation — gateway-only 56c01c72 deploy passed, but fresh CoSuper assignment-ee19e577 reached 42 tool iterations and terminated on provider inference EOF; no candidate or bundle; do not blind-retry"
    problem_ref: not_applicable
    authorization_ref: this Definition now after 2026-08-18 live reorientation
    candidate_or_evidence_refs: [docs/evidence/effects-red-gateway-inference-eof-2026-08-18.md]
    landing:
      source_commit: pending (docs-only reorientation)
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: staging https://choir.news gateway activation receipt 56c01c72; retained epoch 301; mode propose_only generation 1; no active CoSuper; operation selfdev-b090bcd7 executing
      deployed_acceptance: not_applicable — this receipt documents a failed product attempt, not acceptance
    registry_conformance_ref: "effects remains entrypoint; this problem receipt is not a freeze, promotion authority, or permission to retry"
  - id: effects-red-gateway-write-timeout-repair-2026-08-18
    boundary: implement
    commit_or_artifact: "nix/node-b.nix; internal/server/server_test.go"
    proof_refs: [docs/evidence/effects-red-gateway-inference-eof-2026-08-18.md, internal/server/server_test.go]
    rollback_ref: revert eb67848c740fbf3e3e8ef21bf2d78de7dedd9010
    disposition: "accepted as deployed repair — gateway SERVER_WRITE_TIMEOUT=10m30s matches the 10m inference/provider budgets; focused timeout wiring test and Nix parse passed locally; CI and Node B deploy passed. The following owner-refresh exposed the independent computer-surface serving-join problem."
    problem_ref: effects-red-gateway-inference-eof-2026-08-18
    authorization_ref: Definition next_action after the docs-first timeout mismatch receipt
    candidate_or_evidence_refs: [docs/evidence/effects-red-gateway-inference-eof-2026-08-18.md]
    landing:
      source_commit: eb67848c740fbf3e3e8ef21bf2d78de7dedd9010
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/32103816632
      deploy_ref: https://github.com/choir-hip/go-choir/actions/runs/32103816632 (Deploy to Staging (Node B) job 95612326588)
      environment_identity: staging gateway and guest report eb67848c740fbf3e3e8ef21bf2d78de7dedd9010; retained computer epoch 302; mode propose_only generation 1
      deployed_acceptance: not_applicable — gateway repair identity is verified, but the subsequent named computer-surface probe returned 503 and is documented by effects-red-computer-surface-boot-2026-08-18
    registry_conformance_ref: "effects remains entrypoint; timeout repair is not freeze or promotion authority"
  - id: effects-red-computer-surface-boot-repair-2026-08-18
    boundary: implement
    commit_or_artifact: a70c4f727f4687a47954526adbddfde7eb27c8c2
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, internal/agentcore/selfdev_surface_boot_test.go, internal/agentcore/rematerialize.go]
    rollback_ref: revert a70c4f727f4687a47954526adbddfde7eb27c8c2
    disposition: "accepted as deployed product-path repair after the docs-first problem receipt 6fb20c4e — boot calls the existing trusted route-bound baseline import before actor start; route or baseline failure remains fail-closed; no event, checkpoint, operation, mode, or effect mutation. Focused package tests, CI, and Node B host deployment passed. Authorized refresh receipt 01a013c5-c111-7c6f-b0c4-6e6973a71bb4 advanced epoch 302 to 303; guest observability reports 13a0ae7c and the authenticated named surface returns HTML 200, not the platform shell or underivable error. Replay completeness is a separate unresolved 500 EOF receipt and does not become accepted by this product-path proof."
    problem_ref: effects-red-computer-surface-boot-2026-08-18
    authorization_ref: Definition next_action after the docs-first computer-surface receipt
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: 13a0ae7cebc7081753d0a93b92310b00ff41a6d0
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/32108952503
      deploy_ref: https://github.com/choir-hip/go-choir/actions/runs/32108952503 (Deploy to Staging (Node B) job 95627946051)
      environment_identity: staging host services and installed autoputer package 13a0ae7c; retained computer guest 13a0ae7c at active epoch 303; mode propose_only generation 1; refresh receipt 01a013c5
      deployed_acceptance: accepted for the computer-surface boot serving join; replay-completeness HTTP 500 EOF remains a separate blocker and no self-development retry
    registry_conformance_ref: "effects remains entrypoint; boot serving-join repair is not freeze or promotion authority"
  - id: effects-red-replay-completeness-eof-2026-08-18
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-computer-surface-boot-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, 01a013c5-c111-7c6f-b0c4-6e6973a71bb4]
    rollback_ref: this docs-only problem receipt; no runtime rollback
    disposition: "accepted as new problem documentation — after owner refresh advanced the retained computer to epoch 303 and proved guest 13a0ae7 plus authenticated surface 200, replay-completeness repeatedly returned HTTP 500 on durable-chain decode unexpected EOF. No checkpoint, restore, rematerialization, candidate, bundle, or self-development retry is authorized by this receipt."
    problem_ref: not_applicable
    authorization_ref: Definition next_action after the boot serving-join acceptance
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: docs-only post-refresh observation
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: retained computer active epoch 303, guest/autoputer 13a0ae7c, mode propose_only generation 1; replay-completeness 500 unexpected EOF
      deployed_acceptance: not_applicable — this receipt blocks restore/self-development readiness; it does not revoke the accepted boot serving-join proof
    registry_conformance_ref: "effects remains entrypoint; replay EOF is a problem receipt, not a freeze, promotion authority, or permission to retry"
  - id: effects-red-replay-completeness-eof-diagnosis-2026-08-18
    boundary: implement
    commit_or_artifact: docs/evidence/effects-red-computer-surface-boot-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, internal/computerevent/http_client.go, internal/platform/event_replay.go, internal/platform/event_handlers.go]
    rollback_ref: this docs-only diagnosis receipt; no runtime rollback
    disposition: "accepted as source-confirmed diagnosis — the guest HTTP client caps the unpaged corpusd durable-chain response at 1 MiB, and a local valid oversized response reproduces the exact unexpected EOF. A cap increase alone is rejected; repair must page by sequence and fail closed on truncation or non-progress. No checkpoint, restore, candidate, bundle, self-development retry, or effect is authorized."
    problem_ref: effects-red-replay-completeness-eof-2026-08-18
    authorization_ref: Definition next_action after the replay EOF problem receipt
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: docs-only diagnosis receipt
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: retained computer active epoch 303, guest/autoputer 13a0ae7c, mode propose_only generation 1; replay-completeness 500 unexpected EOF
      deployed_acceptance: not_applicable — this diagnosis narrows the repair boundary and does not claim replay eligibility
    registry_conformance_ref: "effects remains entrypoint; replay EOF diagnosis is not a freeze, promotion authority, or permission to retry"
  - id: effects-red-replay-completeness-eof-repair-2026-08-18
    boundary: implement
    commit_or_artifact: c38324f4
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, internal/computerevent/http_client_test.go, internal/platform/event_replay_test.go]
    rollback_ref: revert c38324f4
    disposition: "source repair committed after the diagnosis receipt — replay is now bounded and sequence-progressing: corpusd pages with LIMIT, the guest follows pages, and overlong or non-progressing pages fail closed. Focused local tests pass. Staging identity remains 13a0ae7c; no deployed acceptance, checkpoint, restore, candidate, bundle, self-development retry, or effect is claimed until the landing loop completes."
    problem_ref: effects-red-replay-completeness-eof-2026-08-18
    authorization_ref: Definition next_action after the docs-first replay diagnosis
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: c38324f4
      ci_ref: pending
      deploy_ref: pending
      environment_identity: staging retained computer epoch 303, guest/autoputer 13a0ae7c, mode propose_only generation 1; repair not deployed
      deployed_acceptance: pending — requires staging identity verification and a fresh replay-completeness read
    registry_conformance_ref: "effects remains entrypoint; source replay repair is not freeze, promotion authority, or permission to retry"
  - id: effects-red-replay-completeness-repair-landed-2026-08-18
    boundary: implement
    commit_or_artifact: bdbf7b7eea30bc425e95145040cd9ca55d0a473e
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, https://github.com/choir-hip/go-choir/actions/runs/32114070641]
    rollback_ref: revert c38324f4
    disposition: "accepted as a deployed transport repair only — CI run 32114070641 attempt 2 and Node B deployment passed; staging /health reports bdbf7b7e. Fresh owner-authorized replay reads no longer fail with unexpected EOF and instead reach projection repair required. Replay eligibility, checkpointability, restore, candidate, bundle, self-development retry, and effects remain unaccepted and forbidden."
    problem_ref: effects-red-replay-completeness-eof-2026-08-18
    authorization_ref: Definition next_action after the replay EOF diagnosis and source repair
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: bdbf7b7eea30bc425e95145040cd9ca55d0a473e
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/32114070641 (attempt 2)
      deploy_ref: https://github.com/choir-hip/go-choir/actions/runs/32114070641
      environment_identity: staging proxy bdbf7b7e; retained computer epoch 303; guest/autoputer 13a0ae7c; mode propose_only generation 1
      deployed_acceptance: transport repair exercised; replay-completeness remains HTTP 500 projection repair required
    registry_conformance_ref: "effects remains entrypoint; transport repair is not replay eligibility, freeze, promotion authority, or permission to retry"
  - id: effects-red-replay-completeness-projection-repair-2026-08-18
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-computer-surface-boot-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, internal/computerevent/appender.go, internal/agentcore/replay_completeness.go]
    rollback_ref: this docs-only problem receipt; no product-state rollback
    disposition: "accepted as new problem documentation — after the paged replay repair landed, two owner-authorized replay-completeness reads returned HTTP 500 projection repair required. The prior transport EOF is repaired, but canonical platform head versus replay projection convergence is unproven. Do not rematerialize, restore, bind a checkpoint, retry self-development, or send mail until the mismatch is diagnosed and repaired through a separate problem-first landing loop."
    problem_ref: effects-red-replay-completeness-eof-2026-08-18
    authorization_ref: Definition next_action after the deployed replay transport observation
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: docs-only problem documentation
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: staging proxy bdbf7b7e; retained computer epoch 303; guest/autoputer 13a0ae7c; mode propose_only generation 1; replay-completeness HTTP 500 projection repair required
      deployed_acceptance: not_applicable — this receipt records the blocker and does not authorize a retry
    registry_conformance_ref: "effects remains entrypoint; projection-repair receipt is not a freeze, promotion authority, or permission to retry"
  - id: effects-red-replay-completeness-client-server-skew-2026-08-18
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-computer-surface-boot-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, internal/computerevent/http_client.go, internal/platform/event_handlers.go, internal/platform/event_replay.go]
    rollback_ref: this docs-only diagnosis; no product-state rollback
    disposition: "accepted as source-confirmed problem documentation — the deployed host defaults replay responses to pages of 32, while the retained guest 13a0ae7c still sends the pre-pagination request and treats one response as complete. The resulting version skew explains the projection-repair sentinel when the chain exceeds one page; exact chain/page evidence remains unpaid. The next probe is owner refresh of the same computer, not rematerialization or a retry."
    problem_ref: effects-red-replay-completeness-projection-repair-2026-08-18
    authorization_ref: Definition next_action after source inspection of the changed replay failure
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: docs-only diagnosis
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: staging corpusd bdbf7b7e; retained computer guest/autoputer 13a0ae7c at epoch 303; mode propose_only generation 1
      deployed_acceptance: not_applicable — this receipt documents client/server skew and authorizes only the same-computer refresh probe
    registry_conformance_ref: "effects remains entrypoint; contract-skew diagnosis is not a freeze, promotion authority, or permission to retry"
  - id: effects-red-replay-completeness-proxy-timeout-2026-08-18
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-computer-surface-boot-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, internal/proxy/handlers.go, internal/proxy/self_development.go]
    rollback_ref: this docs-only problem receipt; no product-state rollback
    disposition: "accepted as new problem documentation — after owner refresh installed guest/autoputer bdbf7b7e at epoch 304, the named surface remained HTTP 200 but two replay-completeness reads returned HTTP 502 after approximately 31 seconds. Source confirms the replay proxy client has a fixed 30-second timeout. Do not globally raise or mask timeouts; diagnose and repair only the replay route budget before any replay or effect retry."
    problem_ref: effects-red-replay-completeness-client-server-skew-2026-08-18
    authorization_ref: Definition next_action after the same-computer refresh probe
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: docs-only problem documentation
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: staging proxy/corpusd bdbf7b7e; retained computer guest/autoputer bdbf7b7e at active epoch 304; mode propose_only generation 1; replay-completeness HTTP 502 after 30 seconds
      deployed_acceptance: not_applicable — this receipt blocks replay readiness and authorizes only timeout diagnosis/repair
    registry_conformance_ref: "effects remains entrypoint; proxy-timeout receipt is not a freeze, promotion authority, or permission to retry"
  - id: effects-red-replay-completeness-proxy-timeout-repair-2026-08-18
    boundary: implement
    commit_or_artifact: cf950de7
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, internal/proxy/config.go, internal/proxy/handlers.go, internal/proxy/self_development.go, internal/proxy/handlers_test.go, internal/proxy/self_development_test.go, nix/node-b.nix]
    rollback_ref: revert cf950de7
    disposition: "accepted as locally verified source repair — replay completeness now uses a dedicated PROXY_REPLAY_COMPLETENESS_TIMEOUT budget (default and Node B value 110s) while ordinary autoputer routes remain at 30s; timeout remains fail-closed as HTTP 502. Focused and full internal/proxy tests pass. Staging deployment, retained-guest refresh, and replay eligibility remain unpaid."
    problem_ref: effects-red-replay-completeness-proxy-timeout-2026-08-18
    authorization_ref: Definition next_action after the docs-first timeout diagnosis
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: cf950de7
      ci_ref: pending
      deploy_ref: pending
      environment_identity: local source/test only; prior staging proxy/guest bdbf7b7e at retained epoch 304
      deployed_acceptance: pending — this repair does not authorize replay, checkpoint, restore, self-development, or effects until the landing loop and owner-authorized staging read complete
    registry_conformance_ref: "effects remains entrypoint; source timeout repair is not replay acceptance, freeze, promotion authority, or permission to retry"
  - id: effects-red-replay-completeness-non-equivalent-2026-08-18
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-computer-surface-boot-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, internal/computerevent, internal/platform, internal/proxy, nix/node-b.nix]
    rollback_ref: this docs-only problem receipt; no product-state rollback
    disposition: "accepted as new problem documentation — after timeout repair commit cf950de7 landed in CI 32118922263 and the same retained computer refreshed to epoch 305, one owner-authorized replay-completeness read returned HTTP 200 but result not_equivalent and eligibility false. Four behavior-bearing direct-write tables are non-empty without reducers. This receipt authorizes diagnosis only; do not SQL-empty, checkpoint, rematerialize, restore, retry self-development, or send mail."
    problem_ref: effects-red-replay-completeness-non-equivalent-2026-08-18
    authorization_ref: Definition next_action after the deployed timeout-repair read
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: 7976ec15
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/32118922263
      deploy_ref: staging Node B deploy in CI 32118922263
      environment_identity: staging proxy/autoputer 7976ec15; retained computer epoch 305; mode propose_only generation 1
      deployed_acceptance: not_applicable — replay endpoint completion proves the timeout repair, but semantic equivalence and eligibility remain false
    registry_conformance_ref: "effects remains entrypoint; non-equivalence is a problem receipt, not checkpoint, restore, promotion authority, or permission to retry"
  - id: effects-red-replay-source-diagnosis-2026-08-18
    boundary: define
    commit_or_artifact: docs/evidence/effects-red-computer-surface-boot-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, internal/store/run_memory.go, internal/selfdev/operations.go, internal/store/texture.go, internal/store/projection_tape.go, internal/computerevent/projection_batch.go, internal/store/project.go, docs/choir-unified-event-tape-design-2026-08-16.md]
    rollback_ref: this docs-only diagnosis receipt; no product-state rollback
    disposition: "accepted as docs-first source diagnosis — the four unsupported tables have live direct-write authorities absent from replay. The existing run-memory object-graph adapter is not wired into serving; self-development operations/start intents and Texture agent mutations have no reducer-backed projection/read replacement. No runtime repair, eligibility change, checkpoint, residue import, restore, retry, or effect authorization occurred."
    problem_ref: replay-non-equivalent-direct-write-authority-2026-08-18
    authorization_ref: Definition next_action after the owner-authorized non-equivalent replay read
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: pending (docs-first diagnosis checkpoint)
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: source diagnosis against staging replay evidence; retained computer epoch 305 remains propose_only generation 1
      deployed_acceptance: not_applicable — diagnosis is not a runtime repair or replay acceptance
    registry_conformance_ref: "effects remains entrypoint; this diagnosis is not checkpoint, restore, promotion authority, or permission to retry"
  - id: effects-red-replay-reducer-repair-2026-08-18
    boundary: implement
    commit_or_artifact: docs/evidence/effects-red-replay-reducer-repair-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-replay-reducer-repair-2026-08-18.md, docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    rollback_ref: revert the forthcoming source repair commit; no retained-computer mutation before owner-authorized residue import
    disposition: "accepted as a docs-first repair checkpoint — the approved reducer-backed replacement is authorized after the source diagnosis, but source implementation, deployment, residue import, replay equivalence, and eligibility remain pending. Effects remain OFF and all checkpoint, restore, retry, self-promote, qualified-consensus, and send paths remain forbidden."
    problem_ref: replay-non-equivalent-direct-write-authority-2026-08-18
    authorization_ref: Definition next_action after effects-red-replay-source-diagnosis-2026-08-18
    candidate_or_evidence_refs: [docs/evidence/effects-red-replay-reducer-repair-2026-08-18.md]
    landing:
      source_commit: pending (docs-first repair checkpoint)
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: retained computer epoch 305 remains propose_only generation 1; no residue import performed
      deployed_acceptance: not_applicable — checkpoint authorizes source work and is not runtime or replay acceptance
    registry_conformance_ref: "effects remains entrypoint; reducer repair is not replay eligibility, checkpoint, restore, promotion authority, or permission to retry"
  - id: effects-red-replay-payload-response-bound-2026-08-18
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-computer-surface-boot-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, internal/computerevent/http_client.go, https://github.com/choir-hip/go-choir/actions/runs/32139497055]
    rollback_ref: this docs-only problem receipt; no product-state rollback
    disposition: "accepted as new problem documentation — after c6d0b34a deployed and the retained computer was owner-refreshed from epoch 305 to 306, replay no longer hit the stale guest's unexpected EOF but failed closed because the 8 MiB payload response bound was exceeded at sequence 3218. The prior one-mebibyte truncation path is repaired; replay eligibility, checkpointability, restore, self-development retry, and effects remain unaccepted and forbidden."
    problem_ref: replay-payload-response-bound-2026-08-18
    authorization_ref: Definition next_action after the c6301b07 residue import payload EOF observation
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: c6d0b34a79f63e4ca4350b8a8b9aa9fe9363e66f
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/32139497055
      deploy_ref: https://github.com/choir-hip/go-choir/actions/runs/32139497055
      environment_identity: staging proxy c6d0b34a; retained computer epoch 306 after receipt 01a01529-baa3-7b42-89df-5b53632ffa0e; guest refreshed onto c6d0b34a; mode propose_only generation 1
      deployed_acceptance: not_applicable — the refreshed read proves the explicit bound is active and fail-closed, but the payload envelope exceeds it and replay eligibility remains false
    registry_conformance_ref: "effects remains entrypoint; payload-bound problem is not checkpoint, restore, promotion authority, or permission to retry"
  - id: effects-red-replay-payload-response-bound-repair-2026-08-18
    boundary: implement
    commit_or_artifact: 8754884d
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, internal/computerevent/event.go, internal/computerevent/http_client.go, internal/computerevent/http_client_test.go]
    rollback_ref: revert 8754884d
    disposition: "accepted as locally verified source repair after the payload-bound problem receipt — FetchPayload now uses a separate 64 MiB finite response bound while replay pages remain at 8 MiB; focused and full computerevent tests pass. Staging deployment, retained-guest refresh, replay eligibility, checkpointability, restore, self-development retry, and effects remain pending and forbidden."
    problem_ref: replay-payload-response-bound-2026-08-18
    authorization_ref: Definition next_action after effects-red-replay-payload-response-bound-2026-08-18
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: 8754884d
      ci_ref: pending
      deploy_ref: pending
      environment_identity: staging c6d0b34a; retained computer epoch 306; guest c6d0b34a; source repair not deployed
      deployed_acceptance: pending — local bounded transport proof only; requires staging identity, same-computer refresh, and fresh replay-completeness result
    registry_conformance_ref: "effects remains entrypoint; payload transport repair is not replay eligibility, checkpoint, restore, promotion authority, or permission to retry"
  - id: effects-red-replay-payload-route-timeout-2026-08-18
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-computer-surface-boot-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, https://github.com/choir-hip/go-choir/actions/runs/32146221851, 01a01541-586e-71f3-b4f8-050ad5899f65]
    rollback_ref: this docs-only problem receipt; no product-state rollback
    disposition: "accepted as new problem documentation — after the 64 MiB payload repair deployed and the retained computer refreshed to epoch 307, replay passed the prior payload bound but the owner route returned HTTP 502 after approximately 112 seconds because the dedicated 110-second replay budget expired. Replay eligibility, checkpointability, restore, self-development retry, and effects remain unaccepted and forbidden."
    problem_ref: replay-payload-route-timeout-2026-08-18
    authorization_ref: Definition next_action after effects-red-replay-payload-response-bound-repair-2026-08-18
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: 7ae01fa151e9a6a34c3ef1395026724762f4c8f4
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/32146221851
      deploy_ref: https://github.com/choir-hip/go-choir/actions/runs/32146221851
      environment_identity: staging 7ae01fa1; retained computer epoch 307 after receipt 01a01541-586e-71f3-b4f8-050ad5899f65; guest refreshed onto 7ae01fa1; mode propose_only generation 1
      deployed_acceptance: not_applicable — the route reaches past the payload bound but expires before replay completeness, so eligibility remains false
    registry_conformance_ref: "effects remains entrypoint; replay-route timeout is not checkpoint, restore, promotion authority, or permission to retry"
  - id: effects-red-replay-payload-route-timeout-repair-2026-08-18
    boundary: implement
    commit_or_artifact: f86d6ed7
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, nix/node-b.nix, internal/proxy/config.go, internal/proxy/handlers.go]
    rollback_ref: revert f86d6ed7
    disposition: "accepted as locally verified source/config repair after the replay-route timeout problem receipt — Node B now uses a 119-second replay-only client budget below the 120-second proxy write deadline; ordinary routes remain at 30s, focused proxy tests and Nix parse pass. Staging deployment, retained-guest refresh, replay eligibility, checkpointability, restore, self-development retry, and effects remain pending and forbidden."
    problem_ref: replay-payload-route-timeout-2026-08-18
    authorization_ref: Definition next_action after effects-red-replay-payload-route-timeout-2026-08-18
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: f86d6ed7
      ci_ref: pending
      deploy_ref: pending
      environment_identity: staging 7ae01fa1; retained computer epoch 307; guest 7ae01fa1; Node B route repair not deployed
      deployed_acceptance: pending — local bounded route proof only; requires staging identity, same-computer refresh, and fresh replay-completeness result
    registry_conformance_ref: "effects remains entrypoint; replay route-timeout repair is not replay eligibility, checkpoint, restore, promotion authority, or permission to retry"
  - id: effects-red-replay-route-outer-deadline-2026-08-18
    boundary: diagnose
    commit_or_artifact: docs/evidence/effects-red-computer-surface-boot-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, nix/node-b.nix, internal/proxy/self_development.go, internal/server/server.go]
    rollback_ref: revert f86d6ed7 if the prior route configuration must be restored; no product-state rollback
    disposition: "accepted as a new source-repair blocker after deployed f86d6ed7: the same computer refresh to epoch 308 and owner-authorized replay read still returned HTTP 502 after 120.54s. The 119s client budget and 120s outer server deadline are insufficient; the next repair must extend only this route's response deadline and upstream client budget rather than globally widening proxy timeouts. Effects remain OFF."
    problem_ref: replay-route-outer-deadline-2026-08-18
    authorization_ref: Definition next_action after the deployed replay-route timeout receipt
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: ae806003
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/32148760924
      deploy_ref: https://github.com/choir-hip/go-choir/actions/runs/32148760924
      environment_identity: staging ae806003; retained computer epoch 308; guest ae806003
      deployed_acceptance: failed — replay-completeness HTTP 502 after 120.54s; eligibility remains false
    registry_conformance_ref: "effects remains entrypoint; outer replay-route deadline is not replay eligibility, checkpoint, restore, promotion authority, or permission to retry"
  - id: effects-red-replay-route-local-deadline-repair-2026-08-18
    boundary: implement
    commit_or_artifact: 34c68283
    proof_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md, internal/proxy/self_development.go, internal/proxy/config.go, internal/proxy/self_development_test.go, nix/node-b.nix]
    rollback_ref: revert 34c68283; no product-state rollback
    disposition: "accepted as locally verified source/config repair after the outer-deadline problem receipt — ResponseController extends only the owner-only replay response deadline, Node B uses a bounded 10-minute upstream client budget, ordinary proxy routes retain the global write deadline, focused replay/server tests and Nix parse pass. Staging deployment, retained-guest refresh, replay eligibility, checkpointability, restore, self-development retry, and effects remain pending and forbidden."
    problem_ref: replay-route-outer-deadline-2026-08-18
    authorization_ref: Definition next_action after effects-red-replay-route-outer-deadline-2026-08-18
    candidate_or_evidence_refs: [docs/evidence/effects-red-computer-surface-boot-2026-08-18.md]
    landing:
      source_commit: 34c68283
      ci_ref: pending
      deploy_ref: pending
      environment_identity: staging ae806003; retained computer epoch 308; guest ae806003; route-local repair not deployed
      deployed_acceptance: pending — local route-deadline proof only; requires staging identity, same-computer refresh, and fresh replay-completeness result
    registry_conformance_ref: "effects remains entrypoint; route-local replay deadline repair is not replay eligibility, checkpoint, restore, promotion authority, or permission to retry"
  - id: effects-invoke-readiness-2026-08-15
    boundary: define
    commit_or_artifact: docs/evidence/effects-invoke-readiness-2026-08-15.md
    proof_refs: [docs/evidence/effects-invoke-readiness-2026-08-15.md, docs/definitions/choir-tape-recovery-2026-08-13.md, docs/ACTIVE.md]
    rollback_ref: revert this docs-only reconciliation
    disposition: "accepted — lateral panel GO_WITH_CAVEATS (4/4 completed). Registry promotion was not invoke-safe while start.unknowns and now.reconciliation still presented paid restore work as unpaid. This receipt is the docs-only first slice: now pinned to staging 4ac90583 / HEAD 3c12a9bb; tape-recovery receipts consumed; start.unknowns preserved with dated correction; route-map item 1 retired as live do-first. Effects remain OFF."
    problem_ref: not_applicable
    authorization_ref: owner invoke 2026-08-15 of this Definition; panel .agentic-consensus/effects-invoke-readiness-20260815
    candidate_or_evidence_refs: [docs/definitions/choir-supervised-self-development-effects-2026-08-11.md]
    landing:
      source_commit: 3c12a9bb
      ci_ref: pending (Docs Truth Check)
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583; retained epoch 268; secondary epoch 12
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint in docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml; tape-recovery remains settled non-entrypoint"
  - id: effects-decision-policy-schema-2026-08-15
    boundary: define
    commit_or_artifact: docs/evidence/effects-decision-policy-schema-2026-08-15.md
    proof_refs: [docs/evidence/effects-decision-policy-schema-2026-08-15.md, internal/agentcore/self_development_decision_binding.go, internal/platform/self_development_modes.go, docs/problems/irreversible-effects-human-gate-drift-2026-08-13.md]
    rollback_ref: revert this docs-only schema candidate
    disposition: "define freeze ACCEPT 2026-08-15 after repair. One-document schema. Not implementation. Policy-bytes sub-slice still unpaid."
    problem_ref: irreversible-effects-human-gate-drift-2026-08-13
    authorization_ref: owner correction 2026-08-13; this Definition now.next_action after invoke-readiness reconcile
    candidate_or_evidence_refs: [docs/definitions/choir-supervised-self-development-effects-2026-08-11.md]
    landing:
      source_commit: a8f75a4a
      ci_ref: pending (Docs Truth Check)
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583; retained epoch 268
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
  - id: effects-decision-policy-schema-review-2026-08-15
    boundary: define
    commit_or_artifact: docs/evidence/effects-decision-policy-schema-review-2026-08-15.md
    proof_refs: [docs/evidence/effects-decision-policy-schema-review-2026-08-15.md, docs/evidence/effects-decision-policy-schema-2026-08-15.md]
    rollback_ref: revert this docs-only review stamp
    disposition: "adjudicated REPAIR — six rejection bars passed; Sol required wire-complete fields. Repair addendum landed in the schema file. Next is review of the repaired freeze, not implement."
    problem_ref: irreversible-effects-human-gate-drift-2026-08-13
    authorization_ref: this Definition now.next_action after schema freeze
    candidate_or_evidence_refs: [docs/evidence/effects-decision-policy-schema-2026-08-15.md]
    landing:
      source_commit: ee408100
      ci_ref: pending (Docs Truth Check)
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
  - id: effects-decision-policy-schema-repair-review-2026-08-15
    boundary: define
    commit_or_artifact: docs/evidence/effects-decision-policy-schema-repair-review-2026-08-15.md
    proof_refs: [docs/evidence/effects-decision-policy-schema-repair-review-2026-08-15.md, docs/evidence/effects-decision-policy-schema-2026-08-15.md]
    rollback_ref: revert this docs-only review stamp
    disposition: "ACCEPT 4/4 — Sol's seven REPAIR items satisfied; six rejection bars closed. Schema folded into one body. Next is reversible-selfdev-v1 policy-bytes define sub-slice, not implement."
    problem_ref: irreversible-effects-human-gate-drift-2026-08-13
    authorization_ref: this Definition now.next_action after schema REPAIR
    candidate_or_evidence_refs: [docs/evidence/effects-decision-policy-schema-2026-08-15.md]
    landing:
      source_commit: 8fb8b16d
      ci_ref: pending (Docs Truth Check)
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
  - id: effects-reversible-selfdev-v1-policy-2026-08-15
    boundary: define
    commit_or_artifact: docs/evidence/effects-reversible-selfdev-v1-policy-2026-08-15.md
    proof_refs: [docs/evidence/effects-reversible-selfdev-v1-policy-2026-08-15.md, docs/evidence/effects-decision-policy-schema-2026-08-15.md]
    rollback_ref: revert this docs-only policy freeze
    disposition: "define sub-slice ACCEPT 2026-08-15 (3/3 completed). Digest c34ddf073aecaacc307f375d6f2e398798350d7a48c8d3c2e7c6d10248b394d7. Not implementation."
    problem_ref: irreversible-effects-human-gate-drift-2026-08-13
    authorization_ref: schema freeze ACCEPT; this Definition now.next_action
    candidate_or_evidence_refs: [docs/evidence/effects-decision-policy-schema-2026-08-15.md]
    landing:
      source_commit: 8889ec7b
      ci_ref: pending (Docs Truth Check)
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
  - id: effects-red-capability-ttl-and-background-renewal-2026-08-18
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-capability-ttl-and-background-renewal-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-capability-ttl-and-background-renewal-2026-08-18.md]
    rollback_ref: revert the capability TTL and background renewal repair
    disposition: "open execute — defaultComputerCapabilityTTL increased to 30m with 15m grace window; StartBackgroundRenewal proactive ticker keeps guest capability fresh across long capsule executions; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after 1fc4a369 deployed
    candidate_or_evidence_refs: [docs/evidence/effects-red-capability-ttl-and-background-renewal-2026-08-18.md]
    landing:
      source_commit: pending
      ci_ref: pending
      deploy_ref: pending
      environment_identity: staging https://choir.news deployed 1fc4a369; retained epoch 297 mode propose_only generation 1; assignment-014aeb69 executed 34 iterations; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; capability renewal fix is not freeze and is not permission to self-promote, retry Super without deploy, or send mail"
  - id: effects-red-capsule-toolchain-and-overlay-write-2026-08-18
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-capsule-toolchain-and-overlay-write-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-capsule-toolchain-and-overlay-write-2026-08-18.md]
    rollback_ref: revert 1fc4a369
    disposition: "accepted as verified deploy — 1fc4a369 fixes lower directory 0o755 permissions, base directory traversal, /run/current-system toolchain bind mount, and bash job control; assignment-014aeb69 executed 34 tool loop iterations with complete toolchain success; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after 88652f06 deployed
    candidate_or_evidence_refs: [docs/evidence/effects-red-capsule-toolchain-and-overlay-write-2026-08-18.md]
    landing:
      source_commit: 1fc4a369
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/32093401214
      deploy_ref: 1fc4a3696202b429e8f1da6ce816e86e945595ee
      environment_identity: staging https://choir.news deployed 1fc4a369; retained epoch 297 mode propose_only generation 1; assignment-014aeb69 executed 34 tool loop iterations; toolchain and overlay writes verified; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: verified via 34 successful tool loop iterations in CoSuper run:assignment-014aeb69
    registry_conformance_ref: "effects remains entrypoint; toolchain and overlay permissions fix is not freeze and is not permission to self-promote, retry Super without deploy, or send mail"
  - id: effects-red-super-coagent-cancellation-delivery-2026-08-18
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-super-coagent-cancellation-delivery-2026-08-18.md
    proof_refs: [docs/evidence/effects-red-super-coagent-cancellation-delivery-2026-08-18.md]
    rollback_ref: revert 88652f06
    disposition: "accepted as verified deploy — 88652f06 delivers CoSuper cancellation reports to persistent Super on rewake and instructs fresh assignment opening; epoch 295 operations retry unblocked Super 3a25f0b8, which opened fresh implementation CoSuper assignment-ba37afb1; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after fcee5938 deployed
    candidate_or_evidence_refs: [docs/evidence/effects-red-super-coagent-cancellation-delivery-2026-08-18.md]
    landing:
      source_commit: 88652f06
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/32088966812
      deploy_ref: 88652f064341e92684eda537add5dcdec0df1680
      environment_identity: staging https://choir.news deployed 88652f06; retained epoch 295 mode propose_only generation 1; cancellation delivered; fresh CoSuper assignment-ba37afb1 opened and bound; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: verified via operations POST 200 and CoSuper assignment-ba37afb1 bound and running
    registry_conformance_ref: "effects remains entrypoint; cancellation delivery fix is not freeze and is not permission to self-promote, retry Super without deploy, or send mail"
  - id: effects-red-cosuper-completed-run-cancel-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-cosuper-completed-run-cancel-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-cosuper-completed-run-cancel-2026-08-17.md]
    rollback_ref: revert fcee5938
    disposition: "accepted as verified deploy — fcee5938 allows restart cancel to close an assignment whose run already completed and released ActiveRunID; epoch 294 boot reconcile succeeded with zero errors and assignment-c60a8912 cancelled; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after a1f3d2cf deployed
    candidate_or_evidence_refs: [docs/evidence/effects-red-cosuper-completed-run-cancel-2026-08-17.md]
    landing:
      source_commit: fcee5938
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/32082130883
      deploy_ref: fcee593843386e027f78cecb81442ce317d8b69e
      environment_identity: staging https://choir.news deployed fcee5938; retained epoch 294 mode propose_only generation 1; assignment-c60a8912 cancelled with capsule revoked; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: guest observability commit fcee5938, boot reconcile with zero errors, assignment-c60a8912 cancelled
    registry_conformance_ref: "effects remains entrypoint; this blocker is not freeze and is not permission to self-promote, retry Super, or send mail"
  - id: effects-red-cosuper-capsule-tools-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-cosuper-capsule-tools-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-cosuper-capsule-tools-2026-08-17.md]
    rollback_ref: revert a1f3d2cf
    disposition: "accepted as verified deploy — a1f3d2cf exposes toolchain PATH and writable overlay in the capsule; the CoSuper that observed the pre-fix tool failures completed without terminalizing assignment-c60a8912; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after Texture caller reactivation
    candidate_or_evidence_refs: [docs/evidence/effects-red-cosuper-capsule-tools-2026-08-17.md]
    landing:
      source_commit: a1f3d2cf
      ci_ref: pending
      deploy_ref: a1f3d2cf93a84cf6c20e246202e6b74b90a6e932
      environment_identity: staging https://choir.news deployed a1f3d2cf; retained epoch 293 mode propose_only generation 1; capsule PATH/overlay paid; assignment-c60a8912 leftover bound; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: guest observability commit a1f3d2cf; PATH/overlay not independently re-probed because boot reconcile is blocked
    registry_conformance_ref: "effects remains entrypoint; capsule-tools deploy is not freeze and is not permission to self-promote, retry Super, or send mail"
  - id: effects-red-texture-caller-reactivate-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-texture-caller-reactivate-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-texture-caller-reactivate-2026-08-17.md]
    rollback_ref: revert 7da74d9b
    disposition: "accepted as verified live — 7da74d9b reactivates the deterministic Texture caller (3b18a6d7) instead of adopting a successor; the same operations POST returned 200 and a fresh Super c4cd7200 is running; provenance check left unchanged; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after the consensus panel recommended the caller reactivation
    candidate_or_evidence_refs: [docs/evidence/effects-red-texture-caller-reactivate-2026-08-17.md]
    landing:
      source_commit: 7da74d9b
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/32067847031
      deploy_ref: 7da74d9b78542a0b4e0e39f95dd2a7fa515e7a59
      environment_identity: staging https://choir.news deployed 7da74d9b; retained epoch 292 mode propose_only generation 1; Texture caller reactivated to 3b18a6d7; fresh Super c4cd7200 running; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: verified via operations POST 200 and Texture agent active_run projection
    registry_conformance_ref: "effects remains entrypoint; retry unblock is not freeze and is not permission to self-promote, retry Super, or send mail"
  - id: effects-red-retry-cluster-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-retry-cluster-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-retry-cluster-2026-08-17.md]
    rollback_ref: revert cec68e23
    disposition: "accepted as clustering assessment — CoSuper reconcile fixed, guest credential remedied by refresh+immediate-retry, Texture caller-run provenance mismatch open; escalated rather than patching the red Texture-canonical-write path."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after the credential-blocked retry
    candidate_or_evidence_refs: [docs/evidence/effects-red-retry-cluster-2026-08-17.md]
    landing:
      source_commit: cec68e23
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/32058303879
      deploy_ref: cec68e23afda3b2bc2554384eeb9f87e160faf5f
      environment_identity: staging https://choir.news deployed cec68e23; retained epoch 291 mode propose_only generation 1; CoSuper cancelled; credential remedied; Texture caller-run provenance mismatch; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; retry-path clustering is not freeze and is not permission to self-promote, retry Super, or send mail"
  - id: effects-red-guest-credential-renewal-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-guest-credential-renewal-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-guest-credential-renewal-2026-08-17.md]
    rollback_ref: revert this commit
    disposition: "accepted as live observation — operations-POST retry reached ensureSelfDevelopmentRun but the terminal-Super unbind failed with guest credential renewal refused during the projection event append; auth/session-renewal is a platform-control surface outside the product path; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after CoSuper terminalization
    candidate_or_evidence_refs: [docs/evidence/effects-red-guest-credential-renewal-2026-08-17.md]
    landing:
      source_commit: cec68e23
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/32058303879
      deploy_ref: cec68e23afda3b2bc2554384eeb9f87e160faf5f
      environment_identity: staging https://choir.news deployed cec68e23; retained epoch 290 mode propose_only generation 1; CoSuper cancelled; operations retry blocked by guest credential renewal refusal; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; credential renewal refusal is not freeze and is not permission to self-promote, retry Super, or send mail"
  - id: effects-red-cosuper-terminal-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-cosuper-terminal-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-cosuper-terminal-2026-08-17.md]
    rollback_ref: revert cec68e23
    disposition: "accepted as verified live — cec68e23 skips terminal-unbound capsules so restart reconcile no longer aborts at a pre-bind-cancelled assignment; CoSuper assignment-fa38b037 cancelled with capsule revoked and run cancelled; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after guest observability exposed the failing transition
    candidate_or_evidence_refs: [docs/evidence/effects-red-cosuper-terminal-2026-08-17.md]
    landing:
      source_commit: cec68e23
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/32058303879
      deploy_ref: cec68e23afda3b2bc2554384eeb9f87e160faf5f
      environment_identity: staging https://choir.news deployed cec68e23; retained epoch 290 mode propose_only generation 1; Super f8ee744f spawned bound CoSuper assignment-fa38b037; CoSuper cancelled and capsule revoked; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: verified via /api/runtime/observability and run/assignment projection
    registry_conformance_ref: "effects remains entrypoint; CoSuper terminalization is not freeze and is not permission to self-promote, retry without the operations POST, or send mail"
  - id: effects-red-cosuper-nonconvergence-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-cosuper-nonconvergence-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-cosuper-nonconvergence-2026-08-17.md]
    rollback_ref: revert 23fbd857
    disposition: "accepted as structural assessment — three correct fixes (ed3ffa3f, beb32aeb, 23fbd857) all deployed and refreshed, yet the CoSuper assignments stay unchanged and reconcile never runs on the guest; guest build/log unobservable via product API; escalating rather than blind-patching."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after sweep fix deployed and refreshed without effect
    candidate_or_evidence_refs: [docs/evidence/effects-red-cosuper-nonconvergence-2026-08-17.md]
    landing:
      source_commit: c8580621
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31996691629
      deploy_ref: c8580621db84422ecf0b7a7f3c695a22826c7946
      environment_identity: staging https://choir.news deployed c8580621; retained epoch 288 mode propose_only generation 1; Super f8ee744f spawned bound CoSuper assignment-fa38b037; assignments unchanged across three refreshes; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; non-convergence is not freeze and is not permission to self-promote, retry Super, or send mail"
  - id: effects-red-cosuper-run-state-index-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-cosuper-run-state-index-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-cosuper-run-state-index-2026-08-17.md]
    rollback_ref: revert 23fbd857
    disposition: "accepted as problem documentation and fix — legacy bind omitted the run-state metadata index so rewarm never listed run:assignment-fa38b037; 23fbd857 lists CoSuper assignments by computer and reconciles absent capsules independent of run-state and work-item projections; not a freeze; do not retry Super while live."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after boot-repair deploy left CoSuper running
    candidate_or_evidence_refs: [docs/evidence/effects-red-cosuper-run-state-index-2026-08-17.md]
    landing:
      source_commit: 23fbd857
      ci_ref: pending
      deploy_ref: pending
      environment_identity: staging https://choir.news deployed beb32aeb; retained epoch 287 mode propose_only generation 1; Super f8ee744f spawned bound CoSuper assignment-fa38b037; legacy run invisible to state index; sweep fix landed not deployed; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; legacy CoSuper run invisibility is not freeze and is not permission to self-promote, retry Super, or send mail"
  - id: effects-red-cosuper-boot-repair-still-running-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-cosuper-boot-repair-still-running-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-cosuper-boot-repair-still-running-2026-08-17.md]
    rollback_ref: revert this commit
    disposition: "accepted as live observation — beb32aeb deployed and epoch-287 owner-refresh completed, but CoSuper assignment-fa38b037 remained running with frozen updated_at, empty result, and no tokens; not a freeze; do not retry Super while live."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after boot-miss repair deploy and owner-refresh
    candidate_or_evidence_refs: [docs/evidence/effects-red-cosuper-boot-repair-still-running-2026-08-17.md]
    landing:
      source_commit: beb32aeb
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31993817931
      deploy_ref: beb32aeb265b39a9e0e089450b5239219beedd6d
      environment_identity: staging https://choir.news deployed beb32aeb; retained epoch 287 mode propose_only generation 1; Super f8ee744f spawned bound CoSuper assignment-fa38b037; CoSuper still running after repair deploy and refresh; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; CoSuper still running after boot-repair refresh is not freeze and is not permission to self-promote, retry Super, or send mail"
  - id: effects-red-cosuper-boot-miss-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-cosuper-boot-miss-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-cosuper-boot-miss-2026-08-17.md]
    rollback_ref: revert this commit
    disposition: "accepted as inspection — epoch-286 boot left assignment-fa38b037 running because bind omitted lifecycle run state so rewarm never listed it, leftover cgroup load can fail closed, and overlay-bind failure did not join assignment cancel; not a freeze; do not retry Super while live."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after ForceDestroy wait bound refresh
    candidate_or_evidence_refs: [docs/evidence/effects-red-cosuper-boot-miss-2026-08-17.md]
    landing:
      source_commit: ed3ffa3f
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31990768871
      deploy_ref: ed3ffa3f7eb290e13a510af5a1382d86d57329c2
      environment_identity: staging https://choir.news deployed ed3ffa3f; retained epoch 286 mode propose_only generation 1; Super f8ee744f spawned bound CoSuper assignment-fa38b037; boot miss diagnosed; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; boot miss is not freeze and is not permission to self-promote, retry Super, or send mail"
  - id: effects-red-cosuper-refresh-still-running-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-cosuper-refresh-still-running-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-cosuper-refresh-still-running-2026-08-17.md]
    rollback_ref: revert this commit
    disposition: "accepted as live observation — ForceDestroy wait bound deployed as ed3ffa3f; G4 preserved constructed computer; owner-refresh epoch 286; CoSuper assignment-fa38b037 still running with frozen updated_at; not a freeze; do not retry Super while live."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after ForceDestroy wait bound
    candidate_or_evidence_refs: [docs/evidence/effects-red-cosuper-refresh-still-running-2026-08-17.md]
    landing:
      source_commit: ed3ffa3f
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31990768871
      deploy_ref: ed3ffa3f7eb290e13a510af5a1382d86d57329c2
      environment_identity: staging https://choir.news deployed ed3ffa3f; retained epoch 286 mode propose_only generation 1; Super f8ee744f spawned bound CoSuper assignment-fa38b037; CoSuper still running after refresh; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; hung CoSuper after refresh is not freeze and is not permission to self-promote, retry Super, or send mail"
  - id: effects-red-cosuper-activation-budget-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-cosuper-activation-budget-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-cosuper-activation-budget-2026-08-17.md]
    rollback_ref: revert this commit
    disposition: "accepted as problem documentation — CoSuper assignment-fa38b037 stayed running past default 60m ActivationBudget with frozen updated_at and no tokens; assigned-CoSuper progress deadline cannot finish while ForceDestroy waits unbounded on Background ctx; not a freeze; no Super retry while live."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after first live CoSuper spawn
    candidate_or_evidence_refs: [docs/evidence/effects-red-cosuper-activation-budget-2026-08-17.md]
    landing:
      source_commit: dddcd80d
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31985817733
      deploy_ref: dddcd80da0547ba476f4aee7d431ec70f84f44d5
      environment_identity: staging https://choir.news deployed dddcd80d; retained epoch 285 mode propose_only generation 1; Super f8ee744f spawned bound CoSuper assignment-fa38b037; CoSuper still running past activation budget; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; hung CoSuper is not freeze and is not permission to self-promote, retry Super, or send mail"
  - id: effects-red-super-cosuper-spawn-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-super-cosuper-spawn-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-super-cosuper-spawn-2026-08-17.md]
    rollback_ref: revert this commit
    disposition: "accepted as live observation — Super f8ee744f spawned bound CoSuper assignment-fa38b037 after proc/sys overlay mask; CoSuper still running with no freeze; do not retry Super while that CoSuper is live."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after capsule proc/sys tmpfs mask
    candidate_or_evidence_refs: [docs/evidence/effects-red-super-cosuper-spawn-2026-08-17.md]
    landing:
      source_commit: dddcd80d
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31985817733
      deploy_ref: dddcd80da0547ba476f4aee7d431ec70f84f44d5
      environment_identity: staging https://choir.news deployed dddcd80d; retained epoch 285 mode propose_only generation 1; Super f8ee744f spawned bound CoSuper assignment-fa38b037; CoSuper still running; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; CoSuper spawn is not freeze and is not permission to self-promote, retry Super, or send mail"
  - id: effects-red-super-broker-readiness-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-super-broker-readiness-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-super-broker-readiness-2026-08-17.md, internal/capsule/executor.go]
    rollback_ref: revert this commit
    disposition: "accepted as live observation plus proc/sys tmpfs mask — Super 2210c654 Texture join paid then capsule broker readiness timed out; source masks guest proc/sys in the overlay and surfaces the last readiness probe error; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after capsule broker start timeout
    candidate_or_evidence_refs: [docs/evidence/effects-red-super-broker-readiness-2026-08-17.md]
    landing:
      source_commit: 6fafd2f9
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31983808820
      deploy_ref: 6fafd2f9effd677015ae3b447d4a83d0a4a9c05d
      environment_identity: staging https://choir.news deployed 6fafd2f9; retained epoch 284 mode propose_only generation 1; Super 2210c654 Texture join paid then broker readiness timed out; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; Texture join and CoSuper open are not freeze and are not permission to self-promote or send mail"
  - id: effects-red-super-capsule-spawn-timeout-2026-08-17
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-super-capsule-spawn-timeout-2026-08-17.md
    proof_refs: [docs/evidence/effects-red-super-capsule-spawn-timeout-2026-08-17.md, internal/capsule/start_command.go]
    rollback_ref: revert this commit
    disposition: "accepted as live observation plus broker start timeout — Super 8c6b660d Texture join paid then hung after hosts write; source bounds broker mount/Start and CoSuper Spawn; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after capsule identity-etc upperdir write
    candidate_or_evidence_refs: [docs/evidence/effects-red-super-capsule-spawn-timeout-2026-08-17.md]
    landing:
      source_commit: 34575a8a
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31980781341
      deploy_ref: 34575a8a1d8d894ecf4373f2334f1ccb78116672
      environment_identity: staging https://choir.news deployed 34575a8a; retained epoch 283 mode propose_only generation 1; Super 8c6b660d Texture join paid then hung after hosts write; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; a hung Super is not freeze and is not permission to self-promote or send mail"
  - id: effects-red-super-capsule-hosts-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-super-capsule-hosts-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-super-capsule-hosts-2026-08-16.md, internal/capsule/executor.go]
    rollback_ref: revert this commit
    disposition: "accepted as live observation plus capsule etc upperdir write — Super 00ebeb3d Texture rewake paid then capsule spawn EROFS on merged /etc/hosts; source writes identity files into overlay upperdir; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after Texture Super rewake
    candidate_or_evidence_refs: [docs/evidence/effects-red-super-capsule-hosts-2026-08-16.md]
    landing:
      source_commit: d3819c3b
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31979456374
      deploy_ref: d3819c3bbaeeb6bd39fecec3693948f5f7f4afc1
      environment_identity: staging https://choir.news deployed d3819c3b; retained epoch 282 mode propose_only generation 1; Super 00ebeb3d Texture rewake paid then capsule spawn EROFS; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; Texture rewake and CoSuper open are not freeze and are not permission to self-promote or send mail"
  - id: effects-red-super-texture-rewake-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-super-texture-rewake-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-super-texture-rewake-2026-08-16.md, internal/agentcore/selfdev_texture_join.go]
    rollback_ref: revert this commit
    disposition: "accepted as live observation plus Texture rewake — Super dc66c265 remained terminal; operations POST 409 Texture control did not wake persistent Super; source issues a Texture-authorized rewake when unique Super is terminal; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after Texture Super inject-after-tools restore
    candidate_or_evidence_refs: [docs/evidence/effects-red-super-texture-rewake-2026-08-16.md]
    landing:
      source_commit: efdc131a
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31977938737
      deploy_ref: efdc131a899d4f445fa6666ea892743cd3b3d312
      environment_identity: staging https://choir.news deployed efdc131a; retained epoch 281 mode propose_only generation 1; Super dc66c265 terminal; retry 409 Texture control did not wake persistent Super; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; Texture rewake is not freeze and is not permission to self-promote or send mail"
  - id: effects-red-super-inject-after-tools-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-super-inject-after-tools-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-super-inject-after-tools-2026-08-16.md, internal/agentcore/super_controller.go]
    rollback_ref: revert this commit
    disposition: "accepted as live observation plus inject restore — Super dc66c265 passed Texture join then failed inject-after-tools record not found; source restores Super agent before re-list and does not fail the Super run on later ErrNotFound; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after persistent Super agent-channel restore
    candidate_or_evidence_refs: [docs/evidence/effects-red-super-inject-after-tools-2026-08-16.md]
    landing:
      source_commit: 134b60dd
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31976060235
      deploy_ref: 134b60dd3103d53c954600ed1cd36c18e73cc4ea
      environment_identity: staging https://choir.news deployed 134b60dd; retained epoch 280 mode propose_only generation 1; Super dc66c265 Texture join paid then inject-after-tools failed; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; Texture join is not freeze and is not permission to self-promote or send mail"
  - id: effects-red-super-assign-channel-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-super-assign-channel-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-super-assign-channel-2026-08-16.md, internal/agentcore/selfdev_texture_join.go]
    rollback_ref: revert this commit
    disposition: "accepted as live observation plus agent-channel restore — Super d45d4eb1 passed Texture join with assignment_trajectory_id then assign_co_super refused Super agent ChannelID overwrite; source restores persistent Super agent after Texture wake; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after Texture-control Super join land
    candidate_or_evidence_refs: [docs/evidence/effects-red-super-assign-channel-2026-08-16.md]
    landing:
      source_commit: 7fed24fe
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31973948466
      deploy_ref: 7fed24fe4a394a3ea0acf469e630a5422589a06a
      environment_identity: staging https://choir.news deployed 7fed24fe; retained epoch 279 mode propose_only generation 1; Super d45d4eb1 Texture join paid then assign_co_super refused Super agent channel; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; Texture join is not freeze and is not permission to self-promote or send mail"
  - id: effects-red-super-texture-control-join-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-super-texture-control-join-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-super-texture-control-join-2026-08-16.md, internal/agentcore/selfdev_texture_join.go]
    rollback_ref: revert this commit
    disposition: "accepted as source land — HTTP Super start now applies a Texture Direction=control execution_request plus Super-targeted work using packet.sources operation: URI; persistent Super stays non-lifecycle; live retry unpaid until this deploys; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after Texture-join refusal
    candidate_or_evidence_refs: [docs/evidence/effects-red-super-texture-control-join-2026-08-16.md]
    landing:
      source_commit: 17c1fea3
      ci_ref: pending
      deploy_ref: a9ddf2db7b6f26739be1e482f5bb80d8f345c8cf
      environment_identity: staging https://choir.news still deployed a9ddf2db until this commit; retained epoch 278 mode propose_only generation 1; Super c003412a blocked on Texture join; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; Texture control join is not freeze and is not permission to self-promote or send mail"
  - id: effects-red-super-texture-join-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-super-texture-join-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-super-texture-join-2026-08-16.md]
    rollback_ref: revert this commit
    disposition: "accepted as live observation — fresh Super c003412a passed requirePersistentSuperExecution then assign_co_super refused missing assignment_trajectory_id; Texture Direction=control execution_request plus Super-targeted work still unpaid; not a freeze."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after fresh persistent Super start
    candidate_or_evidence_refs: [docs/evidence/effects-red-super-texture-join-2026-08-16.md]
    landing:
      source_commit: a9ddf2db
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31971778721
      deploy_ref: a9ddf2db7b6f26739be1e482f5bb80d8f345c8cf
      environment_identity: staging https://choir.news deployed a9ddf2db; retained epoch 278 mode propose_only generation 1; Super c003412a blocked on Texture join; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; passing persistent Super identity is not freeze and is not permission to forge Texture control or send mail"
  - id: effects-red-super-fresh-start-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-super-fresh-start-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-super-fresh-start-2026-08-16.md, internal/agentcore/api_self_development.go]
    rollback_ref: revert this commit
    disposition: "accepted as live observation plus start-path fix — owner refresh 276→277 on 82d87bc0; same-run revive of cdf0af4c replayed blocked memory; terminal Super is now unbound and a new persistent Super is started; not a freeze; Texture control join still unpaid."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after persistent Super start land
    candidate_or_evidence_refs: [docs/evidence/effects-red-super-fresh-start-2026-08-16.md]
    landing:
      source_commit: 82d87bc0
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31970219783
      deploy_ref: 82d87bc0a675fae30c33985666cdebbe4d63b241
      environment_identity: staging https://choir.news deployed 82d87bc0; retained epoch 277 mode propose_only generation 1; Super cdf0af4c revived then restated blocked memory; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; a fresh Super start is not freeze and is not permission to promote or send mail"
  - id: effects-red-super-persistent-identity-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-super-persistent-identity-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-super-persistent-identity-2026-08-16.md, internal/agentcore/api_self_development.go]
    rollback_ref: revert this commit
    disposition: "accepted as live observation plus start-path fix — Super cdf0af4c completed blocked because TrajectoryID was set; self-development Super start now strips trajectory like persistent Super; revive of terminal Super with no bundle; not a freeze; Texture control join still unpaid."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after Super start
    candidate_or_evidence_refs: [docs/evidence/effects-red-super-persistent-identity-2026-08-16.md]
    landing:
      source_commit: af08767e
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31968160935
      deploy_ref: a9e4af419aa96018410cb13840cc0ee94afe39cb
      environment_identity: staging https://choir.news deployed a9e4af41; retained epoch 276 mode propose_only generation 1; Super cdf0af4c completed blocked; operation selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; persistent Super identity is not freeze and is not permission to promote or send mail"
  - id: effects-red-super-start-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-super-start-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-super-start-2026-08-16.md]
    rollback_ref: not_applicable
    disposition: "accepted as named solitaire Super start — operation selfdev-b090bcd7 executing on propose_only generation 1 ModeReceipt; no freeze; no promote; genesis 409; no live send."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after restore-eligible pre-A checkpoint
    candidate_or_evidence_refs: [docs/evidence/effects-red-super-start-2026-08-16.md]
    landing:
      source_commit: a9e4af41
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31968160935
      deploy_ref: a9e4af419aa96018410cb13840cc0ee94afe39cb
      environment_identity: staging https://choir.news deployed a9e4af41; retained epoch 276 mode propose_only generation 1; Super selfdev-b090bcd7 executing; OwnerRecovery 663540be not for promotion; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; Super start is not live proof and is not permission to promote or send mail"
  - id: effects-red-pre-a-checkpoint-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-pre-a-checkpoint-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-pre-a-checkpoint-2026-08-16.md]
    rollback_ref: not_applicable
    disposition: "accepted as restore-eligible pre-A fence — OwnerRecovery checkpoint 663540be at epoch 276 sequence 32; SPA derived from trusted image baseline; Super not started; not a promotion checkpoint; no live send."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after checkpoint baseline-SPA deploy
    candidate_or_evidence_refs: [docs/evidence/effects-red-pre-a-checkpoint-2026-08-16.md]
    landing:
      source_commit: a9e4af41
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31968160935
      deploy_ref: a9e4af419aa96018410cb13840cc0ee94afe39cb
      environment_identity: staging https://choir.news deployed a9e4af41; retained epoch 276 mode propose_only generation 1; sequence 32 eligible; checkpoint 663540be owner_recovery; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; OwnerRecovery pre-A fence is not live proof and is not permission to promote"
  - id: effects-checkpoint-baseline-spa-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-checkpoint-baseline-spa-2026-08-16.md
    proof_refs: [docs/evidence/effects-checkpoint-baseline-spa-2026-08-16.md, docs/evidence/effects-replay-eligible-spa-underivable-2026-08-16.md, internal/agentcore/rematerialize.go]
    rollback_ref: revert this commit
    disposition: "accepted as checkpoint trusted-baseline SPA import when updater current is missing — same /nix/store genesis/kernel root; not rematerialize; not genesis. Super not started. No live send."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after replay-eligible SPA 409
    candidate_or_evidence_refs: [docs/evidence/effects-checkpoint-baseline-spa-2026-08-16.md]
    landing:
      source_commit: 06204346
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31966792512
      deploy_ref: 06204346a431f52347586b7f68d39a4d2b9c282a
      environment_identity: staging https://choir.news deployed 06204346; retained epoch 275 mode propose_only generation 1; sequence 31 eligible; checkpoint 409 SPA underivable; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; baseline SPA import is not live proof and is not permission to start Super"
  - id: effects-replay-eligible-spa-underivable-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-replay-eligible-spa-underivable-2026-08-16.md
    proof_refs: [docs/evidence/effects-replay-eligible-spa-underivable-2026-08-16.md]
    rollback_ref: not_applicable
    disposition: "accepted as live observation — replay eligible at sequence 31 after second residue import; checkpoint 409 served SPA underivable because refresh dropped updater current. Super not started. No live send."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after event_projection deploy
    candidate_or_evidence_refs: [docs/evidence/effects-replay-eligible-spa-underivable-2026-08-16.md]
    landing:
      source_commit: 06204346
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31966792512
      deploy_ref: 06204346a431f52347586b7f68d39a4d2b9c282a
      environment_identity: staging https://choir.news deployed 06204346; retained epoch 275; sequence 31 eligible; checkpoint 409
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; eligible replay is not live proof and is not permission to start Super"
  - id: effects-desktop-og-event-projection-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-desktop-og-event-projection-2026-08-16.md
    proof_refs: [docs/evidence/effects-desktop-og-event-projection-2026-08-16.md, docs/evidence/effects-live-residue-import-2026-08-16.md, internal/agentcore/replay_eligibility.go, internal/store/project.go]
    rollback_ref: revert this commit
    disposition: "accepted as desktop+OG event_projection after live residue import — projector replaces leftover desktop_sessions; presence_volatile still refused; desktop_state stays empty_until_supported. Not live checkpoint. Super not started. No live send."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after live residue import existed
    candidate_or_evidence_refs: [docs/evidence/effects-desktop-og-event-projection-2026-08-16.md]
    landing:
      source_commit: bc379902
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31965171961
      deploy_ref: bc3799022704307cb8adf7d2e7bd1eab31df6878
      environment_identity: staging https://choir.news deployed bc379902; retained epoch 274 mode propose_only generation 1; import appended sequence 29; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; reclassification is not live proof and is not permission to start Super"
  - id: effects-live-residue-import-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-live-residue-import-2026-08-16.md
    proof_refs: [docs/evidence/effects-live-residue-import-2026-08-16.md]
    rollback_ref: not_applicable
    disposition: "accepted as live owner-scoped residue snapshot import — 1 desktop, 2 sessions, 24 objects, appended true at sequence 29. Super not started. No live send. EmptyUntilSupported not reclassified in this actuation."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after refresh-runtime image deploy
    candidate_or_evidence_refs: [docs/evidence/effects-live-residue-import-2026-08-16.md]
    landing:
      source_commit: bc379902
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31965171961
      deploy_ref: bc3799022704307cb8adf7d2e7bd1eab31df6878
      environment_identity: staging https://choir.news deployed bc379902; retained epoch 274; sequence 29; import appended
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; live import is not live proof and is not permission to start Super"
  - id: effects-refresh-runtime-image-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-refresh-runtime-image-2026-08-16.md
    proof_refs: [docs/evidence/effects-refresh-runtime-image-2026-08-16.md, docs/evidence/effects-refresh-stale-updater-current-2026-08-16.md, internal/vmmanager/manager.go, nix/autoputer-vm.nix]
    rollback_ref: revert this commit
    disposition: "accepted as refresh-runtime image contract — owner/deploy refresh drops stale updater current and execs the current deploy image; restart/recover keep the pointer. Not live import. Super not started. No live send."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after import 404 on epoch 273
    candidate_or_evidence_refs: [docs/evidence/effects-refresh-runtime-image-2026-08-16.md]
    landing:
      source_commit: c6dda135
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31963987149
      deploy_ref: c6dda13592c5e21ed17355fa5939a600c9534514
      environment_identity: staging https://choir.news deployed c6dda135; retained epoch 273 mode propose_only generation 1; constructed freeze 7122f279; import 404
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; image-runtime refresh is not live proof and is not permission to start Super"
  - id: effects-refresh-stale-updater-current-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-refresh-stale-updater-current-2026-08-16.md
    proof_refs: [docs/evidence/effects-refresh-stale-updater-current-2026-08-16.md]
    rollback_ref: not_applicable
    disposition: "accepted as live observation — host c6dda135, owner refresh 272→273, import still 404 because updater current masked the image. Super not started. No live send."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after residue import proxy
    candidate_or_evidence_refs: [docs/evidence/effects-refresh-stale-updater-current-2026-08-16.md]
    landing:
      source_commit: c6dda135
      ci_ref: https://github.com/choir-hip/go-choir/actions/runs/31963987149
      deploy_ref: c6dda13592c5e21ed17355fa5939a600c9534514
      environment_identity: staging https://choir.news deployed c6dda135; retained epoch 273; import 404 computer route not found
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; 404 is not live proof"
  - id: effects-residue-import-proxy-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-residue-import-proxy-2026-08-16.md
    proof_refs: [docs/evidence/effects-residue-import-proxy-2026-08-16.md, internal/proxy/computer_lifecycle.go, internal/proxy/computer_workspace_replace_test.go]
    rollback_ref: revert this commit
    disposition: "accepted as owner-scoped residue import proxy path — same product path as checkpoint; not VM lifecycle control; not executed live. Super not started. No live send."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after residue import command
    candidate_or_evidence_refs: [docs/evidence/effects-residue-import-proxy-2026-08-16.md]
    landing:
      source_commit: 952aaa5d
      ci_ref: pending
      deploy_ref: 1a17a035960a454e16b163068792479c09d76de3
      environment_identity: staging https://choir.news deployed 1a17a035; retained epoch 272 mode propose_only generation 1; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; proxy path is not live proof and is not permission to start Super"
  - id: effects-residue-import-command-2026-08-16
    boundary: define
    commit_or_artifact: docs/evidence/effects-residue-import-command-2026-08-16.md
    proof_refs: [docs/evidence/effects-residue-import-command-2026-08-16.md, internal/agentcore/residue_import.go, cmd/choir/main.go]
    rollback_ref: revert this commit
    disposition: "accepted as owner-scoped residue import command — POST lifecycle/import-residue-snapshot; not auto-run; not executed live; EmptyUntilSupported unchanged. Super not started. No live send."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after residue import Store method
    candidate_or_evidence_refs: [docs/evidence/effects-residue-import-command-2026-08-16.md]
    landing:
      source_commit: 474b5053
      ci_ref: pending
      deploy_ref: 1a17a035960a454e16b163068792479c09d76de3
      environment_identity: staging https://choir.news deployed 1a17a035; retained epoch 272 mode propose_only generation 1; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; residue import command is not live proof and is not permission to start Super"
  - id: effects-residue-import-2026-08-16
    boundary: define
    commit_or_artifact: docs/evidence/effects-residue-import-2026-08-16.md
    proof_refs: [docs/evidence/effects-residue-import-2026-08-16.md, internal/store/residue_import.go, internal/store/residue_import_test.go]
    rollback_ref: revert this commit
    disposition: "accepted as residue-import code — one snapshot batch of desktop+OG; split residue refused; presence omitted; EmptyUntilSupported unchanged; not executed live. Super not started. No live send."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after live writer cutover
    candidate_or_evidence_refs: [docs/evidence/effects-residue-import-2026-08-16.md]
    landing:
      source_commit: 92149d06
      ci_ref: pending
      deploy_ref: 5557840cab6565ecebb015c4f60b627810b7c1fd
      environment_identity: staging https://choir.news deployed 5557840c; retained epoch 272 mode propose_only generation 1; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; residue import code is not live proof and is not permission to start Super"
  - id: effects-live-desktop-og-tape-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-live-desktop-og-tape-2026-08-16.md
    proof_refs: [docs/evidence/effects-live-desktop-og-tape-2026-08-16.md, internal/store/projection_tape.go, internal/store/project_test.go, internal/autoputer/run.go]
    rollback_ref: revert this commit
    disposition: "accepted as live desktop+OG writer cutover — BindProjectionTape append+project; presence off Dolt; platform payload GET added; no residue import; EmptyUntilSupported unchanged. Super not started. No live send."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after SQL-only Project
    candidate_or_evidence_refs: [docs/evidence/effects-live-desktop-og-tape-2026-08-16.md]
    landing:
      source_commit: 1a17a035
      ci_ref: pending
      deploy_ref: 5557840cab6565ecebb015c4f60b627810b7c1fd
      environment_identity: staging https://choir.news deployed 5557840c; retained epoch 272 mode propose_only generation 1; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; live writer cutover is not live proof and is not permission to start Super"
  - id: effects-sql-only-project-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-sql-only-project-2026-08-16.md
    proof_refs: [docs/evidence/effects-sql-only-project-2026-08-16.md, internal/store/project.go, internal/store/project_test.go, internal/computerevent/appender.go]
    rollback_ref: revert this commit
    disposition: "accepted as SQL-only Project — atomic desktop+OG batches inside Finalize; ResolvePayloads before BeginTx; desktop_sessions presence is in-memory not Dolt; EmptyUntilSupported not weakened; live writers not cut over; no residue import. Super not started. No live send."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after contracts freeze
    candidate_or_evidence_refs: [docs/evidence/effects-sql-only-project-2026-08-16.md]
    landing:
      source_commit: 9d244d9b
      ci_ref: pending
      deploy_ref: 5557840cab6565ecebb015c4f60b627810b7c1fd
      environment_identity: staging https://choir.news deployed 5557840c; retained epoch 272 mode propose_only generation 1; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; Project is not live proof and is not permission to start Super"
  - id: effects-unified-tape-contracts-2026-08-16
    boundary: define
    commit_or_artifact: docs/evidence/effects-unified-tape-contracts-2026-08-16.md
    proof_refs: [docs/evidence/effects-unified-tape-contracts-2026-08-16.md, internal/computerevent/payload_resolver.go, internal/computerevent/projection_batch.go, internal/computerevent/appender.go]
    rollback_ref: revert this commit
    disposition: "accepted as unified-tape contracts freeze — ResolvePayloads before SQL; ProjectionBatch; poison Finalize does not silent-retry; desktop_sessions stays empty_until_supported; choir.event consumers inventoried. Live Project unpaid. Super not started."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after complete_from_head
    candidate_or_evidence_refs: [docs/evidence/effects-unified-tape-contracts-2026-08-16.md]
    landing:
      source_commit: 01d13473
      ci_ref: pending
      deploy_ref: 5557840cab6565ecebb015c4f60b627810b7c1fd
      environment_identity: staging https://choir.news deployed 5557840c; retained epoch 272 mode propose_only generation 1; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; contracts are not live proof and are not permission to start Super"
  - id: effects-recovery-domain-complete-from-head-2026-08-16
    boundary: define
    commit_or_artifact: docs/evidence/effects-recovery-domain-complete-from-head-2026-08-16.md
    proof_refs: [docs/evidence/effects-recovery-domain-complete-from-head-2026-08-16.md, internal/selfdevprotocol/tape_completeness.go, internal/selfdevprotocol/tape_completeness_test.go]
    rollback_ref: revert this commit
    disposition: "accepted as recovery domain complete_from_head — new_epoch refused; incomplete-tape restore remains the tape-recovery substrate; full projected restore fails closed before complete_from_head. Super not started. No live send."
    problem_ref: not_applicable
    authorization_ref: owner complete-tape correction; Reduce duplicate-genesis refuse; no choir computer create; no rematerialize
    candidate_or_evidence_refs: [docs/evidence/effects-recovery-domain-complete-from-head-2026-08-16.md]
    landing:
      source_commit: 8cc24ca6
      ci_ref: pending
      deploy_ref: 5557840cab6565ecebb015c4f60b627810b7c1fd
      environment_identity: staging https://choir.news deployed 5557840c; retained epoch 272 mode propose_only generation 1; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; complete_from_head is not live proof and is not permission to start Super"
  - id: effects-unified-event-tape-design-review-2026-08-16
    boundary: define
    commit_or_artifact: docs/choir-unified-event-tape-design-2026-08-16.md
    proof_refs: [docs/choir-unified-event-tape-design-2026-08-16.md, docs/evidence/effects-unified-event-tape-design-review-2026-08-16.md]
    rollback_ref: revert this docs-only stamp
    disposition: "accepted as unified-tape design plus divergent panel — one computerevent stream; APPROVE_WITH_CONDITIONS. Restore of pre-completeness heads must fail closed. Super not started. No live send. Owner must name complete_from_head vs new epoch before Project implementation."
    problem_ref: not_applicable
    authorization_ref: owner correction merge two event systems; tape covers arbitrary prior-state recovery; no backwards compatibility
    candidate_or_evidence_refs: [docs/choir-unified-event-tape-design-2026-08-16.md, docs/evidence/effects-unified-event-tape-design-review-2026-08-16.md]
    landing:
      source_commit: db59cb41
      ci_ref: pending
      deploy_ref: 5557840cab6565ecebb015c4f60b627810b7c1fd
      environment_identity: staging https://choir.news deployed 5557840c; retained epoch 272 mode propose_only generation 1; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; design is not live proof and is not permission to start Super"
  - id: effects-pre-a-checkpoint-eligibility-review-2026-08-16
    boundary: define
    commit_or_artifact: docs/choir-effects-pre-a-checkpoint-eligibility-2026-08-16.md
    proof_refs: [docs/choir-effects-pre-a-checkpoint-eligibility-2026-08-16.md, docs/evidence/effects-pre-a-checkpoint-eligibility-review-2026-08-16.md]
    rollback_ref: revert this docs-only stamp
    disposition: "accepted as owner-requested report plus convergent panel — 3/5 OPTION_2 medium, dissent OPTION_1 and OPTION_4. Cutover not executed. Super not started. No live send. Owner ratification required before any workspace-replace."
    problem_ref: not_applicable
    authorization_ref: owner request for report, PDF, and agentic consensus
    candidate_or_evidence_refs: [docs/choir-effects-pre-a-checkpoint-eligibility-2026-08-16.md, docs/evidence/effects-pre-a-checkpoint-eligibility-review-2026-08-16.md]
    landing:
      source_commit: 90734517
      ci_ref: pending
      deploy_ref: 5557840cab6565ecebb015c4f60b627810b7c1fd
      environment_identity: staging https://choir.news deployed 5557840c; retained epoch 272 mode propose_only generation 1; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; panel is not live proof and is not permission to cut over"
  - id: effects-red-pre-a-checkpoint-ineligible-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-pre-a-checkpoint-ineligible-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-pre-a-checkpoint-ineligible-2026-08-16.md, docs/evidence/effects-red-named-mode-2026-08-16.md]
    rollback_ref: revert this docs-only stamp
    disposition: "accepted as named solitaire prompt plus ineligible pre-A checkpoint — refresh 271→272 re-exchanged guest credentials; checkpoint 409 because desktop_* and og_objects are non-empty EmptyUntilSupported tables. Super not started. No live send. Do not weaken the gate. Promote/restore unpaid."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after named propose_only
    candidate_or_evidence_refs: [docs/evidence/effects-red-pre-a-checkpoint-ineligible-2026-08-16.md]
    landing:
      source_commit: 81952e13
      ci_ref: pending
      deploy_ref: 5557840cab6565ecebb015c4f60b627810b7c1fd
      environment_identity: staging https://choir.news deployed 5557840c; retained epoch 272 mode propose_only generation 1; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; ineligible checkpoint is not live proof"
  - id: effects-red-named-mode-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-named-mode-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-named-mode-2026-08-16.md, cmd/choir/main.go, cmd/choir/main_test.go]
    rollback_ref: revert this named-mode commit; CAS mode back to off
    disposition: "accepted as named red mode — start is propose_only (live generation 1); decision is qualified_consensus bound to reversible-selfdev-v1. Genesis 409. Start without mode_receipt 409. No Super started. No live send. Promote/restore unpaid."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after verifier refresh
    candidate_or_evidence_refs: [docs/evidence/effects-red-named-mode-2026-08-16.md]
    landing:
      source_commit: 96dead51
      ci_ref: pending
      deploy_ref: 5557840cab6565ecebb015c4f60b627810b7c1fd
      environment_identity: staging https://choir.news deployed 5557840c; retained epoch 271 mode propose_only generation 1; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; named mode is not live proof"
  - id: effects-red-verifier-refresh-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-verifier-refresh-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-verifier-refresh-2026-08-16.md, docs/evidence/effects-guest-verifier-2026-08-16.md]
    rollback_ref: revert this docs-only stamp
    disposition: "accepted as live refresh after verifier wiring — staging 5557840c; choir computer refresh 270→271; start 409; genesis 409; kernel 200 lifecycle_generation 271; mode off; no operation created. No live send. Promote/restore red unpaid."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after guest verifier wiring
    candidate_or_evidence_refs: [docs/evidence/effects-red-verifier-refresh-2026-08-16.md]
    landing:
      source_commit: 5557840c
      ci_ref: "https://github.com/choir-hip/go-choir/actions/runs/31928656492"
      deploy_ref: 5557840cab6565ecebb015c4f60b627810b7c1fd
      environment_identity: staging https://choir.news deployed 5557840c at 2026-08-16T05:32:15Z; retained epoch 271 mode off; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; verifier refresh is not live proof"
  - id: effects-guest-verifier-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-guest-verifier-2026-08-16.md
    proof_refs: [docs/evidence/effects-guest-verifier-2026-08-16.md, internal/autoputer/run.go, nix/autoputer-vm.nix, internal/autoputer/run_test.go]
    rollback_ref: revert this guest-verifier commit
    disposition: "accepted as guest verifier wiring — WithSelfDevelopmentVerifier mounts verifier-control certificate signing so the materializer can run once an authorized operation exists. Mode is not set. Outbox unarmed. No live send. Live verifier unpaid until deploy plus owner-scoped refresh."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after live kernel GET 200
    candidate_or_evidence_refs: [docs/evidence/effects-guest-verifier-2026-08-16.md]
    landing:
      source_commit: cb4ff48f
      ci_ref: pending
      deploy_ref: 7eee9f106cdb987b6a1c5846ae3bf2bdd94f0525
      environment_identity: staging https://choir.news still 7eee9f10 until this commit deploys; retained epoch 270 mode off; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; verifier wiring is not live proof"
  - id: effects-red-kernel-route-live-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-kernel-route-live-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-kernel-route-live-2026-08-16.md, docs/evidence/effects-guest-kernel-route-2026-08-16.md]
    rollback_ref: revert this docs-only stamp
    disposition: "accepted as live kernel capability receipt — staging 7eee9f10; choir computer refresh 269→270; GET kernel-capabilities 200 signed KernelCapabilityReceipt issuer choir-updater lifecycle_generation 270; start 409 current signed mode does not authorize proposal; genesis 409; mode off; no operation created. Verifier unmounted. No live send. Promote/restore red unpaid."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after guest kernel/route wiring
    candidate_or_evidence_refs: [docs/evidence/effects-red-kernel-route-live-2026-08-16.md]
    landing:
      source_commit: 7eee9f10
      ci_ref: "https://github.com/choir-hip/go-choir/actions/runs/31927114217"
      deploy_ref: 7eee9f106cdb987b6a1c5846ae3bf2bdd94f0525
      environment_identity: staging https://choir.news deployed 7eee9f10 at 2026-08-16T05:04:03Z; retained epoch 270 mode off; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; live kernel GET 200 is not live proof"
  - id: effects-guest-kernel-route-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-guest-kernel-route-2026-08-16.md
    proof_refs: [docs/evidence/effects-guest-kernel-route-2026-08-16.md, internal/autoputer/run.go, internal/agentcore/api_self_development.go, internal/agentcore/api_self_development_test.go]
    rollback_ref: revert this guest-kernel-route commit
    disposition: "accepted as guest kernel/route wiring — WithSelfDevelopmentRoute mounts vmctl computer-version reads so kernel-capabilities can fail closed past unmounted authority. Updater already mounted. Verifier remains unmounted. Mode is not set. Outbox unarmed. No live send. Live kernel probe unpaid until deploy plus owner-scoped refresh."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after live 409 mode-off refuse
    candidate_or_evidence_refs: [docs/evidence/effects-guest-kernel-route-2026-08-16.md]
    landing:
      source_commit: d61fcd91
      ci_ref: pending
      deploy_ref: 21b7987292c0cbbd42e8e58ea2059334a6756208
      environment_identity: staging https://choir.news still 21b79872 until this commit deploys; retained epoch 269 mode off; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; kernel/route wiring is not live proof"
  - id: effects-red-mode-off-refuse-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-mode-off-refuse-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-mode-off-refuse-2026-08-16.md, docs/evidence/effects-owner-guest-boot-refresh-2026-08-16.md]
    rollback_ref: revert this docs-only stamp
    disposition: "accepted as red mode-off refuse — staging 21b79872; choir computer refresh 268→269; start 409 current signed mode does not authorize proposal; genesis 409; mode off; no operation created. Kernel capabilities still 503. No live send. Promote/restore red unpaid."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after owner-scoped refresh deploy
    candidate_or_evidence_refs: [docs/evidence/effects-red-mode-off-refuse-2026-08-16.md]
    landing:
      source_commit: 21b79872
      ci_ref: "https://github.com/choir-hip/go-choir/actions/runs/31925691337"
      deploy_ref: 21b7987292c0cbbd42e8e58ea2059334a6756208
      environment_identity: staging https://choir.news deployed 21b79872 at 2026-08-16T04:27:50Z; retained epoch 269 mode off; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; live 409 is not live proof"
  - id: effects-owner-guest-boot-refresh-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-owner-guest-boot-refresh-2026-08-16.md
    proof_refs: [docs/evidence/effects-owner-guest-boot-refresh-2026-08-16.md, internal/proxy/computer_lifecycle.go, internal/platform/lifecycle_control.go, cmd/choir/main.go]
    rollback_ref: revert this owner-scoped refresh commit
    disposition: "accepted as owner-scoped guest-boot refresh product path — not rematerialize, not global deploy rewrite. Host 0ee3a61e; live start still 503 on constructed freeze 7122f279. Refresh unpaid until this commit deploys. Mode not set. No live send."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after guest mode authority deploy
    candidate_or_evidence_refs: [docs/evidence/effects-owner-guest-boot-refresh-2026-08-16.md]
    landing:
      source_commit: 0ee3a61e
      ci_ref: pending
      deploy_ref: 0ee3a61ed7e5abd6e319fe02bd65d478b7a0ffb6
      environment_identity: staging https://choir.news deployed 0ee3a61e at 2026-08-16T02:56:05Z; retained epoch 268 mode off; constructed freeze 7122f279
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; owner-scoped refresh is not live proof"
  - id: effects-guest-mode-authority-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-guest-mode-authority-2026-08-16.md
    proof_refs: [docs/evidence/effects-guest-mode-authority-2026-08-16.md, internal/autoputer/run.go, internal/agentcore/runtime.go, internal/agentcore/api_self_development_test.go]
    rollback_ref: revert this guest-mode-authority commit
    disposition: "accepted as guest mode-authority wiring — WithSelfDevelopmentControl mounts signed mode reads so start can refuse at mode off. Owner-recovery remains a distinct control. Mode is not set. Outbox unarmed. No live send. Staging live 409 unpaid until deploy onto the retained guest."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after red product-path smoke
    candidate_or_evidence_refs: [docs/evidence/effects-guest-mode-authority-2026-08-16.md]
    landing:
      source_commit: 1a2c8ee6
      ci_ref: pending
      deploy_ref: 4543624b0f1f4f5bb472e4e4bafc7536f279a440
      environment_identity: staging https://choir.news still 4543624b until this commit deploys; retained epoch 268 mode off
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; mode-authority wiring is not live proof"
  - id: effects-red-product-path-smoke-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-red-product-path-smoke-2026-08-16.md
    proof_refs: [docs/evidence/effects-red-product-path-smoke-2026-08-16.md, docs/evidence/effects-product-path-forward-2026-08-16.md]
    rollback_ref: revert this docs-only stamp
    disposition: "accepted as red product-path smoke — staging 4543624b forwards decision to the guest (404 no operation) and keeps genesis 409. Start 503 because WithSelfDevelopmentControl is unmounted. Mode off. Epoch 268. No live send. Promote/restore red unpaid."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after product-path forward deploy
    candidate_or_evidence_refs: [docs/evidence/effects-red-product-path-smoke-2026-08-16.md]
    landing:
      source_commit: 4543624b
      ci_ref: "https://github.com/choir-hip/go-choir/actions/runs/31920331644"
      deploy_ref: 4543624b0f1f4f5bb472e4e4bafc7536f279a440
      environment_identity: staging https://choir.news deployed 4543624b at 2026-08-16T02:02:51Z; retained epoch 268 mode off
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; product-path smoke is not live proof"
  - id: effects-product-path-forward-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-product-path-forward-2026-08-16.md
    proof_refs: [docs/evidence/effects-product-path-forward-2026-08-16.md, internal/proxy/self_development.go, internal/proxy/handlers.go, internal/agentcore/api_self_development.go]
    rollback_ref: revert this product-path-forward commit
    disposition: "accepted as route-map-10 product-path prep — start/decision forward to guest; genesis remains disabled. Live smoke is effects-red-product-path-smoke-2026-08-16. No live send. Owner gates unchanged. Effects remain OFF."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after orange rehearsal
    candidate_or_evidence_refs: [docs/evidence/effects-product-path-forward-2026-08-16.md]
    landing:
      source_commit: 4543624b
      ci_ref: "https://github.com/choir-hip/go-choir/actions/runs/31920331644"
      deploy_ref: 4543624b0f1f4f5bb472e4e4bafc7536f279a440
      environment_identity: staging https://choir.news deployed 4543624b at 2026-08-16T02:02:51Z; retained epoch 268 mode off
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; product-path forward is not live proof"
  - id: effects-rehearsal-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-rehearsal-2026-08-16.md
    proof_refs: [docs/evidence/effects-rehearsal-2026-08-16.md, internal/agentcore/effects_rehearsal_test.go, internal/decisionpolicy/reduce.go, internal/agentcore/self_development_decision_binding.go, internal/trustedoutbox/outbox.go, internal/platform/self_development_modes.go]
    rollback_ref: revert this orange-rehearsal commit
    disposition: "accepted as route-map-9 orange rehearsal — in-process reversible promote+restore consumption and irreversible RecordingProvider dispatch. No live send. Owner gates unchanged. Effects remain OFF. Red/live rehearsal and live proof unpaid."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after supervision-wiring land
    candidate_or_evidence_refs: [docs/evidence/effects-rehearsal-2026-08-16.md]
    landing:
      source_commit: 466c0504
      ci_ref: pending
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583; retained epoch 268
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; orange rehearsal is not completion and not deployed proof"
  - id: effects-supervision-wiring-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-supervision-wiring-2026-08-16.md
    proof_refs: [docs/evidence/effects-supervision-wiring-2026-08-16.md, internal/textureowner/texture_evidence_sources.go, internal/textureowner/texture_revision_metadata.go, internal/textureowner/texture_task_types.go, internal/agentcore/tool_profiles.go, internal/agentcore/self_development_decision_binding.go, internal/platform/self_development_modes.go]
    rollback_ref: revert this supervision-wiring commit
    disposition: "accepted as route-map-8 source confirmation — joinable identities without payload schema change; Texture omits generic update_coagent by design. Owner gates unchanged. Effects remain OFF. Rehearsal unpaid."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after trusted-outbox land
    candidate_or_evidence_refs: [docs/evidence/effects-supervision-wiring-2026-08-16.md]
    landing:
      source_commit: 3141b90b
      ci_ref: pending
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583; retained epoch 268
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; supervision wiring is an execute slice, not completion"
  - id: effects-trusted-outbox-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-trusted-outbox-2026-08-16.md
    proof_refs: [docs/evidence/effects-trusted-outbox-2026-08-16.md, internal/trustedoutbox/outbox.go, internal/decisionpolicy/reduce.go, internal/decisionpolicy/irreversible-email-v1.json, internal/agentcore/self_development_decision_binding.go, internal/platform/self_development_modes.go]
    rollback_ref: revert this trusted-outbox commit
    disposition: "accepted as route-map-7 dispatch with Armed=false — no live send. Owner gates unchanged. Effects remain OFF. Supervision and rehearsal unpaid."
    problem_ref: irreversible-effects-human-gate-drift-2026-08-13
    authorization_ref: this Definition now.next_action after email policy ACCEPT
    candidate_or_evidence_refs: [docs/evidence/effects-trusted-outbox-2026-08-16.md]
    landing:
      source_commit: ede00140
      ci_ref: pending
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583; retained epoch 268
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; trusted-outbox is an execute slice, not completion"
  - id: effects-irreversible-email-v1-policy-review-2026-08-16
    boundary: define
    commit_or_artifact: docs/evidence/effects-irreversible-email-v1-policy-review-2026-08-16.md
    proof_refs: [docs/evidence/effects-irreversible-email-v1-policy-review-2026-08-16.md, docs/evidence/effects-irreversible-email-v1-policy-2026-08-16.md]
    rollback_ref: revert this docs-only review stamp
    disposition: "ACCEPT 3/3 completed panelists (Devin no-verdict). Digests verified. Completeness and rejection bars closed. Next is trusted-outbox dispatch, not a live send."
    problem_ref: irreversible-effects-human-gate-drift-2026-08-13
    authorization_ref: this Definition now.next_action after email policy freeze
    candidate_or_evidence_refs: [docs/evidence/effects-irreversible-email-v1-policy-2026-08-16.md]
    landing:
      source_commit: 20d2ac4c
      ci_ref: pending (Docs Truth Check)
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
  - id: effects-irreversible-email-v1-policy-2026-08-16
    boundary: define
    commit_or_artifact: docs/evidence/effects-irreversible-email-v1-policy-2026-08-16.md
    proof_refs: [docs/evidence/effects-irreversible-email-v1-policy-2026-08-16.md, docs/evidence/effects-decision-policy-schema-2026-08-15.md]
    rollback_ref: revert this docs-only policy freeze
    disposition: "define sub-slice ACCEPT 2026-08-16 (3/3 completed). Digests d83c2154 and 33f5dc44. Not implementation. No outbox wired. Effects remain OFF."
    problem_ref: irreversible-effects-human-gate-drift-2026-08-13
    authorization_ref: this Definition now.next_action after reducer land
    candidate_or_evidence_refs: [docs/evidence/effects-decision-policy-schema-2026-08-15.md]
    landing:
      source_commit: a5f8bdcc
      ci_ref: pending (Docs Truth Check)
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
  - id: effects-decision-policy-reducer-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-decision-policy-reducer-2026-08-16.md
    proof_refs: [docs/evidence/effects-decision-policy-reducer-2026-08-16.md, internal/decisionpolicy/reduce.go, internal/agentcore/self_development_decision_binding.go, internal/platform/self_development_modes.go, internal/agentcore/api_self_development.go]
    rollback_ref: revert this decision-policy reducer commit
    disposition: "accepted as route-map-6 atomic land — QualifiedConsensusReceipt reducer exists; owner gates still present; effects remain OFF. Rehearsal and irreversible email unpaid."
    problem_ref: irreversible-effects-human-gate-drift-2026-08-13
    authorization_ref: this Definition now.next_action after freeze/propose wiring
    candidate_or_evidence_refs: [docs/evidence/effects-decision-policy-reducer-2026-08-16.md]
    landing:
      source_commit: fb89c245
      ci_ref: pending
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583; retained epoch 268
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; reducer is an execute slice, not completion"
  - id: effects-freeze-propose-wiring-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-freeze-propose-wiring-2026-08-16.md
    proof_refs: [docs/evidence/effects-freeze-propose-wiring-2026-08-16.md, internal/agentcore/tool_profiles.go, internal/agentcore/tools_capsule.go, internal/agentcore/cosuper_assignment_tools_overlay.go, internal/agentcore/runtime.go]
    rollback_ref: revert this freeze/propose wiring commit
    disposition: "accepted as route-map-5 wiring — assigned CoSuper can freeze/inspect/verify under capsule binding. Owner gates unchanged. Effects remain OFF. Reducer unpaid."
    problem_ref: not_applicable
    authorization_ref: this Definition now.next_action after reconnection
    candidate_or_evidence_refs: [docs/evidence/effects-freeze-propose-wiring-2026-08-16.md]
    landing:
      source_commit: b7b9f0b1
      ci_ref: pending
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583; retained epoch 268
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; freeze/propose wiring is an execute slice, not completion"
  - id: effects-reconnection-2026-08-16
    boundary: execute
    commit_or_artifact: docs/evidence/effects-reconnection-2026-08-16.md
    proof_refs: [docs/evidence/effects-reconnection-2026-08-16.md, internal/agentcore/super_controller.go, internal/agentcore/tools_worker_update.go, internal/agentcore/tool_profiles.go, internal/agentcore/update_coagent_survivor_contract_test.go, internal/store/store.go, internal/agentcore/self_development_decision_binding.go, internal/platform/self_development_modes.go]
    rollback_ref: revert this reconnection commit
    disposition: "accepted as route-map-4 reconnection — assigned CoSuper holds update_coagent; Super executability is sender authorization; survivor contract replaced not deleted. Owner gates unchanged. Effects remain OFF. Freeze/propose unpaid."
    problem_ref: actor-isolation-stopgap-2026-08-11
    authorization_ref: this Definition now.next_action after policy-bytes ACCEPT
    candidate_or_evidence_refs: [docs/evidence/effects-reconnection-2026-08-16.md]
    landing:
      source_commit: dfcb8ad8
      ci_ref: pending
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583; retained epoch 268
      deployed_acceptance: not_applicable
    registry_conformance_ref: "effects remains entrypoint; reconnection is an execute slice, not completion"
  - id: effects-reversible-selfdev-v1-policy-review-2026-08-15
    boundary: define
    commit_or_artifact: docs/evidence/effects-reversible-selfdev-v1-policy-review-2026-08-15.md
    proof_refs: [docs/evidence/effects-reversible-selfdev-v1-policy-review-2026-08-15.md, docs/evidence/effects-reversible-selfdev-v1-policy-2026-08-15.md]
    rollback_ref: revert this docs-only review stamp
    disposition: "ACCEPT — Gemini, Grok, Sol; Devin no-verdict. Digest verified. Policy is complete enough that implement cannot invent quorum/roster/bounds. Does not authorize deleting the owner gate or arming effects."
    problem_ref: irreversible-effects-human-gate-drift-2026-08-13
    authorization_ref: this Definition now.next_action after policy-bytes freeze
    candidate_or_evidence_refs: [docs/evidence/effects-reversible-selfdev-v1-policy-2026-08-15.md]
    landing:
      source_commit: 8d4107b0
      ci_ref: pending (Docs Truth Check)
      deploy_ref: 4ac90583e389e3334efa57ce204d6df3235a68f1
      environment_identity: staging https://choir.news deployed 4ac90583
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
  - id: roadmap-consensus-2026-08-11
    boundary: define
    commit_or_artifact: 2379616d
    proof_refs: [.agentic-consensus/self-dev-roadmap/divergent/manifest.tsv, .agentic-consensus/self-dev-roadmap/lateral/manifest.tsv, .agentic-consensus/self-dev-roadmap/convergent/manifest.tsv]
    rollback_ref: revert 2379616d
    disposition: accepted (unanimous shape; E1/E2 resolved E2-for-vision, E1-as-rehearsal-gate)
    problem_ref: not_applicable
    authorization_ref: not_applicable
    candidate_or_evidence_refs: [docs/choir-self-development-roadmap-2026-08-11.md]
    landing:
      source_commit: 2379616d
      ci_ref: 31450288548 (Docs Truth Check success)
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "registered 2026-08-11 across docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml (entrypoint true; supersedes choir-continuous-texture-supervision-2026-08-07)"
  - id: content-axis-pivot-2026-08-11
    boundary: define
    commit_or_artifact: 56b78f83
    proof_refs: [internal/capsule/transaction/builder.go:95-111, internal/updater/updater.go:145-238, internal/modelpolicy/model_policy.go:89-118, internal/agentcore/tools_capsule.go:251,319,713]
    rollback_ref: revert the docs-only pivot commit
    disposition: accepted (owner-directed; D1 rejected, D3 adopted, capsule authorship required)
    problem_ref: [model-policy-effect-target-mismatch-2026-08-11, model-policy-is-system-owned-2026-08-11]
    authorization_ref: owner direction 2026-08-11
    candidate_or_evidence_refs: [docs/definitions/choir-supervised-self-development-effects-2026-08-11.md]
    landing:
      source_commit: pending
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "definition_id unchanged; no registry flip required — this is a content-axis revision to an already-registered active Definition"
  - id: dolt-revert-consistency-consensus-2026-08-11
    boundary: define
    commit_or_artifact: pending
    proof_refs: [.agentic-consensus/dolt-revert-consistency-2026-08-11/divergent/, .agentic-consensus/dolt-revert-consistency-2026-08-11/convergent/]
    rollback_ref: revert the docs-only revert-design commit
    disposition: "accepted — panel retired the atomic-vs-ordered framing (divergent 7/8, convergent 8/8). Adopted: scope restore to VM-local plus release with platform and cycle out; address by event head; acceptance-fenced verify; rematerialization preferred over pin checkout. Dissent retained: ProjectionMaterializer is non-runtime today, so pin checkout stands as an implementation contingency."
    problem_ref: rollback-is-code-only-2026-08-11
    authorization_ref: owner direction 2026-08-11 (panel commissioned to settle the multi-Dolt question)
    candidate_or_evidence_refs: [docs/definitions/choir-supervised-self-development-effects-2026-08-11.md]
    landing:
      source_commit: pending
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "no registry flip; corrects a scope error in this Definition that would have swept the shared platform store into a user-computer revert"
  - id: one-ask-owner-wide-key-acceptance-2026-08-11
    boundary: acceptance
    commit_or_artifact: "914f7a5d976a (staging frontend+proxy, deploy 2026-08-11T18:11:01Z); owning code 367265f8 + a11aca50"
    proof_refs: [Settings > API keys one-click flow on https://choir.news — one click minted an owner-wide admin key (secret shown once, list row unbound); headless child mint ak_ac534dd1-a779-4284-8823-494080d5fee6 via choir api-key create -scopes read:base -api-key-file; headless revoke of that child via choir api-key revoke; revocation confirmed 2026-08-11]
    rollback_ref: revoke via Settings > Revoke or choir api-key revoke (exercised)
    disposition: accepted (secret shown once; subsequent issuance/revocation fully headless on staging — the at-most-one-ask requirement and Mission 0's owner-bearer residual are disposed)
    problem_ref: not_applicable
    authorization_ref: owner-wide one-click bootstrap design (a11aca50)
    candidate_or_evidence_refs: [docs/definitions/choir-supervised-self-development-effects-2026-08-11.md start.observed_artifact owner-wide-key claim]
    landing:
      source_commit: 914f7a5d976a
      ci_ref: not_applicable (observed post-deploy)
      deploy_ref: "staging https://choir.news, deploy time 2026-08-11T18:11:01Z"
      environment_identity: "frontend 914f7a5d976a; proxy 914f7a5d976a; proxy status ok"
      deployed_acceptance: "one-click mint -> headless create -> headless revoke, all against staging"
    registry_conformance_ref: "no registry flip; feedforward evidence for the rehearsal slice"

  - id: pre-mission-haunted-authority-cutover-2026-08-11
    boundary: define
    commit_or_artifact: pending
    proof_refs: [.agentic-consensus/mission-success-lateral-2026-08-11/lateral/, docs/choir-self-development-roadmap-2026-08-11.md, docs/ACTIVE.md, docs/computer-ontology.md, docs/agent-product-doctrine.md, docs/choir-doctrine.md, docs/memo-persistent-rlm-actors-2026-08-09.md, AGENTS.md, docs/doc-authority-manifest.yaml]
    rollback_ref: revert the docs-only pre-mission cutover commit
    disposition: "accepted — demoted roadmap as live schedule; restored-set and ComputerVersion language in ontology/doctrine; model-policy ≠ self-dev axis; RLM Phase-1 re-derive note; AGENTS restore-vs-deploy + completion_cutover Landing Loop note; finish.completion_cutover + not_done_when for post-success packet; panel repair round: Definition YAML subset-safe quoting, mission-graph note rewrite (replay probe; Mission 0 disposed), ACTIVE Mission 0/Invocation demotion + CTS fbc7ff5a sequence demoted historical, SEM-03 transitional restore/standing-rule wording; re-review panel APPROVE_START_GOAL (land docs commit before /goal)"
    problem_ref: not_applicable
    authorization_ref: owner direction 2026-08-11 (pre-mission changes from lateral panel; completion packet deferred)
    candidate_or_evidence_refs: [docs/definitions/choir-supervised-self-development-effects-2026-08-11.md, docs/definitions/choir-supervised-self-development-effects-2026-08-11-supplement.md, docs/mission-graph.yaml, docs/semantic-registry.md]
    landing:
      source_commit: pending
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "roadmap registered as historical migration receipt in docs/doc-authority-manifest.yaml; Definition remains sole entrypoint; SEM-03 transitional pending full cutover promote"

  - id: replay-completeness-probe-2026-08-12
    boundary: acceptance
    commit_or_artifact: docs/evidence/choir-sandbox-autoputer-replay-completeness-2026-08-12.json
    proof_refs: ["choir computer replay-completeness --computer computer-03335285269bdba4f94377e56879f9e6 --timeout 10m (exit 0)", "captured_at 2026-08-12T05:36:44.159743453Z", "result not_equivalent with 26 deterministic DoltStateExtractor differences", "live_head null and replay_head null", "probe_digest 67ec50ed1526659eb04e7d1be6cabc02d33e6b1f16559d1e2e0036f4f3785af1"]
    rollback_ref: "Delete the evidence projection only if the effects mission is abandoned; the probe appended no event and mutated no live state."
    disposition: "accepted as the required pre-drop observation; it blocks clean rematerialization and checkpoint design until every difference is classified"
    problem_ref: replay-completeness-non-equivalence-2026-08-12
    authorization_ref: "owner answer 2026-08-12: add read-only product API/CLI replay probe"
    candidate_or_evidence_refs: [docs/evidence/choir-sandbox-autoputer-replay-completeness-2026-08-12.json]
    landing:
      source_commit: ba27a3e8
      ci_ref: "31565423783 (success)"
      deploy_ref: "Deploy to Staging (Node B) job 94018717399 (success)"
      environment_identity: "https://choir.news/health deployed_commit ba27a3e8ed1815dff9853bf96741b4333cf7c1f4"
      deployed_acceptance: "pre-drop replay route and stable identity proof passed; exact non-equivalence retained"
    registry_conformance_ref: "effects Definition remains the active entrypoint; rename Definition accepted before effects resumes"
  - id: replay-probe-credential-renewal-refused-2026-08-12
    boundary: acceptance
    commit_or_artifact: docs/definitions/choir-supervised-self-development-effects-2026-08-11.md
    proof_refs: ["CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=10m go run ./cmd/choir computer replay-completeness --computer computer-03335285269bdba4f94377e56879f9e6 at 2026-08-12T18:46Z (HTTP 500: guest credential renewal refused)", "https://choir.news/health deployed_commit d2ab2d2d2184a7918f1a6ff73b6bd29638f85b5c", "choir computer status --computer computer-03335285269bdba4f94377e56879f9e6 (active, realization_epoch 203)"]
    rollback_ref: "Revert the documentation-only finding; no deployed state or event chain was mutated by the failed read-only probe."
    disposition: "blocked: the product-path probe did not produce a replay diff because guest capability renewal was refused during durable-chain reconstruction"
    problem_ref: replay-probe-credential-renewal-refused-2026-08-12
    authorization_ref: "effects Definition acceptance action 1; standing question 9 product API/CLI boundary"
    candidate_or_evidence_refs: [docs/definitions/choir-supervised-self-development-effects-2026-08-11.md]
    landing:
      source_commit: pending
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: "staging https://choir.news deployed_commit d2ab2d2d2184a7918f1a6ff73b6bd29638f85b5c"
      deployed_acceptance: "failed before replay diff; retained computer remains active and unchanged"
    registry_conformance_ref: "effects Definition remains the active entrypoint; no registry topology changed"
  - id: replay-probe-rerun-after-credential-refresh-2026-08-12
    boundary: acceptance
    commit_or_artifact: docs/evidence/choir-supervised-self-development-replay-completeness-2026-08-12.json
    proof_refs: ["owner-scoped restart receipt 019ff74d-5c2f-7693-951e-b3acb8e0fa9e, idempotency_key replay-credential-renewal-20260812-1848, prior_realization_epoch 203 -> resulting_realization_epoch 204", "CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=10m go run ./cmd/choir computer replay-completeness --computer computer-03335285269bdba4f94377e56879f9e6 at 2026-08-12T18:48:42Z (exit 0)", "result not_equivalent with 26 deterministic DoltStateExtractor differences", "live_head null and replay_head null", "probe_digest a6e61857598fb1761ed58e5b27c527c12ebaf19e850db4ceae9743d76a0a12f0", "https://choir.news/health deployed_commit d2ab2d2d2184a7918f1a6ff73b6bd29638f85b5c"]
    rollback_ref: "The read-only probe appended no event and mutated no live Dolt state; the lifecycle restart is forward and separately receipt-bound."
    disposition: "accepted as the required pre-drop observation; exact non-equivalence remains and blocks clean rematerialization/checkpoint design until every difference is classified"
    problem_ref: replay-completeness-non-equivalence-2026-08-12
    authorization_ref: "effects Definition acceptance action 1; owner-scoped lifecycle restart used only to refresh the retained guest credential after the documented refusal"
    candidate_or_evidence_refs: [docs/evidence/choir-supervised-self-development-replay-completeness-2026-08-12.json, docs/evidence/choir-sandbox-autoputer-replay-completeness-2026-08-12.json]
    landing:
      source_commit: d2ab2d2d2184a7918f1a6ff73b6bd29638f85b5c
      ci_ref: "31626424709 (runtime gates passed; unrelated required SBOM candidate failed)"
      deploy_ref: "Deploy to Staging (Node B) job 94218666218 (success)"
      environment_identity: "https://choir.news/health deployed_commit d2ab2d2d2184a7918f1a6ff73b6bd29638f85b5c"
      deployed_acceptance: "owner-scoped restart then read-only replay probe completed; exact 26-difference report retained"
    registry_conformance_ref: "effects Definition remains the active entrypoint; evidence artifact is referenced here and in the authority manifest"
  - id: replay-difference-classification-2026-08-12
    boundary: define
    commit_or_artifact: docs/evidence/choir-supervised-self-development-replay-difference-classes-2026-08-12.md
    proof_refs: ["docs/evidence/choir-supervised-self-development-replay-completeness-2026-08-12.json", "internal/agentcore/replay_eligibility.go replayAirworthinessEntries", "convergent panel .agentic-consensus/replay-resolve-20260812 (6 APPROVE WITH CONDITIONS; deepseek timed out; codex excluded)", "owner direction 2026-08-12: pre-launch, no backwards compatibility"]
    rollback_ref: "Revert the documentation-only classification; retained 26-diff artifacts are unchanged."
    disposition: "accepted as the required classification of the 26 differences; diagnostic equivalence still requires a product-path workspace replacement; eligibility and restore remain fail-closed"
    problem_ref: replay-completeness-non-equivalence-2026-08-12
    authorization_ref: "effects Definition next_action classify-before-checkpoint; owner pre-launch no-compat direction"
    candidate_or_evidence_refs: [docs/evidence/choir-supervised-self-development-replay-difference-classes-2026-08-12.md, docs/evidence/choir-supervised-self-development-replay-completeness-2026-08-12.json]
    landing:
      source_commit: pending
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: "staging https://choir.news deployed_commit d2ab2d2d2184a7918f1a6ff73b6bd29638f85b5c"
      deployed_acceptance: "classification only; 26 differences remain on the retained workspace"
    registry_conformance_ref: "effects Definition remains the active entrypoint; classification evidence is referenced here and in the authority manifest"
  - id: chain-bootstrap-eligible-2026-08-13
    boundary: execute
    commit_or_artifact: docs/evidence/choir-supervised-self-development-replay-completeness-post-bootstrap-2026-08-13.json
    proof_refs: ["choir computer bootstrap-chain --computer computer-03335285269bdba4f94377e56879f9e6 at 2026-08-13T00:59Z (sequence 1, canonical_event_head a3cf16d0d1dbb46e4ebd5841af5007575fb74184d54c2e6fa26f856769b92b44, code_ref git:5d132f66360899d634befd69ba537ab918005c24, appended_event true, published_checkpoint false, wrote_selfdev_operation false)", "restart receipt 019ff8a1-4894-747b-ae86-020a87824416 (boot new guest image, realization_epoch 208)", "choir computer replay-completeness at 2026-08-13T01:00Z: equivalent, zero differences, live_head==replay_head (a3cf16d0), eligible=true, probe_digest 977b68106ed79d19fa7ef4c011f6aa57a79a80ec4fb74f51789baa98e8069f90", "https://choir.news/health deployed_commit 5d132f66360899d634befd69ba537ab918005c24", "CI run 31654611696 success"]
    rollback_ref: "git revert 5d132f66; quarantined pre-cutover workspace retained in-guest; bootstrap append is a forward event, not an in-place mutation"
    disposition: "accepted as replay eligibility (projection-reconstruction equivalence) on a sequence-1 canonical head; not restore license"
    problem_ref: chain-bootstrap-null-head-2026-08-12
    authorization_ref: "owner direction pre-launch no-backcompat; convergent panel .agentic-consensus/replay-chain-bootstrap-20260812 (6/6 APPROVE WITH CONDITIONS)"
    candidate_or_evidence_refs: [docs/evidence/choir-supervised-self-development-replay-completeness-post-bootstrap-2026-08-13.json, docs/evidence/choir-supervised-self-development-chain-bootstrap-design-2026-08-12.md]
    landing:
      source_commit: 5d132f66
      ci_ref: "31654611696 (success)"
      deploy_ref: "Deploy to Staging (Node B) success; deployed_at 2026-08-13T00:58:00Z"
      environment_identity: "https://choir.news/health deployed_commit 5d132f66360899d634befd69ba537ab918005c24, vmctl_status ok"
      deployed_acceptance: "bootstrap-chain + restart + replay-completeness; equivalent, non-nil equal heads, eligible=true, no checkpoint/Operation/effect"
    registry_conformance_ref: "effects Definition remains the active entrypoint; post-bootstrap evidence artifact is referenced here and in the authority manifest"
  - id: workspace-replace-cutover-2026-08-12
    boundary: execute
    commit_or_artifact: docs/evidence/choir-supervised-self-development-replay-completeness-post-cutover-2026-08-12.json
    proof_refs: ["choir computer replace-workspace --computer computer-03335285269bdba4f94377e56879f9e6 at 2026-08-12T21:59:35Z (quarantine /mnt/persistent/workspace-replaced-20260812T215935.028398691Z, appended_event false, published_checkpoint false, store_closed true)", "restart receipt 019ff7fc-a0e4-7b78-972b-78172447d9fb (boot new guest image, realization_epoch 205)", "restart receipt 019ff7fd-5992-75c4-badc-0562bd4e156b (reopen fresh store, realization_epoch 207)", "choir computer replay-completeness at 2026-08-12T22:00:27Z: status equivalent, 82 live and 82 replay observations, zero differences, live_head null, replay_head null, probe_digest 83d31d2e0be42a8e3f508471e22f100f9c0d02fcb59faa79730ec061b947d0bc", "https://choir.news/health deployed_commit bf03286f48c21971212db74c5b6d73a49d65a1d3", "CI run 31635577258 success after Node B disk reclaim"]
    rollback_ref: "Quarantined pre-cutover workspace retained in-guest at /mnt/persistent/workspace-replaced-20260812T215935.028398691Z; platform rollback is git revert of bf03286f"
    disposition: "accepted as diagnostic equivalence on a null-head workspace; replay eligibility, checkpoint, restore, and effects remain fail-closed because heads are null"
    problem_ref: replay-completeness-non-equivalence-2026-08-12
    authorization_ref: "owner direction 2026-08-12: pre-launch, no backwards compatibility; convergent panel .agentic-consensus/replay-resolve-20260812"
    candidate_or_evidence_refs: [docs/evidence/choir-supervised-self-development-replay-completeness-post-cutover-2026-08-12.json, docs/evidence/choir-supervised-self-development-replay-difference-classes-2026-08-12.md]
    landing:
      source_commit: bf03286f
      ci_ref: "31635577258 (success; SBOM rerun after transient network drop; deploy rerun after Node B disk reclaim)"
      deploy_ref: "Deploy to Staging (Node B) success; deployed_at 2026-08-12T21:57:18Z"
      environment_identity: "https://choir.news/health deployed_commit bf03286f48c21971212db74c5b6d73a49d65a1d3, vmctl_status ok"
      deployed_acceptance: "replace-workspace + restart executed on the retained computer; post-cutover probe equivalent with zero differences and null heads"
    registry_conformance_ref: "effects Definition remains the active entrypoint; post-cutover evidence artifact is referenced here and in the authority manifest"

view:
  path: none
  generator: none
---

# Supervised Self-Development with Effects — Successor Definition

Supersedes `choir-continuous-texture-supervision-2026-08-07`. Reasoning, retired
approaches, research findings, and panel records are in the
[supplement](choir-supervised-self-development-effects-2026-08-11-supplement.md).
Read that only if you need the why; everything needed to execute is here.

## What is being proved

That the computer can authorize its own reversible and irreversible effects
through effect-specific multiagent consensus, retain complete consequence
receipts, and restore reversible state. The proof includes its own source change
and one exact external send; neither requires a per-candidate human decision
under the selected acceptance policies.

## Rules of the autonomy window

**Policy-governed consensus authorizes effects.** CoSuper freezes and proposes.
Before outputs exist, the selected policy binds the exact subject, eligible
seats and independence domains, quorum, dissent and failure semantics, evidence
requirements, expiry, actuator, and consequence receipts. Participants cannot
shrink or rewrite that policy after seeing results.

**The owner governs the constitutional envelope, not every candidate.** The
owner can establish and revoke policies. A human may be a required, optional,
or absent consensus participant. Missing a policy-required seat, quorum, or
dissent disposition fails closed.

**Reversibility is recovery, not authority.** Reversible effects gain
acceptance-fenced restore. Irreversible effects such as send, publish, pay, or a
third-party write remain admissible under a stronger policy with exact subject
binding, qualified independent consensus, durable provider and consequence
receipts, and compensation or new forward action for correction. They are not
categorically routed to a human.

**Restore is scoped to the user computer.** Event-derived VM-local projection
plus computer-surface frontend, owned by choir-tape-recovery-2026-08-13 and
paid 2026-08-15 (`serving_join`, `capability_renewal_pass`). The shared
platform store and cycle state are out; restoring them would rewind other
computers. Do not independently green restore in this Definition.

**Revert never erases history.** Returning to an earlier point is a forward
transaction that restores state. The event chain keeps the record of what was
done and undone.

**Checkpoints bind the event head, code, artifact program, and a VM-local content
witness.** The VM-local Dolt HEAD is an audit receipt joined to that head, never
restore authority. A checkpoint that cannot restore state is not a checkpoint;
one that cannot be rebuilt from the tape must fail closed rather than be
recorded.

## Restore procedure

Consumed via choir-tape-recovery-2026-08-13 receipts. Do not rematerialize or
independently green this path here. Promotion of an effect excursion uses this
procedure; it does not re-prove it.

1. Resolve the checkpoint; refuse if incomplete.
2. Quiesce writers / enter the materialization window.
3. Append a forward restore intent naming the target event head.
4. Restage the release and rebuild VM-local state through that head.
5. Re-run `DoltStateExtractor`; require an exact content-hash match.
6. Flip visibility and route only on green.
7. On failure keep the prior realization. Partial never greens.

## The candidate

A CoSuper capsule authors solitaire: rules engine, headless play API sufficient
for automated play without a browser, durable persistence, and play history. It
writes the source, runs build and tests through `capsule_exec`, and freezes the
classified diff into a bundle whose five required refs all bind that capsule's
own execution receipts.

API-only for this candidate. This Definition does not ship UI in the solitaire
candidate. Computer-surface frontend is per-computer (`C15`/`I25`);
choir-tape-recovery-2026-08-13 paid `serving_join` 2026-08-15 (guest-static hop
after vmctl resolve). Thin platform shell remains OUT of restore.

Schema changes are additive only — new tables via `CREATE TABLE IF NOT EXISTS`,
never `ALTER` or `DROP` of existing tables.

The candidate touches no protected surface, and solitaire code never writes
Texture. This is distinct from the mission's Texture writes, which are required:
Texture supervises the work, and the candidate is its subject.

## The correction spine

Version A ships with a pre-declared rule defect its own tests do not catch — a
foundation accepting a same-color build, or win detection firing on an incomplete
tableau. A promotes, restarts healthy, and is played. Admissible evidence then
falsifies it: a headless play sequence drives the engine into the illegal state A
accepted. B corrects the rule and supersedes A as a forward transition; after
restart the same sequence is refused.

The defect must be genuine, pre-declared in a Define receipt before A is
proposed, and invisible to A's own verification. If A's tests catch it, the
bundle is refused and the mission has proven the gate rather than the correction
spine.

Supersession is carried by ordering plus B's proposal event, because the selfdev
state machine is linear and the decision verifier admits only one input artifact
ref and one verifier ref.

The mission closes by winding the excursion back: return to a checkpoint taken
before A promoted, and show the solitaire API absent, its rows absent, state
hashes equal to the recorded checkpoint, and the event chain still carrying the
history of what was undone.

## Supervision

Supervision rides the existing upward path: capsule → CoSuper → `update_coagent`
→ Super → Texture revision. Build no observation subsystem.

Each revision carries the operation, bundle digest, receipt, and head in
**revision metadata and typed source citations — never in the prose**. Prose is
for the human: what happened, what it means, what is being decided. A revision
that recites digests at the owner has made supervision worse.

Revisions are committed while work is open, not assembled into a report after it
settles. Because promotion is autonomous, this is how the owner stays in the loop
at all.

## Route map

1. **Replay completeness probe (green, historical — consumed).** Pre-drop probe returned `not_equivalent` with 26 differences. Classification, workspace-replace, chain bootstrap, and choir-tape-recovery-2026-08-13 paid the restore substrate. Do not rematerialize. Do not treat this item as a live "do first."
2. **Checkpoint completeness (red, owned by choir-tape-recovery-2026-08-13).**
   Consumed here; do not implement or independently green. Bind target event head,
   CodeRef, ArtifactProgramRef, VM-local content witness, and frontend identity.
3. **Revert build and verification (red, owned by choir-tape-recovery-2026-08-13).**
   Consumed here; do not implement or independently green. Tape-recovery proves
   destructive rematerialization, scope refusal, and whole-computer restore.
4. **Reconnection (red, implemented 2026-08-16).** Assigned CoSuper holds
   `update_coagent`; Super executability is Texture `Direction=control` sender
   authorization; survivor contract replaced, not deleted. Keep the privilege
   property — the RLM rebase may rewrite this layer without weakening it.
5. **Freeze/propose wiring (red, implemented 2026-08-16).** Assigned CoSuper
   holds `commit_transaction`, `inspect_self_development_bundle`, and
   `record_self_development_verification` under its capsule binding. Host mutation,
   materialize, checkpoint, route, and owner-decision tools remain absent.
6. **Decision-policy authority (red, implemented 2026-08-16).** Consensus
   reduction produces `QualifiedConsensusReceipt`; canonical decision binding
   accepts that receipt join *or* `external-owner:`. Mode `qualified_consensus`
   binds operation/bundle/heads/commitments plus `policy_digest` and
   `consensus_receipt_digest`. Owner gates remain until deployed acceptance.
   Fail-closed refuse matrix is unit-tested. Effects remain OFF.
7. **Irreversible effect path (red, implemented 2026-08-16, no live send).**
   Policy bytes ACCEPT. Trusted outbox records intent before provider, exact-subject
   idempotency, revocation-before-dispatch, and consequence receipts. Reversible
   policy refuses the subject; irreversible-email-v1 authorizes without a human
   seat; human-required-v1 refuses when that seat is absent. `Armed=false`.
   Orange rehearsal landed 2026-08-16; red/live still unpaid.
8. **Supervision wiring (green, implemented 2026-08-16, source confirmation).**
   Joinable identities ride existing `packet.sources` typed URIs into Texture
   revision metadata and typed citations with no payload schema change. Texture's
   production registry omits generic `update_coagent` by design (CTS-safe);
   Super holds `update_coagent` plus `report_to_texture`; assigned CoSuper holds
   `update_coagent`. Orange rehearsal landed; red/live unpaid.
9. **Rehearsal (orange implemented 2026-08-16; red unpaid).** In-process:
   reversible propose → qualified consensus → promote → consume tape-recovery
   restore → verify; irreversible propose → stronger qualified consensus →
   RecordingProvider dispatch → consequence receipt → crash-window correction.
   No live send. Restore-eligible pre-A OwnerRecovery checkpoint 663540be
   at sequence 32. Texture Super join paid live; Super c4cd7200 completed and bound CoSuper assignment-c60a8912; capsule PATH/overlay paid on a1f3d2cf; boot reconcile now aborts on that completed-run leftover; operation still executing.
10. **Live proof (red).** Capsule authors A → consensus authorizes → promotes →
   played → falsified → B supersedes → restart proves B → total restore; then
   execute and receipt the exact acceptance email under its separate policy.

Out of scope: RLM/Yaegi authoring, model-policy content, the web UI, production,
and the shared platform store.

## Completion cutover (after deployed acceptance)

The trajectory proof is not enough. Before `goal.complete`, run every item under
`finish.completion_cutover` in the frontmatter: promote earned doctrine, cut
lexicon, archive haunted roadmap/registry edges, replace survivor/detector pins,
schedule or ship owner policy arm/revoke surfaces, consume tape-recovery
owner-restore and serving-join receipts, land restore-aware ops
identity, rewrite successor preconditions (RLM / Wire /
in-choir), and name residual realism axes (schema, backups/copies, tape vs
erasure, owner verbs). Skipping that packet is a false complete.
