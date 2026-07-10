# specs/ — model-checked designs

> **Rewrite in progress.** We are pre-launch and replacing the old specs with new ones that model the current architecture (autoputer + object graph + actor runtime + capsules). Only `promotion_protocol.tla` has been rewritten so far; it is the gate for the autoputer. The other specs will follow as Mission S work.

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

## 2. `actor_protocol.tla` — actor runtime with object-graph state (COMING)

**The story.** Agents are durable actors. Their durable state is now the
object graph (Dolt-backed objects and edges), not a raw message log. The spec
models how an actor runtime processes updates, guarantees at-least-once
visibility, and keeps exactly-once ledger effects.

**Status:** not yet rewritten. Mission S next item after the promotion gate.

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

## 5. `autoputer_lifecycle.tla` — VM boot, health, recovery, hibernation (COMING)

**The story.** The autoputer VM must boot, bind its runtime, recover from
crashes, and hibernate without losing in-flight work. The spec will model the
boot sequence and explain why the current VM fails to bind to port 8085.

**Status:** not yet rewritten. Mission S item after actor protocol.

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

The historical sabotage catalog for old specs remains available in Git history.
