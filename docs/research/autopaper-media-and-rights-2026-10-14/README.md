# Autopaper Media and Rights: Images, Video, Audio, and What We May Use

Date: 2026-10-14. Mutation class: **green** (research docs only).
Status: research, not actionable yet (owner: "this is not actionable").
Not legal advice; §7 lists what to ask a lawyer.

Supporting files in this directory:

- [rights-by-jurisdiction.md](rights-by-jurisdiction.md): the law, with
  case and statute citations (US, EU, UK, Japan, others; signals such as
  robots.txt and TDMRep).
- [media-sources-and-terms.md](media-sources-and-terms.md): where images,
  video, audio and transcripts can come from, and each source's terms.
- [repo-media-survey.md](repo-media-survey.md): what Choir already has
  (file paths), what is half-built, what is missing.

## The brief, in the owner's words (2026-10-14)

- "How can we find images that we can use that have written licenses and
  that are actually illustrative of the content?" Not a fan of stock or AI
  images, "but both of these have their place if done right." Images must
  add value, "not merely just visual pop."
- Research "the distinction between information that we consume to learn
  from and sources that we work from and cite", across jurisdictions
  (owner in Massachusetts; Choir is a Delaware C corp; sources worldwide).
- Images on the Autopaper front page, in Texture articles, and expanding
  and collapsing like citations; "if the image is actually valuable, it
  might be expanded automatically."
- Widened: YouTube and podcast embeds, and later YouTube and podcast
  transcription as sources ("one of my core priorities").
- Framing: "if a New York Times journalist were to transcribe podcasts and
  YouTube videos to analyze and then quote them in an article, that would
  be fine. So I don't see why what I'm doing would be any different.
  [...] there is quite a big difference between doing reporting and just
  willy-nilly copying material."
- Intent: if Autopaper becomes a large business, share gains with the
  organisations that provide sources. Not now.

## Bottom line

1. **The owner's position holds.** Transcribing or reading a source to
   analyse it, then quoting the newsworthy part with attribution and a
   link, is reporting. US law names news reporting as a fair-use purpose,
   and *Swatch v. Bloomberg* found publishing a whole earnings-call
   recording and transcript was fair use for news. No court treats
   AI-assisted reporting as a different category.
2. **The edges are substitution, wholesale reproduction, how we got the
   source, and contract.** Courts in the AI cases separate learning
   (usually defensible if the copy was lawfully obtained) from output that
   substitutes for the source (where liability lives: *Meltwater*,
   *Cohere*, *NYT v. OpenAI*). Paywalled or pirated inputs are a separate
   problem (*Bartz*). Site terms (YouTube) bind regardless of fair use.
3. **The EU is stricter on output to news sites.** The press publishers'
   right (DSM Art. 15) covers more than "very short extracts" of EU press
   for two years after publication. A pending CJEU case (*Like Company v.
   Google*, C-250/25) will say how it applies to AI summaries.
4. **One AI Act duty lands on Autopaper now.** From 2 Aug 2026, AI-written
   text on matters of public interest must be labelled, *unless* it had
   human review or editorial control and someone holds editorial
   responsibility (Art. 50(4)). AI images that look real must always be
   labelled.
5. **The og:image of the cited article is not ours.** Using a news photo
   to illustrate news is the purpose it is licensed for (*Warhol*), and
   hot-linking it counts as displaying it outside the Ninth Circuit. Link
   to it; do not show it.
6. **The best images are ones we draw from the story's own cited data**:
   locator maps from OpenStreetMap, charts of cited numbers, timelines.
   They add information, their provenance is complete, and the risk is low.
7. **Second best: free-licensed or public-domain images of the actual
   subject** (Wikimedia Commons via Wikidata, US federal works, Library of
   Congress, Europeana, Flickr Commons), with licence and credit recorded
   at retrieval. Third: official embeds (YouTube, Getty viewer). Stock and
   AI illustration only in narrow, labelled cases.
8. **No image is the default.** An image appears only when a Jev judgement
   says it adds information the text does not, and a licence check has
   recorded why we may show it.
9. **Choir has the parts, not the block.** Citations already render
   YouTube, image, video and audio in editor popovers, and transcript and
   timestamp selectors exist in the source contract. But Texture has no
   media block. The published Autopaper renderer never shows media.
   Captures and the RSS parser throw image and enclosure URLs away.
   Nothing evaluates robots.txt.
