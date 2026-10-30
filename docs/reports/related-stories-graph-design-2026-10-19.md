# Related stories on Autopaper, computed from the graph

Draft 2026-10-19. Design only: no code changed. Mutation class of this
document is green. The class of the work it describes is stated in §5.1
and §11.

- Owner approval, 2026-10-19: "Ideally we use the graph for this. If we
  need to do some changes to the graph that's fine. Shared urls may not be
  enough." And: "The proposed design for similar articles using graph
  sounds good."
- Builds on [the event layer at ingestion](event-layer-at-ingestion-design-2026-10-16.md)
  (called the event doc below) and on its parent,
  [the claims, events and entities ontology](claims-events-entities-ontology-2026-10-14.md)
  (CEE).
- Authority boundaries: [agent product doctrine](../agent-product-doctrine.md).
  Texture stays the sole writer of canonical revisions. This design does
  not touch Texture.

## 1. Summary

A reader who finishes a story on choir.news has nowhere to go next. This
design adds a "More on this" list below the story. The list holds other
published stories about the same subject.

The links come from the object graph, not from a text search. Each
published version gets an edge to every capture it cites. The paper
computer publishes its event layer to Store B, so captures also join
events there. A story's related stories are the other stories that cite
the same captures, or captures of the same event.

A separate cached endpoint serves the list after the article has
rendered. The slow resolve path does not change.

The work lands in two phases. Phase 1 is the story-to-capture edges, links
from shared captures, the endpoint and the UI. Phase 2 publishes events
and adds event-based links behind a flag, after a quality check on live
data.

## 2. Problem and intent

**The problem.** A published story is a dead end. The front list is the
only way to another story, and the front list is ranked by recency and
importance, not by subject.

**Why shared URLs are not enough.** The owner's own words are "shared urls
may not be enough". The evidence in §3.1 agrees. Two outlets that report
one event rarely cite the same URL. Two stories that do share a URL are
often joined by a hub page, such as a live blog or a newsletter, that has
nothing to do with their subject.

**Intent.** The list should hold stories a reader would call "the same
subject, or a development of it". It should be right more often than it is
long. An empty list is a valid answer, and a wrong link costs more than a
missing one.

**Non-goals.**
- No change to what Texture writes or when it writes it.
- No model call on the read path, and none at publish time.
- No personalised ranking and no reader tracking.
- No editorial "follow-up of" link in this design. §12 names it as a later
  option.

## 3. Current state, with evidence

### 3.1 What a shared-URL link gives today

A live probe on 2026-10-19 took the 120 stories on the front list and
compared the URLs each one cites.

- Only 10 of the 120 stories share a cited URL with another story.
- Those shares make 10 linked pairs.
- Some of the pairs are spurious. A DOJ story was linked to a Zelenskyy
  story, because both cite one live-blog or newsletter URL.

So the signal is both thin (about 8% of stories have any link) and noisy
(a hub URL links unrelated stories). Both findings shape the design.
Coverage needs a signal wider than one URL, which is the event. Precision
needs down-weighting of hubs, which is §6.2.

### 3.2 What the graph holds about a published story

Each fact below was read from the code at 44650e46e.

- **A version cites captures through anchors.** A version's manifest
  carries `Anchors []PublicationAnchor`
  (`internal/platform/publication_versions.go:87`, manifest field at
  `:244`). Each anchor has a `Ref`, and `Ref.Doc` is the cited capture's
  canonical id (`internal/types/ref.go:21`). Publish refuses an anchor
  whose `Ref.Doc` is not a readable capture: `captureBlockDoc`
  (`publication_versions.go:448`) requires the `choir.web_capture` kind,
  and `resolveAnchors` (`:480`) calls it for every anchor. So every
  anchor in a published version names a capture that exists in Store B.
- **No edge reaches a capture today.** `PublishTextureToGraph`
  (`internal/platform/publication_graph.go:31`) writes, from the version:
  `has_resolved_blocks`, `supersedes`, `has_policy`, `references_entity`
  (to a `choir.publication_source_entity`, line 496), `transcludes` (to a
  `choir.publication_transclusion`, line 502), `derived_from_proposal`,
  `has_manifest`, `contains_span`, `has_retrieval_source`,
  `has_retrieval_manifest`, and one edge per external citation
  (`:592`). The capture appears only inside the transclusion object's
  JSON metadata (`source_selector_json.doc`). The registered edge
  `transcludes_from` (`internal/objectgraph/registry.go:233`) has no
  writer. The registered edge `cites` (`registry.go:173`) is not written
  by the publish path either.
  Finding the stories that cite a capture therefore means scanning JSON,
  which is the access pattern that made article opens slow before
  2026-10-16 (see §3.5).
- **The reverse lookup is indexed.** `og_edges` has an index on
  `(to_id, kind)`, made by `MigrateRetrieval`
  (`internal/platform/retrieval_schema.go:200`), beside the older
  single-column indexes on `from_id` and `to_id`
  (`internal/platform/store.go:166-167`). "Which versions point at this
  capture?" is cheap once the edge exists.
- **Versions are append-only.** A version is written once and never
  rewritten (`publication_graph.go:127`). A revision adds a new version
  and a `supersedes` edge. The publication's `has_version` edges are the
  record of which versions exist (`:554`).
- **The write gate checks kinds.** `CheckWrite`
  (`internal/objectgraph/registry.go:295`) refuses an edge or object kind
  that `DefaultRegistry` does not register. A new kind must be
  registered with an owner, and the registry test holds every
  non-reserved kind to a writer.

### 3.3 What the event layer holds, and where

- **The layer is live on the paper.** Its log line reads
  `event layer: resolved N captures in K calls: J joined, T tentative, E
  new events, ...` (`internal/agentcore/event_layer.go:806`). On
  2026-10-19 the paper resolved about 10 captures per batch, and roughly
  half joined existing events.
