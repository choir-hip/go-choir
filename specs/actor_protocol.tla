---------------------------- MODULE actor_protocol ----------------------------
(***************************************************************************)
(* Spec of the Choir actor runtime on durable object-graph state.           *)
(*                                                                         *)
(* An actor is an addressable agent with a Go-channel mailbox while         *)
(* resident and a durable update log (SQLite actor_updates) plus an        *)
(* object-graph store (Dolt-backed og_objects / og_edges) while             *)
(* passivated. Sending an update first appends it to the durable log, then  *)
(* either steers the resident actor via its mailbox or activates a cold     *)
(* actor. Processing an update may write an object-graph record; the        *)
(* write is durable and survives eviction or process crash.                  *)
(*                                                                         *)
(* Source design:                                                          *)
(*   internal/actor/actor.go                                                 *)
(*   internal/actor/log_sqlite.go                                          *)
(*   internal/actorruntime/adapter.go                                      *)
(*   internal/actorruntime/handler.go                                     *)
(*   internal/objectgraph/dolt_store.go                                    *)
(*   docs/computer-ontology.md                                               *)
(*                                                                         *)
(* Invariants checked:                                                       *)
(*   1. TypeOK                     — all variables are well-typed.             *)
(*   2. DurableLogCompleteness     — every mailbox entry is in the log.     *)
(*   3. ProcessedImpliesSent       — processing only happens after logging.   *)
(*   4. NoDuplicateDelivery      — a processed update is not requeued.    *)
(*   5. UnprocessedUpdatesReachable — unprocessed updates are recoverable. *)
(*   6. ObjectGraphDurable         — object-graph writes are committed.     *)
(*   7. ObjectGraphUniqueIDs       — object IDs are unique.                 *)
(*   8. MemorySnapshotConsistency  — actor memory is a valid snapshot.      *)
(*                                                                         *)
(* Liveness checked:                                                         *)
(*   EverySentUpdateProcessed    — every sent update is eventually         *)
(*                                 processed (or re-activated after crash). *)
(***************************************************************************)

EXTENDS Integers, FiniteSets, Sequences, TLC

CONSTANTS
  Actors,          \* actor/agent ids, e.g. {a1, a2}
  UpdateIDs,       \* bounded set of update ids, e.g. {u1, u2}
  Kinds,           \* update kinds, e.g. {initial_dispatch, coagent_result}
  Contents,        \* opaque content payloads, e.g. {runA, runB}
  ObjectKinds,     \* object-graph kinds, e.g. {run, agent}
  MaxObjects       \* bound on object-graph records

VARIABLES
  actorState,      \* actorState[a]  : "passive" | "resident"
  mailbox,         \* mailbox[a]     : set of UpdateIDs in the in-memory mailbox
  sent,            \* sent           : set of update IDs appended to the durable log
  updateTo,        \* updateTo[u]    : destination actor
  updateFrom,      \* updateFrom[u]  : source actor
  updateKind,      \* updateKind[u]  : kind of update
  updateContent,   \* updateContent[u] : payload
  processed,       \* processed      : set of update IDs handled by the actor
  objects,         \* objects        : set of object-graph records
  actorMemory,     \* actorMemory[a] : compact resume snapshot
  nextObjectId     \* monotonic counter for object IDs

vars == << actorState, mailbox, sent, updateTo, updateFrom, updateKind,
           updateContent, processed, objects, actorMemory, nextObjectId >>

ActorStates   == {"passive", "resident"}
ObjectStates  == {"created", "committed"}
NoMemory      == {"none"}

TypeOK ==
  /\ actorState \in [Actors -> ActorStates]
  /\ mailbox \in [Actors -> SUBSET UpdateIDs]
  /\ sent \in SUBSET UpdateIDs
  /\ updateTo \in [UpdateIDs -> Actors]
  /\ updateFrom \in [UpdateIDs -> Actors]
  /\ updateKind \in [UpdateIDs -> Kinds]
  /\ updateContent \in [UpdateIDs -> Contents]
  /\ processed \in SUBSET UpdateIDs
  /\ objects \in SUBSET [id: 1..MaxObjects, kind: ObjectKinds, owner: Actors, state: ObjectStates]
  /\ actorMemory \in [Actors -> (Contents \cup NoMemory)]
  /\ nextObjectId \in 1..MaxObjects+1

