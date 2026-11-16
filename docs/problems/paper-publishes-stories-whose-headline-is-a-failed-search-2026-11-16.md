# The paper publishes stories whose headline is a failed search

2026-11-16, 08:10Z. Seen while testing Autoradio against the live paper: the
radio read one aloud as news.

## Problem

Two items on the public front list (`GET /api/platform/publications/front`,
100 newest, read at 08:08Z) are not news. Their headlines report that the
paper's own research did not find what it was looking for:

- "No opinion piece found framing Vance's Europe criticism against a divided
  America" (politics, updated 08:05Z)
- "Belgian youth protests confirmed; De Wever's 'coalition of the willing'
  call not found — his Ukraine remarks date to Oct…" (world, updated 07:58Z)

The first was the "now playing" item of Autoradio's autostart on the phone.

## Evidence

The front list's own answer, above. Two of 100.

## Belief (not confirmed from traces)

A story or a revision is being written from a verification or follow-up
search whose result was negative, and the negative result becomes the
headline. Tonight five paid search providers are out of quota and two
(keenable, SearXNG) answer everything; thinner search results may make
negative findings more common. Neither claim has been checked against the
runs' ledgers.

## Remaining error

Which desk and which step produced them, how many there are beyond the
newest 100, and whether they are new stories or revisions of real ones
(`version_count` was not read). Nothing was changed on the paper: the
lead's authority there tonight is read-only. Next step: read the two
publications' run ledgers before any fix.
