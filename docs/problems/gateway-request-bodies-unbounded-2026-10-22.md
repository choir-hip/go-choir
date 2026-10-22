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
