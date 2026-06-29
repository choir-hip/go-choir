# Overnight Mission Suite Fix Report - 2026-06-28

Status: local first-wave fixes integrated, not pushed.

Source review backlog: `docs/overnight-mission-suite-review-2026-06-28.md`

Team artifacts:

- `.omo/teams/team-5c919ec9/artifacts/auth-recovery-final-2026-06-28.md`
- `.omo/teams/team-5c919ec9/artifacts/trace-redaction-problem.md`
- `.omo/teams/team-5c919ec9/artifacts/trace-redaction-final.md`
- `.omo/teams/team-5c919ec9/artifacts/health-public-report.md`
- `.omo/teams/team-5c919ec9/artifacts/base-ownership-report.md`

This document was updated after the first-wave report to include additional local API-key/proxy, gateway-health routing, gateway circuit-breaker test-confidence, and runtime trace/API deletion-safety fixes integrated by the leader in the main worktree.

## Fixes Integrated

### Auth recovery token and session revocation

Mutation class: red.

Protected surfaces: account recovery, passkey enrollment, session revocation, legal data-flow disclosure.

Rollback path: revert `internal/auth/recovery.go`, `internal/auth/recovery_test.go`, and the two legal-doc edits.

Heresy delta: repaired raw-token disclosure and access-only current-session revocation bypass; updated stale recovery/legal text.

Changes:

- `POST /auth/recovery/request` no longer returns the raw recovery token in JSON.
- Session revocation now requires a valid refresh cookie for the current session and rejects access-only current-session revocation.
- Legal docs now disclose email-based passkey recovery and hashed recovery token data.

### Auth global route rate limiting

Mutation class: red.

Protected surfaces: auth endpoint availability and anti-abuse controls.

Rollback path: revert `cmd/auth/main.go`, `cmd/auth/main_test.go`, `internal/auth/rate_limit.go`, `internal/auth/rate_limit_test.go`, and the `clientIP` parser change in `internal/auth/recovery.go`.

Heresy delta: repaired the M7 gap where general auth endpoints had no 10-per-IP-per-minute route limiter, and repaired the recovery/global limiter's prior trust in the spoofable leftmost `X-Forwarded-For` value.

Changes:

- Auth service route registration now wraps every `/auth/*` handler with a shared per-IP fixed-window limiter.
- The limiter defaults to 10 requests per IP per minute.
- The client-IP parser now uses the proxy-appended rightmost `X-Forwarded-For` value when present, falling back to `RemoteAddr`.
- Added a `cmd/auth` route-table regression that drives `/auth/session` through the real server mux and proves the route-level limiter returns `429`.
- Added auth-package limiter tests for handler bypass prevention and spoofed-leftmost `X-Forwarded-For` handling.

### Frontend protected-route transient auth renewal

Mutation class: red.

Protected surfaces: frontend auth/session renewal, mounted signed-in shell state, and protected API request handling.

Rollback path: revert `frontend/src/lib/auth.js` and `frontend/tests/auth-renewal-transient.spec.js`.

Heresy delta: repaired the C04 gap where `fetchWithRenewal` returned the original protected-route `401` after `/auth/session` failed transiently during renewal, allowing callers that branch on `res.status === 401` to dispatch `authexpired` and log out a still-valid user during auth-service restarts.

Changes:

- `fetchWithRenewal` now throws `TransientAuthError` when a protected request gets `401` and `/auth/session` renewal cannot be checked due to transient auth-service failure.
- Definitive signed-out renewal still throws `AuthRequiredError`.
- Added a browser-runtime Playwright regression that imports `auth.js` through Vite and stubs the protected route plus `/auth/session` to prove the transient path is not misclassified as auth-required.

### User-reachable privacy and terms pages

Mutation class: orange frontend/product surface.

Protected surfaces: public legal disclosures, registration disclosure, and SPA route behavior.

Rollback path: revert `DESIGN.md`, `frontend/src/App.svelte`, `frontend/src/lib/AuthEntry.svelte`, `frontend/src/lib/LegalDocument.svelte`, `frontend/tests/legal-routes.spec.js`, the `frontend/public/legal/*` assets, and the legal-doc heading/appendix cleanup.

Heresy delta: repaired the M13 gap where privacy/terms existed only as internal Markdown, were not discoverable from the product, and included internal mission-evidence appendices inside the public legal artifacts.

Changes:

- Added `DESIGN.md` documenting the existing Choir theme variables and public reader pattern before adding UI.
- Added public SPA routes for `/privacy` and `/terms`.
- Added static frontend legal assets under `frontend/public/legal/`.
- Removed internal "Conjecture Verdict" appendices from the public legal docs.
- Added Privacy and Terms links to the signed-out passkey entry fine print.
- Added Playwright coverage proving both routes render, omit the internal appendix text, and the auth entry exposes both links before registration.

### Trace redaction before Dolt persistence

Mutation class: red.

Protected surfaces: trace/evidence persistence and PII handling.

Rollback path: revert `cmd/sandbox/main.go`, `cmd/sandbox/main_test.go`, `internal/trace/redact.go`, and `internal/trace/redact_test.go`.

Heresy delta: repaired the M20/M21 composition gap where production sandbox mounted raw `trace.NewDoltStore`, and repaired the redaction-error fallback that preserved raw payload bytes when a redactor returned an error.

Changes:

- Sandbox trace persistence now wraps the Dolt trace store with `trace.NewRedactingStore(traceStore, trace.NewPipelineFromConfig(trace.RedactionConfig{}))`.
- Added a sandbox-level SQL-store proof that email and phone values are redacted before the inner store receives payloads.
- `RedactingStore` now preserves the trace event envelope on redaction failure but replaces the unsafe payload with `{"redaction_error":"payload_dropped"}` before persistence.
- Redaction-failure warning logs no longer include the redactor error string, because a failing redactor can include raw payload bytes in its error.

### Trace query owner scoping

Mutation class: red.

Protected surfaces: trace/evidence query API, owner privacy, and parent-chain disclosure.

Rollback path: revert `internal/trace/store.go`, `internal/trace/redact.go`, `internal/trace/query.go`, `internal/trace/query_test.go`, and `internal/runtime/trace_wiring_test.go`.

Heresy delta: repaired trace read-path owner leaks where run listing trusted the first returned event as proof for the whole collection, and single-event detail could return cross-owner ancestors through `parent_chain`.

Changes:

- Trace store contract now includes owner-scoped event detail and run-list methods.
- `SQLStore` scopes those reads in SQL using `owner_id`.
- `RedactingStore` delegates the new owner-scoped query methods so the production wrapper remains a complete `trace.Store`.
- HTTP run listing now reads with `ListByRunForOwner`; if a run exists only for another owner, it still returns `404` instead of an empty authorized list.
- HTTP single-event detail now reads with `GetForOwner` and walks the parent chain through owner-scoped lookup, so cross-owner parents are omitted like missing parents.
- Added regressions for mixed-owner run listing and cross-owner parent-chain disclosure.
- Updated the runtime trace test stub to satisfy the expanded store interface.

### Runtime trace trajectory API restore

Mutation class: red.

Protected surfaces: Trace/evidence observability, run acceptance evidence links, and product-path browser acceptance probes.

Rollback path: revert `internal/runtime/api.go`, `internal/runtime/api_trace.go`, `internal/runtime/api_trace_trajectory.go`, `internal/runtime/api_trace_agents.go`, `internal/runtime/api_trace_moments.go`, and `internal/runtime/api_trace_test.go`.

Heresy delta: repaired the C64 runtime deletion regressions where `/api/trace/trajectories` and `/api/trace/trajectories/{trajectory_id}` were removed while frontend/probe/acceptance callers still required the trace URL namespace, stale comprehensive tests no longer built, and the claimed `work_id` fallback deletion had not actually removed the `defaultChannelID` fallback.

Changes:

- Restored owner-authenticated `/api/trace/trajectories` index and `/api/trace/trajectories/{trajectory_id}` snapshot routes.
- Restored `/api/trace/trajectories/{trajectory_id}/moments/{moment_id}` detail lookup for evidence links emitted by run acceptance.
- The compatibility snapshot returns probe-visible trajectory `live`/`state`/count fields, agent nodes, edges, moment summaries, search summary, and run acceptances.
- Reads remain owner scoped through `ListRunsByOwner`, `ListEventsByTrajectory`, `ListChannelMessagesByTrajectory`, and `ListRunAcceptancesByTrajectory`; another owner receives `404` for the same trajectory id.
- Added a non-comprehensive runtime regression proving the restored trace URL returns app-promotion moment summaries and denies another owner.
- Updated stale comprehensive prompt/Texture test fixtures so the tagged runtime package builds after the deleted prompt field and public revision request author-field cleanup.
- Removed the remaining `metadata["work_id"]` channel-id fallback from `defaultChannelID`.
- Added a regression proving `work_id` metadata cannot override the canonical Super agent channel when `channel_id` is absent.

