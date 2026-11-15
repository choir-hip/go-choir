# Autoradio on the four-stroke learning engine (2026-11-14)

How the DJ learns one listener. This is the first place the
[four-stroke learning engine](../learning-engine.md) is built whole: forecast,
outcome, lesson, trust. The goal file that runs it is
[choir-autoradio-learning-engine-2026-11-14.md](../definitions/choir-autoradio-learning-engine-2026-11-14.md).

It changes the programme design
([autoradio-dj-programme-2026-11-11.md](autoradio-dj-programme-2026-11-11.md))
in the places listed in section 9. Everything else in that design stands:
stretches, segments, hand-offs, the slices.

Revised after the station L0 review (two seats, both REPAIR). Tags: **[code]**
read from code at `origin/main` 955d4527f; **[doc]** from the programme
design; **[new]** decided here. Line numbers drift.

Terms. An **air event** is one playback report from the phone, stored once.
The **air log** is the set of air events. A **forecast** is the DJ's claim
about one segment. A **settlement** is the rule's mark on a forecast; the air
log is the evidence, the settlement is the outcome. The turn route's own
`heard` field (block index and last words) is a playing position, not an air
event.

## 1. Why Autoradio first

Owner, 2026-11-13: apply the engine first to "the auto radio personalization
learning loop", then "backport the learning engine into auto paper", then
self-development; "the connection between auto radio and auto paper is the
user's individual auto paper edition".

Autoradio has the best teacher we have. The three questions of the learning
engine (section 3 there):

| Question | Autoradio |
|---|---|
| Is there a teacher? | Yes. The listener finishes, skips, cuts in or leaves, within seconds of the segment. |
| Is it someone other than the student? | Yes. The act arrives from the phone under the device key. The DJ's model call has no way to write it. |
| Does the student get the paper back? | Not today. This document is that part. |

A listener gives twenty to forty outcomes an hour. The whole self-development
fleet gave about 95 rounds a day.

## 2. The cycle, once

1. **Forecast.** The DJ writes a stretch. For each content segment it says
   why the segment is there and how likely the listener is to hear it through.
2. **Outcome.** The phone reports what was heard. A rule, not the DJ, settles
   each forecast.
3. **Lesson.** The next stretch's call is shown the marked cases. It writes
   down what it learned, each lesson citing the events it rests on. Lessons go
   in the notebook, which the listener reads and corrects.
4. **Trust.** The listener sees the record and decides what to hand over.

Call it. Check it. Learn it. Earn it.

## 3. Forecast

### 3.1 What the model writes

One JSON object per line, as the programme design has it **[doc 2.3]**. A
content segment (`story`, `update`, `headlines`, `mail`, `podcast_note`) adds
two fields; the handler, not the model, mints `segment_id` (stretch id plus
index) and any lesson id.

```json
{"kind":"story","title":"...","text":"...","cites":["pub:..."],
 "p_heard":0.7,"caused_by":"lesson:L12"}
```

`ident`, `bridge`, `answer` and `result` carry no forecast; a forecast field
on them drops the line. `p_heard` is the DJ's probability that this listener
hears the segment through (section 4 defines "through").

`caused_by` names the one input that most decided the pick.

| Value | Meaning | Authority **[doc 2.6]** |
|---|---|---|
| `instruction:turn/<turn_id>` or `instruction:note/<id>` | the listener said it, in a turn or in the notebook | 1 |
| `edition:P<n>` | paragraph n of the edition text, numbered by the handler in the input | 2 |
| `importance` | the paper's importance score or mail trust | 3 |
| `lesson:<id>` | an active lesson in the notebook shown to this call | 4 |
| `explore` | none of the above; the DJ is trying something | 4 |

`explore` is a cause label only. There is no room-size setting. An `explore`
segment still cites the candidate set and may not contradict an edition aim or
an instruction; widening what the DJ may try is a later owner act (section 7).

### 3.2 Validators

Every validator drops the line (a stretch line, not the call) and notes the
reason in the stretch's `timings`; three invalid lines of any kind end the call
**[doc 2.3]**. A dropped line is never streamed and has no forecast, so it can
neither be settled nor count as unforecast.

