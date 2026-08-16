# Effects residue import owner command — 2026-08-16

**Boundary:** define + tested product path. Not live execution. Super was not started. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**HEAD parent:** `474b5053` (residue import Store method)

## What landed

Owner-scoped guest command:

`POST /api/computers/{id}/lifecycle/import-residue-snapshot`

CLI: `choir computer import-residue-snapshot --computer …`

This calls `Store.ImportResidueSnapshot`. Autoputer does **not** auto-import. Unbound tape is 503. Desktop-only or OG-only residue is 409. `EmptyUntilSupported` is unchanged.

Do not run this on staging until current main (this commit or later) is deployed **and** the retained computer has been owner-refreshed onto that guest. Global deploy must not rewrite `constructed-computer-version`.

## Tests

`go test ./internal/agentcore ./cmd/choir ./internal/store -run 'TestImportResidueSnapshot|TestComputerImportResidueSnapshot'`

## Unchanged

Staging was `1a17a035` at probe. Epoch 272. `propose_only` generation 1. Checkpoint 409. Genesis 409. `Armed=false`. Super not started.
