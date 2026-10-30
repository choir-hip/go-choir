# Source coverage-gap analysis of the paper's 28 sources, and a first primary-source registry (2026-10-20)

Mutation class: green (docs only). No code, registry, schedule or staging
state was changed. Working document, not doctrine.

Rulings this serves (owner notes §27, 2026-10-20): item 6, "run the
coverage-gap analysis of the paper's 28 sources before adding sources";
item 11, "start the primary-source registry, licences checked first"; item
12, stay on Keenable's free tier. Standing rulings it must respect: Taiwan
is the first vertical and general plus tech news is vertical zero
(owner-roadmap-rulings 4); Autopaper is commercial, so sources licensed
only for non-commercial use stay out (rulings 3); no PTT or Dcard
scraping (9); RSSHub for China later (10); Telegram through MTProto later
(§25); track outlets separately from claim-makers and never publish with
fewer than two outlets (§24); scale ingestion without more model calls
(§24).

## 1. Findings in one screen

1. The 28 scheduled sources are 19 outlets, 7 link aggregators and 2
   primaries. No wire. No Taiwan, East Asia, Africa, Latin America or
   Middle East source. Everything is English. Sixteen of 28 are tech.
2. That matches the measured symptom: 4 of 113 cited sources on the
   front page were primary records (grounding report, 2026-10-19). The
   registry already holds 11 `official:` rows and none is scheduled.
3. About 110 candidate URLs were probed live and 31 are ranked. The
   first wave (section 5, Wave 1 rows) is registry-only, verified to exist, and
   adds roughly 25 to 60 items a day in total. Today's intake is about
   270 captures an hour (management-intake-backlog doc), so the whole
   first wave is well under 1 percent of current intake. It adds
   captures, not model calls, which is what §24 asked for.
4. Taiwan government feeds mostly exist and several are cleanly
   licensed (Open Government Data License v1.0, which allows commercial
   use with attribution). But three of the best (Central Bank, Ministry of
   Finance, MOFA) are 2.3 to 3.4 MB, and `internal/sources/rss.go` reads
   at most 2 MiB (`io.LimitReader(resp.Body, 2<<20)`) and then parses
   with a strict-off XML decoder, so a truncated body very likely fails
   to parse, as `rss:dw_en` did on 2026-10-11. That is one small orange
   code change (a per-source byte cap), not a registry-only row.
5. Two existing assumptions need an owner or counsel look: the UN News
   terms say personal, non-commercial use only (three `official:un*` rows
   are in the registry, none scheduled), and the WHO publishes under CC
   BY-NC-SA 3.0 IGO. BBC supplies 3 of the 28; its RSS terms were not
   readable in this pass (see section 6).
6. The Taiwan local elections are on 28 November 2026 (nine-in-one,
   11,051 seats; ballot-number draw 23 October, per CEC statements
   relayed by Focus Taiwan; confirmed by a web search, not a CEC page).
   The Central Election Commission has no feed. It publishes a
   Government-Data-License CSV on data.gov.tw after each election. A
   dataset or results connector for that date is the single most
   time-boxed gap.

## 2. Method and confidence

The live schedule list is only readable through the paper's owner API
(`GET /api/autopaper/sources/schedules`, handoff-2026-10-15 §1), which
needs the paper credential. This analysis did not read it (read-only
repo and public web only), so the 28 are reconstructed from the repo:

- 14 are named in `docs/archive/evidence/c2-autopaper-live-2026-10-13.json`
  (owner act 15:22 and tier 1 at 15:28 UTC, 2026-10-13): `rss:bbc_world`,
  `rss:guardian_world`, `rss:npr_world`, `arxiv:cs_ai`, `rss:techcrunch`,
  `rss:theverge`, `rss:wired`, `rss:ars`, `rss:venturebeat`,
  `rss:theregister`, `rss:technologyreview`, `rss:openai_news`,
  `rss:hn_frontpage`, `rss:hn_best`.
- 8 are the politics and business rows added in commit d7554d69d
  (2026-10-14): `rss:npr_politics`, `rss:politico_politics`,
  `rss:thehill_news`, `rss:pbs_newshour_politics`,
  `rss:guardian_us_politics`, `rss:bbc_us_canada`, `rss:bbc_business`,
  `rss:guardian_business`. Docs say "28 at 14:27".
