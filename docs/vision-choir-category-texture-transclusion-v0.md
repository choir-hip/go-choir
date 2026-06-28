# Choir Vision: Textures, Transclusions, and the Category of Choir

**Date:** 2026-06-28
**Status:** vision document — architecture direction, not an implementation plan

## The One Idea

Texture is a generic versioned document with transclusion. That is the entire
type system. Everything else — articles, editions, autopapers, algorithms,
styleguides, renderers, schedules — is a texture distinguished only by its
morphism structure, not by a type tag.

## The Category

Choir is a category. The structure is:

- **Objects:** Textures. No type tags. An article, an edition, a styleguide,
  an algorithm, a renderer, a schedule, a source brief — all the same kind of
  object. What distinguishes them is what transcludes into them and what they
  transclude.

- **Morphisms:** Transclusions. `texture:doc-id` is the arrow. Composition is
  free — edition → edition → article is just following two transclusions. The
  object graph stores these.

- **Functors:**
  - `Embed: Choir → Vec` (Qdrant) — maps each texture to a vector, preserves
    proximity. This is semantic search.
  - `Publish: Choir → Corpus` (corpusd) — faithful functor, preserves
    transclusion structure. This is the publication store of record.
  - `Generate: Autopaper × Sources → Choir` — maps an autopaper (with its
    transcluded algorithm, styleguide, and context textures) plus source
    items to new article textures. This is the agent pipeline, executing
    inside actor handler activations (the execution substrate from
    mission-3c_2). Personalization is generative, not projective — the
    styleguide shapes what gets written, not how it gets reformatted.

- **Natural transformations:** The wire API. `η: PublishedChoir → UserView` —
  transforms "all published textures" into "what this user sees on this
  domain." This is graph traversal + rendering.

### The Yoneda Perspective

A texture IS its morphisms. An article is fully characterized by the editions
that transclude it, the sources it references, the entities it shares with
other articles. You don't ask "what type is this texture?" — you ask "what
transcludes into it and what does it transclude?" The answer tells you what
it is.

An autopaper is a texture that transcludes an algorithm, a styleguide, a
schedule, and a renderer, and is transcluded by editions. That structure IS
the definition. No type tag needed.

## The Clock

Cycles are the fundamental tick of the Choir clock. Each cycle:

1. Sourcecycled fetches new source items
2. Processor agents decide whether to synthesize articles
3. Texture agents write article revisions via LLM
4. Published articles get embedded into Qdrant and synced to corpusd
5. A wire editor agent creates a new edition texture transcluding this cycle's
   published articles

Each step is an actor activation — the agent pipeline runs inside actor
handler invocations on the execution substrate from mission-3c_2. A cycle is
not a timer event; it is a cascade of actor messages: sourcecycled sends to
processor, processor sends to texture agent, texture agent sends to wire
editor. The actor mailbox delivers each step instantly; the durable log
ensures no step is lost across crashes.

Editions accumulate. No data loss. Each edition is an immutable block in the
editorial chain. The full history of editions is the blockchain of editorial
decisions.

## Autopapers

An autopaper is a texture. It transcludes:

```
autopaper texture
  ├── transcludes → algorithm texture (what to cover)
  ├── transcludes → styleguide texture (how to write)
  ├── transcludes → schedule texture (when to publish)
  ├── transcludes → renderer texture (JS/CSS — how to display)
  ├── transcludes → context textures (background, previous coverage,
  │              entity profiles, reference material — anything)
  └── transcluded by → edition textures (one per cycle)
```

