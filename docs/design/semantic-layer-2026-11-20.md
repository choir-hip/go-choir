# The semantic layer: one graph of stories, happenings, entities and claims (design, 2026-11-20)

**Status: a proposal, not ratified.** Written 2026-11-20 between 22:30Z and
24:00Z by a design agent (Opus, no code, no production access) for the owner's
request of the same day. Nothing here is built unless
[current-architecture.md](../current-architecture.md) says so. It replaces the
plan of [entity-layer-2026-11-20.md](entity-layer-2026-11-20.md) from station
E3 on, and the graph half of
[corpus-first-retrieval-and-research-2026-11-18.md](corpus-first-retrieval-and-research-2026-11-18.md).
Owner decisions with defaults are in section F3; claims to precommit in F4.

**Owner ruling after this was written (2026-11-20T23:00Z).** "for stories with significant
overlap we should be revising not making new stories. This way we will
develop fewer deep stories on key topics rather than many shallow stories
that cover parts of it... Choir should instead build a singular narrative
that incorporates all the article as they accumulate over time." This
overrules decision 2 of section F3 (a development older than 24 hours opens a
new story linked by `follows`): a development revises the story that holds
the topic. The boundary, in the owner's words (2026-11-20T23:05Z): "a story stops growing when the
main point, the main beat changes... before the election, the story could be
about momentum or ads or debate or what someone said. These are all different
stories. Then election night, the story is the election results as they're
coming in. Then the next day, the story is who won and who lost... the point
is not to put everything in the same story, but that we can use natural
reasoning to cluster things so that we aren't making new stories every time a
newspaper has a new story." So the unit of a story is its main point, the
join is a reasoned judgment and not a threshold, and this document is to be
amended on that basis before any landing after L2.

Evidence tags. **M** measured, with the file it comes from. **C** read from
code at `origin/main` 3620f9232, with file and line. **A** assumed or
estimated by me. **U** not verified. Ground truth for what exists is
[semantic-layer-as-built-2026-11-20.md](../evidence/semantic-layer-as-built-2026-11-20.md)
("the survey"). The prior-art record is used only as a list of ways such
systems failed; none of its numbers is cited here as fact.

---

## The design on one page

**What the thing is.** The semantic layer is the paper's belief store about
the world. Every belief is a judgment: someone (System 1, a research agent,
the owner) decided that a capture reports a happening, that a story is about
an entity, that two names are one thing. Each judgment has a probability,
the evidence it read, and a time. The objects the judgments connect
(captures, stories, happenings, entities, claims, outlets) are nodes in one
graph, under one id scheme, in one database. Today the judgments are scattered
over ten homes in six shapes, the events live on a different computer from
everything they should join, and the agents cannot read any of it.

**The model in five sentences.** A **happening** (the code calls it an event)
is one occurrence at one place and time; a development is a new happening that
`follows` the old one, and a long-running situation (a war, a trial) is a
parent happening that others are `part_of`. An **entity** is a named,
persistent thing, and an **outlet** is simply an entity of type organization
whose key is its web domain. A **story** reports happenings (one primary) and
is about entities; its **claims** are the cited sentences the publish path
already checks against their captures. Every one of these is an object in the
Autopaper object graph, so one SQL statement can walk from a capture to its
happening to the other stories on it, with vectors in the same statement.
The judgments are records on the deciding computer's tape; the graph rows are
their projection, and any change (a merge, a split, a retirement) is a new
record that supersedes the old one and can itself be undone.

**What changes for the paper.** The paper opened 982 real stories in one day
(988 less six test documents). Of them, 171 (17.4%) were a true duplicate of a story opened in the 72 hours before;
197 (20.1%) were the second or later story on a happening (**M**,
[duplicate-stories-on-the-paper-2026-11-20.md](../evidence/duplicate-stories-on-the-paper-2026-11-20.md)
section 2). It happens because nothing checks: management's model is shown the
50 most recently updated stories, which reach back a median 27 minutes, while
the earlier story was over 8 hours old for half the duplicates (**M**, same
file, section 4). With this layer, a new capture is resolved onto a happening
**before** management sees it; if a live story already reports that happening,
the capture goes to that story as new material for its next version, by rule,
and no second story opens. Captures of new happenings still go to management.
A story whose happening already holds two independent outlets in the corpus
does not need the web to find its second source. Agents get a typed Go library
in their cell (`choir.World`) to ask these questions themselves.

**What I would build first.** Two landings that can start tomorrow, in
parallel. **L1** (orange) is station C1b, which is already being built. It
changes management's page from the newest 50 stories to the stories of the
last 72 hours by time, headlines only. Then the one model that decides open or
feed can see the earlier story. Today it saw the earlier story for 6 of 171
duplicates; an 8-hour page would reach 68 (**M**). This design builds on C1b
and does not redo it. **L2** (red): the event flush records what it was shown
and what it answered on the tape, reads its work from the ready list instead
of a memory queue, and widens its shortlist where the hand read found the
misses. L2 changes no model-call count. Then **L3** moves the happenings into
the Autopaper object graph and links every story to its happenings at open
and at publish. It absorbs the `story-events-e5` lane, changed: the lane
should not land as it stands (section E4).

**What it will not do by itself.** Consolidation saves about 10% to 15% of
web searches (**M**-based estimate, duplicates file section 3). On its own it
leaves about 2,800 to 3,200 searches a day, against a target of 1,000. The road to
under 1,000 searches a day runs through the second outlet: 90% of research runs
are a story's first run (**E**,
[research-runs-with-nothing-new-2026-11-20.md](../evidence/research-runs-with-nothing-new-2026-11-20.md)),
and a first run exists mainly to find a second outlet for the opening claim.
This design makes "do we already hold two outlets on this happening" a
lookup. It does not show that the lookup will answer yes often enough to reach
1,000; section D3 says what would.

---

## 0. How the problem was reframed (cognitive transforms)

1. **Ask what the thing is.** It is not a schema; it is a set of judgments
   about the world, each with its evidence and probability, plus the objects
   they connect. That makes provenance, reversibility and calibration
   properties of the one record kind, not features added later.
2. **Redefine the problem.** "Every story needs a semantic representation"
   becomes "every story, capture, happening, entity, claim and outlet must
   join to the others in one expression". The failing today is the joins, not
   the extraction: entities exist for 5,775 stories (**M**, survey 6.1),
   events exist for about 10,153 occurrences (**E**, survey 6.3), citations
   exist for 202,741 story-capture pairs (**M**). They do not meet.
3. **Physical bounds.** A join across the paper's SQLite file and corpusd's
   PostgreSQL cannot be one expression, so the events must move to where the
   captures, stories, vectors and entities already are. Volumes are small for
   PostgreSQL: about 237,000 captures, 10,900 stories, 10,150 events, 7,000
   entities, 1.73 million edges today (**M**, survey 6.1); new per day about
   10,000 captures and 300 to 560 published stories (**M**, survey 6.2), 982
   opened stories (**M**, duplicates file). One Jev call per flush, with every
   question about the flush's captures, is an owner rule. The cheapest unit
   of model work is a System 1 question at about $0.042 per million input
   tokens (**A**, derived in the entity design section 8 from SYSTEM1.md
   surprise 15).
4. **Map the options for the event home** (section A7): keep events on the
   computer with a public projection (the E5 lane); move them to the Autopaper
   object graph with the judgment on the tape (chosen); move everything to the
   computer (rejected: captures and vectors are shared).
5. **One mechanism, several jobs.** One flush call per minute answers, for
   each new capture: its happening, its entities, and, through the happening,
   which live story it belongs to. It replaces the intake labeller's separate
   call over the same captures. One record kind (`choir.ontology_act`) carries
   every change of identity for happenings and entities. One read path
   (`choir.World`) serves the research desk, management, Texture's scoped
   reads, the brief and the public pages.

---

## A. What the thing is

### A1. A story's semantic representation

A published story is represented by the graph facts that hang off its id. All
of them are already produced somewhere; the design gives them one home and
one id space.

- **The happenings it reports.** A `reports_event` edge to each, with a role:
  `primary` (the occurrence the story is about) or `secondary` (another
  occurrence it tells, such as background or a related incident). One primary,
  ties allowed (a roundup).
- **Its participants.** A `mentions_entity` edge to each entity with the role
  the story layer already records: `central` or `incidental` (**C**,
  `types/story_entity.go:30-47`). Finer roles inside a happening (who did what
  to whom) are not a closed code book; fixed role schemas are one of the
  prior-art failures. When a question needs one, a research agent writes it
  as a claim.
- **Its claims.** Each cited sentence of a version is already an object: a
  `choir.publication_check` keyed by version and anchor, with one
  `choir.support_judgement` per cited capture (**C**,
  `platform/publication_checks.go:42-57`; 506,987 and 305,454 rows, **M**,
  survey 1.3). That is the claim and its evidence. Nothing new is extracted.
- **Time.** When we learned it: the capture's `fetched_at`, the happening's
  `first_seen`, the version's time. When it happened: the capture's
  `published_at`, and a happening's `occurred`, derived as the earliest
  publication time of its counted members (no model call).
- **Place.** A place entity central to the story. There is no separate
  geography.
- **Relation to earlier stories.** Same happening: they share a primary
  `reports_event`. A development: the new story's primary happening
  `follows` the earlier one's. Background: a `secondary` report of an older
  happening. A correction: a later version whose claim supersedes the earlier
  version's claim (versions already carry `supersedes`, 25,557 rows, **M**).

### A2. What "event" means here, and at what grain

**An event is a happening: one occurrence, at one place, at one time.** The
event layer's definition stays word for word (**C**, `event_layer.go:959`):
"the occurrence itself, however many outlets report it ... A later
development of it, another incident of the same story, place or actors, a live
blog or roundup spanning several incidents, and an opinion or analysis piece
about a topic are each a different event."

Why this grain and not a storyline or a topic:

- **Corroboration is counted per occurrence.** Two outlets confirm a fact only
  if they report the same occurrence. A coarser grain fakes corroboration,
  which is the false-merge history the 0.92 threshold came from (**C**,
  `event_layer.go:72-77`).
