# Dead-route census: HTTP routes and API endpoints with no live caller

Repo: /Users/wiz/go-choir, HEAD c30147a6 (2026-10-12). Read-only analysis; nothing edited.

## Method

1. Enumerated every `HandleFunc(`/`Handle(` registration in non-test Go (cmd/ + internal/) with
   `git grep`, then expanded every prefix router by reading its dispatch code:
   `textureowner/router.go`, `agentcore` `HandleRunResource` / `HandleComputersRouter` /
   `HandleTrajectoryDetail` / `HandleInternalRuntimeRunRouter`, `content.HandleContentRouter`,
   `browsercontrol.HandleBrowserSessionRouter`, `solitaire.HandleSolitaireRouter`, the maild
   message/draft/attachment routers, and the proxy `HandleAPI` switch (internal/proxy/handlers.go:763-870).
2. For each route searched, with `git grep` (tracked files only): `frontend/src`, `frontend/tests`
   (+ `frontend/*.mjs`, `frontend/scripts`), `cmd/choir`, `macos/`, non-test Go clients
   (including URL-builder helpers such as vmctl `XEndpoint()` and `selfDevPath()`), `scripts/`,
   `nix/`, `.github/`, `start-services.sh`, `docs/`, and Go `_test.go` files.
3. Checked dynamic callers: the management desk's `choir.ProductAPI` allowlist
   (internal/agentcore/tools_product_api.go:166-201), the desk prompt text
   (internal/runtimeprompts/overlays/rlm_management_runtime.yaml:21-22), the desktop (Wails) app
   (cmd/desktop), Caddy routing in nix/node-b.nix (which prefix reaches which service), and exported
   frontend client functions that nothing imports (a route called only from an unimported function
   counts as no caller).
4. For "does the proxy let it through" I read the proxy dispatcher and the Caddy config.
5. Checked the two re-engineering cut definitions (cut1 authority, cut2 evidence-to-publication) for each item.

Helper scripts used (scratchpad, not in repo): `callers.sh`, `c2.sh`, `fnlen.sh` in the same directory.

## Totals

| | count |
|---|---|
| Registered route patterns (`HandleFunc`/`Handle` literals, non-test) | 217 across 15 services |
| Leaf endpoints after expanding prefix routers (method+path granularity, approximate) | about 335 |
| Live (frontend, CLI, Go client, nix/CI/systemd, external webhook, or management ProductAPI) | about 246 |
| Test-only or ops-probe-only (only Playwright specs, `scripts/*probe*`, or manual operator use) | about 20 (13 items) |
| Dead: no caller, or proxy-blocked, or always-error stub, or never mounted | about 69 (16 items) |

Counts at leaf granularity are hand-tallied from the items below; treat as +/- 5.
The 217 figure is a literal `git grep` count.

Also found: 1 dangling caller (a client whose target route was deleted), 3 allowlist/residue
strings pointing at routes that do not exist, and 2 proxy tombstones.

Already scheduled in the cut definitions (all of these are still LIVE today and were not counted dead):
- Cut 2 deletes texture merge: `POST /api/texture/documents/{id}/merge-preview` and `/accept-merge`
  (texture_merge.go, TextureCompareMergePanel, ProductAPI exception), and the Universal Wire app and
  route: `GET /api/universal-wire/stories` (proxy, FE UniversalWireApp, `choir wire stories`),
  `/internal/platform/universal-wire/stories`, `internal/platform/universal_wire.go`.
- Cut 1 deletes autonomous wire publication: `/internal/wire/platform/publications/texture` (proxy),
  `internal/proxy/wire_platform_publish.go`, `internal/agentcore/wire_platform_publish.go`,
  `internal/wirepublish`, and with them the guest-side callers of `/internal/platform/texture/sync`
  and the guest sources `/internal/texture/documents/` and `/internal/texture/revisions/`
  (cut1 doc lines 128, 645-646). Cut 1 also re-sources `/api/runtime/obligations` counts
  from fold queries (line 649) but keeps the route.
- Cut 1 replaces the Texture-control accumulator (`lifecycle_control_issue.go`) with Cast records,
  which will strand `POST /api/texture/management-open` and `GET /internal/runtime/control-records`
  (both call/audit that accumulator) even though neither route is named.

## Reading guide

Dead items are sorted by approximate size (handler plus the code that exists only to serve it; tests
excluded unless stated). Each item lists routes and handler location, what it does, why dead
(evidence), citers, scheduling, proposal, risk. "Cut 1/Cut 2/no" refers to
docs/definitions/choir-reeng-cut1-authority-2026-10-12.md and
docs/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md; I grepped both for every route
path and feature word; unless stated the answer is **no**.

---

# A. Dead items (sorted by size)

## A1. Base API: the `/api/base/*` family (6 routes), plus its commands

- **Routes**: `POST /api/base/blobs`, `POST /api/base/items`, `GET /api/base/items/{id}`,
  `GET /api/base/items/{id}/status`, `GET /api/base/delta`, `POST /api/base/repair/preview`.
  Handler: internal/base/api/handlers.go:92-97 (532 lines) and persistent.go:74 (84 lines).
  Only hosts: cmd/baseharness/main.go:152 (245 lines), cmd/baseobserve (108), cmd/basecompare (81).
  Client with no importer: internal/desktop/client.go (mirrors the delta API).
- **What it does**: a local "Base" content-addressed sync API (blobs, items, delta, repair preview) for a
  planned desktop sync client. Added 2026-07-04 (30f0301f "substrate computer landing surfaces").
