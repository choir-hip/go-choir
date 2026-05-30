# Mission: Demo Stability Foundations v0

## Status

Problem checkpoint, 2026-05-29. App-state/rendering checkpoint updated
2026-05-30.

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
- On 2026-05-30, mobile Safari staging evidence showed reload restoring the
  Email app as a small, translucent window opened to Drafts even when the owner
  had placed the system in other visible states. The Email content overflowed
  the window and overlapped a Trace window behind it. This is no longer just a
  stale `?app=email` problem; it shows app-owned view state, window geometry,
  foreground stack, and visual opacity can fail to round-trip as one coherent
  product state.
- Local reproduction found two concrete persistence hazards behind the Email
  reload failure: browser-derived fractional window geometry could make
  `/api/desktop/state` reject saves with `400 invalid request body`, and a
  stale desktop live-event merge could overwrite the current driver session's
  newer app context with an older `{guestMode, preview}` context before
  reload/pagehide flushed state.
- Follow-up mobile evidence showed Trace and Email restored in overlapping
  windows, with the front Trace panels visually blending with Email content
  underneath. This is a rendering invariant problem, not just persistence:
  app/window chrome may use glass styling inside the window, but every app
  window must have an opaque backing layer so foreground ownership is visually
  unambiguous.
- Deeper Comet inspection on 2026-05-30 found the remaining overlap symptom
  after the first app-state and opacity fixes. The live owner desktop state had
  12 open windows, 10 minimized windows, and two visible windows: Trace at
  `z_index: 11` with geometry `x:51 y:10 w:445 h:640`, and Email at
  `z_index: 12` with geometry `x:10 y:40 w:396 h:708`. Email was the durable
  active window and its app context had correctly round-tripped to Inbox/detail.
  A staging DOM probe with the same Trace-over-Email restore shape reported
  `opacity: 1`, normal overview state, and opaque window/content/app backing
  layers. This narrows the active defect: the current build no longer has a
  simple CSS alpha leak, but restored overlapping windows can still look
  visually blended on first paint because the shell relies on layered dark
  chrome, large shadows, rounded corners, and ordinary browser compositing
  before any user focus event repaints/raises a window.
- Follow-up visual evidence from mobile Safari still showed Trace cards
  legible through the foreground Email detail pane. Code inspection found the
  actual remaining leak: the global theme stylesheet overrides app structural
  panes such as `.message-detail`, `.message-list`, and `.mail-rail` to
  `var(--choir-panel-soft, rgba(18, 31, 55, 0.68)) !important`. That defeats
  Email's app-local opaque pane backgrounds even though the window shell and
  app host themselves are opaque. The rendering failure is therefore an
  app-theme layering bug: decorative soft panels are being applied to
  full-window structural panes that must occlude background windows.

## Invariants

- Browser presence is observational only. Opening, closing, reloading, or hiding a page must not control whether the background work continues.
- Frontend state is only a projection cache or transient input buffer. It must not be the authority for desktop layout, active app, VText work status, verification status, or background run lifecycle.
- VText working state must be derived from durable backend state, especially the document-scoped pending mutation/run state, not just a transient component variable.
- The visible desktop after reload should prefer the last durable desktop state unless the URL contains a current, explicit, unconsumed route intent.
- All sessions for one owner should converge on the same active desktop state. Session IDs are provenance and driver-lease metadata, not separate authoritative desktops.
- Public signed-out desktop state should be intentionally sparse: blank desktop or one explanatory VText, not multiple owner-like work windows.
- Progress UI should make uncertainty legible. "Working", "pending verification", "stalled", "failed", and "complete but not adopted" should not collapse into the same quiet text.
- App view state must be durable through a universal app protocol, not ad hoc
  per-app browser variables. Each app should expose serializable state,
  hydrate from the persisted state, and emit state changes through the desktop
  shell/API so reload, cross-device sessions, and app promotion have one
  consistent contract.
- Window persistence must carry enough layout invariants to prevent restored
  windows from becoming smaller than their content or visually transparent in a
  way that obscures foreground/background ownership.
- Desktop persistence payloads must be normalized before crossing the universal
  API boundary. Browser layout can produce fractional pixels; persisted window
  geometry is integer pixel state.
- While a browser session is the active driver, remote/live desktop merges may
  converge shared placement but must not regress that driver's fresher local
  app context with older remote context.
- Theme tokens may be translucent, but they are decorative overlays. The
  window shell and app host must provide a solid backing plane behind every
  mounted app so two restored windows cannot visually merge.
- Restored overlapping windows must be paint-isolated from each other. A
  foreground app may reveal background stack depth around its bounds, but
  background app pixels must never appear to bleed through the foreground
  window body, titlebar, or app host before the first focus event.
- Global theme overrides must distinguish decorative cards from structural app
  panes. Full-height panes, sidebars, detail panes, readers, toolbars, and
  app-stage surfaces inside a floating window need solid backing unless the app
  explicitly opts into transparency with a proven foreground-ownership design.

## First Stability Slice

This pass should stay small:

1. Simplify the signed-out public desktop to a single explanatory VText preview.
2. Make stale `?app=email` URL app launch one-shot so it does not keep overriding restored desktop state on reload.
3. Strengthen VText pending feedback by making document-level pending state visible and animated after stream snapshots/reconnects.
4. Add the first universal app-state persistence path: app surfaces can emit a
   context/state change, the desktop shell saves it in server-backed
   `app_context`, and restored apps hydrate from that state. Email Drafts is
   the initial regression because it currently exposes the gap.

Broader verification-state semantics, video QA capture, and campaign compiler promotion safety remain important, but they should follow after the basic visible/durable continuity loop is less ambiguous.

## Structural Debt

The rewritten frontend still uses Svelte stores and component variables in places that can behave like authority rather than projections. The durable target is stricter: desktop windows, active app, app context, VText work state, verification state, and run lifecycle should be backend facts with frontend stores rebuilt from snapshots and live events. Local component state is acceptable only for unsaved text input, pointer/drag gestures before commit, animations, and short-lived request plumbing.

## Acceptance

- A logged-out visit to `/` shows at most one public preview window.
- An authenticated reload with a stale Email URL intent does not repeatedly force Email above the restored active desktop state after the intent is consumed.
- A VText document with a pending agent mutation shows a clear visible working indicator after initial load or SSE reconnect.
- Backend VText stream snapshot behavior remains covered: snapshots include pending mutation state and clear stale terminal mutations before reporting pending.
- Switching the Email mailbox/view and reloading restores that Email state from
  server-backed desktop/app state, without relying on browser local storage.
- Restored mobile Email geometry is usable: it should not be smaller than its
  main content or become unintentionally transparent over another app.
