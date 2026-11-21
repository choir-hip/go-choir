# The semantic layer: one graph of stories, happenings, entities and claims (design, 2026-11-20)

**Status: a proposal, not ratified.** Written 2026-11-20 between 22:30Z and
24:00Z by a design agent (Opus, no code, no production access) for the owner's
request of the same day. Nothing here is built unless
[current-architecture.md](../current-architecture.md) says so. It replaces the
plan of [entity-layer-2026-11-20.md](entity-layer-2026-11-20.md) from station
E3 on, and the graph half of
[corpus-first-retrieval-and-research-2026-11-18.md](corpus-first-retrieval-and-research-2026-11-18.md).
Owner decisions with defaults are in section F3; claims to precommit in F4.

**Rulings worked in (amended 2026-11-21).** The owner's rulings of
2026-11-20 23:00Z and 23:05Z (a story is one main point; material on the same
main point revises it, a changed main point starts a new story, judged by
reasoning: section D1), his word against over-reading prior art and for RLM
agents with System 1 as a tool (B1, D1), and his efficiency goal for search
credits (C7, D3). Also the lead's readings of the paper after the stopgap C1b
landed (d8367e2c8 with repairs; L1 in E2).

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
whose key is its web domain. A **story** is one **main point** (a recorded,
revisable sentence saying what the story is for); it reports the happenings
that serve that point and is about entities; when the main point changes, a
new story begins that `succeeds` it. Its **claims** are the cited sentences
the publish path already checks against their captures. Every one of these is an object in the
Autopaper object graph, so one SQL statement can walk from a capture to its
happening to the other stories on it, with vectors in the same statement.
The judgments are records on the deciding computer's tape; the graph rows are
their projection, and any change (a merge, a split, a retirement) is a new
record that supersedes the old one and can itself be undone.

**What changes for the paper.** The paper opened 982 real stories in one day
(988 less six test documents). Of them, 171 (17.4%) were a true duplicate of a story opened in the 72 hours before;
197 (20.1%) were the second or later story on a happening (**M**,
[duplicate-stories-on-the-paper-2026-11-20.md](../evidence/duplicate-stories-on-the-paper-2026-11-20.md)
section 2). It happened because nothing checked: management's model was shown
the 50 most recently updated stories, which reach back a median 27 minutes,
while the earlier story was over 8 hours old for half the duplicates (**M**,
same file, section 4). With this layer, every capture a management task holds
gets one reasoned judgment: does it serve the main point of a story the paper
holds (revise that story), has that story's main point moved on (open the
story that succeeds it), or is it a different matter (open a new one)? The
judge is shown the candidate stories found by meaning, entities and
happenings, each with its recorded main point, plus the whole recent page;
System 1 gives a first answer in the flush's one call; the management task
decides and records why. A research run reads what the paper holds about its
story before it sends a web search, and the search records what it read.
Agents get a typed Go library in their cell (`choir.World`) to ask these
questions themselves.

**What the stopgap showed.** L1 (station C1b) landed on 2026-11-21: the
open-or-feed decision now sees every headline of the last 8 hours (about 470
of 490) instead of the newest 50. Feeds rose from 51% of decisions (73 of
142) to about 63% (93 of 147) in its first three hours. Its second part, an
offer of held look-alikes by rare title words with a required verdict, fired
on 2 or 3 of about 37 opens, and every verdict was "different" (**E**, the
lead's console reading). So showing the judge the whole page moved the
decision, and matching title words carried almost no identity. Candidates come
from vectors, entities and happenings (D1), not words.

**What I would build first.** **L2** (red): the flush records what it was
shown and what it answered on the tape, reads its work from the ready list,
and widens its shortlist. In parallel, the first half of **L5** (red): the
story judgment with a recorded main point, candidates by vectors and entities,
replacing C1b's title-word offer. Then **L3** moves the happenings into the
Autopaper object graph and links every story to its happenings, which adds
happening candidates to L5. It absorbs the `story-events-e5` lane, changed
(section E4).

**What it will not do by itself.** Consolidation (revising instead of opening)
removes at most about 17% of web searches, the measured share of duplicates
and follow-ups, and realistically 7% to 16% (D3). From about 3,200 searches a
day that leaves about 2,800. The road to under 1,000 runs through the second
outlet: 90% of research runs are a story's first run (**E**,
[research-runs-with-nothing-new-2026-11-20.md](../evidence/research-runs-with-nothing-new-2026-11-20.md)),
and most first-run searches look for a second outlet for the opening claim.
The read before a search (C7) makes "do we already hold a second outlet on this
main point" a fact on the tape. It reaches 1,000 only if the answer is yes for
60% to 70% of new stories when their first run starts; today it is about 5%.
Section D3 says what would close that gap.

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
   each new capture: its happening, its entities, and a first answer to which
   held story's main point it serves (advice to the management task, which
   decides). It replaces the intake labeller's separate call over the same
   captures. One read (`Held`, C7) serves the research run before a search,
   the record of that search, and the brief. One record kind (`choir.ontology_act`) carries
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
- **Its main point.** One sentence saying what the story is for, at the level
  of the beat ("Who won the election and by how much", not "the election").
  It is a field of each Texture revision, so every wording is recorded with
  its author and time, and the next story judgment reads the newest (D1).
