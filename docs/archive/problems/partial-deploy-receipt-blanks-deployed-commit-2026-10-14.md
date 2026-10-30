# A Partial Host Deploy Blanks the Deployed Commit, and Later Deploys Skip

Date: 2026-10-14. Found by the lead at ~20:15 UTC when the jailer slice
(10e9f18e1) never reached staging. Status: **landed** in e7bb34f52 (from branch
`fix/deploy-receipt-platform-commit`; see "Fix" below). The platform
commit is `/health` `.platform.deployed_commit`. `build.deployed_commit`
stays per service, so it is blank after a deploy that did not move the
proxy, by design. Verified 2026-10-15 18:20 UTC: `.platform` =
77633ecdc, source `receipt.deployed_commit`.

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

## Failure modes before the fix (2026-10-14)

Mutation class: red (deployment routing: the receipt writer, the deploy
base, and the execution-identity join that reads the receipt). Written
before code; each mode names the test that pins it.

Settled decisions this fix must keep:

- `buildinfo.Info.deployed_commit` is per-service ("this deploy moved
  this binary"). The execution-identity join (proxy and `choir identity`)
  rejects a host whose proxy was not selected but which claims the
  target ("unselected deployed"), and guests require
  `deployed_commit == commit`. Redefining that field as the platform
  commit breaks both. The platform commit is a separate answer.
- `Info` is inside the guest-signed execution identity and the proxy
  decodes it with `DisallowUnknownFields`. A new `Info` field from a
  guest the deploy moved, read by a proxy it did not, fails the join.
  So the platform commit lives on `/health` beside `build`, not in it.

Receipt merge (`.github/scripts/deploy-receipt-merge`, its test):

- M1 untouched service dropped (this incident): the previous receipt's
  artifacts for services this deploy did not touch are carried forward
  with their own commit and status.
- M2 stale entry wins: an artifact this deploy touched always replaces
  the carried one.
- M3 carried entry read as current identity: carried entries are marked
  `carried_forward_from` (the target of the receipt that carried them,
  kept from the first carry), and per-service identity stays strict
  (`commit == target_commit`), so a carried proxy never reports the
  target.
- M4 missing, corrupt, non-v1 or incomplete previous receipt: the merge
  warns and writes this deploy's artifacts only; it never blocks the
  receipt (a blocked receipt would freeze the platform commit forever).
- M5 invalid new receipt (bad target, no artifacts): the merge fails and
  no receipt is written, keeping the existing refusal.
- M6 the platform answer: `deployed_commit` = `target_commit` (receipts
  are written only after a fully verified deploy; incomplete deploys
  write `deploy-failures/`, never the receipt).

Readers (`internal/buildinfo`, `internal/proxy`, `cmd/choir`):

- R1 strict decoder: the proxy's identity receipt decoder disallows
  unknown fields; `deployed_commit` must be declared there or execution
  identity breaks on the first merged receipt.
- R2 conflicting platform field (`deployed_commit != target_commit`):
  readers report nothing rather than trust either.
- R3 old receipt (no `deployed_commit`): the platform answer falls back
  to the per-service rule.
- R4 carried proxy artifact in the execution-identity join: treated as
  "not selected by this deploy" (host must not claim the target); an
  unmarked artifact whose commit differs from the target is still a
  conflict. Mirrored in `choir identity`.

Base selection (`.github/scripts/deploy-base-select`, its test):

- B1 `/health` blank: never the push before-SHA. Candidates, in order:
  `/health` `platform.deployed_commit`, `/health`
  `build.deployed_commit`, the Node B receipt over SSH
  (`deployed_commit // target_commit`, not an incomplete one), then the
  proxy binary's `build.commit` (older than or equal to the platform
  commit, so its diff is a conservative superset).
- B2 candidate is not a full SHA or does not resolve (shallow clone,
  fetch fails): skipped, next candidate.
- B3 no candidate: deploy-impact deploys everything (the same classes as
  a forced dispatch) and says so; the plan job runs every lane.
- B4 base equals the head (a re-run of a deployed commit): deploy-impact
  has nothing to deploy; the plan job tests the push delta.
- B5 SSH hangs: bounded by a connect timeout and an overall timeout;
  failure is just a missing candidate.

Rollback hazard: reverting this change leaves a merged receipt (with
`deployed_commit`) on Node B that the old proxy's strict decoder rejects,
so execution identity is unavailable until the revert's own deploy
rewrites the receipt in the old shape (the window is that one deploy).

## Fix

- `.github/scripts/deploy-receipt-merge` (test:
  `deploy-receipt-merge-test`): the Node B receipt writer merges the new
  receipt into the previous one (M1-M6). Schema stays 1; the new
  top-level `deployed_commit` and per-artifact `carried_forward_from` are
  additive.
- `internal/buildinfo.PlatformSnapshot` and proxy `/health`
  `platform.{deployed_commit,deployed_at,source}`: the platform answer,
  beside `build`; `build.deployed_commit` keeps its per-service meaning.
- `internal/proxy` identity receipt decoder declares `deployed_commit`
  and rejects a conflicting one; the proxy and `choir identity` joins
  treat a carried proxy artifact as unselected (R1-R4).
- `.github/scripts/deploy-base-select` (test: `deploy-base-select-test`):
  one base chooser for the plan job and deploy-impact; deploy-impact
  reads the Node B receipt over SSH (best effort) and deploys everything
  when no base is trustworthy; the plan job runs every lane (B1-B5).
