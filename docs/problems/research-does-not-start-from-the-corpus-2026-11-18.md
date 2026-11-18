# Research does not start from the corpus (2026-11-18)

## Problem

The owner, 2026-11-18: "Are we correctly searching the corpus? More stories
should be citing existing stories and captured sources. Need to check for
story duplication, but not only duplication. Too much story overlap is
wasteful." And the principle: "research should substantially be upon our
existing corpus, because as the corpus grows, the likelihood that relevant
sources in the corpus exist for any new bit of information is very high. And
then the search should be all on the diff between the existing corpus and
the world." The owner asked for an audit of graph traversal and BM25 as
built, and a feasibility study of embeddings and vector search.

This record is the audit. The search-volume record
([search-volume-outruns-keenable-credits](search-volume-outruns-keenable-credits-2026-11-18.md))
holds the owner's other words on this and the web-search numbers.

## Evidence (read-only audit: code on main, the gateway journal, two hours of the paper's console log, public routes)

### What the corpus search covers

| Layer | Size | How it is searched | What is searched | Fresh |
|---|---|---|---|---|
| Published stories | at least 1,584 updated in three days; archive total not known | BM25 over an in-memory index in corpusd (`platform/publication_pieces_index.go`; k1 1.2, b 0.75; a title word counts three times) | title and full body of the latest version | at once |
| Captured source pages | 206,997 on the paper's computer | a term table (`platform/capture_index.go:176`) | **title, host, source id, language and Jev's labels only; the page text is not indexed** | after each capture |
| World events | newest 3,000 | substring match (`corpus_events.go`) | event headline and member titles | at once |
| Evidence saved by runs | not measured | listing only | nothing | |

- Stories: the model's query goes to BM25 with any-word matching; plurals
  are folded lightly; up to 25 results, 10 by default.
- Captures: at most eight words kept; a page matches when half its query
  words are in its **title or labels** (all of them for a one- or two-word
  query). A fact that appears only in a page's text cannot be found. A
  different wording of the same event cannot be found.
- **The graph is not available to research.** The one traversal, "related
  stories" (`platform/publication_related.go`: story to cited page to other
  stories citing that page, plus research-judged pairs and shared entities),
  is called only by reader routes. Research can write a relation
  (`RelateStories`) and cannot read the graph.
- Tests prove that stories and captures are found and failures reported.
  There is no recall or known-item test.

### Whether it is used

- Research does call it: 384 corpus searches in two hours, against about
  700 web searches an hour, so about 3.3 web searches per corpus search.
- 22% of corpus searches returned no captured page. 22% returned exactly
  the default ten.
- **A hit does not stop the web search.** After 76% of corpus searches that
  found captures, the same keywords were web-searched within ten minutes, a
  median of six times.
- The log line counts captures only. How often a published story was found,
  and which run asked, is not logged.

### Whether stories build on what is held

- **0 of 58** sampled stories cite an earlier Autopaper story.
- 40 of 58 share at least one cited URL with another sampled story; 18 cite
  only sources nothing else cites. Of 363 distinct cited URLs, 29 are cited
  by two or more stories.

### Duplication and overlap

- Of 1,584 stories updated 2026-11-15 to 2026-11-18, clustering by title
  and standfirst finds 84 clusters holding 200 stories (12.6%): 116 stories
  (7.3%) beyond the first of each. This is a lower bound; titles that word
  an event differently are missed.
- Examples: an actor's casting, four stories over two days; a country's
  measles status, four stories in seventeen hours; a bank outage, four; a
  freeway ban on driverless cars, four.
- Later stories of a cluster mostly cite **different outlets** on the same
  event: of 29, none shares 80% of its URLs with the earlier story (median
  20%). Overlap of facts cannot be measured from URLs. It needs the text.
- Revision is in heavy use inside a story: 87% of stories have more than
  one version, the median story is at its sixth.
- Where the existing story is known today: intake has Jev compare each
  capture with story headlines of the last eight hours
  (`intake_labels.go`); the management task sees fifty headlines with
  standfirsts and chooses to feed an existing story or open a new one;
  research, once started, can find earlier stories but can only record a
  relation; the story desk revises only its own document; nothing checks at
  publish. The choice to feed or open is not logged.

