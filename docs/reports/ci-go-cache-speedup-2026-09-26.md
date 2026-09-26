# Go CI compilation speedup

The first hosted candidate passed all 16 main Go shards, vet/build, scale,
contracts, documentation and vocabulary checks, plus all eight weekly Race
shards. No tests, race policy, deployment prerequisites or acceptance checks
were removed. This is a CI maintenance repair, not a product mission cutover.

## Changes and evidence

Problem record (committed first):
[cache starvation](../problems/ci-go-cache-starvation-2026-09-26.md).
Implementation: `becbed88f21c5f14dee8f6edcbe8cf6c4357b8d1`.
PR: https://github.com/choir-hip/go-choir/pull/67.

- Heavy jobs now cache by workload, race mode, runner image, actual Go compiler,
  dependency metadata and source content. Same-workload prefix restores reuse
  unchanged objects across source revisions. Lightweight jobs cannot seed these
  namespaces. A cache never substitutes for running tests (`-count=1` remains).
- Weekly Race uses the same maintained cache namespaces instead of the old
  unwritten `go-build-v2` namespace. Identical store/runtime compilation can be
  reused across test-name shards; different non-store package partitions remain
  separate.
- Discovery uses execution flags, eliminating a standard build before a race
  build. A failed listing now fails the shard, even with partial stdout.

Baseline: https://github.com/choir-hip/go-choir/actions/runs/36193752882.
First candidate CI: https://github.com/choir-hip/go-choir/actions/runs/36207153276.
Weekly Race: https://github.com/choir-hip/go-choir/actions/runs/36207153265.
Raw step/job timing evidence:
[JSON](../evidence/ci-go-cache-timings-2026-09-26.json).

| Representative job | Baseline | Candidate | Cache state |
|---|---:|---:|---|
| Runtime shard 3 | 481s | 313s | cold candidate |
| Store shard 4 | 439s | 248s | cold candidate |
| Runtime shard 4 | 468s | 128s | warm candidate |
| Store shard 2 | 423s | 81s | warm candidate |
| Vet/build | 235s | 234s | cold candidate |
| Scale | 164s | 226s | cold candidate; slower |
| Other packages shard 7 | 223s | 346s | cold candidate; slower |

These are job durations, including setup/cleanup but excluding queue time.
Candidate cache hits were possible for queued shards after earlier siblings
saved entries. Store shard 2 logs prove an exact hit on the new 789 MiB cache,
28s setup and 46s test step. These are observational comparisons on nearby
source revisions, not controlled same-source A/B results or end-to-end deploy
speed claims. Cold cache costs and runner variance explain why not every job
improved. A same-source warm rerun is the next measurement.

Local checks: shell regression harness (flags, exhaustive/disjoint selection,
quarantine, partial discovery failures, execution failures, invalid shard),
classifier and CI/deploy contracts, differential SBOM contracts, pointer
resolver contracts, Bash syntax and actionlint all passed.

## Authority, landing and residual risk

Mutation class red: protected CI assurance surface. Conjecture strengthened:
most race-shard time was compilation/cache starvation, not test execution.
Heresy delta: discovered and repaired cache starvation and hidden discovery
failure; no introduced authority paths. Rollback: revert the implementation
commit (after merge, revert its landed equivalent). Cache entries are optional
and need no cleanup for rollback.

No product trajectory, run acceptance, provider call, VM lifecycle or canonical
write is changed or claimed. The touched paths classify `deploy_needed=false`.
However deployment compares against the *deployed* base: staging initially
lagged main's unrelated runtime edits. Another agent pushed R2x `ebdaef45` while
this PR was validating, so wait for that CI/deploy before landing to avoid
cancelling it or unexpectedly deploying its earlier changes.

Staging observation before landing: `/health` healthy at deployed commit
`537fce042a7ef61887ab8247a336d7dfa1254a0f`. Product acceptance IDs and levels:
not applicable; acceptance here is hosted CI execution with unchanged gates.

Caches cost about 0.8–0.9 GB per heavy namespace; eviction may cause recompiles.
Main cannot restore PR-scoped caches, so its first run must seed its own entries.
The ~12-minute Node B Nix prebuild remains the deployment floor and already
runs concurrently with tests. Next realism axis: warm main source-change runs,
cache eviction/restore overhead, then separately profile Nix builds without
weakening exact identity or activation gates.

The original worktree's concurrent runtime changes were preserved by using
an isolated worktree; no stash or edit of the other agent's files was needed.
Temporary logs/tools are outside the repository. Product mission pointers
remain owned by the other agent.
