# Autoradio on the four-stroke learning engine (2026-11-14)

How the DJ learns one listener. This is the first place the
[four-stroke learning engine](../learning-engine.md) is built whole: forecast,
outcome, lesson, trust. The goal file that runs it is
[choir-autoradio-learning-engine-2026-11-14.md](../definitions/choir-autoradio-learning-engine-2026-11-14.md).

It changes the programme design
([autoradio-dj-programme-2026-11-11.md](autoradio-dj-programme-2026-11-11.md))
in four places, listed in section 9. Everything else in that design stands:
stretches, segments, the air log, hand-offs, the slices.

Tags: **[code]** read from code at `origin/main` 5af04baad; **[doc]** from the
programme design; **[new]** decided here. Line numbers drift.

## 1. Why Autoradio first

Owner, 2026-11-13: apply the engine first to "the auto radio personalization
learning loop", then "backport the learning engine into auto paper", then
self-development; "the connection between auto radio and auto paper is the
user's individual auto paper edition".

Autoradio is the best first site because its teacher is the best one we have.
Run the three questions of the learning engine (section 3 there):

| Question | Autoradio |
|---|---|
| Is there a teacher? | Yes. The listener finishes, skips, cuts in or leaves, within seconds of the segment. |
| Is it someone other than the student? | Yes. The listener's act arrives from the phone under the device key. The DJ's model call has no way to write it. |
| Does the student get the paper back? | Not today. This document is that part. |

A listener gives twenty to forty outcomes an hour. The whole self-development
fleet gave about 95 rounds a day.

## 2. The cycle, once

1. **Forecast.** The DJ writes a stretch. For each segment it says why the
   segment is there and how likely the listener is to hear it through.
2. **Outcome.** The phone reports what was heard. A rule, not the DJ, settles
   each forecast.
3. **Lesson.** When the DJ writes the next stretch it is shown the settled
   outcomes. It writes down what it learned, each lesson citing the outcomes
   it rests on. Lessons go in the notebook, which the listener can read and
   correct.
4. **Trust.** The notebook shows the DJ's record. The listener decides what
   to hand over next: a longer write-ahead, a standing station, room to
   choose outside the stated edition.

Call it. Check it. Learn it. Earn it.

## 3. Forecast

**[new]** Each segment in a stretch carries two more fields:

```json
{
  "segment_id": "s3",
  "kind": "story",
  "cites": ["pub:..."],
  "p_heard": 0.7,
  "caused_by": "lesson:L12"
}
```

- `p_heard` is the DJ's probability that this listener hears the segment to
  its end.
- `caused_by` names the one input that most decided the pick. Its values:

| Value | Meaning | Authority order **[doc 2.6]** |
|---|---|---|
| `instruction:<ref>` | the listener said or wrote it | 1 |
| `edition:<ref>` | a stated aim in the edition | 2 |
| `importance` | the paper's importance score, or mail trust | 3 |
| `lesson:<id>` | a lesson in the notebook | 4 |
| `explore` | none of the above; the DJ is trying something | 4 |

`caused_by` is what makes the outcome teach. When a segment is skipped, the
skip counts against the lesson that put it there, not against the DJ in
general. A forecast with no stated cause can only say "you were wrong".

The forecast record is as the programme design has it **[doc 4.4]**: one
precommitment record per stretch, one typed question per segment, so a
stretch is one record and not ten. `ident` and `bridge` segments carry no
forecast.

Line validation refuses a `story`, `update`, `headlines`, `mail` or
`podcast_note` segment without both fields, and a `lesson:<id>` that is not
in the notebook the call was given.

## 4. Outcome

The air log **[doc 3.4]** is the outcome. The phone posts playback events to
`POST /api/autoradio/heard`; each has an `outcome`. The rule **[doc 4.4]**:

| Air log | Settles the forecast as |
|---|---|
| `finished` | yes |
| `skipped` | no |
| `cut`, then `resume` and finished | yes |
| `cut`, never resumed | open; it lapses with the session |
| `paused` | nothing |
| `left` (app closed) | interrupted; not scored |

Rule 1 holds by construction, and the build must keep it so:

- **1a.** The DJ made the forecast. The settlement rule runs in the route
  handler on the listener's reported act. The DJ's model call is not asked.
