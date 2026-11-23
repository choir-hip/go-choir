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

`POST /api/autoradio/turns`, `Content-Type: application/json`, at most 256 KB (80 KB until 2026-11-15, when the queue snapshot was added). That is the sum of the part maxima with room: 48 KB of page text, about 21 KB for the words, what was heard, the history and the page's other fields, about 120 KB for a queue at its own bounds (51 entries at about 2.3 KB), about 24 KB for 40 candidates (about 213 KB), so every documented bound can be reached.
Unknown fields are refused (`400`) everywhere except inside `queue` and inside each `candidates` item, which are read leniently so a newer phone's extra fields do not break an older server (an unknown top-level field, or one inside `page`, `heard` or `history`, is still `400`).

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
  "heard": {"block_index": 2, "text": "the last words spoken"},
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
| `heard` | optional; `block_index` at least 0; `text` at most 2000 characters (more is `400`, never cut); send the last few sentences spoken |
| `programme` | optional; at most 128 bytes; recorded and shown to the model |
| `history` | optional; at most 8 lines, each `listener` or `radio`, 1 to 1000 bytes; the last few turns, oldest first |
| `queue` | optional; the phone's play queue, see "The queue" below; `null` is the same as absent; a malformed one is `400`, a well formed one over a bound is answered without it |
| `candidates` | optional; at most 40 `{route, title}` (`400` over, a name sent, never cut): the paper's stories the desk may add to the queue. Route has no space, `@` or `#`; at most 256 bytes; title at most 300. Unknown fields in an item are ignored |

Refusals are plain JSON `{"error": "..."}` before any stream starts: `401` no
owner, `405` not POST, `400` bad body or fields, `409` the same `turn_id` is
being answered right now, or was used before for different words (use a new id), `413` body or `page.text` too large, `503` the computer
has no streaming model bound. Once the stream has begun the status is `200` and
a failure is an `error` event.

## The paper is searched for the words (lane `ask-search`, not deployed)

