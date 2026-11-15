# Carry by reference: design section and failure list for T1

Written 2026-11-15 on lane `lane/t1-carry-design`. Design only: no code and
no tests in this change. It is the first slice of station T1 of
[the store-once goal](../definitions/choir-storage-solutions-3-store-once-2026-11-14.md):
a new run carries its conversation by naming the source run and entry range
instead of copying the entries. Background:
[store-once](store-once-2026-11-14.md) (R1, "Stop copying forward") and
[the retirement problem record](../problems/run-memory-retirement-keeps-nearly-everything-2026-11-14.md).

Mutation class when built: red (run memory is tape and evidence, checkpoint
projection, and what the model is sent). Review: two seats on this document
before any code, two seats on the code.

Claims are anchored to `file:line` as read on the worktree at `9883c37a2`.
Anything not read from code is marked **invented** or is in section F as a
question.

## What the measurement says

The goal file records that carried run memory is 55.9% of message bytes on
the tape on the largest computer (definition file, T0 settlement). The
counters that produced it are `run_memory_carry.carried_message_bytes` and
`new_message_bytes` (`internal/agentcore/run_memory_carry_counters.go:12-20`),
counted at the call sites after the append succeeded
(`run_memory.go:229` for a carry; `run_memory.go:417-419` and `:443-445`
for new messages). I did not re-measure; the figure is the goal file's.

## A. The mechanism as it is today

### A1. The only carry

- A new run inherits an earlier run's conversation in exactly one place:
  `carryTexturePriorConversation` (`internal/agentcore/run_memory.go:135`),
  called from `initialize` only when the run has no entries yet
  (`run_memory.go:94-101`). It fires only for a run with
  `actor_rewarm_carry_conversation` set and the document custodian profile
  (`run_memory.go:146`). Every other desk starts from its prompt
  (`run_memory.go:130-134`).
- The flag is set by `textureowner` when the document holds its trajectory
  (`internal/textureowner/texture_agent_revision.go:770-772`); the source
  hint `actor_rewarm_source_loop_id` is set from
  `LatestTextureActorMemorySourceRun` (`texture_agent_revision.go:761-764`,
  `internal/agentcore/texture_lifecycle_api.go:235-259`), which is
  `LatestActorRunMemoryEntries` with no excluded run.
- `carryActorConversation` (`run_memory.go:165-231`):
  1. source = the named run when its entries all belong to this agent
     (`:166-176`, `runMemoryEntriesOfAgent` `:235-249`), else
     `LatestActorRunMemoryEntries` (`:177-186`, `internal/store/run_memory.go:292-359`),
     which takes the newest completed or passivated run of the agent on the
     computer that holds a message or compaction entry (carry order and
     states: `internal/store/run_memory_carry.go:33-58`);
  2. the source's labels are unioned into the new run (`:189-193`), a
     separate object write, untouched by this design;
  3. `latestRunMemoryCheckpointAndTail` (`:307-330`) takes the newest
     compaction entry's summary as `checkpoint`, and as `tail` the message
     entries from `FirstKeptEntryID` up to that compaction plus every message
     entry after it (kind `message` and a non-empty `Message` only, `:318`,
     `:324`). With no compaction the checkpoint is empty and the tail is
     every entry unfiltered (`:331`, so a non-message entry such as a
     `cell_fate` row is in "tail" here and `:219-221` writes it as a message
     entry; see question F8);
  4. `carryableRunMemoryTail` (`:263-305`) drops tool results whose call is
     not in the tail and cuts the tail before the first tool call whose
     result never arrived;
  5. one batch is written: a compaction entry for the checkpoint with
     `Reason: "actor_rewarm"` and `Details.source_loop_id`
     (`:203-217`), then one fresh message entry per tail entry built by
     `messageEntry` (`:219-221`; `messageEntry` at `:424-432` copies the
     bytes with `cloneRawMessage`), in one `AppendRunMemoryEntries` call
     (`:222`), which is one tape event (`internal/store/run_memory.go:159-196`);
  6. counters (`:225-229`).
