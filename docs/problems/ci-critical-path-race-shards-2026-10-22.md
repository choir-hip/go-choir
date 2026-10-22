# CI critical path: two heavy race shards, a 20-runner queue, and the Node B build floor (2026-10-22)

Date: 2026-10-22. Status: open; fix on `lane/fp0-ci-shard-balance`.
Mutation class of the fix: orange (CI workflow and the test shard planner;
no product behavior). Rollback: revert the fix commit.

## Sample

- 27 successful race-mode CI runs on main, 39655640676 to 39830120673
  (2026-10-21 18:45 to 2026-10-22 18:18 UTC), 21 of them with a Node B
  deploy. 5 standard-mode runs are consistent and not tabled; 8 docs-only
  runs are excluded.
- Job and step times come from `gh run view --json jobs`. Package and test
  times come from the `go test -json` lines in the 64 non-runtime shard logs
  of the last 8 race runs. Phase lines come from the deploy and prebuild logs
  of runs 39816936195 and 39830120673.
- All times are seconds after **Plan CI Lanes** ends, unless the table says
  otherwise.

## Where the minutes go (race runs, median of 27)

| Segment | Median | Spread | Note |
|---|---|---|---|
| Run created, then plan starts | 3 | max 529 | queued behind the previous main run (workflow concurrency group) |
| Plan CI Lanes | 10 | max 44 | |
| agentcore/textureowner shards 0-7, job | 218-283 | p90 297-368 | setup about 25 s, test step 175-232 s |
| non-runtime shards 0-5 (store), job | 175-207 | p90 223-276 | test step 147-178 s |
| **non-runtime shard 6**, job | **296** | p90 446, max 633 | last 8 runs: 340-540 |
| **non-runtime shard 7**, job | **355** | p90 484, max 587 | last 8 runs: 190-320 |
| scale, vet+build, doccheck, heresy, vocabulary | 7-112 | | |
| `check` starts | 395 | 146-670 | |
| Detect Staging Deploy Impact ends | 12 | | |
| **Prebuild (Node B nix build) starts** | **110** | 65-125 | waits for a runner slot (finding 3) |
| Prebuild runs | 452 | p90 496 | the host NixOS closure build is 418-441 s of it |
| Prebuild ends | 556 | 480-675 | |
| Deploy to Staging starts / ends | 456 / 750 | | 285 s job, mostly waiting on the Node B build |
| Build Differential SBOM Candidate | 584 | | starts at +3; off the deploy path |
| Accept Differential SBOMs ends | 735 | | the run's tail in about half the runs |

The last gate job to finish before `check` was non-runtime shard 7 in 15 runs,
shard 6 in 6 runs, and another job in 6 runs.

## Findings

### 1. Shards 6 and 7 carry every heavy non-store package

`scripts/go-test-non-runtime-shards` with `TOTAL_SHARDS=8` gives
`internal/store` to shards 0-5, split by test name. It gives the other
~142 packages to shards 6 and 7, dealt out by list index parity. Which shard
gets the heavy packages depends on that parity, so adding or removing one
package moves them. On 10-21 and early 10-22, shard 7 was the slowest; in the
last 8 runs it is shard 6.

Package time (`Elapsed` of the package's pass line, median of the last 8 race
runs):

| Package | Shard | Elapsed (s) | Tests | Longest test (s) |
|---|---|---|---|---|
| internal/platform | 6 | 266 | 261 | 20 |
| internal/objectgraph | 6 | 240 | 76 | 49 |
| internal/projectionbase | 6 | 150 | 50 | 24 |
| internal/actorruntime | 7 | 135 | 61 | 13 |
| internal/capsule | 6 or 7 | 57 | | |
| every other package | 6 / 7 | under 40 each, 484 together | | |
| shard total | 6 / 7 | 896 / 489 | | |

None of the four heavy suites uses `t.Parallel`. Each runs as one serial
binary for 2-4.5 min, so shard 6's test step (288-488 s) ends when
`internal/platform` ends. These suites can be split by test name in the same
way as store.

On the store shards, `internal/store` (469 tests) runs as a single serial
binary. Its test step takes 70-190 s, and 3 of the runner's 4 CPUs are idle
for that time.

