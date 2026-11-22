# The offline slice of the first design: plan and costs (2026-11-22)

Written by a planning agent for the lead, read-only, no model call made. Costs and sizes are estimates with their assumptions stated. The design is [first-design-2026-11-22.md](first-design-2026-11-22.md), section 10 step 2; the claim is ledger C192. Rung 0 needs no spend and is started; rungs 1 and 2 wait for the owner (goal file, decision Q5).


Nothing was run, called or changed. Tags: **C** read from code in the lane
`lead-docs`, **D** read from a doc, **E** my estimate (assumptions stated),
**U** not found. Paths are under `/Users/wiz/Dev/go-choir/.lane/trees/lead-docs`.

## 1. Where a week of captures lives, how many, and what code exists

**How many (order of magnitude 10^4 a week; exact count U).** No existing script
counts captures per day (status-vectors reads journal lines for totals only).
Sources: 222,391 captures in the vector gap on 2026-11-19 (`docs/current-architecture.md:85`);
about 100 captures judged an hour on 2026-11-22 02:09Z to 03:09Z
(`docs/definitions/choir-first-design-2026-11-22.md`, "what_is_true_now");
188 an hour on 2026-11-09 (`docs/evidence/token-efficiency-audit-2026-11-09.md:249`);
70.8 an hour in an older baseline (`docs/learn/05-publication-and-autopaper.md:664`);
4 to 5K a day in October (`docs/evidence/keep-set-accounting-2026-10-29.md:99`).
Week = 12k (70.8/h), 17k (100/h), 32k (188/h); ceiling about 38k
((222k - ~28k at Oct 14)/36 days = 5.4k a day; includes research reads) **E**.
Planning figure: N = 17k to 32k. The first task is one `SELECT count(*)` per
day over `proj_capture_index` (`internal/platform/capture_index.go:61`).

**Where each ingredient is stored (all in corpusd's graph database):**
- Arrival time, outlet, URL, language, title: `proj_capture_index`
  (`capture_index.go:61-77`: `fetched_at`, `published_at`, `host`, `url_key`,
  `canonical_key`, `title`). Outlet = `registrableDomain(host)` (`capture_ready.go:292`).
  Index `idx_pci_fetched` serves a range by time.
- Vector: `vec_captures` (`internal/platform/vector_store.go:79-89`): ONE halfvec(256)
  per capture, made from title plus the lead passage (about 1,800 runes,
  `capture_ready.go:57-59`); only a `day` DATE and `embedded_on`, no arrival timestamp.
  "Passages" for a capture are therefore title + lead; the hot path never sees more.
- Text: the capture object in `og_objects` carries `ExtractedTextBlobID`; the body
  is a file in `CaptureBodyStore` on the host's disk (`capture_ready.go:468`,
  `vector_worker.go:196` `captureLeadPassage`). Size: 0.85 GB for 18,979 bodies in
  October (about 45 KB each; `keep-set-accounting-2026-10-29.md:99`) so a full
  week of bodies is 1 to 2 GB; the lead alone (2,400 runes cap) is about 100 MB.
- Wire origin: not stored. `wireOrigin(texts...)` (`internal/agentcore/event_layer.go:301-325`)
  is a 14-agency regexp on the first 300 runes. Coverage of real syndication unmeasured (U).

