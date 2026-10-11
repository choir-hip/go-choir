---
definition_version: 4

# P2 — one surface. Station of
# definitions/choir-precommitment-protocol-metamission-2026-10-11.md;
# design docs/precommitment-protocol-design-2026-10-11.md §2, §3, §7 P2.
readiness: drafted

review:
  reviewer: agentic-consensus file review (pending)
  frozen_ref: pending
  verdict: pending
  evidence_ref: pending

start:
  captured_at: 2026-10-11T17:25:00Z
  source:
    canonical_ref: main 0f1c86c7 (P1 landed)
    deploy_identity: staging choir.news 0f1c86c7
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty (P1 now card, this draft, one problem doc)
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Texture dispatches through ApplyTexture controls (open_researcher,
      open_persistent_super, target_work_item_id;
      internal/textureowner/tools_texture.go), taught in
      internal/agentcore/tool_profiles.go and the cell author's decode
      error (internal/textureowner/texture_cell_author.go). Since P1 each
      control mints a cast record in its commit, so the ledger sees them,
      but Texture still has a second verb set no other desk has.
    - >-
      Texture's module set (internal/yaegikernel/choir.go deskModuleSets)
      has Ask but not Cast. Every other desk has Cast.
    - >-
      A Cast commits through CommitLifecycleAct and reaches only an agent
      that already exists: directiveTargetAgentID resolves a bare profile
      to profile:<channel doc> (management to the persistent super), and
      validation requires the packet target to match. No act path opens a
      desk's work. Only the control accumulator does that
      (internal/store/lifecycle_control_issue.go).
    - >-
      PD1 already names a document's research desk research:<doc>
      (researchDeskAgentID, internal/textureowner/texture_turn_runtime.go),
      the same id the act resolver derives. The address schemes agree.
    - >-
      Message, Emit and Assign remain on management, engineering and
      research; Message and Emit on Texture. Emit writes an envelope
      outside the cell tray (pendingCellEmits, internal/agentcore/tools_desk.go).
      No default prompt teaches Message or Emit by name.
    - >-
      Management's ProductAPI allowlist admits /api/texture/
      (internal/agentcore/tools_product_api.go validateProductAPIRoute),
      which includes the routes that write a Texture's content.
    - >-
      Texture bounds research rounds per owner request (four,
      textureResearchRoundsPerOwnerRequest) by counting research controls
      since the latest owner input. A Cast path must keep that bound.

finish:
  deliver: >-
    Texture dispatches with Cast and Ask like every desk. A Cast to a desk
    with no open work on its subject opens that work; a Cast or Ask to a
    desk with open work continues it. ApplyTexture carries document edits
    and decide turns only. Message and Emit are gone from every desk.
    Management's ProductAPI cannot write a Texture. Any same-owner desk may
    address texture:<doc>.
  artifact: >-
    Slice P2a: an open-or-continue step inside CommitLifecycleAct for Cast
    and Ask, moved from the control accumulator (agent mint, work item,
    packet, events, one conditional batch, with the P1 record as the act's
    own record); Cast on Texture's module set; ApplyTexture refuses
    controls with a message that names Cast and Ask; the research-round
    bound counts research casts; Texture's control prose in
    tool_profiles.go and the cell author's decode error rewritten to Cast
    and Ask. Slice P2b: Message, Emit and Assign's open decision (design §9
    item 3, default: Assign stays for engineering only) removed from
    deskModuleSets and the reducer; ProductAPI refuses Texture-write
    routes (revisions, revise, restore, accept-merge) for every desk;
    texture:<doc> addressable by any same-owner desk, delivered to the
    Texture desk's mailbox as a directive packet.
  acceptance:
    - action: >-
        Store tests for P2a: a Cast to an unopened research:<doc> opens the
        agent, the work item and the packet with its record in one batch;
        a second Cast continues the same work item; a refused batch commits
        nothing; a replay mints nothing; a Cast past the round bound is
        refused with a visible reason and mints no record.
      proves: Cast replaces all three controls with the same atomicity
      evidence_class: unit (store, real Dolt)
    - action: >-
        Reducer test: Texture's staged ApplyTexture with controls is
        refused, and the refusal text names Cast and Ask.
      proves: the old surface cannot be used by accident
      evidence_class: unit
    - action: >-
        Run the AI news demo with the follow-up on the reader test
        computer at P2a; the P1 audit route shows every research dispatch
        as a cast record from a Texture run through CommitLifecycleAct and
        zero control-accumulator packets after the deploy; the brief is
        written and research reports arrive.
      proves: Texture dispatches through the one surface on staging
      evidence_class: deployed proof
    - action: >-
        At P2b, a management cell that calls ProductAPI on a Texture-write
        route is refused; Message and Emit are not in any desk's surface
        listing; a management Cast to texture:<doc> wakes that Texture.
      proves: the side channels are gone and texture addressing works
      evidence_class: unit plus one staged cell on staging
  rollback: >-
    git revert per slice; records already minted stay (append-once).
    P2a's revert restores the controls; P2b's revert restores the verbs.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: one verb set for every desk, every dispatch a record minted by the act itself
  goodharting_would_be: Cast wrapping the old control path; deleted verbs left reachable through another carrier