## Belief

1. The corpus search is lexical and shallow where it matters most: 207,000
   captured pages are findable only by their titles. The corpus cannot be
   what research stands on until page text is searchable and a different
   wording of the same event can match.
2. Research treats the corpus as one more search and not as its starting
   point. Nothing tells it what it holds, and nothing makes the web search
   a search for what is missing.
3. Overlapping stories open because the only stage that can choose "feed
   the existing story" sees headlines, early, from a recent window; the
   stages that later learn the overlap (research, the story desk) have no
   way to act on it. The owner (2026-11-18): the choice cannot be made
   before research; where it belongs is a design question, not settled
   here.
4. What lexical search cannot fix, from the paper's own stories: the same
   event in other words ("Third US carrier reaches Middle East" and "USS
   Theodore Roosevelt arrives"); spelling of names (Mira and Mirra, Ahmed
   and Ahmad); a French or German source against an English story. This is
   the case for embeddings, under study the same day.

## Direction from the owner (2026-11-18, after the audit)

"Wow, you say pages are not indexed, only headlines. Ah, that's the big
problem. So I guess we'll need to use vector embeddings to index the pages.
I mean, can we use BM25 for that as well? we should be thinking resource
aware, but I think we should be able to do hybrid vectors and BM25 and graph
traversal across entities and events. And time windowing."

"keep looking into running embedding Gemma 2, we should be doing full page
indexing for both BM25 and for vectors. And if we can't do the embedding on
node B, maybe we can do it on node A, or we can rent a big AWS machine for
it. But I think we can do it on our own existing infrastructure... And we
can also think about running it on the Mac for the initial indexing, if that
would help."

"But yes, we should be doing full page indexing for everything with BM25
immediately as well."

On linking stories by event, which the lead had said the radio lacks: "I
thought we had been doing that for a couple weeks at least. We should look
into this because there may be some code on an old branch or something
half-built or something weird going on, because we should already be linking
stories by events, by entities, etc." The audit above did find an events
layer, labels and a related-stories walk on main; what it found missing is
any reader of them in research, at the feed-or-open choice, on the front row
or in the radio. The lead's sentence was too loose. A separate
investigation is finding everything built, half-built or dropped.

What is being done:

- Lane `capture-fulltext`: every captured page's full text in the corpus
  search by BM25, with time windowing, on passages the vector index will
  share. Old title search stays until the backfill completes.
- The embeddings measurement now covers full pages, Node B, Node A and the
  Mac for the first indexing (claims C132, C133).
- The investigation of event and entity linking, as above.
- The target the owner named: hybrid retrieval over full pages (BM25 and
  vectors), graph traversal across entities and events, and time windows.
  Its design waits for those three results.

## Event and entity linking as built (investigated 2026-11-18, main at 60226a031)

The owner expected stories to be linked by event and entity already. They
partly are. Three mechanisms run, each in a different place, and they are
not joined.

| Mechanism | Links | Written by | Read by | State |
|---|---|---|---|---|
| World events (`store/world_event.go`, `agentcore/event_layer.go`; 2026-10-15) | a captured page to an event; an event to the one it follows | Jev, on source-poll captures (join at p 0.92 or more, tentative at 0.5, else a new event) | research's corpus search (newest 3,000 events, substring match); an owner route | running. **A story carries no event id anywhere.** Production by event (`intake.event_wakes`) is held off: it failed its false-merge bar three times |
| Capture labels (`store/capture_label.go`, `intake_labels.go`; 2026-10-13) | a captured page to the story it reports the same event as | Jev at intake, against stories of the last 8 hours plus the newest 30 | the management inbox ("same event as story X"), capture search | running; this is the only overlap check at feed-or-open |
| Story entities (`story_entities_default.go`, `publication_story_entities.go`; 2026-10-26, on front rows since 2026-11-10) | a story to an entity, central or incidental | the research desk's answers, exported at publish | the front row and resolve payload (`entities`); the related-stories walk | running, behind the setting `research.related_story` (default off, on for the paper) |
| Related stories (`platform/publication_related.go`; 2026-10-19) | computed, not stored: shared cited pages, judged pairs, shared central entities, title-word filler | | one reader route, host only | running; `basis.events` is always 0 |