- 6 were added at 14:15 UTC, 2026-10-14 ("14 plus 6"), and no doc lists
  them. I infer `rss:hn_newest`, `rss:hn_show`, `rss:hn_ask`,
  `rss:lobsters`, `rss:slashdot`, `rss:dw_en`. Evidence: the HN doc says
  five hnrss.org feeds are polled every 900 s (frontpage, newest, show,
  best, ask), mentions Lobsters and Slashdot descriptions in the same
  breath, and names `rss:dw_en` polls on 2026-10-17. This is the weakest
  link: confirm against the schedule list before acting on the exact
  tallies. Nothing in the ranking depends on it.

Feed checks (section 5) were single GETs on 2026-10-20 about 02:00 to
02:40 UTC from a developer machine with the registry's own user agent
(`ChoirUniversalWire/0.1 (+https://choir.news)`). That egress is not the
paper's. "Verified" below means: HTTP 200, the body parsed as RSS, Atom or
JSON, and the newest item was within about two weeks (or the feed is a
low-volume series, noted). It does not mean licence cleared. Licence status
is stated separately and says what I actually read.

## 3. The current 28

Sections are the four front-page sections Jev assigns
(`internal/frontimportance/scorer.go`: world, us_politics, business,
tech_ai). Type: outlet = a publisher reporting on events; primary = the
origin of a document, statement or dataset; wire = originates reports for
other outlets; aggregator = a link list whose capture is mostly a pointer
(see docs/problems/link-only-capture-counts-as-an-outlet-2026-10-17.md).

| # | id | origin | section | type |
|---|---|---|---|---|
| 1 | rss:bbc_world | UK, global desk | world | outlet (public broadcaster) |
| 2 | rss:guardian_world | UK | world | outlet |
| 3 | rss:npr_world | US | world | outlet (public broadcaster) |
| 4 | rss:dw_en (inferred) | Germany | world | outlet (public broadcaster) |
| 5 | rss:bbc_us_canada | UK | us_politics | outlet |
| 6 | rss:npr_politics | US | us_politics | outlet |
| 7 | rss:politico_politics | US | us_politics | outlet |
| 8 | rss:thehill_news | US | us_politics | outlet |
| 9 | rss:pbs_newshour_politics | US | us_politics | outlet |
| 10 | rss:guardian_us_politics | UK | us_politics | outlet |
| 11 | rss:bbc_business | UK | business | outlet |
| 12 | rss:guardian_business | UK | business | outlet |
| 13 | rss:techcrunch | US | tech_ai | outlet |
| 14 | rss:theverge | US | tech_ai | outlet |
| 15 | rss:wired | US | tech_ai | outlet |
| 16 | rss:ars | US | tech_ai | outlet |
| 17 | rss:venturebeat | US | tech_ai | outlet |
| 18 | rss:theregister | UK | tech_ai | outlet |
| 19 | rss:technologyreview | US | tech_ai | outlet |
| 20 | arxiv:cs_ai | global | tech_ai | primary (preprints, no peer review) |
| 21 | rss:openai_news | US | tech_ai | primary (a company's own statements) |
| 22 | rss:hn_frontpage | US | tech_ai | aggregator |
| 23 | rss:hn_best | US | tech_ai | aggregator |
| 24 | rss:hn_newest (inferred) | US | tech_ai | aggregator |
| 25 | rss:hn_show (inferred) | US | tech_ai | aggregator |
| 26 | rss:hn_ask (inferred) | US | tech_ai | aggregator |
| 27 | rss:lobsters (inferred) | US | tech_ai | aggregator |
| 28 | rss:slashdot (inferred) | US | tech_ai | aggregator |

Tallies: outlets 19, aggregators 7, primaries 2, wires 0. By section:
tech_ai 16, us_politics 6, world 4, business 2. Distinct outlet
organisations among the 19: 14 (BBC counts 3 rows, Guardian 3, NPR 2), so
a "two outlets" check sees 14 independent publishers, 13 of them
US or UK. Region: United States or United Kingdom for 26 of 28; Germany
one (DW); global one (arXiv). Language: English only.

## 4. Gaps

Against the sections: business has two feeds, both general-news business
desks, and no market, regulator or company-filing source. us_politics is
six outlets that all report the same Washington stories and no primary
record of any of them (no White House, DOJ, court or agency release).
world is four English-language public or liberal-leaning outlets from
three countries. tech_ai is the best covered by count but mostly the same
three or four stories seen 16 times, and its two primaries are one
company's blog and a preprint server.

Against vertical zero (general news): no wire, so no originating copy of
anything; no regional press from Africa, Latin America, Middle East or
Asia, so "the long tail from the global south" the owner asked for on
2026-10-14 is absent from what is scheduled, though 46 non-US, non-English or
regional RSS rows are already in the registry, none of them scheduled (e.g. `rss:scmp_asia`,
`rss:straitstimes_asia`, `rss:cna_asia`, `rss:japantimes`,
`rss:abc_au_world`, `rss:africanews_en`, `rss:aljazeera_all`,
`rss:france24_*`, `rss:rfi_*`). Scheduling registered rows is the cheapest
way to widen outlets and costs no code; this document does not rank them
because they are outlets, not primaries.

Against the Taiwan vertical: zero. No Taiwanese government body, no
Taiwanese outlet, no Chinese-language source, no East Asian wire. The L4
station's oracles named in choir-state-and-vision (TWSE monthly revenue,
election commission data, CISA KEV) have no ingestion path at all, and
the connector surface is RSS, Telegram and GDELT only (no JSON or
dataset poller).