10. **Transcripts become ordinary captures with timed blocks.** The quote
    checker already verifies a quote inside one block. A timed block plus
    the existing `timestamp_range` selector gives a citation that plays
    from the quoted moment. The open issue is YouTube's terms, which
    forbid the automated caption fetching Choir already does.

## 1. Rights: reporting vs copying, learning vs working from

### 1.1 The two lines

| | learning from (consume, analyse) | working from (publish, cite) |
|---|---|---|
| what we do | poll feeds, fetch pages, store capture text, put it in a model's context, transcribe audio | quote, summarise, transclude, show images, embed players |
| legal question | is the *copy* lawful (lawful access, TDM exception, opt-outs, ToS)? | does the *output* carry protected expression, and does it substitute for the source? |
| US | fair use; *Authors Guild v. Google*, *Bartz*, *Kadrey*: analysis copies usually fair if lawfully acquired | fair use for news reporting (*Swatch*, *Núñez*) vs substitution (*Harper & Row*, *Meltwater*, *Cohere*) |
| EU | DSM Art. 4 TDM exception, **unless reserved by machine-readable opt-out** | quotation and current-events exceptions; press publishers' right (Art. 15) beyond very short extracts |
| UK | TDM only non-commercial (s29A) | fair dealing for quotation and reporting current events (s30), not for photos in news reporting |
| Japan | Art. 30-4 broad, unless an "enjoyment" purpose coexists | quotation (Art. 32), current events (Art. 41); publishers suing Perplexity over output |

Citations for every cell are in
[rights-by-jurisdiction.md](rights-by-jurisdiction.md).

**What we must cite even when we only learn.** No law requires a public
citation for material that was only learned from. But three things argue
for recording it: (a) the EU AI Act's direction (providers must publish
training-data summaries; deployers will be asked about inputs); (b) the
DMCA forbids stripping author and licence information we hold (17 USC
1202); (c) honesty: if a story's framing came from a source, the source
should appear. Rule: every capture a research desk *read* for a story is
in its provenance; every capture the story *relies on* is cited.

### 1.2 Verdict on the owner's position

| owner's claim | holds? | why / where the edge is |
|---|---|---|
| A journalist may transcribe podcasts and videos to analyse them and quote them; so may we | **Yes** | *Swatch* (full transcript of a recorded call, published as news, fair use); 17 USC 107 names news reporting; First Amendment protection works *through* fair use (*Eldred*, *Golan*). No AI-specific carve-out exists. |
| If someone says something newsworthy on a podcast, we can cover it | **Yes** | Facts and newsworthiness are free; quote the newsworthy lines, link and timestamp them. |
| Reporting differs from willy-nilly copying | **Yes, and that is the legal line** | Substitution kills fair use: *Harper & Row* (the "heart", ahead of publication), *Meltwater* (automated excerpts as the product), *Cohere* (summaries that are close paraphrase). |
| Given that models are built on wide scraping, it would be odd to be stricter | **Partly** | Learning from lawfully accessed public material is broadly defensible in the US, EU (absent opt-out), Japan and Singapore. It is not defensible from paywall circumvention or pirate copies (*Bartz*), and not in the UK for commercial TDM. Opt-outs bind us in the EU. |
| We need not share gains now | **Yes** | No law requires it. Record it as a future item (§1.6). |

**Where the edges are, concretely, for Autopaper:**

1. **Substitution by one source.** A story built from one article that
   tells the reader everything that article said is what *Meltwater* and
   *Cohere* look like. The just-landed "no one-source lead" ranking rule
   (commit 1f84fa908) helps for product reasons; it also helps here.
   Multi-source synthesis plus Choir's own framing is the defensible shape.
2. **Wholesale reproduction.** Full transcripts of commercial shows, long
   transclusions of third-party articles (`{{embed}}` renders pinned
   blocks as blockquotes on the public page), and reproduced photos.
3. **Paywalled text.** Lawful access is a condition in the EU (Art. 4),
   Singapore (s.244) and in practice the US (*Bartz*). Do not capture behind
   paywalls we have not paid for.
4. **EU press publishers' right.** For EU press less than two years old,
   keep quotes to "very short extracts" (one sentence, with a link) unless
   the quote is itself the news.
5. **Contract, not copyright.** YouTube's terms forbid downloading and
   automated access; Choir's InnerTube caption fallback is a terms risk
   even where the copyright analysis is clean.

