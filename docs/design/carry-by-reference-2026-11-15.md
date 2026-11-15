# Carry by reference: design section and failure list for T1

Written 2026-11-15 on lane `lane/t1-carry-design`; revision 2 after round 1
(two seats, both REPAIR). Design only: no code and no tests in this change.
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

The mechanism can be made safe, but **not as one small mechanism**. Round 1
showed that "a reference plus a resolver" is the visible part and that it
needs these other pieces to be safe, each required by a finding and each
verified against code below:

1. the reference row and its resolver in the store's list function;
2. a new tape op kind, and a store-schema epoch and floor written before
   the first reference (so an older binary refuses to open instead of
   wedging);
3. an apply-time guard in the retire projector (skip runs a live reference
   names);
4. a seq-block rule so numbering matches the copy path;
5. "dangling reference" handled in three named places (the carrier's own
   read is loud; a carry candidate is "not a source"; two readers that now
   swallow errors must stop);
6. residue import expanding a reference to its origin rows;
7. the copy writer kept, but only for the one place that still needs it
   (see B5).

Everything else the first draft carried (a stored digest, a write-time
assertion with a copy fallback, a probe extension, a new counter family, a
`carry_ref` entry in the carry-kinds list, an origin check in the
projector) was removed. If the owner judges seven pieces too many for a
saving of the size in section E, the alternative is to leave carry as it is
and put the effort into T2; this document does not recommend that, because
the carried share is the largest single redundancy measured.

Invented names and numbers are marked **invented**. Everything else is read
from the cited line. Nothing was compiled or run.

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
| `LatestRunMemoryEntry` (`store/run_memory.go:605-622`) | n/a | raw last row | **no non-test caller** (grep); it would return the raw last row, which is a real appended row, never a ref, because the writer always appends after a carry. Left alone |
| `ListRunMemoryEntriesForAgent` (`store/run_memory.go:448-`) | n/a | raw rows by kind | only called with the cursor kind (`rlm_reduce.go:582`); returns raw rows, not resolved. Left alone; documented |
| Non-tape path (`store/run_memory.go:42-118`) | n/a | direct `INSERT` | a `carry_ref` row inserts like any row, with the same explicit `Seq`; there is no op and no floor (a store with no tape). Same resolver |
| Residue import (`store/residue_import.go:437-472`) | loud | raw rows | B8 |
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
| `seq` | the **last** seq of a block of N reserved numbers (below) |
| `details_json.segments` | ordered `{run_id, first_seq, last_seq, count}` over origin runs of the same owner and agent |

No digest, no total count (the sum of segment counts), no message bytes.

**Seq block (copy-path numbering, retained).** The checkpoint entry takes
`s0`. The reference takes `s0 + N` where N is the number of resolved
messages, and the resolved messages are numbered `s0+1 .. s0+N`. The next
appended entry takes `MAX(seq)+1 = s0+N+1`, exactly the number the copy path
gives it (`store/run_memory.go:272-288`, explicit `Seq` honoured at
`:222-238`). This keeps the `seq=` the compaction prompt prints
(`run_memory.go:920-925`) and `UNIQUE(loop_id, seq)` unchanged. It is a
small rule rather than a mechanism, and the test D1 pins it.

**Zero messages:** if the kept tail is empty the writer writes no
reference (the copy path writes nothing for an empty tail either). A
reference that resolves to zero messages is refused by the shape check, so
`initialize`'s "has entries" test (`run_memory.go:94-101`) and
`management_tasks.go:111` never see a run that looks fresh and re-carries.

### B2. The op kind, the epoch and the floor

