# Auth Boundary Gaps From the Surface Audit

Date: 2026-10-14. Source: the surface audit
(`docs/archive/reports/surface-audit-cli-mcp-api-2026-10-14.md`, §2 "Latent", §3
items 2 and 4, §4 FIX, §6 Risky), each finding re-read against the code at
`origin/main` e9d85a71d. Status: documented (7bb8a5b15); repaired on branch
`fix/auth-boundary-audit` (not pushed, not deployed; see "Repair record"
below). Mutation class of the
repair: **red** (auth/session, Trace-adjacent owner acts, vmctl, corpusd key
escrow, deployment script).

Evidence class for every verdict below: **static** (code read, file:line at
e9d85a71d) plus local unit tests on the repair branch. Nothing here was
exercised on staging; no verdict claims deployed proof.

## The problem in one sentence

Several boundaries decide "who is calling" from something the caller can
choose: a header the proxy forwards unstripped, a loopback source any
in-guest process holds, an API key scope (`admin`) that silently includes
owner acts, or a transport corpusd never checks.

## Findings

Severity: **H** (an agent key or in-guest process gains an owner act or
another tenant's data), **M** (a guarded act with a secret or a second
precondition still in the way), **L** (defense in depth; no reachable
exploit found).

### F1. The proxy forwards a client-supplied `X-Authenticated-Computer` (CONFIRMED, H)

- The generic forward strips `clientIdentityHeaders`
  (`internal/proxy/handlers.go:43-54`, applied at `:280-283`). The list has
  no `X-Authenticated-Computer`.
- The guest's computer router accepts that header as the binding
  (`internal/agentcore/api_self_development.go:84`).
- The audit rated this low because the lifecycle and self-development paths
  are intercepted before the generic forward. **They are not intercepted
  with a trailing slash.** The proxy predicates match exact suffixes
  (`computer_lifecycle.go:229-243`, `:528-543`), while the guest trims
  slashes before it splits (`api_self_development.go:67`). So
  `POST /api/computers/{own}/lifecycle/restore/` with a forged
  `X-Authenticated-Computer: {own}` goes through `HandleProtectedAPI`
  (`handlers.go:871-877`), which checks only `write:runtime`, and the guest
  runs restore. The same works for `rematerialize-from-tape`,
  `bootstrap-chain`, `replace-workspace`, `checkpoint` and
  `import-residue-snapshot`. A `write:runtime` agent key therefore bypasses
  the `computer:lifecycle` scope check (and any owner-act check added at
  the proxy) on its own computer. The `self-development/*` family is safe:
  its catch-all 409 (`handlers.go:833-835`) matches by substring.
- **Discovered beside it:** the proxy's carrier headers are not stripped at
  ingress. `setTrustedAuthHeaders` (`handlers.go:419-427`) sets
  `X-Proxy-Trusted-Email` and `-Scopes` only when non-empty, and the
  director copies whatever it finds (`:286-298`). A cookie session (no
  scopes) can send `X-Proxy-Trusted-Scopes: admin` and the guest receives
  `X-Authenticated-Scopes: admin`. No guest code reads that header today
  (audit appendix H), so the impact is latent (L). Likewise
  `X-Resolved-Autoputer-URL` (`:267`) is read by the director before it is
  deleted; a client value survives when the resolved URL equals the static
  `ComputerURL` (`:759-762`), which happens only without vmctl routing
  (local development) (L).
- Other identity headers trusted somewhere, and whether a client value
  reaches them through the proxy today: `X-Internal-Updater` (guest,
  loopback-gated: no), `X-Choir-Operator` and `-Operator-Token` (corpusd:
  the proxy builds fresh requests: no), `X-Choir-Lifecycle-Caller` (vmctl:
  no), `X-Choir-Capsule` (goproxy: no). They are added to the strip list as
  defense in depth.
- **Failure modes:** an agent key restores or rematerializes its own
  computer without the lifecycle scope or an owner; a forged scope header
  becomes authority the day a guest route reads it.
- **Repair:** strip every reserved identity and carrier header at proxy
  ingress for the public handlers, add the missing names to
  `clientIdentityHeaders`, clear the carrier headers in
  `setTrustedAuthHeaders` before setting them, and refuse
  `/api/computers/*` in the generic forward (every legitimate route under
  it is intercepted; the one that is not, `events/{digest}`, is
  unreachable already, audit Broken 5).