- Run-context turns that were in the source are copied like any other
  message (comment `run_memory.go:160-162`; the byte-for-byte promise is for
  the provider's prompt cache). The new run's context turn and prompt are
  appended after the carry as new messages (`run_memory.go:99-111`).

### A2. What one copied entry is on the tape

- Every entry is one op `run_memory_entry_recorded` whose body is
  `computerevent.RunMemoryEntryProjection` (row snapshot, full
  `message_json`): `internal/store/run_memory.go:240-251`;
  struct `internal/computerevent/projection_batch.go:99-116`.
- Each new entry gets its own random id (`normalizeRunMemoryEntry`
  `run_memory.go:135-137`, UUIDv7 `internal/store/run_memory_ids.go:15`), a
  per-run `Seq` and a `ParentEntryID` from the run's last row
  (`run_memory.go:215-239`, `runMemoryTailLocked` `:272-288`), and
  `CreatedAt = now` (`:138-140`). Text is passed through
  `sanitizeStoreText` (`:150-155`), which is `strings.ToValidUTF8(value, "�")`
  (`internal/store/store.go:87-89`), and so is idempotent on its own output.
- The store row is `run_memory_entries(entry_id PK, loop_id, ..., seq, kind,
  role, message_json, summary, first_kept_entry_id, tokens_before, reason,
  model, details_json, created_at, UNIQUE(loop_id, seq))`
  (`internal/store/schema.go:94-112`).

### A3. Replay and projection

- Live append and replay both end in `projectRunMemoryEntry`
  (`internal/store/project.go:90-91`, `:308-353`): decode the snapshot,
  require entry id, run id, `seq > 0` and a kind (`:313-315`), compare with
  an existing row of that id (identical is a no-op, different is
  `ErrProjectionMismatch`, `:319-337`), else insert the row verbatim
  (`:341-348`). It is a pure row copy, order-independent between runs, and
  accepts any `kind` string.
- A batch is validated at decode: an op kind not in `projectionOpKinds` is
  `ErrProjectionBatchInvalid` (`projection_batch.go:385-387`, map `:59-75`;
  the same fall-through at `internal/store/project.go:115-116`). Decode runs
  on every read of a sealed batch (`internal/computerevent/appender.go:393`,
  `:659`, `:745`, `:769`). A failure after the CAS is classified as
  `ErrProjectionPoison` (`projection_batch.go:455-464`), which the appender
  reports as a wedge (`internal/computerevent/wedge.go:158-159`). So an old
  binary meeting an op kind it does not know does not skip it.
- Retirement is a tape op naming runs: `ProjectionOpRunMemoryRetire`
  (`projection_batch.go:41-44`), applied by `projectRunMemoryRetire`
  (`internal/store/run_memory_retire.go:90-114`) as
  `DELETE ... WHERE loop_id IN (...)`, except kept kinds. The entries stay on
  the sealed tape (`run_memory_retire.go:3-8`), and replay applies the same
  named deletes (`:49-50`).
- The product replay probe rebuilds the store from the tape in a disposable
  store and compares `run_memory_entries` rows by content-safe row digests
  (`internal/agentcore/replay_completeness.go:19-52` report,
  `:77-163` `compareReplayRunMemory`; fingerprints
  `internal/store/run_memory.go:504-530`; the table is classed
  `ReplayEventProjection`, `internal/agentcore/replay_eligibility.go:61`).
  This is the drill vehicle in section D2.

### A4. Every reader of a run's entries

All message-building and most other readers go through one store function:
`ListRunMemoryEntries` -> `listRunMemoryEntriesTable`
(`internal/store/run_memory.go:363-414`), a `SELECT ... WHERE loop_id = ?
ORDER BY seq`. Callers (non-test), all read this session:

| Reader | Where | Uses |
| --- | --- | --- |
| Model messages for a run | `contextMessages` `internal/agentcore/run_memory.go:457-463`, via `buildRunMemoryContext` `:1128-1164`; `initialize` `:93-113`; `beforeProviderCall` `:342-356`; overflow rebuild in `onProviderError` `:384-413` | the byte-equality target |
| Compaction | `compactIfNeeded` `run_memory.go:466-`; `planRunMemoryCompaction` `:700-790` | entry ids, `FirstKeptEntryID`, message bytes for token estimate and the summary input |
| Carry source | `carryActorConversation` `:169`; `LatestActorRunMemoryEntries` `store/run_memory.go:346` | origin of the carried entries |
| Texture frame, cell author, controller | `agentcore/texture_frame.go:59`, `textureowner/texture_cell_author.go:104`, `textureowner/texture_controller.go:1498` | not read in detail (see F3) |
| Management, desk shots, engineering assignment | `agentcore/management_controller.go:2552,2704,2777`, `desk_shots.go:512,718,968`, `management_tasks.go:111`, `tools_engineering_assignment.go:183` | not read in detail (see F3) |
| RLM reducer | `agentcore/rlm_reduce.go:559,614` (per run) and `:582` (`ListRunMemoryEntriesForAgent`, kind cursor only) | cursor kinds only for the agent-wide read |
| Lifecycle report authority | `store/lifecycle.go:4020` | whether a control message is in the run's durable context |
| One entry by id | `GetRunMemoryEntry` `store/run_memory.go:626-651`, tool `agentcore/tools_evidence.go:35-84` (run-scope check at `:62-64`) | entry handles named in checkpoints |
| Fork/restore residue | `store/residue_import.go:437-472` (raw rows, `ORDER BY owner_id, loop_id, seq`, one op per row, `ProjectionOpRunMemoryEntry`) | emits rows as they are |
| Snapshot copy | `store/snapshot_to.go:400` maps the table to kind `choir.run_memory_entry` | the table is kept or emptied whole by lease kind |
| Retirement selection | `store/run_memory_retire.go:153-181` (group query), `:245-464` | which runs may be deleted |

The writers of the table are `AppendRunMemoryEntry` (`store/run_memory.go:31`),
`AppendRunMemoryEntries` (`:163`), the tape append (`:215`), the projector
(`project.go:308`), and residue import. No other code inserts.

## B. The change: one mechanism, a reference entry

### B1. What is written

A new kind of run-memory entry, `carry_ref` (**invented name**), written in
place of the copied message entries of a carry. One reference entry per
carry, in the same batch and the same single tape event as today's
checkpoint entry (`run_memory.go:200-222`). It rides the existing row type
and the existing table: same columns, no schema change.

| Column | Value |
| --- | --- |
| `entry_id`, `loop_id`, `owner_id`, `agent_id`, `seq`, `parent_entry_id`, `created_at` | as any entry: assigned by `normalizeRunMemoryEntry` and `appendRunMemoryTapeLocked` |
| `kind` | `carry_ref` |
| `details_json.segments` | ordered list of `{run_id, first_seq, last_seq, count}` over **origin** runs (see B3) |
| `details_json.digest` | sha256 over the resolved messages (the exact bytes the carry would have copied, each prefixed by its length) in order |
| `details_json.count` | total resolved messages |

`message_json` is empty. Order in the batch: checkpoint entry (still a copy:
it is a summary, one per carry, and rewrites `TokensBefore`; see F5), then
the one `carry_ref`. The `carry_ref` takes one `Seq`. The entries after it
number from `ref.Seq + 1`, exactly as a single entry would
(`run_memory.go:230-239`), so `UNIQUE(loop_id, seq)` and the tail logic are
unchanged.

### B2. The tape op

Decision: a **new projection op kind**, `run_memory_carry_recorded`
(**invented name**), with the same body struct as the run-memory entry and
the same decoder, applied by the same row insert. Not the existing
`run_memory_entry_recorded` with a new `kind`. Reason, from A3: the existing
op accepts any `kind` string (`project.go:313-315`) and `buildRunMemoryContext`
ignores kinds it does not know (`run_memory.go:1138`, `:1153`, `:1159`), so
an old binary that replays a `carry_ref` row inside the old op would
**silently** build a conversation without the carried part. A new op kind
makes an old binary refuse at decode (`projection_batch.go:385-387`), loudly,
and the retire op was added the same way. The cost is that a binary older
than the reader cannot run the tape at all; that is what section B6 and E
fence. Open question F1 states the alternative.

`tape_write_counters.go:89-99` classes an op by kind into a family; the new op
needs its own family line so carried bytes stay countable (**invented**
family name `run_memory_carry`).

### B3. How a reference is resolved: always to the origin, flattened at write

A reference never points at another reference. When the source run's
entries include `carry_ref` rows, the writer resolves them first (below) and
names, per tail entry, the physical entry holding the bytes: its origin run
and origin seq. Segments are then maximal runs of consecutive origin seqs in
one origin run. A carry from a run that itself carried is typically two
segments: the older origin's range and the source's own new messages.
Depth is therefore always one, there is no chain walk, and no cycle can form.

Resolution at read, one function in the store, `resolveCarryRef(row)`:

1. for each segment, read the origin run's entries with
   `seq BETWEEN first_seq AND last_seq`, keep `kind = message` with a
   non-empty `message_json` (the same filter as
   `latestRunMemoryCheckpointAndTail`, `run_memory.go:318`, `:324`), in seq
   order;
2. concatenate the segments in order;
3. require the count and the digest in the row to match; any mismatch or
   missing origin row is an error, never a shorter list;
4. emit one `types.RunMemoryEntry` per resolved message with:
   `RunID`, `OwnerID`, `AgentID` of the **carrier**; `Kind` message; `Role`
   and `Message` bytes of the origin row; `EntryID = <ref entry id>#<index>`
   (**invented**; stable and unique within the run; used in
   `FirstKeptEntryID`, raw entry handles and `get_run_memory_entry`);
   `Seq` = the ref's seq; `ParentEntryID` = the previous resolved id;
   `CreatedAt` = the ref's. Unexported provenance (origin run, origin seq) is
   kept on the in-memory struct only (**invented** fields, `json:"-"`).

`listRunMemoryEntriesTable` (`store/run_memory.go:370-414`) calls it for
each `carry_ref` row before returning, replacing the row by its resolved
messages in place. Every reader in A4 then sees a list with the same shape
as today's: messages in order after the checkpoint, ids unique, kinds
message. `buildRunMemoryContext`, `planRunMemoryCompaction`, the retirement
group query on origin rows, the lifecycle authority read and the tool need no
change. `GetRunMemoryEntry` (`:626-651`) learns the `#<index>` suffix:
split, load the ref row by the prefix, resolve, pick the index.
`ListRunMemoryEntriesForAgent` (`:448`) is called with the cursor kind only
in the code read (`rlm_reduce.go:582`); it returns raw rows and never
resolves; a caller that passes an empty `kinds` would see refs (question F3).

The write side uses the same function: `carryActorConversation` computes the
tail as today (`:194-195`), builds segments from each tail entry's origin,
**resolves its own segments**, and requires the result to equal the tail
byte for byte before it appends (`bytes.Equal` over every message, plus
roles); if it does not, it writes the old copy batch and counts a
`carry_ref_fell_back` counter (**invented**), so a bug in segment building
costs the saving, never the conversation. The fallback is the unchanged old
write path, which is kept anyway (B5); it is a correctness refusal, not a
switch.

### B4. Replay

`projectRunMemoryCarry` (**invented**) is `projectRunMemoryEntry` with a
check of the row's kind and details shape, then the same compare-or-insert.
It does **not** look at the origin rows: replay must never wedge on a
missing origin, and replay order across runs is not guaranteed (residue
import emits by `owner_id, loop_id`, `residue_import.go:437-472`, so a
carrier's run id can sort before its origin's). The reference is resolved
only at read. At live write time the dry-run projection that the appender
already performs (the head invariant, `store/run_memory.go:198-214`) is
given the live flag the projector already carries for replay-only behavior
(`allowReplayTextureBootstrap`, `project.go:73-97`, **how the flag reaches
this op is not read**: question F9) and, live only, checks that every origin
row of every segment exists in the table at that head; a refusal is
`ErrBatchRefused` and nothing is sent, the same path an ordinary head race
takes. That makes "origin exists" part of the tape order, so a retire op
ordered before the carry cannot be validated away.

### B5. Old tape stays readable forever with no rewrite

- Every existing copied entry is an ordinary `message` row. `resolveCarryRef`
  is only invoked for `kind = carry_ref`. A run written before this change,
  and every run that fell back, reads as it does now.
- The old writer path (`messageEntry` for each tail entry,
  `run_memory.go:219-222`) stays in the code for the fallback and for
  residue import (F11). It is not behind a setting.
- Nothing on the tape is rewritten, deleted or re-chunked. No migration.
  Old bytes stay where they are and age out through the existing retirement
  rule.
- The reader that understands `carry_ref` and the op kind ships and is
  deployed before any binary that writes one (B6).

### B6. Retirement keeps entries a reference names

Retirement deletes whole runs by `loop_id` (`run_memory_retire.go:101`). A
referenced origin must not be selected while any surviving reference names it.

1. `runMemoryCarryKinds` (`store/run_memory_carry.go:19`) gains `carry_ref`,
   so a run whose carried body is a reference still counts as holding a
   carry body (`runMemoryHasCarryBody` `:22-31`; the group query's `carry`
   column `run_memory_retire.go:176`). Without it, a carrier holding only a
   reference would not be a carry source and the next older run would be
   taken (failure 9).
2. `selectRunMemoryPlan` (`run_memory_retire.go:245-464`) reads, before the
   selection loop, the set of origin run ids named by **every** `carry_ref`
   row in the table whose carrier run is not itself in this pass's plan
   (**invented rule, conservative variant**: simply every `carry_ref` row
   present when the read happens, whether or not its carrier will retire this
   pass; an origin is then freed on the pass after its last carrier goes).
   A run in that set takes no retirement: one more census disposition
   `kept_named_by_reference` (**invented**), placed beside
   `kept_carry_source_*` (`:390-396`), before the state checks. The read is
   one `SELECT loop_id, details_json FROM run_memory_entries WHERE kind =
   'carry_ref'`; whether an index serves it is **not verified** (F6).
3. The rule in the head comment (`run_memory_retire.go:10-37`) and the
   problem record's invariant gain "and the origin runs of any entry still
   in the working store".
4. Carrier retirement is unchanged: a retired carrier's `carry_ref` row is
   deleted with the rest of its run (`:101`), and the origin becomes
   retirable on the next pass.

The existing named residual (an in-pass window of about two minutes between
the read of records and the last append, `docs/problems/run-memory-
retirement-keeps-nearly-everything-2026-11-14.md`, added by d46fb88b7) widens:
a carry can name an origin that the same pass has already planned. Part B4's
live check at the carry's dry run turns that from silent loss into a refused
write (the carry retries or falls back to the copy, which reads the entries
that are still there). It is not a fix of the selection window.

### B7. Compaction of the source, and of the carrier

- Compaction appends a new `compaction` row to its own run and never
  touches earlier rows (`run_memory.go:537-562`); a source run's compaction
  after the carry does not change origin rows. A reference names seqs, and
  an entry's bytes never change (`project.go:333-335` refuses a changed
  row).
