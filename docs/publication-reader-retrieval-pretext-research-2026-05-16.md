# Publication Reader, Retrieval, Pretext, And Transclusion Research

Date: 2026-05-16
Status: research/design input for the next MissionGradient
Operator: Codex

## Executive Summary

The first platform Dolt mission correctly proved the hard trust-domain
transition:

```text
selected private VText revision -> platform Dolt publication records
-> public route -> retrieval/citation/provenance facts
```

It intentionally did not build the final reader. The next correction is to move
public rendering out of `platformd`. `platformd` should remain a localhost-only
platform ledger/API service. Public URLs should be Choir product routes rendered
by the Svelte app and backed by proxy-mediated platform read APIs.

Target shape:

```text
browser /pub/vtext/... -> Caddy -> Svelte Choir shell
VText app in guest/public mode -> /api/platform/publications/resolve?... -> proxy
proxy -> internal platformd JSON API -> platform Dolt/artifact storage
```

For signed-out visitors, `/pub/vtext/...` should show the published piece in the
platform instance of Choir in the VText app, not in a static article template.
Signed-out VText is guest/read-only: the text is still a VText surface, but
editing, forking, transcluding into a host VText, citing into a private VText,
or proposing a new version becomes the funnel moment that asks the reader to
register or log in. For signed-in users, the same published piece should open in
the VText app as a reader/writer surface. They can read it, embed its source
spans inside their own host VText through a Pretext-powered transclusion
interface, edit their own version, and submit that version as a proposal back to
the original author. Mutating actions such as publishing, citing into a private
VText, saving a transclusion, proposing a new version, retracting, or
superseding still cross the auth and user-computer boundary.

Pretext is useful as the interface layer for transclusion: it should make
embedded source VText spans feel like readable inline material inside a host
VText, with stable source affordances, citation markers, and layout that can
survive edits. It is not the transclusion trust model. The currently public
`@chenglou/pretext` package is a text layout/measurement library. Choir still
needs its own publication refs, selectors, provenance, citation edges, proposal
edges, and transclusion resolver.

## Current Choir Ground Truth

Current implementation after the platform Dolt v0 mission:

- `platformd` writes platform publication, artifact, retrieval, citation,
  provenance, consent/review, verifier, and rollback rows to a separate
  localhost `dolt sql-server` primary.
- The signed-in publish product path is:

```text
POST /api/platform/vtext/publications
  -> proxy validates user
  -> proxy resolves active computer
  -> proxy reads selected private VText document/revision
  -> proxy posts public projection to platformd internal API
```

- The current public read path is:

```text
Caddy /pub/* -> platformd -> server-side HTML template
```

That last step is the piece to replace. It proved route reachability and
immutability, but it is not the intended product surface. It also makes
`platformd` a renderer, which muddies the service boundary. The platform service
should own facts and artifacts; Choir should own reading, interaction,
transclusion, and app UI.

## VText As Reader, Writer, And Proposal Surface

The VText app should become the surface for both authored documents and
published documents. A published VText is not a static article. It is an
immutable public version that can be read, cited, transcluded, forked into a
private host VText, and proposed back to the author.

Initial social/collaboration loop:

```text
published source VText version/span
  -> reader opens in VText app
  -> signed-in reader embeds source span in a host VText
  -> reader edits their own private derivative/proposal
  -> submit proposal back to source publication/author
  -> platform Dolt records proposal/citation/transclusion edges
  -> author's computer or author-side VText agent receives the proposal
  -> author or agent may accept, reject, research, revise, or supersede
```

If the author's computer is hibernating, platform delivery should eventually
wake or hydrate it through the same computer ownership infrastructure used for
ordinary product access. The proposal should first exist as a durable
platform-visible event/edge, then be delivered through a hot path to the
author's user-computer VText appagent. The platform database is the durable
ledger and recovery surface, not the live message bus.

When the human author is not present, the author-side VText writer agent can
still respond within its authority. For example, a proposal or citation from
another VText might contain new evidence that should cause the author-side VText
agent to call researcher, request coding work, create a candidate revision, or
prepare a reply proposal. Human review remains the authority boundary for
canonical publication changes unless the author has explicitly delegated policy
to their agents.

This is the beginning of the citation economy, but not yet CHIPS or ranking.
The first economic substrate is typed attention and collaboration:

- who cited or transcluded which exact version/span;
- what derivative/proposal was created from it;
- what evidence, agents, and revisions resulted;
- whether the author accepted, rejected, revised, or ignored it.

That graph is valuable before it has prices.

## External References And Lessons

### Pretext

The official `chenglou/pretext` repository describes Pretext as a pure
JavaScript/TypeScript library for multiline text measurement and layout that can
render to DOM, Canvas, SVG, and eventually server-side. Its API centers on
`prepare`, `layout`, richer line range walking, and a narrow rich-inline helper.
The repository explicitly frames it as avoiding DOM measurements that trigger
layout reflow.

