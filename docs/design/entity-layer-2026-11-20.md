# The entity layer: captures, stories and events linked by who and where (design, 2026-11-20)

**Status: a proposal, not ratified.** Written 2026-11-20 between 05:00Z and
06:00Z by a design agent (Opus, no code) for station E2 of
[the goal of 2026-11-20](../definitions/choir-exact-search-entities-2026-11-20.md).
Nothing here is built unless [current-architecture.md](../current-architecture.md)
says so. It extends stations S8 and S11 of
[corpus-first retrieval](corpus-first-retrieval-and-research-2026-11-18.md)
and replaces their entity half. Claims to precommit are in section 12,
owner questions with defaults in section 13.

Evidence tags: **M** measured (source named), **C** read from code (file and
line at `origin/main` 2ea867cac), **A** assumed or estimated, **U** not
verified. Section 14 lists what I could not verify.

## 1. The premise was wrong: an entity layer exists, for stories only

The brief said no entity layer exists. That is true for captures and false
for stories. Read before designing anything:

- **Code.** `choir.entity` (a person, organisation, place, product, work or
  law) is a registered corpusd kind (`internal/objectgraph/registry.go:224-230`),
  claimed by the corpusd kernel service (`internal/kernelservice/kernelservice.go:103`),
  with the edge `mentions_entity` from a publication, metadata exactly
  `{"role":"central"|"incidental"}` (registry, edge list; `platform/publication_story_entities.go:73`).
  A story's own research desk gets a host-built analysis at its first cell
  (`agentcore/story_analysis.go`): deterministic extraction
  (`internal/entitytext`), corpusd's learned lexicon matched in the text
  (`platform/publication_entity_candidates.go`), then System 1 questions
  (role per candidate, type per new phrase, novelty). The finish rule
  records the top answers by default (`agentcore/story_entities_default.go`)
  as a `story.entities` tape record, and the host exports the record to
  corpusd when the story publishes (`agentcore/story_entities_source.go:185-249`).
  Corpusd learns aliases and stopwords from each export
  (`platform/publication_entity_learning.go`).
- **Production, public reads, 2026-11-20 05:17Z.** `GET /api/platform/publications/front?limit=100`:
  80 of the 100 newest stories carry central entities, 240 attachments, 181
  distinct entities, 0 with a Wikidata id (M). `GET /api/platform/entities/growth`:
  6,687 entities, 9,117 lexicon keys, 34 stopwords; 443 stories reviewed on
  2026-11-19, 252 entities minted that day, about 300 to 450 minted a day
  since 2026-11-09 (M).
- **Its gate.** Every step is gated on the setting `research.related_story`,
  off by default (`store/computer_setting.go:42-52,197`; checked at
  `story_entities_verb.go:131`, `story_analysis.go:134`,
  `story_entities_source.go:194`). It is plainly on for the paper (the growth
  read), so the paper has entities and every other computer has none. That
  is a switch under the owner's rule and goes in this work (station E6).
- **It is not in current-architecture.md.** No row names it. Station E0 adds
  one.

So the job is not "build an entity layer". It is: give captures entity ids
through the same identity and the same corpusd rows the stories use; fix the
identity faults the story layer already shows; add merges; link stories to
events at publish; and make all of it readable by a script.

### 1.1 What the story layer gets wrong today (a quick read, not a score)

My own tally of the 240 front attachments above. It fails rule 1 of the
learning engine (I wrote the rubric and read the rows), so it motivates the
design and settles nothing; station E0 makes the scored version.

| Fault | Attachments | Examples |
|---|---|---|
| Not a named thing: common noun, role, topic | 25 | Health, Pedestrian, Vote (2), National (2), IPO (2), Officer, Women, Energy, City |
| Demonym or adjective | 12 | Chinese (3), Indian (2), American (2), Russian, Canadian, Haitian, Somali |
| An event or a headline fragment | 4 | Iran war, Holiday Special Returns, U.S. Commission, U.S. Defense |
| Ambiguous without context | about 12 | Parliament, City Council, Supreme Court, Gulf, Neon, Nothing |
| Two ids for one thing (pairs among the 181) | 7 pairs | U.S. / United States, Trump / Donald Trump, Hegseth / Pete Hegseth, Huang / Jensen Huang, UAE / United Arab Emirates, BMO / Bank of Montreal, Blazers / Portland Trail Blazers |

About 17% of central attachments are not entities, and seven duplicate
pairs show in one page. Two mechanisms explain most of it, both read from
code:

1. **A junk entity, once minted, is permanent.** The default records a
   lexicon hit by its role answer alone: an existing entity id skips the
   type check (`story_entities_default.go`, the `c.EntityID != ""` branch).
   So "Chinese", minted once, is matched by the gazetteer in every later
   story and attached whenever its role answer ranks central or incidental.
   Nothing can retire an entity.
