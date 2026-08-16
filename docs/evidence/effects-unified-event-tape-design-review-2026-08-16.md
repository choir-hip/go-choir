# Unified event tape design review — 2026-08-16

**Boundary:** define. Not execute. Super was not started. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Design:** `docs/choir-unified-event-tape-design-2026-08-16.md`
**Panel:** `.agentic-consensus/unified-event-tape-20260816/` (Claude + Sol + Terra + Gemini + Grok; Codex failed `--autoputer`)

## Owner correction under review

Merge the two event systems. The tape must reconstruct any prior computer state. No backwards compatibility. Forget unused legacy.

## Verdict

**APPROVE_WITH_CONDITIONS** on the one-tape direction. Not implementable as first written.

Terra **ESCALATE** until the owner names the recovery domain. Sol’s strongest package is the same: import at sequence 27 cannot reconstruct heads 1–26. That is not a disagreement with one tape; it is a disagreement with calling incomplete history “arbitrary-head restore.”

## Conditions (now in the design)

Payload resolver before SQL. Atomic projection batches. Split or project `desktop_sessions`. Do not pretend `choir.event` is unused. Desktop+OG co-move. Projector must not wedge after CAS. Texture workspace atomicity. Projector version bound. Notification outbox. Restore of pre-completeness heads fails closed.

## First unpaid slice

Owner picks: operational restore only at/after `complete_from_head`, or new genesis/epoch. Then freeze those contracts with tests. No Super. No checkpoint until residue is event-derived.

## Live state unchanged

Staging `5557840c`. Epoch 272. `propose_only` generation 1. Checkpoint still 409.
