# Choir Refactor Conjecture Program — 2026-06-11

## Status

Discussion checkpoint and design brief for tomorrow's docs/architecture revision.

This document is not a proof artifact. It is a **conjecture-program artifact**: a compiled statement of what we now think Choir is, where the current abstractions are leaking, what architectural changes are likely next, and which open questions still need careful work before implementation.

Companion handoffs imported into `docs/`:

- `docs/handoff-grand-synthesis-2026-06-10.md`
- `docs/handoff-hybrid-computer-capsule-architecture-2026-06-10.md`
- `docs/handoff-conjecture-learning-fixed-point-2026-06-10.md`

This synthesis adds one major new lesson learned from the Universal Wire debugging:

> **parent/child run trees are not the right primary causality model for shared-artifact coagent work.**

That realization should shape tomorrow's architecture revision.

---

## 0. Compressed thesis

Choir should be treated as a **self-improving private cloud for high-context work** whose control object is not a checklist, not a chat log, and not a raw trace, but a living conjecture program.

At the systems level, Choir should converge on this stack:

```text
conjecture-led supervision
-> persistent autoputers as durable seats of agency
-> nucleus capsules as ephemeral effect chambers
-> candidate computers as speculative futures
-> corpusd as durable public/published corpus state
-> VText as artifact truth
-> trajectory/channel-based causality instead of rooted child trees
-> promotion / mutation transaction gates for state change
```

The system should become easier to understand because its ontology becomes cleaner:

- **autoputer** = durable computer / seat of agency
- **capsule** = ephemeral execution chamber
- **candidate** = speculative future autoputer
- **corpusd** = durable publication corpus service
- **VText** = canonical authored artifact substrate
- **trajectory** = coagent work over a shared artifact/channel/candidate

---

## 1. Conjecture framing

This document is organized as:

- **Hypotheses** — what we think is likely true
- **Hypertheses** — what the current observer may still be blind to
- **Conjectures** — bundled claims + tests + blind edges + observer upgrades + scopes

The point is to make tomorrow's planning resilient to the same kind of confusion that slowed the Wire work.

---

## 2. Core architectural conjectures

### Conjecture A — persistent computers are not sandboxes

**Claim**

What Choir currently calls `sandbox` is usually a durable computer, not a disposable sandbox.

**Why we think this**

The object in question has:
- durable app/runtime state
- durable prompts and traces
- source/build state
- long-lived agent continuity
- route identity
- restart/recovery semantics

That is the ontology of a computer, not a throwaway sandbox.

**Implication**

Rename:
- `sandbox` -> `autoputer`

This rename is not cosmetic. It is an ontology repair.

**Hyperthesis edge**

We may still be conflating two different things:
- persistent user/platform computers
- ephemeral execution sandboxes

So the rename only helps if we also make the execution substrate distinction sharper.

---

### Conjecture B — we need explicit capsules for ephemeral execution

**Claim**

Choir should adopt nucleus sandboxes/capsules as the standard ephemeral execution chamber inside autoputers and candidate computers.

**Why we think this**

The hybrid-compute handoff is persuasive on this point:
- durable agents need durable computers
- risky execution should not mutate the durable seat of agency directly
- capsules are the right place for bounded execution, verification, parsing, rendering, and throwaway probes

**Implication**

Tomorrow's architecture revision should distinguish:
- autoputer (persistent)
- candidate autoputer (persistent, speculative)
- nucleus capsule (ephemeral)

**Hyperthesis edge**

If capsules are allowed to become semantically authoritative rather than execution-bounded, we recreate the same confusion at a new layer.

---

### Conjecture C — `corpusd` is the wrong name for the durable corpus service

**Claim**

`corpusd` should be renamed `corpusd`.

**Why we think this**

The current name conflates:
- platform computers / platform VM ownership
- durable published corpus state

But these are different things.

`corpusd` says what the service actually is:
- durable publication corpus
- public durable VText store
- persistent publication truth distinct from the platform computer doing the work

**Implication**

Rename:
- `corpusd` -> `corpusd`

**Hyperthesis edge**

Renaming alone can produce false clarity if we do not also clean up the actual read/write responsibilities and APIs.

