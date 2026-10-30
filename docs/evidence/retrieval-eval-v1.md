# retrieval-eval-v1: definition (frozen 2026-10-14)

The yardstick for retrieval over the Dolt object graph (rung 0 of
[object-graph-retrieval-research-2026-10-14](../archive/reports/object-graph-retrieval-research-2026-10-14.md)
§5-6; problem record
[corpus-retrieval-windowed-unindexed-untimed-2026-10-14](../archive/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md)).
Every retrieval rung is measured on this set before the next is built; a
rung that moves nothing is reverted or kept only for latency.

## Files

| File | Where | Holds |
|---|---|---|
| `retrieval-eval-v1.queries.jsonl` | repo (this directory) | queries, shape tags, split, judged capture ids and URLs, judgment kind, source run ids. **No article text, no result text.** |
| `scripts/retrieval_eval_extract.py` | repo | rebuilds the queries file from the read-only trace extract |
| trace extract `paper-research-events.json` | outside the repo, `~/.config/choir-qa/traces/search/` | the 136 paper research runs (10-14 10:20Z-14:23Z) the queries come from |
| frozen corpus snapshot | outside the repo | a Dolt tag on a local copy of the public captures the judged ids name, with block text; built when the first rung is scored (`retrieval-eval-v1-corpus`) |
| blind judge labels, pooled top-20 judgments | outside the repo, `~/.config/choir-qa/retrieval-eval-v1/` | per-query relevance from the registered judge, with article text as the judge saw it |

## Classes in v1

- **R1c, corpus-first** (168 queries): each literal `choir.SearchWireCorpus`
  query from the paper's research runs. Relevant: the captures the same run
  read with `choir.ReadCapture` and the URLs it read with `choir.Read` after
  the call. 153 have at least one judged id.
- **R1w, web search as corpus-first would see it** (400 of the 576 queries
  that were followed by a read): each literal `choir.SearchStart` query,
  stratified by shape (URL, non-Latin, `site:`, quoted phrase, short token,
  plain; rarer shapes first, then by query hash). Relevant: the URLs the run
  read after it and before its next search cell.
- R2 (same-event merges), R3 (entity resolution) and R4 (claims) as in the
  research note §6 join as **v2**, not as edits to v1.

Shape counts in v1: short token (AI, EU, US, 5G …) 252, plain 217, `site:`
101, quoted phrase 94, non-Latin 7, URL 1 (a query can carry several).

## Labels

`judgment: "silver:read-after-query"` marks the automatic label: the run
went on to read the item. It is a lower bound on relevance (a run reads one
page of several relevant ones) and is biased toward what the old search
returned. Before a rung's number is reported as recall, every system's
top-20 for each query is **pooled** and labelled blind by the registered
judge (two shuffled calls; a human reads a 10% sample), and the pooled label
replaces the silver one in the outside-the-repo label file. Quoted-phrase
queries also get automatic judgments: every capture block of the frozen
corpus containing the phrase is relevant.

## Splits and freezing

`split` is `held_out` when the first 32 bits of sha256(lowercased query) mod
10 are below 3 (178 of 568 queries), else `tuning`. Tuning a rung (tokenizer
rules, ranking weights) may read only the tuning split. The queries file is
frozen: new queries arrive only as `retrieval-eval-v2`, so numbers across
rungs stay comparable.

## Metrics

Per class and overall: recall@10 and @20 against the judged ids (a capture
id, or a capture of a judged URL in the frozen corpus), MRR, nDCG@10 once
pooled labels exist, p50/p95 latency at production row counts, write cost
per capture, and index bytes. Each miss is classified: not in corpus,
lexical, entity, temporal, semantic-only (research note §5 decides from
that split when embeddings become the next lever).

## Baseline to record first

The window path (newest 1,000 captures, Go word-prefix match) and the SQL
term path (all history) on the same frozen corpus, held-out split, before
any ranking change.