- **Why dead**:
  - No deployed service mounts it. `RegisterPersistentRoutes` is called only from `cmd/baseharness` and a
    `cmd/baseobserve` test. persistent.go says "calling it from a deployed cmd service is a red mutation".
  - `git grep baseharness|baseobserve` hits nothing in `nix/`, `scripts/`, `.github/`, flake.nix.
  - Nothing imports `internal/desktop` (client).
  - The proxy still carries API-key scope strings `read:base` / `write:base`
    (internal/proxy/handlers.go:503) for `/api/base/`, which the autoputer does not serve (it would 404).
- **Citers**: tests in internal/base/api, cmd/baseharness, cmd/baseobserve, internal/desktop; docs/reviews
  (lifecycle-simplification-consensus-2026-08-29 line 100/150, rlm-implementation-consensus-2026-08-29 line 333).
- **Already scheduled?** The 2026-08-29 consensus review classed `internal/base` API + `cmd/baseobserve` +
  `cmd/baseharness` as SAFE-DELETE-NOW (~2,076 LOC). Not in Cut 1 or Cut 2 (no mention). Still present.
- **Proposal**: remove the API handlers, persistent.go, the three cmds, internal/desktop client, and the
  `:base` scope strings. KEEP `internal/base/{blob,journal,model,tree}` (imported by `internal/computerversion`)
  and `planner`. `cmd/basecompare` was marked KEEP by the review (analysis CLI); owner question whether it
  still earns its place.
- **Risk**: low. `blob/journal/model` must stay; do not delete the whole `internal/base` tree.

## A2. Solitaire play API (4 leaf routes)

- **Routes**: `POST /api/solitaire/games`, `GET /api/solitaire/games/{id}`, `POST .../moves`,
  `GET .../history`. Registered at internal/apihandler/routes.go:80-90; handler
  internal/solitaire/handler.go (158 lines); package total 911 LOC (engine 296, store 195, card 84, tests 178).
- **What it does**: a headless Klondike engine and play API built as "Candidate A" for the 2026-08 supervised
  self-development proof (a candidate change authored by the engineering desk, then mounted).
- **Why dead**: no caller in `frontend/src`, `frontend/tests`, `cmd/choir`, `scripts/` (one irrelevant PDF
  generator mentions the word), `nix/`, `.github/`. It is not in the ProductAPI allowlist. Added 2026-08-20
  (0bff0b0c, 6ea15574). The proof it served was recorded on staging epoch 333.
- **Citers**: internal/solitaire/solitaire_test.go; 54 docs files (archive/proof receipts, docs/ACTIVE.md:651).
  Go tests in agentcore use the string "solitaire" only as a prompt payload
  ("Author classic solitaire game engine"), not the route.
- **Already scheduled?** no (0 hits in either cut).
- **Proposal**: remove route, package, and the `solitaire_games` / `solitaire_moves` tables
  (internal/store/store.go:738-767). **Must also** edit internal/agentcore/replay_eligibility.go:87-88, which
  classifies those two tables as `ReplayEventProjection` (deleting tables without editing that table is
  a replay-eligibility failure). Owner question: is Candidate A kept as the canonical "self-development proof
  artifact" the metamission may re-run? If yes, keep and mark as proof-only.
- **Risk**: replay classification coupling; the old proof receipts cite the route.

## A3. Auth: account recovery, credential and session management, root status (8 routes)

- **Routes**: `POST /auth/recovery/request`, `POST /auth/recovery/verify`, `GET /auth/credentials`,
  `POST /auth/credentials/rename`, `DELETE /auth/credentials/{id}`, `GET /auth/sessions`,
  `DELETE /auth/sessions/{id}` (cmd/auth/main.go:77-83; handlers internal/auth/recovery.go, 582 lines), and
  `GET /api/auth/root/status` (cmd/auth/main.go:69; `HandleRootStatus` internal/auth/handlers.go:1086, 26 lines).
- **What it does**: M7 (2026-06-28, 1734ec06) added email magic-link recovery, multi-device passkey
  management and session listing/revocation. Root status (983b8995, 2026-08-26) reports whether the owner has a
  ROOT key and how many PRF-capable passkeys exist.
- **Why dead**:
  - **Recovery cannot work**: `HandleRecoveryRequest` creates a token with `_, err = h.store.CreateRecoveryToken(...)`
    (recovery.go:~153) and never sends it (no mailer in the file; only `log.Printf`). The user can never receive
    the link, so `/recovery/verify` cannot be reached with a real token.
  - No UI: `frontend/src/lib/SettingsApp.svelte` and `auth.js` call only `/auth/api-keys`, register/login,
    session, logout. `git log -S'auth/recovery' -- frontend/src` is empty: no frontend ever called these.
  - **Root status is unreachable**: it is registered on the auth service as `/api/auth/root/status`, but Caddy
    sends `/api/*` to the proxy (8082) not auth (8081) (nix/node-b.nix:230-250), and the proxy forwards `/api/*`
    to the autoputer, which has no such route. `git grep 'api/auth'` finds only the registration and one test.
- **Citers**: internal/auth/handlers_test.go (root status), recovery tests; the unit-test purge 2fbecebc
  removed some. Docs: none that describe a working recovery flow.
- **Already scheduled?** no ("recovery" matches in cut1 are boot-crash recovery, unrelated).
- **Proposal**: remove all 8 handlers, `recovery_tokens` table/store methods, and root-status plus its store
  helpers if nothing else uses `CountPRFCapableCredentials` / `HasOwnerRoot`. If account recovery is wanted
  eventually it needs a mailer and UI first; deleting the broken half does not lose a working feature.
  Owner question for credentials/sessions list/rename/delete only: keep as backend for a future Settings
  passkey panel? (Currently no UI exists to manage a second passkey.)
- **Risk**: `recovery.go` shares helpers (`hashIP`); confirm no other file depends on them. These are
  unauthenticated-reachable routes, so deleting also shrinks the public attack surface.

