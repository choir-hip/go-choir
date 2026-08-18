# Effects capability TTL expiry during long capsule executions — 2026-08-18

**Boundary:** execute. Problem documentation first. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` reports `1fc4a3696202b429e8f1da6ce816e86e945595ee` (deployed `2026-08-18T03:14:55Z`).

## Live observation

Following the deploy of `1fc4a369` (traversal permissions and bash job control fix), the epoch-297 refresh and operations POST succeeded: Super run `8f770a03` opened fresh implementation assignment `assignment-014aeb69-ee85-5af5-913e-e8ed686c6a43` in capsule `capsule-f3e1bb80-e4e3-5dc0-b592-a61c89d3ecd1`.

The CoSuper executed **34 tool loop iterations** writing solitaire code and running build/test commands via `capsule_exec`. The toolchain, bash execution, and overlay writes operated with complete success.

However, after 5.5 minutes of continuous execution, an inference network drop occurred, and the runtime's attempt to cancel/reconcile the run failed with:

```
guest credential: renewal refused
```

## Root cause

Two parameters constrained guest capability lifetime:

1. **5-minute default TTL with 60-second grace window:** `defaultComputerCapabilityTTL` in `internal/platform/event_capability.go` was 5 minutes, and `capabilityRenewalGrace` was 60 seconds.
2. **Lazy renewal only on event append:** `GuestCredentials.Capability()` was only called when an event was appended. During active capsule tool execution (where tools execute locally inside the capsule without touching the event store), no event appends occurred for 5.5 minutes. When `Capability()` was finally invoked at 5.5 minutes, the token had expired past the 60-second grace period.

## Repair

1. `internal/platform/event_capability.go`: `defaultComputerCapabilityTTL` increased to 30 minutes, and `capabilityRenewalGrace` increased to 15 minutes.
2. `internal/selfdev/credentials.go`: `StartBackgroundRenewal` runs a proactive 1-minute ticker that keeps the in-memory capability fresh without waiting for an event append.
3. `internal/autoputer/run.go`: `StartBackgroundRenewal` started on guest boot when credentials are wired.

## State

Operation `selfdev-b090bcd72d300fed17cb3f5a142f8595` stays `executing`, no bundle. Constructed freeze `7122f279` unchanged. Mode `propose_only` gen 1. No mail. This is not a freeze.
