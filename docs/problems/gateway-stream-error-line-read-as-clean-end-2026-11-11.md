# The gateway's untyped mid-stream error line is read as a clean end

Date: 2026-11-11. Found while building the Autoradio desk's streamed call
(`docs/reference/AUTORADIO-TURNS.md`). Not fixed here: the shared reader's
behaviour is unchanged in that change.

## Problem

When an upstream provider fails after the gateway has begun to stream,
`handleStreamingInference` (`internal/gateway/handlers.go`) writes the failure
as a data line with no `type`: `data: {"error":"<sanitized>"}`, then returns
without the `[DONE]` marker. The guest reader `parseGatewaySSE`
(`internal/gatewayruntime/provider.go`) handles an error only when
`chunk.Type == "error"`; the untyped line unmarshals into an empty chunk, calls
`onChunk` with it, and is otherwise skipped. The scan then ends at EOF and the
function returns the text accumulated so far as a successful response, with
`ProviderName` set and no stop reason. A truncated stream is reported as a
completed one.

## Evidence

- Code: the two functions above. The gateway writes `errData, _ :=
  json.Marshal(map[string]string{"error": sanitized})` as a data line
  (handlers.go, the `err != nil` branch after the first chunk); no code path in
  the repo emits a chunk with `type: "error"` for a provider failure.
- Test: `TestStreamTextAnUpstreamErrorAfterStreamingBeganIsAnError` in
  `internal/gatewayruntime/text_stream_test.go` and
  `TestRiverFailureAfterTheFirstChunkIsAnError` in
  `internal/gateway/autoradio_river_stream_test.go` show the untyped line and a
  missing `[DONE]` are distinguishable on the wire; the new reader
  (`StreamText`) turns both into errors.
- Not observed in production: no ledger or journal row was read for a stream
  that ended this way. The existing streaming callers are `Provider.Execute`
  (the plain-prompt path) and the stream helper; the desk tool loop sets
  `Stream: false` and does not use the reader.

## Belief

About 85%: a provider failure mid-stream on `Execute` returns a truncated
answer as success. The reach is small because the desk loop does not stream; the
fix is to make `parseGatewaySSE` read an untyped `{"error"}` line and a body that
ends without `[DONE]` as errors, as `StreamText` does.

## Remaining error

- Whether any caller depends on a truncated stream being accepted (none found,
  not exhaustively searched).
- Whether the gateway should emit a typed error chunk instead of the untyped
  line; that is a wire change with two sides.
- The rate at which this happens in production is unknown.