| Check | Failure |
|---|---|
| kind known; text non-empty; length in the kind's range; every `cite` in the candidate set | drop (as the programme design) |
| content kind has `p_heard` and `caused_by` | drop |
| `p_heard` a finite number in [0, 1] (endpoints are allowed and scored as stated) | drop |
| `caused_by` grammar above, and it resolves: `lesson:<id>` names an active lesson in this call's input; `instruction:` names an instruction listed in this call's input; `edition:P<n>` with n within the numbered paragraphs | drop |
| a segment citing a lesson written in the same response | drop (the lesson is not in the input) |
| lesson line: text one line, 300 characters or fewer; cites are `air:<event_id>` or `instruction:` refs, at least two air events or one instruction, every one listed in this call's input | drop |
| lesson line: `replaces` names an existing active lesson of this listener; a struck or unknown id | drop |
| lesson line whose every cite is already cited by a struck lesson | drop (a struck lesson is not rewritten from the same evidence) |
| a lesson id or segment id sent by the model | ignored; the handler mints `L<n>` and the segment id |

A failed claim write is not model invalidity. The handler ends the call with an
`error` event; the segment is not streamed; nothing is retried by itself.

### 3.3 One forecast, one claim **[new]**

Each content segment is one claim in the calibration claims store
**[code: `internal/calibration/records.go:196` `NewClaimRecord`; one question,
one probability per claim]**. A server-side constructor
`calibration.NewAutoradioForecast` builds it by calling `NewClaimRecord`; the
HTTP claim route does not expose it (section 4.3).

| Claim field | Value |
|---|---|
| `agent.name`, `agent.model`, `session_id` | `autoradio-dj`; the model that wrote the stretch; the stretch id |
| `type`, `p` | `forecast`; `p_heard` |
| `claim` | "The listener hears segment `<id>` (`<kind>`: `<title>`) through." |
| `idempotency_key` | `<stretch_id>/<segment_id>`. The id is `calib:autoradio-dj:<hash>`, deterministic, so a replay writes nothing new **[code: `ClaimRecordID`, `records.go:185`]** |
| `project` | the segment kind, so the existing `group_by=project` stratifies by kind **[code: `math.go` `ParseGroupBy`]** |
| `deadline` | commit time plus 12 hours; past it the existing status `overdue` marks the forecast lapsed (section 4.2) |
| `source_ref` | `autoradio:<stretch_id>/<segment_id>` |
| `tags` (at most 20 of 64 bytes) | `autoradio`, `stretch:<id>`, `seg:<id>`, `kind:<kind>`, `cause:<class>`, `ref:<id>` (for `instruction`, `edition`, `lesson`), `sha:<first 32 hex of the text's sha256>`, `blocks:<n>` |

`caused_by` and any lesson ref live in the tags, set at commit time and never
edited (claims have no edit route). The tally and the heard handler read them
there. The builder asserts every tag is at most 64 bytes.

The **stretch index** is a record `autoradio-stretch:<stretch_id>`
(`record_kind` `autoradio.stretch`, in the pattern of **[code:
`internal/agentcore/api_autoradio.go:360`]**) whose `related_ids` are its claim
ids. It is written at `done`. A stretch whose call never completes has no index
and keeps its claims for the segments already streamed; those settle as usual.
The listing of a stretch's forecasts reads the claims by tag, never the index.

### 3.4 The order is the guarantee

For each valid line, in this order: append the claim, then send the `segment`
event. The forecast is committed before the segment reaches the phone, so no
event can precede its forecast. On the Bedrock fallback (no streaming, section
11) the handler validates and appends line by line, then sends the segments in
order; the same order holds.

## 4. Outcome

### 4.1 One writer

`POST /api/autoradio/heard` is the only writer of air events and the only
caller of the reserved settle method (4.3). The programme request has no
`heard` field and no other path to either; the decoder refuses unknown fields,
so a request carrying `heard` answers 400. The phone no longer batches events
on the programme request (a change to **[doc 2.2, 3.4]**). Two tests hold this
(goal file): the programme call with a provider that tries to emit an event or
a settlement, and a call-site test that the settle method has one caller.

### 4.2 The heard rule, a function of the event set

