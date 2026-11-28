---
date: 2026-11-28
title: a key read that Exact refuses is unreadable, not signed out
class: red
---
What: `api.ts` `keptSession` now tells "no key" from "could not read". `null` means only that the source answered `found: false`. A source that says `unreadable`, throws, or answers something malformed (null, no `found` boolean, `found: true` with no value) makes `keptSession` throw a `SignedOutError` of reason `unknown`, or `locked` when the source threw or said unreadable and the key state also says locked. `storeSource` marks a refusal whose message says "no store here" (Exact's between-answers refusal, `door.rs` `no_store`) with one flag, and that refusal is `unknown`, not `locked`. `keptItem` (the `get` helper) tells absent from unreadable the same way. The `api: session` log line gains `refused=true` or `refused=false`. One shared pair, `isUnreadable` and `unreadableWords` (plus `signedOutStatus`), in `api.ts` gives the words and state for a reason, and every site that showed or mapped a `SignedOutError` uses it: Settings account line, Computer and Activity lines, Texture save sentence (an unreadable key is "offline": the edit is kept and tried again), take publish and delete notices, content sync (state `offline`, not `signedout`, so it is asked again), the radio programme post (kind `transport`, "The desk can't be reached right now", not `signedOut`) and the delete-account post (503, "could not do this just now", not 401). `none` and `refused` keep today's words. Pages that already read the reason (files, textures, reader, mail, themes, notebook) are unchanged.

Why: the store works only while an answer runs (Exact `door.rs:52`). A read from a let-go answer threw, and the `session` op turned that into `found:false`, the same as an empty Keychain. The log flipped between signed in and signed out 313 times in 45 seconds. The first version of this change made a refused read an error, but pages that ignored the reason then said "sign in" or "sign out" for it; this version fixes those sites ([problem](../problems/a-signed-in-app-asks-the-server-for-the-changelog-about-forty-times-a-second-2026-11-28.md)).

Proof: `session-unreadable.test.ts` U1 to U12 and `session-reasons.test.ts` (a table of every reason at every site). They show that a refused or malformed read never gives reason `none`, never sends, never lets `credentialsAbsent` answer true, never lets `reconcile` wipe or adopt, never lets a shared-only sign-out report done, never clears the sign-out block, and never makes a page say sign in or out. Not proved on a device. The table and U8 to U12 were written after the code for the second round, not before.

Residuals, not fixed here:
- When there is no store and Exact is not between answers, `store.get` returns null (`door.rs:53`). That reads as absent and this change does not close it.
- A sign-out begun while the key is unreadable cannot name the key's digest, so its block lifts when the same key is readable again (pinned by U12).
- `beginSignOut` writes `blockedDigest` after an await, so an `abortSignOut` during that await is overwritten. This was already so.
- `keyState` answers `unreadable` (with `error`) when the key state says present but the session read is refused. `deskMode` then gives the desk and the account state shows "unknown". This is intended. `keyState` does not carry the between-answers flag, so the account line for a between-answers refusal still says "locked".

Replaced: `session` answering `found:false` for a refused read; a bare `Error` from a throwing session source; page words that said sign in or out for any `SignedOutError`.

Undo: revert the commit.
