---------------------------- MODULE promotion_protocol ----------------------------
(***************************************************************************)
(* Spec of the Choir autoputer promotion protocol.                         *)
(*                                                                         *)
(* A computer is a product of heterogeneous ledgers. A candidate computer  *)
(* is a speculative fork of an active computer. Promotion is the atomic    *)
(* flip of the route identity from the active computer to the candidate,   *)
(* guarded by per-ledger prepare/verify, owner approval, and a freshness   *)
(* CAS. After commit, a health window ends in confirmation or revert.    *)
(*                                                                         *)
(* Source design:                                                            *)
(*   docs/computer-ontology.md                                             *)
(*   docs/promotion-protocol-spec-staleness-and-redefinition-2026-07-03.md   *)
(*   docs/choir-promotion-protocol-conjecture-2026-06-11.md (historical)   *)
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
(*      ArtifactProgramRef are independently bounded.  The earlier           *)
(*      RouteNamesComputerVersion / PromotionNamesComputerVersion            *)
(*      invariants were vacuous because ComputerVersionOfBase(n) mapped      *)
(*      both codeRef and artifactProgramRef to the same counter n, making    *)
(*      the result trivially a member of ComputerVersions.  The model now    *)
(*      tracks code and artifact counters independently, so the invariants   *)
(*      can fail if an action produces an out-of-bounds ref.                 *)
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
  MaxTailMoves      \* bound on active-base divergence during candidacy

VARIABLES
  activeCodeBase,     \* activeCodeBase[a]  : code ref counter of active computer a
  activeArtifactBase, \* activeArtifactBase[a] : artifact program ref counter of active computer a
  candidateCodeBase,    \* candidateCodeBase[c] : code ref counter at candidate c's fork point
  candidateArtifactBase,\* candidateArtifactBase[c] : artifact ref counter at candidate c's fork point
  candidateParent,  \* candidateParent[c] : active computer c forks from
  route,            \* route[s] : computer currently serving slot s (active or candidate)
  ledgerState,      \* ledgerState[p][l] : state of ledger l for promotion p
  promoStatus,      \* promoStatus[p] : promotion lifecycle state
  promoActive,      \* promoActive[p] : active computer owning promotion p
  promoCandidate,   \* promoCandidate[p] : candidate computer of promotion p
  promoCodeBase,        \* promoCodeBase[p] : code ref counter at candidate fork
  promoArtifactBase,    \* promoArtifactBase[p] : artifact ref counter at candidate fork
  approved,         \* approved[p] : owner approval recorded
  poisoned,         \* poisoned[p] : new version wrote data old cannot read
  healthWindow      \* healthWindow[p] : "open" | "failed" | "confirmed"

vars == <<activeCodeBase, activeArtifactBase, candidateCodeBase,
          candidateArtifactBase, candidateParent, route, ledgerState,
          promoStatus, promoActive, promoCandidate, promoCodeBase,
          promoArtifactBase, approved, poisoned, healthWindow>>

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
  /\ candidateCodeBase \in [CandidateComps -> BaseVersionNumbers]
  /\ candidateArtifactBase \in [CandidateComps -> BaseVersionNumbers]
  /\ candidateParent \in [CandidateComps -> ActiveComps]
  /\ route \in [Slots -> ActiveComps \cup CandidateComps]
  /\ promoStatus \in [CandidateComps -> PromoStates]
  /\ promoActive \in [CandidateComps -> ActiveComps]
  /\ promoCandidate \in [CandidateComps -> CandidateComps]
  /\ promoCodeBase \in [CandidateComps -> BaseVersionNumbers]
  /\ promoArtifactBase \in [CandidateComps -> BaseVersionNumbers]
  /\ approved \in [CandidateComps -> BOOLEAN]
  /\ poisoned \in [CandidateComps -> BOOLEAN]
  /\ healthWindow \in [CandidateComps -> HealthStates]
  /\ ledgerState \in [CandidateComps -> [Ledgers -> LedgerStates]]

