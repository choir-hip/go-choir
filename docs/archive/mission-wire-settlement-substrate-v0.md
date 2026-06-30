# Mission M5a — Wire Settlement Substrate (split from M5 evidence gate) — v0

Source: split from `docs/mission-wire-on-settlement-v0.md` on 2026-06-12
after Parallax passes 2–3 surfaced the `substrate_split` edge. Program:
`docs/choir-rearchitecture-durable-actors-2026-06-11.md` (cutover step 6,
§2.3), `docs/glossary.md` (`settlement`), and `specs/wire_pipeline.tla`
(SuppressedImpliesPublished, EditionHonest, SettledSound). Discipline:
`skills/parallax/SKILL.md`. Predecessor: M1
(`docs/mission-trajectory-model-v0.md`, settled 2026-06-12) and the split
receipts preserved in `docs/mission-wire-on-settlement-v0.md`.

## Source form

**Real artifact:** durable publication decision records plus an honest
settlement protocol for publication trajectories: trajectory subject-ref
patching, work items/decision objects on every publication mutation path,
and a settlement read surface that `sourcecycled` can later consume. This
mission does **not** raise `maxProc`, claim the production falsifier cycle,
or settle the route-switch evidence gate itself.

**Bridge conjecture:** if the repo first gains explicit coverage/publish
decision records and a settlement substrate that does not fake atomicity
across the runtime/VText store split, then the later M5 evidence gate
becomes executable and falsifiable instead of decorative. *Falsifier:* no
honest protocol on the current architecture can satisfy the wire settlement
rule without either hidden decision state or an unbounded inconsistency
window.

**Settlement:** every publication mutation path has a durable obligation or
decision record; `publish_ref` and edition linkage are queryable at the
trajectory layer; the chosen settlement protocol is explicit about its
transaction boundary and residual risk; and local tests cover the selected
protocol at the scope claimed. This mission settles the substrate only, not
the production evidence gate.

**Dependencies:** M1. **Successor consumer:** resume
`docs/mission-wire-on-settlement-v0.md` once this substrate is honest.

## Parallax State

status: open_handoff (2026-06-12; pass 13 landed explicit no-story cancelled terminal semantics while keeping `deferred` visibly open)

**mission conjecture:** if publication decision state is made durable and the
settlement protocol is made explicit at the actual store topology the repo
has, then M5 can later test the real bridge claim on production traffic
without overstating its evidence class.

**deeper goal (G):** durable actors, evidence-bearing promotion, and
self-development operational instead of documentary (portfolio G). This
mission serves M5 by turning a hidden architectural dependency into an
explicit artifact.

**witness/spec (A/S):** durable coverage/publish/edition decision objects
or equivalent obligation records; a trajectory subject-ref patch/update API;
and one honest settlement read/write protocol over the runtime/VText split
(either transaction-domain unification or an explicitly two-phase protocol
with named residual window). Spec transfer target: `wire_pipeline.tla`
settlement/edition invariants, asserted only at the scope the protocol can
actually support.

**invariants / qualities / domain ramp (I/Q/D):**
- I: no fake atomicity; no hidden decision state outside the declared
  protocol; no claim that publication trajectories settle in one
  transaction unless the decisive state truly shares one transaction domain.
- Q: the resulting substrate must make M5's later evidence gate legible:
  observers can answer what publication decision exists, what obligations are
  open, whether `publish_ref` exists, and whether edition linkage has landed.
- D: begins at repo/store/runtime scope with local tests and internal read
  surfaces only. Production-route claims remain with M5.

**authority / bounds:** repo changes on a branch. Because this mission can
change platform behavior, settlement still requires landing proof in its own
document if code lands. Working-tree runtime/store changes now exist for the
first seven constructs, but landing proof is still absent and is not claimed
by this pass.

### Position — inherited receipts from M5

1. Publication trajectories already exist; successive substrate passes have
   now added trajectory-level `publish_ref` / `edition_ref` recording, a
   narrower publish/edition work item, and a higher-level processor-opened
   story-resolution work item, but no explicit suppression verdict yet
   (`internal/runtime/trajectory.go`, `internal/store/trajectory.go`,
   `internal/runtime/tools_coagent.go`,
   `internal/runtime/wire_publication.go`).
2. The concrete publish path is `edit_vtext` →
   `maybeAutonomousPublishWireArticle` → platform publish metadata patch →
   edition revision/head advance (`internal/runtime/tools_vtext.go`,
   `internal/runtime/wire_publication.go`).
3. Trajectory/work-item state lives on the runtime DB, while revisions and
   edition head live on the VText DB (`internal/store/trajectory.go`,
   `internal/store/vtext.go`); there is no cross-store transaction primitive.
4. The ingestion handoff layer batches source items into processor requests
   but does not materialize durable coverage/suppression decisions
   (`internal/cycle/ingestion_handoff.go`).
5. `sourcecycled` can now observe a trajectory-obligation surface on the
   internal run-status payload it already polls: `trajectory_id`,
   trajectory status, settlement-ready bit, waiting-on reasons, and open
   work-item count (`cmd/sourcecycled/main.go`, `internal/runtime/api.go`).
   The remaining question is the honest consumer predicate on that surface,
   not absence of a readable surface.
6. First bounded substrate construct is now present in code: publication
   trajectories can merge subject refs via
   `Store.UpdateTrajectorySubjectRefs`; the publication settlement rule now
   requires both `publish_ref` and `edition_ref`; and autonomous wire
   publish patches those refs onto the trajectory after platform publish and
   edition update (`internal/store/trajectory.go`,
   `internal/runtime/trajectory.go`, `internal/runtime/wire_publication.go`).
   Focused tests passed at store/runtime scope:
   `nix develop -c go test ./internal/store ./internal/runtime -run 'TestTrajectory|TestWire'`.
7. Second bounded substrate construct is now present in code: autonomous
   wire publish opens a publication work item before external publish starts
   and completes it only after edition linkage lands. Failed publishes leave
   that obligation open, so the path is visible while in flight instead of
   becoming visible only after success
   (`internal/runtime/wire_publication.go`,
   `internal/runtime/wire_publication_test.go`).
