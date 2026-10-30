# Autopaper ranks thin stories high: the depth factor is too weak, 2026-10-15

Mutation class: green (documentation only; no code changed). Phase 2
(quality-aware ranking) was started and then cancelled by the owner; see
"Owner direction" at the end.

## Evidence (owner)

Phone screenshots of choir.news, 2026-10-15: "There are some quality issues
with Autopaper, like stub articles with little text and few (or only 1)
sources getting ranked high in verticals and even the front page. Should be
quality aware ranking." Examples:

- "Citizen Lab links targeted online ads to zero-click spyware attacks" (v4,
  Sources 3): one short paragraph plus a quote; the text admits the primary
  Citizen Lab page "could not be verified".
- "Peshawar school collapse toll remains unsettled amid reports of rescue
  fears" (v4 in the screenshot, v5 by measurement time, Sources 5): two short
  paragraphs mostly listing conflicting tolls.

## How ranking works today

The premise "the ranking ignores article quality" is only half right: it
already has a depth term, but a weak one.

1. **Scoring (host, `go-choir-front-importance`).** A systemd timer
   (`nix/node-b.nix`, every ten minutes) runs `corpusd front-importance`
   (`internal/frontimportance/scorer.go`). For each front story without a
   score for its latest revision it asks Jev one question set about the
   title and dek only (not the body): `importance` 1..10 (how important is
   the event as news), `promo` yes/no, `section` (world, us_politics,
   business, tech_ai). It writes `scores.json`. `depth.go` only logs how many
   important stories are thin; nothing reads it back.
2. **Serving (corpusd).** `/api/platform/publications/front` lists stories
   newest first (keyset cursor). `attachFrontImportance`
   (`internal/platform/publication_front_importance.go`) adds `importance`,
   `promo`, `section` from the scores file. `frontDepth`
   (`publication_front_depth.go`) adds `source_count` (distinct source
   entities over the version's anchors, i.e. the reader's "Sources N") and
   `words` (text nodes of the body, title heading included).
   `?section=` filters server side, still newest first. The server does not
   rank.
3. **Ranking (client only, `frontend/src/lib/autopaper.ts`).** The front page
   fetches the newest 60 stories (`FRONT_FIRST_PAGE_SIZE`), a section page
   the newest 60 of that section, and `rankFrontStories` orders only that
   window. When at least half carry an importance:

   `score = importance (unscored = 4) x depthFactor x sectionLift x 0.5^(age/24h)`

   `depthFactor`: 1 source (or 0) = 0.5, 2 sources = 0.75, 3+ = 1.0; and a
   further x0.8 when the body is under 150 words. Promo stories and "Open a
   story on..." titles sort after everything. The lead is the first story
   with at least 2 sources. Older pages ("Older stories") are appended in
   server order, unranked.

   Origin: docs/archive/problems/front-page-leads-with-a-one-source-stub-2026-10-14.md.

### What the score ignores

- Outlet diversity: sources are counted by source entity, so three articles
  from one outlet or a syndication pair count as 3 sources. 7 of the 69
  measured stories have 3+ sources but only 2 outlets (domains), e.g. the
  Guardian plus its Australian partner site.
- Length beyond a single 150-word cliff: 144 words and 1,400 words differ by
  only the x0.8.
- Hedging in the text ("could not be verified", "unverified", "could not be
  traced"). 7 of the top 30 front stories contain such a phrase (crude
  regex, see caveats).
- Primary-source presence, and how many versions have developed the story
  (version count lifts only the recency-only fallback rank, not the
  importance rank).
- Importance is judged from title and dek, never the body, so a good headline
  on a stub scores like a developed story.
- Uncited anchors: `source_count` counts anchors, including an `expanded_ref`
  quote block, not only inline citations. Citizen Lab's Wired source is a
  trailing quote block that no sentence cites.

## Measurement (staging, read-only, 2026-10-15 ~16:05 UTC)

