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
primary desktop -> computer (internal/proxy/handlers.go:494-645, 1208; header strip lists at :44-94). Each
user has one computer, so switching account switches computer.

## Design

**The account set.** A browser holds one more cookie, `choir_accounts`
(HttpOnly, Secure, SameSite=Lax, Path=/auth, persistent: Max-Age 30 days,
refreshed on every switch or sign-in). It is an opaque 256-bit random handle;
the server stores only its SHA-256 (`handle_hash`), as refresh tokens are
stored (store.go:66). Rows: `account_set(id, handle_hash, active_user_id,
created_at, last_used_at)` and `account_set_member(set_id, user_id,
refresh_session_id, added_at)` with `UNIQUE(set_id, user_id)`. The cookie
carries no user ids and no tokens. Membership is written only by the auth
service at the end of a successful full sign-in, never from a request body.
The set has no `display_name`: the list shows `users.email` (store.go:29-33
has only id, email, created_at).

**Sign-in adds.** The four sign-in endpoints (register finish, login finish,
email verify, recovery finish) call `addToAccountSet` after they issue the
session; the shared issuer does not, because `HandleDesktopRedeem` also calls
`issueSessionExpiring` (handlers.go:1616) and would mint orphan sets. It
get-or-creates the set in one transaction (a concurrent first sign-in cannot
leave an orphan), upserts (set, user, new refresh session) on the unique key
and sets `active_user_id` to the signed-in user. Signing in to an account
already in the set replaces its member session and deletes the old session row.

**Member sessions survive rotation (B1).** `rotateRefreshSession`
(handlers.go:476-506) creates a new session row and deletes the old one, so a
member's `refresh_session_id` would dangle at the first renewal (every 5
minutes). Rotation therefore goes through one store transaction,
`RotateRefreshSession(old, new)`: insert the new row (setting `rotated_from` =
old id, store.go:69, which is never populated today), `UPDATE
account_set_member SET refresh_session_id = new WHERE refresh_session_id =
old`, delete the old row. Every rotation path calls it: renewal
(`tryRefreshRotation`, handlers.go:1266-1329, via `rotateRefreshSession`) and
the switch. The grace-window path (handlers.go:1275) hands out the already
rotated session, so the member row already points at it; it needs no repoint
but passes the same gate below. A session with no member row (no set cookie,
CLI-free browser sessions of today) rotates exactly as now.

