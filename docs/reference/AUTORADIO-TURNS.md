# POST /api/autoradio/turns: the Autoradio desk's fast answer

Status: slice 1, built on branch `lane/autoradio-desk`, not deployed. Design:
[autoradio-desk-fast-turns-2026-11-11.md](../design/autoradio-desk-fast-turns-2026-11-11.md)
(sections 3 to 7, and 11 for the widened role). This page is the contract the
app lane calls. Code: `internal/agentcore/api_autoradio.go`,
`internal/speechsplit`, `internal/gatewayruntime/text_stream.go`.

## What it is

One request, one streamed model call, one server-sent-events response. The
listener's words about the page in front are answered by the `autoradio` role's
pool; each complete sentence is sent as a `speech` event as soon as it is
written, so the phone speaks the first sentence while the model is still
writing. No tools, no hand-off, no run, no Texture document in slice 1: when
the question needs real work the answer says so in one sentence. The turn
leaves two records on the tape (below).

Authentication is the phone's other routes' (`/api/system1/judgments`): a
signed-in client through the proxy; no owner is `401`. The route is on the
listener's own computer: send `X-Choir-Computer` with an owner-wide API key, as
`scripts/proofs/system1-route.sh` does.

## Request

`POST /api/autoradio/turns`, `Content-Type: application/json`, at most 80 KB.
Unknown fields anywhere are refused (`400`).

```json
{
  "turn_id": "cmd-7f3a9c",
  "words": "What is the top story?",
  "page": {
    "address": "pub:front@3#sha256:ab12...",
    "title": "Front page",
    "text": "A strike ended in Nancy. Talks resume Monday.",
    "text_sha256": "<64 lowercase hex, optional>"
  },
  "heard": {"block_index": 2, "text": "the last 300 characters spoken"},
  "programme": "<edition document id, optional>",
  "history": [
    {"role": "listener", "text": "Who won?"},
    {"role": "radio", "text": "The union."}
  ]
}
```

| Field | Rule |
|---|---|
| `turn_id` | required; `[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}`; the client's command id and the idempotency key |
| `words` | required; the listener's words verbatim; 1 to 2000 bytes after trimming |
| `page.address` | optional; at most 512 bytes; `pub:<route>` with version and manifest hash, or `texture:<doc>@<revision>`; recorded, not resolved in slice 1 |
| `page.title` | optional; at most 300 bytes |
| `page.text` | the text the phone is showing; at most 48 KB (`413` over); data from the world, never instructions |
| `page.text_sha256` | optional; recorded; the server hashes the text itself when absent |
| `heard` | optional; `block_index` at least 0; `text` is cut to its last 300 characters |
| `programme` | optional; at most 128 bytes; recorded and shown to the model |
| `history` | optional; at most 8 lines, each `listener` or `radio`, 1 to 1000 bytes; the last few turns, oldest first |

Refusals are plain JSON `{"error": "..."}` before any stream starts: `401` no
owner, `405` not POST, `400` bad body or fields, `409` the same `turn_id` is
being answered right now, `413` body or `page.text` too large, `503` the computer
has no streaming model bound. Once the stream has begun the status is `200` and
a failure is an `error` event.

## Response

`200`, `Content-Type: text/event-stream`, `Cache-Control: no-cache`,
`X-Accel-Buffering: no`. The first bytes are the comment `: autoradio` (ignore
lines that start with a colon). Each event is `event: <name>` then
`data: <one-line JSON>` then a blank line.

### `speech`

```
event: speech
data: {"seq":0,"text":"The top story is a strike in Nancy."}
```

One complete sentence, in order, `seq` from 0. Speak it. Plain text for the
ear: no markdown. Sentences are cut at `.` `!` `?` followed by a space and the
next sentence, never inside a number, a dotted abbreviation or after a title
(`Dr.`, `U.S.`, `3.5`); a run of 200 characters with no end is cut at a pause.

### `error`

```
event: error
data: {"spoken":"Sorry, I can't answer that right now.","reason":"river/deepseek-ai/DeepSeek-V4.1-Flash-1017 refused"}
```

