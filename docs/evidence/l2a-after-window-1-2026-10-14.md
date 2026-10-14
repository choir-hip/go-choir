# L2a After-Window 1 on the Paper (06:51-12:51 UTC, 2026-10-14)

Scored with `scripts/l2a_window_pairs.py` (judge
google-antigravity/gemini-3.8-flash, registered in
l2a-judge-registration-2026-10-14.md; 0 calls served by another model).
Raw output outside the repo: `~/.config/choir-qa/traces/l2a/after-1-pairs.json`.

| measure | baseline A (15:00-21:00, 10-13) | after-window 1 |
|---|---|---|
| captures | 181 arrivals | 425 |
| captures attributed to a story | 0 (pre-L2a records don't name captures) | 55 (13%) |
| same-event pairs (judge) | 127 | 404 |
| cross-outlet same-event pairs (denominator) | 46 | 46 |
| merged cross-outlet pairs | 0 (not measurable) | 8 |
| merge recall | not measurable | **0.174** |
| false-merge candidates | - | 31 |
| disputed pairs | - | 7 |

## Reading

- **Recall is far below the fixture's 1.0.** 38 of 46 cross-outlet
  same-event pairs did not end in the same story; e.g. BBC and Guardian on
  the Japanese journalist arrested in Russia, and on the Six Flags death.
- **Only 13% of captures are attributed to a story**, so most captures
  either went to research opens that did not cite them, were declined, or
  were not disposed. The window also contains this morning's incidents:
  the management stall, 28 captures dropped by a failed management turn
  (fixed in e3437f1c), the 10:25 crash-refresh, and circuit-open periods.
- **The 31 false-merge candidates look mostly thematic, not false:** a
  sample is one story combining Guardian live blogs and articles about
  Zelenskyy's UK visit and Russian strikes on Ukraine. Not yet adjudicated
  pair by pair.

## Verdict

L2a's live acceptance ("merge recall rises without false merges") is
**incomplete**: recall rose from unmeasurable to 0.174, which is not the
intended effect size, and the false-merge candidates need adjudication.
Next: adjudicate the 31 candidates; trace why the 38 missed pairs did not
merge (per capture: labels, Jev's settle, the disposition and the story
chosen), from the ledger before any change; then a second window on the
current build (S2a delegation is on the paper from 13:01, with its task
refusal fixed in 6f8e36ceb).
