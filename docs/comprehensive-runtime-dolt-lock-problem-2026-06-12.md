# Comprehensive Runtime Dolt Lock Problem - 2026-06-12

## Problem

Parallel comprehensive runtime tests can open many embedded Dolt workspaces at
once. Under that load, Dolt can time out while acquiring the journal manifest
lock and silently continue with a read-only manifest. Later writes then fail
with errors such as:

```text
Error 1105: cannot update manifest: database is read only
```

The failures surfaced while optimizing `go test -tags comprehensive
./internal/runtime` from a serial runtime of roughly 536 seconds to a parallel
runtime of roughly 94 seconds.

## Evidence

- `/tmp/par-run2.txt` from the interrupted Claude session showed 10 top-level
  failures, including read-only manifest write failures in
  `TestSuperSkipLevelCastRequiresCopiedVSuper` and processor/profile tests.
- A current rerun after sourcefetch de-parallelization still failed in 94.501s
  with 2 top-level failures and retained Dolt load symptoms in the log:
  `Error 1105: context canceled`, `sql: database is closed`, and
  `cannot update manifest: database is read only`.
- The interrupted session traced the Dolt behavior to the embedded journal
  lock timeout path: without the driver backoff option, lock timeout can degrade
  to read-only instead of causing an open retry.

## Belief State

The template-copy test store helper is useful and should stay, but it is not
the whole fix. The remaining root problem is the production store opener's
embedded Dolt connector configuration. It should ask the Dolt driver to fail on
journal lock timeout and retry with bounded backoff rather than returning a
read-only connector.

## Remaining Error

After the connector fix, rerun the comprehensive runtime suite and triage any
remaining failures as test-level timing/order issues. The two current failures
before the connector fix are:

- `TestBridgeProviderSubmitsAndCompletes`
- `TestVerifyVTextWorkflowSeededStochasticOrdering`

## Resolution Evidence

The first connector fix removed the direct read-only manifest failures but left
load-only timing flakes in a handful of tests that had been newly parallelized.
Those tests were tightened to wait for durable evidence instead of sleeping, or
kept serial where they intentionally exercise stochastic workflow ordering.

Final local verification after the fix:

```text
nix develop -c go test -tags comprehensive -count=1 -v ./internal/runtime
ok github.com/yusefmosiah/go-choir/internal/runtime 98.978s

nix develop -c go vet ./...
ok

nix develop -c go vet -tags comprehensive ./...
ok

nix develop -c go test ./internal/runtime ./internal/cycle ./internal/store ./cmd/sourcecycled
ok github.com/yusefmosiah/go-choir/internal/runtime 25.966s
ok github.com/yusefmosiah/go-choir/internal/cycle 5.861s
ok github.com/yusefmosiah/go-choir/internal/store 59.750s
ok github.com/yusefmosiah/go-choir/cmd/sourcecycled 1.419s
```

The final comprehensive runtime log contained no `database is read only`
occurrences. It still contains expected cancellation noise from tests that stop
runtime instances while background runs are active.