### 2. The deploy usually waits on the Node B build, not on the tests

The prebuild ended after `check` started in 17 of the 21 race runs that had a
deploy, and the two ended within a second of each other in one more. In those
runs the deploy job starts first and then waits: its
`nix build` phase ends 3-4 s after the prebuild's host closure build, in both
logs read (39830120673 waited 89 s, 39816936195 waited 153 s). In the 3 runs
where the tests ended later (39725541974, 39777458724, 39822263499), the Go
caches were cold (finding 4).

A replay of the 21 deploying runs, using their real job durations, a 20-slot
FIFO runner queue and the deploy waiting on the prebuild, matches the real
runs to within about 10 s at the median. In that replay:

- With shards 6 and 7 balanced (assumed no longer than the median store shard
  plus 30 s, and every store shard 30 s longer), `check` starts at 323 s
  instead of 425 s: 54 s earlier per run at the median, and up to 210 s
  earlier.
- The median deploy end does not move (745 s), because the Node B build sets
  it.

So balancing the shards gives an earlier CI verdict (pull requests,
non-deploying pushes, and every run once the Node B build is faster), not an
earlier median deploy.

### 3. Runner slots: the prebuild queues behind the test shards

The highest concurrency is exactly 20 jobs in 26 of the 27 full runs (19 in
the other), the account's concurrent-job limit. When plan ends, 24-25 jobs become ready: 16 test
shards, scale, vet+build, govulncheck, doccheck, heresy, vocabulary,
deploy-impact, the SBOM candidate, and npm audit when frontend files change.

- One or two of them start 25-75 s late in most runs.
- The prebuild becomes ready at +12 and starts at +110 (median). Every time
  it starts, 19-20 other jobs are running. It gets a slot only after the
  shards that were already queued.
- The report-only govulncheck (113 s) and npm audit hold slots during this
  window, although nothing waits on them.

### 4. Cold Go caches from cache eviction

In 2 of the last 8 race runs (39777458724, 39822263499), no non-runtime race
lane restored any cache (`Cache not found`). The go.mod hash was the same as
in the warm runs, so the entries had been evicted. `gh cache list` at 19:20
UTC: 10.54 GB in use against the 10 GB repository limit. Two vet-build
entries take 3.88 GB, and no race lane is present at all. These are the
slowest runs (`check` at +644 and +670).

### 5. Queueing between runs

The workflow-level concurrency group runs main runs one at a time. A push
that arrives during a run waits for that run's tail, which is the later of
the deploy end (750) and the SBOM acceptance end (735). The longest wait
observed before plan started was 529 s.

## Fix in this lane

1. **Shard planner** (findings 1 and 2):
   - A tracked list, `scripts/go-test-split-packages`, names the packages
     that are split by test name: store, platform, objectgraph,
     projectionbase and actorruntime. Shards 0-5 run them all in one
     `go test`, so their binaries run in parallel.
   - Each test name goes to one shard for every listed package that has it,
     so each (package, test) pair runs exactly once.
   - Every other package runs whole on shard 6 or 7.
   - The run fails, instead of passing, when `go list` fails, when test
     discovery fails, or when the list names a package that is not a
     non-runtime package.
   - Discovery also lists `Fuzz` targets, whose seed corpora `-run` already
     executes.
   - `.github/scripts/go-shard-discovery-test` is extended first, so that no
     package or test can be dropped or run twice.
2. **Report-only jobs** (finding 3): govulncheck and npm audit wait for
   `check` and run unless the run is cancelled. This frees two of the 20
   slots in the first two minutes. In the replay the prebuild then starts at
   69 s instead of 115 s, and the deploy ends 39 s earlier (median per run;
   up to 77 s).
   No gate changes: neither job feeds `check` or the deploy.
3. **Not changed in this lane, recorded as residuals:**
   - The Node B host closure build (about 440 s) is the deploy floor.
   - The deploy tail after the build: active computer refresh about 125 s,
     service activation 12-62 s.
   - Cache eviction (finding 4).
   - Queueing between runs and the 584 s SBOM candidate (finding 5).
   - Starting the prebuild from inside deploy-impact so it never queues for
     a slot. That would change the prebuild's concurrency group and a
     red-class contract.
