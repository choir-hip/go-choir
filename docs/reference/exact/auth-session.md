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

Only the opening of the approval page and the owner of the sign-in changed; the device flow's server side, the key and its Keychain item (same service and accounts, written by the same native code) did not
(`docs/design/account-boundary-2026-11-21.md`).

### One sign-in attempt is one state machine, owned in one place (review 2026-11-23, third and fourth rounds)

Three review rounds found the same fault in different places (a stale poll answer abandoning an approved key; a card Cancel followed by a key write; a re-opened sheet still carrying a close flag): the sign-in's
state was spread over a contract status, a result slot, a close flag and a native poll loop, each patched on its own. So an attempt now has an id and **one owner: shared code** (`mobile/AutoradioLab/signin-attempt.ts`,
a pure reducer plus the driver that runs it). Every event names the attempt it belongs to. Native keeps only what must be native, the device code, the one poll request and the Keychain write, behind three calls that
each name an attempt: `signInBegin(attempt)`, `signInPoll(attempt)`, `signInForget(attempt)` (the app module's `later`, handled by `ChoirSignIn`). Native has no poll loop, no timer, no close flag and no attempt
phase of its own. (It still keeps `ChoirAuthState.status` and `message`, only so `ChoirApp` can refresh the stories when a key has just been stored; they say nothing about the attempt.)

```
shared code (signin-attempt.ts)                      native (ChoirSignIn, one attempt at a time)            server
 signInRun: id = new, BEGIN(id) ------------------> signInBegin(id): POST /auth/device/begin ----------> codes   (a begin is serialized: only the LATEST installs its device code)
   BEGUN(userCode, link, interval, expires) <------ {userCode, link, interval, expires}   (device_code stays native)
   open the sheet: openAuthSession(link + return + state)      (signin-sheet.ts; the callback is a hint, never read)
   [the owner signs in with the browser's passkey and approves; the page navigates to the callback]
   every interval: ISSUED(n), signInPoll(id) ------> POST /auth/device/token {device_code} -----------> pending | key | ...
     on the key: stored only if `id` is still the live attempt (checked under the lock), else revoked -> "approved" / "stale"
   card Cancel / abandon: signInForget(id) --------> drops the code; a key this attempt wrote is deleted and revoked
```

**Publishing.** Every accepted change of the attempt (a new attempt, each transition, the last one of an attempt included) goes through one function, `publish`, which bumps the view's `rev` and tells `env.publish`; an event the
table ignores publishes nothing, and the poll counter (`ISSUED`) is bookkeeping, not a change of the view. The contract's `siv` resource is keyed on every press's answer, which carries the `rev` the press left behind, and each press
asks the view once more when it settles (`then bumpSiv`); a half-second tick covers the changes in between. So the card reads the attempt's LAST phase as surely as its first.

Transition table (`phase` x event -> phase). `n` is the poll's number (polls are issued one at a time); `closedAt` is the number of polls issued when the machine learns the sheet has closed (a poll issued between the
physical close and that moment counts as before the close: conservative by one interval). Any event whose id is not the attempt's, any event after a terminal phase, and any event that does not fit the phase (for
example a sheet result while `begun`, BEGUN while `sheet`, REOPEN while `begun`) is ignored: it changes nothing and publishes nothing.

