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

## Changes made (2026-10-19 about 02:00, owner: "You can pursue this and make smart changes")

1. **Support unit (red).** A pair is now the span its citation closes,
   with the paragraph as context, so a fact stated plainly with two cites
   is judged against each quote alone. A failing span is quoted to
   Texture, with "naming the outlet does not make a claim supported".
   See the "Fix chosen" section of the pair-unit doc.
2. **Texture writing rules (yellow).** Two news rules became three:
   - "Report the news, not the coverage", with a plain two-cite example
     against "DW reports … Al Jazeera says …";
   - credit a claim to whoever made it, never the relaying outlet; name
     outlets only where they disagree or one alone has a lead; prefer a
     primary record; a two-source sentence cites each source after its
     part;
   - no pipeline voice (four named phrases); the headline states the
     news, never the state of the sources.
   "State each claim in its source's own terms" is gone. The prompt
   budget rose from 4.56k to 5.15k characters.
3. **Research (yellow).** When a report relays a statement, document or
   figure, research reads the original and sends it as that claim's
   source beside the report.

Not changed yet: the source registry (primary feeds wait for the owner's
sources work), and the stored embeds (being traced).

Ways the writing change can fail, to watch in new stories and revisions:
- G1: Texture states a single-source claim as fact (over-correction).
  Watch for plain statements that carry one cite from one outlet;
- G2: a plainly stated two-cite sentence fails support anyway, because
  one outlet's quote says less. That shows up as `system:support`
  failures on short spans with two cites;
- G3: outlet names vanish where outlets really disagree;
- G4: stories grow longer or churn more (versions per story-hour);
- G5: the writing does not change at all, because the support pressure,
  not the prompt, was driving it, or the prompt is ignored.

Before: `grounding/baseline-front-20261019T0152.json` in the session
scratchpad, 60 front stories with their text and sources. Measure after:
outlet-attributed share of sentences (38% before), primary sources per
story (4 of 113), pipeline-voice sentences (7 in 25 stories), failing-pair
median length (417 characters), and support records per Texture run
(about a fifth to a quarter).
