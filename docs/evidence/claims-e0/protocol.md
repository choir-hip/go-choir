# Claims E0: evaluation protocol (frozen before the labelled runs)

Date: 2026-10-14. Mutation class: green/yellow (offline study; no runtime
change). Design: [claims, events and entities §12.3 E0](../../reports/claims-events-entities-ontology-2026-10-14.md);
historical half: [backtesting §3.4](../../research/historical-precommitment-backtesting-2026-10-14.md).
Results: [claims-e0-e6a-results](../../archive/reports/claims-e0-e6a-results-2026-10-14.md).

## 1. The frozen evaluation set

- **Source.** The Autopaper paper computer (`computer-1355b83b…`), read
  only through the QA owner credential, by `scripts/claims_ledger_pull.py`:
  every published story on the front, its `texture:` and `research:`
  inbox records, every capture those records and versions name, and the
  computer's newest 500 captures.
- **Pool.** Re-captures removed (one capture per canonical URL, the
  earliest). A **scheduled** capture is a poll's item: named by a
  management Cast or feed, or in the newest-500 window, not named by
  research, and a single feed block; it needs at least 150 characters.
  Everything else is **targeted** (research reads, which carry full text);
  it needs at least 2 blocks and 300 characters. Targeted claims are
  recorded but would never enter a track record (ontology §4.1).
- **What the pool looks like.** Of the 657 captures management's records
  name, 626 are single-block feed items (title plus summary); full text
  exists almost only for research reads. 97% of the 3,550 pulled captures
  are English; every non-English capture is a research read. The sample
  therefore takes its languages from the targeted track.
- **Sample.** 130 scheduled, stratified round-robin by language (rarest
  first), then by unseen section, at most 9 per outlet and at most 15 HN
  stubs ("Article URL: ... Points: ..."); plus 70 targeted, same rule, at
  most 4 per outlet. Seed 20261014. Result: 200 captures, 110 outlets;
  body kinds 95 feed items, 90 full text, 15 HN stubs; languages en 113,
  unclassified Latin script 37, fr 11, ja 10, es 10, de 8, pt 4, it 3,
  zh 3, ru/uk 1.
- **Splits.** `tune` 20 scheduled (the only split prompts may be tuned
  on); `label` 35 scheduled + 15 targeted (gold labels; never used for
  tuning); `unlabelled` the other 130 (quote checks, cost, feasibility).
- **Frozen**: manifest sha256
  `fd5286c87d0be5aa63050423fcb50b3142f7f86cff03b8501ab9a1bda2ee6d60`.
- The manifest (`evalset-manifest.json`) lists capture ids, doc hashes,
  outlet domains, language, section, selection and split, and its own
  sha256. Article text stays outside the repo
  (`~/.config/choir-qa/traces/claims/e0/evalset.json`).

## 2. Models and roles (omp only; no paid OpenRouter models)

| Role | Model | Why |
|---|---|---|
| extractor A | `openai-codex/gpt-6-luna` | D11 candidate |
| extractor B | `opencode-go/deepseek-v4.1-flash` | D11 candidate |
| labeller, validity judge, settlement judge | `google-antigravity/gemini-3.8-flash` | the registered judge of L2a; a different model family from both extractors |

Prompts are in `scripts/claims_prompts.py`; sha256 prefixes at freeze:

| Prompt | sha256[:16] |
|---|---|
| EXTRACT_V0 | cd0b84d0b5f1bba8 |
| LABEL_V0 | c211262ff7c8e199 |
| JUDGE_V0 | fff4d45e788d4b6b |
| SETTLE_V0 | a435b8cd412c9104 |
| CLOSED_BOOK_V0 | a90b0db475a82348 |
| HYPOTHESIS_SETTLE_V0 (E6a) | 86a018c0353b05ac |
| COVERAGE_V0 (E6a) | d26ef0b73cad0d6a |
| EVENT_FROM_TITLE_V0, ROUTE_V0, ROUTE_TITLES_V0 (routing diagnostic) | 57684de868c43140, eea58f47fb0ac143, ddbd3905da011375 |

