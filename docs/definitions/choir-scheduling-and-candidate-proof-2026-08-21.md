---
definition_version: 2

start:
  captured_at: "2026-08-21T22:00:00Z"
  source:
    canonical_ref: "main@91797509"
    deploy_identity: "staging https://choir.news proxy e42c65c0; guest autoputer e42c65c0 on retained computer computer-03335285269bdba4f94377e56879f9e6 active epoch 361 (propose_only generation 1, effects OFF); pre-A checkpoint 99949fe2 published and untouched; assignment supersession loop documented and unfixed"
  worktree_inventory:
    status: reconciled
    evidence_ref: "2026-08-21 git status; clean single worktree /Users/wiz/go-choir"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP; this Definition owns itself, its receipts, and the three navigation registries."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      recovery: leave_in_place
  observed_artifact:
    - claim: "Architecture review v3 ratified by owner direction 2026-08-21 with consensus-panel corrections incorporated: two writer classes, Living Document Protocol, Super singleton as coherence mechanism, Mission A scheduling contract, memory containment policy, Mission B parallel release gate."
      evidence_ref: "docs/reviews/architecture-review-texture-cosuper-memory-2026-08-21.md; docs/reviews/architecture-review-consensus-synthesis-2026-08-21.md"
    - claim: "All steering docs aligned to the ratified contract in commits baab8e49..91797509 (texture-live-supervision, supervision-protocol, prompting-invariants, doctrine I1/I26, current-architecture, ontology supervision actor contract, runtime-invariants, agent-product-doctrine, ACTIVE/NOW/mission-graph/semantic-registry/heresy-detectors H033-H037, RLM memo, platform ledger)."
      evidence_ref: "git log baab8e49..91797509 --oneline"
    - claim: "Staging blocker is scheduling, not resources: two competing pending Texture execution requests ping-pong fresh assignments; capsule memory reclaim works; every CoSuper cancelled before author/build/freeze."
      evidence_ref: "docs/evidence/effects-red-assignment-supersession-loop-2026-08-21.md; docs/evidence/effects-red-capsule-memory-budget-exhaustion-2026-08-21.md"
    - claim: "Pre-A checkpoint 99949fe2 remains the immutable restore fence from the prior mission; unchanged by docs alignment."
      evidence_ref: "docs/evidence/effects-red-pre-a-checkpoint-published-2026-08-19.md"
  unknowns:
    - "Whether genuine Texture authoring turns (replacing synthetic join wait-turns) produce acceptable revision cadence without forcing milestones into revisions."
    - "Exact arrival-ordinal storage shape: lifecycle authority field vs separate computer-scoped table."

goal:
  product_outcome: "The automatic computer schedules exactly one non-expired execution request at a time per computer under a durable, restart-safe contract; one CoSuper capsule authors, builds, tests, freezes, and proposes candidate change A end-to-end on staging; qualified consensus, promotion, live play verification, falsification with B, and acceptance-fenced restore to checkpoint 99949fe2 follow under the already-ratified candidate-proof decision policy."
  observed_starting_state: "Retained computer epoch 361 on e42c65c0; assignment supersession loop active (two competing parent requests); effects OFF; docs fully aligned to ratified architecture."
  final_artifact: "Deployed staging proof: scheduler selects one request by computer-scoped ordinal, settles stale duplicates as late evidence, survives restart without re-supersession; CoSuper completes full candidate A arc through restore. Texture revisions appear when semantic state changes — authored by the Texture agent — with lifecycle events (assignment/ordinal/deadline) never forcing revisions."
  authority_boundary: "This Definition owns the scheduling-contract implementation, texture-join authoring repair, and the candidate A proof. It does not authorize parallel assignments (Mission B), Cloud Hypervisor migration, PSI/zram tiers, email effects, any live send/mail/restore outside the acceptance-fenced E2 leg, or redesign of the consensus panel architecture (deferred to a post-RLM mission; reversible-selfdev-v1 is used as-frozen with gpt-5.6-terra staffing the independent-reviewer seat). Effects remain OFF until promotion within this Definition's own gates."
  compact_current_state: "Docs aligned; scheduler unimplemented; supersession loop live on staging; candidate A unpaid."

