# The deploy's frontend commit check misses the commit when Vite splits build-info into its own chunk

Found 2026-10-26 04:44 UTC, CI run 40268891859 attempt 2 (commit b4b4e551b),
job "Deploy to Staging (Node B)" 121401819841. Status: documented; fix on lane
deploy-asset-graph.

## What happened

Every service activated at b4b4e551b (activation receipts for auth, proxy,
vmctl, gateway, corpusd and maild), and all four active computers were
refreshed and verified serving it. The next step, "Smoke test: public frontend
asset graph", failed: "Selected frontend asset .../app-Z6GTqhz4.js does not
contain workflow commit b4b4e551b...". The deploy recorded incomplete evidence
(/var/lib/go-choir/deploy-failures/40268891859-2.json), so the platform receipt
still names a03d5266a while the binaries and the frontend are b4b4e551b.

## Cause

The commit is compiled into frontend/src/lib/build-info.js. The Reader
promotion (34f361973) gave the signed-out preview page read.html an entry that
shares modules with the app, so Vite moved build-info into its own chunk
(build-info-B1N3zZMW.js, which carries the commit); the app entry imports it
("./build-info-B1N3zZMW.js"). The check in .github/scripts/node-b-ci-deploy
greps only the entry file.

## Fix

Look for the commit in the entry and in the chunks the entry imports directly.
Dry run against the deployed assets: b4b4e551b is found in build-info-*.js; a
wrong commit is refused. Class: red (deploy procedure), tier 1. Rollback:
revert.
