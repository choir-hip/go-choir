# Autopaper source and story audit, first run (2026-11-10)

Wave 0 item 3 of the overnight mission; station S1 items 1 to 4 and 6 of the
improvement goal, restricted to what the public pages expose. Read-only:
no key, no action on any computer, no ssh.

Script: [scripts/autopaper_source_audit.py](../../../scripts/autopaper_source_audit.py)
(stdlib Python; fixture test `python3 -I scripts/autopaper_source_audit_test.py`).
Files here: `summary.json` (every number, for the diff), `tables.md` (the full
tables for both windows), `stories.csv` (one row per story: id, the paper's own
route, title cut to 12 words, counts), `domains_7d.csv` (one row per cited
domain). No article body, no cited-article text and no cited URL is committed;
bodies sit in the cache outside the repo.

## Method

- **List**: `GET /api/platform/publications/front?limit=100` paged by cursor
  until `updated_at` passes the window start (public, no key).
- **Story**: `GET /api/platform/publications/resolve?route=<permalink>` for
  every story in the window (public). It gives the text artifact, the cited
  source entities with their URLs and titles, the cite anchors, and the
  per-citation check verdicts the page shows readers.
- **Politeness**: one request at a time, at most 3 a second, responses cached
  on disk under `/private/tmp/claude-501/audit-cache` (a story version is
  immutable, so the second run fetches only changed or new stories). The first
  run made 4,228 network requests (the count the script logged) in about 30 minutes: about 22 list pages, 2,108 story
  reads, 2,108 probes of each story's `/v1` route (2,107 returned 404 and
  four requests got a 502 and retried). The tables were then re-rendered from
  the cache (`--offline`) after two script fixes, so `summary.json` meta shows
  0 network requests; the numbers are the first run's data.
- **Skipped as needing auth**: `/export`, `/publications/<id>`, run and tape
  endpoints (all return 401).
- **Window**: the last 7 days, `2026-11-03T02:06Z` to `2026-11-10T02:06Z`, and
  the last 24 hours, `2026-11-09T02:06Z` to `2026-11-10T02:06Z`. Membership is
  by `updated_at`, the time of the latest published version, because older
  versions are mostly not served (2,107 of 2,108 `/v1` probes were 404). A
  story first published earlier but revised in the window is therefore counted;
  only 176 of the 2,108 are new since the earlier audit. Counts: **7d 2,108
  stories (2,107 bodies read, one 502), 24h 749 stories.** Stories per day in
  the 7d window: 118, 152, 99, 253, 258, 418, 744 (11-09), 66 (11-10 to 02:06Z).
- **Distinct sources** = distinct cited URLs; **outlets** = a port of
  `internal/outlet.Of` (registrable domain; aggregators such as yahoo.com,
  Bluesky posts and r.jina.ai/web.archive.org wrappers do not count or are
  unwrapped), so the count is the publish gate's own. The per-domain tables
  still list every cited domain. `bbc.com` and `bbc.co.uk` stay separate there
  and are merged only in the integrity counts.
- **Near-duplicates**: same method as the earlier audit, title-word Jaccard of
  at least 0.6 within 48 hours, union-find groups. Calibration on the earlier
  corpus (date-only timestamps) gives 67 groups and 145 stories against the
  earlier 74 and 161, so the numbers compare as a slightly lower bound.
- **Language**: a stopword vote on the body (the title and dek when the body is
  too short), no library. On the earlier corpus titles it finds 36 non-English
  against the earlier 32 (about 4 false positives). Treat counts as +/- 20%.
- **Roundup**: title patterns only (digest, briefing, live updates, what to
  know). Weak; the unrelated-events defect cannot be seen from counts.
- **Cited item age**: only from a date inside the cited URL (15.4% of cites
  have one); the public page carries no capture or item date.
- Citation checks are the per-citation verdicts published on the page
  (`checks.citations`: supported, failed, pending).

## Headline numbers

| | 7d | 24h |
|---|---:|---:|
| stories | 2,108 | 749 |
| words, median | 187 | 229 |
| under 300 words | 1,630 (77%) | 529 (71%) |
| under 150 words | 789 (37%) | 156 (21%) |
| under 50 words | 24 | 1 |
| cites, distinct cited domains | 21,814; 2,014 | 8,796; 1,134 |
| top 10 domains' share of cites | 24.4% | 21.9% |
| distinct outlets per story, median | 3 | 3 |
| stories with exactly 2 outlets | 909 (43%) | 218 (29%) |
| stories with 1 outlet or 0 sources | 0 and 0 | 0 and 0 |
| no section | 201 (9.5%) | 1 |
| world share of stories | 55% | 67% |
| non-English (body) | 19 (16 fr, 2 de, 1 es) | 9 |
| near-duplicate groups (stories) | 39 (89) | 8 (18) |
| stories with doubled adjacent cite markers | 528 (25%) | 182 (24%) |
| stories with at least one failed citation check | 291 (14%) | 142 (19%) |
| failed checks / all citation checks | 597 / 21,784 | 305 / 8,796 |
| latest revision, median (mean) | 4 (5.4) | 4 (4.4) |
| placeholders, casting-brief titles | 0, 0 | 0, 0 |

