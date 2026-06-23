# Ledger: Diagnose the Email App Freeze

## 2026-06-23 Pass 1

- Claim: staging browser proof can distinguish Email state-machine freeze from auth renewal or desktop suspension.
- Move: probe + source read.
- Expected delta V: -2 by reading named source files and running deployed Email app with console/network capture.
- Actual delta V: -2. Source read narrowed desktop suspension and auth renewal; staging probes captured duplicate initial Email requests but did not reproduce a hard freeze.
- Receipts: `frontend/src/lib/EmailApp.svelte`, `frontend/src/lib/auth.js`, `frontend/src/lib/stores/desktop.js`, `frontend/src/lib/Desktop.svelte`, `frontend/src/lib/apps/registry.ts`; staging temporary users `email-freeze-probe-1782194649400-pqti8r@example.com` and `email-delay-probe-1782194725391-mdembt@example.com`.
- Evidence class: staging smoke/diagnosis, not fix proof.
- Open edge: affected-account freeze remains unreproduced; no stack trace or console exception was observed.

