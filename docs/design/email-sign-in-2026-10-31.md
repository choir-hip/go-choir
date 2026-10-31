# Email sign-in

Date: 2026-10-31. Status: design v2, revised after the tier-2 review (Devin
send_back: three blocking, nine should-fix or notes; see "Review round 1"
at the end). Mutation class of the build: red (auth and session).

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
  budget, for a known and an unknown address. This only keeps the new route
  from adding an oracle: `login/begin` (404) and `register/begin` (409)
  already reveal registration today. Closing those is a separate change.
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
- T10 account squatting: registration proves no inbox, so someone could
  register a victim's address with their own passkey and later receive the
  victim's email sign-in into an account the attacker also holds. Email
  sign-in therefore requires a verified address (`users.email_verified_at`,
  below).
- T9 mail bombing a victim. Per-email limit (5 per hour), per-IP limit (10 per
  hour), and the mail says how to report it.

Out of scope here: Sign in with Google/Apple (launch checklist L4-04, later),
sign-up by email (see Decisions).

## Flow

### Web

1. `POST /auth/email/begin {email}` returns `{attempt_id}` always (T2) and
   sets the attempt cookie `choir_signin_attempt` (HttpOnly, Secure,
   SameSite=Strict, Path=/auth/email, 10 minutes) holding a second random
   value whose hash is on the attempt row. If the address belongs to an
   account **with at least one passkey credential and a verified address**
   (otherwise it is treated as unknown: no sign-up by email, D1; T10) and the limits allow, the auth service
   marks that user's earlier unconsumed attempts dead, creates the attempt row
   and asks maild to send the fixed-template mail with an 8-digit code and a
   link.
2. The user either types the code: `POST /auth/email/verify {attempt_id, code}`,
   which also requires the attempt cookie and `Content-Type:
   application/json` (a cross-site `text/plain` form cannot send it) (a code works only in the client
   that asked for it, which closes login CSRF: an attacker cannot make a
   victim's browser verify the attacker's attempt), or opens the link
   `/signin/email#token=choir_st_<32 bytes>` (fragment, as recovery does, so
   the token never reaches access logs, Referer or history), whose page posts
   the token to `POST /auth/email/verify {token}`. The link is a bearer
   credential: a forwarded mail signs in whoever opens it first. That is the
   accepted residual of the link variant, the same class as recovery links.
3. On success the auth service calls `issueSession` with
   `auth_method=email_code` or `email_link`. Same cookies, same refresh
   rotation; the method rides the session (see "Carrying the method").

### Mac app

The app calls the same two endpoints from its native Go side with its own
cookie jar (`desktopSession`), so the attempt cookie and the session both land
in the app's jar through the canonical issuer. The app shows "Sign in with
email" beside the passkey button: email field, then code field. No `choir://`
handoff is needed for this path; the passkey path is unchanged. The link
variant, opened on the same Mac, completes in the browser; the app then uses
the existing exchange handoff, which carries the method (below), so a
browser email session becomes an email session in the app, never a passkey
one.

### Automated test (no person)

A test account whose address is a choir.news address delivered into a test
computer's inbox (d@e's computer, or a mailbox it owns). The test:

1. drives the app (or its native sign-in function) to `begin`;
2. reads the newest sign-in mail from that inbox through the computer's own
   mail API with that computer's scoped key;
3. submits the code;
4. checks the session, then runs the sync E2E.

