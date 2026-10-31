# Test-suite optimization, 2026-10-31

Branch `tests/optimize-2026-10-31`. Owner's ask: drop tests that no longer earn their keep, speed up the slow ones that do, then rebalance CI.

## Method and limits

- Measured on the choir-agents VM (8 CPUs, 9 GB), `go test -race -count=1 -json`, `-p 2` for the non-runtime set and `-p 1` for agentcore/textureowner. Durations are one run each and include contention, so the ratios are reliable and the absolute seconds are not CI seconds.
- "Before" is origin/main `34f1b157e`, run for the six packages touched (store, objectgraph, snapshot, searchplane, computerversion, researchtools). agentcore has no before run (see below).
- Per-shard "before/after" figures are **computed, not measured per shard**. I ran the shard planner's algorithm (a Python port of `plan_split`) over the measured per-test durations. I did not run the eight shards separately. The whole-package load on the two whole shards is assumed to be 120 s each (half of the 487 s serial sum of the whole packages, with a parallel factor of about 2).
- The 310 `internal/platform` tests skip here because no PostgreSQL is installed (`dbxtest: initdb` not found). Platform's real cost is only visible in CI, so its split weights could not be refreshed.
- Local-only failures, in packages I did not touch: `internal/capsulejob` (6 tests), `internal/visualcapture` (5), and `internal/platform` `TestTapeSegmentsOffWritesTheLegacyFilesExactlyAsToday`. I read them as environment effects (sandbox/no browser/no privileges) and did not investigate or change them. CI is the arbiter.
- Other skips are environmental: 323 need PostgreSQL, about 30 need root, 7 are live-provider integrations behind `RUN_INTEGRATION_TESTS=1`.

## Per-package wall time (race, summed per-test seconds)

| package | before (s) | after (s) |
|---|---|---|
| internal/store | 1773 | 682 |
| internal/objectgraph | 216 | 85 |
| internal/snapshot | 63 | 50 |
| internal/gateway/searchplane | 28 | 24 |
| internal/researchtools | 15 | 5 |
| internal/computerversion | 14 | 3 |
| internal/agentcore | not measured | 998 (1165 tests, 1010 s summed) |
| internal/textureowner | not measured | 152 |

Other non-runtime packages (after, unchanged): capsule 69, actorruntime 56, auth 34, taperepack 33, autoputer 25, maild 23, proxy 20, cmd/taperepack 18.

## Tests deleted

| test | verdict | reason | seconds saved |
|---|---|---|---|
| store `TestOpenWakeListMeasure` | (a) extraneous | It only `t.Log`ged timings of the JSON read versus the side-table read, with no assertion. The agreement of the two reads is asserted by `TestOpenWakeListAgreesWithTheRowsAcrossMarks` and `TestOpenWakeListLeavesTheJSONScan`, which remain. | 39 |

No test of a Dolt-era or retired mechanism turned up among the slow or suspicious ones I examined. I did not do a full-suite audit for retired mechanisms, so this is not a claim that none remain.

## Tests sped up (class (b)), measured before -> after

All keep the same boundary (page size, chunk size, window, lease) and shrink only the fixture or the wait.

