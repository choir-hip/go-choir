# The podcast worker's transcripts are refused for one invalid segment

Recorded 2026-11-16 11:10Z, from the worker log of the overnight run on the Mac.

## Problem

The server refuses a whole transcript when one segment fails its check. The
worker retries the episode with a growing skip (1 h, then 4 h) and sends the
same transcript again, so the episode never gets a transcript and never yields
clips.

## Evidence

- Worker log, 02:00Z to 11:05Z: 24 refusals over 7 distinct episodes, against
  111 episodes transcribed. Shows: The Playbook Podcast, Meet the Press, On
  with Kara Swisher, Decoder with Nilay Patel, Bloomberg Tech.
- The refusal is always the same line from
  `PUT /api/platform/operator/podcasts/transcripts`: `400 invalid podcast
  request: segment N is empty, over 2000 bytes, ends before it starts, or is
  out of order` (N was 62, 377, 389, 772 in the last four).
- Nine other failures in the same log are downloads (ad-serving hosts), which
  the worker is designed to skip.

## Belief

The affected shows are long interviews. The likeliest cause is a segment over
2000 bytes where the recogniser returns a long run with no sentence break, or
a zero-length or overlapping segment at a chunk join. This is a belief: the
error names four conditions in one message and the worker does not keep the
transcript it sent, so the log cannot say which.

## Remaining error

- Which of the four conditions fires is not known. The cheap observer upgrade
  is for the server to name the condition and for the worker to log the
  offending segment's bounds and byte length.
- The fix belongs in the worker (split an over-long segment, drop an empty
  one, clamp an overlap) and not in loosening the server's check, which bounds
  what an operator key can store.
- Retrying an unchanged transcript is wasted work: about 7 episodes are
  transcribed again every few hours while the worker runs.
