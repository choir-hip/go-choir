# Autoradio on the four-stroke learning engine (2026-11-14)

Standing: superseded | kind: decided | scope: in part | settled-by: this file's header (line 5) and the successor's line 6 | replaced-by: docs/design/autoradio-learned-document-2026-11-17.md | note: the sections named in the header

> **Superseded in part, 2026-11-17**, by
> [autoradio-learned-document-2026-11-17.md](autoradio-learned-document-2026-11-17.md)
> (its section 1). Superseded here: 3.1's `edition` and `instruction:note`
> causes; 4.1 to 4.2 and 12.3 to 12.4 (the heard route, air events and their
> classes; the run sheet op log is now the air log); section 5's guard 1 as
> it applies to per-lesson counts; 6.2 to 6.3 (lesson records as the
> notebook, the notebook route); section 8 (the edition becomes part of the
> notebook); and the literals of 12.1 to 12.7 that name those. The rest stands.
>
> What replaced them was built, and the one record kind it
> adds (`autoradio.no_verdict`), is in
> [AUTORADIO-SHEET.md](../reference/AUTORADIO-SHEET.md) "Desk entries,
> settlement and the learning route" and
> [AUTORADIO-PROGRAMME.md](../reference/AUTORADIO-PROGRAMME.md) "On the run sheet".

How the DJ learns one listener. This is the first place the
[four-stroke learning engine](../learning-engine.md) is built whole: forecast,
outcome, lesson, trust. The goal file that runs it is
[choir-autoradio-learning-engine-2026-11-14.md](../definitions/choir-autoradio-learning-engine-2026-11-14.md).

It changes the programme design
([autoradio-dj-programme-2026-11-11.md](autoradio-dj-programme-2026-11-11.md))
in the places listed in section 9. Everything else in that design stands:
stretches, segments, hand-offs, the slices.

Revised after the station L0 review (four rounds). L0 is closed for the scope the first builds need (stations L1a and L1, and P1 on fixtures); the sections for L2, L3, L4, P2 and P3 are re-reviewed by each station's seats at its start, with the round 4 findings as inputs. Tags: **[code]** read from
code; the Go tree is as at `origin/main` 955d4527f, and no Go file has changed
on main since (it has moved in `mobile/` and in three docs files); **[doc]**
from the programme design; **[new]** decided here. Line numbers drift.

**Vocabulary.** Section 12 is normative and is the only place that defines the
claim tags, record kinds and labels, event types and result codes, outcome
classes, id and ref grammars, the hash comparison and the bounds. Every other
section and every station acceptance refers to them by table name, for example
"the claim tags (12.1)", and does not restate the literals. Where prose and
section 12 differ, section 12 wins.

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
two fields; the handler, not the model, mints the segment id and any lesson id
(12.5).

```json
{"kind":"story","title":"...","text":"...","cites":["pub:..."],
 "p_heard":0.7,"caused_by":"lesson:L12"}
```

`ident`, `bridge`, `answer` and `result` carry no forecast; a forecast field
on them drops the line. `p_heard` is the DJ's probability that this listener
hears the segment through (section 4).

`caused_by` names the one input that most decided the pick, as a ref (12.5).

| Cause class | Meaning | Authority **[doc 2.6]** |
|---|---|---|
| `instruction` | the listener said it, in a turn or in the notebook | 1 |
| `edition` | a numbered paragraph of the edition text, numbered by the handler in the input | 2 |
| `importance` | the paper's importance score or mail trust (no ref) | 3 |
| `lesson` | an active lesson in the notebook shown to this call | 4 |
| `explore` | none of the above; the DJ is trying something (no ref) | 4 |

`explore` is a cause label only. There is no room-size setting. An `explore`
segment still cites the candidate set and may not contradict an edition aim or
an instruction; widening what the DJ may try is a later owner act (section 7).

### 3.2 Validators

Every validator drops the line (a stretch line, not the call) and notes the
reason in the stretch's `timings`; a run of three invalid lines, of any kind,
ends the call **[doc 2.3]**. A dropped line is never streamed and has no
forecast, so it can neither be settled nor count as unforecast.

