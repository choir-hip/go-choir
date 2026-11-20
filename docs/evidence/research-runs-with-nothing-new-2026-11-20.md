# Research runs that followed nothing new (2026-11-20, 13:30Z to 20:18Z)

Read-only measurement, `date -u` at the end 20:36Z. It answers the open claim in
[who-sends-the-web-searches-2026-11-20.md](who-sends-the-web-searches-2026-11-20.md)
section 3 ("hold the research run, not only the search; needs a measurement
first; I would expect 20 to 50% of runs") and corrects the premise of
[what-starts-research-runs-and-what-is-new-2026-11-20.md](what-starts-research-runs-and-what-is-new-2026-11-20.md).
Nothing was changed on Node B or on the paper. Query text was read in scratch
and appears here only as topic words.

## Answer in one paragraph

A cast-level hold of the kind the earlier record guessed at cannot reach
1,000 searches a day, and the measurement says why. In the 6 h 48 min after
13:30Z the paper sent 840 searches from 232 research runs (about 124 an hour,
about 2,960 a day at that rate; the gateway's own counter reads 2,764 at
20:26Z for the day so far). **210 of the 232 runs (90%) and 789 of the 840
searches (94%) are the first research run their story has ever had**: a story
is opened by management from new captures, Texture writes its V1 and casts
research once, and the paper opens about 34 stories an hour. There is no
earlier research run to compare with, so "no capture newer than the story's
last research run" cannot fire on them, and none of the 210 stories had a
publication before its research ran (research is where the second outlet
comes from; 158 of the 210 published afterwards). The runs that *are* repeats
are 22 (9.5%) with 51 searches (6.1%), and they split in two. **14 runs, 31
searches (3.7%) are management's replies to research desks' escalations**,
two waves (about 14:50Z and 19:35Z to 19:50Z): the reply says the cause is a
platform defect, tells the desks to stop, and wakes about 20 old research desks
in each wave; these produced no revision and no publication (13 more such runs
in the 19:35Z wave sent no search at all). That is the clearest "nothing new"
in the data, but it is a *reply*, not `choir.Cast("research", ...)`, so a test at
the cast does not see it. **The other 8 repeats (20 searches, 2.4%) are
Texture follow-up casts 5 to 19 minutes after the story's previous round, and all
8 produced a revision and a publication**: a hold keyed to "nothing new" would
have cost publications. So the share of runs that followed nothing new is
**6% (14 of 232) of runs and 3.7% of searches on the evidence, at most 9.5% and
6.1% if every repeat counted**, not the 20 to 50% guessed. A cast-level hold
removes **0 to about 180 searches a day (0 to 6%)**, leaving about 2,800 to
2,960 a day; with the holds already deployed (they are in the base) that is
nowhere near 1,000. The event layer cannot carry the test either: 282 of 300
reachable events have one member, and no code links a story to an event. What
moves the number is the first run: 3.8 searches a run on 210 runs; 1,000 a day
is 1.3 a run at today's story rate.

## 1. The window, and what each source covers

| Source | Span | What it gave |
| --- | --- | --- |
| Gateway journal `go-choir-gateway`, `search request from autoputer <paper>` (870 lines) and `search for autoputer ... answered` (886 lines) | 13:26:38Z to 20:18:08Z (requests); 13:26:39Z to 20:26:36Z (answers) | the true count per run (`run=`), story (`conv=`), `served=`, `seen=`. 870 requests = 870 `served=miss` answers; the 16 `served=hit` answers have no request line. The earlier journal back to 2026-11-18 20:00Z was read (7,957 request lines) only to find each story's earlier runs |
| Paper console `console.log*` | 18:56:28Z to 20:18:31Z (1 h 22 min; the guest keeps about 1 to 1.3 h now) | the chain per run: `research brief`, `actor wake`, `management story choice`, `corpus read`, texture cell lines |
| Run API `GET /api/runs/{id}` and `/events?limit=14` with the paper's read key, for 256 runs | whole window | per-run record: story (`channel_id`), created and finished time, state, the first cell's read of the inbox records (which kind of record woke the run) |
| `GET /api/texture/documents/{doc}/history` for 223 stories | whole window | revision times and labels (first revision label `system:story-open` for 225 of 232 stories) |
| Host graph database, `choir.publication_version` objects (411 since 12:30Z) | whole window | when each story version published (`source_doc_id`, time) |
| `GET /api/autopaper/events?limit=300` | the 300 newest events only: last member 19:40:26Z to 20:18:21Z (38 min); it has no `since` and caps at 300 | event members with source kind, to size the event-level test |

Window used for the per-run table: runs whose first search was at or after
13:30:00Z and before 20:18:08Z: **232 runs, 840 searches** (of the 870
requests, 30 belong to runs that started searching before 13:30Z). The
console window holds **69 research runs** (67 with a `research brief` line,
plus 2 searching runs whose brief came before 18:56Z); 49 of them searched
(208 searches) and 20 did not.

Searches an hour, 13:26Z to 20:18Z (requests, distinct runs):

| UTC hour | searches | runs |
| --- | --- | --- |
| 13Z (from 13:26) | 57 | 12 |
| 14Z | 105 | 32 |
| 15Z | 94 | 25 |
| 16Z | 91 | 30 |
| 17Z | 174 | 55 |
| 18Z | 142 | 40 |
| 19Z | 151 | 36 |
| 20Z (to 20:18) | 56 | 15 |

## 2. Runs by what woke them (232 runs, 840 searches)

How each run's trigger was read: the run's first cell prints `choir.Records()`;
the kind and sender of the record are in that output (a Texture `cast` or
`ask`, or a management `reply`). 33 runs (32 of them first runs on their
story) printed the records in a form the reader did not match, so they are in
the "not read" rows, not guessed.

| Trigger (record that woke the run) | Runs | Searches | Share of searches | First run on story | With a revision (run start to end plus 15 min) | With a publication |
| --- | --- | --- | --- | --- | --- | --- |
| Texture cast or ask, first run of the story | 178 | 651 | 77.5% | 178 | 168 | 121 |
| Texture cast or ask, repeat (5 to 19 min after the previous run) | 7 | 18 | 2.1% | 0 | 7 | 7 |
| Not read (first cell printed another form): 32 first runs | 32 | 138 | 16.4% | 32 | 32 | 24 |
| Not read, repeat (an 17-minute follow-up) | 1 | 2 | 0.2% | 0 | 1 | 1 |
| **Management `reply` to a research desk's escalation** | **14** | **31** | **3.7%** | **0** | **1 of 14** | **0 of 14** |
| All | 232 | 840 | | 210 (789 searches) | 208 | 153 |

The one revision among the 14 replies is on one story (dd22d999); I did not
read what caused it. Across all 256 runs fetched (including the 20 that did not
search): 194 first-read as a Texture cast or ask, 27 as a management reply, 35
not read.

First and repeat runs, from the gateway journal since 2026-11-18 20:00Z, by
order within `conv=`:

| | Runs | Searches | Mean per run | Runs with a revision | Runs with a publication |
| --- | --- | --- | --- | --- | --- |
| First searching run of the story | 210 | 789 (94%) | 3.8 | 200 | 145 |
| Repeat | 22 | 51 (6%) | 2.3 | 9 | 8 |

Repeat runs by gap since the story's previous run ended: under 15 min 5 runs,
15 to 60 min 3, 1 to 6 h 6 (all management replies), 6 to 24 h 8 (all
management replies). Median time from the story's V1 to its first search: 13.5
minutes (p90 25.5), which is the Texture turn, the cast and the dispatch.

The first rows were joined to a story through `conv=` in the gateway journal,
which equals the run's `channel_id` and the research desk's document id
(`research:<doc>`, `deskmanifest/grants.go:579` per the earlier record). That
join is exact (every one of the 256 runs returned a run record with the same
`channel_id`).

### The management-reply runs, in full

Both waves, from the run's first read of its inbox. The record's text (read
from the run ledger, not paraphrased from code) is management's answer to an
escalation: "Your record is one of 43 from 20 research desks this turn with one
root cause; it is not a fault in your desk"; the root cause it names is
`choir.Relate` refusing `role="central"` against the vocabulary guard (the
`vocabfence` error from `internal/vocabmigrate/fence.go:106`), and "the System 1
judge provider unavailable" so `choir.StoryAnalysis` returns no roles. It tells
the desk to "stop retrying Relate", to "deliver your round's findings", and
"stop". The finish rule that makes this a loop is `requireStoryEntitiesRecorded`
(`internal/agentcore/inbox_records.go:718`, rule at `:898`): a story research
desk's final Report waits for a recorded `story.entities` or a platform-side
Relate failure, so a desk that cannot Relate cannot close, escalates, and is
woken by the reply.

| Wave | Runs that searched | Searches | Runs that did not search | Runs older stories (gap since previous run) |
| --- | --- | --- | --- | --- |
| about 14:48Z to 14:59Z (run start; first search 14:54Z to 14:59Z) | 6 | 10 | not observable (no console, no run listing) | 5.7 h to 20 h |
| 19:34Z to 19:50Z | 8 | 21 | 13 (console: `research brief` skipped, "no objective in the unread records", 1.6 to 15.8 min each) | 4.7 h to 12.7 h |

In the console window (18:56Z to 20:18Z), 21 of 69 research runs (30%) and 21
of 208 searches (10%) are management-reply runs. Over the 6.8 hours, counting
only runs that searched, they are 14 of 232 runs (6%) and 31 of 840 searches
(3.7%). The second wave also ignored the instruction: eight desks that were
told to stop sent 21 searches.

## 3. The share that followed "nothing new", with the join stated

What counts as new, and what I could join:

| Candidate evidence of "something new since the previous research run on the story" | Joinable here? | Result |
| --- | --- | --- |
| A new capture joined to the story's event (event layer) | **No** for 232 of 232 runs in the whole window; **partly** for the last 38 min | no code links a story to an event (below); the events route reaches only 19:40Z to 20:18Z |
| A new capture fed to the story (management `feed`) | Console window only (18:56Z to 20:18Z) | the 9 repeat runs in that window (8 management replies, 1 Texture follow-up): feeds to the story between the previous run's start and this run's start: 0 of 9. The one Texture follow-up (70a9fff4, 9 searches) followed the story's own revision 5 min earlier |
| A new owner or desk instruction | From the run's first read | the 14 management replies carry an instruction, but it is "stop", not a new fact; the 8 Texture follow-ups carry a new, different objective (read for 5 of 8, e.g. "round 2: close the remaining gaps", "capture the pages so the story can quote") |
| Texture's own new revision between the previous run's last search and this run's start | From the story history | 5 of the 8 Texture follow-ups had one; the other 3 (gaps 5, 5 and 15 min, 5 searches) had a new objective instead; none of the 14 management replies had one |

So, by run:

| Class | Runs | Share of 232 | Searches | Share of 840 | Basis |
| --- | --- | --- | --- | --- | --- |
| Followed nothing new (trace-confirmed): management reply, no capture, no revision, no publication | 14 | 6.0% | 31 | 3.7% | first-read record, no feed in the console window for the 8 observable, story history and publication list |
| Texture follow-up round (repeat) | 8 | 3.4% | 20 | 2.4% | story history, publication list; each carries a new objective and all 8 produced a revision and a publication |
| First run of a story | 210 | 90.5% | 789 | 94.0% | new by construction: the story was just opened from captures |

**The 20 to 50% of runs guessed in the earlier record is not supported: 6% of
runs and 3.7% of searches on the evidence, an upper bound of 9.5% and 6.1%.**

First runs: a bound only. I looked for evidence that a first run's story was
already covered:

- Another story researching the same event. In the console window the 62 runs
  that read captures read 797 distinct captures; **1** capture was read by more
  than one story and 2 runs share a read capture with another story's run. So
  duplicate stories on one event are rare in this window.
- Research output that was already seen. The gateway's `seen=` (results the
  computer already received): 27% of all returned results (33,251) were seen
  before; 17 runs (108 searches, 12.9% of searches) had 50% or more seen, 3 runs
  (12 searches) 70% or more. That is a property of the web answer, not of the
  story: it does not say the story needed no research.
