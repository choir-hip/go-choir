# specs/ — model-checked designs

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

## 1. `actor_protocol.tla` — how one runtime delivers messages to agents

**The story.** Agents are durable actors: a goroutine with a Go-channel
mailbox while awake, a compacted memory snapshot plus a durable message log
while asleep. The spec models one runtime process and answers: *can a message
ever be lost?*

**The board:**
- `log[a]` — messages durably written for agent *a* (survives crashes; **the truth**)
- `processed[a]` — messages *a* has durably finished incorporating
- `resident` — which agents currently have a live goroutine (volatile)
- `mailbox[a]` — messages handed to the live goroutine (volatile; just a
  delivery vehicle, always rebuildable from `log minus processed`)

**The moves:**
- **Send** — durably append to the log (any ledger effect — work item,
  evidence — commits in the same transaction, so effects are exactly-once),
  then: recipient awake → drop into its mailbox (steering); asleep → wake it.
- **Sweep** — any sleeping agent with unprocessed backlog may be woken. One
  rule covers boot recovery, re-wake after eviction, and the crash window
  where a send hit the log but died before delivery.
- **Process** — an awake agent incorporates one message and durably marks it.
- **Passivate** — graceful sleep, allowed **only** when there is no
  unprocessed backlog. This check is atomic — the whole game.
- **Evict** — forced sleep at any moment, *without* the idle check: memory
  pressure, shutdown, or a future lease policy. Deliberately identical to a
  one-agent crash.
- **Crash** — the process dies; everything volatile vanishes; the log stays.

**The rules that hold (verified, 3,016 states small / 218,055 large):**
- *No lost wake* — every logged message is eventually processed, through any
  combination of crashes, evictions, and sleeps.
- *Mailbox soundness* — nothing exists only in a mailbox; nothing already
  processed is redelivered through one.
- Visibility is **at-least-once** (a crash between incorporating and marking
  replays the message — models handle a duplicate fine); effects are
  **exactly-once** (committed with the append).

**What TLC caught when we sabotaged it:**
- Weaken passivation to check only the mailbox → an agent woken *cold* (its
  message is in the log, never in the mailbox) goes back to sleep with work
  pending. Lost forever.
- Remove the sweep → messages appended just before a crash strand forever.

**Design dividend:** liveness only holds because evictions are *bounded* —
endless evict/re-wake cycles without processing is a livelock. In the
implementation that bound is the **per-owner activation cap**, which is
therefore load-bearing for correctness, not just cost policy.

**What the Go implementation must honor:** dedupe sends on `update_id`; hold
the registry lock across {residency check + mailbox send} and across
{idle check + deregister}; run the sweep on boot; keep activation caps.

---

## 2. `actor_protocol_xvm.tla` — the same guarantee across two VMs

**The story.** super lives on the active computer's runtime, vsuper on the
candidate computer's. Between them: HTTP, which can drop messages, and two
processes that can each crash. Can a cross-VM message be lost?

**The new piece: the transactional outbox.**
- A cross-VM send appends `<message, destination>` to a durable **outbox** on
  the sender's VM — in the same transaction as the sender's ledger effects.
  Nothing touches the network yet.
- A **forwarder** puts copies on the wire and retries forever — safe, because
  the receiver dedupes on `update_id`, so duplicates are no-ops.
- The receiver runs its normal local send (spec 1) on arrival: durable
  append, wake or steer.
- The sender clears the outbox entry **only after confirming the message is
  durably in the receiver's log**. That one guard is the entire safety story.

**The rules that hold (verified, 8,834 states):** everything from spec 1 per
agent, plus *NetworkCovered* — every in-flight message is still retryable
from some durable outbox (or already received), so a network drop is always
recoverable — and *every committed cross-VM message is eventually processed
at its destination*, through drops, duplicate deliveries, and either VM
crashing.

**What TLC caught when we sabotaged it:** acknowledge on *send* instead of on
*durable receipt* (the classic distributed-systems mistake) → caught in
**4 states**: send → forward → premature ack → the in-flight copy now has no
durable backing; one drop and it is gone forever. Notably the safety
invariant caught it before the liveness check even ran.

---

## 3. `wire_pipeline.tla` — the redesigned news system's own logic

**The story.** One layer up: assume messages reach agents (spec 1/2 proved
it) and check the Universal Wire redesign — durable publication trajectories
replacing in-run decisions. Both of last week's production incidents are
encoded as rules and reproduced as counterexamples from sabotaged guards.

**The board:** items (source articles arriving from cycles, where two items
may be the same underlying story), each story's trajectory stage
(none → drafting → published → settled), and the public **edition** (front
page list).

