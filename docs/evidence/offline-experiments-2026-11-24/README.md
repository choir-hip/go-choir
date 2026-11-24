# Offline experiments E1 to E3 (2026-11-24)

Three read-only replays over what the paper holds, for the measurement track
(`docs/definitions/choir-records-and-measurement-2026-11-23.md`). Ledger claims
C215 to C219, each entered with its rules and kill lines and pushed before it
was run (`lane/offline-experiments`, commits `072516624` and `af6bafb2e`).
No model call was made, no spend, nothing was written on Node B. The reader of
every hand read is the investigator, one Claude agent (the ledger rows say
"the lead") that also wrote the grouping: these are leads, not independent
judgments.

## Verdicts

| Claim | Pre-registered line | Result | Settled |
|---|---|---|---|
| C215 E1, the measure `D` over groups of arrivals sees the Huang failure at 19:00Z | Huang group in the world's top 3; covering story at front rank 7 to 13; its term in the top 3 terms | top 3: **yes (first)**; front rank **4**; term rank **8** | **false** |
| C216 E2, happenings grow more slowly than sources (exponent under 0.85) | point under 0.85, upper end under 1.0 | exponent **1.31** [1.25, 1.37] | **false** |
| C217 E3, vector groups are happenings | ONE at least 70%, MERGE at most 15%, SPLIT at most 25% | ONE **68%**, MERGE **16%**, SPLIT **15%** | **false** (narrowly, two of three lines) |
| C218 E1, second form (subject importance times sparsity, no groups) | a Huang-pattern arrival in the top 3 of the hour before 19:00Z | best rank **14 of 170** | **false** |
| C219 E3 addendum, a group is about one subject | S1 at least 60% of 50 groups | **90%** | **true** |

## Data and rules (all fixed before any run)

- **Export** (`scripts/offline-experiments/export`, SQL in the same folder):
  the paper computer's source arrivals fetched 2026-11-22T00:00Z to
  2026-11-24T00:00Z, 6,745 captures, research's own web reads excluded
  (`source_id NOT LIKE 'web:read%'`), not tombstoned, not superseded; their
  vectors (`vec_captures`, 256-d), URLs and the first 8 KiB of each body; the
  published versions of Autopaper stories (`state published`, `publish_mode
  auto`) from 2026-11-21T12:00Z with their `published_at`; the
  `cites_capture` edges from versions to those arrivals; the passage vectors
  of those versions. Each statement ran under `nice -n 19`, read-only, with a
  120 s timeout; the body read under `nice` and `ionice -c3`. The export took
  under a minute. Files stayed on the build VM (`choir-agents`).
- **Copy check:** a capture is dropped if an earlier one has its canonical key
  or text hash: 477 of 6,745, leaving **6,268 arrivals**. Time is `fetched_at`.
