---
name: definition
description: Use when work needs executable semantic authority: defining a mission, object, invariant set, authority boundary, evidence class, completion semantics, rollback policy, or forbidden collapse before or during agentic execution. Produces or updates a mission-definition document that is directly executable with `/goal <document>.md`. This skill replaces legacy mission-gradient/parallax-style control: it defines the mission graph, then the harness executes that graph instead of treating it as a plan or summary.
version: 1.0.0
author: Hermes Agent
license: MIT
metadata:
  hermes:
    tags: [definition, mission-definition, semantic-authority, goal-execution, long-running-agents, verification, orchestration]
    related_skills: [cognitive-transform-portfolio]
---

# Definition

Definition is the high-level semantic authority layer for agentic work.

It exists because long-running agents fail when the governing words are weaker
than the execution pressure. They turn checkpoints into completion, artifacts
into proof, plans into authority, tests into universal truth, toy evidence into
program validation, and motion into progress.

Definition makes the mission executable by defining what the mission's words are
allowed to mean, how those meanings are observed, who has authority, which
claims evidence can support, what must happen next, and when execution must stop.

The output is a **mission-definition document**. When a compatible harness is
called with:

```text
/goal <document>.md
```

it must execute the mission defined in that document. It must not summarize the
document, admire it, checkpoint early, or create a separate control language.

## Core Thesis

A definition is not a gloss. A definition is operational authority.

- A plan lists intended actions.
- A conjecture names a claim to test.
- A gradient names an optimization landscape.
- A definition says what the mission, objects, claims, evidence, authority,
  states, and completion conditions are allowed to mean.

The mission-definition document can contain plans, conjectures, gradients,
variants, ledgers, and reports, but those are subordinate projections. The
controlling object is the **definition graph plus execution state**.

## Replaces Legacy Mission Control Skills

Definition supersedes older mission-control formats by lifting their useful
semantics into one document model:

- topology-preserving mission geometry;
- homotopy from small-real to high-realism domains;
- conjecture-led belief state;
- observer shifts and hyperthesis edges;
- receding-horizon execution;
- evidence-scoped claims;
- variant/descent pressure against motion theater;
- dense feedback and root-cause investigation;
- rollback, promotion, and resumption policy;
- final owner-readable report when required.

Do not invoke or preserve separate legacy mission-control skills when Definition
is the selected authority. Express their useful ideas as definition nodes,
mission execution state, evidence ledgers, and completion semantics inside the
mission-definition document.

## When To Use

Use Definition when:

- the user wants a mission executed through `/goal <document>.md`;
- a long-running or multi-agent run needs stable semantic authority;
- terms like `done`, `blocked`, `verified`, `safe`, `artifact`, `authority`,
  `proof`, `winner`, `checkpoint`, `smoke`, `promotion`, or `mission` are loaded;
- agents are likely to optimize a proxy or fake island;
- evidence may mislead unless scoped;
- the route is uncertain and needs observer shifts, root-cause probes, or
  reparameterization;
- a document must be both the mission contract and the resumable execution state;
- human input should govern group-level intent, not every leaf decision.

Do not use Definition for ordinary coding tasks where the target vocabulary,
authority boundary, and completion semantics are already stable.

## Non-Definitions

Definition is not:

- a static document generator;
- a decorative ontology;
- a normal implementation plan;
- a standalone conjecture ledger;
- a vote among models;
- a report that replaces execution;
- a license to continue after an invariant breaks;
- an excuse to delay implementation when the next executable probe is in bounds.

A definition node is useful only if it changes an action, verifier, route,
scope, authority boundary, claim, rollback policy, or stopping condition.

## Mission-Definition Document

A mission-definition document is the source program for a `/goal` run.

If the user supplies an existing document, compile it in place. Preserve the
author's source text where possible, but add or update the Definition sections
needed to make execution unambiguous. Do not create a parallel control document
unless the source document explicitly requires a split.

A mission-definition document should contain these sections when relevant:

```text
# <Mission Name>

## Harness Invocation Semantics
## Source Authority Order
## Real Artifact / Object Of Work
## Mission Purpose And Non-Purpose
## Definition Graph
## Determined State Snapshot
## Invariants
## Authority Boundaries
## Value Criterion
## Homotopy / Realism Parameters
## Conjecture And Belief State
## Variant / Progress Measure
## Execution Operators
## Receding-Horizon Control Loop
## Dense Feedback Channels
## Evidence Ledger
## Completion Semantics
## Escalation Rules
## Forbidden Collapses
## Rollback And Resumption Policy
## Mission Report Policy
## Run Checkpoint & Resumption State
## Suggested Goal String
```

Use only the sections needed for the mission. Missing load-bearing sections are
opened as definition nodes, not silently inferred past their evidence.

## `/goal <document>.md` Semantics

When a compatible harness receives:

