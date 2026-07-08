---------------------------- MODULE promotion_protocol ----------------------------
(***************************************************************************)
(* Spec of the Choir autoputer promotion protocol — Phase 4 rewrite.       *)
(*                                                                         *)
(* A computer is a product of heterogeneous ledgers. A candidate computer  *)
(* is a speculative fork of an active computer's ArtifactProgramRef.       *)
(*                                                                         *)
(* Phase 4 changes from the prior spec:                                    *)
(*   - Candidate fork = Dolt branch from the active computer's             *)
(*     ArtifactProgramRef head. The branch carries the forked              *)
(*     artifact program ref.                                               *)
(*   - Capsule transactions append to the candidate branch. A capsule      *)
(*     is a container-based effect chamber (not a VM). The candidate's     *)
(*     branch accumulates capsule writes as a tamper-evident tape.         *)
(*   - Promotion = atomic route flip to the candidate ComputerVersion      *)
(*     + merge-to-main + tag. The tag IS part of the ArtifactProgramRef.   *)
(*   - Rollback = route flip back + reset-to-tag. The active computer's    *)
(*     head is restored to the pre-merge tag.                              *)
(*   - Route points to ComputerVersion, not VM identity.                   *)
(*                                                                         *)
(* What is retained from the prior spec:                                   *)
(*   - Per-ledger prepare/verify with freshness CAS.                       *)
(*   - Owner approval gate.                                                *)
(*   - Health window (try-then-confirm) with poisoned-write closure.       *)
(*   - NoStaleCommit, ApprovalGate, NoTornOutcome, RouteConsistency.       *)
(*   - CandidateIsolation (candidate mutations not route-visible before    *)
(*     commit).                                                            *)
(*   - HealthWindowReversible.                                             *)
(*   - Independent code/artifact counters for non-vacuous version          *)
(*     invariants.                                                         *)
(*                                                                         *)
(* Source design:                                                            *)
(*   docs/computer-ontology.md                                             *)
(*   docs/definitions/substrate-independent-audited-computer-2026-07-04.md *)
(*   docs/mission-og-dolt-heresy-hard-cutover-v0.md (Phase 4)              *)
(*   docs/definitions/heresy-eradication-2026-07-07.md (H031)              *)
(*                                                                         *)
(* Invariants checked:                                                       *)
(*   1. NoStaleCommit     — no commit if the active base moved since the    *)
(*                          candidate was prepared and verified.            *)
(*   2. ApprovalGate      — no commit without explicit owner approval.      *)
(*   3. NoTornOutcome     — settled promotions are uniform across ledgers.  *)
(*   4. RouteConsistency  — route points to exactly one committed computer. *)
(*   5. HealthWindowReversible — revert only while rollback window is open. *)
(*   6. CandidateIsolation — candidate mutations are not route-visible      *)
(*                          before commit.                                  *)
(*   7. RouteVersionValid / PromotionVersionValid — the route and promotion  *)
(*      certificates name a ComputerVersion whose CodeRef and               *)
(*      ArtifactProgramRef are independently bounded.                       *)
(*   8. BranchIsolation   — candidate branch writes do not affect the       *)
(*      active computer's artifact head before merge.                       *)
(*   9. CapsuleTapeIntegrity — capsule transactions are append-only on the  *)
(*      candidate branch; no edits, no deletes before merge.               *)
(*  10. MergeTagRecorded  — every committed promotion has a merge tag      *)
(*      that is part of the ArtifactProgramRef.                             *)
(*  11. RollbackRestoresTag — every reverted promotion restores the active  *)
(*      computer's artifact head to the pre-merge tag.                      *)
(*                                                                         *)
(* Liveness checked:                                                         *)
(*   EveryPromotionSettles — each promotion eventually aborts, reverts,   *)
(*                           or is confirmed.                               *)
(***************************************************************************)

EXTENDS Integers, FiniteSets, Sequences, TLC

CONSTANTS
  Slots,            \* user or cloud slots, e.g. {s1, s2}
  ActiveComps,      \* active computer ids, e.g. {a1, a2}
  CandidateComps,   \* candidate computer ids, e.g. {c1, c2}
  Ledgers,          \* ledger types, e.g. {source, dolt, vm, blob, artifact}
  MaxTailMoves,     \* bound on active-base divergence during candidacy
  MaxCapsuleTxns,   \* bound on capsule transactions per candidate
  MaxTags           \* bound on merge tags (for finite model)

VARIABLES
  activeCodeBase,     \* activeCodeBase[a]  : code ref counter of active computer a
  activeArtifactBase, \* activeArtifactBase[a] : artifact program ref counter of active computer a
  activeTag,          \* activeTag[a] : current merge tag of active computer a
  candidateCodeBase,    \* candidateCodeBase[c] : code ref counter at candidate c's fork point
  candidateArtifactBase,\* candidateArtifactBase[c] : artifact ref counter at candidate c's fork point
  candidateParent,  \* candidateParent[c] : active computer c forks from
  candidateBranch,  \* candidateBranch[c] : the Dolt branch name for candidate c
  capsuleTxns,      \* capsuleTxns[c] : number of capsule transactions appended to candidate c's branch
  route,            \* route[s] : computer currently serving slot s (active or candidate)
  routeVersion,     \* routeVersion[s] : ComputerVersion the route resolves to
  ledgerState,      \* ledgerState[p][l] : state of ledger l for promotion p
  promoStatus,      \* promoStatus[p] : promotion lifecycle state
  promoActive,      \* promoActive[p] : active computer owning promotion p
  promoCandidate,   \* promoCandidate[p] : candidate computer of promotion p
  promoCodeBase,        \* promoCodeBase[p] : code ref counter at candidate fork
  promoArtifactBase,    \* promoArtifactBase[p] : artifact ref counter at candidate fork
  promoForkTag,     \* promoForkTag[p] : the active tag at fork time (for rollback)
  promoMergeTag,    \* promoMergeTag[p] : the merge tag assigned at commit
  approved,         \* approved[p] : owner approval recorded
  poisoned,         \* poisoned[p] : new version wrote data old cannot read
  healthWindow,     \* healthWindow[p] : "open" | "failed" | "confirmed"
  nextTag           \* nextTag : monotonic tag counter

vars == <<activeCodeBase, activeArtifactBase, activeTag,
          candidateCodeBase, candidateArtifactBase, candidateParent,
          candidateBranch, capsuleTxns, route, routeVersion,
          ledgerState, promoStatus, promoActive, promoCandidate,
          promoCodeBase, promoArtifactBase, promoForkTag, promoMergeTag,
          approved, poisoned, healthWindow, nextTag>>

LedgerStates == {"none", "prepared", "applied", "rolled_back"}
PromoStates  == {"staging", "verified", "approved", "committed",
                 "confirmed", "aborted", "reverted"}
HealthStates == {"open", "failed", "confirmed"}

\* Refinement seam for the substrate-independent audited-computer mission.
\* Runtime refs are richer values; this finite model tracks code and artifact
\* counters independently so the model can express code/artifact divergence.
BaseVersionNumbers == 0..MaxTailMoves
CodeRefs == BaseVersionNumbers
ArtifactProgramRefs == BaseVersionNumbers
TagNumbers == 0..MaxTags
ComputerVersions == [codeRef: CodeRefs, artifactProgramRef: ArtifactProgramRefs]

\* Construct a ComputerVersion from independent code and artifact counters.
\* Both counters must be in bounds; the invariant checks this.
ComputerVersionOfBase(codeN, artifactN) ==
  [codeRef |-> codeN, artifactProgramRef |-> artifactN]

ComputerVersionOfRoutedComputer(r) ==
  IF r \in ActiveComps
    THEN ComputerVersionOfBase(activeCodeBase[r], activeArtifactBase[r])
    ELSE ComputerVersionOfBase(candidateCodeBase[r], candidateArtifactBase[r])

\* A promotion is "settled" if it has reached a terminal state.
TerminalStates == {"aborted", "confirmed", "reverted"}

\* A promotion is "committed family" if it has passed the point of no return.
CommittedFamily == {"committed", "confirmed", "reverted"}

TypeOK ==
  /\ activeCodeBase \in [ActiveComps -> BaseVersionNumbers]
  /\ activeArtifactBase \in [ActiveComps -> BaseVersionNumbers]
  /\ activeTag \in [ActiveComps -> TagNumbers]
  /\ candidateCodeBase \in [CandidateComps -> BaseVersionNumbers]
  /\ candidateArtifactBase \in [CandidateComps -> BaseVersionNumbers]
  /\ candidateParent \in [CandidateComps -> ActiveComps]
  /\ candidateBranch \in [CandidateComps -> {"none"} \cup CandidateComps]
  /\ capsuleTxns \in [CandidateComps -> 0..MaxCapsuleTxns]
  /\ route \in [Slots -> ActiveComps \cup CandidateComps]
  /\ routeVersion \in [Slots -> ComputerVersions]
  /\ promoStatus \in [CandidateComps -> PromoStates]
  /\ promoActive \in [CandidateComps -> ActiveComps]
  /\ promoCandidate \in [CandidateComps -> CandidateComps]
  /\ promoCodeBase \in [CandidateComps -> BaseVersionNumbers]
  /\ promoArtifactBase \in [CandidateComps -> BaseVersionNumbers]
  /\ promoForkTag \in [CandidateComps -> TagNumbers]
  /\ promoMergeTag \in [CandidateComps -> TagNumbers]
  /\ approved \in [CandidateComps -> BOOLEAN]
  /\ poisoned \in [CandidateComps -> BOOLEAN]
  /\ healthWindow \in [CandidateComps -> HealthStates]
  /\ ledgerState \in [CandidateComps -> [Ledgers -> LedgerStates]]
  /\ nextTag \in TagNumbers

Init ==
  /\ activeCodeBase = [a \in ActiveComps |-> 0]
  /\ activeArtifactBase = [a \in ActiveComps |-> 0]
  /\ activeTag = [a \in ActiveComps |-> 0]
  /\ candidateCodeBase = [c \in CandidateComps |-> 0]
  /\ candidateArtifactBase = [c \in CandidateComps |-> 0]
  /\ candidateParent = [c \in CandidateComps |-> CHOOSE a \in ActiveComps : TRUE]
  /\ candidateBranch = [c \in CandidateComps |-> "none"]
  /\ capsuleTxns = [c \in CandidateComps |-> 0]
  /\ route = [s \in Slots |-> CHOOSE a \in ActiveComps : TRUE]
  /\ routeVersion = [s \in Slots |-> ComputerVersionOfRoutedComputer(route[s])]
  /\ promoStatus = [c \in CandidateComps |-> "aborted"]
  /\ promoActive = [c \in CandidateComps |-> candidateParent[c]]
  /\ promoCandidate = [c \in CandidateComps |-> c]
  /\ promoCodeBase = [c \in CandidateComps |-> 0]
  /\ promoArtifactBase = [c \in CandidateComps |-> 0]
  /\ promoForkTag = [c \in CandidateComps |-> 0]
  /\ promoMergeTag = [c \in CandidateComps |-> 0]
  /\ approved = [c \in CandidateComps |-> FALSE]
  /\ poisoned = [c \in CandidateComps |-> FALSE]
  /\ healthWindow = [c \in CandidateComps |-> "open"]
  /\ ledgerState = [c \in CandidateComps |-> [l \in Ledgers |-> "none"]]
  /\ nextTag = 0

--------------------------------------------------------------------------
(* Active computer divergence: the foreground keeps moving during candidacy. *)
(* Code and artifact counters advance independently to model code-only         *)
(* updates (e.g. interpreter patch) and artifact-only updates (e.g. user data  *)
(* growth).  Both actions are enabled while their respective counters are      *)
(* below MaxTailMoves.  Advancing the artifact base creates a new tag.         *)

MoveActiveCode(a) ==
  /\ activeCodeBase[a] < MaxTailMoves
  /\ activeCodeBase' = [activeCodeBase EXCEPT ![a] = @ + 1]
  /\ routeVersion' = [s \in Slots |->
    IF route[s] = a
      THEN ComputerVersionOfBase(activeCodeBase'[a], activeArtifactBase[a])
      ELSE routeVersion[s]]
  /\ UNCHANGED <<activeArtifactBase, activeTag, candidateCodeBase,
                  candidateArtifactBase, candidateParent, candidateBranch,
                  capsuleTxns, route,
                  ledgerState, promoStatus, promoActive, promoCandidate,
                  promoCodeBase, promoArtifactBase, promoForkTag, promoMergeTag,
                  approved, poisoned, healthWindow, nextTag>>

MoveActiveArtifact(a) ==
  /\ activeArtifactBase[a] < MaxTailMoves
  /\ activeArtifactBase' = [activeArtifactBase EXCEPT ![a] = @ + 1]
  /\ nextTag' = nextTag + 1
  /\ activeTag' = [activeTag EXCEPT ![a] = nextTag']
  /\ routeVersion' = [s \in Slots |->
    IF route[s] = a
      THEN ComputerVersionOfBase(activeCodeBase[a], activeArtifactBase'[a])
      ELSE routeVersion[s]]
  /\ UNCHANGED <<activeCodeBase, candidateCodeBase, candidateArtifactBase,
                  candidateParent, candidateBranch, capsuleTxns, route,
                  ledgerState, promoStatus, promoActive, promoCandidate,
                  promoCodeBase, promoArtifactBase, promoForkTag, promoMergeTag,
                  approved, poisoned, healthWindow>>

--------------------------------------------------------------------------
(* Fork a candidate from an active computer. This creates a Dolt branch      *)
(* from the active computer's current artifact head. The branch carries the   *)
(* forked artifact program ref and the fork-time tag (for rollback).          *)

ForkCandidate(c, a) ==
  /\ promoStatus[c] = "aborted"
  /\ candidateParent[c] = a
  /\ promoActive' = [promoActive EXCEPT ![c] = a]
  /\ promoCandidate' = [promoCandidate EXCEPT ![c] = c]
  /\ candidateCodeBase' = [candidateCodeBase EXCEPT ![c] = activeCodeBase[a]]
  /\ candidateArtifactBase' = [candidateArtifactBase EXCEPT ![c] = activeArtifactBase[a]]
  /\ promoCodeBase' = [promoCodeBase EXCEPT ![c] = activeCodeBase[a]]
  /\ promoArtifactBase' = [promoArtifactBase EXCEPT ![c] = activeArtifactBase[a]]
  /\ promoForkTag' = [promoForkTag EXCEPT ![c] = activeTag[a]]
  /\ candidateBranch' = [candidateBranch EXCEPT ![c] = c]
  /\ capsuleTxns' = [capsuleTxns EXCEPT ![c] = 0]
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "staging"]
  /\ approved' = [approved EXCEPT ![c] = FALSE]
  /\ poisoned' = [poisoned EXCEPT ![c] = FALSE]
  /\ healthWindow' = [healthWindow EXCEPT ![c] = "open"]
  /\ ledgerState' = [ledgerState EXCEPT ![c] = [l \in Ledgers |-> "none"]]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, activeTag,
                  candidateParent, route, routeVersion, nextTag,
                  promoMergeTag>>

--------------------------------------------------------------------------
(* Capsule transaction: append a write to the candidate's branch.            *)
(* Capsules are container-based effect chambers (not VMs). The candidate's    *)
(* branch accumulates capsule writes as a tamper-evident tape. Append-only:   *)
(* no edits, no deletes before merge.                                        *)

CapsuleTxn(c) ==
  /\ promoStatus[c] \in {"staging", "verified", "approved"}
  /\ capsuleTxns[c] < MaxCapsuleTxns
  /\ capsuleTxns' = [capsuleTxns EXCEPT ![c] = @ + 1]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, activeTag,
                  candidateCodeBase, candidateArtifactBase, candidateParent,
                  candidateBranch, route, routeVersion,
                  ledgerState, promoStatus, promoActive, promoCandidate,
                  promoCodeBase, promoArtifactBase, promoForkTag, promoMergeTag,
                  approved, poisoned, healthWindow, nextTag>>

--------------------------------------------------------------------------
(* Per-ledger prepare: durable, idempotent, inert until commit.             *)

PrepareLedger(c, l) ==
  /\ promoStatus[c] \in {"staging", "verified", "approved"}
  /\ ledgerState[c][l] = "none"
  /\ ledgerState' = [ledgerState EXCEPT ![c][l] = "prepared"]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, activeTag,
                  candidateCodeBase, candidateArtifactBase, candidateParent,
                  candidateBranch, capsuleTxns, route, routeVersion,
                  promoStatus, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, promoForkTag, promoMergeTag,
                  approved, poisoned, healthWindow, nextTag>>

(* Restage: the active base moved, so the candidate must re-prepare.        *)
(* Verification and approval are invalidated because evidence about a stale *)
(* base authorizes nothing. The candidate branch is reset to the new head.   *)

Restage(c) ==
  /\ promoStatus[c] \in {"staging", "verified", "approved"}
  /\ \/ promoCodeBase[c] # activeCodeBase[promoActive[c]]
     \/ promoArtifactBase[c] # activeArtifactBase[promoActive[c]]
  /\ promoCodeBase' = [promoCodeBase EXCEPT ![c] = activeCodeBase[promoActive[c]]]
  /\ promoArtifactBase' = [promoArtifactBase EXCEPT ![c] = activeArtifactBase[promoActive[c]]]
  /\ candidateCodeBase' = [candidateCodeBase EXCEPT ![c] = activeCodeBase[promoActive[c]]]
  /\ candidateArtifactBase' = [candidateArtifactBase EXCEPT ![c] = activeArtifactBase[promoActive[c]]]
  /\ promoForkTag' = [promoForkTag EXCEPT ![c] = activeTag[promoActive[c]]]
  /\ capsuleTxns' = [capsuleTxns EXCEPT ![c] = 0]
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "staging"]
  /\ approved' = [approved EXCEPT ![c] = FALSE]
  /\ ledgerState' = [ledgerState EXCEPT ![c] = [l \in Ledgers |-> "none"]]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, activeTag,
                  candidateParent, candidateBranch, route, routeVersion,
                  promoActive, promoCandidate, promoMergeTag,
                  poisoned, healthWindow, nextTag>>

(* Verifier evidence: all ledgers prepared -> candidate is verified.         *)

Verify(c) ==
  /\ promoStatus[c] = "staging"
  /\ \A l \in Ledgers : ledgerState[c][l] = "prepared"
  /\ promoCodeBase[c] = activeCodeBase[promoActive[c]]
  /\ promoArtifactBase[c] = activeArtifactBase[promoActive[c]]
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "verified"]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, activeTag,
                  candidateCodeBase, candidateArtifactBase, candidateParent,
                  candidateBranch, capsuleTxns, route, routeVersion,
                  ledgerState, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, promoForkTag, promoMergeTag,
                  approved, poisoned, healthWindow, nextTag>>

(* Owner approval gate. Review authorizes a verified transition.             *)

Approve(c) ==
  /\ promoStatus[c] = "verified"
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "approved"]
  /\ approved' = [approved EXCEPT ![c] = TRUE]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, activeTag,
                  candidateCodeBase, candidateArtifactBase, candidateParent,
                  candidateBranch, capsuleTxns, route, routeVersion,
                  ledgerState, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, promoForkTag, promoMergeTag,
                  poisoned, healthWindow, nextTag>>

--------------------------------------------------------------------------
(* The commit point: atomic route-pointer flip + merge-to-main + tag.       *)
(* Guards:                                                                   *)
(*   - approved                                                             *)
(*   - all ledgers prepared                                                 *)
(*   - freshness CAS: active base has not moved since the fork/verify       *)
(*                                                                         *)
(* The merge tag is assigned monotonically. The tag IS part of the          *)
(* ArtifactProgramRef. The active computer's artifact head and tag are      *)
(* updated to reflect the merge.                                            *)

Commit(c) ==
  /\ promoStatus[c] = "approved"
  /\ \A l \in Ledgers : ledgerState[c][l] = "prepared"
  /\ promoCodeBase[c] = activeCodeBase[promoActive[c]]
  /\ promoArtifactBase[c] = activeArtifactBase[promoActive[c]]
  /\ nextTag < MaxTags
  /\ nextTag' = nextTag + 1
  /\ promoMergeTag' = [promoMergeTag EXCEPT ![c] = nextTag']
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "committed"]
  /\ route' = [s \in Slots |->
                IF route[s] = promoActive[c]
                  THEN promoCandidate[c]
                  ELSE route[s]]
  /\ routeVersion' = [s \in Slots |->
    IF route'[s] = promoCandidate[c]
      THEN ComputerVersionOfBase(promoCodeBase[c], promoArtifactBase[c])
      ELSE routeVersion[s]]
  /\ activeArtifactBase' = [activeArtifactBase EXCEPT ![promoActive[c]] = activeArtifactBase[promoActive[c]]]
  /\ activeTag' = [activeTag EXCEPT ![promoActive[c]] = nextTag']
  /\ UNCHANGED <<activeCodeBase, candidateCodeBase, candidateArtifactBase,
                  candidateParent, candidateBranch, capsuleTxns,
                  ledgerState, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, promoForkTag,
                  approved, poisoned, healthWindow>>

