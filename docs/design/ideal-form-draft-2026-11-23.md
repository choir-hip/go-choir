# The ideal form: from an arriving item to a better corpus, and the root that holds the view

Standing: superseded | kind: decided | scope: in part | settled-by: owner 2026-11-24 'the new design superseded' (header line 5; system-map section 10) | replaced-by: docs/design/desks-and-engine-2026-11-24.md | note: where they conflict

> **Superseded in part (2026-11-24).** [Desks and engine](desks-and-engine-2026-11-24.md) is the current design and wins where they conflict. Superseded here: "happening" as a kind (a group that matters is named and becomes a subject); flat batches into one model call with no desk (capture is its own desk on the RLM pattern, grouped calls with learned context); the graph custodian as a free-standing host service (it is the capture desk's); management holding a bound on the root (management manages scoring). Still standing: the measure, importance counted from independent origins on subjects, the page as a host render the root steers, the path that begins with instrumenting and experiments.


Draft for the owner, 2026-11-23. A peer's proposal, read-only on the repository (`origin/main` in `lead-docs2`). Nothing here is built. Numbers are from today's session record, draft 4 of the first design, the offline-slice evidence and the system map unless marked **(guess)** or **(estimate)**, which means I derived or assumed them and nobody has measured them.

The short version:

1. The objective can be one number: the time-averaged divergence, in bits, between the world's attention over happenings and the front page's prominence over them, plus an estimate of what no source of ours reported. It needs no held-out source.
2. The funnel's order is: clean, copy check, embed, neighbours, then keywords read *against the neighbourhood*, graph, rule bands, System 1 on the residue, a language model once per group. The failed trial says names cannot lead; vectors lead and keywords find what is new inside a dense region.
3. Importance lives on the subject, not the story, as a counted rate of independent origins. That alone would have put the Huang story at the top today.
4. The root does not write the front page. It edits the parameters of a ranking that host code renders, so the page and the radio queue exist when every desk is down.
5. The owner's batching hypothesis is arithmetically strong at today's rate and at 10x. I expect a hybrid to win. The experiment in section B decides it.

---

## A. The objective, made exact

### A.1 Definition

Three objects, all computed from what the system holds:

- A **happening** `h` is a group of arrivals from sources (never research's own reads, never our stories) that report the same thing. The funnel makes groups anyway (section B).
- **World attention** `p_h(t)`: the share of independent-origin weight on `h` in a trailing window. One origin counts once per happening, wire copies count once, and each origin carries a weight `w_s` (section C.3). `p` sums to 1.
- **Our prominence** `q_h(t)`: the share of front-page rank mass held by the story that covers `h`. Rank mass is a fixed discount by position (for example 1/log2(rank+1), normalised). A happening with no published story gets a floor `ε` (for example 1/1000). `q` also sums to 1.

The measure is the divergence of the page from the world, averaged over time:

`D = mean over t of Σ_h p_h(t) · log2( p_h(t) / q_h(t) )`

It is zero when the paper gives each happening the prominence the world gives it, at the moment the world gives it. It is the "perplexity of our corpus versus the world" in the owner's sense: the bits of surprise a reader of our front page would feel on seeing what the world is reporting.

### A.2 It decomposes into the parts a design can act on

| Term | What it is | Which lever moves it |
|---|---|---|
| **Missing** | Σ over happenings with no story of `p_h · log2(p_h/ε)` | open stories on high-mass uncovered groups |
| **Late** | the same term, charged for every minute between the first origin's own timestamp and our publication | poll interval, funnel latency, the two-outlet hold |
| **Misranked** | Σ over covered happenings of `p_h · log2(p_h/q_h)` | the importance state, successor decisions |
| **Unseen** | mass of happenings no source of ours reported (A.3) | adding sources |
| **Personal** | the same `D` with `p` replaced by one listener's revealed attention and `q` by their edition | the person's root |

Today's failure was a Misranked spike: a happening with 63 sources held about a tenth-place share. The Missing and Late terms were zero. A metric that only counted coverage would have shown a perfect day.

### A.3 The unseen part, with no source withheld

The owner does not want a held-out set. There is a standard estimator that needs none. Count happenings by how many independent origins reported them. Let `f1` be the number reported by exactly one origin and `f2` by exactly two. Then:

- Good-Turing: the probability that the next origin-report is of a happening we have never seen is about `f1 / N` (N = all origin-reports in the window).
- Chao's lower bound on the number of happenings nobody in our panel reported: `f1² / (2·f2)`.

Both are computed continuously from the group table. Rung 0c already gives the inputs: 58% of unmatched arrivals end in groups that never reach two outlets. That says the unseen share is large, and that most of it is chatter. So the estimator is run on the subset System 1 answers "reports a happening" for, and weighted by source class.

Two cross-checks, both cheap:

- **Outside front pages** (the owner's "sources we can't use"). Once a day research reads five to ten front pages we do not ingest and System 1 judges, per headline, whether a published story covers it. About 20 to 50 searches a day. This is the root's meta-reporting.
- **A literal perplexity on a 1% sample (guess that it is feasible).** A small local model scores an arrival's lede twice: with no context, and with the five nearest corpus passages as of the moment before. The fall in bits per token is the corpus's information about that arrival. It is the only estimator here that is perplexity in the textbook sense. I would run it as a check on `D`, not as the measure, because it costs a model pass per sample and cannot be decomposed.

### A.4 Why it cannot be improved by reading less or by self-marking

- **Reading less.** The delay clock starts at the source's own timestamp, so polling less often raises Late. The panel is append-only for measurement: a source keeps a floor fetch (feed and embed, no model) even when its processing is cut, and `D` is also reported on a fixed-vintage panel, like a price index with a fixed basket. Removing a source from the panel is an owner act. This reconciles "ideally we are reading less" with the metric: *process* less, *measure* on everything.
- **Self-marking.** `p` is built from captures only. Our stories, our research reads and any origin that cites choir.news never count. `q` is a distribution, so a story cannot gain by mentioning everything: prominence given to one happening is taken from another. Whether a story covers a happening is judged by System 1 on the story's main point and lede, with a random share re-judged by a language model and a monthly hand read. The writer of the story is never the judge.

### A.5 Cost and known biases

Cost: no new model work beyond the funnel's, plus one System 1 question per (group, candidate story), a few hundred a day, and the daily outside check. It is a fold updated by events (arrival, publish, front change), not a timer.

Biases, stated plainly:

1. **The panel is not the world.** 19,278 of 23,851 weekly arrivals were social. `p` is attention among our sources. Class weights (official, wire, outlet, social) are an editorial prior the owner sets; they are not learned, and the metric should say so.
2. **Grouping is ours.** The pipeline could lower Misranked by merging everything into few groups. Group quality needs its own audit (four in five three-outlet groups were one happening on a read of 50; chaining and aggregator accounts are known defects).
3. **Attention is not consequence.** See A.6.
4. **`ε` and the rank discount are conventions.** They set how much a miss costs against a misrank. Fix them once; report the terms separately so the choice is visible.

### A.6 Is "importance is a forecast of what follows" sound?

It is sound as a forecast target. It is the only version of importance with a prompt, specific teacher that cannot be flattered: later arrivals from independent origins. It matches the owner's definition ("the importance of that story in the world should be roughly the importance of it in Choir"). It fails in three places.

| Failure | Why | What to put beside it |
|---|---|---|
| Things that matter and are barely talked about | follow-on attention is low by definition | (a) a per-source lead record: an item from a source whose past firsts later became large groups starts high; (b) a slow outcome at 7 and 30 days: independent origins still arriving; (c) listeners' asks; (d) standing subjects the owner declares, as owner acts, not learned |
| Manufactured attention | many accounts, one origin | count origins, not copies; collapse near-identical text; weight an origin by its originality record; cap the social class's share of `p` |
| Self-fulfilling prominence | readers and outlets follow our page; listeners hear what we queue | only captures settle; origins citing us are excluded; a small random share of front and queue slots is filled against the ranking with the draw probability recorded, so reader outcomes can be corrected for exposure |

One item in the lead's list should be dropped: "how many later stories cite it". Our stories are our own work. Counting them as the outcome is marking our own paper.

What I would *not* do is ask any model how important something is. That is what failed today.

---

## B. The funnel

### B.1 What the failed trial says about order

Rung 0 failed all four kill lines. Its findings fix the order of stages:

1. **Names cannot lead.** Rules over text found actors and proposed the happening for one capture in five. Vectors found the right story three times in four at K=10 (82% at K=20) with no model. So vectors come first.
2. **Keywords come after vectors, and are read differentially.** The useful keyword question is not "what is this about" but "which words here are rare *in this neighbourhood*". "Reinstated" was not in the petition story. That is the cheap detector for a turn inside a dense region, which neither density nor nearness can see.
3. **Unmatched is the common case** (53 to 59% of arrivals). It cannot escalate per item. It pools and groups.
4. **Clean first, arrivals only.** Two thirds of captures were research's reads; a quarter of news-like arrivals report nothing.
5. **Kill line (d) was never measured.** Predictions as subscriptions are unproved. I would not make a story's wake depend on a prediction existing. Wakes come from counted arrivals; predictions ride on the same events for learning. This is a disagreement with draft 4, section 3 (see G).

### B.2 Stages

Rates: 3,400 source arrivals a day in the week of 11-15 (142 an hour); management judged 200 to 240 an hour today. I use 200 an hour as "today". Survival is out of 100 arrivals.

| # | Stage | Decides | Writes | Left | Cost per item |
|---|---|---|---|---|---|
| 0 | Fetch, clean to the article, source timestamp | is this an item (404, bot check, index: 6%) | capture; the decision row (F) | 94 | network; ~10 ms CPU (guess) |
| 1 | Copy check: canonical URL, content hash, shingle hash over 72 h | same text as a held item | a copy row pointing at the original; one origin if the wire origin differs | ~84 (guess; 7% URL repeats measured) | under 1 ms |
| 2 | Embed the lede (passages later, only for survivors) | nothing | one vector | 84 | ~260 ms local (estimate from the query embedder) |
| 3 | Neighbours: 20 nearest story passages; nearest arrivals in 72 h. Density = novelty | near a story, near a group, or sparse | density, neighbour ids | 84 | two reads, 67 to 97 ms each while K is 80 or less |
| 4 | Names (deployed candidate service) and delta terms against the neighbourhood | carries terms its neighbours lack | tentative mentions; the delta terms | 84 | ~10 ms (guess) |
| 5 | Graph: neighbours' stories, their fixed main points, subjects, importance state, succession chain, the source's record | the candidate list, 8 or fewer | candidates | 84 | 1 to 2 index reads per candidate, ~30 ms (guess) |
| 6 | Rule bands, no model | **echo** (very near a group member, no delta terms): add one origin, stop. **Confident feed**: attach to the story's evidence. **Unmatched**: pool and group online. **Ambiguous**: pass on | origin count; group membership; importance update | echo ~20, feed ~8, pooled ~46, ambiguous ~10 (all guesses except pooled, from 0c) | under 5 ms |
| 7 | System 1, one call per flush | for ambiguous items: reports a happening? serves main point M? reports a development M does not state? For groups: one happening? Plus a random 5% of stage 6's decisions | answers with probabilities; audit rows | ~15 items' worth of questions | ~1,000 tokens per item, $0.00004 |
| 8 | Language model as a tool, per group, never per item | name a group that crossed the line (two or more origins, or a high forecast from a source with a lead record); confirm a turn; the hard residue, many items a call | a main point; open / successor / decline | 0.4 to 2.7 calls per 100 arrivals (from 0c: 13 to 92 groups a day) | one call |
| 9 | Story sub-call; research | the revision; a named gap | Texture revision | | sessions |

Of the 46 pooled, about 27 never reach a second origin and cost nothing more. They stay searchable and stay in the radio's non-agentic ranking.

**The guard on stage 6.** The lead proposed that an echo costs a vector match and no model. That is right for cost and dangerous for exactly today's case: a turn in a running story is dense and near. An item is an echo only if it is very near a group member **and** has no delta terms **and** the group shows no burst. A burst (origins in the last two hours far above the group's trailing rate) sends the group's newest items to stage 7 whatever their nearness.

**Arrival to decision:** local stages total under one second. The flush adds its debounce (30 to 60 s) and one call. So an item is held, searchable, counted and rankable within about two minutes of the poll. Publication of a written story is a different clock (G.2).

### B.3 Physical bounds

| | Today (200/h) | 10x (2,000/h) | 100x (20,000/h) |
|---|---|---|---|
| Arrivals per second | 0.056 | 0.56 | 5.6 |
| Embedder, one unit at ~3.8 texts/s (estimate) | 1.5% | 15% | 146%: **binds** |
| Vector reads, 0.2 s per item | 1% of a core | 11% | 112%: batch per flush, approximate index |
| System 1 questions per 60 s flush (20% residue, 8 each) | 5 | 53 | 530: **accuracy unmeasured above 12** |
| System 1 dollars a day | under $0.10 | ~$0.40 | ~$4 |
| Group-naming calls a day, if groups grow as arrivals^0.7 (guess) | ~70 | ~350 | ~1,750 (73 an hour) |
| Capture bodies, 164 KB each (estimate: 41 GiB / 250,000) | 0.8 GiB/day | 8 GiB/day | 79 GiB/day: **binds first** |

Where the constraint sits:

- **Today** it is designed in: a management session per 12 captures, 12 dispatcher slots at 12/12, about 9 model calls a capture. The hot path as host code removes it.
- **At 10x** it is System 1 accuracy per call and the disk. Node B has 160 GiB free and the store already grows 2 GiB a day. Copies must store a pointer, not a body. Passage vectors are made only for survivors.
- **At 100x** it is the one host: embedder, vector reads, the four poll slots, the disk. The design at that scale needs a second embedder unit, per-flush batched vector reads, and body retention by importance. None of that changes the stage order.

The exponent 0.7 is the scaling law draft 4 asks for, and it can be measured with no spend: replay the week with 10%, 25%, 50% and 100% of sources and count multi-origin groups (section F). If it is near 1, model work grows with sources and "carrying only the signals that comprise the news" is false.

### B.4 The owner's counter-hypothesis: batch 10 to 50 captures into one language-model call

The arithmetic favours it more than the session record admits.

- Today: 3,400 arrivals a day at 25 a call is 136 calls a day, about 6 an hour, about 8,000 input tokens each. The Synthetic bucket sustains about 47 calls or 2.3M tokens an hour and sat at 97% full. **The whole intake fits in a provider that is currently idle.** The system makes about 1,900 calls an hour now.
- With a 60 s flush the batch is 2 to 4 items today and 30 to 40 at 10x. Batching improves with rate. At 100x it is 570 calls an hour and needs the paid provider.
- A batch call with no retrieval cannot know what the corpus holds. So the honest form of the hypothesis still runs stages 0 to 5 and replaces stages 6 to 8.

**The experiment.**

| | |
|---|---|
| Input | one replayed day of source arrivals in order, corpus frozen as of each flush, derived tables only |
| Arm A | the staged funnel of B.2 |
| Arm B1 | batches of 25, cleaned ledes only, no retrieval: group, route, flag |
| Arm B2 | batches of 25, each item with its five nearest stories' main points and its group |
| Arm C | what management did that day (needs the decision record of section F) |
| Outcome 1 (the world) | for each arm's "new happening": did two or more further independent origins arrive within 24 h. For each happening that reached three origins: did the arm flag it by the second origin, and how long after the first |
| Outcome 2 (a blind reader) | 200 items, stratified, read by an agent that sees no arm's answer |
| Outcome 3 (turns) | 20 hand-collected cases from the week where a story's point moved, today's among them |
| Cost | input and output tokens, calls, System 1 questions, local CPU, p95 arrival-to-decision; priced at the paid fallback's rate, since a free quota is not a price |

**Kill lines, set before the run:**

- B2 replaces stages 6 to 8 if its agreement with the blind reader beats A's by 5 points or more, at no more than twice A's paid-rate cost, with p95 latency under 5 minutes.
- A stands if it is within 5 points of B2 at half the cost or less.
- If B1 matches B2, retrieval before the model is not earning its place for routing, and stages 3 to 5 are kept only for the metric and the radio.
- If no arm reaches 14 of 20 on turns, the successor decision needs a different mechanism and nothing is wired.

**What I expect (guess, 0.6):** B2 wins on quality at today's rate. The design then becomes: local stages for copies, echoes, counts and candidates; a batch language-model call for routing and naming; System 1 as the auditor and as the path beyond 10x. That is a hybrid, and it gives the idle bucket its job.

---

## C. The four strokes inside every stage

### C.1 Decision by decision

| Decision | Forecast, by whom | Outcome, when | Judge | Lesson, read where | Trust that moves |
|---|---|---|---|---|---|
| Is it an echo | host rule, with the band's measured rate | the 5% audit (minutes); whether the story later cites it for a claim the original lacks (hours) | System 1; a language model on 1%; a monthly hand read | per source class threshold table in the root's Texture | the band the rule decides alone |
| Which story or group | stage 7 or the batch call | the story's Texture uses it or rejects it ("not this main point"); later arrivals' grouping | the Texture (not the router); host counts | hit rate per candidate source (vector, keyword, graph) | candidate order; which band skips the model |
| Open | the namer: "two or more further origins within 24 h", p | arrivals | host count over captures | root: by source class and subject | whether one origin may open for this source |
| Succeed | the namer: "within 24 h most new arrivals on this subject sit nearer the successor's point than the old one's" | arrivals | host, by vector and System 1 | root: by subject class | the number of origins a turn needs |
| How important | the importance state (C.2): a distribution over origins in the next 24 h | arrivals at 24 h, 7 d, 30 d | host count, our output excluded | skill against the base rate, per subject class | how far the state may move rank alone |
| How novel | not a forecast: density is computed | its usefulness is settled by step 2a: does density separate opened from fed | host | the radius and window | whether density enters any rule |
| Poll a source how often | scheduler: "next poll brings k items, u not echoes" | the poll | host | the source's record | the poll interval, above the measurement floor |
| Spend a research run | the asker: "adds one independent origin, or fills the named gap", p | the Report's new captures, judged by the citation judge that exists | System 1 and host | root: "a second outlet was found for one-origin social items 1 time in 5" | the forecast gain a run needs |
| What goes on the front | the rank itself is the importance forecast | `D`'s Misranked term over 6 to 24 h; readers, corrected by the random slots | host metric | root | how far the root's edits may move the page |
| What a listener hears next | the DJ's `p_heard` (built; settlement proven on the drill computer) | the listener's act, seconds | the listener | the notebook | how far the DJ may depart from the non-agentic order |

### C.2 Where no honest outcome exists

| Decision | Why there is no teacher | What to do |
|---|---|---|
| A decline that would have mattered | counterfactual | a random share of declines is processed anyway, with the draw probability recorded (the 2026-11-10 intake design had this) |
| Quiet but consequential | attention never arrives | owner-declared standing subjects; the 30-day outcome; listeners' asks. Accept that the paper will be late on some of these |
| Is this one story or two | a convention, not a fact | a stated rule (D.4), audited by a second model and a hand read. Do not pretend it is learned |
| A one-origin niche item | no second origin ever settles it | only listeners' asks and exploration can. Small weight |
| A new listener's taste | no data | the public ranking until there is |

By the learning engine's three-question test, a site that has no teacher stops asking for a forecast. Those sites get a rule and an audit instead.

### C.3 Update rules, precise enough to simulate

**Importance state, per subject or group `j`.** Hold a rate of weighted independent origins per hour as a Gamma with shape `a_j` and rate `b_j`. Each hour: `a ← γ·a + n`, `b ← γ·b + 1`, where `n` is the hour's weighted origin count and `γ = 0.5^(1/half-life)`. Keep two copies: fast (half-life 2 h) and slow (7 d). Then:

- importance `I_j` = the fast mean `a/b`, normalised over all `j`. That is `p_j`.
- the forecast for the next 24 h is the negative binomial the Gamma implies. Score it by log loss against the realised count. Skill is the gain over a per-class base rate.
- burst `z_j` = (fast count − slow expectation) / sqrt(slow expectation). `z ≥ 3` (guess) marks a candidate turn.
- a first report with no history takes its prior from its source: the mean follow-on origins of that source's past firsts.

**Source record, per source `s`.** Originality is a Beta: when a group closes, `+1` to the first count if `s` was first, `+1` to the echo count if not. Lead time is the median gap from `s`'s report to the second origin, when `s` was first. Weight `w_s ← w_s · exp(−η · loss_s)` (multiplicative weights), where `loss_s` is the log loss of the forecast "an item `s` reports first gets two more origins in 24 h". Niche is the share of `s`'s firsts on which no source of weight above the median ever followed, and it has no honest outcome (C.2).

**What stops the root declaring a subject important by fiat.**

1. A root edit is a forecast, not an assignment: "subject `j` will receive `n` or more origins in 48 h, p". It takes effect at once.
2. It is a bounded multiplier on the learned state (for example between 1/4 and 4), and it decays back to the learned value with a 24 h half-life unless arrivals settle it true.
3. The bound widens and narrows with the score of the root's past edits. That is the trust stroke, held by management as delegator.
4. The root cannot write outcomes, origin counts or `D`. All three are host folds over captures.
5. A standing subject with no decay exists, and only the owner can set one.

---

## D. The root

### D.1 What it is

The root is the Texture desk's top agent: one long, resettable RLM session whose memory is its own Texture and that Texture's revisions (owner, ruled). What I add is what the Texture *contains*:

- **The index**: subjects, aliases (a merge is a row), each subject's learned importance and any live edit.
- **The page plan**: sections, pins, the ranking's parameters.
- **The notebook**: lessons, each citing the records behind it.
- **The paper's own metrics**: `D` and its terms, the funnel's counts and audit rates, calls per arrival and per story, resource headroom.

**The front page is this Texture rendered by host code.** The body is a ranking function over live state (importance, novelty, recency) with the root's parameters and pins applied. The root edits parameters; it does not type the list. This is how three rulings hold at once: the root Texture is the front page; the radio never waits; there is always a non-agentic ranking. If the root is down, the last revision's parameters and the live state still render a fresh page. If the owner means the model literally writes the page each time, I disagree (G.1).

### D.2 Wakes, inputs, powers

- **Woken by** a debounced digest of threshold crossings: a group reaching its origin line with no story; a burst on a subject that has a story; a forecast settled; a term of `D` moving; a source's record breaking; a listener's ask the corpus could not answer. No timer, no hourly cap, no minimum gap. At today's rate that is a few wakes an hour (guess).
- **Given aggregates, never captures**: the top movers by importance; uncovered groups with origins, age and three sample titles; per source, arrivals, echo share, firsts and cost; funnel counts by stage; `D` by term; the scorecard of its own edits.
- **May change with code** (every change a revision, so seen and revertible): the subject index; importance edits under C.3; pins and section layout; a source's poll weight above the floor.
- **May not write**: captures, origin counts, outcomes, metrics, a story's text, the panel's membership.
- **Delegates**: stories as sub-calls (open, succeed, merge, close), ordered by uncovered world mass and then by misranked mass; research for a named gap and for the daily outside-front-pages check.

### D.3 What remains for management

Computer-wide coherence; admitting engineering; routing the owner's prompts; leases; and being the root's delegator: it reads the root's reliability and holds the bound on its edits. It makes no per-capture decision and holds no story list. Nobody marks their own work: the world judges the importance state, the host judges the root's edits, management sets the root's trust, the owner sets management's.

### D.4 Story identity

1. A story has one main point, fixed at open, with its vector. (Stories opened before 2026-11-21 have none. The few hundred active in the last 14 days get one written once, in a batch call, or they can never turn.)
2. **Importance lives on the subject, not the story.** A successor inherits the subject's accumulated state and its burst on its first minute. Today's reinstatement would have opened at the top.
3. A **turn** is decided on the group, not on one capture: three or more independent origins (guess) whose System 1 answer is "about this subject, and reports a development the main point does not state", or a burst with delta terms absent from the story. One language-model call confirms and writes the new point. The successor records `succeeds`.
4. **"Lean to revising" applies only when the evidence is unclear**: fewer than three origins, or mid-band probabilities. When the trigger is clear there is no lean.
5. The owner's election: campaign, count, result, aftermath are four stories in one chain on one subject. The front shows the head of the chain; the head transcludes its predecessors as background. The chain is the page for the subject.
6. Against shallow fragments: a successor forecasts its own arrivals over 24 h. If it gets none, it folds back into its predecessor as a section (a reversible revision), and the origin line for that subject class rises.

The cause of today's burial was also structural. Feeding costs management nothing; opening costs a main point and a judgment per candidate; the prompt says "when unclear, feed"; 3 of 60 opens were successors. Moving the decision to a group-level trigger removes that asymmetry.

### D.5 A person's edition and the radio

- **Edition** = the same render on the person's computer, over the paper's public objects plus private arrivals (mail, Textures tracking engineering work), with `p` replaced by that listener's record. Private items never leave the computer. To keep interests private, the computer fetches a broad public candidate set and ranks locally.
- **Autoradio connects to the root** and reads the rendered queue. Four layers, each a fallback for the one above:

| Layer | Where | Latency |
|---|---|---|
| 0 | the phone's saved queue, including everything since the last listen (not discarded at 30 minutes) | none |
| 1 | the host's non-agentic edition render | milliseconds |
| 2 | the Autoradio desk (a real RLM) relinearises and scripts transitions, returning a new queue as a function call | seconds to a minute |
| 3 | the root's edits | minutes |

The deck always plays from the lowest layer that has something. The desk replaces the tail, never the item on air. On a voice turn the local model answers at once and the desk's queue arrives behind it. The desk's reorderings are forecasts against layer 1's order; if they do not beat it on listen-through, the distance it may move an item shrinks.

---

## E. Resources

The rule forbids refusing or queuing our own calls. So allocation happens in two places only: **whether work is created** (a decision with a forecast, section C) and **where it is routed** (by measured headroom).

| Resource | Shape | Give it |
|---|---|---|
| Local copy check, vectors, keywords, graph | milliseconds, bounded by one host | every arrival, always |
| Local embedder | ~4 texts a second (estimate) | ledes at arrival; passages only for survivors |
| System 1 | cents a day; accuracy falls with questions per call | the ambiguous residue and every audit |
| Synthetic weekly bucket (about 47 calls or 2.3M tokens an hour sustained; 97% full) | steady, token-rich, wasted when full | the steady batchable load: batch routing and naming, the root's sessions, the outside check. Route more to it the fuller it is; aim to sit near 50 to 80% so a news burst has room |
| Devin flash (fast, free, sheds 40 to 160 an hour; its catalogue entry says free until 2026-12-04) | bursty | story sub-calls, where latency matters and volume follows news |
| Paid fallback | dollars, an owner-set cap is allowed | radio turns and overflow |
| Web search (one provider answering; 1,568 today) | daily credits | by forecast gain: corroborating a high-forecast one-origin item; named gaps; 20 to 50 a day for the outside check |

The largest saving is not routing. Today 41 of 48 new stories got a research Cast, and about half of publish holds are for fewer than two outlets. The paper opens on one origin and then pays research to find the second. If opens wait for the second origin to arrive by itself (51 to 92 such groups a day), most of that search spend disappears, and what remains goes to depth.

Value per unit: for a call about happening `h`, the expected fall in `D` is about its world mass times the chance the call changes coverage or rank. The host computes that and the root orders its sub-calls by it. It orders; it does not refuse.

---

## F. The path

**Instrument first.** Nothing below can be settled without it.

1. **A durable decision record**: one row per capture and stage (capture id, stage, decision, who decided, candidates shown, probability, time), kept on the host, not in a two-hour console file. Today's management decisions are written into it first.
2. The run ledger gains the prompt a model saw, what woke the run, and its search queries.
3. F0 (the desk on every model call) is built and not deployed. Deploy it.

**Station 1: measure. No reader sees a change.**

- The decision record, live.
- Step 2a (ledger C203): does density separate fed from opened.
- `D` version 0, computed offline over the replayed week. *Claim to enter:* on 2026-11-23 at 19:00Z the Huang group is in the top three by world mass while the front held it tenth, and Misranked for that hour is the week's highest. If `D` cannot see the failure that started this, it is the wrong measure.
- The scaling exponent, by source subsampling. *Claim:* under 0.85.
- The 20-turn set, collected by hand.

**Station 2: shadow. Derived tables only.**

- Stages 0 to 6 run live beside the three existing mechanisms; their decisions are compared with management's and settled by later arrivals.
- The batch experiment of B.4.
- The importance state runs in shadow and produces a shadow front order. *Claim:* its Misranked term is below the live order's on five of seven days.
- System 1 accuracy against questions per call, 12 to 500.

**Station 3: the first swap a reader sees.**

- The front order reads the importance state. The ten-minute scorer is retired after the shadow claim settles true. This fixes today's failure and touches no part of intake.
- The turn trigger is offered to the existing management task as one more fact per capture ("this subject is bursting; these terms are new"), which changes its input and none of its authority.

Only after those: group-level opens replace per-capture management tasks, the intake label's question and the event layer, together; then the root with story sub-calls; then the edition and the radio's layers.

---

## G. Unsure, and where I disagree

### G.1 Where rulings pull against each other or the evidence

1. **"The root Texture is the front page" against "the radio never waits".** Resolved only if the page is a render the root parameterises (D.1). Needs the owner's yes.
2. **"Anything consequential more than a minute old should be in Choir."** The default poll is 15 minutes, so the mean wait before we even see an item is 7.5 minutes. Groups took a median 9.3 hours to become stories. I recommend two clocks: *held* (searchable, counted, playable as a one-source item: two minutes after the poll, and one minute only with push sources) and *published* (written and corroborated: tens of minutes).
3. **"One System 1 call with every question" against measured accuracy.** At 10x a flush is 53 questions; hundreds are 10 to 100 times past measured accuracy. The reference page already allows separate calls for states that are not shared. I recommend the owner rule that one call per *neighbourhood* (items sharing candidates) is that case, not a batch constant.
4. **"Lean to revising" against "a story stops when the beat changes".** Together with a free feed and a costly open they produced 3 successors in 60 opens. D.4 is my recommendation.
5. **"Ideally we are reading less" against a metric that must not reward reading less.** Process less; measure on all (A.4).
6. **Predictions as subscriptions (ruled) against kill line (d), never measured.** I would make counted arrivals the wake and predictions the learning signal riding on it, until predictions show skill.
7. **"Desks talk only through precommitment records" and one custodian per kind.** The hot path is host code and no desk. Someone must be custodian of groups, origin counts and the decision record. I propose the Texture desk for the subject index and edits, and the host for derived folds, which hold no canonical kind. This needs a ruling.

### G.2 Where the lead's proposals are weak

- **Polling redundant sources rarely** contradicts the same list's "attention from the many echoes". Echo counts *are* the attention signal. Fetch everything; skip the processing.
- **Echo by vector alone** would have swallowed today's turn (B.2's guard).
- **"Later stories cite it"** as an outcome is self-marking.
- **A new source's first contact** is honest but rare and confounded by that source's beat. Good-Turing over singletons is continuous.
- **Three levels of perplexity** is a good reading and not yet a number.
- **The density lead (AUC 0.72)** had its definition chosen after the data. It is not evidence.

### G.3 Places I am guessing

Every survival share in B.2 except "pooled"; all per-item milliseconds marked so; embedder capacity; the 0.7 exponent; the three-origin line and `z ≥ 3`; the half-lives; the edit bounds; that a local model can give a usable literal perplexity; my 0.6 on the batch arm. The cosine thresholds (0.795, 0.84) were tuned on three days of one week.

### G.4 The assumptions that sink the design, and the cheapest test of each

| Assumption | If wrong | Cheapest test |
|---|---|---|
| Vector groups of arrivals are happenings, well enough to carry attention counts | `p` is noise; the metric and the importance state fall together | hand-read 100 groups at two and three origins; replay 11-23 and check the Huang group forms and bursts. No spend |
| Attention among our panel stands for importance in the world | the paper chases social chatter | one week of the outside-front-pages check: do our top 20 by `p` overlap theirs. About 300 searches |
| Happenings grow slower than sources | model work scales with sources; no scaling law | the subsampled replay. No spend |
| A cheap turn detector exists (burst plus delta terms) | turns stay buried, or every remark fragments | the 20-turn set against the replay. No spend |

One fact outside the design that bears on all of it: the system map records the paper's main model as one vendor session, free until 2026-12-04. Every cost above is priced at the paid rate for that reason.
