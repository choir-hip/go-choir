--------------------------- MODULE promotion_protocol ---------------------------
(***************************************************************************)
(* Spec of the Choir mutation/promotion protocol (MutationTransaction).    *)
(*                                                                         *)
(* One promotion of a candidate's changes into canonical state across      *)
(* heterogeneous ledgers (source ref, app/Dolt data, derived index, ...),  *)
(* with an owner-approval gate, a single atomic commit point, a            *)
(* post-commit health window with auto-revert, and concurrent foreground   *)
(* divergence (the active computer keeps living during candidacy).         *)
(*                                                                         *)
(* Design synthesis (research 2026-06-11):                                 *)
(*  - SINGLE COMMIT POINT (Percolator primary lock / route pointer):       *)
(*    the `commit` variable is the linearization point AND the visibility  *)
(*    gate. Secondary ledgers never determine the outcome; they follow it. *)
(*  - PER-LEDGER 2PC SHAPE (Gray/Lamport TwoPhase): each ledger moves      *)
(*    none -> prepared -> applied | rolled_back; prepare is durable and    *)
(*    idempotent, so any reconciler can finish an interrupted promotion    *)
(*    by reading the commit point alone (roll forward or roll back).       *)
(*  - PIVOT (saga theory): Commit is the point of no return. Before it,    *)
(*    backward recovery (Abort + compensation). After it, forward only —   *)
(*    "revert" is a reverse promotion, not a compensation.                 *)
(*  - FRESHNESS CAS (Dolt three-way / foreground tail): the active state   *)
(*    keeps moving during candidacy; Commit requires the foreground tail   *)
(*    to match what the candidate was prepared against, else restage.      *)
(*    >>> Today's Go code RECORDS this and does not ENFORCE it             *)
(*        (ForegroundTailMergeResult; PromoteAppAdoption has no check).    *)
(*        Sabotage run 1 is therefore today's behavior. <<<                *)
(*  - OWNER APPROVAL GATE: Commit requires owner approval after            *)
(*    verification ("review authorizes a verified transition, doesn't      *)
(*    replace verification" — legacy-promotion learnings).                 *)
(*    >>> Today `owner_approved` is a dead status nothing produces;        *)
(*        PromoteAppAdoption fires straight from `verified`.               *)
(*        Sabotage run 2 is therefore today's behavior. <<<                *)
(*  - TRY-THEN-CONFIRM + AUTO-REVERT (Android A/B, ChromeOS): after        *)
(*    Commit, a health window ends in Confirm or AutoRevert.               *)
(*  - N-1 ROLLBACK WINDOW (blue-green "poisoned writes"): once the new     *)
(*    version writes data the old version cannot read, the rollback        *)
(*    window is CLOSED — auto-revert is no longer safe. Reverting anyway   *)
(*    is the torn-rollback / poisoned-write failure (sabotage run 3).      *)
(*                                                                         *)
(* Out of scope v1 (own modules later): multiple concurrent/sequential     *)
(* promotions (serialization), the merge itself (conflict resolution),     *)
(* verifier independence, capsule effect capture.                          *)
(***************************************************************************)

EXTENDS Integers, FiniteSets

CONSTANTS
  Ledgers,      \* secondary ledgers, e.g. {source, data, index}
  MaxTailMoves  \* bound on foreground divergence during candidacy

VARIABLES
  ledger,    \* ledger[l] : "none" | "prepared" | "applied" | "rolled_back"
  commit,    \* the commit point / visibility gate:
             \*   "staging" | "verified" | "approved" | "committed"
             \* | "confirmed" | "aborted" | "reverted"
  tail,      \* foreground active-state version (moves during candidacy)
  base,      \* tail version the candidate is prepared against (-1 = unset)
  poisoned,  \* new version has written data the old version cannot read
  staleCommit, \* history flag: a commit fired against a moved tail
  approved   \* history flag: the owner actually approved this promotion

vars == <<ledger, commit, tail, base, poisoned, staleCommit, approved>>

LedgerStates == {"none", "prepared", "applied", "rolled_back"}
CommitStates == {"staging", "verified", "approved", "committed",
                 "confirmed", "aborted", "reverted"}

CommittedFamily == {"committed", "confirmed"}

TypeOK ==
  /\ ledger \in [Ledgers -> LedgerStates]
  /\ commit \in CommitStates
  /\ tail \in 0..MaxTailMoves
  /\ base \in -1..MaxTailMoves
  /\ poisoned \in BOOLEAN
  /\ staleCommit \in BOOLEAN
  /\ approved \in BOOLEAN

Init ==
  /\ ledger = [l \in Ledgers |-> "none"]
  /\ commit = "staging"
  /\ tail = 0
  /\ base = -1
  /\ poisoned = FALSE
  /\ staleCommit = FALSE
  /\ approved = FALSE

--------------------------------------------------------------------------
(* Candidacy: prepare each ledger durably and idempotently against the    *)
(* current foreground tail. First prepare records the base.               *)

PrepareLedger(l) ==
  /\ commit = "staging"
  /\ ledger[l] = "none"
  /\ ledger' = [ledger EXCEPT ![l] = "prepared"]
  /\ base' = IF base = -1 THEN tail ELSE base
  /\ UNCHANGED <<commit, tail, poisoned, staleCommit, approved>>

(* The foreground keeps living: user/agents mutate active state during    *)
(* candidacy. This is what makes promotion a three-way problem.           *)
TailMove ==
  /\ tail < MaxTailMoves
  /\ commit \in {"staging", "verified", "approved"}
  /\ tail' = tail + 1
  /\ UNCHANGED <<ledger, commit, base, poisoned, staleCommit, approved>>

(* Restage: the tail moved, so re-prepare against the new base. Ledgers   *)
(* drop back to none (idempotent re-prepare); approval/verification are   *)
(* invalidated — evidence about a stale base authorizes nothing.          *)
Restage ==
  /\ commit \in {"staging", "verified", "approved"}
  /\ base # -1 /\ base # tail
  /\ ledger' = [l \in Ledgers |-> "none"]
  /\ base' = tail
  /\ commit' = "staging"
  /\ approved' = FALSE                        \* stale approval is void
  /\ UNCHANGED <<tail, poisoned, staleCommit>>

(* Verifier evidence: all ledgers prepared -> the candidate is verified.  *)
Verify ==
  /\ commit = "staging"
  /\ \A l \in Ledgers : ledger[l] = "prepared"
  /\ commit' = "verified"
  /\ UNCHANGED <<ledger, tail, base, poisoned, staleCommit, approved>>

(* The owner-approval gate. Review authorizes a verified transition; it   *)
(* does not replace verification (and verification does not replace it).  *)
Approve ==
  /\ commit = "verified"
  /\ commit' = "approved"
  /\ approved' = TRUE
  /\ UNCHANGED <<ledger, tail, base, poisoned, staleCommit>>

--------------------------------------------------------------------------
(* THE COMMIT POINT — the pivot, the linearization point, the visibility  *)
(* gate (route pointer flip). Atomic, tiny, and guarded:                  *)
(*   - approved (verification + owner review both happened)               *)
(*   - all ledgers prepared                                               *)
(*   - freshness CAS: the foreground tail has not moved since prepare     *)

Commit ==
  /\ commit = "approved"
  /\ \A l \in Ledgers : ledger[l] = "prepared"
  /\ base = tail                              \* freshness CAS
  /\ commit' = "committed"
  /\ staleCommit' = IF base # tail THEN TRUE ELSE staleCommit
  /\ UNCHANGED <<ledger, tail, base, poisoned, approved>>

(* Pre-pivot abandonment: backward recovery is always safe before commit. *)
Abort ==
  /\ commit \in {"staging", "verified", "approved"}
  /\ commit' = "aborted"
  /\ UNCHANGED <<ledger, tail, base, poisoned, staleCommit, approved>>

--------------------------------------------------------------------------
(* Reconciliation: any secondary's fate is decided by the commit point    *)
(* alone (Percolator's "any reader can finish the commit"). These are     *)
(* the recovery actions after a coordinator crash, too — all state here   *)
(* is durable, so crash is stutter and recovery is just these actions.    *)

ApplySecondary(l) ==
  /\ commit \in CommittedFamily
  /\ ledger[l] = "prepared"
  /\ ledger' = [ledger EXCEPT ![l] = "applied"]
  /\ UNCHANGED <<commit, tail, base, poisoned, staleCommit, approved>>

RollbackSecondary(l) ==
  /\ commit \in {"aborted", "reverted"}
  /\ ledger[l] \in {"prepared", "applied"}
  /\ ledger' = [ledger EXCEPT ![l] = "rolled_back"]
  /\ UNCHANGED <<commit, tail, base, poisoned, staleCommit, approved>>

--------------------------------------------------------------------------
(* Post-commit health window (try-then-confirm).                          *)

(* The promoted version writes data the OLD version cannot read. This is  *)
(* legal — but it closes the rollback window (blue-green N-1 rule).       *)
PoisonedWrite ==
  /\ commit = "committed"
  /\ poisoned' = TRUE
  /\ UNCHANGED <<ledger, commit, tail, base, staleCommit, approved>>

ConfirmHealthy ==
  /\ commit = "committed"
  /\ \A l \in Ledgers : ledger[l] = "applied"   \* fully reconciled first
  /\ commit' = "confirmed"
  /\ UNCHANGED <<ledger, tail, base, poisoned, staleCommit, approved>>

(* Auto-revert on failed health checks — allowed ONLY while the rollback  *)
(* window is open. After a poisoned write, reverting would hand the old   *)
(* version data it cannot read: the torn-rollback failure. A poisoned     *)
(* unhealthy promotion must roll FORWARD (a new corrective promotion),    *)
(* which is outside this spec's single-promotion scope.                   *)
AutoRevert ==
  /\ commit = "committed"
  /\ poisoned = FALSE                          \* rollback window open
  /\ commit' = "reverted"
  /\ UNCHANGED <<ledger, tail, base, poisoned, staleCommit, approved>>

--------------------------------------------------------------------------

Next ==
  \/ \E l \in Ledgers : PrepareLedger(l)
  \/ TailMove
  \/ Restage
  \/ Verify
  \/ Approve
  \/ Commit
  \/ Abort
  \/ \E l \in Ledgers : ApplySecondary(l)
  \/ \E l \in Ledgers : RollbackSecondary(l)
  \/ PoisonedWrite
  \/ ConfirmHealthy
  \/ AutoRevert

(* Runtime obligations: preparing, restaging, verifying, and reconciling  *)
(* keep happening. Approval, aborts, tail moves, health outcomes, and     *)
(* poisoned writes are environment.                                       *)
Fairness ==
  /\ \A l \in Ledgers : WF_vars(PrepareLedger(l))
  /\ WF_vars(Restage)
  /\ WF_vars(Verify)
  /\ \A l \in Ledgers : WF_vars(ApplySecondary(l))
  /\ \A l \in Ledgers : WF_vars(RollbackSecondary(l))

Spec == Init /\ [][Next]_vars /\ Fairness

--------------------------------------------------------------------------
(* INVARIANTS *)

\* 2PC-style uniformity: secondaries only ever follow the commit point.
SecondaryFollowsCommitPoint ==
  \A l \in Ledgers :
    /\ ledger[l] = "applied"
         => commit \in {"committed", "confirmed", "reverted"}
    /\ ledger[l] = "rolled_back"
         => commit \in {"aborted", "reverted"}

\* No torn final states: once the promotion has fully settled
\* (confirmed / aborted / reverted + reconciled), every ledger agrees.
NoTornOutcome ==
  /\ commit = "confirmed" => \A l \in Ledgers : ledger[l] = "applied"
  /\ commit = "aborted"   => \A l \in Ledgers : ledger[l] # "applied"

\* Freshness: no commit ever fired against a moved foreground tail.
\* (Sabotage 1 — today's PromoteAppAdoption — violates this.)
NoStaleCommit == staleCommit = FALSE

\* Visibility gate: nothing ever becomes user-visible (committed-family,
\* or reverted, which implies it was visible) without the owner having
\* actually approved THIS staging of the promotion.
\* (Sabotage 2 — today's PromoteAppAdoption firing straight from
\* `verified`, owner_approved being dead code — violates this.)
ApprovalGate ==
  commit \in (CommittedFamily \cup {"reverted"}) => approved = TRUE

\* Rollback-window safety: a revert never happens after a poisoned write.
\* (Sabotage 3 violates this.)
RevertSafety == commit = "reverted" => poisoned = FALSE

--------------------------------------------------------------------------
(* TEMPORAL PROPERTIES *)

\* The commit point determines the outcome: a committed promotion is
\* eventually fully applied-and-confirmed or reverted; an aborted one is
\* eventually fully rolled back. No promotion hangs half-reconciled.
CommitPointDeterminesOutcome ==
  /\ (commit = "committed") ~>
       (\/ \A l \in Ledgers : ledger[l] = "applied"
        \/ commit = "reverted")
  /\ (commit = "aborted") ~>
       (\A l \in Ledgers : ledger[l] \in {"none", "rolled_back"})
  /\ (commit = "reverted") ~>
       (\A l \in Ledgers : ledger[l] \in {"none", "rolled_back"})

================================================================================