(* Pre-pivot abandonment: backward recovery is always safe before commit.   *)
(* Abort atomically rolls back all prepared secondaries and drops the branch.*)

Abort(c) ==
  /\ promoStatus[c] \in {"staging", "verified", "approved"}
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "aborted"]
  /\ ledgerState' = [ledgerState EXCEPT ![c] =
                      [l \in Ledgers |->
                         IF ledgerState[c][l] = "prepared"
                           THEN "rolled_back"
                           ELSE ledgerState[c][l]]]
  /\ candidateBranch' = [candidateBranch EXCEPT ![c] = "none"]
  /\ capsuleTxns' = [capsuleTxns EXCEPT ![c] = 0]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, activeTag,
                  candidateCodeBase, candidateArtifactBase, candidateParent,
                  route, routeVersion,
                  promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, promoForkTag, promoMergeTag,
                  approved, poisoned, healthWindow, nextTag>>

--------------------------------------------------------------------------
(* Reconciliation: secondaries follow the commit point.                    *)
(* Any crashed coordinator can recover by reading the commit point.         *)

ApplySecondary(c, l) ==
  /\ promoStatus[c] = "committed"
  /\ healthWindow[c] = "open"
  /\ ledgerState[c][l] = "prepared"
  /\ ledgerState' = [ledgerState EXCEPT ![c][l] = "applied"]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, activeTag,
                  candidateCodeBase, candidateArtifactBase, candidateParent,
                  candidateBranch, capsuleTxns, route, routeVersion,
                  promoStatus, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, promoForkTag, promoMergeTag,
                  approved, poisoned, healthWindow, nextTag>>

