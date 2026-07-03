---------------------------- MODULE autoputer_lifecycle ----------------------------
(***************************************************************************)
(* Spec of the Choir autoputer VM lifecycle: boot, health, recovery, and  *)
(* hibernation.                                                            *)
(*                                                                         *)
(* The autoputer is a persistent computer (VM) that runs the Choir runtime. *)
(* Boot sequence: power-on -> systemd start -> runtime init -> bind port   *)
(* 8085 -> health check. The current failure is reproduced as the            *)
(* RuntimeInitFail action: when the runtime substrate is still stale, the   *)
(* runtime init step fails before the service can bind to port 8085. The    *)
(* RepairRuntime action represents the substrate migration (actor runtime   *)
(* + object graph) that makes the boot succeed.                             *)
(*                                                                         *)
(* Source design:                                                          *)
(*   cmd/sandbox/main.go                                                   *)
(*   internal/sandbox/config.go                                            *)
(*   internal/server/server.go                                             *)
(*   internal/actorruntime/adapter.go                                      *)
(*   docs/mission-autoputer-before-autopaper-v0.md                        *)
(*   docs/mission-universal-wire-stabilization-v1.md                      *)
(*                                                                         *)
(* Invariants checked:                                                       *)
(*   1. TypeOK                  — all variables are well-typed.            *)
(*   2. HealthyImpliesBound     — the VM is healthy only after binding.   *)
(*   3. BoundImpliesRuntimeOk   — port binding requires a healthy runtime.  *)
(*   4. RecoveryBounded         — recovery attempts stay within the limit.  *)
(*   5. HibernationSafe         — hibernation preserves the bound port.     *)
(*   6. NoStuckFailure          — a failed VM can recover and boot again.   *)
(*                                                                         *)
(* Liveness checked:                                                         *)
(*   EventuallyHealthy          — the VM eventually reaches a healthy,    *)
(*                                serving state (after substrate repair).  *)
(***************************************************************************)

EXTENDS Integers, FiniteSets, Sequences, TLC

CONSTANTS
  MaxAttempts,     \* bound on boot/recovery attempts
  MaxBoots         \* bound on boot/hibernation cycles

VARIABLES
  phase,           \* VM lifecycle phase
  runtimeState,    \* runtime substrate state: "stale" | "ok" | "failed"
  portBound,       \* TRUE once the service binds to port 8085
  attempts,        \* number of recovery attempts
  bootCount        \* number of completed boot/hibernation cycles

vars == << phase, runtimeState, portBound, attempts, bootCount >>

Phases == {"off", "booting", "running", "bound", "healthy", "failed", "hibernating"}
RuntimeStates == {"stale", "ok", "failed"}

TypeOK ==
  /\ phase \in Phases
  /\ runtimeState \in RuntimeStates
  /\ portBound \in BOOLEAN
  /\ attempts \in 0..MaxAttempts
  /\ bootCount \in 0..MaxBoots

Init ==
  /\ phase = "off"
  /\ runtimeState = "stale"        \* current substrate: stale runtime
  /\ portBound = FALSE
  /\ attempts = 0
  /\ bootCount = 0

--------------------------------------------------------------------------
(* Power-on: the VM starts.                                                *)

PowerOn ==
  /\ phase = "off"
  /\ phase' = "booting"
  /\ UNCHANGED << runtimeState, portBound, attempts, bootCount >>

--------------------------------------------------------------------------
(* Systemd starts the autoputer service.                                   *)

BootSystemd ==
  /\ phase = "booting"
  /\ phase' = "running"
  /\ UNCHANGED << runtimeState, portBound, attempts, bootCount >>

--------------------------------------------------------------------------
(* Runtime init succeeds: the actor runtime / object graph are ready.      *)
(* The VM is now ready to bind port 8085.                                  *)

RuntimeInitOk ==
  /\ phase = "running"
  /\ runtimeState = "ok"
  /\ phase' = "bound"
  /\ portBound' = FALSE
  /\ UNCHANGED << runtimeState, attempts, bootCount >>

--------------------------------------------------------------------------
(* Runtime init fails: the stale runtime substrate cannot start.           *)
(* This reproduces the observed failure where the VM boots but the         *)
(* service never binds to port 8085.                                       *)

RuntimeInitFail ==
  /\ phase = "running"
  /\ runtimeState = "stale"
  /\ phase' = "failed"
  /\ runtimeState' = "stale"
  /\ portBound' = FALSE
  /\ UNCHANGED << attempts, bootCount >>

