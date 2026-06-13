# Mission M2 — Messaging Cutover Ledger

## 2026-06-12 — Ready After M1, Architecture-First Route

Claim/scope: M2 is now the next active spine mission. Scope is mission state,
not implementation.

Move: rewrite the M2 Parallax State from "blocked on M1 settlement" to
`open_handoff`, add a concrete V=9 variant, and point the next move at the
same-transaction durable update log decision before deletion batching.

Expected ΔV: 0. No M2 blockers are removed by a docs correction, but the
observer state should stop future agents from chasing Universal Wire or
review UI before the messaging cutover.

Actual ΔV: 0. M2 remains unstarted, ready, and first in the active
architecture spine.

Receipt:
- Updated `docs/mission-messaging-cutover-v0.md`.
- M1 is treated as settled; line numbers and grep receipts must still be
  re-verified at M2 start.
- The first M2 discriminator is Q1: whether durable update append and ledger
  effects land in the same transactional domain.

Open edge: start M2 with Parallax, decide Q1, then batch only the deletion
work supported by that decision.

## 2026-06-12 — Q1 Transaction Domain Decided

Claim/scope: M2 can batch deletion work only after deciding the transaction
domain for durable update append plus ledger effects. Scope is repository
inventory and mission state, not runtime behavior.

Move: probe current `submit_coagent_update`, runtime store, actor core,
work-item, and run-acceptance storage.

Expected ΔV: -1 by deciding Q1.

Actual ΔV: -1. Q1 is supported: the M2 durable update append belongs in the
runtime store transaction. The separate `internal/actor` SQLite log cannot be
the M2 source of truth unless backed by the same runtime DB, because
`assignment` work items and `verification` acceptance evidence must commit in
the same domain as the update append.

Receipt:
- `internal/runtime/tools_worker_update.go` builds `submit_coagent_update`
  records and calls `Store.DispatchWorkerUpdate`.
- `internal/store/store.go` `DispatchWorkerUpdate` atomically writes
  `worker_updates`, `channel_messages`, and `inbox_deliveries`.
- `internal/store/trajectory.go` and `internal/store/run_acceptance.go` keep
  work items and acceptance records in the same runtime store.
- `internal/actor/log_sqlite.go` is a separate protocol log implementation
  and stays unsuitable as a separate M2 ledger-effect transaction domain.

Open edge: implement the supported batch: `update_coagent` promotion, old
tool deletion/shims, inbox/notify deletion, (trajectory, slot) slot key,
prompt/test updates, restart exactly-once and silent-stall falsifiers.

## 2026-06-12 — Local Cutover Batch Implemented

Claim/scope: Q1-supported local deletion work is implemented. Scope is the
runtime/store/prompts/test cutover, not platform settlement.

Move: promote `update_coagent` as the sole structured message/wake primitive,
delete the old tool registrations and runtime delivery hooks, make channel
casts audit-only, replace pending inbox delivery with pending
`worker_updates`, wake persistent super/VText/idle coagents from the update
backlog, re-key co-super slots by `(trajectory_id, slot)`, and add the restart
and silent-stall falsifiers.

Expected ΔV: -7 by closing all local code cutover blockers after Q1.

Actual ΔV: -7. Current V=1: local implementation proof is green, but platform
settlement still requires commit, push, CI, deploy identity, and deployed
staging acceptance. Q1 remains a transaction-domain decision; this batch keeps
`assignment` and `verification` as typed `worker_updates` but does not yet add
new kind-specific work-item or run-acceptance ledger writers.

Receipt:
- `update_coagent` is registered; `submit_coagent_update`,
  `cast_agent`, `cast_agent_update`, and `wait_agent` are absent from active
  runtime/tool/prompt code.
- `DispatchWorkerUpdate` writes the addressed audit message and
  `worker_updates` row in the runtime store transaction; pending delivery is
  tracked by `worker_updates.delivered_at`.
- `notifyParent` and per-turn inbox injection are removed; completed
  update-woken runs mark their update IDs delivered only after completion.
