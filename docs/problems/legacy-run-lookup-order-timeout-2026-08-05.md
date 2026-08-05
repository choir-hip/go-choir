# Legacy Run Lookup Order Can Exhaust the Store Deadline

**Date:** 2026-08-05  
**Status:** closed; repaired and deployed in `460c1423`
**Classification:** runtime lookup performance and operability  
**Mutation class of a repair:** orange

## Observation

The deletion-first Texture cleanup restores the accepted pre-mission lifecycle
path. Its local runtime shard repeatedly failed
`TestCancelRunTrajectoryDrainsMoreThanOneActivePage` before cancellation began:

```text
cancel trajectory: lookup run: objectgraph dolt: scan object: context deadline exceeded
```

The test creates 1,001 legacy runs, then calls `CancelRunTrajectory` for one of
them. `Runtime.getRunForComputer` first probes the computer-scoped lifecycle
canonical ID and only then reads the legacy owner-scoped run. For this input the
first probe is known to miss. Under the scale fixture that unnecessary read can
consume the object-store deadline and prevent the valid legacy lookup.

## Boundary

This is not evidence for restoring the rejected supervision transaction,
projection importer, or CI write-mode controller. The smallest candidate repair
is lookup ordering: read the exact legacy owner-scoped identity first, preserve
its existing computer-scope validation, and fall back to the lifecycle-scoped
identity only when the legacy record is absent.

## Evidence

- failing command: `go test ./internal/agentcore -run TestCancelRunTrajectoryDrainsMoreThanOneActivePage -count=1`
- repeated failure duration: approximately 68 seconds
- failure site: `internal/agentcore/trajectory_test.go:336`
- lookup site: `internal/agentcore/runtime.go`, `getRunForComputer`

## Rollback

Revert only the eventual lookup-order change. No schema, persisted data, event
kind, route, or deployment control is part of the repair.

## Repair Receipt

`Runtime.getRunForComputer` now reads the exact owner-scoped legacy identity
first, preserves its computer-scope check, and falls back to the lifecycle
identity only on `ErrNotFound`. The canonical-ID lookup uses the primary
object-graph store, so a just-written run does not depend on a read-only Dolt
connection refreshing a large recent write set. The 1,001-run cancellation
regression passes three consecutive isolated executions. CI run
[`31030833230`](https://github.com/choir-hip/go-choir/actions/runs/31030833230)
passed every runtime shard and deployed exact commit
`460c142394e12b6e307949d0180da08d1b058745` to staging. The scale case was
not recreated against durable staging data because doing so would create 1,001
legacy run records; acceptance is the isolated regression, full CI, and exact
deployed identity rather than a polluting production-data probe.