- **Grouping (the lead's "happening"):** arrival order; an arrival joins the
  open group of highest centroid cosine if that is at least **0.84**, else
  opens a group; a group takes no member after 48 h quiet or 48 h of age; no
  size cap. The threshold is the one rung 2 of the offline slice took from
  days 1 to 3 of a different week; it was not tuned on this data. Result:
  2,548 groups; 1,983 singletons; 418 of 2 to 4 members; 147 of 5 or more;
  the largest 859 (a French-language aggregator feed).
- **Origin:** the wire service for a wire-tagged lead, else the outlet of the
  URL (`internal/outlet`), else the outlet of the first article link in a post,
  else the account; a post opening with a hashtag naming an outlet is that
  outlet. Code copied from rung 2 (`lane/offline-slice-r0`). One origin counts
  once per happening, weight 1 (the owner's ruling).
- **Front at t:** the 60 Autopaper stories with the latest published-version
  time at or before `t` (the order of `/api/platform/publications/front`);
  rank `r` has mass `1/log2(r+1)`, normalised.
- **Covers:** a story covers a group if a version published by `t` cites a
  member (edge created by `t`) or its latest version's best passage has cosine
  0.795 or more to the group centroid as of `t`; the story's mass goes to the
  group it covers most; `q` is the mass assigned, or 0.001 when none.

Rerun: `scripts/offline-experiments/export`, `sync-runner`, then
`run e1 | e1b | e2 | e3-sample | e3-score` (Go program in the same folder;
needs only `internal/outlet`, `internal/entitytext` and what they import).
Outputs here: `e1-results.txt`, `e1b-results.txt`, `e2-results.txt`,
`e3-sample.md`, `e3-key.json`, `e3-labels.tsv`, `e3-score.txt`.

## E1, does the measure see the failure (C215)

`D(t) = sum over groups h of p_h log2(p_h / q_h)`, `p` the share of distinct
origins in the trailing 3 hours (windows of 1, 6 and 24 hours as sensitivity),
evaluated at each hour of 2026-11-23 and, for the Huang group, every five
minutes.

**At 19:00Z (the pre-registered instant):**

- The Huang group `h*` (69 arrivals before 19:00 match huang, nvidia and
  reinstat or return or back as or restor; 106 members) holds **21 of 421**
  origin-slots in the window (p = 0.050). It is **first** of 340 groups by
  world attention, and was first at every five-minute step from about 14:30 to
  23:55 (116 of 116). Clause (a) holds, in all four windows but the 24 h one,
  where it is second.
- The two stories that cover it were at replayed front ranks **4** and 41. So
  `q` = 0.042, almost equal to `p`: its term is 0.012 bits, **eighth** of the
  hour. Clauses (b) and (c) fail. By the rule written first, the measure did
  not see the failure at 19:00Z.
- `D` for the hour is 0.87 bits: the "Missing" part (uncovered groups, almost
  all of them one-origin posts held at the floor 0.001) is 1.16 and the signed
  "Misranked" part is -0.29. Groups with two or more origins carry 0.26 bits.
  The number is dominated by the long tail of singletons and by the choice of
  the floor; it is a poor headline.

**What the replay shows beyond the pre-registered instant** (read after the
outcome, so a lead and not a settlement; trace in `e1-results.txt`):

- The replayed front is a ticker. Order is by last publish time and the paper
  publishes a revision about every 20 to 30 seconds, so a story is in the top
  ten for a few minutes after a revision and then falls. The Huang story's
  best covering rank was 9, 18, 2, 13 at 15:00, 16:00, 17:00 and 18:00, 4 at
  19:00, then 8, 11, 8, 14, 17 at 19:05, 19:10, 19:15, 19:20, 19:25. At 19:15
  to 19:25 the replay reproduces "about tenth", and there the Huang term is
  rank 4, 1 and 1. Over the 116 five-minute steps its covering story was in
  the top ten for 56 and its term in the top three for 66.
- Mean `p` over those steps is 0.061 and mean `q` 0.036: on average the front
  gave the happening about 58% of the attention the world gave it. That is a
  mild, not a dramatic, gap.
- The reinstatement was reported from **14:11Z** (The Verge, the Guardian, El
  Pais, Tagesschau, The Hill, Nvidia's own release by 14:19Z), five hours
  before the owner looked. The problem record's "18:19Z or before" was the
  second story's time, not the first report.
- The page the owner saw is not the front the replay can build. The web page
  and the phone multiply importance by depth and a 24-hour decay, and only
  the latest importance score per story is held (`scores.json`), not its
  history. The importance-ordered front at a past time cannot be recomputed
  from what the paper holds. That, and not the arithmetic of `D`, is why the
  first form cannot show a misranking made by importance 3.

### E1, second form: subject importance times sparsity (C218)

Owner, 2026-11-24: no word "happening"; importance belongs to subjects,
novelty to sparsity, and they multiply. Rules (entered before the run):
subjects are the names `entitytext.Propose` finds in title and lead (at most
12; a key in more than 2% of arrivals so far is dropped as furniture);
`R_s` the distinct origins on a subject in the 3 hours up to the arrival;
`I = sum over subjects of log2(1 + R_s)`; `N = 1 - cosine to the 10th nearest
earlier arrival` (the corpus here is the two days of source arrivals only);
score `I x N`. 170 arrivals fall in (18:00, 19:00]; 12 are Huang-pattern.

| Ranking of the 170 | Best Huang-pattern arrival |
|---|---|
| importance `I` alone | **rank 2** (top 1.2%) |
| novelty `N` alone | rank 132 (top 78%) |
| score `I x N` (the claim) | **rank 14** (top 8.2%); none in the top 3 |

The hour after (19:00 to 20:00): ranks 2, 134 and 20. The hour 15:00 to
16:00: ranks 1, 86 and 40. The top of the product is a UPI sports item, a film
review, a Steam award notice: sparse, with moderately counted subjects.
Novelty ranges only 0.1 to 0.35 across the day because the corpus is source
arrivals only, and a follow-up (every Huang report is near 100 others) sits at
the bottom of it, exactly the "dense, not novel, important" case of the problem
record. The subject count alone is right: at 19:00 the top subject by distinct
origins is `jensen huang` (18), then `nvidia` (13), `nvidia ceo` (8). It is
also noisy: `article url`, `comments url`, `ceo` and `chinese` are in the top
ten.

**Which form sees the failure.** Neither, as written. Both see the *happening*
(group first of 340; subject `jensen huang` first; arrival second of 170 by
importance). Neither sees the *misranking*, the first because the front cannot
be rebuilt as the reader saw it, the second because multiplying by sparsity
buries a follow-up. If novelty is read as the owner's "novelty squared with
importance" in a form that does not punish a dense follow-up (for example
novelty of the subject's recent rate against its own history, not the
arrival's distance to its neighbours), the second form is untested.

**Do groups add anything the subject form lacks?** Two things, both from the
19:00 snapshot, by the lead's reading:

1. Resolution inside a common subject. The group second by world attention is
   "Russia launches a mass missile attack against Ukraine" (10 origins, 74
   members). In the subject form `russia`, `ukraine` and `trump` were dropped
   as furniture (each over 2% of arrivals) and the attack has no subject of
   its own. Groups see events; subjects see names.
2. Counting. Groups collapse a 3-minute cluster of reposts of one article to
   one origin on one thing; subjects count each arrival's names separately.

And what subjects add that groups lack: nothing for the Huang case (both
find it first), and subjects join a group's split halves (the 14:11Z reports
and the 03:09Z rumour posts are one group here, which is itself a defect: the
group does not separate "Trump pushes" from "reinstated"). Of 13 groups with
three or more origins in the window, 4 have one subject key in 60% of members
and 13 in 30%.

### What E1 means for the design

- The measure's world half works: `p` finds the day's biggest happening first
  and early, with no model. Its front half has no source of truth: the paper
  does not store the page a reader saw. R0/R3 must record the ranked list
  served (or its inputs, importance history included) before `q` is anything
  but a guess. The owner's "we can't lose logs" applies to the page.
- A headline `D` is unreadable as it stands: its Missing part (1.16 bits)
  exceeds the whole, and is the floor on one-origin posts. Report `D` over groups of two or more origins, and the top
  terms, not the sum.
- The failure of the front's ranking is a ranking fact (a ticker with a 3-of-10
  importance), not a coverage fact. The measure needs the importance-ordered
  front to show it.

## E2, do happenings grow more slowly than sources (C216)

Unit dropped: an independent origin (485 in the two days), at random, keeping
fractions 0.10, 0.25, 0.50, 0.75, 1.00 (100 draws each below 1, seed
20261124); regroup after every draw; exponent `b` of `log mean N` on `log f`;
95% bootstrap interval of draws (1,000 resamples).

| What is counted | f = 0.10 | 0.25 | 0.50 | 0.75 | 1.00 | exponent b [95%] |
|---|---|---|---|---|---|---|
| groups with 2 or more origins (the claim) | 16 | 58 | 142 | 233 | 330 | **1.31** [1.25, 1.37] |
| groups of any size | 339 | 770 | 1,424 | 2,018 | 2,548 | 0.88 [0.85, 0.91] |
| groups with 3 or more origins | 4.7 | 21.8 | 60.8 | 101.8 | 148 | 1.50 [1.43, 1.56] |
| arrivals kept (check) | 649 | 1,582 | 3,217 | 4,717 | 6,268 | 0.99 [0.94, 1.04] |

With the unit a feed (`source_id`, 58 of them) the exponents are 1.21, 0.92
and 1.38. **There is no scaling law of the kind claimed: the count of
multi-origin happenings grows faster than the number of origins.** It must:
two origins must both survive a drop to report a happening together, so the
count scales between `f` and `f^2`. The only sublinear count is the one that
includes singletons (0.88, and 0.84 between f = 0.5 and 1), which is a count
of arrivals more than of happenings.

For the design: "happenings grow more slowly than sources" cannot be used to
argue that coverage saturates. What the numbers do say is the opposite and
useful: **the happenings that can be corroborated grow faster than the
number of origins** (three or more origins: exponent 1.5), so breadth of origins buys the corroboration the
importance state counts, and a cut of the source list loses it
disproportionately. A saturation claim would have to be made on the count of
happenings of any kind with a corroboration floor fixed per happening, which
this experiment did not test.

## E3, are vector groups happenings (C217, C219)

Groups of the 3,529 arrivals of 2026-11-23 alone (1,693 groups: 1,364
singletons, 329 of two or more; the largest 336). Sample (seed 20261124): 50
groups drawn uniformly from the 329, 40 singletons drawn uniformly from the
1,364. A group was shown with up to 12 members at random; a singleton with its
three nearest other arrivals within 48 hours (cosines hidden). Reader: the
lead, after the grouping was built, labels in `e3-labels.tsv`.

| Groups of two or more (n = 50) | n | share (Wilson 95%) |
|---|---|---|
| ONE (every member reports one happening) | 34 | 68% (54 to 79) |
| MOSTLY one | 2 | 4% (1 to 13) |
| MERGE (two or more happenings, or a topic) | 8 | 16% (8 to 29) |
| NOISE (no happening: reposted promotions, OSINT and CVE rolls, recipes) | 6 | 12% (6 to 24) |
| ... of the 23 with two or more origins: ONE / MOSTLY / MERGE / NOISE | 18 / 2 / 2 / 1 | 78% / 9% / 9% / 4% |
| ... of the 27 with one origin: ONE / MOSTLY / MERGE / NOISE | 16 / 0 / 6 / 5 | 59% / 0% / 22% / 19% |

| Singletons (n = 40) | n | share |
|---|---|---|
| ALONE (none of the 3 nearest reports its happening) | 34 | 85% (71 to 93) |
| SPLIT (a near arrival reports it, so a group should have taken it) | 6 | 15% (7 to 29) |

Of the 6 splits, 3 are borderline (a legislative bill's other coverage, a
weather system's forecast, a day's Gaza strike reports). The pre-registered
lines were ONE at least 70% (68%), MERGE at most 15% (16%) and SPLIT at most
25% (15%): two of three miss, narrowly, so C217 is settled false. ONE plus
MOSTLY is 72%. The groups that matter for attention counts, those with two or
more independent origins, are one happening for 78% (interval 58 to 90) and
merge for 9%; the one-origin groups are where most of the defects sit (a
single outlet's own feed and RSS reposts, aggregator rolls, a county news
wire). A group of two or more origins is the better thing to count; the
design's `p` should count origins, which already collapses most of the
one-origin groups.

**Group against subject (C219).** For 45 of 50 groups (90%, interval 79 to
96) the reader found exactly one main subject; 5 had several (an aggregator
feed of different Canadian crimes, a roll of CVEs, two Faroese items, two OSINT
rolls); none had none. No group was one happening spanning several subjects
(ONE with several subjects: 0). Eleven of the 45 one-subject groups were not
one happening (6 merge, 2 mostly, 3 noise by the cross-tab): a subject
(Lebanon, the Russia-Ukraine war, a Korean county) spans several happenings in
which the group did not split. The mechanical check (an extractor key in 60%
or more of the members) holds for 32 of 50 groups, 46 of 50 at 30%. Reading,
the lead's: in this sample a vector group is almost always inside one subject
and is not a larger thing; what it adds over a subject is resolving a common
subject into its events when the events are tight enough, and what it lacks
is any ability to span subjects. The sample does not contain the case that
would show a group beyond a subject (a happening reported with different
names in different languages), though such a case would show up as SPLIT, and
there were 6.

## Limits

- **Source mix.** Four fifths of last week's arrivals were social posts; this
  day is the same (mastodon 3 feeds, bsky, aggregator relays). `p` is
  attention among our panel, not the world. Origins collapse a repost to an
  account or an outlet only when the link or a leading hashtag names it.
- **The grouping is ours.** 0.84 was fixed from another week; no cap on size
  (the offline slice capped groups at 25) lets chaining show: the 859-member
  French group, and a Huang group that joins the 03:09Z rumour to the 14:11Z
  report. A group of 336 on 11-23 is not a happening.
- **The front.** Replayed from publish times only (a recency ticker). The
  importance-ordered page the owner saw cannot be rebuilt; the first form's
  `q` is therefore the replayed API order and not the page. Stories are
  assigned to one group by citation or passage cosine; a story that covers two
  groups feeds one.
- **One day, one happening.** E1 tests the instant named; the trace past it
  was read after the outcome. E2 and E3 are two days (E3 one) of one source
  mix. 11-22 is warm-up for E1, and for the sparsity form there are only 24
  hours of earlier arrivals at midnight.
- **Hand reads** are by one reader who built the grouping, 50 groups and 40
  singletons; labels were written after reading the samples and before the
  counts; MERGE and SPLIT lines are judgment calls (several borderline).
- **Subjects** are the deployed extractor's names, not entity links (captures
  carry none); furniture is a 2% rule written without looking at what it
  drops (it dropped `russia`, `ukraine`, `trump`).
