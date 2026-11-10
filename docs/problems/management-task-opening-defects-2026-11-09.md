# The management task opening: review defects and failure list (2026-11-09)

Status: open until the repair lands. Mutation class of this doc: green.
Evidence: review of commit `09884e328` (branch `integrate-1109`), recorded in
`docs/design/autopaper-next-mission-handoff-2026-11-09.md` section 8 and in
`docs/definitions/choir-autopaper-improvement-2026-11-09.ledger.md` ("Prompt
cuts: review 1 REPAIR, all four findings on the management task opening").

The change hands a management task its records and the newest page of the story
library in the first user turn, so the task stops spending its first cells on
`choir.Records()` and `choir.Stories()`. The intent stands (every paper task
re-read records the host already holds). The first implementation had three
defects. Line references are to the cherry-picked `09884e328`
(`internal/agentcore/management_tasks.go` `managementTaskOpening`,
`internal/agentcore/runtime.go` `executeWithToolLoop`).

## The three review defects

1. **The handed text arrives without the data wrapper.**
   `managementTaskOpening` appends `json.Marshal(records.view)` and the story
   page to the prompt as plain text (management_tasks.go:93-100;
   runtime.go:4047-4049). A captures record carries web-derived titles and text
   (label `world_untrusted`, `capture_read`); everything else the host injects
   from records is passed through `frameLabelled` (run_context.go:280) or
   `frameLabelledExcept` (tools_desk.go:908-910). Here nothing is, so an
   instruction inside a capture reads as the owner's.
2. **A resumed run marks records and the library shown though the text was
   dropped.** `managementTaskOpening` calls `noteRecordsShown` and (through
   `storiesEgress`) `noteStoryLibraryShown` while building the text
   (management_tasks.go:90-97), before `memory.initialize`. On a run that
   already has run memory, `initialize` discards `initialMessages` and replays
   history (run_memory.go:95-108), so the model never saw the text, yet the
   run is marked shown. The records-shown mark lets a cell's cursor advance
   past records the model never read (`capToShown`), and the library mark
   lets `texture:new` pass the library-head check without a read.
3. **The story read skips the `owner_private:story` label.** The desk's own
   `choir.Stories()` goes through `deskEgress`, which labels the result with
   `egressReadLabels(ActionStories)` = `owner_private:story`
   (run_labels.go:250-251) and taints the run. The opening calls
   `storiesEgress` directly (management_tasks.go:96) and never labels, so the
   run's label set, the disclosure gate and the model-call label log omit it.

## Every other way this can fail, each with its test

All tests are in `internal/agentcore/management_tasks_opening_test.go`.

| Failure | Handling | Test |
|---|---|---|
| Records too large for the token bound (a backlog window is up to `FrameRecordsBudget` bytes; prompts stay within 100,000 tokens) | each section has a token budget; an oversize section is not delivered and not marked, the task reads it with the verb. Never trimmed | `TestTaskOpeningOversizeSectionIsNotDeliveredOrMarked` |
| A record whose content contains the wrapper-closing text (`[End of data]`) | the closing marker is defused inside the handed text, so content cannot end the data frame early | `TestTaskOpeningRecordCannotCloseTheDataWrapper` |
| An empty library | an empty page is delivered and the head 0 is the mark; a library read failure delivers no story section and sets no library mark | `TestTaskOpeningEmptyLibraryDeliversTheEmptyPage` |
| The newest story page is owner-private | it is: the run is tainted `owner_private:story` on delivery, exactly as the verb does | `TestTaskOpeningTaintsTheRunWithTheStoryLabel` |
| A task woken twice | marks are per run id; a second run opens with its own records and its own marks, neither inherits the other's | `TestTaskOpeningTwoRunsMarkIndependently` |
| Restart mid-turn | a crash restart closes the task's runs ("interrupted by a restart"); a planned restart also closes task runs (`passivateInterruptedActivations`). Neither resumes, so no opening is rebuilt for a dead run. Existing proof | `TestManagementTasksAtACrashBootAndAPlannedBoot` |
| A resumed run after a planned restart | tasks are not resumed; but a run that re-enters with run memory (a retry of the same run id) is treated as resumed: no text, no marks | `TestTaskOpeningResumedRunDeliversNothingAndMarksNothing` |
| The marks are set before the model call fails | marks are applied only after the opening is persisted into run memory (`memory.initialize` returned), not while building the text; a build that is never delivered marks nothing | `TestTaskOpeningBuildMarksNothingUntilCommit` |
| Management's desk boundary | the opening holds only what the task already reads: its own inbox records and the story index through `storiesEgress` under the `intake.read` grant. Nothing from Texture documents, research, or evidence; a non-task run gets no opening. The host sends no research material to Texture | `TestTaskOpeningIsTheTaskDesksOwnReads` |
| Compaction later drops the opening | the marks are run-scoped and equal what a model-called `choir.Records()` leaves; a compaction summary that loses the page is the same residual as for any earlier read. Residual, not tested | none (residual) |

## Repair shape

`managementTaskOpening` becomes a pure build returning text plus a `commit`
that applies the marks; the build refuses (returns nothing) for a run with run
memory; each section is framed with `frameLabelled` by its own labels with the
closing marker defused; the story read is labelled through `noteEgressRead`
(the same call `deskEgress` makes); `executeWithToolLoop` calls `commit` after
`memory.initialize`.
