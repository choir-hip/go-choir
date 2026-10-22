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

## Review findings, bunch 109 (2026-10-22, candidate 2e9ebc5eb)

Sonnet 5.5, DeepSeek v4.1 flash and Gemini 3.8 accepted the bunch 88
repair. FP0 checked the follow-ups below against the code; they are
recorded here before any fix.

- **F-A (Sonnet), medium-high: one guest can fill `in_flight` for
  nothing.** `decodeBodyHeld` reserves a body's declared `Content-Length`
  (or the route's cap when chunked) before a byte arrives, and the budget
  is shared by every caller. With the scheduler off and the model routes in
  observe, as at the first deploy, two POSTs declaring 576 MiB and sending
  nothing fill the 1 GiB budget; every other tenant's body, the paper's
  included, waits behind them oldest first and is answered 429 at its 60 s
  deadline. The guest repeats every minute; the rate limiter counts
  requests, not bytes held. Before the budget the guest had to send the
  bytes; now holding the gateway costs it no bandwidth. This is the threat
  this record names, and no test covers it.
- **F1 (DeepSeek), high for the claim that one guest cannot take the
  gateway down: decoded pixels are not in `in_flight`.**
  `provider.NormalizeImage` (`internal/provider/media.go`) decodes an image
  of up to `MaxImagePixels` (50 Mpx) and `downscale` then copies it whole
  into a full-size RGBA image before averaging. One 50 Mpx JPEG is about
  75-150 MB decoded plus 200 MB copied; a 16-bit PNG is 400 MB decoded plus
  the copy. Nothing bounds how many run at once: with the scheduler off the
  60-per-minute window admits 60 single-image calls (about 400 MiB of
  bodies, well inside `in_flight`), about 18 GB of live pixels, over the
  unit's `MemoryMax=8G`; with the scheduler on, 24 calls at 300-400 MB are
  7-10 GB. An OOM kill takes every tenant's calls down, and the nix
  comment's arithmetic ("plus one decoded image per call, about 200 MB")
  leaves out both the concurrency and the second full-size buffer.
- **F-B (Sonnet) and Gemini F1, low: a panic in the decode leaks the
  reservation.** If `dec.Decode` (or a reader) panics, `decodeBodyHeld`
  has not returned its release, so the handler's `defer releaseBody()` is
  never registered; net/http recovers the panic and the bytes stay counted
  for the life of the process. `in_flight_bytes` drifts up until every
  body is 429.
- **F-C (Sonnet), low: the budget wait and the read share one window.** A
  legitimate 150 MiB body that waited 55 s for room has 5 s left to
  arrive and is cut with a 503 (transient, retried).
- **F-D (Sonnet), info:** the budget counts wire bytes, not heap; image
  decode is outside it (the same root as DeepSeek F1).
- **F2 (DeepSeek), medium:** the 160 MiB enforce limit covers 20 images
  already at the 5 MiB downscale bound, not 20 guest originals of up to
  20 MiB (`MaxImageInputBytes`), which are a non-transient 413 in enforce.
- **F3 (DeepSeek), low:** the `SetReadDeadline` error is ignored; a future
  `ResponseWriter` wrapper without `Unwrap` would silently bring back the
  bunch 88 F2 failure (a queued call's read hits the server's deadline).
- **F4 (DeepSeek), low:** strict FIFO makes a small body that would fit
  wait behind a large waiter.
- **F5 (DeepSeek), low:** `in_flight` binds in body caps observe mode, so
  observe is not a pure pass-through; the station text should say so.

## Repair of the review findings (bunch 109)

Tests 007c9ee0d (yellow, each failing on 2e9ebc5eb), code 9f9e1e658 (red),
nix comments e07fdf76d (green).

- **F-A, fixed: a per-caller share of `in_flight`.** `bodyBudget` counts
  each caller's held bytes, keyed by the `caller` every handler already
  passes to `decodeBody`: the computer's gateway identity from its
  authenticated bearer (`authenticateBearer`), or `host` / the host
  caller's label on the operator and host-judgment routes. A body is taken
  in when its caller holds nothing yet, or when its caller's held bytes
  plus this body stay within `limits.per_caller`; otherwise it waits, and
  the grant loop passes over it (and that caller's later waiters, which
  keep their order) instead of stopping at it. Oldest-first still holds
  among callers within their share, and the bound, the 429 with
  `Retry-After: 5` at the body deadline, `refused.in_flight` and the clamp
  of one body to `in_flight` are unchanged. **The share chosen:**
  `per_caller` defaults to `in_flight / 4` (256 MiB at the default 1 GiB),
  never above `in_flight`, settable as `GATEWAY_BODY_LIMITS per_caller=`
  and shown in `/health` `body_caps.limits.per_caller`. Why a quarter: it
  is above the 160 MiB enforced model limit, so one computer can still
  hold one full enforce body plus its small search and source calls; and
  one computer then holds at most max(per_caller, one first body):
  256 MiB in enforce, leaving 768 MiB, or one 576 MiB ceiling body in
  observe, leaving 448 MiB, which still takes two full enforce bodies and
  every small route. The first-body exception is what keeps one body of
  any admitted size from deadlocking when it is larger than the share.
  The stalled guest of the finding now holds 576 MiB at most (one declared
  ceiling body) instead of the whole budget, and its second body is a 429
  of its own. The test stalls two 200 KiB declared bodies from one caller
  with `in_flight=512KiB` (share 128 KiB) and checks that another caller's
  200 KiB body is served at once and the second stalled body is 429.
  Residual: the share is per computer, not per owner; an owner with
  several computers has several shares (with two in observe, about the
  whole budget). Per-owner accounting needs the owner in the gateway's
  identity, which it does not have.
- **DeepSeek F1, fixed: a process-wide decode bound of 4.**
  `provider.decodeAndDownscale` runs `image.Decode` and `downscale` inside
  one of `MaxConcurrentImageDecodes` = 4 slots, so the full-size buffers
  exist for at most four images at once; a call over the bound waits for a
  slot (no new refusal). `MaxImagePixels` stays 50 Mpx: the 1568 px target
  bounds the output, not the input, and 48-50 Mpx phone photos are real
  inputs the gateway exists to downscale; lowering it would turn them into
  non-transient 400s. The arithmetic: one 50 Mpx image is at most 400 MB
  decoded (16-bit RGBA PNG, 8 bytes a pixel; an 8-bit PNG or CMYK JPEG is
  200 MB, a 4:2:0 JPEG 75 MB) plus the 200 MB RGBA copy `downscale` makes,
  plus its 20 MiB input: about 620 MB. Four are about 2.5 GB (2.3 GiB).
  The unit's worst case is then about 3 GiB of bodies (1 GiB `in_flight`
  with the decoder's buffer and the decoded copy) + 2.3 GiB of pixels +
  about 0.5 GiB of runtime and the rest = about 5.8 GiB: under
  `GOMEMLIMIT=6GiB` and 2 GiB under `MemoryMax=8G`, which therefore stays.
  Eight slots would be about 4.6 GiB of pixels and 8.1 GiB in all, over
  the kill, so four. The nix comment now carries this arithmetic with the
  concurrency multiplier and the second full-size buffer. Residuals: the
  slots are not fair across callers (one guest's 20-image calls of tiny
  50 Mpx pixel-bomb PNGs can keep every slot busy, delaying other
  tenants' image calls, never their text calls, by the decode time of the
  queue ahead); a slot wait does not watch the call's context
  (`prepareMediaRequest` takes none); the full-size RGBA copy itself could
  be removed by reading the decoded image's own pixel layout.
- **F-B (and Gemini F1), fixed.** `decodeBodyHeld` gives the bytes back on
  every exit but success from a defer registered right after the grant, so
  a panic in the decoder or a reader releases them before net/http
  recovers it. The test panics in an `UnmarshalJSON` on both
  `decodeBodyHeld` and `decodeBody` and checks the budget holds 0.
- **F-C, fixed (trivial).** The read deadline is set when the body fits,
  to now + `GATEWAY_BODY_READ_TIMEOUT`; the wait for room keeps its own
  deadline of the same length from `decodeBody`. A connection is then held
  at most twice the timeout (120 s by default: the server's own read
  timeout), and a trickle is still cut 60 s after its read starts. The
  test waits half the deadline for room, then sends the body after the
  wait's deadline and inside the read's, and gets 200.
- **F-D**: the same root as DeepSeek F1, answered there.
- **DeepSeek F2, declined (a recorded decision, as the bunch 88 F3
  paragraph says).** 160 MiB is the enforce limit for bodies whose images
  are already within the gateway's own output bound; guest originals
  between 5 and 20 MiB are the observe window's question, answered by
  `max_seen.model` and `would_refuse.model` before the flip. Raising the
  limit to 320 MiB now would loosen enforce without a measurement.
- **DeepSeek F3, fixed (trivial).** A `SetReadDeadline` error is logged
  once per process (`body read deadline not set`), so a future wrapper
  without `Unwrap` shows in the journal. No production-chain test: the
  routes are registered on the mux unwrapped (`internal/server/server.go`),
  and the bunch 88 F2 test runs through a real listener.
- **DeepSeek F4, declined.** Strict FIFO among callers within their share
  is the stated design (a stream of small bodies must not starve a large
  one); a small body behind a large waiter waits at most the deadline and
  gets a transient 429. The share now lets every caller pass a caller over
  its share, which removes the hostile case of this finding.
- **DeepSeek F5, fixed in wording.** The station's section 6 now says
  observe applies to the model limit only; `in_flight`, the share and the
  body deadlines bind in every mode.

## Review findings, bunch 116 (2026-10-22, candidate 982a70f3a)

Sonnet 5.5 accepted the bunch 109 repair with one required fix; Gemini 3.8
approved, naming two residuals already recorded (the share is per
computer, the decode slots are not fair across callers). FP0 checked the
findings below against the code; they are recorded here before any fix.

- **F1 (Sonnet), medium, required: the decoded base64 is outside the
  stated worst case.** `prepareImageSource` (`internal/provider/media.go`)
  decodes an image's base64 into `raw`, up to `MaxImageInputBytes`
  (20 MiB), before `decodeAndDownscale` takes a decode slot, so every call
  queued for a slot holds its `raw` outside both the "4 x 620 MB" pixel
  term and the 3 GiB body term: about 0.5 GiB with the scheduler on
  (24 calls), about 0.75 GiB with it off (about 38 calls of 27 MB bodies
  inside the 1 GiB `in_flight`). The worst case is then about
  6.3-6.6 GiB, over `GOMEMLIMIT=6GiB` (the collector runs flat out) though
  under `MemoryMax=8G`; the "about 5.8 GiB" in the nix comment, this record
  and the station text is not the worst case. FP0 found the same term in
  `SummarizeMedia`, which the inference handler calls for its log line
  before the provider call: it decodes each base64 image whole to hash it,
  one 20 MiB `raw` per call at a time, outside any slot.
- **F2 (Sonnet; Gemini residual 2), medium: the decode-slot wait ignores
  the call's context and is not fair across callers.** The wait
  (`imageDecodeSlots <- struct{}{}`) runs while the call holds a scheduler
  slot and its body bytes; a client that goes away or a call past its
  deadline stays queued. One computer can keep 6 calls x 20 tiny-on-the-wire
  50 Mpx 16-bit PNGs (a flat-colour PNG is a few KB) queued, about 120
  decodes at 1-3 s each over 4 slots, so every other tenant's image calls
  wait behind that FIFO for tens of seconds, on CPU the guest paid nothing
  for.
- **F3 (Sonnet), low: a caller over its own share is counted as global
  exhaustion.** The 429 at the body deadline is counted and logged as
  `refused.in_flight` (`limit=in_flight`) whether the budget was full or
  the waiter was passed over because its own caller held its share; proof
  step 10 and an operator cannot tell a noisy tenant from a full budget.
- **F4 (Sonnet), low:** `grantLocked` does O(waiters) work under `b.mu` on
  every acquire and release; bounded by the scheduler (24) when it is on,
  only by connections for 60 s when it is off.
- **F5 (Sonnet), low:** the share counts every route against one number,
  so a computer holding about 256 MiB of model bodies sees its own small
  search, jev and source bodies wait and answer 429. Self-inflicted and
  bounded.
- **F6 (Sonnet), info:** a client that disconnects during the `in_flight`
  wait is not noticed until the deadline, because net/http starts no
  background read before the body is consumed; pre-existing.
- **F7 (FP0, found while checking F1), medium for the arithmetic: each
  call's downscaled images live outside both bounds for the whole call.**
  `prepareMediaRequest` returns a new request whose downscaled images are
  fresh base64 strings (up to `MaxImageBytes` 5 MiB, 6.7 MiB as base64,
  each), while the gateway's decoded request, counted in `in_flight`, keeps
  the originals; the adapter's serialization of the request to the
  upstream is another copy for the call's life. A downscaled PNG can be
  several times its JPEG input, so a call of 20 compact large-dimension
  JPEGs (about 27 MB on the wire) can hold up to about 133 MiB of
  downscaled base64 plus its upstream body. With the scheduler on (24
  calls, 6 per computer) that is up to about 3 GiB more, about 0.8 GiB per
  computer; with it off it is bounded only by how many such calls fit in
  `in_flight`. No figure in the current arithmetic counts it.

## Repair of the review findings (bunch 116)

Tests 0317092cb (yellow), code a3d04df9d (red), nix comment 2e9329f22
(green). Neither package builds on 982a70f3a with the new tests (new API);
with only the API shimmed onto 982a70f3a (checked, then discarded) each
new check fails behaviorally, as the test commit says. DeepSeek v4.1 flash
reviewed the same delta and accepted it with three low findings, folded in
below (D1 to D3).

- **F1, fixed.** `prepareImageSource` takes a decode slot before it
  decodes an image's base64, and normalizes, downscales and re-encodes it
  inside the same slot; `NormalizeImage` takes a slot itself and
  `decodeAndDownscale` no longer does, so every image (not only one it
  downscales) holds its up to 20 MiB `raw` only inside one of the 4 slots.
  The "4 x 620 MB" term already carried the 20 MiB input, so it now holds,
  and the nix comment says the input is decoded inside the slot.
  `SummarizeMedia` hashes through `base64.NewDecoder` into sha256, never
  holding the image whole (the test allocates under 1 MiB for an 8 MiB
  image; it was 8 MiB). A small image now waits for a slot too; its work
  in the slot is a base64 decode and `DecodeConfig`, milliseconds.
- **F2, fixed: the context, and a per-computer share of the slots.**
  `prepareMediaRequest` takes the provider call's context (all eight call
  sites have one). The wait selects on `ctx.Done()` and returns
  `provider.ErrImageDecodeWait` wrapping the context's error: not a
  `MediaError`, so not a 400, and `providerFailureCounts` does not count
  it against the breaker (the call never reached the upstream). The
  gateway answers it as any failed provider call (502), which the guest
  treats as transient; a client already gone reads nothing.
  `scheduleModelCall` names the computer as the call's decode caller
  (`provider.WithImageDecodeCaller`) with the scheduler on and off, keyed
  as the body share is, and a caller holds or waits on at most
  `MaxImageDecodesPerCaller` = 2 of the 4 process slots: it takes one of
  its own 2 tokens first, so its queued images wait on its own tokens and
  never stand in the process queue. The finding's hostile computer now
  delays its own images and leaves 2 slots to everyone else. A caller-less
  context (`NormalizeImage`, callers outside the gateway) takes the process
  slot only. Residual: the shares are per computer, as the body share is.
