# Yaegi session: output overflow can be reported as a timeout, 2026-10-12

## Evidence

- CI run 38323971229 (main 3c242ec0, after the D0 R4 removals), job "Go
  Test (race, non-runtime shard 7)": `TestEvalOutputOverflowFailsClosed`
  failed with "expected output-exceeded error, got: yaegi: evaluation
  timed out: context canceled (stdout len=2097152)".
- D0 commit 154a9bb5 deleted the prototype `Evaluator` and moved the
  containment and refusal tests onto the live `Session`, the interpreter
  desks actually run. The test now exercises live code. It passed locally
  without the race detector.

## Cause (read from code)

`internal/yaegikernel/session.go`, the overflow watcher in the cell
evaluation path:

1. On a stdout or stderr overflow, the watcher calls `outCancel()` first,
   and only then sends on `overflowed`.
2. The evaluation path selects on `outCtx.Done()`, and then does a
   non-blocking check of `overflowed`.
3. If the cancellation is observed before the watcher's send, the cell is
   classified as `DiagTimeout` ("evaluation timed out: context canceled")
   instead of `DiagOverflow`.

## Impact

- The output stays capped either way (stdout is 2,097,152 bytes, the
  cap), and the session is poisoned either way. Safety holds.
- The diagnostic is wrong: a desk, or the owner reading a trace, is told a
  cell timed out when it printed too much. The remedies differ (print less
  versus do less).
- The race was latent in live code. The prototype evaluator's copy of
  this test hid it.

## Fix

Record the overflow before cancelling, so any observer of the
cancellation also sees the overflow. Mutation class orange (desk act
path diagnostics). Rollback: git revert.
