# Effects live desktop+OG writer cutover — 2026-08-16

**Boundary:** execute live-writer seam. Not residue import. Not EmptyUntilSupported reclassify. Super was not started. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**HEAD parent:** `1a17a035` (SQL-only Project)

## What landed

`Store.BindProjectionTape` routes driver desktop saves and OG PutObject/PutEdge/PutBatch/DeleteObject through `projection_batch_recorded` append+project. Presence stays in-memory. Projector SQL bypasses the interceptor, so Finalize cannot recurse.

Live append remembers pinned payload bytes so Finalize does not need a platform GET on the write path. Reconstruct uses `GET /internal/computers/events/payload` (`event:read`). Autoputer binds the tape and payload resolver after appender construction, before Reconstruct.

Unbound stores still SQL-write (tests without a computer chain). Production autoputer binds. Residue is not imported. `EmptyUntilSupported` is unchanged. Checkpoint remains 409 on staging residue.

## Tests

`go test ./internal/store -run 'TestLiveDesktopAndOGWritersAppendProjectTogether|TestProjectBatch|TestSaveDesktopStateDoesNotWrite'`
`go test ./internal/computerevent`
`go test ./internal/objectgraph ./internal/platform ./internal/agentcore -run 'TestDesktopSessionsRemain|TestChoirEventDurable|TestReplayEligibility'`

## Unchanged

Genesis 409. `Armed=false`. Staging host `5557840c`. Epoch 272. `propose_only` generation 1. Super not started. No live mail. No rematerialize.
