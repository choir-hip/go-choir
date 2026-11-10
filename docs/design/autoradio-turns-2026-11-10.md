# Autoradio: what happens after the listener speaks (2026-11-10)

Status: design, not built. Mutation class of this document: green.
Rollback: revert. Companion to
[autoradio-voice-modes-2026-11-10.md](autoradio-voice-modes-2026-11-10.md),
which covers how a turn is heard. This note covers what the radio does with
it. It feeds lane D (the Autoradio desk) of the
[goal](../definitions/choir-autoradio-pass1-2026-11-10.md).

## The owner's statement (2026-11-10)

- A listener's turn "demands first an immediate acknowledgement, whether
  it's short or long."
- Then there is "a decision point about where things go next. And I would
  say in general, it's not resuming the podcast."
- Where it goes: another topic; different material on the same topic; a
  summary or explanation in place of news and podcast content; the
  listener's private data on their computer; something else.
- One pattern: "talk back to a podcast and record my thoughts. And then
  only after many rounds of that, perhaps when the podcast is over, do I
  want to evaluate my thoughts in totality."
- More often: "an AI response given the context of the material and maybe
  more web research and my input. And even better is then playing some
  other human content if such relevant content exists."
- Ahead: listeners publish what they say as takes, and those takes are
  read by agents and played to other listeners.
- "We do have a challenge of getting the latency down on those pivots."

## The turn

1. **Acknowledge, at once.** A spoken line within about a second of the
   turn ending. It says what was understood and what happens next ("Other
   views on the ceasefire talks, one moment"). It is produced on the phone
   or by the fastest model available, from the turn's text and the current
   item, and does not wait for any search.
2. **Decide.** The desk picks a route. This is the decision point.
3. **Carry out the route.** If it takes time, the radio fills it
   deliberately (below), never with silence.

## Routes

| Route | When | What plays next |
| --- | --- | --- |
| Respond | a question or a view on the current item | a spoken answer from the item's text, the listener's words and, where needed, research; then a human clip that bears on it if one exists |
| Redirect | "something else on this", "what about X" | other human material on the topic or the new topic, with a spoken lead-in |
| Explain | "what does that mean", "summarise" | the radio speaks; no clip |
| Private | asks about the listener's own documents, mail or computer | a spoken answer from the computer's own desks; never published, never shared |
| Note | the listener is thinking aloud | a one-word acknowledgement and the programme continues; the note is kept (below) |
| Command | pause, skip, back, louder | the act, with at most a tone |
| Resume | the turn needs nothing further | the programme, from a few seconds before the dip |

Resume is one route among seven, not the default.

**Telling Note from Respond.** This is the hard classification. The
signals are: no question and no request in the turn; the listener set a
"just listening" mode; the listener's own history at this decision point.
A wrong guess is cheap in one direction only: treating a note as a request
interrupts the programme, while treating a request as a note leaves the
listener unanswered. So an unsure turn gets a short spoken offer ("Noted.
Want a response?") and the programme continues unless they say yes.

## Notes become a document

Each talk-back is appended to one Texture document for the session, with
the moment of the programme it answers (the same reference a citation
uses: capture, block, quote, time span). At the end of the episode or the
session, the radio offers to go through the notes together. That is the
"evaluate my thoughts in totality" pattern, and it needs no new store:
Texture is the custodian of document revisions.

## Takes

A note or a response the listener chooses to publish is a take: their
recorded voice, its transcript, and the reference to what it answers. It
is published through the existing publish gate into the Autopaper object
graph. From there another listener's desk can find it the way it finds a
podcast passage, and play it, attributed. Nothing is published without the
listener's act. Private-route turns can never become takes.

This is pass 2 or later. Pass 1 keeps the reference on every note so that
takes need no migration.

## Latency

The acknowledgement hides the first second. The pivot itself has to be
fast, and a live search is not: one podcast search through Exa took 64
seconds and another 3.4 minutes on 2026-11-10.

- **Prepare the pivots when the edition is built.** For each item in the
  run sheet, the desk finds in advance: other human passages on the same
  story, passages with an opposing view, a short explanation, and the next
  related item. A redirect to any of these is then immediate.
- **Answer from what is in hand first.** A response starts from the item's
  text and its prepared neighbours, spoken as soon as the first sentence
  is ready. Research, if needed, arrives as a second part ("I looked
  further: ...").
- **Fill honestly.** When something does take time, the radio says so and
  continues the programme or plays a prepared neighbour, and brings the
  result in as its own segment when it is ready.

Targets, to measure rather than assume: acknowledgement begins within one
second of the turn's end; a prepared pivot plays within two; an unprepared
one is announced and arrives when ready.

## Learning

Each decision is a precommitment record: the route chosen and the
probability that it was what the listener wanted. What the listener does
next settles it. Listening through, a follow-up on the same line, or
"thanks" resolves it one way; an immediate correction ("no, I meant"), a
skip, or silence followed by a command resolves it the other. The offer in
"Telling Note from Respond" is itself a scored prediction. The desk reads
its own scored record for this listener when it decides the next turn.

## Open

- What produces the acknowledgement within a second: a template filled on
  the phone, a small on-device model, or the fastest hosted model. The
  spike should time each.
- How much is prepared per item before the cost outweighs the use. The
  learning record should show which prepared pivots are ever taken.
- Whether the listener's recorded voice is kept for notes or only the
  transcript, before takes exist. Keeping audio has a storage and privacy
  cost; discarding it means notes cannot later become takes.
