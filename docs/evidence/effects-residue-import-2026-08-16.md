# Effects residue import (code, not live) — 2026-08-16

**Boundary:** define + tested import path. Not live execution. Not EmptyUntilSupported reclassify. Super was not started. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**HEAD parent:** `92149d06` (live desktop+OG writer cutover)

## What landed

`Store.ImportResidueSnapshot` snapshots current desktop workspaces/instances/placements plus OG objects/edges and appends **one** `projection_batch_recorded` event. Desktop-only or OG-only residue is `ErrResidueImportSplit`. Unbound stores are `ErrResidueImportUnbound`. Empty residue is a no-op.

Session identity may ride the desktop op (`session_id`, `device_id`, `viewport_profile` only). Presence (`last_input_at`, `driver_until`, `visibility_state`) is omitted from the snapshot and cleared on project. `EmptyUntilSupported` is unchanged.

This is “state as of now,” not fabricated history of heads 1–26. Live staging must not call it until current main (`92149d06` or later) is deployed. Autoputer does not auto-import.

## Tests

`go test ./internal/store -run 'TestImportResidueSnapshot'`

## Unchanged

Staging host `5557840c`. Epoch 272. `propose_only` generation 1. Checkpoint 409. Genesis 409. `Armed=false`. Super not started. No rematerialize.