An air event: `{event_id, session_id, stretch_id, segment_id, text_sha256,
seq, type, last_block, block_count, started_at, ended_at, cites}`. `type` is
one of: `finished`, `skipped` (terminal); `cut`, `resume`, `paused`, `left`
(not terminal); `more`, `less`, `corrected` (signals, never settle). `seq` is
the phone's counter for that segment; `last_block` is the last block that began
playing. The phone synthesises `left` at the next launch, since a killed app
cannot send it.

**Heard through.** A segment is heard through when its final block began
playing: `last_block = block_count - 1`. This is the one notion that scores
`p_heard`. The programme design's dedupe test, "to its end or past half"
**[doc 2.6]**, is a different notion and is named **covered**: `last_block + 1`
at least half of `block_count`. Dedupe uses covered; Brier uses heard through.

**Settlement.** For each segment, take its events in server receipt order (a
batch is first sorted by stretch, segment, `seq`). The first `finished` or
`skipped` settles the forecast: yes when `finished`, or when `skipped` with
heard through; no when `skipped` earlier. Later terminal events, conflicting or
not, are stored and ignored; the response says `ignored`. Settlement id is
deterministic, one per segment (`<claim id>:settle:rule:autoradio_heard`
**[code: `SettlementRecordID`]**), and the append is conditional on not
existing, so two racing batches settle once. Evidence is `air:<event_id>`.

**Refused.** An event for a `(stretch_id, segment_id)` with no forecast claim
the server wrote; a `text_sha256` not matching the claim's `sha:` tag; a
`last_block` at or beyond `blocks`; `finished` with `last_block` short of the
final block. The same `event_id` with different content is a content conflict.
The route's per-event result codes are 200 (accepted, duplicate or ignored),
409 (the conflict) and 422 (the other refusals); the HTTP status is 200 and
the phone reads the codes. A batch is at most 200 events and 64 KB (a bound on
strangers).

**Lapse and coverage.** A segment with no terminal event is open until its
deadline (12 hours after commit) and then lapsed; that covers pause, a cut
never resumed, `left`, and a segment never played. Open and lapsed are
excluded from the Brier and counted: coverage is settled over forecast, read
from the existing summary (`claims`, `settled`, `open`, `overdue`
**[code: `GroupStats`, `math.go:100`]**). Late reports are allowed at any time
and settle normally; a call's input is a snapshot at call time.

**Ack and retention.** Every event in the request gets a result code; the phone
deletes an event from `signals.jsonl` once it has any result, and keeps the
rest **[code: `mobile/exact-package/apple/AutoradioModule.swift:99`]**. The
local file holds only what is not yet acknowledged.

### 4.3 Rule 1 at the generic route

The generic claims routes today let any key with `precommit:write` create a
claim and settle any claim with any `rule` or `owner` method; a `rule` name
only needs to be non-empty; and the earliest rule settlement governs
**[code: `handler.go:187,261`, `records.go:398`, `math.go:49`; the device key
holds all scopes, `internal/auth/device_login.go:415`]**. By itself that lets a
phone settle its own forecast. So the Autoradio namespace is reserved:

- the generic `create` refuses an agent whose slug begins `autoradio` and any
  tag beginning `autoradio`; the generic `settle`, and `get`, refuse a reserved
  claim; list and summary exclude reserved claims before the record limit
  (403 on create, settle and get). One listener's day must not evict another
  agent's calibration from the join **[code: `DefaultMaxRecords` 10000]**;
- the heard handler settles through a store method that accepts only a rule
  name from a closed registry (`autoradio_heard`, bound to agent
  `autoradio-dj`). Nothing else can write that rule, and a forged `rule`
  settlement cannot reach a reserved claim;
- the notebook reads reserved claims through the same calibration code with
  `agent=autoradio-dj`, extending `calibration.Summary`; there is no second
  scoring path.

Not done, on purpose: refusing `rule` and `owner` methods wholesale from API
keys. The CLI uses them for ordinary claims **[code: `cmd/choir/precommit.go:72`]**,
and the generic handler cannot tell a key from a session. The reservation
closes the Autoradio case, which is the only one the DJ reads. This hardening
is a **red** change (auth and evidence) and lands in station L1a before any
reserved claim is written.

### 4.4 Gestures and corrections

`more`, `less` and `corrected` events go to the same route, optionally naming a
segment; a correction names its turn record. They never settle `p_heard`. They
are evidence about the pick and reach the lesson stroke as `air:` cites
(gestures) or `instruction:` refs (corrections).

