# Autoradio: four lists decide what plays next, and there is no Autoradio desk

Date: 2026-11-16. Status: open; understanding first, no fix chosen.

## Problem

The owner used the phone app and reported: "there's actually no Autoradio.
You can play individual stories, but I haven't really experienced the
Autoradio desk." And: "there's two concepts of queue, actually three."

The overnight report of 2026-11-16 said "there is one play queue". That claim
was wrong for what plays next. It holds only for the kinds of item the explicit
queue accepts.

## Evidence (read from code on `lane/autoradio-qa` 3f11437d3 and main; not run)

- "N of M" and the segmented bar count the paragraphs of the one story
  playing, or segments when the document is a radio stretch
  (`BarState.swift`).
- Skip to the next story checks, in order: a parked clip, the explicit queue
  (`PlayQueue`), the next stretch segment, then the paper's story list as it
  stood when the story started (`ChoirApp.moveStory`).
- "Up next" is `PlayQueue`: filled by the listener and by the host's queue
  edits.
- "N in the deck" is the five fixed pages the listener swipes between. It is
  not a playback list.
- The app asks for a DJ stretch after sign-in and on Play. With no first
  segment inside a deadline (8 s at launch), or an empty or failed stretch, it
  falls back silently to one front-page story. The one proven stretch took
  47.7 s to write on the drill computer.
- The ask field posts to `/api/autoradio/turns`: one model call with no tools,
  given the page text, the queue, up to 40 story titles and 8 lines of
  history. It can speak and edit the queue. It cannot see Textures, mail,
  podcasts, the edition or the stretch.
- `autoradio` is registered in `internal/appdesk/required.json` by name, with
  no cell and no owned object kind. The run sheet, write-ahead, air log and
  hand-offs in `docs/design/autoradio-dj-programme-2026-11-11.md` are unbuilt.
- "Host" names two unrelated routes and prompts (turns, programme). "Queue",
  "deck", "programme" and "station" each name two or three things.

## Belief

- The fallback fires on most launches, so the listener hears single stories
  (likely; which branch fired on the owner's phone was not read from a trace).
- The missing thing is one run sheet owned by one desk, not a repair to any
  of the four lists.

## The owner's direction, 2026-11-16 (their words)

- "with Autoradio, we're even going to have the Autoradio desk as another desk
  in front of texture, like an interface to the interface, because you need to
  have a different kind of interface for voice. And really, Autoradio, the AI
  DJ, is meant to create a program composed of textures with different
  sources, you know, podcasts or clips as sources, and the Autoradio desk is
  going to be writing for voice to be spoken aloud." Texture pieces are "meant
  as one at a time, and then Autoradio is above all of them. It can also
  include content from email."
- "real deal is when I can really talk to it. When things that I say go
  straight to the Autoradio desk, which has a REPL, which controls the data
  structures which drive the app in terms of the deck and the queue, which now
  I think need to be different."
- On deck and queue: "what happens when you open up the mail or reader app
  while you're listening to something, but that shouldn't change what's really
  played. So maybe the card stack is not the queue."
- The desk scripts transitions and opens with an introduction: "before it just
  starts reading one story after the other, it should give like an
  introduction to the top... by signposting, by introducing, then it makes it
  more easy for me to interact with it as a user with my voice." The format is
  not fixed.
- On-device models sit in front of the desk: "just like the Autoradio desk is
  in front of texture, we're going to have the local language models in front
  of the Autoradio desk. So when you give a voice prompt, it should be
  responded to by an on-device model, which has context of your card stack and
  your queue and what was previously played as well, so that they can respond
  while the Autoradio desk is working, and then Autoradio desk can take over
  seamlessly."
- Writing ahead, and one master stream: "I absolutely do want to be writing
  ahead... We should have a universal stream. We'll make that public, in
  fact. We should push it out to YouTube or something. Like, we should
  actually just have a 24/7 radio station, which is like the default stream.
  And so if you were to download the app and then skip through the onboarding,
  that would just be what plays. And beyond that just being default, that's
  also an important thing to have and improve on as a benchmark, because...
  with Autopaper, we are writing new stories and updating existing stories
  pretty frequently, you know, every minute. And so the radio stream should
  similarly be incorporating new stories as breaking news occurs, and
  simultaneously providing updates to ongoing stories, and have the right
  balance between reiterating the top stories and mixing in novel and
  long-tail stories... obviously that's going to be personalized for every
  user, but we should have a single master stream. And then the
  personalizations would kind of be diffs from that, rather than entirely
  developed for every individual user... as we're scaling up to like
  thousands of users, we're probably going to maybe scale up to like dozens of
  master radio streams with minor personalizations on top of it."
- Voice: "it's off unless you turn it on. And then when you turn it on,
  hands-free voice stays on until you stop talking plus a couple minutes."
  And: "I don't want press and hold talk."
- Screen: "for reader and for the embedded web view, the content should get
  the full screen except for the prompt bar area at the bottom."
- Type: "use typography to signify... the difference between static text and
  a button and a headline. Typography is too same same, sans serif."
- Background: "Podcasts and everything else should play in the background,
  even if you're not focused on the Autoradio app. Currently, they don't."
  After a return to the foreground "you can't resume playing".
- Going to the next thing and "I like this or I don't like this" are separate
  questions; the gesture semantics are undecided.

## Remaining error

- Not designed: the run sheet, how listener entries and desk entries order,
  what a swipe on the now-playing card does, how liking is signalled.
- Not measured: time to first spoken word on the owner's phone; how often the
  fallback fires.
- Cost of writing ahead in the background is the owner's to set.