| Check | Failure |
|---|---|
| kind known; text non-empty; length in the kind's range; every cite in the candidate set | drop (as the programme design) |
| content kind has `p_heard` and `caused_by` | drop |
| `title` one line, no control character, at most 200 bytes (a claim write refuses control characters and long text, **[code: `records.go:129`]**, and a failed write ends the call) | drop |
| `p_heard` a finite number in [0, 1] (endpoints are allowed and scored as stated) | drop |
| `caused_by` is a ref of the grammar in 12.5 and resolves against this call's input: a lesson ref names an active lesson; a turn ref names a turn whose request record **[code: `api_autoradio.go:282`]** is listed (a failed or interrupted turn counts: the words were said); a note ref names a listed notebook correction; an edition ref names a numbered paragraph | drop |
| a segment citing a lesson written in the same response | drop (the lesson is not in the input) |
| lesson line: text one line, within the bound of 12.7; cites follow 12.5, at least two air events or one instruction, every one listed in this call's input (6.1 lists an air event as the settling event of a marked case, or as a gesture or correction event); a case of class open, lapsed, abandoned or stopped late has no settling event, so nothing about it can be cited | drop |
| lesson line: `replaces` names an existing active lesson of this listener; a struck or unknown id | drop |
| lesson line whose every cite is already cited by a struck lesson (their cites are shown to the DJ, 6.1) | drop (a struck lesson is not rewritten from the same evidence) |
| a lesson id or segment id sent by the model | discarded; the handler mints them |

Lesson lines are processed after the segments, so an invalid lesson line is dropped and logged and never counts toward the run of three; it cannot end the call before the stretch index is written. A failed claim write is not model invalidity. The handler ends the call with an
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
| `criterion` | `caused_by=<the full ref>` (up to 2000 bytes), so an instruction or edition ref is never a tag |
| `idempotency_key` | the stretch id and segment id joined by `/`. The id is `calib:autoradio-dj:<hash>`, deterministic, so a replay writes nothing new **[code: `ClaimRecordID`, `records.go:185`]** |
| `project` | the segment kind, so the existing `group_by=project` stratifies by kind **[code: `ParseGroupBy`, `handler.go:379`]** |
| `deadline` | the stretch's start time plus the lapse bound (12.7), the same for every segment of the stretch; past it the existing status `overdue` marks the forecast lapsed (12.4) |
| `source_ref` | `autoradio:` and the stretch and segment ids joined by `/` |
| `tags` | the claim tags (12.1) |
| record kind and label | 12.2 |

The tags are set at commit time and never edited (claims have no edit route).
The tally and the heard handler read them there. The builder checks tags with
the same `cleanName` and length test `NewClaimRecord` uses **[code:
`records.go:142,268`]**; the id grammars (12.5) keep every tag within 64 bytes.
The constructor sets the `desk` subject ref, the record kind and the label,
which `NewClaimRecord` does not set itself.

**Stretch start and retry.** The stretch's start time is written once, in the
stretch start record (12.2), before any claim; the deadline derives from it, so
a replay of a claim is byte-identical **[code: `SameClaim` compares the whole
claim, `records.go:319`]**. Each segment's text is written in a segment record
(12.2) before its claim, so the server holds every streamed word. A crash
closes the round "interrupted by a restart" and nothing resumes **[CLAUDE.md]**.
A retry is a new call with a new `request_id` and a new stretch id; it never
re-streams a claimed segment id, so the hash tag and the text the phone holds
cannot diverge. A request record (12.2) maps the `request_id` to its stretch id, written with the stretch start; a replay of the same `request_id` returns that stretch from its segment records and makes no model call. Every claim write compares the stored claim with `SameClaim` after the append, as the generic handler does **[code: `handler.go:213-226`]**; a mismatch ends the call with an error and nothing is streamed.

The **stretch index** (12.2) has the claim ids as `related_ids` and a `status`
(12.2). The handler writes it whenever it itself ends the call, at `done` or at
another ending (12.2 lists them); a reader must never treat "an index exists"
as "completed" and must read `status`. A stretch whose process stops, or whose
claim write fails, has a start record and no index, and keeps its claims and
texts for the segments already streamed; those settle as usual. A replay of a
`request_id` streams only the leading run of segment records whose claim exists
(a content segment is never streamed without its forecast, 3.4). A stretch's
forecasts are listed by the tag query (4.3), never from the index.

### 3.4 The order is the guarantee

For each valid line, in this order: append the segment record, append the
claim, then send the `segment` event. The forecast is committed before the
segment reaches the phone, so no event can precede its forecast. On the Bedrock
fallback (no streaming, section 11) the handler validates and appends line by
line, then sends the segments in order; the same order holds. The `segment` event carries `block_count`, the
server's count by the split function of 12.3, so the phone never recomputes it.

## 4. Outcome

### 4.1 One writer

`POST /api/autoradio/heard` is the only writer of air events and the only
caller of the reserved settle method (4.3). The programme request has no
`heard` field and no other path to either; the decoder refuses unknown fields,
so a request carrying `heard` answers 400. The phone no longer batches events
on the programme request (a change to **[doc 2.2, 3.4]**). Two tests hold this
(goal file): the programme call with a provider that tries to emit an event or
a settlement, and a call-site test that the settle method has one caller.

