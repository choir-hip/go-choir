# Podcast clips: three review rounds did not converge. Structural assessment

2026-11-16, 01:50Z. Written under the operating contract's rule: three
rounds without convergence, stop patching and change the approach.

## What happened

`lane/podcast-clips` was reviewed three times by two seats (DeepSeek and
Sonnet). Each round returned REPAIR from both.

| round | tip | blocker |
|---|---|---|
| 1 | a017cac76 | an anonymous feed report made the worker fetch any URL (SSRF) |
| 2 | 413324958 | `now_playing` created feed rows outside the account's cap; two cheap accounts could make a feed public |
| 3 | 2738595ce | the platform row cap does not hold on the refusal path and refuses the operator's own seed writes; one bad feed item rejects 200 episodes; the worker queue can be flooded by one account's feeds |

## The pattern

Every blocker in every round is in one part: **feeds reported by accounts**
(the route by which a phone tells the platform what its reader subscribes
to, and everything that follows from it: per-account caps, a platform row
cap, now-playing priority, subscriber-only transcripts, popularity counts).
That part was added mid-build from the owner's direction of the same night
("auto get the transcript of all new episodes of users' subscribed
podcasts"). Each repair added machinery (caps, indexes, a listing rule),
and each round found the next way an ordinary account could use the new
machinery against the platform.

The other part, **the operator's own pipeline** (a seed list of feeds the
operator features, the worker that polls and transcribes them, operator-only
writes, the matching job with its one model call, the public clips read,
the programme's clip segment), drew no blocker after round 1's network
guard. Both seats called its authorization, validation, key handling and
model-call discipline sound in rounds 2 and 3.

## What the thing is

Two different products were being landed as one change:

1. A public read of clips from shows the operator chose. Strangers can
   cause nothing: they only read.
2. A multi-tenant intake where any account adds work and stored rows. That
   is an abuse-resistance design problem (identity cost, unsubscribe,
   collection of dead rows, per-feed bounds, fair queueing). It was being
   designed by review rounds, at night, which is the wrong instrument.

## Decision

Split them.

- **Tonight's landing is part 1 only.** The account-reported feed route,
  the per-account and platform caps, now-playing priority, subscriber-only
  transcripts and popularity counts are removed from the branch, not
  repaired. Feeds come from the seed list and operator marks only. The
  transcript route serves featured feeds to a signed-in reader. The popular
  route lists featured feeds.
- The round-3 findings that apply to part 1 are fixed in the same series:
  a bad episode item is skipped and logged, not allowed to reject its whole
  request; a clip's story index is written so a partial write cannot hide a
  clip for good; the feed key trims a trailing dot; a missing index
  document is logged; a failed queue ranking is logged.
- One more review round on the smaller change. This is a round on a changed
  approach, not a fourth patch round.
- **Part 2 becomes its own mission** with a design note before any code:
  what an account may cause, what it costs an account to cause it,
  unsubscribe, collection, per-feed episode bounds, queue fairness, and how
  a widely followed show becomes featured. Until then the owner's own
  shows are added to the featured list by an operator act.

## What the owner loses tonight

The phone's report of subscriptions is answered 404 and ignored (the phone
treats that as "not available yet", silently). A show only the owner
follows gets a transcript only if it is on the featured list. "The
subscriptions of user B become recommendations to user A" is not served.

## Remaining error

Part 1 has not been reviewed in its reduced form. The deletion may leave
dead references. The matching job has never run against real transcripts
on staging.
