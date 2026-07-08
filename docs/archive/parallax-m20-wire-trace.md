# Parallax: M20b — Wire Trace Store into Runtime Router

**Conjecture (C18):** The trace store (from M20, now on main) can be
mounted into the runtime router so trace events are persisted to Dolt
in production, without changing existing request handling.

**Class:** orange — observability infrastructure wiring
**Worktree:** /Users/wiz/.windsurf/worktrees/go-choir/m20-wire-trace
**Branch:** orchestrator/m20-wire-trace

## Open Edge from Pass 2
M20 delivered the trace store package but did not mount it in the runtime
router. This mission wires it in.

## Spec
1. Read `internal/trace/store.go` (the trace store from M20)
2. Read `internal/runtime/` to find where trace events are currently
   emitted/handled
3. Mount the trace store so events flow to Dolt persistence
4. Add a configuration flag or env var to enable/disable Dolt persistence
5. Ensure the store is initialized at startup and closed on shutdown

## Invariants
- No existing request handling behavior changes
- Trace persistence is additive (events still flow to existing handlers)
- Dolt connection failure does not crash the service (degrade gracefully)
- Tests cover: store initialization, event persistence, graceful degradation

## Acceptance Criteria
- `nix develop -c go test ./internal/trace/...` passes
- `nix develop -c go build ./...` passes
- Trace store is mounted in the runtime startup path
- Graceful degradation test passes (Dolt unavailable → log + continue)

Return: conjecture verdict, test output, files modified, how the store
is mounted.
