# Every Library round shows an empty catalog inside its capsule (2026-11-03)

Status: cause confirmed from code and a local run; fix follows in the next
commit.

## Problem

Self-development engineers are building a Library app package
(`frontend/src/packages/library-next`). It browses the package catalog
through `GET /api/catalog/packages`, `/api/catalog/packages/<name>/releases`
and `/api/catalog/releases/<id>`, and installs a release through
`POST /api/computers/<id>/self-development/adoptions`. The pool arm's Library
round was rejected because it had no screenshots, and the one window that
mounted showed an empty catalog with no error. Every arm's Library round
fails the same way, whatever the app does.

## Cause

`/api/catalog/*` exists only at the platform proxy. A capsule's `/api` goes to
an in-capsule autoputer, which has no catalog route.

- Production: the proxy serves the catalog.
  - `internal/proxy/handlers.go:925` sends `isCatalogPath` to `HandleCatalog`.
  - `internal/proxy/packages.go:274-318` relays the request to corpusd
    `/internal/platform/packages/catalog...` for the owner the proxy
    authenticated.
  - The corpusd routes are host only (`internal/platform/packages.go:107-127`,
    `trustedInternalCaller`): no guest reaches them, with or without its
    capability.
- Capsule: `scripts/choir-shot.mjs` sends every same-origin `/api/*` call to an
  autoputer it starts itself.
  - It does this with `isProxied` (lines 451-454) and `serve` (line 610).
  - That autoputer runs over `/computer` with `CHOIR_GUEST_TRUST_LOOPBACK=1`
    and no platform URL or credential (`autoputerEnv`, line 457), and no
    credential is allowed on a capsule.
  - Its catch-all (`internal/autoputer/computer_surface.go:33`) answers any
    unrouted `/api` path with `404 {"error":"api route not found"}`.
- The app's fetch path does not matter. `frontend/src/lib/app-sdk.ts`
  re-exports `fetchWithRenewal`, a same-origin `/api` fetch, so every client
  reaches the in-capsule autoputer.

## Evidence

The autoputer was built from origin/main `994dcfc67` and run with
choir-shot's environment: `RUNTIME_STORE_PATH`, `AUTOPUTER_FILES_ROOT`, a port,
`CHOIR_GUEST_TRUST_LOOPBACK=1` and `X-Authenticated-User: owner`.

```text
GET  /api/catalog/packages                          404 {"error":"api route not found"}
GET  /api/catalog/packages/library/releases         404 {"error":"api route not found"}
GET  /api/catalog/releases/<64 hex>                 404 {"error":"api route not found"}
POST /api/computers/c1/self-development/adoptions   403 {"error":"authenticated computer binding required"}
```

Install does not work in a capsule either. That is correct: adoption is the
owner's act on the real computer, and a capsule must not adopt.

## Options

- **(a) Read-through on the guest.** The guest autoputer would serve a catalog
  read-through to the platform. This is not available today, for two reasons.
  First, corpusd's package routes are host only, so the guest would need a new
  guest-to-corpusd read route (red, Security-Effect). Second, the capsule's own
  autoputer could still not use that route: it has no credential and no network
  path to the real computer's autoputer. A snapshot taken at capsule spawn
  would add a third red change, to the `/computer` layer.
- **(b) Fixture in choir-shot.** choir-shot answers `/api/catalog/*` from a
  fixture and labels it. It is orange and touches only choir-shot.
- **(c) Intended route.** No existing route covers this. Package share was
  retired on 2026-11-03, and adoption is an owner act at the proxy.

## Chosen: (b)

- The fixture is generated from the real corpusd handler by a Go test in
  `internal/platform`. The test publishes releases through `HandlePackages` and
  reads the three answers. The proxy relays corpusd's answer unchanged
  (`relayJSON`), so these are the `/api` shapes.
- The same test fails when corpusd's answer shape drifts from the checked-in
  fixture, so the fixture cannot silently go stale.
- choir-shot labels each answer with `x-choir-fixture: capsule-catalog`. It
  adds a visible "catalog fixture" badge to any shot whose page read the
  fixture, and logs one line for that shot.
- Listing acts (PATCH) answer 409 `capsule_fixture`. Install is unchanged:
  the capsule's autoputer refuses it, which is correct.

## Residual

- A capsule shows a fixture, not the owner's live catalog.
- Option (a) is the route if the owner wants live catalog content in
  capsules. It needs a guest catalog read at corpusd and a capture into
  `/computer`, both red.
