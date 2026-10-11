---
definition_version: 4

# P2d — texture addressing. Station of
# definitions/choir-precommitment-protocol-metamission-2026-10-11.md;
# design docs/precommitment-protocol-design-2026-10-11.md §3 (any desk may
# address texture:<doc>), §7 P2 and P5. Split from P2 at P2-review2.
readiness: drafted

review:
  reviewer: agentic-consensus file review (pending)
  frozen_ref: none
  verdict: none
  evidence_ref: none

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
      directiveTargetAgentID passes a qualified address through unchanged,
      so texture:<doc> already resolves to the Texture agent id.
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
      cannot start a turn.
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
    Any same-owner desk except engineering can Cast or Note to
    texture:<doc>. The act commits its record and a directive packet to
    that document's Texture agent, and opens no engineering assignment.
    The Texture desk wakes on it, even when its request work has closed,
    and its next committed turn disposes the delivery. A crash restart
    closes an undelivered directive's wake like any other work wake. The
    number of directives a Texture takes per owner request is bounded,
    and so is a management and Texture exchange.
  artifact: >-
    1. The delegated-cast step runs only when the Cast's addressee
    resolves to an engineering desk; a Cast to research, management or
    texture:<doc> through the act path commits its record and packet only.
    2. A third Texture occurrence kind, directive, encoded like the other
    two and resolved by its commitment record: the record exists, is a
    directive (cast or note), names texture:<doc> as addressee and the
    same owner and computer, its emitter is a live same-owner desk other
    than engineering, and its packet is pending. No producer-work or
    control-binding check, since the authority is the record.
    3. Arming: a pending directive to a Texture whose trajectory is live
    but has no open Texture work opens a Texture work item in the same
    command that binds the delivery (the initial-work wake path), so the
    turn has authority. A closed or cancelled trajectory terminalizes the
    directive as undeliverable (visible fate, no wake).
    4. Disposition: consume-at-commit as for producer reports; the turn's
    prompt shows the directive's record id, emitter and body so the turn
    can answer it. Texture answers through its existing verbs (a Cast to
    management continues management's open work, P2a).
    5. Bounds: directives to one Texture per owner request share the
    research-round counter's shape (four), and a directive emitted by
    management in response to a Texture dispatch on the same request
    counts against both, so the pair cannot feed each other without end.
    The activation breaker still stops a poison directive at three.
    6. Crash close: the directive's wake kind is in restartWorkKinds.
  acceptance:
    - reducer test: management Cast and Note to texture:<doc> commit record and packet, open no engineering assignment; an engineering Cast still opens one
    - controller tests: a pending directive arms an idle Texture (open work minted), resolves as a directive occurrence, is consumed at commit; a forged directive (wrong owner, engineering emitter, missing record) is refused; a closed trajectory terminalizes it
    - bound test: the fifth directive in one request is refused or held with a visible reason; a management and Texture exchange stops at the bound
    - crash test: a markerless boot closes a pending directive's wake as interrupted_by_restart
    - deployed: on a fresh test computer, a hand-staged management Cast to texture:<doc> (through a management turn, not ProductAPI) produces a Texture revision that cites it, and the audit joins the record, packet and consuming turn
  rollback: >-
    git revert of the station's commits; directive packets written before
    the revert stay pending and the old resolver refuses them, so the
    breaker terminalizes them within three attempts (named residual)
  landing:
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: one address scheme for every desk; Autopaper's management can feed any Texture through a record
  goodharting_would_be: >-
    a directive kind that a Texture can never actually act on (it wakes,
    consumes and ignores), or a bound so tight that W2 cannot feed a story
  realism_axis: reducer test -> controller tests -> fresh test computer with one hand-staged management Cast -> W2's poll-driven feed (P5)
  mutation_class: red (lifecycle reducer, Texture controller, Texture wake authority)
  authority_sources:
    - docs/precommitment-protocol-design-2026-10-11.md §3
    - owner direction 2026-10-11 (all inter-agent communication through precommitment records)
  must_preserve:
    - only the Texture desk writes a Texture (P2b)
    - producer_report and document_revision occurrences resolve exactly as today
    - engineering's delegated cast still opens its assignment
    - the activation breaker and the delivery budget
  excluded: [predictions on directives (P3), scoring (P4), the poll-to-management delivery and story size (P5), engineering addressing a Texture]
  protected_surfaces: [internal/agentcore/rlm_reduce.go, internal/textureowner/texture_controller.go, internal/agentcore/texture_lifecycle_api.go, internal/store/lifecycle.go, internal/actorruntime/handler.go]
  read_only_dependencies: [internal/agentcore/management_controller.go, internal/agentcore/tools_worker_update.go]

now:
  status: drafted
  slice: design from the code; review next
  source_ref: a0598d72
  deploy_identity: staging a2bcf724
  candidate: none
  conjecture:
    id: P2d-1
    claim: >-
      A directive occurrence authorized by its record, plus arming an idle
      Texture through its initial-work path, is enough for management to
      feed any same-owner Texture, with no new wake substrate.
    test: controller tests, then one hand-staged management Cast on staging
    edge: missing_oracle
    delta_o: a Texture revision whose turn consumed the directive, joined to the record by the audit
    scope_if_supported: management to Texture on one computer
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      Directive authority is the record, not a producer work binding: the
      emitter did not receive a control from this Texture, so there is no
      binding to check. Engineering is excluded as an emitter because its
      reports already reach Texture through the producer path and its
      assignment scope does not name a document.
    kind: architecture
    status: proposed
    evidence_ref: none
  belief:
    believed_state: the wake already arms on any pending packet; the gaps are resolution, arming an idle Texture, the delegated-cast gate, and the bound
    main_uncertainty: whether minting Texture work on arming collides with the initial-work wake's own run-suppression rule
    next_observation: the file review, reading reconcileAgentWakeLocked and submitTextureAgentRevisionRun
  blocker_or_risk: >-
    red; the Texture controller is the surface with the most past churn
    (docs/problems/texture-raced-activation-refused-first-turn-2026-10-11.md
    is open on the same activation path)
  next_action: agentic-consensus file review against a frozen worktree

receipts: []
---

# P2d — texture addressing

Autopaper's management has to feed a story's Texture with new evidence.
The protocol says any desk may address texture:<doc>, and the Texture desk
decides what to do with the delivery. Today a management Cast to a Texture
would commit its record and packet, then open an engineering assignment by
mistake. The Texture would wake on the packet and fail to resolve it, since
the resolver only knows producer reports and owner revisions. P2d closes
those two gaps, arms a Texture whose request work has closed, and bounds
the exchange.

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
- a packet pending before the deploy with the directive shape changes
  meaning after it (none exist: no desk could address texture:<doc>
  without opening an engineering assignment, which the audit checks).
