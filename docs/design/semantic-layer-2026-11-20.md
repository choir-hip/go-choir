# The semantic layer: one main point per story, and fewer searches per run (design, 2026-11-20, cut 2026-11-21)

**Status.** Proposal, revised once after two reviews of 57c9ef106 (both REVISE
FIRST). Nothing here is built unless
[current-architecture.md](../current-architecture.md) says so. Part 1 is what
the two landings built tonight, in parallel, need: A, the main-point judgment
at management's open-or-feed decision; B, fewer searches per run with the read
before a search on the tape. Part 2, below the line marked **Later**, follows
and gates neither.

**Rulings worked in.** 2026-11-20: a story is one main point; material on the
same point revises it; a changed point starts a new story; the boundary is
judged by reasoning; RLM agents decide, System 1 is a tool. 2026-11-21 (owner,
verbatim): "raw duplicates may be only 17% but we can probably do more story
clustering for where they overlap"; of the search levers, "Fewer searches per
run"; on an unclear pair, "Lean to revising"; on pace, "no, it shouldnt take
days or weeks to build. you can do this all overnight". Not chosen and deleted
here: waiting before a story's first search, researching fewer stories,
relaxing the 1,000 target, any ceiling.

Tags: **M** measured, **C** code at origin/main db31a7cd1, **E** an evidence
record, **A** assumed or estimated.

---

## 1. The design on one page

