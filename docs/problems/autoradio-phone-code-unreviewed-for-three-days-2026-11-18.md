# Autoradio: three days of phone code reached the owner's phone without a review seat reading it (2026-11-18)

Status: open. The defects found are repaired in lanes and in the build on the
phone; the cause (review is tied to landing, and the phone lanes have not
landed) is not fixed. Class when fixed: green (a rule) and a merge of the
phone lanes into main.

## Problem

About 150 commits of phone work (the run sheet player, mail, files, podcasts,
onboarding, takes, the notebook page) were built in lanes, merged into
candidate branches, checked by the package suite and simulator tours, and
installed on the owner's phone four times. No review seat read any of it. The
contract ties review to a landing, and nothing landed: the candidates were
installed from lanes. So the phone had less review than a one-line server
change.

## Evidence

On 2026-11-18 the lead ran three review seats over the installed candidate
(cf229bf49) for the first time. All three returned REPAIR. Defects a suite and
a tour had not shown:

- **Words that must stay on the phone were posted.** A message or an imported
  file read aloud put its title, and the rest of a passage cut by an answer,
  into the run sheet ops posted to the listener's computer. The standing rule
  is that mail passages and local-only files never go to the server desk.
- **Sign-out wiped too little.** The last mail read aloud, the listening
  signals, the conversation, kept turns, the reader's open pages, the voice
  profile, imported books and the Files cache survived a sign-out; work in
  flight at sign-out could write the old account's data after the wipe.
- **The first-run microphone check could never pass** (it waited on a signal
  nothing produced).
- **Backgrounding with voice on could stop a podcast** (the player read its own
  hold as a system pause).
- **A clip could not be replayed, and "previous" during a clip's lead-in
  seeked the remote audio.**
- **Transcripts of what the listener said were written to the log.**
- A feed's episode notes with markup came out blank; an upload with nothing
  chosen invented a file; a second notebook could be created.

Repairs: lanes fix-player, fix-content and fix-account, each reviewed again
(seat 1 ACCEPT on the last round; seat 2's findings on the first round
answered in a further round), merged as lane/autoradio-integ17 and integ18.

## Belief

Two causes. Review is triggered by a landing, and the phone work was never
landed because the landing script cherry-picks single commits and the phone
lanes are merges of merges. And the lead treated a passing simulator tour as
the acceptance of a candidate, when a tour shows that pages draw, not that
private words stay on the phone or that a sign-out is complete.

## What should change

- **A build that goes on the owner's phone is reviewed like a landing**, at
  the tier of its most sensitive change, before the install. Mail, sign-out
  and anything that posts to the server are red.
- **The phone lanes land on main** (one merge, as with PR #110), so the next
  change is a small diff against main and review is cheap.
- **Privacy rules get tests on the posted bytes**, not on the screen: the
  posted ops for a message, a book and a file are asserted to carry none of
  their words. The first of these now exists (PlayerRepairFixTests).

## Remaining error

- Not reviewed at all: the Exact module files outside the three repair areas,
  the Tetramark motion code, the podcast worker.
- Known and open: a superseded story start can still take the audio when the
  engine, not the player, is slow; a system pause that iOS does not report
  while the player is held is not seen; a Files audio entry cut by an answer
  loses its place; the embedding store rewrites its whole file.
- Nothing here has been checked on the device by a person. Sound, Bluetooth,
  a real phone call and a real sign-out are the owner's to try.
