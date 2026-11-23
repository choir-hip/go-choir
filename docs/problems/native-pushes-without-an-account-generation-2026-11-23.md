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
