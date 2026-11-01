# Derived-key marks: a boot trusts a side table that is known current

Date: 2026-11-01. Status: built (this commit); deployed proof is the
orchestrator's. Mutation class: red (the store's read correctness, the
projector's transaction, the replay path). Problem:
[derived-key-validation-rereads-og-objects-every-boot-2026-11-01.md](../problems/derived-key-validation-rereads-og-objects-every-boot-2026-11-01.md).
Original design and rollback argument:
[store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md](../archive/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md).

## Choice: (A) a persisted mark bound to the projection head, and (C) replay maintains the table

Each family has one **mark**: a sentinel row in `og_key_index` itself
(`canonical_id = ''`, `key_name = <family>`, `key_value = NULL`), whose
`owner_id` names the mark generation and a digest of the family's kind and
expression, and whose `content_hash` is the store's **projection head
fingerprint** (`computer_id|sequence|canonical_event_head` of the one row of
`computer_event_projection_heads`).

- **Stamp.** Every transaction that moves the projection head
  (`finalizeBatch`, live and replay: the only writer of that table) also, in
  the same transaction, writes the mark of every family that is readable in
  memory (validated, not bad) with the new head, and deletes the mark of every
  other family. A validation pass deletes its family's mark when it starts and
  stamps it when it publishes.
- **Trust.** `Open` reads the four marks by primary key (four point lookups;
  nothing scales with og_objects) and the head fingerprint. A mark whose
  generation and family digest match this binary and whose head equals the
  store's head makes its family validated at once: no pass. Any other mark is
  deleted.
- **Invalidate.** A maintenance failure or a malformed row deletes the mark in
  the writer's own transaction. Every withdrawal is also undone durably by the
  next head-moving transaction (it deletes the mark of an unreadable family)
  and by the next pass (it deletes the mark first). A read tripwire also
  deletes the mark at once, through the write handle, after its read
  transaction ends (hardening 1, below).
- **Self-check at open.** Before trusting a mark, Open probes the family for
  a malformed key row through a partial index of malformed rows only
  (`idx_og_key_index_bad ... WHERE derive_ok = 0`); a hit distrusts and
  deletes the mark (hardening 2).
- **Bounded trust.** About weekly, a full pass over every family runs even
  when all are trusted (hardening 3).
- **Replay maintains the table.** `projectBatchForReplay` now keeps
  `og_key_index` in step in its transaction, exactly as the live projector
  does, and stamps the marks. A replayed tail (after a compact rebase, a
  snapshot restore, or a crash between a tape pin and its local finalize) no
  longer leaves the table stale, so it no longer costs a pass. This is the
  531-row case of 05:30: a rebase plus a 1,000-event tail now re-derives
  those rows in the replay itself.

Why the head. Every og_objects writer that does not maintain the table moves
the head in the same transaction, in every binary that can open this store:

- **Pre-table binaries cannot open state.db.** The SQLite store was born in
  871ab6531 (2026-10-30) with the side table already maintained
  (`git show 871ab6531:internal/objectgraph/sql_store.go`: `ensureDerivedKeysTable`
  at line 270, `Maintain` at 341 and 816). Every binary before it is a
  Dolt-store binary; it has no SQLite open path, and Dolt was deleted from the
  release in the same commit. So the only older generation that can run on
  this store is "G1": maintains on every live write, skips on replay, and
  knows no mark.
- **G1's non-maintaining writer is its replay, which moves the head** without
  stamping. Its live projector moves the head too. Either leaves the mark on an
  old head: distrusted at the next forward boot.
- **G1's head-neutral writers maintain**: `PutObject`, `putBatchTx`,
  `DeleteObject`, the vocabulary migration and revert
  (`TestOnlyMaintainedWritersTouchOgObjects` guards the list).
- Bonus: a G1 that runs its validation pass deletes the mark itself. The mark
  is a key row outside the family's id range, so G1's stray phase drops it as
  an orphan (`derivedStrayIDs`; this binary skips `canonical_id = ''` there).

Why not a separate table or a sidecar file. The mark lives in the table it
vouches for, so every copy, swap, snapshot or restore that carries the table
carries the mark, and one that drops or empties the table drops the mark. The
compact copy (`VACUUM INTO`, one read transaction) and the snapshot
(`sqlite_dbpage`, one read transaction, head read first) are each one
committed state, so their marks match their own heads and tables. The
replay that follows either one maintains the table and restamps.

Why not (B) triggers. A trigger on og_objects lives in the database, not the
binary, so "rollback is one revert" fails: the reverted binary keeps running
the trigger, and a trigger defect is not undone by a deploy. A trigger cannot
set the in-memory `bad` flag, so a malformed row would need a read-time
probe. The json_extract cost is the same as `Maintain` today (one
`INSERT ... SELECT` from the row just written). And the schema digest of the
witness would change. Rejected.

