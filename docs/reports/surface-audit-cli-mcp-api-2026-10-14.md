# Surface Audit: CLI, MCP and HTTP Routes, With Recommended Cuts

Date: 2026-10-14. Base: `origin/main` at 7a8ad165c. Branch `surface-audit`, not
pushed. Mutation class: **green** (a report; no code, route, scope or deploy
change). Evidence class: static reading of the tree, plus twelve
unauthenticated GETs against `https://choir.news` (listed in §7). Nothing was
written to staging.

The owner suspected the command and endpoint surface had grown past what is
used, with duplicates, dead routes and broken calls. It has. The suspicion
started with `choir self-dev start`, which posts to a guest route that C1e B1
(a06191f9e) deleted and gets 405. That command is the clearest case, but it is
not alone. There are three more client-to-route mismatches that fail today on
the product path, one of them in the Texture editor's Save. About sixty
endpoints have no caller outside their own tests. Five places offer two or more
routes for the same act. Owner acts are fenced off MCP but not off API keys.

This audit builds on the dead-route census of two days ago
(`docs/reports/dead-code-census-2026-10-12/routes.md`). Some of what that census
named is now gone: the Base API commands, the solitaire routes, `/api/trace`,
the internal channel-cast and control-record routes, the gateway credential
revoke and rotate routes, and the maild ingress, source-packet and
completion-email routes. Others are still present, and this report repeats them
where they still matter: `recovery/job`, the proposal delivery to
`/internal/texture/proposals`, the auth recovery and session routes, and the
vmctl administrative routes.

## 1. Counts

| Service | What is served | Method+path endpoints | No non-test caller | Broken today | Latent broken |
|---|---|---|---|---|---|
| CLI `choir` | one registry, `cmd/choir/commands.go` | 59 commands, 46 MCP tools (58 and 45 on `selfdev-land`) | 1 (`self-dev start`, which cannot work) | 2 | 0 |
| proxy (public edge, :8082) | 15 registered patterns, about 50 dispatch targets in `HandleAPI` | about 50 | about 10 | 2 | 0 |
| guest / autoputer (one binary; `apihandler` plus `internal/autoputer`) | about 116 | about 116 | about 13 | 5 | 0 |
| corpusd (:8086) | 54 registered patterns | about 62 | about 13 (several operator break-glass) | 1 (unreachable route) | 1 |
| vmctl (:8083) | 26 | 26 | 4, plus 2 runbook-only | 0 | 1 (scripts without hostauth) |
| gateway (:8084) | 15 | 15 | 1, plus 2 break-glass | 1 (`/health/ready`) | 0 |
| maild (:8087) | 15 | 15 | 2 | 0 | 0 |
| auth (:8081) | 19 | 19 | 8 (one of them unreachable) | 0 | 0 |
| choir-updater, receipt-signer (unix sockets) | 4 + 3 | 7 | 0 | 0 | 0 |
| goproxy (:8088) | GOPROXY and sumdb path families, `/healthz` | about 5 families | `/healthz` | 0 | 0 |
| checkpointd, front-importance, capsule-broker, zot, platformrelease | no HTTP server | 0 | — | — | — |
| **Total** | | **about 330** | **about 60** | **11** | **3** |

"No non-test caller" means no caller in `frontend/src`, `cmd/choir`, `scripts/`,
CI, `nix/`, another service's Go client, or the management desk's
`choir.ProductAPI` allowlist. The counts are approximate (±5), because prefix
routers are expanded by reading their dispatch code. The 2026-10-12 census
counted about 335 leaf endpoints, so the surface has barely shrunk. Its
deletions were offset by new routes.

## 2. Broken: a caller and its route disagree

These are the items where a client sends a request that cannot succeed. They
are ordered by how much they hurt now. "Static" means the mismatch is proven by
reading the client and the handler. "Live" means a staging response shows it.

1. **`choir self-dev start` and its MCP tool `self_dev_start`.** They POST
   `/api/computers/{id}/self-development/operations` (cmd/choir/commands.go:568-578).
   The proxy still maps that POST to a "propose" action and forwards it
   (internal/proxy/self_development.go:31, :61, and a 1 MiB body allowance at
   :396). The guest answers 405 (internal/agentcore/api_self_development.go:159-165,
   with the comment "C1e B1: the public start-operation API is gone"). The same
   POST also appears in `scripts/m11_selfdev_episode_probe.mjs:432` and `:534`,
   and in `skills/choir-cli/SKILL.md`. **Status:** `selfdev-land` (afc8a85b9,
   unpushed) deletes the CLI command, the tool and the skill line. It does not
   touch the m11 probe, the proxy's propose mapping, or the
   `computer:self_development:propose` scope, which now guards nothing (§4).
   Live: a GET on `…/operations/y/decision` returns 405 before auth, which
   confirms the proxy predicate is still installed.

2. **Saving in the Texture editor is refused with 400 (static, high
   confidence).**
   - **Client:** `createRevision` in `frontend/src/lib/texture.js:117-123`
     always sends `author_kind` and `author_label`. TextureEditor.svelte passes
     `authorKind: 'user'` at :694, :928 and :1085.
   - **Server:** POST `/api/texture/documents/{id}/revisions` decodes with
     `DisallowUnknownFields` (internal/textureowner/texture.go:1037). Its
     request type `textureCreateRevisionRequest` (texture.go:188-198) has
     neither field, so the decoder rejects the body with "invalid request body".
   - **Confirm:** one authenticated save on staging. Without an owner session
     this audit cannot.
   - **Fix:** on the client. The server dropped those fields on purpose: the
     public route always writes user-authored revisions.

3. **Proposal delivery to a publication's author always falls back.** The proxy
   POSTs to the guest's `/internal/texture/proposals`
   (internal/proxy/platform_public.go:382). R3d deleted that route
   (internal/apihandler/routes.go:65). The guest's SPA catch-all answers POST
   with 405, so `deliverPublicationProposalToAuthor` always records
   "recorded_for_author" through corpusd `/proposal-deliveries/state`. The
   2026-10-12 census flagged this as A14, and it is still present.

4. **The management desk's ProductAPI calls to `/api/computers/*` always
   return 403.**
   - **Allowed:** the allowlist includes `/api/computers/`
     (internal/agentcore/tools_product_api.go:186), and the management overlay
     offers it.
   - **Missing header:** `productAPIRequest` sets only `X-Authenticated-User`
     and `-Email` (:69-71).
   - **Refused:** the computers router requires `X-Authenticated-Computer` to
     equal the path id (api_self_development.go:84), so it answers
     "authenticated computer binding required".
   - **Fix:** set the header to the desk's own computer, or drop the
     allowlist entry.

5. **GET `/api/computers/{id}/events/{digest}` cannot be reached.** No proxy
   predicate matches it, so it takes the generic forward (proxy/handlers.go:872).
   That forward never sets `X-Authenticated-Computer`, so the guest answers 403.
   It has no caller anyway: DELETE.

