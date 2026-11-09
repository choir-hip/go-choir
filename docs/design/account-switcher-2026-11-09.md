# Account switcher: several signed-in accounts in one browser (2026-11-09)

Owner, 2026-11-09: "There should be an easy way from the desk menu to switch
accounts. All the accounts that I own, I should be able to switch between them
in the GUI. I should be able to sign in to all of them and have them linked.
That's a standard feature across existing apps. People understand that in Gmail
or Instagram."

Class: red (auth and session renewal). Rollback: revert; the account set is
additive, and with it gone the browser keeps today's single session.

## Today (investigation, file:line in the ledger entry of this date)

One cookie pair per origin: `choir_access` (Ed25519 JWT, Path=/, 5 min) and
`choir_refresh` (Path=/auth, 30 days, 7 for email sessions), backed by a
server refresh-session row that rotates with a grace table
(internal/auth/handlers.go:345-476). A second sign-in overwrites the first.
The proxy derives the user only from the validated JWT or a `choir_sk_`
Bearer key, strips client `X-Proxy-*`/`X-Choir-*` headers, and routes user ->
primary desktop -> computer (internal/proxy/handlers.go:494-645, 1208). Each
user has one computer, so switching account switches computer.

## Design

**The account set.** A browser holds one more cookie, `choir_accounts`
(HttpOnly, Secure, SameSite=Lax, Path=/auth): an opaque random handle to a
server row `account_set(id, created_at)` with members
`account_set_member(set_id, user_id, refresh_session_id, added_at)`. The
cookie names a set; it carries no user ids and no tokens. Membership is only
ever written by the auth service at the end of a successful full sign-in
(passkey or email), never from a request body.

**Sign-in adds.** Every successful sign-in (register finish, login finish,
email verify, recovery finish) creates the set if the browser has none and
upserts (set, user, the new refresh session). The signed-in account becomes
active (today's cookie pair is set as now). Signing in to an account already
in the set replaces its member session.

**Switch.** `POST /auth/accounts/switch {user_id}`, cookie-only (API keys
refused). The server reads the set from `choir_accounts`, finds the member for
`user_id`, checks its refresh-session row is live (not revoked, not expired),
rotates that session (new refresh token, the grace table as today) and sets
`choir_access`/`choir_refresh` for that user. The previously active account's
session stays a member: it is not revoked, it simply stops being the cookie
pair. A member whose session is dead is shown as "signed out" and switching to
it opens sign-in for that account (prefilled email). The identity is always
server-derived: the body names which member, the server proves the browser
holds that member.

**List.** `GET /auth/accounts` (cookie-only): `[{user_id, email, display_name,
active, signed_in}]` for the set, active first. No member is listed to a
browser that does not hold the set cookie.

**Sign out.** `POST /auth/logout` keeps its meaning for the active account and
also removes it from the set; if another member is signed in, the response
names it and the frontend switches to it. `POST /auth/logout?all=1` revokes
every member session, deletes the set and clears all three cookies.

**Renewal.** Unchanged for the active account (`/auth/session` rotation).
Inactive members are not renewed in the background; a member session simply
lives to its own expiry, and the switch rotates it. No background request may
overwrite the active cookie pair.

**Frontend.** The desk menu (the start/launcher menu and the account chip)
gains an Accounts section: each account with its email, the active one
marked, "Add another account", "Sign out of <email>", "Sign out of all
accounts". Switch: flush desktop state (as handleLogout does), close the live
channel, POST switch, then a full page reload (the simplest reset of every
per-user cache, socket and store). "Add another account" opens the sign-in
overlay in add mode (the current session stays a member). The active account
is always visible (the chip), and a refused switch is a visible message
(docs/problems/proof-session-silently-replaced-by-new-account-2026-10-22.md).
Global localStorage keys that hold per-user state (the theme boot cache,
`choir.auth.returning`, Texture drafts, reader prefs) are keyed by user id.

**Out of scope, named:** linking across devices (the set is per browser, like
Gmail's), the native desktop bridge (`/desktop-auth/*` keeps one session),
per-account notifications from inactive accounts.

## Failure modes (tests first)

1. A switch to a user not in the browser's set is refused (403), whatever the
   body says; a request with no set cookie is refused.
2. A switch with an API key is refused.
3. A dead member session (revoked, expired) is refused with a reason and the
   UI offers sign-in for that account.
4. After a switch, `/auth/session` and the proxy resolve the new user and its
   computer; the previous account's refresh session is still live.
5. Rotation grace still holds for the switched-to session (a concurrent old
   token within grace is accepted, as today).
6. Sign out of the active account removes only that member; sign out of all
   revokes every member and clears cookies.
7. A second sign-in to the same account replaces its member, never
   duplicates it.
8. The set cookie leaks no user id or token (opaque handle only).
9. Frontend: switching shows the other account's desktop after reload; the
   active account is always shown; theme boot cache does not bleed between
   accounts.
