# The Autoradio page

Status: design, 2026-11-17. Not built. Owner: "we should definitely have a
page for auto radio itself. That should be what it loads into, not just the
auto paper front page. Because right now we're just using the sheet for
managing voice and it's not so intuitive."

## The problem

The app opens on the Autopaper front page. Every control there acts on one
story (Play, Queue). The radio, which is the product, shows only as a bar at
the bottom and a sheet behind it. Nothing on the first screen says a station
is here, what it is doing, or what it will do next.

## The decision

The app opens on the Autoradio page. It is the first page in the deck and
cannot be closed. It is the run sheet, made readable: what is on air, what
comes next, what was heard, and how to talk to it. The sheet behind the bar
is deleted; tapping the bar's now-playing line goes to this page. There is
one view of the run sheet, not two.

Autopaper stays a page, reached from here. Its per-story Play and Queue stay;
they place entries on the same run sheet, and this page shows the result.

## Layout, top to bottom

1. **Masthead.** The word AUTORADIO in the Autopaper masthead's type, the
   date, and one status line in plain words: "On air", "Paused", "Ready",
   "Writing the next stretch", or "Can't speak: <reason>".
2. **On air.** The largest thing on the page.
   - A kind label in small capitals: Opening, Story, Podcast, Book, Answer,
     Transition.
   - The title in the large serif. Under it the source and when it was
     published.
   - The sentence being spoken, two or three lines, moving as the voice
     moves. The voice becomes visible, and a listener who looks down knows
     where it is.
   - A thin progress line with time heard and time left.
   - Three controls: a large Play or Pause, Next, and "Why this". "Why this"
     shows the desk's reason for the entry in a sentence and, when one
     exists, its forecast that you will hear it through. It is the learning
     engine made visible.
   - Tapping the title opens the story, book or episode itself.
3. **Up next.** The next five entries: kind, title, length. A small mark
   shows who placed each one, you or the desk. Drag to reorder, swipe to
   remove, tap to play now. "All of it" expands the list in place to the
   whole run sheet ahead. While the desk is writing ahead, the last row says
   so.
4. **Say something.** A row of three or four suggestions that change with
   what is on air: "Skip this", "More on this", "What else is new", "Read my
   book". Tapping one sends it as a turn. The prompt bar and the microphone
   stay where they are, at the bottom of every page; this row teaches what
   they are for.
5. **Places.** One row of tiles: Autopaper, Books, Podcasts, Mail, Files.
   Each opens that page in the deck. This page is the hub.
6. **Heard.** The last entries that ended, newest first, each with how it
   ended in a word: finished, skipped, left. Tap to play again.

## States

- **Cold start, nothing played yet.** On air shows a greeting and what is
  lined up: "Good evening. Six things are ready, about eighteen minutes.
  Press play and I'll start with the opening." Play is the only large
  control.
- **Speaking an answer.** On air shows the answer as kind Answer, and under
  it one line: "Then back to: <the held entry's title>".
- **You are speaking.** The status line says "Listening", and the on-air
  card dims. What you said appears as text when it is recognised.
- **Cannot speak.** The reason is on the status line and on the on-air card,
  with one action when there is one ("Try again", "Use the system voice").
- **Nothing ahead and nothing to write from.** Up next says so and offers
  the places row.
- **Offline.** What is already written plays. The status line says the desk
  cannot write ahead.

## What is removed

- The sheet behind the bar, and its separate queue list.
- The front page as the launch page.

## What does not change

- The prompt bar, the microphone and Play in the bar, on every page.
- The deck. This page is its first card.
- The run sheet and its rules. This page only reads it and sends the ops the
  bar and the sheet already send.

## On the Mac

The same page in the same column. No Mac layout for now.

## How it is proved

A simulator tour photographs each state above from a fixture run sheet. The
tour asserts, by test id: `radio-page` is the first deck slot at launch;
`radio-onair-title`, `radio-onair-sentence`, `radio-play`, `radio-next`,
`radio-why`; `radio-next-<n>` rows match the sheet's entries ahead in order;
a drag sends a `move` op and a swipe a `remove` op; `bar-now` navigates to
`radio-page`; no sheet view exists. The owner judges the page on the phone.

## Open questions, with the default taken

- Should per-story Play on Autopaper jump to this page? Default: no, it
  stays on the story and the bar shows the change.
- Should the spoken sentence show for podcasts and clips? Default: only
  where a transcript is on the phone; otherwise the progress line alone.