6. **`choir texture create --desk engineering` is documented but refused.** The
   CLI help says "texture (default) or engineering" (commands.go:205). The
   server returns 400 "desk must be texture"
   (internal/textureowner/texture_lifecycle_create.go:89-90).

7. **The Texture document list ignores `?limit`.** GET `/api/texture/documents`
   hard-codes 50 (texture.go:804). Scripts that ask for 500 or 100 are silently
   truncated:
   - `scripts/l2a_fixture_run.py:100`
   - `l2a_paper_window.py:74`
   - `l2a_window_pairs.py:141`, which already notes the loss at :134
   - `management_latency_probe.py:355`
   - `reeng_tracer.mjs:926`

8. **Unknown guest GET `/api/*` returns 200 with HTML.** The guest has no
   `/api/` 404 handler, so a removed GET route falls through to the SPA
   `index.html` (internal/autoputer/computer_surface.go:28-66). A stale client
   gets a JSON parse error, not a 404. That hides exactly the kind of breakage
   this audit is about. FIX: register a JSON 404 for `/api/`.

9. **Gateway `/health/ready` is always "degraded" (live).** Caddy sends public
   `/health/*` to the gateway (nix/node-b.nix:381). The gateway probes
   `runtime` and `dolt` at `127.0.0.1:8085`, and `ollama` at `:11434`
   (internal/gateway/config.go:73-78). Nothing listens on either port on Node B,
   because computers run in VMs now. Staging answered `"status":"degraded"` with
   `connect: connection refused` for all three. The check cannot go green, so
   it signals nothing, and its error strings name host ports to the public.

10. **corpusd GET `/internal/platform/texture/documents` always 404s.** The
    route is registered on the exact path (internal/platform/handlers.go:507),
    but its handler reads a document id from the path (:345-353).
    `/documents/{id}` goes to the revisions handler instead (:506), so
    `GetPlatformTextureDocument` is unreachable. The route has no caller:
    DELETE it together with the Store A mirror (§3).

11. **`choir computer recover` describes a different act than it performs.**
    Its summary is "Run the owner recovery job for the current computer"
    (commands.go:490). The route it calls, `lifecycle/recover`, clears the
    maintenance hold and starts the computer
    (internal/proxy/computer_lifecycle.go:162-172). The owner recovery job is
    the separate `/api/computers/{id}/recovery/job` (computer_recovery_job.go),
    which nothing calls. FIX the description (§5 merges these routes).

**Latent: works today, breaks when hostauth enforces.** Several scripts call
host-internal routes with only `X-Internal-Caller` and no `X-Choir-Internal-Token`.
They work because `internalAuthMode = "observe"` (nix/node-b.nix:67), and will
get 403 when it flips to `enforce`. One of them runs on every deploy:
`scripts/node-b-deploy-disk-preflight:25-26` (vmctl reclaim and idle-check,
called from ci.yml:1137). The others:
- vmctl: `vmctl_receipt_probe.mjs`, `vmctl_no_escalation_probe.mjs`,
  `vmctl_wedge_recovery_probe.mjs`, `sh_lose_the_disk_proof.mjs`,
  `s3_snapshot_probe.py`, `frontend/tests/s0a-reality-probe.mjs`,
  `texture-acceptance-deployed.spec.js:246`
- corpusd: `scripts/lib/selfdev_approval.mjs:78-83`,
  `m11_selfdev_episode_probe.mjs:135-155`

Only ci.yml (:918-920) and node-b.nix:195 attach the token. The fix is to make
the preflight script send it before anyone flips the mode.

**Not broken, despite a claim to the contrary.** One sub-audit reported that
the guest's POST `…/self-development/rollbacks` could never run because
`rt.selfdevRoute` is never set. It is set (internal/agentcore/runtime.go:760).
The route is merely uncalled (§3).

**What is not broken.** Every `vmctl.Client` method, the gateway clients, every
proxy-to-maild and proxy-to-corpusd forward, and the corpusd clients
(computerevent, selfdev credentials, projectionbase, autoputer, researchtools)
match their handler's path and method. Apart from items 1 to 11, no Go client
calls a deleted route.

## 3. DELETE: dead or duplicate, ranked

Ranking: risk removed first, then confusion removed, then lines removed. Every
item gives its evidence of no caller. Where a route is a break-glass operator
tool, it says so and asks the owner rather than deleting.

1. **`self-dev start`, end to end.**
   - **Remove:** the CLI command and MCP tool (land `selfdev-land`); the
     proxy's `"operations"`-POST "propose" mapping and its 1 MiB body
     allowance (proxy/self_development.go:31, :396); the two POSTs in
     `scripts/m11_selfdev_episode_probe.mjs`.
   - **Scope:** retire `computer:self_development:propose` or give it a real
     meaning (§4).
   - **Evidence:** the guest returns 405; nothing else posts there.

2. **vmctl `GET|POST /internal/vmctl/autoputer-proxy/{owner}/…`**
   (internal/vmctl/handlers.go:1108). This is a generic reverse proxy into any
   owner's computer. Its only callers are in platform_computer_test.go, and the
   last product caller went around a47122de0. An unused route that can reach
   every tenant is the largest exposure on the list.

3. **Auth account-recovery, credential and session routes:**
   `POST /auth/recovery/request`, `/auth/recovery/verify`, `GET
   /auth/credentials`, `POST /auth/credentials/rename`, `DELETE
   /auth/credentials/{id}`, `GET /auth/sessions` and `DELETE
   /auth/sessions/{id}` (internal/auth/recovery.go). Only tests call them, as in
   census A3. `recovery/request` is an unauthenticated database write that
   mints a token and never emails it. Delete them, or build the Settings UI
   that would call them. Also delete `GET /api/auth/root/status`
   (auth handlers.go:1086), which cannot be reached: Caddy sends `/api/*` to
   the proxy, which forwards it to the guest.

4. **The proxy's `/internal/platform/*` pass-through**
   (proxy/handlers.go:1732-1739, 1744). It covers `objects`, `objects/`,
   `edges`, `captures`, `captures/bodies/`, `captures/` and
   `support-verdicts`.
   - **Legacy:** guests reach corpusd directly through
     `CHOIR_PLATFORM_URL=http://<host>:8086` (vmmanager/manager.go:1725,
     provideriface/config.go:119). agentcore posts verdicts straight to the
     platform URL (story_publish.go:338). The pass-through duplicates that.
   - **Weaker check:** its bound-guest check tests the owner only
     (guest_authority.go:113-118). corpusd tests owner and computer
     (objectgraph_handlers.go:361-366).
   - **Keep:** `StoryPublishPath`, which is a real guest-to-proxy publish.