### 1.3 Autopaper's pipeline on the risk spectrum

| step (today) | class | risk | why | what would lower it |
|---|---|---|---|---|
| Poll RSS/Atom feeds | learning | low | feeds are published for syndication | honour robots.txt and per-source terms (labels exist, nothing evaluates them) |
| Fetch article page, store full text on host disk (`CaptureBodyStore`) | learning | low-moderate | US: analysis copy; EU: Art. 4 if not reserved; UK: no commercial TDM exception | evaluate robots.txt, TDMRep and aipref at fetch; record the result on the capture; skip reserved EU sources for body storage or keep metadata only; enforce `retention_days` |
| Research desk reads captures in model context | learning | low | same copy, not published | none beyond the above |
| Transcribe YouTube via InnerTube | learning | **moderate (contract)** | YouTube terms forbid automated access and downloading | prefer publisher-supplied audio (podcast RSS) or channel-published captions at human volume; flag provenance `tos_risk` |
| Quote a sentence with link, quote verified | working from | low | quotation for news reporting; short | keep quotes short for EU press; always link |
| Summarise / write the story | working from | low if multi-source; moderate if one source | facts are free; close paraphrase of one source is the risk (*Cohere*) | multi-source rule; Jev check for close paraphrase of a single capture |
| Public transclusion of capture blocks (`{{embed}}`) | working from | moderate for long press text | reproduces expression on a public page | cap transclusion length for third-party press; full text only for public-domain or licensed sources |
| Show the source's og:image (not done today) | working from | **high** | same purpose as its licence (*Warhol*); display outside Ninth Circuit | do not; link instead |
| AI-written stories on public-interest topics in the EU | disclosure | compliance | AI Act Art. 50(4) from 2 Aug 2026 | name an editorially responsible person and a review step, or label stories as AI-written |

### 1.4 What to record on every capture and media object

| field | why |
|---|---|
| fetch time, URL, final URL, HTTP status, content hash | provenance (exists) |
| robots.txt verdict for our user agent and for named AI agents; TDMRep `tdm-reservation` and policy URL; aipref signals when standardised | EU Art. 4 opt-out evidence; shows good faith |
| paywall / login state at fetch | lawful-access evidence |
| source `tos_class` (exists as label) | contract risk |
| author, publisher, copyright line, licence if stated | DMCA 1202; attribution |
| for media: licence id and URL, creator, credit line, licence evidence snapshot, retrieval method, modifications | written-licence proof that travels with the image |
| jurisdiction of publisher | which regime applies (press right, UK, etc.) |

### 1.5 Keep an eye on (not alarmism; dates where known)

| item | why it matters to Autopaper | when |
|---|---|---|
| *Like Company v. Google* (CJEU C-250/25) | whether AI summaries of EU press engage Art. 15 and reproduction | judgment pending after March 2026 hearing |
| *NYT v. Microsoft & OpenAI* summary judgment | output substitution for news | briefing to Nov 2026 |
| *Advance Local v. Cohere* summary judgment | RAG summaries of news articles | motions due Dec 2026 |
| EU AI Act Art. 50 practice and code on marking | how "editorial responsibility" is read | applies from Aug 2026; marking grace to Dec 2026 |
| IETF aipref | standard opt-out signal we should read | draft-08, Sept 2026 |
| *Getty v. Stability* appeal; UK policy follow-up | UK position on training and outputs | 2026-27 |
| *Kneschke v. LAION* further appeal | what counts as machine-readable opt-out | pending |
| Nikkei/Asahi/Yomiuri v. Perplexity; *ANI v. OpenAI* trial | Japan and India on answer-engine output | ongoing |
| Massachusetts likeness/voice bills | AI depictions of real people | 194th General Court |

### 1.6 Future item: sharing gains with sources

Owner intent (2026-10-14): if Autopaper becomes a large, prosperous
business, find a way to share gains with the organisations whose work it
draws on. **Not a current obligation.** Precedent shapes to revisit then:
per-source licensing (wire services), revenue pools (the Bartz settlement
priced works at roughly $3,000 each, a ceiling not a template), and
pro-rata pools keyed to citations. The provenance fields in §1.4 are
what would make any future pro-rata scheme computable, which is one more
reason to record them now.

## 2. Getting images that add information

Priority order (details and licence terms in
[media-sources-and-terms.md](media-sources-and-terms.md)):

