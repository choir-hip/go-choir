# Autoradio edition picks: the phone's own state, account by account

Status: built on lane `exact-edition-picks` (2026-11-20), checked by bun tests; not run on a phone or a simulator yet (the proof step is in `mobile/scripts/proof-real.sh`).

The picks are the topics the listener chose for their edition. Before this move a Swift session
kept them in `Application Support/Autoradio/edition-topics.json`, written by the topic screens and
read by the paper, the notebook, the DJ's request and the retrieval corpus. Now
`mobile/AutoradioLab/edition-picks.ts` is the ONE reader and writer of
`app:/data/edition-topics.json` (`{"topics": [...]}`), and the tree, tap, typed words and spoken
edits are `edition-model.ts` plus that module. The sources: `editionPicks` (the paper's one-per-line
text), `editionTopics` (the screen), sends `editionToggle`, `editionExpand`, `editionAdd`.

## The rule for the old Swift file

The old file carries no proof of whose it is, and the native delete at sign-out is `try?`, so it can
outlive its account. Every guard that infers ownership from the phone's history (a flag, a marker)
has a window; none is used. The rule is proof from the server, topic by topic, **and what is stored
is the notebook's own line, never the old file's string**:

- The signed-in account's own notebook (`GET /api/texture/edition`, then its document, "What you
  asked for": `edition-picks.ts askedLines`) is the evidence. The Swift app wrote the picks into it
  (`NotebookStore.writeTopicsNow`); the lines are the listener's own, merged and never removed.
- For each topic of the old file whose key (letters and digits, lower case) is among the notebook's
  keys, the **notebook's line** is stored. The old file only says which lines to carry. So A's "C++"
  never becomes B's pick when B asked for "C" (the key is the same; B's "C" is stored), and a topic
  in a different case or punctuation is stored as B wrote it.
- Whether the picks file existed **at the first read of this account generation on this phone**
  (`existedAtOpen`) decides whether the old file may add anything. What the listener does later in
  the same run does not.
- A proof that cannot be asked (offline, an unreadable notebook, an old file that cannot be read)
  keeps the old file and the page says so; the same evidence decides at the next launch.

### The cells, one test each (`edition-picks.test.ts` P6, c0 to c13)

| # | Old file | Picks file at open | Server | Change since open | Action |
|---|---|---|---|---|---|
| c0 | absent | any | not asked | - | nothing; no server call |
| c10a | unreadable | any | not asked | - | keep |
| c10b | corrupt, or holds no topics | any | not asked | - | drop |
| c1 | topics | there | not asked | - | drop; adds nothing (a topic taken off never comes back) |
| c2 | topics | absent | cannot say | none | keep; the page says "could not be checked yet"; next launch decides |
| c3 | topics | absent | cannot say | yes (a tap) | keep this run; next launch the picks are there, so the file is dropped. **This is the cost**: topics picked in the old build that the listener has not seen, after an offline first launch in which they also tapped, are lost and picked again |
| c4 | topics | absent | answered: no notebook, or no match | - | drop; nothing adopted |
| c5 | topics | absent | answered: match | none | store the notebook's lines in the old file's order; then drop; the first page already shows them |
| c6 | topics | absent | answered: match | none, the write fails | keep |
| c7 | topics | absent | answered: match | yes, while the proof ran | merge the proven lines into what the listener has now, then drop |
| c8 | topics | absent | the account moved on mid-proof | - | write nothing; keep |
| c9 | topics | there (adopted last run) | not asked | - | a drop that failed repeats safely: c1 |
| c11 | any | any | no native module | - | nothing read |
| c12 | collision | absent | answered | none | the notebook's text is stored ("C", not "C++") |
| c13 | topics | absent | slow | - | the first page waits up to 3 s for the proof; past it the page says "Looking for the topics you chose before…" and is asked again on a timer until the proof ends |

Plainly, what it costs: a topic picked in the old build and never written to the notebook, every
topic of an account with no notebook, and c3 above. A topic two accounts both have in their
notebooks is stored from the signed-in account's own line, so nothing of the other account's text
arrives.

Code: `edition-picks.ts runLegacy`.

## The table