Against the grounding goal (primary records): 2 of 28 are primaries and
neither is a government or court. The registry's 11 `official:` rows (Fed,
ECB, EC Press Corner, UN News x3, WTO, NASA, NOAA NHC, USGS, CDC US) are
all unscheduled. Scheduling the Fed, ECB, EC and WTO rows is the quickest
macro primary win; see section 5 note A.

Against the outlet/claim-maker rule (§24): the registry has `official` and
`source_standing` fields on `Source` but no source sets `source_standing`
(it exists in `internal/sources/types.go` and nothing reads or writes it). A claim-maker class (government, company, court,
state media) needs values there before ingestion scales, so the
statistics per outlet and per claim-maker stay separate. Suggested values:
`primary_government`, `primary_company`, `primary_court`, `wire`,
`state_media`, `outlet`, `aggregator`. State media (TASS, Kremlin, Anadolu,
RTI) are claim-makers about their state, not neutral outlets.

Operational limits that shape what is addable without code:

- Connector set is `rss`, `telegram`, `gdelt`, `polymarket`. The RSS
  connector reads Atom, RSS 2.0 and RDF, and any charset (UTF-16
  included). It reads 2 MiB at most and takes the first
  `max_items_per_poll` items in document order.
- No JSON-diff or dataset connector exists (LYAPI, TWSE, CISA KEV, the CEC
  CSV all need one; the 2026-10-13 sourcing report already recommended it
  as connector 3).
- User agent: SEC asks for a contact in the agent string. `Source` has a
  per-source `user_agent` override, so this is a registry field.
- TLS: `stat.gov.tw` (DGBAS) serves an incomplete certificate chain and
  failed verification in both curl and Python here. Several other
  Taiwanese government hosts (`president.gov.tw`, `mofa.gov.tw`,
  `cbc.gov.tw`, `fsc.gov.tw`, `cwa.gov.tw`) failed Python's bundled trust
  store but passed curl on macOS. The guest's trust store has to be
  tested before scheduling any Taiwan government feed; I could not.
- Some hosts block unfamiliar agents or regions (403 here): State Dept,
  FCC, OECD, USDA, MOEA, MAC, NDC, GlobeNewswire, METI, MOFA Japan, EFE,
  NK News, Hong Kong Free Press (this run). Retest from the production
  egress before dropping.

## 5. Proposed additions, ranked

Ranks are global across the three tables below (1 to 31). Ranking rule: grounding value for stories the paper already runs and for
Taiwan, then licence cleanliness, then verification, then no-code.
"Wave 1" is registry-only (a row, no code). "Wave 2" needs a small code
change (byte cap or a JSON/dataset connector). "Wave 3" waits on a licence
or owner ruling. Volume is observed items per day from the feed's own
dates on 2026-10-20, rounded; for a feed that lists edits it can
overstate. "V" = verified live as above; "U" = unverified (not fetched, or
fetch failed).

### Taiwan government (vertical one)

