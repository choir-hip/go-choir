---
definition_version: 4

# P2 — one surface. Station of
# definitions/choir-precommitment-protocol-metamission-2026-10-11.md;
# design docs/precommitment-records.md §2, §3, §7 P2.
readiness: reviewed

review:
  reviewer: agentic-consensus file review (four rounds)
  frozen_ref: d1d83c39 (round four, with the P2a and P2b code)
  verdict: accept (round four DeepSeek accept; round three Gemini accept; Claude's last finding fixed with a test)
  evidence_ref: receipts P2-review1, P2-review2, P2-review3, P2-review4

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
      has Ask but not Cast. Every other desk has Cast (the research-child
      role does not).
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
      Message and Emit remain on management, engineering, research and
      Texture; Assign on engineering only. Emit writes an envelope
      outside the cell tray (pendingCellEmits, internal/agentcore/tools_desk.go).
      The runtime overlays teach Message: engineering reports progress to
      management with it (internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml,
      pinned by tool_profiles_authority_test.go), management lists it as a
      coordination verb, research names it retired while its module set
      grants it. Broker action grants (ActionMessage, ActionEmit) and the Emit
      egress handler also carry them; rlm_replay_linux_test.go stages
      choir.Message (the replay goldens in docs/evidence/rlm-replay do not).
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
    stage Message or Emit. Texture's Ask is answered by research's producer
    report, as its controls are today; Ask resolved on a Reply arrives with
    P3. Addressing texture:<doc> from another desk moves to its own station
    (P2d), because the Texture desk cannot yet dispose a directive packet.
  artifact: >-
    Slice P2a (Texture's verbs). The reducer folds a Texture cell's staged
    Cast and Ask addressed to research or management into that cell's
    staged ApplyTexture body (apply or decide) before the intent loop, so
    they never reach commitLifecycleActIntent (whose post-commit
    delegated-cast step would open an engineering assignment) and commit in the same
    ApplyTextureTurn through textureTurnControls and the P1 accumulator: one
    transaction path, the edit and its dispatch atomic, the research-round
    bound, opener merge and management's execution_request check unchanged.
    Cast(research, objective, packet), Ask(research, question) and
    Note(research, body) become a research opener (a follow-up when the
    request already has open research work); Cast(management, objective,
    execution_request packet) folds to the persistent-management opener, and
    textureTurnControls continues the request's open management work on the
    trajectory when there is some (a management branch beside research's
    follow-up branch, since the fold cannot see the lifecycle snapshot); Ask and Note to management are refused naming Cast, because a
    management dispatch needs execution_request actions. On Texture, Cast's
    third argument is a packet, not the cast spec other desks pass (named
    difference). The fold runs in commitTray, which both the commit path and
    recoverPartialActCommit reach, before the intent loop and before
    commitTextureAuthorIntent derives the revision identity. It validates
    in two phases: first the model-authored body, refusing one that carries
    controls (naming Cast and Ask), then the injection. It is a pure
    function of the staged intents, so a replayed cell folds to the same
    bytes. It refuses a Texture Cast whose third argument is a cast spec rather
    than a packet (no kind), naming the packet contract. When a dispatch is
    staged, it refuses a second Cast to management in one cell (a turn takes
    one persistent-management opener), refuses a cell with more
    than one ApplyTexture or with an email one carrying a dispatch, refuses
    a Texture Cast, Ask or Note to any desk other than research or
    management by name, and refuses a Texture Cast, Ask or Note staged
    without an apply or decide ApplyTexture, saying the dispatch commits
    with the turn. A refusal fails the whole cell, as a malformed body does
    today. Each control's record gains
    subject ref staged_act (cast, ask or note; joined with + when a turn's
    openers merge). Cast joins Texture's module set.
    Every surface that teaches the controls is rewritten to Cast and Ask:
    internal/textureprompts (run_system.yaml with its worked example,
    revision_policy.yaml, revision_execution_findings.yaml, texture.yaml),
    tool_profiles.go and the cell author's decode error; the sentences that
    become false are rewritten too (run_system.yaml and revision_policy.yaml
    say a direct Message or Note to research is rejected; after P2a a Note
    to research with a turn is valid); a test fails if any served Texture
    prompt names open_researcher, open_persistent_super or
    target_work_item_id.
    Slice P2b (ProductAPI fence). For every desk, ProductAPI refuses each
    non-GET /api/texture/ route, with an explicit exception list (one
    entry, POST merge-preview, which only computes a preview; no desk use
    of a Texture write through ProductAPI appears in the code or the
    evidence record; management starts a new Texture through
    /api/prompt-bar, the owner's path, where Texture writes it), and a
    test lists the registered Texture routes as samples, since the rule is
    by prefix and method and refuses a later route without the list
    (internal/textureowner/router.go,
    internal/apihandler/routes.go), naming POST /api/texture/management-open
    (the route the audit's owner exception depends on) and
    /api/texture/documents (registered apart); management's overlay
    (internal/runtimeprompts/overlays/rlm_management_runtime.yaml) stops
    listing /api/texture/ as writable. Slice P2c (deletions). Message and Emit leave every prompt that
    teaches them (the management, engineering and research runtime
    overlays; research's overlay also stops calling Cast retired while its
    module set grants it; Texture's prompts, where
    internal/textureprompts/prompts_test.go today requires choir.Message
    and choir.Note in a served prompt, and that test changes with them), the broker action grants and the Emit egress
    handler. Their exports stay as stubs that return a refusal naming the
    act to use, so a model that still calls them learns why instead of
    hitting an undefined symbol, and their Help() summaries in
    internal/yaegikernel/surface.go say they are retired and name the act.
    The live replay test (internal/agentcore/rlm_replay_linux_test.go) stages
    choir.Message in live cells; those rows stage the Message intent through
    the tray directly, so the replay of a pre-deploy tray stays observable. Engineering's progress to management
    moves from Message to ReportPacket, which is already in engineering's
    module set and keeps today's producer_report direction and work-item
    binding (commitAddressedPacketIntent); the overlay and its pinning test
    change with it. The reducer keeps their
    branches only so a tray staged before the deploy still replays (named
    residual, removed after one release). Assign's module entries are
    unchanged: design §9 item 3 recommends folding Assign into Cast, and
    P2 defers that (a named deviation, since it touches engineering
    delegation and replay goldens), pending the owner's answer.
    P2d (texture addressing) is a separate station with its own file: a
    directive addressed to texture:<doc> needs an occurrence kind, a
    postcondition branch, a way to arm a Texture with no open work, a
    disposition the Texture turn commits, a crash-boot close, a round
    bound, and the delegated-cast step gated to engineering addressees
    (P2-review2).
  acceptance:
    - action: >-
        Reducer and store tests for P2a: (P2b lands first, so the demo's
        audit cannot hide a management-open issued through ProductAPI.) A
        Texture cell with ApplyTexture
        and a Cast to research commits one turn whose control opens the
        research desk and its work with a cast record carrying
        staged_act=cast; a second Cast in a later turn of the same request
        continues that work; two Casts in one cell merge into one round; a
        Cast past the round bound is dropped with the runtime note and the
        edit still commits; a second management Cast in a later turn of the
        same request continues that request's open management work, and
        management work from an earlier request is not continued; a Cast to management without execution_request
        is refused; a Cast with no ApplyTexture is refused; a body with
        controls is refused naming Cast and Ask; replay mints nothing.
      proves: Cast and Ask replace the controls on the same transaction path
      evidence_class: unit (reducer, store, real Dolt)
    - action: >-
        AI news demo with the follow-up on the reader test computer at P2a.
        Through the P1 audit route, every control packet created after the
        deploy joins one to one to a cast record whose subject ref staged_act is set,
        except controls whose record names the owner as emitter (the owner's
        management-open route keeps the accumulator directly). The fold
        refuses a model-authored controls array, so an unset staged_act on an
        agent-emitted record would mean a path was missed. Research wakes, reports, and
        Texture incorporates at least one report.
      proves: Texture dispatches through Cast and Ask on staging, and the research chain still closes
      evidence_class: deployed proof (needs research inference; see blocker)
    - action: >-
        P2b: the enumeration test fails if a registered non-GET Texture
        route is reachable through ProductAPI outside the exception list.
      proves: no desk writes a Texture through the product API
      evidence_class: unit
    - action: >-
        P2c: no served prompt or broker grant names Message or Emit; a cell
        calling either gets the stub's refusal naming the act to use; a tray staged with Message before the change still
        replays (the replay goldens keep passing); an assigned engineering
        desk's ReportPacket to management reaches it as a producer report
        bound to the assignment's work item; no served prompt names Message
        or Emit.
      proves: the side channels are gone without stranding in-flight work
      evidence_class: unit
  rollback: >-
    git revert per slice; records already minted stay (append-once). P2a's
    revert restores the controls field; P2b's restores the allowlist; P2c's
    restores the verbs.
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
  realism_axis: enumeration test (P2b, lands first) -> reducer and store tests -> demo on staging (P2a) -> deletions (P2c); texture addressing is P2d, its own station

boundaries:
  mutation_class: red (lifecycle reducer, desk surface, Texture authority)
  authority_sources:
    - owner 2026-10-11 (protocol direction and plan approval)
    - docs/precommitment-records.md
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
  status: landed
  slice: P2a, P2b and P2c on staging and demo-proven (receipts P2a-deployed, P2c-deployed); P2d is its own station
  source_ref: b172895b (pushed; CI 38170958509 green)
  deploy_identity: staging b172895b
  candidate: {id: P2c, state: landed, ref: main, base: a2bcf724, digest: b196d666, scope: [internal/yaegikernel, internal/agentcore, internal/runtimeprompts, cmd/capsule-broker]}
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
    status: supported (research dispatch on one computer; management Cast not exercised by the demo)
    evidence_refs: [receipt P2a-deployed]
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
      Cast; P2 defers it as a named deviation until the owner answers.
      P2b refuses Texture writes in the ProductAPI verb rather than on the
      route side as design §9 item 5 recommends; the verb is the only
      desk path to those routes, so this is stricter, and a named
      deviation. Changing the fold later changes replayed turn identity
      across a deploy (named residual, as with the replay branches)
    kind: architecture
    status: provisional (P2-review1)
    evidence_ref: receipts P2-review1
  belief:
    believed_state: >-
      Texture dispatches with Cast and Ask like every desk, each dispatch a
      cast record in its turn's commit (P2a, staging); no desk writes a
      Texture through ProductAPI (P2b)
    main_uncertainty: whether P2c's ReportPacket path keeps engineering's progress bound to its work on a live assignment (unit-proven; no deployed engineering run yet)
    next_observation: CI and staging for P2c and P2d1; a deployed run with an engineering assignment
  blocker_or_risk: >-
    TestRLMReplayGoldens runs only on Node B against a capture dir, so
    replay row 9's switch to ReportPacket is unverified in CI (named
    residual); the broker's ActionMessage and ActionEmit remain,
    unreachable from cells, and go with the reducer's Message branch after
    one release
  next_action: >-
    none for P2 (landed; residuals above). No completion panel: the owner's
    rule (2026-10-11) is panels for uncertainty, not ceremony, and the
    result is not uncertain. P2d continues under its own file

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
  - id: P2-review2
    at: 2026-10-11T17:45Z
    kind: agentic-consensus file review, second round (session scratchpad p2/review2), frozen 612a2174
    result: >-
      Sent back 3 to 1 (Gemini accepts; Claude, DeepSeek, GLM send back);
      the OpenAI panelists were excluded for the usage limit. All four
      confirm the P2a fold keeps one transaction path, the round bound,
      opener merge and the research chain. Defects: P2d cannot dispose a
      directive (no postcondition branch, no turn disposition, no arm for an
      idle Texture, the delegated-cast step on a management Cast); the fold
      must live in commitTray to cover crash recovery, validate in two
      phases, refuse other addressees, multiple applies, email bodies and a
      second management opener; engineering's progress moves to ReportPacket,
      not Note; removed verbs refuse through stubs. Revised: P2d moved to its
      own station; the rest folded into the artifact and failure list.
  - id: P2-review3
    at: 2026-10-11T18:10Z
    kind: agentic-consensus file review, third round (session scratchpad p2/review3), frozen f363650d
    result: >-
      Gemini accepts; Claude and DeepSeek send back; GLM timed out. Both
      blocking findings were the same: Cast(management) promised to
      continue open management work, but textureTurnControls only
      continued research. Fixed in code with a test
      (TestTextureManagementCastContinuesTheRequestsOpenWork). DeepSeek
      also found the live replay test stages choir.Message (named in P2c)
      and asked for a refusal of spec-shaped Texture Casts (added, tested);
      Claude found the textureprompts test requiring choir.Message and the
      prompt sentences that become false (rewritten in P2a's code).
  - id: P2-review4
    at: 2026-10-11T18:25Z
    kind: fix check by the round-three senders (session scratchpad p2/review4), frozen d1d83c39
    result: >-
      DeepSeek accepts (ran the continuation, fold and prompt tests).
      Claude sends back on one new defect in the round-three fix: on a
      retry of a turn that opened management work, the continuation search
      found the turn's own work and rebuilt the opener as a continuation,
      changing the command digest. Fixed: the turn's own
      persistent-super-work ids are excluded like research's, with a replay
      case in TestTextureManagementCastContinuesTheRequestsOpenWork.
  - id: P2a-deployed
    at: 2026-10-11T20:06Z
    kind: deployed acceptance (staging a2bcf724), AI news demo with follow-up on fresh test computer demo-p2a-2026-10-11
    result: >-
      Texture's first turn cast to research in about twenty seconds (the
      raced-activation refusal did not recur on a fresh computer). Seven
      revisions, two of them the owner's. Eight agent dispatches between
      19:20 and 19:47, all to research: seven Casts and one Note, folded
      into turns. The P1 audit route joins all eight one to one to eight
      distinct cast records, each addressed to the packet's target, each
      naming its control, each with staged_act set; no failures, no
      agent-emitted record without staged_act. The round bound held: four
      rounds for the first request, then four after the follow-up reset
      it. Research reports were incorporated (revisions three, four, five
      and seven), including an unconfirmed claim (Mistral Large 4, single
      outlet) later confirmed on the vendor's newsroom. The last rounds'
      reports carried no sources and Texture rejected them; the trace shows
      research's search timed out at the guest while the gateway logged
      success at 30.4 seconds
      (docs/problems/guest-search-timeout-equals-gateway-provider-timeout-2026-10-11.md).
      Revision seven has an empty "Chips and compute" heading (content
      observation, not a protocol defect). Management Cast was not
      exercised. Evidence: session scratchpad demo19 and p2a/audit-raw.json.
  - id: P2c-deployed
    at: 2026-10-11T21:07Z
    kind: deployed acceptance (staging b172895b, CI 38170958509 green), regression AI news demo with follow-up on fresh test computer demo-p2c-2026-10-11 (booted 20:22:24Z, after the deploy)
    result: >-
      With Message and Emit retired and the delegated-cast gate (P2d1) in,
      the research chain still closes: eight revisions, five research
      dispatches (four rounds, then one after the follow-up), five distinct
      cast records with staged_act cast, no failures. Unit evidence for
      P2c's other items is in its commit (b196d666). Residuals: the replay
      golden row 9 runs only on Node B; broker ActionMessage and ActionEmit
      remain unreachable from cells; no deployed engineering run has
      exercised ReportPacket progress yet. Evidence: session scratchpad
      demo20 and demo20/audit-raw.json.
---

# P2 — one surface

After P1 every Texture dispatch is a record, but Texture still dispatches
through controls that no other desk has. P2 lands in three slices. P2a gives
Texture Cast and Ask and folds them into its turn, so the dispatch keeps the
one transaction path P1 put a record on. P2b fences management's ProductAPI
off every route that writes a Texture. P2c removes Message and Emit
from every desk. Texture addressing (P2d) is its own station, because the
Texture desk cannot yet wake on and dispose a directive packet.

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
  (their records carry emitter owner and are excepted by name, which is
  safe only once P2b refuses that route to desks);
- a tray staged with Message before P2c fails to replay after it (the
  reducer branches stay one release);
- engineering is still told to report progress with Message after P2c and
  loses a cell every turn (the overlay and its pinning test move to
  ReportPacket);
- engineering's progress loses its work binding (ReportPacket keeps the
  producer_report direction; a Note would not);
- Texture keeps its taught follow-up shape, an Ask or Note to research
  after the turn commits, and every such cell is refused (the prompt
  rewrite covers run_system.yaml's follow-up sentence and texture.yaml's
  Ask and Note guidance; the prompt test also scans for Ask and Note
  follow-ups outside a turn);
- a cell with two ApplyTexture intents or only an email one has no fold
  target (refused);
- a Texture Cast or Ask to a desk other than research or management falls
  through to commitLifecycleActIntent and its delegated-cast step (the
  fold refuses every other Texture addressee at the reduction boundary);
- a crash-recovery replay reduces Texture's Cast through the act path
  (the fold lives in commitTray, which recovery also reaches);
- the fold's own injected controls trip the refusal of model-authored
  controls (two-phase validation);
- two management Casts in one cell fail the turn without saying why
  (refused by name);
- folded intents vanish from the reduction receipt and break replay
  accounting (each folded intent is recorded with the turn's seq);
- a management Cast to texture:<doc> opens an engineering assignment
  through the delegated-cast step (P2d gates it to engineering targets);
- Texture wakes on a directive it cannot consume and re-wakes forever
  (P2d adds consume-at-commit and a crash-boot close);
- management and Texture feed each other without end (P2d bounds the
  rounds);
- management issues a management-open through ProductAPI during the P2a
  demo and the audit's owner exception hides it (P2b lands first);
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
