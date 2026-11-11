# Autoradio app: one bar, many pages (2026-11-10)

Status: design, not built; the plan for a whole-app build. Mutation class:
green (a document). Rollback: revert. Lane: `lane/ar-ui`. Companions:
[the lab note](autoradio-lab-2026-11-10.md),
[the turns note](autoradio-turns-2026-11-10.md),
[the voice modes note](autoradio-voice-modes-2026-11-10.md),
[the app API map](autoradio-app-api-2026-11-10.md) (on main). The engine,
Components and Trace belong to the integrator lane.

Evidence marks: **[src]** read in source at file:line; **[doc]** stated in
Exact's docs; **[run]** fetched or run here; **[unrun]** reasoned, to be
proved. `frontend/` and `internal/` paths are in this repo; `exact:` paths
are in the pinned Exact checkout (`76cd50f9e`); `CFA` is
`exact:docs/contract-for-agents.md`, `CG` is `exact:docs/contract-grammar.md`;
`L/`, `AP/`, `TX/` are `frontend/src/lib/`, `.../packages/autopaper/`,
`.../packages/texture/`. Inventories and research are in the appendices.

## The app in one page

Autoradio is the listener's paper, radio and computer in one phone app.

- **Everything is a page.** A story (a published Texture), the listener's
  own document, a PDF, an e-book, an image, a video, an audio file, a mail
  message, a folder: each opens full screen in one viewer that picks its
  renderer by type.
- **One bar is the whole chrome.** A glass capsule floats over the bottom of
  every page, the page running full-bleed beneath it. It is the prompt bar
  (type, or hold to talk about the page in front), the radio (what is
  playing, always there, so listening and talking go on over any page) and
  the launcher (pull it up for places, search and open pages). Its living
  element is the TetraMark, a real 3D tetrahedron that shows what the radio
  is doing.
- **Open pages are cards.** Tap the pages button and the page shrinks into
  a deck; tap a card to go to it, swipe it left to close it, swipe a story
  right for "more of this".
- **Places, not apps.** Paper, Textures, Files, Mail, Activity and Settings
  are where pages are found.
- **Paper is a composed front page**, laid out from data by the edition's
  weights, headline lengths and clusters, not one fixed template.
- **First run is setup, then a conversation.** Sign in; voice setup
  (download the audio models, check them on this phone, fall back where
  needed); then a fully hands-free, interruptible interview about the
  listener's interests, with how Choir works said in passing, that writes
  their edition as a Texture they can read and change; then the front page and
  the first story playing.
- **Themes are data.** One theme format for the web and the phone, depth
  and motion included; a theme can be a Texture.

```text
 ┌──────────────────────────────┐
 │ AUTOPAPER        Tue 11 Nov  │  the open page, full bleed
 │ Russia and Ukraine trade     │
 │ strikes overnight; four dead…│
 │   Developing · 4             │
 │ ───────────────────────────  │
 │ Paraguay's Senate ousts …    │
 │ ╭──────────────────────────╮ │  the bar: glass, no outline, shadow;
 │ │ ▤3  Ask about this page… ◭│ │  pages · aim or now playing · voice (the mark)
 │ ╰──────────────────────────╯ │
 └──────────── ▔▔▔ ─────────────┘  the page shows beneath the home indicator
```

## Owner rulings, 2026-11-10 and 2026-11-11

1. Stories are Textures: one document surface. 2. Parity of function with
choir.news; the UI may be redesigned (appendix A); no block editing on the
phone (goal file). 3. Transclusion without hover; it must read well aloud.
4. Typography is the centre, as web-portable tokens. 5. Files, Mail and
Settings are features of the one app. 6. No desktop metaphor, no
overlapping windows, no tab bar; it must not feel like a wrapper over
choir.news. 7. "Prompt bar with launcher is good actually." 8. Multitasking:
browse, open and switch between files; viewers for PDF, e-books, images,
video, audio. 9. Listen and talk over anything. 10. One bar, many pages is
the direction. 11. The bar floats over the page: full bleed beneath, no
outline, no reserved band. 12. Layers and depth, not flat. 13. Themes are
modular data, one format for web and phone, a theme can be a Texture.
14. Cards for open pages, swipe left to close; swipe right on a story is
"more of this", shading the listener's ranking toward its topics and
entities, with no conversation; whether swipe left shades away is open
(question 6). 15. The front page stays a composed page of many stories, and
layout should get more dynamic; a one-at-a-time stack is an extra mode.
16. Onboarding is in this build: voice onboarding (meaning: download and
configure the voice and audio models, verify them on the device, fall
back), then a fully hands-free, interruptible voice interview (voice only; the grey text option was withdrawn by the owner, 2026-11-11)
on the existing turn engine that creates the edition and explains
how Choir works in passing. 17. Visual pop and motion: the TetraMark spins in 3D, grows and
shrinks; waiting for AI should feel alive. 18. Build the whole thing at
once with parallel builders, about 85%, then review and iterate.

### Rulings after the first phone walk, 2026-11-11

The owner walked the first phone build and ruled on six things. Where these
conflict with rulings 11, 14, 15 and 17 above, or with any section below,
these win; the sections below are to be read through them until rewritten.

