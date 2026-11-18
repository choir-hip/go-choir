# Autoradio: what the owner's first listening session on the learning build showed (2026-11-18)

Status: open. One cause confirmed and repaired in a lane (the blank PDF);
two causes read from the log and the code and being repaired (the silence
after the opening, the slow start); one not understood (the lost audio).
Build on the phone: `lane/autoradio-integ19` (014d71f7f). Evidence: the app's
own console log of the session, pulled from the phone at 10:52Z, and the
owner's words and one screenshot. Nothing here is on main.

## 1. A PDF opens to a blank page (cause confirmed, reproduced, fixed in lane)

Owner: "and pdf files still dont load" (at least the third report).

- Log: `file open kind=pdf bytes=29945 from=cache ok=true`, `file page
  kind=pdf sections=4 frame=pdf`. The app believed it had worked. The
  screenshot shows the bar correct ("1 of 4", the title) and the page body
  empty.
- Cause: the app asked Exact's `context.file("app:/cache/files")` for the
  cache folder. That call names a file: it answers a URL for a path that is
  missing and nil for a path that is an existing folder. On nil the code fell
  back to `Library/Caches/Autoradio/files`, copied the PDF there, checked that
  the copy existed, and gave the page an `app:/cache/...` path with nothing
  behind it. So the first launch after an install can work and every later
  one draws blank, for the Reader and for every viewer in Files.
- Reproduced in the simulator by emptying both cache folders: the same blank
  page. With the owner's own PDF placed where the page reads, it draws, so
  the file was never the problem.
- Why three builds passed: the simulator kept a copy in the right folder from
  an older build, and the tour that opens a PDF asserts that a frame exists,
  not that it drew. The lead's tour runner now empties the caches first.
- Fix: `lane/autoradio-integ20`, the folder is `context.cache/files` in the
  three places that asked. `docs/reference/exact/ui-model.md` says which call
  names a folder (in that lane).

## 2. Silence after the opening (read from the log and the code; fix in a lane)

Owner: "after the first story it stopped. then after 20 seconds the next
story started." Log: the opening finished at 10:47:02Z, the first desk story
went on air at 10:47:32Z; later entries followed within a second or two.

- The server answers an `open` request with one opening line; stories need a
  second, `extend` request. The phone makes its own opening and also sends
  the open ask. The desk's opening came back 12.6 s later and was dropped
  ("one per start"). Because that ask added nothing, the phone marked the
  desk quiet for the entry on air and sent no extend until the opening ended.
  The stretch then took its usual time.
- So one model call per start buys nothing, and the stretch is asked for one
  opening too late. Inferred from code and timings; the log has no line for
  an ask's begin and end, which the fix adds.

## 3. Fourteen seconds to the first audio (read from the log and the code)

Owner: "it takes 10 seconds to start playing." Log: `voice warm ok in 12965
ms`, first audio at 13339 ms; the radio and the sheet were ready in under
200 ms.

- The first block waits for the neural voice's model to load. Warming starts
  at launch, and so does Play.
- A write-ahead exists to cover this (kept samples of an opening and a first
  entry). This start was `source=live`: nothing had been kept. Inferred: the
  one trigger that writes it fires while the writer is busy, is refused, and
  never retries; and kept audio expires after two hours, so the first start
  of a day would be cold in any case.
- The launch line's other numbers (about 4,742,000 ms) are measured from the
  host process's start, 79 minutes earlier.

## 4. The radio lost its audio and could not get it back (not understood)

Log: at 10:51:33Z, as one entry ended, `session reasserted playing
(transition)` and then `on air ... HELD the audio session could not be taken`
five times over 25 s, to the end of the log. The owner's screenshot shows the
bar saying NOW PLAYING with a Pause button meanwhile.

- From the code: that line means the session re-activated and the audio
  engine then failed to start; the engine's error is logged where the console
  log does not capture it, and the retry repeats the step that just failed.
- What preceded it: a take recorded and published at 10:48:48 to 10:49:09
  (the session went to record and back), after which spoken blocks came 47 s
  apart instead of 3 to 7 s; and a PDF page (a web view) opened in the same
  second as the failure. Which of these matters is not known.
- This is the failure ledger claim C118 said was fixed (settled false).

## Also learned

- A take recorded on the phone reached the server and passed its audio
  check (`take published id=take-47372c4d2a029b1f`): the first real one.
- The owner does not want a Take button: takes should come implicitly from
  voice mode. Not designed yet.
- Removed at the owner's word: the "Why this" button. Added: text labels in
  the Tetramark tray (lane `ui-morning`).

## Remaining error

- Items 2 and 3 are inferences until a start is logged with the new lines.
- Item 4 has no cause. The next build prints the engine's error and the
  session's state at the failure.
- A tour still cannot assert that a frame drew; only a person reading the
  screenshot can.
