# Post-Commit Observer Swap Races Background Appends

Date: 2026-10-13. Found by CI run 38476951550 (commit 86e7ea0f, race
shard "agentcore/textureowner shard 1"). It blocked the C1e B0 deploy.

## Evidence

`TestCrashBootRecordsJournaledSelfDevelopmentApply` failed with
`race detected during execution of test`:

- write: `computerevent.(*ComputerEventAppender).SetPostCommitObserver`
  (`internal/computerevent/appender.go:189`), called from
  `internal/agentcore/crash_settle_test.go:302`, which detaches the
  observer to simulate the guest dying;
- read: the appender's post-commit hook (`appender.go:687-688`) on a
  background append. The self-development materializer keeps appending
  while it retries the parked op.

The test was landed with L1a-3 (18d01cc0). CI on d2324a55 passed, so the
race is timing-dependent (a flake), not caused by the B0 diff, which does
not touch the appender or self-development.

## Belief

`postCommitObserver` is a plain function field: it is read on every
commit and written without synchronization. Production writes it once,
at runtime start (`internal/agentcore/runtime.go:783`), before appends
begin, so the race is test-only today. Any later runtime swap (detaching
continuation during shutdown, for example) would race the same way.
`decideWithoutActing` (`crash_settle_test.go:236`) swaps it the same way.

## Fix direction

Hold the observer in an `atomic.Pointer`, so swapping it is safe at any
time. Behavior is unchanged: the observer is still read once per commit
and called detached. Mutation class orange (runtime concurrency), and a
git revert rolls it back.