Method: public `GET https://choir.news/api/platform/publications/front`
(public, 200, no auth; `?limit=60` and `?limit=60&section=<id>`), ranked with
the deployed `rankFrontStories` (the repo's `autopaper.ts` run under Node)
over exactly the 60-story window the app fetches. Per story the latest
version's bundle was read from `.../publications/resolve?route=<permalink>`:
words from `body_doc` text nodes (matches the served `words`), distinct
source entities, distinct cited URLs, distinct outlets (host with `www.`/`m.`
stripped, reduced to registrable domain), and a hedge regex over the text.
69 distinct stories (front top 30; top 20 of world, tech_ai, us_politics).
"v" is `latest_revision`, which is what the reader labels v<n>. "imp" is the
Jev importance (- = unscored).

Prevalence of stubs (words under 150, or 2 or fewer outlets):

| list | top N | words<150 | outlets<=2 | either | hedge phrase | in top 5 / top 10: words<150 | in top 5 / top 10: outlets<=2 |
|---|---|---|---|---|---|---|---|
| front | 30 | 14 | 17 | 21 | 7 | 0 / 4 | 0 / 0 |
| world | 20 | 4 | 8 | 9 | 6 | 0 / 0 | 0 / 0 |
| tech_ai | 20 | 6 | 10 | 11 | 7 | 1 / 2 | 0 / 3 |
| us_politics | 20 | 3 | 4 | 6 | 9 | 2 / 2 | 1 / 1 |

Reading: the very top is mostly sound by these two thresholds, but stubs
begin at rank 6-7 of the front page, and by rank 20-30 the page is dominated
by 1-2 outlet stories (17 of 30; 6 of ranks 21-30 cite a single source).
The factor works as a mild sort, not as a quality gate. The cases the owner
named are not caught by either threshold on the metrics: both sit near the
top with 3 and 5 distinct outlets (see below), so source count is not what
separates them from developed stories.

### Front page, top 30 (rank, imp, v, words, sources / outlets, hedge)

| rank | imp | v | words | srcs/outlets | hedge | title |
|---|---|---|---|---|---|---|
| 1 | 8 | v5 | 178 | 5/5 | | Peshawar school collapse toll remains unsettled amid reports of rescue fears |
| 2 | 6 | v7 | 336 | 3/3 | | Comoros-flagged tanker Yara J catches fire in Romania's territorial waters |
| 3 | 6 | v5 | 221 | 3/3 | | Suspicious mail prompts election-office evacuations in three states |
| 4 | 6 | v6 | 212 | 4/4 | not verified | Mexico City's bullfighting ban and the training question |
| 5 | 6 | v5 | 160 | 3/3 | | Jeffrey Donaldson jailed for 15 years after jury conviction |
| 6 | 7 | v3 | 133 | 3/3 | | Harris says Biden administration should have acted sooner to curb Isra... |
| 7 | 7 | v4 | 144 | 3/3 | could not be verified | Citizen Lab links targeted online ads to zero-click spyware attacks |
| 8 | 5 | v11 | 757 | 11/10 | could not be traced | Banks seek to shift AI data-centre debt as Oracle credit reprices |
| 9 | 6 | v5 | 122 | 5/5 | | Russian jet-powered drone strike reported at Odesa supermarket |
| 10 | 6 | v6 | 139 | 3/3 | unverified | DOJ confirms search in Omar investigation; broader allegations remain |
| 11 | 6 | v3 | 139 | 3/2 | | Queensland bill would give minister power to end strikes |
| 12 | 6 | v5 | 396 | 2/2 | could not be verified | Suicide rates among middle-aged men 'at their highest ever', Telegraph |
| 13 | - | v2 | 240 | 3/3 | | FBI probe into Collins and Navatek remained preliminary |
| 14 | 4 | v7 | 187 | 3/3 | unverified | Talarico returns to Texas Senate campaign after flu absence |
| 15 | 5 | v2 | 177 | 2/2 | | French football ethics body urges stadium neutrality |
| 16 | 6 | v3 | 107 | 2/2 | | Delegates walk out during Herzog address at Baku summit |
| 17 | 5 | v3 | 175 | 2/2 | unverified | Beetaloo emissions claim remains separate from Madeleine King's report |
| 18 | 5 | v2 | 214 | 2/2 | | Columbia nuclear expansion remains a plan, not added power |
| 19 | 6 | v3 | 139 | 2/2 | | Social Security's 3.6% raise is real; whether it keeps pace is disputed |
| 20 | 8 | v3 | 145 | 1/1 | | DW reports Bas resignation as German labor minister |
| 21 | 8 | v2 | 8 | 1/1 | | FTC probes OpenAI and Anthropic over AI risks |
| 22 | 3 | v3 | 232 | 3/3 | | C++26 contracts put preconditions and postconditions in focus |
| 23 | 4 | v4 | 216 | 2/2 | | Oracle's $38bn data-center loan nears distribution after months |
| 24 | 3 | v4 | 158 | 3/2 | | Manchester City representative invited on UK trade mission to India |
| 25 | 5 | v6 | 249 | 1/1 | | California's 22nd District race puts healthcare, costs... |
| 26 | 3 | v4 | 84 | 3/2 | | Fortran community marks the language's 70th anniversary |
| 27 | 6 | v3 | 65 | 1/1 | | Questions over reported quarantine at Irkutsk anti-plague institute |
| 28 | 6 | v3 | 125 | 1/1 | | Russia says it arrested Swede near Pskov, alleges Ukraine ties |
| 29 | 6 | v2 | 32 | 1/1 | | ICE resisted automatic body-camera trigger recommendation |
| 30 | 6 | v2 | 14 | 1/1 | | Severe drought declared in six English areas as reservoirs fall |

### Section pages, top 20 (rank, imp, words, srcs/outlets; hedge marked h)

tech_ai: 1 Citizen Lab (7, 144, 3/3, h); 2 Banks/Oracle debt (5, 757, 11/10, h);
3 OpenAI engineer recorder (7, 1426, 11/9); 4 AI models hide capabilities
(5, 435, 6/6); 5 Flock Safety (5, 868, 8/5, h); 6 UK cyber bill (5, 302, 2/2);
**7 FTC probes OpenAI and Anthropic (8, 8 words, 1/1)**; 8 C++26 contracts
(3, 232, 3/3); 9 Oracle loan (4, 216, 2/2); 10 Anthropic revenue (4, 838,
15/12, h); 11 X suspended accounts (6, 82, 2/2); 12 Australia online-safety
(7, 168, 1/1, h); 13 TSMC (3, 271, 6/5, h); 14 Fortran (3, 84, 3/2);
15 Google phone verification (3, 425, 3/2, h); 16 AirTag tool (5, 143, 2/2);
17 Plasma 6.8 (3, 164, 2/1); 18 Double-headed stack allocator (3, 612, 5/3);
19 Capsem (3, 275, 7/5); 20 Simons Foundation (5, 134, 1/1).

world: 1 Peshawar (8, 178, 5/5); 2 Zelenskyy/NPR (8, 787, 16/12); 3 Manchester
City ownership (8, 312, 8/6, h); 4 Yara J (6, 336, 3/3); 5 Albania ministry
fire (8, 637, 12/5); 6 Mexico City bullfighting (6, 212, 4/4, h); 7 Donaldson
(6, 160, 3/3); 8 Starlink limits (6, 631, 8/5); 9 Trump Ukraine push (7, 259,
6/4, h); 10 IBAC (7, 325, 8/5); 11 Odesa drone (6, 122, 5/5); 12 HMAS Leeuwin
(6, 210, 3/2); 13 Queensland (6, 139, 3/2); 14 Suicide rates (6, 396, 2/2, h);
15 Kenya Ebola (6, 230, 4/3, h); **16 India Supreme Court voter deletions
(8, 36 words, 2/1)**; 17 Land Registry (5, 205, 3/2); 18 French football
(5, 177, 2/2); 19 Herzog walkout (6, 107, 2/2); 20 Beetaloo (5, 175, 2/2, h).

us_politics: 1 Suspicious mail (6, 221, 3/3); 2 Harris (7, 133, 3/3); 3 Omar
home search (7, 1668, 29/24, h); 4 Trump leakers (7, 348, 3/2); 5 DOJ Omar
(6, 139, 3/3, h); 6 Democrats' polling (5, 302, 3/3, h); 7 Taiwan LNG (7, 824,
8/7, h); 8 Retired officers (6, 293, 5/5); 9 Talarico (4, 187, 3/3, h);
10 Supreme Court records (7, 663, 10/8); 11 Guilfoyle ousted (7, 338, 5/4, h);
12 Texas ICE officer (7, 510, 5/4); 13 El-Sayed (5, 585, 9/8, h); 14 Guilfoyle
resigns (7, 381, 3/3); 15 Lawler/Conley (5, 645, 8/5); 16 Social Security
(6, 139, 2/2); 17 Michigan/Maine (5, 419, 6/5, h); 18 Press secretary (6, 216,
2/2); 19 CNN/MS Now access (6, 459, 5/4); 20 DOJ California ballots (7, 200,
2/2, h).

