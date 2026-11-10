# One 10-per-minute auth bucket locks out account switching and sign-in (2026-11-09)

Status: investigated, read-only. Mutation class of this doc: green.
Cause: strongly supported by code, not confirmed by logs (see Evidence).

## The report

Owner, iPhone Safari, https://choir.news, about 02:00 to 02:05 UTC on
2026-11-10 (the owner's local date was 2026-11-09). Signed in as one
account, they tried to sign in with a second account. The sheet showed
"Passkey sign-in failed. Nothing changed, try again or use another device."
for both of their accounts (the owner's account and the test account). The
account switcher showed "too many auth requests from this address". After
closing and reopening Safari they were signed in as the test account, and
could sign in to the owner's account again.

## Cause (leading, code-supported)

`cmd/auth/main.go:51` builds one `IPRateLimiter` (10 requests, 1 minute,
`internal/auth/rate_limit.go:10-16`). `registerRoutes` (`main.go:68-120`)
wraps nearly every `/auth/*` route with it. That includes the read-only
`/auth/session` and `/auth/accounts`, the state-changing switch and signout,
and the unauthenticated `/auth/login/begin` and `/auth/login/finish`. They
all draw on one fixed-window bucket per client address. The window starts at
the first request and does not slide (`rate_limit.go:60-79`). The refusal is
HTTP 429 with the body `too many auth requests from this address`
(`rate_limit.go:49`), with no `Retry-After` header and no log line.

The shell spends that budget on routine reads. Ten requests is about two
account actions.

Why the sheet says "Passkey sign-in failed":

- `loginPasskey` throws `new Error(err.error || 'Login begin failed ...')` for
  a non-OK begin (`frontend/src/lib/auth.js:345-348`) and the same for finish
  (`auth.js:401`). The message is the 429 body.
- `passkeyErrorMessage` (`auth.js:644-670`) matches only cancel, invalid
  state, network wording, not found, begin failed or finish failed. The 429
  text matches none, so it falls to the generic "Passkey sign-in failed"
  line at `auth.js:670`. The real refusal is hidden.
- `handleAuthBegin` (`App.svelte:248-290`) shows that line. Only a thrown
  error shows it.

Why the UI can look signed out while the server signed in:

- `requestSessionState` (`auth.js:466-496`) treats only status 0 and 5xx as
  transient. A 429 on `/auth/session` throws a permanent error.
- `checkSession` (`App.svelte:128-131`) turns a permanent error into
  `authState = 'signed_out'`, `currentUser = null`. A rate-limited read signs
  the UI out.
- So a passkey finish that succeeds (session cookies set), followed by a
  429 on the post-login `checkSession`, leaves the sheet open and no
  message. A restart a minute later reads the new session. That fits "I was
  logged in as the test account after restarting Safari".

Why a Safari restart cleared it: the fixed window ended. Nothing else
changed. A wait of about one minute would have done the same. Not a stale
WebAuthn challenge, not a pending credential request.

## Requests per action (client, file:line)

Counted as `/auth/*` calls that hit the limiter. `/api/*` does not.

| Action | Calls | Where |
| --- | --- | --- |
| Page load or reload | 1 `GET /auth/session` | `App.svelte:441` via `getSession` (`auth.js:531`) |
| Page load that hands off to the computer surface | +1 page load (another `/auth/session`) when the asset graph is stale | `computer-surface-handoff.js:96`, `App.svelte:444` |
| Window focus while signed in | +1 `/auth/session` each time | `App.svelte:481-487` |
| bfcache restore (`pageshow` persisted) | +1 `/auth/session` | `App.svelte:470-475` |
| Any `/api` 401 renewed by `fetchWithRenewal` | +1 `/auth/session` | `auth.js:610` |
| Open the Desk sheet (launcher) | 1 `GET /auth/accounts` | `Launcher.surface.svelte:29-35, 131` |
| Switch account | 1 `POST /auth/accounts/switch`, then a reload (1, sometimes 2 session reads), then the sheet reopens (1 list) | `accounts.js:161-167` |
| Failed switch (member signed out) | +1 list refresh | `Launcher.surface.svelte:53` |
| Sign out the active account | list 1, signout 1, switch 1, reload 1 = 4 | `accounts.js:170-188` |
| Reconcile after `account_switched` | 1 switch + reload (1 or 2) | `accounts.js:203-219`, `auth.js:505-513` |
| Sibling tab hears `account-switched` | 1 reload = 1 or 2 per open tab | `App.svelte:493-497` |
| Passkey sign-in | begin 1, finish 1, `checkSession` 1, then `location.replace` reload 1 (or 2) = 4 to 5 | `App.svelte:255-276`, `auth.js:337-406` |
| Add-account sign-in while signed in | the same 4 to 5, plus a focus read each time the passkey sheet returns focus (1 to 2) | `App.svelte:481-487` |

The owner's sequence (open the sheet, switch, reload, open the sheet,
choose the other account, passkey sign-in, then retries) costs about 12 to
15 requests in a minute. That is over 10 with one tab and no retry. Each
retry or extra tab adds more. The limiter is exceeded by one ordinary
two-account task.

## Is the bucket shared across users?

The key is `clientIP`: the last `X-Forwarded-For` entry, else the peer
(`recovery.go:58-77`). On Node B, Caddy (`nix/node-b.nix:724-726`) proxies
`/auth/*` straight to the auth service on 127.0.0.1:8081, not through the
Go proxy. Caddy sets XFF to the real peer. So the bucket is per real client
address, not one for the whole fleet. Not a fleet-wide lockout.

It is still shared by everyone behind one address: a household, an office,
carrier-grade NAT on cellular, and one person's several devices. On a
phone on cellular this can be many strangers. The `127.0.0.1:8081` path
means that if the auth service were ever fronted by a Go reverse proxy (it
appends the peer, here 127.0.0.1), every user would collapse into one
bucket. Not the case today. Worth a test.

## Evidence

Confirmed from auth logs (journal, unit `go-choir-auth`, window
02:00 to 02:10 UTC on 2026-11-10; ids shortened):

- 02:00:29 session_refresh user c061.
- 02:01:20 session_refresh user 5bd6.
- 02:01:33 login_begin c061 success; 02:01:36 login_finish success, session issued.
- 02:01:45 logout c061.
- 02:01:53 login_begin c061 success; 02:01:55 login_finish success, session issued.
- 02:03:58 login_begin 5bd6 success; 02:04:00 login_finish success, session issued.
- 02:05:00 account_switch to c061, result success (two lines).
- No error lines in the window. No `login_finish result=error`.

What this shows: the server accepted every passkey assertion it saw. So
for those three attempts the UI failure was after the server's success, or
the failing attempts never reached a logged handler. The second is
consistent with 429, which is the first thing the wrapper does.

Not confirmable from logs:

- The limiter writes no log line on a refusal (`rate_limit.go:46-52`).
  Auth logs hold no 429 count.
- The auth service logs only `session_refresh`, not plain session reads
  or account list calls, so request counts cannot be rebuilt.
- Caddy has no access log for `/auth` (journal shows only ACME errors).
  Zero status 429 lines found, and zero request lines at all, so absence
  proves nothing.

What would settle it: log the refusal (address hash, route, count), or read
the owner's Safari network log. Or reproduce in a Playwright run (below).

## Who can hit it

- Single-account users: yes. Reload plus focus plus a retry reaches 10 in
  a minute. Their UI reads as signed out on a 429 for `/auth/session`.
- Signed-out users: yes. Sign-in is 4 to 5 requests, so two attempts
  (a cancelled passkey prompt, then a retry) from one address can refuse
  the third, with the generic message.
- Multi-account users: most exposed, since switch, sheet and reload all
  cost requests.
- Anyone sharing an address with a heavy user.

Severity: can a user be locked out? For one fixed window only (up to 60
seconds), because the bucket resets. But the UI turns a refused read into
"signed out" and a refused begin into a message that says "use another
device", which pushes users to retry and spend more. Not a permanent lock.
A real defect for the launch: it breaks the main account-switch task on
the first try. Orange: runtime behavior and product API.

## Proposed fix at the substrate

The substrate problem: one budget for all cost classes, sized for
credential-guessing defense, applied to cheap own-session reads. The
safety budget for login begin and finish and the budget for reads must
not be the same number.

1. Separate limiters by cost class, in `cmd/auth/main.go` and
   `rate_limit.go`:
   - Cookie-authenticated reads of one's own state
     (`/auth/session`, `/auth/accounts`, `/auth/sessions`,
     `/auth/credentials` GET, `/api/auth/root/status`): key by session or
     account-set handle when present, else by address. Generous: about
     120 per minute per key, 600 per address.
   - State-changing session acts (switch, signout, logout-all, logout,
     credential and key changes): per session key, about 20 per minute.
   - Unauthenticated ceremony begin and finish (login, register): per
     address, tight on finish failures only. Count failed finishes, not
     begins: about 10 failed finishes per 10 minutes per address and per
     target user; allow 30 begins per minute.
   - Recovery, email and device routes keep their own limits unchanged.
   Derive the numbers from the table above: a worst-case switch is 4
   requests, a sign-in 5, a page load 1 to 2; budget for 5 such tasks a
   minute per key, plus 2 tabs.
2. Make the limiter a sliding or token bucket with `Retry-After` (seconds
   to refill) and a JSON `retry_after` field. Log every refusal once per
   key per window (route class, hashed address, count). Add a metric.
3. Client: show the real refusal. A 429 maps to "Too many sign-in requests.
   Try again in N seconds" with a countdown, never the generic line. A 429
   on `/auth/session` is transient: keep the current signed-in state, retry
   after `Retry-After` (add 429 to `isTransientStatus`, `auth.js:459`).
4. Fewer redundant reads: dedupe a focus read within a few seconds of the
   last session check; have the switch and login finish responses carry the
   user and active account so the client skips the follow-up
   `checkSession`; cache the account list for the sheet and update it from
   the switch response; skip the second reload read after
   `location.replace` when the first read just succeeded; read the
   session once per page, not once per tab reload burst.
5. Trust check: a test that two requests from different peers through the
   real Caddy get different buckets, so a proxy change cannot merge all
   users silently.

Failure list for the fix:

- Per-session keys are bypassed by sending no cookie; unauthenticated
  routes must stay address-limited.
- Cookie-keyed read limits can be rotated by an attacker with many
  cookies; keep the per-address ceiling.
- A sliding window raises state size; prune by time, bound the map.
- Counting failed finishes needs the outcome after the handler runs; a
  wrong key (user id from the challenge) must not leak whether the account
  exists.
- A larger read budget weakens enumeration defense only if reads leak
  existence; `/auth/session` and `/auth/accounts` answer only for the
  caller's own cookies, so they do not.
- `Retry-After` must be read through Caddy's compression and CORS; verify
  the header reaches the browser.
- Making 429 transient could leave a truly signed-out UI looking signed in
  until the retry; bound retries to the header value and show a
  "reconnecting" state.
- The fixed-window tests (if any) and the account-switcher spec expect the
  old message; update them.
- Deploy: in-memory buckets reset on auth restart, harmless.

## Reproduction (deterministic, Playwright, Chromium)

No Safari behavior is involved. Chromium with a virtual authenticator
reproduces it.

1. Start staging-like auth with the default limiter, or a local auth
   service. Create two accounts with virtual-authenticator passkeys. Sign in
   as A, add B (two accounts in the set).
2. In one browser context, within one minute from one address: reload the
   page twice, open the Desk sheet (list), switch to B (switch plus
   reload), open the sheet again, then choose Add account and start a
   passkey sign-in for A.
3. Count `/auth/*` requests with `page.on('request')`; the 11th returns 429.
   Assert on the status and on the sheet text. Before the fix the sheet
   shows "Passkey sign-in failed" (or the UI shows signed out after a 429
   on `/auth/session`). After the fix it shows the countdown and the
   session stays signed in.
4. Faster variant that needs no accounts: with `page.route`, return 429
   for `/auth/login/begin` and assert the message. And return 429 for
   `/auth/session` on a signed-in page and assert the shell stays signed
   in (it does not today).
5. Assert the bucket reset: advance time (`page.clock`) 61 seconds and
   sign in again.

The server-side half (10 per minute per address, shared by all `/auth`
routes) can also be a Go test on `registerRoutes` with 11 mixed calls from
one `X-Forwarded-For`.

## Superseded hypotheses

The first guesses (client treats a successful ceremony as failure on an
account comparison, a stale WebAuthn challenge, a pending credential
request, a server refusal while another account is active) are dropped.
The logs show the server issued sessions for all three attempts, and the
limiter explains both the message and the restart timing. Not tested:
Safari credential-request behavior. If a 429 is later ruled out for the
failing attempts, those come back.

## Heresy delta

discovered: one limiter budget for unlike cost classes; a 429 read signs
the UI out; the real refusal is hidden by a generic message; refusals are
not logged. introduced: none. repaired: none.
