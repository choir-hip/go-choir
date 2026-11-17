# Long podcast transcripts refused whole, and downloads that never left the first address

2026-11-17.

## Problem

The Mac worker's log of 2026-11-16 holds 57 failed-episode lines. 45 are the
server's 400 `segment N is empty, over 2000 bytes, ends before it starts, or is
out of order` (PUT transcripts), for about a dozen long interviews and news
hours (Playbook, Kara Swisher, Newshour, Fox News Rundown, Reuters; indices 62
to 772), each retried up to seven times with the same transcript, each retry a
full download and transcription. Five more are `download: ... connection
refused` to a Bloomberg Businessweek ad host on its first IPv6 address; the
episodes succeeded hours later. One is a 502 and one a 404.

## Evidence

- worker.log (read only). The 400 message merges five conditions
  (internal/platform/podcast.go, PutPodcastTranscript): empty, over 2000
  bytes, end not after start, start more than 0.5 s before the previous end,
  invalid UTF-8. The log does not say which one; the transcripts are not
  stored locally and no key was used to read them back.
- `guardedDialer` (internal/podcastworker/netguard.go) checked every resolved
  address and then dialled only the first, so a refusing first address failed
  the host with no fallback.

## Belief

Which of the five conditions a given episode broke is unconfirmed. The worker
could produce all of them: `Segments` cuts only at sentence ends, pauses and
45 s of word timestamps, so words that share a timestamp or a long run
without punctuation make one huge segment; a word with end equal to start
makes a zero-length segment; overlapping words from long audio make an
out-of-order one. The fix does not need to know: repair all of them before the
send and name the first remaining offender.

## What was done

- `RepairSegments` and `ValidateSegments` (internal/podcastworker/segment.go):
  empty dropped, zero length given 0.05 s, overlap moved to the previous end,
  a segment over 1800 bytes cut on word boundaries with times shared in
  proportion; no word changes. The worker repairs, validates and logs the
  repair counts; a transcript that still fails names the episode, segment
  index, bounds, size and reason, and a refusal (the server's 4xx or this
  validation) is not retried for six hours.
- The dialer tries every permitted address in order.

## Remaining error

The cause per episode is a hypothesis until the next log shows `segments
repaired` lines for the episodes that failed. Episodes that failed on
2026-11-16 are still pending on the server and are re-fetched and
transcribed once the new worker runs.