- New projection op kind `run_memory_carry_recorded` (**invented**), same
  body struct and decoder as the run-memory entry, same insert (shape check:
  `kind = carry_ref`, details parse, segments non-empty, no segment names the
  carrier's own run, same owner and agent as the carrier).
  `opFamily` already returns the op kind for a kind it does not list
  (`tape_write_counters.go:89-99`), so carry ops are counted as their own
  family with no edit.
- Why not the existing op with a new entry kind: it accepts any kind
  string (`project.go:313-315`) and `buildRunMemoryContext` ignores kinds it
  does not know (`run_memory.go:1138`), so an older binary would silently
  build a conversation without the carried part. A new op kind is refused at
  decode.
- **Epoch and floor.** `storeschema.Version` goes from 6 to 7 and a floor of
  7 is written, durably, before the first batch containing a `carry_ref` is
  appended (same call shape as `run_memory_retire.go:609`, with a once-per
  workspace guard like `event_index_trim.go:86-99`, and the compile-time
  guard `const _ = storeschema.Version - floorEpoch`
  (`event_index_trim.go:53`)). Effect, from the code in A3: an older binary
  (epoch 6) refuses to open that store with the floor's reason
  (`open.go:223`, `floor.go:109-119`), instead of decoding the op and
  wedging. The updater refuses a release declaring an epoch below 7 on
  Apply, restorePrior, restage and baseline import
  (`updater.go:569-600`); a self-development base restore to such a release
  settles as a refusal (`selfdev_base_restore.go:76-77`). A floor never
  lowers (`floor.go:67-107`), so the way back below 7 is a manual recipe like
  the header/body split's (`schema.go` epoch 6 note), not a revert. **What a
  product restore (checkpoint restore of the whole computer) does when the
  floor is above its target binary was not read** (question F2); the
  conservative reading of the code is that opening the store refuses with a
  named reason.
- Two deploys remain: deploy 1 ships epoch 7, the reader, the projector, the
  retire guard and writes no reference (so no floor); deploy 2 ships the
  writer. This is not a prose rule: after deploy 2 writes, the floor is what
  stops an older build, and reverting deploy 2 lands on a build at epoch 7
  that the floor admits and that reads every reference.

### B3. Resolution

One function in the store, called at the end of `ListRunMemoryEntries` and
by `GetRunMemoryEntry`; no other reader changes.

1. Read the run's rows as today, **close the cursor**, then resolve
   (`listRunMemoryEntriesTable`, `store/run_memory.go:370-414`: collect into
   a slice, `rows.Close()`, loop). Never a query inside the `rows.Next`
   loop (single-connection risk, A3). Resolution is several reads, not one
   snapshot; origin rows are immutable once written (`project.go:333-335`)
   and are removed only by a whole-run delete, so a race shows as a count
   mismatch, not a changed byte.
2. For each segment: rows of the origin run with
   `seq BETWEEN first_seq AND last_seq`, `kind = message`, non-empty
   `message_json` (the filter at `run_memory.go:318`, `:324` and
   `buildRunMemoryContext`), in seq order; concatenate segments.
3. The number of messages must equal the sum of the segments' `count`;
   otherwise return an error naming the carrier, the ref entry id and the
   origin run. Never return a shorter list.
4. Emit per message a `types.RunMemoryEntry` with: carrier's `RunID`,
   `OwnerID`, `AgentID`; `Kind` message; `Message` the origin bytes,
   unchanged; **`Role` = `runMemoryMessageRole(bytes)`**, not the origin
   row's column; `Seq = s0 + 1 + i` (B1); `EntryID = <ref id>#<i>`
   (**invented**); `ParentEntryID` the previous resolved id (the first:
   the checkpoint entry's id); `CreatedAt` the ref's. The `#` form is a
   second id syntax in a table of UUIDs; it exists only in memory and in
   strings stored in the carrier's own compaction rows
   (`FirstKeptEntryID`, `raw_entry_ids`). It is documented where ids are
   described (`run_memory_ids.go`).
5. Provenance for the writer to flatten (B4): exported fields
   `OriginRunID`, `OriginSeq` on `types.RunMemoryEntry` with `json:"-"`
   (**invented**; exported because `store` fills a type of `internal/types`;
   marshalled bodies are built from `RunMemoryEntryProjection`, not this
   struct, `store/run_memory.go:240-250`, so the tape is unaffected; tests
   that compare structs with `reflect.DeepEqual` will see them).
6. `GetRunMemoryEntry` (`:626-651`): if the id contains `#`, split, load the
   ref row, resolve, pick the index; the run-scope check in the tool
   (`tools_evidence.go:62-64`) then sees the carrier's `RunID`. The tool
   prints the block seq (`:69`).

### B4. The writer

`carryActorConversation` (`:165-231`) after step 4 of A1:

- Keep the tail entries that resolution would keep: `kind = message`,
  non-empty bytes. This is a filter of the already-read tail, applied
  **after** `carryableRunMemoryTail`; it drops only entries the model never
  sees (A1), so no tool-pairing decision changes.
- For each kept entry take its origin: `(OriginRunID, OriginSeq)` if it was
  itself resolved, else `(its run, its seq)`. Segments are maximal runs of
  consecutive **origin message entries** in one origin run: two kept entries
  are consecutive iff no other message entry of that origin run, with
  non-empty bytes, lies between their origin seqs. Segments are therefore
  constructed over exactly the entries resolution keeps, which is why no
  runtime equality assertion is needed. Depth is always one: a reference
  names physical rows, never another reference.
- Checkpoint entry as today (a copy: a summary, once per carry; see F4),
  then the one reference entry with the block seq, in one
  `AppendRunMemoryEntries` call, one tape event, one CAS
  (`store/run_memory.go:159-196`); with the floor written first (B2).
- **No write-time fallback to copying.** The earlier draft's assertion
  compared bytes resolved from the rows it had just read and fell back to a
  copy; it was (a) near-tautological, (b) wrong for uncompacted sources
  (round 1, finding 2) and (c) a second writer that hides defects. It is
  removed. A segment-building defect is caught by the tests in D1, which
  fail on any mismatch and on a missing reference op for any shape; in
  production the carrier's own first read (`initialize` ends with
  `contextMessages`, `run_memory.go:112`) resolves the row just written, so
  a defect or a vanished origin fails that run's start with a named error,
  before any model call. The run fails; it is not silently shorter.
- The copy writer (`messageEntry` per entry) is **kept only for residue
  import** (B8), and does not run in `carryActorConversation`.

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
How the set is found without a table scan: the ref row's carrier and origin
share owner and agent (shape check, B2), and `idx_run_memory_entries_kind`
is `(owner_id, agent_id, kind, created_at)` (`schema.go:356`). For each run
in the op, read its distinct `(owner_id, agent_id)` pairs (served by
`idx_run_memory_entries_loop_retire`, `schema.go:357`), then
`SELECT details_json FROM run_memory_entries WHERE owner_id=? AND
agent_id=? AND kind='carry_ref'` and parse the segments. That is one small
read per agent (references of a document agent number one per wake), not a
scan of 5.1M rows. **Query plans were not run** (F6); a pinned plan test is
part of the build, as `runMemoryAgentListQuery` has. Rows are read into a
slice before any delete in the transaction.

The guard is the correctness mechanism. It closes the race round 1 found
(a pass plans origin O at T0, a carry names O at T0+5 s, O's op applies at
T0+60 s): at apply the carry's row is in the table, so O is skipped. The
other order (retire applies, then a carry names O) cannot happen for a
carry that read O's rows, because those rows were gone; the writer reads
the source first (`:169`/`:179`), so it carries nothing from O. If a retire
op lands between the writer's read and its append, the reference dangles:
the carrier's own read fails, loudly (B4), and that run is not a source
(next).

**Selection-time filtering is an optimisation**: `selectRunMemoryPlan` may
call the same helper so the census says `kept_named_by_reference`
(**invented**) rather than counting a skipped run as retired. Without it the
census overstates `retired` for skipped runs, because `RetireRunMemory`
marks every id of an op applied once it appends (`:604-616`). The first
build may omit the optimisation and say so in the census notes; the guard
alone is sufficient for safety.

**A dangling reference on a carry candidate that is not the carrier.**
`LatestActorRunMemoryEntries` (`store/run_memory.go:345-357`) reads each
candidate through the resolver and today returns the first error. It
treats a resolution error on a candidate as "not a source": count it
(**invented** counter `carry_candidates_unresolvable`), log the candidate,
origin and carrier ids, continue to the next candidate. The carrier's own
read stays loud. Without this, one dangling ref would fail every later
wake of the document (round 1 findings 4 and Claude 3).

**Readers that swallow.** `textureFrameMarkForRun` (`texture_frame.go:55-64`)
returns `(mark, error)` and its three consumers (`:187`, `rlm_reduce.go:837`,
`:256`) propagate it and fail the turn; `cellAuthorReceipt`
(`texture_cell_author.go:104-107`) likewise returns the error to its caller
instead of "not found". `management_tasks.go:111` is left as is (A4).

`runMemoryCarryKinds` (`run_memory_carry.go:19`) is **not** changed. The
group query counts `carry` rows from the table (`run_memory_retire.go:176`),
where a carrier always has a compaction row or at least one appended message
row (`initialize` appends the run's opening messages after the carry,
`run_memory.go:102`); adding `carry_ref` would only change the slot pass
(`run_memory_retire.go:322-330`) for a carrier with neither, which cannot
occur if a Texture run always has opening messages (not verified, F9). The
resolved list sees `message` kinds, so `runMemoryHasCarryBody` (`:22-31`) is
unaffected.

### B8. Restore, fork, sibling, residue import, snapshot

A reference whose origin rows are absent from what an act emits would make
the carrier unreadable, and materialising with fresh ids would orphan
`FirstKeptEntryID` and `raw_entry_ids` (round 1). The design: **origin rows
travel with the carrier; nothing is materialised.**

- **Residue import** (`residue_import.go:437-472`): after reading the rows
  of the run scopes, for every `carry_ref` row read its segments and emit
  the origin rows in each segment's seq range (any kind in range; the
  resolver filters) that are not already in the emitted set, as ordinary
  `run_memory_entry_recorded` ops with their original ids and seqs; the ref
  itself goes out as `run_memory_carry_recorded` with its original id and
  seq. **If the origin rows in a segment's range are fewer than the
  segment's `count` messages, the act fails with a named error**, never a
  partial emit. The sibling's tape then holds the origin rows and the ref in
  any order (B6). Ids are unchanged, so `#` ids stored in compaction rows
  keep resolving. Because this emits the new op kind, the import writes the
  floor in the target workspace first (B2).
