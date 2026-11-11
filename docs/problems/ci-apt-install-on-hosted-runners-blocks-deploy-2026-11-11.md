# CI jobs that install tools with apt fail when the runner cannot reach the Ubuntu mirrors, and that blocks the deploy

Date: 2026-11-11. Status: open, cause outside the repo, no fix chosen.

## Problem

Two CI jobs install a tool at run time on a GitHub-hosted runner:
Heresy Detector installs ripgrep with `apt-get install`, and Package
Modularity installs Playwright's browsers with their system packages. When
the runner cannot resolve `security.ubuntu.com`, `archive.ubuntu.com` and
`azure.archive.ubuntu.com`, both jobs fail before they read a line of our
code. "Go Vet + Test + Build" then fails as the aggregate, and "Deploy to
Staging" is skipped.

## Evidence

CI run 43033568565 on main at 42d54ab43 (the web default themes), started
13:55Z: five attempts between 13:55Z and 14:46Z, each failing in the same two
jobs with `Temporary failure resolving 'security.ubuntu.com'` and
`E: Failed to fetch ...`. Every other job passed, among them Build Frontend
and the Go tests. githubstatus.com reported no incident at 14:55Z. The
docs-only run before it (43033191350, 13:53Z) passed; it runs neither job.

## Belief

The failure is the runner's network or the Ubuntu mirrors, not the change:
the failing step is the tool install and the error is a DNS failure. Confidence
high for "not our code", low for where the outage sits.

## Cost

An owner-requested visible fix (the default theme) and the Autoradio desk
slice (6f0c773ed) sat on main undeployed for over an hour.

## Remaining error and options

Not yet known how often this happens; this is the first record. Options, none
taken: have the heresy job use the runner's own grep or a pinned ripgrep
binary fetched from our own release cache; cache Playwright's browsers and
system packages between runs; move both jobs to the Node B runner, which has
the tools from the Nix shell. Each touches `.github/workflows/ci.yml`, a gate
file, so it needs the gate-change trailers and a review.