8. The earlier coverage/suppression decision boundary is still prose-driven.
   Processor/reconciler prompts tell agents to spawn VText when a story
   should be opened or revised and to use `submit_coagent_update` for durable
   checkpoints (`internal/runtime/prompt_defaults/processor.md`,
   `internal/runtime/prompt_defaults/reconciler.md`). The runtime store does
   persist `worker_updates` with `trajectory_id`
   (`internal/runtime/tools_worker_update.go`, `internal/store/store.go`), but
   those records are optional, owner-addressed, and free-form. Their presence
   can mean "a checkpoint was sent"; their absence cannot honestly mean
   "already covered" or "suppressed". So existing worker updates are useful
   evidence packets, not the missing publication decision ledger.
9. The publication trajectory already spans the downstream article path.
   `StartChildRun` inherits `trajectory_id` from the parent
   (`internal/runtime/runtime.go`), processor `spawn_agent role=vtext` routes
   through `submitVTextAgentRevisionRun` under the processor parent
   (`internal/runtime/tools_coagent.go`), and tests already assert that a
   spawned run joins the parent's trajectory rather than minting a second one
   (`internal/runtime/trajectory_test.go`). Processor-specific tests show a
   processor-spawned VText revision run is created under that route
   (`internal/runtime/agent_tools_test.go`), and worker-update tests show the
   persisted `worker_update`, the worker run, and the VText run share one
   `trajectory_id` (`internal/runtime/vtext_test.go`). So the missing ledger
   does not need a second causality root; the publication trajectory is
   already the honest place to hang coverage and revision obligations.
10. A first typed downstream obligation now exists on that publication
    trajectory. When a processor opens a VText route, runtime now creates a
    durable `wire_story_resolution` work item on the inherited publication
    trajectory; successful autonomous publish completes that item together
    with the narrower publish/edition work item; failed platform publish
    leaves both obligations open
    (`internal/runtime/tools_coagent.go`,
    `internal/runtime/wire_publication.go`). Focused tests passed for the
    new handoff opening point and the successful/failed publish transitions:
    `nix develop -c go test ./internal/store ./internal/runtime -run 'TestSpawnMintsTrajectoryAndChildJoinsIt|TestProcessorSpawnMintsPublicationTrajectory|TestProcessorAndReconcilerProfilesDelegateToVTextOnly|TestWire'`.
11. The no-VText processor path is no longer invisible. Runtime now opens a
    durable `wire_processor_request_resolution` work item when a processor
    run starts, so a processor request that finishes without opening VText
    leaves a durable unresolved obligation instead of disappearing behind
    terminal run state (`internal/runtime/runtime.go`,
    `internal/runtime/wire_publication.go`). Focused tests passed:
    `nix develop -c go test ./internal/store ./internal/runtime -run 'TestProcessorAndReconcilerProfilesDelegateToVTextOnly|TestProcessorSpawnMintsPublicationTrajectory|TestTrajectoryObligationsAnswersWaitingOn|TestSpawnMintsTrajectoryAndChildJoinsIt|TestWire'`.
12. A first explicit typed processor verdict surface now exists, but only at
    request scope. Processor runs now expose
    `record_wire_processor_decision` for non-publication outcomes such as
    `already_covered`, and processor → VText handoff auto-records
    `opened_vtext` while completing the generic request-resolution item in
    favor of the more specific `wire_story_resolution` item. For explicit
    non-publication outcomes, the request item now carries a durable typed
    verdict but intentionally remains open because the publication trajectory
    still has no honest suppression settlement path
    (`internal/runtime/tools_wire_processor.go`,
    `internal/runtime/tools_coagent.go`,
    `internal/runtime/wire_publication.go`,
    `internal/store/trajectory.go`). Focused tests passed:
    `nix develop -c go test ./internal/store -run 'TestWorkItemDetailsMergePatch|TestWorkItemFingerprintDedupAndOpenObligationsQuery'`
    and
    `nix develop -c go test ./internal/runtime -run 'TestRecordWireProcessorDecisionToolRecordsExplicitNonPublicationVerdict|TestProcessorVTextRouteCompletesRequestDecisionWorkItem|TestProcessorSpawnMintsPublicationTrajectory|TestTrajectoryObligationsAnswersWaitingOn|TestWire'`.
13. The decision surface is now bound to individual source items on the
    publication trajectory. Processor run start now opens one
    `wire_source_item_resolution` work item per `source_item_id`; processor
    non-publication verdicts complete those per-item items; and processor →
    VText handoff now requires the exact covered `source_item_ids` when the
    request carries more than one item, then records `opened_vtext` on those
    per-item items while completing the generic request item only when every
    source item has a typed verdict and at least one story route exists
    (`internal/runtime/runtime.go`,
    `internal/runtime/tools_coagent.go`,
    `internal/runtime/tools_wire_processor.go`,
    `internal/runtime/wire_publication.go`). This is still substrate-only
    evidence: all-suppressed requests remain intentionally open at the
    request level because the trajectory still lacks an honest suppression
    settlement path. Focused tests passed:
    `nix develop -c go test ./internal/runtime -run 'TestRecordWireProcessorDecisionToolRecordsPerSourceItemNonPublicationVerdict|TestProcessorVTextRouteRequiresExplicitSourceItemsForMultiItemRequest|TestProcessorMixedPerItemDecisionsCompleteRequestOnceStoryRouteExists|TestProcessorSpawnMintsPublicationTrajectory|TestTrajectoryObligationsAnswersWaitingOn|TestWire'`
    and
    `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestInstallDefaultAgentToolsProfiles|TestProcessorAndReconcilerProfilesDelegateToVTextOnly'`.
