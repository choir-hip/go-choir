# The day the paper got lighter

Written on the afternoon of October thirty-first. It covers the lean system work from about noon to a quarter past three UTC, the stretch after the night letter. It answers three questions: what we did, where the aggressive push stands, and how we get to the Mac app with its own virtual machines and sync.

The paper's store went from thirty-two gigabytes to twenty without losing a row, three compression releases are live, the paper now batches its telemetry, and your new finish line is written into the goal: state growth stopped, with no memory spikes to manage it.

## Where things stand

The paper is up on the latest release and publishing. Its store is twenty gigabytes with no free pages, down from thirty-two. Every new snapshot chunk, private payload and tape segment record is now written compressed, and the older forms still read. Telemetry batching is on by default for every computer, the paper included, so trace lines no longer go out one acknowledged event at a time. Real deletion of a destroyed computer is built and deployed: its tape, its snapshots, its database rows, and its escrow key. Telemetry partitions are deployed too, waiting to be turned on for d@e first.

Node B has a hundred and twenty-six gigabytes free of four hundred and seventy-six. That number fell by fifteen in the last hour, and I found why: every deploy adds a new guest release image of almost four gigabytes, and we deployed three times. That is exactly the kind of growth your new verifier is meant to catch.

Nothing waits on you right now. Two deletes wait on checks that are still running.

## The new finish line

You said the data system stops being the weakest link when the rapid state growth has stopped and nothing needs a memory spike to keep it in check. I wrote that into the goal as its stopping condition, with a baseline. I will call it met when Node B's used bytes and its database rows stay flat day over day, growing only with new user state, and when every maintenance job, meaning retirement, the compaction copy, snapshots, backup and the sweep, runs inside its memory line, for at least two days running.

This is a better verifier than the list of slices. A slice can land and the disk can still climb. The disk ledger cannot be argued with.

## Compaction on the paper

Around noon the compaction design you approved ran on the paper for real. While the paper kept working, it wrote a compact copy of its store, checked it, and set it aside. At the next planned restart, the paper swapped onto the copy in forty-four milliseconds and replayed under ten thousand events to catch up. The store went from thirty-two gigabytes to just over twenty. The rollback copy was removed two minutes later, after the replay proved it was not needed.

Before that, I drilled the same swap on a copy of the paper's disk. All twenty-seven tables and the head agreed, nearly six million rows. The lesson from last night held: nothing heavy runs before the boot credential exchange. The copy is made in the background, and the boot only renames files.

## Your decisions, and the policy

You decided the tape is the tape, and it stays. Binary blobs may go. You asked for whole-computer deletion, placed behind several confirmations, after sync to the user's own machine exists. Tool results can be long, so they leave the working store after seven days unless a run acceptance cites them as evidence. That exception is built and tested. Restore is for recovering from a failed self-development step, about a week back, and long history belongs on the user's synced machine. I rewrote the privacy policy to say this, in both copies.

## Where the bytes were

You asked how much free space there was, and why a few users need a hundred gigabytes of tape. The honest answer is that most of it was never your text. Snapshots of the paper were most of the snapshot store, and each one shared almost nothing with the last. The tape was held for two hundred and fifty-one computers when about six exist. And the small files cost more than their contents: the pin receipts take twenty-one gigabytes on disk but hold under five, because each nine-hundred-byte receipt fills a whole four-kilobyte block.

The deepest finding came from the investigation agent. The paper was not batching its telemetry. The batching setting defaulted to off, and the paper had never been switched on. So every trace line became its own event, with two signed receipts and a database row of almost two kilobytes. The paper alone holds five and a half million of the six and a half million receipt rows, and it was adding about three hundred thousand a day. D@e had run batching cleanly since the evening of the thirtieth, across three boots, so I made it the default. The paper booted with it on at fourteen forty-eight.

## The five compaction changes

You approved five changes and said to stay aggressive. Snapshots now come by distance along the tape, about every day and a half on the paper, with a weekly backstop, instead of every day. The paper keeps two. The nightly off-host backup no longer copies snapshot chunks for the paper or your computer, because their whole disk images are already in it, and the paper's snapshots were most of the hundred and nine gigabytes the last run added. Telemetry now goes into one file per day, starting with tool results, so retiring a day is deleting a file and a closed day's snapshot chunks repeat exactly. And compression went out in two releases: readers first, so a rollback can always read what was written, then the writes.

I have not measured chunk reuse between two compact snapshots yet. The first snapshots after compression wrote only about a third of a gigabyte of new chunks between the paper and your computer, which is promising, but I have not tied those chunks to their roots, so I am not claiming the number.

## Real deletion

A destroyed computer now loses everything of its own. That covers its tape segments, its snapshot and file chunks, its rows in every per-computer table, and its key escrow, which makes any copy left behind, including old backups, unreadable. Test computers can skip the archive. The paper and your computer are refused at four separate points, even against a forged manifest. The head row and the tombstone stay, holding no content, so a reused identifier can never start a second history. The tests ran on the Mac and on the agents machine, which now has PostgreSQL thanks to your approval.