Creating an autopaper = creating a texture that transcludes an algorithm +
styleguide + schedule + renderer + context textures. The agent pipeline reads
the autopaper texture, follows ALL its transclusions, pulls those textures
into the generation context, and produces original articles informed by all
of them. Each edition transcludes the autopaper (provenance — "this edition
was produced by this autopaper following this algorithm").

### Personalization Is Generative, Not Projective

The styleguide is not a rendering transform applied to an already-published
article. It is a generative input. The agent pipeline generates NEW articles
from scratch, with the styleguide and context textures as inputs to the LLM.
Personalization means the agent considers the styleguide + algorithm + context
textures (related articles, background, the user's previous coverage, entity
history, source preferences) when deciding what to write and how to write it.

A user who transcludes a "focus on Latin American politics" algorithm texture
and a "skeptical investigative tone" styleguide and a "here's my previous
coverage of Brazil" context texture gets articles that are genuinely different
from the platform default — different topics, different angle, different voice,
informed by their history.

The user VM runs its own generation pipeline. The user's processor agents read
the autopaper texture, follow transclusions for context, and produce original
articles. The platform's articles and the user's articles are both textures in
the same graph, and they can transclude each other — the user's article might
reference and build on a platform article, transcluding it as a source.

### Style Implies Substance

The styleguide is not superficial. It does not merely control word choice or
formatting. Style implies substance — a coherent editorial worldview that
determines what gets seen, what gets questioned, what gets pursued, and what
gets ignored. "Skeptical investigative tone" is not about adjectives; it is
about which sources you trust, which claims you interrogate, which stories you
chase, which entities you track. The styleguide encodes an entire editorial
ontology.

This is holographic: each stylistic choice contains the whole. "Lead with the
human impact" implies an ontology of what matters (people over institutions),
which implies a sourcing strategy (eyewitnesses over officials), which implies
a story selection bias (crises over policy). Change one stylistic axis and the
substance shifts coherently. The style IS the substance, viewed from a
different angle.

Because style implies substance, a user's autopaper does not produce
"reformatted versions" of the platform's articles. It produces different
articles. Different topics surface. Different sources get cited. Different
claims get interrogated. The styleguide shapes the entire generative pipeline,
from source selection through synthesis to final article.

### Style Is Who You Center

Magazines differ not primarily in how they style their language but in how they
style people — who they choose to cover and not cover. Rolling Stone and The
Economist might cover the same trade summit, but one profiles the labor
activists outside and the other analyzes the tariff schedules inside. The
"style" is who you point your camera at. What you choose to cover IS who you
choose to center, and who you center IS your editorial identity.

The language style is downstream. You write differently because you've decided
different people matter. The styleguide is therefore not a prose style guide
at all — it is an editorial attention allocation policy:

- Who gets centered (which people, which voices, which protagonists)
- What gets covered (which topics, which events, which angles)
- What gets ignored (which stories are deemed unimportant)
- Who gets quoted (which sources are treated as authoritative)
- Who gets scrutinized (which entities are investigated, which are given
  benefit of the doubt)

This means the styleguide and the algorithm texture are not separable
concerns. The algorithm determines what sources you pull from; the styleguide
determines how you weight and center them. But they are the same thing viewed
from different angles. The algorithm is the styleguide's sourcing strategy made
explicit. The styleguide is the algorithm's editorial worldview made explicit.

"One texture or two" is a false choice — that's transclusion. The editorial
stance is a texture that transcludes sub-textures, each addressing a facet:
who to center, what to cover, how to source, what to ignore, how to write.
Those sub-textures can themselves transclude others. There is no fixed
granularity. The structure is a graph, not a choice between monolith and split.
The autopaper transcludes its editorial stance, which transcludes its sourcing
policy, which transcludes its entity preferences, which transcludes its
language register. Each node is a texture. Each edge is a transclusion. The
agent pipeline follows the graph and pulls whatever it finds into generation
context.

### Faithful Transclusion

When a story is particularly poignant — a major event, a breaking story that
transcends editorial framing — it can be faithfully transcluded into the
user's edition. The platform's article is included as-is, unmodified, via a
direct transclusion link. No regeneration, no restyling. The user's edition
carries it because the story matters regardless of editorial angle.

This is the complement to generative personalization: some stories are
editorial-frame-independent. They get carried over faithfully. Everything else
gets generated under the user's styleguide, producing genuinely different
articles because style implies substance.

### The Portfolio of Perspectives

Each autopaper is a perspective — a particular allocation of attention that
sees some things and misses others. No single perspective sees everything.
Every editorial stance is blind to something. That's not a flaw, it's
structural — attention is finite, style implies substance, and substance
implies blindness to what doesn't fit your ontology.

With many autopapers covering the same events from different centerings, the
blind spots don't align. What one autopaper misses, another catches. The
object graph makes this visible — the same event has nodes and edges from
every autopaper that covered it, each transcluding different sources, centering
different people, interrogating different claims.

The platform becomes a portfolio of perspectives, and the portfolio is less
deceived than any single perspective. Not because any individual autopaper is
unbiased (there's no such thing), but because the ensemble of biased
perspectives covers more of reality than any one could. This is the wisdom of
crowds, but structured — each autopaper is a coherent editorial stance, not
random noise, and the transclusion graph lets you see exactly where they agree,
where they diverge, and what each one missed.

As the platform grows — more users, more autopapers, more editions — it
produces more perspectives on the same events, more attempts to get less
deceived by partial attention. The value of the network is not more content
but more vantage points on the same content. Each new autopaper is a new
camera angle on reality.

### Wires Get Decoupled

The platform autopaper and user autopapers are independent production
pipelines. The platform publishes its editions; users publish theirs. They
share the same source pool (sourcecycled), the same object graph, the same
Qdrant index. But each autopaper's agent pipeline runs independently, driven
by its own transcluded configuration.

A user's autopaper can transclude the platform's articles as context — "here's
what the platform covered, now give me my angle on these stories." Or it can
ignore the platform entirely and cover completely different ground. The
transclusion structure makes this compositional: any texture can be pulled
into any other texture's generation context.

The default `choir.news` autopaper is just the `universal-wire-platform`
autopaper texture. A user's autopaper is owned by their user VM. Same objects,
same morphisms, same functors — different transcluded configuration.

## The Product

Choir is a publish-your-own-autopaper platform.

**Reading:** You browse `choir.news`, see published editions. Each edition has
its own URL — `choir.news/edition/2026-06-28-0411` — viewable while logged out.
These are the platform's editorial output, curated by the wire editor agent
each cycle.

**Publishing:** You create your own autopaper. Your algorithm texture defines
what you cover (categories, sources, embedding clusters, entity subscriptions).
Your styleguide defines how you write (tone, format, editorial voice). Your
context textures provide background — previous coverage, entity profiles,
reference material, even other autopapers' articles. Your renderer defines how
it looks (JS/CSS). Choir runs the same pipeline — sourcecycled → processor →
texture agent → edition — but following YOUR autopaper's transcluded
configuration, generating original articles informed by your styleguide and
context. Your autopaper gets its own URL: `choir.news/?i=yourname` or
`yourname.com` if you bring a domain.

All editions are public URLs. All are viewable logged out. All are textures in
the object graph. All get embedded in Qdrant. All can be transcluded by other
editions — mosiah can transclude choir.news articles into his autopaper, and
vice versa.

### The Pitch

> Don't just read the news. Publish your own newspaper. Define what you cover
> and how you cover it. Choir's agents do the reporting 24/7. You set the
> editorial direction.

### The Business Model

- Free: read the default choir.news autopaper
- Subscription: publish your own autopaper, custom domain, custom algorithm,
  custom renderer
- Discovery: browse other people's autopapers, follow their editorial taste

## The Surfaces

- `choir.news` — the default instance. Renders the latest editions from the
  platform autopaper with the default renderer. No personalization.
- `choir.news/?i=anyname` — a user's autopaper. Renders that user's latest
  edition with their renderer.
- `choir.news/edition/{cycle_id}` — a specific published edition. Public,
  no auth. Any autopaper's edition.
- `mosiah.org` — custom domain. Same as `choir.news/?i=mosiah` but with a
  vanity domain. The domain maps to an autopaper owner.
- `$anywebsite.com` — any domain can point at Choir. The domain resolves to
  an autopaper, which resolves to its algorithm + styleguide + renderer. Choir
  powers the content layer; the domain owns the brand.

## Rendering: Code Is a Transcluded Object

A texture can transclude code. A renderer is just a texture containing JS/CSS.
The autopaper transcludes its renderer texture alongside its algorithm,
styleguide, and schedule. The frontend reads the autopaper texture, follows
transclusions, gets the renderer JS, and executes it to render the edition.

The renderer is versioned like any other texture — you can revise it, blame
it, diff it. No separate "theme system" or "template engine." It's all
textures.

The current `UniversalWireApp.svelte` is a hardcoded renderer. In the general
case:

1. Resolve the domain/`?i=` to an autopaper texture
2. Follow transclusions to find the renderer texture
3. Load the renderer's current revision content (JS/CSS)
4. Execute it — it fetches editions and articles via the wire API and renders
   them

The Svelte app becomes a thin bootstrap loader: resolve autopaper → load
renderer → execute. Everything else is textures.

Since texture is a generic versioned ID, you can transclude anything: code,
configs, prompts, data, images, embeddings. The "news platform" is just one
application of the general substrate — it's the default autopaper running on
`choir.news`.

## The Wire API

The wire API is a natural transformation: `η: PublishedChoir → UserView`.

It is graph traversal, not special-cased document reading:

- "Latest editions" → find the most recent texture that transcludes the
  autopaper, follow its transclusions to articles
- "Search: X" → apply the Embed functor (Qdrant), rank by vector proximity
- "Category: Y" → filter by embedding cluster or entity node
- "Related to Z" → graph traversal from Z's entity nodes
- "Paginate" → walk the edition chain (newest to oldest)

No `Wire.texture` singleton. No `wire_publication_policy`. No edition-specific
types. Just objects and arrows.

## What Exists vs What's Needed

### Exists

- Texture documents, revisions, transclusion links ✓
- Object graph with entity nodes and edges ✓
- Corpusd publication store (currently `platformd` — rename pending per
  `docs/naming-rectification-2026-06-27.md`) ✓
- Cycle infrastructure (sourcecycled) ✓
- Qdrant vector DB (standing up) ✓
- User VM with Texture store ✓
- Texture write tools (agent-authored canonical revisions) ✓
- Actor runtime as execution substrate ✓ (mission-3c_2 settled — actor
  handlers run `executeActivation` synchronously, Go-channel mailboxes
  deliver updates, park-resume via memory snapshots. H030 database-polling
  heresy repaired. See `docs/mission-3c_2-actor-runtime-migration-real-v0.md`)

### Needed

1. **Per-cycle edition creation** — agent run at cycle boundary that creates
   a new edition texture transcluding this cycle's published articles. Replaces
   the deterministic `wire_publication_policy` code.
2. **Wire API rewrite** — graph traversal from editions, not single-document
   read. Paginated across editions.
3. **Qdrant indexing on publish** — embed each published texture, index by
   publication_id.
4. **Autopaper texture concept** — a texture that transcludes algorithm +
   styleguide + schedule + renderer. The agent pipeline reads these
   transclusions to configure itself.
5. **Instance resolution** — domain/`?i=` param → autopaper texture → owner.
6. **Renderer execution** — frontend bootstrap loader that resolves autopaper,
   loads renderer texture, executes it.
7. **Delete `Wire.texture` singleton** and all edition-reading code
   (`universalWireEditionTextureStories`, `autonomousPublishWireArticleToEdition`,
   `universalWireEditionSourcePath`, `universalWireEditionIncludedDocIDs`,
   `universalWireEditionResponse`).

## What's Wrong Now

The current code has `Wire.texture` — a single Texture document that acts as
a curated index. The wire API reads it, parses `texture:doc-id` transclusion
references, and fetches each referenced document. The edition is maintained by
`wire_publication_policy` — deterministic code that appends transclusion links
via string formatting, no LLM, no editorial judgment.

This is wrong because:

- **Sliding window, not archive** — when the edition is revised, old
  transclusions get dropped. 629 published articles exist but only 39 are
  visible.
- **Deterministic text production** — the `wire_publication_policy` code
  formats headlines and inclusion decisions mechanically, not editorially.
- **Special-cased object** — `Wire.texture` is treated as if it were a
  different kind of object, violating the category-theoretic structure. In the
  category, it's just an object with outgoing morphisms.
- **No personalization** — one edition for everyone. No algorithm texture, no
  styleguide, no per-user editorial direction.
- **No custom rendering** — the frontend is hardcoded Svelte, not a transcluded
  renderer texture.

The fix is not to patch `Wire.texture`. The fix is to delete it and replace it
with the general substrate: per-cycle edition textures, graph traversal, and
autopapers.

## Lineage

This vision emerged from mission-3c_2 (actor runtime migration), which
completed the actor runtime as the execution substrate. The H030 heresy
(database-polling instead of Go-channel mailboxes) was discovered and
repaired post-settlement, confirming that the substrate uses Go channels
for delivery and the durable log only for recovery. The next layer is the
product substrate: textures as universal objects, transclusions as
morphisms, autopapers as the product unit, and the wire API as graph
traversal.

The naming rectification plan (`docs/naming-rectification-2026-06-27.md`)
already identifies "cycle" as the canonical term for the fundamental tick.
Editions map directly to cycles. The plan also aligns `platformd → corpusd`
and `sandbox → computer` with the vision's functors and ontology.

## Not an Implementation Plan

This document is a vision. It defines the target architecture and the
conceptual framework. Implementation will be sequenced into missions, each
with its own mission doc, parallax state, and landing loop. The order matters:

1. Delete `Wire.texture` and deterministic edition code (cleanup)
2. Per-cycle edition creation (agent-authored editions)
3. Wire API rewrite (graph traversal)
4. Qdrant indexing on publish (semantic search)
5. Autopaper texture concept (algorithm + styleguide + schedule + renderer +
   context textures — the agent pipeline reads transclusions as generative
   inputs, not projection rules)
6. User VM generation pipeline (each autopaper runs its own processor →
   texture agent → edition pipeline, independently from the platform)
7. Instance resolution (domain → autopaper)
8. Renderer execution (frontend bootstrap)

Each step is independently valuable and deploys on its own.

## The Audited Computer

The vision above describes textures as the type system for publications. But
the assertion is deeper: **textures are the type system for everything,
including the computer itself.** This is the artifact program doctrine (see
`docs/memo-artifact-program-doctrine-2026-06-28.md`).

### The Equation

```
computer = choir_code(artifact_program)
```

- `choir_code` is the choir source at a specific version — inventoried by an
  SBOM (CycloneDX, via sbomnix now, FlakeBOM when we adopt Determinate Nix)
- `artifact_program` is the mutation transaction history — textures, the
  paragraphs that compute the computer's state
- The output is a running computer, deterministically

The current `data.img` is an opaque cache of this computation. The artifact
program is the source of truth.

### Textures Are Programs

A texture is not a static document. It is a program step — a transaction that
reads current state, applies a transformation, and writes the next state. Each
texture revision is a paragraph in the program. The program is the narrative
of the computer's evolution, written in the language of mutation transactions,
executed by the choir runtime.

This is why the category-theoretic framing matters: textures are objects,
transclusions are morphisms, and the program is the composition of morphisms
over time. The computer is the fixpoint of the program — the current state
that the program computes.

### What an Audited Computer Is

An audited computer is a computer where:

1. **Every state change is a typed transaction** — file writes, database
   mutations, config changes, promotions. Each transaction has an author, a
   timestamp, a type, inputs, and outputs. The tape IS the program.

2. **The interpreter is inventoried** — the choir code version that executes
   the program is recorded alongside every transaction. The SBOM lists every
   dependency, every version, every license. You know exactly what code ran
   to produce this state.

3. **The state is reproducible** — given the program version and the code
   version, you can compute the same computer anywhere. Replication is
   replicating inputs, not outputs. Migration is recomputation, not copying.

4. **The history is tamper-evident** — each transaction references the content
   hash of the previous transaction. Each blob is content-addressed. The choir
   code version is a git commit hash. Modifying any historical transaction
   changes all downstream hashes. This is a Merkle chain, like Git and Nix,
   extended to user state.

5. **Provenance is complete** — for any state at any point in time, you can
   answer: what changed, who changed it, what code executed the change, what
   was the previous state, what inputs were read, what outputs were written.
   Compliance questions become program queries.

### Why This Is a New Level of Information Technology

Current information technology treats computers as opaque state machines:
- You back up their disks (copying opaque state)
- You migrate them by copying disks (moving opaque blobs)
- You audit them by snapshotting and diffing (comparing accidents)
- You secure them by monitoring access (watching the outside)
- State is an accident of execution

Audited computers treat computers as deterministic computations over
audited programs:
- You replicate the program (inputs, not outputs)
- You migrate by recomputing (deterministic, verifiable)
- You audit by reading the tape (intentional, not accidental)
- You secure by typing and validating transactions (structural integrity)
- State is intentional — every byte has a reason

This is the same leap that Nix made for package management: from opaque
installed software to deterministic build outputs with complete dependency
graphs. We are extending that leap from the OS layer to the data layer, from
the package to the computer, from the build to the state.

### The Convergence

The same texture system that powers publications powers computers:

- An autopaper is a texture that transcludes an algorithm, styleguide,
  schedule, and renderer. The agent pipeline reads these transclusions and
  produces articles.
- A computer is a texture that transcludes a file manifest, a Dolt state
  snapshot, and a configuration. The choir runtime reads these transclusions
  and produces a running VM.
- A desktop file view is a texture that transcludes the same file manifest.
  The FileProvider extension reads it and produces a Finder folder.
- A mobile file view is the same texture, projected through iOS Files or
  Android DocumentsProvider.

One program, many projections. The texture graph is the single source of
truth. Every surface is a functor from the texture category to a display
category. Replication, distribution, and consensus operate on the graph,
not on projections.

### Success Definition

The vision succeeds when:

1. **A user's computer is a texture.** The file manifest, the Dolt state,
   the configuration — all textures, all transcluded, all versioned. The
   `data.img` is a derivation output, not a source of truth. You can
   reconstruct the computer from the texture graph alone.

2. **Every state change is a mutation transaction.** When the user writes a
   file, runs a job, or promotes a candidate, the state change is a typed
   transaction in the texture graph. The transaction has provenance: author,
   timestamp, code version, inputs, outputs. No state change happens outside
   the transaction system.

3. **The computer is portable.** Given the texture graph and the choir code
   version, you can compute the same computer on any node. Deployment is
   recomputation. Migration is recomputation. Desktop sync is a projection.
   Mobile access is a projection. No opaque blobs to copy.

4. **The computer is auditable.** For any state at any point in time, you can
   query the tape and the SBOM to answer: what is this state, how
   did it get here, who authorized each change, what code executed it, what
   dependencies were involved. Compliance is a query, not an investigation.

5. **The computer is verifiable.** Build the same texture graph on two nodes.
   Compare the content hashes of the computed `data.img`. If they match, the
   computation is deterministic and the decomposition is lossless. If they
   don't, the diff reveals the gap. Shadow replication validates this in
   production.

6. **The same system serves publications and computers.** The texture graph,
   the transclusion morphisms, the mutation transactions, the SBOM inventory
   — one system, two applications. The news platform and the personal
   computer are the same category, different functors. The infrastructure
   for one serves both.

When all six hold, Choir is not a news platform that happens to run user VMs.
Choir is an audited computer system that happens to publish news as its first
application. The audited computer is the product; the news platform is the
proof.

### The Deeper Goal

The deeper goal is not better deploys or faster sync or easier compliance,
though all of those follow. The deeper goal is a new relationship between
people and their computers.

Today, your computer is an opaque box. You don't know what state it's in. You
can't audit it. You can't reproduce it. You can't move it without copying
opaque blobs. You trust the box because you have no alternative.

In the audited computer, your computer is a program. You can read it. You can
audit it. You can reproduce it. You can move it by sending the program. You
trust it because you can verify it.

And because the tape is both the computation history and the learning history,
the computer is not just a program — it is a self-authoring program. It writes
itself through mutation transactions. Each transaction is a conjecture tested
by execution. The computer learns what it should be by acting on what it
believes, observing the results, and revising its beliefs. The tape is the
medium of learning.

This is the meaning of "promote typed artifacts, not opaque machine accidents."
Every byte of your computer's state has a reason — a transaction that wrote
it, an author who authorized it, a code version that executed it. The
computer is not an accident of execution. It is an intentional, audited,
reproducible, self-learning computation.

That is the new level of information technology.
