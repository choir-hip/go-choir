# Texture's spec follows the owner's later directions (2026-10-25)

Problem record: docs/evidence/selfdev-trace-reviews-2026-10-25.md, Round 2
section 3, and the ledger 03:1x entry in
docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.ledger.md.
Mutation class: red (the input to Texture's canonical writes). Tier 1
review: additive, behind a computer setting, one-act rollback.

## Cause

Evidence: computer b@c, trajectory e3e37430, document db3febbc, Texture run
230c5a4e (03:04:42 to 03:05:01), revision ee41afb6.

1. Trigger (finding, trace and code). Engineer b6628's `choir.Report` at
   03:04:42 minted the progress relay record
   `progress:cell:run:assignment-b6628...:report:tray-1` (sender
   `system:progress-relay`, setting `engineering.progress_to_texture`) into
   Texture's inbox. The Texture controller started a run for unread records:
   intent `integrate_execution_findings`, `request_source=inbox_record`
   (texture_controller.go, the `recordsPending` branch).
2. Prompt source (finding, trace and code). `submitTextureAgentRevisionRun`
   builds the first user turn with `buildAgentRevisionRequest` from the
   metadata of the document's head revision only. The head was ac2623cc, an
   appagent revision. Its metadata carries `seed_prompt` forward
   (`durableMetadataKeys`), so the turn printed "The owner's request:" and the
   original 7075-byte spec (prompt_length 7243 = the wake line plus that
   spec). The owner's 02:59 direction lives only as `owner_prompt` on the user
   revision 31350811. `owner_prompt` is per-revision (not durable), and the
   turn prints it only when the head is that user revision. One appagent
   revision after an owner revise, the owner's direction is gone from the
   prompt and the original request stands alone, labelled as the owner's
   request.
3. No transcript carries it either (finding, code). The run records
   `actor_rewarm_source_loop_id` (dc8b0e5d, the run that did see the
   direction), but nothing reads that key. The only other trace of the
   direction was Texture's own prose in block b2 of ac2623cc ("retain
   PdfApp.svelte and EpubApp.svelte, make Reader the default"). The model
   preferred the labelled "owner's request" over its own prose (hypothesis
   on how the model weighed the two; the inputs are findings).
4. The write (finding, revision metadata). ee41afb6 was written by
   `texture_cell` through the canonical writer with
   `texture_write_base_revision_id` = ac2623cc, the head at the time. Its
   parent context was right. It is a correct revision of stale input, not a
   writer race. Run 9ecdb2c6 (e83d9255, 03:11:04) repeated it: "remove legacy
   static IDs and routes".
5. Downstream (finding, trace). Management's 03:05:22 Cast took Texture's
   cast record from 230c5a4e (the original spec, sent "verbatim") as its
   parent, so assignment 13d84 got the delete spec and a restore objective at
   once.

Existing replacement check: nothing projects "the owner's latest
directives". `owner_prompt` is read in one place
(`buildAgentRevisionRequest`), and only from the head. `textureprompts/texture.yaml`
says nothing about later directions overriding the request. So this is a
missing projection, not an unwired one.

## Fix

Read side only. No write path, revision schema or metadata key changes.

- `textureOwnerDirections` walks the head's parent chain (at most 64
  revisions, reusing the already loaded parent) and collects every user
  revision whose metadata has a non-empty `owner_prompt`, newest 8 kept, each
  capped at 4000 characters, returned oldest first with revision id and time.
  The head itself is excluded (it already prints as "The owner's direction
  for this revision").
- `buildAgentRevisionRequest`, when there are directions or a head direction:
  labels the seed "The owner's original request (the owner's later directions
  below override it wherever they differ)", then lists the earlier directions
  in order under a rule that the newest governs, that the document and every
  cast must follow it, and that a revision must never undo an owner direction
  unless a later owner direction says so. With no directions the turn is
  byte-identical to today.
- Behind a new computer setting `texture.owner_directions` (off by default,
  the house pattern). Off: the walk never runs and the turn is unchanged.

## Failure modes, written before the code

1. An appagent head after an owner revise drops the owner direction (the
   defect). Test: a head appagent revision whose parent is a user revision
   with `owner_prompt` must print that direction after the seed.
2. Directions print out of order, so an older one reads as current. Test: two
   directions print oldest first, and the newest is last.
3. The seed still reads as the owner's whole request when a later direction
   exists. Test: the label changes and the override sentence is present.
4. The head's own direction prints twice. Test: it appears exactly once.
5. With no directions the turn changes (a regression for every other
   document). Test: same output as the builder with no directions.
6. The walk loops on a cycle or runs unbounded on a long story. Test: a
   cycle and a chain longer than the bound both terminate within the bound.
7. A missing parent (deleted or out of scope) fails the run. Test: the walk
   stops and returns what it found; the run starts.
8. A huge direction bloats the prompt. Cap per direction and count.
9. Switch off must not walk. Test: the reader is not called when off.

## Not fixed here (residuals)

- A mechanical guard that refuses an appagent revision contradicting an owner
  direction is not possible without semantic judgment; the fix gives the
  model the right input and the rule. Whether the model then holds to it is
  a staging question (next probe: a revise, then a progress relay wake, and
  read the next revision and cast).
- Management casting while its Note answer is pending (Round 2 item 3) is a
  management defect, separate.
- `actor_rewarm_source_loop_id` is written and never read; recorded, not
  touched.

## Rollback

Live: set `texture.owner_directions` off on the computer (owner act, no
deploy). Code: revert the one fix commit.

## Heresy delta

- discovered: the Texture request turn reads owner intent from the head
  revision's metadata only, so a per-revision owner directive is lost after
  one agent revision while the original seed is carried forever as "the
  owner's request"; `actor_rewarm_source_loop_id` is a write-only key.
- introduced: none intended (one bounded read-only walk per Texture run
  start when the switch is on).
- repaired: the owner's later directions reach every Texture turn, in order,
  ranked above the original request (when the switch is on).
