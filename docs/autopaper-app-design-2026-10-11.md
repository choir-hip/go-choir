# Autopaper app design: the news UI on the protocol

> **Revised 2026-10-12** ([re-engineering decisions](re-engineering-decisions-2026-10-12.md)): Publishing changes from one permalink per document to one immutable permalink per version, with chevrons and a "new version available" control; captures share Texture's block model.

Date: 2026-10-11. Status: **design draft for owner review; nothing built.**
Mutation class of this document: green. The work it proposes ranges from
orange (the app) to red (publication routes, the Texture citation
surface, record publication); each slice names its own class below.

Owner's words (2026-10-11): "AI news demo -> create textures which are
displayed through either a rewritten autopaper app (currently called
universal wire or world wire) which is a news ui which opens up individual
stories as textures which transclude their sources. Should be built on the
protocol ofc. Worth designing first. Management should be able to send
records/scores to texture agent, not modify texture docs ofc."

Inputs: [Autopaper software design](autopaper-sw-design-2026-10-09.md)
(principles, storage, W3), the
[Autopaper metamission](definitions/choir-autopaper-metamission-2026-10-11.md),
[W1](definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md)
(complete), [W2](definitions/choir-autopaper-w2-living-textures-2026-10-11.md)
(blocked on the protocol), the
[precommitment protocol design](precommitment-records.md)
(§3 addressing, §4 predictions, mandatory since the owner's answer of
2026-10-11, §10 one schema), and
[P2d](definitions/choir-protocol-p2d-texture-addressing-2026-10-11.md)
(P2d1 on staging; P2d2 held for the owner after review round three).

## Summary

**What I found.** I am confident about the first four points: each comes
from reading code or from a read-only GET on staging.

1. The current app (`frontend/src/lib/UniversalWireApp.svelte`, 666 lines)
   is a list of cards built on the retired processor's story schema
   (`types.WireStory`: lead/supporting/contrary/context manifests, "wire
   style" projections). It is visible only to signed-in users. Its server
   side (`internal/platform/universal_wire.go`) hard-codes two
   boilerplate "claims" on every story. If one route is broken, the
   whole list fails, and the list makes several store queries per route.
   On staging it returns `{"stories":[]}` (Store B was reset on 2026-10-09).
2. The Texture renderers already do most of what a story page needs.
   They render numbered source references with popovers that quote the
   source, expanded references that show the quote inline, and links to
   other Textures pinned to a revision with a "Newer version available"
   indicator (`frontend/src/lib/texture-source-renderer.ts:378-500`). A
   public, logged-out read of a published Texture also works today:
   `GET /api/platform/publications/resolve` is served from corpusd
   without auth (staging answered 404, not 401, for an unknown route).
   The bundle it returns carries source entities, citations and the full
   revision history.
3. **Today no source is actually transcluded.** The AI news demo's latest
   revision on staging (test computer demo-p2c, document `34f4781e`,
   version 7) has 10 source entities. All 10 are `web_url` with a
   `whole_resource` selector and an empty snapshot, so each popover can
   show only a title and a URL. `FetchURL` keeps nothing it fetches
   (`internal/researchtools/researchtools.go:790-860`). Captures (W1) are
   the only fetched content that persists, and neither the Texture desk
   nor management can read one. `ReadCapture` is on research's surface
   only (`internal/yaegikernel/choir.go:243-275`).
4. **Publishing mints a new permalink every time.** `PublishTexture`
   creates a fresh publication id and a route derived from it on every
   call (`internal/platform/service.go:203-217`), and the route's
   object id derives from that path (`publication_graph.go:145`). A
   living Texture republished after each revision would therefore put a
   new item on the front page each time. This contradicts the design's
   "every later revision appears at the same permalink"
   (autopaper-sw-design §2), and it has to be fixed before anything is
   automatic.
5. No API route reads commitment records. Records live in each computer's
   embedded graph (`internal/store/commitment.go`), and the only reader
   is the in-cell `choir.Pack()`. Showing claims and scores requires
   publishing them to Store B, as protocol design §10 already plans.

**What I recommend.**

- **Rewrite the app.** Reuse the Texture renderers, the Source viewer,
  the version-history component and the corpusd publication substrate.
  Delete the old app, its route and its story schema; the reasons are in §3.
- **Make every story a Texture, and make the front page a Texture too.**
  The front page starts as a computed list of the paper's published
  Textures. Later it becomes an *edition Texture* whose Texture desk
  places stories by transcluding them at pinned revisions. Management
  ranks stories by sending that desk records and scores, never by
  writing the page. This follows the owner's rule and reuses the
  pin-and-indicator machinery that already exists.
- **Sources are pinned captures, quoted.** A citation names a capture by
  id and content hash, plus a verbatim quote. Publishing carries the
  quote, so the public reader shows it inline without ever touching the
  paper's computer. A runtime check that the quote appears in the
  capture body costs no model call. This is the first provenance check
  the paper can show readers.
- **Claims and inferences are records the text points to.** A
  forward-looking statement in a story is a Precommit by the edition,
  referenced from the text by a source reference whose target is the
  record. Scores are separate records by named scorers. The reader shows
  them grouped by scorer and never as a verdict.
- **Order the work so something is visible early.** Build the reader on
  published Textures first (a new slice, W3a). It needs no protocol
  station, while P2d2 is held. Next, capture citations, which use the
  research-to-Texture report path that exists today. Living stories
  come after P2d, predictions after P3, scores in the reader after P4,
  and the unattended paper after P5. This reverses the metamission's
  W2 → W3 order for the reader half of W3. That is an owner decision
  (open question 3).

**How sure.** I am sure of the code findings above. I am moderately sure
of the data-flow design: it composes stations that are drafted or
landed, but P2d2's run model is open. I am least sure of the claims
layer (§4.5), because I have not checked whether news claims yield
enough scoreable predictions; §6 proposes the prototype that would
check it.

## 1. Constraints this design inherits

From the Autopaper design and the protocol, unchanged:

- Only the Texture desk writes a Texture. Management sends records,
  including scores, to `texture:<doc>`. It never writes a document,
  including through ProductAPI. P2b enforces this in
  `tools_product_api.go:176-183`.
- Engineering never addresses a Texture. It goes through management
  (owner answer, protocol design §9).
- Reads of published Textures never touch the paper's computer (July
  cause C6). A list read degrades item by item and costs a bounded number
  of queries (design §0 constraint 4).