| # | Source and feed URL | Format | Licence or terms finding | Volume | Status | Why it fills a gap |
|---|---|---|---|---|---|---|
| 1 | Executive Yuan press, zh `https://www.ey.gov.tw/RSS_Content.aspx?ModuleType=1` and en `https://english.ey.gov.tw/RSS_Content.aspx?ModuleType=1` | RSS 2.0, 50 items, about 900 chars each, same items in two languages | English site footer reads "All Rights Reserved"; I did not find the government website open-data declaration page for this host (U). Taiwanese agencies usually link one; the CBC's is OGDL (below). Treat as excerpt-only until read | about 2 a day | V (newest 2026-10-19) | The Cabinet's own statements: policy decisions, spokesperson lines, ministerial changes. Removes "an official told DW" from Taiwan stories. Wave 1 |
| 5 | Central Bank (CBC) press releases `https://www.cbc.gov.tw/tw/rss-302-1.xml` | RSS 2.0, 500 items, 3.4 MB, full HTML body | Verified: the CBC statement grants OGDL v1.0, "free of charge, non-exclusive, and sublicensable", attribution required, copyright only (logos excluded) | about 1 a day | V (newest 2026-10-19), but over the 2 MiB read cap | Exchange-rate and rate decisions at source; the 2026-10-19 NT dollar statement is in it. Wave 2: needs a per-source byte cap |
| 8 | Taiwan CDC press releases `https://www.cdc.gov.tw/RSS/RssXml/Hh094B49-DRwe2RR4eFfrQ?type=1` | RSS 2.0, 10 items | Footer links a "government website open data declaration", not read (U); US CDC is a different body | about 1 a day | V (newest item 2026-10-13) | Outbreak and public-health statements at source; pairs with the WHO question below. Wave 1 |
| 9 | Central Election Commission election database `https://data.cec.gov.tw/選舉資料庫/voteData.csv` (data.gov.tw dataset 13119) | CSV, no feed | Verified on data.gov.tw: OGDL v1.0, free, "updated 7 days after each election" | event-driven, one batch per election | V as a dataset (the CSV URL answered a 308 redirect; the file itself not downloaded); no CEC RSS found, `web.cec.gov.tw` is a script-rendered site and its robots.txt returned "FORBIDDEN" | 28 Nov 2026 nine-in-one election. Historical results as a baseline; live results during the count are a different surface, not found. Wave 2: dataset connector, with a news-site ingestion fallback if CEC blocks the agent |
| 11 | MOFA (zh) open data `https://www.mofa.gov.tw/OpenData.aspx?SN=8ABA444B9AC1360F` | RSS, 1000 items, 2.3 MB | OGDL v1.0 per data.gov.tw dataset 7668 (cited in the 2026-10-13 report; not re-read) | about 2 a day | V (newest 2026-10-19/20), over the 2 MiB cap | Taiwan's foreign-ministry statements on China, US, Japan and diplomatic allies. Wave 2: byte cap and `max_items_per_poll` 30 |
| 12 | DGBAS (statistics) press releases `https://www.stat.gov.tw/OpenData.aspx?SN=475CDEA90329C775` | RSS 2.0, 10 items, 134 KB | Served under the government open-data scheme (OpenData endpoint); declaration not read (U) | about 1 a week (GDP, CPI, jobs releases) | V (newest 2026-10-15), but the site fails TLS chain verification | The official statistics office: GDP, prices, employment. Wave 2: needs the guest to trust it, or an operator-side fix; a plain row fails otherwise |
| 13 | Ministry of Finance news `https://www.mof.gov.tw/Rss/384fb3077bb349ea973e7fc6f13b6974` | RSS 2.0, 1,229 items, 2.7 MB | Footer reads 版權所有 (copyright reserved); MOF notice not read (U). Another MOF notice found by search says it releases under OGDL, so likely fine but unconfirmed | about 3 a day | V (newest 2026-10-19), over the cap | Customs trade totals, tax, exports: the numbers behind the TSMC and chips stories. Wave 2 |
| 14 | Presidential Office `https://www.president.gov.tw/RSSNEWS.aspx` | RSS, 9 items | Footer "All rights reserved"; policy page not read (U) | about 3 a day | V (newest 2026-10-18) | Presidential statements. Lowest priority of the six because the licence is least clear |
| 17 | Legislative Yuan API `https://ly.govapi.tw/v2/bills`, `/ivods`, `/meets`, `/votes`, `/interpellations` | JSON, no RSS | CC BY 4.0 data, BSD-3 code per the 2026-10-13 Taiwan report (V then); API re-confirmed 200 today (148,183 bills) | bursty; depends on the session calendar | V | What legislators ask and vote, with transcripts. Wave 2: JSON-diff connector |
| 18 | TWSE material announcements `https://openapi.twse.com.tw/v1/opendata/t187ap04_L` | JSON array, 53 rows today | Swagger page shows no terms; none found (U) | about 50 a day | V | Company filings and the monthly-revenue oracle the L4 station wants. Wave 2: JSON-diff connector; read the terms first |
| 19 | Central Weather Administration `https://www.cwa.gov.tw/rss/forecast/36_08.xml` (forecast) and `.../Data/cwa_warning.xml` (warnings) | RSS, 1-2 items, tiny | Terms not read (U) | alerts only | V for warning feed | Typhoon and earthquake warnings, an oracle for disaster events. Low volume, skip until a use appears |
| 20 | Judicial Yuan | no feed found: `judicial.gov.tw/tw/rss.html` is a page pointing at `opendata.judicial.gov.tw` which needs JavaScript; `data.judicial.gov.tw` returned 403 | n/a | n/a | U | The courts are on the owner's list. A news-page connector or the judgments open-data service (registration and a nightly window, unverified) is needed; park until the connector work |
| 21 | Ministry of National Defense daily PLA activity (HTML only; `mnd.gov.tw` 200) | HTML list | n/a | daily | U (no feed) | A standing oracle for cross-strait events (2026-10-13 report recommended an HTML-list connector). Wave 3 |