### Runtime direct event append trace projection

Mutation class: red.

Protected surfaces: trace/evidence persistence, product event observability, and PII redaction ordering.

Rollback path: revert `internal/runtime/runtime.go`, `internal/runtime/product_events.go`, `internal/runtime/browser.go`, `internal/runtime/app_promotion.go`, `internal/runtime/tools_researcher.go`, `internal/runtime/api.go`, `internal/runtime/texture_agent_revision.go`, and `internal/runtime/trace_wiring_test.go`.

Heresy delta: repaired the C39 residual where production runtime paths that built their own `types.EventRecord` called `rt.store.AppendEvent` directly, bypassing trace projection and therefore bypassing the redacting trace store mounted in sandbox.

Changes:

- Added `Runtime.appendEventRecord` as the single runtime helper for append-to-store plus trace projection.
- `emitEvent`, `persistEvent`, health events, product events, browser session events, app-promotion events, researcher channel messages, internal run-event append API, and Texture revision/decision events now use that helper.
- Added a regression proving `EmitProductEvent` projects to the mounted trace store.
- Current production runtime package search now leaves only the centralized helper and submission-store abstraction using `AppendEvent` directly.

### Runtime comprehensive fixture repairs

Mutation class: yellow.

Protected surfaces: none directly; this repairs comprehensive runtime proof coverage for behavior already changed in the local stack.

Rollback path: revert the touched comprehensive runtime test fixtures and the processor prompt-contract assertion update.

Heresy delta: repaired stale comprehensive fixtures that no longer matched current actor dispatch, structured Texture block ids, public Texture revision authorship, and single-block patch semantics.

Changes:

- `TestTaskRecoveryAcrossRestart` now installs the test actor dispatch hook before calling `StartRun`.
- Processor-to-Texture comprehensive coverage now keeps the spawned Texture run pending while the test manually executes `patch_texture`, derives the editable paragraph block id from the stored structured `body_doc`, and uses a single-block article edit.
- Texture comprehensive fixtures now include required `body_doc` values for appagent revisions and use `rewrite_texture` for whole-document markdown rewrites.
- `TestDelegateWorkerVMLocalWorktreeIsolationUsesToolCWD` now installs test actor dispatch on the worker runtime serving `/internal/runtime/runs`.
- The processor article completion contract assertion now includes the source-inventory caveat added to the prompt.

### Coagent update authoritative channel routing

Mutation class: red-adjacent runtime behavior.

Protected surfaces: durable coagent source-packet delivery, Texture worker-update mailbox routing, and channel-message audit history.

Rollback path: revert `internal/runtime/tools_researcher.go` and the related comprehensive routing assertions in `internal/runtime/agent_tools_test.go`.

Heresy delta: repaired a worker-update routing bug where a Super run with stale `channel_id` metadata could send an explicitly addressed Texture update to the wrong channel instead of the target Texture agent's authoritative channel.

Changes:

- `resolveCoagentFindingsTarget` now preserves the existing parent/requester Texture precedence, then resolves an explicit addressed agent before falling back to the sender run's channel context.
- Added/updated comprehensive regressions proving explicit Texture targets use their canonical channel and parent Texture requesters still override decoy explicit agents.
- Updated the researcher content-item comprehensive assertion to require the current structured `update_coagent` source-packet checkpoint instruction instead of the retired findings-update wording.

### Worker VM lease invalidation and machine-class dedupe proof

Mutation class: red-adjacent runtime behavior.

Protected surfaces: worker VM leasing, Super delegation retry behavior, and durable tool-result evidence used for dedupe.

Rollback path: revert `internal/runtime/tools_vmctl.go` and the request-worker fixture updates in `internal/runtime/agent_tools_test.go`.

Heresy delta: repaired the stale-worker request cache behavior where a failed delegation to an unreachable worker did not record invalidation proof or retire the bad lease, and repaired stale comprehensive request-worker fixtures so direct tool execution preserves the durable tool-result facts that product tool-loop execution would emit.

Changes:

- `delegate_worker_vm` unreachable-worker results now record `worker_request_cache_invalidated`.
- When vmctl is configured and the failed result names a worker id, invalidation best-effort hibernates that worker through vmctl and records `worker_hibernated` or a warning.
- The comprehensive vmctl fixture now exposes `/internal/vmctl/hibernate-worker` for the unreachable-worker path.
- The direct request-worker machine-class test now appends each synthetic `request_worker_vm` result into the run event log, matching the product tool-loop evidence contract used by store-backed dedupe.
- `appendRuntimeToolResult` now creates unique event ids, allowing one test run to record multiple results for the same tool and run.

### Coagent terminal parent-channel notifications

Mutation class: red-adjacent runtime behavior.

Protected surfaces: coagent lifecycle evidence, parent/child channel coordination, and failure isolation reporting.

Rollback path: revert the `internal/runtime/runtime.go` lifecycle notification helper and call sites.

Heresy delta: repaired the comprehensive worker concurrency regression where coagent child runs reached terminal state but no longer posted `result`/`error` messages to the requester channel, leaving parent-channel evidence empty.

Changes:

- Completed coagent runs now append a `role=result` channel message to the requester run channel after the completed state is durably persisted.
- Failed coagent runs now append a `role=error` channel message to the requester run channel after the failed state is durably persisted.
- Parent notification is a no-op for runs without requester provenance.
- The channel message includes child run id, child agent id, owner-scoped persistence, and trajectory id when available, then emits the normal channel-message trace event.

### Persistent Super inbox isolation and follow-up runs

Mutation class: red-adjacent runtime substrate.

Protected surfaces: persistent Super mailbox delivery, update_coagent execution requests, and durable worker-update ownership.

Rollback path: revert `internal/runtime/super_controller.go`, `internal/runtime/tools_texture.go`, and the related comprehensive test expectation updates.

Heresy delta: repaired persistent-Super inbox regressions where runs lost objective-specific prompt evidence, duplicate same-run `request_super_execution` left a pending delivery after ownership was established, and a fresh execution request arriving during an active Super run could be absorbed into the same activation instead of spawning a follow-up run after the active run completed.

Changes:

- Persistent Super runs now use the existing `buildPersistentSuperUpdatePrompt` instead of a generic prompt, so each run carries the execution-request content it owns.
- Duplicate same Texture-run requests now mark the existing matching pending Super update delivered to the active Super loop once ownership is known.
- An active persistent Super run with assigned update ids no longer pulls fresh mailbox updates mid-activation; those remain pending for a follow-up run.
- Follow-up reconciliation after the first Super run completes now starts a new run for the next pending execution request and keeps its prompt isolated to that request.

### Conductor Texture route creation idempotency

Mutation class: red-adjacent runtime behavior.

Protected surfaces: conductor-to-Texture route creation, document identity, and initial Texture revision runs.

Rollback path: revert the `ensureConductorTextureRoute` serialization change in `internal/runtime/runtime.go`.

Heresy delta: repaired the concurrent conductor `spawn_agent role=texture` race where two simultaneous calls from the same conductor run could both pass the stored-route check and create separate Texture documents/runs.

Changes:

- Conductor Texture route materialization now uses the existing Texture mutation mutex around the stored-route recheck and create path.
- The second concurrent caller re-reads the conductor run after the first caller persists `doc_id`, `initial_loop_id`, and related route metadata, then returns the existing route instead of creating a duplicate document.

### Texture worker wake debounce, prompt context, and provenance

Mutation class: red-adjacent runtime behavior.

Protected surfaces: Texture worker-update delivery, canonical revision scheduling, run provenance, and worker-message evidence in Texture prompts.

Rollback path: revert the Texture wake changes in `internal/runtime/channel_store.go`, `internal/runtime/runtime.go`, `internal/runtime/texture_agent_revision.go`, `internal/runtime/texture_controller.go`, `internal/runtime/test_helpers_test.go`, and `internal/runtime/tools_coagent.go`.

Heresy delta: repaired the comprehensive Texture auto-wake regressions where legacy addressed channel messages did not create typed worker updates, wake runs lost requester provenance, wake prompts omitted pending worker-message context, rapid messages could bypass debounce, and stale no-write Texture runs could leave pending worker updates without a follow-up revision.

Changes:

- Legacy addressed `ChannelCast` messages to `texture:<docID>` now create typed `CoagentSourcePacket` worker updates while preserving the durable channel-message audit row.
- Legacy Texture worker-message wakes now use the existing Texture wake scheduler instead of directly dispatching the actor, so rapid messages batch and fake-clock debounce tests remain meaningful.
- Texture wake scheduling now keeps one timer per owner/document and replaces prior pending timers for the same document.
- Wake-created Texture revision runs derive `requested_by_run_id` from the source channel message and persist it on the run.
- Conductor-created and universal-wire-created Texture revision runs also carry parent run provenance through the same revision request field.
- Wake-created Texture prompts now include recent addressed worker messages by using the existing `recentWorkerMessages` prompt context.
- No-write Texture runs that injected worker updates now still trigger post-completion reconciliation so pending updates can be consumed by a follow-up revision instead of being stranded.
- The test actor dispatch fallback now resolves owner id from pending worker updates when the durable agent row is absent, matching the legacy Texture worker-message fixtures.

### Actor durability and product-path backpressure

Mutation class: red-adjacent runtime substrate.

Protected surfaces: durable actor mailbox delivery, actor memory snapshots, and sandbox runtime backpressure behavior.

Rollback path: revert `internal/actor/actor.go`, `internal/actor/actor_test.go`, `cmd/sandbox/main.go`, and `cmd/sandbox/main_test.go`.

Heresy delta: repaired the M23/C01 composition gaps where channel-delivered updates could advance actor memory despite failed durable `MarkProcessed`, idle passivation could deregister an actor despite failed `SaveSnapshot`, and sandbox product construction never enabled the backpressure options added by M23.

Changes:

- `processOne` and `processBacklog` now update in-memory actor state and skip sets only after `MarkProcessed` succeeds.
- If `MarkProcessed` fails, the update remains unprocessed and eligible for the immediate backlog retry path with the previous memory state.
- Idle passivation now keeps the actor resident and retries later when `SaveSnapshot` fails instead of deleting residency after an unsaved memory transition.
- Sandbox runtime construction now enables actor backpressure by default in blocking mode with capacity `1000` and send timeout `5s`.
- Added `RUNTIME_ACTOR_BACKPRESSURE_ENABLED`, `RUNTIME_ACTOR_BACKPRESSURE_BLOCKING`, `RUNTIME_ACTOR_INBOX_CAPACITY`, and `RUNTIME_ACTOR_SEND_TIMEOUT` knobs for operational tuning and emergency disablement.
- Added actor fault-injection regressions for `MarkProcessed` and `SaveSnapshot` failures.
- Added sandbox configuration regressions proving product defaults enable blocking backpressure and env overrides are parsed.

### Public gateway health error sanitization

Mutation class: orange/red-adjacent.

Protected surfaces: public unauthenticated health endpoint output.

Rollback path: revert `internal/gateway/handlers.go` and `internal/gateway/service_health_test.go`.

Heresy delta: repaired public checker-error leakage from `/health/{service}`.

Changes:

- Public service-health failures now return generic `dependency check failed` instead of `err.Error()`.
- The no-secret regression test now asserts the response body does not contain injected secret/auth material.

### Public gateway health route exposure

Mutation class: red-adjacent deployment routing.

Protected surfaces: public health/readiness routing and deployed operator observability.

Rollback path: revert the Caddy routing additions in `nix/node-a.nix` and `nix/node-b.nix`.

Heresy delta: repaired the local configuration gap where `/health/ready` and `/health/{service}` fell through to the SPA shell instead of reaching the gateway health handlers.

Changes:

- Node B `choir.news` Caddy config now keeps exact `/health` on proxy `8082` and routes `/health/*` to gateway `8084`.
- Node A `choir-ip.com` Caddy config mirrors the same route split.
- This is local configuration only until pushed and deployed; current staging probes still reflect the previous deployment.

### Universal Wire platform computer API recovery

Mutation class: red-adjacent platform routing.

Protected surfaces: Universal Wire publication surface, proxy-to-vmctl desktop resolution, and public platform computer routing.

Rollback path: revert the Universal Wire platform resolve branches in `internal/proxy/handlers.go` and `internal/vmctl/handlers.go`, plus their focused tests.

Heresy delta: repaired the deployed symptom where Universal Wire showed a `502` load failure instead of reaching the platform computer's `/api/universal-wire/stories` handler. The runtime handler already supports an empty-but-healthy `200` response, so a `502` indicates proxy/vmctl routing failure, not an expected no-articles state.

Changes:

- Proxy sandbox URL resolution now treats `universal-wire-platform/platform` as a provisionable platform computer by calling vmctl `resolve` instead of lookup-only resolution.
- vmctl `HandleResolve` now permits the Universal Wire platform identity, calls `EnsureUniversalWirePlatformComputer`, and returns the active published sandbox URL.
- Added regressions proving proxy uses resolve-only for the platform Wire computer and vmctl resolve boots/returns the stable platform VM.
- This is local only until pushed and deployed; current staging still showed Universal Wire `0 articles` plus `Universal Wire load failed: 502` in the Chrome DOM inspected on 2026-06-29.

### Base API owner isolation and blob validation

Mutation class: orange.

Protected surfaces: Base provenance, owner isolation, journal parent chaining, content-addressed blob integrity.

Rollback path: revert the Base API and journal edits.

Heresy delta: repaired caller-controlled owner writes, cross-owner reads/delta exposure, missing blob-reference validation, and owner-agnostic parent chains.

Changes:

- Base item writes reject explicit `owner_id` values that differ from the authenticated API-key user.
- Item events persist `OwnerID` from the authenticated user, not caller input.
- Item GET, status, and delta derive only from the authenticated owner events.
- File item writes validate item kind, version id, parent id shape, blob ref shape, blob existence, and optional content hash before journal append.
- Memory and SQLite journal parent chains are keyed by `(owner_id, item_id)`.
- Memory and SQLite journal device cursors now have owner-scoped APIs keyed by `(owner_id, device_id)`, while the old device-only wrappers remain for compatibility.
- SQLite journal startup migrates legacy `device_cursors(device_id)` tables into the owner-scoped cursor schema with legacy rows assigned to the empty compatibility owner.
- Integration added one extra parent-hash regression beyond the member patch: an owner update after an interleaved same-item event from another owner must still verify the correct parent hash.

### Base journal append durability

Mutation class: orange.

Protected surfaces: Base journal append-only persistence, parent-chain validity, and SQLite-backed replay integrity.

Rollback path: revert `internal/base/journal/journal.go`, `internal/base/journal/sqlite.go`, `internal/base/journal/sqlite_cursor.go`, `internal/base/journal/journal_test.go`, `internal/base/journal/journal_cursor_test.go`, and `internal/base/journal/sqlite_test.go`.

Heresy delta: repaired the C36/C37 gaps where a first event for an item could declare a bogus parent and where SQLite append computed head, predecessor, parent hash, and insert outside one SQLite transaction.

Changes:

- Memory and SQLite journals now reject a non-empty `ParentEventID` on the first event for an owner/item chain.
- SQLite journal append now runs head read, predecessor lookup, parent hash lookup, insert, and commit inside a pinned `BEGIN IMMEDIATE` transaction.
- SQLite journal opens with a busy timeout and a single connection per handle so independent handles serialize append attempts instead of racing head reads.
- Added regressions for first-parent rejection in memory and SQLite journals.
- Added a file-backed two-handle SQLite append test proving a second handle observes the first handle's head and parent chain.

### Desktop Base sync conflict cursor pause

Mutation class: orange.

Protected surfaces: Base desktop sync state, conflict handling, and local cursor acknowledgement.

Rollback path: revert `internal/desktop/sync.go` and `internal/desktop/sync_test.go`.

Heresy delta: repaired the desktop sync bug where unresolved conflicts still persisted the remote delta cursor and synced snapshot as if the conflict had been accepted.

Changes:

- When conflicts remain unresolved, desktop sync now computes the safe contiguous remote cursor before the first unresolved conflict item.
- The persisted synced state now applies only delta events up to that safe cursor instead of blindly saving `delta.Cursor`.
- Added a regression to `TestSyncEngineConflictPauses` proving the saved cursor and synced version remain at the prior ancestor while a conflict is pending.
- Added a focused helper test proving unordered delta events still produce a safe cursor that stops before the blocked event.

### Base blob download and desktop file content sync

Mutation class: orange.

Protected surfaces: Base content-addressed blob API, authenticated owner isolation, and desktop sync file materialization.

Rollback path: revert `internal/base/api/handlers.go`, `internal/base/api/handlers_test.go`, `internal/desktop/client.go`, `internal/desktop/sync.go`, and `internal/desktop/sync_test.go`.

Heresy delta: repaired the M4/M5 gap where Base exposed blob upload only and desktop remote downloads wrote empty placeholders instead of content.

