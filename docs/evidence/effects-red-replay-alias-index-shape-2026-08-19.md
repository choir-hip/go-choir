# Effects replay alias index name-reuse and desktop leftover — 2026-08-19

**Boundary:** diagnose. Problem documentation first. Not freeze. Not promote.
No live send. No further residue import, checkpoint, restore, or retry until
the source repair below is deployed and re-probed.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Mutation class:** red — replay completeness schema hash, Texture alias
secondary index, desktop_workspaces leftover unscoped rows.

## Probe

After the 997f25cb boot restore and an owner-authorized residue import,
replay completeness completed at `2026-08-19T07:12:12.258556027Z`:

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=15m \
  go run ./cmd/choir computer replay-completeness \
  --computer computer-03335285269bdba4f94377e56879f9e6 --timeout 15m
```

Observed:

- computer: `computer-03335285269bdba4f94377e56879f9e6`
- schema version 3
- live and replay sequence `3369`
- canonical event head
  `4932899d518e2e9caeec7b2b6758f1f9ccee9d0a1c396080e6ce070645ecb2c8`
  on both sides; desired/effective heads also matched
- `run_memory`: 1083/1083, 0 live-only, 0 replay-only, 0 differing
- result: `not_equivalent`
- eligibility: `false`, reason `live and replay workspace schemas differ`
- schema_drift: `texture_document_aliases`
- probe digest
  `4cfaef4035b40df67f41ab29b2abd2b2cf333ca7856c14ac66e1c04f048e17e7`

Differences:

1. `dolt:texture:schema:texture_document_aliases` — live `doc_id` has
   `key=MUL`; replay `doc_id` has no Key. All other columns match, including
   `computer_id` PRI default `''`.
2. `dolt:texture:table:desktop_workspaces` — content hashes differ
   (`c1b46562…` live vs `bc8875fd…` replay).
3. `dolt:texture:content_root` — follows from the two above.
   Alias *table content* is not a listed difference.

## Alias index name-reuse

Original DDL (commit `05162395`) created

```sql
CREATE INDEX IF NOT EXISTS idx_texture_aliases_doc ON texture_document_aliases(doc_id)
```

Family A reused the same index *name* for
`(owner_id, computer_id, doc_id)`. `CREATE INDEX IF NOT EXISTS` on an
existing workspace therefore leaves the old single-column index in place.
`SHOW COLUMNS` reports `doc_id` as `MUL` because it is the first column of
that leftover index.

A disposable replay workspace is `OpenFresh`: current `CREATE TABLE` plus
the new composite index. `SHOW COLUMNS` does not mark `doc_id` as `MUL`
because the composite index starts at `owner_id` (already PRI).

Local probe `TestProbeAliasShowColumnsMigratedVsFresh` confirms that a
fixture which *drops* `idx_texture_aliases_doc` before reopening gets the
new composite index and matching `doc_id` Key on both sides. The staging
drift is specifically name-reuse of the old `(doc_id)` index, not column
order after `ALTER TABLE … ADD computer_id`.

## Desktop leftover unscoped rows

`desktop_workspaces` gained `computer_id` in the PK via
`ensureDesktopPrimaryKeys`. Existing rows keep `computer_id=''`.
`projectDesktopState` and `SaveDesktopStateForSession` `INSERT` the
computer-scoped row and do **not** delete the unscoped twin, unlike
`desktop_app_instances` and `desktop_window_placements` which already
`DELETE … AND (computer_id = ? OR computer_id = '')`.

The residue import therefore added a scoped workspace row beside the
migrated empty-`computer_id` row. Replay of the event chain into a fresh
workspace has only the scoped row.

## Safe repair boundary

1. After `ensureTextureColumn` + PK rewrite, inspect
   `idx_texture_aliases_doc`. If missing or not
   `(owner_id, computer_id, doc_id)`, `DROP INDEX` and create the composite
   index. Never create that index from `textureSchemaDDL` before the column
   exists.
2. When writing a computer-scoped desktop workspace, delete
   `computer_id=''` leftover rows for that owner/desktop, matching the
   existing app-instance/placement replace.

Then deploy, owner-refresh the retained computer, and re-run residue import
plus replay completeness. Do not SQL-empty aliases, replace the computer, bind
a checkpoint, rematerialize, restore, retry self-development, or send mail.

**Heresy delta:** discovered — Family A reused `idx_texture_aliases_doc` so
`IF NOT EXISTS` preserved `(doc_id)` on live workspaces; desktop workspace
projection does not retire unscoped PK rows. introduced — none. repaired —
none at this receipt.

**Conjecture delta:** dropping and recreating the alias doc index, plus
deleting unscoped desktop workspace rows on write, is sufficient for the
observed seq-3369 diffs. Alias row projection is not implicated by this
probe (no alias table-content difference).

**Rollback:** this receipt is documentation only.
