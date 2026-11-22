# The deployed story research context loses its main point under the enforced reader gate

Recorded 2026-11-22, found by the second seat's review of the graph-first change
(its proof: the digest, which read the story's document as the research desk, lost its
story section under enforce) and confirmed here on the code that is on main since `8e9489ac2`
("a story's research continues with what it holds", committed 2026-11-21 16:22 -0500, which
added both `StoryPointOf` and the context that calls it). The release that carried it was
deployed with the main-point judgment of the same day (ledger C189 reads that deploy); the
deploy time of that release is not recorded in this repository, so the exact hour is not
stated here.

## Problem

The story-research context a research run on a story is given
(`internal/agentcore/story_research_context.go`, `storyResearchContextOpening`) took the
story's main point with `store.StoryPointOf(ctx, ...)` under `withDeskReader(ctx, research)`.
`StoryPointOf` reads the story's Texture document (`GetLifecycleDocument`) to find its
trajectory. The object-graph reader gate (X1: "research cannot read a texture document",
`internal/objectgraph/reader_gate_test.go`) ENFORCES as the product ships it
(`objectgraph/writer.go`, `reader.go`: the mode starts at enforce); only this package's
tests run in observe (`TestMain` in `tools_desk_test.go`). So in production the read is
refused, `StoryPointOf` returns "", and the run's context carries no main point line.

## Evidence

`internal/agentcore/research_digest_authority_test.go`
`TestUnderEnforceAResearchReaderGetsNoMainPointAndNoDocument`, with the gate set to enforce:
the custodian reads the document, the revision and the main point; the research reader's
`GetLifecycleDocument` is refused and `StoryPointOf` returns "" (the test logs it). In
observe (the package default) the same read works, which is why no existing test saw it:
`TestResearchRunOnAStorySeesEarlierReadsFedCapturesAndPredecessor` asserted the main point
and passed in observe.

## Belief

The deployed context has been missing its main point (and the successor's: `succeeds`
comes from the same read) in production since `8e9489ac2` was deployed; the pages, the fed captures and
the outlets (inbox records and captures, not documents) are unaffected: the digest's
enforce test reads them as the research desk and they arrive. Nothing was written wrongly;
a section was shorter than designed.

## Fix

`graph-first` removes the context and the read. The story's text, main point and
predecessor reach research inside the Texture's own Cast (`CommitmentCast.Story`, written
by the Texture's turn commit: `store.attachStoryToResearchCasts`); the digest reads them
from the conversation records. Its tests run with the gate in enforce
(`buildDigest`/`buildBrief` in `research_digest_test.go`).

## Remaining error

Whether the production reader gate refuses for the built-in Texture kinds exactly as the
enforce-mode test does is read from the gate's code and its tests, not from a counter on the
paper: `scripts/status-*` has no reader-gate line. A Cast written before the field, or by a
path other than the Texture's turn commit, carries no story: the digest then has none.
Other research-context readers of Texture's kinds (if any) are not audited here.