| phase | event | next phase | what else |
|---|---|---|---|
| (none) | BEGIN(id) | begun | |
| terminal | BEGIN(id) | begun | the ended attempt is NOT forgotten (an approved one owns the key) |
| begun, sheet, closed | BEGIN(id) (a double tap) | begun (the new attempt) | **the newest wins**: the older attempt is cancelled and `signInForget` is sent for it; its late begin answer is ignored; native installs only the latest begin |
| begun | BEGUN | sheet | the sheet opens |
| begun | BEGIN_FAILED | failed | the message says why |
| sheet | SHEET returned | sheet | nothing: a hint; the poll that is running learns the key |
| sheet | SHEET failed (501, 502, a throw) | sheet | a note; the same link is offered in the browser (if the view validates it); the poll goes on |
| sheet | SHEET cancelled (the owner closed it) | closed | `closedAt = polls issued` |
| closed | REOPEN (the owner taps "Open the approval page") | sheet | the close is forgotten (`closedAt` cleared); not while this attempt's sheet is still up |
| sheet | REOPEN | sheet | the sheet opens again (not while it is still up) |
| closed | ANSWER pending, `n > closedAt` | cancelled | a poll that started after the close says nobody approved: the owner walked away; `signInForget` |
| closed | ANSWER pending, `n <= closedAt` | closed | the poll started before the close: it may predate an approval, it decides nothing |
| sheet, closed | ANSWER slow_down / error | same | the server answers `slow_down` to a poll inside the interval even when the code is approved: it decides nothing |
| sheet, closed | ANSWER approved | approved | the key is stored (native wrote it under the attempt's lock) |
| sheet, closed | ANSWER denied | denied | "the sign-in was denied" |
| sheet, closed | ANSWER expired, or TIMEOUT | expired | "the code expired; sign in again" |
| sheet, closed | ANSWER stale / invalid | failed | native does not know the attempt, or the key could not be stored: "sign in again" |
| begun | CANCEL, then native's begin answers late | cancelled | no sheet opens; `signInForget` is sent again once the begin has answered, so the code it installed does not stay live |
| begun, sheet, closed | CANCEL (the card's Cancel) | cancelled | the machine moves FIRST, so any later answer is ignored; then `signInForget`: a key that native had already written in that gap is deleted and revoked |
| ended phases | anything | same | ignored, nothing published |

**What is not promised.** The card's Cancel cannot make "no key was ever written" true in the instant before the machine hears it; what is promised is that **no key remains after Cancel** (written in the gap, it is deleted and
revoked; arriving later, it is never written and is revoked). A code approved from another browser AFTER the owner closed the sheet and walked away is abandoned by a poll that starts after the close and still says pending:
that is a missed sign-in (no key is minted for it), not a leak; the owner signs in again.

### A minted key is either stored and belongs to the live attempt, or revoked

The server mints the key inside the poll that approves it, before the answer reaches the phone. From the moment native holds the answer, every path ends in one of the two:

| path | what native does |
|---|---|
| the attempt is live | stores it (under the lock `forget` takes), records `wrote = attempt`, answers "approved" |
| the Keychain refuses to store it | revokes it at once, ends the attempt (answers "invalid") |
| the attempt was forgotten (the owner's Cancel, a denial, an expiry) while the answer was on its way | does not store it; revokes it |
| a newer begin replaced the attempt | does not store it; revokes it |
| sign-out ran while the answer was on its way | does not store it; revokes it |
| the attempt is forgotten after the key was stored | deletes the three Keychain items and revokes it |
| the process stops between the server's commit and the answer (the answer is lost) | **nothing holds the key**: the phone never saw it, the server's row is consumed, so the next poll answers `invalid_grant` and the key stays live on the server until the owner revokes it in Settings (its label is `choir login: <device label>`). No code can close this without a server-side change (for example, a key delivered only with a client acknowledgement); it is recorded here, not fixed |
| the answer carries no `key_id` (the server always sends one) and the key is not stored | nothing can be revoked by id: recorded as the same limit as the lost answer |
| a revoke itself fails (offline) | the revoke is best effort and held nowhere: native keeps no queue of keys to revoke, and **no later revoke exists today**; sign-out has the same documented limit. The key stays live on the server until the owner revokes it in Settings. It is unusable by anyone who does not hold the plaintext, which native has just dropped |

There is no `await` between native receiving a key and the guarded write, so cancellation of the task cannot land between the two.

**The two limits that remain, and the fourth review's judgement of them (2026-11-23).**

- **A revoke that fails (offline).** *Already on main:* sign-out's revoke is best effort and drops the plaintext key (`ChoirSignIn.signOut`), so an offline sign-out leaves the same live key. *What this change adds:* more places that revoke (a Cancel in the gap, a Keychain refusal, a superseded attempt, a sign-out race), each rare
  (it takes a minted key, a non-store and a failed DELETE together); and it closes one hole main had, a refused Keychain write that left the key live without even trying to revoke. The key is unusable by anyone without the plaintext the app dropped.
  *Likelihood:* an offline sign-out is ordinary, the new paths are rare. *Judged:* acceptable, documented. *The fix* is a durable client-side queue of key ids to revoke (a server cannot know the client's intent for a key it delivered); not built.
- **An answer lost between the server's commit and native (the process dies, the connection drops).** *Already on main:* the same mint-inside-the-token-exchange and the same response path (`device_login_store.go`, `PollDeviceAuthorization`), so a kill in that window left the same live key. *New:* nothing,
  except that a JS reload now loses the machine while native still holds a device code (an orphan code that mints no key). *Likelihood:* rare (a sub-second window, or a drop after the commit). *Judged:* acceptable, documented. *The server-side fix* is two-phase delivery: the token answer carries a handle, a second
  "claim" call releases the key, and the device-flow sweeper (`StartDeviceSweeper`, which today only deletes old rows) revokes a minted key never claimed or used within a short TTL. Not built; it is a red change to the key mint path.
- **A superseded code approved anyway.** After a newer sign-in replaced an attempt, approving the OLD code (for example in the browser fallback tab) mints nothing: approval records a decision, and the key is minted only by the token poll, which nothing runs for a forgotten code. It is a missed sign-in; the owner sees the newer attempt's code and no message about the old one.
- **Follow-ups, not in this change:** the Keychain write of the key's id uses `try?`, so if only that write fails the key is stored but cannot be revoked by sign-out; and `ChoirAuthState.userCode` / `verificationURL` are now unused fields of the `ChoirAuth` shape.

### What the callback and the close mean

- **The callback is a hint.** It carries our own nonce and nothing else: no key, no device code, no user code, no account id. `signin-sheet.ts` never reads it; it reports `returned`, `cancelled` or
  `failed`. The key arrives only by the native poll to `/auth/device/token`, which still needs the device code that only Swift holds and an approval by a passkey session (an email session is refused, `device_login.go`).
  Anyone who can trigger the scheme gains nothing; a callback with no approval leaves the poll asking and the account signed out (`signin-sheet.test.ts` S6).
- **Closing the sheet is a hint too.** The owner closing it is a 499, which can also follow an approval whose callback was not taken. So the close alone abandons nothing (table: `closed`): only a poll
  that started after the close and still answers "pending" ends the sign-in (back to signed out, no message; the code expires on the server as it always did). The ten-minute bound is the same 499.
- **A sheet that cannot open is not a dead end.** On 501 (no browser), 502 or a throw the card says so and offers the same link in the system browser with Exact's `href`
  ("One to an absolute URL (`https://…`) leaves the app: natively it opens in the system browser", `docs/contract-grammar.md:1684-1686`) [doc], shown only if it is https on the granted origin
  (`signin-attempt.ts` `browserLink`). The page then opens without `return` and `state`, as any browser visit does, and the poll collects the key as before. [unrun on a device]
- **Scheme per app id.** `app.ts` reads `const SIGNIN_SCHEME = import.meta.env.SIGNIN_SCHEME ?? 'news.choir.autoradio'` (the production id; since 2026-11-23 the lab apps are the variants `lab` and `lab-one`, each with its own `env.SIGNIN_SCHEME`) and grants `auth.callback ${SIGNIN_SCHEME}:/signed-in`. The build reads only `?? 'default'` (an `||` is refused: "import.meta.env is read only in a top-level const ..."), and `??` does not catch an empty value (an exported empty `SIGNIN_SCHEME` gives the grant `:/signed-in`, a dead sign-in) [run], so `mobile/scripts/exact.sh` passes `SIGNIN_SCHEME=<app-identity.sh id>` for every identity under `env -i` (tested in `test-app-identity.sh`) and the literal is only the fallback for a bare `bun exact.mjs`. `app.json` declares the
  setting (`"env": ["SIGNIN_SCHEME"]`) and the variant `lab-one` gives it `news.choir.autoradio.lab.one` (the reference's `variants.<name>.env`; `contract-for-agents`/schema `app.schema.json:1566-1627`). The scheme is the
  app id, so two installs do not share one; `signin-sheet.test.ts` S8 and the Go test `TestDeviceReturnSchemesAreTheAppsInAppJSON` keep app.json, app.ts and the server's list together. A web test run with
  `--variant one` printed `import.meta.env.SIGNIN_SCHEME = "news.choir.autoradio.lab.one" (variants.one)` [run]. That the native bake carries the grant with the variant's value is **[unrun]** (it is the lead's device build).
- **Server.** `POST /auth/device/approve` takes an optional `return` and `state`. `return` is an identifier from a fixed list in server code (`DeviceReturnSchemes`: the app ids), never a URL; `state` must be 16 to
  128 of `A-Za-z0-9_-`; anything else is **not an error**: the decision is recorded, the key is collected by the poll, and `return_to` is left out (the sheet stays open, the page says "Close this window to return to the
  app"). A hint never refuses an approval and answers nothing about a code (an unknown code is the same 404 with or without it). `return_to` exists only after a decision, only from the list, and never echoes
  the client's values. The page follows `return_to` only if it has exactly the shape `<the scheme it sent>:/signed-in?state=<the state it sent>` (`frontend/src/lib/device-return.js`). Without `return` the page and the response are as they were.
- **Why the begin, the poll request and the key write are still Swift.** The device code and the key must not pass through shared code, and the key's Keychain item is written by native code (account-boundary design).
  That, and only that, is why `ChoirSignIn` stays. It lost its loop, its status, its close flag and its browser call, and the native boundary ceiling fell (the new numbers are in `scripts/autoradio-native-ceiling.txt`, lowered with `--update`, never by hand).
- **Logs** (shared code, ids only, no codes and no links): `signin: sheet opened`, `signin: sheet returned`, `signin: sheet cancelled`, `signin: sheet failed status=<n>` (or `(no callback)`, `(not our link)`, `(no session)`), and one line per attempt transition, `signin: attempt <short id> <phase>`.

## Not done here, and why

- A native passkey sheet inside the app (`@exact/passkeys`) needs the Associated Domains entitlement, an AASA file on choir.news, and a new key-minting route (red); the repo's plan doc says a free team cannot sign Associated
  Domains and nothing here has tested that. This change needs none of it. See the investigation of 2026-11-22.
- Android: the same shared code runs, but Android has no sign-in module today (no native key store), so there is nothing for the poll to write; **[unrun]**.
