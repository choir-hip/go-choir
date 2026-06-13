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
