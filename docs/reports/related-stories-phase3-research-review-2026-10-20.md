# Related stories, phase 3: the research desk reviews the graph, Jev classifies

Design, 2026-10-20. Extends
[related stories, computed from the graph](related-stories-graph-design-2026-10-19.md)
(phase 1 shared captures, live; phase 2 events, waiting on the
one-incident blind rerun). Owner asks, owner notes §28: "more connections
between articles … review all the existing articles and make sure that
they are correctly connected in the object graph when they touch on
similar topics, events, and entities … with a desk like the research
desk"; "We are not yet using the embeddings … I want to really use the
object graph effectively first"; "Jev can also do a lot of this
classification. maybe we should give research the ability to use Jev in
addition to management"; "Even if management isn't using Jev as a
tool, then research should."

Status: design, not built. Mutation class: red.

## 1. Summary

Two stories are related when the graph has a path between them through
something they are both about. Today the only such path is a shared
capture, so 9 of the front 60 stories have links. This phase adds the
missing subject nodes (entities and topic threads) and a reviewer that
attaches every published story to them:

- **Nodes.** `choir.entity` (person, organisation, place, product, work)
  and `choir.topic_thread` (a running subject that spans incidents), next
  to the existing `choir.world_event`.
- **Edges.** `mentions_entity` (story version to entity, with `role:
  central | incidental`), `in_thread` (story version to thread), and
  `related_story` (story to story, with a typed relation and Jev's
  probability).
- **Reviewer.** The research desk, woken by a standing review schedule,
  reads each story, names its central entities and its thread, and asks
  Jev the closed questions. Its judgments are semantic acts on the paper
  computer's tape, exported one way to Store B.
- **Candidates without embeddings.** Pairs to judge come only from graph
  walks: shared captures and recaptures, shared events, shared central
  entities, the same thread.
- **Query.** Same incident, then development, then same thread, then
  strong shared entity. A pair joined only by an incidental entity never
  shows.

## 2. What exists, and what is missing

- **Jev** is the gateway-pinned choice judge
  (`internal/gatewayruntime/jev.go`): a caller sends choice questions,
  gets the full answer distribution back, and interprets it itself; the
  model is not caller-selectable. Three runtime-side judges call it: the
  intake labeller (`intake_labels.go`), the event layer
  (`event_layer.go`) and the support judge (`support_judge.go`), with cost
  recorded by `recordJevCost`.
- **No desk calls Jev.** Management reads Jev's intake labels on its
  captures records ("advice not a decision", management overlay) and
  research sees them as hints in `SearchWireCorpus`. Neither can ask Jev a
  question of its own. This design adds that verb.
- **Research is read-only world evidence.** It is cast by Texture and
  answers Texture through `Report` / `ReportPacket`. It has no standing
  work and no act that records a judgment about published objects.
- **No entity or thread objects exist.** Phase 1 left the explicit
  "follow-up of" edge and story-level topics as open (design §12).

## 3. The two new desk verbs

### 3.1 `choir.System1(questions)`: a closed question, answered with a distribution

Named by the owner (§28): "rather than choir.jev, we should make it
choir.system1". System 1 is the fast, intuitive judge: it answers a closed
question with a distribution. The desk's own reading and reasoning is
System 2, and it decides. Jev is the model behind System 1 today; the verb
does not name it, so the backing judge can change without changing desks.

For research (owner, §28: "Even if management isn't using Jev as a tool, then research should"). A cell passes up to N choice questions (each:
id, text, choices, optional context refs); the runtime sends them through
the existing gateway transport in one call, records the request digest,
the answer distribution and the cost on the tape, and returns the
distributions to the cell. It never collapses a distribution into a
decision: the desk decides.

Bounds: questions per call, calls per cell, Jev spend per run and per day
(a budget the owner sets, off by default on any computer that is not the
paper). Context is passed by reference to captures and story versions the
desk can already read, so a cell cannot smuggle text it did not read into
a question as if it were evidence.

### 3.2 `choir.Relate(...)`: research's judgment about published objects

A staged semantic act, reduced on the ledger like `Report`. It carries one
story version and any of: entity attachments (entity id or a new entity
with name, type and optional Wikidata id; role), a thread attachment
(existing thread or a new one with a title), and pair relations (other
story, relation, the Jev distribution it rests on, a one-line reason
citing the passage). Research authority stays read-only: `Relate` records
evidence about objects that already exist and changes none of them.
Texture remains the only writer of document text; these are links about a
story, not edits to it.

