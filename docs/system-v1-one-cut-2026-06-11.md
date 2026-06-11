# System v1 in One Cut — Derisking Pseudocode — 2026-06-11

## Status

Bulk derisking artifact for the mission portfolio
(`docs/mission-portfolio-2026-06-11.md`). This document specifies the first
iteration of the rearchitected system **in one cut**: one durable schema, one
control flow, the five hardest seams decided in advance. Each mission
implements a slice of this; none designs locally. When implementation
contradicts a decision here, that is target-level learning — update this
document, don't fork it silently.

Pseudocode is Go-flavored but normative only at the level of *shape*: what
is in one transaction, who wakes whom, what is durable vs resident, what is
data vs code.

---

## 0. The foliation

The system state space is foliated into three leaves; every design decision
below is about keeping things on the right leaf:

```
DURABLE   (survives crash; the database remembers)
   updates log, work items, trajectories, slots, promotions, routes,
   snapshots, provenance edges, adoption/package records, dolt branches
RESIDENT  (volatile; Go delivers)
   actor goroutines, mailboxes, LLM sessions, in-flight tool calls
POLICY    (prompts + bounded profiles; the untrusted prover layer)
   what agents decide; never load-bearing for safety
   governed by the role-free actor protocol: actors get obligations,
   not identities (docs/choir-role-free-actor-protocol-2026-06-11.md)
```

Rules of the foliation:
- Nothing is *only* resident if losing it loses work (→ durable).
- Nothing durable encodes a *decision policy* (→ prompts/owners decide;
  durable state records decisions, never makes them — the continuation
  lesson).
- Safety properties live in the durable leaf's transitions and are the
  spec-checked surface.

The five cuts below are the places where two leaves meet and the seam is
hard. Everything else in the portfolio is ordinary work.

---

## 1. The durable schema (one place, all missions)

```sql
-- M1: causality
CREATE TABLE trajectories (
  trajectory_id   TEXT PRIMARY KEY,
  owner_id        TEXT NOT NULL,
  kind            TEXT NOT NULL,     -- publication | adoption | mission | user_session
  subject_json    TEXT NOT NULL,     -- {doc_id, publication_id, adoption_id, mission_ref, ...}
  settlement_json TEXT NOT NULL,     -- the RULE, as data (see Cut 2)
  status          TEXT NOT NULL,     -- live | settled | cancelled
  created_at, settled_at
);

-- M1: work assignment (the continuation record's good half, re-keyed)
CREATE TABLE work_items (
  work_item_id    TEXT PRIMARY KEY,
  trajectory_id   TEXT NOT NULL REFERENCES trajectories,
  assignee_agent  TEXT NOT NULL,
  objective       TEXT NOT NULL,
  authority       TEXT NOT NULL,     -- bounded profile (budgets/caps live in
                                     -- runtime options; no lease concept in v1)
  fingerprint     TEXT NOT NULL,     -- dedupe: owner+trajectory+normalized objective
  status          TEXT NOT NULL,     -- open | done | cancelled
  source_update   TEXT,              -- the update whose send created this (Cut 3)
  created_by_agent TEXT,             -- provenance only, no control reads
  created_at, closed_at
);
CREATE UNIQUE INDEX work_dedupe ON work_items(fingerprint) WHERE status = 'open';

-- M2: messaging (extends internal/actor's actor_updates)
--   actor_updates gains: kind, trajectory_id  (already in the Update struct)
--   kinds: findings | evidence | verification | blocker | question |
--          proposal | status | directive | assignment | capability_request

-- M2: slot registry (replaces parent+slot co-super sequencing)
CREATE TABLE coagent_slots (
  trajectory_id TEXT NOT NULL,
  slot          TEXT NOT NULL,       -- implementation | verifier
  active_work   TEXT REFERENCES work_items,
  PRIMARY KEY (trajectory_id, slot)
);

-- M3: runs demoted to activation records (presentation + acceptance only)
--   runs table kept; parent_loop_id frozen (no new control reads);
--   new column: trajectory_id; new read model: activation = one residency.

-- M6: the commit point made real
CREATE TABLE promotions (
  promotion_id  TEXT PRIMARY KEY,
  adoption_id   TEXT NOT NULL,
  computer_id   TEXT NOT NULL,
  state         TEXT NOT NULL,       -- staging|verified|approved|committed|confirmed|aborted|reverted
  base_ref      TEXT NOT NULL,       -- lineage ref at verification (freshness CAS, shipped)
  ledgers_json  TEXT NOT NULL,       -- per-ledger: none|prepared|applied|rolled_back
  window_open   BOOLEAN NOT NULL DEFAULT TRUE,  -- M8: rollback window
  created_at, committed_at, settled_at
);

CREATE TABLE routes (
  computer_id    TEXT PRIMARY KEY,
  ui_artifact    TEXT NOT NULL,      -- digest/dir of the served UI bundle
  runtime_artifact TEXT NOT NULL,    -- digest of the runtime build
  promotion_id   TEXT NOT NULL,      -- which committed promotion set this
  updated_at
);
```