- **The measured misses were same-occurrence pairs, not grain disputes.** Of 10
  "should have joined" events in the hand read, all were the same occurrence
  (the India-Japan 2+2 meeting, the Bukele candidacy) (**E**,
  [event-join-rate-hand-read-2026-11-20.md](../evidence/event-join-rate-hand-read-2026-11-20.md)
  section 2a).
- **The storyline already has a home: the story.** A Texture story is the
  editorial view over happenings. It reports a primary happening and, by
  `follows`, its developments. A separate storyline kind would be a second
  home for what the story already is.
- **A topic is not an object.** "Energy", "Health" are what vectors and front
  sections are for. The entity hand read found topics attached as entities
  (**E**, `entity-layer-baseline-hand-read-2026-11-20.md` section 2); this
  design keeps them out of both kinds.

How the two hard shapes are held:

- **A development** (a reaction, a ruling after a hearing, a death toll
  revised) is a new happening with a `follows` edge to the earlier one. Today
  `follows` is a field on the event (**C**, `store/world_event.go:34-44`); it
  becomes an edge so it can be read in both directions and superseded.
- **An ongoing situation** (the war in Ukraine, the French lycée movement, a
  trial) is a parent happening with no single time or place, and each
  occurrence is `part_of` it. The flush never mints a parent. A research
  agent mints one through `choir.World.Judge` when it sees a recurring pattern
  (section B4), which is the "promote a recurring pattern" act.

### A3. What an entity is

The entity design's definition stands (**C**, its section 2): a named,
persistent person, organization, place, or named product, work or law. Not a
topic, not an event, not a demonym, not a role without its holder, not the
capture's own byline outlet. The six types stay; removing one would orphan
rows.

Two changes:

- **An outlet is an entity.** An outlet (Reuters, the BBC) is an organization.
  Its key is `outlet:<registrable domain>`, so a capture's outlet id is a pure
  function of its URL, with no lookup. When a story is about the BBC, its
  `mentions_entity` edge points at the same object a BBC capture's outlet
  does, after one merge of the name-minted "BBC" row into it. The two outlet
  functions that disagree today (`outlet.Of` and `registrableDomain`, **C**,
  survey item 8) become one: the registrable domain is the key, and the folds
  `outlet.Of` hard-codes (`bbc.co.uk` into `bbc.com`) become recorded
  `merge` acts. Wire services stay what they are: an `origin` field, so two
  outlets carrying one Reuters story count as one origin (**C**,
  `event_layer.go:303-321`).
