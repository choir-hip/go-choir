# Parallax: Diagnose the Email App Freeze

## Status

Open handoff. First staging diagnosis pass completed on 2026-06-23.

## Suggested Goal String

```text
Use Parallax on docs/paradoc-email-freeze-diagnosis.md. Treat the mission document as the source program and current handoff. Current status: open_handoff after first staging diagnosis pass. Variant V is open reported-freeze discriminator (1) + confirmed bootstrap/request hazards needing fix decision (3) + missing affected-account reproduction evidence (1) = 5. Authority: yellow diagnosis/docs/tests only unless the owner explicitly authorizes an orange frontend fix. Protected surfaces: Email app bootstrap/load state, auth/session renewal fetch path, desktop state persistence/suspension. Next move: decide whether to implement the low-risk Email bootstrap fix (single guarded initial load + request generation/abort timeout) or first obtain affected-account staging evidence; do not claim the hard freeze is fixed without reproducing it on staging. Ledger: docs/paradoc-email-freeze-diagnosis.ledger.md. Settlement requires a diagnosis/fix packet with console/network evidence, or an explicit blocker naming the missing affected-account reproduction authority.
```

## Parallax State

status: open_handoff
mission conjecture: if a browser-based staging probe captures the Email app freeze with console/network/app-state evidence, then Choir can identify whether Email state, auth renewal, desktop suspension, or another request path is the cause and scope the graph-view rewrite/fix correctly.
deeper goal (G): distinguish a short-term salvageable Email app defect from evidence that Email must become a view over object-graph state.
witness/spec (A/S): diagnosis document plus ledger evidence naming reproduction steps, console/network results, state variables involved, root-cause hypothesis, and proposed fix path.
invariants / qualities / domain ramp (I/Q/D): staging is the acceptance environment; no product behavior change in this pass; do not claim local proof; do not weaken Email UX; browser proof must capture console, network, and visible state.
variant (ranking function) V: open reported-freeze discriminator (1) + confirmed bootstrap/request hazards needing fix decision (3) + missing affected-account reproduction evidence (1) = 5; last delta: expected -2, actual -2 from source read plus two staging probes.
budget: first pass spent; remaining budget solvent only if the next move is either an owner-authorized frontend fix or an affected-account reproduction probe.
authority / bounds: yellow diagnosis/docs/tests. Orange frontend runtime changes require explicit owner authorization or a new implementation pass.
mutation class / protected surfaces: yellow docs/evidence. Protected surfaces investigated: Email app load state, auth/session renewal, desktop state persistence, desktop heavy-window suspension.
evidence packet: docs/email-freeze-diagnosis-2026-06-23.md; staging probe on https://choir.news with temporary users `email-freeze-probe-1782194649400-pqti8r@example.com` and `email-delay-probe-1782194725391-mdembt@example.com`; code refs in EmailApp/auth/desktop store.
heresy delta: discovered: Email bootstrap has duplicate initial network load and no request timeout/generation guard. introduced: none. repaired: none.
position / live conjectures / open edges: Fresh staging account did not reproduce a hard freeze. Desktop suspension conjecture is weakened because Email is `heavy: false` and suspension gates on `isHeavyAppId`. Auth renewal-loop conjecture is weakened because `fetchWithRenewal` performs at most one renewal attempt and probes showed no 401 loop. Email bootstrap/load-state conjecture is supported as a real hazard: `onMount` and the reactive `authenticated && !loadedOnce && !loading` block both start aliases/messages loads before `loadedOnce` flips, producing duplicate initial requests. A single delayed duplicate did not wedge the app; a fully hung request path or affected-account data/API latency remains the missing discriminator.
next move: either implement the low-risk Email bootstrap hardening (single guarded initial load, latest-request token, abort/timeout and stale-response ignore) as an orange frontend fix, or obtain affected-account staging evidence if the owner can supply/reproduce the freezing account/session.
ledger file: docs/paradoc-email-freeze-diagnosis.ledger.md
version / lineage: predecessor remains docs/object-graph-synthesis-2026-06-23.md; successor should be the Email bootstrap hardening or mail-object graph migration mission.
learning state: retained here and in docs/email-freeze-diagnosis-2026-06-23.md.
settlement: not settled for the reported hard freeze; open handoff with a bounded fix path and explicit missing evidence.

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