- **Sparsity** is measured against source arrivals only, not the 250,000
  captures the paper holds; with the full corpus novelty would span a wider
  range.

## Ledger settlements

Brier is `(p - outcome)^2`.

| Id | p | Outcome | Evidence | Brier |
|---|---|---|---|---|
| C215 | 0.40 | false | at 19:00Z the Huang group is first by `p` (clause a true), its covering story is at replayed front rank 4 (clause b needs 7 to 13) and its term is eighth (clause c needs the top 3); `e1-results.txt` | 0.160 |
| C216 | 0.15 | false | exponent 1.31, interval [1.25, 1.37], for groups of two or more origins; `e2-results.txt` | 0.023 |
| C217 | 0.45 | false | ONE 34 of 50 = 68% (needs 70%), MERGE 8 of 50 = 16% (needs 15% or less), SPLIT 6 of 40 = 15% (met); `e3-score.txt` | 0.203 |
| C218 | 0.25 | false | best Huang-pattern arrival ranks 14 of 170 by `I x N`, none in the top 3; rank 2 by `I` alone; `e1b-results.txt` | 0.063 |
| C219 | 0.55 | true | exactly one main subject for 45 of 50 groups (90%); `e3-score.txt` | 0.203 |

Mean Brier of the five: 0.13. Five claims, one outcome per claim, but C215 and
C218 share one day and one event, so they are not independent observations.

