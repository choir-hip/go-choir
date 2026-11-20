# Who sends the paper's web searches (2026-11-20)

Read-only investigation, 2026-11-20 05:13Z to 05:19Z (`date -u` at the end:
05:19:16Z). Sources: Node B's gateway journal (`go-choir-gateway`, lines
`gateway: search request from autoputer`, `gateway: search for autoputer`), the
paper guest's console (`console.log*`, 03:27:34Z to 05:14Z), the code at
`origin/main`. Same read paths as `scripts/status-hourly` and
`scripts/status-corpus-reuse`; query text was read in a scratch directory and is
not reproduced here beyond topic words. Nothing was changed on Node B.

The question: the search gate (`3f3e10ba1`, deployed 03:18Z) cut summary-line
searches per searching run from 7.9 to 3.4, yet the gateway still logs
187 to 200 an hour and the console window shows 264 searches against 183 in
summary lines.

## Answer in one paragraph

Every web search the gateway saw was sent by the research desk, through one
guest path (the desk cell's `choir.Search`/`SearchStart`/`SearchAll`, host
side `startSearch` in `internal/agentcore/desk_search.go`). No other desk,
brief, schedule or intake path sends one. The 81 are not a bypass of the
gate: they are searches the **summary line does not count**. The line is
written when a run's terminal record is written, from counts held in the
host's memory (`internal/agentcore/search_run_summary.go`), so it misses runs
still running, runs that go idle without a terminal write, and everything a
run did before a restart. The gate itself did what it was built to do (141
refusals), but the gateway's rate did not move (162 an hour from 12Z to 03Z
yesterday-to-today, 159 in the 04Z hour), because the gate is a small bite on
the number that matters: about 28 research runs an hour, each really sending
about 6 searches, half of them in a few long runs.

## 1. Who sends (03:27:00Z to 05:14:00Z, 1 h 47 min)

Gateway `search request` lines for the paper's computer: **265**.
Summary lines (`runtime: search run=`) in the same window: 54 runs,
`started=` sums to **183**. Difference 82 (the owner's 81 is the same
quantity on a window a few seconds different).

| Caller (gateway `desk=`, guest path) | Searches | Share |
| --- | --- | --- |
| research desk, `choir.Search*` / `SearchAll` in a desk cell | 265 | 100% |
| any other desk (management, texture, engineering, brief, source schedule, intake) | 0 | 0% |

Whole record behind it: 5,245 `search request` lines for every computer from
2026-11-19 00:00Z to 05:17Z today, `desk=research` on every one; the paper's
computer sent 4,924 of them. The corpus brief (`research_brief.go`) and
every corpus search send nothing to the gateway (its `caller=brief` /
`caller=cell` lines are corpusd calls: 179 and 299 in the window). Texture's
`research_texture` child egress (`research_children.go:534`) and
`runResearchChildren` produced no line: no `research child:` line appears in
the console. So **question 1's breakdown is a single row**; the interesting
split is by run, below.

Summary-line verbs in the window (all 54 lines): 183 sent; 79 through
`SearchAll` (one wording each time: `search_all_calls=79 wordings_sent=79`,
213 wordings offered); 162 refused (141 `holds_unread`, 12 `search_running`,
9 `stopped_yielding`); 27 sent over the gate with `fresh` (15% of sends);
1 answered from held results; pages returned 8,186, new to the run 7,627,
read 700 (8.6%).

### The 82-search gap, by run

| Cause | Runs | Searches | Where |
| --- | --- | --- | --- |
| Run had not ended when the console was read (05:13Z): no terminal write, no line | `713e557f` (43), `e6f0936e` (14), `1711d692` (11) | 68 | `runtime.go:769` logs the line only from `SetRunTerminalObserver` |
| Run went idle without a terminal record ("remains idle: Research desk has no unread records", 03:50:51Z) | `8fde37cd` | 3 | same |
| Counts held in memory lost to a restart: the search counts live in `rt.runSearch` (`runtime.go:143`), a `sync.Map` in the host process; the host booted at 03:49:25Z and 05:00:27Z and passivated the running runs (`runtime: passivated run ... after restart`), the resumed run counts from zero | `eb7e8b2d` 16 (gateway 18, line `started=2`), `e08f2a95` 2, `68b893f5` 1, `ac46aa8d` 1, `9692ecf1` 1 | 21 | every one of these searches is dated before the boot that preceded its run's summary (04:54 to 04:59Z before 05:00:58Z; 03:39Z before 03:49Z) |
| Summary lines for searches sent before the gateway window began at 03:27:00Z | `79d4d56b` (7), `d1ef7459` (1) | -8 | window edge |
| A run whose summary counted more than the gateway logged | `ad7fd5c6`, `a7e3d373` | -2 | not determined (one search each; candidates: a start that failed before the gateway logged it) |