2. **Identity is the normalised name.** An id is `ent:` plus a hash of the
   normalised name (`types/story_entity.go`, `MintedEntityID`), and a new
   entity whose normalised name matches another row is refused
   (`publication_story_entities.go:274-285`). So "U.S." and "United States"
   are two ids forever (the refusal stops nothing, since the names differ),
   and two people called John Smith can never be two entities (the refusal
   forces them into one). Aliases are recorded but no two ids can ever join
   (the retrieval design's reading of `publication_entity_learning.go`
   agrees).

## 2. What an entity is here (object transform)

**An entity is a named, persistent thing in the world that many captures
and stories can be about: a person, an organisation, a place, or a named
product, work or law.** The six types are the ones already stored
(`types/story_entity.go`, `EntityTypes`); I keep them, because removing one
would orphan rows, and add none. The category-gap counter in the growth
read already records when System 1 asks for more ("Ebola is a disease"),
which is the evidence a seventh type would need.

What it is not, each a fault seen above:

- **not a topic** ("Health", "Energy", "Legislation"): topics are what
  intake labels and vectors are for;
- **not an event** ("Iran war"): events are the event layer's, one
  incident, one place, one time (`event_layer.go:980`);
- **not a demonym, adjective or common noun** ("Chinese", "Pedestrian"):
  "Chinese" is a mention of China only if the question says so, and the
  resolver may answer "China" for it;
- **not a role without its holder** ("Officer", "City Council" with no
  city): a role names an entity only with its institution ("Las Vegas City
  Council");
- **not the capture's own outlet** when the outlet is only its byline (the
  outlet is already the capture's `outlet` field; "Variety" is an entity in
  a story about Variety, not in every story Variety printed).

**Identity.** An entity's id is opaque and stable once minted. It is minted
as today, `ent:<hash of normalised name>`, for the first entity of a name.
A second, different entity with the same normalised name (the second John
Smith) is minted as `ent:<hash of normalised name and its opening mention's
capture or story id>`, the way an event's id derives from its opening
capture (`event_layer.go:309-312`). Existing ids never change. `wd:Q<n>`
stays valid but nothing here produces one (Wikidata linking is not built,
section 7).

**Canonical name and aliases.** The row's name is the form that minted it;
the learning loop's aliases stay on the row. A merge (section 6) moves the
merged id's name and aliases onto the survivor as aliases.

**What tells two John Smiths apart.** Not the name. Each row carries a
short descriptor the resolver reads: its type, the title of the capture or
story that minted it, and its three most frequent co-mentioned entities
(derived, recomputed at read). When a mention's name matches two or more
rows, every match is offered with its descriptor, and the capture's own
vector neighbours' entities are offered too (section 4.2), so a John Smith
in an Ohio Senate story meets the John Smith whose neighbours are Ohio
captures.

## 3. Where entities live, and who is custodian

### 3.1 Shared behind corpusd, owner-scoped, with the judgment on the computer's tape

The prior ruling ([standing questions](../standing-questions.md) item 2,
owner 2026-10-11) is "shared content lives in Store B behind corpusd;
computers hold overlays". Entities fall under it, and the code already put
them there:

- **Rows and links (the projection) live in corpusd.** `choir.entity` rows,
  `mentions_entity` edges, and the new kinds below, in corpusd's object
  graph in PostgreSQL on Node B. Custodian: the **corpusd kernel service**,
  the sole writer, as it is today for `choir.entity`. A row's canonical id
  is scoped by owner (`entityCanonicalFor`, `publication_story_entities.go:66`),
  so each owner has one entity graph; the paper's is public because the
  paper publishes, and any other owner's is read only under that owner's
  authority.
- **The judgments (the authority) live on the computer's tape.** System 1
  can only be called from the computer (one Jev entry point,
  `agentcore/jev_job.go`, gate `scripts/check-jev-entry`), so the computer
  asks, records the decision on its tape, and exports it to corpusd, as
  `story.entities` is exported today. Story judgments are made by the
  story's research desk (unchanged). Capture judgments are made by the
  event layer's host loop, which writes as **management**, already the
  custodian of `choir.world_event` and `choir.event_membership`
  (`deskmanifest/manifest.go:362`). Merges ride the same loop (section 6).
- **Desks never write corpusd rows.** They send export requests over the
  bound-guest route (`requireInternalOrBoundGuestWrite`,
  `publication_story_entities.go:351`); corpusd checks each request whole
  and refuses what fails. That refusal is a correctness refusal, not a
  switch.

**Single authority (standing question 5).** For a capture's links and for
merges, the tape record wins; corpusd's rows are its one-way, re-runnable
export, and a disagreement is fixed by re-exporting. For a story's entities,
research's `story.entities` record wins; capture entities only feed its
candidates, so there is no second source for one fact.

**Cost of being wrong.**

- If entities should have been per computer, as events are: the rows are a
  projection of each computer's tape, so moving them is a re-export into
  the VM store, with no model call. Cheap.
- If they had been made per computer and should have been shared: that is
  tonight's event layer. Its events sit in the paper's own store, unreadable
  without a key that had expired (the E1 hand read was refused, HTTP 401),
  and every reader would need the computer up. Expensive, and already paid
  once.