- Research that bought nothing visible. Of the 210 first-run stories, 52
  (190 searches, 24%) have no publication yet; restricting to runs that started
  at least two hours ago (before 18:00Z) it is 30 of 138 stories and 88 of 478
  searches (**18%**); at least four hours ago (before 16:00Z) 12 of 57 stories
  and 40 of 213 searches (19%). These are stories that did not reach two
  outlets. That bounds "the research did not produce a publication"; it does
  not say it was unneeded.
- Publication before research. **0 of 210** first-run stories had a
  publication before their run started; 158 had one afterwards (75%). A story
  publishes only on 2 distinct outlets (`story_publish.go:262` to `:264`,
  `MinPublishOutlets` at `:281`) and the intake rarely gives two, so research is
  the second outlet. A hold on the first cast removes the publication, not
  waste.

### The event layer cannot carry the test today

- **No story to event link.** `eventSummary` (`event_layer.go:276`) and
  `eventIndexFor` (`:221`) have no caller in `internal/agentcore` other than the
  read-only events route (`event_api.go`), and the event layer's own comment
  says nothing reads the events but the event view. A story's document has no
  event id; the only join is capture to event through membership, and a story's
  captures are not stored on the story (its V1 is empty, `story-open` seed;
  revision metadata names the opening cast record, not a capture).