1. **Generated from the story's own cited data**: OSM locator map
   (ODbL credit), chart of cited figures, timeline of cited dates, simple
   diagram of a cited mechanism. Highest value, lowest risk.
2. **Primary-source media the story is about**, shown through the
   publisher's official embed: the YouTube video of the speech, the
   podcast episode at the quoted minute, the government's own photo.
3. **Free-licensed or public-domain depictions of the subject**: Wikidata
   `image` (P18) for the people, places and organisations a story names,
   resolved to Wikimedia Commons with `extmetadata` licence fields; US
   federal agencies; Library of Congress; Europeana; Flickr Commons.
4. **Paid wire photos** (AP, Reuters, Getty): only when a business case
   exists; or the Getty embedded viewer for editorial use, accepting its
   iframe, ads and possible withdrawal.
5. **Stock** (Unsplash, Pexels): essentially never for news; at most
   section art.
6. **AI illustration**: only for abstract subjects, labelled, never
   photorealistic, never a real identifiable person or a real event. In
   the US such images carry no copyright (*Thaler*; Copyright Office Part
   2), which does not matter for a newspaper.

## 3. A value test for images

### 3.1 Rubric

An image **adds information** when a reader learns something from it that
the text does not give as well. Score one of:

| kind | example | passes? |
|---|---|---|
| **depicts the event** | photo of the flood at the named place on the stated date | yes, if date and place are verified |
| **depicts the subject** | official portrait of the minister named in the lede | yes, as a small inline image; rarely a lead |
| **locates** | locator map of a town nobody has heard of | yes |
| **shows the data** | chart of the cited series | yes, if every plotted value is cited |
| **shows the document** | page of the court ruling, the treaty map | yes |
| **is the primary source** | the speech video at the quoted minute | yes |
| **generic subject** | a stock photo of "a hospital", the party logo, a file photo of a building | **no** |
| **mood or decoration** | AI image of "inflation", a sunset over a city | **no** (exception: a labelled illustration for an abstract explainer, scored low) |

### 3.2 Pipeline

```text
story (published or about to publish)
  -> candidates: cited data (charts/maps), entities (Wikidata P18 -> Commons),
     primary media in cited captures (embeds, enclosures), agency PD archives
  -> licence verification: fetch licence metadata from the origin, not an
     aggregator; snapshot it; classify: pd | cc0 | cc-by | cc-by-sa |
     us-federal | embed-only | licensed(paid) | unlicensed -> reject
  -> media capture object: bytes in content-addressed store (or embed ref),
     licence fields, credit line, retrieval provenance
  -> Jev typed judgement: does this image add information this story's
     text does not?
       { kind: event|subject|place|data|document|primary_source|generic|decorative,
         adds_information: bool, currentness: current|archival|unknown,
         depicts_matches_story: bool, risk: [wrong_subject, outdated,
         misleading_crop, real_person_ai, licence_doubt], confidence: 0..1 }
  -> placement: none (default) | inline collapsed | inline expanded | front lead
  -> caption (Texture-authored, states date/place) + credit line (licence-required)
```

The judgement is a typed Jev call, like the front-importance scorer
(`internal/frontimportance`), so it is cheap enough to run per candidate.
A second pass can check the caption against the image's own metadata
(date taken, coordinates, depicted entity).

### 3.3 Failure modes and guards

| failure | example | guard |
|---|---|---|
| wrong person | Commons file of a namesake; a mislabeled portrait | match on Wikidata entity id, not name; require `depicts` structured data or P18 |
| wrong place | photo of a different town with the same name | match on coordinates / Wikidata id |
| outdated photo presented as current | 2019 flood photo on a 2026 flood story | caption must carry the photo's date; Jev `currentness`; lead images must be current or labelled "file photo, YEAR" |
| licence laundering | Commons upload of a news agency photo as "own work"; Flickr washing | prefer files with structured licence and long history; reject when author is an agency or uploader has deletions; verify at origin |
| licence changes later | Flickr user switches to all-rights-reserved | snapshot licence at retrieval; keep the evidence blob |
| misleading crop | crop removes context (a crowd that was small) | keep original aspect for event photos; record crop in modifications; BY-SA adaptation marked |
| chart misleads | truncated axis, unlabelled units | chart spec rules: zero baseline for bars, units and source on the chart |
| AI image read as real | photoreal illustration of a protest | never photoreal; visible label; C2PA manifest |
| decorative creep | every story gets a picture because the page looks better | "no image" default; front page tracks share of stories with images as a smell, not a target |
| real-person likeness in promotion | portrait used in an ad for Autopaper | news use only; publicity law (Mass. c. 214 § 3A) covers advertising |