14. A first honest terminal protocol now exists for the fully
    `already_covered` branch. `record_wire_processor_decision` now requires
    `covered_by_doc_id` for `already_covered`, validates that the referenced
    VText has a published route, stores that doc/route evidence on the
    per-source-item/request decision ledger, and when every source item
    resolves that way the generic request item completes and the publication
    trajectory moves to `cancelled`
    (`internal/runtime/tools_wire_processor.go`,
    `internal/runtime/wire_publication.go`,
    `internal/runtime/wire_processor_decision_test.go`). Focused tests
    passed:
    `nix develop -c go test ./internal/store -run 'TestWorkItemDetailsMergePatch|TestWorkItemFingerprintDedupAndOpenObligationsQuery'`
    and
    `nix develop -c go test ./internal/runtime -run 'TestRecordWireProcessorDecisionToolRecordsPerSourceItemNonPublicationVerdict|TestRecordWireProcessorDecisionToolRejectsAlreadyCoveredWithoutPublishedDoc|TestProcessorVTextRouteRequiresExplicitSourceItemsForMultiItemRequest|TestProcessorMixedPerItemDecisionsCompleteRequestOnceStoryRouteExists|TestProcessorSpawnMintsPublicationTrajectory|TestTrajectoryObligationsAnswersWaitingOn|TestWire'`
    plus
    `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestInstallDefaultAgentToolsProfiles|TestProcessorAndReconcilerProfilesDelegateToVTextOnly'`.
15. The internal runtime status payload can now surface publication-trajectory
    obligations directly to sourcecycled's existing poll loop.
    `GET /internal/runtime/runs/{id}` now includes trajectory status,
    settlement-ready, waiting-on reasons, and open-work-item count, and
    sourcecycled's decode shape understands that payload even though it does
    not consume the new fields yet
    (`internal/runtime/api.go`,
    `internal/runtime/api_test.go`,
    `cmd/sourcecycled/main.go`,
    `cmd/sourcecycled/main_test.go`). Focused tests passed:
    `nix develop -c go test ./internal/runtime -run 'TestInternalRuntimeRunRoutesRequireInternalCallerAndConstrainProfiles|TestHandleInternalRunStatusIncludesTrajectoryObligations|TestRecordWireProcessorDecisionToolRecordsPerSourceItemNonPublicationVerdict|TestRecordWireProcessorDecisionToolRejectsAlreadyCoveredWithoutPublishedDoc|TestProcessorVTextRouteRequiresExplicitSourceItemsForMultiItemRequest|TestProcessorMixedPerItemDecisionsCompleteRequestOnceStoryRouteExists|TestProcessorSpawnMintsPublicationTrajectory|TestTrajectoryObligationsAnswersWaitingOn|TestWire'`
    and
    `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcher'`.
16. That internal observer surface now also carries the concrete processor
    request-resolution certificate. `GET /internal/runtime/runs/{id}` now
    includes request-item status, `resolution_state`, source-item counts,
    `last_decision`, and the story/coverage doc ids when present, and the
    durable request item is initialized with `awaiting_source_item_decisions`
    at processor run start so the certificate is visible immediately rather
    than only after a later reconcile
    (`internal/runtime/api.go`,
    `internal/runtime/api_test.go`,
    `internal/runtime/wire_publication.go`,
    `cmd/sourcecycled/main.go`). Focused tests passed:
    `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestHandleInternalRunStatusIncludesTrajectoryObligations|TestHandleInternalRunStatusIncludesProcessorResolutionTerminalBranch|TestInternalRuntimeRunRoutesRequireInternalCallerAndConstrainProfiles'`,
    `nix develop -c go test ./internal/runtime -run 'TestRecordWireProcessorDecisionToolRecordsPerSourceItemNonPublicationVerdict|TestRecordWireProcessorDecisionToolRejectsAlreadyCoveredWithoutPublishedDoc|TestProcessorVTextRouteRequiresExplicitSourceItemsForMultiItemRequest|TestProcessorMixedPerItemDecisionsCompleteRequestOnceStoryRouteExists|TestProcessorSpawnMintsPublicationTrajectory|TestTrajectoryObligationsAnswersWaitingOn|TestWire'`,
    and
    `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcher'`.
17. The successful autonomous publish path now also performs the runtime-side
    lifecycle transition instead of stopping at readiness substrate. After
    platform publish, edition write, trajectory ref patching, and work-item
    completion, runtime now reevaluates obligations and stamps the
    publication trajectory `status=settled` / `settled_at`
    (`internal/runtime/wire_publication.go`,
    `internal/runtime/wire_publication_test.go`). Focused tests passed:
    `nix develop -c go test ./internal/runtime -run 'TestWireAutonomousPublishTranscludesEditionAndDebounces|TestWirePlatformPublishFailsClosedWithoutEditionWhenPlatformdFails'`.
18. The sibling explicit no-story branch now also has bounded terminal
    semantics instead of hanging forever behind a completed processor run.
    Explicit per-item `not_newsworthy` / `insufficient_evidence` decisions
    now complete the request-resolution item and cancel the publication
    trajectory once every source item resolves without a story route, while
    per-item `deferred` decisions keep the request-resolution item open under
    a distinct `all_source_items_deferred_without_story_route` state
    (`internal/runtime/wire_publication.go`,
    `internal/runtime/wire_processor_decision_test.go`,
    `internal/runtime/api_test.go`). Focused tests passed:
    `nix develop -c go test ./internal/runtime -run 'TestRecordWireProcessorDecisionToolRecordsPerSourceItemNonPublicationVerdict|TestRecordWireProcessorDecisionToolCancelsExplicitNoStoryTerminalBranch|TestRecordWireProcessorDecisionToolKeepsDeferredBranchOpen|TestHandleInternalRunStatusIncludesProcessorResolutionTerminalBranch|TestHandleInternalRunStatusIncludesExplicitNoStoryTerminalBranch|TestWireAutonomousPublishTranscludesEditionAndDebounces|TestWirePlatformPublishFailsClosedWithoutEditionWhenPlatformdFails'`.

Blind spots from this position:
- **substrate_choice:** the repo now has one explicit two-phase publication
  protocol: story/edition state lands first on the VText side, then runtime
  patches trajectory refs, closes work items, and only then stamps
  `status=settled`. That is honest for the successful publish path, but it is
  still not a unified transaction domain. The remaining choice is whether to
  widen this named residual-window protocol to the rest of publication
  settlement, or unify the domain before broader claims.
- **frame_lock:** decision objects can still be too coarse; if they do not
  open at work start, settlement can remain vacuous.
- **missing_oracle:** the substrate may be internally sound while still not
  exposing enough state for M5's later honest-and-full verifier.
