# Carry by reference: design section and failure list for T1

Written 2026-11-15 on lane `lane/t1-carry-design`; revision 3 after rounds 1 and 2
(two seats each, all REPAIR). Design only: no code and no tests in this change.
First slice of station T1 of
[the store-once goal](../definitions/choir-storage-solutions-3-store-once-2026-11-14.md):
a new run carries its conversation by naming the source run and entry
range instead of copying the entries. Background:
[store-once](store-once-2026-11-14.md) (R1) and
[the retirement problem record](../problems/run-memory-retirement-keeps-nearly-everything-2026-11-14.md).

Class when built: red (tape and evidence, checkpoint projection, what the
model is sent). Two seats on this document before any code, two on the code.
Line anchors are at `9883c37a2` plus this lane's own docs commit; the code
files are unchanged by it.

## Verdict, stated first

The mechanism can be made safe, but **not as one small mechanism**. It is
**six pieces**, each required by a finding and each verified against code
below:

1. the reference row and its resolver in the store's list function;
2. a new tape op kind, and a store-schema epoch and floor written before
   the first reference (so an older binary refuses to open instead of
   wedging);
3. an apply-time guard in the retire projector, with the selection-time
   filter that keeps the census and the cap honest (required, B7);
4. a seq-block rule so numbering matches what the copy batch assigns;
5. "dangling reference" handled in three named places (the carrier's own
   read is loud; a carry candidate and the hinted source are "not a
   source"; two readers that swallow errors must stop);
6. residue import emitting the origin rows of each reference.

**The copy writer, one statement.** The per-message copy loop in
`carryActorConversation` (`run_memory.go:219-221`) is deleted.
`messageEntry` stays, because `appendMessage` and `appendMessages` use it
for every run's opening and later messages (`run_memory.go:416-445`). The
checkpoint summary row is still copied (one row per carry). Residue import
re-emits table rows (`residue_import.go:450-474`) and has no copy path. The
function that copies for comparison lives only in the D1 test file. It is
therefore not a piece.

Removed since the first draft: a stored digest, a write-time assertion with
a copy fallback, a probe extension, a new counter family, a `carry_ref`
entry in the carry-kinds list, an origin check in the projector.

### The cost to the owner, and the order of the work

- **The epoch 7 floor is one way.** Once the first reference is written, no
  binary below epoch 7 opens that store (`open.go:223`, `floor.go:109-119`),
  and a floor never lowers (`floor.go:67-107`), so this stays true even if
  the writer is later removed. The way back below 7 is a manual recipe
  (like the header/body split's), not a revert.
- **Order.** (1) the reader release: epoch 7, resolver, projector, retire
  guard and filter, residue expansion, no writer, no floor written;
  (2) on d@e: the sibling-restore row, the older-binary refusal row, and
  the retention reading that section E's stop condition needs; (3) only
  then the writer release, which writes the floor on its first reference.
- **How the writer reaches d@e before the paper: there is no existing way.**
  Read: ring-ordered deploys put d@e first (R0) and the paper last (R5)
  (`docs/design/ring-ordered-deploys-2026-11-03.md`, ring list); but the
  soak floors are zero since 2026-11-10 (`ringSoaks`,
  `internal/vmctl/ring_walk.go:86`, and the amendment in that document), so
  a ring opens as soon as its members are seen healthy, minutes apart; and
  every non-docs landing carries `Ring-Expedite` (CLAUDE.md, Delivery),
  which opens every ring at once. There is no per-release hold and this
  design does not invent one (CLAUDE.md: no switches). Open question F1
  states the owner decisions that would give the order. Until the owner
  chooses, the d@e drills cannot finish before the paper receives the
  writer, so the writer release is not landed. Section E gives a read-only
  estimate of the retention gate that needs no writer.

Invented names and numbers are marked **invented**. Everything else is read
from the cited line. Nothing was compiled or run. Round 2 rows are in the
last table.

## A. The mechanism as it is today

### A1. The only carry

- A new run inherits an earlier run's conversation in one place:
  `carryTexturePriorConversation` (`internal/agentcore/run_memory.go:135`),
  called from `initialize` only when the run has no entries
  (`run_memory.go:94-101`), only for a run with
  `actor_rewarm_carry_conversation` and the document custodian profile
  (`:146`). Every other desk starts from its prompt (`:130-134`).
- The flag and the source hint are set by `textureowner`
  (`internal/textureowner/texture_agent_revision.go:761-772`); the hint comes
  from `LatestTextureActorMemorySourceRun`
  (`internal/agentcore/texture_lifecycle_api.go:235-259`).
- `carryActorConversation` (`run_memory.go:165-231`):
  1. source = the hinted run when all its entries belong to this agent
     (`:166-176`, `runMemoryEntriesOfAgent` `:235-249`), else
     `LatestActorRunMemoryEntries` (`:177-186`;
     `internal/store/run_memory.go:292-359`), the newest completed or
     passivated run of the agent with a message or compaction entry (order
     and states `internal/store/run_memory_carry.go:33-58`);
  2. labels are unioned (`:189-193`), a separate write this design does not
     touch;
  3. `latestRunMemoryCheckpointAndTail` (`:307-331`): the newest compaction's
     summary as checkpoint, and as tail the message entries (non-empty
     `Message`, `:318`, `:324`) from its `FirstKeptEntryID` up to it and
     after it. With **no compaction it returns every entry unfiltered**
     (`:331`);
  4. `carryableRunMemoryTail` (`:263-305`) drops tool results whose call is
     not in the tail and cuts before the first unanswered tool call;
  5. one batch: a compaction entry for the checkpoint
     (`Reason: "actor_rewarm"`, `Details.source_loop_id`, `:203-217`) and a
     fresh `message` entry per tail entry (`:219-221`; `messageEntry`
     `:424-432` copies bytes with `cloneRawMessage`, role parsed from the
     bytes by `runMemoryMessageRole` `:1369-1377`), through
     `AppendRunMemoryEntries` (`:222`), one tape event
     (`internal/store/run_memory.go:159-196`);
  6. counters (`:225-229`).
- Because of 3, an uncompacted source's non-message rows are copied too, as
  `message` rows with empty bytes. Round 1 confirmed this against
  `texture_cell_author.go:98` (a `texture_cell_author_receipt` row appended
  per committed cell-author turn, `texture_cell_author.go:126-135`) and the
  `cell_fate` row (`rlm_reduce.go:653`). They never reach the model:
  `buildRunMemoryContext` keeps only `message` rows with non-empty bytes
  (`run_memory.go:1138`, `:1153`, `:1159`).

### A2. What a copied entry is on the tape

- One op `run_memory_entry_recorded`, body
  `computerevent.RunMemoryEntryProjection`
  (`internal/computerevent/projection_batch.go:99-116`), built at
  `internal/store/run_memory.go:240-251`: full `message_json`, own random
  UUIDv7 id (`normalizeRunMemoryEntry` `:135-137`,
  `internal/store/run_memory_ids.go:15`), per-run `Seq` and
  `ParentEntryID` from the table's last row (`:222-239`, `:272-288`),
  `CreatedAt = now` (`:138-140`).
- Text goes through `sanitizeStoreText` (`:150-155`) = `strings.ToValidUTF8`
  (`internal/store/store.go:87-89`), idempotent on its own output.
- Row: `run_memory_entries(entry_id PK, loop_id, ..., seq, kind, role,
  message_json, summary, first_kept_entry_id, ..., created_at,
  UNIQUE(loop_id, seq))` (`internal/store/schema.go:94-112`).
- Numbering: the next entry takes `MAX(seq)+1` of the run's table rows
  (`store/run_memory.go:272-288`); an explicit `Seq > 0` is honoured
  (`:222-238`).

### A3. Replay, projection, retirement, the floor

- Live and replay both end in `projectRunMemoryEntry`
  (`internal/store/project.go:90-91`, `:308-353`): decode, require ids and
  `seq > 0`, compare with an existing row of that id (same: no-op;
  different: `ErrProjectionMismatch`, `:319-337`), else insert verbatim. It
  accepts any `kind` string.
- A batch whose op kind is not in `projectionOpKinds`
  (`projection_batch.go:59-75`) is `ErrProjectionBatchInvalid`
  (`:385-387`; `project.go:115-116`); decode runs at every sealed read
  (`internal/computerevent/appender.go:393`, `:659`, `:745`, `:769`); a
  post-CAS failure is `ErrProjectionPoison` (`projection_batch.go:455-464`),
  reported as a wedge (`internal/computerevent/wedge.go:158-159`).
- Retirement is the op `run_memory_retire_recorded`
  (`projection_batch.go:41-44`), applied by `projectRunMemoryRetire`
  (`internal/store/run_memory_retire.go:90-114`): `DELETE ... WHERE loop_id
  IN (...)` except kept kinds. The plan is computed once
  (`selectRunMemoryPlan` `:245-464`) and appended ten runs to an op with a
  rest between ops (`RetireRunMemory` `:571-640`, `runMemoryRetireRunsPerOp`
  `:561`); `applied` is set for every id of an op once it appends
  (`:604-616`). Deletes are by run, never by row.
- The store schema epoch is `storeschema.Version = 6`
  (`internal/storeschema/schema.go:82`). `WriteFloor` raises a durable
  floor and never lowers it (`internal/storeschema/floor.go:67-107`);
  `Open` calls `CheckFloor` before any DDL and refuses a build whose epoch
  is below the floor (`internal/store/open.go:223`, `floor.go:109-119`). The
  retire path writes its floor before appending the op
  (`run_memory_retire.go:609`, `ensureProjectionBatchTrimFloor`
  `internal/store/event_index_trim.go:86-99`); note that floor is epoch 4
  (`event_index_trim.go:48`), below the current epoch 6, so it fences only
  binaries older than 4. It is a precedent for the call, not for the value.
  The updater refuses a release whose declared epoch is below the floor on
  Apply, restorePrior, restage and baseline import
  (`internal/updater/updater.go:569-600`), and a self-development base
  restore settles such a refusal as `ErrApplyRefused`
  (`internal/agentcore/selfdev_base_restore.go:76-77`).
- The product replay probe rebuilds the store from the tape into a
  disposable store and compares `run_memory_entries` rows by digest
  (`internal/agentcore/replay_completeness.go:19-52`, `:77-163`; fingerprints
  `internal/store/run_memory.go:504-530`; table class
  `ReplayEventProjection`, `internal/agentcore/replay_eligibility.go:61`).
- The main store handle may be a single pooled connection: the code says so
  at `store/run_memory.go:461-462` ("One pooled connection
  (configureEmbeddedDoltDB)"); other handles set `SetMaxOpenConns(1)`
  explicitly (`store/compact.go:408` and others). A query issued while a
  cursor is open on that connection could wait for it.

### A4. Every reader of a run's entries, with what each does with the list

All go through `ListRunMemoryEntries` (`store/run_memory.go:363-414`),
except where noted. Read this round, with the error handling and the fields
each uses:

| Reader | Error handling | Fields used | Effect of resolved `#` ids, shared/derived seq, ref `created_at` |
| --- | --- | --- | --- |
| `contextMessages`, `initialize`, `compactIfNeeded` (`run_memory.go:457-463`, `:93-113`, `:466-`) | loud | message bytes, kind, ids in `FirstKeptEntryID` (`:1145-1156`) | none, provided ids resolve |
| `planRunMemoryCompaction` and its prompt (`:700-790`, `:914-925`) | n/a | ids, **seq printed per summarized entry** (`:920-925`), role | seq must match the copy path (B1 seq block) or the summarizer input differs |
| Carry source `LatestActorRunMemoryEntries` (`store/run_memory.go:346-350`) | **returns the error**: one bad candidate fails the whole selection | list | needs "not a source" (B7) |
| Texture frame mark `textureFrameMarkForRun` (`texture_frame.go:55-64`) | **swallows the error, returns a zero mark** | message text | silent change of behaviour on a dangling ref; consumers `:187`, `capToTextureFrame` `:231-234` (call `rlm_reduce.go:837`), `:256-260`. Must fail the turn (B7) |
| Cell-author receipt `cellAuthorReceipt` (`textureowner/texture_cell_author.go:104-107`) | **swallows the error, returns "not found"** | receipt rows (not messages) | a dangling ref would look like "cell not authored yet" and re-author it. Must fail (B7) |
| Texture recovery (`textureowner/texture_controller.go:1498-1510`) | loud | `entries[len-1].EntryID` as tail id | tail is a real appended row after any ref, so its id is not a `#` id; stable. None |
| Lifecycle authority (`store/lifecycle.go:4020-4040`, `:3780-3805`; `management_controller.go:2552,2704,2777`; `tools_engineering_assignment.go:183`) | loud | `Kind`, `Role == runtime_injection`, `OwnerID`, `RunID`, `AgentID` equal to the run | **Role must be the role parsed from the bytes, never the origin row's `Role`**: an origin run's injection rows have Role `runtime_injection`; the copy path never carries one as such (`run_memory.go:157-159`, `:220`). Using the origin Role would forge delivery authority in the carrier. Resolved `RunID`/`OwnerID`/`AgentID` are the carrier's |
| `desk_shots.go:512,718,968` | loud | message bytes (image refs) | none |
| `management_tasks.go:111` | swallows (`err != nil` returns) | emptiness only | runs of management tasks never carry (A1); no reference exists in them |
| RLM: `LoadInboxCursor`, `CellFate` (`rlm_reduce.go:559-575`, `:614-630`) | loud | cursor and `cell_fate` kinds only | not message rows; a ref row is skipped by kind |
| `GetRunMemoryEntry` and the tool (`store/run_memory.go:626-651`; `tools_evidence.go:62-84`) | not found | one row by `entry_id`; run scope check | must learn `#` ids (B3); prints `seq` |
| `LatestRunMemoryEntry` (`store/run_memory.go:605-622`) | n/a | raw last row | **no non-test caller** (grep); its raw last row is never a ref, because `runtime.go:4160` prepends a run-context message that `initialize` appends after every carry (`run_memory.go:102`). Left alone |
| `ListRunMemoryEntriesForAgent` (`store/run_memory.go:448-`) | n/a | raw rows by kind | only called with the cursor kind (`rlm_reduce.go:582`); returns raw rows, not resolved. Left alone; documented |
| Non-tape path (`store/run_memory.go:42-118`) | n/a | direct `INSERT` | a `carry_ref` row inserts like any row, with the same explicit `Seq`; there is no op and no floor (a store with no tape). Same resolver |
| Hinted source (`run_memory.go:169-171`) | loud (returns `load actor conversation`) | list | a dangling error here falls through to the latest candidate (B4); other errors stay loud |
| `ListRunMemoryEntriesOG` (`store/graph_store.go:2097`) | n/a | object-graph rows | no non-test caller (grep); left alone |
| Residue import (`store/residue_import.go:437-474`) | loud | raw rows | B8 |
| Snapshot copy (`store/snapshot_to.go:395-401`, `:478-481`) | n/a | whole table kept or emptied by lease kind; **no row filter** | a carrier and its origins travel together; ids and seqs unchanged |

Writers: `AppendRunMemoryEntry` (`:31`), `AppendRunMemoryEntries` (`:163`),
the tape append (`:215`), `projectRunMemoryEntry`, residue import,
`recordCellAuthorReceipt` (`texture_cell_author.go:126-135`).

## B. The change

### B1. The reference row

A new entry kind `carry_ref` (**invented**) written in the same batch as the
checkpoint entry, in place of the copied messages; one reference per carry.
Same row type, same table, no schema change.

| Column | Value |
| --- | --- |
| `entry_id`, `loop_id`, `owner_id`, `agent_id`, `parent_entry_id`, `created_at` | as any entry |
| `kind` | `carry_ref` |
| `seq` | `b + N - 1`, defined below |
| `details_json.segments` | ordered `{run_id, owner_id, agent_id, first_seq, last_seq, count}` over origin runs |

No digest, no total count (the sum of segment counts), no message bytes.

**Seq rule, one rule, with or without a checkpoint.** Let `b` be the first
seq the copy batch would have taken for the carried messages: the carrier's
last existing seq plus one, plus one more if a checkpoint row is written
(the carrier is a fresh run, so its last existing seq is 0). Let `N` be the
number of carried messages that resolution keeps. Then:

| Row | Seq |
| --- | --- |
| checkpoint row, when it exists | `b - 1` |
| resolved message `i` (0-based), virtual | `b + i`, for `i = 0 .. N-1` |
| the reference row | `b + N - 1` |
| the next append | `b + N` (`MAX(seq) + 1`, `store/run_memory.go:272-288`) |

These are exactly the numbers the copy batch assigns (checkpoint first,
then each message), so the `seq=` the compaction prompt prints
(`run_memory.go:920-925`), `UNIQUE(loop_id, seq)` and the next append are
unchanged. The writer sets the reference's `Seq` explicitly; an explicit
`Seq > 0` is honoured (`store/run_memory.go:222-238`, tail tracking `:237`).
D1 has one seq row for "no compaction" (no checkpoint row; `b = 1`) and one
for "compaction".

**Zero kept messages.** If the kept tail is empty the writer writes the
checkpoint row if there is one and **no reference**. The model input is the
same as today: the copy path wrote only empty rows for such a tail, and
`buildRunMemoryContext` drops empty rows (`run_memory.go:1138`, `:1153`,
`:1159`). The run still has its opening messages, because
`runtime.go:4160` always prepends a run-context message to
`initialMessages` and `initialize` appends them after the carry
(`run_memory.go:102`). A reference that would resolve to zero messages is
refused by the shape check.

### B2. The op kind, the epoch and the floor

- **The op kind is selected by the entry kind.** New projection op kind
  `run_memory_carry_recorded` (**invented**), same body struct
  (`RunMemoryEntryProjection`) and decoder as the run-memory entry.
  `appendRunMemoryTapeLocked` (`store/run_memory.go:215-252`) builds the op
  per normalized entry and today always uses `ProjectionOpRunMemoryEntry`
  (`:251`); it maps `entry.Kind == carry_ref` to the new op kind and every
  other kind as before. The kind is added to `projectionOpKinds`
  (`projection_batch.go:59-75`) and to `isProjectionV2OnlyKind`
  (`:77-85`), and `projectOp` (`project.go:73-117`) routes it to
  `projectRunMemoryCarry`. The checkpoint row (old op) and the reference
  (new op) are two ops in the one `acked` slice and go in one
  `appendOps` call (`:254-261`): one append, one CAS. `opFamily` already
  returns the op kind for a kind it does not list
  (`tape_write_counters.go:89-99`), so carry ops are counted as their own
  family with no edit.
- **The projector's shape check** (no origin read): `kind = carry_ref`;
  details parse; segments non-empty; every segment's `owner_id` and
  `agent_id` equal the carrier row's; no segment's `run_id` equals the
  carrier's run; each `count` is positive and at most
  `last_seq - first_seq + 1`. A row failing it is refused at decode of the
  writer's own dry run (a refusal before the CAS), never after it.
- **Why not the existing op with a new entry kind:** that op accepts any
  kind string (`project.go:313-315`) and `buildRunMemoryContext` ignores
  kinds it does not know (`run_memory.go:1138`), so an older binary would
  silently build a conversation without the carried part. A new op kind is
  refused at decode (`projection_batch.go:385-387`).
- **Epoch and floor.** `storeschema.Version` goes from 6 to 7 and a floor of
  7 is written, durably, before the first batch containing a `carry_ref` is
  appended (same call shape as `run_memory_retire.go:609`, with a
  once-per-workspace guard like `event_index_trim.go:86-99`, and the
  compile-time guard `const _ = storeschema.Version - floorEpoch`,
  `event_index_trim.go:53`). Effect, from the code in A3: an older binary
  (epoch 6) refuses to open that store with the floor's reason
  (`open.go:223`, `floor.go:109-119`) instead of decoding the op and
  wedging. The updater refuses a release declaring an epoch below 7 on
  Apply, restorePrior, restage and baseline import
  (`updater.go:569-600`); a self-development base restore to such a release
  settles as a refusal (`selfdev_base_restore.go:76-77`). A snapshot copy
  carries the floor file (`snapshotSidecarFiles`, `snapshot_to.go:57`, copied
  at `:137-140`) and a restore or base install reopens through `Open`
  (`Reopen`, `open.go:391-405`, which runs the `CheckFloor` at `:223`), so a
  restored or copied computer on an older binary refuses with the floor's
  reason, and does not wedge. That closes the whole-computer restore case by
  reading.
- **Who decodes batches.** `DecodeProjectionBatch` and `Validate` are called
  only in `internal/computerevent` (`appender.go:393,659,745,769,1792`) and
  `internal/store` (`computer_events.go:214,276`, `project.go:58`); a search
  of `internal` and `cmd` found no other non-test caller. The platform,
  vmctl and other packages link `computerevent` but do not decode batches,
  so no non-guest process meets the new op kind (a grep, not a proof; if a
  later search finds one, the writer release waits for it).
- Two releases remain: the reader release writes no reference and no floor;
  the writer release writes both. After the writer has written, the floor is
  what stops an older build, and reverting the writer release lands on a
  build at epoch 7 that the floor admits and that reads every reference.

### B3. Resolution

One function in the store, called at the end of `ListRunMemoryEntries` and
by `GetRunMemoryEntry`; no other reader changes.

1. Read the run's rows as today, **close the cursor**, then resolve
   (`listRunMemoryEntriesTable`, `store/run_memory.go:370-414`: collect into
   a slice, `rows.Close()`, then loop). Never a query inside the
   `rows.Next` loop (single-connection risk, A3). Resolution is several
   reads, not one snapshot; origin rows are immutable once written
   (`project.go:333-335`) and are removed only by a whole-run delete, so a
   race shows as a count mismatch, not a changed byte.
2. **Position.** The resolved entries **replace the `carry_ref` row at its
   position** in the slice, so the returned slice stays in sequence order:
   `[checkpoint, resolved_0 .. resolved_{N-1}, opening messages ...]`. **The
   returned list does not contain the `carry_ref` row.** Appending the
   resolved entries at the end would put the carrier's opening turn before
   the carried history in `buildRunMemoryContext` and would put a later
   compaction row before the resolved entries, so the pre-compaction scan
   (`run_memory.go:1147-1156`) would never reach them. Readers that take the
   last entry (`texture_controller.go:1498-1510`) or count entries therefore
   see exactly the rows they see today.
3. For each segment: rows of the origin run with `loop_id = run_id`,
   `owner_id` and `agent_id` equal to the segment's, `seq BETWEEN
   first_seq AND last_seq`, `kind = message`, non-empty `message_json` (the
   filter at `run_memory.go:318`, `:324`, `buildRunMemoryContext`), in seq
   order; concatenate segments. A segment whose owner or agent is not the
   carrier's was already refused by the shape check (B2).
4. The number of messages must equal the sum of the segments' `count`;
   otherwise return `ErrRunMemoryReferenceDangling` (**invented**), wrapped
   with the carrier, the ref entry id and the origin run. Never a shorter
   list. Only a count or shape failure returns this error; a database error
   stays a plain error.
5. Emit per message a `types.RunMemoryEntry` with: carrier's `RunID`,
   `OwnerID`, `AgentID`; `Kind` message; `Message` the origin bytes,
   unchanged; **`Role` = `runMemoryMessageRole(bytes)`**, not the origin
   row's column; `Seq = b + i` (B1); `EntryID = <ref id>#<i>`
   (**invented**); `ParentEntryID` the previous resolved id (the first: the
   checkpoint row's id, or empty without a checkpoint); `CreatedAt` the
   ref's. The `#` form is a second id syntax in a table of UUIDs; it exists
   only in memory and in strings stored in the carrier's own compaction rows
   (`FirstKeptEntryID`, `raw_entry_ids`), and is documented where ids are
   described (`run_memory_ids.go`).
6. Provenance for the writer to flatten (B4): exported fields
   `OriginRunID`, `OriginSeq` on `types.RunMemoryEntry` with `json:"-"`
   (**invented**; exported because `store` fills a type of `internal/types`).
   Tape bodies are built from `RunMemoryEntryProjection`, not this struct
   (`store/run_memory.go:240-250`), so the tape is unaffected. A search of
   the `*_test.go` files for `reflect.DeepEqual` or `cmp.Diff` on entries
   found none that compares `RunMemoryEntry` structs (the hits compare id
   lists and maps: `run_memory_retire_test.go:132,176`,
   `run_memory_retire_r1_test.go:173,260,270,476,502`,
   `run_memory_census_test.go:82`); a grep, not a proof.
7. `GetRunMemoryEntry` (`:626-651`): if the id contains `#`, split, load the
   ref row, resolve, pick the index. The tool's run-scope check
   (`tools_evidence.go:62-64`) then sees the carrier's `RunID`. The tool
   prints `seq` (`:69`): for a `#` id that is the virtual seq of B1, which
   is not a row in the table and is the number the copy path would have
   stored.

### B4. The writer

`carryActorConversation` (`:165-231`) after step 4 of A1:

- **The per-message copy loop (`:219-221`) is deleted** and nothing replaces
  it as a fallback (verdict, above).
- The hinted-source read (`:169-171`) is a direct `ListRunMemoryEntries`.
  If it returns `ErrRunMemoryReferenceDangling`, the hinted run is treated
  as not a source: fall through to `LatestActorRunMemoryEntries` (`:177-186`),
  which skips unresolvable candidates (B7), and count it with the same
  counter. Any other error stays loud as today (`:170-172`). The hinted run
  was itself selected by that skipping reader when the hint was set
  (`texture_lifecycle_api.go:247`) and its own reference pins its origins
  until it is retired (B7), so the case is rare, but it is a new failure on
  a path that could not fail before, which is why it is specified.
- Keep the tail entries that resolution would keep: `kind = message`,
  non-empty bytes. This is a filter of the tail applied **after**
  `carryableRunMemoryTail`; it drops only entries the model never sees (A1),
  so no tool-pairing decision changes.
- For each kept entry take its origin: `(OriginRunID, OriginSeq)` if it was
  itself resolved, else `(its run, its seq)`. Segments are maximal runs of
  consecutive **origin message entries** in one origin run: two kept entries
  are consecutive iff no other message entry with non-empty bytes of that
  origin run lies between their origin seqs. Segments are therefore
  constructed over exactly the entries resolution keeps, which is why no
  runtime equality assertion is needed. Depth is always one: a reference
  names physical rows, never another reference. Each segment carries the
  origin rows' `owner_id` and `agent_id`.
- Checkpoint row as today (a copy: a summary, once per carry, see F4), then
  the one reference row with the seqs of B1, in one `AppendRunMemoryEntries`
  call, one tape event, one CAS (`store/run_memory.go:159-196`); the floor
  is written first (B2).
- **No write-time fallback to copying.** The first draft's assertion
  compared bytes resolved from the rows it had just read and fell back to a
  copy; it was near-tautological, wrong for uncompacted sources, and a
  second writer that hides defects. A segment-building defect is caught by
  the D1 tests, which fail on any mismatch and on a missing reference op for
  any shape. In production the carrier's own first read (`initialize` ends
  with `contextMessages`, `run_memory.go:112`) resolves the row just
  written, so a defect or a vanished origin fails that run's start with the
  named error, before any model call. `initialize` counts that failure with
  `carrier_start_failures` (**invented**) when the error is
  `ErrRunMemoryReferenceDangling`. The run fails; it is not silently
  shorter.
- Counters: `recordCarry` is called with the kept messages
  (`run_memory_carry_counters.go:56-72`), so `carried_empty_messages` is zero
  by construction after this change and stays as a regression canary;
  `carried_message_bytes` keeps counting what the model sees. Add
  `written_carry_bytes` (the body bytes of the batch) and
  `carry_refs_written` (**invented**).

### B5. Old tape stays readable forever, nothing is rewritten

Every existing carried entry is an ordinary `message` row. The resolver
runs only for `carry_ref`. No migration, no rewrite, no re-chunk. The only
code kept for old shapes is the plain row read that already exists.

### B6. Replay

`projectRunMemoryCarry` (**invented**) is `projectRunMemoryEntry` plus the
shape check of B2; it inserts the row and **never looks at the origin**.
Replay therefore cannot wedge on a missing origin and does not depend on the
order of runs (residue import orders by `owner_id, loop_id`,
`residue_import.go:437-472`). There is no projector flag and no live-only
check; round 1 (finding 3) asked which side checks, and the answer is: no
write-time check at all. A vanished origin is detected by the carrier's own
loud read (B3.3, B4) and fenced by the retire guard (B7).

### B7. Retirement keeps referenced runs; dangling handling

**The guard is at apply, in the projector's transaction.**
`projectRunMemoryRetire` (`run_memory_retire.go:90-114`) first computes the
set of runs in its list that a live `carry_ref` row names, then deletes only
the others; it **skips**, never refuses (a refusal after the CAS is
`ErrProjectionPoison`, `projection_batch.go:455-464`). It is a pure function
of the table at that point of the tape, so replay applies the same skips.
How the set is found without a scan of the whole table: carrier and origin
share owner and agent (the shape check, B2), and
`idx_run_memory_entries_kind` is `(owner_id, agent_id, kind, created_at)`
(`schema.go:356`). For each run in the op, read its distinct
`(owner_id, agent_id)` pairs (served by `idx_run_memory_entries_loop_retire`,
`schema.go:357`), then `SELECT details_json FROM run_memory_entries WHERE
owner_id=? AND agent_id=? AND kind='carry_ref'` and parse the segments.
`details_json` is not in the index, so the rows are read, not covered. **The
cost is O(references of the agent x ops in a pass)**: one document agent
gains one reference per wake, a pass is up to 50 ops
(`runMemoryRetireMaxOps`, `run_memory_retire.go:83`), and the same agent's
references can be read again in each op that names one of its runs. It is
not a table scan and it is not a cache; a cache across ops would be a second
mechanism and would go stale. The plan and the time are not measured (F6);
a pinned plan test is part of the build, as `runMemoryAgentListQuery` has.
Rows are read into a slice before the delete in the transaction.

The guard is the correctness mechanism. It closes the race round 1 found (a
pass plans origin O at T0, a carry names O at T0+5 s, O's op applies at
T0+60 s): at apply the carry's row is in the table, so O is skipped. The
other order (retire applies, then a carry names O) cannot happen for a carry
that read O's rows, because those rows were gone; the writer reads the
source first. If a retire op lands between the writer's read and its
append, the reference dangles: the carrier's own read fails loudly (B4).

**The selection-time filter is required, not optional.** Two reasons, both
from code. (1) Section E's stop condition and the census read
`kept_named_by_reference` (**invented**); without a filter every skipped
origin is counted `retired`, because `RetireRunMemory` marks every id of an
op applied once the op appends (`run_memory_retire.go:613-619`) and the
census un-retires only runs it did not append. (2) The pass cap: the plan
holds at most `runMemoryRetireMaxOps x RunMemoryRetireMaxRuns = 5,000` runs
(`:376`, test at `:398`). A pinned origin that selection still plans is
re-planned every pass and takes a slot while the guard skips it. The filter
therefore sits in the selection loop **before the cap test** (`:376-401`),
so pinned runs use no slot and cannot starve retirement behind the cap. It
reads the named-origin set once per agent per pass, using the per-pass memo
pattern the selection already has (`runRecordMemo`, `:206-239`), and the
census adds `kept_named_by_reference` beside `kept_carry_source_*`
(`:390-396`). The guard stays the safety; the filter is what makes the
census true and the cap safe.

**A dangling reference on a candidate that is not the carrier.**
`LatestActorRunMemoryEntries` (`store/run_memory.go:345-357`) reads each
candidate through the resolver and today returns the first error. It now
treats **only** `ErrRunMemoryReferenceDangling` (B3.4) as "not a source":
it increments `carry_candidates_unresolvable` (**invented**) at that skip,
logs the candidate, origin and carrier ids, and continues to the next
candidate. Any other error (a lock timeout, an I/O failure) is returned as
today, loud. Otherwise a transient store failure would make the wake carry
from an older source with no error, which changes model input silently. The
carrier's own read stays loud, and the hinted source is handled the same way
(B4).

**Readers that swallow.** `textureFrameMarkForRun` (`texture_frame.go:55-64`)
returns `(mark, error)` and its three consumers (`:187`, `rlm_reduce.go:837`,
`:256`) propagate it and fail the turn; `cellAuthorReceipt`
(`texture_cell_author.go:104-107`) likewise returns the error to its caller
instead of "not found". `management_tasks.go:111` is left as is (A4).

**`runMemoryCarryKinds` (`run_memory_carry.go:19`) is not changed.** The
group query counts `carry` rows from the table (`run_memory_retire.go:176`).
A carrier always has an appended message row: `runtime.go:4160` prepends the
run-context message to `initialMessages` and `initialize` appends them after
the carry (`run_memory.go:102`). So a carrier counts as a carry body without
`carry_ref`, the slot pass (`run_memory_retire.go:322-330`) is unchanged, and
`runMemoryHasCarryBody` (`:22-31`) sees `message` kinds in the resolved list.

### B8. Restore, fork, sibling, residue import, snapshot

A reference whose origin rows are absent from what an act emits would make
the carrier unreadable, and materialising with fresh ids would orphan
`FirstKeptEntryID` and `raw_entry_ids` (round 1). The design: **origin rows
travel with the carrier; nothing is materialised; there is no copy path.**

- **Residue import** (`residue_import.go:437-474`; it seeds the same
  store's tape from its table rows, `:142`, not a sibling): after reading the
  rows of the run scopes, for every `carry_ref` row read its segments and
  emit the origin rows in each segment's seq range that are not already in
  the emitted set. **This is new code in `residue_import.go`**: today the
  query selects by `loop_id IN (the scopes)` (`:450-453`), so an origin
  outside the scopes is not read. The origin rows go out as ordinary
  `run_memory_entry_recorded` ops with their original ids and seqs; the
  reference goes out as `run_memory_carry_recorded` with its original id and
  seq. If the origin rows in a segment's range hold fewer messages than the
  segment's `count`, the act fails with a named error, never a partial emit.
  Because this emits the new op kind, the import first writes the floor on
  `s.texturePath` through the same once-per-workspace function as the writer
  (B2), before the append at `:142`.
- **Snapshot copy and fork** (`snapshot_to.go:395-401`, `:478-481`): the
  table is kept whole or emptied whole by lease kind; carrier and origins are
  in the same table and travel together, with the floor file (B2), so a copy
  opened on an older binary refuses at `Open`.
- **Restore from a checkpoint plus the tape tail**: ops replay in tape
  order; the guard (B7) is deterministic; origin rows of a restored carrier
  are present because the guard skipped them live. A restore target between a
  carry and an unrelated retire is the same state as live at that point.
- **A compacted carrier restored into a sibling** is its own row in D2,
  comparing context bytes, since a compacted carrier's context depends on
  `FirstKeptEntryID` resolving to an id inside the reference.

## C. Failure list

"Test" means a repo test written before code (D1). "Drill" is D2.

1. **Silent change of model input** (resolved bytes differ from the copy's).
   Equality is defined on what the model is sent:
   `buildRunMemoryContext(entries)` and the compaction plan (summary input,
   `TokensBefore`, kept counts, printed seqs; ids compared by position
   because the copy path's ids are random UUIDs). Prevented by construction
   (B3.2, B4) and caught by the D1 tests.
2. **Uncompacted source with non-message rows** (receipts, `cell_fate`,
   empty entries between messages). Segments are built over the entries
   resolution keeps (B4); the test requires a **reference op** for every
   shape, so a silent never-fires cannot pass.
3. **Orphan tool results, an unanswered tool call at the tail.**
   `carryableRunMemoryTail` runs first and the kept filter is applied to its
   output (B4); segments split around any dropped message.
4. **Compacted source, `FirstKeptEntryID` mid-run.** The tail includes
   messages from the first-kept entry to the compaction and after
   (`:312-327`); segments come from the tail's actual origin entries.
5. **Runtime injections forging authority.** Resolved `Role` is parsed from
   the bytes (B3.4), `RunID`/`OwnerID`/`AgentID` are the carrier's; the
   lifecycle readers (A4) test those fields. Test: a source containing an
   injection row yields no `runtime_injection` entry in the carrier.
6. **Seq numbering differs from the copy path, with or without a
   checkpoint.** One rule (B1): checkpoint `b-1`, messages `b..b+N-1`,
   reference `b+N-1`, next append `b+N`. Two D1 rows (no compaction,
   compaction) compare printed seqs and the next append to the copy batch.
7. **Normalisation drift.** The resolver does not re-sanitise; stored text is
   already sanitised and the function is idempotent (A2). Test pins it with
   invalid UTF-8.
8. **A compacted carrier's own compaction names `#` ids.** They are stored
   in its compaction row and resolve while the ref row and origin rows exist
   (B3); the restore-into-sibling row in D2 and a test cover it.
9. **Dangling reference on the carrier's own read.** Loud error naming
   carrier, ref and origin; the run's start fails before a model call (B4).
   Wider effect, stated: every reader of that run's list fails
   (`lifecycle` authority, texture recovery, management delivery). Those
   fail the operation they serve rather than proceed on a shorter
   conversation. The default for what the wake does is F1.
10. **Dangling reference on a carry candidate.** "Not a source", counted,
    selection continues (B7). Test: a newest candidate with a vanished
    origin, an older valid one; the older is taken and the counter is 1.
11. **Swallowed errors change behaviour silently.** `textureFrameMarkForRun`
    and `cellAuthorReceipt` now propagate (B7). Test: each fails on a
    dangling carrier.
12. **Retirement races a carry.** The apply guard skips named origins
    (B7). Test: an op naming O while a ref names O leaves O's rows; replay
    of the same two ops leaves the same rows.
13. **Retire op lists the carrier and the origin together.** The guard reads
    the table before the `DELETE`, so the origin is skipped this pass and
    freed the next; deterministic.
14. **Guard cost.** O(references of the agent x ops in a pass), not a
    scan and not a cache (B7). Plan pinned by a test; not measured (F6).
15. **Census overstates `retired` for skipped runs, and pinned runs use
    cap slots.** The required selection filter, placed before the cap test,
    and `kept_named_by_reference` fix both (B7); the stop gate in E reads the
    census, so it cannot be left out.
16. **Old binary meets the new op kind.** The floor (B2) refuses to open the
    store with a named reason. Test: `WriteFloor(workspace, Version+1)` makes
    `CheckFloor` refuse; `WriteFloor(workspace, Version)` passes. Drill: both
    fences, the updater's and `Open`'s, named refusals, no wedge (D2).
17. **Product restore or automatic rollback onto a build below the floor.**
    The updater and base restore refuse (A3, B2); a snapshot copy carries the
    floor file and a restore reopens through `Open`, which refuses
    (`snapshot_to.go:57,137-140`, `open.go:223,391-405`). Closed by reading;
    the D2 older-binary row exercises it.
18. **First reference written before the floor.** The floor is written
    before the batch is appended, in the same path (B2), durable
    (`floor.go:67-107`). Test: the floor file exists before the op is
    sent; a failure writing it fails the carry.
19. **Replay before the source.** The projector never reads the origin (B6).
    Test: replay the carrier's ops with the origin's after, then before; rows
    equal; resolved lists equal.
20. **Crash mid-write.** Checkpoint and reference are one batch, one CAS,
    durable before return (`store/run_memory.go:159-163`, `:198-214`); a
    crash leaves both or neither; boot closes the obligation without
    resuming (CLAUDE.md, Restarts).
21. **Crash after the floor, before the batch.** The floor is a no-op
    effect-wise (it fences old binaries earlier than needed); harmless and
    never lowered. Stated.
22. **Fork, sibling, residue import.** Origin rows travel; a carrier whose
    origin rows are missing or short fails the act loudly (B8). Test:
    emitted set contains the origin rows outside scope; a short origin
    fails.
23. **Snapshot copy.** Whole table; no filter (A4), nothing to split.
24. **Restore to a checkpoint between two ops.** Same table state as live at
    that tape position, so the guard and resolution behave as then.
25. **Two carriers name one origin.** Allowed; the guard checks every live
    reference.
26. **A reference naming its own run, or another reference.** Rejected by
    the shape check (B2) and impossible by flattening (B4).
27. **Partial range.** Explicit `first_seq`, `last_seq`, `count`; later rows
    of a re-warmed origin are outside it; a vanished middle cannot occur
    (deletes are whole-run).
28. **Single-connection deadlock.** Resolve after `rows.Close()` (B3.1). Test
    runs the resolver on a one-connection handle.
29. **A ref that resolves to nothing makes a run look fresh.** Not written
    (B1).
30. **Compaction of the source after the carry.** Appends only; origin rows
    never change (`project.go:333-335`).
31. **A carrier compacts: its own compaction rows name resolved ids** and are
    written by the existing code unchanged (`run_memory.go:537-562`), stored
    in its own row.
32. **Counters.** `carried_message_bytes` keeps counting what the model sees
    (resolved bytes, from the writer); add `written_carry_bytes` and
    `carry_refs_written` (**invented**) so the acceptance reading (carried
    and duplicated bytes under 5% of payload) reads written bytes.
33. **Non-tape store.** Direct insert path writes the same row with the same
    explicit seq (`store/run_memory.go:42-118`); no floor needed; same
    resolver; used by tests.
34. **Redacted origin batch.** A sealed payload that no longer parses applies
    as no change (`projection_batch.go:51-54`); the reference then dangles,
    loud at the carrier and "not a source" for candidates. A copy would have
    survived. Accepted; a tool to list carriers of a to-be-redacted run is a
    named residual (F7).
35. **Read cost.** Each reference adds reads of one to three origin ranges per
    list call; `ListRunMemoryEntries` is called several times per turn
    (`contextMessages` each provider call, `run_memory.go:342-356`). Not
    measured (F6); no cache is added.
36. **Resolution position reorders the conversation.** Resolved entries
    replace the `carry_ref` row at its position; the returned list has no
    `carry_ref` row (B3.2). D1 shapes "a carry followed by the opening-message
    append" and "a carrier that compacts after the carry" fail an
    append-at-end implementation.
37. **A segment with another owner's or agent's rows.** Each segment carries
    owner and agent; the projector's shape check compares them to the carrier
    row without reading the origin; the resolver's query is scoped by both
    (B2, B3.3). Test: a wrong-owner segment is refused at the dry run.
38. **The new op kind is not selected, so an old binary silently loses the
    carried part.** `appendRunMemoryTapeLocked` maps `carry_ref` to the new op,
    which is in `projectionOpKinds` and `isProjectionV2OnlyKind` (B2). Test:
    the written batch contains the new op and `Validate` accepts it in v2 only.
39. **The hinted source dangles.** A new failure on a path that could not fail
    (`run_memory.go:169-171`); treated as not a source, falls to the latest
    candidate, counted (B4). Any other error stays loud.
40. **A transient store error mistaken for a dangling reference.** Only
    `ErrRunMemoryReferenceDangling` is skipped; a lock timeout or I/O error is
    returned (B3.4, B7).
41. **Carrier-start failure is invisible.** `carrier_start_failures` counts
    the named error at `initialize` (B4).
42. **Residue import cannot read an origin outside its run scopes.** New code
    in `residue_import.go` (B8); a short origin fails the act.
43. **The writer reaches the paper before d@e's drills.** No existing way to
    prevent it (verdict, F1); stated as an owner decision, not a switch.

## D. Proof

### D1. Repo tests, written before code

In `internal/agentcore` next to `texture_hold_trajectory_test.go` and
`run_memory_carry_counters_test.go` (the existing carry tests; their
helpers were not read) and `internal/store` next to
`run_memory_retire_r1_test.go`.

**Equality test.** For each source shape, build the same source in two
stores; carry once with the test-only copy function (the current loop
`run_memory.go:219-221`, which lives only in the test file) and once with
the reference writer. Assert:

1. `buildRunMemoryContext(ListRunMemoryEntries(carrier))` byte-equal message
   by message, **after the carry and after the opening-message append that
   `initialize` performs** (so a resolved list placed after the opening turn
   fails);
2. `planRunMemoryCompaction` gives the same summary input (entry ids
   replaced by position, since the copy path's ids are random), the same
   `TokensBefore`, kept and compacted counts and printed seqs;
3. a **seq row for "no compaction"** (no checkpoint row; messages `1..N`,
   reference `N`, next append `N+1`) and a **seq row for "compaction"**
   (checkpoint `1`, messages `2..N+1`, reference `N+1`, next `N+2`), each
   equal to the copy batch's numbers;
4. a `run_memory_carry_recorded` op was written, and its body contains no
   byte of any carried message (search the body for a marker string placed in
   every message); the returned list contains no `carry_ref` row.

Shapes (each a row; any shape that produces a copy or no reference fails):
uncompacted; compacted with `FirstKeptEntryID` mid-run; orphan result at the
head; unanswered tool call at the tail; a cell-author receipt and a
`cell_fate` row between and after messages (uncompacted); an empty message;
a run-context turn; a runtime-injection source; a source that is itself a
carrier (two, then three segments); invalid UTF-8; **a carry followed by the
run's opening-message append**; **a carrier that compacts after the carry**
(its compaction row's `FirstKeptEntryID` is a `#` id and its context must
equal the copy carrier's after the same compaction).

**Other tests:** retire guard skips a named origin and replay of the same ops
agrees; the selection filter keeps pinned runs out of the plan before the cap
and the census names them; carry candidates and the hinted source with a
dangling reference are skipped and counted, and a store error is not skipped;
`textureFrameMarkForRun` and `cellAuthorReceipt` propagate; the resolver on a
one-connection handle; `GetRunMemoryEntry` on a `#` id; the shape check
refuses a segment with a wrong owner or agent; **floor:** the floor is
written before the op is sent, `WriteFloor(workspace, Version+1)` makes
`CheckFloor` refuse with the floor's reason and `WriteFloor(workspace,
Version)` lets it pass (no "epoch 6 build" is needed, `floor.go:109-119`);
replay of the carrier's ops with the origin's after and before gives equal
rows and equal resolved lists; residue import emits out-of-scope origin rows
and fails on a short origin; the guard's query plan uses the indexes;
`appendRunMemoryTapeLocked` maps `carry_ref` to the new op and `Validate`
accepts it only in a v2 batch.

### D2. Deployed drill on d@e only (never the paper or owner computers)

After the reader release, and before the writer release reaches the paper
(F1):

| Step | Evidence | Expected |
| --- | --- | --- |
| a carrier of a carrier (three Texture runs of one document) | each run's context hash from a live read | recorded |
| replay probe: whole tape into an empty store (`replay_completeness.go`) | `run_memory.different_count`, `live_only_count`, result; the probe compares rows only (`:77-165`) and exposes no store handle, so no context hash is read from the replayed store | zero and equivalent |
| counters before and after | `carried_message_bytes` rises; `written_carry_bytes` is under 1% of it (the E relation); the carry op family shows in the tape counters | as stated |
| **compacted carrier restored into a sibling** (the carrier's compaction has `FirstKeptEntryID` inside a reference) | context hash of the carrier in the sibling vs the original | byte-equal; origin rows present in the sibling |
| **older-binary refusal**: a pinned release declaring epoch 6 on the floored d@e store (the pinned-release path, `RestagePinnedRelease` / `StageBaselineRelease` with the floor, as `selfdev_base_restore.go:62-77` uses) | **both fences**: the updater refusal (`ErrStoreFloorRefused`, `updater.go:569-600`) and, with that bypassed in a disposable copy, `Open` refusing on the floor (`open.go:223`) | two named refusals, no wedge |
| revert of the writer release to the reader release | the three runs read | unchanged |
| retirement | the scheduled wake only: `StartTelemetryRetirement` starts a run about ten minutes after boot and then every 24 hours (`internal/store/telemetry_retire.go:281-300`); no manual trigger found; runs are retirable only after 7 days (`run_memory_retire.go:70`), which a drill cannot wait for | read the census for natural `kept_named_by_reference` entries; the guard itself is proved by D1 (the window is a variable, `:70`) |

Whole-computer checkpoint restore onto an older binary needs no row: it
reopens through `Open` (`open.go:391-405`), and the floor file travels with
the copy (`snapshot_to.go:57`, `:137-140`), so it is the older-binary row.

Evidence class: deployed; local proof does not count for checkpoints,
restore, rollback (CLAUDE.md, Proof).

## E. Expected saving, with inputs, and the stop gate

Inputs: carried message bytes are 55.9% of message bytes on the tape on the
largest computer (goal file, T0; not re-measured). About 190 messages are
carried per run started (store-once design, "Where the redundancy actually
is"; not re-counted). A reference row is one row with a few segments; a
segment is three ids and three integers, a few hundred bytes (**invented**
estimate), against 190 messages each at least a row of overhead plus its
text.

- **Tape**: carried message bytes fall by about 55.9% x (1 - r), where r
  (reference bytes over copied bytes) is under 1% on these inputs, so about
  55 points of message bytes. The share of message bytes in total tape bytes
  was not measured here, so the whole-tape figure is **not stated**; the T0
  counters give it.
- **Working store**: the copied rows are not written, so live carriers are
  smaller by the same fraction. Offsetting cost: retirement keeps a whole
  origin run while any live reference names it. **Retention bound:** a
  reference names its segments forever (a carrier's later compaction does not
  rewrite it), and a carrier's rows are retired only when it is no longer
  among the agent's newest two runs and is older than 7 days
  (`run_memory_retire.go:70-73`, `:299-363`). An origin run therefore stays
  until its last carrier is retired. If a carrier's tail spans m earlier runs
  at the carry, those m runs are held until two generations after that
  carrier, so an origin is held about m + 2 generations behind the newest
  run. m is bounded by the compaction ceiling (`run_memory.go:792-800`, 90,000
  prompt tokens) and is **not measured**; "about three generations" is the
  case m = 1.
- **Stop gate.** The gate is: added retention (bytes of origin runs held
  only because a reference names them, read as the census count and bytes of
  `kept_named_by_reference`) must be under the carried bytes saved; if it is
  not, the design stops and carry stays as it is. The gate is read on d@e
  before the writer reaches the paper (F1). **Without any writer** it can be
  estimated read-only today on d@e and the paper: for each document agent's
  newest two runs, count how many earlier runs' own messages still lie inside
  that run's carried tail (carried messages are in order and each run's own
  message count is known from its entries), which is m. That estimate is
  proposed, not built, and uses no switch.

## F. Open questions, with a conservative default

F1. **How can the writer release be seen on d@e before the paper receives
    it?** No existing way (verdict, cost section): soak floors are zero and
    every landing carries `Ring-Expedite`. Options for the owner, none
    invented here: restore a soak floor for that release (a commit to
    `ringSoaks`, `ring_walk.go:86`), or land the writer without
    `Ring-Expedite`, which still walks the rings within minutes. Default:
    the writer release is not landed until the owner chooses; the reader
    release and the read-only retention estimate go first.
F2. *(closed by reading in round 2.)* A snapshot copy carries the floor file
    and every restore reopens through `Open`, which runs `CheckFloor`
    (`snapshot_to.go:57,137-140`, `open.go:223,391-405`). The conservative
    default was right; no open question remains.
F3. Do the Texture frame and cell-author readers have any caller that relies
    on the swallowed error (a path that must not fail the turn)? Default:
    they fail it (B7).
F4. Should the checkpoint entry also be a reference? Default: no (one per
    carry, small; its `TokensBefore` is part of the rendered bytes,
    `run_memory.go:196-199`, `:1355-1367`). Measure it with the first counter
    reading.
F5. *(closed by reading in round 2.)* `runtime.go:4160` prepends a run-context
    message to `initialMessages`, which `initialize` appends after the carry
    (`run_memory.go:102`); a carrier always has a message row (B7).
F6. Plans and costs: the guard's two reads and the resolver's range reads
    were not planned or timed. Default: pin plans in tests; time on d@e
    before the writer release.
F7. A tool to list carriers that name a run about to be redacted. Default:
    none; named residual (failure 34).
F8. *(closed by reading in round 2, as a grep.)* Only `computerevent` and
    `store` decode batches (B2). If a later search finds another decoder, it
    is a precondition of the writer release.
F9. Names (`carry_ref`, `run_memory_carry_recorded`, `#i` ids,
    `ErrRunMemoryReferenceDangling`, `kept_named_by_reference`, the
    counters, `OriginRunID`/`OriginSeq`, the segment-size estimate) are
    invented; the seats may rename them.

## What I could not verify

Nothing was compiled or run. The 55.9% and 190-per-run figures are from the
goal file and the store-once design. I did not read the existing carry
tests' helpers, any query plan, or the pinned-release path's exact entry
points for the older-binary drill row (the code cited shows the calls the
base restore makes, not a ready drill). The "no other decoder" statement is
a grep. m (E) is unmeasured.

## Round 1 findings and what was done

Seats: DS = the DeepSeek reviewer, CL = the second reviewer
(`claude.out`). "Adopt" means the design now says it; "reject" has
evidence.

| # | Seat, kind | Finding | Verified against | Decision |
| --- | --- | --- | --- | --- |
| 1 | DS blocking 1 | retirement race: plan at T0, carry passes the check, op applies later | `run_memory_retire.go:571-640`, `:90-114` (no guard) | Adopt: apply-time guard, skip not refuse (B7); the carry-time origin check is deleted |
| 2 | DS blocking 2 | assertion contradicts the filter; uncompacted sources never get a reference | `run_memory.go:331`, `:219-221`, `texture_cell_author.go:126-135`, `rlm_reduce.go:653`, `:1138` | Adopt: segments over kept entries; no assertion; equality on `buildRunMemoryContext` and the plan; D1 requires a reference op for every shape (B4, C2) |
| 3 | DS blocking 3, CL blocking 2 | restore/fork prevention has no mechanism and the remedy breaks `first_kept_entry_id` | `snapshot_to.go:395-401`, `:478-481`; `residue_import.go:437-472`; `run_memory.go:543` | Adopt: origin rows travel with the carrier, no materialisation, loud failure on a short origin; compacted-carrier-into-sibling row in D2 (B8) |
| 4 | DS blocking 4, CL blocking 3 | one dangling ref poisons source selection | `store/run_memory.go:345-357` returns the error | Adopt: candidate is "not a source", counted; carrier's own read loud (B7) |
| 5 | DS blocking 5, CL blocking 1 | no durable floor; prose rule conflicts with restore and rollback | `floor.go:67-119`, `open.go:223`, `updater.go:569-600`, `selfdev_base_restore.go:76` | Adopt: epoch 7 and floor before the first reference; prose rule dropped (B2). Correction to the finding's precedent: the retire op's floor is epoch 4, below the current 6 (`event_index_trim.go:48`), so the value is new, not copied. Whole-computer restore closed by reading in round 2 (F2) |
| 6 | DS blocking 6 | `texture_frame.go:55-64` swallows the error | read; consumers `:187`, `:231-234`, `:256-260` | Adopt (B7). Also found `cellAuthorReceipt` swallowing (`texture_cell_author.go:104-107`) |
| 7 | CL blocking 4 | write-time fallback is a second writer hiding defects | B3 of draft 1 | Adopt: removed (B4). The per-message copy loop in `carryActorConversation` is deleted; `messageEntry` stays for `appendMessage(s)`; the checkpoint row is still copied; residue import has no copy path; the comparison copy lives only in the D1 test file (verdict) |
| 8 | CL blocking 3 (F9 inconsistent) | B4 and F9 disagreed on where the origin check lives | draft 1 | Adopt by deletion: no write-time check anywhere (B6) |
| 9 | DS should-fix 1 | unexported provenance fields cannot compile across packages | `internal/types/task.go:488-505` | Adopt: exported `OriginRunID`, `OriginSeq`, `json:"-"` (B3.5) |
| 10 | DS should-fix 2 | `carry_ref` in `runMemoryCarryKinds` is a no-op that perturbs the slot pass | `run_memory_retire.go:322-330`, `:176`; resolved list sees `message` | Adopt: dropped; effect stated (B7, F5) |
| 11 | DS should-fix 3 | F6 understates the scan cost | `schema.go:356-357` | Adopt: guard reads one agent's refs via the kind index; plans not run (B7, F6) |
| 12 | DS should-fix 4 | D2 step 5 not runnable | `telemetry_retire.go:265-285`; window `run_memory_retire.go:70` | Adopt: scheduled-wake census only; guard proved in D1 (D2) |
| 13 | DS should-fix 5 | census lies after the guard | `run_memory_retire.go:604-616` | Adopt: stated; the selection filter is required (round 2), with `kept_named_by_reference` (B7, C15) |
| 14 | DS should-fix 6, CL should-fix 1 | resolved entries share one seq; tool prints seq; compaction prompt prints seq | `tools_evidence.go:69`, `run_memory.go:920-925` | Adopt, stronger than asked: the seq block reproduces the copy path's numbering exactly (B1, C6) |
| 15 | DS should-fix 7 | a ref resolving to zero looks fresh | `run_memory.go:94-101`, `management_tasks.go:111` | Adopt: never written; shape check refuses (B1, C29) |
| 16 | CL should-fix 1 | readers not read | read this round | Adopt: table A4, one row per reader; recovery read compares a real appended row's id (safe) |
| 17 | CL should-fix 2 | `GetRunMemoryEntry`, `LatestRunMemoryEntry` | `store/run_memory.go:605-651` | Adopt for Get (B3.6). `LatestRunMemoryEntry` has no non-test caller; left alone |
| 18 | CL should-fix 3 | single-connection deadlock | `store/run_memory.go:461-462`, `compact.go:408` | Adopt: resolve after `rows.Close()` (B3.1, C28) |
| 19 | CL should-fix 4 | non-tape path | `store/run_memory.go:42-118` | Adopt: row inserts the same way (A4, C33) |
| 20 | CL should-fix 5 | invented names and numbers not all marked | draft 1 | Adopt: all marked; the digest, family line and probe extension removed instead |
| 21 | CL should-fix 6 | retention keeps the whole origin; give a number | B7 | Adopt in part: formula and inputs given in E; absolute whole-tape figure not stated because the byte share was not measured, and the design says it stops if retention exceeds saving |
| 22 | DS nit | `opFamily` already names unknown kinds | `tape_write_counters.go:89-99` | Adopt: family line removed (B2) |
| 23 | DS nit | D1 bound unnamed | draft 1 | Adopt: 128 bytes per segment, marked invented (D1) |
| 24 | DS nit | `#index` is a second id syntax | `run_memory_ids.go` | Adopt: documented where ids are described (B3.4) |
| 25 | DS nit | "one mechanism" understated | draft 1 | Adopt: the verdict section lists the seven pieces plainly |
| 26 | DS nit | segment `count` duplicates top-level `count` | draft 1 | Adopt: total count dropped |
| 27 | CL should-fix 5 | cut the digest and probe extension | draft 1 | Adopt: both removed; count mismatch detects a missing origin (B3.3) |
| 28 | CL nit | failure 12 cites `:200-201` | `run_memory.go:200-222` | Adopt: cites corrected |
| 29 | CL nit | F8 should be settled by a test, not a counter | `run_memory.go:331` | Adopt: D1 shape "uncompacted with receipt and `cell_fate`" |
| 30 | CL nit | `kept_carry_source_*` anchor off by about 40 lines | `run_memory_retire.go:390-396` at this commit | Reject as stated: at `9883c37a2` the census adds are at `:390-396`; the reviewer read a different tree (`842e0fda6`). Anchors re-checked, not changed |

## Round 2 findings and what was done

Seats as before (DS, CL). Verified against the code before adoption.

| # | Seat, kind | Finding | Verified against | Decision |
| --- | --- | --- | --- | --- |
| 1 | DS blocking 1 | resolution position unspecified; append-at-end reorders and drops kept history after a later compaction | `run_memory.go:93-112`, `:1128-1162`, `runtime.go:4160` | Adopt: replace the ref at its position, returned list has no `carry_ref` row (B3.2); D1 shapes added (C36) |
| 2 | DS blocking 2, CL blocking 2 | copy writer stated three ways | `residue_import.go:450-474`, `run_memory.go:416-445` | Adopt: one statement everywhere; six pieces (verdict, B4, B8, round 1 row 7) |
| 3 | DS blocking 3 | shape check claims owner/agent equality without carrying them | B1, B2, B6 of rev 2 | Adopt: segments carry owner and agent; shape check compares to the carrier; resolver scoped (B1-B3, C37) |
| 4 | DS blocking 4 | the new op kind is never selected | `store/run_memory.go:215-252`, `projection_batch.go:59-85` | Adopt (B2, C38) |
| 5 | CL blocking 1, DS should-fix | seq block undefined without a checkpoint | `run_memory.go:203`, `:331` | Adopt: one rule `b-1`, `b..b+N-1`, `b+N-1`, `b+N` (B1); two D1 seq rows |
| 6 | CL should-fix | `ListRunMemoryEntries` returns the ref row or not | `texture_controller.go:1498-1510` | Adopt: not returned (B3.2) |
| 7 | DS should-fix | "not a source" cannot tell a dangling ref from a store failure | `store/run_memory.go:346-350` | Adopt: `ErrRunMemoryReferenceDangling` only; counter at the skip (B3.4, B7, C40) |
| 8 | DS should-fix, CL should-fix | selection filter called optional; E depends on it; cap | `run_memory_retire.go:376-401`, `:613-619` | Adopt: required, before the cap test; pinned runs use no slot (B7, C15) |
| 9 | DS should-fix | floor proof steps cannot run as written | `floor.go:109-119` | Adopt: `WriteFloor(Version+1)`/`CheckFloor`; D2 names the pinned-release path and both fences |
| 10 | DS should-fix | F2 can be closed by reading | `snapshot_to.go:57,137-140`, `open.go:223,391-405` | Adopt: F2 closed; C17 reworded |
| 11 | DS should-fix | D2 context hashes need the cut probe extension | `replay_completeness.go:77-165` | Adopt: row dropped; context equality is D1 and the sibling-restore row |
| 12 | DS should-fix | guard cost is O(refs x ops) | `run_memory_retire.go:83`, `schema.go:356` | Adopt: stated (B7, C14); no cache |
| 13 | DS should-fix | F5 is provable | `runtime.go:4160`, `run_memory.go:102` | Adopt: F5 closed, cited (B7) |
| 14 | DS should-fix, CL should-fix | F8 platform decode | grep: decoders only in `computerevent` and `store` | Adopt: closed as a grep, precondition if a decoder is found (B2, F8) |
| 15 | DS should-fix | zero-kept source behaviour undefined; draft claim "copy writes nothing" false | `store/run_memory_carry.go:22-31`, `run_memory.go:219-221` | Adopt: checkpoint only, no reference; same model input (B1) |
| 16 | DS should-fix | residue import's floor handle; wording conflates same-store backfill and fork | `residue_import.go:142` | Adopt: floor via the same function on `s.texturePath`; wording fixed (B8) |
| 17 | DS should-fix | `written_carry_bytes` row cannot fail; `carried_empty_messages` | `run_memory_carry_counters.go:51-72` | Adopt: assert the E relation (under 1%); the empty counter stays as a canary, zero by construction (B4, D2) |
| 18 | CL should-fix | hinted source can dangle; counter for carrier-start failures | `run_memory.go:169-171` | Adopt (B4, C39, C41) |
| 19 | CL should-fix | guard query not covered by the index | `schema.go:356` | Adopt: wording fixed, rows read not covered (B7) |
| 20 | CL should-fix | pinned runs re-planned each pass cannot starve | cap `run_memory_retire.go:83,376,398` | Adopt, with the reason: the filter precedes the cap test, so they use no slot (B7) |
| 21 | CL should-fix | state the retention bound | `run_memory_retire.go:70-73,299-363` | Adopt in part: stated as m + 2 generations; "about three" holds only for m = 1; m unmeasured (E). The reviewer's "about three generations" is not adopted as a general figure |
| 22 | CL should-fix | who else links `computerevent` and decodes | grep `DecodeProjectionBatch`, `Validate` | Adopt (B2) |
| 23 | CL should-fix | list tests comparing `RunMemoryEntry` structs | grep of `*_test.go` | Adopt: none found; hits compare id lists and maps (B3.6) |
| 24 | CL should-fix | residue expansion is new code | `residue_import.go:450-453` | Adopt (B8, C42) |
| 25 | CL nit | `ListRunMemoryEntriesOG` | `graph_store.go:2097` | Adopt (A4) |
| 26 | CL nit | virtual seqs in `GetRunMemoryEntry` | `tools_evidence.go:69` | Adopt (B3.7) |
| 27 | DS nit | cites: telemetry path, applied marking, residue rows | `telemetry_retire.go:281-300`, `run_memory_retire.go:613-619`, `residue_import.go:450-474` | Adopt, re-anchored |
| 28 | DS nit | `opFamily` test trivial | `tape_write_counters.go:89-99` | Adopt: test dropped; mapping test kept |
| 29 | DS nit | 128-byte bound brittle | D1 of rev 2 | Adopt: replaced by "no message byte in the body" |
| 30 | DS nit | `LatestRunMemoryEntry` claim should cite the run-context turn | `runtime.go:4160` | Adopt (A4) |
| 31 | DS verdict | seven pieces overcounts; filter required | verdict | Adopt: six pieces |
| 32 | DS and CL verdict | owner must accept the one-way floor; d@e first | `floor.go:67-107`; ring walk | Adopt: cost section and order added at the top; no existing way to hold the writer on d@e, stated as F1 |
