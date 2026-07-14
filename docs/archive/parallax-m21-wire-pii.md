# Parallax: M21b — Wire PII Redaction into Trace Ingestion Path

**Conjecture (C19):** The PII redaction pipeline (from M21, now on main)
can be inserted into the trace event ingestion path so all trace events
are redacted before persistence, without changing existing event
production.

**Class:** orange — privacy infrastructure wiring
**Worktree:** /Users/wiz/.windsurf/worktrees/go-choir/m21-wire-pii
**Branch:** orchestrator/m21-wire-pii

## Open Edge from Pass 2
M21 delivered the PII redaction package but did not insert it into the
ingestion path. This mission wires it in.

## Spec
1. Read `internal/pii/` (the redaction pipeline from M21)
   - `regex_redactor.go` — regex-based redaction
   - `slm_redactor.go` — SLM-based redaction (Ollama)
   - `pipeline.go` — pipeline that chains redactors
2. Read `internal/trace/store.go` (the trace store from M20)
3. Insert the PII pipeline as a middleware stage in the trace store's
   ingestion path: events → PII redaction → persistence
4. The regex redactor should always run (fast, deterministic)
5. The SLM redactor should be optional (config flag, defaults off —
   requires Ollama running)
6. Redacted fields should be marked (e.g., `[REDACTED:email]`) so the
   redaction is visible in stored events

## Invariants
- No existing event production behavior changes
- Redaction is at ingestion (before persistence), not after
- Regex redactor always runs; SLM redactor is opt-in
- Redaction failure does not block event storage (store with warning)
- Tests cover: regex redaction in pipeline, SLM redaction optional,
  redaction failure graceful, end-to-end event → redaction → storage

## Acceptance Criteria
- `nix develop -c go test ./internal/pii/... ./internal/trace/...` passes
- `nix develop -c go build ./...` passes
- PII pipeline is inserted in trace ingestion path
- Redaction marks are visible in stored events

Return: conjecture verdict, test output, files modified, how the
pipeline is inserted.