- One authority per fact: one route per document, one revision chain, one
  publication state.
- Show what the world indicates; do not adjudicate (design principle 1).
  Scores are shown by scorer and never merged into a house verdict
  (protocol §10).
- As little structure as possible (principle 2). The app fixes how things
  are displayed, not what a story is or how many Textures cover one.
- Every Cast, Ask and Escalate carries a prediction (owner, 2026-10-11).
- The UI reads only GET routes.

## 2. What exists today (evidence)

### 2.1 The Universal Wire app

`frontend/src/lib/UniversalWireApp.svelte`, registered as
`universal-wire` in `frontend/src/lib/apps/registry.ts:133-141`:

- Loads `GET /api/universal-wire/stories` only when signed in, and polls
  every 30 s (lines 23-87). Logged-out readers see an empty page.
- Renders `types.WireStory` cards: headline, dek, freshness, a "wire
  style" projection and the first two `claims` (lines 300-360).
- "Open source" builds a source entity from `manifest.lead[0]` and
  launches the Source app (lines 128-223).
- Opening a story launches the full Texture editor with
  `read_owner=universal-wire-platform`. The proxy serves such reads from
  corpusd rather than the guest
  (`internal/proxy/handlers.go:1781-1797`), but only for the retired
  platform owner constant.

Server side: `internal/proxy/universal_wire.go` (auth required) →
corpusd `ListUniversalWireStories` (`internal/platform/universal_wire.go:62-145`).
For each `choir.public_route` (limit 100), it loads the publication
bundle, then the version object. It **returns an error for the whole list**
if any route's metadata or version is malformed (lines 84-114). Each story
gets these hard-coded claims (line 168): "This Texture article is
published in the canonical corpusd world-wire store." and "Source
provenance is carried by the canonical publication object graph." They are
displayed in the claims slot. This is a discovered heresy: the claims are
fabricated, not displayed data.

`App.svelte:293-303, 564-620` keeps a dead `/universal-wire/publications/`
public reader that only prints "Legacy publication links were removed."

Two Playwright specs cover the old app:
`frontend/tests/universal-wire-app.spec.js` and
`universal-wire-staging-acceptance.spec.js`.

### 2.2 How a Texture shows its sources

- A revision's structured body (`body_doc`) has `source_ref` nodes with a
  `source_entity_id` and a `display_mode` of `numbered_ref` or
  `expanded_ref` (`texture-structured-editor-doc.ts:190-197`).
- `renderInlineSourceRef` (`texture-source-renderer.ts:378-402`) renders a
  numbered marker with a popover that holds the title, a quote, any media
  and an "Open source" button. The expanded mode puts the quote inline as
  a block. The quote is `transclusion.snapshot_text`, or else a selector's
  `text_quote`/`exact` (`selectorTextQuote`, lines 59-71). With neither,
  there is nothing to quote.
- Texture-to-Texture: markdown `[label](texture:<doc>@<rev>)` renders a
  related-Texture reference with "Version pin vN" and "Newer version
  available" when the entity's current revision is ahead of the pin
  (lines 418-500). This is the design's "transclude specific versions,
  with a subtle indicator" (autopaper-sw-design §2), already drawn.
- The Texture desk cites with ApplyTexture's `insert_source_ref`, whose
  `source_entity` target is `{kind: "web_url", uri}`. It has no selector
  and no quote (`internal/yaegikernel/surface.go:39`).
