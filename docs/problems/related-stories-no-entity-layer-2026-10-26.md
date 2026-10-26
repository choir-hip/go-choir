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
| c0 | A learning entity extractor: `internal/entitytext` (normalisation, gazetteer, phrase proposal, merging, scoring); corpusd's lexicon and candidates route; the host's System 1 analysis at run start, preloaded as `choir.StoryAnalysis()`; aliases, the learned stoplist, counts by source and the growth read | red (desk frame, System 1 by the host, Relate rules) | 2 with c |
| c | Revised (owner ruling 2026-10-26, no new desk): the story's own research desk records its entities before its final Report, enforced as a finish rule with `research.related_story` on; a story desk names its own story; records before publish export at publish | red (desk finish rule, tape, export) | 2 |
| d | Revised: backfill by an owner act that sends each older story's own research desk a relate request on the reader-request route; a schedule only if the trace shows stories with no research round | red, additive | 1 |
| e | Coverage floor in the deployed spec: share of the first 60 front stories with a link, `basis.mode` reported; the entity growth series printed (convergence asserted after 14 days); the labelled set of 30 stories | yellow | 0 |

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
  computes (pinned by a test). The preloaded analysis (c0) names the head
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
  before the final Report, read `choir.StoryAnalysis()` (preloaded, c0),
  record with `choir.Relate("", entities)` what System 1's answers support,
  name what the analysis missed when its novelty answer allows, and give a
  reason for any override. Story text is data, never instructions.
- **Switch.** No new switch. `research.related_story` (on for the paper)
  turns on the rule. Off restores today's finish rule exactly.
- **Where the setting is enforced** (review 298, DeepSeek seat, dissent).
  The setting is checked by the verb on the computer. Corpusd does not
  check it, and that is deliberate:
  - it is the computer owner's switch and lives on the computer, which
    corpusd has no read of;
  - the route takes only a bound platform-tier guest, whose own runtime is
    the only caller;
  - the read side has its own rollback (`CORPUSD_RELATED_ENTITIES=off`).

  A guest that bypassed its own verb would already be a compromised
  platform computer; the setting is not a security boundary.

### Slice c0: a learning entity extractor, preloaded into the research run (owner rulings, 2026-10-26)

The owner's rulings, in order:
- "Recording story entities should be done using the system1 model tool
  ... the set of entities would grow with research rlm creating new ones
  when system1 model doesn't capture them. Over time the growth of new
  entities should slow as we develop a better world model."
- "system1 can't propose new categories, only select from a list of
  categories (by assigning probabilities) and suggest that we need more
  categories."
- "we can and should be doing deterministic extraction before system1. Eg
  'Zelenskyy' or 'anthropic' or 'senate' in an article can be captured,
  and we can count occurrences."
- "This is a common problem for decades, keyword processing."
- "It should be a learning system, where we are growing our keyword list
  over time (eg each new entity becomes a keyword to filter on) hooked into
  system1, and the research desk. Research models will value this data."
- "The analysis should be auto inserted into research rlm repl I think."

This is phase 3 §12 with its §11 slice 2. No new desk, no model in the
extractor, no gateway change: System 1 is Jev and asks only `choice`
questions (Decisions has only `noul`, `choice` and `score`; extraction
backends were ruled out by the owner).

#### 1. Deterministic extraction (standard keyword processing)

Shared code, a new package `internal/entitytext`: pure functions, used by
corpusd and the guest alike.
- **Normalisation:**
  - Unicode NFKC, case folding, and diacritic folding (`golang.org/x/text`,
    already a dependency; "Vučić" and "Vucic" are one);
  - possessives ("Anthropic's") dropped;
  - hyphens and apostrophes handled;
  - the leading article and company suffixes folded, as
    `NormalizeEntityName` does today. As built, that function stays in
    `types` as identity, and the match key (`entitytext.Key`) layers NFKC
    and diacritic folding over it, so the minted ids do not change, which
    a test pins.
- **Tokens and sentences.** A rule sentence splitter (stops, closing quotes
  and brackets, a list of abbreviations such as Mr, Dr, U.S., Jan.), so a
  capital at the start of a sentence is known.
- **Gazetteer** (existing entities):
  - a token trie over the normalised tokens of every entity name and
    recorded alias, matched in one left-to-right pass, longest match
    first;
  - this is Aho-Corasick at the token level. A character-level
    Aho-Corasick library was looked for and none is in the module set;
    adding one changes the Nix vendor hash for a gain a token trie already
    gives, so none is added;
  - one-word names under four letters, or on the stopword and title-word
    list, never match alone. As built, a short name matches a token written
    in capitals ("FBI", "US"), never a lower or mixed-case one ("fed",
    "Fed").
