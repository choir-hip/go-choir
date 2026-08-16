# Effects unified-tape contracts freeze — 2026-08-16

**Boundary:** define + additive contracts. Not live Project. Super was not started. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**HEAD parent:** `01d13473` (`complete_from_head`)

## What landed

Payload resolve is required **before** SQL. `ResolvePayloads` fetches, hash-verifies, and decrypts. `GuardNoSQL` refuses Exec during resolve. `HTTPClient.FetchPayload` is the production `ArtifactReader` seam (`GET /internal/computers/events/payload`); platform route is not shipped in this slice.

`ProjectionBatch` is the atomic Project unit (`ProjectorVersionV1` distinct from `ReducerVersion`). Empty batches are refused.

Post-CAS Project failure is `ErrProjectionPoison` wrapped with `ErrNeedsProjectionRepair`. `RecoverPrepared` does not treat a deterministic constraint crash as a silent retry-to-success. One-shot crash recovery (`TestAppenderRecoversCrashAfterCanonicalCAS`) still converges.

`desktop_sessions` stays `empty_until_supported`. A `presence_volatile` class is refused by manifest `Validate` so leases cannot sneak in as nonempty-ok. `choir.event` consumers are inventoried; the kind is not deleted.

## Tests

`go test ./internal/computerevent ./internal/selfdevprotocol`  
`go test ./internal/agentcore -run 'TestDesktopSessionsRemain|TestChoirEventDurable|TestReplayEligibility'`

## Unchanged

No live writers moved. No desktop/OG import. No Super. Genesis 409. `Armed=false`. Checkpoint still 409. Platform payload GET unpaid.
