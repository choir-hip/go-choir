# Instruction text published as story titles, 2026-10-14

## Evidence

- Live front (GET https://choir.news/api/platform/publications/front?limit=100,
  read 14:50 UTC): 15 of 100 front titles are desk instructions, not
  headlines. Nine begin "Write the story of…" / "Write the story behind…"
  (first published 13:36:22, `pub-45f1a639…`, Cisco and Nvidia); six begin
  "Open a story on…" (first 10:17:46, `pub-d0cbc2ce…`, The Register).
- The paper (computer-1355b83b…) holds the same strings as the Texture
  **document title** (GET /api/texture/documents): ten of the newest 50
  documents are titled with an instruction, each cut at 160 runes
  ("…whether i", "…legal grounds for").

### Trace 1: Switzerland (parent management run)

- Run `108fea08-117d-4c5b-83b8-c8caf368fa53` (management, parent desk,
  created 13:31:03). Cell call `call_00_axbcv9qxgem29kotqovtiljh`
  (tool.invoked 13:34:21.83) ran five
  `choir.Cast("texture:new", "<objective>", spec)` calls. The objective
  argument was a writing brief: "Write the story of Switzerland proposing
  to ban profiling-based advertising aimed at minors: what the Swiss
  federal government has actually published, whether it is a proposal, a
  consultation or an adopted rule, what it would ban, and how the
  capture's claim stands against the primary document."
- Record `cell:108fea08…:call_00_axbcv9qxgem29kotqovtiljh:cast:tray-4`
  (committed 13:34:22.97) carries it as `directive.objective`. The commit
  opened doc `888c8135-f067-5a20-bd62-c8ab87980640` (created 13:34:23.056),
  titled with the objective cut to 160 runes.
- 13:34:34 the same run read `choir.Stories(8, "")`: the five new stories
  show the briefs as headlines. 13:34:46 it sent five Notes, e.g. record
  `…call_00_903utqh42nfo80rr6uw5428c:note:tray-4`: "Title correction: the
  working title for this story is \"Switzerland proposes banning
  profiling-based ads to minors\". The long line in the opening Cast was a
  writing brief, not a title". No act can change a document title, and
  Texture's prompt and verbs say nothing about titles, so the Note did nothing.
- Texture revision `d5a25fa4-f506-5692-9988-be691e88178c` (version 4,
  14:32:34.742) leads with its own headline, `# Switzerland plans to
  prohibit profiling-based ads to minors, but the ban is not yet law`.
  The title stayed the brief. Auto-publish sent it out as
  `pub-e7b6448d04734324d5cba5e5b6975914` at 14:32:35.58, titled
  "Write the story of Switzerland…".

### Trace 2: Mediaite (management task, S2a)

- Task `mt-0d418c466007af444a6d`, run `8e93efa5-300d-4944-b1cd-ebff3a9bd3b5`
  (created 13:58:47). Record `cell:8e93efa5…:chatcmpl-tool-bcc47a7c950c9b48:cast:tray-1`
  (committed 14:01:21.27, provenance `acting_task` mt-0d418c…, sender
  `management:<owner>`) has objective "Open a story on the Mediaite report
  that details leaked from a CBS News meeting … and how the leak fits the
  wider dispute over Weiss's editorial leadership."
- Doc `b3586842-4e4e-5afb-a5af-8af1dbfdc9d9` (created 14:01:21.269) took
  that objective as its title. Revision `7d6b6800…` (14:04:50.339) leads
  with `## CBS foreign editor reportedly warned staff of firing over
  insubordination`. The story was published 14:04:51.29 as
  `pub-5fe0bacd924b0fc06e32d8ed7da19619` under the instruction.

### Trace 3: Paul Christiano (parent management, before S2a)

- Run `03e5155b-3fa6-43d1-b43b-8fa7d7573870` (management, created
  10:11:26). One cell (10:15:45.82) loops over objective/hypotheses/capture
  triples and casts each to texture:new. Its objectives are "Open a story
  on Paul Christiano joining the OpenAI Foundation Board (OpenAI news post
  dated 9 September 2026)." and five siblings. The receipts name six docs.
  Doc `99e90aa5-d203-56ec-a48d-c698edb3236f` was created 10:15:46.997.
- Revision `aebfa162…` (version 2, 10:21:28.979) leads with `# Paul
  Christiano joins OpenAI Foundation board and safety committee`. It was
  published 10:21:29.68 as `pub-2beacfde5f94fc9f9acd94e15ed88067` under
  the instruction.

### Who wrote the instructions

The opening Casts of the ten instruction-titled documents in the newest
50 split as follows:

- 5 came from the parent management run 108fea08, at 13:34:22.
- 5 came from management tasks: mt-0d418c… (1), mt-c1c25007… (3) and
  mt-5bdd3d43… (1), 14:01 to 14:04.

