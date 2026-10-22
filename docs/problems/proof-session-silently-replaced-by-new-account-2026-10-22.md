# A deployed proof silently ran on a new account when its named session expired (2026-10-22)

Found 02:40 UTC while reading FP2 batch 1's step 5 re-run (Texture
acceptance spec, receipt
`evidence/texture-acceptance-2026-10-22T02-23-25-049Z.json`).

## Evidence

- The proof was invoked as
  `CHOIR_AUTH_STATE=~/.config/choir-qa/test-computers/fp2-proof-20261022.storage.json
  scripts/proofs/fp2-desk-boundary.sh --computer computer-9648031f… --step 5`.
  That record's user is `60527529…`, and its computer is `computer-9648031f…`
  (vm-80b173b9).
- The 01:14 run used that account; its T7 refresh names computer-9648031f.
- The 02:23 run's receipt names owner `f25b01e7…`, computer
  `computer-1f86c719…` (vm-fdcb6a45). The storage file's meta now reads
  `playwright-state-1792635806583-…@example.com`, rewritten at 02:23.
- Cause, read in code: `createAuthenticatedState`
  (`frontend/tests/helpers/auth-state.js`) validates the stored state. When
  the session no longer authenticates, it registers a fresh passkey account
  and writes that session to the same path, the one `CHOIR_AUTH_STATE`
  named. Nothing reports the swap. Why the 00:47 session stopped
  authenticating between 01:14 and 02:23 is not established. A hypothesis:
  the 01:14 run's own browser context rotated the refresh token (`GET
  /auth/session`), and the rotated cookie was never written back to the file.

## Harm

- The proof judged a different computer from the one named on the command
  line, and said nothing. Here it did no damage. FP2's confinement is in the
  guest image, so the new computer runs it too, and no owner, paper or
  delegate computer was touched. But a proof that names a computer must run
  on that computer.
- Each silent fallback mints one more test account and computer on Node B.
- The named record's session is overwritten, so the test account's
  browser session is lost. Its API key (separate) still works.

## Repair (yellow: test helper only)

When `CHOIR_AUTH_STATE` is set, the caller has named an account. An
invalid session then fails the run with "the named session no longer
authenticates; re-mint it" and never registers. Without
`CHOIR_AUTH_STATE`, the shared default state keeps today's
register-on-demand behavior.