The turn failed. `spoken` is a plain line to speak (after any `speech` already
sent); `reason` is for logs, not for the listener. Always followed by `done`
with `status` `failed`. It happens when every pool member refused before saying a
word, when the model failed after the first sentence (`spoken` is then "Sorry, I
lost my place.", and a half sentence is never sent), or on the 30 s deadline.

### `done`

Last event of every stream. Example:

```json
{
  "turn_id": "cmd-7f3a9c",
  "status": "completed",
  "replayed": false,
  "provider": "river",
  "model": "deepseek-ai/DeepSeek-V4.1-Flash-1017",
  "sentences": 3,
  "turn_record_id": "autoradio-turn:cmd-7f3a9c",
  "report_record_id": "autoradio-turn:cmd-7f3a9c:report",
  "timings_ms": {"record_written": 4, "upstream_request": 4, "upstream_first_token": 610, "first_sentence": 790, "done": 1180},
  "timestamps": {"received": "2026-11-11T14:00:00.001Z", "record_written": "...", "upstream_request": "...", "upstream_first_token": "...", "first_sentence": "...", "done": "..."},
  "members_tried": [{"member": "river/deepseek-ai/DeepSeek-V4.1-Flash-1017", "outcome": "answered", "ms": 1170}],
  "usage": {"input_tokens": 812, "output_tokens": 61, "cached_input_tokens": 0},
  "record_error": "only when the request record could not be written",
  "report_error": "only when the Report could not be written"
}
```

`status` is `completed`, `failed` or `interrupted`. `timings_ms` are
milliseconds since the server received the request (`-1` for a hop that did not
happen); `received` is the handler's receipt, so the proxy and network hops
before it are the client's to measure (send time to first byte minus
`first_sentence` is the transport). `members_tried` lists each pool member in
order with its `outcome`: `answered`, `refused` (an error before its first
chunk), `silent` (no first chunk within 6 s, cancelled), `failed_mid_stream`,
`deadline`, `client_gone`. A refusal before the first chunk is walked past
silently: the listener sees no `error`.

### `handoff`

Reserved for slice 2 (design section 4). Slice 1 never sends it; ignore unknown
event names.

## Idempotency and disconnects

A `turn_id` already answered `completed` is replayed: the recorded sentences as
`speech` events and a `done` with `"replayed": true`, no model call. A failed or
interrupted turn is not a recorded answer: the same `turn_id` is answered anew.
If the client closes the connection, the model call is cancelled at once and the
turn is settled `interrupted`.

## Records (the tape)

Both are commitment records, owner-private (`owner_private:autoradio_turn`),
with `SubjectRefs` `desk=app:autoradio`, `record_kind=autoradio.turn`,
`turn_id`.

1. `autoradio-turn:<turn_id>`: the owner-origin request, a Cast directive in the
   shape `DeliverOwnerRequest` writes (objective = the words, target = the page
   address), written before the model is called. It names no inbox, so it wakes
   no desk and creates no run.
2. `autoradio-turn:<turn_id>:report` for a completed answer (one claim per
   sentence), or `autoradio-turn:<turn_id>:<status>:<unix-nano>` for a failed or
   interrupted one; `Report.StopReason` is the status; `RelatedIDs` holds record
   1; the subject refs carry provider, model, hop timings and members tried.

A request record that cannot be written does not silence the listener: the
answer streams and `done.record_error` says so.

## The model

Role `autoradio` (policy section `[roles.autoradio]`, optional), pool
`autoradio`, in this order: River `deepseek-ai/DeepSeek-V4.1-Flash-1017`, devin
`swe-2-flash`, opencode-go `beam-free`, Synthetic
`hf:deepseek-ai/DeepSeek-V4.1-Flash-1017`. River and Synthetic are two providers
that serve the same DeepSeek snapshot (catalog entries
`internal/modelcatalog/catalog.go`). Owner 2026-11-11: River, not Bedrock Haiku.
The chain is the pool only; there is no continuation chain behind it. Call:
`max_tokens` 220, reasoning at most `low`, no tools, streamed.

## Measuring it

`scripts/prove-autoradio-turns --host choir.news --api-key-file <file>
[--computer-id <id>] [--count 10]` sends N asks, prints per ask the time to
first `speech` event and to `done` (client clock) beside the server's own hop
timings, and the median; it exits non-zero when the median to the first
sentence is above 2.5 s.

## Surprises found building it

- The gateway reports an upstream failure after it has begun streaming as an
  untyped `data: {"error":"..."}` line (`handleStreamingInference`), not a
  `type: "error"` chunk. `parseGatewaySSE` (the desk loop's streaming reader)
  skips it and returns a clean end; `StreamText` reads it as an error and also
  requires the final `[DONE]` marker.
- A provider quota before the first chunk comes back as an HTTP error before
  any SSE (the typed `provider_quota` answer), which `StreamText` returns as
  `provideriface.ProviderQuotaError` before any delta.
- The package desk `app:autoradio` is a registry profile once registered, but
  a policy role must be known before the desk is, so the policy role is the
  bare `autoradio`.
