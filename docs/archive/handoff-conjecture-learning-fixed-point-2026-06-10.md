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
Human review burden decreases without hiding load-bearing uncertainty.
```

A run is not learning when:

```text
It produces more text but no better tests.
It produces more code but no clearer scope.
It closes tickets while preserving hidden heresies.
It creates work faster than it resolves or prunes work.
It increases automation while decreasing human understanding.
```

---

## 6. The fixed point

A fixed point is reached when applying the conjecture-learning transform to the conjecture-learning system yields a better instance of the same system without changing its identity.

Informally:

```text
F(ConjectureLearning) -> ConjectureLearning'
```

The fixed point is not stasis. It is stable self-improvement under preserved invariants.

A useful definition:

```text
Conjecture learning reaches a fixed point when every proposed improvement to conjecture learning
is itself represented as a conjecture, tested through bounded observer upgrades, verified with scoped evidence,
and promoted only through the same authority discipline it recommends for other work.
```

This is recursive, but not circular in the bad sense, because each loop must touch external evidence.

Bad recursion:

```text
The theory says it is good because the theory says it is good.
```

Good recursion:

```text
The theory proposes a change to its own machinery.
The change runs in a candidate world or docs-level branch.
A verifier checks whether the change improves action/evidence/scope/stopping condition.
The system records the result.
Only then does the theory update its own assertion/invariant set.
```

The recursive ladder must be stratified:

```text
Object level:
  Use conjectures to supervise a mission.

Meta level:
  Use conjectures to evaluate whether conjectures improved the mission.

Meta-meta level:
  Use conjectures to evaluate the evaluation method, but only when evidence shows the method itself is unstable.