- **The copy writer** is therefore not needed for residue import either. It
  is kept for nothing, and is deleted when the reference writer lands, with
  one exception: if the owner judges a sibling must open on a binary below
  epoch 7 (F2), residue import would need the old copy form with synthetic
  ids and rewritten `FirstKeptEntryID`; not designed, because the epoch rule
  says no such binary opens a store holding references.
- **Snapshot copy** (`snapshot_to.go:395-401`, `:478-481`): the table is kept
  whole or emptied whole by lease kind; carrier and origins are in the same
  table, so they travel together. Whether the floor file travels with a
  copied store was not read (F2).
- **Restore from checkpoint plus tape tail**: ops replay in tape order; the
  guard (B7) is deterministic; origin rows of a restored carrier are present
  because the retire guard skipped them live. A restore to a target between
  a carry and an unrelated retire is the same state as live at that point.
- **Fork/sibling "restore a compacted carrier"** is its own row in the
  proof table (D2) comparing context bytes, since a compacted carrier's
  context depends on `FirstKeptEntryID` resolving to an id inside the
  reference.

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
6. **Seq numbering differs from the copy path.** The block rule (B1); test
   compares printed seqs in the compaction input and the next appended seq.
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
14. **Guard cost.** One indexed read per agent per op (B7), not a scan.
    Plan pinned by a test; not measured (F6).
