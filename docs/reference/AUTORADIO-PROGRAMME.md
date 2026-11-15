# POST /api/autoradio/programme: the DJ's stretch with forecasts

Status: station L1 of
[the learning-engine goal file](../definitions/choir-autoradio-learning-engine-2026-11-14.md),
landed as 45bf7ce4c and deployed 2026-11-15 (proof: calibration claim C98). Design:
[autoradio-learning-engine-2026-11-14.md](../design/autoradio-learning-engine-2026-11-14.md)
(sections 3, 9 and 12; section 12 is normative) and the programme design's
slice 2. Failure list and its tests:
[autoradio-l1-failure-list-2026-11-15.md](../design/autoradio-l1-failure-list-2026-11-15.md).
Code: `internal/agentcore/api_autoradio_programme*.go`. The stream the phone
sees is pinned by `mobile/fixtures/learning/programme_stream.json`.

One request, one streamed model call, one server-sent-events response. The
model writes one JSON object per line; each valid line becomes one `segment`
event. For each content segment (`story`, `update`, `headlines`) the server first appends the segment record and one forecast
claim (agent `autoradio-dj`, probability `p_heard`, a stated `caused_by`), and
only then sends the event. The route never writes an outcome: it has no air
event and no settlement (station L2's route does).

Authentication is the turn route's (a signed-in client, or an owner-wide key
with `X-Choir-Computer`). The model is the role `autoradio_programme`: the
`autoradio` pool (River, then Bedrock Haiku 5.5), reasoning `low`, no token
budget, its own cache prefix `autoradio-programme:`. Until River is registered
on the gateway the call is answered from Bedrock, which does not stream: the
first segment then arrives when the whole stretch is written, and no latency
claim is made.

## Request

`Content-Type: application/json`, at most 256 KB (413 over). Unknown fields are
refused (400), so a request carrying `heard`, `after` or `interview_notes` answers 400.

| Field | Rule |
|---|---|
| `request_id` | required; `[A-Za-z0-9][A-Za-z0-9_.-]{0,63}`; the idempotency key |
| `cause` | required; `open`, `extend`, `turn` or `result` |
| `local_time`, `tz` | optional; RFC 3339, and an IANA name (at most 64 bytes). A name the computer cannot resolve is not refused: the time is shown by its own offset |
| `candidates` | optional; at most 40 items `{route, version, title, dek, text?, clips?}`, ordered by importance, highest first. `route` has no space, `@` or `#`; `version` is a whole number or a short string; `title` one line of at most 300 bytes; `dek` required, at most 1000 bytes; `text` at most 8 KB and only on the first ten items. A candidate's cite ref is `pub:<route>@v<version>`. `clips` (at most 3) are the podcast clips that source the story, copied by the phone from `GET /api/platform/podcast-clips` (see below). Over a bound is 400 |
| `edition_text` | optional; at most 32 KB; its paragraphs (blank-line split, `calibration.SplitBlocks`) are numbered `edition:P1`, `edition:P2` and so on |

Refusals are plain JSON `{"error": "..."}` before the stream starts: 401 no
owner, 405 not POST, 400 bad body, 409 the `request_id` is being answered now or
was used with another `cause`, 413 body too large, 503 no streaming model bound.

## Response

`200`, `text/event-stream`, first bytes the comment `: autoradio`. Events:

- `stretch`: `{stretch_id, written_for, request_id}`, first.
- `segment`: `{stretch_id, segment_id, kind, title, text, cites, block_count, est_seconds, cut_at}`.
  `segment_id` is `<stretch_id>.s<n>`, contiguous over the segments streamed
  (a dropped line takes no number). `cites` are `{ref, role, label}` (role
  `subject`, `quoted` or `mentioned`; label defaults to the candidate's title).
  `block_count` is the server's `SplitBlocks` count of `text`, the one the heard
  route will check; the phone never recomputes it. `title` may be empty on an
  `ident` or `bridge`. The event carries no forecast.
- A `podcast_clip` segment is a `segment` event with two differences: `cut_at` is `segment` (real audio is cut only between segments) and it carries `clip: {clip_id, story_ref, seconds}`. Its `text` is the DJ's lead-in, which the phone speaks; the phone then plays the clip, finding `audio_url`, `start` and `end` for `clip.clip_id` in the clip set it fetched. `cites` is the one story the clip is for (role `quoted`), filled by the server.
- `done`: `{stretch_id, status: "completed", replayed, segments, claims, dropped, provider, model, index_record_id, timings_ms, members_tried, usage}`.
- `error`: `{stretch_id, spoken, reason, status}` instead of `done`: a claim
  write failed (`store_failed`), three invalid lines in a row (`invalid_run`),
  the model failed after a segment, or every pool member refused or ended with no valid line (`failed`), or the wall-clock or idle deadline passed (`deadline`).
  Segments already sent stand; the phone keeps playing what it has.

Validators drop a line, never the call (design 3.2): unknown kind, empty or
over-long text, a content segment with no title, a title that is not one line
of at most 200 bytes, a cite outside the candidate set, `p_heard` missing or
outside [0, 1], `caused_by` missing or not one of `importance`, `explore`,
`lesson:L<n>`, `instruction:turn/<id>`, `instruction:note/<id>`, `edition:P<n>`
that resolves against this call's input, a forecast field on `ident` or `bridge`, a kind that is not `story`, `update`, `headlines`, `ident` or `bridge`. Three invalid lines in a row end the call. Lesson lines
are recognised and ignored until station L3.

## Records

All are owner-private notice directives with `desk=app:autoradio` (design
12.2): `autoradio-stretch-start:<sid>` (the start time the deadlines derive
from), `autoradio-request:<request_id>`, `autoradio-seg:<sid>/<segment_id>` (the
text), the forecast claim `calib:autoradio-dj:<hash>` (deadline: the start plus
12 hours), and `autoradio-stretch:<sid>` (the index: claim ids as `related_ids`,
the status, the drop reasons), written whenever the handler itself ends the
call, not after a crash or a failed claim write.

## Bounds and invented numbers

These are deadlines and bounds on what the model can cause, not limits on our
model calls. Invented, because neither the design nor the turn route names one
(the gateway client's own deadline is 10 m 30 s): the wall-clock deadline of 120
seconds for the whole walk, the idle deadline of 30 seconds between deltas
(from the first), the cap of 30 segments per stretch (the prompt asks for 10 to
15), the per-kind text maxima in bytes (story 8000, update 4000, headlines 3000,
ident 800, bridge 500), 1,000 bytes per instruction or note in the prompt, 32 KB
of edition text. A deadline or the cap ends the call through the index with a
`status` (design 12.2). Kinds: `story`, `update`, `headlines`, `ident`,
`bridge`, `podcast_clip`; `mail`, `podcast_note`, `answer` and `result` are dropped until their
stations. A candidate needs a non-empty `dek` (12.7).

Residuals: a candidate's text can steer `caused_by` to a real listed ref (a
forged cause on a valid claim; it cannot be validated); the in-flight key is
process-local, so two overlapping API processes could each answer one
`request_id` (a duplicate model call, not a duplicate claim); the notes and
turns reader fetches at most 5,000 records of a kind.

## Replay, disconnect, crash

The same `request_id` returns the stored stretch with no model call and no
write: the segments, then `done` with `replayed: true`. A client that goes away
does not stop the call; the stretch commits and a replay returns it. A stretch a
restart interrupted has its start, segment records and claims and no index; its
`request_id` replays the stored segments that have their claim and an `error` ("ended without an index"), and the retry is a new `request_id` and a new stretch that never
re-streams a claimed segment id. Nothing resumes at boot.

## Podcast clips (2026-11-15)

Design: [podcast-clips-2026-11-15.md](../design/podcast-clips-2026-11-15.md).
Request: a candidate's `clips` are `{clip_id, show, episode_title, published?, seconds, quote}`
(unknown fields refused): `clip_id` matches `clip-[0-9a-f]{12,64}` and is unique
in the request; `show` and `episode_title` are one line of at most 200 bytes;
`seconds` is 5 to 300; `quote` is 1 to 400 bytes; at most 3 clips a candidate.
Over a bound is 400. The clip lines reach the model inside the candidates fence,
each as one fence-safe line.

The model writes `{"kind":"podcast_clip","title":...,"clip":"<clip id>","text":"<lead-in>"}`
(text at most 800 bytes). A line is dropped when the clip is not one of the
request's, `clip` or `title` is missing, or it carries `p_heard` or
`caused_by`. A `podcast_clip` writes a segment record (with `clip_id` and
`clip_seconds`, so a replay returns it) and no forecast claim (residual R2 of the
design). The segment cap and the invalid-run rule count it like any line.
