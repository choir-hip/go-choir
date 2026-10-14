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
