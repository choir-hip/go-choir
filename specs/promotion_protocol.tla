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
(*                          candidate was prepared and verified.          *)
(*   2. ApprovalGate      — no commit without explicit owner approval.      *)
(*   3. NoTornOutcome     — settled promotions are uniform across ledgers.  *)
(*   4. RouteConsistency  — route points to exactly one committed computer. *)
(*   5. HealthWindowReversible — revert only while rollback window is open. *)
(*   6. CandidateIsolation — candidate mutations are not route-visible      *)
(*                          before commit.                                  *)
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
  activeBase,       \* activeBase[a]  : version of active computer a
  candidateBase,    \* candidateBase[c] : version of candidate c's fork point
  candidateParent,  \* candidateParent[c] : active computer c forks from
  route,            \* route[s] : computer currently serving slot s (active or candidate)
  ledgerState,      \* ledgerState[p][l] : state of ledger l for promotion p
  promoStatus,      \* promoStatus[p] : promotion lifecycle state
  promoActive,      \* promoActive[p] : active computer owning promotion p
  promoCandidate,   \* promoCandidate[p] : candidate computer of promotion p
  promoBase,        \* promoBase[p] : active base version at candidate fork
  approved,         \* approved[p] : owner approval recorded
  poisoned,         \* poisoned[p] : new version wrote data old cannot read
  healthWindow      \* healthWindow[p] : "open" | "failed" | "confirmed"

vars == <<activeBase, candidateBase, candidateParent, route, ledgerState,
          promoStatus, promoActive, promoCandidate, promoBase, approved,
          poisoned, healthWindow>>

LedgerStates == {"none", "prepared", "applied", "rolled_back"}
PromoStates  == {"staging", "verified", "approved", "committed",
                 "confirmed", "aborted", "reverted"}
HealthStates == {"open", "failed", "confirmed"}

\* A promotion is "settled" if it has reached a terminal state.
TerminalStates == {"aborted", "confirmed", "reverted"}

\* A promotion is "committed family" if it has passed the point of no return.
CommittedFamily == {"committed", "confirmed", "reverted"}

TypeOK ==
  /\ activeBase \in [ActiveComps -> 0..MaxTailMoves]
  /\ candidateBase \in [CandidateComps -> 0..MaxTailMoves]
  /\ candidateParent \in [CandidateComps -> ActiveComps]
  /\ route \in [Slots -> ActiveComps \cup CandidateComps]
  /\ promoStatus \in [CandidateComps -> PromoStates]
  /\ promoActive \in [CandidateComps -> ActiveComps]
  /\ promoCandidate \in [CandidateComps -> CandidateComps]
  /\ promoBase \in [CandidateComps -> 0..MaxTailMoves]
  /\ approved \in [CandidateComps -> BOOLEAN]
  /\ poisoned \in [CandidateComps -> BOOLEAN]
  /\ healthWindow \in [CandidateComps -> HealthStates]
  /\ ledgerState \in [CandidateComps -> [Ledgers -> LedgerStates]]

Init ==
  /\ activeBase = [a \in ActiveComps |-> 0]
  /\ candidateBase = [c \in CandidateComps |-> 0]
  /\ candidateParent = [c \in CandidateComps |-> CHOOSE a \in ActiveComps : TRUE]
  /\ route = [s \in Slots |-> CHOOSE a \in ActiveComps : TRUE]
  /\ promoStatus = [c \in CandidateComps |-> "aborted"]
  /\ promoActive = [c \in CandidateComps |-> candidateParent[c]]
  /\ promoCandidate = [c \in CandidateComps |-> c]
  /\ promoBase = [c \in CandidateComps |-> 0]
  /\ approved = [c \in CandidateComps |-> FALSE]
  /\ poisoned = [c \in CandidateComps |-> FALSE]
  /\ healthWindow = [c \in CandidateComps |-> "open"]
  /\ ledgerState = [c \in CandidateComps |-> [l \in Ledgers |-> "none"]]

--------------------------------------------------------------------------
(* Active computer divergence: the foreground keeps moving during candidacy. *)

MoveActiveTail(a) ==
  /\ activeBase[a] < MaxTailMoves
  /\ activeBase' = [activeBase EXCEPT ![a] = @ + 1]
  /\ UNCHANGED <<candidateBase, candidateParent, route, ledgerState,
                  promoStatus, promoActive, promoCandidate, promoBase,
                  approved, poisoned, healthWindow>>

--------------------------------------------------------------------------
(* Fork a candidate from an active computer. This is the durable fork point. *)