- **F3, fixed.** At the body deadline, a waiter whose caller holds bytes
  and would pass its share with this body is answered with its own reason
  ("this caller's request bodies already hold its share of the gateway;
  retry", 429 with `Retry-After`) and counted `refused.per_caller` with
  `limit=per_caller` in the log; a true full budget stays
  `refused.in_flight`. Proof step 10 prints both.
- **F4, declined.** The O(waiters) loop runs under `b.mu` over at most
  the scheduler's 24 queued calls when it is on; off, the waiters are
  bounded by the connections the 60 s deadline lets accumulate, and each
  pass is a few comparisons per waiter. Per-caller sub-queues would add a
  second structure to a red path for a cost not yet seen; revisit if
  `/health` or a profile shows `grantLocked`.
- **F5 and DeepSeek D3, recorded (by design).** The share counts every
  route against one number. A computer holding one enforce model body of
  more than 96 MiB (or any observe body over the share) is over 256 MiB
  with a second full 160 MiB body, so its second full model body and its
  own small search, jev and source bodies wait behind its first and answer
  429 at 60 s if it is still running. Before the share, two full enforce
  bodies fit. This is self-inflicted, transient and settable
  (`GATEWAY_BODY_LIMITS per_caller=`, for example 320MiB for an
  image-heavy computer, which loosens the share for everyone). Exempting
  the 1 MiB routes from the share was not taken: a stalled guest could
  then hold `in_flight` with many small declared bodies.
- **F6, recorded (pre-existing).** A client that goes away during the
  `in_flight` wait is noticed only at the deadline, because net/http
  starts no background read before the body is read; the wait is bounded
  by the 60 s deadline and the waiter holds no bytes.
- **F7 and DeepSeek D2, a named residual.** Each call's downscaled images
  (up to 20 x 6.7 MiB of base64) and the adapter's upstream serialization
  of the request live for the whole call outside `in_flight` and the
  decode slots, up to about 0.27 GiB a call. One computer can hold 6 such
  calls with the scheduler on (about 1.6 GiB, on top of its 256 MiB share
  times 3 and its 2 decode slots, about 3.6 GiB in all, under
  `MemoryMax`); several computers together, 24 calls (about 6.4 GiB) with
  the scheduler on, and more with it off, which with the bodies and the
  decodes is past `MemoryMax=8G`. The "about 5.8 GiB" is the worst case of
  the bodies and the decodes only, and the nix comment and the station text
  now say so. Candidate repairs, not taken in this round: re-encode an
  opaque downscaled image as JPEG (a few hundred KB at 1568 px, a change
  to what the upstream sees), or a process-wide byte budget for
  downscaled output that a call waits on like the slots. The observe
  deploy's `MemoryPeak` (proof 10) is the first measurement.
- **DeepSeek D1, recorded.** The round 2 receipt said each test "failed
  on 2e9ebc5eb"; the provider test did not build there (no hook or
  bound), a build failure, not a behavioral one. This round's receipt
  says which kind each failure was.
