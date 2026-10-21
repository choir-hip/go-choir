# Primary-source registry, wave 1: what it adds, why, and the licence of each row (2026-10-20)

Status: wave 1 registry rows and the per-source byte cap are committed on
branch `worktree-agent-a2f29a88561ca00fd` (not pushed, not deployed). No schedule is activated; activation is an operator act on
the paper (section 6). Working document, not doctrine; the licence readings
are engineering readings, not legal advice.

Mutation class: this note is green. The registry rows are yellow while
unscheduled (an unscheduled row is inert), becoming orange at activation.
The `max_body_bytes` cap is orange (`internal/sources/rss.go`, runtime
behaviour of the RSS poller).

## 1. The problem

The paper's 28 scheduled sources are 19 outlets, 7 link aggregators and 2
primaries, all English, 26 of 28 US or UK. 4 of 113 cited sources on the
front page were primary records (grounding report, 2026-10-19). Source of
the numbers: [source coverage-gap
analysis](../reports/source-coverage-gap-analysis-2026-10-20.md) (sections
1 to 4).

Owner rulings this serves (owner notes 2026-10-14 §27 items 6 and 11): run
the coverage-gap analysis first (done), then "start the primary-source
registry, licences checked first". Standing rulings it respects: Autopaper
is commercial, so non-commercial licences stay out; Taiwan is the first
vertical; scale ingestion without more model calls (a row adds captures,
not model calls; wave 1 is about 17 captures a day against about 270 an
hour of current intake); track outlets separately from claim-makers.

Two gaps in the code, found while reading it:

1. **No way to tell a claim-maker from an outlet.** `Source` has `official`
   and `source_standing`; nothing wrote `source_standing`, and only
   `official` is read (`internal/agentcore/event_wakes.go`,
   `officialSources`: a single-origin event opened by an `official` source
   wakes management once, after 45 minutes, so it can ask for a second
   source). Wave 1 sets both on every new row. `source_standing` values
   used: `primary_government`, `state_media` (the Kremlin: a claim-maker
   about its own state, not a neutral outlet). Existing `official:` rows
   (Fed, ECB, EC, NASA, CDC US, NOAA, USGS, WTO, UN) are not edited.
2. **A 2 MiB read cap on every feed.** `internal/sources/rss.go` reads
   `io.LimitReader(resp.Body, 2<<20)`. The Central Bank (CBC) feed is 3.4
   MB (re-measured 2026-10-20: 3,430,572 bytes, 483 items). A feed cut at
   2 MiB fails to parse ("XML syntax error ... unexpected EOF", confirmed by
   a test with a 3.5 MB feed), so the source polls as an error every time. Wave 1
   makes the cap a per-source setting `max_body_bytes`: default 2 MiB
   unchanged, hard ceiling 8 MiB, a value above the ceiling is clamped to
   it. Only the CBC row sets it (to 4 MiB; 8 MiB is a ceiling, not a target).

## 2. Rows added (11) and the licence of each

Each row carries `official: true`, a `source_standing`, and a new `license`
field (machine-readable, so the guard test can refuse a row whose licence
is not on the commercial-use list). "Read" is what I read on 2026-10-20;
"statute" means US federal government work, public domain under 17 USC 105,
as the coverage-gap analysis recorded. Every feed was fetched once on
2026-10-20 with the registry's user agent from a developer machine (HTTP
200, parsed as RSS or Atom, newest item within two weeks); that is not the
paper's egress.

