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
| `turn_id` | required; `[A-Za-z0-9][A-Za-z0-9_.-]{0,63}` (no colon); the client's command id and the idempotency key; it names one set of `words` |
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
being answered right now, or was used before for different words (use a new id), `413` body or `page.text` too large, `503` the computer
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
with `status` `failed`. It happens when every pool member refused (or ended
with nothing speakable) before a sentence was sent, or when the model failed
after a sentence was sent (`spoken` is then "Sorry, I lost my place.", and a
half sentence is never sent).

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
  "turn_record_id": "autoradio-turn-req:cmd-7f3a9c",
  "report_record_id": "autoradio-turn-rep:cmd-7f3a9c",
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
order with its `outcome`: `answered`, `refused` (an error before a sentence was
sent), `empty` (the stream ended cleanly with nothing speakable), `failed_mid_stream`,
`client_gone`. A refusal or an empty stream before the first sentence is walked
past silently: the listener sees no `error`.

### `handoff`

Reserved for slice 2 (design section 4). Slice 1 never sends it; ignore unknown
event names.

## Idempotency and disconnects

A `turn_id` already answered `completed` (at least one sentence) is replayed:
the recorded sentences as `speech` events and a `done` with `"replayed": true`,
no model call. A failed or interrupted turn, and a turn that produced nothing
speakable, is not a recorded answer: the same `turn_id` with the same words is
answered anew. The same `turn_id` with different words is `409`.

## Records (the tape)

Both are commitment records, owner-private (`owner_private:autoradio_turn`),
with `SubjectRefs` `desk=app:autoradio`, `record_kind=autoradio.turn`,
`turn_id`, `words_sha256`. A record's kind is in its id prefix and a `turn_id`
has no colon, so no client string can alias another record.

1. `autoradio-turn-req:<turn_id>`: the owner's request, written before the model
   is called: a notice-shaped directive (objective and body = the words, target =
   the page address). It is a notice, not a Cast, on purpose: a Cast request is
   counted by the inert-delegation scans (`types.InertKind`, `InertRates`,
   `InertCounts`), which would file every voice turn as a delegation by a desk
   named for the owner with no falsifier. It names no inbox, so it wakes no desk
   and creates no run. A test pins that no scan counts it.
2. `autoradio-turn-rep:<turn_id>` for a completed answer (one claim per
   sentence, never zero), or
   `autoradio-turn-fail:<turn_id>:<status>:<unix-nano>` for a failed or
   interrupted one. `Report.StopReason` is the status; `RelatedIDs` holds
   record 1; the subject refs carry provider, model, hop timings and members
   tried.

A request record that cannot be written does not silence the listener: the
answer streams and `done.record_error` says so.

## The model

Role `autoradio` (policy section `[roles.autoradio]`, optional), pool
`autoradio`, in this order (owner 2026-11-11):

1. River `deepseek-ai/DeepSeek-V4.1-Flash-1017`. River and Synthetic are two
   providers that serve the same DeepSeek snapshot; Synthetic is reserved for
   self-development and is not in this pool.
2. Bedrock `us.anthropic.claude-haiku-5-5` (the catalog id; the design writes
   `claude-haiku-5-5`). **Bedrock does not stream**: `BedrockProvider.Stream`
   makes one whole call (the wire is AWS binary EventStream, not SSE) and emits
   the text as one chunk, so as the fallback its answer arrives at once and the
   sentences follow back to back. Real Bedrock streaming is a separate change.

Not in the pool, by ruling: devin `swe-2-flash` (overloaded by AutoPaper),
opencode-go `beam-free` (often parked), Synthetic.

The chain is the pool only; there is no continuation chain behind it. The call is
streamed, has no tools, and asks for reasoning at most `low`. **Thinking is not
turned off on either member** (owner 2026-11-09, enforced by
`internal/provider/thinking_floor_test.go`: River maps off spellings to `low`,
Bedrock Haiku 5.5 always sends adaptive thinking), and it is counted in
`max_tokens`, so the call asks for `max_tokens` 1200 and the prompt keeps the
spoken answer to a few sentences. A stream that ends with nothing speakable (the
budget spent on thinking) is treated as a refusal and walks to the next member.

## What ends or redirects a turn

The desk holds no timer of its own (owner 2026-11-11: "we shouldn't have these
deterministic timers in the way"). Speed is measured (`done.timings_ms`, the log
line), not enforced. Only real signals end or redirect a turn:

| Signal | What happens |
|---|---|
| A provider's real refusal or error before any sentence has been sent (typed quota, non-200, connection error, the gateway's error line, a stream that ends with nothing speakable) | the turn walks to the next pool member; the listener hears nothing of it |
| Every member has refused | one `error` event with a spoken line, status `failed` |
| A real error after a sentence has been sent | the turn ends `failed` with what was said; no other voice is started; a half sentence is never sent |
| The client goes away (the listener interrupts, or the app closes) | the request context is cancelled, the upstream call is cancelled, no further member is called, the turn is settled `interrupted` |

The listener decides when a turn has gone on too long. Bounds on what a stranger
can cause stay: authentication, the 80 KB body, 48 KB page text, field and
history limits.

### Timeouts that are not the desk's, which could cut a long answer

Listed, not changed in this slice:

| Where | Value | Effect on a streamed answer |
|---|---|---|
| HTTP server `WriteTimeout` (`internal/server/server.go`) | 120 s | would cut a response open longer than that; this handler lifts the deadline for its own response only (`SetWriteDeadline(time.Time{})`), as the lifecycle stream does |
| HTTP server `ReadTimeout`, `IdleTimeout` | 120 s | read before the answer starts; idle between requests |
| Guest to gateway client (`gatewayruntime.gatewayClientTimeout`) | 10 min 30 s | `http.Client.Timeout` covers reading the whole streamed body |
| Gateway per-inference context (`inferenceTimeout`, `internal/gateway/handlers.go`) | 10 min | cancels one upstream call |
| Provider HTTP clients (`defaultProviderHTTPTimeout`) | 10 min | same, per provider |
| Proxy and vmctl hops between the phone and the guest | not read in this change | unknown; the first staging run shows whether the proxy flushes and holds the stream |

None of them is reached by a spoken answer of a few sentences.

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
- Bedrock Haiku 5.5 is not streamed by the gateway provider (one whole call, one
  chunk). Its pool position makes that acceptable; it is not a streaming test.
- A stream can end cleanly with nothing to say when thinking spends the whole
  `max_tokens`. Treated as a refusal; never recorded as a completed answer.
- A provider quota before the first chunk comes back as an HTTP error before
  any SSE (the typed `provider_quota` answer), which `StreamText` returns as
  `provideriface.ProviderQuotaError` before any delta.
- A voice turn written as a Cast request is counted by the inert and
  calibration scans; the request record is a notice for that reason.
- The package desk `app:autoradio` is a registry profile once registered, but
  a policy role must be known before the desk is, so the policy role is the
  bare `autoradio`.
