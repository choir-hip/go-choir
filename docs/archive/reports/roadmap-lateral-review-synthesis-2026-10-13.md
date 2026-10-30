# Roadmap review, lateral round: synthesis (2026-10-13)

Panel: lateral mode, eight members, all `ok` (claude opus, devin, opencode,
codex, gpt-6.1-sol, gemini-3.8-flash, deepseek-v4.1-flash,
muse-spark-1.3). Prompt: `.agentic-consensus/roadmap-prompt-2026-10-13.md`.
Raw outputs are session diagnostics under
`.agentic-consensus/roadmap-lateral-2026-10-13/`. Inputs:
docs/reports/roadmap-after-autopaper-live-DRAFT-2026-10-13.md,
docs/reports/multilingual-osint-sourcing-research-2026-10-13.md, the C2
close receipts and the arrivals problem record.

## The shared reframe (8 of 8)

The draft rested on one default: **each capture is a duty a model-backed
desk must discharge** (management.yaml lines 29-34, `autopaper_live`
criterion 2). Every member rejected it. Their reframes differ in metaphor,
not in substance:

- A capture is an **observation** attributed to its source. It costs no
  model call. The scarce, paid unit is the **open question**: management's
  story hypotheses and a vertical's standing questions.
- Captures pass through free reduction (exact and near-duplicate dedupe,
  per-item language ID, grouping into event candidates with every
  source's provenance kept). Only a candidate that matches an open question
  or crosses a novelty threshold reaches a desk, as **one record per
  candidate, not one per capture**. Every capture still gets a recorded,
  inspectable disposition; it does not get a model-authored answer.
- **Precommitment records are already being emitted and dropped.**
  Management's `Cast("texture:new", …)` spec carries `hypotheses` with
  `statement` and `probability`; nothing mints them as records, resolves
  them or scores them. The first PCR step is to stop dropping them.
- **Sources never precommit.** "Outlet A reported X" is an observation
  attributed to A (protocol design §10). Source track records are views
  over resolved claims, with denominators and lineage, never a single
  reputation number.
- **A vertical is a standing-question set with a resolution calendar**, not
  a feed list.
- **Cost is a design invariant and an acceptance line in every station**
  ($/capture, $/story, $/resolved hypothesis), not a measurement station.
- **The defensible asset is dated, unbackfillable history**: prospective
  hypotheses, source lineage and resolutions. Coverage can be rented; the
  clock cannot be started retroactively.
- **Apps and Autoradio are read models over the same ledger** (evidence
  reader, PDF and media embeds, audio briefing of one pinned story version
  whose corrections appear in the next edition). They complement the core
  when they render it, and conflict when built as separate surfaces.

Analogies offered: astronomy alert brokers with pre-registered filters
(claude, deepseek); innate versus adaptive immunity (devin, opencode); LHC
multi-level triggers with a prescaled audit stream (gemini, muse); clinical
surveillance versus prospective trials (gpt-6.1-sol, codex); double-entry
bookkeeping (deepseek); Crossref rather than Reuters (devin).

## Corrections to the draft

1. **The burst was a cursor bug, not an attention limit** (gpt-6.1-sol,
   codex). The draft read 46/118 as one desk failing at scale. The trace
   says seq 9-16 were never shown. Fixed at cff54bf5; E3 must be re-run on
   the fixed build before the per-capture contract is judged. The scaling
   argument (about 1,000 records an hour at 211 sources on one serial
   desk, median answer 101 s at 14) still holds on its own.
2. **Runtime count-scoring is withdrawn** (codex, gpt-6.1-sol; verified):
   both 2026-10-11 PCR designs carry 2026-10-12 notices: hypotheses replace
   count questions, and resolution and scoring are done by intelligence
   (Jev, a model, an RLM), not runtime counting. Counts (lead time,
   independent corroboration, correction rate) are **analytics views** the
   owner asked for, not scores. Count-resolver designs proposed by claude,
   devin and muse are adopted only in that form.
3. **"No budgets" is a settled metamission boundary**
   (choir-reeng-metamission-2026-10-12.md:156). A monetary ceiling in the
   runtime needs an explicit owner amendment; the owner's "cost and
   performance first" (2026-10-13) is the ruling to cite, and the next
   metamission's boundaries must say so (codex).
4. **Triage must not become a hidden runtime editor** (claude, codex,
   gpt-6.1-sol): management authors the admission policy (questions,
   thresholds, exploration share); runtime executes it and records every
   disposition. Grouping is an observation management can split or merge.
5. **Near-duplicate dedupe already exists unwired**: `qdrant_dedup.go` has
   no callers since 3c963edf (deepseek). Convergence rule: evaluate it
   before writing new dedupe.

## Risks every member named against their own reframe

- **Question-led collection becomes a confirmation-bias machine** (all):
  reserve an explicit exploration share, audit a random sample of
  undisposed and below-threshold captures by language and source class,
  and report unresolved shares. "Filtered" must never quietly mean
  "missed."
- **Corroboration counts measure herding**: syndicated wire copy and state
  media echo. Count independence by owner and lineage, not URL.
- **Resolution scarcity**: news claims resolve slowly and ambiguously; a
  ledger nobody resolves against is a diary. Start with short-horizon
  hypotheses and structured, externally grounded resolutions (TWSE
  filings, MND PLA activity figures, CEC results, CISA KEV).
- **Cross-lingual grouping quality is unproven** for zh-TW/ja/ko/en; test it
  on a labelled set before committing to a model.
- **Legal**: Telegram's 2026-02 terms prohibit scraping and AI use (73
  channels already polled); CNA RSS is non-commercial; ransomware.live and
  ACLED need licences; excerpts do not cure an AI-use prohibition.