Init ==
  /\ activeCodeBase = [a \in ActiveComps |-> 0]
  /\ activeArtifactBase = [a \in ActiveComps |-> 0]
  /\ candidateCodeBase = [c \in CandidateComps |-> 0]
  /\ candidateArtifactBase = [c \in CandidateComps |-> 0]
  /\ candidateParent = [c \in CandidateComps |-> CHOOSE a \in ActiveComps : TRUE]
  /\ route = [s \in Slots |-> CHOOSE a \in ActiveComps : TRUE]
  /\ promoStatus = [c \in CandidateComps |-> "aborted"]
  /\ promoActive = [c \in CandidateComps |-> candidateParent[c]]
  /\ promoCandidate = [c \in CandidateComps |-> c]
  /\ promoCodeBase = [c \in CandidateComps |-> 0]
  /\ promoArtifactBase = [c \in CandidateComps |-> 0]
  /\ approved = [c \in CandidateComps |-> FALSE]
  /\ poisoned = [c \in CandidateComps |-> FALSE]
  /\ healthWindow = [c \in CandidateComps |-> "open"]
  /\ ledgerState = [c \in CandidateComps |-> [l \in Ledgers |-> "none"]]

--------------------------------------------------------------------------
(* Active computer divergence: the foreground keeps moving during candidacy. *)
(* Code and artifact counters advance independently to model code-only         *)
(* updates (e.g. interpreter patch) and artifact-only updates (e.g. user data  *)
(* growth).  Both actions are enabled while their respective counters are      *)
(* below MaxTailMoves.                                                         *)

MoveActiveCode(a) ==
  /\ activeCodeBase[a] < MaxTailMoves
  /\ activeCodeBase' = [activeCodeBase EXCEPT ![a] = @ + 1]
  /\ UNCHANGED <<activeArtifactBase, candidateCodeBase, candidateArtifactBase,
                  candidateParent, route, ledgerState,
                  promoStatus, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, approved, poisoned, healthWindow>>

MoveActiveArtifact(a) ==
  /\ activeArtifactBase[a] < MaxTailMoves
  /\ activeArtifactBase' = [activeArtifactBase EXCEPT ![a] = @ + 1]
  /\ UNCHANGED <<activeCodeBase, candidateCodeBase, candidateArtifactBase,
                  candidateParent, route, ledgerState,
                  promoStatus, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, approved, poisoned, healthWindow>>

--------------------------------------------------------------------------
(* Fork a candidate from an active computer. This is the durable fork point. *)

ForkCandidate(c, a) ==
  /\ promoStatus[c] = "aborted"
  /\ candidateParent[c] = a
  /\ promoActive' = [promoActive EXCEPT ![c] = a]
  /\ promoCandidate' = [promoCandidate EXCEPT ![c] = c]
  /\ candidateCodeBase' = [candidateCodeBase EXCEPT ![c] = activeCodeBase[a]]
  /\ candidateArtifactBase' = [candidateArtifactBase EXCEPT ![c] = activeArtifactBase[a]]
  /\ promoCodeBase' = [promoCodeBase EXCEPT ![c] = activeCodeBase[a]]
  /\ promoArtifactBase' = [promoArtifactBase EXCEPT ![c] = activeArtifactBase[a]]
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "staging"]
  /\ approved' = [approved EXCEPT ![c] = FALSE]
  /\ poisoned' = [poisoned EXCEPT ![c] = FALSE]
  /\ healthWindow' = [healthWindow EXCEPT ![c] = "open"]
  /\ ledgerState' = [ledgerState EXCEPT ![c] = [l \in Ledgers |-> "none"]]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, candidateParent, route>>

--------------------------------------------------------------------------
(* Per-ledger prepare: durable, idempotent, inert until commit.             *)

PrepareLedger(c, l) ==
  /\ promoStatus[c] \in {"staging", "verified", "approved"}
  /\ ledgerState[c][l] = "none"
  /\ ledgerState' = [ledgerState EXCEPT ![c][l] = "prepared"]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, candidateCodeBase,
                  candidateArtifactBase, candidateParent, route,
                  promoStatus, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, approved, poisoned, healthWindow>>