Per-source rows (stories, cites, share, sole-source count, URL-date age,
cited-title language, failed checks) are in `tables.md` and `domains_7d.csv`.
Leaders by cites over 7d (r.jina.ai and web.archive.org wrappers are unwrapped to the real outlet): theguardian.com 6.9% (396 stories), aljazeera.com
4.3% (306), bbc.co.uk 2.5% and bbc.com 2.5%, bsky.app 1.7% (199 stories),
dw.com 1.5%, europesays.com 1.5% (62 stories; 79% of its cited titles are not
English), thehill.com, yahoo.com, com.ua. Sole-source stories do not exist
(the two-outlet gate): no story has one outlet or one source. The "only
source" metric is replaced by outlet-integrity counts in finding 6.

Coverage per section per day is in `tables.md`. Sections over 7d: world 1,159,
us_politics 303, tech_ai 270, business 175, none 201; max over min among named
sections 6.6 (6.9 in 24h). The "none" stories are all from 11-07 (35) and 11-08
(165); the last 24 hours have one.

## New stories only against the earlier audit

New = a story whose id is not in the earlier audit's `all_articles.csv`
(5,736 stories, last update 2026-11-09). 176 stories, 176 bodies read.

| | earlier (5,736) | new (176) |
|---|---:|---:|
| under 300 words | 78.2% | 91.5% |
| under 150 words | 51.1% | 36.4% |
| duplicate groups | 74 (161 stories) | 4 (8 stories) |
| non-English | 32 | 0 |
| no section | 51.9% | 0.6% |
| placeholders | 33 | 0 |
| casting-brief titles | 18 | 0 |
| adjacent duplicate cite markers | 25.5% of 663 read | 13.6% of 176 (25.1% of all 2,107 read in 7d) |

Small sample; read the direction only. Section assignment and the placeholder,
brief and language defects are gone from new stories. Length is not fixed:
new stories are more often between 150 and 300 words and more often under 300.
Duplicates, 4.5% of new stories in groups against 2.8% before, did not improve.
The 7d non-English 19 are all revisions of stories that were already there
or came in after the sample; new-only is zero.

## Ten findings, ranked by how much fixing them would improve the paper

Code locations were grepped in `internal/`; **confirmed** means the line was
read and does what is said, **hypothesis** means the mechanism is inferred.

1. **Stories are short.** Median 187 words; 77% under 300, 37% under 150 (7d).
   The two-outlet floor is the common stopping point: 909 stories (43%) cite
   exactly two outlets and 493 of the 789 stories under 150 words have only
   two (62%); median words by outlet count: 139 (2), 190 (3), 277 (5), 431 (6).
   Confirmed: the publish gate checks outlets and headline only,
   `internal/agentcore/story_publish.go:263` (`MinPublishOutlets = 2`), no length
   check. Hypothesis: the writer prompt stops at the floor
   (`internal/textureprompts/texture.yaml:40-46`, :70).
2. **Citations the paper's own judge calls unsupported are published.** 597 of
   21,784 checks failed (2.7%; 3.5% in 24h), and 291 stories (14%; 19% in 24h)
   carry at least one. 16% of 7d checks are still `pending`, 3,488, against 30
   in 24h, so the judge lags by days on older stories. Confirmed that the
   verdict is written and that p(supported) below 0.5 is "unsupported"
   (`internal/agentcore/support_judge.go:320-324`); hypothesis that nothing
   holds or revises a story on it.
3. **Section coverage is lopsided and sometimes missing.** World holds 55% of
   stories (67% in 24h); named sections differ 6.6x; 201 stories (9.5%) had no
   section, all on 11-07 and 11-08, none since. Confirmed: the scorer offers four
   sections only (`internal/frontimportance/scorer.go:340`: world, us_politics,
   business, tech_ai); hypothesis for the none burst: scorer backlog, not a
   steady-state defect.