Changes:

- Added authenticated `GET /api/base/blobs/{ref}` to return raw blob bytes.
- Blob download is owner-gated: the authenticated owner must have a journal event referencing the blob ref, so a `read:base` key cannot fetch arbitrary known hashes from another owner.
- Added desktop `BaseClient.GetBlob`.
- Desktop remote file downloads now fetch the item version's blob ref and write the downloaded bytes to the local file instead of an empty placeholder.
- Blob upload now rejects bodies larger than 64 MiB with `413` instead of silently storing the first 64 MiB as a valid truncated blob.
- Added API regressions for successful blob download, missing read scope, and foreign-owner blob non-exposure.
- Added desktop sync coverage proving a remote file download writes the expected bytes.
- Added an oversized-upload regression using a streaming reader so the boundary is tested without allocating a 64 MiB fixture.

### Base API product route mounting

Mutation class: orange.

Protected surfaces: Base API route exposure, API-key scope trust boundary, sandbox persistence.

Rollback path: revert `cmd/sandbox/main.go`, `cmd/sandbox/main_test.go`, `internal/base/api/handlers.go`, and `internal/base/api/handlers_test.go`.

Heresy delta: repaired the C48/C49 product-surface gap where proxy authorized `/api/base/*` requests and desktop clients called them, but the sandbox did not mount the Base API routes, and the Base handler only accepted raw Bearer tokens that the proxy correctly strips before upstream forwarding.

Changes:

- Sandbox now mounts `/api/base/*` to the Base API handler.
- Base blobs persist under a `base/blobs` directory beside the runtime store, and the Base journal persists under `base/journal.sqlite`.
- Base API authentication now accepts proxy-validated `X-Authenticated-User`, `X-Authenticated-Email`, and comma-separated `X-Authenticated-Scopes` headers, while preserving the direct Bearer-token validator path.
- Added Base API regressions proving trusted proxy headers authorize `read:base` requests and reject missing scopes.
- Added a sandbox route-table regression proving `GET /api/base/delta` returns the empty owner-scoped delta shape through the mounted route.

### Base planner content-addressed equality

Mutation class: orange.

Protected surfaces: Base reconciliation semantics and conflict generation.

Rollback path: revert `internal/base/planner/planner.go` and `internal/base/planner/planner_test.go`.

Heresy delta: repaired the planner false-conflict behavior where identical content with different non-empty version IDs was treated as divergent.

Changes:

- `versionsEqual` now treats versions as equal when their version IDs match, or when both carry the same non-empty `BlobRef` and `ContentHash`.
- Added add/add and edit/edit regressions proving content-identical versions with distinct IDs converge without actions or conflicts.
- Existing different-content conflict tests still pass, preserving explicit conflict behavior when blob/content hashes differ.

### Base path-collision participant identity

Mutation class: orange.

Protected surfaces: Base reconciliation semantics, desktop conflict blocking, conflict resolution, sync status, and File Provider conflict projection.

Rollback path: revert `internal/base/planner/planner.go`, `internal/base/testkit/scenarios_test.go`, `internal/desktop/conflicts.go`, `internal/desktop/conflicts_test.go`, `internal/desktop/sync.go`, `internal/desktop/syncstatus.go`, `internal/desktop/syncstatus_test.go`, `internal/desktop/fileprovider/types.go`, `internal/desktop/fileprovider/bridge.go`, and `internal/desktop/fileprovider/bridge_test.go`.

Heresy delta: repaired the one-sided path-collision conflict contract where the planner keyed the conflict to the remote item only, leaving the local participant addressable only through an embedded version/reason string and allowing downstream skip/resolve paths to act on the wrong item.

Changes:

- Planner conflicts now carry structured `LocalItemID` and `RemoteItemID`.
- Add/add path-collision conflicts fill both participant IDs while preserving both participant versions.
- Desktop conflict records, sync status, and File Provider conflict JSON expose both participant IDs.
- Desktop action skipping now treats either collision participant as conflicted, so local upload and remote download actions for the colliding path are both blocked until resolution.
- Conflict resolution lookup now accepts either participant ID and stores the resolution on the canonical conflict record.
- `keep_local`, `keep_remote`, and `keep_both` resolution execution use the local or remote participant ID intentionally instead of blindly using the remote/canonical conflict key.

### macOS File Provider bridge contract and compile blockers

Mutation class: orange.

Protected surfaces: native macOS File Provider extension behavior, HTTP-over-Unix-socket IPC, Finder file read/write/move callbacks, and the Swift/Go bridge JSON contract.

Rollback path: revert `cmd/desktop/syncservice.go`, `cmd/desktop/syncservice_test.go`, `cmd/desktop/frontend/dist/.gitkeep`, `macos/ChoirFileProvider/ChoirFileProviderBridge.swift`, `macos/ChoirFileProvider/UnixSocketTransport.swift`, `macos/ChoirFileProvider/ChoirFileProviderExtension.swift`, `macos/ChoirFileProvider/Info.plist`, `macos/ChoirFileProvider/ChoirFileProvider.entitlements`, and `macos/README.md`.

Heresy delta: repaired the Swift/Go snake_case JSON mismatch, repaired bridge success reporting on HTTP error responses, repaired the custom URLProtocol socket-path configuration defect, repaired direct Swift type-check blockers in the extension sources, repaired stale metadata lookup after move/rename, and repaired the host/extension socket-path mismatch by moving the default socket into the shared app-group container.

Changes:

- The Swift bridge now uses a shared encoder/decoder configured for snake_case keys and ISO-8601 dates, including POST bodies such as move/delete/mkdir requests.
- The synchronous bridge request helper now waits for a response, validates that the response is HTTP 2xx, decodes the bridge error envelope on failure, and throws instead of returning an error body as success.
- `UnixSocketTransport` now uses URLProtocol class-level socket configuration because `URLSession` instantiates protocol classes itself; it no longer loses the configured socket path.
- The Unix-socket transport now type-checks with the local macOS SDK by using Darwin socket APIs directly, copying the socket path into `sockaddr_un`, and handling throwable `FileHandle.close`.
- `ChoirFileProviderExtension` now conforms to the current `NSFileProviderReplicatedExtension` signatures, including the `fetchContents` completion item parameter and `invalidate`.
- Directory creation now derives folder intent from `contentType`, content changes use `.contents`, and change enumeration uses the current Swift `didUpdate(_:)` observer method.
- Move/rename completion now re-enumerates the destination parent derived from the updated path, so cross-folder moves can return fresh metadata instead of falling back to the stale pre-move item.
- The File Provider extension `Info.plist`, entitlements, Swift default socket path, and Go host bridge now agree on the default app-group identifier `group.news.choir`.
- The Go host bridge now listens under `~/Library/Group Containers/<app-group>/Choir/fileprovider.sock` on macOS, with `CHOIR_FILEPROVIDER_APP_GROUP_ID` and `CHOIR_FILEPROVIDER_SOCKET_PATH` overrides for signed/team-scoped packaging.
- Added the missing `cmd/desktop/frontend/dist/.gitkeep` placeholder that the desktop module `.gitignore` already expected, allowing the Wails embed directive to compile in a clean source checkout.

### API-key proxy secrecy, scope checks, and deploy config

Mutation class: red.

Protected surfaces: API-key authorization, proxy trust boundary, deployment routing/configuration.

Rollback path: revert `internal/proxy/handlers.go`, `internal/proxy/handlers_test.go`, and the `nix/node-b.nix` proxy service edits.

Heresy delta: repaired the protected reverse-proxy path leaking bearer/cookie credentials upstream, repaired decorative scope propagation for protected sandbox and proxy-owned routes, and repaired missing Node B proxy auth DB wiring.

Changes:

- Reverse-proxy director now strips `Authorization` and `Cookie` before forwarding to sandbox while preserving trusted `X-Authenticated-*` context.
- API keys now require route/method scopes before the proxy forwards protected routes:
  - `/api/base/*`: `read:base` for read methods, `write:base` for mutating methods.
  - `/api/texture/*`: `read:texture` for read methods, `write:texture` for mutating methods.
  - other protected sandbox routes, bootstrap, and WebSockets: `read:runtime` for read methods, `write:runtime` for mutating methods.
  - `admin` remains an override scope.
- Direct proxy-owned handlers that authenticate outside the generic reverse proxy now also call the shared API-key scope gate, including compute status/recovery, app-change package routes, email, notification, platform publication/proposal, and platform texture reads.
- Node B proxy service now sets `PROXY_AUTH_DB_PATH=/var/lib/go-choir/auth/auth.db` and grants the proxy read/write path access to `/var/lib/go-choir/auth` so bearer validation can work after deploy.

