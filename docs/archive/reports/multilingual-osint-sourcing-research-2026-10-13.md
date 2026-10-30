# Multilingual long-tail OSINT sourcing: research for Autopaper

Date: 2026-10-13. Author: research agent (Claude Code), read-only on the repo.
Mutation class of this document: green (docs only). No code, registry or config was changed.

## 0. How to read this report

- **Evidence labels.** `[V]` = I tested it myself on 2026-10-13 (curl, a WebSocket probe, or a repo/API query) and the result is stated. `[S]` = from a web search or fetched documentation only; the URL is cited and I did not test it. `[U]` = could not verify; stated so.
- **Where a source is a vendor page, aggregator, or blog, I say so.** Several fetches of official docs (Bluesky docs, Mastodon timelines doc) returned empty or redirect pages to my fetch tool; those facts are marked `[S]`/`[U]`.
- **Tests were made from a single residential US egress** with the polite UA `ChoirUniversalWire/0.1 (+https://choir.news)`. Many 403s (Cloudflare, Akamai) are IP/fingerprint dependent. **Re-test from the production egress before dropping any source.**
- **Division of labour.** I ran the social-network probes (Section 3) and the repo, translation and legal research (Sections 2, 9, 11) myself. Four delegated research passes (bridges, Taiwan/East Asia, Global South and cyber, aggregators and discovery) were briefed to use the same `[V]`/`[S]`/`[U]` convention and to test with curl; I re-checked the NHK, Korea Herald and 36kr findings and ran the registry audit myself. Their `[V]` tags are their reports of tests, not mine.
- **Costs** are published list prices or my arithmetic from them, with the assumptions shown. They are not quotes.
- Nothing here is legal advice.

## 1. Executive findings (the ten things that matter)

1. **The existing registry has almost no Taiwan/East Asia depth and some of it is broken.** Of 137 RSS sources, only about a dozen are Asian, only 36kr, ITmedia and Nikkei xTech are non-English East Asian, and there is **no Taiwanese feed at all** (Telegram `taiwannews` is the only Taiwan source). A one-shot freshness audit of all 137 feeds today `[V]` found: `rss:nhk_world_en` is **frozen since 2026-08-08** (the old `www3.nhk.or.jp` host still returns 200; the live feed moved to `www.nhk.or.jp/rss/news/cat0.xml`), `rss:koreaherald` (`/rss/news.xml`) returns a **valid RSS shell with zero items** (the `/rss/kh_Business` variant has 50), `rss:36kr` (`https://36kr.com/feed`) returns **HTML, not a feed**, `official:who:news` has no item newer than 2026-02-25, `rss:agriculture_dive` none newer than 2025-01-31. Four more (`arabnews`, `mining_com`, `railfreight` 403; `venturebeat` 429) and `hn_frontpage` (connection reset) failed once and need a retry from production. That is a cheap fix with no new connector.
2. **Native publisher RSS in Taiwan, Japan and Korea is plentiful and fresh.** About 100 feeds in zh-TW, ja, ko, zh and en were curl-verified live today (Section 5). Taiwan: CNA (via FeedBurner), LTN, UDN, PTS, Newtalk, RTI, ETtoday, Yahoo TW, iThome, TechNews, DigiTimes (zh and en), Taiwan FactCheck Center, MyGoPen, Executive Yuan, Presidential Office, MOFA. All plug into the existing RSS connector unchanged.
3. **Closed or hostile surfaces, as of today `[V]`:** PTT (Cloudflare challenge even for its native Atom), Dcard, LIHKG, Mobile01 (all 403), PRC MFA (maintenance page), Xinhua and People's Daily feeds (dead or years stale), TrendForce RSS (frozen 2026-07-01), TSMC/SEMI/SIA newsrooms, Japan MOD/MOFA (403). PTT/Dcard/LIHKG coverage would need custom scrapers and carries real ToS risk.
4. **Bluesky/Mastodon/Nostr/Lemmy are weak for this vertical.** A 4-minute Jetstream sample `[V]` saw 11,326 posts; Japanese was 2.7%, Chinese 0.12%, and essentially none about Taiwan. Taiwanese social life is on Threads, PTT, Dcard and LINE, which are not freely readable.
5. **Global South coverage is cheap if you use publisher RSS and the BBC language services.** 31 BBC language-service feeds (Hindi, Tamil, Swahili, Hausa, Amharic, Tigrinya, Burmese, Nepali, Somali, Yoruba, etc.) are live and **none is in the registry** `[V]` (via the Global South research pass; Section 6). BBC reuse terms must be checked.
6. **Cyber is almost free and mostly structured.** CISA KEV (CC0 JSON), NVD, EPSS, GHSA, OSV, CERT feeds (JP, KR, TW, DE, FR, UK, EU, PL, NL, HK) and ~35 vendor/research blogs are live. This vertical needs a small JSON-diff connector more than new RSS. Ransomware.live's terms require written approval for commercial use and forbid re-serving the data.
7. **X/Twitter is effectively closed.** Nitter and XCancel are down after cease-and-desist letters in Aug-Sep 2026 `[V]`; only the paid API ($0.005 per post read `[S]`) is safe.
8. **Telegram is the biggest legal soft spot in the current registry.** Telegram's terms were amended around 2026-02-03 to prohibit scraping/aggregation and AI/ML use `[S]`. Choir already polls 73 channels via `t.me/s`. This needs an explicit owner decision (Section 12(d)).
9. **Translation is cheap enough to stop worrying about.** At list prices, a Haiku-class LLM costs about $0.00028 per 600-character item versus about $0.012 for Google NMT (about 40x), and NLLB-200 is non-commercial licensed. Translate lazily; embed multilingually; use local language ID (Section 9).
10. **GDELT's two free file feeds are the legal long-tail discovery layer; Google News RSS is the best cheap lead generator but violates its robots.txt and ToS.** GDELT gives URLs and entities in 48+ languages but no text and thin Taiwan depth; see Section 8.

## 2. The connector shape in the repo (what "fits" means)

Read from `/Users/wiz/go-choir/internal/sources/` (`types.go`, `rss.go`, `telegram.go`, `gdelt.go`, `registry.json`):

- A `Source` has `id, type, name, url, verticals, languages, regions, jurisdictions, tier, poll_interval_seconds, max_items_per_poll, rate_limit, conditional_request_mode, user_agent, tos_class, robots_policy, auth_policy, store_body_policy, retention_days, official, source_standing, status`.
- A poller returns `PollResult{Fetch FetchRecord, Items []Item}`. An `Item` carries `URL, CanonicalURL, Title, Body, Published, Language, Region, ContentHash, BodyKind, SourceTOSClass, StoreBodyPolicy, EvidenceLevel, RawJSON`.
- Types: `rss` (generic RSS/Atom/RDF; HTTP conditional GET; 2 MB body cap; 100 items per poll default), `telegram` (HTML scrape of `t.me/s/<channel>`), `gdelt` (the 15-minute `lastupdate.txt` zip triple: GKG, mentions, export), and a `polymarket` constant. `rss.go` sets `Item.Language` to the registry's first language, so language is declared by source, not detected.
- Current registry (counted from `registry.json`): 211 sources = 137 rss, 73 telegram, 1 gdelt. Languages by declaration: en 153, ru 15, es 8, fr 7, uk 7, pt 5, de 5, ar 2, zh 2, ja 2, others 1. `store_body_policy`: bounded_text 100, excerpt_only 97, bounded_release_text 12. Poll interval 900 s for 197 sources. `tos_class` values: public_feed 125, public_web_preview 73, official_public_feed 12, public_dataset 1.

**Implication.** Anything that is RSS/Atom/RDF is a registry row, not code. A **JSON-diff connector** (poll a JSON snapshot or window, emit one Item per new record) would unlock CISA KEV, NVD, TWSE announcements, OCHA FTS, World Bank news, HDX, Indian Parliament. A **language detector** at capture would fix the "declared language only" limitation. Streaming kinds (Jetstream) are a different shape from the poll loop and are not recommended now.

---

## 3. Federated and open social networks (category 1)

**Bottom line.** For the Taiwan/East Asia vertical these networks are low-yield. They are cheap and legal, but the Chinese-language (Traditional) population on them is tiny. Use them as a *narrow tripwire* (a few named accounts and hashtags), not as a coverage layer. Bluesky is the only one with a genuinely bulk-readable stream, and its language mix is English/European/Japanese/Portuguese-heavy.

### 3.1 Measured on 2026-10-13 (our own probes, not vendor claims)

| Probe | Result | Caveat |
|---|---|---|
| Jetstream (`wss://jetstream2.us-east.bsky.network/subscribe?wantedCollections=app.bsky.feed.post`), 45 s | 2,059 new posts, 45.8 posts/s, ~152 MB/hour of uncompressed JSON for posts only (about 3.6 GB/day); top declared languages en 64%, de 6%, es 5%, ja 3%, pt 3% | Single short window, ~13:50 ET; Asia-time peaks will differ |
| Same, 240 s | 11,326 posts (47.2/s); declared-language top: en 7,311 / no 1,225 / de 476 / es 467 / pt 450 / **ja 309 (2.7%)** / fr 271; **zh: 14 posts (0.12%)** | `langs` is author-declared and optional (about 8% of posts had none in the first sample) |
| Same, keyword regex for Taiwan / TSMC / semiconductor / Taipei / Xi / PLA terms | Effectively zero genuine hits in 4 minutes (the 4 regex matches were false positives) | Regex was loose; confirms topical rate is far below 0.1% of the firehose |
| Mastodon `https://mastodon.social/tags/taiwan.rss` | HTTP 200, `application/rss+xml`, 20 items, newest within an hour | Per-instance view of the tag, not federated-wide |
| Mastodon `.../tags/%E5%8F%B0%E7%81%A3.rss` (台灣), mstdn.jp `/tags/台湾.rss`, pawoo.net `/tags/台湾.rss`, mastodon.world `/tags/taiwan.rss` | All HTTP 200, 20 items each | Tag-feed items are sparse in text; mostly short posts |
| `https://g0v.social/tags/taiwan.rss` (Taiwan civic-tech instance) | HTTP 200 but **0 items** | Tag may simply be unused there |
| `https://infosec.exchange/tags/ransomware.rss` and `/tags/cve.rss` | HTTP 200 but an **empty channel (0 items)** | Likely instance-side restriction of tag feeds; I did not diagnose it. Do not plan on infosec.exchange tag RSS |
| Mastodon `/api/v1/timelines/tag/taiwan` on mastodon.social | HTTP 200 JSON | Unauthenticated; mastodon.social `timelines/public`, g0v.social and infosec.exchange public timelines returned **HTTP 422** (not available unauthenticated) |
| Bluesky `public.api.bsky.app/xrpc/app.bsky.feed.searchPosts` unauthenticated | **HTTP 403** (HTML body) | Matches third-party reports (below). `getAuthorFeed` and `getFeed` on the same host returned 200 |
| Bluesky `https://bsky.app/profile/<handle>/rss` | HTTP 302 to the DID-form URL, then 200 `application/xml` | Works for public accounts |
| Lemmy `lemmy.world/api/v3/post/list?community_name=taiwan` and `/feeds/c/taiwan.xml` | HTTP 200, 20 RSS items | Small community |
| Nostr `wss://relay.damus.io`, kind-1 live subscription, 40 s | 19 notes (0.5/s); 0 CJK-script notes | One relay, tiny sample |

### 3.2 Per-network assessment

