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
- Where it may go: another topic; different material on the same topic; a
  summary or explanation; the listener's private data on their computer;
  recording the listener's thoughts to weigh later; "something else
  entirely".
- Best of all: "an AI response given the context of the material and maybe
  more web research and my input. And even better is then playing some
  other human content if such relevant content exists."
- Ahead: listeners publish what they say as takes, and those takes are
  read by agents and played to other listeners.
- "We do have a challenge of getting the latency down on those pivots."
- On a first draft of this note, which listed seven routes and a "note"
  category: "I want to be careful not to prematurely encode a schema ...
  we're looking for the abstraction that makes this structure
  unnecessary."

## The abstraction

**The radio is a Texture document played aloud, and speaking to it is
prompting that document.**

- The edition a listener hears is a document: their own fork of the
  edition, in order. Its blocks are what the radio says and the clips it
  plays. Playing is rendering that document as audio, the way the web
  renders it as a page.
- A listener's turn is owner input to that document at the place being
  played. It is the prompt bar, by voice. This is already the rule in the
  goal file (one input route).
- The desk answers the way it answers any prompt on a document: it
  revises the document from that place onward. The revision is whatever
  the desk judges right: new spoken text, a clip, different material, the
  rest left as it was.
- The player plays the document. After a turn it plays what now follows.

Nothing else is needed. In particular:

- **No routes.** "Respond", "redirect", "explain" and "resume" are
  descriptions of revisions someone might observe, not branches in code.
  The decision point is the revision. "Resume" is a revision that changes
  nothing after the acknowledgement.
- **No note category.** Every turn is kept in the document where it was
  said, with the moment of the programme it answers, whether or not the
  desk does anything more with it. Talking back for an hour and weighing
  it afterwards is a document with many of the listener's passages in it,
  and a later prompt over the whole.
- **No privacy route.** A revision drawn from the listener's private
  material carries that material's labels, as any derived record does, and
  the publish gate already refuses it.
- **No new store.** Texture is the custodian of document revisions. The
  reserved run sheet kind may prove unnecessary; that is a question for
  step 0.1 (the page format), not a decision here.

What a revision must contain is decided by the page format, not by this
note: a block that is speech, a block that is a clip with its source
reference and time span, and the listener's own passages.

## Two fixed behaviours

Only these are structure, because the experience fails without them.

1. **Acknowledge at once.** A spoken line begins within about a second of
   the turn ending and does not wait for the revision. It is the first
   thing the desk writes, or a line produced on the phone when the desk is
   slower than that.
2. **Never silence.** While a revision is being made, the radio says so
   and plays on, and the result comes in when ready.

## Early structure lives in questions, not in types

Owner, 2026-11-10: "some structure may be essential, especially early on,
before we can have more of an end-to-end learning approach ... One of the
key tools ... for when we need a low latency decision ... is using Jev,
the System 1 model. So if we can provide ... the surrounding context of
the situation and the user input and some kind of prompt, we can then
pass that decision to the System 1 model."

So a turn has two speeds.

- **System 1, at once: Jev.** When the turn ends, the phone sends its text
  with the surrounding context (the passage being played, the prepared
  material for it, the last few turns) and a short set of questions. Jev's
  answers choose the acknowledgement and the first thing to play, from
  material already in hand. Measured on the paper's traffic on
  2026-11-10: 110 to 260 ms mean upstream for a call of 1 to 12
  questions, with a tail to a few seconds. The gateway already has an
  interactive class for Jev calls.
- **System 2, when ready: the desk.** The desk's revision of the document
  follows. It may extend what Jev started or replace it.

The structure this needs early (does the turn ask for something; is the
answer in the prepared material, and which piece; should the programme
carry on meanwhile) is held as **questions**: text the desk's package owns
and can revise, not routes or categories in code or in the page format.
The questions shown here are examples, not a list to implement.

Each Jev judgment already leaves a record
(`internal/jevrecord`). Joined to what the listener did next, those
records say which questions earn their place and where their thresholds
belong, and they are the training signal for whatever replaces the
questions later. A question that never changes what plays is deleted.

