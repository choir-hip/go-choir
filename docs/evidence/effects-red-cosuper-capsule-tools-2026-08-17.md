# Effects CoSuper capsule tool environment blocked — 2026-08-17

**Boundary:** execute. Problem documentation first. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host at observation:** `https://choir.news/health` reported `7da74d9b78542a0b4e0e39f95dd2a7fa515e7a59`.
**Fix commit:** `a1f3d2cf93a84cf6c20e246202e6b74b90a6e932` (deployed `2026-08-17T21:44:54Z`; guest `2026-08-17T21:45:46Z`).

## Live observation

The retry path is unblocked and the LLM now runs. Fresh Super `c4cd7200` completed and bound CoSuper `assignment-c60a8912-a578-547e-9293-5a922a3de040` (capsule `active`). The CoSuper `run:assignment-c60a8912` ran ~3.5 minutes, consumed tokens, and completed with a blocker report instead of hanging.

Three capsule-tool blockers on that run (pre-`a1f3d2cf`):

1. **`capsule_exec` cannot run `bash`.** The capsule-broker exec handler runs `exec.CommandContext(ctx, "bash", "-c", ...)` but `bash` is not in the capsule PATH (only `/bin/sh`). Every exec fails `exec: "bash": executable file not found in $PATH`.

2. **`capsule_write_file` is denied.** The overlay upperdir `/run/choir/capsules/capsule-72793e49-042e-5d68-9411-aa93a0a447c8/upper` is not writable by UID 65534 (the capsule process user). Writes to `/workspace/platform/...` and even `/tmp` return "permission denied".

3. **Assignment becomes non-executable** (consequence of 1+2): after the tool failures, `record_assignment_result`/`commit_transaction` report "assignment fate is not active and bound", so no source could be authored, frozen, or proposed.

## What landed

`a1f3d2cf` inherits the autoputer toolchain PATH into the broker and chowns overlay upper/work dirs to UID 65534. Staging and the guest are on that commit at epoch **293**.

The CoSuper that observed the tool failures completed without terminalizing the assignment. Boot reconcile on `a1f3d2cf` now fails on that leftover bound assignment; see `docs/evidence/effects-red-cosuper-completed-run-cancel-2026-08-17.md`. The PATH/overlay fix is deployed and is not the current retry blocker.

## State

Operation `selfdev-b090bcd72d300fed17cb3f5a142f8595` stays `executing`, no bundle. Constructed freeze `7122f279` unchanged. Mode `propose_only` gen 1. No mail. This is not a freeze.