**One active member gates renewal (B2).** `active_user_id` is the single
statement of which member owns the cookie pair; the list's `active` flag is
derived from it. Switch and sign-in set it in the same transaction that
rotates the session. `/auth/session` and `tryRefreshRotation`, when the
request carries `choir_accounts` and the presented refresh session (or its
grace entry) is a member of that set whose user is not `active_user_id`,
refuse: they write no cookie at all, and return 200
`{authenticated:false, reason:"account_switched", active_user_id}`. This is
checked before the grace lookup, so the grace path cannot reverse a switch
either. A request with no set cookie, or a session that is not a member, is
untouched. A stale tab (or a late `fetchWithRenewal` retry carrying the old
account's refresh cookie) therefore gets a clean "account switched" result and
cannot flip the cookie jar back. The frontend treats that reason as "reload",
not as a logout. Sibling tabs are told at once: the switching tab posts
`{type:"account-switched", user_id}` on a `BroadcastChannel("choir-accounts")`
and writes a `choir.accounts.switched` localStorage key as the fallback (the
`storage` event fires in the other tabs). Each receiving tab flushes desktop
state (as handleLogout does), closes its live channel and reloads, so no tab
keeps account A's in-memory state while its calls run under B. The `reason`
result above is the backstop when the broadcast is missed.

**Switch.** `POST /auth/accounts/switch {user_id}`, cookie-only (API keys
refused). The server resolves the set from `choir_accounts`, finds the member,
checks its refresh-session row is live (not revoked, not expired), then in one
transaction rotates that session (via `RotateRefreshSession`, the grace table
as today) and sets `active_user_id`; it then sets `choir_access` and
`choir_refresh` for that user. A switch to the already-active user is an
idempotent no-op (200, no rotation). The previously active account's session
stays a member and live; it simply stops being the cookie pair. A member whose
session is dead is shown as "signed out" and switching to it opens sign-in for
that account (prefilled email). The identity is always server-derived: the body
names which member, the server proves the browser holds that member.

**List.** `GET /auth/accounts` (cookie-only): `[{user_id, email, active,
signed_in}]` for the set, active first. Nothing is listed to a browser without
the set cookie.

**Sign out (B3, B4).** `POST /auth/logout` and its `?all=true` /
`?all_devices=true` (revoke this user's sessions on all devices, passkey-gated
for email sessions, handlers.go:1346-1351) are unchanged in meaning. The one
addition: when the signed-out session is a member, it is also removed from the
set, and if another member is signed in the response names it as
`next_user_id` and the frontend switches to it (the switch route is the only
thing that sets cookies for it). The set-wide and per-member actions are new
routes with new names, so no query parameter is overloaded:

- `POST /auth/accounts/signout {user_id}`: deletes that member and its refresh
  session row (and its grace entry). For an inactive member nothing else
  changes and no cookie is written. For the active member it runs today's
  logout for that session, then, if another live member exists, returns
  `next_user_id` (and sets `active_user_id` to it, leaving the cookie switch to
  the follow-up `switch` call); if none, it is the empty-set case.
- `POST /auth/accounts/logout-all`: revokes every member session of the set
  (this browser's accounts only; no `DeleteRefreshSessionsByUserID`, other
  devices of those users stay signed in), deletes the set, and clears all three
  cookies. It is browser-scope, so the passkey gate does not apply (an email
  session may sign out of its own browser).
- Empty set: when the last member is removed the set row is deleted and
  `choir_accounts` is cleared; the browser is signed out exactly as today.
  Signing out a `user_id` that is not in the browser's set is 403.

**Hygiene.** A sweep next to `CleanExpiredRefreshSessions` (store.go:907)
deletes members whose session row is gone or expired (a member whose session
is dead is also deleted when it is listed past its expiry, so "signed out"
shows once and then goes) and sets with no members or `last_used_at` older
than 30 days.

**CSRF posture.** The new POSTs match the existing auth routes: SameSite=Lax
host-only cookies, method gate (POST only), and `requireJSONContentType`
(email_signin.go:212, used at device_login.go:354 and email_signin.go:267) on
every JSON body. There is no Origin check anywhere in `/auth/*` today and none
is added. `GET /auth/accounts` is read-only. `/auth/*` is routed wholesale to
the auth service (node-b.nix:724-725), so no edge change.

**Renewal.** `/auth/session` rotation is unchanged for the active account.
Inactive members are not renewed in the background; a member session lives to
its own expiry and the switch rotates it. The gate above is what enforces that
no background request overwrites the active cookie pair.

**Frontend.** The desk menu (the start/launcher menu and the account chip)
gains an Accounts section: each account with its email, the active one
marked, "Add another account", "Sign out of <email>" (`accounts/signout`),
"Sign out of all accounts" (`accounts/logout-all`). Switch: flush desktop
state (as handleLogout does, Desktop.svelte:1283), close the live channel,
POST switch, broadcast, then a full page reload (the simplest reset of every
per-user cache, socket and store). "Add another account" opens the sign-in
overlay in add mode (the current session stays a member). The active account
is always visible (the chip), and a refused switch is a visible message
(docs/problems/proof-session-silently-replaced-by-new-account-2026-10-22.md).
Global browser storage that holds per-user state is keyed by user id: the
theme boot cache, `choir.auth.returning`, Texture drafts, reader prefs, the
Autopaper section keys (`SECTION_STORAGE_KEY`, `SECTION_VISITS_STORAGE_KEY`,
autopaper.ts:385,405) and the sessionStorage surface-handoff graph
(computer-surface-handoff.js:46-54). Before the session is known the theme
boot reads a `choir.auth.lastUserId` pointer, written at every successful
session check, to pick the right per-user cache.

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
6. Sign out of the active account removes only that member; `logout-all`
   revokes every member and clears cookies.
7. A second sign-in to the same account replaces its member, never
   duplicates it (`UNIQUE(set_id, user_id)`); two concurrent first sign-ins
   produce one set.
8. The set cookie leaks no user id or token (opaque handle), and the database
   holds only its hash.
9. Frontend: switching shows the other account's desktop after reload; the
   active account is always shown; theme boot cache does not bleed between
   accounts.
10. Renew the active account three times, then list: every member still shows
    signed in (the member row followed each rotation); a racing double switch
    to one member rotates once.
11. Stale tab: after a switch A to B, a renewal carrying A's refresh cookie
    (including inside the grace window) gets `account_switched`, writes no
    cookie, and a following request still resolves B.
12. A sibling tab receives the switch broadcast, flushes and reloads, and its
    next call runs under B; with the broadcast dropped, the `account_switched`
    result does the same.
13. `?all=true` / `?all_devices=true` on logout behave exactly as before
    (passkey gate, all devices of that user); `logout-all` never touches
    another device's sessions.
14. `accounts/signout` for an inactive member removes it without a cookie
    write; for the active member it names the next member; for the last
    member it empties the set and clears the cookie; for a non-member it is 403.
15. A new POST without a JSON content type is refused.
16. A `HandleDesktopRedeem` session creates no set. Switch to the
    already-active user is a no-op. The hygiene sweep removes dead members and
    empty or 30-day-idle sets.

## Adjudicated review findings (Devin, r0)

| Finding | Verdict | Reason |
| --- | --- | --- |
| B1 member row dangles on rotation | adopted | repoint inside `RotateRefreshSession`; `UNIQUE(set_id,user_id)` |
| B2 no gate on stale rotation, tabs bleed | adopted | `active_user_id`, gate before grace, `account_switched`, BroadcastChannel + storage event |
| B3 `all` param collision | adopted | new `accounts/logout-all`; `all_devices` unchanged; no passkey gate (browser scope) |
| B4 no inactive-member sign-out | adopted | `accounts/signout {user_id}`; empty-set edge |
| raw `account_set.id` takeover handle | adopted | `handle_hash`, 256-bit |
| no cookie lifetime / GC | adopted | 30-day persistent cookie, sweep |
| `display_name` does not exist | adopted | dropped; email only |
| hook at four sign-in endpoints | adopted | not the shared issuer |
| idempotent set create and no-op switch | adopted | one transaction |
| `requireJSONContentType` on new POSTs | adopted | CSRF posture section |
| localStorage enumeration incomplete | adopted | keys listed, `choir.auth.lastUserId` pointer |
| grace TTL comment (30s) and dead `rotationGraceDuration` | deferred | pre-existing, not part of this change; live default is `AUTH_ROTATION_GRACE_TTL` 15s |
| strip-header citation range | adopted | the claim stands; anchors are proxy/handlers.go:44-94 with call sites :725/875/1051 |
