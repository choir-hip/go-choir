# Duplicate Race Horn and Serialized SBOM Critical Path

Date: 2026-07-17 UTC
Status: observed; repair not yet applied
Mutation class: red (CI check gate, Race selection, and SBOM publication are protected surfaces)
Classification: CI assurance substrate

## Problem

The restored CI topology is correct but Pareto-suboptimal when `high_risk_race` or `sampled_race` is selected:

1. `ci.yml` first runs the ordinary three-shard non-runtime and four-shard runtime matrices.
2. It then invokes `race.yml`, which repeats the same package population under `-race` as four runtime shards plus one unsharded non-runtime job.
3. Differential SBOM generation starts only after the parent `check` gate, although deploy and FlakeHub depend on `check`, not on SBOM completion.

A selected-Race run therefore pays for two Go-test horns and serializes the longest supply-chain job after the check path.

## Evidence

GitHub Actions run `29550365185` at commit `e3de55581a1cae3ecce1431f5f4440ab01f62fc8` completed successfully with both horns:

- ordinary non-runtime shards: approximately 3m12s, 3m26s, and 2m59s;
- ordinary runtime shards: approximately 2m56s, 3m26s, 3m11s, and 3m27s;
- reusable Race non-runtime: approximately 12m10s;
- reusable Race runtime shards: approximately 5m32s, 6m41s, 6m20s, and 6m02s;
- parent `check`: approximately 12m28s from workflow start;
- differential SBOM: approximately 17m21s after `check`, extending the workflow to approximately 30 minutes.

These are public elapsed durations, not billed-minute evidence. The proposed latency and runner-minute reductions remain forecasts until matched hosted runs measure the replacement topology.

## Existing replacement opportunity

The repository already has the required substrate pieces:

- `scripts/go-test-non-runtime-shards` forwards extra arguments such as `-race` and already owns the three-way non-runtime package partition;
- `scripts/go-test-runtime-shards` forwards extra arguments such as `-race` and already owns the four-way runtime package/test partition;
- `ci-impact-classify` already emits the two inputs, `high_risk_race` and `sampled_race`, from which one authoritative `race_selected` output can be derived;
- `race.yml` remains useful as the complete scheduled/manual Race route, but its non-runtime population is needlessly unsharded;
- differential SBOM construction already produces a checksummed manifest and can be separated from accepted-baseline publication.

Connecting those existing pieces is cheaper and safer than adding another test topology.

## Required repair invariants

- Derive `race_selected` exactly once in `plan`; both Go matrices and `check` consume that output.
- A `ci.yml` change must select the consolidated Race topology it changes; a `race.yml` change must execute the modified standalone workflow through a path-filtered pull-request trigger rather than falsely selecting an unrelated inline route.
- A selected Race run substitutes `-race` on the complete ordinary 3+4 matrix population; it does not run a reduced Race sample.
- Preserve integration smoke on runtime shard 1.
- Preserve `TestCancelRunTrajectoryDrainsMoreThanOneActivePage`, which intentionally skips under Race, with one focused non-Race invocation rather than retaining the duplicate ordinary horn.
- Keep `race.yml` complete for scheduled/manual use and shard its non-runtime population without changing its dispatch contract.
- SBOM candidate construction may overlap tests, but a candidate is not an accepted baseline or durable artifact.
- Only a finalizer downstream of successful `check` may verify exact run/attempt/SHA identity, package cardinality, required-package success, file checksums, and differential consistency, then publish the accepted cache and durable artifact.
- A failed or cancelled check must be structurally unable to publish an accepted SBOM baseline.
- Deploy and FlakeHub continue to depend on `check`, not on SBOM finalization.
- No app/platform source, Node B route, direct workflow dispatch, or runner provider changes are admitted.

## Independent review

A five-agent architecture panel on the frozen observed topology converged on the substitution-and-overlap design. All usable reviews required one authoritative Race selector and fail-closed SBOM promotion. Reviewers also required complete Race coverage, explicit integration-smoke preservation, exact artifact identity/checksum verification, and hosted measurement before quantitative claims. One reviewer identified the Race-only skip in `trajectory_test.go`; this record promotes that edge case to a repair invariant.

Frozen implementation review then found four concrete defects: candidate-supplied required flags, differential records whose substantive contents were not verified, a standalone `race.yml` change that did not execute its modified workflow on pull requests, and a finalizer cache restore path that differed from the accepted cache save path. Rebased commits `1ba8d909` and `a3bf59a1` repaired all four. Follow-up reviewers confirmed the Race route/aggregate selector and end-to-end SBOM promotion surfaces have no remaining blocker.

## Rollback

Revert the bounded workflow/scripts commits through a pull request to restore the complete reusable Race call and post-check SBOM job. The fresh pre-repair workflow at `origin/main@ba1fd5a4973618326c8eebe9b14456941724c114` is the behavioral rollback reference. No Node B rollback is expected because CI-only landing must classify `deploy_needed=false` and skip deployment.