- **The offline study backs that rate.** In the 500-capture study, 273
  events formed. 36 of them reached two or more outlets, and the
  orchestrator judged about 33 of the 36 to be one real occurrence (event
  doc §12). That read was not blind. The join threshold has since risen
  from 0.85 to 0.92 (`event_layer.go:60`) after false merges on staging.
- **Events live only on the computer.** `choir.world_event` and
  `choir.event_membership` are registered with owner
  `OwnerVMStore` (`registry.go:105-106`). `PutWorldEvent` and
  `PutEventMembership` (`internal/store/world_event.go:88`, `:121`) write
  them into the computer's own store. Nothing in `internal/platform`
  mentions either kind. The event doc chose this on purpose, as option
  (a) of its Q1, because Store B was in its Dolt soak.
- **A membership is an object, not an edge.** `EventMembership`
  (`world_event.go:59`) carries `CaptureID`, `EventID`, `Tentative`,
  `P`, `Outlet`, `Origin`, and the model and cost of the call. It is
  found by a metadata scan (`EventMembershipOf`, `:155`). No edge kind
  for it is registered. The event doc's table calls the membership
  "`reports`, capture to event", but the registry has no such kind.
- **Counting rules exist.** `CountedOrigin` (`world_event.go:232`) says a
  tentative member, or a capture of no outlet, adds nothing. A wire tag
  counts once however many outlets carry it. An event is corroborated at
  `EventMinOrigins = 2` distinct counted origins.
- **An event's identity is its opening capture.** `eventID`
  (`event_layer.go:273`) hashes the computer id and the opening capture
  id. The object's identity key is `computer \x00 event id`.
- **Not every capture is resolved.** The layer takes source-poll
  captures only. Research-read captures (a page a desk fetched) get no
  membership (event doc §14). A story that cites only research reads has
  no events.
- **No correction path exists yet.** Both puts are create-if-absent. A
  wrong join stays until the V1.5 merge work, which is not built.

### 3.4 The Dolt soak has passed

The event doc deferred Store B event writes because they would "confound
the soak" (§4, Q1). That reason is gone. The Dolt 2.4.2 soak ended on
2026-10-17 at 15:46 with 5,606 liveness probes over 48 hours and no
strikes (`docs/archive/reports/handoff-2026-10-15.md`, the "Dolt soak passed"
row). Store B retrieval landed the same day. Store B write routes are no
longer blocked by the soak. They are still red work, with the ceremony
in §11.

### 3.5 The read path

- **Resolve is the slow path, and it is already tuned.**
  `GetPublicationBundleByRoute`
  (`internal/platform/service_publication_read.go:134`) builds the whole
  bundle: manifest, blob, resolved blocks, spans, citation edges, policy,
  source entities, transclusions, provenance and the check overlay. Eight
  front stories resolved in 0.16 to 0.26 s after the 2026-10-16 fixes,
  against 1.1 to 1.3 s before (handoff row; problem doc
  `docs/archive/problems/article-open-slow-store-b-scans-2026-10-16.md`). The
  reader polls a lighter nav view (`GetPublicationNavByRoute`, `:121`).
  Related stories must not add to either.
- **`graphCitationEdges` treats unknown kinds as citations.**
  It lists every edge from the version and skips only a fixed set of
  structural kinds (`service_publication_read.go:397-415`). Any other
  kind becomes a `CitationEdge` with `ToKind: "external_reference"`.
  Two kinds already leak this way: `derived_from_proposal` and
  `has_retrieval_source` are not in the skip list. So a new version-to-
  capture edge will not break the function. It will add one spurious
  citation per cited capture to every bundle. §5.2 handles that.
- **The public proxy routes.** The proxy serves the resolve and front
  routes to anyone: `HandlePlatformPublicationResolve`
  (`internal/proxy/platform_public.go:40`) and
  `HandlePlatformFrontStories` (`:88`), registered at
  `internal/proxy/handlers.go:925-933`. The front route reads Store B
  alone and calls no computer, so it works while the paper computer is
  stopped. The related route should keep that property.
- **Sections exist only for front-list stories.** A story's section
  (world, us_politics, business, tech_ai) comes from the host scores file
  that `cmd/front-importance` writes, keyed by publication id
  (`internal/platform/publication_front_importance.go:41`, `:50`). Corpusd
  ignores a file older than 24 hours (`:22`). The scorer keeps a row for
  14 days after the story leaves its window
  (`internal/frontimportance/scorer.go:40`). A story that was never
  scored has no section, so a section can break a tie and nothing more.
- **Front eligibility.** `isAutopaperStory` and `frontStory`
  (`internal/platform/publication_front.go:149`, `:403`) decide that a
  publication is a public Autopaper story: published state, a route, an
  allowed owner, mode `auto`, and a public access policy on the latest
  manifest. A related link must pass the same test. A held story is not
  published at all, so it has no publication to link.

### 3.6 The frontend

The published reader is one `<article data-texture-published-reader>`
(`frontend/src/lib/TextureEditor.svelte:1974`). Three facts matter.

- The article has no Svelte children. Its HTML is written imperatively by
  `syncEditorSurface` (`:231`).
- The article is the scroll container. `.rendered-doc` is `overflow: auto`
  and `flex: 1 1 auto` (`:2125`), inside `.document-body`, which is
  `overflow: hidden` (`:2054`), inside a `height: 100%` editor. The page
  itself never scrolls.
- The standalone Autopaper route mounts this editor
  (`frontend/src/lib/AutopaperRoute.svelte:101`) and listens for
  context changes to move between stories.

## 4. The approved design, checked against the code

The task brief asked that every claim be verified. Two claims differ from
the code, and several need a qualification.

