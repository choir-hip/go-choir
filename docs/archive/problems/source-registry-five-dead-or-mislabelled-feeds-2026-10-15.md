# Source registry: five dead or mislabelled feeds (2026-10-15)

Status: documented; fix follows in a separate commit. First found by the
2026-10-13 sourcing research
(docs/archive/reports/multilingual-osint-sourcing-research-2026-10-13.md §1, §12).
It is an L0 parallel item in the ledger metamission ("registry repairs").
Mutation class of the fix: orange (catalog data in
`internal/sources/registry.json`; no code). Rollback: git revert.

## Re-verified live, 2026-10-15 about 20:50 UTC

One GET per URL; checked item count, newest item date and title language.

| id | registry URL | result |
|---|---|---|
| `rss:nhk_world_en` | `https://www3.nhk.or.jp/rss/news/cat0.xml` | 7 items, newest **2026-08-08**; titles are **Japanese** |
| (candidate) | `https://www.nhk.or.jp/rss/news/cat0.xml` (redirects to `news.web.nhk/n-data/conf/na/rss/cat0.xml`) | 7 items, newest 2026-10-16; Japanese |
| `rss:koreaherald` | `https://www.koreaherald.com/rss/news.xml` | **0 items** (a valid shell) |
| (candidate) | `https://www.koreaherald.com/rss/kh_National` | 50 items, newest 2026-10-15, English |
| `rss:36kr` | `https://36kr.com/feed` (and `/feed-article`) | `text/html`, **not a feed** |
| `official:who:news` | `https://www.who.int/rss-feeds/news-english.xml` | 207 items, newest **2026-02-25** |
| `rss:agriculture_dive` | `https://www.agriculturedive.com/feeds/news/` | 10 items, newest **2025-01-31** |

**Correction to the research report:** the NHK entry was never English.
It is named "NHK World Japan English", with `languages: ["en"]`, but the
feed it polls is NHK's Japanese domestic news, and so is the replacement.
Moving the URL alone would keep the wrong language label.

## Impact

None of the five is scheduled on the paper (its 28 source schedules), and
no code, test or config names these ids. The registry is the catalog that
computers schedule from, so a computer that adds one of these today gets a
dead or mislabelled source.

## Fix

- `rss:nhk_world_en`: URL → `https://www.nhk.or.jp/rss/news/cat0.xml`,
  name → "NHK News (Japanese)", languages → `["ja"]`. The id is kept so
  any schedule that names it continues; it is misnamed, and a later
  registry-id cleanup can rename it.
- `rss:koreaherald`: URL → `https://www.koreaherald.com/rss/kh_National`.
- `rss:36kr`, `official:who:news`, `rss:agriculture_dive`: removed. No
  working feed was found, and the registry has no status field to hold a
  dead entry. Re-adding one is a one-entry change once a live URL exists.
- Not done here: the recurring freshness audit (report §10.3). That
  belongs to tomorrow's ingestion scale-up with the owner.
