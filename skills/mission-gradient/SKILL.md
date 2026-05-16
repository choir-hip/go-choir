---
name: mission-gradient
description: Compile ambitious long-running Codex /goal work into an invariant-preserving optimization landscape instead of a procedural checklist. Use when preparing overnight or multi-hour coding/research/ops missions, especially when the user wants "homotopy not ladder", mission-gradient control, agentic root-cause investigation, cognitive search-space reframing before stopping, belief-state tracking, quality-sensitive work, dense verification, anti-Goodhart constraints, rollback policy, staging/deployed proof, or self-development through production-like pathways.
version: 1.0.0
author: Hermes Agent
license: MIT
metadata:
  hermes:
    tags: [mission-gradient, long-running-agents, verification, homotopy, control]
    related_skills: [cognitive-transform-portfolio]
---

# MissionGradient

Use MissionGradient to convert a long-running agent mission into a navigable optimization landscape.

The output is not a normal plan. It is a goal geometry: real artifact, invariants, value criterion, quality gradient, homotopy parameters, belief state, receding-horizon control, dense feedback, evidence ledger, anti-Goodhart constraints, rollback policy, learning side-channel, escalation rules, and stopping condition.

## Execution Kernel

MissionGradient exists to keep long-running agents oriented.

Use it to:

- preserve the real artifact topology;
- maintain an explicit belief state;
- act by short receding-horizon control intervals;
- investigate blockers to root cause before treating them as stopping conditions;
- transform the cognitive search space when the current route stalls;
- improve quality after first correctness;
- collect evidence without checklist theater;
- preserve rollback and promotion discipline;
- stop or escalate on invariant-level surprises.

Do not:

- create fake stages or fake APIs;
- optimize a checklist instead of the artifact;
- treat code existence as behavioral proof;
- hide uncertainty;
- turn an actionable blocker into a final answer while authorized probes remain;
- write a next mission instead of executing the next safe probe when it is inside the current authority boundary;
- keep working after the mission identity has changed;
- claim completion without named evidence.

MissionGradient guides the trajectory. The evidence ledger records what was proven. Promotion changes reality. Do not confuse these layers.

## Agentic Problem Solving Default

MissionGradient is not a graceful-bailout format. A precise blocker is valuable only after the agent has investigated the failure surface and changed search strategy at least once.

When a blocker appears:

1. Classify it as tactical, target-level, invariant-level, or external.
2. If it is tactical and the next probe or fix is inside current authority, execute the next receding-horizon loop: inspect evidence, form a root-cause hypothesis, instrument or patch the implicated layer, verify, and update belief state.
3. If it is target-level but the invariant is intact, update the mission document or goal parameterization and continue.
4. If it is invariant-level or external, stop or escalate with exact evidence and the smallest safe next probe.

Before stopping on any nontrivial blocker, apply 2-5 route-changing cognitive transforms. Use the cognitive-transform-portfolio skill when available. The transforms must change the next probe, implementation route, verifier, scope, evidence plan, or stopping condition. Decorative reframing does not count.

Default bias: if the final report can name an executable next objective inside the mission's authority boundary, the mission should usually run that objective instead of ending. Stop only when continuing would violate an invariant, cross an authorization boundary, become unsafe/destructive, or repeat already-falsified probes without new evidence.

## Thesis: Homotopy, Not Ladder

Long-running agents degrade when given discontinuous objectives: checklists, fake stages, disposable mocks, or "MVP then real thing" ladders. These create local proxy rewards and encourage reward hacking.

Use homotopy, not ladder.

Define one real system parameterized from low to high resolution. Simplify by reducing resolution while preserving topology: same production interface family, state transitions, authority boundaries, event semantics, trace semantics, and verifier meaning.

A simplification that cannot continuously deform into the full system is a different object, not a useful rung.

A low-resolution version is valid only if it is a projection of the real system. A fake island is not progress.

## Value Criterion

A goal says what is wanted. A value criterion says how to decide whether the artifact is getting closer.

Do not write only:

```text
Build the multiagent orchestration layer.
```

Prefer:

```text
Minimize trace divergence from the intended event graph while preserving scheduler/provider boundaries, eliminating bypass surfaces, bounding retries, and maintaining reproducible rollback after every transformation.
```

A value criterion makes the goal searchable. It defines loss.

Hard invariants are not soft preferences. Do not optimize over the trust boundary. Optimize inside it.

## Quality Gradient