## 4. The review loop

1. **Wake.** A standing review schedule on the paper (a standing intent:
   a crash closes the batch in flight and the next tick is new work, per
   the restart rule) delivers a review batch record to research: up to K
   story versions not yet reviewed, newest first, plus for each the graph
   candidates below. The backfill is the same schedule draining the
   ~1,000 existing stories.
2. **Attach.** Research reads each story (`choir.Read`) and the captures
   it cites, lists its candidate entities, and searches existing entity
   and thread nodes for matches (`SearchWireCorpus` gains entity and
   thread hits). Matching an existing node is the default; a new node
   needs a reason.
3. **Classify with Jev.** For each story: "central or incidental?" per
   entity, "which thread, or none?" among the nearest threads. For each
   candidate pair: same incident / later development / same running
   subject / shares an entity only incidentally / unrelated.
4. **Decide.** A sharp distribution is taken as Jev answers. A flat one
   (top choice below a set p, or two choices close) means research reads
   both stories closely and decides itself, recording that it overrode or
   settled Jev.
5. **Record.** One `Relate` per story. The export route writes the
   entities, threads and edges to Store B in one batch, refusing a private
   computer's export, as phase 2 does for events.

### 4.1 Candidates: graph walks only

For a story S, the candidates are the stories reachable in two hops
through: a capture S cites (or its recapture by canonical key, live pages
excluded); a world event one of those captures reports, and its `follows`
chain; an entity S mentions as central; S's thread. Hubs are down-weighted
as in phase 1 (an entity or capture with too many citers gives no vote on
its own). No embeddings: when they arrive they add a candidate source,
not a judge.

## 5. The query

`related_story` edges with p at or above the show threshold, ranked by
relation (same incident, development, same thread, central shared
entity), then by recency. Phase 1 capture links stay as a fallback. A pair
linked only by an incidental entity, or only by one shared source, is not
shown. The endpoint and UI from phase 1 are unchanged.

## 6. Ways it could fail (before code)

- **R1 Entity sprawl:** "Anthropic", "Anthropic PBC" and "Anthropic's"
  become three nodes. Guard: match before mint; a duplicate audit on a
  sample; a merge act that records `same_as`, never a rewrite.
- **R2 Incidental entity links:** two stories that both mention Trump
  are linked. Guard: only central entities create candidates and links;
  the blind audit counts these as wrong.
- **R3 Thread drift:** a thread grows to mean "politics". Guard: threads
  have a title and a first story; a thread with too many members across
  too long a span is split by a research act, and its members re-judged.
- **R4 Jev miscalibration:** Jev's p is sharp and wrong on this question.
  Guard: the blind audit sets the show threshold per relation; the
  calibration feedback path already scores Jev on other questions.
- **R5 Cost:** about 1,000 stories times 10 candidates is about 10,000 pair
  questions. Guard: batched questions, per-day budget, the backfill
  drains under the budget.
- **R6 Injection:** a story or capture tells Jev or research to link it to
  everything. Guard: context by reference only; research's overlay treats
  story text as data; one story cannot write edges for another.
- **R7 Single authority:** Store B edges disagree with the tape. Guard:
  the tape is the authority; the export is one way and re-runnable.
- **R8 Restart:** a crash mid-batch re-dispatches the batch. Guard: the
  batch closes "interrupted by a restart"; the schedule's next tick picks
  up unreviewed stories (reviewed is a fact on the tape, not process
  state).

## 7. Acceptance

On the paper, staging: a 100-story sample reviewed, then a blind audit of
at least 40 shown pairs per relation kind and 40 entity attachments, by
the same procedure as the event blind sample. Bar: at most 10% wrong
shown links; coverage of the front 60 at least 50% (today 15%). Then the
full backfill, then the links show.

## 8. Order

1. The 15:00 UTC one-incident event rerun decides whether event links
   (phase 2) are usable; this phase does not wait on it but uses events
   when they pass.
2. Problem doc first: coverage of the front 60 and the missing subject
   nodes, with the probe numbers.
3. `choir.System1` for research, with bounds and tape records (red: gateway
   calls).
4. Entity and thread kinds, the three edge kinds, `choir.Relate`, the
   export route (red: Store B schema and write route, desk authority).