Only the turn's text and context go to Jev. No audio leaves the phone.
When the phone is offline or Jev is slow, the acknowledgement is produced
on the phone (next section).

## A model on the phone

Owner, 2026-11-10: "We actually may be able to run a decision model
locally on our device ... we may want to be using that specifically for
the immediate acknowledgments, or the immediate acknowledgement may need
to last like up to a minute or two before we get the response from the
desk."

So the acknowledgement is not always one line. When the desk needs a
minute or two, the phone holds a short conversation: it reflects back what
was said, asks what it needs to ask, and says what is being fetched. That
makes two jobs for a phone model, and they may be two models.

**Job 1: decisions.** The same questions Jev answers, answered on the
phone, offline and without the network round trip.

**Job 2: the holding conversation.** Text out, spoken by a voice on the
phone, for up to a couple of minutes.

What was found (research on 2026-11-10; nothing here is measured by us):

| Candidate | Job | Known | Not known |
| --- | --- | --- | --- |
| Apple's on-device model (Foundation Models framework, iOS 26) | 1, perhaps 2 | no download; typed output; about 70 to 86 tokens a second on an iPhone 17 Pro (independent benchmark, estimated) | answered only 60% of that benchmark's questions; context reported as 4K tokens, unconfirmed |
| Liquid LFM2.5-1.2B | 2, 1 | 45.5 tokens a second on an iPhone 17 Pro and 88% on an instruction-following test (the only independent iPhone figures for a Liquid model) | conversational quality |
| Liquid LFM3-1.7B (2026-10-27) | 2, 1 | about 1 GB at 4-bit; vendor says 1.8 to 2.8 times faster than its comparators on an iPhone 17 Pro | absolute speed on any iPhone; reasoning is on by default and turning it off "can be inconsistent" |
| Liquid d1 decision models (opened 2026-10-07) | 1 | answer yes/no and multiple choice in one forward pass with no tokens; vendor: 30 ms on an M5 Pro Mac | any iPhone build or figure; no independent evaluation |
| Liquid LFM3-Audio-2B | 2, speech in and out | turn-based voice chat in English; vendor: first token "as low as 50 ms" | any phone figure; no documented iOS path; interruption would be ours to handle |
| Phonon (Gradium, 2026-10-21), 31M, Apache 2.0 | voice for job 2 | 64 MB; Swift and CoreML; vendor: 28 ms to first audio on Arm CPUs; English only | measured on an iPhone |

Integration: Liquid's own phone SDK (LEAP) is deprecated; their documented
route is llama.cpp's `llama.xcframework` with a grammar to constrain
output. Licence: Liquid's open licence allows commercial use only under
$10M of annual revenue.

**Provisional order to try.** For decisions: Apple's model first (free,
already on the phone), then LFM3-1.7B or LFM2.5-1.2B under a grammar, and
d1 if an iPhone build appears. For the holding conversation: a small text
model streaming into an on-device voice. A speech-in, speech-out model on
the phone is a later experiment, not a plan.