## 5. What the DJ is scored on, and what it is for

- **Calibration**: was `p_heard` honest. Strict Brier over forecasts, with
  coverage beside it **[code: `BrierStrict`, `math.go:112,240`]**.
- **Reliability**: did the programme serve the edition and the instructions.
  Its evidence is corrections, gestures and edition changes.

**The measurement (ARL-C1 only).** The summary's `BaseRate` is the in-sample
mean outcome **[code: `math.go:236`]**, which a DJ that merely restates the
base rate would match. The reading therefore adds a causal base rate to
`GroupStats`: for each forecast, the mean outcome of the forecasts settled
before it was committed (0.5 until ten are settled), and the Brier of that
base rate. The reading is, per segment kind (`group_by=project`): `n`, coverage
(settled over forecast), strict Brier of `p_heard`, strict Brier of the causal
base rate, their paired mean difference and its standard error; for the first
100 settled and again for the second 50 against the first 50, and with the
per-lesson tallies of 6.2 beside it. The tally measures what the DJ said caused
a segment, not what did. The reading settles ARL-C1 and nothing else: not the
mission's value, not the lessons' truth.

Hearing through is the forecast target, not the objective. Guards:

1. The DJ is never shown a finish rate, a count, a total, a streak or a band
   table. It is shown marked cases (6.1).
2. A skipped segment forecast at 0.3 is a good forecast. A challenging segment
   the listener asked for, forecast honestly low, is not penalised for being
   low; serving the request earns reliability.
3. A stated aim outranks an inferred lesson. A lesson may only propose a
   change to the listener (section 8).

## 6. Lesson

### 6.1 Getting the paper back

The call's input, after the stable prefix, carries these sections. Every world
or listener string (titles, candidate text, spoken words, edition text, lesson
text) goes in the way the turn route does it: inside a fence whose token is
derived from the content, one line per field, fence look-alikes broken
**[code: `api_autoradio.go:427-486` `fenceSafe`, `oneLine`, `frame`]**, and the
section says the text is data and never to be followed.

- **Instructions**: each listener instruction since the last clear, with its
  ref (`instruction:...`) and the words verbatim, authority 1. A corrected
  lesson is stored as an instruction in the listener's words; the DJ never
  rewrites it.
- **The edition**, paragraphs numbered `P1..Pn`.
- **The candidate set**: a request field `candidates` (title, dek, route,
  version, and text of the top ten) sent by the phone as owner-provided data
  **[doc 2.2, 2.3, slice 2 fallback]**. Reading the front list from the guest
  replaces it later; the validator and the input do not change.
- **Marked cases**: the last 40 forecasts by commit time, each `{stretch,
  segment, kind, title, p_heard, caused_by, outcome}` with outcome `heard`,
  `not heard` (and where a cut fell) or `open`. Nothing else: no band table,
  no counts, no rate, no total, no streak. A band's count is a finish rate (the
  lowest band is the finish rate), and the DJ would climb it. Older cases are
  not shown; the lessons carry what they taught, and the listener's notebook
  keeps all of them.
- **Active lessons** (id, text, cites; no tally). At most 30, the most
  recently written or cited; the others are dormant, shown to the listener
  only.
- **Struck lessons**: id and text, so the DJ does not write them again.
- The last stretch's titles, and the cause and local time.

No sentence tells the DJ which way to move. "You are over-confident" invites a
uniform shift, which loses the little discrimination there is (learning
engine, section 9).

### 6.2 Writing it down

Lessons arrive as lines in the same response, validated as in 3.2, and are
processed after the segments (so a same-response cite fails). Each valid lesson
becomes a record `autoradio-lesson:L<n>` (`record_kind` `autoradio.lesson`,
held by the Autoradio desk, written by the programme handler, the first writer
of the kind: red). `<n>` is the next integer for the listener. A lesson:

- cites at least two air events or one instruction, and is about what to play
  for this listener, not a description of them;
- may replace one of the DJ's own lessons; the trail stays (`replaces`);
- has a tally, computed by the notebook route, never written by the DJ.

