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
- Review 298 (tier 2) fixes: two new entities of one normalised name in
  one request are refused (F1); the duplicate check is a seek on the
  `choir.entity` `$.normalized` lookup key (F2); a call whose role differs
  from the tape's latest record for that story and entity is recorded as a
  role change, so the tape and the live edge never disagree (F3).
  Accepted as is (F4): the export writes rows and edges one at a time, not
  in one transaction; a failure part way leaves some edges written, the
  tape holds the whole record, and a repeat re-exports it.
- Review 299 (slice b): the hub cut counts raw central citers, before
  eligibility; it can only make an entity a hub sooner (documented in code,
  not read row by row).

### Slice c, the design taken (before code; revised after review 299)

The first draft (one Cast per story to `research:<doc>`, closed by a
Report to `source:related-review`) failed three checks against the code,
named by review 299 and confirmed:

- **No reply route.** A source inbox takes only management's Note
  (`rlm_reduce.go` destination check, `recordsRoute`); research has no
  route to `source:*`, so the close-out Report could not commit.
- **Per-story desks starve.** A story's research desk runs only on a live
  trajectory (`admitLifecycleResearchProviderEntry`: "lifecycle Research
  scope is not exact and live"), and a story trajectory settles when its
  last run ends with no open work (`ReconcileLifecycleSettlementForTerminalRun`).
  Most published stories are settled within hours, so a review that skips
  them would starve the standing review as well as the backfill.
- **The research inbox prompt** (`recordsInboxPrompts`) tells research to
  answer Texture, so a review on a story's desk risks waking Texture.

The design taken instead is the batch of phase 3 §11 slice 2: **one
computer-wide review desk**, `research:related-review`.

- **The desk.** A research desk with no document and no trajectory
  (lifecycle version 0, so provider admission treats it as a plain research
  run, which `admitLifecycleResearchProviderEntry` admits with no
  trajectory). The tick mints it when absent. `WakeRecordsInbox` starts its
  run without a trajectory and with its own prompt. It has no Texture:
  `recordsRoute` gives it no route at all, so a Report or Note from it is
  refused in the cell, and nothing it does can wake a Texture.
  `choir.System1` reads any story of the computer by `story:<revision_id>`,
  and `choir.Relate` takes any publication id, so one desk serves settled
  and live stories alike.
- **The review is a Note**, not a Cast: it opens no round, so there is
  nothing to close and nothing to wedge. It takes no reply.
  Sender `source:related-review`, inbox `research:related-review`, emitter
  system, record id `related-review:<generation>:<due unix>` (the tick
  slot, so a replay of one tick is one record). The body is one packet
  listing up to K = 10 stories: publication id, document id, title, head
  revision id and the story text bounded to 3,000 characters, marked as
  data, never instructions.
- **The wake exception** (`RecordWakesReader`), as narrow as the record:
  `types.IsRelatedReviewNote` holds only for emitter system, sender exactly
  `source:related-review`, inbox exactly `research:related-review`, a Note,
  and a record id starting `related-review:`. Failure modes, tested first:
  W1 a record minted at a boot wakes (only the tick mints it; a crash boot
  marks every pre-boot work wake `interrupted_by_restart`,
  `MarkActorWakeInterrupted`); W2 a reply from the review desk wakes anyone
  (it has no route); W3 a system record sharing one field (the sender, the
  inbox or the prefix, a Cast instead of a Note) wakes; W4 a review Note to
  another inbox (a story's `research:<doc>`, a Texture) wakes.
- **The schedule is a source schedule** with the reserved id
  `related-review` (registry ids are `<kind>:<name>`, so it cannot collide;
  a test pins that the registry has no such id, and a user start of it goes
  through the review's own start, not the registry). It inherits the
  generation guard, the in-flight mark, the crash-loop bound, boot recovery
  and the overdue sweep. `HandleSourcePollDue` branches on the id before any
  poll code: no fetch, no captures, no arrival; its usage counts stories
  listed (`new`), looked at (`fetched`) and skipped (`skipped`).
- **Who starts and stops it:** the overdue sweep (every 5 minutes, never at
  boot): for each owner and computer with an active source schedule, with
  `research.related_story` on it creates the schedule when absent, first
  tick one interval out, and restarts it only when its state reason is the
  sweep's own "stopped: research.related_story is off"; with the setting off
  it stops an active one with that reason. A schedule the owner stopped
  (any other reason) or the crash-loop bound closed
  (`interrupted_by_restart`) is left alone; the owner's POST of
  `related-review` restarts it, and is refused while the setting is off.
  A tick also checks the setting and lists nothing when it is off.
- **A tick** (every 30 minutes) lists up to K stories, newest first,
  updated in the last 48 hours, past their seed (version 2 or more), opened
  publish auto, with no `story.entities` record on the tape and not listed
  by a review in the last 24 hours (the schedule keeps the listed
  publication ids, bounded). Live or settled does not matter. It sends
  nothing while the review desk has an active run (busy, counted).
- **Tape:** `story.entities` stays on the `system1:<computer>` channel
  (review 299 flag 4: moving it would orphan slice a's records). The
  reviewed read is the same 10,000-event window; at about five events a
  story it covers about 10 days, enough for the 48-hour window; a backfill
  story whose record has left the window may be reviewed twice, which
  records nothing new (named residual).
- **Cost:** one research run per tick with up to 10 stories, one or two
  System 1 calls a story (inside 40 a run), at most 48 runs a day.

### Slice d, the backfill (design)

An owner act, `POST /api/autopaper/related-review/backfill {"limit": n}`
(at most 2,000), sets backfill state on the review schedule: a cursor (the
oldest story time the standing window covers) and a remaining count. Each
tick fills the room the standing window left in its K with older stories
below the cursor, newest first, by the same tests, and moves the cursor
down. Settled stories are handled like any other: the review desk needs no
live trajectory, so nothing is skipped for its trajectory. A crash boot
closes the backfill (state `interrupted_by_restart`, reason "interrupted by
a restart") while the schedule itself survives; only the owner's act starts
it again. `DELETE` stops it.

Restart rule (CLAUDE.md "Restarts End Work"): the schedule is a standing
intent and survives a crash; a tick's review is an occurrence and does not.
A crash closes the tick in flight (its usage records
`interrupted_by_restart`), marks the review Note's pending wake
`interrupted_by_restart`, and closes the review desk's run; the next tick is
new work, one interval after boot, and "reviewed" is a fact on the tape (a
`story.entities` record), not process state, so nothing is re-dispatched.
An interrupted review's Note stays unread; the next tick's Note supersedes
it (the desk prompt says to act on the newest). No boot-time resumer runs
without the planned-restart marker.

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
