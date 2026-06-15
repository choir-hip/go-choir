# Why Texture

Date: 2026-06-15

Status: current explanatory support document and orientation seed. This file
inherits [choir-doctrine.md](./choir-doctrine.md) and the operating contract in
[AGENTS.md](../AGENTS.md). It names the Texture ontology that the hard-cutover
mission must propagate through code, prompts, tests, UI, docs, and the docs
checker.

## Spine

Turn-thread interfaces organize around the conversation. Texture organizes
around the work and what the work teaches.

Texture is the artifact layer that turns autonomous activity into directed
results and compounding learning. It is not only a document, not only a
version log, not only an app surface, and not only an agent workspace. Texture
is the standing state of a thing: what is currently believed, evidenced,
uncertain, intended, pending, and ready for judgment.

The unit of work is not a turn. The unit of work is the durable, owned,
legible artifact.

## Hard Definition

Texture is a single-writer-among-agents, append-only, transclusive,
present-tense artifact layer for directing results with autonomy and turning
action into reusable learning.

Human direct edits can create canonical revisions. Among agents, exactly one
Texture writer writes canonical Texture state. Other agents produce evidence,
proposals, receipts, faults, diffs, source packets, and promotion claims. The
Texture writer metabolizes that material into coherent artifact state.

Every revision is immutable. The latest revision is the default reading
surface, but prior revisions remain addressable, comparable, restorable, and
forkable. Revision is not an embarrassment to hide. Revision is the normal
shape of learning.

## Anti-Collapse

Texture must resist collapse into adjacent weaker categories:

- It is a persistent data structure, not a text box with an agent attached.
- It is a present-tense standing artifact, not a transcript archive.
- It is idea-level supervision, not a status dashboard.
- It is a shared semantic substrate, not one app's private document format.
- It is bandwidth transparency, not generative-interface churn.

The intersection matters. A transcript records what was said. A wiki stores
pages. A workflow engine routes steps. A dashboard reports status. A notebook
mixes code and notes. Texture holds the current state of the work and the
current learnings that should shape future work.

## Idea-Level Supervision

Long-running autonomous work cannot be supervised action by action. Showing
every agent message overloads the owner. Hiding every agent message creates
invisible state. Texture escapes that fork by turning agent work into evidence
for an evolving artifact.

Action-level corrections do not compound well. Idea-level corrections do.

If the owner says "change this sentence," one sentence changes. If the owner
says "this architecture is collapsing canonical state and derived state," an
entire class of future bugs becomes visible. Texture should make that kind of
learning durable.

A good Texture should answer:

- what we believed;
- what changed our belief;
- what we believe now;
- what remains uncertain;
- what evidence bears on it;
- what conjecture is active;
- what would falsify or weaken it;
- what decision is required.

## Transclusion

Texture is transclusive. A Texture can include source material, findings,
diffs, screenshots, audio, video, diagrams, app state, promotion evidence, or
other Textures without flattening provenance.

The default transclusion reference should pin a specific version. The UI should
still show when the referenced Texture has newer versions available, so a reader
can inspect the pinned evidence and notice live development without silently
changing the cited substrate.

The address shape is two-level:

```text
texture_id          -> living Texture object and latest head
texture_id/version  -> immutable revision address
```

Each version owns its transclusions. Later versions may inherit, add, replace,
or remove transclusions, but the previous version's provenance remains intact.

## Writer And Executor Boundary

Texture owns meaning and learning. Super owns privileged execution.

That boundary prevents narrative bias. The main executor has incentives to
defend the path it took. The Texture writer's job is to keep the artifact
legible, not to justify the worker's route. Researcher, source, app, super,
verifier, and candidate-world agents should produce evidence and claims that
Texture can incorporate, reject, qualify, or leave pending.

Texture can request execution when the artifact requires code, generated
assets, privileged actions, candidate computers, verifier contracts, promotion
evidence, or rollback preparation. But ordinary exogenous work should first
materialize as Texture-owned artifact state, not bypass Texture into execution.

## Runtime Boundary

Put semantics in oriented agents. Put invariants in runtime. Do not confuse
them.

Runtime should protect mechanical invariants:

- revision identity;
- authorization;
- base revision checks;
- append-only versioning;
- live and pinned references;
- provenance retention;
- publication visibility;
- canonical/candidate boundaries;
- promotion and rollback gates.

Runtime should not become a semantic decision tree. Phrase matching, role
keyword routing, hardcoded semantic classifiers, and hidden workflow gates make
the system brittle. The Texture writer needs orientation, affordances, and
evidence, not a maze of runtime if-statements pretending to understand the
work.

## Style As Texture

Style guides are Textures. A style Texture can be versioned, forked, owned, and
transcluded into another Texture.

This matters because style is not decoration. Style is part of the artifact's
meaning and audience fit. Treating style as hidden prompt text makes it hard to
inspect, revise, or share. Treating style as Texture keeps it in the same
learning substrate as facts, structure, evidence, and decisions.

## Boundaries

Texture does not own the whole execution tree. Super owns coding-agent trees,
candidate computers, implementer/verifier separation, package/adoption
evidence, and privileged mutation. Texture receives and directs the evidence
surfaced from that work.

Texture does not require a cold protocol cathedral before the product path
works. The minimal protocol should be learned from working implementation and
canonized only after proof.

## Out Of Scope

Inference optimization, cache design, diff-storage strategy, summary-versus-
full transclusion policy, universal cross-model representations, and media
rendering expansions are out of scope for this orientation document. Stability
comes first. Optimization is earned.

## Closing Line

The prompt surface is where you ask. Texture is where the work lives and
learns.