- "Open source" goes to the Source app (`ContentViewer.svelte`, id
  `content`). In published mode it shows the entity's reader snapshot or
  excerpt (lines 38-47). Otherwise it imports the URL into the owner's
  content store, which is a write and is not available to logged-out
  readers.
- The source graph (`choir.source_entity`, `choir.source_ref`) is per
  revision in the computer's embedded store
  (`internal/store/texture_source_graph.go`).

Probe (read-only GET, staging, test computer demo-p2c-2026-10-11):
document `34f4781e`, head revision `b5c28b8c` (version 7, author
`appagent`), 4,212 characters, 10 source entities, all of the shape
`{"target":{"kind":"web_url","uri":...},"selectors":[{"kind":"whole_resource"}]}`,
snapshot length 0. The content also shows markers placed mid-phrase
("Google's[1]", "The[4]"). That is a citation-offset problem worth a
problem record of its own; it is not investigated here. The
2026-10-11 report saw the same thing from the other side: "citations
fell from ten to four ... research reported what it saw in search
results without importing the pages"
(`reports/autopaper-first-station-and-the-protocol-2026-10-11.md:19`).

### 2.3 Publishing and public reads

- `POST /api/platform/texture/publications` → proxy fetches the document
  and revision from the guest → corpusd `PublishTexture`
  (`internal/proxy/platform_publish.go:143`, `internal/platform/service.go:153`).
  Each call creates a new publication, a new version and a new route
  `/pub/texture/<slug>-<shortid>`, along with proposal, consent, review,
  activity and attestation objects. **No call ever adds a version to an
  existing route.**
- `GET /api/platform/publications/resolve?route=` is public. It returns
  `PublicationBundle` (`internal/platform/types.go:185-198`): route,
  publication, version, artifact, citations, source entities,
  transclusions, policy, provenance and `version_history` (the full
  chain with per-revision hashes, optionally platform-signed).
- A logged-out visit to `/pub/texture/...` opens the Desktop with a
  read-only Texture window (`App.svelte:480-494`,
  `Desktop.svelte:1107`). `TextureVersionHistory.svelte` renders
  `version_history` from the bundle.
- The internal platform publish route
  (`/internal/wire/platform/publications/texture`,
  `internal/proxy/wire_platform_publish.go`) works only for the retired
  `universal-wire-platform` owner.

### 2.4 Autopaper and records routes

- `/api/autopaper/sources/schedules` is the only Autopaper route. It is
  guest-served (`internal/apihandler/routes.go:31`) and on management's
  ProductAPI allowlist (`tools_product_api.go:195`).
- Captures: corpusd `POST /internal/platform/captures` and
  `GET /internal/platform/captures/bodies/<sha256>` are reachable only by
  bound guests and internal callers (W1b2). No browser route exists.
- Records: no `/api/...` route reads `choir.commitment_record`. Desks see
  their own through `choir.Pack()`. The record body
  (`internal/types/commitment.go`) still carries `Scores` inside the
  record, which protocol §10 replaces with separate score records from P4.
- Management's in-cell surface has `Precommit`, `Resolve`, `Cast`, `Note`
  and `ProductAPI`, but not `ReadCapture`. The Texture surface has
  `ReadDoc`, `ApplyTexture`, `Cast`, `Ask`, `Note` and `Precommit`, but
  not `ReadCapture` (`internal/yaegikernel/choir.go:243-275`).

## 3. Reuse or rewrite

| Piece | Verdict | Reason |
|---|---|---|
| `UniversalWireApp.svelte` | **Rewrite** (delete) | Built on the retired processor's `WireStory` schema. Signed-in only. Shows fabricated claims. Opens stories through the retired `read_owner` path. Its source button guesses from `manifest.lead[0]`. Nothing in it is about Textures, revisions or records. Its masthead and column CSS can be carried over by hand. |
| `/api/universal-wire/stories`, `ListUniversalWireStories`, `types.WireStory` | **Replace** | The whole list fails on one bad route, it makes several queries per route, it is auth-only, and it fabricates claims. The replacement filters by publisher, degrades per item, is public, and makes a bounded number of queries. |
| `App.svelte` legacy `/universal-wire/publications/` reader | **Delete** | Dead code: it only prints an error. |
| `read_owner=universal-wire-platform` read path, `/internal/wire/platform/publications/texture` | **Retire** with the platform-computer constants (autopaper-sw-design §9) | Tied to the retired owner. Public reads go through route resolution, which is owner-agnostic. |
| Texture renderers (`texture-source-renderer.ts`, `texture-structured-editor-doc.ts`, `texture-markdown-renderer.ts`, `source-contract.ts`) | **Reuse and extend** | Pure functions that already render source references with quotes, related-Texture pins and the newer-version indicator. Extend them with a `web_capture` target and a `commitment_record` target (§4.4, §4.5). |
| `TextureEditor.svelte` (2,703 lines) | **Reuse for the owner only**; not the story reader | Readers need a slim read-only page. The editor stays the owner's working surface and the Desktop's `/pub/texture/` fallback. |
| `ContentViewer.svelte` (Source app) | **Reuse** | Its published mode already prefers the entity's snapshot. Add capture metadata (outlet, capture time, hash, link to the original). |
| `TextureVersionHistory.svelte` | **Reuse** | Already renders `version_history` from the public bundle. |
| corpusd publication substrate (`PublishTexture`, `PublicationBundle`, public resolve) | **Reuse, and change one thing** | Republishing a document must add a version to its existing publication and route, not mint a new one (§2.3). Also decide which of the per-publish proposal, consent and review objects a living Texture needs on every revision. |
| `sourceEntityLaunchPayload` | **Reuse** | Already the one launcher from a source entity to the right viewer. |