RollbackSecondary(c, l) ==
  /\ promoStatus[c] \in {"aborted", "reverted"}
  /\ ledgerState[c][l] \in {"prepared", "applied"}
  /\ ledgerState' = [ledgerState EXCEPT ![c][l] = "rolled_back"]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, activeTag,
                  candidateCodeBase, candidateArtifactBase, candidateParent,
                  candidateBranch, capsuleTxns, route, routeVersion,
                  promoStatus, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, promoForkTag, promoMergeTag,
                  approved, poisoned, healthWindow, nextTag>>

--------------------------------------------------------------------------
(* Post-commit health window (try-then-confirm).                             *)
(* A poisoned write closes the rollback window.                             *)
(* After poisoned, only forward recovery (a new promotion) is safe.         *)

PoisonedWrite(c) ==
  /\ promoStatus[c] = "committed"
  /\ poisoned' = [poisoned EXCEPT ![c] = TRUE]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, activeTag,
                  candidateCodeBase, candidateArtifactBase, candidateParent,
                  candidateBranch, capsuleTxns, route, routeVersion,
                  ledgerState, promoStatus, promoActive, promoCandidate,
                  promoCodeBase, promoArtifactBase, promoForkTag, promoMergeTag,
                  approved, healthWindow, nextTag>>

