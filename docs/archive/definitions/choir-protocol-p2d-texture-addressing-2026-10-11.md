---
definition_version: 4

# P2d — texture addressing. Station of
# definitions/choir-precommitment-protocol-metamission-2026-10-11.md;
# design docs/precommitment-records.md §3 (any desk may
# address texture:<doc>), §7 P2 and P5. Split from P2 at P2-review2.
readiness: drafted

review:
  reviewer: agentic-consensus file review (round one sent back; revised)
  frozen_ref: 352661de (round one)
  verdict: round two GLM accept, Claude and Gemini send back on local defects; revised
  evidence_ref: receipts P2d-review1, P2d-review2

start:
  captured_at: 2026-10-11T19:35:00Z
  source:
    canonical_ref: main a0598d72 (P2a and P2b landed; P2c on PR 71)
    deploy_identity: staging choir.news a2bcf724
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean apart from this draft and P2's now card
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      A desk's Cast or Note commits through commitLifecycleActIntent
      (internal/agentcore/rlm_reduce.go): one CommitLifecycleAct mints the
      record and a directive packet addressed to the resolved agent.
      directiveTargetAgentID would pass texture:<doc> through, but cell
      validation refuses it first: validateSemanticActIntent's needTo
      requires agentprofile.Canonical(ToDesk), which fails on a qualified
      address (P2d-review1).
    - >-
      CommitLifecycleAct writes the packet on the target's trajectory
      (resolved through the target agent's channel document) but numbers
      its MessageSeq from the caller's trajectory counter
      (internal/store/lifecycle_commit_act.go, seq := trajectory.ReducerSeq).
      A Texture turn's consume boundary compares MessageSeq with the
      document trajectory's scheduled seq, so a cross-trajectory number is
      consumed by a run that never saw it, or inflates the boundary.
    - >-
      Consume-at-commit takes only producer reports
      (textureTurnPendingInbound and textureDispositionIneligibleReason,
      internal/textureowner/texture_turn_runtime.go); the store's turn
      commit already accepts directives (internal/store/texture_turn.go).
      A Texture cell does see a directive through choir.Updates()
      (cellUpdateVisible does not filter direction), but the revision
      prompt renders channel messages only, and record-native acts send
      none.
    - >-
      After that commit, every IntentCast runs openDelegatedCastAssignment
      with Kind EngineeringAssignmentImplementation, whatever the
      addressee (rlm_reduce.go, both the record-native and the legacy
      branch). A management Cast to texture:<doc> would open an
      engineering assignment.
    - >-
      The Texture wake (reconcileAgentWakeLocked,
      internal/textureowner/texture_controller.go) arms on
      ListPendingLifecycleUpdates for the Texture agent, which does not
      filter by packet direction (internal/store/lifecycle.go). A
      directive packet addressed to the Texture agent would arm a wake,
      bind to the run, and then fail occurrence resolution:
      ResolveTextureActorOccurrence knows only producer_report (validated
      through the producer's work, run and control binding on the
      document's trajectory) and document_revision.
    - >-
      submitTextureAgentRevisionRun refuses without an open Texture work
      item on the document's trajectory (errTextureLifecycleOpenWorkUnavailable),
      so a delivery to an idle Texture whose request work has closed
      cannot start a turn; the controller then returns nil without error
      and the packet stays pending with no visible fate. The wake returns
      before listing packets when the trajectory is not live.
    - >-
      Precedent for minting a Texture subject inside another command:
      textureSupervisionSubject (internal/store/engineering_assignments.go)
      mints texture:<doc> and an open work item, with not-exists
      conditions, inside the engineering report commit.
    - >-
      Consume-at-commit, stranded repair, the delivery-attempt budget and
      the activation breaker (textureDeliveryMaxAttempts, three) already
      apply to every pending packet the wake arms.
    - >-
      A markerless boot closes work occurrences whose kind is in
      restartWorkKinds (internal/actorruntime/handler.go) with the fate
      interrupted_by_restart; the lifecycle keeps pending reports for the
      owner's next action.
    - >-
      Management's producer reports reach a Texture only on a trajectory
      where that Texture opened management's work (the run's
      assignment_trajectory_id and the control binding). Autopaper W2 needs
      management to feed a Texture it did not hear from
      (definitions/choir-autopaper-w2-living-textures-2026-10-11.md,
      blocked on this station).

finish:
  deliver: >-
    Management can Cast or Note to texture:<doc> for any live document of
    the same owner, and research's Note to its own document's Texture
    (allowed by the messaging matrix today, and stranded today) is
    consumed the same way. The act commits, in one store command, its record, a
    directive packet numbered on the document's trajectory, and open
    Texture work when the Texture has none, and opens no engineering
    assignment. The Texture desk wakes on it, its turn sees it, and the
    turn's commit consumes it. A directive to a closed or cancelled
    trajectory is refused at commit. A crash never mints work for a
    directive; the directive's work was minted with it. Directives to one
    Texture per owner request are bounded, which also bounds a management
    and Texture exchange.
  artifact: >-
    Slice P2d1, the gate (lands first, alone). The post-commit
    delegated-cast step runs only when the Cast's addressee resolves to
    the engineering profile, in the record-native and the legacy branch.
    Today every act-path Cast opens an engineering assignment, whatever
    its addressee.
    Slice P2d2, the delivery (one slice, so no state exists where a
    directive arms but cannot be consumed).
    1. Validation: needTo accepts texture:<doc> when the caller is
    management and the document belongs to the same owner and computer;
    every other qualified address is still refused. Research keeps its
    bare Note to "texture", which resolves to its own document's Texture
    (agentprofile.CanMessage already allows research and management to
    message Texture). Engineering Casts or Notes to texture:<doc> are
    refused at validation. Other act kinds addressed to a Texture (Ask,
    Escalate, Reply) are refused at validation, so no packet the resolver
    cannot consume is minted.
    2. Commit (CommitLifecycleAct, target texture:<doc>): conditioned on
    the document trajectory's version and live status (one trajectory
    object when the caller's trajectory is the document's, two otherwise;
    loaded, conditioned and bumped once), it bumps that trajectory's
    ReducerSeq and numbers the packet from it, and always mints one open
    Texture work item for this directive (id derived from the record,
    not-exists condition, as textureSupervisionSubject does). The work
    belongs to the directive, not to whatever Texture work happens to be
    open: a directive committed during an in-flight turn that completes
    the request's work still has its own work to wake on, and the turn
    that consumes the directive settles that work (item 4). A concurrent
    Texture turn is not lost: applyTextureLifecycleTurn reads the
    trajectory version just before its commit, and on a collision returns
    "reload ... and retry this same run" to the same run
    (internal/textureowner/tools_texture.go). A closed or cancelled trajectory
    refuses the act with a visible reason. The bound (item 7) is checked
    here; because the commit is conditioned on that trajectory, two
    concurrent directives serialize.
    3. Occurrence: a directive occurrence kind (encoded like the other
    two, identity stable across replays) accepted where the adapter
    canonicalizes Texture dispatches, in ResolveTextureActorOccurrence and
    in TextureActorOccurrencePostcondition. Its authority is the record:
    the record exists, is a cast or note directive, is not retracted, its
    addressee is texture:<doc>, its emitter is the owner's persistent
    management or this document's research desk; the packet matches the
    record, has direction directive,
    targets this Texture on this document's trajectory, and is pending.
    The resolver sets ResolvedTargetWorkItemID to the directive's own
    work item; the postcondition reads the packet's disposition (the
    record is immutable and cannot show consumption).
    The occurrence carries the emitter as its ProducerAgentID, so the
    actor handler's expected-source check
    (internal/actorruntime/handler.go) admits the wake instead of
    consuming it as a foreign source envelope.
    4. Consumption: one predicate shared by textureTurnPendingInbound,
    textureDispositionIneligibleReason and cellUpdateVisible admits
    directive packets beside producer reports, keeping today's difference
    that a directive is visible to a cell only once bound to that run
    (DeliveredToRunID and DeliveredAt set) while a producer report may be
    unbound; the store's turn commit already accepts directives
    (internal/store/texture_turn.go). A consumed directive terminalizes as
    delivered with the turn, and the same commit completes the directive's
    own work item.
    5. Rendering: the revision request's worker-message slot exists but is
    passed nil today (buildAgentRevisionRequest's only production call,
    internal/textureowner/texture_agent_revision.go). This item fills it
    with the directives the run will consume (record id, emitter, objective, body, and the
    packet's evidence refs as given; W2's capture payload is defined in
    P5). An owner revision
    and a directive pending together arm one run whose prompt carries
    both, and its commit consumes both.
    6. Crash: the directive's wake is a coagent_result wake, already in
    restartWorkKinds, so a markerless boot closes it as
    interrupted_by_restart; nothing at boot mints work.
    7. Bound: at most four directives to one Texture per owner request
    (since the trajectory's latest owner input). A directive past the
    bound still commits its record, but its packet commits terminal
    (disposition delivered, reason bound_spent), with no work and no wake:
    the act path's wakeUpdatedCoagent call is skipped for a packet
    committed terminal (today it wakes on every non-replay update), so the management cell is not refused and the fate is visible.
    Texture's management Casts get the same drop-with-note bound in
    textureTurnControls (today only research rounds are bounded), so a
    management and Texture exchange stops within four rounds each way.
    8. Retract: a CancelAct of a directive to texture:<doc> commits its
    stand-down packet terminal (no wake, by the same gate); the original directive, if
    still pending, terminalizes as cancelled, and resolution re-reads the
    record so a retracted directive never starts a turn.
    9. Teaching: management's runtime overlay names texture:<docID> as a
    Cast and Note addressee for feeding a document's Texture, with a
    pinning test, and says the Texture desk decides what to do with it.
    10. Refusal fate: an occurrence the resolver refuses (forged,
    retracted, wrong trajectory) gets a terminal packet disposition with
    the reason, written by the controller, because today a refusal
    consumes only the wake, advancing neither the breaker nor the delivery
    budget, so the packet would re-arm forever.
  acceptance:
    - P2d1 reducer test: a management Cast to research or texture:<doc> opens no engineering assignment; an engineering Cast still opens one
    - validation test: management to texture:<doc> accepted; engineering and research to texture:<doc> refused; another owner's document refused
    - store test: the act on an idle Texture mints its work and a packet numbered on the document trajectory, in one command; a closed trajectory refuses; the fifth directive in one request commits its record with a terminal bound_spent packet, no work and no wake; two concurrent directives serialize
    - controller test: the directive arms the idle Texture, resolves as a directive occurrence, and a forged one (wrong owner, research emitter, retracted record, missing record) is refused
    - race test: a directive committed while a Texture turn is in flight, where that turn completes the request's work, still wakes a later turn on its own work item and is consumed
    - turn test: the revision prompt shows the directive's record id, emitter and body; the commit consumes it; a producer report that arrived after scheduling is not consumed with it
    - crash test: a markerless boot closes a pre-boot directive wake and mints no work
    - bound test: the fifth directive commits its record with a terminal bound_spent packet and no wake; Texture's fifth management Cast in a request is dropped with a runtime note
    - retract test: a CancelAct of a pending directive terminalizes it and its stand-down packet without a turn
    - handler test: a directive wake passes the expected-source check
    - prompt test: management's overlay teaches texture:<docID> and is pinned
    - pre-landing staging audit: count pending directive packets addressed to texture:* agents (research Notes today); the count and the drain decision are recorded before P2d2 deploys
    - refusal test: a refused directive occurrence terminalizes its packet with the reason and does not re-arm
    - activation test: an armed directive run passes ValidateOccurrenceActivationAuthority when the document channel also carries eligible channel messages (its scheduled seq is a trajectory seq; the check compares channel-message seqs)
    - deployed: on a fresh test computer, a management turn (owner-prompted, not ProductAPI) casts to texture:<doc>; the Texture's next revision answers it, and the audit joins record, packet and consuming turn
  rollback: >-
    git revert of the station's commits (P2d1 alone reverts cleanly).
    Reverting P2d2 needs a drain first: a store command that terminalizes
    every pending directive packet addressed to a Texture (disposition
    delivered, reason rollback), because the old resolver's refusal leaves
    them pending and re-arming, and closes the directive work items they
    own (a turn's caller work defaults to open, so nothing else would
    close them, and open Texture work arms an initial-work wake)
  landing:
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: one address scheme for every desk; Autopaper's management can feed any Texture through a record
  goodharting_would_be: >-
    a directive kind that a Texture can never actually act on (it wakes,
    consumes and ignores), or a bound so tight that W2 cannot feed a story
  realism_axis: P2d1 gate on staging -> store, controller and turn tests -> fresh test computer with one owner-prompted management Cast -> W2's poll-driven feed (P5)
  mutation_class: red (lifecycle reducer, Texture controller, Texture wake authority)
  authority_sources:
    - docs/precommitment-records.md §3
    - owner direction 2026-10-11 (all inter-agent communication through precommitment records)
  must_preserve:
    - only the Texture desk writes a Texture (P2b)
    - producer_report and document_revision occurrences resolve exactly as today
    - engineering's delegated cast still opens its assignment
    - the activation breaker and the delivery budget
  excluded: [predictions on directives (P3), scoring (P4), the poll-to-management delivery and story size (P5), engineering or research addressing a Texture, Ask to a Texture (Texture answers through its own verbs)]
  protected_surfaces: [internal/actorruntime/adapter.go, internal/agentcore/rlm_reduce.go, internal/store/lifecycle_commit_act.go, internal/textureowner/texture_controller.go, internal/textureowner/texture_turn_runtime.go, internal/textureowner/texture_agent_revision.go, internal/agentcore/texture_lifecycle_api.go, internal/agentcore/tools_desk.go, internal/actorruntime/handler.go, internal/runtimeprompts/overlays/rlm_management_runtime.yaml]
  read_only_dependencies: [internal/agentcore/management_controller.go, internal/agentcore/tools_worker_update.go]

now:
  status: drafted
  slice: P2d1 on staging (b172895b); P2d2 held for owner review after round three found a structural question (receipt P2d-review3)
  source_ref: 352661de
  deploy_identity: staging a2bcf724
  candidate: none
  conjecture:
    id: P2d-1
    claim: >-
      A directive committed with its Texture work and numbered on the
      document's trajectory, resolved by its record, is enough for
      management to feed any same-owner Texture through the existing wake,
      consume and breaker machinery.
    test: store, controller and turn tests, then one owner-prompted management Cast on staging
    edge: missing_oracle
    delta_o: a Texture revision whose turn consumed the directive, joined to the record by the audit
    scope_if_supported: management to Texture on one computer
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      The act's commit mints the Texture's work and numbers the packet on
      the document's trajectory, rather than a separate arming command
      before the run is submitted. One command touches two trajectories,
      as the engineering report commit already does for its supervision
      subject; in exchange the seq is right, the bound serializes, a
      closed trajectory refuses visibly, and no boot path mints work.
      Dissent (Claude, P2d-review1): prefer a smaller pre-submit arming
      command. Only management may address a Texture in this station;
      research and engineering already reach a Texture through producer
      reports.
    open_question: >-
      (P2d-review3) A Texture run binds one work item, and directives come
      from outside the owner's request. The most promising shape follows
      the existing supervision surface: one standing directive inbox work
      item per Texture (texture-directives:<doc>, minted once with a
      not-exists condition, excluded from the initial-work wake by prefix as
      supervision work is), never settled by turns; the pending directive
      packets themselves are the wake and are consumed at commit. The open
      parts are how activation authority accepts an occurrence whose work
      item differs from a run started for the owner's request (coalescing),
      and whether the store turn commit consumes a directive packet without
      a work settlement. Alternative: generalize the run to carry a set of
      work items, a larger change to the Texture root that the owner
      reserved for review.
      Design answer for review: docs/archive/texture-directive-inbox-design-2026-10-11.md
      recommends shape D (a directive carries no work item; the act joins
      the Texture's open request work or reopens it as an owner revise
      does; a turn cannot complete request work while a directive to it is
      pending), and confirms the store's turn commit accepts no directive
      today (ResolveLifecyclePacketWorkBindings has no directive case), so
      widening consumption without that store fix would fail every Texture
      turn while a directive is pending.
    kind: architecture
    status: proposed (P2d-review1); open after P2d-review3
    evidence_ref: receipt P2d-review1
  belief:
    believed_state: the wake already arms on any pending packet; the gaps were validation, seq, consumption, rendering, arming, and the delegated-cast gate
    main_uncertainty: whether conditioning the act on a second trajectory collides with the Texture turn's own head-pinned commits on that trajectory (a concurrent turn would refuse the directive, which then needs a retry)
    next_observation: round two of the file review
  blocker_or_risk: >-
    red; the Texture controller has the most past churn, and
    docs/problems/texture-raced-activation-refused-first-turn-2026-10-11.md
    is open on the same activation path. The bound starves a poll-driven
    W2 feed until P5 re-bases the request on the story (named residual on
    W2's start)
  next_action: >-
    owner direction 2026-10-11 evening: research, review and design before
    building. P2d2 waits for the owner's review of this file and of
    docs/archive/protocol-review-assign-and-engineering-routing-2026-10-11.md
    (engineering reaches a Texture only through management)

receipts:
  - id: P2d-review3
    at: 2026-10-11T21:15Z
    kind: fix check by Claude and Gemini (session scratchpad p2d/review3), frozen 4b114e61
    result: >-
      Both send back on one structural defect. A Texture run binds exactly
      one work item: ApplyTextureTurn settles only the caller's work item
      and accepts completed only with a revision; a delivered inbound packet
      touches no work; ValidateOccurrenceActivationAuthority and
      validateOccurrenceReconciliationCandidates require the occurrence's
      resolved work item to equal the run's; submitTextureAgentRevisionRun
      picks the first open work item. So one work item per directive either
      never closes (a no-change turn leaves it open and the initial-work
      wake loops) or collides with a concurrent owner request (the wake is
      refused and dropped). Gemini also reports the store turn commit's
      directive acceptance is narrower than round two stated. Not revised:
      this is a design question about how Texture's run model takes work
      from outside the owner's request, held for the owner with the shape
      named in now.decision.
  - id: P2d-review2
    at: 2026-10-11T21:00Z
    kind: agentic-consensus file review round two (session scratchpad p2d/review2), frozen 38dc5486
    result: >-
      GLM accepts; Claude and Gemini send back; DeepSeek and Muse timed out;
      Devin could not run tools non-interactively. Agreed defects: the
      fifth-directive acceptance items contradicted each other; a directive
      committed during an in-flight turn that completes the request's work
      strands (work was minted only when none was open), and nothing closes
      directive-minted work; terminal packets still wake, since the act path
      wakes on every update; the resolver needs the directive's own work
      item and the postcondition must read the packet, not the immutable
      record; the revision prompt renders no worker messages today; cells
      see a directive only once bound. Gemini's claim that a directive's
      version bump destroys an in-flight Texture turn was checked against
      the code and does not hold: applyTextureLifecycleTurn reads the
      version just before commit and returns a reload-and-retry error to the
      same run (Claude's reading). Revised: one work item per directive,
      settled by the consuming turn; the wake gate for terminal packets;
      resolver and postcondition sources; one-or-two trajectories; the
      prompt slot; a race acceptance test; the rollback closes directive
      work.
  - id: P2d-review1
    at: 2026-10-11T19:45Z
    kind: agentic-consensus file review (session scratchpad p2d/review), frozen 352661de
    result: >-
      Sent back by Claude and DeepSeek (other panelists failed to start or
      had not finished). Findings: cell validation refuses texture:<doc>;
      consume-at-commit admits producer reports only; the packet's seq comes
      from the caller's trajectory; arming cannot run in the bind step,
      which follows submit; nothing closes a directive on a closed
      trajectory; boot-time arming would resume work without a marker;
      directives never reach the revision prompt; the bound starves W2
      until P5; cut the delegated-cast gate as its own slice. The seq and
      consumption gaps were also found by this session while the panel
      ran. Revised: minting at commit on the document's trajectory,
      management-only addressing, shared consume predicate, rendering,
      P2d1 and P2d2. Gemini and GLM (finished later) agree and add: the
      actor handler's expected-source check would consume a directive
      wake as a foreign source; a CancelAct mints a second packet to the
      Texture; management is never taught the address; the bound needs a
      place and a non-refusing fate, and Texture's management Casts are
      unbounded today; W2 starves under a per-request bound (named
      residual, P5 re-bases the anchor on the story). Folded in as
      artifact items 7 to 9 and the protected-surface list. Devin and Muse
      (finished last): a refused occurrence leaves its packet pending and
      re-arming, so the rollback claim was false and refusals need a
      terminal fate; research's Note to its own Texture already mints
      stranded directives today, so pre-existing packets must be audited
      before P2d2; Ask, Escalate and Reply to a Texture mint unconsumable
      packets; the payload for W2 and owner-plus-directive coalescing were
      unspecified; adapter.go was missing; activation authority compares a
      trajectory seq with channel-message seqs. Devin preferred one slice
      for P2d; Muse and Claude preferred the gate first; the gate landed
      first because it is independent and fixes a live misroute. Folded in
      as items 1, 3, 5 and 10, the rollback drain, and three acceptance
      items.
---

# P2d — texture addressing

Autopaper's management has to feed a story's Texture with new evidence.
The protocol says any desk may address texture:<doc>, and the Texture desk
decides what to do with the delivery. Today cell validation refuses a
management Cast to texture:<doc>. Past that, the act path would open an
engineering assignment by mistake, number the packet on the wrong
trajectory, and the Texture would wake on a packet it can neither resolve
nor consume. P2d1 fixes the engineering misroute on its own. P2d2 makes
the delivery whole: the act commits the packet and the Texture's work on
the document's trajectory, and the Texture resolves, renders and consumes
it, within a bound.

Ways it can fail, written before the code:

- the delegated-cast gate stops engineering casts too (test both sides);
- a directive arms a wake but the resolver refuses it, and the breaker
  burns three activations per directive (the directive kind lands with the
  arming, in one slice);
- a forged directive (another owner's, an engineering emitter, a missing
  or retracted record) wakes a Texture (refused at resolution, tested);
- arming an idle Texture mints Texture work that the initial-work wake
  then suppresses because an earlier run is still active (read the
  suppression rule before coding);
- arming a Texture on a closed or cancelled trajectory reopens it (it
  terminalizes the directive instead);
- a directive is consumed by a turn that never saw it (the prompt shows
  every directive the turn consumes; consume-at-commit keys on the
  scheduled sequence as producer reports do);
- management and Texture feed each other without end (shared bound);
- the bound starves W2: a story with steady captures needs more than four
  directives in a day (the bound is per owner request; W2 re-bases the
  request on the story, P5);
- a crash restart resumes a directive's wake (its kind is in
  restartWorkKinds, tested);
- a directive retracted by CancelAct still wakes the Texture (resolution
  checks the record's state);
- the owner's own revision and a directive arrive together and the turn
  answers only one (both arm the same run; the prompt carries both);
- the supervision-only Texture of an engineering document takes a
  directive and arms a revision cell (refused there, as owner revisions
  are);
- a directive commits while a Texture turn on the same document is
  committing, and one of them refuses on the trajectory condition (the
  turn retries as it does for any concurrent change; the directive's cell
  is refused visibly and the desk can stage it again);
- a directive numbered on the caller's trajectory is consumed by a run
  that never saw it (the packet takes the document trajectory's seq);
- the gate in P2d1 changes what a management Cast to research did before
  (it opened an engineering assignment by mistake; the test pins that it
  no longer does, and that no caller relied on it);
- a packet pending before the deploy with the directive shape changes
  meaning after it (research's Note to its own Texture mints such packets
  today and they strand; the pre-landing audit counts them and P2d2's
  first consumption of them is recorded, not silent);
- a refused occurrence leaves its packet pending and re-arming forever
  (refusals write a terminal disposition);
- Ask, Escalate or Reply addressed to a Texture mint packets nothing
  consumes (refused at validation);
- the old resolver after a rollback re-arms on P2d2's leftover packets
  (the rollback drains them first).
