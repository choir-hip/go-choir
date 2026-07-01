# Parallax: Diagnose Universal Wire and Design the Web Capture Object

## Status

Open. Not yet started.

## Mission conjecture

If we diagnose why `/api/universal-wire/stories` returns HTTP 502 and define the `choir.web_capture` object kind, then we will have a path to a working Universal Wire feed and the first shared news object in the graph.

## Deeper goal

Universal Wire is the first proof that the object graph can process and publish information objects autonomously. The deeper goal is to make the news pipeline a graph-native flow: ingestion creates web capture objects, processors turn them into texture documents, and the feed is a query over the graph. The diagnosis is the first step.

## Witness / spec

Deliver a diagnosis and design document with:

- The exact failure mode of `/api/universal-wire/stories`: request path, response code, timing, logs, upstream status.
- Identification of whether the 502 is a proxy timeout, an upstream crash, a database issue, or a missing service.
- A design for the `choir.web_capture` object kind: URL, canonical URL, title, fetched_at, content_blob_id, extracted_text_blob_id, embedding model/version.
- A feed query design for Universal Wire over web capture objects.
- A plan for how ingestion writes web capture objects and how the processor reads them.
- A verification plan: a test that writes a web capture object and retrieves it through the feed.

## Invariants / qualities / domain ramp

- Do not change production routes unless the fix is a one-line configuration change.
- The web capture object must be durable and citeable.
- The feed query must be a graph query, not a bespoke pipeline.
- Preserve the existing source-cycle infrastructure where possible.

## Authority / bounds

- Orange mutation class: runtime behavior and platform routes, but the diagnosis phase is yellow.
- No production deploy during diagnosis unless a safe fix is found.
- Branch: `diagnose/universal-wire-502`.
- Worktree: `wire-diagnose`.

## Bridge conjecture + sub-conjectures

- Main conjecture: Universal Wire is broken because it lacks a shared news object graph; the 502 is the symptom of that missing substrate.
- Sub-conjecture 1: `/api/universal-wire/stories` returns 502 because the upstream service times out or is misconfigured.
- Sub-conjecture 2: defining `choir.web_capture` will allow the feed to be a simple graph query.
- Sub-conjecture 3: the existing sourcecycled pipeline can be adapted to write web capture objects.

## Ledger / move log

- Move 0: Read the source-cycle and Wire docs.
- Move 1: Reproduce the `/api/universal-wire/stories` 502 on staging.
- Move 2: Capture proxy, service, and database logs.
- Move 3: Identify the root cause of the 502.
- Move 4: Design the `choir.web_capture` object kind.
- Move 5: Design the feed query.
- Move 6: Write a verification plan.
- Move 7: Commit the diagnosis and design document.

## Version / lineage

- Predecessor: `@/Users/wiz/go-choir/docs/design-self-developing-software-2026-06-23.md` and the mission-wire docs.
- Successor link: this work feeds into the web-capture object implementation and the Universal Wire feed rewrite.

## Learning state

- Retained: the exact failure mode of the Universal Wire route and the web capture schema.
- Promoted outward: the feed query design and the web capture object kind.

## Settlement

Done when the 502 root cause is identified and the web capture object and feed query are designed with a verification plan.
