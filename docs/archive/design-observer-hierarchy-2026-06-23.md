# Design: Observer Hierarchy and Self-Learning

This document addresses the question of who watches the meta-conductor, and how the supervision hierarchy terminates without infinite recursion. It is part of the conceptual refactor docset.

## 1. The hierarchy

Choir has a layered supervision model:

```text
User
  └── Meta-conductor (portfolio attention)
        └── Trajectory supervisor (single trajectory health)
              └── Agent / app / appagent (morphism producer)
                    └── Object graph (canonical state)
```

Each layer observes the layer below and sends messages. No layer owns the artifacts of the layer below.

## 2. Who watches the meta-conductor?

The **self-learning layer** watches the meta-conductor. It is not a permanent 8th layer. It is a periodic reflection mode that observes the meta-conductor's decisions and the system's overall behavior.

The self-learning layer is made of the same primitives as everything else: agents reading the object graph, producing findings, and proposing mutation transactions.

Components of the self-learning layer:

- **Policy review agents**: Periodically inspect meta-conductor decisions for bias, drift, or repeated failures.
- **Failure analysis agents**: Read rolled-back transactions, failed trajectories, and user complaints to propose policy changes.
- **Conjecture learning agents**: Update hypotheses about system behavior based on evidence. See `@/Users/wiz/go-choir/docs/handoff-conjecture-learning-fixed-point-2026-06-10.md`.
- **User review agents**: Surface summaries to the user for approval; the user is the root observer.

## 3. How the hierarchy terminates

The hierarchy terminates at the **user** as the root observer. The user is not a layer in the system; the user is the owner of the computer and the source of intention.

Within the system, the hierarchy terminates because the self-learning layer does not need a meta-meta-conductor running all the time. It runs:

- On a schedule (e.g., nightly review).
- On trigger (e.g., a failed transaction, a user complaint, a pattern of supervisor false positives).
- On demand (e.g., the user asks for a system review).

The self-learning layer is event-driven and sparse. It does not create an infinite tower of observers.

## 4. The object graph prevents infinite regress

All layers observe the same object graph. The trajectory supervisor reads `choir.supervision_observation` and `choir.supervision_finding`. The meta-conductor reads trajectory health. The self-learning layer reads all of it.

Because the state is shared, each higher layer does not need to re-implement observation. It just runs a different query over the same graph. The cost of adding a layer is the cost of the query, not the cost of duplicating the world.

## 5. Compute efficiency

We cannot afford a deep hierarchy of supervisors watching supervisors. The practical design is:

- **Three running layers**: user, meta-conductor, trajectory supervisor.
- **One periodic layer**: self-learning review.
- **No continuous meta-meta-supervision**.

The self-learning layer is expensive, so it runs:

- Only when triggered.
- On a small sample of trajectories.
- With bounded scope (e.g., "review all rolled-back transactions this week").

It produces policy changes as `choir.mutation_transaction` objects, which are themselves subject to the same verification and promotion protocol. This closes the loop without adding a runtime layer.

## 6. The self-learning layer is also a user tool

The self-learning layer does not operate autonomously. It proposes. The user decides. The layer's output is a set of candidate transactions and a summary of evidence. The user can:

- Approve a policy change.
- Reject it.
- Ask for more evidence.
- Run a candidate experiment.

This keeps the root observer as the user, not an unbounded AI hierarchy.

## 7. Relation to attention

`@/Users/wiz/go-choir/docs/design-attention-unifying-layer-2026-06-23.md` (or this conceptual layer) treats the conductor as the attention mechanism. The observer hierarchy is the depth of attention. The user decides where attention should go. The meta-conductor allocates attention. The trajectory supervisor maintains focus. The self-learning layer periodically asks whether attention is being spent well.

## 8. Relation to self-developing software

`@/Users/wiz/go-choir/docs/design-self-developing-software-2026-06-23.md` says the system must improve itself. The self-learning layer is the engine of that improvement. It observes the system's own behavior, identifies failures, and proposes changes to code, prompts, schemas, or policy.

## 9. Open questions

- How often should the self-learning layer run?
- What triggers it besides schedule and failure?
- How do we prevent the self-learning layer from overfitting to recent failures?
- How does the user review layer output without being overwhelmed?

These are answered by the first self-learning review mission, not by this doc.
