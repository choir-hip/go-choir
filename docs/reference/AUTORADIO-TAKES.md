# Autoradio takes: the server, as built

Status: built on lane `takes-server`, not deployed, not proven on staging
(2026-11-18). Specification: section 11 of
`docs/design/autoradio-learned-document-2026-11-17.md`. This page records what
the code does where the design is silent or differs. Station L7 of
`docs/definitions/choir-autoradio-learning-loop-2026-11-18.md`.

## Trust assumption (known open edge)

Takes use honest reporting (owner, 2026-11-18). Each listener's computer
reports its own outcome on a take and the platform believes it. Nothing checks
a report against the listener's tape, and nothing detects many accounts
reporting together, so a publisher's record can be rigged up or down by many
accounts. Anti-fraud, anti-sybil and attestation are deferred and belong to
Autocapital. What is kept: the bounds on what one account can cause (length,
size, rate), the rule that a publisher's own listening never counts, one
outcome per listener per take (a repeat report is a replay, not a second
vote), and the marking of drill data.

## Every takes route names a computer

Every takes route except the operator yank takes a credential: a browser
session, or an API key. An API key that is not bound to one computer (an
owner-wide key, which is what a device login gives the phone) must name the
computer on every request, in the `X-Choir-Computer` header, as on every other
owner-wide route: an unnamed request is `400` ("owner-wide api key requires a
named computer"), a computer the account does not own is `403`, and a named,
owned computer is served. A key bound to a computer needs no header. This is
`takesCaller` in `internal/proxy/takes.go`; `internal/proxy/takes_test.go`
pins all three cases with an owner-wide key on every route. The route table
both sides read is `mobile/fixtures/takes/routes.json`, and a Go test asserts
each row is a registered route that accepts its own example.

## Routes (proxy, `internal/proxy/takes.go`; corpusd, `internal/platform/takes*.go`)

| Route | Who | Answer |
| --- | --- | --- |
| `POST /api/platform/takes` | signed in; an owner-wide key names its computer | multipart: `audio`, `transcript`, `duration_seconds`, optional `sha256` and `responds_to` (`{route,title}` JSON). Publisher is named from the credential only; any other field is refused |
| `GET /api/platform/takes?limit=` | signed in; an owner-wide key names its computer | newest live takes of the last 7 days (default 50, max 100), every `choir.take` field except the publisher key, plus `audio_url` and `record` |
| `GET /api/platform/takes?mine=1` | signed in; an owner-wide key names its computer | the caller's live takes with `outcomes` (counted and drill apart) |
| `GET /api/platform/takes/{id}/audio` | signed in; an owner-wide key names its computer | `audio/mp4`, `Cache-Control: private, no-store`; 404 once deleted or yanked |
| `DELETE /api/platform/takes/{id}` | the publisher; an owner-wide key names its computer | state `deleted`, audio object deleted, outcome rows stay |
| `POST /api/platform/takes/{id}/outcome` | an API key naming the listener's computer (`X-Choir-Computer` for an owner-wide key) | `{outcome: heard\|skipped, claim_id}` |
| `GET /api/platform/takes/record?publisher=voice-xxxxxx` | signed in; an owner-wide key names its computer | the publisher's record; `rows` only when the caller is that publisher |
| `POST /api/platform/operator/takes/yank` | platform operator | `{take_id}`, state `yanked` |

## Records

`choir.take` (id `take-` + 16 hex of sha256(publisher key, audio sha256), so
the same audio by the same publisher is one take), `choir.take_audio` (the
base64 bytes in their own object, so lists never read them and the artifact GC
tracks them as it does any externalized body), `choir.take_outcome` (id from
take id and hashed listener key: one row per listener per take; `drill` flag).
Owner corpusd.

## Bounds

The claim is 3 to 60 s. The header's own length (the `mvhd` box is read) may
be 2.5 to 62.0 s (`TakeHeaderMinSeconds`, `TakeHeaderMaxSeconds`) and must be
within 2 s of the claim. The header grace is for container padding and stop
latency: Apple's AAC encoder adds about 0.11 s of priming, so 60.000 s of audio
has an `mvhd` of 60.117 s, and the phone stops on a timer; an honest
full-length take is never refused for that. At most 512 KB; `ftyp` must be present; transcript 1 to 2,000
bytes; 10 takes per publisher per 24 hours (deleted ones count; drill takes do
not); at most 3 takes per programme request, 40 candidates in all; lead-in at
most 400 bytes.

## The publisher's record

From non-drill outcome rows only: offered, heard, skipped, distinct listeners,
distinct takes; `text` is the line the DJ reads ("heard through 7 of 11 times,
by 5 distinct listeners, over 3 takes", or "new: no outcomes yet"). The
publisher's full read shows every row with an opaque listener key (HMAC of the
listener's computer id under the platform signing key) and the `drill` flag.
An outcome is refused with 403 when the reporting account is the publisher.
Drill: an outcome from a computer in `test_computers` or
`operator_proof_computers` (d@e), and any outcome on a take published by such
a computer's key, is marked `drill`, shown on a separate line, never counted.
A drill take is listed only to drill computers.

## After a delete or a yank

The take row stays, with its id, handle, times, size, hash and state
(`deleted` or `yanked`); its transcript and the story it answered are blanked.
The audio object is deleted, but a body of 1 KB or more is externalised, and
those bytes stay on disk until the artifact GC pass removes them. The outcome
rows stay as counts: a yanked take keeps its prior counts in the publisher's
record (a decision for the owner, not made here).

## Reads and scans

The list reads live takes by their `state` metadata (newest first), so deleted
and yanked rows never take a place in the window; the scan is bounded at 2,000
rows and logs when it reaches the bound. A publisher's rows in the record are
the rows of takes that publisher's key published: handles are 24 bits and two
publishers can share one, and a collision never widens the read.

## Refusals that are not switches

The outcome route answers `503` when the platform has no signing key (the
listener key is an HMAC under it; there is no constant fallback) or when the
computer tier file cannot be read (a drill computer is never counted as a real
one). The proxy answers `503` for publish and list for the same tier error.
The audio route sets `X-Content-Type-Options: nosniff`. The proxy's corpusd
relay returns an error, never a truncated `200`, when a response passes its
limit.

## The settlement seam

`calibration.TakeOutcomeFromSettlement(claim, settlement)` reads a take
forecast claim (agent `autoradio-dj`, project `take`, tag `take:<id>`) and its
`autoradio.settlement` record (built by `NewAutoradioSettlementRecord`) and
returns `{TakeID, Outcome heard|skipped, ClaimID}`, or nothing for another
kind of claim, another claim's settlement, or a settlement with no verdict
(partial, unresolvable). Nothing calls it yet. When `lane/learn-settle` lands,
the settlement path of the listener's computer calls it after a settlement is
written and posts the result to the outcome route under the computer's own key.

## Where this differs from the design

- **Publishing is a direct upload**, not a fetch from the computer. Design
  11.3 has the computer hold the original and the proxy fetch it; the
  computer-side route and record (`POST /api/autoradio/takes`,
  `autoradio.take`) are not built, so the phone posts the multipart to the
  proxy. The `take_id` is minted by corpusd.
- **A third kind**, `choir.take_audio`, holds the bytes.
- **Credential.** A device-login key is an owner-wide key with no marker, so an
  owner-wide key held by an agent cannot be told from the phone's; both may
  publish. Residual. The outcome route takes an API key that names a computer
  and refuses a browser session.
- **Mute** is read on the computer from notebook lines matching
  "Never play takes by voice-xxxxxx"; the own-take rule uses the handle derived
  from the owner id. Both remove the take from the call's input, so no line
  can name it.
- A model-written `cites` field on a take line is ignored, as on a clip.
- The take outcome's claim id is checked for shape only.
- Operator yank is a new route beside the podcast operator routes, not part of
  the package yank path.
- Record queries scan the kind by metadata; fine at invited-tester scale.

## Residuals (recorded, no code)

- Play length is bounded by the 512 KB size, not by the header: a file can
  carry more audio than its `mvhd` says.
- The daily bound is a process mutex in corpusd, correct because corpusd is one
  service; a second replica would race.
- Outcomes are not checked against offers (honest reporting, owner ruling): any
  account with a key naming its own computer can report on any take it can
  list; one outcome per listener computer per take stands.
- Yanked takes keep their prior counts (see above).
- Drill marking is by credential: a browser session on d@e publishes non-drill
  data; only the API key names a computer.
- A take candidate in a programme request is not checked against the store; the
  gain is self-only (it lands on the caller's own tape) and an outcome for a
  nonexistent take is refused.
- The audio route has no range requests; the answer is one `200`.

## Not built

The phone, the Swift twin of sheet kind `take`, the computer-side copy of the
audio, the posting of outcomes from the listener's computer, ordering the list
by record, the abuse report and operator queue.