ForkCandidate(c, a) ==
  /\ promoStatus[c] = "aborted"
  /\ candidateParent[c] = a
  /\ promoActive' = [promoActive EXCEPT ![c] = a]
  /\ promoCandidate' = [promoCandidate EXCEPT ![c] = c]
  /\ candidateBase' = [candidateBase EXCEPT ![c] = activeBase[a]]
  /\ promoBase' = [promoBase EXCEPT ![c] = activeBase[a]]
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "staging"]
  /\ approved' = [approved EXCEPT ![c] = FALSE]
  /\ poisoned' = [poisoned EXCEPT ![c] = FALSE]
  /\ healthWindow' = [healthWindow EXCEPT ![c] = "open"]
  /\ ledgerState' = [ledgerState EXCEPT ![c] = [l \in Ledgers |-> "none"]]
  /\ UNCHANGED <<activeBase, candidateParent, route>>

--------------------------------------------------------------------------
(* Per-ledger prepare: durable, idempotent, inert until commit.             *)

PrepareLedger(c, l) ==
  /\ promoStatus[c] \in {"staging", "verified", "approved"}
  /\ ledgerState[c][l] = "none"
  /\ ledgerState' = [ledgerState EXCEPT ![c][l] = "prepared"]
  /\ UNCHANGED <<activeBase, candidateBase, candidateParent, route,
                  promoStatus, promoActive, promoCandidate, promoBase,
                  approved, poisoned, healthWindow>>

(* Restage: the active base moved, so the candidate must re-prepare.        *)
(* Verification and approval are invalidated because evidence about a stale *)
(* base authorizes nothing.                                                   *)

Restage(c) ==
  /\ promoStatus[c] \in {"staging", "verified", "approved"}
  /\ promoBase[c] # activeBase[promoActive[c]]
  /\ promoBase' = [promoBase EXCEPT ![c] = activeBase[promoActive[c]]]
  /\ candidateBase' = [candidateBase EXCEPT ![c] = activeBase[promoActive[c]]]
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "staging"]
  /\ approved' = [approved EXCEPT ![c] = FALSE]
  /\ ledgerState' = [ledgerState EXCEPT ![c] = [l \in Ledgers |-> "none"]]
  /\ UNCHANGED <<activeBase, candidateParent, route, promoActive,
                  promoCandidate, poisoned, healthWindow>>

(* Verifier evidence: all ledgers prepared -> candidate is verified.         *)

Verify(c) ==
  /\ promoStatus[c] = "staging"
  /\ \A l \in Ledgers : ledgerState[c][l] = "prepared"
  /\ promoBase[c] = activeBase[promoActive[c]]
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "verified"]
  /\ UNCHANGED <<activeBase, candidateBase, candidateParent, route,
                  ledgerState, promoActive, promoCandidate, promoBase,
                  approved, poisoned, healthWindow>>

(* Owner approval gate. Review authorizes a verified transition.             *)

Approve(c) ==
  /\ promoStatus[c] = "verified"
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "approved"]
  /\ approved' = [approved EXCEPT ![c] = TRUE]
  /\ UNCHANGED <<activeBase, candidateBase, candidateParent, route,
                  ledgerState, promoActive, promoCandidate, promoBase,
                  poisoned, healthWindow>>

--------------------------------------------------------------------------
(* The commit point: atomic route-pointer flip. Guards:                    *)
(*   - approved                                                             *)
(*   - all ledgers prepared                                                 *)
(*   - freshness CAS: active base has not moved since the fork/verify       *)

Commit(c) ==
  /\ promoStatus[c] = "approved"
  /\ \A l \in Ledgers : ledgerState[c][l] = "prepared"
  /\ promoBase[c] = activeBase[promoActive[c]]
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "committed"]
  /\ route' = [s \in Slots |->
                IF route[s] = promoActive[c]
                  THEN promoCandidate[c]
                  ELSE route[s]]
  /\ UNCHANGED <<activeBase, candidateBase, candidateParent, ledgerState,
                  promoActive, promoCandidate, promoBase, approved,
                  poisoned, healthWindow>>

(* Pre-pivot abandonment: backward recovery is always safe before commit.   *)

Abort(c) ==
  /\ promoStatus[c] \in {"staging", "verified", "approved"}
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "aborted"]
  /\ UNCHANGED <<activeBase, candidateBase, candidateParent, route,
                  ledgerState, promoActive, promoCandidate, promoBase,
                  approved, poisoned, healthWindow>>

--------------------------------------------------------------------------
(* Reconciliation: secondaries follow the commit point.                    *)
(* Any crashed coordinator can recover by reading the commit point.         *)