---

### Conjecture D — parent/child is the wrong primary causality model

**Claim**

Parent/child run relationships should not be the primary causality or liveness model for shared-artifact coagent work.

**Why we think this**

Universal Wire gave us the decisive trace:

```text
processor
-> vtext
-> processor completes
-> super-owned continuation on same channel
   -> vtext
-> corpusd still zero
```

That means:
- root run completion does not mean work completion
- the actual work continues through a shared artifact/channel/trajectory
- a rooted run tree is insufficient as a control invariant

**What parent/child may still be good for**

- provenance
- debugging
- cancellation cascades
- bounded delegation cases

**What it should stop doing**

- primary publication-progress invariant
- primary liveness boundary
- primary coagent coordination abstraction

**Replacement direction**

Use one or more of:
- `channel_id`
- `trajectory_id`
- candidate/publication ids
- artifact-scoped liveness

**Hyperthesis edge**

We could overreact and remove useful provenance/cancellation structure before the replacement model is defined clearly.

So: demote first, then replace deliberately.

---

### Conjecture E — VText delegation scope is too broad

**Claim**

VText is currently allowed too much delegation latitude.

**Desired rule**

- VText -> researcher for evidence / factual / current knowledge work
- VText -> super only for real coding / execution / privileged work
- VText should not route to `co-super` or `vsuper`
- VText should not invoke vague “general continuation/orchestration” behavior

**Reason**

The phrase “general continuation/orchestration” is semantically empty unless we can answer:
- continuation of what?
- orchestration of what?
- over which artifact?
- under which authority?

When those answers are fuzzy, abstraction leaks follow.

**Implication**

Tomorrow's architecture revision should tighten:
- VText tool scope
- VText prompt guidance
- super / researcher / co-super / vsuper boundaries

**Hyperthesis edge**

If we remove VText escape hatches too aggressively, we may break legitimate workflows that really do need bounded privileged help.

---

### Conjecture F — Universal Wire exposed an over-coupled processor design

**Claim**

The current processor run shape is too monolithic.

Right now one processor may be expected to do all of:
- ingest source items
- dedup against coverage
- decide whether to publish
- decide whether VText should spawn
- potentially fetch more evidence
- potentially coordinate downstream work

That is too much semantic authority in one step.

**Realest decoupled pipeline preserving topology**

```text
source fetch
-> normalized source facts / source items
-> processor evidence pass
-> durable candidate story ledger on platform autoputer
-> coverage / dedup against published corpus only
-> publication-candidate selection
-> VText article spawn or revision
-> autonomous publish to corpusd
-> durable Wire edition update
-> public stories list / headline open
```

**Why this is the realest cut**

It does not create a second article truth.

- candidate ledger = pre-article planning state
- VText = article truth
- corpusd = durable public publication truth

**Hyperthesis edge**

If the candidate ledger starts behaving like a second article substrate, we have reproduced the same ontology leak under a new name.

---

### Conjecture G — MissionGradient should be conjecture-native

**Claim**

MissionGradient should be upgraded so conjectures are first-class rather than manually bolted on.

**Why**

What actually happened in the Wire debugging was not checklist execution. It was:
- conjecture
- falsifier
- new evidence
- updated conjecture
- branch outcome

So the system should explicitly carry:
- conjecture ledger
- strongest evidence
- next falsifier
- hyperthesis edge
- outside-the-envelope blind spots
- branch outcomes

**Implication**

MissionGradient should evolve from a plan-control artifact into a conjecture-control artifact.

**Hyperthesis edge**

A conjecture-rich mission format can still degenerate into decorative epistemology if it does not materially change action, verifier choice, or stopping conditions.

---

## 3. Operational lessons from the Universal Wire mission

### Lesson 1 — transport bugs were real, but not the final truth

We fixed multiple real substrate bugs:
- stale dispatch path
- missing publish URL
- wrong desktop resolution
- guest inability to reach host proxy on TAP 8082
- queue accounting freezes
- guest-local list/open split-brain

These mattered. But after enough substrate fixes, the remaining blockers were not more transport issues. They were semantic / causality / lifecycle issues.