5. The review schedule and the 100-story sample, then the audit.
6. Backfill and show.

## 9. Ceremony

- **Conjecture delta.** Subject nodes attached by a reviewer that reads
  the story, with Jev on the closed questions, raise front coverage above
  50% at at most 10% wrong links, without embeddings. Refuted if the
  audit fails the bar at any show threshold that keeps coverage above
  30%.
- **Protected surfaces.** Gateway calls (Jev), Store B schema and write
  route, research desk authority (a new semantic act). Not Texture
  writes, auth, vmctl or routing.
- **Admissible evidence.** The staging blind audit. Unit tests prove the
  plumbing only.
- **Rollback.** The schedule off; the export's edges tombstoned; the
  query falls back to phase 1.
- **Heresy delta.** Discovered: no desk can ask Jev a question, so desks
  re-derive with their own model what Jev answers more cheaply. Introduced:
  none known. Repaired: none until built.

## 10. Decisions taken (No Blocking Asks)

- **D1 Entity identity:** our own entity nodes; a Wikidata id is an
  optional attribute when research is confident, never a dependency.
- **D2 Who reviews:** research, through a standing schedule, not through
  Texture's Casts, because the review is about the graph, not about one
  document.
- **D3 Research gets `choir.System1` first** (owner ruling). Management keeps
  reading Jev's intake labels; giving it the verb is a later, separate
  decision.
- **D4 No embeddings in this phase.**

## 11. Build plan, read against the code (2026-10-20, afternoon)

Two facts from reading `origin/main` change the order in §8:

- **Phase 2 never reached Store B.** `choir.world_event` and
  `choir.event_membership` live only on the computer's own store
  (`internal/store/world_event.go`, `internal/objectgraph/registry.go`);
  no export route and no `reports_event` edge exist. The related-stories
  walk runs in corpusd (`platform/publication_related.go`,
  `Service.RelatedStories`) over Store B's `cites_capture` edges, and its
  `Events` fields are always 0. So phase 3 builds the first export of
  desk judgments to Store B; there is no phase 2 route to copy.
- **The 15:00 event rerun failed the same-incident bar** (15-30% on 20
  joins, `docs/problems/event-layer-false-merges-at-marginal-p-2026-10-16.md`),
  and its wrong joins are real related pairs. Event identity is not used
  as a relation; events stay one candidate source among the graph walks.

Slices, each landed and proven before the next:

1. **`choir.Relate`, reduced onto the paper's tape (red: desk authority,
   ledger).** A staged intent like `ReportPacket`
   (`yaegikernel/intent.go`, checked early in the cell like
   `checkReportPacket`, re-checked in `rlm_reduce.go`
   `validateSemanticActIntent`, committed by `commitActIntent`), in
   research's module set only. Its body: one story version; entity
   attachments (an existing entity id, or a new one with name, type and an
   optional Wikidata id; role central or incidental); at most one thread
   (existing, or new with a title); pair relations (the other story
   version, relation, the System 1 distribution it rests on by its
   judgment id, one-line reason). Objects, not edges, on the computer's
   store, keyed like the event layer's (`computer\x00id`, write-once):
   `choir.entity`, `choir.topic_thread`, `choir.entity_mention`,
   `choir.thread_membership`, `choir.story_relation`. Ways it fails: a
   relation citing a System 1 judgment the run did not record (refused:
   the judgment id must be on this run's `system1:` channel); a story
   version that is not a published story on this computer (refused);
   sprawl (R1: a new entity whose normalised name matches an existing one
   is refused with the match named); one story writing another's
   attachments (refused: entity and thread attachments are for the act's
   own story; a pair relation names both and is stored once, ordered);
   replay (deterministic record id from cell and local id, as
   `commitmentRecordForIntent` does).
2. **Research reads them (orange).** `SearchWireCorpus` gains `entities`
   and `threads` beside `events` (`researchtools/corpus_search.go`,
   `corpus_events.go`), so matching an existing node is the default.
3. **The review schedule (red: standing intent, restart rule).** A
   research review schedule reusing the source schedule chain
   (`internal/agentcore/source_schedule.go`: generation guard, in-flight
   mark, crash-loop bound, `recoverSourceSchedulesAtBoot`, the overdue
   sweep), with its own wake kind added to `actorruntime/handler.go` and
   `RestartWorkKinds`. A tick commits one review-batch record to research:
   up to K story versions with no `Relate` yet, newest first, each with its
   graph-walk candidates (shared captures and recaptures from the related
   walk, shared central entities, same thread, the event layer's members).
4. **Export to Store B (red: Store B schema and write route).** A typed
   corpusd route modelled on support verdicts
   (`types.StoryVerdictsPath`, `platform/publication_checks.go`
   `HandleSupportVerdicts`, `requireInternalOrBoundGuestWrite`, a
   per-request cap, `MaxBytesReader`), refusing a computer that is not
   platform tier (`platformComputer`, fail closed), queued on the guest
   like `publishStoryVerdicts`. Re-runnable: the tape is the authority
   (R7).
5. **The query reads relations (orange).** `relatedWalk` takes
   `story_relation` rows at or above a per-relation show threshold, ranked
   same incident, development, same thread, central shared entity, then
   the phase 1 capture score; `RelatedLink.reason` gains those values; the
   frontend renders them (`texture-related-stories.ts`).
6. **The 100-story sample and the blind audit** (§7), then backfill and
   show.

Mutation classes: 1, 3 and 4 red; 2 and 5 orange; 6 evidence. Ceremony as
§9. Slice 1 starts after the engineering completion hang is fixed, because
both land on the same runtime and a hung guest makes every drill
ambiguous.

## 12. Owner ruling: System 1 does most of the reading (2026-10-20)

Owner: "I think we should use System 1 for a lot of the related stories
work. It should be able to read and classify stories very well and very
cheaply, more cheaply than the model behind research, GPT-6 Luna, and much
faster as well."

This turns §4 around. Before, research read each story and asked System 1
the close calls. Now System 1 reads and classifies every story and every
candidate pair, and research reads only what System 1 cannot settle.

**Per review batch (one research turn for K stories, not one per story):**

1. The batch record carries, per story, its graph-walk candidates (§4.1),
   its candidate entities and its nearest threads. Candidate entities are
   found mechanically, not by a model: existing entity nodes whose names
   (or recorded aliases) occur in the story's title or text, plus the
   capture labels' named subjects. Nearest threads are those the story's
   candidates and candidate entities already belong to.
2. Research's first cell sends the closed questions through
   `choir.System1`, with `story:<revision_id>` context refs so System 1
   reads the stories themselves: per candidate pair, the relation (same
   incident, later development, same running subject, shares an entity only
   incidentally, unrelated); per candidate entity, central, incidental or
   not about it; per story, which nearest thread or none. At the current
   bounds (12 questions a call, 4 refs a question) that is about one or two
   calls a story.
3. The same cell applies a fixed rule in code: an answer whose top choice
   is at or above the show threshold and clear of the runner-up by a set
   margin is taken as System 1 gave it, and recorded with `choir.Relate`
   citing the judgment. No Luna reading for those.
4. Research reads only the residue: flat distributions, and stories where
   System 1 found a central subject with no existing entity or thread (a
   new node needs a name, which a closed question cannot give). It
   records those with `Relate` too, saying whether it settled or overrode
   System 1.

The desk still decides: the threshold rule is research's, applied in its
own cell, and every relation is a `Relate` act on the tape citing the
System 1 judgment it rests on. Desk authority and the audit trail are as in
§3; only who does the reading changes.

**Why not a runtime classifier with no desk at all** (as the intake
labeller calls Jev directly): it would be cheaper again, but the relations
would then be the runtime's, with no desk accountable for them, and new
entities and threads would have no namer. Kept as a later option if the
residue turns out to be tiny.

**Budget.** System 1 is off until both caps are set
(`CHOIR_SYSTEM1_MAX_CALLS_PER_RUN`, `CHOIR_SYSTEM1_MAX_CALLS_PER_DAY`). On
the paper: 40 a run (a batch of 20 stories at two calls each) and 3,000 a
day (about the backfill of 1,000 stories in a day or two), set after the
current drills. The 100-story sample measures the cost per story from
`recordJevCostFor` and the residue share before the backfill runs; the
caps are raised or lowered on that number.

**What the blind audit adds.** It audits System 1's taken answers
separately from research's settled ones, per relation kind, so the
threshold is set from evidence (R4), and a relation kind System 1 gets
wrong is routed to research instead.
