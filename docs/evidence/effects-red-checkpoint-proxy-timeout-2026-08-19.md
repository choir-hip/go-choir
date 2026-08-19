# Effects pre-A checkpoint proxy timeout — 2026-08-19

**Boundary:** diagnose. Problem documentation first. Not freeze. Not promote.
No live send. Super was not started. Do not retry checkpoint, rematerialize,
restore, or residue import until the route-budget repair below is deployed.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Mutation class:** red — owner product-path checkpoint, proxy guest client
budget.

## Probe

After `087cf290` restored replay eligibility (`eligible: true` at sequence
3375; see `docs/evidence/effects-red-replay-alias-index-eligible-2026-08-19.md`),
the owner-authorized pre-A checkpoint:

```text
CHOIR_HOST=https://choir.news CHOIR_TIMEOUT=15m \
  go run ./cmd/choir computer checkpoint \
  --computer computer-03335285269bdba4f94377e56879f9e6
```

returned at 2026-08-19T09:21:25Z after 31.81s with HTTP 502:

```text
{"error":"workspace replace authority unavailable"}
```

No checkpoint receipt. Computer remained `active` epoch 323 on guest
`087cf290`. Replay eligibility was not re-run and is not claimed lost.
Effects remain OFF.

## Route budget

`HandleComputerWorkspaceReplace` forwards checkpoint, restore,
rematerialize, residue import, and workspace replace through one guest
POST. Ordinary `autoputerHTTP` is 30s. Residue import already uses a
dedicated 110s client (`DefaultResidueImportTimeout` /
`PROXY_RESIDUE_IMPORT_TIMEOUT`). Replay completeness uses a dedicated
110s client. Checkpoint still uses the 30s client.

The 502 body is the shared forwarder's `client.Do` timeout error, not a
workspace-replace refusal and not an eligibility refusal. CLI wall time
matched that 30s budget.

Guest journal for `candidate-fleet-e15cb89f25d963c220319b7b` at
09:21:26 (one second after the CLI POST) shows store reopen phases
(`prepare-path` → `workspace-open` → `runtime-schema` →
`objectgraph-schema` → `texture-schema` complete). Checkpoint therefore
reached the guest and started Dolt-state work that replay completeness
already proved takes ~2 minutes. The proxy hung up first.

This is the same class as the 2026-08-18 residue-import 502, already
repaired for import only.

## Safe repair boundary

Route `/lifecycle/checkpoint` through a 110s-class guest client, matching
residue import and replay completeness. Keep the ordinary 30s budget on
workspace replace. Do not SQL-empty rows, replace the computer, weaken
eligibility, start Super, rematerialize, restore, or send mail.

**Heresy delta:** discovered — checkpoint shares the workspace-replace
proxy path and 30s budget, so an eligible computer cannot bind a pre-A
checkpoint through the owner product path; introduced — none; repaired —
none at this receipt.

**Conjecture delta:** giving checkpoint the existing 110s guest-client
budget is sufficient for the observed 31.81s 502. Eligibility remains
true until a contrary probe. Guest store reopen on checkpoint is evidence
the handler started, not that a checkpoint was published.

**Rollback:** this receipt is documentation only.
