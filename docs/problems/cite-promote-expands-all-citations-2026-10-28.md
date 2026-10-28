# The b@c citation candidate leaves every citation open (2026-10-28)

Status: open; documented before any fix. Owner observation, from a
screenshot of a story on b@c's computer (choir.news, "Anthropic's IPO filing
tempers near-term AI science claims and flags gas-price risks", v3).

## Evidence

Every citation block ([1], [2], [3]) renders expanded inline under its
sentence, each with its quote, its "exact / supports the sentence" badge and
a Close button. Nothing was clicked.

## Intended behavior (owner)

"The point was just to get rid of the hover overlays and make the citation
numbered buttons toggle and not disappear when clicked. But all citations
should still be collapsed by default."

## Consequence

The candidate (lane/cite-promote 75d6bc366, b@c's self-development output)
must not be promoted to other computers as it stands. Owner: "Good thing we
didn't get b@c.com version on other machines... we will proceed with
self-development after our rate limit reset with our Claude code delegate."
The fix is a new self-development round on b@c: citations collapsed by
default, the numbered button toggles open and closed and stays visible, and
there are no hover overlays.
