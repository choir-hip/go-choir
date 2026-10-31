# Email sign-in

Date: 2026-10-31. Status: design, for a two-seat review (Devin, DeepSeek)
before code. Mutation class of the build: red (auth and session).

## Why

Owner, 2026-10-14: "In addition to pass keys, we should have email magic link
auth. I don't want to deal with the Google setup or Apple or other sign in
with X systems quite yet." (station LA,
docs/archive/reports/roadmap-sequencing-appdev-security-2026-10-14.md). Owner,
2026-10-31: "for mac, we should think about how to enable fully automated
testing ... I've been meaning to have a non-passkey method actually", and "Yes,
email signin design. Let's design and review that and move forward with it on
agentic consensus acceptance. That unblocks the Mac app."

Two needs, one mechanism:

1. People whose passkey fails (a shared computer, a browser without passkey
   support, a lost device before recovery) can still sign in.
2. The Mac app can be tested end to end with no person present, through the
   product's own sign-in path. A test signs in a test account by email, reads
   the code from that account's inbox, and enters it. Nothing exists only for
   tests.

## What exists (main at 0cbec9b35 and later)

- internal/auth/recovery.go (M7, 2026-06-28): recovery magic link. Opaque
  tokens stored as SHA-256, single use, 15-minute expiry, rate limited per
  email hash (3/hour) and per IP hash before any database write. The link only
  opens a new passkey registration.
- internal/auth/recovery_mail.go and internal/maild/auth_mail.go: the auth
  service calls maild's host-internal `/internal/` route (loopback plus the
  host-internal token; Caddy refuses `/internal/` at the edge; a guest can
  never send Choir-branded sign-in mail). maild renders a fixed template.
- internal/auth/handlers.go: `issueSession` is the one cookie issuer;
  `HandleDesktopExchangeRedirect` and `HandleDesktopRedeem` hand a session to
  the Mac app through a one-time code over the `choir://` scheme.
- Passkey login can unwrap the owner's data ROOT through the passkey PRF
  (`unwrapOwnerRoot`). Failure there is already non-fatal to login: key
  availability is a separate capability from authentication.
- maild receives inbound mail for choir.news addresses (Resend webhook).

## Threat model

An email code is exactly as strong as the inbox. The design accepts that and
limits what an email session can reach.

Attacks considered:

- T1 guessing a code. 8 digits; 5 wrong tries kill the code; per-email and
  per-IP request limits; codes die after 10 minutes.
- T2 account enumeration. `begin` answers the same way, in the same time
  budget, for a known and an unknown address.
- T3 mail interception or a forwarded code. A code is bound to the attempt
  that asked for it (an attempt handle held by the requesting client), so a
  code read off a stolen screen is useless to a different client unless the
  attacker also holds that handle. The link variant carries its own
  high-entropy token and is single use.
- T4 phishing: a fake site asks the user to request a code and type it in.
  The mail names the device and place that asked, says "never give this code
  to anyone", and the code only works with the attempt handle; a phisher
  relaying in real time still wins, as with any OTP. Passkeys remain the
  phishing-resistant method; the settings page says so.
- T5 an email session escalating. It cannot add, rename or remove passkeys,
  cannot create API keys, cannot unwrap the data ROOT, and cannot change the
  account email. Those require a passkey session (or the recovery path, which
  registers a passkey). It can use the product as the owner otherwise,
  including owner acts, since the computer serves the signed-in owner.
- T6 a guest computer minting sign-in mail. Unchanged: maild's route is
  host-internal and refuses guests.
- T7 replay of a consumed code or link. Consumption is one atomic update; a
  second use fails.
- T8 a stolen attempt handle alone. Useless without the code.
- T9 mail bombing a victim. Per-email limit (5 per hour), per-IP limit (10 per
  hour), and the mail says how to report it.

Out of scope here: Sign in with Google/Apple (launch checklist L4-04, later),
sign-up by email (see Decisions).

## Flow

### Web

1. `POST /auth/email/begin {email}` returns `{attempt_id}` always (T2). If the
   address belongs to an account and the limits allow, the auth service
   creates an attempt row and asks maild to send the fixed-template mail with
   an 8-digit code and a link.
