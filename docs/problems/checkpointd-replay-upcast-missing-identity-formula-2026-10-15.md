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