### Worked reading of the owner's two examples

- Citizen Lab: front rank 7, tech_ai rank 1. importance 7, 3 sources (so
  depth 1.0), 144 words (x0.8) = 5.6 before recency decay. The 0.8 is the
  only penalty it takes. By the body: one paragraph; the Guardian is cited
  three times (one source entity, which the counter already dedupes), TechCrunch
  once, and the Wired source appears only as a trailing quote block. Both its
  `source_count` of 3 and "3 outlets" are technically true yet the argument
  rests on a single Guardian report, and the text says the primary page could
  not be verified. Neither count nor length detects that.
- Peshawar: front rank 1, world rank 1. importance 8, 5 sources / 5 outlets,
  178 words: depth factor 1.0, no penalty at all. By these metrics it is not
  a stub. Its weakness is editorial (a toll the sources disagree on), which no
  structural count can see.
- Real stubs do rank: FTC probe (8 words, 1 source, importance 8) is tech_ai
  rank 7 and front rank 21; India voter-deletions (36 words, 1 outlet,
  importance 8) is world rank 16. Importance 8 times 0.5 x 0.8 = 3.2 still
  beats a developed importance-3 story (3.0) and ties importance-4 stories.

## Caveats on the measurement

- One snapshot; ranks move as stories are added. The front window is the
  newest 60, so ranking quality is only ever judged among recent stories.
