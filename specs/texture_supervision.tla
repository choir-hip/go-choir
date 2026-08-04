-------------------------- MODULE texture_supervision --------------------------
(***************************************************************************)
(* Bounded safety model for the active one-tape Texture supervision goal.   *)
(*                                                                         *)
(* Implementation mapping:                                                 *)
(* - tape/projectionSeq/stage model ComputerEventAppender pin -> prepare -> *)
(*   corpusd CAS -> embedded finalize and RecoverPrepared.                  *)
(* - intent, assignment, attempt, disposition, dissent, finding, rebase,    *)
(*   and settlement variables model the proposed supervision/v1 projection. *)
(* - composed/pending/effective candidate variables model the future        *)
(*   one-current-base CapsuleEffectBundle seam. No actuator is invoked.      *)
(*                                                                         *)
(* Authority: docs/definitions/choir-texture-tape-supervision-2026-08-03.md *)
(* Evidence class: bounded model-check only. This cannot prove code/schema   *)
(* conformance, cryptography, deployed reconstruction, or owner usability.   *)
(***************************************************************************)

EXTENDS Integers, FiniteSets, Sequences, TLC

CONSTANTS Assignments, Attempts, Results, Candidates, MaxEvents

None == "none"
AssignmentStates == {"absent", "open", "cancelled", "closed"}
AttemptStates == {"absent", "running", "succeeded", "failed", "cancelled", "late"}
DispositionStates == {
  "none", "preserved", "invalidated", "superseded",
  "compensation_required", "cancelled", "late", "incorporated", "rejected"
}
Stages == {"idle", "prepared", "acknowledged"}

VARIABLES
  tape,
  projectionSeq,
  stage,
  writesEnabled,
  disabledAt,
  rollbackEventCount,
  intentRev,
  assignmentState,
  assignmentIntent,
  assignmentBase,
  attemptAssignment,
  attemptBase,
  attemptState,
  attemptResult,
  attemptDisposition,
  rebasePending,
  dissentOpen,
  findingOpen,
  settled,
  composedCandidate,
  composedBase,
  candidateVerified,
  acceptedBase,
  pendingCandidate,
  desiredCandidate,
  effectiveCandidate

vars == <<
  tape, projectionSeq, stage, writesEnabled, disabledAt, rollbackEventCount,
  intentRev, assignmentState, assignmentIntent, assignmentBase,
  attemptAssignment, attemptBase, attemptState, attemptResult,
  attemptDisposition, rebasePending, dissentOpen, findingOpen, settled,
  composedCandidate, composedBase, candidateVerified, acceptedBase,
  pendingCandidate, desiredCandidate, effectiveCandidate
>>

Init ==
  /\ tape = <<>>
  /\ projectionSeq = 0
  /\ stage = "idle"
  /\ writesEnabled = TRUE
  /\ disabledAt = 0
  /\ rollbackEventCount = 0
  /\ intentRev = 0
  /\ assignmentState = [a \in Assignments |-> "absent"]
  /\ assignmentIntent = [a \in Assignments |-> 0]
  /\ assignmentBase = [a \in Assignments |-> 0]
  /\ attemptAssignment = [t \in Attempts |-> None]
  /\ attemptBase = [t \in Attempts |-> 0]
  /\ attemptState = [t \in Attempts |-> "absent"]
  /\ attemptResult = [t \in Attempts |-> None]
  /\ attemptDisposition = [t \in Attempts |-> "none"]
  /\ rebasePending = {}
  /\ dissentOpen = FALSE
  /\ findingOpen = FALSE
  /\ settled = FALSE
  /\ composedCandidate = None
  /\ composedBase = 0
  /\ candidateVerified = FALSE
  /\ acceptedBase = 0
  /\ pendingCandidate = None
  /\ desiredCandidate = None
  /\ effectiveCandidate = None

CanAppend == writesEnabled /\ stage = "idle" /\ Len(tape) < MaxEvents

