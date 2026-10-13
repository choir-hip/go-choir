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
