# Parallax: Diagnose the Email App Freeze

## Status

Open. Not yet started.

## Mission conjecture

If we reproduce the Email app freeze and capture the exact error state (browser console, runtime logs, app state), then we can identify whether the freeze is caused by the Email app's state machine, an unhandled promise, an authentication renewal loop, or a desktop suspension bug, and produce a fix plan.

## Deeper goal

The Email app freeze is a symptom of the app maintaining its own state instead of being a view over the object graph. The deeper goal is to understand the freeze precisely so that the eventual rewrite to a graph view is scoped correctly. The diagnosis also tests whether the current desktop/app-state model is salvageable in the short term.

## Witness / spec

Deliver a diagnosis document with:

- Reproduction steps that trigger the freeze.
- Browser console output at the moment of freeze.
- Network requests made before the freeze (focus on `/api/email/messages` and `/api/email/aliases`).
- A stack trace or error message if any is thrown.
- Identification of which function or state variable prevents the app from recovering.
- A hypothesis about the root cause and a proposed fix path.

Investigate:
- `frontend/src/lib/EmailApp.svelte` `loadMessages`, `loadAliases`, `scheduleAppStateEmit`, `emitAppState`.
- `frontend/src/lib/auth.js` `fetchWithRenewal` and `AuthRequiredError`.
- `frontend/src/lib/stores/desktop.js` `suspendBackgroundHeavyWindows`, `suspendWindowBody`, `isHeavyAppId`.
- `frontend/src/lib/Desktop.svelte` `handleWindowAppContextChange`, `openApp`.

## Invariants / qualities / domain ramp

- Do not change code in this worktree unless a one-line diagnostic fix is obvious.
- Do not weaken the user experience to avoid the freeze.
- Do not claim local proof if the freeze only reproduces on staging.
- Use browser-based proof if available; otherwise capture logs and code analysis.

## Authority / bounds

- Yellow mutation class: tests, diagnosis, and prompt framing.
- No platform behavior change unless the fix is trivial and safe.
- Branch: `diagnose/email-freeze`.
- Worktree: `email-diagnose`.

## Bridge conjecture + sub-conjectures

- Main conjecture: the Email app freeze is an attention failure where the app state machine gets stuck before the graph view migration can happen.
- Sub-conjecture 1: the freeze is caused by `fetchWithRenewal` entering a renewal loop that never resolves.
- Sub-conjecture 2: the freeze is caused by `scheduleAppStateEmit` being called in a tight loop.
- Sub-conjecture 3: the freeze is caused by the desktop suspension logic applying to the Email app even though it is not a heavy app.

## Ledger / move log

- Move 0: Read the relevant frontend files.
- Move 1: Open the Email app in a browser and reproduce the freeze.
- Move 2: Capture console logs and network requests.
- Move 3: Add targeted logging or use DevTools to identify the stuck state.
- Move 4: Document the root cause and a fix plan.
- Move 5: Commit the diagnosis to the branch.

## Version / lineage

- Predecessor: `@/Users/wiz/go-choir/docs/object-graph-synthesis-2026-06-23.md` maps the Email freeze to the graph-level diagnosis.
- Successor link: the fix will be implemented in the mail-object graph migration.

## Learning state

- Retained: the exact failure mode of the Email app freeze.
- Promoted outward: a fix plan for the mail-object graph migration.

## Settlement

Done when the diagnosis document is committed and the root cause is identified with evidence.
