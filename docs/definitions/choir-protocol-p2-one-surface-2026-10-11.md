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
      The runtime overlays teach Message: engineering reports progress to
      management with it (internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml,
      pinned by tool_profiles_authority_test.go), management lists it as a
      coordination verb, research names it retired while its module set
      grants it. Broker action grants (ActionMessage, ActionEmit), the Emit
      egress handler and the rlm replay goldens also carry them.
    - >-
      Texture is taught its controls mainly by internal/textureprompts
      (overlays/run_system.yaml, with a worked open_researcher example;
      overlays/revision_policy.yaml; overlays/revision_execution_findings.yaml;
      texture.yaml), served through textureprompts.RunOverlay(); the
      paragraph in tool_profiles.go is a secondary addendum.
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
    with no open work on the request opens it; a Cast or Ask to a desk with
    open work continues it. ApplyTexture carries document edits and decide
    turns only. No desk can write a Texture through ProductAPI. No desk can
    stage Message or Emit. Any same-owner desk may address texture:<doc>,
    and the Texture desk wakes on it.
  artifact: >-
    Slice P2a (Texture's verbs). The reducer folds a Texture cell's staged
    Cast and Ask addressed to research or management into that cell's
    staged ApplyTexture body (apply or decide) before the intent loop, so
    they never reach commitLifecycleActIntent (whose post-commit
    delegated-cast step would open an engineering assignment) and commit in the same
    ApplyTextureTurn through textureTurnControls and the P1 accumulator: one
    transaction path, the edit and its dispatch atomic, the research-round
    bound, opener merge and management's execution_request check unchanged.
    Cast(research, objective, packet) and Ask(research, question) become a
    research opener (a follow-up when the request already has open research
    work); Cast(management, objective, execution_request) becomes the
    persistent-management opener. A Texture cell that stages Cast or Ask
    without ApplyTexture is refused with a message saying the dispatch
    commits with an apply or decide turn. The cell body's controls field is
    refused with a message naming Cast and Ask. Each control's record gains
    subject ref staged_act (cast or ask). Cast joins Texture's module set.
    Every surface that teaches the controls is rewritten to Cast and Ask:
    internal/textureprompts (run_system.yaml with its worked example,
    revision_policy.yaml, revision_execution_findings.yaml, texture.yaml),
    tool_profiles.go and the cell author's decode error; a test fails if
    any served Texture prompt names open_researcher, open_persistent_super
    or target_work_item_id.
    Slice P2b (ProductAPI fence). For every desk, ProductAPI refuses each
    non-GET /api/texture/ route, with an explicit exception list (empty:
    no desk use of a Texture write through ProductAPI appears in the code
    or the evidence record; management starts a new Texture through
    /api/prompt-bar, the owner's path, where Texture writes it), and a
    test enumerates the registered Texture routes (internal/textureowner/router.go,
    internal/apihandler/routes.go); management's overlay
    (internal/runtimeprompts/overlays/rlm_management_runtime.yaml) stops
    listing /api/texture/ as writable. Slice P2c (deletions and
    addressing). Message and Emit leave deskModuleSets and every prompt
    that teaches them (the management, engineering and research runtime
    overlays), from the broker action grants and the Emit egress handler,
    so no new cell can stage them. Engineering's progress to management
    moves from Message to Note addressed to management (a record-native
    directive packet, which management already wakes on); the overlay and
    its pinning test change with it. The reducer keeps their
    branches only so a tray staged before the deploy still replays (named
    residual, removed after one release). Assign's module entries are
    unchanged: design §9 item 3 recommends folding Assign into Cast, and
    P2 defers that (a named deviation, since it touches engineering
    delegation and replay goldens), pending the owner's answer.
    Slice P2d (texture addressing). A new Texture actor occurrence kind for
    a directive packet addressed to texture:<doc> by a same-owner desk,
    resolved by the Texture controller (texture_controller.go
    ResolveTextureActorOccurrence, today producer_report and
    document_revision only), so the Texture desk wakes on it and decides
    what to do; a cross-owner address is refused.
  acceptance:
    - action: >-
        Reducer and store tests for P2a: a Texture cell with ApplyTexture
        and a Cast to research commits one turn whose control opens the
        research desk and its work with a cast record carrying
        staged_act=cast; a second Cast in a later turn of the same request
        continues that work; two Casts in one cell merge into one round; a
        Cast past the round bound is dropped with the runtime note and the
        edit still commits; a Cast to management without execution_request
        is refused; a Cast with no ApplyTexture is refused; a body with
        controls is refused naming Cast and Ask; replay mints nothing.
      proves: Cast and Ask replace the controls on the same transaction path
      evidence_class: unit (reducer, store, real Dolt)
    - action: >-
        AI news demo with the follow-up on the reader test computer at P2a.
        Through the P1 audit route, every control packet created after the
        deploy joins one to one to a cast record whose staged_act is set,
        except controls whose record names the owner as emitter (the owner's
        management-open route keeps the accumulator directly). The controls
        field no longer parses, so an unset staged_act on an agent-emitted
        record would mean a path was missed. Research wakes, reports, and
        Texture incorporates at least one report.
      proves: Texture dispatches through Cast and Ask on staging, and the research chain still closes
      evidence_class: deployed proof (needs research inference; see blocker)
    - action: >-
        P2b: the enumeration test fails if a registered non-GET Texture
        route is reachable through ProductAPI outside the exception list.
      proves: no desk writes a Texture through the product API
      evidence_class: unit
    - action: >-
        P2c: no desk's surface listing or broker grant contains Message or
        Emit; a cell calling either is refused with a message naming the
        act to use; a tray staged with Message before the change still
        replays (the replay goldens keep passing); an assigned engineering
        desk's Note to management wakes management with the progress
        packet; no served prompt names Message or Emit.
      proves: the side channels are gone without stranding in-flight work
      evidence_class: unit
    - action: >-
        P2d: a management Cast to texture:<doc> of the same owner wakes
        that Texture with the directive in its occurrence; the Texture turn
        can answer it with a decide or apply turn; another owner's address
        is refused.
      proves: any desk can feed a Texture without writing it
      evidence_class: unit plus one staged cell on staging
  rollback: >-
    git revert per slice; records already minted stay (append-once). P2a's
    revert restores the controls field; P2b's restores the allowlist; P2c's
    restores the verbs; P2d's removes the occurrence kind.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: one verb set for every desk, every dispatch a record, one transaction path per dispatch
  goodharting_would_be: >-
    a second transaction path for dispatch (two ways to open work); deleted
    verbs reachable through another carrier. A control-direction delivery
    packet produced from a staged Cast is not a side channel: the record is
    the act, and the packet is its delivery

homotopy:
  realism_axis: reducer and store tests -> demo on staging (P2a) -> enumeration test (P2b) -> deletions (P2c) -> texture addressing with a staged cell (P2d)

boundaries:
  mutation_class: red (lifecycle reducer, desk surface, Texture authority)
  authority_sources:
    - owner 2026-10-11 (protocol direction and plan approval)
    - docs/precommitment-protocol-design-2026-10-11.md
  must_preserve:
    - Texture is the sole writer of a document's revisions
    - a dispatch's record, agent, work item and packet commit with the turn or not at all
    - the research-round bound per owner request, and opener merge within a turn
    - management openers carry execution_request actions and an arrival ordinal
    - a crash restart never resumes work
    - messages already in flight drain on their old kinds, never re-armed
  excluded: [predictions required (P3), Ask's own record shape resolved on Reply (P3), scoring (P4), Autopaper (P5), the owner's own product routes, Cast opening work for desks other than Texture]
  protected_surfaces: [internal/agentcore/rlm_reduce.go, internal/textureowner/texture_cell_author.go, internal/textureowner/texture_turn_runtime.go, internal/textureowner/texture_controller.go, internal/store/lifecycle_control_issue.go, internal/yaegikernel/choir.go, internal/agentcore/tools_product_api.go, internal/runtimeprompts/overlays]
  read_only_dependencies: [internal/agentcore/management_controller.go, internal/agentcore/texture_lifecycle_api.go, internal/agentcore/tools_worker_update.go (P2a must not change them; a diff there means the fold leaked)]

now:
  status: working
  slice: P2a — Texture's Cast and Ask fold into its turn
  source_ref: 0f1c86c7
  deploy_identity: staging 0f1c86c7
  candidate: none
  conjecture:
    id: P2-1
    claim: >-
      Texture's staged Cast and Ask can fold into its turn's controls with
      no change to what research receives, so Texture loses its second verb
      set and the research chain closes as before.
    test: reducer and store tests, then the AI news demo on staging at P2a
    edge: missing_oracle
    delta_o: control packets after the deploy without staged_act (expect zero); research reports incorporated per run against P1's demo
    scope_if_supported: Texture's research and management dispatch on one computer
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      Texture's Cast compiles into its turn rather than into
      CommitLifecycleAct. The research chain (activation, delivery binding,
      report fold, Texture's report acceptance) requires control-direction
      packets with a target work item, and a turn commits the edit with
      its dispatch; CommitLifecycleAct does neither. The research chain's
      requirements live in management_controller.go (activation as
      lifecycle_texture_control with work_item_ids), texture_lifecycle_api.go
      (ValidateLifecycleProducerReportAuthority), tools_worker_update.go
      (requester provenance) and texture_controller.go (control binding);
      the fold leaves all of them untouched. Dissent (Gemini, P2-review1):
      design §1-§2 make dispatches records with derived directive packets,
      so the chain should learn directive packets instead. Answer: the
      design's invariant is that the record is the act and the only way one
      agent reaches another; the packet's direction is delivery machinery.
      Teaching five runtime sites a second activation shape is the second
      transaction path the value criterion forbids. A later station may
      unify packet directions once nothing else depends on the difference.
      Design §9 item 3 (Assign): the recommendation is to fold Assign into
      Cast; P2 defers it as a named deviation until the owner answers
    kind: architecture
    status: provisional (P2-review1)
    evidence_ref: receipts P2-review1
  belief:
    believed_state: every Texture dispatch is a record (P1), through a verb only Texture has
    main_uncertainty: whether models stage Cast with an apply or decide turn reliably, or keep staging Cast alone
    next_observation: the second file review
  blocker_or_risk: >-
    the P2a deployed proof needs research inference, which is down while
    the ChatGPT account is at its usage limit
    (docs/problems/chatgpt-usage-limit-stops-research-and-management-2026-10-11.md)
  next_action: second file review; then P2a tests first

receipts:
  - id: P2-review1
    at: 2026-10-11T17:35Z
    kind: agentic-consensus file review (session scratchpad p2/review), frozen 42dd14f2
    result: >-
      Sent back 4 to 0 among valid verdicts (Claude, DeepSeek, Gemini, GLM;
      Codex, GPT-6 Luna and GPT-6.1 Sol failed on the OpenAI usage limit).
      Shared finding: the research chain (activation, delivery binding,
      report fold, Texture's report authority) requires control-direction
      packets with a target work item, so a Cast through
      CommitLifecycleAct's directive packet would open work research never
      wakes for and whose reports Texture refuses. Revised: Texture's Cast
      and Ask fold into its turn's controls (one path, bound and merge
      unchanged). Also: Texture prompts in internal/textureprompts;
      engineering's Message-based progress; the delegated-cast step;
      Texture has no wake for directive packets (texture addressing split
      to P2d); the ProductAPI list was incomplete (refuse every non-GET
      route); the owner's management-open in the audit; Assign's §9
      recommendation is fold, so keeping it is a deviation;
      ControlUpdateIDs has no readers. Gemini dissents on packet
      direction; recorded in the decision.
---

# P2 — one surface

After P1 every Texture dispatch is a record, but Texture still dispatches
through controls that no other desk has. P2 lands in four slices. P2a gives
Texture Cast and Ask and folds them into its turn, so the dispatch keeps the
one transaction path P1 put a record on. P2b fences management's ProductAPI
off every route that writes a Texture. P2c removes Message and Emit
from every desk. P2d gives the Texture desk a way to wake on a directive
addressed to texture:<doc>, so any desk of the same owner can feed it.

Ways it can fail, written before the code:

- Cast opens work on a second transaction path that can diverge from the
  turn (it folds into the turn's controls; there is no other path);
- the research chain stops closing: research activation, delivery binding,
  the report fold and Texture's report acceptance all require a
  control-direction packet with a target work item (management_controller.go
  selectLifecycleControlActivation, lifecycle_control_delivery.go,
  lifecycle_commit_act.go producer reports, texture_controller.go control
  binding). The fold keeps the packet exactly as the controls made it;
- the round bound or the opener merge stops applying (both stay in
  textureTurnControls, which every folded dispatch passes through);
- a refused dispatch aborts the turn's edit (an over-budget opener is
  dropped with the runtime note, as today; only a malformed Cast refuses
  the cell);
- a Cast staged without ApplyTexture is silently dropped (refused with a
  message, so the model learns to pair it);
- a staged Cast is folded twice on cell replay (the fold is a pure
  function of the cell's staged intents; the turn identity is unchanged);
- management loses its arrival ordinal or its execution_request
  activation (the persistent-management opener is unchanged);
- Texture's existing Ask to research, which today mints a directive packet
  research never wakes for, keeps that path (Texture's Ask to research or
  management folds like Cast);
- ApplyTexture keeps accepting controls from old prompts or memory
  (refused, naming Cast and Ask);
- ProductAPI keeps a Texture-write route the list missed (refuse every
  non-GET Texture route, enumerate the registered routes in a test);
- texture:<doc> from another desk becomes a write path (it delivers a
  directive packet; only the Texture desk writes);
- texture:<doc> delivers a packet the Texture desk never wakes on (the
  controller resolves only producer reports and document revisions today;
  P2d adds the occurrence kind);
- a Texture Cast reaches commitLifecycleActIntent and its post-commit
  delegated-cast step opens an engineering assignment, or fails the cell
  after the record committed (the fold removes Texture's Casts and Asks
  from the intent loop before it runs);
- the audit counts the owner's management-open controls as a missed path
  (their records carry emitter owner and are excepted by name);
- a tray staged with Message before P2c fails to replay after it (the
  reducer branches stay one release);
- engineering is still told to report progress with Message after P2c and
  loses a cell every turn (the overlay and its pinning test move to Note);
- Texture keeps authoring controls because its main prompt still teaches
  them with a worked example (a test scans every served Texture prompt);
- two actors open the same desk's work at once (only Texture dispatches
  this way, and its turns are head-pinned per document, so they
  serialize; the accumulator's agent mint carries a not-exists condition);
- the fold drops the management opener's arrival ordinal (the
  persistent-management opener path is unchanged, ordinal included);
- deleting Emit loses its latency use (a record's packet already wakes the
  addressee; confirm with a timing on staging);
- in-flight controls from before the deploy are re-armed or orphaned (they
  drain on the old kind; a crash closes them as usual).

ControlUpdateIDs on the turn record has no readers (only its two writers),
so it carries no binding P2 must preserve.
