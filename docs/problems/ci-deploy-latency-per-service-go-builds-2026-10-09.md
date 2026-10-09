# Deploy latency: 15 Go services each compile the full dependency graph on Node B

Date: 2026-10-09. Status: open (proposal). Owner direction: "improve ci to
be faster". Mutation class of a fix: red (deploy artifacts and routing).

## Evidence

Run 37890087817 (50c7074b, Go + Nix change, full deploy) took 1,647 s
wall-clock:

| Job | Duration |
|---|---|
| Build Differential SBOM Candidate | 1,531 s (off the deploy path; gates only SBOM acceptance) |
| Prebuild Deploy Artifacts (Node B) | 1,029 s |
| Deploy to Staging (Node B) | 912 s (started 05:51 after checks, done 06:06) |
| Slowest Go test shard | 355 s |

Push-to-staging was about 21 min: ~6 min of checks, then ~15 min of deploy.
The deploy is dominated by building the NixOS toplevel and guest image
on Node B.

`flake.nix` `mkGoService` defines **15 separate `buildGoModule`
derivations**, one per `cmd/*`. Each derivation:
- compiles the shared dependency graph (embedded Dolt, the store,
  agentcore…) in its own sandbox, with no shared Go build cache;
- depends on the repository source, so any Go change rebuilds all 15.

Since 16806d8e, Node B builds run with `max-jobs=2`, `cores=6` inside a
7G memory slice
([`node-b-memory-overcommitted-swap`](node-b-memory-overcommitted-swap-2026-10-09.md)).
That removes the swap pressure but serializes these redundant compiles
further. Observed on 2026-10-09: two concurrent toplevel builds (the
prebuild of run N+1 overlapping the deploy of run N) at load 61.

## Proposal (ordered by payoff and risk)

1. **One Go build, many outputs.** A single derivation runs
   `go build ./cmd/...` for every service with one build cache. Thin
   per-service derivations copy their binary plus `build.json` from it,
   so service pointers and `verify_artifact_manifest` keep per-service
   identity. Expected: one dependency-graph compile instead of 15.
2. **No duplicate toplevel builds.** Serialize the Node B prebuild of run
   N+1 behind the deploy of run N (one concurrency group for Node B
   builds), or drop the prebuild when a deploy for a newer SHA is already
   building.
3. **Build off the production host.** Build the toplevel and guest image
   on a builder and push to a binary cache Node B substitutes from. The
   production host then never compiles beside guests; this matches fix
   item 5 of the memory problem.
4. **Faster check-to-deploy hand-off.** The deploy job waits on the full
   `check` fan-in (~6 min) although the Node B build could start at once.
   The prebuild already does this; making the deploy reuse the prebuild's
   result instead of rebuilding is part of item 2.

Item 1 is self-contained and verifiable: compare toplevel build time and
per-service `build.json` identity before and after on one deploy.
