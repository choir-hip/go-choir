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