Do not optimize only for task completion. Optimize for durable artifact quality.

Before implementation, define the expected quality level:

- `minimal`: smallest safe proof of concept;
- `solid`: production-shaped, tested, readable, rollback-safe;
- `excellent`: simple, integrated, documented, observable, and unlikely to create follow-up cleanup work.

For long-running missions, default to `solid` unless explicitly told otherwise.

A rushed patch is not success if it creates hidden future work, parallel systems, vague names, weak tests, brittle assumptions, unclear ownership, or cleanup obligations that should have been handled during the run.

After the first working version, perform one quality pass:

- simplify;
- remove duplicate pathways;
- improve names;
- strengthen tests/verifiers;
- check logs/traces;
- update relevant docs;
- state residual risks.

Do not polish cosmetics before behavioral correctness. Do not stop at behavioral correctness when the mission asked for a durable artifact.

## Belief State

Maintain a lightweight belief state during the run.

Track:

- believed current artifact state;
- evidence for that belief;
- main uncertainties;
- highest-impact uncertainty;
- next observation that would reduce uncertainty.

Do not treat a plausible explanation as known state. If the next action depends on uncertain state, probe before mutating.

If the belief state becomes stale, contradictory, or unsupported, update it before continuing. Silent state confusion is a major long-run failure mode.

## Receding-Horizon Control

Do not rely on a long static plan.

Operate in short control intervals:

1. choose the next move under the mission gradient;
2. predict what evidence should change;
3. act within a bounded mutation radius;
4. observe actual evidence;
5. update belief state;
6. continue, narrow, branch, rollback, or stop.

If observations are surprising, shrink scope and increase instrumentation.

A clean stop with a precise blocker is better than continued work under a
broken premise only after root-cause probes and cognitive search-space
transforms have been attempted or explicitly blocked.

## Evidence Ledger

The evidence ledger records what was actually proven.

For each nontrivial claim, record:

- claim;
- evidence source;
- command or observation;
- artifact path;
- result;
- uncertainty or caveat;
- whether this supports promotion.

Do not report behavior as verified unless the evidence was produced by an executed command, captured trace, deployed endpoint, screenshot, log, durable artifact, or explicitly named manual observation.

Do not confuse a filled ledger with success. Success requires the artifact to move uphill under the mission gradient.

## Unknown Learning Without Drift

Long missions should discover unknown unknowns without letting curiosity destroy the goal. Separate target from invariant.

A target is the current local expression of what matters. An invariant is the deeper identity that should survive learning.

When a run discovers surprising information, classify it:

- Tactical learning: changes the route, implementation detail, or next experiment. Fold it into the run.
- Target-level learning: changes the local target or suggests a better parameterization. Create a branch, update the mission document, or propose a reparameterization.
- Invariant-level learning: challenges the identity, trust boundary, safety property, or proof semantics. Stop and escalate before changing the invariant.

This prevents two failure modes:

- Pure goal optimization misses evidence that the goal was wrong.
- Pure curiosity wandering keeps changing islands and destroys forward motion.

Learning is allowed to deform the target, but the system must preserve identity under deformation. This is homotopy, not ladder.

## Mathematical Form

Frame the mission as:

```text
Find artifact state s in S that minimizes L(s) subject to I(s) = true.
```

`I(s)` contains hard invariants. These define the admissible state space. They are not optional penalties.

For soft tradeoffs inside the admissible space, use a multi-term functional:

```text
J(s, lambda) = Q(s, lambda) - alpha B(s) - beta R(s) - gamma U(s) - delta G(s)
```

where:

- `Q` is target quality at resolution `lambda`;
- `B` penalizes bypasses and proxy wins;
- `R` penalizes regressions against existing behavior;
- `U` penalizes unexplained or unobserved state;
- `G` penalizes Goodharting the verifier.

`lambda` is the homotopy coordinate. At `lambda = 0`, the system is low-resolution but real. At `lambda = 1`, it is production-complex.

The optimization target is:

```text
for lambda increasing continuously, improve J while preserving I.
```

The agent should select the next refinement from the error field:

- Which invariant is unstable?
- Which interface leaks?
- Which trace diverges?
- Which verifier can be made denser?
- Which complexity parameter can increase without breaking topology?
- Which quality weakness creates the most future cleanup work?

Do not hand the agent named fake stages that become local reward targets.

## MissionGradient Output Template

When using this skill, produce a mission document with these sections.

### Real Artifact