- `co_super_slots` keys live co-super slot claims by
  `(owner_id, trajectory_id, slot)` and releases a matching claim if
  post-claim run persistence fails.
- `TrajectoryObligations` includes pending undelivered updates so a trajectory
  with queued `update_coagent` work cannot silently appear settled.
- Grep receipt:
  `rg -n '\b(cast_agent|cast_agent_update|wait_agent|submit_coagent_update|notifyParent|injectPendingInboxTurns|ListPendingInboxDeliveries|MarkInboxDeliveriesDelivered|EnqueueInboxDelivery)\b' internal specs cmd --glob '*.go' --glob '*.md' --glob '*.tla'`
  returned no matches.

Local proof:
- `nix develop -c scripts/go-test-runtime-shards` passed.
- `nix develop -c go test ./internal/store` passed.
- `nix develop -c go test ./internal/runtime -run '^$'` passed.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^$'`
  passed.
- `nix develop -c go test ./internal/runtime -run 'Test(UpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TrajectoryObligationsReportPendingUpdateCoagent|VSuperCoSuperSlotReusedByTrajectorySlot|CoagentCastCannotAddressEmailAppagentDirectly)$' -v`
  passed.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run '^TestChannelCastDoesNotCreateWakeDelivery$' -v`
  passed.
- `nix develop -c go test ./internal/store -run 'TestReleaseCoSuperSlotClaimOnlyClearsMatchingRun' -v`
  passed.
- `git diff --check` passed.

Open edge: platform acceptance is still unproven. Do not claim settlement
until the landing loop records pushed SHA, CI result, staging deployed
identity, and deployed acceptance proof.

## 2026-06-13 — Landing Loop Completed And M2 Settled

Claim/scope: the requested M2 deletion cutover is settled. Scope is the
agent-to-agent message/wake primitive cutover, slot re-key, prompts/tests, and
staging smoke proof. It does not claim promotion-level or continuation-level
acceptance.

Move: record the completed landing loop after the behavior-changing commit was
fast-forward pushed to `origin/main`, CI passed, Node B deployed, and staging
reported the deployed code identity.

Expected ΔV: -1 by closing the remaining platform landing proof blocker.

Actual ΔV: -1. Current V=0 for M2 deletion cutover.

Receipt:
- Code commit pushed and deployed:
  `8052d242afc80320b7cd1b34a2f7a4bb306f1f13`
  (`runtime: cut over coagent updates`).
- Pre-cutover rollback ref:
  `d188e88bfc33582bb9479d5d9c0511c599f077de`.
- GitHub Actions CI run `27453151153` succeeded:
  runtime shards, non-runtime Go tests, integration-tagged smoke, TLA+ model
  check, Go vet/build, and Node B staging deploy all passed.
- FlakeHub publish run `27453151152` succeeded.
- `curl -fsS https://choir.news/health | jq '{status, service, upstream, build, upstream_build}'`
  reported `status=ok`, proxy `deployed_commit` =
  `8052d242afc80320b7cd1b34a2f7a4bb306f1f13`, and sandbox
  `deployed_commit` =
  `8052d242afc80320b7cd1b34a2f7a4bb306f1f13`.
- `curl -fsS https://choir.news/` returned HTTP 200 and served the Choir
  frontend asset `index-BAGuPoFu.js`.

Settlement note: Q1 decided that future kind-specific ledger effects must live
in the same runtime store transaction as the durable update append. The M2
deletion batch preserved `assignment` and `verification` as typed
`update_coagent` kinds, but did not add new work-item or run-acceptance writers;
that is a successor edge, not part of this deletion-cutover settlement.

## 2026-06-13 — Post-Settlement Review Reopened M2

Claim/scope: the previous M2 settlement claim was too strong. Scope is
documentation of review evidence before any code repair, per the
problem-documentation-first invariant.

Move: review the landed M2 slice against the mission conjecture and run the
focused falsifiers plus relevant comprehensive tests.

Expected ΔV: +N if the review falsifies settlement; otherwise 0.

Actual ΔV: +3. M2 is reopened with V=3:
1. terminal run completion and delivered `worker_updates` marking are two
   separate writes;
