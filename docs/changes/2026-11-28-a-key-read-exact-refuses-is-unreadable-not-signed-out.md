---
date: 2026-11-28
title: a key read that Exact refuses is unreadable, not signed out
class: red
---
What: `api.ts` `keptSession` now tells "no key" from "could not read". A `session` read that the store refuses (the secret store throws outside an answer) is answered `unreadable` by `storeSource`, and a source that says `unreadable` or throws makes `keptSession` throw a `SignedOutError` of reason `locked` (the key state says locked) or `unknown`, never `none`. Every caller already catches or passes it on: `send` and so every page (words: "Unlock your phone" or "Could not check your sign-in just now"), `beginSignOut`, `refreshSealBlock`, `accountId` and `keyState` (which then answers `unreadable` with `error`, so `reconcile` wipes nothing). The log line `api: session` gains `refused=true` or `refused=false`.

Why: the store works only while an answer runs (Exact `door.rs:45-53`). A read from a let-go answer threw, and the `session` op turned that into `found:false`, the same as an empty Keychain. The app log flipped between signed in and signed out 313 times in 45 seconds, and a page could show "Not signed in" for a listener who was signed in. The trigger is removed in another lane; this closes the weakness under it ([problem](../problems/a-signed-in-app-asks-the-server-for-the-changelog-about-forty-times-a-second-2026-11-28.md)).

Proof: `session-unreadable.test.ts` U1 to U7 (18 tests): a refused or throwing session read never gives reason `none`, never sends, never lets `credentialsAbsent` answer true, never lets `reconcile` wipe or adopt, never clears the sign-out block, never lets a shared-only sign-out report done. Not proved on a device. Not fixed: a sign-out begun while the key is unreadable cannot name the key's digest, so its block lifts for any later key, as it does when no key exists.

Replaced: `session` answering `found:false` for a refused read; a bare `Error` from a throwing session source.

Undo: revert the commit.
