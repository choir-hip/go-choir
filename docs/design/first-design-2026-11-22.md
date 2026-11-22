# The first design: Autopaper and its readers

Draft, 2026-11-22. Status: **idea, in conversation with the owner**. Nothing
here is built or ratified. It records the owner's direction of 2026-11-22 in
their words, the lead's reading beside it, what the code does today (from the
trace of the same day), and the questions still open. It is not doctrine; when
ratified its rules move to `choir-doctrine.md` and this file is deleted or cut
to the build plan.

## Why now

The owner, 2026-11-22: "we are approaching a fundamental redesign. not a
refactor but a redesign... the system works at a prototype level, and this
will let us get to production. in some ways, this is the first design. like,
every version of autopaper and autoradio up to now was a test to prove it
could be done... now weve succeeded. so now its to make it nice".

The objectives, in the owner's words: "to cover the world's stories, to
personalize into individual autopaper editions, and serve as a foundation for
autoradio, while using minimal resources enabling scaling of sources and
agents".

## What the trace found (2026-11-22, code and the paper's console)

- Three mechanisms ask which existing thing a capture belongs to: Jev's intake
  label (`intake_labels.go`), the event layer's Jev call a minute later
  (`event_layer.go`), and a management task's model session with vector
  candidates (`management_candidates.go`). The third ignores the second.
- Event memberships are written and read only by the owner's event view and by
  research's corpus search. No routing, front page or Autoradio code reads
  them.
- A management task is one model session for up to 12 captures; four run at
  once through a dispatcher.
- Roughly 1,000 model calls an hour for roughly 100 captures judged: about ten
  calls a capture across all desks. The split by desk is not logged.
- Research's sub-calls (`choir.Research`, up to 16 children) were used by none
  of 27 runs; the research prompt never mentions them. No other desk has
  sub-calls.
- The root Texture exists behind a setting that defaults off, was sent back by
  review six times, and is not on the capture path
  (`problems/texture-root-desk-six-reviews-no-landing-2026-11-20.md`).
- A personal edition does not exist as a paper; "My edition" is the notebook.
  Autoradio's candidates are the public front page, sent up by the phone.

## The objects

The owner: "stories have many events"; "stories, events, and entities all have
many of each other".

| Object | What it is | Many-to-many with |
|---|---|---|
| capture | one page read from one outlet at one time | events (a capture may report several) |
| event | something that happened, at a time and place | captures, stories, entities |
| entity | a person, organisation or place | events, stories |
| story | a narrative across events, with a main point | events, entities, other stories (succeeds) |

Rules the owner stated:

1. "this is one key way how we can compare captures. by the events they
   cover." Two captures are alike to the degree they report the same events.
2. "we shouldnt have multiple stories with exactly the same events." A story's
   identity includes its event set; two stories with the same set are one
   story.
3. "the importance of events should match the quantity of stories the event is
   involved in." Importance is read from the graph, not assigned.
4. Earlier, still in force: "a story stops growing when the main point, the
   main beat changes"; "revising not making new stories"; unclear cases lean
   to revising.

Lead's reading: events accumulate in a story until one arrives that changes
its main point; that event begins the successor. Rule 2 leaves open what to do
when one story's events are a subset of another's (a part of a larger story,
or a duplicate in the making): open question 4.

## Conventions: what is an event, an entity, a story

