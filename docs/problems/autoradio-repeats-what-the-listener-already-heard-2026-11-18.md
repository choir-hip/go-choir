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