The 10:15 batch was the parent desk, before S2a. So S2a did not cause
this; the model shapes the objective either way. The management prompts
already say "choir.Cast(\"texture:new\", headline, spec) … The headline
is the story's working title" (`internal/promptstore/defaults/management.yaml`,
`rlm_management_runtime.yaml`, `rlm_management_task_runtime.yaml`). The
model sometimes writes a headline and sometimes writes a brief.

## Cause

One string does two jobs, and nothing downstream replaces it.

```text
choir.Cast("texture:new", objective, spec)          internal/yaegikernel/choir.go Cast
  -> CommitmentRecord.Directive.Objective           (also Texture's work brief:
                                                     the Cast record Texture reads)
  -> store.planStoryOpenings / planStoryObjects     internal/store/story_open.go
       Document.Title = storyTitle(objective)       (whitespace-collapsed, 160 runes)
  -> Texture turns commit revisions                 internal/store/texture_turn.go
       commitTextureHeadAuthority: CurrentRevisionID, UpdatedAt; Title never touched
  -> AutoPublishRevision -> proxy publishTextureRevision
       platform.PublishTextureRequest.Title = doc.Title   internal/proxy/platform_publish.go
  -> publication object "title" (latest version) and manifest title
  -> front row title                                internal/platform/publication_front.go frontStory
```

1. **The opening objective is the title, permanently.** A Cast's objective
   is a work brief everywhere else: an engineering Cast, a research Cast,
   a feed Cast. Texture also reads this one as its brief. For a
   story-opening Cast the same string becomes the document title, and
   nothing in the system ever retitles a story.
2. **Texture writes a headline that goes nowhere.** In 24 of the newest
   50 stories, the current revision leads with a heading that is a real
   headline. That includes 9 of the 10 instruction-titled ones. The
   document title, the publish request and the front never read it.
   Texture is the sole writer of the story, but nothing it writes reaches
   the story's title.
3. **Publish has no headline check.** Auto-publish holds a revision that
   cites no source (`texturewrite.CitesSource`). It publishes a title it
   never checked came from the story.

This is a substrate cause, not a prompt slip. The management prompt
rule is already correct. Its only enforcement is the model obeying it,
and the field's other job (the work brief) pulls the other way.

## Sibling: working notes published as deks

- The front's dek is the first paragraph of the published body
  (`dekFromBodyDoc`). Two examples:
  - "Badenoch–Farage dispute over Chagos Islands and Trump"
    (`pub-104336fe…`, doc `eb488b52-a8c2-5260-a0fe-d8765620a17b`,
    revision `b7721049…` v1, 10:15:06): "This document will explain the
    dispute between Kemi Badenoch and Nigel Farage … The next step is to
    establish the chronology…".
  - "QuEra applies AI to quantum-computer laser locking" (`pub-30f36f6c…`):
    "This document is about QuEra Computing's use of AI…".
- Of the 25 headingless current revisions among the newest 50, many are
  V1 acknowledgments: "The owner wants…", "I'll build a careful
  account…", "You want an evidence-grounded account…", "The document
  will explain…".
- The mechanism is a sibling, not the same field. Texture's prompt
  (`internal/textureprompts/texture.yaml`, "Revision shape") makes V1 "a
  fast acknowledgment: a few sentences on what the owner wants, the
  plan". That is right for an owner's document. On a publish-auto story,
  any V1 that cites a source publishes, so the plan becomes the story's
  first version and its dek. What they share with the title problem:
  work-facing text reaches a reader-facing surface with no story-shaped
  gate in between. The trace to compare (the Russia/Ukraine drones dek
  the lead named) is not in the newest 50 documents and was not read
  here; the Badenoch V1 above is read.

## Belief state

- Confirmed from the ledger: the title is `Directive.Objective` from the
  opening Cast, stored once by `planStoryObjects` and published through
  `doc.Title`. Texture's headline sits in the body and is ignored.
- Confirmed: the model put the brief in the objective. The prompts did
  not ask for one. Both desk shapes did it (parent and task).
- Not established: why the model chose a brief on those turns. The
  hypothesis is that the objective is also the brief Texture reads. The
  fix below does not depend on it.

## Remaining error field

- Fix (separate commit): Texture titles the story. A story revision that
  leads with a heading sets the document title in the same commit. A
  publish-auto story revision with no leading heading is held, like one
  that cites no source. Texture's prompt states the headline rule.
- Existing leaked stories: published versions are immutable and keep
  their manifest titles. The publication's latest title and the front row
  update when a story's next heading-led revision publishes. The route
  slug was fixed by v1 ("write-the-story-of-…"); links never change, so
  it stays.
- Dek sibling: the headline hold also holds the headingless V1
  acknowledgments. An acknowledgment written under a heading still
  publishes. The full fix is a story V1 that is already news, and it
  needs its own decision.