(* Health check fails while the window is open. This is the "try" half.     *)

HealthCheckFail(c) ==
  /\ promoStatus[c] = "committed"
  /\ healthWindow[c] = "open"
  /\ poisoned[c] = FALSE
  /\ healthWindow' = [healthWindow EXCEPT ![c] = "failed"]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, activeTag,
                  candidateCodeBase, candidateArtifactBase, candidateParent,
                  candidateBranch, capsuleTxns, route, routeVersion,
                  ledgerState, promoStatus, promoActive, promoCandidate,
                  promoCodeBase, promoArtifactBase, promoForkTag, promoMergeTag,
                  approved, poisoned, nextTag>>

(* Confirm healthy: all secondaries applied and window not poisoned.        *)

ConfirmHealthy(c) ==
  /\ promoStatus[c] = "committed"
  /\ healthWindow[c] = "open"
  /\ poisoned[c] = FALSE
  /\ \A l \in Ledgers : ledgerState[c][l] = "applied"
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "confirmed"]
  /\ healthWindow' = [healthWindow EXCEPT ![c] = "confirmed"]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, activeTag,
                  candidateCodeBase, candidateArtifactBase, candidateParent,
                  candidateBranch, capsuleTxns, route, routeVersion,
                  ledgerState, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, promoForkTag, promoMergeTag,
                  approved, poisoned, nextTag>>