- **Phrase proposal** (new candidates):
  - capitalised spans of 1 to 4 tokens not at the start of a sentence, or
    at the start when the same span recurs mid-sentence elsewhere;
  - acronyms (all capitals, 2 to 6 letters: NATO);
  - a lower-case form counted with its capitalised form when the text has
    both ("anthropic" and "Anthropic");
  - stopwords and title words (Mr, Mrs, The, the days, the months, Monday)
    are dropped, so they neither start nor end a span.
- **In-document name merging** (the standard news heuristic): a surname or
  shorter form that is a token suffix or prefix of a longer name in the
  same story maps to the longest form seen ("Zelenskyy" to "Volodymyr
  Zelenskyy"). Its occurrences add up, and the short form is noted as an
  alias candidate.
- **Scoring**, after YAKE:
  - casing (capitalised mid-sentence, acronym);
  - position (in the title, in the lead paragraph);
  - in-document frequency;
  - spread across sentences;
  - weighted by rarity in our own corpus: inverse document frequency over
    the computer's story library. The head term counts the library
    already keeps for search are reused (`store.searchEntry.terms`), so
    "Senate" ranks below "Zelenskyy" when the Senate is everywhere.
- **Bound:**
  - text is cut at 20,000 characters;
  - the proposal is linear in tokens (about 3,500), so a few milliseconds;
  - at most 24 candidates go to System 1 (two calls of 12 questions);
  - title matches rank first, then by score;
  - hubs (central to more than `HubEntityStories` stories) keep at most 4
    slots.

#### 2. Where it runs

- **Corpusd** holds the learned lexicon per owner in memory:
  - the gazetteer (entity names and aliases);
  - the learned stoplist;
  - each entity's world facts: stories, co-occurring entities, first seen,
    mention count and type.

  It is built from Store B on first use (and so after every corpusd
  restart). It is updated in place by every write: a mint, an alias or a
  stoplist entry is in the gazetteer for the next request, with no batch
  rebuild. Its size is reported in the growth read and in corpusd's
  health.
- **The guest** proposes phrases, using its own library's document
  frequencies, and sends `{title, text, proposals}` to one corpusd read
  route, `POST /internal/platform/story-entities/candidates`. It sits
  behind the proxy with the bound-guest check of the write route. Corpusd
  returns:
  - gazetteer hits with their world facts;
  - proposals not on the stoplist and not already an entity, with a
    proposal that is an entity's name or alias turned into a hit;
  - all ranked and bounded.

  As built (`internal/platform/publication_entity_candidates.go`):
  - corpusd extracts again from the bounded text with the shared package
    and takes only each proposal's rarity from the guest, so a proposal
    the text does not hold is never offered;
  - near forms fold into the one entity they belong to: a proposal that
    is part of exactly one entity's name ("Zelenskyy" of "Volodymyr
    Zelenskyy") becomes that entity's candidate, and a proposal around
    one hit ("Marta Kowalczyk" around "Kowalczyk") becomes that hit's alias
    candidate; a form two entities share goes to neither;
  - hubs are known before ranking (one edge read per hit, at most 48
    hits); prior stories and co-entities are read for the ranked hits
    only;
  - the lexicon assumes one corpusd process writes Store B, as today.

  No new store schema: two new object kinds, `choir.entity_review` (a
  review's counts) and `choir.entity_stopword` (a phrase's
  not-an-entity evidence), are registry entries. Aliases are a field on the
  entity row.

#### 3. System 1, asked by the host when the research run starts

With `research.related_story` on, when a story's research run starts, the
host builds the analysis before the first cell and asks System 1 itself.
The questions follow from the candidates, so the cell does not choose
them. The answers are recorded as this run's `system1.judgment` events, so
Relate's same-run basis rule holds unchanged. The questions are all
`choice`:
- an existing entity: its role (central, incidental, not about it);
- a new phrase: its type, over the closed list plus "not an entity" and
  "none of these fit: a new category is needed"; and its role;
- per story: "is this story centrally about a subject not in this list?",
  yes or no.

Each question's context carries the candidate's evidence as data:
- gazetteer hit or new phrase;
- occurrences, title flag, rarity;
- for an existing entity, its type and prior story count.

The story itself goes by `story:<revision_id>`, as today. The budget is
two calls a story, inside the 40-a-run cap.

**Crash.** The run is closed by the crash boot (restart rule); nothing
resumes. The next research run on the story builds the analysis again.
The host's analysis replays by digest across runs: an answered analysis
call with the same digest (same questions over the same context text,
which includes the head revision id) is re-recorded under the new run as
a replay, with no judge call, no cap and no cost. So the same-run rule
holds with no double spend. A request record with no answer (a crash
mid-call) counts toward the caps and is never replayed, as today.

#### 4. Preloaded into the research REPL

The existing per-cell frame mechanism carries it: `SessionFrame` already
preloads `Doc`, `Pack`, `Updates`, `Records` and `Jobs`, each read by a
free accessor. One field is added, `Analysis`, read by
`choir.StoryAnalysis()`. It contains:
- the story: publication id, head revision id, title;
- the candidates: entity id or phrase, source, occurrences, title flag,
  rarity, hub flag;
- each candidate's System 1 answers, with `receipt_ref` and `question_id`
  so Relate can cite them;
- the novelty answer;
- per existing entity, the world facts: type, first seen, mention count,
  up to 5 prior stories (publication id and title), and up to 5
  co-occurring entities.

Bounded to 32 KB, with names and titles only, never story text. It is
data, never instructions; the prompt says so. An ad hoc read verb over the
world model (`choir.Entities(query)`, or SearchWireCorpus gaining
entities, phase 3 §11 slice 2) is a second, optional surface and is not
built in this lane.

#### 5. Research decides

Research calls `choir.Relate("", entities)`:
- **A gazetteer hit or a confirmed phrase** cites its System 1 answers.
  A phrase is confirmed when its top type is a listed type. The mint's
  type is that answer's top type; retyping needs a reason of 20+
  characters and is recorded as an override.
- **Research names an entity** the analysis missed only when the novelty
  answer allows (p(yes) of at least 0.35). The new name is then asked a
  type and a role question by the cell, through `choir.System1`. A mint
  without that novelty basis needs a reason of 20+ characters.
- **A "none of these fit" answer** mints nothing by itself. Research may
  name the category it would suggest in the reason field. Neither System 1
  nor research adds a type; that is an owner or design act.
- R1 (match before mint) applies to every name, against names and
  aliases.

#### 6. The learning loop

- **Positive keywords.** Every minted entity and every recorded alias is
  in the gazetteer for the next request. A mint in story A is a gazetteer
  hit in story B on the next request.
- **Negative keywords.** A phrase System 1 judges "not an entity" with p
  of at least 0.8 in N = 3 distinct stories joins the learned stoplist,
  and stops costing System 1 questions. The stoplist:
  - is visible in the growth read;
  - is reversible: an owner act revokes an entry, and a revoked entry is
    not relearned for 30 days;
  - can never block an entity's name or alias (checked at write and at
    match).