## A4. Self-development guest routes: genesis (proxy-blocked), rollbacks, kernel-capabilities, events/{digest}

- **Routes** (guest dispatcher `HandleComputersRouter`, internal/agentcore/api_self_development.go:84-235):
  - `POST /api/computers/{id}/self-development/genesis` -> `importSelfDevelopmentGenesis` (446-~740, about 290
    lines with helpers).
  - `POST .../self-development/rollbacks` -> `startSelfDevelopmentRollback` (1248, 113 lines).
  - `GET .../self-development/kernel-capabilities` -> `readKernelCapabilityReceipt` (1362, 43 lines).
  - `GET /api/computers/{id}/events/{sha256-digest}` -> `EventByDigest` (api_self_development.go:231-241).
- **What it does**: genesis imports the self-development genesis event; rollbacks starts a rollback
  operation to a prior applied head; kernel-capabilities reads the kernel capability receipt; events returns one
  computer event by digest.
- **Why dead**:
  - **Genesis is proxy-blocked**: proxy `HandleAPI` sends `.../self-development/{mode,replay-completeness,genesis}`
    to no handler (`selfDevelopmentGuestRoute` returns false for them, proxy/self_development.go:27) and the
    catch-all at proxy/handlers.go:819-821 answers 409 "self-development effects are disabled". Staging
    evidence (docs/evidence/effects-red-*.md) records "Genesis remains proxy-closed". The CLI never calls it
    (only test fixtures in cmd/choir/main_test.go mention the path); the `choir computer bootstrap-chain`
    command uses `lifecycle/bootstrap-chain` instead. Added 2026-07-18 (5483a082).
  - rollbacks, kernel-capabilities, events/{digest}: no FE, CLI, script, nix or CI caller. rollbacks has
    docs-only mention (docs/evidence/m11-episode-call-map-2026-09-27.md:24 plans it; the m11 probe script does
    not call it). kernel-capabilities has only Go tests and the proxy action table. These are reachable from the
    proxy (`rollback`, `read` actions) and from management's ProductAPI (`/api/computers/` is allowlisted,
    so GET kernel-capabilities and GET events/{digest} could be called dynamically).
- **Citers**: api_self_development_test.go (kernel-capabilities lines 493, 526); cmd/choir/main_test.go
  fixtures (genesis); docs 8-10 each.
- **Already scheduled?** no.
- **Proposal**: remove genesis (clear win: proxy-closed since 2026-08-16, 290 LOC). rollbacks / kernel-capabilities /
  events: owner question; they are part of the supervised self-development surface the metamission may keep.
  I would keep rollbacks (a documented restore leg) and kernel-capabilities (updater-backed), drop
  events/{digest} only if nobody reads it.
- **Risk**: genesis helpers may be shared with `bootstrapComputerChain`; confirm before deleting helpers.
  Operators can still call the guest route through vmctl `autoputer-proxy` with `X-Internal-Caller`.

## A5. vmctl administrative routes with no caller (9 routes)

- **Routes** (internal/vmctl/handlers.go:1697-1725, handler lengths in parens):
  `POST /internal/vmctl/hibernate` (42), `/resume` (49), `/recover` (49), `/logout` (32),
  `/hold` (24), `/divergence` (32), `/maintenance-serve` (40), `/prune` (15),
  `GET /internal/vmctl/runtime-package/autoputer` (93).
  Plus the URL helpers `HibernateEndpoint`, `ResumeEndpoint`, `RecoverEndpoint`, `LogoutEndpoint`,
  `PruneEndpoint` (handlers.go:1780-1855) and `SetAutoputerRuntimePackageDir` (handlers.go:152, cmd/vmctl/main.go:208).
- **What they do**: hibernate/resume/recover/logout/prune are the original Firecracker lifecycle operations
  (added 2026-04-11, c6ae3604; prune 2026-05-25); hold/maintenance-serve are the host-authoritative maintenance
  fence (2026-08-24/27); divergence sets platform-version divergence classification (2026-09-08);
  runtime-package streams the autoputer runtime tar to a booting guest (2026-05-24).
- **Why dead**: grep for each path string and for each `*Endpoint()` helper across Go, scripts/, nix/,
  .github/, frontend/tests: only `vmctl_test.go`/`maintenance_hold_test.go`/`divergence_test.go` mention them.
  The vmctl `Client` has no hibernate/resume/recover/logout/hold/prune method (client.go lists Resolve,
  ColdRecover, Refresh, Unhold, Lookup*, List, Pulse, Stop, Remove). `git log -S'HibernateEndpoint('` shows no
  non-test caller since creation. The runtime-package endpoint is not fetched by anything: guests run the
  baked-in `goChoirPackages.autoputer` (nix/autoputer-vm.nix:94-135 never curls vmctl for a package), and the
  only non-doc mentions are `.github/scripts/deploy-impact-classify:288-294` (a text label) and
  `cmd/runtime-ratchet` (an extinction-target id).
- **Citers**: internal/vmctl tests (vmctl_test.go, maintenance_hold_test.go, divergence_test.go,
  runtime_package_immutable_test.go); docs mention hibernate (3), resume (5), recover (7), runtime-package (15),
  maintenance-serve (10).
- **Already scheduled?** no (`hold` hits in the cuts refer to "hold" as a word; `prune`, `divergence`,
  `maintenance` have 0 hits).
- **Proposal**: remove hibernate, resume, recover, logout, runtime-package, prune (no use at all). Owner
  question for hold, divergence, maintenance-serve: these are operator break-glass endpoints (curl from the
  Node B shell) tied to the maintenance fence design; CLAUDE.md's "no-SSH operability" question suggests
  operators should not need them, but removing the fence's only entry points needs an explicit decision.
