# Autoradio edition picks: the phone's own state, account by account

Status: built on lane `exact-edition-picks` (2026-11-21), checked by bun tests and Swift tests; not run on a phone or a simulator yet (the proof drive is in `mobile/scripts/proof-real.sh`).

The picks are the topics the listener chose for their edition. `mobile/AutoradioLab/edition-picks.ts` is the ONE reader and writer of `app:/data/edition-topics.json` (`{"topics": [...]}`);
the tree, tap, typed words and spoken edits are `edition-model.ts` plus that module. Sources: `editionPicks` (the paper's one-per-line text), `editionTopics` (the screen); sends `editionToggle`,
`editionExpand`, `editionAdd`.

## The old Swift file: nothing is carried

Before this move a Swift session kept the picks in `Application Support/Autoradio/edition-topics.json`. That file says nothing about whose it is, and the native delete at sign-out is `try?`, so it
can outlive its account. Three designs that carried it (a flag, a marker, a proof against the account's notebook with a pending field) each had a hole reviewers found. The server notebook is not a
record of the picks either: `NotebookText.merging` (`Notebook.swift:96`, `notebook.ts:104`) only adds topic lines, never removes one, and it is written only when an edition is finished or the notebook
page refreshes, not on each change.

So: **this build never reads the old file and does not delete it.** An upgraded phone starts with NO picks; the listener picks again. That is the cost, stated plainly. The file is left unread because
deleting it would make the upgrade irreversible for the listener's own settings and the owner has not ruled on losing them; an unread file is harmless and leaves a later, proven adoption possible.
The native account wipe at sign-out still removes it (`AccountWipe.swift`, store `notebook`, `legacyTopicsFile`), as before. Nothing else reads that path (checked: `Notebook.swift`, `SavedCorpus`,
`DemoLeftoversTests` only name it).

## The state, row by row

| # | State | Lives in | Belongs to | Cleared by | Can the clearing fail silently? | Across a restart | Test |
|---|---|---|---|---|---|---|---|
| 1 | The picks, own chips, expanded rows, last typed message | shared memory, one per account generation | the account of its `gen` | a moved generation starts a new memory that reads row 2; `wipePicks` drops it | no | starts empty, reads row 2 | P4 a |
| 2 | The picks file | `app:/data/edition-topics.json` | the account | `account.ts` store `edition-picks` (`wipePicks`): rm, else empty it, else name the store failed; every write is `writeCurrent` with the generation the change began under | rm and empty can both fail: `account.ts` names the store, keeps its marker and retries at the next launch (the same window as every moved store) | read again | P4 b d d2 |
| 3 | Changes at the same time | one queue (`mutate`), each applied at write time | the account | a change queued for an account that moved on is skipped | no | gone | P3, P4 c |
| 4 | (removed 2026-11-25, lane `move-b`) There is no native copy. Its only reader was the voice turn's retrieval ask, which handed it straight back to shared code; the retrieval now reads the picks here (`retrieval-live.ts` `sharedFacts`) | - | - | - | - | - | P5 (nothing is pushed), `round-trips.test.ts` C3 |
| 5 | Words the voice heard at the setup screen | native memory (`EditionTalk`) | the listener then | taken on the screen's timer; `clear()` at stop and wipe; the desk's answer has a deadline (6 s); words for a moved account are answered empty | no | gone | P7, `EditionSetupTests` |
| 6 | The old Swift file | Application Support | unknown | never read here; removed by `AccountWipe` (`try?`) | yes, and harmless: nothing reads it | stays | P6, `EditionSetupTests` |
| 7 | The onboarding controller's memory (progress, step, edition, status) | native memory, `onboarding.json` | the account | `AccountWipe` store `onboarding` (`wipeAccount`); an edition, a voice setup or an edit-page Done that arrives after the account moved is dropped (generation checks) | the unlink is `try?`; the in-memory reset is what the next account sees | read from the file | not testable in the kit |

## "Loaded", and empty as a choice

The page says `loaded` once the file was read for this account generation (`EditionPage.loaded`; the contract's `else empty()` frame has it false). The three done sends (`onboardingTopicsDone`,
`onboardingEditDone`, `onboardingEditionRetry`) are sent only `if ep.loaded`, and carry the page's list; An empty list read from an absent file IS the listener's state (nothing picked is the general edition, E4); a list that was never loaded is never sent.

What is still not true: (a) every upgrading listener picks again; (b) a failed wipe is retried at the next launch; (c) the voice keeps listening after the page closes (before this change too), only
the half-second poll stops with the page; (d) nothing here has run on a phone; (e) the edition writer is still Swift: it is the onboarding controller's step machine and moves with that page.