## What went wrong today

The sweep dry run for destroyed computers lives only in memory, and each platform deploy restarts the service that holds it. It was lost twice. It is running a third time now, with no deploy queued. That is a defect to fix: a long job should survive a restart.

I also repeated an old mistake. A kill command on the agents machine matched its own shell and ended the session. The lesson is written down already, and I need to read my own notes before I type the pattern.

A test on the agents machine kept failing because that machine had no database, so every database case was skipped, and the outer check then counted zero lines. The fix was the test and the machine, not the code. One more test, about a missed wake signal, fails about a third of the time on main. It is named in the ledger and not fixed.

## Getting to the Mac app

Here is the honest map. The part that carries a computer between machines exists. The part that runs it on a Mac does not.

The bundle exists. It exports a computer's tape and snapshot into one packed folder, verifies the chain, imports on another host, and sends only what is new since the last bundle. On d@e, a bundle went from thirty-three thousand files to nineteen and from seventy-eight seconds to eleven. Compact snapshots and compression make a bundle of the paper small enough to move.

Four things stand between a bundle and your computer running on your Mac.

First, the keys. A bundle carries no keys, so on another machine it is sealed bytes. The design puts the computer's key in the Mac's Keychain.

Second, authenticity. Verify checks digests and the chain, but not the signatures on the receipts, and import does not yet refuse a snapshot the tape does not name.

Third, the fence. Two machines serving one computer would fork its tape. Until a writer epoch lands, every move must be cold: stop here, export, import there, start there. The writer epoch is the slice that makes sync safe in both directions.

Fourth, the machine itself. The hypervisor seam is a design from October twenty-first, not code. Nothing in the virtual machine manager today can launch anything but Firecracker. A Mac needs an Apple Virtualization backend, an arm build of the guest image, and a small host program that keeps a local tape store and the key.

## How to accelerate

These four pieces split cleanly by package, so they can run in parallel, the way today ran.

One builder takes the seam: put the launch path behind an interface, with Firecracker as its only implementation, and no behaviour change on Node B. A second builds the arm guest image in Nix, which can be checked on its own by booting it. A third finishes bundle authenticity: signature checks and the refusal of an unnamed root. A fourth builds the writer epoch, the fence, which is the riskiest and gets the most review.

Then the Mac host comes together on top: an Apple Virtualization backend behind the seam, the key in the Keychain, and import of a bundle of your computer. The first proof is your own computer moved from Node B to your Mac, used, and moved back, cold. Sync is then the same export and import, run under the fence, in both directions.

The growth work should keep running beside this, because it is the verifier. The next growth cuts are: delete the paper's legacy loose tape once the census proves the segments hold it, cut guest release retention so deploys stop adding four gigabytes each, purge the dead computers, and pack the small receipt files so they stop paying a whole block each.

## The one thing to remember

The data system is now small enough to move, and the remaining work toward the Mac is four parallel pieces: the seam, the arm image, bundle authenticity, and the fence. Meanwhile the disk ledger, not the slice list, tells us when the data system is done.

If I were holding the phone, I would say yes to starting the seam and the arm image builders now, while the census and the sweep finish.

## Names and receipts

- Goal: docs/definitions/choir-lean-system-2026-10-30.md, stopping condition at commit 77cfe8f22; ledger: docs/definitions/choir-lean-system-2026-10-30.ledger.md
- Design: docs/design/one-tape-durability-2026-10-31.md, sections 3, 4, 4a, 5, 6, 6a2, 6b, 6c
- Compaction on the paper: copy staged 12:42 UTC, rebase at the 12:53 planned restart in 44 ms with a 9,673-event replay; state.db 32.4 GB to 20.4 GB
- Releases today: fd6ca5b2d, e7c099e6a, 88ed23e86 (compaction and keep_cited); 6ab7eb060 (compression readers, snapshots by tape distance, epoch 19); 0164a0975 (segment reader, backup exclusion); 27f8696bd (compressed writes, telemetry.batch default on, real deletion, epoch 20); ccf3429a6 (telemetry partitions, epoch 21)
- The paper on 27f8696bd: planned restart 14:48:22 UTC, tail 0, telemetry batching on at 14:48:34
- Real deletion: internal/platform/tape_purge.go and tape_purge_test.go; dry-run job ts-20261031T151105Z-f0fdd637
- Telemetry partitions: internal/store/telemetry_partition.go; switch store.telemetry_partitions, d@e first
- Agents VM: node-a /root/mike-infra/node-a/choir-agents.nix, postgresql_17 and pathsToLink /share/postgresql; backup /root/choir-agents.nix.bak-2026-10-31
- Node B at 15:15 UTC: 336 GB used of 476; guest-releases 26 GB; btrfs metadata 17.6 GB; append receipts table 17 GB, 6.62 million rows
- Census for the paper's loose tape: /var/lib/go-choir/taperepack-work on Node B, still running
- Flaky test: TestMissedSignalWakeIsDeliveredByAudit, 13 of 30 on main
- Prior letter: docs/reports/night-of-the-lean-system-2026-10-31.md