2. the comprehensive persistent-super blocking provider still keys on the old
   "pending inbox deliveries" prompt and the follow-up test fails under the
   new prompt;
3. `/internal/runtime/channel-casts` is a live second wake writer that directly
   calls `DispatchWorkerUpdate` instead of the `update_coagent` authority path.

Receipts:
- `nix develop -c go test ./internal/runtime -run 'Test(UpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TrajectoryObligationsReportPendingUpdateCoagent|VSuperCoSuperSlotReusedByTrajectorySlot)' -count=1`
  failed with `TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce`
  observing the target run terminal while the update still had empty
  `DeliveredToRunID` and nil `DeliveredAt`.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'Test(PersistentSuperProcessesConcurrentInboxDeliveriesInFollowupRun|PersistentSuperBlockedRunDoesNotStarveFreshInboxDelivery|InstallDefaultAgentToolsProfiles)' -count=1 -v`
  failed because `TestPersistentSuperProcessesConcurrentInboxDeliveriesInFollowupRun`
  timed out waiting for a prompt containing `Process the liquid package lane`.
  Root cause: `blockingExecuteProvider` still recognizes only
  `Process the pending inbox deliveries addressed to you as the user's
  persistent super actor.`
- `rg -n "HandleInternalChannelCast|internal/channel/cast|channel-cast|ChannelCast" internal cmd frontend scripts specs --glob '*.*'`
  showed `HandleInternalChannelCast` is registered at `api.go` and called by
  vmctl. Its addressed path constructs a `WorkerUpdateRecord` and calls
  `DispatchWorkerUpdate` directly.

Open edge: repair M2 in a follow-up code commit. The repair must preserve Q1:
all durable update append, wake backlog, and any delivered-state transition
needed for exactly-once semantics must live in the runtime store's transactional
domain. Do not weaken the mission by accepting a second wake writer.

## 2026-06-13 — Local Repair Of Reopened M2 Blockers

Claim/scope: repair only the three reopened M2 blockers. No Universal Wire,
review UI, staging, or product-behavior detour.

Move:
- Added `Store.UpdateRunAndMarkWorkerUpdatesDelivered`, a single runtime-store
  transaction for terminal run persistence plus delivered marking of the
  run's waking `worker_update_ids`.
- Routed terminal update-woken completions, failures, cancellations, and
  restart recovery through that store primitive.
- Updated the comprehensive persistent-super provider/test to key on the
  `update_coagent` prompt and prove queued updates drain after a follow-up run.
- Rejected addressed `/internal/runtime/channel-casts` requests before they can
  write `worker_updates`; the route remains audit-only for unaddressed casts.

Actual ΔV: -3. Local M2 repair blockers are at V=0. Settlement remains open
until a platform landing loop is explicitly run and recorded.

Receipts:
- `nix develop -c go test ./internal/store ./internal/runtime -run 'TestUpdateRunAndMarkWorkerUpdatesDelivered|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TestTrajectoryObligationsReportPendingUpdateCoagent|TestVSuperCoSuperSlotReusedByTrajectorySlot' -count=1`
  passed.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'Test(PersistentSuperInboxBashRequiresCoagentUpdate|ChannelCastDoesNotCreateWakeDelivery|RedirectWorkerDelegationCannotBypassUpdateCoagent|PersistentSuperProcessesConcurrentInboxDeliveriesInFollowupRun|PersistentSuperBlockedRunDoesNotStarveFreshInboxDelivery|InstallDefaultAgentToolsProfiles)' -count=1 -v`
  passed.
- `rg -n '\b(cast_agent|cast_agent_update|wait_agent|submit_coagent_update|notifyParent|injectPendingInboxTurns|ListPendingInboxDeliveries|MarkInboxDeliveriesDelivered|EnqueueInboxDelivery)\b' internal specs cmd --glob '*.go' --glob '*.md' --glob '*.tla'`
  returned no active-code matches.
