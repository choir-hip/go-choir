# Theme rounds on b@c: two stuck, several installed with the work left open

Date: 2026-11-09. Status: open, not fixed. Source: a read-only trace review
(Sonnet) of the day's theme rounds on b@c. Statements are marked trace
(read from the trajectory, assignments and Texture documents), code, or
hypothesis.

## What happened

- Installed on b@c, each with a release id, an install id and b@c as
  publisher (trace, `/api/packages/installed`): oled-black, vectorheart,
  neo-vector-heart, metal-heart, utopian-scholastic, aero-heart,
  chrome-pastel, signal-orange, ink-sakura. Not installed: tide-pool and
  liquid-scholar.
- Publish is part of apply (accept, publish, install) and was never refused
  (trace). The two failed rounds stopped before it.
- Whether the installed themes show in the Library window was not checked.
  b@c serves a frontend from 2026-11-08
  ([stale frontend](stale-frontend-after-base-restore-2026-11-09.md)), so
  its Library window may not show them (hypothesis).

## Causes

A. **Freeze refused for late before screens** (Tide Pool twice, Ink Sakura
   once; trace for the refusals as Texture recorded them, code for the
   rule). `refuseLateBeforeShots` (`internal/capsule/screens.go:172`)
   refuses any before screen whose file time is later than the first source
   write, and it runs for a new package too, where no before and after pair
   is required (`requireScreens`). The driver's prompt asked for before
   screens on every round, so one ordering slip failed the freeze. The exact
   refusal text for Tide Pool was not retrieved.

B. **A run cancelled for quota has nothing to wake it** (Liquid Scholar;
   trace). At 13:56:48 UTC the assignment was cancelled with every model
   out of quota; the reset was seven minutes later; Texture wrote "waiting
   for provider quota reset" and nothing ran again. That no code path
   re-dispatches or closes the work at the reset is a hypothesis.

C. **Work parked on the owner never settles** (Tide Pool, Ink Sakura;
   trace). The last Texture turn waits on an owner decision with no live
   assignment and the work item stays open. Neo Vector Heart, Metal Heart
   and Utopian Scholastic end the same way after a successful install; the
   driver's notes attribute those to a missing apply record (a report, not
   a trace).

## Clustering

Six rounds end with an open work item and nothing that will close it (B and
C). That is one substrate: the closing path of a work item whose last turn
waits. A is separate, a mismatch between the prompt and the freeze rule.

## Fix directions (not built)

- A: the late-before check ignores files the change adds, or applies only
  where a before and after pair is required.
- B: a quota cancel either re-dispatches once after the reset time or
  closes the work with that cause.
- C: a trajectory whose last turn waits on the owner with no live
  assignment settles as blocked, visibly.

## Ids

Trajectories: Liquid Scholar c8d89e74-e8bd-5705-ac8f-aae352ef66b8, Tide
Pool 52695c9e-4335-54a0-b04d-efbaf061feb2, Chrome Pastel
9263779a-14b8-5e8d-8c7c-9ba78bc162a3, Signal Orange
e2d7106b-a8d9-5c99-be88-df79d701733b, Ink Sakura
6413668a-ee53-5f71-8f27-18455084f11b.
