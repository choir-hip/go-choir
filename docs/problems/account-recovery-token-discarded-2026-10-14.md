# Account recovery creates a token and throws it away (2026-10-14)

Status: slice 1 fixed and rebased for landing on `land/account-recovery`
(not pushed, not deployed); slice 2 open. Station LA. On landing, slice 1
supersedes the owner's "delete `auth/recovery/request` for now" ruling;
see "Landing reconciliation" below.
Mutation class of the fix: **red** (auth/session). Found by the
2026-10-14 sequencing report
([roadmap-sequencing-appdev-security-2026-10-14.md](../archive/reports/roadmap-sequencing-appdev-security-2026-10-14.md)).

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

Slice 1 landed on a branch after this document (not pushed, not
deployed):

- **auth** (`internal/auth/recovery.go`, `recovery_mail.go`):
  `HandleRecoveryRequest` keeps the token and, for a matched account
  only, hands `<first RP origin>/recover#token=<token>` to a
  `RecoveryMailer` addressed to `users.email`. Delivery runs in a
  goroutine with a 30-second timeout after the response is decided; a
  failure is logged (`operation=recovery_mail result=error`, no token)
  and never changes the answer. `AUTH_MAILD_URL` (default
  `http://127.0.0.1:8087`) selects maild; empty disables mail and logs
  `reason=mailer_not_configured`.
- **token store** (`internal/auth/store.go`): a new token marks the
  account's older unused tokens used (superseded), in one transaction
  with the insert; rows are kept so rate-limit counts still see them.
  The consume update is now conditional (`used_at IS NULL`) and checks
  the row count. The store already runs on one SQLite connection, so
  concurrent consumes were serialized before; the condition makes
  single-use hold without relying on that.
- **maild** (`internal/maild/auth_mail.go`): new route
  `POST /internal/maild/auth/recovery-link` takes
  `{to_address, link, expires_minutes}`, refuses non-loopback callers,
  requests without `X-Internal-Caller: true`, and requests carrying
  `X-Authenticated-User`; validates the link (https on the primary domain
  or localhost, path `/recover`, fragment `token=choir_rt_…`, no query or
  userinfo); renders a fixed template from `Choir <updates@<primary
  domain>>`, subject "Recover your Choir account"; logs only a recipient
  hash and the provider message id.
- **frontend**: "Lost your passkey?" in the sign-in view opens an email
  form (`RecoveryRequestForm.svelte`) and then "Check your email"
  (copy says "If … belongs to a Choir account"). `/recover`
  (`RecoverRoute.svelte`, routed in `App.svelte` without a session check
  or computer handoff) reads and strips the fragment, redeems the token
  only on "Add a new passkey", runs the passkey creation through the
  shared `createPasskeyFromOptions` (now also used by account creation),
  retries a cancelled prompt with the same options, and offers a new link
  for a used or expired one.

Evidence: `go test ./internal/auth ./internal/maild ./cmd/auth` (new:
`recovery_mail_test.go`, `auth_mail_test.go`, including one test that
runs auth's mailer against a real maild handler and a fake Resend
server); `frontend/tests/auth-recovery.spec.js` (6 tests, mocked
`/auth/*`, phone width, virtual authenticator).

## Landing reconciliation (rebase onto main, 2026-10-14/15)

Main moved about 150 commits while slice 1 waited. Three things changed
what landing means.

**The sender is verified.** The slice waited on the owner verifying
`choir.news` as a Resend sending domain. The owner reports it verified,
with sending and receiving enabled (checked 2026-10-15). The staging
proof (slice 2 item 8) is no longer blocked on sender setup.

**The owner's "delete for now" ruling.** The surface audit
([auth-boundary-gaps-from-surface-audit-2026-10-14.md](auth-boundary-gaps-from-surface-audit-2026-10-14.md)
F6 row g) found `POST /auth/recovery/request` to be an unauthenticated
write that mints a token nobody can use, and asked the owner to choose
between deleting it and building the UI. The owner chose "deleted for
now" ([owner-notes-2026-10-14.md](../reports/owner-notes-2026-10-14.md)
§19). That ruling has not been carried out: on `origin/main` at 3a5084fea
both routes are still registered (`cmd/auth/main.go:77-78`), and no branch
deletes them. Slice 1 is the other branch of that choice. It keeps the
route and gives it a destination: the token is mailed to the account's
stored address, a `/recover` page redeems it, and abuse is bounded by the
existing per-email and per-IP limits (unknown addresses write an unusable
row and get no mail). The DoS concern in row g still holds in part:
an unauthenticated caller can still write rate-limited rows, and there is
no service-wide send cap (residual below, slice 2 item 7). **On landing,
slice 1 supersedes the deletion ruling. Do not also land a deletion of
`/auth/recovery/*`.** If the owner prefers deletion after all, revert the
slice commits and delete the routes; the two outcomes are exclusive.

**Host-internal credential.** Main added `internal/hostauth` (44656ef90):
maild's internal routes require loopback *and* the host-internal token
(observe on Node B now, enforce after a clean day). Slice 1's maild route
checked loopback and `X-Internal-Caller` only, and auth's mailer sent only
the marker. Without a fix, recovery mail would stop the day hostauth
enforces. The rebase adds:

