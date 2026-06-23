# Design: Attention as the Unifying Layer

This document describes the layer above the object graph, supervision, and mutation transactions: attention as the organizing principle of Choir.

## 1. The higher-level unity

The object graph is memory. Agents are computation. Supervision is self-awareness. The conductor is attention. The user is the source of intention.

Together they form a single system: a user-owned, persistent, attention-directed object graph. The higher-level unity is not another object. It is the relationship between the user, the conductor, and the graph.

> **Choir is a system that concentrates the user's attention into computation on a shared object graph.**

## 2. Attention as the organizing principle

In a complex system with many agents, apps, and data sources, the central problem is not storage or computation. The central problem is **where to point the system's attention**.

- The user has limited attention.
- The system has many possible actions.
- The conductor decides which actions the system should focus on.
- The object graph remembers what the system has paid attention to.
- Supervision notices when attention is misdirected or wasted.

## 3. The attention stack

| Layer | Function | Attention role |
|---|---|---|
| User | Intention | Decides what matters |
| Meta-conductor | Portfolio attention | Allocates attention across trajectories and queues |
| Ingress conductor | Routing | Directs incoming input into the graph |
| Trajectory supervisor | Focus | Maintains single-pointed attention on one trajectory |
| Agent | Action | Applies attention as computation on objects |
| Object graph | Memory | Stores the results of attention |

## 4. Single-pointed concentration

The ideal state is **single-pointed concentration**: the system is focused on the user's current priority. The meta-conductor has cleared away lower-priority work. The trajectory supervisor is watching the active trajectory. The agent is executing the next morphism. The object graph is advancing.

This is not always possible. The system may have many open obligations. The meta-conductor's job is to make the priority explicit, not to pretend there is only one thing.

## 5. Attention failures

The bugs we have hit are attention failures:

- Texture source entities were lost because the researcher agent's attention drifted from the source object to prose.
- Email freezes because the Email app keeps attention in its own state machine instead of the graph.
- Docs checker warnings because the system's attention is scattered across outdated vocabulary.
- Appchange bugs because the promotion transaction lacks attention on rollback refs and verifier evidence.

Fixing the object graph is how we make attention durable. When the graph is the center, attention leaves a trace.

## 6. Attention as an object

Attention itself can be an object in the graph:

```text
choir.attention_focus:
  canonical_id: obj:attn:focus:xyz
  metadata:
    user_id: string
    priority: int
    trajectory_id: string | null
    object_id: canonical_id | null
    reason: string
    started_at: timestamp
    expected_duration_ms: int
```

This makes attention explicit and queryable. The meta-conductor can read and write attention objects. The user can see what the system is paying attention to. The supervisor can detect stale or conflicting attention objects.

## 7. Relation to the observer hierarchy

`@/Users/wiz/go-choir/docs/design-observer-hierarchy-2026-06-23.md` describes who watches whom. The attention layer explains why the hierarchy exists: each layer is a different scope of attention. The user is the root. The meta-conductor distributes attention. The trajectory supervisor holds a single focus.

## 8. Relation to the object graph

`@/Users/wiz/go-choir/docs/object-graph-schema-2026-06-23.md` defines the objects. The attention layer explains why those objects matter: they are the durable trace of the system's attention. Without the graph, attention is ephemeral. With the graph, attention is legible.

## 9. Relation to self-developing software

`@/Users/wiz/go-choir/docs/design-self-developing-software-2026-06-23.md` says the system improves itself. The attention layer explains how: the system learns to allocate its own attention better by observing where attention produced good outcomes and where it failed.

## 10. Practical implication

The conductor should be designed as an attention allocator, not just a router. Every routing decision is an attention decision. Every work item is an attention object. Every user prompt is a request to redirect attention.

The object graph is the memory of what happened. The conductor is the attention that decides what happens next. The user is the source of both.