- **The real cost of this choice:** the shared graph is outside a
  computer's restore set ([computer-ontology.md](../computer-ontology.md),
  restore-set boundary). Restoring the paper to an older head rewinds its
  tape but not corpusd, so corpusd would hold links whose records the tape
  no longer has. Story entities have the same gap today. The answer is a
  later reconcile that tombstones projection rows whose tape record id is
  absent; it is named here as residual R1 and not built in this plan.

### 3.2 Derived or canonical

- The **judgments are canonical**: a Jev answer is not reproducible
  (identical requests gave 13 distinct responses in 30, [SYSTEM1.md](../reference/SYSTEM1.md)
  surprise 6), so re-asking is a new judgment, not a rebuild. They are
  never dropped.
- The **corpusd rows and edges are derived**: drop and rebuild by
  re-exporting the tape records, at zero Jev calls. (U: that the export can
  read a channel's whole history; `exportStoryEntities` reads
  `system1ChannelTape`, whose bound I did not read.)
- The **resolution marker table and the lexicon are derived** from the rows
  (the lexicon is already rebuilt at every corpusd start, `publication_entity_candidates.go:1-12`).

### 3.3 Object kinds, edges and tables (named against the registry)

| Name | Kind | Home and writer | New? |
|---|---|---|---|
| `choir.entity` | object | corpusd, kernel service | exists; gains the qualified-id rule (section 2) |
| `choir.entity_review`, `choir.entity_stopword` | object | corpusd | exist; stopwords also learn from capture answers |
| `mentions_entity` from `choir.web_capture` | edge | corpusd | the existing edge kind, from a capture; role `central` (named in the title or the lead's first sentence), `incidental` (elsewhere in the lead), or `tentative` (a link below the join threshold, section 4.3). `types.EntityRoles` gains `tentative`; every reader that counts reads `central` only, as today |
| `choir.entity_merge` | object | corpusd, kernel service (add to `kernelservice.go` Corpusd claims) | new: keyed by the merged id; body `into` (the survivor, or empty for "not an entity, retired"), computer, tape record id, Jev receipt ref, p. A tombstone undoes it |
| `choir.event_projection` | object | corpusd | new: the public projection of a world event (id, headline, first seen, `follows`, computer), exported from management's VM-store event. Declared a projection, like `choir.story_index` (`objectgraph/reader.go:175-190`), so it is not a second home under I27 |
| `in_event` | edge | corpusd | new: a public capture to its event projection; role `member` or `tentative` |
| `reports_event` | edge | corpusd | new (S8): a publication to an event projection; role `primary` or `secondary` |
| `proj_capture_resolution` | table | corpusd, derived | new: one row per capture the layer has answered: capture id, computer, record id, outcome `resolved` or `refused`, time. It is how corpusd knows what is still waiting (section 4.1) |
| `capture.entities` | tape event | computer (management) | new: one per flush; every capture's decisions, mints, merges and retirements, with the Jev receipt ref. Channel: `system1:<computer>`, as `story.entities` (`story_entities_source.go`), unless the builder finds a reason in code to differ |

## 4. How a capture gets entity ids

### 4.1 Wait for the inputs; never flush before they exist

Tonight's lesson: 78% of the questions put to the event layer had no vector
yet (the E1 evidence file, its section 1). A likely cause, read from code and not traced:
**a capture write does not wake the embed worker**. `KickVectors` is called
only by the vector import (`platform/vector_import.go:741`); an idle worker
looks again every five minutes (`vectorIdleInterval`,
`platform/vector_worker.go:62-64`); the event flush fires 60 seconds after
the first queued capture (`event_layer.go:40,1159`). A new capture waits a
mean of about two and a half minutes for its vector, and the flush asks at
one. The fix is one line on corpusd's capture write path (kick the worker),
station E1 here.

Then the layer stops asking about captures whose inputs are not ready:

- **The work list is a fact in corpusd, not a queue in memory.** A capture
  is ready when it has a stored vector, or corpusd will never make one
  (owner-private, refused or tombstoned, the reasons the neighbours route
  already distinguishes, `event_layer.go:854-859`), and it has no row in
  `proj_capture_resolution`.
- **One corpusd request per flush returns everything the call needs**: the
  ready captures (title, lead passage, outlet, language), each one's vector
  neighbours with their event and entity links, each one's entity
  candidates (section 4.2), and a count `waiting=` of this computer's
  captures not yet ready. It extends `/internal/platform/captures/neighbours`
  (`platform/vector_neighbours.go`), the request the flush already makes;
  it is not a second round trip.
- **What triggers a flush is unchanged**: a new capture arms the 60-second
  timer, as today (`enqueueEventCaptures`, `event_layer.go:1131-1169`). The
  timer starts a read of the ready list, not a drain of what was queued.
  When the answer says `waiting>0`, the layer arms one more timer for the
  next minute. So a computer with captures still being embedded makes one
  corpusd request a minute, and a quiet computer makes none. No poll storm,
  no boot path.
- **Unresolved captures are retried by construction.** A call that fails
  writes nothing, the captures keep no resolution row, and the next flush
  reads them again. A capture the vendor refuses when asked alone gets a
  `refused` row and is not asked again. This is the three-effect mapping
  the vector work settled on (refused input, server cannot answer now,
  call failed; only the first writes anything permanent;
  [corpus-first retrieval](corpus-first-retrieval-and-research-2026-11-18.md),
  the assessment after four rounds).

**Restarts.** The memory queue goes away. After a crash nothing runs at
boot. The next new capture arms the timer, and that flush reads every ready
capture with no resolution row, including those a crash interrupted. That
is a derived index catching up, the way the vector worker and the BM25
catch-up re-derive their gaps (retrieval design, "Restart: the gap is
re-derived at boot"), and it is more conservative than both, since the boot
starts nothing. A planned restart needs no marker. A capture whose call
crashes the computer twice is refused, by the rule for a schedule whose
tick crashes twice. Whether this reading of the restart rule holds is the
owner's question 2; the default is yes.

### 4.2 Extraction and the candidate shortlist (no model)

The owner's rule (2026-10-26, quoted in
[the problem record](../problems/related-stories-no-entity-layer-2026-10-26.md)):
deterministic extraction before System 1. Jev answers fixed-choice
questions and writes no text ([SYSTEM1.md](../reference/SYSTEM1.md)
section 1), so it cannot extract a name. Corpusd does it, from text it
already holds, and sends no capture text to the computer beyond what the
question shows.

- **Mentions:** `entitytext.Parse` and `Propose` over the capture's title
  and lead passage (the vector's unit), plus the lexicon's `Match`
  (`internal/entitytext`), with the learned stoplist applied. Every mention
  above the extractor's own score floor is asked about. There is no count
  of mentions a capture may have (a fixed question count of ours is a
  defect, SYSTEM1 rule 7); only the vendor limits bound a call.
- **Candidates for one mention, in this order** (Jev leans to the first
  option, SYSTEM1 surprise 6, so the strongest goes first):
  1. lexicon exact key (name or alias), every matching row (`Gazetteer.Lookup`);
  2. near forms (`Gazetteer.Near`);
  3. **token subset** (new): rows whose name contains every token of the
     mention ("Trump" offers "Donald Trump"), ranked by story count;
  4. **initials** (new): an all-capitals mention offers rows whose
     initials match ("UAE" offers "United Arab Emirates");
  5. **neighbours' entities**: rows linked to the capture's 12 vector
     neighbours whose name passes 1 to 4 loosely (same surname, same
     initials). This is the one mechanism serving two jobs: the neighbours
     request the event shortlist already makes gives the entity resolver
     its context, for free, including for neighbours that belong to no
     event (1,074 such were wasted in E1's window).
- **Not now: name vectors.** Embedding "name plus descriptor" for each
  entity would add a second input to wait for and a new embed load. Built
  only if the E3 hand read finds alias misses that 1 to 5 do not reach;
  that is the trigger, not a date.

### 4.3 One call per flush, both questions (one mechanism, several jobs)

**Yes: events and entities share one Jev call per flush.** The SYSTEM1
rules decide it: questions over one state go in one request (rule 1), and a
second call over an overlapping state is the defect whether it runs at the
same time or after (rule 2). The state here is the same captures. Questions
in a call are answered blind to each other, so the entity questions do not
bias the event ones. Splits are by captures, never by question kind: each
chunk carries both questions for its captures, and the code names the
limit (`planJevCalls`, already used at `event_layer.go:382`).

The call's state gains `entities` (each candidate once: name, type, aliases,
descriptor) and each capture gains its lead passage. Per mention, one
choice question:

- choices: its candidates (`n1`, `n2`, ...), then `new_person`,
  `new_organization`, `new_place`, `new_product`, `new_work`, `new_law`,
  `same_as_<k>.<m>` for the same normalised form in another capture of the
  flush, and `not_an_entity`;
- the instructions point at `state.captures.k3.mentions.m2` and carry
  section 2's definition once in `state.task`, never per option (the
  criteria growth that pushed event calls over 64k, `event_layer.go:1078-1081`).

Thresholds (guesses until the E3 hand read; the event layer's history says
start strict):

| Answer | p | Effect |
|---|---|---|
| an existing entity | at least 0.90 | link, role by position |
| an existing entity | 0.50 to 0.90 | link with role `tentative`; counted nowhere |
| `new_<type>` | top choice and at least 0.50 | mint (first of its name: `ent:<name hash>`; when a same-name candidate was offered and refused: the qualified id) |
| `same_as` | at least 0.90 | the two mentions take one entity |
| `not_an_entity` | at least 0.80 | no link; counts toward a stopword, as story answers do (three distinct captures from three outlets, extending the stories rule in `publication_entity_learning.go:10-14`) |
| anything else | | no link; the mention is recorded as unresolved in the tape record |

Same-flush new mints of one normalised name become one entity
deterministically. Two different new John Smiths in one flush are merged
wrongly by that rule; it is rare, recorded (residual R2) and undone by a
split (section 6).

**What the call costs in fate-sharing.** A failed call leaves both the
event and the entity answers for its captures; both are retried by the
next flush (4.1). One mechanism, one failure mode, one retry path. The cost:
requests grow past OpenRouter's "about 32,000 bytes" long-context line,
whose effect on answers is unmeasured (SYSTEM1 section 3.3, item 2). The
intake calls already pass it. Claim C-E5 measures whether the event answers
change.

## 5. How a story gets event ids and entity ids at publish (S8, red)

**Where.** In the host's publish path, right after a version is published
and where story entities are already exported: the `StoryEntityPublisher`
branch that calls `exportStoryEntities` after a successful publish
(`story_entities_source.go:183-186`). No model call and no tape record are
added for events: the derivation is deterministic from the story's cited
captures and their memberships, which are immutable
(`store/world_event.go:15-21`, per the retrieval design).

**Event ids.** An event belongs to a story when at least two of its cited
captures, or 30% of them, are counted members of it (tentative members do
not count); the primary event has the most counted members; a tie carries
both (the retrieval design's rule, kept). The host exports the event
projection (if absent), each cited public capture's `in_event` edge, and the
story's `reports_event` edges. Memberships are read under the read admission
research and Texture already hold (`objectgraph/reader.go:187-188`).

**Entity ids.** Unchanged in authority: research's `story.entities`
record. Changed in input: the analysis's candidates read now includes the
central entities of the story's cited captures, first, so a story and its
sources use the same ids and aliases. The story analysis also stops asking
two calls of twelve (`story_analysis.go:324-349`): one call, every
question, which SYSTEM1 rule 7 already requires (a separate orange change,
named here because the entity work touches the same function).

**What it means for the export and the tape (protected surfaces).** Three
red surfaces: the tape (a new event kind `capture.entities`); the export
to corpusd at publish (new edges and a projection kind); and provider
calls (the event call's request changes). The tape gains only appended
records; no existing record changes meaning. The export is one-way and
re-runnable: a repeat writes the same deterministic edge ids
(`mentionMetadata`, `publication_story_entities.go:73`). **Evidence
class:** production counts from public reads and the flush journal line,
plus a hand read recorded in `docs/evidence`. **Rollback:** revert the
commit; the tape records stay and are inert without the code; corpusd's
capture-sourced edges and projections are tombstoned by a cleanup command
shipped in the same change and run by the lead (re-export restores them,
so the cleanup is reversible). **Heresy delta:** I27 forbids a second home;
the event projection is declared a projection (3.3), and the VM store stays
the events' home.

**A known weakness, stated before it is measured.** With 84% of captures
minting their own event, most stories' cited captures sit in different
events, and few stories will reach the two-capture or 30% rule. S8's
coverage depends on fixing the join rate (section 10), not on S8. Claim
C-E7 says so with a number.

## 6. Who decides merges and splits (S11)

**Merges ride the flush.** When a mention's candidates include two or more
distinct rows that a merge could join (token subset, initials, alias
collision, same Wikidata id), the same call carries one more question per
pair: "do `state.entities.n1` and `state.entities.n2` name the same real
thing?" (choices `same`, `different`, `n1_not_an_entity`,
`n2_not_an_entity`). So a duplicate is asked about exactly when it would
otherwise split a new link, in the call that already holds both, with no
daily sweep, no schedule and no restart question. The backfill (section 9)
walks every capture, so it meets every live duplicate.

- **`same` at p of at least 0.92** (the event join threshold, chosen for the
  same reason: false merges are worse than misses): a `choir.entity_merge`
  record. The survivor is the row with more central story mentions, ties
  to the older; deterministic, so a replay chooses the same.
- **`nX_not_an_entity` at p of at least 0.92:** a merge record with empty
  `into`: the row is retired. This is how "Chinese" and "Health" stop
  attaching. A retired row's name goes on the stoplist.
- **Ids stay stable for readers.** Edges are never rewritten. Every read
  resolves `merged_into` chains when it reads (cycles refused at write).
  A merged id stays a valid address and answers with its survivor, as a
  redirect; a retired id answers "retired", never 404. The minted-id rule
  (a hash of the name) means a later mint of "Trump" produces the merged
  id and resolves to its survivor, so a duplicate cannot come back by
  spelling.
- **Undo.** An owner act tombstones the merge record (as stopwords are
  revoked, `publication_entity_learning.go:11-13`); the reads stop
  resolving it at once, and nothing else changes.
- **Splits are an owner act in this plan.** One id for two things (the
  second John Smith before the qualified-id rule existed, or residual R2)
  is split by naming the new entity and the mentions it takes: the moved
  edges are tombstoned and re-created on the new id, under one
  `choir.entity_split` record. A detector is built (an entity whose
  mentions' vector neighbourhoods or co-mentioned entities fall into two
  disjoint groups) and lists candidates in the status read; Jev splitting
  by itself is not built.
- **The audit trail** is the tape record (the question, the receipt, p),
  the corpusd merge or split row with the record id, and the growth read's
  daily counts of merges, retirements, splits and undos.

**The learning loop this closes.** Each link is a forecast with p. A later
merge, retirement, split or owner undo is an outcome the resolver did not
write (learning engine, rule 1). The status read reports, per p band, the
share of links later undone: the first calibration table for these
thresholds.

## 7. What reads them

**First reader that makes the layer worth having: research's corpus
search.** When a query or a hypothesis names an entity, the search adds the
captures and stories linked to it (and its merged ids) to both halves'
candidates. Name queries are where BM25 and vectors both slip (aliases,
"Vucic" against "Vučić", "UAE" against the full name), and held pages
found are web searches not made: the owner's number is 1,000 a day. It
reads corpusd only.

**The instrument, shipped with the first station that writes links**: public
reads on choir.news, no key, served by corpusd through the proxy, with no
VM in the path (standing question 7):

- `GET /api/platform/entities/{id}`: the row, aliases, `merged_into`, the
  descriptor, counts, and its newest stories and public captures with
  role and p;
- `GET /api/platform/captures/{id}/links`: a public capture's events and
  entities with p;
- `GET /api/platform/events/{id}`: the event projection and its public
  members;
- `GET /api/platform/entities/sample?kind=link|mint|merge&n=50&since=...`:
  a uniform random sample for hand reads, seeded so a reader can repeat it;
- the growth read gains per-day capture counts: resolved, linked, minted,
  tentative, refused, merges, retirements.

`scripts/status-entities` reads these, and nothing else, to answer "is the
join or merge rate right".

**Then, in order:** the story page's "about" list (already served,
`service_publication_read.go:272`) links to an entity page on the web;
related stories read shared central entities across stories and captures
(the walk exists, `publication_related_entities.go`); management's
feed-or-open and the research brief get "held about this entity"; the
Autoradio DJ tells a follow-up when a candidate shares a central entity
with a heard story. The phone's entity page is written in Exact, never
Swift.

**Deliberately not built yet:** Wikidata linking (no fetch path, and 0 of
181 front entities have a QID today); name vectors; automatic splits;
topic threads (`choir.topic_thread`); entity pages for private individuals
before owner question 1 is answered; any entity read from an owner
computer's private graph by anyone but its owner.

## 8. Cost, at the paper's volume

Volumes (M): public captures grew from 215,794 (2026-11-18 22:48Z) to
222,391 (2026-11-19 14:57Z), about 9,800 a day (current-architecture rows
"Corpus search" and "Vector index"). The event layer asked 899 questions in
104 minutes on 2026-11-20 (E1), about 12,400 a day including research
reads, in 96 flushes, about 1,330 flushes a day, 9.4 questions each.
Stories: 100 front updates in 4 h 51 min (M), about 500 a day.

Price (derived from SYSTEM1 surprise 15: $5.49 for 130.8 M input tokens),
about $0.042 per million input tokens.

| Item | Estimate (A) |
|---|---|
| Mentions per capture (title and lead) | about 6 (stories average 11.5 candidates, growth read 2026-11-19: 5,087 over 443) |
| Added tokens per capture: lead passage, mention questions, candidate entities | about 1,000 |
| Calls a day | unchanged: one per flush, about 1,330; plus splits where the 64k limit binds |
| Captures one call holds before the 64k-token request limit binds (the 32k state limit binds later) | about 40 (event part about 350 tokens a capture, entity part about 1,000, shared events list about 5,000) |
| Largest flush seen | 27 captures (E1), so steady state is one call a flush |
| Added tokens a day | about 12 M, about $0.50 a day |
| Merge questions | a few a flush while duplicates exist; falls as they merge |
| S8 at publish | no Jev call |

Vendor limits that bound the batch (SYSTEM1 section 3.3, **[V]**): 64,000
tokens a request, 32,000 for the state plus one question, 255 options a
choice question, no stated limit on questions, no published rate limit.
The gateway's own Jev limiter (`gateway/jev_limiter.go`, recorded in the
retrieval design as contradicting SYSTEM1 rule 4) is not re-read here (U)
and could slow the backfill.

