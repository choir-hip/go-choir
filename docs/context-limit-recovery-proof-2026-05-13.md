# Context Limit Recovery Proof

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`

## Slice

Verified the current run-memory path for context pressure:

```text
tool loop -> run memory entries -> threshold compaction -> provider context rebuild
provider context overflow -> forced compaction -> one retry -> completed or blocked
completed/blocked run -> continuation compaction -> next objective
```

## Finding

The earlier assumption that context-limit behavior was fully undefined is no longer accurate. Runtime runs already have:

- threshold-based compaction before provider calls;
- provider overflow detection;
- a forced compaction retry once;
- blocked-run behavior if overflow recovery fails;
- continuation selection that compacts before choosing the next objective.

## Verification

Command run:

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestRuntimeRunMemoryThresholdCompaction|TestRuntimeRunMemoryOverflowRetriesOnceThenCompletes|TestRuntimeRunMemoryOverflowFailureBlocksRun|TestRunContinuationCompactsAndStartsBoundedNextGoal'
```

Result: passed.

## Boundary

This is runtime-level recovery, not yet a full Choir-in-Choir continuation controller. Remaining gaps:

- context pressure is not prominent in the product UI;
- compaction quality is heuristic, not an operational sufficient statistic with verifier status, failed approaches, rollback points, and user taste updates;
- continuation selection still needs a controller that can choose the next bounded objective from queue state, traces, vtexts, and mission docs.

## Next Deformation

Make run memory visible and actionable: Trace should show compactions, continuations, objective fingerprints, and blocked recovery as first-class run geometry rather than background events.
