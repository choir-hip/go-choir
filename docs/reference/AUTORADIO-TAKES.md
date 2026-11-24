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
named computer (computer_id)"), a computer the account does not own is `403`, and a named,
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
full-length take is never refused for that. The take is stored with its
header's length (60.1 for an honest 60 s take), so the programme route's take
candidate check uses the same pair, `types.TakeHeaderMinSeconds` and
`types.TakeHeaderMaxSeconds`, and `mobile/fixtures/takes/list-boundary.json`
(takes at 60.1, 2.5 and 62.0 s) is decoded by a platform test and offered
whole by an agentcore test. At most 512 KB; `ftyp` must be present; transcript 1 to 2,000
bytes; 10 takes per publisher per 24 hours (deleted ones count; drill takes do
not); at most 3 takes per programme request (a fourth is a 400, the phone
selects them), 40 candidates in all; lead-in at
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

## A take on the run sheet

The programme route places a take segment on the server run sheet as the phone
builds it (`RunSheetEntry.take` in `Takes.swift` on `origin/lane/autoradio-integ16`):
kind `take`, placed by the desk, id `d-<segment_id>`, the model's title, the
DJ lead-in as `text`, `source` `{ref: "take:<id>", label: <publisher handle>}`,
`cites` the stories it was built on (the story it responds to, role mentioned;
never the take itself), `est_seconds` the lead-in's estimate plus the take's
own seconds, and `forecast` `{p_heard, caused_by, claim_id}` like any desk
entry. `settleEnded` ignores the kind, so the listener's own end op settles it
and `calibration.TakeOutcomeFromSettlement` reads the settlement. The learning
read names the take (`take_id`).

Validation: a take line must carry a lead-in (an empty `text` is dropped by the
programme validator), so Go's `runsheet` keeps `take` in `TextRequired`; the
phone's twin (`RunSheetKind.textRequired`) is looser and allows a take with no
text. No server path writes one, so there is no disagreement to fixture.

Two notes. Outcome rows written before the publisher key was added to them
(an earlier build of this lane) are invisible to a record read; nothing is
deployed, so re-run the drill if an earlier store is reused. The first two
commits' `Security-Effect` text says the length is 3 to 60 s; publish already
stored the header's 2.5 to 62.0 s, and the third commit's wording is the
statement that counts.

## While the publisher's account deletion is pending

A take of an account that asked to be deleted moves from `live` to `suspended` (`SetAccountTakesState`, corpusd `POST /internal/platform/account/suspend`), re-asserted by the purge runner every minute, so no read serves it (every read lists or serves only `live` takes). A cancelled deletion puts the state back; the transcript and audio are untouched in between. After 72 hours the purge deletes it as the publisher's own delete does (`deleted`, audio object gone, transcript blanked) and the row stays. See `docs/design/account-create-delete-2026-11-23.md` and `docs/changes.md` 2026-11-24.

## After a delete or a yank

The take row stays, with its id, handle, times, size, hash and state
(`deleted` or `yanked`); its transcript and the story it answered are blanked.
The audio object is deleted, but a body of 1 KB or more is externalised, and
those bytes stay on disk until the artifact GC pass removes them. That holds
for the take row too: the pre-blank body, transcript included (a transcript
over about 1 KB), survives in the artifact store until the same GC pass. The outcome
rows stay as counts: a yanked take keeps its prior counts in the publisher's
record (a decision for the owner, not made here).

## Reads and scans

The list reads live takes by their `state` metadata (newest first), so deleted
and yanked rows never take a place in the window. It asks the store for twice
the list's limit first and widens only when drill rows or the 7-day window leave
fewer, up to 2,000 rows (the store orders by `updated_at`, which for a live take
is its publication; it has no offset, so there is no deeper paging). Reaching
the bound is logged at most once a minute. The 5,000-row outcome scan of a
record is logged the same way when it is reached, because the record then
undercounts.

A publisher's record is keyed by the publisher key; the handle (`voice-xxxxxx`,
24 bits) is display only, because two publishers can share one. The outcome rows
carry the publisher key, so counts, the drill line and the per-listener rows of
one publisher are never pooled with another's. A record asked for by handle
alone (the route takes a handle) is the caller's own when the caller published
under it, and otherwise that of the key that most recently published under the
handle. A collision never widens the read.

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

## A malformed take candidate