**The tally.** For lesson `L`: of the forecasts tagged `ref:L` and
`cause:lesson`, how many settled heard and how many not, plus open and lapsed.
It is read from the claim tags. It measures what the DJ said caused a segment,
not what did. Lineage: a replacing lesson starts at zero and the notebook shows
the chain side by side (not summed); a struck lesson's tally freezes; clear
does not change tallies. The DJ does not see tallies. The listener does.

### 6.3 The notebook

The notebook is the listener-facing record of what Autoradio holds about them
(rule 2). `GET /api/autoradio/notebook` returns, in sections with cursors so
nothing is cut off:

- `lessons`: every lesson, active, replaced, struck, dormant, with cites
  resolved to events and tallies;
- `forecasts`: every forecast with title, kind, `p_heard`, cause and outcome
  (heard, not heard, open, lapsed);
- `air`: the full air log, cleared events included, marked cleared;
- `turns`: every turn record with the listener's words and the radio's reply
  **[code: `api_autoradio.go:360-425`]**;
- `instructions`: corrections written in the notebook;
- `record`: the calibration view from `calibration.Summary` with
  `agent=autoradio-dj`, `group_by=project`, extended with the causal base-rate Brier
  (section 5; no second scoring path);
- `held_elsewhere`: the operational traces named and where to read them: the
  model call traces hold the assembled prompt, which includes instructions and
  marked cases. They are owner-readable by the existing trace surfaces and are
  not part of the notebook's data.

The listener can **strike** a lesson (the DJ is told it was struck),
**correct** one in their own words (an instruction, authority 1) and **clear**.
Strike and clear are visibility acts: each appends a record
(`autoradio.strike`, `autoradio.clear`) and deletes nothing. Clear tells the
DJ, and the dedupe, to ignore air events and forecasts before it (the listener
asked to start over, so a repeat is expected); the notebook still shows them,
marked cleared, because rule 2 never lets the listener lose sight of what is
held. Erasure is not offered here; adding it is a black change and an owner act.

The phone's local `signals.jsonl` is compacted after ack (4.2). On the phone
this replaces "What the paper has learned" **[code: `LearnedFromPreferences.swift`]**,
which lists local gestures today. The notebook's home is an owner decision with
a default (goal file, D2): the desk-held record kinds above; moving the notebook
onto the edition page waits for hand-offs. The air log and the notebook never
leave the computer and are never aggregated across listeners **[doc 3.4]**.

## 7. Trust

| What is handed over | Today | Who decides |
|---|---|---|
| Wider bounds for `explore` (picks outside the stated aims) | none designed | the listener, in words |
| Write-ahead: a stretch written before the listener asks | slice 6 | the listener; it spends money, so it is an owner act |
| A standing station on a schedule | slice 6 | the same |
| Reading mail headers | slice 3 | the same, by lease |

**[new]** Each of those prompts shows the listener the record at the moment of
asking: "Of 40 segments I expected you to finish, you finished 31. You
corrected me twice this week." The listener reads it and decides. Nothing
widens by itself; a rule that widens latitude from the record alone waits until
there is a record worth reading. The DJ sees its record as marked cases (6.1);
it does not see the sentence above.

## 8. The junction with Autopaper

The edition is stated. The notebook is learned. They meet in one act:

> "You have skipped sport nine times this week. Take it out of your edition?"

The DJ proposes; the listener says yes or no; on yes, Autoradio hands the edit
to Texture, which writes the edition **[doc 3.2]**. A learned lesson becomes a
stated aim only by the listener's word. Autopaper reads the edition nowhere
today **[code: `internal/frontimportance/scorer.go`]**; making it read the
edition is the next mission.

## 9. What this changes in the programme design

