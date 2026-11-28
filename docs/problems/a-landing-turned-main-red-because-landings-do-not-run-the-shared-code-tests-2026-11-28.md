# A landing turned main red because landings do not run the phone app's shared-code tests

Standing: governing | kind: happened | settled-by: CI run 46105470562 on f0fc8c7f4 (red) and the landing of 992bb1881 (green, staging serves it), 2026-11-28 | note: the landing path runs the CI-shaped run (no Exact); the run with Exact is open

## Problem

The landing of `f0fc8c7f4` added a test, `mobile/AutoradioLab/symbol-platform.test.ts`,
that runs the real contract build. The build needs the Exact checkout. CI has
none: it stubs one package and runs `bun test`. The test failed there and main
was red for about 25 minutes, until `992bb1881`.

The local landing checks did not see it, because `scripts/land` does not run
`bun test` in `mobile/AutoradioLab` for a change under `mobile/`. The builder
ran the suite in a lane, where Exact is present, and it passed.

## Evidence

- CI run 46105470562, job "Autoradio Shared-Code Tests": one test failed, the
  build exiting 1 in 7 ms.
- `grep -n "bun test" scripts/land` finds nothing.
- Staging kept serving `ddb2d0f5c` until the fix landed.

## Belief

Two things are wrong, and only the first is fixed.

1. Fixed: the test now skips, and says so, where there is no Exact checkout.
2. Open: a shared-code test that fails only where Exact is absent, or only
   where it is present, reaches main unseen. The landing path should run the
   suite as CI runs it (no Exact) and as a lane runs it (with Exact) when a
   landing touches `mobile/AutoradioLab`.

## Remaining error

- Repaired for the CI shape: `scripts/check-shared-code-tests` runs in `scripts/land` after the gates when a path the tests read changes (`docs/changes/2026-11-28-landings-run-the-shared-code-tests.md`).
- Seen on the first real run: the CI-shaped suite takes about six minutes here (365 s of tests), not the 20 s `ci.yml` says. Whether a runner is faster is not checked.
- Open: the run with Exact present, as a lane runs it. It needs the build tree's Exact checkout and `scripts/sync-exact-package.sh` in the copy, and would add those minutes to a landing or need the Mac build lock to run beside the suites. A test that fails only where Exact is present still reaches main unseen.
- The symbol test no longer runs in CI, so CI does not guard against a new blank icon. Only a `bun test` in a lane or the build tree does. The open run above is what would catch it.