```text
/goal <document>.md
```

it must interpret this as:

```text
Read the mission-definition document as semantic authority. Execute it
autonomously until its completion semantics are satisfied with named evidence,
or until a sharply evidenced escalation/blocker/supersession condition is met.
```

The harness must:

1. read the document and its declared authority sources;
2. reconcile current artifact state with the document's determined state;
3. open definition nodes for missing or contested load-bearing meanings;
4. resolve leaf definitions through the critical process;
5. choose the next executable probe or construct inside the authority boundary;
6. state the active definition/conjecture being tested before mutation;
7. execute, verify, and scope the resulting claim to its evidence class;
8. update the document's definition graph, determined state, evidence ledger,
   and checkpoint/resumption state;
9. continue until completion, blocked escalation, or supersession.

The harness must not stop because a phase boundary, checkpoint, review packet,
passing focused test, or worker claim exists. Those are evidence candidates, not
completion.

## Definition Graph

A definition graph contains typed nodes and explicit execution effects.

Common node kinds:

```text
term
object
mission
boundary
invariant
observable
status
operator
evidence_class
authority_rule
forbidden_collapse
completion_semantics
escalation_rule
formalization_seam
rollback_rule
conjecture
variant
homotopy_parameter
```

Common node statuses:

```text
unresolved
proposed
contested
under_deliberation
testing
settled
promoted
weakened
falsified
invalidated
superseded
requires_human_authority
```

A minimal node:

```yaml
id: <stable-id>
kind: <node-kind>
status: <node-status>
source: user-stated | observed | inferred | reviewer | formal-check | worker-report
term: <name>
definition: <what it means>
non_definition:
  - <what does not count>
examples:
  - <positive case>
counterexamples:
  - <case that breaks or abuses the definition>
observables:
  - <how an agent can inspect it>
execution_effect:
  - <what downstream agents may or may not do if this definition is settled>
forbidden_collapses:
  - <common fake equivalence>
formalization:
  status: not-applicable | candidate | required | done | blocked
  note: <spec, contract, property test, model check, proof obligation, assertion, or executable checker>
settlement:
  rule: <what is enough to settle this node>
  settled_by: orchestrator | human | formal-check | reviewer | evidence
  invalidation_triggers:
    - <what reopens it>
```

## Determined State

Determined state is the current semantic authority snapshot.

A claim belongs to determined state only if it is:

1. **user-stated authority**;
2. **observed fact** from tools, files, commands, traces, artifacts, or systems;
3. **settled definition** with no live contradiction;
4. **operational preference** explicitly stated by the owner.

A claim does not belong if it is merely plausible, stylish, repeated, or asserted
by a model.

Use this shape:

```yaml
determined_state:
  settled:
    - claim: <authoritative statement>
      source: user-stated | observed | settled-definition | operational-preference
      execution_effect: <what this changes>
  contested:
    - node: <definition id>
      issue: <why not settled>
      next_resolution_step: <critical process step>
  open:
    - node: <definition id>
      missing: <what must be defined>
```

## Critical Process

Definition nodes are resolved by a critical process:

```text
OPEN
  Detect an ambiguous, missing, overloaded, contested, or drift-causing meaning.

DIFFERENTIATE
  Split meanings. Name objects, boundaries, authority, and non-definitions.

CRITICIZE
  Generate counterexamples, forbidden collapses, reward hacks, and downstream
  failure modes.

TRANSFORM
  If stuck, shallow, frame-locked, or over-literal, use cognitive transforms.
  Keep only transforms that change the next probe, verifier, route, scope,
  evidence plan, or stopping condition.

OPERATIONALIZE
  Attach observables, execution effects, conformance checks, settlement rules,
  and invalidation triggers.

FORMALIZE
  If the node governs state, concurrency, lifecycle, authority, safety,
  irreversible mutation, or promotion, consider a formalization seam.

PROBE / CONSTRUCT
  Execute the smallest or largest-batched safe action that can settle the node,
  depending on expected information gain and mutation radius.

SETTLE
  Promote, weaken, falsify, invalidate, supersede, or escalate.

MONITOR
  Watch downstream execution for drift and reopen nodes when invalidated.
```

## Mission Execution Loop

Definition uses receding-horizon execution, but the loop operates over the
definition graph rather than over a separate mission format.

Each control interval:

1. **Select** the live node or conjecture whose settlement most reduces mission
   uncertainty or unlocks execution.
2. **State** what the current observer can and cannot see; name any blind spot.
3. **Choose** one move:
   - `define`: make a missing meaning executable;
   - `probe`: test a claim under current observer;
   - `shift`: change observer, vocabulary, domain, instrument, or prover;
   - `construct`: mutate the artifact under invariants;
   - `verify`: check an artifact or claim;
   - `settle`: promote/weaken/falsify/supersede/escalate.
