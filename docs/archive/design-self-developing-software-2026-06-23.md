# Design: Self-Developing Software as Object Graph

This document states the key verification of the Choir object graph: it must enable self-developing software. It also shows that self-developing software and the automatic newspaper are the same shape.

## 1. The verification

The object graph is not an end in itself. It is the substrate that makes self-developing software possible. The real test is:

> Can Choir use its own object graph to understand, modify, verify, and improve itself?

If the answer is no, the object graph is just a nicer database. If the answer is yes, the object graph is the operating system of a personal mainframe.

## 2. Self-developing software

Self-developing software is a system that can:

- **Observe** its own structure: code, schemas, prompts, object graph, stored objects, traces, and transaction history.
- **Propose** changes: bug fixes, schema migrations, new object kinds, new app functors, prompt improvements, new tools.
- **Execute** changes safely inside isolated substrates: candidate VMs, Nucleus capsules, Dolt branches, git worktrees, Qdrant shadow collections.
- **Verify** changes with independent verifiers: tests, type checks, static analysis, runtime probes, user acceptance.
- **Promote** selected changes into canonical state via durable mutation transactions.
- **Rollback** failed or harmful changes using preserved base refs.

This is exactly the `MutationTransaction` protocol described in `@/Users/wiz/go-choir/docs/design-mutation-transaction-2026-06-23.md`.

## 3. Components of the self-development loop

| Component | Role in self-development |
|---|---|
| Object graph | Canonical representation of the system: code, schemas, objects, transactions, history. |
| MicroVMs | Durable computer boundaries for user, candidate, and platform computers. |
| Nucleus capsules | Bounded, effect-fenced sandboxes for experiments, verifiers, and bounded jobs. |
| Mutation transaction | Durable protocol for safe self-modification with base refs, stages, and rollback. |
| Trajectory supervisor | Watches the self-development loop for protocol health without owning artifacts. |
| Vector index | Semantic search over the object graph: find code, schemas, objects, and related changes. |
| Trace / evidence | Immutable audit trail of every self-development step. |
| Versioning | Every change is a versioned object; history is preserved and queryable. |

## 4. The automatic newspaper is the same shape

The automatic newspaper (Universal Wire / Wire) is not a separate product. It is a self-developing system where the objects are information, not code.

Compare the two loops:

| Self-developing software | Automatic newspaper |
|---|---|
| Observe code, schemas, objects, traces | Ingest news feeds, web pages, documents, transcripts |
| Propose a code/schema change | Propose a story, edition, or publication |
| Execute in a candidate VM / capsule | Execute in a processor pipeline / capsule |
| Verify with tests and verifiers | Verify with source checks, fact checks, and editorial review |
| Promote via mutation transaction | Publish via platformd / publication object |
| Rollback if harmful | Retract or supersede the edition |

Both are **24/7 agentic object graphs** that process, produce, and analyze information. The difference is only the kind of information: code objects vs. news objects.

This is the isomorphism. If the object graph can handle one, it can handle the other.

## 5. Why the object graph is the center of both

A system that can only self-develop code is narrow. A system that can only publish news is narrow. A system that treats both as objects in the same graph is general.

- A code change is an object: a `choir.mutation_transaction` with a diff.
- A news story is an object: a `choir.web_capture` with source entities.
- A verifier is a message: a `choir.supervision_finding` or a test result.
- A publication is a version-pinned reference: a `choir.publication` or a promoted release.

The same primitives — objects, edges, morphisms, transactions, supervision — work for both.

## 6. Implications for the open loops

- **Universal Wire** is not a news app. It is the first proof that the object graph can process and publish information objects autonomously.
- **Appchange / self-development** is not a separate system. It is the same object graph operating on code objects.
- **PPTX renderer** is a functor over slide objects, just as a news renderer is a functor over story objects.
- **Mac desktop / local-first** is a local replica of the graph, capable of running both self-development and newspaper loops locally.
- **Qdrant** is the semantic index over both code and news objects, enabling cross-domain search and citation.

## 7. The 24/7 agentic object graph

The end state is a single object graph that runs continuously:

- Agents ingest information from the outside world and from the system's own structure.
- Objects enter the graph, are indexed, and become citeable.
- Other agents process those objects, propose new objects, and produce artifacts.
- Verifiers check artifacts against invariants.
- Supervisors watch for protocol violations and nudge agents.
- Users approve or steer high-risk promotions.
- The graph advances.

This is the personal mainframe: a user-owned, persistent, self-improving, information-producing machine.

## 8. Acceptance criteria

The object graph is verified when:

1. An agent can propose a change to the Choir source code as a `choir.mutation_transaction` object.
2. The change runs in a candidate VM or capsule, producing verifier evidence.
3. The user can promote the change, updating the active computer.
4. The same agent can propose a news story as a `choir.web_capture` object.
5. The story flows through the processor, becomes a `choir.texture_doc` or `choir.publication`, and is published.
6. Both paths share the same object graph, transaction protocol, supervision, and indexing.

Until then, the open loops are separate symptoms. After that, they are the same machine.