The owner, 2026-11-22: "events can span over time and space. locality is a
fallacy... the observation shapes the system inherently. like, is the iran war
an event, an entity, or a story? its a story, comprised of entities (which
include abstract entities like governments or militaries. but strategies...
like 'the blockade of the strait of hormuz'... is that an event or an entity?
its background of many stories. its a story. yes."

"stories, textures, should become much more recursive, more frequently citing
and transcluding other stories. they cite out to the web a lot, but now that
we have a corpus of coverage, now is the time to use it. indeed, the whole
point of choir is that we can be self-referential. textures reference textures
is the essence of it. and because a texture story encapsulates entities and
events, we can traverse the story graph in addition to doing vector semantic
similarity operations".

What this changes:

- The ruling of 2026-10-20 ("one event is one incident, one place, one time",
  the text Jev is given in `event_layer.go`) no longer holds as written: an
  event may extend in time and space. Why that ruling was made still matters:
  the wording before it ("or its direct development") turned a developing
  story into one event. Stories that contain stories now carry that load.
- A story is the composing object. A story cites and transcludes other
  stories; the war is a story, the blockade is a story inside it and in
  others. Story to story is an edge of the graph beside succeeds.
- Entities include abstract ones (a government, a military).
- The graph is traversed through stories: a story holds its events and
  entities, so walking story to story reaches them.

Lead's reading, not ruled: an event is what captures report in common, the
unit of observation; a story is our written account, which may be made of
other stories. When we come to describe the parts of an event separately, it
has become a story. Open question 8.

What exists: the object graph registers `choir.publication_transclusion` and
`transcludes` edges (`internal/objectgraph/registry.go`); how often a
published story cites or transcludes another is not measured.

## Two questions, each asked once

| Question | Kind | Unit | Who |
|---|---|---|---|
| Is this capture a report of events we hold? | factual, high precision | capture | one batched Jev call per poll, vector neighbours as candidates |
| Which stories does this event belong to, and does it move one? | editorial | event | the root Texture |

The event layer is kept and read. Per-capture routing by a management session
is removed. The intake label's "same event" and the event layer's join become
one call. The editorial judgment runs on events, so twenty outlets on one
strike are one decision.

## Desks have one endpoint

The owner, 2026-11-22: "graph traversal should make it easier for autoradio
desk (which needs to become a go rlm anyway) to compose. but hmm, how does
autoradio desk connect to root texture? indeed, i think it shuold connect to
root texture, not to individual textures. thats the whole desk pattern,
actually, that the desk, regardless of how many agents in it, only has one
endpoint to other desks..."

- A desk is addressed at one endpoint however many agents run inside it. For
  the Texture desk that endpoint is the root Texture; story Textures are its
  sub-calls and are not addressed from outside.
- The Autoradio desk becomes a desk that runs Go cells (today it is two server
  routes, one streamed model call each) and talks to the root Texture, never
  to a story's Texture.
- Lead's reading, not ruled: the same holds in the other direction. A story
  Texture that needs research asks through its root, so the root sees every
  request, can merge two stories' requests for the same gap, and can spend
  research by its forecasts. Research is a desk with its own single endpoint
  and its own children. This would answer open question 2.

## How context is found

The owner, 2026-11-22: "we can run captures through vector search, then for
each match we can do graph traversal. this is then sampled and provided with
metadata and full data accessible in repl".

One retrieval shape for every reader (the capture-to-event call, the root
Texture's digest, a story Texture, a research run, a personal edition):

1. **Seed by vector.** The capture, event or question is embedded and its
   nearest neighbours found.
2. **Traverse the graph from each match.** Capture to its events; event to its
   other captures (other outlets), its stories and its entities; story to its
   main point, its events, its predecessor and successor.
3. **Sample.** A large neighbourhood is cut by a stated rule (distinct
   outlets, newest, most connected), and what was left out is counted.
4. **Hand over metadata, keep the data reachable.** The context carries each
   item's id, kind, date, outlet and how it was reached, with compressed
   substance for the nearest; the full object is one REPL read away, with no
   web fetch.

The research digest in flight is the first use of this shape, on the edges
that exist today.

## The Texture tree

The owner: "wouldnt that perhaps make sense with autopaper? giving a global
perspective on the paper. and architecturally, the point is that each texture
agent would be a sub-rlm of the root texture. enabling messaging. so, the root
texture would be in charge of predicting the future of the autopaper. it would
do much of what management is doing now. and for individual users, it would be
similarly tracking a computer-owner model, and have comms with subtextures";
"concurrency is to be achieved by desk subcalls".

- **Root Texture (the paper).** Holds the whole view: the stories, their main
  points, what is developing. Works on a digest of new events, never on raw
  captures. Decides which stories an event joins, when a main point has moved,
  what to pursue. Its forecasts are learning-engine claims (an event will
  follow within a day; a story needs a second outlet), scored, and the scores
  steer where research is spent.
- **Story Textures.** Sub-calls of the root. Parent and child message through
  the call. Several run at once.
- **Research.** Runs for a named gap, opening with a digest from the object
  graph (the change in flight, `problems/read-first-reads-more-but-searches-as-much-2026-11-22.md`).
  The owner: "the goal of any run shouldnt just be to write the best story,
  but to be building on and using our corpus, and then the story adds to our
  corpus"; "we shouldnt need rules... prompts... a lot of compressed context
  from the object graph".
- **Root Texture (a person's computer).** The same shape with a model of the
  owner in place of the world: what they read, heard, skipped, asked. A
  personal edition is its selection over the shared graph. Autoradio reads
  from it.
- **Management.** Returns to computer-wide coherence and admitting
  engineering.

Why the earlier root desk failed review and this need not: it stood outside
the documents and caught up by tailing their commit log (all three open
defects are in that log's handling). Children that are the root's own
sub-calls report to it directly.

## Backfill

The owner, 2026-11-22: "we will need to backfill all this data. we should
think about how to do this. using system1, and we can give jev an 'i dont
know/more/other' button which calls an llm? other data backfill ideas?"

The paper holds about 222,000 captures and 35,000 stories (2026-11-19 counts),
few events and 6,690 story entities of poor quality. Ideas, none decided:

1. **Derive before asking.** A story's events are the events of the captures
   it cites; an event's entities come from extraction over its captures. No
   model.
2. **Cluster by vector first, confirm with Jev.** Captures near each other in
   a time window are candidate events; Jev answers "same happening?" for whole
   clusters in few large calls (the vendor's limits bind, `reference/SYSTEM1.md`),
   never one call a capture.
3. **An "other / cannot tell" option on every Jev question, which escalates to
   a language model.** Jev picks and writes no text, so naming a new event
   also needs a model or a rule (the central capture's headline). The
   reference page's cautions apply (`reference/system1/calibration-and-cascades.md`):
   the escalation test is the whole cascade, and a random share of the answers
   Jev did NOT escalate must also be labelled, or its accuracy is never known.
4. **Newest first, and on demand.** Resolve what traversal touches; a capture
   no reader reaches may never need resolving.
5. **Entities by a public identifier.** Canonical ids (Wikidata) merge
   "U.S." and "United States" by rule; Jev only for the ambiguous.
6. **Offline, then import.** As the vector backfill was: computed off the
   paper's computer, checked on a sample, imported in one step.
7. **Derived tables only.** The backfill writes projections that can be
   dropped and rebuilt, never canonical records, so a bad pass is redone.
8. **A hand-read sample and a ledger claim before it is trusted.**
9. **Outside seeds, to be weighed:** the outlets' own article metadata;
   public event lists.

First output worth having: the count of existing stories that share an event
set (the owner's rule 2), which measures the duplicate problem directly.

## Cost budget (to be set from measurement)

The marginal cost of a capture must approach zero for sources to scale: an
embedding and a share of one batched call. Agent sessions are spent per event
that changes a story, not per capture. First step, before any deletion: log
the desk on every model call so calls per capture, per event and per published
story can be read by desk.

## What would be deleted

Per-capture management tasks and their dispatcher; the intake label's
duplicate "same event" question; the search refusal gate and its escape (in
flight); the root-desk commit-log digest; the `texture.root_desk` and
`research.related_story` settings (no switches); `proj_capture_resolution` if
entities attach through events.

## Open questions

1. Is the root one long-lived session, or a fresh session per digest that
   reads its own document to remember? (Lead leans to the second: cheaper,
   survives restarts, fits "a crash restart never resumes work".)
2. Is research a sub-call of a story's Texture (root, story, research), or a
   desk the story Casts to as now? The contract says desks talk only through
   precommitment records: does a sub-call carry one?
3. Do entities attach to events first, with stories inheriting them?
4. Subset event sets: part of a larger story, or a duplicate to merge?
5. Who owns the event kind? Today the computer is the events' custodian; under
   one-custodian-per-kind the root Texture's desk or a graph desk must be
   named.
6. How a personal root reads the shared graph without a private computer's
   data crossing owners.
7. The order of the build, and what the paper runs meanwhile.
8. What separates an event from a story, now that an event may span time and
   place: is an event the unit captures report in common and a story our
   account of it, an event becoming a story once its parts are told apart?
9. How often stories cite or transclude stories today, and what the writer's
   prompt and verbs make easy.
