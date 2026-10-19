# Articles relay outlets ("BBC says, Al Jazeera says") instead of grounding claims (2026-10-19)

Status: documented, direction proposed, nothing changed. Raised by the owner
on 2026-10-19 from four phone screenshots of choir.news: "it's currently
kind of steganography of other outlets … the article contents are a bit
too 'he said she said' where he and she are eg Al Jazeera and BBC. Not that
we should make things up…but how can we develop more groundedness than
just repeating news claims?"

## Baseline (front page, 2026-10-19 about 01:40 UTC)

Measured on the top 25 front stories from `publications/front` and
`publications/resolve` (`artifact.content`, citation markers stripped).
The script lives in the session scratchpad (`attrib.py`). Its regexes
undercount: they miss "the minister says" and unnamed outlets.

- **38% of sentences (67 of 176) name an outlet as the subject** of
  reports/says/told/describes. The France protests story has 17 of 20.
  The Kenya Ebola story has 7 of 13. The Alberta story has 5 of 7.
- **Primary sources are rare:** 4 of 113 cited sources across the 25
  stories are primary (who.int, elections.ab.ca and two others). Only 4
  stories cite any. The Ebola story cites a WHO Disease Outbreak News
  item, yet leads with what an official told DW.
- **Pipeline voice leaks into prose in 7 sentences.** Examples: "its
  captured text says … but gives no further case details" and "These
  are dated figures and do not establish the tracing position …".

## Hypothesis: our checks train this style (not yet confirmed by traces)

Every sentence must carry a quote that matches a capture. The support
judge then asks whether that one quote supports the text
([support-pair-judges-whole-paragraph-against-one-quote-2026-10-19](support-pair-judges-whole-paragraph-against-one-quote-2026-10-19.md)).
And the publish gate counts outlets. Under those three rules, the safest
sentence is "Outlet X reports <paraphrase of X's quote>". A sentence that
states one fact corroborated by two outlets is judged against each quote
on its own, and today a paragraph that does that fails. Story c0fb708e
shows the pressure directly: under seven support failures, Texture
rewrote the ABC sentence as "ABC independently reports …", then "The
ABC's summary describes …", then "ABC's summary says …". Each rewrite
added attribution, not grounding.

To confirm: compare outlet-attribution density before and after the
support judge moved to Jev (2026-10-14), and in revisions that followed
a support failure against those that did not.

## What grounded would look like

1. **Corroborated facts stated plainly.** When two independent outlets
   agree, the sentence states the fact once and cites both. Attribution
   is kept for single-source claims, contested claims, opinion, and
   forecasts, where it carries information.
2. **Attribute to the origin, not the relay.** Say "Kenya's health
   ministry said", not "DW reported an official told it". When an outlet
   quotes a document or official statement, research fetches that
   document.
3. **Primary sources in the registry.** Examples: WHO outbreak news,
   government and ministry press rooms, election authorities, central
   banks, courts and filings, statistics offices, and wire services that
   originate reporting. More sources help most when they are origins,
   not more relays.
4. **Known, disputed, unknown.** Numbers are reconciled with who counted
   and as of when. Disagreements are named as disagreements.
5. **Continuity as grounding.** The paper keeps its own record of what it
   established yesterday. A later revision can build on that, which no
   repost can.

Order of change: the support unit first (judge a sentence against all its
citations together, or the cited sentence alone). The writing rule second
(Texture prompt, yellow): without the first, a plainly stated two-outlet
fact fails support and churns. Primary-source registry entries third,
with the sources work.

## Two defects seen in the same screenshots

- **An empty whole-capture embed still renders.** The Alberta story
  (v14) opens with a `source_ref` of `display_mode: expanded_ref` and an
  empty quote and hash. On the phone it renders as a lone "1" under the
  headline, and in text as "[source 1: Canada PM warns …]". The owner
  ruled on 2026-10-17: "We shouldn't have those embeds at all!" Not yet
  traced: whether this block was carried forward from an old revision or
  is written fresh.
- **A headline describes the pipeline, not the news.** "A /pkg explainer
  surfaces without its text" (tech_ai, importance 3) was titled while its
  only source was a link-only HN capture (held at 00:40). The body was
  later rewritten from the real anderegg.ca and eclecticlight.co pages,
  but the title kept the pipeline's state. The body also repeats markers
  ("[1] [1]").
