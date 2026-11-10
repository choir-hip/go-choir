# System 1 route on the computer (2026-11-10)

Mutation class: orange (a new product API on the computer runtime). Rollback: revert.

A native client signed in with a bearer API key (`choir_sk_...`) asks Jev a few
fixed-option questions through its own computer: `POST /api/system1/judgments`.
An owner-wide API key names no computer, so the proxy refuses it with
`400 owner-wide api key requires a named computer (computer_id)`
(`internal/proxy/api_key_computer_authority.go`): a client with such a key
names its target with the `X-Choir-Computer: <computer id>` header (or a
`computer_id` query parameter). A key bound to one computer needs neither.
The proxy forwards `/api/...` to the computer unchanged (generic
`HandleProtectedAPI`); no proxy-to-gateway edge and no new gateway caller. The
computer runtime calls the one in-guest path every Jev site uses, `judgeCall`,
with the `interactive` gateway scope, so the call is limited by the gateway's
per-computer limiter and recorded in the guest-local Jev record like any other.

## Scope

The proxy maps a POST on a non-special `/api/` path to `write:runtime`
(`requiredAPIKeyScope`). A device key already carries it, so no scope is
invented. A key without it gets the proxy's 403 before the request reaches the
computer.

## Limits (written before the code)

Body at most 64 KiB; at most 12 questions; at most 8 choices per question (at
least 2, unique, non-empty); state a JSON object of at most 48 KiB; question
names at most 64 bytes; instructions at most 2000 bytes. Only `type: "choice"`
is supported (`score` and `noul` return distributions of another shape and are
not free). Unknown fields are rejected. Overall deadline 4 s.

## Ways it can fail, and what happens

| Failure | Behaviour |
| --- | --- |
| Body over 64 KiB | 413, nothing is called. |
| Malformed JSON, unknown field, trailing data | 400. |
| Over 12 questions, over 8 choices, state over 48 KiB, bad type, empty fields | 400 naming the field; nothing called. |
| Not POST | 405. |
| Cookie session instead of bearer | Same handler; the proxy has authenticated either way and sets the trusted user header. No bearer-only rule. A request without the trusted user header is 401. |
| Key without runtime scope | The proxy answers 403 before the computer sees it. |
| No decision model bound on the computer | 503. |
| Limiter exhausted (gateway answers 429) | 429 with `Retry-After: 1` (the in-guest client does not carry the gateway's value). |
| Gateway down or upstream error | 502 with `{error}`; the failure is recorded in the Jev record. |
| Slow upstream | The call is cut at 4 s: 502 `{error}`. The handler never waits longer. |
| Concurrent calls | Independent; `judgeCall` takes no runtime lock, the gateway limiter is the bound. |
| The model's choice is not one offered | That question carries `error`; other answers are returned. |
| The record write fails | The recorder is a bounded async queue and never blocks or fails the request; a drop is counted and logged by the recorder. |

## Wire shape

Request: `{"state": {...}, "questions": {"<name>": {"type":"choice","instructions":"...","criteria"?:...,"choices":["a","b"]}}, "label"?: "..."}`.
The question objects go to the gateway in the shape the in-guest verb sends:
`type`, `instructions`, `criteria`, `choices`. `criteria` is optional on the
route: when absent the route adds the in-guest default description per choice;
when present it must be a map of string descriptions whose keys are exactly the
choices, else the route answers 400 itself. `label` (1 to 40 characters of `[a-z0-9_.-]`) is recorded as
the record's site: `client` by default, `client:<label>` when given.

Response: `{"answers": {"<name>": {"choice": "a", "confidence": 0.93, "probabilities": {...}}}, "upstream_ms": n, "total_ms": n, "record_id": "<decision id>"}`.
`confidence` is the probability the judge gave the chosen option. `record_id` is
the Jev record's `decision_id` for the call.

## Defects found on staging

2026-11-10 22:46Z, staging at 79bd88f8b, on the test computer d@e.

1. Problem: every request failed `502 {"error":"gateway client: judge judgment
   provider unavailable"}` in about 130 ms, with the proof script's own body.
   Evidence: gateway log `jev call ... class=interactive questions=2
   upstream_ms=21 status=400 failed`, then `OpenRouter returned 400 Bad
   Request`, while the paper's in-guest calls returned 200 in the same minute.
   Cause: the route forwarded each question as `{type, instructions, choices}`.
   The vendor reads a choice question through its `criteria` map (choice name to
   description; `docs/reference/system1/jev-vendor.md` section 3) and rejects a
   question without one. Every in-guest site adds `criteria`
   (`system1Request`, the op-ed gate); the route, whose clients send only
   `choices`, did not. Confirmed on staging with 2 requests to d@e: the same
   question with `criteria` returned 200 (`upstream_ms` 104), without it 502
   and an upstream 400 in the gateway log. Fix: the route adds the default
   criteria when absent and refuses, with its own 400, criteria that do not
   match the choices; the gateway log now carries the provider's error message
   (at most 300 bytes, control characters stripped).
2. Problem: with an owner-wide key and no computer named, the proof got `400
   owner-wide api key requires a named computer`. Cause: correct proxy
   behaviour that the proof script and this document did not state. Fix: the
   rule is written above and `scripts/proofs/system1-route.sh` sends
   `CHOIR_COMPUTER_ID` as `X-Choir-Computer`.