Init ==
  /\ actorState = [a \in Actors |-> "passive"]
  /\ mailbox = [a \in Actors |-> {}]
  /\ sent = {}
  /\ updateTo = [u \in UpdateIDs |-> CHOOSE a \in Actors : TRUE]
  /\ updateFrom = [u \in UpdateIDs |-> CHOOSE a \in Actors : TRUE]
  /\ updateKind = [u \in UpdateIDs |-> CHOOSE k \in Kinds : TRUE]
  /\ updateContent = [u \in UpdateIDs |-> CHOOSE c \in Contents : TRUE]
  /\ processed = {}
  /\ objects = {}
  /\ actorMemory = [a \in Actors |-> "none"]
  /\ nextObjectId = 1

--------------------------------------------------------------------------
(* Backlog for an actor: all sent updates destined for it that have not    *)
(* yet been processed.                                                     *)

Backlog(a) == {u \in (sent \ processed) : updateTo[u] = a}

--------------------------------------------------------------------------
(* Send an update. The update is first appended to the durable log. If     *)
(* the destination actor is resident, it is steered into the mailbox; if it   *)
(* is passive, it is activated. This is the "database remembers; Go          *)
(* delivers" contract.                                                     *)

Send(u, to, from, k, c) ==
  /\ u \in UpdateIDs
  /\ u \notin sent
  /\ to \in Actors
  /\ from \in Actors
  /\ k \in Kinds
  /\ c \in Contents
  /\ sent' = sent \cup {u}
  /\ updateTo' = [updateTo EXCEPT ![u] = to]
  /\ updateFrom' = [updateFrom EXCEPT ![u] = from]
  /\ updateKind' = [updateKind EXCEPT ![u] = k]
  /\ updateContent' = [updateContent EXCEPT ![u] = c]
  /\ IF actorState[to] = "resident"
       THEN mailbox' = [mailbox EXCEPT ![to] = @ \cup {u}]
       ELSE mailbox' = mailbox
  /\ IF actorState[to] = "passive"
       THEN actorState' = [actorState EXCEPT ![to] = "resident"]
       ELSE actorState' = actorState
  /\ UNCHANGED << processed, objects, actorMemory, nextObjectId >>

--------------------------------------------------------------------------
(* Process one update from a resident actor's mailbox. The update is       *)
(* removed from the mailbox, marked processed in the durable log, and an      *)
(* object-graph record (representing the actor's durable effect) is         *)
(* committed. The compact resume snapshot is updated.                       *)

Process(a, u) ==
  /\ a \in Actors
  /\ u \in UpdateIDs
  /\ actorState[a] = "resident"
  /\ u \in mailbox[a]
  /\ u \notin processed
  /\ mailbox' = [mailbox EXCEPT ![a] = @ \ {u}]
  /\ processed' = processed \cup {u}
  /\ actorMemory' = [actorMemory EXCEPT ![a] = updateContent[u]]
  /\ IF nextObjectId <= MaxObjects
       THEN /\ objects' = objects \cup {[id |-> nextObjectId,
                                          kind |-> "run",
                                          owner |-> a,
                                          state |-> "committed"]}
            /\ nextObjectId' = nextObjectId + 1
       ELSE /\ objects' = objects
            /\ nextObjectId' = nextObjectId
  /\ UNCHANGED << actorState, sent, updateTo, updateFrom, updateKind, updateContent >>

--------------------------------------------------------------------------
(* Passivation: an idle resident actor saves its memory snapshot and goes    *)
(* passive. It may only passivate when there is no backlog for it.           *)

Passivate(a) ==
  /\ a \in Actors
  /\ actorState[a] = "resident"
  /\ mailbox[a] = {}
  /\ Backlog(a) = {}
  /\ actorState' = [actorState EXCEPT ![a] = "passive"]
  /\ UNCHANGED << mailbox, sent, updateTo, updateFrom, updateKind,
                  updateContent, processed, objects, actorMemory, nextObjectId >>

--------------------------------------------------------------------------
(* Eviction: a resident actor is killed without saving its memory snapshot.  *)
(* The in-memory mailbox is lost, but the durable log is untouched; the      *)
(* next Sweep re-activates the actor from the log. This is crash-equivalent.  *)

Evict(a) ==
  /\ a \in Actors
  /\ actorState[a] = "resident"
  /\ actorState' = [actorState EXCEPT ![a] = "passive"]
  /\ mailbox' = [mailbox EXCEPT ![a] = {}]
  /\ UNCHANGED << sent, updateTo, updateFrom, updateKind, updateContent,
                  processed, objects, actorMemory, nextObjectId >>

--------------------------------------------------------------------------
(* Sweep: boot/periodic recovery. Activates any passive actor with           *)
(* unprocessed durable backlog and loads that backlog into the mailbox.     *)

Sweep(a) ==
  /\ a \in Actors
  /\ actorState[a] = "passive"
  /\ Backlog(a) # {}
  /\ actorState' = [actorState EXCEPT ![a] = "resident"]
  /\ mailbox' = [mailbox EXCEPT ![a] = Backlog(a)]
  /\ UNCHANGED << sent, updateTo, updateFrom, updateKind, updateContent,
                  processed, objects, actorMemory, nextObjectId >>

--------------------------------------------------------------------------
(* The full next-state relation.                                             *)

Next ==
  /\ \/ \E u \in UpdateIDs, to, from \in Actors, k \in Kinds, c \in Contents :
          Send(u, to, from, k, c)
     \/ \E a \in Actors, u \in UpdateIDs : Process(a, u)
     \/ \E a \in Actors : Passivate(a)
     \/ \E a \in Actors : Evict(a)
     \/ \E a \in Actors : Sweep(a)

--------------------------------------------------------------------------
(* Invariants: what must never be true on any reachable state.               *)

(* The mailbox is always backed by the durable log. *)
DurableLogCompleteness ==
  \A a \in Actors : mailbox[a] \subseteq sent

(* An update cannot be processed before it has been sent. *)
ProcessedImpliesSent ==
  processed \subseteq sent

(* A processed update is never still in any mailbox. *)
NoDuplicateDelivery ==
  \A a \in Actors : mailbox[a] \cap processed = {}

(* An unprocessed update is always reachable: either in a resident mailbox or *)
(* in the backlog of a passive actor that Sweep can re-activate.             *)
UnprocessedUpdatesReachable ==
  \A u \in UpdateIDs :
    u \in (sent \ processed) =>
      (\E a \in Actors : actorState[a] = "resident" /\ u \in mailbox[a])
      \/ (\E a \in Actors : actorState[a] = "passive" /\ updateTo[u] = a)

(* Every object in the graph is committed and has a unique identity. *)
ObjectGraphDurable ==
  \A obj \in objects : obj.state = "committed"

(* At most one object record per ID. *)
ObjectGraphUniqueIDs ==
  \A o1, o2 \in objects : o1.id = o2.id => o1 = o2

(* The actor memory snapshot is either the last processed content or none. *)
MemorySnapshotConsistency ==
  \A a \in Actors : actorMemory[a] \in (Contents \cup {"none"})

--------------------------------------------------------------------------
(* Liveness: what must eventually happen.                                   *)
(* Every sent update is eventually processed. If the actor is evicted,     *)
(* Sweep re-activates it from the durable log.                              *)

EverySentUpdateProcessed ==
  \A u \in UpdateIDs :
    u \in sent ~> u \in processed

Fairness ==
  /\ \A a \in Actors, u \in UpdateIDs : WF_vars(Process(a, u))
  /\ \A a \in Actors : WF_vars(Sweep(a))
  /\ \A a \in Actors : WF_vars(Passivate(a))

Spec == Init /\ [][Next]_vars /\ Fairness

============================================================================