- **The events are almost all singletons.** Of the 300 newest events (19:40Z to
  20:18Z): 282 have one member, 18 have 2 to 6. 116 have an intake member (an
  `rss:`, `social:` or `official:` source), 198 have a `web:read` member (a
  capture that research itself read from the web); 14 have two or more intake
  members; **10 (3%) have an intake capture newer than the first research read
  of the same event**. So "an intake capture joined this event since the last
  research run" is true for about 3% of events and false for about 97%.
- **A test that counts every capture passes by construction.** The earlier
  record already found that 537 of 750 queued captures in its window were
  research reads. In this sample research reads are 198 of 300 events' members.
  The signal has to be intake only, and intake is almost never joined to the
  event it concerns.

The honest reading: an event-keyed test would say "nothing new" for about
97% of repeat casts, which includes the 8 Texture follow-ups that published, so
it is unsafe. The test that is safe uses what the cast already holds: the
records the Texture cell read (below).

## 4. Distribution of searches per run now, and who is in the tail

232 runs, 840 searches, mean 3.6, median 2.

| Searches in the run | Runs | Searches | Share of searches |
| --- | --- | --- | --- |
| 1 | 67 | 67 | 8% |
| 2 to 3 | 83 | 198 | 24% |
| 4 to 7 | 58 | 294 | 35% |
| 8 or more | 24 | 281 | 33% |

