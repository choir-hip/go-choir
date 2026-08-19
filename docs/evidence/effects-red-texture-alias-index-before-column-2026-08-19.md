# Effects Texture alias index created before computer_id column — 2026-08-19

**Boundary:** diagnose. Problem documentation first. Not freeze. Not promote.
No live send. No residue import while computers cannot boot.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Mutation class:** red — guest store bootstrap, vmctl route resolution,
computer-surface serving. This receipt is documentation only.

**Protected surfaces:** Texture schema bootstrap, guest runtime store open,
vmctl resolve, computer-surface hop.

## Live observation

Staging `/health` reports proxy `0aa1ffb3eb6341cae3116c7c6143774720e6e033`
(`fix(store): ensure computer_id column and primary keys in bootstrap schema
migrations`). Host services are active. Unauthenticated `GET https://choir.news/`
returns the platform shell (`surface.total=platform_shell`). Authenticated
computer accounts do not: proxy lifecycle shows `api.resolve` and
`bootstrap.resolve` errors with max duration 60003ms, and vmctl reports
`active_vms=0` with the retained always-on computer in `failed`.

Guest autoputer dies in Texture bootstrap on existing workspaces:

```text
ERROR EnsureTextureSchema failed: apply texture schema: Error 1072: key column 'computer_id' doesn't exist in table
autoputer: open runtime store: runtime store: bootstrap texture: apply texture schema: Error 1072: key column 'computer_id' doesn't exist in table
```

Repeated on the retained computer `computer-03335285269bdba4f94377e56879f9e6`
(`candidate-fleet-e15cb89f25d963c220319b7b`) at 2026-08-19T06:15Z, 06:17Z,
06:20Z, and later retries. Firecracker processes remain; the guest service
never becomes healthy, so resolve times out and choir.news appears down for
computer accounts.

## Source

`CREATE TABLE IF NOT EXISTS texture_document_aliases` leaves the live table
unchanged. `0aa1ffb3` then ran `CREATE INDEX IF NOT EXISTS
idx_texture_aliases_doc ON texture_document_aliases(owner_id, computer_id,
doc_id)` as part of `textureSchemaDDL`, before `ensureTextureColumn(...,
"computer_id", ...)`. Existing alias tables still have the pre-Family-A
columns, so the index statement fails closed and store open aborts.

`texture_agent_mutations` already creates its computer-scoped index after
`ensureTextureColumn`. Desktop `computer_id` columns are added by
`ensureColumn` before primary-key rewrite; they are not the live crash.

## Belief delta

Family A alias scoping cannot be applied by putting `computer_id` into the
CREATE-TABLE-and-index batch. Existing computers skip `CREATE TABLE IF NOT
EXISTS`. Any index or primary key that names `computer_id` must run after the
column exists.

This is a guest-boot substrate failure, not a choir.news host outage.
Non-computer / unauthenticated traffic still receives the platform shell.

## Forbidden while unfixed

- residue import, checkpoint, restore, retry, promotion, qualified consensus,
  or live send;
- SQL-emptying `texture_document_aliases`;
- replacing the retained computer or workspace;
- treating platform-shell 200 as computer-surface health.

## Next execute slice

Move `idx_texture_aliases_doc` out of `textureSchemaDDL` and create it after
`ensureTextureColumn` + `ensureTextureDocumentAliasesPrimaryKey`. Prove
reopen of a legacy alias table (no `computer_id`) succeeds. Deploy that
repair, then confirm guest autoputer opens the store before any residue
import.

**Rollback:** revert this documentation-only receipt; no product-state or
event-chain rollback exists.

**Heresy delta:** discovered — Family A schema index applied before column
migration on existing Texture workspaces; introduced — none; repaired — none.

**Conjecture delta:** computer-scoped DDL must be ordered as
column-then-constraint on live workspaces; CREATE TABLE IF NOT EXISTS is not
a migration.
