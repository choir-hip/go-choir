---
date: 2026-11-28
title: a key that could not be read is not "no key": the reason is kept and the pages word it; the sign-in sheet is still offered on it
class: red
---
What: `api.ts` `keptSession` tells "no key" (null, only for a clean `found: false`) from "could not read" (a `SignedOutError` of reason `locked` or `unknown`, never `none`). The reason comes from the failing read itself: a source that answered `unreadable` is `locked`, a refusal whose message contains "no store here" (Exact's between-answers refusal, `door.rs:421`) is `unknown` and carries `noStore`, a malformed answer is `unknown`. The key state is asked again only when the source threw with nothing to say. `keyState` carries the `noStore` flag. A refusal from a let-go answer (`noStore`) stops quietly (`refusedHere`, texture-edit's `isLetGo`: the journal is kept, no `failed` label). The error is recognised one way, by name (`isSignedOut`). A shared pair, `unreadableWords` and `signedOutStatus`, words and maps it for the pages that showed sign-in or sign-out advice: Settings, Computer, Activity, Texture save, takes, content sync (offline), the radio programme post (transport), delete account (three answers, nothing posted for the third). `session-guard.test.ts` fails for any non-test `.ts` file that touches sign-in state without a listed decision, and for any `instanceof SignedOutError`.

The sign-in sheet is NOT fixed here. `au.signedIn` is false for a key that could not be read, and the contracts still open the sheet on it (ask bar, microphone, Record, Mail's ask, the first-run card), as on main. A build that moved those sites to the account's own answer was sent back in a fourth review (a refused key then opened the sheet and also sent the ask, clearing the typed text) and was taken out. The open problem: [the sign-in sheet is offered while the key cannot be read](../problems/the-sign-in-sheet-is-offered-while-the-key-cannot-be-read-2026-11-28.md).

Why: the store works only while an answer runs (`door.rs:52`). A read from a let-go answer threw and was answered as "no key" for the `session` op. The trigger of the request storm is removed in another lane ([problem](../problems/a-signed-in-app-asks-the-server-for-the-changelog-about-forty-times-a-second-2026-11-28.md)). Three review rounds each found one more channel that held "signed in" as two answers, and a fourth sent back the changed approach; the lane was cut down to the parts no review faulted ([evidence](../evidence/key-read-refused-three-rounds-2026-11-28.md)).

Proof: bun tests `session-unreadable.test.ts` (U1 to U15), `session-reasons.test.ts` (every reason at every word and state site), `session-guard.test.ts`, `account-state.test.ts` (choirAuthState's four cases, the action of every state), `texture-edit.test.ts` T14b and T14c, `account-delete.test.ts` D6b and D6c. Not proved: anything about the contracts; they are as on main. Nothing ran on a device.

Replaced: `session` answering `found:false` for a refused read; page words that said sign in or out for any `SignedOutError`.

Residuals, not fixed here (from the evidence record and the reviews):
- The sign-in sheet on a key that could not be read (above).
- `members-settings.contract` and `members-notebook.contract` pass the plain flag to their sources, so on a locked or unknown key the Computer buttons and the notebook skip their work as if signed out.
- `bar.ts` turns an engine error whose text contains "not signed in" into sign-in words; whether a locked-key error can reach it through the native voice engine is not traced.
- `keptItem` does not carry the between-answers flag; `content-run.ts` words a between-answers refusal as offline where it should stop quietly; `keyState` reports a locked session read as unknown.
- With no store and Exact not between answers, `store.get` returns null, which reads as "no key" (`door.rs:53`). Not closed.
- If Exact rewords "no store here", a between-answers refusal reads as `locked` ("Unlock your phone"): safe, slightly wrong words, and it no longer stops quietly.
- A sign-out begun while the key is unreadable cannot name the key's digest, so its block lifts when the same key is readable again (pinned by U12). `beginSignOut` writes `blockedDigest` after an await, so an `abortSignOut` during that await is overwritten (older).
- Whether errors are ever copied across a boundary that breaks `instanceof` is not shown; recognising by name costs nothing.
- The larger fix: read the key inside the answer that starts a piece of work and hand it on as a value, so nothing reads the store between answers.

Undo: revert the commit.
