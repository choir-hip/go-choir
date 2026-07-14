# Parallax: M22b — Wire Health Endpoints into Gateway Router

**Conjecture (C20):** Health endpoints and circuit breakers (from M22,
now on main) can be mounted in the gateway/proxy router so health is
checkable from outside the service, without disrupting existing request
routing.

**Class:** orange — reliability infrastructure wiring
**Worktree:** /Users/wiz/.windsurf/worktrees/go-choir/m22-wire-health
**Branch:** orchestrator/m22-wire-health

## Open Edge from Pass 2
M22 delivered health endpoints and circuit breakers but did not mount
them in the gateway router. This mission wires them in.

## Spec
1. Read `internal/health/` (the health check package from M22)
2. Read `internal/proxy/handlers.go` and `internal/proxy/router.go` (or
   equivalent) to find the route registration
3. Mount health endpoints:
   - `GET /health` — overall service health
   - `GET /health/{service}` — per-service health (sourcecycled, runtime,
     qdrant, dolt, ollama)
4. Wire circuit breakers into the provider call paths:
   - LLM provider calls: circuit break on repeated failures
   - Qdrant calls: degrade gracefully (skip dedup, don't block)
   - Ollama calls: skip semantic dedup or queue for later
5. Health endpoints should NOT require auth (public, no secrets in response)

## Invariants
- No existing request routing behavior changes
- Health endpoints are public (no auth required) but expose no secrets
- Circuit breakers are additive (existing calls still work, just with
  breaker protection)
- Tests cover: health endpoint responds, per-service health, circuit
  breaker open/closed/half-open states

## Acceptance Criteria
- `nix develop -c go test ./internal/health/... ./internal/proxy/...` passes
- `nix develop -c go build ./...` passes
- Health endpoints mounted in router
- Circuit breakers wired into provider call paths

Return: conjecture verdict, test output, files modified, how endpoints
and breakers are mounted.