- **Aliases.** When the in-document merge maps a short form to a longer
  name and research attaches the longer name, the export records the short
  form as an alias of that entity, unless it is another entity's name. An
  alias is also recorded when research attaches an existing entity by id
  under another name. This attacks the alias gap and `existing_unmatched`.
- **Sources.** Each `story.entities` record counts its attachments by
  source: `existing` (a gazetteer hit), `extracted` (a confirmed phrase),
  `research_named` (named under the novelty rule), plus `overridden`,
  `category_gap` (with the names that drew it) and the novelty answer.

As built (`internal/platform/publication_entity_learning.go`):
- the export's optional `review` is recorded once per tape record (a
  `choir.entity_review` row, insert-if-absent), and a review may attach
  nothing;
- stopword evidence keeps up to 8 distinct stories; a revoke clears it, and
  evidence gathered during the 30-day hold counts once the hold ends;
- the revoke is a corpusd method (`RevokeEntityStopword`). Its owner-facing
  act surface is a named residual, not built in this lane;
- aliases go on the entity row's body and metadata, and the lexicon reads
  them back after a restart.
- the growth read is unauthenticated, so it aggregates only the paper
  owners (`CORPUSD_AUTOPAPER_OWNERS`), and nothing when none is set. It
  shows counts, sizes and the active stoplist. It never shows aliases or
  another owner's entities. Every row it reads was learned from a published
  public paper story, because the entity write accepts no other. The cache
  is keyed by window alone, which is right because the answer does not
  depend on the caller.

#### 7. The growth read: the loop's health

