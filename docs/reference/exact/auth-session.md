# Exact: the system sign-in sheet (`openAuthSession`) and how Autoradio uses it

Part of [EXACT.md](../EXACT.md). Read against the pinned checkout `f53347503` (`mobile/exact.pin`). Marks as in [gaps.md](gaps.md):
**[doc]** read in Exact's docs, **[src]** read in its source, **[unrun]** not run by us. Paths without a prefix are under `mobile/.exact-src/`.
Written 2026-11-23 for the sign-in sheet; **Autoradio no longer uses it** (same day, the owner: "native passkey account creation and sign in. no 'code'").

## What Exact gives

- `openAuthSession(url, { callback, state, ephemeral?, logout?, message? })` and `authCallback()` are declared for every app (`contract/cli/src/typescript.rs:156-159`;
  `docs/contract-for-agents.md:1763-1770`) [src]. They run in a data source (a Contract mutation's source is one); on the web a source must be placed on main, not in a worker (`bake/src/reach.rs:610-640`) [src].
- Grants (`docs/recipes/oauth.md:13`) [doc]: `auth.session <origin>` (the origin of the page the sheet may open; an origin, no path: `runner/src/auth.rs:338-365`) and
  `auth.callback <scheme>:/<path>` (a private-use scheme, or an https callback that needs a domain association). **A private-use scheme needs no entitlement, no
  associated domain and no provisioning capability** (`bake/src/reach.rs:610-618`: "A private-use scheme needs nothing, on purpose") [src], so it works with a free team. [unrun on a device]
- Apple: `ASWebAuthenticationSession` (`host/apple/Sources/ExactAuthSession/AuthSession.swift:25-58`): a sheet over the app that shares Safari's cookies and passkeys unless `ephemeral: true`
  (`prefersEphemeralWebBrowserSession = ephemeral`) [src]. Android: Custom Tabs / Auth Tab, custom-scheme callbacks supported; `ephemeral: true` answers 501 (`docs/reference.md:3512-3525`) [doc].
- The callback the page navigates to is matched by scheme, host and path, and **must carry exactly one `state`, equal to the one the app passed** (`runner/src/auth.rs:535-575`) [src]. A
  callback with another state, no state, a different path or userinfo is not accepted. The full callback URL is handed back to the source; "keep callback URLs in data code: they can contain credentials" (`contract-for-agents.md:1768`) [doc].
- Rejections carry `error.status` (`js/src/auth.js:60-66`) [src]: 499 cancelled (the person closed the sheet, or the ten-minute bound), 403, 409, 428, 501 (no browser, ephemeral on Android),
  502 (could not start; the host's reason). The simulator's driver opens no panel for an auth session (`contract-for-agents.md:6435`) [doc]: **the sheet is only seen on a device.**
- Both installs of an app share one callback **if the grant is a literal**: "URL schemes, the sign-in callback and the app's own document types are shared by both installs" (`docs/reference.md`, "variants") [doc].

## What Autoradio does with it now: nothing

The app signed in through this sheet for a day (2026-11-23): the device flow's approval page in `openAuthSession`, a private-scheme callback, a state machine in `signin-attempt.ts`, three native calls
(`signInBegin`, `signInPoll`, `signInForget`) and a Keychain write in Swift. All of that is deleted. Sign-in and account creation are now the platform's passkey screen through `@exact/passkeys`, with the
whole flow in shared code and the key kept as an Exact secret: see [passkeys.md](passkeys.md). The grants `auth.session` and `auth.callback` are gone from `app.ts`; the build setting `SIGNIN_SCHEME` became
`APP_ID` (the app's id, named to the server's passkey finish route). The device flow stays on the server for the command-line tool (`choir login`); the app never calls it.

What this page still records for the next use of `openAuthSession` (for example an OAuth provider's page): the facts above, and these three lessons from the sheet's three review rounds (2026-11-23):

- a callback is a hint, never a credential: read nothing from it that you cannot check elsewhere;
- a state machine with one owner, an id on every event and "the newest wins" ended three rounds of the same fault in different places (the passkey attempt in `app-signin.ts` is built the same way);
- a key minted for an attempt that is no longer live must be revoked, never kept.