- **Risk**: operators may call these by hand via `curl http://127.0.0.1:8083/internal/vmctl/...` from
  SSH (not visible to grep). Evidence docs show operators did this for `stop`/`remove` in 2026-07 (docs/evidence/g4-*).
  `retention-plan`, `retention-shadow-plan`, `idle-check`, `reclaim` look similar but have script/nix callers
  (scripts/node-b-storage-report:482, scripts/node-b-deploy-disk-preflight:25-26, nix/node-b.nix:86): live.

## A6. Gateway operator and unused routes (8 routes)

- **Routes** (internal/gateway/handlers.go:1083-1097, handler lengths): `GET /provider/v1/search/health` (38),
  `POST /provider/v1/search/health/reset` (29), `GET /provider/v1/breakers` (15),
  `POST /provider/v1/breakers/reset` (29), `POST /provider/v1/credentials/revoke` (28),
  `POST /provider/v1/credentials/rotate` (33), `GET /health/{service}` (70),
  `GET /provider/v1/models` (internal/gateway/openai_compat.go:214, 15).
- **What they do**: search-plane health and breaker inspection/reset (2026-05-26 / 2026-06-28), credential
  revoke/rotate (2026-04-11), per-dependency health probe (`sourcecycled`, `runtime`, `qdrant`, `dolt`,
  `ollama`; M22b 2026-06-30), and the host model catalog and pools (2026-10-12, M0 model pools).
- **Why dead / no caller**:
  - credentials/revoke and /rotate: no non-test caller since creation (vmctl calls only issue and ensure,
    internal/vmctl/ownership.go:1415, 1523).
  - search/health, search/health/reset, breakers, breakers/reset: no caller in the repo; docs/problems/
    search-plane-cooldown-on-caller-cancel-2026-10-09.md:84-98 describes them as operator routes with **no caller
    check** (any computer's tap can reach port 8084). That doc proposes locking them to
    `isAuthorizedCredentialCaller`.
  - `/health/{service}`: Caddy exposes `/health/*` publicly to the gateway (nix/node-b.nix:238), but nothing in
    nix/scripts/.github/frontend/tests curls it. The gateway uses the same checker set for `/health/ready`
    (live).
  - `/provider/v1/models`: **new and intentionally unconsumed**: docs/model-pools-design-2026-10-12.md:182-186
    says the guest resolves pools from its own compiled copy and fetching from the gateway "comes later".
- **Citers**: gateway_test.go; docs/problems/search-plane-cooldown...; docs/model-pools-design-2026-10-12.md.
- **Already scheduled?** no.
- **Proposal**: remove credentials/revoke and /rotate. For search/breaker ops routes and /health/{service}:
  owner question, since they are operator tools; if kept they need the caller check from the problem doc.
  Keep `/provider/v1/models` until the model-pools design decides whether the guest fetches it.
- **Risk**: external monitoring may hit `/health/{service}` (public edge); credentials/revoke and rotate may
  be run by hand from Node B.

## A7. `internal/trace` HTTP handler: `GET /api/trace/events[/{id}]` (never mounted)

- **Routes**: `GET /api/trace/events?run_id=...`, `GET /api/trace/events/{id}`.
  `HTTPHandler.ServeHTTP` at internal/trace/query.go (218-line file; `NewHTTPHandler` line 97).
- **What it does**: owner-scoped query API over the Dolt-persisted trace event store (M20).
- **Why dead**: `NewHTTPHandler` has no non-test caller; `git log -S'trace.NewHTTPHandler'` is empty, so it was
  never mounted since it was added on 2026-06-28 (b399bd2a). No route registration for `/api/trace/` exists
  anywhere. The proxy test `handlers_test.go:994` expects 401 for `/api/trace/trajectories` only because the
  proxy auth-gates all `/api/*` before forwarding.
- **Citers**: internal/trace/query_test.go.
- **Already scheduled?** no. Cut 0/Cut 1/Cut 2 mention "trace" meaning the tape/trace export, not this API.
- **Proposal**: remove `query.go` + `query_test.go` (keep `store.go`, `redact.go`, used by agentcore/autoputer).
- **Risk**: none; confirm `Queries` type has no other user (grep: none outside the package).

## A8. Internal runtime run routes: submission stub, status, cancel, channel-casts, control-records (5 routes)

- **Routes**: `POST /internal/runtime/runs` (`HandleInternalRunSubmission`, internal/agentcore/api.go:541, 49 lines),
  `GET /internal/runtime/runs/{id}` (592, 26), `POST /internal/runtime/runs/{id}/cancel` (622, 25),
  `POST /internal/runtime/channel-casts` (652, 47), `GET /internal/runtime/control-records`
  (internal/agentcore/control_record_audit.go:14, 27).
- **What they do**: `runs` POST was the sourcecycled/processor typed-ingestion handoff; status/cancel its
  service-to-service companions; channel-casts posts an unaddressed audit cast to a channel; control-records
  lists newest control packets for the P1 staging audit (2026-10-11, 0f1c86c7).
- **Why dead**:
  - **`POST /internal/runtime/runs` always returns 400 "internal ingestion profiles are retired"** (api.go:588):
    after validation, every request falls through to that line. Its last caller (sourcecycled reconcile) was
    deleted 2026-10-05 in 3c963edf ("delete processor/reconciler pipeline end to end").
  - status/cancel: only caller was the same pipeline; Go tests only now.
  - channel-casts: added 2026-05-23 (9bad6fd6, async worker supervision); no caller in any non-test Go, script or doc;
    its own message says addressed casts must use `update_coagent`, which Cut 1 deletes.
  - control-records: single-purpose audit endpoint; no script/doc caller after the 2026-10-11 deploy receipt.
  The sibling `GET/POST /internal/runtime/runs/{id}/events` is **live** (maild/trace_events.go:123;
  scripts/texture_revision_cadence_probe.mjs:207).
- **Citers**: agentcore internal_run_*_test.go, apihandler/routes_test.go; docs/runtime-dissolution-inventory.yaml:575;
  docs/definitions/choir-ontology-kernel-2026-09-24.md:330 (stale).
- **Already scheduled?** no. Cut 1 deletes the control accumulator that control-records audits, so it will
  be stranded then.
- **Proposal**: remove submission, status, cancel, channel-casts now. Control-records: remove with Cut 1's control
  deletion (or now).
- **Risk**: `internalRuntimeRunIDFromPath`, `requireInternalRuntimeCaller` still used by the live `/events` route.

## A9. Platform key-escrow break-glass routes: unwrap requests and transparency head (3 patterns)

- **Routes**: `POST /internal/computers/keys/unwrap-requests`,
  `POST /internal/computers/keys/unwrap-requests/{id}/{approve|release}` (action router),
  `GET /internal/computers/keys/transparency-head` (internal/platform/handlers.go:429-431;
  handlers internal/platform/key_escrow_http.go:153-372, roughly 200 lines with helpers).
- **What they do**: operator-quorum break-glass for unwrapping an escrowed computer key (request,
  approve, release) and publishing the escrow transparency log head. Added 2026-08-26 (f0e68b0a).
- **Why dead**: no client anywhere. The live escrow routes (`keys/escrow`, `escrow-public-key`, `escrow/status`,
  `realization-delivery`) have callers in internal/autoputer/key_escrow.go and key_delivery.go and
  vmctl/recovery_admission.go. `unwrap-requests` and `transparency-head` appear only in platform handlers and
  `key_escrow_test.go`. Zero docs mention them.
- **Already scheduled?** no.
- **Proposal**: keep (owner question). This is the designed human-quorum recovery path; it is meant to be
  driven by an operator with `curl`, so absence of a client is by design. It is also the privacy-custody
  boundary and a red surface. Recommend "keep, document the operator runbook" rather than delete.
- **Risk**: deleting it removes the only break-glass key recovery path.

## A10. Texture API: PUT/DELETE document, diff, blame, and three 404 tombstones

- **Routes**: `PUT /api/texture/documents/{id}` (`handleTextureUpdateDocument`, textureowner/texture.go:1089, 41 lines),
  `DELETE /api/texture/documents/{id}` (1131, 28), `GET /api/texture/diff?from=&to=` (`HandleTextureDiff`, 1991, 27),
  `GET /api/texture/revisions/{id}/blame` (`HandleTextureBlame`, 2192, 36); plus the router cases that
  return a fixed 404 for `.../tell`, `.../correct`, `.../agent-revision` (textureowner/router.go:51-55, 69-71).
- **What they do**: PUT renames a document title; DELETE deletes a document; diff/blame are line-diff and blame
  views from the original e-text history UI (2026-04-11).
- **Why dead**: in `frontend/src/lib/texture.js` the client functions `updateDocument`, `deleteDocument`,
  `getDiff`, `getBlame`, `getHistory`, `createAgentRevision` have **zero importers** (checked with
  `git grep -w` across `frontend/src`); the History/Compare UI uses `/compare` and `/history` through other
  functions. No CLI caller (CLI covers create, revise, get, history, revisions, revision, events, source-open).
  No Playwright spec calls PUT/DELETE on a document (the PUTs in specs are to `/api/files/` and
  `/api/desktop/state`). Diff/blame are reachable only through management's ProductAPI GET (allowlisted prefix
  `/api/texture/`), but no prompt or doc tells a desk to use them.
  The tell/correct/agent-revision stubs return 404 either way; the `default` branch would otherwise route to the
  document handler, so they are tombstones from the 2026-09-23 "owner input is a document revision" cut (53f0ffcb).