### Lesson 2 — honesty matters before completeness

The list/open split-brain bug was important because it made the UI lie.

Making the list honest was a correct intermediate move even though durable publication was still broken.

### Lesson 3 — queue bookkeeping and root-run completion are not enough

We observed directly that:
- a processor could complete
- yet the publication chain could continue through super/VText on the same document channel

That falsifies root-run completion as a safe liveness boundary.

### Lesson 4 — published-only corpus search is necessary but not sufficient

The processor had been polluted by guest-local unpublished article revisions and could conclude “already covered.”

That needed fixing — and was fixed — but it was not the whole story.

### Lesson 5 — the system needs durable candidate/publication state

The current architecture has too many semantic decisions embedded in transient processor completions. Durable candidate/publication state is needed so decisions become inspectable and re-runnable.

---

## 4. Mutation / transaction open questions

These remain hard and unresolved.

### Question 1 — what is the transaction unit?

Is a transaction over:
- one file?
- one document revision?
- one app state mutation?
- one autoputer route switch?
- one publication trajectory?

Likely answer: different transaction classes, not one universal unit.

### Question 2 — how do we represent active liveness durably?

Universal Wire suggests we need durable answers to:
- which publication trajectory is still live?
- which artifact is still unsettled?
- what descendant/coagent work is still part of the same trajectory?

### Question 3 — what can be rolled back cleanly?

Need explicit rollback classes for:
- route switches
- VText publication
- candidate adoption
- derived index swaps
- generated artifact updates

### Question 4 — how do we keep capsules from becoming authority leaks?

Capsules should be effect chambers, not hidden promotion pathways.

---

## 5. Future complexity that still fits this architecture

### Vector index service
Likely candidates:
- Qdrant
- Lance-backed service
- maybe both eventually

Invariant:
- derived index, not canonical truth

### More data sources
High-value next source families:
- Asian social/media aggregators
- govt / policy
- macro / econ / financials
- markets
- crypto / prediction markets
- space data

These expand the source substrate but do not change the core truth model.

### Slides app / computational cinematography
Strong architectural direction:
- slide deck = V(Text) + transcluded artifacts + rendering script
- viewer app is cheap projection
- creation stays in VText
- later: animation, transitions, frame interpolation, videogen

Invariant:
- generated visual media always labeled as generated

---

## 6. Tomorrow's recommended sequence

1. **Revise docs / architecture first**
   - ontology
   - causality model
   - authority boundaries
   - transaction questions
   - conjecture-native MissionGradient

2. **Clarify naming system**
   - autoputer
   - capsule
   - candidate
   - corpusd
   - trajectory

3. **Define parent/child demotion plan**
   - what it still does
   - what replaces it
   - how migration happens safely

4. **Tighten VText delegation scope**
   - researcher vs super
   - no co-super / vsuper routing from VText
   - no vague continuation semantics

5. **Design candidate/publication ledger**
   - durable pre-article planning state
   - no second article truth

6. **Then implement**
   - smallest/highest-signal pieces first

---

## 7. Conjecture snapshot

### Active hypotheses
- the old parent/child control model is leaking in multi-agent publication work
- VText delegation scope is too broad
- a durable candidate/publication ledger is needed
- MissionGradient should become conjecture-native

### Active hypertheses
- we may remove parent/child too aggressively before replacing the useful bits
- we may turn candidate state into a second article truth by accident
- we may overconstrain VText and lose legitimate execution routes
- we may confuse ontology cleanup with architectural resolution

### Highest-value falsifiers
- does fresh trajectory-scoped accounting eliminate the Universal Wire liveness leak?
- does tightening VText delegation remove the super-owned continuation pattern?
- does candidate-ledger introduction make publication decisions legible without duplicating article truth?

---

## 8. Final synthesis

The big picture now looks like this:

Choir should become a conjecture-led private cloud whose durable seats of agency are autoputers, whose risky execution happens in capsules, whose future states live in candidates, whose public durable publication lives in corpusd, whose article truth remains in VText, and whose multi-agent causality is modeled as coagent trajectories rather than rooted child trees.

Universal Wire did not just reveal bugs. It revealed where the current abstractions are lying.
