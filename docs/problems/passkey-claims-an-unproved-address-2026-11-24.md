# A passkey can claim an address nobody has proved (2026-11-24)

Structural assessment after three review rounds of `lane/native-passkey`
(contract rule 7: three rounds without convergence, stop patching, change the
frame).

## Problem

`POST /auth/register/begin` creates a `users` row for any address typed into
it, and `register/finish` (the web's and the phone app's) gives whoever holds
the first passkey a session or a full-scope bearer key on that row. The
address is bound to an account before anyone has shown they hold the inbox.
Whoever types a stranger's address first owns the account, and every later
mail-proved act of the rightful owner (recovery, email sign-in) lands on an
account a stranger already holds.

## Evidence: three rounds, each about the last round's fix

- **Round 1** (seats 1 and 2): two live challenges for one address could both
  finish, so the second passkey and key landed on an account it did not
  create. Fix: refuse a second finish when the account has a passkey, except
  for recovery.
- **Round 2**: that check was check-then-act and failed open on a read error;
  and an unproved address could be squatted, recovery did not take it back.
  Fix: atomic finish, and a "takeback" (`ChallengeSourceRecoveryTakeback`):
  recovery of an address with `email_verified_at` empty replaces the account's
  passkeys, keys and sessions.
- **Round 3**: the takeback (a) is decided at `recovery/verify`, which also
  sets `email_verified_at` before the passkey exists, so an interrupted
  recovery turns the takeback off for good and leaves a verified account with
  no passkey that a stranger can claim (seat 2 #1, #2); (b) fails open when the
  user read errors (seat 2 #4); (c) cannot revoke everything it must: a login
  finish in flight, live 5-minute access JWTs, approved device
  authorizations, desktop exchange codes, email sign-in attempts, the
  in-memory rotation grace, the data ROOT's PRF wraps and `owner_root_keys`
  (seat 1 #2, seat 2 #3).

## Belief: the frame is the defect

Revocation after the fact has to enumerate every credential an account can
mint and every one it has minted, and that set grows with each feature (every
new grant is another row the takeback must remember). It cannot be closed by
more patches, only by never creating the thing to undo. The invariant that
closes it:

**No account is ever bound to an address that its holder has not proved, in
the same ceremony.** The account row, its first passkey and its
`email_verified_at` come into being in one transaction at `register/finish`,
and `register/begin` for a new address needs a proof of the inbox that is
single-use, short-lived, bound to the address and to the client that asked.
Nothing exists for an address before that. An address that already has an
account is never registered again: a mailed proof for it goes to recovery,
whose mail is itself the proof, and recovery adds a passkey. No unproved
holder can exist, so nothing needs taking back.

## The change

- `POST /auth/register/mail {email, binding}` mails an 8-digit code through the
  existing email-attempt machinery (`email_signin_attempts`, purpose
  `register`, no user row, the code stored as a keyed hash, the client's
  `binding` stored as a hash exactly as the attempt cookie is). For an address
  that already has an account it mails the recovery link instead (the existing
  `recovery/request`) and answers the same.
- `POST /auth/register/begin {email, attempt_id, code, binding}` consumes the
  code (the wrong-try count is one committed update, the code is compared in Go, and the
  consumption is a second conditional update, so single use holds; 5 wrong tries, 10 minutes) and returns creation
  options for a user id minted now; it writes one `pending_registrations` row
  (challenge, address, new user id, session data, 5 minutes) and no user.
- `register/finish` and `app/register/finish` consume the pending row with one
  `DELETE ... RETURNING`, verify the passkey, and then in one transaction insert
  the user (`email_verified_at` = now), the credential and nothing else. The
  address decides a race: a case-folded count inside the transaction (the store runs on one
  connection, so two creations serialize) refuses the second, with the case-sensitive unique
  index as a backstop, so of any number of finishes for one proof exactly one account exists.
- Recovery finish is the only way a passkey is added to an existing account. In
  one transaction it sets `email_verified_at`, and if that statement changed a
  row (the address had never been proved) it first deletes the account's old
  passkeys, keys, sessions and grants, then adds the new passkey.
- An account whose address is not proved mints nothing: web and app login
  finish refuse it (`address_unproved`), no session or key is issued for it,
  and a signed-in-looking access token of one is refused by the auth handlers.

Deleted: `ChallengeSourceRecoveryTakeback`, the takeback transaction and its
test, `ErrAccountHasCredential` and the one-passkey-per-account check (an
existing account is no longer a registration target, so there is no second
finish to refuse), `HandleRecoveryVerify`'s premature `SetEmailVerified` and
source decision, the early `register/begin` account creation and its
"resume incomplete registration" branch.

## Routes that create or bind an account, a passkey or a key, and what holds each

| Route | What it can bind | What holds it |
| --- | --- | --- |
| `POST /auth/register/mail` | nothing (an attempt row, no user) | rate limits of the email-attempt table; cap check; existing address goes to recovery mail |
| `POST /auth/register/begin` | nothing (a pending row, no user) | needs a valid, unused, expired-checked code bound to this address and binding |
| `POST /auth/register/finish` | new user + passkey, atomically | pending row consumed once; address unique; `email_verified_at` set in the same transaction |
| `POST /auth/app/register/finish` | the same, then a key | the same; the key is minted only for the account that transaction made |
| `POST /auth/recovery/request`, `verify` | nothing until finish | token mailed to the stored address, single use |
| `POST /auth/register/finish` with a recovery challenge | a passkey on an existing account | the recovery token consumed at verify; in one transaction: verify the address, replace the old holder if it was unproved |
| `POST /auth/login/finish`, `POST /auth/app/login/finish` | a session or a key for an account that has a passkey | the session and the key are made by `issueAccessJWT` and `CreateAPIKey`, which refuse an unproved account (`address_unproved`) |
| `POST /auth/email/verify`, `.../verify-address/*` | a session | already required a verified address; verify-address needs a session an unproved account cannot have |
| everything that acts as an account (`/auth/*` with a cookie or a bearer key, the proxy's `/api/*`) | what the credential allows | the credential validators (below), not the handlers |

## Where the rule is enforced (round 4: not handler by handler)

Handler checks escaped by other handlers twice, so the rule lives at the few
functions every request's authority passes through:

- **access token:** `issueAccessJWT` makes none for an unproved account and
  puts the proof's time in an `ev` claim; `validateAccessTokenMethod` (the one
  token validator in the auth service: every authority decision goes through it, `requireAuthUser*`,
  `requirePasskeySession`, `/auth/session`, the desktop exchange, the access log, and the
  logout's choice of whose sessions to delete) requires `ev` and re-reads the account; the **proxy's `validateAccessJWT`
  requires the `ev` claim too** (the proxy opens the auth store and could read the user row, but does not on this path: the claim keeps the check cheap; every token the
  auth service now makes carries it and none is made for an unproved account,
  and a token **without** `ev` is rejected from the deploy on, so tokens made
  before it stop working at once; a signed-in browser reads `/auth/session`,
  which rotates its refresh session and issues a token with the claim);
- **API key:** `GetAPIKeyByHash` joins the account's proof (auth's bearer path
  and the proxy's `validateAPIKey` both go through it), and
  `CreateComputerScopedAPIKey` inserts only for a proved account;
- **refresh rotation:** `GetRefreshSessionByTokenHash` joins the account's proof
  (session reads, rotation, the passkey gate); `GetRefreshSessionByID` and `ListAccountSet` (account switch) join it too;
- **grants:** `PollDeviceAuthorization` and `ConsumeDesktopExchangeCode` do not
  see a grant of an unproved account.

`cmd/auth/unproved_credentials_test.go` takes **every route in the route table**
and, for each kind of credential an unproved account can hold (a token without
`ev`, a token with `ev`, an API key, a refresh cookie, an approved device code, a
desktop code), asserts the route answers as it does to a credential that is not
valid; a new route is covered by being in the table.

## What happens to accounts that already exist with `email_verified_at` empty

**Production, read-only (the lead, 2026-11-24): 199 of 1,097 accounts are
unproved; 27 of them hold a passkey; none owns a computer.** Whether to delete
those rows outright is the owner's decision and is not built here. The rule, without
migration cleverness: such an account cannot sign in or mint anything until
its address is proved by mail. At boot every key, session, device
authorization and desktop code of such an account is revoked or deleted once
(the account that seeds the operator's bootstrap key is exempt), the login and
mint paths refuse it, and the mail recovery that proves the address replaces
its passkeys with the recovering one in the same transaction. A legitimate old
account therefore costs its owner one recovery by mail; a squatter's holdings
are gone before the owner arrives, and cannot come back, because nothing an
unproved account can reach mints.

## What is still not uniform, and what mail now carries

- **Mail is the root of sign-up.** `register/mail` answers an error when the mail could not be sent
  (503, shown on the page), for a new and a known address alike, and never "we mailed a code" when
  nothing went out. New sign-ups stop while mail is down, and the 27 unproved passkey holders have no
  fallback: the operator's escape hatch is the bootstrap Bearer key (the proof checks and the key join
  exempt it).
- **`/auth/login/begin` still answers 404 for an unknown address**: account existence is not hidden
  there; the same-answer work covers `register/mail` only.
- **Case folding is ASCII** (SQLite `lower()`); two rows that differ by case can exist only from before
  this change, the oldest wins the lookup and recovery mails that row's stored address (the same
  mailbox), so there is no hijack.
- **The PRF owner root of an unproved account is deleted when it recovers.** Nothing ships encrypted under
  that root today, so no data is lost; if the root is ever wired to user data before the 27 recover,
  their data would become unreadable. State this before wiring it.
- **Known address: no dead end is hidden, but there is one.** For an address that already has an account `register/mail` answers like any address and mails the recovery link instead of a code; a person creating an account sees the code screen and no code ever comes. The web page and the phone now say so on that screen (open the link on choir.news, add a passkey, then sign in); the phone cannot open the link itself.
- **Timing is not the same.** A known address with a live `register/mail` link returns before the mail call; a known address with none, and a new address, make one mail call; a mail outage answers 503 for the last two and 200 for the first. A caller can therefore tell a known address with a live link from the rest. Account existence is not hidden at `/auth/login/begin` either, so this adds no new class.
- **A failed send still spends the attempt**: `register/mail` makes its email-attempt row before the mail goes, so five failed sends stop that address's sign-up for an hour (the 429); the recovery link of a failed send is discarded at once.
- **Mail per address per hour, across three routes**: up to 5 from `register/mail` (the email-attempt cap; one of them a recovery link for a known address, one live link at a time), 3 from `/auth/recovery/request` and 5 from `/auth/email/begin`, about 13 in all. `register/mail`'s recovery links are their own source: they do not replace, and are not suppressed by, the links `/auth/recovery/request` made, and the other way round. A link that could not be mailed is discarded at once so the retry sends again. `/auth/recovery/request` no longer replaces a link that is still live: a request while one is good makes no link and sends no mail and answers the same 200, so strangers' calls cannot supersede the owner's link or spend the budget (a person who deleted the mail waits for the link's 15 minutes to pass, or uses the one in the inbox).
- **Limits are shared**: five `register/mail` requests for one address stop that address's sign-up for
  an hour (it supersedes the live register attempt), and a known address's request spends the
  email-attempt budget; it no longer spends `/auth/recovery/request`'s or supersedes a recovery link
  already in the inbox.

## Remaining error

- The 27 unproved accounts that hold a passkey cannot sign in until they
  receive a recovery mail, and **if mail is down there is no fallback**.
- The one-time backfill (`migrateEmailVerifiedAt`) marked every account that
  held a passkey when the column was added as proved without proof, so a
  pre-2026-11 account squatted before then is outside the invariant.
- Recovery strength: mail access alone is enough to take over a **proved**
  account: recovery adds a passkey, and a holder of that passkey may delete the
  others. The mailbox is the account's root of trust, as before this change.
- The proof is a code; the phone app does not open a mailed link, so an address
  that already has an account can only be recovered through the web link.
- Access tokens carry `ev`; those without it are rejected by the auth service
  and the proxy from the deploy on, so every signed-in browser refreshes once.

## Security-Effect texts (for the reviewing seat; no trailer is written without a review)

- **The proof rule**: "Account creation via `/auth/register/mail`, `/auth/register/begin` and `/auth/register/finish` or `/auth/app/register/finish` requires a single-use 8-digit code mailed to the address and bound to a client secret, holds a five-minute `pending_registrations` row (with the address) after begin, and writes the `users` row, its first passkey and `email_verified_at` in one transaction at finish, one account per mailbox whatever the case. An account whose address is not proved is refused at the credential validators: the access-token validator in auth and the proxy (both require the `ev` claim, so tokens made before the rule are rejected), the API-key, refresh-session and account-set lookups, the device and desktop grant redemption, and key and token creation. Mail-proved recovery is the only way to add a passkey to an existing account; it replaces an unproved holder's passkeys, keys, sessions, grants, account sets and PRF owner root in the same transaction, and recovery of a proved account only adds a passkey (mailbox access suffices to take over a proved account). The phone's finish routes mint a full-scope bearer key with no cookie, accept `AUTH_RP_ORIGINS` (`https://choir.news` in production), require user verification, and look up credentials for the asserted user handle."
- **The CI stub** (`.github/workflows/ci.yml`, a gate file): "gate change; the shared-code CI job creates a stub `@exact/passkeys` directory (its `package.json` holds the name, a version and `private: true`, no dependencies) so `bun install --frozen-lockfile` succeeds without an Exact checkout; no test, check or permission is removed or weakened."

## Across a deploy (the `ev` claim)

`node-b-ci-deploy` restarts `go-choir-auth`, `maild`, `corpusd` and `proxy` in one `systemctl restart` after the pointers are synced; a pointer deploy restarts the selected services one after the other in the order of `HOST_SERVICES`.
- **auth new, proxy old:** nothing breaks. The old proxy checks the signature and ignores `ev`; tokens from the new auth carry it and tokens from the old auth do not, and both pass. At the new auth, a token without `ev` fails `/auth/session`'s validator and falls to the refresh rotation, which issues one with `ev`; a cookie-only call to an `/auth/*` route with an old token (before any `/auth/session`) answers 401 until the browser reads `/auth/session`.
- **proxy new, auth old:** every cookie-authenticated `/api/*` and websocket call answers 401 from the proxy's restart until auth is also new, because the old auth still issues and rotates tokens **without** `ev`. In the single restart the window is the gap between two units starting (seconds). Recovery is the auth restart itself; the next `/auth/session` (the frontend reads it on load and on a 401) rotates the refresh session into a token with `ev`. **Do not roll back auth alone**: an old auth keeps minting tokens without `ev` and every browser loops until the proxy is rolled back too.
- **API keys (the phone, the CLI, agents inside computers) carry no `ev`** and are unaffected in either order.
- **No computer or service holds an access token.** The proxy removes the `Cookie` header before it forwards to a computer (`internal/proxy/handlers.go`, the request builder), and a computer is given the `X-Proxy-Trusted-*` headers, not the token; only auth and the proxy parse the token (`golang-jwt` is imported in no other package). The one other holder of a token is the native desktop app, which keeps the cookies as a browser does and refreshes through `/auth/session`.
- A signed-in browser pays one refresh at the deploy; the 27 unproved passkey holders and the rest of the 199 are signed out for good (their keys and sessions are revoked at the auth start).

## Sign-out and the pending revoke, as built

Sign-out clears the phone at once and does not wait on the network. In this order (account.ts `signOutInner`): the marker; the held key's id is added to the list in `app:/data/pending-revoke` (written atomically, **before anything is deleted**); native flushes the run sheet (it posts under the key, which still works), deletes the key **only if the stored key id is still the one being signed out** (a sign-in that kept another key meanwhile is left whole, and the page says the sign-out did not finish) and wipes; then shared code revokes the key it read before, with that key (`DELETE /auth/api-keys/<id>`, 8 s). A revoke that was answered removes that id from the list. If native answers **after** its deadline, the late continuation wipes and then makes the same revoke. The file is a list of `{id, tries}`, at most 8, with the try count (the number of retries that answered "not this key's"); a later sign-out **adds** its id and never replaces an unresolved one; when the list is full the entry with the most "not yours" answers goes. A phone that could not reach the server keeps the id and says so: the key is revoked the next time the app is signed in to **this account** (a signed-out phone holds no key to revoke with), and Settings on the web revokes it now. The retry runs at launch and after a sign-in, with the key held then, for each entry in turn; it never revokes the key the phone is signed in with, keeps an entry that this key cannot revoke (another account's key answers 404, a narrower one 403) and drops it after 60 such answers (data sources have no clock), and stops at the first entry it cannot reach the server for.

**What is not guaranteed:** a key is never revoked if the person never signs in to the same account again on this phone and does not use Settings on the web; a sign-in to a **different** account does not revoke it (its 404 or 403 only counts toward the drop); after 60 such answers the entry is dropped and the key stays live until revoked from the web; and a list that is full evicts its most-answered entry. A key with no stored id is looked up with `GET /auth/whoami` using the held key; if that fails too the phone is cleared and the person is told to revoke from the web. The **legacy** key of the old Swift sign-in (`LegacyKey.retire`) with no stored id is the same case without the lookup: it is deleted from the phone unrevoked, the log says so, and it stays live until revoked from the web (the app is unlaunched; nothing carries over from that build).

## The Security-Effect cover for commit 5b2e141b0 (the one in force)

Commit `5b2e141b0` changed a gate file and auth code, so a trailer that says only "gate change; no check weakened" would be false for it. The sentence in force (the reviewing seat's; the lead adds the trailer): "gate change; `ci.yml` stubs `@exact/passkeys` so `bun install --frozen-lockfile` works without Exact; no test or permission removed. Auth: `register/finish` and `login/finish` consume the challenge atomically; the first passkey inserts only if none exists, failing closed; takeback replaces its passkeys, revokes keys, deletes sessions; the app challenge table is bounded per source and at 20000." The takeback and the one-passkey check it names are deleted later in this lane by commit `b65b360ad` (an account is made only at `register/finish`, with the proof of its address).

## Across a deploy (the `ev` claim)

`node-b-ci-deploy` restarts `go-choir-auth`, `maild`, `corpusd` and `proxy` in one `systemctl restart` after the pointers are synced; a pointer deploy restarts the selected services one after the other in the order of `HOST_SERVICES`.
- **auth new, proxy old:** nothing breaks. The old proxy checks the signature and ignores `ev`; tokens from the new auth carry it and tokens from the old auth do not, and both pass. At the new auth, a token without `ev` fails `/auth/session`'s validator and falls to the refresh rotation, which issues one with `ev`; a cookie-only call to an `/auth/*` route with an old token (before any `/auth/session`) answers 401 until the browser reads `/auth/session`.
- **proxy new, auth old:** every cookie-authenticated `/api/*` and websocket call answers 401 from the proxy's restart until auth is also new, because the old auth still issues and rotates tokens **without** `ev`. In the single restart the window is the gap between two units starting (seconds). Recovery is the auth restart itself; the next `/auth/session` (the frontend reads it on load and on a 401) rotates the refresh session into a token with `ev`. **Do not roll back auth alone**: an old auth keeps minting tokens without `ev` and every browser loops until the proxy is rolled back too.
- **API keys (the phone, the CLI, agents inside computers) carry no `ev`** and are unaffected in either order.
- **No computer or service holds an access token.** The proxy removes the `Cookie` header before it forwards to a computer (`internal/proxy/handlers.go`, the request builder), and a computer is given the `X-Proxy-Trusted-*` headers, not the token; only auth and the proxy parse the token (`golang-jwt` is imported in no other package). The one other holder of a token is the native desktop app, which keeps the cookies as a browser does and refreshes through `/auth/session`.
- A signed-in browser pays one refresh at the deploy; the 27 unproved passkey holders and the rest of the 199 are signed out for good (their keys and sessions are revoked at the auth start).

## Sign-out and the pending revoke, as built

Sign-out clears the phone at once and does not wait on the network. In this order (account.ts `signOutInner`): the marker; the held key's id is added to the list in `app:/data/pending-revoke` (written atomically, **before anything is deleted**); native flushes the run sheet (it posts under the key, which still works), deletes the key **only if the stored key id is still the one being signed out** (a sign-in that kept another key meanwhile is left whole, and the page says the sign-out did not finish) and wipes; then shared code revokes the key it read before, with that key (`DELETE /auth/api-keys/<id>`, 8 s). A revoke that was answered removes that id from the list. If native answers **after** its deadline, the late continuation wipes and then makes the same revoke. The file is a list of `{id, tries}`, at most 8, with the try count (the number of retries that answered "not this key's"); a later sign-out **adds** its id and never replaces an unresolved one; when the list is full the entry with the most "not yours" answers goes. A phone that could not reach the server keeps the id and says so: the key is revoked the next time the app is signed in to **this account** (a signed-out phone holds no key to revoke with), and Settings on the web revokes it now. The retry runs at launch and after a sign-in, with the key held then, for each entry in turn; it never revokes the key the phone is signed in with, keeps an entry that this key cannot revoke (another account's key answers 404, a narrower one 403) and drops it after 60 such answers (data sources have no clock), and stops at the first entry it cannot reach the server for.

**What is not guaranteed:** a key is never revoked if the person never signs in to the same account again on this phone and does not use Settings on the web; a sign-in to a **different** account does not revoke it (its 404 or 403 only counts toward the drop); after 60 such answers the entry is dropped and the key stays live until revoked from the web; and a list that is full evicts its most-answered entry. A key with no stored id is looked up with `GET /auth/whoami` using the held key; if that fails too the phone is cleared and the person is told to revoke from the web. The **legacy** key of the old Swift sign-in (`LegacyKey.retire`) with no stored id is the same case without the lookup: it is deleted from the phone unrevoked, the log says so, and it stays live until revoked from the web (the app is unlaunched; nothing carries over from that build).

## The Security-Effect cover for commit 5b2e141b0 (the whole commit)

Commit `5b2e141b0` changed a gate file and auth code, so a trailer that says only "gate change; no check weakened" would be false for it. The sentence that covers all of it: "gate change plus auth changes; `.github/workflows/ci.yml` makes the shared-code CI job create a stub `@exact/passkeys` directory (its `package.json` holds the name, a version and `private: true`, no dependencies) so `bun install --frozen-lockfile` succeeds without an Exact checkout, and no test, check or permission is removed there; in auth, `register/finish` consumes its challenge with one atomic delete and, for a first registration, inserts the passkey only if the account has none, failing closed on a read error, a recovery-takeback challenge may add a passkey to an account that already has one and replaces its passkeys, keys and sessions in one transaction, credential issuance on the app routes is bounded as before, and the app-login challenge table is bounded per source. The takeback, the one-passkey check, `ChallengeSourceRecoveryTakeback` and `AddRegistrationCredential` are all deleted later in this lane by commit `b65b360ad` (an account is made only at `register/finish`, with the proof of its address); that commit and the rounds after it carry the guard in its final form."