| Row id | Feed | Licence (value of `license`) | What I read | Expected a day |
|---|---|---|---|---|
| `official:whitehouse:news` | whitehouse.gov/news/feed/ (30 items, 0.6 MB) | `us-federal-public-domain` | Analysis: site says government-produced materials "are not copyright protected"; third-party content CC BY 3.0 (not re-read this pass) | 2 |
| `official:sec:press` | sec.gov/news/pressreleases.rss | `us-federal-public-domain` | Analysis: "All Government-created content on sec.gov and EDGAR public filing content are free to access and reuse"; scripted access allowed at 10 requests a second with a declared user agent | 0.5 |
| `official:doj:news` | justice.gov/news/rss?m=1 | `us-federal-public-domain` | Read 2026-10-20 (justice.gov/legalpolicies): "Unless otherwise indicated, information on Department of Justice websites is in the public domain"; photos and graphics credited to another source need that source's permission | 3 to 6 |
| `official:federalregister:significant_rules` | federalregister.gov/api/v1/documents.rss, type RULE, significant | `us-federal-public-domain` | Statute. The HTML pages are CAPTCHA-gated to automated access ("programmatic access ... is limited to ... developer APIs"); the `/api/v1` feed is the API and answered 200 | 0.5 |
| `official:federalregister:bis` | same, agency industry-and-security-bureau | `us-federal-public-domain` | As above; the export-control rules on chips | 0.2 |
| `official:uk_fcdo:news` | gov.uk FCDO Atom | `ogl-v3.0` | Analysis: GOV.UK footer "All content is available under the Open Government Licence v3.0, except where otherwise stated" | 1.5 |
| `official:uk_dsit:news` | gov.uk DSIT Atom | `ogl-v3.0` | As above | 0.5 |
| `official:tw_cbc:press` | cbc.gov.tw/tw/rss-302-1.xml (zh-TW, 3.4 MB) | `ogdl-tw-v1.0` | Analysis: the CBC statement grants Open Government Data License v1.0, free, non-exclusive, sublicensable, attribution required, copyright only (logos excluded) | 0.6 |
| `official:kremlin:en_news` | en.kremlin.ru/events/president/news/feed | `cc-by-4.0` | Analysis: site footer, Creative Commons Attribution 4.0 International. Standing `state_media` | 3 |
| `official:cisa:advisories` | cisa.gov/cybersecurity-advisories/all.xml (1.2 MB) | `us-federal-public-domain` | Statute (the CISA policy page was not found at the URLs I tried). The analysis marked this feed "over the cap"; it is 1,169,779 bytes, under 2 MiB, so it needs no cap | 1 to 2 |
| `official:eia:today_in_energy` | eia.gov/rss/todayinenergy.xml | `us-federal-public-domain` | Read 2026-10-20 (eia.gov/about/copyrights_reuse.php): "U.S. government publications are in the public domain and are not subject to copyright protection" | 0.4 |

Total expected: about 14 to 18 new captures a day when all eleven are
scheduled. That is the observed items-per-day of each feed over its own
dates; a feed that lists edits can overstate.

Attribution: OGL v3.0, OGDL v1.0 and CC BY 4.0 require attribution. A row's
`name` carries the publisher's name; the paper's citation of the source
supplies the attribution. Whether a published story's source line is
enough for OGDL and CC BY is a product question, not settled here.

## 3. Rows deliberately not added

| Source | Reason |
|---|---|
| Executive Yuan press | English footer "All Rights Reserved"; the open-data declaration was not found. Stays out until its licence is read |
| Taiwan CDC | Footer links a government open-data declaration that was not read; newest item a week old |
| MOFA (zh), Ministry of Finance, Presidential Office, DGBAS, Central Weather Administration | MOFA's OGDL rests on a dataset page cited, not re-read; MOF, Presidential Office, DGBAS and CWA terms not read; DGBAS also fails TLS chain verification. Wave 2 once each is read |
| UN News (and the three `official:un*` rows already in the registry), WHO | UN News terms: personal, non-commercial use, no derivative works. WHO news: CC BY-NC-SA 3.0 IGO, "Permission is required for commercial uses". Both contradict the commercial ruling; owner or counsel to rule. The existing `official:un*` rows are untouched and unscheduled |
| Council of the EU | EU institutional content is CC BY 4.0 per the Commission legal notice, but the Council's own page was not read |
| BEA, Census economic indicators, BLS latest numbers | US federal, public domain, but unfit as built: BEA's feed is a latest-release-per-indicator list of 48 items, not date ordered (pubDates run back to 2018), so a first poll would capture years-old releases as new; Census dates carry a double space ("19 Oct 2026  09:00:00 EST") that `parseFeedTime` rejects, so the published time falls back to fetch time; BLS is one dashboard item with a stable link and changing numbers, which mints a capture per change and not per release (the shape of the HN counter problem). Each needs a parser or connector decision first |
| Corporate blogs (Google, NVIDIA, Microsoft), ECDC, ANSA, Yonhap, Anadolu, PR Newswire, TASS | Licence unread or all-rights-reserved; not in a commercial-safe set |
| Reuters, AP, AFP, Kyodo, Jiji, Xinhua, Bloomberg, EFE | Licensed products or no live feed |
| Legislative Yuan API, TWSE, CEC election CSV, CISA KEV JSON | No JSON or dataset connector exists; wave 2 |

