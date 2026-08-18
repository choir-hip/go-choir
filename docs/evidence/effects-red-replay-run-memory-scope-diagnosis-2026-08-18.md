# Effects replay run-memory residue scope diagnosis — 2026-08-18

**Boundary:** define/diagnose. Not a runtime repair, checkpoint, restore, retry,
promotion, qualified consensus, or effect authorization. Effects remain OFF.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

## Problem

The owner-authorized replay-completeness read after scoped Texture repair
`e58a0aab` reached sequence `3320` with matching event heads, but the live
`run_memory_entries` table was absent from replay. The existing projection
manifest classifies `run_memory_entries` as `ReplayEventProjection`, so a
canonical projection path should be able to reproduce it. The same read also
reported `texture_document_aliases`, which remains explicitly
`ReplayEmptyUntilSupported` and is a separate blocker.

The prior receipt recorded the mismatch but did not establish whether the run
memory rows were omitted from residue import, appended after the snapshot, or
failed during replay. Source inspection now narrows the first question to a
specific authority mismatch.

## Source-confirmed authority mismatch

1. Production `Store.CreateRun` reaches `CreateRunOG` after lifecycle handling.
   `CreateRunOG` writes the canonical run to the object graph. When the
   production projection tape is bound, the object-graph mutation interceptor
   records an `object_recorded` projection operation; it does not populate the
   legacy SQL `runs` table.
2. Production `Store.AppendRunMemoryEntry` takes the projection-tape branch
   when bound. It allocates sequence/parent values from the SQL projection,
   then records a `run_memory_entry_recorded` projection operation. This is the
   intended reducer-backed authority for `run_memory_entries`.
3. `snapshotResidueRuntime` currently selects run-memory rows only through
   `run_memory_entries e JOIN runs r ON r.loop_id = e.loop_id`, scoped by
   `r.computer_id`. That join names the legacy SQL run projection as the
   computer-scope authority even though production run identity is now in
   owner/computer-scoped `og_objects`.
4. The existing residue runtime test explicitly inserts a SQL `runs` row before
   inserting its memory row. It proves the old join, not the production
   canonical-object path, and therefore cannot catch the mismatch.

This is a substrate-level residue selection bug, not evidence that the run-memory
reducer itself failed. The safe repair boundary is to scope residue memory by
canonical owner/computer run identities from the object graph while retaining
SQL `runs` identities as a compatibility source for genuinely legacy rows.
Selection must remain owner-scoped and pair `(owner_id, loop_id)` rather than
trusting a globally unique run ID.

## Alias boundary

`texture_document_aliases` is still written and read directly by
`UpsertDocumentAlias`, `GetDocumentAlias`, and related helpers. No projection
operation or reducer owns it. It therefore remains unsupported under the
current pre-launch manifest; this diagnosis does not weaken
`EmptyUntilSupported` or invent a partial alias reducer.

## Focused verification contract

Before the source repair lands, a focused store fixture must cover both cases:

- a canonical owner/computer-scoped `choir.run` object plus a direct legacy
  `run_memory_entries` row with no SQL `runs` row; the residue snapshot must
  select the row after repair;
- a legacy SQL `runs` row and memory row; the existing compatibility behavior
  must remain selected.

The fixture must assert that a foreign owner/computer pair is excluded and that
selection emits the exact `run_memory_entry_recorded` operation body. No live
row is deleted or rewritten by this diagnosis.

## Decision and next action

Accept this as the docs-first source diagnosis for the run-memory mismatch.
Implement one narrow residue-scope repair after this receipt: derive eligible
run scopes from the already-scoped object snapshot plus legacy SQL run rows,
then select memory rows by owner/run pair. Add the focused regression fixture,
run the focused store tests, land through CI and Node B, refresh the same
retained computer, and rerun replay-completeness. Keep aliases unsupported until
a separately authorized reducer design and repair exists.

**Rollback:** documentation-only diagnosis; revert its documentation commit if
needed. No product state or event chain was mutated.