homotopy:
  realism_axis: store tests of open-or-continue -> reducer refusal -> demo on staging (P2a) -> deletions with a staged cell (P2b)

boundaries:
  mutation_class: red (lifecycle reducer, desk surface, Texture authority)
  authority_sources:
    - owner 2026-10-11 (protocol direction and plan approval)
    - docs/precommitment-protocol-design-2026-10-11.md
  must_preserve:
    - Texture is the sole writer of a document's revisions
    - a Cast's record, agent, work item and packet commit together or not at all
    - the research-round bound per owner request
    - a crash restart never resumes work
    - messages already in flight drain on their old kinds, never re-armed
  excluded: [predictions required (P3), scoring (P4), Autopaper (P5), the owner's own product routes]
  protected_surfaces: [internal/store/lifecycle_commit_act.go, internal/store/lifecycle_control_issue.go, internal/agentcore/rlm_reduce.go, internal/yaegikernel/choir.go, internal/agentcore/tools_product_api.go]

now:
  status: drafted
  slice: P2a — Cast opens or continues work
  source_ref: 0f1c86c7
  deploy_identity: staging 0f1c86c7
  candidate: none
  conjecture:
    id: P2-1
    claim: >-
      The control accumulator's open step can move under CommitLifecycleAct
      for Cast and Ask without changing what research receives, so Texture
      loses its controls and the research brief is no worse.
    test: store tests, then the AI news demo on staging at P2a
    edge: missing_oracle
    delta_o: control-accumulator packets after the deploy (expect zero) and research reports per run against P1's demo
    scope_if_supported: Texture's research and management dispatch on one computer
    status: proposed
    evidence_refs: []
  decision:
    what: design §9 item 3 (Assign) takes its recommended default, kept for engineering only, until the owner answers
    kind: architecture
    status: provisional
    evidence_ref: docs/precommitment-protocol-design-2026-10-11.md §9
  belief:
    believed_state: every Texture dispatch is a record (P1), but through a verb only Texture has
    main_uncertainty: whether opening work inside CommitLifecycleAct needs the accumulator's run-binding fields (ControlUpdateIDs on the turn record) that Texture's decide path reads
    next_observation: the file review
  blocker_or_risk: none
  next_action: file review by agentic consensus; then store tests first

receipts: []
---

# P2 — one surface

After P1 every Texture dispatch is a record, but Texture still dispatches
through controls that no other desk has. P2 gives Texture Cast and Ask,
makes Cast open a desk's work when none is open, and then deletes the side
channels: Message, Emit, and management's ProductAPI writes to a Texture.

Ways it can fail, written before the code:

- Cast opens work in a second commit after its record (the open step must
  ride CommitLifecycleAct's one conditional batch);
- a Cast to a desk with open work opens a second work item instead of
  continuing (look up open work for the subject inside the batch, with a
  condition that fails on a race);
- two Casts in one cell to the same unopened desk open it twice (the
  second must see the first's staged open, or the batch must refuse it);
- the research-round bound stops counting, because it counts controls
  and research now arrives as casts (count casts addressed to the
  document's research desk since the latest owner input);
- Texture's decide path reads the turn record's ControlUpdateIDs to bind
  reports to the control that asked, and casts leave it empty (find
  every reader of ControlUpdateIDs before removing controls);
- research's report path requires a delivered control to bind its report
  (internal/store/lifecycle_commit_act.go producer reports check
  control.TargetWorkItemID); a cast-opened work item must satisfy it;
- ApplyTexture keeps accepting controls from old prompts or memory (refuse
  with a message naming Cast and Ask; teach Cast in tool_profiles.go);
- deleting Emit loses its latency use (a record's packet already wakes the
  addressee; confirm with a timing on staging);
- management keeps writing Textures through another allowlisted path
  (refuse by route shape, test each Texture-write route);
- texture:<doc> from another desk becomes a write path (it delivers a
  directive packet; only the Texture desk writes);
- in-flight controls from before the deploy are re-armed or orphaned (they
  drain on the old kind; a crash closes them as usual);
- the open step's agent mint for management differs from the
  accumulator's (persistent super id per owner, not per document).