| Claim | Result |
|---|---|
| Each version's manifest lists cited captures through `Anchors[].Ref.Doc` | **Confirmed.** `Ref.Doc` is a capture id (§3.2). `SourceEntityID` names a source entity row, not the capture, and the walk does not need it |
| A new edge kind does not break `graphCitationEdges` | **Partly.** It does not error, but it leaks into every bundle as a citation (§3.5). The skip list must grow by the new kind |
| Edge kinds are registered in `registry.go` | **Confirmed.** `CheckWrite` refuses an unregistered kind (§3.2) |
| The event layer is `OwnerVMStore` only | **Confirmed** (`registry.go:105-106`) |
| Publishing it is design "Q1b" in the event doc | **Differs in name.** The event doc has "Q1" with options (a) and (b). Option (b) is Store B through a typed corpusd route. No "Q1b" appears in it |
| `choir.world_event` objects and capture-to-event membership **edges** exist on the paper | **Differs.** The event objects exist. Memberships are `choir.event_membership` **objects**, and no membership edge kind is registered, though the event doc's table names one (`reports`). Phase 2 must add the edge as a new kind (§5.3) |
| `types.CaptureEvent` is part of the event layer | **Confirmed**, but it is a wake record, not stored state (`internal/types/story.go:75`) |
| The soak that gated Store B write routes passed on 2026-10-17 15:46 | **Confirmed** (§3.4) |
| `proj_capture_index` has `url_key` and `canonical_key` | **Confirmed** (`internal/platform/capture_index.go:62-64`), with indexes on `(owner, computer, url_key, fetched_at)` and `(owner, computer, canonical_key, fetched_at)` |
| Sections come from `frontimportance` scores, front list only | **Confirmed**, with the 24-hour freshness rule and 14-day retention (§3.5) |
| Resolve is the slow path and lives in `GetPublicationBundleByRoute` | **Confirmed** (§3.5). It is 0.16 to 0.26 s since the 2026-10-16 fix, slow compared with a cached read, not slow in absolute terms |
| The proxy route belongs in `platform_public.go` | **Confirmed**, with a second edit in `handlers.go` for the path switch |
| The published reader sits near `<article data-texture-published-reader>` | **Confirmed**, with the scroll-container constraint in §3.6 |

## 5. The design

### 5.1 Data model and mutation class

Two new edge kinds, one new write on the publish path, and (phase 2) two
existing object kinds published to a second store.

| Item | Kind | From to | Store | Writer | Class |
|---|---|---|---|---|---|
| Story cites capture | edge `cites_capture` (new) | publication version to `choir.web_capture` | B | corpusd, inside `PublishTextureToGraph` | red |
| Capture reports event | edge `reports_event` (new) | `choir.web_capture` to `choir.world_event` | B | corpusd, from the guest's typed export route | red |
| Event | object `choir.world_event` (existing kind) | none | B (exported copy) | same route | red |
| Membership | object `choir.event_membership` (existing kind) | none | B (exported copy) | same route | red |
| Related walk | none; a read | none | B | corpusd read | orange |
| Skip-list entry in `graphCitationEdges` | code | none | none | corpusd | orange |
| UI block | code | none | none | frontend | orange |

**Why red.** The owner's rule is that a Store B schema or write route is
red. `cites_capture` changes what the publication write path stores, and
publication is a protected surface. The event export is a new corpusd
write route. Phase 1 therefore needs the full ceremony of CLAUDE.md. §11
gives it.

**Names.** `cites_capture` states the relation and is distinct from the
registered but unused `cites`. `reports_event` is more specific than the
event doc's `reports`, which collides with engineering "reports" in
speech. Both names are a decision I took (§12) and are cheap to change
before any code lands.

### 5.2 Phase 1 write: story to capture edges at publish

**What is written.** For each distinct `Ref.Doc` among the version's
anchors, one edge `cites_capture` from the version's canonical id to the
capture's canonical id. A capture cited by several anchors gets one edge.
Embeds and cites both count. The edge metadata is deliberately small and
deterministic: `{"relation":"cites"}`. Edge ids derive from their
metadata (`makeEdge`, `publication_graph.go:718`), so a replayed publish
writes the same edge ids and nothing new.

**Where.** In `PublishTextureToGraph`, in the same `PutBatch` as the rest
of the version. The anchors are already in `PublishGraphParams` (the
service passes `anchors` at `internal/platform/service.go:414` onward).
The edge is part of the version's atomic batch, so a version exists with
all its capture edges or not at all. No second write, no partial state.

**Reached from.** The paper's story publish enters through
`HandleGuestTexturePublication` and `publishTextureRevision`
(`internal/proxy/platform_publish.go:150`, `:179`), then
`postPlatformPublication` (`:740`), then `Service.PublishTexture`
(`internal/platform/service.go:180`). The owner-published path uses the
same service call, so it gets the edges too. Non-Autopaper publications
write edges that nothing reads.

**The citation leak.** Add `cites_capture` to the skip list in
`graphCitationEdges` (`service_publication_read.go:404`). Without it,
every bundle gains one `CitationEdge` per cited capture. The leak costs
payload and confuses the meaning of `Citations`. A grep of `frontend/src` finds no read of the bundle's `citations`
array (the source panel reads the anchors and the check overlay), so the
leak would not break the UI, only bloat it. Confirm that in review. The skip-list entry is still right, and cheap.

**Backfill.** Versions published before this change have no edges. The
manifest of each is immutable and holds the anchors, so the edges are a
pure function of stored data. A one-off, owner-triggered act walks the
latest version of every Autopaper story and writes the edges. It uses the
same edge ids, so running it twice writes nothing new. It is an explicit
act, reachable through an internal route and the `choir` CLI. It is not a
boot-time sweep, which CLAUDE.md forbids ("do not add boot-time resumers
that run without a planned-restart marker"). Only the latest version of
each story needs it. Older versions are never read by the walk (§5.5).

### 5.3 Phase 2 write: publish the event layer to Store B

This is option (b) of the event doc's Q1, now unblocked.

**Authority.** The computer's own `choir.event_membership` and
`choir.world_event` stay the authority. The Store B rows are a one-way
export, the same shape as `choir.support_verdict` (authority on the
computer) and `choir.support_judgement` (export in Store B). CEE §5.4
already says the paper's judgments are public by nature and a private
computer's are not. This answers standing question 5 (single state
authority): the tape wins, and the export never flows back.

