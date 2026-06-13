# Choir Agentic Depth: Sweeps, Flys, and Cycles

**Status:** canonical architecture vocabulary, pre-Parallax terminology
**Last updated:** 2026-06-13
**Scope:** campaign compiler, agentic run geometry, mission bags, sweeps, flys, cycles, and verification roles

This document names the core run geometry for Choir.

2026-06-13 note: this document predates Parallax. For new broad missions, use
Parallax/paradocs as the active mission discipline and read `MissionGradient`
below as the older orientation-field vocabulary retained for continuity.

Choir should not be built primarily around goals. Goals are useful, but they are too discrete to serve as the top-level ontology for long-running agentic work. Real work accumulates unordered small tasks, partial repairs, research questions, speculative branches, stale docs, weak tests, ambiguous signals, and opportunities for improvement. These do not naturally form one clean goal. They form a bag.

The earlier core shift was:

```text
Do not batch tasks into goals.
Put work in a bag and sweep it under a mission gradient.
```

That remains true for execution, but the Campaign Compiler work adds the layer
above it:

```text
Do not treat prompts as the top-level object.
Attach intent to durable campaigns, then compile bounded missions.
```

A goal completes. A mission has a stopping condition. A campaign has durable
identity. A sweep improves a region. A fly runs a bounded campaign interval. A
cycle keeps the system alive.

## Core Thesis

Agentic depth is the ability of a system to sustain useful agency over longer horizons while preserving state, evidence, quality, human attention, and promotability.

Agentic depth is not raw autonomy. It is not more agents. It is not bigger context. It is not a longer checklist. It is the ability to keep moving uphill without drifting, reward-hacking, corrupting canonical state, or consuming too much human crystal attention.

The primary primitives are now:

```text
Campaign -> MissionCompiler -> MissionGeometry -> MissionGradient
-> MissionBag -> Sweep -> Fly -> Cycle
```

Goals remain as item types inside a bag. They do not define the architecture.

## Campaign Compiler Layer

Campaign Compiler is the Choir-native control layer that sits above
MissionGradient. It is not a Codex `/goal` improvement project.

Campaign Compiler turns raw user intent, campaign pressure, external signals,
and prior evidence into executable mission state:

```text
Intent / Signal / Campaign Pressure
-> Campaign
-> Mission Compiler
-> MissionGeometry
-> WorkOrders
-> EvidencePackets
-> VText / Trace / Promotion / Reentry
```

Campaign Compiler should choose cognitive transforms by default, not only when
the user remembers to request them. A transform invocation is useful only when
it changes the route, verifier, evidence plan, scope, capability envelope, or
stopping condition.

This layer is required for Choir-in-Choir because the target system is not a
single long run. It is a 24/7 multiagent, multi-computer, multi-runtime,
multi-user, multi-perspective system that preserves campaign identity while
bounded missions start, stop, fail, promote, roll back, and reenter the human.

The negative rule:

```text
Do not implement Campaign Compiler as mission-document generation.
```

Mission documents are audit surfaces. The product object is durable campaign
state plus typed transitions through candidate worlds, evidence packets,
promotion gates, and reentry.

## Definitions

### Campaign

A Campaign is a durable objective field. It persists across many missions,
users, computers, agents, failures, promotions, publications, and reentries.

A campaign owns:

- standing objective;
- standing invariants;
- current belief state;
- open MissionBags;
- active and historical MissionGeometry records;
- default cognitive transforms;
- evidence and Trace refs;
- promotion and rollback refs;
- reentry policy;
- human attention requests;
- cycle health when always-on behavior exists.

Campaigns are continuous, but they do not authorize unbounded work. Bounded
missions, capability envelopes, evidence packets, and promotion gates keep the
campaign from becoming ambient slop.

### MissionCompiler

MissionCompiler transforms raw intent or campaign pressure into executable
MissionGeometry. It estimates mission mass, selects cognitive transforms,
chooses a semantic/register style appropriate to the actor, emits capability
envelopes, creates WorkOrders, defines evidence contracts, and chooses reentry
policy.