| test | before -> after (s) | saved | change |
|---|---|---|---|
| store `TestCatchUpEqualsTrimAtEveryTrimPoint` | 440.9 -> 4.9 | 436 | smaller chunk/window fixture crossing the same trim points |
| store `TestOpenCatchesUpAnOldStoreAndFloorsIt` | 142.4 -> 1.4 | 141 | same |
| store `TestCatchUpIsChunkedAndReleasesTheWriter` | 113.1 -> 1.4 | 112 | same |
| store `TestEventTableTrajectoryWindowOverTenThousand` | 125.9 -> 15.2 | 111 | `eventTrajectoryWindow` const -> var (test seam); the test shrinks the window instead of writing 10k+ events |
| store `TestTrimStaysInsideTheWindow` | 92.9 -> 1.1 | 92 | smaller fixture |
| store `TestTrimKeepsPreparedAndOtherKinds` | 70.2 -> 0.8 | 69 | smaller fixture |
| store `TestReopenWithManyRowsDoesNotTouchOgObjects` | 39.5 -> 5.2 | 34 | fewer rows past the same page boundary |
| store `TestStreamSeqSeedFindsAnEventBehindOtherKinds` | 27.6 -> 3.1 | 24 | fewer rows |
| store `TestListRunsByOwnerEquivalentToTheFullKindScan` | 39.4 -> 19.9 | 19 | fewer runs |
| store `TestEventTableNothingStampsInDeploy1` | 16.4 -> 6.3 | 10 | smaller fixture |
| store `TestExactSeedFindsAHighNumberBehindNewerEventsAndObjects` | 9.6 -> 5.8 | 4 | smaller fixture |
| objectgraph `TestValidatorFirstPassAndRepeatPassOnManyRows` | 79.6 -> 9.9 | 70 | rows past the same page boundaries |
| objectgraph `TestOpenDoesNotTouchOgObjects` | 37.8 -> 3.7 | 34 | same |
| objectgraph `TestValidatorHoldsTheWriteConnectionOnlyPerPage` | 15.5 -> 5.3 | 10 | same |
| objectgraph `TestDerivedKeyReadsMatchJSONReads` | 13.6 -> 7.8 | 6 | same |
| objectgraph `TestValidatorComparesOnTheReadPool` | 9.4 -> 4.0 | 5 | same |
| objectgraph `TestValidatorWithWriterStorm`, `TestRecordColumnsMatchJSONReads` | 10.0 -> 5.8, 8.8 -> 5.4 | 8 | same |
| snapshot `TestPageSizesOtherThan4096RoundTrip` | 32.3 -> 18.4 | 14 | 48 blobs still cross the 4 MiB chunk |
| computerversion `TestRowSorterSpilledDigestMatchesInMemory` | 13.2 -> 2.4 | 11 | smaller spill threshold fixture |
| researchtools `TestCaptureSinkSplitsALargePollUnderCorpusdsBound` | 12.8 -> 2.8 | 10 | smaller poll, same bound |
| searchplane `TestMonthlyPace_RouterNeverPassesTheTargetAcrossRestarts` | 19.0 -> 15.4 | 4 | fewer restarts |

agentcore (commit `11752bab1`), not measured before/after because a before run of the 998 s package was not affordable: `api_health_bounded_test.go`, `boot_passivation_cost_test.go`, `carry_select_test.go`, `lifecycle_authority_test.go`, `restore_snapshot_test.go` (lazy chain tape for `TestPlanRestoreTargetBeforeNewestSnapshot`), `selfdev_assignment_end_closes_test.go`, `tools_desk_cell_deadline_test.go`. These shorten stall budgets and fixtures. The touched tests passed under `-race` in 89 s. They are still the 13.0 s and 11.7 s entries in the top-50 list, so the saving is modest and unquantified.

Totals measured for the six packages: summed per-test time 2109 s -> 849 s (about 1260 s saved), almost all in store and objectgraph.

## Product-code seams (the only non-test edits)

1. `internal/store/store.go`: `eventTrajectoryWindow` `const` -> `var`. Production never assigns it.
2. `internal/agentcore/api.go`: `healthStoreBudget` `const` -> `var`. Production never assigns it.
3. `internal/agentcore/engineering_assignment_fate.go`: `selfdevCloseTimeout` `const` -> `var`. Production never assigns it.

## Red-surface check

- The store catch-up/trim tests are boot-path tests. They now exercise the same chunk and trim-point logic on a smaller window. Nothing was deleted from them, and `TestCatchUpEqualsTrimAtEveryTrimPoint` still walks every trim point.
- `TestOpenWakeListMeasure` was log-only; the failing-surface assertions are in the two named tests above.
- Snapshot/recovery tests (`snapshot`, `restore_snapshot_test.go`) keep their boundary values (chunk crossing, newest-snapshot targeting).

## Top 50 slowest tests (after)

The 20 slowest overall are listed here; the full lists come from the `-json` runs and are not committed.