## 4. Video, audio and transcripts

- **Embed, do not host.** YouTube's terms permit its embeddable player and
  forbid downloading; the API policies forbid modifying the player or
  splitting audio from video. Spotify and Apple Podcasts provide embeds.
  For a podcast, play the publisher's own enclosure URL from its RSS feed
  (their host, their analytics). Never re-host the audio.
- **Timestamps.** YouTube embeds accept `start`/`end`; Spotify episode
  embeds accept a start point; `<audio>` supports seeking. A citation can
  therefore play from the quoted moment in all three.
- **Transcripts as sources.** A transcript capture has the same
  provenance as a web capture, plus a retrieval method (publisher
  transcript, channel captions, Choir speech-to-text with model/version)
  and **timed blocks**. Quotes verify against one block (unchanged
  `quotematch`); the citation adds a `timestamp_range` selector (already
  defined in `internal/sourcecontract/selector.go`, no consumer yet).
  Machine transcripts are labelled as such; names and numbers that a claim
  hangs on need a second pass or a human listen.
- **Rights.** Transcribing to report and quoting a few lines with a
  timestamped link is the *Swatch* pattern. Publishing a full transcript
  of a commercial show is substitution. Transcribing YouTube is a
  **contract** question (terms forbid automated access; official captions
  API only for videos you can edit), so the conservative route is to
  transcribe the same show from its podcast RSS when one exists and cite
  the YouTube timestamp for playback.

## 5. Product design sketch (no code)

### 5.1 Front page

- `FrontStory` gains an optional `lead_media` (media object id, kind,
  aspect, credit line, alt, value score). Absent by default.
- A lead image appears only when the Jev value judgement says
  `adds_information` with kind event, place, data or primary source, and
  `currentness` is not `unknown`. Subject portraits do not lead.
- Charts and locator maps are first-class leads, often better than photos
  for economic and geographic stories.
- Credit line under every image, linked to its licence; "Illustration
  (AI)" label where it applies.

### 5.2 Texture: a media block

Today a Texture document has no media node; media lives only as source
entities shown inside citation popovers (see
[repo-media-survey.md](repo-media-survey.md)). Proposal:

- A block node `media_ref` whose attrs point at a **source entity** (not
  raw URLs): `source_entity_id`, `display_mode` (`collapsed` |
  `expanded`), optional `selector` (`timestamp_range` for audio/video,
  `image_region` for a crop), and a Texture-authored `caption` and `alt`.
- The source entity carries kind (image, chart, map, youtube_video,
  audio, transcript), licence, credit line and provenance. One provenance
  path for text citations and media.
- Texture remains the sole writer: research proposes media candidates;
  Texture places, captions and decides display mode.
- Schema change to `internal/texturedoc/schema.go` is a **red** surface
  (Texture canonical writes) and needs full ceremony.

### 5.3 Expand and collapse like citations

- Reuse the existing citation expansion (`toggleInlineSourceRef`,
  `data-source-expansion-surface="media"` in `TextureEditor.svelte`) and
  the media renderer (`sourceEntityMedia` in `texture-source-renderer.ts`).
- Collapsed: a small chip in the text ("map", "video 2:13", "photo") next
  to the claim it supports. Expanded: the figure with caption and credit.
- **Auto-expand** when the value judgement is high (event, place, data,
  primary source with confidence ≥ threshold); collapsed when merely
  relevant (subject portrait, archival); never shown when decorative.
- The published renderer (`texture-published-citations.ts`) must learn to
  render media; today it never calls `sourceEntityMedia`.
- A transcript-linked citation: the quote renders as usual; its expansion
  shows the player positioned at the segment start, with the transcript
  segment highlighted.

### 5.4 Where images live and how the licence travels

- **Bytes we may host** (CC, PD, our charts/maps): content-addressed in the
  same store as capture bodies (`CaptureBodyStore`, `sha256:<hex>`), as a
  media capture object beside `choir.web_capture`, with licence fields and
  a licence-evidence blob (the API response or licence page snapshot).
- **Embeds** (YouTube, Spotify, Apple, Getty): no bytes; store the embed
  reference and oEmbed metadata; refresh per platform rules (YouTube API
  data at most 30 days).