## 4. The design

### 4.1 Objects

These are existing kinds except where marked.

| Thing | Representation | Where |
|---|---|---|
| A story | A Texture document, one desk per document; several Textures may cover one story | the paper's computer; published to Store B |
| The front page | v0: a computed list of the paper's published Textures. v1: an **edition Texture** whose body is a sequence of pinned Texture references `[headline](texture:<doc>@<rev>)` | same |
| A fetched item | `choir.web_capture` (W1), body by sha256 on host disk | Store B |
| A citation of a capture | a `source_ref` node whose source entity targets `{kind: "web_capture", capture_id, content_hash}` with a `text_quote` selector (**new target kind, no new object kind**) | the revision; carried into the publication |
| A claim or inference in a story | a `choir.commitment_record` Precommit by the edition, referenced from the text by a `source_ref` whose target is `{kind: "commitment_record", record_id}` (**new target kind**) | computer, then published to Store B (§4.5) |
| A source's own claim ("Reuters reports X") | an observation record attributed to the outlet (protocol §10 "source" emitter) | same |
| A score | a separate score record naming its scorer, from P4 (protocol §10) | same |
| A directive to a Texture | a Cast or Note to `texture:<doc>`, with a prediction (P2d, P3) | computer only (working state) |

### 4.2 Data flow on the protocol

```text
 sources ──poll (runtime, no model)──▶ Store B: choir.web_capture + body by hash      [W1, done]
                                           │
             observation record (system provenance, capture ids) ──▶ management:<owner>   [P5]
                                           │
 management turn: read captures, decide stories, decide spend
   ├─ Precommit story size ("≥10 captures on this story in 24h? p=0.5") — resolves by count  [P3/P5]
   ├─ new story:   Cast texture:new  {objective, capture ids, Expectation}                     [gap, §4.3]
   ├─ known story: Cast texture:<doc> {capture ids, Expectation:
   │                "next revision cites ≥1 of these? p=0.7"}                                  [P2d, P3]
   ├─ ranking:     Note/score records to texture:<edition>                                     [P2d, P4]
   └─ scores:      score records on claim records; a Note to texture:<doc> when one matters    [P4]
                                           │
 Texture desk (sole writer) wakes on the directive
   ├─ ReadCapture (new on its surface) or a research Cast (with Expectation)                   [C, P3]
   ├─ ApplyTexture: revision with source_ref → web_capture + text_quote
   ├─ Precommit: claim and inference records, subject = this doc and node
   └─ the directive's prediction resolves by count when the revision commits                   [P3]
                                           │
 publish (runtime, on revision commit of an Autopaper document)                                [W3b]
   → Store B: new version on the document's one route; quotes, capture pins,
     the history chain; the revision's published claim records and their scores                [W3a, W4]
                                           │
 Autopaper app (GET only, corpusd, never the paper's computer)                                 [W3a]
```

Publishing is runtime machinery, not agent speech, so it is not a record
(protocol §6). Whether a document auto-publishes is a property of the
document, set when it is opened. For example, management's `texture:new`
carries `publication: auto`, and the paper's owner can turn it off. Manual
publishing stays manual for every other Texture (autopaper-sw-design §2).

### 4.3 "Management sends records and scores, never writes", made concrete

| Management wants to... | Act | Station | Wakes the Texture? |
|---|---|---|---|
| feed new evidence to a story | Cast to `texture:<doc>`, capture ids in the body, prediction about the next revision | P2d (+P3) | yes |
| tell a story something without asking for work | Note to `texture:<doc>` | P2d | yes (bounded) |
| open a new story Texture | Cast to `texture:new` with seed, capture ids and prediction; the commit mints the document, its trajectory and the directive together, with management as the emitter | **not covered by P2d**; propose it as P2d3 or fold it into P5 | yes (first revision) |
| rank stories on the front page | Note or Cast to `texture:<edition>`: "lead with X; X's capture count rose from 4 to 19", with a prediction about the edition's next revision | P2d (+P3) | yes |
| score a story's claim | a score record on the claim record (P4). Recommendation: **a score alone does not wake the Texture**; management adds a Note when a score should change the text (a claim was contradicted). This keeps P2d's bound meaningful and stops scoring from turning into revision churn. | P4 | only through the Note |