- **Gemini 3.8** approved with no required finding; its two residuals (the
  share per computer, the decode slots unfair across callers) are the
  per-owner residual above and F2.

## F7 decided (FP0, round 5, 2026-10-22, candidate 142c17bd1)

FP0's decision: fix F7 now with the conservative option, a process-wide
byte budget for downscaled image output that calls wait on, with a
per-caller share keyed as the others, the call's context honoured while
waiting (a transient answer, not a 400), release on every path including a
panic, its waits and refusals in `/health`, and the full arithmetic in the
nix comment, the station text and this record. No lossy re-encode: image
fidelity is not the gateway's to change silently. The documented unit
worst case (bodies + decode slots + downscaled output + its upstream copy
+ runtime) must stay under `GOMEMLIMIT=6GiB` and `MemoryMax=8G`.

What the sizing has to face, recorded before the fix:

- **There is no room for the output beside four decode slots.** The
  terms already stated are 3 GiB of bodies + 4 x about 620 MB = 2.31 GiB
  of decodes + 0.5 GiB of runtime = about 5.81 GiB, 0.19 GiB under
  `GOMEMLIMIT`. A downscaled image is held as a base64 string (at most
  `MaxImageBytes` 5 MiB, 6.67 MiB as base64) for the whole call and again
  in the upstream request body; with the JSON encoder's growth by doubling,
  the upstream copy can briefly be twice its size, so one output byte costs
  up to three. One full call (20 images, 133 MiB of output) is then up to
  about 0.4 GiB, over the 0.19 GiB left. The output budget has to take its
  room from another term.