**Route.** A typed corpusd route, built on the precedent of
`HandleSupportVerdicts` (`internal/platform/publication_checks.go:171`)
and the capture route (`captures.go:164`). Both accept a write only from
a host-internal caller or from the guest that vmctl binds to exactly the
owner and computer the request names (`requireInternalOrBoundGuestWrite`,
`internal/platform/authority.go:76`). The new route takes a batch of
events, memberships and the edges, validated and written in one
`PutBatch`. A private computer's export is refused, by a platform-tier
check on the caller. It is not left to the client to choose.

**Identity.** The export uses the same identity keys as the computer
(`computer \x00 event id` and `computer \x00 capture id`), so a retry is
a no-op, and a re-export after a crash cannot mint a second event
(failure mode R9). The registry holds one owner string per kind
(`registry.go:26`). The two kinds are now written on two sides, so their
owner text needs a decision, recorded in §12 (D2).

**What leaves the computer.** The event: id, headline, opening capture,
opening outlet, first seen. The membership: capture id, event id,
tentative flag, p, outlet, origin, resolved time, and the model name. The
membership's title, URL, token counts and cost stay home. Title and URL
are already on the capture object, and the cost is the computer's own
accounting.

**The edge.** For each membership the route also writes
`reports_event` from the capture to the event, with metadata
`{"tentative":bool,"origin":"<counted origin or empty>","p":0.93}`. The
object holds the evidence. The edge is the index. The objects are found
by metadata scan, which is the pattern §3.2 warns about. The edge is
found by `(to_id, kind)`, which is indexed. Without the edge the walk
would reintroduce the slow scan.

**When.** After `writeMembership` succeeds
(`event_layer.go:546`), the flush batches that flush's new events and
memberships (at most 12 captures per chunk) into one export call. A
failure is logged and counted. It is not retried at boot. A gap is a
visible state with a counter, and the walk tolerates it (it falls back to
shared captures, §5.5). The owner can ask for a re-export through the
same internal route. That follows the event doc's restart rule: no
durable obligation, no boot sweep.

**A computer setting.** `intake.events_publish`, off by default, next to
`intake.events` (`world_event.go:32`). Turning it off is the first
rollback step.

**Growth.** From the 500-capture study (event doc §12): about one
membership and about half an event per capture, and about 125 captures an
hour. That is roughly 3,000 memberships, 1,500 events and 3,000 edges a
day, or 7,500 small rows. Corpus Dolt stood at 2.2 of 5.4 GB on
2026-10-17 (handoff row). The estimate is mine, from a four-hour sample,
so the check in §10 should read the real row counts after a week.

### 5.4 What is not added

- No projection table in phase 1. The walk uses indexed edge reads. If
  the measurements in §6.5 miss their budget, add a rebuildable
  `proj_story_capture` table in `dolt_ignore`, like `proj_capture_index`
  (`capture_index.go`). The edges stay the authority and the table stays
  a projection that is never authoritative.
- No new object kind for "related". The list is computed, not stored.
  Nothing stored can go stale (§7).

### 5.5 The query

Input: a story (its bare route). Output: up to five other stories.

**Walk.**

1. Resolve the route to the publication and its **latest** version.
   Reuse the route lookup and `publishedVersions` that resolve already
   uses (`resolvePublicationHead`, `service_publication_read.go:43`), and
   stop before the blob and block reads.
2. Read the version's `cites_capture` edges. These are the story's cited
   captures. Index: `from_id`.
3. **Shared captures (phase 1).** For each cited capture, read the
   `cites_capture` edges that point at it. Index: `(to_id, kind)`. Each
   hit is a version of some story.
4. **Shared sources by key (phase 1).** Look up each cited capture in
   `proj_capture_index` by capture id and read its `canonical_key`. Find
   the other captures with the same key under the same owner and computer
   (`idx_pci_canonical`, `capture_index.go:77`). Repeat step 3 for them.
   This catches a recapture of one URL, which the paper does often: the
   handoff records three Deutsche Welle recaptures that differ only by
   copy edits.
5. **Shared events (phase 2).** For each cited capture, read its
   `reports_event` edges (index: `from_id`), keep counted members of
   corroborated events, then read each event's member captures (index:
   `(to_id, kind)`), then repeat step 3 for those captures.
6. Map each hit version to its publication through the `has_version` edge
   that points at it (index: `(to_id, kind)`). Drop the version unless it
   is that publication's **latest**. Drop the publication if it is the
   story itself. Drop it unless `frontStory` accepts it (§3.5).

**Scoring.** For a candidate story, the score is the sum over its shared
signals, multiplied by a recency factor.

- An **event signal** is worth `idf(e) * min(origins(e), 4)`. `origins(e)`
  is the event's count of distinct counted origins. A better-corroborated
  event is stronger evidence that the two stories are about one subject,
  and the cap of 4 stops a huge event from drowning the rest.
- A **capture signal** is worth `idf(c)`, and a canonical-key signal is
  worth the same. Count one signal per distinct key, not per capture, so
  three recaptures of one page are one signal.
- `idf(x) = ln(1 + N / (1 + df(x)))`. `N` is the number of current
  Autopaper stories. `df(x)` is the number of those stories that cite
  the capture or key `x`, or any capture of the event `x`. A rarely
  cited source scores high. A hub cited by many stories scores near zero.
- **Hub cutoff.** A capture or key cited by more than 6 stories, and an
  event with more than 40 member captures, contribute nothing. The IDF
  weight alone would still let a hub link everything weakly. A hard
  cutoff gives hubs no vote. §6.2 explains why.
- **Recency.** Multiply by `0.5 ^ (age_days / 7)`, where age counts from
  the candidate's latest `published_at`, with a floor of 0.2. Old stories
  stay reachable but rank below fresh ones.
- **Same section** adds a small tiebreak (a factor of 1.1) when both
  stories have a fresh section from the scores file. It never creates a
  link and never ranks a story above one with a stronger signal.
- **Threshold.** Show a candidate only at a score of at least 1.0, which
  is about one rare shared source (a source that two of a hundred stories
  share has idf near 3.4). Return fewer than five, or none, rather than
  fill the list.