The compiler should scale down as well as up. Micro tasks should remain light.
Promotion or source-mutation missions should be strict.

### MissionGeometry

MissionGeometry is the compiled bounded mission object: real artifact,
objective, progress criterion, invariants, capability envelope, selected
transforms, coagent topology, WorkOrders, feedback channels, rollback policy,
and stopping condition.

MissionGradient is the local orientation field inside this geometry.

### MissionGradient

A MissionGradient tells agents what counts as moving uphill.

It defines:

- the direction of improvement;
- hard invariants;
- evidence standards;
- quality standards;
- stop conditions;
- forbidden fake-progress attractors.

A MissionGradient is not a checklist. It is not a destination. It shapes the agent's background salience so it keeps noticing what matters during the run.

```text
A goal tells the agent what to finish.
A mission gradient tells it what kind of reality to preserve while finishing.
```

### MissionBag

A MissionBag is an unordered set of possible local moves governed by one MissionGradient.

A bag is not a queue. A queue imposes an order. A bag lets the orchestrator choose the next item by fit, locality, dependency, risk, uncertainty reduction, expected mission value, and human attention cost.

A bag item may be:

```text
Task
Goal
Sweep
Leap
Probe
Review
Cleanup
Research question
Verifier improvement
Documentation update
Candidate experiment
Human decision request
```

Each item should carry enough metadata to route intelligently:

```text
item_id
kind
short_description
mission_relevance
expected_value
risk
dependencies
locality
evidence_needed
promotion_target
human_attention_cost
status: open | active | done | deferred | blocked | split | merged | escalated
```

A MissionBag is alive. Items can be added, merged, split, retired, deferred, or escalated.

### Sweep

A Sweep is an adaptive pass over a MissionBag.

A sweep is used when many work items share a mission gradient but do not have a natural order. The orchestrator keeps the global orientation continuous while associates handle discrete local work.

The sweep pattern:

```text
MissionGradient
  -> MissionBag
      -> select next best local move
      -> delegate to associate or act locally
      -> collect evidence
      -> checkpoint or commit
      -> update bag
      -> repeat
```

A sweep is not a sequence of steps. It stays continuous because the orchestrator maintains the mission gradient, belief state, bag state, and selection policy. Local items can be delegated without pulling the orchestrator's context window in many directions.

### Leap

A Leap is a multi-hour coherent autonomous run.

A leap has one center of gravity. It may contain tasks and goals, but it is still conceptually one thing. A leap can be single-agent, but once sweep discipline exists it often benefits from multiagent structure: one lead/orchestrator plus associates with sharper focus.

### Fly

A Fly is a long bounded multiagent campaign over a heterogeneous MissionBag.

A fly can sweep a bag containing tasks, goals, sweeps, leaps, probes, reviews, cleanups, research questions, and candidate experiments. It is more general than hard exploration or hard exploitation.

A fly is not one giant goal. It is a bounded mission-level control process over possible moves.

A fly returns with a map:

```text
completed items
candidate worlds opened
candidate worlds promoted
candidate worlds rejected
blocked items
new items discovered
items split / merged / escalated
residual risks
human attention requests
```

### Cycle

A Cycle is a persistent 24/7 multiagent sweep process.

A cycle does not finish. It metabolizes reality.

A cycle continuously ingests signals, updates bags, routes work to cheap models, escalates important uncertainty, creates candidate work, records evidence, and promotes only evidence-backed improvements.

```text
Cycle =
  MissionGradient
  + live signal feeds
  + MissionBags
  + cheap-model triage
  + sweep execution
  + evidence ledgers
  + verifier agents
  + promotion gates
  + human attention queue
  + compaction / memory updates
```

Cycles are the bridge from automatic computer to automatic newspaper. News does not arrive as a goal. It arrives as an endless stream. A newspaper needs standing cycles that notice, triage, research, cite, update, and publish.