- Hedge counts are a crude regex (`could not be verified`, `unverified`,
  `not verified`, `not confirmed`, ...) and also match legitimate editorial
  hedging in a well-developed story (e.g. the 29-source Omar story). They
  are an indicator, not a defect count.
- Outlet = registrable domain, so `abc.net.au` and `smh.com.au`-style
  partner sites and syndicated copies are mis-merged or mis-split in
  places; `net.au` above is such an artifact.
- An earlier pass counted words as 0 (wrong `body_doc` path); fixed before any
  number in this document was taken.
- `FBI probe ... Collins and Navatek` (rank 13) was unscored at measurement
  time and ranked as importance 4.

## Open questions the measurements raise

1. Should length and outlet count be the thing ranking watches at all, or
   are 14 of 30 front stories under 150 words a sign of what research
   produces (short stories are mostly 1-3 source stories)?
2. The importance scorer sees only title and dek. A strong headline over a
   stub keeps full importance. Is importance meant to describe the event or
   the paper's coverage of it?
3. `source_count` counts anchors including uncited quote blocks and repeated
   outlets; is "Sources N" the number the reader should be shown?
4. Versions: stories at v11-v15 are the deepest, and v2-v3 stories dominate
   the thin tail. Is revision count a usable signal of development, or just
   of churn?
5. A hedged story ("could not be verified") may be the more honest one.
   Should hedging be treated as a defect or as calibration?
6. Section and front ranking only reorders the newest 60. Older pages are
   unranked. Is that acceptable if the paper's output rises?
7. How much of the thin tail comes from provider-degraded research (see the
   Mauritius trace in front-page-leads-with-a-one-source-stub-2026-10-14.md)
   versus the story's real availability of sources?

## Owner direction (2026-10-15)

The owner redirected before any ranking change was built:

- The point is to produce quality, not just rank by it. Reordering thin
  stories hides the problem; it does not make the paper better.
- Source count is a coarse proxy for quality.
- The real goal is a serious current-events world model: a gestalt of what
  is going on, from which both importance and article quality follow. A
  story is important and well developed to the degree the model of the world
  says so, not because a title scores high or three links are attached.
- The next step is broader and deeper source ingestion, not ranking changes.

Consequently Phase 2 (a quality factor, a flag, per-story quality logging, a
branch `feat/quality-aware-ranking`) is not built and no code was changed;
this document is the Phase 1 record. The failure-mode list prepared for
Phase 2 (citation padding, breaking news suppressed, sections emptying out)
is not carried forward. The current `depthFactor` in
`frontend/src/lib/autopaper.ts` stays as it is.