AppendProjected ==
  /\ tape' = Append(tape, Len(tape) + 1)
  /\ projectionSeq' = Len(tape) + 1
  /\ stage' = "idle"

OpenIntent ==
  /\ CanAppend
  /\ ~settled
  /\ pendingCandidate = None
  /\ AppendProjected
  /\ intentRev' = intentRev + 1
  /\ rebasePending' = {a \in Assignments : assignmentState[a] /= "absent"}
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, assignmentState,
       assignmentIntent, assignmentBase, attemptAssignment, attemptBase,
       attemptState, attemptResult, attemptDisposition, dissentOpen,
       findingOpen, settled, composedCandidate, composedBase,
       candidateVerified, acceptedBase, pendingCandidate, desiredCandidate,
       effectiveCandidate
     >>

OpenAssignment(a) ==
  /\ CanAppend
  /\ intentRev > 0
  /\ ~settled
  /\ assignmentState[a] = "absent"
  /\ AppendProjected
  /\ assignmentState' = [assignmentState EXCEPT ![a] = "open"]
  /\ assignmentIntent' = [assignmentIntent EXCEPT ![a] = intentRev]
  /\ assignmentBase' = [assignmentBase EXCEPT ![a] = Len(tape)]
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       attemptAssignment, attemptBase, attemptState, attemptResult,
       attemptDisposition, rebasePending, dissentOpen, findingOpen, settled,
       composedCandidate, composedBase, candidateVerified, acceptedBase,
       pendingCandidate, desiredCandidate, effectiveCandidate
     >>

StartAttempt(a, t) ==
  /\ CanAppend
  /\ assignmentState[a] = "open"
  /\ attemptState[t] = "absent"
  /\ AppendProjected
  /\ attemptAssignment' = [attemptAssignment EXCEPT ![t] = a]
  /\ attemptBase' = [attemptBase EXCEPT ![t] = Len(tape)]
  /\ attemptState' = [attemptState EXCEPT ![t] = "running"]
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptResult,
       attemptDisposition, rebasePending, dissentOpen, findingOpen, settled,
       composedCandidate, composedBase, candidateVerified, acceptedBase,
       pendingCandidate, desiredCandidate, effectiveCandidate
     >>

ReturnResult(t, r) ==
  /\ CanAppend
  /\ attemptState[t] = "running"
  /\ assignmentState[attemptAssignment[t]] = "open"
  /\ r \notin {attemptResult[x] : x \in Attempts}
  /\ AppendProjected
  /\ attemptState' = [attemptState EXCEPT ![t] = "succeeded"]
  /\ attemptResult' = [attemptResult EXCEPT ![t] = r]
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptDisposition, rebasePending, dissentOpen,
       findingOpen, settled, composedCandidate, composedBase,
       candidateVerified, acceptedBase, pendingCandidate, desiredCandidate,
       effectiveCandidate
     >>

FailAttempt(t) ==
  /\ CanAppend
  /\ attemptState[t] = "running"
  /\ AppendProjected
  /\ attemptState' = [attemptState EXCEPT ![t] = "failed"]
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptResult, attemptDisposition, rebasePending,
       dissentOpen, findingOpen, settled, composedCandidate, composedBase,
       candidateVerified, acceptedBase, pendingCandidate, desiredCandidate,
       effectiveCandidate
     >>

RetryAttempt(a, t) ==
  /\ CanAppend
  /\ assignmentState[a] = "open"
  /\ attemptState[t] = "absent"
  /\ \E prior \in Attempts :
       /\ attemptAssignment[prior] = a
       /\ attemptState[prior] = "failed"
       /\ attemptDisposition[prior] /= "none"
  /\ AppendProjected
  /\ attemptAssignment' = [attemptAssignment EXCEPT ![t] = a]
  /\ attemptBase' = [attemptBase EXCEPT ![t] = Len(tape)]
  /\ attemptState' = [attemptState EXCEPT ![t] = "running"]
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptResult,
       attemptDisposition, rebasePending, dissentOpen, findingOpen, settled,
       composedCandidate, composedBase, candidateVerified, acceptedBase,
       pendingCandidate, desiredCandidate, effectiveCandidate
     >>