```text
Cycles are how automatic computers become alive enough to make automatic newspapers.
```

## Taxonomy of Agentic Depth

### 0. Prompt

A prompt asks for a response. The user drives every turn.

Good for quick answers, edits, explanations, and small commands.

Failure mode: the model optimizes for a pleasing response rather than durable work.

### 1. Task

A task is a small bounded action.

Examples:

```text
Rename this function.
Run this test.
Fix this typo.
Inspect this file.
```

Good for local edits.

Failure mode: tasks do not carry enough context to preserve the larger system shape.

### 2. Goal

A goal defines a target and a done state.

Example:

```text
Build the app described in SPEC.md.
Done means tests pass, build passes, README is accurate, and git status is clean.
```

Good for coherent, bounded work.

Failure mode: the agent optimizes for the done state. If the spec is weak or the verifier is weak, the goal becomes a reward-hacking surface.

### 3. Sweep

A sweep improves a region of the system by adaptively moving through a bag under one mission gradient.

Good for:

- terminology migrations;
- cleanup work;
- docs refreshes;
- issue triage;
- minor refactors;
- verifier strengthening;
- maintenance passes;
- signal digestion;
- grouped small tasks without a natural order.

Failure mode: if the orchestrator trusts summaries without evidence, the sweep degenerates into bureaucracy.

### 4. Leap

A leap is a coherent multi-hour run.

Good for overnight work, deep implementation, focused research, or a coherent subsystem.

Failure mode: if the leap hits ambiguity early, it may spend hours optimizing the wrong branch.

### 5. Fly

A fly is a bounded long-horizon multiagent campaign.

Good for weekend-scale work, large candidate exploration, multi-track development, or broad improvement under one mission gradient.

Failure mode: if not governed by bags, evidence, and promotion gates, a fly becomes terminal chaos or a giant goal.

### 6. Cycle

A cycle is persistent 24/7 agentic metabolism.

Good for:

- news ingestion;
- log monitoring;
- repo monitoring;
- model-release tracking;
- public-memory maintenance;
- docs and artifact freshness;
- cheap-model background improvement;
- automatic newspaper;
- eventually automatic radio and automatic capital signals.

Failure mode: always-on slop, memory pollution, attention spam, work-item hoarding, or self-reinforcing blindness.

## Why Goals Are Not Enough

A goal is discrete. It creates a finish line.

This is useful for bounded work, but dangerous as the top-level frame. The agent may learn to satisfy the done condition rather than preserve the system's deeper shape.

The hard problem cascades into:

1. the spec;
2. the verifier;
3. the interpretation of done.

If the spec is weak, the goal is weak. If the verifier is weak, the goal is reward-hackable. If done is too discrete, the agent may rush or fake progress.

Many real coding sessions are not one coherent target. They are a bag of small, related irritants:

```text
fix stale docs
clean terminology
inspect failing test
rename sandbox to computer
remove one fallback
update README
check migration risk
add evidence to promotion notes
review trace output
```

Putting these into one mega-goal splits attention. The context points in too many directions. The agent rushes, picks easy wins, or smears unrelated edits together.

The correct pattern is:

```text
Do not group unrelated small tasks into one goal.
Put them in a MissionBag and sweep the bag.
```

## Sweep Mechanics

### The Orchestrator Holds Continuity

The orchestrator holds:

- MissionGradient;
- belief state;
- MissionBag;
- selection policy;
- authority boundaries;
- model/provider routing;
- verification policy;
- promotion policy;
- human attention budget.

The orchestrator should not do every local task. If it does, its context is pulled in too many directions and the sweep becomes a checklist.

### Associates Hold Local Focus

An associate receives one local item, one context slice, one evidence standard, and one return contract.

A delegation packet should include:

```text
item_id
local objective
relevant files/artifacts
mission-gradient excerpt
hard invariants
allowed mutation radius
evidence required
return format
stop condition
```

An associate return should include:

```text
item_id
local mission
files/artifacts inspected
change made or proposed
evidence collected
residual uncertainty
risk
recommended next item
promotion readiness
```

