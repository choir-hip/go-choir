# L2a Jev Pair Probe (2026-10-14)

Station: [L2a](../definitions/choir-ledger-l2a-synthesis-intake-2026-10-14.md).
This is the fastest falsifier of the bridge conjecture: can Jev tell the
same event from the same topic but a different event, across languages,
well enough to route on?

## Method

- **Items.** Real headlines and RSS summaries, fetched at 00:58 UTC from
  registry feeds: BBC World, Guardian World, TechCrunch, The Verge (en);
  ITmedia (ja); Folha (pt); Xataka, El País, France 24 es, Euronews es
  (es); France 24 fr (fr); DW (de, ar); Numerama (fr); Golem (de). 36kr
  (zh) failed to parse.
- **Labels.** 26 pairs, labelled by Claude (a different model family from
  Jev's) and fixed in the probe script before any Jev call. Ambiguous
  pairs (Falklands, Rivne) were left out.
  - 13 same event, all but one across languages;
  - 5 same topic but a different event, 2 across languages;
  - 8 unrelated.
- **Call.** `typesafe/jev-1.13` (served as `jev-1.13-20260917`) through
  OpenRouter's Decisions endpoint, the same route as the gateway. One
  choice question: same_event, same_topic_different_event or unrelated,
  with a criterion for each.

## Result

- 26 of 26 correct.
- Median latency 0.2 s, maximum 0.4 s.
- Confidence median 1.0, minimum 0.54. The lowest probability on a
  chosen answer was 0.69.
- Cost $0.00068 for all 26 pairs, about $0.00003 per pair.

Evidence: `~/.config/choir-qa/traces/l2a/` (feeds.json, probe.py,
results.json sha256 586e0c9f1447cc43…), outside the repo.

## What it does not show

- Inputs were headlines and summaries, which are clear signals.
  Whole-capture text, ambiguous follow-ups and analysis pieces are
  harder.
- No zh or ko pair was tested (the 36kr feed did not parse). The fixture
  must include them.
- 26 pairs is small. The fixture's merge recall and false merges against
  its labels remain the decisive test.
- At scale Jev compares a capture against a shortlist of stories, not
  every pair. The shortlist's recall is a separate question.
