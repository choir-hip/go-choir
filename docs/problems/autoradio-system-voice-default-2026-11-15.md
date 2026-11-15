# Autoradio spoke only in the system voice (owner QA round 1, 2026-11-15)

Problem: on a fresh install, everything the app spoke came from `AVSpeechSynthesizer`. The owner: "it's using the built-in Apple fallback
voice. We should be using the better voices." Later the same day he switched the voice in Settings and a better voice loaded and spoke.

Evidence (code read, no device trace was available):
- `StageRegistry.listenerConfiguration()` (`Choir/ChoirApp.swift`) named `avspeech` for the voice stage.
- `LabRuntime.setStage` changed the in-memory configuration only; nothing saved it and nothing read a saved one at launch (`labSelectStage`,
  `AutoradioModule.swift`; `setupFinished`, `AutoradioModule+Onboarding.swift`). A pick lasted until the app closed.
- Nothing recorded that a neural voice's files were on the phone, so no code could prefer one without risking a 350 MB fetch mid-sentence.
- Four call sites made a voice with `registry.make(.voice, ...)` (the programme player, the turn engine) or used `AVSpeechSynthesizer`
  directly (the onboarding hearing check); none could fall back by itself.

Belief: the default and its persistence were the defect, not the neural voices (the owner heard one work). Remaining error: which voice and
how fast on the 14 Pro Max is unmeasured; Kokoro is unvalidated on a physical phone by its vendor.

Fix: [ON-DEVICE-MODELS.md](../reference/ON-DEVICE-MODELS.md) section 9; `VoiceChoice.swift`, `VoiceSampler.swift`, `AutoradioModule+Voice.swift`.
