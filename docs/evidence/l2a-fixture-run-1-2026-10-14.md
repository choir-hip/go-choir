# L2a Fixture Run 1 (Acceptance Action 1)

Date: 2026-10-14. Station:
[L2a](../definitions/choir-ledger-l2a-synthesis-intake-2026-10-14.md).
Verdict: **pass** (amended 04:15 UTC). Merging passes every criterion.
The first reading was "incomplete" because 4 of 8 published versions did
not cite 2 or more languages. The owner then removed that clause: "no hard
rule that multiple languages be in every story. That's silly", and "Was
that just a test? That's not a production autopaper rule". It was a
fixture bar written in the L2a station file, never a production rule; no
prompt requires it. The citation-language table below is kept as
information.

## Run

- Minted test computer `computer-00b7886133ccd87d0cecc7079b9d78ba`, guest
  build 66fe0917, `intake.labels` on.
- One schedule on `fixture:l2a_multilingual_2026_10_14`, 10-minute
  interval, `max_items` 9. 27 items over three polls: a stricter variant
  that also tests lossless deferral and labels across polls.
- 02:27:33 to 03:57:50 UTC, then scored, then the computer was stopped.
  It was started again at about 04:10 for the rescore and stopped after.
- Harness: `scripts/l2a_fixture_run.py`, `scripts/l2a_fixture_score.py`.
  Labels are kept outside the repo
  (`~/.config/choir-qa/traces/l2a/fixture-labels.json`). Outputs are in
  `~/.config/choir-qa/traces/l2a/fixture-run-1/` (`score.json` is the
  first, broken score; `score-v2.json` is the corrected one).

## The first score was a scorer bug

The first score reported recall 0.0 with every capture undisposed, yet the
computer had opened 15 stories. The scorer had three faults:
- it matched capture ids from record text without the `obj:` prefix
  against ids that carry it;
- it took every capture whose link matched a label, including the desks'
  research re-captures of the same pages (30 instead of 27);
- it compared percent-encoded feed links with unencoded labels (two DW
  links, Arabic and Chinese).

Fixed: arrivals now come from the source's `captures` records in
management's inbox. Stories are read from Casts (`cast.captures`) and
Feed dispositions (`disposition.captures`) in each story's Texture inbox.
Ids are normalized, and link paths are unquoted. The same story reader
now serves `scripts/l2a_window_pairs.py`.

## Corrected score

| criterion | bar | result |
|---|---|---|
| fixture items captured | 27 | 27 |
| stories per event | exactly 1 | 1 for all 8 events |
| merge recall (same-event pairs in one story) | at least 0.8 | **1.0** (16 of 16) |
| false merges | at most 1 | **0** |
| captures without a disposition | 0 | **0** |
| ~~each published version cites 2 or more languages and 2 or more outlets~~ (removed by the owner) | — | 4 of 8 on the strict measure (information only) |

Per event, the published version's citations (languages are counted
only for the fixture's own labelled links):

| event | languages counted | cited outlets |
|---|---|---|
| E1 Pope lung cancer | ko, zh-TW | bbc.com, ltn_zh, vaticannews.va, yonhap_ko |
| E2 OpenAI bugging arrest | en | justice.gov, techcrunch |
| E3 Saudi strikes, Riyadh intercepts | ar, en, zh | bbc, dw_ar, keyt.com, news.cn, rfi_zh |
| E4 MI5 threat warning | en, ko | bbc_world, yonhap_ko |
| E5 Nord Stream trial | none counted | bbc.com, bundesgerichtshof.de, generalbundesanwalt.de, spiegel.de |
| E6 Tupac murder sentence | en | ten English-language outlets |
| E7 Guilfoyle ousted | en, es | bbc_world, france24_es, and 5 more |
| E8 Kyiv power plant strike | none counted | bbc.co.uk, cnn.com, kyivindependent.com, kyivpost.com, liga.net |

Read by hand (a hypothesis until a language-aware measure exists): E5 and
E8 cite sources in other languages (German courts and Spiegel; Ukrainian
liga.net), but the published versions cite research's own captures of
origin sources, not the fixture's links, so the strict measure counts
none. E2 and E6 really cite English only, though E2's research report
used the Japanese ITmedia capture.

## What it means

- Synthesis intake is working: every event became one story, with no false
  merge and nothing dropped, across zh, ja, ko, ar, es, fr and de over
  three polls.
- (Superseded by the owner's ruling; kept as an observation.) The
  remaining gap is in Texture's writing, not in intake. The
  published version does not reliably carry the event's other-language
  coverage into its citations. The synthesis contract asks for
  convergence and conflict across sources, but nothing makes it cite each
  language's capture when that capture adds something. Next: tighten the
  contract and score citations by language, including non-fixture
  sources (each capture's language from its arrival record or its Jev
  label).
- Next: enable intake on the paper after C1e B1 lands and the paper
  computer refreshes when idle.
