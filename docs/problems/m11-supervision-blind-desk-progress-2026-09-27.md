# M11: supervision is blind to desk progress — reports queue durably but never wake a supervisor

Date: 2026-09-27
Class: red (actor wake authority, canonical lifecycle packets)
Status: repaired in this session — all three edges wired; regression tests
landed (doc-cast texture subject mint, progress-overdue emit/dedupe/stale
anchor, deadline handler emit). Deployed proof pending on staging.
Owner direction: the deleted 200-iteration tool-loop cap's replacement is
management/texture observing desk progress — or its absence
(`61bd89fc`). This receipt documents why that is currently impossible.

## Observed

Probe `M11_RESTART_PROOF_1790506567900` on staging `88419f88`
(computer `computer-a6033108eae4342f653ffb418343f219`, op
`selfdev-c94250603e50fbfddeac0f3a8d7adf1a`): a restart-cancelled
engineering assignment recast correctly at attempt 2 and re-executed for
27 minutes — and **no actor observed any of it**. The desk ran, emitted
zero progress packets a supervisor could see, hit the (pre-`61bd89fc`)
200-iteration ceiling, and was cancelled with the episode reaching only
`failed`. Supervision saw a disposition row, nothing else.

## The three dead edges (verified in source 2026-09-27)

1. **Engineering → persistent Management: queued, never wakes.**
   `wakeUpdatedCoagent` returns early for any `ProducerReport` packet
   targeting `management:<owner>`
   (`internal/agentcore/management_controller.go:2656-2657`). The report
   is durable and injectable into an already-running Management turn
   (`pendingCoagentUpdatesForRun`, `:1914-1935`), but nothing starts or
   resumes Management to consume it. On a fresh computer — where
   Management has never run — every desk report is invisible until an
   unrelated Texture control happens to open Management.

   Provenance: suppression added in `bbf9edd6` (Definition 1, 2026-09-02)
   to stop the Super continuation storm (`3654d925`): reports must never
   *mint* a run. The suppression over-approximates — it also blocks the
   safe case, waking an already-bound parked Management run to inject
   its queued report.

2. **Document-cast Engineering → supervision: dead mailbox.**
   `buildEngineeringReturnPacket` targets
   `assignment.Binding.ParentAgentID`
   (`internal/store/engineering_assignments.go:1673`) — for document
   casts that is `engineering:<docID>`, an agent that *never runs*
   (`engineering_desk.go:15-17` "the desk agent never runs"). Terminal
   and partial reports from the document channel land in a mailbox
   nothing reads. Texture — the document's owner — cannot see its own
   desk's reports even though `ValidateLifecycleProducerReportAuthority`
   already accepts `agentprofile.Engineering`
   (`texture_lifecycle_api.go:578-591`) and the Texture occurrence
   resolver already handles producer reports
   (`texture_controller.go:1085-1144`).

3. **Silence is unobservable.** Wake-on-report can only fire when a
   report commits. A stuck or slow desk produces no wake and no
   projection change: `EngineeringAssignment` rows carry disposition but
   no `last_report_at`/progress signal, and no deferred observation
   exists to mark a bound assignment `progress_overdue`. Management and
   Texture therefore cannot distinguish "working" from "wedged" — the
   exact judgment the removed iteration cap was masking.

## What this breaks

M11's self-development gate requires supervision to observe the episode:
partial evidence during execution, the terminal report, and the stalled
case. Today only the disposition row is visible; the report content
itself can sit undelivered forever (document casts) or wait on an
unrelated wake (delegated casts).

## Invariants the fix must keep

- Reports are never execution authority: no run is minted from a
  producer report; a report wake only resumes/binds an already-existing
  parked/running Management run, or stays durable-pending when none
  exists (Definition 1, `bbf9edd6`, `3654d925`).
- One committed report, one observation: retargeting must not fork the
  packet — the document-cast return packet targets `texture:<docID>`,
  and the Engineering→Management wake binds exact update identity.
- Texture producer-report validation
  (`texture_controller.go:1085-1144`, `texture_lifecycle_api.go:578-591`)
  stays the admission boundary; no new authority is granted.
- The stall observation is a supervision signal, not auto-cancel:
  `progress_overdue` informs; disposition stays with the actors.

## Fix shape (consensus panel 2026-09-27, adjudicated)

- Lift the `wakeUpdatedCoagent` suppression narrowly: a producer report
  wake resolves a parked/active persistent-Management run and resumes it
  via the existing `coagent_result` path; with no run it stays
  durable-pending (still no minting). The `sha256:` content collision
  with the control-occurrence resolver (`actorruntime/handler.go:433`,
  which scans controls only at `management_controller.go:2602`) must be
  split: report wakes fall through to the generic resume path.
- Retarget `buildEngineeringReturnPacket` for `ParentRunID == ""`
  (document casts) to `texture:<docID>` so Texture's existing
  producer-report occurrence path consumes desk reports.
- Add a progress observation: `last_report_at`-style derivation on the
  assignment plus a deferred `progress_overdue` wake via the existing
  `scheduleContinuation` machinery — re-armed on each accepted report,
  firing one idempotent observation packet to the supervisor when a
  bound assignment stays silent past the review window.

## Repair landed (this session, uncommitted at write time)

- **Edge 1** (`wakeUpdatedCoagent`): producer reports resume a resident
  parked/active Management run and return without dispatch when none exists —
  no minting, per the storm receipt. `ResolvePersistentManagementLiveOccurrence`
  splits report occurrences via `ErrPersistentManagementReportOccurrence`, and
  `persistentManagementReportOccurrencePending` scans both pending halves:
  undelivered (`ListAllPendingLifecycleUpdates`) and delivered-but-unconsumed
  (`ListDeliveredPendingProducerReports`). The actor handler falls through to
  the generic parked-run resume; with no memory it reconciles without minting
  (the suppression's early-return now *precedes* dispatch, closing a re-entry
  where the removed suppression would have fallen through to `dispatchActor`
  and minted).
- **Edge 2** (`buildEngineeringReturnPacket`): document casts retarget the
  report packet to `texture:<docID>` and the new
  `work:texture-supervision:<docID>` work item; both objects are minted
  atomically inside the first report/cancel commit
  (`textureSupervisionSubject`), so the producer-report occurrence path
  resolves an existing authority. `DeliveredAt` is now set only for a real
  live parent run. Producer authority validators accept self-channeled
  Engineering and persistent Management producers
  (`texture_controller.go`, `texture_lifecycle_api.go`).
- **Edge 3** (`actorWakeOutboxFromObject` + `EmitEngineeringProgressObservation`
  + `HandleEngineeringProgressOverdueDeadline`): every bound+active
  assignment commit derives an `engineering_progress_overdue_deadline` wake
  at `UpdatedAt + EngineeringProgressReviewWindow` (30m default,
  `CHOIR_PROGRESS_DEADLINE` override). The handler re-reads durable state —
  superseded anchors, unbound, terminal, or fate-pending assignments no-op —
  and commits exactly one ProducerReport-shaped observation packet per
  silence anchor, deduplicated by CommandID.
