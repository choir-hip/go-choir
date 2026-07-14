# Definition Semantic Methods

Read this reference when the route is unclear, a proxy/fake-island risk exists,
claims need evidence scoping, or a safety-sensitive node needs formalization.
Routine execution on a settled route need not load it.

## Contents

- Critical process
- Conjectures and observer upgrades
- Variant and progress
- Homotopy and realism
- Evidence classes and claim scope
- Formalization seam
- Forbidden collapses

## Critical Process

Resolve definition nodes through:

```text
OPEN
  Detect ambiguous, missing, overloaded, contested, or drift-causing meaning.

DIFFERENTIATE
  Split meanings. Name objects, boundaries, authority, and non-definitions.

CRITICIZE
  Generate counterexamples, forbidden collapses, reward hacks, and downstream
  failure modes.

TRANSFORM
  If stuck or frame-locked, use cognitive transforms. Keep only transforms that
  change the next probe, verifier, route, scope, evidence plan, or stop rule.

OPERATIONALIZE
  Attach observables, execution effects, conformance checks, settlement rules,
  and invalidation triggers.

FORMALIZE
  For state, concurrency, lifecycle, authority, safety, irreversible mutation,
  or promotion, consider a formalization seam.

PROBE / CONSTRUCT
  Execute the smallest or largest-batched safe action that can settle the node,
  according to information gain and mutation radius.

SETTLE
  Promote, weaken, falsify, invalidate, supersede, or escalate.

MONITOR
  Watch downstream execution for drift and reopen invalidated nodes.
```

## Conjectures And Observer Upgrades

A conjecture is a definition node whose truth changes execution. Record its
claim, test, observer blind spot, observer upgrade, supported scope, fastest
falsifier, and execution effect. A conjecture ledger is a typed graph view, not
separate authority.

## Weak Measures, Variants, And Progress

For a long mission, use a small variant or weak measure to decide what to
inspect next: unresolved decision-changing questions, blockers, failing
contract classes, missing observables, open conjectures, or unverified
interfaces. State its baseline, decision use, and what it cannot prove in the
goal file. Do not use effort, elapsed time, files touched, vague percentage
completion, or a documentation-sensitive count as a completion proxy.

A pass that changes no decision, buys no observer evidence, and improves no
artifact verifier is motion theater. Shift observer, vocabulary, domain,
instrument, or prover. A favorable measure may justify that shift; it never
settles the goal or substitutes for its stated acceptance artifact.

## Homotopy And Realism

Preserve topology when simplifying. A low-resolution domain is valid only when
it embeds in the full domain with the same object family, state semantics,
authority boundaries, event causality, verifier meaning, rollback surface, and
evidence class.

Forbid fake islands such as mock APIs bypassing production, test-only
persistence, manually seeded success, local proof cited for deployment, toy
results cited as program validation, or permissive assertions erasing causality.

## Evidence Classes And Claim Scope

Use evidence classes such as observed tool result, unit/example test, property
test, contract test, model/formal check, code-level proof, integration/e2e trace,
deployed proof, human review, or external second opinion.

Claims must not outrun evidence:

- Focused tests prove only their executions and predicates.
- A model check proves the model unless implementation conformance exists.
- A review packet proves attention, not truth.
- An artifact is evidence only after checking schema, provenance, and meaning.

## Formalization Seam

For safety-sensitive nodes ask:

- Can the definition project into a formal spec?
- Can types, assertions, contracts, properties, or model checks enforce it?
- What impossible state should be unreachable?
- What counterexample invalidates it?
- What is the refinement surface from model to code, tests, and traces?

Do not force formal verification everywhere. Make visible when a high-risk node
depends on prose alone.

## Forbidden Collapses

Do not collapse:

- artifact exists -> artifact is valid;
- definition exists -> graph is settled;
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
