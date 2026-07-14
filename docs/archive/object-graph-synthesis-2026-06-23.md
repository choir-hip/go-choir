# Object Graph as the Center of Choir

## 1. Thesis

Choir is not a chatbot with a window manager. It is a user-owned, persistent object graph with a frame-based computation model. Every surface — mail, calendar, news, slides, the desktop itself — is a *view* over that graph. The graph is the product. Everything else is rendering.

This is the conceptual refactor that has been waiting underneath all the half-started work: collapse the scattered app silos into a single substrate, then rebuild the apps as thin, typed projections onto it.

## 2. Philosophical root

The reason to own a personal computer is not to run a nicer SaaS. It is to stop having your data strewn across surfaces that are owned by someone else. The user must have a single, queryable, durable center for their information. From that center, the user can:

- remember, not just search;
- cite, not just quote;
- revise, not just prompt;
- transclude, not just copy.

Transclusion is the defining gesture. A chatbot lets you paste a URL. Choir lets you keep the object alive inside the document. The cited object remains a first-class node in the graph; it can update, be queried, and be re-cited. This is what makes Choir "more than chatbot-shaped."

The frame-based UX is not decoration. It visualizes the feed-forward nature of the work: the user is inside the stream of computation, not a commanding spectator outside it. Each revision is a frame. Each frame is a morphism on the graph. The user sees the morphism, approves or steers it, and the graph advances.

## 3. Category-theoretic lens

The object graph is a category.

- **Objects**: every persistent entity — texture documents, paragraphs, source entities, emails, contacts, calendar events, web captures, slides, audio segments, file blobs.
- **Morphisms**: transformations between objects — a revision, a citation, an import, a migration, a render, a send, an approval.
- **Identity**: canonical IDs (stable, versioned, URL-addressable). Each object is a value; identity is not tied to a file path or a row in a single-purpose database.
- **Composition**: morphisms compose. A research run produces source entities; a revision consumes them; a publish produces a canonical frame. The result is a path in the graph, i.e., a provenance trail.

**App views as functors.** An app is a functor from the object graph into a presentation category. Email maps the graph to a mailbox ordering; Calendar maps it to a timeline; Texture maps it to a structured document; Slides maps it to a deck. The functor must preserve the structure it needs (identity, citation, ordering) without mutating the source graph on its own.

**Limits and colimits.** A Texture document is a coproduct: paragraphs, media nodes, source references, and annotations assembled into a single object. A search result is a limit: the intersection of query constraints over the indexed graph. A feed is a filtered colimit over time.

**Natural transformations.** The same object should appear consistently across apps. A contact cited in an email should be the same object as the contact in a calendar invite. A web page captured in Universal Wire should be the same object as a source entity in a Texture document. The desktop shell is a 2-cell: the user observes the natural transformations and approves the next morphism.

This framing is not an academic overlay. It is a design test: if two apps cannot share the same object identity, the graph is leaking and the category is broken.

## 4. High-performance, secure code

The object graph must be:

- **local-first**: the user’s device holds the authoritative copy; the cloud is a replica and continuity service.
- **transactional**: every morphism is a typed, auditable transition. Rollback is a first-class operation (promotion/rollback is a red surface in Choir Doctrine for a reason).
- **queryable**: vector index, full-text index, and citation index are the same system. Source entities are indexed objects; citations are typed edges.
- **capability-based**: an app gets a capability to a slice of the graph, not blanket access. The email app can see mail objects; the calendar app can see calendar objects. Cross-app citation is a granted capability.
- **content-addressed where possible**: blobs by hash, objects by canonical ID. This deduplicates media, makes citations durable, and enables secure verification.
- **versioned**: every object carries its revision history. A Texture document is a sequence of frames; a mail message is an immutable object; a draft is a mutable object with a version chain.

The vector database is not a separate "AI feature." It is the semantic index of the object graph. It enables "find what I meant" across the whole graph, which is the prerequisite for the citation economy.

## 5. Mapping the open loops onto the graph

