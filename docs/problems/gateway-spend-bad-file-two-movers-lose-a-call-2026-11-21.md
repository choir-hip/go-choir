# Two gateway processes that both find a corrupt spend file can lose one process's calls (cause: the test, not the ledger)

Recorded 2026-11-21, from CI on main commit 49d0d6644 (a change that does not
touch `internal/gateway`); failed again on 34bc283ee.

## Problem

`TestPaidSpendTwoProcessesSeeingABadFileMoveItOnce`
(`internal/gateway/spend_two_writers_test.go:455`) failed once in CI under the
race detector: after both processes flushed, the fresh `spend.json` held one
call (`vm-b`), not two. The bad file was moved aside once, as intended. The
call recorded by the other process (`vm-a`) was not in the fresh file.

## Evidence

- CI run for 49d0d6644, job "Go Test (race, non-runtime shard 7)":
  `spend_two_writers_test.go:479: both processes' calls must be in the fresh
  file: &{USD:1.2e-05 InputTokens:100 ... Calls:1 ... Computers:map[vm-b:1.2e-05]}`.
- The same package passed on the build VM in the landing run of the same tree
  an hour earlier, so the failure is an interleaving, not a constant.
- The test was added with the two-writer work (f1aa4df9d, f883ea88d).

## Cause (found 2026-11-22)

The fault is in the test, not the ledger. `saveOnce`
(`internal/gateway/paid_budget.go:410`) holds the file lock from the read
(`:423`) through the quarantine (`:427`) and the write (`:460`), and the other
process, waiting on that lock, reads the fresh file and merges its delta; the
same shape holds in `AnthropicRelay.saveSpendOnce`
(`anthropic_relay.go:370`). Two writers meeting a bad file, or a good one,
cannot lose each other's call there.

What lost the call: `Record` kicks the ledger's flusher on its first call
(`spend_file_lock.go` `kickSave`), and the test's `stopSaver` only registered
the stop for cleanup, so the flusher kept running. Interleaving:

1. `a.Record(vm-a)` starts a's flusher; it saves vm-a to `spend.json` and
   marks the delta saved (`paid_budget.go:458`).
2. The test overwrites `spend.json` with the bad bytes
   (`spend_two_writers_test.go:462`). The saved vm-a call now exists only in
   a's memory.
3. `a.Flush()` finds an empty delta (`paid_budget.go:414`) and writes nothing.
4. `b.Flush()` finds the bad file, moves it aside and writes b's memory:
   vm-b only. One call, one bad file moved: the CI signature.

Reproduced deterministically on the build VM with the flusher's save forced
to land before step 2 (`spendSaveWindowHook`), same failure text
(`Calls:1 ... Computers:map[vm-b:...]`). Whether the flusher wins the race
before step 2 depends on scheduling, hence rare, and likelier under `-race`.

## Fix

`stopSaver` now stops the flusher at once, before any `Record`, so these
tests save by `Flush` only. `TestStopSaverLeavesNoFlusherToRaceATestsOwnFileWrites`
fails on the old helper (a flusher exists after `Record`) and passes on the
new one. The ledger is unchanged.

## What remains

A spend file corrupted from outside after a call was saved to it loses that
saved call from the file when the other process quarantines it and writes its
own memory; the owning process's memory still holds the call, but it is not
rewritten. This is the documented loss in `noteBadMoved`, a property of
external corruption rather than of two writers, and it is not fixed here.
