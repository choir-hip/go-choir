---
definition_version: 4

# P1 — record-first controls. Station of
# definitions/choir-precommitment-protocol-metamission-2026-10-11.md;
# design docs/precommitment-records.md §7 P1 and §10.
readiness: reviewed

review:
  reviewer: agentic-consensus file review (two rounds)
  frozen_ref: d30024a1 (round two), revised after it
  verdict: accept
  evidence_ref: receipts P1-review1, P1-review2

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
    route); the record fills the existing provenance fields (AgentID, and
    ModelID where the run records one) and gains only what is missing
    (additive, omitempty: emitter kind, computer id, subject refs); the
    desk-pack record query excludes directive records in the query
    (JSON match on $.directive empty, missing matches empty) rather than
    after it; a read-only internal guest route that lists a document's
    control packets with their source record ids and the records they
    name, for the staging audit (orange, read-only); and the audit itself,
    a one-to-one join.
  acceptance:
    - action: >-
        Store tests: a multi-control command mints one record per control in
        the same batch; a failed batch commits none of them; a replay mints
        nothing; both entry points mint; an owner-issued control records
        the owner as emitter.
      proves: the record and its packet commit together or not at all
      evidence_class: unit (store, real Dolt)
    - action: >-
        On one ledger snapshot S with more than 256 records, including
        older directives, compare the old pack pipeline (newest 256 of any
        kind, then drop directives) on S against the new pipeline (drop
        directives in the query) on S plus 300 cast records, at a fixed
        clock. The supervision projection and Texture's evidence read are
        identical; the acting pack differs only by admitting older
        non-directive records that directives used to push out, and the
        test names that widening as the one intended change.
      proves: P1 changes nothing a desk or the owner sees except the intended pack widening
      evidence_class: unit (deterministic)
    - action: >-
        Run the AI news demo on the reader test computer at P1; through the
        new read-only route, join every control packet issued after the
        deploy to its record by SourceRecordID, checking addressee,
        emitter, scope and reserved fields, one to one.
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
    - docs/precommitment-records.md
  must_preserve:
    - a control commits its record, packet and work item together or not at all
    - a replayed command mints nothing new
    - no desk prompt or surface changes
    - a crash restart never resumes work
  excluded: [surface changes (P2), predictions (P3), scoring (P4)]
  protected_surfaces: [internal/store/lifecycle_control_issue.go, internal/types/commitment.go]

now:
  status: working
  slice: P1 landed and audited on staging; completion panel waits for the OpenAI reviewers
  source_ref: 0f1c86c7 (pushed; CI 38157484614 green)
  deploy_identity: staging 0f1c86c7; reader computer restarted onto it at 17:06:38Z
  candidate: {id: P1, state: landed, ref: main, base: 3cee2ac9, digest: 0f1c86c7, scope: [internal/store, internal/types, internal/agentcore, internal/textureowner, internal/apihandler]}
  conjecture:
    id: P1-1
    claim: the control accumulator can mint each control's record in its existing conditional batch, with the packet bound to it
    test: store test of IssueLifecycleControl plus replay; then the staging trace audit
    edge: missing_oracle
    delta_o: records per control in the store after a command and after its replay
    scope_if_supported: Texture controls on one computer
    status: "supported (thin sample: one control on staging)"
    evidence_refs: [store tests in internal/store/lifecycle_control_record_test.go, receipt P1-deployed]
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
    evidence_ref: docs/precommitment-records.md §2
  belief:
    believed_state: every control since the deploy minted its cast record (one control, one record, all checks pass)
    main_uncertainty: the sample is one control, because research was down on the ChatGPT limit
    next_observation: P2a's deployed demo repeats the same join with every research round
  blocker_or_risk: >-
    not P1: the demo's first Texture run took no turn for eight minutes
    (docs/problems/texture-raced-activation-refused-first-turn-2026-10-11.md,
    named residual)
  next_action: completion panel once the OpenAI reviewers return (after 18:59Z); P2a's demo widens the sample