- **consumer_rule:** the observer surface is now present on the internal
  status payload sourcecycled already polls, and now includes the request-
  resolution certificate too. The route now also has a real publication
  `trajectory.status=settled` to consume, not only `settlement_ready`. The
  remaining question is still the honest broader predicate on that surface:
  settlement only, terminal status including `cancelled`, request resolution,
  or a stricter composite.
- **decision_gap:** the ref substrate is necessary but not sufficient.
  Publication trajectories now carry per-source-item decision objects, and
  the published-corpus branch and the explicit terminal no-story branch now
  both have `cancelled` terminal paths. No honest claim can yet say the full
  coverage/suppression protocol is done, because `deferred` still remains an
  intentionally open branch rather than a terminal one.
- **false_ledger:** trajectory-scoped `worker_updates` are durable and
  queryable, but they are still optional prose packets. Reinterpreting them
  as publication verdicts would create a fake island where missing updates
  masquerade as settled suppression decisions.
- **decision_shape:** the root object is now clear and the first concrete
  answer is live: typed source-item work items on the publication
  trajectory. The remaining choice is whether later suppression settlement
  keeps extending that work-item route or introduces a sibling decision table
  for cleaner terminal semantics.
- **suppression_gap:** processor requests can now record explicit typed
  non-publication verdicts per source item, and multi-item VText handoffs now
  bind exact source-item ids. The fully `already_covered` branch now has an
  honest `cancelled` path backed by published-doc evidence, but the sibling
  non-publication verdicts still have no honest terminal protocol, so the
  route cannot yet claim the full suppression ledger is complete.
- **batch_shape:** one VText route can now resolve an explicit subset of the
  batch, but the later attach/suppress protocol is still manual enumeration
  over source-item ids rather than a richer typed batch object.
- **scope_gap:** the publish/edition path now has one honest in-flight
  obligation, processor-opened VText adds a higher-level story-resolution
  obligation, and processor outcomes now have per-source-item typed verdicts.
  This is still not the whole ledger: one suppression terminal branch is now
  honest, but the broader non-publication terminal protocol and later
  consumer semantics are still missing.

### Initial conjectures

- **C1:** a two-phase protocol may be honest enough if the residual
  inconsistency window is explicit, queryable, and blocks settlement until
  edition linkage is durably visible. *Falsifier:* any protocol candidate
  requires silently trusting cross-store timing.
- **C2:** coverage/suppression decisions belong in durable records keyed to
  the publication trajectory and source items, not only in processor prose or
  VText side effects. `worker_updates` alone are not enough because they are
  optional and prose-shaped. *Falsifier:* the route can stay honest without
  adding a new decision object or typed work-item surface.
- **C3:** the smallest future read surface for M5 may be a trajectory-detail
  read keyed by `trajectory_id`, not a broader run-status rewrite. Pass 10
  narrows this directly: the broader run-status rewrite now exists, so the
  remaining conjecture is about the right consumer predicate, not the
  availability of a read surface. Pass 11 narrows it again: the
  request-resolution certificate is now on that same surface, so the
  remaining conjecture is about the right composition rule over already-
  visible fields.
- **C6:** the publication trajectory itself is the correct ledger root for
  downstream article work; no second trajectory or parallel causality object
  is required for processor → VText → coagent obligations. *Falsifier:* a
  real publication mutation path escapes the inherited `trajectory_id`.
- **C7:** a higher-level story-resolution work item opened at processor →
  VText handoff is a useful partial antidote to vacuous settlement before the
  full suppression ledger exists. *Falsifier:* processor-opened article work
  can still disappear from obligations until external publish starts.
- **C8:** a processor-request decision work item opened at processor run start
  is a useful antidote to the no-VText invisibility branch even before
  per-item suppression decisions exist. *Falsifier:* a processor request can
  still terminate with zero durable obligations and no explicit verdict.
- **C9:** a request-scoped typed processor verdict is a useful next construct
  even before per-item suppression settlement exists, as long as explicit
  non-publication outcomes remain visibly open instead of being treated as
  settled. *Falsifier:* the new verdict surface encourages the route to claim
  suppression settlement it still cannot prove.
- **C10:** per-source-item decision work items plus explicit source-item
  binding on processor → VText handoff are the smallest honest extension of
  the current work-item ledger. *Falsifier:* a real processor route still
  needs to claim a story handoff or non-publication outcome for source items
  it cannot name explicitly.
- **C4:** recording `publish_ref` and `edition_ref` at trajectory scope is a
  useful substrate step even before the decision ledger exists. *Falsifier:*
  the new refs do not survive real publication tests or fail to tighten the
  obligations query.
- **C5:** opening a durable work item at publish start is a useful partial
  antidote to vacuous settlement even before the full decision ledger exists.
  *Falsifier:* failed or in-flight publishes still appear obligation-free.

### Open questions

- **Q1:** where should durable coverage/suppression decisions live: runtime
  store trajectory records, a sibling decision table, or a publication-kind
  work-item schema? Existing `worker_updates` no longer count as a candidate
  answer because they cannot encode verdict-by-absence.
- **Q2:** is transaction-domain unification actually required, or can the
  glossary's "evaluated inside the transactions that could change the verdict"
  be satisfied by a two-phase protocol whose decisive verdict is delayed
  until both stores have durably landed their half?
- **Q3:** what exact subject refs should publication settlement require at
  trajectory scope beyond `publish_ref` (for example, edition revision/head
  linkage)?
- **Q4:** what durable shape should coverage/suppression decisions take so
  they can open at work start and block vacuous settlement: new publication
  decision table, publication-kind work items with typed details, or another
  explicit ledger? Optional narrative worker updates are excluded.
- **Q5:** should the eventual decision ledger extend the work-item route just
  used for publish/edition, or does the coverage/suppression side need a
  distinct decision table to stay legible?
- **Q6:** how should suppression-as-already-covered be represented when no
  VText run is spawned at all: a typed completed work item on the
  publication trajectory, or a separate typed decision record? Pass 9 now
  answers the first half: `already_covered` uses typed per-item work items
  plus published `covered_by_doc_id` evidence and trajectory cancellation.
  Pass 13 narrows the second half: explicit terminal no-story verdicts now
  receive a parallel cancelled path, while `deferred` intentionally stays
  open. The remaining question is whether `deferred` should always remain an
  open obligation or later gain a different terminal protocol.
