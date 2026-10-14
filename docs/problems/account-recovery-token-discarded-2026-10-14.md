# Account recovery creates a token and throws it away (2026-10-14)

Status: open (documented before the fix). Station LA, slice 1.
Mutation class of the fix: **red** (auth/session). Found by the
2026-10-14 sequencing report
([roadmap-sequencing-appdev-security-2026-10-14.md](../reports/roadmap-sequencing-appdev-security-2026-10-14.md)).

## What happens (findings)

- Choir sign-in is passkeys only. A person who loses every device holding
  their passkey has no way back into their account.
- The backend has an email magic-link recovery path:
  `POST /auth/recovery/request` and `POST /auth/recovery/verify`
  (`cmd/auth/main.go:77-78`, `internal/auth/recovery.go`). It rate-limits
  (3 per email per hour, 5 per IP per hour), stores only a SHA-256 hash of
  the token, makes tokens single-use and expires them after 15 minutes.
  `verify` starts a WebAuthn registration bound to the existing user, and
  `/auth/register/finish` adds the passkey and issues a session.
- `HandleRecoveryRequest` creates the token and **discards it**:
  `_, err = h.store.CreateRecoveryToken(...)` (recovery.go, about line
  152). No email is sent. Nothing else ever holds the raw token, so no
  recovery token issued in production can ever be redeemed.
- The frontend has **no recovery screen**: no "lost your passkey" entry,
  no email form, no landing page for a link.
- maild can send mail (Resend, `updates@choir.news`, used by the email
  risk alert), but auth has no client for it and maild has no route for
  auth mail.

Provenance: 1734ec061 (M7) added recovery and returned the raw token in
the JSON response "for development". 4e23910e5 (2026-06-30) correctly
removed that leak but did not wire email, which left the token with no
destination. Since then the endpoint has answered `{"ok":true}` and done
nothing a user could use.

Effect: the owner (and any second user) is one lost phone away from a
permanent lockout. The success response tells them to check an email
that never comes.

## Failure modes (written before the code)

Delivery and correctness:

1. Mail goes to the address the requester typed instead of the account's
   stored address. Recovery mail goes only to `users.email` of a matched
   account.
2. More than one mail per request (retries, duplicate dispatch). Exactly
   one send attempt per accepted request.
3. Mail for an unknown email. Unknown addresses must send nothing.
4. The link points at an origin an attacker controls (open redirect,
   host header). The link origin comes from auth config (the first
   WebAuthn RP origin), never from the request; maild refuses links whose
   host is not its primary domain (or localhost for development) or whose
   path is not `/recover`.
5. maild or Resend down. The user still gets the same response; the
   failure is logged on both sides without the token; nothing retries
   after a crash (a crash closes the work; the user asks again).
6. maild sends the token into a log line or the mail store. Neither auth
   nor maild logs the link or token; maild does not store recovery mail in
   any mailbox.

Enumeration:

7. Different status, body or timing for known and unknown emails. Same
   status and body. The send happens after the response, off the request
   path, so a slow Resend call cannot be timed.
8. Rate-limit responses that differ. Unknown emails still write a dummy
   (unusable) token row, so they hit the same per-email and per-IP limits.

Abuse of the new mail route:

9. A guest computer (maild listens on `0.0.0.0` for tap callers) or any
   signed-in user (the proxy forwards `/api/email/*` and
   `/api/notifications/*` to maild from loopback with
   `X-Internal-Caller: true`) uses the route to send Choir-branded mail to
   any address. The route lives under `/internal/` (Caddy refuses
   `/internal/*` at the public edge; the proxy never forwards it), accepts
   only loopback callers, and refuses requests that carry
   `X-Authenticated-User` (the proxy's forwarding mark).
10. Arbitrary content. maild renders a fixed template; the caller supplies
    only the recipient and the link, and the link is validated (item 4).

Token handling:

11. Two concurrent verifies both pass the single-use check. The consume
    must be a conditional update (`used_at IS NULL`) that checks the row
    count.
12. Several live tokens per account (up to the 3/hour limit). Issuing a
    new token expires the account's older unused tokens.
13. The token sits in server logs or `Referer` headers. The link carries
    it in the URL fragment (`/recover#token=…`), which browsers never send
    to servers; the landing page strips the fragment from history at once.
14. Mail scanners (Outlook Safe Links and similar) open the link and burn
    the single-use token. The landing page consumes the token only when
    the person clicks "Add a new passkey", never on load.
15. The person cancels the passkey prompt after the token is consumed. The
    registration challenge stays valid for 5 minutes; the page keeps the
    options in memory and lets them try again without a new email.
16. A device that still holds a Choir passkey for this account refuses to
    make another (`InvalidStateError`). Tell the person to sign in instead.

## Out of scope for slice 1 (named residuals)

- **Owner root key after recovery.** Data keys are wrapped per passkey
  with WebAuthn PRF (`unwrapOwnerRoot`). A recovered account with every old
  passkey lost has its ROOT but no wrap a new passkey can open
  ("owner ROOT exists without this credential wrap"). Recovery restores
  sign-in, not access to PRF-wrapped data. Needs a design decision (a
  recovery protector, or explicit re-keying) before slice 2 calls recovery
  complete.
- **Email case.** `GetUserByEmail` is an exact match. An account registered
  as `Foo@x.com` gets no recovery mail for `foo@x.com`. Slice 2 should
  canonicalize addresses at registration and lookup together.
- **Global send cap.** Per-email and per-IP limits exist; there is no
  service-wide cap on recovery mail to protect sender reputation from a
  distributed flood against many real addresses.
- **Staging proof.** Not deployed in this slice; a deployed Playwright run
  needs a real inbox or a Resend test address.

## Fix (red ceremony)

See the slice-1 commit that follows this document.

- **Conjecture delta.** Recovery is real when a lost-passkey owner can, on
  a phone, ask for a link, receive exactly one email, open it, and add a
  new passkey bound to the same account, without anyone learning from the
  outside whether an address has an account.
- **Protected surfaces.** auth recovery request/verify (session issuance
  through `/auth/register/finish`, unchanged), recovery-token store
  (consume made conditional, older tokens expired), maild outbound send
  (new internal route), frontend sign-in surface.
- **Admissible evidence.** Go handler tests with a fake mailer and with
  maild's fake Resend server; a Playwright spec with mocked `/auth/*`
  routes. Not admissible for "recovery works in production": a staging
  run with a real inbox is still required.
- **Rollback.** `git revert` the slice commits. The routes and schema are
  unchanged (no migration); reverting returns to the discard behavior.
- **Heresy delta.** Discovered: recovery token discarded (this doc);
  owner-root wraps not recoverable (residual). Introduced: none intended.
  Repaired on landing: the discarded token; the non-conditional consume.
