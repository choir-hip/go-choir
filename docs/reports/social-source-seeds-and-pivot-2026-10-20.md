# Social source seeds and the pivot (2026-10-20)

Mutation class: orange (registry rows plus one outlet-key rule); rollback is a
git revert. Owner direction: `docs/reports/owner-notes-2026-10-14.md` sections
30 and 32 (start AT Protocol and Fediverse ingestion; choose supernodes that
are themselves aggregating, then pivot out from them). Prior research:
`docs/archive/reports/multilingual-osint-sourcing-research-2026-10-13.md`.

## The rule that governs everything

Social posts are leads, not citations as outlets. A story still needs two
distinct outlets (`MinPublishOutlets`, `internal/agentcore/story_publish.go`
:239). An account may be cited as the speaker of a claim. Every row below has
`source_standing: social_lead`, no licence, `official` false, and
`store_body_policy: excerpt_only` (no reader import of the pages a post links
to: that would file a third party's page under a social row).

The gate counts outlets from a capture's URL (`outlet.Of`, called from
`citedOutlets`). Before this change a Bluesky post named `bsky.app` and a
Mastodon post named its instance, so one post plus one real outlet read as two
outlets. `internal/outlet/outlet.go` `isSocialPost` now returns no outlet for
`bsky.app` hosts, Mastodon `/@user/<id>` posts on any instance (a tag feed's
items carry the origin instance's URL, so host lists cannot work), and
ActivityPub `/users/<u>/statuses/<id>`. `t.me` and `x.com` keep their existing
account-as-outlet keys; this slice does not revisit them.

## Aggregator-first seed rule

Seed order, most to least valuable:

1. Platform-computed signals: Mastodon `/api/v1/trends/links` per instance (it
   carries a per-day history of uses and accounts) and Bluesky
   `app.bsky.unspecced.getTrends`. The platform has already done the
   aggregation; we read its verdict. Slice 2.
2. Curated feed generators, lists and starter packs. Slice 2.
3. News-bot, wire-service and newsroom accounts that post links, and
   journalist instances such as journa.host.
4. Individual accounts: only when approved from the pivot's candidate list.

Slice 1 ships tier 3 because it is the only tier with plain RSS today. The
20 Bluesky rows are honest about this: they are newsrooms and wire services
that post links (an account that posts a link to a story is already a
syndicator of attention), not aggregators in the strict sense. They are
useful as leads and as the seed set the pivot scores from.

## Seeds (all verified HTTP 200 with fresh items on 2026-10-20)

Method: one GET per URL with `User-Agent: Autopaper source check
(+https://choir.news)`, 1.5 to 2 seconds apart. Bluesky `profile/<handle>/rss`
answers 302 to the DID form; rows poll the DID form so a handle change cannot
break them, and the id keeps the handle.

Bluesky (id `social:bsky:<handle>`, 20 rows, 30-minute poll, 20 items):
apnews.com, reuters.com, afp.com, npr.org, aljazeera.com, theguardian.com,
france24.com, spiegel.de, lemonde.fr, elpais.com, kyivindependent.com,
meduza.io, timesofisrael.com, thehindu.com, japantimes.co.jp, bellingcat.com,
icij.org, occrp.org, globalvoices.org, restofworld.org. Chosen for language
and region spread (German, French, Spanish, Russian-language news in English,
Israeli, Indian, Japanese) and for two investigative collectives that curate
cross-border leads (ICIJ, OCCRP) and one OSINT outfit (Bellingcat). Failed and
skipped: bbc.com, tagesschau.de, dw.com, ft.com, haaretz.com, abc.net.au,
cbc.ca, rferl.org (404, no account under that handle); bbcnews.bsky.social,
scmp.com, hackernews.bsky.social, newslit.bsky.social (200 but empty);
theconversation.com, newsbot.bsky.social, intelcrab.bsky.social (stale, last
post 2023 to 2025).

Mastodon (id `social:mastodon:<instance>:tag:<tag>`, 7 rows, 20 items):
`mastodon.social` #news, #breaking, #breakingnews (30 minutes) and #osint (60
minutes); `journa.host` #news and #breaking; `newsie.social` #news. Tag feeds,
not account feeds, because account feeds mostly failed: journa.host and
mastodon.social account feeds for ProPublica, Reuters, AP, Bellingcat,
Kyiv Independent, NPR, DW, Al Jazeera gave 404 or 410 or were empty, and the
ones that answered (tagesschau, DW, theguardian on mastodon.social) were last
posted in 2017 to 2019. Tag feeds overlap heavily (identical newest items on
mastodon.social, journa.host and newsie.social); item ids are the post's
origin URL, so the repeat check collapses them, and the extra instances buy
only different federation reach. Failed or empty: infosec.exchange #osint
(empty), newsmast.social account (self-promotion only), several others.

