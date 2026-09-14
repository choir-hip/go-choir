#!/usr/bin/env bash
# Test hang guard: fail on unbounded receives of fixture lifecycle channels.
#
# A bare statement-level receive like `<-provider.started` hangs the whole test
# binary until the 10-minute go-test panic when the channel's closer never runs
# — e.g. Execute skipped because the run terminalized first (observed: run
# 34641698611, TestActivationBudgetProgressDeadlineTerminalizesAndReleases).
#
# Bounded forms are fine: `case <-ch:` inside a select with a timeout, or the
# shared waitProviderChan helper. A receive that is intentionally unbounded
# (a fixture's own release wait, where the test itself holds the closer) may
# carry a `// hang-guard: <reason>` marker on the same line.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

# Receives of fixture lifecycle channels in test files.
# Any receive of a fixture lifecycle channel in a test file, unless it sits in
# a select case (bounded by the select's timeout arm) or carries an explicit
# '// hang-guard: <reason>' marker for an intentional fixture-internal wait.
hits="$(grep -rEn '<-[a-zA-Z_][a-zA-Z0-9_]*\.(started|finished|release|done|completed)\b' \
  --include='*_test.go' internal cmd 2>/dev/null | grep -vE 'case[[:space:]]+<-|hang-guard:' || true)"
if [[ -n "$hits" ]]; then
  echo "test-hang-guard FAIL: unbounded fixture-channel receives in tests:" >&2
  echo "$hits" >&2
  echo >&2
  echo "Bound the wait (select + time.After, or waitProviderChan) or mark an" >&2
  echo "intentional fixture-internal wait with '// hang-guard: <reason>'." >&2
  exit 1
fi

echo "test-hang-guard: ok"
