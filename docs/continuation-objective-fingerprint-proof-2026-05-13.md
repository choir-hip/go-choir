# Continuation Objective Fingerprint Proof

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`

## Slice

Continuation selection now carries objective identity:

```text
completed/blocked run -> compact run memory -> objective_fingerprint -> selected continuation -> child run metadata
```

Repeated continuation selection for the same source run and normalized objective returns the existing selected or started continuation instead of creating another next-goal record.

## Reason

Choir should not stop just because one agent finishes, but automatic continuation must not create duplicate runaway next goals. Context-limit recovery needs a durable identity for "what objective is this continuation pursuing?"

## Guarantee

- Continuation selection records `objective_fingerprint` in details.
- Started child runs inherit `objective_fingerprint` in metadata.
- Continuation events include `objective_fingerprint`.
- Selected/started continuations dedupe by source run and normalized objective fingerprint.
- Compaction remains part of first selection, so the continuation has operational memory before it can start.

## Verification

Command run:

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestRunContinuationCompactsAndStartsBoundedNextGoal|TestRunCompletionCanAutoStartConfiguredContinuation|TestQueuePromotionCandidatesDedupesEquivalentPatchsetFingerprint|TestSuperRequestWorkerVMReusesActiveLeaseUnlessParallelAllowed'
```

Result: passed.

## Boundary

This defines continuation identity and dedupe. It does not yet proactively trigger compaction at provider context limits; it makes continuation recoverable once selection happens.

## Next Deformation

Make context pressure observable in the runtime loop so near-limit runs compact before undefined behavior, then select or block a continuation with the same objective fingerprint.
