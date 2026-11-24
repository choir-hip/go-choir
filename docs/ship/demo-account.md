# The reviewer account: runbook (2026-11-23)

Owner, 2026-11-24: "yes, we should make a demo account as well." Apple's reviewers for TestFlight and the App Store cannot hold a passkey for choir.news, and sign-up may be closed at the cap. One account can be opened with a user name and a password instead. Code: `internal/auth/demo_account.go`; host command `demo-account` (`cmd/demo-account`); the app page is the "Reviewer sign-in" link on the sign-in screen. No password, hash or account id is in the repo; the three live in two files on Node B, in `/var/lib/go-choir/auth-signing/demo/` (a directory only the auth service can see, so a file made or rotated later is hidden from the other services too).

## How a reviewer signs in (the app as it is, 2026-11-24)

The app's sign-in is passkey only (a mailed code proves a new address). The reviewer does not use it: on the sign-in screen the small "Reviewer sign-in" link opens a page with a user name and a password; the press posts them to `/auth/app/demo/login`, and the key that comes back is kept by the same function as a passkey sign-in's (`keepKey`, the three Exact secrets). No passkey, no email and no code is involved. Sign-out and the revoke of a key that was minted and not kept use `DELETE /auth/api-keys/{own id}`, which a key may do to itself without `manage:keys`.

## What the account can and cannot do

- **One account, by configuration.** `POST /auth/app/demo/login` takes `{app, username, password}`. The account is the id in the host file `demo-account-id`; the body refuses unknown fields, so a request cannot name another account. It answers 404 "not available" when the id file or the credential file is missing, unreadable or malformed, or the user does not exist. There is no switch in the app.
- **The key** is an owner-wide key with six scopes: `read:` and `write:` of `runtime`, `texture` and `base`, expiring after 30 days. Withheld, against the full app key: `owner:acts` (arm, approve, restore, rematerialize, package publish and install); `admin`; `manage:keys` (no second key, no way past the expiry); `computer:lifecycle`; every `computer:self_development:*`; `precommit:*`; `acceptance:read`. Already refused to every key by the proxy whatever its scopes: sending mail and the daily digest, publishing an opinion piece (browser session only). Account deletion needs a fresh passkey assertion and a session, which the reviewer does not have (see the residuals for the route that is still to refuse this account by name).
- **Not withheld, because a scope cannot say it:** the demo computer's model calls (they cost money like any computer's; the account is one computer with the usual per-computer limits) and a take the reviewer records through the app (a take is published through the same write scope). The reset command below removes the computer; it does not unpublish a take.
- **Bounds on attempts**, never on the account: 5 attempts per address per 10 minutes (an IPv4 address or an IPv6 /64), 60 attempts in 10 minutes across all addresses, and at most two password checks running at once. A refusal answers 429 with `Retry-After`; every bound lifts by itself. There is no per-account counter, so nobody can lock the account by guessing. A crowd can refuse the real reviewer only while it keeps coming (the global bound), and for at most the rest of the 10-minute window. The password is 32 random characters, so the bounds are for cost and noise, not the lock. Every attempt is one log line (`operation=app_demo_login result=success|denied|rate_limited`, the address prefix, never the user name or password).
- The user name and password are compared in constant time (the user names as digests; the password by argon2id, 64 MiB, 3 passes, 2 lanes; a wrong user name costs what a wrong password costs).
- `GET /auth/whoami` answers `"reviewer": true` for this account; the app's delete page reads it.

## Create the account (once, on staging)

1. Make an account the ordinary way (the app or choir.news) before sign-up closes, with an address you can read: sign-up mails an 8-digit code to it and the account works only once that code is typed (the address must be proved; an account with an unproved address makes the route answer "not available"). It is the reviewer's account and nothing personal goes in it.
2. Read its user id (`GET /auth/whoami` with a key it holds, or `sqlite3 /var/lib/go-choir/auth/auth.db "select id from users where email = '<the throwaway address>'"`).
3. On Node B as root: `umask 077; printf '%s\n' '<the user id>' > /var/lib/go-choir/auth-signing/demo/demo-account-id`.

## Install the password (and rotate it)

```
demo-account credential --user <a user name, for example "reviewer"> --out /var/lib/go-choir/auth-signing/demo/demo-credential
```