| Programme design | Now |
|---|---|
| 4.4: one record per stretch, one typed question per segment | One claim per content segment (3.3); the stretch record is an index. |
| 4.4: the desk sees its settled outcomes "under the acting-pack boundary" | The DJ sees marked cases (6.1). Rule 2 replaces the boundary. |
| 2.2 `heard` field on the programme request; 3.4 batch rides the next request | Removed. Only `POST /api/autoradio/heard` writes air events (4.1). Request gains `candidates` (6.1). |
| 2.6 dedupe on "heard to its end or past half" | Named **covered** (4.2); Brier uses **heard through**. |
| Slice 6 carries scoring, after the station | Forecasts ship with the programme call (slice 2), settlement with the air log (slice 4). The station stays in slice 6. |
| Slice 4: dedupe, System 1 resume question, `redirect` | Air log, route and dedupe here. The resume question and `redirect` are not part of this mission (goal file, excluded). |
| 4.1: the stretch pool is River, then Synthetic, then Devin | River, then Bedrock Haiku 5.5, the Autoradio pool **[code: `internal/modelcatalog/catalog.go`, owner 2026-11-11]**. The role `autoradio_programme` is registered like `autoradio` in `NormalizeRole`, reasoning on, own prefix; the comment calling it "future" **[code: `internal/modelpolicy/model_policy.go:58`]** is changed with it. |
| Slice 2 review seats | The seats in `CLAUDE.md`, "Review", as the owner set them (goal file, owner_rulings). |

## 10. Order of building

Each station is proved on d@e before the next starts.

1. **Reserved namespace and forecast store** (4.3, 3.3), plus the wire fixtures
   (section 12). Red.
2. **Programme call with forecasts** (slice 2 plus 3). Red.
3. **Air log, heard route, settlement** (slice 4's air log plus 4). Red.
4. **Paper back and lessons** (6.1, 6.2). Red: first writer of lesson records,
   and a change to a provider call's input.
5. **Notebook route** (6.3) with the record (section 7). Orange.
6. **The proposal to change the edition** (section 8), after hand-offs.

Steps 2 and 3 give a DJ that forecasts and is marked. Step 4 is where it
learns.

## 11. What blocks it on 2026-11-14

1. **River is not registered on the gateway.** The pool then answers from
   Bedrock, which does not stream; the first segment arrives only when the
   whole stretch is written. No latency claim is made until River is
   registered ([reference/RIVER.md](../reference/RIVER.md), section 5, an owner
   act); the programme design's "first segment at most 6 s median" waits for
   it.
2. **The phone install.** Until the owner says go (D3), the phone proof is the
   simulator and a script against d@e.
3. **Deploy headroom on Node B.** A deploy was refused over disk on
   2026-11-11. Every server step needs a deploy.
4. **The edition writer on the phone is a stub** **[code: `Interview.swift`,
   `UnbuiltEditionWriter`]**. Section 8 needs it or the Texture hand-off.

## 12. Records and wire shapes (the one path)

| Record | Id | Written by | Read by |
|---|---|---|---|
| forecast claim | `calib:autoradio-dj:<hash of stretch/segment>` | programme handler, before the segment is sent | heard handler, notebook, input builder |
| settlement | `<claim id>:settle:rule:autoradio_heard` | heard handler only | the same, `calibration.Summary` |
| stretch index (`autoradio.stretch`) | `autoradio-stretch:<stretch_id>` | programme handler at `done` | notebook |
| air event (`autoradio.air`) | `autoradio-air:<event_id>` | heard handler only | the same |
| lesson (`autoradio.lesson`) | `autoradio-lesson:L<n>` | programme handler, from validated lines | notebook, input builder |
| strike, clear (`autoradio.strike`, `autoradio.clear`) | minted | notebook route | input builder, notebook |
| correction (`autoradio.instruction`) | `note/<id>` | notebook route | input builder, notebook |

All are commitment records with `desk=app:autoradio` and a `record_kind`
subject ref, listed by subject ref **[code: `internal/store/commitment.go:122`
`ListCommitmentRecordsBySubjectRef`]**. Air events carry `route=heard` and the
authenticated actor the handler saw.

Fixtures for the builders (L1a delivers them; P1 and P2 consume them): one
stretch of lines with valid and invalid cases for each validator in 3.2; one
heard batch with `finished`, `skipped`, `cut`, `cut` then `resume`, a duplicate,
a conflicting `event_id`, an unknown segment and a hash mismatch, with the
expected result codes and settlements.

## 13. Not determined

- Whether the guest can read the front list itself. Plan A sends `candidates`
  from the phone, so nothing waits on it.
- Whether DeepSeek V4.1 Flash writes useful lessons. Observed, not accepted
  (goal file, observations).
- How many settled segments a lesson needs before it helps. Two cites is a
  floor to stop invention, not a finding.
- Whether 40 marked cases and 30 lessons fit the model's useful context. The
  numbers are prompt bounds; the first stations measure the assembled size.