- **Durability**: the asset is history; every durable home shares one host.
  Off-host copies of the ledger (SP0) move early (gpt-6.1-sol, codex, muse).
- **Authority exposure**: contain guest-to-corpusd internal authority before
  expanding untrusted-content automation or exposing an API.

## Where the panel split

- **C1e before or beside the next metamission.** Finish first, so the
  ledger is not built on packets and work items scheduled for deletion:
  codex, gpt-6.1-sol, muse (time-boxed to about a week), deepseek (as
  station 1). In parallel, since it touches engineering, not the paper
  path: claude, devin, opencode. Synthesis: close C2's owed evidence and
  the cursor fix now; run C1e as the next shared-runtime cutover and do
  not start a second one (the intake redesign) until it lands; prepare
  the Taiwan question set, labelled evaluation set, source shortlist and
  app designs in parallel.
- **Self-development versus news as the first PCR target.** claude notes
  the vision orders supervised self-development first; devin, deepseek and
  muse make the apps the first payload of scored self-development. The
  same hypothesis record serves both.

## Revised first stations (synthesis, for the convergent round)

0. **Close the current metamission**: E3 rerun on cff54bf5, A1 badge, C1e,
   final convergent review. Off-host ledger copy if C1e allows.
1. **Bounded attention**: free reduction, management-authored admission
   policy, one record per event candidate, a disposition for every capture,
   exploration and audit samples, registry repair (NHK, Korea Herald,
   36kr, WHO, Agriculture Dive), per-item language ID, evaluate the unwired
   near-duplicate path. Acceptance: replay the tier-1 burst and a 10×
   synthetic burst; every capture has a recorded disposition; duplicates add
   no paid calls; ≥95% of labelled material events reach a desk; the audit
   sample is reported.
2. **Hypotheses minted, resolved by intelligence, scored as separate
   records**: Cast hypotheses become commitment records with a resolution
   criterion and horizon; a Jev or model scorer resolves them; scores carry
   scorer identity; two scorers can disagree; per-source analytics views
   (lead time, independent corroboration, corrections) with denominators;
   daily public hash of the ledger. Acceptance: ten or more short-horizon
   hypotheses committed before their resolving evidence, at least half
   resolved with evidence refs, Brier per desk visible.
3. **Taiwan as standing questions**: 10-20 questions, about 30 zh-TW and
   36 ja/ko/zh feeds (registry only), a JSON-diff connector for TWSE then
   CISA KEV, structured resolutions, corpus-first retrieval for resolution
   evidence, a daily spend line. Acceptance: each question has matched
   captures from at least 3 sources in 2 languages within 14 days; at least
   3 resolutions close end to end from structured sources.
4. **Ledger read models**: evidence reader with PDF and media embeds, then
   an Autoradio prototype narrating one pinned version, built through the
   self-development loop as its first scored payload.

Later metamissions: further verticals, the apps rebuild beyond the reader,
the security hardening pass and red teaming (authority containment
earlier), SA and SP scale-out (re-measured after C1e), news API, paid
tiers.

## Owner decisions this surfaces

1. A monthly (or daily) model-spend ceiling, as an amendment to "no
   budgets".
2. The Telegram stance given its 2026-02 terms; whether Autopaper counts as
   commercial (CNA, UDN terms).
3. Confirm Taiwan/East Asia as the first vertical.
4. Whether C1e lands before the intake redesign (synthesis default) or in
   parallel.

## Convergent round (2026-10-13, same day)

Panel: convergent, seven selected; six answered (claude, codex,
gpt-6.1-sol, deepseek, gemini: **ready with changes**; opencode: **not
ready**); devin produced no review (its non-interactive tool call was
rejected). Prompt: `.agentic-consensus/roadmap-convergent-prompt-2026-10-13.md`.
The blocking findings overlap; adopted as follows.

### Corrections to the synthesis above

- **Cast hypotheses are not dropped.** `castBodyFromPacket`
  (`internal/agentcore/inbox_records.go`) lifts them into the typed Cast,
  and the inbox fold gives them stable H-ids with open, settled and dropped
  states (`internal/store/inbox.go`). What is missing is a resolution
  criterion, a horizon, a settler that acts, and scoring (claude,
  gpt-6.1-sol). Station 2 is smaller than written.
