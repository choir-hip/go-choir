# checkpointd still exits 1 every minute after the panic fix: one computer's replay stops at a commitment record with no identity formula (2026-10-15)

Date: 2026-10-15 11:42 UTC, build `7b8944f03` (merge of `fix/dolt-guardrails`
over `a64f28e1f`). Found by the overnight lander during the guardrails proof.
Status: documented, evidence only.

## Observed

- The panic loop is gone (HOME fix works): `go-choir-checkpointd` runs, opens
  the store and publishes checkpoints: `checkpoint published
  computer=computer-4687411d… target=2168 … duration=28s` (11:41:32). Three
  publishes in the 10 min before the check.
- The unit still ends `status=1/FAILURE` each run ("Failed to start Choir
  verified projection checkpoint maintenance") because one computer fails:
  computer `computer-c7f57b9031eb44c81f91a18071da760c`, generation 10,
  reason `cadence`, target sequence 1307, watermark 0:
  `rebuilder: replay through target head f9b57cd9…: computer event appender:
  replay finalize sequence 527: deposit upcast: op 0 (object_recorded): og
  migrate: choir.commitment_record obj:choir.commitment_record:…key-27d40897…
  has migrated identity fields but no verified identity formula`.
- `verify-host-hardening.py` on Node B: `[dolt-probe]` checks all PASS (timer
  active, last run success, no probe-error, both Dolt stop timeouts 20 s);
  `go-choir-checkpointd running` FAILS only because the unit's last run failed.

## Classification (hypothesis)

Not a Dolt or HOME problem. A computer's tape holds an `object_recorded` op
for a `choir.commitment_record` whose migrated identity fields have no
registered verified identity formula, so event-appender replay refuses it and
that computer's checkpoint can never be rebuilt; the worker exits 1 and the
remaining computers wait for the next run (they do progress, as the published
checkpoint shows, but a persistent failing computer keeps the unit red). The
tape is not to be edited. The repair is in the migration/identity-formula
registry for `choir.commitment_record`, or in making one computer's replay
failure terminal for that computer instead of the whole run.

Heresy delta: discovered one (a single computer's replay failure fails the
whole checkpoint unit); introduced none; repaired none.

## Status at 2026-10-15 19:05 UTC

- **The unit is green, but the problem is not fixed.** Since 12:16 every
  run ends `success`.
- The two refusing computers are now in the failed-job backoff
  (`failedJobRetryBackoff`, 15 min doubling to 16 h), and nothing else is
  due. A computer is due at a 2,500-event tail, or at 1,000 events and
  24 h.
- **Two computers refuse, not one:**
  - `computer-c7f57b90…`: sequence 527, target 1307;
  - `computer-84cc92fc…`: sequence 107, target 1166, at 12:16:09.

  Both are QA demo test computers from 2026-10-11
  (`~/.config/choir-qa/test-computers/demo-p2a-2026-10-11.json` and
  `demo-p2c-…`), not user computers.
- **Cause, from code:** `ogIdentityFormulas`
  (`internal/store/vocab_migrate_og.go`) has no entry for
  `choir.commitment_record`. Its canonical id is
  `lifecycleCanonicalID(kind, owner, computer, record_id)`, the scoped
  form. A record whose `record_id` was migrated has no formula that
  verifies the old suffix, so the deposit upcast refuses it.
- **Decision: residual, not fixed now.**
  - The candidate fix is a scoped `record_id` formula. But replay
    rekeying a commitment record must match how the live store keyed
    that same record. Otherwise the checkpoint diverges from live: the
    f95311775 class, "replay matches live".
  - That is a red surface (checkpoint projection), and the evidence
    covers only test computers.
  - The next step is to read the live object id of one refused record,
    on a running test computer through the product route. Then add the
    formula with a replay-equals-live test.

## Leading hypothesis, from code (2026-10-15 20:30 UTC; ledger not read)

- `vocab_upcast_deposit.go`: a from-scratch replay upcasts every
  `object_recorded` deposit through the frozen vocabulary rules. The
  live finalize path keeps the writer's spelling ("a live writer's own
  spelling stays authoritative").
- f95311775 (2026-10-10) found engineering writers minting legacy
  `co-super-*` ids that the live store kept and replay rewrote. It named
  the residuals `vocab-guard-ids`, `vocab-recovery-prefix` and
  `upcast-legacy-refs`.
- So the likely shape: a commitment record on these demo computers holds
  a legacy desk-spelled ID-shaped leaf. The record_id itself, or a
  reference such as an inbox, parent or evidence ref, written by a
  pre-fix writer. `identityFieldMigrated()` is true, and no formula
  verifies, so the replay refuses.
- **Consequence for the fix.** A scoped `record_id` formula would let
  replay continue. But if only a reference leaf changed, the replayed row
  still differs from live in its bytes and content hash: live never
  migrated it. This is the `upcast-legacy-refs` residual, not a missing
  formula alone.
- The honest repair belongs with that residual. Either replay keeps a
  pre-fix record's legacy leaves (a typed, versioned exception), or the
  live store gets the same one-time migration. Both are red, and both
  need the refused record's actual leaves first.
- Next probe: from a published base or the demo computer's tape through
  `choir trace export`, read which leaves of
  `obj:choir.commitment_record:…key-27d40897…` the frozen rule rewrites.
