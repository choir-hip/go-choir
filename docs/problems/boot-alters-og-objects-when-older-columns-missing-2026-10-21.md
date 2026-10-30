# Boot alters og_objects when an older generated column is missing

Status 2026-10-30: still open as a boot-migration concern, but `EnsureRecordIndexes` on Dolt is gone. The object graph is now in the PostgreSQL graph database and guest SQLite; re-point the finding to those stores' migrations. Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

Date: 2026-10-21. Status: **open**; not introduced by FP1, not fixed by it.
Found by: the GPT-6.1-sol panel review of the FP1 candidate 8cadecc7e
(finding A1), adjudicated by FP0 as pre-existing on main.
Mutation class of a fix: red (object store schema, boot path).

## The problem

Every boot of a guest runtime calls `EnsureRecordIndexes`
(`internal/objectgraph/dolt_store.go`, via `Store.EnsureObjectGraphIndexes`
in `internal/store/store.go`). It runs `ensureMetadataColumns(ctx, true)`,
which lists og_objects' columns from information_schema and, for every
generated column in `metadataIndexColumns` and `recordColumns` that is not
there, runs `ALTER TABLE og_objects ADD COLUMN ... STORED`. A STORED column
rewrites every row of og_objects while it holds the engine mutex. On a
populated store that is the failure of
[stored-column-add-blocks-paper-boot-2026-10-21.md](../archive/problems/stored-column-add-blocks-paper-boot-2026-10-21.md):
21 GB read and a listener that never came up.

The columns this applies to are the ones already on main before FP1:
`og_meta_run_id`, `og_meta_trajectory_id`, `og_meta_channel_id` (legacy, on
every store since long before), and the record columns of 8f6708541 and
85d2295b7 (2026-10-14): `og_rec_inbox`, `og_rec_sender_inbox`,
`og_rec_conversation_id`, `og_rec_inbox_seq`, `og_rec_claim`. FP1 adds no
column to og_objects; its side table is created empty
(`docs/archive/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md`).

Where it runs (`internal/autoputer/run.go`):

- without the replay appender, the step `object-graph-record-indexes` runs
  before `rt.Start`, so before the runtime serves;
- with replay, it runs behind the boot health gate with the gate's
  heartbeat. The heartbeat keeps the health probe answering; it does not
  bound the rewrite, and the runtime does not start until it ends.

The boot-phase logs (`objectgraph-schema/add-generated-columns <names>`)
name the step if it hangs; they do not prevent it.

A second shape of the same path: a column whose single-column ALTER is
rejected (the fallback loop logs `generated column ... unavailable, JSON
path kept`) stays missing, so every later boot tries it again.

## Which computers could hit it

A computer whose og_objects has rows and lacks one of the columns above:

1. a store created before 2026-10-14 that has not since completed a boot of
   a binary carrying the record columns (a computer that stayed off, or was
   deferred by every deploy since);
2. a workspace or checkpoint restored from before the record columns, or a
   fork from such a checkpoint;
3. a store where an earlier ALTER of these columns failed or was killed
   part way (the column then stays missing and the next boot repeats it);
4. a store whose engine rejected one of the expressions (the per-column
   retry above).

Computers that have booted a binary since 85d2295b7 to the end of this step
have the columns; for them the step is an information_schema read and
returns in milliseconds (the paper's 15:45:56 boot logged `record indexes
ensured in 23ms`).

## How to check a computer

Read-only, inside the guest's store (or from its boot log):

```sql
SELECT COLUMN_NAME FROM information_schema.COLUMNS
 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'og_objects'
   AND COLUMN_NAME IN ('og_meta_run_id','og_meta_trajectory_id','og_meta_channel_id',
     'og_rec_inbox','og_rec_sender_inbox','og_rec_conversation_id',
     'og_rec_inbox_seq','og_rec_claim');
```

Fewer than eight rows on a store with rows means the next boot rewrites
og_objects. In the boot log, a line `objectgraph dolt: added N generated
columns` or a boot phase `objectgraph-schema/add-generated-columns` that
starts and does not end is the same signal; `record indexes ensured in`
with a duration of seconds or more is its trace after the fact.

## The fix, not done here

Boot becomes discovery only: it records which columns and indexes exist
(the live set) and keeps the JSON path for the rest. Adding a missing
column moves to an explicit online operation that is not on the boot path,
for example the derived-key side table's pattern (an empty table, writer
maintenance, a paged validator). The test for it is a populated store
**missing** the columns, opened and booted, with og_objects untouched;
FP1's open-with-many-rows test covers only a store whose schema is already
complete.

This is out of FP1's scope by FP0's adjudication (2026-10-21) and is named
as a residual in
[choir-launch-fp1-store-throughput-2026-10-21.md](../archive/definitions/choir-launch-fp1-store-throughput-2026-10-21.md).
