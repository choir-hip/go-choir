# A published story is a dead end: no way to another story on the same subject (2026-10-20)

Status: documented; design approved by the owner 2026-10-19; Phase 1 fix follows
in later commits. Design:
[related stories, computed from the graph](../reports/related-stories-graph-design-2026-10-19.md).

Owner, 2026-10-19: "Ideally we use the graph for this. If we need to do some
changes to the graph that's fine. Shared urls may not be enough." And: "The
proposed design for similar articles using graph sounds good."

## Problem

A reader who finishes a story on choir.news has nowhere to go next except the
front list, which is ranked by recency and importance, not by subject.

## Evidence (live probe, 2026-10-19, design §3.1)

The 120 stories on the front list were compared by the URLs each one cites.

- Only 10 of the 120 stories share a cited URL with another story, making 10
  linked pairs (about 8% coverage).
- Some of those pairs are spurious: a DOJ story was linked to a Zelenskyy
  story because both cite one live-blog or newsletter URL (a hub).

So a shared-URL signal is both thin and noisy. Coverage needs a wider signal
(the event, Phase 2); precision needs hub down-weighting (Phase 1).

## Evidence (code at 44650e46e, design §3.2)

- A published version cites captures through `Anchors[].Ref.Doc`, but no graph
  edge reaches a capture: `PublishTextureToGraph` writes the capture only inside
  the transclusion object's JSON metadata. Finding "which stories cite this
  capture" is a JSON scan, the access pattern that made article opens slow
  (see [article-open-slow-store-b-scans-2026-10-16.md](../archive/problems/article-open-slow-store-b-scans-2026-10-16.md)).
- `og_edges` already indexes `(to_id, kind)`, so the reverse lookup is cheap once
  a `cites_capture` edge exists.
- `graphCitationEdges` treats unknown edge kinds as external citations, so a new
  version-to-capture edge must be added to its skip list or every bundle gains
  one spurious citation per cited capture.

## Belief state

Remaining error: precision of the capture and canonical-key signals is
unmeasured on live data (ship criterion: at most 10% wrong over 40+ pairs).
Phase 2 (events) is out of scope here. Mutation class of the fix: red
(publication write path, Store B edge kind).

## Staging finding: live blogs link unrelated stories (02:32 UTC)

Deployed e7ea4afda (CI 39358014589). Backfill: 1,032 stories, 4,056
`cites_capture` edges written, 0 mismatches; a second dry run writes 0.
The deployed spec passes (3 of 3). Coverage: 12 of 60 front stories have
links (20%, against the 8% shared-URL baseline).

Precision fails the ship bar. I read all 27 linked pairs on the front 60:
10 are wrong and 1 is loose, about 40% against the 10% limit, and the
DOJ/Zelenskyy pair the design names as a must-not-link is among them.
Nine of the ten wrong pairs share one page by recapture (reason `source`,
`citers: 5`): `theguardian.com/us-news/live/2026/oct/19/...-latest-news-
updates`, a live blog that covered the Kansas rally, the East Potomac golf
course case, Ukraine and the midterms in one feed. It meshes five stories
(Kansas rally, midterm travel, the East Potomac judge, the approval poll,
Zelenskyy on Starlink). The tenth (Italy's protest decree to Macron's
curfews) shares a 2-citer source and is loosely related.

Cause: a live page is many subjects under one URL, and a recapture of it
is a different set of entries. Sharing it says nothing about the subject.
The hub cutoff (7 citers) does not catch it at 5.

Fix: a live page (path segment `live` or `liveblog`, or a slug naming
live updates) gives no vote, by exact capture or by recapture. The rest of
the read (exact shared captures, ordinary recaptures) is 16 of 17 right.

## After the live-page fix (0a541483c, about 02:45 UTC)

Front 60: 9 stories linked, 15 pairs, 14 right; the Italy decree to
Macron curfews pair is the one loose link. The live-blog mesh is gone.

Widened read (the bar asks for at least 40 pairs): the top 300 stories by
the front list give 38 distinct pairs. 33 are right. Loose or wrong: Italy
decree to Macron curfews; Zelenskyy on peace to a Russian diesel deal;
the Ilhan Omar search to DOJ suing California; an inflation-driven AI
stock rout to Anthropic's revenue; a Scion-Mononoke charge to Anthropic's
version control. That is 13% strict, about 8% if the Musk and Ukraine
borderline pairs count as related. At the edge of the 10% bar, not past
it. Each loose pair rests on one shared source cited by two stories.

Latency over the 300 requests (mostly uncached): p50 111 ms, p95 185 ms,
inside the 400 ms cold budget. The deployed spec passes (3 of 3), and the
AI disclosure spec passes against the deployed frontend.

Decision (No Blocking Asks): keep `CORPUSD_RELATED` on. Most links are
right and the paper is pre-launch. Calibrate next from data, not from this
sample: a single shared source between two stories is the weak case; the
candidates are requiring a second signal (another source, or a shared
event once phase 2 lands) when the only signal is one source.

Separate finding: a published story is titled "Write the story of BBC
economics editor Faisal Islam's analysis of surp…", an instruction leaked
into a headline. Recorded for the paper's quality checks; not a
related-stories defect.
