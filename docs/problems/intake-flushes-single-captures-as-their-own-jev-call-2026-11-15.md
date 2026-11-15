# Intake flushes single captures as their own Jev call

Date: 2026-11-15T11:17Z. Seen on the paper in the first fifteen minutes of
8b15ec2e9 (one entry point per Jev job), deployed 10:30Z.

## Problem

The owner's rule is that a job is done with as few Jev calls as possible:
one large context, every question it can answer in that call. The intake
flush takes whatever is waiting when it runs. When captures arrive a few
seconds apart, each becomes its own flush and its own call.

## Evidence

The paper's console log, `intake labels: flush` and `jev job:` lines:

- 10:40:37, 10:40:45: two flushes of 1 capture, 1 call each, 4 questions,
  about 23,400 estimated tokens each.
- 10:42:41, 10:42:59, 10:43:13, 10:43:16: four flushes of 1 capture, 1 call
  each, within 35 seconds, about 23,400 tokens each.
- 10:39:58 and 10:45:39: flushes of 12 captures, 2 calls each, split by the
  vendor's 64k request limit (`jev split: ... forced_by=map[request_tokens_64k:1]`).
- Every flush reports `waited_ms=0` and `unlabelled=0`.

A flush of one capture costs about 23,400 tokens; twelve captures fit in
two calls of about 64,000. So most of a single-capture call is the shared
state, sent again for each capture.

## Belief

The flush has no reason to wait, so it does not. The six single calls above
could have been two. The fix is in the caller: let a flush gather what
arrives while the previous one is in flight, or for a short stated window,
so one call carries them. That is not a limit on calls and not a queue of
calls; it is fewer, fuller calls. How long a capture may wait for its label
is a product question (a deadline), and the code should name it.

Also seen once, at 10:34:20, the first flush after boot: reading the story
index and the corrections timed out, and the flush of 1 capture made no
call and reported unlabelled=0. Whether that capture was labelled later is
not traced.

## Remaining error

The share of intake calls that are single-capture over a full day is not
counted; fifteen minutes after a boot may not be typical. Count it over the
C96 window before sizing the fix.
