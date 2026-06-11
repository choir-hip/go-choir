---------------------------- MODULE actor_protocol ----------------------------
(***************************************************************************)
(* Spec v1 of the Choir durable-actor messaging protocol.                  *)
(*                                                                         *)
(* Models: send / activate-on-send / deliver (cold wake + warm steer) /    *)
(* process / passivate / evict / crash / boot sweep.                       *)
(*                                                                         *)
(* Source design: docs/choir-rearchitecture-durable-actors-2026-06-11.md   *)
(* and the agreed pseudocode walkthrough (2026-06-11).                     *)
(*                                                                         *)
(* Semantics being checked:                                                *)
(*   - at-least-once visibility, exactly-once ledger effects               *)
(*     (ledger effect == durable log append; the log is a set, so          *)
(*      idempotent dedup by update id is by construction here — the Go     *)
(*      implementation must dedup on update_id to match)                   *)
(*   - no lost wake: every logged update is eventually processed           *)
(*   - atomic passivation: an actor may only leave residency when it has   *)
(*     no unprocessed backlog (the idle check and the mailbox check are    *)
(*     one atomic step here; the implementation must hold the registry     *)
(*     lock across {residency check + mailbox send} on the send side and   *)
(*     {idle check + deregister} on the passivate side to honor this)      *)
(*   - eviction (forced passivation) == single-agent crash: safe with     *)
(*     zero new machinery, because the sweep re-activates. Eviction        *)
(*     covers memory pressure, shutdown, and any future lease policy —    *)
(*     leases are deliberately NOT a v1 concept (deferred until service-  *)
(*     tier pricing requirements define their semantics).                 *)
(*                                                                         *)
(* Design dividend already found while writing this spec: unbounded       *)
(* evict/sweep cycles without a Process step are a genuine livelock —      *)
(* liveness only holds because evictions and crashes are BOUNDED. In the   *)
(* implementation that bound is the per-owner activation cap. If the       *)
(* cap is ever removed, EventuallyProcessed fails. (Check it: set          *)
(* MaxEvictions very high and weaken fairness — TLC shows the loop.)       *)
(***************************************************************************)

EXTENDS Naturals, FiniteSets

CONSTANTS
  Agents,       \* agent identities, e.g. {a1, a2}
  Updates,      \* update ids, e.g. {u1, u2, u3}
  MaxCrashes,   \* bound on whole-process crashes (boot sweep follows each)
  MaxEvictions  \* bound on forced passivations (impl: per-owner activation cap)

VARIABLES
  log,        \* log[a]       : set of updates durably appended for agent a
  processed,  \* processed[a] : subset of log[a] durably marked incorporated
  resident,   \* set of agents currently holding a live goroutine (volatile)
  mailbox,    \* mailbox[a]   : updates handed to the live loop (volatile)
  crashes,    \* number of crashes so far
  evictions   \* number of forced passivations so far

vars == <<log, processed, resident, mailbox, crashes, evictions>>

AllLogged == UNION {log[a] : a \in Agents}

TypeOK ==
  /\ log \in [Agents -> SUBSET Updates]
  /\ processed \in [Agents -> SUBSET Updates]
  /\ resident \subseteq Agents
  /\ mailbox \in [Agents -> SUBSET Updates]
  /\ crashes \in 0..MaxCrashes
  /\ evictions \in 0..MaxEvictions

Init ==
  /\ log = [a \in Agents |-> {}]
  /\ processed = [a \in Agents |-> {}]
  /\ resident = {}
  /\ mailbox = [a \in Agents |-> {}]
  /\ crashes = 0
  /\ evictions = 0

(***************************************************************************)
(* Send: durable append (the ledger effect commits here, exactly once),    *)
(* then deliver — into the mailbox if the actor is warm (steering), or by  *)
(* activating it if cold (the Orleans move). Atomic w.r.t. passivation.    *)
(* A resend of an already-logged update id is a no-op in the               *)
(* implementation; here updates are fresh by construction (sets dedup).    *)
(***************************************************************************)
Send(u, a) ==
  /\ u \notin AllLogged                       \* fresh update id
  /\ log' = [log EXCEPT ![a] = @ \cup {u}]
  /\ IF a \in resident
       THEN /\ mailbox' = [mailbox EXCEPT ![a] = @ \cup {u}]   \* warm: steer
            /\ UNCHANGED resident
       ELSE /\ resident' = resident \cup {a}                   \* cold: wake
            /\ UNCHANGED mailbox    \* backlog replay covers delivery
  /\ UNCHANGED <<processed, crashes, evictions>>