- `rg -n 'HandleInternalChannelCast|/internal/runtime/channel-casts|DispatchWorkerUpdate|wakeUpdatedCoagent' internal/runtime internal/store --glob '*.go'`
  showed no `DispatchWorkerUpdate` call inside `HandleInternalChannelCast`.
  The remaining dispatch callers are the intended update/backlog paths:
  `tools_worker_update.go`, `tools_vtext.go`, `delegate_worker_update_fallback.go`,
  `tools_vmctl.go`, `vtext_proposals.go`, and tests.

Residual risk: local proof only. The mission's platform-behavior settlement
standard still requires commit, push, CI, staging deploy identity, and deployed
acceptance proof before final settlement can be re-claimed.

## 2026-06-13 — Review Found Dead Redirect Tool Surface

Claim/scope: the local repair closed the original V=3 blockers but left one
tool-surface inconsistency. Scope is documentation before code repair.

Move: review `bb295012` and run the claimed focused/local tests.

Expected ΔV: +1 if a real M2 blocker remains, otherwise 0.

Actual ΔV: +1. Current local V=1. `redirect_worker_delegation` remains exposed
to super, but its only transport posts an addressed
`/internal/runtime/channel-casts` request. The repaired handler intentionally
rejects addressed channel casts, and the comprehensive test now expects
`redirect_worker_delegation` to fail. A dead advertised control tool is not a
settled M2 surface.

Receipts:
- `internal/runtime/tools.go` still includes `redirect_worker_delegation` in
  the super tool profile.
- `internal/runtime/tools_vmctl.go` implements `redirect_worker_delegation` by
  calling `postInternalWorkerChannelCast` with `ToAgentID` / `ToRunID`.
- `internal/runtime/api.go` rejects any addressed internal channel cast with
  `addressed internal channel casts are disabled; use update_coagent for
  agent-to-agent wake delivery`.
- `internal/runtime/agent_tools_test.go`
  `TestRedirectWorkerDelegationCannotBypassUpdateCoagent` now asserts that the
  tool fails.
- Focused tests passed:
  `nix develop -c go test ./internal/store ./internal/runtime -run 'TestUpdateRunAndMarkWorkerUpdatesDelivered|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TestTrajectoryObligationsReportPendingUpdateCoagent|TestVSuperCoSuperSlotReusedByTrajectorySlot' -count=1`
