# Front page leads with a one-source stub, 2026-10-14

## Evidence

- Owner, about 13:40 UTC, with phone screenshots: the Autopaper front
  page's new lead is "Mauritius reels from worst riots in decades after
  woman shot by police" (v3). The story is one paragraph, and its toolbar
  reads "Sources 1". Owner: "So part of the ranking needs to be the
  quality of our article. And I mean, if it's important, we should be
  devoting more resources towards making sure we do have a quality
  article. That is doing web research in addition to pulling from our
  source ingestion."
- Live front row (GET https://choir.news/api/platform/publications/front?limit=100,
  read about 13:45 UTC): `pub-6515ef386f02e5059cc3e614857f548a`,
  `latest_revision` 3, `version_count` 1, `updated_at`
  2026-10-14T12:58:00Z, `importance` 8 (the LF0 Jev score), no promo.
- Its v3 bundle (GET /api/platform/publications/resolve?route=…/v3): the
  manifest's `body_doc` is one paragraph of 121 words with one
  `source_ref`; `anchors` holds one citation, to one AFP web capture
  (`src_de8f81951b274b53`). The reader's "Sources N" counts distinct
  cited sources over the body's `source_ref` nodes (key: the anchor's
  `source_entity_id`, else its `ref.doc`;
  `frontend/src/lib/texture-published-citations.ts`
  `buildPublishedCitationContext`), so N = 1.
- The ranking that put it first (LF0, `rankFrontStories` in
  `frontend/src/lib/autopaper.ts`): importance × 0.5^(age/24h). Recomputed
  over the first 60 live rows, the top ten and the depth of each (sources
  counted the reader's way, words over the body's text nodes):

  | rank | importance | sources | words | story |
  |---|---|---|---|---|
  | lead | 8 | 1 | 121 | Mauritius riots |
  | 2 | 8 | 5 | 303 | UK apology over stillborn burials |
  | 3 | 8 | 19 | 598 | India voter-roll deletions |
  | 4 | 7 | 3 | 247 | UK hosts Zelenskyy |
  | 5 | 7 | 2 | 160 | Nord Stream trial |
  | 6 | 7 | 3 | 198 | Bank of America fine |
  | 7 | 7 | 1 | 36 | Pope Leo surgery |
  | 8 | 7 | 1 | 2 | Nvidia $250m to UK (dek: "embed test") |
  | 9 | 6 | 15 | 993 | AI botnets on IoT |
  | 10 | 6 | 5 | 733 | Guinea-Bissau vaccine trial |

  6 of the top 10 have 3 or more sources; the median is 3. Three of the
  top eight are one-source stubs, one of them a two-word test body.

## Problem

1. **The rank reads importance of the event, not quality of our article.**
   The front API carries no depth signal (sources, length), so the
   frontend cannot tell a one-paragraph, one-source stub from a
   researched story on the same event. The most important event of the
   hour is led with our thinnest coverage of it.
2. **Important thin stories stay thin.** Nothing in the paper notices
   that a story it published is important but thin and spends more on
   it. Whether the paper's research for Mauritius searched the web or only
   the capture corpus is not read from its trace here; that is a
   hypothesis for the deepen design (part 2), not a finding.

   **Trace read (about 14:10 UTC), superseding that hypothesis:** research
   run dc4c5adc-ae98-4112-a994-b231ab927b1e searched the corpus (0
   captures), then the web (three searches, each on 1 or 2 of 9 providers,
   the rest cooling), read and captured two pages (Africanews/AFP and the
   BBC), and reported one round citing one of them. Texture published that.
   The stub is one shallow research round plus degraded search providers,
   not a missing web search. The twenty newest research runs (13:22–13:41)
   show the same provider state. Detail:
   docs/archive/reports/autopaper-deepen-important-stories-2026-10-14.md.

## Fix direction

Part 1 (orange, frontend rank and an additive corpusd field): corpusd adds
`source_count` (counted the reader's way) and `words` to each front row;
the frontend multiplies importance by a depth factor (1 source 0.5, 2
sources 0.75, 3 or more 1.0; under 150 words a further 0.8), and a story
with fewer than 2 sources may not lead. Missing fields leave the factor at
1, so the ranking is unchanged until corpusd serves them.

Part 2: the paper's management learns which of its own published stories
are important but thin and opens bounded web research to deepen them.
Design note: docs/archive/reports/autopaper-deepen-important-stories-2026-10-14.md.

Measure: share of the top 10 with 3 or more sources, and the median
sources of the top 10, over a six-hour window before and after (baseline
above: 6/10, median 3).
