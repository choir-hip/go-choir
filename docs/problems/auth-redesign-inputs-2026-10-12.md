# Auth redesign inputs, 2026-10-12

Owner, 2026-10-12: "we do need to, for our production readiness, do another
redesign of the auth system. Not a priority right now until we get all the
functional requirements for automatic newspaper operational." The dead-code
census ([docs/archive/reports/dead-code-census-2026-10-12.md](../archive/reports/dead-code-census-2026-10-12.md)
B8; full evidence in its routes.md, A3) found the half-built auth surfaces
below. They stay in place until that redesign. This file collects them so
the redesign starts from what exists.

Status: open, deferred until after Autopaper's functional acceptance.
Mutation class of any change here: red (auth and session).

## What exists and does not work

1. **Account recovery cannot complete.** M7 (1734ec06, 2026-06-28) added
   `POST /auth/recovery/request` and `/auth/recovery/verify`
   (internal/auth/recovery.go). The design is sound on paper:
   - the magic link only opens a new passkey registration, and does not
     bypass WebAuthn;
   - tokens are stored as SHA-256, single use, and expire in 15 minutes;
   - requests are rate limited per email hash and per IP hash before any
     database write.

   The gap is that the request handler creates the token and never sends
   it: there is no mailer call. So `/verify` can never be reached with a
   real token, and an owner who loses their passkey has no way back in.
2. **Passkey and session management has no UI.**
   - The routes: `GET /auth/credentials`, `POST /auth/credentials/rename`,
     `DELETE /auth/credentials/{id}`, `GET /auth/sessions` and
     `DELETE /auth/sessions/{id}`.
   - No frontend ever called them (`git log -S'auth/recovery' -- frontend/src`
     is empty, and SettingsApp calls only api-keys, register, login, session
     and logout).
   - So there is no way to add a second device's passkey or revoke a
     session from the product.
3. **Root status is unreachable.** `GET /api/auth/root/status` (983b8995,
   2026-08-26) is registered on the auth service. Caddy routes `/api/*` to
   the proxy, which forwards it to the guest, which has no such route.
4. **These routes are reachable without being signed in** (recovery request
   and verify). They are rate limited, but every one is attack surface that
   delivers no function today.

## What the redesign has to decide

- Recovery: a delivery channel (maild is live and could send it), or a
  different recovery model such as a second registered passkey, recovery
  codes, or escrowed key unwrap with operator quorum. The key-escrow unwrap
  routes already exist; see the census, B10.
- Multi-device: a Settings panel over the existing credentials and sessions
  routes, or a replacement.
- Root and owner keys: where the ROOT key and PRF-capable passkey count are
  shown, and whether `/api/auth` should be routed to auth at all.
- Whether `/auth/*` endpoints that serve no UI stay mounted in the meantime.
