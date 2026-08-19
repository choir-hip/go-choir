# Effects alias-index-after-column boot restore — 2026-08-19

**Boundary:** deployed acceptance of the guest-boot repair. Not freeze. Not
promote. No live send. Residue import is recorded only as a follow-on probe,
not as eligibility.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Mutation class:** red — guest Texture bootstrap, vmctl route resolution,
computer-surface serving.

## Repair identity

Source `997f25cb172fb4314545fddcccca6ae9d706e7d7`
(`fix(store): create alias computer_id index after column migration`) moved
`idx_texture_aliases_doc` out of `textureSchemaDDL` and creates it in
`bootstrapTexture` after `ensureTextureColumn(computer_id)` and
`ensureTextureDocumentAliasesPrimaryKey`.

Staging `/health` at 2026-08-19T06:56:36Z:

- proxy `deployed_commit=997f25cb172fb4314545fddcccca6ae9d706e7d7`
- `vmctl_status=ok`

CI including Deploy to Staging (Node B):
https://github.com/choir-hip/go-choir/actions/runs/32223315700

## Owner-authorized refresh

Retained computer `computer-03335285269bdba4f94377e56879f9e6` was failed at
realization epoch 316 on the pre-repair guest (Error 1072 during the 06:54Z
deploy window). Product-path refresh:

```text
CHOIR_TIMEOUT=15m go run ./cmd/choir computer refresh \
  --computer computer-03335285269bdba4f94377e56879f9e6 \
  --idempotency-key effects-refresh-997f25cb-2026-08-19T0702Z
```

Lifecycle receipt `01a018d5-44b4-728e-bc68-9937e2b92351`:
`failed@316` → `active@322`.

Guest autoputer `/health` at `http://10.200.1.2:8085/health`:

- `status=ready`, `runtime_health=ready`
- `deployed_commit=997f25cb172fb4314545fddcccca6ae9d706e7d7`
- `deployed_at=2026-08-19T07:03:41Z`

Journal on the same boot: `store: open phase=texture-schema status=complete`,
then `runtime: started`. No Error 1072 after 07:03Z. vmctl
`active_vms=2` with this computer `premium_always_on` and the platform
computer still serving the unauthenticated shell.

## Computer surface

Authenticated `GET https://choir.news/` with
`X-Choir-Computer: computer-03335285269bdba4f94377e56879f9e6` and
`X-Choir-Desktop: primary` returned HTTP 200 `text/html` (523 bytes,
SHA-256 `9d28353b50b1691a4d3778bff273d80c9c6ee2e3b89950ca05b3e9bab50e24f2`)
containing `/assets/` and neither `platform_shell` nor `underivable`.
Proxy lifecycle recorded `surface.resolve` ok in 2ms and
`surface.upstream` `http_200`. Unauthenticated `/` remains the platform
shell.

## Follow-on probes (not acceptance of this receipt)

Owner-authorized residue import appended at 2026-08-19T07:10Z:

```json
{"computer_id":"computer-03335285269bdba4f94377e56879f9e6","desktops":1,"sessions":0,"objects":413,"edges":111,"run_memory_entries":1083,"start_intents":1,"operations":1,"texture_mutations":1,"appended":true}
```

The guest `ResidueImportReport` JSON does not include `texture_aliases`;
absence of that field is not proof that zero aliases were snapshotted.

Replay completeness at `2026-08-19T07:12:12.258556027Z`, sequence `3369`,
matching heads, `run_memory` 1083/1083 with zero row diffs, returned
`not_equivalent` / `eligible=false`. See
`docs/evidence/effects-red-replay-alias-index-shape-2026-08-19.md`.

**Heresy delta:** discovered — none beyond the already documented 1072 boot
crash; introduced — none; repaired — guest Texture bootstrap on the retained
workspace, computer-surface resolve, and choir.news computer-account load.

**Conjecture delta:** Error 1072 is closed on this computer at epoch 322.
Whole-computer replay eligibility remains unpaid.

**Rollback:** platform rollback is `git revert 997f25cb`. The refresh and
residue-import event are forward lifecycle/tape records and must not be
SQL-reversed.