- **What the 3 GiB of bodies covers.** net/http reads a body through a
  4 KiB buffer, so the body's copies are the JSON decoder's buffer (up to
  about twice the body, by doubling, during the decode only), the decoded
  request (held for the call) and the upstream request body (during the
  upstream call only). A body is in its decode or in its upstream call,
  never both, so each held body has at most about three copies live:
  3 x `in_flight`. The upstream copy of a body's own bytes is therefore
  inside the 3 GiB (DeepSeek D2), and only the downscaled output's copies
  are outside it.
- **No hold-and-wait.** The output a call will make is known only after
  each image is decoded, and a call that held output for its first images
  while it waited for more could deadlock with other calls doing the same,
  or with calls that hold a decode slot while they wait for output. A
  call must reserve once, before its first decode slot, for its worst case
  (its base64 images x 6.67 MiB), and give back what it did not use after
  the last image.

## Repair of F7 (FP0 round 5)

Decision and sizing e06f5364e (green), tests ae30081c0 (yellow, neither
package builds on 142c17bd1; with only the API shimmed onto it, checked
then discarded, each check fails behaviorally), code cd6c1cd6e (red), nix
comment 15e00ccad (green).

- **The output budget.** `provider.imageOutput` bounds the downscaled
  images all calls hold, as base64, at `MaxImageOutputBytes` = 192 MiB,
  and one caller's at `MaxImageOutputBytesPerCaller` = 96 MiB, keyed by
  the same decode caller (the computer) as the slots and the body share,
  with the body budget's grant rule: oldest first, a caller over its share
  passed over, its first reservation granted whatever its size (corrected
  in bunch 121: one computer holds at most max(96 MiB, one call's
  133.3 MiB)). 192 MiB
  takes one full call (20 x 6.67 MiB = 133.3 MiB) with 58 MiB to spare.
