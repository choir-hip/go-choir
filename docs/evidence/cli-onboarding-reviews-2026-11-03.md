# CLI onboarding reviews (2026-11-03)

Tier 2 (red: auth and API key minting). Branch cli-agent-onboarding.

- Round 1 (f18f6c4a3): Devin accept, no blocking-real. DeepSeek v4.1 flash
  send_back: `{"scopes": null}` minted every scope including owner:acts
  (handlers.go), plus non-blocking: session-only scopes hard-coded, no
  concurrent consume test, `choir login` could echo a 200 body, decide by
  hash rather than row id.
- Fix (2920ada52): scopes decoded as raw JSON (absent = default; null and []
  are 400), `sessionOnlyScopes` and `DelegableScopes()`, a two-goroutine
  consume test, no 200 body printed, decide by row id with a partial unique
  index on pending user_code_hash and redraw on collision.
- Round 2 (one seat, Devin): accept. Non-blocking residuals: the unique
  index aborts bootstrap if a database already holds duplicate pending
  hashes (the table is new on this branch); the explicit-request 403 still
  names OwnerActsScope literally instead of iterating sessionOnlyScopes.
- Rebase onto texture export (64b4fd9fb): login's writer renamed
  writeFileAtomicMode (mechanical).
