# Media Sources and Their Terms: Images, Video, Audio, Transcripts

Date: 2026-10-14. Supporting file for [README.md](README.md). Licence and
terms claims cite the licensor's own page where it could be reached; bot
walls blocked a few (Unsplash, Pexels, NOAA, OpenAI), which are cited by
their canonical URL or help page.

## 1. Images with written licences

Ordered by how well they fit an automatic newspaper: written licence,
machine-readable metadata, and a chance of showing the actual subject.

| source | licence model | machine-readable metadata | fits Autopaper for | watch out | source |
|---|---|---|---|---|---|
| **Charts and maps Choir draws from cited data** | Choir's own work; underlying facts free; data licence travels (e.g. ODbL for OSM) | we generate it, so provenance is complete | numbers, places, timelines, the highest-value image type | must cite the data capture; never chart a number the story does not cite | (see README §3) |
| **Wikimedia Commons** | Free licences only (CC BY, CC BY-SA, CC0, PD); attribution and licence name required for BY/BY-SA | `imageinfo` + `extmetadata` (Artist, LicenseShortName, LicenseUrl, Credit, DateTimeOriginal) and Structured Data (depicts, inception, coordinates) | people (official portraits), places, buildings, historic documents, maps | "Flickr washing" and other licence laundering; uploads mislabelled as own work; people in photos have personality rights separate from copyright | [Reusing content](https://commons.wikimedia.org/wiki/Commons:Reusing_content_outside_Wikimedia); [Credit line](https://commons.wikimedia.org/wiki/Commons:Credit_line); [API:Imageinfo](https://www.mediawiki.org/wiki/API:Imageinfo); [CommonsMetadata](https://www.mediawiki.org/wiki/Extension:CommonsMetadata); [Structured data](https://commons.wikimedia.org/wiki/Commons:Structured_data); [Flickr washing](https://commons.wikimedia.org/wiki/Commons:Flickr_washing); [License laundering](https://commons.wikimedia.org/wiki/Commons:License_laundering) |
| **Openverse** (WordPress Foundation) | Aggregates CC and PD media from many providers; returns `license`, `license_url`, `creator`, `attribution` | API with ready attribution string | broad search across Flickr, museums, Commons | Openverse states it does not verify that licences or attributions are correct; re-check at the origin | [API](https://api.openverse.org/v1/); [Terms](https://docs.openverse.org/terms_of_service.html); [About](https://openverse.org/about) |
| **Flickr CC and Flickr Commons** | Per-photo CC licence; Flickr Commons = institutions' "no known copyright restrictions" | `flickr.photos.licenses.getInfo` and per-photo license id | event photos by citizens and agencies, archival photos | licence can be changed later by the uploader, so record the licence at retrieval time | [Flickr CC](https://www.flickr.com/creativecommons/); [Flickr Commons](https://www.flickr.com/commons); [licenses API](https://www.flickr.com/services/api/flickr.photos.licenses.getInfo.html) |
| **US federal works** (NASA, NOAA, USGS, DoD, White House, Congress) | Works of US federal employees in their duties have no US copyright | per-agency; usually credit lines | space, weather, disasters, official events | contractor and third-party images are copyrighted and marked; no implied endorsement; logos protected; NASA asks for credit | [17 USC 105](https://www.law.cornell.edu/uscode/text/17/105); [usa.gov](https://www.usa.gov/government-copyright); [NASA guidelines](https://www.nasa.gov/nasa-brand-center/images-and-media/); [NOAA disclaimer](https://www.noaa.gov/disclaimer) |
| **Library of Congress, National Archives** | Many items public domain; LoC "Free to Use and Reuse" sets are rights-cleared | APIs with rights fields | historical context, documents | rights vary per item; read the item record | [LoC Free to Use](https://www.loc.gov/free-to-use/); [LoC APIs](https://www.loc.gov/apis/); [LoC legal](https://www.loc.gov/legal/); [NARA catalog API](https://www.archives.gov/research/catalog/help/api) |
| **Europeana** | Every item carries one of ~14 standard rights statements (CC licences or RightsStatements.org) | API returns `rights` URI | European archives, museums, places | many items are "In Copyright" or "Educational use only"; filter to reusable statements | [Europeana API](https://api.europeana.eu/); [rights statements](https://pro.europeana.eu/page/available-rights-statements); [RightsStatements.org](https://rightsstatements.org/page/1.0/) |
| **OpenStreetMap** | Data under ODbL; requires "© OpenStreetMap contributors" credit; rendered maps from OSM data may be shared | full | locator maps for every place story | the public tile server is not for heavy production use; render or host tiles ourselves | [OSM copyright](https://www.openstreetmap.org/copyright); [Attribution guidelines](https://osmfoundation.org/wiki/Licence/Attribution_Guidelines); [Tile policy](https://operations.osmfoundation.org/policies/tiles/) |
| **Wire photos: AP, Reuters, Getty** | Paid editorial licences, priced by use and audience | via their APIs/feeds | the actual event, today | cost; terms usually restrict to editorial and to the licensed term; historic per-image price points cited in 2014 were ~$60-120 (Reuters) and ~$95 (AP), current pricing is by quote | [Reuters FAQ (archived)](https://webcf.waybackmachine.org/web/20211004045531/https://www.reutersagency.com/en/about/faqs); [Poynter 2014](https://poynter.org/?p=245005) |
| **Getty embedded viewer** | Free embed (iframe) of many Getty editorial images for non-commercial editorial use; Getty may place ads and withdraw images | none we control | a real news photo without paying | iframe from Getty (tracking, ads, can disappear); "commercial" is defined by Getty; check current terms | [Computerworld 2014](https://www.computerworld.com/article/1383852/getty-makes-35-million-images-embeddable-to-gather-data-monetize-usage.html); [Getty terms](https://www.gettyimages.com/company/terms) |
| **Unsplash / Pexels** (stock) | Free use without attribution; Unsplash forbids compiling photos to replicate a competing service; Pexels forbids showing identifiable people as criminal, ill or in sexual contexts, and implying endorsement | API | almost never for news; at most generic section art | stock photos are decorative by definition; misleading if read as depicting the story | [Unsplash license](https://unsplash.com/license); [Unsplash help](https://help.unsplash.com/articles/2612331-why-can-t-i-compile-photos-from-unsplash-to-replicate-a-similar-or-competing-service); [Pexels license](https://www.pexels.com/license/); [Pexels rules](https://help.pexels.com/hc/en-us/articles/360042332714-What-are-the-rules-for-using-Pexels-photos-or-videos) |
| **The cited article's og:image** | Not licensed to us. It is the publisher's (often an agency's) photo | `og:image` meta | linking to the source | copying it is "working from" a photo for the same purpose it was licensed for (*Warhol*); hot-linking is a "display" outside the Ninth Circuit (*Goldman*); in the EU, Art. 15 covers press photos | see [rights-by-jurisdiction.md](rights-by-jurisdiction.md) §1.1, §1.5, §2 |
| **Official social/video embeds** (X, Instagram, YouTube, Bluesky) | Platform terms grant embed via their player | oEmbed | showing the primary source (the statement, the video) | the platform's terms govern; *McGucken v. Newsweek* (S.D.N.Y.) found Instagram's terms gave no clear sublicence to embedders | [Wilson Sonsini](https://wsgr.com/en/insights/thinking-of-framing-or-embedding-content-new-york-federal-courts-question-the-copyright-server-test.html) |

**Attribution format.** CC licences require creator, licence name and link,
a link to the source, and a note of changes; the "TASL" pattern (Title,
Author, Source, Licence) is CC's recommended practice
([CC BY-SA 4.0 legal code §3](https://creativecommons.org/licenses/by-sa/4.0/legalcode.en);
[Recommended practices](https://wiki.creativecommons.org/wiki/Recommended_practices_for_attribution)).
A crop or a colour change is an adaptation; for BY-SA it must be shared
under the same licence and marked as modified.

## 2. AI-generated illustration

- **Copyright.** None in the US for purely prompted output; human
  arrangement can be protected
  ([Copyright Office Part 2](https://www.copyright.gov/ai/Copyright-and-Artificial-Intelligence-Part-2-Copyrightability-Report.pdf);
  [Thaler cert. denied](https://www.bakerdonelson.com/supreme-court-denies-certiorari-in-thaler-v-perlmutter-ai-cannot-be-an-author-under-the-copyright-act)).
- **Model terms.** Major providers assign output rights to the user and
  forbid deceptive impersonation; check the specific model's usage policy
  ([OpenAI terms](https://openai.com/policies/terms-of-use/);
  [Anthropic usage policy](https://www.anthropic.com/legal/aup)).
- **Disclosure law.** EU AI Act Art. 50(4): AI-generated or manipulated
  images that resemble real people, places or events must be disclosed;
  an evidently artistic work needs only a disclosure that does not spoil it
  ([Art. 50](https://artificialintelligenceact.eu/article/50/)). China
  requires explicit and implicit labels
  ([Loeb & Loeb](https://www.loeb.com/en/insights/publications/2025/03/chinas-ai-labeling-measures-and-mandatory-national-standards-take-effect-september-1)).
  Massachusetts publicity law covers a person's "picture" in advertising
  ([c. 214 § 3A](https://malegislature.gov/Laws/GeneralLaws/PartIII/TitleI/Chapter214/Section3A)).
- **Provenance labels.** C2PA Content Credentials embed a signed manifest
  (who made it, with what tool, from what ingredients); viewers can
  verify it ([C2PA 2.1 spec](https://c2pa.org/specifications/specifications/2.1/index.html);
  [contentcredentials.org](https://contentcredentials.org/)). Choir's own
  object graph already does the harder part (content hash + provenance);
  emitting a C2PA manifest is an export of what we already know.
- **Editorial rule (proposed).** AI illustration only for abstract
  subjects (a policy, a market mechanism, an idea), visibly labelled
  "Illustration generated by AI", never photorealistic, never a real
  identifiable person, never a depiction of a specific real event.

## 3. Video: YouTube and other platforms

| question | answer | source |
|---|---|---|
| May Autopaper embed a YouTube video in a story? | Yes. YouTube's terms permit showing videos "through the embeddable YouTube player". | [YouTube Terms](https://www.youtube.com/static?template=terms) |
| May we download the video or audio? | No, except where YouTube offers a download or with prior written permission. Same for automated access ("robots, botnets or scrapers"), except public search engines per robots.txt. | [YouTube Terms](https://www.youtube.com/static?template=terms) |
| API rules for the player | Don't modify, build on, or block player functionality; don't obscure YouTube attribution or links; don't separate audio from video; don't play from a player the user can't see; autoplay starts data collection on page load. | [YouTube API Developer Policies](https://developers.google.com/youtube/terms/developer-policies) (III.E.1, III.F, III.I) |
| API data retention | Most API data may be stored for at most 30 days, then refreshed or deleted. | same, III.E.4 |
| Can we fetch captions through the official API? | Only for videos the authorised user may edit. A third party cannot download another channel's captions through `captions.download`. | [captions.download](https://developers.google.com/youtube/v3/docs/captions/download) |
| oEmbed | YouTube exposes a public oEmbed endpoint returning title, author and embed HTML; oEmbed is an open spec for this. | [oembed.com](https://oembed.com/); `https://www.youtube.com/oembed?url=…&format=json` |
| Start at a timestamp | The embed player accepts `start` and `end` parameters (seconds). | [Player parameters](https://developers.google.com/youtube/player_parameters) |

**The tension for transcription.** There is no official path for a third
party to get a transcript of someone else's YouTube video. Both routes in
use elsewhere (scraping caption tracks, or downloading audio and running
speech-to-text) breach YouTube's terms even where the copyright analysis
(transcribe to report, quote a few lines) is sound. Choir's
`fetchYouTubeTranscript` already uses an InnerTube fallback (see
[repo-media-survey.md](repo-media-survey.md)); that is the contract risk
to decide on, not a copyright one. Options, most to least conservative:
(1) transcribe only where the publisher also offers the audio outside
YouTube (podcast RSS, its own site) and cite the YouTube timestamp for
playback; (2) use captions the channel itself published, fetched
at human-scale volume, recorded as a ToS-risk provenance flag; (3) seek a
licence or written permission for high-volume channels.

## 4. Audio: podcasts

| question | answer | source |
|---|---|---|
| Can a story play a podcast episode? | Playing the publisher's own enclosure URL from its RSS feed streams from the publisher's host (its download counts and ads stay intact). This is how every podcast app works. No case found squarely on a news site doing this; the argument is implied licence by public RSS publication. Ask a lawyer before re-hosting. | (no direct authority; flagged as residual) |
| Re-hosting the audio file | Not without permission. It is reproduction and distribution of the whole work and takes the publisher's analytics and ad revenue. | [17 USC 107 factors](https://www.law.cornell.edu/uscode/text/17/107) |
| Spotify embed | Spotify offers an embed and iFrame API; podcast episodes can start at a set point; governed by Spotify's Developer and Widget terms. | [Spotify embeds](https://developer.spotify.com/documentation/embeds); [Developer terms](https://developer.spotify.com/terms) |
| Apple Podcasts embed | Apple provides embed codes through its marketing tools. | [Apple marketing tools](https://tools.applemediaservices.com/) |
| Transcribing an episode to report on it | Copyright analysis mirrors *Swatch*: transcription is a reproduction for analysis; quoting the newsworthy lines with a timestamped link is reporting; publishing the whole transcript of a commercial show would substitute for it. | [Swatch](https://fairuse.stanford.edu/case/swatch-group-v-bloomberg); [Harper & Row](https://supreme.justia.com/cases/federal/us/471/539/) |
| Podcast licences | Most feeds carry no licence; some declare one (e.g. `<copyright>` or CC in show notes). Record what the feed says. | (feed field, no legal authority) |

## 5. Media as sources: transcripts as captures

A transcript is a capture like any other: fetched bytes (the transcript),
provenance (media URL, publisher, retrieval method: official captions /
publisher transcript / Choir speech-to-text, model and version), and
blocks. The difference is that each block carries a **start and end
time**. Then:

- The quote checker (`internal/quotematch`) needs no change in principle:
  it already matches a quote inside one block and returns the source's own
  span. A transcript block is one timed segment.
- The citation records a `timestamp_range` selector (already in
  `internal/sourcecontract/selector.go`) alongside the `text_quote`.
- The reader sees the quote; pressing the citation plays the media from
  the segment start (YouTube `start`, Spotify start point, or `<audio>`
  `currentTime` on the publisher's enclosure).
- Machine transcripts are a *derived* source: the checker verifies the
  quote against our transcript, not against the audio. The citation should
  say "machine transcript" and the story should avoid hanging a claim on a
  single uncertain word (names, numbers) without a second transcript pass
  or a human listen.