## E4, novelty relative to what the paper holds on the subject (C221 to C231)

Hypothesis (coordinator, from C218): corpus-wide sparsity buries a follow-up in a dense region, so novelty must be
measured against what the paper already holds *on the arrival's subject*. Program `scripts/offline-experiments/e4`
(one top subject per arrival: C221 to C225, entered first) and `e4set` (the whole subject set: C226 to C231, entered
after the owner ruled one top subject out and before they were computed). Data and importance as C218: 6,268
arrivals, the extractor's names as subjects (4.3 per arrival on 11-23), furniture dropped above 2%.
Hour bins are (X-1:00, X:00]; the primary hour is (18:00, 19:00] (170 arrivals, 12 Huang-pattern), the turn hour
(14:00, 15:00] (169 arrivals, 22 Huang-pattern, the first reinstatement reports from 14:11Z). Rank 1 is the top.

### Single top subject (C221 to C225)

| Form (importance `I` times) | Best Huang arrival, hour ending 19:00 | Turn hour, best | Median echo rank share | Claim |
|---|---|---|---|---|
| (none, `I` alone) | 2 | 4 | 0.34 | |
| C218, 10th nearest corpus-wide | 14 | 33 | 0.47 | |
| (a) nearest earlier arrival on the same subject | 38 | 62 | 0.85 | C221 false |
| (b) the covering story's passages | 65 | 30 | 0.43 | C222 false |
| (c) new eligible words (count) | 4 | 24 | 0.72 | C223 false (rank 4, one place short) |
| (d) burst, fast over slow origin rate | 3 | 1 | 0.35 | C224 true |
| C225: a form with the turn in the top 3 and echoes at 0.60 or more | | | | false: (d) has the first and not the second; (a) and (c) the second and not the first |

