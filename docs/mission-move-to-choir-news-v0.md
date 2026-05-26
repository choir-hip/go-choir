# Mission: Move primary domain from draft.choir-ip.com to choir.news

## Goal

Migrate Choir's primary domain from `draft.choir-ip.com` (currently proxied through Cloudflare) to `choir.news` (registered at Gandi), redirecting all old domains, while keeping Node B as the deployment target.

## Reviewed Execution Decision

This is a hard domain cutover, not a seamless WebAuthn migration. Existing
passkeys registered for `draft.choir-ip.com` will not authenticate under the
new unrelated relying-party ID `choir.news`. The owner is currently the only
user and has no precious account data in staging accounts, so v0 may reset the
deployed auth database after backing it up. Acceptance must prove fresh
registration and login on `https://choir.news`; it must not claim preservation
of old `draft.choir-ip.com` passkeys or sessions.

Problem documentation first: before the behavior-changing code/DNS cutover,
commit this mission update as the problem/decision checkpoint. The follow-up
code/DNS commit may then reference this decision.

## Infrastructure

| Name | IP | Role |
|------|-----|------|
| Node B (us-east-vin) | 147.135.70.196 | Current deployment target; runs NixOS + Caddy + all go-choir services + frontend |
| Node A (choiros-a) | 51.81.93.94 | Older node running old code (choiros-rs); serves choir-ip.com via Caddy reverse-proxy to hypervisor on :9090 |
| Orphaned OVH | 147.135.24.51 | Stale DNS target for choir-ip.com; no longer in use |

## DNS Changes

1. `choir.news` — set A record to `147.135.70.196` (Node B). Use Gandi LiveDNS API with a Personal Access Token.
2. `draft.choir-ip.com` — keep pointing to `147.135.70.196` (already correct).
3. `choir-ip.com` — update to point to `147.135.70.196` (currently points to orphaned `147.135.24.51`). This is managed at Cloudflare.
4. Add Caddy redirects on Node B so `draft.choir-ip.com` and `choir-ip.com` 301-redirect to `https://choir.news`.
5. Optionally: add Resend DNS records (SPF, DKIM, etc.) to choir.news zone for future email deliverability.

### Gandi LiveDNS API Reference

**Create PAT (one-time):**
Go to https://admin.gandi.net/organizations/account/pat and create a token
with LiveDNS/domain technical permissions for the organization that owns
`choir.news`.

Personal Access Tokens use bearer authentication. Do not use the deprecated
`X-Api-Key`/`Apikey` style for this PAT.

**Verify PAT without leaking it:**
```bash
curl -H "Authorization: Bearer $GANDI_PAT" https://id.gandi.net/tokeninfo
```

**Set A record for choir.news:**
```bash
curl -X PUT https://api.gandi.net/v5/livedns/domains/choir.news/records/@/A \
  -H "Authorization: Bearer $GANDI_PAT" \
  -H "Content-Type: application/json" \
  -d '{"rrset_ttl": 300, "rrset_values": ["147.135.70.196"]}'
```

**Verify:**
```bash
curl https://api.gandi.net/v5/livedns/domains/choir.news/records \
  -H "Authorization: Bearer $GANDI_PAT"
```

**Delete old A record (if clearing a placeholder):**
```bash
curl -X DELETE https://api.gandi.net/v5/livedns/domains/choir.news/records/@/A \
  -H "Authorization: Bearer $GANDI_PAT"
```

Note: the PAT is organization-scoped, not user-scoped. Make sure the PAT is created under the organization that owns the choir.news domain.

## Files to Change

### 1. `nix/node-b.nix` — Caddy virtual host config

- Rename `virtualHosts."draft.choir-ip.com"` to `virtualHosts."choir.news"`.
- Add `redir https://choir.news{uri} permanent` blocks for `draft.choir-ip.com` and `choir-ip.com` so path/query are preserved.
- Update the `platform@choir-ip.com` email address (used in Dolt git config) to `platform@choir.news`.
- Update deployed auth environment values:
  - `AUTH_RP_ID=choir.news`
  - `AUTH_RP_ORIGINS=https://choir.news`

### 2. `AGENTS.md` — Staging environment URL reference

- Line 7: Change `https://draft.choir-ip.com` to `https://choir.news`.
- Line 204: Change deployed acceptance proof URL from `draft.choir-ip.com` to `choir.news`.

### 3. `internal/auth/config.go` — WebAuthn RPID/RPOrigins documentation

- Line 28: Update RPID example comment from `draft.choir-ip.com` to `choir.news`.
- Line 32: Update RPOrigins example comment from `draft.choir-ip.com` to `choir.news`.

### 4. `internal/auth/config_test.go` — Deployed auth test values

- Update `defaultRPID` and `defaultRPOrigins` from `draft.choir-ip.com` to `choir.news`.

### 5. `internal/auth/handlers_test.go` — Deployed handler env + test

- In `deployedHandlerEnv()`, change `RPID` and `RPOrigins` from `draft.choir-ip.com` to `choir.news`.
- In `TestRegisterBeginWithDeployedRPID`, update the expected `rp.id` assertion from `draft.choir-ip.com` to `choir.news`.

### 6. `internal/gateway/gateway_test.go` — Origin header in test

- Update the `Origin` header value from `https://draft.choir-ip.com` to `https://choir.news`.

### 7. `internal/proxy/platform_publish_test.go` — PublicURL assertions

- Update `PublicURL` expected values from `draft.choir-ip.com` to `choir.news`.

### 8. `internal/runtime/content.go` — Bot user-agent string

