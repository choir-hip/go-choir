# What starts the research runs, and how many searches were for what is new (2026-11-20)

Read-only measurement, 2026-11-20 05:22Z to 06:00Z (`date -u` at start
05:22Z). It continues
[who-sends-the-web-searches-2026-11-20.md](who-sends-the-web-searches-2026-11-20.md),
whose last open claim was a hypothesis "needing a measurement first": that
the paper starts research runs on stories with nothing new. The design that
follows from these numbers is the last section of
[the one-verb definition](../definitions/corpus-first-search-one-verb-2026-11-19.md).

Sources, same read paths as `scripts/status-corpus-reuse` and
`scripts/status-hourly`, read into a scratch directory and not kept:

- the gateway journal, `search request from autoputer` (carries `run=`, `conv=`,
  `desk=`) paired line for line with `search for autoputer` (carries
  `served=`, `results=`, `seen=`): 6,023 requests and 6,023 answers from
  2026-11-18 20:00Z; `run=` and `conv=` are on every line from 2026-11-18 20:xxZ,
  so the per-run window is **33 h, 2026-11-18 21:00Z to 2026-11-20 05:21Z, 757
  searching runs, 5,557 searches, 559 stories**. The pairing is by order; four
  searches sent in one second can swap answers inside the group (counts and
  shares are unaffected, per-search joins inside a burst are approximate);
- the paper guest's console files, which hold only **03:45:45Z to 05:22Z (1 h
  36 min)**: the per-run trigger and objective reads below are for that window
  (41 searching research runs, 236 searches);
- vmctl and the host journal around the two restarts;
- code at `origin/main` (file:line below).

Nothing was changed on Node B; query text was read in scratch and appears
below only as topic words.

## Answer in six lines

1. **The paper is not mostly re-researching stories.** 559 stories ran 757
   research runs in 33 h; **563 runs (74%) and 4,416 searches (79%) are the
   first searching run a story had in the window**; 194 runs (26%) and 1,141
   searches (21%) are a later run on a story researched before. A hold that
   acts only when a story already has research is bounded by that 21%
   (about 800 a day), and its measured floor is far lower (section 2).
2. **Every run in the console window was a Texture cast on a story at its
   first or second revision, and 34 of 41 objectives (213 of 236 searches)
   ask the desk to corroborate a claim from one captured item.** A story
   publishes only when its citations span two distinct outlets
   (`story publish: ... a story publishes only when its citations span at
   least 2 distinct outlets`); in the window 97 of 160 revision attempts were
   held for citing 0 or 1 outlets. Research is the second outlet.
3. **The corpus was never empty for the objective.** At the start of all 40
   delivered briefs the corpus held 6 to 33 captures on the objective (median
   20), the newest a median 26 minutes old. Whether they held a second outlet
   is not in the log.
4. **The searches are in the tail of the run, not the head.** 64% of 33 h
   sends were a run's 4th search or later (54% since the gate at 03:18Z);
   later searches return half the new-to-computer results of the first three
   (45% against 76%). The 8 runs with 8 or more searches sent 140 of 236
   (59%), and 47% of those were exact or near repeats of the run's own
   earlier wording.
5. **The two restarts were planned deploy refreshes**, and cost at most 6 of
   236 searches (section 4).
6. **Why exact repeats got past in-run reuse is not determined**; three
   documented exclusions explain 9 of 265, and 256 are unexplained (section 5).

## 1. What starts each research run (console window, 41 searching runs)

The chain in the console: new captures, then a management task (74 tasks, 239
captures in the window) and a story choice (`open` for a new story, `feed`
into an existing one), a Texture wake, a Texture revision turn that casts
research (`internal/textureprompts/texture.yaml:29`), a `research:<doc>` wake,
the corpus brief (`research_brief.go:739`, called at `runtime.go:4100`), the
run. A research desk is one per story: its inbox id is `research:<doc>`
(`deskmanifest/grants.go:579`), and the gateway's `conv=` is that doc id, so
every run joins to its story.

Other things that can start a research run exist in code and were looked for:

- a reader's "research more" (`internal/store/story_research_request.go`,
  objective "A reader asked for more research on this published story.");
