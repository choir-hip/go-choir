# specs/ — model-checked designs

> **Rewrite in progress.** We are pre-launch and replacing the old specs with new ones that model the current architecture (autoputer + object graph + actor runtime + capsules). `promotion_protocol.tla` is the gate and already checks green. `actor_protocol.tla` and `autoputer_lifecycle.tla` are drafted and model-checked in this pass. `actor_protocol_xvm.tla` and `wire_pipeline.tla` will follow in later Mission S work.

Plain-language guide to the TLA+ specs. No TLA+ knowledge assumed.

## How to read any of this

A spec is not code. It is a precise description of a tiny board game:
**variables** are the board, **actions** are the legal moves, and TLC (the
model checker) plays *every possible game* — every ordering of moves, every
crash at every moment — and reports whether the rules we care about
(**invariants**: "this is never true on any board", and **liveness**: "this
eventually happens in every game") survive all of them. When a rule breaks,
TLC prints the exact move sequence that breaks it.

The specs are deliberately small (2–3 agents, 2–3 messages). That is not a
limitation: concurrency bugs need few participants — just the one ordering
nobody thought of. TLC tries all of them; humans try three.

Run any spec:

```
cd specs && nix shell nixpkgs#tlaplus --command tlc -deadlock -workers auto <name>.tla
```

"No error has been found" = every reachable state checked, all rules hold.

---

## 1. `promotion_protocol.tla` — the autoputer promotion gate (NEW)

**The story.** A computer is not a sandbox. It is a durable product of
heterogeneous ledgers (source/build, Dolt/app, VM/OS, blobs, artifacts,
route identity). A candidate computer is a speculative fork of an active
computer. Promotion is the atomic flip of the route identity from the active
computer to the candidate, guarded by per-ledger prepare/verify, owner
approval, and a freshness CAS. After commit, a health window ends in
confirmation or revert. This is the gate that makes the autoputer a real
computer.

**The board:**
- `activeBase[a]` — version of active computer *a* (the foreground tail).
- `candidateBase[c]` — version at which candidate *c* forked from its parent.
- `candidateParent[c]` — active computer that *c* forks from.
- `route[s]` — the computer currently serving slot *s* (active or candidate).
- `ledgerState[c][l]` — per-ledger state for promotion *c*: `none`,
  `prepared`, `applied`, `rolled_back`.
- `promoStatus[c]` — lifecycle: `staging`, `verified`, `approved`,
  `committed`, `confirmed`, `aborted`, `reverted`.
- `promoBase[c]` — active base version at fork/verify time (freshness CAS).
- `approved[c]` — owner approval recorded.
- `poisoned[c]` — new version wrote data the old version cannot read.
- `healthWindow[c]` — `open`, `failed`, `confirmed`.

**The moves:**
- **ForkCandidate** — durable fork from an active computer; records the base.
- **MoveActiveTail** — the active computer keeps changing during candidacy.
- **PrepareLedger** — per-ledger prepare, durable and inert until commit.
- **Restage** — active base moved; drop back to `staging` and invalidate
  verification/approval (evidence about a stale base authorizes nothing).
- **Verify** — all ledgers prepared and base is fresh → `verified`.
- **Approve** — owner authorizes the verified transition → `approved`.
- **Commit** — approved, all ledgers prepared, freshness CAS holds → atomic
  route-pointer flip to the candidate.
- **Abort** — safe backward recovery before commit.
- **ApplySecondary** — after commit, each ledger rolls forward to `applied`.
- **PoisonedWrite** — after commit, a write closes the rollback window.
- **HealthCheckFail** — the health window fails while still reversible.
- **ConfirmHealthy** — all secondaries applied and window not poisoned.
- **AutoRevert** — health failed and window open → route flips back to active.
- **RollbackSecondary** — after abort or revert, secondaries roll back.

**The rules that hold (model-checked):**
- *NoStaleCommit* — no commit if the active base moved since the candidate
  was verified.
- *ApprovalGate* — no commit without explicit owner approval.
- *NoTornOutcome* — settled promotions are uniform across ledgers.
- *RouteConsistency* — the route pointer always points to an active computer
  or to a committed candidate.
- *CandidateIsolation* — a candidate is not route-visible before commit.
- *HealthWindowReversible* — revert only while the rollback window is open.
- *ConfirmedLedgersApplied* — all ledgers are applied before a promotion is
  confirmed.
- *AbortedLedgersRolledBack* — all ledgers are rolled back after abort/revert.
- *CertificateCompleteness* — every settled promotion records its base and
  candidate.
- *EveryPromotionSettles* (liveness) — every promotion eventually aborts,
  reverts, or is confirmed.

**What TLC will catch when we sabotage it:**
- Drop the freshness CAS → active-computer changes silently overwritten.
- Drop the approval gate → unreviewed candidate becomes the active computer.
- Allow revert after a poisoned write → torn rollback, old version reads data
  it cannot interpret.
- Update the route before all ledgers prepare → route points to inconsistent
  state.

**Design doc:** `docs/promotion-protocol-spec-staleness-and-redefinition-2026-07-03.md`.

---

## 2. `actor_protocol.tla` — actor runtime with object-graph state (NEW)

**The story.** An actor is a long-lived agent. While resident it owns a Go-channel
mailbox; while passivated it is represented by a durable update log plus the
object graph. `Send` first appends the update to the SQLite `actor_updates` log,
then either steers the resident actor through the mailbox or activates a cold
actor. Processing an update may write an object-graph record (Dolt-backed
`og_objects`/`og_edges`); the write survives eviction or process crash. This is
the "database remembers; Go delivers" contract.

**The board:**
- `actorState[a]` — `"passive"` or `"resident"`.
- `mailbox[a]` — in-memory set of update IDs queued for actor *a*.
- `sent` — set of update IDs appended to the durable log.
- `updateTo[u]`, `updateFrom[u]`, `updateKind[u]`, `updateContent[u]` — update metadata.
- `processed` — set of update IDs handled by the actor.
- `objects` — durable object-graph records written by processing.
- `actorMemory[a]` — compact resume snapshot saved on passivation.
- `nextObjectId` — monotonic object identity counter.

**The moves:**
- **Send** — append an update to the durable log; deliver to mailbox or activate.
- **Process** — handle one mailbox update, mark it processed, write an object-graph record, update memory.
- **Passivate** — idle resident actor saves its memory and goes passive.
- **Evict** — resident actor is killed (crash-equivalent); mailbox is lost, durable log stays.
- **Sweep** — boot/periodic recovery activates any passive actor with unprocessed backlog.

**The rules that hold (model-checked):**
- *TypeOK* — all variables are well-typed.
- *DurableLogCompleteness* — every mailbox entry is backed by the durable log.
- *ProcessedImpliesSent* — an update is processed only after it has been sent.
- *NoDuplicateDelivery* — a processed update is never still in a mailbox.
- *UnprocessedUpdatesReachable* — every unprocessed update is either in a resident
  mailbox or in a passive actor's backlog (recoverable by Sweep).
- *ObjectGraphDurable* — every object-graph record is committed.
- *ObjectGraphUniqueIDs* — object IDs are unique.
- *MemorySnapshotConsistency* — actor memory is a valid content snapshot.
- *EverySentUpdateProcessed* (liveness) — every sent update is eventually processed,
  even if the actor is evicted and re-activated.

**What TLC will catch when we sabotage it:**
- Drop the durable log append → updates vanish on eviction.
- Clear the mailbox without marking processed → at-least-once delivery breaks.
- Allow passivation while backlog exists → unprocessed updates starve.
- Forget to Sweep on boot → cold actors never recover their backlog.

**Design docs:** `internal/actor/actor.go`, `internal/actorruntime/adapter.go`,
`internal/objectgraph/dolt_store.go`, `docs/computer-ontology.md`.

---

## 3. `actor_protocol_xvm.tla` — cross-VM and cross-capsule messages (COMING)

**The story.** Agents live in the autoputer. Risky effects run inside Nucleus
capsules. Messages must survive both the VM boundary and the capsule
boundary. The spec extends the actor protocol with durable outboxes, capsule
isolation, and retry semantics.

**Status:** not yet rewritten. Depends on `actor_protocol.tla`.

---

## 4. `wire_pipeline.tla` — Universal Wire on object-graph trajectories (COMING)

**The story.** One layer up from the actor protocol: the wire pipeline turns
source items into published stories through durable trajectory state. The
processor spawns Texture agents, each writes to the object graph, and
settlement is explicit rather than run-tree completion.

**Status:** not yet rewritten. Depends on `actor_protocol.tla`.

---

## 5. `autoputer_lifecycle.tla` — VM boot, health, recovery, hibernation (NEW)

**The story.** The autoputer is a persistent computer (VM) that runs the Choir
runtime. The boot sequence is: power-on → systemd start → runtime init → bind
port 8085 → health check. The current staging failure is reproduced as
**RuntimeInitFail**: when the runtime substrate is still stale, the runtime init
step fails before the service can bind to port 8085. The **RepairRuntime** action
represents the substrate migration (actor runtime + object graph) that makes the
boot path green. Once healthy, the VM can crash, recover, or hibernate and resume.

**The board:**
- `phase` — VM lifecycle: `off`, `booting`, `running`, `bound`, `healthy`, `failed`, `hibernating`.
- `runtimeState` — substrate state: `stale` (current), `ok` (repaired), `failed`.
- `portBound` — `TRUE` once the service binds to port 8085.
- `attempts` — number of recovery attempts.
- `bootCount` — number of boot/hibernation cycles.

**The moves:**
- **PowerOn** — start the VM.
- **BootSystemd** — systemd starts the autoputer service.
- **RuntimeInitOk** — runtime init succeeds because the substrate is `ok`.
- **RuntimeInitFail** — runtime init fails because the substrate is `stale`;
  reproduces the 8085 bind failure.
- **RepairRuntime** — migrate from stale runtime to actor runtime + object graph.
- **BindPort** — the service binds to port 8085.
- **HealthCheck** — health probe passes once the port is bound.
- **Crash** — a healthy VM fails and reverts to stale runtime.
- **Recover** — retry boot after a failed state.
- **Hibernate** — suspend a healthy VM after flushing durable work.
- **Resume** — wake from hibernation and reboot.

**The rules that hold (model-checked):**
- *TypeOK* — all variables are well-typed.
- *HealthyImpliesBound* — the VM is healthy only after port 8085 is bound.
- *BoundImpliesRuntimeOk* — port binding requires a repaired runtime.
- *RecoveryBounded* — recovery attempts stay within the configured limit.
- *HibernationSafe* — hibernation only happens after a successful boot.
- *NoStuckFailure* — a failed VM can recover while attempts remain.
- *EventuallyHealthy* (liveness) — the VM eventually reaches a healthy serving
  state after the substrate is repaired.

**What TLC will catch when we sabotage it:**
- Repair the runtime but never bind the port → health check never passes.
- Crash the VM and remove the recovery path → `EventuallyHealthy` breaks.
- Allow runtime state to revert from `ok` to `stale` without a crash →
  `BoundImpliesRuntimeOk` breaks.

**Design docs:** `cmd/sandbox/main.go`, `internal/sandbox/config.go`,
`internal/server/server.go`, `docs/mission-autoputer-before-autopaper-v0.md`.

---

## Historical context

The previous specs (`actor_protocol.tla`, `actor_protocol_xvm.tla`,
`wire_pipeline.tla`, `promotion_protocol.tla`) described earlier
architectures and have been removed. The old promotion protocol spec is
especially notable: it correctly diagnosed two missing guards
(`NoStaleCommit`, `ApprovalGate`) that were later added to the Go code. The
new spec starts from the current implementation and the computer ontology in
`docs/computer-ontology.md`.

---

## The layering (target)

```
wire_pipeline.tla        the business logic is sound
promotion_protocol.tla   state changes are atomic, approved, reversible
       ↑ assume
autoputer_lifecycle.tla  VM boot, health, recovery, hibernation
       ↑ assume
actor_protocol_xvm.tla   messages survive VM and capsule boundaries
       ↑ extends
actor_protocol.tla       messages reach agents; object graph is the durable log
```

`actor_protocol.tla` is the lowest layer: it says messages reach agents and
actor effects are durable in the object graph. `autoputer_lifecycle.tla` builds
on it by assuming the runtime can boot and recover because the actor runtime's
Sweep and durable log re-activate in-flight work. `promotion_protocol.tla` sits
above the lifecycle and uses the object graph to prove ledger-atomic route
flips. `wire_pipeline.tla` (coming) will be the business layer above the actor
protocol, using object-graph trajectories and editions.

Each future subsystem (adoption state machine, other trajectory kinds) gets
its own module at the right layer. Every PR that changes protocol behavior
changes the spec first; TLC runs in CI so the specs stay load-bearing.

## Sabotage catalog (new specs)

| Sabotage | Caught by | States to counterexample |
|---|---|---|
| commit without freshness CAS | NoStaleCommit | short trace |
| commit without owner approval | ApprovalGate | short trace |
| revert after poisoned write | HealthWindowReversible | short trace |
| route flip before all ledgers prepared | RouteConsistency | short trace |
| candidate route-visible before commit | CandidateIsolation | short trace |
| skip durable log append on actor send | DurableLogCompleteness, UnprocessedUpdatesReachable | short trace |
| mark update processed without writing object | ObjectGraphDurable | short trace |
| passivate actor with unprocessed backlog | UnprocessedUpdatesReachable | short trace |
| bind port 8085 before runtime init | BoundImpliesRuntimeOk | short trace |
| crash without recovery path | EventuallyHealthy | medium trace |

Historical sabotage catalog for the old specs is preserved in the Git history
of `docs/archive/` if needed.
