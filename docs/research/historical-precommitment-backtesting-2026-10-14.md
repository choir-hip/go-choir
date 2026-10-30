# Historical Precommitment Back-Testing: Scoring What Was Said at T From T′

Status 2026-10-30: section 1.8 (Dolt AS OF model) and the Store B lines are stale: Dolt and Store B are deleted; shared claims and scores live in the PostgreSQL graph database. Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

Date: 2026-10-14. Mutation class: **green** (research doc only; no code).
Status: research for the roadmap. The stations named in §6 are proposals
until a throughline goal file authors them. Not legal advice (§5).

Inherits: [claims, events and entities](../reports/claims-events-entities-ontology-2026-10-14.md)
(the ontology, settlement by modality, five uncertainties, box score,
say–do), the [precommitment memo](../precommitment-records.md)
(prequential discipline, the hindsight problem), and
[media and rights](autopaper-media-and-rights-2026-10-14/README.md).
`docs/archive/reports/object-graph-retrieval-research-2026-10-14.md` is not on
main at this commit and is not drawn on.

## The brief (owner, 2026-10-14, about 17:30 UTC)

> Another work stream we should research and add to our roadmap is doing
> the precommitment records on historical data. That's both an eval and a
> product, because we can build a track record and box score from history
> going back — you could take Winston Churchill in 1941 and score him from
> the perspective of 1942. [...] And obviously the more recent is more
> valuable, because then it would apply to people who are actors in the
> world today. It'll probably take quite a bit of iteration to make it
> workable and live, but that becomes very valuable.

## Bottom line

1. **The hard problem is hindsight, and it lives in three places, not
   one.** A model that knows the outcome can leak it into (a) which claims
   get selected, (b) how a claim is read and turned into a checkable
   question, and (c) the verdict. Settlement leakage is the least
   dangerous once settlement is by **count** over dated primary records.
   Selection and operationalization leakage are the dangerous ones, and
   the ontology already has the guard: the system selects, readings and
   criteria are frozen blind to the outcome (§1).
2. **Every historical score carries a leakage class** (L0 to L3, §1.6),
   shown beside the number. A score whose verdict a model could have
   produced from memory is labelled so, and never enters a track record
   as if it were prequential.
3. **Historical claims are a third selection track.** The design's
   `scheduled | targeted` split gains `archival_census`: every document in
   a corpus whose definition was hashed and committed *before* extraction.
   The census is the historical analogue of a scheduled poll (the system,
   not the scorer, chose). It is shown in its own rows of the box score and
   never pooled into the live headline (§4).
4. **The cleanest "said at T, settled at T′" pairs**, best first:
   company quantitative guidance against XBRL actuals; central-bank
   forward guidance and projections against decisions and vintage data;
   professional forecaster surveys; prediction-market questions; party
   and presidential promises with existing ratings; free news archives
   last (§2.3).
5. **Recommended first back-test: the Federal Reserve, 2012–2025, plus
   S&P 100 earnings guidance, 2018–2025** (§3). Both are primary-source,
   precisely dated by a third party (the Fed's release, the SEC's
   acceptance timestamp), free, settle mostly by count, cover actors
   active today, and **keep publishing**, so the same sources become live
   scheduled polls the day the back-test ends. Inference cost is under
   $500 at today's list prices; the real cost is labelling time.
6. **It doubles as E0's missing half.** E0 must measure settlement
   feasibility, but live claims take 30 days or more to settle. Historical
   claims with known outcomes measure extraction precision and recall,
   settlement accuracy and leakage now (§3.4). This is the main reason to
   start the stream early.
7. **Far history (Churchill 1941 from 1942) is feasible as an editorial
   product, not as a track record.** Time-locked models with a 1939 cutoff
   exist (Ranke-4B, announced); Hansard is open; the settlement evidence
   for 1942 is in archives. But every frontier model knows how the war
   ended, so these scores are L2–L3 and labelled "illustrative" (§1.7).
8. **Rights are easier than live news and ethics are harder.** Most of the
   recommended corpora are public domain or openly licensed. The dead
   cannot be defamed in the US and the GDPR does not cover them; living
   actors get the §7.3 excellence bar and a right of reply; private
   individuals are excluded as in D6 (§5).

## 1. The central methodological problem: hindsight leakage

### 1.1 Why it is worse here than in the live system

The precommitment memo's core point is that "a capable model can explain
almost any outcome as unsurprising." In the live system Choir records a
claim before the settling evidence exists, so hindsight cannot reach it. In
a back-test the outcome is already in the world, in Choir's corpus, and in
every model's weights. The research literature names this **lookahead
bias**: pretraining data that post-dates the analysis period leaks into
outputs that should depend only on the past. Sarkar and Vafa found it in
two social-science uses (risk factors from earnings calls; election
winners from candidate biographies) and found that prompting a model to
"pretend it is year X" does not remove it.

Three leak sites, in order of danger:

| Leak site | What goes wrong | Example | Guard (§1.2–1.5) |
|---|---|---|---|
| **Selection** | the extractor or researcher surfaces claims that later turned out memorable (famous misses, famous hits) | "peace for our time" is extracted; a thousand dull, kept commitments are not | census selection: every document of a pre-registered corpus, every claim the extractor finds; no targeted digging |
| **Operationalization** | readings (U2), criteria and the specificity check are written knowing the outcome, so the question bends toward the answer | "transitory" inflation read narrowly because 2022 is known | criteria authored by a time-locked or outcome-masked model, frozen and hashed before any settlement evidence is attached |
| **Verdict** | the judge answers from memory, not from the evidence it was given | the judge "knows" the Fed hiked in March 2022 and ignores ambiguous evidence | settle by count over dated records; blinded judges; counterfactual-evidence tests that measure evidence-following |

### 1.2 Point-in-time evidence only

- Extraction sees only documents witnessed at or before T. Settlement sees
  only documents witnessed at or before T′.
- **Date-restricted web search is not point-in-time.** Paleka et al.
  (ICLR 2026) show search date filters returning misdated pages and
  ranking that favours later-salient results; one query restricted to
  January–September 2024 returned an article discussing the October 2024
  Nobel Peace Prize. Choir must not settle back-tests with live web search.
- Use corpora whose dates are **intrinsic and witnessed by a third party**:
  the SEC acceptance timestamp on an EDGAR filing, the Fed's release date,
  a Hansard sitting date, a Chronicling America issue date, an earliest
  Wayback capture timestamp. Paleka et al. recommend the same: restricted
  corpora with reliable dates, period-specific indexes, and retrievers
  without learned world knowledge (TF-IDF or older embeddings).
- **Vintage data for figures.** Economic figures are revised. ALFRED keeps
  every vintage of 740,000+ FRED series (earliest vintage 1927) precisely
  so that analysis can use "data that were available at the time." A
  claim about inflation settles against the vintage published by T′, and
  the box score can show "settled on first print" beside "settled on
  current revision".

### 1.3 Models with training cutoffs before T

| Model family | Cutoffs | Kind | Use |
|---|---|---|---|
| Ranke-4B (Zurich/Cologne "History LLMs") | 1913, 1929, 1933, 1939, 1946 | 4B generative, Qwen3 architecture, 80B tokens of time-stamped text | far history; listed as upcoming, availability unconfirmed |
| StoriesLM v1/v2 (Sarkar) | yearly, 1900–1963 | encoder-only, trained on American Stories | classification, embeddings, readings ranking; not generation |
| ChronoBERT / ChronoGPT (He, Lv, Manela, Wu) | rolling, up to the 2020s | encoder and small GPT; instruction-tuned follow-up (2510.11677) | modern back-tests; the authors find lookahead bias "modest" in their asset-pricing test |
| DatedGPT (2603.11838) | annual, 2013–2024 | twelve 1.3B models | modern back-tests at yearly granularity |
| Frontier models | 2025–2026 | strong extraction | after their cutoff only, with a buffer |

- These models are small. They are good enough to **freeze readings and
  weights** (U2) and to run the specificity check, which are short typed
  choices, and they can serve as the blind judge in the counterfactual
  test. They are not good enough to replace the extractor for long
  documents.
- **Reported cutoffs are not effective cutoffs.** Cheng et al. ("Dated
  Data", COLM 2024) show effective cutoffs differ from reported ones,
  because old documents reappear in newer crawls and deduplication misses
  near-duplicates. Paleka et al. add that post-training and system prompts
  leak later facts. Rule: treat a frontier model's release date plus a
  buffer as its cutoff, and treat any period before that as known to it.

### 1.4 Blinded settlement and masking

- **Blind judge.** The settling judge sees the frozen criterion and the
  T′-dated evidence only. The outcome label is withheld; the speaker is
  withheld (`saw_speaker = false`, as the ontology already requires); dates
  are shifted or masked where the criterion allows.
- **Masking names and dates** (Glasserman and Lin, 2023): replacing
  company names in headlines changed GPT-3.5's signal, and out-of-sample
  most of the in-sample difference came from knowledge of the future.
  The effect was strongest for large, famous firms. Masking works for
  obscure actors; it cannot hide the Fed, Apple or Churchill, whose
  identity is recoverable from the content.
- **Settle by count wherever possible.** A guidance range against an XBRL
  value, a rate decision against a policy announcement, a release against
  a dated release note: a deterministic predicate does not know the
  future. This is the ontology's settlement order (count, then Jev, then
  research, then panel), and history makes count even more valuable.

### 1.5 Measuring leakage, not assuming it away