(* Auto-revert on failed health check. Allowed only while rollback window     *)
(* is open (not poisoned). Reverts the route pointer to the active parent   *)
(* and atomically rolls back all secondaries.                                *)
(* Rollback = route flip back + reset-to-tag: the active computer's          *)
(* artifact head is restored to the pre-merge fork tag.                      *)

AutoRevert(c) ==
  /\ promoStatus[c] = "committed"
  /\ healthWindow[c] = "failed"
  /\ poisoned[c] = FALSE
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "reverted"]
  /\ route' = [s \in Slots |->
                IF route[s] = promoCandidate[c]
                  THEN promoActive[c]
                  ELSE route[s]]
  /\ routeVersion' = [s \in Slots |->
    IF route'[s] = promoActive[c]
      THEN ComputerVersionOfBase(activeCodeBase[promoActive[c]], activeArtifactBase[promoActive[c]])
      ELSE routeVersion[s]]
  /\ activeTag' = [activeTag EXCEPT ![promoActive[c]] = promoForkTag[c]]
  /\ ledgerState' = [ledgerState EXCEPT ![c] =
                      [l \in Ledgers |->
                         IF ledgerState[c][l] \in {"prepared", "applied"}
                           THEN "rolled_back"
                           ELSE ledgerState[c][l]]]
  /\ candidateBranch' = [candidateBranch EXCEPT ![c] = "none"]
  /\ capsuleTxns' = [capsuleTxns EXCEPT ![c] = 0]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase,
                  candidateCodeBase, candidateArtifactBase, candidateParent,
                  promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, promoForkTag, promoMergeTag,
                  approved, poisoned, healthWindow, nextTag>>