- **Citers**: textureowner tests; internal/agentcore/tools_product_api_test.go (diff); docs 0-2.
- **Already scheduled?** no (cut2 lists merge, not these). Note `GET .../history` and `GET .../events` are live (CLI).
- **Proposal**: remove PUT, DELETE, diff, blame, and the tombstone cases, plus the unused FE functions.
  Remember "only the Texture desk writes a Texture": DELETE and PUT are Texture mutations that bypass the desk,
  so removing them also tightens that authority rule.
- **Risk**: `getHistory`/`getDiff` doc comment in texture.js still advertises them; ProductAPI GET could in
  principle call diff/blame.

## A11. maild: ingress-events and source-packet (3 routes) and the completion-email route

- **Routes**: `GET /api/email/messages/{id}/source-packet` (`handleMessageSourcePacket`, internal/maild/api.go:349, 19 lines),
  `GET`/`POST /api/email/messages/{id}/ingress-events` (369, 8; 378, 46);
  `POST /api/notifications/completion-email` (internal/maild/notifications.go:37, 48 lines) plus the proxy
  forwarder `HandleNotificationAPI` for `/api/notifications/*` (proxy/notifications.go; it still also carries
  the live `email-risk-alert`).
- **What they do**: source-packet returns the message as a source packet; ingress-events lists/records
  "owner-triggered MAS handoff" events (2026-05-26, 73beb6e1); completion-email sent "Choir finished: ..." with a
  link to `/?app=features`.
- **Why dead**:
  - ingress-events/source-packet: no HTTP client. `frontend/src` has no "ingress" string; `cmd/maildctl`
    reads the same data directly from the DB (cmd/maildctl/main.go:79-89), and `scripts/mail-acceptance-check`
    shells out to `maildctl`, not HTTP. The POST has no producer at all, so `email_ingress_events` is never written
    in production.
  - completion-email: its last caller, the Features app (`FeaturesApp.svelte`) and its proxy sender, were
    deleted 2026-07-19 in 5ae5b610. The link target `?app=features` no longer exists.
