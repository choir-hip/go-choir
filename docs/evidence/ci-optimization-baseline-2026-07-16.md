# CI Optimization Baseline — 2026-07-16

This receipt records public GitHub Actions timeline evidence. It is evidence for
job selection and duration, not an authorized billing statement.

## Pre-pause full-signal run

- Run: [29295978398](https://github.com/choir-hip/go-choir/actions/runs/29295978398)
- Commit: `488664d98b7466f47b7639607ef318b241be44e7`
- Event/ref: `push` to `main`
- Started/updated: `2026-07-14T00:27:47Z` / `2026-07-14T00:52:02Z`
- Workflow wall clock: 24m15s.
- Summed raw duration of jobs with runners: 5,115s (85m15s).
- Sum of each job duration rounded up to whole minutes: 99 estimated runner
  minutes. This is telemetry only, not actual billed minutes.
- Race selected all five reusable-workflow jobs: four runtime shards and one
  non-runtime job. Durations were 10m40s, 10m27s, 6m05s, 5m48s, and 5m38s.
- `check` succeeded after race.
- SBOM job `86971008658` started after check and succeeded in 13m15s.
- Deploy and FlakeHub began alongside SBOM after check; neither waited for SBOM.

The checked-in script at this commit already contained the host-side topology
introduced by `c96c7b49`: build the package on the runner, build pinned sbomnix
from `flake.lock`, and invoke sbomnix outside the package's Nix sandbox.

## Current paused run

- Run: [29468123745](https://github.com/choir-hip/go-choir/actions/runs/29468123745)
- Commit: `d87bdc446ecc28585c3bc08d4d469b9f94d3c246`
- Event/ref: `push` to `main`
- Started/updated: `2026-07-16T03:05:06Z` / `2026-07-16T03:10:59Z`
- Workflow wall clock: 5m53s.
- Summed raw duration of jobs with runners: 1,911s (31m51s).
- Sum of each job duration rounded up to whole minutes: 40 estimated runner
  minutes. This is telemetry only, not actual billed minutes.
- Race and SBOM were skipped by literal pause conditions.
- Deploy was skipped; FlakeHub succeeded.

The two runs have different commits and path impact. Their difference ranks
where to investigate but cannot prove that pausing the two lanes caused the
entire wall-clock or runner-duration difference.

## Candidate distinctions

- A common cache key can enable reuse across later runs; it cannot let jobs
  starting simultaneously share one compilation.
- Serialized cache priming changes the critical path and needs its own matched
  run.
- Same-run artifact transfer requires an explicit producer/consumer topology
  and needs its own matched run.
- Ripgrep installation is conditional on `command -v rg`; its baseline setup
  duration must be measured before proposing a cache/removal change.
- Deploy classifier and workflow-contract tests run in `plan` only when
  `ci=true`, but run in deploy-impact for every non-doc main push. Removing the
  latter would narrow coverage and is rejected absent an equivalent all-path
  contract.

## Evidence acquisition

Run and job metadata were fetched from GitHub's public Actions REST resources
on 2026-07-16. The job receipts expose start/end times and conclusions. No
billing export, organization metrics authority, or private billing API receipt
was available, so this document makes no actual billed-minute claim.