- **Relation to earlier stories.** Same main point: there is no second story;
  the material revised the first. A changed main point: the new story
  `succeeds` the earlier one (a story-to-story edge written with the judgment,
  D1). Two stories may share a happening (a strike on Ukraine and the missiles
  that landed in Poland during it are two main points over one occurrence), so
  a shared happening is a candidate, not a verdict. Background: a `secondary`
  report of an older happening. A correction: a later version whose claim
  supersedes the earlier version's claim (versions already carry
  `supersedes`, 25,557 rows, **M**).

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
  editorial view over happenings, bounded by its main point, not by a
  happening. It may report several happenings that serve one point (each
  county's call on election night), and a chain of stories linked by
  `succeeds` is the storyline (before the vote, the count, the winner). A
  separate storyline kind would be a second home for what the chain already
  is.
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
| `succeeds` | publication, earlier publication | `{"record": <disposition id>}` | management's story judgment (D1) | **new** |
| `part_of` | happening, parent happening | `{"record": <tape id>}` | Judge only | **new** |
| `mentions_entity` | publication or capture, entity | `{"role": central\|incidental\|tentative}` | story export; flush export for captures | kept; capture side new |
| `contradicts` | claim, claim | `{"record": <tape id>}` | Judge only | **new** |
| `supersedes` | version, version | as today | publish | kept |

Deleted edges: `related_story` (4 rows; its relations `same_incident`,
`later_development`, `same_subject` are a shared happening, `succeeds` (or no
second story at all), and a shared central entity), `relies_on` (1 row; "ranks nothing and no desk sees
it", **C**, `registry.go:357-359`). The eight provenance edges written and
never read (`was_derived_from`, `was_associated_with`, `attested`,
`granted_consent`, `authored_review`, `derived_from_proposal`, `owns`,
`has_agent`, 22 to 36,461 rows each, **M**) are listed for deletion only after
a read of whether any outside contract (the publication bundle, a verifier)
parses them (**U**).

Net: one new object kind, five new edge kinds; three object kinds and two edge
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
  management's (**C**, `manifest.go:360-362`). Deciding which story's main
  point a capture serves is a disposition, so it is management's kind, made by
  management's task by reasoning (section D1). That is the coherence role I26
  gives management. The main point itself is part of the story, so its
  custodian is Texture, which holds the document.

This moves no authority to a new desk and adds no desk. It is a custody change
of two kinds (red, in L3), and the owner's to ratify (F3, decision 1).

### A8. Canonical and derived, and what records each judgment

| Thing | Canonical or derived | Record that made it |
|---|---|---|
| a capture, a publication | canonical (a fetch, a publish) | the capture route; the publish path |
| a happening's membership, mint, `follows` | **canonical: the judgment** | `semantic.flush` tape record (new, L2), on its own channel `semantic:<computer>` |
| an entity link from a capture | canonical: the judgment | the same `semantic.flush` record (L4) |
| an entity link from a story | canonical: the judgment | `story.entities` (exists) |
| which story a capture serves (same, next, new) and `succeeds` | **canonical: the judgment** | management's `choir.capture_disposition`, extended (D1) |
| a story's main point | canonical: the statement | the Texture revision that carries it (D1) |
| what a search held when it was sent | canonical: the decision | `search.decision` on the run's tape (C7) |
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
| RLM agents (research; management for stories) | hard cases and the ontology itself: split a conflated entity, mint a parent happening, re-type, decide a contradiction, audit a sample; and every story judgment (D1), in the management task that already runs | a research run; the management task's existing turn | `semantic.act`, citing receipts and evidence ids; the story judgment in `choir.capture_disposition` |

The order follows the owner's word (2026-11-20): "We're using RLM. We have a
whole ontology built in" and "we do have the System 1 models as a tool as
well." Reasoning agents make the judgments that need reasoning; System 1 is
the cheap first pass whose answer they read. Prior art is read only for how
such systems failed.

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
   split a new link, with no sweep and no schedule;
4. **the story question** (from L5): for each intake capture, which candidate
   story's main point it serves, with choices `same_<s>` and `next_<s>` per
   candidate story (D1), `with_k<n>` for a sibling capture of the same flush,
   and `new`. This is System 1's first pass; the management task decides.

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
- **offers a story as a candidate to the story judge** (D1): any member or
  tentative (p of at least 0.5, the tentative band of today, **C**,
  `event_layer.go:78-79`). A membership never routes a capture by itself: two
  stories with different main points can report one happening.

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

1. **Texture, on every feed.** When management judges that a capture serves a
   story's main point, Texture either uses it (cites it, or revises from it)
   or marks it unused with a reason (I15: every source is cited, toolbar or
   marked unused, **C**, `choir-doctrine.md:333-339`). That is an outcome
   neither the flush nor management can write. Each feed settles management's
   story forecast and the flush's story answer true or false. L5 adds two
   reason codes, `not this main point` and `main point moved` (D1); whether
   today's reasons can be read that way is **U**.
2. **An audit sample, judged by a research run.** A uniform 3% of each day's
   flush decisions (memberships, mints, entity links) and of management's
   story judgments, seeded so anyone can
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

A typed Go library in the cell: one value `choir.World` with seven methods, in
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
	MainPoint       string // the newest wording, from the latest revision
	MainPointProv   Prov   // who wrote that wording, and when
	Succeeds        ID     // the story whose main point this one moved on from
	Events          []Link // reports_event
}

// Held is what the paper holds on a story's main point at one instant,
// read before a web search and recorded with the search's decision (C7).
type Held struct {
	AsOf      time.Time // nothing written after this is counted
	Seq       string    // the corpus sequence the read stopped at
	Story     ID
	MainPoint string
	Events    []ID      // the story's happenings, and its predecessor's
	Captures  []Hit     // cited by the story, counted members, near by meaning
	Outlets   []ID
	Origins   int       // distinct origins: a wire story counts once
	NewSince  []ID      // intake captures newer than the story's last research run
	Prior     []Prior   // this story's earlier searches
	Verdict   string    // "send", "hold: two origins held", "hold: same words sent", "hold: nothing new"
	Bound     Bound
}
type Prior struct {
	Words string
	At    time.Time
	New   int // captures that search brought that the corpus did not hold
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
| `Held` | `Held(story ID, objective string) (Held, error)` | what the paper holds on the story's main point, as of one corpus sequence, with a verdict on whether a web search would bring anything new (C7) | one corpusd request; no model call | corpusd rows; the run's own tape for the decision | `semantic.read`; the host's search path calls the same read itself |

The reads replace the event half of `SearchWireCorpus`, which is an in-memory
substring match over the newest 3,000 events (**C**,
`researchtools/corpus_events.go:54`). `SearchWireCorpus` keeps its capture and
published halves until `Find` has carried a week of research runs, then folds
into it. `Relate` and `RelateStories` stay until `Judge` covers them (L8,
L9).

### C4. Ten questions, as the agent would write them

Helpers an agent defines once in its cell are written inline. Errors are
printed, not hidden.

**1. Which held stories might this capture serve, and what is each one's
main point?** (the story judgment's candidates, D1)

```go
r, err := choir.World.Get(capID)
if err != nil || len(r.Captures) == 0 {
	fmt.Println("no capture:", err, r.Missing)
	return
}
c := r.Captures[0]
cand := map[choir.ID]string{} // story -> how it was found
// by meaning, at any age: the capture's vector against stories
hs, _ := choir.World.Find(choir.Query{Like: c.ID, Kinds: []string{"story"}, Limit: 12})
for _, h := range hs.Hits {
	cand[h.ID] = "meaning"
}
// by happening: stories reporting it, or what it follows
if c.Event != nil {
	er, _ := choir.World.Get(c.Event.To)
	look := []choir.ID{er.Events[0].ID}
	if f := er.Events[0].Follows; f != "" {
		look = append(look, f)
	}
	for _, id := range look {
		rs, _ := choir.World.Neighbors(id, choir.ReportedBy, choir.Opt{})
		for _, e := range rs.Edges {
			cand[e.From] = "happening"
		}
	}
}
var ids []choir.ID
for id := range cand {
	ids = append(ids, id)
}
got, _ := choir.World.Get(ids...)
for _, s := range got.Stories {
	fmt.Println(s.Key, cand[s.ID], s.LatestAt.Format("01-02 15:04"), "|", s.MainPoint, "| succeeds", s.Succeeds)
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

**10. Before searching the web: what do we already hold on this story's main
point?** (the read before a search, C7)

```go
h, err := choir.World.Held(storyID, "a second outlet for the opening claim")
if err != nil {
	fmt.Println(err)
	return
}
fmt.Println(h.Verdict, "origins", h.Origins, "as of", h.Seq, "|", h.MainPoint)
for _, c := range h.Captures {
	fmt.Println("held:", c.ID, c.Why, c.Title) // read these first
}
for _, p := range h.Prior {
	fmt.Println("searched before:", p.Words, p.At.Format("15:04"), "brought", p.New, "new")
}
// choir.Search reads the same thing itself before it sends and records it;
// sending over a hold needs a reason, as `fresh` does today.
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

### C7. The read before a search, and its record

The owner's goal is "the right design for the REPL to be queried so that we
are more efficiently using our external search provider credits." A side
measurement of "did we already hold this" was sent back three times and
stopped, because it rebuilt after the fact what the corpus held at the instant
of a search from tables the search itself was changing
([search-sufficiency-measurement-three-reviews-no-landing-2026-11-20.md](../problems/search-sufficiency-measurement-three-reviews-no-landing-2026-11-20.md)).
So the search path reads what is held once, before it sends, and records what
it read with the decision.

**What the read covers.** For the run's story (every research run belongs to
one: the gateway's `conv=` is the story's document id, **M**, duplicates file
section 3), one corpusd request returns, bounded at a corpus sequence taken
when the read starts:

1. the story's main point, and its predecessor's if it `succeeds` one;
2. the captures the story's versions cite, with outlet and origin;
3. the counted members and tentatives of the story's happenings and of its
   predecessor's (after L3);
4. held captures near the search's words and the main point by meaning and by
   words, from the last 14 days, that no version of this story cites
   (`Find` with `Uncited`). This part does not depend on happening joins, so
   the read works before L2 and L3 lift them;
5. intake captures on the story newer than its last research run;
6. this story's earlier searches: words, time, and how many captures each
   brought that the corpus did not hold (from `search_leads`).

**The verdict.** `send`, or a hold: `two origins held` (the objective is a
second source and items 2 to 4 hold two distinct origins on the main point:
the publish rule's own bar, **C**, `story_publish.go:263`); `same words sent`
(item 6 holds the same words for this story inside the reuse window the
search design already sets); `nothing new` (a repeat run, and item 5 is
empty). The run reads the held captures and may send over a hold with a
reason, as it does today with `fresh`. The need-test's existing holds
(`holds_unread` and the rest, 30% to 40% of attempts, **E**, the goal file)
become verdicts of this one read instead of a second path. A hold is the
caller fixed, not a cap: nothing limits how many searches a run may send for
something the paper does not hold.

**The record.** Each attempt writes `search.decision` on the run's tape: the
story, the objective, the words, the read's corpus sequence and time, the ids
it returned (captures with their origin, events, prior searches), the
verdict, whether it sent, and the run's reason when it sent over a hold. When
results return, `search.result` cites the decision and records how many
result captures the corpus did not hold before the decision's sequence, and
how many new outlets they brought the story. Because the read is bounded at a
sequence taken before the send, and the search writes its own leads after,
a search can never count its own results as held (review 3's finding). So
"what did we hold when we asked" and "was the search worth sending" are facts
on the tape, read by the status script and by the brief, and each
`search.result` settles the run's forecast that the search would bring
something new (the learning engine's outcome, written by the provider, not
the run).

**Cost.** One corpusd request per attempt (about 4,000 to 5,000 attempts a
day counting holds, **A** from 3,200 sends and the 30% to 40% held), no model
call; two small tape records per attempt.

---

## D. What it makes possible

### D1. One main point per story

The owner's rule (2026-11-20): "for stories with significant overlap we should
be revising not making new stories ... Choir should instead build a singular
narrative that incorporates all the article as they accumulate over time",
and "a story stops growing when the main point, the main beat changes ... the
point is not to put everything in the same story, but that we can use natural
reasoning to cluster things so that we aren't making new stories every time a
newspaper has a new story." So the unit of a story is its main point, and the
boundary is a reasoned judgment, not a time window, a vector threshold or a
topic label.

**The main point.** One sentence, written at the level of the beat, saying
what the story is for: "Pew projects Islam will become the world's largest
religion by 2035"; "The results of the election as they come in". Not a topic
("religion", "the election") and not a headline's detail. Management writes it
in the open, from the opening capture; it is a field of the Texture document,
so each revision carries the wording in force. Texture may reword it in a
version to say the same point better, and each wording is recorded with its
author and time. Texture does not change it to a different point: when its
material has moved to a new point, it says so to management (`main point
moved`, below) and a successor opens. Every story judgment reads the newest
wording, so the statement the judge compares against is itself recorded and
revisable.

**The story judgment.** For each capture a management task holds, one
question: which held story's main point does this capture serve? The answers:

| Answer | Meaning | What happens |
|---|---|---|
| `same S` | it serves S's main point | S is revised: the capture goes to S's Texture desk as a feed, material for S's next version; no story opens |
| `next S` | it is about what S is about, but the main point has moved on (the count after the campaign, the verdict after the hearing) | a new story opens with its own main point and a `succeeds` edge to S; S keeps its own point and stops taking this beat |
| `with k` | same main point as capture k of the same task | the two are one judgment: one story, opened or fed |
| `new` | a different matter | a new story opens, no link |
| `decline` | not news for the paper | as today |

**What the judge is shown.** The host builds it into the task's first message,
inside its token bound:

1. each capture: title, lead, outlet, time; its happening from the flush, with
   p; its central entities;
2. for each capture, up to 12 candidate stories, at any age: stories that
   report its happening, a happening it `follows` or one it is `part_of`
   (after L3); nearest stories by the capture's vector, against published
   stories' vectors and, for unpublished stories (39% of the earlier partners
   in duplicate pairs, **M**), their opening capture's vector; stories sharing
   two or more central entities with it. Each candidate shows its newest main
   point, headline, last revision time, its predecessor's and successor's
   main points, how it was found, and System 1's answer for the pair. No title
   words: the stopgap's title-word offer fired on 2 or 3 of about 37 opens
   and every verdict was "different" (**E**);
3. the whole page of the last 8 hours, headlines and ids, as C1b ships it
   (about 470 of 490 shown). The page reaches the stories the candidates miss
   and moved feeds from 51% to about 63% of decisions (**E**);
4. the task's other captures, so two captures with one new main point open one
   story, not two.

**Who decides, in which call.** The management task, an RLM run that already
decides open, feed or decline for every captured poll (today model
`swe-2-flash`): no new run and no new desk. System 1 gives the first pass in
the flush's one Jev call, with every question about the flush's captures in it
(B2 item 4): for each capture, choices `same_<s>` and `next_<s>` per
candidate, `with_k<n>` per sibling, and `new`. Its distribution is recorded in
`semantic.flush` and shown beside each candidate. It is advice. The task
decides, and its p is its own forecast. Nothing is routed by a probability
band alone.

**What it records.** The task's cast for each capture writes management's
`choir.capture_disposition`, extended with the judgment: the answer and story;
the candidates shown, in order, each with how it was found and the main-point
revision read; System 1's receipt and top answer; the task's p; and a reason
of one sentence, required for `next`, and whenever the task goes against
System 1's top answer. An open carries the new story's main point. The
`succeeds` edge is exported from the disposition. C1b's `held_check` (story,
verdict) is the first form of this record; L5 changes its candidates and its
words (`same_event`, `follow_up`, `different` become `same`, `next`, `new`).

**How a wrong judgment is superseded.** Every correction is a new disposition
naming the one it supersedes; nothing is overwritten.

- *Fed to the wrong story.* Texture marks the capture unused with `not this
  main point`. The forecast settles false, and the capture returns to the next
  management task, which judges it again with that refusal shown.
- *Fed, but the point has moved.* Texture marks it `main point moved` with the
  new point in one sentence. Management's next task opens the successor
  (`next S`) with that point.
- *Opened when it should have revised.* At the new story's first publish, the
  same candidate read runs against held stories (D2). When a candidate is
  judged to hold the same main point, the newer story's Texture is told "S
  holds this main point: fold into S, or keep". A fold writes `same S` over the
  original open, and the newer story answers "continued in S".
- *The audit and the owner.* The daily sample (B6) includes story judgments; a
  verdict against the judgment is a superseding disposition by management,
  with the audit's record as its reason.

**The owner's election, worked.**

| Capture arriving | Held stories and their main points | Answer | Why |
|---|---|---|---|
| a poll shows A up four points | S1 "A's momentum in the final weeks" | `same S1` | serves S1's point |
| an attack ad against B airs | S1 | `new` (S2 "The campaign's ads") | the owner: momentum, ads, the debate and what someone said are different stories |
| a debate clip | S1, S2 | `new` (S3) | a different point, though the same election |
| a county is called at 21:00 | S1 to S3 | `new` (S4 "The results as they come in") | election night is its own point; no earlier story is about the count |
| more counties called, a swing state flips | S4 | `same S4` | the results still coming in: S4 grows all night |
| the networks call it for A | S4 | `next S4` (S5 "A wins; who won and who lost") | the point moves from the count to the outcome |
| B concedes the next morning | S5 | `same S5` | the outcome, told more fully |
| a recount is ordered a week later | S5 | `next S5` | the point becomes whether the result stands |

S1 to S3 share an election and entities; the happenings and entities make them
candidates, and the reasoning keeps them apart. The `succeeds` chain S4, S5 and
the recount is the storyline.

**The rule against real pairs.** The duplicates file names its pairs by title
in section 2.2 (25 rows), section 4.3 (6 traced openings) and three
look-alikes; its appendix gives the 238 labelled pairs as ids only, and the
titles are on the paper, which this pass did not read. So I had 34 titled
pairs and judged 22 of them; the evidence label is the file's (D duplicate, F
follow-up, X counted distinct).

| # | Earlier, later (gap h) | Label | Rule | Why |
|---|---|---|---|---|
| 1 | Six killed in Georgia gas explosion, the same title (5.4) | D | same | identical point |
| 2 | Pew: Islam largest religion by 2035, the same (20.2) | D | same | identical point |
| 3 | UK sanctions eight Israeli settlement firms, the same (47.7) | D | same | same point two days on; a 24-hour window would have opened a story |
| 4 | UK sanctions firms, "Australia joins"; Australia sanctions settler leader Weiss (50.4) | D | same | Australia's act is part of the point the earlier headline already holds |
| 5 | Russia's mass missile strike on Ukraine; massive overnight attack (3.5) | D | same | one strike |
| 6 | Russian missiles and a drone land in Poland; Polish PM calls it "intentional provocation" (9.5) | D | unclear | is a government's charge a new point, or a reaction inside the incident? |
| 7 | Nikhil Gupta sentenced to 19 years 7 months; the same (1.3) | D | same | one sentencing |
| 8 | ICJ to deliver verdict Thursday on Myanmar; ICJ finds genocide (36.0) | D | **next** | the point moved from "a verdict is due" to "what the court found" |
| 9 | First deep-sea mining lease auction off American Samoa (route says "schedules", headline says "one company wins rights"); US sells first leases for $6m (40.9) | D | same, with drift | the later serves the earlier's current point, but the earlier story already crossed from "scheduled" to "held" inside itself: under the rule that was a `next` |
| 10 | US loses measles-free status; the same with a German ex-minister's warning (54.2) | D | same | the loss is the point; the warning is a side |
| 11 | Google adds an AI sidebar to search; Google redesigns its search page (25.9) | D | same | one redesign; the earlier story was 25 hours old and on no page |
| 12 | Trump calls Miller "the most powerful man in the world"; Trump's press conference after Asia, hails Miller (2.2) | D | unclear | a remark's story against the event that holds the remark; the task judged them different |
| 13 | Kynev arrested; Moscow extends his detention to December 24 (42.3) | F | same | his detention is the point; the extension serves it |
| 14 | Eight killed in Brooklyn collapse; new video of the collapse (20.1) | F | same | new material, same point |
| 15 | India and Japan set to review defence ties at 2+2; they agree to expand cooperation (44.8) | F | **next** | a meeting due, then what it agreed |
| 16 | Senate clears the way for the permitting bill; Senate passes it 62-33 (39.8) | F | **next** | the point moved from the vote's prospects to passage |
| 17 | Kress passes first test as interim CEO, board keeps Huang out; Nvidia confirms Huang resigned (46.6) | F | **next** | an interim arrangement, then a resignation |
| 18 | Nations League draw set for Friday; England drawn against Norway (3.1) | F | **next** | a draw due, then the draw |
| 19 | French Senate delays police firearms vote; Parliament approves it (20.0) | F | **next** | delay, then passage |
| 20 | Bukele due to register for a third term; Bukele registers (4.5) | F | unclear | "seeks a third term" holds both; "registers today" does not |
| 21 | Philippine charge against Thomas Markle; US judge finds him fit for trial (48) | X | unclear | two jurisdictions: one prosecution's beats, or two matters? |
| 22 | A third US carrier reaches the Middle East; USS George Washington leaves | X | new | different points under one situation, which is a `part_of` parent happening, not one story |

Tally: 11 same, 6 next, 1 new, 4 unclear. Of the 12 duplicates, 9 revise, 1
is a successor (the ICJ) and 2 are unclear. Of the 8 follow-ups, 2 revise and
5 are successors. What the pairs teach:

- **A follow-up is not the same as a changed point.** The file's follow-up
  label meant "a real development". Under the rule some developments revise
  (Kynev, the Brooklyn video) and most open a successor.
- **The commonest successor is preview, then outcome** (rows 8, 15, 16, 18, 19):
  "is due", "is set", "delays", then "did". A preview's main point should say
  it is a preview, so the judge sees the change when the outcome arrives.
- **Time windows are wrong.** Six of the eleven `same` answers are 25 to 54
  hours apart. The earlier draft's 24-hour development window would have
  opened a second story for row 13, and its 72-hour live window works for
  these only because the earlier stories were revised recently; a story still
  growing three days after its last revision would be missed. Candidates are
  found at any age; recency only ranks them.
- **The unclear cases are decided by how the main point was written.** Rows
  12 and 20 turn on the level of the sentence ("Trump's post-Asia press
  conference" holds the Miller remark; "Trump praises Miller" does not), and
  rows 6 and 21 on whether a reaction or a second proceeding is a new beat. So
  the main point is the thing to write well, to show the judge, and to audit;
  and the judge needs a default for unclear. The default is `same`, with the
  reason recorded: the owner wants fewer, deeper stories; a wrong revise is
  undone by Texture's `main point moved` at the cost of one Texture turn, while
  a wrong open costs a research run (about 3.1 searches, **M**) and, in 53% of
  duplicates, a public repeat.
- **Drift happens inside one story today** (row 9). Without a recorded main
  point Texture revised "an auction is scheduled" into "a company won it". The
  recorded point makes the crossing visible to the next judgment and to the
  audit.

This sample is the evidence file's own examples, chosen to show its labels, so
it over-represents clear cases; 22 pairs measure nothing. It is a first
reading of the rule, and C-S10 asks for a proper sample.

**What sets the bar.** No threshold routes, so there is no bar to tune on day
one. What is tuned is what the judge is shown: the candidate list's recall on
the 238 labelled pairs (the share of earlier partners among a later story's
12 candidates), measured before L5 lands by replaying the pairs' opening
captures through the candidate read, and kept at 90% or above.

**What it saves.** See D3. C1b already moved part of it: feeds went from 51%
to about 63% of decisions; the duplicate share since is unmeasured (C-S3).

### D2. Duplicates already published, and the chain on the front

At a story's first publish, corpusd runs the candidate read of D1 against
stories published in the last 14 days. When a candidate holds the same main
point by the judgment of management's next task, the host writes a record to
the **newer** story's Texture desk: "story S holds this main point; fold your
findings into S, or keep". Texture decides, because it is the custodian of its
document. On fold, the newer story's route answers "continued in S" and leaves
the front; its findings go to S's desk as a cast with its captures, the path
management tasks already use. Merging published text is not built. The front
shows the newest story of a `succeeds` chain, with its predecessors as its
earlier chapters; the reader sees one growing narrative per beat and the
storyline behind it.

### D3. Only search for what is new to us: the arithmetic

Base: 3,557 web searches in the 24 hours to 22:18Z on 2026-11-20 (**M**,
duplicates file section 3); about 3,260 for the day by the goal file's count,
so about 3,200 a day is used below. In the first three hours after C1b the
hourly counts were 74, 106 and 99, against 145 and 109 before (**E**); three
hours do not show a fall. 90% of research runs and 94% of searches are a
story's first run (**E**, research-runs file). 34 of 41 research objectives
asked for a second source of one captured claim (**E**,
[corpus-first-search-one-verb-2026-11-19.md](../definitions/corpus-first-search-one-verb-2026-11-19.md),
its 2026-11-20 note). The owner's target is under 1,000.

| Mechanism | Searches a day removed, of about 3,200 | Basis |
|---|---|---|
| **One main point** (L5): a second story that should have revised never opens | at most about 550 (17%); realistically 220 to 500 (7% to 16%) | duplicates and clear follow-ups took 614 of 3,557 (17.3%, **M**). The 22 pairs revise 9 to 11 in 12 duplicates and 2 to 3 in 8 follow-ups (**A** from a small, selected sample; the range is the unclear pairs counted either way): 450 to 560 of 3,557 (13% to 16%) if a revised story spends nothing on its new material, about half that if it spends half, the file's own low case. Part of this is already taken by C1b |
| **Successors start from their predecessor** (L5, L7): a `next` story's read includes what its predecessor cited | about 50 to 80 | follow-ups that become successors carried about 130 to 150 of 3,557 searches (**A**: 5 or 6 in 8 of the 208 searches of clear and borderline follow-ups); half of that held |
| **Read before a search: a second origin held** (L7) | about 120 at today's joins; about 500 to 600 if the hand read's missed joins were found; unmeasured above that | second-source searches are about 2,500 a day (**A**: 83% of objectives times 94% of 3,200). Today 14 of 300 events (4.7%) hold two intake members (**E**); 10 of 50 hand-read mints should have joined (**E**), which would put perhaps 20% to 25% of happenings at two (**A**). The vector part of the read (C7 item 4) does not depend on joins and is unmeasured |
| **Stop when the objective holds two outlets, during the run** | 200 to 500 | the one-verb note's estimate (**A**) |
| **Repeats with nothing new** | 0 to 180 | research-runs file (**E**) |

The levers overlap, so the sum is not valid. My central guess is that
together they leave about 1,500 to 2,300 searches a day (**A**).

**Consolidation alone does not reach 1,000.** Removing every search of every
duplicate and follow-up would leave about 2,650 of 3,200; the realistic share
leaves about 2,800.

**What would reach 1,000.** Removing about 2,200 searches a day means the
read before a search must hold most second-source searches: 60% to 70% of new
stories' main points must already have a second independent origin in the
corpus when their first run starts. Today it is about 5%. Three things could
close the gap, in order of what is known: (1) joins and the vector read find
the second outlet the paper's own polls already brought (about 10,000 captures
a day, **M**, survey 6.2); how often a second polled outlet exists is the one
number that decides this design's search claim, and C-S6 states it; (2) a
story's first run waits for the next one or two polls before it searches for a
second source, because outlets report one happening within hours; this delays
the first publish (today a median 33 minutes after opening, **M**) and is the
owner's call (F3, decision 10); (3) fewer stories open at all, which is (1) of
the table. If neither (1) nor (2) holds, the design does not reach 1,000, and
it says so.

### D4. Related coverage

The related answer's `basis.events` is always 0 today (**M**, survey item 1).
After L3, related stories are: the story's `succeeds` chain first; same
primary happening; happenings linked by `follows`; shared central entities
(after merges resolve); shared cited captures. The walk that exists is
extended (`publication_related.go:593`), not rewritten. Autoradio tells a
follow-up when a candidate story `succeeds` a heard story, or a revision of a
heard story when the story itself grew.

### D5. Entity pages, and the rule for private people

The public entity read exists (E2). After L4 it gains capture-side mentions
and resolved merges. The rule for people stays: a person page is served only
with a Wikidata id or as central in two public stories (C5). A private person
(a pedestrian struck, a victim) is named only inside the story that names
them, never on an aggregating page, and an agent's read of their mentions is
scoped to one happening or story. Organizations and places are always served.

### D6. Corrections

A correction is a later version whose claim supersedes an earlier claim.
What the layer adds: when a capture is fed to a story (`same S`), the next
flush can ask, in the same call, whether the capture contradicts
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
management's newest-50 page (by the 8-hour page of C1b, landed); C1b's
title-word held offer and its `held_check` words (by the candidate read and the
story judgment, L5); the need-test's separate holds (by the read before a
search, L7); the stopped side measurement of search sufficiency (by the
`search.decision` record); `choir.event_projection` (by the event itself).

