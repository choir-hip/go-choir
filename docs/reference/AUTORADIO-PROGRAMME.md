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

**Opening first, then small writes (slice C, 2026-11-16).** A request with
`cause: "open"` is answered by exactly one `opening` and then `done`; any
other request by one to six entries. The model writes less per call because
the provider may not stream, so the first sound waits on the whole answer:
a timing fact, not a quota (the phone's `extend` trigger is timing, one
request in flight). The kinds are `opening` (was `ident`) and `transition`
(was `bridge`). The route sends the new names only, with no switch and no old
wire names. **Order of landing:** the phone build that speaks `opening` and
`transition` (and asks `open`, then `extend` by timing, and speaks first
itself) goes on the device first, and this route lands after it; the installed
build drops kinds it does not know, so this route alone would play nothing for
it. Lines a model writes as `ident` or `bridge` are still parsed and renamed
(parse tolerance for a stored prompt or an old model habit; they are never
sent). Sections below that
say `ident`, `bridge`, "ten to fifteen" or a cap of 30 are superseded by this
paragraph and the section "Opening and ahead (slice C)".

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
| `edition_text` | optional; at most 32 KB; its paragraphs (blank-line split, `calibration.SplitBlocks`) are numbered `edition:P1`, `edition:P2` and so on. **Only when the computer has no notebook:** when it has one, the server's notebook is the input and `edition_text` is ignored ([AUTORADIO-NOTEBOOK.md](AUTORADIO-NOTEBOOK.md)) |

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
  `ident` or `bridge`. The event carries no probability or cause (they are written to the tape and never sent). It carries `entry_id` (`d-<segment_id>`, characters outside `[A-Za-z0-9_.-]` made `-`, 64 bytes: the phone's own grammar, so the phone's entry and the server's are one) and, for a content segment, `claim_id` (its forecast claim).
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
that resolves against this call's input, a forecast field on `ident` or `bridge`, a kind that is not `story`, `update`, `headlines`, `ident` or `bridge`. Three invalid lines in a row end the call. The refs `try:T<n>`, `mine:<8 hex>` and `tap:<entry id>` resolve against the notebook the call was shown (cause `trying`, `instruction`, `instruction`). Notebook lines (`{"kind":"notebook",...}`) are collected, not counted as entries, and read after the stream ([AUTORADIO-NOTEBOOK.md](AUTORADIO-NOTEBOOK.md), "Proposals"). Old lesson lines
are recognised and ignored until station L3.

## Opening and ahead (slice C)

- **Request:** `ahead_titles`, optional: the titles of the entries already
  ahead, in play order. At most 20, each at most 300 bytes; over is 400.
  They reach the model in a fenced `ahead` section, numbered from 1
  (`ahead:1`, ...), in the volatile part of the user message, with its own
  content-derived fence token, and only when the list is not empty (a request
  without it sends the input as before). The stable prefix, the system prompt
  and the cache key are byte-identical for requests that differ only in
  `ahead_titles`. They let the desk avoid repeating them and write a
  transition into them.
- **open:** the first valid line must be an `opening`; a valid line of
  another kind is dropped (and counts toward three invalid in a row). Once
  the opening is sent, the next valid line of any kind is dropped, noted once
  in the index's `dropped` and as `timings_ms.surplus_lines`, and the call
  ends there (the upstream read is cancelled; the hop's outcome is
  `opening_only`); the response is `done`, not an error. A lesson line or an
  invalid line after the opening is not a surplus line: it is ignored or
  noted, and reading goes on. An opening carries no forecast and no claim.
- **Other causes:** a line of kind `opening` (or `ident`) is dropped at
  validation (noted in the index's `dropped`, counted in `done.dropped`, never
  sent, never written). It does not count toward the three invalid lines in a
  row, so a model that greets and then writes good entries still plays;
  if nothing valid remains the answer is the usual "wrote no valid segment"
  failure. Found 2026-11-17: the deployed proof showed `kinds=opening,headlines`
  for an extend. The cap is 6 segments (was 30); a seventh line is dropped
  and the call ends `completed`, as the old cap did.
- **transition `between`:** the model may write
  `between: {after_segment, before_segment}`, each `s<n>` (the n-th entry
  written in this response, from 0: it counts entries written, not the lines
  the model wrote, so a dropped line takes no number; a `between` that names
  an entry that does not exist because of a dropped line drops the transition) or `ahead:<n>` (the n-th title of `ahead_titles`,
  from 1). `after_segment` must name an entry that already exists: an
  earlier `s<n>` of this response (n below the transition's own number) or an
  `ahead:<n>` within the list. `before_segment` may be `ahead:<n>`, an
  existing `s<n>`, or exactly `s<own number + 1>`, "the entry written next",
  which must be below the cap of 6. That one forward reference is allowed and
  is not held: the transition is committed and sent when its line validates,
  like any entry, so the entry it names may never arrive if the response ends
  first. When a response ends on a
  transition whose `before` is that forward reference, the index's `dropped`
  list carries a note (a note only; the event was already sent). `after_segment` and
  `before_segment` must differ, and no other field is accepted. A line that fails is dropped. The
  event carries `between: {after_segment, before_segment}` with `s<n>` made
  the full `segment_id` (`<stretch_id>.s<n>`) and `ahead:<n>` left as
  written, for the phone to resolve against the titles it sent. A transition
  without `between` is valid (the old bridge had none); `between` on any
  other kind, a podcast clip included, is dropped. The record keeps it (`between_after`,
  `between_before`), so a replay returns it. The route checks shape and
  bounds, not what the words say.
- **Text bounds** in bytes: opening 1500 (was ident 800, raised so an opening
  can name the top stories), transition 500.
- **Prompt:** `firstparty_prompts/autoradio_programme.md` is rewritten as an
  explanation of the system (the listener, time, signposting, transitions,
  honesty, forecasts) with no rule lists, no fixed format for the opening and
  no example lines (the podcast-clips suffix, appended only when the request
  lists clips, still carries one example line). It still states the JSON-line form and the `caused_by`
  grammar, which are runtime facts.
- **Proof:** `scripts/prove-autoradio-programme --host ... --api-key-file ...
  --computer ...` posts five `open` and five `extend` requests and prints the
  time to the first `segment`, the kinds and the server's `first_delta`,
  `first_segment_sent`, `first_segment` and `done`. The claim (p=0.7 that the
  median `open` first segment is under 6 s) is entered in the ledger by the
  lead before the proof is run.

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
each as one fence-safe line. The prompt section that names `podcast_clip`
(`autoradio_programme_clips.md`) is appended to the programme prompt only when the
request lists at least one clip; a request with none sends the prompt byte for
byte as before (test C7).

The model writes `{"kind":"podcast_clip","title":...,"clip":"<clip id>","text":"<lead-in>"}`
(text at most 800 bytes). A line is dropped when the clip is not one of the
request's, or is already used in this stretch (a clip plays once), `clip` or `title` is missing, or it carries `p_heard` or
`caused_by`. A `podcast_clip` writes a segment record (with `clip_id` and
`clip_seconds`, so a replay returns it) and no forecast claim (residual R2 of the
design). The segment cap and the invalid-run rule count it like any line.

## On the run sheet (settlement slice, 2026-11-17)

Status: built and tested on the build VM (`TestL1` to `TestL15`); not deployed;
the proof is `scripts/prove-autoradio-learning` on d@e. Settlement and the
listener's read route: [AUTORADIO-SHEET.md](AUTORADIO-SHEET.md) "Desk entries,
settlement and the learning route".

- When the route commits a segment (its claim written first) it places the
  entry on the listener's sheet in process (`applySheetOps`, `by: desk`,
  `at: end`, the reducer and its bounds, no second sheet), with
  `forecast {p_heard, caused_by, claim_id}` for a content segment and
  `origin: programme/<stretch_id>`. The op id is `o` plus the entry id (the
  phone's own formula). The entry is placed before the `segment` event is sent.
- A replay of a `request_id` places the stored segments again: an op already
  on the tape answers from it and writes nothing; one whose first placement
  failed is repaired. A `place` of the same id by the phone (an older build
  never posts one) is dropped `duplicate_id`.
- A placement failure is logged and counted in `done.place_failed`
  (`done.placed` counts the rest); it never stops the stretch. A forecast whose entry could not be placed gets a
  no-verdict record, reason `unplaced`, so no forecast is silently
  unsettleable (a notice only: the route writes no outcome).
- An entry the phone never plays stays ahead and its forecast lapses at its
  deadline, unsettled. When a call finds 94 or more entries ahead it removes
  the desk entries of stretches more than 12 hours old (desk `remove`, reason
  given); a listener's or the phone's own entry is never removed. The sheet's
  heard/skipped tail keeps 50 (the reducer's rule); settlement is written when
  the op applies, so a trimmed entry loses no evidence.


## The notebook in the input (stations L4 and L5, 2026-11-18, lane `learn-notebook`)

The stable, cached part of the user message gains a fenced `notebook` section
between the instructions and the candidates, when the computer has a notebook;
it replaces the `lessons` and `edition` sections. The volatile part gains a
fenced `settled` section (the last 40 settled forecasts, newest first, each
with its claim id) on every request but `open`. The done event carries
`notebook: {proposals, status, revision_id, dropped, strikes}` when the stretch
ended `completed`; the stretch index record carries the same as `notebook`;
the stretch start record carries the input receipt (`notebook_revision_id`,
`notebook_sha256`, `notebook_bytes`, `marked_cases`: the hash and length of the
notebook rows the model was shown, never the text). Station L5's claim tags gain
`line:<ref>` (within 64 bytes). Full rules, grammar, bounds, grants and routes:
[AUTORADIO-NOTEBOOK.md](AUTORADIO-NOTEBOOK.md).

## What the listener heard, in the input (2026-11-18, lane `heard-memory`)

Problem record: [autoradio-repeats-what-the-listener-already-heard-2026-11-18](../problems/autoradio-repeats-what-the-listener-already-heard-2026-11-18.md).
Code: `agentcore/api_autoradio_programme_heard.go` (named so that TestF41's glob
`api_autoradio_programme*.go` scans it; F41 holds that the programme route's
own source names no settle identifier and none of the reserved record
literals, so the settlement read lives in `calibration.ListAutoradioHeard`).
Built in the lane, not landed or deployed; no phone change (the new fields are
optional).

**Heard means one thing**: a segment of kind story, update or headlines whose
forecast has a heard-rule settlement (rule method, outcome true, an `air:`
evidence ref). Only the run sheet's op path writes it, from the phone's
`finished` outcome. A segment record alone, `skipped`/`cut`, `left`/`failed`
(no-verdict records), a settlement by another method and anything in the DJ's
own output are not heard. A cite whose role is `mentioned` is not a telling and
is dropped from the index; a `quoted` cite is kept (the story relayed what that
publication said, and the words are shown so the DJ can judge). The index is
the computer's, so every device of the listener gets the same marks.

**Reads per programme call, for the heard index** (the same on `open` and
`extend`): two lists, read at the same time, either failing (or panicking: each
reading goroutine recovers and logs `autoradio heard: read panicked`) meaning
"history unavailable". Both are bounded by time in the query: only rows last
written in the last 14 days (the heard window) are read, on the
(kind, owner, updated_at) index, so a sparse history does not walk the owner's
whole commitment kind. (1) The computer's newest segment records with the
forecast flag and no `take_id` (content segments only: openings, transitions
and takes are left out by predicate), at most 2,000 (`heardSegmentsRead`).
(2) The computer's newest heard settlements, at most 2,000
(`heardSettlementsRead`): the query itself holds rule method, the heard rule,
outcome true, so the false settlements that skipped and cut entries write do
not use up the page. A take's heard settlement cannot be told from a story's
by the settlement row, so takes (at most three per request) do count in that
page. An `air:` evidence ref is checked on each row. The join is by the claim id
a segment names; no claim or scorecard read. All limits apply after the
computer filter (`store.ListCommitmentRecordsByBodyOfComputer`).

Past the caps and the window: a story whose heard settlement or segment was
written more than 14 days ago, or older than the newest 2,000 of either page
(unheard content segments use up the segment page; it is the weaker of the two
bounds), reads `heard: not in the last 14 days`, and a full page is logged: `autoradio heard:
read full kind= rows= undecodable= unparsable= limit=`. (The words of a match
are cut at the same 14 days, so the key no longer outlives the words by more
than the gap between a row's write time and its settlement.) A settlement
whose settled time does not parse is skipped and counted (`unparsable=` on that
line and on the call's line); an undecodable row is counted and never fails
the read. Sparse case on the runner, 2026-11-18: 20,000 other commitment
records of the same kind (half older than the window) and 3 heard settlements:
the read took 71 ms and returned 3 rows (128 ms, 4 rows with no window).

The notebook keeps its own scorecard read as before, now through the call's
ledger: `extend`, `turn` and `result` only, never `open`; when it fails, the
notebook's counts and cases are skipped for the call (logged, not a 500); a
truncated read is logged. Timing, two runs on two machines, one number each:
the sequential, unfiltered read on the build VM over 2,000 rows each took
0.64 s and over 5,000 rows each 1.30 s; the concurrent, predicate read over
2,000 rows each took 0.20 s on the runner (join 8 ms; `BenchmarkBuildHeardSegs5000`
26 ms per join). The two figures are not the same machine, so they do not
isolate the change.

**What the DJ is shown.** In the candidates section, a `heard:` line under every
`ref:` line: `heard: not in the last 14 days` (the window, `heardWindow`; no
heard record in it, so the story may be told in full); `heard: at this version (v<n>) on <day>. Said
then: <words>` (v<n> is the version actually heard, which can be later than the
candidate's); or `heard: at an earlier version (v<n>, now v<m>) on <day>. Said
then: <words>`. Routes match by route, versions from the `@v` in the cite. A
fenced `heard` section (before the candidates, only when it has rows) lists
`recent: <day> | <title> | <words>` for stories heard in the last 14 days whose
route is not among the candidates, newest first, one row per route.

**The day is the listener's.** `<day>` is `on <date>` in the request's `tz`, or,
when `tz` is absent or does not resolve, in the fixed offset of its
`local_time`. A fixed offset knows nothing of daylight saving, so the date can
be a day off near a change or when the offset in `local_time` is not the one in
force then: the DST-correct day needs `tz`. With neither, the DJ is given no
date and no weekday: `earlier`, `1 day ago` or `<n> days ago`, counted by UTC
dates.

**History unavailable.** If either list fails, the call does not fail: the log
says `autoradio heard: unavailable ... class=<error type>` (no words), no
`heard:` line and no recent row is shown, and the heard fence says the history
is unavailable for this call. Unknown is never shown as "not in the last 14 days".

**An all-heard day.** When every story candidate (takes are never marked) is
heard at its current version the candidates section says so in one line and
the prompt tells the DJ to write at least one entry anyway: the candidate most
likely to have moved, told briefly as what the listener knows and that nothing
new is reported, or another angle. Both the line and the prompt say that this
brief telling carries `"telling":"followup"`, so the claim tag and the
`followups` count are right.
If the DJ writes none, the call ends as it does for any call with no valid
segment: the listener gets the spoken fallback line and an `error` event,
status `failed`, nothing is placed; the log line carries `all_heard=1` so
those calls are counted.

**Bounds, and why.** These keep the context small; they are not a quota on
calls (there is still one model call). Words of a matched candidate: 600
characters, cut at a sentence end. Recent list: 14 days, 12 rows, title 100
characters, words 160 characters. The words of a matched candidate follow the
same 14 days; a candidate whose heard record is older than that reads `heard: not in the last 14 days` (see the window above). With 40 candidates the heard
text is at most 40 x 600 + 12 x 400 characters.

**The DJ's choice.** The prompt gains one paragraph, "What the listener already
heard": leave out a candidate heard at this version with nothing new; for one
with something new, a short follow-up that connects to what they heard and says
only what is new, with `"telling":"followup"`; otherwise a full telling; always
write at least one entry; the all-heard rule above; never tell a heard story
again as if new. No second call, retry or pass. The line's `telling` is
optional (missing or anything else is `full`); each forecast claim gains the tag
`telling:full` or `telling:followup`, and the learning route's rows show it.

**Log.** One line per call: `autoradio heard: stretch= request= candidates=
never= same= earlier= recent= all_heard= unparsable= followups=`, counts only.

**Residual (named, pinned by a test).** A segment the listener skipped and later
replayed to the end is not heard: its forecast settles on the first outcome
(right for scoring), and the later finished end is not read back. Reading it
means folding the run sheet's ops per entry, which is not a small change.
TestR8 pins today's behaviour so the gap is visible.