15. **Census overstates `retired` for skipped runs.** Known; the optional
    selection filter and `kept_named_by_reference` fix it; stated, not
    safety-relevant.
16. **Old binary meets the new op kind.** The floor (B2) refuses to open the
    store with a named reason. Test: a store with floor 7 refuses
    `CheckFloor` with epoch 6. Drill: confirm the refusal, not a wedge.
17. **Product restore or automatic rollback onto a build below the floor.**
    The updater and base restore refuse (A3, B2). Whole-computer checkpoint
    restore: not read (F2); drill row.
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

## D. Proof

### D1. Repo tests, written before code

In `internal/agentcore` next to `texture_hold_trajectory_test.go` and
`run_memory_carry_counters_test.go` (the existing carry tests; their
helpers were not read) and `internal/store` next to
`run_memory_retire_r1_test.go`.

**Equality test.** For each source shape, build the same source in two
stores; carry once with a test-only copy function (the current code, kept in
the test file) and once with the reference writer. Assert: (1)
`buildRunMemoryContext(ListRunMemoryEntries(carrier))` byte-equal message by
message; (2) `planRunMemoryCompaction` gives the same summary input (with
ids replaced by position), `TokensBefore`, kept and compacted counts and
printed seqs; (3) the next appended entry has the same seq; (4) **a
`run_memory_carry_recorded` op was written and no message bytes** (details
under **128 bytes per segment**, **invented** bound; a table row has at least
the 16 columns of `schema.go:94-112`). Shapes: no compaction; compaction
with `FirstKeptEntryID` mid-run; orphan result at the head; unanswered
tool call at the tail; a cell-author receipt and a `cell_fate` row between
and after messages (uncompacted); empty message; run-context turn; runtime
injection source; a source that is itself a carrier (two, then three
segments); invalid UTF-8. Any shape producing a copy or no reference fails.