- **Rollback:** revert the proxy commit.

### F2. Five drifted `authenticateUser` copies, and loopback is the owner inside the guest (CONFIRMED, H)

- Copies that trust `X-Authenticated-User` with no transport check at all:
  `internal/textureowner/handler.go:226`, `internal/content/handler_helpers.go:18`,
  `internal/browsercontrol/handler.go:64`, `internal/desktopstate/handler.go:99`,
  `internal/mediastate/handler.go:313`, and `internal/apihandler/handler.go:29`
  (used by `GET /api/costs`, `costs.go:43`).
- The copy that does check, agentcore's (`internal/agentcore/api.go:164-173`),
  and the other guest identity reads (`autoputer/files.go:24`,
  `terminal.go:133`, `handlers.go:54`, `run.go:532`,
  `agentcore/residue_import.go:42`) use `server.HostSourcedCaller`, which
  admits loopback (`internal/server/host_caller.go:41-43`).
- So any process inside the guest (a terminal shell, zot, a capsule
  workload that reaches the guest port) can call `127.0.0.1:8085` with
  `X-Authenticated-User: <owner>` and act as the owner on every guest
  route: Texture writes, files, prompt bar, self-development decisions
  (with a forged `X-Authenticated-Computer`), the terminal.
- The guest's internal routes have the same shape:
  `requireInternalRuntimeCaller` (`agentcore/api.go:175-180`, used by
  planned-restart `planned_restart.go:366`, hibernate `hibernate.go:269`,
  `:330`, platform update `platform_update.go:779`, internal run events
  `api.go:345`) admits loopback; an in-guest process can write a
  planned-restart marker and turn the next crash into a resume (a
  violation of the CLAUDE.md restart rule). Textureowner's copy
  (`textureowner/texture.go:48-53`) checks only the forgeable
  `X-Internal-Caller` header.
- **How the host reaches the guest:** vmmanager hands the proxy and vmctl
  the guest's tap address (`internal/vmmanager/manager.go:888-889`), so a
  host request arrives from the tap's host side, which is the guest's
  default gateway. The strict form already exists as
  `server.HostPeerCaller` (`host_caller.go:63-91`) and has guarded
  `/internal/diag/tcp-dial` in production since 98cf3875.
- **Legitimate in-guest callers that loopback served (checked before
  closing it):**
  1. The management desk's `choir.ProductAPI` (`agentcore/tools_product_api.go:68`)
     serves an in-process `httptest.NewRequest`, whose RemoteAddr is the
     TEST-NET-1 address `192.0.2.1`. In-process, not loopback. The repair
     marks it in-process explicitly (empty RemoteAddr), because TEST-NET-1
     is admitted only inside test binaries after the repair.
  2. `choir-updater` calls `/internal/self-development/restart-handoff` and
     `/health` on loopback (`nix/autoputer-vm.nix:713`). Neither uses the
     owner check (the handoff has its own loopback gate,
     `api_self_development.go:878`). Unchanged.
  3. Local development (`start-services.sh:116-122`) runs proxy and
     autoputer on one host and forwards over 127.0.0.1. The repair keeps
     it working with an explicit opt-in, `CHOIR_GUEST_TRUST_LOOPBACK=1`,
     exported only by `start-services.sh`; the guest image never sets it.
  No other in-guest caller sets `X-Authenticated-User` (grep of `internal`,
  `cmd`, `nix`, `scripts`: the remaining setters are vmctl and
  `choir-rebuild-base`, both on the host).
- **Residual:** terminal and zot children still run as the runtime's uid
  (`docs/problems/s1-non-root-runtime-child-uid-boundary-2026-10-04.md`).
  A root child with `CAP_NET_ADMIN` could add the gateway address to `lo`
  and source from it. Closing loopback removes the casual path; the full
  boundary needs that S1 uid split. Named, not repaired here.
  (2026-10-22: zot and its Console PTY are removed,
  `docs/problems/remove-zot-2026-10-22.md`; no terminal or zot child
  remains in production, so the residual is the other guest children.)