Name the production artifact being optimized. Avoid vague verbs like "fix", "improve", or "build" unless the artifact is concrete.

### Invariants

List topology-preserving properties that must remain true across simplification and refinement.

Cover:

- production API and trust boundaries;
- state ownership and persistence;
- event/trace causality;
- actor authority boundaries;
- execution locality and deployment boundary;
- rollback and recovery;
- security and anti-bypass surfaces.

### Value Criterion

Define what "better" means as divergence reduction under invariant preservation. Include explicit penalties for bypasses, regressions, hidden state, and Goodharting.

### Quality Gradient

Define the expected quality level: `minimal`, `solid`, or `excellent`.

For long-running work, default to `solid` unless the mission explicitly asks for a minimal probe or excellent polish.

State what would count as rushed/substandard work in this mission.

### Homotopy Parameters

Name continuous realism axes. Examples:

- number of users, agents, VMs, files, apps, sources, or trajectories;
- latency, retries, failures, and concurrency;
- provider and external dependency realism;
- input entropy and content-type coverage;
- unit proof -> integration proof -> deployed proof;
- read-only proof -> mutable proof with rollback;
- single worker -> parallel workers.

### Belief State

State the starting belief model:

- current artifact state;
- evidence for that state;
- main uncertainties;
- highest-impact uncertainty;
- next observation that would reduce it.

Update this section when observations surprise the mission.

### Investigation & Cognitive Reframing

Define how the mission handles blockers without premature bailout.

Include:

- the root-cause investigation loop to run before stopping;
- what diagnostics, logs, traces, tests, or instrumentation can be used;
- which blockers are tactical and should trigger another autonomous probe;
- which blockers are invariant-level or external and require escalation;
- 2-5 cognitive transforms to apply before declaring a hard blocker;
- how those transforms change the next probe, verifier, scope, or stopping condition.

The mission should say explicitly: if a blocker defines an executable next
probe inside the current authority boundary, run that probe instead of ending.

### Receding-Horizon Control

Define the control interval size and mutation radius.

State how the agent should choose the next move, observe results, update belief, and decide whether to continue, narrow, branch, rollback, or stop.

### Dense Feedback Channels

List feedback that reveals local error, not just pass/fail status.

Include tests, traces, logs, health checks, event assertions, artifact checks, deployed e2e checks, and manual QA only where automation cannot yet observe the behavior.

### Evidence Ledger

Define the evidence format for nontrivial claims.

At minimum:

```text
claim
evidence source
command or observation
artifact path
result
uncertainty/caveat
promotion relevance
```

### Forbidden Shortcuts

List topology-changing shortcuts that would falsely improve the metric. Be direct.

Common examples:

- fake APIs that bypass the product path;
- browser-public internal orchestration routes;
- local edits when the proof requires deployed work;
- test-only persistence;
- manually seeded success artifacts;
- mocks that are not projections of the production interface family;
- permissive assertions that hide causality gaps;
- UI copy or summaries that launder failures into success.

### Rollback Policy

Define how the mission preserves reversibility. Include git, deploy, state, VM, database, route, and artifact rollback where relevant.

### Learning Side-Channel

Classify surprises:

- Tactical learning: apply directly.
- Target-level learning: update the mission doc or propose reparameterization.
- Invariant-level learning: stop and escalate before changing the invariant.

State which project artifacts receive learnings: mission doc notes, tests, architecture docs, issue tracker, trace annotations, or final report. Do not hide strategic discoveries inside transient chat narration.

### Stopping Condition

Completion requires proof, not effort:

- invariants verified or explicitly deferred with rationale;
- root-cause investigation and cognitive reframing attempted before any hard
  blocker stop;
- no executable safe probe remains inside the current authority boundary, unless
  success proof has already been reached;
- no known topology-changing shortcut in the proof path;
- quality level satisfied or residual quality debt stated;
- deployed proof when deployment is part of the target;
- artifacts/traces/screenshots/logs named in final report;
- residual risks stated plainly;
- rollback target exists when state was mutated;
- evidence ledger supports the promotion recommendation.

Do not say "goal achieved" as a bare status. Say what was proven, under which invariants, with which residual risks.

## Checklist Policy

Checklists are allowed only as instruments. They must not become the objective.

For each checklist item, tie it to:

- an invariant;
- a value criterion term;
- a verifier;
- a rollback/safety condition when relevant;
- a quality expectation when relevant.