- A carrier's own compaction sees resolved entries like any other
  (`planRunMemoryCompaction` `:700-790`, ids with `#`), and records
  `FirstKeptEntryID` and `raw_entry_ids` as resolved ids
  (`:762-764`, `:768`); those strings are stored in the carrier's own
  compaction row, and resolve forever as long as the ref row and origin rows
  exist.

### B8. Byte-equality is the contract

For any carrier run, `buildRunMemoryContext(ListRunMemoryEntries(run))` must
equal, byte for byte and in order, what the same carry would have produced by
copying. The pieces that make it true by construction: messages are taken
from origin rows by the same filter as the copy path, in the same order; the
text normalisation is idempotent (A2); the resolved messages are not
re-serialised, only sliced from the stored column; and the writer asserts it
before it appends (B3). The pieces that make it not obviously true are listed
as failures 1 to 6.

## C. Failure list

Each entry: the failure, then how the design prevents or detects it. "Test"
means a repo test written before the code (CLAUDE.md, Proof), and "drill"
means the deployed proof in D2.

1. **Silent change of model input: the reference resolves to different
   bytes than the copy would have.** Prevented by the write-time assertion
   (B3), the stored digest checked at every read (B3 step 3; a changed or
   missing origin row fails the read loudly), and the repo test D1 across
   every shape in failures 2 to 6.
