# Effects desktop+OG event_projection after live residue import — 2026-08-16

**Boundary:** execute. Not live checkpoint. Super was not started. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Observation:** `docs/evidence/effects-live-residue-import-2026-08-16.md`

## What landed

Live import exists, so these tables are event-derived:

- `desktop_workspaces`
- `desktop_sessions` (identity only; presence stays off-tape)
- `desktop_app_instances`
- `desktop_window_placements`
- `og_objects`
- `og_edges`

Replay airworthiness now classifies them `event_projection`. `desktop_state` stays `empty_until_supported`. `presence_volatile` is still refused.

Projector now DELETE+INSERTs `desktop_sessions` for the desktop, same as app instances, so leftover `created_at` / extra tabs cannot keep live ≠ replay. Presence columns stay `''` / NULL.

This does not weaken EmptyUntilSupported for other tables. It does not SQL-empty residue. Checkpoint stays unpaid until this guest is owner-refreshed and a second import (or later projection) replaces live session rows.

## Tests

`go test ./internal/store ./internal/agentcore -run 'TestProjectDesktopStateReplacesLeftoverSessions|TestImportResidueSnapshotCoMovesDesktopAndOG|TestDesktopAndOGAreEventProjectionAfterLiveResidueImport|TestProjectBatchRefusesSessionPresence|TestReplayEligibilityRejectsNonEmptyUnsupportedDirectWrites'`

## Unchanged

Genesis 409. Armed=false. Super not started. Constructed freeze remains 7122f279. Do not present ModeReceipt. Do not send live mail.
