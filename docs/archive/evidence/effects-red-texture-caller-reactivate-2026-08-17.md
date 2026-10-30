# Effects retry unblocked: Texture caller reactivation — 2026-08-17

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` reports `7da74d9b78542a0b4e0e39f95dd2a7fa515e7a59`, deployed `2026-08-17T21:15:03Z`.
**CI:** `https://github.com/choir-hip/go-choir/actions/runs/32067847031` succeeded.

## Fix

`ensureSelfDevelopmentTextureCaller` now prefers the deterministic caller run and reactivates it (passivated → running), releasing any successor run that owns the Texture agent slot, instead of adopting a boot-minted successor. `lifecycleTargetWorkRequestedByTexture` is unchanged — the provenance check stays intact, and the Super work item's `created_by_loop_id`/`requested_by_run_id` (`3b18a6d7`) matches the reactivated caller.

The earlier "empty `CreatedByRunID`" concern was a serialization misread: the field is `created_by_loop_id` in JSON and was set to `3b18a6d7`.

## Verified live

Owner-scoped refresh `effects-texture-caller-refresh-2026-08-17T21:16Z` moved epoch **291 → 292** (LifecycleReceipt `01a01194-668e-7334-b6f1-f269bd217a3f`). Immediately after, the same operations POST (`effects-solitaire-start-2026-08-16T20:08Z`) returned **HTTP 200**.

- Texture agent `texture:0744fc0c-eaa6-5e22-acd5-dcb9cb68fb93` now has `active_run = 3b18a6d7-5fd4-5de4-86d2-c27954698548` (the deterministic caller; successor `aa4fc186` was passivated at boot).
- A fresh Super `c4cd7200-c2c5-4ac9-9912-2258372b087d` is `running` (`request_source=lifecycle_texture_control`).
- Operation `selfdev-b090bcd72d300fed17cb3f5a142f8595` remains `executing`, no bundle.
- CoSuper `run:assignment-fa38b037` remains `cancelled`. Constructed freeze `7122f279` unchanged. Mode `propose_only` generation 1. No mail.

This is not a freeze.