2. **Dropped entries inside the range.** The copy path drops orphan tool
   results and cuts at the first unanswered tool call
   (`carryableRunMemoryTail`, `run_memory.go:263-305`), and skips non-message
   kinds. A bare `[first_seq, last_seq]` would include them. Prevented: the
   reference is a list of segments built from the final tail, split wherever
   the tail skipped a message-kind entry; non-message kinds are filtered by
   resolution (B3 step 1); the assertion catches the rest. Test cases: orphan
   at the head, orphan in the middle, pending tool call at the end, a
   `cell_fate` row between messages, an empty-bytes message
   (`recordCarry` counts `carriedEmptyMessages`, `run_memory_carry_counters.go:61-68`,
   so empty messages exist; resolution drops them like `:318`, the same as
   the copy path's `:324`).
3. **Checkpoint plus `FirstKeptEntryID` window.** The tail includes messages
   from the source's first-kept entry up to its compaction
   (`run_memory.go:312-322`) and after. Those can sit before the compaction
   row in seq. Prevented: segments are built from the tail's actual origin
   entries, not from the compaction's position; test with a source that
   compacted mid-conversation.
4. **Run-context turns and runtime injections.** They are ordinary messages
   copied byte for byte (`:160-162`); the comment says no message is carried
   as a runtime injection (`:157-159`) but the code copies the stored role
   (`messageEntry(runMemoryMessageRole(entry.Message), ...)`, `:220`), which
   is parsed from the message, not from the source row's `Role`. Resolution
   must produce `Role` as `runMemoryMessageRole` of the bytes, not the origin
   row's `Role` column, or an injection authority marker would leak
   (`afterAppendMessage` `:357-381` shows `Role` carrying
   `runtime_injection`). Test: a source with an injection row.
5. **Normalisation drift.** The copy path re-applies `sanitizeStoreText`
   (`store/run_memory.go:150`); the reference does not. Equal only if the
   function is idempotent on stored text, which holds for the current
   implementation (A2) but is a property that must not change. Test pins it.
6. **Entry ids and seqs differ.** Carried entries had fresh ids and a seq
   each; resolved ones have `#index` ids and the ref's seq. Anything that
   keys on either breaks. Prevented for the readers read (ids only in
   compaction, handles, the tool: B3, B7); `Seq` is printed by the tool
   (`tools_evidence.go:69`) and compared in the replay probe only on rows
   (`replay_completeness.go:130-160`). Readers in A4 marked "not read in
   detail" are open (F3); the test D1 runs each over a carrier built both
   ways and diffs their outputs.
7. **Dangling reference after retirement.** Prevented by B6 (kept by
   selection) and by the live origin check at the carry's dry run (B4). If
   it happens anyway (an old retire op replayed, a restore from a checkpoint
   whose tail order differs, a bug), resolution **fails the read**; it never
   returns the surviving part. A failed read fails the wake with a named
   error; it is logged with the carrier and origin ids and counted
   (**invented** `carry_ref_dangling`). See F2 for what the run should do.
8. **Retirement races a new carry.** A carry written after the selection read
   names an origin the pass has planned (the named residual widens, B6).
   Detected at write by the live check (B4), refused, and the next attempt
   re-reads the source and names whatever still exists. Not prevented in
   selection; stated.
9. **A carrier holding only a reference is not seen as a carry source.**
   `runMemoryHasCarryBody` (`run_memory_carry.go:22`) and the group query
   (`run_memory_retire.go:176`) read `runMemoryCarryKinds`. Prevented by
   adding the kind (B6.1). Test: a carrier whose only carry-kind row is a
   `carry_ref` is taken as the source by `LatestActorRunMemoryEntries`, and
   counts toward `kept_carry_source_*`.
10. **Replay order: the reference is replayed before its source.**
    The projector inserts the row and never reads the origin (B4). Residue
    import sorts by `owner_id, loop_id` (`residue_import.go:437-472`), which
    can put a carrier first; harmless because resolution is read-time. A
    read during replay (a replaying store used by a probe) is not made
    until the replay ends.
11. **Replay probe compares only rows.** `compareReplayRunMemory` hashes
    physical rows (`replay_completeness.go:77-163`); a row-equal tape could
    still resolve differently if an origin row were missing in both. Add to
    the probe a per-run resolved digest (the `details.digest` re-verified)
    for every `carry_ref` row; the drill (D2) reads it.
12. **Crash mid-write.** The checkpoint entry and the reference are one batch
    in one tape event: one CAS, durable before it returns
    (`store/run_memory.go:159-163`, `:198-214`; the call returns only after
    the event's CAS and finalize). A crash leaves either both or neither.
    Boot closes the obligation as interrupted and never resumes it
    (CLAUDE.md, Restarts); a half-written carry cannot exist because there is
    no second event. The old path was one batch too (`run_memory.go:200-201`).
13. **Revert to the old binary after reference ops are on the tape.**
    The old binary rejects the new op kind at decode and the appender wedges
    (A3). Prevented only by ordering: reader and projector first (deploy 1,
    no writer), writer second (deploy 2), and no revert of deploy 1 once deploy
    2 has written (E). The drill D2 includes starting the previous release's
    reader against a tape holding references, to confirm what the old binary
    does (**not yet known**: the claim in A3 is from code; it has not been
    run).
14. **Old reader keeps working after a revert of deploy 2.** Deploy 2 reverted
    leaves the deploy 1 reader and projector, so references already on the
    tape still resolve; new carries are copies again. Stated, and proved by
    the drill's last step.
15. **Skew between guest and platform binaries.** The appender decodes batches
    on the guest (`computerevent/appender.go:393`, `:659`, `:745`, `:769`);
    whether the platform side decodes or only stores sealed bytes is not read.
    If any process that handles the tape decodes with an older vocabulary,
    deploy 2 must wait for it (F10). Default: treat as a deploy-order
    requirement and verify before deploy 2.
16. **Fork, restore and sibling computers.** A sibling has its own tape
    (CLAUDE.md, Safety). A snapshot copy keeps or empties the whole table by
    lease kind (`snapshot_to.go:395-401`, comment above); whether rows are
    also filtered by run is not read (F7). Residue import emits table rows as
    ops (`residue_import.go:437-472`), restricted to the computer's run
    scopes (`:380-410`). If a carrier lands in a sibling without its origin,
    reads fail. Prevented by default: residue import emits the carrier's
    resolved messages as old-format copies (this keeps the old writer needed,
    B5) and never a `carry_ref`; a snapshot copy takes the table whole, so
    refs and origins stay together; any path found to split them materialises
    first. Drill: restore a copy of d@e and read a carrier.
17. **Restore to an earlier checkpoint.** Replay from a base blob plus tail
    applies ops in tape order; an origin is always earlier in the tape than
    its carrier (the writer read it first, B4 verifies it at append). A
    restore target between the origin's write and the carrier's applies
    neither; between a retire op and the carrier it applies the retire first,
    and the carrier's check was done at append, so such a tape does not exist
    unless the live check was bypassed. Detected by 7 and by the probe (11).
18. **Two carriers name one origin.** Allowed (a read-only reference). The
    origin stays until all carriers are retired (B6.2 keeps while any row
    names it).
19. **A reference naming its own run, or a cycle.** Flattening makes depth one
    (B3). A segment whose `run_id` equals the carrier's is invalid (the
    existing guard `sourceRunID != runID`, `run_memory.go:168`, is
    kept) and is rejected by `projectRunMemoryCarry`'s shape check.
20. **Partial ranges.** An origin that is still being written (a source in
    `passivated` that is re-warmed) gains entries after the carry. The
    reference names an explicit `last_seq`, and `count`, so later rows are
    outside it. The digest and count detect a retired middle (rows are
    deleted by run, so a middle cannot go alone; a run-level delete leaves
    zero rows, caught as dangling).
21. **Origin rows altered by projection.** A row of the same id with
    different content is refused (`project.go:333-335`). A table repair that
    rewrote messages would break the digest and fail reads, loudly.
22. **Concurrency inside a carrier.** The carry runs inside `initialize`
    only when the run has no entries (`:94-101`) and appends under the run's
    memory lock (`store/run_memory.go:184-186`); the origin is another run,
    read without its lock, after its run finished or passivated
    (`carrySourceState`, `run_memory_carry.go:56-58`). A source that wakes
    and appends while the carry reads is bounded by the explicit
    `last_seq`.
23. **Counters lie after the change.** The carry counters
    (`run_memory_carry_counters.go`) count bytes at the call sites; a
    reference carries bytes only in `details` (a few hundred). The counters
    must count the resolved bytes as `carried_message_bytes` (what the model
    sees) and add `referenced_message_bytes` and `written_carry_bytes`
    (**invented**) so the goal's acceptance reading (carried and duplicated
    bytes under 5% of payload, definition file) reads written bytes, not
    resolved ones. Test.