CancelAssignment(a) ==
  /\ CanAppend
  /\ assignmentState[a] = "open"
  /\ AppendProjected
  /\ assignmentState' = [assignmentState EXCEPT ![a] = "cancelled"]
  /\ attemptState' = [t \in Attempts |->
       IF attemptAssignment[t] = a /\ attemptState[t] = "running"
       THEN "cancelled" ELSE attemptState[t]]
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentIntent, assignmentBase, attemptAssignment, attemptBase,
       attemptResult, attemptDisposition, rebasePending, dissentOpen,
       findingOpen, settled, composedCandidate, composedBase,
       candidateVerified, acceptedBase, pendingCandidate, desiredCandidate,
       effectiveCandidate
     >>

ReturnLateResult(t, r) ==
  /\ CanAppend
  /\ ~settled
  /\ attemptState[t] = "cancelled"
  /\ assignmentState[attemptAssignment[t]] = "cancelled"
  /\ r \notin {attemptResult[x] : x \in Attempts}
  /\ AppendProjected
  /\ attemptState' = [attemptState EXCEPT ![t] = "late"]
  /\ attemptResult' = [attemptResult EXCEPT ![t] = r]
  /\ attemptDisposition' = [attemptDisposition EXCEPT ![t] = "none"]
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, rebasePending, dissentOpen, findingOpen, settled,
       composedCandidate, composedBase, candidateVerified, acceptedBase,
       pendingCandidate, desiredCandidate, effectiveCandidate
     >>

DispositionAttempt(t, d) ==
  /\ CanAppend
  /\ attemptState[t] \in {"succeeded", "failed", "cancelled", "late"}
  /\ attemptDisposition[t] = "none"
  /\ d \in DispositionStates \ {"none"}
  /\ AppendProjected
  /\ attemptDisposition' = [attemptDisposition EXCEPT ![t] = d]
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, rebasePending, dissentOpen,
       findingOpen, settled, composedCandidate, composedBase,
       candidateVerified, acceptedBase, pendingCandidate, desiredCandidate,
       effectiveCandidate
     >>

ReconcileAssignment(a) ==
  /\ CanAppend
  /\ a \in rebasePending
  /\ AppendProjected
  /\ rebasePending' = rebasePending \ {a}
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       dissentOpen, findingOpen, settled, composedCandidate, composedBase,
       candidateVerified, acceptedBase, pendingCandidate, desiredCandidate,
       effectiveCandidate
     >>

RecordDissent ==
  /\ CanAppend
  /\ ~settled
  /\ ~dissentOpen
  /\ AppendProjected
  /\ dissentOpen' = TRUE
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       rebasePending, findingOpen, settled, composedCandidate, composedBase,
       candidateVerified, acceptedBase, pendingCandidate, desiredCandidate,
       effectiveCandidate
     >>

ResolveDissent ==
  /\ CanAppend
  /\ dissentOpen
  /\ AppendProjected
  /\ dissentOpen' = FALSE
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       rebasePending, findingOpen, settled, composedCandidate, composedBase,
       candidateVerified, acceptedBase, pendingCandidate, desiredCandidate,
       effectiveCandidate
     >>

RecordFinding ==
  /\ CanAppend
  /\ ~settled
  /\ ~findingOpen
  /\ AppendProjected
  /\ findingOpen' = TRUE
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       rebasePending, dissentOpen, settled, composedCandidate, composedBase,
       candidateVerified, acceptedBase, pendingCandidate, desiredCandidate,
       effectiveCandidate
     >>

ResolveFinding ==
  /\ CanAppend
  /\ findingOpen
  /\ AppendProjected
  /\ findingOpen' = FALSE
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       rebasePending, dissentOpen, settled, composedCandidate, composedBase,
       candidateVerified, acceptedBase, pendingCandidate, desiredCandidate,
       effectiveCandidate
     >>