- **1b.** The air log has one writer, the heard route, behind the device key.
  The programme call has no verb, field or tool that writes an air log event
  or a settlement. A test asserts this.

Two more outcomes are stronger than playback and the phone already records
both locally **[code: `mobile/AutoradioKit/.../Signals.swift`,
`LearnedFromPreferences.swift`]**:

- a **gesture**, "more like this" or "less of this";
- a **spoken correction** in a turn ("less sport", "I already know this").

They go to the same route as events with `outcome: more | less | corrected`
and the segment they were about. They do not settle `p_heard`. They are
evidence about the pick, and they reach the lesson stroke as such.

## 5. What the DJ is scored on, and what it is for

Two quantities, kept apart (learning engine, section 4).

- **Calibration** is whether `p_heard` was honest. Brier score over settled
  segments. It is shown to the DJ and to the listener.
- **Reliability** is whether the programme served what the listener asked
  for: the edition and their instructions. Its evidence is the corrections,
  the gestures and what the listener changes in their edition.

**Hearing through is the forecast target. It is not the objective.** A DJ
that optimised finish rate would become an engagement feed. Three guards:

1. The DJ is never shown a finish rate, a total or a streak. It is shown
   settled forecasts.
2. A skipped segment forecast at 0.3 is a good forecast. A challenging
   segment the listener asked for, forecast honestly low, costs the DJ
   nothing.
3. A stated aim outranks an inferred lesson **[doc 2.6]**. A lesson may not
   remove what the edition asks for; it may only propose the change to the
   listener (section 8).

## 6. Lesson

### 6.1 Getting the paper back

**[new]** The next programme call's input carries, after the stable prefix:

- the settled questions of the last stretches, per segment: kind, title,
  `p_heard`, `caused_by`, outcome, and where a cut fell;
- the gestures and corrections since;
- the notebook as it stands;
- the DJ's record: by probability band, how many forecasts and how many came
  true.

No sentence tells the DJ which way to move. "You are over-confident" invites
a uniform shift, which loses the little discrimination there is (learning
engine, section 9). The record is numbers and cases.

### 6.2 Writing it down

The stretch call returns, beside its segments:

```json
"lessons": [
  {"id": "L13", "text": "Skips match reports; listens to transfer news and injuries.",
   "cites": ["air:e41", "air:e44", "air:e52"], "replaces": "L7"}
]
```

Rules:

- A lesson cites at least two air log events or one correction. A lesson
  with no cites is dropped at validation.
- A lesson is about what to play for this listener. It is not a description
  of the listener beyond that, and it does not guess at why.
- The DJ may replace or withdraw its own lesson. The trail stays.
- Each lesson keeps a tally from the segments it caused: for and against.
  The tally is computed, never written by the DJ.

### 6.3 The notebook

The notebook is the listener-facing home of the lessons. Rule 2 (nothing is
held about you that you cannot read) makes it the whole of what Autoradio has
learned, not a summary of it.

`GET /api/autoradio/notebook` returns the lessons with their cites and
tallies, the DJ's record, and a link from every cite to its air log event.
The listener can:

- **strike** a lesson (it is gone from the next call, and the DJ is told it
  was struck, so it does not write it again from the same evidence);
- **correct** a lesson in their own words (the correction is an instruction,
  authority 1);
- **clear** the air log.

On the phone this replaces "What the paper has learned" **[code:
`LearnedFromPreferences.swift`]**, which lists local gestures today. The
screen keeps its reset.

Where the notebook lives is an owner decision with a default (goal file, D2).
The default: an owner-private record held by the Autoradio desk, registered
with the run sheet kind. The destination is a section of the listener's
edition page, written by Texture on a hand-off, because the edition is where
stated and learned preferences should sit side by side. That waits for the
hand-offs slice **[doc slice 5]**; the desk does not write Texture.

The air log and the notebook never leave the computer and are never
aggregated across listeners **[doc 3.4]**.

## 7. Trust

Trust is what the DJ is given ahead of the next outcome. For a listener that
is concrete:

| What is handed over | Today | Who decides |
|---|---|---|
| Room for `explore` segments | none designed | the listener, in words |
| Write-ahead: a stretch written before the listener asks | slice 6 | the listener; it spends money, so it is an owner act |
| A standing station on a schedule | slice 6 | the same |
| Reading mail headers | slice 3 | the same, by lease |

**[new]** Each of those prompts shows the DJ's record at the moment of
asking: "Of 40 segments I expected you to finish, you finished 31. You
corrected me twice this week." The listener reads the record and decides.
Nothing widens by itself in this version; a rule that widens latitude from
the record alone waits until there is a record worth reading.

The DJ also sees its own record (6.1). That is the other half of trust: a
student who can see what they got right has grounds to commit.

## 8. The junction with Autopaper

The edition is stated. The notebook is learned. They meet in one act:

> "You have skipped sport nine times this week. Take it out of your edition?"

The DJ proposes; the listener says yes or no; on yes, Autoradio hands the
edit to Texture, which writes the edition **[doc 3.2]**. A learned lesson
becomes a stated aim only by the listener's word. This is how the notebook
feeds the paper without the paper reading a hidden profile.

Autopaper reads the edition nowhere today **[code: importance is one global
score, `internal/frontimportance/scorer.go`]**. Making the paper read the
edition, and giving source processing its own forecasts and outcomes, is the
backport, and it is the next mission, not this one.

## 9. What this changes in the programme design

| Programme design | Now |
|---|---|
| 4.4: the desk sees its settled outcomes "under the acting-pack boundary" | The DJ sees every settled outcome and its whole record. Rule 2 replaces the boundary. |
| Slice 6 carries scoring, after the station | Forecast fields ship with the programme call (slice 2). Settlement ships with the air log (slice 4). The station stays in slice 6. |
| 4.1: the stretch pool is River, then Synthetic, then Devin | River, then Bedrock Haiku 5.5, the Autoradio pool **[code: `internal/modelcatalog/catalog.go`, owner 2026-11-11]**. Synthetic is reserved for self-development and Devin is refused for this desk. The role still runs with reasoning on, on its own prefix. |
| Slice 2 review: Gemini 3.8 flash, then Sonnet | The seats in `CLAUDE.md`, "Review". |

## 10. Order of building

Smallest first; each step runs the engine a little further round.

1. **Programme call with forecasts** (slice 2 plus section 3). Red.
2. **Air log with settlement** (slice 4 plus section 4). The phone reports
   playback, gestures and corrections.
3. **Paper back and lessons** (section 6.1, 6.2). A prompt and validation
   change to the programme call.
4. **Notebook route and screen** (6.3), with the record (section 7).
5. **The proposal to change the edition** (section 8), after hand-offs.

Steps 1 and 2 give a DJ that forecasts and is marked. Step 3 is where it
starts to learn, and it is small once 1 and 2 exist.

## 11. What blocks it on 2026-11-14

1. **River is not registered on the gateway.** The pool then answers from
   Bedrock, which does not stream, so the first segment arrives only when the
   whole stretch is written. The procedure is
   [reference/RIVER.md](../reference/RIVER.md), section 5. An owner act.
2. **The phone install.** Phone-side work needs the owner's go for an
   install candidate. Until then the proof is a script against d@e and the
   simulator.
3. **Deploy headroom on Node B.** A deploy was refused over disk on
   2026-11-11; storage is the live station. Every server step needs a deploy.
4. **The edition writer on the phone is a stub** **[code:
   `Interview.swift`, `UnbuiltEditionWriter`]**. Section 8 needs it or the
   Texture hand-off.

## 12. Not determined

- Whether a device key's call to the claims or commitment routes works end
  to end. Neither was exercised.
- Whether the commitment record can carry a per-question settlement written
  by a route handler, or needs the two rule kinds the programme design
  expects **[doc 4.4]**. The first hour of step 2 settles it. If the rule
  kinds are more than a day's work, the generic claims store
  (`internal/agentcore/api_precommit.go`, `internal/calibration`) is the
  fallback, with the handler as the settler and never `self`.
- How many settled segments a lesson needs before it helps. Two is a floor
  to stop invention, not a finding.
- Whether DeepSeek V4.1 Flash writes useful lessons at all. Step 3's proof
  is a reading of real lessons from real outcomes, not a count.
