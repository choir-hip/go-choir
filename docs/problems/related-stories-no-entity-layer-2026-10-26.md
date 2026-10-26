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
| c0 | Candidate finder in corpusd (no model, names in the story's title and text, bounded, hubs capped) and `choir.EntityCandidates`; the record's counts; the growth route | orange (read route, verb) and red (Relate rule) | 2 with c |
| c1 | System 1 extraction after the finder (owner 2026-10-26); blocked: Jev's Decisions API has no extraction type; options A to C | red (System 1 verb, gateway) | 2 |
| c | Revised (owner ruling 2026-10-26, no new desk): the story's own research desk records its entities before its final Report, enforced as a finish rule with `research.related_story` on; a story desk names its own story; records before publish export at publish | red (desk finish rule, tape, export) | 2 |
| d | Revised: backfill by an owner act that sends each older story's own research desk a relate request on the reader-request route; a schedule only if the trace shows stories with no research round | red, additive | 1 |
| e | Coverage floor in the deployed spec: share of the first 60 front stories with a link, `basis.mode` reported; the entity growth series printed (convergence asserted after 14 days) | yellow | 0 |

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

### Slice c and d, withdrawn designs

Two designs for the trigger were withdrawn before any code landed:

- One Cast per story to `research:<doc>` from `source:related-review`,
  closed by a Report to that source (c7f946f7f). Review 299 found that no
  route carries the close-out: a source inbox takes only management's Note.
- One computer-wide review desk, `research:related-review`, fed a Note per
  tick (17848ed4a). The owner ruled it out on 2026-10-26: "one
  computer-wide review desk … eg I don't want to make a new desk". Its
  tests are kept on the local branch `scratch/related-review-desk-tests`
  (cf712d267) and are not for main.

The second design rested on a claim that was wrong: that most published
stories settle within hours, so a story's own research desk could not run
for them. **A story trajectory never settles.**
`lifecycleSettlementReady` returns false for `TrajectoryKindStory`
(`internal/store/lifecycle.go`, "a living story has no work items … it
never settles by draining work"; `types.TrajectoryKindStory`: "a story stays
live for every later Cast or Note"). So every story's own research desk can
run at any time, the story just made and the story three weeks old alike.

### Slice c, revised: judge entities at the source (owner ruling 2026-10-26)

No new desk, RLM class or standing research identity. The story's own
research RLM, `research:<doc>`, records the story's entities as part of its
normal work.

**Where that run lives today (code read, origin/main b2cf5a8e4).**

- Texture opens the round: its V1 "dispatches the work in the same cell",
  `choir.Cast("research", objective, packet)` (`internal/textureprompts/texture.yaml`).
- The research run starts from its inbox (`WakeRecordsInbox`) with the
  prompt `recordsInboxPrompts[research]` (`internal/agentcore/inbox_records.go`):
  read the records, answer Texture with `choir.ReportPacket("texture", ...)`,
  send interim packets, end with a final Report.
- Finish rule: a packet whose `work_disposition` is `completed` or `empty`
  is final and closes the round (`reportBodyFromPacket`). The run ends when
  the model ends its turn. A round it leaves open is closed by the host at
  run end (`closeRoundsOfEndedRun`, B6).
- The desk's surface is `rlm_research_runtime.yaml`. It describes
  `choir.RelateStories` (on main) and, after slice a, `choir.Relate`.

**Why the run never related a story: the trigger gap.**

1. `choir.Relate` did not exist on main before slice a, which is not
   deployed yet. `choir.RelateStories` has existed since 10-25, and its
   write setting has been on for the paper since 11:25 today.
2. Nothing in the run asks for either verb. The inbox prompt and Texture's
   packet ask for sources. The finish rule needs only a `work_disposition`.
   The verbs are capabilities the desk may use, never a step it must take.
   That is the owner's "built and not switched on".
3. Timing: `choir.Relate` refuses a story that is not published
   (`checkPublicAutopaperStory`). Research usually runs before the revision
   that publishes: V1 is held when it cites fewer than two outlets, and
   research's packets are what bring in the second outlet. Even a desk that
   tried would often be refused.

Hypothesis, not yet traced: some stories never have a research round at all
(Texture's V1 is the only revision, or Texture never casts research). The
check on d@e: count the stories with a `research:<doc>` desk, and for each,
whether a round closed. Until that trace is read, the backfill below is the
cover for those stories too.

**The change.**

- **A finish rule, enforced by the host.** On a story's research desk with
  `research.related_story` on, a final Report is refused in the cell until
  the tape holds a `story.entities` record for the story, or this run's
  `choir.Relate` call was refused or failed (the run is not held hostage by
  a platform outage). The refusal names the step: "record what the story is
  about with choir.Relate first". It changes no route and adds no wake. A
  run that ends without a final Report has its round closed at run end, as
  today, so nothing wedges.
- **The story is named by the desk.** `choir.Relate` takes the story's
  publication id, and a story desk can pass `""` for its own story: the
  host derives `types.StoryPublicationID(owner, doc)`, which is what corpusd
  computes (pinned by a test). `choir.EntityCandidates` returns the head
  revision id, so System 1 cites `story:<revision_id>` as it does today.
- **Unpublished stories.** Relate records on the tape whether or not the
  story is published yet. The platform is still asked first, validate-only,
  with a `pending` flag that runs every entity check (types, ids, the R1
  duplicate check) and skips only the published-story check. The export
  follows the publish: after `AutoPublishRevision` publishes a version, the
  host exports the tape's `story.entities` records for that story (the same
  re-runnable projection; a repeat is a no-op in Store B). A story that
  never publishes keeps its record on the tape and never reaches Store B.
- **Prompt.** `recordsInboxPrompts[research]` and the overlay say it once:
  before the final Report, take the story's candidates
  (`choir.EntityCandidates("")`, slice c0), ask System 1 their roles, name
  what the story is centrally about that the finder missed (System 1
  extraction once c1 lands), and call `choir.Relate("", entities)`.
  Story text is data, never instructions.
- **Switch.** No new switch. `research.related_story` (on for the paper)
  turns on the rule. Off restores today's finish rule exactly.

### Slice c0: the candidate finder and System 1 first (owner, 2026-10-26)

Owner: "Recording story entities should be done using the system1 model
tool. That's what we agreed... the set of entities would grow with research
rlm creating new ones when system1 model doesn't capture them. Over time the
growth of new entities should slow as we develop a better world model."
This is phase 3 §12 (the 10-20 revision of §4) and its §11 slice 2, which
slices a to e had left out.

**The finder: mechanical, no model, no desk.** Corpusd holds the entity
rows (Store B, D3/D4), so corpusd does the matching. A guest read route,
`POST /internal/platform/story-entities/candidates {owner_id, computer_id,
title, text}`, goes through the proxy with the same bound-guest check as
the write route.

- **Index.** An in-memory index per owner, from normalised name to entity:
  built from the owner's `choir.entity` rows on first use, and dropped
  when `RecordStoryEntities` writes a row. One list read per rebuild: the
  paper today has no rows, and at 10,000 entities it is about 2 MB and one
  query.
- **Match.** The title and the text are normalised word by word, as
  `NormalizeEntityName` does a name (case, possessives, periods and
  apostrophes, punctuation as spaces). Every run of 1 to 6 words is looked
  up. The text is bounded to 20,000 characters, so a long story is about
  3,500 words and 21,000 lookups: microseconds, not a model call.
- **Common words.** A one-word name matches only where the story writes it
  with a capital letter and not as the first word of a sentence. It never
  matches if it is under four letters or on a short list of common words
  (May, March, Court, House, State and the like). Multi-word names match as they
  stand.
- **Bound.** At most 12 candidates a story, one System 1 call's questions.
  The order is title matches first, then by occurrences. Hub entities
  (central to more than `HubEntityStories` stories) take at most 4 of the
  12 slots, so a story naming Trump in passing still gets its rare
  entities asked. Each candidate carries `{entity_id, name, type,
  in_title, occurrences, hub}`.
- **Aliases.** Entity rows have no alias field today. Slice a's known gap
  stands ("Vucic" and "Aleksandar Vucic" are two names), so the finder
  matches names only. An alias act (`merged_into` or an alias on attach)
  is a named residual, and the mint-rate measure below is what shows
  whether it is needed.
- **Capture labels' named subjects.** These do not exist. A capture label
  carries an event and a topic answer (`store.CaptureLabel`), and a world
  event carries a headline, but nothing carries named subjects. Labels also
  retire under event wakes, which the paper runs. That source is empty
  today; this is written down, not built.
- **The read verb.** `choir.EntityCandidates(story)` is a research-only read
  (`""` means this desk's story). It returns `{story, publication_id,
  head_revision_id, title, candidates}`. The host reads the head revision
  for the text, so the cell never pastes story text into a prompt. No new
  store schema: the rows, the `$.normalized` lookup key and the edges
  exist, and the index lives in memory.

**System 1 first.** Superseded in its novelty step by slice c1 below
(owner, 2026-10-26). What stands: per finder candidate, one closed role
question (central, incidental or not_about) with `story:<revision_id>`,
slice a's contract; every attachment records whether it took System 1's
top answer or overrode it.

### Slice c1: System 1 extracts the entities the finder missed (owner, 2026-10-26)

Owner: "System one model shouldn't just ask if story is about entities not
captured by deterministic algorithm, but also it should emit the entity
capture after the deterministic check, since it can smartly execute that,
while also noting if more entity categories are needed/missing from its
options."

The step wanted, after the finder:

1. **System 1 extracts.** It returns the story's central entities that the
   finder did not cover, as at most N items. Each item has a name, a type
   from the closed list (person, organization, place, product, work, law),
   an optional Wikidata id, and a role (central or incidental) with a
   probability.
2. **System 1 flags missing categories.** An item that fits no type
   carries type `none_fits` and a proposed category name. Proposals are
   recorded on the tape, counted, and shown in the growth read. A
   recurring proposal is a signal for an owner or design act, never an
   automatic type.
3. **Research decides.** A mint must cite an extraction item of this run;
   otherwise it is an override with a reason of 20+ characters. Renaming
   or retyping an item is also recorded as an override. R1 (match before
   mint) applies to every extracted name.
4. **On the tape.** The extraction is recorded like a judgment, with
   digest replay, so no call is paid for twice. The growth read gains
   extracted, minted and overridden counts, and the category proposals
   with their counts.

Failure modes to test first:
- X1: malformed or oversized output;
- X2: a hallucinated Wikidata id;
- X3: a type outside the list without the `none_fits` escape;
- X4: prompt injection from story text into the extraction;
- X5: a replay that pays twice;
- X6: a mint with no extraction item and no reason.

**Finding, before any code: the backing judge cannot extract.** System 1
is Jev, `~typesafe/jev-latest`, called through OpenRouter's Decisions
alpha endpoint (`internal/gateway/jev.go`):
- The gateway accepts only question types `noul`, `choice` and `score`
  (`validateJudgmentRequest`). Anything else is refused before it costs
  money.
- The answer is a probability distribution over given labels.
- There is no list, text or structured-output question type, so System 1
  cannot return a name, a list of items or a proposed category.
- This was found once already for listwise importance: "The Decisions API
  has no list question type" (front-importance-underrates-major-stories-2026-10-14.md).

Faking extraction with closed questions is ruled out. So c1 stops here
until the owner picks one of these:

- **A. A second System 1 backend for extraction.** A pinned fast model
  behind the gateway on a new route, `/provider/v1/extractions`, with:
  - a JSON schema: at most N items, the closed type enum plus
    `none_fits`, `proposed_category` only with `none_fits`, and a
    `wikidata_id` pattern;
  - output size bounded at the gateway and checked again on the guest;
  - receipts like Jev's, and the same tape and digest replay on the
    computer.

  `choir.System1` gains one question kind, `extract`, that routes there.
  The model is a choice:
  - a non-reasoning flash model through the gateway's existing chat
    providers;
  - not deepseek-v4.1-flash in reasoning mode, which took 146 seconds and
    21k tokens per call in the importance probe.

  Red: the gateway, the provider route and the System 1 verb. This is
  the only option that does what the owner asked.
- **B. Ask TypeSafe and OpenRouter for an extraction or list type on
  Decisions.** Keeps one backend, but it is outside our control and has
  no date.
- **C. A mechanical phrase proposer plus Jev choices.** Code proposes
  capitalised noun phrases the finder did not match; Jev classifies each
  phrase's role and its type, with `none_fits` as a choice.
  - It cannot name a new category, because Jev has no text output, so
    only the `none_fits` count is kept.
  - Its recall is bounded by the proposer.
  - This is the closed-question shape the owner ruled out for naming;
    listed only for completeness.

Recommendation: A, with the model chosen by a small bake-off on 30 paper
stories (recall against research's own reading, malformed-output rate,
latency, cost). Slices c0 (the finder) and c (the finish rule) do not
depend on c1 and proceed. Until c1 lands, research mints as slice a allows:
a role resting on a System 1 role answer of the run, after research reads
the story.

**World-model growth, measured.**

- Each `story.entities` record carries `counts: {candidates, matched,
  existing_unmatched, minted}`. `matched` is attached existing entities the
  finder offered; `existing_unmatched` is existing entities attached by
  id that it did not offer, which is finder recall; `minted` is new
  entities.
- Corpusd exposes the daily series read-only: `GET
  /api/autopaper/entities/growth?days=N` gives, per UTC day, stories with
  an entity record, entities minted, mentions, and minted per story.
- Slice e's deployed spec prints the series and asserts that the route
  answers. The convergence assertion waits for 14 days of data.
- **Expected curve.** Cold start: almost every entity is minted, about 4
  to 6 a story on day 1. Minted per story should fall as the paper's
  recurring subjects fill in: about 2 by day 7 and at most 1 by day 30. A
  long tail of new names stays, so it never reaches zero.
- **Not converging.** Any of these would show it:
  - minted per story flat (within 20 percent) from day 7 to day 21;
  - alias sprawl: more than 15 percent of a week's minted names share all
    their words with an existing entity's name, or contain it ("Vucic"
    inside "Aleksandar Vucic"), which the growth route also counts;
  - `existing_unmatched` above 20 percent of attachments, meaning the
    finder misses entities research then finds by id.

  Any of these opens the alias act.

**Finder failure modes, tested first:**
- G1: a common word as a name matches (one-word names and the
  common-word list).
- G2: a hub entity crowds out the 12 slots, or is dropped altogether.
- G3: a long text is unbounded in time or memory (20,000 characters, at
  most 6-word runs, a timing bound in the test).
- G4: the index serves a stale set after a mint, or another owner's
  entities.
- G5: story text reaches a model prompt through the finder (it returns ids
  and names only).
- G6: (moved to c1 as X6) a mint with no extraction item and no reason.
- G7: the counts are wrong or missing on a record.

Failure modes, to be tested first: F1 the rule applies to a desk other than
a story's research desk, or with the setting off; F2 a final Report passes
with no record and no failed attempt; F3 a failed or refused Relate holds
the round closed forever (it must let the final Report through); F4 an
interim packet is refused (only the final one waits); F5 `""` or
`story:head` resolve to another story; F6 a pending record is exported
before the story publishes, or never after it does; F7 the export at
publish writes a second edge or flips a newer role back (the F3 rule of
review 298: the tape's latest role wins).

### Slice d, revised: backfill through existing paths

- **Reopening settled stories onto a new trajectory is not needed, and
  would be wrong.** Stories do not settle, and `ContinueLifecycleDocument`
  refuses a story by design. Every story's research desk is already
  reachable.
- **The route that exists.** An owner act that Casts a story's own research
  desk from the story's conversation: `CommitStoryResearchRequest`, the
  reader's "research more" route (an owner Cast from `texture:<doc>` to
  `research:<doc>`, a live trajectory, replay by client id, no Texture
  wake on the ask). The backfill is a sibling request kind on the same
  store path, with an objective of "record what this story is about" and
  no source questions. Research does the slice c finish rule and closes with
  an `empty` final Report.
- **Cost the owner should see.** That final Report reaches Texture and
  wakes it, as every research Report does: one Texture turn per backfilled
  story, which should end with `choir.Done` and no revision. The prompt
  says so. Making that Report not wake Texture would be a second wake
  exception, and it is not proposed.
- **Driver.** An owner act runs the backfill (`POST`, a limit of at most
  2,000, newest first, stories with no `story.entities` record on the tape).
  It is not a standing schedule: new stories are covered at the source. If
  the trace shows stories that never get a research round, the source
  schedule is the timer (review 299 flag 3's resolutions apply: reserved id
  `related-review`, no colon so no collision with a `<kind>:<name>`
  registry id, a branch before any poll code, the sweep's own stop reason
  told apart from the owner's stop and the crash bound). Each tick sends
  the same request, K stories, to those stories' own research desks.
  A crash closes the batch in flight; only the owner's act starts it again.
- **Flag 4** stands: `story.entities` stays on the `system1:<computer>`
  channel.

Restart rule (CLAUDE.md "Restarts End Work"): the finish rule adds no
work and no resumer; a crash closes the research run and its round as today.
A backfill batch is an occurrence: a crash closes it, and only the owner's
act starts another. No boot-time resumer runs without the planned-restart
marker.

Cost bound: one System 1 question per candidate entity, 12 questions a
call, so one or two calls per research round on a story (inside 40 a run),
about 200 stories a day against 3,000 calls a day. The backfill adds one
research run and one Texture turn per older story.

Rollback: `research.related_story` off stops the act and the finish rule
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