## 9. Backfill

**There is no backfill mechanism.** The ready list (4.1) is the backfill:
every capture without a resolution row is waiting, newest first, and each
flush takes as many as one call holds, then the next. Two differences for
old captures:

- **Entity questions only** for captures older than the event layer's
  horizon (72 hours, `eventRecentHorizon`, `event_layer.go:70`). Putting
  256,000 old captures to the event question would mint about 200,000
  events from captures the recent slot can never reach. That is a
  different question set for a different unit, not a split of one job.
- **The catch-up runs only while the flush loop runs**, started by new
  captures, so it rides the paper's own arrivals and makes no boot path.

Size: 256,000 captures at about 1,200 tokens (entity questions plus the
lead) is about 307 M tokens, about 5,600 calls at about 55,000 tokens, about
$13 (A). One call at a time, at a few seconds a call, is about three to
five hours (A). It waits behind two things: station E1 (vectors present at
flush) so it does not stall on its own leads, and station E3 deployed, so
the hand read of steady-state links comes first and a bad threshold is not
applied 256,000 times. Stories: those reviewed since 2026-10-26 already
hold entities; older ones use the existing owner-act backfill
(`agentcore/story_entities_backfill.go`). Story event ids are derived at
each publish and are not backfilled: old captures have no events.

## 10. What this means for the event layer (a separate change)

