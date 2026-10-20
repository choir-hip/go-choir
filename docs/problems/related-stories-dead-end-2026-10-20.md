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
  (see [article-open-slow-store-b-scans-2026-10-16.md](article-open-slow-store-b-scans-2026-10-16.md)).
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