**The story.** A story is one main point: one sentence, written at the level
of the beat, saying what the story is for ("Who won the election and by how
much", not "the election"). Material that serves the point revises the story;
when the point moves on, a new story begins and records the one it succeeds.
On an unclear case the judge leans to revising (owner). So the paper builds a
few deep stories that take in every article as it arrives, where a newspaper
prints one per beat.

**Where the judgment is made.** In the management task that already decides
open, feed or decline for every captured poll (model `swe-2-flash`). No new
run, no new desk, no new model call. The task is shown, for each capture, the
held stories its nearest captures already belong to (by vector, and by the
search leads a story's research turned up), each with its main point, plus the
whole 8-hour page that the stopgap C1b ships. Title words are dropped: the
stopgap's title-word offer fired on 2 or 3 of about 37 opens and every verdict
was "different", while showing the whole page moved feeds from 51% to about
63% of decisions (**E**, the lead's reading of the paper's console,
2026-11-21).

**The search.** A research run sends 3.92 searches; 44% of its later ones go
out with no page read since the one before (**M**, section 4.1). The fix is the
caller: a run reads what its last search returned, and what the paper already
held, before it sends another; its prompt says when to stop; each send records
what the run held and had read when it asked.

**The numbers.** About 690 research runs a day at 3.92 is about 2,700
searches. 1,000 a day needs about 1.45 a run. My central estimate after A and
B is about 1,250 a day (range 1,000 to 1,700), section 4. Reaching under 1,000
is a bet at p 0.2, not a plan.

---

## 2. The story judgment

### 2.1 The answers

For each capture the task holds, one question: which held story's main point
does it serve?

| Answer | Meaning | What happens |
|---|---|---|
| `same S` | serves S's main point | the capture is fed to S (`choir.Feed`, a feed disposition to S's inbox); no story opens |
| `next S` | about what S is about, but the point has moved on (the count after the campaign, the verdict after the hearing) | a new story opens with its own main point and `succeeds: S` |
| `new` | a different matter | a new story opens |
| `decline` | not news for the paper | as today |

Two captures of one task with one new point open one story (the second is fed
to it in the same task). An unclear case is `same` (owner, 2026-11-21: "Lean
to revising"), with the reason written.

### 2.2 The main point: one writer, written once

The custodian rule (CLAUDE.md: each data kind has one custodian with sole
write) decides it. **The main point is a field of the opening cast**, the
`texture:new` Cast that management's task writes: management's own
precommitment record, its kind. The open path copies it into the story
trajectory's subject refs (`store/story_open.go:199-204` already copies the
opening record's id there, `types.StoryRefOpeningRecord`), and nothing ever
writes it again. Texture never writes it. That settles seat 2's "two writers"
and seat 1's drift: there is no rewording, so there is no drift of the
statement. A story opened before landing A has no main point; the judge is
shown its headline and lede instead.

**A changed point is refused, not absorbed.** Texture's story has a fixed
point. When a capture fed to it carries a new point, Texture rejects it
(`choir.Reject`, `yaegikernel/choir.go:1669-1704`, which today commits a free
reason with the turn and routes nowhere) with the reason beginning `main point
moved:` and the new point in one sentence; or `not this main point:` when it
was fed to the wrong story. Landing A2 (red, after A) makes the host turn
those two reasons into a record to management, and management's own turn
re-disposes the capture: a task cannot dispose a capture twice
(`store/management_tasks.go:458-460`) and `captureDelegable` skips disposed
captures (`agentcore/management_tasks.go:774-781`), but management's parent
turn can, and the replaced disposition is kept as an override
(`store/management_tasks.go:466-475`). For `main point moved` it opens the
successor with that point and `succeeds: S`. The story itself never changes
its point.

### 2.3 The two zoom cases

- **One remark against the whole press conference.** A story's point is never
  widened. A capture of the whole press conference feeds the remark's story
  only when the remark is what the capture leads with. Otherwise it opens its
  own story; the remark's story stays, and later captures about the remark
  feed it.
- **A second country's reaction.** Words (a condemnation, "an intentional
  provocation") feed the incident's story. An act with its own consequences
  (sanctions, an expulsion, a formal request to an alliance, a court filing)
  is a new story, unless the held story's point already names that act (the
  UK sanctions story whose point is "the UK and allies sanction settlement
  firms" takes Australia's sanctions).

---

## 3. Landing A: the main-point judgment at the open-or-feed decision

**Class: red** (run acceptance of management's casts; the story open path
writes the story trajectory). Protected surfaces: the `texture:new` Cast
refusal in `inbox_records.go`, the story trajectory's subject refs, the
management task's first message. Evidence class: the paper's console and
ledger after deploy. Rollback: revert (the new subject refs are inert without
the reader). Review: two seats. `Ring-Expedite`: one published story in five
repeats one the paper holds.

**Files and functions (origin/main db31a7cd1).**

1. `internal/agentcore/management_opening_page.go`
   - Delete the title-word matcher: `heldIndex`, `heldStory`,
     `unspacedScript`, `titleStems`, `newHeldIndex`, `find` (:173-314) and the
     constants `heldMinShared`, `heldMinOverlap` (:49-52).
   - Replace `buildHeldSection` (:344-381) with `buildCandidateSection`: per
     arrival, up to 6 candidate stories, each `{inbox, headline, main_point (or
     lede when empty), age, version, found_by: ["near:<n>", "lead"]}`, ranked
     by the number of the arrival's nearest captures the story holds, ties to
     the newest. Keep the section bound (`managementTaskOpeningSectionTokens`)
     and its "not listed" note.
   - Replace `heldVerdicts` (:385) and `requireHeldVerdicts` (:447-503) with
     `requireJudgment`, refusing a `texture:new` Cast (each refusal a
     correctness refusal naming the shape, nothing sent) when: `main_point` is
     missing or under 20 or over 240 characters; a candidate offered for its
     captures has no answer in `judgment: [{story, answer: same|next|new,
     why}]`; an answer is `same` (feed S instead); `succeeds` is set without a
     `next` answer for that story, or two `next` answers are given.
   - `detailOfDecision` and `choiceDetail` (:505-571) carry the answers and
     sources for the log line.
2. `internal/agentcore/management_tasks.go`
   - `storiesOpeningPage` (:172-205): build candidates from (a) one
     `CaptureNeighbours` request for the arrivals (the path
     `eventNeighbours` uses, `event_layer.go:858-900`; limit 12, horizon 14
     days, timeout 8 s, on failure the section says so and the page alone is
     shown); (b) each neighbour mapped to its story by its local
     `CaptureDispositionOf` (`store/management_tasks.go:794`; a feed or open
     disposition names the story's inbox); (c) each arrival and neighbour
     mapped by the search leads that returned it (below).
   - `managementTaskPrompt` (:70): rewritten for the answers, the main point
     and the two zoom rules, inside the 12,100-character task bound.
   - The heading `openingHeldHeading` (`management_opening_page.go:57`).
3. `internal/platform/search_leads.go` and corpusd's route table: a route
   returning, for capture ids, the story conversations whose searches returned
   them (`conversation_ids` where `capture_id` is in the list; the columns
   exist, :80-96; `markSearchLeadsTried` sets `capture_id` when a research run
   reads a lead). Add `idx_sl_capture` on `(owner_id, computer_id,
   capture_id)` where not null, built like the others (:103-107). A
   researchtools client method beside `WriteSearchLeads`
   (`researchtools/search_leads.go:44`).
4. `internal/store/story_open.go` `planStoryObjects` (:191): copy the cast's
   `main_point` and `succeeds` into `subjectRefs` (`types.StoryRefMainPoint`,
   `types.StoryRefSucceeds`, new constants in `internal/types`).
   `internal/store/stories.go` `StorySummary` (:23) gains `MainPoint`,
   read from the trajectory.
5. `internal/agentcore/management_story_choice.go` (:78): the log line gains
   `answers=<doc>:<answer>,...`, `sources=near:<n>,lead:<n>`, `main_point=<chars>`
   and `succeeds=<doc|->`; `scripts/status-story-choice` reads them.

**Entities.** Captures have no entity links today (station E3 stopped; the
capture side of `mentions_entity` holds no rows, **M**, survey 6.1), so an
entity candidate would be title words again. Entities join the candidate read
when captures carry them (Part 2, L4).

**Record shapes.**

```json
// the texture:new Cast's spec (management's precommitment record)
{"captures": ["<capture id>"],
 "main_point": "Who won the 2026 election and by how much",
 "judgment": [{"story": "texture:<doc>", "answer": "next", "why": "the count is over; this is the result"}],
 "succeeds": "texture:<doc>"}
// story trajectory subject refs, written once at open
{"main_point": "Who won the 2026 election and by how much", "succeeds": "<doc>",
 "opening_record": "<cast record id>"}
```

A feed carries no judgment field: its disposition's inbox is the story, and
the log line records the candidates offered.

**Prompt and every test tied to it.** `managementTaskPrompt` is bounded by
`taskPromptBound = 12100` in `internal/agentcore/prompt_cut_test.go:36`; the
role test is `internal/yaegikernel/management_task_role_test.go`; the overlay
is `internal/runtimeprompts/overlays/rlm_management_task_runtime.yaml` (read
by `runtimeprompts/prompts.go`); `management_opening_page_test.go` and
`inbox_records.go` name `held_check`. Grep every test for `held_check`,
`held_stories`, `same, follow_up` and the overlay's basename, and run those
packages (`internal/agentcore` through `scripts/go-test-runtime-shards`,
`internal/yaegikernel`, `internal/runtimeprompts`, `internal/store`,
`internal/platform`).

**Tests first.**

1. A capture whose nearest capture was fed to story S gets S as a candidate,
   S published or not; one whose only link is a search lead of S's run gets S.
2. No candidate is built from title words: two stories with identical titles
   and no shared neighbour or lead offer nothing.
3. Refusals: missing or out-of-bound `main_point`; an offered candidate
   unanswered; a `same` answer on an open; `succeeds` without `next`; two
   `next`. Each names the shape and sends nothing.
4. An open writes `main_point` and `succeeds` into the trajectory's subject
   refs, and `StorySummary` returns them; a story without them shows its lede.
5. A neighbours request that fails or times out leaves the page and a note,
   never a refused open.
6. The prompt fits 12,100 characters; the role test passes.

**Deployed proof.** `scripts/status-story-choice --since <deploy>` on the
paper's console: every open carries a main point and an answer for each
candidate; the share of decisions that are feeds; candidates offered per
arrival by source. Then the duplicates method re-run on one day's opens by a
reader who did not build A (C-A1). Numbers the author cannot write: the
console's lines and the second reader's labels.

**Claim.** C-A1: on the first full UTC day after A, opened stories that a
reader who did not build A judges to serve the main point of a story already
held are at most 8% of opens (from 17.4% exact duplicates, and 24.2% second or
later, before C1b), and stories first published that day do not fall more than
10% against the week before. p 0.45. Settle by A's deploy plus 2 days.

---

## 4. The search: the caller fixed

### 4.1 What a run does today (M, tonight, 705 searches, 180 runs, 6.3 hours)

- 3.92 searches a run; runs sending 1, 2, 3 and 4 or more: 46, 35, 31, 68.
  The 30 runs that send 7 or more send 45% of all searches. 93% of searches
  are a story's first run.
- The median gap before the next search is 32 seconds (25th percentile 5 s).
  44% of later searches (58 of 133 sampled) go out with no page read since the
  previous one. Of 6,040 pages returned to 46 ended runs, 547 were read (9%).
- What later searches ask for, against the one before: a different need 35%,
  middling overlap 33%, a `site:` hunt for one outlet 19%, a rewording 13%.
- The need test (`refuseForUnread`, `yaegikernel/search_need.go:138`) refuses
  at 15 or more unread pages (`searchEnoughUnread`, :24) and refused 47% of
  attempts (136 of 142 refusals were `holds_unread`; 15 sent over it with
  `fresh`). `SearchAll` counts unread pages per question key
  (`search_all.go:176`), so a question worded differently starts at zero and
  passes with nothing read; the plain verbs count run-wide
  (`search_store.go:558`). `mostly_seen` caught 0 of 151.
- The research prompt (`runtimeprompts/overlays/rlm_research_runtime.yaml`)
  never says when to stop: it says to write the analysed packet "with the next
  round of searches", its worked cell sends two wordings, and the cast's
  criterion is "a second independent outlet".

So a run searches before it has read what it got, and nothing says it is done.
Not a quota: a run that has read and still lacks something sends the search.

### 4.2 The rule

1. **Read the last search first.** A web search is sent only when, for every
   earlier search of the run, the run has tried its top 5 pages new to the run
   (fewer if it returned fewer), or has read from it a page of an outlet new to
   the story. Tried means read or attempted (a dead link counts once tried, as
   today). The count is run-wide for every verb, `SearchAll` included. This
   replaces the 15-unread line.
2. **The pages the paper already held count first.** The brief already reads
   the corpus for every research run (`agentcore/research_brief.go:386`,
   `buildResearchBrief`). Its captures enter the run's search store as search
   zero, marked held, so rule 1 applies to them: a run's first web search waits
   until it has tried the top held pages, when there are any. That is the read
   before a search.
3. **No `site:` hunt for an outlet the run already holds.** A query naming
   `site:<domain>` is refused when the run's earlier results or held pages
   include an untried page from that domain, and the refusal lists those pages.
4. **Override with a reason.** `choir.SearchFresh(query, why)` sends over rules
   1 to 3 with a reason of at least 20 characters, recorded.
   `choir.Search(query, true)` is refused with the message to use
   `SearchFresh`. No count of sends is limited.
5. **The stop rule in the prompt.** "Your objective is met when you have read
   a page from a second independent outlet that supports the claim. Then
   report; do not search again." The worked cell sends one wording, reads its
   top pages and stops on that test. "With the next round of searches" is
   deleted.

### 4.3 The record: what the run held and had read when it asked

The guest passes, with each search start (`yaegikernel.SearchStartPayload`),
the need state: earlier searches; pages returned, tried and read run-wide;
held pages from the brief, tried; the outlets read; the verb; `fresh_why`. The
host writes a run event `search.decision` before it opens the stream
(`agentcore/desk_search.go:113`, `startSearch`, before `client.Stream` at
:146), and `search.result` when the stream ends (the `record` callback in
`forwardSearch`, :178): pages returned, pages new to the corpus, outlets new to
the story. Refusals stay counted in the guest's summary counts by rule
(`read_first`, `held_first`, `site_held`). The decision is written before the
search and the search's leads after it, so a search cannot count its own
results as held: the failure that stopped the side measurement three times
([problem record](../problems/search-sufficiency-measurement-three-reviews-no-landing-2026-11-20.md)).

### 4.4 The arithmetic

Base: about 690 runs a day at 3.92 is about 2,700 searches (**M**, tonight's
rate; the day of 2026-11-20 counted 3,557, **M**). 1,000 a day is about 1.45 a
run at today's runs.

| Change | What it acts on | Effect (A) |
|---|---|---|
| Read the last search first, run-wide (rule 1) | later searches are 2.92 of 3.92 a run; 44% go with no read; rewordings 13% | later searches fall 40% to 55% |
| Stop rule and one-wording cell (rule 5) | the 7-or-more tail: 30 of 180 runs, 45% of searches | the tail falls to about 3 a run; overlaps the line above |
| No `site:` hunt for a held outlet (rule 3) | 19% of later searches | most become reads; overlaps rule 1 |
| Held pages first (rule 2) | first searches, 1.0 a run | unmeasured; 0% to 15% of first searches |
| Fewer stories open (landing A) | runs: 20.7% of searches went to second-or-later stories | runs fall 10% to 20% |

Together, my central estimate is 2.0 searches a run (range 1.7 to 2.5) on 550
to 620 runs a day: about 1,250 a day (range 1,000 to 1,700). Under 1,000 needs
about 1.6 a run with A's fewer runs, or 1.45 without them. Clustering alone
(A) leaves about 2,200 to 2,400. B does most of the work, and only if runs do
stop after reading. Claim C-B2 states the bet.

### 4.5 Landing B

**Class: red** (provider calls; the run's tape). Protected surfaces: the search
verbs' need test, the research prompt, the gateway attribution path, run
events. Rollback: revert. Review: two seats. `Ring-Expedite`: the paper sends
about 2,700 web searches a day against 1,000.

**Files and functions.**

1. `internal/yaegikernel/search_need.go`: `searchEnoughUnread` (:24) and
   `refuseForUnread` (:138) replaced by `refuseUntilRead`: rule 1 per earlier
   search (top 5 new-to-run pages tried, or a read page from an outlet new to
   the story); rule 3 for `site:` queries; the refusal texts
   `unreadRefusal` (:152) rewritten to list the untried pages.
2. `internal/yaegikernel/search_all.go`: the per-question count at :176 becomes
   run-wide; `SearchAll` sends one wording per call (its worked use already
   does, 79 of 79, **E**).
3. `internal/yaegikernel/search_store.go`: `SearchStart` (:501, need test at
   :558) and `research_read.go:167` `Search`: the new test; `SearchFresh(query,
   why)`; `Search(query, true)` refused with the pointer; the need state on
   `SearchStartPayload`; search zero (held pages from the brief) seeded into
   the store, marked `Held`.
4. `internal/agentcore/research_brief.go`: hand the brief's captures (ids, URL,
   outlet) to the run's search store as search zero.
5. `internal/agentcore/desk_search.go`: `startSearch` (:113) writes
   `search.decision`; `forwardSearch` (:178) writes `search.result` through
   `record`.
6. `internal/runtimeprompts/overlays/rlm_research_runtime.yaml`: the stop rule;
   the worked cell with one wording and the stop test; "with the next round of
   searches" deleted; `SearchFresh` named.
7. `scripts/status-hourly` and `scripts/status-corpus-reuse`: searches a run by
   gateway `run=`, the share of later searches with zero reads since the last,
   the tail of 7 or more, refusals by rule, `fresh_why` sends.

**Tests first.**

1. A second search with the first search's top 5 untried is refused, listing
   them; trying them lets it through; so does reading one page from an outlet
   new to the story.
2. `SearchAll` with a new wording is refused when another question's pages are
   untried (run-wide).
3. A first search with held pages from the brief untried is refused; with none
   held it passes.
4. `site:example.com` is refused when an untried result from example.com is
   held; it passes when none is.
5. `SearchFresh` with a reason under 20 characters is refused; with one, it is
   sent and the reason is in `search.decision`; `Search(q, true)` is refused.
6. `search.decision` is written before the stream opens and `search.result`
   after; a search's own leads never appear as held in its decision.
7. Every test tied to the research overlay by path: grep the basename over
   test files; the prompt size and byte-stability tests in
   `internal/agentcore`; the role tests in `internal/yaegikernel`.

**Deployed proof.** `scripts/status-hourly --history` over the first full UTC
day after B: gateway search lines a day; searches a run by `run=`; the share of
later searches sent with zero reads since the previous; stories first
published that day against the week before. The gateway's lines and the
publication count are numbers the author cannot write.

**Claims.** C-B1: on the first full UTC day after B, research searches a run
fall from 3.92 to at most 2.2, and stories first published that day do not
fall more than 10% against the week before. p 0.55. C-B2: with A and B both
deployed, one full UTC day has fewer than 1,000 web searches with stories first
published not more than 10% lower. p 0.20. C-B3: after B, under 10% of later
searches are sent with no page read since the previous one (from 44%). p 0.70.
Settle each by its deploy plus 2 days.

### 4.6 As built, 2026-11-21: where the code differs from 4.2 to 4.5

Built in lane `search-read-first`; the code wins where it differs.

- **Rule 1 has no "or an outlet new to the story".** The guest does not know the
  story's outlets, so the alternative cannot be tested there. The rule is only:
  the top 5 pages new to the run of every earlier search are tried (fewer when
  fewer came back). It is stricter than 4.2, in the direction of the owner's
  ruling ("reading much more and searching less").
- **No new event kinds.** `search.decision` and `search.result` are not new
  `EventKind`s (each would need rows in the storage, durability and sealer
  tables, which is its own red change). The decision is a desk egress event with
  `verb: "search_decision"`, written before `client.Stream` opens
  (`agentcore/search_decision.go`, called from the egress handler in
  `tools_desk.go` just before `startSearch`); the result is the existing
  `search_done` event with `pages_returned`. "Pages new to the corpus" and
  "outlets new to the story" are not recorded: the leads are written after the
  stream ends and the guest does not know the story's outlets.
- **The need state is the guest's own report**, in `SearchStartPayload`
  (`verb`, `earlier`, `returned`, `tried`, `read`, `outlets`, `held_brief`,
  `held_tried`, `fresh_why`). It is a record, not a gate: the gate is the guest's
  refusal.
- **`SearchAll` sends one wording per call as a consequence, not a special case**:
  after the first wording is sent its top pages are untried, so the next wording
  of the same call is refused (`read_first`, one count for the call).
- **Search zero** is the brief's captures in the brief's own order (top 5), handed
  to the run's cells in `SessionFrame.Held` from the brief result
  (`researchBriefResult.held`), kept per run in `Runtime.briefHeld` and
  forgotten when the run ends.
- **`SearchFresh(query, why)`** needs a reason of at least 20 characters and
  overrides every refusal, including the running rule. `Search(q, true)` and
  `SearchStart(q, true)` keep their signature so an old cell gets the message
  instead of a compile error. The refusal reasons counted on the run line are
  `read_first`, `held_first`, `site_held`, `fresh_arg`, `fresh_why`; `fresh_sent`
  replaces `fresh_over_unread`. `refuseForUnread`, `searchEnoughUnread` and
  `unreadRefusal` are deleted; `mostly_seen` is kept (it holds a wording once the
  web has shown what it has).
- **The 7-or-more tail.** The rule makes a run's 7th search cost the reading of
  the top 5 pages of each of the six before it (30 pages), and the stop rule in
  the prompt says to report once a second independent outlet is read. Nothing
  forces a stop, by design (no quota): a run that reads and still lacks something
  sends the search, and one that disagrees with the held pages says so in a
  recorded `SearchFresh` reason. If the tail persists with reasons, the prompt's
  stop test is not enough (section 7); `scripts/status-search-reads` prints the
  tail (runs with 7 or more, their share of searches) and the `SearchFresh`
  count.
- **Deployed proof:** `scripts/status-search-reads --since T`.

---

## 5. After A and B: over-merges and the clustering measure

### 5.1 An over-merged story: seen, and split

Leaning to revising will sometimes blend two points in one story's prose.

- **Seen.** Once a day management casts research (its world-evidence desk; a
  different desk from the one judged, learning engine rule 1b) to read every
  story revised that day that took three or more fed captures, up to 40: its
  main point, its latest version and the fed captures. The run reports, per
  story, `one point` or `blends`, naming the captures that belong elsewhere.
  That report is the outcome that settles the task's forecasts for those
  feeds. The status script prints the day's count; the owner can read any.
- **Split.** On `blends`, management's parent turn re-disposes the named
  captures (override kept) into a new story opened with its own point and
  `split_from: S`, and casts S's Texture desk: "remove the material of
  captures X and Y; your point is P". Texture writes a new revision; the
  blended revision stays in the history, superseded. Unblending prose costs one
  Texture turn. That is the price of leaning to revise, and the daily count is
  how the owner sees whether it is worth it.

The daily read and the split are landing A3, after A2. Landing A records what
they need.

### 5.2 Clustering past exact duplicates

The owner expects more than the 17% exact duplicates. What the evidence
supports: of 982 stories opened in a day, 17.4% repeated a held story, 2.6%
were clear follow-ups and 4.2% borderline follow-ups, so 24.2% were the second
or later story on a happening already covered; they took 20.7% of the day's
searches (**M**, duplicates file sections 2 and 3). The reader of that file
counted as distinct some stories that overlap in substance without sharing a
happening (successive Houthi strikes, successive ransomware claims, a carrier
arriving and another leaving); how many such stories a "lean to revising"
judge would fold is not measured. Of 22 titled pairs judged under the rule
(Part 2, section 9), 11 revise, 6 open a successor, 1 is new and 4 are unclear, which the
owner's ruling now folds. My estimate is that 20% to 30% of opens fold (**A**).
The measure: the duplicates method re-run on one day after A, labelled by main
point, counting stories that overlap in substance whether or not they share a
happening (claim C-A1).

---

## 6. The owner's decisions, with defaults

1. **Settled (2026-11-20 and 2026-11-21).** One main point per story; revise on
   the same point; a successor on a changed point; no time window; unclear
   leans to revising.
2. **Settled (2026-11-21).** Fewer searches per run is the search lever; no
   wait before a first search, no fewer stories researched, no ceiling, the
   1,000 target kept.
3. **The main point is written once, by management, in the opening cast; Texture
   never rewrites it.** Default: yes. (The alternative, Texture rewording it,
   is what both reviewers showed drifts.)
4. **A daily read of over-merges by research, and splits by management (A3,
   section 5.1).**
   Default: yes, daily, up to 40 stories.
5. **Happenings move to the Autopaper object graph later, with research as
   their judgment authority (Part 2).** Default: yes, after A and B.
6. **Spending.** A and B add no model call; about 14,000 input tokens a
   management task for the page (landed) and a few hundred for candidates.
   Default: proceed.

## 7. What I am least sure of

- Whether runs stop after reading. B removes the excuse to search before
  reading; whether the read then satisfies the objective is the bet (C-B1,
  C-B2). If the 7-or-more tail keeps searching with `SearchFresh` reasons, the
  prompt's stop test is not enough.
- Whether `swe-2-flash` at low reasoning writes main points at the level of the
  beat. A point written too narrow makes every detail a new story; too broad,
  and a story swallows a war. The daily read (5.1) is how it is seen.
- Whether vectors and leads find the earlier story. The candidate read's recall
  on the 238 labelled pairs of the duplicates file is measurable before A
  lands (replay the pairs' opening captures through `buildCandidateSection`);
  it is not yet measured.

Facts where the reviewers differed, read from code: a vendor's refusal for
size reaches the caller as a gateway 502, not as a size refusal
(`agentcore/jev_failure.go:9-14`; the 32,000-token limit for state plus one
question is `jevbudget.VendorQuestionTokens`, `jevbudget/jevbudget.go:28`).
Landings A and B add no Jev question, so the flush's size is a question for
Part 2's L5, to be measured on a real burst flush before the story question
rides it. Unpublished stories have no row in corpusd (both reviewers), so A's
candidate read is a host join over the computer's own dispositions, not a
corpusd story search.

---

**Later.** Everything below follows A and B and gates neither.

---

## 8. The layer after A and B

### 8.1 The model in brief

A **happening** (the code calls it an event) is one occurrence at one place and
time; a development is a new happening that `follows` the old one; an ongoing
situation is a parent happening others are `part_of`. An **entity** is a
named, persistent thing; an **outlet** is an entity of type organization keyed
by its web domain. A **story** is one main point and reports the happenings
that serve it. Every judgment (a capture reports a happening, two names are one
thing) is a record on the deciding computer's tape with its probability and
evidence; the rows in the Autopaper object graph are their projection, and a
change is a new record that supersedes the old. Today the judgments sit in ten
homes in six shapes and events live on the paper computer, apart from the
captures, stories and vectors they should join (**M**,
[semantic-layer-as-built-2026-11-20.md](../evidence/semantic-layer-as-built-2026-11-20.md)).
Prior art is read only for how such systems failed.

### 8.2 Kinds and edges: the smallest set

Every kind below lives in the Autopaper object graph (corpusd's `graph`
database). Row custodian is the corpusd kernel service, as it already is for
`choir.entity` (**C**, `kernelservice/kernelservice.go:103-110`). Judgment
authority is the desk whose tape records the rows project (section 8.3).

| Kind | What it is | Judgment authority | Id rule (key; canonical is `obj:<kind>:<b64 owner>:key-<sha256 of key>`) | Status |
|---|---|---|---|---|
| `choir.web_capture` | one fetched page | none: a fact of fetching | URL or content, unchanged (**C**, `objectgraph/web_capture.go:11-45`) | kept |
| `choir.publication`, `choir.publication_version` | a story and its versions | Texture writes; publish | `pub-<32 hex>`, unchanged | kept |
| `choir.publication_check` (+ `choir.support_judgement`) | a claim: one cited sentence of a version, and its support verdict per cited capture | the support judge | (version, anchor), unchanged | kept; named the claim |
| `choir.world_event` | a happening | **research** (the host flush declares research) | `ev-<20 hex>` of (computer, opening capture), unchanged (**C**, `event_layer.go:294-299`) | **moved** from the computer's store |
| `choir.entity` | a named persistent thing, outlets included | research | `ent:<24 hex>` for the first of a name; qualified for a second; `outlet:<domain>` for an outlet; `wd:Q<n>` rows stay | kept, extended |
| `choir.ontology_act` | one change of identity or type: `merge`, `split`, `retire`, `retype`, `part_of`, `follows`, `same_as`, `not_same` | research, or the owner | (op, subject, tape record id) | **new**; replaces the unbuilt `choir.entity_merge` and `choir.entity_split` |
| `choir.entity_review`, `choir.entity_stopword` | the lexicon's learning loop | corpusd from exports | unchanged | kept |

Deleted kinds: `choir.event_membership` (becomes the `in_event` edge),
`choir.capture_label` and `choir.capture_label_override` (their event and
topic answers are absorbed by the flush, L4), and `choir.event_projection`
(the E5 lane's kind, never landed; the event itself takes its place).

| Edge | From, to | Metadata (part of the edge id) | Written by | Status |
|---|---|---|---|---|
| `cites_capture` | version, capture | `{"relation":"cites"}` | publish | kept (202,741 rows, **M**) |
| `in_event` | capture, happening | `{"role": opened\|member\|tentative, "record": <tape id>}` | the flush's export | registered, 0 rows; first written in L3 |
| `reports_event` | publication, happening | `{"role": primary\|secondary, "basis": opened\|cited\|judged}` | the host at open and at publish | **new** (the E5 lane) |
| `follows` | happening, earlier happening | `{"record": <tape id>}` | flush or Judge | **new** (today a field) |
| `succeeds` | publication, earlier publication | `{"record": <opening cast id>}` | exported from the story trajectory's `succeeds` ref (landing A) | **new** |
| `part_of` | happening, parent happening | `{"record": <tape id>}` | Judge only | **new** |
| `mentions_entity` | publication or capture, entity | `{"role": central\|incidental\|tentative}` | story export; flush export for captures | kept; capture side new |
| `supersedes` | version, version | as today | publish | kept |

Deleted edges: `related_story` (4 rows; its relations `same_incident`,
`later_development`, `same_subject` are a shared happening, `succeeds` (or no
second story at all), and a shared central entity), `relies_on` (1 row; "ranks nothing and no desk sees
it", **C**, `registry.go:357-359`). The eight provenance edges written and
never read (`was_derived_from`, `was_associated_with`, `attested`,
`granted_consent`, `authored_review`, `derived_from_proposal`, `owns`,
`has_agent`, 22 to 36,461 rows each, **M**) are listed for deletion only after
a read of whether any outside contract (the publication bundle, a verifier)
parses them (**U**).

Net: one new object kind, four new edge kinds; three object kinds and two edge
kinds deleted, eight more edge kinds reviewed for deletion.

### 8.3 Where it lives, who decides

**Decision: happenings move from the paper computer's own store to the
Autopaper object graph, beside the captures, stories, vectors and entities.**

| Option | Joins in one expression | Who can read | Cost |
|---|---|---|---|
| Keep events on the computer, corpusd holds a projection (the E5 lane, the entity design 3.3) | only the projection joins; the home does not | the paper, through a key; the public through the projection | two copies of every event; memberships immutable, so no correction; the computer's index is capped at 10,000 events and already drops the oldest (**C**, `event_layer.go:85-86`; about 10,153 events, **E**) |
| **Move them to the Autopaper object graph; the judgment stays on the tape** | yes | as captures and entities: public objects for all, owner-private ones for their owner | a migration of about 10,000 events and their memberships (section 10.1) |
| Move everything to the computer | no: captures and vectors are shared across 35 owners (**M**, survey 1.1) | the computer only | not possible without copying the corpus |

Doctrine allows it. I27 says "the Autopaper object graph and a computer's graph
are one model with two scopes" and "the graph is the tape's projection"
(**C**, `choir-doctrine.md:449-483`). A one-time import that empties the old
home is a migration, not a second home (same lines). The vector and projection
tables are "rebuildable caches, derived indexes", a named exception (**C**,
`choir-doctrine.md:473-474`).

**The doctrine tension, and how this resolves it.** I26 says management "holds
no document/event mutation authority" (**C**, `choir-doctrine.md:419-421`),
while the manifest makes management the custodian of `choir.world_event` and
`choir.event_membership` (**C**, `deskmanifest/manifest.go:356-363`), written
by a host loop that declares management (**C**, `registry.go:122-127`). The
survey read "event" in I26 as the runtime event. Either way, the clean split
is by what the judgment is about:

- **Research decides what the world is.** Research has world-evidence
  authority (I26) and is already the authority for `story.entities` and
  `story.relation`. A judgment that a capture reports a happening is world
  evidence. It mutates nothing in the world. So the flush, a host loop with no
  run, declares **research**, and research is the judgment authority for
  happenings, memberships, entity links and ontology acts.
- **Management decides what the paper does about it.** Open a story, feed it,
  decline it: `choir.capture_disposition` and `choir.management_task` stay
  management's (**C**, `manifest.go:360-362`). Deciding which story's main
  point a capture serves is a disposition, so it is management's kind, made by
  management's task by reasoning (section 2). That is the coherence role I26
  gives management. The main point itself is part of the story, so its
  custodian is Texture, which holds the document.

This moves no authority to a new desk and adds no desk. It is a custody change
of two kinds (red, in L3), and the owner's to ratify (section 6, decision 5).

### 8.4 Canonical and derived, and what records each judgment

| Thing | Canonical or derived | Record that made it |
|---|---|---|
| a capture, a publication | canonical (a fetch, a publish) | the capture route; the publish path |
| a happening's membership, mint, `follows` | **canonical: the judgment** | `semantic.flush` tape record (new, L2), on its own channel `semantic:<computer>` |
| an entity link from a capture | canonical: the judgment | the same `semantic.flush` record (L4) |
| an entity link from a story | canonical: the judgment | `story.entities` (exists) |
| which story a capture serves (same, next, new) and `succeeds` | **canonical: the judgment** | the opening cast's `judgment`, or the feed disposition (landing A) |
| a story's main point | canonical: the statement, written once | management's opening cast (section 2.2) |
| what a search held and had read when it was sent | canonical: the decision | `search.decision` run event (landing B) |
| a merge, split, retirement, retype, `part_of`, `same_as`, `contradicts` | canonical: the judgment | `semantic.act` tape record (new; deferred), citing a `system1.judgment` receipt |
| the graph rows for all of the above | derived: a one-way, re-runnable export of the records | the export carries the record id in the edge metadata |
| `reports_event` with basis `cited` | derived deterministically from `cites_capture` and memberships | none needed |
| `reports_event` with basis `opened` | derived from the opening capture's membership | none needed |
| outlets and origins of a happening, counts, `occurred` | derived at read | none |
| vectors, `proj_capture_index`, `proj_capture_resolution`, the lexicon | derived indexes (I27 exception) | none |

A Jev answer is not reproducible (identical requests gave 13 distinct responses
in 30, **E**, SYSTEM1.md surprise 6), so the judgment record is the authority
and a re-ask is a new judgment. The graph rows can be dropped and rebuilt from
the records at zero model calls. Two requirements follow from the code:

- **The semantic records need their own channel.** System 1 judgments,
  `story.entities` and `story.relation` share `system1:<computer>`, and every
  System 1 call reads up to 10,000 events of that channel to look for a replay
  (**C**, `system1_verb.go:66, 87-89, 230`). A flush record a minute would
  fill it in about a week (**A**, about 1,330 flushes a day from the entity
  design section 8) and slow every call. `semantic:<computer>` is read by
  record id or by a sequence cursor, never by a bounded scan.
- **A rebuild pages by sequence**, not by the 10,000-event bound, so it can
  read the whole history (the entity design left this **U**).

**What the flush record holds.** Today the event layer stores p, model and cost
on each membership, but not the question or the shortlist, so the hand read
could not tell a shortlist miss from a judge miss (**E**, join-rate file
section 2). The record holds, per capture: the candidates offered (ids, in the
order shown), the full distribution, the decision taken, and the rule that
took it (for example `member p>=0.92`). Per merge question, the same. It holds
ids and numbers, not text; the text is in the captures.

**The restore gap (residual R1, carried from the entity design).** The
Autopaper object graph is outside a computer's restore set. Restoring the paper
to an older head rewinds its tape but not corpusd. Every exported row carries
its record id, so a reconcile can tombstone rows whose record the tape no
longer holds. It is named, not built in these landings.

---

### 8.5 Three tiers, each where it is cheapest

| Tier | Used for | Cost | Recorded as |
|---|---|---|---|
| Vectors and code (no model) | finding candidates: a capture's nearest captures and their happenings; entity lexicon and token, initials and neighbour lookups (built in E2, **C**, current-architecture "Entity layer ... E2" row); the outlet key; occurred times; counts | an index read | nothing: a candidate is not a belief |
| System 1 (Jev) | typed questions over candidates with a recorded distribution: which happening, which entity, same thing or not | one call per flush, every question in it | `semantic.flush`, or a `system1.judgment` receipt a `semantic.act` cites |
| RLM agents (research; management for stories) | hard cases and the ontology itself: split a conflated entity, mint a parent happening, re-type, decide a contradiction, audit a sample; and every story judgment (section 2), in the management task that already runs | a research run; the management task's existing turn | `semantic.act`, citing receipts and evidence ids; the story judgment in `choir.capture_disposition` |

The order follows the owner's word (2026-11-20): "We're using RLM. We have a
whole ontology built in" and "we do have the System 1 models as a tool as
well." Reasoning agents make the judgments that need reasoning; System 1 is
the cheap first pass whose answer they read. Prior art is read only for how
such systems failed.

A vector score is never a decision. The duplicates file shows why: story-vector
cosine 0.85 finds 93 of 100 published duplicate pairs with precision 0.46, and
0.92 has precision 0.71 (**M**, duplicates file 4.2). Vectors propose; a judge
decides.

### 8.6 The flush: one call, every question about its captures

What a flush is today: a minute after the first queued capture, one Jev call
asks, for each capture, which of up to 30 recent events and 3 vector-neighbour
events it reports, a sibling capture, or new (**C**, `event_layer.go:39-59,
1010-1072`). Separately, the intake labeller asks Jev, for each polled capture,
which story reports the same event and which shares its topic, in another call
before arrival (**C**, `intake_labels.go:5-20`). Two calls over the same
captures are the defect SYSTEM1 rule 2 names.

After L4, one flush call asks, for each capture in it:

1. **its happening**: the candidates (8.7), each listed once in `state.events`
   with its headline, its newest member titles and, when a story reports it,
   that story's headline; plus `after_<e>` for a development, `k<n>` for a
   sibling capture, and `new`;
2. **each mention's entity**: its candidates from the E2 lookup, the six
   `new_<type>` choices, `same_as_<k>.<m>` for the same form elsewhere in the
   flush, and `not_an_entity` (the entity design section 4.3);
3. **merge questions** for each pair of candidate happenings or entities the
   flush exposed as possible duplicates ("do these name the same real
   occurrence / thing?"), so a duplicate is asked about exactly when it would
   split a new link, with no sweep and no schedule;
4. **the story question** (from L5): for each intake capture, which candidate
   story's main point it serves, with choices `same_<s>` and `next_<s>` per
   candidate story (section 2), `with_k<n>` for a sibling capture of the same flush,
   and `new`. This is System 1's first pass; the management task decides.

Questions are answered blind to each other (SYSTEM1 rule 1). A split is made
only where a vendor limit forces it (64,000 tokens a request, 32,000 for the
state plus one question, 255 options a choice question, **C**, SYSTEM1.md
3.3), by captures, never by question kind, and the code names the limit
(`planJevCalls`, **C**, `event_layer.go:382`).

**What arrives when.** The flush moves in front of arrival: management's task
for a poll's captures starts when the flush has resolved them. That adds about
one to two minutes to intake (**A**: the 60-second timer plus one call). It is
the price of knowing the happening before the open-or-feed choice, and it is
the owner's call.

**Work list, retries and restarts** come from the entity design section 4.1,
unchanged: the flush reads the ready list (captures with a vector, or that will
never have one, and no `proj_capture_resolution` row) from corpusd in the one
request it already makes; a failed call writes nothing and the next flush asks
again; after a crash nothing runs at boot, and the next arrival-triggered flush
picks up what a crashed flush never answered. That is a derived index catching
up, not resumed work (the entity design's owner question 2, default kept).
Research reads join happenings (for corroboration and the search state) but
never route anything: a page research read is not news to the paper.

**Before any of this is built (both reviews).** (1) Do not delete the
in-memory event index in L2: `SearchWireCorpus` and the events route read it
until L3 and L6. (2) Write an export-pending marker with each `semantic.flush`
record and replay unexported records before the next Jev call, so a failed
export does not cause a second, different answer for the same captures.
(3) Measure the token size of a real burst flush (27 captures were seen)
before the story question rides it: the vendor's 32,000-token limit for state
plus one question is real, a size refusal reaches the caller as a gateway 502
(`agentcore/jev_failure.go:9-14`), and `eventPrepareChunk` trims candidates
when a one-capture chunk does not fit (`event_layer.go:540-547`), which
SYSTEM1 rule 6 forbids. Relay the vendor's size reason through
`gateway/jev.go` and remove the trim in the same landing. (4) Store each
flush's candidate list once, by short keys, not per capture.

### 8.7 What makes a singleton join

The measured state: 84.1% of questions minted a new event and 2.8% joined
(**E**, join-rate file section 1); 282 of the 300 newest events had one member
(**E**, research-runs file); 10 of 50 hand-read mints should have joined
(**E**, join-rate file 2a). The causes found, with the fix for each:

| Cause (measured or read) | Share of the 10 misses (**E**) | Fix | Landing |
|---|---|---|---|
| No vector at flush: 78% of questions had none (03:28Z to 05:12Z) | 1 certain, others unknown | the embed kick on capture write (landed 2026-11-20 as E1; effect unmeasured, **U**) plus the ready list, which waits for the vector | E1 done; L2 |
| Same-flush siblings: the only link is a `k<n>` answer at p of at least 0.92 | 6 | cluster the flush's captures by vector before asking; list each capture's near siblings first among its `k` choices (Jev leans to the first option, **E**, SYSTEM1.md surprise 6); give sibling answers the same three bands as event answers | L2 |
| The recent slot is the 30 newest events, about 3 minutes of a burst | 4 to 5 | candidates from every distinct happening among the capture's 50 nearest captures within 14 days (today 12 neighbours, at most 3 events, **C**, `event_layer.go:55-59`), bounded only by the vendor's 255 options and token budget | L2 |
| A neighbour in no event offers nothing (1,074 unmapped in 100 minutes, **E**) | unknown | falls as joins rise; a later backfill gives old captures entities, which feed the next slot | L4 |
| Entities unused for events | unknown | a slot of recent happenings whose members share at least two central entities with the capture (instant from the lexicon, works when the vector is late) | L4 |
| Fragments minted before their partner existed | unknown | merge questions ride the flush for pairs of recent happenings whose members are near (B2 item 3) | L4 |

**One distribution, several thresholds by consequence.** SYSTEM1.md 5.4: the
riskier the action, the higher the bar. The flush stores the distribution once
and each reader applies its own bar:

- **counted toward corroboration** (outlets, origins): p of at least 0.92, as
  today, because a false merge here fakes a second source;
- **offers a story as a candidate to the story judge** (section 2): any member or
  tentative (p of at least 0.5, the tentative band of today, **C**,
  `event_layer.go:78-79`). A membership never routes a capture by itself: two
  stories with different main points can report one happening.

No new threshold is invented on day one. The record makes each one movable
later with no re-ask (SYSTEM1.md 5.4: "keep stored answers so re-routing under
a new threshold costs no calls").

### 8.8 Quality, measured by someone other than the writer

Each judgment is a forecast with a probability: the learning engine's first
stroke (**C**, `learning-engine.md` sections 1 to 3). The design gives it
three teachers, none of which is the flush:

1. **Texture, on every feed.** When management judges that a capture serves a
   story's main point, Texture either uses it (cites it, or revises from it)
   or marks it unused with a reason (I15: every source is cited, toolbar or
   marked unused, **C**, `choir-doctrine.md:333-339`). That is an outcome
   neither the flush nor management can write. Each feed settles management's
   story forecast and the flush's story answer true or false. L5 adds two
   reason codes, `not this main point` and `main point moved` (section 2); whether
   today's reasons can be read that way is **U**.
2. **An audit sample, judged by a desk that did not make it.** A uniform 3% of each day's
   flush decisions (memberships, mints, entity links) and of management's
   story judgments, seeded so anyone can
   redraw it, is cast to a research run once a day by management (the first
   management turn after 00:00Z; a crashed audit run is not resumed, the next
   day's covers it). The run reads the captures themselves, never System 1's
   answer as a premise (SYSTEM1.md 5.5), and records verdicts as
   `semantic.act` rows of kind `audit`. That settles the forecasts and gives
   precision per probability band, which is what moves the thresholds in B3.
3. **The owner, on demand.** The sample route (the entity design section 7)
   serves any draw for a hand read.

The paper goes back to the student (rule 2): the flush's calibration table by
band, and each desk's settled forecasts, are readable by the desks.

Rule 1b note: the audit run is research, and research is also the judgment
authority for the flush. The flush is System 1 driven by a host loop; the
audit is an RLM run that reads the evidence afresh. They are different
actors, but the same desk. If the owner wants a stricter separation, the audit
can be a Texture-free management task instead, as 8.8 now says.

---

Seat 1's point is taken: the flush declares research, so its audit is a
management task; research audits management's story judgments (section 5.1).

### 8.9 `choir.World`: the verbs

| Verb | Signature | One call does | Cost | Custodian of what it reads or writes | Grant and holders |
|---|---|---|---|---|---|
| `Get` | `Get(ids ...ID) (Objects, error)` | reads up to 200 objects of any kind by id, merges resolved | one corpusd read by exact ids; tens of ms (**A**) | rows: corpusd | `semantic.read`: research, management (Texture gets no world read: it "never fetches the world", `deskmanifest/manifest.go:372-373`) |
| `Find` | `Find(q Query) (Hits, error)` | hybrid search with graph filters | one query embed (median 0.26 s for longer queries, **M**, current-architecture "Vector index" row) plus one statement; 4 s vector deadline | corpusd | `semantic.read` |
| `Neighbors` | `Neighbors(id ID, rel Rel, o Opt) (Edges, error)` | one typed hop with filters | one indexed edge read | corpusd | `semantic.read` |
| `Walk` | `Walk(from ID, hops ...Hop) (Paths, error)` | up to four hops in one request | one request; at most 200 per node a hop and 2,000 nodes in all; 8 s | corpusd | `semantic.read` |
| `Ask` | `Ask(qs ...Question) (Answers, error)` | System 1 over any objects, every question in one call | one Jev call; tokens by the rendered objects (about $0.0001 for a 2,000-token call, **A**) | the System 1 kernel service; `system1.judgment` on the tape | `system1.call`: research (as today, **C**, `deskmanifest/grants.go:144`) |
| `Judge` | `Judge(acts ...Act) (Receipts, error)` | records ontology acts and exports them | a tape append and one corpusd export; no model call | research is the authority; rows corpusd | `semantic.judge`: research only |
| `Held` | `Held(story ID, objective string) (Held, error)` | what the paper holds on the story's main point, as of one corpus sequence, with a verdict on whether a web search would bring anything new (section 4) | one corpusd request; no model call | corpusd rows; the run's own tape for the decision | `semantic.read`; the host's search path calls the same read itself |

The reads replace the event half of `SearchWireCorpus`, which is an in-memory
substring match over the newest 3,000 events (**C**,
`researchtools/corpus_events.go:54`). `SearchWireCorpus` keeps its capture and
published halves until `Find` has carried a week of research runs, then folds
into it. `Relate` and `RelateStories` stay until `Judge` covers them (deferred).

## 9. The rule against 22 real pairs (evidence for section 2)

The duplicates file names its pairs by title
in section 2.2 (25 rows), section 4.3 (6 traced openings) and three
look-alikes; its appendix gives the 238 labelled pairs as ids only, and the
titles are on the paper, which this pass did not read. So I had 34 titled
pairs and judged 22 of them; the evidence label is the file's (D duplicate, F
follow-up, X counted distinct).

| # | Earlier, later (gap h) | Label | Rule | Why |
|---|---|---|---|---|
| 1 | Six killed in Georgia gas explosion, the same title (5.4) | D | same | identical point |
| 2 | Pew: Islam largest religion by 2035, the same (20.2) | D | same | identical point |
| 3 | UK sanctions eight Israeli settlement firms, the same (47.7) | D | same | same point two days on; a 24-hour window would have opened a story |
| 4 | UK sanctions firms, "Australia joins"; Australia sanctions settler leader Weiss (50.4) | D | same | Australia's act is part of the point the earlier headline already holds |
| 5 | Russia's mass missile strike on Ukraine; massive overnight attack (3.5) | D | same | one strike |
| 6 | Russian missiles and a drone land in Poland; Polish PM calls it "intentional provocation" (9.5) | D | unclear | is a government's charge a new point, or a reaction inside the incident? |
| 7 | Nikhil Gupta sentenced to 19 years 7 months; the same (1.3) | D | same | one sentencing |
| 8 | ICJ to deliver verdict Thursday on Myanmar; ICJ finds genocide (36.0) | D | **next** | the point moved from "a verdict is due" to "what the court found" |
| 9 | First deep-sea mining lease auction off American Samoa (route says "schedules", headline says "one company wins rights"); US sells first leases for $6m (40.9) | D | same, with drift | the later serves the earlier's current point, but the earlier story already crossed from "scheduled" to "held" inside itself: under the rule that was a `next` |
| 10 | US loses measles-free status; the same with a German ex-minister's warning (54.2) | D | same | the loss is the point; the warning is a side |
| 11 | Google adds an AI sidebar to search; Google redesigns its search page (25.9) | D | same | one redesign; the earlier story was 25 hours old and on no page |
| 12 | Trump calls Miller "the most powerful man in the world"; Trump's press conference after Asia, hails Miller (2.2) | D | unclear | a remark's story against the event that holds the remark; the task judged them different |
| 13 | Kynev arrested; Moscow extends his detention to December 24 (42.3) | F | same | his detention is the point; the extension serves it |
| 14 | Eight killed in Brooklyn collapse; new video of the collapse (20.1) | F | same | new material, same point |
| 15 | India and Japan set to review defence ties at 2+2; they agree to expand cooperation (44.8) | F | **next** | a meeting due, then what it agreed |
| 16 | Senate clears the way for the permitting bill; Senate passes it 62-33 (39.8) | F | **next** | the point moved from the vote's prospects to passage |
| 17 | Kress passes first test as interim CEO, board keeps Huang out; Nvidia confirms Huang resigned (46.6) | F | **next** | an interim arrangement, then a resignation |
| 18 | Nations League draw set for Friday; England drawn against Norway (3.1) | F | **next** | a draw due, then the draw |
| 19 | French Senate delays police firearms vote; Parliament approves it (20.0) | F | **next** | delay, then passage |
| 20 | Bukele due to register for a third term; Bukele registers (4.5) | F | unclear | "seeks a third term" holds both; "registers today" does not |
| 21 | Philippine charge against Thomas Markle; US judge finds him fit for trial (48) | X | unclear | two jurisdictions: one prosecution's beats, or two matters? |
| 22 | A third US carrier reaches the Middle East; USS George Washington leaves | X | new | different points under one situation, which is a `part_of` parent happening, not one story |

Tally: 11 same, 6 next, 1 new, 4 unclear. Of the 12 duplicates, 9 revise, 1
is a successor (the ICJ) and 2 are unclear. Of the 8 follow-ups, 2 revise and
5 are successors. What the pairs teach:

- **A follow-up is not the same as a changed point.** The file's follow-up
  label meant "a real development". Under the rule some developments revise
  (Kynev, the Brooklyn video) and most open a successor.
- **The commonest successor is preview, then outcome** (rows 8, 15, 16, 18, 19):
  "is due", "is set", "delays", then "did". A preview's main point should say
  it is a preview, so the judge sees the change when the outcome arrives.
- **Time windows are wrong.** Six of the eleven `same` answers are 25 to 54
  hours apart. The earlier draft's 24-hour development window would have
  opened a second story for row 13, and its 72-hour live window works for
  these only because the earlier stories were revised recently; a story still
  growing three days after its last revision would be missed. Candidates are
  found at any age; recency only ranks them.
- **The unclear cases are decided by how the main point was written.** Rows
  12 and 20 turn on the level of the sentence ("Trump's post-Asia press
  conference" holds the Miller remark; "Trump praises Miller" does not), and
  rows 6 and 21 on whether a reaction or a second proceeding is a new beat. So
  the main point is the thing to write well, to show the judge, and to audit;
  and the judge needs a default for unclear. The owner has ruled `same` (lean to revising), with the
  reason recorded: the owner wants fewer, deeper stories; a wrong revise is
  undone by Texture's `main point moved` at the cost of one Texture turn, while
  a wrong open costs a research run (about 3.1 searches, **M**) and, in 53% of
  duplicates, a public repeat.
- **Drift happens inside one story today** (row 9). Without a recorded main
  point Texture revised "an auction is scheduled" into "a company won it". The
  recorded point makes the crossing visible to the next judgment and to the
  audit.

This sample is the evidence file's own examples, chosen to show its labels, so
it over-represents clear cases; 22 pairs measure nothing. It is a first
reading of the rule, and C-A1 asks for a proper sample.

## 10. Landings after A and B

| # | Landing | Class | Notes |
|---|---|---|---|
| L1 | Management's page by time | orange | **landed** as C1b (d8367e2c8, repaired by 2b862a00e and e33533850); its page stays, its title-word offer is deleted by A |
| A, A2, A3 | The main-point judgment; Texture's two reasons routed to management; the daily over-merge read and split | red | sections 2, 3 and 5 |
| B | Fewer searches per run and the read before a search | red | section 4 |
| L2 | Flush provenance and reach: the ready list, `semantic.flush` on `semantic:<computer>`, 50 vector neighbours, siblings clustered by vector; the four conditions of 8.6 | red, two seats | C-S1: intake mints fall from 84% to at most 65% in a day, p 0.55; it serves corroboration, not consolidation |
| L3 | Happenings move to the Autopaper object graph (absorbs the `story-events-e5` lane, changed: read events from corpusd, not the capped index; no projection kind; link at open; memberships carry their record) | red, two seats | after L2; adds happening candidates to A's read |
| L4 | Entities and merges in the flush; outlets as entities; the intake labeller's call absorbed (kept until A's candidate read no longer needs its labels) | red, two seats | adds entity candidates to A's read |
| L5 | The story question in the flush as System 1's first pass, shown to the task as advice | red, two seats | only after the flush size of 8.6 (3) is measured |
| L6 | `choir.World` reads for research and management | orange | when a run needs graph walks the brief does not give |

Deferred, to be designed again from A's and B's readings: ontology acts by
research agents (`Ask`, `Judge`, entity splits, parent happenings), the
`research.related_story` switch's removal with its finish-rule fix, the
entity backfill, contradiction checks on claims, entity pages, and the
published-duplicate fold and the front chain.

### 10.1 Migrating entities and events without a risky rewrite

- **Entities: no rewrite.** Ids stay. Merges, retirements and splits are new
  `choir.ontology_act` rows layered on top, resolved at read. The 23,966
  `mentions_entity` edges are untouched (**M**, survey 6.1). Capture-side
  mentions are new rows written by the flush and a later backfill.
- **Events: one import that empties the old home.** At L3's deploy the paper's
  flush loop, on its first run, reads its own `choir.world_event` and
  `choir.event_membership` objects (about 10,153 and their memberships, **E**)
  and sends them through the same corpusd commit route as new judgments, in
  batches by exact id lists, under a `choir.migration_marker` so it runs once.
  Ids are deterministic, so a repeat writes nothing. The old local rows are
  tombstoned only after corpusd's counts match. No boot path resumes work: the
  import is a migration step, not a resumed obligation.
- **No range predicates on ids.** `og_edges.from_id` and `to_id` use the
  database collation (en_US.UTF-8) while `og_objects.canonical_id` is C, and a
  byte-prefix range matched nothing in production today (**M**, survey 1.2;
  `docs/problems/entity-read-lists-no-stories-in-production-2026-11-20.md`).
  Every read and every migration step uses equality on exact id lists or
  `starts_with()`. Each landing carries a test that moves the id columns to a
  non-C collation, as the E5 lane does.
- **Large tables change by adding, never by rewriting.** `proj_capture_index`
  (237,050 rows, **M**) gains a nullable `outlet_id` column filled in batches,
  then an index built concurrently. `og_edges` gains one index built
  concurrently. No column changes type.

The imported events carry one `semantic.act` per batch citing their ids, so a
rebuild from the tape keeps them (seat 2).

### 10.2 Residuals

R1, the restore gap (8.4). R2, two new same-name entities in one flush are
joined (entity design 4.3), undone by a split. R3, OpenRouter's long-context
line above about 32,000 bytes may change answers (SYSTEM1 section 10). R4, a
private computer's objects are never compared with another owner's. R5, a
podcast clip names its story by a route string until a `clip_of` edge exists.
R6, `search_leads` is a home without a kind. R7, a `next` story does not yet close its predecessor's open research; the predecessor may keep searching on a stale point until A2's record closes it.
