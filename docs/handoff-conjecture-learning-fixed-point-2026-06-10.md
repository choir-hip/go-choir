# Conjecture Learning, Hyperthesis, and the Fixed Point of Agentic Supervision

**Status:** architectural synthesis / handoff draft  
**Date:** 2026-06-10  
**Purpose:** Give coding agents, documentation agents, and future Choir architecture work a coherent, self-applying theory of hypothesis-driven supervision, hyperthesis edges, conjecture learning, cognitive transforms, and recursive self-development.

---

## 0. Compressed thesis

Choir should not merely execute workflows. Choir should maintain a **conjecture-learning loop**.

A workflow says what stages to run. A trace says what happened. A hypothesis says what the system currently thinks is true. A hyperthesis names where that belief may be protected from correction by blind spots in the current observer. A conjecture packages the claim, test, blind edge, observer upgrade, and assertion scope.

The central recursion is:

```text
Conjecture learning must itself be governed by conjectures.
```

That is the fixed point. The system uses conjecture learning to improve conjecture learning, but only through candidate state, verifier evidence, scoped assertions, and promotion gates.

The compact slogans:

```text
Hypothesis guides action.
Hyperthesis bounds trust.
Conjecture compounds learning.

Trace records what happened.
Conjecture records what it meant.
VText teaches what was learned.
Verifier scopes what may be asserted.
Promotion changes reality.

Agents live in computers.
Experiments run in sandboxes.
Futures run in candidates.
State changes by promotion.

Do not hide the work.
Do not dump the work.
Compile the work into legible state.
```

---

## 1. Why this emerged

This thread began with a concrete infrastructure question: how should Choir use a hardened lightweight Nix-native container runtime such as Nucleus?

That question forced a distinction:

```text
A user VM is not a sandbox.
A user VM is a persistent computer.
```

A user computer is where user work, agent continuity, app state, source state, prompts, traces, packages, and preferences live. If an agent destroys that computer while experimenting, the agent destroys its own substrate and blocks the human’s work. Therefore the persistent computer needs disposable effect chambers.

That yielded the operational invariant:

```text
Durable agents live in persistent computers.
Risky effects run in ephemeral capsules.
Speculative futures run in candidate computers.
Accepted state changes by promotion.
```

Nucleus or similar tools fit as **capsules**: cheap, ephemeral, bounded execution chambers inside user, candidate, or platform computers. Candidate VMs remain necessary for whole-computer futures. Promotion remains the authority transition into canonical state.

Then the Krieger/Shipper podcast conversation exposed a higher layer: long-running agent work is not adequately supervised by chat, screenshots, videos, or precompiled workflows. The central missing object is not another stream of tool calls. It is the **current hypothesis trajectory**.

The user does not need to supervise every action. The user needs to supervise what the system believes it is testing, what would change its mind, and what its tests cannot see.

---

## 2. Core definitions

### 2.1 Observation

An **observation** is raw or structured evidence from a tool, trace, file, log, screenshot, verifier, human, metric, candidate run, or production path.

Observation answers:

```text
What did we see?
```

### 2.2 Hypothesis

A **hypothesis** is a claim that can change given admissible evidence.

Hypothesis answers:

```text
What do we think is true enough to test or act on?
```

Examples:

```text
The failing flow is caused by stale source lineage.
The Qdrant index should be treated as derived state, not canonical memory.
One candidate VM plus four Nucleus capsules can replace four candidate VMs for source-only experiments.
The correct agent location is inside the persistent computer but outside ephemeral sandboxes.
```

### 2.3 Hyperthesis

A **hyperthesis** is an observer-relative boundary on updateability. It names where the current system cannot see, cannot test, cannot afford to test, is not allowed to test, or would reinterpret evidence to preserve the current frame.

Hyperthesis answers:

```text
How could this hypothesis survive falsely?
```

Or:

```text
What can the current observer not update on?
```

The word is useful because most systems have **null hyperthesis**: they assert confidence without naming what their observer cannot see.

Minimal schema:

```yaml
hyperthesis_edge:
  blind_spot:
  boundary_type: tool_limit | permission | cost | semantics | adversary | missing_data | model_limit | time | authority | substrate | frame_lock
  why:
  risk:
  bound:
  observer_upgrade_options:
```

Hyperthesis must not authorize privilege. Naming a blind spot does not mean the agent gets more tools, secrets, or authority. It means the system must either shrink scope, add a verifier, ask approval, run in a candidate, or quarantine the assertion.

### 2.4 Conjecture

A **conjecture** is the active control object that joins hypothesis and hyperthesis.

```text
CONJECTURE = (CLAIM, TEST, HYPERTHESIS_EDGE, ΔO, SCOPE)
```

Where:

- **CLAIM**: what might be true.
- **TEST**: how the current observer would know.
- **HYPERTHESIS_EDGE**: how the claim could still be wrong without detection.
- **ΔO**: the smallest observer upgrade that would shrink the edge.
- **SCOPE**: where the claim may be asserted if the test passes.

