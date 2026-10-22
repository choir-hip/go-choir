# The Gateway Reads Guest Request Bodies With No Size Limit (B7, RES-11 precondition)

Date: 2026-10-22. Source: the FP7 audit (station
[FP7](../definitions/choir-launch-fp7-gateway-identity-2026-10-21.md), residual
4: "decoded bodies (no MaxBytesReader) sit in memory while queued, up to 256
per computer, and Acquire does not check an already-cancelled context; both
are preconditions of turning the scheduler on") and a re-read of the code at
origin/main a35b3586f. Mutation class of any fix: **red** (the gateway and
provider calls are a protected surface). Problem first; no fix in the commit
that wrote this record.

## The problem in one sentence

The gateway runs as root on `0.0.0.0:8084`, holds every provider key, and
decodes the JSON body of its two model routes and its search route with no
size limit, so one guest holding its own bearer can make the gateway buffer
as many bytes as it can send in 120 s, on as many connections as it likes,
and take every tenant's model calls down with it.

## Evidence (code, origin/main a35b3586f)

The only server-wide bound is time: `internal/server/server.go:105-107`,
`ReadHeaderTimeout` 30 s and `ReadTimeout` 120 s (the gateway's unit sets
`SERVER_WRITE_TIMEOUT=10m30s` and leaves the read timeout at its default,
`nix/node-b.nix:2178-2182`). There is no `MaxBytesReader` in front of the mux
and no `Content-Length` check. Go's `json.Decoder` buffers a whole value
before it unmarshals it, so a decode holds the wire bytes and then the decoded
strings, about twice the body.

Every request body the gateway reads, by route:

| route | read at | reachable by | bound today | over the bound |
|---|---|---|---|---|
| `POST /provider/v1/inference` | `handlers.go:413` `json.NewDecoder(r.Body)` | any guest with its bearer (auth at `:384`, then the 60/min window at `:394` only when the scheduler is off) | **none** (120 s read timeout) | buffered |
| `POST /provider/openai/v1/chat/completions` | `openai_compat.go:125` `json.NewDecoder(r.Body)` | any guest with its bearer (`authorizeOpenAICompatibleRequest`, window skipped when the scheduler is on) | **none** | buffered |
| `POST /provider/v1/search` | `handlers.go:905` `json.NewDecoder(r.Body)` | any guest with its bearer (search window first) | **none** | buffered |
| `POST /provider/v1/credentials/issue` | `handlers.go:644` | host only: `isAuthorizedCredentialCaller` (loopback + internal marker + hostauth) runs first | none | buffered |
| `POST /provider/v1/credentials/ensure` | `handlers.go:685` | host only, as above | none | buffered |
| `POST /provider/v1/search/health/reset` | `handlers.go:1110` | host only, as above | none | buffered |
| `POST /provider/v1/breakers/reset` | `handlers.go:1171` | host only, as above | none | buffered |
| `POST /provider/v1/judgments` (Jev) | `jev.go:300` `http.MaxBytesReader(..., 1 MiB)` | a guest with its bearer and a Jev pin | 1 MiB | **400**, not 413 |
| `POST /provider/v1/host/judgments` | `host_judgment.go:49` `MaxBytesReader(1 MiB)` | host only | 1 MiB | 413 (corrected after the record's first commit, which said 400) |
| `POST /provider/v1/source-api` | `source_api.go:694` `io.LimitReader(64 KiB)` | any guest with its bearer | 64 KiB (silently truncated) | 400 (the truncated JSON fails to parse) |
| `POST /provider/v1/fetch` (host fetch) | `internal/hostfetch/host.go:115` `LimitReader(2 MiB + 64 KiB)` | granted computers | 2 MiB + 64 KiB | 413 |
| `/provider/anthropic/*` (relay, dark) | `anthropic_relay.go:320` `MaxBytesReader(32 MiB)` | allowlisted computers only | 32 MiB | 413 |
| `GET /health`, `/health/{service}`, `/provider/v1/models`, `/provider/openai/v1/models`, `/provider/v1/search/health`, `/provider/v1/breakers` | no body read | | | |

Unauthenticated callers never reach a decode: every route authenticates
first, and `net/http` discards at most 256 KiB of an unread body before it
closes the connection. The attacker in this record is a guest using its own,
valid bearer, which every computer has.

The multiplier is concurrency. With the scheduler **off**, the window admits
60 model calls a minute per computer before the decode; with the 120 s read
timeout about 120 bodies can be in flight per computer. With the scheduler
**on**, the window is skipped for model calls and the decode runs *before*
`Acquire` (`handlers.go:413` then `:455`; `openai_compat.go:125` then
`:162`), so nothing bounds the number of bodies being read, and up to 256
decoded bodies per computer wait in the queue.

`ModelScheduler.Acquire` (`model_scheduler.go:178-236`) never looks at
`ctx.Err()` before it queues. With an already-cancelled context it enqueues,
`dispatchLocked` may grant at once, and the `select` at `:201` then picks
between `w.ready` and `ctx.Done()` at random: about half the time a caller
that has gone away is handed a slot and its call starts.

## The largest legitimate body each route needs

Measured from the code, not guessed:

- **Model routes (inference, OpenAI chat).** Two parts.
  - Text: the largest context window in the catalog is 1,000,000 tokens
    (`internal/modelcatalog/catalog.go:145`). At a dense 6 bytes a token and
    Go's JSON escaping (`<`, `>`, `&` become six bytes, newlines two) a full
    window is about 9 MB; tool definitions add a few hundred KB. Text needs
    about **16 MiB** with headroom.
  - Images: the guest inlines every `artifact_ref` image as base64 on every
    call (`internal/gatewayruntime/provider.go:50-79`). The gateway accepts
    up to 20 images of up to 20 MiB each before resizing
    (`internal/provider/media.go:27-29`); a capsule's visual bundle is at
    most 24 MiB in all (`internal/visualcapture/visualcapture.go:69-70`),
    32 MiB as base64.
  - So: a full context plus one full visual bundle is about **48 MiB**. The
    contract's ceiling, 20 images at 20 MiB plus a full context, is 20 ×
    26.7 MiB + 16 MiB, about **550 MiB**. Nothing in the product sends that
    today, but nothing prevents a conversation that keeps 20 uploaded photos
    (a phone photo is 3 to 8 MB, so 20 of them is 80 to 210 MB as base64).
    A limit between those two numbers could refuse a real call; a 413 is not
    transient to the guest (`gatewayHTTPStatusError.Transient`, 429 and 5xx
    only), so the turn fails.
- **Search.** `SearchRequest` is a query, a count and a depth
  (`search.go:48-61`); a guest query is a sentence. 1 MiB cannot refuse a
  real call.
- **Credential and operator routes.** One token, one address or one provider
  name: under 1 KiB. 64 KiB cannot refuse a real call.
- **Jev, host judgments, source-api, host fetch, relay.** Already bounded at
  sizes their callers were built against; only the status for "too large"
  is inconsistent (400 on Jev and source-api).

No staging measurement of real body sizes exists: the gateway logs message
and image counts per call (`handlers.go:449-450`), never the body's bytes.

## Failure modes a fix must avoid

1. A limit refuses a real model call (a long context with screenshots): the
   turn fails with a non-transient 413.
2. The limit reads the whole oversized body before refusing (the bound is
   then time, not bytes), or trusts a lying `Content-Length`.
3. The refusal is a 400 or a dropped connection, so the guest cannot tell
   "too large" from "malformed".
4. Bodies still pile up behind the scheduler: a cap per body with unbounded
   bodies in flight is still unbounded.
5. A cancelled caller is still handed a slot.
6. The limits cannot be read without SSH, or cannot be raised without a code
   change.
7. Turning the scheduler on later re-opens the hole (the scheduler path reads
   the body in a different place).

## Repair (lane/fp7-gateway-body-caps)

Tests 9d748f316 (yellow), code ee0183488 (red), nix 56a6df469 (red); the
station's section 6 has the limits, the rollout and the proof steps.
Observe first for the model routes only: a 64 MiB limit is below the media
contract's ceiling, so it could refuse a real conversation with many
images; every other route enforces at once.

## Residuals this record does not cover

- `prepareMediaRequest` decodes each image to pixels; `MaxImagePixels` is 50
  million, about 200 MB of RGBA per image, decoded one at a time. A bounded
  body can still cost the gateway that much per image.
- Upstream provider responses are read by the provider adapters, not by the
  guest; they are out of this record.
- The gateway still runs as root on `0.0.0.0` with every key in one process
  (host containment, FP3).

## Review findings, bunch 88 (2026-10-22, candidate 02010a2e0)

DeepSeek v4.1 flash rejected the candidate as deployable (core mechanism
accepted, mandatory fixes); Gemini 3.8 approved. FP0 checked the findings
below against the code; they are recorded here before any fix.

- **F1, high: the cap is per body only.** `body_caps.go:207-241` bounds one
  body; nothing bounds how many are buffered at once. The first deploy runs
  with the scheduler off (`nix/node-b.nix:546`) and the model routes in
  observe (`:559`, up to the 576 MiB ceiling), so the only bound on bodies
  in flight is the 60-per-minute window and the 120 s read timeout: about
  120 bodies per computer, each up to 576 MiB, about twice that once
  decoded. The gateway unit (`node-b.nix` around `:2131`) has no
  `MemoryMax`, so the bound is the host's RAM. One guest with its own bearer
  can still take every tenant's calls down, the threat this record names.
  Residual (a) of the station's section 6 deferred the aggregate bound to
  the scheduler, which is off for the whole observe and enforce window.
- **F2, high: a queued call has already spent its read timeout.** The model
  routes now take the scheduler slot before reading the body
  (`handlers.go:423`, `openai_compat.go:125`). The scheduler waits up to
  120 s; the server's `ReadTimeout` is also 120 s (`server.go:106`) and is
  an absolute deadline from the request's start. A call granted late reads
  its body past that deadline, the read fails with `i/o timeout`,
  `decodeBody` returns the decoder's error and the handler answers 400
  `invalid request body`. A 400 is not transient to the guest
  (`gatewayHTTPStatusError.Transient`: 429 and 5xx only), so backpressure
  that used to be a 429 with `Retry-After` becomes a failed turn.
- **F3, medium (partly true): the 64 MiB enforce limit is below the media
  contract.** The gateway accepts 20 images (`provider/media.go:27`) and
  downscales each to at most 5 MiB (`MaxImageBytes`). 20 images already at
  that bound are 100 MiB raw, about 133 MiB as base64, plus a context: a
  body the gateway's own contract treats as normal output of its own
  downscale is refused by the enforce limit, with a non-transient 413. The
  limit is not wrong by itself (the observe deploy measures before
  enforce), but it was not recorded as a decision against this arithmetic.
- **F4, medium (same root as F2): a trickled body holds a slot.** With the
  slot taken first, a guest that sends its body slowly holds a scheduler
  slot until the server's read timeout, 120 s; six such calls hold one
  computer's whole share, four guests hold all 24 slots.

The other review findings (F5 rollback wording, F6 proof step 11 never
reads a large model body under the ceiling, F7 the observe 413 names the
ceiling, /health counters across tenants, "every body through one helper"
wording) are low and are dispositioned with the fix.

## Repair of the review findings (bunch 88)

Tests a843f5e14 (yellow, each failing on 02010a2e0), code a303a223e (red),
nix 95240b65c (red).

- **F1, fixed.** `body_caps.go` keeps one process-wide byte budget,
  `limits.in_flight` (`DefaultBodyInFlight`, 1 GiB, never below the model
  ceiling). Every decoded body reserves its declared `Content-Length` (or
  its route's cap when chunked, shrunk to what was read once decoded); the
  model routes keep their bytes reserved until the handler returns, because
  the decoded request lives for the whole call. A body that does not fit
  waits, oldest first; at its body deadline it is answered **429 with
  `Retry-After: 5`, unread**, and counted in `refused.in_flight`. `/health`
  `body_caps` shows `limits.in_flight` and `in_flight_bytes`. This holds
  whatever the scheduler's mode. **Deviation from the brief, stated:** FP0
  asked for observe mode to log what enforce would refuse. The budget
  enforces in observe too, because the review's point was that observe with
  the scheduler off is exactly the unbounded window. It waits before it
  refuses, so a legitimate call is delayed, not failed, unless the gateway
  holds a gigabyte of bodies for a whole minute; every wait is logged
  (`body in_flight waited`). Backstop: the gateway unit now has
  `MemoryMax=8G`, `MemorySwapMax=0` and `GOMEMLIMIT=6GiB`. No measurement
  of the gateway's normal RSS exists in the repo, so 8G is deliberately
  generous (1 GiB of bodies is about 3 GiB with the JSON decoder's buffer
  and the decoded copy, plus one decoded image per call, up to about
  200 MB); no `MemoryHigh`, since throttling would stall every tenant.
  Tighten it from proof 10's `MemoryPeak`.
- **F2 and F4, fixed.** `decodeBody` sets the connection's read deadline
  to now + `GATEWAY_BODY_READ_TIMEOUT` (`DefaultBodyReadTimeout`, 60 s:
  576 MiB at 100 Mbit/s is 48 s) through `http.ResponseController`. On the
  model routes it runs right after the scheduler grants the slot, so the
  deadline is measured from acquisition: a call that queued past the
  server's 120 s `ReadTimeout` reads its body normally (the test grants the
  slot 600 ms after a 300 ms server timeout and gets 200). The wait for
  `in_flight` and the read share that deadline. A body cut by it is
  answered **503 with `Retry-After: 5`**, transient to the guest
  (`gatewayHTTPStatusError.Transient`), counted in `refused.read_deadline`,
  and its slot is freed. A trickling guest now holds a slot at most 60 s,
  not 120 s.
- **F3, decided: the enforce limit is raised from 64 MiB to 160 MiB**
  (`DefaultModelBodyLimit`, which cites this paragraph). The arithmetic: the
  gateway accepts `MaxImagesPerRequest` = 20 images and downscales each to
  `MaxImageBytes` = 5 MiB. Twenty images already at that bound are
  100 MiB, 100 × 4/3 = 133.3 MiB as base64; a full 1M-token context is
  about 16 MiB of JSON; together about 149.3 MiB, rounded up to 160 MiB. A
  body whose every image is already within the gateway's own output bound
  is a body the contract clearly allows, and 64 MiB refused it with a
  non-transient 413. Above 160 MiB (images between 5 and 20 MiB each, up to
  the 576 MiB ceiling) is still a 413 in enforce; the observe deploy's
  `max_seen.model` and `would_refuse.model` decide whether that range is
  real before the flip. Memory is bounded by `in_flight`, not by this
  limit, so raising it does not widen the aggregate bound.
- **F5, fixed in wording.** The nix comment now says rollback to observe
  is safe only while the scheduler is off (or with `in_flight` unchanged,
  which now bounds observe too); otherwise raise `model=` in enforce.
- **F6, declined for this round.** Proof step 11 still reads no large
  under-ceiling model body. With `in_flight` bounding every body, the
  memory check's bound changes with the body size; a streamed 300 MiB
  observe step needs its own expected peak. Named residual for the proof
  author.
- **F7, partly declined.** (a) The observe 413 names the ceiling because
  that is the limit it refused against; kept. (b) `/health` counters are
  aggregate per route class, never per computer, consistent with the
  scheduler's; kept. (c) The wording "every decoded body goes through one
  helper" excludes host fetch and the relay, which the station text
  already lists as keeping their own caps.