A take candidate in a programme request that fails validation (a bad id or
handle, a length outside the header's 2.5 to 62.0 s, a multi-line title, a
transcript or record out of bounds, a bad `responds_to`, take fields beside a
route) is dropped, logged and counted; it never fails the request, so a
stranger's published take cannot make another listener's programme call fail.
No client on main sends takes yet; the phone lane
(`origin/lane/autoradio-integ16`,
`mobile/AutoradioKit/Sources/AutoradioKit/Choir/RadioProgramme.swift`) is what
was read, and it does not depend on a 400 for a malformed take. One log line
per request names the count dropped. What stays a 400
is structure the phone itself controls: a fourth take, an unknown kind, take
fields on a story, an unknown key.

## Known fixture differences

The phone's fixtures in `mobile/fixtures/takes/` are shape-only and were not
edited by this lane. For the next phone lane to correct:

- `publish.json` carries `"publisher_key"`, which the server never answers.
- `refused-daily.json` says `invalid take: 10 takes in the last 24 hours, the
  limit is 10`; the server says `daily take limit reached: 10 takes in the last
  24 hours, the limit is 10` (429 both).
- `list.json` holds rows the public list never answers: a `deleted` take and a
  row with no `take_id`. (The public list does answer the listener's own live
  take, as it does for every account; the phone removes its own.) `record` there lacks the
  `publisher` and `summary` keys the server always writes. `mine.json` lacks
  `audio_url` and `responds_to`. `record.json` lacks `summary` and `drill`.
- The phone's header comment says deleting an already-removed take is 409; the
  server answers 200 (idempotent).
- `routes.json` declares publish as `201`, which is what corpusd answers; the
  proxy relays it.

## The phone's own state, account by account (2026-11-20)

Written after three reviews found, one instance at a time, the same defect: state that belongs to one account reaching the next. This table lists the takes state the phone keeps, with what clears each
row and whether that can fail without a sign. It lists the state of the Takes section and its stores; the run sheet's own rows are the run sheet's (row 11). Code: `mobile/AutoradioLab/take-session.ts`,
`takes.ts`, `mobile/AutoradioKit/.../TakeSession.swift`, `Choir/Takes.swift`, `AccountWipe.swift`. The account generation (`api.ts accountGeneration`, bumped by `account.ts` before the wipes) is the one
clock: shared writes are held to it (`writeCurrent`), and the shared overlay is dropped when it moves.

### Nothing is carried from the pre-release build

Before this move the phone kept the listener's own take ids in `Application Support/Autoradio/takes-own.json` (`{handle, ids}`) and muted voices in `takes-muted.json` (`[handle]`). Those files carry no
proof of whose they are, and the native delete at sign-out is `try?`, so a file can outlive its account. Every guard that inferred ownership from the phone's history (a flag, then a marker file, then a
proof by take id) was a statement about that history with a window where it was false, and four reviews found the windows one at a time. The owner's ruling (2026-11-20): "I don't need to save stuff from the
last couple days of testing. The app is very new. We haven't launched it yet. I don't need to save anything." So **the Takes code of this build reads neither file, adopts nothing from them and deletes neither at launch** (the demo-mode cleanup no longer touches `takes-own.json` either);
the account wipe still removes both when an account leaves (`AccountWipe`, store `takes`, `legacyTakeFiles`; `AccountWipeTests`). The new shared store (`app:/data/takes.json`) starts
empty.

What that means for a listener with a pre-release build: **muted voices start empty** (mute again), and **the own-ids list starts empty and is re-learned from the server**: at the first `refresh` the
server's list of the account's live takes (`GET /api/platform/takes?mine=1`) is read and their ids and the account's handle are learned. Takes published with a pre-release build that are still live on the
server are therefore still shown under "my takes"; what is not carried is the ids of takes that are no longer live (deleted or yanked), which only kept those takes out of the programme's candidates and
which the list no longer offers anyway. Nothing recorded and not published was ever stored across launches.

The handle is not an identity, for the record: it is `"voice-" +` the first 6 hex digits of `sha256("choir-voice-author:" + userID)` (`internal/types/autoradio_takes.go:35-38`), 24 bits, display only, and two
publishers can share one (`internal/platform/takes.go:141-142`; `takes_test.go:486`). The programme's copy of these (`takes-copy-own.json`, `takes-copy-muted.json`, once pushed by the `takeMirror` op) is gone since 2026-11-22: nothing in Swift read it, so the op, the
stores and the pushes were deleted; the files are legacy and the account wipe removes them.

### The table