5. **corpusd's Store A Texture mirror and dead object-graph routes.**
   - **Mirror:** `POST /internal/platform/texture/sync`,
     `GET /internal/platform/texture/documents`,
     `GET …/texture/documents/{id}[/revisions]` and
     `GET …/texture/revisions/{id}` (platform/handlers.go:313-399). The last
     caller of the sync went in dca5154eb. One route is unreachable (Broken
     10), and the revisions read is unbounded (store.go:478).
   - **Object graph:** `GET /internal/platform/objects/{id}`, `GET|POST|PUT
     /internal/platform/edges`, and `POST|PUT /internal/platform/objects`.
     Their only client, `objectgraph.HTTPStore`, has had no non-test
     constructor since 03964b064.
   - **Superseded:** `GET /internal/platform/captures/bodies/{sha}`, replaced
     by `captures/{id}` in 4ea25d284.
   - **Delete with them:** the dead `objectgraph.HTTPStore` client.

6. **The proposal-delivery residue.** This means the proxy's
   `deliverPublicationProposalToAuthor` call to the deleted guest route
   (Broken 3), corpusd `POST /internal/platform/proposal-deliveries/state`
   (which only ever records the constant fallback), and the guest's
   `/internal/texture/documents/{id}` and `/internal/texture/revisions/{id}`
   (textureowner/texture.go:846, :873).
   - **Callers:** the two guest routes are called only by
     apihandler/routes_test.go:58-59.
   - **Duplicate:** they repeat `/api/texture/...`.
   - **Weak gate:** they are guarded only by a forgeable `X-Internal-Caller`
     header (texture.go:48-53).

7. **Guest `POST /api/mail/inbound`** (internal/autoputer/maildir.go:213).
   Its sender, the maild spool drain, was removed in 8e060b98b. It is still
   reachable by any owner session or any API key with `write:runtime`, which
   makes it a mail-injection path with no purpose.

8. **maild `POST /api/notifications/email-risk-alert`, plus the proxy's
   `/api/notifications/*` forwarder** (proxy/notifications.go:8,
   handlers.go:869). The last caller, agentcore/email_lifecycle.go, was
   deleted in 9679837f3. Also delete maild `DELETE /api/email/messages/{id}/read`,
   which duplicates `POST …/unread` and is uncalled, and maild's guest-caller
   path (`s1aBindTapCallerToOwner`, `maildLookupGuestOwnership`), because no
   guest uses `RUNTIME_MAILD_URL` any more.

9. **Uncalled self-development routes and scopes.**
   - **Guest kernel-capabilities:** `GET …/self-development/kernel-capabilities`
     (api_self_development.go:153) is tests only. The proxy still forwards it.
     The updater's `/v1/kernel-capabilities` on the unix socket is a different
     route and stays.
   - **Guest events read:** `GET /api/computers/{id}/events/{digest}` is
     unreachable (Broken 5).
   - **Genesis:** the proxy's `"genesis"` exclusion (self_development.go:29)
     and the `computer:self_development:genesis` scope. Auth issues that scope
     and SettingsApp.svelte:48 offers it in the key form, but nothing checks it.
   - **Base:** the proxy's `read:base` and `write:base` scope mapping
     (proxy/handlers.go:517). No service serves `/api/base/` any more; the
     selfdev-readiness report's recommended agent key still lists `read:base`.
   - **Owner ruling:** `POST …/self-development/rollbacks`
     (api_self_development.go:168). It is uncalled, but rollback may be the
     intended owner undo; `computer restore` is the one in use. Either give it
     a CLI owner command or delete it. Census A4 asked the same question.

10. **The proxy's `/api/computers/{id}/recovery/job` (GET and POST)**
    (proxy/computer_recovery_job.go). It has no frontend, CLI or script
    caller, only a test (census A12, two days ago, still present). Also
    delete:
    - **vmctl:** `POST /internal/vmctl/divergence`,
      `GET /internal/vmctl/retention-plan` and
      `GET /internal/vmctl/retention-shadow-plan` (tests or none), plus four
      URL helpers nothing references (`IdleCheckEndpoint`, `ReclaimEndpoint`,
      `RetentionPlanEndpoint`, `RetentionShadowPlanEndpoint`,
      handlers.go:1382-1404).
    - **gateway:** `GET /health/{service}`. It is public and uncached, and
      each hit makes a 3-second outbound probe.

**Further deletions.** These are smaller, all with no caller or test-only
callers:
- **Guest Texture tooling:** `POST /api/texture/markdown-lineage/import`
  (a one-off migration) and `GET /api/texture/documents/{id}/export`. Both are
  called only by `texture-markdown-lineage.spec.js`.
- **Guest test plumbing:** `PUT|DELETE /api/prompts/{role}`, which is
  test-gated write plumbing; `/api/shell/error`, test-gated; and the GET
  branch of `/internal/runtime/runs/{id}/events`, which duplicates
  `/api/runs/{id}/events`.
- **`POST /internal/runtime/platform-update`.** It has had no caller since
  b9f195b1c deleted the station probes. Wire it into deploy or delete it, as
  an owner ruling.
- **corpusd:** `DELETE …/projection-base/pins` (uncalled) and
  `GET …/keys/transparency-head` (uncalled; an operator ruling).
- **CLI:** `choir precommit import-ledger`, a one-time migration; delete it
  once the level-1 ledger import has been applied.

**Gate rather than delete:** `POST /api/evals/texture-prompt` is live in
production with only a spec caller, and it creates a document and a run with
any model-policy overlay. Put it behind `enableTestAPIs`, as `/api/shell/error`
already is.

**Operator break-glass routes, to keep but list in one runbook:**
- vmctl `maintenance-serve` and `hold` (runbook only; `hold` should log its
  caller)
- gateway `search/health/reset` and `breakers/reset`
- corpusd `keys/unwrap-requests*`, `platform-updates/offer`, PUT
  `platform-updates/blob` and `artifact-gc`

They have no code caller by design. The unwrap routes need a transport fix
(§6).

## 4. FIX: wrong, but the route should stay

- **Owner acts are fenced off MCP, not off API keys.**
  - **What happened:** 5b17bb572 made `self-dev approve`, `mode set`,
    `computer restore`, `bootstrap-chain` and `rematerialize-from-tape`
    CLI-only.
  - **Still open over HTTP:** any key carrying
    `computer:self_development:approve`, `:mode` or `:rollback`, or `admin`,
    passes the proxy (self_development.go:140-145, :349-353). The
    test-computer mint script grants all five self-development scopes to one
    key (scripts/qa_test_computer.mjs:34-36).
  - **The stricter precedent:** mail send is session-only, and the proxy
    refuses API keys on it (proxy/email.go:30).
  - **Two fixes:**
    1. *Cheap:* split the mint into an agent key and an owner key, as the
       selfdev-readiness report recommends.
    2. *Structural:* make `decision` and mode `PUT` require an owner session,
       or a key the owner minted interactively with an explicit owner-act
       flag. Then `admin` stops being a silent approval scope.
  - `ownerActReason` also names "the Choir UI" as an approval path; none
    exists.