**What can be called offline as a library:**
| Ingredient | Code | Callable from outside its package? |
|---|---|---|
| candidates (exact, near, subset, initials, neighbour-entity) | `entityLexicon.mentionsFor` (`capture_ready.go:612`), `Service.assembleReady` (`:395`) | NO: unexported in package `platform`; needs an in-package test/harness or an export shim |
| extraction, proposals, gazetteer | `internal/entitytext` (`Parse`, `Propose`, `Gazetteer.Match/Near/Subset/Initials`) | YES, exported, no DB |
| lexicon build | `buildEntityLexicon` (`publication_entity_candidates.go:201`) reads `choir.entity` objects; each has `CreatedAt` (`FirstSeen`, `:221`) | unexported; as-of filtering is possible by `CreatedAt`, but aliases in metadata and the learned stoplist have no per-alias time (`publication_entity_learning.go`) |
| vector neighbours | `Store.CaptureNeighbours` (`vector_neighbours.go`), needs Postgres+pgvector; has `since` (lower bound only, `:47`), no upper bound | needs DB; the replay can instead brute-force float16 vectors in Go (see 2) |
| wireOrigin | `event_layer.go:306` | unexported in `agentcore`; 10 lines, copy or export |
| job splitter | `jevJob` (`internal/agentcore/jev_job.go:25`, `Plan` `:166`, `Halve` `:212`) over `internal/jevbudget` (exported: `Measure`, `Limit`, `:105-131`; limits 64k request / 32k state+question / 255 choices, `:25-32`) | `jevjob` unexported; `jevbudget` exported |
| the call | `gatewayruntime.JudgmentClient.Decide` (`SYSTEM1.md` section 3.4) | exported |

**Not usable as is.** The deployed ready list (`CaptureReadyList`, `capture_ready.go:315`)
lists only the NEWEST unresolved captures first (`ORDER BY fetched_at DESC`, `:196`),
max 500 a request (`:50`), with a lower-bound `ready_since` only: a past week
cannot be paged. It answers platform-tier computers only (`:430`; d@e is explicitly
"not a platform computer", `internal/computertiers/computer-tiers.json:38`). Its
neighbours have no upper time bound and its lexicon is today's, so it leaks the future.
**The "subjects of the nearest captures" source is empty today:** captures carry no
`mentions_entity` edges until E3 runs (`current-architecture.md:86` item 3; the
design says captures carry none). The replay must build that source itself from its
own earlier hours.

## 2. Where the replay runs, and getting the data there

Never write to the paper's database. Run on the build VM (`ssh choir-agents`,
`CLAUDE.md` Review) as a Go program over flat files; no Postgres is needed
(brute-force cosine over float16 vectors: 38k x ~100k x 256 is about 1e12 flops,
minutes) **E**. Derived tables as SQLite or JSONL on that VM only.