(***************************************************************************)
(* Sweep: an unresident agent with unprocessed backlog is eligible for     *)
(* (re)activation. This single rule covers boot recovery after a crash,    *)
(* re-wake after eviction, and the crash window where a send appended      *)
(* to the log but died before delivery.                                    *)
(***************************************************************************)
Sweep(a) ==
  /\ a \notin resident
  /\ log[a] \ processed[a] # {}
  /\ resident' = resident \cup {a}
  /\ UNCHANGED <<log, processed, mailbox, crashes, evictions>>

(***************************************************************************)
(* Process: a resident actor incorporates one unprocessed update and       *)
(* durably marks it. A crash after incorporation but before this mark      *)
(* replays the update — that IS the accepted at-least-once visibility.     *)
(* (LLM work inside the activation is abstracted into this single step.)   *)
(***************************************************************************)
Process(a, u) ==
  /\ a \in resident
  /\ u \in log[a] \ processed[a]
  /\ processed' = [processed EXCEPT ![a] = @ \cup {u}]
  /\ mailbox' = [mailbox EXCEPT ![a] = @ \ {u}]
  /\ UNCHANGED <<log, resident, crashes, evictions>>

(***************************************************************************)
(* Passivate: graceful exit. The guard is the atomic idle check — no       *)
(* unprocessed backlog may exist. (Checking only the mailbox would be the  *)
(* lost-wake bug: a cold-activated actor has backlog but an empty          *)
(* mailbox. Weaken the guard to mailbox[a] = {} and liveness fails.)       *)
(***************************************************************************)
Passivate(a) ==
  /\ a \in resident
  /\ log[a] \ processed[a] = {}
  /\ resident' = resident \ {a}
  /\ mailbox' = [mailbox EXCEPT ![a] = {}]
  /\ UNCHANGED <<log, processed, crashes, evictions>>

(***************************************************************************)
(* Evict: forced passivation without the idle guard — memory pressure,     *)
(* shutdown, or any future lease policy. Deliberately identical to a       *)
(* single-agent crash. Safe because Sweep re-activates; bounded because    *)
(* unbounded eviction without progress is a livelock (impl: per-owner      *)
(* activation caps).                                                       *)
(***************************************************************************)
Evict(a) ==
  /\ a \in resident
  /\ evictions < MaxEvictions
  /\ resident' = resident \ {a}
  /\ mailbox' = [mailbox EXCEPT ![a] = {}]
  /\ evictions' = evictions + 1
  /\ UNCHANGED <<log, processed, crashes>>

(***************************************************************************)
(* Crash: the whole process dies. All volatile state vanishes; durable     *)
(* state survives. Recovery is just Sweep becoming enabled.                *)
(***************************************************************************)
Crash ==
  /\ crashes < MaxCrashes
  /\ resident' = {}
  /\ mailbox' = [a \in Agents |-> {}]
  /\ crashes' = crashes + 1
  /\ UNCHANGED <<log, processed, evictions>>

Next ==
  \/ \E u \in Updates, a \in Agents : Send(u, a)
  \/ \E a \in Agents : Sweep(a)
  \/ \E a \in Agents, u \in Updates : Process(a, u)
  \/ \E a \in Agents : Passivate(a)
  \/ \E a \in Agents : Evict(a)
  \/ Crash

(***************************************************************************)
(* Fairness: the runtime must keep scheduling Process and Sweep — these    *)
(* are the implementation's obligations (the actor loop runs; the sweep    *)
(* rule fires). Sends, passivations, evictions, crashes are environment.   *)
(***************************************************************************)
Fairness ==
  /\ \A a \in Agents : \A u \in Updates : WF_vars(Process(a, u))
  /\ \A a \in Agents : WF_vars(Sweep(a))

Spec == Init /\ [][Next]_vars /\ Fairness

--------------------------------------------------------------------------
(* INVARIANTS (safety: true in every reachable state) *)

\* durable marks never reference unlogged updates
ProcessedSubsetLog == \A a \in Agents : processed[a] \subseteq log[a]

\* nothing lives only in a mailbox, and nothing already processed is
\* redelivered through it: the mailbox is a vehicle, never the truth
MailboxSound == \A a \in Agents : mailbox[a] \subseteq (log[a] \ processed[a])

\* only resident actors hold mailbox contents
ColdMailboxEmpty == \A a \in Agents : a \notin resident => mailbox[a] = {}

--------------------------------------------------------------------------
(* TEMPORAL PROPERTIES (liveness: something good eventually happens) *)

\* THE no-lost-wake property: every durably appended update is eventually
\* durably incorporated, despite crashes, evictions, and passivations
EventuallyProcessed ==
  \A a \in Agents : \A u \in Updates :
    (u \in log[a]) ~> (u \in processed[a])

\* the system quiesces: once work exists it is eventually all done and
\* every actor eventually passivates (no zombie residency)
EventuallyQuiescent ==
  <>[](\A a \in Agents : log[a] = processed[a])

================================================================================