**Deleted:** the setting `research.related_story` (a switch, **C**,
`store/computer_setting.go:190`) and the environment switches
`CORPUSD_RELATED`, `CORPUSD_RELATED_STORY`, `CORPUSD_RELATED_ENTITIES` (**C**,
`publication_related.go:60-64`); `RelateStories`, the `related_story` and
`relies_on` edges and the `story.relation` record's export; the kinds
`choir.event_membership`, `choir.capture_label`, `choir.capture_label_override`;
`intakeOverlapPicks`; C1b's rare-title-word matcher; the related walk's
title-word filler. Reviewed for
deletion: the eight provenance edges no one reads.

**Side tables with no custodian entry.** `vec_*`, `proj_capture_*` and the
lexicon are derived indexes under I27's named exception and stay so, with
corpusd as their only writer. `search_leads` is not derivable (it holds
provider results kept nowhere else), so under I27 it is a home without a kind.
It carries no semantic fact; the read before a search (C7) reads it for a
story's earlier searches, and this design leaves it as it is, residual R6.

### E2. The landings

Review tiers by the contract: orange one seat (seat 2); red two seats (seat 1,
Gemini 4 Hydrogen through the official `agy` CLI or, on a rate limit, a Claude
Sonnet subagent that did not write the change, plus seat 2). Drills on d@e only; the paper changes only by deploy. Every
non-docs landing carries `Ring-Expedite`.

