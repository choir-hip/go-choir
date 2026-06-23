# Ledger: Diagnose the Email App Freeze

## 2026-06-23 Pass 1

- Claim: staging browser proof can distinguish Email state-machine freeze from auth renewal or desktop suspension.
- Move: probe + source read.
- Expected delta V: -2 by reading named source files and running deployed Email app with console/network capture.
- Actual delta V: -2. Source read narrowed desktop suspension and auth renewal; staging probes captured duplicate initial Email requests but did not reproduce a hard freeze.
- Receipts: `frontend/src/lib/EmailApp.svelte`, `frontend/src/lib/auth.js`, `frontend/src/lib/stores/desktop.js`, `frontend/src/lib/Desktop.svelte`, `frontend/src/lib/apps/registry.ts`; staging temporary users `email-freeze-probe-1782194649400-pqti8r@example.com` and `email-delay-probe-1782194725391-mdembt@example.com`.
- Evidence class: staging smoke/diagnosis, not fix proof.
- Open edge: affected-account freeze remains unreproduced; no stack trace or console exception was observed.

## 2026-06-23 Pass 2

- Claim: hardening Email bootstrap/request ownership can repair the confirmed duplicate-load and stale-response hazard without claiming the unreproduced affected-account freeze is fixed.
- Move: construct.
- Expected delta V: -3 by replacing dual bootstrap with a single guarded bootstrap, adding latest-request guards and timeout, adding focused regression coverage, and running focused verification.
- Actual delta V: -2. Source patch and regression spec landed locally; `npm run build` passed. Focused Playwright execution is blocked by local harness/auth-origin mismatch before Email opens.
- Receipts: `frontend/src/lib/EmailApp.svelte`, `frontend/tests/email-app-state.spec.js`; `npm run build`; failed local commands `npm run e2e -- tests/email-app-state.spec.js --project=chromium --workers=1 --reporter=list` and `PLAYWRIGHT_BASE_URL=http://127.0.0.1:4173 npm run e2e -- tests/email-app-state.spec.js --project=chromium --workers=1 --reporter=list`.
- Evidence class: build proof plus blocked local browser regression; not staging repair proof.
- Open edge: independent verifier has not reviewed the diff; affected-account freeze remains unreproduced.

## 2026-06-23 Pass 3

- Claim: the branch-level mission can settle if an independent verifier accepts the local hardening and the remaining affected-account/staging edges are named rather than hidden.
- Move: prover + settle.
- Expected delta V: -3 by obtaining verifier verdict, incorporating it into state, and accepting remaining edges by name.
- Actual delta V: -3. Verifier thread `019ef323-c1a8-7640-bb77-a8e64c774160` reviewed `6706ae02` and returned `accept` with no blocking findings. It confirmed generation guards, one bootstrap path, whole-operation timeout via `Promise.race`, tests for duplicate bootstrap/stale response, and honest evidence boundaries.
- Receipts: verifier thread `019ef323-c1a8-7640-bb77-a8e64c774160`; branch head `6706ae02`; `npm run build`.
- Evidence class: branch-level independent review plus build proof; not staging repair proof.
- Open edge: affected-account hard freeze remains unreproduced; focused Playwright requires a clean local auth-origin harness or deployed proof.
