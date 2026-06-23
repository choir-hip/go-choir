# Choir's Conceptual Refactor: A Report

## Executive Summary

Choir is not a chatbot with a window manager. It is a user-owned, persistent object graph with a frame-based computation model. Every surface — mail, calendar, news, slides, the desktop itself — is a view over that graph. The graph is the product. Everything else is rendering.

This report describes the conceptual refactor that has been waiting underneath the last week of work. It explains what led up to it, what the model is, how to implement it quickly, and what it means for the future of software architecture.

The immediate trigger was a cluster of bugs: Texture source entities disappearing, the Email app freezing, Universal Wire not working, the PPTX renderer missing, and the docs checker accumulating nearly a thousand warnings. Each bug looked like a separate problem. The deeper pattern was that Choir had no shared center. The object graph is that center.

## 1. The Crisis: Symptoms Without a Center

### 1.1 Texture source entities

Texture documents were getting stuck at version 1 or version 2. They were not performing the iterative deep research expected of them. Source citations were missing. The researcher agent was encountering tool errors. The source entities that researchers produced were not being carried forward into the next revision.

The proximate fix was to add `texture_available_source_entities` to the list of durable metadata keys and to instruct the researcher prompt to emit `web_url` sources even when full content import failed. That fix worked. But it was a patch. The real problem was that source entities were not durable objects in the graph. They were JSON arrays inside run metadata, ephemeral, unindexed, and unqueryable.

### 1.2 The Email app freeze

The Email app was freezing the desktop. Other apps could not open until the page was reloaded. The storage layer was correct: the maild multi-tenancy migration had moved per-owner mailbox data into per-owner databases. The emails were present. But the Email app was maintaining its own state machine, its own `appContext`, and its own loading logic. It was not a view over mail objects. When the state machine broke, the whole desktop froze.

### 1.3 Universal Wire, PPTX, Mac desktop

Universal Wire was not working because there was no shared news object. A captured story had no canonical object type. The PPTX renderer was missing because there was no slide-deck object with a functor to the presentation surface. The Mac desktop was being built as a new app stack instead of a local replica of the graph.

Each of these was a symptom of the same missing center. Each app was a silo. Each had its own data model, its own identity scheme, its own state machine.

### 1.4 The docs checker

The docs checker reported 975 warnings. The bulk were retired vocabulary warnings: `vtext`, `continuation-level`, `AI workspace`, `chat`, `publishing system`, `workflow app`, `StoryGraph`, `Terminal app`, `Trace app`, `raw Terminal`, `parent_id`, `RunContinuation`, `lease`, and so on. These were not random errors. They were the trace of a system whose object model had changed without a corresponding migration of its documentation. The docs were drifting from the code because the code had no durable object model to migrate the docs against.

### 1.5 Appchange fragility

The `appchange` system had bugs. Promotions happened without complete capture, rollback refs were missing, and verifier evidence was not always attached. The system could propose changes to itself, but it could not yet make those changes safely. The conceptual model of a durable mutation transaction was missing.

## 2. The Last Week: From Symptoms to Pattern

The week began with the maild multi-tenancy migration. Emails were moving from a legacy shared `mail.db` into per-owner databases. The migration was idempotent and tracked completion in a `maild_migrations` table. CI passed. The user confirmed the migration was correct at the storage layer.

Then attention shifted to Texture. The document was stuck. The researcher was failing. Source citations were missing. The fix was twofold: a prompt change to make the researcher emit durable sources even when import failed, and a runtime change to carry `texture_available_source_entities` across appagent revisions. The test `TestTextureAgentRevisionRegistersMediaSourceEntities` passed.

But these were patches. The user asked to take a step back. The request was to analyze all open loops, to recognize the half-started projects, and to propose a conceptual refactor that centralized data in a system the user owned. The answer was the object graph.

The insight emerged from several sources at once:

- Smalltalk: everything is an object; the user is inside the system.
- Xanadu: transclusion is the native linking model; you cite, not copy.
- HyperCard: the user can author by direct manipulation; stacks are object collections.
- Category theory: objects, morphisms, functors, limits, colimits, natural transformations.
- Mainframe transactional discipline: durable logs, unit of work, rollback, recovery.
- The existing hybrid architecture doc: persistent computers, candidate computers, capsules, mutation transactions.

The pattern was that Choir had been building the pieces of an object graph without naming it. The refactor was to name it and make it the center.

## 3. The Last Day: A Design Sprint

The last day of work was a cascade of design documents. The user and the agent moved from high-level philosophy to concrete schema to implementation strategy. The documents were written as a single coherent docset.

### 3.1 The philosophy