71 + 21 - 8 - 2 = 82. A second, quieter loss: `takeSearchStats`
(`yaegikernel/search_marks.go:140`) is read when a cell ends
(`tools_desk.go:931`), so a cell still in flight adds nothing until it ends.

Consequence for the owner's gauge. The 7.9 to 3.4 comparison is made from
lines that exist only for runs that ended. Long runs are the ones likeliest to
be unfinished or to have crossed a restart, so the lines lose exactly the
heaviest runs. The gateway's own count per run (below) says about 6, not 3.4.
The gateway line, which carries `run=`, is the reliable meter. What I did not
do: re-count the pre-gate 7.9 the same way, because the console does not keep
the pre-gate day.

## 2. Runs an hour, what starts them, and what they search for

Gateway lines per hour for the paper, with distinct run ids in the hour:

| UTC hour | searches | runs | searches/run |
| --- | --- | --- | --- |
| 2026-11-19 22Z | 260 | 34 | 7.6 |
| 23Z | 282 | 35 | 8.1 |
| 2026-11-20 00Z | 124 | 25 | 5.0 |
| 01Z | 180 | 27 | 6.7 |
| 02Z | 170 | 34 | 5.0 |
| 03Z (gate at 03:18) | 207 | 29 | 7.1 |
| 04Z | 159 | 26 | 6.1 |
| 05Z (17 min) | 52 | 12 | 4.3 |

From 12Z yesterday to 03Z (before the gate) 162 an hour and about 25 runs an
hour; the 04Z hour 159 and 26. The gate moved the rate by nothing visible.
Today so far the gateway counter shows `keenable=868` at 05:17Z and 24 cache
hits (all other paid providers answer `quota_limited`; searxng
`success_empty`; chatgpt `auth_error`). Yesterday: 4,224 sent plus 128 hits
plus 1 joined for all computers.

In the window: 59 distinct run ids searched (two hours, 03:14 to 05:14Z),
**about 30 runs an hour**, mean 5.5 searches a run, median 3. The spread is
the story: of 265 searches in 03:27 to 05:14Z, 8 runs with 8 or more
searches sent 139 (52%); the top run alone sent 43, the top five 106 (40%).
34 runs of 1 to 3 searches sent 70 (26%). So the hourly total is many runs times
a few searches, plus a short tail of live-story runs times many.

What starts them. The console shows the chain in these lines: new captures
make a `management task` (73 `started management task ... (N captures)` in
the window, 14 + 47 + 12 an hour); the management run takes a `management
story choice` (70 `open`, 67 `feed`); Texture wakes on the story
(`texture wake`, `actor wake latency ... target=texture:`) and its prompt
sends `choir.Cast("research", objective, packet, req)`
(`internal/textureprompts/texture.yaml:29`), which wakes a research agent
(`actor wake latency kind=initial_dispatch target=research:` 25 and
`kind=inbox_record target=research:` 22 in the window). The run's first
turn then carries the corpus brief (`research brief run=` 52 lines: 39
`delivered`, 13 `skipped`). What I could not establish from the logs: which
record kind (a new story's v1, a revision, a story revision after a
capture joined it) started each of the 52 runs; the console does not name it.
A schedule-origin check was not needed: no source-schedule run searched.

What heavy runs are. Live-event stories searched again and again with date
words (a Kharkiv strike, a Las Vegas collision: 43 and 18 searches), several
`site:` queries per outlet. In the Las Vegas run, 18 searches in 13 minutes,
almost all rewordings of one event.

### Were the searches for things the corpus already held?

The console's `corpus search` line carries a result count (`captures=`,
`published=`, `vector=`) but **not the cosine or the age** of the captures it
returned. So "strong and recent" cannot be read from the traces; what can be
read is below, each as a bound, not as a finding that those searches were
unnecessary.

- Of the 262 searches sent after 03:28Z, 259 (99%) had a corpus search in the
  same run before them (cell or brief); 48 of 51 runs had one before the
  first send. The run is not skipping the corpus.
