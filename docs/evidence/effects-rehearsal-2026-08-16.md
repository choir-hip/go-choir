# Effects rehearsal — orange in-process, no live send

**Boundary:** execute (route map 9, orange). Not live proof. Not a live send. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Parent HEAD:** `466c0504` (supervision wiring)
**Mutation class:** orange (joined in-process rehearsal of landed paths; restore substrate consumed, not re-owned)

## What landed

One joined rehearsal, `TestEffectsRehearsalReversibleProposeConsensusPromoteRestore` and `TestEffectsRehearsalIrreversibleProposeConsensusOutboxNoLiveSend`:

1. Reversible: `reversible-selfdev-v1` Reduce → `QualifiedConsensusReceipt` → decision binding accepts `qualified-consensus:<digest>` → neither-owner-nor-consensus refused → OwnerRecovery refused → memory route promote writes the slot → tape-recovery restore API consumed (`EventRestoreRequested`, projection stops at the checkpoint head). Restore was not rematerialized as a new product path.
2. Irreversible: reversible policy refuses the email subject; `irreversible-email-v1` authorizes with human seat absent (3 counting accepts); `human-required-v1` refuses when that seat is absent; trusted-outbox `RecordingProvider` dispatch greens with intent recorded; mode `off` refuses; unarmed live provider refuses (`Armed=false`); unknown outcome does not green; crash-window reconcile recovers an accepted send.

## What did not change

- `external-owner:` and `accept_once` remain.
- `awaiting_approval` remains.
- Mode `off` remains the default. Outbox `Armed` remains false.
- No mail was sent. Restore was not rematerialized as a new authority.
- OwnerRecovery remains inadmissible for promotion.
- Deployed staging is still `4ac90583`. This receipt is orange source confirmation on current main, not deployed acceptance and not red/live rehearsal.

## Ceremony

- **Conjecture delta:** The landed reducer, decision binding, route promote, tape-recovery restore API, and trusted outbox compose into both rehearsal paths without arming a live send.
- **Protected surfaces:** decision binding still requires owner or consensus; live provider still requires Armed; restore still consumes choir-tape-recovery-2026-08-13 rather than a second restore authority.
- **Admissible evidence:** `go test ./internal/agentcore -run TestEffectsRehearsal`.
- **Rollback:** revert this commit. Owner gates and effects-OFF are unchanged.
- **Heresy delta:** `introduced` a named orange rehearsal that joins already-landed slices. `preserved` owner gates, Armed=false, and restore ownership.

## Residual

Red/live rehearsal and live proof remain unpaid until a staging deploy of current main. Irreversible send remains unpaid. Completion cutover remains unpaid.

## Next

Route map 10 live proof is gated on a staging deploy. Do not send live mail. Do not rematerialize. Do not delete owner gates. Do not independently green restore.
