# CI wall clock: one store test and a 6 GB-per-push cache (2026-10-24)

Date: 2026-10-24. Station: CI0 (docs/definitions/ci-test-cooptimization-2026-10-24.md).
Status: findings recorded before any fix. Mutation class of this note: green.
Fixes follow in separate commits, each naming the finding it answers.

Evidence is the CI logs of main runs 40138087758 (baseline, 19.9 min),
40136125633 and 40132893674, read with `gh api .../actions/jobs/<id>/logs`
and `go test -json` lines. Times are seconds unless marked.

## Findings

### 1. Shard 1 is the outlier every run, and one test is half of it

Non-runtime shard job time, the last three code runs on main (shards 0-7):

| Run | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 |
|---|---|---|---|---|---|---|---|---|
| 40138087758 | 418 | **1038** | 397 | 360 | 551 | 335 | 173 | 129 |
| 40136125633 | 438 | **1037** | 405 | - | 485 | 421 | 176 | 155 |
| 40132893674 | 307 | **863** | 308 | 437 | 571 | 476 | 187 | 125 |

In the baseline run, shard 1's `internal/store` binary took 960 s, and
`TestEventTableTrajectoryWindowOverTenThousand` took 489 s of it: it builds
three stores of 10,050 events each (21 batches of 500 through `projectAt`)
under -race and Dolt, serially. The next tests on that shard are
`TestOpenWakeListMeasure` (104 s; a measurement test over 3,000 outbox rows
that also asserts the list), `TestPlatformOutageIsBounded` (42 s),
`TestEventTableNothingStampsInDeploy1` (26 s). The round-robin dealer
(`k % split_shards`) puts the heavy tests where discovery order puts them, so
shard 1 got three of the top five.

Per-package serial test time summed over the six split shards (baseline):
store 2,663 s, platform 438, objectgraph 378, projectionbase 276, actorruntime
189. Store alone is 2,663 s of serial binary; even perfectly dealt over six
shards it is 444 s each. Whole shards 6 and 7 finish in 130-190 s: they are
the idle capacity.

### 2. The deploy floor after the outlier is the Node B build

Prebuild ran 22:21:51 to 22:28:55 in the baseline (418 s of nix build);
the tests ended later only because of shard 1. With every shard under about
6.5 min the deploy waits on the prebuild again (the 2026-10-22 note, finding 2).

### 3. Each push writes about 6.4 GB of Actions cache; the limit is 10 GB

`gh cache list` (2026-10-25 00:02): 13 entries, 10.2 GB active against the
10 GB limit. Per push, one new entry per lane: runtime-race 1.29 GB,
non-runtime-race-split 1.28, race-6 1.04, race-7 0.99, vet-build 0.92,
scale-standard 0.89 (about 6.4 GB). Each lane's previous entry stays until
eviction, so the repository holds about 1.5 pushes of caches. Whatever is
evicted, by last access, makes the next run cold in that lane (the 2026-10-22
finding 4). The entries also grow: a prefix restore brings back stale objects
of earlier source versions, and the save writes them out again.

### 4. Runtime shards are balanced and are the test floor

agentcore/textureowner shards: package time 125-196 s, job 3.6-4.8 min,
setup about 35 s. No test over 40 s. Nothing to deal differently.

### 5. Step times, a non-runtime split shard (baseline shard 3)

checkout 3, set up Go (cache restore) 38, test step 304, post cache save 10.
`TMPDIR=/dev/shm` is set for the test step, so Dolt workspaces are in RAM.

## Measured after the first candidate (pull-request run 40146522999, 2026-10-25)

Candidate: heaviest-first deal by name, six split shards, the 489 s test
filling its stores in parallel. Created to `check` done: 638 s (10.6 min; the
baseline's `check` finished at 18.3 min). Test step per non-runtime shard:
split shards 242-545 s, whole shards 100-110 s; runtime shards 250-385 s job.

### 6. The deal balanced the wrong quantity

The weights balanced the sum of test seconds per shard (584-587 on all six),
but the wall time follows `internal/store` alone, because the packages run
side by side and a package's tests run in series. Store seconds per shard in
that run: 481, 452, 407, 217, 500, 417 (test step 516, 490, 444, 242, 545,
454 s). Shards 6 and 7 finished their whole packages in 100-110 s and then
idled while the split shards ran 5-9 min.

### 7. A serial store binary uses one CPU of the runner's four (hypothesis)

Each shard already runs up to five test binaries at once, yet its test step
stays near the store slice's length. If the store binary is the only long one
and the runner has idle CPUs, a second concurrent process with half the store
tests halves the slice. This is a hypothesis until a pull-request run shows the
test step; the candidate that follows runs two processes per shard and the
planner models each as its own shard.

## Measured after the second candidate (pull-request run 40150248142, 2026-10-25)

Candidate: every shard takes a per-package weighted share of the split tests,
shards 6 and 7 start loaded with their whole packages, two concurrent go test
processes per shard. Created to the last test job: 592 s; `check` started at
649 s (it waited 56 s for a runner after its dependencies finished, GitHub
queueing outside the workflow), done 651 s. Test step per non-runtime shard:
295, 238, 212, 487 (shard 3), 400 (4), 453 (5), 268 (6), 205 (7). Runtime
shards 231-344 s.

### 8. Four store tests compile the whole repository inside the test, and the cache lottery decides who pays

Shards 3, 4 and 5 ran 400-487 s while the others ran 205-295 s. The extra
time is four store tests that were 9-25 s in the baseline and in the first
pull-request run: TestEventTableStampGuardCatchesEveryForm 241 s (was 15-19),
TestEventTableOnlyEventObjectOpStamps 194 s (was 19-21),
TestEventTableRollbackArtifactBuildsInTheStorePackage 152 s (was 9),
TestEventTableNothingStampsInDeploy1 42 s (was 26). They call
`listExportData`, which runs `go list -e -export -deps ./internal/...
./cmd/... ./scripts/...` once per test process (internal/store/
event_table_stamp_guard_test.go:79). That compiles every package of the
repository without -race. It costs about 15 s when the build cache already
holds those objects and 150-240 s when it does not.

Why it was cold: the Actions cache entry for the lane is saved by whichever job
of the lane finishes first (the key is the same for all of them), and
restored by prefix by later runs. A job that did not run those tests never put
the non-race repository objects in its cache, so if it is the saver the next
run starts cold on whichever shard hosts the tests. The balance of the deal
cannot see this: the cost lands on whichever shard draws the tests and
whichever cache happened to be saved. In the baseline and the first
pull-request run the cache happened to hold the objects.

Hypothesis check still open: the cold cost needs one more run with the warm-up
(below) to see all shards near 205-300 s.