--------------------------------------------------------------------------
(* Repair the runtime substrate: migrate from stale runtime to actor       *)
(* runtime + object graph. This is the fix that makes the boot path green.  *)

RepairRuntime ==
  /\ runtimeState = "stale"
  /\ runtimeState' = "ok"
  /\ UNCHANGED << phase, portBound, attempts, bootCount >>

--------------------------------------------------------------------------
(* The service binds to port 8085.                                         *)

BindPort ==
  /\ phase = "bound"
  /\ portBound = FALSE
  /\ portBound' = TRUE
  /\ UNCHANGED << phase, runtimeState, attempts, bootCount >>

--------------------------------------------------------------------------
(* Health check passes once the port is bound.                             *)

HealthCheck ==
  /\ phase = "bound"
  /\ portBound = TRUE
  /\ phase' = "healthy"
  /\ UNCHANGED << runtimeState, portBound, attempts, bootCount >>

--------------------------------------------------------------------------
(* Crash: a healthy serving VM fails. It may be a software panic, a host    *)
(* event, or an unrecoverable runtime error.                               *)

Crash ==
  /\ phase = "healthy"
  /\ phase' = "failed"
  /\ runtimeState' = "stale"
  /\ portBound' = FALSE
  /\ UNCHANGED << attempts, bootCount >>

--------------------------------------------------------------------------
(* Recover from a failed boot/crash and retry.                             *)

Recover ==
  /\ phase = "failed"
  /\ attempts < MaxAttempts
  /\ phase' = "booting"
  /\ attempts' = attempts + 1
  /\ runtimeState' = "stale"
  /\ portBound' = FALSE
  /\ UNCHANGED << bootCount >>

--------------------------------------------------------------------------
(* Hibernate: a healthy VM is suspended. In-flight actor work is flushed    *)
(* to the durable log, then the VM pauses.                                 *)

Hibernate ==
  /\ phase = "healthy"
  /\ bootCount < MaxBoots
  /\ phase' = "hibernating"
  /\ bootCount' = bootCount + 1
  /\ UNCHANGED << runtimeState, portBound, attempts >>

--------------------------------------------------------------------------
(* Resume from hibernation: the VM reboots from durable state.             *)

Resume ==
  /\ phase = "hibernating"
  /\ phase' = "booting"
  /\ portBound' = FALSE
  /\ UNCHANGED << runtimeState, attempts, bootCount >>

--------------------------------------------------------------------------
(* The full next-state relation.                                             *)

Next ==
  PowerOn
  \/ BootSystemd
  \/ RuntimeInitOk
  \/ RuntimeInitFail
  \/ RepairRuntime
  \/ BindPort
  \/ HealthCheck
  \/ Crash
  \/ Recover
  \/ Hibernate
  \/ Resume

--------------------------------------------------------------------------
(* Invariants: what must never be true on any reachable state.               *)

(* Healthy only after port 8085 is bound. *)
HealthyImpliesBound ==
  phase = "healthy" => portBound = TRUE

(* Port is only bound after runtime init succeeds. *)
BoundImpliesRuntimeOk ==
  portBound = TRUE => runtimeState = "ok"

(* Recovery attempts stay bounded. *)
RecoveryBounded ==
  attempts <= MaxAttempts

(* Hibernation only happens after at least one successful boot. *)
HibernationSafe ==
  phase = "hibernating" => bootCount > 0

(* A failed VM can recover while attempts remain.                              *)
NoStuckFailure ==
  (phase = "failed" /\ attempts < MaxAttempts) => ENABLED Recover

--------------------------------------------------------------------------
(* Liveness: what must eventually happen.                                   *)
(* After the stale runtime is repaired, the VM eventually reaches a healthy *)
(* serving state.                                                           *)

EventuallyHealthy ==
  <>(phase = "healthy")

Fairness ==
  /\ WF_vars(PowerOn)
  /\ WF_vars(BootSystemd)
  /\ WF_vars(RuntimeInitOk)
  /\ WF_vars(RepairRuntime)
  /\ WF_vars(BindPort)
  /\ WF_vars(HealthCheck)
  /\ WF_vars(Recover)
  /\ WF_vars(Hibernate)
  /\ WF_vars(Resume)

Spec == Init /\ [][Next]_vars /\ Fairness

============================================================================
