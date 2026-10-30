# Taiwan and Sinosphere public-discourse sources: research for Autopaper

Date: 2026-10-13. Author: research agent (Claude Code), read-only on the repo.
Mutation class of this document: green (docs only). No code, registry or config was changed.
Companion to `docs/archive/reports/multilingual-osint-sourcing-research-2026-10-13.md` (news-outlet RSS and general sourcing). That report is not repeated here; where this one overlaps it says so.

## 0. How to read this report

- **Evidence labels.** `[V]` = I tested it myself on 2026-10-13 (curl, or an API fetch) and the result is stated. `[S]` = from a web search or a fetched documentation page; the URL is cited and I did not test the claim itself. `[U]` = could not verify; stated so.
- **Test conditions.** All `[V]` probes came from one residential US egress with the polite UA `ChoirUniversalWire/0.1 (+https://choir.news)` (a browser-style UA only where stated). Cloudflare and Akamai 403s depend on IP and fingerprint. Re-test from the production egress before dropping any source.
- **Search-tool caveat.** WebSearch summaries are model-written. I used them for pointers and cite the underlying page; where a summary was my only source it is `[S]` and says so. One search result page contained text addressed to automated readers; I ignored it.
- **Not legal advice.** Legal sections are a map of the terrain with sources, not an opinion.
- **Raw samples** (JSON, RSS) were saved in the session scratchpad only; nothing was written to the repo except this file.

## 1. Executive findings

1. **The best Taiwanese "public voice" sources are government and civic-tech open data, and they work today.** The JOIN platform's proposal data is a daily-updated JSON under the Open Government Data License: 4,604 citizen proposals already in 2026, each with endorsement count, comment count and full text `[V]`. The Legislative Yuan has a free CC-BY-4.0 JSON API (LYAPI) covering 148,164 bills, 11,691 interpellations, 10,919 roll-call votes and 104,762 video records with machine transcripts `[V]`. Neither needs a login.
2. **The two most-discussed Taiwanese social surfaces (PTT, Dcard) are walled; Threads has an official but gated API.** PTT, Dcard, Mobile01, Cofacts' own site and LIHKG all returned 403 `[V]`. PTT Web (`pttweb.cc`) serves readable HTML lists with push counts `[V]` but is a third-party mirror of user content with no terms I could read. Threads offers an official keyword-search endpoint (2,200 queries per rolling 24 h, app review required) `[S]`; I could not test it without an approved app.
3. **Cofacts is the single best window into private-chat (LINE) rumor traffic** and its licence conflict is resolved: CC BY-SA 4.0 plus a Data User Agreement requiring attribution (`LEGAL.md` v1, 2021-02-17) `[V]`. Access is a gated Hugging Face download (auto-approval on accepting the agreement), 432 MB, 11 CSV tables, hashed user IDs, last modified 2026-10-11 `[V]`. The live API and website are behind Cloudflare and the agreement forbids scraping the site.
4. **Polling is mostly not machine-readable and some series have gone quiet.** The government's MAC publishes unification/independence and cross-strait opinion CSVs through data.gov.tw (latest survey 2026-08-13 to 08-17) `[V]`. NCCU's trend series is published as PNG charts plus a methodology PDF, updated 2026-07-03 through 2026/06 `[V]`. TPOF's RSS has no item newer than 2025-12-31 `[V]`, which may mean it stopped publishing or moved (not resolved).
5. **Mainland China: four hot-list endpoints work without login (Baidu, Douyin, Toutiao, Bilibili); Weibo and Zhihu do not.** Baidu `top.baidu.com/api/board`, Douyin `aweme.snssdk.com/aweme/v1/hot/search/list/`, Toutiao `hot-board` and three Bilibili endpoints returned JSON `[V]`. Weibo returns 403 or a "Sina Visitor System" page, Zhihu returns 401 `[V]`. These are undocumented, platform-curated rankings: they measure what the platform surfaces, not what people think, and carry unfair-competition and PRC data-law risk `[S]`.
6. **Hong Kong and Singapore have usable feeds; Malaysia's Chinese-language press does not.** inmediahk (15 items, fresh), The Witness HK, HKFP, RFA Mandarin, Initium; HardwareZone's XenForo forums expose full-post RSS with minutes-old items (and usernames) `[V]`. Sin Chew, China Press, Oriental Daily and Nanyang feeds were all 404 or 403 `[V]`.
7. **Reddit is legally closed and technically flaky.** `robots.txt` is `User-agent: * / Disallow: /` `[V]`; `.json` is 403, subreddit `.rss` works for `r/taiwan` (100 entries, about 25 posts/day) but 429s on rapid sequential requests `[V]`. API commercial terms are unverified (reported about 0.24 USD per 1,000 calls) `[S]`.
8. **Volumes are small, which is a feature for a precommitment ledger.** Mastodon `#台灣` on mastodon.social: 40 posts in 33 days `[V]`; `r/taiwan` about 25 posts/day `[V]`; JOIN about 500 proposals/month `[V]`; g0v.social 1,019 monthly active users `[V]`. A weekly aggregate must carry its denominator because many cells will be tiny.
9. **PDPA fit is workable for aggregate-only collection by a company.** Taiwan's PDPA Article 19 allows non-government collection of data the subject made public or that is publicly available "unless the data subject has an overriding interest"; its research exemption is for academic institutions only; political opinion is not on the Article 6 sensitive-data list in the MOJ English text `[S]`. The November 2025 amendment's commencement was not set as of January 2026 `[S]`.
10. **Social-media sentiment is a weak stand-in for polls in Taiwan.** One 2022 Taipei mayoral analysis found internet sentiment indicators inconsistent with results while telephone polls held up; a 2016 study found online data predictive and polls useful for correcting online sample bias `[S]`. Treat platform data as topic prevalence within a frame, never as national opinion.

## 2. Connector shape recap (what "fits" means)

From `/Users/wiz/go-choir/internal/sources/rss.go` and the prior report: a poller returns items with `URL, CanonicalURL, Title, Body, Published, Language, Region, ContentHash, BodyKind` and per-source `store_body_policy`, `tos_class`, `retention_days`. `rss.go` reads RSS/Atom/RDF with conditional GET, a 2 MB body cap and 100 items per poll, and sets language from the registry row, not detection.

Four mapping classes are used below:

