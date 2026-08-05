----------------------- MODULE texture_supervision -----------------------
EXTENDS Naturals, FiniteSets, Sequences, TLC

CONSTANTS Assignments, Attempts, Results, Candidates, Verifiers, EvidenceIds, CommandKeys, Digests, EventIds, MaxEvents, AllowedKinds, AllowedDispositions
Symmetry ==
    Permutations(Assignments)
    \union Permutations(Attempts)
    \union Permutations(Results)
    \union Permutations(Candidates)
    \union Permutations(Verifiers)
    \union Permutations(EvidenceIds)
    \union Permutations(Digests)


None == "none"
Roles == {"texture", "owner", "super"}
Stages == {"idle", "prepared", "acknowledged"}
CommandStates == {"absent", "reserved", "frozen", "prepared", "acknowledged", "done"}
AssignmentStates == {"absent", "open", "cancelled", "closed"}
AttemptStates == {"absent", "started", "cancelled", "returned"}
ResultStates == {"absent", "returned"}
Dispositions == {None, "preserved", "invalidated", "superseded", "compensation_required", "cancelled", "late", "incorporated", "rejected"}
RebaseDispositions == {None, "preserved", "invalidated", "superseded", "compensation_required"}
CandidateStates == {None, "composed", "verified", "stale", "accepted"}
ProposalStates == {None, "current", "consumed"}

\* Abstract semantic-plan vocabulary mapped to the closed supervision schema.
\* update_recorded and terminal_delivery_observed are deliberately absent;
\* rebase_opened and attempt_cancelled are reducer outputs, not authorable plans.
Authorable == {"open_trajectory", "intent_revised", "assignment_opened", "attempt_started", "attempt_retried", "attempt_result", "late_result", "assignment_cancelled", "attempt_dispositioned", "result_dispositioned", "rebase_dispositioned", "assignment_closed", "finding_opened", "finding_resolved", "dissent_opened", "dissent_resolved", "owner_attention_opened", "owner_attention_resolved", "evidence_completed", "artifact_head_completed", "compensation_resolved", "texture_owner_settlement_proposed", "super_settlement_proposed", "trajectory_settled", "candidate_composed", "candidate_verified", "candidate_accepted", "candidate_discarded", "composition_failed", "materialization_applied", "materialization_failed", "actor_message_acknowledged"}
Derived == {"rebase_opened", "attempt_cancelled"}
Recorded == Authorable \cup Derived
Forbidden == {"rollback_requested", "rollback_completed", "effect_accepted", "checkpoint_failed", "route_failed"}
Failures == {"composition_failed", "materialization_failed"}
SuperOnly == {"assignment_opened", "attempt_started", "attempt_retried", "assignment_cancelled", "attempt_dispositioned", "result_dispositioned", "rebase_dispositioned", "assignment_closed", "dissent_opened", "dissent_resolved", "compensation_resolved", "super_settlement_proposed", "candidate_composed", "candidate_discarded", "composition_failed", "materialization_applied", "materialization_failed"}

Plan(kind, actor, digest, assignment, attempt, prior, result, candidate, evidence, disposition, selected, affectedA, affectedT, base, mutations) ==
  [kind |-> kind, actor |-> actor, digest |-> digest, assignment |-> assignment,
   attempt |-> attempt, prior |-> prior, result |-> result, candidate |-> candidate,
   evidence |-> evidence, disposition |-> disposition, selected |-> selected,
   affectedA |-> affectedA, affectedT |-> affectedT, base |-> base,
   mutations |-> mutations]
Simple(kind, actor, digest) == Plan(kind, actor, digest, None, None, None, None, None, None, None, {}, {}, {}, None, {kind})
EmptyPlan == Plan(None, None, None, None, None, None, None, None, None, None, {}, {}, {}, None, {})
SimpleSuperKinds == {"finding_opened", "finding_resolved", "dissent_opened", "dissent_resolved", "owner_attention_opened", "owner_attention_resolved", "evidence_completed", "artifact_head_completed", "compensation_resolved", "super_settlement_proposed", "composition_failed"}
Plans ==
  UNION {
    {Simple("open_trajectory", "texture", d) : d \in Digests},
    {Simple("actor_message_acknowledged", "texture", d) : d \in Digests},
    {Plan("intent_revised", "texture", d, None, None, None, None, None, None, None, {}, aa, {}, None, {"intent_revised", "rebase_opened"}) : d \in Digests, aa \in SUBSET Assignments},
    {Plan("assignment_opened", "super", d, a, None, None, None, None, None, None, {}, {}, {}, None, {"assignment_opened"}) : d \in Digests, a \in Assignments},
    {Plan("attempt_started", "super", d, a, t, None, None, None, None, None, {}, {}, {}, None, {"attempt_started"}) : d \in Digests, a \in Assignments, t \in Attempts},
    {Plan("attempt_retried", "super", d, a, t, prior, None, None, None, None, {}, {}, {}, None, {"attempt_retried"}) : d \in Digests, a \in Assignments, t \in Attempts, prior \in Attempts},
    {Plan("attempt_result", "cosuper", d, None, t, None, r, None, None, None, {}, {}, {}, None, {"attempt_result"}) : d \in Digests, t \in Attempts, r \in Results},
    {Plan("late_result", "cosuper", d, None, t, None, r, None, None, None, {}, {}, {}, None, {"late_result"}) : d \in Digests, t \in Attempts, r \in Results},
    {Plan("assignment_cancelled", "super", d, a, None, None, None, None, None, None, {}, {}, at, None, {"assignment_cancelled", "attempt_cancelled"}) : d \in Digests, a \in Assignments, at \in SUBSET Attempts},
    {Plan("attempt_dispositioned", "super", d, None, t, None, None, None, None, disp, {}, {}, {}, None, {"attempt_dispositioned"}) : d \in Digests, t \in Attempts, disp \in Dispositions},
    {Plan("result_dispositioned", "super", d, None, None, None, r, None, None, disp, {}, {}, {}, None, {"result_dispositioned"}) : d \in Digests, r \in Results, disp \in Dispositions},
    {Plan("rebase_dispositioned", "super", d, a, None, None, None, None, None, disp, {}, {}, {}, None, {"rebase_dispositioned"}) : d \in Digests, a \in Assignments, disp \in RebaseDispositions},
    {Plan("assignment_closed", "super", d, a, None, None, None, None, None, None, {}, {}, {}, None, {"assignment_closed"}) : d \in Digests, a \in Assignments},
    {Plan("candidate_composed", "super", d, None, None, None, None, c, None, None, selected, {}, {}, base, {"candidate_composed"}) : d \in Digests, c \in Candidates, selected \in SUBSET Results, base \in Digests},
    {Plan("candidate_verified", v, d, None, None, None, None, c, e, None, {}, {}, {}, base, {"candidate_verified"}) : v \in Verifiers, e \in EvidenceIds, d \in Digests, c \in Candidates, base \in Digests},
    {Plan(kind, "super", d, None, None, None, None, c, None, None, {}, {}, {}, base, {kind}) : kind \in {"candidate_discarded", "materialization_applied", "materialization_failed"}, d \in Digests, c \in Candidates, base \in Digests},
    {Plan("candidate_accepted", "owner", d, None, None, None, None, c, None, None, {}, {}, {}, base, {"candidate_accepted"}) : d \in Digests, c \in Candidates, base \in Digests},
    {Simple(kind, "super", d) : kind \in SimpleSuperKinds, d \in Digests},
    {Simple("texture_owner_settlement_proposed", "owner", d) : d \in Digests},
    {Simple("trajectory_settled", "owner", d) : d \in Digests}
  }

