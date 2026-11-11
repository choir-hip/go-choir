# The interview bank (default wording)

This file is the default question bank for the first-run interview (plan: docs/design/autoradio-app-ui-2026-11-10.md,
section 2). It is plain text on purpose: the desk's package will own the live copy, so the wording can be revised
without an app release. `Interview.swift` parses it (`InterviewBank.parse`); a test keeps the copy compiled into the
app (`InterviewBank.defaultText`) identical to this file.

Format: `## id` starts an entry; `key: value` lines belong to it. `{sections}`, `{noted}`, `{topic}` and `{echo}` are
filled in. Entries starting with `@` are not questions.

- `ask` is the question as first said. `rephrase` is said once if the listener did not understand, then the question
  is skipped. `vague` is the line for an answer like "everything" or "the news".
- `aside` is the one sentence, said in passing after the answer, that tells how Choir works. Never a lecture.
- `follow.<word>` is a prepared follow-up for the topics question, chosen by the first word it finds in the answer
  (System 1 can choose by `follow` instead); `follow.*` is the default.
- `kind: follow` marks the second question; it is skipped when the first answer was already specific.

## topics
note: topics
ask: What do you want to know about every day?
rephrase: Tell me one or two things you would like your paper to cover for you.
vague: Let me help. Today's paper has {sections}. Which of those, or something else?
aside: This becomes your edition: a document on your own computer, not a chat. You can read it, change it, or ask me to.

## follow
kind: follow
note: topics
rephrase: Which part of that matters most to you?
follow.tech: When you say tech, is that the companies, the science, or what it does to work?
follow.ai: When you say AI, is that the companies, the research, or what it does to work?
follow.local: When you say local, is that the council, the schools, or what is happening on your street?
follow.world: When you say the world, is that wars and politics, the economy, or something else?
follow.sport: When you say sports, is that results, the business of it, or one team in particular?
follow.*: Which part of that matters most to you?

## tired
note: less
ask: Anything you are tired of hearing about?
rephrase: Is there a subject you would like to hear less of?
aside: The paper learns from what you do. Swipe a story right for more like it, and skip what you do not want.

## sources
note: sources
ask: Any sources you trust, or do not?
rephrase: Are there newspapers, writers or sites you rely on, or avoid?
aside: Every story cites its sources. Tap a number to see where a sentence came from.

## places
note: places
ask: Where are you, and are there places or people you follow?
rephrase: Which city are you in, and is there anyone you want me to keep an eye on?

## listening
note: listening
ask: When do you usually listen, and for how long?
rephrase: Is it mornings, the commute or the evening, and for about how long?
aside: I keep what I learn in documents you can read. Nothing is hidden in a chat log.

## @say
open: Let's make this yours.
ack: Okay.
noted: Got it: {noted}.
noted_interrupt: Right: {noted}. Let's move on.
nothing_noted: Okay, nothing to cut.
interrupt_answer: Mm, {noted}. Skipping ahead.
interrupt_other: Sure. Let me put it another way.
rephrase_lead: Let me put that another way.
skip_lead: No problem, we will leave that one.
whole_paper: Then I will start you with the whole paper and learn as we go.
silence1: Take your time. Or say skip.
silence_pause: I will be here when you are ready.
loud: It is too loud here for me to hear you well. Try headphones, or come back when it is quiet.
resume: Welcome back.
time_up: That is plenty for now.
readback_intro: Here is what I have.
readback_ask: Anything to change?
readback_empty: Not much yet, so I will start you with the whole paper.
readback_again: Okay, I changed that.
closing: Good. Writing your edition now.
closing_limit: Okay, I will go with that. You can change it in the document, just ask.
ask_default: I can answer that in a sentence. Your edition is a document on your own computer that you and I both edit.

## @faq
faq: who|what are you|what is this => I am the radio on your own computer. I read you the paper and I change it when you ask.
faq: private|privacy|listening|record => Your voice stays on this phone as sound. Only the words go to your computer.
faq: why|how come => So the paper can start with what you care about, not what everyone else does.
faq: how|work => You talk, I answer, and a writer on your computer revises the document. You can read every change.
