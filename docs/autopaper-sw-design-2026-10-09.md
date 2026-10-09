# Autopaper design (station SW) — draft v2 for owner review

Date: 2026-10-09. Status: **rough draft, co-designed with the owner;
decisions marked OPEN are the owner's.** Station: `SW-world-wire-interface`
in the [app-dev metamission](definitions/choir-supervised-app-development-metamission-2026-10-01.md).
Gate 3 (Autopaper) ships in October, after Gate 1 (today) and Gate 2
(tomorrow).

Inputs:
- *Choir — Canonical Project Context* (2026-10-08);
- [Prospective Self-Alignment](prospective-self-alignment-2026-10-09.md)
  (2026-10-09 revision);
- [10-05 re-architecture](world-wire-rearchitecture-2026-10-05.md);
- [Store B inventory](evidence/world-wire-store-b-inventory-2026-10-09.md);
- [precommitment records memo](<Precommitment Records — Engineering Memo.md>);
- the [Jev supervision orientation](orientation-jev-supervision-metamission-2026-09-29.md).

v1 of this draft (git history) is superseded. Owner brief for v2,
2026-10-09:

> pull content (rss, telegram/mtproto, atproto, gdelt, other osint,
> eventually private data feeds) as much and as fast as we can process it
> [...] stick to our four core desks, think about how we incorporate
> precommitment records, and how we use inference efficiently since we are
> budget limited and want to cover key stories globally on ai & tech,
> geopolitics, supply chains, science, health and wellness, business and
> finance. once we get a general system going, then i want to drill in. for
> example, taiwan [...] then broader asian languages social media. big
> value in multilingual and global coverage.
>
> we ingest and store the content in the OG, then we transclude it into
> textures.. and transclude textures into textures. [...] researcher plays
> a role in doing web search (not too much, rate limits and cost).
> basically, we need to categorize and verify everything. and the more data
> we collect and classify and organize, the more we precommit and learn
> from what happens, the more accurately we can cover multiplicity of
> perspectives and discrepancies of reports. thats the real alpha. we dont
> adjudicate just show what the world indicates

## 1. Principles

1. **Show what the world indicates; do not adjudicate.** The unit of
   record is the *report*: who said what, when, where and in which
   language. Articles present agreement, divergence and open questions
   with attribution. "Verify" means verifying provenance and attribution
   (did this source say this, where did it first appear), not issuing truth
   verdicts.
2. **Four desks only.** Deterministic work (fetch, parse, dedup, embed,
   cluster) runs as desk tool modules. There are no separate actors and no
   host daemons.
3. **Ingest everything cheaply; spend inference on stories, not items.**
   Model cost must scale with the number of developing stories, not the
   number of reports (§6).
4. **Everything lands in the object graph, classified.** Texture
   transcludes graph objects into articles, and articles into articles.
5. **Precommit wherever an outcome will arrive anyway.** Most forecasts
   resolve from data the paper is already collecting, so learning costs
   almost no extra inference (§5).
6. **Budget is the only ceiling.** Management sets the spend. Ingest rate
   follows processing capacity, and unprocessed reports are kept as data,
   never silently dropped.

## 2. Two levels of storage

- **The paper's computer** (embedded Dolt plus content storage) holds
  working state: reports, classifications, stories, claims, records and
  Texture documents.
- **The host-level store** (the corpus Dolt behind corpusd) holds only what
  is published. Every computer publishes into it, and every reader reads
  from it.

The old store grew to 104 G because high-volume ingest was committed into
Dolt history (inventory doc). So in the paper's computer:

- report bodies live in content-addressed storage, not Dolt rows;
- high-volume tables are either kept out of version history (to be
  verified: Dolt's ignored-table mechanism) or committed on a slow, bounded
  cadence;
- claims, stories, records and articles are versioned.

## 3. The pipeline

Each stage names its desk, its cost tier and what it writes to the graph.

### S0 Observe (research tool modules; deterministic; no model)

- **Adapters:**
  - existing in `internal/sources`: RSS, Telegram (web preview), GDELT,
    Polymarket;
  - new: AT Protocol (a filtered firehose from curated lists and
    keywords), Telegram over MTProto (more channels, history and media),
    other open-source intelligence feeds, and private feeds later.
  - Engineering writes and repairs adapters in capsules.
- **Per report:**
  - normalize into a `report` object, with the body in content storage;
  - detect the language;
  - drop exact duplicates by hash and mark near-duplicates
    (simhash/minhash: wire rewrites and reposts are data about spread, not
    noise);
  - compute a multilingual embedding;
  - extract entities with cheap deterministic methods (gazetteers, links,
    handles).
- **Adaptive polling:** sources that report early or uniquely are polled
  more often, and dead or redundant sources less. The cadence is a
  management setting, held as durable timer obligations (paused after a
  restart, per the no-auto-resume rule).

### S1 Classify (decision model, batched; Jev by default)

For each batch of new reports, Jev answers typed questions, each with
probabilities:

- vertical(s): AI and tech, geopolitics, supply chains, science, health
  and wellness, business and finance, other;
- region(s) concerned;
- report type: first-hand, official statement, wire rewrite, analysis,
  opinion, rumor or unverified, market signal;
- salience and novelty against the story it would join.

Escalation and audits:
- Low confidence escalates to a small LLM.
- A random sample is audited by a larger model, and the audit resolves
  the decision model's answers (§5).
- Escalation thresholds move with measured calibration.

### S2 Stories (deterministic first, model on the edges)

- Reports join **stories** by embedding similarity across languages plus
  entity and time overlap. A model is asked only for ambiguous merges and
  splits.
- A story moves through states: emerging, developing, stable, dormant.
- A story's version history is its timeline.

### S3 Claims and discrepancies (research; mid-size model; per story window)

- Research reads a **representative set** per story window, never every
  report: the earliest report, each new language, each new region, each
  new source type, and anything flagged as novel.
- It extracts normalized `claim` objects, capped per story window to
  prevent graph explosion. Each report links to claims with a **stance**:
  affirms, denies, attributes ("X says"), hedges or questions.
- Where stances conflict, research writes a `discrepancy` object: who
  says what, split by source type, region and language.
- Translation is late and partial. Only claim-bearing spans are
  translated, and translations are cached as derived objects. Clustering
  and classification work in the original language through multilingual
  embeddings.

### S4 Verify provenance (research; rationed web search)

- Web search is rationed by a daily quota per vertical. It fires only
  when salience is high, independent corroboration is low, or a
  discrepancy is sharp.
- Questions it answers:
  - Did the attributed party actually say this?
  - Where and when did the claim first appear?
  - Is this media recycled?
  - Is the official primary document available?
- The results are attribution and provenance objects, not verdicts.

### S5 Write (Texture)

- **A story article** transcludes live graph objects: the claims with
  their stance maps, discrepancy objects, a timeline, key reports in the
  original language with translation, and the open questions together
  with what would resolve them.
- **Beat and vertical pages** transclude story articles, and the front
  page transcludes beats. A Taiwan semiconductor story can appear inside
  the supply-chains page and the Taiwan beat at once.
- **Revision on new evidence:** when a transcluded object changes past a
  materiality threshold (a new stance, a new region, a resolution),
  Texture revises the article. Below the threshold, transclusion shows the
  new state without a rewrite, so most updates cost no inference.
- Every article follows the same structure:
  - what is widely reported;
  - where reports differ, with attribution;
  - what is unclear;
  - what would settle it;
  - a source map by country, language and source type.

### S6 Publish (Texture into the host store)

- At first a publish is owner-supervised: precommit, authorize, then a
  publication transaction binding one head.
- Later, management can delegate publishing per beat under a policy, for
  example "stories with N or more independent sources in two or more
  regions publish without review; corrections always publish".
- A correction is a forward revision, and earlier revisions stay readable.

## 4. The four desks

| Desk | Owns |
|---|---|
| **Management** | Attention and spend: budget per vertical, region and day; polling cadence; which stories get research (S3) and which get articles (S5); escalation thresholds; publish policy. It scores records, with Jev as its default scorer. |
| **Research** | Observation (S0 tool modules), classification (S1), stories (S2), claims and discrepancies (S3), provenance checks (S4), and resolving records. Its authority is read-only toward the world; it writes the paper's own graph. |
| **Engineering** | Source adapters and parsers, new platforms (AT Protocol, MTProto), and fixes when an adapter breaks (management casts the fix). It also maintains the deterministic modules: dedup, clustering and entity gazetteers. |
| **Texture** | Story, beat and front-page articles; transclusion; revision on material change; publishing. It is the sole writer of canonical article revisions. |

## 5. Precommitment records: where they come from and how they resolve

The rule: **precommit where the world will answer anyway**. Most of these
resolve automatically from counts and stance edges the paper already
collects, so the learning loop costs little inference.

| Record | Who commits | Example | Resolves from |
|---|---|---|---|
| Salience forecast | management at triage | "this story reaches 10 or more independent sources in 3 or more regions within 48 hours: 0.35" | report counts (automatic) |
| Story trajectory | research | "this becomes an official statement or denial within 72 hours: 0.6" | report types on the story (automatic) |
| Claim corroboration | research at extraction | "an independent primary source will affirm this claim within a week: 0.5" | stance edges (automatic) |
| Source behavior | research | "this outlet's report will later be contradicted by two or more independent sources: 0.2" | stance edges (automatic) |
| Classification | Jev in S1 | vertical, report type and salience, with probabilities | sampled audits by a larger model |
| Article stability | Texture at publish | "this section needs a material revision within 72 hours: 0.3" | revision history (automatic) |
| Spend value | management at allocation | "the AI and tech budget today covers 8 of the 10 stories that end up biggest" | later salience (automatic) |

What the loop buys:
- **Cheaper coverage.** Calibrated triage and classification set the
  cascade thresholds. Better salience forecasts put mid-model and search
  spend on stories that matter, so cost per well-covered story falls.
- **Track records as data, not verdicts.** Per source and per topic: how
  early it reported, how often its reports were later affirmed or
  contradicted by independent sources, and its correction rate. These are
  shown to readers as evidence ("this outlet's reports on this topic were
  later contradicted 3 of 40 times"). They are never used to declare what
  is true.
- **Perspective coverage gets measurable.** Records show which regions and
  languages tend to report first or differently. That is where the
  multilingual advantage becomes visible.
- **PSA research evidence.** Every record is the kind of evidence the
  paper's hypotheses need, especially H1 (information quality) and H4
  (reputation). The pre-outcome ordering caveat in the PSA paper's
  Section 10 applies.

## 6. Inference economics (illustrative; prices to be measured)

Shape, at an assumed 100,000 reports a day:

| Tier | Runs on | Volume | Tokens per day (rough) |
|---|---|---|---|
| S0 deterministic and embeddings | every report | 100,000 | embeddings only |
| S1 decision model (batched) | every report | 100,000 | ~30M small or decision-model tokens |
| S2 ambiguous merges | ~5% of reports | ~5,000 | ~5M small-model tokens |
| S3 claims and discrepancies | story windows, not reports | ~3,000 story updates | ~25M mid-model tokens |
| S4 web search | rationed | tens to low hundreds of queries | quota-bound |
| S5 articles | material revisions only | ~100–300 | ~5–10M large-model tokens |

The point is the ratios. The large-model tier is a small slice, and it
scales with stories that change materially. Levers, in order of effect:
1. translate late and partially;
2. extract claims per story window, not per report;
3. let transclusion carry small updates without a rewrite;
4. batch and cache;
5. resolve deterministically wherever possible;
6. move escalation thresholds with measured calibration.

Management sees cost per covered story by vertical and region, and the
budget dial changes how deep each tier goes.

## 7. Object graph

Kinds:
- `source`: outlet, account or feed, with country, language, platform
  and source type (and declared affiliation where public);
- `report`: immutable; body in content storage;
- `translation`: derived and cached;
- `story`: versioned;
- `claim`, `discrepancy`;
- `entity`, `place`;
- `attribution`: the result of a provenance check;
- `article`: a Texture document;
- `precommitment`: the existing commitment records.

Edges: `in_story`, `asserts` (with stance), `about`, `translates`,
`corroborates`, `contradicts`, `attributed_to`, `first_seen_in`,
`transcludes`, `cites`, `supersedes`, `resolves`.

Retention:
- Report bodies are kept for a bounded window (OPEN: 30 days), then
  reduced to a stub (hash, URL, time, language, source).
- Bodies that a claim, record or article cites are pinned.
- Metadata, claims, stories, records and articles are durable.

## 8. Drilling in: beats

A **beat** is a region or topic with its own budget (management), sources
(engineering), language capacity (research) and a beat page (Texture).
Global coverage comes first. Then:

1. **Taiwan.** Open internet, a free press, active forums, and official
   open data. It is strategically central to geopolitics and supply chains.
   Candidate sources: national and local news outlets, government and
   legislative releases, forums and social platforms, and Taiwan-focused
   Telegram and AT Protocol accounts. Engineering confirms each platform's
   terms and access.
2. **Broader Asian-language social media:** Japanese, Korean, Chinese,
   Vietnamese, Thai, Indonesian and others, in order of value and access.
   Multilingual clustering is what makes this cheap. Reports in a new
   language join existing stories without translation, and translation
   happens only at the claim level.

## 9. Constraints this places on Gates 1–2

| Gate / station | Constraint |
|---|---|
| SL (Gate 1) | durable timer obligations with a post-restart `paused` state; impact-propagation wake (a changed transcluded object wakes the article; new stance edges wake open records) |
| SH (Gate 1) | retention classes per object kind; content storage for report bodies outside Dolt rows; a bounded commit cadence for high-volume tables; records immutable |
| S1 (Gate 2) | capsule egress for adapters, with recorded network access |
| S5/S6 (Gate 2) | publish = precommit + authorization + a publication transaction into the host store; live transclusion of graph objects in Texture |
| Texture contract | articles are ordinary Texture documents; transclusion of graph objects and of other Textures; revision on material change |
| Platform | the host store accepts publication transactions and serves all reads; its history has a declared bound |

## 10. Build order for October (sketch)

- **W1 Observe and classify:**
  - the primary Autopaper computer;
  - the existing adapters moved into research tool modules;
  - reports in the graph with bodies in content storage;
  - Jev classification with sampled audits;
  - the timer obligation;
  - restoring the June publications into the host store.
- **W2 Stories, claims, discrepancies:** multilingual clustering, the
  stance graph, late translation, and the automatically resolved records
  (salience, corroboration).
- **W3 Articles:** story articles transcluding graph objects, vertical
  pages transcluding stories, revision on material change, supervised
  publish.
- **W4 Reading and learning:** the Autopaper app and public pages from the
  host store; source track records as data; cost per covered story on the
  management dashboard.
- **W5 Reach:** AT Protocol and MTProto adapters, then the Taiwan beat,
  then the Asian-language beats.

## 11. The old Store B data (proposal)

The corpus store behind corpusd stays as the host-level store. It was
reset empty at 12:33Z. The history-free dump finished at 13:15Z (23 GB of
SQL).

| Data | Proposal |
|---|---|
| All 633 publications with their chain and the 926 routes | migrate into the host store so the URLs work again (148 are users' work, 38 the owner's) |
| The 185 platform Texture documents (545 revisions) | migrate into the host store |
| 211 sources | import into the primary Autopaper as its initial source list |
| 2.4M items, 8.8M objects | keep in the dump. Candidate seed for W1: import recent items into the paper's graph to warm up clustering and source track records, or use as an offline evaluation set. Retrospective, not PSA evidence. |
| ~12M log rows | archive only (inside the dump) |
| The 104 G old repo | delete once the migrated tables' row counts match the dump; keep the compressed dump |

## 12. Open decisions for the owner

Decided on 2026-10-09:
- **Sequencing:** Gate 1 today, Gate 2 tomorrow, Gate 3 (Autopaper) in
  October.
- **Names:** Autopaper, Autoputer and Autoradio, never camel case. "World
  Wire", "Universal Wire" and "sourcecycled" are retired; the code rename
  is a TODO (§13).
- **Storage:** no cross-computer reading. Publishing and reading go
  through the host-level Dolt, and the June publications are restored into
  it.

Open:
1. Daily budget to start with, and its split across the six verticals.
2. Who decides publish for the first weeks: you per story, or a delegated
   policy per beat from day one?
3. Report body retention window (proposal: 30 days, cited bodies pinned).
4. Source affiliation labels (for example, state-affiliated media): which
   public taxonomy to use, or record only what sources declare?
5. Whether the 2.4M June items seed W1 or stay archive-only.
6. Order of new platforms: AT Protocol or MTProto first?
7. Taiwan sources: any you already rely on and want first.

## 13. TODO: retire the old names (owner, 2026-10-09; not done now)

"lets get rid of world wire, universal wire, and sourcecycled name.
everything can be autopaper or some derivation", then "dont rename it now,
just mark that as a todo."

Footprint measured 2026-10-09:

| Name | Code (non-Markdown) | Markdown files |
|---|---|---|
| Universal Wire | 65 files, 558 occurrences | 86 |
| sourcecycled | 33 files, 115 occurrences | 92 |
| World Wire | 6 files, 13 occurrences | 115 |

Notes for whoever does it:

- **Persisted identity, not just symbols.** `universal-wire-platform`
  (owner), `vm-universal-wire-platform` and
  `computer-universal-wire-platform` are stored in vmctl ownership on Node
  B. vmctl ensures that platform computer by these constants
  (`internal/vmctl/platform_computer.go`, `cmd/vmctl/main.go`,
  `internal/vmctl/handlers.go`), and the proxy authorizes by them
  (`internal/proxy/guest_authority.go`, `handlers.go`). Renaming the
  constants without a migration would orphan the existing platform
  computer and could start a new one. Prefer retiring it in W1, when the
  primary Autopaper computer replaces it, over an in-place rename.
- **sourcecycled** is slated for deletion (§1 principle 2). Delete rather
  than rename, after W1 moves the adapters in `internal/sources` into
  research tool modules. The host env file
  `/var/lib/go-choir/corpus-dsn.env` also carries `SOURCECYCLED_DOLT_DSN`.
- **Routes:** `/api/universal-wire/stories` and the corpusd
  `/internal/platform/universal-wire/*` endpoints are renamed or replaced
  when the Autopaper app gets its routes in W4. The host store stays.
- **Docs:** rename current documents and roadmap gate names. Dated
  receipts and `docs/archive/` keep their text, with a glossary line in the
  doctrine mapping the former names to Autopaper.
