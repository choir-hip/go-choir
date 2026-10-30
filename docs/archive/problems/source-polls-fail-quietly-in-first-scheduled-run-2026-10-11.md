# Source polls fail, one of them quietly, in the first scheduled run

Found 2026-10-11 in W1's hour-long scheduled run on staging (19b2f964, test
computer demo-arxiv-2026-10-10, six sources every ten minutes from
11:52:44Z). Evidence class: schedule usage from
`/api/autopaper/sources/schedules`, plus a direct fetch of the DW feed.

## What happened

Three of six sources produced nothing on their first poll:

| source | recorded status | what it means |
|---|---|---|
| rss:dw_en | ok, HTTP 200, fetched 0 | the feed served 127 KB with dozens of items; the parser found none |
| rss:france24_en | error, HTTP 403 | the site refused the fetch |
| official:un:all_news | error, timeout awaiting headers | the site did not answer in time |

The two errors are visible and back off as designed. The DW result is not:
it reads as a healthy, quiet feed.

## Cause

DW serves RSS 1.0 (RDF). In RSS 1.0 the `<item>` elements are children of
`<rdf:RDF>`, beside `<channel>`, not inside it. `parseRSSLikeFeed`
(internal/sources/rss.go) reads items only from `<channel>` (RSS 2.0) or
`<entry>` (Atom). An RDF feed has a channel with no items, so the parser
returns zero items and no error. Six registry sources are RDF:
rss:dw_en, rss:dw_de, rss:dw_ar, rss:dw_ru, rss:allafrica_top and
rss:nikkei_xtech. None of them has ever produced an item.

The 403 and the timeout are the sites' behavior toward Node B's fetches.
They belong in a per-source health record, not a parser fix.

## Test

An RSS 1.0 feed with items beside its channel parses to those items, with
`dc:date` as the published time. The registry's RDF sources then produce
items on staging.