4. **Duplicate coverage persists at the earlier rate.** 39 groups of 89
   stories in 7d (4.2% of stories), 8 groups of 18 in 24h; new-only 4 of 176.
   Titles are a lower bound. Confirmed windows:
   `internal/agentcore/intake_labels.go:50-52` (30 newest plus 10 overlap within
   72 hours); hypothesis: later reports of an event fall outside both windows
   (the problem doc's account).
5. **Doubled cite markers.** 528 of 2,107 read stories (25%) have "[1] [1]" in
   the stored text artifact itself, not only in the reader. This shifts the
   cause: the writer emits two adjacent `source_ref` nodes to the same source
   and the text projection numbers each one. Hypothesis, not read:
   `internal/texturewrite/apply.go`; the reader's per-anchor rendering
   (`frontend/src/packages/texture/texture-published-citations.ts:300`) would
   need the same merge.
6. **The two-outlet floor is a floor, and some of its pairs are not independent.**
   Confirmed in `internal/outlet/outlet.go`: the gate already ignores
   aggregators (yahoo.com, msn.com, reddit.com), Bluesky and Fediverse posts,
   and unwraps jina and archive wrappers, so those never make a pair. What the
   page shows is left: 295 stories cite a domain the gate ignores (they carry
   it as a lead); `bbc.com` with `bbc.co.uk` count as two outlets (3 stories
   rest on that pair alone); `europesays.com` (a translation aggregator, 62
   stories, 79% of its cited titles not English) counts as an outlet, and 22
   stories fall below two outlets without it (7 in 24h); in 872 stories (41%)
   one outlet carries 60% or more of the cites. Wire copy syndicated across
   outlets is acknowledged in the package comment and not measurable here.
7. **Revision churn.** Median latest revision 4, mean 5.4 per story; 1,432 of
   2,108 stories have more than one published version. Each revision is model
   calls; this is where "900 calls an hour" comes from in part. Hypothesis:
   revise-on-every-new-capture with no materiality test (wave 3's clock).
8. **Non-English stories.** 19 in 7d (0.9%; 16 French), 9 in 24h, 0 among new
   stories since the earlier audit. Sources behind them: europesays.com (5
   stories), ici.fr, la-croix.com, lefigaro.fr, pubeurope.com. 79 to 100% of
   the cited titles of europesays.com, franceinfo.fr, spiegel.de, lefigaro.fr,
   ici.fr, bfmtv.com are not English. Confirmed: the language stamped on a
   capture is the source's first language (`internal/sources/rss.go:131`); no
   prompt names the writing language (earlier doc). Hypothesis: the writer
   mirrors its sources.
9. **Dependence on a few English-language wires with a long tail.** The Guardian
   is cited by 396 of 2,107 stories (19%), the BBC (two domains) by up to 276, Al
   Jazeera 306; yet 2,014 domains appear in 7d, so the tail is wide
   (search-found). Outlet count per story is healthy (median 3). Hypothesis: search ranks
   a few wires first; relevant to the 7,200 searches a day question, not
   to the registry.
10. **The pipeline cannot be audited from outside.** The page exposes no
    capture time, no source-item date, no first-publication time (older
    versions 404), and only 15% of cites carry a date in the URL (median
    1 day where present; Al Jazeera 8 to 10 days over 24 dated items, too
    few). Capture-to-publish, freshness and failure rate are therefore
    unmeasured; see next section. Fixing this (an operator read-only export,
    S1 step 1) improves every future audit rather than the paper.

## Not measurable from public pages

| Quantity | Why not | Read that would expose it |
|---|---|---|
| Captures read by a desk but never cited | no capture list or desk reads on public pages | the paper computer's tape or world events (`internal/store/world_event.go`), via an owner-authorized operator read |
| Captures per source per day, fetched vs new, dedupe rate, items dropped past `max_items` | capture arrival is not published | the source schedules' arrival records on the tape; a per-source counter export |
| Per-source failure rate (venturebeat, cisa) | failed polls leave no public trace | schedule status or poll results on the operator surface (needs auth) |
| Queue age | the queue is internal | `GET /api/runs?limit=` with the paper computer's authority, or the tape task rows |
| Capture-to-publish time | no capture timestamp on the page; old versions are not served | first-capture time per cited capture plus the first publication time from the tape |
| Cited item's own age | only 15% of URLs carry a date | the capture's `published` field (`rss.go:128` stores it) |
| Which of the 66 registry sources feed which cited domains | cited URLs are not mapped to schedules | join of capture records to `internal/sources/registry.json` |
| Share of cited sources that came from search vs feeds | not marked on the page | provenance on the capture (source id vs search) |
| Why a story was revised | the page shows revision numbers only | the turn-commit reasons for the story's runs |

## Second run (at least 6 hours after `2026-11-10T02:06Z`)

```
python3 scripts/autopaper_source_audit.py --since 7d \
  --out docs/evidence/autopaper-source-audit-2026-11-10-run2 \
  --diff docs/evidence/autopaper-source-audit-2026-11-10/summary.json
```

It reuses the cache (only new or changed story versions are fetched; the cost
is a few hundred requests), writes the same files plus `diff.md` (every moved
number and the 15 largest domain-share movers). For a same-window comparison
rather than a rolling one, pass `--since 2026-11-03T02:06:11Z` instead of `7d`.