### 4.2 The heard rule: deterministic given receipt order

The event fields, types, checks and result codes are in 12.3. In words:

**Heard through.** A segment is heard through when its final block began
playing. This is the one notion that scores `p_heard`. The programme design's
dedupe test, "to its end or past half" **[doc 2.6]**, is a different notion,
named **covered** (12.3). Dedupe uses covered; Brier uses heard through.

**Settlement.** A settlement is evidence and is never rewritten. So the rule is
plain: **the first terminal event the server receives for a segment settles
it.** A batch is first sorted by stretch, segment and sequence, so one batch is
normalised; across batches the order is receipt order. The rule is therefore
deterministic given receipt order, and idempotent under replay of the same
events; it is not independent of which batch arrives first. The settling event
settles yes when it is a finish, or a skip with the final block begun, and no
when it is an earlier skip. The settlement id is one per segment
(`<claim id>:settle:rule:autoradio_heard` **[code: `SettlementRecordID`]**) and
the append is conditional on not existing, so when two batches race, whichever
append wins keeps its settlement. `AppendCommitmentRecord` returns success when
the record already exists **[code: `internal/store/commitment.go:47-50`]**, so
the loser cannot tell it lost: after every conditional append the handler
re-reads the record and compares it, as the generic handler does **[code:
`handler.go:213-226`]**. Evidence is the settling event's `air` ref
(12.5).

**Partial writes.** The air record is appended before the settlement. If the
process stops between them, a replay of the event finds it stored and answers
`duplicate`; on `duplicate` the handler checks whether the segment has a
settlement, and if none exists it applies the rule to the stored events and
settles. The fixture holds an air record with no settlement, then a replay.
The heard handler finds the claim by its deterministic id (a point read); only
the notebook and a stretch's listing use the tag query.

**Later events.** After a segment is settled each event gets exactly one of
three codes (12.3): a repeat of a stored event is `duplicate`; a later terminal
event that disagrees with the settlement is `conflict-recorded`; anything else
is `ignored`. A conflict is stored as a conflict record (12.2): the notebook's
air log shows it beside the settlement (rule 2), and the DJ is not shown it as
an outcome. A replay of a finished segment changes nothing.

**Refusals and bounds.** An event is refused for the reasons in 12.3. A batch
over the bound of 12.7 is refused whole, with HTTP 400 and nothing stored; the
phone sends chunks within the bound.

**Lapse, classes and coverage.** A segment with no terminal event is open until
its deadline and then falls into one of the other classes of 12.4. In the
**primary reading** only settled segments enter the Brier; the rest are counted,
and coverage is settled over forecast, read from the existing summary
**[code: `GroupStats`, `math.go:100`]**. Late reports are allowed at any time
and settle normally; a call's input is a snapshot at call time. A **second,
stated reading** (12.4) adds the abandoned and stopped-late classes, so
abandonment is not silently dropped. It is computed beside the summary by
joining the claims with the air events, with the same `calibration.BrierScore`;
nothing is written for it.

**Ack and retention.** Every event in the request gets a result code. The phone
deletes an event from its local signals file only on `accepted`, `duplicate`,
`ignored` or `conflict-recorded`; it keeps `refused` and `changed` events and
surfaces them, since a refusal is a defect to read, not an acknowledgement. It
keeps the rest **[code: `mobile/exact-package/apple/AutoradioModule.swift:99`]**. The
local file holds only what is not yet acknowledged. The phone synthesises
`left` at the next launch, since a killed app cannot send it.

### 4.3 Rule 1 at the generic route

The generic claims routes today let any key with `precommit:write` create a
claim and settle any claim with any `rule` or `owner` method; a `rule` name
only needs to be non-empty; and the earliest rule settlement governs
**[code: `handler.go:187,261`, `records.go:398`, `math.go:49`; the device key
holds all scopes, `internal/auth/device_login.go:415`]**. By itself that lets a
phone settle its own forecast. So the Autoradio namespace is reserved:

- the generic `create` refuses an agent whose cleaned, slugged name begins
  `autoradio` and any tag beginning `autoradio` (case-insensitive);
- the generic `settle` and `get` load the claim and test its **stored** agent
  and tags, never the request or the id prefix (`ClaimRecordID` hashes the
  cleaned name, so `Autoradio-DJ` gets another id than `autoradio-dj` yet slugs
  to the reserved agent); all three answer 403;