- Final tiebreak: newer first, then publication id.

The weights, the cap of 4, the hub cutoffs, the half-life and the
threshold are starting values. I have not tested them. They are
hypotheses to calibrate in the phase 1 and phase 2 checks (§8, §10), and
they live in one table of constants so a calibration is a one-line
change.

**Cost.** For a story with about 10 cited captures, phase 1 is roughly
one edge read for the version, 10 reverse edge reads, 10 index lookups,
and a short second round for canonical keys. The event walk adds about 10
forward reads and a few reads per event. The total is on the order of 100
indexed reads, a few tens of milliseconds warm. That number is an
estimate, not a measurement. §8 makes the measurement a ship criterion,
and §6.5 sets a per-request budget so a slow walk cannot hurt the page.

### 5.6 Caching and invalidation

The endpoint is cheap but not free, and the answer changes slowly. So it
is cached, and the cache is allowed to be a few minutes stale.

- **Key.** Publication id, latest revision, and the flags that change the
  basis (events on or off).
- **Store.** A small in-process map in corpusd with a TTL of 5 minutes and
  a cap of about 500 entries. A corpusd restart empties it, and the next
  request refills it. The cache holds a derived answer only, so losing it
  loses nothing (standing question 8).
- **Revision of the story itself.** A new version changes the key, so the
  old entry is simply never read again.
- **New stories elsewhere.** A new story that should appear in this
  story's list appears when the TTL expires. A five-minute lag is
  acceptable for a "read more" list. A publish does not purge other
  stories' entries, because that would couple the publish path to the read
  cache. That is a cost I chose to avoid.
- **Do not cache failures or timeouts.** A request that exceeds its budget
  returns an empty list with `partial: true` and is not stored.
- **HTTP.** The proxy sets `Cache-Control: public, max-age=300` on a
  success, and `no-store` on an empty `partial` answer. A browser or an
  intermediate cache then also absorbs repeat opens.

### 5.7 API

One public read route, served from Store B alone. Like the front list, it
calls no computer, so it works while the paper computer is stopped.

```text
GET /api/platform/publications/related?route=/pub/texture/<slug>&limit=5

200
{
  "route": "/pub/texture/<slug>",
  "revision": 7,
  "basis": { "captures": 9, "events": 4, "mode": "captures+events" },
  "partial": false,
  "generated_at": "2026-10-19T21:30:00Z",
  "links": [
    {
      "publication_id": "...",
      "title": "...",
      "dek": "...",
      "route": { "bare": "/pub/texture/<other>", "permalink": "/pub/texture/<other>/v3" },
      "updated_at": "...",
      "section": "world",
      "reason": "event",
      "shared": { "events": 1, "sources": 2 }
    }
  ]
}
```

- `links[]` reuses the `FrontStory` row (`publication_front.go`), so the
  title, dek and routes match what the front list shows, and the reader
  needs one story renderer. `reason` and `shared` are added.
- `basis` lets a test and an operator see why a list is empty without
  SSH: how many captures the story cites, how many events they reach, and
  which signals were on (standing question 9).
- An unknown route, a held or unpublished story, or a non-Autopaper
  publication returns `200` with `links: []`, the same body shape. The
  reader treats "nothing to show" and "nothing known" alike, and the
  route leaks no existence signal that resolve does not.
- The route is `GET` only, public, read-only, and takes no body.

Wiring, one edit per layer:

- corpusd: `GET /internal/platform/publications/related`, registered next
  to `publications/front` (`internal/platform/handlers.go:522`), with the
  walk in a new file under `internal/platform/`.
- proxy: a `HandlePlatformPublicationRelated` beside the front handler
  (`internal/proxy/platform_public.go:88`) and a `case` beside
  `handlers.go:931`. The lifecycle metric is `platform_related.list`.

### 5.8 The UI

A heading "More on this" and up to five links, below the last paragraph
of the story.

**Placement.** The article is the scroller and has no Svelte children
(§3.6). A sibling after the `<article>` would sit outside the scroller and
take height from the reader at all times, on a phone too. So the block
goes **inside** the article's HTML. The published reader appends
`renderRelatedHTML(related)` to the string it passes to `syncEditorSurface`
and re-syncs when the list arrives. Click handling reuses the delegated
`handleEditorClick` on the article.

**Timing.** The fetch starts after the article has rendered, from the end
of `loadPublishedContext` (`TextureEditor.svelte:983`), and is not
awaited. It never blocks the resolve or the first paint. A response is
dropped if the reader has moved to another route in the meantime. A
failure shows nothing.

**Markup.** Each link is a real `<a href="/autopaper/<slug>">`, so a
reader can open it in a new tab. A plain click is intercepted and moves
the reader through the same context change the front list uses
(`AutopaperRoute.svelte:101`). Each link shows the title, the dek cut to
two lines, the relative time and, when known, the section. The block
carries `data-texture-related`, and each link carries
`data-related-link` and `data-related-reason`, for the acceptance spec.

**Which pages.** The bare route and a `/v<n>` permalink both show the
story's current list, computed from its latest version. A reader on an
old version still sees where the story stands now. If the list is empty,
render nothing, with no heading and no placeholder.

**Phone.** The spec must hold at 375 px: no horizontal overflow, and the
block reachable by scrolling the article. `autopaper-reader-deployed.spec.js`
already checks both for the reader (`expectNoHorizontalScroll`).

## 6. Ways it could fail

Written before any code. Tests are written against this list (CLAUDE.md:
failure modes first, then code, E2E preferred).

**R1 Wrong event joins produce wrong links.**
A false merge in the event layer now shows up in a reader's list. The
event doc's own history has this: at p from 0.85 to 0.92 every join was a
topic merge. Mitigations: only counted members of corroborated events
count (a tentative member never links); the §10 check gates shipping; a
per-flag kill switch; and a Store B-side suppression for one bad event
(tombstone its `reports_event` edges, which `CheckWrite` lets pass
(`registry.go:295`)). Residual: the computer has no correction path
today (§3.3), so a wrong join persists on the tape until the V1.5 merge
exists.