- Relevant comprehensive tests passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'Test(PersistentSuperInboxBashRequiresCoagentUpdate|ChannelCastDoesNotCreateWakeDelivery|RedirectWorkerDelegationCannotBypassUpdateCoagent|PersistentSuperProcessesConcurrentInboxDeliveriesInFollowupRun|PersistentSuperBlockedRunDoesNotStarveFreshInboxDelivery|InstallDefaultAgentToolsProfiles)' -count=1 -v`
- Retired primitive grep returned no active-code matches:
  `rg -n '\b(cast_agent|cast_agent_update|wait_agent|submit_coagent_update|notifyParent|injectPendingInboxTurns|ListPendingInboxDeliveries|MarkInboxDeliveriesDelivered|EnqueueInboxDelivery)\b' internal specs cmd --glob '*.go' --glob '*.md' --glob '*.tla'`

Open edge: choose one architecture and finish it. Either remove/de-register
`redirect_worker_delegation` and update prompts/docs/tests, or re-route it
through the same `update_coagent` authority/delivery semantics without
reviving addressed channel casts or adding a second wake writer.

## 2026-06-13 — Dead Redirect Tool Surface Removed

Claim/scope: close only the dead redirect tool surface. No Universal Wire,
review UI, staging, or product-behavior detour.

Move: chose deletion. `redirect_worker_delegation` had no clean M2-compliant
transport because its implementation depended on addressed channel casts, and
that route is intentionally audit-only now. Removed the super tool
registration, deleted the vmctl implementation/client helper, moved the
channel-cast request/response structs to the API file, and changed tests to
assert the tool is not installed.

Actual ΔV: -1. Current local V=0. Local M2 repair remains open_handoff until
the platform landing loop is explicitly requested and recorded.

Receipts:
- `nix develop -c go test ./internal/store ./internal/runtime -run 'TestUpdateRunAndMarkWorkerUpdatesDelivered|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TestTrajectoryObligationsReportPendingUpdateCoagent|TestVSuperCoSuperSlotReusedByTrajectorySlot' -count=1`
  passed.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'Test(PersistentSuperInboxBashRequiresCoagentUpdate|ChannelCastDoesNotCreateWakeDelivery|RedirectWorkerDelegationIsNotInstalled|PersistentSuperProcessesConcurrentInboxDeliveriesInFollowupRun|PersistentSuperBlockedRunDoesNotStarveFreshInboxDelivery|InstallDefaultAgentToolsProfiles)' -count=1 -v`
  passed.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'Test.*Prompt|TestWorkerBootstrapPrompt|TestHandlePromptBarOperationalProofInitialRunRequestsPersistentSuper' -count=1`
  passed.
- `rg -n '\b(cast_agent|cast_agent_update|wait_agent|submit_coagent_update|notifyParent|injectPendingInboxTurns|ListPendingInboxDeliveries|MarkInboxDeliveriesDelivered|EnqueueInboxDelivery)\b' internal specs cmd --glob '*.go' --glob '*.md' --glob '*.tla'`
  returned no active-code matches.
- `rg -n 'redirect_worker_delegation' internal/runtime internal/store cmd specs --glob '*.go' --glob '*.md' --glob '*.tla'`
  returned only tests that assert the tool is absent.
- `rg -n 'redirecting|redirect or finish|redirect the vsuper|observe, redirect|redirect, cancel|redirect/cancel|redirecting/cancelling|redirect_worker_delegation|postInternalWorkerChannelCast|worker_redirect_sent' internal/runtime/prompt_defaults internal/runtime/*.go internal/runtime/*_test.go`
  returned only tests that assert `redirect_worker_delegation` is absent.
- `rg -n 'HandleInternalChannelCast|/internal/runtime/channel-casts|DispatchWorkerUpdate|wakeUpdatedCoagent|postInternalWorkerChannelCast|worker_redirect_sent' internal/runtime internal/store --glob '*.go'`
  showed no `postInternalWorkerChannelCast`, no `worker_redirect_sent`, and no
  `DispatchWorkerUpdate` call inside `HandleInternalChannelCast`.

Residual risk: local proof only. Final M2 settlement still requires push, CI,
staging deploy identity, and deployed acceptance proof.

## 2026-06-13 - Platform Landing And Settlement

Claim/scope: settle M2 after the repaired messaging cutover lands on staging.
Scope remains the architecture spine; no Universal Wire product completeness or
review UI claim.

Move: pushed `794d28dd76ff00a2ae27c98a14dbce9e34834695` to `origin/main`,
monitored CI and Node B deploy, verified staging health identity, and ran a
deployed public-origin lifecycle/prompt-bar acceptance proof.

Actual Delta V: 0 inside M2 because local V was already 0; evidence class
increased from local repair to staging-smoke-level settlement. Portfolio Delta
V is -1 because M2 is now done and M3 is next.

Receipts:
- `git push origin HEAD:main` advanced `main` from
  `760c42f0df5e1c0c096ae0bcbdb1b87ce9171c08` to
  `794d28dd76ff00a2ae27c98a14dbce9e34834695`.
- CI run `27455953966` passed:
  `https://github.com/choir-hip/go-choir/actions/runs/27455953966`.
- Node B deploy job `81160546255` passed.
- `curl -fsS https://choir.news/health | jq .` reported proxy and sandbox
  `build.commit` / `deployed_commit`
  `794d28dd76ff00a2ae27c98a14dbce9e34834695`, deployed at
  `2026-06-13T04:03:19Z`.
- Deployed acceptance passed:
  `GO_CHOIR_RUN_DEPLOYED_LIFECYCLE=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news pnpm --dir frontend exec playwright test tests/adaptive-lifecycle-control-deployed.spec.js --project=chromium --reporter=list`.

Settlement: M2 is settled at staging-smoke-level for the cutover scope. M3
lifecycle cutover now carries the next actor-spine descent.
