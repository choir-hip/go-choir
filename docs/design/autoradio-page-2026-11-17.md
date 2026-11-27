# The Autoradio page

Standing: built | kind: happened | scope: in part | settled-by: a RadioPage page kind is wired in mobile/AutoradioLab/pages.contract lines 22 and 35; run sheet and radio row docs/current-architecture.md:113 | note: header line 3 says not built; the Built on rows and tile removal not individually verified

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
4. **Say something: removed by the owner on 2026-11-17** ("get rid of the say
   something prompts. They are broken anyway."). The row of suggestions that
   changed with what was on air, and its code and test ids, are deleted. The
   prompt bar and the microphone, at the bottom of every page, are the one way to
   say something.
5. **Places: removed by the owner on 2026-11-17** ("The auto radio screen shows
   some links to different pages. We can remove those because those pages are
   accessible from hitting the tetramark button."). The row of tiles (Autopaper,
   Books, Podcasts, Mail, Files) and its ids are deleted; the launcher behind the
   mark is the one way to the other pages. Instead the page shows **Built on**:
   under the entry on air, the stories it was built on, each a tappable title
   that opens that story's Texture in the deck (which shows its own sources), and
   the same list on a Heard row when expanded. An entry with no stories (an
   opening, an answer) shows no heading. The phone reads them from the entry's
   `cites` and `source` refs (`pub:<route>[@version]`, `texture:<id>`); a podcast
   clip's `story` ref is kept as a cite.
6. **Heard.** The last entries that ended, newest first, each with how it
   ended in a word: finished, skipped, left. Tap to play again.

7. **Takes** (station L7; [the learned document](autoradio-learned-document-2026-11-17.md) section 11). Below Heard: a **Record a take**
   action. Pressing it pauses the radio (the same pause a turn makes), turns hands-free off for the while, takes the capture session and
   records an `.m4a` (AAC-LC, mono, 24 kHz, 32 kbit/s) with the time against the 1:00 bound; Stop ends it, and so does the bound, which says
   so. A take of under 3 s is not kept. If an entry was on air, its story is the take's `responds_to`, shown as "Answering: <title>". Review
   shows the transcript (made on the phone from the file, **not editable**), Play back, Re-record, Discard and, only with a transcript,
   Publish, with the line "Your voice will be offered to other Autoradio listeners. You can delete it at any time." and the rights line.
   The radio stays paused through the review and goes on when the take is published, discarded or fails. Under it: the listener's own takes
   (Delete), the **Muted voices** (each with Undo) and "Your record: heard through by 7 of 11 listeners over 5 takes" (or "new: no outcomes
   yet"). A take on air shows the publisher's handle, the transcript, the publisher's record, **Built on** rows for its cites, and **Mute
   voice-xxxxxx** (also on its Heard row). The phone offers at most 3 takes to the DJ as candidates (never its own, never a muted
   publisher's); a take plays as lead-in then the publisher's audio fetched signed-in, and ends through the same `aired` op as any entry.
   Built: the model is `TakesPage.swift` and `TakeSession.swift`; the wire is `Choir/Takes.swift` (`TakesClient`). A mute is kept on the phone
   only: the notebook line ("Never play takes by voice-xxxxxx.") has no route to be written through yet.

8. **My notebook** (station L6; [the learned document](autoradio-learned-document-2026-11-17.md) sections 2 and 5 to 8). A row under Takes opens the page
   `notebook:` (kind `notebook`; also a glyph in the launcher's tray, `place-notebook`). The page shows the notebook's lines by section (the document is the
   Texture found by `GET /api/texture/edition`; **Edit the notebook** opens it in the Texture reader, which edits it as the listener's own revision),
   the scorecard from `GET /api/autoradio/learning` (settled, heard through, still open, by kind, forecasts with no verdict), the forecasts behind a line when
   it is tapped (the ledger rows whose `caused_by` is the line's ref: `lesson:L<n>`, `try:T<n>`, `mine:<8 hex>`), and the plain list of everything held about
   the listener with where each is read (rule 2). With no learning route (404, 501, an unknown shape) the scorecard says "not available yet" and shows no
   number; offline and signed-out say so. Built: `Notebook.swift` (the model and `NotebookStore`), `Choir/Learning.swift` (`LearningClient`, `SteerClient`,
   `SteerQueue`), `AutoradioLab/notebook.contract`.
   What the phone now sends about the listener: the `segment` event's `entry_id` and `claim_id` are kept on the entry; a relaunch that finds an entry on air
   closes it `failed` with the reason "interrupted by a restart" (never `left`, which is sign-out); the programme request carries `edition_text` (the notebook
   text when held, else the topics as text); the turn's `queue` carries `on_air` (`id`, `title`, `claim_id`, `stories`); more and less taps and mutes go to
   `POST /api/autoradio/steer` through a queue that survives a relaunch and waits while the route is absent; onboarding writes the topics as the first lines of
   "What you asked for" (the local topics file stays as a cache). Not built on the server at the time of writing: the notebook writer and the steer route.

## States

- **Cold start, nothing played yet.** On air shows a greeting and what is
  lined up: "Good evening. Six things are ready, about eighteen minutes.
  Press play and I'll start with the opening." Play is the only large
  control.
- **Speaking an answer.** On air shows the answer as kind Answer, and under
  it one line: "Then back to: <the held entry's title>".
- **You are speaking.** The status line says "Listening", and the on-air
  card dims. What you said appears as text when it is recognised.
- **Writing ahead, nothing on air.** One line on the status line ("Writing the next stretch. It will start by itself."); no "Nothing is on air" and no Play button, because pressing it would do nothing the desk is not already doing.
- **Heard** lists only entries that made a sound: an entry that failed to play is not heard.
- **Cannot speak.** The reason is on the status line and on the on-air card,
  with one action when there is one ("Try again", "Use the system voice").
- **Nothing ahead and nothing to write from.** Up next says so ("Open the mark below to find a story, a page or a podcast, and add it."); there is no row of places.
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
