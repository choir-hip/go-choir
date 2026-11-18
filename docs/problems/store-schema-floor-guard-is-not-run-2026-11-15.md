# The store schema floor guard is not run by anything

Recorded 2026-11-15T22:50Z, found by the DeepSeek seat in the third review
round of the inline-body reader and confirmed by grep.

## Problem

`scripts/check-store-schema-floor.sh` refuses a push that lowers
`storeschema.Version` unless the head commit carries a
`Store-Floor-Override` trailer. Nothing runs it: no line in `.github/`,
`scripts/gates` or `scripts/land` names it. A commit that lowers the store
epoch would land and deploy with no refusal.

## Evidence

`grep -rn check-store-schema-floor .github scripts/gates scripts/land`
returns nothing (2026-11-15, origin/main at 58abd4024). The reviewer also
found a CI step for it written as a patch under `docs/evidence/` and never
applied.

## Belief

The guard was written for a deploy rehearsal and the step that would run it
in CI was left as a patch. The lead's own commit text relied on it twice
this day as if it ran.

## Remaining error

Not checked: whether the deploy program runs an equivalent check of its
own. The fix (one line in `scripts/gates`) is yellow and not yet made.

## Status (2026-11-18)

Fixed. `scripts/gates` runs `scripts/check-store-schema-floor.sh --base
origin/main --head HEAD` and its selftest, and CI's Vocabulary Gates job has a
step "Check the store schema floor". `scripts/check-gates-cover-ci` finds any
executable `scripts/check-*` that nothing runs, so this class cannot sit
unseen again. Not checked: whether the deploy program runs an equivalent.