CloseAssignment(a) ==
  /\ CanAppend
  /\ assignmentState[a] \in {"open", "cancelled"}
  /\ \A t \in Attempts :
       attemptAssignment[t] = a /\ attemptState[t] /= "absent"
       => attemptDisposition[t] /= "none"
  /\ AppendProjected
  /\ assignmentState' = [assignmentState EXCEPT ![a] = "closed"]
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentIntent, assignmentBase, attemptAssignment, attemptBase,
       attemptState, attemptResult, attemptDisposition, rebasePending,
       dissentOpen, findingOpen, settled, composedCandidate, composedBase,
       candidateVerified, acceptedBase, pendingCandidate, desiredCandidate,
       effectiveCandidate
     >>

SettleTrajectory ==
  /\ CanAppend
  /\ intentRev > 0
  /\ ~settled
  /\ rebasePending = {}
  /\ ~dissentOpen
  /\ ~findingOpen
  /\ pendingCandidate = None
  /\ \A a \in Assignments : assignmentState[a] \in {"absent", "closed"}
  /\ \A t \in Attempts :
       attemptState[t] = "absent" \/ attemptDisposition[t] /= "none"
  /\ AppendProjected
  /\ settled' = TRUE
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       rebasePending, dissentOpen, findingOpen, composedCandidate,
       composedBase, candidateVerified, acceptedBase, pendingCandidate,
       desiredCandidate, effectiveCandidate
     >>

ComposeCandidate(c) ==
  /\ CanAppend
  /\ c \in Candidates
  /\ composedCandidate = None
  /\ pendingCandidate = None
  /\ ~settled
  /\ rebasePending = {}
  /\ \A a \in Assignments : assignmentState[a] \in {"absent", "closed"}
  /\ \E t \in Attempts :
       attemptResult[t] \in Results /\ attemptDisposition[t] = "incorporated"
  /\ AppendProjected
  /\ composedCandidate' = c
  /\ composedBase' = Len(tape)
  /\ candidateVerified' = FALSE
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       rebasePending, dissentOpen, findingOpen, settled, acceptedBase,
       pendingCandidate, desiredCandidate, effectiveCandidate
     >>

VerifyCandidate ==
  /\ CanAppend
  /\ composedCandidate \in Candidates
  /\ composedBase + 1 = Len(tape)
  /\ ~candidateVerified
  /\ AppendProjected
  /\ candidateVerified' = TRUE
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       rebasePending, dissentOpen, findingOpen, settled, composedCandidate,
       composedBase, acceptedBase, pendingCandidate, desiredCandidate,
       effectiveCandidate
     >>

AcceptCandidate ==
  /\ CanAppend
  /\ composedCandidate \in Candidates
  /\ candidateVerified
  /\ composedBase + 2 = Len(tape)
  /\ pendingCandidate = None
  /\ AppendProjected
  /\ acceptedBase' = composedBase
  /\ pendingCandidate' = composedCandidate
  /\ desiredCandidate' = composedCandidate
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       rebasePending, dissentOpen, findingOpen, settled, composedCandidate,
       composedBase, candidateVerified, effectiveCandidate
     >>

MaterializeApplied ==
  /\ CanAppend
  /\ pendingCandidate \in Candidates
  /\ AppendProjected
  /\ effectiveCandidate' = pendingCandidate
  /\ desiredCandidate' = pendingCandidate
  /\ pendingCandidate' = None
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       rebasePending, dissentOpen, findingOpen, settled, composedCandidate,
       composedBase, candidateVerified, acceptedBase
     >>

MaterializeFailed ==
  /\ CanAppend
  /\ pendingCandidate \in Candidates
  /\ AppendProjected
  /\ desiredCandidate' = effectiveCandidate
  /\ pendingCandidate' = None
  /\ UNCHANGED <<
       writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       rebasePending, dissentOpen, findingOpen, settled, composedCandidate,
       composedBase, candidateVerified, acceptedBase, effectiveCandidate
     >>