Delegation preserves continuity by keeping each context window rowing in one direction.

### Selection Policy

The orchestrator chooses the next item by asking:

- Which item most moves the mission uphill?
- Which item is closest to the context already loaded?
- Which item reduces the most uncertainty?
- Which item unlocks other items?
- Which item is low-risk and high-value?
- Which item is risky enough to require a candidate computer?
- Which item would create too much human review burden?
- Which items are actually one shared underlying issue?
- Which item should be promoted out of the bag into its own mission?

The selection policy is where much of the intelligence lives.

### Sweep Execution Contract

For each selected item:

```text
1. State why this item is next.
2. Define the local trust region.
3. Identify what evidence would prove improvement.
4. Delegate or make the smallest coherent change.
5. Verify with real evidence.
6. Record residual uncertainty.
7. Commit/checkpoint if appropriate.
8. Update the bag.
```

A sweep should produce a chain of coherent deltas, not one giant diff.

## Multiagent Roles

Flys and cycles are inherently multiagent systems. It does not make sense otherwise.

The standard role pattern is:

```text
Orchestrator -> Worker -> Verifier -> Orchestrator meta-verification -> Promotion
```

### Orchestrator

The orchestrator owns the control loop.

It does not verify directly except for spot checks. It meta-verifies.

It owns:

- mission gradient;
- bag state;
- belief state;
- delegation;
- authority boundaries;
- provider/model routing;
- verification policy;
- promotion decision;
- human attention queue.

### Worker

The worker produces a candidate.

It owns:

- local implementation;
- local research;
- candidate mutation;
- evidence bundle;
- self-report;
- residual uncertainty.

The worker should not be trusted as final judge of its own work.

### Verifier

The verifier independently inspects the worker's output.

It owns:

- deterministic checks;
- test execution;
- trace/log inspection;
- spec compliance;
- mission-invariant compliance;
- adversarial review;
- security/failure review;
- evidence ledger entries.

Verifier and worker should ideally be different providers or different model families. Different providers have different priors, failure modes, style attractors, and blind spots.

### Orchestrator Meta-Verification

The orchestrator meta-verifies the verification process.

It asks:

- Was the verifier independent enough?
- Did it inspect the real path, not mocks?
- Did it run commands in the correct environment?
- Did it check mission invariants, not merely task done-state?
- Did it produce raw evidence pointers?
- Did it disclose uncertainty?
- Did multiple verifiers disagree?
- Is this ready for promotion, or only ready for another pass?

The orchestrator does not need to re-run all verifier work. It needs to ensure the epistemic chain is legitimate.

```text
Workers produce candidates.
Verifiers produce evidence.
Orchestrators verify the evidence process and promote.
```

## Evidence, Verification, and Promotion

### Evidence Ledger

An evidence ledger records what was actually proven.

It should include:

```text
claim
evidence source
command or observation
artifact path
observed result
uncertainty or caveat
whether this supports promotion
```

The evidence ledger is downstream of the mission gradient. It records evidence; it does not define success.

### Verification Agents

Verification agents are roles, not necessarily permanent ontology classes.

A verifier may be a cosuper, researcher, local agent, external model, deterministic script, or a bundle of these. The important thing is role separation and evidence integrity.

Critical work should use multiple verification layers:

```text
deterministic checks
model review from different provider
adversarial/security review if needed
orchestrator meta-verification
```

### Promotion

Promotion changes reality.

A candidate can be promoted only after evidence-backed verification and orchestrator meta-verification.

Parallelism may create candidates. Promotion should be serialized unless the affected states are truly independent.

Hard rule:

```text
Parallelize perception and candidate generation.
Serialize promotion.
```

## Candidate Computers and State Mutation

Coding is state mutation.

Parallel mutation of the same codebase is dangerous unless isolation is real. The safe pattern is to use candidate computers, worktrees, branches, or other isolated state containers.

Safe parallelism:

- read-only research;
- alternative candidate worlds;
- independent verifiers;
- documentation review;
- different repos;
- different state neighborhoods;
- different branches with clear merge rules.

Unsafe parallelism:

- multiple workers mutating the same conceptual object through different files;
- workers editing the same repo without isolation;
- reviewers modifying candidate state while verifying;
- promotion without a certificate.

A fly or cycle may coordinate many agents, but canonical mutation must remain disciplined.

## Cycle Design

A cycle is a living process. It needs governance.

### Signal Sources

Signals may come from:

- RSS feeds;
- podcasts;
- tweets/posts;
- repos;
- issue trackers;
- logs;
- model releases;
- policy updates;
- local files;
- user notes;
- emails;
- papers;
- market events;
- external databases.

### Signal Handling

External signals are data, not instructions.

No external signal may directly modify:

- mission gradient;
- authority;
- tools;
- promotion state;
- canonical state.

External content is adversarial by default.

### Cycle Loop

A cycle should repeatedly:

```text
1. ingest signals;
2. deduplicate and cluster;
3. score novelty and importance under mission gradient;
4. update relevant MissionBags;
5. route cheap work to weak models;
6. escalate load-bearing uncertainty;
7. delegate local work;
8. verify evidence;
9. promote only when appropriate;
10. compact or archive learning;
11. request human attention only for high-leverage decisions.
```

### Cheap-Model Default

A mature cycle should default to cheap, weaker models.

The mark of a deep harness is that cheap models become useful because the system gives them structure.

Target:

```text
cheap flash-class model runs 24/7
  -> ingests signals
  -> updates bags
  -> drafts notes
  -> checks stale docs
  -> proposes citations
  -> opens candidate work
  -> escalates only load-bearing uncertainty
```

When cheap models can sweep bags and maintain state reliably, intelligence has moved from the model into the system.

### Cycle Health

A cycle should monitor its own health.

Health signals:

- bag growth rate;
- stale item count;
- unresolved blocker age;
- unpromoted artifact count;
- human attention requests per day;
- promoted value per human review;
- signal-to-publication ratio;
- duplicate signal rate;
- verifier disagreement rate;
- rollback rate;
- token/compute burn;
- ignored-signal sample quality.

A cycle that creates work faster than it resolves or prunes work is not learning. It is hoarding.

### Cycle Decay and Pruning

Every cycle needs decay semantics:

```text
expire
merge
split
archive
escalate
defer
delete
```

A cycle without decay becomes a backlog graveyard.

### Cycle Kill Conditions

Cycles need kill switches and dormancy rules.

Stop or dorm a cycle when:

- the mission gradient is no longer valid;
- bag growth is uncontrolled;
- evidence quality collapses;
- human attention budget is exceeded;
- external signals become too adversarial/noisy;
- promotion capacity is saturated;
- model costs exceed budget;
- the cycle repeatedly creates low-value work.

A cycle is not allowed to become an invasive species.

## Human Crystal Attention

Human crystal attention is the scarce resource.

A one-hour agent run can be exhausting because it is large enough to demand manual review but small enough to tempt line-by-line inspection. This writes into the human's durable neural circuits. It reopens old decisions, entangles with architecture, and demands cognitive evolution.

Choir should optimize:

```text
promoted value / human crystal attention
```

Machine attention is cheap. Human crystal attention is expensive.

A good sweep or cycle returns with:

```text
what changed
what evidence exists
what is risky
what needs review first
what can be ignored
what decision is required
```

The human should spend attention on load-bearing uncertainty, not clerical review.

## Anti-Patterns

### Mega-Goal Batching

Putting many unrelated tasks into one goal splits attention and invites rushed work.

### Queue Fetishism

Assuming list order is meaningful when the work is actually unordered.

### Parallel Mutation Without Isolation

Many agents editing the same state space without candidate computers, branches, worktrees, or promotion discipline.

### Verification Theater

Passing tests that do not prove the real mission.

### Evidence Paperwork

