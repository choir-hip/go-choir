# A Test on Voice and models kept acting after the account left (structural assessment, 2026-11-24)

**Rule 7 (three rounds without convergence).** Seat 2 sent the lane back three times on the same defect, each time one await further along: a cancelled Test still recorded (round 3), still spoke (round 4), still loaded and started playing during the model load (round 5). Each round added a `Task.isCancelled` check at the await the reviewer had named. The next await was always unguarded. Stop patching; change the frame.

## What a Test is

A Test is an **obligation owned by one account generation**: the person pressed Test on a row while account A was signed in; it may open the microphone, record, load a model, run it, play sound and write a line on the row. All of those are effects in the world (the microphone, the speaker, the page). When A signs out, the obligation is void: none of those effects may happen afterwards, however far along the work is, and however the work is suspended (a permission prompt, a model load of seconds, a library call that cannot be cancelled).

`Task.isCancelled` is the wrong owner for that fact: it belongs to a Swift task, is only seen at the points the code looks, does not cross into audio callbacks or libraries, and a sign-out never cancels the audio engine's own work. The owner of the fact is the **account generation** (`AccountWipe.generation`, bumped first by sign-out, before any wipe).

## The mechanism (one)

`TrialGate` carries the generation the Test began in. `gate.live` is true while that generation is the current one and the task is not cancelled. **Every side effect passes through it at the moment of the effect**, not at a few awaits before it:

| effect | where the gate is read |
| --- | --- |
| microphone permission, engine open, tap, mic on | `SetupMicLease.record(seconds:gate:)`: `wanted` includes `gate.live`; `begin` re-asks `wanted` after the permission answer and after the engine opens |
| recording kept | the frame tap adds samples only while `gate.live` |
| a model loads or runs | `ModelTrial` reads it before every backend call and after every return |
| sound starts | `LiveAudio.playVoice(…, gate:)`: before `voiceNode.play()`, before every buffer is scheduled; a dead gate stops the node |
| the page's line | `ModelRunBook.test(…, generation:)` and `ModelTrial`'s `report` |

Tasks are kept by **(generation, row)**, so one account's cleanup can never remove another account's task; sign-out cancels every task, stops the voice and the sample, and clears the run state.

## The proof

A scripted fake backend suspends at every step of every job. For each job and each step k the test signs the account out (bumps the generation) while step k is suspended, releases everything, and asserts that **no backend call and no report happens after the sign-out**; a second run does the same with `Task.cancel`. The same is done for the lease with fake hooks (a sign-out between the permission answer and the engine open opens nothing) and for `playVoice` (a dead gate schedules nothing).

## What this does not prove

The real `LiveAudio` node, the system permission prompt and a real model load run only on a phone; the native wiring (`LiveTrialBackend`, `OnboardingController.reset`) is not compiled in this lane's Swift package tests.