24. **Read cost.** Today the carrier's list is one `SELECT` of its own rows;
    now each reference adds a second read of the origin's range, and a long
    chain flattened to two or three origins adds a few more. Measure before
    deploy 2 on d@e; no cache is added (rule: one mechanism). Origin rows
    are read through the same primary indexes (`UNIQUE(loop_id, seq)`);
    cost not measured (F6).
25. **Compaction of a source after it was referenced shrinks nothing.** The
    origin rows stay while referenced (B6). Disk saved is the copy bytes
    not written, not the origin's. The retirement of the source's own
    old messages waits for the carriers. Accepted; the goal figure is tape
    bytes, which do shrink (A2 shows the copy is a full row).
26. **Sanitisation or schema column added later.** A new column on
    `run_memory_entries` must be in `RunMemoryEntryProjection`
    (`projection_batch.go:99-116`) or the probe diverges; unchanged by this
    design, which adds none.
27. **Encrypted or redacted payloads.** The sealed batch can have a payload
    redacted (`ErrProjectionBatchUnreadable`, `projection_batch.go:51-54`:
    the event advances the head with no state change). A redacted carry
    batch leaves a carrier with no reference row and no checkpoint; same as
    a redacted copy today. A redacted **origin** batch leaves the reference
    dangling (failure 7, loud) where a copy would have survived. Stated:
    redaction of an origin is an owner act; the redaction tooling should
    list carriers that name it (F12).
