# Hacker News point counts mint a new capture every poll and wake the story each time (2026-10-17)

Status: fixed (67201a94c, staging 09:07 UTC). Two-hour measurement in progress.

Found at 08:53 UTC during the hourly paper check. The story `365f889a`
("Llmspeed's linked page offers no captured product details") went from
4 revisions at 06:53 to 9 at 07:54 and 15 at 08:53, while the gate held it
off the front.

## Evidence (run traces, the paper)

**One wake, traced (run `a920cb2d`, 08:47:47).** The only record was a
management Note: "Same HN listing and linked page; Jev's existing story
already records this capture. Fed captures:
obj:choir.web_capture:…:key-54b68c1f…". Texture read that capture. It
was 120 bytes, from `rss:hn_frontpage`, with one block:

```
Article URL: https://llmspeed.dev/ Comments URL: https://news.ycombinator.com/item?id=50127979 Points: 52 # Comments: 21
```

The previous capture held the same text with `Points: 50`. Texture
rewrote b2 from "50 points" to "52 points … compared with 50 points in an
earlier captured listing". The write was refused for a bad `{{ref 1}}`,
then retried and committed. Each revision also opens new support
checks: the story's pack holds three unresolved `support-judge` records,
committed at 06:34, 07:47 and 08:32.

**The beat.** Since 06:48 (the oldest of the 100 runs the API returns),
the documents with the most Texture wakes all come from Hacker News, and
they wake on the 15-minute poll:

| Document | Texture wakes since 06:48 | Wake times |
|---|---|---|
| `365f889a` Llmspeed | 11 | :02 :17 :32 :47, some twice |
| `3cb6c4b9` "Nine-Inch Snails" | 8 | 07:02 through 08:47, every 15 min |
| `d4ffc335` car lights essay | 7 | 06:48 ×3, then every 15 min to 07:47 |
| `9ff4d12d` FLUX 3 Image Klein | 6 | 07:17 through 08:32, every 15 min |
| `4fe4f5df` Anthropic "90%" | 4 | 07:02, 08:02, 08:17, 08:47 |

Those 36 runs are 58% of the paper's 62 Texture runs in the window. Each
one also follows a management turn that Noted the capture.

**Published churn.** "Nine-Inch Snails" is on the public front with
**25 published versions**, the latest at 08:48. Every content-changing
revision is a new publication version, with its own pins, receipts and
resolved blocks in Store B and the evidence store. Store B already grows
by about 1.5M small files a day
(node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md).

## Mechanism (code)

1. `internal/sources/rss.go` `Poll` keeps the feed description as the
   item body (`cleanFeedDescriptionText`). For hnrss.org feeds that body
   is the link, the comments link and the live counters `Points: N` and
   `# Comments: N`.
2. `item.ContentHash` covers the body. `researchtools/poll_source.go`
   `handledKey` is `item.ID + "|" + item.ContentHash`, so an item whose
   counters moved is "an entry the feed rewrote" and is examined again,
   including the article read.
3. `internal/sourcegraph/captures.go` keys a capture by
   `canonical URL + "#" + DocHash(blocks)`. New counters give a new doc
   hash, so the result is a new capture, not `Repeated`.
4. The new capture is an arrival. Management sees the same URL already
   has a story and Notes it there (disposal by Note). Texture is woken
   and, being honest, folds the new count into the story.

Five feeds use hnrss.org: `rss:hn_frontpage`, `hn_newest`, `hn_show`,
`hn_best` and `hn_ask`, each polled every 900 s. An item can stay on the
front page for many hours. Lobsters and Slashdot descriptions carry no
live counters.

## Classification

This is substrate (source ingestion identity), not Texture or management
behavior. Both desks act correctly on what they are given: a new capture
of a story they already cover. The counters are engagement metadata, not
the content of the item. A capture's identity must not change when they
do.

## Fix direction (orange: capture content for HN feed items; rollback is a git revert)

At RSS parse time, drop hnrss.org's `Points: N` and `# Comments: N` from
the description before it becomes the body. Only drop them when the
description carries a news.ycombinator.com comments link, so that no other feed's text is touched.
With the counters gone the body is stable, so:
- `ContentHash` and `handledKey` stay stable, and the item is not
  re-examined or re-read each poll;
- the doc hash is stable, so the capture key repeats and nothing new
  arrives.

On deploy, each HN item still on a feed mints one more capture (the
counter-free form), then stops.

Ways the fix could fail (to test first):
- a counter change still yields a different body or content hash;
- the counters are stripped from a feed that is not HN, or from article
  text that happens to contain "Points:";
- the article and comments URLs are lost from the body (they are the HN
  item's only content);
- an HN item with no counters, or with a self-post text (Ask HN), loses
  its text.

Protected surfaces: capture content (world evidence). No Texture,
publication, Trace or vmctl code changes. Heresy delta: discovered (a
capture identity that moves with engagement counters), introduced none,
repaired on landing.

Measure after deploy (paper, 2 h): Texture wakes on documents with an
HN capture and a 15-minute beat (36 of 62 before), new versions of
"Nine-Inch Snails" (one every 15 minutes before), and new captures per
HN poll.

## Not fixed here

- Management Notes a repeat capture to an existing story even when it
  says it is the same listing. Once identical captures repeat, this stops
  arising for HN. For other sources that legitimately update an article,
  the Note is correct.
- The stories already opened on link-only HN items (the residual in
  texture-opens-stories-blind-to-their-capture-2026-10-16.md) stay as they are.

## Landed (67201a94c)

`dropHackerNewsCounters` in `internal/sources/rss.go` removes the
counters at parse time. `TestRSSPollerDropsHackerNewsCounters` was written
first and failed on the old parser. It checks four things: the body and
content hash are stable when the counters move; a non-HN item that also
reads "Points: N" is left alone; both URLs survive; Ask HN self-text
survives. CI run 39019383320 passed. Staging deployed it at 09:07:18,
and the paper took a planned restart at 09:06:56 with a clean console.

First hour (paper):
- Each HN story was woken **once** after the deploy, as expected, when the
  counter-free capture arrived: `365f889a` at 09:18, and `3cb6c4b9`,
  `9ff4d12d` and `4fe4f5df` at 09:33.
- The 09:48 poll woke **none** of them. Before the fix, each was woken on
  every poll.
- "Nine-Inch Snails" stayed at 30 revisions. The 09:33 wake made no new
  revision.
