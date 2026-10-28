# A gate-file commit landed without its trailer and wedged main (2026-10-28)

Mutation class: green (record). Owner-authorized repair: force-push of main.

## What happened

FP0 landed lane/switch-registry (d48a5513d, 10:32 UTC) as tier 0 after
running its lint and selftest, but without the CI mirror. The commit adds a
step to `.github/workflows/ci.yml`, a gate file of
`scripts/check-guest-security-epoch` (rule 6): such a commit needs
`Security-Effect: gate change; <why>` and a Reviewed-By trailer ALWAYS, and
an epoch raise does not excuse it. CI run 40672878142 failed Vocabulary
Gates and v1-inventory.

## Why it wedged main

CI checks every commit from the deployed base (deploy-base-select:
staging's deployed commit, then 203be57db) to the head, and also runs the
base's copy of the script. A pushed commit cannot gain trailers, and no
later commit can cover a gate-file change, so every CI run on main fails
until staging deploys past it, and staging deploys only on a green CI.

## Repair (owner yes, 11:2x UTC)

Devin review 377 approved the ci.yml change (findings low/nit: a `ring:
"unknown"` entry skips the off-everywhere rule; an unresolvable `--base`
degrades silently; `${{ }}` in `run:` instead of `env:`). FP0 rebuilt main
from b51d2ee22: d48a5513d amended with the trailers (now a new SHA), the
eight S3/S4 commits re-applied unchanged. Tree identical to daeb323bd;
force-pushed with lease daeb323bd -> 8707ee4d7.

## What changes

- A lane is never landed on main without the CI mirror, tier 0 included:
  the mirror runs the gates CI runs against the deployed base.
- The gate has no recovery path for a pushed gate-file commit short of a
  history rewrite. A follow-up worth considering: an owner-signed exemption
  commit naming the SHA (reviewed like any gate change).
