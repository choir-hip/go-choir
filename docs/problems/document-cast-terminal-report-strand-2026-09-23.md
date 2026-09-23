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

**Primary:** the model passed its summary prose as the `verdict` argument
to `choir.Complete(result, verdict, summary, …)` — a positional-argument
confusion. `Tray.Complete` validated `result` but not `verdict`, so free
text staged fine. The saga committed freeze+revoke, then
`RecordCoSuperAssignmentReport` → `ValidateAgainst` rejected the non-`none`
verdict on an implementation assignment — after the capsule was already
revoked. Every retry (in-flight loop, fate watchdog, stranded-frozen sweep,
restart reconcile) replays the same staged proposal and fails identically.
The staged proposal's `verdict` field confirms: it carries the model's
full summary text, not a typed verdict.

**Secondary (same strand, found during diagnosis):**
`RecordCoSuperAssignmentReport` and `CancelCoSuperAssignment` decoded
`parentAuthority.parentRun` unconditionally; document-parented bindings
(`binding.ParentRunID == ""`) carry no parent run, so the return-packet
build would have failed on `unexpected end of JSON input` had the verdict
been valid. And `recordAssignedCoSuperReportOnce` treated
`CapsuleDisposition == Revoked` as late fate even when the staged
`PendingProposal` matched — routing resumes through a binder whose
per-command `SourceTreeDigest == SubjectDigest` check is impossible for
multi-command reports.

## Resolution

- `eaaacfaf` — `coSuperParentReturnTarget` resolves the return-packet
  parent run/channel from the binding; `lateFate` excludes the saga's own
  pending proposal.
- `aa835d12` — the revoked strand binds durable raw execution receipts on
  resume; `bindLateAssignmentExecutionReceipts` drops the per-command
  `SourceTreeDigest == SubjectDigest` check (only the first command's
  source tree equals the binding subject).
- verdict-validation commit — `Tray.Complete` rejects a non-enum verdict
  in-cell (model sees the error and retries); `commitCompleteIntent`
  fail-fasts on a typed-but-wrong verdict before the saga stages.

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
