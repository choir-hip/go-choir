# Problem Receipt: Stale MaintenanceHold Survives Unhold (2026-09-03)

- Date: 2026-09-03
- Mutation class of this receipt: green (documentation); repair is orange
- Status: root-caused; fix in flight
- Computer: `computer-03335285269bdba4f94377e56879f9e6`

## The Problem

Two consecutive `unhold` calls both returned `{"status":"unheld"}` and the
on-disk ownership shows `hold_status: null`, yet every subsequent boot
(refresh and resolve alike) still carries `choir.runtime_maintenance_hold=1`
and the guest stays mutation-fenced (`RUNTIME_MAINTENANCE_HOLD=1`, no rewake).

## Root Cause

`mergeVMConfigOverrides` (`internal/vmmanager/manager.go`) merges the live
manager instance config with per-call overrides, but never touches the
`MaintenanceHold` / `RecoveryReplayOnly` bools: a plain bool merge cannot
express "clear", so the stale `true` retained in the manager's in-memory
instance config (from the 19:55Z maintenance-serve boot) wins on every later
`RecoverVM` / `RefreshVM`, regardless of the cleared ownership hold.
Unhold clears the ownership record; nothing clears the manager instance
config, and no boot path re-derives the flags from the ownership hold.

Contributing evidence: `HandleUnhold`/`ClearHold` verified correct (memory +
`ownerships.json` both null); the 19:59Z `fc-config.json` still contains the
hold kernel param; refresh refuses held computers outright, so a refresh can
never legitimately need a carried hold.

## Repair (in flight)

Take both boot-mode flags from overrides unconditionally in
`mergeVMConfigOverrides` — they are authoritative per-call intent. Caller
audit: normal resolve/recover/refresh paths build overrides with both false;
`recoverVMForDesktop` sets both explicitly from its held/replayOnly args;
maintenance-serve still fences. Test:
`TestMergeVMConfigOverridesTakesBootModeFromOverrides`.

## Residual

The epoch-870 guest is correctly fenced and servable for reads; a clean
unfenced boot after the fix deploys is the proof. Longer term, boot-mode
should be re-derived from ownership hold at every boot rather than trusted
from merged config.