- the entities backfill (same file, kind `entities`, "Search for no new
  sources");
- a source schedule.

The brief's first corpus query of each run is the cast's objective text. **None
of 41 carries the reader or the entities objective text** (0 matches in the
whole console), and no source-schedule run searched (earlier record). The
Cast's record kind is not logged, so "Texture cast" is by elimination for all
41, and directly seen (a Texture revision turn within a dispatch delay before
the research wake) for 20 of them (146 searches); the other 21 (90 searches)
have their Texture turn before the console begins.

| Class (41 runs, 236 searches, 03:46Z to 05:21Z) | Runs | Searches | Per run |
| --- | --- | --- | --- |
| Texture cast, first searching run the story has in 33 h | 38 | 227 | 6.0 |
| Texture cast, later run on a story researched before | 3 | 9 | 3.0 |
| reader "research more", entities backfill, source schedule | 0 | 0 | |
| Objective asks to corroborate or confirm a claim from a captured item, or the full report of it | 34 | 213 | 6.3 |
| Objective asks for details, coverage or an answer to a question (a new event, no claim to confirm) | 7 | 23 | 3.3 |
| An update on an event already published on | 3 | 9 | 3.0 |
| Background on a held topic | 0 identifiable | | |
| Runs of 1 to 3 searches | 25 | 52 | 2.1 |
| Runs of 4 to 7 | 8 | 44 | 5.5 |
| Runs of 8 or more | 8 | 140 | 17.5 |

The objective split is by wording of the brief's threads (corroborate,
confirm, verify, second independent outlet, find the full report); it is a
keyword read, not a judgement, and the 7 "other" are the runs that ask a
question without a claim to confirm. "Update on an event already published
on" is the repeat class above, by earlier searching run on the story, not by a
read of the objective. What starts the Texture turn: of the 20 casts with the
turn seen, 1 had a management `feed` since the previous Texture turn on that
story; the rest follow the story's first revisions after an `open`
(management story choice `story=-` carries no doc id, so a new story cannot be
tied to its `open` line by id).

Trigger over 33 h, which needs only the gateway:

| Run ordinal on its story (33 h) | Runs | Searches | Share of searches |
| --- | --- | --- | --- |
| 1st (no earlier searching run in the window) | 563 | 4,416 | 79% |
| 2nd and later | 194 | 1,141 | 21% |
| of which started under 15 min after the previous | 19 | 268 | |
| 15 to 60 min | 67 | 424 | |
| 1 to 6 h | 65 | 269 | |
| 6 to 24 h | 43 | 180 | |
| over 24 h | 0 | 0 | |

504 of 559 stories had one searching run in 33 h; 55 had more (up to 14). The
first-run share holds across the six 6-hour blocks (75% to 85% of searches in
each), so it is not an artifact of the journal's start. A story older than 33 h
is the one case that would lift the first-run count; the longest gap seen
is 24 h.

Runs a day at this rate: 544 searching runs and 402 stories a day. 1,000
searches a day is **1.8 searches a run, or 2.5 a story**.

## 2. Repeat runs and nothing new

The question: of the 194 runs (1,141 searches) on a story researched before, how
many had nothing new since the last run?

What cannot be measured from the journals: the arrival of a capture for the
story's event. The guest logs a story choice `feed` (captures fed to a story),
the event layer's flush counts, and a brief's `newest_held`, none of which names a
capture or its event. What follows is every reading the logs support.

- **Time since the last run** (table above): half the repeats start within an
  hour of the last run (86 runs, 692 searches), the fast ones are live events.
- **Fed captures** in the console window: the 3 repeat runs (9 searches) had no
  management `feed` to the story since the previous run, 0 of 3.
- **The corpus side of one repeat**: its brief's `newest_held` was one second
  after the previous run's last search. The newest capture the corpus held on
  its objective was the previous run's own read. **A test of "a capture newer
  than the last run" that counts every capture passes by construction**: the
  event layer's flushes in the window queued 750 captures, 537 of them research
  reads (`research_reads=`). The signal must be intake only.
- **The web side**: of a run's results, the gateway marks those the computer
  had received before (`seen=`). Repeat runs whose searches together returned
  under 25% new-to-computer results: **19 runs, 102 searches, 1.8% of 5,557
  (about 70 a day)**; under 25% new covers 5 of the 19 runs started within 15
  min of the last (62 searches) and 12 of the 67 within the hour (38 searches),
  2 of 65 at 1 to 6 h, 0 beyond. First runs: 3 runs, 99 searches in this band.
  New-to-computer results across repeat runs 50% (34% when started within 15
  min, 71% beyond 6 h); across first runs 59%.