1. **Kick the embed worker on a capture write** (E1 here). Expected to take
   `missing_unembedded` from 78% of questions to near zero.
2. **Read the ready list, not the memory queue**, so unresolved captures are
   retried, a crash loses nothing that a later flush cannot pick up, and the
   48 and 24 captures lost in gateway drains would have been asked again
   (4.1). Lands with E3.
3. **Shared entities as an event slot.** For each capture, the recent events
   whose counted members share at least two central entities with it, after
   the vector slot. Entities from the lexicon match are instant, so this
   slot works even when a vector is late.
4. **Siblings first by shared entities.** Captures of one flush that share
   central entities are listed first among each other's `k` choices,
   since Jev leans to the first option and same-flush captures can only
   join through a sibling choice at p of at least 0.92 (the E1 evidence file, section 3, item 3).
5. **Export a public projection of events** (S8, 3.3), so the join rate is
   hand-read through public reads and never again needs a computer key.

Even flushes with every vector minted 66% (E1). So items 1 and 2 will not
reach the design's 60% bar by themselves; 3 and 4 are the next measured
step, and the hand read of 50 events (station E1 of the goal) must be done
through the projection once it exists.

## 11. Stations, smallest first

Review tiers by the contract: green and yellow none; orange one seat (seat
2, DeepSeek v4.1 flash); red two seats (a Sonnet subagent that wrote none of
it, and seat 2). Drills on d@e only; the paper changes only by deploy.

