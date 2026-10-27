# Related stories do not show on the paper: the entity record is refused by the vocabulary write guard (2026-10-27)

Class: green (diagnosis only; no code changed). Read-only: GETs against
https://choir.news with the paper key, plus `choir trace export`. Nothing was
POSTed to the paper; related-review, heal and the replay report were not run.
Observed 2026-10-27 02:20 to 02:55 UTC; paper build `6df2cbd92` (deployed 02:02:20).

Owner report (~02:18): the new story "Iran and pact members hold Tehran talks,
agree to continue consultations" (v8, 4 sources) shows no related stories; the
top story shows one. "If the backfill's running, are we running it live now? I
still don't trust it."

## Answer in one paragraph

No entity has ever been attached to a story on the paper, live or backfilled.
Every `choir.Relate` call that names at least one entity fails on the guest
when it appends its `story.entities` event to the tape: the store's vocabulary
write guard reads `payload.attachments[0].role = "central"` as a desk role and
refuses it ("not in v2 vocabulary"). Nothing reaches corpusd, so no
`mentions_entity` edge exists and the entity read has nothing to join. The only
calls that land are the empty ones (`choir.Relate("", nil)`, "about none of its
candidates"), which carry no `role` field: 183 such reviews, 0 attachments.
The backfill's 202 receipts are real asks, but each ask is a cast that waits in
a saturated dispatcher and ends in the same refusal (or is passivated by a
restart first). The 202 says "asked", nothing more.

## Evidence

### 1. The Iran story and the top story

- Related reads (`GET /api/platform/publications/related?route=<bare>&limit=6`,
  the route the Reader uses; frontend `texture-related-stories.ts`):
  - Iran story (`/pub/texture/iran-and-pact-members-...-pubda66f9e05`, rev 8):
    `links: []`, `basis {captures: 4, mode: "captures+relations+entities"}`,
    `partial: false`.
  - Top story now (UN inquiry, West Bank, rev 4): `links: []`, same basis. The
    owner saw one link on the top story earlier; at 02:20 the top story had
    none. The front changes with each publish, so the owner's top story was
    probably a different one. Not resolved.
- The mode string shows both read switches are on in corpusd
  (`CORPUSD_RELATED_STORY=on` in nix/node-b.nix; `CORPUSD_RELATED_ENTITIES`
  default on). The read is not switched off.
- No public per-story entity read exists. The growth read
  (`GET /api/platform/entities/growth?days=30`) is the entity census:
  `lexicon {entities: 0, keys: 0, stopwords: 2}`. Zero entity nodes exist in
  Store B for the paper owner, so no story has an entity. Days with reviews:
  10-26: 90 reviews, 604 candidates offered, attachments 0 (hits, existing,
  extracted, research_named all 0), minted 0. 10-27 (to 02:20): 93 reviews,
  606 candidates, all attachment counts 0. Every earlier day: 0.
- Coverage over the 100 newest front stories (each read through the related
  route): 18 have a link, 0 by reason `entity`, 0 by judged relation. Reasons:
  capture 3, capture+capture 4, source 8, capture+source 1, and two mixed. The
  problem doc `related-stories-no-entity-layer-2026-10-26.md` measured 13 of
  the same 100 early on 10-26 (4 of the first 60). Today's first 60: 12. The
  rise is the capture walk finding more shared captures, not the entity layer.

### 2. Did the live path fire for the Iran story

Trace: `choir trace export 97a591be-0575-58ff-8b3c-7ec96778c823` (doc
`83304189-5cdc-5fee-8a37-4c61a5ecdace`), 4.3 MB bundle, scratchpad copy only.

- The story's research desk ran twice. Run `1f5ff19b` (17:59 to 18:04 on 10-26)
  was before train 20 (about 19:30), so it had neither the preloaded analysis
  nor the entities finish rule. Run `d3aac772` (20:33:00) carries the new
  prompt ("Before your final Report, record what this story is about ...") and
  is the backfill ask `related-review:rr-2026-10-26-3:83304189-...` (cast
  20:24:48). It was **passivated at 20:33:54, `recovery: passivated_on_restart`**,
  with `provider_admission_refusal: "Research agent does not own the exact live
  run"`, and has stayed `passivated` since (updated 20:40:54).
- The research inbox is `head 2, cursor 1`: the backfill record (seq 2) has
  never been read; the commitment record is `discrepancy: unresolved`.
- Texture revised the story seven more times (v2 to v8, last 01:09:15) without
  waking research again. After 20:33 the live path never had a research run for
  this story. There is no `story.entities` event, no System 1 call, no related
  computation and no error for the Iran story in the bundle. It simply has not
  run since the deploy, so the owner's "brand-new" v8 never had its entities
  asked, on either path.

### 3. The refusal, seen on the same guest (boot layer `recent_log`, 02:35 to 02:51)

18 lines in 16 minutes, 11 distinct stories, e.g.

```
story entities: record pub-8491aa7b... on run 4413ff18-...: store: create object:
vocab write guard: vocabfence: og:choir.event:body.payload.attachments[0].role="central"
not in v2 vocabulary
```

(also `role="incidental"`). Same run retries 2 to 3 times, 8 to 13 seconds apart.
The same log shows `closed round 1 ... ended without a final Report` (3 lines)
and the dispatcher at `slots=8/8 due_agents=94..114`, activation waits of 14 to
15 minutes, `obligations`: runs passivated 48, pending 6, running 8; actors due
814 (813 `cell_terminal_deadline`, 77 `inbox_record`).

### 4. Code path

- `internal/agentcore/story_entities_verb.go`: `Relate` appends
  `storyEntitiesRecord{Attachments: [{EntityID, Role: "central"|"incidental", ...}]}`
  as a `story.entities` `choir.event` on the system1 channel
  (`appendStoryEntitiesEvent`). On failure it logs `story entities: record %s on
  run %s: %v` and returns "the attachments could not be recorded on the tape, so
  they are not exported". The export to corpusd (`PublishEntities`) comes after
  the append, so it never runs.
- `internal/store/vocab_migrate_og.go`: `vocabWriteGuard` (active after the
  cutover report exists) walks every `choir.event` body for leaves named in
  `ogRoleLeafKeys` (`role`, `profile`, ...). `ogRoleSkipKinds["choir.event"]`
  exempts only `kind` and `phase`. A `role` leaf at `payload.attachments[N].role`
  is checked against the desk vocabulary (`management`, `research`, ...), and
  `central` and `incidental` are not desk roles. Same class as
  `s2a-management-tasks-refused-by-vocab-fence-2026-10-14.md` and
  `run-acceptance-refused-by-vocab-fence-on-multi-desk-trajectory-2026-10-22.md`.
- Why the tests did not see it: the guard is a no-op until `vocabCutover` is set,
  and the slice a to e tests run on stores that never cut over.
- An empty list appends a record with `attachments` empty and a review that has
  no `role` leaf, so it passes. That is the whole population on corpusd.

## What the backfill actually does

`POST /api/texture/related-review` (`textureowner/texture_related_review.go`) calls
`Runtime.StoryEntitiesBackfillFrom` (`agentcore/story_entities_backfill.go`):

1. Refuses (409) if `research.related_story` is off for the computer.
2. Reads the tape for stories with a `story.entities` `recorded` event.
3. Lists story documents newest first from the cursor; for each, in order:
   already asked in this batch id (replay, a no-op), recorded on the tape
   (`recorded`), head text empty (`empty`), research desk has an active run
   (`busy`), otherwise commits a `StoryResearchRequest` of kind `entities`
   (`CommitStoryResearchRequest`): a commitment record `related-review:<batch>:<doc>`
   and an owner cast `directive` into `research:<doc>`'s inbox. That is all it
   does synchronously. The story is in `requested`.
4. Returns `Requested`, `Replayed`, `Recorded`, `Busy`, `Empty`, `Gone`,
   `Scanned`, `Next`. HTTP 202 for any success, whatever happens next.

So `{"n":12,"busy":0,"recorded":0,"empty":0}` means 12 casts were committed.
`recorded: 0` means: none of the 12 had a `recorded` `story.entities` event on
this computer's tape. That is accurate and, given section 3, expected: a
recorded event with attachments cannot be written. It says nothing about
outcomes. The asks go to the research inbox, wait for a dispatcher slot (8
slots, 100+ due agents), run a cell, and end at `Relate`. With the vocab
refusal the best outcome is the empty fallback. A restart passivates the run
(Iran's did). A passivated or interrupted ask is not re-dispatched (crash
policy; and a planned restart resumes only runs the marker covers). The receipt
has no completion read; the only completion evidence is corpusd's review count,
which counts empty reviews too.

## Ranked causes (marked)

1. **Established (trace and guest log, code read):** the vocab write guard
   refuses every attachment-bearing `story.entities` event. Entity layer
   receives nothing. This alone explains zero entity links on all stories,
   live and backfilled.
2. **Established for the Iran story, hypothesis elsewhere:** asks and live
   research runs are slow and lossy. Dispatcher saturated (activation waits 15
   min), restarts passivate runs (`passivated_on_restart`, 48 passivated), the
   Iran backfill ask died that way and was never re-asked. After fix 1, a
   second pass is needed for every story whose ask was lost.
3. **Hypothesis (no per-run trace read):** the 183 empty reviews are the desk's
   fallback to `Relate("", nil)` after the attachment call failed, because the
   finish rule blocks the final Report until something is recorded. A trace of
   one such run would confirm it. The alternative (System 1 judged all
   candidates not_about) is not consistent with a role that fails identically
   for `central` and `incidental` in every logged line.
4. **Hypothesis:** the finish rule plus a refused record keeps research desks
   looping ("tool loop: iteration 74 to 77" in the log; "ended without a final
   Report" x3) and adds to the dispatcher backlog. Unmeasured.
5. **Ruled out:** corpusd kill switches (mode shows `+entities`), Store B write
   path (separate store without the guard), the Reader's related fetch.

## Smallest fixes (not done; each needs its own record and mutation class)

1. Orange/red (store guard): change `ogRoleSkipKinds["choir.event"]` to also skip
   `role` under `payload.attachments`, or rename the tape field in
   `storyEntityAttachment` (`entity_role`; no attachment-bearing record exists
   on any tape yet, so no compatibility cost), and add a test that runs
   `Relate` with a cut-over store (`vocabCutover` set) so the guard is in the
   loop. Rename is the smaller blast radius: it leaves the guard untouched.
2. After 1 deploys: run one backfill batch of 1 to 3 stories and verify through
   `GET /api/platform/entities/growth` (lexicon entities greater than 0) and
   the related route for a pair that shares an entity. The 202 and the counters
   are not evidence.
3. Make the receipt honest: add to the act's read a per-batch outcome (asks
   open / recorded with attachments / recorded empty / passivated), so "queued"
   is not inferred from `n - busy - recorded - empty`.
4. Re-ask the passivated asks (Iran's `rr-2026-10-26-3`) with a new batch id once
   the dispatcher backlog drains; do not rely on the old ones resuming.
5. Surface `story entities: record ... vocab write guard` as a counted
   metric or a growth-read field ("refused records today"), since the growth
   series showed 90 reviews with 0 attachments for a day and no one read that
   as failure.

## Caveats

- The paper guest was intermittently unreachable during this read (host reported
  `computer_waking_failed ... degraded` several times; `GET /api/trajectories`
  timed out repeatedly, and a 150 s list may have added load). The trace was
  obtained on a later retry. The planned restart marker in the bundle
  (`platform update`, target `6df2cbd92`) matches the 02:02 deploy.
- Hypotheses 3 and 4 need one run trace of a research desk that called
  `Relate` (the log names runs `4413ff18`, `7e1104a1`, `bcf82095`).
- Heresy delta: discovered 1 (the guard refuses the entity record), introduced 0,
  repaired 0.

## Fixed by (lane/entity-role)

Smallest fix 1, rename option: the `story.entities` tape attachment's `Role`
field is now `entity_role` (`storyEntityAttachment` in
`internal/agentcore/story_entities_verb.go`). The vocabulary write guard is
unchanged. Every reader of the tape record decodes through the same struct
(`story_entities_verb.go`, `story_entities_source.go`,
`story_entities_backfill.go`), so none needed a separate edit; the desk-facing
`choir.Relate` result and the corpusd wire (`types.StoryEntity`) keep `role`
(neither is a tape event). No migration: no attachment-bearing record was ever
stored, and a stored record with `role` could not exist. A reader of an old
record would see an empty role, not an error. Tests: a cut-over store accepts
the event and reads it back; a bare `role=central` and `role=boss` stay
refused (`internal/store/story_entities_vocab_fence_test.go`,
`TestRelateRecordsOnAStoreThatHasCutOver`). Smallest fixes 2 to 5 remain open
(backfill re-ask after deploy, honest receipt, refusal metric).