The first doc established that object-oriented programming, properly understood, is exactly what Choir needs: identity, messages, composition, encapsulation. The user-owned object graph is the center. Apps are functors. Agents are morphisms. The vector database is the semantic index. The frame-based UX visualizes the feed-forward computation.

### 3.2 The schema

The second doc defined the object graph schema: canonical IDs, object kinds, edge kinds, capability model, persistence mapping, and Qdrant index shape. It named the first migration: `choir.source_entity` as a real object with a stable ID, cited by `choir.source_ref` edges, and embedded in Qdrant.

### 3.3 Supervision

The third doc defined the conductor supervision protocol. The trajectory supervisor watches graph health without owning artifacts. It writes only supervision objects and addressed messages. The meta-conductor allocates attention across trajectories. The user remains the author. Supervision is a protocol immune system.

### 3.4 Mutation transactions

The fourth doc connected the object graph to the hybrid architecture. Every change is a `MutationTransaction` object with base refs, stages, verifier evidence, commit, and rollback refs. Self-authoring becomes safe. The personal mainframe is born.

### 3.5 Versioning

The fifth doc unified Dolt versioning, Git versioning, and app revision IDs into a single object-version model. Publications pin object versions. Kind renames like `vtext` → `texture` become supersession edges and migration transactions.

### 3.6 Self-developing software

The sixth doc stated the key verification: the object graph must enable self-developing software. It showed that self-developing software and the automatic newspaper are the same shape: both are 24/7 agentic object graphs that ingest, process, produce, and verify information.

### 3.7 The virtual graph

The seventh doc refined the model: the graph is not a graph database. It is a virtual graph across Dolt, SQLite, Postgres, blob stores, and Qdrant. Navigation is agentic search over embeddings plus structured metadata plus provenance edges. Storage can change without the graph minding.

### 3.8 Attention

The eighth doc introduced attention as the unifying layer. The conductor is the attention allocator. The object graph is memory. Agents are computation. The user is the source of intention. The bugs are attention failures.

### 3.9 Observer hierarchy

The ninth doc addressed who watches the meta-conductor. The answer is a sparse, event-driven self-learning layer that periodically reviews the meta-conductor's decisions and proposes policy changes as mutation transactions. The user is the root observer. The hierarchy is shallow: user, meta-conductor, trajectory supervisor, agent.

## 4. The Unified Model

### 4.1 The object graph is the substrate

Every persistent entity is an object: texture documents, paragraphs, source entities, emails, contacts, calendar events, web captures, slides, audio segments, file blobs, supervision findings, mutation transactions, work items, users, computers.

Every transformation is a morphism: a revision, a citation, an import, a migration, a render, a send, an approval, a publish.

Every app is a functor from the object graph into a presentation category. Email maps the graph to a mailbox ordering. Calendar maps it to a timeline. Texture maps it to a structured document. Slides maps it to a deck. The functor preserves structure without mutating the source graph.

### 4.2 Apps are views

`appContext` should be replaced by a capability to a graph query. The desktop shell should hold the graph, not the app windows. The Email app should be a thin view over `choir.mail_message` and `choir.contact` objects. The Email freeze disappears because the app no longer maintains its own state machine.

### 4.3 Agents write objects

A researcher does not "send a message" to Texture. It writes source objects and a proposed morphism. Texture applies the morphism if the user approves. The boundary between agents is single-writer per object type. A researcher writes `choir.source_entity`. Texture writes `choir.texture_revision`. The supervisor writes `choir.supervision_finding`.

### 4.4 Source entities are graph edges

Source entities must be durable, versioned, and queryable across all apps. They are not metadata. They are objects. A citation is a `choir.source_ref` edge object. The vector index contains the embeddings of source entities. The citation economy is the same as the semantic index.

### 4.5 Supervision is a graph object

Trajectory health, findings, and nudges are durable objects in the graph, not invisible controller state. The supervisor is a functor that reads the graph and writes only supervision objects and addressed messages. The user remains the author.

### 4.6 Mutation transactions make self-development safe

Every change is a `MutationTransaction` object with base refs, stages, verifier evidence, commit, and rollback refs. A self-authored code change runs in a candidate VM or capsule, is verified, and is promoted atomically. A kind rename like `vtext` → `texture` is a migration transaction with a supersession edge.

### 4.7 Versioning is unified

Dolt versions the canonical state. Git versions the code. The object graph provides semantic versioning: objects have version chains, references are pinned by default, publications point to specific versions, and supersession records kind renames.

### 4.8 The graph is virtual and agentic

The graph is not a graph database. It is a virtual patchwork of objects across Dolt, SQLite, Postgres, blob stores, and Qdrant. Navigation is agentic search: vector similarity plus structured filters plus provenance edges. Node-to-node traversal is for within-domain provenance only.