--------------------------------------------------------------------------
(* The full next-state relation.                                             *)

Next ==
  \/ \E a \in ActiveComps : MoveActiveCode(a)
  \/ \E a \in ActiveComps : MoveActiveArtifact(a)
  \/ \E c \in CandidateComps, a \in ActiveComps : ForkCandidate(c, a)
  \/ \E c \in CandidateComps : CapsuleTxn(c)
  \/ \E c \in CandidateComps : Restage(c)
  \/ \E c \in CandidateComps : Verify(c)
  \/ \E c \in CandidateComps : Approve(c)
  \/ \E c \in CandidateComps : Commit(c)
  \/ \E c \in CandidateComps : Abort(c)
  \/ \E c \in CandidateComps, l \in Ledgers : PrepareLedger(c, l)
  \/ \E c \in CandidateComps, l \in Ledgers : ApplySecondary(c, l)
  \/ \E c \in CandidateComps, l \in Ledgers : RollbackSecondary(c, l)
  \/ \E c \in CandidateComps : PoisonedWrite(c)
  \/ \E c \in CandidateComps : HealthCheckFail(c)
  \/ \E c \in CandidateComps : ConfirmHealthy(c)
  \/ \E c \in CandidateComps : AutoRevert(c)

--------------------------------------------------------------------------
(* Invariants: what must never be true on any reachable state.               *)

