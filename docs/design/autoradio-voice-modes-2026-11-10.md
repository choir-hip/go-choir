# Autoradio voice: three modes and a hybrid (2026-11-10)

Status: design, not built. Research only; nothing here is measured on a
phone. Mutation class of this document: green. Rollback: revert.

Owner, 2026-11-10: "Pure push to talk is not above the usability
threshold." "We should basically have all three of these as different
options to play around with. And then we can try to hybridize them. And I
want to pull this forward because this is very crucial to the product
experience. ... we do need to be above that threshold from the get-go."

This note changes the [Autoradio goal](../definitions/choir-autoradio-pass1-2026-11-10.md):
voice is built and judged first, before the shell and the renderer.

## What iOS allows

From Apple's documentation and developer forum threads read on 2026-11-10.
Items marked (unconfirmed) have no Apple statement behind them.

1. **Echo cancellation costs playback quality.** Apple's voice processing
   cancels the app's own audio, but everything it plays is ducked and
   band-limited "by design". A podcast played through it sounds like a
   phone call. This holds on the speaker and on headphones.
2. **A Bluetooth microphone forces call-quality playback.** Input over the
   Hands-Free Profile takes the output with it. The alternative is the
   phone's own microphone with A2DP output (unconfirmed; one WWDC lab
   report). iOS 26 adds a high-quality AirPods recording option, but not in
   the EU, with more latency, and Apple advises against it for real-time
   use.
3. **The microphone cannot be opened from the background.** A press on a
   headphone button while the phone is locked cannot start recording unless
   a record session is already live. So any locked-screen voice mode keeps
   a session open the whole time, and the orange indicator stays on.
4. **Headphones remove the echo problem.** With playback in the ears, the
   phone's microphone does not hear it, so echo cancellation is not needed.
   The problem is the loudspeaker and the car.
5. **No power figures exist** for any of this on iPhone. They have to be
   measured.

## What exists

The owner's Mike speech plugin (read-only review, 2026-11-10) has:

- open hands-free mode with Apple's voice processing;
- an interruption rule: 300 ms of speech and two words not in the clause
  being spoken; fillers do not count; playback pauses at 700 ms and resumes
  if no words follow within 1.5 s;
- a learned voice with no enrolment screen (three agreeing turns), after
  which another voice cannot interrupt and its turns are dropped;
- end-of-turn detection (Smart Turn v3.2).

It lacks a wake word, a time window, and background use. Its session is set
to voice chat with the Bluetooth call profile.

Wake word, from research:

| Route | Custom word | Size | Licence |
| --- | --- | --- | --- |
| Soniqo `SpeechWakeWord` (open vocabulary) | typed as text, no training | about 4 MB | model Apache-2.0; package licence to confirm |
| LiveKit `livekit-wakeword` | trained in about an hour on a Mac from synthetic speech | 451 KB to 7 MB | Apache-2.0 |

Porcupine ended its free tier in June 2026. openWakeWord's ready models are
non-commercial. Apple has no wake-phrase API for other apps.

"Choir" alone is too short (about four phonemes; guidance is six or more).
The candidates are "Hey Choir" and "Okay Choir". Bare "Choir" is included
in the test to show the difference.

## The design: two audio states

The constraints above conflict only if one audio configuration must serve
both listening and talking. It need not.

**Listening state.** Playback at full quality. No voice processing. The
phone's own microphone feeds only the wake word detector and, in a window,
the voice activity detector. Over headphones the microphone hears no
playback. On the loudspeaker it hears the radio, which the wake word must
tolerate and the speaker check must reject.

**Talking state.** Entered when the listener is addressed or speaks.
Playback pauses, so there is nothing to cancel while the listener talks.
The answer is spoken with voice processing on, so the listener can
interrupt the answer. Call-quality audio is acceptable for a spoken
answer. When the exchange ends, the app returns to the listening state and
resumes the programme.

