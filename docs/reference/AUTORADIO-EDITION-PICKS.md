# Autoradio edition picks: the phone's own state, account by account

Status: built on lane `exact-edition-picks` (2026-11-20), checked by bun tests; not run on a phone or a simulator yet (the proof step is in `mobile/scripts/proof-real.sh`).

The picks are the topics the listener chose for their edition. Before this move a Swift session
kept them in `Application Support/Autoradio/edition-topics.json`, written by the topic screens and
read by the paper, the notebook, the DJ's request and the retrieval corpus. Now
`mobile/AutoradioLab/edition-picks.ts` is the ONE reader and writer of
`app:/data/edition-topics.json` (`{"topics": [...]}`), and the tree, tap, typed words and spoken
edits are `edition-model.ts` plus that module. The sources: `editionPicks` (the paper's one-per-line
text), `editionTopics` (the screen), sends `editionToggle`, `editionExpand`, `editionAdd`,
`editionClear`.

## The rule for the old Swift file

The old file carries no proof of whose it is, and the native delete at sign-out is `try?`, so it can
outlive its account. The phone cannot tell, from its own history, whose it is, and every guard of
that kind (a flag, a marker) failed review on the takes page. So the rule is proof, topic by topic,
from the server, with no marker:

- A topic of the old file is adopted only when **the signed-in account's own notebook already lists it**
  under "What you asked for" (`GET /api/texture/edition`, then the document, `edition-picks.ts askedKeys`).
  The notebook is the account's Texture document; the Swift app wrote the picks into it
  (`NotebookStore.writeTopics`, `Notebook.swift` of the old build; the lines are the listener's own,
  merged and never removed). A topic in that list is one this account itself said, so adopting it can
  hand over nothing of another account.
- Only while **no picks are stored yet** (the new file does not exist). Picks already stored are
  the listener's; the old file adds nothing, so a topic taken off never comes back, even when the old
  file's delete keeps failing.
- Then the old file is **dropped**, whatever was decided. A server that cannot say (offline, an
  unreadable notebook) decides nothing and leaves the file; the same evidence decides at the next
  launch.
- No native module (Android, the web): nothing is read.
- What it costs, said plainly: a topic the listener picked and never reached the notebook (the write
  failed or was never run), and every topic of an account with no notebook, are **not carried**; the
  listener picks them again. Topics are not re-adopted after a delete that failed because the new
  file exists.
- Residual: a topic that two accounts both have in their notebooks is adopted by whichever signs in
  first; it is that account's own word either way.

Code: `edition-picks.ts runLegacy`; tests P6 (a to j) in `edition-picks.test.ts`.

## The table

| # | State | Lives in | Belongs to | Cleared by (sign-out, account change) | Can the clearing fail silently? | Across a restart | Test |
|---|---|---|---|---|---|---|---|
| 1 | The picks, their own chips, the expanded rows, the last typed message | shared memory (`edition-picks.ts mem`), one per account generation | the account of its `gen` | a moved generation (`accountGeneration`, bumped by `account.ts` before the wipes) starts a new empty memory that reads row 2 again; `wipePicks` drops it | no (memory) | starts empty, reads row 2 | P4 a |
| 2 | The picks file | `app:/data/edition-topics.json` | the account | `account.ts` store `edition-picks` (`wipePicks`): rm; if rm fails the file is emptied; if neither works the store is named failed, the stamp and marker stay and the next launch wipes again. Every write is `writeCurrent` with the generation the change began under, so a write in flight when the account changes lands nowhere (and is removed if it landed) | the rm and the empty write can both fail: not silent, `account.ts` names `edition-picks` and `leftovers()` looks at the path; the retry is at the next launch (a sign-in in the same process before that reads the emptied file or, if neither worked, the old one: the same window as every moved store) | read again | P4 b d, the wipe-list tests |
| 3 | Changes made at the same time (taps, typed words, speech, the carry-over) | one queue (`mutate`), each applied to the state at write time | the account | a change queued for an account that has moved on is skipped when it runs; a change in flight at the end is dropped | no | gone | P3, P4 c |
| 4 | The native copy of the picks (the DJ's request text, the saved corpus source, the edition writer's input) | native memory only (`EditionPicksMirror`), no file | the account (a copy of row 2) | pushed by shared code after the file was written and never for an account that moved on; the empty copy at every wipe (`wipePicks`) and by `AccountWipe` store `edition-picks`; at the first read of each generation | a push can hang or fail: each is bounded (5 s) and the chain goes on; the native wipe is the other path. Until the first push after a restart it is empty (the DJ then has no topics, not another account's) | gone, empty until shared code's first push | P5 |
| 5 | The words the voice heard at the setup screen | native memory (`EditionTalk`) | the listener at that moment | taken by shared code on the screen's timer; `clear()` when talking stops and at a wipe; the desk's answer has a deadline (6 s). Words for an account that moved on are answered empty and change nothing | no | gone | P7, `EditionSetupTests` |
| 6 | The old Swift file | `Application Support/Autoradio/edition-topics.json` | unknown: decided by proof (above) | adopted-or-refused, then dropped by `editionLegacyDrop`; `AccountWipe` also removes it (`try?`) | yes (both deletes are `try?`); the drop answers truthfully (it looks again) and the rule is safe to repeat | read again at the next first read of a generation | P6 |
| 7 | The edition text written from the picks | the Swift edition writer's memory (`OnboardingController`) | the account | the controller's page state; it reads row 4 when a write starts | no | gone | `EditionCompletionTests` |
| 8 | The notebook's "What you asked for" lines | the server (the account's notebook) | the account | not ours: the account's own document | not ours | server | `notebook.test.ts` |

What is still not true: (a) between a restart and the first signed-in read the native copy is empty
(the DJ names no topics for a moment); (b) the listener whose old picks never reached the notebook
picks again; (c) row 2's retry is at the next launch, as for every moved store; (d) the voice reply at
the setup screen waits for the screen's timer (half a second) and is dropped after six seconds;
(e) nothing here has run on a phone.
