# Native pushes that carry account data and no account generation (2026-11-23)

## Problem

Shared code pushes state to native through the app module's calls. Three of them now carry the account generation they were computed under and native drops a stale one (`radioSet`, `editionMirror`, `pageText`,
with `SharedGeneration` in AutoradioKit, the `OpenPageFacts.replace(gen:)` pattern). Other pushes that carry an account's data do not. A push in flight when the account changes lands after the wipe and puts the
old account's data where the next account's native code reads it.

## Evidence

The second review of the account-generation commits (d0089d7ce, b7511375c) listed these as unstamped pushes carrying account data, as given:

- the deck's ops (`deckPlay`, `deckEnqueue`, `deckAct`, `deckAck`: items with an account's words and the held list),
- `retrievalAnswer` (the answer to the voice's retrieval ask),
- the take ops,
- the onboarding calls (`ob*`),
- `contentBusy`.

Not yet shown by a run that any of them lands wrongly; the reading is that nothing stops it. The three stamped calls were found by the same reading and closed (`SharedGenerationTests`, `generation.test.ts`).

## Belief

Same shape as the stamped three: a window of one pulse or one call at sign-out. The deck is the one with the largest consequence (the next account would hear the old account's held entries until the next pump
replaces them). The wipe's last act now announces a generation (`announceGeneration`, account.ts `wipeAll`), so stamping each of these with `accountGeneration()` and taking them through `SharedGeneration.admit`
closes each the same way. The account boundary needs a structural assessment (one stamp for every native call, not one per call) before more per-call guards are added.

## Remaining error

Which of these can in fact be in flight at a sign-out is unmeasured. The JS context may restart with the process alive (the dev menu's reload and the agent's `reload` step re-boot the session, native module kept),
so every stamp depends on the boot announcement (`nativeLater` in api.ts sends `{op: 'generation', boot: true}` first): a stamp added without it would refuse everything after a reload.

## Windows the guard leaves (third review round, 2026-11-23)

Written down for the account-boundary assessment, not closed by more mechanism:

- **The boot lowers `newest` for ops already queued.** `SharedGeneration.boot` is right for a context that starts again, but `radioSet`, `pagesMirror`, the deck and the sign-in ops run as `Task { @MainActor }`: one queued by the
  old context before a reload runs after the new context's boot and is admitted at its old stamp if that is at or above the new `newest`. A token naming the JS context would close it (no token exists). The reload is the dev menu and the agent's `reload` step.
- **`OpenPageFacts` is a fourth guarded store with its own floor and no boot.** After a reload at generation 0 the first `pagesMirror` is refused silently (`holdPages` answers false, pages.ts ignores it), and the voice's page facts stay empty until the floor is passed. The same reload problem as the three stores.
- **Two contexts.** A second shared-code context (the agent's and the UI's) can lower the floor with its boot.
- **A boot that fails** is now logged (`api: the boot announcement failed`), not retried.
- **Closed in the third round:** the block is set before `beginSignOut`'s awaits; a sign-out given up after a native throw does not give the radio the new generation; the sheet file is removed in a `finally` and a failed removal is retried before any radio restores (the next radio starts empty); the wipe's announcement has a deadline and says when it failed.