- 174 (66%) of those had an earlier corpus search whose query shared at least
  30% of its words with the sent query (Jaccard); every one of the 174 had
  captures returned (153 with 10 or more, the list's usual cap). 89 (34%) shared
  half or more of the words. These are upper bounds: "the corpus returned
  ten captures on nearly the same words". Whether they were strong or recent
  is not in the log.
- The gateway's own `seen=` field (results the computer already received in
  an earlier search) on the 311 searches that missed the cache: 34.8% of all
  returned results had been seen before; median search 30%; 37% of searches
  had half or more already seen, 16% had 80% or more, 25% had none, 0.6% had
  all.
- Wording variants inside one run: 96 of 327 sends (29%, 03:14 to 05:14Z) had
  a word-set Jaccard of 0.6 or more with an earlier send of the same run.
  Across different runs inside 30 minutes: 0. So the near-duplication is
  within a run, not between runs.
- Pages: 8,186 returned, 700 read (8.6%).

A reading of these together: the lower bound on "not new information" is the
35% already seen plus the same-run near-duplicates; the upper bound is the
66% where the corpus answered the same words. I would put the plausible
not-new share of sends at 35% to 65%, centre about half. A 40-search random sample was
read by eye (seed 1, scratch only); I did not tally it, and the 262-search
figures above are the ones to cite. The logs cannot settle strength.

## 3. The arithmetic to under 1,000 a day (42 an hour)

Base: 160 to 200 an hour measured by the gateway, 25 to 30 runs an hour at
5 to 7 each. 1,000 a day means a 75 to 80% cut, or about 1.5 searches per
run at today's run rate. Estimates below are searches a day removed from a
base of about 4,000; each is a hypothesis with the uncertainty shown.

| Change | Effect on sends | Searches/day | Basis and uncertainty |
| --- | --- | --- | --- |
| Fix 4 (a run that starts with no objective gets one) | none in steady state | 0 to 400, likely under 100 | all 13 `skipped` briefs fall between 03:29 and 03:38Z, a backlog burst after the 03:18 deploy (the dispatcher logs `waited=15m`); 0 of 39 later briefs were skipped. Those 7 searching runs sent 28 searches (10.6% of the window) at 4 a run, no heavier than others |
| L1 leads table (durable, per computer) | none by itself; it is the memory L3's rows 4 needs | 0 alone | removes the restart and cross-run forgetting: the gate's unread list lives in the guest's `SearchStore` and restarts empty after a boot (two restarts in this 2-hour window passivated the long runs) |
| L2 `Sufficiency` (corpus says how strong and recent its best answer is) | none by itself; it makes rows 3 and 5 decidable | 0 alone | also gives the missing data of section 2: the log needs the best cosine and age per search before any threshold can be set |
| L3 one verb `choir.Search` deciding held / sent / sent-with-since | the real lever | 1,000 to 2,200 | row 3 (unread strong capture holds) bites where the corpus answered well; rows 0 and 4 bite on repeats and wording variants. Bound by section 2: 35% already seen, 29% same-run variants (overlapping), up to 66% corpus-answered; I take 25 to 55% held. Row 4's K = 15 is the gate's own threshold, so rows 0 and 4 add only cross-run and restart memory to what the gate does now |
| L4 `since` on sent searches | none on the count | 0 (reduces results per search, not searches) | row 5 still sends |
| Gate in-run (shipped, 03:18Z) | already in the base | 0 more | 141 refusals; 15% of sends were `fresh` overrides (27 of 183) |

Sum of everything listed: 1,000 to 2,300 of 4,000 removed, leaving 1,700 to
3,000 a day. **The listed changes do not reach 1,000.** Even the optimistic
end (55% held) leaves 1,800.

What nobody has listed, and what the traces point at:

1. **Hold the research run, not only the search** (largest). The run count is
   the multiplier, 25 to 30 an hour. A story already researched whose event
   has no capture newer than its last research run needs no research run at
   all: the new-information test applied at `choir.Cast("research", ...)`
   (host side, using the event layer's event id and the newest capture time
   it already holds). Not a quota or a rate: it is the owner's sentence ("only
   search for what is new to us") evaluated one step earlier. Estimated
   effect: unknown; the traces do not say how many casts follow a story with
   no new capture. Needs a measurement first: for each research run, newest
   capture joined to the story's event before the run vs the previous run's
   start. I would expect 20 to 50% of runs, so 800 to 2,000 a day, but this is
   a guess, not a reading.