**LFM3-Audio-2B on iOS (owner, 2026-11-10: "definitely get
lfm3-audio-2b on ios").** Second research pass, same day.

- The documented Swift route is FluidAudio's Core ML build
  (`LFM3AudioManager`, iOS 18 and later): offline transcription, live
  English transcription in chunks of about a second, and streaming speech
  synthesis in two voices. It does not document speech-to-speech chat, and
  its authors have not measured it on an iPhone. On an M5 Pro Mac they
  report live transcription at about 10 times real time, synthesis at
  about 4.4 times, 37 ms to first audio, a 1.9 GB peak and a 5.3 GB
  first-load cache.
- llama.cpp merged support on 2026-10-27 and its iOS framework carries the
  multimodal library. Audio in should work. Whether speech out is
  reachable from the framework's public interface is not confirmed. A
  Swift wrapper is one to two weeks.
- MLX Swift has a port of the previous model only (LFM2.5-Audio, with
  speech-to-speech). A port to LFM3 is several weeks.
- The model's voice chat is turn-based and English only. It takes a system
  prompt, so the passage being played can be given to it; the Core ML
  build limits context to 4,096 positions.

Order of work in the spike: run the FluidAudio demo on the iPhone and
measure memory, heat and speed; if it holds, compose the holding
conversation from its live transcription, a small text model and its
voice; try true speech-to-speech only if that composition is not good
enough. The same model would supply the on-device live transcription the
voice design needs.

**Beyond Liquid (second pass).** No new phone-sized decision model from
October or November 2026 was found; that is a limit of the search, not
proof. What changes the plan is a pattern, not a release:

- **A small encoder with answer heads.** One forward pass, no generated
  text, tens of megabytes to a few hundred. An existing example, a
  prompt router built on mmBERT-small, answers seven questions in about
  14 ms on four server cores (its own figure). Ours would be trained on
  Jev's recorded answers to our own questions, which the Jev record
  already collects. This is the likely long-run form of the phone's
  System 1: Jev teaches it.
- **A small generative model used as a scorer.** Constrain the output to
  one token from a fixed answer set, so the time to the first token is
  the whole latency. Qwen3-0.6B (about 456 MB) prefills about 5,800
  tokens a second on an iPhone 17 Pro in a community Core AI export;
  Gemma 4 E2B measures 0.3 s to first token on the same phone (Google's
  figure).
- **Smart Turn v3.2**, an 8M end-of-turn detector with a Core ML port
  (3.5 ms per window on a Mac), is already in the voice design.

**Rules for a phone model, whichever it is.**

- It holds; it does not answer. It works only from the listener's words,
  the passage being played and the prepared material. Where it does not
  know, it says it is finding out. It must not state something the desk
  may then contradict.
- What it said goes into the document as blocks, so the desk reads the
  holding conversation before it revises and the listener hears one
  continuous radio, not two voices in turn.
- Its decisions are recorded like Jev's and scored the same way, so the
  phone model and Jev can be compared on the same questions.
- Jev stays the reference. The phone model earns a question only when its
  record on that question matches Jev's.

**To measure on the owner's iPhone, in the voice spike:** time to the
first token at about 2K tokens of context for Apple's model, LFM2.5-1.2B
and LFM3-1.7B, and for Qwen3-0.6B as a one-token scorer; the FluidAudio
LFM3-Audio demo (memory, speed, heat); agreement with Jev on a fixed set
of our own questions;
time to first audio for the text model plus Phonon; heat and battery over
ten minutes of holding conversation.

## Latency

A live search cannot serve a pivot: one podcast search through Exa took
64 seconds and another 3.4 minutes on 2026-11-10.

- **Prepare when the edition is built.** For each passage the desk already
  holds what it found while building: other human material on the story,
  contrary views, background. A revision that draws on these is fast.
  Which of these are worth preparing is learned, not listed here.
- **Answer from what is in hand first.** The first sentence of a revision
  is spoken when it exists; research arrives as later blocks.

Targets to measure: the acknowledgement begins within one second; a
revision from prepared material is playing within two.

## Learning

Each revision is a precommitment record: what the desk put next and its
probability that the listener wanted it. What the listener does next
settles it, in terms of the document alone: they hear the new blocks
through, they speak again on the same line, they skip, or they correct it.
No route label is needed to score it. The desk reads its own scored record
for this listener when it makes the next revision.

## Takes

A passage of the listener's own that they choose to publish is a take: the
passage (their recorded voice and its transcript) and the reference it
already carries. It goes through the existing publish gate into the
Autopaper object graph, where another listener's desk can find it like any
other human material and play it, attributed. Nothing is published
without the listener's act. This is pass 2 or later; pass 1 only keeps the
reference on every passage.

## Open

- The whole path to the first acknowledgement, timed on a phone: end of
  turn, text, Jev, speech. Jev's part is measured; the rest is not.
- Whether the listener's recorded voice is kept with their passage, or
  only the transcript, before takes exist. Without the audio a passage
  cannot later become a take.
- Whether one document per edition holds up when a session wanders far
  from the edition, or the desk should start a new document. The fork
  mechanism allows either.