- **Failure modes of the repair:** a host caller that reaches the guest
  through `127.0.0.1:<hostPort>` DNAT (`hostfirewall.go:157-158`) instead
  of the tap address would now be refused; none was found in code (vmctl,
  proxy, maild and vmmanager use the tap URL). Local tests that drive a
  real `httptest.Server` (loopback) with an owner header need the opt-in.
- **Repair:** one function, `server.AuthenticatedOwner`, over one transport
  rule, `server.GuestHostCaller` (in-process, host peer, TEST-NET-1 in test
  binaries, loopback only with the opt-in). Every guest copy calls it;
  internal runtime routes and textureowner's internal reads use
  `GuestHostCaller`.
- **Rollback:** revert the guest commit; or, without a deploy, nothing
  (the opt-in only widens).

### F3. `GET /auth/api-keys` has no scope check and no limit (CONFIRMED, M)

- `HandleListAPIKeys` (`internal/auth/handlers.go:1787-1810`) authenticates
  any cookie or key and lists every key of the owner, unbounded
  (`store.go:981-985`). A zero-scope or `precommit:write` key reads the
  labels, scopes, computer bindings and last-use times of every sibling.
- Auth accepts an empty `scopes` list at creation (`handlers.go:1728`);
  such a key can do nothing on the proxy but list and revoke itself.
- **Repair:** a key without `manage:keys` or `admin` sees only itself; the
  list takes `?limit=` (default 100, max 500); creation refuses an empty
  scope list.
- **Callers checked:** `choir api-key list` (`cmd/choir/commands.go:496`)
  and Settings (`SettingsApp.svelte:105`, cookie) keep working.
- **Rollback:** revert the auth commit.

### F4. Owner acts pass with any key holding the scope, or `admin` (CONFIRMED, H)

- `self-dev approve` (decision POST), `mode set` (mode PUT), `restore`,
  `bootstrap-chain` and `rematerialize-from-tape` are CLI-only since
  5b17bb572, but over HTTP the proxy admits any key with the route scope or
  `admin` (`proxy/self_development.go:140-145`, `:349-354`;
  `computer_lifecycle.go:330-336`, `:565-571`). Settings mints `admin` as
  its "simple" key (`SettingsApp.svelte:71`), so the default key a person
  makes can arm and approve. The precedent is mail send
  (`proxy/email.go:30`), which refuses every key.
- Decision (conservative default, per the selfdev update architecture P3
  and decision record in this doc): an owner act needs the owner's signed-in
  session, or a key that carries the new scope `owner:acts`. Auth grants
  `owner:acts` only to a key created from the owner's session; no key, not
  even `admin`, can mint or delegate it. `admin` alone no longer passes an
  owner act. Rejecting a candidate (`decision: "reject"`) stays an agent
  act: it is fail-safe and in the lean agent set.
- **Coordination:** the guest arming its own mode through corpusd's
  `event:append` path is being repaired on `fix/selfdev-mode-owner-only`.
  This repair does not touch corpusd's mode route; it gates only the proxy
  (public) side.
- **Callers checked:** `scripts/reeng_tracer.mjs` arms and approves with the
  key `scripts/qa_test_computer.mjs` mints from a browser session; the mint
  adds `owner:acts`. `scripts/m11_selfdev_episode_probe.mjs` and
  `demo_selfdev_prompt_bar.mjs` use a browser session (unaffected).
  `scripts/lib/selfdev_approval.mjs` arms over SSH against corpusd
  (unaffected). CI deploy scripts call none of these routes.
- **Failure modes:** an existing `admin` key on an operator's machine stops
  approving; the operator mints a key with `owner:acts` from Settings. A
  decision body the proxy cannot parse is treated as an approval (refused
  for a non-owner key).
- **Rollback:** revert the proxy and auth commits; existing keys are
  untouched (the scope is additive).

### F5. corpusd: key reveal without a transport check, cross-tenant object lists (CONFIRMED, M and H)

- `authorizeKeyEscrowOperator` (`internal/platform/key_escrow_http.go:297-309`)
  checks only `X-Choir-Operator` and its token; corpusd listens on
  `0.0.0.0:8086` and guests reach it through their tap DNAT
  (`vmmanager/manager.go:3444`). A guest that learned an operator token
  could request, approve and reveal a data-encryption key (M: two operator
  secrets still required).
