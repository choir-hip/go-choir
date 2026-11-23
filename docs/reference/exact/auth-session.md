# Exact: the system sign-in sheet (`openAuthSession`) and how Autoradio uses it

Part of [EXACT.md](../EXACT.md). Read against the pinned checkout `f53347503` (`mobile/exact.pin`). Marks as in [gaps.md](gaps.md):
**[doc]** read in Exact's docs, **[src]** read in its source, **[unrun]** not run by us. Paths without a prefix are under `mobile/.exact-src/`.
Written 2026-11-23 with the sign-in sheet change (owner, 2026-11-23: "A sign-in sheet that slides over the app, passkey sheet in app later").

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

## How Autoradio uses it

Only the opening of the approval page changed; the device flow, the key and its Keychain item did not (`docs/design/account-boundary-2026-11-21.md`).

```
shared code (account.contract)           Swift (ChoirSignIn, unchanged but for dropping its browser call)       server
 signIn      -> choirSignIn()  ------------> POST /auth/device/begin {label}  ---------------------------------> codes
 au.status=="pending" and a link  <--------- state: userCode, verificationURL (device_code stays here)
 task openSheet -> signInSheet(link)
   openAuthSession(link + &return=<scheme>&state=<nonce>, {callback, state})   (signin-sheet.ts)
   [the sheet shows /device; the owner signs in with the browser's passkey and approves]
   page POSTs /auth/device/approve {user_code, decision, return, state} ----------------------------------------> {status, label, return_to}
   page navigates to return_to = <scheme>:/signed-in?state=<nonce>   -> the sheet closes by itself
 outcome returned / cancelled / failed
                                          poll POST /auth/device/token {device_code} every 5 s  ----------------> the key, once
                                          Keychain write (same service and accounts as before)
```

- **The callback is a hint.** It carries our own nonce and nothing else: no key, no device code, no user code, no account id. `signin-sheet.ts` never reads it; it reports `returned`, `cancelled` or
  `failed`. The key arrives only by the native poll to `/auth/device/token`, which still needs the device code that only Swift holds and an approval by a passkey session (an email session is refused, `device_login.go`).
  Anyone who can trigger the scheme gains nothing; a callback with no approval leaves the poll asking and the account signed out (`signin-sheet.test.ts` S6).
- **Closing the sheet is a hint too (review, 2026-11-23).** The owner closing it is a 499, and a 499 can also follow an approval whose callback was not taken (the page shows "Approved"
  and the owner swipes it away). So nothing is abandoned on the close itself: the contract asks native `choirCancelSignIn(by="sheet")`, and Swift's running poll answers first. A key is
  taken; "still pending" ends the sign-in (back to signed out, no message; the code expires on the server as it always did); `slow_down` and network errors decide nothing (the server
  answers `slow_down` to a poll inside the interval even when the code is approved, so a second, immediate poll would prove nothing). The card's own Cancel (`by="card"`) abandons at once.
  Only the pending sign-in's own result counts: the result carries the verification link it was opened for, and the contract compares it with the pending one; a new sign-in clears the last result.
  The ten-minute bound is the same 499.
- **A sheet that cannot open is not a dead end.** On 501 (no browser), 502 or a throw the card says so and offers the same link in the system browser with Exact's `href`
  ("One to an absolute URL (`https://…`) leaves the app: natively it opens in the system browser", `docs/contract-grammar.md:1684-1686`) [doc]. The page then opens without `return` and `state`, as
  any browser visit does, and the native poll collects the key as before. No Swift opens a browser any more. [unrun on a device]
- **Scheme per app id.** `app.ts` reads `const SIGNIN_SCHEME = import.meta.env.SIGNIN_SCHEME ?? 'news.choir.autoradio.lab'` and grants `auth.callback ${SIGNIN_SCHEME}:/signed-in`. The build reads only `?? 'default'` (an `||` is refused: "import.meta.env is read only in a top-level const ..."), and `??` does not catch an empty value (an exported empty `SIGNIN_SCHEME` gives the grant `:/signed-in`, a dead sign-in) [run], so `mobile/scripts/exact.sh` passes `SIGNIN_SCHEME=<app-identity.sh id>` for every identity under `env -i` (tested in `test-app-identity.sh`) and the literal is only the fallback for a bare `bun exact.mjs`. `app.json` declares the
  setting (`"env": ["SIGNIN_SCHEME"]`) and the variant `one` gives it `news.choir.autoradio.lab.one` (the reference's `variants.<name>.env`; `contract-for-agents`/schema `app.schema.json:1566-1627`). The scheme is the
  app id, so two installs do not share one; `signin-sheet.test.ts` S8 and the Go test `TestDeviceReturnSchemesAreTheAppsInAppJSON` keep app.json, app.ts and the server's list together. A web test run with
  `--variant one` printed `import.meta.env.SIGNIN_SCHEME = "news.choir.autoradio.lab.one" (variants.one)` [run]. That the native bake carries the grant with the variant's value is **[unrun]** (it is the lead's device build).
- **Server.** `POST /auth/device/approve` takes an optional `return` and `state`. `return` is an identifier from a fixed list in server code (`DeviceReturnSchemes`: the app ids), never a URL; `state` must be 16 to
  128 of `A-Za-z0-9_-`; anything else is **not an error**: the decision is recorded, the key is collected by the poll, and `return_to` is left out (the sheet stays open, the page says "Close this window to return to the
  app"). A hint never refuses an approval and answers nothing about a code (an unknown code is the same 404 with or without it). `return_to` exists only after a decision, only from the list, and never echoes
  the client's values. The page follows `return_to` only if it has exactly the shape `<the scheme it sent>:/signed-in?state=<the state it sent>` (`frontend/src/lib/device-return.js`). Without `return` the page and the response are as they were.
- **Why the begin and poll are still Swift.** The device code and the key must not pass through shared code, and the key's Keychain item is written by native code (account-boundary design). That, and only that, is
  why `ChoirSignIn` stays; it lost its `openURL` and the native `choirOpenApproval` call, and the native boundary ceiling fell (lines 6224 to 6221, names 30 to 29).
- **Logs** (shared code, ids only): `signin: sheet opened`, `signin: sheet returned`, `signin: sheet cancelled`, `signin: sheet failed status=<n>` (or `(no callback)`, `(not our link)`, `(no session)`).

## Not done here, and why

- A native passkey sheet inside the app (`@exact/passkeys`) needs the Associated Domains entitlement, an AASA file on choir.news, and a new key-minting route (red); the repo's plan doc says a free team cannot sign Associated
  Domains and nothing here has tested that. This change needs none of it. See the investigation of 2026-11-22.
- Android: the same shared code runs, but Android has no sign-in module today (no native key store), so there is nothing for the poll to write; **[unrun]**.