- list and summary exclude reserved claims before the record limit, so one
  listener's day cannot evict another agent's calibration from the join
  **[code: `DefaultMaxRecords` 10000]**. `JSONFieldMatch` has only an exact
  value **[code: `internal/objectgraph/sql_store.go:369`]**, so L1a adds two
  store queries: calibration records excluding an agent prefix, and claims by
  tag (a stretch's forecasts). Neither can be done by filtering the returned
  slice;
- the heard handler settles through a store method that accepts only a rule
  name from a closed registry (`autoradio_heard`, bound to agent
  `autoradio-dj`) and only a claim whose stored agent is exactly
  `autoradio-dj` with the server-written tags (12.1). The settlement's
  provenance is its evidence: the `air` ref names an air event the heard route
  wrote (the air record carries the route and the actor); the acceptance reads
  that and the one-caller test, never a scorer-name string. Nothing else can
  write that rule, and a forged `rule` settlement cannot reach a reserved
  claim;
- the notebook and the measurement read reserved claims through the same
  calibration code with `agent=autoradio-dj`, extending `calibration.Summary`;
  there is no second scoring path. Their reads are not capped at 10000 records
  (claims and settlements together, about 60 an hour of listening): they page
  by cursor until exhausted, and a response that stops early says `truncated`,
  never silently.

Not done, on purpose: refusing `rule` and `owner` methods wholesale from API
keys. The CLI uses them for ordinary claims **[code: `cmd/choir/precommit.go:72`]**,
and the generic handler cannot tell a key from a session. The reservation
closes the Autoradio case, which is the only one the DJ reads. This hardening
is a **red** change (auth and evidence) and lands in station L1a before any
reserved claim is written.

### 4.4 Gestures and corrections

Signal events (12.3) go to the same route. A signal may name a stretch and
segment, and then the forecast check of 12.3 applies; with none it is stored
for the session. A correction carries the `turn_id` of its turn record. They never settle `p_heard`. They are
evidence about the pick and reach the lesson stroke as air refs (gestures) or
instruction refs (corrections).

## 5. What the DJ is scored on, and what it is for

- **Calibration**: was `p_heard` honest. Strict Brier over forecasts, with
  coverage beside it **[code: `BrierStrict`, `math.go:112,240`]**.
- **Reliability**: did the programme serve the edition and the instructions.
  Its evidence is corrections, gestures and edition changes.

**The measurement (ARL-C1 only).** The summary's `BaseRate` is the in-sample
mean outcome **[code: `math.go:236`]**, which a DJ that merely restates the
base rate would match. The reading therefore adds a causal base rate to
`GroupStats`: for each forecast, the pooled mean outcome of all forecasts
settled before it was committed, shrunk per kind toward that pooled rate with
the weight of 12.7 (0.5 until one is settled), and the Brier of that base rate.
Pooling stops a rare kind from being compared with a fixed 0.5. The reading is,
per segment kind (`group_by=project`) and per block-count class (12.7): `n`, coverage, strict Brier of
`p_heard`, strict Brier of the causal base rate, their paired mean difference
and its standard error; for the first 100 settled and again for the second 50
against the first 50, and with the per-lesson tallies of 6.2 beside it. The
tally measures what the DJ said caused a segment, not what did. This is the
primary reading; the second reading of 4.2 is reported next to it, labelled.
The reading settles ARL-C1 and nothing else: not the mission's value, not the
lessons' truth, and not whether the notebook caused the gain (the
empty-notebook replay is a probe, not a control). The DJ chooses how it
paragraphs a segment, and a one-block segment is heard through as soon as it
begins, so the block-count strata keep easy short segments from flattering the
reading.

Hearing through is the forecast target, not the objective. Guards:

1. The DJ is never shown a finish rate, a count, a total, a streak or a band
   table. It is shown marked cases (6.1). That guards what is shown, not
   arithmetic: 40 cases let a model count a rate by hand.
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
  ref (12.5) and the words verbatim, authority 1. A corrected lesson is stored
  as an instruction in the listener's words; the DJ never rewrites it.
- **The edition**, paragraphs numbered for the edition refs (12.5).
- **The candidate set**: a request field `candidates` sent by the listener's
  phone as world data, fenced as such, shaped and bounded by 12.7. The cite
  validator trusts this set, which is acceptable for the listener's own phone
  **[doc 2.2, 2.3, slice 2 fallback]**. Reading the front list from the guest
  replaces it later; the validator and the input do not change.