What is deliberately ABSENT: a continuations table (deleted), a parent/child
control column (frozen), any "current step" workflow state (actors decide),
any content field on trajectories or work items (no second article truth).

---

## 2. Cut 1 — the LLM tool-loop as an actor turn (M2/M3's hardest seam)

The question: `executeRun` is a long streaming tool-loop; the actor model
wants discrete update-driven turns. The cut:

**An activation hosts one LLM session. Updates are the only inputs. The
session yields; yielding with an empty mailbox is passivation.**

```go
// The adapter implements actor.Handler. One activation == one session.
type llmActor struct{ rt *Runtime }

func (a *llmActor) HandleActivation(ctx, agentID, wake []Update, memory []byte) ([]byte, error) {
  sess := newSession(envelopeFor(agentID),          // authority envelope, not persona:
                     rebuildContext(memory, wake))  // prompts are obligation-first
                                                    // (role-free actor protocol)
  inputs := renderUpdates(wake)               // updates become the user-turn content
  for {
    // one STEP: one provider call + its tool executions
    out, err := sess.step(ctx, inputs)         // tools may call rt.Send (Cut 3)
    if err != nil { return memory, err }       // backlog replays: at-least-once

    a.markProcessed(consumedUpdates(inputs))   // after the step that saw them completes

    steering := a.drainMailbox(agentID)        // warm deliveries since last step
    switch {
    case len(steering) > 0:
      inputs = renderUpdates(steering)         // injected at the step boundary
    case out.yielded:                          // model stopped calling tools
      if sess.contextLarge() || timeForCheckpoint() {
        memory = compact(sess)                 // compaction INSIDE the activation
      }
      return compact(sess), nil                // passivate (actor core does atomic idle check)
    default:
      inputs = nil                             // model continues its own loop
    }
  }
}
```

Decisions made here, once:

1. **internal/actor's Handler interface upgrades** from per-update to
   per-activation: `HandleActivation(ctx, agentID, wake []Update, memory)
   ([]byte, error)`, with `drainMailbox`/`markProcessed` callbacks exposed by
   the runtime. (The current per-update shape was PR-1 scaffolding; this is
   the v1 shape. The atomic passivation check stays in the actor core,
   unchanged.)
2. **Updates are marked processed after the step that consumed them
   completes.** Crash between step and mark → replay → duplicate visibility.
   Accepted (models tolerate duplicates; ledger effects already committed at
   send).
3. **Memory snapshot = compacted markdown** (the existing CompactRunMemory
   output), not raw context. Context is rebuilt at activation from snapshot
   + rendered wake updates. Compaction also runs *inside* long activations;
   passivation just takes the final one.
4. **Eviction mid-step**: ctx cancellation aborts the provider call; the
   in-flight step's work is lost; unprocessed updates replay on rewake.
   Crash-equivalent, as specced.
5. **A "run" is recorded as an activation record** (start, end, agent,
   trajectory, outcome) for Trace/UI/acceptance — a read model, not a
   control object.

## 3. Cut 2 — settlement as data, evaluated on transitions (M1/M5)

The question: who decides a trajectory is settled, and when? The cut:

**The settlement rule is a declarative predicate stored on the trajectory.
A pure evaluator runs it inside the same transaction as any write that could
change its verdict. Nothing polls.**

```go
// settlement_json, by kind (v1 vocabulary — extend by adding predicates):
//  publication: {"all_of": ["no_open_work_items", "publish_ref_recorded", "edition_updated"]}
//  adoption:    {"all_of": ["no_open_work_items", "promotion_terminal"]}
//  mission:     {"all_of": ["no_open_work_items", "owner_closed"]}

func maybeSettle(tx, trajectoryID) {        // called INSIDE these transactions:
  //   work item closed/cancelled, publish ref recorded, edition updated,
  //   promotion reaching terminal state, owner close
  t := tx.getTrajectory(trajectoryID)
  if t.status != live { return }
  if eval(tx, t.settlementRule) {           // pure reads within tx
    tx.setStatus(t, settled)
    tx.emitEvent(trajectory.settled, t)     // sourcecycled + UI consume this
  }
}
```