- **Citers**: maild api_test.go (4 tests), notifications_test.go; proxy/notifications_test.go:26,40.
  A stale bundle copy remains in `cmd/desktop/frontend/dist/assets/FeaturesApp-*.js` (tracked build output).
- **Already scheduled?** no.
- **Proposal**: remove all four routes plus `email_ingress_events` table/store methods; remove completion-email
  (keep `email-risk-alert`, called by agentcore/email_lifecycle.go:367). Ask owner whether `maildctl source-packet`
  should stay as an operator CLI (it works without the HTTP route).
- **Risk**: none observed; nothing external posts to these.

## A12. Proxy: `GET|POST /api/computers/{id}/recovery/job`

- **Route**: `HandleComputerRecoveryJob` (internal/proxy/computer_recovery_job.go:63, 114 lines), dispatched at
  proxy/handlers.go:807-809; forwards to corpusd `/internal/computers/projection-base/jobs`.
- **What it does**: owner-visible checkpoint/repair job surface (GET job status, POST request a repair).
  Added 2026-10-08 (0f7c58ba, red recovery mission).
- **Why dead**: no caller. FE ComputeMonitor uses `/api/compute/recovery` (a different route); CLI, scripts and docs
  have no `recovery/job`; only `internal/proxy/recovery_refusal_test.go` calls it.
- **Already scheduled?** no.
- **Proposal**: owner question: this is four days old and part of the new recovery design, probably an
  intentionally unconsumed owner surface. Keep until the recovery UI/CLI decision, then either wire a
  `choir computer recovery-job` command or delete.
- **Risk**: the corpusd `projection-base/jobs` route it fronts is live (vmctl recovery_admission.go).

## A13. Content and browser read routes (4 routes)

- **Routes**: `POST /api/content/import-file` (`HandleContentImportFile`, internal/content/content.go:263, 24 lines),
  `GET /api/content/items` (`HandleContentList`, 165, 19), `GET /api/browser/sessions` (list, browsercontrol/handler.go:170 GET branch),
  `GET /api/browser/sessions/{id}` (handler.go:253-268).
- **What they do**: import an existing user-computer file as content (2026-06-08, d3b4cb98); list content items;
  list/read browser sessions.
- **Why dead**: FE `importContentFile` has no importer (texture.js:386), no other caller; the content list has no
  GET caller (FE only POSTs `/api/content/items` from BrowserApp and GETs `/items/{id}`); BrowserApp
  uses only `POST /api/browser/sessions`, `/navigate`, `/control`, `/close` and `GET /capabilities`.
  Specs POST `/api/content/items` only.
- **Already scheduled?** no.
- **Proposal**: remove the four read/import handlers and the unused FE client functions (`importContentFile`).
- **Risk**: small; the `GET .../items/{id}` sibling is live (ContentViewer, PodcastApp, media-utils).

## A14. Dangling proxy caller: `POST {guest}/internal/texture/proposals` (route was deleted)

- **Where**: `deliverPublicationProposalToAuthor` (internal/proxy/platform_public.go:327, 53 lines) POSTs to the
  author's guest `/internal/texture/proposals`. The guest no longer registers it: internal/apihandler/routes.go:49
  says "R3d: /internal/texture/proposals removed - the desk-originated worker_updates channel is deleted".
- **Effect**: the call always fails (404), so every reader-derivative proposal is recorded with the
  constant state `recorded_for_author`, then written through `recordPublicationProposalDeliveryState` to
  corpusd `/internal/platform/proposal-deliveries/state` (a live route that now only stores a constant).
- **Callers of the outer route**: `POST /api/platform/publications/{id}/proposals` is live (FE
  `submitPublicationProposal`).
- **Already scheduled?** cut2 has 1 hit for "proposals" (orchestrator proposals, unrelated).
- **Proposal**: remove the author-delivery call and the delivery-state recording (or record the constant
  without a network call); then drop `/internal/platform/proposal-deliveries/state` if the table is write-only.
  Owner question: should proposals reach the author's Texture desk some other way after Cut 1's inbox/Cast model?
- **Risk**: product behavior is already "never delivered"; this is cleanup of a failing call, not a feature loss.

## A15. Terminal and system tombstones (proxy) plus an unwired registrar

- `GET /api/terminal/ws`: the proxy returns 410 "terminal app has been replaced by Management Console"
  (proxy/handlers.go:833-835). The autoputer's `RegisterTerminalRoutes` (internal/autoputer/terminal.go:439-443)
  has **no caller** (`git grep RegisterTerminalRoutes`: definition + doc only), so the route is not even
  registered in the guest. Replacement `/api/super-console/ws` is live (SuperConsoleApp.svelte:195).
  Already flagged in docs/reviews/rlm-implementation-consensus-2026-08-29.md:350 (TEST-MIGRATE-FIRST) and
  docs/problems/s0-guest-reaches-host-internal-authority-2026-10-04.md:157. Not in either cut.
  Proposal: remove the 410 case, `RegisterTerminalRoutes`, and the `/api/terminal/ws` assertions in
  terminal_test.go; docs/heresy-detectors.md H028 names this string as a detector, so update the detector in
  the same change.
- `/api/system/*`: proxy answers 404 for the whole prefix (proxy/handlers.go:825-827); the autoputer would 404 too,
  so the case adds nothing. One test line. Proposal: remove.
- Minor: the `/api/super-console/ws` case at handlers.go:831-832 has no `return` (Go has no fallthrough, so it
  is harmless), unlike its neighbours; tidy when editing that switch.