### 4.9 Attention is the organizing principle

The conductor is the attention allocator. The meta-conductor distributes attention across trajectories. The trajectory supervisor maintains single-pointed focus on one trajectory. The bugs are attention failures. The object graph makes attention durable.

### 4.10 The observer hierarchy terminates at the user

The self-learning layer watches the meta-conductor, but it is sparse and event-driven. It proposes policy changes as mutation transactions. The user is the root observer. The hierarchy is shallow to remain compute-efficient.

## 5. How to Effectuate It Expeditiously

### 5.1 Do not rewrite everything

The refactor is not a big-bang rewrite. It is a sequence of typed migrations. Each migration adds an object kind, updates the apps that consume it, and deprecates the old representation. The first migration is the proof.

### 5.2 First migration: source entities

The first migration is to make `choir.source_entity` a real object in the graph.

Steps:

1. Define the `choir.source_entity` object kind with a canonical ID, metadata, and body storage.
2. Define the `choir.source_ref` edge object for inline citations.
3. Update the researcher to write `choir.source_entity` objects instead of embedding source JSON in packets.
4. Update Texture to read `choir.source_entity` objects and cite them via `choir.source_ref` edges.
5. Update the runtime to carry source entities as objects across revisions, not as metadata.
6. Index source entities in Qdrant.
7. Add verifier tests that source entities survive across Texture revisions.
8. Deprecate the old metadata representation.

This migration fixes the Texture bug and proves the object graph model.

### 5.3 Second migration: mail objects

Maild already has per-owner databases. The next step is to expose mail objects through the object graph API.

Steps:

1. Define `choir.mail_message`, `choir.mail_thread`, and `choir.contact` object kinds.
2. Build an object service adapter that reads from the maild databases and presents objects.
3. Rewrite the Email app as a view over mail objects.
4. Remove the Email app's state machine and `appContext` persistence.
5. Test that the Email app no longer freezes the desktop.

### 5.4 Third migration: web captures

Universal Wire needs a canonical news object.

Steps:

1. Define `choir.web_capture` object kind.
2. Update the source ingestion pipeline to write web capture objects.
3. Build the Wire feed as a query over `choir.web_capture` objects.
4. Index web captures in Qdrant.
5. Test that captured stories can be cited in Texture documents.

### 5.5 Fourth migration: slide deck objects

The PPTX renderer becomes a functor over `choir.slide_deck` objects.

Steps:

1. Define `choir.slide_deck` and `choir.slide` object kinds.
2. Update the Slides app to read slide deck objects.
3. Integrate the PPTX renderer as a presentation functor.
4. Test that slide decks can be rendered and published.

### 5.6 Clear the docs checker

Clearing the 975 docs checker warnings is a key gate. The strategy is:

1. Update current docs (README, AGENTS, choir-doctrine) to current vocabulary: object graph, texture, source entity, conductor, trajectory supervisor, mutation transaction.
2. Mark historical mission docs as evidence or historical so retired vocabulary warnings stop firing.
3. Add evidence docs to the mission graph so R3 warnings resolve.
4. Fix the remaining H3/H4 doctrinal warnings by updating the docs to the object-graph model.

This is a migration transaction on the documentation itself.

### 5.7 Implement the object service

The object service is a small API that presents the object graph to the rest of the system.

Endpoints:

- `CreateObject(kind, body, metadata)` → canonical ID.
- `GetObject(id, version)` → object bytes and metadata.
- `ListObjects(filter)` → list of object IDs.
- `PutEdge(from, to, kind, metadata)` → edge ID.
- `QueryObjects(vector, structured, provenance)` → agentic search results.

The service delegates to the appropriate store: Dolt for canonical app state, SQLite for host state, blob store for content, Qdrant for vector search.

### 5.8 Implement the trajectory supervisor

After the object graph is in place, the trajectory supervisor can be built.

Steps:

1. Define `choir.supervision_observation`, `choir.supervision_finding`, and `choir.supervision_message` object kinds.
2. Add sensors to the runtime: source packet validator, revision validator, actor liveness.
3. Implement the first validator: `researcher_packet_has_sources`.
4. Implement the supervisor loop: read observations, produce findings, send addressed messages.
5. Test that the supervisor catches the Texture source-entity bug before it reaches the user.

### 5.9 Implement the first mutation transaction

The first mutation transaction is a schema delta: adding `choir.source_entity` as a real object kind.

Steps:

1. Create a Dolt branch for the schema change.
2. Create a `choir.mutation_transaction` object with base refs.
3. Implement the migration.
4. Run verifiers.
5. Merge the Dolt branch.
6. Record the commit.
7. Keep rollback refs until TTL.

