# Two gateway processes that both find a corrupt spend file can lose one process's calls

Recorded 2026-11-21, from CI on main commit 49d0d6644 (a change that does not
touch `internal/gateway`).

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

## Belief

Not confirmed from the code. The likely shape: process A flushes its delta
into the fresh file and marks it saved; process B, which saw the bad file at
the same moment, then writes a fresh file from its own view and replaces A's.
If so the paid spend ledger can undercount by one process's unsaved calls
whenever a corrupt file is met by two writers at once (a handoff between two
gateway processes). The effect on the owner's spending cap is an undercount,
which is the unsafe direction, though only in the rare corrupt-file case.

## Remaining error

Whether the fault is in the code or in the test's own ordering (two extra
`mustFlush` calls should have merged both deltas) is not known. Next step: a
read-only reading of `Flush` on the bad-file path for a lost update, then a
test that forces the interleaving. Until then the CI job was rerun.
