# Worker Lease Portfolio Control Proof

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`

## Slice

Added default worker-lease reuse for `request_worker_vm`:

```text
super request_worker_vm -> vmctl worker lease -> worker handle
```

By default, vmctl reuses an active worker lease with the same owner, desktop, parent agent, trajectory, purpose, and machine class. A caller must set `allow_parallel: true` to intentionally create another worker for the same objective.

The lease now uses a deterministic `objective_fingerprint`, so simple capitalization, whitespace, and punctuation variance do not create a new worker lease.

## Reason

The live Playwright dogfood showed that one product prompt could expand into multiple worker handles and delegate calls. Candidate portfolios are valuable, but accidental portfolios increase review burden and duplicate promotion candidates.

The controller now has a local invariant:

```text
One active worker lease per objective unless parallelism is explicit.
```

## Guarantee

- Exact repeated worker requests return the same active lease.
- Normalized repeated worker requests return the same active lease.
- Worker handles expose `objective_fingerprint`.
- Explicit portfolio requests can still create a distinct worker by setting `allow_parallel: true`.
- The runtime tool exposes `allow_parallel`, so portfolio intent is part of the product-path trace instead of hidden behavior.

## Verification

Command run:

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/vmctl -run 'TestSuperRequestWorkerVMReusesActiveLeaseUnlessParallelAllowed|TestSuperRequestWorkerVMReturnsTypedHandle|TestOwnershipRegistry_RequestWorkerReusesActiveLeaseUnlessParallelAllowed|TestHandler_RequestWorker|TestClient_RequestWorker'
```

Result: passed.

## Boundary

This is deterministic normalization, not semantic equivalence. Different wording with the same intent can still create another lease until richer objective fingerprints are durable and visible in trace.

## Next Deformation

Use the fingerprint in continuation/run-memory selection so recurrence control can decide whether to retry, fork a portfolio, or stop for review.
