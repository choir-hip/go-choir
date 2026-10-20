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

### 3.1 `choir.Jev(questions)`: a closed question, answered with a distribution

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
3. `choir.Jev` for research, with bounds and tape records (red: gateway
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
- **D3 Research gets `choir.Jev` first** (owner ruling). Management keeps
  reading Jev's intake labels; giving it the verb is a later, separate
  decision.
- **D4 No embeddings in this phase.**
