# M9a platform-follow bootstrap binds the route slot's desktop component as a computer ID

**Status:** confirmed on staging.  
**Found:** 2026-10-04 on disposable `computer-ac1808b4f04fc749c0781083ee747403` (`vm-b59963240d2cd8da769b92cacb213802`, owner `69fcbf14-7fa7-442b-9e69-9c2666274b9e`).

## Symptom

A platform-control-signed M9a update reached `effect_accepted`, `materialization_started`, `materialization_applied`, and `checkpoint_published`; the restarted guest served the new release marker. The expected fresh-route bootstrap did not run: the platform-follow route slot remained absent (`generation: 0`).

The M9a driver defines a fresh owner/desktop route slot as `computer:<owner-id>:primary` and explicitly requires the first update to bootstrap it to generation 1. The restarted guest made that valid route query, but vmctl's guest binding interpreted the slot's second component (`primary`) as a stable computer ID:

```text
runtime: platform update resume: vmctl client: ComputerVersion route resolution failed (status 403): caller not bound to route slot computer: no ownership for computer primary
```

## Evidence

- `docs/evidence/s0b-m9a-bundle-lifecycle-2026-10-04.json`.
- Event tape, update `s0b-m9a-20261004-0707`:
  - sequence 2 `effect_accepted`
  - sequence 3 `materialization_started`
  - sequence 4 `materialization_applied`
  - sequence 5 `checkpoint_published`
- The guest health after restart reported `self_development_marker: S0B_M9A_20261004_0707` and release digest `2e874fb951940e8156c4e50882ca96180763a51289555bbbab8433540f7739f1`.
- `vmctl`'s guest binding parses the same slot and calls `GetOwnershipByComputerID` on its second component. For this valid slot that lookup is `GetOwnershipByComputerID("primary")`, while the actual stable computer ID is `computer-ac1808b4f04fc749c0781083ee747403`.
- Host-internal resolution of `computer:69fcbf14-7fa7-442b-9e69-9c2666274b9e:primary` after the update returned `route_absent: true`.
- A product checkpoint and restore still succeeded (`witness_matched: true`, `frontend_restaged: true`), so the failure is specifically the route-projection tail, not update materialization or restore.

## Impact

M9a transport acceptance and even applied-release health can look successful while the route-promotion contract has failed. A subsequent operation requiring the ComputerVersion route identity is refused; this probe observed `kernel-capabilities` return `503 computer route identity unavailable`.

## Repair boundary

The guest request binding and route-slot schema must agree on the identity being bound. Preserve the owner/desktop slot contract and resolve its owner-plus-desktop pair to the stable computer before comparing the caller tap IP, or change both contracts atomically. Add an end-to-end fresh-computer assertion that the first M9a update exposes its marker and bootstraps the route to generation 1.

## Rollback

No repair was made. The disposable computer remains active on the M9a release and has a successful product restore checkpoint; it can be torn down as a disposable fixture.
