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

Item 1 (2026-11-27, change `sign-out-closes-the-microphone-before-the-key-goes`, two rounds): sign-out shuts a gate with one epoch as its first statement and closes the voice (microphone, turn, speech, the take's give-back) before the wait for shared code and before the key is deleted; every turn entry and the one native key read check the epoch. `SignOutOrderTests` observe the order. Left open, found on the way and outside this change: when a sign-in replaces the key during sign-out, native now keeps the new key and skips the wipe but answers shared code `done: false` without `superseded` (ModelsEntry.swift:61), and account.ts then drops the old key's pending revoke (account.ts, the `!superseded` line), so the old key may never be revoked; and the account wipe itself has no guard against a new account's stores written while it runs (it bumps the generation first and then deletes by store). Not proved on a device. Items 2 to 4 are untouched.

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