4. **Bound** the mutation radius and rollback surface.
5. **Execute** the move.
6. **Update** node status, determined state, evidence ledger, and checkpoint.
7. **Continue** unless completion, supersession, or hard escalation is reached.

If the route is clear and low-risk, batch foreseeable constructs in one interval.
The tripwire is surprise: any unexpected evidence returns execution to a full
select/state/choose/bound loop.

## Conjectures As Definition Nodes

A conjecture is a definition node for a claim whose truth affects execution.
Use this shape when needed:

```yaml
id: <conjecture-id>
kind: conjecture
status: proposed | testing | settled | weakened | falsified | superseded
claim: <what might be true>
test: <how the current observer would know>
edge:
  blind_spot: <what this observer cannot see>
  class: independence | resource | missing_oracle | frame_lock
observer_upgrade: <smallest shift that shrinks the edge>
scope_if_supported: <domain over which the claim may be asserted>
falsifier: <fastest observation that would kill the claim>
execution_effect: <what changes if supported/falsified>
```

A conjecture ledger is not separate authority. It is a typed slice of the
definition graph.

## Variant / Progress Measure

For long missions, define a variant: a concrete measure of unresolved mission
state that productive execution should reduce.

Good variants count decided definition nodes, unresolved blockers, failing
contract classes, missing observables, open conjectures, or unverified artifact
interfaces. Bad variants count effort, elapsed time, number of files touched, or
vague percentage completion.

A pass that changes no node status, buys no new observer evidence, and improves
no artifact verifier is motion theater. The next move must shift observer,
vocabulary, domain, instrument, or prover.

## Homotopy / Realism Parameters

When simplifying a mission, preserve topology.

A low-resolution mission domain is valid only if it continuously embeds in the
full domain: same object family, state semantics, authority boundaries, event
causality, verifier meaning, rollback surface, and evidence class.

Fake islands are forbidden. Examples:

- mock APIs that bypass the production path;
- test-only persistence;
- manually seeded success artifacts;
- local proof when the claim requires deployment;
- toy results cited as full program validation;
- permissive assertions that erase causal structure.

## Evidence Classes And Claim Scope

Every nontrivial claim must state its evidence class and scope.

Evidence classes include:

```text
observed file/process/tool result
unit/example test
property test
contract test
model check / formal spec
code-level proof
integration/e2e trace
deployed production/staging proof
human review
external second opinion
```

Claims must not outrun their evidence class.

- Passing focused tests proves only those executions/predicates.
- A model check proves the model, not the implementation, unless conformance is
  established.
- A review packet is reviewer attention, not automatic truth.
- An artifact is evidence only after its schema, provenance, and meaning are
  checked.

## Evidence Ledger

For each promoted or settled claim, record:

```yaml
claim: <scoped claim>
definition_node: <node id>
evidence_class: <class>
source: <file/tool/command/trace/reviewer>
command_or_observation: <exact command or observation>
artifact_path: <path or URI>
result: <observed result>
uncertainty: <remaining edge or caveat>
promotion_relevance: <what this authorizes, if anything>
```

The ledger is not a success substitute. It records proof reach.

## Authority And Human Escalation

Escalate to the human only for group-level decisions:

- purpose or identity changes;
- authority-boundary changes;
- unsafe/destructive or high-blast-radius mutations;
- paid/long-running compute beyond already granted policy;
- conflicting values or taste calls;
- irreversible actions without accepted rollback;
- definitions whose settlement would authorize risky mutation.

Do not escalate every leaf definition. Resolve leaf definitions through the
critical process when they stay inside established authority.

Escalations must name:

```yaml
human_escalation:
  node: <definition id>
  issue: <why orchestration cannot settle it>
  options:
    - choice: <option>
      execution_consequence: <what happens if chosen>
  recommendation: <recommended choice>
```

## Formalization Seam

When a node governs stateful or safety-sensitive behavior, ask:

- Can this definition be projected into a formal spec?
- Can code enforce it with types, assertions, contracts, properties, or model
  checks?
- What impossible state should be unreachable?
- What counterexample would invalidate the definition?
- What is the refinement surface from spec/model to code/tests/traces?

Do not force formal verification everywhere. Make the absence of formalization
visible when a high-risk definition depends on prose alone.

## Second Opinions

Use second opinions only if they can change the graph:

- chosen definition;
- split/merge/narrow/widen decision;
- execution effect;
- verifier or evidence class;
- formalization requirement;
- escalation boundary;
- stopping condition;
- downstream route.

Second opinions are not votes. The orchestrator adjudicates and updates the
graph.

Before requesting an external second opinion, record:

```yaml
second_opinion_request:
  node: <definition id>
  unresolved_question: <specific question>
  expected_decision_impact: <what could change>
  why_internal_deliberation_is_insufficient: <reason>
  chosen_tool: <tool>
  compute_tier: internal | normal_external | premium
  max_output_shape: <verdict/counterexample/execution effect/etc.>
```

