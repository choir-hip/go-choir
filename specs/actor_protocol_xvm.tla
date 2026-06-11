-------------------------- MODULE actor_protocol_xvm --------------------------
(***************************************************************************)
(* Spec v2 of the Choir durable-actor protocol: CROSS-VM messaging.        *)
(*                                                                         *)
(* First real pair: super (active computer's runtime) <-> vsuper           *)
(* (candidate computer's runtime). Two processes, each with its own        *)
(* durable store and volatile residency, connected by a lossy network      *)
(* (the existing HTTP boundary).                                           *)
(*                                                                         *)
(* Mechanism: the transactional OUTBOX.                                    *)
(*   - A cross-VM send appends to a durable outbox on the sender's VM in   *)
(*     the same transaction as the sender's ledger effects.                *)
(*   - A forwarder retries delivery over the network until acked.          *)
(*   - The receiving runtime's send() dedupes on update_id, so retries     *)
(*     and duplicate deliveries are harmless.                              *)
(*   - The ack is sent only after the update is durably in the receiver's  *)
(*     log. (Sabotage: ack before the durability check — premature ack —   *)
(*     and TLC shows an update acknowledged, dropped, and lost forever.)   *)
(*                                                                         *)
(* Same wire semantics as local: at-least-once visibility, exactly-once    *)
(* ledger effects. Local delivery within each VM is exactly the v1 spec    *)
(* (actor_protocol.tla); this module adds only the boundary.               *)
(***************************************************************************)

EXTENDS Naturals, FiniteSets

CONSTANTS
  VMs,          \* runtime processes, e.g. {vmA, vmB}
  Agents,       \* agent identities, e.g. {super, vsuper}
  Updates,      \* update ids
  MaxCrashes,   \* bound on VM crashes (boot sweep follows each)
  MaxEvictions, \* bound on forced passivations (impl: activation caps)
  MaxDrops      \* bound on network losses (impl: retries eventually land)

\* Each agent is homed on exactly one VM; every VM hosts at least one agent.
HomeOf == CHOOSE f \in [Agents -> VMs] :
            \A v \in VMs : \E a \in Agents : f[a] = v

VARIABLES
  log,        \* log[a]       : durable inbox log on a's home VM
  processed,  \* processed[a] : durably incorporated updates
  resident,   \* set of agents with a live goroutine (volatile, per home VM)
  mailbox,    \* mailbox[a]   : in-memory delivery (volatile)
  outbox,     \* outbox[v]    : durable set of <<update, destAgent>> pending
              \*                remote delivery from VM v
  network,    \* set of <<update, destAgent>> in flight (volatile, lossy)
  crashes, evictions, drops

vars == <<log, processed, resident, mailbox, outbox, network,
          crashes, evictions, drops>>

AllLogged   == UNION {log[a] : a \in Agents}
AllOutboxed == UNION {outbox[v] : v \in VMs}
Fresh(u) ==
  /\ u \notin AllLogged
  /\ \A pair \in AllOutboxed \cup network : pair[1] # u

TypeOK ==
  /\ log \in [Agents -> SUBSET Updates]
  /\ processed \in [Agents -> SUBSET Updates]
  /\ resident \subseteq Agents
  /\ mailbox \in [Agents -> SUBSET Updates]
  /\ outbox \in [VMs -> SUBSET (Updates \X Agents)]
  /\ network \subseteq (Updates \X Agents)
  /\ crashes \in 0..MaxCrashes
  /\ evictions \in 0..MaxEvictions
  /\ drops \in 0..MaxDrops

Init ==
  /\ log = [a \in Agents |-> {}]
  /\ processed = [a \in Agents |-> {}]
  /\ resident = {}
  /\ mailbox = [a \in Agents |-> {}]
  /\ outbox = [v \in VMs |-> {}]
  /\ network = {}
  /\ crashes = 0 /\ evictions = 0 /\ drops = 0

--------------------------------------------------------------------------
(* Local send: sender and recipient share a VM — exactly v1 semantics.     *)
(* The sending VM is HomeOf[a] itself (an agent on a's VM sends to a).     *)

LocalSend(u, a) ==
  /\ Fresh(u)
  /\ log' = [log EXCEPT ![a] = @ \cup {u}]
  /\ IF a \in resident
       THEN /\ mailbox' = [mailbox EXCEPT ![a] = @ \cup {u}]
            /\ UNCHANGED resident
       ELSE /\ resident' = resident \cup {a}
            /\ UNCHANGED mailbox
  /\ UNCHANGED <<processed, outbox, network, crashes, evictions, drops>>

--------------------------------------------------------------------------
(* Cross-VM send: the sender's ledger effects and the outbox entry commit  *)
(* in ONE durable transaction on the sender's VM. Nothing touches the      *)
(* network yet.                                                            *)

RemoteSend(u, v, a) ==
  /\ Fresh(u)
  /\ HomeOf[a] # v                       \* destination is on another VM
  /\ outbox' = [outbox EXCEPT ![v] = @ \cup {<<u, a>>}]
  /\ UNCHANGED <<log, processed, resident, mailbox, network,
                 crashes, evictions, drops>>

(* Forwarder: put a copy on the wire. Retries are free — the outbox keeps  *)
(* the entry until acked, and re-forwarding is idempotent.                 *)
Forward(v, pair) ==
  /\ pair \in outbox[v]
  /\ network' = network \cup {pair}
  /\ UNCHANGED <<log, processed, resident, mailbox, outbox,
                 crashes, evictions, drops>>

(* The network may lose messages (bounded; impl: retries eventually land). *)
Drop(pair) ==
  /\ pair \in network
  /\ drops < MaxDrops
  /\ network' = network \ {pair}
  /\ drops' = drops + 1
  /\ UNCHANGED <<log, processed, resident, mailbox, outbox,
                 crashes, evictions>>

(* Receiver-side delivery: the remote runtime's send() runs — dedupe on    *)
(* update_id, durable append, deliver-or-activate. A duplicate delivery    *)
(* (lost ack, retry) hits the dedupe and is harmless.                      *)
Deliver(pair) ==
  /\ pair \in network
  /\ network' = network \ {pair}
  /\ LET u == pair[1] a == pair[2] IN
     IF u \in log[a]
       THEN UNCHANGED <<log, resident, mailbox>>          \* dup: dedupe
       ELSE /\ log' = [log EXCEPT ![a] = @ \cup {u}]
            /\ IF a \in resident
                 THEN /\ mailbox' = [mailbox EXCEPT ![a] = @ \cup {u}]
                      /\ UNCHANGED resident
                 ELSE /\ resident' = resident \cup {a}
                      /\ UNCHANGED mailbox
  /\ UNCHANGED <<processed, outbox, crashes, evictions, drops>>

(* Ack: the sender clears its outbox entry ONLY once the update is durably *)
(* in the receiver's log. This guard is the whole safety story — weaken it *)
(* (ack on send, before durability) and a dropped message is lost forever. *)
Ack(v, pair) ==
  /\ pair \in outbox[v]
  /\ pair[1] \in log[pair[2]]            \* durably received
  /\ outbox' = [outbox EXCEPT ![v] = @ \ {pair}]
  /\ UNCHANGED <<log, processed, resident, mailbox, network,
                 crashes, evictions, drops>>

--------------------------------------------------------------------------
(* Per-agent lifecycle — identical to v1.                                  *)

Sweep(a) ==
  /\ a \notin resident
  /\ log[a] \ processed[a] # {}
  /\ resident' = resident \cup {a}
  /\ UNCHANGED <<log, processed, mailbox, outbox, network,
                 crashes, evictions, drops>>

Process(a, u) ==
  /\ a \in resident
  /\ u \in log[a] \ processed[a]
  /\ processed' = [processed EXCEPT ![a] = @ \cup {u}]
  /\ mailbox' = [mailbox EXCEPT ![a] = @ \ {u}]
  /\ UNCHANGED <<log, resident, outbox, network, crashes, evictions, drops>>

Passivate(a) ==
  /\ a \in resident
  /\ log[a] \ processed[a] = {}
  /\ resident' = resident \ {a}
  /\ mailbox' = [mailbox EXCEPT ![a] = {}]
  /\ UNCHANGED <<log, processed, outbox, network, crashes, evictions, drops>>

Evict(a) ==
  /\ a \in resident
  /\ evictions < MaxEvictions
  /\ resident' = resident \ {a}
  /\ mailbox' = [mailbox EXCEPT ![a] = {}]
  /\ evictions' = evictions + 1
  /\ UNCHANGED <<log, processed, outbox, network, crashes, drops>>

(* One VM dies: its resident agents and their mailboxes vanish; its        *)
(* durable log/processed/outbox survive; in-flight network traffic is      *)
(* unaffected (loss is modeled by Drop).                                   *)
CrashVM(v) ==
  /\ crashes < MaxCrashes
  /\ resident' = resident \ {a \in Agents : HomeOf[a] = v}
  /\ mailbox' = [a \in Agents |->
                   IF HomeOf[a] = v THEN {} ELSE mailbox[a]]
  /\ crashes' = crashes + 1
  /\ UNCHANGED <<log, processed, outbox, network, evictions, drops>>

--------------------------------------------------------------------------

Next ==
  \/ \E u \in Updates, a \in Agents : LocalSend(u, a)
  \/ \E u \in Updates, v \in VMs, a \in Agents : RemoteSend(u, v, a)
  \/ \E v \in VMs, p \in Updates \X Agents : Forward(v, p)
  \/ \E p \in Updates \X Agents : Drop(p)
  \/ \E p \in Updates \X Agents : Deliver(p)
  \/ \E v \in VMs, p \in Updates \X Agents : Ack(v, p)
  \/ \E a \in Agents : Sweep(a)
  \/ \E a \in Agents, u \in Updates : Process(a, u)
  \/ \E a \in Agents : Passivate(a)
  \/ \E a \in Agents : Evict(a)
  \/ \E v \in VMs : CrashVM(v)

(* Runtime obligations: the actor loop runs, the sweep fires, the          *)
(* forwarder forwards, deliveries and acks happen. Sends, drops, crashes,  *)
(* evictions, passivations are environment.                                *)
Fairness ==
  /\ \A a \in Agents, u \in Updates : WF_vars(Process(a, u))
  /\ \A a \in Agents : WF_vars(Sweep(a))
  /\ \A v \in VMs, p \in Updates \X Agents : WF_vars(Forward(v, p))
  /\ \A p \in Updates \X Agents : WF_vars(Deliver(p))
  /\ \A v \in VMs, p \in Updates \X Agents : WF_vars(Ack(v, p))

Spec == Init /\ [][Next]_vars /\ Fairness

--------------------------------------------------------------------------
(* INVARIANTS *)

ProcessedSubsetLog == \A a \in Agents : processed[a] \subseteq log[a]

MailboxSound == \A a \in Agents : mailbox[a] \subseteq (log[a] \ processed[a])

ColdMailboxEmpty == \A a \in Agents : a \notin resident => mailbox[a] = {}

\* outboxes hold only genuinely remote traffic
OutboxRemoteOnly ==
  \A v \in VMs : \A pair \in outbox[v] : HomeOf[pair[2]] # v

\* nothing rides the network that the sender has already given up on:
\* every in-flight message is still retryable from some durable outbox
NetworkCovered ==
  \A pair \in network :
    \/ pair \in AllOutboxed
    \/ pair[1] \in log[pair[2]]          \* or already durably received

--------------------------------------------------------------------------
(* TEMPORAL PROPERTIES *)

\* THE cross-VM no-loss property: every update a sender durably committed
\* (outbox) or a receiver durably logged is eventually processed at its
\* destination — despite drops, duplicate deliveries, VM crashes,
\* evictions, and passivations.
EventuallyProcessed ==
  \A u \in Updates, a \in Agents :
    (u \in log[a] \/ <<u, a>> \in AllOutboxed) ~> (u \in processed[a])

\* outboxes eventually drain (no eternally retrying forwarder)
OutboxDrains ==
  \A v \in VMs : \A u \in Updates, a \in Agents :
    (<<u, a>> \in outbox[v]) ~> (<<u, a>> \notin outbox[v])

================================================================================
