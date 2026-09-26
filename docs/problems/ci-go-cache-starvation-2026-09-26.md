# Go CI cache starvation and duplicate discovery compilation

Date: 2026-09-26 UTC
Status: observed; bounded repair under verification
Mutation class: red (CI assurance substrate; deployment activation unchanged)

## Evidence and causal assessment

Hosted run 36193752882 at 537fce04: runtime race shards took 362–481s,
store race shards 423–447s, vet/build 235s. Runtime shard 3 (job 108264758078)
restored an 11,569,582-byte setup-go cache created 2026-08-26. Discovery began
after setup at 21:51:35 and selected its first tests at 21:54:27 (172s).
The test step ended 21:59:20. Its post step said the primary key was hit and
therefore no cache was saved. The cache key includes Go version and go.sum,
not source revision, build mode, or job workload. A tiny cache can win once
and prevent all heavy jobs from publishing their compiled work indefinitely.
The cache API confirmed that same August entry was still used September 26.

Both test-name shard runners use an uninstrumented `go test -list` before
`go test -race`: cold runners compile the same dependency graph in two modes.
Discovery also hides compile failures in process substitution and treats an
empty test list as success. This is assurance debt discovered during analysis.

A second baseline, 36169334693, shows runtime shards 349–495s, store shards
363–455s, vet/build 237s. Deploy prebuild takes 722–723s; activation job takes
356–369s because prebuild already overlaps tests. Faster CI alone will not
remove the remaining Nix build critical path. Do not claim otherwise.

## Replacement and bounded change

Use existing actions/setup-go with its cache disabled for heavy lanes, and
standard actions/cache with workload/build-mode/toolchain/source-aware keys
and same-workload restore prefixes. Keep lightweight docs/SBOM caches separate.
Use the same composite action in scheduled Race, whose old go-build-v2 cache
namespace currently has no writer. No new build orchestration service.

Pass execution flags to discovery and propagate discovery errors. Preserve
all shard counts, quarantine selection, test execution, race policy, check
requirements, and deployment prerequisites. Cache hits must never skip tests.

## Conjecture, evidence boundary, rollback

Conjecture delta: repeated compile latency is partly cache starvation, not
only expensive tests. Matching discovery mode removes redundant compilation;
warm hosted runs must measure actual savings. Cold cache seeding may cost
extra upload time. Cache size/eviction remains a monitoring concern.

Failure cases to verify before implementation: discovery compile failure
(including partial stdout), race/tag flags lost, tests duplicated or omitted
across shards, quarantine leakage, test command failure hidden, invalid shard
arguments, cache collisions across lanes/modes/toolchains, stale cache preventing
refresh, cache hit bypassing test execution, weekly Race restoring an unwritten
namespace. Test harnesses may fake Go to verify shell control flow; only hosted
Go jobs establish CI acceptance/performance. No product acceptance claim.

Protected surfaces: CI assurance only. No deployment routing, provider, VM,
canonical write, or product-state edits. Heresy delta: discovered cache
starvation and swallowed discovery errors; introduced none intended; repaired
only after checks. Rollback: revert bounded repair commit; old uncached work
remains correct, slower behavior. Keep the other agent's R2x mission pointers
and runtime WIP untouched: this is a separate maintenance change, not a new
product mission station.

Sources: https://github.com/choir-hip/go-choir/actions/runs/36193752882 and
https://github.com/choir-hip/go-choir/actions/runs/36169334693; cache semantics:
https://github.com/actions/cache/blob/v6/README.md.