Top 5 runs: 93 (11%); top 8: 130 (15%); the largest run: 26. The runs of 8 or more
sent 66% and 60% of searches in the two blocks before the re-ask hold and **33%
since**: the "8 runs send half" tail is gone. The same measure, by first-search
time, same method (gateway lines with `run=`):

| Block | Runs | Searches | Mean per run | Runs of 8 or more | Their searches (share) | Per hour |
| --- | --- | --- | --- | --- | --- | --- |
| 2026-11-19 12:00Z to 11-20 03:00Z | 346 | 2,460 | 7.1 | 88 | 1,613 (66%) | 164 |
| 2026-11-20 03:30Z to 13:24Z | 237 | 1,346 | 5.7 | 50 | 808 (60%) | 136 |
| 2026-11-20 13:30Z to 20:18Z | 232 | 840 | 3.6 | 24 | 281 (33%) | 124 |

The searches a run falls by about a third, and the runs an hour rose from about
24 to 34, so the hourly total fell by only 9% (136 to 124). The multiplier is
now the number of runs, and 90% of the runs are first runs.

Searches beyond the k-th in a run (840 total): beyond the 1st 608 (72%), beyond
the 2nd 443 (53%), beyond the 3rd 329 (39%), beyond the 4th 247 (29%). Runs by
their number of searches and whether the story published within the run plus 15
minutes: 1 search 39 of 67 (58%); 2 to 3 searches 58 of 83 (70%); 4 to 7
38 of 58 (66%); 8 or more 18 of 24 (75%). Selection explains some of this
(harder stories search more); read it only as "more searches did not lower the
publication rate", not as a causal curve.