## A16. Small items

- `GET /internal/source-service/health` (`handleSourceServiceHealth`, cmd/sourcecycled/main.go:408, 20 lines):
  returns item/fetch counts. The gateway's dependency checker calls `sourcecycled` at plain `/health`
  (internal/gateway/config.go:73), not this path; the other three `/internal/source-service/*` routes are live
  (researchtools). Added 2026-06-04. Proposal: remove. Not scheduled.
- `HandlePublicTexture` (internal/platform/handlers.go:120): 3-line function, never registered, returns 404
  "corpusd does not render public HTML". Remove. (Renamed 2026-06-16.)
- ProductAPI allowlist and prompt residue for routes that do not exist: `/api/trace/` and
  `/api/continuations` (tools_product_api.go:189-192, rlm_management_runtime.yaml:21),
  and `run_acceptance.go:573` which emits evidence `URL: "/api/trace/trajectories/{id}/moments/{id}"`. None is
  served. `cmd/doccheck/main.go:1291` lists `/api/continuations` as a "continuation residue" term. Not scheduled.
  Proposal: drop them from the allowlist and the desk prompt (this is a prompt change: mutation class yellow,
  affects desk instructions; per the repo rules, do not tune prompts without trace evidence, but removing a
  reference to a nonexistent route is a correctness fix).

---

# B. Test-only or ops-probe-only (no production caller)

| Route(s) and handler | Only callers | What it is | Scheduled? | Proposal |
|---|---|---|---|---|
| `POST /api/evals/texture-prompt` (textureowner/api_texture_prompt_eval.go, 157 lines) | frontend/tests/texture-deployed-model-sweep.spec.js:273 | overlay-pinned Texture prompt eval for model sweeps (2026-06-17) | no | keep while model-sweep evals run; otherwise remove with the spec |
| `GET /api/runtime/obligations` (agentcore/obligations_surface.go, 250 lines) | frontend/tests/texture-acceptance-deployed.spec.js:127,267 | owner obligation-fate surface | Cut 1 re-sources it from fold queries (cut1:649), route stays | keep (Cut 1 owns) |
| `GET /api/texture/documents/{id}/export` (`HandleTextureExportDocument`, 59 lines) and `POST /api/texture/markdown-lineage/import` (181 lines) | frontend/tests/texture-markdown-lineage.spec.js | markdown export / import with lineage | no | owner question; remove with the spec if markdown lineage is not a product feature |
| `/api/shell/error` (autoputer/handlers.go:74, 11 lines) | flag-gated by `RUNTIME_ENABLE_TEST_APIS`, set only in `start-services.sh:119`, never in nix; proxy/handlers_test.go:59 | deliberate 500 for proxy passthrough tests | no | remove (unreachable in deployed computers) |
| `GET /api/run-acceptances`, `POST .../synthesize`, `GET .../{id}` (agentcore/api.go:441-541) | frontend/tests/texture-deployed-versioned-publish.spec.js, scripts/m3_vmctl_refresh_probe.mjs; management ProductAPI allowlist (live prompt text) | run acceptance records used for the final-evidence report | no | keep (it is the evidence surface per CLAUDE.md "Final Evidence") |
| `POST /api/files/sync` (autoputer/file_sync.go:790) | scripts/sh_lose_the_disk_proof.mjs:103 | synchronous file-CAS sync barrier | no | keep (durability proof) or fold into checkpoint |
| `POST /api/texture/management-open` (textureowner/management_open.go, 224 lines) | scripts/smg_rlm_acceptance_probe.mjs:125 (SMG mission 2026-10-06) | deterministic owner-side management opener that mints a lifecycle control | no, but Cut 1 replaces the control accumulator it uses | remove with Cut 1 or earlier |
| `GET /internal/diag/tcp-dial` (autoputer/boot_timeline.go:585-745, about 140 lines with http probe) | frontend/tests/s0a-reality-probe.mjs:102, scripts/s1a_refusal_matrix_probe.mjs:67 | in-guest reachability probe instrument (S0a/S1a) | no | remove with the S0a/S1a probes; it is a network-dial primitive inside the guest, host-sourced-caller gated (server/host_caller.go) |
| `GET /internal/boot/timeline` (autoputer) and `GET /internal/vmctl/boot-timeline` | frontend/tests/s0a-reality-probe.mjs:89,94; scripts/s2_station_acceptance.sh:37 | boot timeline observer (S0a) | no | keep/owner question (observability) |
| `GET /api/prompts`, `GET|PUT|DELETE /api/prompts/{role}` (agentcore/prompts.go, 239 lines) | none in code. Specs only mock/ignore it; deployed use is ad hoc `curl` (docs/evidence/sr-deployed-acceptance-2026-10-06.json). PUT/DELETE gated by `EnableTestAPIs`, never on in nix | read-only introspection of composed desk prompts and tool registry | no | keep GET (acceptance introspection); delete PUT/DELETE and the `PromptStore().Save/Reset` plumbing |
| `POST /internal/computers/platform-updates/offer`, `PUT /internal/computers/platform-updates/blob/{sha}`, `POST /internal/runtime/platform-update` | scripts/s2_station_acceptance.sh:84-101, m9a/s1a/s2 probes only; no CI or nix caller | platform-update offer mint, blob upload and apply trigger | no | keep (the update path is manual today); owner question whether deploy automation should call them |
| `POST /internal/vmctl/remove` | scripts/sh_lose_the_disk_proof.mjs:108 | drop an ownership row | no | keep (ops) |
| `GET /api/computers/{id}/self-development/operations/{id}/receipts` | scripts/m11_selfdev_episode_probe.mjs:445 | operation receipts | no | keep |