**Other tests:** retire guard skips a named origin and replay of the same ops
agrees; carry candidates with a dangling ref are skipped and counted;
`textureFrameMarkForRun` and `cellAuthorReceipt` propagate; resolver on a
one-connection handle; `GetRunMemoryEntry` on a `#` id; floor written before
the op and `CheckFloor` at epoch 6 refuses; replay order both ways; residue
import emits out-of-scope origin rows and fails on a short origin;
`opFamily` names the carry op; the guard's query plan uses the indexes.

### D2. Deployed drill on d@e only (never the paper or owner computers)

Deploy 1, then deploy 2. Rows of the table, each with its evidence:

| Step | Evidence | Expected |
| --- | --- | --- |
| a carrier of a carrier (three Texture runs of one document) | context hash of each run's `contextMessages` | recorded |
| replay probe: whole tape into an empty store (`replay_completeness.go`) | `run_memory.different_count`, `live_only_count`, result | zero and equivalent |
| the same three runs' context hashes read from the replayed store | hash list | equal to the live ones |
| counters before and after | `carried_message_bytes` rises, `written_carry_bytes` does not; op counters show the carry family | as stated |
| **compacted carrier restored into a sibling** (carrier whose compaction has `FirstKeptEntryID` inside a reference) | context hash of the carrier in the sibling vs the original | byte-equal; origin rows present in the sibling |
| older binary against the store | what happens | refusal at open citing the floor (not a wedge) |
| redeploy deploy 1 reader after a revert of deploy 2 | the three runs read | unchanged |
| retirement | the scheduled wake only: the retire run starts about ten minutes after boot, then every 24 h (`telemetry_retire.go:265-285`; no manual trigger found); runs are retirable only after 7 days (`run_memory_retire.go:70`), which a drill cannot wait for | read the census for natural `kept_named_by_reference` entries; the guard itself is proved by the D1 tests (the window is a variable, `:70`) |

Evidence class: deployed; local proof does not count for checkpoints,
restore, rollback (CLAUDE.md, Proof).

## E. Expected saving, with inputs

Inputs: carried message bytes are 55.9% of message bytes on the tape on the
largest computer (goal file, T0; not re-measured). About 190 messages are
carried per run started (store-once design, "Where the redundancy actually
is"; I did not re-count). A reference row is one row plus segments,
estimated at under 1 KB including row overhead (**invented**; a segment is
under 128 bytes, depth is at most the number of runs inside the carried
tail), against 190 messages each at least a row of overhead plus its text.

- **Tape**: carried message bytes fall by about 55.9% x (1 - r), where
  r (reference bytes over copied bytes) is under 1% on these inputs, so
  about 55 percentage points of message bytes. The share of message bytes in
  total tape bytes was not measured by me, so the whole-tape figure is
  **not stated**; the T0 counters give it after deploy 2.
- **Working store**: the copied rows are not written, so live carriers are
  smaller by the same fraction. Offsetting cost: retirement keeps a whole
  origin run while any live reference names it (the guard is by run,
  B7), so older runs inside a carrier's tail stay until that carrier is
  retired. The newest two runs per document agent are already kept by the
  existing rule (`run_memory_retire.go:299-363`); the added retention is the
  runs older than those that a kept carrier's tail still names. A tail is
  bounded by the compaction ceiling (`run_memory.go:792-800`, 90,000 prompt
  tokens), but how many runs that spans was not measured. The census
  `kept_named_by_reference` count and byte total, read after deploy 2, is the
  measure; if it exceeds the saving, the design fails its own purpose and
  stops.

## F. Open questions, with a conservative default

F1. What should a wake do when its carrier's reference dangles? Default:
    the run fails to start with the named error; the next wake skips that
    carrier as a source (B7). Falling back to an older source for the same
    run changes model input and is not taken.
F2. What do whole-computer checkpoint restore, a copied store's floor file,
    and automatic rollback do when the floor is above the target? Read: the
    updater and base restore refuse (A3); `Open` refuses. Not read: restore
    of a checkpoint older than epoch 7 onto an older identity. Default:
    refusal with the floor's reason, never a wedge; to be confirmed by the
    drill row.
F3. Do the Texture frame and cell-author readers have any caller that relies
    on the swallowed error (a path that must not fail the turn)? Default:
    they fail it (B7).
F4. Should the checkpoint entry also be a reference? Default: no (one per
    carry, small; its `TokensBefore` is part of the rendered bytes,
    `run_memory.go:196-199`, `:1355-1367`). Measure it with the first counter
    reading.
F5. Does a Texture run always have opening messages after the carry
    (`run_memory.go:102`)? Needed only for B7's statement that
    `runMemoryCarryKinds` stays unchanged. Default: assume yes; the D1 test
    asserts it for the hold-trajectory path.
F6. Plans and costs: the guard's two reads and the resolver's range reads
    were not planned or timed. Default: pin plans in tests; time on d@e
    before deploy 2.
F7. A tool to list carriers that name a run about to be redacted. Default:
    none; named residual (failure 34).
F8. The platform side's decoding of tape batches (whether anything but the
    guest decodes `Validate`). Not read. Default: the floor protects the
    guest store, and deploy 2 waits for deploy 1 everywhere a batch is
    decoded.
