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

## Built: the corpus brief (station S3, lane `corpus-brief`, 2026-11-18)

Not deployed. Before a research desk run's first model call the host searches
the computer's own corpus and puts the result at the end of the run's first
message (`internal/agentcore/research_brief.go`, wired at
`runtime.go` next to `managementTaskOpening`). It costs no model turn.

What it does:

- The threads are the objective of the unread Cast (or Ask) record of this
  run's conversation (else the newest with an objective; a clause counts the
  other unread records that name one) plus its typed hypotheses and uncertainties and the `questions` list of its
  packet: the objective and at most 5 more; the rest are named, not searched.
- The threads' searches run at the same time, under the one deadline, one call
  each with no pool, cap or spacing: one `search_wire_corpus` call takes a
  median of 2,096 ms on the paper (p75 3,397, p90 5,423, max 18,399; 39 calls,
  Node B, 2026-11-19 01:30Z to 02:30Z,
  `corpus-search-takes-two-seconds-on-the-paper-2026-11-19.md`), so six in turn
  would answer about three threads and hold every research run for the whole
  8 s. A call still running at the deadline is that thread's "did not answer".
  The log's `ms=` is the whole build and `slowest_ms=` the slowest search whose
  reply the builder read.
- Each thread is one call of the verb's own host function
  (`search_wire_corpus`), as the run's owner and computer, all at the same
  time (one goroutine per thread, a panic in one is that thread's "did not
  answer"), under one 8 s deadline for the whole build (the run-memory and inbox reads included).
  A slow or failing corpusd, a run with no objective, owner or computer starts
  the run with one plain sentence saying why there is no brief; the run always
  starts. Per thread the brief says one of three things and nothing more: the
  pages and stories found, or "Nothing found by the host's search.", or "The
  host's search did not answer for this thread." (the call errored, timed out,
  was cut by the deadline, was never reached, or its result could not be
  decoded: one binary fact known at the call site). It does not say what a
  search covered and never says that the corpus holds nothing. The outcome is
  `delivered` (a page or story is in the brief), `empty` (the brief holds no page or story;
  `unanswered=` says how many threads did not answer), `failed` (no
  thread answered), `skipped` or `dropped`. A field over its clip (title,
  passage, snippet) ends `[cut]`; a url, route_path or id over its limit is
  omitted, never cut.
  Thread text is searched as plain words (the verb's `since:`, quote and
  `-word` operators are neutralised).
- It delivers at most 6 captures (with dates, capture ids and the matching
  passage) and 5 published stories a thread, and 8,000 estimated tokens in all
  (it rides on every turn; the verb gives more). Over the bound whole threads
  are dropped from the last and named.
- A resumed run builds nothing: the first message is replayed from run memory.
- Labels: the labels of the record the objective and threads came from, and
  what the verb gives for the pages delivered (the capture floor when a passage
  is shown, world otherwise), marked only when the run memory took the text as
  a fresh start, on every outcome that put record text in the message. The run-label union feeds publish admission and the
  disclosure gate, so an owner's computer may see the capture label on more
  research runs (the v16 prompt already asks the desk to search the corpus
  first, which gives the same label).
- The ledger is not seeded: brief pages are not pages the run searched for, so
  the "has not tried to read" count and the `search run=` line speak of web
  searches only.
- One journal line per research run start: `research brief run= desk= ms=
  slowest_ms= threads= queries= stories= captures= passages= newest_held= bytes=
  tokens_est= dropped= left= unanswered= outcome=(delivered|empty|failed|skipped|dropped)
  reason=`, written once the run memory has taken (or not taken) the text:
  `dropped` means the desk never saw the brief; `unanswered` counts the threads
  whose search did not answer (0 where no thread was ever known, as in an
  opening that skips; left-out threads are in `left`). A build in which a
  search panicked also writes one panic line (the run id and the count, no
  query text). The
  `corpus search` line gains `caller=brief` or `caller=cell` at its end.
  "Runs that made a web search before any corpus read": the first `gateway:
  search request ... run=` against the first `corpus read run=` or
  `corpus search ... caller=cell`.

What it cannot do:

- It cannot judge a story stale. A published story carries no date in the
  corpus answer (`PublishedPiece`), so the brief says so in one clause and
  shows dates only for held captures. "Stale" waits on story dates or event
  ids (S8).
- Each thread is three searches (captures, published stories, events), not one
  neighbourhood read; the brief shows captures and stories, not events.
- Query quality on long objectives is unmeasured: the text half takes every
  word of the query with term caps and pruning, and the benchmark queries were
  12 words.
- Whether the delivered brief lowers web searches is not measured; the claim
  is entered separately.
- A decodable answer whose halves did not run (no page-text answer, the
  window fallback, a capped published scan) reads as "Nothing found by the
  host's search." The brief does not describe what a search covered; the
  prompt's one corpus search in the desk's own words, for any thread the brief
  found nothing on, covers it.
