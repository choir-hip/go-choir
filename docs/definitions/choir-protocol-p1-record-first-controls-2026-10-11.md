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
      Texture's controls (open_researcher, open_persistent_super,
      target_work_item_id) reach the store through two commands that share
      one accumulator (lifecycleControlAccumulator,
      internal/store/lifecycle_control_issue.go): ApplyTextureTurn
      (internal/store/texture_turn.go) for controls inside a Texture turn,
      and IssueLifecycleControl for controls issued outside one. The
      accumulator mints, per control, the target agent when opening
      research, the work item, the source packet and lifecycle events, all
      in one conditional batch. No commitment record is minted.
    - >-
      The owner's management-open route (internal/textureowner/management_open.go,
      reason "owner_management_open") also calls IssueLifecycleControl,
      with a Texture run as the caller. That control is the owner's act,
      not Texture's.
    - >-
      Readers of SourceRecordID (management_controller.go, lifecycle_control_delivery.go,
      lifecycle.go) act only on directive-direction packets; control
      packets keep their own direction. Directive records are excluded from
      claims, tallies and the materiality projection (ResolveCommitments,
      internal/types/commitment_views.go). But the desk pack reads the
      newest 256 records of any kind before that filter
      (internal/agentcore/tools_desk.go, ListCommitmentRecords), so new
      records compete for that window.
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
    Each control mints one cast record, addressed to the control's target
    agent, in the same commit as its packet, and the packet names the record
    as its source. A control the owner issued records the owner as its
    emitter, not Texture. Records carry the fields design §10 reserves.
    What a desk or the owner sees does not change: packs, tallies and the
    supervision projection come out the same with and without these
    records.
  artifact: >-
    The accumulator mints the record for both commands; an explicit emitter
    on the control request (agent by default, owner for the management-open
    route); CommitmentRecord gains the reserved fields (additive JSON:
    emitter kind, computer id, agent id, model id from the caller's run,
    subject refs); the desk-pack and supervision record queries exclude
    directive records in the query rather than after it; a trace audit
    that joins each control packet to its record one to one.
  acceptance:
    - action: >-
        Store tests: a multi-control command mints one record per control in
        the same batch; a failed batch commits none of them; a replay mints
        nothing; both entry points mint; an owner-issued control records
        the owner as emitter.
      proves: the record and its packet commit together or not at all
      evidence_class: unit (store, real Dolt)
    - action: >-
        On one ledger snapshot, each desk's acting pack, issuer tallies and
        the supervision projection are identical with and without 300 added
        cast records.
      proves: P1 changes nothing a desk or the owner sees
      evidence_class: unit (deterministic)
    - action: >-
        Run the AI news demo on the reader test computer at P1; join every
        control packet issued after the deploy to its record by
        SourceRecordID, checking addressee, emitter, scope and reserved
        fields, one to one.
      proves: every Texture dispatch is on the ledger on staging
      evidence_class: deployed proof (the demo itself is a smoke test, not a comparison)
  rollback: >-
    git revert; records already minted stay (append-once). The query-level
    directive filter is what keeps them out of packs, so revert it last or
    not at all
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
    what: >-
      a control opens or continues work, so its record is a directive of
      subtype cast addressed to the target agent; the questions in its
      packet are the cast's instruction. Ask is reserved for a directed
      question that resolves on a Reply, which Texture gains in P2. The
      design's §7 wording ("a Cast or Ask per control") is corrected to
      match
    kind: architecture
    status: "settled for P1 (design §2 — Cast opens or continues work)"
    evidence_ref: docs/precommitment-protocol-design-2026-10-11.md §2
  belief:
    believed_state: dispatch is off-ledger
    main_uncertainty: whether any reader of packets breaks when SourceRecordID is set on control packets
    next_observation: the file review, then the store test
  blocker_or_risk: none
  next_action: second file review; then the store tests, then the code

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
- the owner's management-open control is recorded as Texture speaking
  (the request carries an explicit emitter; the owner route sets it);
- cast records crowd the desk pack's newest-256 window and change what a
  desk sees (exclude directives in the query, and prove packs are
  unchanged on a snapshot);
- only one of the two entry points mints (both share the accumulator;
  test both);
- "content hash" means a field that hashes itself (it means the store
  object's existing content hash; nothing new);
- the commit time is the process clock rather than the commit (the record
  takes the lifecycle receipt's time from the same commit);
- each report's resolution scan (up to 10,000 records,
  mechanicalResolveForReport) slows as records grow (named residual; P3
  replaces it with targeted lookups);
- the reserved fields are added in a way older readers reject (additive,
  omitempty);
- the crash-boot phase treats the new records as open work (records are
  not work items; check the boot closers).
