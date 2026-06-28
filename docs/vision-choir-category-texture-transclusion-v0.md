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
  - `Publish: Choir → Corpus` (platformd) — faithful functor, preserves
    transclusion structure. This is the publication store of record.
  - `Project: Choir × Styleguide → UserChoir` — maps textures to personalized
    variants. This is styleguide projection.

- **Natural transformations:** The wire API. `η: PublishedChoir → UserView` —
  transforms "all published textures" into "what this user sees on this
  domain." This is graph traversal + projection.

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
4. Published articles get embedded into Qdrant and synced to platformd
5. A wire editor agent creates a new edition texture transcluding this cycle's
   published articles

Editions accumulate. No data loss. Each edition is an immutable block in the
editorial chain. The full history of editions is the blockchain of editorial
decisions.

## Autopapers

An autopaper is a texture. It transcludes:

```
autopaper texture
  ├── transcludes → algorithm texture (what to cover)
  ├── transcludes → styleguide texture (how to present)
  ├── transcludes → schedule texture (when to publish)
  ├── transcludes → renderer texture (JS/CSS — how to display)
  └── transcluded by → edition textures (one per cycle)
```

Creating an autopaper = creating a texture that transcludes an algorithm +
styleguide + schedule + renderer. The agent pipeline reads the autopaper
texture, follows its transclusions to get the configuration, and produces
editions. Each edition transcludes the autopaper (provenance — "this edition
was produced by this autopaper following this algorithm").

The default `choir.news` autopaper is just the `universal-wire-platform`
autopaper texture. A user's autopaper is owned by their user VM. Same objects,
same morphisms, same functors — different algorithm texture.

## The Product

Choir is a publish-your-own-autopaper platform.

**Reading:** You browse `choir.news`, see published editions. Each edition has
its own URL — `choir.news/edition/2026-06-28-0411` — viewable while logged out.
These are the platform's editorial output, curated by the wire editor agent
each cycle.

**Publishing:** You create your own autopaper. Your algorithm texture defines
what you cover (categories, sources, embedding clusters, entity subscriptions).
Your styleguide defines how you present it (tone, format, editorial voice).
Your renderer defines how it looks (JS/CSS). Choir runs the same pipeline —
sourcecycled → processor → texture agent → edition — but following YOUR
algorithm texture instead of the platform default. Your autopaper gets its own
URL: `choir.news/?i=yourname` or `yourname.com` if you bring a domain.

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
- Platformd publication store ✓
- Cycle infrastructure (sourcecycled) ✓
- Qdrant vector DB (standing up) ✓
- User VM with Texture store ✓
- Texture write tools (agent-authored canonical revisions) ✓

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
completed the actor runtime as the execution substrate. The next layer is the
product substrate: textures as universal objects, transclusions as morphisms,
autopapers as the product unit, and the wire API as graph traversal.

The naming rectification plan (`docs/naming-rectification-2026-06-27.md`)
already identifies "cycle" as the canonical term for the fundamental tick.
Editions map directly to cycles.

## Not an Implementation Plan

This document is a vision. It defines the target architecture and the
conceptual framework. Implementation will be sequenced into missions, each
with its own mission doc, parallax state, and landing loop. The order matters:

1. Delete `Wire.texture` and deterministic edition code (cleanup)
2. Per-cycle edition creation (agent-authored editions)
3. Wire API rewrite (graph traversal)
4. Qdrant indexing on publish (semantic search)
5. Autopaper texture concept (algorithm + styleguide + schedule + renderer)
6. Instance resolution (domain → autopaper)
7. Renderer execution (frontend bootstrap)

Each step is independently valuable and deploys on its own.