2. **The live-story tail**: 8 runs sent 52% of the searches. Each re-asks
   the web for the same ongoing event with new date words. An event-level
   hold (the event layer already joins captures to events; "this event was
   searched 10 minutes ago and nothing new has been captured since") would
   fold the variants L3's term-key row cannot (different words, same event).
   Estimate 500 to 1,000 a day, overlapping with 1 and with L3.
3. **Make the gauge true before judging the gate**: write the search count
   from the gateway line (it carries `run=`), or write the run summary on
   every cell end and survive a restart (state into the run's record, not
   `rt.runSearch`). Without it the owner's 7.9 and 3.4 are not comparable.
   This changes no search count.
4. **Resumed runs start with an empty `SearchStore`**, so the in-run gate and
   the same-terms reuse know nothing of the pre-restart pages; L1 covers it
   only if the lead is written on arrival. Not measured: how many sends in the
   resumed segments would have been refused. Two restarts in two hours, so the
   effect is not small in a deploy-heavy day.

Rough combined picture: with 1 (cut runs 20 to 50%) and L3 (25 to 55% held on
what remains) the base falls to 900 to 2,400. Only the upper half of those
ranges is under 1,000 a day. Treat 1,000 as reachable only if the cast-level
hold works; I would not claim it from L1 to L4 alone.

## 4. Defects seen

- **Same-run exact repeats**: 18 of 327 sends (5.5%) repeated an earlier
  query of the same run character for character, 12 to 690 seconds apart; 16
  of the 18 came back from the gateway's cache, which is the 16 `hit`s of the
  2 hours. Only one of the 18 straddled a restart. The in-run reuse
  (`search_marks.go:238`, window 30 min, 10 for news words) should have
  answered all of them; why it did not is **not determined** (candidates: the
  model sent `fresh`; the earlier search was `searchUsable`-false because it
  returned nothing or one provider failed; the cell ran in a different desk
  session with its own store). Bursts of four in one second at 05:05:00Z
  (run `713e557f`) look like one cell re-sending a list. The same-run
  exact repeats cost one provider call each only when the cache missed (2).
- **Why only 16 cache hits**: `cacheKey` is scope + query up to whitespace +
  max results (`searchplane/cache.go:124`); case, `site:`, a date word or a
  reordering is a different key. Exact repeats are the only hits, and
  exact repeats are 5.5% of sends; across runs there were none (0 queries in
  more than one run). The cache cannot be the fix: the volume is wording
  variants (29%), not repeats.
- **Searches by a run that never reads the results**: 8 of 52 fresh briefs had
  no gateway search at all; of searching runs, 700 pages read of 8,186
  returned (8.6%). A run that sends 43 searches reads at most what it can; the
  gate's refusal number (141) says it was holding 15 or more unread pages 141
  times and sent 27 more with `fresh` anyway.
- **Retries after provider refusal**: none seen as new searches. 0 `search
  failed` lines in 4 hours; 311 of 327 were cache misses that each reached
  one provider (keenable, 234 `1 of 1`, 88 `2 of 2`), the paid providers
  answered `quota_limited` and are skipped inside one request, not retried
  as requests. `seen=` and per-request counts do not double count.
- **Provider state**: tavily, brave, parallel, serper `quota_limited`,
  searxng `success_empty`, chatgpt `auth_error`: every search is
  effectively one provider (keenable, `today=[keenable=868]`).
- **The summary line is a defect as a meter** (section 1): it lacks in-flight,
  idle and pre-restart searches. Fix is to count at the gateway or to persist
  the totals.

## What I could not determine

- The strength and age of the captures a corpus search returned: the guest
  line has counts only. Needs `best_cosine` and `newest_strong` in the line
  (design L2) before any "was it new" statement beyond the bounds above.
- Which record started each research run (new story, revision, capture join).
- Whether the 18 same-run exact repeats used `fresh` (the argument is not
  logged except when 15 or more unread pages were held).
- The 2 runs whose summary exceeded the gateway count.
- The pre-gate summary figures (7.9 per run): the console keeps about 1.75
  hours, so I could not recompute it on the gateway's basis for the
  same period; the gateway rate before and after (162 an hour vs 159) is
  the comparison I can make.
- Whether the 03:49Z and 05:00Z restarts were deploys (assumed planned, as
  `passivated ... after restart` and a resume suggest; the cause was not
  read).
- Cross-run duplicates beyond 30 minutes were not examined.

## Method, for the next reader

Window 2026-11-20 03:27:00Z to 05:14:00Z for per-run joins (console start),
03:14Z to 05:14Z for the two-hour gateway counts. Gateway: `journalctl -u
go-choir-gateway --since ... -o cat --utc` filtered to the paper's computer
id; console: `sudo -n cat console.log*` under the paper VM state directory
over `ssh node-b`. Run ids are joined between the two sources. Counts are
exact for the gateway and console lines read; "similar" queries use a word-set
overlap of the lower-cased words and are a proxy.