| Open loop | Current symptom | Graph-level diagnosis |
|---|---|---|
| Texture source citations | Source entities not carried forward; revisions stall | Object graph edges (`source_entities`) are not first-class morphisms; the agent loses the provenance trail |
| Email app freeze | UI locks; mail data is present but not rendered | Email app is a separate view with its own state machine; it should be a thin functor over mail objects |
| Universal Wire | Not working | It is a feed app without a shared news object graph; define the capture object and the feed query |
| PPTX renderer | Slides app cannot render PPTX | Missing functor from slide objects to a presentation surface |
| Mac desktop / local model | Mac app is a new surface | It should be a local replica of the graph, not a new app stack |
| Vector DB / citation economy | Mentioned but not central | Move it from feature to substrate: every object is indexed and citeable |
| Multi-agent orchestration | Model keeps changing | Agents are morphisms on the graph; stabilize the morphism protocol, not the agent topology |
| Conductor vs. Texture/Super | Authority boundary confusion | Conductor routes ingress into the graph; Texture/Super execute morphisms on the graph. The boundary is clear once the graph is the center. |
| Trajectory supervision | Texture stalls, malformed packets, missing sources | No typed supervisor object watches graph health and nudges actors without stealing authorship |
| Maild migration | Emails exist but UI stuck | The migration is correct at the storage layer; the view layer is not yet a graph view |

## 6. Implied decisions

If the graph is the center, the following decisions follow:

1. **Texture becomes the canonical object format**, not only for articles. Any object that can be represented as a structured document should be. A mail thread is a document; a slide deck is a document; a contact is a document fragment.
2. **App state is a projection.** `appContext` should be replaced by a capability to a graph query. The desktop shell should hold the graph, not the app windows.
3. **Source entities are graph edges.** They must be durable, versioned, and queryable across all apps. The fix just pushed is a step toward this, but the real fix is to stop treating them as metadata and start treating them as objects.
4. **Agents write to the graph.** A researcher does not "send a message" to Texture. It writes source objects and a proposed morphism. Texture applies the morphism if the user approves.
5. **Vector index is the citation economy.** Capture, search, and cite are the same operation: an object enters the graph, gets indexed, and becomes citeable.
6. **Supervision is a graph object.** Trajectory health, findings, and nudges are durable objects in the graph, not invisible controller state. The supervisor is a functor that reads the graph and writes only supervision objects and addressed messages.

## 7. Supervision as a graph object

The metaconductor/trajectory-supervisor open loop is not separate from the object graph. It is another kind of object in it.

Current Choir doctrine rejects a semantic babysitter that owns artifacts. The way to honor that is to make supervision itself a typed, auditable object with a narrow actuator set:

- **Sensors**: trace events, appagent events, source packets, mailbox state, work items, artifact validators, actor liveness.
- **Private model**: trajectory health state, open findings, actor responsibility map, recent action fingerprints.
- **Actuators**: addressed actor message, durable work item, user question, protocol-violation receipt, all-clear/settlement note.
- **Not allowed**: patch texture, edit app artifacts, invent source packets, rewrite findings, execute super work.

There are three supervision scopes, each a different object in the graph:

1. **Ingress conductor** — routes exogenous input into the graph. It is a router, not a supervisor.
2. **Trajectory supervisor** — watches one trajectory for liveness and protocol health. It sends addressed messages, never canonical mutations.
3. **Meta-conductor / portfolio supervisor** — allocates attention across many trajectories and work queues. It is a higher-order functor over trajectory-supervisor objects.

The trajectory supervisor is the missing piece for the Texture regression we just hit: it would notice that `source_entities` are empty, record a `malformed_researcher_packet` finding, and send exactly one idempotent message to the researcher asking for a proper `coagent_source_packet.v1`. The user remains the author; the supervisor is just a protocol immune system.

Settlement is also a graph object. "All threads clear" should be the result of a query, not a model phrase:

- no open obligations
- no pending mailbox items
- no active actor holding work
- artifact validators pass
- no blocking protocol findings

## 8. Next move

The next concrete step is not to rewrite the frontend. It is to write the graph schema: the object types, the edge types, the identity scheme, the capability model, and the supervision object types. Once that schema exists, the existing apps become refactor targets, and the half-started projects become clearly scoped.

For supervision, see `@/Users/wiz/go-choir/docs/design-conductor-supervision-protocol-2026-06-23.md` for the detailed protocol and first validator.

The open loops are not separate projects. They are symptoms of one missing center.