28. **The tool's run scope.** `get_run_memory_entry` allows only the run's
    own entries (`tools_evidence.go:62-64`). A resolved `#index` id belongs to
    the carrier and passes; the origin's own id does not (as today, an entry
    of another run reads as absent), so checkpoint handles written by the
    carrier name resolved ids. A checkpoint written **before** this change
    names copy ids, still present as rows. No change.

## D. Proof

### D1. The repo test

Written first, in `internal/agentcore` beside
`texture_hold_trajectory_test.go` and `run_memory_carry_counters_test.go`
(the existing carry tests, not read in full; this design does not assume
their helpers). One test, table-driven over source shapes, that builds two
stores with the same source run and then:

1. carries the source into a carrier with the **copy** writer (kept as a
   test-visible function, the fallback) and into a carrier with the
   **reference** writer;
2. asserts `buildRunMemoryContext(ListRunMemoryEntries(carrier))` is
   byte-equal in both, message by message, and that
   `planRunMemoryCompaction` produces the same summary input, `TokensBefore`
   and kept count (ids differ by design, compared by position);
3. asserts the carrier's tape ops: the reference carrier writes one
   `run_memory_carry_recorded` op whose body is under a fixed small bound
   and no message bytes.

Source shapes (each a row of the table, from failures 2 to 6): no
compaction; compaction with `FirstKeptEntryID` mid-run; orphan tool result at
the head; unanswered tool call at the tail; `cell_fate` row between messages;
empty message; run-context turn; runtime injection; a source that is itself a
carrier (flattened, two segments, then three); invalid UTF-8 in a message.