- **Captures keep candidates, not bytes**: og:image, `<figure>`
  `<img src alt>` + caption, RSS `enclosure`/`media:content` recorded as
  metadata with the capture so the candidate generator has something to
  look at. The bytes of an unlicensed image are never stored or shown.
- The credit line is derived from the licence fields at render time, so
  every place the media appears (front page, story, Autopaper object
  graph adoption) carries it. A computer adopting a story adopts the
  media object with its licence, never a bare URL.
- Optional later: emit C2PA manifests for images Choir generates.

## 6. Recommended first steps (small, in order)

| # | step | class | rollback |
|---|---|---|---|
| 1 | This research; decide the AI Act Art. 50(4) stance (editorial responsibility named in Autopaper's about page, or an "AI-written" label) | green | revert doc |
| 2 | Record rights signals at fetch: robots.txt verdict, TDMRep, paywall state, onto capture provenance. Record only, no blocking. | orange | revert commit; fields are additive |
| 3 | Keep media candidates in captures and feeds: og:image, figure img/alt/caption, RSS enclosure/media:content/itunes:image as metadata only | orange | revert; metadata only |
| 4 | Published renderer shows embeds for media source entities that already exist (YouTube, audio enclosures), collapsed by default | orange | revert frontend commit |
| 5 | Locator maps and charts from cited data as the first generated images (OSM credit, chart spec rules) | orange | revert; no stored third-party bytes |
| 6 | Jev typed judgement "adds information?" plus front-page `lead_media`, behind the "no image" default | orange (model routing touches Jev) | revert; field optional |
| 7 | Commons/Wikidata lookup with licence verification and evidence snapshot | orange | revert; delete media objects |
| 8 | Texture `media_ref` block with display mode and selectors | **red** (Texture canonical writes): full ceremony | schema version gate; revert |
| 9 | Transcript captures with timed blocks and `timestamp_range` citations; decide the YouTube InnerTube fallback (likely: disable, prefer podcast RSS + speech-to-text) | orange | env kill switch exists (`CHOIR_DISABLE_YOUTUBE_TRANSCRIPT_FETCH=1`) |
| 10 | One hour with a media/IP lawyer on §7 | green | n/a |

Steps 2-3 and 5 are independent; 4 is the quickest visible win; 8 waits
until 4-7 show what the block must carry.

## 7. Questions for a lawyer

1. Does Autopaper's "editorial responsibility" (an automated pipeline with
   a named owner and checks) meet AI Act Art. 50(4)'s exception, or must
   we label AI-written stories?
2. Retention of full capture bodies: is indefinite storage of fetched
   article text consistent with DSM Art. 4 ("as long as necessary") and
   with UK law, which has no commercial TDM exception?
3. Under DSM Art. 15, how long may a verified quote from an EU press
   publication be, by member state, and do public transclusions count?
4. Is playing a podcast's enclosure from the publisher's URL inside a
   story covered by implied licence? Does the answer change if we show
   our own transcript beside it?
5. YouTube: is fetching channel-published captions at low volume for news
   reporting an acceptable risk under its terms, or do we need permission?
6. Do we have exposure on the server test if we embed third-party images
   (Getty viewer, social embeds) from Massachusetts or Delaware?
7. Which forum risks matter given Delaware incorporation, Massachusetts
   operations and worldwide readers (EU publishers suing under Art. 15 in
   their home courts)?
8. Do we need a DMCA agent now (do readers post anything), and what
   takedown process should Autopaper publish?

## Sources

All legal and licensing claims are cited inline in the three supporting
files; the README's claims summarise those citations. Key primary
sources: [17 USC 107](https://www.law.cornell.edu/uscode/text/17/107),
[DSM Directive 2019/790](https://eur-lex.europa.eu/eli/dir/2019/790/oj),
[AI Act Art. 50](https://artificialintelligenceact.eu/article/50/),
[CDPA s30](https://www.legislation.gov.uk/ukpga/1988/48/section/30),
[Japan Copyright Act](https://www.cric.or.jp/english/clj/cl2.html),
[Copyright Office AI reports](https://www.copyright.gov/ai/),
[YouTube Terms](https://www.youtube.com/static?template=terms),
[YouTube API policies](https://developers.google.com/youtube/terms/developer-policies),
[Swatch v. Bloomberg](https://fairuse.stanford.edu/case/swatch-group-v-bloomberg).
