# Opening an article costs about a second of server time, every time (2026-10-16)

Status: overlay fixed and deployed (575cb3ef5, 22:46 UTC); bundle cache and
Store B indexes remain open. Reported by the owner ("opening an article is slow. Makes me
think we have memory and/or storage pressure").

## Evidence (22:10–22:20 UTC)

**The resolve route carries a fixed cost of about a second.** The top ten
front stories were each resolved twice from outside
(`/api/platform/publications/resolve?route=<bare>`):

| Story | Size | First | Second |
|---|---|---|---|
| Katie Zacharia … bar exam (v31, 4 sources) | 61 KB | 2.71 s | 1.24 s |
| Wyden … long-term care (v2, 2 sources) | 45 KB | 1.13 s | 1.05 s |
| Black Forest Labs ships FLUX 3 (v5, 7 sources) | 206 KB | 1.24 s | 1.12 s |
| White House press secretary … (v17, 19 sources) | 394 KB | 1.26 s | 1.33 s |

The other six took 1.13–1.37 s on first read. A 45 KB story costs nearly
what a 394 KB one does, and a second read is no faster. The proxy's
lifecycle stage `platform_publication.resolve` agrees: 22 calls, 1124 ms
mean, 2744 ms max. The front listing from the same store takes 148 ms
(`platform_front.list`).

**The host is not short of memory.** Node B: 31 GB, 12 GB available,
swap unused, memory pressure 0.00. IO pressure was 2.8% (avg10), CPU load
1.8 on 12 cores. corpusd holds 112 MB; the corpus Dolt 2.3 GB.

**The reader pays it more than once.** `TextureEditor.svelte` resolves the
article, then polls the bare route every 60 s
(`PUBLISHED_POLL_MS`) for the whole bundle (up to 394 KB and about a
second of server time) only to read `nav.latest`.

## Measured (22:35, e8379a0ca's step log)

Twelve resolves of six front stories: the check overlay took 889–2399 ms
of each; every other step together took 30–50 ms (route lookup 3–37 ms,
transclusions 4–22 ms, entities 2–7 ms). The overlay is the cost, and it
does not depend on the story: it is the scan of every
`choir.publication_check` row in Store B.

## After the fix (575cb3ef5, deployed 22:46)

The overlay now reads each version's checks and judgements by ID, in one
`IN (...)` query each, instead of scanning every check row. The reader's
minute poll asks for the nav alone (3b345f87c). The same eight front stories
resolved from outside in 0.16–0.26 s, first read and second, against
1.1–1.3 s before: 36 KB to 257 KB, up to 10 sources. Since corpusd
restarted at 22:45 it has logged no resolve over the 300 ms threshold. The
deployed reader spec (`tests/autopaper-reader-deployed.spec.js`) passes,
2 of 2.

What is left is about 150 ms of fixed cost that grows with the story
(transclusions and entities are still N+1 reads) and no reuse of an
immutable version's bundle. The cache and the indexes below are still the
next steps, but neither is urgent now.

## Mechanism (code-read; confirmed for the overlay by the step log above)

`GetPublicationBundleByRoute` (`internal/platform/service_publication_read.go`)
rebuilds the bundle on every request, in sequence:

1. **Two scans by JSON path, with no index behind them.** The route is
   found with `GetObjectByMetadata("choir.public_route", "$.route_path", …)`,
   and the check overlay reads `ListObjectsByMetadata("choir.publication_check",
   "$.publication_version_id", …)` (`publication_checks.go`). Store B's
   `ObjectGraphStore` (`internal/platform/objectgraph_store.go`) runs both as
   `JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?)) = ?` over every
   row of the kind; the only index is `(object_kind, owner_id)`. Every
   citation of every published version has a check row, so the second
   scan grows with the whole paper, not with the story.
2. **N+1 graph reads.** Spans, entities and transclusions are each an edge
   list, then one `GetObject` per edge, plus one judgement read per
   citation in the overlay.
3. **Nothing is reused.** A published version is immutable (its manifest
   is hashed into the version), yet its bundle is rebuilt for each reader
   and each poll.

**An existing replacement is not wired in.** The computer's object graph
(`internal/objectgraph/dolt_store.go`) already serves metadata lookups
from indexed generated columns (`metadataColumnFor`, `metadataListSQL`).
Store B's platform wrapper never adopted them.

## Fix direction

1. **Measure (green to orange):** log each resolve step's time when the
   request is slow, so the split above is a finding rather than a guess.
2. **Reuse the immutable bundle (orange):** corpusd caches each version's
   bundle, everything but the live nav and check overlay, keyed by the
   version, and the route's version list, dropped when that publication
   publishes. Publishing warms the new version, so the first reader after
   a publish hits the cache too. The overlay keeps a short TTL.
3. **Poll cheaply (orange, frontend):** the reader's 60 s poll asks for
   the nav only.
4. **Index Store B's hot lookups (red, gated):** generated columns and
   indexes for the route path and the check's version id, adopting the
   object graph's existing mechanism. It is a Store B schema change, so
   it waits for the soak to end (about 2026-10-17 15:40).
