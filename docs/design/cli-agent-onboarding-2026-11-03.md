# CLI and agent onboarding: full-access default keys and `choir login`

Date: 2026-11-03. Status: design, build follows on branch
`cli-agent-onboarding`. Mutation class of the build: red (auth and session,
API key minting). Review: tier 2 (two seats), run by the owner.

## Why

Owner, 2026-11-03: "We should change the default to actually give people what
they need. Really it should be easy to give the choir cli or mcp to an agent
and its auto setup. Otherwise, it's too complex of a developer product."
Earlier: "for now, we should just grant all access. We don't need to do this
shenanigans of all these different API key scopes currently."

## The friction today (main at 05a443da0)

1. The `choir` CLI and `choir mcp` need an API key from `--api-key-file`,
   `$CHOIR_API_KEY` or (MCP only) `$CHOIR_API_KEY_FILE`. Nothing else. There is
   no `choir login` and no credentials file, so every agent host needs a
   person to paste a secret into an env var or a file.
2. The key comes only from the browser: Settings, "API keys". The simple
   button sends `scopes: ["admin", "owner:acts"]`; the advanced picker starts
   from `read:texture, read:base, read:runtime`. The API refuses a create with
   no scopes (400 "at least one scope is required"), and the CLI's
   `api-key create` defaults to the same three read scopes.
3. Owner acts need `owner:acts`, which `admin` does not imply, so a key made
   from the picker without it fails at the first approve or abandon with a
   403 the agent cannot fix.
4. There are 19 scope strings (internal/auth/handlers.go `validScopes`). A
   person setting up an agent has to understand them to get a working key.

## Decision

1. **Default full access.** A key created with the `scopes` field omitted gets
   every scope the auth service issues, `owner:acts` and `admin` included.
   "Every scope" is one list, `auth.AllScopes()`, computed from the scope
   registry (`validScopes`) minus the retired
   `computer:self_development:propose`. An explicit `scopes` list still works
   and still means exactly that list: per-scope keys stay possible, they are
   just no longer the default. An explicit empty list is still refused.
2. **Who gets what by default.** A signed-in passkey session that omits scopes
   gets `AllScopes()`. A Bearer key with `manage:keys` or `admin` that omits
   scopes gets what it may delegate: its own scopes (every scope for an
   `admin` key), minus `owner:acts`, which stays session-only. That rule is
   unchanged: a key cannot mint `owner:acts`.
3. **Settings.** The simple button sends no scope list (the server fills it).
   The advanced picker starts with every scope selected, so narrowing is an
   explicit act.
4. **The CLI's `api-key create`** sends no scope list unless `--scopes` is
   given.
5. **Existing keys are not widened.** A key keeps the scope list it was minted
   with. Keys minted from the simple button already carry `admin` and
   `owner:acts`, which reach every route (the proxy treats `admin` as every
   scope except owner acts). A narrower key exists because someone chose a
   narrower key, for example an agent key for one computer or the e2e inbox
   reader (`read:runtime`). Widening those silently would hand owner acts to
   credentials that were deliberately scoped down. To get a full key, run
   `choir login` or create a new key in Settings, and revoke the old one.
6. **`choir login`, a device-authorization flow** (the RFC 8628 shape):
   - `POST /auth/device/begin {label}`, no auth, returns a `device_code` (a
     256-bit secret the CLI keeps), a `user_code` (`XXXX-XXXX`, 8 letters
     from a 20-letter alphabet without look-alikes), the approve link
     `https://<host>/device?code=XXXX-XXXX`, `expires_in` (600 s) and
     `interval` (5 s).
   - The person opens the link in a browser where they are signed in with a
     passkey (or signs in there), sees which device is asking (the label the
     CLI sent, the browser or tool and the network it asked from, when it
     asked) and presses Approve or Deny. `GET /auth/device/lookup` and
     `POST /auth/device/approve` take the session cookie only: no API key,
     and no email-method session (an email session cannot mint keys, email
     sign-in design T5, and approving a device mints one).
   - The CLI polls `POST /auth/device/token {device_code}`. The answers are
     `authorization_pending`, `slow_down` (polled faster than the interval),
     `access_denied`, `expired_token`, `invalid_grant` (unknown or already
     used) or, once, the key. The key is minted in the same transaction that
     marks the authorization used, so it exists exactly once, and its secret
     is never stored: it is generated at that moment and returned in that
     one response.
   - The CLI writes `~/.config/choir/credentials` (`$XDG_CONFIG_HOME` when
     set), mode 0600 in a 0700 directory, as JSON: host, key id, label and
     the secret. A re-login revokes the key the old file held, best effort.
   - `choir logout` revokes the key (a key may always revoke itself) and
     removes the file.
   - `choir mcp install <claude-code|cursor|claude-desktop|codex>` writes that
     host's MCP config entry (`command: <this choir binary>`, `args: ["mcp"]`,
     no env: the server reads the credentials file); `--print` prints it
     instead. The agent host is positional because `--host` already names the
     Choir host on every command.