receipts:
  - id: P1-review1
    at: 2026-10-11T16:21Z
    kind: agentic-consensus file review (session scratchpad p1/review)
    result: >-
      Sent back 4 to 3 (Claude, Codex, Gemini, GPT-6 Luna send back;
      DeepSeek, GLM, GPT-6.1 Sol accept). The desk pack's newest-256 window
      would be crowded by cast records; the owner's management-open control
      would be recorded as Texture; the accumulator has two entry points;
      the Cast/Ask rule was unstated; the reserved fields fell short of
      design §10; the acceptance compared noisy demo output. Revised in
      d30024a1.
  - id: P1-review2
    at: 2026-10-11T16:36Z
    kind: agentic-consensus file review (session scratchpad p1/review2)
    result: >-
      Accepted 3 to 1 among valid verdicts (DeepSeek, Gemini, GLM accept;
      Claude sends back). Codex, GPT-6 Luna and GPT-6.1 Sol failed: the
      OpenAI account reached its usage limit. Claude's findings were file
      edits and are applied: compare the old pack pipeline against the new
      one at a fixed clock; reuse the provenance agent and model fields
      (the model id has no source on a run, a named residual); add a
      read-only route for the staging audit; the record's emitter is
      authoritative over the packet's transport fields; keep the packet id
      as the control id; filter directives on $.directive with missing
      fields matching empty; keep the emitter field omitempty.
  - id: P1-deployed
    at: 2026-10-11T17:36Z
    kind: landing loop and deployed audit
    result: >-
      Pushed 0f1c86c7; CI 38157484614 green; staging deployed 0f1c86c7;
      reader test computer demo-arxiv-2026-10-10c restarted onto it at
      17:06:38Z. AI news demo with the follow-up (session scratchpad
      demo18). Audit through GET /internal/runtime/control-records: one
      control after the deploy (17:16:00Z, research:00f36207…), one cast
      record control:72f48fce3290de5aeb0cdd16a3b1528d, addressed to the
      target, emitter agent, computer set, subject refs naming the
      control, zero failures. The sample is one control: Texture's first
      run took no turn (problems/texture-raced-activation-refused-first-turn-2026-10-11.md)
      and every research run failed on the ChatGPT usage limit
      (problems/chatgpt-usage-limit-stops-research-and-management-2026-10-11.md),
      so no later rounds were dispatched. Neither failure involves P1.
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
- the packet id changes: copying the RN3 pattern literally
  (recordID + ":packet") would break control bindings, the turn record's
  ControlUpdateIDs and the owner route's wake. The packet keeps
  UpdateID = control id, and the record id derives from the command and
  control ids;
- the directive filter misses older records with no explicit kind (the
  list query supports equality only; filter on $.directive being empty,
  with missing fields matching empty);
- the new emitter field on the control request changes the command digest
  of existing commands, and replaying a pre-P1 command conflicts (the
  field is omitempty and empty by default);
- an owner-issued control is attributed two ways (the record's emitter is
  authoritative for who spoke; the packet's Role and AgentID stay the
  transport caller, and P1 does not change them);
- P1 casts carry no prediction, though design §2 says a cast requires one
  (P3 enforces it; until then a cast without a prediction is expected);
- "content hash" means a field that hashes itself (it means the store
  object's existing content hash; nothing new);
- the commit time is the process clock rather than the commit (the record
  takes the lifecycle receipt's time from the same commit);
- each report's resolution scan (up to 10,000 records,
  mechanicalResolveForReport) can miss a resolution outside its window
  once records pass 10,000, and mint a second "answered" record. Cast
  records bring that point closer (named correctness residual; P3
  replaces the scan with targeted lookups);
- the run carries no model id (RunRecord has no model field), so ModelID
  stays empty as it does for existing acts (named residual);
- the reserved fields are added in a way older readers reject (additive,
  omitempty);
- the crash-boot phase treats the new records as open work (records are
  not work items; check the boot closers).
