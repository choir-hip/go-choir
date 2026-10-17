# A link-only capture counts as an outlet in the two-outlet publish gate (2026-10-17)

Status: documented. Fix to follow.

Found on 2026-10-17 at about 17:00 UTC by the article-quality research pass
(a read-only Sonnet subagent), and confirmed in the code by the
orchestrator. No reader-facing case has been observed yet; this is a hole
in the owner's §24 rule ("We always should have two, ideally more").

## Evidence (code)

- `internal/agentcore/story_publish.go` `citedOutlets` (line 261) counts a
  cited capture as the outlet of `outlet.Of(capture.URL)`.
- A Hacker News feed item's capture URL is the **linked article**, not the
  HN item. Its body is only the links: the paper's capture
  `…key-54b68c1f…` (`rss:hn_frontpage`) holds one block, "Article URL:
  https://llmspeed.dev/ Comments URL: https://news.ycombinator.com/item?id=50127979"
  (docs/problems/hn-point-counts-mint-a-capture-every-poll-2026-10-17.md).
- `outlet.Of` names aggregators (HN, Google, Yahoo…) as no outlet
  (afd160449), but it is given the linked URL, so the HN capture counts as
  `llmspeed.dev`.

So a story that cites one real outlet's capture, plus a link-only HN
capture of any other site, passes the gate as "two outlets". The linked
site said nothing in that capture. Texture can quote from it only the link
text, which the quote check accepts because it is in the capture.

The stories titled about their own link-only capture (365f889a,
d4ffc335) were held only because they cited nothing else. About 2% of
openings are on link-only captures
(docs/problems/texture-opens-stories-blind-to-their-capture-2026-10-16.md).

## Classification

This is a symptom in the publish gate, which reads the capture's address and not its
content. The substrate cause, that a capture carries no body-thinness mark
from ingestion, is noted under "Not fixed here".

## Fix direction (orange: the publish gate; rollback is a git revert)

In `citedOutlets`, a cited capture counts toward outlets only if it holds
some of its outlet's own words. That means at least 40 letters remain once
its URLs and the HN link labels ("Article URL:", "Comments URL:",
"Points:", "# Comments:") are removed. The gate already reads every cited
capture's blocks (`CaptureBlocks`), so this adds no read. The hold message
names the uncounted capture, so Texture knows why.

Ways the fix could fail (to test first):
- a link-only capture still counts as an outlet (the bug);
- a real excerpt counts for less than it should, for example a short but real
  one-sentence summary of about 60 letters, or a non-Latin excerpt (CJK
  letters count as letters);
- a story citing two real outlets plus a link-only capture is held (it
  must publish);
- the hold message does not say a capture held only links.

Measure after deploy: the paper's `story publish … not published` lines
naming a link-only capture, and that no front story's outlets rest on a
link-only capture.

## Not fixed here (residuals)

- **The event layer counts link-only captures too.** It uses
  `outlet.Of(in.Item.URL)` at `event_layer.go:548` and `:668` for
  corroboration. It sees only title and summary, not blocks, and V3 event
  wakes are off on the paper, so today it only shapes research's event
  hints. It needs a body-thinness mark carried from ingestion
  (`CapturedItem` has no body kind or length).
- **Management is still told about link-only captures** and may open a
  story on one. Fetching the linked article for HN feeds
  (`store_body_policy` `bounded_text` for `hn_frontpage` and `hn_show`)
  would give real text. That is a registry decision for the sources work.