Export (one read-only job on Node B, off-peak, `nice`, statement timeout; **not
done by me and not covered by an existing script**: `scripts/status-storage`
shows the read-only `sudo -n -u postgres psql` pattern, but against the `control`
database; the corpus database's name was not found, U):
1. `proj_capture_index` rows for the week plus 3 prior weeks: about 1 KB x 150k = 150 MB text, gz 30 MB.
2. `vec_captures` for the same range, embedding as float16 binary: 512 B x 222k (all history for neighbours) = 114 MB; the week alone 20 MB.
3. Title + first 2,400 runes of lead per week capture: about 17k to 32k x 2.5 KB = 45 to 80 MB (read from the body files through the same code as `captureLeadPassage`, so it is the live view).
4. `choir.entity` objects and aliases (6,690 rows, under 5 MB) and the stoplist rows, with `created_at`.
5. `og_objects` flags (tombstone, superseded_by) for the week; `url_key`/`canonical_key` from (1).
Total about 150 to 300 MB compressed; transfer minutes; the query time on Node B is
the unknown (one index range read plus point reads of about 30k objects and about 30k files;
the only comparable load is the scan at `capture_ready.go:196`, 20,000 rows in one statement, U for its time).
Public routes do not carry captures; no export route for captures or vectors exists
(the vector import has a route, the export side was Mac-only: `vector_worker.go:200`).

## 3. System 1 cost (pick-only, Jev)

Facts from `docs/reference/SYSTEM1.md`: price $0.042 per million input tokens, output
billed 0 (section 7); limits 64k tokens a request, 32k for state plus one question,
255 choices (section 3.3); no stated questions-a-request limit, rate limits unpublished;
tokens = 412 + 0.224 per state char + 0.210 per question char, `jevbudget` more
conservative (Latin 0.30 a char). Measured: observed max input 17,481 tokens, 13+
questions a call is 81 calls total (section 7); accuracy above 12 captures is unmeasured
(rule 7). **A call here holds hundreds of questions: 10x to 100x beyond anything measured.**
Output was about 800 tokens a question on intake (section 7), unpriced but real
latency (the intake call timeout is 5 s + 1 s per 1k tokens, `our-usage.md` row 1).

Assumptions **E**: state per capture = title + 600 runes of lead about 210 tokens
(full 2,400-rune lead would be about 720); a question about 40 tokens; subject entries
about 15 tokens each, shared in a call. Candidates per capture c is the big unknown
(the E2 list gives up to 8 candidates a MENTION and a capture has several mentions):
c = 5 / 12 / 25.
| c | tokens a capture (incl. "other") | captures a call (64k, state also under 32k) | requests an hour at 100/h | tokens, week 17k | tokens, week 32k |
|---|---|---|---|---|---|
| 5 | 450 | about 140 | 1 | 7.7M | 14M |
| 12 | 730 | about 76 | 2 | 12M | 23M |
| 25 | 1,250 | about 43 | 3 | 21M | 40M |
Requests for the week: 168 to 840 (mid about 340; 188/h and c=25 gives 5 an hour).
Money at $0.042/M: **$0.3 to $1.7 for the week** (mid about $0.5 to $1.0). With the long
lead, roughly double. Compare: Jev ran $7.47 a day on 2026-11-10 (`SYSTEM1.md` section 7;
one weekday, stale). At 17.7 hours that is 16,133 calls; the replay is 340 calls.
The rate (15 a second, `internal/gateway/jev_limiter.go`) is nowhere near binding.

**Spending cap.** None for Jev found: `internal/gateway/paid_budget.go` caps only
Bedrock ($40 a day, owner 2026-11-10, `:64-68`) and River ($10 a day, `:69-74`),
enforced in `:154`. Jev's only brake is the OpenRouter credit balance, which ran out
once (HTTP 402, 3,907 failures, `docs/problems/paper-jev-credits-and-search-quotas-exhausted-2026-11-11.md`).
Balance today: U. The replay would draw from the same balance as the paper.

**Credential and identity.** The gateway is the sole holder of the Jev key (SEC-16,
`internal/gateway/host_judgment.go:12-16`). Two doors:
(a) guest `POST /provider/v1/judgments` with a computer's bearer: the d@e bearer
(computer-59c9..., `computer-tiers.json:38`), through `gatewayruntime.JudgmentClient`,
called from a program on the build VM or from a d@e cell (the `choir.System1` verb
takes only 12 questions, `SYSTEM1.md` 3.3, so a program, not the verb);
how a build-VM program obtains a d@e bearer: U, an operator act (`/provider/v1/credentials/issue`
exists, `handlers.go:1297`).
(b) host door `POST /provider/v1/host/judgments` allows only the caller `front-importance`
(`host_judgment.go:20-22`); a new caller is a reviewed gateway change (red: gateway and
provider calls). Do not take (b).
**Does it need the owner's word? Yes for any spend.** `CLAUDE.md` Delivery: "A grant
that spends the owner's money is an owner act on the tape." The amount is small (under
$2) but there is no recorded standing grant for experiments. Rung 0 needs none.

## 4. Escalation step (language model)