P2d admits only Cast and Note to a Texture, and it holds a bound of four
directives per owner request. A steady story needs more than that. P2d
names this as its own residual: P5 re-bases the bound on the story. Until
then, a living story revises at most four times per owner prompt.

Until `texture:new` exists, the only way a new Texture starts is
`/api/prompt-bar` (`tools_product_api.go:178-179`). A prompt-bar document
records the owner as its author. W2-review1 already flagged that a
management call there would carry the wrong provenance. This design
recommends that the owner open the first story Textures, which is
truthful provenance (§4.7). Management should not do it through
prompt-bar.

### 4.4 Sources transcluded inline, with provenance

A capture citation in the revision's `source_entities`:

```json
{
  "source_entity_id": "src_…",
  "kind": "web_capture",
  "target": {"kind": "web_capture", "capture_id": "choir.web_capture/…",
             "content_hash": "sha256:…", "uri": "https://outlet/…"},
  "selectors": [{"kind": "text_quote", "exact": "Pricing is $0.10 input and $0.50 output per million tokens"}],
  "display": {"mode": "numbered_ref"},
  "provenance": {"outlet": "letsdatascience.com", "captured_at": "2026-10-07T14:02Z",
                 "published_at": "2026-10-07T12:40Z", "writer_computer": "computer-…"}
}
```

- **Pinned:** the content hash is the version. Captures are
  content-hash identified (`objectgraph/registry.go:53`), so a citation
  can never drift. A later capture of the same URL with a different body
  is a different capture. The UI can show "newer capture of this page
  available" with the same indicator style as Textures.
- **Quoted, not republished:** the publication carries the quote, which
  the reader shows inline (`renderSourceTransclusionBody` already does).
  The full body is never served publicly. "Open source" shows capture
  metadata, the quote in context (a bounded window around it) and a link
  to the original. How much context to show is open question 4
  (copyright).
- **Checked for free:** when ApplyTexture commits, the runtime checks
  that the quote occurs in the capture body (after whitespace
  normalization) and records `quote_verified: true/false` on the entity.
  No model is involved. The reader shows a small "quote checked against
  the capture" mark. This is design principle 1 in practice: verifying
  means checking provenance. The count of verified quotes per revision is
  also a free resolver for any prediction about citation quality.
- **Trusted writer:** shared captures are whatever the writing guest
  says (W1 residual), so publishing resolves `capture_id` only among
  captures written by the paper's own computer, or by a writer on a
  configured trust list.
- **Needed on the desk surface:** `ReadCapture` on Texture and
  management (read-only, shared content). The `insert_source_ref` source
  entity needs a `web_capture` target and a `text_quote` selector. This
  is slice C below.

### 4.5 Claims and inferences as records in the reader

Protocol §10: "The claims that we extract from news and the inferences
drawn from them are precommitments we score over time." Three emitters
appear in a story:

| In the text | Record | Scored how |
|---|---|---|
| "ABC News reported on 4 October that Trump named Clayton…" | observation attributed to the outlet (emitter kind `source`), evidence = the capture | later captures confirm or contradict; builds the outlet's track record |
| "Mistral says open weights will follow at the end of October" | observation attributed to Mistral, and optionally a Precommit by the edition: "Open weights published by 31 Oct? yes p=0.6" | by count or date when a capture shows the weights (or does not by 31 Oct) |
| "The task force has no statutory authority, so it is unlikely to issue binding rules this year" | Precommit by the edition (emitter kind `edition`), deadline 31 Dec | by the answer, scored by any scorer |

How the text points at a record: the Texture stages `choir.Precommit` in
the same cell as its ApplyTexture, then inserts a `source_ref` whose
source entity targets `{kind: "commitment_record", record_id}`.
`Precommit` returns a string at staging (`choir.go:923`). *Hypothesis to
verify:* that string is the record's final id, derivable from the cell
identity, so the turn can reference it before commit. If it is not, the
reducer needs to bind staged record refs into the revision at commit.

Why a source reference and not a new node type: the renderer, the
numbering, the popover and the source panel already exist, and so does
the publish path for source entities. A claim then reads like a cited
sentence, and its "source" is the record.

The popover (wireframe in §4.6) shows:

- the questions and the probabilities committed;
- the deadline and the commit time (store-assigned, hashed);
- the status (open, resolved, or past due);
- the scores, **grouped by scorer** ("this edition's management, Jev:
  confirmed, 1 Nov", "edition B: qualified");
- the evidence each scorer cited (capture links).

The reader shows no aggregate.

Publication of records: when a revision publishes, the records its text
references, and every score record on them, go to Store B with the
publication version. They carry emitter identity, subject references,
content hash and store-assigned commit time, and are marked unsigned
until per-computer keys exist (protocol §9, item 8). Desk-to-desk records
(management's directives, research Casts) stay in the computer, where
they are working state, and appear only in the owner lens. This is the
same split protocol §10 makes ("Where records live").

### 4.6 What the UI reads, and how it looks

All routes are GET, all are served by corpusd from Store B, and none
touches the paper's computer. The `/api/platform/` prefix already means
"corpusd, may be public" (`/api/platform/publications/resolve`). The
guest-served `/api/autopaper/` stays for the owner's control routes, such
as schedules.

| Route | New? | Returns | Station |
|---|---|---|---|
| `GET /api/platform/autopaper/front?publisher=<computer>&cursor=&limit=` | new (replaces `/api/universal-wire/stories`) | v0: one card per published document (headline, dek, route, latest version time, version count, source count, open-record count), newest revision first; per-item degrade with a `skipped` count; at most two queries per page. v1: the publisher's edition Texture's latest published version, resolved into cards. | W3a |
| `GET /api/platform/publications/resolve?route=` | exists | the story bundle; extended with capture pins, verified quotes and the referenced record ids | W3a, C |
| `GET /api/platform/publications/history?route=` | optional | version list with per-version revision notes, if `version_history` in the bundle grows too large | later |
| `GET /api/platform/records?subject=<publication>` | new | the published claim and observation records for a story, with score records grouped by scorer | W4 |
| `GET /api/platform/captures/<id>?around=<quote>` | new | capture metadata (outlet, URL, times, hash) and a bounded window around a quote. Only for captures cited by a published version. Never the whole body. | C |
| owner lens: `GET /api/records?subject=texture:<doc>` (guest, signed in, owner only) | new | desk records about the story: directives, predictions, resolutions, calibration | P4 |

Front page (v0 computed; v1 laid out by the edition Texture):

```text
┌──────────────────────────────────────────────────────────────────────┐
│ AUTOPAPER                         Sun 11 Oct · 14 stories · 3 min ago │
├──────────────────────────────────────────────────────────────────────┤
│ Anthropic releases Claude Haiku 5.5 at $0.10 / $0.50     ● updated 12m│
│ Two dated reports agree on the price; Anthropic's own page 404'd.     │
│ 5 sources · v4 · 1 open prediction                                    │
├───────────────────────────────────┬──────────────────────────────────┤
│ Mistral previews Large 4;         │ Trump names Clayton to lead a     │
│ open weights promised for October │ "Super Intelligence Force"        │
│ 2 sources · v2 · 1 open           │ 3 sources · v3 · sources disagree │
│ prediction                        │ on membership                     │
├───────────────────────────────────┴──────────────────────────────────┤
│ Also covered: Cantwell framework · Congress leaves · Google–Constella│
└──────────────────────────────────────────────────────────────────────┘
```

"● updated 12m" means the story's latest published version is newer than
the one the edition pinned. This is the existing newer-version indicator,
restyled. In v0 there is no pin, and the card shows the latest version.

Story page (opens the story Texture at its latest published version, at
its one permalink):

```text
AUTOPAPER ‹ front page
Anthropic releases Claude Haiku 5.5 at $0.10 / $0.50
v4 · revised 12 min ago · history ▸ v1 Oct 7 09:10 · v2 · v3 · v4

Anthropic released Haiku 5.5 on 7 October at $0.10 input and $0.50
output per million tokens [1][2]. Its own announcement page returned
a 404 when checked, so the primary source is unconfirmed [3].
  ┌ [1] Let's Data Science · captured 7 Oct 14:02 UTC ──────────────┐
  │ "Pricing is $0.10 input and $0.50 output per million tokens for │
  │  prompts up to 100K"                       ✓ quote checked       │
  │ capture sha256:3f2a…  ·  open source  ·  original ↗              │
  └──────────────────────────────────────────────────────────────────┘
Mistral says open weights will follow at the end of October ◆
  ┌ ◆ Prediction by this edition · committed 6 Oct 18:20 UTC ───────┐
  │ Open weights published by 31 Oct?   yes 0.6                     │
  │ status: open, resolves 31 Oct · scores: none yet                │
  └──────────────────────────────────────────────────────────────────┘

Sources 5 · Predictions 1 · Observations 4 · History 4
```

Claim popover after resolution:

```text
◆ Open weights published by 31 Oct?  committed yes 0.6
  resolved 30 Oct · evidence: capture of huggingface.co/mistralai/… ↗
  scores
    this edition (management, Jev)   confirmed
    edition "B" (their scorer)       confirmed
```

Owner lens (signed-in owner of the paper only; same story page, extra
panel, guest-served):

```text
Why this revision: Cast from management, 11 Oct 13:58
  "3 new captures on Haiku 5.5 pricing"
  predicted: next revision cites ≥1 of them, p=0.7 → resolved yes (2 cited)
Research rounds: 2 (predictions 3, confirmed 2, contradicted 1)
Spend on this story: 41k tokens · 3 revisions
```

Desktop and public use the same components. They are mounted as the
`autopaper` app in the Desktop registry, replacing `universal-wire`, and
as full-page public routes for logged-out readers. A newspaper permalink
should open as a page, not as a Desktop window. URL shape is open
question 5.

### 4.7 The AI news demo as the first content

Today the prompt "Research the news on and around AI for the last week"
produces one Texture: a brief with five topic bullets and about 10
numbered sources, none of them quoted (§2.2).

**Stage 1, no protocol dependency.** The demo runs on the paper's
computer, and its document is published as the paper's first **edition**:
one Texture whose sections are stories. The owner then opens one story
Texture per item with the prompt bar (truthful owner provenance), seeded
with the item and its source URLs. A short script with the QA key can
send these prompts. Each story Texture's desk researches and writes its
story. The paper publishes each one, and the front page (v0) lists them.
Nothing here is throwaway: owner-opened Textures stay valid content, and
the edition becomes v1's front page once its desk can be fed.

**Stage 2, capture citations (slice C).** The story Textures Cast
research, which already reports to Texture through producer reports.
Research polls or fetches into captures, and the Texture cites captures
with quotes. To do this, research's fetches must land as captures:
`FetchURL` writes through the same corpusd capture route as
`PollSource`, so every page a story cites persists in Store B. This
makes the demo's sources transcludable and gives the paper its first
pinned, quoted, verified citations.

**Stage 3, management (P2d, `texture:new`, P3).** The owner prompts
management instead: "Cover AI news this week; one Texture per story."
Management Casts research for the slices, each Cast with a prediction
("how many in-window items will this slice find?"). It clusters the
results, opens story Textures with `texture:new`, opens the edition
Texture, and feeds it ranking Notes. The demo then exercises every arrow
in §4.2 except polling.

**Stage 4, living (P5).** The same paper keeps the stories alive from
polls. The demo's own baseline already contains forward-looking items
that resolve within weeks and serve as the first scoreable inferences:
Mistral's open weights "at the end of October" and the AI Agent
Accountability Act's expected introduction
(`evidence/ai-news-haiku-baseline-2026-10-10/report.md`), plus the Gemini
Deep Research agent shutdown on 23 October from the staging demo
document.

## 5. Stations, in an order that is visible early

| # | Slice | Depends on | Visible result | Class |
|---|---|---|---|---|
| 0 | **Prototype A** (§6): the reader renders a real published bundle with the existing renderers. Local frontend against staging GETs; this is the CLAUDE.md "fast frontend visual iteration" case. | nothing | a story page with today's sources | none (prototype) |
| 1 | **W3a, the reader on published Textures**: republishing adds a version to the document's one route; the public `front` list; the `autopaper` app (Desktop and public page); delete the old app, its route, `WireStory` and the dead reader; Playwright specs replaced. | nothing in P1–P5 | choir.news shows a front page of owner-opened AI news story Textures, logged out | red (publication and route projection), orange (app) |
| 2 | **C, capture citations**: `ReadCapture` on Texture (and management); `web_capture` target and `text_quote` selector in `insert_source_ref`; the free quote check; `FetchURL` writes captures; publish carries pins and quotes; `GET …/captures/<id>` window | W1 (done); research-to-Texture reports (exist) | sources are quoted inline with "quote checked" marks | red (Texture citation surface, Store B writes) |
| 3 | **P2d2**, then **`texture:new`** | P2 (landed), the owner's decision on P2d-review3's run model | stories revise when management feeds them; "updated" badges; history strip; then management opens stories | red |
| 4 | **W3b, auto-publish** on revision commit for documents opened with `publication: auto`; edition Texture v1 front page | 1, 3 | the paper updates itself after each revision; front page curated by its own desk | red |
| 5 | **P3**: predictions on every Cast, Ask and Escalate; count resolver | P2 | owner lens shows each directive's prediction and outcome | red |
| 6 | **P4**: scores as separate records; calibration in packs; owner records GET | P3 | owner lens shows calibration per desk | red |
| 7 | **W4, the record in the reader** (new): claim and observation records referenced from text, published with their scores; `GET …/records`; claim popovers | P4, slice 2 (the citation path), slice 4 (publish) | readers see predictions, deadlines and scores by scorer | red |
| 8 | **P5 (W2 re-based)**: poll observations to management; story size by count; unattended | P4, P2d | the paper runs from polls for two hours unattended (W2 acceptance) | red |

Slices 1 and 2 can run in parallel with the protocol stations, and
neither touches the Texture run model that P2d2 is held on. Slice 1 is
the metamission's W3 minus auto-publish, moved ahead of W2 (open question
3). The metamission's value criterion rewards it: "maximize grounded,
current Textures readable at permalinks".

## 6. Riskiest assumptions, and what to prototype first

| # | Assumption | Why risky | Evidence today | Prototype / check |
|---|---|---|---|---|
| R1 | A Texture can cite a capture with a verbatim quote that matches the body | The citation surface has no quote. Models paraphrase. Markers already land mid-phrase. | 0 of 10 demo sources carry a quote (§2.2) | **First.** On one test computer, research polls or captures 20 items and reports packets with quotes; the Texture cites them. Count verified quotes against the bodies (free) and markers at word boundaries. Target ≥ 90% verified. |
| R2 | The publication substrate can serve a living document at one permalink | Every publish mints a route today, along with about a dozen objects (publication, version, proposal, manifest, source, span, retrieval manifest, consent, review, activity, attestation) | `service.go:203-217` | Read `PublishTextureToGraph` for what a version-append needs; measure the object count per publish. Decide which per-publish objects a revision needs. |
| R3 | The existing renderers make a readable news page outside the editor | They were built for the editor's window, popovers and contenteditable spans | renderers are pure functions (§2.2) | **Prototype A**: a standalone Svelte page that takes a `PublicationBundle` and renders it with `renderStructuredTextureDocHTML` and the source renderer. Judge it on phone width. |
| R4 | A front page can be built from one-Texture-per-story without duplicate floods | Management's clustering is untested. The owner welcomes several Textures per story, but readers need grouping. | none | Run a research Cast over one day of captures that returns story clusters; count duplicates by hand. Hand-author an edition Texture of pinned refs and render it as cards (tests the v1 layout). |
| R5 | News claims yield enough scoreable predictions to be worth showing | Most news sentences are attributions, not predictions. Showing empty or vacuous predictions damages trust. | the baseline has at least 3 dated forward items in about 35 | Hand-label the baseline report's items into attribution, scoreable inference and neither. Count how many resolve by count or date within 30 days. If few, ship observations (source track records) before edition predictions. |
| R6 | Cost and latency per story are tolerable | One Texture turn per story per evidence batch; a demo has 10 to 35 stories | the W2 acceptance measures turn latency; not yet run | Measure tokens and minutes per story revision in stage 1. |
| R7 | Quotes shown publicly are acceptable use | The paper republishes others' text to logged-out readers | none | Owner decision (open question 4): quote length cap, no full bodies. |
| R8 | P2d's per-request bound and its open run model do not stall living stories | four directives per owner request; P2d2 held | P2d receipts | No prototype. The early slices are ordered to avoid it. Track it as P5's residual. |

## 7. Discovered heresies and residuals

Discovered, not repaired (recorded here; each deserves its own problem
record before any fix):

- `universal_wire.go:168` shows two fabricated claim strings on every
  story as if they were claims.
- `ListUniversalWireStories` fails the whole list on one malformed route
  and makes several queries per route (violates design §0 constraint 4).
- `PublishTexture` mints a new route on every publish, against the
  design's "same permalink".
- The demo revision's citation markers land mid-phrase ("Google's[1]",
  "The[4]"). Not investigated.