Totals for this section: about 20 leaf endpoints.

---

# C. Live families (checked, for the record)

| Family | Callers found |
|---|---|
| `/auth/register|login|session|logout|api-keys`, `/auth/desktop/{exchange-redirect,redeem}` | FE auth.js, App.svelte, SettingsApp; `choir` CLI api-keys; cmd/desktop (Wails) |
| `/api/shell/bootstrap`, `/api/ws`, `/api/super-console/ws`, `/api/desktop/state`, `/api/media/*`, `/api/preferences/theme`, `/api/podcast/*`, `/api/model-policy/resolve`, `/api/pulse/summary`, `/api/compute/{status,recovery}` | FE |
| `/api/prompt-bar[/submissions/]`, `/api/runs*`, `/api/trajectories*` (incl. capsule-evidence, events, stream, cancel), `/api/acceptance/execution-identity`, `/api/texture/{documents,revisions,lifecycle-documents,files/open}`, `/api/texture/documents/{id}/{revise,history,revisions,events,source-open,stream,manifest,compare,restore,diagnosis}`, `/api/computers/{id}/lifecycle/*`, `/self-development/{mode,replay-completeness,operations,operations/{id},decision}` , `/api/platform/*`, `/api/files/*` | FE and/or `choir` CLI |
| `/api/browser/{capabilities,sessions,sessions/{id}/navigate|control|close}`, `/api/content/{items POST, items/{id}, import-url}` | FE |
| `/api/email/*` (aliases, messages, drafts, attachments, read/unread, send, approval-email), `/api/notifications/email-risk-alert`, `/api/email/resend/webhook` | MailApp.svelte; agentcore email_lifecycle.go; Resend (external; Caddy routes it to maild 8087, the proxy deliberately 404s it) |
| `/api/mail/inbound` (autoputer) | maild drain.go |
| `/api/autopaper/sources/schedules` | management ProductAPI (prompt text) |
| `/provider/v1/inference|judgments|search|credentials/{issue,ensure}`, `/provider/openai/v1/{chat/completions,models}` | gatewayruntime clients, vmctl ownership.go, search client, autoputer terminal.go:298 (Super Console base URL) |
| `/internal/vmctl/{resolve,cold-recover,computer-version-*,lookup,lookup-guest,list,stop,refresh,unhold,pulse,autoputer-proxy,idle-check,reclaim,retention-plan,retention-shadow-plan}` | vmctl Client (proxy, maild, corpusd, sourcecycled); nix/node-b.nix:86; scripts; CI |
| `/internal/computers/*`, `/internal/platform/*` (except A9/A14/B) | computerevent http_client, projectionbase source_http, selfdev credentials, vmctl, proxy, autoputer key/file sync; nix artifact-gc timer (node-b.nix:558) |
| `/internal/runtime/runs/{id}/events`, `/internal/self-development/restart-handoff` | maild trace_events.go, updater runtime.go |
| `/internal/source-service/{search,items,ingestion-handoff/latest}` | researchtools |
| choir-updater `/v1/*`, receipt-signer `/v1/*` | agentcore and receiptsigner clients |
| `/internal/debug/pprof/` | operator diagnostics by design (docs/problems/texture-list-cold-latency-2026-10-09.md) |
| cut-scheduled but live now: `/api/universal-wire/stories`, merge-preview, accept-merge, `/internal/wire/platform/publications/texture`, `/internal/platform/texture/sync`, `/internal/texture/{documents,revisions}/` | FE UniversalWireApp, `choir wire stories`, TextureCompareMergePanel, agentcore wire_platform_publish.go |

---

# D. Cross-cutting notes for the re-engineering

1. **Frontend client residue** (`frontend/src/lib/texture.js`): exported functions with no importer:
   `pushTextureReadOwner`, `popTextureReadOwner` (no-op stubs), `updateDocument`, `deleteDocument`, `getHistory`,
   `getDiff`, `getBlame`, `createAgentRevision`, `importContentURL`, `importContentFile`, `searchPublishedTexture`.
   The file header comment still documents the dead routes. Removing a route without removing the client
   function leaves the dead code looking alive.
2. **Operator/break-glass vs dead**: several "no caller" items (vmctl hold/divergence/maintenance-serve,
   gateway ops, key-escrow unwrap, platform-update offer) are designed to be driven by an operator with `curl`
   from Node B. The `no-SSH operability` standing question makes them suspect, but their removal needs an
   explicit owner decision, not a grep result.
3. **Proxy hides guest routes**: because the proxy 409s `/self-development/*` outside its three predicates,
   and 404/410s `/api/system/`, `/api/terminal/ws`, and the Resend webhook, some guest handlers are unreachable from
   outside even though they exist. Genesis is the biggest.
4. **Stale tracked build output**: `cmd/desktop/frontend/dist/assets/FeaturesApp-*.js` still contains the
   deleted Features app and its `completion-email` call; it will keep matching greps for dead routes until the
   dist is regenerated or untracked (not a route, flagged because it shows up as a false "caller" in plain `grep -r`).
5. **Cut-doc coverage**: the cut definitions schedule only texture merge, Universal Wire, and autonomous wire
   publication among the routes in this census; everything in section A is new to the cuts.
6. **Dynamic-caller risk**: ProductAPI can reach `GET /api/texture/**` (including diff, blame, export),
   `/api/computers/**` (including kernel-capabilities, events/{digest}, rollbacks via POST), `/api/run-acceptances/**`,
   `/api/universal-wire/**`, `/api/autopaper/**`, and `/api/prompt-bar`. I found no prompt text directing a desk to
   the dead ones, but a desk could construct such a call; that is why A4 and A10 say "no caller found" rather than
   "unreachable".