**R2 Hub captures link everything.**
A live blog or newsletter cited by many stories joins unrelated ones (the
DOJ and Zelenskyy pair). Mitigation: IDF weighting plus the hard cutoff
of 6 citing stories (§5.5). A test seeds one hub capture cited by 8
stories and asserts it produces no link on its own.

**R3 Stale links after a story is revised.**
A story's later version may drop a source. Edges from old versions would
keep it linked. Mitigation: the walk reads only the **latest** version's
edges for the story and ignores older versions' edges of every
candidate (§5.5 step 6). Versions are immutable, so no edge is ever
rewritten or tombstoned for this. The cache key includes the revision,
and the TTL bounds a candidate's staleness to 5 minutes.

**R4 Links to held, unpublished or private stories.**
A candidate must pass `frontStory`: published state, a route, an allowed
owner, mode `auto`, and a public access policy (`publication_front.go:403`).
A held story has no publication. A story made private later fails the
access-policy test on its latest manifest. The cache TTL bounds how long
a withdrawn story can linger in someone else's list. A test publishes a
story with a non-public policy and asserts it never appears.

**R5 Cost on the read path.**
The walk adds Store B reads. Mitigations: a separate endpoint after the
article renders; a cache; a 400 ms budget per request (an overrun returns
an empty `partial` list); indexed reads only; and a cap on fan-out (at
most 12 cited captures, 40 events and 200 candidate versions per
request). Resolve does not change. Success is measured, not assumed
(§8, ship criterion).

**R6 Cold start for stories with no events.**
Phase 1 stories have no event data. Stories that cite only research-read
captures never will (§3.3). Stories published before the event layer
went live never will either, because the layer does not resolve old
captures. Mitigation: the capture and canonical-key signals work with no
events. `basis.events: 0` tells a test and an operator why. Expect a
shorter list for these stories, and an empty one for some.

**R7 Privacy and owner scope of events.**
A private computer's events must not enter Store B, and a related list
must not name a private story. Mitigations: the export route refuses a
non-platform-tier computer; the walk starts and ends in Autopaper
stories (`isAutopaperStory`); captures are matched under one owner and
computer (the capture index keys include both); and the response carries
titles and routes of public stories only, never an event headline or a
member capture's text.

**R8 A story linking to itself or its own versions.**
The walk drops candidates with the same publication id, which covers
every version of the same story. A story published twice under different
publication ids from the same revision would still link to its twin;
that case cannot arise from one source document, since identity is
`(owner, source doc)` (`publicationIdentity`,
`publication_versions.go:269`). A test publishes v1 and v2 and asserts
neither version appears in the other's list.

**R9 Duplicate or conflicting exports.**
A retried export, or an export that races a new membership, writes the
same ids and is a no-op. A membership the computer writes once and never
changes cannot conflict with its copy. The test replays one export batch
twice and counts rows.

