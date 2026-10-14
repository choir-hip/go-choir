# Clients and routes that disagree: the surface audit's broken calls (2026-10-14)

Status: repaired in code on branch `fix/surface-broken-calls` (not pushed,
not deployed; the fix commit follows this note's first commit). Staging
proof is still owed for items 1, 3, 5 and 6, and the m11 probe has not been
rerun.
Mutation class of the fix: **orange** (product APIs, runtime behaviour,
probe scripts). Rollback: `git revert` of the fix commit. Source:
[surface-audit-cli-mcp-api-2026-10-14.md](../reports/surface-audit-cli-mcp-api-2026-10-14.md)
§2 ("Broken" and "Latent"). This note re-verifies each item at
`origin/main` e9d85a71d before any code changes. One of the audit's
claims needed correcting (marked **Correction**), and items 2 and 3 carry
consequences the audit did not name.

## Findings

1. **Texture editor Save is refused with 400.** Confirmed against the real
   handler. `createRevision` (frontend/src/lib/texture.js:117-123) always
   sends `author_kind` and `author_label`; TextureEditor.svelte passes
   them at :694, :928 and :1085. `handleTextureCreateRevision`
   (internal/textureowner/texture.go:1035-1041) decodes with
   `DisallowUnknownFields` into `textureCreateRevisionRequest`, which has
   neither field. A test that posts the editor's exact body to
   `HandleTextureRevisions` gets 400 "invalid request body"; the same body
   without the two fields gets 201 with author `user`/owner id
   (`TestTextureEditorSaveBodyContract`). The fields were dropped on
   purpose in fa166de1f: "the public route always creates user-authored
   revisions". The client is wrong.

2. **Proposal delivery posts to a deleted guest route.** The proxy's
   `deliverPublicationProposalToAuthor`
   (internal/proxy/platform_public.go:373-425) POSTs to the author
   guest's `/internal/texture/proposals`, which R3d deleted
   (internal/apihandler/routes.go:65: "the desk-originated worker_updates
   channel is deleted; children report through the ledger"). Every
   delivery falls back to `recorded_for_author`. Two consequences the
   audit did not name: (a) the fallback equals corpusd's initial state, so
   `recordPublicationProposalDeliveryState` is never called and corpusd's
   `/proposal-deliveries/state` has no live caller; (b) the attempt runs
   `resolveComputerURL` for the author, so a reader's proposal can resolve,
   and so start, the author's computer, then wait up to 30 s for the dead
   POST. The test `TestHandlePublicationProposalReadsPrivateDerivativeAndPostsProjection`
   mocks the deleted route and so certifies a path that cannot run. The
   intended design (R3d) has no guest delivery: corpusd records the
   proposal for the author.

3. **The management desk's `choir.ProductAPI` calls to `/api/computers/*`
   always get 403.** `productAPIRequest`
   (internal/agentcore/tools_product_api.go:69-71) sets only
   `X-Authenticated-User`/`-Email`. `handleSelfDevelopmentRoute`
   (internal/agentcore/api_self_development.go:84) requires
   `X-Authenticated-Computer` equal to the path id, so every call answers
   "authenticated computer binding required". The management overlay
   (internal/runtimeprompts/overlays/rlm_management_runtime.yaml:29) offers
   GET/POST/PUT/DELETE on `/api/computers/`. **Risk the audit did not
   name:** the router behind that prefix serves owner acts — operation
   `decision` (approve), `lifecycle/restore`, `replace-workspace`,
   `bootstrap-chain`, `rematerialize-from-tape`, `import-residue-snapshot`.
   5b17bb572 fenced those off MCP as owner-only. Adding the binding alone
   would hand them to the desk. The 403 is today the only thing keeping
   them off it, by accident.

4. **`GET /api/computers/{id}/events/{digest}` is unreachable and
   uncalled.** No proxy predicate matches it, so it takes the generic
   forward, which never sets `X-Authenticated-Computer`; the guest answers
   403. No caller in frontend, CLI, scripts or tests.
   **corpusd `GET /internal/platform/texture/documents` always 404s.**
   **Correction:** the audit says `GetPlatformTextureDocument` is
   unreachable. It is reachable: `/documents/{id}` goes to
   `HandleInternalListTextureRevisions`, which delegates to it when the
   path does not end in `/revisions` (internal/platform/handlers.go:378-380).
   Only the exact collection path (handlers.go:507) is dead: its handler
   needs an id and so always 404s. It has no caller.

5. **Gateway `/health/ready` is permanently `degraded`.** Live, 2026-10-14
   22:38 UTC: `dolt` and `runtime` (127.0.0.1:8085) and `ollama`
   (:11434) are `connection refused`; `qdrant` is `ok`. Computers run in
   VMs, so nothing listens on 8085 on Node B, and Ollama is not deployed.
   `DefaultServiceHealthURLs` (internal/gateway/config.go:73-78) still
   names all four. Only qdrant is a live host service (nix/node-b.nix:424).

6. **Unknown guest `GET /api/*` returns 200 HTML.** The guest mounts the
   SPA at `/` (internal/autoputer/computer_surface.go:99-104); a GET that
   no route claims falls back to `index.html`, and an unknown POST gets
   405 text. A client of a removed route sees a JSON parse error, not 404.

7. **The Texture document list ignores `?limit`.**
   `HandleTextureListDocuments` (internal/textureowner/texture.go:804)
   hard-codes 50. Five scripts ask for 100 or 500 and are silently
   truncated (`l2a_fixture_run.py:100`, `l2a_paper_window.py:74`,
   `l2a_window_pairs.py:141`, `management_latency_probe.py:355`,
   `reeng_tracer.mjs:926`). There is no paging, so documents past the
   newest 50 cannot be listed at all.

8. **Misleading CLI help.** `texture create --desk` says "texture (default)
   or engineering" (cmd/choir/commands.go:205); the server refuses any desk
   but texture (texture_lifecycle_create.go:89-90). `computer recover`
   says "Run the owner recovery job" (commands.go:490); the route clears
   the maintenance hold and starts the computer
   (internal/proxy/computer_lifecycle.go:162-172).

9. **Scripts that will break when hostauth enforces.** They call
   host-internal routes with `X-Internal-Caller` only, no
   `X-Choir-Internal-Token`. They work because `internalAuthMode =
   "observe"` (nix/node-b.nix:67). CI runs one on every deploy that needs
   reclaim. Re-verified list (a grep of `scripts/` and `frontend/tests/`
   for host ports 8083/8084/8086/8087):
   - `scripts/node-b-deploy-disk-preflight` (vmctl reclaim, idle-check;
     called from ci.yml)
   - `scripts/vmctl_receipt_probe.mjs` (vmctl list, lifecycle POSTs)
   - `scripts/vmctl_no_escalation_probe.mjs` (vmctl list, resolve, stop)
   - `scripts/vmctl_wedge_recovery_probe.mjs` (vmctl list, stop)
   - `scripts/sh_lose_the_disk_proof.mjs` (vmctl list, remove)
   - `scripts/s3_snapshot_probe.py` (vmctl list, hibernate, boot-timeline;
     it sends no marker header either)
   - `scripts/m11_selfdev_episode_probe.mjs` (vmctl list and route
     resolve; corpusd events head, replay and mode)
   - `scripts/lib/selfdev_approval.mjs` (corpusd events head and mode)
   - `frontend/tests/s0a-reality-probe.mjs` (vmctl list, boot-timeline,
     refresh)
   - `frontend/tests/texture-acceptance-deployed.spec.js` (vmctl refresh)
   Not affected: `mail-provider-readiness` and
   `texture-real-workflow-demo.spec.js` read `/health`, which hostauth does
   not gate; `s0a-reality-probe.mjs`'s guest fetches go to a guest port and
   must never carry the host token.

10. **The m11 probe and the proxy still use the deleted self-dev start
    route.** `scripts/m11_selfdev_episode_probe.mjs:432` and `:534` POST
    `/api/computers/{id}/self-development/operations`, which the guest
    answers 405 (api_self_development.go:159-165). The proxy still maps
    that POST to action `propose` with a 1 MiB body allowance
    (internal/proxy/self_development.go:31, :396). The scope
    `computer:self_development:propose` now guards nothing. Auth still
    issues it (internal/auth/handlers.go:1544), SettingsApp offers it in
    its self-development bundle, and `qa_test_computer.mjs` mints it.

## Belief state

- Items 1, 4 (collection path), 5, 6, 7 and 10 are certain from code plus
  a test or a live response. Items 2, 3 and 4 (events) are certain from
  code. Item 9 is latent: nothing fails until the mode flips.
- Item 1 is the only one that breaks an owner today (every editor Save).

## Fix direction (chosen, not yet applied)

1. Client: stop sending authorship fields. Add a frontend contract test
   that reads the server's request struct, so the next drift fails a test.
2. Delete the guest delivery and the never-reached state update from the
   proxy; the response keeps corpusd's `recorded_for_author`.
3. Caller only (another agent is hardening identity headers on
   `fix/auth-boundary-audit`): bind the request to the computer's own
   boot identity (`CHOIR_COMPUTER_ID`, the same source as execution
   identity and the self-development updater), never to cell input.
   Narrow `/api/computers/` to GET in the ProductAPI allowlist, so the
   binding gives the desk reads and no owner act.
4. Delete the events/{digest} case and the corpusd exact-path
   registration.
5. Probe only qdrant by default.
6. Answer unknown `/api` paths with JSON 404 inside the SPA handler.
7. Honour `limit` (default 50, max 500) and add a keyset cursor.
8. Correct both help strings.
9. Send the header file the way ci.yml does, through one shared helper.
10. Replace the two POSTs with the live path (`POST /api/prompt-bar` on the
    armed computer, then find the operation by trajectory). Map the proxy's
    `operations` to read-only. Stop offering the propose scope in the UI
    and the QA mint.

## Remaining error

- Auth must keep **accepting** `computer:self_development:propose` for now:
  computers serve their own SettingsApp from their release, and a busy
  computer keeps an older one until its next idle refresh. Refusing the
  scope at auth would make that older form's self-development bundle fail
  to mint. Remove it from `validScopes` once every computer serves the new
  form.
- corpusd's Store A mirror and `/proposal-deliveries/state` lose their last
  caller here but are left for the audit's §3 sweep, which rechecks citers
  per service.
- Scripts that need more than 500 documents must follow `next_cursor`;
  none do yet.