- **Marked cases**: the last forecasts by commit time (bound in 12.7), each
  `{stretch, segment, kind, title, p_heard, caused_by, outcome, air}`, rendered
  by outcome class (12.4): a settled case shows the outcome and the settling
  event's `air` ref; an open case shows `open`; a lapsed case shows `not
  played`; an abandoned or stopped-late case shows `stopped at block i of n`.
  Only a settled case carries an id, because nothing else can be cited (3.2). A
  conflict is never shown. Nothing else: no band table, no counts, no rate, no
  total, no streak. A band's count is a finish rate (the lowest band is the
  finish rate), and the DJ would climb it. Older cases are not shown; the
  lessons carry what they taught, and the listener's notebook keeps all of
  them.
- **Gesture and correction events** since the last clear, each with its `air`
  ref, so they can be cited.
- **Active lessons** (id, text, cites; no tally), bounded by 12.7, the most
  recently written or cited; the others are dormant, shown to the listener
  only.
- **Struck lessons**: id, text and cites, so the DJ does not write them again
  and can see why a line was dropped.
- The last stretch's titles, and the cause and local time.

No sentence tells the DJ which way to move. "You are over-confident" invites a
uniform shift, which loses the little discrimination there is (learning
engine, section 9).

### 6.2 Writing it down

Lessons arrive as lines in the same response, validated as in 3.2, and are
processed after the segments (so a same-response cite fails). Each valid lesson
becomes a lesson record (12.2), held by the Autoradio desk and written by the
programme handler, the first writer of the kind: red. The lesson number is the
next integer for the listener, minted by compare-and-append: read the highest
number, append the next, re-read and compare, and on a different record try the
next number, so two overlapping calls never share an id. A lesson:

- cites at least two air events or one instruction, and is about what to play
  for this listener, not a description of them;
- may replace one of the DJ's own lessons; the trail stays;
- has a tally, computed by the notebook route, never written by the DJ.

**The tally.** For a lesson: of the forecasts carrying that lesson's tally tags
(12.1), how many settled heard and how many not heard, and how many fell in
each other class (12.4). It is read from the claim tags. It measures what the
DJ said caused a segment, not what did. Lineage: a replacing lesson starts at
zero and the notebook shows the chain side by side (not summed); a struck
lesson's tally is as of the strike (forecasts settled by then; later reports do
not move it and are shown separately); clear does not change tallies. The DJ
does not see tallies. The listener does.

### 6.3 The notebook

The notebook is the listener-facing record of what Autoradio holds about them
(rule 2). `GET /api/autoradio/notebook` returns, in sections with cursors so
nothing is cut off:

- `lessons`: every lesson, active, replaced, struck, dormant, with cites
  resolved to events and tallies;
- `forecasts`: every forecast with title, kind, `p_heard`, cause, outcome class
  (the enum of 12.4) and the segment's text, which the server holds (3.3);
- `air`: the full air log, cleared events included, marked cleared, and
  conflicts shown beside the settlement they disagree with;
- `turns`: every turn record with the listener's words and the radio's reply
  **[code: `api_autoradio.go:360-425`]**;
- `instructions`: corrections written in the notebook;
- `record`: the calibration view from `calibration.Summary` with
  `agent=autoradio-dj`, `group_by=project`, extended with the causal base-rate
  Brier (section 5; no second scoring path);
- `held_elsewhere`: the operational traces named and where to read them: the
  model call traces hold the assembled prompt, which includes instructions and
  marked cases, and the model's response. They are owner-readable by the
  existing trace surfaces and are not part of the notebook's data.

The listener can **strike** a lesson (the DJ is told it was struck),
**correct** one in their own words (an instruction, authority 1) and **clear**.
Strike and clear are visibility acts: each appends a record (12.2) and deletes
nothing. A notebook correction is a note record (12.2). Clear tells the DJ, and
the dedupe, to ignore air events and forecasts before it (the listener asked to
start over, so a repeat is expected); the notebook still shows them, marked
cleared, because rule 2 never lets the listener lose sight of what is held.
Erasure is not offered here; adding it is a black change and an owner act.

Every autoradio record kind carries its owner-private label (12.2); a test
asserts that no other read path (generic claims, commitment listings by
addressee, other desks) returns them. The phone's local signals file is
compacted after ack (4.2). On the phone this replaces "What the paper has
learned" **[code: `LearnedFromPreferences.swift`]**, which lists local gestures
today. The notebook's home is an owner decision with a default (goal file, D2):
the desk-held record kinds of 12.2; moving the notebook onto the edition page
waits for hand-offs. The air log and the notebook never leave the computer and
are never aggregated across listeners **[doc 3.4]**.

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
| 4.4: `cut` then `resume` settles nothing; `left` settles as interrupted | A finish, or a skip with the final block begun, settles yes; an earlier skip no; cut, pause and left leave the forecast open until it lapses (4.2, 12.3, 12.4). |
| 4.4: the desk sees its settled outcomes "under the acting-pack boundary" | The DJ sees marked cases (6.1). Rule 2 replaces the boundary. |
| 2.2 `heard` field on the programme request; 3.4 batch rides the next request | Removed (2.3's run of three invalid lines stands). Only `POST /api/autoradio/heard` writes air events (4.1). Request gains `candidates` (6.1, 12.7). |
| 2.6 dedupe on "heard to its end or past half" | Same threshold, named **covered** (12.3); Brier uses **heard through**. |
| Slice 6 carries scoring, after the station | Forecasts ship with the programme call (slice 2), settlement with the air log (slice 4). The station stays in slice 6. |
| Slice 4: dedupe, System 1 resume question, `redirect` | Air log, route and dedupe here. The resume question and `redirect` are not part of this mission (goal file, excluded). |
| 4.1: the stretch pool is River, then Synthetic, then Devin | River, then Bedrock Haiku 5.5, the Autoradio pool **[code: `internal/modelcatalog/catalog.go`, owner 2026-11-11]**. The role `autoradio_programme` is registered like `autoradio` in `NormalizeRole`, reasoning on, own prefix; the comment calling it "future" **[code: `internal/modelpolicy/model_policy.go:58`]** is changed with it. |
| Slice 2 review seats | The owner's ruling of 2026-11-15 (DeepSeek V4.1 Flash and Sonnet while the Gemini seat is out of quota); otherwise the seats in `CLAUDE.md`, "Review". |

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
learns. Fine detail that section 12 does not fix is settled by each station's
failure list, written before its tests, under that station's own review.

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

## 12. Vocabulary (normative)

Every literal below is defined here and nowhere else. Sections 3 to 6 and the
goal file refer to these tables by number.

### 12.1 Claim tags

Written by the forecast constructor at commit time (3.3); at most 20 tags of at
most 64 bytes; never edited. A claim of the Autoradio namespace carries exactly
these.

| Tag | Grammar | Written when | Read by |
|---|---|---|---|
| `autoradio` | exact | always | the reservation (4.3), list exclusion |
| `stretch:<stretch_id>` | id of 12.5 | always | the stretch's forecast query, the notebook |
| `seg:<segment_id>` | id of 12.5 | always | the heard handler, the notebook |
| `kind:<kind>` | a content segment kind | always | the notebook |
| `cause:<class>` | `instruction`, `edition`, `importance`, `lesson` or `explore` | always | the notebook, the tally |
| `lesson:<lesson_id>` | lesson id of 12.5 | only when the cause class is `lesson` | the tally (6.2) |
| `sha:<32 hex>` | the first 32 hex digits of the segment text's sha256 (12.6) | always | the heard handler |
| `blocks:<n>` | decimal integer, at least 1 | always | the heard handler |

The tally tags are `cause:lesson` together with the `lesson:<lesson_id>` of the
lesson; both are matched exactly. The full ref is in the claim's `criterion`
(3.3), never in a tag.

### 12.2 Record kinds and labels

All are commitment records with `desk=app:autoradio`, the `record_kind` subject
ref shown, and the owner-private label `types.OwnerPrivateLabel(<label>)`
**[code: `api_autoradio.go:348`, label `autoradio_turn`]**, listed by subject
ref **[code: `internal/store/commitment.go:122`]**. The label is `autoradio_`
plus the word after the dot in `record_kind`. The L1a test iterates this table.

| `record_kind` | Record id | Written by | Read by |
|---|---|---|---|
| `autoradio.turn` (exists) | `autoradio-turn-req:<turn_id>` and the reply records | the turn route | notebook, input builder |
| `autoradio.request` | `autoradio-request:<request_id>` | programme handler, with the stretch start | programme handler (replay) |
| `autoradio.forecast` | `calib:autoradio-dj:<hash of stretch and segment>` | programme handler, before the segment is sent | heard handler (point read), notebook, input builder |
| `autoradio.settlement` | `<claim id>:settle:rule:autoradio_heard` | heard handler only | notebook, input builder, `calibration.Summary` |
| `autoradio.stretch_start` | `autoradio-stretch-start:<stretch_id>` | programme handler, before any claim | notebook |
| `autoradio.segment` | `autoradio-seg:<stretch_id>/<segment_id>` | programme handler, before its claim | heard handler, notebook |
| `autoradio.stretch` (the index) | `autoradio-stretch:<stretch_id>` | programme handler when it ends the call itself, with a `status` (below); not after a crash or a failed claim write | notebook |
| `autoradio.air` | `autoradio-air:<event_id>` | heard handler only (it carries the route and the actor) | heard handler, notebook, input builder, dedupe |
| `autoradio.conflict` | `autoradio-conflict:<event_id>` (the later event's id) | heard handler only | notebook |
| `autoradio.lesson` | `autoradio-lesson:<lesson_id>` | programme handler, from validated lines | notebook, input builder |
| `autoradio.strike`, `autoradio.clear` | `autoradio-strike:<note_id>`, `autoradio-clear:<note_id>` | notebook route | input builder, notebook |
| `autoradio.note` (a correction) | `autoradio-note:<note_id>` | notebook route | input builder, notebook |

The index's `status` is one of: `completed` (the model ended, or the segment
cap of 12.7 ended the stretch; the replay answers `done`); `invalid_run` (three
invalid lines in a row); `failed` (the model failed after a segment, every
pool member refused, or every member ended cleanly with no valid segment); `deadline` (the wall-clock or the idle deadline of 12.7).
Its refs also carry `dropped_count` (exact), `dropped` (the first 100 reasons)
and `client_gone` (`true` when a write to the client failed, so a reader can tell
"never delivered" from "delivered, not played"). Only `completed` is a finished
stretch.

### 12.3 Air events: fields, types, checks, result codes

Event fields: `event_id`, `session_id`, `stretch_id`, `segment_id`,
`text_sha256` (12.6), `seq`, `type`, `last_block`, `block_count`, `started_at`,
`ended_at`, `cites`, and `turn_id` on a correction. `seq` is the phone's counter for that segment, monotonic
across app launches (kept in the signals file). `last_block` is the last block
that began playing, from 0.

| Event type | Class | Effect |
|---|---|---|
| `finished`, `skipped` | terminal | the first received for a segment settles it (4.2) |
| `cut`, `resume`, `paused`, `left` | not terminal | stored; they decide the class of a lapsed segment (12.4) |
| `more`, `less`, `corrected` | signal | stored; never settle; citable (4.4) |

**Blocks.** One server function, `SplitBlocks`, makes the blocks of a segment's
text: line endings are normalised to LF, the text is split on one or more blank
lines (a line of only whitespace is blank), each part is trimmed, and empty
parts are dropped. The claim's `blocks` is its count and the `segment` event
sends it. The phone's player may subdivide a block for playback (the programme
design keeps a first block to one sentence); it reports `last_block` as the
index of the server block being played. The fixture holds edge texts (a
trailing newline, CRLF, blank lines with spaces, leading blank lines, one
paragraph) with their counts.

**Checks.** The claim's `blocks` governs. Predicates used by the rules:
**final block began** is `last_block` equal to `blocks - 1`; **covered** is
`2 * (last_block + 1) > blocks`, or the final block began. An event is refused
for: an unknown `type`; an `ended_at` before `started_at`; no forecast claim the server wrote for the named stretch and segment; a hash
mismatch (12.6); a `block_count` that differs from `blocks`; a `block_count` of
zero; a negative `last_block` or one at or above `blocks`; a `finished` whose
final block has not begun; an event_id outside the grammar of 12.5. A
`skipped` at `last_block` 0 on a segment of several blocks is valid and settles
no; a one-block segment finished at `last_block` 0 settles yes.

**Result codes.** One per event, as body fields (`code`, and `reason` for a
refusal). They are not HTTP statuses; the HTTP status is 200 (a batch over the
bound is the one 400, 12.7).

| Code | Meaning | The phone |
|---|---|---|
| `accepted` | stored, and the segment was not settled when it arrived (a settling event, or any other event before settlement, including a repeat `cut` with a new id) | deletes it |
| `duplicate` | the same `event_id` with the same content was already stored | deletes it |
| `ignored` | stored; the segment was already settled, and the event is not a disagreeing terminal event | deletes it |
| `conflict-recorded` | a later terminal event that disagrees with the settlement; stored, with a conflict record (12.2) | deletes it |
| `changed` | the same `event_id` with different content; not stored | keeps and surfaces it |
| `refused` | a check above failed; `reason` names it; not stored | keeps and surfaces it |

### 12.4 Outcome classes

One class per forecast, a function of the claim, its settlement and the air
events at read time. It is the notebook's enum.

| Class | Definition | Primary reading | Second reading |
|---|---|---|---|
| `heard` | settled yes | counted, outcome 1 | outcome 1 |
| `not heard` | settled no | counted, outcome 0 | outcome 0 |
| `open` | no settlement, before the deadline | excluded, counted in coverage | excluded |
| `lapsed` | no settlement, past the deadline, no `cut`, `resume`, `paused` or `left` event: never played | excluded, counted | excluded |
| `abandoned` | no settlement, past the deadline, at least one of those events, and the final block never began | excluded, counted | outcome 0 |
| `stopped late` | no settlement, past the deadline, at least one of those events, and the final block began | excluded, counted | outcome 1 |

Coverage is settled (classes `heard` and `not heard`) over forecast. A late
report settles a lapsed, abandoned or stopped-late forecast and moves it to
`heard` or `not heard`. The DJ's marked case renders each class as in 6.1.

### 12.5 Ids and refs

| Item | Grammar |
|---|---|
| `stretch_id` | `[A-Za-z0-9][A-Za-z0-9_.-]{0,31}`, minted by the handler |
| `segment_id` | the stretch id, `.s`, the segment index; at most 40 bytes |
| `event_id`, `turn_id` | `^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`, the turn route's `autoradioTurnID` **[code: `api_autoradio.go:98`]** |
| `lesson_id` | `L` and a decimal integer, minted by the handler |
| `note_id` | `[a-z0-9]{1,32}`, minted by the notebook route; also the suffix of a strike or clear record |
| `request_id` | the turn route's grammar, as `event_id` |

Refs, used by `caused_by`, lesson cites and evidence:

| Ref | Names |
|---|---|
| `lesson:<lesson_id>` | a lesson |
| `instruction:turn/<turn_id>` | a turn's request record (12.2) |
| `instruction:note/<note_id>` | a note record |
| `edition:P<n>` | paragraph n of the edition text as numbered in the input |
| `air:<event_id>` | an air event |

The cause classes without a ref are `importance` and `explore`.

### 12.6 The hash comparison

The segment text's sha256 is 64 hex digits. The claim's hash tag holds the
first 32. An event's `text_sha256` is the full 64 digits; the check compares
its first 32 digits with the tag. The hash-mismatch fixture pins this.

### 12.7 Bounds

| Bound | Value |
|---|---|
| Lapse | 12 hours after the stretch's start record |
| Shrinkage weight for the causal base rate (5) | 10 |
| Heard batch | at most 200 events and 64 KB; the phone sends chunks of at most 200 events and 48 KB |
| Block-count classes for the reading (5) | one block; two or three; four or more |
| Programme request body | at most 256 KB, so an over-size `candidates` answers 400 from the parser, not 413 from the reader (the turn route caps at 80 KB, **[code: `autoradioMaxBody`]**) |
| `candidates` | at most 40 items, ordered by the paper's importance score, highest first, each with title, dek, route and version; the text, at most 8 KB, only on the first ten (so at most 80 KB of text); else 400 |
| Marked cases in the input | the last 40 forecasts |
| Active lessons in the input | at most 30 |
| Lesson text | one line, at most 300 characters |
| Instruction or note text in the prompt | cut to at most 1,000 bytes on a rune boundary, then marked `[truncated]` (the prompt line is at most 1,012 bytes) |
| Segments per stretch | at most 30; a further valid line is dropped with the reason "segment cap 30 reached: the stretch ended" and the stretch ends as `completed` (a bound on what the model can cause, not on calls; the prompt asks for 10 to 15) |
| Programme call deadlines | 120 seconds of wall clock for the whole walk of the pool, and 30 seconds idle between two deltas (from the first); both end the call as `deadline`. Invented numbers: deadlines, not limits on model calls |
| Segment kinds in station L1 | `story`, `update`, `headlines`, `ident`, `bridge`; `mail`, `podcast_note`, `answer` and `result` are dropped until their stations |

Prompt bounds are not limits on model calls; whether they fit the model's
useful context is open (13).

Fixtures for the builders (L1a delivers them; P1 and P2 consume them): one
stretch of lines with a valid and an invalid case for each validator in 3.2;
one heard batch for each event type and each result code of 12.3, including a
one-block finish, a first-block skip, a wrong block count, a hash mismatch (full
and first-32 forms), a second batch with a conflicting terminal event, a
repeat `cut`, an air record with no settlement followed by a replay, the edge
texts of 12.3, and a title with a control character, with the expected codes
and settlements.

## 13. Not determined

- A candidate's text can steer the model's `caused_by` to a real listed ref (an
  instruction or a lesson that exists): a forged cause on a valid claim, which
  skews the tally and the causal base rate. It cannot be validated by the
  handler; it is a residual of putting world text in the input.

- Whether the guest can read the front list itself. Plan A sends `candidates`
  from the phone, so nothing waits on it.
- Whether DeepSeek V4.1 Flash writes useful lessons. Observed, not accepted
  (goal file, observations).
- How many settled segments a lesson needs before it helps. Two cites is a
  floor to stop invention, not a finding.
- Whether 40 marked cases and 30 lessons fit the model's useful context. The
  numbers are prompt bounds; the first stations measure the assembled size.