### SBOM manifest and required-package completeness gate

Mutation class: yellow.

Protected surfaces: CI evidence quality, supply-chain artifact diagnostics, and main-push SBOM completeness policy.

Rollback path: revert the `.github/workflows/ci.yml` SBOM loop and upload-artifact edits.

Heresy delta: repaired the C67 diagnostic regression where skipped package builds printed only the first 20 stderr lines and could hide the root cause, and repaired the remaining silent-success class where required SBOM artifacts could be missing while the job stayed green.

Changes:

- SBOM package-build failures now print a bounded first 40 stderr lines and last 80 stderr lines.
- This keeps logs bounded while preserving the trailing Nix error that usually carries the actionable hash, missing source, or fetch failure.
- The SBOM loop now writes `sboms/manifest.jsonl` with one machine-readable status row per package.
- Required packages are `auth`, `proxy`, `gateway`, `maild`, `maildctl`, `platformd`, `sandbox`, `sourcecycled`, `vmctl`, `frontend`, and `zot`.
- Optional `obscura` may still skip with a warning; any skipped required package increments `required_failed` and fails the SBOM step after the loop.
- SBOM artifact upload now runs with `if: always()` and includes both generated `sbom.json` files and the manifest, with `if-no-files-found: error`.

### Gateway circuit-breaker timeout literals

Mutation class: yellow.

Protected surfaces: gateway health/circuit-breaker test confidence.

Rollback path: revert `internal/gateway/circuit_breaker_test.go`.

Heresy delta: repaired the remaining gateway side of the bare `OpenTimeout: 3600` test-literal class after the qdrant timeout fix.

Changes:

- Replaced four `OpenTimeout: 3600` gateway test literals with `OpenTimeout: time.Hour`.
- This preserves the intended long-open test window and avoids silently using a 3.6 microsecond `time.Duration`.

### vmctl ownership snapshot returns

Mutation class: red.

Protected surfaces: vmctl lifecycle routing, sandbox proxy URL resolution, worker routing, and persistent user computer ownership state.

Rollback path: revert `internal/vmctl/ownership.go` and `internal/vmctl/vmctl_test.go`.

Heresy delta: repaired the remaining C45/C46 vmctl snapshot review gaps where handler-facing APIs returned or read live `*VMOwnership` pointers after releasing `r.mu`, allowing callers or concurrent lifecycle transitions to race with registry-owned state.

Changes:

- `LiveSandboxURL` now copies the ownership record and VM manager pointer while holding `r.mu`.
- URL resolution after unlock reads only the snapshot, not the live map pointer.
- Added a focused `-race` regression that overlaps repeated `LiveSandboxURL` calls with `RefreshVMForDesktop` mutating the sandbox URL.
- `ResolveOrAssignDesktopContext` now returns snapshot copies for active, resumed, newly assigned, and pending-waiter ownerships.
- First-boot completion now snapshots the newly assigned ownership while still holding `r.mu` before notifying waiters or returning to the first caller.
- `RequestWorker` now returns snapshot copies for both newly assigned workers and reused active worker leases.
- Added regressions proving caller mutation of returned desktop/worker ownerships cannot mutate registry state, plus a `-race` regression that reads a resolved ownership while refresh mutates the live registry entry.

## Verification

Passing commands in the main worktree:

- `nix develop -c go test ./internal/auth -run 'TestRecoveryRequest(SucceedsForExistingUser|ResponseDoesNotContainRawToken|SucceedsForNonexistentUser|RateLimitsByEmail|RateLimitsByIP)|TestRevokeSession(SucceedsForOtherSession|RejectsCurrentSession|RejectsAccessOnlyRequest|RejectsMismatchedRefreshCookie|RejectsNonOwner|RejectsUnauthenticated)$' -count=1`
- `nix develop -c go test ./internal/auth -count=1`
- `nix develop -c go test ./internal/auth -run 'TestIPRateLimiter|TestClientIPUsesProxyAppendedForwardedAddress|TestRecoveryRequestRateLimitsByIP' -count=1`
- `nix develop -c go test ./cmd/auth -count=1`
- `nix develop -c go test -race ./cmd/auth ./internal/auth -count=1`
- `rg -n "HandleFunc\(" cmd/auth/main.go` showed all registered auth routes wrapped with `limit(...)`.
- `npx playwright test tests/auth-renewal-transient.spec.js --project chromium` from `/Users/wiz/go-choir/frontend` against local Vite at `127.0.0.1:4173`.
- `npm run build` in `frontend/` passed. Existing unrelated Svelte warnings remain in `UniversalWireApp.svelte`, `CalendarApp.svelte`, and `SlidesApp.svelte`.
- `npx playwright test tests/legal-routes.spec.js --project chromium` passed in `frontend/`.
- Browser visual QA against local Vite at `http://127.0.0.1:4173` captured `/privacy` and `/terms` at 375x812 and 1280x900; DOM checks reported `overflowX: false` and the expected legal headings.
- `nix develop -c go test ./internal/gateway -run 'TestHandleServiceHealth_(NoSecretsInError|UnhealthyWhenDown|OkWhenHealthy|RouteViaMux)' -count=1`
- `nix develop -c go test ./cmd/sandbox -run 'TestTracePersistenceStoreRedactsPayloadBeforeSQLAppend|TestBuildRuntimeConfigPreservesHostServiceURLs' -count=1`
- `nix develop -c go test ./internal/trace -run '^TestRedactingStore_RedactionFailureDropsOriginalPayloadWithWarning$' -count=1` failed before the fix with `raw PII persisted after redaction failure: not json but alice@example.com here`.
- `nix develop -c go test ./internal/trace -run '^TestRedactingStore_RedactionFailureDropsOriginalPayloadWithWarning$' -count=1`
- `nix develop -c go test ./cmd/sandbox ./internal/trace ./internal/gateway -count=1`
- `nix develop -c go test ./internal/trace -run 'TestHTTPHandler(ListByRunFiltersMixedOwnerEvents|SingleParentChainOmitsCrossOwnerParent)$' -count=1` failed before the fix because mixed-owner run listing exposed Bob's event to Alice and parent-chain detail exposed Bob's parent event through Alice's child.
- `nix develop -c go test ./internal/trace -run 'TestHTTPHandler(ListByRunFiltersMixedOwnerEvents|SingleParentChainOmitsCrossOwnerParent|ListOwnerScopedNotFound|SingleWithChain|SingleOwnerScopedNotFound)$' -count=1`
- `nix develop -c go test ./internal/trace -count=1`
- `nix develop -c go test ./cmd/sandbox ./internal/trace -count=1`
- `nix develop -c go test ./internal/runtime -run '^TestEmitProductEventProjectsToTraceStore$' -count=1` failed before the fix with `expected 1 trace event, got 0`.
- `nix develop -c go test ./internal/runtime -run '^TestEmitProductEventProjectsToTraceStore$' -count=1`
- `rg -n "rt\\.store\\.AppendEvent|h\\.rt\\.Store\\(\\)\\.AppendEvent" internal/runtime -g '*.go'` now finds only the centralized helper and a direct test append.
- `rg -n "AppendEvent\\(" internal/runtime -g '*.go' -g '!**/*_test.go'` now finds only the runtime submission-store abstraction and centralized helper.
- `nix develop -c go test ./internal/runtime -run 'Test(EmitEventProjectsToTraceStore|PersistEventProjectsToTraceStore|EmitProductEventProjectsToTraceStore|PersistSubmittedRunProjectsToTraceStore|EmitEventDegradesGracefullyOnTraceStoreFailure|NilTraceStoreIsNoOp|StopClosesTraceStore|EmitEventProjectsToSQLiteTraceStore)$' -count=1`
- `nix develop -c go test ./internal/runtime -run '^TestDefaultChannelIDIgnoresLegacyWorkID$' -count=1` failed before the fix with `channel_id = "legacy-work-channel"`.
- `nix develop -c go test ./internal/runtime -run '^TestDefaultChannelIDIgnoresLegacyWorkID$|^TestHandleTraceTrajectoriesRestoresAcceptanceSnapshotRoute$' -count=1`
- `nix develop -c go test ./internal/runtime -run '^TestHandleTraceTrajectoriesRestoresAcceptanceSnapshotRoute$' -count=1`
- `nix develop -c go test ./internal/runtime -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^TestAppChangePackageMigratesAcrossCandidateComputers$|^TestHandlePromptListReturnsEffectivePrompts$|^TestTextureAPICreateRevisionUserEdit$' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^$' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^(TestTaskRecoveryAcrossRestart|TestTextureWorkerUpdateRevisionRejectsNoOpPatch|TestTextureAgentRevisionDeliversOwnerRequestToResidentActor|TestTextureAppagentEditCanonicalizesAliasedMarkdownTitle|TestProcessorAndReconcilerProfilesDelegateToTextureOnly)$' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^TestDelegateWorkerVMLocalWorktreeIsolationUsesToolCWD$' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^(TestSubmitWorkerUpdateUsesTextureRequesterMetadataWhenAgentMissing|TestSubmitWorkerUpdateUsesTargetChannelOverExplicitChannel|TestSubmitWorkerUpdateUsesParentAgentOverExplicitAgent)$' -count=1 -v`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^(TestSuperRequestWorkerVMReplacesUnreachableLeaseAfterDelegateFailure|TestSuperRequestWorkerVMDedupesSameRunByMachineClass)$' -count=1 -v`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^(TestSuperRequestWorkerVMReturnsTypedHandle|TestSuperRequestWorkerVMNormalizesStandardMachineClass|TestSuperRequestWorkerVMReusesActiveLeaseUnlessParallelAllowed|TestSuperRequestWorkerVMDedupesSameRunByMachineClass|TestSuperRequestWorkerVMReplacesUnreachableLeaseAfterDelegateFailure)$' -count=1 -v`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^(TestConcurrentWorkers_Spawn5WorkersRapidlyThenVerifyAllComplete|TestConcurrentWorkers_ResultsPostedToParentChannelOnCompletion|TestConcurrentWorkers_FailedChildPostsErrorToParentChannel)$' -count=1 -v`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^(TestRequestSuperExecutionDedupesSameTextureRun|TestPersistentSuperBlockedRunDoesNotStarveFreshInboxDelivery|TestPersistentSuperProcessesConcurrentInboxDeliveriesInFollowupRun)$' -count=1 -v`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^TestConcurrentConductorTextureSpawnsShareRoute$' -count=1 -v`
- `nix develop -c go test -tags comprehensive ./internal/runtime -count=1 -timeout=10m` was rerun on 2026-06-29 and failed after 216.317s. Visible failures included the `TestTextureAgentRevisionRealLLM*` comprehensive tests returning `get task status: status = 404` after provider/context-cancellation noise, so full comprehensive runtime proof remains open.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^(TestTextureWorkerMessageAutoWakeCreatesFollowUpRevision|TestTextureWorkerMessageAutoWakeBatchesRapidMessages|TestTextureWorkerMessageDebounceUsesFakeClock)$' -count=1 -v`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^TestTextureSeededStochasticWorkflowContracts$' -count=1 -v`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^(TestTextureSeededStochasticWorkflowContracts|TestTextureWorkerMessageAutoWakeCreatesFollowUpRevision|TestTextureWorkerMessageAutoWakeBatchesRapidMessages|TestTextureWorkerMessageDebounceUsesFakeClock|TestConcurrentConductorTextureSpawnsShareRoute|TestRequestSuperExecutionDedupesSameTextureRun|TestPersistentSuperBlockedRunDoesNotStarveFreshInboxDelivery|TestPersistentSuperProcessesConcurrentInboxDeliveriesInFollowupRun)$' -count=1 -v`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^TestResearcherReadContentItemReturnsPrivateSourceArtifact$' -count=1 -v`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^(TestSubmitWorkerUpdateUsesTargetChannelOverExplicitChannel|TestSubmitWorkerUpdateUsesParentAgentOverExplicitAgent)$' -count=1 -v`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^(TestSubmitWorkerUpdateUsesTextureRequesterMetadataWhenAgentMissing|TestSubmitWorkerUpdateUsesTargetChannelOverExplicitChannel|TestSubmitWorkerUpdateUsesParentAgentOverExplicitAgent)$' -count=1 -v`
- `nix develop -c go test ./internal/actor -run 'Test(MarkProcessedFailureRetriesWithoutAdvancingMemory|SnapshotFailureKeepsActorResident)$' -count=1 -v` failed before the actor durability fix: snapshot memory became `"xx"` after one injected `MarkProcessed` failure, and the actor deregistered after injected `SaveSnapshot` failure.
- `nix develop -c go test ./internal/actor -run 'Test(MarkProcessedFailureRetriesWithoutAdvancingMemory|SnapshotFailureKeepsActorResident|NoLostWakeUnderConcurrentSendsAndPassivations|NonBlockingBackpressureReturnsErrInboxFull|BlockingBackpressureWaitsForSpace|BlockingBackpressureTimeoutReturnsErrInboxFull)$' -count=1`
- `nix develop -c go test -race ./internal/actor -run 'Test(MarkProcessedFailureRetriesWithoutAdvancingMemory|SnapshotFailureKeepsActorResident|NoLostWakeUnderConcurrentSendsAndPassivations)$' -count=1`
- `nix develop -c go test ./cmd/sandbox -run 'TestActorMailboxConfigFromEnv|TestTracePersistenceStoreRedactsPayloadBeforeSQLAppend|TestBuildRuntimeConfigPreservesHostServiceURLs' -count=1`
- `nix develop -c go test ./internal/actor ./internal/actorruntime ./cmd/sandbox -count=1`
- `nix develop -c go test -race ./internal/actor -count=1`
- `nix develop -c go test ./internal/base/api ./internal/base/journal -count=1`
- `nix develop -c go test ./internal/base/... -count=1`
- `nix develop -c go test -race ./internal/base/... -count=1`
- `nix develop -c go test ./internal/base/journal -run 'Test(MemAppendRejectsParentOnFirstItemEvent|SQLiteAppendRejectsParentOnFirstItemEvent|SQLiteAppendWorksAcrossIndependentHandles|SQLiteAppendAndReadBack|SQLiteVerifyChain)$' -count=1 -v`
- `nix develop -c go test ./internal/base/... -count=1`
- `nix develop -c go test -race ./internal/base/... -count=1`
- `nix develop -c go test ./internal/desktop -run '^TestSyncEngineConflictPauses$' -count=1` failed before the fix with `synced cursor advanced through unresolved conflict: got 1 want 0`.
- `nix develop -c go test ./internal/desktop -run '^TestSyncEngineConflictPauses$' -count=1`
- `nix develop -c go test ./internal/desktop ./internal/base/... -count=1`
- `nix develop -c go test -race ./internal/desktop -run 'TestSyncEngine(ConflictPauses|ResolveConflict|DownloadCycle|UploadCycle)$|TestSafeCursorBeforeUnresolvedStopsAtFirstBlockedEvent' -count=1`
- `nix develop -c go test ./internal/base/api -run '^TestGetBlob' -count=1` failed before the fix with `status: got 404 want 200`.
- `nix develop -c go test ./internal/desktop -run '^TestSyncEngineDownloadCycle$' -count=1` failed before the fix with `downloaded file content: got "" want "remote file bytes"`.
- `nix develop -c go test ./internal/base/api -run '^TestPutBlobRejectsOversizedBody$' -count=1` failed before the fix with `status: got 200 want 413` and showed a stored 64 MiB truncated blob.
- `nix develop -c go test ./internal/base/api -run '^TestGetBlob' -count=1`
- `nix develop -c go test ./internal/desktop -run '^TestSyncEngineDownloadCycle$' -count=1`
- `nix develop -c go test ./internal/base/api -run '^TestPutBlob(RejectsOversizedBody|Success|WrongScope|NoAuth)$|^TestGetBlob' -count=1`
- `nix develop -c go test ./internal/base/api ./internal/base/blob ./internal/base/journal ./internal/desktop -count=1`
- `nix develop -c go test ./internal/base/api -run 'TestDeltaAcceptsTrustedProxyIdentity|TestDeltaRejectsTrustedProxyIdentityWithoutScope' -count=1`
- `nix develop -c go test ./cmd/sandbox -run '^TestRegisterBaseAPIRoutesServesTrustedProxyRequest$' -count=1`
- `nix develop -c go test ./internal/base/api -count=1`
- `nix develop -c go test ./cmd/sandbox -count=1`
- `nix develop -c go test ./internal/base/... ./internal/desktop -count=1`
- `nix develop -c go test ./internal/base/... -count=1`
- `nix develop -c go build ./cmd/sandbox`
- `nix develop -c go test -race ./internal/desktop -run 'TestSyncEngine(DownloadCycle|ConflictPauses|ResolveConflict|UploadCycle)$|TestSafeCursorBeforeUnresolvedStopsAtFirstBlockedEvent' -count=1`
- `nix develop -c go test ./internal/base/planner -run 'Test(AddAddSameContentDifferentVersionIDsConverges|BothChangeSameContentDifferentVersionIDsConverges)$' -count=1` failed before the fix with one conflict in each case.
- `nix develop -c go test ./internal/base/planner -run 'Test(AddAddSameContentDifferentVersionIDsConverges|BothChangeSameContentDifferentVersionIDsConverges|AddAddDifferentContentConflict|BothChangeConflict)$' -count=1`
- `nix develop -c go test ./internal/base/journal -run 'Test(Mem|SQLite)CursorTrackingIsOwnerScoped|TestSQLiteCursorMigrationPreservesLegacyDeviceCursor|Test(Mem|SQLite)CursorTracking|Test(Mem|SQLite)AppendKeepsParentEventIDOwnerScoped|TestSQLiteAppendWorksAcrossIndependentHandles' -count=1`
- `nix develop -c go test ./internal/base/... -count=1`
- `nix develop -c go test -race ./internal/base/... -count=1`
- `nix develop -c go test ./internal/base/testkit -run '^TestScenario1LocalAddRemoteAddSamePath$' -count=1`
- `nix develop -c go test ./internal/desktop -run '^TestHasConflictRecordMatchesCollisionParticipants$|^TestConflictManagerResolveMatchesCollisionParticipants$|^TestStatusTrackerUpdateFromPlanCarriesCollisionParticipants$' -count=1`
- `nix develop -c go test ./internal/desktop/fileprovider -run '^TestConflictsExposeCollisionParticipants$' -count=1`
- `swiftc -typecheck macos/ChoirFileProvider/ChoirFileProviderBridge.swift macos/ChoirFileProvider/UnixSocketTransport.swift macos/ChoirFileProvider/ChoirFileProviderExtension.swift -framework FileProvider -framework UniformTypeIdentifiers`
- `nix develop -c go test ./internal/desktop/fileprovider -count=1`
- `plutil -lint macos/ChoirFileProvider/Info.plist macos/ChoirFileProvider/ChoirFileProvider.entitlements`
- `nix develop -c go test -run 'TestFileProviderSocketPath' -count=1` from `/Users/wiz/go-choir/cmd/desktop`
- `nix develop -c go test -count=1` from `/Users/wiz/go-choir/cmd/desktop`
- `xcodebuild -project macos/ChoirFileProvider.xcodeproj -scheme ChoirFileProvider -configuration Debug CODE_SIGNING_ALLOWED=NO build` was attempted and failed before project compilation because local Xcode could not load `com.apple.dt.IDESimulatorFoundation`; source-level Swift proof comes from `swiftc -typecheck`.
- `nix develop -c go test ./internal/base/planner ./internal/base/testkit ./internal/desktop -count=1`
- `nix develop -c go test ./internal/base/... ./internal/desktop ./internal/desktop/fileprovider -count=1`
- `nix develop -c go test -race ./internal/desktop ./internal/desktop/fileprovider -run '^TestHasConflictRecordMatchesCollisionParticipants$|^TestConflictManagerResolveMatchesCollisionParticipants$|^TestStatusTrackerUpdateFromPlanCarriesCollisionParticipants$|^TestConflictsExposeCollisionParticipants$|TestSafeCursorBeforeUnresolvedStopsAtFirstBlockedEvent|TestSyncEngineConflictPausesExecution|TestSyncEngineResolveConflictKeepLocal$' -count=1`
- `nix develop -c go test ./internal/proxy -run 'TestBearerTokenAuth(ProtectedAPI|RejectsMissingScope_whenProtectedAPIRouteRequiresRuntimeWrite|AcceptsBaseReadScope_whenProtectedAPIRouteIsBaseRead|AcceptsValidAPIKey|ScopePropagation|StripsClientSuppliedScopes)$' -count=1`
- `nix develop -c go test ./internal/proxy -run 'TestBearerTokenAuthRejectsMissingScope_whenComputeRecoveryRequiresRuntimeWrite$' -count=1`
- `nix develop -c go test ./internal/proxy -run 'TestBearerTokenAuth' -count=1`
- `nix develop -c go test ./internal/proxy -count=1`
- `nix eval .#nixosConfigurations.go-choir-b.config.systemd.services.go-choir-proxy.serviceConfig.Environment --json` returned `PROXY_AUTH_DB_PATH=/var/lib/go-choir/auth/auth.db`.
- `nix eval .#nixosConfigurations.go-choir-b.config.systemd.services.go-choir-proxy.serviceConfig.ReadWritePaths --json` returned both `/var/lib/go-choir/auth-signing` and `/var/lib/go-choir/auth`.
- `nix eval '.#nixosConfigurations.go-choir-b.config.services.caddy.virtualHosts."choir.news".extraConfig' --raw` returned exact `/health -> 127.0.0.1:8082` and `/health/* -> 127.0.0.1:8084`.
- `nix eval '.#nixosConfigurations.go-choir-a.config.services.caddy.virtualHosts."choir-ip.com".extraConfig' --raw` returned exact `/health -> 127.0.0.1:8082` and `/health/* -> 127.0.0.1:8084`.
- Chrome inspection of `https://choir.news/` on 2026-06-29 found the Universal Wire window showing `0 articles` and `Universal Wire load failed: 502`, confirming this was not a healthy empty-state response.
- `nix develop -c go test ./internal/proxy -run 'TestResolveSandboxURL(UsesResolveForUniversalWirePlatformComputer|RetriesTransientVMctlFailure)$|TestProtectedAPIResolveTarget_UniversalWireStoriesUsePlatformComputer' -count=1`
- `nix develop -c go test ./internal/vmctl -run 'TestHandleResolveEnsuresUniversalWirePlatformComputer|TestEnsureUniversalWirePlatformComputerBootsStableVM' -count=1`
- `nix develop -c go test ./internal/proxy ./internal/vmctl -count=1`
- `python3` YAML parse of `.github/workflows/ci.yml` returned `yaml-ok`; `actionlint` is not installed in this environment.
- `bash -n` on the extracted `Build SBOMs for all packages` run block passed.
- A fake `nix`/`sbomnix` harness proved optional `obscura` package-build failure exits 0, records `{"package":"obscura","required":false,"status":"skipped","reason":"package_build_failed"}`, and still emits required package SBOMs.
- The same harness proved required `auth` package-build failure exits 1 and records `{"package":"auth","required":true,"status":"skipped","reason":"package_build_failed"}`.
- `nix develop -c go test ./internal/gateway -run 'TestCircuitBreakingProvider|TestWrapMultiProvider|TestBreakerRegistry|TestInferencePath_CircuitBreakerOpen' -count=1`
- `nix develop -c go test ./internal/gateway -count=1`
- `rg -n "OpenTimeout: 3600" internal docs/overnight-mission-suite-review-2026-06-28.md docs/overnight-mission-suite-fix-report-2026-06-28.md` now finds only the historical review-backlog row documenting C69.
- `nix develop -c go test -race ./internal/vmctl -run '^TestOwnershipRegistry_LiveSandboxURLSnapshotsDuringRefresh$' -count=1` failed before the fix with a race between `LiveSandboxURL` reading `SandboxURL` and `RefreshVMForDesktop` writing it.
- `nix develop -c go test -race ./internal/vmctl -run '^TestOwnershipRegistry_LiveSandboxURLSnapshotsDuringRefresh$' -count=1`
- `nix develop -c go test ./internal/vmctl -run 'TestOwnershipRegistry_(ResolveOrAssignReturnsSnapshot|RequestWorkerReturnsSnapshot|LiveSandboxURLSnapshotsDuringRefresh|ResolveReturnSnapshotDuringRefresh)$' -count=1`
- `nix develop -c go test -race ./internal/vmctl -run 'TestOwnershipRegistry_(ResolveOrAssignReturnsSnapshot|RequestWorkerReturnsSnapshot|LiveSandboxURLSnapshotsDuringRefresh|ResolveReturnSnapshotDuringRefresh)$' -count=1`
- `nix develop -c go test ./internal/vmctl -count=1`
- `nix develop -c go test -race ./internal/vmctl -count=1` failed before the final first-boot snapshot adjustment with a race between cloning the assigned ownership after unlock and a concurrent resolver updating `LastActiveAt`.
- `nix develop -c go test -race ./internal/vmctl -count=1`
- `git diff --check`
- `nix develop -c scripts/doccheck --report /tmp/overnight-fix-doccheck-report.md --json /tmp/overnight-fix-doccheck.json` completed report-only with `330 docs, 141 warnings`.