**R10 An export for a capture Store B does not have.**
A membership could point at a capture whose Store B write failed (the
event doc's E8). The export route validates that each capture exists in
Store B and drops the edge, with a count, when it does not.

**R11 Related links change the cited-source counts.**
The publish gate counts outlets of cited captures
(event doc §13, tentative-member residual). The new edges must not feed
the gate or the front depth numbers. The edges are read only by the
related walk. A test publishes a story and asserts its `source_count` is
the same with and without the edges.

**R12 A new kind breaks a replay or a vocabulary migration.**
Replay and `vocabmigrate` do not call `CheckWrite`, by design
(`registry.go:279-293`). A new edge kind in a replayed event must
round-trip. The registry test and a replay test cover it.

**R13 A publish fails because the edge write fails.**
The edges are in the version's own batch, so a failure aborts the whole
publish. That is a worse outcome than a missing related list. Mitigation:
build the edge list from anchors that publish already resolved and
checked, so there is no new input to fail on. An edge to a capture that
resolve just read cannot dangle. If the batch grows near a limit, split
the edges into a second `PutBatch` that logs and counts a failure, like
`writeVersionChecks` (`service.go:567`), and let the backfill repair a
miss. That is the fallback. The default is the atomic batch.

**R14 The list shows near-duplicates.**
Two stories on one event from one desk are both valid. But the list
should not hold five variants of one story. Mitigation: after scoring,
drop a candidate whose cited captures are a superset or near-equal (over
80% overlap) of an already chosen candidate's, unless it is much newer.
This is a refinement; the first release may skip it and measure how often
it happens.

**R15 Sections make a topic bubble.**
The same-section tiebreak could push same-section stories over better
cross-section ones. It is a tiebreak only, and a 10% factor, so it cannot
overturn a stronger signal. The check reads how often it changes the
order.

**R16 Event-based links for a story whose own capture was never
corroborated.**
A story cites capture A. A has an event with one outlet. No other story
shares it. The walk finds nothing. That is correct, not a failure: the
list is empty and the page shows nothing.

## 7. Staleness, summed up

The design stores no list, so nothing stored goes stale. What can be
stale is the cached answer (at most 5 minutes) and the per-story edges
(never changed, so never wrong about a version, only about what is
current, which step 6 of the walk handles). A withdrawn or revised story
leaves other stories' lists within one TTL.

## 8. Phased plan

### Phase 1: story to capture edges, capture-based links, endpoint, UI

Red. Lands without events.

1. Document the problem first (CLAUDE.md "Problem documentation first"):
   a short `docs/problems/` note carrying §3.1's evidence. This is its own
   commit, before the code.
2. Register `cites_capture` (owner `OwnerCorpusd`); add it to the
   `graphCitationEdges` skip list; add the edge writes to
   `PublishTextureToGraph`.
3. Write the backfill act and run it on staging. Verify that the count of
   `cites_capture` edges per latest version equals the count of distinct
   `Ref.Doc` in its manifest.
4. Build the walk with capture and canonical-key signals only, the cache,
   the corpusd and proxy routes, and the UI block.
5. Landing loop: commit, push `origin/main`, watch CI, watch the staging
   deploy, confirm the deployed commit, run §9.

**Ship criteria for phase 1** (all on staging, against the live front list):
- Coverage: the share of front stories with at least one link, against
  the 10 of 120 shared-URL baseline. I expect some rise from canonical
  keys, but the main gain is phase 2. Record the number. Do not set a bar
  on it, because the point of phase 1 is precision.
- Precision: read every link on a sample of at least 40 linked pairs and
  confirm the DOJ and Zelenskyy hub pair is not among them. Wrong or
  loosely related pairs at or under 10%.
- Latency: p95 of the related endpoint under 150 ms warm and under 400 ms
  cold, over at least 100 requests. Resolve p95 unchanged. A comparison to
  the resolve timings in the step log (`resolveSteps`).
- Zero errors in the corpusd log from the new route and from publishing.

### Phase 2: event publication and event-based links, behind a flag

Red. Starts only after phase 1 is stable and the event-quality check in
§10 has met its threshold.

1. Register `reports_event`, and decide the kind-owner text (D2). Add the
   typed export route, with the platform-tier check and the capture
   existence check.
2. Add the guest-side export and the `intake.events_publish` setting.
   Turn it on for a minted test computer first, then the paper.
3. Let it run for at least 48 hours to fill events. Run the check in §10
   on the Store B copy, not only on the computer's.
4. Add event signals to the walk behind `CORPUSD_RELATED_EVENTS`
   (default off). Turn it on for the endpoint only after the check passes.
5. Landing loop as in phase 1, plus the §9 event assertions.

## 9. Acceptance proof on staging

Staging is `https://choir.news`. The proof is a deployed Playwright spec,
added beside the existing one:
`frontend/tests/autopaper-related-deployed.spec.js`, modelled on
`autopaper-reader-deployed.spec.js` (it runs at 375 by 812, without
signing in, and reads `/api/platform/publications/front`).

**API assertions.**
1. Take the front list (`limit=60`). Call the related route for each
   story. Assert every response is 200 with the documented shape.
2. At least one story has a non-empty list. If none does, fail with the
   `basis` fields of the first five stories in the message, so the cause
   is readable.
3. For every link: its route resolves with 200; it appears in the front
   list; it is not the story itself or any of its versions; and no
   publication appears twice in one list.
4. For a story that cites sources, `basis.captures` is greater than zero.
   That is the publish-path proof: the edges exist.
5. Hub test: no link in any list is justified by a single capture cited
   by more than 6 stories. The spec reads the `shared` counts and the
   front list to check it.
6. Phase 2 only: at least one link has `reason: "event"`, and no link has
   `reason: "event"` when `basis.events` is 0.

**Browser assertions.** Open a story that has a non-empty list.
1. Wait for `[data-texture-published-reader]`. Record the time of its
   first paint.
2. Assert the related request started after the reader was attached, and
   that the resolve request finished before it. (Use `page.on('request')`
   timestamps.) This proves the list is fetched after the article, not
   inside resolve.
3. Scroll the article to its end (`[data-texture-editor-area]`, the
   scroller). Assert `[data-texture-related]` is visible below the last
   paragraph, shows the heading "More on this", and has between 1 and 5
   links.
4. Assert no horizontal overflow (`expectNoHorizontalScroll`).
5. Click the first link. Assert the URL becomes `/autopaper/<slug>`, the
   reader's `data-publication-id` changes, and the new page renders.
6. Open a story with an empty list. Assert no `[data-texture-related]`
   element and no empty heading.

**The artifact.** The spec writes the linked pairs it saw, with their
reasons and `basis`, as a JSON attachment. The run is then verifiable and
repeatable (CLAUDE.md, Testing). The same JSON feeds the human reading in
§10.

**Publish-path proof for a fresh story.** After the next real story
publishes on staging, call the related route for it and assert
`basis.captures` equals its distinct cited captures. Do not publish a
synthetic story into the live paper to get this.

**Final evidence** (CLAUDE.md "Final evidence") names: the pushed SHA, CI
run, staging build identity, the spec command and result, the artifact,
the backfill count, the latency numbers, the mutation class and protected
surfaces, the heresy delta, and the next realism axis.

## 10. The event-quality check

Run before any reader sees an event-based link. It is the gate for phase
2, step 4.

**Why.** The event layer's own precision was measured on titles and
summaries, as a topic-or-event join, by a non-blind reader. Related
stories depend on it in a new way. A wrong join in the event doc cost a
false corroboration. Here it costs a wrong link on a public page.

**Setup.**
1. Let the Store B export run for at least 48 hours.
2. Take every event in Store B that is corroborated (at least two counted
   origins) and has a member capture that a current story cites.
3. Build the candidate story pairs the walk would link through events
   alone: story A and story B that cite different captures of one event.
   Exclude pairs that already share a capture, since phase 1 links them.
4. Draw at least 100 such pairs at random. If there are fewer than 100,
   take them all and say so.

**Judging.** Show each pair as the two story titles and deks, with the
event headline hidden. Two judges rate it: the registered judge the event
layer already uses, and a human read of a subsample of 30. Each rating is
one of:
- **same event or direct development** (correct);
- **same topic, different event** (a topic link; not wrong enough to
  hide, but counted separately);
- **unrelated** (a false join).

**Measures.**
- False-join rate: unrelated, over all pairs.
- Topic-link rate: same topic, different event.
- Coverage lift: the share of front stories with at least one event-based
  link that they would not have without events, against the phase 1
  coverage.
- Judge agreement on the human subsample.
- Join-origin: how many of the false joins sit at p between 0.92 and 0.95.
  If most do, raise the join threshold for publication rather than for
  resolution.

**Threshold to ship.** Event-based links go on when all hold:
- the false-join rate is at most 5%, which is the event doc's own V1 bar
  (event doc §9, slice V1), and the 95% upper bound is at most 10%;
- the topic-link rate is at most 20%;
- the coverage lift is at least 15 percentage points, or the feature is
  not paying for its risk;
- no single event accounts for more than 10% of the pairs.

**If it fails.** Do not tune the prompt or the weights blind (CLAUDE.md,
"understand before tuning"). Read the false joins in the trace first,
using the memberships' `p`, alternatives and model fields. Then choose
between publishing only memberships above a higher p, requiring three
origins for a link, or leaving events off and keeping phase 1. Phase 1
stands on its own either way.

## 11. Ceremony

**Mutation class.** Phase 1: red (publication write path; a Store B edge
kind). Phase 2: red (a new corpusd write route; a Store B write kind;
guest-to-platform data flow). The read endpoint, the skip-list change and
the UI are orange within them.

**Protected surfaces.** Texture's canonical publication path (reads its
output, does not write Texture), Store B and its write gate, corpusd's
publication and capture routes, and the proxy's public routes. Vmctl,
auth and renewal, the gateway and provider routing are not touched.