7. **Credential resolution, CLI and MCP alike:** `--api-key-file`, then
   `$CHOIR_API_KEY`, then `$CHOIR_API_KEY_FILE`, then the credentials file.
   (For `choir mcp` this puts `$CHOIR_API_KEY` ahead of `$CHOIR_API_KEY_FILE`,
   where the file used to win; both set at once is a misconfiguration.) A key
   from the credentials file is sent only to the host it was minted for: with
   no `--host` and no `$CHOIR_HOST` the CLI uses the file's host, and a
   different explicit host is refused with "run choir login --host".

## Limits on the device flow (as the email code does it)

- Expiry: an authorization lives 10 minutes; an approval not collected
  before then is dead.
- Single use: approve and deny are one conditional update from `pending`;
  collecting the key is one conditional update from `approved` in the same
  transaction as the key insert. A second poll after the key was collected
  answers `invalid_grant`.
- Attempt binding: only the holder of the `device_code` can collect the key.
  The `user_code` on the screen, in the link or in a forwarded message
  approves the request but can never receive the key. Both are stored only as
  SHA-256 hashes.
- Rate limits: begin, 10 per hour per IP (counted in the store, in the insert
  transaction) under the auth service's global per-IP limiter; lookup and
  approve under the global limiter, plus 10 unknown-code lookups per hour per
  signed-in user; token polls under their own per-IP limiter (60 a minute),
  since the global one (10 a minute) would refuse a 5-second poll, plus
  `slow_down` per authorization.
- CSRF: approve takes `Content-Type: application/json` only (a cross-site
  form cannot send it without a CORS preflight, which the auth service does
  not answer), and the session cookies are SameSite=Lax, so a cross-site POST
  carries no session.
- A sweeper deletes authorizations older than 24 hours.

## Security consequences

- **What a leaked full-access key can do.** Everything the owner can do
  through the API on every computer they own: read and write Texture, files
  and mail; run agents and spend model budget; self-development approve,
  apply, rollback and abandon; lifecycle stop, refresh and recover; owner acts
  (approve, arm, restore, rematerialize, bootstrap a chain, re-pin a verifier
  key); mint more keys without `owner:acts` (`admin` includes key
  management) and revoke sibling keys. It cannot: mint a key with
  `owner:acts`; approve a device login; add, rename or remove passkeys; list
  or revoke sessions; unwrap the data ROOT; change the account email. Those
  stay behind a passkey session. A leaked full-access key is therefore close
  to a stolen owner session for everything the computers hold, short of
  taking over the account itself.
- **Before and after.** The simple Settings button has minted `admin` plus
  `owner:acts` since 2026-10-24, so for most keys in use nothing changes. What
  changes: the API default (omitted scopes) and the advanced picker's starting
  point are now full access, and `choir login` puts a full-access key on disk
  without a copy and paste.
- **The new path's own risks.** Device-code phishing: someone runs
  `choir login`, sends the owner the link and asks them to approve. Approving
  gives that person a full-access key. The approve page names the requesting
  device and network, says what approving grants, and says to approve only a
  login you started yourself; this is the residual every device flow carries.
  A stolen `user_code` alone is useless to its thief: it can only approve a
  request whose key goes to the CLI that began it.
- **Revocation.** Settings, "API keys" (or `choir api-key revoke <id>`, or
  `choir logout` from the machine holding the key) revokes a key at once: the
  auth service and the proxy check `revoked_at` on every request. Device-login
  keys are labelled `choir login: <label>` and show their last use. Signing
  out of the browser does not revoke keys.
- **Expiry.** None by default: like the Settings keys, a device-login key
  lives until revoked, because an agent's setup should not break on a timer.
  Settings' advanced form still sets an expiry for a key that should die.
  Residual: an unused key on a forgotten machine stays live; last-used dates
  in Settings are the control for now.
- **The credentials file** holds a live secret in plain text at mode 0600, as
  `~/.aws/credentials` and `gh`'s hosts file do. Any process running as that
  user can read it, including an agent the user runs. That is the point: the
  agent uses it. The CLI refuses a credentials file with any other mode.

## Rollback

`git revert` of the build commit. The `device_authorizations` table is
additive and harmless when unused; keys minted by the flow remain ordinary
keys, revocable in Settings. The scope default reverts with the commit; keys
minted under it keep their scopes until revoked.

## Acceptance

`scripts/e2e-cli-login-staging.sh`: a fresh test account on a test domain,
registered with a virtual passkey in Playwright; `choir login` against
staging; the spec opens the printed link, checks the requester label on the
page and presses Approve; the CLI writes the credentials file; then
`choir self-dev list` runs with no flags and no env. A second collect of the
same `device_code` answers `invalid_grant`. The script writes
`<out>/acceptance.json`.