| s | test |
|---|---|
| 19.9 | store `TestListRunsByOwnerEquivalentToTheFullKindScan` |
| 18.6 | capsule `TestSeccompSelfTestThroughAChild` |
| 18.4 | snapshot `TestPageSizesOtherThan4096RoundTrip` |
| 17.4 | agentcore `TestCrashBootClosesOpenRunBehindManyFinishedRuns` |
| 16.1 | store `TestOpenRoundsAddressedToMatchesRecordFold` |
| 15.4 | searchplane `TestMonthlyPace_RouterNeverPassesTheTargetAcrossRestarts` |
| 15.2 | store `TestEventTableTrajectoryWindowOverTenThousand` |
| 15.1 | agentcore `TestPersistentManagementReport...Atomically` |
| 14.9 | store `TestRetireTelemetryAppliesThePolicy` |
| 13.0 | agentcore `TestPlanRestoreTargetBeforeNewestSnapshot` |
| 12.6 | cmd/taperepack `TestTaperepackToolPath` |
| 12.3 | capsule `TestCapsuleSeccompCloneRules` |
| 11.7 | agentcore `TestRematerializeRefusesFailureClasses` |
| 11.1 | capsule `TestBatchCommitDigestEqualsPerFileDigestOnThisRepo` |
| 10.6 | agentcore `TestActivationBudgetProgressDeadlineTerminalizesAndReleases` |
| 10.5 | agentcore `TestResearchCellFansOutToChildActivations` |
| 10.0 | store `TestTraceLogEmptyLogIsByteIdenticalToOriginMain` |
| 10.0 | agentcore `TestMediaStateLargeSeedSpansSeveralBatchesAndReplaysEquivalent` |
| 9.9 | objectgraph `TestValidatorFirstPassAndRepeatPassOnManyRows` |
| 9.9 | agentcore `TestSecondSnapshotOfTheRealStoreIsIncremental` |

No test is above 20 s any more, so wall time is now set by the number of tests, not by outliers. Remaining candidates for a later pass (not done, not classified): the other store trace-log tests (`TestTraceLog*`, which belong to R7's tracelog deletion and were not touched), `TestRetireTelemetryAppliesThePolicy`, `TestOpenRoundsAddressedToMatchesRecordFold`, the capsule seccomp tests, and `TestCrashBootClosesOpenRunBehindManyFinishedRuns`.

## CI rebalance

### Done: `scripts/go-test-split-weights`

Regenerated from the new `-race -json` run (tests of 2 s or more, store/platform/objectgraph/actorruntime only), with `@whole 150`. The old file had 213/141/102/88 s entries for tests that now take under 20 s, so the deal was placing work by stale numbers. `.github/scripts/go-shard-discovery-test` passes.

Computed per-shard time (split share plus whole-package load on shards 6 and 7), non-runtime race shards 0-7:

| | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | max | spread |
|---|---|---|---|---|---|---|---|---|---|---|
| before (origin/main tests, old weights) | 156 | 252 | 200 | 115 | 114 | 230 | 318 | 628 | 628 | 82% |
| new tests, old weights | 39 | 82 | 132 | 105 | 106 | 95 | 178 | 185 | 185 | 79% |
| new tests, new weights | 114 | 114 | 118 | 110 | 110 | 114 | 120 | 122 | 122 | 9% |

The critical shard drops from about 628 to 122 in this model (all in contended-VM seconds, not CI seconds). The 9% spread depends on the 120 s whole-package assumption. After the first CI run, check the `go-test-with-timing` output of shards 6 and 7. If they run longer than the others, raise `@whole`; if shorter, lower it.

### Proposed, not done

1. **Weight the runtime shards.** `scripts/go-test-runtime-shards` deals agentcore and textureowner tests round-robin by index. Using the measured durations, the shard sums are 133-177 s (about 25% spread: 146, 139, 133, 140, 135, 156, 177, 137). A weights file in the style of `go-test-split-weights`, dealt heaviest first, would bring that to a few percent. This is a script change that alters which tests each shard runs, so I left it for review.
2. **Total work:** store is now 682 s serial across 6 split shards, about 114 s each. With the outliers gone, the 8-shard layout is near its floor; fewer, larger shards would trade parallelism for per-shard setup (module download, `-race` build). If runners are the scarce resource, 6 non-runtime shards would finish within about 25% of today's critical shard. This needs a CI measurement before changing `.github/workflows/ci.yml` (matrix `shard: [0..7]` at lines 235 and 279, and `matrix.shard < 6` at line 247).
3. **Do not edit `ci.yml` from this branch.** The changes above are all in `scripts/`. Any job-graph change needs the reviewed trailers.
4. **Platform split weights** need a refresh from a CI run, since the local VM skips 310 platform tests.

## Verification

For each commit: `go build ./...`, `go vet` on the touched packages, touched packages' tests (store without race; the agentcore tests touched, with `-race`), `scripts/check-decode-roots.sh`, `scripts/check-test-hang-guards.sh`, `scripts/check-v1-writer-purity.sh` (all passing at the agentcore commit). The store package passed in full (215 s, no race, uncontended) before commit; the full non-runtime and runtime `-race` runs above show no failures in any touched package.