| Id | What | Class | Review | Proof on staging | Rollback |
|---|---|---|---|---|---|
| E0 | **Tonight.** `scripts/status-entities` over the two public reads that exist (front list, growth read): share of stories with entities, attachments per story, Wikidata share, minted a day, alias-pair candidates (token subset and initials among labels), and a uniform sample of 200 central attachments written to a sheet. A reader who did not write this design (a Sonnet subagent with the rubric of section 2, fixed in the evidence file before reading) marks the 200; the verdicts land in `docs/evidence/entity-quality-baseline-2026-11-20.md`. A current-architecture row for story entities. Claims C-E1 and C-E2 entered | yellow (developer script) and green | none | the script's output and the marked sheet; rerunnable by anyone with curl | delete the script |
| E1 | Corpusd kicks the embed worker on each public capture write | orange | one seat | the paper's `op=event_flush` lines over 2 hours after deploy: `missing_unembedded` summed over `queued` (C-E3) | revert |
| E2 | Corpusd: the ready list and entity candidates in the neighbours request (4.1, 4.2); token-subset and initials lookups; `proj_capture_resolution` table; the public reads of section 7 serving existing story entities; `types.EntityRoles` gains `tentative` | orange | one seat | on d@e: the route answers d@e's captures with candidates and `waiting=`; the public entity read serves a paper entity with its stories; no model call yet | revert; drop the derived table |
| E3 | The combined flush: entity and merge questions in the event call, the ready list replaces the queue, the `capture.entities` tape record, the export of capture links and mints; the qualified-id rule replaces the same-name refusal; the event layer's `intake.events` check gone if W1 has not removed it | red (tape, provider calls, export) | two seats | on d@e: a source poll's captures resolved, links readable at `/api/platform/captures/{id}/links`; after the deploy reaches the paper, C-E4 to C-E6 from the public reads and the flush lines; a hand read of 100 links and 50 mints from the sample route | revert; run the shipped cleanup that tombstones capture-sourced edges and resolution rows (reversible by re-export) |
| E4 | Merges and retirements (section 6): `choir.entity_merge`, read-time resolution everywhere entities are read (front, story page, related walk, lexicon, candidates), the owner act that undoes a merge, the split detector in the status read | red (tape, public projection semantics) | two seats | 30 alias pairs picked from E0's list before landing (C-E8); a d@e drill: a forced merge and its undo, read back through public reads | tombstone merge records (owner act); revert |
| E5 | S8: story event ids at publish, the event projection, `in_event` and `reports_event`, `basis.events` filled in the related answer; capture entities first among the story analysis's candidates | red (export at publish) | two seats | on d@e: a published drill story carries its event ids in the public read; on the paper: C-E7 | revert; tombstone projection edges |
| E6 | Remove `research.related_story`: the story layer runs on every computer. Its finish rule applies only to a desk whose story publishes to the paper (the reason it was held, `computer_setting.go:46-50`); the story analysis asks one call, not two of twelve; `CORPUSD_RELATED_ENTITIES` deleted | red (desk finish rule) | two seats | on d@e: a story run records entities with no setting; an owner-computer research round that publishes nothing is not held | revert |
| E7 | Backfill: lift the ready list's horizon cut for entity questions (section 9) | orange | one seat | the growth read's resolved count reaches the capture total; calls counted from the gateway's `site=event_layer` lines (C-E9) | revert; links stay valid |
| E8 | First product reader: corpus search expands an entity in the query to its linked captures and stories | orange | one seat | `scripts/proofs/corpus-search-recall` on its 200 questions does not fall; a name-query set of 30 (aliases, diacritics) rises (C-E10) | revert |

