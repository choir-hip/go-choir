# Theme rounds on b@c: two stuck, several installed with the work left open

Date: 2026-11-09. Status: open; B is mitigated by 36534be53, A and C are not fixed (see Investigation 2026-11-09 evening). Source: a read-only trace review
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


## Investigation 2026-11-09 evening

Read-only: code at origin/main 0ebab72a7, and b@c's trajectories through the
`choir` CLI (`trajectories`, `trajectory <id>`), plus one
`/api/runtime/observability` read of each of b@c and d@e at about 01:30Z.
Cluster assessment:
[unsettled-obligations-cluster-2026-11-09.md](unsettled-obligations-cluster-2026-11-09.md).

### Current truth

Still visible. b@c has 50 trajectories: 43 settled, 7 live (6 task, 1
document). Six task trajectories are live with one open work item each and no
live assignment: Liquid Scholar c8d89e74, Tide Pool 52695c9e, Ink Sakura
6413668a, and three earlier ones (4b9398fc Neo Vector Heart, 5342f74b Metal
Heart, f8778238 Utopian Scholastic). Chrome Pastel 9263779a settled normally
(`work_settled`, `trajectory_settled` at 20:49Z), so the settle path works
when the last Texture turn says Done rather than Wait.

### A: late before screens (confirmed by trace and code)

Tide Pool, event 19 (22:10:55Z): "Second bounded freeze refused on before-shot
order; nothing frozen or installed; owner decides on a fresh engineering
round". The code is as the record says: `stageScreens`
(`internal/capsule/screens_linux.go:42`) runs `refuseLateBeforeShots` whenever
the bundle is not plan-based, and `sourceChangeTimes` counts every created
non-exempt file as a source edit. For a new package every file is created, so
any before shot taken after the first file write is "late", yet
`requireScreens` (`screens_required.go:155`) asks for a before and after pair
only when the change modifies or deletes a file, or adds to a package root
that exists on the base (`existing`). The two rules disagree: the late check
fires on changes that need no before shot. Root cause confirmed. Symptom, in
a leaf rule; not substrate.

Smallest correct fix: run the late-before check only when the same predicate
that makes a pair required holds (extract the `existing` computation from
`requireScreens` into `needsBeforeAfterPair(changes, baseHasDir)` and pass its
result into `stageScreens` beside `planned`). Two or three files
(`screens_required.go`, `screens_linux.go`, the `executor.go:~2374` call site)
and a table test. Mutation class red (it loosens a Freeze refusal), tier 1,
one-act rollback. About 2 hours for a Sonnet builder. No restart-rule
implication. Not fixed on main.

### B: quota cancel (confirmed by trace; two defects)

Liquid Scholar, event 23 (13:56:48Z), assignment 41f8710b: "tool loop
iteration 268: every model is out of quota after 8 retry rounds over 6m59s
... resets at 14:03:46Z (estimated)". Events 24-25 (13:56:49, 13:57:17):
Texture committed "waiting: Waiting for provider quota reset to resume the
bounded Liquid Scholar fix build". Nothing after that except the 00:04Z
boot's `run_terminalized` events.

1. The cancel itself was the paper-quota defect: an incident bounded by retry
   rounds, not time
   ([paper-quota-incidents-2026-11-09.md](paper-quota-incidents-2026-11-09.md)).
   Mitigated by 36534be53 (2026-11-09: bounded by time, waits for the
   earliest reset, does not spend the progress deadline). b@c and d@e both
   run 0ebab72a7, which contains it. Not re-observed since; a quota that
   outlasts the activation hard wall can still cancel.
2. The stranding: the cancel report reached Texture (it committed two turns),
   Texture wrote a free-text Wait, and no code owns that Wait. No timer, no
   re-dispatch, no close. The record's hypothesis ("no code path re-dispatches
   or closes the work at the reset") is confirmed on the Texture side:
   `turnNamesNextActor` (`textureowner/texture_next_actor.go:52`) accepts any
   non-empty `Wait`, and the unattended backstop (`closeUnattendedRequest`)
   runs only after the runtime's Idle turn. Substrate, the same cause as C
   (see the cluster doc).

### C: parked on the owner or on a record that never comes (confirmed; two shapes)

- Owner decision: Ink Sakura (event 26, 23:09:47Z) "Engineering final: Ink
  Sakura built and installed ... waiting: Waiting on the owner: confirm the
  ink-sakura.theme id and the install order"; Tide Pool (event 19) waits on
  "the owner's decision whether to authorise a fresh engineering round";
  Metal Heart (event 13, 09:36Z) "Waiting on the owner's decision boundary to
  publish to Library and install". Each has a completed or failed assignment,
  no live one, and an open work item.
- A record that never comes: Neo Vector Heart (event 15, 08:13Z) and Utopian
  Scholastic (event 13, 10:41Z) wait for "the separate record confirming ...
  application". Both packages are installed (trace, this record), so the
  apply happened and the confirming wake did not reach Texture, or reached it
  and was not acted on. Hypothesis, not confirmed. What would confirm it: the
  wake rows addressed to `texture:<doc>` between the apply and 00:04Z, which
  the CLI does not expose.
- At 00:04Z a boot terminalized the parked runs (events 20-22, 27-32; the
  trajectory's `activation` shows run state `cancelled`). That boot closed the
  runs and left the work items open: `closePreBootRunsAfterCrash`
  (`runtime.go:3030`) closes runs, `closeOpenRoundsAfterCrash` closes rounds,
  and nothing closes a work item whose agent has no live run. The restart rule
  says every open obligation, work items included, closes with the visible
  fate "interrupted by a restart"; for these work items that does not hold.

Smallest correct fix (shared with B): cluster fix 1. A Texture turn that
commits a Wait while nothing serves the request refuses the work item with the
typed `no_next_actor` wait record and notifies management (the existing
`closeUnattendedRequest` path), and the crash-boot closure does the same for
open Texture work with no live run. For the owner-decision shape this is the
right end state: the work item is visibly closed "waiting on the owner", the
trajectory can settle, and the owner's next revise opens new work. Class red
(lifecycle refusal on Texture work); the Wait leg is tier 1, the boot leg
tier 2 (boot path). Not fixed on main.