- `GET /internal/platform/objects` and `/edges` accept any bound guest with
  caller-chosen `owner`, `computer`, `from`, `to` filters
  (`objectgraph_handlers.go:60`, `:109`, `:340-351`). Any computer can list
  another owner's objects and edges (H for confidentiality). The proxy's
  pass-through does the same (`proxy/guest_authority.go:56-64`), and there
  corpusd sees the proxy as an internal caller.
- The only guest caller, `researchtools.ListCaptures`
  (`internal/researchtools/capture_sink.go:164`), always asks for its own
  owner and computer. No guest lists edges (the HTTPStore client has no
  constructor, audit §3 item 5).
- **Repair:** the operator routes also require a trusted host transport and
  the host credential; a guest's object list must name exactly its bound
  owner and computer; guests may not list edges. The proxy pass-through
  applies the same rule.
- **Rollback:** revert the corpusd and proxy commits.

### F6. Other holes the audit lists

| # | Finding | Verdict | Sev | Action here |
|---|---|---|---|---|
| a | vmctl `/internal/vmctl/autoputer-proxy/{owner}/…` relays any request, headers included, into any owner's computer (`vmctl/handlers.go:1108-1175`); only tests call it | CONFIRMED | H (host-local; any Node B process in observe mode) | deleted; the operator equivalent is curl to the guest's tap URL from Node B |
| b | `scripts/node-b-deploy-disk-preflight:25-26` calls vmctl without the hostauth token; breaks the deploy the day hostauth enforces | CONFIRMED | M (availability) | the script sends the header file, as ci.yml does |
| c | textureowner `/internal/texture/*` gated only by `X-Internal-Caller` (`texture.go:48-53`, `:851`, `:878`) | CONFIRMED | M (in-guest read of any document) | now `GuestHostCaller` (part of F2) |
| d | corpusd event read admits any host-internal caller for any owner (`event_handlers.go:332`) | CONFIRMED | L once hostauth enforces | residual: depends on hostauth `enforce` |
| e | maild treats loopback + marker + hostauth as the owner session (`maild/api.go:240-255`) | CONFIRMED | L once hostauth enforces | residual: depends on hostauth `enforce` |
| f | hostauth is `observe` on Node B (`nix/node-b.nix:67`) | CONFIRMED | M | not flipped here (deploy-shape decision); item b removes the deploy blocker |
| g | `POST /auth/recovery/request` is an unauthenticated write that mints a token (`auth/recovery.go`) | CONFIRMED | M (DoS) | owner ruled "delete for now" (owner-notes §19), not carried out; the account-recovery slice (`land/account-recovery`) builds the UI instead and supersedes the deletion on landing ([account-recovery-token-discarded-2026-10-14.md](account-recovery-token-discarded-2026-10-14.md), "Landing reconciliation") |
| h | guest `POST /api/mail/inbound` (`autoputer/maildir.go:213`) lets a `write:runtime` key inject mail | CONFIRMED | L (own mailbox) | deferred to the dead-route sweep (audit §3 item 7) |
| i | `/internal/self-development/restart-handoff` trusts loopback plus a forgeable `X-Internal-Updater` (`api_self_development.go:878`) | DISCOVERED | M | residual: needs the updater to authenticate by socket peer credential, not loopback |
| j | management ProductAPI to `/api/computers/*` always 403 (audit Broken 4) | CONFIRMED (a bug, not a hole) | — | unchanged: the fail-closed side |

## Red ceremony

- **Conjecture delta.** Before: "the proxy is the only caller that can set
  owner identity on the guest, and an owner act needs the owner." After:
  that holds only once (1) the proxy strips every reserved header at
  ingress, (2) the guest believes identity only from the host peer or
  in-process, and (3) owner acts check for a session or an
  owner-minted `owner:acts` key. The conjecture remains weaker than it
  reads while in-guest children share the runtime uid (F2 residual).
- **Protected surfaces touched:** auth (API key scopes and listing), the
  proxy's identity injection, guest owner identity and internal runtime
  routes (planned-restart marker), self-development owner acts, corpusd key
  escrow and object graph reads, vmctl (route deletion), and the deploy
  preflight script.