- A search that panics after the deadline is not logged as a panic (its thread
  still reads "did not answer"); the panic value is formatted with `%v`.
- The child Report label gap above is not touched.

## Why the brief does not describe its search (rounds 1 to 6, 2026-11-18)

Six review rounds found one defect in new forms: the brief said "the corpus
holds nothing" when part of the search had not run (a failed half, a deadline
cut, an index still catching up, page text never searched), and from round 3 on
each repair added a typed description of what the search covered (a `coverage`
object in the verb's answer) that was itself hard to make exactly true in the
fallback paths (the window of the newest captures, a thread of stop words,
Jev's labels read from a separate store, a capped published scan, pages the
full-text index never holds).

Round 6 changed the approach: the description has no reader who needs it. The
desk is told to make one corpus search in its own words for any thread the
brief found nothing on, whatever the brief says it covered, so the scope
statement was cost with no use. It was deleted, with the `coverage` object and
everything added to the verb for it. The verb's result and notes are as they
were on the lane's base, plus only `caller=` on the `corpus search` log line.
The brief says what it found, that the search found nothing, or that the
search did not answer. A test sweeps every render path for absence and scope
phrases so neither can return unnoticed.

## `candidates_shown=0` read from the traces, 2026-11-19 (station L1; read-only)

The paper's console, 02:27Z to 05:22Z, holds 233 `management story choice`
lines: 157 feed an existing story, 76 open a new one. **186 of the 233 (80%)
read `candidates_shown=0`**: 139 of the 157 feeds and 47 of the 76 opens.

The count does not mean what the line's name says. `candidates_shown` is
incremented only in the `choir.Stories` verb
(`internal/agentcore/intake_view.go:160`, `noteStoriesShown`). A management
task does not call the verb: the host puts the newest page of the story
library in its first message (`managementTaskPrompt`, `managementTaskOpening`,
`management_tasks.go:68-196`, "as choir.Stories(50, \"\") return[s] them"),
and delivering that page calls `noteStoryLibraryShownAt` and
`noteEgressRead`, not `noteStoriesShown`. So 0 is "this run made no
`choir.Stories` call", which is what a run does when the opening already held
the page. The evidence that the opening carried candidates is that 139 runs
fed a story by its id without a `choir.Stories` call: a model cannot supply a
document id it was never shown. The 47 opens with 0 are what a run does
that saw the page and found no match, or that was shown the "did not come
with this message" note and did not read the page; the line cannot tell the two
apart, and nothing logs the note (`openingStoriesNote`).

Cause: **harness** (the instrument, not the model): the line undercounts what
was shown. Confidence 0.85 for "most zeros are opening-shown", from the
139 feeds; 0.5 that none of the 47 opens followed a missing page. What the
management run does after the line is `choir.Done`; the line is written when
the record commits.

What would settle it: add the opening's story count to the line (a count, an
ids-free change, yellow), then read the opens: how many of the zeros had a
page of 50 and chose to open anyway, and what their headlines overlapped. The
comparison that tells the model from the harness: the same capture batch to
the same model with the page in the opening, with the page absent (the model
must call the verb), and with the page cut to the stories whose titles share
a term with the captures; and the same with another model. The choice between
feed and open is then compared to a human reading of overlap, not to the
line.

Seen on the way, in the new record
([recurring agent failures read from traces](recurring-agent-failures-read-from-traces-2026-11-19.md)):
`choir.Stories` paging is refused by the broker's own bound (the cursor is 67
bytes and the bound is 64), 4 times in 3 hours, so a management run that
wants the second page of candidates cannot have it.

## The brief, measured on the paper (2026-11-19T06:45Z)

The corpus brief deployed at 05:19Z. Read from the paper's console from
05:39:52Z:

- **It is delivered.** Of the first 30 briefs, 27 were delivered and 3
  failed the 8 second deadline; 21 had every thread answered. Median 5.9 s,
  median four threads, about 5,900 tokens. (Ledger C141, true.)
- **The model searches the web as much as before.** The first 24 search
  summaries have a median of 4.5 web searches started (mean 7.3, most 40),
  against 4 before the brief; 4.9% of returned pages were read. (Ledger
  C142, false, entered at 0.3.)

Belief now: giving the model what the computer holds, unasked, does not
change what it asks the web for. That fits the trace read's finding that the
many-search runs are the model rewording one question
(`recurring-agent-failures-read-from-traces-2026-11-19.md`). Remaining
error: the window crosses a second refresh at 06:16Z (the faster corpus
search), two dozen runs is a small sample, and whether the stories the runs
wrote stood on held pages more often was not read. What would tell: the
same read over a full day, and the calibration experiment's model arms.
The brief costs one corpus search of up to three threads a run and no web
search; it stays.