- Nothing was removed by a refactor and no unmerged branch holds linking
  work. What is missing was never wired: stories to events; any reader of
  entities or the related walk in research, at the feed-or-open choice, in
  drafting, at publish, in the front-page score, or in the radio.
- Production, public routes: 79 of 100 front rows carry entities; none
  carries an event or cluster id. The related route links same-event stories
  by entity where entities exist (a Yemen group of four: yes) and falls back
  to title words where they do not.
- **A fifth of stories have no entities** (21 of 100 front rows). The lead
  first tied this to 664 refusals in corpusd's log ("is not a published
  Autopaper story"); that was wrong. The refusals are 656 distinct stories,
  nearly all refused once: the desk's first entity write arrives before the
  story publishes and is expected to be refused; the export at publish
  follows. The candidate cause is elsewhere: the export at each published
  revision rebuilds the entities from the newest 10,000 events of a channel
  (`system1ChannelTape`, `ListAcknowledgedEventsByChannel`), so on a busy
  computer an older story's record falls out of the window. In a sample the
  stories without entities were the many-version ones (3, 4, 6 and 25
  versions). **Refuted the same day by reading the chain**: an export that
  finds no record sends nothing (`story_entities_source.go:229`), and the
  front reads story-level edges that a later revision cannot remove
  (`publication_story_entities_read.go`), so the window can only block a
  first export, and nothing shows first exports failing. The cause is not
  known. Untested and cheaper: those stories have only incidental entities
  and the front shows central ones. What would settle it: for the stories
  with an empty list, whether a `story.entities` record exists on the
  paper's tape and what roles it holds. No repair was built.
- **Entity ids are not merged**: "U.S." and "United States", "Trump" and
  "Donald Trump" are separate, so groups split; generic entities ("Drone",
  "Woman") join unrelated stories.
- **Events seldom cluster**: in the sampled window 63 of 74 captures minted
  a new event and 3 joined one.
- No call to `RelateStories` appears in the logs.
- For the radio: a different story on an event already heard can be
  proposed today from shared entities (each front row carries them). It
  cannot be looked up by event id: events live on the paper's computer and
  no story points at one.

## Remaining error

- Share of a story's sources already held before its research began: not
  measurable. Needs the run id on the corpus-search and web-search lines
  and the capture id on each read.
- Why each duplicate opened: needs the feed-or-open choice logged with the
  labels it saw.
- Rank of a known story for its own title: needs a guest call to the
  search the desk uses; no public route runs it.
- Exact duplicates were not separated from updates and overlaps: first
  publication times were not available from the public routes.
- Fact overlap between stories of a cluster: not measured.

## Update, 2026-11-18: page text is searchable in the lane `capture-fulltext`

The owner (2026-11-18, on reading belief 1): "we should be doing full page
indexing for everything with BM25 immediately as well", and hybrid vectors,
graph traversal and time windowing after. Built in the lane, not deployed:
a BM25 index over the passages of every captured page's text, with time
windows and a recency order, merged with the old title-and-label search
([reference/CAPTURE-FULLTEXT.md](../reference/CAPTURE-FULLTEXT.md): facts,
the four engines measured, resource projections, the catch-up). This record's
beliefs 2 to 4 (research does not start from the corpus; overlap is chosen
before research; wording and language need vectors) are untouched. The
measurement the ledger asks for (claims C132, C133: story headline as query,
the pages the story cites as the relevant set) is not run; the index is what
it needs.

## Design proposal (2026-11-18)

[design/corpus-first-retrieval-and-research-2026-11-18.md](../design/corpus-first-retrieval-and-research-2026-11-18.md):
the event as the unit and a story as one telling of it; vectors beside the
BM25 passages; every captured page through the event layer; story event ids
at publish; research that reads a corpus brief first and searches the web
only for the gaps; the new-or-revise choice at a new story's first publish.
A proposal with stations and the owner's decisions; not ratified.
