# Effects SQL-only Project — 2026-08-16

**Boundary:** execute projector + presence split. Not live writer cutover. Not residue import. Super was not started. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**HEAD parent:** `9d244d9b` (contracts freeze)

## What landed

`projection_batch_recorded` is a causal event. Payload is a `ProjectionBatch` already resolved **before** `BeginTx`. `Store.FinalizeBatch` applies desktop + OG ops in the same serializable transaction as the head update. Missing batch on that kind is refused. Presence fields (`last_input_at`, `driver_until`, `visibility_state`, `is_driver`) and table `desktop_sessions` are `ErrProjectionPresence`.

`SaveDesktopStateForSession` still SQL-writes workspaces/instances/placements. It no longer writes Dolt `desktop_sessions`. Driver leases live in process memory and die on restart, which is correct for tab-scoped presence.

`EmptyUntilSupported` is unchanged. Checkpoint remains 409 on staging residue. No import event. No Super. `Armed=false`.

## Tests

`go test ./internal/store -run 'TestProjectBatch|TestSaveDesktopStateDoesNotWrite|TestFinalizeWithoutBatch|TestDesktop|TestComputerEvent'`
`go test ./internal/computerevent`
`go test ./internal/agentcore -run 'TestDesktopSessionsRemain|TestChoirEventDurable|TestReplayEligibility'`

## Unchanged

Live `PutObject` still SQL-writes. Platform payload GET unpaid. Genesis 409. Staging host `5557840c`. Epoch 272. `propose_only` generation 1.