VARIABLES tapeSeq, tapeKinds, eventDigests, projectionSeq, semanticSeq,
  stage, pendingPlan, acknowledgedPlan, activeKey, reservationDigest, eventIdentity, commandState, retryReceipts, conflictRefused,
  writesEnabled, disabledAt, projectionAvailable, rebuiltHead, rebuiltSemantic,
  trajectoryOpen, intentRev, assignmentState, assignmentIntent, attemptAssignment, attemptState, attemptResult, attemptDisposition, resultAttempt, resultState, resultDisposition, resultDigest, lateResults, rebaseRequired, rebaseDisposition, findingOpen, dissentOpen, ownerAttentionOpen, evidenceComplete, artifactHeadComplete, compensationOpen, compensationHistory,
  textureOwnerProposal, textureOwnerHead, superProposal, superProposalHead, settled,
  composedCandidate, composedBase, composedAtHead, composedResults, composedResultDigests, candidateState, verifiedBy, verificationEvidence, verifiedHead, acceptedHead, pendingDesired, effectiveCandidate, acceptedCandidates, terminalEvidence, secondPendingRefusals

vars == <<tapeSeq,tapeKinds,eventDigests,projectionSeq,semanticSeq,stage,pendingPlan,acknowledgedPlan,activeKey,reservationDigest,eventIdentity,commandState,retryReceipts,conflictRefused,writesEnabled,disabledAt,projectionAvailable,rebuiltHead,rebuiltSemantic,trajectoryOpen,intentRev,assignmentState,assignmentIntent,attemptAssignment,attemptState,attemptResult,attemptDisposition,resultAttempt,resultState,resultDisposition,resultDigest,lateResults,rebaseRequired,rebaseDisposition,findingOpen,dissentOpen,ownerAttentionOpen,evidenceComplete,artifactHeadComplete,compensationOpen,compensationHistory,textureOwnerProposal,textureOwnerHead,superProposal,superProposalHead,settled,composedCandidate,composedBase,composedAtHead,composedResults,composedResultDigests,candidateState,verifiedBy,verificationEvidence,verifiedHead,acceptedHead,pendingDesired,effectiveCandidate,acceptedCandidates,terminalEvidence,secondPendingRefusals>>
SemanticVars == <<trajectoryOpen,intentRev,assignmentState,assignmentIntent,attemptAssignment,attemptState,attemptResult,attemptDisposition,resultAttempt,resultState,resultDisposition,resultDigest,lateResults,rebaseRequired,rebaseDisposition,findingOpen,dissentOpen,ownerAttentionOpen,evidenceComplete,artifactHeadComplete,compensationOpen,compensationHistory,textureOwnerProposal,textureOwnerHead,superProposal,superProposalHead,settled,composedCandidate,composedBase,composedAtHead,composedResults,composedResultDigests,candidateState,verifiedBy,verificationEvidence,verifiedHead,acceptedHead,pendingDesired,effectiveCandidate,acceptedCandidates>>

CurrentHead == IF Len(eventDigests) = 0 THEN None ELSE eventDigests[Len(eventDigests)]
ProjectedHead == IF projectionSeq = 0 THEN None ELSE eventDigests[projectionSeq]
AffectedAssignments == {a \in Assignments : assignmentState[a] \in {"open", "cancelled"}}
StartedFor(a) == {t \in Attempts : attemptAssignment[t] = a /\ attemptState[t] = "started"}
Returned == {r \in Results : resultState[r] = "returned"}
ResultDigests(rs) == {resultDigest[r] : r \in rs}
RECURSIVE ReplayFold(_)
ReplayFold(n) ==
  IF n = 0 THEN <<>>
  ELSE Append(ReplayFold(n - 1), <<eventDigests[n], tapeKinds[n]>>)

Init ==
 /\ tapeSeq=0 /\ tapeKinds = <<>> /\ eventDigests = <<>> /\ projectionSeq=0 /\ semanticSeq=0 /\ stage="idle" /\ pendingPlan=EmptyPlan /\ acknowledgedPlan=EmptyPlan /\ activeKey=None
 /\ reservationDigest=[k \in CommandKeys |-> None] /\ eventIdentity=[k \in CommandKeys |-> None] /\ commandState=[k \in CommandKeys |-> "absent"] /\ retryReceipts={} /\ conflictRefused={}
 /\ writesEnabled=TRUE /\ disabledAt=0 /\ projectionAvailable=TRUE /\ rebuiltHead=None
 /\ trajectoryOpen=FALSE /\ intentRev=0 /\ assignmentState=[a \in Assignments |-> "absent"] /\ assignmentIntent=[a \in Assignments |-> 0] /\ attemptAssignment=[t \in Attempts |-> None] /\ attemptState=[t \in Attempts |-> "absent"] /\ attemptResult=[t \in Attempts |-> None] /\ attemptDisposition=[t \in Attempts |-> None] /\ resultAttempt=[r \in Results |-> None] /\ resultState=[r \in Results |-> "absent"] /\ resultDisposition=[r \in Results |-> None] /\ resultDigest=[r \in Results |-> None] /\ lateResults={} /\ rebaseRequired={} /\ rebaseDisposition=[a \in Assignments |-> None] /\ findingOpen=FALSE /\ dissentOpen=FALSE /\ ownerAttentionOpen=FALSE /\ evidenceComplete=FALSE /\ artifactHeadComplete=FALSE /\ compensationOpen=FALSE /\ compensationHistory={}
 /\ textureOwnerProposal=None /\ textureOwnerHead=None /\ superProposal=None /\ superProposalHead=None /\ settled=FALSE
 /\ composedCandidate=None /\ composedBase=[c \in Candidates |-> None] /\ composedAtHead=[c \in Candidates |-> None] /\ composedResults=[c \in Candidates |-> {}] /\ composedResultDigests=[c \in Candidates |-> {}] /\ candidateState=[c \in Candidates |-> None] /\ verifiedBy=[c \in Candidates |-> None] /\ verificationEvidence=[c \in Candidates |-> None] /\ verifiedHead=[c \in Candidates |-> None] /\ acceptedHead=[c \in Candidates |-> None] /\ pendingDesired=None /\ effectiveCandidate=None /\ acceptedCandidates={} /\ terminalEvidence={} /\ secondPendingRefusals=0
 /\ rebuiltSemantic=ReplayFold(0)