Further tests written before code: retirement keeps an origin named by a
reference (and frees it after the carrier is retired, next pass); a carrier
holding only a reference is a carry source; resolution fails on a missing or
altered origin; replay of the carrier's ops into an empty store, in
both `owner/run` orders, gives equal rows and equal resolved messages;
`get_run_memory_entry` resolves a `#index` id; the old binary's reader is
exercised by decoding a batch containing the new op with the previous
vocabulary and asserting it is refused (pins failure 13).

### D2. The deployed drill, on d@e only

Never on the paper or the owner's computers (CLAUDE.md, Proof). After deploy 1
(reader only) and then deploy 2 (writer):

1. drive d@e through a Texture hold-trajectory carry twice in sequence so
   there is a carrier of a carrier, and record the three runs' ids and the
   resolved digest of each (the read through the product API; the tool for
   one entry shows the resolved id, `tools_evidence.go:60-84`);
2. run the product replay probe (`replay_completeness.go`, schema version 5):
   whole tape into an empty store, compare. Expected: `run_memory.different_count`
   0, `live_only_count` 0, the extended per-run resolved digests equal
   (failure 11), result equivalent;
3. compare the model input: for each of the three runs, the context built by
   `contextMessages` on the live store and on the replayed store, as one
   content hash of the message list; equal;
4. read the carry counters before and after: `carried_message_bytes` rises as
   before, `written_carry_bytes` does not, and the tape op counters show a
   `run_memory_carry` family of small ops instead of `run_memory` bytes for
   the carry;
5. force a retirement pass on d@e (the census read) with the carrier live
   and confirm `kept_named_by_reference` names the origin, then retire the
   carrier and confirm the origin is released on the next pass;
6. revert drill: start the previous release's binary against the tape and
   record what it does (failure 13); then redeploy the deploy 1 reader and
   confirm the three runs still read;
7. restore a checkpoint into a sibling (failure 16) and read a carrier.

Evidence class: deployed (staging acceptance; local proof does not count for
checkpoints, rollback or restore, CLAUDE.md). Final report items per CLAUDE.md
(SHA, CI, deploy, accepted run ids, rollback, expedites).

## E. Rollback

- Deploy 1 (reader, projector, retirement rule, counters, live check with
  no writer): a revert is safe while no reference is on the tape, because
  nothing wrote one.
- Deploy 2 (the writer path in `carryActorConversation`): a revert returns to
  copying. References already on the tape stay readable because the reader
  from deploy 1 stays. **Deploy 1 is never reverted after deploy 2 has
  written a reference**: the older binary cannot decode the op (A3). That
  is a one-way property, written into the landing notes and the deploy
  check.
