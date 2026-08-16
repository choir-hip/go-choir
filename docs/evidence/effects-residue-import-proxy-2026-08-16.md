# Effects residue import proxy product path — 2026-08-16

**Boundary:** execute proxy allowlist. Not live import. Super was not started. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**HEAD parent:** `952aaa5d` (guest command)

## Why

`choir computer import-residue-snapshot` posts `/api/computers/{id}/lifecycle/import-residue-snapshot`. Guest handler existed; proxy only forwarded checkpoint/restore/replace/rematerialize on that lifecycle guest path. Unmatched `/api/*` goes through `HandleProtectedAPI`, which binds the API-key computer rather than the path computer. Live import would have missed owner-scoped product path.

## What landed

Proxy treats `import-residue-snapshot` like checkpoint: owner-scoped forward to the active guest with `X-Authenticated-User` / `X-Authenticated-Computer`. Not VM lifecycle control. `computer:lifecycle` scope still required for API keys.

## Tests

`go test ./internal/proxy -run 'TestParseComputerLifecyclePathRejectsWorkspaceReplace|TestImportResidueSnapshotForwardsOwnedComputer'`
`go test ./internal/agentcore ./internal/store ./cmd/choir -run 'TestImportResidueSnapshot|TestComputerImportResidueSnapshot'`

Compiler is the authority: `store.ErrResidueImportUnbound`, `store.ErrResidueImportSplit`, and `Store.ImportResidueSnapshot` exist. gopls MissingFieldOrMethod on `internal/agentcore/residue_import.go` is stale.

## Unchanged

Not executed live. Staging still older than this commit. EmptyUntilSupported unchanged. Genesis 409. Armed=false. Super not started.
