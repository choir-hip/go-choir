---
date: 2026-11-28
title: The reader's aim is shared code
class: orange
---

What: what the bar's prompt is aimed at (a document, or one block of it) is held in shared code (`AutoradioLab/reader-aim.ts`, served from `app.ts` as `readerAimShared` and `readerAimSetShared`), not in Swift. The Swift `ReaderAim` and `ReaderAimInfo`, the module's `readerAim` and `readerAimSet` ops and their two names in `interface/reader.json` and `exact.json` are deleted; the shape `ReaderAimInfo` is declared in `reader-types.contract`. The aim is kept under the account generation, so a sign-out forgets it. New: an aim is dropped when its page is no longer open, or when the document in front is that page, has loaded, and no longer has the block (before, a deleted block's label kept showing). Android has the aim for the first time (its module answered "NotSupportedError" for both names; shared sources do not go through the module). Non-voice native lines 3066 to 3036, names 15 to 13.

Why: nothing native ever read the aim; it was shared-code state held in Swift (on origin/main `git grep -n ReaderAim -- mobile` finds only the class, the wipe in AccountWipe.swift:69 and the two op handlers in AutoradioModule+Reader.swift; the comments saying the voice turn reads it were stale). The first slice of the sentence follow plan ([the plan](../design/sentence-follow-in-shared-code-2026-11-28.md)).

Proof: Account change in a running app: `adoptKey` then `settleAccount` runs `wipeAll`, whose first act is `nextGeneration()` (account.ts:109), and every source's first answer awaits `settleAccount` (app.ts:331), so the aim reads empty before any page can read it; the resource's arguments carry `acs.state` so it is asked again when a sign-in changes the account (account.test.ts S19-aim). `reader-aim.test.ts` (9 tests, red before the module existed, green after; each of A4 to A9 fails under a mutation of the line it guards), the whole `bun test`, the contract build and `exact test web`, `swift test` (MailSeal's K3 skipped), the three gates. Not proved: on a phone (the long-press, the page menu's "Ask about this", the banner), and the iPhone compile of the touched Swift is the landing step's.

Replaced: the Swift `ReaderAim` and its two ops.

Undo: revert the commit.