- The old reader and writer paths (`message` rows, the copy batch) are kept
  forever; nothing deletes them. No setting turns the reference writer off;
  a revert is a commit revert.
- Nothing on the tape is rewritten, so there is no data rollback.
- Mutation class red. Protected surfaces: tape and evidence (a new op kind),
  run memory projection, run acceptance inputs (what the model is sent).
  Heresy delta and conjecture delta to be written with the code per CLAUDE.md.

## F. Open questions, with a conservative default

F1. **New op kind (loud on an old binary) or the existing op with a new
    entry kind (silent on an old binary)?** Default: new op kind, B2.
F2. **What should a wake do when a reference dangles?** Default: fail the
    wake with a named error and log it (failure 7); never carry a shorter
    conversation. The alternative, falling back to the newest live source and
    carrying that, changes model input and is not taken without the owner.
F3. **Do the readers marked "not read in detail" (A4) depend on the carried
    entries' ids, seqs, `CreatedAt` or `RunID`?** I did not read them.
    Default: resolved entries carry the carrier's run, owner, agent and the
    ref's time (B3), and the D1 test diffs the output of each such reader
    between a copy and a reference carrier before code is accepted.
F4. **Should the checkpoint entry also be a reference?** It is one entry per
    carry and its `TokensBefore` is part of the rendered bytes
    (`run_memory.go:196-199`, `compactionSummaryMessage` `:1355-1367`).
    Default: keep it a copy; measure its bytes in the first counter reading
    after deploy 2 (the counters count message bytes only).
F5. **Is `carry_ref` also needed for `FirstKeptEntryID` that names an entry
    inside a reference?** Resolved ids `#index` are stored in the
    carrier's own compaction rows and must resolve forever; they resolve as
    long as the ref row and origin rows exist. Default: yes, as designed.
F6. **Read cost and an index for "which origins does any reference name".**
    The existing indexes are `(owner_id, agent_id, kind, created_at)` and
    `(loop_id, kind, created_at, owner_id, agent_id)`
    (`schema.go:356-357`); I did not run a plan for `WHERE kind = 'carry_ref'`
    nor measure the origin range read. Default: run the plan on d@e before
    deploy 2 and add an index only if the plan scans (separate change, with its
    own review).
F7. **Does the snapshot copy filter `run_memory_entries` rows by run, or keep
    the table whole?** I read the table-level disposition
    (`snapshot_to.go:395-401`) but not the row filter. Default: assume it can
    split a carrier from its origin and materialise at residue import
    (failure 16).
F8. **A source with no compaction takes every entry as the tail
    (`run_memory.go:331`).** Entries of other kinds in `entries` (cell_fate,
    cursors, receipts) are not messages and `latestRunMemoryCheckpointAndTail`
    returns them when there is no compaction (the early `return "", entries`
    at `:331` returns the unfiltered list, while the compaction branch
    filters by kind). Then `carryActorConversation` writes
    `messageEntry(runMemoryMessageRole(entry.Message), entry.Message)` for
    them (`:220`) with empty bytes. If so the copy path writes empty-message
    rows for non-message entries today, which `carriedEmptyMessages` would
    count (`run_memory_carry_counters.go:24`, `:61-68`). I did not verify this
    on data. Default: resolution drops non-message and empty entries
    (B3), the D1 test asserts the context bytes only (empty messages never
    reach `buildRunMemoryContext`, `:1138`, `:1153`, `:1159`), and the first
    counter reading after deploy 2 shows whether `carried_empty_messages` falls.
F9. **How does the projector tell a live dry run from replay for this op?**
    `projectOp` takes `allowReplayTextureBootstrap`, `replayComputerID`
    (`project.go:73`); how the dry run in the appender is distinguished for
    the origin check was not read. Default: implement the check at the writer
    (a read of the origin rows under the same store connection before the
    append, refusing if absent), not in the projector, which loses the tape
    ordering guarantee in failure 8 but never risks a replay wedge; and say so
    in review.
F10. **Is any process other than the guest runtime decoding projection
    batches (platform replay, the host)?** Not read. Default: deploy 2 waits
    until every such binary carries deploy 1.
F11. **Does residue import need the old writer, or can it emit refs when the
    origin is in scope?** Default: old format only (failure 16); simpler,
    costs no saving on restore.
F12. **A tool to list carriers that name a run to be redacted.** Default:
    none now; a named residual (failure 27).
F13. **Names.** `carry_ref`, `run_memory_carry_recorded`, `#index` ids,
    `kept_named_by_reference` and the counters are invented here; the owner
    or the review seats may rename them before code.

## What I could not verify

- Nothing here was compiled or run. Every behaviour claim is from reading the
  cited lines; claims about how an older binary behaves (failure 13) are from
  code, not from running it.
- The 55.9% figure is taken from the goal file, not re-measured.
- The readers in A4 marked "not read in detail" and the existing carry tests
  were not read beyond their names.
- The platform side's decoding of tape batches, the snapshot row filter, the
  query plans and the read cost are open (F6, F7, F10).