LSP diagnostics were attempted on changed Go files, but the LSP tool transport was closed in this session. The member worktrees reported clean LSP diagnostics for their slices; main-worktree proof here relies on compiler/test coverage instead.

## Not Verified Through Staging

These fixes are local only. No commit, push, CI run, staging deploy, staging identity check, or deployed acceptance proof has been performed in this pass.

That means red/orange acceptance is not complete under the repo landing-loop contract. Before claiming product acceptance, push the integrated branch, monitor CI/deploy, confirm `https://choir.news/health` reports the new commit, and run deployed acceptance against the relevant auth, trace, Base, and health surfaces.

## Residual Issues

The overnight review backlog is not closed. Known residual items include:

- Auth/legal: public `/privacy` and `/terms` exposure, frontend transient protected-route renewal classification, raw recovery token removal, session revocation hardening, and global auth route limiting are fixed locally. Legal review proof, contact-address proof, erasure implementation proof, and deployed auth/session acceptance remain unresolved.
- Trace/runtime: redaction-before-persistence, fail-closed redaction-error payload handling, trace query owner scoping, direct runtime event append trace projection, the deleted `/api/trace/trajectories` compatibility surface, C64's stale comprehensive runtime build blockers, several stale comprehensive fixtures, worker/coagent terminal notifications, persistent Super inbox isolation, Texture auto-wake/quiescence, and the remaining legacy `work_id` channel fallback are fixed locally. Remaining trace/runtime concerns include the regex-only default redaction policy, production/deployed proof that trace persistence is enabled and writing redacted rows on staging, failing full `-tags comprehensive ./internal/runtime` package proof with visible `TestTextureAgentRevisionRealLLM*` failures, and pre-existing oversized trace/runtime test files (`internal/trace/store.go`, `internal/trace/query_test.go`, `internal/runtime/trace_wiring_test.go`, `internal/runtime/texture_test.go`) that should be split in a dedicated refactor rather than inside this red privacy fix stack.
- Base: desktop unresolved-conflict cursor advancement, blob-backed remote file download, oversized blob upload rejection, content-addressed planner equality, path-collision participant identity, Base journal first-parent/SQLite append transaction gaps, owner-scoped device cursors, and local Base API sandbox mounting are fixed locally. Remaining Base concerns include deployed Base API product-route acceptance proof, broader desktop conflict UI acceptance, and the pre-existing oversized `internal/desktop/sync.go` / File Provider bridge files that should be split in a dedicated refactor rather than inside this behavior-fix stack.
- macOS File Provider: Swift source type-check blockers, Swift/Go JSON key mismatch, HTTP error success reporting, URLProtocol socket-path loss, stale move metadata lookup, and app-group socket-path alignment are fixed locally. Remaining File Provider concerns include host app domain registration, appex embedding/signing, Xcode project build proof, and signed Finder/manual acceptance proof.
- API keys: protected sandbox and direct proxy-owned routes now share route/method scope enforcement locally, and Node B proxy DB wiring exists locally. Remaining API-key acceptance gaps are deployed proof, more granular future scopes for mail/notifications/platform operations if desired, and route-level coverage beyond the representative generic/proxy-owned regressions added here.
- Gateway health: local Node A/B Caddy config now routes `/health/*` to gateway `8084`, but public staging paths `/health/ready` and `/health/{service}` still require push/deploy proof; previous probes returned SPA HTML rather than gateway JSON.
- Universal Wire: local proxy/vmctl routing now provisions and resolves the platform computer for `/api/universal-wire/stories`, but staging still requires push/deploy proof. The observed 2026-06-29 Chrome state showed a `502` load failure, not an expected empty Wire edition.
- Gateway circuit-breaker timeout literals are fixed locally in gateway tests. Production timeout defaults were already using explicit `time.Duration` values; this repair closes the remaining test false-confidence issue only.
- vmctl: `LiveSandboxURL`, `ResolveOrAssignDesktopContext`, and `RequestWorker` now return/read snapshots instead of exposing live registry ownership pointers. Remaining vmctl ownership work is a broader DTO/API split for other lifecycle methods that may still return ownership structs, plus the pre-existing oversized `internal/vmctl/ownership.go` and `internal/vmctl/vmctl_test.go` files that should be split in a dedicated refactor.
- SBOM diagnostics now include bounded head-and-tail stderr for package build failures, a machine-readable manifest, and a required-package completeness gate. Remaining SBOM proof gap is CI/main-push execution on Linux; optional `obscura` still has warning semantics by design.
- Actor: `MarkProcessed` and `SaveSnapshot` failure handling plus sandbox product-path backpressure are fixed locally. Remaining actor work includes deeper crash-atomicity design for handler side effects outside actor memory, deployed/runtime acceptance of backpressure under real workload, any future bounded-inbox tuning after staging evidence, and pre-existing oversized actor/sandbox files (`internal/actor/actor.go`, `internal/actor/actor_test.go`, `cmd/sandbox/main.go`) that should be split in a dedicated refactor.
- Broader vmctl DTO cleanup, File Provider registration/packaging issues, and mission-ledger accounting/doccheck gaps remain outside this local fix set.