ApplySecondary(c, l) ==
  /\ promoStatus[c] = "committed"
  /\ healthWindow[c] = "open"
  /\ ledgerState[c][l] = "prepared"
  /\ ledgerState' = [ledgerState EXCEPT ![c][l] = "applied"]
  /\ UNCHANGED <<activeBase, candidateBase, candidateParent, route,
                  promoStatus, promoActive, promoCandidate, promoBase,
                  approved, poisoned, healthWindow>>

RollbackSecondary(c, l) ==
  /\ promoStatus[c] \in {"aborted", "reverted"}
  /\ ledgerState[c][l] \in {"prepared", "applied"}
  /\ ledgerState' = [ledgerState EXCEPT ![c][l] = "rolled_back"]
  /\ UNCHANGED <<activeBase, candidateBase, candidateParent, route,
                  promoStatus, promoActive, promoCandidate, promoBase,
                  approved, poisoned, healthWindow>>

--------------------------------------------------------------------------
(* Post-commit health window (try-then-confirm).                             *)
(* A poisoned write closes the rollback window.                             *)
(* After poisoned, only forward recovery (a new promotion) is safe.         *)

PoisonedWrite(c) ==
  /\ promoStatus[c] = "committed"
  /\ poisoned' = [poisoned EXCEPT ![c] = TRUE]
  /\ UNCHANGED <<activeBase, candidateBase, candidateParent, route,
                  ledgerState, promoStatus, promoActive, promoCandidate,
                  promoBase, approved, healthWindow>>

(* Health check fails while the window is open. This is the "try" half.     *)

HealthCheckFail(c) ==
  /\ promoStatus[c] = "committed"
  /\ healthWindow[c] = "open"
  /\ poisoned[c] = FALSE
  /\ healthWindow' = [healthWindow EXCEPT ![c] = "failed"]
  /\ UNCHANGED <<activeBase, candidateBase, candidateParent, route,
                  ledgerState, promoStatus, promoActive, promoCandidate,
                  promoBase, approved, poisoned>>

(* Confirm healthy: all secondaries applied and window not poisoned.        *)

ConfirmHealthy(c) ==
  /\ promoStatus[c] = "committed"
  /\ healthWindow[c] = "open"
  /\ poisoned[c] = FALSE
  /\ \A l \in Ledgers : ledgerState[c][l] = "applied"
  /\ promoStatus' = [promoStatus EXCEPT ![c] = "confirmed"]
  /\ healthWindow' = [healthWindow EXCEPT ![c] = "confirmed"]
  /\ UNCHANGED <<activeBase, candidateBase, candidateParent, route,
                  ledgerState, promoActive, promoCandidate, promoBase,
                  approved, poisoned>>

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
  /\ UNCHANGED <<activeBase, candidateBase, candidateParent, promoActive,
                  promoCandidate, promoBase, approved, poisoned, healthWindow>>

--------------------------------------------------------------------------
(* The full next-state relation.                                             *)

Next ==
  \/ \E a \in ActiveComps : MoveActiveTail(a)
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
(* at the moment of commit.                                                   *)
NoStaleCommit ==
  \A c \in CandidateComps :
    promoStatus[c] \in CommittedFamily
      => promoBase[c] = activeBase[promoActive[c]]

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
      => promoBase[c] >= 0 /\ promoCandidate[c] = c

--------------------------------------------------------------------------
(* Liveness: what must eventually happen.                                   *)
(* Every promotion eventually reaches a terminal state.                     *)
(* We use weak fairness on the key actions to ensure progress.              *)

(* A committed promotion that never becomes poisoned eventually reaches a     *)
(* terminal state: confirmed or reverted. After a poisoned write, only         *)
(* forward recovery (a new promotion) is safe and is outside this single-       *)
(* promotion model.                                                            *)
EveryCommittedPromotionSettles ==
  \A c \in CandidateComps :
    (promoStatus[c] = "committed" /\ poisoned[c] = FALSE)
      ~> promoStatus[c] \in {"confirmed", "reverted"}

(* A promotion in staging/verified/approved will not be blocked forever by     *)
(* system inaction alone. The owner may still choose not to approve, but the   *)
(* system must make progress on prepare/verify/restage when enabled.        *)
SystemProgress ==
  \A c \in CandidateComps :
    (promoStatus[c] = "staging" /\ promoBase[c] = activeBase[promoActive[c]])
      ~> (promoStatus[c] \in {"verified", "approved", TerminalStates})

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
