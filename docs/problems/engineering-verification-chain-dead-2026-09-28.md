# Engineering verification chain dead — canonical ref miss + agent-head gate wedge ops at `frozen`

Date: 2026-09-28
Status: **problem documented; repair lands next commit.**
Mutation class: **red** (engineering-desk assignment authority, self-development
operation state machine — a run-acceptance-adjacent surface).

## Observed on staging (deployed `b85af274`)

M11 episode probe `M11_SELFDEV_EPISODE_1790628310490`,
computer `computer-94b04ad5c86c4b687b47c4ea8d779594`, operation
`selfdev-ffbdd629334eeb6bb12aac56407f4c0f`:

- Implementation assignment `assignment-99883ab1-b394-5df0-8455-3e81d65f1abb`
  completed at 21:00:21Z (seq 8 `co_super_assignment_reported`); the report
  carries `candidate_id` (`obj:choir.co_super_subject_candidate:…`).
- The bound self-development operation reached `frozen` with
  `bundle_digest` at 21:00:07Z.
- `reconcileEngineeringVerification` should have opened the verification
  assignment host-side at report commit (`rlm_reduce.go:1194` →
  `ReconcileEngineeringDeskForTrajectory`). No verification assignment was
  ever opened; the operation has been stuck at `frozen` for ~35 minutes
  with zero follow-on runs.

## Defect 1: report refs never resolve

`internal/agentcore/engineering_desk.go:276-289` —
`reconcileEngineeringVerification` iterates `implementation.ReportRefs`,
calls `objectgraph.ParseCanonicalID(ref)`, takes the **suffix**, and loads
`GetEngineeringAssignmentReport(owner, computer, suffix)`.

The suffix of a canonical object ID is `key-<sha256(identity key)>`
(`objectgraph.StableSuffixFromKey`), not `report.ReportID`
(`report:sha256:…`). `GetEngineeringAssignmentReport` keys objects by
`ReportID` (`engineering_assignments.go:2457` → `lifecycleGetObject`).
The lookup misses every time, `candidateID` stays `""`, and the function
returns `nil, nil` — a silent skip with no log and no event.

Introduced by `98c6d96e` (M2 document-bound desk cutover, 2026-09-23).
Verification chaining has **never** opened on a document-bound op.
The correct helper already exists and is used by the restart-recast
sibling path: `GetEngineeringAssignmentReportByCanonicalID`
(`engineering_assignments.go:2475`, called from `engineering_desk.go:75`
via `restartRecastReportRef`).

## Defect 2: agent-headed document gate deadlocks chaining

`internal/agentcore/engineering_desk.go:147-150` —
`ReconcileEngineeringDesk` early-returns unless the document head revision
is `AuthorUser`. Once the desk's own supervision turn lands a revision
(this episode: appagent revision v1 at 21:19:58Z), every subsequent
reconcile — including the restart-recast path that exists to recover
exactly this shape — no-ops. Verification owed to a completed
implementation is gated on a property of the *cast admission* decision
(owner-authored head), not of the verification obligation.

Combined effect: verification can open only in the narrow window where the
report-commit reconcile runs while the head is still owner-authored AND
the ref lookup works. Both failed here; the op is terminally wedged —
no event, no error, no retry.

## Repair (next commit)

1. Resolve report refs by canonical ID
   (`GetEngineeringAssignmentReportByCanonicalID`), matching the
   restart-recast convention — one loader for `ReportRefs`.
2. Split the head gate: cast admission still requires an owner-authored
   head; verification chaining on a completed implementation runs
   regardless of head authorship (it binds the admitting revision via
   `ParentControlID`, never the head).

Refs: `docs/problems/m11-engineering-desk-deferral-2026-09-27.md`,
`docs/problems/m11-restart-passivation-cancels-desk-strands-op-2026-09-27.md`,
`docs/definitions/choir-selfdev-gate-2026-09-27.md` (M11),
`docs/evidence/m11-episode-call-map-2026-09-27.md`.
Mission: `docs/definitions/choir-sub-rlm-document-channel-2026-09-22.md` (M2).