Decisions:
1. Predicates are a closed vocabulary evaluated in Go; the *combination* is
   data per trajectory. New trajectory kinds add predicates by code review,
   not by agents inventing rules (rules are safety surface → durable leaf).
2. `no_open_work_items` is the universal core predicate; blockers and
   questions sent as updates with `kind=blocker|question` against a
   trajectory **open an obligation work item** addressed to whoever can
   discharge it — this is what makes "open obligations" queryable and the
   stall detector (`live AND no resident assignee`) a pure query.
3. **sourcecycled's reconcile** (main.go:590) becomes: a processor request is
   complete when its publication trajectories are all `settled` or
   `cancelled`. That's the whole M5 rewire.

## 4. Cut 3 — the transactional send (everything rides on this)

The question: how do messages, work items, evidence, and obligations stay
consistent? The cut:

**One transaction per send: append update (idempotent) + apply its kind's
ledger effect + maybeSettle. Delivery happens after commit.**

```go
func (rt *Runtime) Send(ctx, u Update) error {
  tx := rt.db.Begin()
  appended := tx.appendUpdate(u)            // ON CONFLICT(update_id) DO NOTHING
  if !appended { tx.Rollback(); return nil } // resend: full no-op
  switch u.Kind {
  case assignment:
    tx.insertWorkItem(fromAssignment(u))     // fingerprint-deduped
    tx.claimSlotIfAny(u)                     // (trajectory, slot) atomic claim
  case verification, evidence:
    tx.insertAcceptanceEvidence(u)
    tx.closeWorkItemIfDischarged(u)
  case blocker, question:
    tx.openObligation(u)                     // a work item for the discharger
  }
  maybeSettle(tx, u.TrajectoryID)
  tx.Commit()
  rt.deliver(u)                              // mailbox or activation (actor core)
  return nil
}
```

Decisions:
1. Exactly-once ledger effects, at-least-once visibility — already proven at
   the spec layer; this is its implementation shape.
2. **Slot claim is part of the assignment transaction** — the check-then-act
   race (runtime.go:534) dies here, not in a lock audit.
3. Crash after commit, before deliver: the boot/periodic sweep delivers.
   (Already implemented and tested in internal/actor.)
4. Work-item close and evidence are one transition → settlement evaluation
   is never missed between them.

## 5. Cut 4 — what the route flip physically does (M6)

The question: `RouteProfile` has no consumer; what does "Activate" actually
change? The v1 cut, minimal but real:

**The routes row is the commit point. The proxy serves the UI bundle named
by the route. vmctl applies the runtime artifact by unit restart. Both
follow the committed promotion record; a reconciler finishes either.**

```go
// COMMIT (the atomic flip; one transaction — this IS promotion_protocol's
// commit state, with the shipped approval + freshness guards as preconditions)
func commitPromotion(tx, p Promotion) {
  tx.setPromotionState(p, committed)
  tx.upsertRoute(p.computerID, p.uiArtifact, p.runtimeArtifact, p.promotionID)
}
// SECONDARIES (idempotent; reconciler-driven; safe after crash)
//  ui:      proxy cache invalidation -> serves artifact dir named by routes row
//  runtime: vmctl restarts the computer's runtime unit pointing at the new build
//  index/blobs: alias swaps, as designed
func reconcilePromotions(db) {              // boot + after any commit
  for p in db.promotions(state in {committed, reverted} where !fullyReconciled(p)):
    for ledger, st in p.ledgers: if st == prepared:
      apply/rollback per p.state            // fate read from the commit point alone
}
// SERVE (the proxy change — the actual new consumer)
func serveUI(computerID) http.Handler {
  route := routesCache.get(computerID)      // invalidated by promotion events
  return serveDir(artifactDir(route.uiArtifact))
}
```

Decisions:
1. v1 scope: **UI bundle swap + runtime unit restart**. Not VM re-imaging,
   not NixOS generations — those join as additional ledgers later without
   changing the commit-point shape.
2. The routes row replaces lineage.RouteProfile as the thing consumed;
   lineage remains the bookkeeping record.
3. Health window v1: promotion sits `committed` until the owner confirms in
   the Features app or a basic health probe passes → `confirmed`. AutoRevert
   v1 is manual rollback (already shipped) gated by `window_open` (M8).

## 6. Cut 5 — where Dolt forks and merges (M8)