The eight largest runs (searches, minutes, revisions, publications): 26 (34 min,
2, 1), 21 (19 min, 2, 1), 17 (33 min, 3, 1), 16 (21 min, 3, 3), 13 (15 min, 2,
0), 13 (14 min, 4, 4), 12 (9 min, 1, 0), 12 (15 min, 1, 0); all first runs, none
a management reply.

## 5. What the casters are

- **Texture** casts the first research run of a story, and the follow-up
  rounds. Trigger: a Texture turn on a story-open seed record. The chain, from
  the console and the story history: new captures, management task (`started
  management task ... (N captures)`), management's `story choice` (`open`: 68 in
  82 min, `feed`: 52 in 82 min), the story-open record (every story's first
  revision label is `system:story-open`, metadata `origin_desk: management`,
  `revision_role: story_seed`, `opening_record: cell:<management run>:...:cast:tray-1`,
  checked on the 232 stories' history labels and one revision in full), then
  Texture's V1 and `choir.Cast("research", objective, packet, req)`
  (`internal/textureprompts/texture.yaml:29`; `choir.Ask` reaches the same
  work). In 194 of 256 runs fetched, the first read of the inbox shows a Texture
  `cast` or `ask` record; the other runs either printed another form (35) or
  were management replies (27 of 256, 14 of them searching).
- **Management** woke research in this window only through the reply to an
  escalation (the 14 runs and the 13 non-searching ones); no run's first read
  shows a management cast to research. (Not checked in code whether management
  can cast research.)
- Others in code that were not seen in the window: a reader's "research more"
  (`internal/store/story_research_request.go`), the entities backfill, a source
  schedule, a rebase notice (`texture_rebase_notice_cast.go:30`). None of the 256
  runs' first reads names one. (Absence in a first cell printed by the model is
  weak evidence for the 35 unread; it is strong for the 194 where a
  Texture cast was read.)

## 6. Where a new-information test would sit, and what it must not break

1. **Texture's Cast and Ask**: `internal/agentcore/texture_dispatch_fold.go`,
   inside `foldTextureDispatch` (`:31`), in the loop over staged intents at
   `:44` to `:70`, after `ResolveAddress` (`:49`) and before
   `inboxRecordForIntent(scope, in, toInbox, false)` (`:64`), for
   `desk == "research"` (the line `desk, _, _ := strings.Cut(...)` is at `:57`).
   It reads: `scope` (owner, computer, `DeskAgentID = texture:<doc>`), the
   cell's `records` argument (`cellRecords.view`, `inbox_records.go:46`: what
   the cell read, which says which record woke the turn: a capture feed, a
   research report, an owner direction, a rebase notice), and the story's last
   research round (`ListRunsByChannel`, `runtime.go:2807`, or the conversation
   state the reducer already reads at `inbox_records.go:700`). Safe rule from
   the data: hold a cast only when the story **has** an earlier research round
   and the records in view are neither a fed capture, an owner direction, a
   research report nor a rebase notice. In this window that rule would hold none of the 8 Texture follow-ups
   (each carries a new objective; 5 followed a Texture revision) and is not
   where the 14 management-reply runs come from.
