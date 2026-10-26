# Related stories have no entity layer, and nothing ever casts the review (2026-10-26)

Status: documented; build in slices below (lane `related-review`).
Continues [related stories are thin: no subject nodes](related-stories-thin-no-subject-nodes-2026-10-20.md)
and builds the ratified design
[related stories, phase 3](../reports/related-stories-phase3-research-review-2026-10-20.md)
(slices 1 to 5) with the entity kind of
[claims, events and entities](../reports/claims-events-entities-ontology-2026-10-14.md)
(owner rulings D1 to D5 and D7 to D13, ratified 2026-10-20).

## Owner, 2026-10-26

> "We're supposed to relate stories not by the captured content, but by
> shared entities in the object graph. I thought we built this already."

> "I don't like the policy of building stuff and not switching it on...
> We keep rebuilding that and then it stops working."

## Evidence (read-only, public GETs of choir.news, origin/main 99524f638, about 11:30 UTC)

- The only live signal is "two stories cite the same capture". Among the 100
  newest stories (368 distinct cited captures, from `/resolve`
  transclusions) no capture is cited twice: 0 pairs. The walk
  (`internal/platform/publication_related.go` `relatedWalk`,
  `relatedCaptureCandidates`) links only through a shared capture or a
  recapture of one page, so it has nothing to link.
- 13 of the 100 stories have any link; 0 of the first 20; 4 of the first 60
  (7 percent), against 14 of 60 (23 percent) at 00:50 UTC the same day.
  None of the 32 links points at a story among the 100: links appear only
  when a new story re-cites a capture from an earlier day.
- Every answer has `basis.mode == "captures"`, `basis.events == 0`,
  `partial == false`, no error. It is not a fault, a budget, a cache or a
  renderer bug: the empty list renders as nothing, by design.
- By cited captures: 2 captures 4/63 linked, 3 to 4 captures 7/27, 5 or
  more 2/10. 63 of 100 stories cite exactly two captures.

## What exists in the object graph (code read, origin/main)

- **No entity node.** `choir.entity` and `choir.topic_thread` are not
  registered (`internal/objectgraph/registry.go`), and no `mentions_entity`
  or `in_thread` edge exists. No Go or TypeScript code names them.
- Look-alikes that are not entities: `choir.subject`
  (`internal/platform/publication_graph.go:45`) is the owner's or submitter's
  "self" node; `choir.publication_source_entity` (`publication_graph.go:475`)
  is a cited source (`texturedoc.SourceEntity`).
- Publishing links a story only to the captures it cites (`cites_capture`).
- The nearest subject signals stay on the computer: `choir.world_event`
  with its memberships and `follows` (`internal/store/world_event.go:46`),
  and the capture label's per-capture `TopicDocID`
  (`internal/store/capture_label.go:56`). Neither reaches Store B.
- Built and switched: `choir.System1` (research only, `research.system1`
  on for the paper) and the pair edge `related_story` with
  `choir.RelateStories` (lane related-p3, 10-25): the write setting
  `research.related_story` is on for the paper since 11:25 UTC today; the
  read switch `CORPUSD_RELATED_STORY=on` is in review on
  `lane/related-read-on`.
- The design's entity and thread kinds, `choir.Relate`, the review schedule
  and the batch record (phase 3 §11 slices 1 to 3) were never built. The
  10-26 build note says "FP0 casts the review by hand until the schedule
  exists". No schedule, management step or hook casts research to relate
  stories; the research overlay only describes the verb.

## Substrate cause

1. **The graph has no entity layer.** "Same capture" is a by-product of
   duplicate coverage, and the event layer exists to make one story per
   event, so the by-product disappears as production improves. The durable
   subject facts (who and what a story is about) were designed (phase 3,
   ontology D3) and never stored where the walk reads.
2. **Nothing triggers the review.** Every judgment verb waits for a cast
   that no standing intent issues. Built and not switched on, or built and
   never cast, is the same failure the owner names.
3. **No coverage floor.** `frontend/tests/autopaper-related-deployed.spec.js`
   asserts only that one story has a list, so 7 percent and 23 percent both
   pass and decay is silent.

## Build plan (reusing the ratified kinds, names and authority rules)

Authority (ontology §12.2, phase 3 D2): research records relation judgments
about published stories; it changes no document (Texture stays the only
writer of canonical revisions, I26). Management is not involved: the review
is about the graph, not about one document (D2). Story text is data in
every prompt and question; evidence goes to System 1 by reference
(`story:<revision_id>`, capture ids), never pasted.

