# Effects alias-index reshape replay eligibility — 2026-08-19

**Boundary:** deployed acceptance of the Family A leftover-index / desktop-leftover
repair. Not freeze. Not promote. No live send. Super was not started. A later
pre-A checkpoint attempt is a separate route-budget miss, not this receipt.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Mutation class:** red — replay completeness, Texture alias secondary index,
desktop_workspaces leftover unscoped rows.

## Repair identity

Source `087cf2903440ca07e8d3f03d4c231db440418ba7`
(`fix(store): reshape leftover alias doc index and retire unscoped desktops`)
drops leftover `idx_texture_aliases_doc(doc_id)` after the column/PK exist and
recreates `(owner_id, computer_id, doc_id)`, and deletes `computer_id=''`
`desktop_workspaces` rows on scoped write.

CI including Deploy to Staging (Node B):
https://github.com/choir-hip/go-choir/actions/runs/32228390315

Staging `/health` at 2026-08-19T09:10:17Z:

- proxy `deployed_commit=087cf2903440ca07e8d3f03d4c231db440418ba7`
- `vmctl_status=ok`

## Owner-authorized refresh

Retained computer `computer-03335285269bdba4f94377e56879f9e6` was active at
epoch 322 on guest `997f25cb`. Product-path refresh:

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=15m \
  go run ./cmd/choir computer refresh \
  --computer computer-03335285269bdba4f94377e56879f9e6 \
  --idempotency-key effects-refresh-087cf290-2026-08-19T0910Z
```

Lifecycle receipt `01a01949-fe80-7448-aec4-1009e9a6b38e`:
`active@322` → `active@323`.

Guest autoputer `/health` at `http://10.200.4.2:8085/health`:

- `status=ready`, `runtime_health=ready`
- `deployed_commit=087cf2903440ca07e8d3f03d4c231db440418ba7`
- `deployed_at=2026-08-19T09:11:11Z`

Journal on the same boot: `store: open phase=texture-schema status=complete`
at 09:11:12, then `runtime: started`. No Error 1072. Authenticated
`GET https://choir.news/` with `X-Choir-Computer:
computer-03335285269bdba4f94377e56879f9e6` returned HTTP 200 `text/html`
(523 bytes) containing `/assets/` and neither `platform_shell` nor
`underivable`. Unauthenticated `/` remains the platform shell.

## Residue import

Owner-authorized residue import appended after the refresh:

```json
{"computer_id":"computer-03335285269bdba4f94377e56879f9e6","desktops":1,"sessions":0,"objects":413,"edges":111,"run_memory_entries":1083,"start_intents":1,"operations":1,"texture_mutations":1,"appended":true}
```

## Replay completeness

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=15m \
  go run ./cmd/choir computer replay-completeness \
  --computer computer-03335285269bdba4f94377e56879f9e6
```

Captured at `2026-08-19T09:19:57.843999438Z`:

- computer: `computer-03335285269bdba4f94377e56879f9e6`
- schema version 3
- live and replay sequence `3375`
- canonical event head
  `a03fc002d7514ff3a3a750c446e6a669d1d79e5a116873a8c8b4ddfd66f053f4`
  on both sides; desired/effective heads also matched
  (`a3cf16d0d1dbb46e4ebd5841af5007575fb74184d54c2e6fa26f856769b92b44`)
- `run_memory`: 1083/1083, 0 live-only, 0 replay-only, 0 differing
- observations: 82 live == 82 replay, same keys
- result: `equivalent`
- eligibility: `true`, reason
  `canonical event replay is eligible for the declared manifest`
- probe digest
  `4889e604386d1f44f877c3b4edaaf67e50bd9f3c5132c95257cb8a83f6d6e960`

`dolt:texture:schema:texture_document_aliases` matches on both sides.
`doc_id` has no Key. `computer_id` and `owner_id` remain PRI.
`dolt:texture:table:texture_document_aliases` hash
`sha256:2fcda003cf2e31f88093de22e160f578a869ed83e57e04c45edf8d5aa39c7683`
matches. `dolt:texture:table:desktop_workspaces` hash
`sha256:abe5e1086a56fdcc15c53478e2b5662374b9ff0cc7297126098692713e40a7af`
matches. `dolt:texture:content_root`
`sha256:861d5c7ea8b23a8f67dfe1a13a73c79bcd8c56b95060452e4f03815ecfdd1526`
matches.

The only observation value difference is `dolt:texture:head` (live Dolt
`HASHOF('HEAD')` vs the disposable replay workspace HEAD). That is an audit
receipt, not schema or table content, and does not keep eligibility false.

**Heresy delta:** discovered — none beyond the already documented leftover
`(doc_id)` index and unscoped desktop rows; introduced — none; repaired —
live SHOW COLUMNS and desktop_workspaces content now match OpenFresh replay.

**Conjecture delta:** Family A leftover-index name-reuse and unscoped desktop
PK twins are closed on this computer at epoch 323 / sequence 3375.
Eligibility is true. Pre-A checkpoint, Super start, restore, and promotion
are not claimed.

**Rollback:** platform rollback is `git revert 087cf290`. The refresh and
residue-import event are forward lifecycle/tape records and must not be
SQL-reversed.