The test uses only product paths. No existing account has a choir.news
address (checked on Node B 2026-10-31, domains only), so the build adds a
dedicated test account on a choir.news address and records its id (never an
owner address in the repo). Because `begin` mails only accounts with a
passkey, the test account gets one passkey once, through a virtual WebAuthn
authenticator in a browser run (Playwright's CDP authenticator), the same
registration path a person uses. The scoped key the test reads the inbox
with cannot be minted by an email session (the gate refuses it); it is minted
once from that passkey session and kept in the developer's gitignored env.

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

Plus `cookie_hash` (SHA-256 of the attempt cookie value) and `sent`
(false when maild failed, F9). A row whose user is null gets a code hash that
matches nothing, so verify on it fails exactly like a wrong code.

Rows older than 24 hours are deleted by a new sweeper goroutine in the auth
service (the `CleanExpired*` store methods have no production caller today);
expired rows are the rate-limit history, so 24 hours is the floor.
`code_hash` is HMAC-SHA-256 under `AUTH_EMAIL_CODE_KEY` (the service refuses
to start with the switch on and no key); compare with `hmac.Equal`.
`email_hash` is HMAC under the same key, not bare SHA-256, so a database read
does not recover addresses by dictionary.

`refresh_sessions` gains `auth_method` (`passkey`, `email_code`,
`email_link`); existing rows read as `passkey`. `desktop_exchange_codes`
gains `auth_method` too.

`users` gains `email_verified_at`. The deploy backfills every existing
account as verified: registration has been invite-capped
(`internal/auth/registration_cap.go`), so the existing set is known people.
A new account verifies its address from settings while signed in with a
passkey: `POST /auth/email/verify-address/begin` mails a code through the
same attempt machinery (purpose `verify_address`), and
`POST /auth/email/verify-address/finish` sets the mark. Recovery also sets it,
since a completed recovery proves the inbox. Addresses are compared exactly as
`GetUserByEmail` stores them (trimmed, lowercased at registration); the hash
uses the same normalization.

### Carrying the method

The method must survive every way a session is re-minted (review B1-B3):

- `issueSession` and `generateRefreshToken` take the method; register finish
  and login finish pass `passkey`, email verify passes `email_code` or
  `email_link`.
- `rotateRefreshSession` copies the old row's method into the new row. A test
  asserts the method survives many rotations.
- The access JWT carries the method as an `am` claim, minted at issue and at
  every rotation from the refresh row. The gate reads the claim, so it works
  on every route, including `/api/auth/root/status`, which never sees the
  `Path=/auth` refresh cookie. A JWT without `am` (issued before the deploy)
  reads as `passkey`, and such tokens live at most the 5-minute access TTL.
- `HandleDesktopExchangeRedirect` copies the method from the refresh session
  it already validates into the exchange code; `HandleDesktopRedeem` stamps
  it on the session it issues.
- Owner acts stay open to email sessions (decision, both seats flagged it):
  the computer serves the signed-in owner, and inbox control already reaches
  the account through recovery, so gating owner acts would not raise the
  bar. The gate covers what outlives the session: credentials, keys, other
  sessions and the data ROOT.
- Owner-act receipts record `auth_method=session` for any cookie session
  today. v1 keeps that (the verifier refuses unknown methods); the receipt's
  session id joins to the refresh row's method for audit. Extending the
  receipt enum is a follow-up.

## Capability gating (T5)

One helper, `requirePasskeySession` (`requireAuthUser` plus the `am` claim
not `email_*`), guards the routes that exist (review S5):

- `POST /auth/credentials/rename`, `DELETE /auth/credentials/{id}`;
- `POST /auth/api-keys`, `DELETE /auth/api-keys/{id}`;
- `DELETE /auth/sessions/{id}`, `POST /auth/logout?all=true`;
- `GET /api/auth/root/status` and the ROOT unwrap.

Listing, reads and self-logout stay open. There is no credential-add route
for a session (adding a passkey goes through recovery) and no email-change
route, so neither needs a gate yet; a future one is born gated. The gate
applies only to cookie sessions: a Bearer API key is never an email session
and keeps its existing scope rules (`manage:keys` may still mint sub-scoped
keys). It refuses with "Sign in with a passkey to do this." The proxy's
owner-act signing is unchanged.

## Limits

- per email hash: 5 `begin` per hour, counted whether or not the address has
  an account;
- per IP hash: 10 `begin` per hour;
- per attempt: 5 wrong codes;
- `verify` per IP hash: 30 per hour;
- one live attempt per user: a new `begin` kills the earlier ones.

Each limit counts and inserts in one transaction (the store runs with one
open connection, so the transaction serializes), which F6 needs. The wrong-code
count and consumption are single conditional updates, as
`ConsumeRecoveryToken` does: `attempts = attempts + 1 WHERE id = ? AND
consumed_at IS NULL AND attempts < 5`, then the compare; `consumed_at = now
WHERE consumed_at IS NULL`, rows affected must be 1. Exactly one concurrent
verify wins. The new routes also sit behind the global per-IP limiter.

## maild

The sign-in mail route is new code on a mux that also serves guest-reachable
routes, so it copies the recovery route's gate verbatim (review S7): the
loopback host check, `X-Internal-Caller`, `hostauth.Authorize`, and the
refusal of any `X-Authenticated-User` header. Its link validator pins the
origin to the primary domain (or localhost in development), the exact path
`/signin/email`, no query and no userinfo, the token only in the fragment;
the link is never echoed or logged.

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
app's native verify puts the session in its own jar and nowhere else. F14 the
method survives ten rotations. F15 an email session made in the browser and
handed to the app over the exchange stays an email session. F16 a code
verify without the attempt cookie fails. F17 an account with no passkey gets
no mail. F18 a second `begin` kills the first attempt's code. F19 an email
session past 7 days cannot refresh. F20 every gated route refuses an email
session and accepts a passkey session and a scoped Bearer key as before. F21
an account with an unverified address gets no mail. F22 verify without
`application/json` fails. F23 with the switch off, an outstanding attempt
cannot verify. F24 maild refuses the sign-in route when hostauth is not
enforcing.

## Rollback

A switch `auth.email_sign_in`, registered in `switches/registry.yaml` (on for
staging, per ring rules), hides the UI and makes `begin` answer as if every
address were unknown; with it off, `verify` also refuses and the
`/signin/email` page shows the passkey path, and turning it off deletes every
unconsumed attempt, so no outstanding code or link can still mint a session.
maild's route requires hostauth in enforce mode (it refuses in off or observe,
since loopback alone is not proof across the guest DNAT). Code revert removes
the routes; the table and the column are additive.

