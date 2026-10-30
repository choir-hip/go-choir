# Inference breaker: one computer's rejected requests open the provider for everyone (2026-10-10)

Found by the sixth Gate 2 reality rerun (M11 probe) on staging, build
3e69567b, disposable `vm-084aa257…` (operation `selfdev-…`, receipt
`evidence/m11-rerun-2026-10-10T00-19-25Z.json`). Written 00:34Z. Problem
first; no fix in this commit. Mutation class of the fix: red
(gateway / provider calls).

## Evidence (gateway journal, Node B)

- 00:23:48 – 00:24:52: nine `opencode-go: status 400 Bad Request
  (sanitized)` responses, all for the M11 computer's engineering run
  (deepseek-v4.1-flash).
- 00:24:55 – 00:24:57: the `opencode-go` breaker is open; calls from a
  **different computer** (`vm-fda28af1…`, the Texture suite's run 9)
  fail with `circuit open (upstream unhealthy)`.
- 00:25:07 – 00:25:09: the M11 run's next calls fail on the open
  breaker; at iteration 45 the run ends, the assignment is cancelled and
  the operation goes `failed` ("gateway call failed … circuit open").
- 00:33Z: the provider serves both computers normally.

## What this shows

1. **A client error strikes the shared breaker.**
   `CircuitBreakingProvider` (`internal/gateway/circuit_breaker.go`)
   records every error as an upstream failure, including a 400 (the
   provider refusing our request) and a caller cancellation. One
   computer sending bad requests opens the provider for every computer.
   This is the same class as
   `search-plane-cooldown-on-caller-cancel-2026-10-09`: a per-caller
   failure counted as provider health.
2. **Why the provider refused the requests is unknown.** Providers
   discard non-2xx bodies (`internal/provider/provider.go`, "may contain
   provider details or credentials"), so the 400's reason (context too
   long, malformed tool sequence after compaction, …) is not recorded
   anywhere. Hypotheses only: H1 the run's context passed the model's
   limit (rerun 4 showed ~140K input tokens per call near compaction);
   H2 a malformed message sequence after a compaction.
3. **One provider outage fails a whole self-development operation.**
   The engineering run has no fallback provider and no retry after the
   breaker closes; the assignment is cancelled at the first open breaker.

## Fix directions

- The breaker counts only provider-side failures: 5xx, 408, 429,
  transport errors and provider timeouts. A 4xx request rejection and a
  caller cancellation pass through to the caller without a strike.
- Record the provider's error code and message for 4xx (redacted,
  truncated), so the cause of a rejected request is diagnosable.
- Later (named residual `engineering-provider-outage-fate`): an
  engineering run that meets an open breaker waits or fails over instead
  of cancelling its assignment.