**The moves:** fetch an item; a processor then makes a *durable* decision —
**open** a trajectory (new story), **attach** to one in flight (duplicate),
or **suppress** as already covered, *allowed only against the published
corpus*; draft work publishes; the edition updates; settlement closes a
trajectory only when it is published *and* listed; a draft chain may be
abandoned (evicted VText work, bounded); an abandoned trajectory can be
**reopened** from its durable items — this layer's sweep, possible only
because the open-decision outlives the run that made it.

**The rules that hold (verified, 412 states, full concurrency):**
- *SuppressedImpliesPublished* — "already covered" may only ever mean
  covered by **published** truth, never by an unpublished draft.
- *EditionHonest* — the front page lists only published stories. The list
  never lies.
- *SettledSound* — settlement is earned: published and listed.
- *EveryItemSettles* — every fetched item's story eventually settles, with
  processors running fully in parallel. This is the property `maxProc=1`
  was protecting by serialization; it holds here under arbitrary concurrency
  because decisions are durable state — the formal case for retiring the
  stopgap.

**What TLC caught when we sabotaged it:**
- Let the coverage check see drafts → the **empty-front-page incident**
  (commit f44065ed) reproduced in 5 states: a duplicate item suppressed as
  "covered" against an unpublished draft.
- Drop the publish guard on edition updates → the **list/open split-brain**
  reproduced in 4 states: a drafting story on the public front page.

---

---

## 4. `promotion_protocol.tla` — promoting a candidate into canonical state

**The story.** Agents build changes in a candidate; promoting them into the
user's real computer must be atomic, approved, fresh, and reversibly safe.
The spec models one promotion across several ledgers (source ref, app data,
derived index) with a single commit point.

**The shape (from the research synthesis):** each ledger *prepares*
durably and inertly (`none → prepared → applied | rolled_back`); one tiny
atomic flip — the commit point, which is also the visibility gate (route
pointer) — decides the outcome; secondaries are reconciled afterward by
reading the commit point alone, so a crashed promotion is always finishable.
Before the flip: abort is always safe. After: forward recovery only, with a
post-commit health window ending in Confirm or AutoRevert (the Android A/B
pattern). The foreground keeps changing during candidacy, so Commit requires
a freshness check (the candidate was prepared against the current state —
else restage, which voids verification AND approval). A "poisoned write"
(new version writes data the old can't read) closes the rollback window;
reverting after that is the torn-rollback bug.

**The rules that hold (verified, 405 states):** secondaries never lead the
commit point; settled promotions are uniform across ledgers; no commit
against a moved foreground; nothing becomes visible without owner approval
of *this* staging; no revert after the window closes; no promotion hangs
half-reconciled.

**What TLC caught when we sabotaged it — two of three are TODAY'S CODE:**
- Drop the freshness check → `NoStaleCommit` violated. This is the current
  `PromoteAppAdoption`, which records foreground drift but never checks it.
- Allow commit from `verified` without approval → `ApprovalGate` violated.
  This is also current behavior: `owner_approved` is a dead status nothing
  produces.
- Allow revert after a poisoned write → `RevertSafety` violated (the
  blue-green torn-rollback).

So this spec doubles as a formal certification that the current
implementation violates its own intended protocol in two specific ways —
both cheap to fix. Design doc:
`docs/choir-promotion-protocol-conjecture-2026-06-11.md`.

---

## The layering

```
wire_pipeline.tla        the business logic is sound
promotion_protocol.tla   state changes are atomic, approved, reversible
       ↑ assume
actor_protocol_xvm.tla   messages survive the VM boundary
       ↑ extends
actor_protocol.tla       messages reach agents at all
```

Each future subsystem (adoption state machine, other trajectory kinds) gets
its own module at the right layer. Every PR that changes protocol behavior
changes the spec first; TLC runs in CI so the specs stay load-bearing.

## Sabotage catalog (proof the checker has teeth)

| Sabotage | Caught by | States to counterexample |
|---|---|---|
| passivation checks mailbox only | liveness (lost wake) | livelock trace |
| no boot sweep | liveness (crash-window strand) | short trace |
| premature ack | NetworkCovered (safety) | 4 |
| coverage check sees drafts | SuppressedImpliesPublished | 5 |
| edition lists unpublished | EditionHonest | 4 |
| no freshness CAS at promote (= today's code) | NoStaleCommit | short trace |
| promote without owner approval (= today's code) | ApprovalGate | short trace |
| revert after poisoned write | RevertSafety | short trace |
