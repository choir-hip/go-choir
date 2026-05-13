# Objective Fingerprint Promotion Dedupe Proof

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`

## Slice

Added deterministic objective fingerprints and patchset digests to the worker/candidate path:

```text
request_worker_vm -> objective_fingerprint
delegate_worker_vm -> export_patchset -> patchset_sha256 -> promotion candidate
```

The fingerprint normalizes simple textual variance in objectives. Promotion candidates record both the normalized objective fingerprint and the patchset SHA-256 in candidate JSON.

## Reason

Exact dedupe is not enough for long runs. Agents can repeat the same intent with changed capitalization, whitespace, or punctuation, and workers can produce equivalent patchsets with different worker heads or manifest paths.

This pass adds a conservative equivalence signal without hiding real portfolios:

```text
same source run + same base SHA + same normalized objective + same patchset digest => same promotion candidate
```

## Guarantee

- Worker lease reuse no longer depends on byte-identical purpose text.
- Worker handles expose `objective_fingerprint` in the product-path trace.
- Promotion candidates expose `objective_fingerprint` and `patchset_sha256`.
- Equivalent patchsets from the same source run dedupe even when worker head and manifest path differ.

## Verification

Command run:

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/vmctl -run 'TestSuperRequestWorkerVMReusesActiveLeaseUnlessParallelAllowed|TestQueuePromotionCandidatesDedupesEquivalentPatchsetFingerprint|TestQueuePromotionCandidatesForWorkerExportsDedupesExactExport|TestOwnershipRegistry_RequestWorkerReusesActiveLeaseUnlessParallelAllowed|TestHandler_RequestWorker|TestClient_RequestWorker'
```

Result: passed.

## Boundary

This is deterministic normalization, not semantic embedding. It catches low-level recurrence noise, but it will not know that "build the launcher" and "add a start menu" are the same objective until a richer objective model is attached.

## Next Deformation

Make objective fingerprints first-class run memory: visible in Trace, attached to continuation records, and available to the controller when choosing whether to continue, retry, fork a portfolio, or stop for review.
