---
definition_version: 7
readiness: intent
authority: 'the owner, 2026-11-23, on reading what runs: "How did the voice pipeline not get built?"; "The voice pipeline should work locally"; "I don''t think voice on should be ''call style'' — it shouldn''t sound different to voice off".'
review:
  reviewer: none (an intention recorded from the owner's words; reviewed before it becomes executable)
  frozen_ref: none
  verdict: none
start:
  captured_at: 2026-11-23T22:02Z
  source:
    canonical_ref: origin/main bcea7eefd
    deploy_identity: staging and the paper on a2ba96767
  what_is_true_now:
  - 'The phone''s listener configuration is a fixed list written in the app''s first commit (2026-11-11) and unchanged: a loudness detector, Apple''s recogniser, one second of silence as end of turn, no speaker check, call-style audio.'
  - The voice setup flow downloads and measures better models and picks them, in memory only; nothing reads the picks at launch.
  - 'Built and not the default: Silero speech detection, Parakeet recognition stages, a local System 1 scorer, a speaker gate that needs a learned voice. Not built: the recogniser the owner named (Fluid Speech Mini), a turn-detection model (only the policy file was copied from the owner''s earlier engine), spoken onboarding.'
  - Voice-on uses the call-style session; a full-band trial that keeps normal sound until the listener speaks is on an unmerged branch (lane/voice-fullband) and was never tried by the owner.
  - Each turn makes one System 1 call on the server whose only effect is to cancel the reply when nothing was asked.
  - The thing that answers a turn on the server is one streamed model call with no tools; no Autoradio desk with a REPL exists.
  - 'Faults the owner reported: an answer cut off by a stretch arriving; playback stopping on a switch to another app with voice on; other apps unable to use the microphone during playback (not explained from main).'
  - '2026-11-24: why the pipeline is not the one the owner chose is traced in docs/problems/autoradio-voice-pipeline-not-the-one-chosen-2026-11-24.md (never scheduled; the 11-15 design never reached main). The owner''s earlier research on the rest of the loop is in the Mike project and was never carried here: docs/reports/voice-turn-taking-2026-10-22.md and its six parts (end of turn, barge-in, speaker separation and other voices, frameworks). Its conclusions: a speaker-aware speech detector in front of the turn manager is the cheapest structural fix for other voices; target-speaker extraction (a separator) is not ready as a front end; no vendor isolator fits on-device iPhone with privacy; Apple''s Voice Isolation cannot be set by an app; Smart Turn v3.2 confirms end of turn; echo is handled by one voice-processing engine plus a speaker check. The owner asked on 2026-11-24 about the separator, isolator, turn detector and echo detector: none is in Autoradio''s default path.'
finish:
  deliver: With voice on, the phone detects speech, recognises it, ends the turn and rejects its own echo with on-device models the owner specified, sounds the same as with voice off, answers at once from the phone, and hands over to the Autoradio desk's new queue without a cut.
  artifact: 'on main and on the owner''s phone: the listener configuration read from kept choices, the specified models as defaults where the phone can run them, the session that sounds the same, and a rerunnable turn drill'
  acceptance:
  - action: relaunch after voice setup and read which stages run
    proves: the chosen models persist and are the default
    evidence_class: device drive
  - action: 'a scripted set of turns on the phone: speech over the programme, a pause mid-sentence, the radio''s own voice from the speaker, a cough'
    proves: detection, end of turn and echo rejection are done by the models, with measured error rates
    evidence_class: device drive with counts, rerunnable
  - action: the owner listens with voice on and off
    proves: voice-on does not sound call-style
    evidence_class: the owner's verdict
  - action: ask during playback while a stretch arrives
    proves: the answer finishes and the queue takes over after it
    evidence_class: device drive
  rollback: each station is a commit that reverts
  landing:
    required: true
    environment: staging and the owner's phone where the station touches the app
    required_receipts:
    - pushed_commit
    - ci
    - deploy
    - deployed_acceptance
value:
  better_means: the share of turns that are heard, ended and answered correctly on the phone alone rises, measured on a fixed set, and the listener cannot hear a difference when voice is on
  goodharting_would_be: models installed and not selected; a demo turn in a quiet room; a benchmark on the Mac called a phone result; hiding voice when it fails
homotopy:
  realism_axis: 'one stage at a time behind the same stage contract: detection, then recognition, then end of turn, then echo, each measured on the phone against the baseline it replaces'
boundaries:
  mutation_class: orange for shared code; the native voice is the one place Swift is allowed, with the reason beside each call
  authority_sources:
  - the owner's words in docs/design/design-session-2026-11-23.md; CLAUDE.md
  owner_rulings:
  - decision: speech detection is a local model, not a loudness detector
    said: The phone deciding if I'm speaking is a loudness detector? wtf? Don't we have a local model for this?
    'on': '2026-11-23'
  - decision: the recogniser is the one the owner named, not Apple's
    said: The speech recognizer isn't supposed to be Apple's! ... Fluid speech mini was my request when we built this
    'on': '2026-11-23'
  - decision: end of turn is decided by a model
    said: turn detection is supposed to be done with a model as well, not just waiting
    'on': '2026-11-23'
  - decision: voice-on sounds the same as voice-off
    said: I don't think voice on should be "call style" — it shouldn't sound different to voice off
    'on': '2026-11-23'
  - decision: the pipeline works on the phone; the server judgement per turn was not asked for
    said: I don't remember asking for that. The voice pipeline should work locally
    'on': '2026-11-23'
  - decision: the local model answers while the real desk rewrites the queue, then playback takes over smoothly
    said: the local model should respond while the autoradio desk (real rlm desk) writes the new radio queue as a function call then once it's done it smoothly transitions to take over voice again
    'on': '2026-11-23'
  - decision: onboarding is spoken, after the voice model downloads
    said: done in voice after the voice model downloads (you speak your interests)
    'on': '2026-11-23'
  - decision: signed out is listening only; asking prompts sign-in or sign-up
    said: logged out should be listening only. if you ask it should prompt you to log in/ sign up
    'on': '2026-11-23'
  - decision: the recogniser is FluidVoice-Labs/fluid-speech-mini-coreml; the owner also wants the other voice models (speech separation, voice isolation, turn detection, echo detection) accounted for
    said: https://huggingface.co/FluidVoice-Labs/fluid-speech-mini-coreml ... what about the other voice models? what we were looking into last night. speech separator. voice isolator. turn detector. echo detector. etc
    'on': '2026-11-24'
  must_preserve:
  - voice stays on the device; the owner's recordings are never published
  - only voice is native; page logic stays in shared Exact code
  - 'no switches: a stage that a phone cannot run says so, it is not hidden'
  - one native build at a time through the lock; several slim simulators may run
  excluded:
  - the Autoradio desk itself (M2); this mission owns the phone's side of the handover
  - Android's voice in Kotlin (M4)
stations:
- id: V0
  name: The listener configuration kept and read at launch
  scope: setup's picks persist and are the configuration; the fixed first-commit list is deleted
  class: orange
  review: one seat
  depends_on: []
  status: pending
- id: V1
  name: Speech detection by model
  scope: Silero as the default, measured against the loudness detector on a fixed set on the phone
  class: orange
  review: one seat
  depends_on:
  - V0
  status: pending
- id: V2
  name: The recogniser the owner named
  scope: read its reference first and write its page; the stage built and measured against Apple's and Parakeet on the phone (accuracy, load, memory). Lane voice-default, 2026-11-24
  class: orange
  review: one seat
  depends_on:
  - V0
  status: working
- id: V3
  name: End of turn by model
  scope: the turn model and the end-of-utterance stream from the owner's earlier engine brought in behind the policy that already exists
  class: orange
  review: one seat
  depends_on:
  - V2
  status: pending
- id: V4
  name: Echo and the same sound
  scope: the full-band session (normal sound until the listener speaks) tried on the owner's phone; echo rejection by reference detector and speaker gate; playback that survives a switch to another app; the microphone left free when voice is off
  class: orange
  review: one seat
  depends_on: []
  status: pending
- id: V5
  name: The turn judged on the phone
  scope: the per-turn question answered by the local scorer; the server call removed once the local one is measured as good
  class: orange
  review: one seat
  depends_on:
  - V2
  status: pending
- id: V6
  name: The handover
  scope: the local answer finishes, then the desk's queue takes over; a stretch arriving mid-answer waits; the phone's side of the contract with the Autoradio desk
  class: orange
  review: one seat
  depends_on:
  - 'M2: the Autoradio desk'
  status: pending
- id: V7
  name: Spoken onboarding
  scope: after the voice model downloads the listener says what they are interested in, and that becomes the edition's topics; speaker learning with thresholds fitted on real audio
  class: orange
  review: one seat
  depends_on:
  - V2
  - V3
  status: pending
decisions_for_the_owner:
- id: Q2
  question: May the owner try the full-band side build?
  why: it has waited since 2026-11-17 for a trial
  options:
  - 'yes'
  - 'no'
  default: offered as soon as V4 starts; nothing merges without the owner's ears
now:
  status: working
  as_of: 2026-11-23T22:02Z
  slice: not started; waits for the design discussion and for the records track's turn records
  source_ref: origin/main bcea7eefd
  deploy_identity: staging on a2ba96767
  candidate: none
  believed_state: what runs and why is known from code; nothing in this mission is built
  main_uncertainty: which of the specified models the owner's phone runs within its memory and latency, measured on the phone
  next_observation: the owner's answer on Q1
  blocker: none
  next_action: write up to executable after the design discussion; V0 first
operating_notes:
- Every stage change is measured on the phone against the stage it replaces, on the same recorded set.
receipts:
- docs/design/design-session-2026-11-23.md
- docs/design/autoradio-voice-engine-2026-11-15.md
- docs/current-architecture.md
---

# The voice pipeline as specified

Track T1. What runs on the phone when voice is on is a first-commit baseline. The models the owner specified sit in a setup path that cannot keep its choices. This mission makes the specified pipeline the one that runs.
