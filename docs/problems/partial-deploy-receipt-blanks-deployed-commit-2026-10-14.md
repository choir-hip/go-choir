# A Partial Host Deploy Blanks the Deployed Commit, and Later Deploys Skip

Date: 2026-10-14. Found by the lead at ~20:15 UTC when the jailer slice
(10e9f18e1) never reached staging. Status: documented; workaround is a
manual forced deploy (workflow_dispatch); fix to follow.

## Evidence

- CI run 38624806929 (023ab7312, 19:39 UTC) deployed one host service
  only. The new /var/lib/go-choir/deploy-receipt.json has
  schema_version 1, target_commit 023ab7312… and exactly one artifact
  (not the proxy).
- internal/buildinfo reads the deployed commit for a service from that
  service's artifact in the receipt; the proxy has no artifact in it, so
  `/health` reports `build.deployed_commit: null` (proxy binary still at
  efbd69e27).
- CI's deploy-impact plan diffs from the deployed commit read from
  `/health`; with null it falls back to the push's own before-SHA. The
  jailer run (10e9f18e1) was cancelled by a later push; the next run
  (f8a7dc176, docs only) diffed only its own docs commit, found no deploy
  impact, and skipped "Deploy to Staging". The jailer, the lab and every
  code change since efbd69e27 are not on staging.

## Cause

The receipt is written per deploy with only the artifacts that deploy
touched, replacing the previous receipt instead of merging; the health
identity is per service; the CI base depends on the proxy's entry.

## Fix directions

- Merge receipts: carry forward artifacts of services a deploy did not
  touch (with their own commits), and record the platform's deployed
  commit as the target of the last successful deploy regardless of which
  services moved.
- CI: never fall back to the push before-SHA when /health has no deployed
  commit; read the receipt's target_commit (or the last successful deploy
  run's SHA) instead, and fail loudly if neither is available.
