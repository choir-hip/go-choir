---
definition_version: 4

# P1 — record-first controls. Station of
# definitions/choir-precommitment-protocol-metamission-2026-10-11.md;
# design docs/precommitment-protocol-design-2026-10-11.md §7 P1 and §10.
readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-11T16:20:00Z
  source:
    canonical_ref: main f1be9e2f
    deploy_identity: staging choir.news 25c8b5b0
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Texture's ApplyTexture controls (open_researcher,
      open_persistent_super, target_work_item_id) reach the store as
      IssueLifecycleControl (internal/store/lifecycle_control_issue.go).
      Its accumulator mints, per control, the target agent when opening
      research, the work item, the source packet and lifecycle events, all
      in one conditional batch. No commitment record is minted.
    - >-
      CommitLifecycleAct (internal/store/lifecycle_commit_act.go) mints a
      commitment record object (ogKindCommitmentRecord, deterministic
      record id, not-exists condition) and binds its derived packet with
      SourceRecordID (internal/types/evidence.go).
    - >-
      CommitmentRecord has Addressee and EvidenceRefs but no emitter kind,
      computer id, subject references or content hash
      (internal/types/commitment.go).

finish:
  deliver: >-
    Each Texture control mints one cast record, addressed to the control's
    target desk, in the same commit as its packet, and the packet names the
    record as its source. Records carry the fields design §10 reserves:
    emitter kind and computer, subject references, and a content hash with
    the store's commit time.
  artifact: >-
    The control accumulator mints the record; CommitmentRecord gains the
    reserved fields (additive JSON); a trace audit script that counts
    controls against records.
  acceptance:
    - action: >-
        Run the AI news demo on the reader test computer at P1, then count
        Texture's issued controls and the cast records whose packet names
        them.
      proves: every Texture dispatch is on the ledger
      evidence_class: deployed proof
    - action: compare the demo's research rounds and brief with S1i
      proves: no desk behavior changed (P1 changes no surface)
      evidence_class: deployed comparison
  rollback: git revert; records already minted stay (append-once) and nothing reads them yet
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: every Texture dispatch is a record a reader can count, at no change in behavior
  goodharting_would_be: records minted in a second commit that can diverge from the packet; records without the reserved fields

homotopy:
  realism_axis: unit test of the accumulator -> lifecycle replay -> one demo on staging -> trace audit

boundaries:
  mutation_class: red (lifecycle reducer, evidence ledger)
  authority_sources:
    - owner 2026-10-11 (protocol direction and plan approval)
    - docs/precommitment-protocol-design-2026-10-11.md
  must_preserve:
    - a control commits its record, packet and work item together or not at all
    - a replayed command mints nothing new
    - no desk prompt or surface changes
    - a crash restart never resumes work
  excluded: [surface changes (P2), predictions (P3), scoring (P4)]
  protected_surfaces: [internal/store/lifecycle_control_issue.go, internal/types/commitment.go]

now:
  status: drafted
  slice: P1a — the record in the control commit
  source_ref: f1be9e2f
  deploy_identity: staging 25c8b5b0
  candidate: none
  conjecture:
    id: P1-1
    claim: the control accumulator can mint each control's record in its existing conditional batch, with the packet bound to it
    test: store test of IssueLifecycleControl plus replay; then the staging trace audit
    edge: missing_oracle
    delta_o: records per control in the store after a command and after its replay
    scope_if_supported: Texture controls on one computer
    status: proposed
    evidence_refs: []
  decision:
    what: a control's record is a directive of subtype cast (open or continue work), addressed to the target agent
    kind: architecture
    status: provisional (design §2; an Ask-shaped question packet stays a cast until P2 splits them)
    evidence_ref: docs/precommitment-protocol-design-2026-10-11.md §2
  belief:
    believed_state: dispatch is off-ledger
    main_uncertainty: whether any reader of packets breaks when SourceRecordID is set on control packets
    next_observation: the file review, then the store test
  blocker_or_risk: none
  next_action: file review; then write the ways P1 can fail and the store test, then the code

receipts: []
---

# P1 — record-first controls

Texture dispatches through controls, and none of them is on the ledger.
P1 makes the existing control command mint a cast record for each control,
inside the same commit, and binds the control's packet to it, as
CommitLifecycleAct already does for the other acts. Nothing a desk sees
changes.

Ways it can fail, written before the code:

- the record commits in a separate step and can diverge from the packet
  (it must ride the same conditional batch);
- a replay mints a second record (deterministic record id from the
  command and control ids, with a not-exists condition);
- a packet reader treats SourceRecordID as "record-native report" and
  routes the control differently (find every reader of SourceRecordID
  before setting it);
- the record's body copies the whole packet and grows the ledger (keep the
  objective, the question summaries and a reference to the packet);
- the reserved fields are added in a way older readers reject (additive,
  omitempty);
- the crash-boot phase treats the new records as open work (records are
  not work items; check the boot closers).