2. **Management's reply to research**: the same records route,
   `inbox_records.go:700-708` (the loop that builds `inboxRecordForIntent(r.scope,
   in, toInbox, open)` at `:708`; `recordsIntent` includes `IntentReply` at
   `:364`). The 14 + 13 runs are woken here; a "stop, platform defect" reply to
   a desk with an empty round does not need a new run. Cheapest alternative:
   the research run's start, where the corpus brief already skips these runs
   ("no objective in the unread records", `research_brief.go:860`) and the run
   goes on for 1.6 to 15.8 minutes anyway.
3. **The root cause of the reply wave** is the finish rule, not the cast:
   `requireStoryEntitiesRecorded` (`inbox_records.go:718`, rule `:898`) with
   `choir.Relate` refused by the vocabulary fence. Fixing it removes both the
   run and the escalation (and likely the reply). I did not read the Relate
   refusal in the ledger for these runs; I read management's text about it.

What the test must not break (each a rule or a measured fact):

- **The second-outlet publish rule** (`story_publish.go:262` to `:264`,
  `:281`): research is the only source of the second outlet; 0 of 210 first-run
  stories published before research and 158 after. Never hold a first cast;
  never hold a cast for a story that is held for fewer than 2 outlets because
  "nothing is new in the corpus": the web is the new information. A previous
  change nearly broke the hunt for a second outlet; this data repeats the
  reason.
- **The Texture contract** (`texture.yaml:25`, `:49`): a Write with no Cast,
  Wait or Done is refused, and Texture must not say work was dispatched unless
  the Cast committed. A held cast must come back to the cell as an explicit
  answer ("research ran at T; nothing new since: Wait or Done"), not a dropped
  record, or the cell refuses and retries.
- **Rebase notices and owner directions** are new information by definition:
  `texture_rebase_notice_cast.go:30` and `texture_owner_directions_cast.go:115`
  must pass the test.
- **The finish rule and the round close**: a research round the host did not
  open cannot be closed by the finish rule; do not hold a cast whose round is
  already open (`inbox_records.go:700` reads that state).
- **Restart rules** (`CLAUDE.md`, Restarts): no boot-time resumer; the hold
  must not turn a restart into a resume of held work.
- **The cache and resumed runs** (`gateway` hits are 15 of 870): not a lever.

## 7. The arithmetic, with its basis

Base: 840 searches in 6.8 h is 124 an hour, **about 2,960 a day (estimate, a
straight extrapolation of one 6.8-hour window; the gateway counter's 2,764 for
20.4 hours is 135 an hour or about 3,250 a day)**. The existing holds (gate,
re-ask hold at 13:24Z) are in the base. Target 1,000 a day is 42 an hour.

| Change | What it acts on | Searches a day removed (estimate) | Basis |
| --- | --- | --- | --- |
| Cast-level hold, Texture follow-up casts, "nothing new" test (item 1) | 8 runs, 20 searches (2.4%) | 0 to 70 | upper bound is every repeat; the 8 all produced a revision and a publication, so a correct test holds about none |
| Hold or fix at the management reply (item 2 or 3) | 14 runs, 31 searches (3.7%), plus runs that did not search | about 110 | straight scale 31 per 6.8 h; the real number depends on whether the defect recurs (two waves in 6 h) |
| Cast-level hold on first runs | 210 runs, 789 searches (94%) | none safe | nothing earlier to compare; 0 of 210 had published before; 75% published after |
| Event-keyed hold | | not measurable | no story-event link; 97% of sampled events say "nothing new" |

**A cast-level hold removes 0 to about 180 a day (0 to 6%), leaving about 2,780 to
2,960 a day. Plus the existing holds (already in the base) it does not reach
1,000.** To reach 1,000 a day the base must fall by 66%. At 34 runs an hour, 1,000
is 1.2 searches a run; at today's 3.6 a run it is about 12 runs an hour. The
data's bounds on where that could come from (each a bound, not a plan, and
none a recommendation to cap anything; a search is not a model call, but the
owner's rule against our own limits is why I give no number as a design):