- **Identity is opaque and stable once minted.** The first entity of a name
  keeps today's key `ent:<24 hex of the normalised name>` (**C**,
  `types/story_entity.go:125-133`). A second, different entity with the same
  normalised name gets `ent:<hash of the name and its opening mention's id>`
  (the entity design's qualified-id rule). A Wikidata id is a `same_as` act
  with evidence, not the identity; the four `wd:Q` keyed rows stay valid.

### A4. Claims: first-class, at the cost the publish path already pays

Claims are first-class objects at the grain of a published version's cited
sentence, because that object already exists (A1) and costs nothing more to
keep. What is new is one edge, `contradicts`, between two claims, written only
by a research agent's judgment (section C) or by the correction path (D6).

Not built, and why. Claims are not extracted from captures. System 1 cannot
write text, and extraction by a generative model over 10,000 captures a day is
the "LLM-extracted triples" failure the prior art lists, at a model cost this
design does not need for any of the owner's goals. A claim with no story
behind it has no reader yet.

### A5. Kinds and edges: the smallest set

Every kind below lives in the Autopaper object graph (corpusd's `graph`
database). Row custodian is the corpusd kernel service, as it already is for
`choir.entity` (**C**, `kernelservice/kernelservice.go:103-110`). Judgment
authority is the desk whose tape records the rows project (section A7).

| Kind | What it is | Judgment authority | Id rule (key; canonical is `obj:<kind>:<b64 owner>:key-<sha256 of key>`) | Status |
|---|---|---|---|---|
| `choir.web_capture` | one fetched page | none: a fact of fetching | URL or content, unchanged (**C**, `objectgraph/web_capture.go:11-45`) | kept |
| `choir.publication`, `choir.publication_version` | a story and its versions | Texture writes; publish | `pub-<32 hex>`, unchanged | kept |
| `choir.publication_check` (+ `choir.support_judgement`) | a claim: one cited sentence of a version, and its support verdict per cited capture | the support judge | (version, anchor), unchanged | kept; named the claim |
| `choir.world_event` | a happening | **research** (the host flush declares research) | `ev-<20 hex>` of (computer, opening capture), unchanged (**C**, `event_layer.go:294-299`) | **moved** from the computer's store |
| `choir.entity` | a named persistent thing, outlets included | research | `ent:<24 hex>` for the first of a name; qualified for a second; `outlet:<domain>` for an outlet; `wd:Q<n>` rows stay | kept, extended |
| `choir.ontology_act` | one change of identity or type: `merge`, `split`, `retire`, `retype`, `part_of`, `follows`, `same_as`, `not_same` | research, or the owner | (op, subject, tape record id) | **new**; replaces the unbuilt `choir.entity_merge` and `choir.entity_split` |
| `choir.entity_review`, `choir.entity_stopword` | the lexicon's learning loop | corpusd from exports | unchanged | kept |

Deleted kinds: `choir.event_membership` (becomes the `in_event` edge),
`choir.capture_label` and `choir.capture_label_override` (their event and
topic answers are absorbed by the flush, L4), and `choir.event_projection`
(the E5 lane's kind, never landed; the event itself takes its place).

| Edge | From, to | Metadata (part of the edge id) | Written by | Status |
|---|---|---|---|---|
| `cites_capture` | version, capture | `{"relation":"cites"}` | publish | kept (202,741 rows, **M**) |
| `in_event` | capture, happening | `{"role": opened\|member\|tentative, "record": <tape id>}` | the flush's export | registered, 0 rows; first written in L3 |
| `reports_event` | publication, happening | `{"role": primary\|secondary, "basis": opened\|cited\|judged}` | the host at open and at publish | **new** (the E5 lane) |
| `follows` | happening, earlier happening | `{"record": <tape id>}` | flush or Judge | **new** (today a field) |
| `part_of` | happening, parent happening | `{"record": <tape id>}` | Judge only | **new** |
| `mentions_entity` | publication or capture, entity | `{"role": central\|incidental\|tentative}` | story export; flush export for captures | kept; capture side new |
| `contradicts` | claim, claim | `{"record": <tape id>}` | Judge only | **new** |
| `supersedes` | version, version | as today | publish | kept |

Deleted edges: `related_story` (4 rows; its relations `same_incident`,
`later_development`, `same_subject` are a shared happening, `follows`, and a
shared central entity), `relies_on` (1 row; "ranks nothing and no desk sees
it", **C**, `registry.go:357-359`). The eight provenance edges written and
never read (`was_derived_from`, `was_associated_with`, `attested`,
`granted_consent`, `authored_review`, `derived_from_proposal`, `owns`,
`has_agent`, 22 to 36,461 rows each, **M**) are listed for deletion only after
a read of whether any outside contract (the publication bundle, a verifier)
parses them (**U**).

Net: one new object kind, four new edge kinds; three object kinds and two edge
kinds deleted, eight more edge kinds reviewed for deletion.

### A6. One identity space: today's id forms collapse

Every object has exactly two forms: its canonical id (graph, vectors, edges)
and its short key (what people and public URLs use). The key determines the
canonical id given the owner, by `StableSuffixFromKey` (**C**,
`objectgraph/object.go:124-135, 214`). The verb surface accepts either and
returns both.

| Concept | Forms today | After |
|---|---|---|
| story | `pub-<32 hex>`; canonical `...key-<64 hex>`; Texture `doc_id`; route path; a podcast clip's `story_route` string | canonical plus `pub-` key; `doc_id` maps by the pure `StoryPublicationID` (**C**, `types/story_entity.go:135-140`); a clip's route string becomes a `clip_of` edge later (residual R5) |
| capture | canonical; a frame handle `r1.c1` in a Texture cell | canonical; the frame handle stays a cell-local alias, never stored |
| happening | `ev-<20 hex>`, computer-scoped, on the computer only | canonical in the Autopaper object graph; `ev-` key kept, so existing ids survive |
| entity | `ent:`/`wd:` key; canonical | unchanged |
| outlet | two functions' strings; `host`, `source_id` | the entity `outlet:<registrable domain>`; `proj_capture_index` gains a derived `outlet_id` column |

The owner part of every id stays. A page captured by two owners is two
objects, and so is "Anthropic" in two owners' graphs (**C**, survey 1.1).
That is the privacy boundary, not a defect. The paper's objects are public
because the paper publishes; another owner's computer reads them as
candidates and mints its own objects only for what it alone holds (F3,
decision 7).

### A7. Where it lives, who decides, and the doctrine tension

**Decision: happenings move from the paper computer's own store to the
Autopaper object graph, beside the captures, stories, vectors and entities.**

| Option | Joins in one expression | Who can read | Cost |
|---|---|---|---|
| Keep events on the computer, corpusd holds a projection (the E5 lane, the entity design 3.3) | only the projection joins; the home does not | the paper, through a key; the public through the projection | two copies of every event; memberships immutable, so no correction; the computer's index is capped at 10,000 events and already drops the oldest (**C**, `event_layer.go:85-86`; about 10,153 events, **E**) |
| **Move them to the Autopaper object graph; the judgment stays on the tape** | yes | as captures and entities: public objects for all, owner-private ones for their owner | a migration of about 10,000 events and their memberships (section E5) |
| Move everything to the computer | no: captures and vectors are shared across 35 owners (**M**, survey 1.1) | the computer only | not possible without copying the corpus |

Doctrine allows it. I27 says "the Autopaper object graph and a computer's graph
are one model with two scopes" and "the graph is the tape's projection"
(**C**, `choir-doctrine.md:449-483`). A one-time import that empties the old
home is a migration, not a second home (same lines). The vector and projection
tables are "rebuildable caches, derived indexes", a named exception (**C**,
`choir-doctrine.md:473-474`).

**The doctrine tension, and how this resolves it.** I26 says management "holds
no document/event mutation authority" (**C**, `choir-doctrine.md:419-421`),
while the manifest makes management the custodian of `choir.world_event` and
`choir.event_membership` (**C**, `deskmanifest/manifest.go:356-363`), written
by a host loop that declares management (**C**, `registry.go:122-127`). The
survey read "event" in I26 as the runtime event. Either way, the clean split
is by what the judgment is about:

- **Research decides what the world is.** Research has world-evidence
  authority (I26) and is already the authority for `story.entities` and
  `story.relation`. A judgment that a capture reports a happening is world
  evidence. It mutates nothing in the world. So the flush, a host loop with no
  run, declares **research**, and research is the judgment authority for
  happenings, memberships, entity links and ontology acts.
- **Management decides what the paper does about it.** Open a story, feed it,
  decline it: `choir.capture_disposition` and `choir.management_task` stay
  management's (**C**, `manifest.go:360-362`). Routing a capture to the story
  that already reports its happening is a disposition made by rule, so it is
  management's kind, written by management's rule (section D1). That is the
  coherence role I26 gives management.

This moves no authority to a new desk and adds no desk. It is a custody change
of two kinds (red, in L3), and the owner's to ratify (F3, decision 1).

### A8. Canonical and derived, and what records each judgment

| Thing | Canonical or derived | Record that made it |
|---|---|---|
| a capture, a publication | canonical (a fetch, a publish) | the capture route; the publish path |
| a happening's membership, mint, `follows` | **canonical: the judgment** | `semantic.flush` tape record (new, L2), on its own channel `semantic:<computer>` |
| an entity link from a capture | canonical: the judgment | the same `semantic.flush` record (L4) |
| an entity link from a story | canonical: the judgment | `story.entities` (exists) |
| a merge, split, retirement, retype, `part_of`, `same_as`, `contradicts` | canonical: the judgment | `semantic.act` tape record (new, L8), citing a `system1.judgment` receipt |
| the graph rows for all of the above | derived: a one-way, re-runnable export of the records | the export carries the record id in the edge metadata |
| `reports_event` with basis `cited` | derived deterministically from `cites_capture` and memberships | none needed |
| `reports_event` with basis `opened` | derived from the opening capture's membership | none needed |
| outlets and origins of a happening, counts, `occurred` | derived at read | none |
| vectors, `proj_capture_index`, `proj_capture_resolution`, the lexicon | derived indexes (I27 exception) | none |

A Jev answer is not reproducible (identical requests gave 13 distinct responses
in 30, **E**, SYSTEM1.md surprise 6), so the judgment record is the authority
and a re-ask is a new judgment. The graph rows can be dropped and rebuilt from
the records at zero model calls. Two requirements follow from the code:

- **The semantic records need their own channel.** System 1 judgments,
  `story.entities` and `story.relation` share `system1:<computer>`, and every
  System 1 call reads up to 10,000 events of that channel to look for a replay
  (**C**, `system1_verb.go:66, 87-89, 230`). A flush record a minute would
  fill it in about a week (**A**, about 1,330 flushes a day from the entity
  design section 8) and slow every call. `semantic:<computer>` is read by
  record id or by a sequence cursor, never by a bounded scan.
- **A rebuild pages by sequence**, not by the 10,000-event bound, so it can
  read the whole history (the entity design left this **U**).

**What the flush record holds.** Today the event layer stores p, model and cost
on each membership, but not the question or the shortlist, so the hand read
could not tell a shortlist miss from a judge miss (**E**, join-rate file
section 2). The record holds, per capture: the candidates offered (ids, in the
order shown), the full distribution, the decision taken, and the rule that
took it (for example `member p>=0.92`). Per merge question, the same. It holds
ids and numbers, not text; the text is in the captures.

**The restore gap (residual R1, carried from the entity design).** The
Autopaper object graph is outside a computer's restore set. Restoring the paper
to an older head rewinds its tape but not corpusd. Every exported row carries
its record id, so a reconcile can tombstone rows whose record the tape no
longer holds. It is named, not built in these landings.

---

## B. Identity and change as judgments

### B1. Three tiers, each where it is cheapest

| Tier | Used for | Cost | Recorded as |
|---|---|---|---|
| Vectors and code (no model) | finding candidates: a capture's nearest captures and their happenings; entity lexicon and token, initials and neighbour lookups (built in E2, **C**, current-architecture "Entity layer ... E2" row); the outlet key; occurred times; counts | an index read | nothing: a candidate is not a belief |
| System 1 (Jev) | typed questions over candidates with a recorded distribution: which happening, which entity, same thing or not | one call per flush, every question in it | `semantic.flush`, or a `system1.judgment` receipt a `semantic.act` cites |
| RLM agents (research) | hard cases and the ontology itself: split a conflated entity, mint a parent happening, re-type, decide a contradiction, audit a sample | a research run | `semantic.act`, citing receipts and evidence ids |

A vector score is never a decision. The duplicates file shows why: story-vector
cosine 0.85 finds 93 of 100 published duplicate pairs with precision 0.46, and
0.92 has precision 0.71 (**M**, duplicates file 4.2). Vectors propose; a judge
decides.

### B2. The flush: one call, every question about its captures

What a flush is today: a minute after the first queued capture, one Jev call
asks, for each capture, which of up to 30 recent events and 3 vector-neighbour
events it reports, a sibling capture, or new (**C**, `event_layer.go:39-59,
1010-1072`). Separately, the intake labeller asks Jev, for each polled capture,
which story reports the same event and which shares its topic, in another call
before arrival (**C**, `intake_labels.go:5-20`). Two calls over the same
captures are the defect SYSTEM1 rule 2 names.

After L4, one flush call asks, for each capture in it:

1. **its happening**: the candidates (B3), each listed once in `state.events`
   with its headline, its newest member titles and, when a story reports it,
   that story's headline; plus `after_<e>` for a development, `k<n>` for a
   sibling capture, and `new`;
2. **each mention's entity**: its candidates from the E2 lookup, the six
   `new_<type>` choices, `same_as_<k>.<m>` for the same form elsewhere in the
   flush, and `not_an_entity` (the entity design section 4.3);
3. **merge questions** for each pair of candidate happenings or entities the
   flush exposed as possible duplicates ("do these name the same real
   occurrence / thing?"), so a duplicate is asked about exactly when it would
   split a new link, with no sweep and no schedule.

Questions are answered blind to each other (SYSTEM1 rule 1). A split is made
only where a vendor limit forces it (64,000 tokens a request, 32,000 for the
state plus one question, 255 options a choice question, **C**, SYSTEM1.md
3.3), by captures, never by question kind, and the code names the limit
(`planJevCalls`, **C**, `event_layer.go:382`).

**What arrives when.** The flush moves in front of arrival: management's task
for a poll's captures starts when the flush has resolved them. That adds about
one to two minutes to intake (**A**: the 60-second timer plus one call). It is
the price of knowing the happening before the open-or-feed choice, and it is
the owner's call (F3, decision 3).

**Work list, retries and restarts** come from the entity design section 4.1,
unchanged: the flush reads the ready list (captures with a vector, or that will
never have one, and no `proj_capture_resolution` row) from corpusd in the one
request it already makes; a failed call writes nothing and the next flush asks
again; after a crash nothing runs at boot, and the next arrival-triggered flush
picks up what a crashed flush never answered. That is a derived index catching
up, not resumed work (the entity design's owner question 2, default kept).
Research reads join happenings (for corroboration and the search state) but
never route anything: a page research read is not news to the paper.

### B3. What makes a singleton join

The measured state: 84.1% of questions minted a new event and 2.8% joined
(**E**, join-rate file section 1); 282 of the 300 newest events had one member
(**E**, research-runs file); 10 of 50 hand-read mints should have joined
(**E**, join-rate file 2a). The causes found, with the fix for each:

| Cause (measured or read) | Share of the 10 misses (**E**) | Fix | Landing |
|---|---|---|---|
| No vector at flush: 78% of questions had none (03:28Z to 05:12Z) | 1 certain, others unknown | the embed kick on capture write (landed 2026-11-20 as E1; effect unmeasured, **U**) plus the ready list, which waits for the vector | E1 done; L2 |
| Same-flush siblings: the only link is a `k<n>` answer at p of at least 0.92 | 6 | cluster the flush's captures by vector before asking; list each capture's near siblings first among its `k` choices (Jev leans to the first option, **E**, SYSTEM1.md surprise 6); give sibling answers the same three bands as event answers | L2 |
| The recent slot is the 30 newest events, about 3 minutes of a burst | 4 to 5 | candidates from every distinct happening among the capture's 50 nearest captures within 14 days (today 12 neighbours, at most 3 events, **C**, `event_layer.go:55-59`), bounded only by the vendor's 255 options and token budget | L2 |
| A neighbour in no event offers nothing (1,074 unmapped in 100 minutes, **E**) | unknown | falls as joins rise; the L10 backfill gives old captures entities, which feed the next slot | L4, L10 |
| Entities unused for events | unknown | a slot of recent happenings whose members share at least two central entities with the capture (instant from the lexicon, works when the vector is late) | L4 |
| Fragments minted before their partner existed | unknown | merge questions ride the flush for pairs of recent happenings whose members are near (B2 item 3) | L4 |

**One distribution, several thresholds by consequence.** SYSTEM1.md 5.4: the
riskier the action, the higher the bar. The flush stores the distribution once
and each reader applies its own bar:

- **counted toward corroboration** (outlets, origins): p of at least 0.92, as
  today, because a false merge here fakes a second source;
- **routes a capture to a live story** (D1): starts at the same 0.92, and is
  lowered only when the audit (B6) shows precision of at least 0.9 at the lower
  band, because a wrong feed costs one Texture turn while a duplicate story
  costs a research run and a public repeat;
- **shown to management as a likely story**: 0.5 to the routing bar, the
  tentative band of today (**C**, `event_layer.go:78-79`).

No new threshold is invented on day one. The record makes each one movable
later with no re-ask (SYSTEM1.md 5.4: "keep stored answers so re-routing under
a new threshold costs no calls").

### B4. Changing the ontology: merge, split, retire, re-type, promote

Every change is a `choir.ontology_act`, recorded on the tape first as a
`semantic.act`, and exported. Nothing is overwritten.

- **Merge** (two ids, one thing): the act names the merged id and the
  survivor (the one with more central story mentions, ties to the older:
  deterministic, so a replay chooses the same). Edges are never rewritten.
  Every read resolves the chain of merges when it reads, so a merged id stays a
  valid address that answers with its survivor. Cycles are refused at write.
  Automatic at p of at least 0.92 when the question rode the flush; otherwise
  a research agent's act.
- **Retire** (not an entity: "Chinese", "Markets"): a merge into nothing. A
  retired id answers "retired", never 404. Its name goes on the stoplist. This
  is how the 29% of story attachments that are not entities (**E**, baseline
  section 2) stop attaching.
- **Split** (one id, two things: the second John Smith; Parliament on four
  countries' parliaments, **E**, baseline section 4): a research agent names
  the new entity and the mentions it takes. The moved edges are tombstoned and
  re-created on the new id under one act. A split detector (an entity whose
  mentions' neighbourhoods fall into two disjoint groups) lists candidates in
  the status read; it never splits by itself.
- **Re-type** (a person filed as an organization): an act that changes the
  type the reads return.
- **Promote a pattern** (`part_of`): when a research agent sees three or more
  happenings of one ongoing situation, it mints the parent happening and links
  the children. The same act kind can later promote a new entity type when the
  category-gap counter shows demand ("Ebola is a disease", the entity design
  section 2); adding a type is a registry change and the owner's.
- **Undo** is a later act that supersedes the earlier one (`superseded_by`
  exists on every object, **C**, `objectgraph/object.go:36`). History stays;
  the reads follow the newest act. An owner undo is an owner act on the tape.

A person merge needs stronger evidence than other kinds (same-name conflation
is the documented failure for people): the flush never merges two people; only
a research agent's act with at least two pieces of distinguishing evidence
(different outlets' captures naming the same role and place) can.

### B5. Time: only as far as it pays

- **When we learned it** is on everything: `fetched_at`, the record's time,
  `first_seen`. It is what "new since T" and "what did we believe at T" read.
  It pays at once: the search state (D3), the story diff (C, question 2) and
  the audit all need it.
- **When it was true** is kept only where a reader exists: a capture's
  `published_at`, a happening's derived `occurred`. Validity intervals on
  facts between entities (who held which office when) are not built: no
  question the paper asks today needs them, and they would need relation
  facts this design does not extract.
- **Superseding, never overwriting** holds for judgments (acts and re-asked
  memberships tombstone the old edge and point at the new record), for
  claims (versions supersede versions), and for ids (merges resolve at read).

### B6. Quality, measured continuously by someone other than the writer

Each judgment is a forecast with a probability: the learning engine's first
stroke (**C**, `learning-engine.md` sections 1 to 3). The design gives it
three teachers, none of which is the flush:

1. **Texture, on every routed feed.** When a capture is routed to a story by
   rule, Texture either uses it (cites it, or revises from it) or marks it
   unused with a reason (I15: every source is cited, toolbar or marked unused,
   **C**, `choir-doctrine.md:333-339`). That is an outcome the flush cannot
   write. Each routed feed settles its membership forecast true or false. How
   cleanly the mark-unused reason can be read as "wrong story" is **U**; L5
   adds the reason code if it is not there.
2. **An audit sample, judged by a research run.** A uniform 3% of each day's
   flush decisions (memberships, mints, entity links), seeded so anyone can
   redraw it, is cast to a research run once a day by management (the first
   management turn after 00:00Z; a crashed audit run is not resumed, the next
   day's covers it). The run reads the captures themselves, never System 1's
   answer as a premise (SYSTEM1.md 5.5), and records verdicts as
   `semantic.act` rows of kind `audit`. That settles the forecasts and gives
   precision per probability band, which is what moves the thresholds in B3.
3. **The owner, on demand.** The sample route (the entity design section 7)
   serves any draw for a hand read.

The paper goes back to the student (rule 2): the flush's calibration table by
band, and each desk's settled forecasts, are readable by the desks.

Rule 1b note: the audit run is research, and research is also the judgment
authority for the flush. The flush is System 1 driven by a host loop; the
audit is an RLM run that reads the evidence afresh. They are different
actors, but the same desk. If the owner wants a stricter separation, the audit
can be a Texture-free management task instead (F3, decision 9).

---

## C. The agent's side: `choir.World`

### C1. What it is

A typed Go library in the cell: one value `choir.World` with six methods, in
the pattern `choir.Searches` already uses (**C**, `yaegikernel/choir.go:457`).
Each method is one host action over the frame protocol, answered by one corpusd
request. The agent composes them in Go. There is no query language: a free
query language invites invented ids and malformed joins, and the agent already
writes Go. Every result is a struct with ids and provenance, never prose.

Design rules:

- **Ids in, ids out.** Every object carries its canonical id and its key.
  `choir.Ref` turns any public form into an id without a read.
- **Vector and graph in one call.** `Find` runs the vector search, the BM25
  half and the graph filters (in this happening, about this entity, from this
  outlet, cited by no story) in one corpusd statement, as the ranked corpus
  answer already joins vectors to `cites_capture` (**C**, survey section 3,
  `capture_rank.go:629`).
- **Bounds are part of the type.** Every read takes a `Limit` (default 25, at
  most 200) and returns a `Bound` with `Complete` and a cursor. `Walk` takes at
  most four hops. Every read has a deadline (4 s for the vector half, as today;
  8 s for the request). These bound a read, not a model call.
- **No cap on System 1 questions.** `Ask` sends every question in one call and
  splits only where a vendor limit refuses, naming the limit. The 12-question
  cap of today's verb (**C**, `yaegikernel/system1.go:26`) and the story
  analysis's two calls that follow from it (**C**, `story_analysis.go:322-347`)
  are deleted (SYSTEM1 rule 7).
- **Custody by the surface, not by the agent.** The host runs every read as
  the run's owner and computer (C5).

### C2. The types

```go
package choir // what a cell sees; served by the host action "world"

type ID string // always canonical: "obj:<kind>:<b64 owner>:<suffix>"

// Ref turns a public form into an ID with no read: "pub-…", "ev-…",
// "ent:…", "wd:Q…", "outlet:bbc.com", a held capture's URL, or an ID.
func Ref(s string) (ID, error)

type Prov struct {
	By     string    // "research:flush", "research:run-…", "owner", "rule"
	Record string    // the tape record of the judgment
	P      float64   // the probability the decision rested on; 0 for a rule
	Rule   string    // "member p>=0.92", "cited>=2 or 30%", "opened"
	At     time.Time // when we learned it
}

type Bound struct {
	Complete bool     // false: more exists past Limit or the deadline
	Next     string   // cursor for the next page
	Ms       int      // time the read took
	Labels   []string // labels this read added to the run
}

type Link struct {
	To   ID
	Role string // member|tentative|opened, primary|secondary, central|incidental
	Prov Prov
}

type Capture struct {
	ID, Outlet                 ID
	URL, Title, Lead, Language string
	Published, Fetched         time.Time
	Intake                     bool  // a source poll's, not a research read
	Event                      *Link // nil until a flush resolved it
	Labels                     []string
}

type Event struct {
	ID                            ID
	Key, Headline                 string // Key is "ev-…"
	FirstSeen, LastSeen, Occurred time.Time
	Members, Intake               int  // counted members; of them, from polls
	Outlets                       []ID // distinct outlets of counted members
	Origins                       int  // distinct origins: a wire story counts once
	Follows, PartOf, Into         ID   // Into is set when merged
	Prov                          Prov
}

type Story struct {
	ID              ID
	Key, Title      string // Key is "pub-…"
	Route           string
	Published       bool
	Latest          ID // latest version
	LatestAt        time.Time
	Events          []Link // reports_event
}

type Entity struct {
	ID                  ID
	Key, Name, Type     string
	Aliases             []string
	Into                ID   // set when merged
	Retired, Public     bool // Public: a person served on public pages (C5)
	Prov                Prov
}

type Claim struct {
	ID, Story, Version ID
	Text               string
	Cites              []ID
	Support            string // the support verdict
}

type Objects struct {
	Captures []Capture
	Events   []Event
	Stories  []Story
	Entities []Entity
	Claims   []Claim
	Missing  []ID // not found, or not visible to this run
	Bound    Bound
}

type Edge struct {
	From, To ID
	Rel      Rel
	Role     string
	Prov     Prov
}
type Edges struct {
	Edges []Edge
	Bound Bound
}

type Hit struct {
	ID             ID
	Kind           string   // capture|story|event|entity|claim
	Score          float64  // cosine for the vector half, else BM25 rank
	Why            []string // text, meaning, story, event, entity
	Title, Passage string
	At             time.Time
}
type Hits struct {
	Hits  []Hit
	Bound Bound
}

type Query struct {
	Text                  string // embedded once; also searched as words
	Like                  ID     // or: near this object's vector
	Kinds                 []string
	Since, Until          time.Time
	Event, Entity, Outlet ID   // graph filters, joined in the same statement
	Uncited               bool // captures no story version cites
	Limit                 int  // 0 means 25; at most 200
}

type Opt struct {
	Since, Until time.Time
	Role         string
	MinP         float64
	Within       ID  // required to read a non-public person's mentions (C5)
	Limit        int // 0 means 25; at most 200
}

type Hop struct {
	Rel   Rel
	Role  string
	Since time.Time
	Limit int // per node at this hop; at most 200
}
type Paths struct {
	Paths [][]Edge
	Bound Bound
}

type Rel string

const (
	Members     Rel = "members"      // event -> captures
	EventOf     Rel = "event_of"     // capture -> event
	ReportedBy  Rel = "reported_by"  // event -> stories
	Reports     Rel = "reports"      // story -> events
	Cites       Rel = "cites"        // version -> captures
	CitedBy     Rel = "cited_by"     // capture -> stories (Prov names the version)
	Mentions    Rel = "mentions"     // story or capture -> entities
	MentionedIn Rel = "mentioned_in" // entity -> stories and captures
	Follows     Rel = "follows"      // event -> the earlier event
	FollowedBy  Rel = "followed_by"  // event -> later events
	PartOf      Rel = "part_of"      // event -> parent event
	Parts       Rel = "parts"        // parent event -> events
	Published   Rel = "published"    // outlet -> captures
	Claims      Rel = "claims"       // story -> its latest version's claims
	Contradicts Rel = "contradicts"  // claim -> claims
)

type Question struct {
	ID, Text string
	Choices  []string
	About    []ID     // any objects; the host renders each one itself
	Passages []string // text the cell read, as today's verb allows
}
type Answer struct {
	ID string
	P  map[string]float64 // the distribution, as the judge gave it
}
type Answers struct {
	Receipt string   // "system1:<record id>"
	Answers []Answer
	Split   string   // empty, or the vendor limit that forced a second call
}

type Act struct {
	Op              string // merge|split|retire|retype|part_of|follows|same_as|not_same|contradicts|audit
	Subject, Object ID
	Type            string // for retype, and split's new entity
	Moves           []ID   // for split: the mentions that move
	Receipt, Question string // the System 1 answer the act rests on
	Reason          string   // at least 20 characters when the act goes against the top answer
	Evidence        []ID
}
type Receipts struct {
	Recorded []string // tape record ids
	Exported bool
	Error    string
}
```

### C3. The verbs, their cost and their custodian

| Verb | Signature | One call does | Cost | Custodian of what it reads or writes | Grant and holders |
|---|---|---|---|---|---|
| `Get` | `Get(ids ...ID) (Objects, error)` | reads up to 200 objects of any kind by id, merges resolved | one corpusd read by exact ids; tens of ms (**A**) | rows: corpusd | `semantic.read`: research, management; Texture scoped to its own story's events |
| `Find` | `Find(q Query) (Hits, error)` | hybrid search with graph filters | one query embed (median 0.26 s for longer queries, **M**, current-architecture "Vector index" row) plus one statement; 4 s vector deadline | corpusd | `semantic.read` |
| `Neighbors` | `Neighbors(id ID, rel Rel, o Opt) (Edges, error)` | one typed hop with filters | one indexed edge read | corpusd | `semantic.read` |
| `Walk` | `Walk(from ID, hops ...Hop) (Paths, error)` | up to four hops in one request | one request; at most 200 per node a hop and 2,000 nodes in all; 8 s | corpusd | `semantic.read` |
| `Ask` | `Ask(qs ...Question) (Answers, error)` | System 1 over any objects, every question in one call | one Jev call; tokens by the rendered objects (about $0.0001 for a 2,000-token call, **A**) | the System 1 kernel service; `system1.judgment` on the tape | `system1.call`: research (as today, **C**, `deskmanifest/grants.go:144`) |
| `Judge` | `Judge(acts ...Act) (Receipts, error)` | records ontology acts and exports them | a tape append and one corpusd export; no model call | research is the authority; rows corpusd | `semantic.judge`: research only |

The reads replace the event half of `SearchWireCorpus`, which is an in-memory
substring match over the newest 3,000 events (**C**,
`researchtools/corpus_events.go:54`). `SearchWireCorpus` keeps its capture and
published halves until `Find` has carried a week of research runs, then folds
into it. `Relate` and `RelateStories` stay until `Judge` covers them (L8,
L9).

### C4. Ten questions, as the agent would write them

Helpers an agent defines once in its cell are written inline. Errors are
printed, not hidden.

**1. Which published stories already report this happening?** (consolidation)

```go
r, err := choir.World.Get(capID)
if err != nil || len(r.Captures) == 0 {
	fmt.Println("no capture:", err, r.Missing)
	return
}
c := r.Captures[0]
if c.Event == nil {
	fmt.Println("not resolved yet; the next flush answers it")
	return
}
er, _ := choir.World.Get(c.Event.To)
ev := er.Events[0]
look := []choir.ID{ev.ID}
if ev.Follows != "" { // a development belongs with the story of what it follows
	look = append(look, ev.Follows)
}
for _, id := range look {
	rs, err := choir.World.Neighbors(id, choir.ReportedBy, choir.Opt{Since: time.Now().Add(-72 * time.Hour)})
	if err != nil {
		fmt.Println(err)
		continue
	}
	for _, e := range rs.Edges {
		fmt.Println(e.From, e.Role, e.Prov.Rule, "via", id, "p", c.Event.Prov.P)
	}
}
```

**2. What is new about this happening since the story's last version?**

```go
sr, _ := choir.World.Get(storyID)
s := sr.Stories[0]
var primary choir.ID
for _, l := range s.Events {
	if l.Role == "primary" {
		primary = l.To
	}
}
cited := map[choir.ID]bool{}
cs, _ := choir.World.Neighbors(s.Latest, choir.Cites, choir.Opt{Limit: 200})
for _, e := range cs.Edges {
	cited[e.To] = true
}
ms, _ := choir.World.Neighbors(primary, choir.Members, choir.Opt{Since: s.LatestAt, Role: "member", Limit: 200})
var fresh []choir.ID
for _, e := range ms.Edges {
	if !cited[e.From] {
		fresh = append(fresh, e.From)
	}
}
got, _ := choir.World.Get(fresh...)
for _, c := range got.Captures {
	fmt.Println(c.Fetched.Format(time.RFC3339), c.Outlet, c.Intake, c.Title)
}
fmt.Println(len(fresh), "new since", s.LatestAt.Format(time.RFC3339), "complete:", ms.Bound.Complete)
```

**3. Everything we hold about this entity since T, with sources.**

```go
hs, _ := choir.World.Find(choir.Query{Text: "Pete Hegseth", Kinds: []string{"entity"}, Limit: 3})
ent := hs.Hits[0].ID
since := time.Now().Add(-48 * time.Hour)
ed, err := choir.World.Neighbors(ent, choir.MentionedIn, choir.Opt{Since: since, Limit: 200})
if err != nil {
	fmt.Println(err) // a non-public person needs Opt.Within (C5)
	return
}
for _, e := range ed.Edges {
	fmt.Println(e.From, e.Role, e.Prov.By, e.Prov.P, e.Prov.At.Format(time.RFC3339))
}
// and the happenings it is central to, in the same request
ps, _ := choir.World.Walk(ent,
	choir.Hop{Rel: choir.MentionedIn, Role: "central", Since: since, Limit: 100},
	choir.Hop{Rel: choir.Reports, Role: "primary", Limit: 2})
for _, p := range ps.Paths {
	fmt.Println(p[0].To, "->", p[1].To)
}
```

**4. Which stories contradict each other on this happening?**

```go
rs, _ := choir.World.Neighbors(ev, choir.ReportedBy, choir.Opt{})
var ids []choir.ID
for _, r := range rs.Edges {
	cl, _ := choir.World.Neighbors(r.From, choir.Claims, choir.Opt{Limit: 40})
	for _, e := range cl.Edges {
		ids = append(ids, e.To)
	}
}
got, _ := choir.World.Get(ids...)
cls := got.Claims
var qs []choir.Question
for i := range cls {
	for j := i + 1; j < len(cls); j++ {
		if cls[i].Story == cls[j].Story {
			continue
		}
		qs = append(qs, choir.Question{
			ID:      fmt.Sprintf("p%d_%d", i, j),
			Text:    "Do these two statements disagree about a fact of the same occurrence?",
			Choices: []string{"contradict", "agree", "different facts", "cannot tell"},
			About:   []choir.ID{cls[i].ID, cls[j].ID},
		})
	}
}
ans, err := choir.World.Ask(qs...) // one call, every pair
if err != nil {
	fmt.Println(err)
	return
}
var acts []choir.Act
for k, a := range ans.Answers {
	if a.P["contradict"] >= 0.8 {
		acts = append(acts, choir.Act{Op: "contradicts", Subject: qs[k].About[0], Object: qs[k].About[1],
			Receipt: ans.Receipt, Question: a.ID})
	}
}
rec, err := choir.World.Judge(acts...)
fmt.Println(len(acts), "contradictions recorded", rec.Recorded, err)
```

**5. Who are the central entities across the stories citing this outlet this week?**

```go
o, _ := choir.Ref("outlet:reuters.com")
ps, _ := choir.World.Walk(o,
	choir.Hop{Rel: choir.Published, Since: time.Now().AddDate(0, 0, -7), Limit: 200},
	choir.Hop{Rel: choir.CitedBy, Limit: 5},
	choir.Hop{Rel: choir.Mentions, Role: "central", Limit: 12})
stories := map[choir.ID]map[choir.ID]bool{} // entity -> stories
for _, p := range ps.Paths {
	story, ent := p[1].To, p[2].To
	if stories[ent] == nil {
		stories[ent] = map[choir.ID]bool{}
	}
	stories[ent][story] = true
}
type row struct {
	id choir.ID
	n  int
}
var rows []row
for id, s := range stories {
	rows = append(rows, row{id, len(s)})
}
sort.Slice(rows, func(i, j int) bool { return rows[i].n > rows[j].n })
for i := 0; i < len(rows) && i < 10; i++ {
	fmt.Println(rows[i].n, rows[i].id)
}
fmt.Println("complete:", ps.Bound.Complete)
```

**6. Captures like this text that belong to no story.**

```go
h, _ := choir.World.Find(choir.Query{
	Text: passage, Kinds: []string{"capture"}, Uncited: true,
	Since: time.Now().Add(-48 * time.Hour), Limit: 20,
})
for _, x := range h.Hits {
	fmt.Printf("%.3f %s %s %v\n", x.Score, x.ID, x.Title, x.Why)
}
```

**7. Walk from a capture to its happening, to the happening's other captures, to their stories.**

```go
ps, _ := choir.World.Walk(capID,
	choir.Hop{Rel: choir.EventOf},
	choir.Hop{Rel: choir.Members, Role: "member", Limit: 50},
	choir.Hop{Rel: choir.CitedBy, Limit: 5})
for _, p := range ps.Paths {
	fmt.Println(p[0].To, p[1].To, p[2].To, p[2].Prov.Rule)
}
```

**8. Are these two entities the same thing? If so, merge them.**

```go
a, _ := choir.Ref("ent:5d0c2f6e1a9b3c7d8e4f0a12") // "U.S."
b, _ := choir.Ref("ent:9a1b2c3d4e5f60718293a4b5") // "United States"
ans, _ := choir.World.Ask(choir.Question{
	ID: "same", Text: "Do these two entries name the same real thing?",
	Choices: []string{"same", "different", "first is not an entity", "second is not an entity"},
	About:   []choir.ID{a, b},
})
if p := ans.Answers[0].P["same"]; p >= 0.92 {
	rec, err := choir.World.Judge(choir.Act{Op: "merge", Subject: a, Object: b,
		Receipt: ans.Receipt, Question: "same"})
	fmt.Println(rec.Recorded, err)
} else {
	fmt.Println("not merged; p(same) =", p)
}
```

**9. What happened after this happening?** (its follow-up chain)

```go
cur, seen := ev, map[choir.ID]bool{}
for depth := 0; depth < 6 && cur != "" && !seen[cur]; depth++ {
	seen[cur] = true
	nx, _ := choir.World.Neighbors(cur, choir.FollowedBy, choir.Opt{Limit: 5})
	if len(nx.Edges) == 0 {
		break
	}
	er, _ := choir.World.Get(nx.Edges[0].To)
	fmt.Println(depth, er.Events[0].FirstSeen.Format("01-02 15:04"), er.Events[0].Headline)
	cur = nx.Edges[0].To
}
```

**10. Do we already hold two independent outlets on this happening, so the
web is not needed?**

```go
er, _ := choir.World.Get(ev)
e := er.Events[0]
if e.Origins >= 2 {
	ms, _ := choir.World.Neighbors(e.ID, choir.Members, choir.Opt{Role: "member", Limit: 50})
	for _, m := range ms.Edges {
		fmt.Println("held:", m.From) // read these with ReadCapture; no web search for a second source
	}
} else {
	fmt.Println("one origin held:", e.Outlets, "- search the web for what is new since", e.LastSeen.Format(time.RFC3339))
}
```

### C5. Custody and privacy, enforced by the surface

- **Owner and computer.** Every read runs as the run's owner and computer.
  Corpusd returns public objects (the paper's, publishers') and the owner's
  own objects, and nothing else. A private computer's objects never reach
  another owner's run. This is the capture rule today (**C**, survey section
  5), applied to every kind.
- **Labels.** Each returned object carries its labels. The host adds their
  union to the run's label set, as `SearchWireCorpus` does (**C**,
  `types/labels.go:8-25`: "a derived value carries the union of its inputs'
  labels"). The disclosure gate then treats the run as it does today; the
  surface removes nothing from it.
- **Grants.** `semantic.read` for research, management, and Texture with
  every read forced to `Within` its own story's happenings (Texture "reads the
  captures its frame names; it never fetches the world", **C**,
  `manifest.go:367-370`). `semantic.judge` for research only. `Ask` keeps
  `system1.call`, research only.
- **People.** A person entity is public when it has a Wikidata id or is
  central in two public stories (the E2 rule, **C**, current-architecture "E2"
  row). For a person who is not public, `Neighbors(person, MentionedIn, …)`
  answers only with `Opt.Within` set to a happening or a story, and refuses
  otherwise with the reason. That is a correctness refusal, not a switch: it
  stops any desk, or any page built on the surface, from assembling a dossier
  on a private person across stories. The public entity page keeps the same
  rule.

### C6. What the surface costs a day

Reads cost no model call. Their load is index reads on tables that hold 2.3
million objects and 1.7 million edges (**M**, survey 6.1). The new indexes are
`og_edges (from_id, kind)` for forward hops, and `proj_capture_index (outlet_id,
fetched_at)` for the outlet hop, both built concurrently (as the E2 index is,
**C**, current-architecture "E2" row). `Ask` costs one Jev call per use. How
many a research run makes is unknown (**U**); the owner rule is to fix a caller
that asks too often, not to cap it.

---

## D. What it makes possible

### D1. Consolidating overlapping stories, end to end

What happens to a polled capture after L5:

1. **Arrival.** A source poll writes captures. The embed worker is woken
   (landed, E1). The flush timer arms.
2. **The flush.** One call resolves each capture onto a happening, its
   entities, and merge questions (B2). The `semantic.flush` record is written;
   the rows are exported.
3. **The route, by rule, as management's disposition.**
   - The capture is a counted member (p at least the routing bar) of a
     happening H, or of a development of H (`after_H`), and a live story S
     reports H as primary. "Live" means S was revised in the last 72 hours,
     published or not. Then the host writes a `choir.capture_disposition` of
     kind feed to S, as management's rule, and delivers the capture to S's
     Texture inbox as a feed. Management's task does not see it as an open
     candidate; its records list it as "routed to S by rule".
   - The capture is tentative on such an H (0.5 to the bar). Management's
     task gets it with "likely story S (p)" beside it, and decides.
   - Otherwise (a new happening, or one no live story reports): management
     decides open, feed or decline, as today.
4. **The open.** When management opens a story from a capture, the host
   writes `reports_event` (role primary, basis `opened`) from the opening
   capture's happening. So a story is linked to its happening from its first
   minute, published or not. That matters: the earlier story was unpublished
   in 39% of the duplicate pairs (**M**, duplicates file 4.2).
5. **The publish.** At each publish the host recomputes `reports_event` with
   basis `cited` by the E5 rule (two cited captures or 30% are counted
   members; primary has the most) (**C**, E5 lane `story_events_source.go`).
   An `opened` primary that the cited rule disagrees with stays, as
   `secondary`, until Texture's next version.
6. **Texture decides.** A fed capture is evidence. Texture uses it in the next
   version or marks it unused with a reason (I15). Either way the routing
   forecast is settled (B6).

**Developments.** A capture whose answer is `after_H` (a development) routes to
H's story only when that story was revised in the last 24 hours; otherwise it
opens a new story, linked by `follows`. Follow-ups sit a median 26.6 hours
after the earlier story (**M**, duplicates file 2), and a real development
can deserve its own headline (Huang resigned; the Senate passed the bill,
same file, option 4's risk). This window is the owner's call (F3, decision
2).

**What sets the routing threshold.** Three things, in order:

1. **The labelled pairs.** The duplicates file's appendix gives 238 later and
   earlier story pairs (171 duplicates, 26 clear and 41 borderline follow-ups)
   and 744 distinct stories (**M**). Replaying their opening captures through
   the L2 flush's recorded distributions gives precision and recall of
   routing per probability band before anything routes.
2. **The cost asymmetry.** A wrong feed costs Texture one turn and is visible
   as a marked-unused source. A duplicate costs about 3.1 searches, a research
   run of about 1.0 million input tokens, and, in 53% of cases, a repeat on the
   public page (**M**, duplicates file 2 and 3). So the bar can sit where
   routing precision is 0.9, not 0.99.
3. **The audit and Texture's verdicts** after it ships, by band (B6).

It starts at 0.92 (the join bar) and moves only on those numbers.

**What it saves.** Of the day's 3,557 web searches, 528 went to duplicates and
86 to clear follow-ups (**M**, duplicates file 3). The file's own estimate of
the saving is 7% to 21%, most likely 10% to 15% (**M**-based). This design
routes only captures whose happening is joined, so its reach depends on the
join rate. Until L2 and L4 lift it, L1 (management's page by time) carries most
of the gain: it raises the page's reach from 6 to 68 of 171 duplicates (**M**,
duplicates file 4.2), at zero model calls.

### D2. Duplicates already published

At each publish, corpusd checks whether another published story from the last
72 hours shares the new version's primary happening. When one does, the host
writes a record to the **newer** story's Texture desk: "story S reports the
same happening; fold your findings into S, or keep". Texture decides, because
it is the custodian of its document. On fold, the newer story's route answers
"continued in S" and leaves the front; its findings go to S's desk as a cast
with its captures, the path management tasks already use. Merging published
text is not built. The front shows at most one story per primary happening in a
window; the others appear as its related stories.

### D3. Only search for what is new to us

Base: 3,557 web searches in the 24 hours to 22:18Z (**M**, duplicates file 3);
the gateway's counter read 2,764 by 20:26Z (**E**, research-runs file 7). 90%
of research runs and 94% of searches were a story's first run (**E**, same).
The owner's target is under 1,000.

The happening state makes three tests lookups:

1. **Second outlet held.** A story publishes only with citations from two
   distinct outlets (**C**, `story_publish.go:263`), and 34 of 41 research
   objectives asked for a second source of one captured claim (**E**,
   [corpus-first-search-one-verb-2026-11-19.md](../definitions/corpus-first-search-one-verb-2026-11-19.md),
   its 2026-11-20 note). When the story's primary happening already has two or
   more origins among held captures, the brief says so with the capture ids,
   and the one verb holds sends for that objective unless the run gives a
   reason. Today 14 of 300 events have two or more intake members (**E**,
   research-runs file 3), so this test fires rarely now; it grows with the
   join rate.
2. **Nothing new since the last run.** For a repeat run, the brief states the
   happening's intake members newer than the story's last research run
   (intake only: a research read is not news). Repeats are 9.5% of runs at
   most (**E**), so this is small.
3. **The story should not exist.** Consolidation (D1) removes the first run of
   a duplicate story entirely.

| Lever | Searches a day removed | Basis |
|---|---|---|
| Consolidation (L1, L5) | 350 to 530 | 10% to 15% of 3,557 (**M**-based, duplicates file) |
| Second outlet held at open (L7) | 300 to 1,300 | **A**: 10% to 40% of the about 3,300 first-run searches, depending on how many new stories open with two origins once joins work; unmeasured |
| Stop when the objective holds two outlets, during the run (one-verb landing 3) | 200 to 500 | that note's estimate (**A**) |
| Repeats with nothing new | 0 to 180 | research-runs file 7 (**E**) |

The levers overlap, so their sum is not valid. My central guess is that they
leave 1,500 to 2,300 searches a day (**A**). **Under 1,000 needs the
second-outlet test to fire on about half of new stories**, which needs a join
rate this design aims for but has not shown. The precommit C-S6 states it.

**What consolidation alone leaves.** Duplicates and clear follow-ups took 614
of 3,557 searches (17.3%, **M**, duplicates file 3). Removing all of them
would leave about 2,900. At the realistic 10% to 15% it leaves about 2,800 to
3,200 a day. So consolidation by itself does not reach 1,000, and this design
does not claim it does.

**Where the rest would have to come from.** It would come from the first run of
a story that is not a duplicate. That run is about 94% of searches (**E**). It
would have to start from what the corpus already holds about the happening:
its members, their outlets and origins, and what is new since the last run.
Today it starts from the opening capture alone and searches the web for a
second outlet. The levers that do this are L7 (the brief carries the happening
state) and the one verb's stop-at-need. Both depend on the happening holding
the outlets the corpus really has. That is the join rate, which L2 and L4 aim
to raise. If C-S1 fails (minting stays above 65%), the corpus-first first run
cannot work through happenings. Then the design does not reach 1,000. The
fallback would be the one verb's own corpus retrieval by vector and words,
with no happening.

### D4. Related coverage

The related answer's `basis.events` is always 0 today (**M**, survey item 1).
After L3, related stories are: same primary happening; happenings linked by
`follows`; shared central entities (after merges resolve); shared cited
captures. The walk that exists is extended (`publication_related.go:593`), not
rewritten. Autoradio tells a follow-up when a candidate's happening `follows`
a heard story's happening.

### D5. Entity pages, and the rule for private people

The public entity read exists (E2). After L4 it gains capture-side mentions
and resolved merges. The rule for people stays: a person page is served only
with a Wikidata id or as central in two public stories (C5). A private person
(a pedestrian struck, a victim) is named only inside the story that names
them, never on an aggregating page, and an agent's read of their mentions is
scoped to one happening or story. Organizations and places are always served.

### D6. Corrections

A correction is a later version whose claim supersedes an earlier claim.
What the layer adds: when a capture joins a happening that a live story
reports, the flush can ask, in the same call, whether the capture contradicts
any of that story's claims (one question per claim of the reporting story).
At a high bar, the host writes a record to the story's Texture: "capture X on
your happening appears to contradict your claim Y". Texture decides. This is
L11, last, because the question's accuracy is unmeasured and claims per story
make it the largest token increase in the plan (**A**: 10 to 30 claims a
reporting story).

---

## E. The path from here

### E1. Kept, replaced, deleted

**Kept:** the registry, write gate, custody derivation and id shape
(`objectgraph`); corpusd as row custodian; captures, publications, citations,
checks and support judgements; `choir.entity` rows and ids, `mentions_entity`
edges, the lexicon and its learning loop; the E2 ready list and candidate
lookups; the vector index and ranked answer; the event layer's definition, its
three probability bands and its wire-origin rule; the System 1 kernel service
and its tape records; the E5 lane's routes, edges, public event read,
consolidation read and collation test.

**Replaced:** the event layer's memory queue and capped index (by the ready
list and corpusd); events and memberships on the computer (by the Autopaper
object graph); immutable memberships (by superseding edges); the intake
labeller's event and topic call (by the flush); the event substring search in
research (by `Find`); the two outlet functions (by one key and merge acts);
the 12-question System 1 cap and the story analysis's two calls (by one call);
management's newest-50 page (by a 72-hour page by time, then by retrieval);
`choir.event_projection` (by the event itself).

**Deleted:** the setting `research.related_story` (a switch, **C**,
`store/computer_setting.go:190`) and the environment switches
`CORPUSD_RELATED`, `CORPUSD_RELATED_STORY`, `CORPUSD_RELATED_ENTITIES` (**C**,
`publication_related.go:60-64`); `RelateStories`, the `related_story` and
`relies_on` edges and the `story.relation` record's export; the kinds
`choir.event_membership`, `choir.capture_label`, `choir.capture_label_override`;
`intakeOverlapPicks`; the related walk's title-word filler. Reviewed for
deletion: the eight provenance edges no one reads.

**Side tables with no custodian entry.** `vec_*`, `proj_capture_*` and the
lexicon are derived indexes under I27's named exception and stay so, with
corpusd as their only writer. `search_leads` is not derivable (it holds
provider results kept nowhere else), so under I27 it is a home without a kind.
It carries no semantic fact and this design leaves it; it is named as residual
R6 for the owner.

### E2. The landings

Review tiers by the contract: orange one seat (seat 2); red two seats (seat 1, a
Claude Sonnet subagent that did not write the change, plus seat 2). Drills on d@e only; the paper changes only by deploy. Every
non-docs landing carries `Ring-Expedite`.

| # | Landing | Class | Review | Proof on staging | Rollback | Model calls and money a day |
|---|---|---|---|---|---|---|
| **L1** | **Management's page by time** (station C1b, already being built ahead of this design; this design depends on it and does not redo it). Today the page is the newest 50 stories, about 27 minutes back, though the prompt promises 8 hours (**C**, `agentcore/management_tasks.go:68, 73, 107, 153, 157`). The target is the stories updated in the last 72 hours, headline and id only (about 30 tokens a story, **A**), inside the 20,000-token bound. If C1b ships a shorter window, L5's routing covers the rest | orange | one seat | `management story choice` lines show the page reaching 72 hours; the duplicates method re-run on the next day's opens (C-S3) | revert | 0 calls; about 5,000 more input tokens per management task (**A**) |
| **L2** | **Flush provenance and reach.** The ready list replaces the memory queue; `proj_capture_resolution` written; the `semantic.flush` tape record on channel `semantic:<computer>`; candidates from every distinct event among 50 vector neighbours; same-flush siblings clustered and listed first, banded like events; the memory index and its caps deleted. Starts tomorrow, parallel to L1 | red (tape, provider call content) | two seats | `op=event_flush` lines over a day (C-S1); a hand read of 50 mints through the record by a reader who did not build it (C-S2); a crash drill on d@e shows unresolved captures answered by the next flush | revert; the record is inert without the code | same call count; about 30% more tokens a call (**A**), under $0.10 a day |
| **L3** | **Happenings move to the Autopaper object graph** (absorbs the E5 lane). A corpusd commit route for flush results; `choir.world_event` re-registered under corpusd; `in_event` edges with role and record; `follows` as an edge; the paper's events migrated (E5); the VM kinds and `choir.event_membership` deleted; custody of judgments to research; `reports_event` at open and at publish; the public event read, consolidation read and `basis.events` | red (custody, export, tape, public projection) | two seats | on d@e a polled capture's event readable at `/api/platform/events/{id}`; on the paper, share of new stories with a `reports_event` edge (C-S8); migration counts equal the paper's old totals | revert; the cleanup script tombstones exported rows (E5 lane's `scripts/cleanup-story-events`); the paper's state.db rows are tombstoned, not deleted, until the next landing | 0 calls |
| **L4** | **Entities and merges in the flush** (absorbs E3, E4's flush half). Mentions from captures; outlet entities and the one outlet key; the qualified-id rule; merge and retire questions ride the flush; `choir.ontology_act`; the intake labeller's call deleted, its questions absorbed | red (tape, provider calls, export) | two seats | C-S7 (entity links), C-S9 (Jev calls fall); a d@e drill of a forced merge and its undo read back publicly | revert; tombstone capture-sourced edges and acts (re-export restores them) | calls fall (one call replaces two per poll); about 12 million more tokens, about $0.50 (**A**, entity design 8) |
| **L5** | **Routing and published duplicates.** Counted members of a happening a live story reports are fed by management's rule; tentatives named in management's records; the published-duplicate record to the newer story's Texture; front one story per primary happening | red (crosses management dispositions and Texture's inbox) | two seats | C-S4 (second-or-later share of opens), C-S5 (Texture uses routed feeds); stories first published a day does not fall over 10% | revert | 0 calls |
| **L6** | **`choir.World` reads** for research, management, and Texture scoped; labels; the person rule; the event substring search deleted | orange | one seat | a d@e research run answering questions 1, 2, 6 and 7 of C4; the person refusal; recall at ten of the 200-question set does not fall | revert | 0 calls |
| **L7** | **The brief and the one verb read the happening state** (changes search station L3): origins held and new intake since the last run | orange; red for the held-run path | one seat; two for the held run | C-S6 (searches a day); stories first published a day does not fall over 10% | revert | 0 calls |
| **L8** | **`Ask` and `Judge`.** No 12-question cap; the story analysis in one call; ontology acts by research; owner undo; the daily audit run | red (tape) | two seats | a d@e drill of each act and its undo; the first day's audit settles the flush's forecasts by band | revert; acts are superseded, not deleted | `Ask` per use; the story analysis halves its calls; one audit run a day (**A**: about 1 million input tokens, from the 1.0 million mean of a research run, duplicates file 3) |
| **L9** | **Deletions**: `research.related_story` with its finish-rule fix (E6), the three `CORPUSD_RELATED*` switches, `RelateStories`, `related_story`, `relies_on`, the provenance edges once the read shows no reader | red (desk finish rule) for the setting; orange for the rest | two seats; one | on d@e a story run records entities with no setting | revert | 0 calls |
| **L10** | **Backfill**: entity questions for old captures ride the ready list (E7) | orange | one seat | resolved count reaches the capture total in the growth read; gateway call counts | revert; links stay valid | about 5,600 calls once, about $13 (**A**, entity design 9) |
| **L11** | **Corrections**: contradiction questions for captures joining a reported happening | red (provider call content, Texture inbox) | two seats | a hand read of 50 flagged contradictions | revert | **A**: 10 to 30 more questions a routed capture; cost unknown until L2's records show routed volume |

The first three to build are L1, L2 and L3. L1 and L2 can start tomorrow in
parallel; L3 waits for L2's record so its migration carries provenance
forward.

### E3. Today's stations: absorbed, changed or dropped

| Station | Fate |
|---|---|
| E3 (captures get entity ids in the flush; stopped today) | absorbed into L4, changed: rows go to the corpusd home; its `capture.entities` tape record becomes part of `semantic.flush`; `choir.entity_merge` becomes `choir.ontology_act` |
| E4 (merges and retirements) | split: merges that ride the flush go in L4; research acts, splits and the owner undo in L8 |
| E5 and the lane `story-events-e5` | absorbed into L3; see E4 below |
| E6 (remove `research.related_story`) | L9, unchanged in substance |
| E7 (backfill) | L10, unchanged |
| E8 (corpus search expands an entity) | absorbed into `Find` (L6): an entity filter and the entity's merged ids |
| Search station L3 (the one verb) | changed: kept as designed, and it reads the happening state from L7 |
| Corpus-first S9 (neighbourhood read for management), S10 (fold at first publish), S11, S12 | S9 and S12 into L5; S10 replaced by routing before research (L5) and the published-duplicate record (D2); S11 into L4 and L8 |

### E4. The `story-events-e5` lane: does its shape survive?

The lane holds one unpushed commit, `6dcb04e4f` (**M**, read with `git show
--stat`): 28 files, 2,852 lines, red. It writes `choir.event_projection`,
`in_event` and `reports_event` at publish, by the rule "two cited captures or
30% are counted members", with no model call; it serves a public event read and
a consolidation read; it pins the non-C collation in a test.

**Ruling: change it; do not land it as it stands, and do not replace it.** Most
of its shape survives. The edges, the deterministic rule, the routes, the
public read, the consolidation read, the collation test and the cleanup script
are what L3 needs. Its weak point is where it reads events from. It derives a
story's events from the event layer's in-memory index, which holds at most
10,000 events and 20,000 memberships (**C**, `event_layer.go:85-86`). The paper
already has about 10,153 events (**E**), so the oldest are already dropped. A
story that cites an older capture would silently get no event. After L3 the
lane reads the corpusd rows instead, and the index is deleted (L2). Five
changes:

0. **Read events from corpusd, not from the index in memory.** This is the
   cap above.

1. **No projection kind.** The event in corpusd is the event, not a copy:
   `choir.event_projection` becomes `choir.world_event` under corpusd, which
   needs the VM registration removed in the same landing. So the lane should
   not land alone; it lands as the corpusd half of L3.
2. **Link at open, not only at publish.** `reports_event` with basis `opened`
   when management opens a story, so unpublished stories (39% of the earlier
   partners in duplicate pairs, **M**) can be matched.
3. **Memberships are judgments that can change.** The lane's commit message
   says "memberships are immutable and are the authority". After L2 and L4 a
   membership can be superseded by a merge or a re-judgment, so the edge carries
   its record id and reads resolve merges.
4. **A tape record, not none.** The lane follows the entity design's "no tape
   record for events". The judgments now have one (`semantic.flush`), so the
   exported edges cite it. The publish-time derivation itself stays
   record-free, because it is deterministic from facts already recorded.

If the owner wants story-event links before L2 lands, the lane can land as
written; the cost is a second red migration of its projection rows in L3 (a
few thousand rows, **A**). I would not take it: its coverage is low while 84%
of questions mint (its own claim C-E7 says so).

### E5. Migrating 7,000 entities and 10,000 events without a risky rewrite

- **Entities: no rewrite.** Ids stay. Merges, retirements and splits are new
  `choir.ontology_act` rows layered on top, resolved at read. The 23,966
  `mentions_entity` edges are untouched (**M**, survey 6.1). Capture-side
  mentions are new rows written by the flush and the L10 backfill.
- **Events: one import that empties the old home.** At L3's deploy the paper's
  flush loop, on its first run, reads its own `choir.world_event` and
  `choir.event_membership` objects (about 10,153 and their memberships, **E**)
  and sends them through the same corpusd commit route as new judgments, in
  batches by exact id lists, under a `choir.migration_marker` so it runs once.
  Ids are deterministic, so a repeat writes nothing. The old local rows are
  tombstoned only after corpusd's counts match. No boot path resumes work: the
  import is a migration step, not a resumed obligation.
- **No range predicates on ids.** `og_edges.from_id` and `to_id` use the
  database collation (en_US.UTF-8) while `og_objects.canonical_id` is C, and a
  byte-prefix range matched nothing in production today (**M**, survey 1.2;
  `docs/problems/entity-read-lists-no-stories-in-production-2026-11-20.md`).
  Every read and every migration step uses equality on exact id lists or
  `starts_with()`. Each landing carries a test that moves the id columns to a
  non-C collation, as the E5 lane does.
- **Large tables change by adding, never by rewriting.** `proj_capture_index`
  (237,050 rows, **M**) gains a nullable `outlet_id` column filled in batches,
  then an index built concurrently. `og_edges` gains one index built
  concurrently. No column changes type.

### E6. Cost per day, all landings

Model calls: fewer than today after L4 (the intake labeller's call per poll
is gone). Tokens: about 12 to 15 million more a day (**A**), about $0.50 to
$0.70 at the derived price. One audit run a day. One backfill of about $13
(**A**). No new model, no new service, no new database.

---

## F. Honest limits

### F1. What I could not verify

- **Events in production.** Like the survey, I read no event: the store is the
  paper's and the route needs its key. Every event number is **E**, from
  records of 05:26Z and 19:40Z to 20:18Z.
- **The join rate since the embed kick (E1).** Unmeasured. If E1 already
  lifted joins a lot, L2's gain is smaller than B3 implies.
- **Whether `intake.labels` and `research.related_story` are on for the
  paper.** Inferred by the survey, not read.
- **Whether Texture's mark-unused reason can be read as "wrong story".** B6's
  first teacher depends on it.
- **Whether the eight provenance edges have an outside reader.**
- **Token and cost figures** are estimates from SYSTEM1's figures and the
  entity design, not measurements.
- **The cost of `Walk` at four hops on production data.** The bounds are
  designed; the latency is not measured.
- **Whether yaegi handles the struct types as written.** `choir.Searches`
  returns typed values through a handle, so the pattern exists (**C**,
  `yaegikernel/choir.go:457`); the exact types are untested.

### F2. Where I am guessing

- That widening the shortlist and clustering siblings lifts joins enough for
  routing to matter (C-S1 states it with a number).
- That 72 hours and 24 hours are the right windows for live stories and
  developments (the duplicates file's 72-hour lookback and its follow-up
  gaps are the only basis).
- That the second-outlet test can fire for a large share of new stories once
  joins work. Nothing measures it today.
- That research is the right judgment authority rather than a new desk. I chose
  it because it adds nothing and fits I26's words; a panel might disagree.

### F3. The owner's decisions, each with the default I would take

1. **Happenings move to the Autopaper object graph, and research becomes the
   judgment authority for happenings, memberships, entity links and ontology
   acts; management keeps dispositions.** Default: yes.
2. **A development of a happening that a live story reports becomes a new
   version of that story if the story was revised in the last 24 hours;
   otherwise a new story linked by `follows`.** Default: yes, with those
   windows, revisited after a week of L5 data.
3. **Arrival waits for the flush (about one to two minutes) so the route knows
   the happening.** Default: yes.
4. **Captures of a reported happening are fed by rule, without a management
   model turn.** Default: yes, for counted members only; tentatives go to
   management.
5. **A published duplicate is marked "continued in" and leaves the front;
   published text is never merged automatically.** Default: yes.
6. **People: public page only with a Wikidata id or central in two public
   stories; agent reads of a private person scoped to one happening or
   story.** Default: yes.
7. **Other computers mint happenings and entities in their own owner's scope
   and read the paper's as candidates; nothing crosses owners.** Default: yes.
8. **Spending: about $0.50 to $0.70 a day more and about $13 once, with no
   grant.** Default: proceed, and report the day's cost in the status read.
9. **The audit is run by research.** Default: yes; the alternative is a
   management task, if the owner reads rule 1b as forbidding the same desk.
10. **`search_leads` stays a table without a kind (residual R6).** Default:
    leave it; it carries no semantic fact.

### F4. Claims worth precommitting

Ids are provisional; the ledger assigns them at entry. Each outcome is a count
or a hand read by someone other than the forecaster.

| Id | Claim | p | Test | Settle by |
|---|---|---|---|---|
| C-S1 | After L2, intake questions that mint a new event fall from 84% to at most 65% over a day | 0.55 | `op=event_flush` lines, intake only | L2 deploy + 1 day |
| C-S2 | After L2, of 50 hand-read mints, at most 5 should have joined (from 10 of 50) | 0.50 | hand read through the `semantic.flush` record by a reader who did not build L2 | L2 deploy + 2 days |
| C-S3 | After L1, true duplicates fall from 17.4% to at most 12% of a day's opened stories | 0.45 | the duplicates file's method re-run on one day | L1 deploy + 2 days |
| C-S4 | After L5, second-or-later stories on a happening fall from 20.1% to at most 10% of opens, and stories first published a day does not fall over 10% | 0.40 | the same method; publication counts | L5 deploy + 3 days |
| C-S5 | Of 100 rule-routed feeds after L5, Texture uses at least 75 | 0.50 | the stories' revisions and source marks | L5 deploy + 3 days |
| C-S6 | After L5 and L7, web searches a day fall to at most 2,400; to under 1,000 | 0.50; 0.10 | gateway `search request` lines | L7 deploy + 7 days |
| C-S7 | After L4, at least 70% of resolved intake captures carry a counted entity link, and at least 90 of 100 sampled links are right | 0.55 | growth read; hand read | L4 deploy + 2 days |
| C-S8 | After L3, at least 80% of stories opened in a day carry a `reports_event` edge | 0.60 | public story reads | L3 deploy + 1 day |
| C-S9 | After L4, Jev calls of the sites `intake_labels` and `event_layer` together fall by at least 30% at equal captures | 0.60 | gateway site counts, a day before and after | L4 deploy + 1 day |

### F5. What would falsify the design

- **Happenings stay singletons.** If after L2 and L4, with vectors present,
  more than 70% of intake questions still mint, happening-keyed routing will
  not carry consolidation. Then the open decision should be a retrieval of
  candidate stories (story and opening-capture vectors, entities, 72 hours)
  judged by System 1 at open, the duplicates file's option 2, and happenings
  stay for corroboration only.
- **Texture rejects the routes.** If Texture marks more than 40% of routed
  feeds unused as the wrong story, the grain (occurrence, not storyline) or
  the bar is wrong.
- **Agents do not use the surface.** If a week of research ledgers after L6
  shows `choir.World` in under 10% of runs, the library is the wrong shape for
  how the agents work, and the brief should carry the answers instead.
- **The second outlet does not stop searches.** If briefs that show two held
  origins are followed by web searches for a second source at the same rate
  as briefs that do not, the search hypothesis in D3 is false.

### F6. Residuals

R1, the restore gap (A8). R2, two new same-name entities in one flush are
joined (entity design 4.3), undone by a split. R3, OpenRouter's long-context
line above about 32,000 bytes may change answers (SYSTEM1 section 10). R4, a
private computer's objects are never compared with another owner's. R5, a
podcast clip names its story by a route string until a `clip_of` edge exists.
R6, `search_leads` is a home without a kind.