## Mission Report Policy

Broad mission-definition runs should maintain an owner-readable report when the
run changes durable system state, doctrine, deployed behavior, or long-running
training/execution state.

The report should explain:

```text
mission goal and artifact
invariants preserved or violated
major decisions and route changes
what shipped
verification evidence
what was proven vs merely attempted
residual risks
rollback refs
next mission or next executable probe
```

Do not dump logs. Link evidence artifacts.

PDF export is optional unless the mission document or owner requests it.

## Checkpoint And Resumption State

Mission-definition documents must carry resumable state:

```yaml
run_checkpoint_and_resumption_state:
  status: working | complete | checkpoint_incomplete | blocked_incomplete | superseded
  last_checkpoint: <commit/artifact/state>
  current_artifact_state: <what exists now>
  what_shipped: []
  what_was_proven: []
  unproven_or_partial_claims: []
  belief_state_changes: []
  remaining_error_field: []
  highest_impact_remaining_uncertainty: <node or claim>
  next_executable_probe: <next safe in-bound action>
  suggested_goal_string: "/goal <document>.md"
  evidence_artifact_refs: []
  rollback_refs: []
```

A checkpoint is not completion. If a safe executable probe remains inside the
mission authority boundary, execute it instead of presenting the checkpoint as
success.

## Completion Semantics

Completion means the document's own completion semantics are satisfied with
named observables and evidence.

Use statuses:

```text
working
complete
checkpoint_incomplete
blocked_incomplete
superseded
```

- `complete`: stopping condition satisfied with scoped evidence.
- `checkpoint_incomplete`: useful progress landed, but stopping condition is
  not satisfied. This is not success.
- `blocked_incomplete`: progress is blocked after root-cause probes and
  cognitive transforms, with exact blocker and required authority/prerequisite.
- `superseded`: learning changed the mission identity enough that continuing
  would optimize the wrong object.

Before any non-complete exit, the agent must verify that no safe executable
probe remains inside the authority boundary.

## Forbidden Collapses

Do not collapse:

- artifact exists -> artifact is valid;
- definition document exists -> definition graph is settled;
- plan exists -> mission is executing;
- review packet exists -> review passed;
- tests passed -> behavior is universally proven;
- checkpoint landed -> mission complete;
- model agreement -> definition settled;
- formal spec exists -> implementation conforms;
- implementation exists -> definition was followed;
- local smoke passed -> production claim proven;
- toy result green -> program validated;
- second opinion -> authority;
- route is familiar -> route is correct;
- worker says done -> done.

## Definition Operators

The orchestration agent may apply:

```text
define(node)
split(node)
merge(nodes)
narrow(node)
widen(node)
counterexample(node)
operationalize(node)
formalize(node)
probe(node)
shift(node)
construct(node)
verify(node)
request_second_opinion(node, tier)
settle(node)
weaken(node)
falsify(node)
invalidate(node)
supersede(node)
promote(node)
escalate(node)
monitor(node)
```

Each operator must leave a graph or determined-state update. No silent semantic
changes.

## Conformance Checklist

A run conforms to Definition if:

- [ ] It names the active mission-definition document.
- [ ] It treats `/goal <document>.md` as executable authority, not passive context.
- [ ] It identifies the real artifact/object of work.
- [ ] It separates purpose from non-purpose.
- [ ] It names authority sources and boundaries.
- [ ] It maintains a definition graph and determined-state snapshot.
- [ ] It attaches observables and execution effects to settled nodes.
- [ ] It scopes claims to evidence classes.
- [ ] It preserves topology when simplifying.
- [ ] It uses conjectures as definition nodes when truth affects execution.
- [ ] It uses variants/progress measures for long runs.
- [ ] It executes safe in-bound probes instead of stopping at checkpoints.
- [ ] It records evidence and rollback/resumption state.
- [ ] It escalates only group-level or sharply evidenced hard blockers.
- [ ] It does not claim completion until the document's completion semantics are satisfied.

## Suggested Invocation

```text
Use Definition. Treat <document>.md as an executable mission-definition document, not a plan or summary. Read its authority sources, reconcile determined state, define missing terms/objects/boundaries/invariants/observables/evidence classes/completion semantics, then execute the mission through the definition graph. Maintain evidence, rollback, and resumption state. Use conjectures, variants, observer shifts, homotopy parameters, and dense feedback only as subordinate definition nodes when they change execution. Continue safe in-bound probes instead of stopping at checkpoints. Escalate only on group-level authority changes, unsafe/high-blast-radius actions, external boundaries, or hard blockers after root-cause probes and cognitive reframing. Stop only when the document's completion semantics are satisfied with named evidence, or when it is honestly blocked/superseded.
```
