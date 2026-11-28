---
date: 2026-11-28
title: A landing runs the phone app's shared-code tests as CI does
class: yellow
---

What: `scripts/check-shared-code-tests` is a landing step. When the train changes a path the tests read (`mobile/AutoradioLab/`, `mobile/exact-package/`, `mobile/scripts/`, `mobile/fixtures/`, `layouts/`, `themes/`, `mobile/docs/typography.md`, `internal/autoradioqueue/queue.go`, `internal/texturedoc/golden/`, `scripts/android-deck-tests/`), it unpacks HEAD with `git archive` into a scratch directory, writes CI's stub for `@exact/passkeys` (no Exact checkout), runs `bun install --frozen-lockfile` and `bun test` with the pinned Bun (`$HOME/.bun-1.4.2/bin/bun`, the version `ci.yml` installs), and removes the scratch directory. 900 s limit. A failing test fails the landing and is named at the end of the output. A missing pinned Bun fails with how to install it; no other Bun is used. `scripts/land` runs it after the app compile and before the suites and the push; `scripts/land-queue` stages it with `land`; `scripts/gates` runs its selftest (fake tree, fake runner). Any other train prints one line and passes. There is no way to skip it.

Why: `f0fc8c7f4` added a test that needs the Exact checkout. It passed in a lane and failed in CI, and main was red for about 25 minutes ([the record](../problems/a-landing-turned-main-red-because-landings-do-not-run-the-shared-code-tests-2026-11-28.md)).

Proof: the selftest (20 checks). One real run against a train that touches the app (`--changed-from ddb2d0f5c~1`) took 6 min 5 s on this Mac and passed: 1856 pass, 2 skip, 0 fail. Its first run failed one test, because the scratch copy lacked `scripts/android-deck-tests/`, which `android-package.test.ts` checks for; that path is now unpacked and listed. A landing that touches the app now takes about six minutes longer, in sequence with the other steps. Not proved on a real landing.

Replaced: nothing. The run with Exact present is not built; it is open in the record.

Undo: revert this commit.
