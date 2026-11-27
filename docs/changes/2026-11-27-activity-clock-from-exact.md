---
date: 2026-11-27
title: Activity's clock is Exact's, not a native fact
class: orange
---
What: the ages on the Activity page ("2 hours ago") are measured from Exact's date: the contract asks `exactTime()` (`members-settings.contract`, resource `stime`) and passes `stime.epochAtZero + performanceNow()` after `with` to `activityPage` as its eighth argument, after `current(stime)` as the seventh (a call argument, before `with`, so the rows are asked again when the date answers; Exact says a change to either list asks again, `contract-grammar.md` line 627): until Exact's date has answered no age is shown (`now` is `null` inside `activityInfo`, never a near-zero number). `now` is gone from the native `SettingsFacts` (`interface/settings.json`, regenerated `exact.json`) and from `AutoradioModule+Settings.swift`, which no longer reads `Date()`. Exact documents the date this way in `.exact-src/docs/recipes/dates-and-days.md` (line 3) and `contract-grammar.md` (`performanceNow()`, line 1324); `with` is in `contract-grammar.md` (the resource section, "A resource's optional `with`"). Left in place on purpose: the app's version (`appVersion`) is still the native `settingsFacts` field, and `barFacts.at` and `radioFacts.at` are untouched.

Why: shared code asked Swift for the clock, which Exact already gives it. The app's version was not moved because the move-b lane's way (the build writes `EXACT_VERSION` into `build-info.gen.ts`) reads the caller's environment, where `exact-env.sh` sets `EXACT_VERSION` only for some builds; the bundle's own version for a development build is Exact's default, so About could change from that value to "dev" and only a device build can say.

Proof: `exact-facts.test.ts` H1 to H3 (H1b and H1c: unknown date shows no age, signed in; `activityPage` forwards `known` and `now`) and `activity.test.ts` (updated for the new argument); whole `bun test`, `bun exact.mjs contract build app.contract` and `bun exact.mjs test web` as in the landing report (`voices.test.contract` fails the same two tests on origin/main). Shown: no age while the date is unknown and the ages once it is known, in `bun test`, and that the contract passes the flag and the date. Not shown: that the page re-asks and shows ages when `exactTime` answers, because the web harness has no signed-in changelog to ask. Not proved on a device: that the ages on the Activity page read right on the phone, and that the first ask, before `exactTime` answers, shows no ages.

Replaced: `now` in the native `settingsFacts` answer and in `Facts` of `activity.ts`.

Undo: revert the commit.