Not verified at all: MAC, MOEA, NDC, FSC feeds (hosts blocked or feed URL
not found: `mac.gov.tw` and `moea.gov.tw` 403; FSC publishes a `rss.jsp`
page whose feed links I did not find).

### International primaries relevant to vertical zero

| # | Source and feed URL | Format | Licence or terms finding | Volume | Status | Why it fills a gap |
|---|---|---|---|---|---|---|
| 2 | White House news `https://www.whitehouse.gov/news/feed/` (also `/presidential-actions/feed/`) | RSS 2.0, 30 items, 0.6 MB | Verified on the site: "government-produced materials appearing on this site are not copyright protected"; third-party content CC BY 3.0 | about 2 a day | V (newest 2026-10-20) | The origin for six us_politics outlets. Executive orders and fact sheets. Wave 1 |
| 3 | SEC press releases `https://www.sec.gov/news/pressreleases.rss`; later EDGAR 8-K Atom `https://www.sec.gov/cgi-bin/browse-edgar?action=getcurrent&type=8-K&output=atom` | RSS 2.0, 25 items; Atom, 40 items | Verified: "All Government-created content on sec.gov and EDGAR public filing content are free to access and reuse"; scripted access allowed, 10 requests a second, declare a user agent with an admin contact | press about 0.5 a day; 8-K hundreds a day | V | Enforcement and rule releases at source; the 8-K feed is how company announcements originate. Wave 1 for press; hold 8-K (volume) until Jev filtering exists. Set a contact in `user_agent` |
| 4 | DOJ news `https://www.justice.gov/news/rss?m=1` | RSS 2.0, 25 items | US federal government work, public domain by statute (17 USC 105; not re-read on the site) | about 6 a day | V (newest 2026-10-19) | Indictments, settlements and antitrust at source. Wave 1 |
| 6 | Federal Register significant rules `https://www.federalregister.gov/api/v1/documents.rss?conditions%5Btype%5D%5B%5D=RULE&conditions%5Bsignificant%5D=1`; BIS export controls `...conditions%5Bagencies%5D%5B%5D=industry-and-security-bureau` | RSS, 12 and 5 items | US federal, public domain by statute (17 USC 105) | about 0.5 a day each | V | Export-control rules on chips are the US side of Taiwan's key story. Wave 1 |
| 7 | gov.uk, Foreign Office and Science and Technology departments `https://www.gov.uk/government/organisations/foreign-commonwealth-development-office.atom`, `.../department-for-science-innovation-and-technology.atom`; all news `https://www.gov.uk/search/news-and-communications.atom` | Atom, 20 items each | Verified: GOV.UK footer says "All content is available under the Open Government Licence v3.0, except where otherwise stated" | FCDO about 6 a day; all-news about 40 a day | V | UK statements on sanctions, travel advice, AI policy. Use the departmental feeds, not the all-news one, to keep volume down. Wave 1 |
| 10 | Kremlin English `http://en.kremlin.ru/events/president/news/feed` | Atom, 20 items, 0.1 MB | Verified on the site footer: Creative Commons Attribution 4.0 International | about 2 a day | V (newest 2026-10-19) | The Russian head of state's own readouts. A state claim-maker, tag `state_media`. Wave 1 |
| 15 | CISA advisories `https://www.cisa.gov/cybersecurity-advisories/all.xml` (KEV JSON `.../known_exploited_vulnerabilities.json`) | RSS, 30 items, 1.2 MB; KEV JSON 1.8 MB | US federal government, public domain by statute (not re-read on the site) | about 2 a day | V | Origin for the security stories in tech_ai. KEV is also an L4 oracle. Advisory feed over the cap; wave 2 |
| 16 | Council of the EU press releases `https://www.consilium.europa.eu/en/rss/pressreleases.ashx` | RSS, 20 items | EU institutional content is CC BY 4.0 per the Commission legal notice (the Council's own page not read) | about 3 a day (the sample's dates did not parse) | V in format; newest not read | Sanctions and trade decisions at source. The EC Press Corner row is already registered and unscheduled; schedule it too. Wave 1 |
| 22 | Macro statistics: BEA `https://apps.bea.gov/rss/rss.xml`, Census economic indicators `https://www.census.gov/economic-indicators/indicator.xml`, EIA `https://www.eia.gov/rss/todayinenergy.xml`, BLS `https://www.bls.gov/feed/bls_latest.rss` | RSS | US federal, public domain by statute | each under 1 a day | V | The paper's business section has no statistics office. Wave 1, hourly or slower |
| 23 | Corporate originators: Google blog `https://blog.google/rss/`, NVIDIA `https://blogs.nvidia.com/feed/`, Microsoft `https://blogs.microsoft.com/feed/` | RSS | All rights reserved (U, not read); excerpt-only | 1 to 4 a day each | V | OpenAI is the only company in the 28. Claim-makers about their own products. Wave 1 with `excerpt_only` |
| 24 | ECDC `https://www.ecdc.europa.eu/en/taxonomy/term/1310/feed` | RSS, 10 items | ECDC reuse policy not read (U) | under 1 a day | V | European public-health notices at source |
| 25 | WHO news `https://www.who.int/rss-feeds/news-english.xml` | RSS, 126 items, 1 MB, unsorted | CC BY-NC-SA 3.0 IGO for publications; "Permission is required for commercial uses". Verified on the WHO copyright page. News-item terms not separately found | under 1 a day | V in format (newest 2026-10-10); the 2026-10-15 problem doc found this URL stale (newest 2026-02-25) and removed it from the registry; it looks live now, re-check once more | WHO Disease Outbreak News was cited on the front page and is the primary the Ebola story used. Wave 3: a commercial-use ruling (links and facts versus stored text) |
| 26 | UN News `https://news.un.org/feed/subscribe/en/news/all/rss.xml` | RSS, 30 items | The UN terms page says downloads are "for the User's personal, non-commercial use, without any right to resell or redistribute", and no derivative works. Verified on the page | about 6 a day | V | Three `official:un*` rows already sit in the registry. Wave 3: do not schedule until counsel or the owner rules; this contradicts ruling 3 |