## The validator without body overflow (three of the four families)

og_objects columns, in order: `canonical_id, object_kind, owner_id,
computer_id, version_id, content_hash, body, metadata, created_at,
updated_at, ...`. The validator compared `owner_id, content_hash` (before
body) and `updated_at` (after body and metadata). Reading `updated_at` walks
the body's overflow chain. Measured locally: with a row's overflow pages
zeroed, `SELECT <a column before body>` succeeds and `SELECT <a column after
body>` fails "database disk image is malformed".

`updated_at` matters only for the commitment-record addressee read, which is
newest-first through `(key_name, key_value, owner_id, updated_at,
canonical_id)`: a stale `updated_at` there changes which rows a limit
returns, silently. The three metadata families page by `canonical_id`; their
side `updated_at` is never an answer. So for those three, the validator and
the read tripwire compare `(present, kind, owner_id, content_hash)` only,
which reads nothing after `content_hash`. `content_hash` covers kind, body
and metadata (`ContentHash`), so a changed key is still caught. The addressee
family keeps the `updated_at` comparison and still reads the chain. A repair
(`rederive`) reads the row in full, as it must: it evaluates the expression.

Note on the brief: the metadata families "derive from metadata", but
`metadata` itself sits after `body`, so deriving reads the chain too. What
the three families avoid is the comparison's read, which is the whole pass
when nothing is stale.

## Ways it can fail (written before the code), and the test for each

Tests are in `internal/store/derived_key_marks_test.go` (the real `Open`,
`FinalizeBatch`, `FinalizeReplayBatch`, `PrepareCompactCopy`,
`RebaseOntoCompactCopy`) and `internal/objectgraph/derived_keys_test.go`.

1. **No mark after a clean run**: the boot still validates. A store that
   validated and then finalized is reopened; every family is validated with no
   pass run (`TestReopenTrustsMarksStampedByTheProjector`).
2. **A stale mark is trusted after a non-maintaining writer**: a G1 replay,
   simulated by raw og_objects and head writes with no mark stamp, then a
   reopen: no family trusted, and the pass repairs
   (`TestHeadMovedWithoutStampDistrustsTheMarks`).
3. **Missed invalidation by a head-neutral writer's failure**: a malformed
   body through a direct maintained writer deletes the mark in its
   transaction; the reopen does not trust that family
   (`TestMalformedDirectWriteDeletesTheMark`).
4. **Missed invalidation by an in-memory withdrawal** (a read tripwire): the
   next head-moving finalize deletes the mark
   (`TestWithdrawnFamilyLosesItsMarkAtTheNextFinalize`).
5. **Replay leaves the table stale** (the 531 rows): a replayed batch
   maintains the table and keeps the marks; the table equals a fresh
   derivation and the reopen trusts it (`TestReplayMaintainsTheTableAndTheMarks`).
   A replay onto a store whose families are not validated stamps nothing and
   leaves the table complete for the pass
   (`TestReplayOntoAnUnvalidatedStoreFillsTheTable`).
6. **Compact rebase**: a copy staged while the store runs, more finalizes
   after it, a planned boot that swaps the copy in, the tail replayed: the
   table equals a fresh derivation and no pass is needed
   (`TestCompactRebaseThenReplayKeepsTheMarksTrue`).
7. **Crash between steps**: a finalize whose transaction fails after the
   projection writes (a hook error before commit) commits neither the rows nor
   the mark; the reopen trusts the old mark against the old head and the table
   matches truth (`TestFailedFinalizeCommitsNeitherRowsNorMark`). The pass is
   page transactions that repair only; a crash mid-pass leaves the mark
   deleted (it is deleted when the pass starts)
   (`TestPassDeletesTheMarkWhenItStartsAndStampsWhenItPublishes`).
8. **A mark from another family definition or generation** (a future binary
   changes an expression): rejected and deleted at open
   (`TestMarkOfAnotherDefinitionIsDeleted`).
9. **A store with no head or more than one head row**: never trusted, never
   stamped (`TestNoHeadNeverTrusts`).
10. **The mark leaks into reads or validation**: its `key_value` is NULL, so
    no equality read matches it; this binary's stray phase skips it
    (`TestMarkIsNeverAReadAnswerOrAStray`).
11. **Rollback to G1, then forward**: G1's pass deletes the mark as an orphan
    stray (`TestOlderPassDropsTheMarkAsAnOrphan`, which runs the stray
    reconcile the way G1 does, with no `canonical_id = ''` exclusion); G1's
    replay is case 2.
12. **The validator still reads body overflow for metadata families**: a pass
    over rows whose overflow pages are zeroed completes for the three metadata
    families (`TestMetadataFamiliesValidateWithoutReadingBodyOverflow`).
