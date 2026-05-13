# Trace Control Artifact Links Proof

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`

## Slice

Trace moment detail now links control-system moments to their durable artifacts:

```text
compaction moment -> run_memory entry
continuation moment -> run_continuation record
promotion moment -> promotion_candidate record and rollback report JSON
```

The Trace inspector renders an `Artifacts` section when a selected moment has one of those durable records.

## Reason

The previous Trace run-geometry panel made control moments visible, but a visible moment without the underlying artifact is still weak evidence. Long-running Choir development needs to inspect the actual compaction checkpoint, continuation objective, candidate identity, verifier report, and rollback record from the same product path.

## Verification

Command run:

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestTraceRunGeometryMomentsHaveReadableSummaries|TestRuntimeRunMemoryThresholdCompaction|TestRunContinuationCompactsAndStartsBoundedNextGoal'
```

Result: passed.

Command run:

```text
pnpm build
```

Result: passed.

## Boundary

The backend artifact resolution is verified. The frontend inspector compiles and renders from the same detail shape, but there is not yet a Playwright test that seeds a trajectory with compaction, continuation, and promotion artifacts and clicks each inspector artifact in the browser.

## Next Deformation

Add a product-path Trace artifact Playwright test, then expose explicit navigation from artifact cards to promotion review, continuation child run, and compaction/run-memory views.