### Originating wires and news agencies

| # | Source and feed URL | Format | Licence or terms finding | Volume | Status | Why it fills a gap |
|---|---|---|---|---|---|---|
| 27 | ANSA English `https://www.ansa.it/english/news/english_nr_rss.xml` | RSS, 100 items, 0.2 MB | Terms not read (U) | about 30 a day | V | A national agency (Italy): first copy of European stories. Wave 3 until terms read |
| 28 | Yonhap English `https://en.yna.co.kr/RSS/news.xml` | RSS, 98 items | Terms page blocked to my fetcher (U); agency content is generally licensed for redistribution | about 90 a day (high) | V | East Asia's originating wire (Korea, DPRK, Japan, China, Taiwan). The most direct answer to "wire" for the first vertical. Wave 3: read terms; set `max_items_per_poll` |
| 29 | Anadolu Agency English `https://www.aa.com.tr/en/rss/default?cat=world` | RSS, 30 items | Terms not read (U); Turkish state agency | about 30 a day | V | Middle East and Turkey; state claim-maker. Wave 3 |
| 30 | PR Newswire `https://www.prnewswire.com/rss/news-releases-list.rss`, tech feed `.../technology-latest-news-list.rss` | RSS, 20 items each | Feed terms not found (the RSS page links a Terms of Use I could not read) (U) | hundreds a day | V | Where corporate statements originate. Wave 3: too high-volume for now; the 8-K feed has the same shape |
| 31 | TASS English `https://tass.com/rss/v2.xml` | RSS, 100 items | Terms not read (U) | about 150 a day | V | Russian state agency. A claim-maker; the Kremlin feed (rank 10) covers the same voice at far lower volume. Not recommended now |

