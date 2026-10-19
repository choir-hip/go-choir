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

## Stored whole-capture embeds (traced 2026-10-19)

8 of 60 front stories carry a quoteless `expanded_ref`. All four checked
were first published on Oct 15–16, before `{{embed}}` was retired
(3143ade5b, Oct 16 18:13). No current path mints one: story opening
seeds an empty body, and Write mints only `numbered_ref`. They survive
because Write copies stored citations back unchanged, on purpose
(`TestStoredEmbedsRoundTripWithoutTheEmbedToken`). Besides the lone "1",
the embed's capture **counts toward the publish gate's outlets**
(`CitedCaptures` and `citedOutlets` read any Ref), though it grounds no
sentence.

Fix (red: a Texture canonical write): after each Write, `Apply` drops
every quoteless `expanded_ref`, and any paragraph it leaves with no words,
including in blocks the Write did not touch. Quoted stored embeds stay. A
story whose second outlet was only such an embed will be held at its next
revision. That is correct, but visible: watch for new holds that name one
outlet on these eight stories. Stories that are never revised keep the
embed. A read-side skip would cover them, and is not done.

## First hour after deploy (02:15 to 02:58, small sample)

- 39 runs, all gpt-6-luna. Holds as usual. Console clean; the three
  "OOM" matches are oomd's boot lines, not kills. btrfs metadata rose
  from 86.2% to 87.4% with the new image.
- **New story written under the rules** (79b4bfc1, KillSwitch): 0 of 9
  sentences name an outlet. It credits the creator, cites the
  repository (a primary record), and its open question is in the
  story's terms.
- **Revisions keep the old style.** Iran (a648c3bb v7) still ends 6 of 6
  sentences with "Al Jazeera / The Hill reports", even after crediting
  the speaker ("Vance said … The Hill reports"). The rule's escape
  clause ("name an outlet … where one alone reports a lead") covers
  almost every single-source claim. That is a wording loophole, to fix
  if the next hours confirm it.
- **Pipeline voice survived once** (Steele-John v8): "The capture is a
  feed extract, not the full article, and attributes the dossier
  account to Nine …". G5 for that rule, at least on gpt-6-luna.
- **Support, since 02:16:** 9 records over 27 Texture runs, 13 failing
  pairs. Their median text fell from 417 to 183 characters. The pairs
  without context are blocks with one closing citation (S3 by
  design). **Attribution clauses fail support on their own:** "according
  to TechCrunch, which reported the announcement on Oct. 18" (p 0.44)
  and "The BBC reported the announcement as …" (p 0.18). The passage
  holds the fact, not the outlet's framing or date. So relaying is not
  even the safe sentence under the new unit.

## Second hour (02:58 to 03:59): revisions still write relay sentences

- 43 runs, all gpt-6-luna. Console clean (no panic, fatal, OOM or "ended
  run" in the window). btrfs metadata 87.5%.
- **Short stories follow the rules.** Austrian Airlines (new, 0 of 3
  sentences name an outlet) credits "Bulgarian authorities". Alberta
  base fire v16 (2 of 5 before, 0 of 3 now) credits Prime Minister
  Carney. Lambie v13 (2 of 3 before, 0 of 4 now) states the facts and
  credits Lambie.
- **Revisions write new relay sentences, not only carry old ones.** On
  the 7 baseline stories revised since the deploy, I split each
  sentence into new (not in the baseline text) or carried. **9 of 22
  new sentences name an outlet as the subject (41%)**, against 9 of 46
  carried (20%) and 38% at baseline. Examples, all written after the
  deploy: "Euronews says the town hall's spire collapsed" and "Euronews
  says 19 people were detained" (Limoges v5), "Al Jazeera reported on
  Oct. 19 that President Donald Trump accused …" (Zelenskyy v14), "The
  BBC describes video published by news outlets as showing gunmen …"
  (Tiger Shroff v5), "Araghchi also warned …, The Hill reports" (Iran
  v7). Each is a one-source claim.
- **Cause (from the prompt text and the output, not a frame trace):**
  the rule calls a claim confirmed only with a primary record or two
  reports, and lets Texture "name an outlet only where outlets disagree
  or one alone reports a lead". Almost every claim in a revision has
  one source, so the escape clause covers it. The new stories pass
  because their few claims have a primary record or an official maker.
- **Pipeline voice in other words.** Maine v20 carries "three newsrooms
  … describe the same beats", "a fourth outlet carrying the account,
  but not a fourth separate one" and "in the material reviewed". The
  rule's four named phrases do not cover these.
- **Support:** 7 records over 22 Texture runs (32%), 10 failing pairs,
  median text 160 characters. All 4 failing pairs on the Anthropic story
  (3 records in 5 runs) are relay sentences: "according to TechCrunch,
  which reported the announcement on Oct. 18", "The BBC reported the
  announcement as …", "Al Jazeera says the fund could be used …".
  Others are hedges about sources: "that is the authority's account, not
  an identification of …", "the cited account does not identify them".
- **The pair-unit conjecture is half confirmed.** The failing-span
  length fell as predicted (417, then 183, then 160 characters). But
  records per Texture run did not fall below a tenth: 33% then 32%. The
  paragraph unit made failures long. It did not cause most of them. Most
  remaining failures are sentences about outlets and sources, which no
  quote can support. So the writing rule, not the judge, is now the
  lever for both groundedness and support churn.

Decision (yellow, prompt text): close the escape clause. An event or
statement a report describes is stated as fact with its cite, credited
to whoever made the statement. An outlet is named only for its own
unconfirmed finding (an unnamed source, its own analysis) or where
outlets disagree. The pipeline-voice rule widens to cover talk about
outlets and coverage. A revision rewrites a block it finds breaking
these rules. Risk to watch: G1 (single-source claims stated as fact),
and G4 (a one-time rewrite wave as each story's next revision fixes
old prose, with more support judgements and published versions).

Deployed: 8be39bb36, CI run 39197635727 green, staging at 04:14:35.
The paper's `/api/prompts` serves the new Texture default (the new rule
present, the escape clause gone), and runs resumed by 04:19, all
gpt-6-luna. Measure from 04:15: new-in-revision outlet share (41% this
hour), G1 and G4, and support records per Texture run (32%).

## First hour on the second rule set (04:15 to 05:15, thin sample)

- 30 runs, all gpt-6-luna; console clean since the 04:14 restart; btrfs
  metadata 86.6%.
- Only two front stories changed. OpenCode v11 credits OpenCode itself
  in all three sentences, with no outlet named.
- France protests v25 to v27 (the worst baseline story, 17 of 20 outlet
  sentences) **kept 16 of its 19 carried outlet sentences**, and 2 of
  its 3 new sentences name an outlet ("CNN reported protesters in Lille
  and Marseille …", "…, BBC correspondent Katya Adler wrote"). The new
  "a revision rewrites any block that breaks these rules" did not fire
  on this revision. Hypothesis, not traced: the turn was a small
  integration edit, and "change only the blocks that change" won. Its
  arrest-count paragraph (Al Jazeera, the BBC, the Guardian and DW
  giving 1,200 to 2,400) is the case where naming outlets is right:
  they disagree.
- Support: 3 records over 17 Texture runs (18%, from 32%), 4 failing
  pairs. Two are France's relay sentence; two are a DeepSeek benchmark
  sentence ("Its model page lists …").
- Not yet seen: a new story under the second rules, or a revision that
  rewrites old relay prose. Next hour decides whether the rewrite rule
  needs a stronger trigger.
