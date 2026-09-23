# Document-Cast Terminal Report Strands After Capsule Revoke

Date: 2026-09-23
Status: documented before fix, per the problem-documentation-first invariant.
Found by the M2 staging acceptance probe (first real document-channel cast).
Mutation class of this document: green (analysis; no runtime change).

## Observed

On staging (`computer-03335285269bdba4f94377e56879f9e6`, deployed build
`98c6d96e`), an owner-authored revision on an engineering-bound Texture
document opened and bound assignment
`assignment-8bb64048-2c77-5821-a5fe-fa7c85bfab30` — the first real
document-channel sub-RLM cast. The assigned run executed the task end-to-end
(wrote `/workspace/platform/fib/main.go`, ran it, reported output) and
finished `completed`.

The terminal `choir.Complete` saga then stranded:

- Canonical events: `freeze_requested` → `frozen` → `revoke_requested` →
  `revoked` — the saga's own mid-flight dispositions, all committed.
- Missing: `co_super_assignment_reported`, `co_super_assignment_completed`.
- Assignment state: `disposition=bound`, `capsule_disposition=revoked`,
  `pending_proposal` staged — permanently.
- The model observed the refusal and reported "the terminal `choir.Complete`
  could not be recorded because the assignment was cancelled host-side."

## Root cause

`RecordCoSuperAssignmentReport` (internal/store/cosuper_assignments.go) and
`CancelCoSuperAssignment` unconditionally decode
`parentAuthority.parentRun` as a `types.RunRecord` to build the return
packet. For document-parented bindings (`binding.ParentRunID == ""`),
`requireCoSuperDocumentParentAuthority` returns `parentRun` unset — a zero
`objectgraph.Object` with nil `Body`. `decodeLifecycleObject` fails with
`unexpected end of JSON input`.

The failure lands *after* the saga already committed freeze + revoke
dispositions, so every retry — the in-flight retry loop, the fate watchdog,
the stranded-frozen sweep, the restart reconcile — re-enters at the same
post-revoke step and fails identically. The assignment is bound+revoked
forever; no report, no completion event, no verification chaining.

This path was never exercised before: M1's deployed acceptance used
Super-run parents (`assign_co_super` from a persistent Super run), where
`parentRun` is always set. The document channel is the first
document-parented assignment flow.

## Secondary defect (same strand)

`recordAssignedCoSuperReportOnce` computes
`lateFate = … || CapsuleDisposition == Revoked` without excluding the saga's
own staged `PendingProposal`. A resume after a post-revoke strand is routed
through `bindLateAssignmentExecutionReceipts`, which requires every raw
receipt's `SourceTreeDigest == Binding.SubjectDigest` — false for any
multi-command report (each command's source tree is the prior command's
result). Even when it succeeds, the store marks the report `Late`, which is
evidence-only and never completes the assignment. The store layer already
excludes the saga's own proposal via `pendingMatches`; the runtime gate did
not.

## Evidence

- Trajectory `a006d048-3946-51fa-a345-af5183b2ae7a` on staging: events 1–7
  (`trajectory_started`, `co_super_assignment_opened`,
  `co_super_assignment_bound`, four `co_super_capsule_disposition_set`).
- Assignment `assignment-8bb64048-2c77-5821-a5fe-fa7c85bfab30`:
  `disposition=bound`, `capsule_disposition=revoked`, `bound_loop_id` set,
  `parent_loop_id` empty (document parent), `parent_control_id` = the
  admitting revision `44eb989c-…`.
- Run `run:assignment-8bb64048-…`: `state=completed`, result text records
  the host-side refusal.

## Fix direction

1. Resolve the return-packet parent target from the binding: decode
   `parentRun` only when `ParentRunID != ""`; for document parents decode
   `parentAgent` and use its `ChannelID` (the bound document). The packet
   stays undelivered until a desk activation consumes it.
2. Exclude the saga's own pending proposal from the runtime `lateFate` gate,
   mirroring the store's `pendingMatches`, so a stranded resume re-enters
   the freeze/revoke path and commits the report non-late.
