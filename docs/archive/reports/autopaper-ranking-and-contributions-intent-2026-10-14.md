# Autopaper: Front-Page Ranking, Personalization and Contributions (Intent)

Date: 2026-10-14. Owner direction (in session, about 12:40 UTC), recorded
before any design work. Status: intent. Not actionable until investigated
and designed; nothing here changes the live front page.

## The problem the owner named

"What's the auto paper algorithm? Not for making stories, but for ordering
stories, laying out the homepage. [...] What are we learning? How are we
optimizing that? And how are we letting that be personalized? [...] I hate
thumbs up and thumbs down." Example: the story about someone caught spying
in OpenAI's office sits far down the front page, while a story about a
company using Codex for productivity is prominent without scrolling.

Why, today: the front page ranks with `rankFrontStories`
(`frontend/src/lib/autopaper.ts`, from the newspaper UI of 10-14): newest
first, lifted by version count within a fresh window. Version count
measures Texture and research churn (and recently stub repairs), not
importance. The objective is wrong, not only its weights.

Owner caution: "I don't think I want to actually edit this right now [...]
if you edit it, you might break it."

## Station LF: front-page ranking and personalization (intent)

**Optimize importance, with attention as a weak signal.** Signals:
- breadth of coverage: independent outlets and languages per event (free
  from L2a's synthesis intake: an event is a merged story);
- velocity: new captures per hour on the event;
- relevance to the paper's standing questions (L4);
- an editorial judgment of consequence and novelty, recorded with reasons;
- reader attention (opens, dwell, depth, returns), weighted low, never the
  objective, to avoid optimizing for clicks.

**Learning by scored predictions.** Ranking a story commits a prediction of
its importance to the ledger (a precommitment record: "this will draw
follow-up coverage, will matter"); later evidence settles it (coverage
growth, follow-ups, sustained attention). The ranker is calibrated against
settled predictions instead of tuned by hand.

**Personalization without thumbs.**
- Implicit: what a reader opens, how long, how far, whether they return to
  a story's updates.
- Explicit actions with meaning: **Follow this story** (updates when it
  develops), **Less like this** (mute a topic or source, optional one-line
  reason), **More on this** (ask your own agent to dig deeper).
- Where it lives: the shared front page is the paper's editorial ranking;
  each reader's own computer adopts the stories from the Autopaper object
  graph and re-ranks them privately. Attention data stays on the reader's
  computer.

**Layout.** The lead story is the most important, not the most revised;
sections by topic; a "developing" mark for velocity.

## Station LC: contributions, op-eds and editing (intent)

The owner's primary use case: "use the auto paper [...] newspapers have the
op-ed section. I want to do my own op-ed. But referencing all of the news
content that we're building." And: "the published articles should be
editable [...] not comments, but just editing the articles directly [...]
you could have forking, or [...] make a new texture that transcludes this
one."

- **Bring your own sources:** a shared link or file becomes a capture on the
  contributor's computer with provenance, citable like any source.
- **Op-eds:** a Texture document that cites the paper's stories at specific
  published versions; published into a clearly labelled **Opinion** section,
  separate from synthesized news.
- **Editing published stories:** Texture stays the sole writer of the
  canonical story. A reader's edit is a **proposed revision** with its
  sources; Jev checks the citations; Texture or management accepts it; it
  appears in the version history with the contributor's name.
- **Versions and inclusion:** "make your own version" forks with recorded
  lineage; "include this story" embeds a live, updating copy (transclusion).
- **Trust and moderation:** contributor provenance on every edit and source;
  labelled opinion; accepted edits only through the same checks as desk
  writing.

**Words for readers who don't know "transclusion":**

| concept | plain words |
|---|---|
| transclude | **Include this story** ("stays up to date as the story develops") |
| fork | **Make your own version** |
| respond / op-ed | **Write a response** |
| cite a pinned version | **Quote this** (keeps the exact version) |
| proposed revision | **Suggest an edit** |

## Next steps (when the owner opens these)

Each station gets an investigation (current ranking inputs; what the
ledger already records per story; attention capture that respects
privacy), a design note, and a review panel before any code. The live
front page is not changed until the owner reviews the design.

## Owner update, about 12:10 UTC: a first slice before the demos

"Maybe I do want to make these improvements before my meetings [...] But
yeah, let's be safe about it [...] I definitely don't want to break it."
And: "I would trust models to understand that the [...] OpenAI spying story
is more important than some story about [...] Codex [...] which is
basically an ad."

Evidence (preview, not deployed): Gemini, asked to score each front story's
importance 1-10 and flag promotion, put the Pope's surgery, Ukraine's power
loss and the UK stillborn-burials apology at 8-9, the OpenAI bugging arrest
at 7 (from #33 to #12 once recency decays with a 24 h half-life), and gave 1
with a promotion flag to "1Password reports productivity gains from Codex"
and to two leaked management instruction titles ("Open a story on ...").
A second preview, ranking by distinct cited outlets, lifted research-heavy
stories instead, so breadth alone is not importance.

Second problem found: **management instruction text published as a
story title.** Two front-page stories are titled "Open a story on ...",
the wording of a management instruction, not a headline.

First slice (LF0), additive and reversible: a host timer asks Jev (the
gateway's typed-judgement model) for each front story's importance and a
promotion flag, keyed by publication and revision; corpusd attaches the
scores to the front API when the file exists; the front page ranks by
importance with recency decay only when most stories have a score, and
keeps promotion and instruction-titled stories out of the lead and top
tier. Missing or bad scores fall back to today's ranking. Rollback: revert,
or delete the scores file. The rest of LF (scored predictions, breadth,
personalization) is unchanged intent.

## Owner update, about 13:45 UTC: depth in the ranking, deepening, sections

Depth: the new lead ("Mauritius reels from worst riots...", v3) is a
one-paragraph stub citing one source. "Part of the ranking needs to be the
quality of our article. [...] if it's important, we should be devoting
more resources towards making sure we do have a quality article. That is
doing web research in addition to pulling from our source ingestion."
So: rank by importance and depth (sources cited, length); an important,
thin story is deepened by research that searches the web
(docs/problems/front-page-leads-with-a-one-source-stub-2026-10-14.md and
docs/reports/autopaper-deepen-important-stories-2026-10-14.md, to come).

Sections: "a very simple classification of world news, U.S. politics,
business and finance, and tech and AI. [...] pages or sections where
users can move between them. [...] whichever [...] they select should be
remembered. So then they close and reopen the app. If they were just on
the tech and AI section, they should return back to that. And then that is
also something to learn from. And then the front page can be learned and
customized from that. Very simple."

Slice LF1: the importance scorer also assigns one of four sections (World,
U.S. Politics, Business & Finance, Tech & AI); the front page gets a
section switcher (Front page plus the four); the reader's last section is
remembered on their device and reopened; section visits are counted on the
reader's device only and lift that section's stories on their front page
(private personalization, per LF: attention data stays with the reader).

## Owner update, about 14:15 UTC: more sources, sublinear articles

The sections showed thin verticals (of 100 front stories: Tech & AI 62,
World 31, Business 4, U.S. Politics 3), which mirrors the paper's 14
sources (mostly tech). Owner: "what is the quickest path to getting more
sources so we can scale up our verticals in less than two hours?" Then:
"we should be able to scale up the sources while only scaling up article
production significantly sublinearly. More sources means only marginally
more articles. Fundamentally, the ratio of sources to articles is a crude
proxy of quality."

Done: six registry sources added to the paper's schedules at 14:15
(CNBC Markets, MarketWatch, FT Markets, Federal Reserve press, Al Jazeera,
DW English); eight feeds added to the registry for U.S. politics and
business (NPR Politics, Politico, The Hill, PBS NewsHour Politics, Guardian
US Politics, BBC US & Canada, BBC Business, Guardian Business), scheduled
on the paper after they deploy.

Measure (with L2a's merge recall): sources cited per published story and
captures per new story should rise as sources are added, while new
stories per hour rise far less than captures per hour. Before: 14
sources; the depth log (`journalctl -u go-choir-front-importance | grep
depth:`) and the front API's source_count give the per-story numbers.