It prints the 32-character password once and writes the file (mode 0600: the user name, then an argon2id hash). Copy the password into Apple's form; it is not stored anywhere else. The auth service reads the file on every attempt, so there is nothing to restart. **To rotate**, run the same command again: the old password stops working at once, and keys already issued keep working until they expire or `reset` revokes them.

## Turn it off (one step)

```
demo-account off --id-file /var/lib/go-choir/auth-signing/demo/demo-account-id
```

Removes the credential file (the route answers "not available" at once), then revokes every key and session the account holds, so a reviewer's phone is out at once and not after the 30 days. The id file stays, so the account stays protected from deletion; remove it too to forget the account. The computer is left as it is (`reset` removes it). Run as root on Node B. `rm` of the credential file alone stops new sign-ins but leaves issued keys working until they expire.

`off` closes every way into the account that this service controls. Before it removes anything it proves the auth database exists and holds the account, and that the credential path is the one the auth service reads (`--credential`, default `/var/lib/go-choir/auth-signing/demo/demo-credential`, the nix value; any other path is refused, so a stale copy or a typo can neither pass nor be deleted in its place). A missing file is not success unless you pass `--already-off`, and even then the command proves the configured path absent. A stale copy at the old path `/var/lib/go-choir/auth-signing/demo-credential` is removed and said. It then removes the credential, revokes every key and deletes every session, deletes the account's pending device authorizations, desktop exchange codes, recovery links, email sign-in attempts and passkey challenges, waits two seconds, and proves keys and sessions gone a second time (a password login that was inside its check when the file was removed takes its own key back, so it cannot outlive the proof). While the credential file is absent the auth service makes no session token or app key for this account by any route (passkey, mailed link, recovery, device approval) and reads no session token for it on any route, including one made before `off`; the account is therefore also closed to its creator on the web until `credential` is run again. What `off` cannot close: the proxy checks an access token's signature and not this service, so a token made before `off` still opens `/api` for at most `AUTH_ACCESS_TOKEN_TTL` (5 minutes).

## Before each review: reset (state hygiene, not a way to end access)

```
demo-account reset --id-file /var/lib/go-choir/auth-signing/demo/demo-account-id
```

Run it as root on Node B. It revokes every key and deletes every session of the account (each reviewer phone is signed out), then asks vmctl (`/internal/vmctl/remove`, host-internal) to stop and remove the account's computer: its ownership and its snapshot are dropped, and the next sign-in makes a new empty computer on first use. It does not purge the old computer's disk image or tape (the retention prune takes an orphaned state directory after its age, and the account-deletion work purges the rest) and it does not unpublish a take. It is idempotent. Never run against any other account: it reads only the id file. `reset` does not change the password: whoever holds it can sign in again the moment it ends. To end someone's access run `off`, or rotate with `credential` (then give Apple the new password); `reset` is for returning the computer to empty. It sends vmctl the host-internal token it reads from `--token-file`, and only to a loopback host on vmctl's port 8083 (never another port, and a redirect is not followed) (default `/var/lib/go-choir/secrets/internal-service-token`, root-only), because vmctl enforces it and a root shell has none loaded; without it vmctl answers 403 and the command exits non-zero.

## What goes in Apple's review form

- Sign-in required: yes. User name: `REVIEW_USER_PLACEHOLDER`. Password: `REVIEW_PASSWORD_PLACEHOLDER` (the value the credential command printed).
- Notes (`docs/ship/testflight-texts.md`, "REVIEW ACCOUNT"): tap "Reviewer sign-in" at the bottom of the sign-in page. The listening, the paper and the reader work without any account.

## Residuals

- The account-deletion route is on `lane/account-delete`, not in this lane's base. It must refuse `Handler.IsDemoAccount(userID)` and the app's delete page must show the reviewer text when `whoami` says `reviewer`; both are one line each and are named in the hand-off to that lane. Until they land, the reviewer holds no passkey session, which the delete route requires.
- The reviewer account holds a passkey of whoever created it (step 1). That person can sign in to it on the web; they hold nothing else. The passkey routes (`/auth/credentials`) need a session, never this key.
- Nothing here was run against Node B.