## 4. Known weak points of the included rows (not fixed)

- **Published time falls back to fetch time** where the feed has no `pubDate`
  on an RSS 2.0 item: DOJ and CISA (`dc:date`) are in this shape.
  `parseRSSLikeFeed` reads `dc:date` for RSS 1.0 only. Item identity is the
  link or guid, so there is no duplicate-capture risk; only the displayed
  time is wrong.
- **`official: true` wakes management** (once, after 45 minutes) for any
  single-origin event a row opens, when `intake.event_wakes` is on for the
  computer. DOJ at 3 to 6 items a day is the largest source of such wakes.
  That is the intended behaviour for a claim-maker, and the volume is
  bounded by the feed, but watch management's wake load for the first day.
- **Reader import.** `store_body_policy: bounded_release_text` makes the
  poller fetch each item's article page. Not used on the Federal Register
  rows (HTML is CAPTCHA-gated); those rows are `excerpt_only`.
- **SEC user agent.** SEC asks for a contact in the user agent. The rows use
  the registry user agent (`ChoirUniversalWire/0.1 (+https://choir.news)`),
  which answered 200. Setting a contact address is an operator decision
  (the per-source `user_agent` field); I did not invent one.
- The `license` field is new and read by nothing but the guard test.

## 5. What the change is

1. This note (green).
2. `max_body_bytes` on `Source`, honoured by the RSS poller. Failure modes
   written first as tests: a feed over the default cap and no setting
   (behaviour unchanged, truncated); a 3 MB feed with a raised cap (all
   items parse); a setting above the ceiling (clamped to 8 MiB; a body
   over the ceiling is cut there, so memory is bounded); zero and negative
   values (the default); the CBC registry row (cap set, within the ceiling).
3. The eleven rows, with a guard test: every `official:` wave-1 row has
   `official`, an allowed `source_standing`, an allowed `license`, a valid
   URL, a unique id and URL, and `max_body_bytes` within the ceiling; no
   non-commercial host (UN, WHO) is in the set.

## 6. Activation procedure (operator act on the paper; not done)

Schedules are per owner and computer; the only surface is the owner API on
the paper (no `choir` CLI verb exists for it).

0. **Prerequisite: deploy.** The rows live in the embedded registry
   (`internal/sources/registry.json`), so the paper computer needs a build
   that contains this change (Landing Loop: push, CI, staging deploy,
   confirm the deployed commit). Before that, `POST` answers `unknown
   source`. The CBC row also needs the cap code on the guest; before the
   deploy it would fail to parse.
1. **List first.** `GET /api/autopaper/sources/schedules` with the paper
   credential. Confirm the existing 28 are `active` and none has
   `overdue: true`. Do not add sources to a paper that is not healthy
   (paper healthy = health ok and schedules not overdue and inference in
   the last 30 minutes; docs/runbooks/node-b-host-reboot.md).