- **The unwired near-duplicate path is not a candidate.** `qdrant_dedup.go`
  needs Qdrant and Ollama, neither provisioned, and it drops items with only
  a log line, which breaks "every capture has a recorded disposition"
  (claude, gemini, deepseek). Retire it as dead code; do not wire it.

### Adopted changes (blocking)

1. **Admission shape** (claude, opencode, gpt-6.1-sol). The runtime does
   only lossless, identity-preserving reductions: exact duplicates by URL or
   content hash (already in `sourcegraph/captures.go`), per-item language
   as metadata, and grouping proposed as an observation a desk can split or
   merge. Captures reach management as **one digest record per poll
   window**. Dispositions are authored by a model: a cheap triage call per
   digest, not per capture, applies management's written policy (standing
   questions, exploration share) and records a disposition for every
   capture, escalating candidates. The runtime never thresholds a capture
   out of a model's sight, and a near-duplicate is a recorded, reversible
   disposition, never a drop.
2. **Station 1 acceptance must be runnable and safe** (claude, gemini,
   gpt-6.1-sol, deepseek). The replay runs on a minted test computer, never
   the paper computer (published versions are permanent). Bursts enter
   through the real poller: a test source schedule pointing at a
   deterministic fixture feed. A frozen labelled set (model-proposed,
   owner-sampled, at least 40 material events, multilingual duplicates,
   wire copies, corrections and contradictions) is built during station 0.
   Acceptance adds paid calls per digest versus per unique event, replay
   completion time, recall by language and source class, and the audit
   sample.
3. **Station 3 splits into a deterministic gate and a live soak**
   (deepseek, gemini, gpt-6.1-sol). Gate: a seeded zh-TW/en/ja fixture
   proves question match, cross-lingual grouping and a structured
   resolution end to end within a tracer run. The 14-day live window is an
   observation that does not hold the station. Questions are chosen with
   scheduled resolutions (TWSE monthly revenue by the 10th, CISA KEV). A
   200-pair cross-lingual evaluation precedes choosing a grouping model.
   Telegram stays out pending the owner's ruling; CNA and UDN stay out if
   Autopaper counts as commercial.
4. **Station 4 gets a test and loses its hidden prerequisite** (all).
   Reader: one pinned story version opens with PDF and media evidence and
   every chip resolves to a capture block, for a logged-out reader.
   Autoradio: an audio artifact names the exact version it narrates, and a
   later correction produces a new edition citing the change. Self-
   development authoring is optional, not a gate (Gate 2 stays deferred).
   The full apps rebuild moves to a later metamission; the owner is told
   Autoradio slips past "about a month" unless it is pulled forward.
5. **Cost boundary, written as a named amendment** (all). Proposed text:
   "Owner amendment 2026-10-13 ('cost and performance first') to the
   2026-10-12 'no budgets' boundary: spend is measured, not budgeted, by the
   runtime. Every station's acceptance reports measured cost from trace
   events (the llmcost aggregator) as $/capture, $/story and $/settled
   hypothesis. Any ceiling is an owner kill switch in provider or host
   policy, never a value the runtime reads to decide. No new recurring
   spend (paid APIs, licences, hosts) without owner sign-off. No runtime
   reasoning budgets, round caps or runtime scoring."
6. **Station 2 smallest version** (claude, gpt-6.1-sol, opencode). Add
   `resolves_by` (7 days or less) and `criterion` to Cast hypotheses;
   settlement (outcome plus evidence refs, by the designated settler) stays
   separate from scoring (a separate record with scorer identity, from Jev
   or a model). Score the committed hypothesis version with its original
   probability; require that the commitment's seq precedes the earliest
   capture its settlement cites. Brier is an analytics view with
   denominators, not calibration evidence at n≈10. The public daily hash is
   deferred. This is prompt plus an existing record type, so it may start
   inside station 0 to start the clock. A recoverable off-host copy of the
   ledger is station 2's entry criterion.
7. **E3 rerun criterion** (claude): first confirm the paper runs cff54bf5
   or later (deploys defer busy computers); then the pass is the cursor
   invariant (no record marked read unless a `choir.Records()` call returned
   it, with mid-run arrivals waking the next turn). "Every item answered"
   is the contract station 1 retires.

### Station 0 (unanimous among respondents)

Close first, time-boxed to about a week: E3 rerun, A1 badge, C1e, final
convergent review. One shared-runtime cutover at a time. In parallel:
labelled set, Taiwan question set, source shortlist and permission checks,
station 2's smallest version, app designs.

### Owner decisions still open

1. Approve the cost amendment text (change 5), and any kill-switch level.
2. Telegram given its 2026-02 terms; whether Autopaper is commercial.
3. Taiwan/East Asia as the first vertical (default: yes).
4. Accept that Autoradio follows stations 1-3 unless pulled forward.