```go
// at adoption create:   dolt branch candidate/<adoption_id> from active
// candidate work:       writes only on its branch
// at commitPromotion:   in the SAME control flow, before the flip:
merge := dolt.Merge(active, candidate, base=forkPoint)   // cell-level 3-way
if merge.schemaConflicts: block(promotion, conflicts)     // structural: hard block
if merge.dataConflicts:   block(promotion, conflicts)     // v1: ALL conflicts block;
                                                          // resolution policy is an
                                                          // owner decision, not a default
// first N-1-incompatible change (schema contract) after commit:
db.setWindowOpen(p, false)                                // closes the rollback window
```

Decision: v1 blocks on *every* conflict and surfaces it in the Features app
plan view. Auto-resolution policies (active-wins for live data) come only
after the owner has seen real conflict sets and chosen policies — the merge
policy is S5 material, not a code default.

## 7. Event-driven adoption (M4, replacing continuation synthesis)

```go
// Adoption transitions already emit events. The replacement for
// SynthesizeRunContinuation is a 10-line router, not a planner:
func onAdoptionEvent(e) {
  wi := db.workItemThatCreatedAdoption(e.adoptionID)   // provenance from Cut 3
  rt.Send(Update{
    UpdateID:     deterministic(e),                     // idempotent on event id
    ToAgentID:    wi.createdByAgent,                    // the requesting actor decides
    Kind:         status,
    TrajectoryID: wi.trajectoryID,
    Content:      renderAdoptionEvent(e),
  })
}
```

Decision: events route to the **actor that asked for the work** (provenance
recorded at send time), which decides what's next in its own activation. No
priority lists, no hardcoded mission docs, no objective synthesis in Go.
Mission resumption = the owner (or a triage agent with a prompt, not a
policy table) sends an update.

## 8. Cancel, evict, and the stall detector (cross-cutting)

```go
func cancelTrajectory(trajID) {             // replaces CancelRunGraph
  tx: set trajectory cancelled; close open work items (cancelled);
      release slots; maybeSettle
  then: evict resident assignees (crash-equivalent; their backlog includes
        the cancellation update so rewake sees it)
}

func stalled(db) []Trajectory {             // observability, never control
  return db.query(`trajectories live
                   AND open work items exist
                   AND no assignee resident AND no assignee backlog`)
  // surfaced to the owner / triage agent; the system never self-prescribes
}
```

---

## 9. Hard problems answered in advance (the decision register)

| # | Problem | Decision | Edge kept |
|---|---|---|---|
| 1 | LLM loop vs actor turn | activation hosts one session; updates are the only inputs; yield+empty mailbox = passivate | streaming/tool machinery may resist the seam (resource); shim in adapter, never in actor core |
| 2 | when is an update "incorporated" | after the step that saw it completes | duplicate visibility on crash — accepted |
| 3 | what is actor memory | compacted markdown, not raw context | compaction quality bounds rewarm quality (existing eval covers it) |
| 4 | who evaluates settlement | pure predicate inside the writing transaction | closed predicate vocabulary; agents don't invent rules |
| 5 | blockers/questions | open obligation work items via send | obligation inflation → settlement never true; watch in M5 |
| 6 | slot races | claim inside the assignment transaction | none beyond schema |
| 7 | what route flip does (v1) | routes row + UI bundle swap + runtime unit restart | not whole-VM promotion yet; ledgers extend later |
| 8 | promotion recovery | reconciler reads fate from the commit point alone | requires every secondary apply idempotent — review each |
| 9 | dolt conflicts (v1) | all conflicts block, surfaced to owner | merge policy deliberately undecided (S5) |
| 10 | adoption progression | event → update to the requesting actor | if the requester is long-gone, triage agent is the fallback wake |
| 11 | runs | demoted to activation records (read model) | UI/acceptance depending on run semantics migrate with M3 |
| 12 | stall handling | pure query + surface; never synthesized objectives | someone must look; the Features/desk surface owns the list |

## 10. Mission → sections map

| Mission | Implements |
|---|---|
| M1 | §1 (trajectories, work_items), §3, §8 stall query |
| M2 | §1 (slots), §2 handler upgrade, §4, prompt updates |
| M3 | §2, §8 cancel/evict, runs-as-activations |
| M4 | §7 |
| M5 | §3 decision 3 (sourcecycled rewire), maxProc raise |
| M6 | §1 (promotions, routes), §5 |
| M7 | consumes §5 (plan view, window status) + preview wiring |
| M8 | §6, window_open |

Each mission's first control interval: re-read its sections here, then write
its mission doc's conjecture ledger with any disagreement as the first
conjecture.