- **`computer:self_development:propose` guards only the deleted route.** The
  real proposal authority is `write:runtime` on `/api/prompt-bar`. Either retire
  the scope or require it on prompt-bar requests that will reach engineering on
  an armed computer.

- **The proxy does not strip a client-supplied `X-Authenticated-Computer`.**
  The generic forward removes a list of identity headers
  (proxy/handlers.go:43-54) that does not include it. The guest's computer
  router trusts that header as the binding (api_self_development.go:84). The
  impact today is small, because the self-development and lifecycle paths are
  intercepted before the generic forward and the forward goes to the caller's
  own resolved computer. Still, a forgeable binding header is a substrate
  hole. Add it to `clientIdentityHeaders`.

- **`GET /auth/api-keys` has no scope check and no limit.** A zero-scope or
  precommit-only key can list every key the owner has (metadata only)
  (auth handlers.go:1787). Require `manage:keys`, or return only the calling
  key. Auth also accepts zero-scope keys.

- **Run cancel has no idempotency key or CAS.** POST `/api/runs/{id}/cancel`
  has neither; POST `/api/trajectories/{id}/cancel` has both. See §5.

- **`/api/costs` skips the host-source check.** It trusts
  `X-Authenticated-User` without the `HostSourcedCaller` check every agentcore
  route makes (apihandler/handler.go:29). The textureowner, content, browser,
  desktop and media handlers do the same, through five copies of
  `authenticateUser` that have drifted from agentcore's. Because
  `HostSourcedCaller` admits loopback, even agentcore's check admits any
  in-guest process (terminal, capsule workload) as the owner. That is the
  deeper issue: owner identity inside the guest is a header any local process
  can set. It belongs in the capsule-confinement work, not a route patch.

- **Search only covers recent publications.** corpusd retrieval search, behind
  public `GET /api/platform/retrieval/search` and `choir search`, reads only
  the 100 most recent publications (platform/service_publication_read.go:569).
  Older ones are silently missed. Staging returned `{"results":[]}` for
  "choir".

- **Gateway `/health/ready`.** Drop the dead `runtime`, `dolt` and `ollama`
  probes (Broken 9), or stop routing public `/health/*` to the gateway.

- **`/api/files/sync` overlaps the file namespace.** When the sync service is
  absent, POST creates a directory named `sync`. When it is present, a root
  file named `sync` cannot be read (internal/autoputer/file_sync.go:790). Move
  it under a non-file prefix.

- **Unbounded lists:**
  - guest: `GET /api/run-acceptances` (limit uncapped), `GET
    /api/calibration?detail=1`, `GET …/self-development/operations` and `GET
    …/lifecycle/producer-reports`
  - corpusd: `GET /internal/platform/objects` and `/edges` (client-chosen SQL
    LIMIT), and `GET …/projection-base/pins`
  - vmctl: `GET /internal/vmctl/list`
  - Texture revisions allow 10,000 full bodies, and both the frontend and
    `choir trace export` ask for 10,000.

- **Writes without an idempotency key:**
  - POST `/api/texture/documents`
  - `/api/content/items`
  - `/api/browser/sessions`
  - `/api/prompt-bar` when `command_id` is omitted (the CLI always sends one;
    the frontend should too)
  - `lifecycle/replace-workspace`, `bootstrap-chain`,
    `import-residue-snapshot` and `checkpoint`
  - corpusd `…/publications/{id}/proposals`, which mints a new proposal on
    every call
  - gateway `credentials/issue`
  - forced vmctl `/refresh`

- **Uncapped request bodies:** the podcast subscription POST, the
  source-schedules POST, the computer-settings POST, the prompts PUT and
  run-acceptances/synthesize.

## 5. MERGE: one act, several routes

- **Making a computer go.** There are five routes:
  - `POST /api/compute/recovery`: wake or resume, with the frontend as its
    caller; scope `write:runtime`
  - `lifecycle/start`
  - `lifecycle/recover`: unhold, then start
  - `lifecycle/cold-recover`: script callers only
  - `recovery/job`: no caller

  They use two different scopes for the same act. Keep `lifecycle/{start,
  stop, restart, refresh}` as the owner verbs, fold the unhold into `start`
  with a flag, and move the frontend onto `lifecycle/start`. Keep `cold-recover`
  as a vmctl-backed operator route. Delete `recovery/job`.

- **Texture document creation.** There are five entry points:
  - `POST /api/texture/documents`: no lifecycle, no idempotency; the frontend
    uses it
  - `/api/texture/lifecycle-documents`: idempotent; the CLI uses it
  - `/api/texture/files/open`
  - `/api/prompt-bar`
  - `/api/evals/texture-prompt`

  The doctrine says a Texture starts through the prompt bar and only the
  Texture desk writes one. Converge on `lifecycle-documents` (or prompt-bar)
  for creation and keep `files/open` as the file alias.

- **Texture reads.** In the CLI, `texture read` and `texture show` both GET
  the document, and `texture history` (metadata, 50) is a narrower
  `texture revisions` (bodies, up to 10,000). Merge `read` into `show`, and
  `history` into `revisions --metadata` with a real `limit`. On the server,
  `/stream` (SSE) and `/events` (cursor) read the same events; keep both,
  since the frontend uses SSE and the CLI uses the cursor.

- **Trajectory and run reads.**
  - `choir lifecycle snapshot` and `choir trajectory` call the same route,
    GET `/api/trajectories/{id}`; the first only adds a schema check. Merge
    them.
  - `/api/prompt-bar/submissions/{id}` is a filtered view of `/api/runs/{id}`.
  - `run cancel` and `trajectory cancel` differ only in that the second is
    safe. Keep `trajectory cancel` and make `run cancel` resolve the run's
    trajectory and call it.

- **Two calibration stores.** `/api/calibration` plus `/settlements` (desk
  self-digests, `write:runtime`) and `/api/precommit/calibration` plus claim
  settlements (outside agents, `precommit:write`) both write settlement
  commitment records. Unify them under one route family and one scope, or
  rename one so a reader can tell them apart.

- **vmctl `idle-check` and `reclaim`** run the same sweep (handlers.go:784-831).
  Keep `reclaim` and move the preflight script's fallback onto it.

- **Gateway model lists:** `/provider/v1/models` and
  `/provider/openai/v1/models`. Keep the OpenAI-compatible one that zot uses.

- **Build identity is exposed three times:** guest `/health`,
  `/api/runtime/observability` and `/api/acceptance/execution-identity`. That
  is defensible, because only the last is signed. But vmctl's identity check
  should read the signed one.

## 6. Risky: keep, but harden

- **corpusd key unwrap is gated by operator headers alone.** corpusd listens
  on `0.0.0.0:8086` (nix/node-b.nix:765), and vmmanager makes that port
  reachable from guest taps. `keys/unwrap-requests*` checks only
  `X-Choir-Operator` and its token (platform/key_escrow_http.go:297-309),
  with no transport or hostauth check, and `reveal` returns the plaintext key.
  Add the trusted-transport check the other INT routes use.