E0 is the one that ships tonight. It is real because it measures the
entity layer that is already in production and was missing from every
account of the system, and its verdicts are written by a reader other than
its author.

## 12. Claims to precommit (enter each in the ledger at its station's landing)

Ids are provisional (`C-E*`); the ledger's next free id is assigned at
entry. Each outcome is a count from production, a public read, or a hand
read by someone other than the forecaster.

| Id | Claim | p | Test | Settle by |
|---|---|---|---|---|
| C-E1 | Of 200 sampled central story attachments, at least 12% are not entities by section 2's rubric | 0.80 | E0 marked sheet | E0 + 1 day |
| C-E2 | Of the same 200, the reader finds at least 10 whose entity has a second id in the lexicon | 0.65 | E0 marked sheet | E0 + 1 day |
| C-E3 | After E1, `missing_unembedded` is at most 10% of `queued` over 2 hours of the paper's flushes | 0.80 | flush lines, `scripts/status-corpus-reuse` read path | E1 deploy + 3 h |
| C-E4 | After E3, at least 70% of the paper's public captures resolved in a day carry at least one counted entity link | 0.70 | growth read | E3 deploy + 1 day |
| C-E5 | E3's flushes make at most 1.1 times the Jev calls of the event layer alone on the day before, and the minted share does not rise by more than 3 points | 0.75 | gateway `site=event_layer` counts; flush lines | E3 deploy + 1 day |
| C-E6 | Of 100 sampled counted capture links, at least 90 are right (right entity, and an entity) | 0.60 | hand read, sample route | E3 deploy + 2 days |
| C-E7 | After E5, at least 50% of stories published in a day carry an event id; under 80% while the minted share is above 60% | 0.55 | public story reads | E5 deploy + 1 day |
| C-E8 | Of 30 alias pairs picked before E4 lands, at least 24 end with one surviving id within 7 days | 0.55 | public entity reads | E4 deploy + 7 days |
| C-E9 | The backfill finishes within 24 hours of E7 with at most 7,000 Jev calls | 0.65 | growth read; gateway counts | E7 deploy + 1 day |
| C-E10 | On 30 name queries, E8 raises recall at ten by at least 0.10 without lowering the 200-question set | 0.50 | `corpus-search-recall` | E8 deploy + 1 day |