Unmade choices: which model is open question 4 (`first-design:11`); I price the two
paths the code already prices (`internal/llmcost/pricing.go`): Bedrock Haiku 5.5, $0.10 in /
$0.50 out per M (`:123`; owner's $40-a-day budget); River DeepSeek v4.1 flash, $0.12 /
$0.48 (`:129`). Per escalated capture **E**: title + 1,200 runes + top-10 nearest subjects
with one-line labels about 1,200 tokens in, 150 out; one pass per hour batch, 25 captures
a session, one retrieval turn each (x1.5 for repeats, mostly cached at $0.01/M on Haiku).
| escalation rate | captures (17k week / 32k week) | sessions (25 a session; at least one an hour with any) | tokens in / out (17k) | money, Haiku (17k / 32k) |
|---|---|---|---|---|
| 10% | 1.7k / 3.2k | about 168 | 2.0M / 0.26M | $0.3 / $0.6 |
| 30% | 5.1k / 9.6k | about 200 | 6.1M / 0.77M | $1.0 / $1.8 |
| 60% | 10.2k / 19.2k | about 410 | 12.2M / 1.5M | $2.0 / $3.7 (with the 1.5x: up to $5.5) |
River costs about the same. All of it is inside the $40 a day Bedrock budget but is the
paper's same pool: if the budget trips, the paper's own desks are refused, so spread
the run across more than one UTC day or take River. A 60% rate also means kill line (a)
(over 30%) already failed; the money is what is spent to learn that, so rung 1 stops
there. The gateway path for sessions run as d@e desks is the normal `/provider/v1/inference`
(`handlers.go:1280`); how a bare script on the build VM would call a model: U.

## 5. Rung 0: candidates only, no model, no spend

Free-standing and leak-controlled version: run the E2 candidate rules on (i) the NEWEST
500 captures through the deployed ready list as they stand today (one existing internal
POST, read only; no code; auth from the build VM U), whose lexicon cannot yet hold their
own stories, so there is no hindsight, and (ii) the replayed week with an as-of lexicon.
Measures:
- Kill line (c) in its own terms: hand-read 100 captures (one reader who did not write the
  rules; rubric fixed first: the reader writes the main subject in their own words, then
  says whether any candidate label is it; "captures that have one" = captures where the
  reader finds a subject at all), the count of 100.
- Candidates per capture (distribution), and the share with no candidate at all.
- Proxy for line (b): new proposed keys (names not in the lexicon) per 1,000 captures,
  days 1 and 2 against days 4 to 7, adding each proposal to a growing vocabulary. A proxy
  only: proposals are not subjects, 29% of today's rows are junk.
- Cost: the ready-list entries are entity-neighbour sourced only; with captures
  carrying no links the neighbour source reads zero, so rung 0 measures the rules AND the
  lexicon, not the vector source.
Cannot measure: the escalation share (a), skill (d), merges, whether System 1's yes/no
is accurate at 700 questions a call, whether a vector-neighbour-derived subject helps
(the source is empty), or any cost. A hand read of 100 gives a 95% interval of about
+/-8 points on a rate near 80%, so line (c) at 80 is decided only if the count is under 72
or over 88; between is "cannot tell", which counts as failing.

## 6. Settlement replayed offline (50 stories)

Needed: (1) predictions: existing stories hold none in a settleable form (`first-design:6`:
"nothing matches an arrival to a prediction"). One model call a story (50 calls, 5k in, 0.5k out,
about $0.05 on Haiku) reads the story revision as of the end of day 3 (the revision
timestamp; any later text is leakage) plus its subjects and writes 3 predictions with
subject, yes/no question, probability, deadline in days 4 to 7: about 150 predictions.
(2) Settlement: for each prediction, the days 4 to 7 captures that match its subject
(rule candidates or confirmed mentions) get the question in the hour's job; counted once
per origin (`wireOrigin` plus canonical_key plus vector cosine over a threshold I did not
set); an unanswered deadline settles false. 150 predictions x about 40 matching captures
(hub subjects: hundreds) = 6k to 20k questions, about 40 tokens plus the passage already in
the state if added to the same job (a question needing a confirmed mention is a second
request, `SYSTEM1.md` 5.3): about 0.3M to 1M tokens, under $0.05. Two requests a day
for days 4 to 7. (3) Base rate: the same questions asked of the 3 weeks before, for those
subjects only (small). (4) Skill: Brier skill against the base rate with a bootstrap interval.
**Power warning:** 150 binary outcomes, correlated within a story, give an interval on skill
of perhaps +/-0.1 or wider **E**; line (d) "interval includes zero" may be unmeasurable, which
counts as failing. Say so before the run, or raise to 150 stories.

## 7. Risks to validity

1. Hindsight: today's lexicon (6,690 entities with aliases that grew from stories published
   after the replay days), the learned stoplist, and `CaptureNeighbours` (no upper time
   bound) all see the future. Fix: filter entities by `created_at` before each day; restrict
   neighbours to captures with `fetched_at` earlier than the capture; freeze the stoplist
   and alias set as of day 0 (alias times are not stored, so aliases of entities
   created before day 0 may still carry names learned later: U how much).
