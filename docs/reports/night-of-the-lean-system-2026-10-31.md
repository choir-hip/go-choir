# The night of the lean system

Written early on October thirty-first. It covers about six hours of work, from late on the thirtieth until about four in the morning, on the lean system mission, after you went to bed.

The paper went down for about an hour tonight because of my change, and it is back, lighter at boot, and the one-tape design you approved is now built far enough to export and verify a real computer.

## Where things stand

The paper is up and serving the latest release. Boot is light now: nothing heavy runs before the computer has its credentials, and compaction no longer runs at boot at all. The old trace log, the one we called R seven, is deleted. One append-only tape is the only durable record, and telemetry can ride it in sealed batches. That batching is on for d@e, your test computer, and nowhere else yet. D@e booted with it on at three thirty-seven; I have not yet seen its first sealed batch, because d@e has not run a tool since. The crash-loop hold passed its drill on d@e. The old candidate computers are gone. A bundle tool now exports a computer's tape and snapshot into a folder and verifies it, and d@e's first bundle verified tonight. A Sonnet orchestrator on the agents machine is working through the test suite, looking for tests to cut and slow tests to speed up.

Two decisions are yours this morning: whether the privacy policy should promise that a deleted item leaves the tape, and whether tool results should leave the working store after seven days.

## The outage

At about one twenty-five, the batch with the event index trim reached the paper. I had put a catch-up step inside the store's opening, and on the paper's thirty-two gigabyte store it ran for ten minutes. The store opens before the computer exchanges its boot credential, and that credential only lives fifteen minutes. So the exchange refused it, vmctl killed the boot, and the paper stayed down from about one twenty-five to two twenty-two. That is about fifty-seven minutes.

The fix follows your words: boot should be light, then essential services, and compaction is its own heavy operation. The catch-up now runs in the background, two minutes after the store opens, and boot never compacts. Then the paper reported its store stalled, nine minutes after the new boot. The background catch-up was holding the paper's single writer almost all of the time. DeepSeek's review also found that the trim inside each write deleted the whole backlog at once. A second fix made the trim delete only its own small interval, and made the catch-up rest at least four times as long as each chunk took. It now uses about a fifth of the writer at most. That has held since.

The lesson is the one we already wrote down on the twenty-fourth and I failed to apply: anything slow before the credential exchange ends the boot. I now check every boot-path change against the fifteen-minute envelope, not the ninety-minute readiness timeout.

## Main was wedged, and the force-push

Before the outage, main was wedged for about an hour and forty minutes. The docs triage rewrote four doc paths in comments of the continuous integration workflow file. Any change to that file counts as a security gate change, and a pushed commit cannot gain the trailer it needs. Devin reviewed the hunks as comments only. You approved a force-push, and I rebuilt main with the trailer on that one commit and everything else unchanged. The old main is kept on a backup branch. The lesson is that a bulk path rewrite must leave the gate files alone.

## The crash drill and the candidate cleanup

I killed d@e three times inside half an hour. The first two crashes recovered on the next request, about thirty seconds each, with their open work closed as interrupted, never resumed. The third crash put d@e in a crash-loop hold, and every automatic start was refused. Your start cleared the hold. That is the restart rule doing what it says.

You called the old candidate computers trash, so I removed a hundred and forty-five of them, everything last active before October. The last old one, candidate control from July, refused removal. Its routing record no longer joins its ownership. I added a narrow override to vmctl for exactly that case, reviewed by Sonnet, Devin and DeepSeek. It only applies to a stopped or hibernated computer, idle for more than thirty days, and never to a protected one. It deployed at three thirty-eight, and the old row is gone. Six computers remain.

## The one-tape design

You approved one tape as the single source of durability, with recovery, backup and sync to your Mac as one mechanism. I wrote that design and built its first slices. The trace log is deleted. A settings bug meant a setting that defaulted to on still read as off on every computer that had never stored it, so telemetry retirement had never really run on most of the fleet. It does now. The bundle tool exports, verifies and imports. Your Macs are the first off-host copy, and paid server backup waits until the system is stable.

Three reviewers, Devin, DeepSeek and Sonnet, each said the design is partly aligned, and they agreed on why. The bundle carries no keys, so it can back up a computer but not yet bring it back elsewhere. Its snapshot root is not yet authenticated, and signatures are not checked. Import is one huge transaction. And the physics is real: the paper's snapshot alone is about twenty-five gigabytes of unique chunks, and the tape is millions of small files. D@e's bundle tonight bore that out, at about four files for every event. Packing those into segment files is the next bundle slice.

You also asked what compaction really is. The design now lays out four answers: files split by time, incremental vacuum, compact snapshots with a planned rebase, and a rebuild from the tape. I chose compact snapshots with a planned rebase, because it reuses the planned-restart marker we already trust.

## The paper's size

The paper's store is thirty-two gigabytes. Retirement and the index trim should bring it to about eighteen once it is compacted, and the reviews estimate eight to ten gigabytes as a steady state. You said to settle for what is realistic if three gigabytes is not, so I have stopped treating three as the target. The biggest items are tool results and loop progress in the event table, and the object graph's indexes.

## Your two decisions

First, the privacy policy. The tape is append-only and sealed. If the policy promises that a deleted item is gone, then the tape needs a purge path, which means a crypto-shred of that item's key. Today a deletion leaves the working store but stays on the sealed tape.

Second, tool results. Retirement removes them from the working store after seven days. They stay on the tape, but acceptance re-synthesis that reads older tool results from the working store would no longer find them. If that matters, the window should be longer, or re-synthesis should read the tape.

## What I would do next

The one thing to remember: boot is light now, and every heavy operation has its own lane.

If I were holding the phone, I would answer the two decisions, then let the night's work run a day on the paper before widening telemetry batching beyond d@e. The next build slices are compact snapshots with a planned rebase, so the paper's file actually shrinks, then packed bundle segments, then the Mac pull.

## Names and receipts

- Mission: docs/definitions/choir-lean-system-2026-10-30.md, ledger alongside.
- Design: docs/design/one-tape-durability-2026-10-31.md (sections 6a, 6a2, 6b).
- Force-push: ad60a50d7 to 11efa88c3; backup branch backup/main-pre-rewrite-2026-10-31; Devin review 382.
- Outage fixes: 2734deee6, 53be139fe, hotfix 34f1b157e.
- Batch: b8ac237a8 (settings defaults, trace log deletion, paced retirement, bundle tool, vmctl remove answers, epoch 14), deployed 03:27 UTC.
- vmctl override narrowing and design notes: 888d0eef1, deployed 03:38 UTC; candidate-control-20260717-j removed after it.
- Held, not pushed: local branch hold/settings-defaults (three settings readers use declared defaults; touches a guest security surface, so it rides the next epoch raise).
- Paper: vm-1669020adcece51c2f5ff9b6df399ef5. d@e: vm-70070cf7337d45f2d46eafc83b9783b9.
- d@e bundle: /var/lib/go-choir-measure/bundle-de on Node B, 258 MB, 33,641 files, verified.
- Test-suite run: choir-agents, branch tests/optimize-2026-10-31, report docs/reports/test-suite-optimization-2026-10-31.md when it finishes.