invariants:
  - id: two-writer-classes
    invariant: "Texture revisions are written only by AuthorAppAgent (the Texture agent); AuthorUser owner edits use immediate canonical-head CAS. No other agent writes document text. texture_turn_committed is not a revision."
    evidence_ref: "docs/choir-doctrine.md I1; docs/heresy-detectors.md H033"
  - id: super-singleton-coherence
    invariant: "Exactly one Super per ComputerID as coherence/error-correction and resource arbitration — never a concurrency limiter, never a document/computer mutator."
    evidence_ref: "docs/choir-doctrine.md I26; docs/heresy-detectors.md H034"
  - id: scheduling-contract
    invariant: "Durable computer-scoped arrival ordinals; FIFO among non-expired requests; expiry on terminal operation / superseding owner correction / deadline; assignment deadlines fail rather than hang; admission refusal is retryable with work pending."
    evidence_ref: "docs/choir-doctrine.md I26; docs/heresy-detectors.md H035"
  - id: memory-containment
    invariant: "memory.high=requested, memory.max=2x requested, memory.events OOM counters as feedback; no PSI pause/resume or zram in this mission."
    evidence_ref: "docs/computer-ontology.md Supervision Actor Contract"
  - id: capsule-cardinality
    invariant: "N capsules per CoSuper assignment is doctrinal; this mission implements transitional 1:1."
    evidence_ref: "docs/heresy-detectors.md H036"
  - id: effects-off-until-gates
    invariant: "No effect write occurs until promotion inside this Definition's own consensus gates; no live send/mail ever in this Definition."
    evidence_ref: "docs/ACTIVE.md"

conjectures:
  - id: fifo-scheduler-suffices
    conjecture: "A durable computer-scoped arrival ordinal plus FIFO among non-expired requests eliminates the assignment supersession loop without priority or preemption machinery."
    falsifier: "A legitimate starvation case appears during the candidate A arc (e.g., an expired-but-important request that FIFO cannot reach before restore)."
  - id: genuine-authoring-turn-cadence-ok
    conjecture: "Routing selfdev joins through genuine Texture authoring turns produces healthy revision cadence without milestone-forcing."
    falsifier: "Turn-outcome census after repair shows semantic changes going unpublished or revision spam from mechanical transitions."
  - id: panel-diversity-deferred
    conjecture: "A two-model verification panel (luna authoring + terra reviewing) is adequate for candidate A; deeper panel design (true model diversity, seat topology, RLM-era agentic model config) is deliberately deferred to a post-RLM mission when node-b gateway compute expands."
    falsifier: "Candidate A promotion passes while the two models share a correlated blind spot that restore/falsification then exposes — evidence that same-family review was insufficient."

phases:
  - name: Scheduler Contract
    items:
      - "Add durable computer-scoped arrival ordinal to execution_request controls (additive schema)"
      - "Implement FIFO selection among non-expired requests in persistent-Super controller; select exactly one work item per cycle"
      - "Implement request expiry (terminal operation / superseding owner correction / deadline) and assignment deadlines failing closed"
      - "Settle the two stale selfdev textures on staging as hygiene: cancel + tombstone pending requests as late evidence"
      - "Tests: cross-trajectory arrival order, same-timestamp tie-break, restart recovery, duplicate settlement, deadline failure"
  - name: Texture Authoring Repair
    items:
      - "Route selfdev texture join through genuine Texture agent authoring turn when semantic state must change; remove synthetic deterministic run projection"
      - "Add turn-outcome vs revision-outcome census receipt before/after repair"
      - "Tests: semantic change produces exactly one version; wait produces none; no milestone-forcing"
  - name: Candidate Proof
    items:
      - "Verify scheduler holds under real candidate A load (no supersession across full arc)"
      - "CoSuper capsule authors, builds, tests, freezes candidate A with five bundle refs bound to capsule receipts"
      - "Qualified consensus under reversible-selfdev-v1 approves exact subject; independent-reviewer seat runs on gpt-5.6-terra (distinct from authoring CoSuper's luna) per owner direction 2026-08-21"
      - "Promotion, live play API verification, falsification with B"
      - "Acceptance-fenced restore to pre-A checkpoint 99949fe2"

