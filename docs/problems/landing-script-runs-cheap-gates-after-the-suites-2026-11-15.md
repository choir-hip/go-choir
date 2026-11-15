# The landing script runs the cheap gates after the suites

Date: 2026-11-15.

## Problem

`scripts/land` ran the `.github/scripts/*-test` self-tests last, after every
Go suite. A gate that takes seconds refused a train after 107 minutes.

## Evidence

The third run of the Jev and L1a train (started 06:34Z) passed the agentcore
race suite (3,628 s) and the store race suite (1,992 s), then failed at
08:20Z in `ci-impact-classify-test`: `scripts/jev-entry-baseline.txt` is read
by a CI gate and was not listed in `ci_deploy_script_path`. The fix was one
line.

## Belief

The contract says cheap checks run before every landing. The script had the
order backwards for this group only. Moving the loop ahead of the suites
makes the same failure cost under a minute.

## Remaining error

The frontend build still runs last; it is minutes, not seconds, and was left.
The earlier two failed runs of this train (a goal-file YAML gate, a 5 s test
wait) were caught at different points; this change would not have saved
those.
