# M11 candidate-B reject decision fails pin verification

**Status:** identified 2026-09-29; fix in flight.

## Symptom

`POST /api/computers/{id}/self-development/operations/{op}/decision` with
`decision: "reject"` returns 400:

```
computer event appender: head CAS: computer event client: corpusd returned 400:
computer event CAS: pin verification: event artifact service: payload pin mismatch
```

The operation remains `awaiting_approval`; the reject event never commits.
Observed live on staging during m11 probe run 6
(`computer-ab32de9c71db4f798ad6681021b97956`, op
`selfdev-b7f59abf44f0c62d0ca82121395c9272`).

## Root cause

`internal/agentcore/api_self_development.go` builds the reject decision event
with `PrivacyClass: "owner"` but appends the rejection reason as an output
payload with `PrivacyClass: "private"` and `Private: true`.

`internal/platform/event_artifacts.go:154` validates each payload pin receipt:

```go
receipt.KindFields["privacy_class"] != request.Event.PrivacyClass
    -> "payload pin mismatch"
```

`PinPrivatePayload` (`internal/computerevent/http_client.go:126`) pins
`privacy_class = "private"` unconditionally, so the pin receipt can never equal
the event's `"owner"` class. Both sides landed in the same commit `7d635330`
("freeze disabled audited cutover"); the reject branch has never committed an
event through this path.

## Why the approve path survives

Approves only attach input payloads (`mode receipt`, optional consensus
receipt) pinned non-privately with `PrivacyClass: "owner"` — matching the
event. Rejects uniquely append a `private` output payload.

## Fix direction

The rejection reason is an owner-visible decision record, not secret payload —
pin it as `"owner"` (non-private), matching the event class. Alternative —
relaxing the authority check to accept `private` payload pins on non-private
events — widens the pin contract for one caller; rejected as non-boring.