## Decisions for the panel

- D1 Sign-up by email: not in v1. New accounts still register a passkey.
  Revisit after launch onboarding (L4-06).
- D2 Session lifetime: **7 days** for email sessions (review: an
  inbox-strength session still reaches owner acts and every product surface,
  so it should not last the passkey's 30 days). The refresh row's expiry is
  set from the method at issue, and rotation keeps the original expiry for
  email sessions rather than extending it.
- D3 Code length: 8 digits vs 6. Proposed 8, with the attempt binding.
- D4 Both code and link ship in v1, the link in the fragment form with the
  maild validator.

## Review round 1 (2026-10-31)

Devin (send_back): B1 the method was lost on refresh rotation; B2 the gate
had no source for the method outside `/auth`; B3 the desktop exchange
laundered an email session into a passkey one. Should-fix: S4 login CSRF on
the code path; S5 the gate list named routes that do not exist and missed
destructive ones that do; S6 the Bearer branch; S7 maild's gate and link
validator; S8 the link token in the query; S9 mail to accounts with no
passkey is sign-up by email; S10 several live attempts; S11 no production
cleanup and racy limits; notes on conditional updates, null-user rows, the
HMAC key, the method enum, receipts, keyed email hash, the switch registry
and the test key. All adopted above.

DeepSeek (send_back): the same three blocking defects (method absent from
the access credential, lost on rotation, laundered by the desktop exchange),
plus: owner acts open to email sessions (kept, as a stated decision above);
CSRF through `text/plain` forms (JSON content type plus the attempt cookie);
the link in the query (fragment); the conditional updates; maild's
`X-Internal-Caller` being a hint and hostauth passing in off/observe (enforce
required); the missing `sent` column and cleanup; a rollback that left verify
live (now refused, attempts deleted); account squatting through unverified
registration (verified address required, existing accounts backfilled);
enumeration already open through login/register (stated as pre-existing);
the Bearer branch; the test bootstrap (one virtual-authenticator passkey).
D2: 7 days absolute, no sliding (adopted). All adopted or decided above.