PlanAuthorized(p) == IF p.kind \in SuperOnly THEN p.actor="super" ELSE IF p.kind="open_trajectory" THEN p.actor \in {"texture","owner"} ELSE IF p.kind \in {"attempt_result","late_result"} THEN p.actor="cosuper" ELSE IF p.kind="candidate_verified" THEN p.actor \in Verifiers ELSE IF p.kind \in {"texture_owner_settlement_proposed","trajectory_settled","candidate_accepted"} THEN p.actor="owner" ELSE TRUE
PlanShape(p) ==
 /\ p \in Plans /\ p.kind \in p.mutations /\ PlanAuthorized(p)
 /\ IF p.kind="intent_revised" THEN p.mutations={"intent_revised","rebase_opened"} /\ p.affectedA=AffectedAssignments ELSE TRUE
 /\ IF p.kind="assignment_cancelled" THEN p.mutations={"assignment_cancelled","attempt_cancelled"} /\ p.affectedT=StartedFor(p.assignment) ELSE TRUE
 /\ IF p.kind \notin {"intent_revised","assignment_cancelled"} THEN p.mutations={p.kind} /\ p.affectedA={} /\ p.affectedT={} ELSE TRUE
 /\ IF p.kind \in {"candidate_composed","candidate_verified","candidate_accepted"} THEN p.base=CurrentHead ELSE TRUE
 /\ IF p.kind="candidate_verified" THEN p.evidence \in EvidenceIds ELSE p.evidence=None