The cost is the switch between the two. One iOS 27.0.1 report describes a
20 to 41 second stall when the route changed with audio rendering. The
spike measures the switch.

## The modes

All three are built, behind one mode setting the owner changes in QA. This
is a product setting on the listener's device, not a platform switch.

| Mode | Opens the talking state | Notes |
| --- | --- | --- |
| Button | headphone play/pause, lock-screen control, on-screen hold | needs the live session of item 3 |
| Wake word | "Hey Choir" from either detector | detector runs in the listening state |
| Open | any speech that passes Mike's interruption rule and speaker check | over the loudspeaker this needs voice processing all the time, so playback is call quality; over headphones it does not |

**Hybrid (the owner's proposal).** Closed until the wake word or the button.
Then open for a window. The window's timer resets only on a turn that
passes the speaker check, so a television or a passenger cannot hold it
open. After the window, closed again. Until the voice is learned the window
is 30 seconds, because nothing rejects other voices yet. The window length
is the number to tune: the owner proposes five minutes; no published
system uses more than about ten seconds, so this is new and the speaker
check is what makes it plausible.

## Addressing: a cascade, not one detector (added 2026-11-10, second pass)

Owner, 2026-11-10: no "hey" or "okay"; candidate bare words are "radio",
"choir", "DJ" and "tetra"; "maybe custom words are the right approach.
That could be part of onboarding"; "we need to be able to model the
accuracy during onboarding to guide the user to pick a functional wake
word".

Two more research passes (history, and 2026 only) changed the plan:

- **Nothing in 2026 replaces a small detector** for a phone playing speech.
  The year's papers join a text-defined keyword with speaker verification
  in one tiny model, and detect device-directed speech from interaction
  history, but none has released code or tested against playback.
- **The text-defined detector is too loose alone.** Its own figure is 0.27
  false triggers per utterance on its negative set. It is a first stage.
- **Bare short words need the extra stages.** Apple shipped bare "Siri"
  only with a second-stage checker and speaker identification run twice,
  on the trigger and again on the trigger plus the request.
- **The device's own speech is the adversary.** Commercial speakers woke
  about once an hour on television dialogue (PETS 2020). Amazon's paper on
  barge-in during playback finds the fix is the playback reference, not the
  word.
- **Amazon patented a way to score a word before training** (US 9,275,637):
  phonetic distance to a corpus of real speech, how often the phone
  sequence occurs in it, neighbours in the lexicon, pronunciation
  ambiguity, and frequency.

The cascade:

1. **Loose first stage.** The text-defined spotter on the phone's
   microphone, set to fire easily on the listener's word.
2. **Self-reference veto (our own idea, untested).** The app knows exactly
   what it is playing. Run the same spotter on the playback signal itself.
   A trigger on the microphone that coincides with a trigger on the
   playback is the programme, not the listener, and is dropped. This costs
   no playback quality, unlike voice processing.
3. **Confirmation.** On a surviving trigger only: the speaker check on the
   trigger and the words that follow it, and a transcript match on that
   short stretch of audio.
4. **Window.** After a confirmed turn, no word is needed until the window
   lapses.

Because stages 2 and 3 do the rejecting, the word matters less than it
would with a single detector.

### Choosing and grading the word

There is a default word, chosen by the owner after the bench results, and
onboarding lets the listener choose their own. The listener says the word
four or five times; the same recordings enrol the voice.

Onboarding grades a candidate two ways:

- **Will it fire by itself?** From text, at once: the patent's measures
  over transcripts of our own programme audio. Then acoustically: the
  spotter set to the word, run over a stored sample of programme audio.
- **Will it miss this listener?** The spotter's score margin on the
  enrolment recordings, clean and mixed with programme audio.

The verdict is a precommitment record with a stated probability. The field
settles it: a wake followed by an immediate cancel is a false trigger; a
button press straight after a failed attempt is a miss.

### The test bench

Offline, on the build VM, before a phone is involved. For each candidate
word:

- positives from real speakers (single-word clips cut from Common Voice,
  with its accent labels; the Multilingual Spoken Words Corpus where it has
  the word) and from multi-speaker synthetic speech for words nobody has
  recorded;
- noise: programme audio mixed at a ladder of levels, standard noise sets,
  simulated room and pocket acoustics;
- negatives: hours of programme audio, general speech without the word,
  and a list of near-sounding words.

Measured: miss rate by accent and noise level, false triggers per hour on
each negative set, and the best threshold. Run over about two hundred
words, the results are the ground truth for fitting and checking the
onboarding grader. First run: "radio", "choir", "DJ", "tetra", their "hey"
forms for comparison, and a spread of coined and rare three-syllable
words.

Limits: the bench runs the open model, the phone a compressed build of it;
simulated rooms are not real rooms. The bench ranks words; the owner's QA
session gives the absolute numbers.

## The radio is already running (owner, 2026-11-10, third pass)

Owner: "it shouldn't stop playing as soon as it feels like the user starts
speaking ... if the user doesn't produce a cogent utterance, it doesn't
count ... it's not waking it up. The system is already running. ... when
you interrupt it, you're probably going to give more than one word ... a
cogent statement and usually relevant to the content."

This is the difference from a smart speaker, and it replaces "wake, then
listen" as the main path:

- **Keep playing and judge the utterance.** Speech in the listener's voice
  is buffered and transcribed while the programme continues. It counts as
  an interruption only when it is a complete, cogent utterance. Noise,
  fillers, half-words and other voices never register, so there is nothing
  to resume from.
- **Soft acknowledgement.** At the first second of the listener's speech
  the programme lowers its volume a little; it returns if the utterance
  does not count. Mike's hold-and-resume becomes duck-and-return.
- **Sustained speech pauses the programme** (owner, 2026-11-10: "if the
  user keeps talking for more than like 10 seconds straight, then it
  should pause ... one of the key usage patterns is the user giving
  relatively long dictations"). Three stages: the dip at about one second;
  a pause as soon as the utterance is judged to count; and a pause in any
  case after a set stretch of continuous speech in the listener's voice,
  starting at 10 seconds and tuned in QA. Once paused, nothing is playing,
  so recognition is at its best and there is no echo. Dictation has no
  length cut (Mike cuts a turn at 60 seconds; that limit goes) and a
  patient end-of-turn, since people pause to think. When the turn is
  handed off, the radio acknowledges at once and then decides what plays
  next; resuming is one outcome among several
  ([autoradio-turns-2026-11-10.md](autoradio-turns-2026-11-10.md)).
- **The app knows what it is saying.** It has the text of the story being
  read and of the clip being played. Words that match the programme are
  the programme. An utterance about the current content, or a command, is
  likely addressed to it. This is the relevance test and the echo test,
  from the same known text.
- **The word becomes an override, not a gate.** Saying the word first makes
  a short or off-topic utterance count ("tetra, pause"), and it is what
  works when the room is noisy. The agent's coined three-syllable words are
  dropped (owner: "we just can't force users to use cadenza or marimba").
  If "tetra" does not measure well, the answer is a word the listener
  chooses.
- **Noisy rooms.** People seldom talk to a radio over a television or in a
  café. When other voices persist, the app says so on screen and requires
  the word or the button until the room is quiet. It does not try to win
  that case.

What this costs, to measure in the spike: transcribing continuously while
playing (power), and the seconds the programme talks over the listener
before the utterance is judged. The voice check runs first so recognition
runs only on the listener's voice.

## Which microphone (owner, 2026-11-10)

The microphone source is a listener setting, with a default per route.

| Route | Microphone | Playback | Hears |
| --- | --- | --- | --- |
| Loudspeaker | phone | full quality | the programme and the room |
| Wired or USB-C headset | headset | full quality | mostly the listener |
| Bluetooth headphones | phone (default) | full quality (unconfirmed) | the room, not the programme |
| Bluetooth headphones | headphones | call quality, or the iOS 26 high-quality mode (not in the EU, more latency) | mostly the listener |

Two microphones at once (phone and Bluetooth): believed not possible. An
iOS audio session has one input route at a time; this is from general
knowledge and is a spike question. The phone's own several microphones can
be used together, which is what the close-to-microphone research relies
on.

## What the spike must answer

On the owner's iPhone, with a podcast playing, on the loudspeaker and on
AirPods, screen locked:

1. Does A2DP playback with the phone's own microphone hold? What does the
   microphone hear from a pocket?
2. How long does the switch between the two audio states take, and is it
   audible?
3. Does each wake word detector fire on "Hey Choir", and how often on the
   programme itself over the loudspeaker?
4. Does Mike's speaker check hold on a real microphone over programme
   audio? (Its accuracy is measured only on simulated audio.)
5. Battery per hour in each mode.
6. Each row of the microphone table: what the route really is, what
   playback sounds like, and whether a second input can be opened.

## The QA session

About 30 minutes, each mode for six, the same script in each: interrupt on
purpose at nine random points; play a television across the room; have a
second person speak; say "mm-hm" during answers.

Record per mode: false interruptions per ten minutes; missed interruptions
of nine; turns taken from the television or the second person; time from
the first word to the audio stopping; times the window closed while the
listener was still engaged; battery used.

The gate is the owner's judgement that at least one mode or the hybrid is
above the threshold. The numbers are for hill-climbing after that.

## Not decided

- Whether the car is in pass 1. Recording in a car switched one
  developer's route to a call route.
- Whether to ask Apple for the PushToTalk entitlement, the one documented
  way to open the microphone from the background. It is meant for
  walkie-talkie apps.
- App Review. Reviewers have rejected always-listening apps when the
  reason was not visible. The app shows a listening state and a stop
  control.

## Sources

- Apple: [playAndRecord](https://developer.apple.com/documentation/avfaudio/avaudiosession/category-swift.struct/playandrecord),
  [voiceChat](https://developer.apple.com/documentation/avfaudio/avaudiosession/mode-swift.struct/voicechat),
  [bluetoothHighQualityRecording](https://developer.apple.com/documentation/avfaudio/avaudiosession/categoryoptions-swift.struct/bluetoothhighqualityrecording),
  [ducking configuration](https://developer.apple.com/documentation/avfaudio/avaudioinputnode/voiceprocessingotheraudioduckingconfiguration),
  [interruptions](https://developer.apple.com/documentation/avfaudio/handling-audio-interruptions),
  [PushToTalk](https://developer.apple.com/documentation/pushtotalk/creating-a-push-to-talk-app).
- Apple forums: threads 829753 (band-limited by design), 784318 (built-in
  microphone with A2DP), 120038 (no recording start in the background),
  853278 (route switch stall), 836224 (car), 826849 (review rejection).
- Wake word: [livekit-wakeword](https://github.com/livekit/livekit-wakeword),
  [Soniqo guide](https://soniqo.audio/guides/wake-word),
  [Picovoice on word choice](https://picovoice.ai/docs/tips/choosing-a-wake-word/).
- Follow-up windows: Alexa Follow-Up Mode (5 s), Google Continued
  Conversation (about 8 s).
- Second pass: [Apple voice trigger](https://machinelearning.apple.com/research/voice-trigger),
  [PETS 2020 misactivations](https://petsymposium.org/popets/2020/popets-2020-0072.php),
  [Amazon on barge-in during playback](https://arxiv.org/pdf/2111.10639),
  [wake word evaluation patent](https://patents.google.com/patent/US9275637),
  [ZP-KWS](https://arxiv.org/abs/2606.20106),
  [addressee detection](https://arxiv.org/abs/2604.08412),
  [prefix bias](https://arxiv.org/abs/2602.08930).
