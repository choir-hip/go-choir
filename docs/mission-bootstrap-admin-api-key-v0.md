# Mission: Bootstrap Admin API Key v0

**Date:** 2026-06-30
**Status:** active paradoc
**Ledger:** (in-place; this is a small single-track mission)
**Source program:** this document
**Mission graph node:** `bootstrap-admin-api-key-v0`
**Parent mission:** `news-live-pr-merge-model-default-v0` (Track D requires an
API key to run the CLI against staging; this mission unblocks that)

## Mission Conjecture

If we add a one-time, env-var-configured bootstrap admin API key that is
seeded into the auth DB on startup **only when no API keys exist yet**
(first-run bootstrap), and log it loudly when it activates, then the
chicken-and-egg problem of "I need an API key to test the API but I need
the API to make an API key" is resolved without weakening WebAuthn for
normal users — because: the bootstrap key only fires on an empty auth DB,
it has a distinct label, it is logged at startup, and it can be revoked
the moment a WebAuthn-provisioned key exists. The product-visible outcome
is that a headless agent (the choir CLI) can authenticate to staging
without a browser, which is the evidence gate for the parent mission's
Track D.

The load-bearing bridge: **a first-run-only bootstrap key is the minimal
auth-surface addition that unblocks headless API access while preserving
the WebAuthn-authorizes-machines design.**

## Deeper Goal (G)

The audited computer requires that every mutation transaction has an
author. WebAuthn is the right auth for humans in browsers; API keys are
the right auth for machines. But the first machine key cannot be created
by a machine — that is the bootstrap problem. This mission adds the
minimal escape hatch that lets a human operator seed the first key from
config (env var on the platform/host), after which the normal
WebAuthn-provisioned-key flow takes over.

## Operating Model

Single track, single agent. Red class — full ceremony.

## Invariants / Qualities / Domain Ramp (I/Q/D)

**Invariants (never optimize across):**
- No weakening of WebAuthn for normal users (the bootstrap path is
  additional, not a replacement)
- No silent activation: the bootstrap key logs loudly at startup when it
  fires, with its key ID and label
- No persistence of the raw key: only the SHA-256 hash is stored (same
  as WebAuthn-provisioned keys)
- First-run-only: the bootstrap fires only when the auth DB has zero API
  keys. If any key exists (WebAuthn-provisioned or otherwise), the
  bootstrap is a no-op and logs that it skipped.
- Revocable: the bootstrap key is a normal `api_keys` row, revocable via
  the existing revoke flow
- The raw key is never logged, never written to disk beyond the env var

**Qualities:**
- The bootstrap key has `admin` scope (full access) so it can provision
  other keys and verify any route
- The env var is read once at startup; changing it requires a restart
- Tests cover: first-run seeds the key, second-run is a no-op, the key
  validates via the proxy, revoking it disables it
- The env var name is documented and gitignored (lives in `.envrc.local`
  or host systemd config, never in tracked files)

**Domain ramp:**
- Wave 1: implement + test locally
- Wave 2: land to main, deploy to staging
- Wave 3: use the bootstrap key to run the choir CLI proof on staging
- Critical path: this mission is on the critical path for the parent
  mission's Track D staging proof

## Variant (Conjecture Descent) V

**Initial conjectures:**

- C1 (bootstrap seeds): "Reading `CHOIR_BOOTSTRAP_ADMIN_API_KEY` at
  startup and seeding it when the auth DB has zero API keys works without
  breaking existing WebAuthn flows." — undecided
- C2 (first-run-only): "The bootstrap is a correct no-op when any API key
  already exists, so reboots after first provisioning do not create
  duplicate or shadow keys." — undecided
- C3 (proxy validates): "The bootstrap key validates through the existing
  proxy API key path (Bearer header -> SHA-256 -> lookup) with no special
  casing." — undecided
- C4 (revocable): "Revoking the bootstrap key via the existing revoke
  flow disables it for future requests, identical to a WebAuthn-provisioned
  key." — undecided

**V = 4** (all undecided)

## Budget

**Granted:** small (single-track red-class mission)
**Spent:** 0
**Remaining:** full
**Solvency:** 4 conjectures in one track is feasible.

## Authority / Bounds

May implement the bootstrap in `internal/auth` (store + startup wiring).
May add the env var read to the sandbox/proxy startup path. May NOT
change the WebAuthn flow, the proxy's existing API key validation logic
(only the seeding), or the scope model. Must push to main and verify on
staging.

## Mutation Class / Protected Surfaces

**RED.** Protected surfaces: auth/session renewal, the auth DB schema
(uses existing `api_keys` table — no schema change), startup bootstrap.
No gateway/provider calls, no VM lifecycle, no Texture writes.

**Rollback path:** revert the commit; the seeded key row remains in the
DB but is harmless (it's a normal revocable key). If the raw key leaked,
rotate it (change the env var, restart, revoke the old row).

**Heresy delta:**
- `discovered`: 1 (the headless-auth memo left the bootstrap path
  unspecified; this is the gap)
- `introduced`: TBD (the bootstrap key is a new auth path — verify it
  does not contradict doctrine)
- `repaired`: 0

## Evidence Packet

- Local tests: first-run seeds, second-run no-op, proxy validates,
  revoke disables
- CI green on the PR
- Staging deploy identity (commit SHA on choir.news)
- `choir wire stories --api-key=$CHOIR_BOOTSTRAP_ADMIN_API_KEY` returns
  a response (not 401) on staging — this is the deployed acceptance proof

## Implementation Sketch

1. `internal/auth/store.go`: add `SeedBootstrapAdminAPIKey(ctx, rawKey,
   label) error` that:
   - Checks if any API key exists (`SELECT COUNT(*) FROM api_keys WHERE
     revoked_at IS NULL`). If >0, log "bootstrap skipped: API keys
     already exist" and return nil.
   - Hashes the raw key (SHA-256), generates a UUID key ID, inserts a
     row with `scopes='["admin"]'`, `label="bootstrap-admin"`, no expiry.
   - Logs "bootstrap admin api key seeded: key_id=... label=bootstrap-admin"
     (never the raw key).
2. Startup wiring: in the sandbox/proxy startup (wherever the auth store
   is opened), if `CHOIR_BOOTSTRAP_ADMIN_API_KEY` is set and non-empty,
   call `SeedBootstrapAdminAPIKey`. This runs once per process start; the
   first-run-only guard makes repeated starts safe.
3. Tests in `internal/auth/apikeys_test.go`: cover the four conjectures.
4. Document the env var in the headless-auth memo (append a "Bootstrap"
   section) and in AGENTS.md or a deploy note.

## References

- `docs/memo-headless-auth-choir-base-artifact-program-2026-06-28.md` —
  the API key design (WebAuthn-provisioned; bootstrap path unspecified)
- `internal/auth/store.go` — `api_keys` table, `APIKeyPrefix`,
  `GetAPIKeyByHash`, existing key creation
- `internal/auth/handlers.go` — WebAuthn-provisioned key creation flow
- `internal/proxy/handlers.go` — `validateAPIKey` (the path the bootstrap
  key must validate through, unchanged)
- `docs/mission-news-live-pr-merge-model-default-v0.md` — parent mission,
  Track D (the CLI proof this unblocks)
