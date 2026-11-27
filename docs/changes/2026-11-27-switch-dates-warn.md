---
date: 2026-11-27
title: a passed date in the switch registry warns and no longer fails every run
class: yellow
---
What: `scripts/check-switch-registry` no longer fails because a date passed. An entry's `expires` or `grandfathered_off_until` date warns from 14 days ahead. The lint fails only when a change adds a new switch while an entry is due or overdue with no `retire: delete` verdict, counted from the day before the date. `--overdue` lists exactly those entries by owner; `--strict-expiry` keeps the old failure for whoever wants it. `scripts/gates` gives the lint the same base CI gives it.

Why: on 2026-11-26 every CI run on main went red on a date alone (five `calibration_e1_*` entries), with no change to any switch. A date that fails the build stops all work to demand one decision.

Proof: `python3 -I scripts/selftest-switch-registry` -> `0 failure(s)`; the lint on the real registry with and without `--base origin/main` -> `ok (65 switches, 51 warning(s))`. Reviewed twice (seat 1, six findings; seat 2, seven findings, one a real off-by-one on the due day); all fixed. A third review timed out and gave no verdict. Not proved: a real change that adds a switch being refused in CI.

Replaced: failing on the calendar.

Undo: revert this commit; the lint then fails again the next time a date passes.