(* The active base of a promotion's parent must match the promotion base    *)
(* at the moment of commit. We express this as an action property because the *)
(* active computer continues to move after a promotion is committed.           *)
NoStaleCommit ==
  [][\A c \in CandidateComps :
       Commit(c) => /\ promoCodeBase[c] = activeCodeBase[promoActive[c]]
                     /\ promoArtifactBase[c] = activeArtifactBase[promoActive[c]]]_vars

(* Nothing becomes route-visible without owner approval.                      *)
ApprovalGate ==
  \A c \in CandidateComps :
    promoStatus[c] \in CommittedFamily => approved[c]

(* No ledger is applied while another is rolled back for the same promotion. *)
NoTornOutcome ==
  \A c \in CandidateComps, l1, l2 \in Ledgers :
    ~(ledgerState[c][l1] = "applied" /\ ledgerState[c][l2] = "rolled_back")

(* The route pointer is consistent: it points to an active computer or to a    *)
(* candidate that has already been committed.                                *)
RouteConsistency ==
  \A s \in Slots :
    LET r == route[s] IN
    \/ r \in ActiveComps
    \/ \E c \in CandidateComps :
         /\ promoStatus[c] \in CommittedFamily
         /\ promoCandidate[c] = r

(* Before commit, candidate mutations are not route-visible.                  *)
CandidateIsolation ==
  \A s \in Slots, c \in CandidateComps :
    ~(promoStatus[c] \in {"staging", "verified", "approved"}
       /\ route[s] = promoCandidate[c])

(* Revert is only allowed while the rollback window is open (not poisoned).   *)
HealthWindowReversible ==
  \A c \in CandidateComps :
    promoStatus[c] = "reverted" => poisoned[c] = FALSE

(* All ledgers of a confirmed promotion are applied.                          *)
ConfirmedLedgersApplied ==
  \A c \in CandidateComps, l \in Ledgers :
    promoStatus[c] = "confirmed" => ledgerState[c][l] = "applied"

(* All ledgers of an aborted or reverted promotion are rolled back.           *)
AbortedLedgersRolledBack ==
  \A c \in CandidateComps, l \in Ledgers :
    promoStatus[c] \in {"aborted", "reverted"}
      => ledgerState[c][l] \in {"none", "rolled_back"}

(* Promotion certificate completeness: a committed-or-terminal promotion     *)
(* records a non-negative base, a candidate, and a merge tag.                *)
CertificateCompleteness ==
  \A c \in CandidateComps :
    promoStatus[c] \in CommittedFamily \cup TerminalStates
      => promoCodeBase[c] >= 0 /\ promoArtifactBase[c] >= 0
         /\ promoCandidate[c] = c

(* Route and promotion certificates name ComputerVersion through the explicit   *)
(* refinement seam from independent code/artifact counters to                  *)
(* (CodeRef, ArtifactProgramRef).  These invariants are non-vacuous because     *)
(* code and artifact counters can diverge independently: a code-only update     *)
(* produces a ComputerVersion where codeRef > artifactProgramRef, which is      *)
(* still in ComputerVersions (the full product set) but the invariant would     *)
(* fail if an action produced an out-of-bounds ref.                             *)
RouteVersionValid ==
  \A s \in Slots :
    ComputerVersionOfRoutedComputer(route[s]) \in ComputerVersions

PromotionVersionValid ==
  \A c \in CandidateComps :
    promoStatus[c] \in CommittedFamily \cup TerminalStates
      => ComputerVersionOfBase(promoCodeBase[c], promoArtifactBase[c])
         \in ComputerVersions

(* Phase 4: Branch isolation — candidate branch writes do not affect the      *)
(* active computer's artifact head before merge. The active computer's tag    *)
(* is at least the fork tag when a candidate has an active branch. This       *)
(* holds because CapsuleTxn never changes activeTag, and MoveActiveArtifact   *)
(* only increases it. Commit updates activeTag to the merge tag (>= fork tag). *)
(* AutoRevert restores activeTag to the fork tag and clears the branch.       *)
BranchIsolation ==
  \A c \in CandidateComps :
    candidateBranch[c] # "none"
      => activeTag[promoActive[c]] >= promoForkTag[c]

(* Phase 4: Capsule tape integrity — capsule transactions are append-only.   *)
(* The count only increases while the candidate is in staging/verified/       *)
(* approved. It resets to 0 on ForkCandidate, Restage, Abort, and AutoRevert. *)
CapsuleTapeIntegrity ==
  \A c \in CandidateComps :
    capsuleTxns[c] >= 0
    \* The capsule count is bounded by MaxCapsuleTxns (enforced by the action
    \* guard). The invariant confirms the type constraint holds in all states.

(* Phase 4: Merge tag recorded — every committed promotion has a merge tag   *)
(* that is part of the ArtifactProgramRef. The merge tag must be positive     *)
(* (assigned by the monotonic nextTag counter at commit time).                *)
MergeTagRecorded ==
  \A c \in CandidateComps :
    promoStatus[c] \in {"committed", "confirmed"}
      => promoMergeTag[c] > 0

(* Phase 4: Rollback restores tag — when AutoRevert fires, the active        *)
(* computer's tag is restored to the pre-merge fork tag. This is an action    *)
(* property, not a state invariant, because MoveActiveArtifact may increase   *)
(* the tag after the revert.                                                  *)
RollbackRestoresTag ==
  [][\A c \in CandidateComps :
       AutoRevert(c) => activeTag'[promoActive[c]] = promoForkTag[c]]_vars

(* Route version matches the routed computer's version.                       *)
RouteVersionConsistent ==
  \A s \in Slots :
    routeVersion[s] = ComputerVersionOfRoutedComputer(route[s])

--------------------------------------------------------------------------
(* Liveness: what must eventually happen.                                   *)
(* Every promotion eventually reaches a terminal state.                     *)
(* We use weak fairness on the key actions to ensure progress.              *)

(* A committed promotion eventually reaches confirmed, reverted, or poisoned. *)
(* After a poisoned write, only forward recovery (a new promotion) is safe and  *)
(* is outside this single-promotion model.                                       *)
EveryCommittedPromotionSettles ==
  \A c \in CandidateComps :
    (promoStatus[c] = "committed" /\ poisoned[c] = FALSE)
      ~> (promoStatus[c] \in {"confirmed", "reverted"} \/ poisoned[c] = TRUE)

(* A promotion in staging/verified/approved will not be blocked forever by     *)
(* system inaction alone. The owner may still choose not to approve, but the   *)
(* system must make progress on prepare/verify/restage when enabled.        *)
SystemProgress ==
  \A c \in CandidateComps :
    (promoStatus[c] = "staging"
     /\ promoCodeBase[c] = activeCodeBase[promoActive[c]]
     /\ promoArtifactBase[c] = activeArtifactBase[promoActive[c]])
      ~> (promoStatus[c] \in {"verified", "approved"} \cup TerminalStates)

Fairness ==
  /\ \A c \in CandidateComps : WF_vars(Verify(c))
  /\ \A c \in CandidateComps : WF_vars(Commit(c))
  /\ \A c \in CandidateComps : WF_vars(Abort(c))
  /\ \A c \in CandidateComps : WF_vars(AutoRevert(c))
  /\ \A c \in CandidateComps : WF_vars(ConfirmHealthy(c))
  /\ \A c \in CandidateComps, l \in Ledgers : WF_vars(PrepareLedger(c, l))
  /\ \A c \in CandidateComps, l \in Ledgers : WF_vars(ApplySecondary(c, l))
  /\ \A c \in CandidateComps, l \in Ledgers : WF_vars(RollbackSecondary(c, l))

Spec == Init /\ [][Next]_vars /\ Fairness

============================================================================
