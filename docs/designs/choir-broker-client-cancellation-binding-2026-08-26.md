# Define Boundary: Client-Cancellation Broker Binding (single-path)

<decision_id: broker-client-cancellation-binding>
<frozen_candidate_digest: f252fc550fd0a1acc22a5aa0198dcf9b6078ce04db96d8b6fc141a42a73a634a>
<deployed_commit_at_decision: ae12f82d>
<recorded_at: 2026-08-26>

## Decision

Client/activation cancellation must cancel the in-flight broker operation on the
SAME single broker path used by all verbs (exec, go_eval, read_file, write_file,
etc.). It is NOT a go_eval-only patch. The design below is the accepted boundary
for implementing this as one broker-protocol change.

## Current behavior (verified)

- Broker RPC is synchronous: `handleRPC` (cmd/capsule-broker/main.go:322) creates
  `ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)` per
  request and runs the verb to completion inside that window.
- `BrokerClient.call` (internal/capsule/broker_client.go) serializes the encode/
  decode round trip with `connMu`, but only sets socket read/write deadlines from
  the caller context. It sends NO cancellation to the broker and does not
  identify the in-flight operation.
- Therefore when a caller (Executor.GoEval / Exec) context is cancelled (client
  timeout, activation death, run termination), the client stops waiting, but the
  broker keeps executing the verb until its own 60s deadline or capsule
  destruction. Pdeathsig covers only broker death, not activation death.

## The accepted design (single-path, all verbs)

1. **Operation IDs**: the broker issues a per-RPC `OperationID` when it admits a
   long-running verb (exec, go_eval). The client receives it in the response
   header and can reference it for cancellation. Non-long-running verbs
   (read_file, stat, list_dir) need no cancellation.

2. **Cancel verb**: add a broker `cancel` verb (capability-verified like every
   other verb) that takes the `OperationID`. The broker cancels that operation's
   context, SIGKILLs its worker process group (for exec/go_eval), and reaps the
   worker before acknowledging cancellation. This is added to `RoleVerbSets` for
   `RoleCoSuper` (and `RoleResearcher` when Researcher needs it).

3. **Connection-lifetime / activation binding**: on client disconnect (the broker
   sees the connection close) or on activation/capability revocation, the broker
   cancels any in-flight operation owned by that connection. This makes
   activation death terminate the worker even if the client never sends an
   explicit cancel.

4. **Admission + reap invariant**: the broker retains the process-group handle and
   operation record until the worker is confirmed reaped. It acknowledges
   cancellation only after reaping, and classifies the attempt as
   cancelled/interrupted (never successful).

5. **Single-path**: the same cancel/operation-ID mechanism is used for `exec` and
   `go_eval`; no verb gets a separate cancellation path.

## Why not implemented now

This is a broker-protocol change across the single execution substrate (protected
surface: capsule execution + capability broker + cancellation). It requires a
Define boundary, a frozen candidate, a linux cross-build + real RPC round-trip
test, and a re-review before landing. It also depends on the Researcher
capsule-context design (which adds a Researcher verb set). Recorded here so the
next implementer adds it as one coherent protocol change rather than a
go_eval-only band-aid.

## Related design

- docs/designs/choir-researcher-go-eval-capsule-context-2026-08-26.md

## Evidence

- cmd/capsule-broker/main.go handleRPC (60s context.Background) + handleExec /
  handleGoEval (worker spawn, Pdeathsig, reap-on-timeout)
- internal/capsule/broker_client.go call (connMu serialization, socket deadlines,
  no cancel verb / operation ID)
- internal/capsule/capsule.go acquireOp/releaseOp (inflight counter, not
  per-operation cancel)
