# Parallax: Diagnose the Email App Freeze

## Status

Settled for branch-level diagnosis and local hardening on 2026-06-23. Not a staging repair claim.

## Suggested Goal String

```text
Use Parallax on docs/paradoc-email-freeze-diagnosis.md. Current status: settled for branch-level diagnosis and local Email bootstrap hardening, with affected-account freeze and deployed repair proof retained as explicit open edges. Branch head: `6706ae02`. Independent verifier thread `019ef323-c1a8-7640-bb77-a8e64c774160` returned `accept` with no blocking findings. Do not claim staging repair without landing-loop proof. If continuing, next move is either run the focused Playwright spec in a clean auth-origin harness or land/push through the platform behavior loop and verify staging. Ledger: docs/paradoc-email-freeze-diagnosis.ledger.md.
```

## Parallax State

status: settled
mission conjecture: if a browser-based staging probe captures the Email app freeze with console/network/app-state evidence, then Choir can identify whether Email state, auth renewal, desktop suspension, or another request path is the cause and scope the graph-view rewrite/fix correctly.
deeper goal (G): distinguish a short-term salvageable Email app defect from evidence that Email must become a view over object-graph state.
witness/spec (A/S): diagnosis document plus ledger evidence naming reproduction steps, console/network results, state variables involved, root-cause hypothesis, and proposed fix path.
invariants / qualities / domain ramp (I/Q/D): staging is the acceptance environment; no product behavior change in this pass; do not claim local proof; do not weaken Email UX; browser proof must capture console, network, and visible state.
variant (ranking function) V: 0 for this branch-level mission. Accepted residual edges are tracked below, not hidden: clean local Playwright harness run or staging proof (open), affected-account hard-freeze reproduction (open).
budget: spent; mission settled at branch level after independent verifier acceptance.
authority / bounds: orange frontend runtime hardening plus tests/docs. Do not claim staging repair without landing-loop proof. Do not weaken Email UX.
mutation class / protected surfaces: orange. Protected surfaces touched: Email app load state and Email browser regression coverage. Auth renewal and desktop suspension were inspected but not changed.
evidence packet: docs/email-freeze-diagnosis-2026-06-23.md; `npm run build` passed; focused Playwright spec added but local execution blocked by stale/mixed auth service origin configuration (`8081` already bound before `start-services.sh` auth could start); verifier thread `019ef323-c1a8-7640-bb77-a8e64c774160` accepted `6706ae02`.
heresy delta: discovered: duplicate initial Email bootstrap and no request ownership/timeout. repaired: local patch removes dual bootstrap, adds latest-request guards, and bounds Email fetches with timeout. introduced: none found by verifier.
position / live conjectures / open edges: Email bootstrap/load-state hazard is locally repaired in source and independently accepted. Desktop suspension and auth renewal loop remain weakened, not repaired surfaces. The exact affected-account hard freeze is still unreproduced; the fix is preventive for the confirmed request-state hazard, not proof that the reported freeze cannot recur. Local Playwright proof is blocked by harness auth-origin mismatch, not by observed Email behavior. Staging repair proof is not claimed.
next move: outside this settled branch-level mission, either run the focused Playwright spec in a clean local auth-origin harness or land/push through the platform behavior loop and verify staging.
ledger file: docs/paradoc-email-freeze-diagnosis.ledger.md
version / lineage: predecessor remains docs/object-graph-synthesis-2026-06-23.md; successor should be the Email bootstrap hardening or mail-object graph migration mission.
learning state: retained here and in docs/email-freeze-diagnosis-2026-06-23.md.
settlement: settled for branch-level diagnosis and local hardening. Independent verifier accepted the diff; residual edges are explicitly retained: affected-account hard freeze unreproduced, focused Playwright not run cleanly due local auth-origin harness, and no deployed/staging repair proof.

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