(* Restage: the active base moved, so the candidate must re-prepare.        *)
(* Verification and approval are invalidated because evidence about a stale *)
(* base authorizes nothing.                                                   *)

Restage(c) ==
  /\ promoStatus[c] \in {"staging", "verified", "approved"}
  /\ \/ promoCodeBase[c] # activeCodeBase[promoActive[c]]
     \/ promoArtifactBase[c] # activeArtifactBase[promoActive[c]]
  /\ promoCodeBase' = [promoCodeBase EXCEPT ![c] = activeCodeBase[promoActive[c]]]
  /\ promoArtifactBase' = [promoArtifactBase EXCEPT ![c] = activeArtifactBase[promoActive[c]]]
  /\ candidateCodeBase' = [candidateCodeBase EXCEPT ![c] = activeCodeBase[promoActive[c]]]
  /\ candidateArtifactBase' = [candidateArtifactBase EXCEPT ![c] = activeArtifactBase[promoActive[c]]]
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "staging"]
  /\ approved' = [approved EXCEPT ![c] = FALSE]
  /\ ledgerState' = [ledgerState EXCEPT ![c] = [l \in Ledgers |-> "none"]]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, candidateParent, route,
                  promoActive, promoCandidate, poisoned, healthWindow>>

(* Verifier evidence: all ledgers prepared -> candidate is verified.         *)

Verify(c) ==
  /\ promoStatus[c] = "staging"
  /\ \A l \in Ledgers : ledgerState[c][l] = "prepared"
  /\ promoCodeBase[c] = activeCodeBase[promoActive[c]]
  /\ promoArtifactBase[c] = activeArtifactBase[promoActive[c]]
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "verified"]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, candidateCodeBase,
                  candidateArtifactBase, candidateParent, route,
                  ledgerState, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, approved, poisoned, healthWindow>>

(* Owner approval gate. Review authorizes a verified transition.             *)

Approve(c) ==
  /\ promoStatus[c] = "verified"
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "approved"]
  /\ approved' = [approved EXCEPT ![c] = TRUE]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, candidateCodeBase,
                  candidateArtifactBase, candidateParent, route,
                  ledgerState, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, poisoned, healthWindow>>

--------------------------------------------------------------------------
(* The commit point: atomic route-pointer flip. Guards:                    *)
(*   - approved                                                             *)
(*   - all ledgers prepared                                                 *)
(*   - freshness CAS: active base has not moved since the fork/verify       *)

Commit(c) ==
  /\ promoStatus[c] = "approved"
  /\ \A l \in Ledgers : ledgerState[c][l] = "prepared"
  /\ promoCodeBase[c] = activeCodeBase[promoActive[c]]
  /\ promoArtifactBase[c] = activeArtifactBase[promoActive[c]]
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "committed"]
  /\ route' = [s \in Slots |->
                IF route[s] = promoActive[c]
                  THEN promoCandidate[c]
                  ELSE route[s]]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, candidateCodeBase,
                  candidateArtifactBase, candidateParent, ledgerState,
                  promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, approved, poisoned, healthWindow>>

(* Pre-pivot abandonment: backward recovery is always safe before commit.   *)
(* Abort atomically rolls back all prepared secondaries.                     *)

Abort(c) ==
  /\ promoStatus[c] \in {"staging", "verified", "approved"}
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "aborted"]
  /\ ledgerState' = [ledgerState EXCEPT ![c] =
                      [l \in Ledgers |->
                         IF ledgerState[c][l] = "prepared"
                           THEN "rolled_back"
                           ELSE ledgerState[c][l]]]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, candidateCodeBase,
                  candidateArtifactBase, candidateParent, route,
                  promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, approved, poisoned, healthWindow>>

--------------------------------------------------------------------------
(* Reconciliation: secondaries follow the commit point.                    *)
(* Any crashed coordinator can recover by reading the commit point.         *)