**Conjecture delta.** Stories that cite the same or recaptured sources,
and stories whose sources share a corroborated event, are a better
"related" set than shared URLs, at a precision a reader accepts. It is
falsified for events if §10 fails, and for captures if the phase 1
precision read exceeds 10% wrong.

**Admissible evidence.** The deployed spec and its artifact on
`choir.news`; the corpusd step log for latency; the §10 read. Local runs
shape the ranking function and the UI only. They do not prove anything
about Store B, vmctl, or the live paper.

**Rollback.**
- Phase 1: set `CORPUSD_RELATED=off`. The proxy route then returns
  `links: []`, and the UI renders nothing. The edges are additive and
  harmless to leave, and no code reads them but this feature. If a publish
  failure traces to the edge write, revert the commit. Older code reads the
  unknown kind as one more citation (the leak in §3.5), which is harmless,
  so a rollback needs no data migration.
- Phase 2: set `CORPUSD_RELATED_EVENTS=off` for the read. Turn off
  `intake.events_publish` for the export, per computer. The Store B event
  rows can stay. If wrong joins must be removed, tombstone the
  `reports_event` edges of the named events (`CheckWrite` passes
  tombstones). Then revert.
- The product restore path is not involved. Related lists are derived
  and not part of any computer's event chain.

**Heresy delta.**
- Discovered: the event doc names a `reports` membership edge that was
  never registered, while memberships are objects. `graphCitationEdges`
  already leaks `derived_from_proposal` and `has_retrieval_source` as
  citations. No edge reaches a capture, so every capture-reverse lookup
  would be a JSON scan.
- Introduced: none. The export is a one-way copy with the tape as
  authority. The skip-list entry is a symptom fix in a hard-coded list;
  moving it to a registry flag would be the substrate fix, and §12 names
  it.
- Repaired: none until the edges land. Then the dead `transcludes_from`
  and `cites` registrations remain as unused kinds; this design does not
  touch them.

**Standing questions** ([docs/standing-questions.md](../standing-questions.md)).
- Decision provenance: the owner's two quotes at the top, 2026-10-19.
- Settled-decision conformance: shared content lives in Store B behind
  corpusd, computers hold overlays (standing question 2). The export is
  that rule applied to events.
- Deletion citers: nothing is deleted.
- Single state authority: the tape for memberships; the graph for edges.
- Artifact-verified success: §9's artifact.
- Fate-sharing: the read fate-shares with corpusd and Store B only, not
  with the paper computer.
- Restart durability: the cache is memory-only and rebuilt; the export has
  no boot sweep.
- No-SSH operability: `basis` in the response and the internal
  re-export and backfill routes.

## 12. Decisions taken, and what remains open

Taken under "No blocking asks":

- **D1 Names:** `cites_capture` and `reports_event`.
- **D2 Kind owner text:** keep the two event kinds as they are and let the
  registry's owner say both sides write them
  ("internal/store (computer), exported by internal/platform (corpusd)").
  The alternative is separate Store B kinds (`choir.published_event`).
  That avoids a shared name but loses idempotent ids across stores.
  Settle this at the start of phase 2 against the registry test, which
  holds each kind to a writer.
- **D3 Edge source:** from the version, not the publication. Versions are
  immutable, so nothing is tombstoned on revision.
- **D4 Cache:** in-process, 5 minutes. A purge on publish is not built.
- **D5 Weights and thresholds:** the starting values in §5.5, calibrated
  in the two checks.
- **D6 Where the block renders:** inside the article HTML (§5.8).

Open, recorded and not blocking:

- Move the `graphCitationEdges` skip list to a registry property, so a
  new structural kind cannot leak again.
- A `merged_into` or correction path for event memberships (the event
  doc's V1.5). Until it exists, a wrong join is permanent on the tape and
  is hidden in Store B by tombstoning edges.
- An explicit editorial "follow-up of" edge, written by management or
  Texture through a semantic act, so a continuing story links its
  predecessor with certainty. This design leaves room for it as another
  edge kind and does not build it. Texture would still be the only
  writer of document text. The edge is metadata about a story, and the
  doctrine should decide who may write it.
- Backfilling events for stories published before the layer went live.
  The layer does not resolve old captures, and this design does not ask it
  to.
- A `proj_story_capture` projection, only if the §8 latency criterion
  fails.
- Research-read captures have no events (§3.3). Stories built on them rely
  on shared captures.


## Merged from docs/reports/related-stories-phase3-research-review-2026-10-20.md (2026-10-20)

**Related stories, phase 3: the research desk reviews the graph, Jev classifies.**
Design, 2026-10-20. Extends [related stories, computed from the graph](related-stories-graph-design-2026-10-19.md) (phase 1 shared captures, live; phase 2 events, waiting on the one-incident blind rerun). Owner asks, owner notes §28: "more connections between articles … review all the existing articles and make sure that they are correctly connected in the object graph when they touch on similar topics, events, and e
- Status: design, not built. Mutation class: red.
- decision: the desk decides.
- evidence about objects that already exist and changes none of them.
- decision.

(Full text in git history at the path above.)