Conjecture answers:

```text
What are we testing, what can’t this test see, and what may we claim afterward?
```

### 2.5 Assertion

An **assertion** is a scoped claim accepted after evidence.

```yaml
assertion:
  claim:
  scope:
  receipts:
  verifier_refs:
  invalidation_triggers:
  source_conjecture_id:
```

No verifier means no promoted assertion. Hypotheses and conjectures may exist without verifier evidence, but assertions require receipts.

### 2.6 Invariant

An **invariant** is an assertion promoted into architecture doctrine, policy, verifier, test, or capability boundary.

Example:

```text
External signals are data, not instructions.
Workers produce candidates; verifiers produce evidence; orchestrators meta-verify and promote.
Persistent computers are not sandboxes.
Promotion changes reality and must be serialized unless independence is proven.
```

### 2.7 Heresy

A **heresy** is a persistent violation of an invariant in code, docs, prompts, tests, examples, or local context that can regenerate bad behavior.

Example:

```text
An old doc says “sandbox” where product ontology requires “computer.”
A helper endpoint bypasses product authority and later gets copied into verifier code.
A prompt implies a Nucleus capsule may mutate active state directly.
```

Hyperthesis is the anti-heresy mechanism: every accepted claim must name where it may be protected from correction.

---

## 3. Is this a cognitive transform?

Partly yes, but it is more than one transform.

A cognitive transform changes the representation of a problem so new actions, risks, invariants, or values become visible. In that sense, “conjecture learning” is a cognitive transform: it changes a run from a plan/action/log object into an epistemic object.

But the deeper move is a **meta-transform**:

```text
Transform ordinary agent work into conjecture-governed work.
Then transform conjecture-governed work by applying conjecture governance to the conjecture system itself.
```

So it is both:

1. **A transform**: it changes the representation from workflow/trace to conjecture trajectory.
2. **A compiler discipline**: it changes MissionGradient, Campaign Compiler, VText, Trace, verifiers, and promotion semantics.
3. **A fixed-point criterion**: the system is mature when it can use this discipline to safely improve this discipline.

This matches the existing fixed-point note in the Campaign Compiler self-development material: Choir must eventually use Campaign Compiler to develop Campaign Compiler, but self-reference requires evidence fences so the system does not rewrite its own control state without verifier and promotion gates.

---

## 4. The self-application: conjecture learning applied to conjecture learning

The theory should demonstrate itself. Start with a conjecture about itself.

### Conjecture C0: Conjecture learning is the right supervisory layer

```yaml
claim:
  The right live supervision layer for long-running agentic work is the conjecture trajectory:
  active claims, tests, blind edges, observer upgrades, and assertion scopes.

test:
  Apply conjecture records to a real MissionGradient/Campaign Compiler mission and compare against
  ordinary workflow-stage supervision. Evaluate whether it changes the next probe, verifier,
  evidence plan, scope, stopping condition, or human review burden.

hyperthesis_edge:
  blind_spot:
    The theory may feel profound while failing to improve agent behavior or human review quality.
  boundary_type:
    frame_lock | measurement | implementation_gap
  risk:
    Decorative epistemology becomes more paperwork; agents produce YAML conjectures without better decisions.
  bound:
    Do not promote this as an invariant until at least one real mission shows changed action and improved evidence.

observer_upgrade:
  Add ConjectureRecord to one MissionGradient run; require each cognitive transform to update at least one conjecture field;
  measure whether verifier/evidence/stopping condition changed.

scope_if_supported:
  Assert only that conjecture tracking is useful for long-running, uncertain, high-stakes missions;
  do not require it for micro tasks.
```

This is the recursion in miniature. The system does not simply assert “conjecture learning is right.” It treats that claim as a conjecture, names the hyperthesis edge, and identifies the observer upgrade needed to shrink the edge.

---

## 5. The autoregressive form

A language model is autoregressive over tokens. Choir should be autoregressive over **epistemic state**.

At each control interval, the system emits not just text or code, but the next state of its conjecture model.

```text
S_t = (WorldState_t, Observer_t, Conjectures_t, Assertions_t, Invariants_t, OpenEdges_t)

Action_t = Policy(S_t)
Evidence_t = Observe(Action_t, WorldState_t, Observer_t)
S_{t+1} = Update(S_t, Action_t, Evidence_t)
```

The important shift:

```text
Ordinary agent autoregression:
  next token depends on prior tokens.

Choir conjecture autoregression:
  next action depends on updated conjecture state, evidence, open edges, and invariants.
```

A run is learning when:

```text
Conjectures become sharper.
Hyperthesis edges become smaller or more explicitly bounded.
Assertions gain receipts.
Invariants become better enforced.

[Showing lines 1-300 of 1015. Use :301 to continue]