## 13. Owner questions, each with the default I would take

1. **Public entity pages for people.** A page that gathers every capture
   naming a private person (a pedestrian struck, a victim) is a dossier.
   *Default:* the public entity read serves people only with a Wikidata id
   or central in at least two published stories; others answer with
   their links from story reads but no aggregating page. Organisations and
   places are always served.
2. **The restart rule and a derived catch-up.** After a crash, the next
   arrival-triggered flush asks about captures a crashed flush never
   answered (4.1). Is that "resuming work"? *Default:* no, it is a derived
   index catching up, like the vector worker; the boot starts nothing.
3. **Spending.** About $13 for the backfill and about $0.50 a day more (A).
   *Default:* proceed without a grant, as the other Jev sites run, and
   report the day's cost in the status read.
4. **Retiring entities already published.** About 17% of the paper's
   central attachments may be retired as not entities, changing what the
   public front shows. *Default:* retire only at p of at least 0.92, every
   retirement undoable by an owner act and listed in the growth read.
5. **The word readers see.** *Default:* "About" on a story page for its
   central entities; an entity page titled by the entity's name, no other
   label.

Not the owner's, decided here: management as the judging desk for
captures; corpusd as custodian; one call; the six types.

## 14. Residuals and what I could not verify

**Residuals.** R1: a restored computer's tape and corpusd's projection
disagree (3.1), no reconcile yet. R2: two new same-name entities in one
flush are joined (4.3), undone only by an owner split. R3: OpenRouter's
long-context mode above about 32,000 bytes may change answers; measured by
C-E5 only for the event half. R4: entities from owner-private captures stay
in that owner's graph; no read of another owner's graph is built.

**Not verified (U):**

- That `research.related_story` is on for the paper: inferred from the
  growth read (443 stories reviewed on 2026-11-19), not read from the
  setting.
- That the five-minute idle wait causes the 78%: a code reading; no trace
  of a capture's vector time against its flush was read.
- That a full re-export can read a channel's whole history (3.2).
- Where corpusd's capture write path is (E1's builder finds it).
- That the corpus brief (retrieval S3) has landed; section 7 does not
  depend on it.
- The gateway Jev limiter's current state (section 8).
- That d@e captures pages through a source schedule, which E2 and E3's
  drills need; if not, the drill adds one source to d@e.
- Every token and cost figure in sections 8 and 9 is an estimate from
  SYSTEM1's figures and the E1 counts, not a measurement.
- `basis.events` is always 0: read from the comment at
  `publication_related.go:185`, not from a live answer.
- The section 1.1 tally: my own reading of one front page, which settles
  nothing.
