# Parallax: M7 — Auth Account Recovery + Multi-Device

**Conjecture (C14):** Email magic link recovery and multi-device passkey
management can be added to the existing auth service without weakening
WebAuthn security or the API key system from M1.

**Class:** orange — auth behavior change, new endpoints, new DB tables
**Worktree:** /Users/wiz/.windsurf/worktrees/go-choir/m7-auth-recovery
**Branch:** orchestrator/m7-auth-recovery
**Depends on:** M1 (API key system now on main)

## Spec

Add to `internal/auth/`:

### Account Recovery
- `POST /auth/recovery/request` — email magic link generation
  - Rate limited (per-email, per-IP)
  - Magic link is single-use, time-limited (15 min)
  - Link contains opaque token (not user ID or email)
  - Token stored as SHA-256 hash (same pattern as API keys)
- `POST /auth/recovery/verify` — magic link verification
  - Validates token hash, expiry, single-use
  - On success: creates new WebAuthn registration challenge
  - Does NOT auto-login (must complete WebAuthn registration)

### Multi-Device Passkey Management
- `GET /auth/credentials` — list user's WebAuthn credentials
  - Returns: credential ID, name, created_at, last_used_at
  - Does NOT return private keys or transports
- `DELETE /auth/credentials/{id}` — remove a credential
  - Requires existing WebAuthn session
  - Cannot delete last credential (must have recovery or at least 2)
- `POST /auth/credentials/rename` — rename a credential
  - Requires existing WebAuthn session

### Session Management
- `GET /auth/sessions` — list active sessions
  - Returns: session ID, device info, created_at, last_used_at
- `DELETE /auth/sessions/{id}` — revoke a session
  - Cannot revoke current session via this endpoint (use logout)

### Rate Limiting
- Recovery request: 3 per email per hour, 5 per IP per hour
- All auth endpoints: 10 per IP per minute (existing + new)

## Invariants
- No weakening of WebAuthn (recovery is a fallback, not a bypass)
- No weakening of API key system (recovery creates WebAuthn, not API keys)
- Magic link tokens are hashed at rest (SHA-256)
- Rate limiting is enforced before any DB write
- Cannot delete last credential without recovery setup
- All new endpoints require existing auth (except recovery/request and
  recovery/verify)

## Acceptance Criteria
- `go test ./internal/auth/...` passes
- `go build ./...` passes
- Tests cover: magic link generation, magic link verification (valid,
  expired, reused, unknown), credential listing, credential deletion
  (including last-credential guard), session listing, session revocation,
  rate limiting
- No secrets in logs (tokens, hashes, keys are never logged)

## Verification
Run from the worktree:
```
cd /Users/wiz/.windsurf/worktrees/go-choir/m7-auth-recovery
nix develop -c go test ./internal/auth/...
nix develop -c go build ./...
```

Return: conjecture verdict (SUPPORTED/REFUTED/PARTIAL), test output,
and list of files created/modified.