Switches: no new owner setting. The write side reuses
`research.related_story` (on for the paper today): with it on, the act
records and exports, and the review schedule runs. The read side is on by
default in corpusd, with a kill switch (`CORPUSD_RELATED_ENTITIES=off`) as
rollback, like `CORPUSD_RELATED`.

| Slice | What | Class | Review tier |
|---|---|---|---|
| a | `choir.entity` kind and `mentions_entity` edge registered (owner corpusd, D3/D4); `choir.Relate` for research: one story's entity attachments (existing id, or new with name, type, optional Wikidata id, D1/D5; role central or incidental), each resting on a System 1 answer of the run (as `RelateStories`); match before mint (R1: a new entity whose normalised name or alias matches an existing one is refused with the match named); recorded as a `story.entities` tape event (acknowledged), the tape the authority (R7) | red (new tape event kind, desk surface) | 2 |
| b | `relatedWalk` reads shared **central** entities (on by default, `CORPUSD_RELATED_ENTITIES=off` the rollback); an entity central to more than 30 stories (`HubEntityStories`) gives no vote; ranking: judged `related_story` pairs, then shared central entities (more first), then shared captures (now a minor signal). The export route moved into slice a (the duplicate check needs the rows) | orange (product read, behind a kill switch) | 1 |
| c | Standing review schedule on the source-schedule chain (generation guard, in-flight mark, crash-loop bound, `recoverSourceSchedulesAtBoot`, the overdue sweep): each tick hands research up to K published stories with no `story.entities` record yet, newest first, with mechanically found candidate entities | red (boot path, standing intent) | 2 |
| d | Backfill of existing stories: an owner act that lets the review drain older stories under the same K per tick; a crash closes it and only an owner act restarts it | red, additive | 1 |
| e | Coverage floor in the deployed spec: share of the first 60 front stories with a link, `basis.mode` reported | yellow | 0 |

### Slice a as built

- The duplicate check (R1) needs the entity rows, and the rows live in Store
  B (D3/D4), so the corpusd write route came with slice a: the verb asks it
  validate-only before the tape record (as `choir.RelateStories` does), and
  the export follows the record. Slice b is the walk alone.
- Authority: one `story.entities` event per call on the `system1:<computer>`
  channel (acknowledged), as `story.relation`. The Store B rows and edges
  are its re-runnable export.
- Identity: `wd:Q<n>` when research gives a Wikidata id, otherwise
  `ent:<24 hex>` derived from the normalised name (lower case, possessive,
  periods, a leading "the" and company suffixes folded), so a repeat or a
  second spelling of the same normalised name is the same node. A new
  entity whose normalised name matches a row of another id is refused with
  that id named. Known gap, written down: aliases ("Biden" and "Joe
  Biden", "Vucic" and "Vučić") are not matched; the duplicate audit and a
  later `merged_into` act cover them.
- A repeat after a failed export re-sends the description of an entity the
  tape minted, so Store B can still write its row.
- Edge: `mentions_entity` from the publication (not a version, as
  `related_story`) to the entity, metadata exactly `{"role": ...}`; a role
  change tombstones the other role's edge.
- Threads (`choir.topic_thread`, `in_thread`) are not built in this lane.

Restart rule (CLAUDE.md "Restarts End Work"): the schedule is a standing
intent and survives a crash; a tick's batch is an occurrence and does not.
A crash closes the batch in flight ("interrupted by a restart"); the next
tick is new work, and "reviewed" is a fact on the tape (a `story.entities`
record), not process state, so nothing is re-dispatched. No boot-time
resumer runs without the planned-restart marker.

Cost bound: one System 1 question per candidate entity and per candidate
pair, 12 questions a call, so about one or two calls a story; at K = 10
stories a tick every 30 minutes the schedule takes at most 480 stories a
day, against about 200 published a day, inside the default caps (40 calls a
run, 3,000 a day). The backfill drains under the same caps.

Rollback: `research.related_story` off stops the act and the schedule
(nothing recorded or exported); `CORPUSD_RELATED_ENTITIES=off` plus a
corpusd restart drops entity links from the answer at once; `git revert`.
Entities and edges already written stay in Store B and are ignored when
the read is off.

Acceptance (phase 3 §7, unchanged): on the paper, a sample reviewed and a
blind audit of at least 40 entity attachments and 40 shown pairs, at most
10 percent wrong; coverage of the front 60 at least 50 percent.

Heresy delta. Discovered: built judgment verbs with no standing trigger;
a deployed spec with no coverage floor. Introduced: none intended.
Repaired: none until the slices land.
