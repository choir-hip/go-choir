# Self-development visual capture of "/" shows the landing's error state (2026-11-02)

Class: orange (capture server fixture). Rollback: revert the commit.

## Problem

Operation selfdev-e9adea22747bb9601fc2abf6808f5f47 (b@c, base-rebase) was
failed at 2026-11-02T17:52Z by the independent judge: the required visual
capture of `/` "found an error message ('Could not load stories. Try again
shortly.') and no document content". Captures were `/` and `/read.html`; the
`/` PNG is the signed-out desktop with an Autopaper window in its error state.

## Cause (confirmed from code, git history and the PNG)

- The signed-out home opens the Autopaper front window
  (frontend/src/lib/Desktop.svelte `seedPublicPreviewWindows`, since
  95a5d9428, 2026-10-13).
- It fetches `/api/platform/publications/front`
  (frontend/src/lib/autopaper.ts `fetchFrontPage`); on a non-2xx it renders the
  error (frontend/src/lib/AutopaperApp.svelte:272).
- The capture server (internal/visualcapture/capture.go `Server`) serves the
  candidate's static assets and answers every `/api/` route 503 on purpose,
  so a capture of `/` can only ever show the error state. The default route is
  `/` (capture.go `Normalize`); the engineer may add routes.
- The judge is a model reading the screenshots, not a rule. The error state was
  already in the 2026-10-25 captures
  (docs/evidence/selfdev-trace-reviews-2026-10-25.md section 4) and those
  rounds passed: the judge's tolerance varied, the capture never could show
  content. So this blocks every candidate that captures `/`, at the judge's
  discretion.

## Fix

The capture server answers the one public GET route the landing needs with a
fixed, labelled sample front list (five "Sample ..." stories), and a section
page with an empty list. Every other `/api/` route stays 503, `/auth/` is
unchanged, only GET/HEAD are served. The same pattern as the existing
`/auth/session` signed-out answer. The judge is untouched: an empty or error
capture still fails. Rejected: capture routes from the diff (still shows the
shell's other public fetches, and a candidate that touches no app has no
route); an environment-error exemption in the judge (weakens the check).

Test: internal/visualcapture TestServerAnswersTheFrontListWithAFixtureAndNothingElseUnderAPI
(written first, failed with 503).

Residual: other public GET routes a future signed-out surface fetches need a
fixture too; check the capture of `/` after a landing change.
