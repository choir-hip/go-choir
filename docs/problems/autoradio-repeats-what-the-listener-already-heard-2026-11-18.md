# Autoradio repeats what the listener already heard (2026-11-18)

## Problem

The owner, 2026-11-18: "in auto radio, we should have a way to remember what
has already been played for the user, so that then we can follow up rather
than repeating things in a way that seems unintelligent. Because I already
experienced hearing the same story today that I heard yesterday. It was told
slightly differently, but didn't have substantively new information."

A radio that tells you yesterday's story again as if it were new sounds
unintelligent. One that says "more on the story from yesterday: here is what
changed" sounds like it knows you.

## Evidence

- The owner's report above: the same story on two days, reworded, nothing
  new in it.
- The phone's run sheet, pulled the morning of 2026-11-18
  (`runsheet.json`, ids and counts only): 58 entries, every one of them
  ended on 2026-11-18. Nothing from the day before is on the phone. Of 43
  distinct things aired, 37 are published stories. The phone cannot know
  what was heard yesterday.
- The programme route (`internal/agentcore/api_autoradio_programme.go`)
  sends the DJ the queue and candidate stories. Nothing in it reads a
  history of what this listener heard.
- The design of 2026-11-11 named an air log and a heard route as slice 4;
  `docs/current-architecture.md` lists it as not built.
- Since 2026-11-18 the server does hold a durable record that could serve:
  each forecast the DJ makes is settled by the phone's report of what was
  heard (the settlement ledger). It is used to score the DJ, not to choose
  what to play.

## Belief

Two causes, and the owner's case may be either or both.

1. **No memory across days.** What a listener heard is kept only as the
   current run sheet, which is short. The DJ is not told what was heard
   yesterday, so a story that is still on the front page is a candidate
   again.
2. **A story is identified by its publication, not by its event.** If
   Autopaper publishes a second story on the same event (the overlap problem
   in `search-volume-outruns-keenable-credits-2026-11-18.md` and the corpus
   investigation of the same day), the radio sees a new story. "Told slightly
   differently" fits this.

The fix has the same shape as the paper's: remember, compare against what is
held, and treat the second telling as a follow-up that says what is new, or
skip it when nothing is.

## Remaining error

- Which of the two causes produced the owner's repeat is not known: the
  phone holds no entry from the day before, and the server's record for the
  owner's computer has not been read.
- "Substantively new" needs a test the DJ can apply without a second model
  call: a newer version of the same publication with its changes, or a
  different publication whose sources and facts mostly match one heard.

## Investigation (2026-11-18, read-only, code on main and `lane/learn-notebook`)

- **Candidates are chosen on the phone from the front page.** The phone
  fetches the front (60 rows), orders by the paper's weight and offers up to
  40 (`RadioProgramme.swift:456`, `StretchCandidates.swift:68-89`). The
  listener's history does not enter.
- **The only exclusion is by route, from the live run sheet.**
  `toldRoutes()` reads the entries now on the sheet
  (`RunSheetFeed.swift:175-184`), which keeps the last 50 ended entries. On
  the first request after the app opens the exclusion set is empty
  (`RunSheetFeed.swift:214`), and when every story was told the whole list
  is offered again (`RadioProgramme.swift:461-462`). So each morning
  everything heard the day before is a candidate again.
- **The DJ is told nothing about what was heard.** The programme input is
  the listener's turns, notes and lessons plus the phone's candidates
  (`api_autoradio_programme_input.go:82-131`,
  `api_autoradio_programme.go:178-189`).
- **The record already exists on the listener's computer.** Each segment the
  DJ writes is kept with its spoken text, title and cites as
  `pub:<route>@v<version>` (`api_autoradio_programme_records.go:130-149`);
  its forecast is settled true only by the phone's `finished` outcome
  (`api_autoradio_settle.go:9-15,169-182`). The learning route already joins
  them. The programme route never reads them.
- **Nothing links two publications on one event**: the front row has no
  event or cluster id and only a count of sources.
- All three causes in "Belief" are possible in the code. Which one the owner
  heard is settled by reading the owner's segment and settlement records
  (same route and version; same route, higher version; different route).

## Design: the DJ is told what the listener heard (first slice)

No new store, no new route, no second model call, no switch, nothing new in
Swift. The fix is on the listener's computer.

1. **A heard index, derived per programme call** from the Autoradio desk's
   own records: segments whose forecast the phone settled as heard, with the
   day, the title, the publication refs and what was said. Skipped, cut,
   left and failed entries are not heard.
2. **Each candidate is marked** in the DJ's input: never heard; heard at
   this version on a day, with what was said; heard at an earlier version,
   with what was said then. The DJ sees the candidate as it stands now
   beside what the listener was told, so it judges what is new in the call
   it already makes.
3. **Recently heard stories are listed** (day, title, a short line of what
   was said) so the DJ can recognise a different publication on an event
   already told. This is the weak part until Autopaper links stories by
   event; it rides the same call.
4. **The DJ chooses and the choice is recorded**: leave it out, a short
   follow-up that says what is new, or a full telling. The choice is a tag
   on the entry's forecast, so the phone's outcomes settle whether
   follow-ups are heard through more than repeats.
5. What is kept: the key (route, highest version heard, last day) for as
   long as the records are; the text handed to the DJ is bounded to recent
   days.

Lane `heard-memory`, on top of the notebook. Class red (forecast records).
