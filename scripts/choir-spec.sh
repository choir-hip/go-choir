#!/bin/sh
# choir-spec.sh: run frontend Playwright specs in a capsule job.
#
# A capsule's /workspace/platform/frontend has no node_modules, so a spec's
# `import ... from '@playwright/test'` fails with ERR_MODULE_NOT_FOUND (problem
# 18). The locked tree is $CHOIR_FRONTEND_DEPS/node_modules (read-only, in the
# Nix store). This links it for the run and removes the link on exit, so the
# workspace is the same when the job ends (a job that leaves it changed is
# refused as evidence, and a leftover link freezes into the candidate).
#
#   sh /workspace/platform/scripts/choir-spec.sh tests/theme-contract.spec.js --grep "package files-next has"
#
# Scope: specs that read files (theme-contract and the like). A spec that
# launches a browser needs the store Chromium ($CHOIR_BROWSER), which the
# Playwright config does not wire; take screenshots with choir-shot.mjs.
#
# Run it as a capsule job (choir.Start), not a choir.Exec.
set -u
root=${CHOIR_PLATFORM_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}
fe=$root/frontend
deps=${CHOIR_FRONTEND_DEPS:-}
[ -n "$deps" ] && [ -d "$deps/node_modules" ] || { echo "choir-spec: CHOIR_FRONTEND_DEPS is unset or has no node_modules (a capsule job has it)" >&2; exit 2; }
nm=$fe/node_modules
if [ -L "$nm" ]; then
  [ "$(readlink "$nm")" = "$deps/node_modules" ] || { echo "choir-spec: $nm links elsewhere; remove it and run again" >&2; exit 2; }
  rm -f "$nm" # a link a killed earlier run left behind
elif [ -e "$nm" ]; then
  echo "choir-spec: $nm exists and is not a link; remove it (rm -rf) and run again, it hides the locked dependencies" >&2
  exit 2
fi
ln -s "$deps/node_modules" "$nm" || exit 2
cleanup() { rm -f "$nm"; }
trap cleanup EXIT
trap 'exit 143' INT TERM
cd "$fe" || exit 2
# Playwright writes test-results/ (even .last-run.json on a pass) under the
# frontend; --output moves it to /tmp so the workspace digest does not change.
"$nm/.bin/playwright" test --output "${TMPDIR:-/tmp}/choir-spec-out" "$@"