## Polling cadence

Thirty minutes for news and breaking, sixty for #osint; never under thirty
(guard-tested). Bluesky answers HEAD with 404 and sends no ETag, so those rows
use `conditional_request_mode: none`; Mastodon sends ETag, so tag rows use
`etag`. 27 rows at 20 items is at most a few hundred item-reads per hour, well
below any limit.

## Pivot method (slice 2, not built here)

1. A Mastodon trends-links poller type reads `/api/v1/trends/links` on a few
   instances (and Bluesky getTrends), recording uses and distinct accounts per
   day.
2. From seed posts (RSS above, plus trends) extract external links,
   canonicalize them (strip tracking parameters, resolve shorteners such as
   the `bit.ly` links AP posts), and score each URL by co-citation: the number
   of distinct seed accounts that link it, not boosts or reposts.
3. Run bipartite HITS on the account-to-URL graph. Accounts with high hub
   scores that no seed already covers become a hub-candidate list for owner
   approval. Nothing is auto-followed: an approved candidate becomes a new
   `social:` row by the same guard test.
4. For a link that trends, `/api/v1/timelines/link?url=` on Mastodon shows who
   shares it, which finds the next hubs without a crawl.
5. A trending URL is a lead for the outlet that published it. The paper cites
   that outlet's own capture; the social account is cited only as a speaker.

## Etiquette

A descriptive User-Agent on every request (the registry's
`ChoirUniversalWire/0.1 (+https://choir.news)` today; rename with the Autopaper
code rename). Honor 429 and `X-RateLimit-*` headers with backoff. Honor
Mastodon `discoverable` and `indexable` flags and, in slice 2, skip accounts
that opt out. robots.txt checked 2026-10-20: bsky.app allows all and
documents 429 backoff and a firehose for bulk use; mastodon.social and
newsie.social disallow only media proxy and interact paths (GPTBot is
blocked); journa.host publishes `Content-Signal: search=yes, ai-train=no,
use=reference` and blocks several scrapers, so we read and cite posts as
references and do not train. A post is its author's work: rows carry no licence
and store an excerpt; the paper links the post rather than reprinting it.

## Failure modes

- Silent feeds: an account stops posting and the row looks healthy. The
  registry status checks should flag a row with no new item in 14 days.
- Link-only posts counted as outlets: closed at the key (`isSocialPost`), and
  by `captureHoldsOnlyLinks` for captures that hold only links.
- Bluesky items have no title and the body is the post text; a title-less item
  must not be treated as a headline.
- Account deletion or handle change: Bluesky rows poll the DID, so a rename
  survives and a deletion yields 404. Mastodon account moves leave a stale
  feed; this slice avoids account feeds.
- Rate limits: backoff on 429, never a retry storm; one request per URL per
  interval.
- Instance blocks or shutdowns: an instance can block our User-Agent or go
  away; three consecutive failures should mark the row degraded.
- Tag-feed noise: #news is open to anyone, including spam and non-news.
  Posts there are the weakest leads and must pass the same two-outlet rule.
- Shortened and tracking links hide the destination; canonicalize before
  scoring.

## Guard

`internal/sources/registry_social_test.go` lists all 27 ids and urls and
asserts the standing, no licence, not official, no reader import, a 30-minute
floor, at most 20 items, a unique valid URL, and that sample post URLs of the
row's platform name no outlet. `internal/outlet/outlet_test.go`
`TestSocialPostsNameNoOutlet` covers the key rule, including that Medium
author pages still name `medium.com`.