F9. Names (`carry_ref`, `run_memory_carry_recorded`, `#i` ids,
    `kept_named_by_reference`, counters, `OriginRunID`/`OriginSeq`, the 128
    byte bound, the 1 KB estimate) are invented; the seats may rename them.

## What I could not verify

Nothing was compiled or run. The 55.9% and 190-per-run figures are from the
goal file and the store-once design. I did not read the existing carry
tests, the platform decode path, the whole-computer restore path, or any
query plan.

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
| 5 | DS blocking 5, CL blocking 1 | no durable floor; prose rule conflicts with restore and rollback | `floor.go:67-119`, `open.go:223`, `updater.go:569-600`, `selfdev_base_restore.go:76` | Adopt: epoch 7 and floor before the first reference; prose rule dropped (B2). Correction to the finding's precedent: the retire op's floor is epoch 4, below the current 6 (`event_index_trim.go:48`), so the value is new, not copied. Whole-computer restore path not read (F2) |
| 6 | DS blocking 6 | `texture_frame.go:55-64` swallows the error | read; consumers `:187`, `:231-234`, `:256-260` | Adopt (B7). Also found `cellAuthorReceipt` swallowing (`texture_cell_author.go:104-107`) |
| 7 | CL blocking 4 | write-time fallback is a second writer hiding defects | B3 of draft 1 | Adopt: removed (B4). Copy writer no longer needed even for residue import (B8) |
| 8 | CL blocking 3 (F9 inconsistent) | B4 and F9 disagreed on where the origin check lives | draft 1 | Adopt by deletion: no write-time check anywhere (B6) |
| 9 | DS should-fix 1 | unexported provenance fields cannot compile across packages | `internal/types/task.go:488-505` | Adopt: exported `OriginRunID`, `OriginSeq`, `json:"-"` (B3.5) |
| 10 | DS should-fix 2 | `carry_ref` in `runMemoryCarryKinds` is a no-op that perturbs the slot pass | `run_memory_retire.go:322-330`, `:176`; resolved list sees `message` | Adopt: dropped; effect stated (B7, F5) |
| 11 | DS should-fix 3 | F6 understates the scan cost | `schema.go:356-357` | Adopt: guard reads one agent's refs via the kind index; plans not run (B7, F6) |
| 12 | DS should-fix 4 | D2 step 5 not runnable | `telemetry_retire.go:265-285`; window `run_memory_retire.go:70` | Adopt: scheduled-wake census only; guard proved in D1 (D2) |
| 13 | DS should-fix 5 | census lies after the guard | `run_memory_retire.go:604-616` | Adopt: stated; optional selection filter and `kept_named_by_reference` (B7, C15) |
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
