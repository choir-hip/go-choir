# L2a Registered Judge

Date: 2026-10-14, registered before any S1a run. Station:
[L2a](../archive/definitions/choir-ledger-l2a-synthesis-intake-2026-10-14.md),
acceptance actions 1 (fixture) and 2 (live windows).

## Model and route

- Model: `google-antigravity/gemini-3.8-flash`, thinking `high`.
- Route: the owner's `omp` CLI (`omp -p --mode json --no-session
  --no-tools`), not OpenRouter. The owner ruled out paying for
  OpenRouter models for this harness on 2026-10-14.
- Family: Google. It differs from Jev (typesafe/jev) and from the paper's
  desk models (opencode-go: longcat, cat-jam, deepseek).
- Identity check: every call's JSON reply must name provider
  `google-antigravity` and model `gemini-3.8-flash`. A call served by any
  other model is discarded and counted, never scored.

## What it judges

**Live windows (the denominator).** The window's captures, from
`GET /api/captures?since=&until=`, as numbered lines: outlet, language
and title. The judge groups the lines that report the same real-world
event: the same incident or its direct development, in any language.
Same topic but a different event is not the same event. It answers as
JSON: `{"groups": [[i, j, ...], ...]}`, listing only groups of two or
more.

**The fixture.** The labels file (outside the repo) is the answer. The
judge is used only for a capture the labels do not cover, which it groups
against the labelled events by the same rule.

## Prompt

    You group news items by real-world event. Two items are the same
    event when they report the same incident or its direct development
    (an arrest and the charges that follow; a strike and its casualty
    count), in any language. Items on the same topic that report a
    different incident are different events. Below are numbered items:
    outlet, language, title. Answer only with JSON:
    {"groups": [[numbers of items that report one event], ...]}. List
    only groups of two or more items. Leave out items that match nothing.

## Tie rule

- Two independent calls on the same input, in shuffled order. A
  same-event pair counts only when both calls group it together.
- Pairs that only one call grouped are reported as `disputed`, and their
  count is published next to the denominator.
- A window over 400 captures is judged in blocks of 400 by time, with
  each block overlapping the next by 100. A pair counts if any block
  grouped it under the rule above.

## What it counts

- Cross-outlet same-event pairs: pairs inside a group whose outlets'
  registered domains differ. This is the denominator of merge recall.
- Merge recall: the share of those pairs whose two captures were fed
  into the same story.
- False merge: two captures in one story that the judge put in different
  groups, or that it left ungrouped. Each one is checked by reading the
  pair.