- Neither management nor the Texture desk can read a capture, though W2
  and this design both need them to.

Residuals of this design: `texture:new` is not in any station file; the
records GET routes are not in P4's intent line; W4 is a new slice; the
`publication: auto` property and its owner switch are unspecified; the
trust list for capture writers is unspecified.

## 8. Open questions for the owner

1. **Front page:** start with a computed list (v0) and move to an edition
   Texture curated by its own desk (v1), or start with the edition
   Texture? Recommendation: v0 first. It needs no protocol station and
   leaves the layout question to evidence.
2. **First stories:** may the owner open the first story Textures with the
   prompt bar (truthful owner provenance), or should stories wait for
   management's `texture:new`? Recommendation: owner-opened first; never
   management through prompt-bar.
3. **Reorder W3:** land the reader half of W3 (W3a) before W2, since it
   reads only published Textures and does not depend on P2d2?
   Recommendation: yes.
4. **Quotes and copyright:** what maximum quote length, and how much
   context around a quote should a logged-out reader see? Recommendation:
   quotes and a short window around them, never full bodies, always a
   link to the original.
5. **URLs:** keep `/pub/texture/<slug>` for stories, with the front page
   at `/paper` (or at `/` for logged-out visitors)? And should a story
   open as a full page or in a Desktop window for logged-out readers?
   Recommendation: full page, same components as the Desktop app.
6. **Predictions in public:** show edition predictions to readers from
   day one, or keep them owner-only until a track record exists?
7. **Default scorers:** which scorers does a logged-out reader see by
   default? Only this edition's, or every scorer with a filter?
8. **Scores and wakes:** confirm that a score record alone never wakes a
   Texture, and that management adds a Note when a score should change
   the text.
9. **Desk ledger transparency:** is management's directive ledger
   (what it fed each story and what it predicted) public as part of an
   edition's transparency, or owner-only? Recommendation: owner-only for
   now, because it is working state (protocol §10).
10. **Name:** the Desktop app becomes "Autopaper" (id `autopaper`), and
    `universal-wire` is deleted rather than renamed. Confirm?
