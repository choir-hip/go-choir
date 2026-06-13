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
