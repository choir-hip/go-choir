# Self-development visual capture misses the app a candidate changes

Date: 2026-11-02. Mutation class of this record: green (a doc).

## Problem

The self-development verify and judge never see the app a candidate changes.
`choir-capture` (internal/visualcapture) renders only the routes the engineer
names, at one 1280x800 viewport, against a signed-out backend: `/auth/session`
answers signed out and every `/api/` route but the Autopaper front list answers
503. A package app (`frontend/src/packages/<pkg>/`) opens only in a signed-in
desktop with a restored window, a theme preference and a document to open, so
no route the tool can serve shows it. Engineers capture `/` and `/read.html`,
and the judge sees the front page.

## Evidence (2026-11-02, the two-arm Slides-next runs on b@c and d@e)

- The judge passed selfdev-00b0b460 (Slides-next), whose evidence was `/` and
  `/read.html` only. FP0's real-shell render of the same patch found 4 visible
  app defects (docs/evidence/selfdev-apps-design-judgments-2026-11.md; the
  "Product gap" section of
  docs/evidence/selfdev-compare-claude-arm-slides-next-2026-11-02.md). Frozen
  candidates 2949047c, ceb607ca and 00b0b460 all listed exactly those routes.
- selfdev-09e2815b failed only because its engineer ran out of budget
  re-capturing the app in a hand-built harness (vite dev over a mocked backend,
  Chromium jobs): about 38 of its 47 minutes, with 7 to 12 minute blocking
  polls. Its captures stayed in the capsule's /tmp; `choir self-dev visual`
  returns 404 "the candidate carries no visual evidence".
- On d@e (no capture grant at the time), the engineer spent 7 of 18 minutes
  trying to build a Chromium (docs/evidence/selfdev-compare-deepseek-arm-slides-next-2026-11-02.md H5).
- The design bar (docs/design/app-design-bar-2026-10-26.md check 1, and the
  acceptance in docs/definitions/choir-selfdev-apps-side-by-side-2026-11-02.md)
  wants three presets at two widths (a 1100x750 window and 390px), each state,
  and the documented harness mocks exactly: a signed-in `/auth/session`,
  `/api/shell/bootstrap`, `/api/desktop/state` with one restored window whose
  `app_context.sourceUrl` names a fixture, `/api/preferences/theme` and
  `/api/media/recents`.

## Belief state

The renderer is fine; the scene is missing. Each engineer rebuilds the bar's
harness by hand, at most of its budget, and the result never reaches the
frozen evidence. The judge then reasons from the engineer's report.

## Remaining error

- Whether a fixed `--virtual-time-budget` screenshot (no scripting) renders
  a package window in the shell without the live channel (`/api/ws`).
- True fullscreen (Present) needs a user gesture; a screenshot run cannot
  click.
- Which packages a candidate touches is known to Freeze (the capsule diff),
  not to the capture job inside the capsule.

## Direction

The capture tool renders package scenes itself: for each named package, its
window in the real shell from the candidate's built frontend, at each preset
and both widths, with a fixture document for each kind the manifest opens.
It is bounded in captures and time, and a failed scene is a note in the
evidence, not a failed run. Freeze and the visual-evidence listing name any
package the patch touches that the evidence does not show, and the judge
prompt reads the package captures against the bar.
