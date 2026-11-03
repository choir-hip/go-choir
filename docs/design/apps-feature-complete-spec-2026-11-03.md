# Apps feature-complete spec (2026-11-03)

Status: green (design text; the requests the self-development arms build
from). Goal: [`definitions/choir-selfdev-apps-feature-complete-2026-11-03.md`](../definitions/choir-selfdev-apps-feature-complete-2026-11-03.md).

Owner, 2026-11-03: "we really need to make them feature complete. Like I saw
the new slides app has like import from files, the files app built into
Choir, but not like upload a file, for example. So all of the obvious
features need to be there, and the UI design all needs to make sense. And in
fact, the files app itself needs to be reinvented."

No per-app feature list existed in the repo before this file. The earlier
request texts lived under `_runs/` outside it. This file is now the one
in-repo list. A request cites the section for its app plus the design bar
([`app-design-bar-2026-10-26.md`](app-design-bar-2026-10-26.md), checks 1 to
12). A feature on the list that a candidate lacks is a fail, not a residual,
unless the section marks it later.

## Rules every app shares

- **Package beside the original.** The rewrite is
  `frontend/src/packages/<app>-next`; the original is untouched until the
  owner switches its route. A -next package declares no `opens` until the
  switch (one opener per extension).
- **One home per kind (I27).** No owner data in `localStorage`. Files are
  `choir.file` objects through `/api/files` and `/api/file-objects`. A kind
  an app needs (deck, event) is declared by its package when the package
  checker allows kinds; until then the request says which existing store
  carries the data and the gap is named in the candidate.
- **Getting content in.** Every app that opens files offers, in its empty
  state and its toolbar: open from Files (a picker over the files tree),
  upload from the device (file input, and drag and drop onto the window),
  and recents. Uploads go to `/api/files` and show progress and failure.
- **Getting content out.** Every document an app can show can be saved to
  Files or downloaded.
- **States.** Empty, loading, loaded and error, each per the bar's check 9.
  Undo or confirm on every destructive act.
- **Keyboard and touch.** Every action has a pointer path, a keyboard path
  and a long-press path on touch. Shortcuts are listed in a `?` sheet.
- **Proof is real content.** The engineering desk takes screenshots of the
  app with real files on the computer it runs on, in the bar's presets and
  sizes, saved as files Texture shows. Empty-state screenshots alone prove
  nothing.

## Files (reinvented)

Files is the hub: the place every file is found, opened, moved and shared.
The reinvention is design and data together: a fresh layout, and the file
object (`docs/design/file-object-2026-10-24.md`) as its model.

- Browse: list view and grid view with thumbnails; sort by name, date, size
  and kind; breadcrumbs, back and forward; a sidebar with home, recents,
  starred and trash.
- Select: click, shift-click, cmd/ctrl-click, select all, rubber-band in
  grid; the selection drives the toolbar.
- Open: double-click or Enter opens with the default app; a context menu on
  right-click, Shift+F10 and long-press offers Open with every app that can
  open the file (from the registry routes), plus rename, move, duplicate,
  download, delete, copy link, and info.
- Preview: a preview pane (space bar toggles it) for images, PDF first page,
  text, audio and video, with name, size, kind, modified time and path.
- Upload: a toolbar button, drag and drop from the desktop into any folder,
  multiple files and folders, per-file progress, conflict prompt (keep both,
  replace, skip).
- Organize: new folder, rename inline (F2), move by drag onto a folder or
  breadcrumb and by a Move to picker, duplicate, delete to trash with undo,
  restore from trash, empty trash.
- Find: search by name in the current folder and everywhere, with a kind
  filter.
- Live: the listing updates on file events from other apps and devices.
- Later, named: sharing links and public previews (the public-preview path
  exists; its UI waits), folder move on the server (regular files first).

## Slides

Slides opens and presents decks, and authors simple ones.

- Open: PDF, PPTX and HTML decks, from Files, from upload or drag and drop,
  and from recents. HTML decks run in a sandboxed frame with no shell-origin
  script (`docs/problems/slides-html-deck-markup-runs-in-shell-origin-2026-11-03.md`).
- View: filmstrip with real thumbnails for every format, a 16:9 stage that
  fills at least 90% of its binding dimension, one 44px bar (position, prev
  and next, Present), click and arrow keys to advance, Home and End, a
  number then Enter to jump.
- Present: full screen with readable controls in every preset, speaker notes
  where the format carries them, a presenter view with the next slide and a
  timer, Escape to leave.
- Author: new deck; add, duplicate, delete and reorder slides (drag in the
  filmstrip); edit a title and body text per slide; pick a layout (title,
  title and body, image, two columns); insert an image from Files or upload;
  autosave and Ctrl+S. The saved deck is a file in Files the app reopens.
- Export: download the original; export an authored deck to PDF.

## Calendar

Calendar keeps the owner's events on the computer, not in memory.

- Views: month, week, day and agenda; Today; prev and next; a mini month for
  jumping.
- Events: create by clicking or dragging a time range, edit in a side panel
  (title, start, end, all day, location, notes, repeat), move and resize by
  drag, delete with undo. Events persist on the computer and survive a
  reload; they are not written back into a source `.ics`.
- Repeat: daily, weekly, monthly and yearly rules, and editing one
  occurrence or the series.
- Import and export: import `.ics` from Files, upload or drag and drop into
  the calendar's own store; export the calendar or one event as `.ics` to
  Files.
- Time zones: events show in the local zone, with the event's zone visible
  when it differs.
- Later, named: invites, reminders as notifications, and sync with outside
  calendars (they need app-agent network grants).

## Podcasts

- Library: subscriptions with artwork and unplayed counts; search and
  recommendations; add by RSS URL; unsubscribe.
- Episodes: list per show with date, length and played state; mark played
  and unplayed; filter unplayed; download state where it applies.
- Queue: Up Next, add to queue, play next, reorder by drag, remove.
- Player: play and pause, seek, skip back 15 and forward 30, speed, a mini
  player that stays while browsing, and resume from the saved position on
  any device.
- Durability: played state, queue and speed live on the computer
  (server-backed, as the playback position already is), never in
  `localStorage`.

## Desktop background (separate from the theme)

Owner, 2026-11-03: "for themes, i want to separate the desktop background
from the theme. the desktop bg should have its own mini-interface for
uploading an image or using one of some stock images or designs."

- The theme sets colors, type, radii and chrome; the background is its own
  owner setting and survives a theme change.
- A small background picker (from the desktop's context menu and from
  Settings): upload an image (file input and drag and drop, saved to Files),
  pick one from Files, or choose a stock image or design (a small built-in
  set, including each theme's current background).
- Fit: fill, fit, center, tile; a live preview before applying.
- Stored on the computer like other owner preferences, never in
  `localStorage`; the image is a `choir.file`.
- Each theme keeps a default background used until the owner picks one.

## UI design

Each app passes the bar's twelve checks in the seven presets at 1100 by 750
and at 390px. Content first: the title bar and prompt bar are the only
standing bars, an app adds at most one toolbar, and no band is empty. Write
fresh; do not imitate the patterns the owner rejected on 2026-10-26.
