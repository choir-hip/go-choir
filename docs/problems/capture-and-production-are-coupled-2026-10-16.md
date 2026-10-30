# Capture and article production are coupled: one source publishes, and model calls scale with polls

Found 2026-10-16 about 00:10 UTC. It surfaced while acting on owner §24
and §25 (docs/reports/owner-notes-2026-10-14.md): two sources minimum,
ingestion scale-up without more model calls, and separating capture from
production. The evidence is a code survey at 6c0368219. File and line
references are from that commit.

## The problem

Capture is already free of model calls:
- poll (`agentcore/source_schedule.go:150`, `researchtools/poll_source.go:139`);
- exact-content dedup;
- the Store B write (`sourcegraph/captures.go:134`);
- the `CaptureArrival` announcement (`agentcore/capture_arrival.go:104`).

Everything after the arrival is a desk run, and nothing stands between
one capture and a published article.

1. **One source publishes.** `AutoPublishRevision`
   (`agentcore/story_publish.go:168`) holds a revision only when:
   - it is the seed;
   - it cites no captured source (`texturewrite.CitesSource`, which is
     true on a single `source_ref`);
   - or it has no leading headline.

   Nothing counts distinct sources or outlets. "Two independent reports
   (different publishers)" is prompt advice in the Texture overlay, not a
   check. `source_count` is used only to rank the front page, after
   publication. This contradicts owner §24: "We always should have two,
   ideally more."
2. **Model calls scale with polls, not with stories.** Each poll batch is
   one `CaptureArrival` record. Each record wakes a management run and
   spawns one intake task (`choir.Delegate("intake", …, "per_record")`).
   The optional Jev label (`intake.labels`, default off) is one call per
   capture. Activating more of the 217 registry sources (28 polled today)
   multiplies desk runs. This contradicts owner §24: "Increasing source
   ingestion shouldn't imply significantly increasing the quantity of
   model calls."
3. **No intermediate representation sits between captures and articles.**
   No event, claim, entity or outlet object exists. "Outlet" is a text
   field. A story is opened by a desk's judgment on the capture in front
   of it, so a capture never waits for a corroborating second capture.
   The ontology design (docs/reports/claims-events-entities-ontology-2026-10-14.md)
   is unbuilt. Owner §25 makes it the overnight priority, and says it
   must be retrieved from first.
4. **Retrieval cannot see the world model, because there is none.**
   `SearchWireCorpus` is a keyword match over the titles of the newest
   1,000 captures, about 5 hours
   (docs/archive/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md).

## Classification

- **Items 2 and 3 are substrate.** The pipeline's unit of work is the poll
  batch, where it should be the event. Patching prompts cannot fix that.
- **Item 1 is a missing gate.** It can be fixed on its own and should be
  fixed first, because the owner ruled on it directly.

## Existing parts to build on, not replace

- **Jev same-event labels** (`agentcore/intake_labels.go`, S1a,
  flag-gated). They already judge "same event as story X or sibling
  capture Y" per capture. Owner §25 allows Jev at ingestion, so this is
  the seed of deterministic-plus-Jev event clustering with no desk run.
- **`land/store-b-retrieval-rebased`** (ba12a1e0c): title postings and a
  capture index. It lands after the Dolt soak, about 2026-10-17. It is the
  candidate-shortlist source for clustering.
- **`frontimportance` depth/`source_count`**: it already counts sources
  for ranking.

## Fix direction (design to follow, panel-reviewed)

- **F1, the publish gate (first).** A publish-auto revision is held until
  its citations span at least two distinct outlets. The outlet is the
  publisher's registrable host of the cited capture. Distinct outlets are
  a floor, not proof of independence: wire copy syndicated across outlets
  is a known residual for the world-model layer.
- **F2, event clusters at ingestion.** Each capture joins or opens an
  event cluster using a deterministic shortlist plus one Jev choice. No
  desk runs. A cluster records its captures, outlets and first and last
  arrival.
- **F3, production per cluster.** Management or Texture wakes when a
  cluster first reaches at least two distinct outlets, or when it grows
  under an existing story. It does not wake per poll batch. Desk runs then
  scale with events, not captures.
- **F4, retrieval from clusters first.** Researchers query the clusters
  ("what is known about X, from whom, since when") before captures and
  the web.

## Ways the publish gate could fail (tests are written against these)

- **G1:** a revision citing two captures from the same outlet (different
  URLs or subdomains of one publisher) publishes.
- **G2:** a revision citing two outlets is held.
- **G3:** a citation that cannot be resolved to a capture counts as an
  outlet. Only resolved captures count, and an unreadable store holds the
  revision; it never publishes.
- **G4:** the hold text does not tell Texture what is missing. It must
  name the outlet count, so the next revision can fix it.
- **G5:** a story already published under the old rule is retracted or
  broken. The gate holds new revisions only; published versions stand.

## Mutation class

- **F1** is orange: the publishing gate on the runtime path. Rollback is a
  revert.
- **F2–F4** are red where they touch Store B writes and the frozen
  inbox declarations. They need the full ceremony.

## Found after F1 landed: aggregators and syndicators count as outlets

An offline event study on 2026-10-16, about 00:20 UTC, replayed 500 real
paper captures (about 4 hours, 184 outlets) through batched Jev event
resolution. The script and outputs are in the session scratchpad,
outside the repo.

Of the 36 events that reached two or more "outlets", four were an
article plus its Hacker News thread (`news.ycombinator.com`). Under the
d54242fc9 key, the following also count as outlets:
- syndicators that republish other outlets' copy (Yahoo News carrying a
  Reuters story, AOL);
- search pages (google.com/search);
- reader-proxy captures (`r.jina.ai/<url>`, named after the proxy
  instead of the publisher).

So the F1 gate can pass a story whose second "source" is only a link to,
or a copy of, the first. The fix: aggregators, search and syndicators
name no outlet, and proxies are unwrapped (O5, O6 in
internal/outlet/outlet_test.go).

Distinct outlets still do not prove independence: wire copy carried by
two originating outlets reads as two. Wire-origin detection (wire tags
plus lead-block overlap; panel Gemini, round 1) belongs to the event
layer's V1.
