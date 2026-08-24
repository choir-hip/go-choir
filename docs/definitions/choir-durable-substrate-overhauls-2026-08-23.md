---
definition_version: 2
definition_id: choir-durable-substrate-overhauls-2026-08-23
execution_mode: mission_orchestrator

start:
  captured_at: "2026-08-23T02:00:00Z"
  source:
    canonical_ref: "main@3f9173be"
    deploy_identity: "staging https://choir.news"
  worktree_inventory:
    status: reconciled
    evidence_ref: "git status 2026-08-23"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP; this Definition owns the full durable substrate overhaul program across keys, files, mail, and assurance."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      paths_or_digest: [docs/designs/choir-durable-substrate-2026-08-23.md]
      recovery: leave_in_place
  candidates:
    - id: none
  observed_artifact:
    - claim: "The four substrate gaps exist in current source: files not backed up in tape; mail is host-side per-owner SQLite; keys are unrecoverable mode-0400 files on guest disk; recovery from cold is O(history)."
      evidence_ref: "docs/designs/choir-durable-substrate-2026-08-23.md §1; internal/computerevent/privacy.go; internal/maild/store.go; internal/provideriface/config.go"
    - claim: "The durable substrate design was validated across 5 agentic-consensus rounds and corrected."
      evidence_ref: "docs/designs/choir-durable-substrate-2026-08-23.md"

finish:
  deliver: "The complete durable, secure, and recoverable computer substrate across all four tracks: (1) Track K key escrow with user passkey PRF wrapping, host-held two-approval recovery, and eager backfill; (2) Track F content-addressed file-CAS write-back commit protocol with FileRootCommitted Merkle roots, periodic ProjectionBase event-head checkpoints, and O(delta) recovery; (3) Track M per-computer-address host MTA spool + guest Maildir SoR with spool deletion bound to file-CAS checkpoints; (4) Assurance & Scale with portable recovery capsules, automated daily restore drills, continuous integrity scrub, and recovery cells. Proven on staging with measured SLOs."
  artifact: "A deployed staging substrate trajectory proving: (1) DEK escrow on create + eager offline backfill + passkey PRF key derivation; (2) file tree recovery from CAS after simulated disk loss + sync_computer_files() barrier + referential GC (strictly allowlisted to preserve canonical event payloads) + periodic ProjectionBase event watermarks; (3) inbound mail delivered to down computer, spooled, and drained to Maildir with Message-ID deduplication and spool retention until FileRootCommitted; (4) portable capsule restore + daily restore drill with measured RTO/RPO numbers published."
  acceptance:
    - action: "Track K Keys Acceptance: Create a computer → DEK is escrowed under owner ROOT key; eager backfill escrows existing active computer DEKs; simulate disk loss → recover key under application-level two-approval gate; prove passkey PRF key derivation wrap."
      proves: "Keys survive disk loss and are recoverable under authorized ceremony, with zero unescrowed active computers."
      evidence_class: deployed proof on test computer
    - action: "Track F Files & Periodic Bases Acceptance: Write files to guest → checkpoint pins changed chunks to CAS and commits FileRootCommitted → simulate disk loss → recover file tree to last root in O(delta); verify sync_computer_files() forces immediate checkpoint; verify GC preserves live roots and canonical event stores (computer-event/, computer-event-payload/, pin-receipts/, projection-base/); verify periodic ProjectionBase watermark publishing keeps event replay bounded."
      proves: "Guest files and event state survive full-disk loss with bounded RPO, O(delta) recovery, and permanent prevention of unbounded event replay."
      evidence_class: deployed proof on test computer
    - action: "Track M Mail Acceptance: Send mail to a down computer → host spool fsyncs and returns 250 → boot computer → mail drains asynchronously to guest Maildir → host deletes spool entry only after FileRootCommitted checkpoint covers the Maildir state."
      proves: "Mail conforms to RFC 5321 store-and-forward, is isolated per-computer under DEK encryption, and is durable against crash before write-back."
      evidence_class: deployed proof on test computer
    - action: "Assurance Acceptance: Automated restore drill runs daily in an isolated realization, exercising both key unwrap and data restore; published SLO numbers reflect measured drill results; background integrity scrub verifies blob health."
      proves: "Published recovery guarantees are continuously measured and verified."
      evidence_class: deployed drill receipts + scrub telemetry

rollback: "Revert mission commits via origin/main + CI to last accepted runtime. Canonical events are never rewound."
landing:
  required: true
  environment: staging
  required_receipts: [pushed_commit, ci, deploy, staging_build_identity, track_k_keys_proof, track_f_files_proof, track_m_mail_proof, assurance_drill_proof]
not_done_when:
  - cross-owner dedup is enabled (violates DEK boundary)
  - mail 250 is sent before host spool is fsync'd
  - mail spool entry is deleted before FileRootCommitted covers the message
  - GC collects any blob reachable from a live root, recovery capsule, or canonical event payload/envelope namespace
  - SLO numbers are published without automated drill measurements
  - periodic ProjectionBase event checkpoints are omitted, allowing event replay depth to grow unbounded
  - canonical event chain is rewound

