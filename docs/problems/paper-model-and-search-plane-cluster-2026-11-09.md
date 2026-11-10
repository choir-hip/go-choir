# The paper's model and search plane: clustering assessment (2026-11-09)

Class: green (assessment). Read-only: eight problem records, main at
b45451433, the Node B gateway journal 19Z to 01:33Z. Each record has an
"Investigation 2026-11-09 evening" section with its own findings.

## Status of the eight records

| Record | Status tonight |
| --- | --- |
| web-search-dead (11-05) | Stale headline; Keenable alone answers; budget open |
| published-search-refuses | Fixed 244325257; live proof pending |
| pool-routing-walks-dead | Fixed ead8873b6, 7334daf86 |
| texture-wake-cache / Bedrock 400 | Fixed (1f8dfe4a5, d2adcb6ca, 18a654482 and others); reasoning=high residual |
| paper-quota-incidents | Partly fixed 36534be53; parking runs open |
| judge-model-record / rarity | Fixed 267364fa7 |
| model-provenance-unwired | Built; judge line fixed |
| autopaper-article-quality | Mission lane |

Seven of eight were fixed in the day; the headers were not updated. What
remains is capacity and supply, not defects.

## Do they share a cause?

Two clusters, one real.

1. The real cluster is provider availability memory, in two halves that do not
   meet. The computer's `QuotaLedger` (Demote, Succeed, Persist) records which
   models are down. The gateway's search plane keeps its own provider health
   (`sqlite_health.go`, cooldown ladders per outcome class). Both now follow
   the doctrine 5ca86505a (shared memory, exponential backoff, survives
   restarts). Both depend on a classifier that reads an HTTP status and error
   text to decide quota versus rate versus error. Tonight's misses are all
   classifier or capacity questions: brave's 429 flips between quota and rate
   in three seconds; serper's 400 is `error` and probably means no credit;
   parallel's 402 is real. No unwired replacement exists; the memory is wired.
   The gap is that no layer owns supply: nothing knows that the only live
   search provider has a monthly allowance and the paper spends it at 300 an
   hour, and nothing knows that free model capacity is exhausted so the paper
   runs on paid Bedrock (2,584 of about 4,500 request lines in 3 hours).
2. A second pattern, failure as a note in a successful result: published
   search returned 403 inside `published_note`; the judge and rarity
   failures were log lines only; Bedrock's 4xx reason was sanitized away.
   Three silent failures found by reading code, not by an alarm. Fixed one by
   one; no gate yet reads these notes.

The other records (cache carry, Bedrock translation, vocab guard) are
independent symptoms with independent causes and are fixed.

## Ranked fix list, for publishing well overnight on free models with a $20 cap

1. Search supply is the overnight risk. Keenable is the single point of
   search and the paper uses about 300 searches an hour, 7,200 a day,
   against about 3,300 a day of allowance (owner figure, month-level, not
   verified). Fix first: find the research step that issues the searches
   (a run trace; not read), dedupe and cache them (7 of 772 hit the cache),
   then give Keenable a daily budget so it degrades gracefully. Orange; the
   dedupe depends on the trace (unsized), the budget about 40 lines.
2. Model supply: free capacity is exhausted, so the paper runs on the paid
   tail. Watch the $20 cap, whose enforcement landed in 45844274b (Bedrock
   and River stop at their daily budgets). The overnight risk is
   the paper stopping at the cap, not overspending. Fix: read the spend report
   (scripts/spend-report) at the start of the night, confirm the cap's
   behavior at the limit (a hold, not a crash), and confirm the Texture
   `reasoning=high` residual is not inflating cost. Orange, about 2 hours.
3. Classifier corrections for brave (monthly 429) and serper (400 as quota).
   Small, orange, about 10 lines, 1 hour; log the first brave 429 body so it
   is evidence, not guess.
4. Live proofs for the fixed items (published search on d@e; absence of the
   judge and rarity log lines on the paper): green/yellow, 1 hour, so the
   records can close.
5. Later: parking incident runs off dispatcher slots; a gate that fails a
   proof when any `*_note` reports a half-failed verb.

## Could not verify

The paper's VM log (so no count of quota-incident lines, judge or rarity
lines), the real Keenable allowance and its account balance, brave's and
serper's actual response bodies, tonight's Bedrock spend (no daily total was
readable), which research step issues the searches, and whether the deployed
build includes the fixes named above.