**The number that sizes a hold on repeat runs: an upper bound of 21% of
searches (about 800 a day), a web-side floor of 1.8% (about 70 a day).** The
capture-side measure that would sit between them is not in the logs (section
"What I could not determine"). The earlier expectation "20 to 50% of runs
start with nothing new" is not supported as a share of searches: at most 26%
of runs are repeats at all.

The event layer cannot carry the test today: in the window it queued 750
captures and joined 27 to an existing event, minted 632 (3.6% joined), the
same minted-heavy shape the S7 acceptance (minted at most 60%) is meant to
fix. "No capture newer than the last run for this story's event" cannot be
answered when most captures mint an event of their own.

## 3. First runs with a new objective: what the heaviest runs did

Pre-gate (33 h, 698 runs starting before 03:18Z, 5,275 sends) and post-gate
(51 runs starting after 03:30Z, 265 sends) read the same way:

| Counterfactual, run on the logs after the fact | Pre-gate | Post-gate |
| --- | --- | --- |
| sends that were a run's 4th or later | 65% | 54% |
| sends of near-duplicate wording (word-set Jaccard 0.6 or more with an earlier send of the run) | 27% | 29% |
| sends after two searches in a row had returned 70% or more already-seen results | 32% | 24% |
| either of the last two | 43% | 38% |
| sends beyond the 2nd | 75% | 62% |

These are replays, not behaviour: they say where the volume sits. "Seen" is
the gateway's computer-wide count, wider than a run's own held marks.

Position in the run against what the gateway returned (33 h): first search
6% of results already seen; 2nd 34%; 3rd 37%; 4th 45%; 6th 47%; 10th and later
61%. Searches with 80% or more already seen: 3% of first searches, 6%, 7%,
12%, 14%, 16%, 22%, 22%, 23%, and 35% of 10th-and-later searches.

### The eight runs of 8 or more (140 of 236 searches, 59%)

| Run | Searches | Model calls | Minutes first to last | Exact repeats | Near repeats | Already seen | Objective |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 713e | 43 | 43 | 37 | 9 | 23 | 46% | corroborate a live missile attack from four angles |
| eb7e | 18 | 73 | 13 | 0 | 9 | 83% | a single social-media report of a pedestrian collision |
| c9e5 | 16 | 27 | 9 | 5 | 2 | 31% | quote from a leaders' summit, three experts |
| 81c3 | 15 | 22 | 16 | 0 | 4 | 52% | a stock index's intraday loss, level and cause |
| e6f0 | 15 | 52 | 42 | 1 | 8 | 45% | a two-country defence agreement's text |
| 8131 | 11 | 100 | 11 | 0 | 0 | 30% | a study behind a medical-news claim |
| 1711 | 11 | 37 | 4 | 0 | 2 | 31% | an arrest reported by one outlet |
| 2b57 | 11 | 29 | 2 | 0 | 3 | 34% | a training programme in a French shelter |