Every model call is cached by sha256(model, prompt) under
`~/.config/choir-qa/traces/claims/cache`; the cache is the audit trail and
makes a rerun free and identical.

## 3. Blind labelling

1. The labeller sees only the rendered document (outlet, URL, title,
   fetch time, numbered blocks, at most 12,000 characters). It never sees
   an extractor's output, and labels are produced before any extraction
   is read.
2. Two independent passes (`annotation pass 1` / `2` in the prompt, so the
   cache does not collapse them). Every labelled quote must pass
   `quotematch` against its capture, like any claim.
3. **Gold = consensus**: claims whose verified spans overlap by at least
   half of the shorter span across both passes. A gold claim is
   **material** when either pass marked it material. Pass-to-pass
   agreement (F1) is reported as the labeller's reliability.
4. Recall is measured against consensus and material gold. Precision is
   measured two ways: span precision against the union of both passes,
   and validity by the blind judge (below).
5. **Status: model-assisted, not human gold.** A human audit is owed:
   take 10 label-split captures, read each document, check every gold
   claim (speaker, quote, modality) and list the claims both passes
   missed. Until that audit, recall numbers are upper bounds on a
   labeller that shares the extractors' blind spots.

## 4. Validity and settlement-feasibility judge

The judge sees the same rendered document and the numbered retained
claims of one extractor, never which extractor. Sample: all label-split
captures plus 30 unlabelled captures (seed 20261015), for both
extractors. Per claim: `is_claim`, `speaker_correct`, `modality_correct`
and the correct modality, `readings_ok`, `specific` (the specificity
check), and `settle_route` (primary_record, independent_sources,
actor_action, deadline_event, not_settleable).

## 5. Bars, fixed before the run

From ontology §12.3 E0 unless marked *proposed here*:

| Measure | Bar |
|---|---|
| speaker correct | ≥ 0.90 |
| modality correct | ≥ 0.85 |
| readings non-vacuous | ≥ 0.90 (*proposed here*; the design says "non-vacuous" without a number) |
| quote matches retained / attempted | ≥ 0.95 |
| entity-link precision | ≥ 0.90 (measured here only on the extractor's own Wikidata guesses; real linking is a judgment, D5) |
| recall of material claims | ≥ 0.70 (*proposed here*; backtesting §3.4 sets 0.85 for checkable historical claims) |
| historical extraction recall of checkable claims | ≥ 0.85 (backtesting §3.4) |
| historical count settlement accuracy | ≥ 0.98 (backtesting §3.4) |
| settlement feasibility | CE-1 falsifier: count rule covers < 20% of assertions; CE-2 falsifier: < 1 in 5 commitments checkable |
| routing | event-keyed routing beats the control (L2a's management routing on the same pairs) on merge recall without more false merges |

**Exit (ontology §12.3):** routing beats the control → merging-first
order; routing fails but settlement feasibility passes → settlement-first
(E6a, E6, then E1 for settlement); both fail → stop, CE-1 falsified for
now.

## 6. Order of runs

The routing diagnostic and the historical slice (H0, v0 prompt) ran before
this protocol was committed; H0's gold questions were written from the
documents before any extraction was read. Everything on the live set
(labels, extraction, judge) runs after this commit. EXTRACT_V1_FORWARD was
written after reading H0's v0 recall and was run only on H0, so its H0
numbers are optimistic.

## 7. What this protocol cannot do

- No human labels exist yet (§3.5).
- The routing diagnostic uses the after-window's titles only (the
  window's captures are past the capture route's newest-500 listing), and
  its same-event gold comes from the registered judge grouping the same
  titles; it is a diagnostic, not E2.
- Live assertions settle in 30 days; settlement feasibility here is the
  judge's estimate of the route, plus count settlement on the historical
  slice.
