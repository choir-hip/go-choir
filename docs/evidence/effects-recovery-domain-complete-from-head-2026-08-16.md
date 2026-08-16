# Effects recovery domain — complete_from_head — 2026-08-16

**Boundary:** define + additive restore admission. Not execute. Super was not started. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Design:** `docs/choir-unified-event-tape-design-2026-08-16.md`

## Decision

**`complete_from_head`.** `new_epoch` is refused on the retained computer.

`Reduce` already rejects duplicate genesis. Owner product path cannot mint a computer. Rematerialize is forbidden. The paid tape-recovery chain (sequence 26, eligible empty-table checkpoints) stays the restore substrate.

Full projected computer restore (desktop/OG payloads) is only valid at or after an explicit completeness head. Restore of a complete-tape checkpoint to a pre-completeness sequence fails closed. Heads 1–26 are not fabricated history.

## Code

- `internal/selfdevprotocol/tape_completeness.go`
- Additive `CheckpointRequest.TapeCompleteness` / `CompleteFromHead` (`omitempty` so existing digests stay stable)
- Tests: `go test ./internal/selfdevprotocol`

## Unchanged

Staging `5557840c`. Epoch 272. `propose_only` generation 1. Checkpoint still 409. Genesis 409. `Armed=false`. Super not started.