- **Reserve once, before any slot.** `prepareMediaRequest` reserves
  `ImageOutputPerImage` (6.67 MiB) per base64 image in one acquire before
  its first decode slot, then decodes the images one at a time, then cuts
  the reservation to the new base64 its request carries (0 for an image
  that passes unchanged, which keeps the original string). No call holds
  output while it waits for more, and no slot holder waits for output, so
  the budget, the slots, the body budget and the scheduler slot are taken
  in one order and never in a cycle. The cost: while a call is being
  prepared it holds its worst case, a few milliseconds per small image.
- **Held for the call.** `prepareMediaRequest` returns a release (never
  nil, once only); all eight adapter call sites defer it right after the
  call, so the bytes stay counted until the upstream call, a stream
  included, returns.
- **Context, errors and panics.** A wait for output that ends with the
  call's context returns `ErrImageDecodeWait` (now "call ended while
  waiting for image decode capacity") wrapping the context's error: not a
  `MediaError`, not counted by the breaker, the same answer as the decode
  slot wait. A grant that arrives together with the cancellation is given
  back. An error after the reservation (a refused image) and a panic in the
  decode give it back from a defer. The test hook now runs after the
  slot's release is deferred, so a panic there gives the slot back too
  (before, a panicking hook kept its slot; production has no hook).
