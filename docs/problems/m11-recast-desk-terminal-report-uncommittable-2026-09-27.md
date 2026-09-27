# M11 follow-on blocker: restart-recast desk run's terminal report is uncommittable; retries corrupt receipt cardinality

**Status:** open, blocking M11 episode leg `primary_started -> awaiting_approval`
(the desk completes its work but cannot land the terminal report).
**First observed:** 2026-09-27, staging deploy `530fcdb2` (the restart-recast
repair), probe run `m11_selfdev_episode_probe.mjs` op
`selfdev-8ddedccc4e7335ff20933decd9d19302`, computer
`computer-debba20a4d99de89cfd7c17174575e5b`, VM `vm-406350e594d5ff4b87a1e7350c8285ab`,
assignment `assignment-6b963d5c-55be-51b6-9c3c-091cd011f293`, trajectory
`trajectory-c4ec96ab9c8154e8a5c32ef75f8fe0f2`.

## Context

This is the *successor* failure to
`m11-restart-passivation-cancels-desk-strands-op-2026-09-27.md`. The option-A
fix landed (`301fcc21` + `530fcdb2`): a restart-cancelled cast now recasts at
attempt N+1. Today's forced restart exercised it end-to-end and proved the
repair — this receipt documents the next defect the same probe surfaced.

## Evidence

Timeline (Node B journalctl, guest `vm-406350e5`):

```
09:39:17 vmctl hibernated VM epoch=12732
09:39:40 fc_vcpu resume on new exec PID; tap re-bound 10.200.5.2 -> 10.200.8.2
09:39:46 autoputer: projection recovery resume for computer-debba20a (local=743 W=0 H=743)
09:39:52 runtime: passivated run run:assignment-6b963d5c-… (was running) after restart
09:39:52 runtime: boot phase engineering_assignment_capsules dur=146ms   <- restart-cancel + revoke
09:41+   desk run re-executing (deepseek-v4.1-flash inference resumes,
         tool-loop iterations climb 1 -> 93 over ~24 min)
10:03:11 terminal report attempt failed x4:
         disposition=bound capsule=revoked version=6
         err[1] = "invalid runtime execution attestation: invalid transition"
         err[2..4] = "timely grant-attested command evidence requires exact
                    receipt and attestation cardinality: invalid transition"
10:05:41 op selfdev-8ddedccc… -> state=failed, terminal_error=null,
         bundle_digest=null, verifier_refs=[]
```

Interpretation:

1. The restart-cancel + revoke fired exactly as designed; the recast opened
   attempt 2; the desk ran the full episode to completion (`tool loop:
   iteration 93` then the terminal report).
2. **First report attempt** — `validateExecutionAttestations`
   (`internal/store/engineering_assignments.go:431-449`) rejected one
   attestation field. Candidates that can differ on a recast binding:
   `SourceSubjectDigest != assignment.Binding.SubjectDigest`,
   `FinalSubjectDigest != report.ObservedSubjectDigest`,
   `CapsuleID`/`RunID`/`Attempt` vs the new binding. Exact field is not yet
   isolated — the error is wrapped without naming it.
3. **Retries 2–4** — `commitAssignedEngineeringReport`
   (fate.go:1075-1103) re-enters after `ErrEngineeringAssignmentInvalid`,
   and the report object has already been mutated:
   `bindFrozenAssignmentExecutionReceipts` (fate.go:1042-1072) does
   `report.ExecutorReceiptRefs = append(...)` and
   `report.ExecutionAttestations = append(...)` per call, so a retried
   commit sees `len(refs) > len(commands)` -> permanent cardinality
   failure. The first failure is unrecoverable by construction.
4. The op then transitioned `executing -> failed` at 10:05:41 with
   `terminal_error=null` — the strand detection works, but the failure is
   silent (no error text on the op record).

## Diagnosed causes

Two distinct defects, one latent pre-existing:

- **(new) Recast attestation/binding mismatch.** The attempt-2 report's
  minted `EngineeringExecutionAttestation`s fail field validation against
  the attempt-2 `assignment.Binding`. Most probable: `SourceSubjectDigest`
  — attempt-2 bound the capsule at the attempt-1 *final* subject (or the
  tape head), while `engineeringExecutionAttestationFromReceipt`
  (fate.go:1026-1028) requires `receipt.SourceTreeDigest ==
  Binding.SubjectDigest` and the validator requires
  `att.SourceSubjectDigest == Binding.SubjectDigest`; if the recast re-bound
  the *original* revision subject while the desk executed against a
  different base, every command attestation is invalid. Not yet proven which
  field — needs either field-naming in the error or a repro dump.
- **(pre-existing, now load-bearing) Non-idempotent report binding.**
  `bindFrozenAssignmentExecutionReceipts` and
  `bindLateAssignmentExecutionReceipts` mutate `report` in place and append;
  `recordAssignedEngineeringReport` retries up to 4× on
  `ErrEngineeringAssignmentInvalid`/`ErrConcurrentStateChange` without
  restoring `report` between attempts. Any first-attempt rejection —
  including the transient `ErrConcurrentStateChange` the retry loop exists
  for — permanently poisons the report. This bug existed before the recast
  work but only becomes reachable now that a recast can produce a first
  attempt that fails validation.

## Blast radius

Any restart-recast desk run whose terminal report hits a first-attempt
validation or CAS failure can never commit: the op fails after the desk's
full work product is produced. On staging this fires on the same deploy
churn that motivated the recast fix — the recast moved the strand point
from "never re-executes" to "executes then fails to commit".

## Fix options (not yet decided)

- **A.** Name the failing field: extend the `validateExecutionAttestations`
  wrap with the offending field index/name so the next repro is
  self-diagnosing (evidence repair, near-zero risk).
- **B.** Snapshot/restore `report` around each
  `recordAssignedEngineeringReportOnce` attempt (or re-derive
  ExecutorReceiptRefs/ExecutionAttestations fresh per attempt) so retry is
  idempotent. Bug class: in-place mutation across a retry loop.
- **C.** Reconcile the recast binding subject with the subject the desk
  actually executes against: if attempt-2 intentionally re-bases the
  capsule, the attestation `SourceSubjectDigest` contract must bind to the
  executed base, not the original admit subject. Requires the A evidence
  first.
- **D.** Populate `terminal_error` on the op's `executing -> failed`
  transition so silent report-commit failures are operator-visible.

A+B are the convergent pair: A makes the recast-specific mismatch legible,
B removes the poison-once-fail-forever retry hazard that turned one bad
attempt into four permanent ones.

## What proves closure

A forced hibernate/resume mid-desk-run on staging produces a recast
attempt whose terminal report commits (op reaches `awaiting_approval`),
or — if attestation validation legitimately rejects — the rejection is
named in the guest log and the op's `terminal_error`, with no cardinality
noise on retry.

## Recovery posture

- Stranded/failed ops are disposable fresh-owner computers; no data risk.
- The probe is resumable without a browser session: the guest API accepts
  `X-Authenticated-User`/`X-Authenticated-Computer` on the guest's
  `10.200.x:8085` surface and corpusd's `:8086` internal endpoints accept
  `X-Internal-Caller` — the full episode (awaiting_approval ->
  qualified_consensus arm -> approve -> applied -> candidate-B reject ->
  document render) was driven by `/tmp/m11_resume.mjs` during this session.