Not workable without a licence or at all: Reuters (`reutersagency.com`
feed 404), Associated Press (feed URL returns the HTML homepage), AFP (no
public feed found), Kyodo and Jiji (both timed out, U), Xinhua English
(feeds frozen at 2018, V), State Department and FCC (403, U), MOFA Japan
and METI (403), Bloomberg (none), EFE (403). These are licensed products;
the free-tier ruling excludes buying them.

### Taiwan and Chinese-language outlets (outlets, not primaries)

The 2026-10-13 reports already carry the vetted list. Re-verified today
(V, newest 2026-10-20): Focus Taiwan (CNA English, 21 a day), Taipei
Times (52 titles), PTS (Atom, 31 a day), RTI (143 a day, a state-funded
broadcaster), Taiwan FactCheck Center (0.5 a day), MyGoPen (Atom, full
text), TechNews (zh, 57 a day), DigiTimes English (43 a day), iThome,
Liberty Times politics (231 a day), BBC Chinese (Traditional), DW Chinese,
RFA Mandarin. Licence cautions already on record: CNA RSS is
personal-and-non-profit only and UDN bars AI and commercial use, so both
stay out under ruling 3; Newtalk declares `ai-input=no`, so excerpt-only
at most. Focus Taiwan comes from CNA, so it is out too. Rank them after
the primaries: Taiwan needs at least two independent Taiwanese outlets
for the two-outlet rule, so PTS, Taipei Times (title only), TechNews,
DigiTimes and Liberty Times are the likely first set, each licence to be
read.

## 6. Existing scheduled sources: licence flags

- BBC (3 rows) and Guardian (3 rows): the BBC's RSS page was not
  fetchable here, and the 2026-10-13 sourcing report lists BBC RSS terms
  as unverified. From memory (not re-read) the BBC's feed terms are
  personal and non-commercial; if so, three of the 28 sit against ruling
  3. Read before expanding the BBC set.
- arXiv, HN and Lobsters link to third-party work; their capture is a
  pointer, so they count only as discovery signals, not outlets.
- The `official:un*` and (if re-added) WHO rows carry non-commercial terms,
  as above.

## 7. Recommended order of work

1. Owner or an agent reads the live schedule list and confirms the 28
   (one GET).
2. Wave 1, registry-only, all verified, about 15 rows: EY (zh and en),
   White House, SEC press, DOJ, Federal Register (significant rules and
   BIS), gov.uk FCDO and DSIT, Taiwan CDC, Kremlin English, Council of the
   EU, BEA, Census, EIA, BLS, Google, NVIDIA, Microsoft, plus scheduling
   the registered Fed, ECB, EC and WTO `official:` rows. Set `official:
   true`, `source_standing`, `poll_interval_seconds` 1800 to 3600,
   `max_items_per_poll` 25 to 30, `store_body_policy` `excerpt_only`
   unless the licence is open. Expected total: roughly 25 to 60
   captures a day.
3. One orange change: a per-source `max_body_bytes` (default 2 MiB) so
   the CBC, MOF and MOFA feeds parse; then rows 5, 11, 13 and 15.
4. A JSON/dataset connector (LYAPI, TWSE, CEC CSV, KEV), sized for the 28
   November election. This is the same connector the 2026-10-13 report
   named; it is the one real code item.
5. Wave 3 decisions for the owner: commercial reading of WHO and UN
   terms, Yonhap and ANSA terms, and the BBC question above.
6. Add a freshness gate to the registry (see the 2026-10-15 problem doc and
   the 2026-10-13 report §10.3), because five registry rows were already
   dead or mislabelled when audited, and Taiwan's `cdc.gov.tw` feed's
   newest item is already a week old.

## 8. What would change these rankings

- If the paper's egress is blocked where mine was not (or the reverse),
  the V marks change. Retest from the guest.
- The inferred six of the 28 may be wrong; the gap findings do not depend
  on which six they are, only on the tally of three types.
- Licence readings above are secondary reports of the agency text in some
  cases (OGDL via CBC and a web search of agency notices). The authoritative
  OGDL text on data.gov.tw is script-rendered and I could not read it
  directly. This is a working document, not legal advice.
