# Autoradio Prototype

Date: 2026-10-14. Status: a prototype on a branch, not pushed and not
deployed. It is for trying the experience, and it fixes no architecture.
Mutation class: orange (a new frontend app and route, with no backend
change). Rollback: drop the branch, or revert its commits.

## Why now, and why so small

The owner describes Autoradio as "somewhere between an asynchronous
Clubhouse / intellectual salon" and, for organizations, as asynchronous
meetings that drive the multi-agent system. It would mix Autopaper news,
podcasts and the listener's own work, and the listener could interrupt at any
point, record a take, and publish it or share it with a team
(`docs/archive/reports/choir-state-and-vision-2026-10-14.md` §2.11). The voice
principle (`docs/archive/reports/pricing-and-economics-notes-2026-10-14.md`): "I want
my AI to be clearly sounding like AI, and then we're going to be mixing in
human voices — that contrast is so much better." The owner has also ruled
that foundations come first and that a prototype is welcome "if its framed as
a demo/prototype, not setting architecture that will be annoying to refactor"
(owner-roadmap-rulings, ruling 5). So this prototype runs entirely in the
browser. It adds no object kinds, no endpoints, no services and nothing paid.

## What it does

- **A programme from today's front page.** The prototype reads the public
  front list (`GET /api/platform/publications/front`, the route Autopaper
  uses) and ranks it with `rankFrontStories`, the same rank the front page
  uses. It then reads the top eight stories as one programme: a short intro,
  then for each story its headline, its dek and what is new, then a closing
  line. "What's new" says only what the list carries (the version, when the
  story was last updated, how many sources it cites) and the version this
  device last heard: "New since you last listened: this is version 3; you
  heard version 2." It never invents a change note.
- **A clearly synthetic AI voice.** The prototype uses the browser's speech
  synthesis (Web Speech API) and picks the most machine-sounding English
  voice on the device: Fred, Ralph or Albert on macOS, eSpeak on Linux, the
  Microsoft desktop voices on Windows. It avoids lifelike voices (Siri,
  Enhanced, Premium, Natural, Google network voices), and reads at a slightly
  lowered pitch. While the AI speaks, an "AI voice" badge (blue, monospace)
  is visible, and every rundown row is tagged AI or Human. The listener can
  pick another voice and remove the human voices.
- **Human voices mixed in.** Five curated publisher feeds are mapped to
  Autopaper sections, all readable from a browser because they send CORS
  headers: BBC Global News Podcast, BBC Business Daily, The Daily, Hard Fork
  and The Vergecast. The source registry holds no podcast feeds, so the list
  lives in `autoradio.ts`. After each story the prototype looks for a related
  episode: one that shares at least two distinctive words with the story.
  If there is none, it uses the newest episode from a feed for the story's
  section, once per section. If there is no match, it skips. Each programme
  has at most four clips. The AI voice introduces each clip with its
  attribution ("From Business Daily, by BBC World Service: ..."). The clip
  then plays the episode's first 30, 60 or 120 seconds from the
  **publisher's own enclosure URL** in an `<audio>` element, upgraded to
  https and never proxied or re-hosted
  (`docs/research/autopaper-media-and-rights-2026-10-14/README.md` §4). A
  "Full episode" link goes to the publisher.
- **Interrupt and record.** Talk pauses the programme and records the
  listener's take with MediaRecorder. Hold to talk, or tap to start and tap
  to stop; Enter or Space works from the keyboard. Each take stays in the
  page as a `blob:` URL that can be played back. It carries the segment it
  interrupts, and the take offers four actions:
  - Save downloads the take.
  - Share with team shows what would be sent. Nothing is sent.
  - Publish shows what would be sent. Nothing is sent.
  - Discard removes it.

  The two preview actions show the audience, what the take is about (the
  publication id and version), its length, its media type, and
  `transcript: null, sent: false`. Resume the programme then picks up at the
  sentence that was interrupted.
- **Remembered on this device.** The current segment and sentence, the
  voice, the speed, the clip length, whether human voices are on, and the
  version heard of each story. Every storage access goes through try/catch,
  so blocked storage only means nothing is remembered.
- **Two surfaces, one component.** Autoradio opens from the desk sheet as a
  desktop window (there is deliberately no desktop icon, because it is a
  prototype). It is also a standalone public page at `/autoradio`, which
  reads like `/autopaper`, signed in or not. The layout follows its own width
  (a container query): one column on a phone, and the rundown beside the
  player when wide. At 375px there is no horizontal scroll and every key is
  at least 44px tall. Media Session handlers give lock-screen play, pause and
  skip where the browser supports them.