| Network | Access method | Cost | Volume / signal | Languages / regions | Reliability | Legal/ToS | Choir mapping |
|---|---|---|---|---|---|---|---|
| **Bluesky: per-account RSS** | `https://bsky.app/profile/<handle>/rss` (302 then RSS) | Free, ~0 compute | Per-account; original posts only per vendor docs; text-only, links not clickable per OpenRSS | Whatever the account posts | One blogger could not get it working out of the box ([source](https://d4blog.codeberg.page/blog/fediverse-and-bluesky-rss-feed/)); I got it working. Treat as best-effort | Public data, official feature ([OpenRSS](https://openrss.org/blog/bluesky-has-launched-rss-feeds)) | **Plain RSS connector, no change** |
| **Bluesky: Jetstream** | WebSocket JSON; `wantedCollections`, `wantedDids`, optional `compress` (zstd) and `cursor` ([Jaz's design post](https://jazco.dev/2024/09/24/jetstream/); official docs at bsky.network/docs/jetstream/ were not fetchable by my tool, so parameter details are secondhand) | Free public instances; bandwidth ~3.6 GB/day raw for all posts on our sample | ~4M posts/day, of which topical-for-us is tiny. Needs client-side keyword/DID filtering | Global; ja is the main East Asian language; zh nearly absent | Public instances have no SLA; you can self-host | Public data, protocol is open | **New connector kind** (long-lived stream, not poll). Low value for Taiwan; only worthwhile with a `wantedDids` allow-list of curated accounts |
| **Bluesky: searchPosts / custom feed generators** | `app.bsky.feed.searchPosts` (lang, since, until, domain, tag params) now rejects unauthenticated callers with 403 ([GitHub issue](https://github.com/bluesky-social/bsky-docs/issues/332), [DEV post](https://dev.to/cooki/nobody-is-monitoring-bluesky-so-i-built-a-mentions-scraper-for-it-19mi), [Apify docs](https://apify.com/bitofacoder/bluesky-scraper)); authenticated with an app password reportedly works. `getFeed` for existing feed generators works unauthenticated (tested) | Free; account needed | A curated community feed (e.g. a Taiwan-politics feed) is effectively someone else's filter, cheap to poll | Depends on feed | Auth requirement and rules can change without notice | Small new connector (`getFeed` / `getAuthorFeed` JSON to items) |
| **Mastodon: tag and account RSS** | `https://<instance>/tags/<tag>.rss`, `https://<instance>/@user.rss` ([search summary of fedi.tips and Feedly docs](https://fedi.tips/how-do-i-follow-hashtags-on-mastodon-and-the-fediverse)); tested | Free | 20 items/feed; per-instance federated view; Mastodon default rate limit 300 requests / 5 min per IP ([Mastodon docs](https://docs.joinmastodon.org/api/rate-limits/)) | Chinese-language Mastodon: ~154k Chinese-posting users in 2023 (mostly mainland refugees from censorship) per [SCMP](https://scmp.com/tech/tech-trends/article/3192843/chinese-social-media-users-are-flocking-decentralised-mastodon); Japan has large instances (mstdn.jp, pawoo.net) | Instance admins can restrict tag feeds (infosec.exchange returned empty) | Public, no auth, widely accepted use | **Plain RSS connector, no change** |
| **Mastodon: MAU context** | n/a | n/a | About 750k-1M monthly active users in 2026 ([Marketful](https://marketful.com/mastodon-statistics), [Manifold/FedIndex](https://manifold.markets/AIBear/what-will-be-the-number-of-monthly), secondary sources) | n/a | n/a | n/a | n/a |
| **ActivityPub relays** | Relays push public posts to subscribed instances (requires running an instance) | Running a Mastodon/GoToSocial instance, ops-heavy | Larger firehose than any single instance | Mixed | Fragile, relay set changes | Gray: you become an instance operator receiving others' public posts | Not recommended now. I did not verify current relay inventories |
| **Nostr** | Raw relay WebSocket (`REQ`), NIP-50 search relays (relay.nostr.band, search.nos.today per [Zenn](https://zenn.dev/nikolat/articles/4f401c2cd60f77)) | Free | Our 40 s probe on one relay: 0.5 notes/s, none CJK. Tiny, crypto-skewed | Japanese community exists per [nostr.wine](https://nostr.wine/) mirrors but no 2026 volume data found | Relays come and go; I did not verify nostr.band status | Public, permissionless | New connector (WebSocket); **skip** |
| **Lemmy** | Per-community RSS `https://<instance>/feeds/c/<community>.xml` and `/api/v3/post/list` (both tested, 200) | Free | Roughly 48.6k MAU across ~455 instances at end of 2025 ([poster.ly guide citing FediDB](https://www.poster.ly/guides/lemmy-guide)) | Mostly English | Stable | Public | **Plain RSS connector**; r/taiwan-like communities are small |
| **Threads (Meta)** | No public feed/RSS; Taiwan's dominant new social app in 2024 per [MIT Tech Review](https://www.technologyreview.com/2024/04/03/1090601/threads-taiwanese-users-talk-politics/) | n/a | The platform where Taiwanese discussion actually lives, and not freely readable | zh-TW | n/a | Scraping is ToS-risky; official Threads API requires app review and is mostly for your own account (not verified in this report) | Not available cheaply. Flagged as the biggest coverage gap |

**Bluesky scale context (secondary sources, treat as ranges):** about 43-46M registered accounts but declining activity, 10.4M mobile MAU in June 2026 per Similarweb via [Gigazine](https://gigazine.net/gsc_news/en/20260812-blueskys-active-user/); 1.41B posts in 2025 (about 3.86M/day) per its transparency report as cited by [SocialPilot](https://www.socialpilot.co/blog/bluesky-statistics/). The 3.86M/day figure agrees with our measured 47 posts/s. A country breakdown listing Brazil, US and Japan as the largest ([Adam Connell](https://adamconnell.me/bluesky-statistics/)) is undated and unverified; our language sample is consistent with a Brazil/Japan presence but the `pt` share we saw (4%) is lower than that claim would suggest.

**Could not verify:** official Jetstream public instance list and current bandwidth figures (the official docs pages returned empty to my fetch tool; Jaz's 2024 post gives 24 GB/day pre-surge, 232 GB/day post-surge for the *full* firehose, not Jetstream), Bluesky searchPosts policy (no official statement found), Taiwan-specific Bluesky adoption (no source found).

**Recommendation for this category:** (1) Add ~20-40 hand-picked Mastodon/Bluesky accounts and 5-10 hashtags as plain RSS items under a `social_post` body kind: zero new code. (2) Do not build a Jetstream consumer for the Taiwan vertical. Revisit if/when a Japanese-language vertical needs it: ja is ~2.7% of posts, which is ~1.2 posts/s, still low-signal without account lists.

---

## 4. RSS bridges for platforms with no feeds (category 2)

*(Research pass for this section: tests and repo queries on 2026-10-13; `[V]` marks what was tested.)*

**Bottom line.** For Taiwan and East Asia, **native publisher RSS beats bridges** (cheaper, legal, stable). Bridges are for the few platforms with no feed, and the useful no-login routes are Telegram, Bilibili, Bluesky and Mastodon. The Chinese platforms that matter (Weibo, Zhihu, Douyin, Xiaohongshu, WeChat) need cookies, puppeteer and proxies, which is an ops and ToS burden Choir should not take on at this budget.

### 4.1 RSSHub

| Fact | Detail | Evidence |
|---|---|---|
| Project | `DIYgod/RSSHub`, about 46.5k stars, last release v2026.10.13 (today), **AGPL-3.0** (not MIT) | `[V]` via GitHub API, https://github.com/DIYgod/RSSHub |
| Breadth | About 1,983 route namespaces | `[V]` https://github.com/DIYgod/RSSHub/tree/master/lib/routes |
| Health | 53 open issues; 65 issues created since 2026-07-13 and only 2 still open, i.e. breakage is reported and closed quickly. Per-platform issue counts in that window: bilibili 6, twitter 5, youtube 2, douyin 1, zhihu 1, telegram 1, weibo 0. A low open count can also mean aggressive closing | `[V]` GitHub API |
| Self-host image | `diygod/rsshub:latest` 129 MB compressed; `chromium-bundled` 496 MB; the `latest` tag has no Puppeteer; compose adds Redis (+ Browserless for browser routes) | `[V]` Docker Hub API https://hub.docker.com/v2/repositories/diygod/rsshub/ ; `[S]` https://docs.rsshub.app/deploy/ |
| RAM | No official figure. My estimate: roughly 200-500 MB without Chromium, 1 GB or more with browser routes | `[U]` estimate |
| Public instance | `rsshub.app` served a Cloudflare challenge (HTTP 403) to curl on `/` and a route; the FAQ itself says to self-host. Community instances tested once: two returned valid RSS, two 502, one Cloudflare challenge, one timeout | `[V]` ; `[S]` https://docs.rsshub.app/guide/faqs , https://docs.rsshub.app/guide/instances |

Routes that matter, from the route source `features` flags `[V]`:

| Platform | Route | Needs | Verdict |
|---|---|---|---|
| Telegram | `/telegram/channel/:username` | nothing (uses `t.me/s`); MTProto session optional | Works; same legal question as our own Telegram connector |
| Bilibili | `/bilibili/user/video/:uid` | nothing; dynamic/following routes need a cookie | Good |
| Bluesky, Mastodon | `/bsky/profile/:handle`, `/mastodon/acct`, `/mastodon/tag` | nothing | Native RSS exists; no need |
| YouTube | `/youtube/channel/:id` | Data API key | **Prefer native `youtube.com/feeds/videos.xml?channel_id=`**, tested 200 Atom `[V]` |
| Weibo | `/weibo/user/:uid` | `WEIBO_COOKIES`, Puppeteer, anti-crawler | Fragile |
| Douyin | `/douyin/user/:uid` | Puppeteer, anti-crawler | Fragile |
| Xiaohongshu | `/xiaohongshu/user/:id/notes` | cookie, Puppeteer, anti-crawler | Fragile |
| Zhihu | `/zhihu/activities` | cookie, anti-crawler | Fragile |
| WeChat official accounts | `/wechat/ce`, `/wechat/mp/homepage`, `/wechat2rss`, `/feeddd`, `/sogou` etc. | anti-crawler or a paid relay | Mostly third-party relays |
| X/Twitter | `/twitter/user/:id` | `TWITTER_AUTH_TOKEN` (logged-in session) or paid API keys | Legally risky (see 4.3) |
| Instagram, Facebook | cookie, anti-crawler | | Fragile |
| Dcard | `/dcard/:section` | Puppeteer, anti-crawler | Fragile; Dcard's own site is 403 to curl |
| LINE Today, NHK, Yahoo TW/JP/HK | no flags | | OK, but native feeds exist |
| **PTT, LIHKG** | **no RSSHub route exists** | | Would need a custom scraper |
| Naver | `/naver/search` | | Search only, no news-portal route |

RSSHub also has routes for publishers: Taiwan (cna, udn, ltn, setn, chinatimes, pts, taiwannews, focustaiwan), HK (rthk, scmp), Japan (asahi, kyodo, nikkei), Korea (yna, koreaherald), and 262 government (`gov`) files `[V]`. These are useful where a publisher has no feed but the route scrapes HTML.

**Anti-scraping breakage.** The cookie/Puppeteer routes are the ones that break; there is no published breakage rate. Bypassing the platform walls requires residential proxies and burner accounts (the README promotes a proxy vendor), which is both an ops cost and a ToS violation. **Legal:** the RSSHub docs say nothing about legality. AGPL only matters if you modify and expose it as a service.

**Recommendation.** If a bridge is wanted at all: one self-hosted RSSHub (no Chromium, Redis for cache, `ACCESS_KEY` set), no-login routes only, output consumed by the existing RSS connector. Cost: one small container. Defer all cookie/Puppeteer routes.

### 4.2 RSS-Bridge

- `RSS-Bridge/rss-bridge`: Unlicense, about 9.3k stars, PHP (7.4+), about 97 MB Docker image, no Redis/browser needed, 276 open issues `[V]` https://github.com/RSS-Bridge/rss-bridge .
- The official instance lists 548 bridges and bans commercial use ("Non-commercial usage only", max 10k requests/24h) `[V]` https://rss-bridge.org/bridge01/?action=list .
- `TelegramBridge` returned valid Atom `[V]`; `TwitterBridge` returned HTTP 500 `[V]`. I found no Weibo/Bilibili/Zhihu/Xiaohongshu bridges.
- Verdict: lighter than RSSHub, but no CJK-platform coverage. Not needed.

### 4.3 X/Twitter, Nitter, mirrors

- `nitter.net` returns HTTP 410 with a notice that X Corp sent cease-and-desist letters on 2026-08-24 and 2026-09-08; `xcancel.com` returns HTTP 451 "service is suspended… ongoing legal proceedings"; other instances were bot-walled, redirecting or dead `[V]`. Press: https://techcrunch.com/2026/08/25/x-sends-cease-and-desist-to-open-source-project-nitter-over-alleged-scraping/ , https://reclaimthenet.org/nitter-goes-dark-after-x-corp-cease-desist-letters `[S]`. A September report that XCancel was back is stale `[V]` (https://thenextweb.com/news/nitter-continues-xcancel-back-online-x-cease-and-desist).
- `syndication.twitter.com` returned 429 on first request `[V]`.
- **Official API is pay-per-use**: $0.005 per post read, $0.010 per user read, reads capped at 3M posts/month `[S]` https://docs.x.com/x-api/getting-started/pricing . Free tier effectively gone (secondary: https://gigazine.net/gsc_news/en/20260209-x-api-pay-per-use). Example: 100 accounts × 20 new posts/day = 2,000 reads/day, about $10/day or $300/month (my arithmetic).
- **Recommendation:** do not build on scraping or Nitter. If X matters, buy a small API budget for a hand-picked account list, else skip.

### 4.4 Other bridges and hosted services

| Service | Finding | Evidence |
|---|---|---|
| YouTube native RSS | `https://www.youtube.com/feeds/videos.xml?channel_id=…` returned valid Atom; free, no key, no transcripts | `[V]` |
| Telegram `t.me/s/<channel>` | Returned 20 messages for a test channel; `t.me/robots.txt` 404. **Telegram's terms now prohibit scraping/aggregation and AI/ML use**, dated by Open Terms Archive to 2026-02-03 | `[V]` ; `[S]` https://telegram.org/tos/content-licensing , https://opentermsarchive.org/en/memos/telegram-prohibits-collecting-data-for-ai-use/ |
| Telegram MTProto | Needs api_id and a phone account; ban risk; undocumented flood limits | `[S]` https://core.telegram.org/api/obtaining_api_id , https://core.telegram.org/api/terms |
| openrss.org | Nonprofit; **terms prohibit commercial use and redistribution** | `[S]` https://openrss.org/terms ; site up `[V]` |
| rss.app | Free: 2 feeds, 24 h refresh; Basic about $8-10/mo; Pro about $83/mo with API and 500 feeds, 15-minute refresh | `[S]` https://rss.app/pricing |
| Inoreader | Pro about $7.50-10/mo; reader, not a pipeline feed generator | `[S]` https://inoreader.com/pricing |
| FetchRSS, PolitePol, Kill the Newsletter | Pricing pages 404 or untested | `[U]` |
| PTT | Cloudflare challenge even for its native `/atom/<board>.xml`; unreachable without a browser | `[V]` |

**Connector mapping:** everything above that outputs RSS/Atom is the existing RSS connector. Only an X API reader or a PTT/Dcard/LIHKG scraper would be new code.

---

## 5. Taiwan and East Asia specifics (category 3)

All feeds below were fetched once on 2026-10-13 with the Choir UA `[V]`. "Excerpt" = description is a snippet; "Full" = body is in the feed. Dates are newest item at fetch time.

### 5.1 Taiwan, Traditional Chinese (zh-TW)

| Feed | Items | Newest | Body |
|---|---|---|---|
| CNA politics `https://feeds.feedburner.com/rsscna/politics` (also `/mainland` cross-strait, `/intworld`, `/finance`, `/technology`, `/social`) | 20 each | 10-13 | excerpt ~110 chars |
| LTN all `https://news.ltn.com.tw/rss/all.xml` (also `/politics.xml`, `/world.xml`, `/business.xml`) | 40 | 10-13 | excerpt |
| UDN `https://udn.com/rssfeed/news/2/6638?ch=news` (also 6639) | 20 | 10-13 | title + tiny snippet |
| PTS (公視) `https://news.pts.org.tw/xml/newsfeed.xml` (Atom) | 25 | 10-13 | excerpt ~170 |
| Newtalk `https://newtalk.tw/rss/all` | 100 | 10-13 | HTML body, likely full |
| RTI `https://www.rti.org.tw/rss` | 158 | 10-13 | ~880 chars |
| ETtoday `https://feeds.feedburner.com/ettoday/news` | 50 | 10-13 | ~380 |
| Yahoo TW `https://tw.news.yahoo.com/rss` (+ `/rss/politics`) | 100 | 10-13 | excerpt |
| Global Views `https://www.gvm.com.tw/rss` | 20 | 10-13 | excerpt |
| The Reporter `https://www.twreporter.org/a/rss2.xml` | 10 | 10-11 | excerpt |
| iThome `https://www.ithome.com.tw/rss` (and `/rss/security`) | 30 | 10-13 | excerpt |
| TechNews 科技新報 `https://technews.tw/feed/` (also `/category/semiconductor/feed/`) | 40 | 10-13 | excerpt |
| DigiTimes Chinese `https://www.digitimes.com.tw/tech/rss/xml/xmlrss_10_0.xml` | 30 | 10-12 | ~220 |
| Taiwan FactCheck Center `https://tfc-taiwan.org.tw/feed` | 10 | 10-12 | ~710 |
| MyGoPen `https://www.mygopen.com/feeds/posts/default` (Atom) | 25 | 10-13 | **Full** |
| Executive Yuan `https://www.ey.gov.tw/RSS_Content.aspx?ModuleType=1` | 50 | 10-13 | ~900 |
| Presidential Office `https://www.president.gov.tw/RSSNEWS.aspx` | 9 | 10-13 | short |
| MOFA news (open data) `https://www.mofa.gov.tw/OpenData.aspx?SN=8ABA444B9AC1360F` | 1000 | 10-12 | ~875; Open Government Data License v1.0 per https://data.gov.tw/en/datasets/7668 |
| TWSE material announcements (JSON) `https://openapi.twse.com.tw/v1/opendata/t187ap04_L` | 102 rows for 10-12 | daily | structured fields; needs JSON-diff connector |

### 5.2 Taiwan, English and trade

| Feed | Items | Newest | Body |
|---|---|---|---|
| Focus Taiwan `https://feeds.feedburner.com/rsscna/engnews/` (linked from focustaiwan.tw) | 51 | 10-13 | excerpt ~230 |
| Taipei Times `https://www.taipeitimes.com/xml/index.rss` | 50 | 10-13/14 | title only |
| DigiTimes English `https://www.digitimes.com/rss/daily.xml` | 83 | 10-13 | ~350, has categories |
| Taiwan AI Labs `https://ailabs.tw/feed/` | 10 | 2026-09-10 | infrequent |

### 5.3 Japan

NHK (ja) `https://www.nhk.or.jp/rss/news/cat0.xml` (**new path; the old `www3.nhk.or.jp` path is what the registry uses and it has been frozen since 2026-08-08**), Japan Today, Mainichi English and Mainichi Flash (ja), Asahi headlines (RDF), Yomiuri latest (RDF), Jiji ranking, Yahoo Japan topics, Nippon.com, Tokyo Reporter, The Diplomat (incl. East Asia and SE Asia regional feeds), PC Watch, Gizmodo Japan. Most Japanese feeds are **title-only** (Asahi, Yomiuri, Mainichi Flash, Jiji) so body must be fetched from the article, which has copyright implications.

### 5.4 Korea

Yonhap English and Korean (`https://en.yna.co.kr/RSS/news.xml`, `https://www.yna.co.kr/rss/news.xml`, plus `/politics.xml`, `/northkorea.xml`), Chosun (`.../arc/outboundfeeds/rss/?outputType=xml`, ~380 chars), Hankyoreh, Kyunghyang, Dong-A, Maeil Business, Korea Economic Daily, Segye, Newsis (`/RSS/sokbo.xml`, 100 items, ~1.1k chars), ETNews, **The Elec** (semiconductors, Korean), Business Korea, KED Global, KBS World, Korea Times, **NK News** (300 items), **38 North** (full text).

### 5.5 China, Hong Kong, Chinese-language international

| Feed | Lang | Body |
|---|---|---|
| RFA Mandarin `https://www.rfa.org/mandarin/rss2.xml` | zh | **Full** |
| The Initium `https://theinitium.com/rss` | zh | **Full** |
| BBC Chinese (trad/simp), DW Chinese `https://rss.dw.com/xml/rss-chi-all`, RFI Chinese, VOA Chinese `https://www.voachinese.com/api/`, FT Chinese | zh | excerpts; VOA newest was 19 h old, check cadence |
| Hong Kong Free Press `https://hongkongfp.com/feed/` | en | **Full** |
| RTHK Chinese and English | zh-HK / en | ~260 / ~1.1k |
| CGTN world/china `https://www.cgtn.com/subscribe/rss/section/world.xml` (state media) | en | **Full** |
| Sixth Tone, China Digital Times (full), TechNode, Pandaily | en | mixed |
| Tech in Asia `https://www.techinasia.com/feed` | en | works only with a browser UA; skip |

**Dead/blocked PRC and official surfaces `[V]`:** Xinhua English feeds (newest items 2017-2018), Xinhua Chinese (last item 2022-12-14), People's Daily (2025-06 and 2011), Global Times (widget feed with unreliable dates), PRC MFA (maintenance page, no feed), Taiwan Affairs Office, MND (both sides), MOFCOM, CAC, Yicai, Jiemian, The Paper, China Daily (no feed found), Caixin (403). **PRC official statements need a different method than RSS** (HTML change detection on a handful of pages, or a GDELT/Google News route).

### 5.6 Southeast Asia and think tanks

Rappler (full text), Inquirer, Bangkok Post, VnExpress (vi) and VnExpress International, Vietnam News (**UTF-16 encoded**: the connector's charset handling must decode it), Lowy Interpreter, ISEAS Perspective (full text). East Asia Forum 403.

### 5.7 Semiconductor and hardware press

| Feed | Status |
|---|---|
| DigiTimes (en, zh) | Live, excerpts. The only fresh Taiwan-supply-chain trade feed |
| **TrendForce** `https://www.trendforce.com/news/rss` | **Valid but frozen at 2026-07-01** `[V]`; no press-center RSS (404). Treat as unreliable; use a page-diff or Google News query on `site:trendforce.com` instead |
| The Elec (ko) | Live |
| TechNews semiconductor (zh), Tom's Hardware (full text), EE News Europe (full), CSET (full, slow), ChinaTalk (full), SemiAnalysis Substack (full for free posts), Sinocism, Pekingnology | Live |
| TSMC newsroom, SEMI, SIA | 403 on every tested path |
| Asianometry | Stale (newest 2025-01-21) |

### 5.8 Fact-checkers and disinformation orgs

| Org | Access | Note |
|---|---|---|
| Taiwan FactCheck Center | RSS live `[V]` | |
| MyGoPen | Atom live, full text `[V]` | |
| Cofacts | `cofacts.tw` and `api.cofacts.tw/graphql` returned 403; open dataset on GitHub/Hugging Face (`Cofacts/line-msg-fact-check-tw`, snapshot 2026-10-11). **License conflict:** an older note says CC0, the Hugging Face API tags it CC-BY-SA 4.0, and the README returned "restricted, must authenticate" | `[V]` partial; resolve license and access before building |
| Doublethink Lab, IORG, Taiwan Information Environment Research Center | No RSS (`/feed`, `/index.xml` 404; one host did not resolve) | `[V]`; track via newsletter or page diff |

### 5.9 Robots and reuse terms for the major Taiwanese outlets `[V]` (robots.txt read) / `[S]`

- **CNA** RSS terms (https://www.cna.com.tw/about/rss.aspx): RSS content is for personal and non-profit, non-commercial use; must cite 中央通訊社; only titles, summaries, links and lead-image links are supplied; reproducing full articles infringes; CNA can end the service at any time. `robots.txt` carries `Content-signal: search=yes, ai-input=yes, ai-train=no`. **Whether Choir's use is "non-commercial" is unclear; an email asking permission is cheap and advisable.**
- **UDN** robots.txt says content is available only under its terms and bars LLM/ML/AI and commercial uses.
- **Newtalk** robots.txt declares `ai-input=no, ai-train=no`, but its feed carries full HTML bodies (policy tension: store excerpt only).
- **LTN** robots.txt has separate groups for GPTBot, ClaudeBot, CCBot and others (rules under the group cut off in my read).
- **DigiTimes** disallows GPTBot and lists `rss/daily.xml` as its sitemap.
- Public broadcasters and Dcard/LIHKG/Yahoo/DW/BBC/RFI terms were not read.

**Block pattern note.** Most 403s persisted with a browser UA, so they look like IP/fingerprint blocks from the test network. Retest from the real egress.

---

## 6. Global South, Africa, India, development (category 4)

All tests 2026-10-13 `[V]` unless marked. "Full" = full article text in the feed; "exc" = excerpt.

**Gotchas for connector design.**
- **Bot walls** returned 403 to the polite UA for AfDB, Al-Ahram, APA News, Citi, News24, Quartz, Addis Standard, TheCable, Guardian NG, Monitor UG, The EastAfrican, Dhaka Tribune, New Age BD, Ekantipur, OECD, UNHCR (Cloudflare); Devex (DataDome); PIB, Anandabazar, NITI, Firstpost, News18 (Akamai "Access Denied"). **ReliefWeb RSS/Atom, OCHA site feeds and `data.humdata.org/feeds/rss` return an AWS WAF challenge (HTTP 202, empty body).** Spoofing a browser UA is not recommended.
- Send `Accept: */*` to JSON APIs (`api.first.org` returned 400 for an RSS-style Accept header).

### 6.1 Multilingual feeds with the best value

| Source | Languages | Notes |
|---|---|---|
| **BBC language services** `https://feeds.bbci.co.uk/<service>/rss.xml` | hindi, tamil, marathi, telugu, bengali, gujarati, punjabi, urdu, nepali, sinhala, swahili, hausa, amharic, afaanoromoo, somali, tigrinya, yoruba, igbo, pidgin, gahuza, afrique (fr), arabic, persian, portuguese, mundo (es), burmese, kyrgyz; 9-61 items each, fresh | 31 feeds, **none in the registry**. Excerpts only. I could not fetch BBC's RSS terms; they are believed to restrict reuse to personal non-commercial use `[U]`, check https://www.bbc.co.uk/usingthebbc/terms/ before production |
| **Global Voices** `https://globalvoices.org/feed/` (+ `es.`, `fr.`, `ar.` subdomains) | en, es, fr, ar | Full text, **CC BY** (attribution, link, indicate changes) `[S]` https://globalvoices.org/about/global-voices-attribution-policy/ ; Swahili edition stale since 2022 |
| DW language feeds | Registry has ar, de, en, ru; my guesses at DW Swahili/Hausa/Amharic/Hindi/Bengali/Urdu feed names failed | Real codes unknown `[U]`; look up on dw.com RSS page |

### 6.2 Africa (publisher RSS, live)

- **Kenya (Nairobi urbanization vertical):** Nation `https://nation.africa/kenya/rss.xml` (titles and links only), Standard `https://www.standardmedia.co.ke/rss/headlines.php`, Capital FM `https://capitalfm.africa/news/feed/` (full), Kenyans.co.ke `/feeds/news`, Tuko `/rss/all.rss`, KNBS `https://www.knbs.or.ke/feed/` (official statistics, low volume). The Star Kenya 404; Kenya Gazette 403.
- **Nigeria:** Premium Times, Punch `https://rss.punchng.com/v1/category/latest_news`, Vanguard (full), Channels (full), Daily Trust (full), Leadership (full), Nairametrics, BusinessDay, Sahara Reporters. TheCable and Guardian NG blocked.
- **South Africa:** Mail & Guardian `https://mg.co.za/rss/` (50 items), GroundUp `https://www.groundup.org.za/sitenews/rss/`, SABC (full), IOL, TechCentral. (Daily Maverick already in registry; News24 blocked.)
- **Ghana:** MyJoyOnline (50, full), Graphic, Ghanaian Times (full). **Senegal:** APS. **Mozambique:** O País (pt). **Ethiopia:** Ethiopian Monitor (slow), The Reporter Ethiopia (full), Fana (full). **Tanzania:** Daily News (full). **Zambia:** Lusaka Times. **Egypt:** Daily News Egypt, Egypt Independent, Mada Masr (all full), Youm7 and Al-Masry Al-Youm (Arabic).
- **Pan-African and development:** The Africa Report, African Arguments (full), The Elephant (full), TechCabal (full), Mongabay (full), Dialogue Earth (full), The New Humanitarian `https://www.thenewhumanitarian.org/rss/all.xml` (note: `/rss.xml` starts in July), Crisis Group `https://www.crisisgroup.org/rss`, ECOWAS (full). AU.int stale.

### 6.3 India and South Asia (live)

Hindi: Dainik Bhaskar (57 items, full), Amar Ujala, Aaj Tak (145), NDTV Khabar, Zee Hindi. Tamil: Hindu Tamil, Daily Thanthi, Dinamani (full, via Quintype `stories.rss`). Malayalam: Mathrubhumi. Telugu: Sakshi. Bengali: Sangbad Pratidin (full), ABP Bengali, Prothom Alo (full). English: Scroll.in, The Federal, Mint, Hindu BusinessLine, Hindustan Times, The Better India, Newslaundry (1 item at test). Government/regulator: **RBI press releases, SEBI, MyGov** live; **PIB** blocked by Akamai for the polite UA; MEA and PRS no RSS. Pakistan: Dawn, Express Tribune, The News, Geo, ProPakistani, Jang/Express (Urdu). Nepal: Online Khabar, Kathmandu Post. Sri Lanka: EconomyNext, Ada Derana. Bangladesh: Prothom Alo; Daily Star stale since June. Dead/blocked: The Wire, Caravan, Jagran, Eenadu (410), Lokmat, Tribune India. Indian Parliament bills API (`https://sansad.in/api_rs/legislation/getBills?...`) returns 10,099 records (JSON).

### 6.4 Development and humanitarian JSON APIs (no key; all 200 today)

| Endpoint | What |
|---|---|
| `https://api.hpc.tools/v1/public/fts/flow?year=2026&limit=1`, `.../emergency/year/2026` | OCHA Financial Tracking: flows ($22.4B across 15,795 flows in 2026), emergencies |
| `https://data.humdata.org/api/3/action/package_search?q=cholera&rows=1` | HDX CKAN; per-dataset licence in `license_id` |
| `https://api.unhcr.org/population/v1/population/?limit=1&year=2024` | UNHCR statistics |
| `https://api.worldbank.org/v2/country/KE/indicator/NY.GDP.MKTP.CD?format=json` ; `https://search.worldbank.org/api/v2/news?format=json&rows=2` ; `.../projects` | World Bank indicators, news (32,141 docs), projects (22,729) |
| `https://www.imf.org/external/datamapper/api/v1/NGDP_RPCH/KEN` | IMF DataMapper |
| `https://fdw.fews.net/api/` | FEWS NET index; resource queries timed out |

**ReliefWeb API v2** returns 403 "not an approved appname" until you register one; docs allow 1,000 calls/day and 1,000 rows/call and say site text is CC BY 4.0 with possible third-party copyright `[S]` https://apidoc.reliefweb.int/ . **Request an appname now: it is the best structured humanitarian source and the RSS is walled.** Dead: IMF news/blog RSS, WB blog RSS, FAO/WFP/IOM/UNICEF guessed URLs (real paths unknown).

### 6.5 WhatsApp Channels and community radio

- **WhatsApp Channels:** no public API. Gateways such as Whapi.Cloud/WAHA read Channels by linking a WhatsApp account, which almost certainly breaches WhatsApp's terms `[S]` https://whapi.cloud/whatsapp-channels , https://waha.devlike.pro/whatsapp-channels . No legal path at scale; follow the same publishers via RSS/website/Telegram.
- **Community radio:** no structured transcript source exists that I could find. Radio Browser API lists 61,525 stations `[V]` https://de1.api.radio-browser.info/json/stats (directory only). The Internet Archive `radio` collection (about 3.8M items with auto speech-to-text) sampled US stations only. Self-transcription: third-party pages quote about $0.003-0.006 per audio minute (https://convertaudiototext.com/blog/openai-whisper-api-pricing-2026, unverified), so one 24/7 station is about $4.3-8.6/day, ten stations $43-86/day (my arithmetic). Low-resource-language accuracy and broadcast copyright are unverified. **Not recommended at this budget.**

### 6.6 Telegram ecosystems in the Global South

Previews that work today `[V]`: `theprintindia`, `fanatelevision` (Ethiopia), `bbcpersian` (already in registry), and cyber channels below. Stale: `news24` (Jul 2024), `BBCAfrica` (Jul 2024), `ethiopianreporter` (Apr 2025). Many guessed channel names (aajtak, ANI_official, bbchindi, PIBHindi, nationafrica, punchnewsng etc.) have **no web preview**. Telegram's scraping prohibition (Section 11) applies here too.

---

## 7. Cybersecurity (category 5)

All tests 2026-10-13 `[V]`.

### 7.1 Vulnerability data (JSON/CSV; a "JSON-diff" connector)

| Source | Result | Licence / rate |
|---|---|---|
| **CISA KEV** `https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json` | 1,740 entries, catalog 2026.10.13; 43 added in Sep, 10 so far in Oct (about 1.4/day) | CC0 per https://raw.githubusercontent.com/cisagov/kev-data/develop/README.md |
| **NVD API 2.0** `https://services.nvd.nist.gov/rest/json/cves/2.0?...` | 405,540 CVEs; a 6-hour `lastModStartDate` window returned 38 | 5 req/30 s without key, 50 with key; updates no more than every 2 hours `[S]` https://nvd.nist.gov/developers/start-here |
| CVE Services `https://cveawg.mitre.org/api/cve-id/...`; `cvelistV5` deltaLog (20 MB, do not poll) | works | CVE Program terms; use GitHub releases/sparse clone instead |
| **EPSS** `https://api.first.org/data/v1/epss?cve=...` and daily CSV | 385,987 scores for 2026-10-12 | free with attribution `[S]`; rate limit unverified |
| **GitHub Advisory DB** `https://api.github.com/advisories` | works; 60 req/hour unauthenticated | CC BY 4.0 |
| **OSV** `https://api.osv.dev/v1/vulns/...` | works; bulk file 148 MB (use per-ecosystem dumps) | per-source: GHSA CC-BY 4.0, some CC0, Ubuntu CC-BY-SA |
| CERT/CC Atom, MSRC update-guide RSS (4,037 items; entries are near-empty "Information published" + CVE), HIBP breaches API (1,044), Shodan InternetDB, GreyNoise community | work | licences unverified |
| OTX, VulnCheck KEV, abuse.ch URLhaus API, ransomware.live PRO API | **need keys** (401/403) | |

### 7.2 Vendor advisories and research blogs (RSS/Atom; no code)

CISA `all.xml` and `ics-advisories.xml` (full text), Cisco PSIRT, Fortinet, Palo Alto (titles only), Ivanti blog, Chrome Releases, **Project Zero** (`https://projectzero.google/feed.xml`), ZDI published (about 3/day), Exploit-DB, Seclists oss-sec, SANS ISC, **Talos**, **Mandiant/GTIG** (`https://cloudblog.withgoogle.com/topics/threat-intelligence/rss/`), Unit 42, ESET WeLiveSecurity, Securelist, Check Point Research, watchTowr, Rapid7, Microsoft Security Blog, Schneier, The Record, CyberScoop, The Hacker News, Help Net Security, Security Affairs, Hackread, GBHackers, CybersecurityNews, DataBreaches.net, CIRCL. (BleepingComputer, Dark Reading, SecurityWeek, Krebs, The Register already in registry.) Dead/HTML-only: Citrix, Ivanti advisories, Sophos, Ars security, The Cyber Express, cvedetails.

### 7.3 CERTs worldwide (live)

NCSC UK, **CERT-EU** (CC BY 4.0 per https://cert.europa.eu/legal-notice), CERT-FR `avis`, **BSI CERT-Bund WID (about 230 advisories/day; filter by severity)**, **JPCERT/CC (ja) and JVN (en)**, **TWCERT/CC** `https://www.twcert.org.tw/tw/rss-104-1.xml` (zh, newest 09-30; other numbered feeds redirect to HTML), **KrCERT** `https://knvd.krcert.or.kr/rss/security/notice` (ko), CERT-UA, CERT.PL, NCSC-NL, **HKCERT**, CERT.br. Not working: CERT-In (no RSS), CSA Singapore, ACSC/AusCERT, ENISA, CNVD (HTTP 521), CNNVD (403), CCCS. Taiwan cyber news: iThome security `https://www.ithome.com.tw/rss/security`.

### 7.4 Ransomware and IOC trackers

| Source | Finding |
|---|---|
| **ransomware.live** RSS `https://www.ransomware.live/rss.xml` | 200 items in about 4 days (about 50/day). **ToS: free for non-commercial use with attribution; commercial use needs written approval; re-serving the data through your own API or feed is prohibited; cached 30 min, more frequent polling counts as a breach** (https://www.ransomware.live/t&c, updated 2026-08-19). Autopaper would need written approval |
| RansomLook `https://www.ransomlook.io/api/recent` | 100 entries per ~26 h; software is GPL-3.0, data terms unstated `[U]` |
| ransomwatch (GitHub) | Dead (newest 2025-06-16) |
| abuse.ch (URLhaus, MalwareBazaar, ThreatFox, Feodo) | Keyless bulk exports returned 200 today (for example ThreatFox 6,896 IOCs) but the docs say an Auth-Key is required and ask for no more than one fetch per 5 minutes; **do not assume CC0**; commercial users may need a paid tier `[S]` https://urlhaus.abuse.ch/api/ , https://threatfox.abuse.ch/faq/ |
| ecrime.ch, Shadowserver feed, HHS breach portal CSV | not reachable/not checked |

### 7.5 Social sources for cyber

mastodon.social tag RSS (`ransomware`, `cve`, `infosec`) returns 20 items within minutes; **infosec.exchange and ioc.exchange tag RSS return empty channels** (their API requires auth, HTTP 422), but `https://infosec.exchange/@briankrebs.rss` works (per-account). Reddit `r/netsec/.rss` works (25 items, short); Reddit `.json` is 403. Telegram previews that work: `thehackernews`, `vxunderground`, `ransomlook`.

**Connector mapping for cyber.** About 80% is plain RSS (vendors, CERTs, blogs). KEV, NVD, EPSS, GHSA, OSV, TWSE announcements need the JSON-diff connector: one small shared poller with a per-source JSON-path mapping (id, title, url, published) would serve all of them.

---

## 8. Aggregators and event databases (category 6)

*Tests 2026-10-13. `[V]` tested, `[D]` fetched documentation, `[S]` search snippet only, `[?]` could not verify.*

### 8.1 GDELT: what it does and does not give

| Item | Finding | Evidence |
|---|---|---|
| Licence | "Unlimited and unrestricted use" for academic, commercial or governmental purposes; redistribution allowed; **cite GDELT and link** | `[D]` https://www.gdeltproject.org/about.html |
| Languages | About page: "over 100 languages". DOC 2.0 launch post: 65 languages machine-translated into English, covering 98.4% of non-English volume | `[D]` same; https://blog.gdeltproject.org/gdelt-doc-2-0-api-debuts/ |
| What you get | Metadata only: URL, source domain, themes, entities, locations, tone, translation info. **No article text** | `[V]` GKG files |
| **File feeds (what Choir already uses)** | `lastupdate.txt` (English) and `lastupdate-translation.txt` (machine-translated foreign-language stream). Both list export, mentions and GKG zips and 301-redirect HTTP to HTTPS | `[V]` https://data.gdeltproject.org/gdeltv2/lastupdate.txt |
| GKG size and shape | 27 tab-separated columns; col 4 = source domain, col 5 = URL, col 26 = TranslationInfo (e.g. `srclc:zho;eng:GT-ZHO 1.0`). English slot: about 6.6 MB zipped, 1,552 rows, 265 distinct domains, **zero `.tw` domains**. Translation slot: about 9.3 MB zipped, 2,905 rows, **48 source languages** (top: Spanish 506, Italian 293, German 287, Russian 257, Portuguese 184, Greek 174, French 139, Chinese 124) | `[V]` slot 17:00 UTC |
| Taiwan depth | Only 4 translation-slot rows from `cna.com.tw` in that 15-minute slot. **GDELT will not give Taiwan depth; direct feeds are required** | `[V]` |
| DOC 2.0 API | Query operators `sourcelang:`, `sourcecountry:`, `domain:`, `theme:`, `tone`, `near#:`; maxrecords default 75, max 250; timespan minimum 15 min, window the last 3 months | `[D]` blog post above |
| DOC API reality | **Every request from my IP returned HTTP 429** (5 attempts over about 15 minutes, 9-11 s each); body asks for no more than one request per 5 seconds and suggests the ngrams dataset for high-traffic users. I never got a 200 | `[V]` |
| Operational quirk | At 17:52 UTC `lastupdate.txt` advertised 18:00 files that returned 404; the newest retrievable were the 17:00 slots. Cause unknown. A connector should retry and fall back to the previous slot | `[V]` |
| Outage history | Network outage 2025-06-14 to 2025-07-01 (40-60 fewer records in one downstream dataset); no 2026 notice found | `[D]` https://gotech.umd.edu/node/13294 |
| BigQuery | Reported free tier 1 TB queries/month then about $5-6.25/TB; cap with `maximum_bytes_billed`. GKG table sizes not found | `[S]` https://airbyte.com/data-engineering-resources/bigquery-pricing |
| Bias | Over-indexes English-language and globally indexed outlets; local coverage uneven | `[D]` https://gdeltcloud.com/methodology |

**Observations about the current Choir GDELT connector (read from `internal/sources/gdelt.go` and `registry.json`).** (1) It consumes only the English `lastupdate.txt`. The registry row has `max_items_per_poll: 500` while the English GKG slot I measured had 1,552 rows, so roughly two thirds of each slot may be dropped or sampled; I did not trace which rows are kept. (2) The item `Body` is a string of themes/organizations/locations, which is metadata, matching `BodyKindMetadataPacket`. (3) The parser matches filenames containing `gkg.csv.zip`, which the translation stream's `*.translation.gkg.csv.zip` also contains, so **adding a second registry row pointing at `lastupdate-translation.txt` probably works with no code change** (untested; the translation file's column layout was the same 27 columns in my sample `[V]`). Filtering to Taiwan/East Asia (domain suffix `.tw/.jp/.kr/.hk`, `srclc:zho/jpn/kor`, or theme/entity match) before storing would be a small code change and is the cost control that matters (2,905 translation rows per 15 minutes is about 280,000 per day).

### 8.2 Google News RSS (the best cheap multilingual lead generator, with a ToS problem)

| Item | Finding | Evidence |
|---|---|---|
| Works | `https://news.google.com/rss/search?q=台灣&hl=zh-TW&gl=TW&ceid=TW:zh-Hant` returned 104 items; ja-JP 100; ko-KR 100; zh-CN 101; en-US 102. `+when:1d` and `site:cna.com.tw` operators worked (100 items); `+when:1h` returned 9 | `[V]` |
| Item shape | title, pubDate, `<source url=…>` publisher, HTML description with related links | `[V]` |
| Link problem | Links are `news.google.com/rss/articles/CBM…?oc=5`, not publisher URLs. Decoding needs the article page plus a POST to `news.google.com/_/DotsSplashUi/data/batchexecute` (rpc `Fbv4je`); 2 extra requests per link; resolved 3 of 3 | `[V]` |
| **ToS/robots** | `robots.txt` has `User-agent: *` / `Disallow: /` with a short allow-list that does not include `/rss`; Google's Terms of Service forbid "using automated means to access content from any of our services" including robots.txt violations. Reports of 503 blocks on automated access | `[V]` robots; `[D]` https://policies.google.com/terms ; `[S]` https://community.make.com/t/google-rss-feed-blocked-503-error-automated-access-blocked-any-workarounds/110931 |

**Verdict:** treat as an optional low-volume lead generator behind an explicit owner decision, not as a backbone. Use only the headline, publisher and pubDate, then fetch the publisher's own page or feed.

### 8.3 Other aggregator-like surfaces

| Surface | Finding | Evidence |
|---|---|---|
| Google Trends RSS | `https://trends.google.com/trending/rss?geo=TW` returned 10 items with approximate traffic and news URLs. Terms not checked | `[V]` |
| Bing News RSS | `bing.com/news/search?q=台灣&format=rss&setmkt=zh-TW` returned 12 items; links are `apiclick.aspx?...&url=<encoded>` redirects. The Bing Search API was retired 2025-08-11. ToS not checked | `[V]` ; `[S]` https://learn.microsoft.com/en-us/lifecycle/announcements/bing-search-api-retirement |
| Yahoo TW News RSS | 100 items with direct article URLs | `[V]` |
| Reddit | `r/taiwan/.rss` returned 25 Atom entries; `.json` returned 403. Free API tier reportedly non-commercial, self-service registration closed, commercial around $0.24 per 1,000 calls (conflicting reports) | `[V]` ; `[S]` https://octolens.com/blog/reddit-api-pricing |
| Hacker News Algolia | `hn.algolia.com/api/v1/search_by_date?query=taiwan` 200 JSON | `[V]` |
| Polymarket | `gamma-api.polymarket.com/public-search?q=taiwan` returned markets with prices (e.g. "China x Taiwan military clash before 2027?" at 4.75% Yes, $3.47M volume). API terms not checked. The repo already has a `polymarket` source-type constant | `[V]` |
| Kalshi, SEC EDGAR Atom, GovInfo | respond 200 | `[V]` |
| **Taiwan MND daily PLA activity** `https://www.mnd.gov.tw/news/plaactlist` | Daily entry "中共解放軍臺海周邊海、空域動態" (e.g. 2026-10-13: aircraft 5, of which 4 crossed the median line; PLA Navy ships 6; official ships 4). **No RSS**; entry pages like `/news/plaact/87976` are scrapeable; robots `Disallow: /*?*` is addressed to Googlebot only | `[V]` |
| PLATracker (CSIS ChinaPower) | Google Sheet download, no licence stated; 2025 total 3,762, 2024 total 3,075, record 111 aircraft on 2024-10-14 | `[D]` https://chinapower.csis.org/data/taiwan-adiz-violations/ ; `[S]` https://www.globalmilitary.net/conflicts/china-taiwan/adiz-violations/ |
| Japan MOD Joint Staff press | Cloudflare challenge; RSS path 403 | `[V]` |

### 8.4 Media Cloud, Common Crawl, commercial APIs, ACLED

| Source | Key facts | Verdict for Choir | Evidence |
|---|---|---|---|
| **Media Cloud** | API key; default 4,000 requests/week, some endpoints 2/min; "open source and open data"; paid plans case-by-case; **"Due to copyright restrictions we cannot release the actual text of a story"** (URLs only); about 20 supported languages; fed by RSS plus sitemaps; directory of curated collections from 100+ countries (source count quoted as 25,000+ on the directory page, 60,000+ in a snippet). Taiwan collection, bulk download and licence not confirmed | Useful as a **source-list reference**, not a text source | `[D]` https://mediacloud.org/documentation/faqs , https://mediacloud.org/media-cloud-directory |
| **Common Crawl CC-NEWS** | WARCs under `crawl-data/CC-NEWS/YYYY/MM/`, daily cadence; September 2026 had 374 WARCs of about 1.07 GB each, about 400 GB/month compressed (about 13 GB/day); Terms: limited non-transferable licence, content belongs to owners, you must respect copyrights and indemnify Common Crawl including for AI training; they advise counsel for commercial use. Tooling: `news-please` (Apache-2.0) | A daily batch filtered to `.tw/.jp/.kr` is feasible, but legal stance makes it **a URL/lead source, not a display source**. No published processing cost; I did not measure | `[V]` file listing; `[D]` https://commoncrawl.org/terms-of-use , https://github.com/fhamborg/news-please |
| **ACLED** | EULA: "Commercial entities may not access or use the Content… without first obtaining a corporate license"; base licence non-commercial; no redistribution or scraping; AI training limited where it substitutes for ACLED products; tiers Open, Research, Partner, Enterprise; API moved from keys to login/OAuth in 2025 | **Skip** unless a corporate licence is bought | `[D]` https://acleddata.com/eula , https://acleddata.com/myacled-faqs |
| **UCDP GED** | CC BY 4.0; GED v26.1; monthly candidate events; API token required, 5,000 requests/day | Optional conflict-event enrichment, lagged and conflict-only | `[D]` https://ucdp.uu.se/downloads/ , https://ucdp.uu.se/apidocs/ |
| NewsAPI.ai (Event Registry) | Free 2,000 searches; $90/mo (5K plan); 50+ languages, full text, archive to 2014 | Best commercial fit for multilingual; redistribution terms not stated on the pricing page | `[D]` https://newsapi.ai/plans |
| NewsAPI.org | Developer $0 (dev only, 24 h delay); Business $449/mo; URL only | Poor fit | `[D]` https://newsapi.org/pricing |
| GNews | Free €0 (non-commercial); Essential €49.99 to Enterprise €249.99/mo | Possible | `[D]` https://gnews.io/pricing |
| NewsCatcher | Starter $50; production is Enterprise-only | | `[D]` https://newscatcherapi.com/pricing |
| Webz.io | $0.001/call + $0.0005/result; 170+ languages | Cheapest pay-per-use for foreign languages; terms for storing/redistribution not stated | `[D]` https://webz.io/pricing/ |
| Mediastack, Currents, TheNewsAPI | $24.99-$249.99/mo; Currents requires custom terms for republishing/long-term caching | Weak for CJK | `[D]` https://mediastack.com/product , https://currentsapi.services/en/product , https://www.thenewsapi.com/pricing |
| Meltwater, Dataminr, Factiva, Aylien/Quantexa | No public list prices (Meltwater reportedly around $10k-130k/yr) | Out of scope | `[S]` https://octolens.com/blog/meltwater-pricing |

Search APIs usable for discovery: Brave about $5 per 1,000 requests `[S]` https://costbench.com/software/ai-search-apis/brave-search-api/ ; Exa $4-7 per 1,000 `[D]` https://exa.ai/pricing ; Tavily $0.008/credit `[D]` https://www.tavily.com/pricing ; Perplexity $5 per 1,000 `[D]` https://docs.perplexity.ai/docs/getting-started/pricing ; Diffbot Extract $299/mo startup `[D]` https://www.diffbot.com/pricing/ . **Chinese-language news coverage of all commercial APIs was not verified.**

### 8.5 Wikipedia and Wikimedia (cheap trending-topic signal)

| Surface | Result | Evidence |
|---|---|---|
| Pageviews top lists | `https://wikimedia.org/api/rest_v1/metrics/pageviews/top/zh.wikipedia/all-access/2026/10/12` returned 200; top included 沈伯洋 (24,718 views) and 921大地震 (10,425), plus main-page and search noise to filter. ja works the same | `[V]` |
| Current events | en: `Portal:Current_events/2026_October_12` via `action=raw` returned 5 KB wikitext with bullet items and source links. zh: `Portal:新聞動態`. ja: `Portal:最近の出来事` (1.9 KB). ko not tested | `[V]` |
| EventStreams | `stream.wikimedia.org/v2/stream/recentchange` delivered SSE immediately; per-connection limits not found | `[V]` |
| Rate/UA rules | Descriptive UA with contact required; anonymous compliant bots 200 requests/min, unidentified 10/min; REST at most 5 req/s, concurrency 3 | `[D]` https://foundation.wikimedia.org/wiki/Policy:Wikimedia_Foundation_User-Agent_Policy , https://www.mediawiki.org/wiki/Wikimedia_APIs/Rate_limits , https://wikitech.wikimedia.org/wiki/Robot_policy |
| Wikimedia Enterprise | Free account: 30 snapshot requests and 1,500 chunks/month, 50,000 on-demand requests/month; paid pricing unpublished | `[D]` https://enterprise.wikimedia.com/pricing/ |
| Licence | Text is CC BY-SA (attribution plus share-alike) | `[?]` general knowledge; licence page not fetched |

**Use:** a daily pageviews-top job per language (zh, ja, ko, en) is a nearly free "what are people reading about" detector for topics (e.g. a Taiwanese legislator spiking), usable to seed keyword queries or priority. Wikidata recent-changes was not tested.

---

## 9. Multilingual handling (category 7)

### 9.1 Translation cost: the numbers

Working assumption for one captured item (title plus feed excerpt): 600 characters of CJK text, about 800 input tokens and 400 output tokens of English. These token ratios are my estimates and vary by tokenizer; replace them with measured counts from `usage` once a pilot runs.

| Option | Published price | Cost per item (my arithmetic) | Per 10,000 items/day | Notes |
|---|---|---|---|---|
| Claude Haiku 5.5 | $0.10 in / $0.50 out per MTok (prompts up to 100K tokens), Message Batches at 50% off | ~$0.00028 (about $0.00014 batched) | ~$2.80/day (~$1.40 batched) | Prices from the claude-api reference bundled with this Claude Code session (cached 2026-10-06); confirm on Anthropic's pricing page before budgeting |
| Claude Sonnet 5.5 | $2 in / $10 out per MTok | ~$0.0056 | ~$56/day | Use only for escalation (hard items, low-resource languages) |
| Gemini Flash-Lite class | Trackers disagree: $0.10/$0.40 (2.5 Flash-Lite), $0.25/$1.50 (3.1 Flash-Lite), $0.30/$2.50 (3.5 Flash-Lite) ([MorphLLM](https://www.morphllm.com/gemini-api-pricing), [anotherwrapper](https://anotherwrapper.com/llm-pricing/gemini-3.5-flash-lite)) | $0.0002-0.0012 | $2-12/day | Third-party price trackers; not verified against Google's page |
| Google Cloud Translation (NMT) | $20 per million characters after 500k free/month ([Google pricing](https://cloud.google.com/translate/pricing)) | $0.012 | ~$120/day | ~40x the Haiku route at these assumptions. Billed characters include HTML markup |
| Google Cloud Translation LLM | $10/M input chars + $10/M output chars (same page) | ~$0.01 | ~$100/day | |
| DeepL API | Sources conflict: new signups reportedly get a one-time 1M-character credit; Growth plan ~$32.50/month base + ~$27.50 per million characters ([simplelocalize](https://simplelocalize.io/blog/posts/ai-machine-translation-cost-comparison/), [eesel](https://www.eesel.ai/blog/deepl-pricing)); old 500k free tier is stale | ~$0.017 | ~$165/day | DeepL does not cover many long-tail languages |
| NLLB-200 self-hosted | Model weights are **CC-BY-NC 4.0** (non-commercial) per the [UNESCO space discussion](https://huggingface.co/spaces/UNESCO/nllb/discussions/12) and the [CTranslate2 forum](https://forum.opennmt.net/t/ctranslate2-converter-for-nllb/5010) | GPU time only | n/a | **Not usable for a commercial product** unless you have separate permission; 200 languages |
| MADLAD-400 3B self-hosted | Apache 2.0 per a [guide](https://soniqo.audio/hi/guides/translate); 400+ languages | GPU time only; I have no measured throughput | n/a | Needs a GPU box (ops burden); break-even against Haiku is far away at <100k items/day (my estimate) |

**Conclusion.** At these prices, a small LLM is cheaper than classic MT APIs by one to two orders of magnitude for headline/excerpt-sized text, and it handles mixed-script slang, transliterated names and metadata extraction in the same call. Self-hosting NLLB or MADLAD is a premature optimization at Choir's scale and NLLB is license-blocked. I found **no benchmark comparing LLM versus dedicated MT on low-resource languages** (could not verify); for Chinese, Japanese and Korean the LLM route is safe, for languages like Hausa, Amharic, Nepali or Tigrinya run a small human spot-check before trusting it.

### 9.2 Translate at capture, or lazily at clustering?

Recommendation: **three-step lazy pipeline, translate only what survives a cheap gate.**

1. **At capture (free):** store the original title and excerpt, run local language ID, set `Item.Language` accurately (today the RSS poller copies the registry's first language string; see `internal/sources/rss.go`, which assigns `Language: firstString(source.Languages)`), and compute a local multilingual embedding of the original text.
2. **At clustering (cheap):** cluster in embedding space across languages. Cross-lingual dedupe does not need translation at all if the embedding model is multilingual.
3. **At story-writing time (only for items that join a story or cross a salience threshold):** translate the title plus lead (at most ~600 characters) with Haiku-class, cache the result keyed by `content_hash`, and keep the original alongside with a "machine-translated" label for citation honesty.

Why not translate everything at capture: most captured items never enter a published story; a pre-translated copy of every item multiplies storage and cost for no reader value. Why not translate nothing until writing time: the Texture/editor LLM can read CJK directly, so for East Asian languages the translation step may be unnecessary at all (it can cite original-language sources and write English). Translation matters most for (a) reader-facing quotes and (b) dedupe against English-language wires. That's an argument for translating **titles only** at capture (about 40 characters, ~$0.00003 per item with a Haiku-class model at my token assumptions), because an English title makes the editor's triage and keyword rules work uniformly across languages. Treat that as a cheap option, not a requirement.

### 9.3 Language identification

- Capture-time language ID should be local and free. **lingua-go** (pemistahl/lingua-go) is a pure-Go library covering 75+ languages, runs offline, and is designed for short and mixed text ([pkg.go.dev](https://pkg.go.dev/github.com/pemistahl/lingua-go), [GitHub](https://github.com/pemistahl/lingua-go)); accuracy claims are the maintainer's own benchmarks. For long-tail languages beyond its 75, fastText-based GlotLID covers ~2,000 languages but I could not verify its license or Go integration cost, so I would not plan on it yet.
- Script detection (Unicode ranges) is a cheaper first pass: Han vs Kana vs Hangul vs Latin. It distinguishes Japanese (kana present) from Chinese, but **does not separate Traditional from Simplified Chinese** reliably; for Taiwan versus PRC provenance use the source's registry metadata (`regions`, `jurisdictions`) as the primary signal and script as a hint. That distinction matters for Autopaper because Traditional vs Simplified framing is itself signal.

### 9.4 Cross-lingual embeddings and clustering

| Model | License | Notes | Source |
|---|---|---|---|
| BGE-M3 | MIT | 568M params, 1,024 dims, 8,192-token input; widely used multilingual all-rounder | [wz-it comparison](https://wz-it.com/en/blog/best-embedding-models-german/) |
| Qwen3-Embedding 0.6B / 4B / 8B | Apache 2.0 | 32k context; 8B reportedly ranked first on MTEB multilingual in June 2025 | same source |
| multilingual-e5-large | MIT | 100 languages, 512-token limit; lightest | same source |
| gemini-embedding-001 | Hosted API | Reported to hold a top multilingual MTEB spot since March 2025 | same source |

I could not verify current prices for hosted embedding APIs, so none are quoted. For Choir: BGE-M3 or Qwen3-Embedding-0.6B on CPU is adequate for a few hundred thousand short items per day (my estimate; benchmark on the actual box). All sources compared were secondary blog posts: **run a 200-pair Taiwan-specific cross-lingual evaluation (zh-TW vs en vs ja headlines about the same event) before choosing.**

Clustering design note: story identity should be keyed on embedding similarity within a time window plus shared named entities (GDELT GKG entities already give this for free), not on translated text.

---

## 10. Source discovery at scale (category 8)

### 10.1 Resource lists

| Resource | Finding | Evidence |
|---|---|---|
| plenaryapp/awesome-rss-feeds | CC0-1.0, pushed 2026-10-05, 132 OPML files; country OPMLs only for Japan (8 feeds) and Hong Kong; **no Taiwan or Korea file** | `[V]` GitHub API |
| xiangyugongzuoliu/awesome-rss-feeds-list | CC0-1.0, pushed 2026-09-08; contents not examined | `[V]` metadata only |
| tuan3w/awesome-rss, osmoscraft/awesome-rss, foorilla/allainews_sources | GitHub API returned Not Found for all three | `[V]` |
| Kagi Small Web | MIT; `smallweb.txt` has 42,772 feed URLs; English-only personal blogs; irrelevant for Taiwan news | `[V]` https://github.com/kagisearch/smallweb |
| ABYZ News Links | Taiwan page `http://www.abyznewslinks.com/taiwa.htm` works over HTTP only, about 57 links, **no RSS URLs**; terms not checked | `[V]` |
| onlinenewspapers.com | `taiwan.htm` 200, 17.7 KB | `[V]` |
| W3Newspapers | 403 | `[V]` |
| Feedspot | Top 15 shown free, "show 16 to 201" link; lists site URLs not feeds; endpoints and terms page 403; scraping ToS unverified | `[V]` |
| **Wikidata** | Newspaper items (Q11032 subclasses) with official website: Taiwan 33 of 44, Japan 109 of 151, South Korea 90 of 121, PRC 168 of 220. Sparse and includes junk | `[V]` SPARQL |
| Media Cloud directory | See 6.4; no documented bulk export | `[D]` |

Wikidata query pattern: `SELECT ?itemLabel ?site WHERE { ?item wdt:P31/wdt:P279* wd:Q11032 ; wdt:P17 wd:Q865 ; wdt:P856 ?site . SERVICE wikibase:label { bd:serviceParam wikibase:language "zh,en". } }` (Q865 = Taiwan; swap the country item).

### 10.2 Autodiscovery tools and measured hit rates

- **Feedsearch API** `https://feedsearch.dev/api/v1/search?url=<site>` worked free without a key (first call 19 s). For `cna.com.tw` it returned the FeedBurner CNA feeds with item count, velocity and score. `[V]`
- **Feedly search** `https://cloud.feedly.com/v3/search/feeds?query=...` returned metadata; terms for automated use not checked. `[V]`
- **Experiment on 16 Taiwan outlet homepages** `[V]`: classic `<link rel="alternate">` autodiscovery found a feed on only **2 of 16** (about 12%). A page-link scan plus probing `/feed`, `/rss`, `/rss.xml`, `/feed.xml`, `/atom.xml`, validated by parsing, found valid feeds for **6 of 16** (PTS, Focus Taiwan, Taipei Times, The News Lens, RTI, Mirror Media). Feeds recalled from memory and then validated worked for LTN, CNA, ETtoday and UDN, while about 30% of recalled URLs were dead or wrong (`setn`, `taiwannews`, `storm`, `tvbs`, `cw` 404/HTML; `ctee` 403; `chinatimes` 502; `ftvnews` TLS failure).
- Conclusion: generic discovery gets about 12-40% of Taiwan outlets; a curated seed list validated by the gates below, plus feedsearch.dev for the remainder, is the practical route. Many Taiwan outlets publish through **FeedBurner URLs** (CNA, ETtoday), which look unrelated to the outlet host.

### 10.3 Validation and pruning pipeline (design)

1. **Fetch gate:** one GET, short timeout, require HTTP 200, feed-like content type or root element `rss/feed/rdf:RDF`, under 2 MB; record redirect target and TLS errors. (Handle UTF-16 feeds: Vietnam News is UTF-16 `[V]`.)
2. **Parse gate:** at least 3 items each with title and link; reject empty-shell feeds (Korea Herald `/rss/news.xml` and VOA Chinese were valid shells with zero items `[V]`).
3. **Freshness gate:** newest item date (fallback HTTP `Date`); flag older than 14 days as stale and re-check weekly; older than 90 days dead. This would have caught the frozen NHK, TrendForce, WHO and Agriculture Dive feeds, all found in this report.
4. **Language gate:** detect on titles; keep zh-Hant and zh-Hans apart; compare with the host's expected language.
5. **Dedupe:** by canonical feed URL after redirects; keep one best feed per host/section; drop a feed when more than 80% of item URLs also appear in a sibling feed; normalise FeedBurner against the origin host.
6. **Cost control:** conditional GET (ETag/Last-Modified; the RSS poller already sends both unless `conditional_request_mode: none`); back off dead feeds (1, 3, 7 days, then monthly); scale poll interval to feed velocity (the registry currently polls 197 of 211 sources at a flat 900 s).
7. **Compliance:** robots.txt per host (record TDM/AI-signal lines such as CNA's `ai-train=no`), identify the bot with contact info, apply `store_body_policy` per host.

A one-shot freshness audit script of the same kind, run today against the 137 RSS registry entries (fetch, count items, take newest date), found the five problem feeds in Section 1 in about two minutes. **A recurring version of that audit is the single cheapest reliability investment** and could report as `Source.Status`.

---

## 11. Legal and ToS risk, by category (category 9)

*This is a risk map for engineering decisions, not legal advice. The repo's current `store_body_policy` registry field (`excerpt_only` for 97 sources, `bounded_text` for 100) is a good existing control; extend it rather than invent a new one.*

### 11.1 Access method ranking (lowest to highest risk)

| Rank | Method | Why | Evidence |
|---|---|---|---|
| 1 | Official bulk data / open licenses (government press releases, CC-BY, ACLED excluded; CISA, NVD, abuse.ch, UN, ReliefWeb with terms) | License grants reuse | per-source, see tiers |
| 2 | Publisher-provided RSS, headlines + excerpts + link back | Publisher published the feed for syndication-style consumption, but terms vary a lot | See below |
| 3 | Official platform APIs under their terms (X pay-per-use, Bluesky authenticated) | Contractual clarity, costs money | [X pricing docs](https://docs.x.com/x-api/getting-started/pricing) |
| 4 | Logged-out reads of public pages (Telegram `t.me/s`, Mastodon public RSS) | Public, but platform terms may prohibit scraping | Telegram prohibition below |
| 5 | Bridges needing logged-in cookies (RSSHub Weibo/Zhihu/Xiaohongshu/X) | Breaches platform terms; accounts can be banned; X now sends legal threats | Nitter shutdown below |
| 6 | Circumventing bot walls or paywalls | Do not do this | n/a |

### 11.2 What the case law and statutes say (as of the sources found)

- **Scraping public pages (US).** *hiQ v. LinkedIn* and *Van Buren* narrowed the CFAA for public data; contract claims remain the main risk ([FBM summary](https://www.fbm.com/alex-reese/publications/what-recent-rulings-in-hiq-v-linkedin-and-other-cases-say-about-the-legality-of-data-scraping/)). *Meta v. Bright Data* (N.D. Cal., Jan 2024) held Meta's terms did not bar logged-out scraping of public data (same source). District-court level, wording-specific: **do not read this as "scraping is legal".** The secondary summaries were not checked against the opinions.
- **Training/using copyrighted text with an LLM (US).** The Third Circuit affirmed on 2026-09-29 that Ross infringed Westlaw headnotes and rejected fair use for that AI-training use; Ross says it will seek Supreme Court review ([Let's Data Science](https://letsdatascience.com/news/third-circuit-rejects-ross-ai-training-fair-use-abdd1d03), [Plagiarism Today](https://www.plagiarismtoday.com/2026/10/01/understanding-the-reuters-ross-ai-ruling/)). Commentary stresses it is narrow (non-generative AI, specific record). Direction of travel: **a product that reproduces substitutive text is exposed; headline/excerpt + link + original synthesis is the safer posture.** Autopaper's design (cited story versions rather than copied articles) is aligned with that, as long as story text paraphrases and quotes sparingly.
- **EU.** The DSM Directive's text-and-data-mining exception (Art. 4) lets rightsholders opt out in a machine-readable way. The Hamburg Higher Regional Court (Dec 2025, *Kneschke v. LAION*) found the reproduction covered by the TDM exception and discussed whether a natural-language reservation is machine-readable; the case is not final, and other national courts (Amsterdam, Denmark) differ ([Freshfields](https://technologyquotient.freshfields.com/post/102mebl/ai-training-copyright-part-2-text-and-data-mining-under-court-scrutiny), [Open Future](https://openfuture.eu/blog/laion-round-2-machine-readable-but-still-not-actionable/)). Practical: **honour robots.txt and any TDM-reservation headers for EU publishers.**
- **Japan.** Art. 30-4 of the Copyright Act permits use for "information analysis" not aimed at enjoying the expressed ideas, unless it unduly harms rights holders; there is no explicit opt-out mechanism, which the Japan Newspaper Association criticised in Nov 2025 ([storialaw](https://storialaw.jp/en/service/bigdata-12/12-2), [JNA statement](https://www.pressnet.or.jp/statement/pdf/20251113_en.pdf)). I could not verify the news-search-service/snippet provisions (Art. 47-5 details).
- **Taiwan.** Art. 49 (reporting current events) is read narrowly by the IP Court: same-day factual reporting, and only works actually perceived during the report; Art. 65 is a four-factor fair-use test modeled on US law ([Lee & Li](https://www.leeandli.com/EN/Newsletters/5895.htm), [Judicial Yuan IP Court](https://ipc.judicial.gov.tw/en/cp-697-4361-9c629-092.html)). I found nothing on snippet aggregators specifically: **unresolved**, so keep excerpts short and link back.
- **PRC.** Not researched in depth; PRC state-media and government pages are published for broad reach, but I found no authoritative statement of reuse terms. **Could not verify.** Also note jurisdictional risk of what your pipeline fetches from the PRC (blocking, not liability).

### 11.3 Platform terms found

- **Telegram:** The terms were updated (Open Terms Archive dates the change to 2026-02-03) to prohibit "scraping, indexing, harvesting, aggregation" and use for AI/ML ([Telegram content licensing terms](https://telegram.org/tos/content-licensing), [OTA memo](https://opentermsarchive.org/en/memos/telegram-prohibits-collecting-data-for-ai-use/)). Choir already polls 73 channels through `t.me/s`. Whether read-only polling for news summarization is "aggregation" is unclear; **this is the single most important open legal question for the existing registry.** Mitigation: store bounded text only, link back, never use channel text as model training data, and keep a per-channel kill switch.
- **X/Twitter:** Nitter and XCancel are down after legal threats in Aug-Sep 2026 (X's claims cite Texas computer-access law and the Lanham Act) ([TechCrunch](https://techcrunch.com/2026/08/25/x-sends-cease-and-desist-to-open-source-project-nitter-over-alleged-scraping/), [Reclaim The Net](https://reclaimthenet.org/nitter-goes-dark-after-x-corp-cease-desist-letters)). Only the paid API is safe.
- **RSS feed terms** are publisher-specific: some restrict feeds to personal non-commercial use (Grist; benefits-industry publications), some allow excerpt reuse with attribution and links (RFA), some bar paid/commercial use of feeds (Trend News Agency) ([Grist](https://grist.org/about/rss-terms/), [RFA](https://www.rfa.org/about/info/terms-of-use.html), [Trend](https://trend.az/copyright)). **A registry-time "feed terms" check for the top 50 outlets is cheap insurance.** Choir's `tos_class` field is the right place for it.
- **OpenRSS:** its terms prohibit commercial use (see bridges section).

### 11.4 Storage policy that matches the evidence

1. Always store: URL, title, published time, source id, language, hash, entity tags.
2. Store excerpt (first N characters of the feed's own description) for ordinary publisher RSS; link-only for feeds with restrictive terms.
3. Store bounded full text only for: government/official releases and sources with open licenses (CC-BY, CC0, public domain).
4. Never republish translations of full articles; publish translated *titles/quotes of one or two sentences* with attribution.
5. Log robots.txt and TDM-reservation state per host; refuse hosts that opt out unless a license exists.

---

## 12. Recommendations

### 12.(a) Prioritized next three connectors / source expansions (small budget, Taiwan/East Asia first)

**Priority 0 (before anything new): repair what is already in the registry.** No code, an hour of work, and it removes silent dead sources.
- `rss:nhk_world_en`: change URL from `https://www3.nhk.or.jp/rss/news/cat0.xml` (frozen since 2026-08-08) to `https://www.nhk.or.jp/rss/news/cat0.xml` `[V]`.
- `rss:koreaherald`: `/rss/news.xml` is an empty shell; use `https://www.koreaherald.com/rss/kh_Business` (50 items) and look up its section feeds `[V]`.
- `rss:36kr`: `https://36kr.com/feed` returns HTML; find a working feed or drop it `[V]`.
- `official:who:news` (newest item 2026-02-25) and `rss:agriculture_dive` (newest 2025-01-31): re-check the URL or drop `[V]`.
- Retry `arabnews`, `mining_com`, `railfreight` (403), `venturebeat` (429), `hn_frontpage` (connection reset) from production egress.

**Connector/expansion 1: a registry-only RSS expansion for Taiwan, Japan, Korea and Chinese-language sources (no new code).**
- Add the verified feeds in Section 5: roughly 30 Taiwan zh-TW/en (CNA ×6 sections, LTN, UDN, PTS, Newtalk, RTI, ETtoday, Yahoo TW, iThome, TechNews, DigiTimes zh/en, Focus Taiwan, TFC, MyGoPen, Executive Yuan, Presidential Office, MOFA), about 12 Japan (NHK ja, Asahi, Yomiuri, Mainichi, Jiji, Japan Today, Nippon.com, The Diplomat), about 12 Korea (Yonhap en/ko, Chosun, Newsis, Dong-A, Maeil, The Elec, Business Korea, KED Global, NK News, 38 North), and about 12 Chinese-language international and HK (RFA Mandarin, Initium, BBC/DW/RFI/VOA/FT Chinese, HKFP, RTHK, CGTN, CDT, Sixth Tone).
- Cost: nothing but polling and token spend (Section 9: well under $1/day for translation of titles at this volume; my estimate is 2,000-8,000 new items/day for about 100 feeds).
- Set per-source `tos_class`, `store_body_policy` (`excerpt_only` for CNA, UDN, Newtalk and anything with AI-use restrictions; bounded text for official/government feeds) and `languages` correctly (`zh-Hant`/`zh-Hans`/`ja`/`ko`).
- Value: converts the Taiwan vertical from effectively zero native sources to broad daily coverage, in Chinese, Japanese and Korean, from first-party publishers. Why first: highest value per unit of effort and risk, uses the existing connector, and each source has a clear terms position.
- Companion small code change: **language detection at capture** (lingua-go, pure Go, 75+ languages `[S]` https://pkg.go.dev/github.com/pemistahl/lingua-go), because `Item.Language` is currently `firstString(source.Languages)` (`rss.go`) and multilingual feeds (BBC, Business Korea) need per-item detection.

**Connector/expansion 2: the GDELT translation stream plus a Taiwan/East Asia filter (a registry row and a small filter).**
- Add a second `gdelt` registry row for `https://data.gdeltproject.org/gdeltv2/lastupdate-translation.txt` (probably works unchanged, untested; Section 8.1) and add a **pre-storage filter** by source domain suffix (`.tw .jp .kr .hk .cn .sg .vn .th .ph .id`), `srclc` language and theme/entity match so the 280,000-rows/day translation stream is reduced to a few thousand lead URLs. Raise or replace `max_items_per_poll: 500` (the English slot alone had 1,552 rows).
- Cost: free data; the cost risk is downstream (fetching and translating whatever you keep). Value: discovers outlets Choir does not know exist (48 languages in one slot), cross-lingual coverage of the same story, and entity/theme tags for free. Limits: no text, thin Taiwan depth (4 CNA rows in a slot), fetch retry needed for the 404-ahead-of-publication quirk.
- The leads should go through the existing `article_import.go` path only when they match a story already forming (see cost caps below).

**Connector/expansion 3: a small JSON-diff connector (`json_feed` source type).**
- One poller that fetches a JSON snapshot (or a windowed query), applies a per-source JSON-path mapping (`id`, `title`, `url`, `published`, `body`), and emits an Item per new id. Reuses `StableItemID` and `ContentHash`.
- First targets, in order: **TWSE material announcements** `https://openapi.twse.com.tw/v1/opendata/t187ap04_L` (Taiwan equities, Chinese, structured, daily) ; **CISA KEV** (CC0, 1.4 new per day) ; **NVD 2.0** windowed by `lastModStartDate` every 2 hours or more with a free key ; then OCHA FTS, World Bank news, HDX, UCDP, Indian Parliament bills ; JVN/JPCERT/TWCERT/KrCERT stay RSS.
- Cost: free APIs; one new Go file and tests. Value: the structured, highest-trust, lowest-noise sources for the semiconductor/equities angle and the whole cybersecurity vertical, and a reusable shape for Verticals 2 and 3.
- Needed in parallel: register a **ReliefWeb appname** (Vertical 2), get a free **abuse.ch Auth-Key** and an **NVD API key** (Vertical 3).

**Deliberately not in the top 3:** Bluesky/Mastodon/Nostr firehoses (Section 3), RSSHub with cookie routes, X API, Google News RSS, PTT/Dcard/LIHKG scrapers, community radio transcription, commercial news APIs, ACLED. Reasons are in their sections.

**Next after the top 3, in order:** (i) the BBC language-service feeds (31 feeds; after confirming BBC reuse terms) and Global Voices (CC BY) for the Global South vertical; (ii) a recurring feed-health audit that sets `Source.Status`; (iii) a Wikimedia pageviews job per language as a trending signal; (iv) an HTML-list connector for Taiwan MND's daily PLA activity page and a few PRC/Japan official pages that have no feed (only after confirming the Cloudflare walls are not egress-specific); (v) a hand-picked Mastodon/Bluesky account list as plain RSS.

### 12.(b) Tiered source plan

| Tier | Definition | Contents | Monthly cost (est.) | Ops burden |
|---|---|---|---|---|
| **0: free RSS/Atom** | Publisher or government feeds on the existing RSS connector | Section 5 Taiwan/JP/KR/CN feeds; Section 6 Africa/India/BBC language/Global Voices feeds; Section 7 vendor/CERT/blog feeds; YouTube channel feeds; Mastodon/Bluesky/Lemmy account and tag RSS; Wikipedia current events (en/zh/ja raw) | $0 plus polling | Low: feed-health audit; re-resolve moved URLs |
| **1: free structured data** | Free, redistributable or attribution-only, needs a small connector or key | GDELT GKG (English and translation); TWSE announcements; CISA KEV; NVD (key); EPSS; GHSA; OSV; OCHA FTS; World Bank; HDX; UNHCR; UCDP (token); Wikimedia pageviews; Google Trends RSS; Polymarket; MND daily PLA page (HTML); ReliefWeb (appname); abuse.ch (Auth-Key); ransomware.live only with written approval | $0 | Medium: one JSON-diff connector, keys, schema drift |
| **2: low-cost metered** | Small recurring bill, bounded by caps | LLM translation and tagging (Haiku-class; about $1-5/day at 10,000 items/day); local multilingual embeddings on one CPU/GPU box; self-hosted RSSHub (no-login routes only); X API for about 50 hand-picked accounts (about $150/month at 20 reads/day each, my arithmetic); Google News RSS lead generation if the owner accepts the ToS risk; Feedsearch/Feedly for discovery | roughly $50-400 | Medium |
| **3: paid or heavy** | Large fixed cost, legal review, or substantial compute | NewsAPI.ai/Event Registry ($90+/mo); Webz.io pay-per-use; Common Crawl CC-NEWS daily batch (about 13 GB/day); self-hosted NLLB/MADLAD translation (NLLB licence blocks commercial use); Whisper transcription of radio ($43-86/day for ten stations); ACLED corporate licence; residential proxies and cookie routes for Weibo/Zhihu/Douyin/Xiaohongshu; Threads/LINE/PTT/Dcard access; Meltwater/Dataminr/Factiva | $1,000+ | High; legal and anti-bot exposure |

### 12.(c) Biggest cost risks and how to cap them

| Risk | Why it bites | Cap |
|---|---|---|
| **Translation and LLM tagging volume** | GDELT translation stream alone is about 280,000 rows/day; feeds can spike; Sonnet-class is about 20x Haiku-class | Translate only titles at capture (about $0.00003 each) and only items that join a candidate story; per-day token budget with a circuit breaker; use Message Batches (50% off) for non-urgent work; Haiku-class by default, escalate per item |
| **Article fetching for GDELT leads and thin feeds** | Fetching every lead is crawling at scale: bandwidth, bans, Cloudflare walls, copyright | Fetch only for leads that match a forming story; respect robots.txt; per-host rate limit; keep `store_body_policy` excerpt-only by default |
| **Polling cost scaling with source count** | 211 sources at 900 s is cheap; 1,000 sources at 900 s with full parsing is not | Conditional GET (already supported); velocity-based intervals; back off dead feeds; `max_items_per_poll` per source |
| **Pay-per-use APIs (X, Webz.io, NewsAPI.ai overage)** | Linear in volume; X reads capped at 3M/month but $0.005 each | Hard monthly caps in code, not just dashboards; fixed account lists; no polling of firehose-like queries |
| **Unbounded streams (Jetstream, EventStreams, CC-NEWS)** | 3.6 GB/day for Bluesky posts alone; 13 GB/day CC-NEWS | Do not consume them without an allow-list; filter before storage |
| **Storage of raw snapshots and bodies** | The fetch record stores payload hashes/snapshots; full text multiplies storage and legal exposure | Retention days per source (the `retention_days` field exists); store excerpts |
| **Embedding compute** | Per-item cost small but constant | Embed titles plus lead only; one small multilingual model on CPU; benchmark first |
| **Legal cost (Telegram, CNA/UDN commercial terms, ransomware.live)** | A takedown or demand is more expensive than any API | Decide policy in Section 11/12(d) before scaling; kill switch per source; link back, excerpt only |
| **Proxy/anti-bot escalation** | Cookie routes and walled sites push toward residential proxies and burner accounts | Do not start. Walled sources stay out of scope unless a first-party feed or licence exists |
| **Silent source rot** | NHK, Korea Herald, TrendForce, WHO were dead while still "polled"; wasted polls and false confidence in coverage | Recurring feed-health audit with freshness gate (Section 10.3) |

### 12.(d) Open questions for the owner

1. **Telegram.** Telegram's amended terms (about 2026-02-03) prohibit scraping, indexing and aggregation and AI/ML use. Choir polls 73 channels via `t.me/s` and feeds an LLM. Keep, narrow to a vetted list, or replace with first-party feeds? (No legal advice here; counsel is cheap compared with a ban or demand.)
2. **Is Autopaper commercial?** CNA RSS is licensed for personal/non-profit use, ransomware.live needs written commercial approval, other publishers vary. If revenue is planned, should Choir email CNA, BBC and ransomware.live now?
3. **Storage policy.** Full-text feeds (Newtalk, RFA, Initium, HKFP, CGTN, MyGoPen, 38 North) exist, but some outlets' robots/terms bar AI use. Store excerpts only everywhere unless the licence is open (CC BY / government), or allow bounded full text for chosen outlets?
4. **Google News RSS.** Accept the ToS/robots risk for a low-volume lead generator, or exclude it?
5. **Translation policy.** Titles-only at capture and lazy translation later (recommended), or translate bodies? Is it acceptable to display machine-translated quotes with a label?
6. **Threads, LINE, PTT, Dcard and LIHKG** are where Taiwanese discussion lives and none is freely readable. Is a paid/ToS-grey route ever acceptable, or do they stay out of scope?
7. **X.** Is there a budget (about $150-300/month) for a hand-picked account list, or skip X entirely?
8. **Budget ceiling** per month for Tier 2 (translation, embeddings, X, hosting) and Tier 3 experiments, and a daily LLM-token ceiling the system should never exceed?
9. **PRC official sources** (MFA, TAO, MND, Xinhua) have no working feeds. Is HTML change-detection on a handful of pages acceptable, or a GDELT/aggregator route only?
10. **Egress.** Many 403s here may be network-specific. Can the production egress IP be used to re-test the walled sources (China Times, MAC, NCC, TSMC, Japan MOD, Caixin) before dropping them?
11. **Dependencies.** OK to add lingua-go for language ID, and a JSON-diff connector type to the registry schema?
12. **Verticals 2 and 3.** Register ReliefWeb/NVD/abuse.ch credentials now under which identity, and which Nairobi-/India-specific themes should drive the Global South source choice (urbanization, housing, transport, informal settlements)?

---

## 13. What I could not verify (consolidated)

- Official Bluesky/Jetstream documentation and current official bandwidth numbers (pages returned empty or redirects to my fetch tool); Bluesky `searchPosts` policy (no official statement found; 403 observed).
- Taiwan-specific Bluesky adoption; any 2026 Nostr volume data; ActivityPub relay inventories.
- RSSHub RAM requirements; breakage rate of cookie routes (not tested); PTT/LIHKG/Dcard terms of use.
- BBC RSS terms; Telegram's wording as applied to read-only preview polling; GlotLID licence; hosted embedding API prices; LLM-versus-MT quality on low-resource languages; Gemini price page (third-party trackers disagree).
- GDELT DOC API behaviour (all requests got HTTP 429 from the test IP); GDELT GEO API path; why `lastupdate` runs ahead of published files; whether the existing connector drops GKG rows above 500.
- Media Cloud Taiwan collection and licence; Chinese-language coverage of commercial news APIs; Newsdata.io official pricing; Reddit official API terms; terms of Bing News RSS, Yahoo RSS, Google Trends RSS, Wikidata/Feedly automated use; PRC reuse terms; Japan Article 47-5 snippet provisions; Taiwan snippet/aggregator case law.
- The sub-pass tests for Sections 4-8 were run in this session with the same method (single request per URL, Choir UA, short timeouts) from one US network; individual feeds can differ from production egress.

## 14. Method and artifacts

- Repo files read (read-only): `internal/sources/types.go`, `rss.go`, `telegram.go`, `gdelt.go`, `registry.json`.
- Live probes: Jetstream WebSocket samples (45 s and 240 s), Nostr relay sample (40 s), Mastodon/Lemmy/Bluesky HTTP checks, a full-registry freshness audit (137 RSS entries, one GET each), plus per-topic curl tests described in each section.
- Scratch files lived in the session scratchpad directory, not the repo. Nothing was committed and no repo file other than this report was written.
