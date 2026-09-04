# RLM review blockers — problem receipt (2026-09-04)

Agentic consensus panel (codex, cursor, opencode, omp-gpt56-sol,
omp-gpt56-terra, omp-gemini38, omp-cursor-grok46; convergent mode)
reviewed the Def 2 RLM stack `31afb93c~1..aa4c4835` for live-proof
readiness. Verdict: **6 block, 1 ship-with-fixes. Do not run the live
sealed CoSuper proof on this stack.** Raw outputs:
`.agentic-consensus/agentic-consensus-20260904-085633/`.

## Locally verified blockers (traced in-tree, not panel hearsay)

**B1 — Session workers die at spawn (correctness, blocking).**
`sessionConfigFor` hardcodes `computerID: ""`
(`cmd/capsule-broker/session_worker.go:197`). The worker passes it as
`--session-computer-id ""` to `ExecuteWorkerSessionStdin`, which calls
`NewBroker` *before* the `"worker-local"` fallback
(`internal/yaegikernel/sidecar.go:195-208`); `NewBroker` rejects empty
`ComputerID` (`internal/yaegikernel/broker.go:38-41`) and the worker
`os.Exit(2)`s. First RLM cell gets EOF/transport failure; every cell
respawns a doomed worker. `sessionWorkerReady` is hardcoded `true`, so
no tools fallback engages. The shipped tests never spawn a real worker
(`/nonexistent/broker` + `sleep`), which is why this shipped.

**B2 — Researcher `go_eval` inherits full choir authority (guest-TCB
escalation, blocking).** `RoleResearcher` holds `go_eval`
(`internal/capsule/roles.go:25-28`) but the worker mints a handle with
`ActionExec|ReadFile|WriteFile|ListDir|Assign|Message` unconditionally
(`internal/yaegikernel/choir.go:38-40`) with no role input. Fixing B1
alone would arm this: a read-only Researcher could `choir.WriteFile` /
`choir.Exec`. B1+B2 ship atomically or not at all.

**B3 — Session lifecycle verbs fail closed for every role (wiring,
blocking for host pre-flight).** `get_actuator`, `init_session`,
`close_session` appear in no `RoleVerbSets` entry; `handleRPC`
(`cmd/capsule-broker/main.go:380-381`) rejects them before dispatch.
No agentcore caller exists for `get_actuator`, and the CoSuper overlay
still carries all legacy JSON tools — route selection does not drive
the model-facing schema. Schema derivation is proof-run work (agentcore
overlay + model), not this repair.

**B4 — Worker teardown leaks pipe FDs (reliability, medium).**
`spawnSessionWorker` keeps parent `stdinW`/`stdoutR`; `killLocked`
never closes them. Every poison/timeout replacement leaks two FDs in
the long-lived broker.

## Adjudication

- gemini's "ready for direct `go_eval`" dissent is rejected: it missed
  the `NewBroker` empty-id rejection that five panelists traced and
  local inspection confirms (B1). Majority + demonstrated fact agree.
- The brief's "read-only choir" premise was wrong: Def 2 item 3
  requires CoSuper `WriteFile`/`Exec`/`Assign`. The defect is *unscoped*
  binding (B2), not CoSuper capability. opencode's scope-doc
  contradiction is resolved this way.
- `Assign`/`Message`/`Outcome` worker-local retention (all panelists)
  is accepted as an architectural gap; the repair surfaces receipts in
  `SessionResult` so the host can reconcile, deferring a full
  callback-channel to a later Definition if the proof demands it.
- Deferred to the proof run (not code defects): agentcore
  route→schema derivation, guest-side `CHOIR_ACTUATOR` staging,
  `run Acceptance` on a live capsule.

## Repair plan (orange; rollback: git revert per commit)

1. B1: real computer identity (`b.capsuleID`) into worker config;
   stdout ready-handshake after prebind; per-call tools fallback with
   receipt on spawn failure (fallback becomes real).
2. B2 (same commit as 1): `--session-role` from verified capability;
   Researcher scope = read-only exports + read-only handle; method-level
   guards as defense in depth.
3. B3: `get_actuator` (both roles, read-only query) +
   `init_session`/`close_session` (CoSuper) in `RoleVerbSets`.
4. B4: close pipe FDs in `killLocked`.
5. Receipts: drain worker-local assign/message entries into
   `SessionResult` after each cell.
6. Tests (linux CI): real `exec-go-session` spawn + `1+1` eval;
   Researcher confinement (write/exec denied); fallback on bad binary;
   containment stays `sleep`-based plus a Yaegi-loop timeout case if
   cheap.
