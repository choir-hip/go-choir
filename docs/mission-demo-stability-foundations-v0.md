# Mission: Demo Stability Foundations v0

## Status

Problem checkpoint, 2026-05-29.

The next Choir-in-Choir campaign is tabled until the everyday product surface is more reliable during demos. The acceptance target is not a large new self-development loop; it is a smaller stability foundation: the visible desktop and VText surfaces must truthfully reflect durable background work across reloads, closed tabs, and signed-out visits.

## Observed Failures

- VText can show "Revising..." with too little visible feedback, leaving the owner unsure whether work is live, stalled, or merely a stale local flag.
- After reload, the background VText run may still be active while the visible revision indicator disappears or becomes too easy to miss.
- Chyron streaming is useful narrative context, but it is not sufficient as the primary evidence that a specific VText document has an active background mutation.
- A Minesweeper demo document reached v4 after multiple revise requests, but the owner could not easily tell whether verification was pending, blocked, or still progressing.
- Reloading the authenticated desktop can reopen the Email app even when the last visible work was another app. The suspected trigger is stale URL app intent such as `?app=email` being replayed on every load.
- The signed-out public desktop currently opens several preview apps. For a public first view, that feels like someone else's messy computer, not a blank or intentionally explained desktop.
- Product observation in Comet on `https://choir.news/` showed the live tab in a session-expired/signed-out state with the auth overlay over many open preview/user-like windows. This confirms the public-first-view issue and makes the session-sync problem concrete rather than theoretical.
- After passkey reauthentication in Comet as `yusefnathanson@me.com`, the desktop restored to a Trace window for "Make an app that is a minesweeper game". Trace showed the trajectory completed at 2026-05-29 16:34:22, with conductor and super completed and five VText runs. The Desk still showed 11 open windows, including multiple duplicate email-draft VText windows, which reinforces that stale open-window state needs canonical cleanup and cross-session convergence.
- Code inspection found session-local desktop placement authority: `GetDesktopStateForSession` preferred the current browser session's old placement row before the owner's latest placement, so phone/desktop sessions could diverge for the same user.

## Invariants

- Browser presence is observational only. Opening, closing, reloading, or hiding a page must not control whether the background work continues.
- Frontend state is only a projection cache or transient input buffer. It must not be the authority for desktop layout, active app, VText work status, verification status, or background run lifecycle.
- VText working state must be derived from durable backend state, especially the document-scoped pending mutation/run state, not just a transient component variable.
- The visible desktop after reload should prefer the last durable desktop state unless the URL contains a current, explicit, unconsumed route intent.
- All sessions for one owner should converge on the same active desktop state. Session IDs are provenance and driver-lease metadata, not separate authoritative desktops.
- Public signed-out desktop state should be intentionally sparse: blank desktop or one explanatory VText, not multiple owner-like work windows.
- Progress UI should make uncertainty legible. "Working", "pending verification", "stalled", "failed", and "complete but not adopted" should not collapse into the same quiet text.

## First Stability Slice

This pass should stay small:

1. Simplify the signed-out public desktop to a single explanatory VText preview.
2. Make stale `?app=email` URL app launch one-shot so it does not keep overriding restored desktop state on reload.
3. Strengthen VText pending feedback by making document-level pending state visible and animated after stream snapshots/reconnects.

Broader verification-state semantics, video QA capture, and campaign compiler promotion safety remain important, but they should follow after the basic visible/durable continuity loop is less ambiguous.

## Structural Debt

The rewritten frontend still uses Svelte stores and component variables in places that can behave like authority rather than projections. The durable target is stricter: desktop windows, active app, app context, VText work state, verification state, and run lifecycle should be backend facts with frontend stores rebuilt from snapshots and live events. Local component state is acceptable only for unsaved text input, pointer/drag gestures before commit, animations, and short-lived request plumbing.

## Acceptance

- A logged-out visit to `/` shows at most one public preview window.
- An authenticated reload with a stale Email URL intent does not repeatedly force Email above the restored active desktop state after the intent is consumed.
- A VText document with a pending agent mutation shows a clear visible working indicator after initial load or SSE reconnect.
- Backend VText stream snapshot behavior remains covered: snapshots include pending mutation state and clear stale terminal mutations before reporting pending.