- Update the `choir-bot` UA string from `draft.choir-ip.com` to `choir.news`.

### 9. `internal/runtime/podcast.go` — Bot user-agent string

- Update the `choir-podcast-download` UA string from `draft.choir-ip.com` to `choir.news`.

### 10. `.github/workflows/ci.yml` — Smoke test URL

- Update the smoke-test `curl` URL from `draft.choir-ip.com` to `choir.news`.

### 11. `frontend/check-network.mjs` — Hardcoded URLs

- Update all four `https://draft.choir-ip.com` references to `https://choir.news` (lines 28, 34, 58, 64-65, 68, 75).

### 12. Other frontend files with hardcoded draft.choir-ip.com

The following files also contain hardcoded URLs and should be updated:

- `frontend/tests/` — 10 `*.spec.js` files with `BASE_URL`/`DEPLOYED_ORIGIN` fallback defaults
- `frontend/*.mjs` — 12 ad-hoc script files
- `frontend/scripts/setup-auth-state.mjs` — `DEPLOYED_ORIGIN` fallback
- `load/k6/*.js` (3 files) — `BASE_URL` defaults
- `load/README.md` — documentation URL

Use a targeted search-and-replace: replace all occurrences of `draft.choir-ip.com` with `choir.news` in those files. Do a full repo grep for `draft.choir-ip.com` after changes to confirm nothing is missed.

### 13. Current docs, not historical evidence

Update current operational docs such as `AGENTS.md`, `docs/current-architecture.md`,
and `docs/runtime-invariants.md`. Do not bulk-edit historical mission logs or
evidence reports whose commands/results were actually run against
`draft.choir-ip.com`; leave those as historical records unless adding an
explicit note.

## Auth Reset

Because `choir.news` is a different WebAuthn relying-party ID, old passkeys are
not expected to work. For v0, reset deployed auth after backing up the SQLite
database:

```bash
ssh root@147.135.70.196 'set -euo pipefail
  ts=$(date -u +%Y%m%dT%H%M%SZ)
  systemctl stop go-choir-auth go-choir-proxy
  cp -a /var/lib/go-choir/auth/auth.db /var/lib/go-choir/auth/auth.db.pre-choir-news-$ts
  rm -f /var/lib/go-choir/auth/auth.db /var/lib/go-choir/auth/auth.db-wal /var/lib/go-choir/auth/auth.db-shm
  systemctl start go-choir-auth go-choir-proxy
'
```

Acceptance must record the backup filename and prove fresh registration/login
on `https://choir.news`.

## Deployment Order

1. Docs checkpoint: commit this reviewed mission update before behavior-changing code/DNS.
2. Preflight: confirm current DNS and staging health:
   - `dig +short A choir.news`
   - `dig +short A draft.choir-ip.com`
   - `dig +short A choir-ip.com`
   - `curl -fsS https://draft.choir-ip.com/health`
3. Code: make source changes above, including `choir.news` Caddy host, old-domain redirects, and deployed auth RP config.
4. Push to `origin/main`; monitor CI and Node B deploy.
5. DNS: Set `choir.news` A record → `147.135.70.196` via Gandi LiveDNS API.
6. DNS: Update `choir-ip.com` → `147.135.70.196` via Cloudflare. This remains underspecified until Cloudflare zone id, DNS record id, token, and proxied/DNS-only choice are confirmed.
7. Wait for DNS and Caddy certificate readiness.
8. Reset deployed auth database with backup as described above.
9. Verify staging identity: `curl -fsS https://choir.news/health` reports the pushed/deployed commit.
10. Verify redirects:
   - `curl -I https://draft.choir-ip.com` returns 301 to `https://choir.news{uri}`.
   - `curl -I https://choir-ip.com` returns 301 to `https://choir.news{uri}`.
11. Run deployed Playwright/API acceptance proof against `https://choir.news`, including fresh registration/login.

## Rollback

- Revert the git commit and push to `origin/main`.
- Or point `choir.news` DNS back to Gandi's placeholder IP (`217.70.184.38`).
- Restore the auth database from the `auth.db.pre-choir-news-*` backup if the hard reset must be undone.
- Old domains `draft.choir-ip.com` and `choir-ip.com` remain pointing to Node B regardless, but redirect behavior rolls back with the NixOS/Caddy deploy.

## Acceptance Evidence

Final evidence must include:

- docs checkpoint commit SHA;
- behavior-changing commit SHA pushed to `origin/main`;
- CI run and Node B deploy status for the behavior-changing SHA;
- `choir.news` Gandi record response and public `dig` result, without leaking `GANDI_PAT`;
- Cloudflare `choir-ip.com` update evidence, or a precise blocker if Cloudflare credentials are unavailable;
- auth reset backup filename;
- `https://choir.news/health` build identity;
- redirect headers for `draft.choir-ip.com` and `choir-ip.com`;
- deployed acceptance command/result proving fresh registration/login on `https://choir.news`;
- rollback refs and residual risks.

## Prior Art

This document replaces the session-level reasoning. The motivation is:
- **choir-ip.com** is a throwaway domain registered through Cloudflare; the user does not want to pay for it long-term.
- **choir.news** is registered at Gandi and is the user's preferred primary domain.
- Node A runs legacy code (choiros-rs) and will eventually be converted to NixOS running go-choir for two-node load-balanced deployment.

## Email

No Gandi-hosted email. Email will be built inside Choir using Resend for deliverability. Add Resend DNS records to choir.news zone when ready. Skip this step for v0.
