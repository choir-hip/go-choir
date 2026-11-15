# SBOM accept job failed on 801f76b3e (2026-11-14)

Recorded 2026-11-14T05:48Z from a read-only reading of the run's logs.

## Problem
CI run 43560540641 on main commit 801f76b3e concluded failure. The only
failed job was "Accept Differential SBOMs" (132345249446), step "Verify
candidate identity, completeness, and checksums": "SBOM manifest failed
package, source, status, or required-package validation". The staging
deploy still went out at 05:25Z. No accepted baseline was saved for 801f76b3e.

## Evidence
- Build job 132343066415, 05:15:01Z: cache hit for restore-key
  sbom-bundle-v2-d4a27b2a2; 1 built, 8 rebound, 0 failed; every manifest
  row has previous_commit=d4a27b2a2.
- Accept job, 05:23:28Z: "Cache not found for input keys:
  sbom-bundle-v2-801f76b3..., sbom-bundle-v2-". The verifier takes
  previous_commit from the restored baseline, or "" when absent
  (.github/scripts/verify-sbom-candidate:24-27). Line 41,
  all(.previous_commit == $previous_commit), compared d4a27b2a2 with "".
- Repo cache usage is 10.72 GB against the 10 GB limit; no sbom-bundle-v2-*
  entry exists now. Nothing in .github deletes caches.
- deploy-staging needs [check, build-frontend, deploy-impact], not sbom, so
  the deploy is not gated on this job.
- Runs d4a27b2a2 (accepted), the docs-only d81cb98b4 and 5a4eb6ba2 (SBOM
  jobs skipped) and a56eba610 (skipped) succeeded. This is the only
  accept failure in the last 40 main runs.

## Belief
The baseline cache entry was evicted between the build job's restore (05:15Z)
and the accept job's restore (05:23Z) because the repo is over its cache
quota. The two jobs restore the baseline independently, so a mid-run
eviction makes them disagree. This is a race against an environmental
limit, not a defect in 801f76b3e or in the manifest. Confidence is high on
the failing rule and moderate on eviction as the reason the entry was gone.
The next non-docs run should self-heal (both jobs miss, previous_commit is
empty), at the cost of a full rebuild of all nine packages.

## Remaining error
- Eviction is inferred from usage and absence, not observed.
- Self-healing on the next run is predicted, not seen.
- The accepted baseline now lags and has no cache copy; it is held only in
  run artifacts.
- The race will recur while the cache sits at the quota. The fix is to have
  the verifier use the previous_commit the candidate recorded, or to restore
  the baseline once and pass it between the jobs. Neither is done.