- **Cross-tenant object reads.** corpusd `GET /internal/platform/objects` and
  `/edges` accept any bound guest with a caller-chosen owner or computer filter
  (`allowAnyGuest=true`, objectgraph_handlers.go:61, :110). Any computer can
  list another owner's objects.
- **The corpusd event-read bypass.** It accepts any `X-Authenticated-User`
  from a host-internal caller without checking ownership
  (event_handlers.go:332).
- **hostauth is in observe mode.** Until it enforces, every `/internal/*`
  route on vmctl, corpusd, the gateway and maild is open to any process on
  Node B's loopback. Make the deploy preflight send the token first (§2,
  Latent).
- **maild trusts the proxy blindly.** It treats any loopback caller with
  `X-Internal-Caller` and `X-Authenticated-User` as the owner's session
  (internal/maild/api.go:244-255).
- **Public diagnostics.** The proxy's `/health` publishes lifecycle-stage
  counters, and the gateway's `/health/ready` publishes host ports and
  dependency errors. Neither is a secret, but both belong behind auth or in a
  summary.

## 7. Live probes (unauthenticated GETs, 1 s apart, 2026-10-14 22:23 UTC)

| Path | Status | What it shows |
|---|---|---|
| `/health` | 200 | proxy; includes lifecycle counters |
| `/health/ready` | 200, `"status":"degraded"` | the gateway answers; `dolt`, `runtime` and `ollama` are connection refused (Broken 9) |
| `/api/pulse/summary` | 200 | public aggregate, as designed |
| `/api/platform/retrieval/search?q=choir` | 200, empty results | public; see the search FIX |
| `/api/computers/x/self-development/operations` | 401 | the proxy guest-route predicate matches (method allowed, so auth runs) |
| `/api/computers/x/self-development/operations/y/decision` | 405 | the predicate is still installed; a GET is refused before auth |
| `/api/computers/x/recovery/job` | 401 | the dead route is still deployed |
| `/auth/api-keys` | 401 | auth service |
| `/api/email/resend/webhook` | 405 | Caddy sends it to maild |
| `/internal/platform/objects` | 403 | Caddy blocks `/internal/*` at the edge |
| `/api/notifications/x` | 401 | the dead forwarder is still deployed |
| `/api/runs` | 401 | the generic forward |

The proxy returns 401 for every unauthenticated `/api/*` request before the
guest sees it. Existence probes therefore cannot tell a live guest route from a
deleted one; only the static reading can. The guest's 405-versus-404
difference is visible only with a key.

## 8. The lean CLI and MCP set for the self-development loop

The loop: an agent proposes with `run start`, watches the run and the
operation, reads the patch, and opens a draft PR with its own GitHub
credentials; the owner arms the computer and approves. That needs about a
dozen agent tools and four owner commands. Today the default MCP list has 46
tools (45 after `selfdev-land`), so an agent doing this loop sees about four
times more tools than it uses, including ones that replace a workspace or stop
a computer.

**Agent: the default MCP profile and CLI (13 tools, 14 after B3)**

| Step | Tool (CLI) | Why |
|---|---|---|
| Propose | `run_start` (`run start --idempotency-key`) | the only working start; the prompt bar reaches engineering on an armed computer |
| Watch the request | `run_status`, `trajectory_get` | run state and lifecycle snapshot |
| Find and poll the operation | `self_dev_list`, `self_dev_show` | the operation's `trajectory_id` equals the run's; on the CLI, `self-dev wait` polls |
| See what engineering ran | `lifecycle_capsule_evidence` | capsule commands and results |
| Read the patch | `self_dev_patch` (after B3) | the read-only `…/operations/{op}/source-patch` route the readiness report designs; until then, `trace export` on the CLI |
| Pin the pre-apply state | `computer_checkpoint` | a write, not destructive |
| Withdraw a bad candidate | `self_dev_reject` | fail-safe; frees a pinned computer |
| Stop a runaway request | `trajectory_cancel` | idempotent and CAS-guarded |
| Say what it expects, then settle | `precommit_add`, `precommit_settle`, `precommit_list`, `calibration_summary` | the calibration loop |

The draft PR is opened with `gh` from the agent's own checkout, not through
Choir. `identity_verify` is worth keeping on the CLI to check the deployed
commit. It does not need to be in the agent's default tool list.

**Owner: CLI only, owner key or session**

- `self-dev mode set`, narrowed to `off`, `audit_only` and `propose_only`.
  `approve` already arms `accept_once` with the exact binding, so the eight
  binding flags on `mode set` are a second, error-prone way to do the same
  thing. Keep `qualified_consensus` API-only until a live policy needs it.
- `self-dev approve` (arm `accept_once`, then decide)
- `computer restore`, the undo
- `api-key create`, `list` and `revoke`, to mint the split agent and owner keys

**Everything else.** Keep it on the CLI and move it off the default MCP list,
behind an explicit profile (`choir mcp --profile operator`). This covers:
- computer operations: `stop`, `start`, `restart`, `refresh`, `recover`,
  `replace-workspace`, `import-residue-snapshot`, `producer-reports`,
  `settle-producer-reports`, `replay-completeness` and `status`
- lifecycle reads: `lifecycle events`
- unrelated to this loop: `texture *`, `files *`, `search`, `api_key_list`,
  `api_key_revoke`, `self_dev_head` and `self_dev_mode_get`

The profile flag is new code (orange); until it lands, the agent's MCP server
should run with the agent key, so the extra tools refuse on scope rather than
act.

**CLI cuts, from 59 commands to 53 with no loss of capability**
- **Delete:** `self-dev start`, and `precommit import-ledger` once applied.
- **Merge:** `lifecycle snapshot` into `trajectory`, `texture read` into
  `texture show`, `texture history` into `texture revisions --metadata`, and
  `run cancel` into `trajectory cancel`.
- **Owner-only:** `self-dev head` becomes CLI-only, since `approve` reads it
  internally.
- **Fix:** the description of `computer recover`, and the `--desk` help on
  `texture create`.

## 9. Suggested order

1. Land `selfdev-land` (deletes `self-dev start` from the CLI, the tool and the
   skill). In a follow-up, remove the proxy's propose mapping and the m11
   probe's POSTs (orange; rollback by revert).
2. Confirm the Texture Save 400 with one authenticated save on staging. If it
   reproduces, write the problem doc first, then fix the client (orange).
3. Delete the vmctl `autoputer-proxy`, the auth recovery, credential and
   session routes, and guest `/api/mail/inbound`. These are the largest
   exposures among the unused routes (red for vmctl and auth: full ceremony).
4. Strip `X-Authenticated-Computer` at the proxy, add the transport check to
   corpusd key unwrap, and make the deploy preflight send the hostauth token
   (red).