| # | Landing | Class | Review | Proof on staging | Rollback | Model calls and money a day |
|---|---|---|---|---|---|---|
| **L1** | **Management's page by time: landed** as station C1b (d8367e2c8, repaired by 2b862a00e and e33533850; the paper on it since 2026-11-21 02:46Z). The page is every headline of the last 8 hours (about 470 of 490 shown) instead of the newest 50; held look-alikes by rare title words are offered with a required verdict. Readings, first three hours (**E**): feeds 51% to about 63% of decisions; the title-word offer fired on 2 or 3 of about 37 opens, every verdict "different"; web searches 74, 106, 99 an hour, no visible fall. So the page carries; the title-word offer does not, and L5 replaces it | orange | one seat (done) | the duplicates method re-run on a day's opens (C-S3) | revert | 0 calls; about 14,000 input tokens per management task for the page (**A**, 470 headlines at 30 tokens) |
| **L2** | **Flush provenance and reach.** The ready list replaces the memory queue; `proj_capture_resolution` written; the `semantic.flush` tape record on channel `semantic:<computer>`; candidates from every distinct event among 50 vector neighbours; same-flush siblings clustered by vector and listed first, banded like events; the memory index and its caps deleted. Its join rate (C-S1) serves corroboration and the happening part of the read before a search; story consolidation no longer rests on it (L5 finds candidates by vectors and entities as well) | red (tape, provider call content) | two seats | `op=event_flush` lines over a day (C-S1); a hand read of 50 mints through the record by a reader who did not build it (C-S2); a crash drill on d@e shows unresolved captures answered by the next flush | revert; the record is inert without the code | same call count; about 30% more tokens a call (**A**), under $0.10 a day |
| **L3** | **Happenings move to the Autopaper object graph** (absorbs the E5 lane). A corpusd commit route for flush results; `choir.world_event` re-registered under corpusd; `in_event` edges with role and record; `follows` as an edge; the paper's events migrated (E5); the VM kinds and `choir.event_membership` deleted; custody of judgments to research; `reports_event` at open and at publish; the public event read, consolidation read and `basis.events` | red (custody, export, tape, public projection) | two seats | on d@e a polled capture's event readable at `/api/platform/events/{id}`; on the paper, share of new stories with a `reports_event` edge (C-S8); migration counts equal the paper's old totals | revert; the cleanup script tombstones exported rows (E5 lane's `scripts/cleanup-story-events`); the paper's state.db rows are tombstoned, not deleted, until the next landing | 0 calls |
| **L4** | **Entities and merges in the flush** (absorbs E3, E4's flush half). Mentions from captures; outlet entities and the one outlet key; the qualified-id rule; merge and retire questions ride the flush; `choir.ontology_act`; the intake labeller's call deleted, its questions absorbed | red (tape, provider calls, export) | two seats | C-S7 (entity links), C-S9 (Jev calls fall); a d@e drill of a forced merge and its undo read back publicly | revert; tombstone capture-sourced edges and acts (re-export restores them) | calls fall (one call replaces two per poll); about 12 million more tokens, about $0.50 (**A**, entity design 8) |
| **L5** | **One main point per story** (D1, D2; replaces C1b's title-word offer, rewrites C1b on the ontology as the owner asked). The main point as a field of each Texture revision, written at open; the candidate read (vectors, including unpublished stories' opening captures; shared central entities; happenings once L3 lands); the story judgment `same`, `next`, `with`, `new`, `decline` in the management task, recorded in the extended `choir.capture_disposition` with candidates, receipt, p and reason; System 1's story question in the flush; the `succeeds` edge; Texture's reasons `not this main point` and `main point moved`; the first-publish fold record; the front shows a chain's newest story. First half (no happening candidates) may start with L2 | red (crosses management dispositions, Texture's inbox and the provider call content) | two seats | before landing: the candidate read's recall on the 238 labelled pairs (D1); after: C-S4, C-S5, C-S10; stories first published a day does not fall over 10% | revert; dispositions are superseded, not deleted | 0 new calls; more tokens in the flush call and the management task (**A**, unmeasured) |
| **L6** | **`choir.World` reads** for research, management, and Texture scoped; labels; the person rule; the event substring search deleted | orange | one seat | a d@e research run answering questions 1, 2, 6 and 7 of C4; the person refusal; recall at ten of the 200-question set does not fall | revert | 0 calls |
| **L7** | **The read before a search** (C7; this is search station L3's read, and replaces the stopped side measurement): `choir.World.Held`; the host's search path reads it before each send; verdicts `send` or a hold, with the need-test's holds folded in; `search.decision` and `search.result` on the run's tape; the brief carries the same read | red (tape, provider calls) | two seats | C-S6 (searches a day), C-S11 (every search has a decision read before it); stories first published a day does not fall over 10% | revert | 0 model calls; one corpusd read per attempt |
| **L8** | **`Ask` and `Judge`.** No 12-question cap; the story analysis in one call; ontology acts by research; owner undo; the daily audit run | red (tape) | two seats | a d@e drill of each act and its undo; the first day's audit settles the flush's forecasts by band | revert; acts are superseded, not deleted | `Ask` per use; the story analysis halves its calls; one audit run a day (**A**: about 1 million input tokens, from the 1.0 million mean of a research run, duplicates file 3) |
| **L9** | **Deletions**: `research.related_story` with its finish-rule fix (E6), the three `CORPUSD_RELATED*` switches, `RelateStories`, `related_story`, `relies_on`, the provenance edges once the read shows no reader | red (desk finish rule) for the setting; orange for the rest | two seats; one | on d@e a story run records entities with no setting | revert | 0 calls |
| **L10** | **Backfill**: entity questions for old captures ride the ready list (E7) | orange | one seat | resolved count reaches the capture total in the growth read; gateway call counts | revert; links stay valid | about 5,600 calls once, about $13 (**A**, entity design 9) |
| **L11** | **Corrections**: contradiction questions for captures fed to a story | red (provider call content, Texture inbox) | two seats | a hand read of 50 flagged contradictions | revert | **A**: 10 to 30 more questions a fed capture; cost unknown until L5's records show fed volume |

L1 has landed. Next are L2 and the first half of L5, in parallel; L3 waits
for L2's record so its migration carries provenance forward, and adds
happening candidates to L5. L7 can follow L6 without waiting for L3, because
the read's vector part does not need joins.

### E3. Today's stations: absorbed, changed or dropped

| Station | Fate |
|---|---|
| E3 (captures get entity ids in the flush; stopped today) | absorbed into L4, changed: rows go to the corpusd home; its `capture.entities` tape record becomes part of `semantic.flush`; `choir.entity_merge` becomes `choir.ontology_act` |
| E4 (merges and retirements) | split: merges that ride the flush go in L4; research acts, splits and the owner undo in L8 |
| E5 and the lane `story-events-e5` | absorbed into L3; see E4 below |
| E6 (remove `research.related_story`) | L9, unchanged in substance |
| E7 (backfill) | L10, unchanged |
| E8 (corpus search expands an entity) | absorbed into `Find` (L6): an entity filter and the entity's merged ids |
| Search station L3 (the one verb) | changed: its read before a search is L7, with the `search.decision` record; search station L2 (the side measurement) was stopped and is not resumed |
| Corpus-first S9 (neighbourhood read for management), S10 (fold at first publish), S11, S12 | S9 and S12 into L5; S10 replaced by the story judgment before research (L5) and the first-publish fold record (D2); S11 into L4 and L8 |
| C1b (the open decision sees what the paper holds) | landed as L1; its page stays, its title-word offer is replaced by L5 |
| C2 (one narrative per topic) | is L5 and L7 |

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
(**A**). The management task's 8-hour page adds about 14,000 input tokens a
task (L1, landed); the story question adds tokens to the flush call (L5,
unmeasured). The read before a search costs one corpusd request per attempt
and no model call. No new model, no new service, no new database.

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
  first teacher depends on it; L5 adds the two reason codes.
- **The stopgap's readings.** Feeds, opens, offers and verdicts after C1b are
  the lead's reading of the paper's console over three hours, not mine.
- **The 238 pairs' titles.** The appendix gives ids only; the 22 pairs of D1
  are the titled examples in the evidence file's text.
- **Whether the open cast and the Texture revision can carry a main-point
  field as designed.** Not read in code.
- **Whether the eight provenance edges have an outside reader.**
- **Token and cost figures** are estimates from SYSTEM1's figures and the
  entity design, not measurements.
- **The cost of `Walk` at four hops on production data.** The bounds are
  designed; the latency is not measured.
- **Whether yaegi handles the struct types as written.** `choir.Searches`
  returns typed values through a handle, so the pattern exists (**C**,
  `yaegikernel/choir.go:457`); the exact types are untested.

### F2. Where I am guessing, and what I am least sure of

**Least sure of:**

- **Whether a model can hold a main point steadily.** The whole rule rests on
  a one-sentence statement written at the right level. Four of 22 real pairs
  were unclear, and each turned on how the point was worded. If management's
  task (today `swe-2-flash` at low reasoning) writes points too narrow, every
  new detail looks like a new beat and stories stay shallow; too broad, and
  one story swallows a war. C-S10 tests it; nothing has yet.
- **Whether the paper's own polls bring a second outlet often enough** for
  the read before a search to remove most second-source searches (D3). That
  number decides whether 1,000 a day is reachable, and it is unmeasured.
- **How the stopgap's three hours generalise.** Feeds 51% to 63% is one
  evening's console, read by the lead; whether opens fell in duplicates or in
  distinct stories is unknown until C-S3 is settled.

Also guessing:

- That widening the shortlist and clustering siblings lifts happening joins
  (C-S1 states it with a number). Consolidation no longer depends on it;
  corroboration and part of the search read do.
- That the default `same` for an unclear judgment is the right side to err on;
  it follows the owner's "fewer deep stories", and Texture's `main point
  moved` is its undo.
- That 12 candidates, at any age, hold 90% of earlier partners; the replay on
  the 238 pairs measures it before L5 lands.
- That research is the right authority for happenings and management for the
  story judgment, rather than a new desk. Both add nothing and fit I26's
  words; a panel might disagree.

### F3. The owner's decisions, each with the default I would take

1. **Happenings move to the Autopaper object graph, and research becomes the
   judgment authority for happenings, memberships, entity links and ontology
   acts; management keeps dispositions, which now include the story
   judgment; Texture holds each story's main point.** Default: yes.
2. **Settled by the rulings.** A development whose main point is the story's
   revises it; a changed main point opens a new story that `succeeds` the
   old one; no time window. (Replaces the earlier default of a 24-hour
   window.)
3. **Arrival waits for the flush (about one to two minutes)** so the judge
   sees the capture's happening, entities and System 1's first answer.
   Default: yes.
4. **Settled in direction by the rulings: reasoning, not a threshold.** Every
   story judgment is made by the management task, with System 1's answer as
   advice; no capture is fed by a probability band alone. Open: whether,
   after a month of audits, a band where System 1 and management agree at
   0.95 or more may feed without a management turn. Default: no, until the
   audit shows it.
5. **Partly settled ("revising not making new stories").** A published story
   found to hold another's main point is offered a fold and, if folded,
   answers "continued in"; published text is never merged automatically; the
   front shows the newest story of a `succeeds` chain. Default: yes.
6. **People: public page only with a Wikidata id or central in two public
   stories; agent reads of a private person scoped to one happening or
   story.** Default: yes.
7. **Other computers mint happenings and entities in their own owner's scope
   and read the paper's as candidates; nothing crosses owners.** Default: yes.
8. **Spending: about $0.50 to $0.70 a day more in model tokens and about $13
   once, with no grant; search credits fall.** Default: proceed, and report
   the day's cost and searches in the status read.
9. **The audit, of flush decisions and story judgments, is run by research.**
   Default: yes; the alternative is a management task for flush decisions, if
   the owner reads rule 1b as forbidding the same desk. Story judgments are
   management's, so research auditing them already satisfies rule 1b.
10. **A new story's first search for a second source waits for the paper's
    next one or two polls** (D3), trading a later first publish for fewer
    searches. Default: no until L7's `search.decision` records show how often
    a second outlet arrives by poll within that time; then the owner decides
    on the measured trade. (The earlier decision 10, `search_leads` without a
    kind, is now residual R6 only.)

### F4. Claims worth precommitting

Ids are provisional; the ledger assigns them at entry. Each outcome is a count
or a hand read by someone other than the forecaster.

| Id | Claim | p | Test | Settle by |
|---|---|---|---|---|
| C-S1 | After L2, intake questions that mint a new event fall from 84% to at most 65% over a day. (Gates corroboration and the read's happening part, not consolidation) | 0.55 | `op=event_flush` lines, intake only | L2 deploy + 1 day |
| C-S2 | After L2, of 50 hand-read mints, at most 5 should have joined (from 10 of 50) | 0.50 | hand read through the `semantic.flush` record by a reader who did not build L2 | L2 deploy + 2 days |
| C-S3 | After L1, true duplicates fall from 17.4% to at most 12% of a day's opened stories. (Set before the readings; first three hours: feeds 51% to 63%) | 0.45 | the duplicates file's method re-run on one day | L1 deploy (2026-11-21 02:46Z) + 2 days |
| C-S4 | After L5, opened stories that a reader who did not build L5 judges should have revised a held story (same main point) are at most 6% of a day's opens (from 17.4% duplicates before L1), and stories first published a day do not fall over 10% | 0.40 | the duplicates method, labelled by main point; publication counts | L5 deploy + 3 days |
| C-S5 | Of 100 `same` feeds after L5, Texture uses at least 75 and marks at most 10 `not this main point` | 0.50 | the stories' revisions and source marks | L5 deploy + 3 days |
| C-S6 | After L5 and L7, web searches a day fall to at most 1,800; to under 1,000 | 0.40; 0.10 | gateway `search request` lines | L7 deploy + 7 days |
| C-S7 | After L4, at least 70% of resolved intake captures carry a counted entity link, and at least 90 of 100 sampled links are right | 0.55 | growth read; hand read | L4 deploy + 2 days |
| C-S8 | After L3, at least 80% of stories opened in a day carry a `reports_event` edge | 0.60 | public story reads | L3 deploy + 1 day |
| C-S9 | After L4, Jev calls of the sites `intake_labels` and `event_layer` together fall by at least 30% at equal captures | 0.60 | gateway site counts, a day before and after | L4 deploy + 1 day |
| C-S10 | Of 100 story judgments sampled after L5, a reader who did not make them gives the same answer (same, next or new) for at least 80 | 0.50 | hand read of the disposition records with the captures and main points | L5 deploy + 3 days |
| C-S11 | After L7, at least 99% of gateway search lines in a day have a `search.decision` whose corpus sequence is earlier than the search's own leads, and at least 20% of first-run second-source attempts end in a hold | 0.70; 0.35 | gateway lines joined to the run tapes; the decision records | L7 deploy + 2 days |

### F5. What would falsify the design

- **The judge cannot hold a main point.** If C-S10 finds agreement under 70%,
  or Texture marks more than 20% of feeds `not this main point`, the
  one-sentence main point is the wrong tool; the next step is to show the
  judge the story's latest version text instead of its statement.
- **Stories swallow.** If a hand read finds stories whose latest version
  covers several main points (a list, not a narrative), the default `same`
  for unclear is wrong or the points are written too broad.
- **Happenings stay singletons.** If after L2 and L4, with vectors present,
  more than 70% of intake questions still mint, happenings will not carry
  corroboration or the read's happening part. Story judgments still work from
  vectors and entities; the second-outlet test then rests on the vector read
  alone.
- **Agents do not use the surface.** If a week of research ledgers after L6
  shows `choir.World` in under 10% of runs, the library is the wrong shape for
  how the agents work, and the brief should carry the answers instead.
- **The read does not stop searches.** If runs send over `two origins held`
  at the same rate as they send after `send`, the search hypothesis in D3 is
  false.

### F6. Residuals

R1, the restore gap (A8). R2, two new same-name entities in one flush are
joined (entity design 4.3), undone by a split. R3, OpenRouter's long-context
line above about 32,000 bytes may change answers (SYSTEM1 section 10). R4, a
private computer's objects are never compared with another owner's. R5, a
podcast clip names its story by a route string until a `clip_of` edge exists.
R6, `search_leads` is a home without a kind.
