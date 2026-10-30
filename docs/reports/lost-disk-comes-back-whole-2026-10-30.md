# A lost disk comes back whole

I wrote this on the evening of October thirtieth. It covers about three hours of the data system mission, from late afternoon to nine at night, while you were away. It is a checkpoint letter, not the mission's closing report.

A computer that loses its disk now comes back whole, with its events and its files, in under a minute at d@e's size.

That was not true at the start of the evening. Two things stood in the way. The host refused to let an empty disk boot, because it planned against a record nothing has written since the cutover. And files saved in the hour before a crash never reached the host at all. Both are fixed, deployed, and proven by a drill that destroyed d@e's disk and read back every file it had just saved. Along the way, the telemetry retirement you asked for got built. It runs on d@e, and it is off on the paper until you turn it on, which is one owner act. Nothing else needs you.

## The disk that would not boot

I deleted d@e's disk on purpose, a little before seven. The host would not boot it. It was reading a recovery watermark from the old projection system, and that row had not moved since October twenty-eighth. Even a fresh row would have met a recovery limit of ten thousand events, set in the Dolt days, which the paper outgrows twenty minutes after each daily snapshot. I pointed the host at the newest snapshot instead, and I restated the limit in replay time: seven hundred and twenty thousand events, about forty minutes of replay. After that the empty disk booted. It took its key from escrow, installed its snapshot, replayed the tail, and matched the live state.

## The files that never left

Then I checked the files. None of the four hundred and fifty-seven files I had saved that afternoon came back. The guest only sent files to the host every fifteen minutes, on a timer started at boot. An idle computer is put to sleep about ten minutes after it boots, and it wakes cold, so that timer never fired. Nothing failed, and nothing logged. Mail lands in the same tree, so it had the same exposure. The guest now checks its files every two minutes and sends them when anything changed.

The first drill after that fix proved nothing, and that was my mistake. I started the writers while d@e was rebooting onto the new release, and I never made their folder, so not one save landed. It did teach me that a crashed computer stays down until its owner starts it, which is the restart rule working as designed. The second drill worked. Three writers saved one thousand and seven files in a minute. Two minutes later the host held a new file root. I destroyed the disk and started the computer. Fifty seconds later every one of the thousand and seven files read back, byte for byte.

## Replay speed

Replay runs at about three hundred and thirty events a second, up from two hundred. Fetching the payloads in parallel bought most of that. I believed the rest of the cost was disk syncs, and I said so in the design. That was only partly right. Turning the syncs down helped a little. The real cost is the work of finalizing each event, about a millisecond and a half, and that needs a profile, not a guess. I corrected the design text.

## Retirement, and a stall I caused

The retirement op removes old telemetry rows from a computer's working store as an event on its tape, so a replay agrees on what was dropped. The rows stay on the sealed tape. Agent traces older than fourteen days leave the working store. The unread kinds go now. Tool results stay.

A Sonnet builder wrote it in two passes. The first version would have read all two and a half million of the paper's telemetry rows for every batch it removed, while holding the write lock. I caught that by reading the query plan on a copy of the paper's disk. The fix reads one hour of rows at a time through a new index. That index built on the paper in thirty-five seconds at its planned restart, as forecast.

I also caused one stall on the paper. To time the deletes, I ran them on a copy of its disk, and those writes slowed the paper's own commits for about a second. The watchdog caught it on the first slow health check and stopped. Measurements on Node B now stay read-only.

## What to remember

The thing worth remembering is that the computer is now recoverable from its host alone, and the drill proves it with every file checked. If I were holding the phone, I would turn on retirement for the paper, and I would profile finalize next.

## Names and receipts

The recovery fix is ec2887f84, with the guest security epoch raised from eight to nine. The file sync fix is d900a7a46, epoch ten. Replay at synchronous NORMAL is b957ef538. The retirement op is bd475b1d5, its gate is the setting store.telemetry_retire, and its hour slices are ed807a520, epoch eleven. The last deploy is 42940338a. Drill G ran from 20:22 to 20:29 UTC: 1,007 of 1,007 files by sha256, replay-completeness equivalent, file root 143e2f4a. My stall on the paper was at 20:36:38 UTC. To turn retirement on for the paper, the paper's owner posts the key store.telemetry_retire with value true to /api/computer-settings. Everything is recorded in docs/definitions/choir-data-system-build-2026-10-30.ledger.md.