(c') the share form ranks the best Huang arrival 1 at 19:00. The C224 pass is the 18:06 arrival of The Hill (the
Congress hearing on Huang) whose top subject is `democrats`: the burst is the subject's, not the story's, so the pass
is luck of the top-subject choice. A reinstat* word is new at 11:07 (`reinstatement`, MarketWatch), at 14:11
(`reinstated`, The Verge) and at 14:19 for the company release and wire copies, and again at 18:06 for an arrival
in another subject: it fires for the turn, as expected, and is not new by 18:00.

### The whole subject set (C226 to C231)

`I_sum` is the sum over the arrival's subjects of log2(1 + `R_s`) (as C218, which already summed over all subjects);
`I_top5` the mean of the five largest. Ranks of the best Huang arrival:

| Form | 19:00, `I_sum` | 19:00, `I_top5` | Turn hour, `I_sum` | Echo median share, `I_sum` | Huang echoes, median share | Claim |
|---|---|---|---|---|---|---|
| `I` alone | 2 | 3 | 4 | 0.34 | 0.23 | |
| (a2) nearest earlier arrival sharing two subjects | 14 | 2 | 17 | 0.90 | 0.77 | C226 false |
| (b2) the story covering the set | 8 | 4 | 44 | 0.47 | 0.80 | C227 false |
| (c2) new words among arrivals on its subjects | 14 | 15 | 23 | 0.81 | 0.77 | C228 false |
| (d2) burst of its five main subjects | 2 | 6 | 1 | 0.31 | 0.13 | C229 true |
| (e) new subject pairs and new subjects | 1 | 1 | 25 | 0.64 | 0.66 | C230 true |
| C231: turn in the top 3 and echoes at 0.60 or more | | | | | | false |

