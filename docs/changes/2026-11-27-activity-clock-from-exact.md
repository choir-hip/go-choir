---
date: 2026-11-27
title: Activity's clock is Exact's, not a native fact
class: orange
---
What: the ages on the Activity page ("2 hours ago") are measured from Exact's date: the contract asks `exactTime()` (`members-settings.contract`, resource `stime`) and passes `stime.epochAtZero + performanceNow()` after `with` to `activityPage` as its seventh argument. `now` is gone from the native `SettingsFacts` (`interface/settings.json`, regenerated `exact.json`) and from `AutoradioModule+Settings.swift`, which no longer reads `Date()`. Exact documents the date this way in `.exact-src/docs/recipes/dates-and-days.md` (line 3) and `contract-grammar.md` (`performanceNow()`, line 1324); `with` is in `contract-grammar.md` (the resource section, "A resource's optional `with`"). Left in place on purpose: the app's version (`appVersion`) is still the native `settingsFacts` field, and `barFacts.at` and `radioFacts.at` are untouched.

Why: shared code asked Swift for the clock, which Exact already gives it. The app's version was not moved because the move-b lane's way (the build writes `EXACT_VERSION` into `build-info.gen.ts`) reads the caller's environment, where `exact-env.sh` sets `EXACT_VERSION` only for some builds; the bundle's own version for a development build is Exact's default, so About could change from that value to "dev" and only a device build can say.

Proof: `exact-facts.test.ts` H1 to H3 and `activity.test.ts` (updated for the new argument); whole `bun test`, `bun exact.mjs contract build app.contract` and `bun exact.mjs test web` as in the landing report (`voices.test.contract` fails the same two tests on origin/main). Not proved on a device: that the ages on the Activity page read right on the phone, and the first ask, before `exactTime` answers, passes a date near zero (as the native clock's 0 did before its first answer).

Replaced: `now` in the native `settingsFacts` answer and in `Facts` of `activity.ts`.

Undo: revert the commit.