Files: `frontend/src/lib/autoradio.ts` (pure: feeds, matching, programme,
voice choice, preferences, take preview), `frontend/src/lib/AutoradioApp.svelte`,
one registry entry in `frontend/src/lib/apps/registry.ts`, the `/autoradio`
branch in `frontend/src/App.svelte`, and `frontend/tests/autoradio.spec.js`.

## Evidence

`frontend/tests/autoradio.spec.js` (9 tests) runs against `vite preview`. It
mocks the front list and the feeds at their real URLs, and stubs speech
synthesis, `<audio>`, getUserMedia and MediaRecorder:

- **The programme (pure).** It checks rank order, a related clip placed after
  its story, one section clip, no clip when a section's only feed fails, and
  the https publisher URL. It also checks "what's new" against heard
  versions, Fred chosen over Samantha and Google US English, bad storage
  tolerated, and a take preview with `sent: false`.
- **The page.**
  - Play speaks the intro in Fred at a lowered pitch.
  - The human-voice badge, the attribution and the publisher URL appear
    during a clip.
  - Skip, pause and the rundown jump all work.
  - Position and heard versions survive a reload.
- **Recording.** Tap, hold and keyboard all record. A take plays back from a
  blob URL, and Save is a download. Share with team and Publish show the
  preview and make no non-GET request. Resume continues at the interrupted
  sentence. A refused microphone records nothing and says so.
- **Layout.** At 375px there is no overflow. The desktop window opens, and
  the recording state stays visible under the London Salmon theme. That
  theme restyles every button inside a window, so the keys draw their
  borders and fills on wrappers.

A manual smoke run against the **live** feeds (only the front list was
mocked) found all five answering a real Chromium page, and the programme
built with four clips.

## What it deliberately leaves out

- **Server-side or model TTS.** The voice is whatever the device ships. The
  next step is a small open TTS model, run on the device or as a
  negligible-cost server call, chosen to sound like a machine (pricing notes,
  "Voice").
- **Transcription.** Takes are not transcribed, and podcast audio is not
  transcribed. Transcripts as captures with timed blocks and
  `timestamp_range` citations belong to the media and rights plan (step 9
  there). They are not a client feature.
- **Agent routing of takes.** A take does not become a capture, a Texture
  draft, a team object or a precommitment record to an app desk. The
  previews only show what would travel. That flow (§2.11 of the state and
  vision report: speech to capture, to Texture, to publish or share, to
  management, to an app agent) depends on contributions and accounts (LC,
  LA), verb registries (LP) and the settlement loop (L3).
- **Rights.** The prototype plays only the publisher's audio from the
  publisher's host, with attribution and a link out, and it quotes nothing.
  It does not decide whether to clip mid-episode at the quoted minute,
  whether the start of an episode is the right excerpt, or how a published
  audio op-ed is licensed. Those questions stay open in the media and rights
  research (open question 4).
- **Durable state.** Takes live in the page and are gone on reload.
  Preferences live in localStorage. Nothing is written to the tape, and the
  reserved `choir.autoradio_run_sheet` kind is unused.
- **Private work and mail.** The mix is public news and public podcasts
  only. Mixing in the listener's own work needs the privacy rules first
  (state and vision report, risks: "aggregate box scores must never be
  computed over private data").
- **Version-pinned narration.** The L5 acceptance asks that "an audio artifact
  names the exact version it narrates, and a later correction produces a new
  edition citing the change". The programme knows each story's version and
  shows it in the take preview, but it produces no audio artifact.

## What we learned

- **Lexical matching is crude.** Against live feeds, an AI data-centre story
  matched a Business Daily episode on AI regulation, on words alone. A
  better match needs an agent or embedding judgement, or the feeds as
  sources in the source registry with their episodes captured and linked to
  stories.
- **The contrast works on screen.** Blue monospace "AI" against warm
  "Human" tags in the rundown makes the alternation of voices legible at a
  glance, before any audio plays.
- **The desktop theme matters.** The theme strips button chrome inside
  windows, so state that must stay visible (recording) cannot live on a
  `<button>`.
- **Browser speech is fragile.** Speaking one sentence at a time gives
  pause, resume, progress and the interrupt point for free, and avoids long
  utterances being cut off. A real TTS service should keep that unit.

## Next iterations

1. Listen to it on a phone (`/autoradio`) and tune the script: the length of
   the intro, story transitions, how a clip is introduced.
2. Make podcast feeds first-class sources (registry entries, with enclosure
   metadata kept, as step 3 of the media plan), so matching uses captured
   episodes rather than the browser's fetch.
3. Choose a small open TTS voice for consistency across devices, still
   clearly synthetic.
4. Turn a take into a capture on the listener's computer, transcribe it, and
   draft a Texture team note or audio op-ed. This needs the foundations
   above, and it is the first point at which Autoradio writes anything.
5. Use asynchronous meetings as a programme built from a team's objects
   instead of the front page, with takes as the replies.