finish:
  deliver: "Staging computer runs the scheduling contract end-to-end: one selected request executes to completion while others stay pending untouched; restart-safe; supersession loop impossible. On that substrate, candidate A completes the full author->freeze->consensus->promote->play->falsify->restore arc under reversible-selfdev-v1 with zero per-candidate human decisions."
  artifact: "Authenticated staging trajectory carrying: scheduler receipts (ordinal, selection, expiry, deadline events); Texture revisions wherever semantic state changed during the arc (authored by the Texture agent; scheduler/assignment lifecycle never calls ApplyTextureTurn); frozen bundle A with the five required refs bound to capsule-exec receipts; consensus/quorum receipts; promotion event + epoch receipt; play API evidence; B falsification evidence; restore-to-99949fe2 witness."
  acceptance:
    - action: "Scheduler proof: with two or more pending requests, exactly one runs; restart mid-run preserves selection; stale duplicates settle as late evidence without spawning assignments."
      proves: "The assignment supersession loop is structurally impossible, not merely quiet."
      evidence_class: deployed proof
    - action: "Capsule authorship proof: all five bundle refs bind execution receipts from the authoring capsule."
      proves: "Candidate A is computer-authored, not hand-authored."
      evidence_class: deployed proof
    - action: "Policy-governed reversible promotion with qualified consensus on exact subject; refusal proofs when policy is absent/expired/stale/below-quorum."
      proves: "Autonomy boundary holds under the ratified decision policy."
      evidence_class: deployed proof
    - action: "Live E2: A promotes -> solitaire plays via headless API writing rows -> evidence falsifies A -> B supersedes -> restart shows B effective -> restore to 99949fe2."
      proves: "Correction spine and restore fence on the new scheduler substrate."
      evidence_class: deployed proof
    - action: "Supervision legibility: Texture revisions authored by the Texture agent appear wherever semantic state changed during the arc (inherited prior gate); turn-outcome vs revision-outcome census shows semantic changes produce exactly one version each and scheduler/assignment lifecycle produces none; prose human-readable, identity in metadata/citations."
      evidence_class: deployed proof
  not_done_when:
    - "Candidate bundle lacks any of the 5 required capsule-bound artifact references."
    - "Effects are enabled prior to qualified consensus authorization under reversible-selfdev-v1."
    - "Promotion occurs without qualified consensus receipt."
    - "Restore fails to return computer state to exact pre-A checkpoint 99949fe2."
    - "Texture supervision exposes only post-hoc summary without live revisions during open work."
    - "More than one CoSuper assignment is live simultaneously, or any assignment supersedes a live predecessor."
    - "Scheduler/assignment lifecycle calls ApplyTextureTurn or manufactures revisions."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  completion_cutover:
    - id: scheduler-survivor-detectors
      action: "Promote H035 from review-level to enforced family once scheduler lands; add detector for duplicate-live-assignment paths."
      class: detector update
    - id: now-registry-refresh
      action: "Refresh NOW.md observation and Definition registries on landing."
      class: registry update

now:
  status: active
  slice: "Candidate proof resumed on fresh snapshot-backed staging computer computer-aa7739a6c3f7ebec4e984bad8b086486 following substrate overhauls, cleanup cutover, and Phase 1 ArrivalOrdinal FIFO scheduling deployment (cc2cb702)."
  question: "How does CoSuper capsule candidate A authoring, consensus, promotion, play API verification, and restore to pre-A checkpoint execute under the hardened ArrivalOrdinal scheduler on fresh staging microVM?"
  reconciliation:
    observed_at: "2026-08-27T16:15:00Z"
    source_ref: "docs/reports/choir-system-state-and-consensus-review-2026-08-27.md"
    deploy_identity: "staging https://choir.news proxy cc2cb702; active microVM vm-fe04398168752e256835e02d3876bd1c (computer-aa7739a6c3f7ebec4e984bad8b086486) bootstrapped on cc2cb702 with sequence 1 head 96f7728a6ce323622ca900b109c3f2c58798954cc6077de8e243668d43e498f9"
    authority_identities: [docs/choir-vision.md, docs/choir-doctrine.md, docs/standing-questions.md, docs/definitions/choir-scheduling-and-candidate-proof-2026-08-21.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "clean single worktree /Users/wiz/go-choir"
    status: reconciled
  decision:
    selected: "Execute candidate change A arc on fresh staging computer computer-aa7739a6c3f7ebec4e984bad8b086486 with qualified consensus under reversible-selfdev-v1, promotion, live play verification, falsification, and restore to checkpoint 99949fe2."
    kind: execution
    status: owner-ratified
    source: owner
    evidence_ref: "docs/reports/choir-system-state-and-consensus-review-2026-08-27.md"
    owner_ratification_ref: "owner direction 2026-08-27: proceed with candidate proof arc"
    recorded_at: "2026-08-27T16:15:00Z"
    consequence: "Active execution resumed under choir-scheduling-and-candidate-proof-2026-08-21.md."
  evidence_refs:
    - "docs/reports/choir-system-state-and-consensus-review-2026-08-27.md"
  blocker_or_risk: "None blocking execution; staging microVM is online, bootstrapped, and proven with live Texture model revision."
  next_action: "Enable propose_only mode and author candidate A inside CoSuper capsule on staging computer computer-aa7739a6c3f7ebec4e984bad8b086486."
---


