# Texture desk `decide` rejects `decision_kind: "wait"` — run dies after verbatim retries

**Status:** DIAGNOSED 2026-09-28 on staging (deployed `6f1417ef`); fix
commit follows per `docs/memo-problem-documentation-first.md`.

## Symptom

M11 episode probe op `selfdev-006a0b220c664cf17a33dcc4ee3b4006`,
computer `computer-e7c6857858cad4bfd4377f0f8a0e7a1d` (VM bounced across
`10.200.5.2` → `10.200.6.2` → `10.200.7.2` through two suspensions):
after the second resume, the texture desk entered a ~45 s
activate-fail-respawn loop on `apply_owner_revision`. Nine consecutive
texture runs failed (`f7b6185a`, `0cb58059`, `cb4c3e09`, `1c69bb80`,
`f886af05`, `045d377d`, `e326269b`, `caa0217e`, `e92aa2bb`).

Run `e92aa2bb` event tape (seq 31–35):

- `tool.invoked` `choir.ApplyTexture({"op":"decide","decision_kind":"wait",
  "reason":"The durable Engineering ..."})`
- `tool.result` is_error:
  `tool_error: reduce: persist tray-1: decision_kind must be one of
  delegation_opened, delegation_skipped, delegation_deferred,
  wait_for_evidence, blocker, no_worker_needed`
- retry re-invoked the **identical payload under the same `call_id`**
  (`call_MA7jrBuq0GhCBVVwXxsLTedb`) — same verbatim `decision_kind:"wait"`
- `texture.agent_revision.failed`: `tool loop: required write tool did not
  succeed after 2 retries`

## Root cause

The enum is fixed in code (`tools_texture.go` `validTextureDecisionKind`).
`"wait"` is not a member; `wait_for_evidence` is. The desk model guessed
the natural spelling, and the executor's required-write retry replays the
rejected call byte-for-byte rather than letting the model re-compose —
so the enum-bearing error never gets a chance to produce a corrected
call within the retry window.

## Fix shape (separate commit)

Normalize the common synonym at ingest in `commitTextureNonRevisionTurn`
(`"wait"` → `wait_for_evidence`) before validation, and pass the
canonicalized kind onward so `textureDecisionTurnOutcome` reads
`TextureTurnWait`, not the `NoSemanticChange` default. One-line
substrate normalization; does not widen the canonical enum.

Residual: the required-write retry replaying identical arguments on a
deterministic validation error is a separate tool-loop defect — any
non-synonym validation rejection still burns the run unchanged.
