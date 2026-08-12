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
      paths_or_digest: [docs/definitions/choir-supervised-self-development-effects-2026-08-11.md, docs/definitions/choir-supervised-self-development-effects-2026-08-11-supplement.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
      recovery: revert the docs-only commit
  candidates:
    - id: none
  observed_artifact:
    - claim: "Checkpoints bind no state. ComputerVersion is CodeRef plus ArtifactProgramRef only; the Dolt state extractor is read-only and ProjectionMaterializer is non-runtime; rollback re-applies a prior release while rows written under it remain. Three stores call DOLT_COMMIT; nothing in production calls checkout, branch, or reset, and those commits carry no event-head binding."
      evidence_ref: internal/computerversion/types.go:40-48; internal/computerversion/dolt_state_extractor.go:27-83; internal/computerversion/projection_materializer.go:9-22; internal/agentcore/self_development_materializer.go:224-273; internal/platform/store.go:481-499; internal/store/store.go:119; internal/cycle/storage.go:56
    - claim: "Per-candidate owner approval is enforced in code: the decision binding requires an external-owner: authority prefix, and accept_once requires exact operation/bundle/heads/pending-transition/commitments plus a future canonical UTC expiry. Relaxing these is in scope."
      evidence_ref: internal/agentcore/self_development_decision_binding.go:30-33,45,53; internal/platform/self_development_modes.go:226-242
    - claim: "The frozen CapsuleEffectBundle refuses any bundle lacking SourceTreeRef, a capsule-exec BuildRecipeRef, RuntimeArtifactRef, TestReceipts, and DependencyToolchainRefs, so every required ref must bind a real capsule execution receipt."
      evidence_ref: internal/capsule/transaction/builder.go:95-111
    - claim: "Freeze/propose/verify tools have no production call site; an assigned CoSuper holds exactly five capsule-local tools and no upward channel. CoSuper lacks update_coagent, pinned by TestSurvivorContract_GenericCoSuperCannotAuthorPersistentSuperPackets, because Super reads executability from the model-written packet.kind."
      evidence_ref: internal/agentcore/tool_profiles.go:309-315,386; internal/agentcore/tools_capsule.go:61-102; internal/agentcore/super_controller.go:784-796; internal/agentcore/update_coagent_survivor_contract_test.go:193-199
    - claim: "The capsule can build: lower layer is the guest root, platform source is snapshotted by git commit, and go/gcc/git/make/nodejs/pkg-config/icu are on PATH with persistent Go caches and Dolt ICU CGO flags set."
      evidence_ref: nix/autoputer-vm.nix:675-700,717-745,777-798; internal/autoputer/capsule_executor_linux.go:14-56
    - claim: "Staging serves the web frontend from the host (Caddy on Node B, /var/www/go-choir/frontend-current), outside the updater-controlled release. The candidate is API-only."
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
      problem: "Per-candidate owner approval is encoded in the decision binding and accept_once, but it guards the reversible side where reversibility is the better guard, and does not guard the irreversible side at all."
      evidence_ref: see supplement section 2
      consequence: "Auto-promotion inside a reversible envelope replaces per-candidate approval; decisions are reserved for effects leaving that envelope."
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
      consequence: "A clean rematerialization is not licensed. Classify every difference as behavior-bearing event-derived state, pinned-receipt state, or retired/legacy residue; make behavior-bearing writes event-derived or fail checkpoint creation closed before designing restore."
    - id: replay-probe-credential-renewal-refused-2026-08-12
      problem: "After the replay gate repair deployed at d2ab2d2d, the owner-scoped product CLI reaches the retained active computer but fails during event-chain reconstruction: the guest capability renewal endpoint returns a non-201 response and the guest reports renewal refused. The required read-only replay diff is not recaptured; no state mutation or clean rematerialization is authorized."
      evidence_ref: "CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=10m go run ./cmd/choir computer replay-completeness --computer computer-03335285269bdba4f94377e56879f9e6 at 2026-08-12T18:46Z returned HTTP 500: replay completeness: reconstruct event chain: computer event appender: fetch durable chain: computer event client: capability: guest credential: renewal refused; /health reported deployed_commit d2ab2d2d2184a7918f1a6ff73b6bd29638f85b5c; computer status reported the requested stable computer active at realization_epoch 203."
      consequence: "The replay acceptance remains pending. Diagnose the scoped product credential-renewal path and repair it through the repository/deployment path; do not bypass the guest capability, inject credentials, use SSH, or drop retained state."

  unknowns:
    - "Which of the 26 pre-drop replay differences are behavior-bearing VM-local writes versus retired or legacy residue, and what event or pinned receipt derives each behavior-bearing write?"
    - "How the replay projection obtains the required VM-local schema and baseline when the fresh projection lacks five schema and five table observations present in the live workspace."
    - "Whether rematerialization is product-ready. ProjectionMaterializer is non-runtime today; if the classification and subsequent build show it is not usable, restore falls back to a single-workspace pin checkout on an interim basis with the event head still the sole semantic authority."
    - "Which effect classes are outside the reversible envelope and must refuse to promote under a standing rule."
    - "Whether the upward coagent packet payload can carry operation id, bundle digest, receipt id, and head into Texture revision metadata and citations without a payload schema change."
    - "Whether the CTS-observed registry gap (Texture production registry omitting update_coagent) is still present on the deployed staging build."
    - "Retained computer epoch 8253 disposition; ak_45ce1796 row and root-only auth rollback cleanup."

finish:
  deliver: "The computer works autonomously inside a reversible envelope and can be put back. A CoSuper capsule authors, builds, tests, freezes, and proposes its own source change (solitaire: headless play API, durable persistence, play history); Super and Texture jointly auto-approve it inside a standing rule the owner armed, with no per-candidate owner decision; it promotes, runs, and writes real state; and then the computer is returned to a point in its history — code and state together — with the restored state verified equal to what was recorded there. Texture revises an owner-readable document throughout so the work is supervisable while it happens. The correction spine runs inside the envelope: defective A promotes, admissible evidence falsifies it, B supersedes A, restart proves B, and a full revert proves the whole excursion was undoable."
  artifact: "An authenticated staging computer trajectory carrying: the armed standing rule and its bounds; a frozen bundle A with real SourceTreeRef, capsule-exec BuildRecipeRef, DependencyToolchainRefs, TestReceipts, RuntimeArtifactRef, and an independent verifier receipt; a CoSuper-authored proposal; two seat approvals bound to the exact bundle digest and head; a promotion receipt with checkpoint and route TransitionPromote; a checkpoint binding the target event head, code, artifact program, and a VM-local content witness, with the VM-local Dolt HEAD joined as an audit receipt rather than as restore authority; solitaire game rows written under the promoted release; a headless play transcript falsifying A; superseding candidate B and a post-restart proof that B is effective; and a total revert to a pre-promotion checkpoint — a forward restore intent naming the prior event head, VM-local state rebuilt and the release restaged — whose post-revert re-extraction reproduces the recorded schema and content hashes exactly, with the solitaire capability and its rows both absent. Joined to that chain, a Texture document with a revision at each consequential transition, carrying identity in revision metadata and typed citations while its prose stays human."
  acceptance:
    - action: "Replay completeness probe (do this first): rematerialize VM-local state from the event chain through the current head and compare the result against a live DoltStateExtractor reading. Report the exact diff."
      proves: "Whether every behavior-bearing VM-local write is a deterministic function of the event chain plus pinned receipts. A clean match licenses rematerialization as the restore path; a diff names precisely which writes are not event-derived and must be fixed or fail the checkpoint closed."
      evidence_class: deployed proof
    - action: "Checkpoint completeness: take a checkpoint and show it binds the target applied/effective event head, CodeRef, ArtifactProgramRef, and a VM-local content witness (extractor schema/table/content-root hashes), with the VM-local Dolt HEAD joined as an audit receipt. Show checkpoint creation refuses when a behavior-bearing local row is not event- or receipt-derivable."
      proves: "A point in history is addressable, and the checkpoint cannot silently promise a restore it could not perform."
      evidence_class: deployed proof
    - action: "Revert build and verification: resolve the checkpoint, quiesce writers, append a forward restore intent naming the prior event head, restage the release and rebuild VM-local state through that head, re-run the extractor, and flip visibility only on exact hash match. Prove a partial or mismatched restore keeps the prior realization and never greens."
      proves: "Restore is an acceptance-fenced forward transaction, not a distributed commit. The extractor that today observes drift becomes the fence."
      evidence_class: deployed proof
    - action: "Scope refusal: show that a user-computer revert does not touch the shared platform store or cycle state, and that an attempt to include them is refused."
      proves: "One computer's restore cannot rewind another computer or shared service state."
      evidence_class: deployed proof
    - action: "Autonomous promotion: a CoSuper capsule authors, builds, tests, freezes, and proposes; Super and Texture both sign bound to the exact bundle digest and head; the candidate promotes with no per-candidate owner decision. Prove promotion refuses when the standing rule is absent, expired, or revoked, and when either seat is missing, failed, or withheld."
      proves: "The computer promotes its own work inside a rule the owner granted, and the envelope's edges hold."
      evidence_class: deployed proof
    - action: "Capsule authorship proof: every required bundle ref binds a real execution receipt from the authoring capsule."
      proves: "The candidate is authored by the computer, not hand-authored and pushed through the pipeline."
      evidence_class: deployed proof
    - action: "Live E2 inside the envelope: A promotes -> solitaire is played through the headless API and writes rows -> evidence falsifies A -> B supersedes A -> restart proves B effective."
      proves: "The correction spine runs autonomously and survives restart."
      evidence_class: deployed proof
    - action: "Total revert of the excursion: return to a checkpoint taken before A promoted. Show the solitaire API absent, the solitaire rows absent, state hashes equal to the recorded checkpoint, and the event chain still carrying the full history of what was undone."
      proves: "Autonomy is safe because it is reversible: weeks of work can be wound back to a chosen point without erasing the record that it happened."
      evidence_class: deployed proof
    - action: "Irreversible-boundary refusal: attempt an effect that leaves the reversible envelope (external send/publish) under the same standing rule and show it is refused without a decision, while internal promotion proceeds."
      proves: "Reversibility substitutes for approval exactly where effects are reversible, and not at all where they are not."
      evidence_class: deployed proof
    - action: "Supervision legibility: at each consequential transition a Texture revision is readable through the public API and CLI, its prose human and its metadata and typed citations carrying operation, bundle digest, receipt, and head. At least one revision is committed while capsule work is still open."
      proves: "The owner can watch and intervene during autonomous work rather than reading a post-hoc report."
      evidence_class: deployed proof
    - action: "Replay the accepted trajectory from canonical events."
      proves: "Single semantic authority reconstruction; no second state authority was introduced."
      evidence_class: deployed proof

  rollback: "Revoke the standing rule (no further promotion without a new grant); total revert to a prior checkpoint restoring the release pointer and VM-local state only, platform and cycle stores explicitly out of scope; revert the behavior commits through origin/main and CI to the last accepted runtime. Direct file edits on the deployed node are not the proof and are not a rollback path. Effects that already left the reversible envelope are not recoverable by revert and must be reported as such."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  completion_cutover:
    required: true
    purpose: "Staging trajectory proof alone is a false complete if haunted teaching remains live authority. These obligations are part of goal.complete, not post-hoc cleanup."
    upon_deployed_acceptance:
      - id: doctrine-promote
        action: "Promote earned invariants into choir-doctrine / computer-ontology / agent-product-doctrine: checkpoint = event head + CodeRef + ArtifactProgramRef + VM-local content witness; restore acceptance-fenced and scoped; standing-rule + seats replace per-candidate approval for reversible effects; irreversible refusal lane; effects OFF is pre-gate not destination. Preserve platform/cycle/frontend OUT exclusions unless a successor changes them."
        class: promote-to-doctrine
      - id: lexicon-cutover
        action: "Cut product and API vocabulary from rollback/accept_once/approval-as-safety to restore/standing_rule/seats/irreversible decision where those names teach the old world (selfdev states, CLI, Settings copy, Definition skill templates). Keep accept_once only as the irreversible/decision lane if retained."
        class: rewrite-in-place
      - id: roadmap-registry-archive
        action: "Confirm choir-self-development-roadmap is historical-only in registries; stamp SUCCESS on this Definition; ensure CTS cannot be read as an entrypoint; remove live authority edges that still schedule D1/Mission-0/accept_once rehearsal."
        class: delete-live-authority
      - id: survivor-detector-replace
        action: "Replace (do not merely green) survivor/detector pins of CoSuper-without-update_coagent and packet.kind executability; add detectors against standing-rule authorizing send/publish/pay and against treating ComputerVersion code tuple alone as full restore."
        class: test/detector update
      - id: owner-product-surface
        action: "Ship or open a successor Definition for owner-reachable standing-rule arm/revoke, checkpoint list, restore invoke, and irreversible refusal receipts via CLI/API/Settings — restore must not remain a lab-only ceremony."
        class: successor-mission
      - id: ops-identity
        action: "Land restore-aware health/identity reporting (event head + CodeRef + witness + route; distinguish product restore from failed deploy / git revert). Retain release artifacts needed for advertised checkpoints."
        class: successor-mission
      - id: frontend-ownership
        action: "Decide and document: host frontend is platform control-plane OR fold into frontend-in-release successor; do not leave 'total revert of the computer' ambiguous."
        class: "promote-to-doctrine | successor-mission"
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
    - Promotion required a per-candidate owner decision, or promoted with no standing rule armed, or under an expired or revoked one.
    - A candidate promoted on one seat, or the pair was quietly reduced to one when a seat failed, timed out, or withheld.
    - An irreversible effect promoted under the standing rule without a decision.
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
    - Effects stay OFF until the staging rehearsal gate passes, and the rehearsal gate now includes a proven total revert.
    - Revert never erases history. Returning to a prior point is a forward transaction that restores state; the event chain retains the full record of what was done and undone.
    - Reversibility substitutes for approval only within the reversible envelope. Effects that cross an external boundary — send, publish, pay, third-party write — are outside it and are not promotable under a standing rule.
    - Checkpoints bind the target event head, code, artifact program, and a VM-local content witness. A checkpoint that cannot restore state is not a checkpoint, and one that cannot be rebuilt from the tape must fail closed rather than be recorded.
    - The event chain is the address and the authority. VM-local Dolt is a projection and an audit witness, never an alternate head.
    - "Restore is scoped to the user computer: VM-local state and the release pointer. The shared platform store and cycle state are out of scope, because restoring them would rewind other computers."
    - "Restore is acceptance-fenced, not a distributed transaction: stage, verify against recorded content hashes, and flip visibility only on exact match. Partial never greens."
    - The candidate is authored inside a CoSuper capsule; every required bundle ref binds a real execution receipt from that capsule.
    - CoSuper holds update_coagent and reports upward as an actor, while a CoSuper packet can never open privileged execution on Super. Executability derives from the sender's authorization, not from the model-written packet.kind. The pinned survivor contract is replaced by a stronger test, never deleted.
    - CoSuper freezes and proposes its own candidate. Proposal is worker authority, not a supervisory gate.
    - "Approval is joint and automatic: the Super seat and the Texture seat both sign, each bound to the exact bundle digest and the head under decision, and the candidate promotes without a per-candidate owner decision. Missing, failed, or withheld approval blocks promotion; the pair cannot be shrunk to one."
    - Auto-promotion runs only inside a standing rule the owner armed. The owner grants the rule; the seats approve instances within it. An expired, revoked, or absent rule means no promotion, and the rule names its own bounds and expiry.
    - The owner retains revocation and rollback at all times, and revocation does not require agent cooperation.
    - The historical self-development roadmap is migration evidence only — not live schedule authority; execute this Definition, not Mission 0–5 from that table.
    - "Reconnection stays minimal on purpose: the smallest change that restores the channel and holds the privilege property. The RLM/Yaegi rebase is expected to rewrite this layer, so no general authorization or quorum framework is built here."
    - A's defect is pre-declared in a Define receipt before A is proposed, so the falsification cannot later be reported as discovered.
    - Self-development schema changes are additive only — new tables via CREATE TABLE IF NOT EXISTS, never ALTER or DROP of existing tables — because there is no migration framework and rollback does not reverse schema.
    - "The solitaire candidate itself touches no protected surface: no auth/session, no event/decision path, no provider routing, and no Texture write from solitaire code. This is distinct from the mission's Texture writes, which are required — Texture supervises the self-development through its own owner and atomic reducer, and the candidate is its subject, never its author."
    - Texture's canonical write path, single-owner contract, and atomic revision transition are preserved exactly; observing self-development adds an evidence source to Texture, never a second writer or a second document authority.
    - A Texture revision is evidence and legibility, never acceptance authority; reading a revision does not accept an effect, and no Texture write may append, accept, materialize, checkpoint, or route.
    - No headless/CDP/virtual-authenticator substitute for exact owner presence; no SSH-shaped operations; product path only.
    - "Problem-documentation-first: a discovered platform problem is documented in a code-free Define receipt before any repair-code commit."
  excluded:
    - RLM/Yaegi actor authoring (successor capability mission; Phase 1 of the RLM memo preserves effects-OFF).
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
  slice: "The deployed pre-drop replay completeness probe is complete: it returned not_equivalent with 26 deterministic DoltStateExtractor differences and both event heads null. Clean rematerialization is not licensed. Next classify each difference and make behavior-bearing writes event-derived or fail checkpoint creation closed before checkpoint design."
  question: "Which differences are behavior-bearing VM-local writes versus retired or legacy residue, and what event or pinned receipt derives each one? The answer determines the restore mechanism and the checkpoint fail-closed boundary."

  reconciliation:
    observed_at: 2026-08-12T06:56:12Z
    source_ref: main@24fb24de
    deploy_identity: "staging https://choir.news reconciled 2026-08-12: health reports proxy status ok, deployed commit 3cd12d1452ad1d06b5df57cf9183313568f60cb5, vmctl status ok; this Definition remains effects-OFF until its own rehearsal and restore gates pass"
    authority_identities: [docs/choir-vision.md, docs/choir-doctrine.md, docs/standing-questions.md, docs/computer-ontology.md, docs/agent-product-doctrine.md, AGENTS.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: 2026-08-12 read-only git status (clean)
    status: reconciled

  blocker_or_risk: "The deployed pre-drop probe found 26 deterministic differences, so rematerialization is not licensed. The effects mission must classify the five missing schema observations, five missing table observations, and sixteen differing observations, then either make behavior-bearing writes event/receipt-derived or refuse checkpoints that cannot restore them. ProjectionMaterializer is still non-runtime, so pin checkout remains only an implementation contingency."
  next_action: "Document and classify the retained replay diff before any checkpoint or restore implementation. For each difference, identify its owning ledger and event/receipt derivation; retire or exclude legacy residue, repair event derivation for behavior-bearing state, and add a fail-closed checkpoint predicate for anything not reproducible. Do not rerun the probe as if unanswered, and do not drop or mutate the retained evidence."

  candidate:
    id: none
    state: none
  decision:
    selected: "Reversibility is the proof target, not acceptance. Revert is built and verified in this mission. Restore is scoped to VM-local state plus the release pointer, addressed by event head, and acceptance-fenced: rematerialize through the target head, verify against recorded content hashes, flip visibility only on exact match. The shared platform store and cycle state are out. Checkpoints bind the target event head, CodeRef, ArtifactProgramRef, and a VM-local content witness, with the VM-local Dolt HEAD as an audit receipt rather than restore authority. CoSuper authors and proposes; Super and Texture jointly auto-approve inside an owner-armed standing rule; no per-candidate owner decision. Decisions are reserved for effects leaving the reversible envelope. CoSuper regains update_coagent with executability relocated from packet.kind to sender authorization. Content remains capsule-authored solitaire, API-only. RLM strictly after."
    kind: architecture
    status: ratified
    source: orchestrator
    evidence_ref: "docs/choir-self-development-roadmap-2026-08-11.md; .agentic-consensus/self-dev-roadmap/{divergent,lateral,convergent}/; .agentic-consensus/readiness-key-2026-08-11/"
    owner_ratification_ref: "owner direction 2026-08-11, in sequence: model policy rejected as content axis ('we should be changing source code'); capsule authorship required ('definitely written by a cosuper in a capsule'); Texture supervision required ('just writing solitaire isnt getting at the essence of the point'); metadata out of prose ('that kind of information should live at the super level, and/or in citations or metadata, not in texture version prose'); CoSuper reconnection required ('cosuper absolutely needs update_coagent - thats essential'); two seats are auto-approval not a proposal gate ('because users have rollback, we can do auto-promotion'); and the reversibility reframe ('ive never once thought i was building something that requires human approval for changes ... rollback arbitrarily to any point in its history'), with Dolt revert explicitly in scope."
    recorded_at: 2026-08-11T20:05:00Z
    consequence: "Per-candidate owner approval is removed as a requirement and replaced by an owner-armed standing rule plus two-seat auto-approval. The decision-binding verifier and mode CAS are relaxed deliberately, which is the mission's heaviest evidence burden. Checkpoint and revert become deliverables rather than a rollback field. Effects leaving the reversible envelope gain an explicit refusal requirement. Freeze/propose authority must be wired onto CoSuper, which today has no production call site. Pre-mission haunted-authority cutover (roadmap demotion, doctrine/ontology transitional language, RLM Phase-1 re-derive note, restore-set boundary, AGENTS restore-vs-deploy note) landed green; finish.completion_cutover must still run after deployed acceptance or the goal is a false complete."
  evidence_refs: [docs/choir-self-development-roadmap-2026-08-11.md, docs/choir-crashed-prime-session-review-2026-08-09.md, docs/memo-persistent-rlm-actors-2026-08-09.md, docs/memo-live-retrospective-evals-2026-08-09.md]
  blocker_or_risk: "Revert is the mission: nothing in production reads Dolt history back, checkpoints bind no state, and Dolt commits carry no head binding. Replay completeness is unresolved and is the deciding measurement; ProjectionMaterializer is explicitly non-runtime today, so the preferred rematerialization path has no runtime implementation yet and an interim single-workspace pin checkout may be needed. Relaxing the decision-binding verifier and mode CAS touches the surfaces that make the tape trustworthy and carries the heaviest evidence burden. RESOLVED 2026-08-11: staging deploy identity reconciled; owner-bearer residual disposed; capsule build capability confirmed; frontend serving location determined (UI out of scope)."
  next_action: "Run the replay completeness probe: rematerialize VM-local state from the event chain through the current head, diff against a live DoltStateExtractor reading, and report exactly which writes are not event-derived. That single measurement decides whether restore is rematerialization (preferred) or an interim single-workspace pin checkout, and it is the only remaining computer-science unknown in the revert design."

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

That the computer can work autonomously and be put back. It promotes its own
source change without a per-candidate owner decision, runs it, writes real
state — and is then returned to a chosen point in its history, code and state
together, with the restored state verified equal to what was recorded there.

## Rules of the envelope

**Inside the envelope, the computer promotes its own work.** CoSuper freezes and
proposes. Super and Texture both sign, each approval bound to the exact bundle
digest and head. The candidate promotes with no per-candidate owner decision.

**The owner arms a standing rule, not each candidate.** The rule carries explicit
bounds and expiry. Absent, expired, or revoked means no promotion. Both seats are
required; a pair never shrinks to one because a seat failed, timed out, or
withheld.

**The envelope has an edge.** Reversibility substitutes for approval exactly to
the degree an effect is reversible. Internal state, code, documents, and the
object graph are recoverable. Anything crossing an external boundary — send,
publish, pay, third-party write — is not, and is not promotable under a standing
rule. That boundary is where a decision belongs.

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

API-only. Staging serves the web frontend from the host, outside the release the
updater controls, so a UI change would land where the browser never reads.

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
2. **Checkpoint completeness (red).** Bind target event head, CodeRef,
   ArtifactProgramRef, and a VM-local content witness, with the VM-local Dolt HEAD
   as an audit receipt. Refuse checkpoint creation when a behavior-bearing local
   row is not event- or receipt-derivable.
3. **Revert build and verification (red, heaviest).** Implement the restore
   procedure above. Prove partial and mismatched restores keep the prior
   realization and never green. Prove the platform store and cycle state are
   refused.
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
6. **Auto-promotion authority (red).** A seat-signed decision authority the
   decision-binding verifier accepts alongside `external-owner:`, requiring both
   seats bound to the exact bundle digest and head; a standing-rule mode beside
   `accept_once` with owner-set bounds and expiry; refusal proofs for absent,
   expired, and revoked rules and for a missing seat.
7. **Irreversible boundary (red).** Classify effects leaving the envelope; prove
   they refuse to promote under a standing rule.
8. **Supervision wiring (green).** Confirm the upward packet carries joinable
   identities and that Texture's production registry has `update_coagent`.
9. **Rehearsal (orange→red).** Trivial change: propose → auto-approve → promote →
   write state → total revert → verify. The live run is gated on this passing,
   revert included.
10. **Live proof (red).** Capsule authors A → promotes → played → falsified → B
   supersedes → restart proves B → total revert of the excursion.

Out of scope: RLM/Yaegi authoring, model-policy content, the web UI, production,
and the shared platform store.

## Completion cutover (after deployed acceptance)

The trajectory proof is not enough. Before `goal.complete`, run every item under
`finish.completion_cutover` in the frontmatter: promote earned doctrine, cut
lexicon, archive haunted roadmap/registry edges, replace survivor/detector pins,
schedule or ship owner restore/standing-rule surfaces, land restore-aware ops
identity, decide frontend ownership, rewrite successor preconditions (RLM / Wire /
in-choir), and name residual realism axes (schema, backups/copies, tape vs
erasure, owner verbs). Skipping that packet is a false complete.