- **Admissible evidence class:** unit tests at the handler boundary (local),
  `go build ./...`. Deployed acceptance (a forged header refused on
  staging, the tracer's approve still accepted) is required before calling
  any of this repaired in production; it is not claimed here.
- **Rollback:** each fix is its own commit; `git revert` of any one restores
  the prior behavior of that surface. No data migration; the `owner:acts`
  scope is additive in the auth DB.
- **Deploy ordering:** auth before or with the proxy (a key with
  `owner:acts` must be mintable before the proxy requires it); the guest
  change ships in the base image and reaches each computer on its next
  refresh, so mixed fleets are expected and every guest change is
  fail-closed only for in-guest callers.
- **Heresy delta.** Discovered: the trailing-slash lifecycle bypass (F1),
  forgeable proxy carrier headers (F1), the loopback planned-restart
  marker (F2), the restart-handoff loopback gate (F6 i). Introduced: none
  intended; `CHOIR_GUEST_TRUST_LOOPBACK` is a new opt-in that must stay out
  of the guest image. Repaired: F1-F5, F6 a-c, on the branch only (not
  deployed).

## Repair record (branch `fix/auth-boundary-audit`, not pushed)

| Commit | Finding | What changed | Tests |
|---|---|---|---|
| 4e10028df | F3, F4 (auth) | key list scoped to the calling key without `manage:keys`/`admin`, `?limit` (100, max 500); empty scope lists refused; `owner:acts` issued only from the owner's session | `internal/auth/auth_boundary_test.go` |
| 196f6294a | F1, F4, F5 (proxy) | ingress strip of identity and carrier headers; `/api/computers/*` refused in the generic forward; owner-act gate on mode set, approve, rollback, restore, rematerialize, bootstrap-chain; guest object list scoped, edges refused | `internal/proxy/auth_boundary_test.go`, `computer_bootstrap_chain_test.go` |
| 1f67f0e0a | F5 (corpusd) | unwrap routes need host transport + host credential; object list bound to the caller's computer; edges host-only | `internal/platform/auth_boundary_test.go` |
| 6f7bffb30 | F6 a, b | vmctl `autoputer-proxy` deleted; deploy preflight sends the host credential | vmctl package tests |
| 7e9fd4451 | F2, F6 c | `server.AuthenticatedOwner` / `server.GuestHostCaller` replace every guest copy; internal runtime and textureowner internal routes host-only | `internal/server/guest_owner_test.go`, `internal/apihandler/in_guest_caller_test.go`, `internal/autoputer/terminal_test.go` |

Callers changed: `scripts/qa_test_computer.mjs` (mints `owner:acts` from the
owner's browser session, so `scripts/reeng_tracer.mjs` keeps its owner
acts), `start-services.sh` (`CHOIR_GUEST_TRUST_LOOPBACK=1`), Settings
(offers `owner:acts`), the CLI's owner-act commands and the choir-cli skill
(declare it), `choir.ProductAPI` (explicit in-process RemoteAddr), and the
agentcore/autoputer tests that drove guest handlers over a loopback test
server (now presented as the host side).

Deploy ordering risks:

1. Keys minted before the deploy lack `owner:acts`. After the proxy
   deploys, `choir self-dev approve`, `mode set`, `computer restore`,
   `bootstrap-chain` and `rematerialize-from-tape` with an existing key,
   `admin` included, return 403 until the owner mints a key with
   `owner:acts` from Settings. A tracer run needs a fresh
   `qa_test_computer.mjs` mint.
2. Auth and the proxy share the auth DB and ship in one Node B activation;
   no ordering gap between them.
3. The guest change reaches each computer with its next base image. Until
   then the proxy's ingress strip already closes the forged-binding path;
   the loopback hole stays open on not-yet-refreshed computers.
4. Any local stack started without `start-services.sh` (proxy and autoputer
   on one host) must export `CHOIR_GUEST_TRUST_LOOPBACK=1`, or every guest
   route answers 401.
5. hostauth is `observe` on Node B, so the corpusd unwrap credential check
   only logs until `enforce`; once enforced, an operator's unwrap curl must
   send the header file.