### 5.10 Iterate, do not redesign

The rule is: one object kind at a time, one app at a time, one transaction at a time. Each change is a typed delta with a rollback path. The system is redesigned in place, not rebuilt from scratch.

## 6. What It Portends for the Future of Software Architecture

### 6.1 The end of app silos

The dominant model of software today is a collection of apps, each with its own database, identity, and state. Choir's model inverts this. The object graph is the center. Apps are views. This is the architecture that makes transclusion, citation, and versioning native instead of afterthoughts.

### 6.2 The personal mainframe

A personal mainframe is a single logical computer owned by the user, with durable state, transactional self-modification, and sandboxed agents. The cloud is a replica and continuity service, not the landlord. This is the opposite of SaaS: the user owns the runtime, the data, and the code.

### 6.3 Self-developing software

Software that can modify its own code, schemas, and prompts is not science fiction. It requires a durable object graph, safe transactions, and verifiable effects. Choir's model shows how to build it without unbounded risk. The system can improve itself, and every improvement leaves a trace.

### 6.4 Attention-directed computing

The conductor as attention allocator is a new abstraction for operating systems. Instead of a user launching apps and managing windows, the user expresses intention and the system focuses computational attention on the relevant objects. The desktop is a visualization of attention, not a container for apps.

### 6.5 The automatic newspaper and beyond

The automatic newspaper is a special case of self-developing software: an agentic object graph that ingests information, produces artifacts, and publishes them. The same architecture applies to code, research, design, music, and any other domain where a system can observe, reason, and produce. The boundary between "software" and "media" dissolves.

### 6.6 The object graph as operating system

In this architecture, the object graph is the operating system. Storage, compute, networking, and UI are services around it. The OS is user-owned, versioned, and self-improving. This is the spiritual successor to Smalltalk, Lisp machines, and mainframes, but built for a distributed, agentic, AI-native world.

### 6.7 The role of the user

The user is not a spectator. The user is the source of intention and the final arbiter of high-risk promotions. The system amplifies the user's attention, memory, and agency. The user remains the author even when agents do most of the work.

### 6.8 Implications for AI

Current AI systems are stateless or rely on context windows. The object graph gives AI systems persistent memory, structured identity, and durable provenance. This makes AI less like a chatbot and more like a collaborator with a long-term model of the work. It is the missing substrate for reliable, recursive, self-improving AI systems.

### 6.9 Implications for SaaS

SaaS centralizes data and intelligence in a vendor's system. Choir's model decentralizes both. The user owns the object graph. The vendor provides components, hosting, and optional services. The economic model shifts from renting access to owning infrastructure.

### 6.10 A new design discipline

This architecture requires a new design discipline: every feature must be expressible as objects, morphisms, and transactions. If a feature cannot be expressed that way, it does not belong in the system. This is a rigorous filter. It prevents the accumulation of ad hoc silos and forces every change to be auditable, reversible, and shareable.

## 7. Conclusion

The last week of work on Choir was not a series of unrelated bug fixes. It was the slow discovery of a missing center. The object graph is that center. Once it is in place, the bugs become migrations, the apps become views, the agents become morphisms, and the system becomes self-improving.

The last day's design sprint produced the vocabulary and the schema. The next phase is implementation: one object kind at a time, starting with source entities. The payoff is not just a better Choir. It is a new architecture for personal computing: a user-owned, persistent, self-developing, attention-directed object graph.

This is the personal mainframe. This is the future of software architecture.

## Appendix: The Design Docset

The following documents were produced during the refactor:

- `@/Users/wiz/go-choir/docs/object-graph-synthesis-2026-06-23.md`
- `@/Users/wiz/go-choir/docs/design-philosophy-object-graph-2026-06-23.md`
- `@/Users/wiz/go-choir/docs/object-graph-schema-2026-06-23.md`
- `@/Users/wiz/go-choir/docs/design-conductor-supervision-protocol-2026-06-23.md`
- `@/Users/wiz/go-choir/docs/design-mutation-transaction-2026-06-23.md`
- `@/Users/wiz/go-choir/docs/design-object-graph-versioning-2026-06-23.md`
- `@/Users/wiz/go-choir/docs/design-self-developing-software-2026-06-23.md`
- `@/Users/wiz/go-choir/docs/design-virtual-agentic-graph-2026-06-23.md`
- `@/Users/wiz/go-choir/docs/design-attention-unifying-layer-2026-06-23.md`
- `@/Users/wiz/go-choir/docs/design-observer-hierarchy-2026-06-23.md`
- `@/Users/wiz/go-choir/docs/report-conceptual-refactor-2026-06-23.md`

This report is the synthesis of that docset.