| # | State | Lives in | Belongs to | Cleared by (sign-out, account change, key change) | Can the clearing fail silently? | Across a process restart | Test |
|---|---|---|---|---|---|---|---|
| 1 | The overlay: kept-data cache, my takes, my record, notice, "published as", publishing flag, the change queue | shared memory (`app.ts`, one `Overlay`) | the account of its `gen` | `sync`, at the next page or act after the generation moved: dropped whole, the recorder discarded; the page takes `signedOutTick` | no (memory) | starts empty, reads row 2 | `take-session.test.ts` B2 (wipe, render, mute), "row 1" |
| 2 | Own ids, handle, muted voices | `app:/data/takes.json` | the account | `account.ts` store `takes` (`wipeKept`); every write is `saveKept` through `writeCurrent` with the overlay's generation (refused after the generation moved, removed if it landed after); a write that returns after the generation moved is not kept | as for every moved store: `account.ts` keeps its marker and retries | read again | "row 3" (a mute suspended inside the write; a write landing as the account changes), S4 |
| 3 | Changes to row 2 made at the same time | the overlay's queue (`mutate`) | the account | dropped with row 1; a queued change whose account moved on is skipped | no | gone | "row 4" |
| 4 | (removed 2026-11-22) The programme's copy of the own ids and muted voices: `TakeOwn`, `TakeMutes`, `TakeMirror` and the `takeMirror` op were written and never read, and are deleted | | | | | | |
| 5 | Files earlier builds left: own ids and handle, muted voices, and the programme's copy of them | `Application Support/Autoradio/takes-own.json`, `takes-muted.json`, `takes-copy-own.json`, `takes-copy-muted.json` | unknown: nothing in them says whose | never read, adopted from or deleted at launch; the account wipe removes all four (`legacyTakeFiles`) | the unlink is `try?`: yes; but nothing reads them, so they can reach no account | never read | `AccountWipeTests` (removed with the account); `take-session.test.ts` "nothing is carried from the pre-release build" (the device is never asked, nothing adopted, on a failure, on no live take, on a restart); `TakesTests` (the programme's copy is other files) |
| 6 | The recorder's live take: phase, transcript, the story answered, the preview, the gate hold | native memory (`TakeSession`, `TakeSession.live`) | the account that recorded it | `ChoirApp.signOut` discards it FIRST (before playback is stopped for good); the `AccountWipe` store `takes` discards it on an account change; shared `sync` discards it again | no (a call into memory); the radio is given back | gone (a new session is idle) | `AccountWipeTests` (review, wipe, idle, file gone, radio given back), "row 1" |
| 7 | The recording and the played take | the temporary folder: `takes/*.m4a`, `take-play-*.m4a` | the account | row 6's discard; the `AccountWipe` store `takes` removes the folder and the played file | yes (`try?`); no session reads a file it did not make, so it is leftover disk only | orphaned until the system clears temporary files; never read | `AccountWipeTests` (files gone) |
| 8 | "A publish is under way" | native `TakeSession.publishing`, the overlay flag | the account | row 6's discard and `done` clear it; shared clears it in `finally`; nothing reads the native bit as a fact | no | gone | "B1" |
| 9 | A mute sent to the learning engine | native file `steer-queue.json` (`SteerQueue`) | the account | `AccountWipe` store `steer` (not changed here); a mute is not sent once the account changed | the unlink: yes, as every native store | read at launch | `AccountWipeTests` (steer), "S4" |
| 10 | The public takes by id (the run sheet entry, the audio fetch) | native memory (`TakeStore`) | nobody (the public list) | `AccountWipe` store `takes`, anyway | no | gone | `AccountWipeTests` |
| 11 | A take on the run sheet (an entry of kind `take`: lead-in, publisher, cites) | the run sheet's own store (`RunSheetStore`) | the account | the run sheet's wipe (`AccountWipe` store `run-sheet`), not changed here | as the run sheet's | the run sheet's | the run sheet's (`AccountWipeTests`) |

What is still not true, said plainly: (a) between a restart and the first `refresh` the programme can use whatever copy files were left (row 4): the push is the first thing `refresh` does, and `refresh` runs
when the radio page is on screen and the phone is signed in (`takesLoadTask`), an act, or after a publish; until then it is a few programme requests with the wrong own-take and mute filter (no voice is
published and no take is lost, but it is cross-account influence). (b) Muted voices and the ids of no-longer-live takes start empty on a phone upgraded from a pre-release build (above). (c) The unlinks of rows 4, 5, 7 and 9 are `try?` and can fail without a sign; none of them can
hand data to another account through the upgrade, because nothing reads the old files. (e) The signed-in requests of the takes routes are not marked uncacheable by the proxy
(only the audio is), and the host's cache policy is not read here.

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