- **Q7:** should the processor-opened `wire_story_resolution` work item be
  completed only by successful publish (current bounded construct), or should
  a later explicit typed suppression decision also be allowed to complete it?
- **Q8:** once per-item suppression/attach decisions exist, should the
  request-scoped `wire_processor_request_resolution` item complete when every
  source item has a typed verdict, or should it disappear in favor of only
  per-item records? Pass 9 now gives a partial answer: the request item
  completes when every source item either routes to story or resolves as
  published-corpus coverage. Pass 13 narrows this again: explicit terminal
  no-story branches now also complete/cancel, while `deferred` intentionally
  keeps the request item open. The remaining question is whether that
  `deferred` asymmetry is permanent.
- **Q9:** now that `GET /internal/runtime/runs/{id}` can carry trajectory
  obligations, should sourcecycled's later reconcile flip consume
  `trajectory.status`, `settlement_ready`, request-resolution details, or a
  stricter composite? The surface now exists, and pass 11 adds the
  request-resolution certificate to it. Pass 12 narrows this again by making
  successful publication trajectories actually reach `status=settled`; the
  honest predicate is still open for the broader route, but the publication
  success branch no longer has to infer settlement from readiness alone.
  Pass 13 narrows it again by splitting explicit terminal no-story outcomes
  from the still-open `deferred` branch.

**ledger / move log:**

- 2026-06-12 CREATED (split from M5 pass 3; docs-only).
  Claim: M5's next move is not a route-switch construct but a substrate
  construct.
  Position: repeated probes/shifts on M5 found a store-topology split and
  missing decision objects, not just unwired helper calls.
  Move: shift.
  Bound: documentation only; create a successor mission with the live
  conjectures and obligations that no longer fit M5's evidence-gate scope.
  Update: this mission now owns the substrate choice and decision-ledger
  questions; M5 remains the later production evidence gate.
  Exit: proposed.

