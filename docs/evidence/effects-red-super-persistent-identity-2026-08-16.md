# Effects Super start is not persistent Super — 2026-08-16

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `a9e4af419aa96018410cb13840cc0ee94afe39cb`
**Computer:** `computer-03335285269bdba4f94377e56879f9e6` epoch **276**
**Operation:** `selfdev-b090bcd72d300fed17cb3f5a142f8595`

## Live observation

Named solitaire Super start created run `cdf0af4c-fc24-47e8-881c-c6e9d1e6fa0b` (`request_source=self_development_operation`). The run completed at 2026-08-16T20:07:55Z. Operation state remained `executing`. Bundle digest empty. Mode stayed `propose_only` generation 1. Genesis 409. No mail.

Super result: blocked before implementation. `assign_co_super` returned `assigned CoSuper tools require the exact non-lifecycle persistent Super`. `report_to_texture` returned the same authority error. No source mutation.

Cause: `ensureSelfDevelopmentRun` stamped the computer-event trajectory onto the Super run (`trajectory-6235753c4abf1d67789796e165736f91`). `requirePersistentSuperExecution` refuses any Super whose `TrajectoryID` is nonempty. Agent ID was already `super:<owner>`.

## What landed

Self-development Super start now matches the persistent Super identity used by `assign_co_super`:

- persistent Super agent ID
- `TrajectoryID` stripped after create (same pattern as lifecycle-control persistent Super)
- operation id stays in metadata so `ListRunsBySelfDevelopmentOperation` still finds the unique run
- an executing operation whose unique Super run is terminal and has no bundle can be revived by the same idempotent POST

This does not mint a Texture `Direction=control` `execution_request`. CoSuper still cannot open Super. Texture still has no generic `update_coagent`. After this guest is deployed and owner-refreshed, Super can pass the first assignment gate. Opening CoSuper still requires a Texture-authorized control/work join (`assignment_trajectory_id` + delivered control). That remains unpaid.

## Tests

`go test ./internal/agentcore -run 'TestConcurrentExactRetriesRepairOneRequestedOperationRun|TestSelfDevelopmentStartRevivesTerminalPersistentSuper'`
