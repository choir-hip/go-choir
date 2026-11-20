# A double tap on Record can start two recorders and leak one (2026-11-20)

Status: open, recorded and not fixed here. Found in the review of the takes move (`e9d8ba6a9`); the code is older than that change (`TakeSession.start`, unchanged by it).

## Problem

Two presses of Record in quick succession can run two `TakeSession.start` calls. The second press can overwrite the shared `AVTakeRecorder`'s `AVAudioRecorder` without stopping the first, so a recorder keeps running with nothing holding it: the microphone stays open, the radio's gate is held for a take nobody can stop, and the first file is orphaned in the temporary `takes` folder.

## Evidence

Read, not reproduced. `mobile/AutoradioKit/Sources/AutoradioKit/TakeSession.swift`, `start(_:)`: the guard is `state == .idle || state == .review`, and `state` stays `.idle` across `await micAllowed()` and `await gate.yieldForTake()` (it becomes `.recording` only after `recorder.start`). A second call arriving in either await passes the same guard. `AVTakeRecorder.start` assigns `rec = r` without stopping an existing one. The review (seat 2, 2026-11-20) names it and says it is unchanged from `origin/main`. No trace, no device run.

## Belief

A real, narrow race on the main actor: the awaits are suspension points, so a second `start` interleaves. It needs two taps inside the microphone-permission or gate wait (tens to hundreds of milliseconds, longer on the first launch when the permission sheet is up). Likelihood on a phone is low and the cost is a live microphone, so it is worth a fix; it is voice code, so it was not touched in the takes move.

## Remaining error

Not reproduced, so the window's real width and whether the shared code's own button handling (the press is a `send`, which may already coalesce repeats) hides it are unknown. The fix is likely a `starting` flag set before the first await (or a state `.starting`), cleared on every exit path, and `AVTakeRecorder.start` cancelling a recorder it already holds; the proof is a Swift test that runs two `start` calls against a recorder fake whose `micAllowed` suspends, and checks one `recorder.start`, one gate yield and one give-back.