- Searches beyond a run's 3rd: 329 of 840 (39%, about 1,150 a day); beyond the
  2nd: 443 (53%, about 1,560 a day); beyond the 1st: 608 (72%, about 2,140 a day).
  The 2nd and 3rd searches do not look unproductive by the publication rate
  above.
- First-run stories that never published (18% of first-run searches in runs at
  least 2 h old, about 530 a day): bound on "searches that bought nothing yet".
- The management-reply class (about 110 a day).

These overlap; adding them is not valid. Without the event link and a per-run
"second outlet found" mark the data cannot say how many of the first-run
searches the paper did not need. The log line that would: on each research run's
end, `outlets_held_at_start`, `outlets_held_at_end`, `second_outlet_found_at_search=k`
(the summary line has `outlets_held_max` and `outlets_held_last`, but only for
runs that end with a summary line, and not in the gateway); and on each
Texture cast, the kinds of the records the cell read
and the story's last research run time. With both, the share of searches after
the second outlet was held is a count, not an estimate.

## 8. What I could not determine

- **Research runs that did not search, outside the console window.** The run
  list route (`GET /api/runs?limit=...`) timed out at 120 s twice (a per-owner
  scan), and a per-story list takes about 56 s each, so I did not enumerate
  them. In the console window 20 of 69 research runs did not search (13
  management replies, 6 Texture casts whose brief was delivered, 1 not read);
  for the other 5 h 26 min the count is unknown, so "share of all runs" is
  stated over searching runs (232) and, for the console window, over 69.
- **The event-level join for the whole window.** Events reach back only to
  19:40Z, no story-event link exists, and memberships live in the guest's object
  graph, not the host's. I could not test "no capture newer than the last run
  for this story's event" for 232 of 232 runs; the 3% figure is a 38-minute
  sample of events, not a join.
- **The Texture-side trigger of each first run.** I did not list the Texture
  runs (the list route is too slow); the trigger is inferred from the
  story-open seed on the story's history and from the research run's first
  read. 35 of 256 runs printed their inbox in a form I did not parse; 32 of
  them are first runs with revisions after.
- **Whether the vocabulary-fence refusal is the root of both waves.** I read
  management's account in the reply records and the finish rule in code; I did
  not read the Relate refusal or the escalation in a ledger, and the console
  holds no `vocabfence` line for the window (cell output is not logged). Whether
  the defect is still active at 20:36Z is not known: the last management-reply
  run started at 19:50Z.
- **Objective text for about half the runs** (the first cell is free-form). I
  did not classify objectives by wording here; the earlier record did, for 41
  runs.
- **Whether the 190 never-published first-run searches were needed**, and how
  many of those stories are still young (the 18% figure restricts to runs at
  least two hours old).
- **The re-ask hold's effect** is not isolated: runs an hour and the story rate
  also changed over the day. The three blocks above are before and after, not a
  controlled comparison.

## Method, for the next reader

Gateway: `journalctl -u go-choir-gateway --since ... -o cat --utc` filtered on
the paper's computer id, query text replaced by `X` on the Mac before saving.
Run ids and `conv=` are exact; the request to answer pairing for `seen=` is by
order (every answer after the first request matched one request; 16 answers
at the edge, the `hit`s, matched none). Console: `sudo -n cat console.log.4 ...
console.log` over `ssh node-b`. Run records and first events: `GET
/api/runs/{id}` and `/events?limit=14` with the paper's read key from
`~/.config/choir-qa/test-computers/autopaper-paper.json` (one request every half
second, 256 runs, 223 histories; the key never printed). Story history:
`GET /api/texture/documents/{doc}/history`. Publication: `og_objects` rows of
kind `choir.publication_version` with `source_doc_id`, read with
`default_transaction_read_only=on`. "Produced" is a revision (or a publication
version) of the run's story dated from the run's start to 15 minutes after its
end (5 more for the publication). First versus repeat is by order within
`conv=` across the journal from 2026-11-18 20:00Z; a story researched before
that shows as first. Percentages are over the 232 runs or 840 searches of the
window unless a row says otherwise; every per-day figure is an estimate by
scaling a 6.8-hour window.
