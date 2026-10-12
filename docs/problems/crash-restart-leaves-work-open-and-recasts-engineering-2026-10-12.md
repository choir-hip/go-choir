# A crash restart leaves work open and can resume engineering work

Found 2026-10-12 during read-only design research (studies B and G).
Status: documented, not fixed. Both halves contradict the standing rule in
CLAUDE.md, "Restarts End Work (Crash) Or Resume It (Planned Update)": at a
markerless boot every open obligation (run, wake, control, directive,
assignment, work item) is closed with the visible fate "interrupted by a
restart", never re-dispatched.

## 1. Nothing closes open work items at a crash boot

A markerless boot closes **actor wakes** of work kinds only, with the fate
`interrupted_by_restart` (internal/actorruntime/handler.go:97-118, the
`restartWorkKinds` map and `preBootWorkOccurrence`). No boot path sets a
work item to cancelled. The only writers of `WorkItemCancelled` are
trajectory cancel and the engineering terminal projection. Open work items
and pending packets therefore survive a crash.

Boot paths then **read** that surviving open work and act on it (study B
§1(f), from code; not yet observed on staging):

- spawned-work open replay (internal/agentcore/runtime.go:2405-2410);
- terminal settlement repair (runtime.go:2510-2551);
- wire-publication claim recovery (wire_publication.go:21-43);
- the Texture wake re-arming on open desk work
  (texture_controller.go:638-660, 735-748).

So the wake is closed, but the obligation it served stays open, and other
paths can pick it back up.

## 2. A restart re-opens a restart-cancelled engineering assignment

At restart, executor binding reconciliation revokes an assignment's absent
capsule and cancels it with the reason "restart revoked absent assignment
capsule" (internal/agentcore/engineering_assignment_fate.go:530-548). The
engineering desk's reconcile then treats that cancel as system fate, not a
decision, and **re-casts the assignment at the next attempt**, up to
`engineeringMaxRecastAttempts = 3` (engineering_desk.go:45-50, 213-279).
Neither path checks the planned-restart marker. A crash restart therefore
resumes engineering work, which is the crash-loop shape the rule forbids;
the attempt cap only bounds the loop.

Verified from code; how often it fires on staging is not yet observed.

## Why it matters

- The owner's rule is the defence against crash-restart loops. Today it
  holds for wakes and fails for the obligations behind them.
- Work items have a bug cluster already (five closure bugs in one day,
  docs/problems/clustering-texture-obligation-closure-2026-10-09.md).
  Patching crash-close into work items would extend a subsystem the agreed
  design deletes.

## Hypotheses for the fix

1. Records-as-work (agreed direction 2026-10-12): at a markerless boot,
   append a system fate record "interrupted by a restart" to every open
   Cast thread and terminalize its pending packets; a planned-restart
   marker skips this. Work items are then a derived view and later deleted.
2. The engineering restart recast belongs to "path B" (an owner edit to an
   engineering-bound document goes straight to engineering), which the
   agreed design dissolves into owner → Texture → management →
   engineering. Deleting path B deletes the recast. Interim alternative:
   gate the recast on the planned-restart marker.

Mutation class of a fix: red (run lifecycle, evidence ledger, restart
semantics). Studies: docs/reports/re-engineering-studies-2026-10-12/
B-work-items-as-records.md §1(f) and G-affordance-audit-runtime-pass.md §4.
