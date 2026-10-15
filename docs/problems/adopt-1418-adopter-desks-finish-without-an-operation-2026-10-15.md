# Adopt drill adopt-1418: with polling in place the adopter never opens an operation (2026-10-15)

Date: 2026-10-15 14:18-14:49 UTC. Drill: `node scripts/reeng_tracer.mjs --label adopt-1418 --scenario adopt --expect-sha e485b1966… --timeout-min 30 --apply-min 10`
on staging `e485b1966` with the polling tracer change (`yellow(tracer)`, 120 s
poll of `adopter_serves_the_change` and `identity_names_adoption`). Computers:
publisher `computer-2bb1d7e0…`, adopter `computer-3916f9ba…` (VM
`vm-5539585dea4d0bd4d5dae410498f3566`), both stopped by the tracer. Evidence:
`~/.config/choir-qa/traces/adopt-1418/receipt.json`. Status: documented;
nothing patched (the polling did not matter here: the run never reached
`applied`).

## Outcome

Publisher engineering drill reached `applied` (14:30:32; restore skipped by
design). Release published and read back; tampered record refused (422
`release_digest_mismatch`) and unpublished record refused (404
`release_not_published`), both unchanged; adoption requested 14:30:47
(`201`, `same_base: true`, adopter tree `2a5d1772…`). Then no adopter operation
appeared: `acceptance` 14:48:39 `reached=adoption_requested`, failed
`[frozen_patch_is_the_release, adopter_applied, planned_restart,
adopter_serves_the_change, identity_names_adoption]`. No `adopter_operation_state`
phase was ever recorded.

## Evidence (adopter console)

- First adopter engineering assignment `assignment-454e18e5…` ended `terminal
  failed` at 14:32:47 (about 2 min after the adoption request); its precommit
  rules settled `no` / `unresolved`.
- Texture authored its final turn `done:true` at 14:34:19; management's round 1
  logged "stays open ... an assignment it opene[d]" twice (14:31:20, 14:31:54).
  After 14:34:19 the console shows only the once-a-minute `fc_api` 400
  `Device not found` line (hotplug probe noise on a VM without a device) and no
  further runtime work through 14:48. 33 tool-loop iterations in total.
- No `not a directory`, no `mutation authority mismatch`, no
  `dispatcher: poison`, no broker `i/o timeout`.

## Classification (hypothesis; ledger not read)

The adoption request was accepted (`201`) but the adopter's desks did not carry
it to an operation: one engineering assignment failed within two minutes and
the Texture request was closed `done` without a second attempt, so nothing
froze. This looks like the engineering-desk judgment/environment class already
seen in jobs-0101 (A1-A3) and p5-go-served-0825, not an adoption-gate defect
(the gate refused both negative cases correctly and created the adoption). The
adoption's request path to the desks, and why the assignment failed, were not
read from the run ledger. Owed: read the adopter's ledger (turn-commit
reasons, controls, assignment failure reason) before deciding between a desk
prompt/environment problem and an adoption objective the desks cannot satisfy
(for example the release patch not applying to the adopter's source).

Heresy delta: discovered 0; introduced 0; repaired 0.