PrepareObservation ==
  /\ writesEnabled
  /\ stage = "idle"
  /\ Len(tape) < MaxEvents
  /\ stage' = "prepared"
  /\ UNCHANGED <<
       tape, projectionSeq, writesEnabled, disabledAt, rollbackEventCount,
       intentRev, assignmentState, assignmentIntent, assignmentBase,
       attemptAssignment, attemptBase, attemptState, attemptResult,
       attemptDisposition, rebasePending, dissentOpen, findingOpen, settled,
       composedCandidate, composedBase, candidateVerified, acceptedBase,
       pendingCandidate, desiredCandidate, effectiveCandidate
     >>

AcknowledgeObservation ==
  /\ stage = "prepared"
  /\ tape' = Append(tape, Len(tape) + 1)
  /\ stage' = "acknowledged"
  /\ UNCHANGED <<
       projectionSeq, writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       rebasePending, dissentOpen, findingOpen, settled, composedCandidate,
       composedBase, candidateVerified, acceptedBase, pendingCandidate,
       desiredCandidate, effectiveCandidate
     >>

FinalizeOrRecoverObservation ==
  /\ stage = "acknowledged"
  /\ projectionSeq' = Len(tape)
  /\ stage' = "idle"
  /\ UNCHANGED <<
       tape, writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       rebasePending, dissentOpen, findingOpen, settled, composedCandidate,
       composedBase, candidateVerified, acceptedBase, pendingCandidate,
       desiredCandidate, effectiveCandidate
     >>

DiscardUnacknowledged ==
  /\ stage = "prepared"
  /\ stage' = "idle"
  /\ UNCHANGED <<
       tape, projectionSeq, writesEnabled, disabledAt, rollbackEventCount,
       intentRev, assignmentState, assignmentIntent, assignmentBase,
       attemptAssignment, attemptBase, attemptState, attemptResult,
       attemptDisposition, rebasePending, dissentOpen, findingOpen, settled,
       composedCandidate, composedBase, candidateVerified, acceptedBase,
       pendingCandidate, desiredCandidate, effectiveCandidate
     >>

DisableWrites ==
  /\ stage = "idle"
  /\ writesEnabled
  /\ writesEnabled' = FALSE
  /\ disabledAt' = Len(tape)
  /\ UNCHANGED <<
       tape, projectionSeq, stage, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       rebasePending, dissentOpen, findingOpen, settled, composedCandidate,
       composedBase, candidateVerified, acceptedBase, pendingCandidate,
       desiredCandidate, effectiveCandidate
     >>

RebuildProjection ==
  /\ stage = "idle"
  /\ ~writesEnabled
  /\ projectionSeq' = Len(tape)
  /\ UNCHANGED <<
       tape, stage, writesEnabled, disabledAt, rollbackEventCount, intentRev,
       assignmentState, assignmentIntent, assignmentBase, attemptAssignment,
       attemptBase, attemptState, attemptResult, attemptDisposition,
       rebasePending, dissentOpen, findingOpen, settled, composedCandidate,
       composedBase, candidateVerified, acceptedBase, pendingCandidate,
       desiredCandidate, effectiveCandidate
     >>

Idle == UNCHANGED vars

Next ==
  OpenIntent
  \/ (\E a \in Assignments : OpenAssignment(a))
  \/ (\E a \in Assignments, t \in Attempts : StartAttempt(a, t))
  \/ (\E t \in Attempts, r \in Results : ReturnResult(t, r))
  \/ (\E t \in Attempts : FailAttempt(t))
  \/ (\E a \in Assignments, t \in Attempts : RetryAttempt(a, t))
  \/ (\E a \in Assignments : CancelAssignment(a))
  \/ (\E t \in Attempts, r \in Results : ReturnLateResult(t, r))
  \/ (\E t \in Attempts, d \in DispositionStates : DispositionAttempt(t, d))
  \/ (\E a \in Assignments : ReconcileAssignment(a))
  \/ RecordDissent
  \/ ResolveDissent
  \/ RecordFinding
  \/ ResolveFinding
  \/ (\E a \in Assignments : CloseAssignment(a))
  \/ SettleTrajectory
  \/ (\E c \in Candidates : ComposeCandidate(c))
  \/ VerifyCandidate
  \/ AcceptCandidate
  \/ MaterializeApplied
  \/ MaterializeFailed
  \/ PrepareObservation
  \/ AcknowledgeObservation
  \/ FinalizeOrRecoverObservation
  \/ DiscardUnacknowledged
  \/ DisableWrites
  \/ RebuildProjection
  \/ Idle

