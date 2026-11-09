# The judge's served model and story rarity are never recorded (2026-11-09)

Mutation class of the fix: orange. Found in the live paper computer's log on
build 94c3609ab, on every occurrence of both lines. Listed as defects (a) and
(b) in `docs/definitions/choir-autopaper-improvement-2026-11-09.ledger.md`.

## Problem (a): the served model of a Jev call is never recorded

Log line, after every decision-model (typesafe/jev) call:

    support judge: record served model on run <id>: store: create object:
    vocab write guard: vocabfence: og:choir.event:body.payload.role="system1"
    not in v2 vocabulary

Root cause:

- `recordJevCostFor` (internal/agentcore/support_judge.go:399-410) builds a
  `model.served` event whose payload is a `ModelServedRecord`. It sets
  `served.Role = types.ModelServedRoleSystem1` ("system1",
  internal/types/model_served.go:27).
- That field is a model-provenance role, not a desk role. The payload lands in
  the body of a `choir.event` object as `payload.role`.
- The store's vocabulary write guard (`vocabWriteGuard`,
  internal/store/vocab_migrate_og.go:946-962) collects every leaf named `role`
  and the like (`ogRoleLeafKeys`, line 821) and checks it against the v2 desk
  vocabulary (`v2LiveSet`, internal/vocabmigrate/fence.go:40). "system1" is
  not a desk, so the write is refused.
- The skip list `ogRoleSkipKinds` (line 834) already excuses `role` on
  `choir.run_memory_entry` (message roles). It has no entry for this payload.

Why tests did not catch it: `TestJevServedLineTakesTheProviderFromTheCall`
(internal/agentcore/model_provenance_test.go:277) and its neighbours run on
`testRuntime`, whose store never sets `vocabCutover`, so the guard is inactive
(`vocabWriteGuard` returns nil before the cutover). The guard is only tested
on its own, in internal/store.

Since when: `ModelServedRoleSystem1` and the write came in with
dab4b7f8c on 2026-11-05 ("model provenance from what served the runs"). The
guard has existed since 1e96e0b2a (2026-09-10). So every Jev call since
2026-11-05 on a post-cutover computer has lost its `model.served` line. The
cost event (`run.progress`, phase judgment) is written separately and was not
refused, so spend is recorded; only the model provenance of the judge is
missing. Texture's source panel and the published reader that show the
decision model have shown none for those calls.

A second hazard: the boot fence and the replay fence read the same leaves
(`servingOGRoleFields`, `vocab_replay_fence.go:220`). If the write were let
through without teaching the leaf walker, the next boot fence would refuse the
stored rows.

Fix options:

1. Teach the leaf walker that `payload.role` of a `model.served` event is not a
   desk role (recognised by the payload's `schema`, `choir.model_served.v1`),
   so the write guard, the boot fence and the replay fence agree. About 15
   lines, no vocabulary change. Chosen.
2. Add "system1" to `v2LiveSet`. Wrong: it would make "system1" a valid desk
   role everywhere (agent objects, acceptances).
3. Rename the payload field. A schema change for stored lines and readers;
   not needed.

## Problem (b): story rarity (IDF) is never computed

Log line:

    story analysis: <doc>: library frequencies: objectgraph: wrong reader for
    kind: kind "choir.texture_document" is kept by "desk:texture"; the read
    declares "desk:research" (rarity unknown)

Root cause: `buildStoryAnalysis` (internal/agentcore/story_analysis.go:236)
calls `rt.store.StoryLibraryIDF(ctx, owner)` on the research desk's context.
`StoryLibraryIDF` (internal/store/search_index.go:303) lists the owner's Texture
documents (`searchableTextureDocuments`) and reads their heads. Those are
`choir.texture_document` and `choir.texture_revision`, kept by desk:texture.
The reader gate (internal/objectgraph/reader.go, enforcing) refuses a read
declared as desk:research. The function logs "(rarity unknown)" and sets
`idf = nil`, so `entitytext.Propose` gets no rarity.

The precedent is in the same function: its two document reads, at lines
211 and 221, use `objectgraph.WithCustodianReader(ctx, "choir.texture_document")`
and `"choir.texture_revision"`, with the comment "the host builds the analysis
... on the research desk's behalf: Texture's kinds, read as their custodian (a
forced custodian read; S3 review B3)". `researchDeskDoc`
(story_entities_verb.go:335) does the same. The IDF read was added in
12156a88c (2026-10-26) and never declared a reader. It has been refused since
the reader gate began counting every read (89db770e9, 2026-11-05) and
enforcing; before that it was only uncounted. The IDF is a host-computed
scalar function over word counts; no cell reads the documents. That is exactly
the case `WithCustodianReader` documents (custody.go:171).

Why tests did not catch it: the story analysis tests call with
`context.Background()`, which declares no reader, and an undeclared read of a
desk kind is admitted in that path. No test ran the analysis under the research
desk's context.

Effect: since the gate enforced, every story analysis has proposed entities
with no rarity, so the ranking of entity candidates that depends on rarity has
run blind.

Fix options:

1. Follow the precedent: read the IDF with
   `WithCustodianReader(ctx, "choir.texture_document")`. One line. Chosen.
2. Ask the Texture custodian for the counts through a desk message. More
   machinery for a host-side scalar; not the precedent in this file.
3. Add research to the supplemental readers of Texture's kinds. Rejected:
   it widens every cell-reachable read, which the gate exists to show.
