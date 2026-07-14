# Design Philosophy: Choir as a Personal Object Graph

## 1. The core idea

Choir is a user-owned personal computer modeled as a persistent object graph. Every piece of information the user cares about — documents, emails, contacts, calendar events, web captures, media, drafts, publications — is an object. Every transformation is a message or morphism between objects. Every surface is a view onto the graph.

This is not a metaphor. It is the architectural center that makes the other open loops resolve into a single design.

## 2. Why object-oriented now

Object-oriented programming has been misused for decades. In most codebases it means class hierarchies, inheritance taxonomies, and brittle encapsulation. That is not what we mean.

The real insight of object orientation is:

- **Identity**: objects are stable entities you can refer to, cite, and share.
- **Messages**: objects communicate by sending messages, not by direct mutation.
- **Composition**: complex systems are built from objects composed by reference, not by copying.
- **Encapsulation**: the internal representation can change without breaking references from the outside.

These four properties are exactly what Choir needs for a personal knowledge system. The user’s data is too complex and too interconnected for a document-centric or chat-centric model. Objects are the right granularity.

## 3. The lineage: Smalltalk + Xanadu + HyperCard + open source

Choir inherits from four traditions:

- **Smalltalk**: everything is an object; the user is inside the system; computation is live and inspectable.
- **Xanadu**: transclusion is the native linking model. You do not copy, you cite. The cited object remains alive and versioned.
- **HyperCard**: the user can author by direct manipulation, stacks are object collections, and the boundary between user and builder is thin.
- **Open source**: the user owns the runtime, the data, and the code. No vendor holds the keys.

The combination is: a live, personal, versioned, transclusive object graph that the user owns and can inspect, extend, and run on their own machines.

## 4. Object graph vs. chatbot

A chatbot is a sequence of turns. State is implicit, buried in context windows, and lost between sessions. Citations are paste and hope. The user is a spectator who types prompts.

Choir is a sequence of frames. State is explicit, named, and persistent in the graph. Citations are typed edges. The user is a participant in the computation, steering morphisms, approving deltas, and watching the graph evolve.

This is the difference between a conversation and a construction site.

## 5. Object graph vs. file system

A file system has identity by path, but no semantics. A document is a blob. A citation is a string. There is no durable notion of “this paragraph references that source object.”

The object graph has identity by object, with typed edges. A document is a tree of paragraph objects. A citation is a `source_ref` edge to a `source_entity` object. A revision is a `successor` edge. The graph is queryable by structure, not just by full text.

## 6. Object graph vs. database silos

Currently Choir has separate databases for mail, documents, trace, source cycle, users, and more. Each silo has its own schema and its own idea of identity. This is why the apps feel separate and the data feels strewn.

The object graph unifies the substrate. There is still storage specialization — Dolt for versioned app state, SQLite for lightweight host state, blob store for content, Qdrant for vector index — but all of them are servants of the same object model. The graph is the source of truth; the stores are projections.

## 7. The three primitives

Choir is built on three primitives:

1. **Object**: a persistent, typed, addressable entity. Has an ID, a kind, a body or content, and metadata.
2. **Edge**: a typed relationship between objects. Citations, revisions, authorship, containment, causality.
3. **Message**: a request for a morphism. An agent sends a message to an object or to another actor; the receiver decides how to mutate the graph.

Every app is a functor from the graph to a presentation. Every agent is a morphism producer. Every user action is a message.

## 8. Transclusion as the defining gesture

The object graph is not enough by itself. The defining gesture is transclusion: embedding an object by reference, not by copy. When a Texture document cites a web page, the page object remains a first-class node in the graph. If the page is updated, the citation can reflect it. If the page is deleted, the citation records the provenance.

Transclusion makes the graph a living system, not a graveyard of pasted snippets. It is what makes Choir a knowledge tool instead of a note-taking tool.

## 9. Local-first, federated, indexed

The object graph belongs to the user.

- **Local-first**: the authoritative copy lives on the user’s machine. The cloud is a replica for continuity and collaboration.
- **Federated**: a user can host objects on their own hardware or on a Choir host. The object identity and capability model make federation possible without a central landlord.
- **Indexed**: every object is embedded and indexed. The vector database (Qdrant) is a derived index, not canonical memory. The graph can survive a Qdrant rebuild; Qdrant cannot survive a graph loss.

## 10. Why this solves the current bugs

The Texture source-entities bug is not a prompt bug. It is an object graph bug. Source entities were treated as metadata inside a run instead of durable objects in the graph. The fix we pushed is a patch. The real fix is that source entities are objects with stable IDs, edges, and a Qdrant index.

The Email app freeze is not an Email bug. It is a graph-view bug. The Email app is maintaining its own state machine instead of being a thin functor over mail objects.

Universal Wire is not a news app bug. It is a missing object kind: the captured news item, with its canonical URL, content hash, and vector embedding.

Once the object graph is the center, each bug becomes a clearly scoped migration: define the object, define the edge, rewrite the app as a view.

## 11. The design stance

We will not rewrite everything at once. We will:

1. Name the object graph and its primitives.
2. Define the schema for the first object kinds.
3. Migrate one object kind at a time into the graph.
4. Rewrite the affected app as a view.
5. Redesign in place, never with a big-bang rewrite.

The object graph is not a new project. It is the missing center that makes the existing projects coherent.