2. The user either types the code: `POST /auth/email/verify {attempt_id, code}`,
   or opens the link `/signin/email?t=<token>`, which posts the token to
   `POST /auth/email/verify {token}` from Choir's own page.
3. On success the auth service calls `issueSession` with
   `auth_method=email_code` or `email_link`. Same cookies, same refresh
   rotation.

### Mac app

The app calls the same two endpoints from its native Go side with its own
cookie jar (`desktopSession`), so the session lands in the app's jar through
the canonical issuer. The app shows "Sign in with email" beside the passkey
button: email field, then code field. No `choir://` handoff is needed for this
path; the passkey path is unchanged. The link variant, opened on the same Mac,
completes in the browser; the app then uses the existing exchange handoff.

### Automated test (no person)

A test account whose address is a choir.news address delivered into a test
computer's inbox (d@e's computer, or a mailbox it owns). The test:

1. drives the app (or its native sign-in function) to `begin`;
2. reads the newest sign-in mail from that inbox through the computer's own
   mail API with that computer's scoped key;
3. submits the code;
4. checks the session, then runs the sync E2E.

The test uses only product paths. If d@e's account address is not a
choir.news address, the build adds a dedicated test account on a choir.news
address and records its id (never an owner address in the repo).

## Data

New table `email_signin_attempts`:

| column | meaning |
|---|---|
| id | attempt id (random, 128 bits, returned to the client) |
| user_id | null when the address matched no account (row still written, T2) |
| email_hash | SHA-256 of the normalized address |
| code_hash | HMAC-SHA-256(server key, attempt_id + code) |
| link_token_hash | SHA-256 of the link token |
| ip_hash, user_agent_summary | for the mail text and limits |
| created_at, expires_at | 10-minute life |
| attempts | wrong-code count; at 5 the row is dead |
| consumed_at | set once, atomically |

Rows older than 24 hours are deleted by the auth service's existing cleanup.

`refresh_sessions` gains `auth_method` (`passkey`, `email_code`,
`email_link`, `recovery`, `api_key` is separate). Existing rows read as
`passkey`.

## Capability gating (T5)

One helper, `requirePasskeySession`, guards: credential add/rename/delete,
API key create, account email change, ROOT status/unwrap. It refuses an
`email_*` session with a clear message: "Sign in with a passkey to do this."
Everything else accepts any session. The proxy's owner-act signing is
unchanged.

## Limits

- per email hash: 5 `begin` per hour, counted whether or not the address has
  an account;
- per IP hash: 10 `begin` per hour;
- per attempt: 5 wrong codes;
- `verify` per IP hash: 30 per hour.

## Failure modes, each with a test before code

F1 wrong code 5 times kills the attempt; the right code after that fails.
F2 an expired code fails. F3 a consumed code or link fails on second use,
including two concurrent verifies (exactly one wins). F4 `begin` for an
unknown address returns the same shape and status, writes a row, and sends
nothing. F5 the code works only with its attempt id. F6 limits hold across
concurrent requests. F7 an email session is refused on every
`requirePasskeySession` route. F8 a passkey session is unaffected. F9 maild
down: `begin` still answers the same, the attempt is marked unsent, and the
log names it. F10 the mail text never contains the attempt id or the link
token's hash. F11 the auth service never logs a code, token or address
(hashes only). F12 a guest cannot reach maild's sign-in route. F13 the Mac
app's native verify puts the session in its own jar and nowhere else.

## Rollback

A switch `auth.email_sign_in` (on for staging, per ring rules) hides the UI
and makes `begin` answer as if every address were unknown. Code revert removes
the routes; the table and the column are additive.

## Decisions for the panel

- D1 Sign-up by email: not in v1. New accounts still register a passkey.
  Revisit after launch onboarding (L4-06).
- D2 Session lifetime for email sessions: same as passkey sessions in v1, or
  shorter (for example 7 days)? Proposed: same, because the session is
  already marked and gated.
- D3 Code length: 8 digits vs 6. Proposed 8, with the attempt binding.
- D4 Whether the link variant ships in v1 or only the code. Proposed both;
  the link is what most people expect.