- 2026-06-12 CONSTRUCT (bounded ref substrate; code + focused tests).
  Claim: the smallest honest construct is not a settle flip but publication
  ref substrate at trajectory scope.
  Position: the repo had publication trajectories and a pure obligations
  evaluator, but no way to patch trajectory subject refs after publish /
  edition events, and the publication rule only required `publish_ref`.
  Blind spot reduced: M5's later settle/read path now has durable
  `publish_ref` and `edition_ref` anchors to observe, and the obligations
  query now names both missing refs before any settle flip can occur.
  Move: construct.
  Bound: no cross-store atomicity claim, no decision-ledger object, no
  sourcecycled reconcile flip. Only add the ref-patching surface and tighten
  the publication rule to the scope the current code can honestly support.
  Update: landed `Store.UpdateTrajectorySubjectRefs`, publication settlement
  rule now requires `publish_ref` + `edition_ref`, wire autonomous publish
  writes those refs, and focused tests passed:
  `nix develop -c go test ./internal/store ./internal/runtime -run 'TestTrajectory|TestWire'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (bounded in-flight publish obligation; code + focused tests).
  Claim: the next smallest honest construct is to make the publish/edition
  path visible while it is in flight, not only after it succeeds.
  Position: pass 1 added durable publication/edition refs, but failed
  publishes still left no open obligation on the trajectory.
  Blind spot reduced: a platform-publish or edition-link failure now leaves
  an open publication work item on the trajectory, and a successful publish
  completes that item only after edition linkage lands.
  Move: construct.
  Bound: no coverage/suppression ledger yet, no cross-store atomicity claim,
  no settle flip, no sourcecycled reconcile change. Only add a durable
  publish-path work item keyed to the trajectory + revision.
  Update: autonomous wire publish now opens and completes a durable
  publication work item around the publish/edition path; focused tests passed:
  `nix develop -c go test ./internal/store ./internal/runtime -run 'TestTrajectory|TestWire'`.
  Exit: open_handoff.

- 2026-06-12 SHIFT (worker-updates are not the decision ledger; docs-only).
  Claim: the existing trajectory-scoped `worker_updates` surface might already
  be enough to serve as the first durable coverage/suppression ledger.
  Position: processor/reconciler prompts already require
  `submit_coagent_update` checkpoints, and the runtime atomically persists
  `worker_updates` with `trajectory_id` and channel delivery in the runtime
  store.
  Blind spot reduced: those updates are durable evidence packets, but they are
  optional, owner-addressed, and free-form. They can prove that a checkpoint
  happened; they cannot make the absence of a checkpoint mean
  "already covered", "suppressed", or "nothing left to do". Treating them as
  the ledger would fake settlement by absence.
  Move: shift.
  Bound: documentation only; inspect prompt/runtime/store surfaces without
  code mutation.
  Update: the next honest bounded construct must materialize typed
  coverage/suppression obligations or decisions on the publication trajectory
  itself, either by extending work items with typed publication details or by
  adding a sibling decision table.
  Exit: open_handoff.

- 2026-06-12 PROBE (publication trajectory spans downstream article work; docs-only).
  Claim: the missing publication decision ledger might require a second
  causality root because processor, VText, and downstream worker work could be
  on different trajectories.
  Position: `ensureTrajectoryID` inherits the parent's `trajectory_id` on
  child runs; processor `spawn_agent role=vtext` submits the VText revision
  run under the processor parent; tests already assert that spawned runs join
  the parent's trajectory, that processor-spawned VText runs use the normal
  VText revision path, and that later worker updates persist on the same
  trajectory as the worker and VText runs.
  Blind spot reduced: there is no need to invent a second ledger root for the
  downstream article path. The existing publication trajectory can honestly
  carry coverage opening, revision, publish, and later suppression/settlement
  obligations.
  Move: probe.
  Bound: documentation only; inspect runtime inheritance, VText routing, and
  tests without code mutation.
  Update: the next construct is narrowed from "where does the ledger live?" to
  "what typed decision/work-item shape should the publication trajectory
  carry?" The remaining edge is suppression-by-non-spawn, which still needs an
  explicit typed verdict.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (processor-born story-resolution obligation; code + focused tests).
  Claim: the next smallest honest construct is to make processor-opened
  article work visible on the publication trajectory before external publish
  starts.
  Position: pass 4 proved the publication trajectory already spans processor,
  VText revision runs, and downstream worker updates, but the obligations
  query still stayed empty until `maybeAutonomousPublishWireArticle` opened the
  narrower publish work item.
  Blind spot reduced: processor-born article work is now visible from the
  first VText handoff. Runtime opens a durable `wire_story_resolution` work
  item on the inherited publication trajectory when processor routes into
  VText, and successful autonomous publish completes that higher-level item
  together with the publish/edition work item. Failed platform publish now
  leaves both obligations open.
  Move: construct.
  Bound: no suppression verdict yet, no reconcile flip, no cross-store
  atomicity claim, no broader decision table. This construct covers only the
  processor-opened article path that actually routes into VText and reaches
  the autonomous publish policy.
  Update: landed in `internal/runtime/tools_coagent.go`,
  `internal/runtime/wire_publication.go`, and focused tests passed:
  `nix develop -c go test ./internal/store ./internal/runtime -run 'TestSpawnMintsTrajectoryAndChildJoinsIt|TestProcessorSpawnMintsPublicationTrajectory|TestProcessorAndReconcilerProfilesDelegateToVTextOnly|TestWire'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (processor-request decision obligation; code + focused tests).
  Claim: the next smallest honest construct is to make the no-VText processor
  branch visible on the publication trajectory without fabricating a
  suppression verdict.
  Position: pass 5 made processor-opened VText work visible, but a processor
  request that never opened VText could still end with no durable obligation
  record on its publication trajectory.
  Blind spot reduced: every processor request now opens a durable
  `wire_processor_request_resolution` work item when the processor run starts.
  If the processor later routes into VText, that generic request item remains
  open while the more specific story-resolution item tracks the opened story.
  If the processor never opens VText, the generic item still remains open, so
  the missing explicit verdict is visible instead of being silently treated as
  done.
  Move: construct.
  Bound: no explicit suppression decision yet, no per-source-item decision
  object, no reconcile flip, no cross-store atomicity claim. This construct
  covers only visibility of the processor request branch.
  Update: landed in `internal/runtime/runtime.go`,
  `internal/runtime/wire_publication.go`, and focused tests passed:
  `nix develop -c go test ./internal/store ./internal/runtime -run 'TestProcessorAndReconcilerProfilesDelegateToVTextOnly|TestProcessorSpawnMintsPublicationTrajectory|TestTrajectoryObligationsAnswersWaitingOn|TestSpawnMintsTrajectoryAndChildJoinsIt|TestWire'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (request-scoped typed processor verdicts; code + focused tests).
  Claim: the next smallest honest construct is not per-item suppression
  settlement but an explicit request-scoped typed processor verdict surface.
  Position: pass 6 made the no-VText branch durably visible, but the
  processor request item still carried no typed outcome. A processor opening
  VText and a processor deciding "already covered" both remained under-typed.
  Blind spot reduced: processor → VText handoff now records
  `opened_vtext` and completes the generic request item in favor of the
  downstream story-resolution item; a processor that decides
  `already_covered`, `not_newsworthy`, `insufficient_evidence`, or
  `deferred` can now record that verdict durably with
  `record_wire_processor_decision`.
  Move: construct.
  Bound: no per-source-item decision ledger, no suppression settlement claim,
  no sourcecycled reconcile flip, no cross-store atomicity claim. Explicit
  non-publication verdicts remain open obligations because the settlement
  substrate for suppression is still missing.
  Update: landed `Store.UpdateWorkItemDetails`, processor-only
  `record_wire_processor_decision`, and auto-record/complete behavior for
  processor → VText handoff in `internal/runtime/tools_coagent.go`,
  `internal/runtime/tools_wire_processor.go`,
  `internal/runtime/wire_publication.go`; focused tests passed:
  `nix develop -c go test ./internal/store -run 'TestWorkItemDetailsMergePatch|TestWorkItemFingerprintDedupAndOpenObligationsQuery'`
  and
  `nix develop -c go test ./internal/runtime -run 'TestRecordWireProcessorDecisionToolRecordsExplicitNonPublicationVerdict|TestProcessorVTextRouteCompletesRequestDecisionWorkItem|TestProcessorSpawnMintsPublicationTrajectory|TestTrajectoryObligationsAnswersWaitingOn|TestWire'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (per-source-item processor decisions; code + focused tests).
  Claim: the next smallest honest construct is to move the processor decision
  ledger from request scope down to explicit source-item scope on the same
  publication trajectory.
  Position: pass 7 added a typed processor verdict surface, but multi-item
  requests could still only record that verdict at request scope. A VText
  route for one story still could not honestly say which source items it
  resolved.
  Blind spot reduced: processor run start now opens one
  `wire_source_item_resolution` work item per source item. Non-publication
  verdicts complete those per-item items, and multi-item processor → VText
  handoffs must now name exact `source_item_ids` so the runtime can record
  `opened_vtext` only on the items actually covered by that story.
  Move: construct.
  Bound: no trajectory-level suppression settlement yet, no reconcile flip,
  no cross-store atomicity claim. All-suppressed requests intentionally keep
  the generic request item open because the trajectory still has no honest
  suppression-settlement path.
  Update: landed per-source-item work-item minting in
  `internal/runtime/runtime.go`, explicit source-item binding in
  `internal/runtime/tools_coagent.go`, and per-item decision recording in
  `internal/runtime/tools_wire_processor.go` /
  `internal/runtime/wire_publication.go`; focused tests passed:
  `nix develop -c go test ./internal/runtime -run 'TestRecordWireProcessorDecisionToolRecordsPerSourceItemNonPublicationVerdict|TestProcessorVTextRouteRequiresExplicitSourceItemsForMultiItemRequest|TestProcessorMixedPerItemDecisionsCompleteRequestOnceStoryRouteExists|TestProcessorSpawnMintsPublicationTrajectory|TestTrajectoryObligationsAnswersWaitingOn|TestWire'`
  and
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestInstallDefaultAgentToolsProfiles|TestProcessorAndReconcilerProfilesDelegateToVTextOnly'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (published-coverage evidence + cancelled all-covered branch; code + focused tests).
  Claim: the next smallest honest construct is not full suppression
  settlement but an evidence-backed terminal path for the fully
  `already_covered` branch.
  Position: pass 8 gave the route per-source-item decision identity, but an
  all-suppressed request still stayed open forever even when every source
  item could be tied to already-published coverage.
  Blind spot reduced: `already_covered` now requires a published
  `covered_by_doc_id`, stores the corresponding publication evidence on the
  decision ledger, completes the generic request item when every source item
  resolves that way, and marks the publication trajectory `cancelled`.
  Move: construct.
  Bound: no publication settlement claim, no sourcecycled reconcile flip, no
  cross-store atomicity claim, no terminal semantics yet for the sibling
  non-publication verdicts. This construct covers only the
  published-corpus-coverage branch.
  Update: landed the `covered_by_doc_id` tool/runtime contract and the
  all-covered cancellation branch in
  `internal/runtime/tools_wire_processor.go` /
  `internal/runtime/wire_publication.go`; focused tests passed:
  `nix develop -c go test ./internal/store -run 'TestWorkItemDetailsMergePatch|TestWorkItemFingerprintDedupAndOpenObligationsQuery'`,
  `nix develop -c go test ./internal/runtime -run 'TestRecordWireProcessorDecisionToolRecordsPerSourceItemNonPublicationVerdict|TestRecordWireProcessorDecisionToolRejectsAlreadyCoveredWithoutPublishedDoc|TestProcessorVTextRouteRequiresExplicitSourceItemsForMultiItemRequest|TestProcessorMixedPerItemDecisionsCompleteRequestOnceStoryRouteExists|TestProcessorSpawnMintsPublicationTrajectory|TestTrajectoryObligationsAnswersWaitingOn|TestWire'`,
  and
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestInstallDefaultAgentToolsProfiles|TestProcessorAndReconcilerProfilesDelegateToVTextOnly'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (internal run-status trajectory observer surface; code + focused tests).
  Claim: the next smallest honest construct is not the reconcile flip itself
  but the narrower observer surface on the payload sourcecycled already polls.
  Position: pass 9 gave the route one honest non-publication terminal branch,
  but sourcecycled still could not see trajectory obligations on
  `/internal/runtime/runs/{id}` without issuing a second trajectory read.
  Blind spot reduced: internal run status now carries trajectory status,
  settlement-ready, waiting-on, and open-work-item count, and sourcecycled's
  decode shape understands that payload.
  Move: construct.
  Bound: no reconcile flip, no publication-settlement claim, no sourcecycled
  consumer-rule claim, no cross-store atomicity claim. This construct covers
  only the observer surface.
  Update: landed the internal-status trajectory payload in
  `internal/runtime/api.go` and the corresponding sourcecycled/runtime tests
  in `internal/runtime/api_test.go` and `cmd/sourcecycled/main_test.go`;
  focused tests passed:
  `nix develop -c go test ./internal/runtime -run 'TestInternalRuntimeRunRoutesRequireInternalCallerAndConstrainProfiles|TestHandleInternalRunStatusIncludesTrajectoryObligations|TestRecordWireProcessorDecisionToolRecordsPerSourceItemNonPublicationVerdict|TestRecordWireProcessorDecisionToolRejectsAlreadyCoveredWithoutPublishedDoc|TestProcessorVTextRouteRequiresExplicitSourceItemsForMultiItemRequest|TestProcessorMixedPerItemDecisionsCompleteRequestOnceStoryRouteExists|TestProcessorSpawnMintsPublicationTrajectory|TestTrajectoryObligationsAnswersWaitingOn|TestWire'`
  and
  `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcher'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (processor request-resolution observer surface; code + focused tests).
  Claim: the next smallest honest construct is not yet the reconcile flip,
  but the concrete request-resolution certificate on the payload sourcecycled
  already polls.
  Position: pass 10 gave the route trajectory obligations on
  `/internal/runtime/runs/{id}`, but the payload still lacked the durable
  processor request-resolution certificate that distinguishes
  `awaiting_source_item_decisions`, story-route completion, and the
  published-corpus-coverage terminal branch.
  Blind spot reduced: internal run status now carries that processor
  request-resolution state, and the underlying request item is initialized
  with `awaiting_source_item_decisions` at processor run start so the
  observer can see the certificate immediately.
  Move: construct.
  Bound: no reconcile flip, no publication-settlement claim, no consumer-rule
  claim, no cross-store atomicity claim. This construct covers only the
  observer certificate.
  Update: landed the processor-resolution payload in
  `internal/runtime/api.go`, initialized the durable request item in
  `internal/runtime/wire_publication.go`, and added focused runtime/sourcecycled
  tests in `internal/runtime/api_test.go` and `cmd/sourcecycled/main_test.go`;
  focused tests passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestHandleInternalRunStatusIncludesTrajectoryObligations|TestHandleInternalRunStatusIncludesProcessorResolutionTerminalBranch|TestInternalRuntimeRunRoutesRequireInternalCallerAndConstrainProfiles'`,
  `nix develop -c go test ./internal/runtime -run 'TestRecordWireProcessorDecisionToolRecordsPerSourceItemNonPublicationVerdict|TestRecordWireProcessorDecisionToolRejectsAlreadyCoveredWithoutPublishedDoc|TestProcessorVTextRouteRequiresExplicitSourceItemsForMultiItemRequest|TestProcessorMixedPerItemDecisionsCompleteRequestOnceStoryRouteExists|TestProcessorSpawnMintsPublicationTrajectory|TestTrajectoryObligationsAnswersWaitingOn|TestWire'`,
  and
  `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcher'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (successful publish stamps `status=settled`; code + focused tests).
  Claim: the next smallest honest construct is to stop the successful
  publication path at an actual lifecycle transition instead of leaving it at
  "ready to settle" forever.
  Position: pass 11 gave the route trajectory obligations and the
  request-resolution certificate on the internal status payload, but
  successful publication still ended with zero open work items and both
  required refs present while the trajectory status remained `live`.
  Blind spot reduced: the successful publish path now uses the explicit
  two-phase protocol rather than a silent pseudo-transaction. After the story
  revision and edition mutation are durable, runtime patches the trajectory
  refs, completes the publication/story obligations, reevaluates the
  publication rule, and stamps `status=settled` when that verdict is earned.
  Move: construct.
  Bound: no cross-store atomicity claim, no new suppression semantics, no
  general consumer-rule claim. This construct covers only the successful
  publication branch.
  Update: landed the runtime-side settlement stamp in
  `internal/runtime/wire_publication.go` with focused proof in
  `internal/runtime/wire_publication_test.go`:
  `nix develop -c go test ./internal/runtime -run 'TestWireAutonomousPublishTranscludesEditionAndDebounces|TestWirePlatformPublishFailsClosedWithoutEditionWhenPlatformdFails'`.
  Exit: open_handoff.

- 2026-06-12 CONSTRUCT (explicit no-story terminal semantics; code + focused tests).
  Claim: the next smallest honest construct is to stop explicit no-story
  processor outcomes from hanging forever behind a completed run while still
  keeping truly deferred outcomes visibly open.
  Position: pass 12 made successful publication trajectories reach
  `status=settled`, but explicit `not_newsworthy` /
  `insufficient_evidence` outcomes still had no terminal protocol, and
  `deferred` was not distinguished from terminal no-story decisions.
  Blind spot reduced: explicit terminal no-story outcomes now complete the
  request-resolution item and cancel the trajectory, while `deferred`
  remains a named open branch (`all_source_items_deferred_without_story_route`)
  instead of silently sharing the same state.
  Move: construct.
  Bound: no cross-store atomicity claim, no new batch object, no general
  sourcecycled consumer-rule claim. This construct covers only the runtime
  terminal semantics and observer surface for explicit non-story outcomes.
  Update: landed the explicit no-story terminal/deferred-open split in
  `internal/runtime/wire_publication.go` with focused proof in
  `internal/runtime/wire_processor_decision_test.go` and
  `internal/runtime/api_test.go`:
  `nix develop -c go test ./internal/runtime -run 'TestRecordWireProcessorDecisionToolRecordsPerSourceItemNonPublicationVerdict|TestRecordWireProcessorDecisionToolCancelsExplicitNoStoryTerminalBranch|TestRecordWireProcessorDecisionToolKeepsDeferredBranchOpen|TestHandleInternalRunStatusIncludesProcessorResolutionTerminalBranch|TestHandleInternalRunStatusIncludesExplicitNoStoryTerminalBranch|TestWireAutonomousPublishTranscludesEditionAndDebounces|TestWirePlatformPublishFailsClosedWithoutEditionWhenPlatformdFails'`.
  Exit: open_handoff.

**version / lineage:** v0. Predecessors:
`docs/mission-trajectory-model-v0.md` (M1) and
`docs/mission-wire-on-settlement-v0.md` (split receipts). Successor
consumer: `docs/mission-wire-on-settlement-v0.md`.

**learning state:** retained here. This mission exists because repeated
M5 passes showed the witness was ahead of the architecture: current store
topology cannot honestly provide the implied transaction, and coverage
decisions are still implicit. New in pass 1: the repo now has durable
trajectory-level publication/edition refs and a tighter publication
settlement rule, but still lacks the decision ledger and explicit substrate
choice that would make later settlement claims sound. New in pass 2: the
publish/edition path itself now carries an in-flight work item, so failed
publishes no longer disappear from the obligations query, but the broader
decision ledger is still absent. New in pass 3: the repo's existing
`worker_updates` surface is not that missing ledger; it is durable and
trajectory-scoped, but optional and prose-shaped, so using it as a verdict
surface would create a fake island. New in pass 4: the missing decision
ledger does not need a second root object, because processor-spawned VText
runs and downstream worker updates already stay on the same publication
trajectory. New in pass 5: processor-opened article work no longer stays
invisible until external publish; it now opens a durable story-resolution
work item on the publication trajectory at VText handoff. The remaining gap
is the explicit suppression/non-publication verdict. New in pass 6: even a
processor request that never opens VText now leaves a durable open
request-resolution obligation on the publication trajectory. New in pass 7:
that request-scoped obligation can now carry a typed processor verdict, and
the processor → VText path auto-resolves the generic request item into the
more specific story-resolution item. The remaining gap is no longer "there
is no typed suppression verdict at all"; it is that the typed verdict is
still request-scoped and does not yet supply an honest suppression
settlement path. New in pass 8: that gap is narrower again. The decision
surface is now per-source-item, and multi-item VText handoffs must name the
exact covered source items. What remains missing is no longer item identity
or typed verdict shape; it is the trajectory-level terminal protocol for
all-suppressed requests. New in pass 9: the route now has one honest
terminal branch. `already_covered` must point at published coverage, and a
fully covered batch can end by completing the request item and cancelling
the trajectory. What remains missing is the sibling terminal semantics for
other non-publication verdicts and the later consumer rule that will read
this distinction. New in pass 10: the internal run-status payload now
carries trajectory obligations directly, so the observer-surface question is
reduced; the remaining gap is the honest consumer predicate on that now-live
surface. New in pass 11: that same payload now carries the processor
request-resolution certificate from request start through the
published-corpus-coverage branch, so the remaining gap is narrower again:
not missing observer data, but the honest composition rule over the fields
that are now visible. New in pass 12: the successful publication path no
longer stops at readiness substrate; it now explicitly stamps
`trajectory.status=settled` after the VText-side publish/edition work and the
runtime-side obligation closure both land. The remaining gap is no longer
"can publication success ever reach settled honestly?" but how far that
named two-phase protocol should extend before broader settlement claims are
made. New in pass 13: explicit `not_newsworthy` /
`insufficient_evidence` outcomes no longer masquerade as unresolved forever;
they now cancel the trajectory, while `deferred` is a separately visible open
branch. The remaining gap is no longer "do sibling no-story verdicts have any
terminal semantics at all?" but whether `deferred` should always stay open
and how sourcecycled should consume that distinction.

**settlement:** not started. Exit requires a chosen settlement substrate and
decision-record design, code and tests at the scope claimed, and if behavior
changes land, the same landing proof discipline required by `AGENTS.md`.