13. **A stale `updated_at` on a metadata family loops the tripwire**: the
    tripwire no longer compares it for those families; the addressee family
    still does (`TestTripwireComparesUpdatedAtOnlyForTheOrderedFamily`).

## Hardenings after review (Devin, non-blocking, on 6b30bcd67)

Three more ways it could fail, each with its test:

14. **A tripwire withdrawal is in memory only**: a crash before the next
    finalize re-trusts a family known stale, and `OnlyNeeded` then skips it.
    The tripwire now deletes the mark durably on the write handle, after its
    read transaction ends, so a read on the write store does not hold the one
    connection the delete needs. If the delete fails, the family stays
    withdrawn in memory and the failure is logged
    (`TestTripwireDeletesTheMarkDurably`, through the read store with no
    finalize after it; `TestTripwireWhoseMarkDeleteFailsStaysWithdrawn`).
15. **A mark sits over a malformed row** (the open sets `bad` false): the open
    probes `SELECT 1 ... INDEXED BY idx_og_key_index_bad WHERE key_name = ?
    AND derive_ok = 0 AND canonical_id <> '' LIMIT 1`. The partial index holds
    only malformed rows, so the probe does not scale with the family; the
    index is built once on an existing store by reading og_key_index, never
    og_objects (`TestTrustRejectsAMarkOverAMalformedRow`,
    `TestBadRowProbeUsesThePartialIndex`, which EXPLAINs it). The witness
    skips the index with its table (the extractor filters `sqlite_master` by
    `tbl_name`).
16. **A trusted family is never compared again**: a pass over every family
    runs at most `derivedKeyFullPassEvery` (7 days) after the last pass that
    compared all four. The clock is a row in the side table
    (`canonical_id ''`, `key_name 'og_key_index/full_pass'`, its `updated_at`),
    so a restart does not reset it, and it goes with the table through a copy
    or a restore. Any pass that compared and published all four restarts it,
    so the first deploy's catch-up pass counts. The loop waits
    `derivedKeyFullPassDelay` (30 minutes) after it starts, which is after the
    listener is up, and uses the 25% duty cycle
    (`TestFullPassRunsWhenDueAndItsClockSurvivesARestart`).

## The background loop

The loop now passes `OnlyNeeded`: a pass runs only the families that are not
validated (or have a repair pending). Before, one withdrawn family re-ran all
four. Each family is stamped when its own pass publishes, so a deploy that
interrupts a pass keeps the families already done. When all are validated,
the loop runs the periodic full pass once it is due.

## Deployed proof (orchestrator)

1. First boot on the new binary: no marks exist yet, so the log shows
   `derived keys: 0 families trusted from their marks` and one more pass. The
   three metadata families should now report rows/s far above tonight's 40 to
   92 (no overflow reads); the addressee family still takes about an hour on
   the paper.
2. Each `validated N rows` line is followed by its mark (no log line; the
   next boot shows it).
3. The next boot (any deploy, a crash, a planned restart with no rebase):
   `derived keys: 4 families trusted from their marks at the store's head`
   and no `validation pass starts` line.
4. A boot after a compact rebase: the copy's marks are trusted, the tail
   replay maintains the table, and no pass starts.

## Rollback

`git revert` of the commit. The marks stay in `og_key_index` as four rows the
reverted binary never reads (NULL key), and its first pass drops them as
orphan strays. Its replay moves the head without stamping, so a later forward
deploy distrusts any mark left over. The full-pass row stays too (no family's
key_name, so nothing reads it), and so does `idx_og_key_index_bad`, a partial
index the reverted binary never names; both are on a table the witness skips.
No og_objects schema change, no epoch change, no trigger.

## Residuals

- A head-neutral writer whose `Maintain` fails and whose in-transaction mark
  delete also fails, followed by a crash before the next finalize or pass,
  would leave a trusted mark over a stale row. Both statements failing while
  the transaction commits needs an engine fault.
- **The tripwire sees only rows the table returns.** It catches a key row
  whose object changed (kind, owner, hash, and `updated_at` for the
  addressee family) or is gone. It cannot catch an absent one: an object with
  no key row, or whose key row still names its old value, is missing from the
  answer for its current value, and nothing in the read says so (measured in
  `TestTripwireWhoseMarkDeleteFailsStaysWithdrawn`: the read for the new value
  returns nothing until a read for the old one trips). Under a trusted mark
  that can only follow a defect in the stamping invariant (the cases above).
  The residual: such a defect answers incompletely, silently, until the next
  full pass, at most `derivedKeyFullPassEvery` (7 days) away, or until a read
  of the old value trips the family.
- A hand edit of og_objects with no head move (an operator with sqlite3) is
  not detected before the next full pass. No code path does it.
- The addressee family still reads the body chain when a pass runs. A pass
  now runs only after a rollback, a definition change, a malformed row, or a
  withdrawal.
- A store without event authority (no head) validates at every boot, as
  before.
