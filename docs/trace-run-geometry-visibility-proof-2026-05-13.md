# Trace Run Geometry Visibility Proof

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`

## Slice

Trace now projects control-system moments as readable run geometry:

```text
run-memory compaction -> retry -> continuation -> promotion candidate
```

The backend summarizes compaction, retry, continuation, and promotion events with specific labels and tones. The Trace UI counts control moments in a `Run geometry` panel when those moments exist in the selected trajectory.

## Reason

Context recovery, continuation, and promotion cannot remain invisible runtime mechanics. Users and future Choir controllers need to see where memory was compacted, when a retry happened, which continuation was selected, and where promotion entered the queue.

## Verification

Command run:

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestTraceRunGeometryMomentsHaveReadableSummaries|TestHandleTraceTrajectorySnapshotIncludesGraphAndMoments|TestRuntimeRunMemoryThresholdCompaction|TestRunContinuationCompactsAndStartsBoundedNextGoal'
```

Result: passed.

Command run:

```text
pnpm build
```

Result: passed.

Command run with `CHOIR_SERVICES_FOREGROUND=1 ./start-services.sh`:

```text
npx playwright test trace-settings-registry.spec.js --workers=1 --timeout=120000
```

Result: 3 passed.

## Boundary

Trace now exposes control moments, but it does not yet let a user click through from a continuation to its run memory checkpoint or from a promotion moment to the candidate review record.

## Next Deformation

Connect Trace control moments to their durable artifacts: compaction entries, continuation records, promotion candidates, and rollback reports.
