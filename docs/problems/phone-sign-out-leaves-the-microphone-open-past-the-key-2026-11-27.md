# The phone's sign-out deletes the key before it closes the microphone, and three other gaps

Standing: governing | kind: happened | settled-by: a reading of origin/main 97dea4262 on 2026-11-27; the first item checked by the lead in ChoirApp.swift:213-225 | note: none of the four was reproduced on a device

Found on 2026-11-27 while designing the move of sign-out and the server
client to shared code
([the design](../design/phone-sign-out-and-server-client-in-exact-2026-11-27.md)).
The app has no users yet, so nobody has been harmed. Each item is a reading
of the code, not an observed failure.

## Problem

1. **Sign-out order.** `ChoirApp.signOut` (ChoirApp.swift:213-225) pauses
   the player, clears the deck and waits for shared code, then deletes the
   key (`signIn.signOut()`), and only then does the account wipe run, which
   is where hands-free listening is switched off and the microphone closed
   (AutoradioModule+Onboarding.swift, `reset`). It never cancels a turn in
   flight. So for a short time after the key is gone the microphone can
   still be open and a turn still running for the account that just left.
   A comment on `TurnGate` says it goes false at sign-out; nothing sets it.
2. **Redirects.** The native client checks only the base address before it
   attaches the key. No refusal of a redirect to another host was found.
   Shared code refuses redirects (`redirect: 'manual'`). Whether the
   platform drops the key's header on a redirect to another host was not
   established.
3. **A refused key on the voice turn.** When the server answers 401 to the
   voice turn, nothing tells the account card, so the app can keep trying
   with a key the server no longer accepts.
4. **The native wipe has no inventory.** Shared code's wipe is held to a
   list; the native one is not. Three native files look unwiped at
   sign-out: the console log, `voice-choice.json` and
   `voice-configuration.json`. Whether they hold anything of the person
   was not established.

## Done

Item 1 (2026-11-27, change `sign-out-closes-the-microphone-before-the-key-goes`, three rounds): sign-out shuts a gate with one epoch as its first statement and closes the voice (microphone, turn, speech, the take's give-back, takes that start meanwhile) before the wait for shared code and before the key is deleted; every turn entry, every request a turn makes (the epoch travels with it to `ChoirClient.request`) and the places that open the microphone check it. `SignOutOrderTests` observe the order. Not proved on a device. Items 2 to 4 are untouched.

## 5. A sign-in that lands while sign-out is running is destroyed (found 2026-11-27, pre-existing, not fixed)

On main, sign-out deletes the stored key and runs the wipe unconditionally, so a sign-in that completes while a sign-out is running (during its 5 s wait or its wipe) is destroyed along with the old one: the person signs in again. That is a small failure with no cross-account exposure and the revoke intact. A second hazard of the same race: the account wipe has no guard against a new account's writes while it runs (it bumps the generation first and deletes by store), so a new account's early writes can be deleted. The obvious fix (sign-out deletes only the key it began with and skips the wipe when a newer key landed) is wrong, and was tried and removed: the old account's native stores are then not wiped and the new account can read them; `ModelsEntry.swift:56` still forgets the mail seal, which is the new account's; native answers `done:false` without `superseded` (line 61), and `account.ts:429` then drops the old key's pending revoke, so the old key is never revoked. A correct fix needs native to report `superseded` to shared code and to skip the seal forget, together, and a wipe keyed on the account generation. Evidence: the reviewer's reading of the three files above; `testASignInThatLandsDuringSignOutIsDestroyedAsOnMain` pins main's behaviour.

## Evidence

The file and line for each is in section 3 of the design (items D2, I5,
E3, G2).

## Belief

Item 1 is real and small: the window is the time between two awaited
calls. Items 2 to 4 are probably real and not yet confirmed.

## Remaining error

No test fails today for any of these. Each needs its failing test written
first: 1 and 4 can be tested in the Swift package; 2 needs a local server
that redirects; 3 needs the voice turn against a server that refuses.

## Fix

Stations E6.3 (order) and E6.4 (the inventory) of the design, which do not
wait on the owner. Items 2 and 3 go with the turn's transport (E6.5),
which does.

## Left open by the reviews that accepted af5d69455 (2026-11-27)

Both seats accepted with nothing blocking. These were adjudicated as deferred
to station E6 and are not fixed:

1. If the account wipe hung, the sign-out task would never end and every turn,
   hands-free and take would refuse until relaunch, even after a new sign-in.
   Before this change the key was already gone, so a new sign-in still worked.
   No wipe step was seen to hang; not all were read.
2. `SessionCoordinator.setRecordingTake(true)` checks the gate before taking
   its own lock, and a take whose yield is suspended across a whole sign-out
   opens the microphone briefly before its own re-check gives it back.
3. When the 2 s session move times out, the move's task is not cancelled and
   can hold the coordinator's mutex after the direct close.
4. Nothing stops code inside the sign-out body from awaiting sign-out itself.
   No such call exists today.
5. A typed turn refused because a screen holds turns back shows "Sign in
   first, then ask." to a signed-in person.
6. A `setHandsFree(true, persist: false)` that passes the gate just before it
   shuts can write its override after sign-out cleared it.
7. One actor hop lies between the native op arriving and the gate shutting.
   Closing it by shutting at arrival caused the stuck gate of round 3.