As of this research pass, `npm view @chenglou/pretext` reports version `0.0.7`,
MIT license, and repository `https://github.com/chenglou/pretext.git`.

Design consequences for Choir:

- Use `@chenglou/pretext` for reader/editor layout, predictable text
  measurement, citation markers, inline chips, footnote sizing, virtualized
  blocks, embedded source spans, and transclusion previews inside a host VText.
- Do not treat Pretext as the semantic transclusion engine. It does not decide
  source identity, permission, revision immutability, provenance, proposal
  validity, or citation validity.
- Pin the dependency deliberately and run Playwright visual checks. Text layout
  is user-visible enough that upgrades should not float unobserved.
- Keep a DOM/CSS fallback for the first reader if Pretext integration threatens
  the publication boundary. The mission is the product reader over immutable
  platform records, not a library demo.

Sources:

- https://github.com/chenglou/pretext
- https://pretextjs.dev/

### Transclusion

MediaWiki's help page describes transclusion as including content from another
page using reference syntax, with source-side controls for what is included.
Wikisource separates a proofreading namespace from a reading namespace and
notes that validated or at least presentable text should be transcluded into the
main reading space. DocBook's draft transclusion model layers on XInclude and
then performs a second pass to fix IDs and cross-references.

These systems are not Choir's product model, but they expose useful invariants:

- transclusion is a reference-resolved reading operation, not a blind copy;
- the authoring/proofing surface and public reading surface can be separate;
- include/exclude controls matter because not every source node should appear
  in the target context;
- ID and cross-reference repair must be explicit when pieces are assembled from
  multiple sources;
- a preview before public publication is useful because cross-boundary rendering
  can fail even when source text is valid.

Design consequences for Choir:

- A Choir transclusion ref should point at an immutable public publication
  version/span, not a mutable private document head.
- Redaction/projection must create a public projection hash; the reader should
  never dereference private omitted content.
- Transcluded blocks need stable local IDs to avoid collisions in composed
  documents.
- Citation and transclusion can share selectors, but they are not identical:
  citation says "this supports or relates to this claim"; transclusion says
  "include this source material here."
- A proposal can include citations and transclusions, but it is a separate
  collaboration artifact: "I made this version in relation to your published
  version; consider it."

Sources:

- https://www.mediawiki.org/wiki/Help:Transclusion/en
- https://en.wikisource.org/wiki/Help:Beginner%27s_guide_to_transclusion
- https://docbook.org/docs/transclusion/transclusion.html

### Web Annotation And Provenance

W3C Web Annotation is directly relevant because published retrieval and citation
need exact target selectors, not only document-level links. Its model treats
annotations as body/target relationships and supports specific resource
selectors such as text positions, text quotes, fragments, ranges, and state.

W3C PROV remains relevant because public pieces need to answer who/what
produced a version, from what source, under which activity, and with which
agent/review evidence.

Design consequences for Choir:

- Platform reader APIs should return a publication bundle: immutable version,
  artifact manifest, rendered/document model, retrieval spans, citation edges,
  and provenance summaries.
- Retrieval spans should carry selectors over exact version/content hashes.
- Citation edges should render from platform records, not from best-effort
  Markdown links in the text.
- The reader can expose friendly UI, but the backend records should retain
  typed body/target/selectors/activity/agent structure.

Sources:

- https://www.w3.org/TR/annotation-model/
- https://www.w3.org/TR/prov-overview/

## Recommended Next Architecture

### Service Boundary

Replace:

```text
Caddy /pub/* -> platformd HTML template
```

with:

```text
Caddy /pub/* -> Svelte SPA
Svelte SPA -> public proxy read APIs
proxy -> internal platformd JSON APIs
platformd -> platform Dolt + artifact blobs
```

`platformd` remains bound to localhost and should not be directly reachable from
the public internet. The public internet reaches Choir's edge, static app, auth
service, and proxy APIs only.

### Public And Authenticated Reading

Signed-out visitor:

```text
GET /pub/vtext/:slug
  -> platform Choir instance
  -> VText app in guest/read-only published-reader mode
  -> public platform read API
  -> VText surface with citations/retrieval provenance
  -> edit/fork/transclude actions ask the user to register or log in
```

Signed-in visitor:

```text
GET /pub/vtext/:slug
  -> Desktop opens VText in published-reader mode
  -> same platform read API
  -> user can cite/transclude/edit their own derivative
  -> user can submit a proposal back to the source author
```

The public read should not require a private computer. Authenticated mutation
should.

### Platform APIs

Add proxy-public read APIs that do not require auth and only expose
platform-visible data:

- `GET /api/platform/publications/resolve?route=/pub/vtext/...`
- `GET /api/platform/publications/{publication_id}/versions/{version_id}`
- `GET /api/platform/publications/{publication_id}/citations`
- `GET /api/platform/retrieval/sources/{source_id}/spans`
- `GET /api/platform/retrieval/search?q=...`
- `POST /api/platform/publications/{publication_id}/proposals`

Add internal `platformd` JSON APIs behind those proxy routes:

- `GET /internal/platform/publications/resolve?route=...`
- `GET /internal/platform/publications/{id}/bundle`
- `GET /internal/platform/retrieval/search?q=...`
- `POST /internal/platform/publications/{id}/proposals`

The public proxy API should sanitize responses. It may return public owner
handle/display metadata, publication ids, version ids, hashes, routes, span
selectors, citation edges, and public content. It should not return private
computer ids, private paths, unpublished revisions, prompt traces, raw worker
logs, internal storage paths, or service-only rollback refs. Proposal submission
is authenticated and may read the submitter's private derivative VText revision,
but platform records should store only the selected proposal projection and
typed refs.

### Published Reader Data Model

The reader should consume one stable bundle shape, approximately:

```json
{
  "route": {"path": "/pub/vtext/example", "state": "active"},
  "publication": {"id": "pub-...", "title": "...", "state": "published"},
  "version": {
    "id": "pubver-...",
    "content_hash": "sha256...",
    "source_revision_hash": "sha256...",
    "published_at": "..."
  },
  "artifact": {
    "manifest_id": "artman-...",
    "media_type": "text/markdown; charset=utf-8",
    "content": "...",
    "render_model": []
  },
  "retrieval": {
    "source_id": "source-...",
    "spans": []
  },
  "citations": [],
  "proposals": {
    "can_submit": true,
    "source_publication_id": "pub-..."
  },
  "provenance": {
    "consent": [],
    "review": [],
    "attestations": []
  }
}
```

`render_model` can start as a derived Markdown/block AST. Pretext should operate
over the rendered block/inline model, not raw backend HTML.

### Transclusion Interface

The first transclusion interface should make a source span feel embedded inside
a host VText without losing source identity. A host VText revision should be
able to contain a structured transclusion node:

```json
{
  "type": "transclusion",
  "source_kind": "published_vtext_span",
  "publication_id": "pub-...",
  "publication_version_id": "pubver-...",
  "span_id": "span-...",
  "content_hash": "sha256...",
  "selector": {"kind": "block", "index": 3},
  "snapshot_text": "quoted public source text",
  "display": {"mode": "inline_source_card"}
}
```

The snapshot text makes the host revision stable. The source refs make it
auditable and refreshable. Pretext can handle measurement, inline flow,
source-card sizing, and citation marker layout.

### Retrieval Over Published Pieces

The first retrieval path should be real but deliberately modest:

- chunk published text into stable block/span records;
- store selectors, text hashes, token counts, and source/version refs in
  platform Dolt;
- materialize enough public span text or artifact offsets for retrieval;
- expose a public retrieval search API over published sources;
- return exact source/span refs and snippets;
- let VText/researcher cite returned spans into a private or public document.
- let a signed-in VText reader turn a retrieval span into a transclusion node or
  citation edge in a private host VText.

Do not build CHIPS ranking, paywalls, or public citation scoring in this pass.
If search needs ordering, use transparent deterministic order such as exact
match count, title match, publication time, or source id, and label it as a
retrieval debug order rather than a quality/economic rank.

### Publish UX

The VText editor needs a visible product path for publication:

- publish the current or selected historical revision;
- show a pre-publish preview from the same reader component;
- explain that only this version/projection is public and later private edits
  remain private;
- allow slug/handle selection when available;
- call `POST /api/platform/vtext/publications`;
- show the public URL, publication id, content hash, and source revision hash;
- open the published reader after success.

Retraction/supersession can be minimal but should be shaped: store route-disable
or supersession rollback refs and expose them to owner UI only if the mission has
time after the reader and retrieval path work.

### Proposal UX

The VText app also needs a visible path for reader proposals:

- open a published VText in read mode;
- let a signed-in reader create "my version" in their private computer;
- preserve source publication/version/span refs when transcluding text;
- submit the reader's selected derivative revision as a proposal to the source
  publication author;
- record proposal, citation, transclusion, provenance, and delivery state in
  platform Dolt;
- attempt author-side delivery through the product computer path when feasible;
- show the submitter a durable proposal id and delivery state.

The proposal should not mutate the source publication or author document. It is
a candidate relation until the author or author-side policy accepts it.

## Mission Implication

The next implementation mission should not merely "make the public page pretty."
It should replace the renderer boundary:

```text
platformd HTML page
```

with:

```text
VText public reader/writer + platform bundle APIs + Pretext transclusion
interface + proposal/citation/retrieval edges
```

That is the continuous path from the V0 proof to the intended publication,
retrieval, citation, transclusion, collaboration, and eventual citation economy
substrate.
