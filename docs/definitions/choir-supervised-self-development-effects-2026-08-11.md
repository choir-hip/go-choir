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
        action: "Promote earned invariants into choir-doctrine / computer-ontology / agent-product-doctrine: effect-specific consensus policy is the autonomy boundary; checkpoint = event head + CodeRef + ArtifactProgramRef + VM-local content witness + frontend identity; restore is acceptance-fenced and scoped; irreversible effects require stronger policy and consequence receipts, not a categorical human gate; effects OFF is pre-gate not destination. Platform/cycle remain OUT of restore. Computer-surface frontend is IN by C15/I25; the host-global SPA is current non-conformance, closed by choir-tape-recovery-2026-08-13, not by treating UI as platform software."
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
        action: "Decided 2026-08-13: computer-surface frontend is per-computer, not platform control-plane (docs/memo-per-computer-frontend-2026-08-13.md; doctrine C15/I25). The serving envelope is owned by choir-tape-recovery-2026-08-13 and satisfied-by its serving-join receipt. This Definition does not ship UI in the solitaire candidate; Caddy still serves host frontend-current until that receipt. Thin platform shell (TLS, auth, picker chrome) remains OUT of restore."
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
  slice: "The staging readiness incident is resolved. Both candidate realizations are active/ready on db265d1e after three substrate fixes. Checkpoint completeness, rematerialization, and whole-computer restore are now owned by choir-tape-recovery-2026-08-13 and satisfied-by its receipts. This Definition owns decision-policy and effect promotion on top of that restore substrate."
  question: "Does the decision-policy envelope authorize reversible and irreversible effects on top of a proven whole-computer restore substrate?"

  reconciliation:
    observed_at: 2026-08-13T14:10:00Z
    source_ref: main@db265d1e32e73ab4c51914332eaf6fb55f62a09c
    deploy_identity: "staging deployed db265d1e32e73ab4c51914332eaf6fb55f62a09c; retained computer computer-03335285269bdba4f94377e56879f9e6 and platform computer computer-4c20ff4a21a021c4306d8c783be0037d both active/ready; legacy platform primary vm-universal-wire-platform discarded"
    authority_identities: [docs/choir-vision.md, docs/choir-doctrine.md, docs/standing-questions.md, docs/computer-ontology.md, docs/agent-product-doctrine.md, AGENTS.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: 2026-08-13 read-only git status before incident documentation (clean at ca138dff)
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
  evidence_refs: [docs/choir-self-development-roadmap-2026-08-11.md, docs/choir-crashed-prime-session-review-2026-08-09.md, docs/memo-persistent-rlm-actors-2026-08-09.md, docs/memo-live-retrospective-evals-2026-08-09.md]
  blocker_or_risk: "The kill-loop slice is closed. Checkpoint completeness and restore are owned by choir-tape-recovery-2026-08-13; this Definition must not independently green its restore legs. Decision-policy and effect promotion await the tape-recovery restore proof. Problem receipt: docs/problems/candidate-realization-readiness-kill-loop-2026-08-13.md."
  next_action: "Await the tape-recovery Definition's whole-computer restore proof, then design decision-policy and effect promotion on that substrate."

receipts:
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

**Restore is scoped to the user computer.** VM-local state and the release
pointer. The shared platform store and cycle state are out; restoring them would
rewind other computers.

**Revert never erases history.** Returning to an earlier point is a forward
transaction that restores state. The event chain keeps the record of what was
done and undone.

**Checkpoints bind the event head, code, artifact program, and a VM-local content
witness.** The VM-local Dolt HEAD is an audit receipt joined to that head, never
restore authority. A checkpoint that cannot restore state is not a checkpoint;
one that cannot be rebuilt from the tape must fail closed rather than be
recorded.

## Restore procedure

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

API-only for this candidate. Staging still serves the web frontend from the
host, outside the release the updater controls, so a UI change would land where
the browser never reads. That is serving-topology constraint, not product
ontology: computer-surface frontend is per-computer (`C15`/`I25`);
choir-tape-recovery-2026-08-13 owns the serving join.

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

1. **Replay completeness probe (green, do first — complete with non-equivalence).** The deployed pre-drop probe returned a deterministic `not_equivalent` result with 26 differences and both event heads null. Classify every difference before checkpoint design; clean rematerialization is not licensed until behavior-bearing writes are event- or receipt-derived, and checkpoint creation must fail closed for anything that is not reproducible.
2. **Checkpoint completeness (red, owned by choir-tape-recovery-2026-08-13).**
   Consumed here; do not implement or independently green. Bind target event head,
   CodeRef, ArtifactProgramRef, VM-local content witness, and frontend identity.
3. **Revert build and verification (red, owned by choir-tape-recovery-2026-08-13).**
   Consumed here; do not implement or independently green. Tape-recovery proves
   destructive rematerialization, scope refusal, and whole-computer restore.
4. **Reconnection (red).** Give CoSuper `update_coagent`; move Super's
   executability decision from `packet.kind` to sender authorization; replace the
   survivor contract with the stronger assertion. Keep minimal — the RLM rebase
   rewrites this layer.
5. **Freeze/propose wiring (red).** `commit_transaction`,
   `inspect_self_development_bundle`, and `record_self_development_verification`
   exist but have no production call site, so no live actor can freeze or propose.
   Wire them onto the assigned CoSuper under its capsule binding, preserving the
   capsule-local authority boundary for everything else. Without this step the
   live proof cannot start.
6. **Decision-policy authority (red).** Replace `external-owner:` as the only
   accepted decision authority with versioned effect policies and typed
   consensus receipts. Bind exact subject, policy version, frozen eligible seats
   and independence domains, quorum, abstention/timeout/recusal/replacement,
   dissent disposition, evidence, expiry, actuator, and consequence-receipt
   contract. Prove fail-closed behavior for every missing or stale binding.
7. **Irreversible effect path (red).** Add a stronger policy and trusted outbox
   for one exact email to an owner-controlled acceptance inbox. Prove that the
   reversible policy refuses it; the irreversible policy can authorize it
   without a human seat; delivery and later correction remain durably receipted;
   and a separate human-required policy refuses when that seat is absent.
8. **Supervision wiring (green).** Confirm the upward packet carries joinable
   identities and that Texture's production registry has `update_coagent`.
9. **Rehearsal (orange→red).** Reversible change: propose → qualified consensus
   → promote → write state → total restore → verify. Irreversible send: propose
   → stronger qualified consensus → outbox/provider delivery → consequence
   receipt → correction receipt. The live run is gated on both paths passing.
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