5. Then the dead-route sweep (§3, items 4 to 10), one service per commit, each
   citing this report's evidence lines and rechecking callers at that commit.
6. The MCP profile split and the owner-act boundary (§4, first item) need an
   owner ruling on the shape. They are recommendations, not a census.

Per CLAUDE.md, each deletion commit should recheck its citers at the commit it
lands on. This report reads `origin/main` at 7a8ad165c, and the counts drift.

## Heresy delta

- **Discovered:**
  - three product-path client/route mismatches beyond `self-dev start`: the
    Texture Save extra fields, the management ProductAPI computer binding, and
    the proxy's proposal delivery (the last carried over from the census)
  - gateway readiness that cannot go green
  - owner acts reachable with agent-mintable key scopes
  - a forgeable computer-binding header
  - corpusd key reveal without a transport check
  - cross-tenant object listing
  - about 60 uncalled endpoints, including a cross-tenant reverse proxy in
    vmctl
- **Introduced:** none.
- **Repaired:** none (analysis only).

## Appendix: inventory by service

Verdicts: K keep, D delete, F fix, M merge, O owner ruling. "tests" means the
only callers are tests. File references are to `origin/main` at 7a8ad165c.

### A. CLI and MCP (`cmd/choir`)

| Command | MCP tool | Route | Verdict |
|---|---|---|---|
| trajectories | trajectories_list | GET /api/trajectories | K (operator profile) |
| trajectory | trajectory_get | GET /api/trajectories/{id} | K (loop) |
| trajectory cancel | trajectory_cancel | POST …/{id}/cancel | K (loop) |
| lifecycle snapshot | lifecycle_snapshot | GET /api/trajectories/{id} | M into `trajectory` |
| lifecycle events | lifecycle_events | GET …/{id}/events | K (operator) |
| lifecycle capsule-evidence | lifecycle_capsule_evidence | GET …/capsule-evidence/{a} | K (loop) |
| identity | identity_verify | GET /api/acceptance/execution-identity | K (CLI) |
| texture create | texture_create | POST /api/texture/lifecycle-documents | K; F `--desk` help |
| texture revise | texture_revise | POST …/{doc}/revise | K (operator) |
| texture read | texture_read | GET …/{doc} | M into `show` |
| texture history | texture_history | GET …/{doc}/history | M into `revisions` |
| texture revisions | texture_revisions | GET …/{doc}/revisions | K |
| texture show | texture_show | GET …/{doc}, …/revisions/{rev} | K |
| texture watch | CLI only | GET …/{doc}/events | K |
| texture open-source | texture_open_source | GET …/{doc}/source-open | K (operator) |
| search | search | GET /api/platform/retrieval/search | K; F recency cap |
| run start | run_start | POST /api/prompt-bar | K (loop) |
| run status | run_status | GET /api/runs/{id} | K (loop) |
| run list | run_list | GET /api/runs | K (operator) |
| run cancel | run_cancel | POST /api/runs/{id}/cancel | M into `trajectory cancel` |
| files mkdir, put, get | files_* | /api/files/{path} | K (operator) |
| computer status | computer_status | proxy lifecycle/status | K (operator) |
| computer replay-completeness | computer_replay_completeness | proxy, then guest | K (operator) |
| computer replace-workspace | computer_replace_workspace | proxy, then guest | K (operator; destructive) |
| computer rematerialize-from-tape | CLI only (owner act) | proxy, then guest | K |
| computer checkpoint | computer_checkpoint | proxy, then guest | K (loop) |
| computer import-residue-snapshot | computer_import_residue_snapshot | proxy, then guest | O (migration-shaped) |
| computer restore | CLI only (owner act) | proxy, then guest | K (owner) |
| computer bootstrap-chain | CLI only (owner act) | proxy, then guest | K (owner) |
| computer producer-reports, settle-producer-reports | computer_* | proxy, then guest | K (operator) |
| computer stop, start, restart, refresh | computer_* | proxy, then vmctl | K (operator) |
| computer recover | computer_recover | proxy lifecycle/recover | F description; M (§5) |
| api-key list, revoke | api_key_list, api_key_revoke | /auth/api-keys | K (owner) |
| api-key create | CLI only | POST /auth/api-keys | K (owner) |
| self-dev list, show | self_dev_list, self_dev_show | guest operations | K (loop) |
| self-dev head | self_dev_head | corpusd events/head | K, CLI only |
| **self-dev start** | **self_dev_start** | **POST operations (405)** | **D** (on `selfdev-land`) |
| self-dev wait | CLI only | polls the operation | K |
| self-dev approve | CLI only (owner act) | mode PUT plus decision | K (owner) |
| self-dev reject | self_dev_reject | decision | K (loop) |
| self-dev mode get | self_dev_mode_get | corpusd mode | K, CLI only |
| self-dev mode set | CLI only (owner act) | corpusd mode PUT | K; narrow the modes |
| trace export | CLI only | about a dozen GETs | K |
| precommit add, settle, list, calibration | precommit_*, calibration_summary | /api/precommit/* | K (loop) |
| precommit import-ledger | CLI only | POST claims | D after apply |
| mcp, version, help | — | — | K |

### B. Proxy (`internal/proxy`, :8082)

| Method | Path | Handler | Callers | Verdict |
|---|---|---|---|---|
| GET | /health | handlers.go HandleHealth | CI deploy-base select | K |
| GET | /api/shell/bootstrap | proxy, then guest | frontend auth.js, Desktop | K |
| GET (WS) | /api/ws | HandleWS | frontend Desktop | K |
| GET (WS) | /api/super-console/ws | HandleManagementConsoleWS | frontend SuperConsoleApp | K |
| GET | /api/acceptance/execution-identity | execution_identity.go | CLI identity | K |
| GET | /api/compute/status | compute_status.go | frontend compute-monitor, SettingsApp; 2 scripts | K |
| POST | /api/compute/recovery | compute_status.go:391 | frontend compute-monitor | M (§5) |
| GET, POST | /api/computers/{id}/lifecycle/{status,start,stop,restart,refresh,recover} | computer_lifecycle.go | CLI; scripts | K; M recover |
| POST | …/lifecycle/cold-recover | computer_lifecycle.go | scripts (vmctl_receipt_probe, sh_lose_the_disk) | K (operator) |
| POST | …/lifecycle/{replace-workspace, rematerialize-from-tape, restore, checkpoint, import-residue-snapshot} | proxy, then guest | CLI; tracer, m11 | K |
| GET, POST | …/lifecycle/{producer-reports, settle-producer-reports} | computer_producer_reports.go | CLI | K |
| POST | …/lifecycle/bootstrap-chain | computer_lifecycle.go:530 | CLI; tracer, m11 | K |
| GET, POST | /api/computers/{id}/recovery/job | computer_recovery_job.go | tests | D |
| GET, PUT | …/self-development/mode; GET …/head | self_development.go, to corpusd | CLI; scripts | K |
| GET | …/self-development/replay-completeness | self_development.go, to guest | CLI | K |
| GET, POST | …/self-development/{operations[/{op}[/decision\|/receipts]], rollbacks, kernel-capabilities} | self_development.go, to guest | CLI; scripts | K; D the operations POST mapping, kernel-capabilities, and probably rollbacks |
| any | …/self-development/* (other) | 409 catch-all | — | K (tombstone); D the genesis exclusion |
| GET | /api/pulse/summary | handlers.go | frontend pulse.js | K (public) |
| GET, POST | /api/platform/texture/publications | platform_publish.go | frontend texture.js; scripts | K |
| GET | /api/platform/publications/{resolve,export,front} | platform_public.go | frontend; CLI trace | K |
| GET | /api/platform/retrieval/search | platform_public.go | CLI search | K; F |
| POST | /api/platform/publications/{id}/proposals | platform_public.go | frontend texture.js | K; D the dead author delivery |
| any | /api/email/resend/webhook | 404 (Caddy routes it to maild) | — | K (defense in depth) |
| any | /api/email/* | email.go, to maild | frontend MailApp | K |
| any | /api/notifications/* | notifications.go, to maild | none | D |
| any | /api/* | HandleProtectedAPI, to guest | everything else | K |
| any | /provider/*, /internal/vmctl/* | deny | — | K |
| any | /internal/platform/{objects, objects/, edges, captures, captures/bodies/, captures/}, support-verdicts | pass-through to corpusd | guests go direct | D (§3, item 4) |
| POST | /internal/platform/texture/publications | HandleGuestTexturePublication | agentcore story_publish.go:328 | K |
| GET | /assets/, / | computer_surface.go | browsers | K |

### C. Guest (autoputer)

See §2 to §5 for the verdicts that are not K.

| Method | Path | Callers | Verdict |
|---|---|---|---|
| POST | /api/prompt-bar | frontend; CLI; scripts; ProductAPI | K |
| GET | /api/prompt-bar/submissions/{id} | frontend conductor.js | M (view of runs) |
| GET | /api/runs, /api/runs/{id}, /api/runs/{id}/events | CLI; scripts | K |
| POST | /api/runs/{id}/cancel | CLI | M |
| GET | /api/inboxes/{inbox}/records | CLI trace; scripts | K |
| GET, POST | /api/precommit/claims[/{id}[/settlements]], /api/precommit/calibration | CLI | K |
| GET, POST | /api/calibration, /api/calibration/settlements | scripts/icrl_calibration.py | M |
| GET | /api/captures | CLI trace; scripts | K |
| GET | /api/model-policy/resolve | frontend TextureEditor | K |
| GET | /api/costs | scripts/cpu_efficiency_probe.py | K; F auth |
| GET, POST | /api/podcast/{search, subscriptions, subscriptions/refresh} | frontend PodcastApp | K |
| GET, POST, DELETE | /api/autopaper/sources/schedules | ProductAPI; scripts | K |
| GET, POST | /api/computer-settings | scripts | K |
| GET (WS) | /api/ws | proxy | K |
| POST | /api/computers/{id}/lifecycle/* (6 routes) | proxy | K |
| GET, POST | …/lifecycle/{producer-reports, settle-producer-reports} | proxy | K |
| GET | …/self-development/replay-completeness | proxy; vmctl | K |
| GET | …/self-development/kernel-capabilities | tests | D |
| GET | …/self-development/operations | CLI; scripts | K; cap the list |
| POST | …/self-development/operations | 405 | (the CLI caller is deleted) |
| POST | …/self-development/rollbacks | tests | O |
| GET | …/operations/{op}, …/receipts | CLI; scripts | K |
| POST | …/operations/{op}/decision | CLI; scripts | K; F the owner-act boundary |
| GET | /api/computers/{id}/events/{digest} | unreachable | D |
| GET | /api/trajectories, /{id}, /{id}?view=summary, /{id}/events, /{id}/stream, /{id}/capsule-evidence/{a} | frontend; CLI | K |
| POST | /api/trajectories/{id}/cancel | frontend; CLI | K |
| GET | /api/acceptance/execution-identity | proxy | K |
| GET | /api/runtime/{observability, obligations, census} | CLI trace; vmctl; scripts | K |
| GET, POST | /api/run-acceptances[/{id}], /synthesize | ProductAPI only | K; cap the list |
| GET | /api/prompts, /api/prompts/{role} | none (introspection) | K |
| PUT, DELETE | /api/prompts/{role} | test-gated | D |
| POST | /api/evals/texture-prompt | spec | F (gate) |
| GET, POST | /api/texture/documents | frontend; scripts; ProductAPI | K; F limit; M create |
| POST | /api/texture/lifecycle-documents | CLI | K |
| POST | /api/texture/files/open | frontend Desktop | K |
| POST | /api/texture/markdown-lineage/import | spec | D |
| GET | /api/texture/documents/{id}, /revisions, /history, /events, /stream, /source-open, /diagnosis; /api/texture/revisions/{rev} | frontend; CLI | K (M history) |
| POST | /api/texture/documents/{id}/revisions | frontend | **F (Broken 2)** |
| POST | /api/texture/documents/{id}/{revise, restore, manifest} | frontend; scripts | K |
| GET | /api/texture/documents/{id}/export | spec | D |
| GET | /internal/texture/documents/{id}, /internal/texture/revisions/{id} | tests | D |
| POST, GET | /api/content/items, /items/{id}, /import-url | frontend; proxy | K |
| GET, POST | /api/browser/{capabilities, sessions, sessions/{id}/{navigate, control, close}} | frontend BrowserApp | K |
| GET, PUT | /api/desktop/state, /api/media/{progress, recents}, /api/preferences/theme | frontend | K |
| GET, POST, PUT, DELETE | /api/files, /api/files/{path} | frontend; CLI | K |
| POST | /api/files/sync | one script | F (namespace) |
| GET (WS) | /api/super-console/ws | proxy | K |
| GET | /api/shell/bootstrap | proxy | K |
| any | /api/shell/error | test-gated | D |
| POST | /api/mail/inbound | none | D |
| GET | /health, /health/ready, /internal/boot/timeline, /internal/debug/pprof/* | vmctl; vmmanager; gateway; scripts | K |
| GET | /internal/diag/tcp-dial | one probe | K (diagnostic) |
| GET, POST | /internal/runtime/runs/{id}/events | POST: maild; GET: none | K POST; D GET |
| POST | /internal/runtime/platform-update | none | O |
| POST, DELETE | /internal/runtime/planned-restart | vmctl deploy-refresh | K |
| POST | /internal/runtime/hibernate/{prepare, resumed} | vmmanager | K |
| POST | /internal/self-development/restart-handoff | choir-updater | K |
| — | updater /v1/{public-key, kernel-capabilities, import-baseline, apply}; receipt-signer /v1/{public-key, sign-receipt, sign-verifier-certificate} | agentcore | K |

### D. corpusd (:8086)

| Method | Path | Callers | Verdict |
|---|---|---|---|
| GET | /health, /internal/debug/pprof/* | CI; hardening script | K |
| POST | /internal/computers/credentials/{exchange, consume, issue, renew} | selfdev; vmctl | K |
| POST | …/lifecycle/control | proxy | K |
| GET, POST | …/self-development/mode | proxy; selfdev; scripts | K |
| GET, POST | …/events/{head, pin, append, payload, replay} | computerevent client; proxy; vmctl | K |
| GET, POST | …/checkpoints, …/route-projection-certificates | selfdev | K |
| POST | …/platform-updates/offer | none | O (operator) |
| GET, PUT | …/platform-updates/blob/{sha} | GET: client; PUT: manual | K |
| GET, PUT | …/keys/{escrow-public-key, escrow, escrow/status}, …/keys/realization-delivery | autoputer; vmctl | K |
| POST | …/keys/unwrap-requests[/{id}/{approvals, reveal}] | none (break-glass) | K; **F transport** |
| GET | …/keys/transparency-head | none | O |
| GET, PUT | …/files/{chunks/{d}, root, watermark, roots, projection-base/descriptor, projection-base/blob} | autoputer; projectionbase; vmctl | K |
| GET, POST | …/projection-base/jobs | vmctl; proxy | K |
| GET, POST, DELETE | …/projection-base/pins | GET and POST: projectionbase; DELETE: none | K; D DELETE; cap GET |
| GET | /internal/platform/control-key | vmctl | K |
| POST | /internal/platform/execution-identity/attest | proxy | K |
| POST | /internal/platform/publications/texture | proxy | K |
| POST | /internal/platform/support-verdicts | agentcore (direct) | K |
| GET | /internal/platform/publications/{resolve, export, front, owned} | proxy; front-importance | K |
| GET | /internal/platform/retrieval/search | proxy | K; F recency cap |
| POST | /internal/platform/publications/{id}/proposals | proxy | K; F idempotency |
| POST | /internal/platform/proposal-deliveries/state | proxy (constant fallback) | D |
| POST, GET | /internal/platform/texture/{sync, documents, documents/{id}, revisions/{id}} | none | D |
| POST | /internal/platform/artifact-gc | operator | K |
| GET | /internal/platform/objects | researchtools capture sink | K; F cross-tenant and limit |
| POST, PUT | /internal/platform/objects; GET /objects/{id}; any /edges | none | D |
| POST, GET | /internal/platform/captures, /captures/{id} | researchtools | K |
| GET | /internal/platform/captures/bodies/{sha} | none | D |

### E. vmctl (:8083)

| Method | Path | Callers | Verdict |
|---|---|---|---|
| GET | /health | CI | K |
| POST | /internal/vmctl/{resolve, stop, refresh, unhold} | proxy; maild; CI; scripts | K |
| POST | …/computers/{id}/cold-recover | proxy | K |
| POST, GET | …/computer-version-inputs/resolve, …/computer-version-routes/{resolve, apply-self-development, apply-platform-follow} | agentcore; proxy; maild | K |
| GET, POST | …/lookup, …/lookup-guest | proxy; corpusd; maild | K; M maild's private client |
| POST | …/remove | one script | K (operator); F the stale "logout" comment |
| GET | …/list | proxy; CI; scripts | K; cap |
| POST | …/maintenance-serve, …/hold | runbook only | K (break-glass) |
| GET, POST | …/deploy-refresh | CI | K |
| POST | …/idle-check | preflight fallback | M into reclaim |
| POST | …/reclaim | nix; preflight | K |
| POST | …/divergence | tests | D |
| GET | …/retention-plan, …/retention-shadow-plan | none | D |
| GET | …/pulse | proxy | K |
| any | …/autoputer-proxy/{owner}/… | tests | **D** |
| GET | …/boot-timeline | scripts | K |
| POST | …/hibernate | one script | K (operator) |

### F. gateway (:8084)

| Method | Path | Callers | Verdict |
|---|---|---|---|
| GET | /health | CI; start-services.sh | K |
| GET | /health/ready | public (Caddy) | **F** |
| GET | /health/{service} | none; public | D |
| POST | /provider/v1/{inference, judgments, search} | gatewayruntime; search | K |
| POST, GET | /provider/openai/v1/{chat/completions, models} | zot | K |
| GET | /provider/v1/models | none (by design) | M |
| GET | /provider/v1/search/health, /provider/v1/breakers | hardening script | K (operator) |
| POST | /provider/v1/search/health/reset, /provider/v1/breakers/reset | none | K (break-glass) |
| POST | /provider/v1/credentials/{issue, ensure} | vmctl | K; F idempotency on issue |

### G. maild (:8087)

| Method | Path | Callers | Verdict |
|---|---|---|---|
| GET | /health | CI | K |
| POST | /api/email/resend/webhook | Resend | K |
| GET, POST | /api/email/{aliases, drafts, drafts/{id}, drafts/{id}/send, drafts/{id}/approval-email} | frontend MailApp | K |
| POST, GET, DELETE | /api/email/attachments[/{id}] | frontend MailApp | K |
| GET, POST | /api/email/messages[/{id}], …/{id}/read, …/{id}/unread | frontend MailApp | K |
| DELETE | /api/email/messages/{id}/read | none | D |
| POST | /api/notifications/email-risk-alert | none (last caller deleted 9679837f3) | D |

### H. auth (:8081)

| Method | Path | Callers | Verdict |
|---|---|---|---|
| POST | /auth/register/{begin, finish}, /auth/login/{begin, finish} | frontend auth.js; qa script | K |
| GET | /auth/session | frontend; scripts | K (a GET that rotates the refresh token: note) |
| POST | /auth/logout | frontend | K |
| GET, POST | /auth/desktop/exchange-redirect, /auth/desktop/redeem | desktop bridge; cmd/desktop | K |
| GET | /api/auth/root/status | unreachable | D |
| POST, DELETE | /auth/api-keys[/{id}] | frontend Settings; CLI; qa script | K |
| GET | /auth/api-keys | frontend; CLI | F (scope, limit) |
| POST | /auth/recovery/{request, verify} | none | D |
| GET, POST, DELETE | /auth/credentials, /credentials/rename, /credentials/{id} | none | D (or build the UI) |
| GET, DELETE | /auth/sessions[/{id}] | none | D (or build the UI) |

**Scope matrix.** Auth issues only scopes on its `validScopes` allowlist
(internal/auth/handlers.go:1530-1549). The proxy checks no scope that auth
refuses to issue. Auth issues three scopes that guard nothing live:
- `computer:self_development:genesis`: no route checks it
- `read:base`, `write:base`: no service serves `/api/base/`
- `computer:self_development:propose`: guards only the deleted POST

The proxy forwards `X-Authenticated-Scopes` to guests, but no guest code reads
it, so only the proxy enforces scopes.