NextCommandKey == CHOOSE k \in CommandKeys : commandState[k]="absent"
Reserve(k,d) == /\ stage="idle" /\ activeKey=None /\ commandState[k]="absent" /\ k=NextCommandKey /\ tapeSeq<MaxEvents /\ (CurrentHead=None \/ d#CurrentHead) /\ reservationDigest'=[reservationDigest EXCEPT ![k]=d] /\ commandState'=[commandState EXCEPT ![k]="reserved"] /\ activeKey'=k /\ UNCHANGED <<tapeSeq,tapeKinds,eventDigests,projectionSeq,semanticSeq,stage,pendingPlan,acknowledgedPlan,eventIdentity,retryReceipts,conflictRefused,writesEnabled,disabledAt,projectionAvailable,rebuiltHead,rebuiltSemantic,SemanticVars,terminalEvidence,secondPendingRefusals>>
Freeze(e) == /\ stage="idle" /\ activeKey \in CommandKeys /\ commandState[activeKey]="reserved" /\ e=activeKey /\ eventIdentity'=[eventIdentity EXCEPT ![activeKey]=e] /\ commandState'=[commandState EXCEPT ![activeKey]="frozen"] /\ UNCHANGED <<tapeSeq,tapeKinds,eventDigests,projectionSeq,semanticSeq,stage,pendingPlan,acknowledgedPlan,activeKey,reservationDigest,retryReceipts,conflictRefused,writesEnabled,disabledAt,projectionAvailable,rebuiltHead,rebuiltSemantic,SemanticVars,terminalEvidence,secondPendingRefusals>>
Retry(k,d) == /\ reservationDigest[k]=d /\ commandState[k]#"absent" /\ retryReceipts'=retryReceipts \cup {k} /\ UNCHANGED <<tapeSeq,tapeKinds,eventDigests,projectionSeq,semanticSeq,stage,pendingPlan,acknowledgedPlan,activeKey,reservationDigest,eventIdentity,commandState,conflictRefused,writesEnabled,disabledAt,projectionAvailable,rebuiltHead,rebuiltSemantic,SemanticVars,terminalEvidence,secondPendingRefusals>>
Conflict(k,d) == /\ reservationDigest[k]#None /\ reservationDigest[k]#d /\ conflictRefused'=conflictRefused \cup {k} /\ UNCHANGED <<tapeSeq,tapeKinds,eventDigests,projectionSeq,semanticSeq,stage,pendingPlan,acknowledgedPlan,activeKey,reservationDigest,eventIdentity,commandState,retryReceipts,writesEnabled,disabledAt,projectionAvailable,rebuiltHead,rebuiltSemantic,SemanticVars,terminalEvidence,secondPendingRefusals>>

CanPlan == /\ stage="idle" /\ activeKey \in CommandKeys /\ commandState[activeKey]="frozen" /\ writesEnabled /\ projectionAvailable /\ ~settled
Prepare(p) == /\ CanPlan /\ PlanShape(p) /\ p.digest=reservationDigest[activeKey] /\ pendingPlan'=p /\ stage'="prepared" /\ commandState'=[commandState EXCEPT ![activeKey]="prepared"] /\ UNCHANGED <<tapeSeq,tapeKinds,eventDigests,projectionSeq,semanticSeq,acknowledgedPlan,activeKey,reservationDigest,eventIdentity,retryReceipts,conflictRefused,writesEnabled,disabledAt,projectionAvailable,rebuiltHead,rebuiltSemantic,SemanticVars,terminalEvidence,secondPendingRefusals>>
Abort == /\ stage="prepared" /\ pendingPlan'=EmptyPlan /\ stage'="idle" /\ commandState'=[commandState EXCEPT ![activeKey]="frozen"] /\ UNCHANGED <<tapeSeq,tapeKinds,eventDigests,projectionSeq,semanticSeq,acknowledgedPlan,activeKey,reservationDigest,eventIdentity,retryReceipts,conflictRefused,writesEnabled,disabledAt,projectionAvailable,rebuiltHead,rebuiltSemantic,SemanticVars,terminalEvidence,secondPendingRefusals>>
CAS == /\ stage="prepared" /\ tapeSeq<MaxEvents /\ tapeSeq'=tapeSeq+1 /\ tapeKinds'=Append(tapeKinds,pendingPlan.mutations) /\ eventDigests'=Append(eventDigests,reservationDigest[activeKey]) /\ stage'="acknowledged" /\ acknowledgedPlan'=pendingPlan /\ commandState'=[commandState EXCEPT ![activeKey]="acknowledged"] /\ UNCHANGED <<projectionSeq,semanticSeq,pendingPlan,activeKey,reservationDigest,eventIdentity,retryReceipts,conflictRefused,writesEnabled,disabledAt,projectionAvailable,rebuiltHead,rebuiltSemantic,SemanticVars,terminalEvidence,secondPendingRefusals>>

CandidateInvalidating(p) == p.kind \in {"intent_revised","assignment_opened","attempt_started","attempt_retried","attempt_result","late_result","assignment_cancelled","attempt_dispositioned","result_dispositioned","rebase_dispositioned","dissent_opened","dissent_resolved"}
SettlementInvalidating(p) == p.kind \notin {"texture_owner_settlement_proposed","super_settlement_proposed","trajectory_settled"}
Finish ==
 /\ stage="acknowledged" /\ pendingPlan=acknowledgedPlan /\ projectionSeq+1=tapeSeq /\ projectionSeq'=tapeSeq /\ semanticSeq'=tapeSeq /\ stage'="idle" /\ pendingPlan'=EmptyPlan /\ acknowledgedPlan'=EmptyPlan /\ commandState'=[commandState EXCEPT ![activeKey]="done"] /\ activeKey'=None
 /\ trajectoryOpen'=IF acknowledgedPlan.kind="open_trajectory" THEN TRUE ELSE IF acknowledgedPlan.kind="trajectory_settled" THEN FALSE ELSE trajectoryOpen
 /\ intentRev'=IF acknowledgedPlan.kind="intent_revised" THEN intentRev+1 ELSE intentRev
 /\ assignmentState'=[a \in Assignments |-> IF acknowledgedPlan.kind="assignment_opened" /\ a=acknowledgedPlan.assignment THEN "open" ELSE IF acknowledgedPlan.kind="assignment_cancelled" /\ a=acknowledgedPlan.assignment THEN "cancelled" ELSE IF acknowledgedPlan.kind="assignment_closed" /\ a=acknowledgedPlan.assignment THEN "closed" ELSE assignmentState[a]]
 /\ assignmentIntent'=IF acknowledgedPlan.kind="assignment_opened" THEN [assignmentIntent EXCEPT ![acknowledgedPlan.assignment]=intentRev] ELSE assignmentIntent
 /\ attemptAssignment'=IF acknowledgedPlan.kind \in {"attempt_started","attempt_retried"} THEN [attemptAssignment EXCEPT ![acknowledgedPlan.attempt]=acknowledgedPlan.assignment] ELSE attemptAssignment
 /\ attemptState'=[t \in Attempts |-> IF acknowledgedPlan.kind \in {"attempt_started","attempt_retried"} /\ t=acknowledgedPlan.attempt THEN "started" ELSE IF acknowledgedPlan.kind \in {"attempt_result","late_result"} /\ t=acknowledgedPlan.attempt THEN "returned" ELSE IF acknowledgedPlan.kind="assignment_cancelled" /\ t \in acknowledgedPlan.affectedT THEN "cancelled" ELSE attemptState[t]]
 /\ attemptResult'=IF acknowledgedPlan.kind \in {"attempt_result","late_result"} THEN [attemptResult EXCEPT ![acknowledgedPlan.attempt]=acknowledgedPlan.result] ELSE attemptResult
 /\ attemptDisposition'=IF acknowledgedPlan.kind="attempt_dispositioned" THEN [attemptDisposition EXCEPT ![acknowledgedPlan.attempt]=acknowledgedPlan.disposition] ELSE attemptDisposition
 /\ resultAttempt'=IF acknowledgedPlan.kind \in {"attempt_result","late_result"} THEN [resultAttempt EXCEPT ![acknowledgedPlan.result]=acknowledgedPlan.attempt] ELSE resultAttempt
 /\ resultState'=IF acknowledgedPlan.kind \in {"attempt_result","late_result"} THEN [resultState EXCEPT ![acknowledgedPlan.result]="returned"] ELSE resultState
 /\ resultDisposition'=IF acknowledgedPlan.kind="result_dispositioned" THEN [resultDisposition EXCEPT ![acknowledgedPlan.result]=acknowledgedPlan.disposition] ELSE resultDisposition
 /\ resultDigest'=IF acknowledgedPlan.kind \in {"attempt_result","late_result"} THEN [resultDigest EXCEPT ![acknowledgedPlan.result]=acknowledgedPlan.digest] ELSE resultDigest
 /\ lateResults'=IF acknowledgedPlan.kind="late_result" THEN lateResults \cup {acknowledgedPlan.result} ELSE lateResults
 /\ rebaseRequired'=IF acknowledgedPlan.kind="intent_revised" THEN acknowledgedPlan.affectedA ELSE IF acknowledgedPlan.kind="rebase_dispositioned" THEN rebaseRequired \ {acknowledgedPlan.assignment} ELSE rebaseRequired
 /\ rebaseDisposition'=IF acknowledgedPlan.kind="intent_revised" THEN [a \in Assignments |-> IF a \in acknowledgedPlan.affectedA THEN None ELSE rebaseDisposition[a]] ELSE IF acknowledgedPlan.kind="rebase_dispositioned" THEN [rebaseDisposition EXCEPT ![acknowledgedPlan.assignment]=acknowledgedPlan.disposition] ELSE rebaseDisposition
 /\ findingOpen'=IF acknowledgedPlan.kind="finding_opened" THEN TRUE ELSE IF acknowledgedPlan.kind="finding_resolved" THEN FALSE ELSE findingOpen
 /\ dissentOpen'=IF acknowledgedPlan.kind="dissent_opened" THEN TRUE ELSE IF acknowledgedPlan.kind="dissent_resolved" THEN FALSE ELSE dissentOpen
 /\ ownerAttentionOpen'=IF acknowledgedPlan.kind="owner_attention_opened" THEN TRUE ELSE IF acknowledgedPlan.kind="owner_attention_resolved" THEN FALSE ELSE ownerAttentionOpen
 /\ evidenceComplete'=IF acknowledgedPlan.kind="evidence_completed" THEN TRUE ELSE evidenceComplete
 /\ artifactHeadComplete'=IF acknowledgedPlan.kind="artifact_head_completed" THEN TRUE ELSE artifactHeadComplete
 /\ compensationOpen'=IF acknowledgedPlan.kind \in Failures THEN TRUE ELSE IF acknowledgedPlan.kind="compensation_resolved" THEN FALSE ELSE compensationOpen
 /\ compensationHistory'=IF acknowledgedPlan.kind \in Failures THEN compensationHistory \cup {acknowledgedPlan.kind} ELSE compensationHistory
 /\ textureOwnerProposal'=IF acknowledgedPlan.kind="texture_owner_settlement_proposed" THEN "current" ELSE IF acknowledgedPlan.kind="trajectory_settled" THEN "consumed" ELSE IF SettlementInvalidating(acknowledgedPlan) THEN None ELSE textureOwnerProposal
 /\ textureOwnerHead'=IF acknowledgedPlan.kind \in {"texture_owner_settlement_proposed","super_settlement_proposed"} THEN CurrentHead ELSE textureOwnerHead
 /\ superProposal'=IF acknowledgedPlan.kind="super_settlement_proposed" THEN "current" ELSE IF acknowledgedPlan.kind="trajectory_settled" THEN "consumed" ELSE IF acknowledgedPlan.kind="texture_owner_settlement_proposed" \/ SettlementInvalidating(acknowledgedPlan) THEN None ELSE superProposal
 /\ superProposalHead'=IF acknowledgedPlan.kind="super_settlement_proposed" THEN CurrentHead ELSE superProposalHead
 /\ settled'=(settled \/ acknowledgedPlan.kind="trajectory_settled")
 /\ composedCandidate'=IF acknowledgedPlan.kind="candidate_composed" THEN acknowledgedPlan.candidate ELSE IF acknowledgedPlan.kind \in {"candidate_discarded","materialization_failed"} /\ composedCandidate=acknowledgedPlan.candidate THEN None ELSE composedCandidate
 /\ composedBase'=IF acknowledgedPlan.kind="candidate_composed" THEN [composedBase EXCEPT ![acknowledgedPlan.candidate]=acknowledgedPlan.base] ELSE composedBase
 /\ composedAtHead'=IF acknowledgedPlan.kind="candidate_composed" THEN [composedAtHead EXCEPT ![acknowledgedPlan.candidate]=CurrentHead] ELSE composedAtHead
 /\ composedResults'=IF acknowledgedPlan.kind="candidate_composed" THEN [composedResults EXCEPT ![acknowledgedPlan.candidate]=acknowledgedPlan.selected] ELSE composedResults
 /\ composedResultDigests'=IF acknowledgedPlan.kind="candidate_composed" THEN [composedResultDigests EXCEPT ![acknowledgedPlan.candidate]=ResultDigests(acknowledgedPlan.selected)] ELSE composedResultDigests
 /\ candidateState'=[c \in Candidates |-> IF acknowledgedPlan.kind="candidate_composed" /\ c=acknowledgedPlan.candidate THEN "composed" ELSE IF acknowledgedPlan.kind="candidate_verified" /\ c=acknowledgedPlan.candidate THEN "verified" ELSE IF acknowledgedPlan.kind="candidate_accepted" /\ c=acknowledgedPlan.candidate THEN "accepted" ELSE IF acknowledgedPlan.kind \in {"candidate_discarded","materialization_failed"} /\ c=acknowledgedPlan.candidate THEN None ELSE IF c=composedCandidate /\ candidateState[c] \in {"composed","verified"} /\ CandidateInvalidating(acknowledgedPlan) THEN "stale" ELSE candidateState[c]]
 /\ verifiedBy'=IF acknowledgedPlan.kind="candidate_verified" THEN [verifiedBy EXCEPT ![acknowledgedPlan.candidate]=acknowledgedPlan.actor] ELSE verifiedBy
 /\ verificationEvidence'=IF acknowledgedPlan.kind="candidate_verified" THEN [verificationEvidence EXCEPT ![acknowledgedPlan.candidate]=acknowledgedPlan.evidence] ELSE verificationEvidence
 /\ verifiedHead'=IF acknowledgedPlan.kind="candidate_verified" THEN [verifiedHead EXCEPT ![acknowledgedPlan.candidate]=CurrentHead] ELSE verifiedHead
 /\ acceptedHead'=IF acknowledgedPlan.kind="candidate_accepted" THEN [acceptedHead EXCEPT ![acknowledgedPlan.candidate]=CurrentHead] ELSE acceptedHead
 /\ pendingDesired'=IF acknowledgedPlan.kind="candidate_accepted" THEN acknowledgedPlan.candidate ELSE IF acknowledgedPlan.kind \in {"materialization_applied","materialization_failed"} THEN None ELSE pendingDesired
 /\ effectiveCandidate'=IF acknowledgedPlan.kind="materialization_applied" THEN acknowledgedPlan.candidate ELSE effectiveCandidate
 /\ acceptedCandidates'=IF acknowledgedPlan.kind="candidate_accepted" THEN acceptedCandidates \cup {acknowledgedPlan.candidate} ELSE acceptedCandidates
 /\ UNCHANGED <<tapeSeq,tapeKinds,eventDigests,reservationDigest,eventIdentity,retryReceipts,conflictRefused,writesEnabled,disabledAt,rebuiltHead,rebuiltSemantic,terminalEvidence,secondPendingRefusals>>
Finalize == /\ projectionAvailable /\ Finish /\ UNCHANGED <<projectionAvailable>>
CrashAfterCAS == /\ stage="acknowledged" /\ projectionAvailable /\ projectionAvailable'=FALSE /\ UNCHANGED <<tapeSeq,tapeKinds,eventDigests,projectionSeq,semanticSeq,stage,pendingPlan,acknowledgedPlan,activeKey,reservationDigest,eventIdentity,commandState,retryReceipts,conflictRefused,writesEnabled,disabledAt,rebuiltHead,rebuiltSemantic,SemanticVars,terminalEvidence,secondPendingRefusals>>
RecoverFinalize == /\ ~projectionAvailable /\ Finish /\ projectionAvailable'=TRUE
AssignmentsClosed == \A a \in Assignments : assignmentState[a] \in {"absent","closed"}
AttemptsDone == \A t \in Attempts : attemptState[t]="absent" \/ attemptDisposition[t]#None
ResultsDone == \A r \in Results : resultState[r]="absent" \/ resultDisposition[r]#None
RebasesDone == rebaseRequired={}
SettlementReady == trajectoryOpen /\ intentRev>0 /\ AssignmentsClosed /\ AttemptsDone /\ ResultsDone /\ RebasesDone /\ ~findingOpen /\ ~dissentOpen /\ ~ownerAttentionOpen /\ ~compensationOpen /\ evidenceComplete /\ artifactHeadComplete /\ pendingDesired=None
Enabled(p) ==
 /\ p.kind \in AllowedKinds
 /\ (p.kind \notin {"attempt_dispositioned", "result_dispositioned", "rebase_dispositioned"} \/ p.disposition \in AllowedDispositions)
 /\ CanPlan
 /\ CASE p.kind="open_trajectory" -> ~trajectoryOpen
       [] p.kind="intent_revised" -> trajectoryOpen
       [] p.kind="assignment_opened" -> trajectoryOpen /\ intentRev>0 /\ assignmentState[p.assignment]="absent"
       [] p.kind \in {"attempt_started","attempt_retried"} -> assignmentState[p.assignment]="open" /\ attemptState[p.attempt]="absent" /\ (IF p.kind="attempt_started" THEN TRUE ELSE attemptState[p.prior]="returned" /\ attemptDisposition[p.prior] \in {"superseded","cancelled","late"})
       [] p.kind="attempt_result" -> attemptState[p.attempt]="started" /\ resultState[p.result]="absent"
       [] p.kind="late_result" -> attemptState[p.attempt]="cancelled" /\ resultState[p.result]="absent"
       [] p.kind="assignment_cancelled" -> assignmentState[p.assignment]="open"
       [] p.kind="attempt_dispositioned" -> attemptState[p.attempt] \in {"cancelled","returned"} /\ attemptDisposition[p.attempt]=None /\ p.disposition#None
       [] p.kind="result_dispositioned" -> resultState[p.result]="returned" /\ resultDisposition[p.result]=None /\ p.disposition#None
       [] p.kind="rebase_dispositioned" -> p.assignment \in rebaseRequired /\ p.disposition \in RebaseDispositions \ {None}
       [] p.kind="assignment_closed" -> assignmentState[p.assignment] \in {"open","cancelled"} /\ (\A t \in Attempts : attemptAssignment[t]#p.assignment \/ (attemptState[t] \in {"cancelled","returned"} /\ attemptDisposition[t]#None)) /\ (\A r \in Results : IF resultAttempt[r]=None THEN TRUE ELSE attemptAssignment[resultAttempt[r]]#p.assignment \/ resultDisposition[r]#None)
       [] p.kind="candidate_composed" -> composedCandidate=None /\ pendingDesired=None /\ p.selected#{} /\ p.selected \subseteq Returned /\ (\A r \in p.selected : resultDisposition[r]="incorporated") /\ (\A a \in Assignments : assignmentState[a] \in {"absent","closed"})
       [] p.kind="candidate_verified" -> composedCandidate=p.candidate /\ candidateState[p.candidate]="composed" /\ composedAtHead[p.candidate]=CurrentHead /\ p.base=composedAtHead[p.candidate] /\ p.actor \in Verifiers /\ p.evidence \in EvidenceIds
       [] p.kind="candidate_accepted" -> composedCandidate=p.candidate /\ candidateState[p.candidate]="verified" /\ verifiedHead[p.candidate]=CurrentHead /\ pendingDesired=None /\ p.candidate \notin acceptedCandidates
       [] p.kind="materialization_applied" -> pendingDesired=p.candidate /\ candidateState[p.candidate]="accepted"
       [] p.kind="materialization_failed" -> pendingDesired=p.candidate /\ ~compensationOpen
       [] p.kind="candidate_discarded" -> composedCandidate=p.candidate /\ candidateState[p.candidate]="stale"
       [] p.kind="composition_failed" -> ~compensationOpen
       [] p.kind="compensation_resolved" -> compensationOpen
       [] p.kind="texture_owner_settlement_proposed" -> SettlementReady
       [] p.kind="super_settlement_proposed" -> SettlementReady /\ textureOwnerProposal="current" /\ textureOwnerHead=CurrentHead
       [] p.kind="trajectory_settled" -> SettlementReady /\ textureOwnerProposal="current" /\ superProposal="current" /\ textureOwnerHead=CurrentHead /\ superProposalHead=CurrentHead
       [] OTHER -> TRUE

ObserveTerminal(r) == /\ settled /\ r \in Results /\ terminalEvidence'=terminalEvidence \cup {r} /\ UNCHANGED <<tapeSeq,tapeKinds,eventDigests,projectionSeq,semanticSeq,stage,pendingPlan,acknowledgedPlan,activeKey,reservationDigest,eventIdentity,commandState,retryReceipts,conflictRefused,writesEnabled,disabledAt,projectionAvailable,rebuiltHead,rebuiltSemantic,SemanticVars,secondPendingRefusals>>
RefuseSecond(c) == /\ (pendingDesired#None \/ c \in acceptedCandidates) /\ secondPendingRefusals=0 /\ secondPendingRefusals'=1 /\ UNCHANGED <<tapeSeq,tapeKinds,eventDigests,projectionSeq,semanticSeq,stage,pendingPlan,acknowledgedPlan,activeKey,reservationDigest,eventIdentity,commandState,retryReceipts,conflictRefused,writesEnabled,disabledAt,projectionAvailable,rebuiltHead,rebuiltSemantic,SemanticVars,terminalEvidence>>
DisableWrites == /\ stage="idle" /\ writesEnabled /\ writesEnabled'=FALSE /\ disabledAt'=tapeSeq /\ UNCHANGED <<tapeSeq,tapeKinds,eventDigests,projectionSeq,semanticSeq,stage,pendingPlan,acknowledgedPlan,activeKey,reservationDigest,eventIdentity,commandState,retryReceipts,conflictRefused,projectionAvailable,rebuiltHead,rebuiltSemantic,SemanticVars,terminalEvidence,secondPendingRefusals>>
DropProjection == /\ stage="idle" /\ ~writesEnabled /\ projectionAvailable /\ projectionAvailable'=FALSE /\ UNCHANGED <<tapeSeq,tapeKinds,eventDigests,projectionSeq,semanticSeq,stage,pendingPlan,acknowledgedPlan,activeKey,reservationDigest,eventIdentity,commandState,retryReceipts,conflictRefused,writesEnabled,disabledAt,rebuiltHead,rebuiltSemantic,SemanticVars,terminalEvidence,secondPendingRefusals>>
RebuildProjection == /\ stage="idle" /\ ~writesEnabled /\ ~projectionAvailable /\ semanticSeq=tapeSeq /\ projectionAvailable'=TRUE /\ rebuiltHead'=CurrentHead /\ rebuiltSemantic'=ReplayFold(tapeSeq) /\ UNCHANGED <<tapeSeq,tapeKinds,eventDigests,projectionSeq,semanticSeq,stage,pendingPlan,acknowledgedPlan,activeKey,reservationDigest,eventIdentity,commandState,retryReceipts,conflictRefused,writesEnabled,disabledAt,SemanticVars,terminalEvidence,secondPendingRefusals>>

Next == \/ \E k \in CommandKeys,d \in Digests : Reserve(k,d) \/ Retry(k,d) \/ Conflict(k,d)
        \/ \E e \in EventIds : Freeze(e) \/ Abort \/ CAS \/ Finalize \/ CrashAfterCAS \/ RecoverFinalize
        \/ \E p \in Plans : Enabled(p) /\ Prepare(p)
        \/ \E r \in Results : ObserveTerminal(r)
        \/ \E c \in Candidates : RefuseSecond(c)
        \/ DisableWrites \/ DropProjection \/ RebuildProjection
Spec == Init /\ [][Next]_vars
CoverageNext ==
  \/ \E k \in CommandKeys, d \in Digests : Reserve(k,d)
  \/ \E e \in EventIds : Freeze(e)
  \/ \E p \in Plans : Enabled(p) /\ Prepare(p)
  \/ CAS
  \/ Finalize
CoverageSpec == Init /\ [][CoverageNext]_vars

TypeOK ==
  /\ tapeSeq \in 0..MaxEvents
  /\ AllowedKinds \subseteq Authorable
  /\ AllowedDispositions \subseteq Dispositions
  /\ tapeKinds \in Seq(SUBSET Recorded)
  /\ eventDigests \in Seq(Digests)
  /\ Len(tapeKinds)=tapeSeq
  /\ Len(eventDigests)=tapeSeq
  /\ projectionSeq \in 0..MaxEvents
  /\ semanticSeq \in 0..MaxEvents
  /\ stage \in Stages
  /\ (pendingPlan=EmptyPlan \/ pendingPlan \in Plans)
  /\ (acknowledgedPlan=EmptyPlan \/ acknowledgedPlan \in Plans)
  /\ activeKey \in CommandKeys \cup {None}
  /\ reservationDigest \in [CommandKeys -> Digests \cup {None}]
  /\ eventIdentity \in [CommandKeys -> EventIds \cup {None}]
  /\ commandState \in [CommandKeys -> CommandStates]
  /\ retryReceipts \subseteq CommandKeys
  /\ conflictRefused \subseteq CommandKeys
  /\ writesEnabled \in BOOLEAN
  /\ disabledAt \in 0..MaxEvents
  /\ projectionAvailable \in BOOLEAN
  /\ rebuiltHead \in Digests \cup {None}
  /\ trajectoryOpen \in BOOLEAN
  /\ intentRev \in Nat
  /\ assignmentState \in [Assignments -> AssignmentStates]
  /\ assignmentIntent \in [Assignments -> Nat]
  /\ attemptAssignment \in [Attempts -> Assignments \cup {None}]
  /\ attemptState \in [Attempts -> AttemptStates]
  /\ attemptResult \in [Attempts -> Results \cup {None}]
  /\ attemptDisposition \in [Attempts -> Dispositions]
  /\ resultAttempt \in [Results -> Attempts \cup {None}]
  /\ resultState \in [Results -> ResultStates]
  /\ resultDisposition \in [Results -> Dispositions]
  /\ resultDigest \in [Results -> Digests \cup {None}]
  /\ lateResults \subseteq Results
  /\ rebaseRequired \subseteq Assignments
  /\ rebaseDisposition \in [Assignments -> RebaseDispositions]
  /\ findingOpen \in BOOLEAN
  /\ dissentOpen \in BOOLEAN
  /\ ownerAttentionOpen \in BOOLEAN
  /\ evidenceComplete \in BOOLEAN
  /\ artifactHeadComplete \in BOOLEAN
  /\ compensationOpen \in BOOLEAN
  /\ compensationHistory \subseteq Failures
  /\ textureOwnerProposal \in ProposalStates
  /\ textureOwnerHead \in Digests \cup {None}
  /\ superProposal \in ProposalStates
  /\ superProposalHead \in Digests \cup {None}
  /\ settled \in BOOLEAN
  /\ composedCandidate \in Candidates \cup {None}
  /\ composedBase \in [Candidates -> Digests \cup {None}]
  /\ composedAtHead \in [Candidates -> Digests \cup {None}]
  /\ composedResults \in [Candidates -> SUBSET Results]
  /\ composedResultDigests \in [Candidates -> SUBSET Digests]
  /\ candidateState \in [Candidates -> CandidateStates]
  /\ verifiedBy \in [Candidates -> Verifiers \cup {None}]
  /\ verificationEvidence \in [Candidates -> EvidenceIds \cup {None}]
  /\ verifiedHead \in [Candidates -> Digests \cup {None}]
  /\ acceptedHead \in [Candidates -> Digests \cup {None}]
  /\ pendingDesired \in Candidates \cup {None}
  /\ effectiveCandidate \in Candidates \cup {None}
  /\ acceptedCandidates \subseteq Candidates
  /\ terminalEvidence \subseteq Results
  /\ secondPendingRefusals \in 0..1
ProjectionNeverLeadsTape == projectionSeq <= tapeSeq
NoPartialSemanticVisibility == /\ semanticSeq=projectionSeq
                             /\ (stage="prepared" => tapeSeq=projectionSeq)
                             /\ (stage="acknowledged" => tapeSeq=projectionSeq+1)
AcknowledgedPlanIsRecoverable == stage="acknowledged" => pendingPlan=acknowledgedPlan /\ tapeKinds[tapeSeq]=acknowledgedPlan.mutations
ReservationPrecedesEntropy == \A k \in CommandKeys : eventIdentity[k]#None => reservationDigest[k]#None
NoRollbackEvents == \A i \in DOMAIN tapeKinds : tapeKinds[i] \cap Forbidden={}
SettlementComplete == settled => ~trajectoryOpen /\ AssignmentsClosed /\ AttemptsDone /\ ResultsDone /\ RebasesDone /\ ~findingOpen /\ ~dissentOpen /\ ~ownerAttentionOpen /\ ~compensationOpen /\ evidenceComplete /\ artifactHeadComplete /\ textureOwnerProposal="consumed" /\ superProposal="consumed"
SuperSettlementFresh == superProposal="current" => textureOwnerProposal="current" /\ textureOwnerHead=ProjectedHead /\ superProposalHead=ProjectedHead
PostSettlementTerminal == settled => stage="idle" /\ pendingPlan=EmptyPlan /\ acknowledgedPlan=EmptyPlan /\ rebaseRequired={} /\ pendingDesired=None
LateResultsDispositioned == settled => \A r \in lateResults : resultDisposition[r]#None /\ attemptDisposition[resultAttempt[r]]#None
CandidateBoundToCurrentBase == \A c \in Candidates : candidateState[c] \in {"composed","verified","accepted","stale"} => composedResults[c]#{} /\ composedBase[c]#None /\ composedAtHead[c]#None /\ composedBase[c]#composedAtHead[c] /\ composedResultDigests[c]=ResultDigests(composedResults[c]) /\ \A r \in composedResults[c] : resultDisposition[r]="incorporated"
VerificationFreshAtAcceptance == \A c \in acceptedCandidates : acceptedHead[c]#None /\ verifiedHead[c]#None /\ verifiedBy[c] \in Verifiers /\ verificationEvidence[c] \in EvidenceIds
OnePendingDesired == pendingDesired=None \/ pendingDesired \in Candidates
CompensatingHistoryRecorded == compensationOpen => compensationHistory#{}
WritesDisabledAreQuiescent == ~writesEnabled => tapeSeq=disabledAt
RebuildIsReplayDerived == rebuiltHead=CurrentHead => rebuiltSemantic=ReplayFold(tapeSeq)
SuccessiveEventHeadsDiffer == \A i \in 2..Len(eventDigests) : eventDigests[i]#eventDigests[i-1]

\* Deterministic selectors let the witness specification prove reachability
\* without pretending the 39-event path is exhaustively explored.
\* Independent coverage obligations for branching model configurations.  They
\* intentionally name observable states rather than prescribe one trace.
CoverageAssignmentClosed == \E a \in Assignments : assignmentState[a]="closed"
CoverageRetry == \E t \in Attempts : attemptState[t]="started" /\ \E prior \in Attempts : attemptState[prior]="returned" /\ attemptDisposition[prior]="superseded"
CoverageCancelLate == \E r \in lateResults : resultDisposition[r]="late" /\ attemptDisposition[resultAttempt[r]]="late"
CoverageRebase == rebaseRequired#{} \/ \E a \in Assignments : rebaseDisposition[a]#None
CoverageSettlement == settled
CoverageVerification == \E c \in Candidates : candidateState[c] \in {"verified","accepted"} /\ verifiedBy[c] \in Verifiers /\ verificationEvidence[c] \in EvidenceIds
CoverageMaterialization == \E c \in Candidates : effectiveCandidate=c /\ c \in acceptedCandidates
NotCoverageAssignmentClosed == ~CoverageAssignmentClosed
NotCoverageRetry == ~CoverageRetry
NotCoverageCancelLate == ~CoverageCancelLate
NotCoverageRebase == ~CoverageRebase
NotCoverageSettlement == ~CoverageSettlement
NotCoverageVerification == ~CoverageVerification
NotCoverageMaterialization == ~CoverageMaterialization
WA1 == CHOOSE a \in Assignments : TRUE
WA2 == CHOOSE a \in Assignments \ {WA1} : TRUE
WA3 == CHOOSE a \in Assignments \ {WA1,WA2} : TRUE
WT1 == CHOOSE t \in Attempts : TRUE
WT2 == CHOOSE t \in Attempts \ {WT1} : TRUE
WT3 == CHOOSE t \in Attempts \ {WT1,WT2} : TRUE
WT4 == CHOOSE t \in Attempts \ {WT1,WT2,WT3} : TRUE
WR1 == CHOOSE r \in Results : TRUE
WR2 == CHOOSE r \in Results \ {WR1} : TRUE
WR3 == CHOOSE r \in Results \ {WR1,WR2} : TRUE
WR4 == CHOOSE r \in Results \ {WR1,WR2,WR3} : TRUE
WC1 == CHOOSE c \in Candidates : TRUE
WC2 == CHOOSE c \in Candidates \ {WC1} : TRUE
WitnessLength == 39

WitnessPlan(p,i) ==
  CASE i=1  -> p.kind="open_trajectory"
    [] i=2  -> p.kind="intent_revised" /\ p.affectedA={}
    [] i=3  -> p.kind="assignment_opened" /\ p.assignment=WA1
    [] i=4  -> p.kind="attempt_started" /\ p.assignment=WA1 /\ p.attempt=WT1
    [] i=5  -> p.kind="attempt_result" /\ p.attempt=WT1 /\ p.result=WR1
    [] i=6  -> p.kind="attempt_dispositioned" /\ p.attempt=WT1 /\ p.disposition="preserved"
    [] i=7  -> p.kind="result_dispositioned" /\ p.result=WR1 /\ p.disposition="incorporated"
    [] i=8  -> p.kind="assignment_closed" /\ p.assignment=WA1
    [] i=9  -> p.kind="assignment_opened" /\ p.assignment=WA2
    [] i=10 -> p.kind="attempt_started" /\ p.assignment=WA2 /\ p.attempt=WT2
    [] i=11 -> p.kind="attempt_result" /\ p.attempt=WT2 /\ p.result=WR2
    [] i=12 -> p.kind="attempt_dispositioned" /\ p.attempt=WT2 /\ p.disposition="superseded"
    [] i=13 -> p.kind="result_dispositioned" /\ p.result=WR2 /\ p.disposition="superseded"
    [] i=14 -> p.kind="attempt_retried" /\ p.assignment=WA2 /\ p.attempt=WT3 /\ p.prior=WT2
    [] i=15 -> p.kind="attempt_result" /\ p.attempt=WT3 /\ p.result=WR3
    [] i=16 -> p.kind="attempt_dispositioned" /\ p.attempt=WT3 /\ p.disposition="preserved"
    [] i=17 -> p.kind="result_dispositioned" /\ p.result=WR3 /\ p.disposition="incorporated"
    [] i=18 -> p.kind="assignment_closed" /\ p.assignment=WA2
    [] i=19 -> p.kind="assignment_opened" /\ p.assignment=WA3
    [] i=20 -> p.kind="attempt_started" /\ p.assignment=WA3 /\ p.attempt=WT4
    [] i=21 -> p.kind="assignment_cancelled" /\ p.assignment=WA3 /\ p.affectedT={WT4}
    [] i=22 -> p.kind="late_result" /\ p.attempt=WT4 /\ p.result=WR4
    [] i=23 -> p.kind="attempt_dispositioned" /\ p.attempt=WT4 /\ p.disposition="late"
    [] i=24 -> p.kind="result_dispositioned" /\ p.result=WR4 /\ p.disposition="late"
    [] i=25 -> p.kind="assignment_closed" /\ p.assignment=WA3
    [] i=26 -> p.kind="evidence_completed"
    [] i=27 -> p.kind="artifact_head_completed"
    [] i=28 -> p.kind="candidate_composed" /\ p.candidate=WC1 /\ p.selected={WR1,WR3}
    [] i=29 -> p.kind="candidate_verified" /\ p.candidate=WC1 /\ p.actor \in Verifiers /\ p.evidence \in EvidenceIds
    [] i=30 -> p.kind="candidate_accepted" /\ p.candidate=WC1
    [] i=31 -> p.kind="materialization_failed" /\ p.candidate=WC1
    [] i=32 -> p.kind="compensation_resolved"
    [] i=33 -> p.kind="candidate_composed" /\ p.candidate=WC2 /\ p.selected={WR1,WR3}
    [] i=34 -> p.kind="candidate_verified" /\ p.candidate=WC2 /\ p.actor \in Verifiers /\ p.evidence \in EvidenceIds
    [] i=35 -> p.kind="candidate_accepted" /\ p.candidate=WC2
    [] i=36 -> p.kind="materialization_applied" /\ p.candidate=WC2
    [] i=37 -> p.kind="texture_owner_settlement_proposed"
    [] i=38 -> p.kind="super_settlement_proposed"
    [] i=39 -> p.kind="trajectory_settled"
    [] OTHER -> FALSE

WitnessDigest == CHOOSE d \in Digests : CurrentHead=None \/ d#CurrentHead
WitnessPrepare ==
  /\ stage="idle" /\ activeKey \in CommandKeys /\ commandState[activeKey]="frozen"
  /\ LET p == CHOOSE q \in Plans : WitnessPlan(q,tapeSeq+1) /\ q.digest=reservationDigest[activeKey] /\ PlanShape(q)
     IN Enabled(p) /\ Prepare(p)
WitnessNext ==
  \/ /\ stage="idle" /\ activeKey=None /\ tapeSeq<WitnessLength
     /\ ~(tapeSeq=30 /\ secondPendingRefusals=0)
     /\ Reserve(NextCommandKey,WitnessDigest)
  \/ /\ stage="idle" /\ activeKey \in CommandKeys /\ commandState[activeKey]="reserved"
     /\ Freeze(activeKey)
  \/ WitnessPrepare
  \/ /\ stage="prepared" /\ CAS
  \/ /\ stage="acknowledged" /\ Finalize
  \/ /\ stage="idle" /\ tapeSeq=30 /\ secondPendingRefusals=0 /\ RefuseSecond(WC2)
WitnessSpec == Init /\ [][WitnessNext]_vars /\ WF_vars(WitnessNext)
WitnessComplete == settled /\ tapeSeq=WitnessLength /\ effectiveCandidate=WC2 /\ secondPendingRefusals=1 /\ "materialization_failed" \in compensationHistory /\ \A a \in Assignments : assignmentState[a]="closed"
EventuallyWitnessComplete == <>WitnessComplete
=============================================================================