2. **Test the guest's TLS trust for the Taiwan host before scheduling it.**
   `cbc.gov.tw` passed curl on macOS and failed Python's bundled trust
   store in the analysis run, so the chain may be incomplete. Run one
   `choir.PollSource` for `official:tw_cbc:press` from research on the paper
   computer (a single poll from the guest, no standing schedule; it returns
   a report with `status`, `status_code`, `fetched` and `error`). Read the
   error: `x509: certificate signed by unknown authority` means the guest
   trust store lacks the intermediate, and the row must not be scheduled
   (a schedule on it would fail every poll, backing off to 24 hours, and
   the row would sit `active` and silent). A passing poll must report
   `fetched` above zero and no error. The same test applies to any future
   `.gov.tw` source; DGBAS (`stat.gov.tw`) is known to fail a chain check.
3. **Start the US and UK rows**, staggered 15 s apart as in the 2026-10-15
   recovery. For each of the ten non-Taiwan rows:

   ```text
   POST /api/autopaper/sources/schedules
   {"source_id": "<row id>", "interval_minutes": 60, "max_items": 25}
   ```

   Suggested intervals: 30 for `official:doj:news`,
   `official:whitehouse:news`, `official:kremlin:en_news`,
   `official:uk_fcdo:news`; 60 for the rest. `max_items` 25 (the poll
   applies `min(max_items, the row's max_items_per_poll)` to new captures
   after the repeat check). Expected: about 14 to 18 new captures a day in
   total once the first poll has captured each feed's current items (up to
   25 per feed on the first poll, so a burst of about 100 to 150 in the
   first hour, then the daily rate).
4. **Start the Taiwan row** only if step 2 passed:
   `{"source_id": "official:tw_cbc:press", "interval_minutes": 60,
   "max_items": 25}`; about 0.6 a day, 25 on the first poll. The feed is
   Traditional Chinese; the first event-layer pass on it is the first
   Chinese-language primary the paper ingests, so read the first captures.
5. **Verify.** After one interval, `GET /api/autopaper/sources/schedules`:
   each new row `active`, `consecutive_failures` 0, `overdue` false, next
   due set. Confirm captures by source id on the paper's capture list.
6. **Back out.** `DELETE /api/autopaper/sources/schedules?source_id=<id>&reason=<why>`
   stops one; the row stays in the registry, inert.

## 7. Residual risks

- The V marks come from a developer machine, not the paper's egress. Some
  hosts (SEC, Federal Register) restrict automated clients; a 403 from the
  paper means the row is dead there.
- Licence readings of the analysis (White House, SEC, gov.uk, CBC, Kremlin)
  were not re-read this pass; DOJ and EIA were. The OGDL text on
  data.gov.tw could not be read directly.
- Third-party material inside a government page (photos, quoted text) can
  carry its own rights; the feeds are used for text and links.
- The Kremlin row is a state's own account; `state_media` is recorded, but
  nothing yet reads `source_standing`. Until something does, the two-outlet
  rule counts it like an outlet. The statistic split (outlets versus
  claim-makers) is not built.
- Nothing yet checks a registry row's freshness (analysis section 7 item
  6); a feed that goes stale stays `active`.

## 8. Activation, 2026-10-21 00:35 UTC (owner, notes section 32: "schedule wave-1")

- Paper healthy first: 28 schedules active, none overdue (one with
  failures, a pre-existing row), no liveness-probe alerts in the last hour.
  Read with the paper credential (states and timings only).
- Started 10 rows, 15 s apart, each `max_items` 25. At 30 minutes:
  `official:whitehouse:news`, `official:doj:news`,
  `official:kremlin:en_news`, `official:uk_fcdo:news`. At 60 minutes:
  `official:sec:press`, `official:federalregister:significant_rules`,
  `official:federalregister:bis`, `official:uk_dsit:news`,
  `official:cisa:advisories`, `official:eia:today_in_energy`. Every POST
  returned 200, and the list then showed 38 active and none overdue.
- Not started: `official:tw_cbc:press`. Its guest TLS test (step 2, one
  `choir.PollSource` from research on the paper) has not run.
- To verify after one interval: each new row has `consecutive_failures` 0,
  has a next due time, and shows captures by source id.