Reading, the lead's:

- **Both passes are one arrival.** The arrival that ranks 1 under (e) and 2 under (d2) at 19:00 is an 18:21 post by an
  aggregator account (`fllics.com`) that strings a Huang headline together with a roll of other Reuters headlines;
  it carries 11 subjects, importance 19.4 and 81% new pairs. The next Huang arrival is 21st under (e) and 13th
  under (d2). The set form does not find "the story"; it finds the arrival with the most symbols.
- **(e) floods its top ten with many-subject arrivals,** which are press releases, arXiv titles, heise features and
  rolls: by hand, 5 of the ten at 19:00 and 21:00, 6 at 15:00 and 5 at 03:00, and 9 at 09:00 are routine or a roll
  (the turn itself, a German Nvidia piece, is 6th at 15:00). The number of subjects an arrival carries drives the
  score. (d2) is cleaner at 15:00, where 8 of the top ten are the Huang reinstatement reports (the others a deal and
  a recipe) and the hour's top is the BBC piece on the US government's role, but at 03:00, 09:00 and 21:00 it
  floods with arXiv, earthquake and sports items (6 to 8 of ten, by hand).
- **Which form separates a turn from an echo and from an ordinary follow-up.** None of the registered forms. Burst
  separates the turn from the follow-ups at the arrival level (turn hour: Huang arrivals ranked 1 to 8; the
  18:00 to 19:00 follow-ups 13 and below once the roll is set aside) and cannot tell an echo (echo median 0.31,
  Huang echoes 0.13: it ranks them high, because an echo is part of the burst). The novelty forms (a, c, a2, c2) rank
  echoes low (0.72 to 0.90) and the turn low (best 17 to 62). (e) is in between on both.
- **Post hoc, not registered, one day:** burst with echoes zeroed (an arrival whose nearest earlier arrival is
  within cosine 0.95 scores nothing) keeps the turn at ranks 1 2 3 4 5 7 13 in the turn hour, puts the
  follow-ups at 2 13 16 22 32 ... and ranks echoes at median share 0.89. That echoes rank low by
  construction is trivial; what is not trivial is that the turn arrivals are mostly not near-copies of earlier
  arrivals and keep their place. This is the one combination that, on this day, separates a turn from an echo and
  a follow-up. It was found after the outcome and has not been tested on another day.
- **Does importance alone do the work?** Importance `I` ranks the best Huang arrival 2 at 19:00 and 1 in the hours
  16, 17, 22 and 00; what it cannot do is tell an old running story's follow-up from its turn, and every novelty
  form that tried to do that with a distance (a, b, c and their set forms) failed to rank the turn, because the
  turn's reports are close to the story's earlier reports (that is what makes them a running story). The signal that
  worked is a rate (burst), not a distance.