19. **The page stack is the main view.** Paper front page, each story
(Texture), Files, Mail and Settings are full-screen pages in one stack. A
page scrolls vertically; a swipe left or right cycles to the neighbouring
page. There is no separate pile view. (Replaces ruling 14's "swipe left to
close" and "swipe right is more of this": closing a page and "more of this"
become visible controls, because the horizontal swipe now means next and
previous. Replaces ruling 15's "one-at-a-time stack is an extra mode".)
20. **The mark is the TetraMark**, the brand drawing used on choir.news
(`frontend/src/lib/tetramark.ts`: three curved blades in a 512 box), not a
regular tetrahedron with straight edges. It is the button that opens the
launcher, as on the web. It is animated: it spins, dances and takes depth.
It is the app icon (the brand PNG exports).
21. **No hidden gestures on the mark.** The push-to-talk experiment failed:
tap for play and pause, hold to talk and drag up for a menu were
undiscoverable. Talk, play and pause are separate visible controls with
their own glyphs and labels for assistive tech.
22. **The prompt bar is always visible and usable**, on every page and
layer. It is a glass lens that covers and overlaps the bottom of the screen
and runs off the bottom edge, not a capsule floating inside the screen.
(Replaces ruling 11's floating capsule; the page still runs full bleed
beneath the glass.)
23. **Voice prompts go to an Autoradio desk with a fast model**, so a turn
answers quickly. **The immediate response is generated on the phone by a
local model**, from what was just said and the page in front, not drawn
from stock phrases.
24. **The real voice models must load.** A build where only the fallbacks
run is a defect ("model … not registered").

25. **The stack is a deck that cycles** (owner, later the same day:
"Swiping pages should cycle them to the bottom of the deck. So for a
specific texture or file or mail, it can basically disappear, but it's
always reaccessible. And then, yeah, the launcher should still be able to
open the key pages."). Swiping the top page sends it to the bottom of the
deck and brings up the next; nothing is closed by a swipe. Opening a page
puts it on top. The launcher opens the key pages (Paper, Textures, Files,
Mail, Settings) and can bring any page in the deck to the top. This refines
ruling 19: the swipe is "send to the bottom", not previous and next.

26. **Autoradio is an AI DJ, not an article reader, and the Autoradio desk
is the interface** (owner, 2026-11-11). You open the app and it starts
playing. What plays is not any one article: the Autoradio desk writes a
script that spans many articles and mixes content from the paper, podcasts,
the listener's mail and more. Texture shows individual articles; the
Autoradio desk sits in front of Texture and the other desks. The listener
can interrupt at any time and, through the Autoradio desk, interact with any
of their apps or desks. "Listen to this article" is one thing the DJ can do,
not the shape of the product. Design:
[autoradio-desk-fast-turns](autoradio-desk-fast-turns-2026-11-11.md) for the
turn path; the programme script design follows it.

27. **The on-device model talks for as long as the desk is working**
(owner, 2026-11-11): "it shouldn't be limited to one line per se, but the
information should be front-loaded so it can be interrupted and have the
streams replaced at any time. And I suppose there should be some kind of a
transition." The local response is a stream of stand-alone sentences, most
useful first, replaced by the desk's speech the moment that is ready, with a
short transition between the two. This refines ruling 23.
28. **No deterministic timers around model calls** (owner, 2026-11-11): make
it fast, measure it and optimise it; do not put cutoffs in the way. A turn
moves on real signals: a refusal, an error, the end of a stream, the
listener.

29. **Steer models with prompts, not deterministic rules** (owner,
2026-11-11): "I don't like hard time budgets or token budgets. We should be
able to steer models with prompts, not deterministic rules, which will lead
to weird behaviors." No token budget, word cap or output filter is used to
shape what a model says, on the desk or on the phone; the prompt does that.
A provider's required max-tokens field carries the provider's own ceiling.
Bounds on what a stranger can cause (auth, request size) are not steering
and stay.
30. **Reasoning by role** (owner, 2026-11-11): reasoning off for the
fast spoken turn, on for writing the script. It is fixed per role, each role
with its own prompt prefix, so no call switches reasoning on a prefix
another call cached. Elsewhere the 2026-11-09 rule (reasoning stays on)
holds.
31. **The lens is glass and holds the buttons** (owner, 2026-11-11): "make
it glass, and make the buttons inside of it, rather than the prompt area
inside of the section at the bottom". One glass shape with the page showing
through. The text entry and every control sit directly on it; there is no
inner field box inside a bottom section.
32. **Every model downloads; onboarding configures them** (owner,
2026-11-11): "I want all the models downloadable. None ship with the app.
The point is to build a configurator that selects the best models for users
specific device in the onboarding." The app bundle carries no weights. During
onboarding a configurator probes the device, measures the whole voice
pipeline plus the language model on it, and picks the best model and quant
that fit. A newer phone gets better quality. The iPhone 14 Pro Max is the
benchmark: supported, but too old for the newest models. LFM3 1.7B runs on
it by itself in another app (several quants); the open question is the quant
that still fits alongside the voice pipeline. Phones older than about the 14
generation, such as the iPhone 8 or XR, are out of scope.
33. **No fallback for a missing desk route** (owner, 2026-11-11): "We
shouldn't plan for the route to be missing. Need to make sure all computers
are updated." The app does not fall back to the old Texture desk. Deploys
make sure every computer carries the Autoradio desk.
34. **Later, not this week: Autoradio on choir.news and in the Mac app**
(owner, 2026-11-11). The web gets an Autoradio app that shares state with
the iOS app. It has no on-device models. The Choir Mac app's Autoradio is
full-featured, with on-device models and better language models where the
Mac allows. Design only for now. This week's priority is an iOS app good
enough to share on TestFlight.

Also from the walk: a page title is never a raw address, a section name is
never a raw id, nothing is clipped off a screen edge, and the bar speaks
about the page in front.

Conflict: the goal file
([choir-autoradio-pass1](../definitions/choir-autoradio-pass1-2026-11-10.md),
`excluded:`) lists Mail; ruling 5 supersedes it (question 3).

## 1. One bar, many pages

### The layers

```text
 ┌──────────────────────────────┐
 │  AUTOPAPER       Tue 11 Nov  │   the open page, full bleed, edge to edge
 │  Russia and Ukraine trade    │
 │  strikes overnight; four…    │
 │  ……                          │
 │  ……page scrolls under the bar│
 │ ╭──────────────────────────╮ │   the bar: glass, no outline, a shadow,
 │ │ ▤3  Ask about this page… 🎙︎│ │   8 pt above the home indicator
 │ ╰──────────────────────────╯ │
 └──────────── ▔▔▔ ─────────────┘   home indicator; the page shows beneath
```

Three layers, bottom to top: the **page** (elevation 0), the **bar**
(elevation 2), and **layers** that rise from the bar (elevation 3: the
launcher, the pile, the expanded player, the source panel). UIKit sheets
(elevation 4) are kept for tasks that take over: Settings, compose, sign-in.

### The bar's three jobs in about 60 points

The bar is a capsule 56 pt tall (it grows only for words: listening and
typing), inset 12 pt from the sides, its bottom 8 pt above the home
indicator (`calc(env(safe-area-inset-bottom) + 8px)`, CFA:4281-4295). It
has three zones, and only the centre changes job.

```text
 ╭─────────────────────────────────────────────╮
 │ [▤3]   centre: field / now playing / words  [●] │
 ╰─────────────────────────────────────────────╯
  pages            changes with the state          talk
  (tap: pile;                                      (hold: talk;
   drag up: launcher)                               tap: read this page)
```

- **Leading, the pages button** (44 pt): a small stack glyph with the count
  of open pages. Tap opens the pile; drag up anywhere on the bar opens the
  launcher.
- **Trailing, the voice button** (44 pt): hold to talk; tap to play the page
  in front ("read this to me") or pause what plays. One button, because
  playing and talking are both "the radio".
- **Centre**: one line that says what the bar is doing now.

| State | Centre shows | Trailing | Notes |
|---|---|---|---|
| Idle | "Ask about this page…" (the aim, as the web: "Reply to <title>…" on a published story, "Edit <title>…" on the listener's own) | ▶ play this page; hold = talk | tap the centre to type |
| Playing | what plays: title, and a hairline progress rule along the bar's lower edge; the bar's tint warms slightly | ❙❙ pause; hold = talk | tap the centre to expand the player; swipe the centre left or right for the previous or next passage |
| Listening | the words as heard, live, up to three lines (the bar grows upward); a level ring round the trailing button | the ring | the programme dips, then pauses; release ends the turn |
| Heard, on it | the holding line ("Looking at the third paragraph…"), a slow ring | ring turning | the page shows the listener's passage in place |
| Revision arriving | "Revised · Show" for 6 s, then back to Playing | ❙❙ | tap jumps to the new blocks; the radio reads them |
| Typing | a multi-line field above the keyboard; the aim line sits above it; Send replaces the voice button; the pages button hides | Send | `interactive-widget="resizes-content"` on the root keeps the bar over the keyboard (`exact:docs/recipes/chat-thread.md:32,75`) |
| Could not | one plain sentence for 4 s, danger tint on the ring | ▶ | then back to the previous state |

Typing while something plays: tap the centre to expand the player; its
lowest row is the Ask field. Talking never needs the player: hold the voice
button in any state.

**What talk and Ask address.** Talk addresses what is playing, at the play
point; when nothing plays, the page in front at the paragraph in view.
Typed Ask always addresses the page in front (it is in front of the eyes).
The aim line names the target before anything is sent.

**Insets.** A page's scroll content ends with
`padding-bottom = 56 + 8 + env(safe-area-inset-bottom) + 16` so its last
line scrolls clear of the bar (the documented pattern, CFA:1464-1470). A
page that pins controls to its bottom (a video's scrubber) lifts them above
the bar by the same sum.

**Legible over any page.** The bar is `backgroundMaterial="glass"`: iOS
26's `UIGlassEffect`, falling back to `systemUltraThinMaterial` before 26
(`exact:host/apple/Sources/ExactKit/Backdrop.swift:241-251`; used on a
composer bar in `exact:apps/messages/app.contract:337`). It carries the
theme's elevation-2 shadow (multi-layer `box-shadow`,
`exact:host/apple/Sources/ExactKit/BoxShadow.swift:1-10`) and its text uses
vibrancy over the blur (`exact:host/apple/Sources/ExactKit/IOS/VibrancyIOS.swift:1-17`).
No border. A theme may replace the material with a solid tint (section 5).

### The launcher: places, search, open pages

Pull the bar up (a `pan` with `heightrelease` velocity, CG:1675,1862-1866)
and it becomes a layer reaching about 70% of the screen; let go below a
third and it falls back. It is words and live content, never a grid of
icons.

```text
 ╭──────────────────────────────╮
 │ 🔍 Search pages, files, mail │
 │ Open ─────────────────────── │
 │ [card][card][card][card] →   │   the pile in miniature; tap to go
 │ Places ───────────────────── │
 │ Paper     Updated 4 min ago  │   each row: the place's name and
 │           Russia and Ukraine…│   what is new there, in one line
 │ Textures  "My edition" v12   │
 │ Files     Downloads · 2 new  │
 │ Mail      3 unread · Ana R.  │
 │ Activity  2 new revisions    │
 │ Settings                     │
 │ ╭──────────────────────────╮ │
 │ │ ▤3  Ask about this page… ●│ │   the bar stays at the bottom of the layer
 ╰─┴──────────────────────────┴─╯
```

Choosing a place opens it as a page (Paper's front page, the Textures list,
a folder, the inbox). Search covers page titles, document titles, file
names and mail subjects; results open as pages. The bar sits at the foot
of the launcher, so "Ask" and "talk" are still under the thumb.

### Open pages are cards

The pile holds every page the listener has open, most recent on top. Tap
the pages button: the page in front shrinks into its card (scale 0.92,
elevation 0 to 2) and the others appear behind it as a deck, each offset
upward and slightly smaller, so the deck has visible depth.

```text
        ┌──────────────────┐   ← older cards, offset up, smaller, dimmer
      ┌─┴────────────────┐ │
    ┌─┴────────────────┐ │─┘
    │  The current page │─┘     swipe left: close   tap: open
    │  (title, type,    │       swipe right on a story: more of this
    │   a live preview) │       drag the deck: flick through
    └──────────────────┘
     ╭──────────────────╮
     │ ▤3  …          ● │      the bar stays
     ╰──────────────────╯
```

- **Swipe left closes** the page, with no ranking effect by default (the
  next section). An "Undo" pill appears on the bar for 5 s.
- **Swipe right** means "more of this" on a published story (below). On any
  other page (a PDF, a mail message, a folder, the listener's own document)
  swipe right does nothing: the card leans and springs back. Recommended
  over a second meaning such as "pin", because a gesture that means one
  thing everywhere it acts is learnable eyes-off; pinning lives in the
  card's long-press menu.
- **Never lose work.** A page with unsent words in the Ask field, a mail
  draft with unsaved changes, an upload in flight, or a turn in progress
  asks first ("Discard your unsent words?") instead of closing. Mail
  drafts are saved as drafts on the server as they are typed, so closing a
  compose card keeps the draft.
- **Back and close.** A page opened from another page remembers its opener.
  The edge swipe goes back to the opener and leaves the newer page in the
  pile (as Safari does with a link's tab). Within a page, the edge swipe
  first closes that page's own layers (a quote card, the source panel).
  Closing is only the card's swipe left or "Close" in its menu.
- The pile keeps the 30 most recent unpinned pages and restores them at
  launch (a list of addresses, not resumed work, per the restart rule).
- Built from `pan`, `translate`, `rotate` and `scale` with `sharedElement=`
  for the page-to-card flight (`exact:docs/recipes/motion.md:53,60-61`;
  CG:1862-1916). Exact has no general gesture arena and warns against
  hand-built card drags in lists (`exact:docs/contract-for-humans.md:1517-1519`):
  the deck is not a list, and its drag is one `pan` on the top card
  [unrun: the first spike].

### What the swipes mean: closing, more of this, and the signals

**Recommended default: closing is neutral.** Swipe left closes, with no
ranking effect, everywhere (open pages and story cards). Swipe right is
"more of this", the one cheap explicit signal. "Less of this" is a
deliberate act of its own: a row in the card's long-press menu and in "Why
this story". People close things for many reasons (finished, later, the
wrong moment), and a close read as dislike would quietly degrade the
ranking.

Alternatives, open (question 7):

- **(a)** Swipe left shades away; an explicit close button (×) on the card
  closes neutrally.
- **(b)** Swipe left shades away only in the story-stack mode (above),
  where a pass is the point, and is neutral on open pages.

**The negative side comes better from what the listener does** than from a
close. What the app can observe on the phone without asking:

| Signal | Observable | Meaning |
|---|---|---|
| opened a story from the front page or the stack | yes | interest |
| read through (reached the last block; time on page against its words) | yes | interest; a quick bounce is weak disinterest |
| listened through a story on the radio | yes (the programme position) | interest |
| skipped early on the radio (next within the first passage) | yes | disinterest (the turns note's settlement signal) |
| spoke about it (a turn whose focus is the story) | yes | strong interest |
| opened a citation or the sources | yes | interest or doubt; not scored at first |
| closed unopened, or after a bounce | yes | recorded, not scored by default |
| shared it | yes (the share sheet reports completion) | interest |

**Gesture meanings are a setting.** Because the owner wants to play with
this, each gesture's meaning is a component in Settings, Components (the
integrator's screen, beside the pipeline stages): "Swipe left on a story:
close / close and less of this / less of this only in the stack"; "Swipe
right on a story: more of this / nothing". A change applies to the next
gesture.

**Every gesture is an event.** One line per act on the listener's computer:
`{gesture, surface: pile|stack|front, page kind, publication_id and version
(for a story), meaning in force, opened before, dwell ms, listened
fraction, at}`. To compare variants, run each for about a week and read
three numbers from the same records: (1) how often a closed story's subject
is opened or listened through later (often means a close was not dislike);
(2) whether the top of the listener's front page is opened and listened
through more after shading than before; (3) how often the listener undoes a
close or resets what was learned. Compare on those, not on gesture counts.

**Acknowledgement, nothing more.** Right: the card leaves to the right with
a small "More like this" mark and a light haptic. Left: it leaves to the
left, and "Undo" sits on the bar for 5 s. No dialogue, no question, no new
story.

**Where the learning lives.** The scored acts are kept as a Texture beside
the listener's edition document, "What I've told the paper", one line per
act in plain words ("More like this: Russia–Ukraine talks (Ukraine, Russia,
Abu Dhabi talks) · 11 Nov 00:44"), so it can be read, edited and deleted as
a document. The weights the ranking uses are derived from it, never stored
apart from it. The raw gesture events are lab records, like turn records.

**Effect.** The listener's front page and stack re-rank over time: a
story sharing a central entity with "more" acts gets a lift, one sharing
with "less" acts a dip, bounded like the web's section lift (at most
±25%, `L/autopaper.ts:~395-460`) and decaying with age. The shared paper's
ranking is never changed by one listener. If the edition made a ranking
commitment for that story (a precommitment record, turns note,
"Learning"), the act can settle it the way a skipped clip does; that
record is the desk's, not built here.

**Why and undo.** Long-press a story: "Why this story" says "Ranked by
the paper: importance 8, 27 sources. Lifted for you: you asked for more
on Ukraine (3 times)", with "Less of this" and "Forget this" beside it.
Settings, What the paper has learned, opens the document with Reset.

**Dependency, a server gap.** Published stories carry no topic or entity
labels on the routes the app reads: the front list has `section`,
`sections`, `importance`, `source_count`, `words` and `kind` [run:
`front?limit=60`, 2026-11-11]; the resolve bundle's `source_entities` are
sources, not subjects [run]. The paper does hold central entities: the
research desk writes `mentions_entity` edges with role `central`, read
only inside the related-stories walk
(`internal/platform/publication_related_entities.go:1-12`;
`internal/platform/publication_related.go:206-208`). Needed: `entities:
[{id, label}]` (central only) on each front-list story and in the
bundle. Until then the signal can use the section and the source hosts,
weakly.

### First run

A fresh install opens on Paper's front page as the one open page, signed
out, with real stories; sign-in, onboarding and the interview follow
(section 2).

### Audio and video pages beside the radio

One voice at a time: two spoken streams at once are unintelligible, so the
radio pauses rather than ducks.

- Playing a video or audio page pauses the radio; the bar's centre shows
  that media, and Now Playing on the lock screen follows it (Exact gives
  Now Playing to the claimant that played last, CFA:2202;
  `exact:host/apple/videoarm/NowPlaying.swift:1-8`).
- When the media ends or is paused, the bar offers "Resume the radio".
- Holding to talk pauses whatever plays; the turn addresses what was
  playing (the video's page, at its time).
- Picture in picture keeps a video playing over other pages
  (`exact:host/apple/videoarm/VideoArm.swift:394-402`).
- Speech uses `transient` (duck others) and resumes others on release
  (`exact:host/apple/Sources/ExactKit/AudioSession.swift:24-26,179-185`); the
  talking state's session claim is the integrator's (voice modes note).

### The talk control: states, look and sound

The kit's phases (`mobile/AutoradioKit/Sources/AutoradioKit/TurnEngine.swift:77`)
map to six states. Colour is never the only signal: each has a glyph, a
VoiceOver label and a sound or haptic.

| Listener sees | Kit phase | Look | Sound, haptic |
|---|---|---|---|
| Ready | playing | voice glyph | none |
| Listening | addressed, capturing | level ring; the words in the bar and, on the page, as a passage at the play point in italics | the programme dips, then pauses; selection haptic on press |
| Heard | ended | the passage sets in roman, "You" in its margin | two rising notes |
| On it | acknowledging, holding | slow ring (reduced motion: three static dots); holding lines under the passage, marked "holding" | the spoken acknowledgement |
| Revised | revised | new blocks enter under the passage with an accent rule, "Revised just now"; reading moves to the first new block | success haptic; the revision is read |
| Could not | error | ring in the danger colour; one plain sentence | one low note; the radio resumes |

The listener's words and the desk's revision are part of the document, not
a chat log (turns note: "the radio is a Texture document played aloud").

### The expanded player

Tap the bar's centre while playing: a layer rises (elevation 3) with the
page behind scaled to 0.96. It shows the document's title and source, the
passage being read in reading type with the current sentence lit, the next
passage dimmed, a progress rule through the document, transport (back 15 s
or a passage, play or pause, forward), the source hosts of the current
paragraph as chips (tap: the source panel), and Up Next from the edition.
Its last row is the Ask field. Swipe down to return; "Open the page" shows
the document at the play point.

### One hand and VoiceOver

- **One hand** (the owner's phone is a 6.7-inch 14 Pro Max): every frequent
  act is in the bottom fifth: the bar's three zones, the deck, the
  launcher's rows (it opens upward, its most used rows lowest), the
  expanded player's transport. Nothing that matters lives only at the top.
- **VoiceOver.** The bar is three elements: "Open pages, 3, button";
  "Ask about <title>, text field" or "Now playing, <title>, button,
  double-tap to expand"; "Voice, button, double-tap to play, double-tap and
  hold to talk". The two-finger double tap (the system's Magic Tap) plays
  and pauses. A card reads "Page 2 of 5, <title>, story" with actions Open,
  Close, More like this; a story on the front page reads its headline,
  dek, section and "updated 12 minutes ago". Swipes always have a named
  action; where Exact cannot add custom actions [unrun], the card shows
  visible Close and More buttons while VoiceOver runs.

### How the UI can fail

| Case | What the listener sees |
|---|---|
| Empty front page | "No stories yet." with Refresh; the radio offers the last edition |
| Signed out | Paper and published stories work; talk, Ask, Textures, Files, Mail present sign-in, then continue |
| Offline | the front page shows the last list with "Offline · updated 2 h ago"; talk says "No connection. Your words are kept." (the turn is kept, not resent by itself) |
| Computer asleep or booting | "Your computer is waking up"; Wake if it stays asleep; talk still acknowledges on the phone |
| Slow desk | the On it state holds; after 20 s "Still working on it" and Carry on |
| Key revoked (401) | "Your sign-in is no longer accepted." and Sign in again |
| Long documents | blocks in a virtualized list (CFA:1452-1510); the play point is reached by `scrollIntoView` by block key (CG:2711) |
| Dynamic Type | every size is in `rem` or a text style (section 5); at the largest sizes the bar's centre wraps to two lines and the bar grows |
| Dark mode | every colour is a theme role with both sides |
| Too many pages | the oldest unpinned page leaves the pile quietly; it is in the launcher's history |

### Alternatives considered

**B. A small tab bar with the reader underneath** (Paper, Radio, Library,
Mail, and a mini-player above the bar). Familiar and quick to build, and
each tab maps to one custodian. Ruled out by ruling 6; it also makes
switching between two files (ruling 8) a matter of tabs plus Back, and the
mini-player and a prompt bar compete for the same strip.

**C. A feed-first root.** One "Today" stream mixing the edition, new
revisions and mail that needs the listener, with places pushed from it.
Strong for an eyes-off morning, weak for "open that PDF again", and one
list carrying three custodians' data needs a merge rule that does not
exist.

| | A. One bar, many pages | B. Tabs and a reader | C. Feed-first |
|---|---|---|---|
| Listen and talk over anything | yes: the bar is on every page | yes, through the mini-player | yes, on the feed only |
| Open and switch between files | the pile: one tap and a flick | Back, tabs | through the feed |
| Feels unlike choir.news and unlike a wrapper | yes | partly | yes |
| Chrome on screen | one capsule | tab bar, mini-player, headers | header and the feed |
| Hardest part to build | the bar's states and the deck | little | the merge rule |
| Risk in Exact | a persistent layer cannot sit above a UIKit sheet (inferred from CFA:2698), so layers are authored overlays | none | none |

**Recommendation: A**, agreed by the owner. Its cost is that the launcher,
the pile, the player and the source panel are authored layers rather than
UIKit sheets, because the bar must stay above them.

## 2. First run: sign in, voice setup, the interview, the edition

Owner correction, 2026-11-11: "By voice onboarding I mean configure all of
the voice and audio models": download them, make sure they work on this
device, and fall back where they do not. Introducing the app by voice is
not a separate step; it belongs inside the interview. "Nobody wants a
lecture ... if we make it interactive, then we can intersperse the
explanation in the interview."

### The path

```text
 Paper front page (signed out, real stories, playable)
   └─ "Make it yours" on the bar, or any act that needs the computer
       → Sign in (device flow, browser approval)            ~30 s
       → Voice setup: download the models for this phone,
         check each one works here, fall back where not      minutes, mostly waiting
       → The interview: hands-free, about interests, with
         how the radio and Choir work taught along the way   3 to 6 min
       → The edition is written (the desk; the alive wait)   ~15 s
       → The edition shown as a Texture; then the front page,
         re-ranked, and the first story playing
```

A fresh install opens on Paper's front page, signed out, so the first
thing the listener sees is the paper, real and playable. The bar's idle
aim reads "Make it yours" until the edition exists. "Skip for now" is on
every step and leaves the listener on the front page with the default
edition (all sections); the Textures place then shows a card "Make your
edition" (the entry lane O asked for).

### Voice setup (configuring the voice and audio models)

The job: end with a working set of on-device models for this phone, one
per stage (speech bounds, transcription, the fast decision, the voice,
later the word spotter and speaker check), each checked on this device,
and written as the listener's configuration (the same data Components
shows). Nothing here explains the product.

1. **Pick the set for this phone.** From the model table
   (`ModelStore`), by chip and memory: the recommended implementation of
   each stage, its download size and its fallback. The page says the total
   plainly before anything downloads ("About 1.4 GB. On Wi-Fi this takes a
   few minutes.") and asks before using mobile data.
2. **Download with honest progress.** One row per model: name in plain
   words ("Hearing you", "The voice"), size, a real progress bar, done or
   failed. Downloads resume, continue in the background, and survive the
   app closing. The models lane found the library's downloader can stall
   silently for many minutes; a row that makes no progress says so and
   offers retry or the fallback. The TetraMark carries the wait.
3. **The listener is not held here.** The paper is readable and playable
   during setup with what is already on the phone (the system voice,
   Apple's recogniser where the phone has it). Setup runs as a card that
   can be left and returned to; the bar shows its progress.
4. **Check each model on this device.** After a model loads, a self-test
   with a bundled clip and a fixed phrase: does it load within its memory
   budget, how fast is it (real-time factor or milliseconds per decision),
   is its output right (the transcriber returns the known sentence; the
   voice produces audio of the expected length; the decision model answers
   two fixed questions as the reference does). First load compiles the
   model and can take tens of seconds; the row says "Preparing" rather than
   looking hung. The numbers are kept as a record on the listener's
   computer: they are the per-device measurements the project lacks.
5. **Fall back, per stage, without asking.** A model that fails to
   download, load or pass its check is replaced by the next in its ladder,
   ending in something that always exists:
   - transcription: the larger on-device model, then the smaller, then
     Apple's recogniser;
   - the voice: the chosen on-device voice, then a lighter one, then the
     system voice;
   - the fast decision: the on-device model, then System 1 through the
     listener's computer (the default until an on-device one is proven);
   - speech bounds: the model, then the energy detector.
   The page states the result in one line per stage and marks any fallback,
   with "Try again" beside it.
6. **The microphone, asked here, in context.** The hearing check needs it:
   "To check I can hear you I need the microphone. What you say stays on
   this phone as sound; only the words go to your computer." Then one live
   check: "Say anything." The level moves, the words appear. Refused or
   silent: setup finishes with the bundled clip alone and the interview
   waits; the paper stays readable and playable, typed prompts work, and
   the "Make your edition" card says the interview needs the microphone. A hearing check
   through the current output route ("Playing on AirPods"; "Can you hear
   this?") closes the step.
7. **Later changes are the same screen.** A new model, a changed phone or a
   failed model reopens this card from Settings → Components; nothing about
   setup is first-run only.

How to talk and how to interrupt are not taught here. The interview
teaches them by doing: its first exchange is the listener's first turn, and
the first time the radio reads something back it invites the listener to
cut in.

### The interview (three to six minutes)

Owner, 2026-11-11: "We need to do the full hands-free audio ... we have to
enable interruptions from the get-go. The user can always interrupt the
agent, and that's important because then the agent will reframe their
question based off the user's interruption." And: no special agent loop for
onboarding; "we should just reuse what we already have."

So the interview is the radio's own conversation, fully open:

- **The microphone is open for the whole interview.** No listening window
  after each question and no button. Speech bounds and end of turn are
  detected; the talk-mode setting (button, word, open) is chosen after
  onboarding and does not apply here.
- **The listener can interrupt at any moment**, mid-question included. The
  voice stops, the turn is taken, and the next line answers what was said:
  the question is reframed, dropped or replaced. This is the same
  interruption design as Mike's (the owner's reference) and the same
  yielding rule the radio uses during a programme.
- **It is the existing turn engine, not a new one.** The interview is a
  programme whose document is the interview Texture and whose "script" is
  the question bank. The session coordinator, the System 1 questions, the
  holding voice and the desk are the ones the radio already uses. Nothing
  interview-specific is built below the bank and its questions.
- **Voice only.** There is no typed interview (owner, 2026-11-11: "We
  need to do the onboarding with voice. Even if that won't work in every
  single circumstance, it's better to just make that the standard, make it
  the ritual ... they'll just have to wait until they find a quiet space";
  this supersedes the earlier grey text option). The design from Mike
  already works well in ordinary rooms, and that is the bar.
- Each line is short, and one sentence here and there, in passing, says
  how Choir works. It is an interview about the listener, never a lecture.

Open interruption needs the phone not to hear itself: echo cancellation
(voice processing) on the speaker, or headphones. Voice setup has already
measured which this phone and route can do; where the speaker route cannot
hold it, the interview says so and asks for headphones. When the room is
too loud, it stops kindly and waits for a quiet place; it does not fall
back to a button or to typing.

| # | Question (the bank's default wording) | Branches (System 1 decides) | How Choir works, in passing |
|---|---|---|---|
| 1 | "What do you want to know about every day?" | answered → 2; vague ("everything", "news") → offer three sections from today's paper as choices; asks a question → answer briefly, repeat | "This becomes your edition: a document on your own computer, not a chat. You can read it, change it, or ask me to." |
| 2 | Follow-up on the strongest topic: "When you say tech, is that the companies, the science, or what it does to work?" | one of two or three prepared follow-ups, chosen by System 1 from the answer; skipped when the answer was specific | |
| 3 | "Anything you're tired of hearing about?" | none → 4; names topics → noted as less | "The paper learns from what you do. Swipe a story right for more like it; skip what you don't want." |
| 4 | "Any sources you trust, or don't?" | names → noted; none → 5 | "Every story cites its sources. Tap a number to see where a sentence came from." |
| 5 | "Where are you, and are there places or people you follow?" | place → local news on; people → followed | |
| 6 | "When do you usually listen, and for how long?" | morning, commute, evening; a length | "I keep what I learn in documents you can read. Nothing is hidden in a chat log." |
| 7 | Read-back: "Here's what I have: … Anything to change?" | changes → correct, read back once more (at most twice); "that's it" → write | |

Rules: at most about six minutes; "that's enough", "skip" or "later" at any
point is recognised by System 1 and goes to the read-back with what has
been said. A question the listener does not understand is rephrased once
from the bank, then skipped.

**What System 1 decides and what the desk writes.** System 1 (Jev through
the listener's computer, or the phone model later) answers small typed
questions per turn: did the listener answer, ask, refuse or wander; which
prepared follow-up fits; should the next line include an explanation beat;
is the interview done. The questions and their wording are text owned by
the desk's package, as the turns note requires ("early structure lives in
questions, not in types"), so they can be revised without an app release.
The desk writes once, at the end: the edition document. Nothing in the
interview waits on the desk, so every reply is fast.

**What is stored where.** The interview is itself a Texture on the
listener's computer ("Interview, 11 Nov"): the interviewer's lines and the
listener's passages in order, exactly as a radio turn is kept in the
document where it was said. At the read-back's "that's it" the app submits
one prompt-bar request focused on a new document titled "My edition", with
the interview as its source; the desk writes the edition: what I follow
(topics, with the listener's words), less of, sources, places and people,
when and how long. "What I've told the paper" (section 1) starts empty
beside it. No audio is kept unless the listener later turns that on.

**How it ends.** The ~15 s write is the alive wait of section 5: the mark
tumbles, the page shows what the desk is doing and the edition streams in.
Then the edition opens as a page: "This is your edition. Change anything by
asking." Swipe it away (or tap "Start the radio") and the front page
re-ranks by it and the first story plays.

**Recovery.**

| Case | What happens |
|---|---|
| Silence after a question | reprompt once ("Take your time; or say skip"); then the question is skipped; after two silent questions: "I'll be here when you're ready," and the interview pauses on its card |
| Noise, or the radio hearing itself | "It's too loud here for me to hear you well. Try headphones, or come back when it's quiet." The interview pauses on its card with what was said kept; the paper and the default edition go on meanwhile |
| Microphone refused | no interview yet; the card "Make your edition" explains it is a spoken conversation and links to the microphone setting |
| "Skip for now" | default edition; the card "Make your edition" in the Textures place |
| The desk fails to write | the interview document is kept; "I couldn't write your edition just now. I'll keep what you said." and a Retry; the default edition meanwhile |
| A crash mid-interview | per the restart rule, nothing resumes by itself; the interview document holds what was said, and the card reads "Continue your interview" (an owner act) |

**What the server lacks for this.** A per-account edition: today a
document titled "My edition" is an ordinary Texture. The paper's ranking
and the radio need to find it (a document role or marker the server
recognises, as `IsGuideTitle` finds the Style Guide,
`internal/textureowner/style_owner.go`), and entities on stories to match
its topics (section 1).

## 3. Paper: a front page laid out from data

### What the web does today

The web lays the front page out with CSS grid and container queries: 1, 2
or 4 columns by width, and a fixed template of tiers (lead 1, top 4 with
deks, briefs 6, then More stories) filled in rank order
(`L/autopaper.ts:466,480`; `AP/App.svelte:~375-530`). `pretext` is not used
there (it serves the citation flow and sign-in only). The result is the
same shape every hour whatever the edition holds.

### The layout system

A layout is a document, separate from the theme, that the app renders and
that the desk may later write. The theme says how things look (faces,
colours, depth); the layout says what goes where and at what size.

```jsonc
{
  "schema": "choir.layout.v1",
  "name": "Autopaper default",
  "grid": { "units": 6, "gutter": "0.75rem", "margin": "1.25rem" },
  "steps": {                      // headline sizes by name; the theme maps names to type tokens
    "headline": ["xl", "l", "m", "s"]
  },
  "modules": {
    "lead":    { "slots": [{ "span": 6, "headline": ["xl","l"], "maxLines": 4, "dek": true, "meta": "full" }] },
    "colead":  { "slots": [{ "span": 6, "headline": ["l","m"], "maxLines": 3, "dek": true },
                           { "span": 6, "headline": ["l","m"], "maxLines": 3, "dek": true }], "rule": "between" },
    "cluster": { "anchor": "lead", "follow": { "max": 4, "headline": ["s"], "indent": true, "label": "Developing" } },
    "pair":    { "slots": [{ "span": 3, "headline": ["m","s"], "maxLines": 4 }, { "span": 3, "headline": ["m","s"], "maxLines": 4 }],
                 "degrade": "stack" },
    "briefs":  { "list": { "max": 6, "headline": ["s"], "dek": false } },
    "river":   { "bySection": true, "perSection": 4, "headline": ["s"], "kicker": "section" },
    "opinion": { "list": { "max": 4, "headline": ["s"], "byline": true } }
  },
  "rules": [
    { "if": "top.weight >= 1.6 * second.weight", "use": "lead" },
    { "if": "top.weight < 1.15 * second.weight", "use": "colead" },
    { "if": "related(lead) >= 2", "use": "cluster" },
    { "else": "pair" },
    { "then": "briefs" },
    { "then": "river" },
    { "then": "opinion" }
  ]
}
```

How it is rendered:

1. **Weights** come from the ranking (the web's importance mode, depth
   factor and recency, `L/autopaper.ts:293,307`, plus the listener's shading
   of section 1), so size and position express rank.
2. **Rules** walk the ranked stories band by band and choose a module for
   each band. A band's stories leave the pool.
3. **Fit.** For each slot the engine measures the headline at each allowed
   step for the slot's width and takes the largest step within `maxLines`;
   a dek is kept when the slot's area allows. A module whose slots all fail
   their fit uses its `degrade` form (a pair becomes two stacked items). On
   a phone (353 pt of column) a 118-character headline is seven lines in a
   half-width slot, so most pairs stack; on the web or an iPad the same
   document lays them side by side. Measuring text to choose a size is
   exactly the job `pretext` does on the web; on the phone the kit measures
   with Core Text.
4. **Clusters** gather stories about the lead's subject under it as a
   "Developing" list, using the related-stories reasons
   (`same_incident`, `later_development`, `same_subject`, `entity`;
   `TX/texture-related-stories.ts:18`) or shared central entities.
5. **Art.** No published story carries an image today (embeds are refused
   at publication, `internal/texturedoc/embed.go`); modules with an art
   slot become eligible when one does.

Where it runs: a pure function in the kit (stories, layout document, width,
a measurer) returning bands of placed slots; Contract draws each band as one
of a fixed set of module views. The same function in TypeScript with
`pretext` as the measurer would give the web the same page (a back-port).

Every story on the page is a card in the theme's elevation system
(elevation 1, pressable with `-exact-press-scale`, CFA:4492-4496), so the
front page and the pile share one visual language: on a flat theme the
card is only its rule; on a deep theme it lifts.

### Three worked layouts from real editions

From `front?limit=60` and `front?section=tech_ai&limit=30` at about 00:45
UTC on 2026-11-11 [run].

**1. Everything, a dominant story with follow-ups.** Nine stories at
importance 8; one has 27 sources and 22 versions ("Russia and Ukraine trade
strikes overnight; four dead as attacks on energy grid…"), far above the
next by depth, so `lead`. Four more importance-8 stories share its subject
("Russia, Ukraine and the US open talks in Abu Dhabi…", "Rubio says progress
made on post-war security architecture…", "Ukraine says large prisoner
exchange…", "Zelensky disputes Russian Donbas claims…"), so `cluster`
"Developing" under it. Next two are close in weight ("Paraguay's Senate
ousts President Peña…", "Senate Democrats mount December shutdown
threat…"): `pair`, which degrades to stacked on the phone (118 and 119
characters). Then `briefs` (Trinidad and Tobago ruling, the SNAP proposal,
the Hegseth vote…), a `river` by section (U.S., Technology, Business), and
Opinion.

```text
 WORLD · Updated 4 min ago
 Russia and Ukraine trade strikes      ← xl, 4 lines
 overnight; four dead as attacks on
 energy grid…
 dek … · 27 sources · 22 versions
   Developing
   · US, Russia, Ukraine open talks in Abu Dhabi
   · Rubio: progress on post-war security
   · Large prisoner exchange discussed
   · Zelensky disputes Donbas claims
 ───────────────────────────────
 Paraguay's Senate ousts President     ← m, stacked pair
 Peña over collapse of Ueno Bank
 Senate Democrats mount December
 shutdown threat over Iran war funding
 ───────────────────────────────
 IN BRIEF  · · · · · ·
 U.S. ▸  four headlines        TECHNOLOGY ▸ …
```

**2. Technology section, two stories about one person.** The top two are
both about Nvidia's chief executive and close in weight (14 sources each,
12 and 13 versions): `colead` with a rule between, not a lead and a
secondary. "Grok Build apps exposed confidential… data" has only two
sources (depth factor 0.75), so it drops to briefs despite importance 7.
Four security stories (Barracuda, SAP NetWeaver, fake GitHub tools,
eMudhra certificates) share a subject and would form a labelled band
"Security" once entities are on the route; until then they fall into the
briefs in rank order.

**3. A quiet hour.** When no story is 1.15 times the next and fewer than
ten stories are fresh, the rules choose no lead: a `pair` and a `river`.
An honest small page instead of a weak story set large.

### The story stack: an additional mode

"One by one" in Paper's header (and "Go through today's stories" from the
bar's idle state on the front page) shows the edition as a stack of story
cards in rank order: tap opens the story as a page, left closes it (neutral
by default; variant (b) makes it a pass), right is more of this (section 1). The bar stays; its trailing button reads the top
card. "Undo" on the bar restores the last card for 5 s; passed stories are
listed at the end of the stack ("Set aside, 4") and remain on the front
page, which is always one tap away ("All stories"). Scrolling past a story
on the front page writes nothing.

## 4. Pages and the reader

### One viewer, a renderer per type

| Type | Renders with | In Exact today | Needs |
|---|---|---|---|
| Texture (story or own document) | our reader: `body_doc` blocks as Contract text runs | native: parent `text` with child runs (`exact:apps/exact-live/app.contract:857-878`) | the renderer (build) |
| Markdown, plain text | the same reader (converted to blocks) or `text markup="markdown"` (the read-only reader; `textarea` is the editor) | native (CFA:2213-2236) | none |
| Image | `image` from an `app:/cache` file | native; remote URLs carry no auth header, so the kit downloads with `fetch(…, {exactBodyTo})` first; SVG does not draw on Apple; no pinch zoom (CFA:1837-1907) | pinch zoom by hand with `pan` and `scale`, or a package view |
| Video | `video` (AVPlayerLayer), controls, picture in picture, full screen | native (CFA:2115-2119; `exact:host/apple/videoarm/VideoArm.swift:394-402,850-880`) | none |
| Audio | `audio` with `metadata=` for Now Playing | native; needs `audio_session: "playback"` and the `audio` background mode (CFA:2154-2206) | none |
| PDF | `iframe src="app:/cache/x.pdf"` shows PDFKit's `PDFView`, without a toolbar or fit controls | partial (`exact:docs/agent-pitfalls.md:1414-1418`; `exact:host/apple/Sources/ExactKit/WebModule.swift:308-310,496-500`) | a package view on PDFKit for page fit, search and position |
| EPUB | not found in Exact | none | recommended: convert chapters (XHTML) to the reader's blocks, so books get the same typography (question 1); else a package reader |
| Mail message | text part in the reader's type; headers as a page header | native for text | "Show original" for HTML: a package `WKWebView`, scripts and remote loads off (question 2) |
| Office files, other | Quick Look (`QLPreviewController`) | not found | a package view |
| Folder | a list page | native | none |

A page's header (title, type, source) scrolls away with the content; the
page's own actions are in a menu reached from its title or by long-pressing
its card, so nothing but the bar is fixed on screen.

### Citations and transclusion, designed

- **Inline mark.** A numbered pill in the sans face at 0.75em, tabular
  digits, accent tint on a soft accent field, on the baseline (not
  superscript; Exact's `vertical-align` was not verified). Consecutive
  marks group into one pill "3 · 4". The pill pads its tap area to the line
  height; a missed tap on the paragraph selects nothing else, so a near
  miss on a mark still opens it [unrun: measure on the phone].
- **Mechanism candidates [unrun].** In a Markdown-sourced page the mark can be
  an atom pill (`atom citation link="cite:"`, `atoms="citation"`, tap =
  `atompress`; `docs/reference/exact/capabilities.md` section 5), since no
  footnote tap is documented. In the block reader it is a run with `press=`.
  On a wide screen the expanded quote may be a side note that the paragraph's
  text flows around (`wrap-flow`, `shape-outside`; capabilities section 4); on
  a phone column it stays the stacked card below.
- **Tap: expand in place.** A quote card is inserted after the paragraph
  and pushes the text below it down (the web's own stacked mode). It shows
  the source host and title, the pinned quote with its check badge
  ("Exact quote", "Close match", "Not yet checked"), and "Open source".
  Tap the mark again, or the card's close, to fold it. Several can be open.
  `-exact-layout-transition` animates the push; reduced motion jumps.
- **Open source: the source panel**, an authored layer above the bar (a
  UIKit sheet would cover the bar), opening at about half height: title,
  host, date, the quote highlighted inside its source block
  (`resolved_blocks`), Open original (system browser), and for a signed-in
  reader the stored reader snapshot. Dragged to full height it shows the
  whole snapshot.
- **Texture-to-Texture transclusions** (`texture:<doc>@<rev>`) are inline
  chips with the target title; tap pushes that document at the pinned
  version, with "A newer version exists" in its version line.
- **Read aloud.** Marks are not spoken. When a paragraph with citations
  plays, the bar's expanded player shows its source hosts as chips under
  the passage ("developer.apple.com · macrumors.com"); tapping one opens
  the source panel without stopping the radio. Saying "what's the source"
  is an ordinary turn the desk answers.
- **Sources** at the end of every document: numbered, host, title, and its
  quotes; the same list from the page menu.

The research behind this (what `pretext` does on the web, the data, and
Exact's text model) is appendix B.

## 5. Visual system: type, depth, motion and themes as data

### Faces (theme `type.read`, `type.ui`, `type.mono`)

| Token | iOS | Web (back-port) | From |
|---|---|---|---|
| `font.read` (body, headlines, standfirst) | `ui-serif` (New York, with optical sizes) | `'Iowan Old Style', Charter, 'Sitka Text', Cambria, Georgia, serif` | Autopaper's `--ap-serif`, `AP/style-design.js:8-11` |
| `font.ui` (labels, meta, controls, kicker, h2-h3 in documents) | `system-ui` (SF Pro) | `Inter, ui-sans-serif, system-ui, sans-serif` | `--choir-font-ui` |
| `font.mono` | `ui-monospace` | `ui-monospace, SFMono-Regular, Menlo, monospace` | `--choir-font-mono` |

Change from the web: Texture's body is Inter today
(`TX/App.svelte:2413-2437`); the body face is the theme's `type.read`
(owner, 2026-11-11: "the body text font should be part of the theme"), so
no component names a font. The default phone theme reads in the serif, as
Autopaper's headlines already do; another theme may read in a sans. Optical sizing is
iOS-only (New York's `opsz`); the web stack has none.

### Type scale

| Token | Face | Size | Weight | Line height | Other | Web source |
|---|---|---|---|---|---|---|
| `type.nameplate` | read | 2.25rem | 700 | 1.0 | tracking −0.01em; 3px double rule under | `AP/App.svelte:~390` |
| `type.kicker` | ui | 0.75rem | 700 | 1.2 | uppercase, tracking 0.08em, accent | section tabs `AP/App.svelte:~440-470` |
| `type.headline.lead` | read | 1.875rem | 700 | 1.08 | `text-wrap: balance` | lead 2rem phone, `AP/AutopaperStory.svelte:~125-145` |
| `type.headline.doc` | read | 1.75rem | 700 | 1.12 | balance | |
| `type.headline.top` | read | 1.3125rem | 650 | 1.16 | balance | top 1.22-1.36rem |
| `type.headline.brief` | read | 1.0625rem | 600 | 1.25 | | brief 1.02rem |
| `type.standfirst` | read | 1.1875rem | 400 | 1.42 | `color.ink-soft` | lead dek 1.12rem, lh 1.5 |
| `type.body` | read | 1.125rem | 400 | 1.6 | hyphens auto; widows and orphans 2 | document lh 1.72; reader 18px, lh 1.7 (`packages/reader/reader.css:349-373`) |
| `type.h2` | ui | 1.1875rem | 650 | 1.25 | tracking −0.01em; space above 1.5em | h1-h4 lh 1.18, −0.03em |
| `type.h3` | ui | 1.0rem | 650 | 1.3 | | |
| `type.quote` | read | 1.125rem | 400 italic | 1.55 | 2px accent rule, indent 1rem | |
| `type.list` | read | as body | | | marker in `font.ui`, tabular; hanging indent by negative `text-indent` | |
| `type.meta` | ui | 0.8125rem | 500 | 1.3 | tabular digits, `color.muted` | meta 0.72rem |
| `type.cite` | ui | 0.75em of its line | 700 | 1 | tabular, pill, accent | marker 0.62em, 820 |
| `type.label` | ui | `-exact-subheadline` (web 0.9375rem) | 500 | | | |
| `type.player.passage` (expanded player) | read | 1.375rem | 400 | 1.45 | current sentence field | |

Rhythm: paragraphs separated by 0.85em (no first-line indent: a phone
column is narrow and paragraphs are short); heading space above 1.5em,
below 0.5em. Measure: `max-width: 34rem` on the body column (about 66
characters of `type.body`); on the web the same token can be written
`66ch` because there `ch` is the face's width (iOS-dependent difference:
Exact's `ch` is 8 px, `exact:kernel/src/style/cells.rs:1-30`).

### Elevation

Five levels. A theme gives each level its cues; Exact can draw every cue
listed, including a small 3D tilt (`rotate` with a `perspective` parent; the
angle animates, the axis does not).

| Level | What | Cues in the default theme (Depth) | In a flat theme (Newsprint) |
|---|---|---|---|
| 0 | the page | none | none |
| 1 | a story on the front page, a quote card, a list row that is pressable | a light top edge (1 px inset highlight), a soft two-layer shadow; pressed: scale 0.98 | a hairline rule; pressed: tint |
| 2 | the bar; cards in the pile | glass material, a stronger shadow, a light edge | a solid paper fill and a hairline top rule |
| 3 | layers: the launcher, the pile's backdrop, the expanded player, the source panel | material, the deepest shadow; the page behind scales to 0.96 and dims by 8% | solid fill; the page behind dims only |
| 4 | system sheets and alerts | the platform's | the platform's |

Parallax is limited to the scale-behind of level 3 and the deck's offsets;
scroll-driven parallax waits until `scroll=` proves cheap enough [unrun].

### Motion and the TetraMark

**The mark.** The TetraMark (`L/tetramark.ts`, `L/TetraMark.svelte`) is
three straight-edged polygons in a 512 viewBox: a leaning spire, a long
left shard and a flat base. Read as a drawing, it is a tetrahedron in
hand-drawn perspective: three of its six edges drawn as tapering shards
with gaps where they would meet.

**The living mark.** A real tetrahedron (four vertices, six edges) whose
rest orientation, under perspective, projects onto the drawn mark. Each
edge is drawn as a shard, not a line: a quad that tapers from one end to
the other, its cut ends angled and its widths taken from the three drawn
shards, so the character survives rotation. Front edges are full colour,
back edges at 35%, faces unfilled except as progress (below). It sits in
the bar's trailing voice button (28 pt), large on onboarding and the empty
states, and small beside any wait in a page.

| State | Motion of the mark | Why |
|---|---|---|
| Idle | at rest in the drawn pose; a drift of one turn a minute about the vertical axis | alive, never busy |
| Speaking (the radio reads) | turns about the vertical axis at a speed following the voice's loudness; edges brighten on stressed syllables | the radio's voice made visible |
| Listening | turns an apex toward the listener, grows to 1.15; edge width follows the input level | "I'm hearing you" |
| Heard | one fast flip about the horizontal axis (a spring, about 400 ms) | the acknowledgement, before any word |
| Thinking, holding (the desk works) | tumbles on two axes at incommensurate speeds, so it never repeats; each real desk step fills one face | waiting that tells the truth |
| Revision arriving | snaps to the rest pose with a spring overshoot and a flash along the edges | arrival |
| Could not | settles flat, a short shake, danger tint | failure, without alarm |
| Reduce Motion | the static drawn mark; states shown by tint, a face fill and the bar's words | the system setting wins |

**The ~15 s wait, alive and truthful.** Where the listener spoke (or at the
top of a document being revised) the page shows a working block: the small
mark tumbling, one line saying what the desk is doing now, and the sources
it is reading arriving as chips ("macrumors.com", "developer.apple.com").
Each line comes from a real event of the document's lifecycle stream
(`synth_started`, `synth_progress`, `revision_created`, `head_changed`;
`TX/texture.js:298`, `TX/lifecycle.js:22,93`); the four faces fill as
started, reading, writing and checking are reported. Never a fake percentage
or a timer posing as progress. When the revision exists, its new blocks are
revealed into the page at reading pace (about 40 words a second) while the
radio reads them. Meanwhile the radio's holding lines keep the listener
company (turns note: never silence). Whether `synth_progress` names the
step and the sources is not verified; if it does not, that is a server gap
(appendix C).

**Other motion.**

- **Cards.** Throws follow release velocity (`panrelease`, CG:1862-1864);
  a dragged card tilts up to 8 degrees; springs settle the deck
  (`-exact-spring(stiffness, damping, mass)`, `exact:docs/recipes/motion.md`);
  the deck's offsets and scale give it depth; the page-to-card flight uses
  `sharedElement=`.
- **The front page composes itself.** On load the bands rise in rank order
  with a 40 ms stagger; on a re-rank stories move to their new places with
  `-exact-layout-transition`; a story that just updated pulses its kicker
  once.
- **Text arriving.** New blocks rise 8 pt and fade in; the listener's own
  words set from italic to roman when heard.
- **The bar.** Its height animates as it grows for words (a numeric height
  animates, `motion.md`); the launcher rubber-bands; the Undo pill slides.

**What Exact can draw and animate.**

| Need | Exact | Evidence | Verdict |
|---|---|---|---|
| Transitions, keyframes, springs, exit and layout animation | yes | `exact:docs/recipes/motion.md` [doc] | native |
| 3D transforms (perspective, rotate with axis, translate z) | yes on boxes: `rotate="y 40deg"`, third `translate` length, parent `perspective`, `backface-visibility`; drawn as `CATransform3D`. No `preserve-3d`, `matrix` or `skew`; only the angle animates | `exact:scripts/fixtures/space.contract:15-33`; `exact:host/apple/Sources/ExactKit/SpaceTransform.swift:1-45` [src]; corrects CFA:4163, which lists 2D only | native for flat cards; a solid needs a package view or a Contract projection |
| Per-frame custom drawing | Canvas 2D `canvas surface=name(args)`; `app.ts` exports `draw(surface, args, ctx, frame)` and returns `true` to be drawn again next frame; Core Graphics on iOS | `exact:apps/canvas-gallery/app.ts:1-17`; CG:1940 [src, doc] | native but the last fallback for the mark: it calls Hermes every frame (see the mark's ranking) |
| GPU drawing (Metal) | the GPU module (`exact-gpu`, a Rust crate with WGSL shaders, wgpu: Metal, Vulkan, WebGPU) used as `canvas surface=name(args)`, loaded on demand; the Canvas 2D Vello replay and SVG filters need the Metal Toolchain (not installed here; `xcodebuild -downloadComponent MetalToolchain`). Contract itself is not drawn by Metal. Android draws nothing for a GPU canvas with children | `exact:docs/reference.md:142` [doc]; `exact:host/apple/build.mjs:147-163, 872`; `exact:host/android/README.md:1254` [src]; `docs/reference/exact/capabilities.md` | ranked second for the mark (below); build cost unmeasured |
| Text around a figure or note | `wrap-flow="both"`, `shape-outside`, `shape-margin` on the obstacle box beside a `text` body | `exact:apps/textflow/app.contract`; `exact:README.md:597` [src] | native on iOS per the README; unrun by us |
| Timers per frame | `every(ms, action, wait=frame)` | `exact:docs/recipes/timers.md:72` [doc] | native |
| Lottie-style files | not found | | not available; the mark is code |
| SVG geometry animation | yes | `motion.md` "What animates" [doc] | native: the static and reduced-motion mark |

**How the mark and the waiting animation are built, in ranked order**
(owner, 2026-11-11: "i would rather have as much in exact as possible";
`docs/reference/exact/rendering-and-motion.md` section 8a and
`capabilities.md`; all unrun). The ranking for both:

1. Pure Contract: projection `fn`s, `svg polygon` and an `every(frame)` task
   for the mark; Contract transitions, springs and keyframes for the wait.
2. If richer visuals are wanted: an Exact wgpu GPU surface in Rust with WGSL
   (portable Metal, Vulkan and WebGPU). Android draws nothing for a GPU canvas
   with children, so the surface has none; build cost is unmeasured.
3. The game add-on only if a scene graph, particles or physics are needed.
4. A Swift or Core Animation view last, iOS-only.

Glass and depth stay in Contract: `backdrop-filter`, materials
(`backgroundMaterial`) and `box-shadow`; no GPU surface is needed for them.

The detailed ranking of the mark, as first written:

Owner, 2026-11-11: "ideally we can do pure contract so we can easily run
on other platforms (android eg)." The research ranked by iOS frame cost
alone; portability outranks that. UI is written in Contract. A Swift
package view is for what has no Contract path (audio, models, system
services), not for drawing.

1. **First choice: pure Contract.** `sin`/`cos` projection in `fn`s, `svg
   polygon` faces with back-face culling by signed area, ticked by an
   `every(frame, step)` task. No TypeScript and no Swift, themed by `th`,
   and the same on every Exact host. The per-frame work runs in Exact's
   runner (not JavaScript): one string and one path update per face.
2. **Only if measured too slow or too hot on the phone: a Swift package
   view** (`CATransformLayer`, four faces, Core Animation) behind the same
   `Mark` component, as an iOS-only optimisation with the Contract mark kept
   as the portable form. The measurement and its numbers are recorded
   before this is taken.
3. **Not used: Canvas 2D from TypeScript** (a Hermes call per frame).

**Motion belongs to the theme.** The theme's `motion` block carries the
durations, easings and springs above and a `mark` block (drift, tumble
speeds, flip spring, breath scale), each with a reduced-motion value
(section 5, themes).

### Themes as data: one format for the web and the phone

**What exists.** The web's theme is JSON, schema version 2
(`L/theme.ts:3-21`): `id`, `name`, 45 `colors`, five `radii`, `motion`,
`layout`, `fonts` and six `effects`, all as CSS strings; stored opaque at
`GET`/`PUT /api/preferences/theme` with a `theme.updated` event for other
devices (`internal/mediastate/handler.go:249-320`); seven presets, the
default Aero Chrome Heart (`L/theme.ts:298,334`); a strict form for themes a
package contributes, with a contrast gate (`L/theme.ts:497-537`). Light or
dark is not in the JSON (the web sets `color-scheme` by theme id in
`frontend/src/app.css:2,24,487,511`), and window chrome and textures are CSS
keyed on six preset ids (`app.css:486-573`), so a new theme gets neither.
The decision to make themes Textures is already recorded
(`docs/design/autopaper-next-mission-handoff-2026-11-09.md:134,359-361,505`),
and the Autopaper Style Guide is the precedent: a Texture parsed on the
server into a closed vocabulary (`internal/textureowner/style_owner.go`,
`internal/styleguide/styleguide.go`, `GET /api/texture/style`).

**Proposal: theme schema version 3.** A superset of version 2, so every
existing theme still loads on the web. What version 3 adds is what the
phone needs and the web lacks: an explicit scheme, both schemes in one
theme, type, elevation and materials as data.

```jsonc
{
  "schema_version": 3,
  "id": "depth",
  "name": "Depth",
  "schemes": ["light", "dark"],          // which sides it defines; v2 themes are one side, inferred from bg luminance
  "colors": {                            // the v2 keys, each a colour or { "light": c, "dark": c }
    "paper":  { "light": "#FFF7FE", "dark": "#120D17" },   // v2 "panel"
    "paper2": { "light": "#FBEFFA", "dark": "#1C1523" },   // v2 "bg2"
    "fg":     { "light": "#20143A", "dark": "#F3ECF6" },
    "muted":  { "light": "#55396F", "dark": "#B9A8C7" },
    "accent": { "light": "#A60072", "dark": "#F263B9" },
    "accent2":{ "light": "#7A0A57", "dark": "#FF9BD6" },
    "success":{ "light": "#0C6644", "dark": "#4FD3A0" },
    "danger": { "light": "#A3142F", "dark": "#FF6B7B" },
    "onAccent": "#FFFFFF",
    "rule":   { "light": "rgba(32,20,58,.22)", "dark": "rgba(243,236,246,.18)" }
  },
  "type": {                              // faces as generic-first stacks; the reader maps generics per platform
    "read": "ui-serif, 'Iowan Old Style', Charter, Georgia, serif",
    "ui":   "system-ui, Inter, sans-serif",
    "mono": "ui-monospace, Menlo, monospace",
    "scale": "editorial"                 // a named scale (the type table above); a theme may override single steps
  },
  "radii": { "card": "14px", "control": "12px", "pill": "999px", "rules": "0px" },
  "elevation": {
    "1": { "shadow": "0 1px 2px rgba(32,20,58,.08), 0 6px 16px rgba(32,20,58,.08)", "edge": "inset 0 1px 0 rgba(255,255,255,.6)", "press": 0.98 },
    "2": { "material": "glass", "shadow": "0 8px 24px rgba(32,20,58,.18)", "edge": "inset 0 1px 0 rgba(255,255,255,.5)" },
    "3": { "material": "glass", "shadow": "0 18px 48px rgba(32,20,58,.24)", "behind": { "scale": 0.96, "dim": 0.08 } }
  },
  "motion": { "fast": "120ms ease", "enter": "320ms cubic-bezier(0.16,1,0.3,1)", "sheet": "260ms cubic-bezier(0.2,0.8,0.2,1)",
              "spring": { "stiffness": 240, "damping": 26, "mass": 1 }, "stagger": "40ms",
              "reduced": { "enter": "0ms", "spring": "none" } },
  "mark": { "drift": "60s", "tumble": ["3.3s", "5.9s"], "flip": { "stiffness": 300, "damping": 22 }, "listenScale": 1.15,
            "reduced": "static" }
}
```

```jsonc
{
  "schema_version": 3,
  "id": "newsprint",
  "name": "Newsprint",
  "schemes": ["light", "dark"],
  "colors": {
    "paper":  { "light": "#F7F5F0", "dark": "#0E0E0E" },
    "paper2": { "light": "#F7F5F0", "dark": "#0E0E0E" },
    "fg":     { "light": "#111111", "dark": "#EDEDED" },
    "muted":  { "light": "#555555", "dark": "#A3A3A3" },
    "accent": { "light": "#B3261E", "dark": "#FF6E5E" },
    "accent2":{ "light": "#7F1A15", "dark": "#FFA095" },
    "success":{ "light": "#1E5E3A", "dark": "#5CC48A" },
    "danger": { "light": "#B3261E", "dark": "#FF6E5E" },
    "onAccent": "#FFFFFF",
    "rule":   { "light": "#111111", "dark": "#EDEDED" }
  },
  "type": { "read": "ui-serif, Georgia, serif", "ui": "ui-serif, Georgia, serif", "mono": "ui-monospace, monospace", "scale": "editorial" },
  "radii": { "card": "0px", "control": "0px", "pill": "0px", "rules": "0px" },
  "elevation": {
    "1": { "shadow": "none", "edge": "0 1px 0 rule", "press": 1.0, "pressTint": true },
    "2": { "material": "solid", "shadow": "none", "edge": "0 -1px 0 rule" },
    "3": { "material": "solid", "shadow": "none", "behind": { "scale": 1.0, "dim": 0.12 } }
  },
  "motion": { "fast": "0ms", "enter": "160ms ease", "sheet": "200ms ease", "stagger": "0ms" },
  "mark": { "drift": "none", "tumble": ["6s", "11s"], "flip": { "stiffness": 200, "damping": 30 }, "listenScale": 1.0,
            "reduced": "static" }
}
```

**Rules for both readers.**

- Values are a restricted CSS subset both can parse: hex and `rgb(a)`
  colours; `px` lengths; shadow lists; `cubic-bezier` and named easings;
  font stacks whose first entry is a generic. Gradients and the web's
  window-chrome keys stay legal in version 3 but are web-only.
- Contrast gate on every scheme a theme defines (the web's pairs:
  `fg` on paper 4.5, `muted` on paper 4.5, `onAccent` on accent 3), on the
  server as well as in each client.
- **A theme is a Texture or a file.** A document with one fenced
  `choir-theme` JSON block, parsed and validated on the server the way the
  Style Guide is, served as `GET /api/texture/theme?doc=` (server gap) and
  chosen by reference in the preference: `{"theme": {"ref":
  "texture:<doc>@<rev>"}}` or a preset id. A theme made on the web works on
  the phone and the reverse, and a theme can be shared, forked and
  published like any Texture.

**What the phone ignores or maps.** It ignores `layout`
(`promptSurfacePlacement`: the bar is always at the bottom;
`deskSheetHeight`), `effects.texture` (a web texture name), the landing
canvas `field*` colours, `chart*` and the window motion durations. It maps
`fonts.ui` and `fonts.display` to `type.ui` and `type.read` by their first
generic (Inter to SF Pro, Georgia and Iowan to New York), the desk colours
to the holding lines and the revision rule, `effects.blur` above 0 to
material `glass`, `effects.shadowFloating` to elevation 2's shadow. A
version 2 theme has one scheme; the phone shows it in that scheme only.

**How the phone applies a theme** [unrun on a device]. Shipped themes
compile into `app.tamber` and switch with `setMode`. A theme from data binds
each role through a function from the theme resource to a colour string
(CFA:4718). The binding mechanism is proven upstream (Spark, design-tool and
canvas-gallery bind style attributes from data); our own bindings have not
been run on a device. If a bound colour string does not paint, a data theme needs an
app update, which is an Exact gap to record and take upstream; the format
and the web are unaffected.

Space, radius and motion beyond the theme: a 4-point spacing scale (4, 8,
12, 16, 20, 24, 32, 40, 48), a 20 pt side gutter, touch targets of at least
44 by 44; every motion gated on reduced motion
(`exact:docs/recipes/motion.md`). The mark is the TetraMark's three
polygons (`L/tetramark.ts`, viewBox 512) drawn as an `svg` path in
`accent2`: sign-in, empty states, the app icon. Our own asset.

## 6. How we build it: a spine, then a fan-out

Owner, 2026-11-11: build the whole thing end to end at once, accept about
85%, then review and iterate. So: one small shared spine, built in one
sitting by one builder, then nine lanes in parallel, each in its own
worktree, split along file boundaries so no two lanes edit the same file.
The integrator lane (engine, Components, Trace, audio session) continues
alongside and owns its files.

An Exact lesson learned by any lane goes into docs/reference/EXACT.md or its subpages, not a lane README.

### Where today's code would collide, and the spine's answer

| Single file today | Why lanes would collide | Spine change |
|---|---|---|
| `mobile/AutoradioLab/app.contract` (one root) | every lane needs resources, actions and views | the root stays small: routes, the page host, the bar and layer hosts. Each lane owns a `members "./<lane>.contract"` file for its resources and actions (CFA:690-695) and its own component files imported with `use … from "./<file>.contract"` (CFA:279,762-764). Child components take props and cannot hold resources (CFA:672-688), hence the members files |
| `mobile/exact-package/apple/AutoradioModule.swift` (one `later` switch) | every source and send is a case | a dispatch table: each lane registers its handlers from its own Swift file (`AutoradioModule+Paper.swift`, …) |
| `mobile/scripts/gen-exact-interface.py` (one shapes table) | every lane adds shapes and names | the script merges per-lane fragments `mobile/exact-package/interface/<lane>.json` |
| `mobile/AutoradioLab/strings/en.json` | every lane adds strings | key prefixes per lane (`paper.*`, `reader.*`, …); the spine adds a check for duplicate keys |

### The spine (one sitting)

1. **Routes and the page host**: a page is `{address, kind, title}`;
   `kind` picks the renderer; the host shows the top page full screen under
   the bar. Addresses: `paper:front`, `paper:section/<id>`,
   `pub:<route>`, `texture:<doc>[@rev]`, `file:<path>`, `mail:<id>`,
   `mailbox:<folder>`, `textures:`, `activity:`, `settings:<screen>`.
2. **The page-viewer interface**: a renderer is a component taking the page
   and a `PageModel` resource answered by the kit; the spine ships two
   trivial renderers (plain text, "not yet") so every address opens.
3. **Open pages and navigation state** in the kit (`OpenPages.swift`): open,
   close, focus, opener, pin, persisted list of addresses; sources
   `openPages` and sends `pageOpen`, `pageClose`, `pageFocus`.
4. **The bar shell and the layer host**: the three zones and the state
   machine as data (`barState` source: idle, playing, listening, heard,
   holding, revised, typing, error), with plain placeholders for each
   state's look; the layer host for launcher, pile, player and source panel.
5. **Card and Layer components** with the elevation levels as props.
6. **The theme loader**: `app.tamber` with the two worked themes as
   compiled tokens and the role names of section 5; the data-theme path
   behind it once the spike shows bound colours paint.
7. **Kit interfaces each lane calls**: `PageSource` (load an address to a
   page model), `Signals` (record a gesture event), `Demo` (the demo
   listener fixture mode: URLProtocol stubs for every signed-in route, with
   fixtures in `mobile/fixtures/demo/`).
8. **Conventions**: testIds `<lane>-<thing>[-<id>]` (`bar-voice`,
   `card-<address>`, `paper-story-<pubid>`, `reader-cite-<anchor>`); every
   interactive element has one; the simulator proof keeps the integrator's
   ids or is updated in the same commit.
9. **The three spikes** decide the spine's details: a glass bar over a
   full-bleed scroll with the keyboard; a card dragged left and closed with
   `pan`; a colour bound from data. A failed spike's fallback is written
   into the spine before fan-out.

Done when: the app opens on a placeholder front page under the bar, a
second page opens and the pile shows two cards, one closes by swipe, the
theme switches light, dark and Newsprint, and demo mode serves a fixture.

### The lanes

| Lane | Owns (files) | Consumes from the spine | Done when (on the simulator) | 85% means: must work / may be rough | Server |
|---|---|---|---|---|---|
| **1. Bar, radio, talk** | `bar.contract`, `player.contract`, `members-bar.contract`, `AutoradioModule+Bar.swift`, `Kit/BarState.swift` | bar shell, layer host, the integrator's engine and `ChoirApp` | every bar state is reachable and looks right; a story plays from the bar; the expanded player follows the paragraph; hold to talk runs a demo turn through all six states | must: states, play/pause, talk, follow-along by paragraph / rough: sentence highlight, chips of sources | none |
| **2. Paper layout** | `paper.contract`, `members-paper.contract`, `Kit/Layout/*.swift` (engine, Core Text measurer, rules), `mobile/layouts/default.json` | page host, Card, `PageSource` | the real front page composes itself from the live edition; sections, paging, opinion; the three worked cases render as designed | must: lead, colead, cluster, pair (with degrade), briefs, river; real data / rough: art slots, iPad widths | ranked front (later), entities |
| **3. Texture reader** | `reader.contract`, `members-reader.contract`, `Kit/TextureDoc.swift` (body_doc to blocks and runs), `Kit/Citations.swift` | page host, `PageSource`, Layer (source panel) | a real story renders every block type in the type system; citations expand in place; the source panel; versions; related; share; a private document from demo mode | must: all block types, citation cards, Sources list, versions / rough: texture-to-texture chips, embeds | block-addressed prompt; `request_id` on version events (for lane 1's revision placement) |
| **4. Cards, launcher, signals** | `cards.contract`, `launcher.contract`, `members-cards.contract`, `Kit/OpenPagesUI.swift`, `Kit/Signals.swift`, `Kit/Preferences.swift` | OpenPages, Card, Layer, `Signals` | the deck with depth; swipe left closes, right marks a story; Undo; long-press menus; the launcher with live place rows and search; the story stack mode; gesture events recorded; "Why this story" | must: deck, gestures, launcher, events / rough: search ranking, shading weights | entities on front stories |
| **5. Files and viewers** | `files.contract`, `viewers.contract`, `members-files.contract`, `Kit/ChoirFiles.swift`, `Kit/Epub.swift`, package views `PDFPage.swift`, `QuickLookPage.swift` | page host, `PageSource` | browse folders, open text in the reader, image, video, audio and PDF pages, an EPUB as blocks, Quick Look for the rest; upload, rename, delete | must: browse and open every type / rough: PDF search, EPUB tables and images, image zoom | none |
| **6. Mail** | `mail.contract`, `members-mail.contract`, `Kit/ChoirMail.swift`, package view `MailOriginal.swift` | page host, `PageSource` | inbox and folders, a message as a page, reply and compose as drafts, attachments, the approval link | must: read, draft, attach / rough: original HTML view, threading | send and digest accepted from a key |
| **7. Settings, themes, accounts** | `settings.contract`, `members-settings.contract`, `Kit/ThemeLoader.swift`, `mobile/themes/*.json`, `Kit/Accounts.swift`; restyles the integrator's Components and Trace (their files stay theirs) | theme loader, page host | themes listed and switched (and loaded from a Texture in demo mode); accounts and devices; talk mode; Computer (status, Wake, Restart to update); What the paper has learned; gesture meanings as components | must: themes, account, sign out, Computer / rough: theme from a Texture on staging | bearer "who am I"; `GET /api/texture/theme` |
| **8. Onboarding and the interview** | `onboarding.contract`, `members-onboarding.contract`, `Kit/Interview.swift`, `mobile/interview/questions.md` (the default bank) | bar, page host, the integrator's engine, lane 3's reader for the edition page | sign-in presented; voice setup (model download, per-device checks, fallbacks; with lane M's `ModelStore`); the open-microphone, interruptible interview on the integrator's turn engine, by voice only (demo mode drives it with recorded clips); the edition written (demo) and shown; then the front page and a story playing | must: the whole path and its recoveries / rough: question wording, read-back phrasing | edition marker; the bank as a desk package document (later) |
| **9. Motion and the mark** | `mark.contract`, the mark in pure Contract (`fn` projection, `svg polygon`, frame task; a `CATransformLayer` package view only if measured too slow; no canvas `draw` in `app.ts` only as a last fallback), motion tokens in the themes | bar shell (the voice button), theme loader | the 3D mark in every state; the alive wait with real lifecycle lines in demo mode; card physics tuned; the front page composing; Reduce Motion | must: the mark in all states, the wait / rough: syllable sync, GPU path | lifecycle progress detail (if missing) |
| **S. Server** (Go, its own branch, ring rules, Ring-Expedite per CLAUDE.md) | `internal/platform` (entities on front and bundle), `internal/textureowner` (theme route, edition marker, block-addressed prompt, `request_id` on version events), `internal/auth` (bearer "who am I"), `internal/proxy/email.go` (owner act from a key) | none | each route deployed on staging with its own acceptance check | must: entities, who-am-I, theme route / later: mail owner act (red: auth) | itself |

Exact limits and fallbacks, by lane: no layer above a UIKit sheet (lanes 1,
4, 3: authored layers); 3D exists on boxes but not `preserve-3d` (lane 9: a pure Contract projection first, for portability; a package view only if measured too slow);
no EPUB or Quick Look element (lane 5: conversion and package views); PDF
only through an iframe (lane 5: a PDFKit package view); data themes
unrun on device (lane 7: the binding is proven upstream; compiled themes first, data themes after an app update if it does not paint);
no custom accessibility actions confirmed (lane 4: visible buttons under
VoiceOver). An Exact lesson learned by any lane goes into docs/reference/EXACT.md or its subpages, not a lane README.
Each goes there with its file:line as it is hit.

Review after the fan-out: every lane's screens in light, dark and large
text on the simulator, side by side; one adjudication pass on the bar, the
deck and the front page; then a second, smaller fan-out for what is rough.

## 7. Open questions for the owner (with my recommendation)

1. **E-books.** Recommend converting EPUB chapters into the reader's blocks,
   so books get the same typography, playback and Ask; a package reader
   only if conversion fails on real books.
2. **Mail's HTML bodies.** Recommend the text part in the reader's type,
   with "Show original" in a package web view, scripts and remote content
   off. Our own pages stay web-view free.
3. **The goal file excludes Mail.** Recommend amending `excluded:` (ruling 5).
4. **One theme format.** Recommend theme schema version 3, a superset of the
   web's version 2 (explicit schemes, type, elevation, materials, motion
   and the mark), with themes as Textures parsed on the server like the
   Style Guide.
5. **Body font.** Settled by the owner, 2026-11-11: it is part of the
   theme (`type.read`). The phone's default theme uses the serif; the web
   keeps Inter until its themes carry `type.read` (the back-port).
6. **Does swipe left shade the ranking?** Default taken for the build
   (2026-11-11, the owner may overrule): swipe left closes and is neutral.
   Recommend closing as neutral
   everywhere, swipe right as "more of this", and "less of this" as a
   deliberate menu act; the meanings are a Components setting so variants
   (a) and (b) can be tried and compared from the gesture events
   (section 1). The learned preferences live in a Texture beside the
   edition ("What I've told the paper") either way.
7. **Hands-free during the interview.** Settled by the owner, 2026-11-11:
   the microphone is open for the whole interview and the listener can
   interrupt at any time; the talk mode is chosen after onboarding. What
   stays open is the speaker route: whether echo cancellation on this phone
   is good enough for open interruption without headphones (to measure).

Settled in this note unless the owner objects: the front page is composed
on the client from a layout document, with ranking on the server and the
listener's shading on the listener's computer; the desk writes layout
documents later.

## Appendix A. What the web product does today, and the phone's answer

The signed-out site is the desktop shell with Autopaper open as its home
window. `/pub/texture/<slug>[/v<n>]` opens a Texture window in published
read-only mode (`frontend/src/lib/services/links.ts:24-30,206`). The same
Texture app renders published stories and private documents (`isPublishedReadOnly`
in `frontend/src/packages/texture/App.svelte`). Default packages:
`frontend/src/packages/base-packages.json` (24).

Custodians are from doctrine I26 (`docs/choir-doctrine.md:402`) and the
modular plan §8.2 (`docs/design/modular-architecture-plan-2026-11-04.md:968-990`).
"Shell" below means the shell and owner-act services. Phone column legend:
**now** (the kit already does it), **design** (designed here, not built),
**server** (needs a server route or field; named in appendix C), **out**
(deliberately not on the phone; reason given).

Short paths: `AP` = `frontend/src/packages/autopaper`, `TX` =
`frontend/src/packages/texture`, `L` = `frontend/src/lib`.

### A1. Autopaper front page (custodian: corpusd, publications)

| Function | Route | Web source | Phone |
|---|---|---|---|
| Front list, 60 a page | `GET /api/platform/publications/front?limit&cursor&section` (public) | `L/autopaper.ts:131,139` | now (kit reads title, dek, route, section; must also read importance, source_count, words, version_count, updated_at, kind, author) |
| Ranking: recency mode, importance mode, depth factor, lead eligibility, section lift from the reader's own visits | none, client-side | `L/autopaper.ts:~100-135,293,307,395-460` | design: port `rankFrontStories` to the kit; server: a ranked front so web and phone agree (question 6) |
| Layout tiers: lead 1, top 4 with dek, briefs 6, more | none | `L/autopaper.ts:466,480` | design: replaced by the layout system (section 3) |
| Masthead, date, "Updated <relative>" | none | `AP/App.svelte:62,236-255`; `L/autopaper.ts:492` | design |
| Sections (server list), remembered choice | front `?section=` | `AP/App.svelte:129-220,257` | design: horizontal section strip under the masthead |
| Older stories (cursor) | front `?cursor=` | `AP/App.svelte:194,293,341` | design: list reaches end, loads next page |
| Opinion section, reader bylines | front `?section=opinion&limit=8` | `AP/App.svelte:350` | design |
| Per-row "Updated", time, "N versions" | none | `AP/AutopaperStory.svelte:33-70` | design |
| Reader's Style Guide (serif, weights) | `GET /api/texture/style` | `AP/style-design.js:56` | design: folded into themes as data (section 5); the Style Guide is the precedent for a theme as a Texture |
| Live refresh | none on the web | | design: pull to refresh; the phone re-reads on foreground |

### A2. The document (Texture), published and private (custodian: Texture desk for revisions; corpusd for publications)

| Function | Route | Web source | Phone |
|---|---|---|---|
| Resolve a published story (body_doc, citations, transclusions, source_entities, nav) | `GET /api/platform/publications/resolve?route=` (public) | `TX/texture.js:362`; `TX/App.svelte:1120` | now (kit reads `payload.text`); design: render `artifact.body_doc` natively |
| Body model: paragraph, heading 1-6, lists, blockquote, code_block, rule, embed; inline text with strong, emphasis, code; hard_break; `source_ref` | | `internal/texturedoc/schema.go:255-380` | design: one renderer for every block type (section 4) |
| Version navigation: older, newer, Latest/Historical label | resolve `nav{prev,next,latest}` | `TX/App.svelte:1172,1654,1991-2001` | design: a version line under the headline opens a versions layer |
| "New version available" (60 s poll, `view=nav`) | `GET …/resolve?route=&view=nav` | `TX/App.svelte:1182,191` | design: a banner at the top of the reader; the phone also checks on foreground |
| AI-written disclosure; opinion label and notice | in bundle (`publication.kind`) | `TX/texture-ai-disclosure.ts:20,46,57` | design: closing note, same words |
| Related stories ("More on this", 5) | `GET /api/platform/publications/related?route=&limit=5` | `TX/texture-related-stories.ts:18,49,105` | design |
| "More research" (ask the paper to expand) | `GET …/expansion?route=`, `POST …/expand` | `TX/autopaper-expand.ts` | design: a row at the end of a story |
| Citation marks numbered in reading order, check badge (exact, tolerant, pending, unread, failed) | in bundle | `TX/texture-published-citations.ts:69-105,135,307` | design: tap to expand (section 4) |
| Hover popover: source title, pinned quote, Open source | none | `TX/App.svelte:2551` | deliberately different: no hover; tap expands in place |
| Expanded "journal flow" side note (pretext) | none | `TX/texture-source-flow.ts:1-12,83-170,325` | design: the reader lane uses Exact's text flow (`wrap-flow`, `shape-outside` on the note box beside the paragraph's `text`) on a wide screen (an iPad), and the stacked card (section 4) on a phone column, as the web does below 620 px; unrun (`docs/reference/exact/capabilities.md` section 4) |
| Sources panel: numbered list, host, pinned blocks, model provenance | in bundle; private: `GET /api/texture/documents/{id}/models` | `TX/texture-published-citations.ts:326`; `TX/texture-model-provenance.ts:163,197` | design: "Sources" at the end of every document and from the page menu |
| Open source: Texture span, Web Lens, media, Source Viewer | `/api/content/import-url`, `/api/content/items` | `L/source-contract.ts:100-165` | design: the source panel; "Open original" goes to the system browser; a stored reader snapshot shows in the panel (signed in) |
| Texture-to-Texture transclusion `texture:<doc>@<rev>` with version pin and "newer version" | none | `L/texture-source-renderer.ts` (`renderInlineTextureRef`) | design: an inline chip; tap pushes that document at the pinned version |
| Embeds: image, file, video, audio, capsule_evidence (private only) | `GET /api/assets/sha256/<digest>` | `TX/texture-embeds.ts:1-60` | design: image inline; file and media open in Quick Look (a package view); capsule evidence as a card |
| Recent documents, New document | `GET`, `POST /api/texture/documents` | `TX/texture.js:41,73` | design (the Textures place); New document is a prompt, not an empty editor |
| Open a private document, its revisions, a revision | `GET /api/texture/documents/{id}`, `…/revisions`, `GET /api/texture/revisions/{id}` | `TX/texture.js:85,168,185` | design |
| Version history (no diff anywhere on the web) | revisions | `TX/texture-editor-state.ts:23,80-90` | design: the versions layer; a diff is not designed (the web has none) |
| Restore a version as latest | `POST …/{id}/restore` | `TX/texture.js:197` | design |
| Fork, "make it mine" (toolbar) and Reply via prompt bar | `POST /api/texture/documents` + revisions; `POST /api/texture/fork` | `TX/texture-publication-context.ts:27-59`; `L/conductor.js:101-125` | now (the desk stage forks on the first turn); design: the bar's aim "Reply to <title>" on a published story |
| Propose to the author | `POST /api/platform/publications/{id}/proposals` | `TX/texture.js:391` | design (actions menu) |
| Revise button, pending banner, cancel | `POST …/{id}/revise`, `POST /api/texture/lifecycle/{traj}/cancel` | `TX/texture.js:246,260`; `TX/App.svelte:517-546` | design: "Revising…" banner with Stop |
| Document stream (snapshot, synth_*, revision_created, head_changed) and lifecycle stream | `GET …/{id}/stream` (SSE), `GET /api/texture/lifecycle/{traj}/stream` | `TX/texture.js:298`; `TX/lifecycle.js:22,93` | now (desk stage polls `?format=json`); design: the reader follows head changes |
| Block editing, autosave, local drafts | revisions | `TX/texture-structured-editor-doc.ts` | out: excluded by the goal file; the phone changes a document only by prompt |
| Publish a version; publish as opinion; result card | `POST /api/platform/texture/publications`, `…/opinions` | `TX/texture.js:316,344` | design (actions menu, signed in) |
| Copy text, download md/txt/html/docx/pdf | `GET …/export?route=&format=` | `TX/App.svelte:1563,1574` | design: the system share sheet with the permalink, the text, or a PDF |
| Permalinks bare and pinned | | `L/autopaper.ts:90-110` | design: share uses `route.permalink` |
| Diagnosis panel (owner) | `GET …/{id}/diagnosis` | `TX/texture-source-diagnosis.ts` | out for now: an engineering view; reachable later from the actions menu |
| File-backed documents | `POST /api/texture/files/open` | `TX/texture.js:55,97` | design: how Files opens a text file in the reader |

### A3. Prompt bar (custodian: the conductor routes; Texture desk revises)

| Function | Route | Web source | Phone |
|---|---|---|---|
| Aim: "Edit <title>" (owned) or "Reply to <title>" (published), "v3 of v7" | none | `TX/texture-prompt-aim.ts:47-80` | design: the bar's aim line says the same |
| Classify, then submit with focus and base revision; status poll | `POST /api/prompt-bar/classify`, `POST /api/prompt-bar`, `GET /api/prompt-bar/submissions/{id}` | `packages/prompt-bar/PromptBar.surface.svelte:358-410,493`; `L/prompt-submit.ts:100-260` | now (desk stage); design: the bar, typed or spoken |
| Route chip, one-tap re-send under another class | | same | design: after a send, a chip "Edit / Comment / Ask" for 12 s |
| Ask the paper; add a source feed | `POST /api/autopaper/ask`; `POST /api/autopaper/sources/schedules` | `L/conductor.js:15-140` | design: the bar on Paper's front page (no focus) |
| Status ticker ("Texture created a new revision") | `/api/ws` | `PromptBar.surface.svelte:437-480` | design: the bar's states (section 1) carry this |

### A4. Files (custodian: ownerfiles)

| Function | Route | Web source | Phone |
|---|---|---|---|
| List root and folders | `GET /api/files[/{path}]` | `packages/files/App.svelte:120-122` | design (the Files place) |
| New folder, delete, rename or move, duplicate | `POST`, `DELETE`, `PATCH {move_to}`, `GET`+`PUT` | `:171-176,214-219,449-492,522-533` | design: swipe to delete, long-press menu |
| Upload, drag-drop | `PUT /api/files/{path}` | `:260-263` | design: from the Files app and Photos pickers |
| Download, share | `GET` | `:556` | design: share sheet |
| Open by type: text and markdown to the reader; pdf/epub to Reader; media to players; ics to Calendar | `GET /api/files/{path}?disposition=inline`; `POST /api/texture/files/open` | `L/open-with.ts:25-38,78`; `L/Desktop.svelte:1310-1355` | design: text and markdown open in the one reader; everything else in Quick Look (a package view) |
| Import docx or pdf into Texture | server converts | `packages/files/App.svelte:683-687` | design (long-press menu) |
| Set as desktop wallpaper | `PUT /api/preferences/pkg.settings.background` | `:621` | out: there is no desktop |

### A5. Mail (custodian: the Mail desk, through maild)

| Function | Route | Web source | Phone |
|---|---|---|---|
| Folders inbox, drafts, sent, quarantine; counts | `GET /api/email/messages?folder=` | `packages/mail/App.svelte:357-360,437,492` | design |
| Read a message (headers, HTML body in a locked iframe, attachments, trust chip); mark read or unread | `GET /api/email/messages/{id}`, `POST …/read`, `…/unread` | `:579-655,1237-1292` | design: plain text natively; original formatting on request (question 2) |
| Reply, compose, edit and delete drafts; aliases | `POST`, `PUT`, `DELETE /api/email/drafts`; `GET /api/email/aliases` | `:284,623,683,717,754` | design |
| Send a draft | `POST /api/email/drafts/{id}/send` | `:755` | server: refused for an API key, "owner session required" (`internal/proxy/email.go:30-35,76-95`) |
| Email me an approval link | `POST …/drafts/{id}/approval-email` | `:783` | design: the phone's interim way to send |
| Attachments: stage, from Files, remove, download (outbound only) | `POST`, `DELETE`, `GET /api/email/attachments` | `:888-1036` | design; inbound download is not offered on the web either |
| Daily Autopaper digest on or off | `GET`, `PUT`, `DELETE /api/email/digest` | `:827,846` | server: `PUT`/`DELETE` refused for a key (same file) |
| Add .ics to Calendar | `openApp('calendar', …)` | `:1310` | out: the handoff is dead on the web (no reader of `icsAttachmentId`); the phone can offer "Add to Calendar" through the system |

### A6. Settings, account, and the shell's functions (custodians: shell; auth service for keys and sessions)

| Function | Route | Web source | Phone |
|---|---|---|---|
| Sign in: passkey, email code, recovery | `/auth/register/*`, `/auth/login/*`, `/auth/email/*`, `/auth/recovery/*` (cookie) | `L/auth.js`; `L/AuthEntry.svelte` | now: the device flow (`/auth/device/begin`, `/token`), approved in the browser; recovery hands off to the browser |
| Device approval page | `/device` | `L/DeviceApproveRoute.svelte:79,110` | used as is, in the browser |
| Accounts: list, switch, sign out one or all, add | `/auth/accounts*`, `/auth/logout` (cookie, `choir_accounts`) | `packages/launcher/Launcher.surface.svelte:38-95`; `L/accounts.js` | design: one Keychain key per account; switch in Settings; server: a bearer "who am I" route to show which account a key is |
| API keys: list, create, revoke | `GET`, `POST`, `DELETE /auth/api-keys` | `packages/settings/App.svelte:230,265,290` | now (sign-out revokes its own key); design: list the account's devices |
| Theme: 7 presets, JSON editor | `GET`, `PUT /api/preferences/theme` | `L/preferences.js:4-31`; `L/theme.ts` | design: themes as data, listed and switched in Settings; light, dark, system; a Sepia reading tone (section 5, question 4) |
| Prompt bar top or bottom | theme layout | `packages/settings/App.svelte:410` | out: the phone's bar is always at the bottom, in reach |
| Package settings, "Who provides this?" | `/api/preferences/pkg.*`, `packages.selection` | `PackageSettings.svelte`; `WhoProvides.svelte` | design: read-only list later |
| Build and commit | `GET /health` | `packages/settings/App.svelte:361` | design: Settings, About |
| Restart to update | `GET /api/computers/{id}/update`, `…/lifecycle/status`, `POST …/lifecycle/update` | `L/UpdateChip.svelte:60,123,202` | design: Settings, Computer; a quiet banner when an update waits |
| Wake the computer; computer status | `POST /api/compute/recovery {wake_current_computer}`; `GET /api/compute/status` | `packages/compute-monitor/compute-monitor.js:4,15` | now (status, for discovery); design: the "computer asleep" state offers Wake |
| Changelog: events, mark seen, deep links into Texture | `GET /api/changelog`, `PUT /api/changelog/seen` | `packages/changelog/changelog.js:6-61` | design: the Activity place; server: push (APNs) for the closed app |
| Live events `/api/ws` | bearer accepted (`internal/proxy/handlers.go:1062`) | `L/Desktop.svelte:1034-1130` | design: open while foregrounded; the stream per document otherwise |
| Windows, overview, desktop icons, menus, last-resort launcher, wallpaper | `GET`, `PUT /api/desktop/state` | `L/Desktop.svelte`; `packages/overview`; `packages/desktop-icons` | out: ruling 6 |
| Library (package store) | `/api/catalog/*`, `/api/computers/{id}/packages/*` | `packages/library-next` | out: an installed package is a Svelte module a native app cannot run |

### A7. Other web surfaces

| Surface | What it does | Phone |
|---|---|---|
| Reader (PDF, EPUB; measure, tone, progress) | `packages/reader`; `GET`, `POST /api/media/progress` | design: PDF and EPUB pages (section 4); the reader's measure and Sepia tone feed section 5 |
| Content, Source Viewer, Web Lens | `packages/content`, `packages/browser`; `POST /api/content/import-url` | design: the source panel (section 4); "Open original" in the system browser |
| Podcast (search, subscribe, play, progress) | `packages/podcast`; `/api/podcast/*`, `/api/media/progress` | out as a separate surface: clips arrive on the radio from the run sheet |
| Audio, Video, Image, Slides, Calendar | `GET /api/files/…`, `/api/media/*` | design: pages of their own kind, opened from Files (section 4) |
| Pulse (public dashboard) | `GET /api/pulse/summary` (public) | out for now (question 7) |
| Compute Monitor | `GET /api/compute/status` | design: Settings, Computer (status, Wake, Restart to update) |
| Notifications (client toasts) | none server-side | design: the phone's own banners; server: push |


## Appendix B. Research: pretext, Exact's text model, typography, materials

### What pretext does on the web

- `@chenglou/pretext ^0.0.7` (`frontend/package.json:25`), with its
  `rich-inline` entry. It measures text and breaks lines from font strings
  without DOM reflow.
- Texture uses it for the expanded citation "journal flow"
  (`TX/texture-source-flow.ts:1-12,83-170`): the paragraph and up to six
  neighbours are laid out as absolutely positioned lines, narrower beside
  the note (`containerWidth − noteWidth − gap`) and full width below it,
  so the prose wraps around the note. Citation marks are `break:'never'`
  atoms. Below `SOURCE_FLOW_MIN_WIDTH = 620` px it uses `stacked` mode:
  the note goes under the lines (`TX/App.svelte:187-189`).
- The second use is `L/PretextInlineDisclosure.svelte` in sign-in only: it
  reserves the expanded height so nothing jumps.
- The web's citation popover is CSS `:hover`/`:focus`
  (`TX/App.svelte:2551-2553`); a pointer-down toggles the flow, which is
  the touch path today (`TX/App.svelte:1803-1830,1884`).

### What a transclusion is in the data

A citation is an inline `source_ref` atom (`attrs.id`,
`source_entity_id`, `display_mode` numbered_ref or expanded_ref;
`internal/texturedoc/schema.go:343-380`). A published bundle also carries
`transclusions[]` keyed by `anchor_id` (the `source_ref` id) with
`snapshot_text`, `relation_type: cites`, `default_display_mode:
collapsed_citation`, and `payload.resolved_blocks[]` with the source's
block text, URL and title [run: the resolve bundle for "Apple announces
Swift Student Challenge 2027…", 5 paragraphs, 19 `source_ref`s, fetched
2026-11-10]. So every mark already has its pinned quote and its source
block in hand: expanding needs no request.

### What Exact offers for the same job

| Need | Exact | Evidence |
|---|---|---|
| Mixed inline styling in one paragraph | a parent `text` with child `text` runs, each with colour, weight, style, family, `href`, `press`, `background-color` | `exact:apps/exact-live/app.contract:857-878`; `exact:host/apple/Sources/ExactKit/InlineText.swift:39-45` [src] |
| A tappable citation inside a sentence | a run with `press=` (or a Markdown `atom citation` pill with `atompress`) | CFA:3367-3370; CG:1462-1540 [doc] |
| Highlight a word or sentence (karaoke) | split the paragraph into runs; the current run gets `background-color` and `border-radius`, painted as `InlineBackground`; no `mark` tag, `::highlight` is textarea-only | `exact:host/apple/Sources/ExactKit/Text.swift:210-232` [src]; CG:1328-1360 [doc] |
| Where an inline run is on screen | `frame(id)` on Apple; `rangeFrame` only for atom pills | CG:1073,1079-1086 [doc] |
| Read-only Markdown | `text markup="markdown"` is the reader (the `textarea` is the editor); fixed heading scale 1.6/1.4/1.2/1.1/1.0; no per-element styling hooks; taps through atom pills and `atompress`; no documented footnote tap, so citations as atoms is the candidate mechanism [unrun] | CG:1381-1395, 1462-1483; `exact:markdown/src/pieces.rs:133-142` [src] |
| Expand in place | no `details`; state plus `if open` | CFA:1401-1403 [doc] |
| A sheet with detents | route `navigationPresentation="modal"`, `navigationDetent="medium large"`, `navigationInitialDetent`, `navigationGrabber` | CFA:2694-2750 [doc] |
| Arbitrary-content popover on iOS | none found (macOS only); menus become `UIMenu` | CFA:1581-1584,1689 [doc] |
| Text-flow around an obstacle (pretext's side note) | yes: `wrap-flow="both"` and `shape-outside` (`circle()`, or a `clip-path` outline) with `shape-margin` on the obstacle box, in the same container as the `text`; an earlier version of this table said none was found, which was wrong | `exact:apps/textflow/app.contract:102,113`; `exact:README.md:597,645` [src]; unrun by us; supported rectangular shapes unverified |
| Long documents | `list virtualized` with keyed rows; `scrollIntoView(list, key, block="center")` | CFA:1452-1510; CG:2711 [doc] |
| Selection | `user-select="text"`, one paragraph at a time on iOS | CFA:4599-4607 [doc] |
| A bar floating over the pages | no `bottomAccessory`; an authored overlay after the panels paints over routes and bars; or the tabs hatch hands a package the `UITabBarController` | CFA:2604,2692-2693 [doc]; `exact:host/apple/Sources/ExactKit/IOS/NavigationTabsIOS.swift:99` [src] |

### Exact's typography controls on iOS

| Control | Status | Evidence |
|---|---|---|
| System faces | `system-ui`/`sans-serif` = SF Pro; `ui-serif`/`serif` = New York; `ui-rounded` = SF Pro Rounded; `ui-monospace` = SF Mono. A quoted "New York" is not resolved: use `ui-serif` | `exact:host/apple/Sources/ExactCore/Text.swift:679,798-810` [src] |
| Text styles that follow Dynamic Type | `-exact-large-title`, `-title1..3`, `-headline`, `-body`, `-callout`, `-subheadline`, `-footnote`, `-caption1..2` | CFA:4662-4701 [doc] |
| Sizes that follow Dynamic Type | `rem` (1rem is Dynamic Type's body on iOS); `min(-exact-subheadline, 31px)` caps growth; px does not scale | CFA:3244-3256,4696-4701 [doc] |
| Weight, italic, optical size, variation axes | numeric weights; real italics; `font-optical-sizing`, `font-variation-settings` | `exact:host/apple/Sources/ExactCore/Text.swift:790-826`; `exact:host/apple/Sources/ExactCore/FontAxes.swift:1-45` [src] |
| Numerals | `tabular-nums` only; `oldstyle-nums`, `lining-nums`, fractions refused | `exact:contract/lower/src/values.rs:560-578` [src] |
| Line height, letter spacing, transform | supported | CFA:5010-5012 [doc] |
| Hyphenation | `hyphens="auto"` by document language | CFA:4378-4381 [doc] |
| `text-wrap` | `balance` (up to 6 lines); `pretty` refused | CFA:4383-4388 [doc]; `values.rs:622-624` [src] |
| Widows, orphans | supported, default 2 | CFA:4429 [doc] |
| `text-indent`, justify, `line-clamp` | px/rem/em indent; justify; clamp | CFA:4375-4377,1533 [doc] |
| `hanging-punctuation`, `font-feature-settings` | absent | grep found nothing [src] |
| Underline colour and thickness | refused by name | CFA:5072-5074 [doc] |
| `ch` | a fixed 8 px cell, not glyph width | `exact:kernel/src/style/cells.rs:1-30` [src] |

### What in Exact blocks parity

Nothing blocks a function. These shape the design:

1. No hover and no iOS content popover: sheets and in-place expansion
   (designed above).
2. Text-flow around an obstacle exists (`wrap-flow`, `shape-outside`;
   corrected 2026-11-11), so the reader lane may flow prose around a side
   note or figure instead of stacking, where the column is wide. The stacked
   card stays the phone-width form, as on the web [unrun].
3. Sub-sentence highlight re-lays out the paragraph on every change; at a
   sentence a few seconds the cost is unmeasured [unrun]. Floor:
   paragraph highlight, which changes only a background.
4. No web view for our screens. A mail body is third-party HTML: the
   native view shows its text; "original formatting" would need a package
   `WKWebView` with scripts and remote loads off (question 2).
5. No PDF or media view in Contract: Quick Look or PDFKit in a package
   view (`exact:docs/reference/exact/ui-model.md` route 3; one native view
   per hatched node, gaps.md #7).
6. Old-style numerals, hanging punctuation, `text-wrap: pretty` and
   underline thickness are not available; the type system avoids them.
7. No persistent layer above a UIKit sheet: the bar's layers (launcher,
   pile, player, source panel) are authored overlays, and UIKit sheets are
   kept for tasks that take over.

These go into `docs/reference/EXACT.md` or its subpages when the build starts.


### Materials, depth and theming in Exact

| Need | Exact | Evidence | Verdict |
|---|---|---|---|
| Glass or blur behind a container | `backgroundMaterial="glass" \| "ultra-thin" …`; `backdrop-filter` maps to the nearest system blur | `exact:host/apple/Sources/ExactKit/Backdrop.swift:19-28,226,241-251` [src] | native |
| Text over a blur | vibrancy view | `exact:host/apple/Sources/ExactKit/IOS/VibrancyIOS.swift:1-17` [src] | native |
| Shadows | multi-layer, outer and inset, with spread | `exact:host/apple/Sources/ExactKit/BoxShadow.swift:1-10` [src] | native |
| Transforms | `translate`, `rotate`, `scale`, plus 3D `rotate` axis, `translate` z, parent `perspective`, `backface-visibility`; no `preserve-3d`, `matrix`, `skew` | `exact:scripts/fixtures/space.contract:15-33` [src]; CFA:4163 lists 2D only | native; solids need a package view |
| Pressed state | `:active` with colour, opacity, `-exact-press-scale`; not shadow or border | CFA:4492-4503 [doc] | native |
| Scroll-linked effects | `scroll=` events; bounded timeline extensions | CG:1745,1802-1810; `exact:docs/contract-for-humans.md:1510-1512` [doc] | partial |
| A layer above a UIKit sheet | not found | inferred from CFA:2698 | not possible: keep the bar's layers authored |
| Theme tokens | `app.tamber` tokens and declared axes, switched at runtime with `setMode` and `setScheme` | CFA:4722-4734; CG:2701 [doc] | native for declared themes |
| A theme's values from data | a bound `color=` string from data reads like a token name or colour; CSS custom properties cannot be set from state; `var()` refused in lengths | CFA:4718, CG:1530-1534, CFA:3166,4271 [doc] | binding proven upstream; our bindings unrun on a device |
| `mix-blend-mode` | a row exists but the iOS SVG path logs "not supported on iOS" | `exact:host/apple/Sources/ExactKit/SvgIsland.swift:283` [src] | not possible on iOS boxes |

## Appendix C. Real data, fixtures and server gaps

- Real, public, no key: the front list and the resolve bundles (body,
  citations, transclusions, resolved source blocks, nav), related stories,
  expansion state [run].
- Real, signed in (kit stages exist and ran on staging): device sign-in,
  computer discovery, System 1, the fork and prompt bar, the document
  stream.
- Signed in, not yet called by the kit: documents, revisions, restore,
  files, mail, changelog, compute recovery, update routes. Until a key is
  available to the builder these pages run on the kit's URLProtocol stubs
  through a "demo listener" fixture mode.
- Not on any route: a per-account edition document, block positions in
  prompts, a revision's request id, clip time spans, story entities on the
  front list, a theme served from a Texture, push.

Server gaps, in the order the lanes need them (each its own small change on
the server lane):

1. `entities: [{id, label}]` (central) on front-list stories and in the
   bundle (`internal/platform`; the edges exist,
   `internal/platform/publication_related_entities.go:1-12`).
2. A bearer "who am I" route (account id, email domain hidden, computers);
   `GET /auth/session` is cookie-only.
3. `GET /api/texture/theme?doc=` returning a validated version 3 theme from
   a Texture, as `/api/texture/style` does for the Style Guide.
4. An edition marker: a document role the server recognises for "My
   edition".
5. Block-addressed prompts (`focus.selection` or `block_id` for the Texture
   desk; API map §8 #3) and `request_id` on version events.
6. Lifecycle progress detail: the step and the sources being read on
   `synth_progress`, if not already there.
7. Mail send and digest from a key: an owner-act path for a device key
   (`internal/proxy/email.go:30-95`; red, auth).
8. A ranked front list, so the web and the phone agree (later).
9. Push (APNs registration and fan-out) for the closed app (later).