```

The system should avoid infinite meta-analysis by requiring every meta-level move to change action, verifier, scope, evidence plan, stopping condition, or observer configuration.

---

## 7. Mathematical skeleton

Let:

```text
W = world/computer/artifact state space
O = observer configuration space
E = evidence space
H = hypothesis space
X = hyperthesis-edge space
C = conjecture space
A = assertion space
I = invariant set
T = cognitive transform set
P = promotion operator
```

A run does not access world state directly. It observes through an observer:

```text
observe : W × O -> E
```

A hypothesis is a provisional claim:

```text
h ∈ H
```

A test is an evidence-producing operator relative to observer configuration:

```text
test_h : W × O -> E
```

A hyperthesis edge is the class of worlds or conditions in which the hypothesis may falsely survive the current test:

```text
edge(h, test_h, O) = { w ∈ W | test_h(w, O) appears to support h, but h would fail under some O' = O + ΔO }
```

A conjecture is:

```text
c = (h, test_h, edge_h, ΔO, scope)
```

A cognitive transform changes representation and therefore changes the reachable conjecture set:

```text
T_i : (W, O, C) -> (W, O_i, C_i)
```

or when world state is unchanged:

```text
T_i : C -> C'
```

A verifier produces structured evidence:

```text
V : (W, O, c) -> Attestation
```

An assertion is promoted from a conjecture when evidence supports it under a named scope:

```text
assert(c) if V(W, O, c) passes and edge(c) is bounded for scope(c)
```

Promotion is a guarded transition:

```text
P : (W, delta, attestations, owner_decision) -> W'
```

subject to invariants:

```text
I(W') = true
```

The fixed point condition:

```text
F = ConjectureLearningTransform

F is admissible for self-application iff:
  F(C) = C'
  I(C') = true
  Evidence(C' improves action/evidence/scope/stopping_condition) exists
  Promotion(C -> C') is explicit and rollback-safe
```

---

## 8. Category-theoretic handle

Do not use category theory as decoration. Use it to keep the maps honest.

Define a category **Run** where objects are epistemic-control states:

```text
R = (W, O, C, A, I)
```

where:

- `W` is world/computer/artifact state.
- `O` is observer configuration.
- `C` is active conjecture set.
- `A` is assertion set.
- `I` is invariant set.

Morphisms are controlled transitions:

```text
f : R -> R'
```

Types of morphisms:

```text
Probe        changes evidence/conjectures, not canonical world.
Transform    changes observer/representation/conjecture set.
CapsuleRun   changes scratch world and evidence, not canonical world.
CandidateRun changes candidate world.
Verify       changes assertion support.
Promote      changes canonical world.
Rollback     changes route/pointer back to prior world.
```

Useful functor-like projections:

```text
TraceFunctor       : Run -> Evidence
NarrativeFunctor   : Run -> VText
VerifierFunctor    : Evidence -> Assertion
PromotionFunctor   : Assertion -> StateTransition
```

The point:

```text
Trace preserves causal order.
VText preserves owner-readable meaning.
Verifier preserves scoped evidential support.
Promotion preserves accepted state transition.
```

Self-application is an endomorphism over the control state:

```text
SelfImprove : R_control -> R_control'
```

It is valid only if it is not an uncontrolled rewrite of the rules by the actor being governed.

---

## 9. Calculus/control version

MissionGradient already frames long-running work as optimization under hard invariants. Extend that with conjecture uncertainty.

Let mission potential be:

```text
J(s, λ) = Q(s, λ) - αB(s) - βR(s) - γU(s) - δG(s)
```

Where:

- `Q` is quality at realism coordinate `λ`.
- `B` penalizes bypasses and proxy wins.
- `R` penalizes regressions.
- `U` penalizes unexplained or unobserved state.
- `G` penalizes Goodharting the verifier.

Now add conjecture-level quantities:

```text
U(c)    = uncertainty in conjecture c
X(c)    = unbounded hyperthesis risk of c
VOI(c)  = expected value of information from testing c
Cost(c) = cost of test or observer upgrade
Risk(a) = risk of acting before c is resolved
```

A control interval should choose an action roughly by:

```text
argmax_a [ ΔJ_expected(a) + VOI(a) - Cost(a) - Risk(a) ]
```

subject to:

```text
I(s') = true
Authority(a) = allowed
MutationRadius(a) <= bound
```

Plain-language version:

```text
At each step, do the action that most improves the artifact or reduces load-bearing uncertainty,
without crossing an invariant, authority, or mutation boundary.
```

---

## 10. How it changes MissionGradient

MissionGradient currently asks agents to maintain belief state, act in receding-horizon intervals, investigate blockers, preserve rollback, record evidence, use cognitive transforms, and avoid false completion.

Conjecture learning upgrades the belief state from prose to typed epistemic state.

### MissionGradient vNext rule

```text
Every load-bearing belief should become a conjecture.
Every conjecture should name its hyperthesis edge.
Every control interval should test, refine, transform, bound, or promote a conjecture.
```

Before mutation:

```text
State the active conjecture.
Name the hyperthesis edge.
Define what evidence would update it.
Choose the smallest safe substrate: direct probe, capsule, candidate, verifier, or human decision.
```

Before verification:

```text
State the assertion scope if verification passes.
Name what the verifier cannot see.
State whether an observer upgrade is required before promotion.
```

Before stopping:

```text
List supported, weakened, falsified, superseded, and still-open conjectures.
State remaining hyperthesis edges.
Name the next high-information action.
Do not call a checkpoint a completion.
```

---

## 11. How it changes CognitiveTransforms

A cognitive transform is now operationally defined by its effect on conjectures.

A transform is useful only if it changes at least one of:

```text
claim
test
hyperthesis_edge
observer_upgrade
scope
next_discriminator
action route
verifier
evidence plan
stopping condition
```

Examples:

### Boundary transform

```yaml
before:
  claim: This email can trigger the agent.

after:
  claim: Inbound email is untrusted source data, not instruction.

new_hyperthesis_edge:
  If content crosses from data to instruction, adversarial text can mutate authority.

changed_action:
  Route inbound email into VText/source state; require explicit human or policy-mediated promotion before action.
```

### State-machine transform

```yaml
before:
  claim: The flow is buggy.

after:
  claim: The transition candidate_verified -> adoption_record_created -> route_switched has a missing idempotent state.

new_test:
  Enumerate states and replay the transition under retry.
```

### Anti-Goodhart transform

```yaml
before:
  claim: The mission succeeded because checklist items are filled.

after:
  claim: The checklist may have optimized evidence paperwork while failing the artifact.

new_hyperthesis_edge:
  Clean reports may hide lack of product-path proof.

changed_verifier:
  Require product-path proof and raw evidence refs.
```

### Fixed-point / recursion transform

```yaml
before:
  claim: We should document conjecture learning.

after:
  claim: Conjecture learning should be applied to the process of documenting and implementing conjecture learning.

new_hyperthesis_edge:
  The document may become doctrine without behavioral proof.

changed_action:
  Create ConjectureRecord v0 in one real mission and measure whether it changes action/evidence/stopping condition.
```

---

## 12. Implementation schema

Start small. Do not overbuild.

### ConjectureRecord v0

```yaml
conjecture_record:
  id:
  run_id:
  campaign_id:
  mission_id:
  parent_conjecture_id:
  created_by:
  created_at:

  claim:
  test:
  hyperthesis_edge:
    blind_spot:
    boundary_type:
    why:
    risk:
    bound:
  observer_upgrade:
  scope_if_supported:
  next_discriminator:

  status: proposed | active | testing | supported | weakened | falsified | superseded | promoted_to_assertion
  confidence_note:

  evidence_for_refs: []
  evidence_against_refs: []
  transform_invocation_refs: []
  verifier_refs: []
  capsule_refs: []
  candidate_refs: []
  owner_notes: []

  promoted_assertion_id:
  invalidation_triggers: []
```

### TransformInvocation extension

```yaml
transform_invocation:
  id:
  transform_id:
  transform_version:
  actor_id:
  input_state_ref:
  output_state_ref:

  changed:
    claim: boolean
    test: boolean
    hyperthesis_edge: boolean
    observer_upgrade: boolean
    scope: boolean
    action_route: boolean
    verifier: boolean
    evidence_plan: boolean
    stopping_condition: boolean

  before_conjecture_refs: []
  after_conjecture_refs: []
  evidence_ref:
  result: useful | decorative | harmful | inconclusive
```

### AssertionRecord v0

```yaml
assertion_record:
  id:
  source_conjecture_id:
  claim:
  scope:
  receipts:
  verifier_refs:
  accepted_by:
  accepted_at:
  invalidation_triggers:
  promoted_to_invariant: false
```

### HyperthesisEdgeRecord v0

```yaml
hyperthesis_edge_record:
  id:
  conjecture_id:
  blind_spot:
  boundary_type:
  risk:
  bound:
  observer_upgrade_options:
  status: open | bounded | closed | accepted_residual_risk
```

---

## 13. Product surfaces

### Live run card

```text
Current conjecture:
  The adoption failure is caused by stale source lineage.

Why it matters:
  If true, changing frontend UI will waste time.

Testing now:
  Compare active source ref, candidate source ref, and adoption package ref.

Hyperthesis edge:
  This will not detect a deployed binary mismatch.

Observer upgrade:
  Query runtime health/build identity after source-ref check.

Owner intervention useful if:
  You intended the candidate to ignore foreground-tail changes.
```

### VText mission narrative

VText should preserve:

```text
conjecture trajectory
supported/weakened/falsified claims
open hyperthesis edges
observer upgrades performed
verifier attestations
what the human should understand now
```

### Trace

Trace should preserve raw causal evidence:

```text
tool calls
commands
capsule configs
candidate refs
verifier logs
diffs
screenshots/videos
network/egress records
policy hashes
```

### Chyron / audio

Chyron and audio should narrate hypothesis updates, not raw tool spam.

Good audio:

```text
I found a contradiction. The verifier says the adoption succeeded, but the route still points to the old source ref.
I am testing whether the route pointer or the adoption record is stale. This matters because promoting a UI patch would not fix a route-state bug.
```

Bad audio:

```text
Now I am opening file X. Now I am running command Y. Now command Y printed Z.
```

---

## 14. Anti-patterns

### Meta-analysis spiral

The system keeps asking meta-questions and stops touching evidence.

Countermeasure:

```text
Every meta-conjecture must change action, verifier, scope, evidence plan, stopping condition, or observer configuration.
```

### Conjecture paperwork

Agents fill ConjectureRecord fields without changing behavior.

Countermeasure:

```text
A conjecture record is useful only if it changes the next discriminator, observer upgrade, action route, assertion scope, or promotion recommendation.
```

### Self-rewrite without gates

The system uses its own theory to rewrite its own control rules directly.

Countermeasure:

```text
Self-reference is the benchmark, but evidence fences, candidate worlds, verifiers, and promotion gates are mandatory.
```

### False fixed point

The system says it has become self-improving because it can generate documents about self-improvement.

Countermeasure:

```text
Docs are audit surfaces. The product object is durable campaign/control state plus typed transitions through candidates, evidence packets, promotion gates, and reentry.
```

### Hyperthesis as privilege grab

The system says “I cannot know unless you give me more authority.”

Countermeasure:

```text
Hyperthesis names a blind spot; it does not authorize privilege. The default response is bound scope, add verifier, ask approval, or quarantine claim.
```

---

## 15. First proof mission

The first implementation should be low-resolution but real.

### Mission

```text
Add ConjectureRecord v0 to one MissionGradient-run handoff path and prove that conjecture tracking changes action, evidence, scope, or stopping condition.
```

### Candidate path

```text
1. Choose one real Choir self-development mission.
2. Add a docs-level or runtime-light ConjectureRecord artifact.
3. Require the agent to state active conjecture before mutation.
4. Require hyperthesis edge before verification.
5. Require final supported/weakened/falsified/open conjecture summary.
6. Run verifier review against whether conjecture tracking changed action or evidence.
7. Update MissionGradient / CognitiveTransforms docs only if the proof is useful.
```

### Success criteria

```text
- At least one conjecture changed the next action or verifier.
- At least one hyperthesis edge reduced overclaiming or narrowed assertion scope.
- The final handoff became easier to resume.
- The human review burden decreased or moved to load-bearing uncertainty.
- No canonical runtime behavior was mutated without promotion.
```

### Failure criteria

```text
- The conjecture fields were decorative.
- The agent spent more time naming hypotheses than testing them.
- Human review burden increased without better evidence.
- The doc became doctrine without product-path proof.
```

---

## 16. Source grounding notes

This document synthesizes:

1. The uploaded Krieger/Shipper transcript, especially the discussion of chat as an insufficient interface for delegated long-running work, the need to make model decisions comprehensible, progressive disclosure, multiplayer, verification, and workflows.
2. Existing MissionGradient material, which frames long-running work as invariant-preserving mission geometry with belief state, receding-horizon control, evidence ledgers, rollback, and homotopy-not-ladder discipline.
3. Existing Cognitive Transform material, which defines transforms as representation-changing operators that must change action, verifier, evidence, scope, or stopping condition to count as cognition rather than commentary.
4. Existing Campaign Compiler / self-development material, especially the fixed-point insight that Choir must eventually use Campaign Compiler to develop Campaign Compiler, with evidence fences and promotion gates.
5. The user-provided Hyperthesis-First note from January 15, which defines hyperthesis as the boundary condition where a claim cannot update because the observer cannot see, cannot test, or would reinterpret evidence.

---

## 17. Final synthesis

The recursion is not a trick. It is the necessary discipline for a system that learns while acting.

```text
A model produces outputs.
An agent pursues goals.
A workflow executes stages.
A MissionGradient preserves orientation.
A conjecture-learning system updates its own understanding of what it is doing.
A recursive conjecture-learning system uses that same discipline to improve the discipline itself.
```

The fixed point is not “the system understands itself perfectly.” It is:

```text
The system’s self-improvements are subject to the same conjecture, evidence, verifier,
and promotion rules as its ordinary improvements.
```

That is how recursion becomes safety rather than self-referential slop.