(Model calls are console `model call: run=` lines. The summary line
`runtime: search run=` is missing or partial for four of the eight, section 1
of the earlier record: 713e's line says `started=14` for 43 sends.)

What they did, turn by turn:

- **713e, 43 searches** (the earlier record's Kharkiv run). The brief carried
  four threads (a count of missiles, a strike on a city district, a strike on
  a town, the capital's bridges) and held 32 captures, the newest 43 minutes
  old. The model then ran **six rounds of the same four questions**, one
  `SearchAll` per round, each with the date ("20 November 2026") and
  outlet words ("site:" in 19 of 43). Every send returned 40 to 60 results, 46%
  already seen; every one of 43 carried the date word. The model made a
  model call between rounds and asked again. At 05:01 a planned
  restart interrupted it; it **resumed on a different model** (a small flash
  model replaced the free contributor model) and **re-sent the same four
  `site:` wordings in three consecutive turns 40 s and 30 s apart**
  (05:03:51, 05:04:30, 05:05:00; most of the second and third turn were gateway cache
  hits). After the restart the line shows `search_all_calls=14
  wordings_offered=80 wordings_sent=14 not_sent=66 (holds_unread=58
  stopped_yielding=8)`: **the gate refused 66 wordings and each call still sent
  its first**. Every refusal was per wording within a call; nothing refused the
  next turn's call.
- **eb7e, 18 searches in 6 minutes.** A story from one social-media report; the
  objective is whether police or a news outlet confirmed it. The model ran a
  loop of one search then four to six reads, 73 model calls: 18 rewordings of
  "pedestrian Nellis Stewart Las Vegas ..." (struck, hit, crash, collision,
  accident, tonight, Wednesday, Nov 19, a TV station name, the police
  department). **Seen-by-computer climbed 0, 18, 30, 34, 34, 29, 37, 32, 37, 39,
  38, 39, 39, 39, 40 of 40**: by the 4th search 85% of what came back had
  already come back, by the 12th it was all of it. The run read 46 pages (all
  fetched, none held) and found no confirming outlet; **the outcome was
  "nothing exists" and the run never concluded it**.
- **c9e5, 16 searches**: three wordings re-sent verbatim in five consecutive
  rounds (04:29:42, 04:30:55, 04:33:47, 04:34:57, 04:36:31 with small edits), 5
  exact repeats, 31% seen. Same shape as 713e at smaller scale.
- **81c3, e6f0**: an outlet-by-outlet walk (`site:` of four news and
  government sites, three wordings of the same event each), 15 searches over
  16 and 42 minutes.
- **8131, 1711, 2b57**: 11 searches each over 2 to 11 minutes, a source or
  name being hunted through variants (study title, author names, a street, a
  charity's name); 30 to 34% already seen, the least repetitive of the eight.

### Why the need test let each one through

The gate (`internal/yaegikernel/search_need.go`, `searchEnoughUnread = 15`,
`refuseForUnread` at `search_store.go:512`) refuses a search only when the run
holds 15 or more pages that were **new to the run and not yet tried**. What
let the eight through, from the code and the lines above:

1. **A tried page leaves the count.** The model reads three to six results
   after each search (eb7e: 46 reads in 6 minutes); each read, a dead link
   included, takes the page off `unreadPages`
   (`search_need.go:51`). The count therefore resets every turn.
2. **A returned page that was already returned is not new to the run**
   (`!h.Held`), so the count does not climb on repeats: the more a loop
   repeats, the less the gate sees. At 85% seen the gate sees 6 new pages of
   40.
3. **The yield stop is `SearchAll`'s only** (`stopped_yielding`,
   `search_all.go:267`); a model that uses the plain verb (eb7e,
   `search_all_calls=0`) has no yield test at all.
4. **A refusal is per wording, not per goal.** 713e's gate fired 58 times and
   the run sent 43.
5. **Nothing asks whether the objective is met.** The cast's need is a second
   outlet (the publication rule). The host holds the reads; no code compares
   them to the need.

What would have to be true for a run to stop at 3: (a) the verb refuses a
send that re-asks pages and terms the run already holds (rows 0 and 4 of the
one verb, for every verb, not SearchAll only); (b) the verb refuses after two
sends whose pages were mostly already returned, whatever the model has read;
(c) a run that has read pages from two distinct outlets on the objective is
told so and is not sent more for it; (d) a run whose three distinct wordings
returned nothing that confirms the claim reports "not found" instead of a
fourth. On the logs, (a) and (b) together remove 38% to 43% of sends; (c)
and (d) cannot be sized (no outlet per read in the logs).

## 4. The two restarts

Both were **planned deploy refreshes**, not crashes or memory:

| | Restart 1 | Restart 2 |
| --- | --- | --- |
| Host asked | 03:48:45Z `host requested a planned restart (platform update 39d2138d...; allow_busy=true, interrupting 11 running run(s))` | 04:59:46Z the same for `bd2ff1c2...`, `interrupting 8 running run(s)` |
| vmctl | `deploy refresh ... outcome=refreshed planned_restart=true` after seven minutes of `deferred` retries from 03:40Z | `outcome=refreshed planned_restart=true busy_allowed=true` at 05:00:18Z |
| Guest | `restart kind=planned ... marker_age_at_process_start=8s`; boot passivation of 26 candidates; `platform_update_resume` 03:50:11Z | `restart kind=planned`, 24 candidates, `platform_update_resume` 05:01:04Z |
| Research runs passivated | 5 running, 2 pending | 8 running, 5 pending |

Two deploys 71 minutes apart. Each is the marker-backed planned restart the
operating contract allows ("a planned restart may resume once"); both
interrupted busy runs because the paper is `public_platform`.

Sends that followed a restart, counting only runs that had already searched
before it (the only runs whose gate had something to forget): restart 1, none
(the five runs passivated had not sent yet); restart 2, **5 runs, 61 sends
before and 21 after; 2 of the 21 are exact repeats of a pre-restart send and 6
(29%) have a word-set overlap of 0.6 or more with one**. So at most **6 of
236 searches (2.6%)** followed from the gate forgetting, plus whatever the
gate would have refused for the 15-unread rule, which the logs cannot say.
The store is `rt.runSearch` in the host and `SearchStore` in the guest
(`search_store.go:221`), both empty after boot.

Two further costs of the restart show in the same runs: the summary line
counts only the resumed segment (713e: 43 searches, line says 14), and 713e
was resumed on a smaller model, which went on to re-send identical wordings.
Whether the model change is a rule (a fallback when the first provider is
cooling) or a drift was not read.

## 5. The 18 exact repeats in one run

Over 33 h: **265 of 5,557 sends (4.8%) repeated an earlier query of the same
run character for character**, in 105 of 757 runs (the top two runs, 20 and 15;
median gap 21 s, quartiles 9 s and 51 s). 223 came back from the gateway's
cache (`served=hit`), 41 reached a provider, 1 joined. In the console window
there are 16 (15 in the heavy runs; the earlier record's 18 is a slightly
wider window). 108 of the 265 are from 18 Nov 20Z to 19 Nov 00Z, before the
gate; 19 since 03:18Z.

`reuseSearch` (`search_marks.go:238`) should answer a repeat unless one of
these holds:

| Exclusion | Where | Repeats it explains |
| --- | --- | --- |
| gap beyond the window (10 minutes for news words, `searchquery.IsTimeSensitive`; 30 otherwise) | `search_marks.go:250` | 7 |
| the earlier search returned no page or no provider answered (`searchUsable`) | `search_marks.go:200` | 1 |
| a restart emptied the store | `search_store.go:221` | 1 |
| the call sent `fresh` | `startSearch` `override`, `search_store.go:495` | 0 in the heavy run (713e's 14 post-restart sends were all `SearchAll`, `fresh_over_unread=0`; `SearchAll` offers no `fresh`) |
| **none of the above** | | **256** |

The 256 had a prior identical search that returned results, a gap under ten
minutes, in one resumed or ordinary segment, from a `SearchAll` that cannot
pass `fresh`. So the fault is somewhere the logs do not reach. Candidates,
each with the test that would settle it:

1. the earlier search was not in the store when the later one was judged (the
   start frame sets `key`, `search_store.go:241`, and `knows` waits only 2 s,
   `search_store.go:535`): log "key matched but not usable / out of window /
   unknown" at the decline;
2. the same run's cells run in different desk sessions, each with its own
   store: log the store id with the run's search count;
3. the model re-asking inside one cell before the earlier arrival lands: a
   burst test, repeats whose gap is under 3 s.

Gaps under 3 s are 3 of the 265 repeats, so (3) is not the main cause.
Not determined.

## What I could not determine

- **Capture-side "nothing new"** for repeat runs: no capture id, event id or
  arrival time per story in any journal; the brief's `newest_held` and the
  event flush lines count but do not name. The measure needs `best_cosine`,
  `newest_strong`, and an intake-only "newest capture for this story's event"
  on the brief line and on the Cast, both read from corpusd, which the earlier
  record already asks for.
- **Which record kind** started each run for the 21 casts whose Texture turn
  is before the console starts, and for all 41 directly (the Cast's record
  kind is not logged).
- **Whether the corpus held a second outlet** on each objective: the console
  lines carry counts and hashed capture ids, not hosts. This is the number that
  decides how much a first run could be held; it is the one the design most
  needs.
- **Stories a day and the first-publish rate**: the console shows
  `published vN` for 42 distinct stories in 1 h 36 min (63 publications) and 97
  revisions not published for outlet count; the daily distinct-story count and
  the week before are not in any journal I read.
- **Why `reuseSearch` declined 256 exact repeats** (section 5).
- **Why 713e resumed on another model.**
- **A read of any run ledger.** Turn-commit reasons and tool calls were not
  reachable without a production database read, which was not made; the
  anatomy above is from the console and gateway lines, whose join by run id is
  exact for sends and approximate in burst order.
- Research runs that never searched are not in section 1's table (they have no gateway line).

## Method

Console window 03:45:45Z to 05:22:12Z; research runs are the gateway `run=`
ids with at least one send in 03:46Z to 05:21Z (41, plus 2 whose edge time
fell just outside), each joined to its brief, corpus-search and read lines by
run id and to its story by `conv=`. First and later runs are by order within
`conv=` in the 33-hour gateway journal; a story researched before 18 Nov 21Z
shows as a first run. Near-duplicate and "seen" figures use lower-cased word
sets and the gateway's `seen=`. Counts are exact for the lines read.