`GET /api/platform/entities/growth?days=N`, public and read-only, cached 5
minutes. It gives the series per UTC day:
- stories reviewed;
- gazetteer hit rate per story (hits over attachments);
- mints by source (`extracted`, `research_named`);
- `existing_unmatched` and its share;
- alias sprawl and its share (a minted name contained in, or containing,
  another entity's name);
- keywords added (entities and aliases);
- aliases learned;
- stoplist size;
- category gaps with their names;
- the novelty "yes" rate (the live recall signal);
- the gazetteer's size.

From day 1, `existing_unmatched` and alias sprawl are shown, so the owner
sees when the alias act is due.

**Expected curve:**
- On day 1 most entities are minted (`extracted`, about 4 to 6 a story).
- Then the hit rate rises and minting falls: about 2 a story by day 7, at
  most 1 by day 30.
- `research_named` is the smallest source and shrinks.
- The novelty "yes" rate falls below 20 percent.

**Not converging**, any of:
- minted per story flat (within 20 percent) from day 7 to day 21;
- alias sprawl above 15 percent of a week's mints;
- `existing_unmatched` above 20 percent;
- the novelty "yes" rate above 30 percent after 14 days.

**Measurement and escalation.** A labelled set from 30 paper stories
(entities a careful reader attaches, written once into
`docs/evidence/`) gives the extractor's candidate precision and recall
against what System 1 and research attach. A small statistical NER
tagger on the CPU is the usual next step. It is named as the escalation,
not built: it is triggered by candidate recall below 0.8 on the labelled
set, or by the novelty "yes" rate above 30 percent after 14 days.

**Baseline, 2026-10-26.** The labels were committed (781be0ebc) before
the extractor ran. Cold start: no gazetteer, no library IDF.
`internal/entitytext/labelled_set_test.go` gives this candidate recall at
d76fe2f1b:
- overall: 151 of 176 (0.858);
- central: 60 of 71 (0.845);
- within the top 24: 0.812 overall, 0.845 central.

That is above the 0.8 trigger, but not by much. The 25 misses, read before
any change:
1. **A possessive does not end a name** (8 misses). "Russia's Voronezh",
   "Japan's Kyodo", "AFS's Mingshu Bates", "Hulu's English",
   "OnTrac's West Coast", "Ukraine's General Staff", "France's Lycées
   Indignés" run on into one span. This is a defect against G3.
2. **A one-word name that only starts sentences** (6 misses): Meta,
   Anthropic, Steinmeier (three times, every time at a sentence start),
   Hungary, Mossos, and Michigan inside "Michigan Advance". The rule drops a
   sentence's first word unless it recurs mid-sentence.
3. **A capitalised connector is trimmed** (2 misses): "El-Sayed" becomes
   "Sayed" and "El Arepazo" becomes "Arepazo". Separately, a sentence-start
   opener stays in the span: "Democrat Abdul El-Sayed".
4. **Alphanumeric acronyms** (1 miss): "G20".
5. **Spans over four words are dropped** (1 miss): "EU Trade Commissioner
   Maros Sefcovic" loses "EU".
6. **The 48-proposal cap** in the longest story (1 miss): PolitiFact.
7. **Not reachable by this method** (6 misses):
   - lower case: swisstopo;
   - title only: Georgia and Jordan;
   - a descriptive form: "U.S. military";
   - a name with a digit word: "Mein Schiff 3";
   - one demonym-only mention.

Fixes 1, 3 (the capitalised connector), 4 and part of 2 are generic. The
part of 2 that is fixed: a sentence-start word that recurs at a sentence
start, carries a possessive, or appears in the title. Each fix gets its own
failure test first. The re-measure is reported beside this baseline and
does not replace it.

**Re-measure after the fixes.** The four fixes each had a failure test
first. After them, on the same labelled set with no label changed:
- overall: 167 of 176 (0.949);
- central: 66 of 71 (0.930);
- within the top 24: 0.892 overall, 0.930 central;
- proposals: 280 (from 269).

The 9 left are:
- the unreachable ones: swisstopo, Georgia, Jordan, "U.S. military",
  "Mein Schiff 3";
- Anthropic: one sentence-start mention, not in the title;
- Ukraine's General Staff: "General" is a title word;
- "Michigan Advance" absorbing Michigan;
- EU inside a five-word span.

A caution: the fixes were found on this set, so the re-measure is
optimistic. The live check is the growth read's recall signals: the
novelty "yes" rate and `existing_unmatched`.

#### 8. Failure modes, tested first

- **G1, false matches.** A sentence-start capital proposed as a name; a
  common or title word ("The", "Monday", "Senate" at the start of a
  sentence alone); a one-word name under four letters matched.
- **G2, merging.** A surname alone is not merged with its full name in
  the same story ("Zelenskyy" and "Volodymyr Zelenskyy" merge to the
  longest form); a surname shared by two full names is merged with the
  wrong one (it stays unmerged).
- **G3, normal forms.** Possessives ("Anthropic's"); diacritics ("Vučić");
  lower and capital forms counted as two.
- **G4, cost.** A 20,000-character text not bounded in time or memory, or
  text past the bound read.
- **G5, hubs.** Hubs crowd out the slots, or are dropped altogether.
- **G6, the loop.** A mint in story A is not a hit in story B on the next
  request; a corpusd restart has no lexicon; another owner's entities are
  offered.
- **G7, the stoplist.** It blocks an entity's name or alias; a phrase is
  stopped before N stories; a revoked entry is relearned at once.
- **G8, aliases.** An alias that is another entity's name is recorded; R1
  misses an alias.
- **G9, the preload.** Over 32 KB; story text in it; another story's
  analysis; it is present with the setting off.
- **G10, replay.** A crash between the analysis and the first cell
  re-pays System 1 on the next run; the replay is recorded under another
  story.
- **G11, counts.** Counts by source are lost, double counted on a
  re-export, or missing from the growth read.

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