| Test | How | What it measures | Bar (proposal) |
|---|---|---|---|
| **Closed-book control** | ask the judge the settlement question with no evidence | how much the judge already knows | reported per corpus; high knowledge is expected for the Fed |
| **Counterfactual evidence swap** | for a sample, replace the T′ evidence with a plausible document that implies the opposite outcome; the judge should follow the evidence | evidence-following rate | ≥ 0.95, or the judge is not used for that corpus |
| **Masked vs unmasked divergence** | settle the same item with names and dates masked and unmasked | identity-driven leakage | divergence ≤ 5% |
| **Time-locked agreement** | compare U2 weights and specificity verdicts from a frontier model with a time-locked model's | operationalization leakage | disagreements flagged; time-locked governs where they differ |
| **Logical leakage audit** | Paleka et al.: questions whose framing or resolution date reveals the answer (≥ 10% of one prior benchmark's items) | criteria that encode the outcome | criteria with a resolution date chosen after T are rejected |
| **Selection audit** | compare the census extraction rate of hits and misses with a random sample re-extracted by a time-locked model | whether famous outcomes are over-extracted | difference within the sampling interval |

### 1.6 Leakage classes, reported on every score

| Class | Settlement | Operationalization | Shown as |
|---|---|---|---|
| **L0** | count over T′-dated primary records | frozen by a time-locked model or a fixed template (guidance ranges) | full historical record |
| **L1** | blind judge that passed the counterfactual swap | time-locked or masked | historical record, "model-judged" |
| **L2** | frontier judge, masked; leakage measured but not ruled out | frontier, masked | historical record, "leakage risk measured: x%" |
| **L3** | frontier judge, unmasked famous actor | frontier | **illustrative**, never a track record |

A box-score cell aggregates only one class at a time, or states the mix.
The settlement record already carries scorer, method, `saw_speaker` and
p; it adds `leakage_class` and the evidence cut-off `as_of_T′`.

### 1.7 How far back each approach works

| Era | Point-in-time evidence | Time-locked model | Masking | Count settlement | Best class reachable |
|---|---|---|---|---|---|
| before 1913 | Chronicling America, Hansard (1803–), American Stories | Ranke-1913, StoriesLM | weak (OCR, famous names) | rare | L2 |
| 1914–1963 (Churchill) | same, plus British archives (paid) | Ranke 1929/33/39/46, StoriesLM | useless for famous actors | some (votes, enactments, battles as dated events) | L2–L3 |
| 1964–1993 | Congressional Record, presidential papers, GDELT 1.0 events (1979–), Greenbook (1966–), SPF | none open | partial | macro figures via ALFRED | L1–L2 |
| 1994–2012 | EDGAR (1994–), FOMC statements (1994–), Wayback, Common Crawl (2008–) | ChronoGPT early checkpoints | partial | good | L0–L1 |
| 2013–2024 | everything above, GDELT 2.0 (2015–) | DatedGPT, ChronoGPT | good for small actors | good | **L0–L1** |
| 2025–now | same | none yet; frontier models' cutoffs are near | good | good | L0 for count; frontier judges are nearly prequential for the most recent months |

The owner's instinct that the recent past is most valuable is also the
methodological answer: the recent past is where L0 is reachable.

### 1.8 The Dolt "as of" model: two times, not one

Store B is Dolt, and `AS OF` gives transaction time: what Choir's store
held at a commit. A back-test ingested in 2026 has transaction times in
2026 regardless of when the speech was given. So historical work is
**bitemporal**:

- **valid time**: `uttered_at`, `published_at`, and a new `witnessed_at`
  (§4.1), all in the world's past;
- **transaction time**: the Dolt commit when Choir ingested, extracted or
  settled, which gives reproducibility ("this back-test as computed on
  2026-11-02").

"What we knew at T" is a query over valid time (`witnessed_at ≤ T`);
"what Choir's back-test said last month" is `AS OF` over transaction time.
Both are needed; neither replaces the other.

## 2. Data sources by era and type

### 2.1 News and the open web

| Source | Coverage | Access | Licence / terms | Cost | Notes |
|---|---|---|---|---|---|
| Chronicling America (LoC) | US papers, about 1770s–1963 | loc.gov JSON API, bulk OCR | public domain, "no known copyright restrictions" | free | OCR noise lowers U1 |
| American Stories (Dell et al.) | ~20M Chronicling America scans, article-level | Hugging Face dataset | CC-BY | free | better layout and OCR than raw LoC |
| Internet Archive Wayback | 1996– | CDX API, YYYYMMDDHHMMSS capture times | IA terms; content keeps its own copyright | free, polite rate | earliest capture is a witness time |
| Common Crawl | 2008– (dense from ~2013) | S3, CDXJ index | CC terms; content keeps its copyright | free (egress) | old pages reappear in later crawls: date by first sighting |
| GDELT 1.0 / 2.0 | events 1979–; 2.0 with URLs from Feb 2015, 15-min | BigQuery, files | free for any use with citation | free | an action-event index for commitments; noisy coding |
| NYT Archive API, Guardian Open Platform | 1851– (metadata); 1999– (Guardian) | key-based APIs | publisher terms, commercial use restricted | free tier | metadata and links; bodies need licence |
| ProQuest Historical, British Newspaper Archive | 18th c.– | subscription | licensed, no bulk ML by default | paid | needed for UK 1941–42 press |
| Factiva, LexisNexis | 1980s– | enterprise | Factiva advertises 8,000+ sources licensed for GenAI use (Jan 2026) | paid, quote-based | the licensed route if free corpora fall short |

### 2.2 Primary records (the settle-able core)

| Source | Coverage | Access | Licence | Settle-ability |
|---|---|---|---|---|
| UK Hansard | 1803– | historic Hansard archive; current Hansard | Open Parliament Licence | statements dated to the sitting; votes and enactments are action events |
| US Congressional Record | bound 1873–; daily 1994– | govinfo, congress.gov | US government work, public domain | votes and bills enacted |
| American Presidency Project / govinfo presidential documents | 1789– | web | public domain texts | executive orders, signings: count-settleable actions |
| FOMC statements, minutes, press conferences, SEP | statements 1994–; pressers 2011–; dot plot 2012–; SEP quarterly since 2007 | federalreserve.gov | public domain | **excellent**: explicit guidance, dated decisions, numeric projections |
| FOMC transcripts, Tealbook/Greenbook | 5-year lag; Greenbook PDFs 1966– | federalreserve.gov, Philadelphia Fed | public domain | staff forecasts with known outcomes; individual SEP projections by participant code after 5 years, by name after 10 (2016 Secretariat memo; verify current practice) |
| ECB, BoJ | ECB guidance since July 2013; BoJ Summary of Opinions since 2016 | central bank sites | reproduction with acknowledgement (check each) | excellent, multilingual |
| ALFRED | vintages since 2006 archive, series back to 1927 | API | St. Louis Fed terms | the T′ figure as then known |
| SEC EDGAR | 1994– | APIs, 10 req/s, declared User-Agent; XBRL company facts | public filings | **excellent**: 8-K Item 2.02 guidance vs later XBRL actuals |
| I/B/E/S Guidance (LSEG via WRDS) | 8,200+ companies, 13 quantitative items | WRDS subscription | licensed, research | a labelled gold set for guidance extraction |
| Earnings-call transcripts | 2000s– | vendors (S&P, FactSet, Seeking Alpha); some company sites | vendor copyright | rich, but bulk use needs a licence |

### 2.3 Forecasts and promises with known outcomes

| Source | Size | Use | Licence |
|---|---|---|---|
| Philadelphia Fed SPF | quarterly since 1968, anonymous IDs | calibration machinery, forecaster box scores by ID | free |
| Metaculus, Good Judgment Open, Manifold, Polymarket | tens of thousands of resolved questions | test the scoring math (Brier, Murphy decomposition, reference forecasts); **no extraction step** | site terms; Metaculus API public, bulk dumps not official |
| PolitiFact promise trackers | Obama 533 (255 kept, 146 compromise, 125 broken); Trump 2017–20 102; Biden 99 (33/32/34) | gold labels for commitment settlement; agreement with an external scorer | PolitiFact copyright; use ratings as labels, never republish |
| Comparative Party Pledges Project | 20,000+ pledges, 57 campaigns, 12 countries (AJPS 2017) | the academic standard for "pledge specific enough to judge"; a specificity-check benchmark | academic; request from authors |
| Manifesto Project | party manifestos, 50+ countries | source text for pledges | research licence |
| Wikipedia revisions; Wikidata | 2001–; 2012– | revision history dates when a fact entered the public record; Wikidata gives entity identity | CC BY-SA; CC0 |

### 2.4 Which give the cleanest pairs

| Rank | Pair | Why clean | Ambiguity left |
|---|---|---|---|
| 1 | company quantitative guidance → XBRL actual | numeric range, fiscal period named, same issuer reports the outcome, SEC timestamps both | non-GAAP items, mid-quarter revisions (an `updates` edge), fiscal vs calendar quarter (U2) |
| 2 | FOMC SEP and forward guidance → decisions and vintage data | dated, numeric, primary, public domain | conditional guidance ("if the outlook...") needs readings; medians vs individuals |
| 3 | SPF / Tealbook → ALFRED | explicit numeric forecasts | not public actors (IDs, staff) |
| 4 | prediction-market questions → resolution | already operationalized | no extraction; tests only the scoring layer |
| 5 | campaign promises → enacted actions | external labels exist | vague promises, "blocked by Congress" (PolitiFact rates outcomes, not effort) |
| 6 | statements in news archives → later events | broadest | attribution, OCR, interpretation, selection |

## 3. Recommended first back-test

### 3.1 Choice

**Two corpora, one pipeline:**

- **H-Fed: the Federal Reserve, January 2012 – December 2025.** Every
  FOMC statement, minutes, press-conference transcript and SEP, plus Board
  governors' speeches. Actors: the Committee (an institution) and
  individual participants in public roles, many still active.
- **H-Guide: S&P 100 earnings guidance, FY2018 – FY2025.** Every 8-K
  Item 2.02 earnings release exhibit, settled against later XBRL company
  facts and, for non-GAAP items, the next release's reported figure.
  Actors: companies (organizations; `actor_scope: organization`).

Why these two, beyond §2.4:
- **They become live with no new connector.** The Fed site and EDGAR are
  pollable; the moment H0 ends, new statements and 8-Ks are scheduled
  captures and enter the live track. The back-test seeds exactly the
  records that continue.
- They exercise all the settlement modalities: forecasts (SEP, guidance
  ranges), commitments ("we will complete the buyback by Q4"), intents
  ("data-dependent"), and assertions (reported figures).
- They are the B2B data-product line's natural first customers' interest
  (funds and analysts; strategy takeaways §2).
- **H-Promise** (PolitiFact's 734 rated promises) runs as a small
  evaluation set only: it tests commitment settlement against an external
  scorer but is not the product.

### 3.2 Size (estimates, to be measured)

| Corpus | Documents | Claims (est.) | Checkable (est.) |
|---|---|---|---|
| FOMC statements, 2012–2025 | ~112 | ~600 | ~300 |
| Minutes | ~112 | ~3,000 | ~800 |
| Press conferences (2012–, every meeting since 2019) | ~85 | ~4,000 | ~1,000 |
| SEP releases (medians; participant dots by code after 5 years) | ~56 | ~2,500 numeric | ~2,500 |
| Board speeches | ~1,000 | ~8,000 | ~1,500 |
| S&P 100 earnings releases, FY2018–25 | ~3,200 | ~15,000 guidance items | ~12,000 |
| PolitiFact promises (eval only) | 734 promises | — | 734 labels |

### 3.3 Cost at today's prices (`internal/llmcost/pricing.go`)

- Extraction with deepseek-v4.1-flash ($0.30 in / $1.20 out per million):
  ~4,500 documents × ~10k tokens in and ~1.5k out ≈ 45M in, 7M out ≈
  **$22**. Long transcripts chunked; double it for re-runs: ~$50.
- Jev resolution and blind settlement at $0.042 per million input: ~40k
  claims × ~15 questions × 400 tokens ≈ 240M tokens ≈ **$10**.
- Counterfactual swaps and time-locked checks on a 10% sample with
  deepseek-v4-pro ($1.32 / $3.96): ~**$40**.
- Panel on disagreements and a gold sample: ~**$100–300**.
- **Total under $500.** The binding cost is labelling: ~300 guidance
  releases and all 112 statements hand-labelled for extraction recall
  (about 40 hours), unless I/B/E/S Guidance via WRDS is licensed as the
  guidance gold set.

### 3.4 How it doubles as the pipeline's evaluation

| Measure | Gold | Bar (proposal, fixed before the run) |
|---|---|---|
| Extraction recall of checkable claims | hand-labelled statements and releases (or I/B/E/S Guidance) | ≥ 0.85 |
| Extraction precision (speaker, modality, quantity, period) | same | ≥ 0.9 |
| Specificity-check agreement | CPPP's "specific enough to judge" definition on a sample | κ ≥ 0.6 |
| Settlement accuracy, count | XBRL and decision records (exact) | ≥ 0.98 (failures are parsing bugs) |
| Settlement accuracy, judged | PolitiFact ratings; panel gold | agreement ≥ 0.8 on kept/broken |
| Evidence-following (counterfactual swap) | constructed | ≥ 0.95 |
| Leakage divergence (masked vs unmasked) | — | ≤ 5% |
| Box-score stability | split-half by year | intervals overlap |

These are E0's settlement-feasibility measures, delivered without waiting
for live claims to mature. CE-1's falsifier ("settlement by count covers
under 20% of assertions") gets its first real number here.

## 4. How historical records fit the live system

### 4.1 Provenance marking

- **Selection gains a third value:** `selection: scheduled | targeted |
  archival_census`.
  - `archival_census` requires a **corpus manifest**: a definition (source,
    document types, date window, inclusion rule), the list of document
    hashes, and the manifest's own hash, committed to Store B before any
    extraction runs. Adding a document after extraction means a new
    manifest version, never a silent addition.
  - Targeted remains never-scored. Census claims are scored, but only in
    historical rows.
- **Three times plus a witness.** `uttered_at` and `published_at` keep
  their meaning; `fetched_at` stays Choir's ingest time (2026) and so
  stops being the commit time. A new `witnessed_at` is the earliest
  independent dated witness, with `witness_kind` (`sec_acceptance`,
  `issuer_release`, `gazette`, `hansard_sitting`, `newspaper_issue`,
  `wayback_capture`, `commoncrawl_first_seen`). For historical claims,
  **T_commit = witnessed_at**, and the 48-hour freshness rule becomes
  `witnessed_at − published_at` under the bound.
- **Settlement cut-off.** Every historical settlement records `as_of_T′`
  (the evidence horizon), `leakage_class` (§1.6), and the vintage used for
  figures.
- These are fields on existing kinds (extraction, settlement), not new
  kinds. The census manifest is the one new object; it can be a
  `choir.capture_extraction` batch header or a small manifest row (D3's
  rule: a kind exists only when something consumes it).

### 4.2 Storage

- Historical captures go to Store B like any capture, content-addressed;
  bodies follow the rights rule (public-domain bodies may be stored in
  full; licensed bodies follow their licence; `bounded_metadata`
  otherwise).
- Extraction, judgments and settlements use the E1–E2 kinds unchanged;
  no historical fork of the schema.
- Ingest is an explicit owner act per manifest (a backfill), never a boot
  sweep, per the restart rule.

### 4.3 Box-score presentation

```text
Federal Reserve FOMC (wd:Q… · institution · us)        scorer policy: paper default
LIVE      Oct 2026 –           claims 41 · checkable 22 · settled 6        n too small
HISTORICAL (archival census, manifest fomc-2012-2025@v1, L0 74% · L1 21% · L2 5%)
  Forward guidance  commitments 214 · kept 151 · partial 31 · broken 18 · overtaken 14   say–do 0.79 [0.73–0.84]
  SEP median        forecasts 1,120 · Brier 0.21 vs base 0.25 · first print vs revised: Δ 0.01
  Verbal calibration "likely" → 0.71 (n 40)
  Leakage           closed-book 0.88 · evidence-following 0.97 · masked divergence n/a (identity unmaskable)
```

The numbers are invented to show the shape.

- Historical and live rows are **never summed**. A combined view exists
  only on request and states the mix.
- The leakage line is part of the cell, not a footnote.
- L3 items appear only in an "illustrative history" panel, never in a
  record.
- **The bridge test.** At the first live settlement, Choir precommits:
  "this actor's historical say–do predicts its live say–do within the
  interval." That is itself a scored precommitment, and it is the honest
  test of whether historical seeding means anything. If historical scores
  do not predict live ones, the seeding is shown as such.

### 4.4 Choir's own agents

The say–do eval of Choir's desks (E6a) has no history problem: the trace
is complete and timestamped. Historical back-tests do add one thing
there: running today's extractor and settlement over 2012–2025 corpora is
a precommitment of Choir's *pipeline* (its predicted extraction and
settlement), scored against gold labels. That is the eval half of the
owner's "both an eval and a product".

## 5. Rights and ethics

### 5.1 Archives and copyright

| Corpus | Status | Rule |
|---|---|---|
| US federal (Fed, Congressional Record, presidential documents, govinfo) | public domain (17 USC 105) | full text may be stored and quoted |
| Chronicling America, American Stories | public domain; American Stories CC-BY | store; attribute the dataset |
| UK Hansard | Open Parliament Licence | store and quote with attribution |
| Churchill's own words | his literary estate holds copyright (licensed by Curtis Brown); UK term runs to 2035 | Hansard text under the OPL; outside Hansard, short quotation for reporting and criticism only |
| EDGAR filings | public filings; issuer may hold copyright in prose | store for analysis; quote short spans |
| Vendor transcripts, ProQuest, Factiva, LexisNexis | licensed | only under a licence that permits ML use; Factiva sells one |
| Wayback, Common Crawl | archive terms; content keeps its copyright | analysis copy; quote briefly; the rights doc's learning-vs-working-from line (§1.1) applies unchanged |
| PolitiFact, CPPP | copyrighted editorial judgments / academic data | use ratings as evaluation labels; never republish them as Choir's |

The rights doc's line holds: reading for analysis is learning; publishing
a short attributed quote and a settlement is reporting; reproducing whole
archives is not.

### 5.2 Scoring the dead and the living

- **The dead.** In the US a dead person cannot be defamed (Restatement
  (Second) of Torts §560; *Flynn v. Higham*), and the GDPR does not cover
  deceased persons (Recital 27), though France, Italy and Spain have
  posthumous rules. The risk with Churchill is not legal; it is
  scholarly: a wrong settlement is a public error about history. L3
  labelling and showing readings guard it.
- **The living.** Everything in ontology §7.3 applies unchanged: verified
  verbatim span, all readings shown, dated evidence linked, uncertainty
  shown, right of reply that reopens settlement, corrections as prominent
  as the settlement. The owner accepted defamation risk with the
  excellence standard; historical scores of living actors are published
  only at L0–L1.
- **Private individuals.** Excluded, as D6 recommends. Historical corpora
  are full of private people (letters to the editor, witnesses, people
  named in old local papers); `actor_scope` defaults to `private` and the
  census never scores them. GDPR profiling of living public figures is the
  open D6 question; historical data does not change it.

### 5.3 Fairness

- **Era norms.** Say–do is procedural, not normative (ontology §7.1). A
  1941 promise is judged against what was promised, not against today's
  values. The box score never mixes eras into one number.
- **Era base rates.** Forecast skill is measured against the base rate
  and reference forecasts *of the time* (SPF, contemporaneous markets),
  never today's.
- **Archive coverage bias.** Digitized archives over-represent English,
  the US and the UK, and national institutions over regional ones.
  Coverage is shown per actor; an actor whose actions are poorly archived
  reads "not observable", never "broken".
- **Translation.** Original text is stored; the English gloss is a
  reading; cross-language settlements are panel-sampled (as live).
- **Survivorship.** Censuses include companies that later failed and
  officials who left office; a census of today's S&P 100 alone would be
  biased. Manifests use point-in-time index membership.

## 6. Staged plan and roadmap placement

| Stage | What | Class | Depends on | Acceptance (bars fixed before the run) | Time |
|---|---|---|---|---|---|
| **H0** | Offline study beside E0: manifests for H-Fed and H-Guide; gold labels; extraction with the E0 prompt; count settlement; blind judge; leakage tests | green/yellow | nothing in the runtime; runs with E0 | §3.4 bars; leakage classes measured per corpus; cost per 1,000 claims reported. **Exit:** if count covers under 20% of checkable claims or evidence-following < 0.95, narrow to L0-only corpora | 2–3 weeks |
| **H1** | Historical ingest into Store B: census manifests, `witnessed_at`, `selection: archival_census`, through the E1 typed routes | orange (new fields; corpusd write route is E1's) | E1 kinds and routes | manifests reproducible; every claim carries witness and leakage fields; no live-path regression | 1–2 weeks after E1 |
| **H2** | Historical settlement and internal box scores, separate rows | orange | E2 events; H1 | intervals reported; split-half stability; scorer agreement ≥ 0.8 | 2 weeks |
| **H3** | Bridge: H-Fed and H-Guide sources polled live; bridge precommitment registered | orange | live paper polling | live claims flow from the same sources; the bridge test is registered with a settlement date | at E6 |
| **H4** | Publish historical box scores for institutions and companies (L0–L1) | red (publication) | E7's gates: D1, D6 ratified | excellence bar on every named settlement; leakage line on every cell | with E7 |
| **H5** | Widen: ECB and BoJ (multilingual), political promises (CPPP, PolitiFact as labels), then far history as an editorial "history desk" with time-locked models (Ranke if released) | orange/red | H4 | per corpus, the §3.4 bars; far history published only as illustrative | later |

**What "workable" means:** a back-test that any agent can rerun from a
manifest hash and get the same claims, settlements and intervals, with
the leakage class on every score and the §3.4 bars met.

**What "live" means:** the same actors' new statements arrive as
scheduled captures and enter the live rows; the historical rows update
only when a settlement is revised (new evidence, a new vintage); the
bridge test is running.

**Roadmap position.** H0 is green and offline, so it can start now beside
E0 and E6a, and it supplies E0's settlement-feasibility numbers early.
H1–H2 wait for the claims layer (E1 kinds, E2 events). The owner expects
about a month of live operation before track records mean anything; H3
uses that month, and the historical rows mean the data-product line opens
with years of record instead of weeks. Security, red-teaming and the
self-development gate keep their place ahead of this in the build order
(strategy takeaways, "Roadmap consequences").

## 7. Open decisions for the owner

1. **HD1.** Add `selection: archival_census` with pre-registered
   manifests, scored only in historical rows. Recommended: yes.
2. **HD2.** Publish historical scores of living named actors only at
   L0–L1; L2 internal; L3 illustrative only. Recommended: yes.
3. **HD3.** First corpora: H-Fed and H-Guide, with PolitiFact as an
   evaluation set. Recommended: yes.
4. **HD4.** Paid data: a WRDS/I/B/E/S Guidance licence as the guidance
   gold set (saves ~30 hours of labelling); Factiva only if H5 needs
   licensed news. Recommended: defer until H0 shows hand labels are the
   bottleneck.
5. **HD5.** Far history (Churchill) as an editorial product with L3
   labels. Recommended: after H4, not before; it is a showcase, not a
   record.

## 8. Conjecture and heresy deltas

- **Conjecture (proposed), HB-1:** historical census back-tests over
  primary-source corpora reach L0–L1 for ≥ 70% of checkable claims after
  2012, and an actor's historical say–do predicts its live say–do within
  the reported interval. Falsified by H0 (L0–L1 share) and by the H3
  bridge test.
- **Heresy delta:** discovered: `fetched_at` cannot serve as T_commit for
  historical claims (the ontology's single commit time assumes live
  capture); repaired in design only by `witnessed_at`. Introduced: none.
  Repaired: none (docs only).
- **Mutation class:** green.

## Sources

Leakage and time-locked models:
- Sarkar and Vafa, "Lookahead Bias in Pretrained Language Models": https://conference.nber.org/conf_papers/f205530/f205530.pdf ; summary https://www.cxoadvisory.com/investing-expertise/lookahead-bias-in-large-language-model-training-data
- Glasserman and Lin, "Assessing Look-Ahead Bias in Stock Return Predictions Generated by GPT Sentiment Analysis": https://arxiv.org/abs/2309.17322
- Paleka, Goel, Geiping, Tramèr, "Pitfalls in Evaluating Language Model Forecasters" (ICLR 2026): https://arxiv.org/html/2506.00723v1
- Cheng et al., "Dated Data: Tracing Knowledge Cutoffs in Large Language Models" (COLM 2024): https://arxiv.org/abs/2403.12958 ; https://allenai.org/blog/knowledge-cutoffs
- Karger et al., "ForecastBench" (ICLR 2025): https://arxiv.org/html/2409.19839v2
- He, Lv, Manela, Wu, "Chronologically Consistent Large Language Models": https://arxiv.org/html/2502.21206v2 ; follow-up https://arxiv.org/html/2510.11677v1 ; model card https://huggingface.co/manelalab/chrono-gpt-v1-20111231
- DatedGPT: https://arxiv.org/html/2603.11838v1
- StoriesLM: https://huggingface.co/StoriesLM/StoriesLM-v1-1944 ; https://huggingface.co/suproteem/StoriesLM-v2-1938
- History LLMs (Ranke-4B): https://www.sourcepulse.org/projects/20984392 ; https://feedbagel.com/post/history-llms-project-developing-large-language-models-for-historical-analysis-wi

Data sources:
- Chronicling America: https://en.wikipedia.org/wiki/Chronicling_America ; https://blogs.loc.gov/thesignal/2022/07/new-interactive-map-and-timeline-added-to-chronicling-america/ ; https://www.neh.gov/news/expanding-our-current-scope-ndnp
- American Stories: https://arxiv.org/abs/2308.12477 ; https://github.com/dell-research-harvard/AmericanStories
- GDELT: https://datahub.aalto.fi/en/data-sources/the-gdelt-database ; https://jwatte.com/blog/gdelt-open-data-kalev-leetaru/
- Common Crawl: https://arxiv.org/html/2404.09770v1 ; https://commoncrawl.org/examples
- Wayback CDX: https://wayback.readthedocs.io/en/v0.3.0/usage.html ; https://nbviewer.jupyter.org/github/GLAM-Workbench/web-archives/blob/0.1.1/find_all_captures.ipynb
- Factiva GenAI licensing: https://www.businesswire.com/news/home/20260120489743/en/Dow-Jones-Factiva-Surpasses-8000-Licensed-Sources-for-GenAI-Use
- Guardian and NYT APIs: https://techcrunch.com/2009/03/10/the-guardian-launches-open-api-for-all-content-but-they-still-control-the-ads
- UK Hansard, Open Parliament Licence: https://certificates.theodi.org/it/datasets/214233
- Congressional Record: https://en.wikipedia.org/wiki/Congressional_Record ; https://wisblawg.law.wisc.edu/2023/05/16/complete-congressional-record-back-to-1873-now-searchable-in-congress-gov/
- FOMC SEP and transcript lags: https://WWW.FEDERALRESERVE.GOV/monetarypolicy/files/FOMC20160111memo01.pdf ; https://www.atlantafed.org/economy-matters/economic-research/2019/12/05/why-the-big-fuss-about-little-dots
- Tealbook/Greenbook: https://www.philadelphiafed.org/surveys-and-data/real-time-data-research/greenbook
- ALFRED: https://alfred.stlouisfed.org/help ; https://fredblog.stlouisfed.org/2021/04/alfred-at-15-archiving-fred-data-since-2006/
- ECB forward guidance (July 2013): https://www.bis.org/review/r131212a.htm ; https://bofbulletin.fi/en/2017/4/are-market-expectations-in-line-with-the-forward-guidance-of-the-ecb
- BoJ minutes and Summary of Opinions: https://www.boj.or.jp/en/mopo/mpmsche_minu/minu_2016/index.htm
- SEC EDGAR access: https://www.sec.gov/os/accessing-edgar-data
- I/B/E/S Guidance: https://www.dig-in.com/news/wharton-research-data-services-adds-thomson-reuters-i-b-e-s-guidance-data ; https://wrds-www.wharton.upenn.edu/pages/grid-items/overview-ibes-demo/
- PolitiFact: https://politifact.com/truth-o-meter/promises/obameter/ ; https://www.politifact.com/article/2025/jan/14/did-joe-biden-keep-these-99-promises-we-reviewed/
- Comparative Party Pledges Project: https://ajps.org/2017/06/07/the-fulfillment-of-parties-election-pledges-a-comparative-study-on-the-impact-of-power-sharing/
- Metaculus API: https://metaculus.com/notebooks/28595/updates-to-metaculus-api

Rights:
- Churchill copyright: https://winstonchurchill.org/resources/reference/the-churchill-copyright/ ; https://richardlangworth.com/the-churchill-copyright
- Defamation of the deceased: https://www.recordinglaw.com/us-laws/defamation-laws/defamation-of-a-deceased-person/ ; https://www.uvm.edu/d10-files/documents/2025-01/Defamation-of-Deceased.pdf
- GDPR Recital 27: https://activemind.legal/legislation/gdpr/recital-27

Unverified in this pass (check before relying on them): the exact
current FOMC transcript and named-projection release schedule; Ranke-4B
weight availability; Common Crawl's current terms text; Metaculus and
Manifold bulk-data terms; LexisNexis ML terms; the size estimates in §3.2.