- **`/health` `image_media`.** `decode_slots`, `decode_slots_per_caller`,
  `decode_in_use`, `decode_waited`, `decode_refused`, `output_limit`,
  `output_per_caller`, `output_in_use`, `output_peak`, `output_waited`,
  `output_refused`. "waited" counts waits that did not get their slot or
  bytes at once; "refused" counts waits that ended with the call's
  context. Proof step 10 checks the block is present.
- **The room, from the decode slots.** Four slots left 0.19 GiB under
  `GOMEMLIMIT`, less than one full call's output (about 0.4 GiB at three
  copies). `MaxConcurrentImageDecodes` is now 3 (1.73 GiB) and
  `MaxImageDecodesPerCaller` 1, so one computer's images leave two slots to
  every other computer, as the bunch 116 F2 share of 2 of 4 did. A
  computer's own image calls now decode one image at a time; one call
  already decoded its images one at a time, so only its parallel image
  calls wait on each other. The other options were kept out: shrinking
  `in_flight` reopens the per-caller share's reasoning, shrinking
  `MaxImagePixels` turns real phone photos into 400s, and removing the
  RGBA copy in `downscale` changes the downscaled bytes (a prompt-cache
  miss for every re-sent image) on a red path.
- **The full arithmetic.** Every term at its bound at once: bodies 3 x
  1 GiB = 3.00 GiB (each held body is in its decode, with the decoder's
  buffer up to twice its size, or in its call, with its decoded request and
  its upstream body; never both, so about three copies); decodes 3 x
  about 620 MB (400 MB 16-bit decode + 200 MB RGBA copy + 20 MiB input) =
  1.73 GiB; downscaled output 3 x 192 MiB (the string, its upstream copy,
  the encoder's growth by doubling) = 0.56 GiB; the runtime and everything
  else about 0.5 GiB. About 5.80 GiB: 0.2 GiB under `GOMEMLIMIT=6GiB` and
  2.20 GiB under `MemoryMax=8G` (corrected in bunch 121: systemd reads 8G
  as 8 GiB). `TestBodyCapsUnitMemoryArithmeticFitsUnderGOMEMLIMIT`
  computes the capacity terms from the constants, so a later change to any
  of them fails a test before it fails the unit; the copy factors, the
  runtime term and the 6 GiB limit are literals in it (bunch 121 D8).
- **Residuals.** The worst case is computed, not measured; the observe
  deploy's `MemoryPeak` (proof 10) is the first measurement, and the
  0.5 GiB runtime term is an estimate. The margin under `GOMEMLIMIT` is
  thin by construction: at the bound the collector works hard, which is
  what `GOMEMLIMIT` is for, and `MemoryMax` is 2.20 GiB further. The
  output budget is a second copy of the body budget's grant code (the
  provider cannot import the gateway); one shared byte-budget package is
  a clean-up for later. Shares are per computer, not per owner.