## Worktree State Notes

Intentional source/doc edits are in:

- `cmd/sandbox/main.go`
- `cmd/sandbox/main_test.go`
- `cmd/auth/main.go`
- `cmd/auth/main_test.go`
- `cmd/desktop/syncservice.go`
- `cmd/desktop/syncservice_test.go`
- `cmd/desktop/frontend/dist/.gitkeep`
- `DESIGN.md`
- `.github/workflows/ci.yml`
- `docs/legal/privacy-policy.md`
- `docs/legal/terms-of-service.md`
- `frontend/public/legal/privacy-policy.md`
- `frontend/public/legal/terms-of-service.md`
- `frontend/src/App.svelte`
- `frontend/src/lib/AuthEntry.svelte`
- `frontend/src/lib/auth.js`
- `frontend/src/lib/LegalDocument.svelte`
- `frontend/tests/legal-routes.spec.js`
- `frontend/tests/auth-renewal-transient.spec.js`
- `internal/auth/recovery.go`
- `internal/auth/recovery_test.go`
- `internal/auth/rate_limit.go`
- `internal/auth/rate_limit_test.go`
- `internal/actor/actor.go`
- `internal/actor/actor_test.go`
- `internal/base/api/handlers.go`
- `internal/base/api/handlers_test.go`
- `internal/base/api/validation.go`
- `internal/base/journal/journal.go`
- `internal/base/journal/journal_test.go`
- `internal/base/journal/sqlite.go`
- `internal/base/planner/planner.go`
- `internal/base/planner/planner_test.go`
- `internal/desktop/client.go`
- `internal/base/testkit/scenarios_test.go`
- `internal/desktop/conflicts.go`
- `internal/desktop/conflicts_test.go`
- `internal/desktop/fileprovider/bridge.go`
- `internal/desktop/fileprovider/bridge_test.go`
- `internal/desktop/fileprovider/types.go`
- `internal/desktop/sync.go`
- `internal/desktop/sync_test.go`
- `internal/desktop/syncstatus.go`
- `internal/desktop/syncstatus_test.go`
- `internal/gateway/circuit_breaker_test.go`
- `internal/gateway/handlers.go`
- `internal/gateway/service_health_test.go`
- `macos/ChoirFileProvider/Info.plist`
- `macos/ChoirFileProvider/ChoirFileProvider.entitlements`
- `macos/ChoirFileProvider/ChoirFileProviderBridge.swift`
- `macos/ChoirFileProvider/UnixSocketTransport.swift`
- `macos/ChoirFileProvider/ChoirFileProviderExtension.swift`
- `macos/README.md`
- `internal/proxy/app_change_packages.go`
- `internal/proxy/compute_status.go`
- `internal/proxy/email.go`
- `internal/proxy/handlers.go`
- `internal/proxy/handlers_test.go`
- `internal/proxy/notifications.go`
- `internal/proxy/platform_public.go`
- `internal/proxy/platform_publish.go`
- `internal/runtime/trace_wiring_test.go`
- `internal/runtime/api.go`
- `internal/runtime/api_trace.go`
- `internal/runtime/api_trace_trajectory.go`
- `internal/runtime/api_trace_agents.go`
- `internal/runtime/api_trace_moments.go`
- `internal/runtime/api_trace_test.go`
- `internal/runtime/channel_metadata_test.go`
- `internal/runtime/prompts_test.go`
- `internal/runtime/runtime.go`
- `internal/runtime/texture_test.go`
- `internal/trace/store.go`
- `internal/trace/redact.go`
- `internal/trace/query.go`
- `internal/trace/query_test.go`
- `internal/vmctl/ownership.go`
- `internal/vmctl/vmctl_test.go`
- `nix/node-a.nix`
- `nix/node-b.nix`
- `docs/overnight-mission-suite-fix-report-2026-06-28.md`

Untracked coordination/report state remains under `.omo/`, and the source review backlog file remains untracked at `docs/overnight-mission-suite-review-2026-06-28.md`.