| # | State | Lives in | Belongs to | Cleared by (sign-out, account change) | Can the clearing fail silently? | Across a restart | Test |
|---|---|---|---|---|---|---|---|
| 1 | The picks, their own chips, the expanded rows, the last typed message | shared memory (`edition-picks.ts mem`), one per account generation | the account of its `gen` | a moved generation (`accountGeneration`, bumped by `account.ts` before the wipes) starts a new empty memory that reads row 2 again; `wipePicks` drops it | no (memory) | starts empty, reads row 2 | P4 a |
| 2 | The picks file | `app:/data/edition-topics.json` | the account | `account.ts` store `edition-picks` (`wipePicks`): rm; if rm fails the file is emptied, and an emptied picks file counts as cleared in `leftovers` (so a rm that keeps failing does not re-run the whole shared wipe at every launch); if neither works the store is named failed, the stamp and marker stay and the next launch wipes again. Every write is `writeCurrent` with the generation the change began under, so a write in flight when the account changes lands nowhere (and is removed if it landed) | the rm and the empty write can both fail: not silent, `account.ts` names `edition-picks` and `leftovers()` looks at the path; the retry is at the next launch (a sign-in in the same process before that reads the emptied file or, if neither worked, the old one: the same window as every moved store) | read again | P4 b d, the wipe-list tests |
| 3 | Changes made at the same time (taps, typed words, speech, the carry-over) | one queue (`mutate`), each applied to the state at write time | the account | a change queued for an account that has moved on is skipped when it runs; a change in flight at the end is dropped | no | gone | P3, P4 c |
| 4 | The native copy of the picks (the DJ's request text, the saved corpus source, `NotebookStore`'s write, the edition writer when the send carries no list) | native memory only (`EditionPicksMirror`), no file | the account (a copy of row 2) | pushed by shared code after the file was written and never for an account that moved on; the empty copy at every wipe (`wipePicks`) and by `AccountWipe` store `edition-picks`; at the first read of each generation | a push can hang or fail: each is bounded (5 s) and the chain goes on; the native wipe is the other path. Until the first push after a restart it is EMPTY AND NOT KNOWN (`known` false). What each reader does then: the DJ's request has no edition topics; the saved corpus has no topics document; `NotebookStore.refresh` writes nothing (it requires `known` and a non-empty list; an empty copy is never written to the notebook); the edition writer refuses ("Your topics are not loaded yet") unless the done send carries the page's own list, which it does. An empty list is the listener's choice only when pushed empty | gone, empty and not known until shared code's first push | P5, `EditionSetupTests` |
| 5 | The words the voice heard at the setup screen | native memory (`EditionTalk`) | the listener at that moment | taken by shared code on the screen's timer; `clear()` when talking stops and at a wipe; the desk's answer has a deadline (6 s). Words for an account that moved on are answered empty and change nothing | no | gone | P7, `EditionSetupTests` |
| 6 | The old Swift file | `Application Support/Autoradio/edition-topics.json` | unknown: decided by proof (above) | adopted-or-refused, then dropped by `editionLegacyDrop`; `AccountWipe` also removes it (`try?`) | yes (both deletes are `try?`); the drop answers truthfully (it looks again) and the rule is safe to repeat | read again at the next first read of a generation | P6 |
| 7 | The onboarding controller's memory: progress (offered, skipped, editionDone, step), the edition written from the picks, its status | native memory and `onboarding.json` (`OnboardingController`) | the account | `AccountWipe` store `onboarding` (`wipeAccount`: resets it all and removes the file); an edition that finishes writing after the account moved on is dropped (generation check) | the unlink is `try?`: the file is only the progress, and the in-memory reset is what the next account sees | gone; read from `onboarding.json` | not testable in the kit (the controller is in the app target); read by the lead's run |
| 8 | The notebook's "What you asked for" lines | the server (the account's notebook) | the account | not ours: the account's own document | not ours | server | `notebook.test.ts` |

What is still not true: (a) between a restart and the first signed-in read the native copy is empty
(the DJ names no topics for a moment); (b) the listener whose old picks never reached the notebook
picks again; (c) row 2's retry is at the next launch, as for every moved store; (d) the voice reply at
the setup screen waits for the screen's timer (half a second, only while the onboarding page is on
top) and is dropped after six seconds; the voice keeps listening after the page closes (before this
change too); (e) nothing here has run on a phone; (f) the edition writer is still Swift: it is the
onboarding controller's step machine, which moves with the onboarding page (no TS twin is kept).
