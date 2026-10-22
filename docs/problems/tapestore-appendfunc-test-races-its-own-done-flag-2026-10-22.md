# A tapestore test races its own done flag and failed deploy I's CI (2026-10-22)

Found by FP0 at 08:12 UTC. Deploy I (4c501bae6, freeze round 5, agentcore
only) failed CI run 39744123040 in "Go Test (race, non-runtime shard 7)" on
one test outside its change:

    --- FAIL: TestAppendFuncBuildsOnlyBetweenOtherCalls (0.15s)
        appender_review_test.go:269: build ran while another call was still inside its batch

Mutation class of this record: green. The test fix is yellow. Nothing
deployed (staging kept 43e318712).

## Cause (read in code)

`internal/tapestore/appender_review_test.go:216-275`. The first `Append`
blocks in the `syncFile` hook, inside the writer's lock, until `release`.
`AppendFunc`'s build is meant to run only after that batch ends, and the
test checks this by reading `aDone`. But `aDone` is set by the first
goroutine after `Append` returns, and `Append` releases the writer's lock
before it returns. So a waiting `AppendFunc` can take the lock and run its
build in the window between the first call's unlock and the goroutine's
`aDone = true`. Under -race on a loaded runner that window is wide enough
to hit. The product property (the build runs inside the writer's lock,
never mid-batch) is not what failed; the test's witness is set outside the
lock it is meant to witness.

## Fix

Set the witness inside the batch: the holding `syncFile` call marks the
batch finished after its `f.Sync()` returns, still inside the writer's
lock, and the build reads that mark. The build cannot run before the mark,
because the build runs under the same lock and the mark is set before the
lock is released. Rollback: git revert of the test commit.
