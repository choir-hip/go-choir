# Self-development verify cannot see signed-in surfaces, and judges that gap inconsistently

Date: 2026-10-26. Status: open; documented before any fix. Class of a platform fix: orange to red (capsule capture, run acceptance evidence).

## Evidence

Every app the owner asked for (Reader, window manager, Slides, Files, Calendar, Podcasts, the themes) lives behind sign-in. The capsule's visual capture renders routes of the frozen release signed out (`/`, `/read.html`), so the only screenshot management's verify sees is a public page. Engineering cannot run the repo's signed-in Playwright specs either: they need the auth stack on :8081 (the window manager engineer marked its spec "UNRUN IN THIS CAPSULE" and said why).

Management's verify has judged that same gap both ways:

| Round | Verify | Its stated basis |
|---|---|---|
| Reader 3 and 4 (2026-10-26 01:xx-02:xx) | pass | "non-blank and coherently rendered" (trace review 7, R7-2) |
| Reader 5a (04:52) | pass | "nonblank but shows a story-load error, not the auth-needed empty state" |
| Reader 5b (04:55) | fail | the screenshot "is the Autopaper home route ..., not the Reader sign-in state" (trace review 8, R8-3) |
| Window manager 1 (06:09) | fail | runtime behavior not verified; Playwright could not run |
| Window manager 1, management's own follow-up (06:57) | fail | same |
| Window manager 2 (07:51) | pass | build, diff reading; restoreWindow threw a ReferenceError on every call (FP0's render found it) |
| Window manager 3 (08:05) | fail | "runtime behavior was not established", although the direction said a signed-out capture is expected and named the checks to verify on |

So the gate both blocks correct work and passes broken work, and the reason is recorded only in prose (Verify takes no reason: R8-2, SD-09). FP0's real-shell render with a mocked backend (scratchpad/design/capture-slow.mjs, wm-probe*.mjs) is today the only runtime evidence for any signed-in app change.

## What would close it (candidates; not decided)

1. Direction-level, no platform change: engineering renders the signed-in shell in the capsule against a mocked backend (Vite plus Playwright `context.route`, the store chromium), and attaches the screenshots and measurements as visual evidence. Being tried in window manager round 4 (sent 08:10:55).
2. Platform: the capsule capture gains a signed-in shell mode that serves the frozen frontend with a fixture session and fixture desktop state, so every round gets signed-in screenshots without each engineer rebuilding the harness (trace review 7, R7-7).
3. Verify records its reason as data (R8-2), so an inconsistent judgment is visible without a trace review.