Before the one model call the desk asks the paper for the listener's words,
unless the words are a bare transport command ("pause", "skip this", "next
please"). Nothing changes in the request or the response: the phone sends what
it always sent. Code: `internal/agentcore/api_autoradio_paper.go`; the platform
read: `GET /internal/platform/published/ask?q=&limit=` (`internal/platform/published_ask.go`).

- **What leaves the computer:** the words, one line, cut to 300 bytes, as the
  query of one public GET. No body, no owner header, no page text, queue,
  history or turn id. What returns is published stories, which any reader can
  open.
- **One call, both halves:** BM25 over published Autopaper stories and the
  nearest story passages by vector (the platform embeds the words), up to 15 of
  each, in one answer; each row carries its rank in each half, its update time,
  importance (the front list's stored 1 to 10, if scored), distinct source
  count, section, lede and route. If the vector half cannot run the keyword
  rows are still answered and the answer says why.
- **Rank** (`paperScore`, one place, no settings): relevance (reciprocal rank
  fusion, k 60) x recency (half life 72 h, floor 0.3) x importance (0.6 to 1,
  unscored 5 of 10) x citations (0.7 to 1, log of sources, saturating at 12).
- **What the model sees:** a fenced block `<<<AR... from the paper>>>` ... `<<<AR...
  end from the paper>>>` in the volatile part, before the queue block: the top
  10 stories, each one line (route, title, section, age, sources) and its lede,
  every field one fence-safe line, the block marked untrusted. Their routes are
  added to the candidates the model may name in a queue op, validated as any
  candidate.
- **Three cases, told apart to the model:** the search ran and found stories
  (the block lists them); ran and found none (the block says the paper holds
  nothing on it, or, when the keyword index was still being built or the vector
  half did not answer or ended partway, that the search may be incomplete); did not run (a refusal, a bad
  answer or the desk's 1.5 s wait; the platform's own deadline is 1.2 s, so a slow index or vector half comes back as capped or partial first: the block says so and the model is told to say
  it could not search). No block at all means no search was made for the turn
  (a bare command, or a computer with no paper to search). A vector half that
  did not run, or ended partway, is named in the block.
- **Failure:** logs one line (`paper search failed`) and the turn goes on.
- **Log line per turn:** `autoradio: turn=<id> paper found=N given=M
  search_ms=T named=K vector="answered" capped=false`; `named` counts queue
  ops that named a searched story. The same facts ride in the `done` event as
  `paper: {state, found, given, search_ms, vector, capped, named}` (absent
  when no search was made), so a deployed proof can read them.
- **Cost:** no second answering call (the one new model call is the platform's query embedding); one request before the call (measured on the
  platform's search path, 100 to 250 ms; not yet measured end to end on
  staging) and about 1.5 to 3 k more input tokens.

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
(`Dr.`, `U.S.`, `3.5`); a run of 4000 characters with no terminator at all is cut at a pause (a transport safeguard far above any real sentence, not a steer on length).

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
  "usage": {"input_tokens": 812, "output_tokens": 61, "cached_input_tokens": 700, "cache_reported": true},
  "applied": {"reasoning": "off", "cache_break": "accepted"},
  "in_force": {"reasoning": "off", "max_tokens": 0},
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

### `queue_ops`

Sent at most once, after the last `speech` and before `done`, and only when the
request carried a `queue` and at least one op survived validation, or when a
replayed turn had sent ops (its `ops` may then be empty, see "The queue"). See
"The queue" below for the shape and the rule the phone applies.

### `handoff`

Reserved for slice 2 (design section 4). Slice 1 never sends it; ignore unknown
event names.

`applied` is what the gateway said it applied for the answering member (header
`X-Choir-Applied`): `reasoning` `off` or `on`, `cache_break` `accepted` (Bedrock),
`ignored` (River, which caches unchanged prefixes by itself) or `none`. An older
gateway sends no header and both read `unknown`; `unknown` or `on` for reasoning
means the flag did not take effect. **Deploy the gateway before the guests that
call this route.** `in_force` is what the computer's policy gave the call.

## The queue (2026-11-15)

Design: [autoradio-queue-desk-2026-11-15.md](../design/autoradio-queue-desk-2026-11-15.md)
(sections 3 and 4 are the contract). Code: `internal/autoradioqueue` (the
bounds and the rules), `internal/agentcore/api_autoradio.go` (the route).

The phone's play queue, sent with a turn so the DJ can answer about it and
change its order. An item is flat with a `kind` tag: `story` (`route`, `title`),
`page` (`url`, `title`), `document` (`id`, `title`, `at`) or `audio` (`url`,
`title`, `show`, `start`, `end`, `episode_id`).

```json
"queue": {
  "version": 7,
  "current": {"id": "q12", "item": {"kind": "story", "route": "front", "title": "A strike ended in Nancy"}, "position_seconds": 41.2},
  "upcoming": [{"id": "q13", "item": {"kind": "audio", "url": "https://example.test/a.mp3", "title": "Harbour talks", "show": "Up First"}, "added_by": "dj"}]
}
```

Bounds. Malformed is `400`, naming the field: `version` a whole number, not
negative; an entry `id` is `[A-Za-z0-9][A-Za-z0-9_.-]{0,63}` and unique across
`current` and `upcoming`; `kind` is one of the four, with the field its kind
needs (`route`, `url`, `id`, `url`); `position_seconds` between 0 and 10,000,000.
Over a size bound is not a refusal: a listener's words are never refused because
their queue is long. The turn is answered without the queue (the model is not
shown it, no `queue_ops` is sent, `done` has no `queue_ops`) and, at the end of
the answer, the desk says "I couldn't see your queue this time, it is too long
for me." (the request record carries `queue_unused`). The size bounds: at most 50
`upcoming`; a title 300 bytes, `url` 1024, `route` 256, `show` 200, a document
or episode id 128, `added_by` 32. The body bound (`413`) is the only other way a
queue is refused, and a queue of ordinary text within these bounds fits it (the
bounds count decoded bytes; a body whose strings are mostly JSON escapes can be
larger than the same text and reach the `413`). **The phone sends at most 50
`upcoming` entries**: the first 50, in order. Unknown fields inside `queue` and
its items are ignored.

The model is shown the queue within a byte budget (12 KB of entry lines, 8 KB of
candidate lines); past it the farthest entries appear as "and N more entries, not
shown". The desk's copy holds them all, so an op may name one the model was not
shown.

When `queue` is present the desk may answer in the same single model call that
writes the spoken reply. The queue block of the request gives the model the rules
and the exact fence line for this request, `<<<AR... queue>>>` with the request's
own token (the one on the request's other fences); the model ends its reply with
that line and a JSON array of ops. Only that exact line starts the ops, and only
when it starts a line (after a newline or the stream's start, spaces and tabs
aside) and a `[` (or `{`, a malformed array that is dropped and said, never read
aloud, or a code fence around the array) is the next thing after it (blank space
aside). Text that is not the ops fence is speech, with no failure line (a quote,
a page fence, another token's fence, an end fence, the exact line echoed in a
sentence or followed by words), EXCEPT that on a queue turn a sentence that
carries this request's fence token, or an ops object (it contains `{"op":` and
`"reason":`, or begins with an array of ops, spaces aside), is dropped by a last
guard and logged. The cost: a reply made only of such sentences yields no speech
and walks to the next pool member. Text after the closing `]` is ignored
(logged); the array's ops stand. A turn with no
`queue` sends the prompt and streams exactly as it did before the queue existed:
no cut, no queue text, no `queue_ops` in `done`. A reply that is only the fence
and ops is an answer (no walk to another pool member): the desk speaks "Okay."
and the first op's reason. The desk validates every op against
the queue as the earlier ops left it, then sends:

```
event: queue_ops
data: {"turn_id":"cmd-7f3a9c","base_version":7,"replayed":false,"ops":[
  {"op":"move","id":"q14","to_index":0,"base_version":7,"reason":"you asked for the long read next"}]}
```

Ops: `play_now`, `play_next`, `enqueue` (with `item`, always a `story` named in
`candidates`, titled by the candidate), `remove`, `move` (`id`, `to_index` into
`upcoming`), `skip` (`id` is the entry playing), `previous`, `clear`. Every op
has `base_version` (the snapshot's `version`) and a one-line `reason` of at most
160 bytes. Refused and dropped, not sent: an id not in the queue, a move or
remove of the entry playing, a move to the entry's own place (a move past an end
is clamped), a skip, previous or clear with nothing to act on, an add that is
not a named story or is already queued, an op without a reason, malformed JSON,
and any op past the eighth in a turn (a bound on what a stranger's page text
can steer, not a model-call limit). If the model asked for a change and none
survives, the desk speaks one more sentence ("Sorry, I couldn't change the
queue.") and sends no `queue_ops`.

The phone's rule for ops against a changed queue: an op that names an entry
(`move`, `remove`) applies if that entry still exists, whatever the version, and
`skip` only if the entry is still the one playing; an op that adds an item
always applies; `clear` and `previous` apply only if the phone's version still
equals `base_version`. A dropped op is not an error. The phone applies each
`turn_id`'s ops once. A replay of a turn that sent ops sends `queue_ops` with
`replayed: true`, and its `ops` are the recorded ones only when the retry's
`queue.version` equals the recorded `base_version`; a retry with another version,
no queue, or a queue over a bound gets the event with an empty `ops` list.

The desk keeps no copy of the queue: the next request's snapshot is the truth.
The request record carries `queue_version`, `queue_sha256` and `queue_entries`;
the Report carries `queue_ops` (JSON) and `queue_base_version`. `done` has
`queue_ops` (the number of ops sent) when the request carried a usable queue, and
no such key otherwise. Not on the programme route tonight.

## Idempotency and disconnects

A `turn_id` already answered `completed` (at least one sentence) is replayed:
the recorded sentences as `speech` events and a `done` with `"replayed": true`,
no model call. A failed or interrupted turn, and a turn that produced nothing
speakable, is not a recorded answer: the same `turn_id` with the same words is
answered anew. The same `turn_id` with different words is `409`, whether the check finds the
request record or only the Report (both carry the words' hash). A post of a
`turn_id` that is still being answered is `409` too, including for a moment after
the client dropped and the cancelled upstream call unwinds: retry after a short
wait. `page.text_sha256`, if sent, must equal the SHA-256 of `page.text` (else
`400`); the tape records the server's own hash.

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
streamed and has no tools.

### Reasoning, the token budget and the cache (owner 2026-11-11)

Owner: "for auto radio, we may want to have reasoning off, at least for super
fast responses, but at the same time, we definitely want reasoning on for
scripting going forward ... Also to be cache aware, because I don't know if
every model allows you to switch reasoning without breaking the cache." And: "I
don't want a token budget either ... We should be able to steer models with
prompts, not deterministic rules."

- **Reasoning is a property of the role, fixed per role, never toggled within a
  prompt lineage.** The fast turn role `autoradio` runs with reasoning off
  (`modelpolicy.ReasoningOff`, the value `off`). The future script role
  (`autoradio_programme`, design section 11) will run with reasoning on, on its
  own prompt prefix, so each role has its own stable cache and no call flips
  reasoning on a prefix another call cached.
- **The exemption is explicit and narrow.** The 2026-11-09 rule (thinking is
  never off) and its test stay in force for every other role. `off` is accepted
  in a policy file for the `autoradio` role only (any other role naming it gets
  a diagnostic and the default effort), `PoolChain` and the platform fallback
  give it to that role, `Selections()` leaves that role out of every other
  role's continuation chain, and only `gatewayruntime.StreamText` can send the
  request flag `reasoning_off` (`CallWithTools` has no such field). Only River
  and Bedrock Haiku 5.5 honor the flag; every other adapter ignores it and keeps
  its floor. Tests: `internal/modelpolicy/autoradio_role_test.go`,
  `effort_low_test.go` (names the exemption), `internal/gatewayruntime/
  text_stream_test.go`, `internal/gateway/autoradio_river_stream_test.go`.
- **No token budget of the desk's.** The call sends `max_tokens` 0: River's field
  is omitted; Bedrock's is the model's own ceiling from the catalog
  (`provider.defaultMaxTokens`), which is the provider's limit, not a steer. A
  `max_tokens` in the owner's policy file is honored (the owner's own setting).
  The length and shape of the answer are steered by the desk's prompt
  (`internal/appdesk/firstparty_prompts/autoradio.md`: a few short spoken
  sentences, the most important first, then stop, with the reason given).

A policy section `[roles.autoradio]` overrides only what it names: the compiled
role values (pool `autoradio`, reasoning off, no `max_tokens`) are the base, and
the section does not inherit the defaults' effort or `max_tokens`. `reasoning =
"off"` and `"none"` both mean off for this role; any other value is the owner's
and is honored (`done.in_force` says what was in force). The gateway honors
`reasoning_off` only for a no-tools call to a member of the compiled autoradio
pool and ignores it (counted, logged) otherwise.
`/api/model-policy/resolve?role=autoradio` shows this route.

### Per provider (vendor documentation read 2026-11-11; each point marked)

| | River DeepSeek V4.1 Flash 1017 | Bedrock Claude Haiku 5.5 |
|---|---|---|
| Can reasoning be off? | **Yes, River documents it.** "For both DeepSeek models ... reasoning is off by default. Chat Completions accepts `reasoning_effort` with `none` (off), `low`, `high`, and `max`" (docs.river.ai/guides/inference-controls). The `low` floor was **ours** (the 2026-11-09 rule; `riverReasoningEffort` maps off spellings to `low`), not River's. The autoradio call sends `none`. | **Yes.** "Claude Haiku 5.5 also has thinking on by default and accepts `thinking: {type: "disabled"}` at effort `high` or below" (platform.claude.com thinking page; the page lists Amazon Bedrock InvokeModel as a supported platform). Lowest otherwise: effort `low` ("minimizes thinking; skips thinking for simple tasks"). The call sends `thinking: {type: "disabled"}` and `output_config.effort: "low"`. Telling Haiku 5.5 in the prompt to answer directly "didn't stop it from thinking" (same source). |
| Does changing the setting between calls that share a prefix invalidate the cache? | **Not documented.** The River pages say only "Unchanged prefixes can reuse the cache when available" and "There is no separate cache-write fee." relayed-unverified: assume a change could miss the cache; the design never changes it within a role. | **Yes, for the messages, always:** "changing [thinking parameters] always invalidates message blocks; tool and system caches are also invalidated on models that render the configuration ahead of them" (model-specific, not stated for Haiku 5.5); the same holds for `output_config.effort` ("Setting effort explicitly to the model's default [medium on Haiku 5.5] is equivalent to omitting it and does not invalidate"). Our own comment records that `disabled` drops the cache relative to adaptive. Per-message effort (beta) keeps the cache but is not used here. |
| Reasoning tokens: streamed and billed | Not documented for DeepSeek on River (River documents `reasoning_content` for GLM only). Our reader treats `reasoning_content` as hidden state and never speaks it. Billing: not stated for DeepSeek; GLM's "count toward the output budget and price". relayed-unverified. | Billed as output tokens, counted in `max_tokens`; `usage.output_tokens_details.thinking_tokens` has the breakdown on the final `message_delta` (vendor docs). With thinking disabled there are none. |
| Cache mechanism | Automatic prefix reuse, no markers; the frame puts the stable part first. | Explicit breakpoints only: the system block (1 hour) and the page block (5 minutes, `cache_break`), none on the volatile block. A prompt below the model's minimum cacheable length reports no cache (the minimum for Haiku 5.5 is not verified). |
| First-sentence cost that remains | Reasoning off removes thinking latency. Remaining: network, River time to first token (not measured). | Not streamed by our provider: the whole call completes before the first sentence is sent, so as the fallback it costs the full generation time (short answer, thinking off). |

### Cache-aware frame

The user message is two content blocks. The stable block comes first and is
byte-identical for the same page: the page between fence lines whose token is
derived from the page's own content hash (a page cannot name its own hash, so
it cannot forge a fence, and the same page keeps the same prefix). The volatile
block follows: programme, where the listener was, the last turns, the listener's
words. The system prompt is the third stable part. A test asserts the system
prompt, cache key and stable block are byte-identical across two turns about one
page. `done.usage` carries `input_tokens`, `cached_input_tokens` and
`cache_reported`, and `scripts/prove-autoradio-turns` prints the cached share per
ask and asks two questions per page so the second can hit.

A stream that ends with nothing speakable is treated as a refusal and walks to the
next member.

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

## Deterministic shaping rules in this slice, and what became of them

Owner 2026-11-11: no hard time budgets, no token budgets; steer with prompts.

| Rule | Status |
|---|---|
| First-chunk walk 6 s, stall timer, 30 s whole-turn deadline | removed |
| `max_tokens` 220, then 1200 | removed; none sent (above) |
| Reasoning clamped to low | removed; the role's setting is passed as policy gives it |
| Forced sentence cut at 200 characters | kept only as a transport safeguard at 4000 (`speechsplit.MaxRunes`) |
| `heard.text` silently cut to its last 300 characters | removed; over 2000 characters is `400` |
| History at most 8 lines of 1000 bytes | kept as a request bound (rejects, never truncates); named here for the owner: it does shape what the model is given, by refusal, so it is a stranger bound only if the app sends a few lines |
| Markdown marks stripped from a sentence before it is sent | kept: a transport transform for the ear (`**`, backticks, a leading bullet), it never changes the words |
| Fixed spoken fallback lines on failure | kept: they are not a model output |
| Body 80 KB, page 48 KB, words 2000 bytes, field lengths | kept: bounds on what a stranger can cause |

## Residual risks

- No per-owner concurrency bound on turns (by ruling: no limits on our own model
  calls). An authenticated caller can run many distinct `turn_id`s at once, each
  with a gateway stream and two durable records; failed and interrupted Reports
  get a new id per attempt (`autoradio-turn-fail:<id>:<status>:<ns>`), so tape
  growth is not bounded by the request body alone. Watch the log volume line.
- Bedrock charges a call's `max_tokens` (here the model ceiling) against its
  token quota when the call starts, so concurrent fallback turns can be
  throttled earlier than their output would suggest. It is the last member, so a
  refusal there ends the turn. Watch it in the staging proof.
- The request record is addressed to `app:autoradio:<owner>`, the same agent id
  as the Report; if that desk ever ran, the unresolved notice would appear in its
  pack. It has no inbox and no accepts today.

## Measuring it

`scripts/prove-autoradio-turns --host choir.news --api-key-file <file>
[--computer-id <id>] [--count 10]` sends N asks, two per page, prints per ask the time to
first `speech` event and to `done` (client clock) beside the server's own hop
timings, and the median; it exits non-zero when the median to the first
sentence is above 2.5 s, when any ask failed, when the gateway did not confirm
reasoning off on every ask, or when River never answered. It has no socket
timeout by default (`--timeout` sets one): a slow tail is reported in the
distribution.

## Surprises found building it

- The gateway reports an upstream failure after it has begun streaming as an
  untyped `data: {"error":"..."}` line (`handleStreamingInference`), not a
  `type: "error"` chunk. `parseGatewaySSE` (the desk loop's streaming reader)
  skips it and returns a clean end; `StreamText` reads it as an error and also
  requires the final `[DONE]` marker.
- Bedrock Haiku 5.5 is not streamed by the gateway provider (one whole call, one
  chunk). Its pool position makes that acceptable; it is not a streaming test.
- A stream can end cleanly with nothing to say when thinking spends the whole
  `max_tokens`. Treated as a refusal; never recorded as a completed answer. With
  no token budget of the desk's and reasoning off it should not happen.
- A provider quota before the first chunk comes back as an HTTP error before
  any SSE (the typed `provider_quota` answer), which `StreamText` returns as
  `provideriface.ProviderQuotaError` before any delta.
- A voice turn written as a Cast request is counted by the inert and
  calibration scans; the request record is a notice for that reason.
- The package desk `app:autoradio` is a registry profile once registered, but
  a policy role must be known before the desk is, so the policy role is the
  bare `autoradio`.
