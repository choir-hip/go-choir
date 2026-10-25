# CI test pruning manifest (CI0, slice C5), 2026-10-24

Every deleted or merged test gets a row here: what it covered, the category
(duplicate of a named test, tests deleted code, superseded by a named test,
asserts nothing), and why that coverage stays. Tests of red surfaces (tape and
evidence, auth and session, restore and checkpoint, Texture canonical writes,
store schema, vmctl, gateway calls) are only proposed, in the second section.

## Removed

None. Rows are added only with the deleting commit.

## Sped up without losing an assertion (not removals)

| Test | Change | Assertions kept | Before |
|---|---|---|---|
| internal/store TestEventTableTrajectoryWindowOverTenThousand | the three stores fill in parallel subtests (`fill/off`, `fill/on`, `fill/toggled`) instead of one after another; the reads and every assertion are unchanged | the fixture check on today's window, and both `limit` reads against `on` and `toggled` | 489 s in CI (race) |

## Proposed (red surfaces; FP0 adjudicates with a Devin seat)

None yet. The slow store tests below are candidates for speeding, not
deletion; each asserts a bound or an equivalence that no sibling test asserts
(read from the test source, not from names):

| Test (internal/store unless noted) | CI seconds (race) | What the time is | Proposal |
|---|---|---|---|
| TestListRunsByOwnerEquivalentToTheFullKindScan | 139 | seeds thousands of runs, then compares two read paths | seed in larger `PutBatch` groups; keep the equivalence |
| TestOpenWakeListMeasure | 104 | seeds 3,000 outbox rows and times the read five times per path; also asserts the open list | `WAKE_SCAN_ROWS` default down to the row count at which the JSON and key-index reads differ; keep `sameIDs` |
| TestListRunsByOwnerCostDoesNotGrowWithLifecycleRuns | 70 | cost-bound check at 2,000 lifecycle runs | the reference sizes are the assertion; speed only |
| TestReopenWithManyRowsDoesNotTouchOgObjects | 60 | reopen of a many-row store | speed only |
| objectgraph TestValidatorFirstPassAndRepeatPassOnManyRows | 78 | validator over many rows | speed only |
| objectgraph TestDerivedKeyReadsMatchJSONReads | 52 | equivalence over many rows | speed only |

## Why there are no removals yet

A heuristic scan (a `Test*` body with no `t.Fatal`/`Error`/`Fail` call, no
assert/require word and no helper taking `t` as its first argument) listed 14
candidates. It is a lead, not a verdict: reading five of them showed the
heuristic missing assertions made through helpers and child processes
(`testRuntime(t)`, `newSessionEvaluator`, helper-process tests). Two are census
tests that skip unless an environment variable is set (internal/store
TestJournalAmplificationCensus and TestStallHoldersCensus), so they cost
nothing in CI. No test was confirmed to be a duplicate, to test deleted code,
to be superseded or to assert nothing, so nothing was removed. The slow tests
are slow because of what they seed, not because of what they assert, which is
why the work here is speed, not deletion. The nine remaining scan candidates
(yaegikernel, capfloor, agentcore) were not read.
