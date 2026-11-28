# A landing turned main red because landings do not run the phone app's shared-code tests

Standing: governing | kind: happened | settled-by: CI run 46105470562 on f0fc8c7f4 (red) and the landing of 992bb1881 (green, staging serves it), 2026-11-28 | note: the landing path still does not run these tests

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

- The symbol test no longer runs in CI, so CI does not guard against a new
  blank icon. Only a `bun test` in a lane or the build tree does.
- The suite takes about six minutes locally with Exact. Whether both runs
  belong in every mobile landing, or only the CI-shaped one, is not decided.