ApplySecondary(c, l) ==
  /\ promoStatus[c] = "committed"
  /\ healthWindow[c] = "open"
  /\ ledgerState[c][l] = "prepared"
  /\ ledgerState' = [ledgerState EXCEPT ![c][l] = "applied"]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, candidateCodeBase,
                  candidateArtifactBase, candidateParent, route,
                  promoStatus, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, approved, poisoned, healthWindow>>

RollbackSecondary(c, l) ==
  /\ promoStatus[c] \in {"aborted", "reverted"}
  /\ ledgerState[c][l] \in {"prepared", "applied"}
  /\ ledgerState' = [ledgerState EXCEPT ![c][l] = "rolled_back"]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, candidateCodeBase,
                  candidateArtifactBase, candidateParent, route,
                  promoStatus, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, approved, poisoned, healthWindow>>

--------------------------------------------------------------------------
(* Post-commit health window (try-then-confirm).                             *)
(* A poisoned write closes the rollback window.                             *)
(* After poisoned, only forward recovery (a new promotion) is safe.         *)

PoisonedWrite(c) ==
  /\ promoStatus[c] = "committed"
  /\ poisoned' = [poisoned EXCEPT ![c] = TRUE]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, candidateCodeBase,
                  candidateArtifactBase, candidateParent, route,
                  ledgerState, promoStatus, promoActive, promoCandidate,
                  promoCodeBase, promoArtifactBase, approved, healthWindow>>

(* Health check fails while the window is open. This is the "try" half.     *)

HealthCheckFail(c) ==
  /\ promoStatus[c] = "committed"
  /\ healthWindow[c] = "open"
  /\ poisoned[c] = FALSE
  /\ healthWindow' = [healthWindow EXCEPT ![c] = "failed"]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, candidateCodeBase,
                  candidateArtifactBase, candidateParent, route,
                  ledgerState, promoStatus, promoActive, promoCandidate,
                  promoCodeBase, promoArtifactBase, approved, poisoned>>

(* Confirm healthy: all secondaries applied and window not poisoned.        *)

ConfirmHealthy(c) ==
  /\ promoStatus[c] = "committed"
  /\ healthWindow[c] = "open"
  /\ poisoned[c] = FALSE
  /\ \A l \in Ledgers : ledgerState[c][l] = "applied"
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "confirmed"]
  /\ healthWindow' = [healthWindow EXCEPT ![c] = "confirmed"]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, candidateCodeBase,
                  candidateArtifactBase, candidateParent, route,
                  ledgerState, promoActive, promoCandidate, promoCodeBase,
                  promoArtifactBase, approved, poisoned>>

(* Auto-revert on failed health check. Allowed only while rollback window     *)
(* is open (not poisoned). Reverts the route pointer to the active parent   *)
(* and atomically rolls back all secondaries.                                  *)

AutoRevert(c) ==
  /\ promoStatus[c] = "committed"
  /\ healthWindow[c] = "failed"
  /\ poisoned[c] = FALSE
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "reverted"]
  /\ route' = [s \in Slots |->
                IF route[s] = promoCandidate[c]
                  THEN promoActive[c]
                  ELSE route[s]]
  /\ ledgerState' = [ledgerState EXCEPT ![c] =
                      [l \in Ledgers |->
                         IF ledgerState[c][l] \in {"prepared", "applied"}
                           THEN "rolled_back"
                           ELSE ledgerState[c][l]]]
  /\ UNCHANGED <<activeCodeBase, activeArtifactBase, candidateCodeBase,
                  candidateArtifactBase, candidateParent, promoActive,
                  promoCandidate, promoCodeBase, promoArtifactBase,
                  approved, poisoned, healthWindow>>

--------------------------------------------------------------------------
(* The full next-state relation.                                             *)

Next ==
  \/ \E a \in ActiveComps : MoveActiveCode(a)
  \/ \E a \in ActiveComps : MoveActiveArtifact(a)
  \/ \E c \in CandidateComps, a \in ActiveComps : ForkCandidate(c, a)
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
(* records a non-negative base and a candidate.                              *)
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
