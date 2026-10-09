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

## Item 1 landed: fe5bbcbf (run 37906238438, 2026-10-09)

| Job | Before (f8a968dd, 8fd243ba) | After (fe5bbcbf) |
|---|---|---|
| Prebuild Deploy Artifacts (Node B) | 15.0–16.2 min | **5.8 min** |
| Deploy to Staging (Node B) | 12.6–13.0 min | **7.5 min** |
| First job start → deploy done | 17–19 min | **8.1 min** |

The run itself still took 18 min wall-clock: it queued ~10 min behind the
previous run's Node B concurrency group (item 2 is still open).

**Regression introduced by fe5bbcbf:** "Build Differential SBOM
Candidate" failed with `semantic fingerprint Go toolchain mismatch:
Nix=unknown setup-go=1.26.4`. Both `build-sboms-differential` and
`verify-sbom-candidate` find the Nix Go version among the **direct**
input derivations of `packages.auth`. `auth` is now a copier whose only
Go-built input is `go-choir-services`, so the probe finds nothing. Fix:
read the recursive derivation graph and take Go from the
`go-choir-services` derivation. The deploy is not gated on this job, so
staging was not affected; the SBOM acceptance was skipped for this SHA.

## Items 3 and SBOM fix landed (2026-10-09 ~10:15Z)

- 275b02f7: the SBOM Go-version probe reads `go-choir-services`. The
  c01bc2f9 run passed "Build Differential SBOM Candidate" and "Accept
  Differential SBOMs".
- **Deploy to Staging (Node B)** with the shared Go build: 2.7 min on
  c01bc2f9, 1b6c1b73 and 63c3263c (was 12.6–13.0 min).
- 63c3263c: Node B builds on node-a over ssh-ng
  (`nix.distributedBuilds`, key restricted to `nix-daemon --stdio` on
  node-a), with local fallback. Verified: a forced remote build
  (`--max-jobs 0`) from Node B was built on node-a and copied back. The
  first deploy built with the old configuration; the next deploy is the
  first to use node-a.

Open: item 2. Workflow-level `concurrency: ci-refs/heads/main` serializes
whole main runs, so a push waits for the previous run to finish (~10 min
seen for fe5bbcbf). The deploy job already has its own concurrency group
and a stale-target guard, so main runs could run checks in parallel.
This changes the deploy pipeline (red), so it is a proposal, not a
change.

## Hole: a docs push after a cancelled code run deploys nothing (2026-10-09 12:15Z)

Sequence:
1. Code commit 9c0eac3b was pushed while 964a68ea's run was in progress,
   so its run queued.
2. Docs commit 3b860cd7 was pushed next. GitHub replaced the queued
   9c0eac3b run (cancelled); a group keeps one running and one pending.
3. The 3b860cd7 run's "Plan CI Lanes" classified the push as docs-only
   from the push's own before/after. It skipped tests, "Detect Staging
   Deploy Impact" and the deploy. Staging stayed on 3ef77efb, without
   c0903096, 3ef77efb's successors 4b9bf31d, 964a68ea or 9c0eac3b.

Deploy-impact does diff against the live staging commit, but it never ran
because the planner gated it on push-local paths. Workaround used:
`gh workflow run ci.yml -f force_staging_deploy=true` (run 37928689108).

Fix direction: on main, the planner's docs-only shortcut must use the
same base as deploy-impact (the live staging commit), not the push's
`before`. Or make item 2 moot: stop serializing whole main runs, so a
queued code run is never replaced by a docs run.

**Fixed (6d6db691, 12:24Z):** on main, the planner now uses the commit
staging reports in `x-choir-build-commit` as its base, falling back to the
push `before` if the header is missing or unfetchable. First run 37929273657
logged `Plan base: live staging commit ae61f153…` and planned go=true.
Item 2 (whole-run serialization) stays open, but it can no longer hide
undeployed code.

## Item 2 decision: keep whole-run serialization on main (2026-10-09 14:50Z)

The proposal was to give each main commit its own workflow-level group and
rely on the job-level deploy group plus the stale-target guard. Rejected
before landing, for two reasons:

1. **Deploy ordering can invert.** GitHub concurrency keeps one running and
   one pending job per group, and a newly queued job cancels the pending one
   whatever its commit. With parallel main runs: deploy A is running, run C
   finishes checks first and its deploy goes pending, then older run B
   finishes and its deploy replaces C's. B deploys, the guard passes (live
   A is behind B), and C's deploy no longer exists. Staging silently stays
   on B without C, which is the hole class fixed in 6d6db691.
2. **Duplicate builds come back.** Parallel runs let run N+1's prebuild
   build beside run N's deploy on Node B (separate groups), which is the
   "two concurrent toplevel builds" this item was opened to remove.

Serialized main runs, with one pending run that coalesces any number of
later pushes, plus the live-staging plan base, give the right result with
the fewest builds. The cost is latency: a push waits for the run ahead of
it. If that becomes the bottleneck, the fix is a catch-up step at the end
of the deploy job (if `origin/main` moved past the deployed commit with
deploy impact and no run is queued for it, dispatch a deploy for the head),
not parallel runs. Item 2 is closed; serialization is intended.
