# Settlement Gate Item 2 — Code-Free Define Receipt (2026-09-09)

Mutation class: green (docs only). No source changed in this receipt.
Mission: `docs/definitions/choir-rlm-settlement-gate-2026-09-09.md`, acceptance item 2.
Prior: item-1 repair `b8aaa89b` (typed `EvalError`: preserve vs unsafe-to-reuse +
diagnostic kind, carried on `SessionResult`/`GoEvalResult`).
Worktree: 4 untracked unrelated-WIP paths preserved (mission-0 completion report,
report generator script, pycache, tmp/).

## Problem (observed, not inferred)

Under actuator=rlm, a session worker start failure diverts the cell to the one-shot
tools worker (`cmd/capsule-broker/session_worker.go:385-388` → `fallbackGoEval`,
`:426-438`). Two defects in one path:

1. **Two execution paths on the active RLM route.** The RLM cell executes with
   one-shot semantics (fresh interpreter per call, no heap, no tray staging) while
   the caller asked for the persistent session. The route dispatch
   (`cmd/capsule-broker/main.go:674-682`) selects the session; the fallback silently
   re-enters the tools worker from inside the RLM handler (the comment at
   `:684-688` admits the recursion hazard it avoids by calling one-shot directly).
2. **Exact diagnostics replaced by the process wait error.** The one-shot maps the
   worker's raw stdout bytes to `result.Stdout` and sets
   `result.Error = waitErr.Error()` (`main.go:787-802`): for a dead/failing worker
   that string is `exit status N`, not the typed session diagnostic. The
   `Fallback: true` mark plus stderr suffix (`session_worker.go:435-436`) narrates
   the diversion but does not restore the lost diagnostic or the lost heap/tray.

Route inventory (all other RLM→one-shot edges checked):
- `effectiveRoute()` (`main.go:73-80`) downgrades rlm→tools when
  `sessionWorkerReady` is false, but the flag is set `true` unconditionally at
  startup (`main.go:252-253`) and never mutated: the downgrade is dead config,
  advertised via `get_actuator`, never a live silent diversion. Out of scope;
  left as is.
- `handleGoEval` tools branch (`main.go:678-681`) serves explicit actuator=tools:
  the preserved rollback route, untouched.
- No readers of `GoEvalResult.Fallback` exist outside the setter: stopping the
  fallback strands no consumer. The field stays for wire compat (never set on the
  RLM route after this repair).

## Authorized repair boundary (item 2 only; red, next commit)

1. Delete `fallbackGoEval` (`session_worker.go:426-438`); the spawn-failure branch
   (`:385-388`) returns a typed session diagnostic instead and never executes the
   cell: `GoEvalResult{ExitCode: 1, Error: "session worker unavailable: ...",
   Reuse: unsafe_to_reuse, DiagKind: worker}` — the same shape item 1 already uses
   for transport failures (`:389-395`), so one typed diagnostic contract governs
   the RLM route.
2. Update the two comments that name the removed path (`session_worker.go:360-366`,
   `main.go:684-688`).
3. Update `TestGoEvalSessionFailsClosedWithoutBinary` to the new contract: spawn
   failure yields a Result (not top-level Error) with ExitCode 1, Fallback unset,
   `Reuse == unsafe_to_reuse`, `DiagKind == worker`, no worker leaked, and — with
   `brokerBin` nonexistent — proof the one-shot never ran (no fallback text, no
   second error).
4. Update the item-2 `now` card in the same commit (Define+Implement co-commit).

## Explicitly not in this boundary

`effectiveRoute` downgrade (dead config, advertised, out of scope above); one-shot
worker itself (explicit actuator=tools route, preserved not deleted);
`GoEvalResult.Fallback` field removal (wire compat; never set on RLM route);
identity/canonicalization (item 3), admission grammar (item 4), fate saga (item 5),
orphan close (item 6), staging proof (item 7).

## Evidence floor / rollback

Floor: local_test — updated broker contract test plus existing
`go test ./internal/yaegikernel ./internal/capsule ./internal/toolregistry` and
the agentcore capsule/fate subset with no regression; broker build
(`CGO_ENABLED=0 GOOS=linux`, linux-only package). Rollback: revert the repair
commit; this Define retained. If spawn-failure diagnostics cannot be typed without
the one-shot, the repair stops and this receipt records the refusal.

## Standing-questions answers for this boundary

1. Settled by: owner-chartered definition item 2 (this receipt executes, settles nothing).
2. Topology: conforms to charter entrypoints (broker session/one-shot workers).
3. Deletion citers: `fallbackGoEval` has one caller (the spawn-failure branch);
   `Fallback` field has no readers; both comments name the removed path and are
   updated in the same commit.
4. Production consumers: RLM route keeps session-only serving; tools route keeps
   one-shot serving; no caller depends on diversion.
5. Single authority for heap truth: unchanged (live interpreter or durable snapshot
   via poison+respawn); a spawn failure now surfaces instead of substituting a
   fresh-interpreter execution.
6. Artifact: local_test broker contract (this receipt is Define, not proof).
7. Fate-sharing: spawn failure needs only the broker to report; no worker required.
8. Restart: nothing durable added; failed spawn holds no worker (existing
   `len(b.sessionWorkers) != 0` assertion retained).
9. No SSH: broker unit tests run local (linux-tagged; darwin runs the portable
   subset, cross-build covers the rest — same as item 1).
