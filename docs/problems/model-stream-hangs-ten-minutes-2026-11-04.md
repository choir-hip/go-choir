# A silent model stream idles a round for ten minutes (2026-11-04)

Mutation class: green (this doc). Fix: orange (provider stream readers).

## Evidence

Node B gateway journal, 2026-11-04, luna test arm vm-774a03f8. A chatgpt
gpt-6-luna stream (reasoning xhigh, 171 messages) started at 03:22:56Z and
produced nothing until the gateway logged `chatgpt: read stream: context
deadline exceeded` at 03:32:56Z, exactly the 10 minute `inferenceTimeout`
(internal/gateway/handlers.go). The guest's transient-failure retry
(internal/gatewayruntime/provider.go `call`) re-sent at 03:32:57 and the retry
finished in under a minute. The ten idle minutes cost the round its frozen
candidate. A second call of the same VM at 03:34:45 ended `context canceled`.

## Cause

No provider stream reader has a per-read deadline. Only the whole-call context
bounds a stream, so an upstream that accepts the request and then sends nothing
holds the call for the full 10 minutes.

## Failure modes the fix must handle

1. Silent from the start (no first byte): abort after the first-byte window.
2. Bytes then silence mid-stream: abort after the shorter idle window.
3. A reasoning model that thinks silently, then answers inside the window:
   must not be aborted.
4. A steady trickle of bytes (including SSE comments/pings): never aborted.
5. A stream that completes: the timer stops; no late close.
6. A body that cannot be closed (not an io.Closer): left unwrapped.
7. The abort must read as its own reason, `stream stalled: no bytes for Ns`,
   come back as a gateway 502 (transient), and consume the existing guest
   retry budget (3 attempts). No new retry loop.

## Windows

Journal 2026-11-02..04, chatgpt xhigh, 388 calls. Time-to-first-byte is not
logged, so call-to-next-call gaps per computer bound it from above (they
include tool time): p50 7 s, p90 44 s, p95 91 s, p99 894 s (the p99 is idle
gaps between rounds, not model latency). First-byte window 180 s; inter-chunk
window 75 s. The stream completion line now logs `ttfb_ms` so the next review
can measure the real distribution.

## Residuals

Non-streaming Call paths that `io.ReadAll` the body (zai, bedrock Call) are not
covered; a stall before response headers is bounded only by the 10 minute
deadline.