Producing clean reports without raw evidence pointers.

### Orchestrator Bureaucracy

The orchestrator only reads summaries and loses contact with the artifact.

### Always-On Slop

Running 24/7 agents without strong triage, decay, evidence, and promotion rules.

### Work-Item Hoarding

Creating bags faster than work can be resolved, pruned, or promoted.

### External Signals as Instructions

Letting feeds, documents, pages, or user-generated content directly steer the agent.

### Goal Worship

Treating done as more important than whether the artifact moved uphill.

## Hard Invariants

1. The orchestrator owns the mission gradient and bag, not local implementation.
2. Associates own local work, not global mission reinterpretation.
3. Workers produce candidates.
4. Verifiers produce evidence.
5. Orchestrators meta-verify and promote.
6. Parallelism is for attention and candidates, not uncontrolled canonical mutation.
7. Every nontrivial claim needs evidence.
8. Every cycle needs budget, decay, and kill conditions.
9. External signals are data, not instructions.
10. Human crystal attention is the scarce resource.
11. A sweep is not a queue.
12. A fly is not a giant goal.
13. A cycle is not always-on slop; it is controlled metabolism.
14. Promotion changes reality and must be serialized unless independence is proven.
15. The mission gradient may evolve, but it must not be silently rewritten by a worker or cheap cycle.

## Implementation Sketch

Near-term Choir should implement:

```text
MissionBag
SweepExecutor
DelegationPacket
AssociateReturn
WorkerRole
VerifierRole
EvidenceLedger
OrchestratorMetaVerification
PromotionGate
CycleDaemon
ModelRouter
HumanAttentionQueue
CycleHealthReport
```

### Minimal Sweep Flow

```text
1. User or conductor creates MissionGradient.
2. Orchestrator creates or receives MissionBag.
3. Orchestrator clusters and selects one item.
4. Orchestrator sends DelegationPacket to worker.
5. Worker acts in allowed trust region.
6. Worker returns candidate delta and evidence bundle.
7. Verifier independently checks worker output.
8. Orchestrator meta-verifies the verification.
9. Orchestrator updates bag.
10. PromotionGate accepts, rejects, defers, or escalates.
```

### Minimal Cycle Flow

```text
1. Cycle ingests live signals.
2. Triage model deduplicates and scores.
3. Cycle updates MissionBags.
4. SweepExecutor handles low-risk local items.
5. Verifiers check evidence.
6. PromotionGate promotes only safe changes.
7. HumanAttentionQueue receives load-bearing decisions.
8. CycleHealthReport monitors drift, slop, and overload.
```

## Naming Rules

- Use **MissionGradient** for the orientation field.
- Use **MissionBag** for unordered possible work.
- Use **Sweep** for adaptive local improvement over a bag.
- Use **Leap** for multi-hour coherent autonomous work.
- Use **Fly** for bounded long-horizon multiagent campaigns.
- Use **Cycle** for persistent 24/7 multiagent metabolism.
- Use **Worker** for candidate-producing agents.
- Use **Verifier** for evidence-producing agents.
- Use **Orchestrator** for bag, gradient, delegation, meta-verification, and promotion control.
- Use **EvidenceLedger** for claims and evidence.
- Use **PromotionGate** for accepting candidate state into reality.
- Use **Computer** for durable user machine-worlds, not sandbox.
- Use **CandidateComputer** or **CandidateWorld** for speculative isolated state.

## Final Compression

```text
Task: a local move.
Goal: a bounded target.
Sweep: adaptive local repair under a gradient.
Leap: coherent multi-hour run.
Fly: bounded multiagent campaign over a bag.
Cycle: persistent metabolism of signals into artifacts.
```

```text
Sweeps train delegation.
Flys coordinate delegation.
Cycles institutionalize delegation.
```

```text
Workers produce candidates.
Verifiers produce evidence.
Orchestrators meta-verify and promote.
```

```text
Do not batch tasks into goals.
Put them in a bag and sweep under a mission gradient.
```