2. Subjects are not the same as story entities: only stories gave the 6,690 rows, the slice
   mints its own; its merges and junk are then the slice's; but the starting lexicon carries
   29% non-entities and 24% duplicates (`docs/evidence/entity-layer-baseline-hand-read-2026-11-20.md`),
   which depresses (c) and inflates line (b). Run the lexicon through the same stoplist or
   report both.
3. Repeats by URL: `url_key`/`canonical_key` repeats and `superseded_by` rows; dedupe
   inside the week, count N after dedup and say so; wire copies need one origin (the regexp
   covers 14 agencies on the first 300 runes; others become independent evidence).
4. Embeddings missing at arrival: at 2026-11-19 14:57Z 6,449 of 222,391 captures had none and the
   median write to embed was 115 s before E1; the replay uses the final vector, so it
   overstates neighbour availability. Mark the share of captures with no vector at
   arrival time (U, not recorded).
5. Batching at 40 to 100 captures per call is unmeasured; hour boundaries differ from the live poll
   cadence; answers are blind to each other. Rung 1 includes a same-captures comparison
   at 12 captures a call.
6. Hand-read bias (author reads own rules); one reader; the design wants a reader that did not
   make the judgment (`choir-first-design-...md` acceptance).
7. Contention: the paper's Jev credit and Bedrock budget.
8. A run that cannot measure a line counts as failing it (design kill lines): decide the measurement
   code before any run (also ledger C192 already entered).

## 8. Staged plan

| Rung | What | Decides | Spend | Builder effort |
|---|---|---|---|---|
| 0 | Export (1,2,3,4,5 above); candidate stage as an in-package harness (`platform` test with env data dir; or an export shim); as-of lexicon; hand read of 100; counts of candidates, zero-candidate share, proposal novelty by day; optional newest-500 via the deployed route | kill line (c) with a real measure; whether rule candidates are good enough to be worth any model spend; actual N, c | none | export 0.5 to 1 d (plus lead/owner time on Node B), harness 1.5 to 2 d, hand read 0.5 d: **about 3 to 4 days** |
| 1 | One day (about 2.4k to 4.5k captures) through System 1 and the escalation step; same 100 captures at 12 per call versus full hour; 25-capture escalation session; escalation share; mentions written | lines (a) and the accuracy at large batch; real tokens per capture, c, requests an hour | Jev about $0.05 to $0.2; model about $0.1 to $0.5 | `jevjob` wrapper + job build + answer parsing 2 d; escalation prompt and subject check 1.5 d; **about 3 to 4 days** |
| 2 | Full week, days 1 to 7; 50 stories predict on days 1 to 3, settle on 4 to 7; hand read 200 | all four lines | Jev $0.3 to $1.7; model $0.3 to $5.5; predictions about $0.1; total under about $8 | prediction generation + settlement + skill 3 d; rerun and report 1.5 d; **about 4 to 5 days** |
Total about 10 to 13 builder days; one lane, no landing, all output off the paper.

## 9. Owner decisions needed before any spend

1. Permission for a read-only export of the paper's captures, vectors and entity rows from
   Node B's corpus database (who runs it, at what hour). Needed for rung 0 even with no spend.
2. A spending grant on the tape for Jev: suggested ceiling $5 for rungs 1 and 2 combined (estimate
   $0.4 to $2), through the d@e identity and the guest door; and who issues the d@e bearer to the build VM.
   Refuse a new host caller (red change).
3. A model and a ceiling for the escalation desk and the 50 predictions: suggested Haiku 5.5 (or River
   DeepSeek v4.1 flash), ceiling $10, spread over at least two UTC days, inside the $40 Bedrock budget.
4. Confirm the OpenRouter credit balance is enough (not recorded anywhere I found).
5. Who is the hand reader and the rubric for "the right subject" (fixed before the read, by someone who did
   not write the rules), and whether 50 stories is enough for line (d) given the power warning.
6. Ledger entries for the new measurements before each rung (CLAUDE.md Proof).

Could not find: the exact capture count for any week; the corpus database name and host access path from the
build VM; how a bare script calls a model through the gateway; the OpenRouter balance; the share of captures without a vector at
arrival; alias creation times; per-question output tokens for a yes/no choice at large batch.