| Class | Meaning | Code needed |
|---|---|---|
| **RSS** | Existing connector; a registry row | none (check charset: JOIN's feed is UTF-16, see 3.2) |
| **JSON-diff** | Poll a JSON list or window and emit one Item per new record id | new small connector (also useful for TWSE, CISA KEV, per the prior report) |
| **Dataset** | Periodic file download (CSV, zip), diffed by row | JSON-diff variant: fetch, parse, emit changed rows |
| **Aggregate snapshot** | Store counts or ranks per window, not documents | new "observation" path (see section 12) |

## 3. Taiwan civic tech, deliberation and government open data (brief item 1)

### 3.1 Summary table

| Source | Access | Tested 2026-10-13 | Cost | Volume | Slice of opinion and skew | Legal/ToS | Choir mapping |
|---|---|---|---|---|---|---|---|
| **JOIN proposals ("提點子")** | JSON per year `https://join.gov.tw/toOpenData/v2/ey/idea?year=2026` (catalogue: data.gov.tw dataset 58036) | `[V]` HTTP 200, `application/json`, 13.9 MB, 4,604 records | Free | about 500 new proposals/month; 2,207 of 4,604 have at least 1 endorsement; 46 reached the 5,000 threshold; max 44,623 endorsements | Self-selected, registered, aggrieved/organised citizens; strong on salience of grievances, weak on representativeness | Open Government Data License v1.0 (政府資料開放授權條款-第1版) per catalogue; `提議者` is a nickname, drop it | **Dataset / JSON-diff**, daily |
| **JOIN RSS** | `https://join.gov.tw/rss/feed` | `[V]` 200, `application/rss+xml;charset=utf-16`, 61 items, newest pubDate 2026-10-05 | Free | latest proposals only | same | Site terms not read `[U]` | **RSS** (UTF-16 decode required) |
| **JOIN "眾開講" policy consultations** | `https://join.gov.tw/toOpenData/ey/policy` (dataset 58297) | `[V]` 200, 417 records; sample record untitled, dates "無"; metadata last updated 2025-08-01 | Free | stale | government-initiated consultation, comment counts only | OGDL v1.0 | Low value now |
| **data.gov.tw catalogue** | `https://data.gov.tw/datasets/export/json?dnid=58297` (the `dnid` parameter is ignored; it returns the whole catalogue) | `[V]` 200, 90 MB JSON, 52,503 dataset records | Free | catalogue | n/a | OGDL v1.0 | discovery only (do not poll) |
| **LYAPI (Legislative Yuan)** | `https://ly.govapi.tw/v2/{bills,meets,ivods,interpellations,votes,legislators,committees,laws,gazettes}` | `[V]` all 200 JSON except `/speeches` (404) | Free | bills 148,164; ivods 104,762 (15 on 2026-10-13); interpellations 11,691; votes 10,919 | Elite/institutional discourse: what legislators ask, say and vote | Data CC-BY-4.0, code BSD-3 (site footer `[V]`); LY content is government data | **JSON-diff**, filter by `日期` or `屆` |
| **IVOD transcripts** | `https://ly.govapi.tw/v2/ivod/<id>` | `[V]` `transcript` has `pyannote` (speaker segments with `speaker_name`) and `whisperx` (timestamped text, 1,141 segments for a 40-minute meeting) | Free | per meeting | machine transcript, noisy (the sample repeated one phrase) | as above | Body text = joined whisperx segments; store excerpt only |
| **vTaiwan** | `https://vtaiwan.tw/` (2026 rebuild, CC-BY-SA, Hono + Vue) | `[V]` 200; `/issues`, `/blog`, `/feed`, `/rss.xml`, `/sitemap.xml` all 404 (likely client-routed); no feed found | Free | low | deliberative process on digital policy | site licence CC-BY-SA | Not feasible as a poller; watch GitHub |
| **vTaiwan-meetings (GitHub)** | `github.com/g0v/vTaiwan-meetings`, MIT | `[V]` last push 2026-10-07; commits "AI初校" (AI first pass) then "人工二校" (human second proof) | Free | per meeting | transcripts and outlines of vTaiwan meetings | MIT | Poll commits feed (Atom) |
| **Pol.is open data** | `github.com/compdemocracy/openData` | `[V]` last push 2026-03-08, CC BY 4.0, 21 directories, only one Taiwanese set (`vtaiwan.uberx`) | Free | historical | training/evaluation corpus for stance models; not a live feed | CC BY 4.0 with required attribution statement | Dataset, one-off |
| **Pol.is API** | `https://pol.is/api/v3/...` | `[V]` 400 for a guessed conversation id; no listing endpoint | Free | per conversation | n/a | n/a | Not feasible without known report ids `[U]` |
| **Talk to the City** | `github.com/AIObjectives/tttc-light-js` (Apache-2.0, pushed 2026-10-13) | `[V]` repo metadata only | Free | n/a | LLM clustering tool, not a data source | Apache-2.0 | A method to borrow, see 11.4 |
| **Executive Yuan press** | `https://www.ey.gov.tw/RSS_Content.aspx?ModuleType=1` | `[V]` 200, 50 items, newest 2026-10-13 | Free | daily | government's own voice | gov | **RSS** (already in prior report) |
| **Control Yuan, MODA, MAC site** | `cy.gov.tw` 200 (no RSS at the guessed path, 404); `moda.gov.tw` 200 (guessed RSS 404); `mac.gov.tw` root and `rss.aspx` 403 | `[V]` | n/a | n/a | n/a | n/a | No feed found `[U]`; MAC data is reachable as CSV (section 6) |

### 3.2 Notes

**JOIN open data.** Field list from the catalogue and confirmed in the 2026 file `[V]`: `publishDate`, `網址`, `標題`, `提議內容`, `利益與影響`, `附議數量`, `附議門檻` (5000 in the sample), `提送日期`, `關注數量`, `留言數量`, `提議者`. The catalogue says the dataset is updated daily and has per-year files for 2020-2024 as listed (the 2025 and 2026 URLs also answer, by pattern `[V]`). 388 of 4,604 records have `無` for the submit date, so date handling must tolerate it. The three most-endorsed 2026 proposals (44,623; 29,285; 18,572 endorsements) concern a labour-import programme, an animal-pharmacy rule and juvenile sentencing `[V]`, which illustrates the signal: it surfaces organised issue campaigns. Sources: `https://data.gov.tw/dataset/58036`, `https://join.gov.tw/toOpenData/v2/ey/idea?year=2026`.

**JOIN RSS charset.** The feed declares UTF-16 and carries a BOM; decoding as UTF-16 gave 61 clean items `[V]`. The prior report found Vietnam News also uses UTF-16, so connector charset handling should be verified once for both. `rss.go` imports `golang.org/x/net/html/charset`; I did not run the connector against the JOIN feed.

**LYAPI.** Pagination is `page` and `limit`; the response carries `total`, `total_page`, `supported_filter_fields` and `output_fields`. Default sort differs by endpoint: `bills` lists the latest first, `interpellations?limit=1` returned the oldest record (2024-02-16), so pass a sort or filter explicitly. Filtering works (`/v2/bills?屆=11&議案狀態=三讀` returned 710) `[V]`. A bad sort parameter returned a JSON error, not a crash `[V]`. Rate limits are not documented in what I read `[U]`. `votes` includes per-legislator names; they are public officials acting in office, which is a different privacy class from private citizens. There are static Hugging Face snapshots (`openfun/tw-ly-*`, CC-BY-4.0, March 2025) for bulk training `[V]` via `https://huggingface.co/api/datasets?author=openfun`. Docs: `https://ly.govapi.tw/v2/`, guide at `https://hackmd.io/@openfunltd/S1iLBqP21l`.

**g0v is alive in 2026.** Repos pushed within the last 10 days include `g0v/sensemaker-frontend` (2026-10-13), `g0v/civic-talk-hono` ("Civic Talk" rebuild with AI to lower deliberation barriers, 2026-10-12), `g0v/vTaiwan-hono` (vTaiwan rebuilt with a 2026 look, 2026-10-08), `g0v/vtaiwan-transcription-worker` and `g0v/fact-check-api` (2026-10-04) `[V]` via `https://api.github.com/orgs/g0v/repos?sort=pushed`. I did not find a published data export from Civic Talk.

**Alignment assemblies.** Audrey Tang says the information-integrity assembly sent 200,000 SMS invitations to random numbers and selected 447 participants to mirror the population (stratified), in Taipei and Tainan, using Pol.is and Talk to the City `[S]` `https://archive.tw/speech/627520`, `https://therecord.media/taiwan-using-ai-to-fight-disinformation`, Participedia case `https://participedia.net/case/13316`. I did not find a published Pol.is dataset or results report for it. Treat it as a method precedent, not a source.

**Not verified:** any vTaiwan Pol.is conversation active in 2026; the Collective Intelligence Project's Taiwan outputs (the GitHub repo name I tried does not exist); MODA open-data APIs beyond data.gov.tw.

## 4. Misinformation and narrative tracking (brief item 2)

| Source | Access | Tested | Cost | Volume | What it represents | Legal | Mapping |
|---|---|---|---|---|---|---|---|
| **Cofacts dataset** | Hugging Face `Cofacts/line-msg-fact-check-tw` (gated "auto"), also source code at `github.com/cofacts/opendata` | `[V]` metadata API 200; `lastModified` 2026-10-11; license tag `cc-by-sa-4.0`; 432 MB; 11 zipped CSV tables (`articles`, `replies`, `article_replies`, `reply_requests`, `article_categories`, `analytics`, `anonymized_users`, etc.); user ids are `userIdsha256` | Free; account + accept agreement | size tag 10M-100M rows; 125 downloads last month | Messages that LINE users forwarded to the Cofacts bot to ask "is this true", plus crowd replies. Skews to people who use the bot (likely older, rumor-exposed, civic-minded); not a random sample of chat | **CC BY-SA 4.0** + Data User Agreement `LEGAL.md` v1 (2021-02-17): attribution statement required, share-alike on redistribution, automated scraping of the site/bot banned except low-frequency public crawlers | **Dataset** (monthly) |
| **Cofacts live API** | `https://api.cofacts.tw/graphql` | `[V]` 403 Cloudflare "Just a moment"; `api.cofacts.org` 522 | needs `app-id` from the working group | n/a | same | per `LEGAL.md` §3: apply to the WG for an `app-id` | Not feasible without approval |
| **Taiwan FactCheck Center** | RSS `https://tfc-taiwan.org.tw/feed`; WP JSON `https://tfc-taiwan.org.tw/wp-json/wp/v2/posts` | `[V]` RSS 200, 10 items, newest 2026-10-12; JSON 200 | Free | weekly "闢謠TOP10" roundups plus individual checks | Fact-checker's view of what circulated | publisher terms not read `[U]` | **RSS** |
| **MyGoPen** | Atom `https://www.mygopen.com/feeds/posts/default` | `[V]` 200, 25 entries, full text, newest 2026-10-13 | Free | daily | same | not read `[U]` | **RSS** |
| **IORG (Information Environment Research Center)** | HTML `https://iorg.tw/_en`; no feed (`/feed`, `/rss.xml` 404) | `[V]` 200; lists monthly "中共月報" reports (the June 2026 issue published 2026.7.10; newest listed item is dated July) | Free | monthly | PRC narrative tracking on the Mandarin information environment | not read `[U]` | Page diff on the report list (new item = new Item) |
| **Doublethink Lab** | `https://doublethinklab.org/` | `[V]` 200; `/feed`, `/feed/` 404 | Free | irregular | PRC info-ops research; recorded over 10,000 suspicious items before the 2024 election `[S]` `https://globaltaiwan.org/2026/02/enhancing-taiwans-information-resilience/` | n/a | Page diff |
| **DFRLab** | RSS `https://dfrlab.org/feed/` | `[V]` 200, 10 items, newest 2026-10-12 | Free | weekly | global OSINT, occasional Taiwan | publisher terms | **RSS** |
| **Taiwan AI Labs** | RSS `https://ailabs.tw/feed/`; "Infodemic" tool | `[V]` RSS newest 2026-09-10; Infodemic has no public dataset I could find `[S]` | Free | low | LLM-based coordinated-behaviour detection | n/a | RSS only |
| **Global Taiwan Institute** | RSS `https://globaltaiwan.org/feed/` | `[V]` 200, 10 items, newest 2026-09-16 | Free | low | think-tank analysis | not read | **RSS** |
| **Graphika** | `graphika.com/rss.xml` | `[V]` 404 | n/a | n/a | n/a | n/a | Not feasible |

Notes. Cofacts' own `LEGAL.md` resolves the licence contradiction recorded in the prior report: the data are CC BY-SA 4.0, and the Hugging Face card requires accepting the agreement ("auto" approval, an optional free-text "who are you" field that may be answered "n/a") `[V]` `https://huggingface.co/api/datasets/Cofacts/line-msg-fact-check-tw`, `https://raw.githubusercontent.com/cofacts/opendata/master/LEGAL.md`. The dataset card warns that scraped web content inside it is not Cofacts' to license, that replies are responses not verified truth, and that the data contain noise, sampling bias and malicious submissions `[V]` (card text via WebFetch). Share-alike: whether a derived weekly aggregate counts as "adapted material" is a question for counsel, not settled here.

**Not verified:** Cofacts update cadence (card is silent; `lastModified` suggests frequent refresh); any Doublethink Lab or IORG machine-readable data download; IORG's "Information Environment Archive" access terms.

## 5. Taiwanese social platforms (brief item 3)

### 5.1 Platform reach, to calibrate skew

| Metric | Number | Source |
|---|---|---|
| Social media user identities | 18.1 M (78.4% of population), internet penetration 96.7% | DataReportal Digital 2026 Taiwan `[V]` `https://datareportal.com/reports/digital-2026-taiwan` |
| Ad reach, late 2025 | YouTube 18.1 M, Facebook 17.3 M, Instagram 12.2 M, TikTok 8.67 M (adults), Threads 6.65 M (28.8%), X 6.01 M (26.0%); X audience 28.8% female / 71.1% male (X-inferred) | same `[V]`; these are ad reach, not MAU |
| TWNIC 2024 Taiwan Internet Report | Facebook 50.28% (down from 61.22% in 2022); PTT 1.83% (4th); Threads 1.26% (6th, ahead of Dcard); Dcard 0.72%; 16% of internet users use no social media | `[V]` via PTS `https://news.pts.org.tw/article/726382` for Facebook, PTT and Threads; Dcard figure and the 16% from a search summary `[S]`; fieldwork by Taiwan Information Society Study Association; sample size not stated |
| MIC (III) late 2024 survey, n=1,091 | YouTube 72.3%, Facebook 72.1%, Instagram 44.7%, Dcard 17.6%, Threads 17.5%, PTT 17.1%; Dcard/Threads/Instagram skew 18-24, PTT skews 35-44 | search summary `[S]` only; the underlying page was not fetched |
| LINE | 90.2% of internet users aged 16-64 | secondary article citing DataReportal `[S]` `[U]` |

The two Taiwan surveys disagree by an order of magnitude on PTT (1.83% vs 17.1%) because they measure different things (primary platform share vs "commonly used"). Any weighting scheme must state which it uses.

### 5.2 Per-platform table

| Platform | Access method | Tested | Cost | Volume / slice and skew | Legal/ToS risk | Mapping |
|---|---|---|---|---|---|---|
| **PTT (ptt.cc)** | No official API found. Web and Atom (`/atom/<board>.xml`) | `[V]` 403 on `/bbs/Gossiping/index.html`, `/atom/Gossiping.xml`, `Tech_Job`, `HatePolitics`, also with a Safari UA plus `over18=1` cookie; `/robots.txt` 404 | n/a | Taiwan's largest BBS; research use is common `[S]` | Third-party libraries note PTT blocks cloud IPs (PyPtt PyPI page `[S]`); no terms or research-access programme found `[U]` | Not feasible direct |
| **PTT Web mirror** | `https://pttweb.cc/hot/all/today`, `/bbs/<Board>` (server-rendered HTML) | `[V]` 200; hot page lists 50 unique articles with category, push and reply counts, board, author, time; a board page listed 55 article links; robots.txt `Disallow: /imgur/` only | Free | Hot/board lists give title + counts, not full text from the list pages | Mirror of user-authored content; its terms not read `[U]`; scraping a mirror sidesteps PTT's block and does not change authors' copyright or PTT's likely position | HTML poller (new); collect title + counts only, aggregate-only, owner decision (section 14) |
| **disp.cc** | `https://disp.cc/b/Gossiping` | `[V]` 200, 30 KB; robots disallows GPTBot, Ahrefs, Semrush, DataForSEO, rate-limits OpenAI/Meta search bots | Free | board index | an explicit anti-AI-bot posture | Avoid |
| **PTT research datasets** | Hugging Face: `yuhuanstudio/PTT-pretrain-zhtw` (apache-2.0 declared, 2025, first 200 pages per board), `Riyuechang/PTT-Corpus-100K_Gossiping-*` (apache-2.0 declared, 2024) | `[V]` metadata + README only | Free | historical, not live | licence is the uploader's declaration; underlying posts are the authors' | Evaluation set for stance classifier, not a live feed |
| **Dcard** | undocumented web API `https://www.dcard.tw/service/api/v2/posts` exists per iThome `[S]` | `[V]` 403; `about.dcard.tw/terms` 403 so terms unread | n/a | young, student/18-24 skew per MIC `[S]` | high; terms unread `[U]` | Not feasible |
| **Mobile01** | | `[V]` 403 on RSS and a topic list | n/a | older, consumer-tech, hardware | high | Not feasible |
| **Threads (Meta)** | Official Threads API `GET /keyword_search` | `[S]` not tested; requires app approval for `threads_keyword_search` plus `threads_basic`; 2,200 queries per user per rolling 24 h shared across apps; `limit` max 100; `search_type` TOP/RECENT; `search_mode` KEYWORD/TAG; `since` ≥ 1688540400; returns id, text, media_type, permalink, timestamp, username, has_replies, is_quote_post, is_reply; "owner" field excluded; sensitive keywords return empty `[S]` `https://developers.facebook.com/docs/threads/keyword-search` (page updated 2026-01-21 per search) | Free | theoretical ceiling about 220,000 posts/day per user token; Taiwan reach 6.65 M (28.8%) `[V]` DataReportal; skew young, urban, politically active; roughly equal gender | Meta Platform Terms govern use and I did not read them for aggregate analytics `[U]`; third-party sources claim a stricter 500 queries per 7 days that Meta's page does not state `[S]` | **JSON-diff** (query-per-topic poller) after approval |
| **Facebook public pages** | CrowdTangle ended 2024-08-14; successor Meta Content Library + API | `[S]` access via ICPSR/SOMAR review; academic and non-profit researchers qualify; commercial news orgs generally do not; no fees; applicants must not be in sanctioned jurisdictions | Free | largest platform (TWNIC 50.28%) | eligibility excludes a for-profit publisher | Not available unless via a university or non-profit partner `[S]` `https://www.icpsr.umich.edu/sites/icpsr/news/icpsr-to-facilitate-researcher-access-to-metas-api-products`, `https://developers.secure.facebook.com/docs/content-library-api/get-access`, `https://cjr.org/tow_center/meta-is-getting-rid-of-crowdtangle.php` |
| **LINE (chats, OpenChat, Today comments)** | No public chat API; `https://today.line.me/tw/v2/publisher/rss` (my guess) 404; `openchat.line.me` 200 landing page | `[V]` | n/a | Dominant messenger; private | scraping OpenChat is ToS-risky `[U]` | Use Cofacts as the aggregate window onto LINE |
| **YouTube comments** | Data API v3 `commentThreads.list` | `[S]` default 10,000 units/day; `commentThreads.list`, `comments.list`, `videos.list`, `playlistItems.list`, `channels.list` cost 1 unit each; `search.list` has its own 100/day bucket per the cost page as summarised by my fetch tool; quota increases only via Google audit form, no paid tier `[S]` `https://developers.google.com/youtube/v3/determine_quota_cost`, `https://outlierkit.com/resources/youtube-api-quota/` | Free within quota | 10,000 units/day = up to about 10,000 comment pages (100 comments/page max, so up to about 1 M comments/day in theory, far less in practice). Broadest platform by reach (18.1 M) | API ToS allow API-based access; storing comments must respect developer policies (refresh requirements) `[U]` | **JSON-diff** per channel's latest videos; native channel RSS for video discovery (prior report) |
| **Mastodon (Taiwan)** | tag RSS or `/api/v1/timelines/tag/<tag>` on mastodon.social | `[V]` `#台灣` returned 40 posts spanning 2026-09-10 to 2026-10-13 (about 1/day); `g0v.social` has 1,019 monthly active users and public timelines return 422 unauthenticated | Free | tiny | niche, civic-tech, tech-literate | public, accepted | **RSS** for accounts/tags; low value |
| **Bluesky (Taiwan)** | per-account RSS `https://bsky.app/profile/<handle>/rss` | `[V]` `audreyt.org` 200 RSS; `audrey.tang` 404 (wrong handle for the platform) | Free | tiny; prior report measured Chinese at 0.12% of the stream | | **RSS** per hand-picked account |
| **Plurk, Matters, vocus** | web only | `[V]` home pages 200, no feeds tested | n/a | niche | n/a | `[U]`, skipped |

**Key point on PTT.** The prior report found PTT blocked by a Cloudflare challenge even for its native Atom feed; I re-tested with a browser UA and the `over18` cookie and got the same 403 `[V]`, and found no official API, archive programme or research access page `[S]`. The only legal-ish routes are (a) published research corpora (historical, uploader-licensed), (b) a university partner, (c) PTT Web lists under a conservative, title-and-counts-only policy that the owner must approve.

**Key point on Threads.** It is the platform where the research and press say new Taiwanese political talk migrated (the MIT Technology Review piece cited in the prior report), and it has the only official, free keyword search among the large Taiwan platforms. Applying for `threads_keyword_search` is the one action in this report with long lead time and no downside. Whether an organisation without an end-user product qualifies is `[U]`.

## 6. Opinion polling and survey data (brief item 4)

| Series | Access | Tested | Cadence and format | Notes |
|---|---|---|---|---|
| **MAC unification/independence/status quo** (dataset 79342) | CSV `https://www.mac.gov.tw/big5/data/218_民眾對統一、獨立或維持現狀的看法民調.csv` | `[V]` 200, UTF-8 with BOM, 36 lines; newest row 2026.8.13-2026.8.17: 儘快統一 2.1%, 維持現狀以後統一 5.5%, 看情形再決定 25.4%, 永遠維持現狀 31.6%, 以後走向獨立 21.1%, 儘快宣布獨立 5.5%, 不知道 8.8% | irregular (catalogue says 不定期; looks like roughly semiannual); proportions as decimals, start and end date columns | Government-commissioned (the Executive Yuan's MAC), so framing and question wording are the government's. Quote the exact row and the fieldwork dates. License OGDL v1.0 |
| **MAC cross-strait issues** (dataset 42536) | `https://www.mac.gov.tw/big5/data/42536_兩岸相關議題民調.csv` | `[V]` 200, 50 lines; columns: survey name, date, question text, five response proportions; catalogue metadata updated 2026-10-07 | irregular | Rich question text, useful for stance anchors |
| **NCCU Election Study Center trends** (identity, unification/independence, party preference) | `https://esc.nccu.edu.tw/PageDoc?fid=7800` (identity), `fid=7801` (independence/unification), `fid=7802`-style page for party preference `[U]` | `[V]` 200; the pages embed PNG charts (`Tondu202606.png`, `People202606.png`) and a methodology PDF; homepage announcement dated 2026-07-03 says the series was updated to 1992-2026/06; a staff obituary notice is dated 2026-08-24 | semiannual (June and December) | Wikipedia's NCCU-derived table gives Dec 2025: Taiwanese-only 62.0%, Chinese-only 2.5%, both 31.7%, n=14,985 `[S]` `https://en.wikipedia.org/wiki/Opinion_polling_on_Taiwanese_identity`. No CSV found; "Dataset and Regulations" implies an application for microdata `[U]` |
| **TPOF** (Taiwan Public Opinion Foundation) | RSS `https://www.tpof.org/feed`; WP JSON `https://www.tpof.org/wp-json/wp/v2/posts` | `[V]` both 200; 10 RSS items, newest 2025-12-31 ("台灣人眼中的民主進步黨" English excerpt); a November 2025 special report on unification/independence used a 2025-10-13 to 10-15 fieldwork, n=1,070 `[S]` `https://www.tpof.org/wp-content/uploads/2025/11/20251113-「台灣人統獨傾向的最新發展」台灣民意基金會11月專題報告.pdf` | monthly historically; no 2026 post found | Either TPOF stopped posting, changed site or the feed is stale; unresolved `[U]`. Reports are PDF/HTML charts. No licence statement; footer "Copyright 2016" |
| **My-Formosa, TVBS, Academia Sinica TSCS** | `my-formosa.com` 301; `polls.tvbs.com.tw` and `tscs.sinica.edu.tw` connection failed (HTTP 000); `news.tvbs.com.tw/rss` 404 | `[V]` | monthly (My-Formosa), per TVBS release `[U]` | Re-test from the production egress |
| **Asian Barometer** | `https://www.asianbarometer.org/` | `[V]` 200 | survey waves | microdata by registration, terms not read `[U]` |
| **NDC social-listening topics** (dataset 172722) | CSV links on data.gov.tw | `[V]` catalogue entry: 28 key social-development topics, "online-community opinion observation analysis", metadata updated 2025-07-28 | irregular | A government precedent for exactly the aggregate-observation idea in section 12; not fetched in full |

These polls are the only probability-sample anchors in this report. They arrive twice a year at best, so the weekly discourse observations in section 12 should be compared with the most recent anchor rather than aligned to it.

## 7. Mainland China discourse (brief item 5)

All endpoints below are unofficial/undocumented or dependent on cookies. They list what the platform chooses to show; heavy curation and censorship make them evidence about the information environment, not about opinion.

| Source | Access | Tested | Fields / volume | Stability and legal | Mapping |
|---|---|---|---|---|---|
| **Baidu hot search** | `https://top.baidu.com/api/board?platform=wise&tab=realtime` | `[V]` 200 JSON, 51 items, no auth | `word`, `url`, `index`, `hotTag`, `isTop` (no numeric heat in the sample I parsed) | no `robots.txt` (404); undocumented | Aggregate snapshot |
| **Douyin hot search** | `https://aweme.snssdk.com/aweme/v1/hot/search/list/` (the `www.douyin.com` path returned 200 with an empty body) | `[V]` 200 JSON, 49 words | `word`, `hot_value`, `position`, `video_count`, `event_time`, `group_id` | undocumented mobile endpoint, likely to change | Aggregate snapshot |
| **Toutiao hot board** | `https://www.toutiao.com/hot-event/hot-board/?origin=toutiao_pc` | `[V]` 200 JSON, 50 items | `Title`, `HotValue`, `ClusterId`, `InterestCategory` | | Aggregate snapshot |
| **Bilibili** | `https://api.bilibili.com/x/web-interface/popular?ps=20&pn=1`; `.../ranking/v2?rid=0&type=all`; `.../search/square?limit=20`; `https://s.search.bilibili.com/main/hotword` | `[V]` all 200 JSON (20, many, 20 and 10 items) | video title, owner, views, likes, replies, danmaku; trending keywords | popular is widely used; one trending keyword was "DRAM内存短缺或持续到2028年", relevant to the hardware vertical | Aggregate snapshot or JSON-diff |
| **Weibo hot search** | `https://weibo.com/ajax/side/hotSearch`, `.../hot_band`; `m.weibo.cn/api/container/getIndex?...realtimehot`; `s.weibo.com/top/summary` | `[V]` 403 `{"error":"Forbidden"}`; the mobile and `s.` paths return the "Sina Visitor System" page | n/a | needs cookies/visitor tokens; RSSHub's Weibo routes need `WEIBO_COOKIES` and Puppeteer (prior report) | Not feasible |
| **Zhihu hot list** | `https://www.zhihu.com/api/v3/feed/topstory/hot-lists/total` | `[V]` 401 `AuthenticationError`; `/hot` 403 | | login required | Not feasible |
| **tophub.today** | `https://tophub.today/` (aggregates about 40 hot lists as HTML) | `[V]` home 200, 1.2 MB; per-board page `/n/KqndgxeLl9` 403; `robots.txt` 503 | page lists Zhihu, Weibo, WeChat, Baidu, Bilibili, Douyin, Toutiao and others by name | an "official API" repo exists per a search snippet but terms were not found `[S]`; commercial redistribution unclear | Avoid; use first-party endpoints above |
| **RSSHub** | public instance `rsshub.app` | `[V]` 403 (see prior report) | | self-host needed | See prior report |
| **China Digital Times** (censorship archive, English) | RSS `https://chinadigitaltimes.net/feed/` | `[V]` 200, 15 items, newest 2026-10-06; the Chinese feed path 403 | | publisher terms not read | **RSS** |
| **What's on Weibo** | `https://www.whatsonweibo.com/feed/` | `[V]` 5 items, newest 2026-10-04 | slow | | **RSS** |
| **China Media Project** | `https://chinamediaproject.org/feed/` | `[V]` 10 items, newest 2026-10-02 | | | **RSS** |
| **Sinocism** | `https://www.sinocism.com/feed` | `[V]` 20 items, newest 2026-10-13 | partly paywalled | | **RSS**; ToS read needed |
| **Free Weibo, Weiboscope, GreatFire** | `freeweibo.com` 403; `weiboscope.jmsc.hku.hk` unreachable (HTTP 000); `greatfire.org` 200 HTML | `[V]` | | Weiboscope described as tracking 300,000 Weibo accounts with more than 1,000 followers to find deleted posts, but I found nothing confirming it is active in 2026 `[S]` | Not feasible; CDT carries the human-curated censorship reporting |

**Legality and stability.**
- Chinese courts treat large-scale scraping of another platform's data through the Anti-Unfair Competition Law, weighing ease of access, whether the data were processed, and whether anti-crawling measures were used; Weibo v. Maimai (effective end of 2016) found infringement and unfair competition for overstepping limited authorisation `[S]` `https://neudata.co/alternative-data-news/chinese-weibo-vs-maimai-case-shows-limitations-of-web-scraping`, `https://law.asia/platform-data-capture-unfair-competition/`, `https://cms.law/en/chn/legal-updates/china-s-supreme-court-releases-six-guiding-cases-on-data-rights-providing-insights-for-corporate-counsels`. Hot-list snapshots are small aggregates of public rankings, a lighter touch than bulk post scraping, but this is my judgment, not authority `[U]`.
- PRC Cybersecurity Law amendments took effect 2026-01-01; the Data Security Law lets regional/sector regulators define "important data"; CAC export-assessment rules apply to certain data and large processors `[S]` `https://globalinvestigationsreview.com/guide/the-guide-cyber-investigations/fourth-edition/article/china-data-rules-tighten-around-cross-border-investigations`, `https://asiapacific.ca/asia-watch/china-implement-new-data-export-assessment-rules`. These mainly bind data held and moved from inside the PRC; I found no authority on a foreign company polling public hot lists `[U]`. Consult PRC counsel before production.
- Stability: undocumented endpoints; build as best-effort snapshots with a health metric and expect breakage (the prior report notes RSSHub bilibili/twitter routes break often).

## 8. Hong Kong, Singapore, Malaysia, diaspora (brief item 6)

| Source | Access | Tested | Notes | Mapping |
|---|---|---|---|---|
| **inmediahk (獨立媒體)** | RSS `https://www.inmediahk.net/rss.xml` | `[V]` 200, 15 items, newest 2026-10-13 13:43 UTC (site root 403) | citizen-journalism/commentary, zh-HK; body is HTML-escaped fields | **RSS** |
| **The Witness (HK)** | `https://thewitnesshk.com/feed/` | `[V]` 10 items, newest 2026-10-13 | independent Hong Kong outlet; relationship to Stand News not verified `[U]` | **RSS** |
| **Hong Kong Free Press** | `https://hongkongfp.com/feed/` | `[V]` 30 items, full text | English | **RSS** (prior report) |
| **RFA Mandarin / Initium** | `rfa.org/mandarin/rss2.xml` (30 items), `theinitium.com/rss` (15 items) | `[V]` newest 2026-10-13 and 10-12 | zh-Hant/Hans | **RSS** |
| **LIHKG** | `www.lihkg.com/api_v2/...` | `[V]` 403; no official research access found `[S]` | Protected by reCAPTCHA per the R package `LIHKGr` `[S]`; historical scrapes exist on Hugging Face (`AlienKevin/LIHKG` MIT-declared, 1M-10M rows, 2023; `izhx/yue-lihkg-topic` CC-BY-4.0-declared) | Not feasible live; historical corpora only |
| **HKGolden** | `forum.hkgolden.com` | `[V]` 200 (40 KB shell), no feed | | Not feasible |
| **HardwareZone (Singapore)** | XenForo RSS `https://forums.hardwarezone.com.sg/forums/-/index.rss` and per-forum, e.g. `/forums/eat-drink-man-woman.16/index.rss` | `[V]` 200, 19 items, newest within about 30 minutes; each item has `title`, `link`, `pubDate`, `category` (forum), `dc:creator` (username), and full post HTML in `content:encoded` | EDMW is the general/politics forum (a thread on COE rules and a US residency policy were in the sample); `robots.txt` returns a 404 page; terms path I guessed returned 404 so terms unread `[U]`; demographic skew unknown `[U]` | **RSS**; strip `dc:creator` and `author` at ingest |
| **r/singapore** | `https://www.reddit.com/r/singapore/.rss` | `[V]` one 200, others 429 | see section 9 | with caution |
| **Mothership** | `https://mothership.sg/feed/` | `[V]` 10 items, newest 2026-10-13 | news with a youth skew; comments are not in the feed | **RSS** |
| **Straits Times Asia, CNA (SG)** | `straitstimes.com/news/asia/rss.xml` (50), `channelnewsasia.com/api/v1/rss-outbound-feed?_format=xml` (20) | `[V]` | news | **RSS** |
| **Zaobao** | `zaobao.com.sg/rss/realtime/china`, `zaobao.com/rss` | `[V]` 404 | feed path unknown `[U]` | Find via publisher page |
| **REACH (Singapore feedback unit)** | `https://www.reach.gov.sg/` | `[V]` 200 HTML only (867 KB consultation page); no open feedback data found | citizen feedback is not published as data `[U]` | Not feasible |
| **Singapore Parliament Hansard** | `sprs.parl.gov.sg/search/` | `[V]` 200 small stub | not explored | `[U]` |
| **Lowyat Forum (Malaysia)** | `https://forum.lowyat.net/` | `[V]` 200 HTML; RSS endpoint `?act=rssout&id=1` 403; robots.txt allows most paths, disallows `/index.php` and `/lofiversion`, crawl-delay 2 for named bots | consumer-tech forum, English/Malay | Page poll possible, ToS unread `[U]` |
| **Free Malaysia Today** | `freemalaysiatoday.com/feed/` | `[V]` 50 items, newest 2026-10-13 | English news | **RSS** |
| **Malaysian Chinese-language press** (Sin Chew, China Press, Oriental Daily, Nanyang) | guessed `/feed`, `/rss`, `/rss.xml` | `[V]` Sin Chew 404 (also `?feed=rss2` returned HTML), China Press 403, Oriental Daily 404, Nanyang 404 | no working feed found | Use GDELT or page diff; not RSS |

Southeast Asian Chinese-language communities largely live on Facebook, WhatsApp and local portals; none is reachable by a free documented feed from what I tested. Treat that tier as "news-outlet coverage via GDELT/Google News" until the owner decides on Meta access.

## 9. Reddit and English-language forums (brief item 7)

| Fact | Evidence |
|---|---|
| `https://www.reddit.com/robots.txt` says `User-agent: *` then `Disallow: /`, and points to the Public Content Policy and `r/reddit4researchers` | `[V]` |
| `r/taiwan/.rss?limit=100`: HTTP 200 Atom, 100 entries spanning 2026-10-07 to 2026-10-11 (about 25 posts/day); `r/sino/.rss`: 100 entries spanning 2024-08 to 2026-10 (low volume) | `[V]` |
| `r/China`, `r/singapore`, `r/malaysia`, `r/HongKong`, `r/China_irl`, `r/taiwanese`: HTTP 429 when requested within seconds of each other; `/new.json` HTTP 403; `/top/.rss?t=week` and `/comments/.rss` 429 | `[V]` |
| Free Data API tier is about 100 queries/min per OAuth client for non-commercial use; commercial use needs a separate agreement; widely reported commercial price about 0.24 USD per 1,000 calls; self-service registration reportedly closed in late 2025; a 12,000 USD minimum is reported inconsistently as per month or per year | `[S]` secondary sources only: `https://www.techloy.com/reddit-api-pricing-in-2026-complete-guide-for-developers-and-businesses/`, `https://www.xpoz.ai/blog/guides/reddit-api-pricing-tiers-and-alternatives/`, `https://octolens.com/blog/reddit-api-pricing`. I did not fetch Reddit's own pricing or terms pages `[U]` |

Slice: English-speaking, global, male-skewed, expat- and diaspora-heavy; not Taiwanese public opinion. **Recommendation:** skip as a Tier 1 source. If wanted, poll `r/taiwan` and one or two others at a low frequency (every few hours, one subreddit per request with spacing) after counsel reviews the Public Content Policy. Mapping: RSS connector, per-subreddit.

## 10. Methods: measuring opinion from discourse cheaply (brief item 8)

### 10.1 What the evidence says

- **Stance classification on PTT.** A lexicon-based classifier for Mandarin PTT comments reached about 71% average accuracy on a binary task (Chuang and Hsieh) `[S]` via search summary; most stance work is single-author documents, whereas PTT comments are multi-author, which a 2022 PACLIC paper treats explicitly `[S]` `https://aclanthology.org/2022.paclic-1.45.pdf`.
- **Online sentiment vs polls.** 2016 presidential election: online data had more predictive power and timeliness than polls, but offline polls were useful for correcting online sample bias, and Facebook "likes" were the strongest indicator `[S]` `https://ar5iv.arxiv.org/html/1803.08010`. 2022 Taipei mayoral election (NTU study): telephone polls closest to the election, after excluding the undecided, nearly matched the vote; internet big-data indicators ("total volume", "sentiment volume", "favorability") were inconsistent with the result `[S]` `https://tdr.lib.ntu.edu.tw/jspui/handle/123456789/88430`. I found no 2024-presidential comparison `[U]`. A study of unification-independence sentiment found PTT results not different from surveys `[S]` `https://politics.ntu.edu.tw/psr/?p=3580`.
- **Platform bias is general.** Pew's 2013 comparison found Twitter reactions matched public opinion in only two of eight events and attributed the gap mainly to demographics `[S]` `https://www.pewresearch.org/internet/2013/03/04/twitter-reaction-to-events-often-at-odds-with-overall-public-opinion/`. I did not find PTT-specific age/gender data `[U]`.
- **Deliberation tooling.** Talk to the City (Apache-2.0) clusters free-text opinions with an LLM and presents them with attributable summaries; the Taiwan alignment assembly pairs a random-sample frame (200,000 SMS, 447 selected) with Pol.is `[S]` (section 3.2). The sampling frame, not the tool, is what makes it representative.

### 10.2 Cheap design that avoids single-platform bias

1. **Three frames, never merged.** (A) probability-sample anchors: NCCU, MAC, TPOF; (B) self-selected civic channels with known skew: JOIN, Cofacts, LY interpellations; (C) platform discourse samples: Threads (after approval), PTT Web titles, Reddit, forum RSS, news opinion columns. Report each frame separately; comparison is the output.
2. **Topic prevalence before stance.** Topic shares are far cheaper and more robust than stance shares. Add stance only for topics with enough volume and a gold set.
3. **Unit of analysis is the author-window, not the post.** One vote per author per topic-week (or per thread if authors are not stored) to blunt brigading and heavy posters.
4. **Cluster-aware intervals.** Comments within a thread are correlated; use thread-level bootstrap or a design effect, not a plain binomial interval.
5. **Gold set per language and platform.** 300-500 human-labelled items per frame gives a measured precision/recall, which becomes part of each observation. LLM labelling is cheap (the prior report costs a Haiku-class call at about 0.00028 USD per 600-character item) but needs the gold set to be trusted.
6. **No cross-platform weighting without demographics.** Without age/gender/region on posts, post-stratification is fiction. State the platform's known skew (section 5.1) in a `known_skew` field instead.

## 11. Ethics, privacy and law (brief item 9)

### 11.1 Taiwan PDPA

From the Ministry of Justice English text (page shows "Amended Date: 2025-11-11", site last update 2026/10/02; the Chinese version prevails) `[S]` `https://law.moj.gov.tw/ENG/LawClass/LawAll.aspx?pcode=I0050021`:

- **Art. 19 (non-government collection and use):** allowed where the data subject has "manifestly made public" the data or it was lawfully publicised (sub-para. 3), or where data are from publicly available sources unless the subject has an overriding interest in prohibiting processing (sub-para. 7). The academic-research exemption (sub-para. 4) is limited to academic institutions pursuing public interest, with de-identified results; a company cannot rely on it.
- **Art. 6 (sensitive data):** the English text lists medical, healthcare, genetic, sex life, physical examination and criminal-record data; political opinion is not on that list. This is easy to misread by analogy to GDPR, where political opinions are a special category.
- **Art. 20:** use beyond the original collection purpose is restricted; marketing use must stop on objection.
- **Arts. 41-42:** criminal liability requires intent to gain unlawfully or harm another's interests; up to 5 years and NT$1,000,000 fine.
- **2025 amendment:** promulgated 2025-11-11; creates an independent Personal Data Protection Commission, new breach-notification duties for non-government entities and inspection powers; as of 2026-01-13 the Executive Yuan had not set a commencement date and the commission's establishment was not confirmed `[S]` `https://www.klgates.com/New-Developments-in-the-Taiwan-Personal-Data-Protection-Act-1-13-2026`. I did not find whether it changed the public-data exemptions `[U]`. Check status before launch.

**Reading for Choir.** Aggregate-only counting of public posts is defensible under Art. 19; the risk grows with identifiable storage, profiling, or republishing a named person's statement. Keeping identity out of storage removes most of the exposure.

### 11.2 Other jurisdictions (thin evidence)

- **Singapore:** the PDPA has a publicly-available exception; PDPC's generative-AI guidance says organisations may rely on it for scraping if the data are genuinely publicly accessible and use is reasonable, with consultation open until 2026-07-01 `[S]` `https://www.stephensonharwood.com/insights/singapores-pdpc-issues-new-advisory-guidance-on-use-of-personal-data-in-generative-ai/`; current status unverified `[U]`.
- **Hong Kong:** a joint statement of twelve regulators, including Hong Kong, said there is no general exemption for publicly available data under their laws `[S]` (search summary; the underlying statement was not fetched) `[U]`.
- **Malaysia PDPA, GDPR Art. 14(5)/89, PRC PIPL:** not verified. GDPR matters only if EU data subjects are targeted; PIPL has extraterritorial reach in principle `[U]`.

### 11.3 Platform terms and robots

| Platform | What I verified |
|---|---|
| Reddit | robots disallows all `[V]`; commercial terms secondhand `[S]` |
| Cofacts | scraping banned except low-frequency public crawlers; dataset licence CC BY-SA 4.0 `[V]` |
| Threads | official API; aggregate-analytics terms unread `[U]` |
| Telegram | prior report: terms prohibit scraping and AI/ML use since about 2026-02 `[S]` |
| PTT, Dcard, LIHKG, Mobile01 | no terms retrievable; all block by 403 `[V]` |
| HardwareZone | no robots; terms unread `[U]` |
| Disp.cc | robots blocks AI bots `[V]` |
| Weibo, Zhihu, etc. | cookie/visitor walls; PRC unfair-competition precedents `[S]` |

### 11.4 Should usernames be stored?

**No.** For topic and stance shares you need: a stable key per author per window (to dedupe and cap), not an identity. Recommended: keyed hash (HMAC) of the platform user id with a window-scoped key that is destroyed when the window closes; store no handle, no profile link, no avatar. Where the source supplies a username in the payload (HardwareZone `dc:creator`, JOIN `提議者`, Mastodon account, Threads `username`) drop it at ingest, before persistence. Keep the post permalink only if the item is retained as a cited document; for aggregate-only frames store neither text nor link beyond the retention window. Elected officials' and public bodies' speech (LY votes, interpellations, Executive Yuan) is a separate class and may be stored and cited by name.

## 12. Turning discourse into aggregate, privacy-preserving observations (deliverable c)

### 12.1 Observation record (what is written to the ledger)

One row per (frame, source, window, topic [, stance]):

```json
{
  "kind": "discourse_observation",
  "window": {"start": "2026-10-05", "end": "2026-10-11", "tz": "Asia/Taipei"},
  "frame": "B_civic_self_selected",
  "source": "join_proposals",
  "query_def": {"topic_id": "labour_import", "definition_hash": "sha256:..."},
  "denominator": {"total_items_in_window": 91, "total_items_source_published": 91,
                  "coverage_ratio": 1.0, "collection_failures": 0},
  "n_items_on_topic": 14,
  "n_distinct_author_keys_bucket": "10-19",
  "share": 0.154, "interval": {"method": "wilson", "lo": 0.09, "hi": 0.24, "clustered_by": "thread_or_item"},
  "stance": null,
  "classifier": {"model": "<id>", "gold_n": 400, "precision": null, "recall": null},
  "salience": {"endorsements_total": 52113, "comments_total": 810},
  "known_skew": "registered, self-selected petitioners; organised campaigns inflate counts",
  "not_a_claim_about": "national opinion",
  "anchor": {"series": "mac_status_quo", "fieldwork": "2026-08-13..17", "ref": "..."},
  "privacy": {"identity_stored": false, "text_stored": "none | excerpt | link_only"}
}
```

### 12.2 Rules

- **Denominator and sampling frame in every row.** Items seen, items the source says exist (when knowable: JOIN total, LYAPI `total`), items lost to errors.
- **Minimum cell size.** Publish or hand to management only cells with at least 20 distinct author keys (or at least 20 items for sources without authors, such as Cofacts reports); otherwise merge topics or widen the window.
- **Share, not count, plus the count.** Cheap sources are small (r/taiwan about 25 posts/day; Mastodon about 1/day), so report intervals and let the dashboard suppress what is noise.
- **Salience and stance are different columns.** JOIN endorsements measure mobilisation; Cofacts report counts measure rumor exposure; LY interpellations measure elite attention; poll anchors measure population attitude.
- **Hypotheses for management** are phrased against frame and anchor: "In frame B the share of proposals mentioning labour import rose from 4% to 15% in two weeks while the MAC anchor did not move; the frame is self-selected; propose weekly tracking and a gold-set review." Management forms hypotheses; the ledger never says "Taiwanese think X."
- **Re-run determinism.** `definition_hash` pins the keyword/classifier definition so a later restatement is a new observation, not an edit.

### 12.3 Pipeline sketch within the existing shape

1. Pollers write documents with `store_body_policy: excerpt_only` (or `none` for frame C) and no author field.
2. A weekly job classifies topic (and stance where a gold set exists), then writes observation rows and drops text beyond retention.
3. Anchors (MAC CSV, NCCU PNG hand-entered, TPOF) are written as separate evidence records with their own fieldwork dates and sample sizes.

## 13. Recommendations

### (a) Five highest-value discourse sources to add first for Taiwan, cheap and legal

| # | Source | Why | Class | Effort |
|---|---|---|---|---|
| 1 | **JOIN proposals** (open-data JSON, plus RSS) | Free, daily, OGDL licence, 4,604 proposals so far in 2026 with endorsement and comment counts; directly measurable salience | Dataset / JSON-diff | small |
| 2 | **LYAPI** (bills, interpellations, votes, IVOD transcripts) | Free CC-BY-4.0, ~150k bills and 105k videos with transcripts; what legislators ask and say about US-China, chips, budgets | JSON-diff | small-medium |
| 3 | **Cofacts dataset + TFC + MyGoPen** | The only look at LINE rumor traffic, with licence settled; TFC and MyGoPen feeds fresh and full | Dataset + RSS | small (accept agreement first) |
| 4 | **Poll anchors**: MAC CSVs, NCCU trend pages, TPOF feed | The only probability-sample anchors; make every other observation interpretable | Dataset + page diff + manual entry | small |
| 5 | **Opinion-column RSS** (LTN opinion `https://news.ltn.com.tw/rss/opinion.xml` 40 items; UDN opinion `https://udn.com/rssfeed/news/2/6643?ch=news` 20 items) with the prior report's news feeds | Free, legal under the same reuse caveats as the prior report, fresh to the minute; elite and partisan commentary from two outlets with different leanings (leaning not verified here) | RSS | none (rows only) |

**File in parallel (long lead time):** a Threads API app for `threads_keyword_search`; the Cofacts agreement (and an `app-id` request if the API is ever wanted); a researcher partnership (NCCU DSPOP programme is named on the ESC site, other options open) for Meta Content Library; legal review of the Reddit policy. **Owner decision, not recommended by default:** PTT Web title/count polling.

### (b) Sinosphere tier plan

| Tier | Contents | Method | Gate |
|---|---|---|---|
| **0: anchors and editorial** (now) | CDT, China Media Project, What's on Weibo, Sinocism, RFA Mandarin, Initium, HKFP, inmediahk, The Witness, DFRLab, IORG report list (page diff), plus the prior report's news feeds | RSS (existing) | Publisher terms read once |
| **1: PRC platform rankings, aggregate snapshots** (after PRC counsel) | Baidu hot, Douyin hot, Toutiao hot, Bilibili popular/search-square; hourly or 4-hourly rank + heat; store only keywords, rank, heat value, timestamp | Aggregate snapshot, new light JSON poller | Counsel sign-off; "platform-curated, not opinion" label on every row |
| **2: diaspora and regional forums** | HardwareZone RSS (drop usernames), Free Malaysia Today, Mothership, Straits Times/CNA; Mastodon and Bluesky account lists; `r/taiwan` low-frequency RSS if counsel agrees | RSS | Read HardwareZone terms; never store handles |
| **3: do not build** | Weibo, Zhihu, WeChat, Xiaohongshu, LIHKG, Dcard, PTT direct, Mobile01, REACH | none | Revisit only through a research partner, official API, or a licence |
| **Gaps** | Chinese-language Malaysian press, Zaobao, Facebook groups | GDELT/page diff; Meta Content Library via partner | Owner decision |

### (c) Aggregate observations

See section 12: weekly `discourse_observation` rows per (frame, source, topic) with denominator, coverage, cluster-aware interval, classifier performance, known skew, anchor reference, and a non-identity privacy block; minimum cell size 20; HMAC window-scoped author keys only where deduplication is needed; username fields dropped at ingest.

### (d) Open questions for the owner

1. **Commercial posture.** Is Autopaper commercial? It decides CNA's non-commercial RSS terms (prior report), Reddit, and whether Cofacts' CC BY-SA share-alike needs counsel before you publish derived aggregates.
2. **PTT.** Accept PTT Web (title and push-count polling, aggregate-only) with its legal ambiguity, or defer until a research partner or licence is available?
3. **Meta access.** Apply for the Threads API now? Seek a university or non-profit partner for Meta Content Library (the access constraint is real for for-profit publishers `[S]`)?
4. **PRC touch.** Is polling PRC platform hot lists from production egress acceptable before PRC counsel reviews it?
5. **Stance appetite.** Topic prevalence first, stance later? Who labels the gold sets (owner, paid annotators, LLM with spot checks) and in which languages (zh-TW, zh-CN, yue, English)?
6. **Publication.** Are observations internal inputs for management hypotheses only, or published to readers (which raises defamation and misrepresentation standards)?
7. **Telegram.** The prior report's unresolved decision remains and interacts with Taiwan Telegram sources.
8. **Retention.** Default retention for frame C text (suggest none or seven days) and for frame B excerpts.
9. **TPOF.** Treat its feed as dead, or contact them (the feed shows nothing after 2025-12-31)?
10. **Anchors by hand.** Is manual transcription of the NCCU PNG charts (twice a year) acceptable, or should the NCCU microdata application be pursued?

## 14. What I could not verify

- Threads API behaviour (needs an approved app); Meta Platform Terms for aggregate analytics; whether a for-profit publisher can get `threads_keyword_search`.
- PTT, Dcard, LIHKG and HardwareZone terms of service (inaccessible or unread); PTT's demographic profile; any official PTT research access.
- TPOF's 2026 status; My-Formosa, TVBS and TSCS (unreachable from this egress); NCCU microdata terms; the party-preference page URL.
- Reddit's own current API pricing/terms (only third-party reports).
- PRC law application to a foreign company polling public hot lists; PIPL, Malaysian PDPA and GDPR specifics; Hong Kong PCPD guidance.
- LYAPI rate limits; JOIN site terms; Cofacts update cadence; Doublethink Lab and IORG data downloads.
- Whether the 2025 PDPA amendment is in force and whether it changed public-data exemptions.
- The Witness HK's lineage and any Stand News successor; Malaysian Chinese-press feed paths.
- Whether `rss.go` decodes the JOIN feed's UTF-16 correctly (I decoded it separately; the connector was not run).

## 15. Test log (selected, 2026-10-13, polite UA unless noted)

| Endpoint | Result |
|---|---|
| `ly.govapi.tw/v2/bills?limit=2` | 200 JSON, total 148,164 |
| `ly.govapi.tw/v2/ivod/17731` | 200, `transcript.whisperx` 1,141 segments |
| `join.gov.tw/toOpenData/v2/ey/idea?year=2026` | 200 JSON 13.9 MB, 4,604 records |
| `join.gov.tw/rss/feed` | 200 RSS UTF-16, 61 items |
| `data.gov.tw/datasets/export/json?dnid=58297` | 200 JSON 90 MB catalogue, 52,503 datasets |
| `mac.gov.tw/big5/data/218_*.csv`, `42536_*.csv` | 200 CSV |
| `esc.nccu.edu.tw/PageDoc?fid=7800`, `7801` | 200 HTML (charts as PNG) |
| `tpof.org/feed`, `/wp-json/wp/v2/posts` | 200; newest 2025-12-31 |
| `huggingface.co/api/datasets/Cofacts/line-msg-fact-check-tw` | 200; gated auto; CC BY-SA 4.0 tag |
| `api.cofacts.tw/graphql`, `cofacts.tw` | 403 Cloudflare |
| `ptt.cc` (UA, browser UA + `over18`) | 403 |
| `pttweb.cc/hot/all/today`, `/bbs/HatePolitics` | 200 SSR HTML |
| `dcard.tw/service/api/v2/posts`, `lihkg.com/api_v2/...`, `mobile01.com` | 403 |
| `reddit.com/r/taiwan/.rss?limit=100` | 200, 100 entries; others 429; `.json` 403 |
| `top.baidu.com/api/board?...`, `aweme.snssdk.com/...hot/search/list/`, `toutiao.com/hot-event/hot-board/` | 200 JSON |
| `api.bilibili.com/x/web-interface/popular`, `.../search/square`, `s.search.bilibili.com/main/hotword` | 200 JSON |
| `weibo.com/ajax/side/hotSearch`, `zhihu.com/api/v3/feed/...` | 403, 401 |
| `forums.hardwarezone.com.sg/forums/-/index.rss` | 200 RSS, 19 items, full posts |
| `inmediahk.net/rss.xml`, `thewitnesshk.com/feed/` | 200 RSS |
| `tfc-taiwan.org.tw/feed`, `mygopen.com/feeds/posts/default`, `dfrlab.org/feed/` | 200 |
| `chinadigitaltimes.net/feed/`, `whatsonweibo.com/feed/`, `chinamediaproject.org/feed/`, `sinocism.com/feed` | 200 RSS |
| `mastodon.social/api/v1/timelines/tag/台灣` | 200, 40 posts across 33 days |
| `g0v.social/api/v2/instance` | 200, 1,019 monthly active |
