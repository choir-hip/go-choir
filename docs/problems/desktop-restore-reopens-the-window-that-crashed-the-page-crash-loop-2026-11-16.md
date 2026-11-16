# The desktop reopens the window that crashed the page: a crash loop

Recorded 2026-11-16, from the owner's report (phone, Safari, choir.news, their
own computer) and a read of the code. Nothing was done on the owner's computer.

## Problem

Opening a PDF in Reader on the phone killed the page. On reload the desktop
restored its saved windows, mounted Reader with the same document, and the page
died again. The loop ended only because the owner closed the window in the
moment between restore and crash. No user should need to win a race to get
their computer back.

## Evidence

- Owner's screenshot: Reader window, title a report PDF, page counter `1 / ?`,
  status `Opening <title>…`. The counter is set right after `getDocument`
  resolves (`frontend/src/packages/reader/App.svelte`, `loadPdfDocument`), so
  the page died while fetching the file, loading pdfjs and its worker, or
  parsing, and before the first page was drawn.
- Window state is saved on the server (`PUT /api/desktop/state`) and restored
  at load (`Desktop.svelte`, `loadDesktopState`). A restored window's body is
  mounted at once unless `shouldSuspendRestoredWindow` holds it back, and that
  only happens for a heavy app that is not the active window, when there are
  many windows. The active window is never held back. The window that crashed
  the page is almost always the active one.
- Nothing records that the last load ended in a crash. The restore path cannot
  tell a clean reload from a page death.

## Belief

Two separate defects.

1. The category: restore resumes work after a crash. The operating contract
   already rules on this for computers ("a crash restart never resumes work";
   a planned restart resumes once, with a marker). The web desktop does not
   follow it. The fix is the same rule at this layer: a window body is mounted
   again only when the last load is known to have gone well, and a window that
   was opening when the page died comes back held, with its reason, for the
   user to reopen or close.
2. The instance: something in Reader's PDF open path kills Safari on the
   phone. Cause not known. The report PDFs are small (tens of kilobytes), so
   file size is unlikely.

## Remaining error

- The cause of the page death is not known; it needs a reproduction on iOS
  Safari. A crash guard makes it survivable, not fixed.
- Other things that restore work at load (an app's own "reopen my last
  document", media that resumes) are the same category and are not yet listed.

## Status, 2026-11-16 16:05Z

The category fix landed as a43cf5013 (CI run 43898136595) and passed its
deployed proof on staging in Chromium and WebKit
(`frontend/tests/crash-guard-deployed.spec.js`, calibration claim C110). The
review history and the change of approach are in
[the structural assessment](desktop-crash-guard-review-did-not-converge-in-three-rounds-structural-assessment-2026-11-16.md).

Still open:

- The cause of the page death in Reader's PDF path. The guard now records the
  step a window was on; the next occurrence will name it.
- A signed-out visitor's desktop is not guarded (reproduced by a reviewer on a
  public story route).
- A crash later than the quiet period after an app's last foreground step is
  not held.
- A computer serves its own frontend. After a deploy, the drill computer took
  the new build 12 minutes later; a fix is not live for a user until their
  computer has taken the release. Whether the owner's computer has it was not
  established (an operator read of its page returned the platform's build).
