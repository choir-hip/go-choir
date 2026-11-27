---
date: 2026-11-27
title: sign-out closes the microphone and the turn before the key goes
class: red
---
What: `ChoirApp.signOut` now first shuts `TurnGate` (new `closeForSignOut`, which stays shut until the key has been deleted and a key is present again), then runs `LabRuntime.closeVoiceForSignOut`: a take in hand is dropped, the hands-free switch goes off, `TurnEngine.endForSignOut` drops a half-heard turn, cancels the turn already sent with everything it started and cuts the voice, and the audio session is moved off the microphone (awaited). Only then the existing steps run: playback stops, shared code gets up to 5 s to post that it left the air with the old key still held, the key is deleted, the wipe runs. A second `signOut` while one runs joins it (its `stopAll` would reset the first's wait). `typedTurn` now refuses while the gate is shut (the bar's message says "Sign in first, then ask."). Not changed: where the key lives, the server client, what the wipe holds, shared code's sign-out.

Why: before this the microphone was closed only inside the account wipe, after the key was deleted, and nothing cancelled a turn in flight; `TurnGate` was documented as false at sign-out but nothing set it (problem record `phone-sign-out-leaves-the-microphone-open-past-the-key-2026-11-27`, item 1). A kill leaves: before the key is deleted, signed in with the microphone off (hands-free is never kept across a launch) and shared code's pending marker finishes the sign-out; after it, signed out, and the marker finishes the wipe.

Proof: `mobile/AutoradioKit/Tests/AutoradioKitTests/SignOutOrderTests.swift`, 7 tests that observe order (key store delete, session claims, desk cancellation and voice end share one sequence): idle, hands-free listening, a turn in flight (its request never sent after sign-out), the first spoken answer, a wait nobody answers (microphone closed and turn cancelled 1 s in, old key still held), no new turn until a new sign-in, two sign-outs at once. All 7 failed on the old code (24 failed assertions) and pass now. Mutation check: removing the switch-off and session move fails 4 tests; removing the turn cancel fails 4; removing the gate fails 5; deleting the key before the wait fails 1. Swift package tests only. Not proved on a device: on a phone, sign out while it is listening and while it is answering.

Replaced: a sign-out that closed the voice inside the wipe, after the key.

Undo: revert this commit.
