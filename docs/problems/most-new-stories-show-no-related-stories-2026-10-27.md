# Most new stories show no related stories (2026-10-27)

Status: open; documented before any fix. Mutation class of the fix: orange
(the related-stories read path, a product API). Owner (11:4x): "There should
always be 3+ related stories. Just a question of how related they are. But
never no related stories."

## Evidence

At 11:42 UTC FP0 read `GET /api/platform/publications/related?route=…&limit=5`
(the reader's own call, `frontend/src/lib/texture-related-stories.ts:49`) for
the 40 newest stories of `GET /api/platform/publications/front?limit=60`:

| links | stories |
|---|---|
| 0 | 25 |
| 1 | 6 |
| 2 | 3 |
| 3 | 2 |
| 4 | 1 |
| 5 | 3 |

No answer was `partial` (the 400 ms budget was not the cause). Plainly
related stories show nothing: three Israel-election stories ("Israelis vote
amid uncertainty over Netanyahu's future", "Israel votes as polls point to
Netanyahu coalition losing majority", "Israel's election campaign sidelines
Palestinian future") each have 0 links. The stories that do reach 3 to 5 are
mostly on entity links (Russia/Ukraine, Brazil, Venezuela) or shared sources.

## Why (code read, file:line on 88d804db5)

- The list is computed at read time from three strong signals only
  (`internal/platform/publication_related.go:678`): a shared cited capture
  or recaptured page, a judged `related_story` pair, or a shared *central*
  entity (`publication_related_entities.go:38`, `:56`).
- It is never padded, by design (`publication_related.go:353`, pinned by
  `TestRankDropsBelowThresholdAndNeverFillsTheList`). A story with no shared
  source, no judged pair and no shared central entity gets `[]`, and the
  reader hides the section (`texture-related-stories.ts:107`).
- New stories do get the entities ask: `research.related_story` is on for
  the paper (since 2026-10-26 11:25) and the finish rule requires a
  `story.entities` record (`internal/agentcore/inbox_records.go:853`). But
  entity links need a *central* entity that another story also marks
  central, and the lexicon is young (138 entities); and the backfill of
  older stories (running) is what gives a new story someone to match.

So "related" is all-or-nothing on strong evidence, and most stories have
none yet. The owner's rule replaces that: strength varies, presence does not.

## Proposed fix

A fallback tier in `relatedWalk`, after the strong candidates, filling the
list to at least 3 (when the paper has 4+ stories) from weaker signals,
ordered after every strong link and labelled with its own reason so the
reader can tell: shared title/summary terms (idf-weighted, stopwords out),
then same section, then recency. `rankRelated` stays padding-free (its pin
stays true); the floor is a separate step. The cache key gains the mode so
an old empty answer is not served. Bounded to one store read of recent
eligible stories so the 400 ms budget holds.

The entity coverage of new stories is a separate, slower lever (the
backfill, sped up at 11:4x to batches of 30 every 5 minutes).