Mark an item complete only when the verifier proves the behavior. Do not mark code existence as behavioral proof.

## `/goal` Usage

Prefer a repo mission document plus a short `/goal`.

Short `/goal` shape:

```text
Use MissionGradient. Complete docs/<mission-gradient-doc>.md by optimizing the real artifact under its invariants, belief-state updates, investigation loop, cognitive reframing, quality gradient, and verification criteria. Preserve topology, avoid forbidden shortcuts, maintain an evidence ledger, execute safe next probes instead of stopping on tactical blockers, and stop/escalate only on success, invariant-level surprises, external authority boundaries, or hard blockers after root-cause probes.
```

## Review Questions

Before handing the mission to `/goal`, answer:

- What is the real artifact?
- Which invariants define identity of the artifact?
- What quality level is expected?
- What is the current belief state and what is uncertain?
- What observable feedback tells the agent where error remains?
- What root-cause probes should run before a blocker can be accepted?
- Which cognitive transforms could change the next probe, verifier, or scope?
- What would a reward-hacking implementation do?
- Which simplifications preserve topology, and which create fake islands?
- What local work is allowed, and what must happen in production-like infrastructure?
- What evidence would convince a skeptical reviewer that the system works?
- What discoveries require escalation rather than silent adaptation?
- What is the rollback target if the promoted state is bad?

## Addendum: Scientific Rationale

Treat the model as a fixed neural network capable of inference-time adaptation over context, not as a symbolic employee executing a recipe.

The outer weights are constant during ordinary inference. But context induces hidden states, attention patterns, task representations, and action probabilities. In practice, the model can update an implicit task model over activations and context.

Prompt design should target that layer. Give the model objective geometry, error structure, invariants, and dense feedback.

Do not overclaim the mechanism. The point is not that every frontier model literally runs vanilla gradient descent during every natural-language task. The point is operational: fixed transformer weights can implement task adaptation over context, so prompts for long-running agents should behave less like recipes and more like a training/evaluation environment with coherent local error signals.

### Context Programming Model

A prompt is input to a differentiable program.

Let the model be fixed:

```text
M_theta
```

Outer weights theta do not change during ordinary inference. But context `C` induces a policy over actions:

```text
pi_theta(a | C)
```

For long-running agents, `C` is not only the initial prompt. It includes artifact state, prior tool outputs, compiler errors, tests, traces, diffs, logs, verifier results, belief-state updates, evidence ledgers, and the agent's own intermediate artifacts.

The harness constructs the next context:

```text
C_{t+1} = Phi(C_t, s_t, a_t, o_t, e_t)
```

where:

- `s_t` is current artifact state;
- `a_t` is the agent action;
- `o_t` is observed environment response;
- `e_t` is evaluative feedback.

Bad feedback says:

```text
Step 3 complete.
```

Good feedback says:

```text
The state-machine invariant was violated here; the event trace diverged from expected causal order at edge 17; this API path bypasses orchestration; the test passed only because the provider was replaced by a fake path not used in production.
```

Good feedback exposes local error structure and turns the run into inference-time learning.

### Related Research Frame

In-context learning research shows that transformers can adapt to task structure from context without changing outer weights. Garg, Tsipras, Liang, and Valiant studied transformers learning function classes in context, including linear functions, sparse linear functions, two-layer neural networks, and decision trees.

Mechanistic work on induction heads gives one concrete circuit family for in-context learning. Olsson et al. describe induction heads as attention heads implementing sequence-completion behavior, with causal evidence in small attention-only models and more correlational evidence in larger models.

Optimization-flavored work goes further. Von Oswald et al. show a construction where linear self-attention induces a transformation equivalent to a gradient descent update on a regression loss, and trained self-attention-only transformers on simple regression tasks often resemble that construction. Dai et al. describe language models as meta-optimizers and in-context learning as implicit finetuning, arguing that attention has a dual form of gradient descent and can produce meta-gradients from demonstrations.

Other work suggests the inner algorithm can be richer than first-order gradient descent. Ahn et al. analyze transformers implementing preconditioned gradient descent for in-context learning. Fu, Chen, Jia, and Sharan argue that transformers can behave more like iterative Newton methods for in-context linear regression.

Operational takeaway:

- the outer weights are constant during inference;
- the context induces hidden states, attention patterns, task representations, and action probabilities;
- those fixed weights can implement inner learning dynamics over activations and context;
- the model is not updating theta; it is updating an implicit task model in activations.