- maild `HandleAuthRecoveryLink` calls `hostauth.Authorize` after the
  transport check, as the other maild internal routes do;
- auth's mailer calls `hostauth.MarkInternal`, which attaches the token
  only to a loopback internal-service port (maild 8087);
- `nix/node-b.nix`: `go-choir-auth` gets `LoadCredential =
  internalTokenCredential`, `CHOIR_INTERNAL_AUTH_MODE`, and an explicit
  `AUTH_MAILD_URL=http://127.0.0.1:8087`;
- tests: maild refuses the route in enforce with no or a wrong
  credential; auth's mailer, run against a real maild server in enforce,
  is refused when the credential is withheld and delivers when it is
  attached.

The proxy's reserved-header stripping (fe21ef2b2) does not touch this
path: Caddy sends `/auth/*` straight to auth on 8081, and `/recover` is a
signed-out GET that the proxy answers with the host platform shell.

Residual from the routing: a browser that is still signed in gets `/recover`
from its own computer's frontend, which carries the recovery page only
after that computer's next base-image refresh. A person who has lost
every passkey is normally signed out, so this affects only a person who
opens the link on a device that still has a session.

### Staging proof (after the deploy reports the landed SHA)

Uses one of the owner's three recovery requests per hour. No schema
migration: the token table and routes are unchanged.

```sh
# 1. identity: auth and maild report the landed commit
ssh node-b 'for p in 8081 8087; do curl -fsS http://127.0.0.1:$p/health | jq -c "{service, commit:.build.commit}"; done'
# 2. a real account and an unknown address get identical answers (200 {"ok":true})
for e in "$OWNER_EMAIL" "nobody-$(date +%s)@example.com"; do
  curl -sS -w ' %{http_code}\n' -X POST https://choir.news/auth/recovery/request \
    -H 'Content-Type: application/json' -d "{\"email\":\"$e\"}"
done
# 3. exactly one send, for the real account only; no token in either journal;
#    no hostauth "would refuse" line for the recovery route
ssh node-b 'journalctl -u go-choir-auth -u go-choir-maild --since -5min \
  | grep -E "operation=(recovery_mail|auth_recovery_mail|recovery_request)|would refuse.*recovery-link"; \
  journalctl -u go-choir-auth -u go-choir-maild --since -5min | grep -c choir_rt_'
#    expect: auth result=sent once, maild result=sent provider_message_id=<id> once,
#    user_not_found for the unknown address, zero "would refuse", count 0
# 4. on a phone, signed out: open the email, tap "Add a new passkey", create
#    the passkey, land signed in to the same account (same computer, same documents).
# 5. open the same link again: "used or expired", with an offer of a new link.
```

Steps 4 and 5 are the owner's acceptance; steps 1 to 3 need only the
shell. A deployed Playwright spec cannot do step 4 without a real inbox.

## Slice 2 needs (magic-link sign-in, several passkeys, visible sessions)

1. **Magic-link sign-in as a login method.** A second token purpose
   (`sign_in`) beside `recovery`, or a `purpose` column on
   `recovery_tokens`, so a sign-in link can never be replayed as a
   recovery link or the reverse. Verify issues a session directly
   (`issueSession`) instead of a registration challenge, and should mark
   the session as email-authenticated (weaker than a passkey) so
   sensitive actions (adding or deleting passkeys, API keys, ROOT unwrap)
   can require a passkey or a fresh link. Decide whether a sign-in link
   must be opened in the browser that asked for it (bind a short-lived
   cookie at request time) to stop a forwarded link from signing in a
   stranger.
2. **Email ownership at sign-up.** Registration today never proves the
   email address. Magic-link sign-in and recovery both trust it, so
   sign-up should verify the address (or the first sign-in link proves
   it) before recovery is offered for that account.
3. **Several passkeys per account.** The backend has list, rename and
   delete (`/auth/credentials`), with a last-credential guard. Missing:
   an "add a passkey" begin route for a signed-in user, the Settings UI,
   and naming a new passkey from its authenticator (AAGUID) and device.
4. **Visible, revocable sessions.** `/auth/sessions` list and revoke
   exist. Missing: the Settings UI, readable device labels from the user
   agent, "sign out everywhere else", and revoking other sessions when a
   recovery link is used (a recovered account may have a thief's session
   alive).
5. **Owner ROOT after recovery.** Decide the recovery protector for
   PRF-wrapped data, or show plainly that recovered sign-in does not
   restore encrypted data; request PRF on the recovery registration once
   a re-wrap path exists.
6. **Email canonicalization** (case) at registration and lookup.
7. **A service-wide send cap** in maild for auth mail.
8. **Staging proof** with a real inbox (or a Resend test address) and a
   deployed Playwright run, after the owner confirms the sender setup.

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
  owner-root wraps not recoverable (residual); registration never proves
  email ownership (slice 2). Introduced: none intended. Repaired on
  landing: the discarded token. Hardened: single-use consume no longer
  relies on the one-connection pool; older tokens are superseded.