Spec == Init /\ [][Next]_vars

TypeOK ==
  /\ tape \in Seq(0..MaxEvents)
  /\ Len(tape) <= MaxEvents
  /\ \A i \in 1..Len(tape) : tape[i] = i
  /\ projectionSeq \in 0..Len(tape)
  /\ stage \in Stages
  /\ writesEnabled \in BOOLEAN
  /\ disabledAt \in 0..MaxEvents
  /\ rollbackEventCount \in 0..MaxEvents
  /\ intentRev \in 0..MaxEvents
  /\ assignmentState \in [Assignments -> AssignmentStates]
  /\ assignmentIntent \in [Assignments -> 0..MaxEvents]
  /\ assignmentBase \in [Assignments -> 0..MaxEvents]
  /\ attemptAssignment \in [Attempts -> (Assignments \cup {None})]
  /\ attemptBase \in [Attempts -> 0..MaxEvents]
  /\ attemptState \in [Attempts -> AttemptStates]
  /\ attemptResult \in [Attempts -> (Results \cup {None})]
  /\ attemptDisposition \in [Attempts -> DispositionStates]
  /\ rebasePending \subseteq Assignments
  /\ dissentOpen \in BOOLEAN
  /\ findingOpen \in BOOLEAN
  /\ settled \in BOOLEAN
  /\ composedCandidate \in Candidates \cup {None}
  /\ composedBase \in 0..MaxEvents
  /\ candidateVerified \in BOOLEAN
  /\ acceptedBase \in 0..MaxEvents
  /\ pendingCandidate \in Candidates \cup {None}
  /\ desiredCandidate \in Candidates \cup {None}
  /\ effectiveCandidate \in Candidates \cup {None}

ProjectionNeverLeadsTape == projectionSeq <= Len(tape)

AcknowledgedIsRecoverable ==
  stage = "acknowledged" => projectionSeq + 1 = Len(tape)

IdleProjectionEqualsTape ==
  stage = "idle" => projectionSeq = Len(tape)

AssignmentLineageRetained ==
  \A t \in Attempts :
    attemptState[t] /= "absent" =>
      /\ attemptAssignment[t] \in Assignments
      /\ attemptBase[t] <= Len(tape)
      /\ assignmentIntent[attemptAssignment[t]] > 0

SettlementComplete ==
  settled =>
    /\ rebasePending = {}
    /\ ~dissentOpen
    /\ ~findingOpen
    /\ pendingCandidate = None
    /\ \A a \in Assignments : assignmentState[a] \in {"absent", "closed"}
    /\ \A t \in Attempts :
         attemptState[t] = "absent" \/ attemptDisposition[t] /= "none"

LateResultNeedsDisposition ==
  \A t \in Attempts :
    attemptState[t] = "late" /\ settled => attemptDisposition[t] /= "none"

OnePendingCandidate ==
  pendingCandidate = None \/
    /\ pendingCandidate = desiredCandidate
    /\ pendingCandidate = composedCandidate
    /\ acceptedBase = composedBase
    /\ candidateVerified

NoBranchResultPromotion ==
  pendingCandidate = None \/ pendingCandidate \in Candidates

NoDesiredDivergenceWithoutPending ==
  pendingCandidate = None => desiredCandidate = effectiveCandidate

WritesDisabledAreQuiescent ==
  ~writesEnabled => Len(tape) = disabledAt

NoRollbackEvents == rollbackEventCount = 0

=============================================================================