boundaries:
  mutation_class: red
  authority_sources: [owner direction 2026-08-23, docs/designs/choir-durable-substrate-2026-08-23.md, docs/choir-doctrine.md, docs/computer-ontology.md, AGENTS.md]
  must_preserve:
    - Single guest ComputerEventAppender remains the sole semantic event writer; tape cites Merkle roots via FileRootCommitted, CAS owns blob bytes.
    - Three-ledger authority strictly split: (1) event tape (corpusd HeadCAS), (2) file-CAS (platform-artifacts blob store), (3) key wrap table (host escrow / passkey PRF). Mail lives under (2).
    - Cross-owner dedup is strictly forbidden.
    - Referential GC must namespace-allowlist computer-event/, computer-event-payload/, pin-receipts/, and projection-base/.
    - Effects remain OFF until decision-policy gates pass.
  not_goals:
    - Immediate recovery of stopped computer 0333528 (owned by choir-durable-substrate-recovery-2026-08-23.md).
    - Production environment deployment.
  protected_surfaces: [canonical computer event chain, guest ComputerEventAppender, checkpoint/route projection, platform-artifacts blob store, privacy-key file, maild store, vmctl lifecycle]
  completion_evidence_floor: [deployed proof across all 4 tracks, independent review of frozen boundary]

phases:
  - name: Track K — Key Escrow & Wrap Hierarchy
    items:
      - "Build host-side wrap table and DEK escrow on computer creation under owner ROOT key."
      - "Implement eager offline wrap backfill for existing active computers and lazy per-boot upgrade."
      - "Build application-level two-approval authorization gate and transparency logging."
      - "Implement passkey PRF wrapping key derivation (Track K 1b)."
  - name: Track F — File-CAS & Commit Protocol & Periodic Event Bases
    items:
      - "Build content-addressed file-CAS in platform-artifacts with chunk pinning and Merkle root generation."
      - "Implement FileRootCommitted guest projection op and write-back cache with epoch checkpointing."
      - "Implement sync_computer_files() syscall/RPC barrier."
      - "Build referential-integrity-anchored GC with 24-hour grace window, namespace-allowlisting canonical event payloads."
      - "Implement periodic ProjectionBase event-head watermark publishing to permanently bound event replay depth."
      - "Prove O(delta) restore by materializing root pre-replay on a test computer."
  - name: Track M — Mail MTA Spool & Guest Maildir
    items:
      - "Migrate host maild to Postfix-shaped MTA with fsync'd spool queue as SoR for in-flight mail."
      - "Implement per-computer address resolution and async LMTP-style drain with Message-ID deduplication."
      - "Build guest Maildir mailbox in /mnt/persistent under DEK encryption and file-CAS."
      - "Bind host spool deletion to FileRootCommitted checkpoints covering the received mail."
      - "Replace owner-scoped mail auth with computer-scoped relay capability."
  - name: Assurance & Scale
    items:
      - "Build self-describing portable recovery capsule (platform + customer media, pull/WORM custody)."
      - "Build automated daily restore drill in isolated realization and publish measured SLOs."
      - "Implement continuous background integrity scrub with repair."
      - "Build recovery cells with per-cell restore budgets and weighted fair scheduling."

now:
  status: working
  slice: "Pre-Flight definition (choir-durable-substrate-preflight-2026-08-24.md) completed; 0333528 active and liveness restored on staging. Beginning Track K (Key Escrow & Wrap Hierarchy)."
  question: none
  reconciliation:
    observed_at: "2026-08-23T02:00:00Z"
    source_ref: "main@3f9173be"
    deploy_identity: "staging https://choir.news"
    authority_identities: [docs/choir-doctrine.md, docs/computer-ontology.md, AGENTS.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "working tree owns substrate overhauls definition"
    status: reconciled
  candidate:
    id: none
    state: none
  decision:
    selected: "Execute complete substrate overhauls across Tracks K, F, M, and Assurance as one sequenced goal following immediate recovery."
    kind: architecture
    status: settled
    source: owner direction 2026-08-23
    evidence_ref: docs/designs/choir-durable-substrate-2026-08-23.md
    owner_ratification_ref: "owner confirmed 2026-08-23"
    recorded_at: "2026-08-23T02:00:00Z"
    consequence: "All four overhaul tracks proceed under one cohesive definition once 0333528 is unblocked."
  evidence_refs:
    - "docs/designs/choir-durable-substrate-2026-08-23.md"
  blocker_or_risk: "None for definition authoring; execution waits on completion of the recovery definition."
  next_action: "Execute Track K: Key Escrow & Wrap Hierarchy."

receipts:
  - id: define-overhauls-boundary
    boundary: define
    commit_or_artifact: "(pending)"
    proof_refs: [docs/designs/choir-durable-substrate-2026-08-23.md]
    rollback_ref: revert docs commit
    disposition: "Overhauls definition drafted and sequenced after recovery"
    problem_ref: none
    authorization_ref: "owner ratification 2026-08-23"
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: not_applicable