## Review findings, bunch 121 (2026-10-22, candidate f78ea9f96)

Sonnet 5.5, DeepSeek v4.1 flash and Gemini 3.8 accepted rounds 4 and 5.
DeepSeek named two required low-cost fixes and Sonnet one nit; they are
recorded here before any change.

- **D1 (DeepSeek), medium, required: the first-call-over-share rule is not
  pinned by a test.** With the production values a full call reserves
  20 x `ImageOutputPerImage` = 139,810,160 bytes, over the 96 MiB
  (100,663,296) share; only the `held > 0 &&` guard in
  `imageOutputBudget.grantLocked` admits it. Every output test reserves at
  most its share, so deleting the guard keeps them green, while in
  production every full image call would then wait until its context and
  fail.
- **D2 (DeepSeek), low, required: the record states the opposite rule.**
  This record's round 5 repair says a caller's first reservation is
  "always within its share", the station text says "96 MiB per computer",
  and the `media_budget.go` comment says the first reservation "always
  fits within its share". The rule is that a caller's first reservation is
  granted whatever its size; one computer holds at most max(96 MiB, one
  call's reservation of up to 133.3 MiB), and the 192 MiB total still
  holds.
- **D3 (DeepSeek) and Sonnet F1, low, required: the MemoryMax margin is
  wrong.** systemd reads `MemoryMax=8G` as 8 GiB, so the 5.80 GiB worst
  case is 2.20 GiB under it, not the 1.65 GiB the nix comment, the station
  text and this record state (that figure took 8G as 8 x 10^9 bytes).
- **D4 (DeepSeek), low-medium:** a call waiting for output holds its
  scheduler slot and its body bytes, and one 20-image call (133 of
  192 MiB) holds back every other full image call for its whole upstream
  call; the record notes the hold, not the blocked scheduler slots.
- **D5 (DeepSeek), low:** a capacity wait that ends with the call's
  context is answered 502, though no upstream was contacted; 503 with
  `Retry-After` would match the body caps.
- **D6 (DeepSeek), info:** the exported `NormalizeImage` uses
  `context.Background()` and no caller: uncancellable and outside the
  per-caller share; no caller outside tests today.
- **D7 (DeepSeek), info:** the round 5 receipt's rollback names only
  cd6c1cd6e; reverting it alone leaves ae30081c0's tests on a removed API,
  so neither package builds.
- **D8 (DeepSeek), info:** the arithmetic test takes the capacity terms
  from the constants but the copy factors, the 0.5 GiB runtime and the
  6 GiB `GOMEMLIMIT` are literals, so a nix `GOMEMLIMIT` change does not
  fail it.

## Repair of the review findings (bunch 121)

Findings 56f43277e (green), test d6e94f34e (yellow), comments 4a94a39e8 (green).

- **D1, pinned.** `TestImageOutputFirstFullCallOverTheShareIsGrantedAtOnce`
  builds the budget with the production sizes (192 MiB, 96 MiB share),
  checks a full call's reservation is over the share, and checks it is
  granted at once on an empty budget, that the same caller's next call
  then waits while another caller's goes ahead, and that the waiting call
  leaves with `ErrImageDecodeWait` when its context ends. It passes on
  f78ea9f96 (the rule was already the code's) and was checked to fail with
  the `held > 0` guard removed from `grantLocked`; no code changed.
- **D2, corrected.** The round 5 repair above, section 6, residual (b),
  the nix comment and the `media_budget.go` comment now say a caller's
  first reservation is granted whatever its size, so one computer holds at
  most max(96 MiB, one call's 133.3 MiB) of the 192 MiB.
- **D3 and Sonnet F1, corrected.** The worst case is 2.20 GiB under
  `MemoryMax=8G` (8 GiB) and 0.2 GiB under `GOMEMLIMIT`, in the nix
  comment, section 6 and the record.
- **D4, recorded (a cost, not a deadlock).** A call waiting for output
  holds its scheduler slot and its body bytes; while one 20-image call
  holds 133.3 of the 192 MiB, every other full image call (any computer)
  waits for it to end, holding its scheduler slot. With the scheduler on,
  one computer's waiters take at most its own 6 slots (the scheduler's
  per-computer cap); several computers' waiters together can fill the
  global 24. Text calls and calls of a few images go
  ahead (58 MiB is about 8 images). If `output_waited` in `/health`
  shows this in practice, the remedy is to reserve before the scheduler
  slot, a change of order on a red path not taken here.
- **D5, declined for now.** The 502 is the answer every failed provider
  call already gets and the guest already retries; the only waits that
  end this way are a client gone (nobody reads the answer) or the call's
  own inference deadline (a 503 would not let it retry sooner). Revisit
  with the scheduler flip if `output_refused` or `decode_refused` move.
- **D6, declined for now.** `NormalizeImage` has no caller outside the
  tests; the gateway path takes the call's context and caller through
  `prepareMediaRequest`. Taking a context in the exported function is a
  signature change with no user; recorded.
- **D7, corrected.** The round 5 receipt's rollback now names cd6c1cd6e
  and ae30081c0 together.
- **D8, recorded.** The literals are the arithmetic's assumptions, not
  capacities; tying the test to the nix `GOMEMLIMIT` would make a Go test
  read a nix file. A `GOMEMLIMIT` change in nix must be paired with the
  test by hand; the nix comment names the test.
